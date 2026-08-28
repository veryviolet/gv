package release

import (
	"bufio"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"gitverse.ru/veryviolet/gv/internal/api"
	"gitverse.ru/veryviolet/gv/internal/cmdutil"
)

// NewCmdDelete returns `gv release delete`.
func NewCmdDelete(f *cmdutil.Factory) *cobra.Command {
	var confirmed bool

	cmd := &cobra.Command{
		Use:     "delete <tag>",
		Short:   "Delete a release",
		Long:    `Delete a release. The git tag itself is left alone.`,
		Example: `  $ gv release delete v0.1.0 --yes`,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			tag := args[0]
			rel, repo, err := resolveRelease(cmd.Context(), f, tag)
			if err != nil {
				return err
			}

			ios := f.IOStreams
			cs := ios.ColorScheme()

			if !confirmed {
				if !ios.CanPrompt() {
					return cmdutil.FlagErrorf("--yes is required when not running interactively")
				}
				fmt.Fprintf(ios.ErrOut, "%s Delete release %s with %d asset(s)? Type the tag to confirm: ",
					cs.WarningIcon(), cs.Bold(rel.TagName), len(rel.Assets))
				scanner := bufio.NewScanner(ios.In)
				if !scanner.Scan() || strings.TrimSpace(scanner.Text()) != rel.TagName {
					fmt.Fprintf(ios.ErrOut, "%s Nothing was deleted.\n", cs.FailureIcon())
					return cmdutil.ErrCancel
				}
			}

			client, err := f.HTTPClient()
			if err != nil {
				return err
			}
			if err := api.DeleteRelease(cmd.Context(), client, repo, rel.ID); err != nil {
				return err
			}
			fmt.Fprintf(ios.ErrOut, "%s Deleted release %s\n", cs.SuccessIcon(), cs.Bold(rel.TagName))
			return nil
		},
	}

	cmd.Flags().BoolVar(&confirmed, "yes", false, "Skip the confirmation prompt")

	return cmd
}
