// Command gv is a command-line client for GitVerse.
package main

import (
	"errors"
	"fmt"
	"os"

	"gitverse.ru/veryviolet/gv/internal/cmdutil"
	"gitverse.ru/veryviolet/gv/pkg/cmd/root"
)

// Exit codes, chosen to match the conventions gh established.
const (
	exitOK     = 0
	exitError  = 1
	exitCancel = 2
	exitAuth   = 4
)

func main() {
	os.Exit(run())
}

func run() int {
	f := cmdutil.NewFactory()
	stderr := f.IOStreams.ErrOut
	cs := f.IOStreams.ColorScheme()

	rootCmd := root.NewCmdRoot(f)
	rootCmd.SetArgs(os.Args[1:])

	if _, err := rootCmd.ExecuteC(); err != nil {
		var flagErr *cmdutil.FlagError
		var authErr *cmdutil.AuthError

		switch {
		case errors.Is(err, cmdutil.ErrSilent):
			return exitError
		case errors.Is(err, cmdutil.ErrCancel):
			return exitCancel
		case errors.As(err, &authErr):
			fmt.Fprintf(stderr, "%s %s\n", cs.FailureIcon(), err)
			fmt.Fprintf(stderr, "Run %s to authenticate.\n", cs.Bold("gv auth login"))
			return exitAuth
		case errors.As(err, &flagErr):
			fmt.Fprintf(stderr, "%s %s\n", cs.FailureIcon(), err)
			fmt.Fprintf(stderr, "\nRun %s for usage.\n", cs.Bold("gv --help"))
			return exitError
		default:
			fmt.Fprintf(stderr, "%s %s\n", cs.FailureIcon(), err)
			return exitError
		}
	}
	return exitOK
}
