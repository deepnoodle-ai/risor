package parser

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/risor-io/risor/ast"
	"github.com/risor-io/risor/token"
	"github.com/stretchr/testify/require"
)

func TestTokenLineCol(t *testing.T) {
	code := `
let x = 5;
let y = 10;
	`
	program, err := Parse(context.Background(), code)
	require.Nil(t, err)

	statements := program.Statements()
	require.Len(t, statements, 2)

	stmt1 := statements[0].(*ast.Var)
	stmt2 := statements[1].(*ast.Var)

	t1 := stmt1.Token()
	start := t1.StartPosition
	end := t1.EndPosition

	// Position of the "let" token
	require.Equal(t, 2, start.LineNumber())
	require.Equal(t, 1, start.ColumnNumber())
	require.Equal(t, 2, end.LineNumber())
	require.Equal(t, 3, end.ColumnNumber())

	t2 := stmt2.Token()
	start = t2.StartPosition
	end = t2.EndPosition

	// Position of the "let" token
	require.Equal(t, 3, start.LineNumber())
	require.Equal(t, 1, start.ColumnNumber())
	require.Equal(t, 3, end.LineNumber())
	require.Equal(t, 3, end.ColumnNumber())
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
		fmt.Println(err)
		require.Nil(t, err)
		require.Len(t, program.Statements(), 1)
		stmt, ok := program.First().(*ast.Var)
		require.True(t, ok)
		testVarStatement(t, stmt, tt.ident)
		name, val := stmt.Value()
		testLiteralExpression(t, val, tt.value)
		require.Equal(t, tt.ident, name)
	}
}

func TestDeclareStatements(t *testing.T) {
	input := `
	let x = foo.bar()
	let y = foo.bar()
	`
	program, err := Parse(context.Background(), input)
	require.Nil(t, err)
	statements := program.Statements()
	require.Len(t, statements, 2)
	stmt1, ok := statements[0].(*ast.Var)
	require.True(t, ok)
	stmt2, ok := statements[1].(*ast.Var)
	require.True(t, ok)
	fmt.Println(stmt1)
	fmt.Println(stmt2)
}

func TestMultiDeclareStatements(t *testing.T) {
	input := `let x, y, z = [1, 2, 3]`
	program, err := Parse(context.Background(), input)
	require.Nil(t, err)
	statements := program.Statements()
	require.Len(t, statements, 1)
	stmt1, ok := statements[0].(*ast.MultiVar)
	require.True(t, ok)
	names, expr := stmt1.Value()
	require.Len(t, names, 3)
	require.Equal(t, "x", names[0])
	require.Equal(t, "y", names[1])
	require.Equal(t, "z", names[2])
	require.Equal(t, "[1, 2, 3]", expr.String())
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
		require.NotNil(t, err)
		e, ok := err.(ParserError)
		require.True(t, ok)
		require.Equal(t, tt.err, e.Error())
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
		require.Nil(t, err)
		require.Len(t, program.Statements(), 1)
		stmt, ok := program.First().(*ast.Const)
		require.True(t, ok)
		if !testConstStatement(t, stmt, tt.expectedIdentifier) {
			return
		}
		name, val := stmt.Value()
		require.Equal(t, tt.expectedIdentifier, name)
		if !testLiteralExpression(t, val, tt.expectedValue) {
			return
		}
	}
}

func TestControl(t *testing.T) {
	tests := []struct {
		input   string
		keyword string
	}{
		{"continue;", "continue"},
		{"break;", "break"},
	}
	for _, tt := range tests {
		program, err := Parse(context.Background(), tt.input)
		require.Nil(t, err)
		require.Len(t, program.Statements(), 1)
		control, ok := program.First().(*ast.Control)
		require.True(t, ok)
		require.Equal(t, tt.keyword, control.Literal())
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
		require.Nil(t, err)
		require.Len(t, program.Statements(), 1)
		control, ok := program.First().(*ast.Return)
		require.True(t, ok)
		require.Equal(t, tt.keyword, control.Literal())
	}
}

