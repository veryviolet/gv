package api

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// The package registries (deb, rpm, generic and the rest) are not part of the
// REST API: they live under https://<host>/api/packages, take HTTP basic auth
// with the personal access token, and expect the file as a plain PUT body.
//
// The body has to go up as an opaque stream. Sending it as form data makes the
// server fail with 500 rather than a useful message, which is worth knowing
// when this code is changed.

// PackageRegistryClient publishes files to the GitVerse package registries.
type PackageRegistryClient struct {
	http     *http.Client
	baseURL  string
	username string
	token    string
	ua       string
}

// NewPackageRegistryClient builds a client for the package registries of host.
func NewPackageRegistryClient(host, username, token, userAgent string) *PackageRegistryClient {
	if username == "" {
		username = "x-access-token"
	}
	return &PackageRegistryClient{
		http:     &http.Client{Timeout: 10 * time.Minute},
		baseURL:  PackageRegistryEndpoint(host),
		username: username,
		token:    token,
		ua:       userAgent,
	}
}

// PackageRegistryEndpoint maps a host to the root of its package registries.
func PackageRegistryEndpoint(host string) string {
	if v := os.Getenv("GV_PACKAGES_URL"); v != "" {
		return strings.TrimSuffix(v, "/")
	}
	host = strings.TrimSuffix(strings.TrimPrefix(strings.TrimPrefix(host, "https://"), "http://"), "/")
	if host == "" {
		host = "gitverse.ru"
	}
	return "https://" + host + "/api/packages"
}

// upload PUTs a file to a path under the registry root.
func (p *PackageRegistryClient) upload(ctx context.Context, path, file string) error {
	f, err := os.Open(file)
	if err != nil {
		return err
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, p.baseURL+path, f)
	if err != nil {
		return err
	}
	req.ContentLength = info.Size()
	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set("User-Agent", p.ua)
	req.SetBasicAuth(p.username, p.token)

	resp, err := p.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return newHTTPError(resp, body)
	}
	return nil
}

// UploadDebian publishes a .deb into a distribution and component, e.g.
// "stable" and "main".
func (p *PackageRegistryClient) UploadDebian(ctx context.Context, owner, distribution, component, file string) error {
	path := fmt.Sprintf("/%s/debian/pool/%s/%s/upload",
		url.PathEscape(owner), url.PathEscape(distribution), url.PathEscape(component))
	return p.upload(ctx, path, file)
}

// UploadRPM publishes a .rpm. An empty group uses the owner-wide repository.
func (p *PackageRegistryClient) UploadRPM(ctx context.Context, owner, group, file string) error {
	path := fmt.Sprintf("/%s/rpm", url.PathEscape(owner))
	if group != "" {
		path += "/" + escapeFilePath(group)
	}
	return p.upload(ctx, path+"/upload", file)
}

// UploadGeneric publishes an arbitrary file under a name and version.
func (p *PackageRegistryClient) UploadGeneric(ctx context.Context, owner, name, version, file string) error {
	path := fmt.Sprintf("/%s/generic/%s/%s/%s",
		url.PathEscape(owner), url.PathEscape(name), url.PathEscape(version),
		url.PathEscape(filepath.Base(file)))
	return p.upload(ctx, path, file)
}

// DebianRepositoryKey returns the URL of the signing key clients must trust.
func DebianRepositoryKey(host, owner string) string {
	return fmt.Sprintf("%s/%s/debian/repository.key", PackageRegistryEndpoint(host), url.PathEscape(owner))
}

// DebianSourceLine renders the line to put in sources.list.
func DebianSourceLine(host, owner, distribution, component, keyring string) string {
	return fmt.Sprintf("deb [signed-by=%s] %s/%s/debian %s %s",
		keyring, PackageRegistryEndpoint(host), url.PathEscape(owner), distribution, component)
}

// RPMRepoFile returns the URL of the server-generated .repo file.
func RPMRepoFile(host, owner, group string) string {
	base := fmt.Sprintf("%s/%s/rpm", PackageRegistryEndpoint(host), url.PathEscape(owner))
	if group != "" {
		base += "/" + escapeFilePath(group)
	}
	return base + ".repo"
}

// PackageKindForFile guesses the registry from a file name.
func PackageKindForFile(file string) string {
	switch strings.ToLower(filepath.Ext(file)) {
	case ".deb":
		return "deb"
	case ".rpm":
		return "rpm"
	default:
		return "generic"
	}
}
