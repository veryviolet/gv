package export

import (
	"bytes"
	"strings"
	"testing"

	"gitverse.ru/veryviolet/gv/internal/iostreams"
)

func TestFilterJSON(t *testing.T) {
	input := `[{"full_name":"veryviolet/gv","stars":3},{"full_name":"veryviolet/other","stars":0}]`

	cases := []struct {
		query string
		want  string
	}{
		{".[].full_name", "veryviolet/gv\nveryviolet/other\n"},
		{"length", "2\n"},
		{".[0].stars", "3\n"},
		{"[.[].full_name]", "[\"veryviolet/gv\",\"veryviolet/other\"]\n"},
	}

	for _, tc := range cases {
		var out bytes.Buffer
		if err := FilterJSON(&out, strings.NewReader(input), tc.query); err != nil {
			t.Errorf("FilterJSON(%q): %v", tc.query, err)
			continue
		}
		if out.String() != tc.want {
			t.Errorf("FilterJSON(%q) = %q, want %q", tc.query, out.String(), tc.want)
		}
	}
}

func TestFilterJSONRejectsBadExpression(t *testing.T) {
	var out bytes.Buffer
	err := FilterJSON(&out, strings.NewReader(`{}`), ".[")
	if err == nil {
		t.Fatal("expected an error for a malformed jq expression")
	}
	if !strings.Contains(err.Error(), "invalid jq expression") {
		t.Errorf("error = %v", err)
	}
}

func TestTemplateHelpers(t *testing.T) {
	ios, _, out, _ := iostreams.Test()
	input := `{"title":"a rather long title","login":"veryviolet"}`

	tmpl := NewTemplate(ios, `{{truncate 10 .title}}|{{.login}}`)
	if err := tmpl.Execute(strings.NewReader(input)); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if err := tmpl.Flush(); err != nil {
		t.Fatalf("Flush: %v", err)
	}
	if got, want := out.String(), "a rathe...|veryviolet"; got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
}

func TestTemplateTableRow(t *testing.T) {
	ios, _, out, _ := iostreams.Test()
	input := `[{"n":"one"},{"n":"two"}]`

	tmpl := NewTemplate(ios, `{{range .}}{{tablerow .n}}{{end}}{{tablerender}}`)
	if err := tmpl.Execute(strings.NewReader(input)); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if got, want := out.String(), "one\ntwo\n"; got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
}

func TestTemplateRejectsBadTemplate(t *testing.T) {
	ios, _, _, _ := iostreams.Test()
	tmpl := NewTemplate(ios, `{{`)
	if err := tmpl.Execute(strings.NewReader(`{}`)); err == nil {
		t.Fatal("expected an error for a malformed template")
	}
}
