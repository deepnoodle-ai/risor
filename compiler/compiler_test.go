package compiler

import (
	"context"
	"strings"
	"testing"

	"github.com/risor-io/risor/ast"
	"github.com/risor-io/risor/internal/token"
	"github.com/risor-io/risor/op"
	"github.com/risor-io/risor/parser"
	"github.com/stretchr/testify/require"
)

func TestNil(t *testing.T) {
	c, err := New()
	require.Nil(t, err)
	scope, err := c.Compile(&ast.Nil{})
	require.Nil(t, err)
	require.Equal(t, 1, scope.InstructionCount())
	instr := scope.Instruction(0)
	require.Equal(t, op.Nil, op.Code(instr))
}

func TestUndefinedVariable(t *testing.T) {
	c, err := New()
	require.Nil(t, err)
	_, err = c.Compile(ast.NewIdent(token.Token{
		Type:          token.IDENT,
		Literal:       "foo",
		StartPosition: token.Position{Line: 1, Column: 1},
	}))
	require.NotNil(t, err)
	require.Equal(t, "compile error: undefined variable \"foo\"\n\nlocation: unknown:2:2 (line 2, column 2)", err.Error())
}

func TestCompileErrors(t *testing.T) {
	testCase := []struct {
		name   string
		input  string
		errMsg string
	}{
		{
			name:   "undefined variable foo",
			input:  "foo",
			errMsg: "compile error: undefined variable \"foo\"\n\nlocation: t.risor:1:1 (line 1, column 1)",
		},
		{
			name:   "undefined variable x",
			input:  "x = 1",
			errMsg: "compile error: undefined variable \"x\"\n\nlocation: t.risor:1:3 (line 1, column 3)",
		},
		{
			name:   "undefined variable y",
			input:  "let x = 1;\ny = x + 1",
			errMsg: "compile error: undefined variable \"y\"\n\nlocation: t.risor:2:3 (line 2, column 3)",
		},
		{
			name:   "undefined variable z",
			input:  "\n\n z++;",
			errMsg: "compile error: undefined variable \"z\"\n\nlocation: t.risor:3:2 (line 3, column 2)",
		},
		{
			name:   "invalid argument defaults",
			input:  "function bad(a=1, b) {}",
			errMsg: "compile error: invalid argument defaults for function \"bad\"\n\nlocation: t.risor:1:1 (line 1, column 1)",
		},
		{
			name:   "invalid argument defaults for anonymous function",
			input:  "function(a=1, b) {}()",
			errMsg: "compile error: invalid argument defaults for anonymous function\n\nlocation: t.risor:1:1 (line 1, column 1)",
		},
		{
			name:   "unsupported default value",
			input:  "function(a, b=[1,2,3]) {}()",
			errMsg: "compile error: unsupported default value (got [1, 2, 3], line 1)",
		},
		{
			name:   "cannot assign to constant",
			input:  "const a = 1; a = 2",
			errMsg: "compile error: cannot assign to constant \"a\"\n\nlocation: t.risor:1:16 (line 1, column 16)",
		},
	}
	for _, tt := range testCase {
		t.Run(tt.name, func(t *testing.T) {
			c, err := New(WithFilename("t.risor"))
			require.Nil(t, err)
			ast, err := parser.Parse(context.Background(), tt.input)
			require.Nil(t, err)
			_, err = c.Compile(ast)
			require.NotNil(t, err)
			require.Equal(t, tt.errMsg, err.Error())
		})
	}
}

func TestCompoundAssignmentWithIndex(t *testing.T) {
	// test[0] *= 3
	input := "let test = [1, 2]; test[0] *= 3"
	expected := [][]op.Code{
		{op.LoadConst, 0}, // 1
		{op.LoadConst, 1}, // 2
		{op.BuildList, 2},
		{op.StoreGlobal, 0}, // store into 'test'
		{op.LoadGlobal, 0},  // load 'test'
		{op.LoadConst, 2},   // load index 0
		{op.BinarySubscr},   // get test[0]
		{op.LoadConst, 3},   // load 3
		{op.BinaryOp, op.Code(op.Multiply)},
		{op.LoadGlobal, 0}, // load 'test' again
		{op.LoadConst, 4},  // load index 0
		{op.StoreSubscr},   // store result back in test[0]
		{op.Nil},           // implicit return value
	}

	c, err := New()
	require.Nil(t, err)

	ast, err := parser.Parse(context.Background(), input)
	require.Nil(t, err)

	code, err := c.Compile(ast)
	require.Nil(t, err)

	// Compare the generated instructions
	actual := NewInstructionIter(code).All()

	require.Equal(t, len(expected), len(actual),
		"instruction length mismatch. got=%d, want=%d",
		len(actual), len(expected))

	for i, want := range expected {
		got := actual[i]
		require.Equal(t, want, got,
			"wrong instruction at pos %d. got=%v, want=%v",
			i, got, want)
	}
}

