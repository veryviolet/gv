package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// HTTPError is returned for any non-2xx response. The raw body is kept so that
// `gv api` can print exactly what the server said: GitVerse does not document
// its error envelope, so we parse defensively and never discard the original.
type HTTPError struct {
	StatusCode int
	Status     string
	RequestURL string
	Message    string
	Errors     []string
	Body       []byte
}

func (e *HTTPError) Error() string {
	var b strings.Builder
	fmt.Fprintf(&b, "HTTP %d", e.StatusCode)
	if e.RequestURL != "" {
		fmt.Fprintf(&b, " (%s)", e.RequestURL)
	}
	if e.Message != "" {
		fmt.Fprintf(&b, ": %s", e.Message)
	}
	for _, m := range e.Errors {
		fmt.Fprintf(&b, "\n  %s", m)
	}
	return b.String()
}

// IsNotFound reports whether err is a 404 from the API.
func IsNotFound(err error) bool {
	he, ok := err.(*HTTPError)
	return ok && he.StatusCode == http.StatusNotFound
}

// IsUnauthorized reports whether err is a 401/403 from the API.
func IsUnauthorized(err error) bool {
	he, ok := err.(*HTTPError)
	return ok && (he.StatusCode == http.StatusUnauthorized || he.StatusCode == http.StatusForbidden)
}

// newHTTPError builds an HTTPError, extracting a human-readable message from
// whichever of the common JSON error shapes the server happened to use.
func newHTTPError(resp *http.Response, body []byte) *HTTPError {
	e := &HTTPError{
		StatusCode: resp.StatusCode,
		Status:     resp.Status,
		Body:       body,
	}
	if resp.Request != nil && resp.Request.URL != nil {
		e.RequestURL = resp.Request.URL.String()
	}

	// GitVerse wraps validation failures in {"error": {...}}, while other
	// responses put the text at the top level; accept both shapes.
	var nested struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
			Status  int    `json:"status"`
		} `json:"error"`
	}
	if json.Unmarshal(body, &nested) == nil && nested.Error.Message != "" {
		e.Message = nested.Error.Message
		if nested.Error.Code != "" {
			e.Message = nested.Error.Code + ": " + e.Message
		}
		return e
	}

	var envelope struct {
		Message string `json:"message"`
		Error   string `json:"error"`
		Detail  string `json:"detail"`
		Errors  []struct {
			Message  string `json:"message"`
			Field    string `json:"field"`
			Resource string `json:"resource"`
			Code     string `json:"code"`
		} `json:"errors"`
	}
	if json.Unmarshal(body, &envelope) == nil {
		switch {
		case envelope.Message != "":
			e.Message = envelope.Message
		case envelope.Error != "":
			e.Message = envelope.Error
		case envelope.Detail != "":
			e.Message = envelope.Detail
		}
		for _, item := range envelope.Errors {
			switch {
			case item.Message != "":
				e.Errors = append(e.Errors, item.Message)
			case item.Field != "" && item.Code != "":
				e.Errors = append(e.Errors, fmt.Sprintf("%s.%s: %s", item.Resource, item.Field, item.Code))
			}
		}
	}

	if e.Message == "" {
		// Not JSON, or JSON without a recognised field: fall back to the body
		// itself when it is short enough to be useful on one line.
		text := strings.TrimSpace(string(body))
		if text != "" && len(text) <= 200 && !strings.HasPrefix(text, "{") && !strings.HasPrefix(text, "<") {
			e.Message = text
		}
	}
	return e
}
