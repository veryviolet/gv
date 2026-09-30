package auth

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"

	"gitverse.ru/veryviolet/gv/internal/api"
	"gitverse.ru/veryviolet/gv/internal/build"
	"gitverse.ru/veryviolet/gv/internal/cmdutil"
	"gitverse.ru/veryviolet/gv/internal/config"
)

// NewCmdDockerLogin returns `gv auth docker-login`.
func NewCmdDockerLogin(f *cmdutil.Factory) *cobra.Command {
	var (
		hostname string
		engine   string
		verify   bool
	)

	cmd := &cobra.Command{
		Use:   "docker-login",
		Short: "Log the container engine in to the GitVerse registry",
		Long: `Authenticate docker or podman against the GitVerse container registry
using the token gv already holds.

The registry lives on the same host as the repositories, so images are named
` + "`<host>/<owner>/<image>`" + `. The token is passed to the engine on standard
input and never appears in the process list or the shell history.`,
		Example: `  $ gv auth docker-login
  $ gv auth docker-login --engine podman`,
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
			token, _ := cfg.Token(host)
			if token == "" {
				return cmdutil.NewAuthError(fmt.Errorf("no token found for %s", host))
			}

			username := cfg.User(host)
			if username == "" {
				// The registry ignores the user name for token auth, but the
				// engine insists on one.
				username = "x-access-token"
			}
			registry := api.RegistryHost(host)

			if verify {
				client := api.NewRegistryClient(host, username, token, "gv/"+build.Version)
				if err := client.Ping(cmd.Context()); err != nil {
					return fmt.Errorf("the registry rejected the credentials: %w", err)
				}
				fmt.Fprintf(ios.ErrOut, "%s Registry %s accepted the token\n", cs.SuccessIcon(), registry)
			}

			bin, err := exec.LookPath(engine)
			if err != nil {
				return fmt.Errorf("%s not found in PATH: %w", engine, err)
			}

			c := exec.CommandContext(cmd.Context(), bin, "login", "--username", username, "--password-stdin", registry)
			c.Stdin = strings.NewReader(token)
			c.Stdout = ios.ErrOut
			c.Stderr = ios.ErrOut
			if err := c.Run(); err != nil {
				return fmt.Errorf("%s login failed: %w", engine, err)
			}

			fmt.Fprintf(ios.ErrOut, "%s Logged %s in to %s as %s\n",
				cs.SuccessIcon(), engine, cs.Bold(registry), username)
			fmt.Fprintf(ios.ErrOut, "  Images are named %s\n",
				cs.Bold(fmt.Sprintf("%s/%s/<image>:<tag>", registry, username)))
			return nil
		},
	}

	cmd.Flags().StringVarP(&hostname, "hostname", "h", "", "The GitVerse host whose registry to log in to")
	cmd.Flags().StringVar(&engine, "engine", "docker", "Container engine to use: {docker|podman|nerdctl}")
	cmd.Flags().BoolVar(&verify, "verify", false, "Check the token against the registry before logging in")

	return cmd
}
