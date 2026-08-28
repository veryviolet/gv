package api

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"gitverse.ru/veryviolet/gv/internal/gvrepo"
)

// Ptr returns a pointer to v, for filling the pointer fields of request types.
func Ptr[T any](v T) *T { return &v }

// DisplayName returns the friendliest identifier available for a user.
func (u *User) DisplayName() string {
	switch {
	case u.Login != "":
		return u.Login
	case u.Name != "":
		return u.Name
	case u.FullName != "":
		return u.FullName
	default:
		return ""
	}
}

// IsPullRequest reports whether an entry from the issues endpoint is really a
// pull request: GitVerse, like GitHub, returns both from the same collection.
func (i *Issue) IsPullRequest() bool {
	return i.PullRequest.URL != "" || i.PullRequest.HTMLURL != ""
}

// repoPath builds "/repos/{owner}/{repo}/..." with each segment escaped.
func repoPath(repo gvrepo.Interface, format string, args ...any) string {
	base := fmt.Sprintf("/repos/%s/%s",
		url.PathEscape(repo.RepoOwner()),
		url.PathEscape(repo.RepoName()))
	if format == "" {
		return base
	}
	return base + "/" + strings.TrimPrefix(fmt.Sprintf(format, args...), "/")
}

func withQuery(path string, q url.Values) string {
	if len(q) == 0 {
		return path
	}
	sep := "?"
	if strings.Contains(path, "?") {
		sep = "&"
	}
	return path + sep + q.Encode()
}

// ---------------------------------------------------------------- users

// CurrentUser fetches the account the token belongs to.
func CurrentUser(ctx context.Context, c *Client) (*User, error) {
	var user User
	if err := c.REST(ctx, http.MethodGet, "/user", nil, &user); err != nil {
		return nil, err
	}
	return &user, nil
}

// GetUser fetches a user by login.
func GetUser(ctx context.Context, c *Client, username string) (*User, error) {
	var user User
	path := "/users/" + url.PathEscape(username)
	if err := c.REST(ctx, http.MethodGet, path, nil, &user); err != nil {
		return nil, err
	}
	return &user, nil
}

// ---------------------------------------------------------------- repositories

// GetRepository fetches repository details.
func GetRepository(ctx context.Context, c *Client, repo gvrepo.Interface) (*Repository, error) {
	var r Repository
	if err := c.REST(ctx, http.MethodGet, repoPath(repo, ""), nil, &r); err != nil {
		return nil, err
	}
	return &r, nil
}

// RepositoryLanguages returns the byte counts per language.
func RepositoryLanguages(ctx context.Context, c *Client, repo gvrepo.Interface) (map[string]int64, error) {
	langs := map[string]int64{}
	if err := c.REST(ctx, http.MethodGet, repoPath(repo, "languages"), nil, &langs); err != nil {
		return nil, err
	}
	return langs, nil
}

// ListMyRepositories lists repositories belonging to the authenticated user.
func ListMyRepositories(ctx context.Context, c *Client, limit int) ([]Repository, error) {
	return ListAll[Repository](ctx, c, "/user/repos", limit)
}

// ListUserRepositories lists the public repositories of a user.
func ListUserRepositories(ctx context.Context, c *Client, username string, limit int) ([]Repository, error) {
	return ListAll[Repository](ctx, c, "/users/"+url.PathEscape(username)+"/repos", limit)
}

// ListOrgRepositories lists the repositories of an organization.
func ListOrgRepositories(ctx context.Context, c *Client, org string, limit int) ([]Repository, error) {
	return ListAll[Repository](ctx, c, "/orgs/"+url.PathEscape(org)+"/repos", limit)
}

// CreateRepository creates a repository owned by the authenticated user.
func CreateRepository(ctx context.Context, c *Client, in CreateRepoRequest) (*Repository, error) {
	var r Repository
	if err := c.RESTWithBody(ctx, http.MethodPost, "/user/repos", in, &r); err != nil {
		return nil, err
	}
	return &r, nil
}

// CreateOrgRepository creates a repository owned by an organization.
func CreateOrgRepository(ctx context.Context, c *Client, org string, in CreateOrgRepoOptionRequest) (*Repository, error) {
	var r Repository
	path := "/orgs/" + url.PathEscape(org) + "/repos"
	if err := c.RESTWithBody(ctx, http.MethodPost, path, in, &r); err != nil {
		return nil, err
	}
	return &r, nil
}

// ForkRepository forks a repository, optionally into an organization.
func ForkRepository(ctx context.Context, c *Client, repo gvrepo.Interface, in ForkRepoRequest) (*Repository, error) {
	var r Repository
	if err := c.RESTWithBody(ctx, http.MethodPost, repoPath(repo, "forks"), in, &r); err != nil {
		return nil, err
	}
	return &r, nil
}

