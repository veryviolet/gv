// Package api implements `gv api`, the raw escape hatch to the GitVerse REST API.
package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"gitverse.ru/veryviolet/gv/internal/api"
	"gitverse.ru/veryviolet/gv/internal/cmdutil"
	"gitverse.ru/veryviolet/gv/internal/export"
	"gitverse.ru/veryviolet/gv/internal/gvrepo"
	"gitverse.ru/veryviolet/gv/internal/iostreams"
)

type Options struct {
	IO         *iostreams.IOStreams
	HTTPClient func() (*api.Client, error)
	BaseRepo   func() (gvrepo.Interface, error)

	RequestPath   string
	RequestMethod string
	methodPassed  bool

	MagicFields []string
	RawFields   []string
	Headers     []string
	InputFile   string

	ShowResponseHeaders bool
	Paginate            bool
	Silent              bool
	Limit               int
	Template            string
	FilterOutput        string
}

// NewCmdAPI returns the `gv api` command.
func NewCmdAPI(f *cmdutil.Factory) *cobra.Command {
	opts := Options{
		IO:         f.IOStreams,
		HTTPClient: f.HTTPClient,
		BaseRepo:   f.BaseRepo,
	}

	cmd := &cobra.Command{
		Use:   "api <endpoint>",
		Short: "Make an authenticated GitVerse API request",
		Long: `Make an authenticated HTTP request to the GitVerse API.

The endpoint is a path such as "/user" or "repos/{owner}/{repo}/pulls"; a full
URL is also accepted. The placeholders {owner}, {repo} and {hostname} are
filled in from the repository in the current directory.

The default method is GET, or POST when parameters are supplied. Parameters are
sent as a query string for GET and HEAD, and as a JSON body otherwise.

With --paginate, every page of a collection is fetched and the arrays are
merged into a single JSON array.`,
		Example: `  # the authenticated user
  $ gv api /user

  # pull requests of the current repository
  $ gv api repos/{owner}/{repo}/pulls

  # every repository you can see, as a flat list of names
  $ gv api /user/repos --paginate --jq '.[].full_name'

  # create an issue
  $ gv api repos/{owner}/{repo}/issues -f title='Bug' -f body='Steps...'

  # close a pull request
  $ gv api -X PATCH repos/{owner}/{repo}/pulls/12 -f state=closed`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.RequestPath = args[0]
			opts.methodPassed = cmd.Flags().Changed("method")

			if opts.Paginate && !strings.EqualFold(opts.RequestMethod, http.MethodGet) && opts.methodPassed {
				return cmdutil.FlagErrorf("--paginate is only supported with GET requests")
			}
			if opts.Silent && opts.FilterOutput != "" {
				return cmdutil.FlagErrorf("--silent cannot be combined with --jq")
			}
			if opts.Silent && opts.Template != "" {
				return cmdutil.FlagErrorf("--silent cannot be combined with --template")
			}
			if opts.FilterOutput != "" && opts.Template != "" {
				return cmdutil.FlagErrorf("--jq cannot be combined with --template")
			}
			return run(cmd.Context(), &opts)
		},
	}

	cmd.Flags().StringVarP(&opts.RequestMethod, "method", "X", http.MethodGet, "HTTP method for the request")
	cmd.Flags().StringArrayVarP(&opts.MagicFields, "field", "F", nil, "Add a typed parameter in `key=value` format")
	cmd.Flags().StringArrayVarP(&opts.RawFields, "raw-field", "f", nil, "Add a string parameter in `key=value` format")
	cmd.Flags().StringArrayVarP(&opts.Headers, "header", "H", nil, "Add an HTTP request header in `key:value` format")
	cmd.Flags().StringVar(&opts.InputFile, "input", "", "Send the contents of `file` as the body (use \"-\" for stdin)")
	cmd.Flags().BoolVarP(&opts.ShowResponseHeaders, "include", "i", false, "Include the HTTP status line and response headers")
	cmd.Flags().BoolVar(&opts.Paginate, "paginate", false, "Fetch every page of a paginated collection")
	cmd.Flags().IntVar(&opts.Limit, "limit", 0, "Stop after fetching `n` items when paginating")
	cmd.Flags().BoolVar(&opts.Silent, "silent", false, "Do not print the response body")
	cmd.Flags().StringVarP(&opts.FilterOutput, "jq", "q", "", "Filter the JSON response with a jq `expression`")
	cmd.Flags().StringVarP(&opts.Template, "template", "t", "", "Format the JSON response with a Go `template`")

	return cmd
}

