// Package pr implements `gv pr`.
package pr

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"gitverse.ru/veryviolet/gv/internal/api"
	"gitverse.ru/veryviolet/gv/internal/cmdutil"
	"gitverse.ru/veryviolet/gv/internal/git"
	"gitverse.ru/veryviolet/gv/internal/gvrepo"
	"gitverse.ru/veryviolet/gv/internal/iostreams"
	"gitverse.ru/veryviolet/gv/internal/text"
)

// NewCmdPR returns the `gv pr` command group.
func NewCmdPR(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "pr <command>",
		Short:   "Work with pull requests",
		Aliases: []string{"pulls"},
		Long: `List, inspect and open pull requests.

Commands that take a pull request accept its number; inside a git checkout the
pull request for the current branch is found automatically.

Note: the GitVerse API exposes no endpoint for merging a pull request, so gv
has no "pr merge" command. Merge from the web interface.`,
	}

	cmd.PersistentFlags().StringP("repo", "R", "", "Select a repository using the `OWNER/REPO` format")
	cmd.PersistentPreRunE = func(c *cobra.Command, args []string) error {
		if nwo, _ := c.Flags().GetString("repo"); nwo != "" {
			return f.OverrideBaseRepo(nwo)
		}
		return nil
	}

	cmd.AddCommand(NewCmdList(f))
	cmd.AddCommand(NewCmdView(f))
	cmd.AddCommand(NewCmdCreate(f))
	cmd.AddCommand(NewCmdDiff(f))
	cmd.AddCommand(NewCmdFiles(f))
	cmd.AddCommand(NewCmdCommits(f))
	cmd.AddCommand(NewCmdCheckout(f))
	cmd.AddCommand(NewCmdEdit(f))
	cmd.AddCommand(NewCmdClose(f))
	cmd.AddCommand(NewCmdReopen(f))

	return cmd
}

// resolvePR turns an optional argument into a concrete pull request. With no
// argument, the pull request whose head branch matches the current git branch
// is used.
func resolvePR(ctx context.Context, f *cmdutil.Factory, arg string) (*api.PullRequest, gvrepo.Interface, error) {
	repo, err := f.BaseRepo()
	if err != nil {
		return nil, nil, err
	}
	client, err := f.HTTPClient()
	if err != nil {
		return nil, nil, err
	}

	if arg != "" {
		number, err := api.ParseNumber(arg)
		if err != nil {
			return nil, nil, cmdutil.FlagErrorf("%v", err)
		}
		pr, err := api.GetPullRequest(ctx, client, repo, number)
		if err != nil {
			return nil, nil, err
		}
		return pr, repo, nil
	}

	branch, err := f.GitClient.CurrentBranch(ctx)
	if err != nil {
		if errors.Is(err, git.ErrNotOnBranch) {
			return nil, nil, errors.New("no pull request number given and HEAD is not on a branch")
		}
		return nil, nil, fmt.Errorf("no pull request number given and the current branch could not be determined: %w", err)
	}

	pr, err := findPRForBranch(ctx, client, repo, branch)
	if err != nil {
		return nil, nil, err
	}
	return pr, repo, nil
}

// findPRForBranch looks for an open pull request whose head is branch, then
// falls back to closed ones.
func findPRForBranch(ctx context.Context, client *api.Client, repo gvrepo.Interface, branch string) (*api.PullRequest, error) {
	for _, state := range []string{"open", "all"} {
		prs, err := api.ListPullRequests(ctx, client, repo, api.PullRequestListOptions{
			State: state,
			Limit: 100,
		})
		if err != nil {
			return nil, err
		}
		for i := range prs {
			if prs[i].Head.Ref == branch {
				return &prs[i], nil
			}
		}
	}
	return nil, fmt.Errorf("no pull request found for branch %q", branch)
}

// stateLabel renders open/merged/closed the way gh does, since the API reports
// merged pull requests with state "closed".
func stateLabel(cs *iostreams.ColorScheme, pr *api.PullRequest) string {
	switch {
	case pr.Merged || pr.MergedAt != "":
		return cs.Magenta("merged")
	case strings.EqualFold(pr.State, "closed"):
		return cs.Red("closed")
	case pr.IsDraft:
		return cs.Gray("draft")
	default:
		return cs.Green("open")
	}
}

// stateColor picks the colour for a pull request's state column.
func stateColor(cs *iostreams.ColorScheme, pr *api.PullRequest) func(string) string {
	switch {
	case pr.Merged || pr.MergedAt != "":
		return cs.Magenta
	case strings.EqualFold(pr.State, "closed"):
		return cs.Red
	case pr.IsDraft:
		return cs.Gray
	default:
		return cs.Green
	}
}

func plainState(pr *api.PullRequest) string {
	switch {
	case pr.Merged || pr.MergedAt != "":
		return "merged"
	case strings.EqualFold(pr.State, "closed"):
		return "closed"
	case pr.IsDraft:
		return "draft"
	default:
		return "open"
	}
}

// parseAPITime accepts the timestamp formats the API returns.
func parseAPITime(raw string) (time.Time, bool) {
	if raw == "" {
		return time.Time{}, false
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02T15:04:05Z0700", "2006-01-02 15:04:05"} {
		if ts, err := time.Parse(layout, raw); err == nil {
			return ts, true
		}
	}
	return time.Time{}, false
}

func relativeTime(now time.Time, raw string) string {
	if ts, ok := parseAPITime(raw); ok {
		return text.RelativeTimeAgo(now, ts)
	}
	return raw
}

func labelNames(labels []api.Label) []string {
	out := make([]string, 0, len(labels))
	for _, l := range labels {
		out = append(out, l.Name)
	}
	return out
}
