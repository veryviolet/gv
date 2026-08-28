package issue

import (
	"fmt"

	"github.com/spf13/cobra"

	"gitverse.ru/veryviolet/gv/internal/api"
	"gitverse.ru/veryviolet/gv/internal/cmdutil"
)

// NewCmdComment returns `gv issue comment`.
func NewCmdComment(f *cmdutil.Factory) *cobra.Command {
	var (
		body     string
		bodyFile string
	)

	cmd := &cobra.Command{
		Use:   "comment <number>",
		Short: "Comment on an issue or pull request",
		Long: `Add a comment.

The same endpoint serves issues and pull requests, so this works for either.`,
		Example: `  $ gv issue comment 12 --body "Reproduced on 1.4.0"
  $ git log -1 | gv issue comment 12 --body-file -`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if body == "" && bodyFile == "" {
				return cmdutil.FlagErrorf("pass --body or --body-file")
			}
			if body != "" && bodyFile != "" {
				return cmdutil.FlagErrorf("specify only one of --body or --body-file")
			}

			repo, err := f.BaseRepo()
			if err != nil {
				return err
			}
			number, err := api.ParseNumber(args[0])
			if err != nil {
				return cmdutil.FlagErrorf("%v", err)
			}
			text, err := cmdutil.ReadBody(f.IOStreams, body, bodyFile)
			if err != nil {
				return err
			}
			if text == "" {
				return cmdutil.FlagErrorf("the comment body is empty")
			}

			client, err := f.HTTPClient()
			if err != nil {
				return err
			}
			comment, err := api.CreateIssueComment(cmd.Context(), client, repo, number, api.CreateIssueCommentRequest{
				Body: api.Ptr(text),
			})
			if err != nil {
				return err
			}

			ios := f.IOStreams
			fmt.Fprintf(ios.ErrOut, "%s Added comment to #%d\n", ios.ColorScheme().SuccessIcon(), number)
			if comment.HTMLURL != "" {
				fmt.Fprintln(ios.Out, comment.HTMLURL)
			}
			return nil
		},
	}

	cmd.Flags().StringVarP(&body, "body", "b", "", "Text of the comment")
	cmd.Flags().StringVarP(&bodyFile, "body-file", "F", "", "Read the comment from `file` (use \"-\" for stdin)")

	return cmd
}
