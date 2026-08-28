package pr

import (
	"fmt"

	"github.com/spf13/cobra"

	"gitverse.ru/veryviolet/gv/internal/api"
	"gitverse.ru/veryviolet/gv/internal/cmdutil"
	"gitverse.ru/veryviolet/gv/internal/tableprinter"
)

// NewCmdFiles returns `gv pr files`.
func NewCmdFiles(f *cmdutil.Factory) *cobra.Command {
	var jsonOut cmdutil.JSONFlags

	cmd := &cobra.Command{
		Use:     "files [<number>]",
		Short:   "List the files a pull request changes",
		Example: `  $ gv pr files 42`,
		Args:    cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := jsonOut.Validate(); err != nil {
				return err
			}
			arg := ""
			if len(args) > 0 {
				arg = args[0]
			}
			pr, repo, err := resolvePR(cmd.Context(), f, arg)
			if err != nil {
				return err
			}
			client, err := f.HTTPClient()
			if err != nil {
				return err
			}
			files, err := api.ListPullRequestFiles(cmd.Context(), client, repo, int(pr.Number), 0)
			if err != nil {
				return err
			}
			if jsonOut.Enabled() {
				return cmdutil.PrintJSON(f.IOStreams, files, jsonOut.JQ, jsonOut.Template)
			}

			ios := f.IOStreams
			cs := ios.ColorScheme()
			tp := tableprinter.New(ios.Out, ios.IsStdoutTTY(), ios.TerminalWidth())
			tp.HeaderRow("file", "status", "additions", "deletions")
			for _, file := range files {
				tp.AddField(file.Filename, tableprinter.WithTruncate(true))
				tp.AddField(file.Status, tableprinter.WithColor(cs.Gray), tableprinter.WithTruncate(false))
				tp.AddField(fmt.Sprintf("+%d", file.Additions), tableprinter.WithColor(cs.Green), tableprinter.WithTruncate(false))
				tp.AddField(fmt.Sprintf("-%d", file.Deletions), tableprinter.WithColor(cs.Red), tableprinter.WithTruncate(false))
				tp.EndRow()
			}
			return tp.Render()
		},
	}

	cmd.Flags().BoolVar(&jsonOut.JSON, "json", false, "Output the files as JSON")
	cmd.Flags().StringVarP(&jsonOut.JQ, "jq", "q", "", "Filter the JSON output with a jq `expression`")
	cmd.Flags().StringVarP(&jsonOut.Template, "template", "t", "", "Format the JSON output with a Go `template`")

	return cmd
}
