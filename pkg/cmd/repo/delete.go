package repo

import (
	"bufio"
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"gitverse.ru/veryviolet/gv/internal/api"
	"gitverse.ru/veryviolet/gv/internal/cmdutil"
	"gitverse.ru/veryviolet/gv/internal/gvrepo"
)

// NewCmdDelete returns `gv repo delete`.
func NewCmdDelete(f *cmdutil.Factory) *cobra.Command {
	var confirmed bool

	cmd := &cobra.Command{
		Use:   "delete [<repository>]",
		Short: "Delete a repository",
		Long: `Delete a repository on GitVerse. This cannot be undone.

Interactively you are asked to retype the full repository name. In scripts,
pass --yes to skip the prompt.`,
		Example: `  $ gv repo delete veryviolet/scratch
  $ gv repo delete veryviolet/scratch --yes`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			arg := ""
			if len(args) > 0 {
				arg = args[0]
			}
			repo, err := cmdutil.RepoFromArg(f, arg)
			if err != nil {
				return err
			}
			return deleteRun(cmd.Context(), f, repo, confirmed)
		},
	}

	cmd.Flags().BoolVar(&confirmed, "yes", false, "Skip the confirmation prompt")

	return cmd
}

func deleteRun(ctx context.Context, f *cmdutil.Factory, repo gvrepo.Interface, confirmed bool) error {
	ios := f.IOStreams
	cs := ios.ColorScheme()
	fullName := gvrepo.FullName(repo)

	if !confirmed {
		if !ios.CanPrompt() {
			return cmdutil.FlagErrorf("--yes is required when not running interactively")
		}
		fmt.Fprintf(ios.ErrOut, "%s This will permanently delete %s.\n", cs.WarningIcon(), cs.Bold(fullName))
		fmt.Fprintf(ios.ErrOut, "Type %s to confirm: ", cs.Bold(fullName))

		scanner := bufio.NewScanner(ios.In)
		if !scanner.Scan() {
			return cmdutil.ErrCancel
		}
		if strings.TrimSpace(scanner.Text()) != fullName {
			fmt.Fprintf(ios.ErrOut, "%s Names did not match; nothing was deleted.\n", cs.FailureIcon())
			return cmdutil.ErrCancel
		}
	}

	client, err := f.HTTPClient()
	if err != nil {
		return err
	}
	if err := api.DeleteRepository(ctx, client, repo); err != nil {
		return err
	}

	fmt.Fprintf(ios.ErrOut, "%s Deleted %s\n", cs.SuccessIcon(), cs.Bold(fullName))
	return nil
}
