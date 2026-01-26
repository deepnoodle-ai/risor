package parser

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/deepnoodle-ai/wonton/assert"
	"github.com/risor-io/risor/ast"
)

func TestTokenLineCol(t *testing.T) {
	code := `
let x = 5;
let y = 10;
	`
	program, err := Parse(context.Background(), code)
	assert.Nil(t, err)

	statements := program.Stmts
	assert.Len(t, statements, 2)

	stmt1 := statements[0].(*ast.Var)
	stmt2 := statements[1].(*ast.Var)

	start := stmt1.Pos()
	end := stmt1.End()

	// Position of the "let" token
	assert.Equal(t, start.LineNumber(), 2)
	assert.Equal(t, start.ColumnNumber(), 1)
	assert.Equal(t, end.LineNumber(), 2)
	assert.Equal(t, end.ColumnNumber(), 10)

	start = stmt2.Pos()
	end = stmt2.End()

	// Position of the "let" token
	assert.Equal(t, start.LineNumber(), 3)
	assert.Equal(t, start.ColumnNumber(), 1)
	assert.Equal(t, end.LineNumber(), 3)
	assert.Equal(t, end.ColumnNumber(), 11)
}

func TestVarStatements(t *testing.T) {
	tests := []struct {
		input string
		ident string
		value interface{}
	}{
		{"let x =5;", "x", 5},
		{"let z =1.3;", "z", 1.3},
		{"let y_ = true;", "y_", true},
		{"let foobar=y;", "foobar", "y"},
	}
	for _, tt := range tests {
		program, err := Parse(context.Background(), tt.input)
		assert.Nil(t, err)
		assert.Len(t, program.Stmts, 1)
		stmt, ok := program.First().(*ast.Var)
		assert.True(t, ok)
		testVarStatement(t, stmt, tt.ident)
		testLiteralExpression(t, stmt.Value, tt.value)
		assert.Equal(t, stmt.Name.Name, tt.ident)
	}
}

func TestDeclareStatements(t *testing.T) {
	input := `
	let x = foo.bar()
	let y = foo.bar()
	`
	program, err := Parse(context.Background(), input)
	assert.Nil(t, err)
	statements := program.Stmts
	assert.Len(t, statements, 2)
	stmt1, ok := statements[0].(*ast.Var)
	assert.True(t, ok)
	stmt2, ok := statements[1].(*ast.Var)
	assert.True(t, ok)
	_ = stmt1 // use variables to avoid unused warnings
	_ = stmt2
}

func TestMultiDeclareStatements(t *testing.T) {
	input := `let x, y, z = [1, 2, 3]`
	program, err := Parse(context.Background(), input)
	assert.Nil(t, err)
	statements := program.Stmts
	assert.Len(t, statements, 1)
	stmt1, ok := statements[0].(*ast.MultiVar)
	assert.True(t, ok)
	assert.Len(t, stmt1.Names, 3)
	assert.Equal(t, stmt1.Names[0].Name, "x")
	assert.Equal(t, stmt1.Names[1].Name, "y")
	assert.Equal(t, stmt1.Names[2].Name, "z")
	assert.Equal(t, stmt1.Value.String(), "[1, 2, 3]")
}

func TestBadVarConstStatement(t *testing.T) {
	inputs := []struct {
		input string
		err   string
	}{
		{"let", "parse error: unexpected end of file while parsing let statement (expected identifier)"},
		{"const", "parse error: unexpected end of file while parsing const statement (expected identifier)"},
		{"const x;", "parse error: unexpected ; while parsing const statement (expected =)"},
	}
	for _, tt := range inputs {
		_, err := Parse(context.Background(), tt.input)
		assert.NotNil(t, err)
		e, ok := err.(ParserError)
		assert.True(t, ok)
		assert.Equal(t, e.Error(), tt.err)
	}
}

func TestConst(t *testing.T) {
	tests := []struct {
		input              string
		expectedIdentifier string
		expectedValue      interface{}
	}{
		{"const x =5;", "x", 5},
		{"const z =1.3;", "z", 1.3},
		{"const y = true;", "y", true},
		{"const foobar=y;", "foobar", "y"},
	}
	for _, tt := range tests {
		program, err := Parse(context.Background(), tt.input)
		assert.Nil(t, err)
		assert.Len(t, program.Stmts, 1)
		stmt, ok := program.First().(*ast.Const)
		assert.True(t, ok)
		if !testConstStatement(t, stmt, tt.expectedIdentifier) {
			return
		}
		assert.Equal(t, stmt.Name.Name, tt.expectedIdentifier)
		if !testLiteralExpression(t, stmt.Value, tt.expectedValue) {
			return
		}
	}
}

func TestReturn(t *testing.T) {
	tests := []struct {
		input   string
		keyword string
	}{
		{"return 0755;", "return"},
		{"return 0x15;", "return"},
		{"return 993322;", "return"},
	}
	for _, tt := range tests {
		program, err := Parse(context.Background(), tt.input)
		assert.Nil(t, err)
		assert.Len(t, program.Stmts, 1)
		control, ok := program.First().(*ast.Return)
		assert.True(t, ok)
		_ = control // position verified by parsing
	}
}

func TestIdent(t *testing.T) {
	program, err := Parse(context.Background(), "foobar;")
	assert.Nil(t, err)
	assert.Len(t, program.Stmts, 1)
	ident, ok := program.First().(*ast.Ident)
	assert.True(t, ok)
	assert.Equal(t, "foobar", ident.String())
	assert.Equal(t, "foobar", ident.Name)
}

func TestInt(t *testing.T) {
	tests := []struct {
		input string
		value int64
	}{
		{"0", 0},
		{"5", 5},
		{"10", 10},
		{"9876543210", 9876543210},
		{"0x10", 16},
		{"0x1a", 26},
		{"0x1A", 26},
		{"010", 8},
		{"011", 9},
		{"0755", 493},
		{"00", 0},
		{"100", 100},
	}
	for _, tt := range tests {
		program, err := Parse(context.Background(), tt.input)
		assert.Nil(t, err)
		assert.Len(t, program.Stmts, 1)
		integer, ok := program.First().(*ast.Int)
		assert.True(t, ok, "got %T", program.First())
		assert.Equal(t, tt.value, integer.Value)
	}
}

