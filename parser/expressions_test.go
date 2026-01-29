package parser

import (
	"context"
	"testing"

	"github.com/deepnoodle-ai/wonton/assert"
	"github.com/risor-io/risor/ast"
)

// Tests for expression parsing (expressions.go)
// - Identifiers
// - Prefix expressions
// - Infix expressions
// - Ternary expressions
// - If expressions
// - Switch expressions
// - Index/slice expressions
// - Call expressions
// - Pipe expressions
// - In/not in expressions
// - GetAttr expressions
// - Optional chaining

func TestIdent(t *testing.T) {
	program, err := Parse(context.Background(), "foobar;", nil)
	assert.Nil(t, err)
	assert.Len(t, program.Stmts, 1)

	ident, ok := program.First().(*ast.Ident)
	assert.True(t, ok)
	assert.Equal(t, "foobar", ident.String())
	assert.Equal(t, "foobar", ident.Name)
}

func TestIdentAST(t *testing.T) {
	program, err := Parse(context.Background(), "myVar", nil)
	assert.Nil(t, err)

	ident, ok := program.First().(*ast.Ident)
	assert.True(t, ok)

	// Verify AST node fields
	assert.Equal(t, "myVar", ident.Name)
	assert.Equal(t, "myVar", ident.String())
	assert.NotEqual(t, ident.Pos(), ident.End())
}

func TestPrefix(t *testing.T) {
	tests := []struct {
		input    string
		operator string
		value    interface{}
	}{
		{"!5;", "!", 5},
		{"-15;", "-", 15},
		{"!true;", "!", true},
		{"!false", "!", false},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			program, err := Parse(context.Background(), tt.input, nil)
			assert.Nil(t, err)
			assert.Len(t, program.Stmts, 1)

			exp, ok := program.First().(*ast.Prefix)
			assert.True(t, ok)
			assert.Equal(t, tt.operator, exp.Op)
			testLiteralExpression(t, exp.X, tt.value)
		})
	}
}

func TestPrefixAST(t *testing.T) {
	program, err := Parse(context.Background(), "-42", nil)
	assert.Nil(t, err)

	prefix, ok := program.First().(*ast.Prefix)
	assert.True(t, ok)

	// Verify AST node fields
	assert.Equal(t, "-", prefix.Op)
	assert.Equal(t, "(-42)", prefix.String())

	operand, ok := prefix.X.(*ast.Int)
	assert.True(t, ok)
	assert.Equal(t, int64(42), operand.Value)
}

func TestInfix(t *testing.T) {
	tests := []struct {
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
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			program, err := Parse(context.Background(), tt.input, nil)
			assert.Nil(t, err)
			assert.Len(t, program.Stmts, 1)

			expr, ok := program.First().(ast.Expr)
			assert.True(t, ok)
			testInfixExpression(t, expr, tt.leftValue, tt.operator, tt.rightValue)
		})
	}
}

func TestInfixAST(t *testing.T) {
	program, err := Parse(context.Background(), "1 + 2", nil)
	assert.Nil(t, err)

	infix, ok := program.First().(*ast.Infix)
	assert.True(t, ok)

	// Verify AST node fields
	assert.Equal(t, "+", infix.Op)
	assert.Equal(t, "(1 + 2)", infix.String())

	left, ok := infix.X.(*ast.Int)
	assert.True(t, ok)
	assert.Equal(t, int64(1), left.Value)

	right, ok := infix.Y.(*ast.Int)
	assert.True(t, ok)
	assert.Equal(t, int64(2), right.Value)
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
		t.Run(tt.input, func(t *testing.T) {
			program, err := Parse(context.Background(), tt.input, nil)
			assert.Nil(t, err)
			assert.Equal(t, tt.expected, program.String())
		})
	}
}

func TestIf(t *testing.T) {
	program, err := Parse(context.Background(), "if (x < y) { x }", nil)
	assert.Nil(t, err)
	assert.Len(t, program.Stmts, 1)

	exp, ok := program.First().(*ast.If)
	assert.True(t, ok)
	testInfixExpression(t, exp.Cond, "x", "<", "y")
	assert.Len(t, exp.Consequence.Stmts, 1)

	consequence, ok := exp.Consequence.Stmts[0].(*ast.Ident)
	assert.True(t, ok)
	assert.Equal(t, "x", consequence.String())
	assert.Nil(t, exp.Alternative)
}

func TestIfAST(t *testing.T) {
	program, err := Parse(context.Background(), "if (true) { 1 } else { 2 }", nil)
	assert.Nil(t, err)

	ifExpr, ok := program.First().(*ast.If)
	assert.True(t, ok)

	// Verify AST node fields
	assert.NotNil(t, ifExpr.Cond)
	assert.NotNil(t, ifExpr.Consequence)
	assert.NotNil(t, ifExpr.Alternative)

	// Verify condition
	cond, ok := ifExpr.Cond.(*ast.Bool)
	assert.True(t, ok)
	assert.True(t, cond.Value)

	// Verify consequence
	assert.Len(t, ifExpr.Consequence.Stmts, 1)

	// Verify alternative
	assert.Len(t, ifExpr.Alternative.Stmts, 1)
}

