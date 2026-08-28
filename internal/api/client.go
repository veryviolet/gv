// Package api is gv's HTTP layer for the GitVerse public API.
//
// Every request carries Bearer auth and the media-type header that pins the
// major API version:
//
//	Accept: application/vnd.gitverse.object+json;version=1
package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

// DefaultAPIVersion is the major API version gv speaks.
const DefaultAPIVersion = "1"

// acceptFormat is the media type GitVerse uses to select an API version.
const acceptFormat = "application/vnd.gitverse.object+json;version=%s"

// ClientOptions configures a Client.
type ClientOptions struct {
	// Host is the GitVerse instance, e.g. "gitverse.ru".
	Host string
	// Token is the bearer credential; may be empty for anonymous requests.
	Token string
	// APIVersion pins the major API version. Defaults to DefaultAPIVersion.
	APIVersion string
	// UserAgent identifies gv to the server.
	UserAgent string
	// Transport overrides the HTTP transport, mainly for tests.
	Transport http.RoundTripper
	// BaseURL overrides the derived API endpoint, mainly for tests.
	BaseURL string
	// Timeout bounds a single request. Zero means 30s.
	Timeout time.Duration
	// Debug, when non-nil, receives a log of requests and responses.
	Debug io.Writer
	// MaxRetries bounds retries of 429 and 5xx responses. Zero means 2.
	MaxRetries int
}

type Client struct {
	http       *http.Client
	baseURL    *url.URL
	token      string
	apiVersion string
	userAgent  string
	debug      io.Writer
	maxRetries int
}

// NewClient builds a Client from opts.
func NewClient(opts ClientOptions) (*Client, error) {
	base := opts.BaseURL
	if base == "" {
		base = APIEndpoint(opts.Host)
	}
	u, err := url.Parse(strings.TrimSuffix(base, "/") + "/")
	if err != nil {
		return nil, fmt.Errorf("invalid API base URL %q: %w", base, err)
	}

	timeout := opts.Timeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}
	version := opts.APIVersion
	if version == "" {
		version = DefaultAPIVersion
	}
	ua := opts.UserAgent
	if ua == "" {
		ua = "gv"
	}
	retries := opts.MaxRetries
	if retries == 0 {
		retries = 2
	}

	return &Client{
		http:       &http.Client{Transport: opts.Transport, Timeout: timeout},
		baseURL:    u,
		token:      opts.Token,
		apiVersion: version,
		userAgent:  ua,
		debug:      opts.Debug,
		maxRetries: retries,
	}, nil
}

// NewClientForHost builds a client for host, applying the environment
// overrides gv understands. Every caller that needs a client — the command
// factory as well as `gv auth`, which verifies a token before it is stored —
// goes through here, so that GV_API_URL and friends apply consistently.
func NewClientForHost(host, token, userAgent string) (*Client, error) {
	opts := ClientOptions{
		Host:      host,
		Token:     token,
		UserAgent: userAgent,
	}
	if v := os.Getenv("GV_API_URL"); v != "" {
		opts.BaseURL = v
	}
	if v := os.Getenv("GV_API_VERSION"); v != "" {
		opts.APIVersion = v
	}
	if v := os.Getenv("GV_DEBUG"); v != "" && v != "0" && v != "false" {
		opts.Debug = os.Stderr
	}
	return NewClient(opts)
}

// EndpointForHost reports the API base URL gv will actually use for host,
// taking the GV_API_URL override into account.
func EndpointForHost(host string) string {
	if v := os.Getenv("GV_API_URL"); v != "" {
		return strings.TrimSuffix(v, "/")
	}
	return APIEndpoint(host)
}

// APIEndpoint maps a GitVerse host to its API base URL. The public instance
// serves the API from a dedicated subdomain; anything else is assumed to expose
// it under /api on the same host.
func APIEndpoint(host string) string {
	host = strings.TrimSuffix(strings.TrimPrefix(strings.TrimPrefix(host, "https://"), "http://"), "/")
	switch {
	case host == "" || host == "gitverse.ru":
		return "https://api.gitverse.ru"
	case strings.HasPrefix(host, "api."):
		return "https://" + host
	default:
		return "https://" + host + "/api"
	}
}

// BaseURL reports the API root this client talks to.
func (c *Client) BaseURL() string {
	return strings.TrimSuffix(c.baseURL.String(), "/")
}

// HasToken reports whether the client is authenticated.
func (c *Client) HasToken() bool { return c.token != "" }

// ResolveURL turns an API path into an absolute URL. Absolute URLs pass through
// unchanged so that callers can follow links returned by the API.
func (c *Client) ResolveURL(p string) (string, error) {
	if strings.HasPrefix(p, "http://") || strings.HasPrefix(p, "https://") {
		return p, nil
	}
	ref, err := url.Parse(strings.TrimPrefix(p, "/"))
	if err != nil {
		return "", err
	}
	return c.baseURL.ResolveReference(ref).String(), nil
}

