package compiler

import (
	"context"
	"strings"
	"testing"

	"github.com/deepnoodle-ai/wonton/assert"
	"github.com/risor-io/risor/ast"
	"github.com/risor-io/risor/internal/token"
	"github.com/risor-io/risor/op"
	"github.com/risor-io/risor/parser"
)

func TestNil(t *testing.T) {
	c, err := New()
	assert.Nil(t, err)
	scope, err := c.Compile(&ast.Nil{})
	assert.Nil(t, err)
	assert.Equal(t, scope.InstructionCount(), 1)
	instr := scope.Instruction(0)
	assert.Equal(t, op.Code(instr), op.Nil)
}

func TestUndefinedVariable(t *testing.T) {
	c, err := New()
	assert.Nil(t, err)
	_, err = c.Compile(&ast.Ident{
		NamePos: token.Position{Line: 1, Column: 1},
		Name:    "foo",
	})
	assert.NotNil(t, err)
	assert.Equal(t, err.Error(), "compile error: undefined variable \"foo\"\n\nlocation: unknown:2:2 (line 2, column 2)")
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
			errMsg: "compile error: undefined variable \"x\"\n\nlocation: t.risor:1:1 (line 1, column 1)",
		},
		{
			name:   "undefined variable y",
			input:  "let x = 1;\ny = x + 1",
			errMsg: "compile error: undefined variable \"y\"\n\nlocation: t.risor:2:1 (line 2, column 1)",
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
			errMsg: "compile error: cannot assign to constant \"a\"\n\nlocation: t.risor:1:14 (line 1, column 14)",
		},
	}
	for _, tt := range testCase {
		t.Run(tt.name, func(t *testing.T) {
			c, err := New(WithFilename("t.risor"))
			assert.Nil(t, err)
			ast, err := parser.Parse(context.Background(), tt.input)
			assert.Nil(t, err)
			_, err = c.Compile(ast)
			assert.NotNil(t, err)
			assert.Equal(t, err.Error(), tt.errMsg)
		})
	}
}

func TestBadExprCompilation(t *testing.T) {
	c, err := New(WithFilename("test.risor"))
	assert.Nil(t, err)

	// Create a program with a BadExpr
	badExpr := &ast.BadExpr{
		From: token.Position{Line: 0, Column: 4, File: "test.risor"},
		To:   token.Position{Line: 0, Column: 10, File: "test.risor"},
	}

	_, err = c.Compile(badExpr)
	assert.NotNil(t, err)
	assert.True(t, strings.Contains(err.Error(), "syntax error in expression"))
}

func TestBadStmtCompilation(t *testing.T) {
	c, err := New(WithFilename("test.risor"))
	assert.Nil(t, err)

	// Create a program with a BadStmt
	badStmt := &ast.BadStmt{
		From: token.Position{Line: 0, Column: 0, File: "test.risor"},
		To:   token.Position{Line: 0, Column: 15, File: "test.risor"},
	}

	_, err = c.Compile(badStmt)
	assert.NotNil(t, err)
	assert.True(t, strings.Contains(err.Error(), "syntax error in statement"))
}

func TestBadExprInVarCompilation(t *testing.T) {
	c, err := New(WithFilename("test.risor"))
	assert.Nil(t, err)

	// Create a var statement with a BadExpr as value
	program := &ast.Program{
		Stmts: []ast.Node{
			&ast.Var{
				Let: token.Position{Line: 0, Column: 0},
				Name: &ast.Ident{
					NamePos: token.Position{Line: 0, Column: 4},
					Name:    "x",
				},
				Value: &ast.BadExpr{
					From: token.Position{Line: 0, Column: 8},
					To:   token.Position{Line: 0, Column: 15},
				},
			},
		},
	}

	_, err = c.Compile(program)
	assert.NotNil(t, err)
	assert.True(t, strings.Contains(err.Error(), "syntax error in expression"))
}

