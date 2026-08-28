package api

import (
	"strings"
	"testing"

	"gitverse.ru/veryviolet/gv/internal/gvrepo"
	"gitverse.ru/veryviolet/gv/internal/iostreams"
)

func testRepo() func() (gvrepo.Interface, error) {
	return func() (gvrepo.Interface, error) {
		return gvrepo.New("veryviolet", "gv"), nil
	}
}

func TestFillPlaceholders(t *testing.T) {
	cases := map[string]string{
		"/user":                           "/user",
		"repos/{owner}/{repo}/pulls":      "repos/veryviolet/gv/pulls",
		"repos/{owner}/{repo}":            "repos/veryviolet/gv",
		"{hostname}/x":                    "gitverse.ru/x",
		"repos/{owner}/{repo}/issues/{n}": "repos/veryviolet/gv/issues/{n}",
		"unterminated {owner":             "unterminated {owner",
	}
	for in, want := range cases {
		got, err := fillPlaceholders(in, testRepo())
		if err != nil {
			t.Errorf("fillPlaceholders(%q): %v", in, err)
			continue
		}
		if got != want {
			t.Errorf("fillPlaceholders(%q) = %q, want %q", in, got, want)
		}
	}
}

// A path with no placeholder must not trigger repository resolution, so that
// `gv api /user` works outside a git checkout.
func TestFillPlaceholdersSkipsRepoLookupWhenUnused(t *testing.T) {
	called := false
	lookup := func() (gvrepo.Interface, error) {
		called = true
		return nil, nil
	}
	if _, err := fillPlaceholders("/user", lookup); err != nil {
		t.Fatalf("fillPlaceholders: %v", err)
	}
	if called {
		t.Error("the base repository was resolved for a path without placeholders")
	}
}

func TestParseFields(t *testing.T) {
	ios, _, _, _ := iostreams.Test()

	params, err := parseFields(ios,
		[]string{"draft=true", "milestone=3", "assignee=null", "title={repo}"},
		[]string{"body=plain string", "number=12"},
		testRepo())
	if err != nil {
		t.Fatalf("parseFields: %v", err)
	}

	if v, ok := params["draft"].(bool); !ok || !v {
		t.Errorf("draft = %#v, want true", params["draft"])
	}
	if v, ok := params["milestone"].(int); !ok || v != 3 {
		t.Errorf("milestone = %#v, want int 3", params["milestone"])
	}
	if params["assignee"] != nil {
		t.Errorf("assignee = %#v, want nil", params["assignee"])
	}
	if params["title"] != "gv" {
		t.Errorf("title = %#v, want the expanded repository name", params["title"])
	}
	// -f keeps everything a string, even something that looks numeric.
	if v, ok := params["number"].(string); !ok || v != "12" {
		t.Errorf("number = %#v, want the string \"12\"", params["number"])
	}
	if params["body"] != "plain string" {
		t.Errorf("body = %#v", params["body"])
	}
}

func TestParseFieldsRejectsMalformedInput(t *testing.T) {
	ios, _, _, _ := iostreams.Test()
	if _, err := parseFields(ios, nil, []string{"nokey"}, testRepo()); err == nil {
		t.Error("expected an error for a field without \"=\"")
	}
	if _, err := parseFields(ios, []string{"nokey"}, nil, testRepo()); err == nil {
		t.Error("expected an error for a magic field without \"=\"")
	}
}

func TestMagicFieldValueReadsStdin(t *testing.T) {
	ios, stdin, _, _ := iostreams.Test()
	stdin.WriteString("body from stdin\n")

	v, err := magicFieldValue(ios, "@-", testRepo())
	if err != nil {
		t.Fatalf("magicFieldValue: %v", err)
	}
	if s, ok := v.(string); !ok || !strings.Contains(s, "body from stdin") {
		t.Errorf("value = %#v", v)
	}
}