func TestIfElseIf(t *testing.T) {
	input := `if (x > 0) {
		"positive"
	} else if (x < 0) {
		"negative"
	} else {
		"zero"
	}`
	program, err := Parse(context.Background(), input, nil)
	assert.Nil(t, err)
	assert.Len(t, program.Stmts, 1)

	ifExpr, ok := program.First().(*ast.If)
	assert.True(t, ok)
	assert.NotNil(t, ifExpr.Alternative)

	// Alternative should contain another if
	assert.Len(t, ifExpr.Alternative.Stmts, 1)
	nestedIf, ok := ifExpr.Alternative.Stmts[0].(*ast.If)
	assert.True(t, ok)
	assert.NotNil(t, nestedIf.Alternative)
}

func TestSwitch(t *testing.T) {
	input := `switch (val) {
	case 1:
	default:
      x
	  x
}`
	program, err := Parse(context.Background(), input, nil)
	assert.Nil(t, err)
	assert.Len(t, program.Stmts, 1)

	switchExpr, ok := program.First().(*ast.Switch)
	assert.True(t, ok)
	assert.Equal(t, "val", switchExpr.Value.String())
	assert.Len(t, switchExpr.Cases, 2)

	choice1 := switchExpr.Cases[0]
	assert.Len(t, choice1.Exprs, 1)
	assert.Equal(t, "1", choice1.Exprs[0].String())

	choice2 := switchExpr.Cases[1]
	assert.Len(t, choice2.Exprs, 0) // default case
}

func TestSwitchAST(t *testing.T) {
	input := `switch (x) { case 1: "one" case 2: "two" default: "other" }`
	program, err := Parse(context.Background(), input, nil)
	assert.Nil(t, err)

	sw, ok := program.First().(*ast.Switch)
	assert.True(t, ok)

	// Verify AST node fields
	assert.NotNil(t, sw.Value)
	assert.Len(t, sw.Cases, 3)

	// First case
	assert.Len(t, sw.Cases[0].Exprs, 1)
	caseVal, ok := sw.Cases[0].Exprs[0].(*ast.Int)
	assert.True(t, ok)
	assert.Equal(t, int64(1), caseVal.Value)

	// Default case (last)
	assert.Len(t, sw.Cases[2].Exprs, 0) // no expressions = default
}

func TestSwitchMultipleCaseValues(t *testing.T) {
	input := `switch (x) { case 1, 2, 3: "small" default: "big" }`
	program, err := Parse(context.Background(), input, nil)
	assert.Nil(t, err)

	sw, ok := program.First().(*ast.Switch)
	assert.True(t, ok)
	assert.Len(t, sw.Cases, 2)

	// First case has 3 values
	assert.Len(t, sw.Cases[0].Exprs, 3)
}

func TestIndex(t *testing.T) {
	input := "myArray[1+1]"
	program, err := Parse(context.Background(), input, nil)
	assert.Nil(t, err)
	assert.Len(t, program.Stmts, 1)

	indexExp, ok := program.First().(*ast.Index)
	assert.True(t, ok)
	testIdentifier(t, indexExp.X, "myArray")
	testInfixExpression(t, indexExp.Index, 1, "+", 1)
}

func TestIndexAST(t *testing.T) {
	program, err := Parse(context.Background(), "arr[0]", nil)
	assert.Nil(t, err)

	index, ok := program.First().(*ast.Index)
	assert.True(t, ok)

	// Verify AST node fields
	obj, ok := index.X.(*ast.Ident)
	assert.True(t, ok)
	assert.Equal(t, "arr", obj.Name)

	idx, ok := index.Index.(*ast.Int)
	assert.True(t, ok)
	assert.Equal(t, int64(0), idx.Value)

	assert.Equal(t, "arr[0]", index.String())
}

func TestSlice(t *testing.T) {
	tests := []struct {
		input    string
		expected string
		hasLow   bool
		hasHigh  bool
	}{
		{`x[1:3]`, "x[1:3]", true, true},
		{`x[:3]`, "x[:3]", false, true},
		{`x[1:]`, "x[1:]", true, false},
		{`x[:]`, "x[:]", false, false},
		{`arr[start:end]`, "arr[start:end]", true, true},
		{`s[0:len(s)]`, "s[0:len(s)]", true, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			program, err := Parse(context.Background(), tt.input, nil)
			assert.Nil(t, err)
			assert.Len(t, program.Stmts, 1)

			slice, ok := program.First().(*ast.Slice)
			assert.True(t, ok, "expected Slice, got %T", program.First())
			assert.Equal(t, tt.expected, slice.String())

			if tt.hasLow {
				assert.NotNil(t, slice.Low)
			} else {
				assert.Nil(t, slice.Low)
			}
			if tt.hasHigh {
				assert.NotNil(t, slice.High)
			} else {
				assert.Nil(t, slice.High)
			}
		})
	}
}

func TestSliceAST(t *testing.T) {
	program, err := Parse(context.Background(), "arr[1:5]", nil)
	assert.Nil(t, err)

	slice, ok := program.First().(*ast.Slice)
	assert.True(t, ok)

	// Verify AST node fields
	obj, ok := slice.X.(*ast.Ident)
	assert.True(t, ok)
	assert.Equal(t, "arr", obj.Name)

	low, ok := slice.Low.(*ast.Int)
	assert.True(t, ok)
	assert.Equal(t, int64(1), low.Value)

	high, ok := slice.High.(*ast.Int)
	assert.True(t, ok)
	assert.Equal(t, int64(5), high.Value)
}

