package pr

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"gitverse.ru/veryviolet/gv/internal/api"
	"gitverse.ru/veryviolet/gv/internal/cmdutil"
)

// NewCmdClose returns `gv pr close`.
func NewCmdClose(f *cmdutil.Factory) *cobra.Command {
	var comment string

	cmd := &cobra.Command{
		Use:     "close [<number>]",
		Short:   "Close a pull request",
		Example: `  $ gv pr close 42 --comment "Superseded by #45"`,
		Args:    cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return setState(cmd.Context(), f, args, "closed", comment)
		},
	}

	cmd.Flags().StringVarP(&comment, "comment", "c", "", "Leave a closing `comment`")

	return cmd
}

// NewCmdReopen returns `gv pr reopen`.
func NewCmdReopen(f *cmdutil.Factory) *cobra.Command {
	var comment string

	cmd := &cobra.Command{
		Use:     "reopen [<number>]",
		Short:   "Reopen a closed pull request",
		Example: `  $ gv pr reopen 42`,
		Args:    cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return setState(cmd.Context(), f, args, "open", comment)
		},
	}

	cmd.Flags().StringVarP(&comment, "comment", "c", "", "Leave a `comment` when reopening")

	return cmd
}

func setState(ctx context.Context, f *cmdutil.Factory, args []string, state, comment string) error {
	arg := ""
	if len(args) > 0 {
		arg = args[0]
	}
	pr, repo, err := resolvePR(ctx, f, arg)
	if err != nil {
		return err
	}

	ios := f.IOStreams
	cs := ios.ColorScheme()

	if pr.Merged || pr.MergedAt != "" {
		return fmt.Errorf("pull request #%d is merged and its state cannot be changed", pr.Number)
	}
	if strings.EqualFold(plainState(pr), state) {
		fmt.Fprintf(ios.ErrOut, "%s Pull request #%d is already %s\n", cs.WarningIcon(), pr.Number, state)
		return nil
	}

	client, err := f.HTTPClient()
	if err != nil {
		return err
	}

	// Comment first: once closed, the discussion still accepts comments, but
	// posting beforehand keeps the timeline in a sensible order.
	if comment != "" {
		if _, err := api.CreateIssueComment(ctx, client, repo, int(pr.Number), api.CreateIssueCommentRequest{
			Body: api.Ptr(comment),
		}); err != nil {
			return err
		}
	}

	updated, err := api.UpdatePullRequest(ctx, client, repo, int(pr.Number), api.UpdatePullRequestAPIRequest{
		State: api.Ptr(state),
	})
	if err != nil {
		return err
	}

	verb := "Closed"
	if state == "open" {
		verb = "Reopened"
	}
	fmt.Fprintf(ios.ErrOut, "%s %s pull request %s\n",
		cs.SuccessIcon(), verb, cs.Bold(fmt.Sprintf("#%d", updated.Number)))
	return nil
}
