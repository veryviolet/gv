package pkg

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"gitverse.ru/veryviolet/gv/internal/cmdutil"
	"gitverse.ru/veryviolet/gv/internal/tableprinter"
)

// NewCmdList returns `gv package list`.
func NewCmdList(f *cmdutil.Factory) *cobra.Command {
	var (
		maxItems int
		withTags bool
		jsonOut  cmdutil.JSONFlags
	)

	cmd := &cobra.Command{
		Use:     "list [<owner>]",
		Short:   "List container images of an owner",
		Aliases: []string{"ls"},
		Long: `List the registry repositories belonging to an owner.

The REST API cannot enumerate packages, so this reads the registry catalogue,
which is shared by the whole platform and sorted by name: gv starts at the
owner's prefix rather than walking the catalogue from the beginning.`,
		Example: `  $ gv package list
  $ gv package list veryviolet --tags`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := jsonOut.Validate(); err != nil {
				return err
			}
			arg := ""
			if len(args) > 0 {
				arg = args[0]
			}
			owner, err := resolveOwner(cmd.Context(), f, arg)
			if err != nil {
				return err
			}
			registry, host, err := registryClient(f)
			if err != nil {
				return err
			}

			repos, err := registry.RepositoriesForOwner(cmd.Context(), owner, maxItems)
			if err != nil {
				return err
			}

			ios := f.IOStreams
			cs := ios.ColorScheme()

			type row struct {
				Name  string   `json:"name"`
				Image string   `json:"image"`
				Tags  []string `json:"tags,omitempty"`
			}
			rows := make([]row, 0, len(repos))
			for _, repo := range repos {
				r := row{Name: repo, Image: host + "/" + repo}
				if withTags {
					tags, err := registry.Tags(cmd.Context(), repo)
					if err != nil {
						// A repository without readable tags is still worth
						// listing; the name is the useful part.
						fmt.Fprintf(ios.ErrOut, "%s tags of %s: %v\n", cs.WarningIcon(), repo, err)
					}
					r.Tags = tags
				}
				rows = append(rows, r)
			}

			if jsonOut.Enabled() {
				return cmdutil.PrintJSON(ios, rows, jsonOut.JQ, jsonOut.Template)
			}

			if len(rows) == 0 {
				if ios.IsStdoutTTY() {
					fmt.Fprintf(ios.ErrOut, "No packages found for %s.\n", owner)
				}
				return nil
			}

			tp := tableprinter.New(ios.Out, ios.IsStdoutTTY(), ios.TerminalWidth())
			if withTags {
				tp.HeaderRow("image", "tags")
			} else {
				tp.HeaderRow("image")
			}
			for _, r := range rows {
				tp.AddField(r.Image, tableprinter.WithColor(cs.Bold), tableprinter.WithTruncate(true))
				if withTags {
					tp.AddField(strings.Join(r.Tags, ", "), tableprinter.WithColor(cs.Cyan), tableprinter.WithTruncate(true))
				}
				tp.EndRow()
			}
			return tp.Render()
		},
	}

	cmd.Flags().IntVarP(&maxItems, "limit", "L", 100, "Maximum number of packages to list")
	cmd.Flags().BoolVar(&withTags, "tags", false, "Also fetch the tags of each image")
	cmd.Flags().BoolVar(&jsonOut.JSON, "json", false, "Output the packages as JSON")
	cmd.Flags().StringVarP(&jsonOut.JQ, "jq", "q", "", "Filter the JSON output with a jq `expression`")
	cmd.Flags().StringVarP(&jsonOut.Template, "template", "t", "", "Format the JSON output with a Go `template`")

	return cmd
}
