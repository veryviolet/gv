package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
)

// catalogServer serves an OCI catalogue with the last/n pagination the real
// registry implements.
func catalogServer(t *testing.T, all []string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v2/" {
			w.WriteHeader(http.StatusOK)
			return
		}
		if r.URL.Path != "/v2/_catalog" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		last := r.URL.Query().Get("last")
		n, _ := strconv.Atoi(r.URL.Query().Get("n"))
		if n <= 0 {
			n = 100
		}
		out := []string{}
		for _, name := range all {
			if last != "" && name <= last {
				continue
			}
			out = append(out, name)
			if len(out) == n {
				break
			}
		}
		json.NewEncoder(w).Encode(map[string]any{"repositories": out})
	}))
}

func testRegistry(t *testing.T, srv *httptest.Server) *RegistryClient {
	t.Helper()
	t.Setenv("GV_REGISTRY_URL", srv.URL)
	return NewRegistryClient("gitverse.ru", "veryviolet", "token", "gv/test")
}

func TestRepositoriesForOwnerFiltersByPrefix(t *testing.T) {
	// Sorted the way a registry catalogue is.
	all := []string{
		"aaa/one", "aaa/two",
		"veryviolet/first", "veryviolet/nested/image", "veryviolet/second",
		"zzz/other",
	}
	srv := catalogServer(t, all)
	defer srv.Close()

	got, err := testRegistry(t, srv).RepositoriesForOwner(context.Background(), "veryviolet", 0)
	if err != nil {
		t.Fatalf("RepositoriesForOwner: %v", err)
	}
	want := []string{"veryviolet/first", "veryviolet/nested/image", "veryviolet/second"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("entry %d = %q, want %q", i, got[i], want[i])
		}
	}
}

// The catalogue holds every repository on the platform, so gv must start at
// the owner's prefix instead of paging from the beginning.
func TestRepositoriesForOwnerSkipsEarlierPages(t *testing.T) {
	all := make([]string, 0, 501)
	for i := 0; i < 500; i++ {
		all = append(all, fmt.Sprintf("aaa/repo-%03d", i))
	}
	all = append(all, "veryviolet/gv")

	var requests int
	base := catalogServer(t, all)
	defer base.Close()
	counting := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		base.Config.Handler.ServeHTTP(w, r)
	}))
	defer counting.Close()

	got, err := testRegistry(t, counting).RepositoriesForOwner(context.Background(), "veryviolet", 0)
	if err != nil {
		t.Fatalf("RepositoriesForOwner: %v", err)
	}
	if len(got) != 1 || got[0] != "veryviolet/gv" {
		t.Fatalf("got %v, want [veryviolet/gv]", got)
	}
	if requests > 2 {
		t.Errorf("made %d catalogue requests; the owner prefix should need at most 2", requests)
	}
}

func TestRepositoriesForOwnerRespectsLimit(t *testing.T) {
	srv := catalogServer(t, []string{"veryviolet/a", "veryviolet/b", "veryviolet/c"})
	defer srv.Close()

	got, err := testRegistry(t, srv).RepositoriesForOwner(context.Background(), "veryviolet", 2)
	if err != nil {
		t.Fatalf("RepositoriesForOwner: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("got %d repositories, want 2", len(got))
	}
}

func TestRegistryUsesBasicAuth(t *testing.T) {
	var user, pass string
	var ok bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok = r.BasicAuth()
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	if err := testRegistry(t, srv).Ping(context.Background()); err != nil {
		t.Fatalf("Ping: %v", err)
	}
	if !ok || user != "veryviolet" || pass != "token" {
		t.Errorf("basic auth = %q/%q (present=%v)", user, pass, ok)
	}
}

func TestRegistryEndpointAndHost(t *testing.T) {
	t.Setenv("GV_REGISTRY_URL", "")
	if got := RegistryEndpoint("gitverse.ru"); got != "https://gitverse.ru" {
		t.Errorf("RegistryEndpoint = %q", got)
	}
	if got := RegistryHost("gitverse.ru"); got != "gitverse.ru" {
		t.Errorf("RegistryHost = %q", got)
	}
	if got := RegistryEndpoint(""); got != "https://gitverse.ru" {
		t.Errorf("RegistryEndpoint(\"\") = %q", got)
	}
}