func TestIdent(t *testing.T) {
	program, err := Parse(context.Background(), "foobar;")
	require.Nil(t, err)
	require.Len(t, program.Statements(), 1)
	ident, ok := program.First().(*ast.Ident)
	require.True(t, ok)
	require.Equal(t, ident.String(), "foobar")
	require.Equal(t, ident.Literal(), "foobar")
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
		require.Nil(t, err)
		require.Len(t, program.Statements(), 1)
		integer, ok := program.First().(*ast.Int)
		require.True(t, ok, "got %T", program.First())
		require.Equal(t, integer.Value(), tt.value)
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
		require.Nil(t, err)
		require.Len(t, program.Statements(), 1)
		exp, ok := program.First().(*ast.Bool)
		require.True(t, ok)
		require.Equal(t, exp.Value(), tt.boolValue)
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
		require.Nil(t, err)
		require.Len(t, program.Statements(), 1)
		exp, ok := program.First().(*ast.Prefix)
		require.True(t, ok)
		require.Equal(t, exp.Operator(), tt.operator)
		testLiteralExpression(t, exp.Right(), tt.integerValue)
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
		require.Nil(t, err)
		require.Len(t, program.Statements(), 1)
		expr, ok := program.First().(ast.Expression)
		require.True(t, ok)
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
		{"a*[1,2,3,4][b*c]*d", "((a * ([1, 2, 3, 4][(b * c)])) * d)"},
		{"add(a*b[2], b[1], 2 * [1,2][1])", "add((a * (b[2])), (b[1]), (2 * ([1, 2][1])))"},
		{"1 - (2 - 3);", "(1 - (2 - 3))"},
		{"return 1 - (2 - 3)", "return (1 - (2 - 3))"},
		{"return foo[0];\n -3;", "return (foo[0])\n(-3)"},
	}
	for _, tt := range tests {
		program, err := Parse(context.Background(), tt.input)
		require.Nil(t, err)
		actual := program.String()
		require.Equal(t, tt.expected, actual)
	}
}

func TestIf(t *testing.T) {
	program, err := Parse(context.Background(), "if x < y { x }")
	require.Nil(t, err)
	require.Len(t, program.Statements(), 1)
	exp, ok := program.First().(*ast.If)
	require.True(t, ok)
	if !testInfixExpression(t, exp.Condition(), "x", "<", "y") {
		return
	}
	require.Len(t, exp.Consequence().Statements(), 1)
	consequence, ok := exp.Consequence().Statements()[0].(*ast.Ident)
	require.True(t, ok)
	require.Equal(t, "x", consequence.String())
	require.Nil(t, exp.Alternative())
}

func TestFunc(t *testing.T) {
	program, err := Parse(context.Background(), "function f(x, y=3) { x + y; }")
	require.Nil(t, err)
	require.Len(t, program.Statements(), 1)
	function, ok := program.First().(*ast.Func)
	require.True(t, ok)
	params := function.Parameters()
	require.Len(t, params, 2)
	testLiteralExpression(t, params[0], "x")
	testLiteralExpression(t, params[1], "y")
	require.Len(t, function.Body().Statements(), 1)
	bodyStmt, ok := function.Body().Statements()[0].(*ast.Infix)
	require.True(t, ok)
	require.Equal(t, "(x + y)", bodyStmt.String())
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
		require.Nil(t, err)
		require.Len(t, program.Statements(), 1)
		function, ok := program.First().(*ast.Func)
		require.True(t, ok)
		params := function.Parameters()
		require.Len(t, params, len(tt.expectedParam))
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
			require.Nil(t, err, "parse error for %q", tt.input)
			require.Len(t, program.Statements(), 1)
			function, ok := program.First().(*ast.Func)
			require.True(t, ok, "expected Func, got %T", program.First())
			require.Nil(t, function.Name(), "arrow functions should not have names")
			params := function.Parameters()
			require.Len(t, params, len(tt.expectedParam))
			for i, ident := range tt.expectedParam {
				testLiteralExpression(t, params[i], ident)
			}
		})
	}
}

func TestArrowFunctionWithDefaults(t *testing.T) {
	program, err := Parse(context.Background(), "(x, y = 5) => x + y")
	require.Nil(t, err)
	require.Len(t, program.Statements(), 1)
	function, ok := program.First().(*ast.Func)
	require.True(t, ok)
	params := function.Parameters()
	require.Len(t, params, 2)
	testLiteralExpression(t, params[0], "x")
	testLiteralExpression(t, params[1], "y")
	defaults := function.Defaults()
	require.Len(t, defaults, 1)
	require.Contains(t, defaults, "y")
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
			require.Nil(t, err, "parse error for %q", tt.input)
			require.Len(t, program.Statements(), 1)
			function, ok := program.First().(*ast.Func)
			require.True(t, ok, "expected Func, got %T", program.First())
			require.Nil(t, function.Name(), "arrow functions should not have names")
			params := function.Parameters()
			require.Len(t, params, 1)
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
			require.NotNil(t, err)
			pe, ok := err.(ParserError)
			require.True(t, ok)
			require.Equal(t, tt.expected, pe.Error())
		})
	}
}