func TestCall(t *testing.T) {
	program, err := Parse(context.Background(), "add(1, 2*3, 4+5)", nil)
	assert.Nil(t, err)
	assert.Len(t, program.Stmts, 1)

	expr, ok := program.First().(*ast.Call)
	assert.True(t, ok)
	testIdentifier(t, expr.Fun, "add")

	args := expr.Args
	assert.Len(t, args, 3)
	testLiteralExpression(t, args[0].(ast.Expr), 1)
	testInfixExpression(t, args[1].(ast.Expr), 2, "*", 3)
	testInfixExpression(t, args[2].(ast.Expr), 4, "+", 5)
}

func TestCallAST(t *testing.T) {
	program, err := Parse(context.Background(), "print(42)", nil)
	assert.Nil(t, err)

	call, ok := program.First().(*ast.Call)
	assert.True(t, ok)

	// Verify AST node fields
	fn, ok := call.Fun.(*ast.Ident)
	assert.True(t, ok)
	assert.Equal(t, "print", fn.Name)

	assert.Len(t, call.Args, 1)
	arg, ok := call.Args[0].(*ast.Int)
	assert.True(t, ok)
	assert.Equal(t, int64(42), arg.Value)
}

func TestCallEmptyWithNewlines(t *testing.T) {
	program, err := Parse(context.Background(), "noop(\n)", nil)
	assert.Nil(t, err)
	assert.Len(t, program.Stmts, 1)

	call, ok := program.First().(*ast.Call)
	assert.True(t, ok)
	assert.Len(t, call.Args, 0)
}

func TestCallWithKeywordArgs(t *testing.T) {
	input := `foo(a=1, b=2)`
	program, err := Parse(context.Background(), input, nil)
	assert.Nil(t, err)
	assert.Len(t, program.Stmts, 1)

	call, ok := program.First().(*ast.Call)
	assert.True(t, ok)
	assert.Equal(t, "foo", call.Fun.String())

	args := call.Args
	assert.Len(t, args, 2)

	arg0 := args[0].(*ast.Assign)
	assert.Equal(t, "a = 1", arg0.String())

	arg1 := args[1].(*ast.Assign)
	assert.Equal(t, "b = 2", arg1.String())
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
		t.Run(tt.input, func(t *testing.T) {
			program, err := Parse(context.Background(), tt.input, nil)
			assert.Nil(t, err)
			assert.Len(t, program.Stmts, 1)

			stmt := program.First().(*ast.Var)
			assert.Equal(t, "x", stmt.Name.Name)

			pipe, ok := stmt.Value.(*ast.Pipe)
			assert.True(t, ok)

			pipeExprs := pipe.Exprs
			assert.Len(t, pipeExprs, len(tt.expectedIdents))

			if tt.exprType == "ident" {
				for i, ident := range tt.expectedIdents {
					identExpr, ok := pipeExprs[i].(*ast.Ident)
					assert.True(t, ok)
					assert.Equal(t, ident, identExpr.String())
				}
			} else if tt.exprType == "call" {
				for i, ident := range tt.expectedIdents {
					callExpr, ok := pipeExprs[i].(*ast.Call)
					assert.True(t, ok)
					assert.Equal(t, ident, callExpr.Fun.String())
				}
			}
		})
	}
}

func TestPipeAST(t *testing.T) {
	program, err := Parse(context.Background(), "data | filter | sort", nil)
	assert.Nil(t, err)

	pipe, ok := program.First().(*ast.Pipe)
	assert.True(t, ok)

	// Verify AST node fields
	assert.Len(t, pipe.Exprs, 3)

	for i, name := range []string{"data", "filter", "sort"} {
		ident, ok := pipe.Exprs[i].(*ast.Ident)
		assert.True(t, ok)
		assert.Equal(t, name, ident.Name)
	}
}

func TestIn(t *testing.T) {
	program, err := Parse(context.Background(), "x in [1, 2]", nil)
	assert.Nil(t, err)
	assert.Len(t, program.Stmts, 1)

	node, ok := program.First().(*ast.In)
	assert.True(t, ok)
	assert.Equal(t, "x", node.X.String())
	assert.Equal(t, "[1, 2]", node.Y.String())
	assert.Equal(t, "x in [1, 2]", node.String())
}

func TestInAST(t *testing.T) {
	program, err := Parse(context.Background(), "key in map", nil)
	assert.Nil(t, err)

	in, ok := program.First().(*ast.In)
	assert.True(t, ok)

	// Verify AST node fields
	x, ok := in.X.(*ast.Ident)
	assert.True(t, ok)
	assert.Equal(t, "key", x.Name)

	y, ok := in.Y.(*ast.Ident)
	assert.True(t, ok)
	assert.Equal(t, "map", y.Name)
}

func TestNotIn(t *testing.T) {
	program, err := Parse(context.Background(), "x not in [1, 2]", nil)
	assert.Nil(t, err)
	assert.Len(t, program.Stmts, 1)

	node, ok := program.First().(*ast.NotIn)
	assert.True(t, ok)
	assert.Equal(t, "x", node.X.String())
	assert.Equal(t, "[1, 2]", node.Y.String())
	assert.Equal(t, "x not in [1, 2]", node.String())
}

