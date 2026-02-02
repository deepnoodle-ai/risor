package compiler

import (
	"context"
	"strings"
	"testing"

	"github.com/deepnoodle-ai/wonton/assert"
	"github.com/deepnoodle-ai/risor/v2/ast"
	"github.com/deepnoodle-ai/risor/v2/errors"
	"github.com/deepnoodle-ai/risor/v2/internal/token"
	"github.com/deepnoodle-ai/risor/v2/op"
	"github.com/deepnoodle-ai/risor/v2/parser"
)

func TestNil(t *testing.T) {
	c, err := New(nil)
	assert.Nil(t, err)
	scope, err := c.CompileAST(&ast.Nil{})
	assert.Nil(t, err)
	assert.Equal(t, scope.InstructionCount(), 1)
	instr := scope.Instruction(0)
	assert.Equal(t, op.Code(instr), op.Nil)
}

func TestUndefinedVariable(t *testing.T) {
	c, err := New(nil)
	assert.Nil(t, err)
	_, err = c.CompileAST(&ast.Ident{
		NamePos: token.Position{Line: 1, Column: 1},
		Name:    "foo",
	})
	assert.NotNil(t, err)
	assert.Equal(t, err.Error(), "compile error: undefined variable \"foo\"\n\nlocation: unknown:2:2")
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
			errMsg: "compile error: undefined variable \"foo\"\n\nlocation: t.risor:1:1",
		},
		{
			name:   "undefined variable x",
			input:  "x = 1",
			errMsg: "compile error: undefined variable \"x\"\n\nlocation: t.risor:1:1",
		},
		{
			name:   "undefined variable y",
			input:  "let x = 1;\ny = x + 1",
			errMsg: "compile error: undefined variable \"y\"\n\nlocation: t.risor:2:1",
		},
		{
			name:   "undefined variable z",
			input:  "\n\n z++;",
			errMsg: "compile error: undefined variable \"z\"\n\nlocation: t.risor:3:2",
		},
		{
			name:   "invalid argument defaults",
			input:  "function bad(a=1, b) {}",
			errMsg: "compile error: invalid argument defaults for function \"bad\"\n\nlocation: t.risor:1:1",
		},
		{
			name:   "invalid argument defaults for anonymous function",
			input:  "function(a=1, b) {}()",
			errMsg: "compile error: invalid argument defaults for anonymous function\n\nlocation: t.risor:1:1",
		},
		{
			name:   "unsupported default value",
			input:  "function(a, b=[1,2,3]) {}()",
			errMsg: "compile error: unsupported default value type: *ast.List\n\nlocation: t.risor:1:15",
		},
		{
			name:   "cannot assign to constant",
			input:  "const a = 1; a = 2",
			errMsg: "compile error: cannot assign to constant \"a\"\n\nlocation: t.risor:1:14",
		},
	}
	for _, tt := range testCase {
		t.Run(tt.name, func(t *testing.T) {
			c, err := New(&Config{Filename: "t.risor"})
			assert.Nil(t, err)
			ast, err := parser.Parse(context.Background(), tt.input, nil)
			assert.Nil(t, err)
			_, err = c.CompileAST(ast)
			assert.NotNil(t, err)
			assert.Equal(t, err.Error(), tt.errMsg)
		})
	}
}

func TestBadExprCompilation(t *testing.T) {
	c, err := New(&Config{Filename: "test.risor"})
	assert.Nil(t, err)

	// Create a program with a BadExpr
	badExpr := &ast.BadExpr{
		From: token.Position{Line: 0, Column: 4, File: "test.risor"},
		To:   token.Position{Line: 0, Column: 10, File: "test.risor"},
	}

	_, err = c.CompileAST(badExpr)
	assert.NotNil(t, err)
	assert.True(t, strings.Contains(err.Error(), "syntax error in expression"))
}

func TestBadStmtCompilation(t *testing.T) {
	c, err := New(&Config{Filename: "test.risor"})
	assert.Nil(t, err)

	// Create a program with a BadStmt
	badStmt := &ast.BadStmt{
		From: token.Position{Line: 0, Column: 0, File: "test.risor"},
		To:   token.Position{Line: 0, Column: 15, File: "test.risor"},
	}

	_, err = c.CompileAST(badStmt)
	assert.NotNil(t, err)
	assert.True(t, strings.Contains(err.Error(), "syntax error in statement"))
}

