package api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
)

// DefaultPerPage is the page size gv asks for when paginating.
const DefaultPerPage = 30

// maxPages is a safety stop so a server that never signals the end of a
// collection cannot spin forever.
const maxPages = 1000

var linkRE = regexp.MustCompile(`<([^>]+)>;\s*rel="([^"]+)"`)

// ParseLinkHeader extracts the rel -> URL pairs from an RFC 5988 Link header.
func ParseLinkHeader(header string) map[string]string {
	out := map[string]string{}
	for _, m := range linkRE.FindAllStringSubmatch(header, -1) {
		out[m[2]] = m[1]
	}
	return out
}

// nextPageURL decides where the next page lives.
//
// GitVerse documents page/per_page query parameters but not a Link header, so
// the header is used when present and otherwise the page number is incremented
// for as long as the server keeps returning full pages.
func nextPageURL(resp *http.Response, requestURL string, items, perPage int) string {
	if next, ok := ParseLinkHeader(resp.Header.Get("Link"))["next"]; ok && next != "" {
		return next
	}
	if perPage <= 0 || items < perPage {
		return ""
	}
	u, err := url.Parse(requestURL)
	if err != nil {
		return ""
	}
	q := u.Query()
	page, _ := strconv.Atoi(q.Get("page"))
	if page == 0 {
		page = 1
	}
	q.Set("page", strconv.Itoa(page+1))
	q.Set("per_page", strconv.Itoa(perPage))
	u.RawQuery = q.Encode()
	return u.String()
}

// countJSONItems reports how many elements a JSON array holds. A non-array
// payload counts as zero, which stops pagination.
func countJSONItems(data []byte) int {
	var arr []json.RawMessage
	if err := json.Unmarshal(data, &arr); err != nil {
		return 0
	}
	return len(arr)
}

// PageFunc receives the raw body of each page. Returning false stops paginating.
type PageFunc func(body []byte, resp *http.Response) (bool, error)

// Paginate walks a collection endpoint, invoking fn once per page.
//
// limit caps the total number of items fetched; zero means no cap.
func (c *Client) Paginate(ctx context.Context, method, path string, limit int, fn PageFunc) error {
	perPage := DefaultPerPage
	next, err := c.ResolveURL(path)
	if err != nil {
		return err
	}
	if u, err := url.Parse(next); err == nil {
		if v := u.Query().Get("per_page"); v != "" {
			if n, err := strconv.Atoi(v); err == nil && n > 0 {
				perPage = n
			}
		} else if limit > 0 && limit < DefaultPerPage {
			q := u.Query()
			perPage = limit
			q.Set("per_page", strconv.Itoa(perPage))
			u.RawQuery = q.Encode()
			next = u.String()
		}
	}

	fetched := 0
	for page := 0; next != "" && page < maxPages; page++ {
		req, err := c.NewRequest(ctx, method, next, nil)
		if err != nil {
			return err
		}
		resp, err := c.Do(req)
		if err != nil {
			if resp != nil {
				resp.Body.Close()
			}
			return err
		}
		body, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			return readErr
		}

		items := countJSONItems(body)
		fetched += items

		keepGoing, err := fn(body, resp)
		if err != nil {
			return err
		}
		if !keepGoing {
			return nil
		}
		if limit > 0 && fetched >= limit {
			return nil
		}
		next = nextPageURL(resp, req.URL.String(), items, perPage)
	}
	return nil
}

// ListAll collects every page of a JSON array endpoint into a single slice.
// limit caps the result; zero means no cap.
func ListAll[T any](ctx context.Context, c *Client, path string, limit int) ([]T, error) {
	var out []T
	err := c.Paginate(ctx, http.MethodGet, path, limit, func(body []byte, _ *http.Response) (bool, error) {
		var page []T
		if err := json.Unmarshal(body, &page); err != nil {
			return false, err
		}
		out = append(out, page...)
		if limit > 0 && len(out) >= limit {
			out = out[:limit]
			return false, nil
		}
		return true, nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}
