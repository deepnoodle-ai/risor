package parser

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/deepnoodle-ai/risor/v2/pkg/ast"
	"github.com/deepnoodle-ai/wonton/assert"
)

// Core parser tests (parser.go)
// - Token position tracking
// - Context cancellation
// - Max depth limits
// - Multi-error reporting
// - Newline handling policy
// - Fuzz testing
// - Bad input handling

func TestTokenLineCol(t *testing.T) {
	code := `
let x = 5;
let y = 10;
	`
	program, err := Parse(context.Background(), code, nil)
	assert.Nil(t, err)

	statements := program.Stmts
	assert.Len(t, statements, 2)

	stmt1 := statements[0].(*ast.Var)
	stmt2 := statements[1].(*ast.Var)

	start := stmt1.Pos()
	end := stmt1.End()

	assert.Equal(t, 2, start.LineNumber())
	assert.Equal(t, 1, start.ColumnNumber())
	assert.Equal(t, 2, end.LineNumber())
	assert.Equal(t, 10, end.ColumnNumber())

	start = stmt2.Pos()
	end = stmt2.End()

	assert.Equal(t, 3, start.LineNumber())
	assert.Equal(t, 1, start.ColumnNumber())
	assert.Equal(t, 3, end.LineNumber())
	assert.Equal(t, 11, end.ColumnNumber())
}

func TestFilenameInErrors(t *testing.T) {
	_, err := Parse(context.Background(), `@@@`, &Config{Filename: "test.risor"})
	assert.NotNil(t, err)

	pe, ok := err.(ParserError)
	assert.True(t, ok)
	assert.Equal(t, "test.risor", pe.File())

	_, err = Parse(context.Background(), `#invalid`, &Config{Filename: "early.risor"})
	assert.NotNil(t, err)

	pe, ok = err.(ParserError)
	assert.True(t, ok)
	assert.Equal(t, "early.risor", pe.File())
}

func TestMaxDepth(t *testing.T) {
	// Test 1: Deeply nested parentheses
	var sb strings.Builder
	for i := 0; i < 600; i++ {
		sb.WriteString("(")
	}
	sb.WriteString("1")
	for i := 0; i < 600; i++ {
		sb.WriteString(")")
	}
	parenInput := sb.String()

	_, err := Parse(context.Background(), parenInput, nil)
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "maximum nesting depth")

	_, err = Parse(context.Background(), parenInput, &Config{MaxDepth: 1000})
	assert.Nil(t, err)

	// Test 2: Deeply nested lists
	sb.Reset()
	for i := 0; i < 600; i++ {
		sb.WriteString("[")
	}
	sb.WriteString("1")
	for i := 0; i < 600; i++ {
		sb.WriteString("]")
	}
	listInput := sb.String()
	_, err = Parse(context.Background(), listInput, nil)
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "maximum nesting depth")

	// Test 3: Deeply nested function calls
	sb.Reset()
	for i := 0; i < 600; i++ {
		sb.WriteString("f(")
	}
	sb.WriteString("1")
	for i := 0; i < 600; i++ {
		sb.WriteString(")")
	}
	callInput := sb.String()
	_, err = Parse(context.Background(), callInput, nil)
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "maximum nesting depth")

	// Test 4: Custom lower depth limit
	_, err = Parse(context.Background(), `((((((1))))))`, &Config{MaxDepth: 5})
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "maximum nesting depth")

	// Test 5: Just under the custom limit should succeed
	_, err = Parse(context.Background(), `((((1))))`, &Config{MaxDepth: 10})
	assert.Nil(t, err)

	// Test 6: Normal code with moderate nesting works with default limit
	_, err = Parse(context.Background(), `let x = ((((1 + 2) * 3) - 4) / 5)`, nil)
	assert.Nil(t, err)

	// Test 7: Nested blocks (function/if/switch)
	_, err = Parse(context.Background(), `
		function a() {
			function b() {
				function c() {
					if (true) {
						[1, 2, 3]
					}
				}
			}
		}
	`, nil)
	assert.Nil(t, err)
}

func TestContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// Test 1: Main parse loop respects cancellation
	_, err := Parse(ctx, `let x = 1; let y = 2; let z = 3`, nil)
	assert.NotNil(t, err)
	assert.True(t, errors.Is(err, context.Canceled))

	// Test 2: Block parsing respects cancellation
	_, err = Parse(ctx, `{ let x = 1 }`, nil)
	assert.NotNil(t, err)

	// Test 3: Switch parsing respects cancellation
	_, err = Parse(ctx, `switch (x) { case 1: y }`, nil)
	assert.NotNil(t, err)

	// Test 4: Function params parsing respects cancellation
	_, err = Parse(ctx, `function f(a, b, c) { }`, nil)
	assert.NotNil(t, err)

	// Test 5: Map parsing respects cancellation
	_, err = Parse(ctx, `{a: 1, b: 2, c: 3}`, nil)
	assert.NotNil(t, err)

	// Test 6: Destructuring respects cancellation
	_, err = Parse(ctx, `let {a, b, c} = obj`, nil)
	assert.NotNil(t, err)

	// Test 7: Array destructuring respects cancellation
	_, err = Parse(ctx, `let [a, b, c] = arr`, nil)
	assert.NotNil(t, err)
}

func TestMultiErrorReporting(t *testing.T) {
	t.Run("multiple statement errors", func(t *testing.T) {
		input := `let x =
let y =
let z =`
		program, err := Parse(context.Background(), input, nil)
		assert.NotNil(t, err)

		errs, ok := err.(*Errors)
		assert.True(t, ok, "expected *Errors type")
		assert.GreaterOrEqual(t, errs.Count(), 2, "expected multiple errors")
		assert.NotNil(t, program)
	})

	t.Run("errors implement ParserError", func(t *testing.T) {
		input := "let x ="
		_, err := Parse(context.Background(), input, nil)
		assert.NotNil(t, err)

		pe, ok := err.(ParserError)
		assert.True(t, ok, "expected ParserError interface")
		assert.NotEmpty(t, pe.Error())
		assert.NotEmpty(t, pe.Type())
	})

	t.Run("errors.As works for SyntaxError", func(t *testing.T) {
		input := "`unterminated"
		_, err := Parse(context.Background(), input, nil)
		assert.NotNil(t, err)

		var syntaxErr *SyntaxError
		ok := errors.As(err, &syntaxErr)
		assert.True(t, ok, "expected errors.As to find SyntaxError")
		assert.NotNil(t, syntaxErr.Cause())
	})

	t.Run("First returns first error", func(t *testing.T) {
		// Input has two incomplete let statements
		// After newline handling change, the parser tries to parse "let"
		// as an expression, which gives "unexpected let" error
		input := `let x =
let y =`
		_, err := Parse(context.Background(), input, nil)
		assert.NotNil(t, err)

		errs, ok := err.(*Errors)
		assert.True(t, ok)

		first := errs.First()
		assert.NotNil(t, first)
		// The error message indicates "let" appeared where an expression was expected
		assert.Contains(t, first.Error(), "let")
	})

	t.Run("partial AST returned on error", func(t *testing.T) {
		input := `let x = 1
let y =`
		program, err := Parse(context.Background(), input, nil)
		assert.NotNil(t, err)
		assert.NotNil(t, program)

		assert.GreaterOrEqual(t, len(program.Stmts), 1)
		stmt, ok := program.Stmts[0].(*ast.Var)
		assert.True(t, ok)
		assert.Equal(t, "x", stmt.Name.Name)
	})

	t.Run("error limit prevents infinite collection", func(t *testing.T) {
		var sb strings.Builder
		for i := 0; i < 20; i++ {
			sb.WriteString("@@@\n")
		}
		_, err := Parse(context.Background(), sb.String(), nil)
		assert.NotNil(t, err)

		errs, ok := err.(*Errors)
		assert.True(t, ok)
		assert.LessOrEqual(t, errs.Count(), MaxErrors+1)
	})
}