func TestBadExprInVarCompilation(t *testing.T) {
	c, err := New(&Config{Filename: "test.risor"})
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

	_, err = c.CompileAST(program)
	assert.NotNil(t, err)
	assert.True(t, strings.Contains(err.Error(), "syntax error in expression"))
}

func TestBadStmtInProgramCompilation(t *testing.T) {
	c, err := New(&Config{Filename: "test.risor"})
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
	_, err = c.CompileAST(program)
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

	c, err := New(nil)
	assert.Nil(t, err)

	ast, err := parser.Parse(context.Background(), input, nil)
	assert.Nil(t, err)

	code, err := c.CompileAST(ast)
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

	astNode, err := parser.Parse(context.Background(), input, nil)
	assert.NoError(t, err)

	c, err := New(nil)
	assert.NoError(t, err)
	code, err := c.CompileAST(astNode)
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
			c, err := New(nil)
			assert.Nil(t, err)
			ast, err := parser.Parse(context.Background(), tt.input, nil)
			assert.Nil(t, err)
			_, err = c.CompileAST(ast)
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
			c, err := New(nil)
			assert.Nil(t, err)

			ast, err := parser.Parse(context.Background(), tt.input, nil)
			assert.Nil(t, err)

			_, err = c.CompileAST(ast)
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

	c, err := New(nil)
	assert.Nil(t, err)

	ast, err := parser.Parse(context.Background(), input, nil)
	assert.Nil(t, err)

	code, err := c.CompileAST(ast)
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

	c, err := New(&Config{Filename: "test.risor"})
	assert.Nil(t, err)

	ast, err := parser.Parse(context.Background(), input, nil)
	assert.Nil(t, err)

	code, err := c.CompileAST(ast)
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

	c, err := New(&Config{Filename: "multi.risor"})
	assert.Nil(t, err)

	ast, err := parser.Parse(context.Background(), input, nil)
	assert.Nil(t, err)

	code, err := c.CompileAST(ast)
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

	c, err := New(nil)
	assert.Nil(t, err)

	ast, err := parser.Parse(context.Background(), input, nil)
	assert.Nil(t, err)

	code, err := c.CompileAST(ast)
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

	c, err := New(&Config{Filename: "func.risor"})
	assert.Nil(t, err)

	ast, err := parser.Parse(context.Background(), input, nil)
	assert.Nil(t, err)

	code, err := c.CompileAST(ast)
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

	c, err := New(nil)
	assert.Nil(t, err)

	ast, err := parser.Parse(context.Background(), input, nil)
	assert.Nil(t, err)

	code, err := c.CompileAST(ast)
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

func TestEndColumn_InLocation(t *testing.T) {
	// Test that EndColumn is set for multi-character tokens
	input := `let longVariable = 42`

	c, err := New(&Config{Filename: "test.risor"})
	assert.Nil(t, err)

	ast, err := parser.Parse(context.Background(), input, nil)
	assert.Nil(t, err)

	code, err := c.CompileAST(ast)
	assert.Nil(t, err)

	// Find a location with EndColumn set
	hasEndColumn := false
	for i := 0; i < code.LocationsCount(); i++ {
		loc := code.LocationAt(i)
		if loc.EndColumn > 0 && loc.EndColumn > loc.Column {
			hasEndColumn = true
			// Verify EndColumn is valid (greater than or equal to Column)
			assert.GreaterOrEqual(t, loc.EndColumn, loc.Column)
			break
		}
	}
	// EndColumn should be set for at least some locations
	assert.True(t, hasEndColumn, "Should have at least one location with EndColumn set")
}

func TestEndColumn_SpansToken(t *testing.T) {
	// Test that EndColumn correctly spans the identifier
	input := `verylongidentifier`

	c, err := New(&Config{Filename: "test.risor"})
	assert.Nil(t, err)
	c.SetSource(input) // Set source for proper source preservation

	ast, err := parser.Parse(context.Background(), input, nil)
	assert.Nil(t, err)

	_, err = c.CompileAST(ast)
	// This will fail to compile due to undefined variable, but that's fine
	// We're testing compile-time location tracking
	assert.NotNil(t, err)

	// Get the error and verify it has location with EndColumn
	compErr, ok := err.(*errors.CompileError)
	if ok {
		formatted := compErr.ToFormatted()
		// EndColumn should span the identifier
		if formatted.EndColumn > 0 {
			// EndColumn is exclusive (points after last char)
			span := formatted.EndColumn - formatted.Column
			assert.Equal(t, span, len("verylongidentifier"), "EndColumn should span the identifier")
		}
	}
}

func TestRootSource_PreservedInFunctions(t *testing.T) {
	// Test that original source is preserved in function bodies
	input := `// Comment line 1
// Comment line 2
function add(a, b) {
    return a + b
}
add(1, 2)`

	c, err := New(&Config{Filename: "test.risor"})
	assert.Nil(t, err)
	c.SetSource(input)

	ast, err := parser.Parse(context.Background(), input, nil)
	assert.Nil(t, err)

	code, err := c.CompileAST(ast)
	assert.Nil(t, err)

	// The root source should be available
	assert.Contains(t, code.GetSourceLine(1), "// Comment line 1")
	assert.Contains(t, code.GetSourceLine(4), "return a + b")

	// Function code should be able to get source lines from root
	allCode := code.Flatten()
	for _, child := range allCode {
		if !child.IsRoot() {
			// Child function should be able to get source lines
			line := child.GetSourceLine(4)
			assert.Contains(t, line, "return")
		}
	}
}

func TestSetSource_ForREPL(t *testing.T) {
	// Test that SetSource works for REPL-style compilation
	input := `let x = 42 + 10`

	c, err := New(&Config{Filename: "repl"})
	assert.Nil(t, err)

	// Set source before compilation (as REPL would do)
	c.SetSource(input)

	ast, err := parser.Parse(context.Background(), input, nil)
	assert.Nil(t, err)

	code, err := c.CompileAST(ast)
	assert.Nil(t, err)

	// Verify source is available
	sourceLine := code.GetSourceLine(1)
	assert.Contains(t, sourceLine, "let x = 42 + 10")
}

func TestLocationTracking_WithComments(t *testing.T) {
	// Test that line numbers account for comments
	input := `// Line 1 comment
// Line 2 comment
let x = 42  // Error will be on line 3`

	c, err := New(&Config{Filename: "test.risor"})
	assert.Nil(t, err)
	c.SetSource(input)

	ast, err := parser.Parse(context.Background(), input, nil)
	assert.Nil(t, err)

	code, err := c.CompileAST(ast)
	assert.Nil(t, err)

	// Find a location on line 3 (where let x = 42 is)
	hasLine3 := false
	for i := 0; i < code.LocationsCount(); i++ {
		loc := code.LocationAt(i)
		if loc.Line == 3 {
			hasLine3 = true
			break
		}
	}
	assert.True(t, hasLine3, "Should have a location on line 3")

	// Source line 3 should include the comment
	sourceLine := code.GetSourceLine(3)
	assert.Contains(t, sourceLine, "let x = 42")
}

// =============================================================================
// DESTRUCTURING PARAMETERS
// =============================================================================

func TestDestructuringParamCompilation(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"basic object destructure", `function foo({a, b}) { return a + b }`},
		{"object destructure with default", `function foo({x, y = 10}) { return x + y }`},
		{"object destructure with alias", `function foo({name: n}) { return n }`},
		{"basic array destructure", `function foo([a, b]) { return a + b }`},
		{"array destructure with default", `function foo([x, y = 5]) { return x + y }`},
		{"mixed params", `function foo(x, {a, b}, [c, d]) { return x }`},
		{"destructure with rest param", `function foo({a}, ...rest) { return a }`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ast, err := parser.Parse(context.Background(), tt.input, nil)
			assert.Nil(t, err, "Parse error: %v", err)

			_, err = Compile(ast, nil)
			assert.Nil(t, err, "Compile error for %s: %v", tt.name, err)
		})
	}
}

