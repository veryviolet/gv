package api

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

type capturedUpload struct {
	method      string
	path        string
	contentType string
	body        string
	user        string
	pass        string
	length      int64
}

func uploadServer(t *testing.T, status int) (*httptest.Server, *capturedUpload) {
	t.Helper()
	got := &capturedUpload{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		got.method = r.Method
		got.path = r.URL.Path
		got.contentType = r.Header.Get("Content-Type")
		got.body = string(body)
		got.length = r.ContentLength
		got.user, got.pass, _ = r.BasicAuth()
		w.WriteHeader(status)
	}))
	return srv, got
}

func tempFile(t *testing.T, name, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writing %s: %v", path, err)
	}
	return path
}

func testPackageClient(t *testing.T, srv *httptest.Server) *PackageRegistryClient {
	t.Helper()
	t.Setenv("GV_PACKAGES_URL", srv.URL)
	return NewPackageRegistryClient("gitverse.ru", "veryviolet", "token", "gv/test")
}

// The registry answers a form-encoded body with a 500, so the upload must go
// up as an opaque stream. This pins that down.
func TestUploadSendsRawBody(t *testing.T) {
	srv, got := uploadServer(t, http.StatusCreated)
	defer srv.Close()

	file := tempFile(t, "gv_0.2.0-1_amd64.deb", "binary-ish content")
	if err := testPackageClient(t, srv).UploadDebian(context.Background(), "veryviolet", "stable", "main", file); err != nil {
		t.Fatalf("UploadDebian: %v", err)
	}

	if got.method != http.MethodPut {
		t.Errorf("method = %s, want PUT", got.method)
	}
	if got.contentType != "application/octet-stream" {
		t.Errorf("Content-Type = %q, want application/octet-stream", got.contentType)
	}
	if got.body != "binary-ish content" {
		t.Errorf("body = %q", got.body)
	}
	if got.length != int64(len("binary-ish content")) {
		t.Errorf("Content-Length = %d, want %d", got.length, len("binary-ish content"))
	}
	if got.user != "veryviolet" || got.pass != "token" {
		t.Errorf("basic auth = %q/%q", got.user, got.pass)
	}
}

func TestUploadPaths(t *testing.T) {
	file := tempFile(t, "pkg.bin", "x")

	cases := []struct {
		name string
		call func(*PackageRegistryClient) error
		want string
	}{
		{
			"debian",
			func(c *PackageRegistryClient) error {
				return c.UploadDebian(context.Background(), "veryviolet", "stable", "main", file)
			},
			"/veryviolet/debian/pool/stable/main/upload",
		},
		{
			"rpm grouped",
			func(c *PackageRegistryClient) error {
				return c.UploadRPM(context.Background(), "veryviolet", "gv", file)
			},
			"/veryviolet/rpm/gv/upload",
		},
		{
			"rpm root",
			func(c *PackageRegistryClient) error {
				return c.UploadRPM(context.Background(), "veryviolet", "", file)
			},
			"/veryviolet/rpm/upload",
		},
		{
			"generic",
			func(c *PackageRegistryClient) error {
				return c.UploadGeneric(context.Background(), "veryviolet", "gv", "0.2.0", file)
			},
			"/veryviolet/generic/gv/0.2.0/pkg.bin",
		},
	}

	for _, tc := range cases {
		srv, got := uploadServer(t, http.StatusCreated)
		client := testPackageClient(t, srv)
		if err := tc.call(client); err != nil {
			t.Errorf("%s: %v", tc.name, err)
		}
		if got.path != tc.want {
			t.Errorf("%s path = %q, want %q", tc.name, got.path, tc.want)
		}
		srv.Close()
	}
}

// A duplicate upload is rejected because versions are immutable; the server's
// explanation must reach the user.
func TestUploadSurfacesServerMessage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusConflict)
		fmt.Fprint(w, "package file already exists")
	}))
	defer srv.Close()

	file := tempFile(t, "gv.deb", "x")
	err := testPackageClient(t, srv).UploadDebian(context.Background(), "veryviolet", "stable", "main", file)
	httpErr, ok := err.(*HTTPError)
	if !ok {
		t.Fatalf("error type = %T, want *HTTPError", err)
	}
	if httpErr.StatusCode != http.StatusConflict {
		t.Errorf("StatusCode = %d, want 409", httpErr.StatusCode)
	}
	if httpErr.Message != "package file already exists" {
		t.Errorf("Message = %q", httpErr.Message)
	}
}

func TestClientHintURLs(t *testing.T) {
	t.Setenv("GV_PACKAGES_URL", "")

	if got, want := DebianRepositoryKey("gitverse.ru", "veryviolet"),
		"https://gitverse.ru/api/packages/veryviolet/debian/repository.key"; got != want {
		t.Errorf("DebianRepositoryKey = %q, want %q", got, want)
	}
	if got, want := RPMRepoFile("gitverse.ru", "veryviolet", "gv"),
		"https://gitverse.ru/api/packages/veryviolet/rpm/gv.repo"; got != want {
		t.Errorf("RPMRepoFile = %q, want %q", got, want)
	}
	if got, want := RPMRepoFile("gitverse.ru", "veryviolet", ""),
		"https://gitverse.ru/api/packages/veryviolet/rpm.repo"; got != want {
		t.Errorf("RPMRepoFile without a group = %q, want %q", got, want)
	}

	line := DebianSourceLine("gitverse.ru", "veryviolet", "stable", "main", "/etc/apt/keyrings/gv.asc")
	want := "deb [signed-by=/etc/apt/keyrings/gv.asc] https://gitverse.ru/api/packages/veryviolet/debian stable main"
	if line != want {
		t.Errorf("DebianSourceLine = %q, want %q", line, want)
	}
}

func TestPackageKindForFile(t *testing.T) {
	cases := map[string]string{
		"gv_0.2.0-1_amd64.deb":  "deb",
		"gv-0.2.0-1.x86_64.rpm": "rpm",
		"gv_0.2.0_linux.tar.gz": "generic",
		"SHA256SUMS":            "generic",
		"GV_0.2.0-1_AMD64.DEB":  "deb",
	}
	for file, want := range cases {
		if got := PackageKindForFile(file); got != want {
			t.Errorf("PackageKindForFile(%q) = %q, want %q", file, got, want)
		}
	}
}
