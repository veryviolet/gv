package pr

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"gitverse.ru/veryviolet/gv/internal/api"
	"gitverse.ru/veryviolet/gv/internal/cmdutil"
)

// NewCmdDiff returns `gv pr diff`.
func NewCmdDiff(f *cmdutil.Factory) *cobra.Command {
	var nameOnly bool

	cmd := &cobra.Command{
		Use:   "diff [<number>]",
		Short: "Show the changes of a pull request",
		Long: `Print the patch of a pull request.

The diff is assembled from the per-file patches the API returns, so it is a
unified diff without the commit headers a "git format-patch" would carry.`,
		Example: `  $ gv pr diff
  $ gv pr diff 42 --name-only`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
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

			ios := f.IOStreams
			cs := ios.ColorScheme()

			if nameOnly {
				for _, file := range files {
					fmt.Fprintln(ios.Out, file.Filename)
				}
				return nil
			}

			if len(files) == 0 {
				fmt.Fprintln(ios.ErrOut, "This pull request changes no files.")
				return nil
			}

			for _, file := range files {
				fmt.Fprintf(ios.Out, "%s\n", cs.Bold("diff --gv "+file.Filename))
				if file.Patch == "" {
					fmt.Fprintf(ios.Out, "%s\n\n", cs.Gray(fmt.Sprintf("(%s, no textual diff available)", file.Status)))
					continue
				}
				fmt.Fprintln(ios.Out, colorizePatch(cs, file.Patch))
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&nameOnly, "name-only", false, "Print only the names of the changed files")

	return cmd
}

// colorizePatch applies the usual diff colours line by line.
func colorizePatch(cs interface {
	Green(string) string
	Red(string) string
	Cyan(string) string
}, patch string) string {
	lines := strings.Split(patch, "\n")
	for i, line := range lines {
		switch {
		case strings.HasPrefix(line, "+++"), strings.HasPrefix(line, "---"):
			// Leave file headers uncoloured so they stand apart from hunks.
		case strings.HasPrefix(line, "@@"):
			lines[i] = cs.Cyan(line)
		case strings.HasPrefix(line, "+"):
			lines[i] = cs.Green(line)
		case strings.HasPrefix(line, "-"):
			lines[i] = cs.Red(line)
		}
	}
	return strings.Join(lines, "\n")
}