func TestBitwiseAnd(t *testing.T) {
	input := `3 & 1`
	expectedCode := []op.Code{
		op.LoadConst, 0, // 3
		op.LoadConst, 1, // 1
		op.BinaryOp,
		op.Code(op.BitwiseAnd),
	}
	expectedConstants := []interface{}{int64(3), int64(1)}

	astNode, err := parser.Parse(context.Background(), input)
	require.NoError(t, err)

	code, err := Compile(astNode)
	require.NoError(t, err)

	require.Equal(t, expectedCode, code.instructions)
	require.Equal(t, expectedConstants, code.constants)
}

func TestFunctionRedefinition(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name: "duplicate function definition",
			input: `
function bar() {
    print("first bar")
}

function bar() {
    print("second bar")
}
`,
			expected: `function "bar" redefined`,
		},
		{
			name: "multiple duplicate function definitions",
			input: `
function foo() {
    print("first foo")
}

function foo() {
    print("second foo")
}

function foo() {
    print("third foo")
}
`,
			expected: `function "foo" redefined`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, err := New()
			require.Nil(t, err)
			ast, err := parser.Parse(context.Background(), tt.input)
			require.Nil(t, err)
			_, err = c.Compile(ast)
			if err == nil {
				t.Errorf("Expected error but got none")
				return
			}
			if !strings.Contains(err.Error(), tt.expected) {
				t.Errorf("Expected error containing %q, got %q", tt.expected, err.Error())
			}
		})
	}
}

func TestForwardDeclarationCompilation(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name: "basic forward declaration",
			input: `
			function main() {
				return helper()
			}
			
			function helper() {
				return 42
			}
			`,
			wantErr: false,
		},
		{
			name: "mutual recursion",
			input: `
			function is_even(n) {
				if n == 0 {
					return true
				}
				return is_odd(n - 1)
			}
			
			function is_odd(n) {
				if n == 0 {
					return false
				}
				return is_even(n - 1)
			}
			`,
			wantErr: false,
		},
		{
			name: "multiple forward declarations",
			input: `
			function a() {
				return b() + c()
			}
			
			function b() {
				return d()
			}
			
			function c() {
				return 10
			}
			
			function d() {
				return 20
			}
			`,
			wantErr: false,
		},
		{
			name: "forward declaration with closures",
			input: `
			function outer() {
				let x = 10
				return function() {
					return inner() + x
				}
			}
			
			function inner() {
				return 5
			}
			`,
			wantErr: false,
		},
		{
			name: "forward declaration with default parameters",
			input: `
			function caller(op="add") {
				if op == "add" {
					return adder(5, 3)
				}
				return multiplier(5, 3)
			}
			
			function adder(a, b) {
				return a + b
			}
			
			function multiplier(a, b) {
				return a * b
			}
			`,
			wantErr: false,
		},
		{
			name: "undefined function should error",
			input: `
			function caller() {
				return undefined_function()
			}
			`,
			wantErr: true,
		},
		{
			name: "function redefinition should error",
			input: `
			function duplicate() {
				return 1
			}
			
			function duplicate() {
				return 2
			}
			`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, err := New()
			require.Nil(t, err)

			ast, err := parser.Parse(context.Background(), tt.input)
			require.Nil(t, err)

			_, err = c.Compile(ast)
			if tt.wantErr {
				require.NotNil(t, err)
			} else {
				require.Nil(t, err)
			}
		})
	}
}

func TestForwardDeclarationInstructionGeneration(t *testing.T) {
	// Test that forward declarations generate correct instructions
	input := `
	function main() {
		return helper(5)
	}
	
	function helper(x) {
		return x * 2
	}
	
	main()
	`

	c, err := New()
	require.Nil(t, err)

	ast, err := parser.Parse(context.Background(), input)
	require.Nil(t, err)

	code, err := c.Compile(ast)
	require.Nil(t, err)

	// Verify that the code compiles successfully and has expected structure
	require.NotNil(t, code)
	require.Greater(t, code.InstructionCount(), 0)

	// Verify that the code compiles successfully and contains expected constants
	require.Greater(t, code.ConstantsCount(), 0, "should have constants")

	// The main verification is that compilation succeeded without errors
	// indicating that forward declarations were properly resolved
}
