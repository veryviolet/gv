package auth

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"gitverse.ru/veryviolet/gv/internal/api"
	"gitverse.ru/veryviolet/gv/internal/build"
	"gitverse.ru/veryviolet/gv/internal/cmdutil"
	"gitverse.ru/veryviolet/gv/internal/config"
)

type statusOptions struct {
	factory *cmdutil.Factory

	hostname   string
	showToken  bool
	skipVerify bool
}

// NewCmdStatus returns `gv auth status`.
func NewCmdStatus(f *cmdutil.Factory) *cobra.Command {
	opts := statusOptions{factory: f}

	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show the authentication state",
		Long: `Report which hosts gv has credentials for, where each credential came
from, and whether it is still accepted by the API.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return statusRun(cmd.Context(), &opts)
		},
	}

	cmd.Flags().StringVarP(&opts.hostname, "hostname", "h", "", "Check a single host")
	cmd.Flags().BoolVarP(&opts.showToken, "show-token", "t", false, "Display the token in the output")
	cmd.Flags().BoolVar(&opts.skipVerify, "skip-verify", false, "Do not contact the API")

	return cmd
}

func statusRun(ctx context.Context, opts *statusOptions) error {
	f := opts.factory
	ios := f.IOStreams
	cs := ios.ColorScheme()

	cfg, err := f.Config()
	if err != nil {
		return err
	}

	hosts := cfg.Hosts()
	// A token in the environment counts as a configured host even when
	// hosts.yml has never been written.
	current := f.Host()
	if opts.hostname != "" {
		current = config.Host(opts.hostname)
	}
	if !contains(hosts, current) {
		if token, _ := cfg.Token(current); token != "" {
			hosts = append(hosts, current)
		}
	}
	if opts.hostname != "" {
		hosts = filter(hosts, current)
	}

	if len(hosts) == 0 {
		fmt.Fprintf(ios.ErrOut, "%s You are not logged in to any GitVerse host.\n", cs.FailureIcon())
		fmt.Fprintf(ios.ErrOut, "  Run %s to authenticate.\n", cs.Bold("gv auth login"))
		return cmdutil.ErrSilent
	}

	failed := false
	for _, host := range hosts {
		token, source := cfg.Token(host)
		fmt.Fprintf(ios.Out, "%s\n", cs.Bold(host))

		if token == "" {
			fmt.Fprintf(ios.Out, "  %s No token found\n", cs.FailureIcon())
			failed = true
			continue
		}

		login := cfg.User(host)
		if !opts.skipVerify {
			user, err := verifyStatusToken(ctx, host, token)
			if err != nil {
				fmt.Fprintf(ios.Out, "  %s %s: %v\n", cs.FailureIcon(), host, err)
				failed = true
				continue
			}
			login = user.DisplayName()
		}

		if login != "" {
			fmt.Fprintf(ios.Out, "  %s Logged in as %s\n", cs.SuccessIcon(), cs.Bold(login))
		} else {
			fmt.Fprintf(ios.Out, "  %s Token present\n", cs.SuccessIcon())
		}
		fmt.Fprintf(ios.Out, "  - Token source: %s\n", source)
		fmt.Fprintf(ios.Out, "  - Git protocol: %s\n", cfg.GitProtocol(host))
		fmt.Fprintf(ios.Out, "  - API endpoint: %s\n", api.EndpointForHost(host))
		fmt.Fprintf(ios.Out, "  - Token: %s\n", displayToken(token, opts.showToken))
	}

	if failed {
		return cmdutil.ErrSilent
	}
	return nil
}

func verifyStatusToken(ctx context.Context, host, token string) (*api.User, error) {
	client, err := api.NewClientForHost(host, token, "gv/"+build.Version)
	if err != nil {
		return nil, err
	}
	return api.CurrentUser(ctx, client)
}

// displayToken keeps a stored credential out of terminal scrollback unless the
// user explicitly asked to see it.
func displayToken(token string, show bool) string {
	if show {
		return token
	}
	if len(token) <= 4 {
		return strings.Repeat("*", len(token))
	}
	return strings.Repeat("*", len(token)-4) + token[len(token)-4:]
}

func contains(list []string, want string) bool {
	for _, v := range list {
		if v == want {
			return true
		}
	}
	return false
}

func filter(list []string, want string) []string {
	var out []string
	for _, v := range list {
		if v == want {
			out = append(out, v)
		}
	}
	return out
}