func TestCall(t *testing.T) {
	program, err := Parse(context.Background(), "add(1, 2*3, 4+5)")
	require.Nil(t, err)
	require.Len(t, program.Statements(), 1)
	expr, ok := program.First().(*ast.Call)
	require.True(t, ok)
	if !testIdentifier(t, expr.Function(), "add") {
		return
	}
	args := expr.Arguments()
	require.Len(t, args, 3)
	testLiteralExpression(t, args[0].(ast.Expression), 1)
	testInfixExpression(t, args[1].(ast.Expression), 2, "*", 3)
	testInfixExpression(t, args[2].(ast.Expression), 4, "+", 5)
}

func TestString(t *testing.T) {
	program, err := Parse(context.Background(), `"hello world";`)
	require.Nil(t, err)
	require.Len(t, program.Statements(), 1)
	literal, ok := program.First().(*ast.String)
	require.True(t, ok)
	require.Equal(t, "hello world", literal.Value())
}

func TestList(t *testing.T) {
	program, err := Parse(context.Background(), "[1, 2*2, 3+3]")
	require.Nil(t, err)
	require.Len(t, program.Statements(), 1)
	ll, ok := program.First().(*ast.List)
	require.True(t, ok)
	items := ll.Items()
	require.Len(t, items, 3)
	testIntegerLiteral(t, items[0], 1)
	testInfixExpression(t, items[1], 2, "*", 2)
	testInfixExpression(t, items[2], 3, "+", 3)
}

func TestIndex(t *testing.T) {
	input := "myArray[1+1]"
	program, err := Parse(context.Background(), input)
	require.Nil(t, err)
	require.Len(t, program.Statements(), 1)
	indexExp, ok := program.First().(*ast.Index)
	require.True(t, ok)
	testIdentifier(t, indexExp.Left(), "myArray")
	testInfixExpression(t, indexExp.Index(), 1, "+", 1)
}

func TestParsingMap(t *testing.T) {
	input := `{"one":1, "two":2, "three":3}`
	program, err := Parse(context.Background(), input)
	require.Nil(t, err)
	require.Len(t, program.Statements(), 1)
	m, ok := program.First().(*ast.Map)
	require.True(t, ok)
	require.Len(t, m.Items(), 3)
	expected := map[string]int64{
		"one":   1,
		"two":   2,
		"three": 3,
	}
	for _, item := range m.Items() {
		literal, ok := item.Key.(*ast.String)
		require.True(t, ok)
		expectedValue := expected[literal.Value()]
		testIntegerLiteral(t, item.Value, expectedValue)
	}
}

func TestParsingEmptyMap(t *testing.T) {
	input := "{}"
	program, err := Parse(context.Background(), input)
	require.Nil(t, err)
	require.Len(t, program.Statements(), 1)
	m, ok := program.First().(*ast.Map)
	require.True(t, ok)
	require.Len(t, m.Items(), 0)
}

func TestParsingMapLiteralWithExpression(t *testing.T) {
	input := `{"one":0+1, "two":10 - 8, "three": 15/5}`
	program, err := Parse(context.Background(), input)
	require.Nil(t, err)
	require.Len(t, program.Statements(), 1)
	m, ok := program.First().(*ast.Map)
	require.True(t, ok)
	require.Len(t, m.Items(), 3)
	tests := map[string]func(ast.Expression){
		"one": func(e ast.Expression) {
			testInfixExpression(t, e, 0, "+", 1)
		},
		"two": func(e ast.Expression) {
			testInfixExpression(t, e, 10, "-", 8)
		},
		"three": func(e ast.Expression) {
			testInfixExpression(t, e, 15, "/", 5)
		},
	}
	for _, item := range m.Items() {
		literal, ok := item.Key.(*ast.String)
		require.True(t, ok)
		testFunc, ok := tests[literal.Value()]
		require.True(t, ok, literal.Value())
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
	}
	for _, input := range inputs {
		_, err := Parse(context.Background(), input)
		require.Nil(t, err)
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
		require.Nil(t, err)
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
		{`for let i = 0; i < 5; i++ {`, "parse error: unterminated block statement"},
		{`{`, "parse error: unexpected end of file while parsing map (expected :)"},
		{`[`, "parse error: invalid syntax in list expression"},
		{`{ "a": "b", "c": "d"`, "parse error: unexpected end of file while parsing map (expected })"},
		{`{ "a", "b", "c"`, "parse error: unexpected , while parsing map (expected :)"},
		{`foo |`, "parse error: invalid pipe expression"},
		{`(1, 2`, "parse error: unexpected end of file while parsing grouped expression or arrow function (expected ))"},
	}
	for _, tt := range tests {
		_, err := Parse(context.Background(), tt.input)
		require.NotNil(t, err)
		pe, ok := err.(ParserError)
		require.True(t, ok)
		require.Equal(t, tt.expected, pe.Error())
	}
}

