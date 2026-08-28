package cmdutil

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"gitverse.ru/veryviolet/gv/internal/export"
	"gitverse.ru/veryviolet/gv/internal/gvrepo"
	"gitverse.ru/veryviolet/gv/internal/iostreams"
)

// PrintJSON writes v as JSON, optionally filtered by a jq expression or
// rendered through a Go template. It is the shared implementation behind the
// --json/--jq/--template flags.
func PrintJSON(ios *iostreams.IOStreams, v any, jqExpr, tmpl string) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}

	switch {
	case jqExpr != "":
		return export.FilterJSON(ios.Out, bytes.NewReader(data), jqExpr)
	case tmpl != "":
		t := export.NewTemplate(ios, tmpl)
		if err := t.Execute(bytes.NewReader(data)); err != nil {
			return err
		}
		return t.Flush()
	default:
		var indented bytes.Buffer
		if err := json.Indent(&indented, data, "", "  "); err != nil {
			return err
		}
		fmt.Fprintln(ios.Out, indented.String())
		return nil
	}
}

// AddJSONFlags registers the trio of output flags on a command's flag set.
type JSONFlags struct {
	JSON     bool
	JQ       string
	Template string
}

// Enabled reports whether any machine-readable output was requested.
func (f JSONFlags) Enabled() bool {
	return f.JSON || f.JQ != "" || f.Template != ""
}

// Validate rejects combinations that make no sense.
func (f JSONFlags) Validate() error {
	if f.JQ != "" && f.Template != "" {
		return FlagErrorf("--jq cannot be combined with --template")
	}
	return nil
}

// ReadBody resolves the body of an issue, comment or pull request from the
// --body flag, a file, or standard input when the file is "-".
func ReadBody(ios *iostreams.IOStreams, body, bodyFile string) (string, error) {
	if bodyFile == "" {
		return body, nil
	}
	var data []byte
	var err error
	if bodyFile == "-" {
		data, err = io.ReadAll(ios.In)
	} else {
		data, err = os.ReadFile(bodyFile)
	}
	if err != nil {
		return "", err
	}
	return strings.TrimRight(string(data), "\n"), nil
}

// RepoFromArg resolves an "OWNER/REPO" argument, falling back to the
// repository in the working directory when arg is empty.
func RepoFromArg(f *Factory, arg string) (gvrepo.Interface, error) {
	if arg == "" {
		return f.BaseRepo()
	}
	repo, err := gvrepo.FromFullNameWithHost(arg, f.Host())
	if err != nil {
		return nil, FlagErrorf("%v", err)
	}
	return repo, nil
}
