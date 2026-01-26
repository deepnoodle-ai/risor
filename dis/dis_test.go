package dis

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/deepnoodle-ai/wonton/assert"
	"github.com/deepnoodle-ai/wonton/color"
	"github.com/risor-io/risor/compiler"
	"github.com/risor-io/risor/parser"
)

func TestFunctionDissasembly(t *testing.T) {
	// Disable colors for consistent test output
	color.Enabled = false
	defer func() { color.Enabled = true }()
	src := `
	function f() {
		42
		error("kaboom")
	}`
	ast, err := parser.Parse(context.Background(), src)
	assert.Nil(t, err)
	code, err := compiler.Compile(ast, compiler.WithGlobalNames([]string{"try", "error"}))
	assert.Nil(t, err)
	assert.Equal(t, code.ConstantsCount(), 1)

	c := code.Constant(0)
	f, ok := c.(*compiler.Function)
	assert.True(t, ok)
	instructions, err := Disassemble(f.Code())
	assert.Nil(t, err)

	var buf bytes.Buffer
	Print(instructions, &buf)

	result := buf.String()
	expected := strings.TrimSpace(`
+--------+--------------+----------+----------+
| OFFSET |    OPCODE    | OPERANDS |   INFO   |
+--------+--------------+----------+----------+
|      0 | LOAD_CONST   |        0 | 42       |
|      2 | POP_TOP      |          |          |
|      3 | LOAD_GLOBAL  |        0 | error    |
|      5 | LOAD_CONST   |        1 | "kaboom" |
|      7 | CALL         |        1 |          |
|      9 | RETURN_VALUE |          |          |
+--------+--------------+----------+----------+
`)
	assert.Equal(t, result, expected+"\n")
}