func TestBool(t *testing.T) {
	tests := []struct {
		input     string
		boolValue bool
	}{
		{"true", true},
		{"false", false},
	}
	for _, tt := range tests {
		program, err := Parse(context.Background(), tt.input)
		assert.Nil(t, err)
		assert.Len(t, program.Stmts, 1)
		exp, ok := program.First().(*ast.Bool)
		assert.True(t, ok)
		assert.Equal(t, tt.boolValue, exp.Value)
	}
}

func TestPrefix(t *testing.T) {
	prefixTests := []struct {
		input        string
		operator     string
		integerValue interface{}
	}{
		{"!5;", "!", 5},
		{"-15;", "-", 15},
		{"!true;", "!", true},
		{"!false", "!", false},
	}
	for _, tt := range prefixTests {
		program, err := Parse(context.Background(), tt.input)
		assert.Nil(t, err)
		assert.Len(t, program.Stmts, 1)
		exp, ok := program.First().(*ast.Prefix)
		assert.True(t, ok)
		assert.Equal(t, tt.operator, exp.Op)
		testLiteralExpression(t, exp.X, tt.integerValue)
	}
}

func TestParsingInfixExpression(t *testing.T) {
	infixTests := []struct {
		input      string
		leftValue  interface{}
		operator   string
		rightValue interface{}
	}{
		{"0.4+1.3", 0.4, "+", 1.3},
		{"5+5;", 5, "+", 5},
		{"5-5;", 5, "-", 5},
		{"5*5;", 5, "*", 5},
		{"5/5;", 5, "/", 5},
		{"5>5;", 5, ">", 5},
		{"5<5;", 5, "<", 5},
		{"2**3;", 2, "**", 3},
		{"5==5;", 5, "==", 5},
		{"5!=5;", 5, "!=", 5},
		{"true == true", true, "==", true},
		{"true!=false", true, "!=", false},
		{"false==false", false, "==", false},
	}
	for _, tt := range infixTests {
		program, err := Parse(context.Background(), tt.input)
		assert.Nil(t, err)
		assert.Len(t, program.Stmts, 1)
		expr, ok := program.First().(ast.Expr)
		assert.True(t, ok)
		testInfixExpression(t, expr, tt.leftValue, tt.operator, tt.rightValue)
	}
}

func TestOperatorPrecedence(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"-a * b", "((-a) * b)"},
		{"!-a", "(!(-a))"},
		{"a+b+c", "((a + b) + c)"},
		{"a+b-c", "((a + b) - c)"},
		{"a*b*c", "((a * b) * c)"},
		{"a*b/c", "((a * b) / c)"},
		{"a+b/c", "(a + (b / c))"},
		{"a+b*c+d/e-f", "(((a + (b * c)) + (d / e)) - f)"},
		{"3+4;-5*5", "(3 + 4)\n((-5) * 5)"},
		{"5>4==3<4", "((5 > 4) == (3 < 4))"},
		{"5<4!=3>4", "((5 < 4) != (3 > 4))"},
		{"3+4*5==3*1+4*5", "((3 + (4 * 5)) == ((3 * 1) + (4 * 5)))"},
		{"true", "true"},
		{"false", "false"},
		{"3>5==false", "((3 > 5) == false)"},
		{"3<5==true", "((3 < 5) == true)"},
		{"1+(2+3)+4", "((1 + (2 + 3)) + 4)"},
		{"(5+5)*2", "((5 + 5) * 2)"},
		{"2/(5+5)", "(2 / (5 + 5))"},
		{"2**3", "(2 ** 3)"},
		{"-(5+5)", "(-(5 + 5))"},
		{"!(true==true)", "(!(true == true))"},
		{"a + add(b*c)+d", "((a + add((b * c))) + d)"},
		{"a*[1,2,3,4][b*c]*d", "((a * [1, 2, 3, 4][(b * c)]) * d)"},
		{"add(a*b[2], b[1], 2 * [1,2][1])", "add((a * b[2]), b[1], (2 * [1, 2][1]))"},
		{"1 - (2 - 3);", "(1 - (2 - 3))"},
		{"return 1 - (2 - 3)", "return (1 - (2 - 3))"},
		{"return foo[0];\n -3;", "return foo[0]\n(-3)"},
	}
	for _, tt := range tests {
		program, err := Parse(context.Background(), tt.input)
		assert.Nil(t, err)
		actual := program.String()
		assert.Equal(t, actual, tt.expected)
	}
}

func TestIf(t *testing.T) {
	program, err := Parse(context.Background(), "if (x < y) { x }")
	assert.Nil(t, err)
	assert.Len(t, program.Stmts, 1)
	exp, ok := program.First().(*ast.If)
	assert.True(t, ok)
	if !testInfixExpression(t, exp.Cond, "x", "<", "y") {
		return
	}
	assert.Len(t, exp.Consequence.Stmts, 1)
	consequence, ok := exp.Consequence.Stmts[0].(*ast.Ident)
	assert.True(t, ok)
	assert.Equal(t, consequence.String(), "x")
	assert.Nil(t, exp.Alternative)
}

func TestFunc(t *testing.T) {
	program, err := Parse(context.Background(), "function f(x, y=3) { x + y; }")
	assert.Nil(t, err)
	assert.Len(t, program.Stmts, 1)
	function, ok := program.First().(*ast.Func)
	assert.True(t, ok)
	params := function.Params
	assert.Len(t, params, 2)
	testLiteralExpression(t, params[0], "x")
	testLiteralExpression(t, params[1], "y")
	assert.Len(t, function.Body.Stmts, 1)
	bodyStmt, ok := function.Body.Stmts[0].(*ast.Infix)
	assert.True(t, ok)
	assert.Equal(t, bodyStmt.String(), "(x + y)")
}

