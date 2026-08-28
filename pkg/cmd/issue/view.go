package issue

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"gitverse.ru/veryviolet/gv/internal/api"
	"gitverse.ru/veryviolet/gv/internal/cmdutil"
	"gitverse.ru/veryviolet/gv/internal/text"
)

// NewCmdView returns `gv issue view`.
func NewCmdView(f *cmdutil.Factory) *cobra.Command {
	var (
		web      bool
		comments bool
		jsonOut  cmdutil.JSONFlags
	)

	cmd := &cobra.Command{
		Use:   "view <number>",
		Short: "Show an issue",
		Example: `  $ gv issue view 12
  $ gv issue view 12 --comments`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := jsonOut.Validate(); err != nil {
				return err
			}
			issue, repo, err := resolveIssue(cmd.Context(), f, args[0])
			if err != nil {
				return err
			}
			if web {
				fmt.Fprintf(f.IOStreams.ErrOut, "Opening %s in your browser.\n", issue.HTMLURL)
				return f.Browser.Browse(issue.HTMLURL)
			}
			if jsonOut.Enabled() {
				return cmdutil.PrintJSON(f.IOStreams, issue, jsonOut.JQ, jsonOut.Template)
			}

			ios := f.IOStreams
			cs := ios.ColorScheme()
			now := time.Now()

			if !ios.IsStdoutTTY() {
				fmt.Fprintf(ios.Out, "title:\t%s\n", issue.Title)
				fmt.Fprintf(ios.Out, "state:\t%s\n", strings.ToLower(issue.State))
				fmt.Fprintf(ios.Out, "author:\t%s\n", issue.User.Login)
				fmt.Fprintf(ios.Out, "labels:\t%s\n", strings.Join(labelNames(issue.Labels), ", "))
				fmt.Fprintf(ios.Out, "comments:\t%d\n", issue.Comments)
				fmt.Fprintf(ios.Out, "url:\t%s\n", issue.HTMLURL)
				fmt.Fprintf(ios.Out, "--\n%s\n", issue.Body)
				return nil
			}

			kind := "issue"
			if issue.IsPullRequest() {
				kind = "pull request"
			}
			fmt.Fprintf(ios.Out, "%s %s\n", cs.Bold(issue.Title), cs.Gray(fmt.Sprintf("#%d", issue.Number)))
			fmt.Fprintf(ios.Out, "%s • %s opened this %s %s • %s\n",
				stateColor(cs, issue)(strings.ToLower(issue.State)),
				issue.User.Login,
				kind,
				cs.Gray(text.TimeAgo(now, issue.CreatedAt)),
				cs.Gray(text.Pluralize(int(issue.Comments), "comment")))

			if names := labelNames(issue.Labels); len(names) > 0 {
				fmt.Fprintf(ios.Out, "%s\n", cs.Gray("labels: "+strings.Join(names, ", ")))
			}
			if len(issue.Assignees) > 0 {
				var who []string
				for _, a := range issue.Assignees {
					who = append(who, a.Login)
				}
				fmt.Fprintf(ios.Out, "%s\n", cs.Gray("assignees: "+strings.Join(who, ", ")))
			}

			fmt.Fprintln(ios.Out)
			if body := strings.TrimSpace(issue.Body); body == "" {
				fmt.Fprintln(ios.Out, cs.Gray("No description provided."))
			} else {
				fmt.Fprintln(ios.Out, text.RemoveExcessBlankLines(body))
			}

			if comments {
				client, err := f.HTTPClient()
				if err != nil {
					return err
				}
				list, err := api.ListIssueComments(cmd.Context(), client, repo, int(issue.Number), 0)
				if err != nil {
					return err
				}
				fmt.Fprintln(ios.Out)
				if len(list) == 0 {
					fmt.Fprintln(ios.Out, cs.Gray("No comments."))
				}
				for _, c := range list {
					fmt.Fprintf(ios.Out, "%s %s\n", cs.Bold(c.User.Login), cs.Gray(text.TimeAgo(now, c.CreatedAt)))
					fmt.Fprintf(ios.Out, "%s\n\n", text.Indent(text.RemoveExcessBlankLines(c.Body), "  "))
				}
			}

			fmt.Fprintf(ios.Out, "\n%s\n", cs.Gray("View on GitVerse: "+issue.HTMLURL))
			return nil
		},
	}

	cmd.Flags().BoolVarP(&web, "web", "w", false, "Open the issue in a browser")
	cmd.Flags().BoolVarP(&comments, "comments", "c", false, "Show the comment thread")
	cmd.Flags().BoolVar(&jsonOut.JSON, "json", false, "Output the issue as JSON")
	cmd.Flags().StringVarP(&jsonOut.JQ, "jq", "q", "", "Filter the JSON output with a jq `expression`")
	cmd.Flags().StringVarP(&jsonOut.Template, "template", "t", "", "Format the JSON output with a Go `template`")

	return cmd
}