func TestSwitch(t *testing.T) {
	input := `switch val {
	case 1:
	default:
      x
	  x
}`
	program, err := Parse(context.Background(), input)
	require.Nil(t, err)
	require.Len(t, program.Statements(), 1)
	switchExpr, ok := program.First().(*ast.Switch)
	require.True(t, ok)
	require.Equal(t, "val", switchExpr.Value().String())
	require.Len(t, switchExpr.Choices(), 2)
	choice1 := switchExpr.Choices()[0]
	require.Len(t, choice1.Expressions(), 1)
	require.Equal(t, "1", choice1.Expressions()[0].String())
	choice2 := switchExpr.Choices()[1]
	require.Len(t, choice2.Expressions(), 0)
}

func TestMultiDefault(t *testing.T) {
	input := `
switch val {
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
	require.NotNil(t, err)
	parserErr, ok := err.(ParserError)
	require.True(t, ok)
	require.Equal(t, "parse error: switch statement has multiple default blocks", parserErr.Error())
	require.Equal(t, 0, parserErr.StartPosition().Column)
	require.Equal(t, 8, parserErr.StartPosition().Line)
	require.Equal(t, 6, parserErr.EndPosition().Column) // last col in the word "default"
	require.Equal(t, 8, parserErr.EndPosition().Line)
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
		require.Nil(t, err)
		require.Len(t, program.Statements(), 1)
		stmt := program.First().(*ast.Var)
		name, expr := stmt.Value()
		require.Equal(t, "x", name)
		pipe, ok := expr.(*ast.Pipe)
		require.True(t, ok)
		pipeExprs := pipe.Expressions()
		require.Len(t, pipeExprs, len(tt.expectedIdents))
		if tt.exprType == "ident" {
			for i, ident := range tt.expectedIdents {
				identExpr, ok := pipeExprs[i].(*ast.Ident)
				require.True(t, ok)
				require.Equal(t, ident, identExpr.String())
			}
		} else if tt.exprType == "call" {
			for i, ident := range tt.expectedIdents {
				callExpr, ok := pipeExprs[i].(*ast.Call)
				require.True(t, ok)
				require.Equal(t, ident, callExpr.Function().String())
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
	require.Nil(t, err)
	require.Len(t, program.Statements(), 1)
	expr := program.First()
	m, ok := expr.(*ast.Map)
	require.True(t, ok)
	require.Len(t, m.Items(), 2)
}

func TestMapExpressionWithoutComma(t *testing.T) {
	input := `{
		"a": "b",

		"c": "d"


	}
	`
	program, err := Parse(context.Background(), input)
	require.Nil(t, err)
	require.Len(t, program.Statements(), 1)
	expr := program.First()
	m, ok := expr.(*ast.Map)
	require.True(t, ok)
	require.Len(t, m.Items(), 2)
}

func TestCallExpression(t *testing.T) {
	input := `foo(
		a=1,
		b=2,
	)
	`
	program, err := Parse(context.Background(), input)
	require.Nil(t, err)
	require.Len(t, program.Statements(), 1)
	expr := program.First()
	call, ok := expr.(*ast.Call)
	require.True(t, ok)
	require.Equal(t, "foo", call.Function().String())
	args := call.Arguments()
	require.Len(t, args, 2)
	arg0 := args[0].(*ast.Assign)
	require.Equal(t, "a = 1", arg0.String())
	arg1 := args[1].(*ast.Assign)
	require.Equal(t, "b = 2", arg1.String())
}

func TestGetAttr(t *testing.T) {
	program, err := Parse(context.Background(), "foo.bar")
	require.Nil(t, err)
	require.Len(t, program.Statements(), 1)
	expr := program.First()
	getAttr, ok := expr.(*ast.GetAttr)
	require.True(t, ok)
	require.Equal(t, "bar", getAttr.Name())
	require.Equal(t, "foo.bar", getAttr.String())
}

func TestForInLoop(t *testing.T) {
	tests := []struct {
		input    string
		variable string
		iterable string
	}{
		{
			"for x in [1, 2, 3] { }",
			"x",
			"[1, 2, 3]",
		},
		{
			"for item in items { print(item) }",
			"item",
			"items",
		},
		{
			"for fruit in fruits { print(fruit) }",
			"fruit",
			"fruits",
		},
	}
	for _, tt := range tests {
		program, err := Parse(context.Background(), tt.input)
		require.Nil(t, err)
		require.Len(t, program.Statements(), 1)
		expr, ok := program.First().(*ast.ForIn)
		require.True(t, ok)
		require.Equal(t, tt.variable, expr.Variable().String())
		require.Equal(t, tt.iterable, expr.Iterable().String())
	}
}

func TestForInLoopErrors(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expectedErr string
	}{
		{
			name:        "missing iterable",
			input:       "for x in { }",
			expectedErr: "unexpected end of file",
		},
		{
			name:        "missing opening brace",
			input:       "for x in items",
			expectedErr: "unexpected end of file",
		},
		{
			name:        "missing variable",
			input:       "for in items { }",
			expectedErr: "unexpected \"in\"",
		},
		{
			name:        "invalid token after in",
			input:       "for x in { }",
			expectedErr: "unexpected end of file",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Parse(context.Background(), tt.input)
			require.NotNil(t, err)
			require.Contains(t, err.Error(), tt.expectedErr)
		})
	}
}

func TestForInLoopAST(t *testing.T) {
	input := "for item in collection { print(item) }"
	program, err := Parse(context.Background(), input)
	require.Nil(t, err)
	require.Len(t, program.Statements(), 1)

	forIn, ok := program.First().(*ast.ForIn)
	require.True(t, ok)

	// Test all AST node methods
	require.Equal(t, "for", forIn.Literal())
	require.Equal(t, "FOR", string(forIn.Token().Type))
	require.False(t, forIn.IsExpression())
	require.Equal(t, "item", forIn.Variable().Literal())
	require.Equal(t, "collection", forIn.Iterable().String())
	require.NotNil(t, forIn.Consequence())

	// Test String() method
	expected := "for item in collection print(item)"
	require.Equal(t, expected, forIn.String())
}

func TestForInLoopEdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		variable string
		iterable string
	}{
		{
			name:     "nested array access",
			input:    "for val in arr[0] { }",
			variable: "val",
			iterable: "(arr[0])",
		},
		{
			name:     "map access",
			input:    "for item in data.items { }",
			variable: "item",
			iterable: "data.items",
		},
		{
			name:     "complex expression chain",
			input:    "for x in obj.method().field[0] { }",
			variable: "x",
			iterable: "(obj.method().field[0])",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			program, err := Parse(context.Background(), tt.input)
			require.Nil(t, err)
			require.Len(t, program.Statements(), 1)
			expr, ok := program.First().(*ast.ForIn)
			require.True(t, ok)
			require.Equal(t, tt.variable, expr.Variable().String())
			require.Equal(t, tt.iterable, expr.Iterable().String())
		})
	}
}

func TestForInLoopParseErrorCases(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{
			name:  "missing variable name",
			input: "for in items { }",
		},
		{
			name:  "missing 'in' keyword",
			input: "for x items { }",
		},
		{
			name:  "unterminated - missing brace",
			input: "for x in items",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Parse(context.Background(), tt.input)
			require.NotNil(t, err, "Expected parse error for input: %s", tt.input)
		})
	}
}

func TestMultiVar(t *testing.T) {
	program, err := Parse(context.Background(), "let x, y = [1, 2]")
	require.Nil(t, err)
	require.Len(t, program.Statements(), 1)
	mvar, ok := program.First().(*ast.MultiVar)
	require.True(t, ok)
	names, expr := mvar.Value()
	require.Equal(t, []string{"x", "y"}, names)
	require.Equal(t, "[1, 2]", expr.String())
}

func TestIn(t *testing.T) {
	program, err := Parse(context.Background(), "x in [1, 2]")
	require.Nil(t, err)
	require.Len(t, program.Statements(), 1)
	node, ok := program.First().(*ast.In)
	require.True(t, ok)
	require.Equal(t, "in", node.Literal())
	require.Equal(t, "x", node.Left().String())
	require.Equal(t, "[1, 2]", node.Right().String())
	require.Equal(t, "x in [1, 2]", node.String())
}

func TestNotIn(t *testing.T) {
	program, err := Parse(context.Background(), "x not in [1, 2]")
	require.Nil(t, err)
	require.Len(t, program.Statements(), 1)
	node, ok := program.First().(*ast.NotIn)
	require.True(t, ok)
	require.Equal(t, "not", node.Literal())
	require.Equal(t, "x", node.Left().String())
	require.Equal(t, "[1, 2]", node.Right().String())
	require.Equal(t, "x not in [1, 2]", node.String())
}

func TestBreak(t *testing.T) {
	program, err := Parse(context.Background(), "break")
	require.Nil(t, err)
	require.Len(t, program.Statements(), 1)
	_, ok := program.First().(*ast.Control)
	require.True(t, ok)
}

func TestBacktick(t *testing.T) {
	input := "`" + `\\n\t foo bar /hey there/` + "`"
	program, err := Parse(context.Background(), input)
	require.Nil(t, err)
	require.Len(t, program.Statements(), 1)
	expr, ok := program.First().(*ast.String)
	require.True(t, ok)
	require.Equal(t, `\\n\t foo bar /hey there/`, expr.Value())
}

func TestUnterminatedBacktickString(t *testing.T) {
	input := "`foo"
	_, err := Parse(context.Background(), input)
	require.NotNil(t, err)
	require.Equal(t, "syntax error: unterminated string literal", err.Error())
	var syntaxErr *SyntaxError
	ok := errors.As(err, &syntaxErr)
	require.True(t, ok)
	require.NotNil(t, syntaxErr.Cause())
	require.Equal(t, "unterminated string literal", syntaxErr.Cause().Error())
	require.Equal(t, NewSyntaxError(ErrorOpts{
		ErrType: "syntax error",
		Cause:   syntaxErr.Cause(),
		StartPosition: token.Position{
			Value: rune('`'),
		},
		EndPosition: token.Position{
			Value:  rune('o'),
			Column: 3, // the last "o" in foo is at index 3
			Char:   3, // the last "o" in foo is at index 3
		},
		File:       "",
		SourceCode: "`foo",
	}), syntaxErr)
}

