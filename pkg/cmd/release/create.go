package release

import (
	"fmt"

	"github.com/spf13/cobra"

	"gitverse.ru/veryviolet/gv/internal/api"
	"gitverse.ru/veryviolet/gv/internal/cmdutil"
)

// NewCmdCreate returns `gv release create`.
func NewCmdCreate(f *cmdutil.Factory) *cobra.Command {
	var (
		title      string
		notes      string
		notesFile  string
		target     string
		draft      bool
		prerelease bool
		assets     []string
	)

	cmd := &cobra.Command{
		Use:   "create <tag> [<file>...]",
		Short: "Publish a release",
		Long: `Create a release for a tag and, optionally, attach files to it.

Files given as positional arguments or with --asset are uploaded once the
release exists.`,
		Example: `  $ gv release create v0.1.0 --title "gv 0.1.0" --notes-file CHANGELOG.md
  $ gv release create v0.1.0 dist/*.deb dist/*.rpm
  $ gv release create v0.2.0-rc1 --prerelease --target main`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if notes != "" && notesFile != "" {
				return cmdutil.FlagErrorf("specify only one of --notes or --notes-file")
			}
			tag := args[0]
			files := append(append([]string{}, args[1:]...), assets...)

			repo, err := f.BaseRepo()
			if err != nil {
				return err
			}
			client, err := f.HTTPClient()
			if err != nil {
				return err
			}
			body, err := cmdutil.ReadBody(f.IOStreams, notes, notesFile)
			if err != nil {
				return err
			}

			in := api.CreateReleaseOptionRequest{TagName: api.Ptr(tag)}
			if title != "" {
				in.Name = api.Ptr(title)
			}
			if body != "" {
				in.Body = api.Ptr(body)
			}
			if target != "" {
				in.TargetCommitish = api.Ptr(target)
			}
			if draft {
				in.Draft = api.Ptr(true)
			}
			if prerelease {
				in.Prerelease = api.Ptr(true)
			}

			rel, err := api.CreateRelease(cmd.Context(), client, repo, in)
			if err != nil {
				return err
			}

			ios := f.IOStreams
			cs := ios.ColorScheme()
			fmt.Fprintf(ios.ErrOut, "%s Created release %s\n", cs.SuccessIcon(), cs.Bold(rel.TagName))

			for _, path := range files {
				asset, err := api.UploadReleaseAsset(cmd.Context(), client, repo, rel.ID, path, "")
				if err != nil {
					return fmt.Errorf("uploading %s: %w", path, err)
				}
				fmt.Fprintf(ios.ErrOut, "%s Uploaded %s\n", cs.SuccessIcon(), asset.Name)
			}

			fmt.Fprintln(ios.Out, rel.HTMLURL)
			return nil
		},
	}

	cmd.Flags().StringVarP(&title, "title", "t", "", "Title of the release")
	cmd.Flags().StringVarP(&notes, "notes", "n", "", "Release notes")
	cmd.Flags().StringVarP(&notesFile, "notes-file", "F", "", "Read the release notes from `file` (use \"-\" for stdin)")
	cmd.Flags().StringVar(&target, "target", "", "Branch or commit the tag should point at")
	cmd.Flags().BoolVarP(&draft, "draft", "d", false, "Save the release as a draft")
	cmd.Flags().BoolVarP(&prerelease, "prerelease", "p", false, "Mark the release as a prerelease")
	cmd.Flags().StringSliceVarP(&assets, "asset", "a", nil, "Additional `files` to attach")

	return cmd
}
