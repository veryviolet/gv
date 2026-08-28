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

type forkOptions struct {
	org        string
	name       string
	clone      bool
	remote     bool
	remoteName string
}

// NewCmdFork returns `gv repo fork`.
func NewCmdFork(f *cmdutil.Factory) *cobra.Command {
	var opts forkOptions

	cmd := &cobra.Command{
		Use:   "fork [<repository>]",
		Short: "Create a fork of a repository",
		Long: `Fork a repository into your account or an organization.

Run inside a clone of the upstream repository and pass --remote to add the new
fork as a git remote.`,
		Example: `  $ gv repo fork gitverse/rest-api-description --clone
  $ gv repo fork --org myorg --remote`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			arg := ""
			if len(args) > 0 {
				arg = args[0]
			}
			repo, err := cmdutil.RepoFromArg(f, arg)
			if err != nil {
				return err
			}
			return forkRun(cmd.Context(), f, repo, &opts)
		},
	}

	cmd.Flags().StringVar(&opts.org, "org", "", "Create the fork in an `organization`")
	cmd.Flags().StringVar(&opts.name, "fork-name", "", "Rename the forked repository")
	cmd.Flags().BoolVar(&opts.clone, "clone", false, "Clone the fork afterwards")
	cmd.Flags().BoolVar(&opts.remote, "remote", false, "Add the fork as a git remote in the current repository")
	cmd.Flags().StringVar(&opts.remoteName, "remote-name", "origin", "Remote `name` to use with --remote")

	return cmd
}

func forkRun(ctx context.Context, f *cmdutil.Factory, repo gvrepo.Interface, opts *forkOptions) error {
	ios := f.IOStreams
	cs := ios.ColorScheme()

	client, err := f.HTTPClient()
	if err != nil {
		return err
	}

	in := api.ForkRepoRequest{}
	if opts.org != "" {
		in.Organization = api.Ptr(opts.org)
	}
	if opts.name != "" {
		in.Name = api.Ptr(opts.name)
	}

	forked, err := api.ForkRepository(ctx, client, repo, in)
	if err != nil {
		return err
	}

	fmt.Fprintf(ios.ErrOut, "%s Created fork %s\n", cs.SuccessIcon(), cs.Bold(forked.FullName))

	cfg, err := f.Config()
	if err != nil {
		return err
	}
	protocol := cfg.GitProtocol(repo.RepoHost())

	owner, name := splitOwnerName(forked.FullName)
	if forked.Owner.Login != "" {
		owner, name = forked.Owner.Login, forked.Name
	}
	forkRepo := gvrepo.NewWithHost(owner, name, repo.RepoHost())
	forkURL := cloneURLFor(forked, forkRepo, protocol)

	if opts.remote {
		gitClient := &git.Client{Stderr: ios.ErrOut}
		remotes, err := gitClient.Remotes(ctx)
		if err != nil {
			return fmt.Errorf("--remote requires running inside a git repository: %w", err)
		}
		if _, exists := remotes.FindByName(opts.remoteName); exists {
			// Renaming the user's remotes without asking would be surprising,
			// so report the clash instead.
			return fmt.Errorf("a remote named %q already exists; pass --remote-name to choose another", opts.remoteName)
		}
		if err := gitClient.AddRemote(ctx, opts.remoteName, forkURL); err != nil {
			return err
		}
		fmt.Fprintf(ios.ErrOut, "%s Added remote %s\n", cs.SuccessIcon(), opts.remoteName)
	}

	if opts.clone {
		gitClient := &git.Client{Stderr: ios.ErrOut, Stdout: ios.ErrOut}
		dir, err := gitClient.Clone(ctx, forkURL, nil)
		if err != nil {
			return err
		}
		inRepo := &git.Client{RepoDir: dir, Stderr: ios.ErrOut}
		upstreamURL := gvrepo.CloneURL(repo, protocol)
		if err := inRepo.AddRemote(ctx, "upstream", upstreamURL); err != nil {
			return err
		}
		fmt.Fprintf(ios.ErrOut, "%s Cloned into %s with remote upstream -> %s\n",
			cs.SuccessIcon(), dir, gvrepo.FullName(repo))
	}

	fmt.Fprintln(ios.Out, forked.HTMLURL)
	return nil
}