func TestUnterminatedString(t *testing.T) {
	input := `42
let x = "a`
	ctx := context.Background()
	_, err := Parse(ctx, input, WithFile("main.tm"))
	require.NotNil(t, err)
	fmt.Printf("%+v\n", err.(*SyntaxError).StartPosition())
	require.Equal(t, "syntax error: unterminated string literal", err.Error())
	var syntaxErr *SyntaxError
	ok := errors.As(err, &syntaxErr)
	require.True(t, ok)
	require.NotNil(t, syntaxErr.Cause())
	require.Equal(t, "unterminated string literal", syntaxErr.Cause().Error())
	require.Equal(t, NewSyntaxError(ErrorOpts{
		ErrType: "syntax error",
		Cause:   syntaxErr.Cause(),
		StartPosition: token.Position{
			Value:     rune('"'),
			Column:    8,
			Line:      1,
			LineStart: 3,
			Char:      11,
			File:      "main.tm",
		},
		EndPosition: token.Position{
			Value:     rune('a'),
			Column:    9,
			Line:      1,
			LineStart: 3,
			Char:      12,
			File:      "main.tm",
		},
		File:       "main.tm",
		SourceCode: `let x = "a`,
	}), syntaxErr)
}

func TestMapIdentifierKey(t *testing.T) {
	input := "{ one: 1 }"
	program, err := Parse(context.Background(), input)
	require.Nil(t, err)
	require.Len(t, program.Statements(), 1)
	m, ok := program.First().(*ast.Map)
	require.True(t, ok)
	require.Len(t, m.Items(), 1)
	for _, item := range m.Items() {
		ident, ok := item.Key.(*ast.Ident)
		require.True(t, ok, fmt.Sprintf("%T", item.Key))
		require.Equal(t, "one", ident.String())
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
		`for let i = 0; i < 10; i++ { x += i }`,
		`let x = 1; for i in [1, 2, 3] { print(x + i) }`,
		`[1] in {1, 2, 3}`,
		`let f = function(x) { function() { x + 1 } }; f(1)`,
		`switch x { case 1: 1 case 2: 2 default: 3 }`,
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
		{"if", `parse error: invalid syntax`},
		{"else", `parse error: invalid syntax (unexpected "else")`},
		{"&&", `parse error: invalid syntax (unexpected "&&")`},
		{"[", `parse error: invalid syntax in list expression`},
		{"[1,", `parse error: unexpected end of file while parsing an expression list (expected ])`},
		{"0?if", `parse error: invalid syntax in ternary if true expression`},
		{"0?0:", `parse error: invalid syntax in ternary if false expression`},
		{"range", `parse error: invalid range expression`},
		{"in", `parse error: invalid syntax (unexpected "in")`},
		{"x in", `parse error: invalid in expression`},
		{"switch x { case 1: \xf5\xf51 case 2: 2 default: 3 }", `syntax error: invalid identifier: �`},
		{"switch x { case 1: 1 case 2: 2 defaultIIIIIII: 3 }", "parse error: unexpected defaultIIIIIII while parsing case statement (expected ;)"},
		{`{ one: 1
			two: 2}`, "parse error: unexpected two while parsing map (expected })"},
		{`[1 2]`, "parse error: unexpected 2 while parsing an expression list (expected ])"},
		{`[1, 2, ,]`, "parse error: invalid syntax (unexpected \",\")"},
	}
	for _, tt := range tests {
		program, err := Parse(context.Background(), tt.input)
		fmt.Println(program)
		require.NotNil(t, err)
		require.Equal(t, tt.expected, err.Error())
	}
}