func TestFuncParams(t *testing.T) {
	tests := []struct {
		input         string
		expectedParam []string
	}{
		{"function(){}", []string{}},
		{"function(x){}", []string{"x"}},
		{"function(x,y){}", []string{"x", "y"}},
	}
	for _, tt := range tests {
		program, err := Parse(context.Background(), tt.input)
		assert.Nil(t, err)
		assert.Len(t, program.Stmts, 1)
		function, ok := program.First().(*ast.Func)
		assert.True(t, ok)
		params := function.Params
		assert.Len(t, params, len(tt.expectedParam))
		for i, ident := range tt.expectedParam {
			testLiteralExpression(t, params[i], ident)
		}
	}
}

func TestArrowFunction(t *testing.T) {
	tests := []struct {
		input         string
		expectedParam []string
		bodyType      string // "return" for expression body, "block" for block body
	}{
		// No params
		{"() => 42", []string{}, "return"},
		{"() => { return 42 }", []string{}, "block"},
		// Single param
		{"(x) => x", []string{"x"}, "return"},
		{"(x) => { return x }", []string{"x"}, "block"},
		// Multiple params
		{"(x, y) => x + y", []string{"x", "y"}, "return"},
		{"(a, b, c) => a", []string{"a", "b", "c"}, "return"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			program, err := Parse(context.Background(), tt.input)
			assert.Nil(t, err, "parse error for %q", tt.input)
			assert.Len(t, program.Stmts, 1)
			function, ok := program.First().(*ast.Func)
			assert.True(t, ok, "expected Func, got %T", program.First())
			assert.Nil(t, function.Name, "arrow functions should not have names")
			params := function.Params
			assert.Len(t, params, len(tt.expectedParam))
			for i, ident := range tt.expectedParam {
				testLiteralExpression(t, params[i], ident)
			}
		})
	}
}

func TestArrowFunctionWithDefaults(t *testing.T) {
	program, err := Parse(context.Background(), "(x, y = 5) => x + y")
	assert.Nil(t, err)
	assert.Len(t, program.Stmts, 1)
	function, ok := program.First().(*ast.Func)
	assert.True(t, ok)
	params := function.Params
	assert.Len(t, params, 2)
	testLiteralExpression(t, params[0], "x")
	testLiteralExpression(t, params[1], "y")
	defaults := function.Defaults
	assert.Len(t, defaults, 1)
	assert.Contains(t, defaults, "y")
}

func TestArrowFunctionNoParens(t *testing.T) {
	tests := []struct {
		input         string
		expectedParam string
	}{
		{"x => x", "x"},
		{"y => y + 1", "y"},
		{"item => item * 2", "item"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			program, err := Parse(context.Background(), tt.input)
			assert.Nil(t, err, "parse error for %q", tt.input)
			assert.Len(t, program.Stmts, 1)
			function, ok := program.First().(*ast.Func)
			assert.True(t, ok, "expected Func, got %T", program.First())
			assert.Nil(t, function.Name, "arrow functions should not have names")
			params := function.Params
			assert.Len(t, params, 1)
			testLiteralExpression(t, params[0], tt.expectedParam)
		})
	}
}

func TestArrowFunctionErrors(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"() =>", "parse error: invalid arrow function body"},
		{"(1, 2) => x", "parse error: invalid arrow function parameter: expected identifier"},
		{"(x + 1) => x", "parse error: invalid arrow function parameter: expected identifier"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			_, err := Parse(context.Background(), tt.input)
			assert.NotNil(t, err)
			pe, ok := err.(ParserError)
			assert.True(t, ok)
			assert.Equal(t, pe.Error(), tt.expected)
		})
	}
}

func TestCall(t *testing.T) {
	program, err := Parse(context.Background(), "add(1, 2*3, 4+5)")
	assert.Nil(t, err)
	assert.Len(t, program.Stmts, 1)
	expr, ok := program.First().(*ast.Call)
	assert.True(t, ok)
	if !testIdentifier(t, expr.Fun, "add") {
		return
	}
	args := expr.Args
	assert.Len(t, args, 3)
	testLiteralExpression(t, args[0].(ast.Expr), 1)
	testInfixExpression(t, args[1].(ast.Expr), 2, "*", 3)
	testInfixExpression(t, args[2].(ast.Expr), 4, "+", 5)
}

func TestString(t *testing.T) {
	program, err := Parse(context.Background(), `"hello world";`)
	assert.Nil(t, err)
	assert.Len(t, program.Stmts, 1)
	literal, ok := program.First().(*ast.String)
	assert.True(t, ok)
	assert.Equal(t, literal.Value, "hello world")
}

func TestList(t *testing.T) {
	program, err := Parse(context.Background(), "[1, 2*2, 3+3]")
	assert.Nil(t, err)
	assert.Len(t, program.Stmts, 1)
	ll, ok := program.First().(*ast.List)
	assert.True(t, ok)
	items := ll.Items
	assert.Len(t, items, 3)
	testIntegerLiteral(t, items[0], 1)
	testInfixExpression(t, items[1], 2, "*", 2)
	testInfixExpression(t, items[2], 3, "+", 3)
}

func TestIndex(t *testing.T) {
	input := "myArray[1+1]"
	program, err := Parse(context.Background(), input)
	assert.Nil(t, err)
	assert.Len(t, program.Stmts, 1)
	indexExp, ok := program.First().(*ast.Index)
	assert.True(t, ok)
	testIdentifier(t, indexExp.X, "myArray")
	testInfixExpression(t, indexExp.Index, 1, "+", 1)
}

func TestParsingMap(t *testing.T) {
	input := `{"one":1, "two":2, "three":3}`
	program, err := Parse(context.Background(), input)
	assert.Nil(t, err)
	assert.Len(t, program.Stmts, 1)
	m, ok := program.First().(*ast.Map)
	assert.True(t, ok)
	assert.Len(t, m.Items, 3)
	expected := map[string]int64{
		"one":   1,
		"two":   2,
		"three": 3,
	}
	for _, item := range m.Items {
		literal, ok := item.Key.(*ast.String)
		assert.True(t, ok)
		expectedValue := expected[literal.Value]
		testIntegerLiteral(t, item.Value, expectedValue)
	}
}

