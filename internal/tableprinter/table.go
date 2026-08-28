// Package tableprinter renders tabular output: aligned and coloured on a
// terminal, tab-separated and colourless when piped.
package tableprinter

import (
	"fmt"
	"io"
	"strings"

	"gitverse.ru/veryviolet/gv/internal/text"
)

// Field flags.
const (
	// WidthMaxFlag marks a column that may be truncated when space runs short.
	WidthMaxFlag = 1 << iota
	// NoTruncate marks a column that must never be truncated.
	NoTruncate
)

type field struct {
	text  string
	color func(string) string
	flags int
}

// TablePrinter accumulates rows and renders them on Render.
type TablePrinter struct {
	out      io.Writer
	rows     [][]field
	headers  []string
	maxWidth int
	tty      bool
	padding  int
}

// New returns a printer. When isTTY is false the output is TSV, which keeps gv
// usable in pipelines.
func New(out io.Writer, isTTY bool, maxWidth int) *TablePrinter {
	return &TablePrinter{
		out:      out,
		tty:      isTTY,
		maxWidth: maxWidth,
		padding:  2,
	}
}

// HeaderRow sets column headers. Headers are only rendered on a terminal.
func (t *TablePrinter) HeaderRow(columns ...string) {
	t.headers = columns
}

// AddField appends a cell to the current row.
func (t *TablePrinter) AddField(s string, opts ...Option) {
	f := field{text: s}
	for _, o := range opts {
		o(&f)
	}
	if len(t.rows) == 0 {
		t.rows = append(t.rows, nil)
	}
	last := len(t.rows) - 1
	t.rows[last] = append(t.rows[last], f)
}

// EndRow closes the current row.
func (t *TablePrinter) EndRow() {
	t.rows = append(t.rows, nil)
}

// Option customises a field.
type Option func(*field)

// WithColor renders the field through fn when colour is in use.
func WithColor(fn func(string) string) Option {
	return func(f *field) { f.color = fn }
}

// WithTruncate marks the field as truncatable (true) or protected (false).
func WithTruncate(truncatable bool) Option {
	return func(f *field) {
		if truncatable {
			f.flags |= WidthMaxFlag
		} else {
			f.flags |= NoTruncate
		}
	}
}

// Render writes the accumulated table.
func (t *TablePrinter) Render() error {
	rows := t.rows
	// EndRow leaves a trailing empty row; drop it.
	if len(rows) > 0 && len(rows[len(rows)-1]) == 0 {
		rows = rows[:len(rows)-1]
	}
	if len(rows) == 0 {
		return nil
	}

	if !t.tty {
		return t.renderTSV(rows)
	}
	return t.renderTable(rows)
}

func (t *TablePrinter) renderTSV(rows [][]field) error {
	for _, row := range rows {
		cells := make([]string, len(row))
		for i, f := range row {
			// Tabs and newlines would corrupt the record structure.
			cells[i] = strings.NewReplacer("\t", " ", "\n", " ", "\r", "").Replace(f.text)
		}
		if _, err := fmt.Fprintln(t.out, strings.Join(cells, "\t")); err != nil {
			return err
		}
	}
	return nil
}

func (t *TablePrinter) renderTable(rows [][]field) error {
	numCols := 0
	for _, r := range rows {
		if len(r) > numCols {
			numCols = len(r)
		}
	}
	if len(t.headers) > numCols {
		numCols = len(t.headers)
	}

	widths := make([]int, numCols)
	for i, h := range t.headers {
		widths[i] = text.DisplayWidth(h)
	}
	for _, r := range rows {
		for i, f := range r {
			if w := text.DisplayWidth(f.text); w > widths[i] {
				widths[i] = w
			}
		}
	}

	t.shrinkToFit(widths, rows)

	if len(t.headers) > 0 {
		header := make([]field, len(t.headers))
		for i, h := range t.headers {
			header[i] = field{text: strings.ToUpper(h)}
		}
		if err := t.writeRow(header, widths, numCols); err != nil {
			return err
		}
	}
	for _, r := range rows {
		if err := t.writeRow(r, widths, numCols); err != nil {
			return err
		}
	}
	return nil
}

// shrinkToFit narrows truncatable columns until the table fits the terminal.
func (t *TablePrinter) shrinkToFit(widths []int, rows [][]field) {
	if t.maxWidth <= 0 {
		return
	}
	total := func() int {
		sum := 0
		for _, w := range widths {
			sum += w
		}
		return sum + t.padding*(len(widths)-1)
	}
	if total() <= t.maxWidth {
		return
	}

	truncatable := map[int]bool{}
	for _, r := range rows {
		for i, f := range r {
			if f.flags&NoTruncate != 0 {
				truncatable[i] = false
				continue
			}
			if f.flags&WidthMaxFlag != 0 {
				if _, seen := truncatable[i]; !seen {
					truncatable[i] = true
				}
			}
		}
	}

	// Shave the widest truncatable column one cell at a time; simple, and the
	// column count here is always small.
	const minWidth = 5
	for total() > t.maxWidth {
		widest, widestIdx := 0, -1
		for i, w := range widths {
			if truncatable[i] && w > widest && w > minWidth {
				widest, widestIdx = w, i
			}
		}
		if widestIdx < 0 {
			return
		}
		widths[widestIdx]--
	}
}

func (t *TablePrinter) writeRow(row []field, widths []int, numCols int) error {
	var b strings.Builder
	for i := 0; i < numCols; i++ {
		var f field
		if i < len(row) {
			f = row[i]
		}
		cell := text.Truncate(widths[i], f.text)
		pad := widths[i] - text.DisplayWidth(cell)
		if f.color != nil {
			cell = f.color(cell)
		}
		b.WriteString(cell)
		if i < numCols-1 {
			b.WriteString(strings.Repeat(" ", pad+t.padding))
		}
	}
	_, err := fmt.Fprintln(t.out, strings.TrimRight(b.String(), " "))
	return err
}
