package dis

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/deepnoodle-ai/risor/v2/pkg/bytecode"
	"github.com/deepnoodle-ai/risor/v2/pkg/compiler"
	"github.com/deepnoodle-ai/risor/v2/pkg/op"
	"github.com/deepnoodle-ai/risor/v2/pkg/parser"
	"github.com/deepnoodle-ai/wonton/assert"
	"github.com/deepnoodle-ai/wonton/color"
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
	ast, err := parser.Parse(context.Background(), src, nil)
	assert.Nil(t, err)
	code, err := compiler.Compile(ast, &compiler.Config{GlobalNames: []string{"try", "error"}})
	assert.Nil(t, err)
	assert.Equal(t, code.ConstantCount(), 1)

	c := code.ConstantAt(0)
	f, ok := c.(*bytecode.Function)
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

func TestDisassembleLoadAttrOrNil(t *testing.T) {
	ast, err := parser.Parse(context.Background(), `let o = {}; o?.missing`, nil)
	assert.Nil(t, err)
	code, err := compiler.Compile(ast, nil)
	assert.Nil(t, err)
	instructions, err := Disassemble(code)
	assert.Nil(t, err)
	var found bool
	for _, instr := range instructions {
		if instr.Name == "LOAD_ATTR_OR_NIL" {
			found = true
			assert.Equal(t, instr.Annotation, "missing")
		}
	}
	assert.True(t, found)
}

func TestDisassembleInvalidInstructions(t *testing.T) {
	code := bytecode.NewCode(bytecode.CodeParams{
		Instructions: []op.Code{op.Nop, op.LoadConst},
	})
	_, err := Disassemble(code)
	assert.NotNil(t, err)
	assert.Equal(t, err.Error(), "truncated LOAD_CONST instruction at offset 1")
}