func TestInWithNewline(t *testing.T) {
	program, err := Parse(context.Background(), "x in\n[1, 2]", nil)
	assert.Nil(t, err)
	assert.Len(t, program.Stmts, 1)
	assert.Equal(t, "x in [1, 2]", program.First().String())
}

func TestNotInWithNewline(t *testing.T) {
	program, err := Parse(context.Background(), "x not in\n[1, 2]", nil)
	assert.Nil(t, err)
	assert.Len(t, program.Stmts, 1)
	assert.Equal(t, "x not in [1, 2]", program.First().String())
}

func TestNotInAST(t *testing.T) {
	program, err := Parse(context.Background(), "key not in set", nil)
	assert.Nil(t, err)

	notIn, ok := program.First().(*ast.NotIn)
	assert.True(t, ok)

	// Verify AST node fields
	x, ok := notIn.X.(*ast.Ident)
	assert.True(t, ok)
	assert.Equal(t, "key", x.Name)

	y, ok := notIn.Y.(*ast.Ident)
	assert.True(t, ok)
	assert.Equal(t, "set", y.Name)
}

func TestInPrecedence(t *testing.T) {
	input := `2 in sorted([1,2,3])`
	program, err := Parse(context.Background(), input, nil)
	assert.Nil(t, err)
	assert.Len(t, program.Stmts, 1)

	inStmt, ok := program.First().(*ast.In)
	assert.True(t, ok)
	assert.Equal(t, "2", inStmt.X.String())
	assert.Equal(t, "sorted([1, 2, 3])", inStmt.Y.String())
}

func TestNotInPrecedence(t *testing.T) {
	input := `2 not in sorted([1,2,3])`
	program, err := Parse(context.Background(), input, nil)
	assert.Nil(t, err)
	assert.Len(t, program.Stmts, 1)

	notInStmt, ok := program.First().(*ast.NotIn)
	assert.True(t, ok)
	assert.Equal(t, "2", notInStmt.X.String())
	assert.Equal(t, "sorted([1, 2, 3])", notInStmt.Y.String())
}

func TestInNotInWithArithmetic(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{`1 in 2 + 3`, "1 in (2 + 3)"},
		{`1 + 2 in [1, 2]`, "(1 + 2) in [1, 2]"},
		{`1 not in 2 + 3`, "1 not in (2 + 3)"},
		{`1 + 2 not in [1, 2]`, "(1 + 2) not in [1, 2]"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			program, err := Parse(context.Background(), tt.input, nil)
			assert.Nil(t, err)
			assert.Len(t, program.Stmts, 1)
			assert.Equal(t, tt.expected, program.First().String())
		})
	}
}

func TestGetAttr(t *testing.T) {
	program, err := Parse(context.Background(), "foo.bar", nil)
	assert.Nil(t, err)
	assert.Len(t, program.Stmts, 1)

	getAttr, ok := program.First().(*ast.GetAttr)
	assert.True(t, ok)
	assert.Equal(t, "bar", getAttr.Attr.Name)
	assert.Equal(t, "foo.bar", getAttr.String())
}

func TestGetAttrAST(t *testing.T) {
	program, err := Parse(context.Background(), "obj.field", nil)
	assert.Nil(t, err)

	getAttr, ok := program.First().(*ast.GetAttr)
	assert.True(t, ok)

	// Verify AST node fields
	obj, ok := getAttr.X.(*ast.Ident)
	assert.True(t, ok)
	assert.Equal(t, "obj", obj.Name)

	assert.Equal(t, "field", getAttr.Attr.Name)
}

func TestChainedAttributeAccess(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{`a.b.c`, `a.b.c`},
		{`obj.inner.value`, `obj.inner.value`},
		{`a.b.c.d.e`, `a.b.c.d.e`},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			program, err := Parse(context.Background(), tt.input, nil)
			assert.Nil(t, err)
			assert.Len(t, program.Stmts, 1)
			assert.Equal(t, tt.expected, program.First().String())
		})
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
		t.Run(tt.input, func(t *testing.T) {
			result, err := Parse(context.Background(), tt.input, nil)
			assert.Nil(t, err, "input: %s", tt.input)
			assert.Equal(t, tt.expected, result.String(), "input: %s", tt.input)
		})
	}
}

func TestOptionalChainingAST(t *testing.T) {
	program, err := Parse(context.Background(), "obj?.field", nil)
	assert.Nil(t, err)

	// Optional chaining is represented by GetAttr with Optional=true
	getAttr, ok := program.First().(*ast.GetAttr)
	assert.True(t, ok, "expected GetAttr node")

	// Verify AST node fields
	assert.True(t, getAttr.Optional, "expected Optional=true for ?.operator")
	assert.Equal(t, "field", getAttr.Attr.Name)

	obj, ok := getAttr.X.(*ast.Ident)
	assert.True(t, ok, "expected Ident for object")
	assert.Equal(t, "obj", obj.Name)
}

func TestMethodChaining(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{`obj.method1().method2()`, `obj.method1().method2()`},
		{`"hello".upper().trim()`, `"hello".upper().trim()`},
		{`list.filter(f).map(g)`, `list.filter(f).map(g)`},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			program, err := Parse(context.Background(), tt.input, nil)
			assert.Nil(t, err)
			assert.Len(t, program.Stmts, 1)
			assert.Equal(t, tt.expected, program.First().String())
		})
	}
}

