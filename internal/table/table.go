package table

import (
	"fmt"
	"io"
	"regexp"
	"strings"
)

// Alignment represents text alignment in a table column
type Alignment int

const (
	AlignLeft Alignment = iota
	AlignRight
	AlignCenter
)

// Table represents a simple ASCII table
type Table struct {
	writer       io.Writer
	header       []string
	rows         [][]string
	alignment    []Alignment
	columnWidths []int
	headerAlign  []Alignment // Optional separate alignment for headers
}

// ANSI color code pattern
var ansiPattern = regexp.MustCompile("\x1b\\[[0-9;]*m")

// NewTable creates a new table that will write to the given writer
func NewTable(writer io.Writer) *Table {
	return &Table{
		writer: writer,
	}
}

// WithHeader sets the table header
func (t *Table) WithHeader(header []string) *Table {
	t.header = header
	return t
}

// WithColumnAlignment sets the alignment for each column
func (t *Table) WithColumnAlignment(alignment []Alignment) *Table {
	t.alignment = alignment
	return t
}

// WithHeaderAlignment sets specific alignment for headers, separate from column content
func (t *Table) WithHeaderAlignment(alignment []Alignment) *Table {
	t.headerAlign = alignment
	return t
}

// Append adds a row to the table
func (t *Table) Append(row []string) {
	t.rows = append(t.rows, row)
}

// WithRows sets the rows of the table
func (t *Table) WithRows(rows [][]string) *Table {
	t.rows = rows
	return t
}

// stripAnsi removes ANSI color codes from a string
func stripAnsi(s string) string {
	return ansiPattern.ReplaceAllString(s, "")
}

// visibleLength returns the visible length of a string without ANSI codes
func visibleLength(s string) int {
	return len(stripAnsi(s))
}

// Render draws the table to the writer
func (t *Table) Render() {
	if len(t.header) == 0 && len(t.rows) == 0 {
		return
	}

	// Calculate column widths
	t.calculateColumnWidths()

	// Draw the table
	t.drawSeparator()
	if len(t.header) > 0 {
		t.drawHeader()
		t.drawSeparator()
	}
	for _, row := range t.rows {
		t.drawRow(row)
	}
	t.drawSeparator()
}

func (t *Table) calculateColumnWidths() {
	// Determine column count
	columnCount := len(t.header)
	if columnCount == 0 && len(t.rows) > 0 {
		columnCount = len(t.rows[0])
	}

	// Initialize column widths
	t.columnWidths = make([]int, columnCount)

	// Update widths from header
	for i, h := range t.header {
		h = strings.TrimSpace(h)
		visLen := visibleLength(h)
		if i < columnCount && visLen > t.columnWidths[i] {
			t.columnWidths[i] = visLen
		}
	}

	// Update widths from data
	for _, row := range t.rows {
		for i, cell := range row {
			visLen := visibleLength(cell)
			if i < columnCount && visLen > t.columnWidths[i] {
				t.columnWidths[i] = visLen
			}
		}
	}
}

func (t *Table) drawSeparator() {
	line := "+"
	for _, width := range t.columnWidths {
		line += strings.Repeat("-", width+2) + "+"
	}
	fmt.Fprintln(t.writer, line)
}

func (t *Table) drawHeader() {
	line := "|"
	for i, header := range t.header {
		width := t.columnWidths[i]
		visLen := visibleLength(header)
		padding := width - visLen

		// Determine which alignment to use for the header
		var alignment Alignment
		if i < len(t.headerAlign) {
			// Use header-specific alignment if set
			alignment = t.headerAlign[i]
		} else if i < len(t.alignment) {
			// Fall back to column alignment
			alignment = t.alignment[i]
		} else {
			// Default to left alignment
			alignment = AlignLeft
		}

		switch alignment {
		case AlignRight:
			line += " " + strings.Repeat(" ", padding) + header + " |"
		case AlignCenter:
			leftPad := padding / 2
			rightPad := padding - leftPad
			line += " " + strings.Repeat(" ", leftPad) + header + strings.Repeat(" ", rightPad) + " |"
		default: // AlignLeft
			line += " " + header + strings.Repeat(" ", padding) + " |"
		}
	}
	fmt.Fprintln(t.writer, line)
}

func (t *Table) drawRow(row []string) {
	line := "|"
	for i, width := range t.columnWidths {
		var cell string
		if i < len(row) {
			cell = row[i]
		} else {
			cell = ""
		}

		// ANSI escape codes don't count toward padding width
		// but we need to include them in the output
		visLen := visibleLength(cell)
		padding := width - visLen

		if i < len(t.alignment) {
			switch t.alignment[i] {
			case AlignRight:
				line += " " + strings.Repeat(" ", padding) + cell + " |"
			case AlignCenter:
				leftPad := padding / 2
				rightPad := padding - leftPad
				line += " " + strings.Repeat(" ", leftPad) + cell + strings.Repeat(" ", rightPad) + " |"
			default: // AlignLeft
				line += " " + cell + strings.Repeat(" ", padding) + " |"
			}
		} else {
			// Default to left alignment
			line += " " + cell + strings.Repeat(" ", padding) + " |"
		}
	}
	fmt.Fprintln(t.writer, line)
}
