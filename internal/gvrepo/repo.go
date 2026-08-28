// Package gvrepo identifies a GitVerse repository: owner, name and host.
package gvrepo

import (
	"errors"
	"fmt"
	"net/url"
	"strings"

	"gitverse.ru/veryviolet/gv/internal/config"
)

// Interface is the minimal description of a repository that commands need.
type Interface interface {
	RepoOwner() string
	RepoName() string
	RepoHost() string
}

type repo struct {
	owner string
	name  string
	host  string
}

func (r repo) RepoOwner() string { return r.owner }
func (r repo) RepoName() string  { return r.name }
func (r repo) RepoHost() string  { return r.host }

// New builds a repository reference on the default host.
func New(owner, name string) Interface {
	return NewWithHost(owner, name, config.DefaultHost)
}

// NewWithHost builds a repository reference on an explicit host.
func NewWithHost(owner, name, host string) Interface {
	return repo{owner: owner, name: name, host: normalizeHost(host)}
}

// FullName renders "owner/repo".
func FullName(r Interface) string {
	return r.RepoOwner() + "/" + r.RepoName()
}

// IsSame reports whether two references point at the same repository.
func IsSame(a, b Interface) bool {
	if a == nil || b == nil {
		return false
	}
	return strings.EqualFold(a.RepoOwner(), b.RepoOwner()) &&
		strings.EqualFold(a.RepoName(), b.RepoName()) &&
		strings.EqualFold(a.RepoHost(), b.RepoHost())
}

// FromFullName parses "OWNER/REPO", "HOST/OWNER/REPO" or a repository URL.
func FromFullName(nwo string) (Interface, error) {
	return FromFullNameWithHost(nwo, config.Host(""))
}

// FromFullNameWithHost is FromFullName with an explicit fallback host.
func FromFullNameWithHost(nwo, fallbackHost string) (Interface, error) {
	nwo = strings.TrimSpace(nwo)
	if nwo == "" {
		return nil, errors.New("expected the \"OWNER/REPO\" format")
	}

	if strings.Contains(nwo, "://") || strings.HasPrefix(nwo, "git@") {
		u, err := ParseURL(nwo)
		if err != nil {
			return nil, err
		}
		return FromURL(u)
	}

	parts := strings.Split(strings.Trim(nwo, "/"), "/")
	for _, p := range parts {
		if p == "" {
			return nil, fmt.Errorf("invalid repository %q: expected the \"OWNER/REPO\" format", nwo)
		}
	}
	switch len(parts) {
	case 3:
		return NewWithHost(parts[1], strings.TrimSuffix(parts[2], ".git"), parts[0]), nil
	case 2:
		return NewWithHost(parts[0], strings.TrimSuffix(parts[1], ".git"), fallbackHost), nil
	default:
		return nil, fmt.Errorf("invalid repository %q: expected the \"OWNER/REPO\" format", nwo)
	}
}

// FromURL derives a repository reference from a clone or browser URL.
func FromURL(u *url.URL) (Interface, error) {
	if u == nil {
		return nil, errors.New("nil URL")
	}
	parts := strings.SplitN(strings.Trim(u.Path, "/"), "/", 3)
	if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
		return nil, fmt.Errorf("invalid path: %s", u.Path)
	}
	return NewWithHost(parts[0], strings.TrimSuffix(parts[1], ".git"), u.Hostname()), nil
}

// ParseURL normalises the URL forms git understands, including scp-like syntax
// (git@gitverse.ru:owner/repo.git), into a *url.URL.
func ParseURL(rawURL string) (*url.URL, error) {
	if !strings.Contains(rawURL, "://") && strings.Contains(rawURL, ":") && !strings.HasPrefix(rawURL, "/") {
		// scp-like: [user@]host:path
		host, path, ok := strings.Cut(rawURL, ":")
		if ok && !strings.HasPrefix(path, "/") {
			if _, h, found := strings.Cut(host, "@"); found {
				host = h
			}
			rawURL = "ssh://" + host + "/" + path
		}
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}
	if u.Scheme == "git+ssh" {
		u.Scheme = "ssh"
	}
	return u, nil
}

// WebURL builds a browser URL for the repository, with optional extra path
// segments already escaped by the caller.
func WebURL(r Interface, path string) string {
	base := fmt.Sprintf("https://%s/%s/%s", r.RepoHost(), r.RepoOwner(), r.RepoName())
	if path == "" {
		return base
	}
	return base + "/" + strings.TrimPrefix(path, "/")
}

// CloneURL builds the clone URL for the given protocol ("https" or "ssh").
func CloneURL(r Interface, protocol string) string {
	if protocol == "ssh" {
		return fmt.Sprintf("git@%s:%s/%s.git", r.RepoHost(), r.RepoOwner(), r.RepoName())
	}
	return fmt.Sprintf("https://%s/%s/%s.git", r.RepoHost(), r.RepoOwner(), r.RepoName())
}

func normalizeHost(h string) string {
	h = strings.ToLower(strings.TrimSpace(h))
	if h == "" {
		return config.DefaultHost
	}
	return h
}
