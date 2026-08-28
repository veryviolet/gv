package repo

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"gitverse.ru/veryviolet/gv/internal/api"
	"gitverse.ru/veryviolet/gv/internal/cmdutil"
	"gitverse.ru/veryviolet/gv/internal/gvrepo"
)

type createOptions struct {
	name        string
	description string
	public      bool
	private     bool
	gitignores  []string
	addReadme   bool
	isTemplate  bool
	clone       bool
}

// NewCmdCreate returns `gv repo create`.
func NewCmdCreate(f *cmdutil.Factory) *cobra.Command {
	var opts createOptions

	cmd := &cobra.Command{
		Use:   "create <name>",
		Short: "Create a new repository",
		Long: `Create a repository on GitVerse.

Pass "OWNER/NAME" to create it inside an organization you belong to; a bare
name creates it under your own account. New repositories are private unless
--public is given.`,
		Example: `  $ gv repo create my-project --private --add-readme
  $ gv repo create myorg/service --description "Billing service" --public
  $ gv repo create tools --clone`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.name = args[0]
			if err := cmdutil.MutuallyExclusive("specify only one of --public or --private", opts.public, opts.private); err != nil {
				return err
			}
			return createRun(cmd.Context(), f, &opts)
		},
	}

	cmd.Flags().StringVarP(&opts.description, "description", "d", "", "Description of the repository")
	cmd.Flags().BoolVar(&opts.public, "public", false, "Make the repository public")
	cmd.Flags().BoolVar(&opts.private, "private", false, "Make the repository private (default)")
	cmd.Flags().StringSliceVar(&opts.gitignores, "gitignore", nil, "Gitignore `templates` to apply")
	cmd.Flags().BoolVar(&opts.addReadme, "add-readme", false, "Initialise the repository with a README")
	cmd.Flags().BoolVar(&opts.isTemplate, "template", false, "Mark the repository as a template")
	cmd.Flags().BoolVarP(&opts.clone, "clone", "c", false, "Clone the new repository afterwards")

	return cmd
}

func createRun(ctx context.Context, f *cmdutil.Factory, opts *createOptions) error {
	client, err := f.HTTPClient()
	if err != nil {
		return err
	}
	ios := f.IOStreams
	cs := ios.ColorScheme()

	owner, name := splitOwnerName(opts.name)
	if name == "" {
		return cmdutil.FlagErrorf("a repository name is required")
	}

	private := !opts.public

	var created *api.Repository
	if owner == "" {
		created, err = api.CreateRepository(ctx, client, api.CreateRepoRequest{
			Name:        api.Ptr(name),
			Description: optionalString(opts.description),
			Private:     api.Ptr(private),
			AutoInit:    api.Ptr(opts.addReadme),
			IsTemplate:  api.Ptr(opts.isTemplate),
			Gitignores:  opts.gitignores,
		})
	} else {
		// An owner that matches the authenticated user is still their own
		// namespace, not an organization.
		me, userErr := api.CurrentUser(ctx, client)
		if userErr == nil && strings.EqualFold(me.Login, owner) {
			created, err = api.CreateRepository(ctx, client, api.CreateRepoRequest{
				Name:        api.Ptr(name),
				Description: optionalString(opts.description),
				Private:     api.Ptr(private),
				AutoInit:    api.Ptr(opts.addReadme),
				IsTemplate:  api.Ptr(opts.isTemplate),
				Gitignores:  opts.gitignores,
			})
		} else {
			created, err = api.CreateOrgRepository(ctx, client, owner, api.CreateOrgRepoOptionRequest{
				Name:        api.Ptr(name),
				Description: optionalString(opts.description),
				Private:     api.Ptr(private),
				AutoInit:    api.Ptr(opts.addReadme),
				IsTemplate:  api.Ptr(opts.isTemplate),
				Gitignores:  opts.gitignores,
			})
		}
	}
	if err != nil {
		return err
	}

	fmt.Fprintf(ios.ErrOut, "%s Created repository %s\n", cs.SuccessIcon(), cs.Bold(created.FullName))
	fmt.Fprintln(ios.Out, created.HTMLURL)

	if !opts.clone {
		return nil
	}

	cfg, err := f.Config()
	if err != nil {
		return err
	}
	repo := gvrepo.NewWithHost(created.Owner.Login, created.Name, f.Host())
	cloneURL := cloneURLFor(created, repo, cfg.GitProtocol(f.Host()))
	dir, err := f.GitClient.Clone(ctx, cloneURL, nil)
	if err != nil {
		return err
	}
	fmt.Fprintf(ios.ErrOut, "%s Cloned into %s\n", cs.SuccessIcon(), dir)
	return nil
}

func splitOwnerName(s string) (owner, name string) {
	s = strings.Trim(strings.TrimSpace(s), "/")
	if before, after, ok := strings.Cut(s, "/"); ok {
		return before, after
	}
	return "", s
}

func optionalString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// cloneURLFor prefers the URL the API reported, falling back to one derived
// from the repository coordinates.
func cloneURLFor(r *api.Repository, repo gvrepo.Interface, protocol string) string {
	if protocol == "ssh" {
		if r.SSHURL != "" {
			return r.SSHURL
		}
		return gvrepo.CloneURL(repo, "ssh")
	}
	if r.CloneURL != "" {
		return r.CloneURL
	}
	return gvrepo.CloneURL(repo, "https")
}
