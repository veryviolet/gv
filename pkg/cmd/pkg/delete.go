package pkg

import (
	"bufio"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"gitverse.ru/veryviolet/gv/internal/api"
	"gitverse.ru/veryviolet/gv/internal/cmdutil"
)

// NewCmdDelete returns `gv package delete`.
func NewCmdDelete(f *cmdutil.Factory) *cobra.Command {
	var (
		pkgType   string
		isOrg     bool
		versionID string
		confirmed bool
	)

	cmd := &cobra.Command{
		Use:   "delete [<owner>/]<package>",
		Short: "Delete a package or one of its versions",
		Long: `Delete a whole package, or a single version with --version.

This cannot be undone: an image deleted from the registry is gone, and anyone
pulling that tag will start getting errors.`,
		Example: `  $ gv package delete gv --version 42
  $ gv package delete veryviolet/gv --yes`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			owner, name, err := splitPackageName(cmd.Context(), f, args[0])
			if err != nil {
				return err
			}

			ios := f.IOStreams
			cs := ios.ColorScheme()
			target := fmt.Sprintf("%s/%s", owner, name)
			what := "package " + target
			if versionID != "" {
				what = fmt.Sprintf("version %s of %s", versionID, target)
			}

			if !confirmed {
				if !ios.CanPrompt() {
					return cmdutil.FlagErrorf("--yes is required when not running interactively")
				}
				fmt.Fprintf(ios.ErrOut, "%s This will permanently delete %s.\n", cs.WarningIcon(), cs.Bold(what))
				fmt.Fprintf(ios.ErrOut, "Type %s to confirm: ", cs.Bold(name))
				scanner := bufio.NewScanner(ios.In)
				if !scanner.Scan() || strings.TrimSpace(scanner.Text()) != name {
					fmt.Fprintf(ios.ErrOut, "%s Names did not match; nothing was deleted.\n", cs.FailureIcon())
					return cmdutil.ErrCancel
				}
			}

			client, err := f.HTTPClient()
			if err != nil {
				return err
			}
			pkgOwner := api.PackageOwner{Name: owner, IsOrg: isOrg}

			if versionID != "" {
				err = api.DeletePackageVersion(cmd.Context(), client, pkgOwner, pkgType, name, versionID)
			} else {
				err = api.DeletePackage(cmd.Context(), client, pkgOwner, pkgType, name)
			}
			if err != nil {
				return err
			}

			fmt.Fprintf(ios.ErrOut, "%s Deleted %s\n", cs.SuccessIcon(), what)
			return nil
		},
	}

	cmd.Flags().StringVar(&pkgType, "type", DefaultPackageType, "Package `type`: {container|generic|debian|rpm|maven|npm|pypi|helm|nuget|go}")
	cmd.Flags().BoolVar(&isOrg, "org", false, "The owner is an organization")
	cmd.Flags().StringVar(&versionID, "version", "", "Delete only this version `id`")
	cmd.Flags().BoolVar(&confirmed, "yes", false, "Skip the confirmation prompt")

	return cmd
}
