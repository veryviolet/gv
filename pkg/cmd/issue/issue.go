// Package issue implements `gv issue`.
package issue

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"gitverse.ru/veryviolet/gv/internal/api"
	"gitverse.ru/veryviolet/gv/internal/cmdutil"
	"gitverse.ru/veryviolet/gv/internal/gvrepo"
	"gitverse.ru/veryviolet/gv/internal/iostreams"
)

// NewCmdIssue returns the `gv issue` command group.
func NewCmdIssue(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "issue <command>",
		Short:   "Work with issues",
		Aliases: []string{"issues"},
		Long: `List, inspect, open and close issues.

GitVerse returns issues and pull requests from the same endpoint; gv filters
pull requests out of issue listings unless --include-prs is given.`,
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
	cmd.AddCommand(NewCmdComment(f))
	cmd.AddCommand(NewCmdEdit(f))
	cmd.AddCommand(NewCmdClose(f))
	cmd.AddCommand(NewCmdReopen(f))

	return cmd
}

// resolveIssue fetches the issue identified by arg, which may be "12" or "#12".
func resolveIssue(ctx context.Context, f *cmdutil.Factory, arg string) (*api.Issue, gvrepo.Interface, error) {
	repo, err := f.BaseRepo()
	if err != nil {
		return nil, nil, err
	}
	number, err := api.ParseNumber(arg)
	if err != nil {
		return nil, nil, cmdutil.FlagErrorf("%v", err)
	}
	client, err := f.HTTPClient()
	if err != nil {
		return nil, nil, err
	}
	issue, err := api.GetIssue(ctx, client, repo, number)
	if err != nil {
		return nil, nil, err
	}
	return issue, repo, nil
}

func stateColor(cs *iostreams.ColorScheme, issue *api.Issue) func(string) string {
	if strings.EqualFold(issue.State, "closed") {
		return cs.Magenta
	}
	return cs.Green
}

func labelNames(labels []api.Label) []string {
	out := make([]string, 0, len(labels))
	for _, l := range labels {
		out = append(out, l.Name)
	}
	return out
}

func issuesURL(repo gvrepo.Interface) string {
	return fmt.Sprintf("https://%s/%s/%s/issues", repo.RepoHost(), repo.RepoOwner(), repo.RepoName())
}
