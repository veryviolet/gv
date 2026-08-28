// Package repo implements `gv repo`.
package repo

import (
	"github.com/spf13/cobra"

	"gitverse.ru/veryviolet/gv/internal/cmdutil"
)

// NewCmdRepo returns the `gv repo` command group.
func NewCmdRepo(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "repo <command>",
		Short:   "Work with GitVerse repositories",
		Aliases: []string{"repository"},
		Long: `Create, inspect, clone and fork repositories.

Commands that take a repository accept it as "OWNER/REPO"; run inside a git
repository with a GitVerse remote and the argument can be omitted.`,
	}

	cmd.AddCommand(NewCmdView(f))
	cmd.AddCommand(NewCmdList(f))
	cmd.AddCommand(NewCmdCreate(f))
	cmd.AddCommand(NewCmdClone(f))
	cmd.AddCommand(NewCmdFork(f))
	cmd.AddCommand(NewCmdDelete(f))

	return cmd
}
