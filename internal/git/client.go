// Package git wraps the git command line. gv shells out rather than linking a
// git implementation so that the user's own config, credential helpers and
// hooks apply exactly as they do on the command line.
package git

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// ErrNotOnBranch is returned by CurrentBranch in a detached HEAD state.
var ErrNotOnBranch = errors.New("git: not on any branch")

// ErrNotARepository is returned when the working directory is not a git repo.
var ErrNotARepository = errors.New("not a git repository")

// Error carries the exit status and stderr of a failed git invocation.
type Error struct {
	Args     []string
	Stderr   string
	ExitCode int
	err      error
}

func (e *Error) Error() string {
	msg := strings.TrimSpace(e.Stderr)
	if msg == "" {
		return fmt.Sprintf("git %s: %v", strings.Join(e.Args, " "), e.err)
	}
	return fmt.Sprintf("git %s: %s", strings.Join(e.Args, " "), msg)
}

func (e *Error) Unwrap() error { return e.err }

// Client runs git commands, optionally pinned to a repository directory.
type Client struct {
	// GitPath is the git executable; resolved from PATH when empty.
	GitPath string
	// RepoDir is the working directory for commands; the process CWD when empty.
	RepoDir string
	// Stderr receives git's stderr for commands run interactively.
	Stderr io.Writer
	// Stdout receives git's stdout for commands run interactively.
	Stdout io.Writer
	// Stdin is passed to commands run interactively.
	Stdin io.Reader
}

func (c *Client) path() (string, error) {
	if c.GitPath != "" {
		return c.GitPath, nil
	}
	p, err := exec.LookPath("git")
	if err != nil {
		return "", fmt.Errorf("unable to find git executable in PATH: %w", err)
	}
	c.GitPath = p
	return p, nil
}

// Command builds a git command without running it.
func (c *Client) Command(ctx context.Context, args ...string) (*exec.Cmd, error) {
	gitPath, err := c.path()
	if err != nil {
		return nil, err
	}
	if c.RepoDir != "" {
		args = append([]string{"-C", c.RepoDir}, args...)
	}
	cmd := exec.CommandContext(ctx, gitPath, args...)
	cmd.Stderr = c.Stderr
	cmd.Stdout = c.Stdout
	cmd.Stdin = c.Stdin
	return cmd, nil
}

// Run executes git and streams its output to the client's streams.
func (c *Client) Run(ctx context.Context, args ...string) error {
	cmd, err := c.Command(ctx, args...)
	if err != nil {
		return err
	}
	var stderr bytes.Buffer
	if cmd.Stderr == nil {
		cmd.Stderr = &stderr
	} else {
		cmd.Stderr = io.MultiWriter(cmd.Stderr, &stderr)
	}
	if err := cmd.Run(); err != nil {
		return wrapErr(args, stderr.String(), err)
	}
	return nil
}

// Output runs git and captures stdout, trimming the trailing newline.
func (c *Client) Output(ctx context.Context, args ...string) (string, error) {
	cmd, err := c.Command(ctx, args...)
	if err != nil {
		return "", err
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.Stdin = nil
	if err := cmd.Run(); err != nil {
		return "", wrapErr(args, stderr.String(), err)
	}
	return strings.TrimRight(stdout.String(), "\n"), nil
}

func wrapErr(args []string, stderr string, err error) error {
	e := &Error{Args: args, Stderr: stderr, err: err}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		e.ExitCode = exitErr.ExitCode()
	}
	return e
}

// ToplevelDir returns the root of the repository containing the working directory.
func (c *Client) ToplevelDir(ctx context.Context) (string, error) {
	out, err := c.Output(ctx, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", ErrNotARepository
	}
	return filepath.ToSlash(out), nil
}

// IsRepository reports whether the working directory is inside a git repository.
func (c *Client) IsRepository(ctx context.Context) bool {
	_, err := c.Output(ctx, "rev-parse", "--git-dir")
	return err == nil
}

