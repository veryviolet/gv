package pkg

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"gitverse.ru/veryviolet/gv/internal/api"
	"gitverse.ru/veryviolet/gv/internal/cmdutil"
	"gitverse.ru/veryviolet/gv/internal/tableprinter"
	"gitverse.ru/veryviolet/gv/internal/text"
)

// NewCmdVersions returns `gv package versions`.
func NewCmdVersions(f *cmdutil.Factory) *cobra.Command {
	var (
		pkgType string
		isOrg   bool
		limit   int
		jsonOut cmdutil.JSONFlags
	)

	cmd := &cobra.Command{
		Use:     "versions [<owner>/]<package>",
		Short:   "List the versions of a package",
		Aliases: []string{"version"},
		Long: `List the published versions of one package.

For container images the version name is the tag. Pass --org when the package
belongs to an organization: users and organizations live under different API
paths and gv cannot tell them apart from the name alone.`,
		Example: `  $ gv package versions gv
  $ gv package versions veryviolet/gv
  $ gv package versions myorg/service --org`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := jsonOut.Validate(); err != nil {
				return err
			}
			owner, name, err := splitPackageName(cmd.Context(), f, args[0])
			if err != nil {
				return err
			}
			client, err := f.HTTPClient()
			if err != nil {
				return err
			}

			versions, err := api.ListPackageVersions(cmd.Context(), client,
				api.PackageOwner{Name: owner, IsOrg: isOrg}, pkgType, name, limit)
			if err != nil {
				return err
			}

			if jsonOut.Enabled() {
				return cmdutil.PrintJSON(f.IOStreams, versions, jsonOut.JQ, jsonOut.Template)
			}

			ios := f.IOStreams
			cs := ios.ColorScheme()

			if len(versions) == 0 {
				if ios.IsStdoutTTY() {
					fmt.Fprintf(ios.ErrOut, "No versions found for %s/%s.\n", owner, name)
				}
				return nil
			}

			now := time.Now()
			tp := tableprinter.New(ios.Out, ios.IsStdoutTTY(), ios.TerminalWidth())
			tp.HeaderRow("version", "id", "type", "created")
			for i := range versions {
				v := &versions[i]
				tp.AddField(v.Name, tableprinter.WithColor(cs.Bold), tableprinter.WithTruncate(true))
				tp.AddField(fmt.Sprintf("%d", v.ID), tableprinter.WithColor(cs.Gray), tableprinter.WithTruncate(false))
				tp.AddField(v.PackageType(pkgType), tableprinter.WithColor(cs.Gray), tableprinter.WithTruncate(false))
				tp.AddField(text.TimeAgo(now, v.CreatedAt), tableprinter.WithColor(cs.Gray), tableprinter.WithTruncate(false))
				tp.EndRow()
			}
			if err := tp.Render(); err != nil {
				return err
			}

			if pkgType == DefaultPackageType && ios.IsStdoutTTY() {
				host := api.RegistryHost(f.Host())
				fmt.Fprintf(ios.Out, "\n%s\n",
					cs.Gray(fmt.Sprintf("docker pull %s/%s/%s:%s",
						host, owner, name, firstVersionName(versions))))
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&pkgType, "type", DefaultPackageType, "Package `type`: {container|generic|debian|rpm|maven|npm|pypi|helm|nuget|go}")
	cmd.Flags().BoolVar(&isOrg, "org", false, "The owner is an organization")
	cmd.Flags().IntVarP(&limit, "limit", "L", 30, "Maximum number of versions to list")
	cmd.Flags().BoolVar(&jsonOut.JSON, "json", false, "Output the versions as JSON")
	cmd.Flags().StringVarP(&jsonOut.JQ, "jq", "q", "", "Filter the JSON output with a jq `expression`")
	cmd.Flags().StringVarP(&jsonOut.Template, "template", "t", "", "Format the JSON output with a Go `template`")

	return cmd
}

func firstVersionName(versions []api.PackageVersion) string {
	for i := range versions {
		if name := strings.TrimSpace(versions[i].Name); name != "" {
			return name
		}
	}
	return "latest"
}
