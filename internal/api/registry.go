package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// The container registry speaks the OCI distribution protocol, not the REST
// API, so it needs its own small client: a different base URL, a different
// media type and HTTP basic auth with the personal access token as password.

// RegistryClient talks to the GitVerse container registry.
type RegistryClient struct {
	http     *http.Client
	baseURL  string
	username string
	token    string
	ua       string
}

// NewRegistryClient builds a client for the registry of host.
func NewRegistryClient(host, username, token, userAgent string) *RegistryClient {
	if username == "" {
		// The registry ignores the user name for token auth, but basic auth
		// still needs something in the field.
		username = "x-access-token"
	}
	return &RegistryClient{
		http:     &http.Client{Timeout: 60 * time.Second},
		baseURL:  RegistryEndpoint(host),
		username: username,
		token:    token,
		ua:       userAgent,
	}
}

// RegistryEndpoint maps a GitVerse host to its registry root. The registry
// lives on the main domain rather than a separate one, the way ghcr.io is
// separate from github.com.
func RegistryEndpoint(host string) string {
	if v := envRegistryURL(); v != "" {
		return strings.TrimSuffix(v, "/")
	}
	host = strings.TrimSuffix(strings.TrimPrefix(strings.TrimPrefix(host, "https://"), "http://"), "/")
	if host == "" {
		host = "gitverse.ru"
	}
	return "https://" + host
}

// RegistryHost reports the host name to pass to `docker login`.
func RegistryHost(host string) string {
	return strings.TrimPrefix(strings.TrimPrefix(RegistryEndpoint(host), "https://"), "http://")
}

func (r *RegistryClient) do(ctx context.Context, path string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, r.baseURL+path, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", r.ua)
	if r.token != "" {
		req.SetBasicAuth(r.username, r.token)
	}

	resp, err := r.http.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, newHTTPError(resp, body)
	}
	return resp, nil
}

// Catalog lists repository names in the registry, starting after last.
//
// The catalogue is global and lexicographically ordered, so callers that want
// one owner's repositories start at that owner's prefix instead of paging
// through everything.
func (r *RegistryClient) Catalog(ctx context.Context, last string, n int) ([]string, error) {
	q := url.Values{}
	if n > 0 {
		q.Set("n", strconv.Itoa(n))
	}
	if last != "" {
		q.Set("last", last)
	}
	path := "/v2/_catalog"
	if len(q) > 0 {
		path += "?" + q.Encode()
	}

	resp, err := r.do(ctx, path)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var out struct {
		Repositories []string `json:"repositories"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("decoding the registry catalogue: %w", err)
	}
	return out.Repositories, nil
}

// RepositoriesForOwner returns the registry repositories belonging to owner.
func (r *RegistryClient) RepositoriesForOwner(ctx context.Context, owner string, limit int) ([]string, error) {
	const pageSize = 100
	prefix := owner + "/"

	// Start just before the owner's block rather than at the beginning of a
	// catalogue that holds every repository on the platform.
	last := owner
	var found []string

	for page := 0; page < 100; page++ {
		names, err := r.Catalog(ctx, last, pageSize)
		if err != nil {
			return nil, err
		}
		if len(names) == 0 {
			break
		}
		for _, name := range names {
			switch {
			case strings.HasPrefix(name, prefix):
				found = append(found, name)
				if limit > 0 && len(found) >= limit {
					return found[:limit], nil
				}
			case name > prefix && !strings.HasPrefix(name, prefix):
				// Past the owner's block: the catalogue is sorted, so
				// nothing further can match.
				return found, nil
			}
		}
		last = names[len(names)-1]
	}
	return found, nil
}

// Tags lists the tags of one repository, e.g. "veryviolet/gv".
func (r *RegistryClient) Tags(ctx context.Context, repository string) ([]string, error) {
	resp, err := r.do(ctx, "/v2/"+escapeFilePath(repository)+"/tags/list")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var out struct {
		Name string   `json:"name"`
		Tags []string `json:"tags"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("decoding the tag list of %s: %w", repository, err)
	}
	return out.Tags, nil
}

// Ping reports whether the registry accepts the credentials.
func (r *RegistryClient) Ping(ctx context.Context) error {
	resp, err := r.do(ctx, "/v2/")
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}