// TestNewlineHandling documents and tests the parser's newline behavior:
//
// POLICY:
//  1. Trailing operators continue expressions: "x +\ny" parses as one expression
//  2. Newlines at start of line terminate expressions: "x\n+ y" parses as two statements
//  3. Inside parentheses: leading/trailing newlines are allowed: "(\nx + y\n)"
//  4. Inside brackets/braces: newlines after commas are allowed: "[1,\n2]"
//  5. Postfix operators (++, --) must be on same line as operand
//  6. Chaining operators (., ?., |>) can follow newlines: "x\n.method()", "x\n|> f"
func TestNewlineHandling(t *testing.T) {
	validCases := []struct {
		name     string
		input    string
		expected string
	}{
		{"trailing +", "x +\ny", "(x + y)"},
		{"trailing &&", "x &&\ny", "(x && y)"},
		{"trailing ||", "x ||\ny", "(x || y)"},
		{"chained trailing ops", "x +\ny +\nz", "((x + y) + z)"},
		{"trailing * with paren", "x *\n(y + z)", "(x * (y + z))"},
		{"grouped with leading newline", "(\nx + y)", "(x + y)"},
		{"grouped with trailing newline", "(x + y\n)", "(x + y)"},
		{"grouped with both newlines", "(\nx + y\n)", "(x + y)"},
		{"list with newlines", "[1,\n2,\n3]", "[1, 2, 3]"},
		{"map with newlines", "{a: 1,\nb: 2}", "{a:1, b:2}"},
		{"function args with newlines", "f(x,\ny,\nz)", "f(x, y, z)"},
		// Method chaining across newlines (rule 7)
		{"newline before . method call", "obj\n.method()", "obj.method()"},
		{"newline before ?. optional chain", "obj\n?.method()", "obj?.method()"},
		{"multi-line method chain", "obj\n.method1()\n.method2()", "obj.method1().method2()"},
		{"multi-line optional chain", "obj\n?.prop\n?.nested", "obj?.prop?.nested"},
		{"multi-line mixed chain", "obj\n.method()\n?.prop", "obj.method()?.prop"},
		{"chain with args", "list.filter(x => x > 0)\n.map(x => x * 2)", "list.filter(function(x) { return (x > 0) }).map(function(x) { return (x * 2) })"},
		// Pipe chaining across newlines (rule 6, same as . and ?.)
		{"newline before |>", "a\n|> b", "(a |> b)"},
		{"multi-line pipe chain", "a\n|> b\n|> c", "(a |> b |> c)"},
		{"method chain then pipe", "obj\n.method()\n|> f", "(obj.method() |> f)"},
	}

	for _, tt := range validCases {
		t.Run(tt.name, func(t *testing.T) {
			program, err := Parse(context.Background(), tt.input, nil)
			assert.Nil(t, err, "unexpected error for %q: %v", tt.name, err)
			if err == nil {
				assert.Len(t, program.Stmts, 1, "expected 1 statement for %q", tt.name)
				if len(program.Stmts) == 1 {
					assert.Equal(t, tt.expected, program.First().String(), "mismatch for %q", tt.name)
				}
			}
		})
	}

	multiStmtCases := []struct {
		name     string
		input    string
		numStmts int
	}{
		{"newline before [", "arr\n[0]", 2},
		{"two assignments", "x = 1\ny = 2", 2},
		{"two idents", "x\ny", 2},
	}

	for _, tt := range multiStmtCases {
		t.Run(tt.name, func(t *testing.T) {
			program, err := Parse(context.Background(), tt.input, nil)
			assert.Nil(t, err, "unexpected error for %q: %v", tt.name, err)
			if err == nil {
				assert.Len(t, program.Stmts, tt.numStmts, "expected %d statements for %q", tt.numStmts, tt.name)
			}
		})
	}

	errorCases := []struct {
		name  string
		input string
	}{
		{"newline before + (no unary plus)", "x\n+ y"},
		{"newline before postfix ++", "x\n++"},
		{"newline before postfix --", "x\n--"},
	}

	for _, tt := range errorCases {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Parse(context.Background(), tt.input, nil)
			assert.NotNil(t, err, "expected error for %q", tt.name)
		})
	}
}

func TestMultilineInfixExprs(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"1 +\n2", "(1 + 2)"},
		{"1 +\n2 /\n3", "(1 + (2 / 3))"},
		{"false || \n\n\ntrue", "(false || true)"},
		{"true &&\n \nfalse", "(true && false)"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result, err := Parse(context.Background(), tt.input, nil)
			assert.Nil(t, err)
			assert.Equal(t, tt.expected, result.String())
		})
	}
}

