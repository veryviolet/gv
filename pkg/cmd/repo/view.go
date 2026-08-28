package repo

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"gitverse.ru/veryviolet/gv/internal/api"
	"gitverse.ru/veryviolet/gv/internal/cmdutil"
	"gitverse.ru/veryviolet/gv/internal/gvrepo"
	"gitverse.ru/veryviolet/gv/internal/text"
)

// readmeCandidates are tried in order when rendering the repository overview.
var readmeCandidates = []string{"README.md", "README.rst", "README.txt", "README"}

// NewCmdView returns `gv repo view`.
func NewCmdView(f *cmdutil.Factory) *cobra.Command {
	var (
		web      bool
		branch   string
		jsonOut  cmdutil.JSONFlags
		showJSON bool
	)

	cmd := &cobra.Command{
		Use:   "view [<repository>]",
		Short: "Show a repository",
		Long: `Display the description, statistics and README of a repository.

Without an argument the repository of the current directory is used.`,
		Example: `  $ gv repo view
  $ gv repo view gitverse/rest-api-description
  $ gv repo view --web`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			jsonOut.JSON = showJSON
			if err := jsonOut.Validate(); err != nil {
				return err
			}
			arg := ""
			if len(args) > 0 {
				arg = args[0]
			}
			repo, err := cmdutil.RepoFromArg(f, arg)
			if err != nil {
				return err
			}
			if web {
				url := gvrepo.WebURL(repo, "")
				fmt.Fprintf(f.IOStreams.ErrOut, "Opening %s in your browser.\n", url)
				return f.Browser.Browse(url)
			}
			return viewRun(cmd.Context(), f, repo, branch, jsonOut)
		},
	}

	cmd.Flags().BoolVarP(&web, "web", "w", false, "Open the repository in a browser")
	cmd.Flags().StringVarP(&branch, "branch", "b", "", "Read the README from this `branch`")
	cmd.Flags().BoolVar(&showJSON, "json", false, "Output the repository as JSON")
	cmd.Flags().StringVarP(&jsonOut.JQ, "jq", "q", "", "Filter the JSON output with a jq `expression`")
	cmd.Flags().StringVarP(&jsonOut.Template, "template", "t", "", "Format the JSON output with a Go `template`")

	return cmd
}

func viewRun(ctx context.Context, f *cmdutil.Factory, repo gvrepo.Interface, branch string, jsonOut cmdutil.JSONFlags) error {
	client, err := f.HTTPClient()
	if err != nil {
		return err
	}

	r, err := api.GetRepository(ctx, client, repo)
	if err != nil {
		return err
	}

	if jsonOut.Enabled() {
		return cmdutil.PrintJSON(f.IOStreams, r, jsonOut.JQ, jsonOut.Template)
	}

	ios := f.IOStreams
	cs := ios.ColorScheme()

	if !ios.IsStdoutTTY() {
		fmt.Fprintf(ios.Out, "name:\t%s\n", r.FullName)
		fmt.Fprintf(ios.Out, "description:\t%s\n", r.Description)
		fmt.Fprintf(ios.Out, "visibility:\t%s\n", visibility(r))
		fmt.Fprintf(ios.Out, "default-branch:\t%s\n", r.DefaultBranch)
		fmt.Fprintf(ios.Out, "url:\t%s\n", r.HTMLURL)
		return nil
	}

	fmt.Fprintf(ios.Out, "%s\n", cs.Bold(r.FullName))
	if r.Description != "" {
		fmt.Fprintf(ios.Out, "%s\n", r.Description)
	}
	fmt.Fprintln(ios.Out)

	meta := []string{
		visibility(r),
		fmt.Sprintf("%s stars", plural(r.StargazersCount)),
		fmt.Sprintf("%s forks", plural(r.ForksCount)),
		fmt.Sprintf("%s open issues", plural(r.OpenIssuesCount)),
	}
	if r.Language != "" {
		meta = append(meta, r.Language)
	}
	if r.Fork && r.Parent != nil && r.Parent.FullName != "" {
		meta = append(meta, "fork of "+r.Parent.FullName)
	}
	if r.Archived {
		meta = append(meta, cs.Yellow("archived"))
	}
	fmt.Fprintf(ios.Out, "%s\n", cs.Gray(strings.Join(meta, " · ")))

	if langs, err := api.RepositoryLanguages(ctx, client, repo); err == nil && len(langs) > 0 {
		fmt.Fprintf(ios.Out, "%s\n", cs.Gray("languages: "+formatLanguages(langs)))
	}

	if readme := fetchREADME(ctx, client, repo, branch); readme != "" {
		fmt.Fprintln(ios.Out)
		fmt.Fprintln(ios.Out, text.RemoveExcessBlankLines(readme))
	}

	fmt.Fprintf(ios.Out, "\n%s\n", cs.Gray("View on GitVerse: "+r.HTMLURL))
	return nil
}

// fetchREADME returns the first README it can find, or an empty string. A
// missing README is not an error worth reporting.
func fetchREADME(ctx context.Context, client *api.Client, repo gvrepo.Interface, branch string) string {
	for _, name := range readmeCandidates {
		contents, err := api.GetContents(ctx, client, repo, name, branch)
		if err != nil {
			continue
		}
		body, err := contents.DecodedContent()
		if err != nil || strings.TrimSpace(body) == "" {
			continue
		}
		return body
	}
	return ""
}

func visibility(r *api.Repository) string {
	if r.Visibility != "" {
		return r.Visibility
	}
	if r.Private {
		return "private"
	}
	return "public"
}

func plural(n int64) string {
	return fmt.Sprintf("%d", n)
}

// formatLanguages renders the language breakdown largest-first.
func formatLanguages(langs map[string]int64) string {
	type entry struct {
		name  string
		bytes int64
	}
	entries := make([]entry, 0, len(langs))
	var total int64
	for name, size := range langs {
		entries = append(entries, entry{name, size})
		total += size
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].bytes > entries[j].bytes })
	if total == 0 {
		return ""
	}

	parts := make([]string, 0, len(entries))
	for i, e := range entries {
		if i == 3 {
			break
		}
		parts = append(parts, fmt.Sprintf("%s %.1f%%", e.name, float64(e.bytes)*100/float64(total)))
	}
	return strings.Join(parts, ", ")
}