func TestObjectMethodCall(t *testing.T) {
	inputs := []string{
		`"steve".len()`,
		"let x = 15; x.string();",
	}
	for _, input := range inputs {
		_, err := Parse(context.Background(), input, nil)
		assert.Nil(t, err)
	}
}

func TestObjectCallAST(t *testing.T) {
	program, err := Parse(context.Background(), "obj.method(arg)", nil)
	assert.Nil(t, err)

	objCall, ok := program.First().(*ast.ObjectCall)
	assert.True(t, ok)

	// Verify AST node fields
	obj, ok := objCall.X.(*ast.Ident)
	assert.True(t, ok)
	assert.Equal(t, "obj", obj.Name)

	assert.NotNil(t, objCall.Call)
	assert.Equal(t, "method", objCall.Call.Fun.String())
	assert.Len(t, objCall.Call.Args, 1)
}

func TestNullishCoalescing(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{`a ?? b`, `(a ?? b)`},
		{`a ?? b ?? c`, `((a ?? b) ?? c)`},
		{`obj.value ?? "default"`, `(obj.value ?? "default")`},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			program, err := Parse(context.Background(), tt.input, nil)
			assert.Nil(t, err)
			assert.Len(t, program.Stmts, 1)
			assert.Equal(t, tt.expected, program.First().String())
		})
	}
}

func TestNullishCoalescingAST(t *testing.T) {
	program, err := Parse(context.Background(), "x ?? y", nil)
	assert.Nil(t, err)

	infix, ok := program.First().(*ast.Infix)
	assert.True(t, ok)

	// Verify AST node fields
	assert.Equal(t, "??", infix.Op)

	x, ok := infix.X.(*ast.Ident)
	assert.True(t, ok)
	assert.Equal(t, "x", x.Name)

	y, ok := infix.Y.(*ast.Ident)
	assert.True(t, ok)
	assert.Equal(t, "y", y.Name)
}

func TestBitShiftOperators(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{`1 << 2`, `(1 << 2)`},
		{`8 >> 1`, `(8 >> 1)`},
		{`a << b >> c`, `((a << b) >> c)`},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			program, err := Parse(context.Background(), tt.input, nil)
			assert.Nil(t, err)
			assert.Len(t, program.Stmts, 1)
			assert.Equal(t, tt.expected, program.First().String())
		})
	}
}

func TestBitwiseAnd(t *testing.T) {
	input := "1 & 2"
	result, err := Parse(context.Background(), input, nil)
	assert.Nil(t, err)
	assert.Equal(t, "(1 & 2)", result.String())
}

func TestComparisonOperators(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{`a >= b`, `(a >= b)`},
		{`a <= b`, `(a <= b)`},
		{`a > b`, `(a > b)`},
		{`a < b`, `(a < b)`},
		{`a == b`, `(a == b)`},
		{`a != b`, `(a != b)`},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			program, err := Parse(context.Background(), tt.input, nil)
			assert.Nil(t, err)
			assert.Len(t, program.Stmts, 1)
			assert.Equal(t, tt.expected, program.First().String())
		})
	}
}

func TestModuloOperator(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{`5 % 2`, `(5 % 2)`},
		{`a % b % c`, `((a % b) % c)`},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			program, err := Parse(context.Background(), tt.input, nil)
			assert.Nil(t, err)
			assert.Len(t, program.Stmts, 1)
			assert.Equal(t, tt.expected, program.First().String())
		})
	}
}

func TestArrowFunction(t *testing.T) {
	tests := []struct {
		input         string
		expectedParam []string
		bodyType      string
	}{
		{"() => 42", []string{}, "return"},
		{"() => { return 42 }", []string{}, "block"},
		{"(x) => x", []string{"x"}, "return"},
		{"(x) => { return x }", []string{"x"}, "block"},
		{"(x, y) => x + y", []string{"x", "y"}, "return"},
		{"(a, b, c) => a", []string{"a", "b", "c"}, "return"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			program, err := Parse(context.Background(), tt.input, nil)
			assert.Nil(t, err, "parse error for %q", tt.input)
			assert.Len(t, program.Stmts, 1)

			function, ok := program.First().(*ast.Func)
			assert.True(t, ok, "expected Func, got %T", program.First())
			assert.Nil(t, function.Name, "arrow functions should not have names")

			params := function.Params
			assert.Len(t, params, len(tt.expectedParam))
			for i, expectedName := range tt.expectedParam {
				paramIdent, ok := params[i].(*ast.Ident)
				assert.True(t, ok, "Expected *ast.Ident")
				assert.Equal(t, expectedName, paramIdent.Name)
			}
		})
	}
}

func TestArrowFunctionWithDefaults(t *testing.T) {
	program, err := Parse(context.Background(), "(x, y = 5) => x + y", nil)
	assert.Nil(t, err)
	assert.Len(t, program.Stmts, 1)

	function, ok := program.First().(*ast.Func)
	assert.True(t, ok)
	assert.Len(t, function.Params, 2)
	assert.Len(t, function.Defaults, 1)
	assert.Contains(t, function.Defaults, "y")
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
			program, err := Parse(context.Background(), tt.input, nil)
			assert.Nil(t, err, "parse error for %q", tt.input)
			assert.Len(t, program.Stmts, 1)

			function, ok := program.First().(*ast.Func)
			assert.True(t, ok, "expected Func, got %T", program.First())
			assert.Nil(t, function.Name)
			assert.Len(t, function.Params, 1)
			paramIdent, ok := function.Params[0].(*ast.Ident)
			assert.True(t, ok, "Expected *ast.Ident")
			assert.Equal(t, tt.expectedParam, paramIdent.Name)
		})
	}
}

