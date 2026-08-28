package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"

	"gitverse.ru/veryviolet/gv/internal/gvrepo"
)

// ListReleases lists releases, newest first as returned by the API.
func ListReleases(ctx context.Context, c *Client, repo gvrepo.Interface, limit int) ([]Release, error) {
	return ListAll[Release](ctx, c, repoPath(repo, "releases"), limit)
}

// GetRelease fetches a release by its numeric id.
func GetRelease(ctx context.Context, c *Client, repo gvrepo.Interface, id int64) (*Release, error) {
	var r Release
	if err := c.REST(ctx, http.MethodGet, repoPath(repo, "releases/%d", id), nil, &r); err != nil {
		return nil, err
	}
	return &r, nil
}

// GetReleaseByTag fetches the release attached to a tag.
func GetReleaseByTag(ctx context.Context, c *Client, repo gvrepo.Interface, tag string) (*Release, error) {
	var r Release
	path := repoPath(repo, "releases/tags/%s", url.PathEscape(tag))
	if err := c.REST(ctx, http.MethodGet, path, nil, &r); err != nil {
		return nil, err
	}
	return &r, nil
}

// CreateRelease publishes a release for a tag.
func CreateRelease(ctx context.Context, c *Client, repo gvrepo.Interface, in CreateReleaseOptionRequest) (*Release, error) {
	var r Release
	if err := c.RESTWithBody(ctx, http.MethodPost, repoPath(repo, "releases"), in, &r); err != nil {
		return nil, err
	}
	return &r, nil
}

// EditRelease updates an existing release.
func EditRelease(ctx context.Context, c *Client, repo gvrepo.Interface, id int64, in EditReleaseOptionRequest) (*Release, error) {
	var r Release
	if err := c.RESTWithBody(ctx, http.MethodPatch, repoPath(repo, "releases/%d", id), in, &r); err != nil {
		return nil, err
	}
	return &r, nil
}

// DeleteRelease removes a release by id.
func DeleteRelease(ctx context.Context, c *Client, repo gvrepo.Interface, id int64) error {
	return c.REST(ctx, http.MethodDelete, repoPath(repo, "releases/%d", id), nil, nil)
}

// DeleteReleaseByTag removes the release attached to a tag.
func DeleteReleaseByTag(ctx context.Context, c *Client, repo gvrepo.Interface, tag string) error {
	path := repoPath(repo, "releases/tags/%s", url.PathEscape(tag))
	return c.REST(ctx, http.MethodDelete, path, nil, nil)
}

// ListReleaseAssets lists the files attached to a release.
func ListReleaseAssets(ctx context.Context, c *Client, repo gvrepo.Interface, id int64) ([]Attachment, error) {
	var assets []Attachment
	path := repoPath(repo, "releases/%d/assets", id)
	if err := c.REST(ctx, http.MethodGet, path, nil, &assets); err != nil {
		return nil, err
	}
	return assets, nil
}

// DeleteReleaseAsset removes one attached file.
func DeleteReleaseAsset(ctx context.Context, c *Client, repo gvrepo.Interface, releaseID, assetID int64) error {
	path := repoPath(repo, "releases/%d/assets/%d", releaseID, assetID)
	return c.REST(ctx, http.MethodDelete, path, nil, nil)
}

// UploadReleaseAsset attaches a local file to a release.
//
// The endpoint takes multipart/form-data with the file in the "attachment"
// field and the visible file name in the query string.
func UploadReleaseAsset(ctx context.Context, c *Client, repo gvrepo.Interface, releaseID int64, path, name string) (*Attachment, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	if name == "" {
		name = filepath.Base(path)
	}

	// The whole file is buffered because a retry has to replay the body, and
	// release artefacts are small enough for that to be reasonable.
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	part, err := w.CreateFormFile("attachment", name)
	if err != nil {
		return nil, err
	}
	if _, err := io.Copy(part, file); err != nil {
		return nil, err
	}
	if err := w.Close(); err != nil {
		return nil, err
	}

	uploadPath := withQuery(
		repoPath(repo, "releases/%d/assets", releaseID),
		url.Values{"name": []string{name}},
	)
	req, err := c.NewUploadRequest(ctx, http.MethodPost, uploadPath, w.FormDataContentType(), &buf)
	if err != nil {
		return nil, err
	}

	resp, err := c.Do(req)
	if resp != nil {
		defer resp.Body.Close()
	}
	if err != nil {
		return nil, err
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return &Attachment{Name: name}, nil
	}
	var attachment Attachment
	if err := json.Unmarshal(data, &attachment); err != nil {
		return nil, fmt.Errorf("decoding the upload response for %s: %w", name, err)
	}
	return &attachment, nil
}

// ParseReleaseID accepts a numeric release id.
func ParseReleaseID(s string) (int64, error) {
	id, err := strconv.ParseInt(s, 10, 64)
	if err != nil || id <= 0 {
		return 0, fmt.Errorf("invalid release id %q", s)
	}
	return id, nil
}
