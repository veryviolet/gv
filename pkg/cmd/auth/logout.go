package auth

import (
	"fmt"

	"github.com/spf13/cobra"

	"gitverse.ru/veryviolet/gv/internal/cmdutil"
	"gitverse.ru/veryviolet/gv/internal/config"
)

// NewCmdLogout returns `gv auth logout`.
func NewCmdLogout(f *cmdutil.Factory) *cobra.Command {
	var hostname string

	cmd := &cobra.Command{
		Use:   "logout",
		Short: "Remove stored credentials for a host",
		Long: `Forget the token stored for a GitVerse host.

Tokens supplied through GV_TOKEN or GITVERSE_TOKEN are not stored by gv and
cannot be removed this way; unset the variable instead.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
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

			if _, source := cfg.Token(host); source != "" && source != config.SourceFile {
				return fmt.Errorf("the token for %s comes from %s; unset that variable to log out", host, source)
			}
			if cfg.User(host) == "" {
				if token, _ := cfg.Token(host); token == "" {
					return fmt.Errorf("not logged in to %s", host)
				}
			}

			user := cfg.User(host)
			cfg.RemoveHost(host)
			if err := cfg.Write(); err != nil {
				return err
			}

			if user != "" {
				fmt.Fprintf(ios.ErrOut, "%s Logged out of %s as %s\n", cs.SuccessIcon(), host, cs.Bold(user))
			} else {
				fmt.Fprintf(ios.ErrOut, "%s Logged out of %s\n", cs.SuccessIcon(), host)
			}
			return nil
		},
	}

	cmd.Flags().StringVarP(&hostname, "hostname", "h", "", "The host to log out of")

	return cmd
}
