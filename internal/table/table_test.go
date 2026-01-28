package table

import (
	"bytes"
	"strings"
	"testing"

	"github.com/deepnoodle-ai/wonton/assert"
	"github.com/deepnoodle-ai/wonton/color"
)

func TestTable(t *testing.T) {
	var buf bytes.Buffer
	table := NewTable(&buf)
	table.WithHeader([]string{"HEADER1", "H2", "h3"})
	table.WithColumnAlignment([]Alignment{AlignLeft, AlignRight, AlignLeft})
	table.WithHeaderAlignment([]Alignment{AlignCenter, AlignCenter, AlignRight})
	table.Append([]string{"ROW1", "ROW2", "foo bar"})
	table.Append([]string{"a", "b", "c"})
	table.Render()

	expected := `
+---------+------+---------+
| HEADER1 |  H2  |      h3 |
+---------+------+---------+
| ROW1    | ROW2 | foo bar |
| a       |    b | c       |
+---------+------+---------+
`
	assert.Equal(t, buf.String(), strings.TrimSpace(expected)+"\n")
}

func TestColoredTable(t *testing.T) {
	var buf bytes.Buffer

	// Create a table with colored content
	table := NewTable(&buf)
	table.WithHeader([]string{"HEADER1", "HEADER2", "HEADER3"})
	table.WithColumnAlignment([]Alignment{AlignLeft, AlignRight, AlignLeft})
	table.WithHeaderAlignment([]Alignment{AlignCenter, AlignCenter, AlignCenter})

	// Add rows with colored content
	table.Append([]string{
		color.ApplyBold("Bold text"),
		"12345",
		color.Green.Sprint("Green text"),
	})

	table.Append([]string{
		"Normal",
		color.ApplyBold("999"),
		color.Green.Sprint("More color"),
	})

	// Render the table
	table.Render()

	// Check that the output has correct spacing and alignment
	result := buf.String()
	t.Log(result) // Log the actual output for visual inspection

	// Simple validation that color codes don't break alignment
	lines := strings.Split(result, "\n")
	assert.True(t, len(lines) >= 5, "Table should have at least 5 lines")

	// Check all lines have the same length
	expectedLength := len(lines[0])
	for i := 1; i < len(lines)-1; i++ { // Skip last line which might be empty
		assert.Equal(t, len(stripAnsi(lines[i])), expectedLength,
			"Line %d has incorrect length after stripping ANSI codes", i)
	}
}
