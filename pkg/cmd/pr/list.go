package pr

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

// NewCmdList returns `gv pr list`.
func NewCmdList(f *cmdutil.Factory) *cobra.Command {
	var (
		state     string
		base      string
		head      string
		sortBy    string
		direction string
		limit     int
		web       bool
		jsonOut   cmdutil.JSONFlags
	)

	cmd := &cobra.Command{
		Use:     "list",
		Short:   "List pull requests",
		Aliases: []string{"ls"},
		Example: `  $ gv pr list
  $ gv pr list --state all --limit 50
  $ gv pr list --base main --json --jq '.[].title'`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := jsonOut.Validate(); err != nil {
				return err
			}
			switch state {
			case "open", "closed", "all", "merged":
			default:
				return cmdutil.FlagErrorf("--state must be one of open, closed, merged or all")
			}

			repo, err := f.BaseRepo()
			if err != nil {
				return err
			}
			if web {
				url := fmt.Sprintf("https://%s/%s/%s/pulls", repo.RepoHost(), repo.RepoOwner(), repo.RepoName())
				fmt.Fprintf(f.IOStreams.ErrOut, "Opening %s in your browser.\n", url)
				return f.Browser.Browse(url)
			}

			client, err := f.HTTPClient()
			if err != nil {
				return err
			}

			// The API has no "merged" state; fetch closed ones and filter.
			apiState := state
			if state == "merged" {
				apiState = "closed"
			}
			fetchLimit := limit
			if state == "merged" {
				fetchLimit = 0
			}

			prs, err := api.ListPullRequests(cmd.Context(), client, repo, api.PullRequestListOptions{
				State:     apiState,
				Base:      base,
				Head:      head,
				Sort:      sortBy,
				Direction: direction,
				Limit:     fetchLimit,
			})
			if err != nil {
				return err
			}

			if state == "merged" {
				merged := prs[:0]
				for _, pr := range prs {
					if pr.Merged || pr.MergedAt != "" {
						merged = append(merged, pr)
					}
				}
				prs = merged
				if limit > 0 && len(prs) > limit {
					prs = prs[:limit]
				}
			}

			if jsonOut.Enabled() {
				return cmdutil.PrintJSON(f.IOStreams, prs, jsonOut.JQ, jsonOut.Template)
			}
			return printPRList(f, prs, state)
		},
	}

	cmd.Flags().StringVarP(&state, "state", "s", "open", "Filter by state: {open|closed|merged|all}")
	cmd.Flags().StringVarP(&base, "base", "B", "", "Filter by base `branch`")
	cmd.Flags().StringVarP(&head, "head", "H", "", "Filter by head branch, as \"branch\" or \"owner:branch\"")
	cmd.Flags().StringVar(&sortBy, "sort", "", "Sort by `field`")
	cmd.Flags().StringVar(&direction, "direction", "", "Sort `direction`: {asc|desc}")
	cmd.Flags().IntVarP(&limit, "limit", "L", 30, "Maximum number of pull requests to list")
	cmd.Flags().BoolVarP(&web, "web", "w", false, "List pull requests in a browser")
	cmd.Flags().BoolVar(&jsonOut.JSON, "json", false, "Output the pull requests as JSON")
	cmd.Flags().StringVarP(&jsonOut.JQ, "jq", "q", "", "Filter the JSON output with a jq `expression`")
	cmd.Flags().StringVarP(&jsonOut.Template, "template", "t", "", "Format the JSON output with a Go `template`")

	return cmd
}

func printPRList(f *cmdutil.Factory, prs []api.PullRequest, state string) error {
	ios := f.IOStreams
	cs := ios.ColorScheme()

	if len(prs) == 0 {
		if ios.IsStdoutTTY() {
			fmt.Fprintf(ios.ErrOut, "No %s pull requests found.\n", state)
		}
		return nil
	}

	now := time.Now()
	tp := tableprinter.New(ios.Out, ios.IsStdoutTTY(), ios.TerminalWidth())
	tp.HeaderRow("id", "title", "branch", "state", "updated")
	for i := range prs {
		pr := &prs[i]
		tp.AddField(fmt.Sprintf("#%d", pr.Number), tableprinter.WithColor(cs.Gray), tableprinter.WithTruncate(false))
		title := text.FirstLine(pr.Title)
		if names := labelNames(pr.Labels); len(names) > 0 {
			title += "  " + cs.Gray("("+strings.Join(names, ", ")+")")
		}
		tp.AddField(title, tableprinter.WithTruncate(true))
		tp.AddField(pr.Head.Ref, tableprinter.WithColor(cs.Cyan), tableprinter.WithTruncate(true))
		tp.AddField(plainState(pr), tableprinter.WithColor(stateColor(cs, pr)), tableprinter.WithTruncate(false))
		tp.AddField(relativeTime(now, pr.UpdatedAt), tableprinter.WithColor(cs.Gray), tableprinter.WithTruncate(false))
		tp.EndRow()
	}
	return tp.Render()
}