func run(ctx context.Context, opts *Options) error {
	client, err := opts.HTTPClient()
	if err != nil {
		return err
	}

	requestPath, err := fillPlaceholders(opts.RequestPath, opts.BaseRepo)
	if err != nil {
		return err
	}

	params, err := parseFields(opts.IO, opts.MagicFields, opts.RawFields, opts.BaseRepo)
	if err != nil {
		return err
	}

	method := strings.ToUpper(opts.RequestMethod)
	if !opts.methodPassed && (len(params) > 0 || opts.InputFile != "") {
		method = http.MethodPost
	}

	body, err := requestBody(opts, method, params, &requestPath)
	if err != nil {
		return err
	}

	if opts.Paginate {
		return runPaginated(ctx, opts, client, method, requestPath)
	}

	req, err := client.NewRequest(ctx, method, requestPath, body)
	if err != nil {
		return err
	}
	if err := applyHeaders(req, opts.Headers); err != nil {
		return err
	}

	resp, doErr := client.Do(req)
	if resp == nil {
		return doErr
	}
	defer resp.Body.Close()

	if opts.ShowResponseHeaders {
		printHeaders(opts.IO.Out, resp)
	}

	payload, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if !opts.Silent {
		if doErr != nil {
			// The body of a failed request is the server's error message, not
			// the data the filter was written for; print it verbatim.
			writeRaw(opts, payload)
		} else if err := writeOutput(opts, payload); err != nil {
			return err
		}
	}

	if doErr != nil {
		return httpFailure(opts, doErr)
	}
	return nil
}

// runPaginated walks every page and merges JSON arrays into one array so the
// output stays valid JSON for downstream tools.
func runPaginated(ctx context.Context, opts *Options, client *api.Client, method, requestPath string) error {
	var merged []json.RawMessage
	var nonArray [][]byte
	headersPrinted := false

	err := client.Paginate(ctx, method, requestPath, opts.Limit, func(body []byte, resp *http.Response) (bool, error) {
		if opts.ShowResponseHeaders && !headersPrinted {
			printHeaders(opts.IO.Out, resp)
			headersPrinted = true
		}
		var page []json.RawMessage
		if err := json.Unmarshal(body, &page); err != nil {
			// Not an array: pass the page through untouched.
			nonArray = append(nonArray, body)
			return false, nil
		}
		merged = append(merged, page...)
		return true, nil
	})
	if err != nil {
		return httpFailure(opts, err)
	}

	if opts.Silent {
		return nil
	}

	if len(nonArray) > 0 {
		for _, body := range nonArray {
			if err := writeOutput(opts, body); err != nil {
				return err
			}
		}
		return nil
	}

	if opts.Limit > 0 && len(merged) > opts.Limit {
		merged = merged[:opts.Limit]
	}
	if merged == nil {
		merged = []json.RawMessage{}
	}
	payload, err := json.Marshal(merged)
	if err != nil {
		return err
	}
	return writeOutput(opts, payload)
}

func writeOutput(opts *Options, payload []byte) error {
	switch {
	case opts.FilterOutput != "":
		if len(bytes.TrimSpace(payload)) == 0 {
			return nil
		}
		return export.FilterJSON(opts.IO.Out, bytes.NewReader(payload), opts.FilterOutput)
	case opts.Template != "":
		if len(bytes.TrimSpace(payload)) == 0 {
			return nil
		}
		tmpl := export.NewTemplate(opts.IO, opts.Template)
		if err := tmpl.Execute(bytes.NewReader(payload)); err != nil {
			return err
		}
		return tmpl.Flush()
	default:
		return writeRaw(opts, payload)
	}
}

// writeRaw copies the response body to stdout, adding the trailing newline the
// API tends to omit.
func writeRaw(opts *Options, payload []byte) error {
	if len(payload) == 0 {
		return nil
	}
	if _, err := opts.IO.Out.Write(payload); err != nil {
		return err
	}
	if !bytes.HasSuffix(payload, []byte("\n")) {
		fmt.Fprintln(opts.IO.Out)
	}
	return nil
}

// httpFailure keeps the response body on stdout (already printed) and turns the
// error into a quiet non-zero exit, after reporting it on stderr.
func httpFailure(opts *Options, err error) error {
	var httpErr *api.HTTPError
	if !errors.As(err, &httpErr) {
		return err
	}
	if httpErr.StatusCode == http.StatusUnauthorized {
		// main prints this one, together with the hint to log in.
		return cmdutil.NewAuthError(httpErr)
	}
	cs := opts.IO.ColorScheme()
	fmt.Fprintf(opts.IO.ErrOut, "%s %s\n", cs.FailureIcon(), httpErr.Error())
	return cmdutil.ErrSilent
}

