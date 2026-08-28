package auth

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"gitverse.ru/veryviolet/gv/internal/api"
	"gitverse.ru/veryviolet/gv/internal/build"
	"gitverse.ru/veryviolet/gv/internal/cmdutil"
	"gitverse.ru/veryviolet/gv/internal/config"
)

// tokenSettingsPath is where GitVerse issues personal access tokens.
const tokenSettingsPath = "/settings/tokens"

type loginOptions struct {
	factory *cmdutil.Factory

	hostname    string
	withToken   bool
	gitProtocol string
	skipVerify  bool
}

// NewCmdLogin returns `gv auth login`.
func NewCmdLogin(f *cmdutil.Factory) *cobra.Command {
	opts := loginOptions{factory: f}

	cmd := &cobra.Command{
		Use:   "login",
		Short: "Authenticate with a GitVerse host",
		Long: `Store a GitVerse personal access token.

GitVerse has no OAuth device flow, so the token is created in the web interface
and pasted here. Create one at https://gitverse.ru/settings/tokens and grant it
the scopes the commands you intend to use require.`,
		Example: `  # paste the token interactively
  $ gv auth login

  # read the token from a file, for scripts and CI
  $ gv auth login --with-token < token.txt`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if opts.gitProtocol != "" && opts.gitProtocol != "https" && opts.gitProtocol != "ssh" {
				return cmdutil.FlagErrorf("--git-protocol must be either https or ssh")
			}
			return loginRun(cmd.Context(), &opts)
		},
	}

	cmd.Flags().StringVarP(&opts.hostname, "hostname", "h", "", "The GitVerse host to authenticate with")
	cmd.Flags().BoolVar(&opts.withToken, "with-token", false, "Read the token from standard input")
	cmd.Flags().StringVarP(&opts.gitProtocol, "git-protocol", "p", "", "The protocol to use for git operations: {https|ssh}")
	cmd.Flags().BoolVar(&opts.skipVerify, "skip-verify", false, "Store the token without checking it against the API")

	return cmd
}

func loginRun(ctx context.Context, opts *loginOptions) error {
	f := opts.factory
	ios := f.IOStreams
	cs := ios.ColorScheme()

	host := f.Host()
	if opts.hostname != "" {
		host = config.Host(opts.hostname)
	}

	cfg, err := f.Config()
	if err != nil {
		return err
	}

	if token, source := cfg.Token(host); token != "" && source != config.SourceFile {
		return fmt.Errorf("the %s environment variable is set; unset it to log in with a stored token", source)
	}

	token, err := readToken(opts, host)
	if err != nil {
		return err
	}
	if token == "" {
		return cmdutil.FlagErrorf("no token provided")
	}

	login := ""
	if !opts.skipVerify {
		user, err := verifyToken(ctx, host, token)
		if err != nil {
			return fmt.Errorf("the token was rejected by %s: %w", host, err)
		}
		login = user.DisplayName()
	}

	cfg.SetToken(host, login, token)
	if opts.gitProtocol != "" {
		cfg.SetGitProtocol(host, opts.gitProtocol)
	}
	if err := cfg.Write(); err != nil {
		return fmt.Errorf("failed to save the token: %w", err)
	}

	if login != "" {
		fmt.Fprintf(ios.ErrOut, "%s Logged in to %s as %s\n", cs.SuccessIcon(), host, cs.Bold(login))
	} else {
		fmt.Fprintf(ios.ErrOut, "%s Token stored for %s (not verified)\n", cs.SuccessIcon(), host)
	}
	fmt.Fprintf(ios.ErrOut, "  Configure git to use it with %s\n", cs.Bold("gv auth setup-git"))
	return nil
}

// readToken obtains the token from stdin or an interactive prompt.
func readToken(opts *loginOptions, host string) (string, error) {
	ios := opts.factory.IOStreams

	if opts.withToken || !ios.CanPrompt() {
		data, err := readAll(ios.In)
		if err != nil {
			return "", err
		}
		return strings.TrimSpace(data), nil
	}

	fmt.Fprintf(ios.ErrOut, "Create a token at https://%s%s\n", host, tokenSettingsPath)
	fmt.Fprint(ios.ErrOut, "Paste your authentication token: ")

	// Reading without echo needs the real terminal file descriptor; fall back
	// to a plain read when stdin is not one.
	if f, ok := ios.In.(*os.File); ok && term.IsTerminal(int(f.Fd())) {
		data, err := term.ReadPassword(int(f.Fd()))
		fmt.Fprintln(ios.ErrOut)
		if err != nil {
			return "", err
		}
		return strings.TrimSpace(string(data)), nil
	}

	data, err := readAll(ios.In)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(data), nil
}

func readAll(r io.Reader) (string, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// verifyToken checks a candidate credential without disturbing stored config.
func verifyToken(ctx context.Context, host, token string) (*api.User, error) {
	client, err := api.NewClientForHost(host, token, "gv/"+build.Version)
	if err != nil {
		return nil, err
	}
	return api.CurrentUser(ctx, client)
}