func TestArrowFunctionErrors(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"() =>", "parse error: invalid arrow function body"},
		{"(1, 2) => x", "parse error: invalid arrow function parameter: expected identifier or destructuring pattern"},
		{"(x + 1) => x", "parse error: invalid arrow function parameter: expected identifier or destructuring pattern"},
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

func TestArrowFunctionDestructureParams(t *testing.T) {
	t.Run("object destructure shorthand", func(t *testing.T) {
		program, err := Parse(context.Background(), "({a, b}) => a + b", nil)
		assert.Nil(t, err)

		fn, ok := program.First().(*ast.Func)
		assert.True(t, ok)
		assert.Len(t, fn.Params, 1)

		dp, ok := fn.Params[0].(*ast.ObjectDestructureParam)
		assert.True(t, ok)
		assert.Len(t, dp.Bindings, 2)
		assert.Equal(t, dp.Bindings[0].Key, "a")
		assert.Equal(t, dp.Bindings[1].Key, "b")
	})

	t.Run("object destructure with alias", func(t *testing.T) {
		program, err := Parse(context.Background(), "({name: n, value: v}) => n", nil)
		assert.Nil(t, err)

		fn, ok := program.First().(*ast.Func)
		assert.True(t, ok)
		assert.Len(t, fn.Params, 1)

		dp, ok := fn.Params[0].(*ast.ObjectDestructureParam)
		assert.True(t, ok)
		assert.Len(t, dp.Bindings, 2)
		assert.Equal(t, dp.Bindings[0].Key, "name")
		assert.Equal(t, dp.Bindings[0].Alias, "n")
		assert.Equal(t, dp.Bindings[1].Key, "value")
		assert.Equal(t, dp.Bindings[1].Alias, "v")
	})

	t.Run("object destructure with default", func(t *testing.T) {
		program, err := Parse(context.Background(), "({a = 10}) => a", nil)
		assert.Nil(t, err)

		fn, ok := program.First().(*ast.Func)
		assert.True(t, ok)
		assert.Len(t, fn.Params, 1)

		dp, ok := fn.Params[0].(*ast.ObjectDestructureParam)
		assert.True(t, ok)
		assert.Len(t, dp.Bindings, 1)
		assert.Equal(t, dp.Bindings[0].Key, "a")
		assert.NotNil(t, dp.Bindings[0].Default)

		defaultInt, ok := dp.Bindings[0].Default.(*ast.Int)
		assert.True(t, ok)
		assert.Equal(t, defaultInt.Value, int64(10))
	})

	t.Run("object destructure with multiple defaults", func(t *testing.T) {
		program, err := Parse(context.Background(), "({a = 1, b = 2, c = 3}) => a + b + c", nil)
		assert.Nil(t, err)

		fn, ok := program.First().(*ast.Func)
		assert.True(t, ok)

		dp, ok := fn.Params[0].(*ast.ObjectDestructureParam)
		assert.True(t, ok)
		assert.Len(t, dp.Bindings, 3)

		for i, key := range []string{"a", "b", "c"} {
			assert.Equal(t, dp.Bindings[i].Key, key)
			assert.NotNil(t, dp.Bindings[i].Default)
		}
	})

	t.Run("object destructure mixed shorthand and default", func(t *testing.T) {
		program, err := Parse(context.Background(), "({x, y = 10, z}) => x + y + z", nil)
		assert.Nil(t, err)

		fn, ok := program.First().(*ast.Func)
		assert.True(t, ok)

		dp, ok := fn.Params[0].(*ast.ObjectDestructureParam)
		assert.True(t, ok)
		assert.Len(t, dp.Bindings, 3)

		// x - no default
		assert.Equal(t, dp.Bindings[0].Key, "x")
		assert.Nil(t, dp.Bindings[0].Default)

		// y = 10 - has default
		assert.Equal(t, dp.Bindings[1].Key, "y")
		assert.NotNil(t, dp.Bindings[1].Default)

		// z - no default
		assert.Equal(t, dp.Bindings[2].Key, "z")
		assert.Nil(t, dp.Bindings[2].Default)
	})

	t.Run("array destructure", func(t *testing.T) {
		program, err := Parse(context.Background(), "([a, b]) => a + b", nil)
		assert.Nil(t, err)

		fn, ok := program.First().(*ast.Func)
		assert.True(t, ok)
		assert.Len(t, fn.Params, 1)

		dp, ok := fn.Params[0].(*ast.ArrayDestructureParam)
		assert.True(t, ok)
		assert.Len(t, dp.Elements, 2)
		assert.Equal(t, dp.Elements[0].Name.Name, "a")
		assert.Equal(t, dp.Elements[1].Name.Name, "b")
	})

	t.Run("mixed regular and destructure params", func(t *testing.T) {
		program, err := Parse(context.Background(), "(x, {a, b}, [c, d], y) => x", nil)
		assert.Nil(t, err)

		fn, ok := program.First().(*ast.Func)
		assert.True(t, ok)
		assert.Len(t, fn.Params, 4)

		// x - regular ident
		_, ok = fn.Params[0].(*ast.Ident)
		assert.True(t, ok)

		// {a, b} - object destructure
		_, ok = fn.Params[1].(*ast.ObjectDestructureParam)
		assert.True(t, ok)

		// [c, d] - array destructure
		_, ok = fn.Params[2].(*ast.ArrayDestructureParam)
		assert.True(t, ok)

		// y - regular ident
		_, ok = fn.Params[3].(*ast.Ident)
		assert.True(t, ok)
	})

	t.Run("array destructure with default", func(t *testing.T) {
		program, err := Parse(context.Background(), "([a = 1]) => a", nil)
		assert.Nil(t, err)

		fn, ok := program.First().(*ast.Func)
		assert.True(t, ok)
		assert.Len(t, fn.Params, 1)

		dp, ok := fn.Params[0].(*ast.ArrayDestructureParam)
		assert.True(t, ok)
		assert.Len(t, dp.Elements, 1)
		assert.Equal(t, "a", dp.Elements[0].Name.Name)
		assert.NotNil(t, dp.Elements[0].Default)

		defaultInt, ok := dp.Elements[0].Default.(*ast.Int)
		assert.True(t, ok)
		assert.Equal(t, int64(1), defaultInt.Value)
	})

	t.Run("array destructure with multiple defaults", func(t *testing.T) {
		program, err := Parse(context.Background(), "([a = 1, b = 2, c]) => a + b + c", nil)
		assert.Nil(t, err)

		fn, ok := program.First().(*ast.Func)
		assert.True(t, ok)
		assert.Len(t, fn.Params, 1)

		dp, ok := fn.Params[0].(*ast.ArrayDestructureParam)
		assert.True(t, ok)
		assert.Len(t, dp.Elements, 3)

		// a = 1
		assert.Equal(t, "a", dp.Elements[0].Name.Name)
		assert.NotNil(t, dp.Elements[0].Default)

		// b = 2
		assert.Equal(t, "b", dp.Elements[1].Name.Name)
		assert.NotNil(t, dp.Elements[1].Default)

		// c (no default)
		assert.Equal(t, "c", dp.Elements[2].Name.Name)
		assert.Nil(t, dp.Elements[2].Default)
	})

	t.Run("array destructure with complex default", func(t *testing.T) {
		program, err := Parse(context.Background(), "([a = x + 1]) => a", nil)
		assert.Nil(t, err)

		fn, ok := program.First().(*ast.Func)
		assert.True(t, ok)

		dp, ok := fn.Params[0].(*ast.ArrayDestructureParam)
		assert.True(t, ok)

		// Default should be an Infix expression
		_, ok = dp.Elements[0].Default.(*ast.Infix)
		assert.True(t, ok, "Default should be an infix expression")
	})
}

