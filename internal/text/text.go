// Package text holds the small string helpers shared by gv's output code.
package text

import (
	"fmt"
	"math"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

const ellipsis = "..."

// DisplayWidth reports how many terminal cells s occupies, counting East Asian
// wide characters as two and skipping ANSI escape sequences and combining marks.
func DisplayWidth(s string) int {
	w := 0
	inEscape := false
	for _, r := range s {
		if inEscape {
			if r == 'm' {
				inEscape = false
			}
			continue
		}
		if r == '\x1b' {
			inEscape = true
			continue
		}
		w += runeWidth(r)
	}
	return w
}

func runeWidth(r rune) int {
	switch {
	case r == 0:
		return 0
	case unicode.Is(unicode.Mn, r), unicode.Is(unicode.Me, r), unicode.Is(unicode.Cf, r):
		return 0
	case isWide(r):
		return 2
	default:
		return 1
	}
}

// isWide covers the ranges that matter in practice for repository metadata:
// CJK, Hangul, Kana and emoji.
func isWide(r rune) bool {
	switch {
	case r >= 0x1100 && r <= 0x115F, // Hangul Jamo
		r >= 0x2E80 && r <= 0xA4CF, // CJK radicals .. Yi
		r >= 0xAC00 && r <= 0xD7A3, // Hangul syllables
		r >= 0xF900 && r <= 0xFAFF, // CJK compatibility ideographs
		r >= 0xFE30 && r <= 0xFE6F, // CJK compatibility forms
		r >= 0xFF00 && r <= 0xFF60, // Fullwidth forms
		r >= 0xFFE0 && r <= 0xFFE6,
		r >= 0x1F300 && r <= 0x1F64F, // emoji
		r >= 0x1F900 && r <= 0x1F9FF,
		r >= 0x20000 && r <= 0x3FFFD:
		return true
	}
	return false
}

// Truncate shortens s to at most maxWidth display cells, appending an ellipsis.
func Truncate(maxWidth int, s string) string {
	if maxWidth <= 0 {
		return ""
	}
	if DisplayWidth(s) <= maxWidth {
		return s
	}
	if maxWidth <= len(ellipsis) {
		return strings.Repeat(".", maxWidth)
	}

	target := maxWidth - len(ellipsis)
	var b strings.Builder
	w := 0
	for _, r := range s {
		rw := runeWidth(r)
		if w+rw > target {
			break
		}
		b.WriteRune(r)
		w += rw
	}
	return b.String() + ellipsis
}

// Pluralize renders "1 thing" / "3 things".
func Pluralize(n int, thing string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, thing)
	}
	return fmt.Sprintf("%d %ss", n, thing)
}

// RelativeTimeAgo renders a coarse "about 3 hours ago" for a past time.
func RelativeTimeAgo(now, t time.Time) string {
	if t.IsZero() {
		return ""
	}
	ago := now.Sub(t)
	if ago < 0 {
		return "in the future"
	}
	switch {
	case ago < time.Minute:
		return "less than a minute ago"
	case ago < time.Hour:
		return fmtAgo(int(math.Round(ago.Minutes())), "minute")
	case ago < 24*time.Hour:
		return fmtAgo(int(math.Round(ago.Hours())), "hour")
	case ago < 30*24*time.Hour:
		return fmtAgo(int(math.Round(ago.Hours()/24)), "day")
	case ago < 365*24*time.Hour:
		return fmtAgo(int(math.Round(ago.Hours()/24/30)), "month")
	default:
		return fmtAgo(int(math.Round(ago.Hours()/24/365)), "year")
	}
}

func fmtAgo(n int, unit string) string {
	if n == 1 {
		return fmt.Sprintf("about 1 %s ago", unit)
	}
	return fmt.Sprintf("about %d %ss ago", n, unit)
}

// apiTimeLayouts are the timestamp formats the GitVerse API is known to emit.
var apiTimeLayouts = []string{
	time.RFC3339,
	"2006-01-02T15:04:05Z0700",
	"2006-01-02 15:04:05",
	"2006-01-02",
}

// ParseAPITime parses a timestamp from an API response.
func ParseAPITime(raw string) (time.Time, bool) {
	if raw == "" {
		return time.Time{}, false
	}
	for _, layout := range apiTimeLayouts {
		if ts, err := time.Parse(layout, raw); err == nil {
			return ts, true
		}
	}
	return time.Time{}, false
}

// TimeAgo renders an API timestamp as a relative time, passing through
// anything it cannot parse.
func TimeAgo(now time.Time, raw string) string {
	if ts, ok := ParseAPITime(raw); ok {
		return RelativeTimeAgo(now, ts)
	}
	return raw
}

// Indent prefixes every non-empty line of s.
func Indent(s, prefix string) string {
	if s == "" {
		return s
	}
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		if strings.TrimSpace(l) != "" {
			lines[i] = prefix + l
		}
	}
	return strings.Join(lines, "\n")
}

// RemoveExcessBlankLines collapses runs of blank lines down to one.
func RemoveExcessBlankLines(s string) string {
	lines := strings.Split(strings.ReplaceAll(s, "\r\n", "\n"), "\n")
	out := make([]string, 0, len(lines))
	blank := 0
	for _, l := range lines {
		if strings.TrimSpace(l) == "" {
			blank++
			if blank > 1 {
				continue
			}
		} else {
			blank = 0
		}
		out = append(out, strings.TrimRight(l, " \t"))
	}
	return strings.TrimRight(strings.Join(out, "\n"), "\n")
}

// FirstLine returns the first line of s, useful for commit subjects.
func FirstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}

// ValidUTF8 reports whether s is well-formed UTF-8.
func ValidUTF8(s string) bool { return utf8.ValidString(s) }
