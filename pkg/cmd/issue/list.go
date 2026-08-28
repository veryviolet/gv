package issue

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"gitverse.ru/veryviolet/gv/internal/api"
	"gitverse.ru/veryviolet/gv/internal/cmdutil"
	"gitverse.ru/veryviolet/gv/internal/tableprinter"
	"gitverse.ru/veryviolet/gv/internal/text"
)

// NewCmdList returns `gv issue list`.
func NewCmdList(f *cmdutil.Factory) *cobra.Command {
	var (
		state      string
		labels     []string
		assignee   string
		author     string
		mention    string
		search     string
		milestone  string
		limit      int
		includePRs bool
		web        bool
		jsonOut    cmdutil.JSONFlags
	)

	cmd := &cobra.Command{
		Use:     "list",
		Short:   "List issues",
		Aliases: []string{"ls"},
		Example: `  $ gv issue list
  $ gv issue list --state all --label bug --limit 100
  $ gv issue list --assignee veryviolet --json --jq '.[].title'`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := jsonOut.Validate(); err != nil {
				return err
			}
			switch state {
			case "open", "closed", "all":
			default:
				return cmdutil.FlagErrorf("--state must be one of open, closed or all")
			}

			repo, err := f.BaseRepo()
			if err != nil {
				return err
			}
			if web {
				url := issuesURL(repo)
				fmt.Fprintf(f.IOStreams.ErrOut, "Opening %s in your browser.\n", url)
				return f.Browser.Browse(url)
			}

			client, err := f.HTTPClient()
			if err != nil {
				return err
			}

			// Pull requests share the endpoint, so over-fetch and trim after
			// filtering rather than cutting the list short.
			fetchLimit := limit
			if !includePRs {
				fetchLimit = 0
			}

			issues, err := api.ListIssues(cmd.Context(), client, repo, api.IssueListOptions{
				State:      state,
				Query:      search,
				Labels:     strings.Join(labels, ","),
				Milestones: milestone,
				CreatedBy:  author,
				AssignedBy: assignee,
				Mentioned:  mention,
				Limit:      fetchLimit,
			})
			if err != nil {
				return err
			}

			if !includePRs {
				kept := issues[:0]
				for _, issue := range issues {
					if !issue.IsPullRequest() {
						kept = append(kept, issue)
					}
				}
				issues = kept
			}
			if limit > 0 && len(issues) > limit {
				issues = issues[:limit]
			}

			if jsonOut.Enabled() {
				return cmdutil.PrintJSON(f.IOStreams, issues, jsonOut.JQ, jsonOut.Template)
			}

			ios := f.IOStreams
			cs := ios.ColorScheme()

			if len(issues) == 0 {
				if ios.IsStdoutTTY() {
					fmt.Fprintf(ios.ErrOut, "No %s issues found.\n", state)
				}
				return nil
			}

			now := time.Now()
			tp := tableprinter.New(ios.Out, ios.IsStdoutTTY(), ios.TerminalWidth())
			tp.HeaderRow("id", "title", "labels", "state", "updated")
			for i := range issues {
				issue := &issues[i]
				tp.AddField(fmt.Sprintf("#%d", issue.Number), tableprinter.WithColor(cs.Gray), tableprinter.WithTruncate(false))
				tp.AddField(text.FirstLine(issue.Title), tableprinter.WithTruncate(true))
				tp.AddField(strings.Join(labelNames(issue.Labels), ", "), tableprinter.WithColor(cs.Cyan), tableprinter.WithTruncate(true))
				tp.AddField(strings.ToLower(issue.State), tableprinter.WithColor(stateColor(cs, issue)), tableprinter.WithTruncate(false))
				tp.AddField(text.TimeAgo(now, issue.UpdatedAt), tableprinter.WithColor(cs.Gray), tableprinter.WithTruncate(false))
				tp.EndRow()
			}
			return tp.Render()
		},
	}

	cmd.Flags().StringVarP(&state, "state", "s", "open", "Filter by state: {open|closed|all}")
	cmd.Flags().StringSliceVarP(&labels, "label", "l", nil, "Filter by `labels`")
	cmd.Flags().StringVarP(&assignee, "assignee", "a", "", "Filter by assignee `login`")
	cmd.Flags().StringVar(&author, "author", "", "Filter by author `login`")
	cmd.Flags().StringVar(&mention, "mention", "", "Filter by mentioned `login`")
	cmd.Flags().StringVar(&milestone, "milestone", "", "Filter by `milestone`")
	cmd.Flags().StringVarP(&search, "search", "S", "", "Search issues with a `query`")
	cmd.Flags().IntVarP(&limit, "limit", "L", 30, "Maximum number of issues to list")
	cmd.Flags().BoolVar(&includePRs, "include-prs", false, "Include pull requests in the listing")
	cmd.Flags().BoolVarP(&web, "web", "w", false, "List issues in a browser")
	cmd.Flags().BoolVar(&jsonOut.JSON, "json", false, "Output the issues as JSON")
	cmd.Flags().StringVarP(&jsonOut.JQ, "jq", "q", "", "Filter the JSON output with a jq `expression`")
	cmd.Flags().StringVarP(&jsonOut.Template, "template", "t", "", "Format the JSON output with a Go `template`")

	return cmd
}
