package release

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"gitverse.ru/veryviolet/gv/internal/cmdutil"
	"gitverse.ru/veryviolet/gv/internal/tableprinter"
	"gitverse.ru/veryviolet/gv/internal/text"
)

// NewCmdView returns `gv release view`.
func NewCmdView(f *cmdutil.Factory) *cobra.Command {
	var (
		web     bool
		jsonOut cmdutil.JSONFlags
	)

	cmd := &cobra.Command{
		Use:   "view [<tag>]",
		Short: "Show a release",
		Long:  `Show a release and the files attached to it. Without a tag, the latest release is shown.`,
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := jsonOut.Validate(); err != nil {
				return err
			}
			tag := ""
			if len(args) > 0 {
				tag = args[0]
			}
			rel, _, err := resolveRelease(cmd.Context(), f, tag)
			if err != nil {
				return err
			}
			if web {
				fmt.Fprintf(f.IOStreams.ErrOut, "Opening %s in your browser.\n", rel.HTMLURL)
				return f.Browser.Browse(rel.HTMLURL)
			}
			if jsonOut.Enabled() {
				return cmdutil.PrintJSON(f.IOStreams, rel, jsonOut.JQ, jsonOut.Template)
			}

			ios := f.IOStreams
			cs := ios.ColorScheme()
			now := time.Now()

			if !ios.IsStdoutTTY() {
				fmt.Fprintf(ios.Out, "tag:\t%s\n", rel.TagName)
				fmt.Fprintf(ios.Out, "title:\t%s\n", rel.Name)
				fmt.Fprintf(ios.Out, "kind:\t%s\n", releaseKind(rel))
				fmt.Fprintf(ios.Out, "url:\t%s\n", rel.HTMLURL)
				for _, a := range rel.Assets {
					fmt.Fprintf(ios.Out, "asset:\t%s\t%d\t%s\n", a.Name, a.Size, a.BrowserDownloadURL)
				}
				fmt.Fprintf(ios.Out, "--\n%s\n", rel.Body)
				return nil
			}

			title := rel.Name
			if title == "" {
				title = rel.TagName
			}
			fmt.Fprintf(ios.Out, "%s %s\n", cs.Bold(title), cs.Gray(rel.TagName))
			fmt.Fprintf(ios.Out, "%s • %s • %s\n",
				kindColor(cs, rel)(releaseKind(rel)),
				rel.Author.Login,
				cs.Gray(text.TimeAgo(now, publishedAt(rel))))

			if body := strings.TrimSpace(rel.Body); body != "" {
				fmt.Fprintf(ios.Out, "\n%s\n", text.RemoveExcessBlankLines(body))
			}

			if len(rel.Assets) > 0 {
				fmt.Fprintf(ios.Out, "\n%s\n", cs.Bold("Assets"))
				tp := tableprinter.New(ios.Out, true, ios.TerminalWidth())
				tp.HeaderRow("name", "size", "downloads")
				for _, a := range rel.Assets {
					tp.AddField(a.Name, tableprinter.WithTruncate(true))
					tp.AddField(humanSize(a.Size), tableprinter.WithColor(cs.Gray), tableprinter.WithTruncate(false))
					tp.AddField(fmt.Sprintf("%d", a.DownloadCount), tableprinter.WithColor(cs.Gray), tableprinter.WithTruncate(false))
					tp.EndRow()
				}
				if err := tp.Render(); err != nil {
					return err
				}
			}

			fmt.Fprintf(ios.Out, "\n%s\n", cs.Gray("View on GitVerse: "+rel.HTMLURL))
			return nil
		},
	}

	cmd.Flags().BoolVarP(&web, "web", "w", false, "Open the release in a browser")
	cmd.Flags().BoolVar(&jsonOut.JSON, "json", false, "Output the release as JSON")
	cmd.Flags().StringVarP(&jsonOut.JQ, "jq", "q", "", "Filter the JSON output with a jq `expression`")
	cmd.Flags().StringVarP(&jsonOut.Template, "template", "t", "", "Format the JSON output with a Go `template`")

	return cmd
}

// humanSize renders a byte count the way package managers do.
func humanSize(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for size := n / unit; size >= unit; size /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(n)/float64(div), "KMGT"[exp])
}
