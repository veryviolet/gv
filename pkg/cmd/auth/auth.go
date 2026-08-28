// Package auth implements `gv auth`: storing and inspecting GitVerse credentials.
package auth

import (
	"github.com/spf13/cobra"

	"gitverse.ru/veryviolet/gv/internal/cmdutil"
)

// NewCmdAuth returns the `gv auth` command group.
func NewCmdAuth(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth <command>",
		Short: "Authenticate gv with GitVerse",
		Long: `Manage the credentials gv uses.

GitVerse issues personal access tokens from the web interface; gv stores the
token you paste in ` + "`$GV_CONFIG_DIR/hosts.yml`" + ` with 0600 permissions.
The environment variables GV_TOKEN and GITVERSE_TOKEN take precedence over the
stored credential and are never written to disk.`,
	}

	cmd.AddCommand(NewCmdLogin(f))
	cmd.AddCommand(NewCmdLogout(f))
	cmd.AddCommand(NewCmdStatus(f))
	cmd.AddCommand(NewCmdToken(f))
	cmd.AddCommand(NewCmdSetupGit(f))
	cmd.AddCommand(NewCmdGitCredential(f))

	return cmd
}