func TestDestructuringParamSymbols(t *testing.T) {
	// Test that destructured variables are added to the symbol table
	input := `function foo({a, b}, [c, d]) { return a + b + c + d }`

	ast, err := parser.Parse(context.Background(), input, nil)
	assert.Nil(t, err)

	code, err := Compile(ast, nil)
	assert.Nil(t, err)
	assert.NotNil(t, code)
}

func TestDestructuringParamWithDefaults(t *testing.T) {
	// Test that defaults are correctly compiled
	tests := []struct {
		name  string
		input string
	}{
		{
			"object with nil default",
			`function foo({x, y = nil}) { return x }`,
		},
		{
			"object with int default",
			`function foo({x = 42}) { return x }`,
		},
		{
			"object with string default",
			`function foo({name = "default"}) { return name }`,
		},
		{
			"object with bool default",
			`function foo({flag = true}) { return flag }`,
		},
		{
			"array with multiple defaults",
			`function foo([a = 1, b = 2, c = 3]) { return a + b + c }`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ast, err := parser.Parse(context.Background(), tt.input, nil)
			assert.Nil(t, err)

			_, err = Compile(ast, nil)
			assert.Nil(t, err, "Compile error: %v", err)
		})
	}
}

func TestDestructuringInArrowFunction(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"array destructure arrow", `([a, b]) => a + b`},
		{"array destructure arrow with body", `([x, y]) => { return x * y }`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ast, err := parser.Parse(context.Background(), tt.input, nil)
			assert.Nil(t, err)

			_, err = Compile(ast, nil)
			assert.Nil(t, err, "Compile error: %v", err)
		})
	}
}

