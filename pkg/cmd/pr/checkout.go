package pr

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"gitverse.ru/veryviolet/gv/internal/api"
	"gitverse.ru/veryviolet/gv/internal/cmdutil"
	"gitverse.ru/veryviolet/gv/internal/git"
	"gitverse.ru/veryviolet/gv/internal/gvrepo"
)

// NewCmdCheckout returns `gv pr checkout`.
func NewCmdCheckout(f *cmdutil.Factory) *cobra.Command {
	var branchName string

	cmd := &cobra.Command{
		Use:   "checkout <number>",
		Short: "Check out a pull request locally",
		Long: `Fetch the head of a pull request and switch to it.

gv first tries the server-side ref "pull/<number>/head". Not every GitVerse
deployment publishes those refs, so it falls back to fetching the head branch
directly from the fork it lives in.`,
		Example: `  $ gv pr checkout 42
  $ gv pr checkout 42 --branch review-42`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			pr, repo, err := resolvePR(cmd.Context(), f, args[0])
			if err != nil {
				return err
			}
			return checkoutRun(cmd.Context(), f, pr, repo, branchName)
		},
	}

	cmd.Flags().StringVarP(&branchName, "branch", "b", "", "Local branch `name` to create")

	return cmd
}

func checkoutRun(ctx context.Context, f *cmdutil.Factory, pr *api.PullRequest, repo gvrepo.Interface, branchName string) error {
	ios := f.IOStreams
	cs := ios.ColorScheme()

	if branchName == "" {
		branchName = localBranchName(pr)
	}

	remotes, err := f.Remotes()
	if err != nil {
		return fmt.Errorf("pr checkout must run inside a git repository: %w", err)
	}
	remote, ok := remotes.FindByName("upstream", "origin", "*")
	if !ok {
		return fmt.Errorf("no git remote found to fetch from")
	}

	gitClient := &git.Client{Stderr: ios.ErrOut}

	// Preferred path: the pull request ref published by the server.
	refspec := fmt.Sprintf("refs/pull/%d/head:%s", pr.Number, branchName)
	err = gitClient.Fetch(ctx, remote.Name, "--force", refspec)
	if err != nil {
		// Fallback: fetch the head branch from wherever it lives. For a fork
		// that is a different clone URL than the base repository.
		headURL := pr.Head.Repo.CloneURL
		if headURL == "" {
			headURL = gvrepo.CloneURL(repo, "https")
		}
		if pr.Head.Ref == "" {
			return fmt.Errorf("could not fetch pull request #%d: %w", pr.Number, err)
		}
		branchRefspec := fmt.Sprintf("%s:%s", pr.Head.Ref, branchName)
		if fallbackErr := gitClient.Fetch(ctx, headURL, "--force", branchRefspec); fallbackErr != nil {
			return fmt.Errorf("could not fetch pull request #%d: %w", pr.Number, fallbackErr)
		}
	}

	if err := gitClient.CheckoutBranch(ctx, branchName); err != nil {
		return err
	}

	fmt.Fprintf(ios.ErrOut, "%s Checked out pull request #%d as %s\n",
		cs.SuccessIcon(), pr.Number, cs.Bold(branchName))
	return nil
}

// localBranchName keeps the head branch name when it is unambiguous, and
// prefixes the contributor otherwise, mirroring gh's behaviour for forks.
func localBranchName(pr *api.PullRequest) string {
	ref := pr.Head.Ref
	if ref == "" {
		return fmt.Sprintf("pr-%d", pr.Number)
	}
	owner := pr.Head.Repo.Owner.Login
	base := pr.Base.Repo.Owner.Login
	if owner != "" && base != "" && !strings.EqualFold(owner, base) {
		return fmt.Sprintf("%s-%s", owner, ref)
	}
	return ref
}
