package parser

import (
	"testing"

	"github.com/deepnoodle-ai/risor/v2/pkg/ast"
)

func testVarStatement(t *testing.T, s *ast.Var, name string) bool {
	t.Helper()
	if s.Name.Name != name {
		t.Errorf("s.Name not '%s'. got=%s", name, s.Name.Name)
		return false
	}
	return true
}

func testConstStatement(t *testing.T, s *ast.Const, name string) bool {
	t.Helper()
	if s.Name.Name != name {
		t.Errorf("s.Name not '%s'. got=%s", name, s.Name.Name)
		return false
	}
	return true
}

func testIntegerLiteral(t *testing.T, il ast.Expr, value int64) bool {
	t.Helper()
	integ, ok := il.(*ast.Int)
	if !ok {
		t.Errorf("il not *ast.Int. got=%T", il)
		return false
	}
	if integ.Value != value {
		t.Errorf("integ.Value not %d. got=%d", value, integ.Value)
		return false
	}
	return true
}

// skip float literal test
func testFloatLiteral(t *testing.T, exp ast.Expr, v float64) bool {
	t.Helper()
	float, ok := exp.(*ast.Float)
	if !ok {
		t.Errorf("exp not *ast.Float. got=%T", exp)
		return false
	}
	if float.Value != v {
		t.Errorf("float.Value not %f. got=%f", v, float.Value)
		return false
	}
	return true
}

func testIdentifier(t *testing.T, exp ast.Expr, value string) bool {
	t.Helper()
	ident, ok := exp.(*ast.Ident)
	if !ok {
		t.Errorf("exp not *ast.Ident. got=%T", exp)
		return false
	}
	if ident.Name != value {
		t.Errorf("ident.Name not %s. got=%s", value, ident.Name)
		return false
	}
	return true
}

func testBooleanLiteral(t *testing.T, exp ast.Expr, value bool) bool {
	t.Helper()
	bo, ok := exp.(*ast.Bool)
	if !ok {
		t.Errorf("exp not *ast.Bool. got=%T", exp)
		return false
	}
	if bo.Value != value {
		t.Errorf("bo.Value not %t, got=%t", value, bo.Value)
		return false
	}
	return true
}

func testLiteralExpression(t *testing.T, exp ast.Expr, expected interface{}) bool {
	t.Helper()
	switch v := expected.(type) {
	case int:
		return testIntegerLiteral(t, exp, int64(v))
	case int64:
		return testIntegerLiteral(t, exp, v)
	case string:
		return testIdentifier(t, exp, v)
	case bool:
		return testBooleanLiteral(t, exp, v)
	case float32:
		return testFloatLiteral(t, exp, float64(v))
	case float64:
		return testFloatLiteral(t, exp, v)
	}
	t.Errorf("type of exp not handled. got=%T", exp)
	return false
}

func testInfixExpression(t *testing.T, exp ast.Expr, left interface{},
	operator string, right interface{},
) bool {
	t.Helper()
	opExp, ok := exp.(*ast.Infix)
	if !ok {
		t.Errorf("exp is not ast.Infix. got=%T(%s)", exp, exp)
		return false
	}
	if !testLiteralExpression(t, opExp.X, left) {
		return false
	}
	if opExp.Op != operator {
		t.Errorf("exp.Operator is not '%s'. got=%q", operator, opExp.Op)
		return false
	}
	if !testLiteralExpression(t, opExp.Y, right) {
		return false
	}
	return true
}
