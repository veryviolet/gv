package pr

import (
	"fmt"

	"github.com/spf13/cobra"

	"gitverse.ru/veryviolet/gv/internal/api"
	"gitverse.ru/veryviolet/gv/internal/cmdutil"
)

// NewCmdEdit returns `gv pr edit`.
func NewCmdEdit(f *cmdutil.Factory) *cobra.Command {
	var (
		title    string
		body     string
		bodyFile string
		base     string
	)

	cmd := &cobra.Command{
		Use:   "edit [<number>]",
		Short: "Edit a pull request",
		Long: `Change the title, body or base branch of a pull request.

Only the flags you pass are sent, so unspecified fields keep their values.`,
		Example: `  $ gv pr edit 42 --title "Better title"
  $ gv pr edit 42 --body-file notes.md
  $ gv pr edit 42 --base develop`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if body != "" && bodyFile != "" {
				return cmdutil.FlagErrorf("specify only one of --body or --body-file")
			}
			if title == "" && body == "" && bodyFile == "" && base == "" {
				return cmdutil.FlagErrorf("nothing to edit: pass --title, --body, --body-file or --base")
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

			in := api.UpdatePullRequestAPIRequest{}
			if title != "" {
				in.Title = api.Ptr(title)
			}
			if body != "" || bodyFile != "" {
				text, err := cmdutil.ReadBody(f.IOStreams, body, bodyFile)
				if err != nil {
					return err
				}
				in.Body = api.Ptr(text)
			}
			if base != "" {
				in.Base = api.Ptr(base)
			}

			updated, err := api.UpdatePullRequest(cmd.Context(), client, repo, int(pr.Number), in)
			if err != nil {
				return err
			}

			ios := f.IOStreams
			fmt.Fprintf(ios.ErrOut, "%s Updated pull request #%d\n", ios.ColorScheme().SuccessIcon(), updated.Number)
			fmt.Fprintln(ios.Out, updated.HTMLURL)
			return nil
		},
	}

	cmd.Flags().StringVarP(&title, "title", "t", "", "New title")
	cmd.Flags().StringVarP(&body, "body", "b", "", "New body")
	cmd.Flags().StringVarP(&bodyFile, "body-file", "F", "", "Read the new body from `file` (use \"-\" for stdin)")
	cmd.Flags().StringVarP(&base, "base", "B", "", "New base `branch`")

	return cmd
}