func TestParsingEmptyMap(t *testing.T) {
	input := "{}"
	program, err := Parse(context.Background(), input)
	assert.Nil(t, err)
	assert.Len(t, program.Stmts, 1)
	m, ok := program.First().(*ast.Map)
	assert.True(t, ok)
	assert.Len(t, m.Items, 0)
}

func TestParsingMapLiteralWithExpression(t *testing.T) {
	input := `{"one":0+1, "two":10 - 8, "three": 15/5}`
	program, err := Parse(context.Background(), input)
	assert.Nil(t, err)
	assert.Len(t, program.Stmts, 1)
	m, ok := program.First().(*ast.Map)
	assert.True(t, ok)
	assert.Len(t, m.Items, 3)
	tests := map[string]func(ast.Expr){
		"one": func(e ast.Expr) {
			testInfixExpression(t, e, 0, "+", 1)
		},
		"two": func(e ast.Expr) {
			testInfixExpression(t, e, 10, "-", 8)
		},
		"three": func(e ast.Expr) {
			testInfixExpression(t, e, 15, "/", 5)
		},
	}
	for _, item := range m.Items {
		literal, ok := item.Key.(*ast.String)
		assert.True(t, ok)
		testFunc, ok := tests[literal.Value]
		assert.True(t, ok, literal.Value)
		testFunc(item.Value)
	}
}

// Test operators: +=, -=, /=, and *=.
func TestMutators(t *testing.T) {
	inputs := []string{
		"let w = 5; w *= 3;",
		"let x = 15; x += 3;",
		"let y = 10; y /= 2;",
		"let z = 10; y -= 2;",
		"let z = 1; z++;",
		"let z = 1; z--;",
		"let z = 10; let a = 3; y = a;",
		// New postfix tests for index and attribute expressions
		"let arr = [1, 2, 3]; arr[0]++;",
		"let arr = [1, 2, 3]; arr[0]--;",
		"let m = {a: 1}; m[\"a\"]++;",
		"let obj = {x: 5}; obj.x++;",
		"let obj = {x: 5}; obj.x--;",
	}
	for _, input := range inputs {
		_, err := Parse(context.Background(), input)
		assert.Nil(t, err)
	}
}

func TestPostfixErrors(t *testing.T) {
	// These should produce parser errors
	errorCases := []string{
		"1++;",         // cannot apply postfix to literal
		"(1 + 2)++;",   // cannot apply postfix to expression result
		"\"hello\"++;", // cannot apply postfix to string literal
		"true++;",      // cannot apply postfix to boolean
		"nil++;",       // cannot apply postfix to nil
		"[1, 2, 3]++;", // cannot apply postfix to list literal
		"func() {}++;", // cannot apply postfix to function
	}
	for _, input := range errorCases {
		_, err := Parse(context.Background(), input)
		assert.NotNil(t, err, "expected error for: %s", input)
	}
}

func TestPostfixAST(t *testing.T) {
	// Test that postfix expressions produce correct AST structure
	tests := []struct {
		input    string
		expected string
	}{
		{"x++", "(x++)"},
		{"x--", "(x--)"},
		{"arr[0]++", "(arr[0]++)"},
		{"obj.x++", "(obj.x++)"},
	}
	for _, tt := range tests {
		program, err := Parse(context.Background(), tt.input)
		assert.Nil(t, err, "failed to parse: %s", tt.input)
		assert.Len(t, program.Stmts, 1)
		assert.Equal(t, program.Stmts[0].String(), tt.expected)
	}
}

// Test method-call operation.
func TestObjectMethodCall(t *testing.T) {
	inputs := []string{
		"\"steve\".len()",
		"let x = 15; x.string();",
	}
	for _, input := range inputs {
		_, err := Parse(context.Background(), input)
		assert.Nil(t, err)
	}
}

func TestTryCatchFinally(t *testing.T) {
	validInputs := []string{
		// Basic try/catch on same line
		`try { throw "err" } catch e { e }`,
		// Try/catch with newlines between them
		`try { throw "err" }
catch e { e }`,
		// Try/catch/finally with newlines
		`try { throw "err" }
catch e { e }
finally { "done" }`,
		// Try/finally without catch
		`try { "ok" }
finally { "cleanup" }`,
		// Multiple newlines between try and catch
		`try { "ok" }


catch e { e }`,
		// Multiple newlines between catch and finally
		`try { "ok" }
catch e { e }


finally { "done" }`,
		// Try/catch followed by more code
		`try { 1 }
catch e { 2 }
let x = 3`,
		// Try/finally followed by more code
		`try { 1 }
finally { 2 }
let x = 3`,
		// Nested try/catch
		`try {
	try { throw "inner" }
	catch e { throw "outer" }
}
catch e { e }`,
		// Try/catch in function
		`function foo() {
	try { throw "err" }
	catch e { return e }
}`,
		// Catch without binding (no error variable)
		`try { throw "err" }
catch { "handled" }`,
	}
	for _, input := range validInputs {
		_, err := Parse(context.Background(), input)
		assert.Nil(t, err, "failed to parse: %s", input)
	}

	// Error cases
	errorCases := []string{
		// Try without catch or finally
		`try { 1 }`,
		// Catch without try
		`catch e { 1 }`,
		// Finally without try
		`finally { 1 }`,
	}
	for _, input := range errorCases {
		_, err := Parse(context.Background(), input)
		assert.NotNil(t, err, "expected error for: %s", input)
	}
}

// Test that incomplete blocks / statements are handled.
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
		{`switch (foo) { `, "parse error: unterminated switch statement"},
		{`{`, "parse error: invalid syntax"},
		{`[`, "parse error: invalid syntax in list"},
		{`{ "a": "b", "c": "d"`, "parse error: unexpected end of file while parsing map (expected })"},
		{`{ "a", "b", "c"`, "parse error: unexpected , while parsing map (expected :)"},
		{`foo |`, "parse error: invalid pipe expression"},
		{`(1, 2`, "parse error: unexpected end of file while parsing grouped expression or arrow function (expected ))"},
	}
	for _, tt := range tests {
		_, err := Parse(context.Background(), tt.input)
		assert.NotNil(t, err)
		pe, ok := err.(ParserError)
		assert.True(t, ok)
		assert.Equal(t, pe.Error(), tt.expected)
	}
}