// CurrentBranch returns the checked-out branch name.
func (c *Client) CurrentBranch(ctx context.Context) (string, error) {
	out, err := c.Output(ctx, "symbolic-ref", "--quiet", "HEAD")
	if err != nil {
		var gitErr *Error
		if errors.As(err, &gitErr) && gitErr.ExitCode == 1 && gitErr.Stderr == "" {
			return "", ErrNotOnBranch
		}
		return "", err
	}
	return strings.TrimPrefix(out, "refs/heads/"), nil
}

// HasLocalBranch reports whether a local branch of that name exists.
func (c *Client) HasLocalBranch(ctx context.Context, branch string) bool {
	_, err := c.Output(ctx, "rev-parse", "--verify", "refs/heads/"+branch)
	return err == nil
}

// CheckoutBranch switches to an existing local branch.
func (c *Client) CheckoutBranch(ctx context.Context, branch string) error {
	return c.Run(ctx, "checkout", branch)
}

// Fetch fetches refspecs from a remote.
func (c *Client) Fetch(ctx context.Context, remote string, refspecs ...string) error {
	args := append([]string{"fetch", remote}, refspecs...)
	return c.Run(ctx, args...)
}

// Push pushes a ref to a remote, setting upstream tracking.
func (c *Client) Push(ctx context.Context, remote, ref string) error {
	return c.Run(ctx, "push", "--set-upstream", remote, ref)
}

// Clone clones a repository, returning the directory it landed in.
func (c *Client) Clone(ctx context.Context, cloneURL string, args []string) (string, error) {
	cloneArgs, target := parseCloneArgs(args)
	cloneArgs = append(cloneArgs, cloneURL)
	if target != "" {
		cloneArgs = append(cloneArgs, target)
	} else {
		target = strings.TrimSuffix(filepath.Base(strings.TrimSuffix(cloneURL, "/")), ".git")
	}
	cloneArgs = append([]string{"clone"}, cloneArgs...)
	if err := c.Run(ctx, cloneArgs...); err != nil {
		return "", err
	}
	return target, nil
}

// parseCloneArgs splits user-supplied `git clone` arguments into flags and an
// optional target directory.
func parseCloneArgs(args []string) (flags []string, target string) {
	for i := 0; i < len(args); i++ {
		a := args[i]
		if strings.HasPrefix(a, "-") {
			flags = append(flags, a)
			continue
		}
		if target == "" {
			target = a
			continue
		}
		flags = append(flags, a)
	}
	return
}

// AddRemote registers a remote and fetches from it.
func (c *Client) AddRemote(ctx context.Context, name, url string) error {
	return c.Run(ctx, "remote", "add", name, url)
}

// SetRemoteURL points an existing remote at a new URL.
func (c *Client) SetRemoteURL(ctx context.Context, name, url string) error {
	return c.Run(ctx, "remote", "set-url", name, url)
}

// ConfigGet reads a git config value; a missing key returns an empty string.
func (c *Client) ConfigGet(ctx context.Context, key string) (string, error) {
	out, err := c.Output(ctx, "config", "--get", key)
	if err != nil {
		var gitErr *Error
		if errors.As(err, &gitErr) && gitErr.ExitCode == 1 {
			return "", nil
		}
		return "", err
	}
	return out, nil
}

// ConfigSetGlobal writes a value into the user's global git config.
func (c *Client) ConfigSetGlobal(ctx context.Context, key, value string) error {
	return c.Run(ctx, "config", "--global", key, value)
}

// ConfigUnsetGlobalAll removes every value of key from the global git config.
func (c *Client) ConfigUnsetGlobalAll(ctx context.Context, key string) error {
	err := c.Run(ctx, "config", "--global", "--unset-all", key)
	var gitErr *Error
	// Exit code 5 means the key was not set, which is the desired end state.
	if errors.As(err, &gitErr) && gitErr.ExitCode == 5 {
		return nil
	}
	return err
}

// NewClient returns a Client writing diagnostics to stderr of the current process.
func NewClient() *Client {
	return &Client{Stderr: os.Stderr}
}