// NewRequest builds a request with gv's standard headers applied.
func (c *Client) NewRequest(ctx context.Context, method, path string, body io.Reader) (*http.Request, error) {
	u, err := c.ResolveURL(path)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, method, u, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", fmt.Sprintf(acceptFormat, c.apiVersion))
	req.Header.Set("User-Agent", c.userAgent)
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json; charset=utf-8")
	}
	return req, nil
}

// Do sends a request, retrying transient failures. Non-2xx responses are
// returned as *HTTPError with the body already drained.
func (c *Client) Do(req *http.Request) (*http.Response, error) {
	// Buffer the body so a retry can replay it.
	var payload []byte
	if req.Body != nil {
		var err error
		payload, err = io.ReadAll(req.Body)
		req.Body.Close()
		if err != nil {
			return nil, err
		}
	}

	for attempt := 0; ; attempt++ {
		if payload != nil {
			req.Body = io.NopCloser(bytes.NewReader(payload))
			req.ContentLength = int64(len(payload))
		}
		c.logRequest(req, payload)

		resp, err := c.http.Do(req)
		if err != nil {
			if attempt < c.maxRetries && isRetryableErr(err) {
				select {
				case <-time.After(backoff(attempt)):
					continue
				case <-req.Context().Done():
					return nil, req.Context().Err()
				}
			}
			return nil, err
		}

		if resp.StatusCode < 300 {
			c.logResponse(resp, nil)
			return resp, nil
		}

		body, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			return nil, readErr
		}
		c.logResponse(resp, body)

		if attempt < c.maxRetries && isRetryableStatus(resp.StatusCode) {
			wait := retryAfter(resp, attempt)
			select {
			case <-time.After(wait):
				continue
			case <-req.Context().Done():
				return nil, req.Context().Err()
			}
		}

		// Hand the body back on the response too, so callers such as
		// `gv api` can print it verbatim alongside the error.
		resp.Body = io.NopCloser(bytes.NewReader(body))
		return resp, newHTTPError(resp, body)
	}
}

// REST performs a request and decodes a JSON response into out. A nil out
// discards the body. A 204 leaves out untouched.
func (c *Client) REST(ctx context.Context, method, path string, body io.Reader, out any) error {
	req, err := c.NewRequest(ctx, method, path, body)
	if err != nil {
		return err
	}
	resp, err := c.Do(req)
	if resp != nil {
		defer resp.Body.Close()
	}
	if err != nil {
		return err
	}
	if out == nil || resp.StatusCode == http.StatusNoContent {
		_, _ = io.Copy(io.Discard, resp.Body)
		return nil
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return nil
	}
	if err := json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("decoding response from %s: %w", req.URL, err)
	}
	return nil
}

// RESTWithBody marshals in as JSON and decodes the response into out.
func (c *Client) RESTWithBody(ctx context.Context, method, path string, in, out any) error {
	var body io.Reader
	if in != nil {
		data, err := json.Marshal(in)
		if err != nil {
			return err
		}
		body = bytes.NewReader(data)
	}
	return c.REST(ctx, method, path, body, out)
}

func (c *Client) logRequest(req *http.Request, body []byte) {
	if c.debug == nil {
		return
	}
	fmt.Fprintf(c.debug, "* Request %s %s\n", req.Method, req.URL)
	for _, k := range []string{"Accept", "Content-Type", "User-Agent"} {
		if v := req.Header.Get(k); v != "" {
			fmt.Fprintf(c.debug, "> %s: %s\n", k, v)
		}
	}
	if req.Header.Get("Authorization") != "" {
		fmt.Fprintf(c.debug, "> Authorization: Bearer ***\n")
	}
	if len(body) > 0 {
		fmt.Fprintf(c.debug, "> %s\n", string(body))
	}
}

func (c *Client) logResponse(resp *http.Response, body []byte) {
	if c.debug == nil {
		return
	}
	fmt.Fprintf(c.debug, "< HTTP %d\n", resp.StatusCode)
	if len(body) > 0 {
		fmt.Fprintf(c.debug, "< %s\n", string(body))
	}
}

func isRetryableStatus(code int) bool {
	return code == http.StatusTooManyRequests ||
		code == http.StatusBadGateway ||
		code == http.StatusServiceUnavailable ||
		code == http.StatusGatewayTimeout
}

func isRetryableErr(err error) bool {
	// Timeouts and connection resets are worth one more try; anything the
	// caller cancelled is not.
	return !strings.Contains(err.Error(), "context canceled")
}

// retryAfter honours the Retry-After header, falling back to exponential backoff.
func retryAfter(resp *http.Response, attempt int) time.Duration {
	if v := resp.Header.Get("Retry-After"); v != "" {
		if secs, err := strconv.Atoi(strings.TrimSpace(v)); err == nil && secs >= 0 {
			d := time.Duration(secs) * time.Second
			if d > time.Minute {
				d = time.Minute
			}
			return d
		}
		if t, err := http.ParseTime(v); err == nil {
			if d := time.Until(t); d > 0 && d <= time.Minute {
				return d
			}
		}
	}
	return backoff(attempt)
}

func backoff(attempt int) time.Duration {
	d := time.Duration(1<<attempt) * time.Second
	if d > 8*time.Second {
		d = 8 * time.Second
	}
	return d
}
