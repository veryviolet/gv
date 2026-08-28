// Package release implements `gv release`.
package release

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"gitverse.ru/veryviolet/gv/internal/api"
	"gitverse.ru/veryviolet/gv/internal/cmdutil"
	"gitverse.ru/veryviolet/gv/internal/gvrepo"
)

// NewCmdRelease returns the `gv release` command group.
func NewCmdRelease(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "release <command>",
		Short:   "Work with releases",
		Aliases: []string{"releases"},
		Long: `Publish releases and attach build artefacts to them.

A release is identified by its tag; the tag must already exist in the
repository, or be created from --target when the release is published.`,
	}

	cmd.PersistentFlags().StringP("repo", "R", "", "Select a repository using the `OWNER/REPO` format")
	cmd.PersistentPreRunE = func(c *cobra.Command, args []string) error {
		if nwo, _ := c.Flags().GetString("repo"); nwo != "" {
			return f.OverrideBaseRepo(nwo)
		}
		return nil
	}

	cmd.AddCommand(NewCmdList(f))
	cmd.AddCommand(NewCmdView(f))
	cmd.AddCommand(NewCmdCreate(f))
	cmd.AddCommand(NewCmdUpload(f))
	cmd.AddCommand(NewCmdDelete(f))

	return cmd
}

// resolveRelease finds a release by tag, or the most recent one when tag is empty.
func resolveRelease(ctx context.Context, f *cmdutil.Factory, tag string) (*api.Release, gvrepo.Interface, error) {
	repo, err := f.BaseRepo()
	if err != nil {
		return nil, nil, err
	}
	client, err := f.HTTPClient()
	if err != nil {
		return nil, nil, err
	}

	if tag != "" {
		rel, err := api.GetReleaseByTag(ctx, client, repo, tag)
		if err != nil {
			return nil, nil, err
		}
		return rel, repo, nil
	}

	releases, err := api.ListReleases(ctx, client, repo, 1)
	if err != nil {
		return nil, nil, err
	}
	if len(releases) == 0 {
		return nil, nil, fmt.Errorf("no releases found in %s", gvrepo.FullName(repo))
	}
	return &releases[0], repo, nil
}
