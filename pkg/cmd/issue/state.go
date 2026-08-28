package issue

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"gitverse.ru/veryviolet/gv/internal/api"
	"gitverse.ru/veryviolet/gv/internal/cmdutil"
)

// NewCmdClose returns `gv issue close`.
func NewCmdClose(f *cmdutil.Factory) *cobra.Command {
	var comment string

	cmd := &cobra.Command{
		Use:     "close <number>",
		Short:   "Close an issue",
		Example: `  $ gv issue close 12 --comment "Fixed in 1.4.1"`,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return setState(cmd.Context(), f, args[0], "closed", comment)
		},
	}

	cmd.Flags().StringVarP(&comment, "comment", "c", "", "Leave a closing `comment`")

	return cmd
}

// NewCmdReopen returns `gv issue reopen`.
func NewCmdReopen(f *cmdutil.Factory) *cobra.Command {
	var comment string

	cmd := &cobra.Command{
		Use:     "reopen <number>",
		Short:   "Reopen a closed issue",
		Example: `  $ gv issue reopen 12`,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return setState(cmd.Context(), f, args[0], "open", comment)
		},
	}

	cmd.Flags().StringVarP(&comment, "comment", "c", "", "Leave a `comment` when reopening")

	return cmd
}

func setState(ctx context.Context, f *cmdutil.Factory, arg, state, comment string) error {
	issue, repo, err := resolveIssue(ctx, f, arg)
	if err != nil {
		return err
	}

	ios := f.IOStreams
	cs := ios.ColorScheme()

	if strings.EqualFold(issue.State, state) {
		fmt.Fprintf(ios.ErrOut, "%s Issue #%d is already %s\n", cs.WarningIcon(), issue.Number, state)
		return nil
	}

	client, err := f.HTTPClient()
	if err != nil {
		return err
	}

	if comment != "" {
		if _, err := api.CreateIssueComment(ctx, client, repo, int(issue.Number), api.CreateIssueCommentRequest{
			Body: api.Ptr(comment),
		}); err != nil {
			return err
		}
	}

	updated, err := api.UpdateIssue(ctx, client, repo, int(issue.Number), api.UpdateIssueRequest{
		State: api.Ptr(state),
	})
	if err != nil {
		return err
	}

	verb := "Closed"
	if state == "open" {
		verb = "Reopened"
	}
	fmt.Fprintf(ios.ErrOut, "%s %s issue %s\n",
		cs.SuccessIcon(), verb, cs.Bold(fmt.Sprintf("#%d", updated.Number)))
	return nil
}