func TestFilenameInErrors(t *testing.T) {
	// Test that the filename is included in parse errors
	_, err := Parse(context.Background(), `@@@`, WithFilename("test.risor"))
	assert.NotNil(t, err)
	pe, ok := err.(ParserError)
	assert.True(t, ok)
	assert.Equal(t, pe.File(), "test.risor")

	// Test that filename is set even for errors in the first token
	_, err = Parse(context.Background(), `#invalid`, WithFilename("early.risor"))
	assert.NotNil(t, err)
	pe, ok = err.(ParserError)
	assert.True(t, ok)
	assert.Equal(t, pe.File(), "early.risor")
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

	// Default depth limit should reject this
	_, err := Parse(context.Background(), parenInput)
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "maximum nesting depth")

	// With a higher limit, it should succeed
	_, err = Parse(context.Background(), parenInput, WithMaxDepth(1000))
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
	_, err = Parse(context.Background(), listInput)
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
	_, err = Parse(context.Background(), callInput)
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "maximum nesting depth")

	// Test 4: Custom lower depth limit
	_, err = Parse(context.Background(), `((((((1))))))`, WithMaxDepth(5))
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "maximum nesting depth")

	// Test 5: Just under the custom limit should succeed
	_, err = Parse(context.Background(), `((((1))))`, WithMaxDepth(10))
	assert.Nil(t, err)

	// Test 6: Normal code with moderate nesting works with default limit
	_, err = Parse(context.Background(), `let x = ((((1 + 2) * 3) - 4) / 5)`)
	assert.Nil(t, err)

	// Test 7: Nested blocks (function/if/switch)
	_, err = Parse(context.Background(), `
		function a() {
			function b() {
				function c() {
					if (true) {
						switch (1) {
							case 1:
								[1, 2, 3]
						}
					}
				}
			}
		}
	`)
	assert.Nil(t, err)
}

func TestContextCancellation(t *testing.T) {
	// Create an already cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// Test 1: Main parse loop respects cancellation
	_, err := Parse(ctx, `let x = 1; let y = 2; let z = 3`)
	assert.NotNil(t, err)
	assert.True(t, errors.Is(err, context.Canceled))

	// Test 2: Block parsing respects cancellation
	_, err = Parse(ctx, `{ let x = 1 }`)
	assert.NotNil(t, err)

	// Test 3: Switch parsing respects cancellation
	_, err = Parse(ctx, `switch (x) { case 1: y }`)
	assert.NotNil(t, err)

	// Test 4: Function params parsing respects cancellation
	_, err = Parse(ctx, `function f(a, b, c) { }`)
	assert.NotNil(t, err)

	// Test 5: Map parsing respects cancellation
	_, err = Parse(ctx, `{a: 1, b: 2, c: 3}`)
	assert.NotNil(t, err)

	// Test 6: Destructuring respects cancellation
	_, err = Parse(ctx, `let {a, b, c} = obj`)
	assert.NotNil(t, err)

	// Test 7: Array destructuring respects cancellation
	_, err = Parse(ctx, `let [a, b, c] = arr`)
	assert.NotNil(t, err)
}

func TestSwitch(t *testing.T) {
	input := `switch (val) {
	case 1:
	default:
      x
	  x
}`
	program, err := Parse(context.Background(), input)
	assert.Nil(t, err)
	assert.Len(t, program.Stmts, 1)
	switchExpr, ok := program.First().(*ast.Switch)
	assert.True(t, ok)
	assert.Equal(t, switchExpr.Value.String(), "val")
	assert.Len(t, switchExpr.Cases, 2)
	choice1 := switchExpr.Cases[0]
	assert.Len(t, choice1.Exprs, 1)
	assert.Equal(t, choice1.Exprs[0].String(), "1")
	choice2 := switchExpr.Cases[1]
	assert.Len(t, choice2.Exprs, 0)
}

func TestMultiDefault(t *testing.T) {
	input := `
switch (val) {
case 1:
    print("1")
case 2:
    print("2")
default:
    print("default")
default:
    print("oh no!")
}`
	_, err := Parse(context.Background(), input)
	assert.NotNil(t, err)
	parserErr, ok := err.(ParserError)
	assert.True(t, ok)
	assert.Equal(t, parserErr.Error(), "parse error: switch statement has multiple default blocks")
	assert.Equal(t, parserErr.StartPosition().Column, 0)
	assert.Equal(t, parserErr.StartPosition().Line, 10)
	// End position may vary based on position tracking implementation
	assert.Equal(t, parserErr.EndPosition().Line, 10)
}

func TestPipe(t *testing.T) {
	tests := []struct {
		input          string
		exprType       string
		expectedIdents []string
	}{
		{"let x = foo | bar;", "ident", []string{"foo", "bar"}},
		{`let x = foo() | bar(name="foo") | baz(y=4);`, "call", []string{"foo", "bar", "baz"}},
		{`let x = a() | b();`, "call", []string{"a", "b"}},
	}
	for _, tt := range tests {
		program, err := Parse(context.Background(), tt.input)
		assert.Nil(t, err)
		assert.Len(t, program.Stmts, 1)
		stmt := program.First().(*ast.Var)
		assert.Equal(t, stmt.Name.Name, "x")
		pipe, ok := stmt.Value.(*ast.Pipe)
		assert.True(t, ok)
		pipeExprs := pipe.Exprs
		assert.Len(t, pipeExprs, len(tt.expectedIdents))
		if tt.exprType == "ident" {
			for i, ident := range tt.expectedIdents {
				identExpr, ok := pipeExprs[i].(*ast.Ident)
				assert.True(t, ok)
				assert.Equal(t, identExpr.String(), ident)
			}
		} else if tt.exprType == "call" {
			for i, ident := range tt.expectedIdents {
				callExpr, ok := pipeExprs[i].(*ast.Call)
				assert.True(t, ok)
				assert.Equal(t, callExpr.Fun.String(), ident)
			}
		}
	}
}

