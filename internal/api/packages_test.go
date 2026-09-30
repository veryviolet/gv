package api

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPackagePathsSplitUsersAndOrgs(t *testing.T) {
	cases := []struct {
		owner PackageOwner
		want  string
	}{
		{PackageOwner{Name: "veryviolet"}, "/users/veryviolet/packages/container/gv"},
		{PackageOwner{Name: "myorg", IsOrg: true}, "/orgs/myorg/packages/container/gv"},
	}
	for _, tc := range cases {
		if got := packagePath(tc.owner, "container", "gv"); got != tc.want {
			t.Errorf("packagePath = %q, want %q", got, tc.want)
		}
	}
}

// Image names may contain slashes ("owner/my/image"); they must survive as
// path separators rather than being escaped away.
func TestPackagePathKeepsNestedNames(t *testing.T) {
	got := packagePath(PackageOwner{Name: "veryviolet"}, "container", "nested/image")
	if want := "/users/veryviolet/packages/container/nested/image"; got != want {
		t.Errorf("packagePath = %q, want %q", got, want)
	}
}

func TestListPackageVersions(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		fmt.Fprint(w, `[{"id":42,"name":"1.0","metadata":{"package_type":"container"}}]`)
	}))
	defer srv.Close()

	versions, err := ListPackageVersions(context.Background(), newTestClient(t, srv),
		PackageOwner{Name: "veryviolet"}, "container", "gv", 0)
	if err != nil {
		t.Fatalf("ListPackageVersions: %v", err)
	}
	if want := "/users/veryviolet/packages/container/gv/versions"; gotPath != want {
		t.Errorf("path = %q, want %q", gotPath, want)
	}
	if len(versions) != 1 || versions[0].Name != "1.0" || versions[0].ID != 42 {
		t.Fatalf("versions = %+v", versions)
	}
	if got := versions[0].PackageType("fallback"); got != "container" {
		t.Errorf("PackageType = %q, want container", got)
	}
}

func TestPackageTypeFallsBackWhenMetadataEmpty(t *testing.T) {
	v := PackageVersion{}
	if got := v.PackageType("container"); got != "container" {
		t.Errorf("PackageType = %q, want the fallback", got)
	}
}

func TestDeletePackageAndVersion(t *testing.T) {
	var paths []string
	var methods []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		methods = append(methods, r.Method)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	owner := PackageOwner{Name: "veryviolet"}
	if err := DeletePackage(context.Background(), c, owner, "container", "gv"); err != nil {
		t.Fatalf("DeletePackage: %v", err)
	}
	if err := DeletePackageVersion(context.Background(), c, owner, "container", "gv", "42"); err != nil {
		t.Fatalf("DeletePackageVersion: %v", err)
	}

	want := []string{
		"/users/veryviolet/packages/container/gv",
		"/users/veryviolet/packages/container/gv/versions/42",
	}
	for i := range want {
		if paths[i] != want[i] {
			t.Errorf("request %d path = %q, want %q", i, paths[i], want[i])
		}
		if methods[i] != http.MethodDelete {
			t.Errorf("request %d method = %s, want DELETE", i, methods[i])
		}
	}
}

// GitVerse reports validation failures as {"error": {...}}; the message must
// survive into the error text instead of being dropped.
func TestHTTPErrorParsesNestedEnvelope(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, `{"error":{"code":"VALIDATION_ERROR","message":"Invalid request parameters","status":400}}`)
	}))
	defer srv.Close()

	err := newTestClient(t, srv).REST(context.Background(), http.MethodGet, "/x", nil, nil)
	httpErr, ok := err.(*HTTPError)
	if !ok {
		t.Fatalf("error type = %T, want *HTTPError", err)
	}
	if want := "VALIDATION_ERROR: Invalid request parameters"; httpErr.Message != want {
		t.Errorf("Message = %q, want %q", httpErr.Message, want)
	}
}
