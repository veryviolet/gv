package export

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"text/template"
	"time"

	"gitverse.ru/veryviolet/gv/internal/iostreams"
	"gitverse.ru/veryviolet/gv/internal/tableprinter"
	"gitverse.ru/veryviolet/gv/internal/text"
)

// Template renders JSON through a Go text/template, with the helper functions
// gh users expect: color, timeago, truncate, tablerow and friends.
type Template struct {
	io       *iostreams.IOStreams
	tmpl     *template.Template
	template string
	tp       *tableprinter.TablePrinter
}

// NewTemplate builds a Template writing to io.Out.
func NewTemplate(io *iostreams.IOStreams, tmpl string) *Template {
	return &Template{io: io, template: tmpl}
}

// Execute renders the JSON read from r.
func (t *Template) Execute(r io.Reader) error {
	if t.tmpl == nil {
		if err := t.parse(); err != nil {
			return err
		}
	}
	var data any
	dec := json.NewDecoder(r)
	dec.UseNumber()
	if err := dec.Decode(&data); err != nil {
		if err == io.EOF {
			return nil
		}
		return err
	}
	return t.tmpl.Execute(t.io.Out, data)
}

// Flush renders any table rows buffered by the tablerow function.
func (t *Template) Flush() error {
	if t.tp == nil {
		return nil
	}
	err := t.tp.Render()
	t.tp = nil
	return err
}

func (t *Template) parse() error {
	cs := t.io.ColorScheme()
	now := time.Now()

	funcs := template.FuncMap{
		"color": func(name string, v any) string {
			return cs.ColorFromString(name)(toString(v))
		},
		"autocolor": func(name string, v any) string {
			return cs.ColorFromString(name)(toString(v))
		},
		"join": func(sep string, v any) string {
			items, ok := v.([]any)
			if !ok {
				return toString(v)
			}
			parts := make([]string, len(items))
			for i, it := range items {
				parts[i] = toString(it)
			}
			return strings.Join(parts, sep)
		},
		"pluck": func(field string, v any) []any {
			items, ok := v.([]any)
			if !ok {
				return nil
			}
			out := make([]any, 0, len(items))
			for _, it := range items {
				if m, ok := it.(map[string]any); ok {
					out = append(out, m[field])
				}
			}
			return out
		},
		"timefmt": func(layout string, v any) (string, error) {
			ts, err := parseTime(v)
			if err != nil {
				return "", err
			}
			return ts.Format(layout), nil
		},
		"timeago": func(v any) (string, error) {
			ts, err := parseTime(v)
			if err != nil {
				return "", err
			}
			return text.RelativeTimeAgo(now, ts), nil
		},
		"truncate": func(width int, v any) string {
			return text.Truncate(width, toString(v))
		},
		"tablerow": func(fields ...any) (string, error) {
			if t.tp == nil {
				t.tp = tableprinter.New(t.io.Out, t.io.IsStdoutTTY(), t.io.TerminalWidth())
			}
			for _, f := range fields {
				t.tp.AddField(toString(f), tableprinter.WithTruncate(true))
			}
			t.tp.EndRow()
			return "", nil
		},
		"tablerender": func() (string, error) {
			return "", t.Flush()
		},
		"hyperlink": func(url, label any) string {
			u, l := toString(url), toString(label)
			if l == "" {
				l = u
			}
			if !t.io.IsStdoutTTY() {
				return l
			}
			return fmt.Sprintf("\x1b]8;;%s\x1b\\%s\x1b]8;;\x1b\\", u, l)
		},
	}

	tmpl, err := template.New("output").Funcs(funcs).Parse(t.template)
	if err != nil {
		return fmt.Errorf("invalid template: %w", err)
	}
	t.tmpl = tmpl
	return nil
}

func toString(v any) string {
	switch tv := v.(type) {
	case nil:
		return ""
	case string:
		return tv
	case json.Number:
		return tv.String()
	case bool:
		if tv {
			return "true"
		}
		return "false"
	case float64:
		return fmt.Sprintf("%v", tv)
	default:
		data, err := json.Marshal(v)
		if err != nil {
			return fmt.Sprintf("%v", v)
		}
		return string(data)
	}
}

// parseTime accepts the timestamp shapes the API returns: RFC 3339 strings and
// Unix seconds.
func parseTime(v any) (time.Time, error) {
	switch tv := v.(type) {
	case string:
		for _, layout := range []string{time.RFC3339, "2006-01-02T15:04:05Z0700", "2006-01-02 15:04:05", "2006-01-02"} {
			if ts, err := time.Parse(layout, tv); err == nil {
				return ts, nil
			}
		}
		return time.Time{}, fmt.Errorf("could not parse time %q", tv)
	case json.Number:
		secs, err := tv.Int64()
		if err != nil {
			return time.Time{}, err
		}
		return time.Unix(secs, 0), nil
	case float64:
		return time.Unix(int64(tv), 0), nil
	case nil:
		return time.Time{}, nil
	default:
		return time.Time{}, fmt.Errorf("could not parse time from %T", v)
	}
}