func TestBadInputs(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"if", `parse error: unexpected end of file while parsing an if expression (expected ()`},
		{"else", `parse error: invalid syntax (unexpected "else")`},
		{"&&", `parse error: invalid syntax (unexpected "&&")`},
		{"[", `parse error: invalid syntax in list`},
		{"[1,", `parse error: invalid syntax`},
		{"0?if", `parse error: unexpected token "?" following statement`},
		{"0?0:", `parse error: unexpected token "?" following statement`},
		{"in", `parse error: invalid syntax (unexpected "in")`},
		{"x in", `parse error: invalid in expression`},
		{`{ one: 1
			two: 2}`, "parse error: unexpected two while parsing map (expected })"},
		{`[1 2]`, "parse error: unexpected 2 while parsing list (expected ])"},
		{`[1, 2, ,]`, "parse error: invalid syntax (unexpected \",\")"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			_, err := Parse(context.Background(), tt.input, nil)
			assert.NotNil(t, err)
			if errs, ok := err.(*Errors); ok {
				assert.Equal(t, tt.expected, errs.First().Error())
			} else {
				assert.Equal(t, tt.expected, err.Error())
			}
		})
	}
}

func TestIncompleThings(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{`if ( true ) { `, "parse error: unterminated block statement"},
		{`if ( true ) { puts( "OK" ) ; } else { `, "parse error: unterminated block statement"},
		{`let x = `, "parse error: assignment is missing a value"},
		{`const x =`, "parse error: assignment is missing a value"},
		{`function foo( a, b ="steve", `, "parse error: unterminated function parameters"},
		{`function foo() {`, "parse error: unterminated block statement"},
		{`{`, "parse error: invalid syntax"},
		{`[`, "parse error: invalid syntax in list"},
		{`{ "a": "b", "c": "d"`, "parse error: unexpected end of file while parsing map (expected })"},
		{`{ "a", "b", "c"`, "parse error: unexpected , while parsing map (expected :)"},
		{`foo |>`, "parse error: invalid pipe expression"},
		{`(1, 2`, "parse error: unexpected end of file while parsing grouped expression or arrow function (expected ))"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			_, err := Parse(context.Background(), tt.input, nil)
			assert.NotNil(t, err)
			pe, ok := err.(ParserError)
			assert.True(t, ok)
			assert.Equal(t, tt.expected, pe.Error())
		})
	}
}

func TestDoubleSemicolon(t *testing.T) {
	input := "42; ;"
	_, err := Parse(context.Background(), input, nil)
	assert.Error(t, err)
	assert.Equal(t, "parse error: invalid syntax (unexpected \";\")", err.Error())
}

func TestInvalidMultipleExpressions(t *testing.T) {
	input := "42 33"
	_, err := Parse(context.Background(), input, nil)
	assert.Error(t, err)
	assert.Equal(t, "parse error: unexpected token \"33\" following statement", err.Error())
}

func TestInvalidMultipleExpressions2(t *testing.T) {
	input := "42\n 33 oops"
	_, err := Parse(context.Background(), input, nil)
	assert.Error(t, err)
	assert.Equal(t, "parse error: unexpected token \"oops\" following statement", err.Error())
}

func TestInvalidListTermination(t *testing.T) {
	input := `
	{ data: { blocks: [ { type: "divider" },
		}
	}`
	_, err := Parse(context.Background(), input, nil)
	assert.Error(t, err)
	if errs, ok := err.(*Errors); ok {
		assert.Equal(t, `parse error: invalid syntax (unexpected "}")`, errs.First().Error())
	} else {
		assert.Equal(t, `parse error: invalid syntax (unexpected "}")`, err.Error())
	}
}

func TestUnterminatedBacktickString(t *testing.T) {
	input := "`foo"
	_, err := Parse(context.Background(), input, nil)
	assert.NotNil(t, err)
	assert.Equal(t, "syntax error: unterminated string literal", err.Error())

	var syntaxErr *SyntaxError
	ok := errors.As(err, &syntaxErr)
	assert.True(t, ok)
	assert.NotNil(t, syntaxErr.Cause())
	assert.Equal(t, "unterminated string literal", syntaxErr.Cause().Error())
	assert.Equal(t, 3, syntaxErr.EndPosition().Column)
	assert.Equal(t, "`foo", syntaxErr.SourceCode())
}