func TestRangePrecedence(t *testing.T) {
	// This confirms the correct precedence of the "range" vs. "call" operators
	input := `range list(99)`

	// Parse the program, which should be 1 statement in length
	program, err := Parse(context.Background(), input)
	require.Nil(t, err)
	require.Len(t, program.Statements(), 1)
	stmt := program.First()

	// The top-level of the AST should be a range statement
	require.IsType(t, &ast.Range{}, stmt)
	rangeStmt := stmt.(*ast.Range)

	// The container of the range statement should be a call expression
	require.IsType(t, &ast.Call{}, rangeStmt.Container())
	callStmt := rangeStmt.Container().(*ast.Call)

	// The function of the call expression should be an identifier (list)
	require.IsType(t, &ast.Ident{}, callStmt.Function())
	ident := callStmt.Function().(*ast.Ident)
	require.Equal(t, "list", ident.String())

	// The argument of the call expression should be an integer
	require.IsType(t, &ast.Int{}, callStmt.Arguments()[0])
	intVal := callStmt.Arguments()[0].(*ast.Int)
	require.Equal(t, int64(99), intVal.Value())
}

func TestInPrecedence(t *testing.T) {
	// This confirms the correct precedence of the "in" vs. "call" operators
	input := `2 in sorted([1,2,3])`

	// Parse the program, which should be 1 statement in length
	program, err := Parse(context.Background(), input)
	require.Nil(t, err)
	require.Len(t, program.Statements(), 1)
	stmt := program.First()

	// The top-level of the AST should be an in statement
	require.IsType(t, &ast.In{}, stmt)
	inStmt := stmt.(*ast.In)
	fmt.Println(inStmt.String())

	require.Equal(t, "2", inStmt.Left().String())
	require.Equal(t, "sorted([1, 2, 3])", inStmt.Right().String())
}