// DeleteRepository deletes a repository.
func DeleteRepository(ctx context.Context, c *Client, repo gvrepo.Interface) error {
	return c.REST(ctx, http.MethodDelete, repoPath(repo, ""), nil, nil)
}

// UpdateRepository edits repository settings.
func UpdateRepository(ctx context.Context, c *Client, repo gvrepo.Interface, in RepositoryUpdateDTORequest) (*Repository, error) {
	var r Repository
	if err := c.RESTWithBody(ctx, http.MethodPatch, repoPath(repo, ""), in, &r); err != nil {
		return nil, err
	}
	return &r, nil
}

// GetContents fetches a file or directory listing. ref is a branch, tag or
// commit; an empty ref means the default branch.
func GetContents(ctx context.Context, c *Client, repo gvrepo.Interface, filepath, ref string) (*ContentsResponse, error) {
	path := repoPath(repo, "contents/%s", escapeFilePath(filepath))
	if ref != "" {
		path = withQuery(path, url.Values{"ref": []string{ref}})
	}
	var out ContentsResponse
	if err := c.REST(ctx, http.MethodGet, path, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// DecodedContent returns the file body, decoding base64 when the API used it.
func (c *ContentsResponse) DecodedContent() (string, error) {
	if c.Encoding != "base64" {
		return c.Content, nil
	}
	data, err := base64.StdEncoding.DecodeString(c.Content)
	if err != nil {
		return "", fmt.Errorf("decoding %s: %w", c.Path, err)
	}
	return string(data), nil
}

// escapeFilePath escapes each segment while keeping the slashes intact.
func escapeFilePath(p string) string {
	parts := strings.Split(strings.TrimPrefix(p, "/"), "/")
	for i, seg := range parts {
		parts[i] = url.PathEscape(seg)
	}
	return strings.Join(parts, "/")
}

// ListBranches lists repository branches.
func ListBranches(ctx context.Context, c *Client, repo gvrepo.Interface, limit int) ([]Branch, error) {
	return ListAll[Branch](ctx, c, repoPath(repo, "branches"), limit)
}

// ---------------------------------------------------------------- pull requests

// PullRequestListOptions filters GET /repos/{owner}/{repo}/pulls.
type PullRequestListOptions struct {
	State     string
	Head      string
	Base      string
	Sort      string
	Direction string
	Limit     int
}

func (o PullRequestListOptions) values() url.Values {
	q := url.Values{}
	set := func(k, v string) {
		if v != "" {
			q.Set(k, v)
		}
	}
	set("state", o.State)
	set("head", o.Head)
	set("base", o.Base)
	set("sort", o.Sort)
	set("direction", o.Direction)
	return q
}

// ListPullRequests lists pull requests in a repository.
func ListPullRequests(ctx context.Context, c *Client, repo gvrepo.Interface, opts PullRequestListOptions) ([]PullRequest, error) {
	path := withQuery(repoPath(repo, "pulls"), opts.values())
	return ListAll[PullRequest](ctx, c, path, opts.Limit)
}

// GetPullRequest fetches a single pull request.
func GetPullRequest(ctx context.Context, c *Client, repo gvrepo.Interface, number int) (*PullRequest, error) {
	var pr PullRequest
	path := repoPath(repo, "pulls/%d", number)
	if err := c.REST(ctx, http.MethodGet, path, nil, &pr); err != nil {
		return nil, err
	}
	return &pr, nil
}

// CreatePullRequest opens a pull request.
func CreatePullRequest(ctx context.Context, c *Client, repo gvrepo.Interface, in CreatePullRequestAPIRequest) (*PullRequest, error) {
	var pr PullRequest
	if err := c.RESTWithBody(ctx, http.MethodPost, repoPath(repo, "pulls"), in, &pr); err != nil {
		return nil, err
	}
	return &pr, nil
}

// UpdatePullRequest edits a pull request, including its open/closed state.
func UpdatePullRequest(ctx context.Context, c *Client, repo gvrepo.Interface, number int, in UpdatePullRequestAPIRequest) (*PullRequest, error) {
	var pr PullRequest
	path := repoPath(repo, "pulls/%d", number)
	if err := c.RESTWithBody(ctx, http.MethodPatch, path, in, &pr); err != nil {
		return nil, err
	}
	return &pr, nil
}

// ListPullRequestFiles lists the files a pull request touches, with patches.
func ListPullRequestFiles(ctx context.Context, c *Client, repo gvrepo.Interface, number, limit int) ([]CommitFiles, error) {
	return ListAll[CommitFiles](ctx, c, repoPath(repo, "pulls/%d/files", number), limit)
}

// ListPullRequestCommits lists the commits of a pull request.
func ListPullRequestCommits(ctx context.Context, c *Client, repo gvrepo.Interface, number, limit int) ([]CommitResponse, error) {
	return ListAll[CommitResponse](ctx, c, repoPath(repo, "pulls/%d/commits", number), limit)
}

// ListCommits lists commits on a branch, tag or commit range.
func ListCommits(ctx context.Context, c *Client, repo gvrepo.Interface, sha string, limit int) ([]CommitResponse, error) {
	path := repoPath(repo, "commits")
	if sha != "" {
		path = withQuery(path, url.Values{"sha": []string{sha}})
	}
	return ListAll[CommitResponse](ctx, c, path, limit)
}

// PullRequestMerged reports whether a pull request has been merged. The API
// answers with 204 when merged and 404 when not.
func PullRequestMerged(ctx context.Context, c *Client, repo gvrepo.Interface, number int) (bool, error) {
	path := repoPath(repo, "pulls/%d/merge", number)
	err := c.REST(ctx, http.MethodGet, path, nil, nil)
	if err == nil {
		return true, nil
	}
	if IsNotFound(err) {
		return false, nil
	}
	return false, err
}

// UpdatePullRequestBranch merges the base branch into the pull request branch.
func UpdatePullRequestBranch(ctx context.Context, c *Client, repo gvrepo.Interface, number int) error {
	path := repoPath(repo, "pulls/%d/update-branch", number)
	return c.REST(ctx, http.MethodPut, path, nil, nil)
}

// ---------------------------------------------------------------- issues

// IssueListOptions filters GET /repos/{owner}/{repo}/issues.
type IssueListOptions struct {
	State      string
	Query      string
	Labels     string
	Milestones string
	CreatedBy  string
	AssignedBy string
	Mentioned  string
	Type       string
	Since      string
	Before     string
	Limit      int
}

func (o IssueListOptions) values() url.Values {
	q := url.Values{}
	set := func(k, v string) {
		if v != "" {
			q.Set(k, v)
		}
	}
	set("state", o.State)
	set("q", o.Query)
	set("labels", o.Labels)
	set("milestones", o.Milestones)
	set("created_by", o.CreatedBy)
	set("assigned_by", o.AssignedBy)
	set("mentioned_by", o.Mentioned)
	set("type", o.Type)
	set("since", o.Since)
	set("before", o.Before)
	return q
}

// ListIssues lists issues and pull requests. Callers that want only issues
// should drop the entries for which Issue.IsPullRequest reports true.
func ListIssues(ctx context.Context, c *Client, repo gvrepo.Interface, opts IssueListOptions) ([]Issue, error) {
	path := withQuery(repoPath(repo, "issues"), opts.values())
	return ListAll[Issue](ctx, c, path, opts.Limit)
}

// GetIssue fetches a single issue or pull request by its number.
func GetIssue(ctx context.Context, c *Client, repo gvrepo.Interface, number int) (*Issue, error) {
	var issue Issue
	path := repoPath(repo, "issues/%d", number)
	if err := c.REST(ctx, http.MethodGet, path, nil, &issue); err != nil {
		return nil, err
	}
	return &issue, nil
}

// CreateIssue opens an issue.
func CreateIssue(ctx context.Context, c *Client, repo gvrepo.Interface, in CreateIssueRequest) (*Issue, error) {
	var issue Issue
	if err := c.RESTWithBody(ctx, http.MethodPost, repoPath(repo, "issues"), in, &issue); err != nil {
		return nil, err
	}
	return &issue, nil
}

// UpdateIssue edits an issue, including its open/closed state.
func UpdateIssue(ctx context.Context, c *Client, repo gvrepo.Interface, number int, in UpdateIssueRequest) (*Issue, error) {
	var issue Issue
	path := repoPath(repo, "issues/%d", number)
	if err := c.RESTWithBody(ctx, http.MethodPatch, path, in, &issue); err != nil {
		return nil, err
	}
	return &issue, nil
}

// ListIssueComments lists the comments on an issue or pull request.
func ListIssueComments(ctx context.Context, c *Client, repo gvrepo.Interface, number, limit int) ([]Comment, error) {
	return ListAll[Comment](ctx, c, repoPath(repo, "issues/%d/comments", number), limit)
}

// CreateIssueComment comments on an issue or pull request.
func CreateIssueComment(ctx context.Context, c *Client, repo gvrepo.Interface, number int, in CreateIssueCommentRequest) (*Comment, error) {
	var comment Comment
	path := repoPath(repo, "issues/%d/comments", number)
	if err := c.RESTWithBody(ctx, http.MethodPost, path, in, &comment); err != nil {
		return nil, err
	}
	return &comment, nil
}

// ParseNumber accepts "123" or "#123" and returns the numeric part.
func ParseNumber(s string) (int, error) {
	s = strings.TrimPrefix(strings.TrimSpace(s), "#")
	n, err := strconv.Atoi(s)
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("invalid number %q", s)
	}
	return n, nil
}