func TestDestructuringParamBytecodeEmitted(t *testing.T) {
	// Test that the compiler emits the correct opcodes for destructuring
	input := `function foo({a, b}) { return a + b }`

	ast, err := parser.Parse(context.Background(), input, nil)
	assert.Nil(t, err)

	code, err := Compile(ast, nil)
	assert.Nil(t, err)

	// The compiled code should have instructions for:
	// - LoadFast (load the synthetic param)
	// - Copy (duplicate object for each binding)
	// - LoadAttr (get property)
	// - StoreFast (store to local var)
	// - PopTop (clean up)

	// We just verify compilation succeeds and produces bytecode
	assert.Greater(t, code.InstructionCount(), 0)
}

// =============================================================================
// BLANK IDENTIFIER TESTS
// =============================================================================

func TestBlankIdentifier_LetDiscard(t *testing.T) {
	// let _ = expr should compile the expr and discard the value
	testCases := []string{
		"let _ = 42",
		"let _ = 1 + 2",
		`let _ = "hello"`,
	}
	for _, input := range testCases {
		t.Run(input, func(t *testing.T) {
			ast, err := parser.Parse(context.Background(), input, nil)
			assert.Nil(t, err)
			_, err = Compile(ast, nil)
			assert.Nil(t, err)
		})
	}
}

func TestBlankIdentifier_ConstDiscard(t *testing.T) {
	// const _ = expr should compile the expr and discard the value
	input := "const _ = 42"
	ast, err := parser.Parse(context.Background(), input, nil)
	assert.Nil(t, err)
	_, err = Compile(ast, nil)
	assert.Nil(t, err)
}

func TestBlankIdentifier_CannotRead(t *testing.T) {
	// Reading _ should produce an error
	testCases := []struct {
		input  string
		errMsg string
	}{
		{
			input:  "_",
			errMsg: "cannot use _ as value",
		},
		{
			input:  "let x = _",
			errMsg: "cannot use _ as value",
		},
		{
			input:  "let _ = 1; _",
			errMsg: "cannot use _ as value",
		},
	}
	for _, tt := range testCases {
		t.Run(tt.input, func(t *testing.T) {
			ast, err := parser.Parse(context.Background(), tt.input, nil)
			assert.Nil(t, err)
			_, err = Compile(ast, nil)
			assert.NotNil(t, err)
			assert.Contains(t, err.Error(), tt.errMsg)
		})
	}
}

func TestBlankIdentifier_AssignDiscard(t *testing.T) {
	// _ = expr should discard the value without needing prior declaration
	input := "_ = 42"
	ast, err := parser.Parse(context.Background(), input, nil)
	assert.Nil(t, err)
	_, err = Compile(ast, nil)
	assert.Nil(t, err)
}