// requestBody turns collected parameters into either a query string appended to
// path, or a JSON body.
func requestBody(opts *Options, method string, params map[string]any, path *string) (io.Reader, error) {
	if opts.InputFile != "" {
		if len(params) > 0 {
			return nil, cmdutil.FlagErrorf("--input cannot be combined with -f/-F")
		}
		if opts.InputFile == "-" {
			return opts.IO.In, nil
		}
		data, err := os.ReadFile(opts.InputFile)
		if err != nil {
			return nil, err
		}
		return bytes.NewReader(data), nil
	}

	if len(params) == 0 {
		return nil, nil
	}

	if method == http.MethodGet || method == http.MethodHead || method == http.MethodDelete {
		q := url.Values{}
		keys := make([]string, 0, len(params))
		for k := range params {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			q.Add(k, fmt.Sprintf("%v", params[k]))
		}
		sep := "?"
		if strings.Contains(*path, "?") {
			sep = "&"
		}
		*path += sep + q.Encode()
		return nil, nil
	}

	data, err := json.Marshal(params)
	if err != nil {
		return nil, err
	}
	return bytes.NewReader(data), nil
}

func applyHeaders(req *http.Request, headers []string) error {
	for _, h := range headers {
		key, value, ok := strings.Cut(h, ":")
		if !ok {
			return cmdutil.FlagErrorf("invalid header %q: expected \"key:value\"", h)
		}
		req.Header.Set(strings.TrimSpace(key), strings.TrimSpace(value))
	}
	return nil
}

func printHeaders(w io.Writer, resp *http.Response) {
	fmt.Fprintf(w, "%s %s\n", resp.Proto, resp.Status)
	keys := make([]string, 0, len(resp.Header))
	for k := range resp.Header {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Fprintf(w, "%s: %s\n", k, strings.Join(resp.Header[k], ", "))
	}
	fmt.Fprintln(w)
}

// parseFields builds the parameter map. -f values stay strings; -F values are
// interpreted: true/false/null, integers, @file and "-" for stdin.
func parseFields(io *iostreams.IOStreams, magic, raw []string, baseRepo func() (gvrepo.Interface, error)) (map[string]any, error) {
	params := map[string]any{}

	for _, f := range raw {
		key, value, ok := strings.Cut(f, "=")
		if !ok {
			return nil, cmdutil.FlagErrorf("invalid field %q: expected \"key=value\"", f)
		}
		filled, err := fillPlaceholders(value, baseRepo)
		if err != nil {
			return nil, err
		}
		params[key] = filled
	}

	for _, f := range magic {
		key, value, ok := strings.Cut(f, "=")
		if !ok {
			return nil, cmdutil.FlagErrorf("invalid field %q: expected \"key=value\"", f)
		}
		v, err := magicFieldValue(io, value, baseRepo)
		if err != nil {
			return nil, err
		}
		params[key] = v
	}

	return params, nil
}

func magicFieldValue(ios *iostreams.IOStreams, value string, baseRepo func() (gvrepo.Interface, error)) (any, error) {
	if strings.HasPrefix(value, "@") {
		name := value[1:]
		if name == "-" {
			data, err := io.ReadAll(ios.In)
			if err != nil {
				return nil, err
			}
			return string(data), nil
		}
		data, err := os.ReadFile(name)
		if err != nil {
			return nil, err
		}
		return string(data), nil
	}

	switch value {
	case "true":
		return true, nil
	case "false":
		return false, nil
	case "null":
		return nil, nil
	}
	if n, err := strconv.Atoi(value); err == nil {
		return n, nil
	}
	return fillPlaceholders(value, baseRepo)
}

// fillPlaceholders substitutes {owner}, {repo} and {branch} from the repository
// in the working directory. Values are path-escaped so that names containing
// unusual characters cannot break the URL.
func fillPlaceholders(value string, baseRepo func() (gvrepo.Interface, error)) (string, error) {
	if !strings.Contains(value, "{") || baseRepo == nil {
		return value, nil
	}

	var repo gvrepo.Interface
	var repoErr error
	lookup := func() (gvrepo.Interface, error) {
		if repo == nil && repoErr == nil {
			repo, repoErr = baseRepo()
		}
		return repo, repoErr
	}

	var sb strings.Builder
	rest := value
	for {
		before, after, found := strings.Cut(rest, "{")
		sb.WriteString(before)
		if !found {
			break
		}
		name, tail, closed := strings.Cut(after, "}")
		if !closed {
			sb.WriteString("{")
			sb.WriteString(after)
			break
		}
		switch name {
		case "owner":
			r, err := lookup()
			if err != nil {
				return "", err
			}
			sb.WriteString(url.PathEscape(r.RepoOwner()))
		case "repo":
			r, err := lookup()
			if err != nil {
				return "", err
			}
			sb.WriteString(url.PathEscape(r.RepoName()))
		case "hostname":
			r, err := lookup()
			if err != nil {
				return "", err
			}
			sb.WriteString(r.RepoHost())
		default:
			sb.WriteString("{" + name + "}")
		}
		rest = tail
	}
	return sb.String(), nil
}
