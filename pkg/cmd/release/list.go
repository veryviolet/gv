package release

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"gitverse.ru/veryviolet/gv/internal/api"
	"gitverse.ru/veryviolet/gv/internal/cmdutil"
	"gitverse.ru/veryviolet/gv/internal/tableprinter"
	"gitverse.ru/veryviolet/gv/internal/text"
)

// NewCmdList returns `gv release list`.
func NewCmdList(f *cmdutil.Factory) *cobra.Command {
	var (
		limit   int
		jsonOut cmdutil.JSONFlags
	)

	cmd := &cobra.Command{
		Use:     "list",
		Short:   "List releases",
		Aliases: []string{"ls"},
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := jsonOut.Validate(); err != nil {
				return err
			}
			repo, err := f.BaseRepo()
			if err != nil {
				return err
			}
			client, err := f.HTTPClient()
			if err != nil {
				return err
			}
			releases, err := api.ListReleases(cmd.Context(), client, repo, limit)
			if err != nil {
				return err
			}
			if jsonOut.Enabled() {
				return cmdutil.PrintJSON(f.IOStreams, releases, jsonOut.JQ, jsonOut.Template)
			}

			ios := f.IOStreams
			cs := ios.ColorScheme()
			if len(releases) == 0 {
				if ios.IsStdoutTTY() {
					fmt.Fprintln(ios.ErrOut, "No releases found.")
				}
				return nil
			}

			now := time.Now()
			tp := tableprinter.New(ios.Out, ios.IsStdoutTTY(), ios.TerminalWidth())
			tp.HeaderRow("tag", "title", "kind", "assets", "published")
			for i := range releases {
				r := &releases[i]
				tp.AddField(r.TagName, tableprinter.WithColor(cs.Bold), tableprinter.WithTruncate(false))
				tp.AddField(r.Name, tableprinter.WithTruncate(true))
				tp.AddField(releaseKind(r), tableprinter.WithColor(kindColor(cs, r)), tableprinter.WithTruncate(false))
				tp.AddField(fmt.Sprintf("%d", len(r.Assets)), tableprinter.WithColor(cs.Gray), tableprinter.WithTruncate(false))
				tp.AddField(text.TimeAgo(now, publishedAt(r)), tableprinter.WithColor(cs.Gray), tableprinter.WithTruncate(false))
				tp.EndRow()
			}
			return tp.Render()
		},
	}

	cmd.Flags().IntVarP(&limit, "limit", "L", 30, "Maximum number of releases to list")
	cmd.Flags().BoolVar(&jsonOut.JSON, "json", false, "Output the releases as JSON")
	cmd.Flags().StringVarP(&jsonOut.JQ, "jq", "q", "", "Filter the JSON output with a jq `expression`")
	cmd.Flags().StringVarP(&jsonOut.Template, "template", "t", "", "Format the JSON output with a Go `template`")

	return cmd
}

func releaseKind(r *api.Release) string {
	switch {
	case r.Draft:
		return "draft"
	case r.Prerelease:
		return "prerelease"
	default:
		return "release"
	}
}

func kindColor(cs interface {
	Gray(string) string
	Yellow(string) string
	Green(string) string
}, r *api.Release) func(string) string {
	switch {
	case r.Draft:
		return cs.Gray
	case r.Prerelease:
		return cs.Yellow
	default:
		return cs.Green
	}
}

func publishedAt(r *api.Release) string {
	if r.PublishedAt != "" {
		return r.PublishedAt
	}
	return r.CreatedAt
}
