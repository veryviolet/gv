package tableprinter

import (
	"bytes"
	"strings"
	"testing"
)

func TestTSVOutputWhenNotATTY(t *testing.T) {
	var buf bytes.Buffer
	tp := New(&buf, false, 80)
	tp.HeaderRow("id", "title")
	tp.AddField("#1")
	tp.AddField("first")
	tp.EndRow()
	tp.AddField("#2")
	tp.AddField("second")
	tp.EndRow()

	if err := tp.Render(); err != nil {
		t.Fatalf("Render: %v", err)
	}

	want := "#1\tfirst\n#2\tsecond\n"
	if buf.String() != want {
		t.Errorf("output = %q, want %q", buf.String(), want)
	}
}

func TestTSVSanitisesTabsAndNewlines(t *testing.T) {
	var buf bytes.Buffer
	tp := New(&buf, false, 80)
	tp.AddField("has\ttab")
	tp.AddField("has\nnewline")
	tp.EndRow()

	if err := tp.Render(); err != nil {
		t.Fatalf("Render: %v", err)
	}
	if got := buf.String(); got != "has tab\thas newline\n" {
		t.Errorf("output = %q", got)
	}
}

func TestTableOutputAlignsColumnsAndPrintsHeader(t *testing.T) {
	var buf bytes.Buffer
	tp := New(&buf, true, 80)
	tp.HeaderRow("id", "title")
	tp.AddField("#1")
	tp.AddField("first")
	tp.EndRow()
	tp.AddField("#100")
	tp.AddField("second")
	tp.EndRow()

	if err := tp.Render(); err != nil {
		t.Fatalf("Render: %v", err)
	}

	lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
	if len(lines) != 3 {
		t.Fatalf("got %d lines, want 3:\n%s", len(lines), buf.String())
	}
	if !strings.HasPrefix(lines[0], "ID") {
		t.Errorf("header line = %q, want it to start with ID", lines[0])
	}
	// The title column must start at the same offset in both data rows.
	if strings.Index(lines[1], "first") != strings.Index(lines[2], "second") {
		t.Errorf("columns are not aligned:\n%q\n%q", lines[1], lines[2])
	}
}

func TestTableTruncatesToTerminalWidth(t *testing.T) {
	var buf bytes.Buffer
	tp := New(&buf, true, 20)
	tp.AddField("#1", WithTruncate(false))
	tp.AddField(strings.Repeat("x", 60), WithTruncate(true))
	tp.EndRow()

	if err := tp.Render(); err != nil {
		t.Fatalf("Render: %v", err)
	}

	line := strings.TrimRight(buf.String(), "\n")
	if len(line) > 20 {
		t.Errorf("line is %d wide, want at most 20: %q", len(line), line)
	}
	if !strings.HasSuffix(line, "...") {
		t.Errorf("truncated cell should end with an ellipsis: %q", line)
	}
}

func TestRenderWithNoRowsWritesNothing(t *testing.T) {
	var buf bytes.Buffer
	tp := New(&buf, true, 80)
	tp.HeaderRow("id")
	if err := tp.Render(); err != nil {
		t.Fatalf("Render: %v", err)
	}
	if buf.Len() != 0 {
		t.Errorf("expected no output, got %q", buf.String())
	}
}