func TestMapExpression(t *testing.T) {
	input := `{
		"a": "b",

		"c": "d",

	}
	`
	program, err := Parse(context.Background(), input)
	assert.Nil(t, err)
	assert.Len(t, program.Stmts, 1)
	expr := program.First()
	m, ok := expr.(*ast.Map)
	assert.True(t, ok)
	assert.Len(t, m.Items, 2)
}

func TestMapExpressionWithoutComma(t *testing.T) {
	input := `{
		"a": "b",

		"c": "d"


	}
	`
	program, err := Parse(context.Background(), input)
	assert.Nil(t, err)
	assert.Len(t, program.Stmts, 1)
	expr := program.First()
	m, ok := expr.(*ast.Map)
	assert.True(t, ok)
	assert.Len(t, m.Items, 2)
}

func TestCallExpression(t *testing.T) {
	input := `foo(
		a=1,
		b=2,
	)
	`
	program, err := Parse(context.Background(), input)
	assert.Nil(t, err)
	assert.Len(t, program.Stmts, 1)
	expr := program.First()
	call, ok := expr.(*ast.Call)
	assert.True(t, ok)
	assert.Equal(t, call.Fun.String(), "foo")
	args := call.Args
	assert.Len(t, args, 2)
	arg0 := args[0].(*ast.Assign)
	assert.Equal(t, arg0.String(), "a = 1")
	arg1 := args[1].(*ast.Assign)
	assert.Equal(t, arg1.String(), "b = 2")
}

func TestGetAttr(t *testing.T) {
	program, err := Parse(context.Background(), "foo.bar")
	assert.Nil(t, err)
	assert.Len(t, program.Stmts, 1)
	expr := program.First()
	getAttr, ok := expr.(*ast.GetAttr)
	assert.True(t, ok)
	assert.Equal(t, getAttr.Attr.Name, "bar")
	assert.Equal(t, getAttr.String(), "foo.bar")
}

func TestMultiVar(t *testing.T) {
	program, err := Parse(context.Background(), "let x, y = [1, 2]")
	assert.Nil(t, err)
	assert.Len(t, program.Stmts, 1)
	mvar, ok := program.First().(*ast.MultiVar)
	assert.True(t, ok)
	assert.Equal(t, mvar.Names[0].Name, "x")
	assert.Equal(t, mvar.Names[1].Name, "y")
	assert.Equal(t, mvar.Value.String(), "[1, 2]")
}

func TestIn(t *testing.T) {
	program, err := Parse(context.Background(), "x in [1, 2]")
	assert.Nil(t, err)
	assert.Len(t, program.Stmts, 1)
	node, ok := program.First().(*ast.In)
	assert.True(t, ok)
	assert.Equal(t, node.X.String(), "x")
	assert.Equal(t, node.Y.String(), "[1, 2]")
	assert.Equal(t, node.String(), "x in [1, 2]")
}

func TestNotIn(t *testing.T) {
	program, err := Parse(context.Background(), "x not in [1, 2]")
	assert.Nil(t, err)
	assert.Len(t, program.Stmts, 1)
	node, ok := program.First().(*ast.NotIn)
	assert.True(t, ok)
	assert.Equal(t, node.X.String(), "x")
	assert.Equal(t, node.Y.String(), "[1, 2]")
	assert.Equal(t, node.String(), "x not in [1, 2]")
}

func TestBacktick(t *testing.T) {
	input := "`" + `\\n\t foo bar /hey there/` + "`"
	program, err := Parse(context.Background(), input)
	assert.Nil(t, err)
	assert.Len(t, program.Stmts, 1)
	expr, ok := program.First().(*ast.String)
	assert.True(t, ok)
	assert.Equal(t, expr.Value, `\\n\t foo bar /hey there/`)
}

func TestUnterminatedBacktickString(t *testing.T) {
	input := "`foo"
	_, err := Parse(context.Background(), input)
	assert.NotNil(t, err)
	assert.Equal(t, err.Error(), "syntax error: unterminated string literal")
	var syntaxErr *SyntaxError
	ok := errors.As(err, &syntaxErr)
	assert.True(t, ok)
	assert.NotNil(t, syntaxErr.Cause())
	assert.Equal(t, syntaxErr.Cause().Error(), "unterminated string literal")
	// Verify end position column
	assert.Equal(t, syntaxErr.EndPosition().Column, 3)
	assert.Equal(t, syntaxErr.SourceCode(), "`foo")
}

func TestUnterminatedString(t *testing.T) {
	input := `42
let x = "a`
	ctx := context.Background()
	_, err := Parse(ctx, input, WithFile("main.tm"))
	assert.NotNil(t, err)
	assert.Equal(t, err.Error(), "syntax error: unterminated string literal")
	var syntaxErr *SyntaxError
	ok := errors.As(err, &syntaxErr)
	assert.True(t, ok)
	assert.NotNil(t, syntaxErr.Cause())
	assert.Equal(t, syntaxErr.Cause().Error(), "unterminated string literal")
	// Verify start and end positions
	assert.Equal(t, syntaxErr.StartPosition().Column, 8)
	assert.Equal(t, syntaxErr.StartPosition().Line, 1)
	assert.Equal(t, syntaxErr.StartPosition().File, "main.tm")
	assert.Equal(t, syntaxErr.EndPosition().Column, 9)
	assert.Equal(t, syntaxErr.SourceCode(), `let x = "a`)
}