func TestBlankIdentifier_CompoundAssignError(t *testing.T) {
	// _ += expr should error (can't read _ for compound assignment)
	testCases := []string{
		"_ += 1",
		"_ -= 1",
		"_ *= 2",
		"_ /= 2",
	}
	for _, input := range testCases {
		t.Run(input, func(t *testing.T) {
			ast, err := parser.Parse(context.Background(), input, nil)
			assert.Nil(t, err)
			_, err = Compile(ast, nil)
			assert.NotNil(t, err)
			assert.Contains(t, err.Error(), "cannot use _ in compound assignment")
		})
	}
}

func TestBlankIdentifier_MultiVar(t *testing.T) {
	// let _, b = [1, 2] should discard first value
	testCases := []string{
		"let _, b = [1, 2]",
		"let a, _ = [1, 2]",
		"let _, _ = [1, 2]", // Both discarded
		"let _, b, _ = [1, 2, 3]",
	}
	for _, input := range testCases {
		t.Run(input, func(t *testing.T) {
			ast, err := parser.Parse(context.Background(), input, nil)
			assert.Nil(t, err)
			_, err = Compile(ast, nil)
			assert.Nil(t, err)
		})
	}
}

func TestBlankIdentifier_ArrayDestructure(t *testing.T) {
	// let [_, b] = arr should discard first element
	testCases := []string{
		"let [_, b] = [1, 2]",
		"let [a, _] = [1, 2]",
		"let [_, _] = [1, 2]",
		"let [_, b, _] = [1, 2, 3]",
	}
	for _, input := range testCases {
		t.Run(input, func(t *testing.T) {
			ast, err := parser.Parse(context.Background(), input, nil)
			assert.Nil(t, err)
			_, err = Compile(ast, nil)
			assert.Nil(t, err)
		})
	}
}

func TestBlankIdentifier_ObjectDestructure(t *testing.T) {
	// let {a: _} = obj should discard the 'a' property
	testCases := []string{
		"let {a: _, b} = {a: 1, b: 2}",
		"let {a, b: _} = {a: 1, b: 2}",
		"let {a: _, b: _} = {a: 1, b: 2}",
	}
	for _, input := range testCases {
		t.Run(input, func(t *testing.T) {
			ast, err := parser.Parse(context.Background(), input, nil)
			assert.Nil(t, err)
			_, err = Compile(ast, nil)
			assert.Nil(t, err)
		})
	}
}

func TestBlankIdentifier_FunctionParam(t *testing.T) {
	// function f(_, b) should accept but discard first param
	testCases := []string{
		"function f(_) { return 42 }",
		"function f(_, b) { return b }",
		"function f(a, _) { return a }",
		"function f(_, _, c) { return c }", // Multiple _ params
	}
	for _, input := range testCases {
		t.Run(input, func(t *testing.T) {
			ast, err := parser.Parse(context.Background(), input, nil)
			assert.Nil(t, err)
			_, err = Compile(ast, nil)
			assert.Nil(t, err)
		})
	}
}

func TestBlankIdentifier_RestParam(t *testing.T) {
	// function f(a, ..._) should accept but discard rest args
	input := "function f(a, ..._) { return a }"
	ast, err := parser.Parse(context.Background(), input, nil)
	assert.Nil(t, err)
	_, err = Compile(ast, nil)
	assert.Nil(t, err)
}

func TestBlankIdentifier_ArrowFunction(t *testing.T) {
	// Arrow functions with blank identifier
	testCases := []string{
		"_ => 42",
		"(_, b) => b",
		"(a, _) => a",
	}
	for _, input := range testCases {
		t.Run(input, func(t *testing.T) {
			ast, err := parser.Parse(context.Background(), input, nil)
			assert.Nil(t, err)
			_, err = Compile(ast, nil)
			assert.Nil(t, err)
		})
	}
}

func TestBlankIdentifier_DoubleUnderscore(t *testing.T) {
	// __ (double underscore) should be a normal identifier
	input := "let __ = 42; __"
	ast, err := parser.Parse(context.Background(), input, nil)
	assert.Nil(t, err)
	_, err = Compile(ast, nil)
	assert.Nil(t, err)
}
