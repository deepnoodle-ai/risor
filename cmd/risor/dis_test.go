package main

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/deepnoodle-ai/wonton/assert"
	"github.com/deepnoodle-ai/wonton/color"
)

func TestDisassembly(t *testing.T) {
	// Disable colors for consistent test output
	color.Enabled = false
	defer func() { color.Enabled = true }()

	// Capture stdout
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	defer func() { os.Stdout = old }()

	disCmd.Run(disCmd, []string{"fixtures/ex1.risor"})

	w.Close()

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	capturedOutput := buf.String()
	expected := `
+--------+------------+----------+------+
| OFFSET |   OPCODE   | OPERANDS | INFO |
+--------+------------+----------+------+
|      0 | LOAD_CONST |        0 | 3    |
|      2 | LOAD_CONST |        1 | 4    |
|      4 | BINARY_OP  |        1 | +    |
+--------+------------+----------+------+
`
	assert.Equal(t, capturedOutput, strings.TrimPrefix(expected, "\n"))
}
