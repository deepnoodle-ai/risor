package dis

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/risor-io/risor/compiler"
	"github.com/risor-io/risor/parser"
	"github.com/stretchr/testify/require"
)

func TestFunctionDissasembly(t *testing.T) {
	src := `
	function f() {
		42
		error("kaboom")
	}`
	ast, err := parser.Parse(context.Background(), src)
	require.Nil(t, err)
	code, err := compiler.Compile(ast, compiler.WithGlobalNames([]string{"try", "error"}))
	require.Nil(t, err)
	require.Equal(t, 1, code.ConstantsCount())

	f := code.Constant(0)
	require.IsType(t, &compiler.Function{}, f)
	instructions, err := Disassemble(f.(*compiler.Function).Code())
	require.Nil(t, err)

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
	require.Equal(t, expected+"\n", result)
}
