package auth

import (
	"bufio"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"gitverse.ru/veryviolet/gv/internal/cmdutil"
	"gitverse.ru/veryviolet/gv/internal/config"
)

// gitCredentialUser is sent when gv has no login recorded. GitVerse, like most
// token-based hosts, ignores the username for token authentication.
const gitCredentialUser = "x-access-token"

// NewCmdGitCredential returns the hidden `gv auth git-credential` command that
// implements git's credential helper protocol.
func NewCmdGitCredential(f *cmdutil.Factory) *cobra.Command {
	return &cobra.Command{
		Use:    "git-credential",
		Short:  "Implement the git credential helper protocol",
		Hidden: true,
		Args:   cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// git asks helpers to "store" and "erase" too; gv owns the token
			// elsewhere, so those are deliberately no-ops.
			if args[0] != "get" {
				return nil
			}

			input, err := readCredentialRequest(cmd.InOrStdin())
			if err != nil {
				return err
			}
			if proto := input["protocol"]; proto != "" && proto != "https" {
				// Only HTTPS credentials are ours to supply.
				return cmdutil.ErrSilent
			}

			host := config.Host(input["host"])
			cfg, err := f.Config()
			if err != nil {
				return err
			}
			token, _ := cfg.Token(host)
			if token == "" {
				return cmdutil.ErrSilent
			}

			username := cfg.User(host)
			if username == "" {
				username = gitCredentialUser
			}

			out := f.IOStreams.Out
			fmt.Fprintf(out, "protocol=https\n")
			fmt.Fprintf(out, "host=%s\n", host)
			fmt.Fprintf(out, "username=%s\n", username)
			fmt.Fprintf(out, "password=%s\n", token)
			return nil
		},
	}
}

// readCredentialRequest parses the key=value lines git writes to a helper,
// terminated by a blank line or EOF.
func readCredentialRequest(r io.Reader) (map[string]string, error) {
	fields := map[string]string{}
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			break
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		fields[key] = value
	}
	return fields, scanner.Err()
}
