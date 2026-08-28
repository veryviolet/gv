package pr

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"gitverse.ru/veryviolet/gv/internal/api"
	"gitverse.ru/veryviolet/gv/internal/cmdutil"
	"gitverse.ru/veryviolet/gv/internal/text"
)

// NewCmdView returns `gv pr view`.
func NewCmdView(f *cmdutil.Factory) *cobra.Command {
	var (
		web      bool
		comments bool
		jsonOut  cmdutil.JSONFlags
	)

	cmd := &cobra.Command{
		Use:   "view [<number>]",
		Short: "Show a pull request",
		Long: `Display the title, state, body and metadata of a pull request.

Without a number, the pull request for the current branch is used.`,
		Example: `  $ gv pr view
  $ gv pr view 42 --comments
  $ gv pr view 42 --web`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := jsonOut.Validate(); err != nil {
				return err
			}
			arg := ""
			if len(args) > 0 {
				arg = args[0]
			}
			pr, repo, err := resolvePR(cmd.Context(), f, arg)
			if err != nil {
				return err
			}
			if web {
				fmt.Fprintf(f.IOStreams.ErrOut, "Opening %s in your browser.\n", pr.HTMLURL)
				return f.Browser.Browse(pr.HTMLURL)
			}
			if jsonOut.Enabled() {
				return cmdutil.PrintJSON(f.IOStreams, pr, jsonOut.JQ, jsonOut.Template)
			}

			ios := f.IOStreams
			cs := ios.ColorScheme()

			if !ios.IsStdoutTTY() {
				fmt.Fprintf(ios.Out, "title:\t%s\n", pr.Title)
				fmt.Fprintf(ios.Out, "state:\t%s\n", plainState(pr))
				fmt.Fprintf(ios.Out, "author:\t%s\n", pr.User.Login)
				fmt.Fprintf(ios.Out, "base:\t%s\n", pr.Base.Ref)
				fmt.Fprintf(ios.Out, "head:\t%s\n", pr.Head.Ref)
				fmt.Fprintf(ios.Out, "url:\t%s\n", pr.HTMLURL)
				fmt.Fprintf(ios.Out, "--\n%s\n", pr.Body)
				return nil
			}

			now := time.Now()
			fmt.Fprintf(ios.Out, "%s %s\n", cs.Bold(pr.Title), cs.Gray(fmt.Sprintf("#%d", pr.Number)))
			fmt.Fprintf(ios.Out, "%s • %s wants to merge %s into %s • %s\n",
				stateLabel(cs, pr),
				pr.User.Login,
				cs.Cyan(pr.Head.Ref),
				cs.Cyan(pr.Base.Ref),
				cs.Gray(relativeTime(now, pr.CreatedAt)))

			if names := labelNames(pr.Labels); len(names) > 0 {
				fmt.Fprintf(ios.Out, "%s\n", cs.Gray("labels: "+strings.Join(names, ", ")))
			}
			if len(pr.RequestedReviewers) > 0 {
				var reviewers []string
				for _, r := range pr.RequestedReviewers {
					reviewers = append(reviewers, r.Login)
				}
				fmt.Fprintf(ios.Out, "%s\n", cs.Gray("reviewers: "+strings.Join(reviewers, ", ")))
			}

			body := strings.TrimSpace(pr.Body)
			fmt.Fprintln(ios.Out)
			if body == "" {
				fmt.Fprintln(ios.Out, cs.Gray("No description provided."))
			} else {
				fmt.Fprintln(ios.Out, text.RemoveExcessBlankLines(body))
			}

			if comments {
				client, err := f.HTTPClient()
				if err != nil {
					return err
				}
				list, err := api.ListIssueComments(cmd.Context(), client, repo, int(pr.Number), 0)
				if err != nil {
					return err
				}
				fmt.Fprintln(ios.Out)
				if len(list) == 0 {
					fmt.Fprintln(ios.Out, cs.Gray("No comments."))
				}
				for _, c := range list {
					fmt.Fprintf(ios.Out, "%s %s\n", cs.Bold(c.User.Login), cs.Gray(relativeTime(now, c.CreatedAt)))
					fmt.Fprintf(ios.Out, "%s\n\n", text.Indent(text.RemoveExcessBlankLines(c.Body), "  "))
				}
			}

			fmt.Fprintf(ios.Out, "\n%s\n", cs.Gray("View on GitVerse: "+pr.HTMLURL))
			return nil
		},
	}

	cmd.Flags().BoolVarP(&web, "web", "w", false, "Open the pull request in a browser")
	cmd.Flags().BoolVarP(&comments, "comments", "c", false, "Show the comment thread")
	cmd.Flags().BoolVar(&jsonOut.JSON, "json", false, "Output the pull request as JSON")
	cmd.Flags().StringVarP(&jsonOut.JQ, "jq", "q", "", "Filter the JSON output with a jq `expression`")
	cmd.Flags().StringVarP(&jsonOut.Template, "template", "t", "", "Format the JSON output with a Go `template`")

	return cmd
}
