package main

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/deepnoodle-ai/wonton/assert"
	"github.com/deepnoodle-ai/wonton/cli"
	"github.com/deepnoodle-ai/wonton/color"
)

func TestDisassembly(t *testing.T) {
	// Disable colors for consistent test output
	oldEnabled := color.Enabled
	color.Enabled = false
	defer func() { color.Enabled = oldEnabled }()

	// Create the CLI app
	app := cli.New("risor").
		SetColorEnabled(false).
		GlobalFlags(
			cli.Bool("no-default-globals", "").Help("Disable the standard library"),
		)

	app.Command("dis").
		Args("file?").
		Flags(
			cli.String("code", "c").Help("Code to disassemble"),
			cli.Bool("stdin", "").Help("Read code from stdin"),
			cli.String("func", "").Help("Function to disassemble"),
		).
		Run(disHandler)

	// Capture stdout
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := app.ExecuteArgs([]string{"dis", "fixtures/ex1.risor"})

	w.Close()
	os.Stdout = old

	assert.Nil(t, err)

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
