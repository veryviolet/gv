package api

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
)

func TestParseLinkHeader(t *testing.T) {
	header := `<https://api.gitverse.ru/user/repos?page=2>; rel="next", <https://api.gitverse.ru/user/repos?page=9>; rel="last"`
	links := ParseLinkHeader(header)
	if got := links["next"]; got != "https://api.gitverse.ru/user/repos?page=2" {
		t.Errorf("next = %q", got)
	}
	if got := links["last"]; got != "https://api.gitverse.ru/user/repos?page=9" {
		t.Errorf("last = %q", got)
	}
	if len(ParseLinkHeader("")) != 0 {
		t.Error("empty header should yield no links")
	}
}

// TestListAllFollowsLinkHeader covers the case where the server advertises the
// next page explicitly.
func TestListAllFollowsLinkHeader(t *testing.T) {
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Query().Get("page") {
		case "", "1":
			w.Header().Set("Link", fmt.Sprintf(`<%s/user/repos?page=2>; rel="next"`, srv.URL))
			fmt.Fprint(w, `[{"name":"one"},{"name":"two"}]`)
		default:
			fmt.Fprint(w, `[{"name":"three"}]`)
		}
	}))
	defer srv.Close()

	repos, err := ListAll[Repository](context.Background(), newTestClient(t, srv), "/user/repos", 0)
	if err != nil {
		t.Fatalf("ListAll: %v", err)
	}
	if len(repos) != 3 {
		t.Fatalf("got %d repositories, want 3", len(repos))
	}
	if repos[2].Name != "three" {
		t.Errorf("last repository = %q, want three", repos[2].Name)
	}
}

// TestListAllIncrementsPageWithoutLinkHeader covers the GitVerse behaviour
// documented in the specification: page/per_page query parameters only.
func TestListAllIncrementsPageWithoutLinkHeader(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		page, _ := strconv.Atoi(r.URL.Query().Get("page"))
		if page == 0 {
			page = 1
		}
		switch {
		case page < 3:
			// A full page keeps pagination going.
			items := make([]string, DefaultPerPage)
			for i := range items {
				items[i] = fmt.Sprintf(`{"name":"repo-%d-%d"}`, page, i)
			}
			fmt.Fprintf(w, "[%s]", join(items, ","))
		default:
			fmt.Fprint(w, `[{"name":"last"}]`)
		}
	}))
	defer srv.Close()

	repos, err := ListAll[Repository](context.Background(), newTestClient(t, srv), "/user/repos", 0)
	if err != nil {
		t.Fatalf("ListAll: %v", err)
	}
	want := DefaultPerPage*2 + 1
	if len(repos) != want {
		t.Fatalf("got %d repositories, want %d", len(repos), want)
	}
	if repos[len(repos)-1].Name != "last" {
		t.Errorf("last repository = %q, want last", repos[len(repos)-1].Name)
	}
}

func TestListAllRespectsLimit(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `[{"name":"a"},{"name":"b"},{"name":"c"}]`)
	}))
	defer srv.Close()

	repos, err := ListAll[Repository](context.Background(), newTestClient(t, srv), "/user/repos", 2)
	if err != nil {
		t.Fatalf("ListAll: %v", err)
	}
	if len(repos) != 2 {
		t.Errorf("got %d repositories, want 2", len(repos))
	}
}

func TestPaginateStopsOnShortPage(t *testing.T) {
	var requests int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		fmt.Fprint(w, `[{"name":"only"}]`)
	}))
	defer srv.Close()

	err := newTestClient(t, srv).Paginate(context.Background(), http.MethodGet, "/user/repos", 0,
		func(body []byte, resp *http.Response) (bool, error) { return true, nil })
	if err != nil {
		t.Fatalf("Paginate: %v", err)
	}
	if requests != 1 {
		t.Errorf("made %d requests, want 1", requests)
	}
}

func join(items []string, sep string) string {
	out := ""
	for i, s := range items {
		if i > 0 {
			out += sep
		}
		out += s
	}
	return out
}