func TestMatchExpression(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{
			`match x { 1 => "one", _ => "other" }`,
			`match x { 1 => "one", _ => "other" }`,
		},
		{
			`match x { 1 => "one", 2 => "two", _ => "other" }`,
			`match x { 1 => "one", 2 => "two", _ => "other" }`,
		},
		{
			`match "hello" { "hi" => 1, "hello" => 2, _ => 0 }`,
			`match "hello" { "hi" => 1, "hello" => 2, _ => 0 }`,
		},
		{
			`match true { true => "yes", false => "no", _ => "unknown" }`,
			`match true { true => "yes", false => "no", _ => "unknown" }`,
		},
		{
			`match nil { nil => "nothing", _ => "something" }`,
			`match nil { nil => "nothing", _ => "something" }`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			program, err := Parse(context.Background(), tt.input, nil)
			assert.Nil(t, err)
			assert.Len(t, program.Stmts, 1)
			assert.Equal(t, tt.expected, program.First().String())
		})
	}
}

func TestMatchExpressionAST(t *testing.T) {
	program, err := Parse(context.Background(), `match x { 1 => "one", 2 => "two", _ => "other" }`, nil)
	assert.Nil(t, err)
	assert.Len(t, program.Stmts, 1)

	matchExpr, ok := program.First().(*ast.Match)
	assert.True(t, ok)

	// Verify subject
	subject, ok := matchExpr.Subject.(*ast.Ident)
	assert.True(t, ok)
	assert.Equal(t, "x", subject.Name)

	// Verify arms
	assert.Len(t, matchExpr.Arms, 2)

	// First arm: 1 => "one"
	arm1 := matchExpr.Arms[0]
	lit1, ok := arm1.Pattern.(*ast.LiteralPattern)
	assert.True(t, ok)
	int1, ok := lit1.Value.(*ast.Int)
	assert.True(t, ok)
	assert.Equal(t, int64(1), int1.Value)
	result1, ok := arm1.Result.(*ast.String)
	assert.True(t, ok)
	assert.Equal(t, "one", result1.Value)

	// Second arm: 2 => "two"
	arm2 := matchExpr.Arms[1]
	lit2, ok := arm2.Pattern.(*ast.LiteralPattern)
	assert.True(t, ok)
	int2, ok := lit2.Value.(*ast.Int)
	assert.True(t, ok)
	assert.Equal(t, int64(2), int2.Value)

	// Default arm: _ => "other"
	assert.NotNil(t, matchExpr.Default)
	_, ok = matchExpr.Default.Pattern.(*ast.WildcardPattern)
	assert.True(t, ok)
	defaultResult, ok := matchExpr.Default.Result.(*ast.String)
	assert.True(t, ok)
	assert.Equal(t, "other", defaultResult.Value)
}