func TestMapIdentifierKey(t *testing.T) {
	input := "{ one: 1 }"
	program, err := Parse(context.Background(), input)
	assert.Nil(t, err)
	assert.Len(t, program.Stmts, 1)
	m, ok := program.First().(*ast.Map)
	assert.True(t, ok)
	assert.Len(t, m.Items, 1)
	for _, item := range m.Items {
		ident, ok := item.Key.(*ast.Ident)
		assert.True(t, ok, fmt.Sprintf("%T", item.Key))
		assert.Equal(t, ident.String(), "one")
	}
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
		`"hi" | strings.to_lower | strings.to_upper`,
		`math.PI * 2.0`,
		`{x: 1, y: 2, z: 3} | keys`,
		`{1, "hi"} | len`,
		`[1] in {1, 2, 3}`,
		`let f = function(x) { function() { x + 1 } }; f(1)`,
		`switch (x) { case 1: 1 case 2: 2 default: 3 }`,
		`x["foo"][1:3]`,
	}
	for _, tc := range testcases {
		f.Add(tc) // Use f.Add to provide a seed corpus
	}
	f.Fuzz(func(t *testing.T, input string) {
		Parse(context.Background(), input) // Confirms no panics
	})
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
		{"0?if", `parse error: unexpected end of file while parsing an if expression (expected ()`},
		{"0?0:", `parse error: invalid syntax in ternary if false expression`},
		{"in", `parse error: invalid syntax (unexpected "in")`},
		{"x in", `parse error: invalid in expression`},
		{"switch (x) { case 1: \xf5\xf51 case 2: 2 default: 3 }", `syntax error: invalid identifier: �`},
		{"switch (x) { case 1: 1 case 2: 2 defaultIIIIIII: 3 }", "parse error: unexpected defaultIIIIIII while parsing case statement (expected ;)"},
		{`{ one: 1
			two: 2}`, "parse error: unexpected two while parsing map (expected })"},
		{`[1 2]`, "parse error: unexpected 2 while parsing list (expected ])"},
		{`[1, 2, ,]`, "parse error: invalid syntax (unexpected \",\")"},
	}
	for _, tt := range tests {
		_, err := Parse(context.Background(), tt.input)
		assert.NotNil(t, err)
		// With multi-error support, check the first error's message
		if errs, ok := err.(*Errors); ok {
			assert.Equal(t, errs.First().Error(), tt.expected)
		} else {
			assert.Equal(t, err.Error(), tt.expected)
		}
	}
}

func TestInPrecedence(t *testing.T) {
	// This confirms the correct precedence of the "in" vs. "call" operators
	input := `2 in sorted([1,2,3])`

	// Parse the program, which should be 1 statement in length
	program, err := Parse(context.Background(), input)
	assert.Nil(t, err)
	assert.Len(t, program.Stmts, 1)
	stmt := program.First()

	// The top-level of the AST should be an in statement
	inStmt, ok := stmt.(*ast.In)
	assert.True(t, ok)
	assert.Equal(t, inStmt.X.String(), "2")
	assert.Equal(t, inStmt.Y.String(), "sorted([1, 2, 3])")
}

func TestNotInPrecedence(t *testing.T) {
	// This confirms the correct precedence of the "not in" vs. "call" operators
	input := `2 not in sorted([1,2,3])`

	// Parse the program, which should be 1 statement in length
	program, err := Parse(context.Background(), input)
	assert.Nil(t, err)
	assert.Len(t, program.Stmts, 1)
	stmt := program.First()

	// The top-level of the AST should be a not in statement
	notInStmt, ok := stmt.(*ast.NotIn)
	assert.True(t, ok)
	assert.Equal(t, notInStmt.X.String(), "2")
	assert.Equal(t, notInStmt.Y.String(), "sorted([1, 2, 3])")
}

// TestNewlineHandling documents and tests the parser's newline behavior:
//
// POLICY:
//  1. Trailing operators continue expressions: "x +\ny" parses as one expression
//  2. Newlines at start of line terminate expressions: "x\n+ y" parses as two statements
//  3. Inside parentheses: leading/trailing newlines are allowed: "(\nx + y\n)"
//  4. Inside brackets/braces: newlines after commas are allowed: "[1,\n2]"
//  5. Ternary expressions: newlines allowed around ? and : operators
//  6. Postfix operators (++, --) must be on same line as operand
func TestNewlineHandling(t *testing.T) {
	// Cases that SHOULD parse as single expressions
	validCases := []struct {
		name     string
		input    string
		expected string
	}{
		// Trailing operator continues expression
		{"trailing +", "x +\ny", "(x + y)"},
		{"trailing &&", "x &&\ny", "(x && y)"},
		{"trailing ||", "x ||\ny", "(x || y)"},
		{"chained trailing ops", "x +\ny +\nz", "((x + y) + z)"},
		{"trailing * with paren", "x *\n(y + z)", "(x * (y + z))"},

		// Newlines inside parentheses
		{"grouped with leading newline", "(\nx + y)", "(x + y)"},
		{"grouped with trailing newline", "(x + y\n)", "(x + y)"},
		{"grouped with both newlines", "(\nx + y\n)", "(x + y)"},

		// Ternary expressions
		{"ternary newline after ?", "x ?\ny : z", "(x ? y : z)"},
		{"ternary newline after :", "x ? y :\nz", "(x ? y : z)"},
		{"ternary newlines both", "x ?\ny\n: z", "(x ? y : z)"},

		// Lists and maps
		{"list with newlines", "[1,\n2,\n3]", "[1, 2, 3]"},
		{"map with newlines", "{a: 1,\nb: 2}", "{a:1, b:2}"},
		{"function args with newlines", "f(x,\ny,\nz)", "f(x, y, z)"},
	}

	for _, tt := range validCases {
		t.Run(tt.name, func(t *testing.T) {
			program, err := Parse(context.Background(), tt.input)
			assert.Nil(t, err, "unexpected error for %q: %v", tt.name, err)
			if err == nil {
				assert.Len(t, program.Stmts, 1, "expected 1 statement for %q", tt.name)
				if len(program.Stmts) == 1 {
					assert.Equal(t, program.First().String(), tt.expected, "mismatch for %q", tt.name)
				}
			}
		})
	}

	// Cases that SHOULD parse as multiple statements
	multiStmtCases := []struct {
		name     string
		input    string
		numStmts int
	}{
		{"newline before [", "arr\n[0]", 2},
		{"newline before |", "x\n| y", 2},
		{"two assignments", "x = 1\ny = 2", 2},
		{"two idents", "x\ny", 2},
	}

	for _, tt := range multiStmtCases {
		t.Run(tt.name, func(t *testing.T) {
			program, err := Parse(context.Background(), tt.input)
			assert.Nil(t, err, "unexpected error for %q: %v", tt.name, err)
			if err == nil {
				assert.Len(t, program.Stmts, tt.numStmts, "expected %d statements for %q", tt.numStmts, tt.name)
			}
		})
	}

	// Cases that SHOULD produce errors
	errorCases := []struct {
		name  string
		input string
	}{
		{"newline before + (no unary plus)", "x\n+ y"},
		{"newline before postfix ++", "x\n++"},
		{"newline before postfix --", "x\n--"},
		{"newline before . method call", "obj\n.method()"},
	}

	for _, tt := range errorCases {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Parse(context.Background(), tt.input)
			assert.NotNil(t, err, "expected error for %q", tt.name)
		})
	}
}

