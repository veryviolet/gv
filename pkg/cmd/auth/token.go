package auth

import (
	"fmt"

	"github.com/spf13/cobra"

	"gitverse.ru/veryviolet/gv/internal/cmdutil"
	"gitverse.ru/veryviolet/gv/internal/config"
)

// NewCmdToken returns `gv auth token`.
func NewCmdToken(f *cmdutil.Factory) *cobra.Command {
	var hostname string

	cmd := &cobra.Command{
		Use:   "token",
		Short: "Print the token gv would use",
		Long: `Print the authentication token gv resolves for a host.

Useful for feeding other tools, e.g. ` + "`curl -H \"Authorization: Bearer $(gv auth token)\"`" + `.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := f.Config()
			if err != nil {
				return err
			}
			host := f.Host()
			if hostname != "" {
				host = config.Host(hostname)
			}
			token, _ := cfg.Token(host)
			if token == "" {
				return cmdutil.NewAuthError(fmt.Errorf("no token found for %s", host))
			}
			fmt.Fprintln(f.IOStreams.Out, token)
			return nil
		},
	}

	cmd.Flags().StringVarP(&hostname, "hostname", "h", "", "The host to look up")

	return cmd
}