func TestMatchExpressionWithNewlines(t *testing.T) {
	input := `match x {
		1 => "one"
		2 => "two"
		_ => "other"
	}`
	program, err := Parse(context.Background(), input, nil)
	assert.Nil(t, err)
	assert.Len(t, program.Stmts, 1)

	matchExpr, ok := program.First().(*ast.Match)
	assert.True(t, ok)
	assert.Len(t, matchExpr.Arms, 2)
	assert.NotNil(t, matchExpr.Default)
}

func TestMatchExpressionErrors(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{
			`match x { 1 => "one" }`,
			"parse error: match expression requires a default arm (_)",
		},
		{
			`match x { _ => "a", _ => "b" }`,
			"parse error: match expression has multiple default arms",
		},
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

func TestMatchExpressionPatterns(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		// Variable patterns
		{
			`match x { y => "matched y", _ => "other" }`,
			`match x { y => "matched y", _ => "other" }`,
		},
		// Expression patterns
		{
			`match x { 1 + 1 => "two", _ => "other" }`,
			`match x { (1 + 1) => "two", _ => "other" }`,
		},
		// Function call patterns
		{
			`match x { get_value() => "matched", _ => "other" }`,
			`match x { get_value() => "matched", _ => "other" }`,
		},
		// Parenthesized patterns
		{
			`match x { (a) => "matched a", _ => "other" }`,
			`match x { a => "matched a", _ => "other" }`,
		},
		// Attribute access patterns
		{
			`match x { obj.field => "matched", _ => "other" }`,
			`match x { obj.field => "matched", _ => "other" }`,
		},
		// Index patterns
		{
			`match x { arr[0] => "matched", _ => "other" }`,
			`match x { arr[0] => "matched", _ => "other" }`,
		},
		// Logical operators in patterns
		{
			`match x { (a && b) => "both", _ => "other" }`,
			`match x { (a && b) => "both", _ => "other" }`,
		},
		// Comparison in pattern
		{
			`match x { (1 < 2) => "less", _ => "other" }`,
			`match x { (1 < 2) => "less", _ => "other" }`,
		},
		// In operator pattern
		{
			`match x { (1 in arr) => "found", _ => "other" }`,
			`match x { 1 in arr => "found", _ => "other" }`,
		},
		// Nullish coalescing pattern
		{
			`match x { (a ?? b) => "matched", _ => "other" }`,
			`match x { (a ?? b) => "matched", _ => "other" }`,
		},
		// Optional chaining pattern
		{
			`match x { obj?.field => "matched", _ => "other" }`,
			`match x { obj?.field => "matched", _ => "other" }`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			program, err := Parse(context.Background(), tt.input, nil)
			assert.Nil(t, err)
			assert.Len(t, program.Stmts, 1)
			assert.Equal(t, tt.expected, program.First().String())
		})
	}
}

func TestMatchNested(t *testing.T) {
	input := `match x {
		1 => match y {
			2 => "x=1,y=2"
			_ => "x=1,y=other"
		}
		_ => "x=other"
	}`
	program, err := Parse(context.Background(), input, nil)
	assert.Nil(t, err)
	assert.Len(t, program.Stmts, 1)

	outer, ok := program.First().(*ast.Match)
	assert.True(t, ok)
	assert.Len(t, outer.Arms, 1)

	// First arm's result should be a nested match
	inner, ok := outer.Arms[0].Result.(*ast.Match)
	assert.True(t, ok)
	assert.Len(t, inner.Arms, 1)
}

func TestMatchInArrowFunction(t *testing.T) {
	input := `(x) => match x { 1 => "one", _ => "other" }`
	program, err := Parse(context.Background(), input, nil)
	assert.Nil(t, err)
	assert.Len(t, program.Stmts, 1)

	fn, ok := program.First().(*ast.Func)
	assert.True(t, ok)
	assert.Len(t, fn.Params, 1)

	// Arrow function expression body is wrapped in a return statement
	assert.Len(t, fn.Body.Stmts, 1)
	ret, ok := fn.Body.Stmts[0].(*ast.Return)
	assert.True(t, ok)

	// The return value should be the match expression
	_, ok = ret.Value.(*ast.Match)
	assert.True(t, ok)
}

func TestMatchAsSubject(t *testing.T) {
	input := `match (match x { 1 => 2, _ => 0 }) { 2 => "two", _ => "other" }`
	program, err := Parse(context.Background(), input, nil)
	assert.Nil(t, err)
	assert.Len(t, program.Stmts, 1)

	outer, ok := program.First().(*ast.Match)
	assert.True(t, ok)

	// Subject should be a match expression
	_, ok = outer.Subject.(*ast.Match)
	assert.True(t, ok)
}

func TestMatchSpreadNotAllowed(t *testing.T) {
	_, err := Parse(context.Background(), `match x { ...arr => "matched", _ => "other" }`, nil)
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "spread operator not supported in match patterns")
}
