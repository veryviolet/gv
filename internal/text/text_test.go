package text

import (
	"testing"
	"time"
)

func TestDisplayWidth(t *testing.T) {
	cases := map[string]int{
		"":                   0,
		"hello":              5,
		"привет":             6,
		"日本語":                6,
		"\x1b[31mred\x1b[0m": 3,
	}
	for in, want := range cases {
		if got := DisplayWidth(in); got != want {
			t.Errorf("DisplayWidth(%q) = %d, want %d", in, got, want)
		}
	}
}

func TestTruncate(t *testing.T) {
	cases := []struct {
		width int
		in    string
		want  string
	}{
		{10, "short", "short"},
		{8, "a longer string", "a lon..."},
		{3, "abcdef", "..."},
		{2, "abcdef", ".."},
		{0, "abcdef", ""},
	}
	for _, tc := range cases {
		if got := Truncate(tc.width, tc.in); got != tc.want {
			t.Errorf("Truncate(%d, %q) = %q, want %q", tc.width, tc.in, got, tc.want)
		}
		if w := DisplayWidth(Truncate(tc.width, tc.in)); w > tc.width {
			t.Errorf("Truncate(%d, %q) produced width %d", tc.width, tc.in, w)
		}
	}
}

func TestRelativeTimeAgo(t *testing.T) {
	now := time.Date(2026, 8, 28, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		ago  time.Duration
		want string
	}{
		{30 * time.Second, "less than a minute ago"},
		{time.Hour, "about 1 hour ago"},
		{5 * time.Hour, "about 5 hours ago"},
		{48 * time.Hour, "about 2 days ago"},
		{60 * 24 * time.Hour, "about 2 months ago"},
		{800 * 24 * time.Hour, "about 2 years ago"},
	}
	for _, tc := range cases {
		if got := RelativeTimeAgo(now, now.Add(-tc.ago)); got != tc.want {
			t.Errorf("RelativeTimeAgo(-%s) = %q, want %q", tc.ago, got, tc.want)
		}
	}
}

func TestParseAPITime(t *testing.T) {
	if _, ok := ParseAPITime("2026-08-28T09:30:00Z"); !ok {
		t.Error("RFC 3339 timestamps should parse")
	}
	if _, ok := ParseAPITime("not a time"); ok {
		t.Error("garbage should not parse")
	}
	if got := TimeAgo(time.Now(), "not a time"); got != "not a time" {
		t.Errorf("TimeAgo should pass unparsable input through, got %q", got)
	}
}

func TestRemoveExcessBlankLines(t *testing.T) {
	in := "first\n\n\n\nsecond\n   \n\nthird\n\n\n"
	want := "first\n\nsecond\n\nthird"
	if got := RemoveExcessBlankLines(in); got != want {
		t.Errorf("RemoveExcessBlankLines = %q, want %q", got, want)
	}
}

func TestIndentAndFirstLine(t *testing.T) {
	if got := Indent("a\n\nb", "  "); got != "  a\n\n  b" {
		t.Errorf("Indent = %q", got)
	}
	if got := FirstLine("subject\nbody"); got != "subject" {
		t.Errorf("FirstLine = %q", got)
	}
}

func TestPluralize(t *testing.T) {
	if got := Pluralize(1, "comment"); got != "1 comment" {
		t.Errorf("Pluralize(1) = %q", got)
	}
	if got := Pluralize(3, "comment"); got != "3 comments" {
		t.Errorf("Pluralize(3) = %q", got)
	}
}