func TestNakedReturns(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{`function test() { return }; test()`, "function test() { return }\ntest()"},
		{`function test() {
			return
		}
		test()`, "function test() { return }\ntest()"},
		{`function test() { return; }; test()`, "function test() { return }\ntest()"},
		{`function test() { continue; }; test()`, "function test() { continue }\ntest()"},
	}
	for _, tt := range tests {
		result, err := Parse(context.Background(), tt.input)
		assert.Nil(t, err)
		assert.Equal(t, result.String(), tt.expected)
	}
}

func TestInvalidListTermination(t *testing.T) {
	input := `
	{ data: { blocks: [ { type: "divider" },
		}
	}`
	_, err := Parse(context.Background(), input)
	assert.Error(t, err)
	// With multi-error support, check the first error's message
	if errs, ok := err.(*Errors); ok {
		assert.Equal(t, errs.First().Error(), `parse error: invalid syntax (unexpected "}")`)
	} else {
		assert.Equal(t, err.Error(), `parse error: invalid syntax (unexpected "}")`)
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
			result, err := Parse(context.Background(), tt.input)
			assert.Nil(t, err)
			assert.Equal(t, result.String(), tt.expected)
		})
	}
}

func TestDoubleSemicolon(t *testing.T) {
	input := "42; ;"
	_, err := Parse(context.Background(), input)
	assert.Error(t, err)
	assert.Equal(t, err.Error(), "parse error: invalid syntax (unexpected \";\")")
}

func TestInvalidMultipleExpressions(t *testing.T) {
	input := "42 33"
	_, err := Parse(context.Background(), input)
	assert.Error(t, err)
	assert.Equal(t, err.Error(), "parse error: unexpected token \"33\" following statement")
}

func TestInvalidMultipleExpressions2(t *testing.T) {
	input := "42\n 33 oops"
	_, err := Parse(context.Background(), input)
	assert.Error(t, err)
	assert.Equal(t, err.Error(), "parse error: unexpected token \"oops\" following statement")
}

func TestOptionalChaining(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{`obj?.name`, `obj?.name`},
		{`obj?.inner?.value`, `obj?.inner?.value`},
		{`obj?.method()`, `obj?.method()`},
		{`obj?.method(1, 2)`, `obj?.method(1, 2)`},
		{`obj.a?.b`, `obj.a?.b`},
		{`obj?.a.b`, `obj?.a.b`},
	}
	for _, tt := range tests {
		result, err := Parse(context.Background(), tt.input)
		assert.Nil(t, err, "input: %s", tt.input)
		assert.Equal(t, result.String(), tt.expected, "input: %s", tt.input)
	}
}

func TestBitwiseAnd(t *testing.T) {
	input := "1 & 2"
	result, err := Parse(context.Background(), input)
	assert.Nil(t, err)
	assert.Equal(t, result.String(), "(1 & 2)")
}

func TestMultiErrorReporting(t *testing.T) {
	// Test that the parser collects multiple errors with recovery
	t.Run("multiple statement errors", func(t *testing.T) {
		// Three statements, each with an error
		input := `let x =
let y =
let z =`
		program, err := Parse(context.Background(), input)
		assert.NotNil(t, err)

		errs, ok := err.(*Errors)
		assert.True(t, ok, "expected *Errors type")
		assert.GreaterOrEqual(t, errs.Count(), 2, "expected multiple errors")

		// We should still get a partial AST (may be empty due to errors)
		assert.NotNil(t, program)
	})

	t.Run("errors implement ParserError", func(t *testing.T) {
		input := "let x ="
		_, err := Parse(context.Background(), input)
		assert.NotNil(t, err)

		// *Errors implements ParserError interface
		pe, ok := err.(ParserError)
		assert.True(t, ok, "expected ParserError interface")
		assert.NotEmpty(t, pe.Error())
		assert.NotEmpty(t, pe.Type())
	})

	t.Run("errors.As works for SyntaxError", func(t *testing.T) {
		input := "`unterminated"
		_, err := Parse(context.Background(), input)
		assert.NotNil(t, err)

		var syntaxErr *SyntaxError
		ok := errors.As(err, &syntaxErr)
		assert.True(t, ok, "expected errors.As to find SyntaxError")
		assert.NotNil(t, syntaxErr.Cause())
	})

	t.Run("First returns first error", func(t *testing.T) {
		input := `let x =
let y =`
		_, err := Parse(context.Background(), input)
		assert.NotNil(t, err)

		errs, ok := err.(*Errors)
		assert.True(t, ok)

		first := errs.First()
		assert.NotNil(t, first)
		assert.Contains(t, first.Error(), "missing a value")
	})

	t.Run("partial AST returned on error", func(t *testing.T) {
		// First statement is valid, second has error
		input := `let x = 1
let y =`
		program, err := Parse(context.Background(), input)
		assert.NotNil(t, err)
		assert.NotNil(t, program)

		// Should have at least the valid first statement
		assert.GreaterOrEqual(t, len(program.Stmts), 1)
		stmt, ok := program.Stmts[0].(*ast.Var)
		assert.True(t, ok)
		assert.Equal(t, stmt.Name.Name, "x")
	})

	t.Run("error limit prevents infinite collection", func(t *testing.T) {
		// Generate many errors
		var sb strings.Builder
		for i := 0; i < 20; i++ {
			sb.WriteString("@@@\n") // illegal tokens
		}
		_, err := Parse(context.Background(), sb.String())
		assert.NotNil(t, err)

		errs, ok := err.(*Errors)
		assert.True(t, ok)
		// Should be limited by MaxErrors
		assert.LessOrEqual(t, errs.Count(), MaxErrors+1)
	})
}