func TestNotInPrecedence(t *testing.T) {
	// This confirms the correct precedence of the "not in" vs. "call" operators
	input := `2 not in sorted([1,2,3])`

	// Parse the program, which should be 1 statement in length
	program, err := Parse(context.Background(), input)
	require.Nil(t, err)
	require.Len(t, program.Statements(), 1)
	stmt := program.First()

	// The top-level of the AST should be a not in statement
	require.IsType(t, &ast.NotIn{}, stmt)
	notInStmt := stmt.(*ast.NotIn)
	fmt.Println(notInStmt.String())

	require.Equal(t, "2", notInStmt.Left().String())
	require.Equal(t, "sorted([1, 2, 3])", notInStmt.Right().String())
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
		require.Nil(t, err)
		require.Equal(t, tt.expected, result.String())
	}
}

func TestGoIsReservedKeyword(t *testing.T) {
	// go statement was removed in v2 but keyword is reserved
	input := "go function() { 42 }()"
	_, err := Parse(context.Background(), input)
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "reserved keyword: go")
}

func TestFromImport(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"from math import min", `from "math" import "min"`},
		{"from math import min, max", `from "math" import "min", "max"`},
		{"from math import min as a, max as b", `from "math" import "min" as a, "max" as b`},
		{`from math import (
			min as a,
			max as b,
		  )`, `from "math" import ("min" as a, "max" as b)`},
	}
	for _, tt := range tests {
		result, err := Parse(context.Background(), tt.input)
		require.Nil(t, err)
		require.Equal(t, tt.expected, result.String())
		require.IsType(t, &ast.FromImport{}, result.Statements()[0])
	}
}

func TestBadFromImport(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"from math import", "parse error: unexpected end of file while parsing a from-import statement (expected identifier)"},
		{"from math import min,", "parse error: unexpected end of file while parsing a from-import statement (expected identifier)"},
		{"from math import min as a,", "parse error: unexpected end of file while parsing a from-import statement (expected identifier)"},
		{"from math import ", "parse error: unexpected end of file while parsing a from-import statement (expected identifier)"},
		{"from math", "parse error: from-import is missing import statement"},
		{"from math import (a", "parse error: unexpected end of file while parsing a from-import statement (expected ))"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			_, err := Parse(context.Background(), tt.input)
			require.NotNil(t, err)
			require.Equal(t, tt.expected, err.Error())
		})
	}
}

func TestInvalidListTermination(t *testing.T) {
	input := `
	{ data: { blocks: [ { type: "divider" },
		}
	}`
	_, err := Parse(context.Background(), input)
	require.Error(t, err)
	require.Equal(t, `parse error: invalid syntax (unexpected "}")`, err.Error())
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
			require.Nil(t, err)
			require.Equal(t, tt.expected, result.String())
		})
	}
}

func TestDoubleSemicolon(t *testing.T) {
	input := "42; ;"
	_, err := Parse(context.Background(), input)
	require.Error(t, err)
	require.Equal(t, "parse error: invalid syntax (unexpected \";\")", err.Error())
}

func TestInvalidMultipleExpressions(t *testing.T) {
	input := "42 33"
	_, err := Parse(context.Background(), input)
	require.Error(t, err)
	require.Equal(t, "parse error: unexpected token \"33\" following statement", err.Error())
}

