package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func newTestClient(t *testing.T, srv *httptest.Server, opts ...func(*ClientOptions)) *Client {
	t.Helper()
	o := ClientOptions{
		BaseURL:   srv.URL,
		Token:     "secret-token",
		UserAgent: "gv/test",
	}
	for _, fn := range opts {
		fn(&o)
	}
	c, err := NewClient(o)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	return c
}

func TestAPIEndpoint(t *testing.T) {
	cases := map[string]string{
		"":                "https://api.gitverse.ru",
		"gitverse.ru":     "https://api.gitverse.ru",
		"api.gitverse.ru": "https://api.gitverse.ru",
		"git.example.com": "https://git.example.com/api",
	}
	for host, want := range cases {
		if got := APIEndpoint(host); got != want {
			t.Errorf("APIEndpoint(%q) = %q, want %q", host, got, want)
		}
	}
}

func TestRequestHeaders(t *testing.T) {
	var gotAuth, gotAccept, gotUA string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotAccept = r.Header.Get("Accept")
		gotUA = r.Header.Get("User-Agent")
		fmt.Fprint(w, `{"login":"veryviolet"}`)
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	var user User
	if err := c.REST(context.Background(), http.MethodGet, "/user", nil, &user); err != nil {
		t.Fatalf("REST: %v", err)
	}

	if want := "Bearer secret-token"; gotAuth != want {
		t.Errorf("Authorization = %q, want %q", gotAuth, want)
	}
	if want := "application/vnd.gitverse.object+json;version=1"; gotAccept != want {
		t.Errorf("Accept = %q, want %q", gotAccept, want)
	}
	if gotUA != "gv/test" {
		t.Errorf("User-Agent = %q, want gv/test", gotUA)
	}
	if user.Login != "veryviolet" {
		t.Errorf("Login = %q, want veryviolet", user.Login)
	}
}

func TestRESTResolvesRelativeAndAbsolutePaths(t *testing.T) {
	var paths []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		fmt.Fprint(w, `{}`)
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	for _, p := range []string{"/user", "user", srv.URL + "/user"} {
		if err := c.REST(context.Background(), http.MethodGet, p, nil, nil); err != nil {
			t.Fatalf("REST(%q): %v", p, err)
		}
	}
	for _, got := range paths {
		if got != "/user" {
			t.Errorf("resolved path = %q, want /user", got)
		}
	}
}

func TestHTTPErrorCarriesMessageAndBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		fmt.Fprint(w, `{"message":"validation failed","errors":[{"message":"title is required"}]}`)
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	err := c.REST(context.Background(), http.MethodPost, "/repos/o/r/issues", nil, nil)
	if err == nil {
		t.Fatal("expected an error")
	}
	httpErr, ok := err.(*HTTPError)
	if !ok {
		t.Fatalf("error type = %T, want *HTTPError", err)
	}
	if httpErr.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("StatusCode = %d, want 422", httpErr.StatusCode)
	}
	if httpErr.Message != "validation failed" {
		t.Errorf("Message = %q", httpErr.Message)
	}
	if len(httpErr.Errors) != 1 || httpErr.Errors[0] != "title is required" {
		t.Errorf("Errors = %v", httpErr.Errors)
	}
	if !strings.Contains(httpErr.Error(), "title is required") {
		t.Errorf("Error() = %q, want it to mention the field error", httpErr.Error())
	}
}

func TestHTTPErrorFallsBackToPlainBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		fmt.Fprint(w, "token lacks the required scope")
	}))
	defer srv.Close()

	err := newTestClient(t, srv).REST(context.Background(), http.MethodGet, "/user", nil, nil)
	httpErr, ok := err.(*HTTPError)
	if !ok {
		t.Fatalf("error type = %T, want *HTTPError", err)
	}
	if httpErr.Message != "token lacks the required scope" {
		t.Errorf("Message = %q", httpErr.Message)
	}
	if !IsUnauthorized(err) {
		t.Error("IsUnauthorized = false, want true for 403")
	}
}

func TestIsNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	err := newTestClient(t, srv).REST(context.Background(), http.MethodGet, "/repos/o/r", nil, nil)
	if !IsNotFound(err) {
		t.Errorf("IsNotFound(%v) = false, want true", err)
	}
}

func TestRetriesOnServerError(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) < 3 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		fmt.Fprint(w, `{"login":"veryviolet"}`)
	}))
	defer srv.Close()

	c := newTestClient(t, srv, func(o *ClientOptions) { o.Timeout = 5 * time.Second })
	var user User
	if err := c.REST(context.Background(), http.MethodGet, "/user", nil, &user); err != nil {
		t.Fatalf("REST: %v", err)
	}
	if got := calls.Load(); got != 3 {
		t.Errorf("calls = %d, want 3", got)
	}
}

func TestDoReplaysBodyOnRetry(t *testing.T) {
	var bodies []string
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf := make([]byte, r.ContentLength)
		r.Body.Read(buf)
		bodies = append(bodies, string(buf))
		if calls.Add(1) < 2 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		fmt.Fprint(w, `{}`)
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	in := map[string]string{"title": "hello"}
	if err := c.RESTWithBody(context.Background(), http.MethodPost, "/repos/o/r/issues", in, nil); err != nil {
		t.Fatalf("RESTWithBody: %v", err)
	}
	if len(bodies) != 2 {
		t.Fatalf("got %d requests, want 2", len(bodies))
	}
	if bodies[0] != bodies[1] {
		t.Errorf("retry sent a different body: %q vs %q", bodies[0], bodies[1])
	}
	var decoded map[string]string
	if err := json.Unmarshal([]byte(bodies[1]), &decoded); err != nil || decoded["title"] != "hello" {
		t.Errorf("body = %q", bodies[1])
	}
}

func TestNoContentLeavesOutputUntouched(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	out := User{Login: "unchanged"}
	if err := newTestClient(t, srv).REST(context.Background(), http.MethodDelete, "/repos/o/r", nil, &out); err != nil {
		t.Fatalf("REST: %v", err)
	}
	if out.Login != "unchanged" {
		t.Errorf("Login = %q, want unchanged", out.Login)
	}
}
