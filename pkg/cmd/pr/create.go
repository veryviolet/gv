package pr

import (
	"context"
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"gitverse.ru/veryviolet/gv/internal/api"
	"gitverse.ru/veryviolet/gv/internal/cmdutil"
	"gitverse.ru/veryviolet/gv/internal/git"
	"gitverse.ru/veryviolet/gv/internal/gvrepo"
)

type createOptions struct {
	title     string
	body      string
	bodyFile  string
	base      string
	head      string
	draft     bool
	labelIDs  []int64
	assignees []string
	push      bool
	web       bool
}

// NewCmdCreate returns `gv pr create`.
func NewCmdCreate(f *cmdutil.Factory) *cobra.Command {
	var opts createOptions

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Open a pull request",
		Long: `Create a pull request.

The head branch defaults to the current git branch and the base branch to the
repository's default branch. A title is required; gv does not open an editor.`,
		Example: `  $ gv pr create --title "Fix pagination" --body "Closes #12"
  $ gv pr create -t "WIP" --draft --base develop
  $ gv pr create -t "Docs" -F description.md --push`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if opts.body != "" && opts.bodyFile != "" {
				return cmdutil.FlagErrorf("specify only one of --body or --body-file")
			}
			return createRun(cmd.Context(), f, &opts)
		},
	}

	cmd.Flags().StringVarP(&opts.title, "title", "t", "", "Title of the pull request")
	cmd.Flags().StringVarP(&opts.body, "body", "b", "", "Body of the pull request")
	cmd.Flags().StringVarP(&opts.bodyFile, "body-file", "F", "", "Read the body from `file` (use \"-\" for stdin)")
	cmd.Flags().StringVarP(&opts.base, "base", "B", "", "The `branch` to merge into")
	cmd.Flags().StringVarP(&opts.head, "head", "H", "", "The `branch` that contains the commits")
	cmd.Flags().BoolVarP(&opts.draft, "draft", "d", false, "Open the pull request as a draft")
	// The pull request endpoint takes label IDs, not names, and the API offers
	// no way to look a label up by name.
	cmd.Flags().Int64SliceVarP(&opts.labelIDs, "label-id", "l", nil, "Add labels by numeric `id`")
	cmd.Flags().StringSliceVarP(&opts.assignees, "assignee", "a", nil, "Assign `people` to the pull request")
	cmd.Flags().BoolVar(&opts.push, "push", false, "Push the head branch to the remote first")
	cmd.Flags().BoolVarP(&opts.web, "web", "w", false, "Open the compare page in a browser instead")

	return cmd
}

func createRun(ctx context.Context, f *cmdutil.Factory, opts *createOptions) error {
	ios := f.IOStreams
	cs := ios.ColorScheme()

	repo, err := f.BaseRepo()
	if err != nil {
		return err
	}
	client, err := f.HTTPClient()
	if err != nil {
		return err
	}

	head := opts.head
	if head == "" {
		head, err = f.GitClient.CurrentBranch(ctx)
		if err != nil {
			if errors.Is(err, git.ErrNotOnBranch) {
				return errors.New("HEAD is not on a branch; pass --head")
			}
			return fmt.Errorf("could not determine the current branch: %w", err)
		}
	}

	base := opts.base
	if base == "" {
		r, err := api.GetRepository(ctx, client, repo)
		if err != nil {
			return err
		}
		base = r.DefaultBranch
	}
	if base == head {
		return cmdutil.FlagErrorf("the head and base branches are both %q", base)
	}

	if opts.push {
		remotes, err := f.Remotes()
		if err != nil {
			return err
		}
		remote, ok := remotes.FindByName("origin", "*")
		if !ok {
			return errors.New("--push needs a git remote to push to")
		}
		if err := f.GitClient.Push(ctx, remote.Name, head); err != nil {
			return err
		}
	}

	if opts.web {
		url := gvrepo.WebURL(repo, fmt.Sprintf("compare/%s...%s", base, head))
		fmt.Fprintf(ios.ErrOut, "Opening %s in your browser.\n", url)
		return f.Browser.Browse(url)
	}

	if opts.title == "" {
		return cmdutil.FlagErrorf("--title is required")
	}

	body, err := cmdutil.ReadBody(ios, opts.body, opts.bodyFile)
	if err != nil {
		return err
	}

	in := api.CreatePullRequestAPIRequest{
		Title: api.Ptr(opts.title),
		Head:  api.Ptr(head),
		Base:  api.Ptr(base),
	}
	if body != "" {
		in.Body = api.Ptr(body)
	}
	if opts.draft {
		in.Draft = api.Ptr(true)
	}
	if len(opts.labelIDs) > 0 {
		in.Labels = opts.labelIDs
	}
	if len(opts.assignees) > 0 {
		in.Assignees = opts.assignees
	}

	pr, err := api.CreatePullRequest(ctx, client, repo, in)
	if err != nil {
		return err
	}

	fmt.Fprintf(ios.ErrOut, "%s Created pull request %s\n",
		cs.SuccessIcon(), cs.Bold(fmt.Sprintf("#%d", pr.Number)))
	fmt.Fprintln(ios.Out, pr.HTMLURL)
	return nil
}
