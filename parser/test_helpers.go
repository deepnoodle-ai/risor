package parser

import (
	"fmt"
	"testing"

	"github.com/risor-io/risor/ast"
)

func testVarStatement(t *testing.T, s *ast.Var, name string) bool {
	t.Helper()
	if s.Literal() != "let" {
		t.Errorf("s.Literal not 'let'. got %q", s.Literal())
		return false
	}
	varName, _ := s.Value()
	if varName != name {
		t.Errorf("s.Name not '%s'. got=%s", name, varName)
		return false
	}
	return true
}

func testConstStatement(t *testing.T, s *ast.Const, name string) bool {
	t.Helper()
	if s.Literal() != "const" {
		t.Errorf("s.Literal not 'const'. got %q", s.Literal())
		return false
	}
	constName, _ := s.Value()
	if constName != name {
		t.Errorf("s.Name not '%s'. got=%s", name, constName)
		return false
	}
	return true
}

func testIntegerLiteral(t *testing.T, il ast.Expression, value int64) bool {
	t.Helper()
	integ, ok := il.(*ast.Int)
	if !ok {
		t.Errorf("il not *ast.Int. got=%T", il)
		return false
	}
	if integ.Value() != value {
		t.Errorf("integ.Value not %d. got=%d", value, integ.Value())
		return false
	}
	if integ.Literal() != fmt.Sprintf("%d", value) {
		t.Errorf("integ.Literal not %d. got=%s", value, integ.Literal())
		return false
	}
	return true
}

// skip float literal test
func testFloatLiteral(t *testing.T, exp ast.Expression, v float64) bool {
	t.Helper()
	float, ok := exp.(*ast.Float)
	if !ok {
		t.Errorf("exp not *ast.Float. got=%T", exp)
		return false
	}
	if float.Value() != v {
		t.Errorf("float.Value not %f. got=%f", v, float.Value())
		return false
	}
	return true
}

func testIdentifier(t *testing.T, exp ast.Expression, value string) bool {
	t.Helper()
	ident, ok := exp.(*ast.Ident)
	if !ok {
		t.Errorf("exp not *ast.Ident. got=%T", exp)
		return false
	}
	if ident.String() != value {
		t.Errorf("ident.Value not %s. got=%s", value, ident.String())
		return false
	}
	if ident.Literal() != value {
		t.Errorf("ident.Literal not %s. got=%s", value, ident.Literal())
		return false
	}
	return true
}

func testBooleanLiteral(t *testing.T, exp ast.Expression, value bool) bool {
	t.Helper()
	bo, ok := exp.(*ast.Bool)
	if !ok {
		t.Errorf("exp not *ast.Bool. got=%T", exp)
		return false
	}
	if bo.Value() != value {
		t.Errorf("bo.Value not %t, got=%t", value, bo.Value())
		return false
	}
	if bo.Literal() != fmt.Sprintf("%t", value) {
		t.Errorf("bo.Literal not %t, got=%s",
			value, bo.Literal())
		return false
	}
	return true
}

func testLiteralExpression(t *testing.T, exp ast.Expression, expected interface{}) bool {
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

func testInfixExpression(t *testing.T, exp ast.Expression, left interface{},
	operator string, right interface{},
) bool {
	t.Helper()
	opExp, ok := exp.(*ast.Infix)
	if !ok {
		t.Errorf("exp is not ast.Infix. got=%T(%s)", exp, exp)
		return false
	}
	if !testLiteralExpression(t, opExp.Left(), left) {
		return false
	}
	if opExp.Operator() != operator {
		t.Errorf("exp.Operator is not '%s'. got=%q", operator, opExp.Operator())
		return false
	}
	if !testLiteralExpression(t, opExp.Right(), right) {
		return false
	}
	return true
}