func TestBadStmtInProgramCompilation(t *testing.T) {
	c, err := New(WithFilename("test.risor"))
	assert.Nil(t, err)

	// Create a program with a BadStmt followed by valid code
	program := &ast.Program{
		Stmts: []ast.Node{
			&ast.BadStmt{
				From: token.Position{Line: 0, Column: 0},
				To:   token.Position{Line: 0, Column: 10},
			},
			&ast.Var{
				Let: token.Position{Line: 1, Column: 0},
				Name: &ast.Ident{
					NamePos: token.Position{Line: 1, Column: 4},
					Name:    "x",
				},
				Value: &ast.Int{
					ValuePos: token.Position{Line: 1, Column: 8},
					Value:    42,
				},
			},
		},
	}

	// Compilation should fail on the BadStmt
	_, err = c.Compile(program)
	assert.NotNil(t, err)
	assert.True(t, strings.Contains(err.Error(), "syntax error in statement"))
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
	assert.Nil(t, err)

	ast, err := parser.Parse(context.Background(), input)
	assert.Nil(t, err)

	code, err := c.Compile(ast)
	assert.Nil(t, err)

	// Compare the generated instructions
	actual := NewInstructionIter(code).All()

	assert.Equal(t, len(actual), len(expected),
		"instruction length mismatch. got=%d, want=%d",
		len(actual), len(expected))

	for i, want := range expected {
		got := actual[i]
		assert.Equal(t, got, want,
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
	assert.NoError(t, err)

	code, err := Compile(astNode)
	assert.NoError(t, err)

	assert.Equal(t, code.instructions, expectedCode)
	assert.Equal(t, code.constants, expectedConstants)
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
			assert.Nil(t, err)
			ast, err := parser.Parse(context.Background(), tt.input)
			assert.Nil(t, err)
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
				if (n == 0) {
					return true
				}
				return is_odd(n - 1)
			}

			function is_odd(n) {
				if (n == 0) {
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
				if (op == "add") {
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
			assert.Nil(t, err)

			ast, err := parser.Parse(context.Background(), tt.input)
			assert.Nil(t, err)

			_, err = c.Compile(ast)
			if tt.wantErr {
				assert.NotNil(t, err)
			} else {
				assert.Nil(t, err)
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
	assert.Nil(t, err)

	ast, err := parser.Parse(context.Background(), input)
	assert.Nil(t, err)

	code, err := c.Compile(ast)
	assert.Nil(t, err)

	// Verify that the code compiles successfully and has expected structure
	assert.NotNil(t, code)
	assert.Greater(t, code.InstructionCount(), 0)

	// Verify that the code compiles successfully and contains expected constants
	assert.Greater(t, code.ConstantsCount(), 0, "should have constants")

	// The main verification is that compilation succeeded without errors
	// indicating that forward declarations were properly resolved
}

func TestLocationTracking(t *testing.T) {
	// Test that locations are recorded for each instruction
	input := `let x = 42`

	c, err := New(WithFilename("test.risor"))
	assert.Nil(t, err)

	ast, err := parser.Parse(context.Background(), input)
	assert.Nil(t, err)

	code, err := c.Compile(ast)
	assert.Nil(t, err)

	// Verify locations are recorded
	assert.Greater(t, code.LocationsCount(), 0)
	assert.Equal(t, code.LocationsCount(), code.InstructionCount())

	// Verify location at instruction 0 has correct info
	loc := code.LocationAt(0)
	assert.Equal(t, loc.Filename, "test.risor")
	assert.Equal(t, loc.Line, 1)
	assert.Greater(t, loc.Column, 0)
}

func TestLocationTracking_MultiLine(t *testing.T) {
	input := `let x = 1
let y = 2
x + y`

	c, err := New(WithFilename("multi.risor"))
	assert.Nil(t, err)

	ast, err := parser.Parse(context.Background(), input)
	assert.Nil(t, err)

	code, err := c.Compile(ast)
	assert.Nil(t, err)

	// Collect unique line numbers from locations
	lines := make(map[int]bool)
	for i := 0; i < code.LocationsCount(); i++ {
		loc := code.LocationAt(i)
		if loc.Line > 0 {
			lines[loc.Line] = true
		}
	}

	// Should have instructions from multiple lines
	assert.GreaterOrEqual(t, len(lines), 2, "should have locations from multiple lines")
}

func TestLocationTracking_OutOfBounds(t *testing.T) {
	input := `42`

	c, err := New()
	assert.Nil(t, err)

	ast, err := parser.Parse(context.Background(), input)
	assert.Nil(t, err)

	code, err := c.Compile(ast)
	assert.Nil(t, err)

	// Test out-of-bounds access returns zero location
	loc := code.LocationAt(-1)
	assert.True(t, loc.IsZero())

	loc = code.LocationAt(code.LocationsCount() + 100)
	assert.True(t, loc.IsZero())
}

func TestLocationTracking_Function(t *testing.T) {
	input := `function add(a, b) {
	return a + b
}
add(1, 2)`

	c, err := New(WithFilename("func.risor"))
	assert.Nil(t, err)

	ast, err := parser.Parse(context.Background(), input)
	assert.Nil(t, err)

	code, err := c.Compile(ast)
	assert.Nil(t, err)

	// Main code should have locations
	assert.Greater(t, code.LocationsCount(), 0)

	// Flatten returns all code objects including nested functions
	allCode := code.Flatten()
	assert.Greater(t, len(allCode), 1, "should have function code")

	// Find a non-root code (the function)
	var funcCode *Code
	for _, c := range allCode {
		if !c.IsRoot() {
			funcCode = c
			break
		}
	}
	assert.NotNil(t, funcCode, "should find function code")
	assert.Greater(t, funcCode.LocationsCount(), 0)

	// Function code should have filename inherited from parent
	funcLoc := funcCode.LocationAt(0)
	assert.Equal(t, funcLoc.Filename, "func.risor")
}

func TestGetSourceLine(t *testing.T) {
	input := `let x = 1
let y = 2
let z = x + y`

	c, err := New()
	assert.Nil(t, err)

	ast, err := parser.Parse(context.Background(), input)
	assert.Nil(t, err)

	code, err := c.Compile(ast)
	assert.Nil(t, err)

	// Test getting source lines (1-indexed)
	// Note: The source is stored from the AST's String() representation
	assert.Contains(t, code.GetSourceLine(1), "let x = 1")
	assert.Contains(t, code.GetSourceLine(2), "let y = 2")
	assert.Contains(t, code.GetSourceLine(3), "let z =")

	// Out of bounds
	assert.Equal(t, code.GetSourceLine(0), "")
	assert.Equal(t, code.GetSourceLine(100), "")
}
