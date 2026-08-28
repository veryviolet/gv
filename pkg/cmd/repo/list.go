package repo

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"gitverse.ru/veryviolet/gv/internal/api"
	"gitverse.ru/veryviolet/gv/internal/cmdutil"
	"gitverse.ru/veryviolet/gv/internal/tableprinter"
	"gitverse.ru/veryviolet/gv/internal/text"
)

type listOptions struct {
	limit   int
	org     string
	source  bool
	fork    bool
	private bool
	public  bool
	jsonOut cmdutil.JSONFlags
}

// NewCmdList returns `gv repo list`.
func NewCmdList(f *cmdutil.Factory) *cobra.Command {
	var opts listOptions

	cmd := &cobra.Command{
		Use:     "list [<owner>]",
		Short:   "List repositories",
		Aliases: []string{"ls"},
		Long: `List repositories owned by you, by a user, or by an organization.

The API returns every repository the token can see; the --fork, --source,
--public and --private flags filter that list client-side.`,
		Example: `  $ gv repo list
  $ gv repo list veryviolet --limit 50
  $ gv repo list --org myorg --source`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := opts.jsonOut.Validate(); err != nil {
				return err
			}
			if err := cmdutil.MutuallyExclusive("specify only one of --source or --fork", opts.source, opts.fork); err != nil {
				return err
			}
			if err := cmdutil.MutuallyExclusive("specify only one of --public or --private", opts.public, opts.private); err != nil {
				return err
			}
			if opts.limit < 0 {
				return cmdutil.FlagErrorf("--limit must not be negative")
			}
			owner := ""
			if len(args) > 0 {
				owner = args[0]
			}
			if owner != "" && opts.org != "" {
				return cmdutil.FlagErrorf("pass either an owner argument or --org, not both")
			}
			return listRun(cmd.Context(), f, owner, &opts)
		},
	}

	cmd.Flags().IntVarP(&opts.limit, "limit", "L", 30, "Maximum number of repositories to list")
	cmd.Flags().StringVar(&opts.org, "org", "", "List repositories of an `organization`")
	cmd.Flags().BoolVar(&opts.source, "source", false, "Show only repositories that are not forks")
	cmd.Flags().BoolVar(&opts.fork, "fork", false, "Show only forks")
	cmd.Flags().BoolVar(&opts.private, "private", false, "Show only private repositories")
	cmd.Flags().BoolVar(&opts.public, "public", false, "Show only public repositories")
	cmd.Flags().BoolVar(&opts.jsonOut.JSON, "json", false, "Output the repositories as JSON")
	cmd.Flags().StringVarP(&opts.jsonOut.JQ, "jq", "q", "", "Filter the JSON output with a jq `expression`")
	cmd.Flags().StringVarP(&opts.jsonOut.Template, "template", "t", "", "Format the JSON output with a Go `template`")

	return cmd
}

func listRun(ctx context.Context, f *cmdutil.Factory, owner string, opts *listOptions) error {
	client, err := f.HTTPClient()
	if err != nil {
		return err
	}

	// Filtering happens after fetching, so ask for more than the display limit
	// when a filter is active.
	fetchLimit := opts.limit
	if filtersActive(opts) && fetchLimit > 0 {
		fetchLimit = 0
	}

	var repos []api.Repository
	switch {
	case opts.org != "":
		repos, err = api.ListOrgRepositories(ctx, client, opts.org, fetchLimit)
	case owner != "":
		repos, err = api.ListUserRepositories(ctx, client, owner, fetchLimit)
	default:
		repos, err = api.ListMyRepositories(ctx, client, fetchLimit)
	}
	if err != nil {
		return err
	}

	repos = filterRepos(repos, opts)
	if opts.limit > 0 && len(repos) > opts.limit {
		repos = repos[:opts.limit]
	}

	if opts.jsonOut.Enabled() {
		return cmdutil.PrintJSON(f.IOStreams, repos, opts.jsonOut.JQ, opts.jsonOut.Template)
	}

	ios := f.IOStreams
	cs := ios.ColorScheme()

	if len(repos) == 0 {
		if ios.IsStdoutTTY() {
			fmt.Fprintln(ios.ErrOut, "No repositories found.")
		}
		return nil
	}

	now := time.Now()
	tp := tableprinter.New(ios.Out, ios.IsStdoutTTY(), ios.TerminalWidth())
	tp.HeaderRow("name", "description", "visibility", "updated")
	for _, r := range repos {
		tp.AddField(r.FullName, tableprinter.WithColor(cs.Bold), tableprinter.WithTruncate(false))
		tp.AddField(text.FirstLine(r.Description), tableprinter.WithTruncate(true))
		tp.AddField(visibility(&r), tableprinter.WithColor(visibilityColor(cs, &r)))
		tp.AddField(updatedAt(now, r.UpdatedAt), tableprinter.WithColor(cs.Gray))
		tp.EndRow()
	}
	return tp.Render()
}

func filtersActive(opts *listOptions) bool {
	return opts.source || opts.fork || opts.public || opts.private
}

func filterRepos(repos []api.Repository, opts *listOptions) []api.Repository {
	if !filtersActive(opts) {
		return repos
	}
	out := repos[:0]
	for _, r := range repos {
		switch {
		case opts.source && r.Fork:
			continue
		case opts.fork && !r.Fork:
			continue
		case opts.private && !isPrivate(&r):
			continue
		case opts.public && isPrivate(&r):
			continue
		}
		out = append(out, r)
	}
	return out
}

func isPrivate(r *api.Repository) bool {
	return r.Private || strings.EqualFold(r.Visibility, "private")
}

func visibilityColor(cs interface{ Yellow(string) string }, r *api.Repository) func(string) string {
	if isPrivate(r) {
		return cs.Yellow
	}
	return func(s string) string { return s }
}

func updatedAt(now time.Time, raw string) string {
	if raw == "" {
		return ""
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02T15:04:05Z0700", "2006-01-02 15:04:05"} {
		if ts, err := time.Parse(layout, raw); err == nil {
			return text.RelativeTimeAgo(now, ts)
		}
	}
	return raw
}
