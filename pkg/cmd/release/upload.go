package release

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"

	"gitverse.ru/veryviolet/gv/internal/api"
	"gitverse.ru/veryviolet/gv/internal/cmdutil"
)

// NewCmdUpload returns `gv release upload`.
func NewCmdUpload(f *cmdutil.Factory) *cobra.Command {
	var clobber bool

	cmd := &cobra.Command{
		Use:   "upload <tag> <file>...",
		Short: "Attach files to a release",
		Long: `Upload build artefacts to an existing release.

With --clobber an asset of the same name is deleted first; otherwise the API
decides what to do with a duplicate name.`,
		Example: `  $ gv release upload v0.1.0 dist/gv_0.1.0_amd64.deb
  $ gv release upload v0.1.0 dist/* --clobber`,
		Args: cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			tag, files := args[0], args[1:]

			rel, repo, err := resolveRelease(cmd.Context(), f, tag)
			if err != nil {
				return err
			}
			client, err := f.HTTPClient()
			if err != nil {
				return err
			}

			ios := f.IOStreams
			cs := ios.ColorScheme()

			existing := map[string]int64{}
			for _, a := range rel.Assets {
				existing[a.Name] = a.ID
			}

			for _, path := range files {
				name := filepath.Base(path)
				if id, ok := existing[name]; ok {
					if !clobber {
						return fmt.Errorf("release %s already has an asset named %q; pass --clobber to replace it", rel.TagName, name)
					}
					if err := api.DeleteReleaseAsset(cmd.Context(), client, repo, rel.ID, id); err != nil {
						return fmt.Errorf("replacing %s: %w", name, err)
					}
				}
				asset, err := api.UploadReleaseAsset(cmd.Context(), client, repo, rel.ID, path, name)
				if err != nil {
					return fmt.Errorf("uploading %s: %w", path, err)
				}
				fmt.Fprintf(ios.ErrOut, "%s Uploaded %s (%s)\n", cs.SuccessIcon(), asset.Name, humanSize(asset.Size))
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&clobber, "clobber", false, "Replace assets of the same name")

	return cmd
}