func TestUnterminatedString(t *testing.T) {
	input := `42
let x = "a`
	ctx := context.Background()
	_, err := Parse(ctx, input, &Config{Filename: "main.tm"})
	assert.NotNil(t, err)
	assert.Equal(t, "syntax error: unterminated string literal", err.Error())

	var syntaxErr *SyntaxError
	ok := errors.As(err, &syntaxErr)
	assert.True(t, ok)
	assert.NotNil(t, syntaxErr.Cause())
	assert.Equal(t, "unterminated string literal", syntaxErr.Cause().Error())
	assert.Equal(t, 8, syntaxErr.StartPosition().Column)
	assert.Equal(t, 1, syntaxErr.StartPosition().Line)
	assert.Equal(t, "main.tm", syntaxErr.StartPosition().File)
	assert.Equal(t, 9, syntaxErr.EndPosition().Column)
	assert.Equal(t, `let x = "a`, syntaxErr.SourceCode())
}

func TestProgramAST(t *testing.T) {
	program, err := Parse(context.Background(), "1; 2; 3", nil)
	assert.Nil(t, err)

	// Verify Program AST
	assert.Len(t, program.Stmts, 3)
	assert.NotNil(t, program.First())
	assert.Equal(t, "1\n2\n3", program.String())
}

func FuzzParse(f *testing.F) {
	testcases := []string{
		"1/2+4+=5-[1,2,{}]",
		" ",
		"!12345",
		"let x = [1,2,3];",
		`; const z = {"foo"}`,
		`"foo_" + 1.34 /= 2.0`,
		`{hey: {there: 1}}`,
		`'foo bar'`,
		`x.func(x=1, y=2).bar`,
		`0A=`,
		`"hi" |> strings.to_lower |> strings.to_upper`,
		`math.PI * 2.0`,
		`{x: 1, y: 2, z: 3} |> keys`,
		`{1, "hi"} |> len`,
		`[1] in {1, 2, 3}`,
		`let f = function(x) { function() { x + 1 } }; f(1)`,
		`switch (x) { case 1: 1 case 2: 2 default: 3 }`,
		`x["foo"][1:3]`,
	}
	for _, tc := range testcases {
		f.Add(tc)
	}
	f.Fuzz(func(t *testing.T, input string) {
		Parse(context.Background(), input, nil) // Confirms no panics
	})
}

// Ensure error interfaces work correctly
func TestErrorInterface(t *testing.T) {
	_, err := Parse(context.Background(), "@@@", nil)
	assert.NotNil(t, err)

	// Test error string
	errStr := err.Error()
	assert.NotEmpty(t, errStr)

	// Test ParserError interface
	pe, ok := err.(ParserError)
	assert.True(t, ok)
	assert.NotEmpty(t, pe.Type())
	assert.NotEmpty(t, pe.SourceCode())

	// Test FriendlyErrorMessage
	friendly := pe.FriendlyErrorMessage()
	assert.NotEmpty(t, friendly)
	assert.Contains(t, friendly, "-->")
}

// Test that positions are correctly tracked
func TestPositionTracking(t *testing.T) {
	code := `let x = 1
let y = 2`
	program, err := Parse(context.Background(), code, nil)
	assert.Nil(t, err)
	assert.Len(t, program.Stmts, 2)

	stmt1 := program.Stmts[0]
	stmt2 := program.Stmts[1]

	// First statement starts at line 1
	assert.Equal(t, 0, stmt1.Pos().Line)
	assert.Equal(t, 0, stmt1.Pos().Column)

	// Second statement starts at line 2
	assert.Equal(t, 1, stmt2.Pos().Line)
	assert.Equal(t, 0, stmt2.Pos().Column)

	// End positions should be after the value
	assert.Greater(t, stmt1.End().Column, stmt1.Pos().Column)
	assert.Greater(t, stmt2.End().Column, stmt2.Pos().Column)
}

// Test Config.Filename option
func TestConfigFilenameOption(t *testing.T) {
	_, err := Parse(context.Background(), "@@@", &Config{Filename: "test.risor"})
	assert.NotNil(t, err)

	pe, ok := err.(ParserError)
	assert.True(t, ok)
	assert.Equal(t, "test.risor", pe.File())
}

// Test that identifiers preserve their names correctly
func TestIdentPreservesName(t *testing.T) {
	names := []string{"x", "foo", "_bar", "camelCase", "snake_case", "CAPS"}

	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			program, err := Parse(context.Background(), name, nil)
			assert.Nil(t, err)

			ident, ok := program.First().(*ast.Ident)
			assert.True(t, ok)
			assert.Equal(t, name, ident.Name)
		})
	}
}
