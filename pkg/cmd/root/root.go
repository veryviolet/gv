// Package root assembles gv's command tree.
package root

import (
	"github.com/spf13/cobra"

	"gitverse.ru/veryviolet/gv/internal/cmdutil"
	apiCmd "gitverse.ru/veryviolet/gv/pkg/cmd/api"
	authCmd "gitverse.ru/veryviolet/gv/pkg/cmd/auth"
	issueCmd "gitverse.ru/veryviolet/gv/pkg/cmd/issue"
	pkgCmd "gitverse.ru/veryviolet/gv/pkg/cmd/pkg"
	prCmd "gitverse.ru/veryviolet/gv/pkg/cmd/pr"
	releaseCmd "gitverse.ru/veryviolet/gv/pkg/cmd/release"
	repoCmd "gitverse.ru/veryviolet/gv/pkg/cmd/repo"
	versionCmd "gitverse.ru/veryviolet/gv/pkg/cmd/version"
)

// NewCmdRoot builds the root command with every subcommand attached.
func NewCmdRoot(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "gv <command> <subcommand> [flags]",
		Short: "GitVerse from the command line",
		Long:  "Work with GitVerse repositories, pull requests and issues without leaving the terminal.",
		Example: `  $ gv repo view veryviolet/gv
  $ gv pr list
  $ gv issue create --title "Bug" --body "..."
  $ gv api /user`,
		SilenceErrors: true,
		SilenceUsage:  true,
		Annotations: map[string]string{
			"versionInfo": versionCmd.Format(),
		},
	}

	cmd.SetOut(f.IOStreams.Out)
	cmd.SetErr(f.IOStreams.ErrOut)

	cmd.PersistentFlags().String("hostname", "", "GitVerse hostname to operate against")
	cmd.PersistentFlags().Bool("help", false, "Show help for command")
	cmd.Flags().BoolP("version", "v", false, "Show gv version")

	cmd.PersistentPreRunE = func(c *cobra.Command, args []string) error {
		if host, _ := c.Flags().GetString("hostname"); host != "" {
			f.OverrideHost(host)
		}
		return nil
	}

	cmd.RunE = func(c *cobra.Command, args []string) error {
		if v, _ := c.Flags().GetBool("version"); v {
			c.Print(versionCmd.Format())
			return nil
		}
		if len(args) > 0 {
			return cmdutil.FlagErrorf("unknown command %q for %q", args[0], c.CommandPath())
		}
		return c.Help()
	}

	cmd.AddCommand(versionCmd.NewCmdVersion(f))
	cmd.AddCommand(authCmd.NewCmdAuth(f))
	cmd.AddCommand(repoCmd.NewCmdRepo(f))
	cmd.AddCommand(prCmd.NewCmdPR(f))
	cmd.AddCommand(issueCmd.NewCmdIssue(f))
	cmd.AddCommand(releaseCmd.NewCmdRelease(f))
	cmd.AddCommand(pkgCmd.NewCmdPackage(f))
	cmd.AddCommand(apiCmd.NewCmdAPI(f))

	return cmd
}