func TestInvalidMultipleExpressions2(t *testing.T) {
	input := "42\n 33 oops"
	_, err := Parse(context.Background(), input)
	require.Error(t, err)
	require.Equal(t, "parse error: unexpected token \"oops\" following statement", err.Error())
}

func TestStringImport(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{`import "foo"`, `import "foo"`},
		{`import "mydir/foo"`, `import "mydir/foo"`},
		{`import "mydir/foo" as bar`, `import "mydir/foo" as bar`},
	}
	for _, tt := range tests {
		result, err := Parse(context.Background(), tt.input)
		require.Nil(t, err)
		require.Equal(t, tt.expected, result.String())
		require.IsType(t, &ast.Import{}, result.Statements()[0])
	}
}

func TestStringFromImport(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{`from mydir.foo import bar`, `from "mydir.foo" import "bar"`},
		{`from "mydir/foo" import bar`, `from "mydir/foo" import "bar"`},
		{`from "mydir/foo" import bar as baz`, `from "mydir/foo" import "bar" as baz`},
		{`from "mydir/foo" import (bar, baz)`, `from "mydir/foo" import ("bar", "baz")`},
	}
	for _, tt := range tests {
		result, err := Parse(context.Background(), tt.input)
		require.Nil(t, err)
		require.Equal(t, tt.expected, result.String())
		require.IsType(t, &ast.FromImport{}, result.Statements()[0])
	}
}

func TestESImport(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{`import { get } from "http"`, `import "http" import ("get")`},
		{`import { get, post } from "http"`, `import "http" import ("get", "post")`},
		{`import { get as httpGet } from "http"`, `import "http" import ("get" as httpGet)`},
		{`import { min, max as maximum } from "math"`, `import "math" import ("min", "max" as maximum)`},
		{`import { read } from "os"`, `import "os" import ("read")`},
		// Trailing comma is allowed
		{`import { get, } from "http"`, `import "http" import ("get")`},
	}
	for _, tt := range tests {
		result, err := Parse(context.Background(), tt.input)
		require.Nil(t, err, "input: %s", tt.input)
		require.Equal(t, tt.expected, result.String(), "input: %s", tt.input)
		require.IsType(t, &ast.FromImport{}, result.Statements()[0], "input: %s", tt.input)
	}
}

func TestBadESImport(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{`import { } from "http"`, "parse error: import list cannot be empty"},
		{`import { get from "http"`, "parse error: expected ',' or '}' in import list"},
		{`import { get }`, "parse error: unexpected end of file while parsing ES import (expected FROM)"},
		{`import { get } from`, "parse error: unexpected end of file while parsing ES import (expected STRING)"},
	}
	for _, tt := range tests {
		_, err := Parse(context.Background(), tt.input)
		require.NotNil(t, err, "expected error for: %s", tt.input)
		require.Contains(t, err.Error(), tt.expected, "input: %s", tt.input)
	}
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
		require.Nil(t, err, "input: %s", tt.input)
		require.Equal(t, tt.expected, result.String(), "input: %s", tt.input)
	}
}

func TestBitwiseAnd(t *testing.T) {
	input := "1 & 2"
	result, err := Parse(context.Background(), input)
	require.Nil(t, err)
	require.Equal(t, "(1 & 2)", result.String())
}

func TestForLoop(t *testing.T) {
	tests := []struct {
		input   string
		initStr string
		condStr string
		postStr string
	}{
		{
			"for let i = 0; i < 5; i++ { }",
			"let i = 0",
			"(i < 5)",
			"(i++)",
		},
		{
			"for let i = 2+2; x < i; x-- { }",
			"let i = (2 + 2)",
			"(x < i)",
			"(x--)",
		},
		{
			"for let i = range mymap { }",
			"",
			"let i = range mymap",
			"",
		},
		{
			"for let k, v = range [1,2,3,4] { }",
			"",
			"let k, v = range [1, 2, 3, 4]",
			"",
		},
	}
	for _, tt := range tests {
		program, err := Parse(context.Background(), tt.input)
		require.Nil(t, err)
		require.Len(t, program.Statements(), 1)
		expr, ok := program.First().(*ast.For)
		require.True(t, ok)
		require.Equal(t, tt.condStr, expr.Condition().String())
		if tt.initStr != "" {
			require.Equal(t, tt.initStr, expr.Init().String())
		} else if expr.Init() != nil {
			t.Fatalf("expected no init statement. got='%v'", expr.Init().String())
		}
		if tt.postStr != "" {
			require.Equal(t, tt.postStr, expr.Post().String())
		} else if expr.Post() != nil {
			t.Fatalf("expected no post statement. got='%v'", expr.Post().String())
		}
	}
}
