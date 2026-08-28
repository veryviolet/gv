package issue

import (
	"fmt"

	"github.com/spf13/cobra"

	"gitverse.ru/veryviolet/gv/internal/api"
	"gitverse.ru/veryviolet/gv/internal/cmdutil"
)

// NewCmdEdit returns `gv issue edit`.
func NewCmdEdit(f *cmdutil.Factory) *cobra.Command {
	var (
		title     string
		body      string
		bodyFile  string
		labels    []string
		assignees []string
		dueDate   string
	)

	cmd := &cobra.Command{
		Use:   "edit <number>",
		Short: "Edit an issue",
		Long: `Change the title, body, labels or assignees of an issue.

Only the flags you pass are sent. Note that --label and --assignee replace the
existing sets rather than adding to them, because the API takes whole lists.`,
		Example: `  $ gv issue edit 12 --title "Clearer title"
  $ gv issue edit 12 --label bug --label regression`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if body != "" && bodyFile != "" {
				return cmdutil.FlagErrorf("specify only one of --body or --body-file")
			}
			if title == "" && body == "" && bodyFile == "" && len(labels) == 0 && len(assignees) == 0 && dueDate == "" {
				return cmdutil.FlagErrorf("nothing to edit: pass --title, --body, --body-file, --label, --assignee or --due-date")
			}

			issue, repo, err := resolveIssue(cmd.Context(), f, args[0])
			if err != nil {
				return err
			}
			client, err := f.HTTPClient()
			if err != nil {
				return err
			}

			in := api.UpdateIssueRequest{}
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
			if len(labels) > 0 {
				in.Labels = labels
			}
			if len(assignees) > 0 {
				in.Assignees = assignees
			}
			if dueDate != "" {
				in.DueDate = api.Ptr(dueDate)
			}

			updated, err := api.UpdateIssue(cmd.Context(), client, repo, int(issue.Number), in)
			if err != nil {
				return err
			}

			ios := f.IOStreams
			fmt.Fprintf(ios.ErrOut, "%s Updated issue #%d\n", ios.ColorScheme().SuccessIcon(), updated.Number)
			fmt.Fprintln(ios.Out, updated.HTMLURL)
			return nil
		},
	}

	cmd.Flags().StringVarP(&title, "title", "t", "", "New title")
	cmd.Flags().StringVarP(&body, "body", "b", "", "New body")
	cmd.Flags().StringVarP(&bodyFile, "body-file", "F", "", "Read the new body from `file` (use \"-\" for stdin)")
	cmd.Flags().StringSliceVarP(&labels, "label", "l", nil, "Replace the labels with these `names`")
	cmd.Flags().StringSliceVarP(&assignees, "assignee", "a", nil, "Replace the assignees with these `logins`")
	cmd.Flags().StringVar(&dueDate, "due-date", "", "New due `date` in YYYY-MM-DD format")

	return cmd
}
