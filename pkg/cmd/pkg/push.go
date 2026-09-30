package pkg

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"

	"gitverse.ru/veryviolet/gv/internal/api"
	"gitverse.ru/veryviolet/gv/internal/build"
	"gitverse.ru/veryviolet/gv/internal/cmdutil"
)

// NewCmdPush returns `gv package push`.
func NewCmdPush(f *cmdutil.Factory) *cobra.Command {
	var (
		owner        string
		kind         string
		distribution string
		component    string
		group        string
		name         string
		version      string
	)

	cmd := &cobra.Command{
		Use:   "push <file>...",
		Short: "Publish packages to the GitVerse registries",
		Long: `Upload .deb, .rpm or arbitrary files to the package registries.

The registry signs the repository metadata itself, so clients verify the
packages against a published key — unlike a plain file share. The kind is
guessed from the extension and can be forced with --kind.

Files are immutable: republishing the same name, version and architecture
fails until the existing version is deleted.`,
		Example: `  $ gv package push dist/gv_0.2.0-1_amd64.deb
  $ gv package push dist/*.rpm --group gv
  $ gv package push notes.txt --kind generic --name gv --version 0.2.0`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ios := f.IOStreams
			cs := ios.ColorScheme()

			ownerName, err := resolveOwner(cmd.Context(), f, owner)
			if err != nil {
				return err
			}
			cfg, err := f.Config()
			if err != nil {
				return err
			}
			host := f.Host()
			token, _ := cfg.Token(host)
			if token == "" {
				return cmdutil.NewAuthError(fmt.Errorf("no token found for %s", host))
			}
			client := api.NewPackageRegistryClient(host, cfg.User(host), token, "gv/"+build.Version)

			pushedDeb, pushedRPM := false, false
			for _, file := range args {
				k := kind
				if k == "" {
					k = api.PackageKindForFile(file)
				}

				switch k {
				case "deb", "debian":
					err = client.UploadDebian(cmd.Context(), ownerName, distribution, component, file)
					pushedDeb = true
				case "rpm":
					err = client.UploadRPM(cmd.Context(), ownerName, group, file)
					pushedRPM = true
				case "generic":
					if name == "" || version == "" {
						return cmdutil.FlagErrorf("generic packages need --name and --version")
					}
					err = client.UploadGeneric(cmd.Context(), ownerName, name, version, file)
				default:
					return cmdutil.FlagErrorf("unknown kind %q: expected deb, rpm or generic", k)
				}
				if err != nil {
					return fmt.Errorf("uploading %s: %w", filepath.Base(file), err)
				}
				fmt.Fprintf(ios.ErrOut, "%s Published %s\n", cs.SuccessIcon(), filepath.Base(file))
			}

			if ios.IsStdoutTTY() {
				printClientHints(f, ownerName, host, distribution, component, group, pushedDeb, pushedRPM)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&owner, "owner", "", "Publish under this `owner` instead of the authenticated user")
	cmd.Flags().StringVar(&kind, "kind", "", "Registry to use: {deb|rpm|generic} (guessed from the extension)")
	cmd.Flags().StringVar(&distribution, "distribution", "stable", "Debian `distribution`")
	cmd.Flags().StringVar(&component, "component", "main", "Debian `component`")
	cmd.Flags().StringVar(&group, "group", "", "RPM `group`; empty publishes to the owner-wide repository")
	cmd.Flags().StringVar(&name, "name", "", "Generic package `name`")
	cmd.Flags().StringVar(&version, "version", "", "Generic package `version`")

	return cmd
}

// printClientHints shows the lines users need to consume what was just pushed.
func printClientHints(f *cmdutil.Factory, owner, host, distribution, component, group string, deb, rpm bool) {
	ios := f.IOStreams
	cs := ios.ColorScheme()
	keyring := fmt.Sprintf("/etc/apt/keyrings/gitverse-%s.asc", owner)

	if deb {
		fmt.Fprintf(ios.Out, "\n%s\n", cs.Bold("apt"))
		fmt.Fprintf(ios.Out, "  sudo curl -fsSL %s -o %s\n", api.DebianRepositoryKey(host, owner), keyring)
		fmt.Fprintf(ios.Out, "  %s\n", api.DebianSourceLine(host, owner, distribution, component, keyring))
	}
	if rpm {
		fmt.Fprintf(ios.Out, "\n%s\n", cs.Bold("dnf"))
		fmt.Fprintf(ios.Out, "  sudo dnf config-manager --add-repo %s\n", api.RPMRepoFile(host, owner, group))
	}
}
