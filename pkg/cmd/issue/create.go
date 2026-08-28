package issue

import (
	"fmt"

	"github.com/spf13/cobra"

	"gitverse.ru/veryviolet/gv/internal/api"
	"gitverse.ru/veryviolet/gv/internal/cmdutil"
)

// NewCmdCreate returns `gv issue create`.
func NewCmdCreate(f *cmdutil.Factory) *cobra.Command {
	var (
		title     string
		body      string
		bodyFile  string
		labels    []string
		assignees []string
		dueDate   string
		web       bool
	)

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Open a new issue",
		Long: `Create an issue.

A title is required; gv does not open an editor. Use --body-file - to read a
long description from standard input.`,
		Example: `  $ gv issue create --title "Crash on startup" --body "Steps: ..."
  $ gv issue create -t "Docs" -F notes.md --label docs --assignee veryviolet`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if body != "" && bodyFile != "" {
				return cmdutil.FlagErrorf("specify only one of --body or --body-file")
			}

			repo, err := f.BaseRepo()
			if err != nil {
				return err
			}
			if web {
				url := issuesURL(repo) + "/new"
				fmt.Fprintf(f.IOStreams.ErrOut, "Opening %s in your browser.\n", url)
				return f.Browser.Browse(url)
			}
			if title == "" {
				return cmdutil.FlagErrorf("--title is required")
			}

			text, err := cmdutil.ReadBody(f.IOStreams, body, bodyFile)
			if err != nil {
				return err
			}

			client, err := f.HTTPClient()
			if err != nil {
				return err
			}

			in := api.CreateIssueRequest{Title: api.Ptr(title)}
			if text != "" {
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

			issue, err := api.CreateIssue(cmd.Context(), client, repo, in)
			if err != nil {
				return err
			}

			ios := f.IOStreams
			fmt.Fprintf(ios.ErrOut, "%s Created issue %s\n",
				ios.ColorScheme().SuccessIcon(), ios.ColorScheme().Bold(fmt.Sprintf("#%d", issue.Number)))
			fmt.Fprintln(ios.Out, issue.HTMLURL)
			return nil
		},
	}

	cmd.Flags().StringVarP(&title, "title", "t", "", "Title of the issue")
	cmd.Flags().StringVarP(&body, "body", "b", "", "Body of the issue")
	cmd.Flags().StringVarP(&bodyFile, "body-file", "F", "", "Read the body from `file` (use \"-\" for stdin)")
	cmd.Flags().StringSliceVarP(&labels, "label", "l", nil, "Add `labels` to the issue")
	cmd.Flags().StringSliceVarP(&assignees, "assignee", "a", nil, "Assign `people` to the issue")
	cmd.Flags().StringVar(&dueDate, "due-date", "", "Due `date` in YYYY-MM-DD format")
	cmd.Flags().BoolVarP(&web, "web", "w", false, "Open the new-issue form in a browser instead")

	return cmd
}
