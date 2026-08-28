package auth

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"gitverse.ru/veryviolet/gv/internal/cmdutil"
	"gitverse.ru/veryviolet/gv/internal/config"
)

// NewCmdSetupGit returns `gv auth setup-git`.
func NewCmdSetupGit(f *cmdutil.Factory) *cobra.Command {
	var hostname string

	cmd := &cobra.Command{
		Use:   "setup-git",
		Short: "Configure git to authenticate with the stored token",
		Long: `Register gv as a git credential helper for a GitVerse host.

Afterwards ` + "`git clone`" + `, ` + "`git fetch`" + ` and ` + "`git push`" + ` over
HTTPS use the token gv holds, so no separate git credential setup is needed.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			ios := f.IOStreams
			cs := ios.ColorScheme()

			cfg, err := f.Config()
			if err != nil {
				return err
			}
			host := f.Host()
			if hostname != "" {
				host = config.Host(hostname)
			}
			if token, _ := cfg.Token(host); token == "" {
				return cmdutil.NewAuthError(fmt.Errorf("no token found for %s", host))
			}

			exe, err := os.Executable()
			if err != nil {
				return fmt.Errorf("could not determine the gv executable path: %w", err)
			}

			key := fmt.Sprintf("credential.https://%s.helper", host)
			// An empty first value clears helpers inherited from a broader
			// scope, so gv is the only helper consulted for this host.
			if err := f.GitClient.ConfigSetGlobal(ctx, key, ""); err != nil {
				return err
			}
			helper := fmt.Sprintf("!%s auth git-credential", shellQuote(exe))
			if err := f.GitClient.Run(ctx, "config", "--global", "--add", key, helper); err != nil {
				return err
			}

			fmt.Fprintf(ios.ErrOut, "%s git configured to use gv for %s\n", cs.SuccessIcon(), host)
			return nil
		},
	}

	cmd.Flags().StringVarP(&hostname, "hostname", "h", "", "The host to configure")

	return cmd
}

// shellQuote protects a path containing spaces, since git runs helper values
// beginning with "!" through the shell.
func shellQuote(s string) string {
	if !strings.ContainsAny(s, " \t'\"\\$`") {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
