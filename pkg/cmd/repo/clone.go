package repo

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"gitverse.ru/veryviolet/gv/internal/api"
	"gitverse.ru/veryviolet/gv/internal/cmdutil"
	"gitverse.ru/veryviolet/gv/internal/git"
	"gitverse.ru/veryviolet/gv/internal/gvrepo"
)

// NewCmdClone returns `gv repo clone`.
func NewCmdClone(f *cmdutil.Factory) *cobra.Command {
	var protocol string
	var upstreamName string

	cmd := &cobra.Command{
		Use:   "clone <repository> [<directory>] [-- <git-flags>...]",
		Short: "Clone a repository locally",
		Long: `Clone a GitVerse repository.

Arguments after "--" are passed straight to git clone. When the repository is
a fork, a remote named "upstream" is added for its parent.`,
		Example: `  $ gv repo clone gitverse/rest-api-description
  $ gv repo clone veryviolet/gv gv-cli
  $ gv repo clone veryviolet/gv -- --depth 1`,
		Args:                  cobra.MinimumNArgs(1),
		DisableFlagsInUseLine: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if protocol != "" && protocol != "https" && protocol != "ssh" {
				return cmdutil.FlagErrorf("--protocol must be either https or ssh")
			}
			return cloneRun(cmd.Context(), f, args, protocol, upstreamName)
		},
	}

	cmd.Flags().StringVarP(&protocol, "protocol", "p", "", "Clone protocol to use: {https|ssh}")
	cmd.Flags().StringVar(&upstreamName, "upstream-remote-name", "upstream", "Remote `name` to create for the parent of a fork")

	return cmd
}

func cloneRun(ctx context.Context, f *cmdutil.Factory, args []string, protocol, upstreamName string) error {
	ios := f.IOStreams
	cs := ios.ColorScheme()

	repo, err := gvrepo.FromFullNameWithHost(args[0], f.Host())
	if err != nil {
		return cmdutil.FlagErrorf("%v", err)
	}

	if protocol == "" {
		cfg, err := f.Config()
		if err != nil {
			return err
		}
		protocol = cfg.GitProtocol(repo.RepoHost())
	}

	client, err := f.HTTPClient()
	if err != nil {
		return err
	}

	// Fetching the repository first both validates the name and yields the
	// canonical clone URL and the parent of a fork.
	r, err := api.GetRepository(ctx, client, repo)
	if err != nil {
		return err
	}

	cloneURL := cloneURLFor(r, repo, protocol)
	gitClient := &git.Client{Stderr: ios.ErrOut, Stdout: ios.ErrOut}
	dir, err := gitClient.Clone(ctx, cloneURL, args[1:])
	if err != nil {
		return err
	}

	if r.Fork && r.Parent != nil && r.Parent.FullName != "" {
		parent := gvrepo.NewWithHost(r.Parent.Owner.Login, r.Parent.Name, repo.RepoHost())
		if r.Parent.Owner.Login == "" {
			owner, name := splitOwnerName(r.Parent.FullName)
			parent = gvrepo.NewWithHost(owner, name, repo.RepoHost())
		}
		upstreamURL := cloneURLFor(r.Parent, parent, protocol)

		inRepo := &git.Client{RepoDir: dir, Stderr: ios.ErrOut}
		if err := inRepo.AddRemote(ctx, upstreamName, upstreamURL); err != nil {
			return err
		}
		fmt.Fprintf(ios.ErrOut, "%s Added remote %s for %s\n", cs.SuccessIcon(), upstreamName, gvrepo.FullName(parent))
	}

	fmt.Fprintf(ios.ErrOut, "%s Cloned %s into %s\n", cs.SuccessIcon(), cs.Bold(r.FullName), dir)
	return nil
}
