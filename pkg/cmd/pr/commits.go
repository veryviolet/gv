package pr

import (
	"time"

	"github.com/spf13/cobra"

	"gitverse.ru/veryviolet/gv/internal/api"
	"gitverse.ru/veryviolet/gv/internal/cmdutil"
	"gitverse.ru/veryviolet/gv/internal/tableprinter"
	"gitverse.ru/veryviolet/gv/internal/text"
)

// NewCmdCommits returns `gv pr commits`.
func NewCmdCommits(f *cmdutil.Factory) *cobra.Command {
	var jsonOut cmdutil.JSONFlags

	cmd := &cobra.Command{
		Use:     "commits [<number>]",
		Short:   "List the commits of a pull request",
		Example: `  $ gv pr commits 42`,
		Args:    cobra.MaximumNArgs(1),
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
			client, err := f.HTTPClient()
			if err != nil {
				return err
			}
			commits, err := api.ListPullRequestCommits(cmd.Context(), client, repo, int(pr.Number), 0)
			if err != nil {
				return err
			}
			if jsonOut.Enabled() {
				return cmdutil.PrintJSON(f.IOStreams, commits, jsonOut.JQ, jsonOut.Template)
			}

			ios := f.IOStreams
			cs := ios.ColorScheme()
			now := time.Now()

			tp := tableprinter.New(ios.Out, ios.IsStdoutTTY(), ios.TerminalWidth())
			tp.HeaderRow("sha", "message", "author", "when")
			for _, c := range commits {
				tp.AddField(shortSHA(c.SHA), tableprinter.WithColor(cs.Gray), tableprinter.WithTruncate(false))
				tp.AddField(text.FirstLine(c.Commit.Message), tableprinter.WithTruncate(true))
				tp.AddField(commitAuthor(&c), tableprinter.WithTruncate(true))
				tp.AddField(relativeTime(now, c.Created), tableprinter.WithColor(cs.Gray), tableprinter.WithTruncate(false))
				tp.EndRow()
			}
			return tp.Render()
		},
	}

	cmd.Flags().BoolVar(&jsonOut.JSON, "json", false, "Output the commits as JSON")
	cmd.Flags().StringVarP(&jsonOut.JQ, "jq", "q", "", "Filter the JSON output with a jq `expression`")
	cmd.Flags().StringVarP(&jsonOut.Template, "template", "t", "", "Format the JSON output with a Go `template`")

	return cmd
}

func shortSHA(sha string) string {
	if len(sha) > 7 {
		return sha[:7]
	}
	return sha
}

// commitAuthor prefers the GitVerse account, falling back to the git author.
func commitAuthor(c *api.CommitResponse) string {
	if c.Author.Login != "" {
		return c.Author.Login
	}
	if c.Commit.Author.Name != "" {
		return c.Commit.Author.Name
	}
	return c.Commit.Author.Email
}
