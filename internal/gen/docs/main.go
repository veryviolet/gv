// Command docs generates the man page and shell completions that the
// distribution packages install.
//
//	go run ./internal/gen/docs -out share
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra/doc"

	"gitverse.ru/veryviolet/gv/internal/build"
	"gitverse.ru/veryviolet/gv/internal/cmdutil"
	"gitverse.ru/veryviolet/gv/pkg/cmd/root"
)

func main() {
	outDir := flag.String("out", "share", "directory to write the generated files into")
	flag.Parse()

	f := cmdutil.NewFactory()
	rootCmd := root.NewCmdRoot(f)
	rootCmd.DisableAutoGenTag = true

	manDir := filepath.Join(*outDir, "man", "man1")
	if err := os.MkdirAll(manDir, 0o755); err != nil {
		fail(err)
	}
	header := &doc.GenManHeader{
		Title:   "GV",
		Section: "1",
		Source:  "gv " + build.Version,
		Manual:  "gv manual",
	}
	if err := doc.GenManTree(rootCmd, header, manDir); err != nil {
		fail(err)
	}

	complDir := filepath.Join(*outDir, "completions")
	if err := os.MkdirAll(complDir, 0o755); err != nil {
		fail(err)
	}
	completions := map[string]func(string) error{
		"gv.bash": rootCmd.GenBashCompletionFile,
		"gv.zsh":  rootCmd.GenZshCompletionFile,
		"gv.fish": func(path string) error { return rootCmd.GenFishCompletionFile(path, true) },
	}
	for name, gen := range completions {
		if err := gen(filepath.Join(complDir, name)); err != nil {
			fail(err)
		}
	}

	entries, _ := os.ReadDir(manDir)
	fmt.Fprintf(os.Stderr, "wrote %d man pages and %d completion scripts to %s\n",
		len(entries), len(completions), *outDir)
}

func fail(err error) {
	fmt.Fprintf(os.Stderr, "docs: %v\n", err)
	os.Exit(1)
}
