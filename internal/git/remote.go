package git

import (
	"context"
	"net/url"
	"regexp"
	"strings"
)

// Remote is a git remote with its parsed fetch and push URLs.
type Remote struct {
	Name     string
	FetchURL *url.URL
	PushURL  *url.URL
}

func (r *Remote) String() string { return r.Name }

// RemoteSet is an ordered collection of remotes.
type RemoteSet []*Remote

// FindByName returns the first remote matching any of names, which are tried in
// order. The name "*" matches anything.
func (rs RemoteSet) FindByName(names ...string) (*Remote, bool) {
	for _, want := range names {
		for _, r := range rs {
			if r.Name == want || want == "*" {
				return r, true
			}
		}
	}
	return nil, false
}

var remoteRE = regexp.MustCompile(`(.+)\s+(.+)\s+\((push|fetch)\)`)

// Remotes lists the repository's remotes, ordered with the conventional
// upstream/github/origin first so callers can pick a base repository the way
// gh does.
func (c *Client) Remotes(ctx context.Context) (RemoteSet, error) {
	out, err := c.Output(ctx, "remote", "-v")
	if err != nil {
		return nil, err
	}
	return parseRemotes(strings.Split(out, "\n")), nil
}

func parseRemotes(lines []string) RemoteSet {
	byName := map[string]*Remote{}
	var order []string

	for _, line := range lines {
		m := remoteRE.FindStringSubmatch(strings.TrimSpace(line))
		if m == nil {
			continue
		}
		name, rawURL, kind := strings.TrimSpace(m[1]), strings.TrimSpace(m[2]), m[3]
		rem, ok := byName[name]
		if !ok {
			rem = &Remote{Name: name}
			byName[name] = rem
			order = append(order, name)
		}
		u, err := ParseURL(rawURL)
		if err != nil {
			continue
		}
		if kind == "fetch" {
			rem.FetchURL = u
		} else {
			rem.PushURL = u
		}
	}

	remotes := make(RemoteSet, 0, len(order))
	for _, name := range order {
		remotes = append(remotes, byName[name])
	}
	sortRemotes(remotes)
	return remotes
}

// sortRemotes puts "upstream", then "gitverse", then "origin", then the rest,
// preserving the original order within each group.
func sortRemotes(remotes RemoteSet) {
	rank := func(name string) int {
		switch name {
		case "upstream":
			return 0
		case "gitverse":
			return 1
		case "origin":
			return 2
		default:
			return 3
		}
	}
	// Insertion sort keeps it stable without pulling in sort.SliceStable for
	// what is always a handful of entries.
	for i := 1; i < len(remotes); i++ {
		for j := i; j > 0 && rank(remotes[j].Name) < rank(remotes[j-1].Name); j-- {
			remotes[j], remotes[j-1] = remotes[j-1], remotes[j]
		}
	}
}

// ParseURL normalises git's URL forms, including scp-like syntax, into a *url.URL.
func ParseURL(rawURL string) (*url.URL, error) {
	if !strings.Contains(rawURL, "://") && strings.Contains(rawURL, ":") && !strings.HasPrefix(rawURL, "/") {
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
