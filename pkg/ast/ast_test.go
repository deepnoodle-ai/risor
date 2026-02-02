package ast

import (
	"testing"

	"github.com/deepnoodle-ai/wonton/assert"
	"github.com/deepnoodle-ai/risor/v2/internal/token"
)

func TestString(t *testing.T) {
	program := &Program{
		Stmts: []Node{
			&Var{
				Let: token.Position{Line: 1, Column: 1},
				Name: &Ident{
					NamePos: token.Position{Line: 1, Column: 5},
					Name:    "myVar",
				},
				Value: &Ident{
					NamePos: token.Position{Line: 1, Column: 13},
					Name:    "anotherVar",
				},
			},
		},
	}
	assert.Equal(t, program.String(), "let myVar = anotherVar")
}

func TestBadExpr(t *testing.T) {
	from := token.Position{Line: 1, Column: 5, File: "test.risor"}
	to := token.Position{Line: 1, Column: 15, File: "test.risor"}

	bad := &BadExpr{From: from, To: to}

	// Test Pos() returns From
	assert.Equal(t, bad.Pos(), from)

	// Test End() returns To
	assert.Equal(t, bad.End(), to)

	// Test String() returns placeholder
	assert.Equal(t, bad.String(), "<bad expression>")

	// Test that BadExpr implements Expr interface
	var _ Expr = bad
}

func TestBadStmt(t *testing.T) {
	from := token.Position{Line: 2, Column: 1, File: "test.risor"}
	to := token.Position{Line: 2, Column: 20, File: "test.risor"}

	bad := &BadStmt{From: from, To: to}

	// Test Pos() returns From
	assert.Equal(t, bad.Pos(), from)

	// Test End() returns To
	assert.Equal(t, bad.End(), to)

	// Test String() returns placeholder
	assert.Equal(t, bad.String(), "<bad statement>")

	// Test that BadStmt implements Stmt interface
	var _ Stmt = bad
}

func TestBadExprInProgram(t *testing.T) {
	// Test that BadExpr can be used as a value in a Var statement
	program := &Program{
		Stmts: []Node{
			&Var{
				Let: token.Position{Line: 1, Column: 1},
				Name: &Ident{
					NamePos: token.Position{Line: 1, Column: 5},
					Name:    "x",
				},
				Value: &BadExpr{
					From: token.Position{Line: 1, Column: 9},
					To:   token.Position{Line: 1, Column: 15},
				},
			},
		},
	}

	// Verify the program can be stringified
	str := program.String()
	assert.True(t, str != "", "Program with BadExpr should stringify")
}

func TestBadStmtInProgram(t *testing.T) {
	// Test that BadStmt can be included in a program
	program := &Program{
		Stmts: []Node{
			&BadStmt{
				From: token.Position{Line: 1, Column: 1},
				To:   token.Position{Line: 1, Column: 10},
			},
			&Var{
				Let: token.Position{Line: 2, Column: 1},
				Name: &Ident{
					NamePos: token.Position{Line: 2, Column: 5},
					Name:    "x",
				},
				Value: &Int{
					ValuePos: token.Position{Line: 2, Column: 9},
					Value:    42,
				},
			},
		},
	}

	// Verify the program has both statements
	assert.Len(t, program.Stmts, 2)

	// Verify BadStmt is first
	_, ok := program.Stmts[0].(*BadStmt)
	assert.True(t, ok, "First statement should be BadStmt")
}

// Program tests
func TestProgramEmpty(t *testing.T) {
	program := &Program{Stmts: []Node{}}

	assert.Equal(t, program.Pos(), token.NoPos)
	assert.Equal(t, program.End(), token.NoPos)
	assert.Nil(t, program.First())
	assert.Equal(t, program.String(), "")
}

func TestProgramFirst(t *testing.T) {
	stmt := &Int{ValuePos: token.Position{Line: 1, Column: 1}, Value: 42}
	program := &Program{Stmts: []Node{stmt}}

	assert.Equal(t, program.First(), stmt)
}

// Expression tests
func TestIdent(t *testing.T) {
	ident := &Ident{
		NamePos: token.Position{Line: 1, Column: 1},
		Name:    "foo",
	}

	assert.Equal(t, ident.Pos().Column, 1)
	assert.Equal(t, ident.End().Column, 4) // 1 + len("foo")
	assert.Equal(t, ident.String(), "foo")
}

func TestPrefix(t *testing.T) {
	prefix := &Prefix{
		OpPos: token.Position{Line: 1, Column: 1},
		Op:    "!",
		X: &Bool{
			ValuePos: token.Position{Line: 1, Column: 2},
			Literal:  "true",
			Value:    true,
		},
	}

	assert.Equal(t, prefix.Pos().Column, 1)
	assert.Equal(t, prefix.String(), "(!true)")
}

func TestSpread(t *testing.T) {
	// With expression
	spread := &Spread{
		Ellipsis: token.Position{Line: 1, Column: 1},
		X: &Ident{
			NamePos: token.Position{Line: 1, Column: 4},
			Name:    "arr",
		},
	}

	assert.Equal(t, spread.Pos().Column, 1)
	assert.Equal(t, spread.String(), "...arr")

	// Without expression (rest parameter)
	restSpread := &Spread{
		Ellipsis: token.Position{Line: 1, Column: 1},
		X:        nil,
	}
	assert.Equal(t, restSpread.String(), "...")
	assert.Equal(t, restSpread.End().Column, 4) // 1 + len("...")
}

func TestInfix(t *testing.T) {
	infix := &Infix{
		X: &Int{
			ValuePos: token.Position{Line: 1, Column: 1},
			Literal:  "1",
			Value:    1,
		},
		OpPos: token.Position{Line: 1, Column: 3},
		Op:    "+",
		Y: &Int{
			ValuePos: token.Position{Line: 1, Column: 5},
			Literal:  "2",
			Value:    2,
		},
	}

	assert.Equal(t, infix.Pos().Column, 1)
	assert.Equal(t, infix.String(), "(1 + 2)")
}

func TestIfExpression(t *testing.T) {
	ifExpr := &If{
		If:     token.Position{Line: 1, Column: 1},
		Lparen: token.Position{Line: 1, Column: 4},
		Cond: &Bool{
			ValuePos: token.Position{Line: 1, Column: 5},
			Literal:  "true",
			Value:    true,
		},
		Rparen: token.Position{Line: 1, Column: 9},
		Consequence: &Block{
			Lbrace: token.Position{Line: 1, Column: 11},
			Stmts:  []Node{},
			Rbrace: token.Position{Line: 1, Column: 13},
		},
	}

	assert.Equal(t, ifExpr.Pos().Column, 1)
	assert.Equal(t, ifExpr.End().Column, 14) // Consequence end

	// The exact string format depends on the implementation
	s := ifExpr.String()
	assert.True(t, s != "", "If.String() should not be empty")

	// With alternative
	ifExpr.Alternative = &Block{
		Lbrace: token.Position{Line: 1, Column: 20},
		Stmts:  []Node{},
		Rbrace: token.Position{Line: 1, Column: 22},
	}
	assert.Equal(t, ifExpr.End().Column, 23)
}

func TestCall(t *testing.T) {
	call := &Call{
		Fun: &Ident{
			NamePos: token.Position{Line: 1, Column: 1},
			Name:    "foo",
		},
		Lparen: token.Position{Line: 1, Column: 4},
		Args: []Node{
			&Int{ValuePos: token.Position{Line: 1, Column: 5}, Literal: "1", Value: 1},
			&Int{ValuePos: token.Position{Line: 1, Column: 8}, Literal: "2", Value: 2},
		},
		Rparen: token.Position{Line: 1, Column: 9},
	}

	assert.Equal(t, call.Pos().Column, 1)
	assert.Equal(t, call.End().Column, 10)
	assert.Equal(t, call.String(), "foo(1, 2)")
}

func TestGetAttr(t *testing.T) {
	// Non-optional
	getAttr := &GetAttr{
		X: &Ident{
			NamePos: token.Position{Line: 1, Column: 1},
			Name:    "obj",
		},
		Period: token.Position{Line: 1, Column: 4},
		Attr: &Ident{
			NamePos: token.Position{Line: 1, Column: 5},
			Name:    "foo",
		},
		Optional: false,
	}

	assert.Equal(t, getAttr.String(), "obj.foo")

	// Optional chaining
	getAttr.Optional = true
	assert.Equal(t, getAttr.String(), "obj?.foo")
}

func TestPipe(t *testing.T) {
	pipe := &Pipe{
		Exprs: []Expr{
			&Ident{NamePos: token.Position{Line: 1, Column: 1}, Name: "a"},
			&Ident{NamePos: token.Position{Line: 1, Column: 5}, Name: "b"},
			&Ident{NamePos: token.Position{Line: 1, Column: 9}, Name: "c"},
		},
	}

	assert.Equal(t, pipe.Pos().Column, 1)
	assert.Equal(t, pipe.String(), "(a |> b |> c)")
}

func TestObjectCall(t *testing.T) {
	objCall := &ObjectCall{
		X: &Ident{
			NamePos: token.Position{Line: 1, Column: 1},
			Name:    "obj",
		},
		Period: token.Position{Line: 1, Column: 4},
		Call: &Call{
			Fun: &Ident{
				NamePos: token.Position{Line: 1, Column: 5},
				Name:    "method",
			},
			Lparen: token.Position{Line: 1, Column: 11},
			Args:   []Node{},
			Rparen: token.Position{Line: 1, Column: 12},
		},
		Optional: false,
	}

	assert.Equal(t, objCall.String(), "obj.method()")

	objCall.Optional = true
	assert.Equal(t, objCall.String(), "obj?.method()")
}

func TestIndex(t *testing.T) {
	index := &Index{
		X: &Ident{
			NamePos: token.Position{Line: 1, Column: 1},
			Name:    "arr",
		},
		Lbrack: token.Position{Line: 1, Column: 4},
		Index: &Int{
			ValuePos: token.Position{Line: 1, Column: 5},
			Literal:  "0",
			Value:    0,
		},
		Rbrack: token.Position{Line: 1, Column: 6},
	}

	assert.Equal(t, index.Pos().Column, 1)
	assert.Equal(t, index.End().Column, 7)
	assert.Equal(t, index.String(), "arr[0]")
}

func TestSlice(t *testing.T) {
	// Full slice
	slice := &Slice{
		X: &Ident{
			NamePos: token.Position{Line: 1, Column: 1},
			Name:    "arr",
		},
		Lbrack: token.Position{Line: 1, Column: 4},
		Low:    &Int{ValuePos: token.Position{Line: 1, Column: 5}, Literal: "1", Value: 1},
		High:   &Int{ValuePos: token.Position{Line: 1, Column: 7}, Literal: "3", Value: 3},
		Rbrack: token.Position{Line: 1, Column: 8},
	}

	assert.Equal(t, slice.String(), "arr[1:3]")

	// No low
	slice.Low = nil
	assert.Equal(t, slice.String(), "arr[:3]")

	// No high
	slice.Low = &Int{ValuePos: token.Position{Line: 1, Column: 5}, Literal: "1", Value: 1}
	slice.High = nil
	assert.Equal(t, slice.String(), "arr[1:]")
}

func TestCase(t *testing.T) {
	// Regular case
	caseExpr := &Case{
		Case:  token.Position{Line: 1, Column: 1},
		Exprs: []Expr{&Int{ValuePos: token.Position{Line: 1, Column: 6}, Literal: "1", Value: 1}},
		Colon: token.Position{Line: 1, Column: 7},
		Body: &Block{
			Lbrace: token.Position{Line: 2, Column: 1},
			Stmts: []Node{
				&Int{ValuePos: token.Position{Line: 2, Column: 2}, Literal: "42", Value: 42},
			},
			Rbrace: token.Position{Line: 2, Column: 4},
		},
	}

	s := caseExpr.String()
	assert.True(t, s != "", "Case.String() should not be empty")

	// Default case
	defaultCase := &Case{
		Case:    token.Position{Line: 1, Column: 1},
		Default: true,
		Colon:   token.Position{Line: 1, Column: 8},
		Body: &Block{
			Lbrace: token.Position{Line: 2, Column: 1},
			Stmts:  []Node{},
			Rbrace: token.Position{Line: 2, Column: 2},
		},
	}

	s = defaultCase.String()
	assert.True(t, s != "", "Default Case.String() should not be empty")
}

func TestSwitch(t *testing.T) {
	switchExpr := &Switch{
		Switch: token.Position{Line: 1, Column: 1},
		Lparen: token.Position{Line: 1, Column: 8},
		Value:  &Ident{NamePos: token.Position{Line: 1, Column: 9}, Name: "x"},
		Rparen: token.Position{Line: 1, Column: 10},
		Lbrace: token.Position{Line: 1, Column: 12},
		Cases:  []*Case{},
		Rbrace: token.Position{Line: 1, Column: 14},
	}

	assert.Equal(t, switchExpr.Pos().Column, 1)
	assert.Equal(t, switchExpr.End().Column, 15)
}

func TestIn(t *testing.T) {
	inExpr := &In{
		X:     &Ident{NamePos: token.Position{Line: 1, Column: 1}, Name: "x"},
		InPos: token.Position{Line: 1, Column: 3},
		Y:     &Ident{NamePos: token.Position{Line: 1, Column: 6}, Name: "list"},
	}

	assert.Equal(t, inExpr.Pos().Column, 1)
	assert.Equal(t, inExpr.String(), "x in list")
}

func TestNotIn(t *testing.T) {
	notInExpr := &NotIn{
		X:        &Ident{NamePos: token.Position{Line: 1, Column: 1}, Name: "x"},
		NotInPos: token.Position{Line: 1, Column: 3},
		Y:        &Ident{NamePos: token.Position{Line: 1, Column: 10}, Name: "list"},
	}

	assert.Equal(t, notInExpr.Pos().Column, 1)
	assert.Equal(t, notInExpr.String(), "x not in list")
}

// Literal tests
func TestIntLiteral(t *testing.T) {
	intLit := &Int{
		ValuePos: token.Position{Line: 1, Column: 1},
		Literal:  "42",
		Value:    42,
	}

	assert.Equal(t, intLit.Pos().Column, 1)
	assert.Equal(t, intLit.End().Column, 3)
	assert.Equal(t, intLit.String(), "42")
}

func TestFloatLiteral(t *testing.T) {
	floatLit := &Float{
		ValuePos: token.Position{Line: 1, Column: 1},
		Literal:  "3.14",
		Value:    3.14,
	}

	assert.Equal(t, floatLit.Pos().Column, 1)
	assert.Equal(t, floatLit.End().Column, 5)
	assert.Equal(t, floatLit.String(), "3.14")
}

func TestNilLiteral(t *testing.T) {
	nilLit := &Nil{
		NilPos: token.Position{Line: 1, Column: 1},
	}

	assert.Equal(t, nilLit.Pos().Column, 1)
	assert.Equal(t, nilLit.End().Column, 4)
	assert.Equal(t, nilLit.String(), "nil")
}

func TestBoolLiteral(t *testing.T) {
	boolLit := &Bool{
		ValuePos: token.Position{Line: 1, Column: 1},
		Literal:  "true",
		Value:    true,
	}

	assert.Equal(t, boolLit.Pos().Column, 1)
	assert.Equal(t, boolLit.End().Column, 5)
	assert.Equal(t, boolLit.String(), "true")
}

func TestStringLiteral(t *testing.T) {
	strLit := &String{
		ValuePos: token.Position{Line: 1, Column: 1},
		Literal:  `"hello"`,
		Value:    "hello",
	}

	assert.Equal(t, strLit.Pos().Column, 1)
	assert.Equal(t, strLit.End().Column, 8)
	assert.Equal(t, strLit.String(), `"hello"`)
}

func TestFuncLiteral(t *testing.T) {
	funcLit := &Func{
		Func:   token.Position{Line: 1, Column: 1},
		Lparen: token.Position{Line: 1, Column: 9},
		Params: []FuncParam{
			&Ident{NamePos: token.Position{Line: 1, Column: 10}, Name: "x"},
			&Ident{NamePos: token.Position{Line: 1, Column: 13}, Name: "y"},
		},
		Rparen: token.Position{Line: 1, Column: 14},
		Body: &Block{
			Lbrace: token.Position{Line: 1, Column: 16},
			Stmts:  []Node{},
			Rbrace: token.Position{Line: 1, Column: 18},
		},
	}

	assert.Equal(t, funcLit.Pos().Column, 1)
	assert.Equal(t, funcLit.String(), "function(x, y) {  }")

	// With name
	funcLit.Name = &Ident{NamePos: token.Position{Line: 1, Column: 10}, Name: "foo"}
	assert.Equal(t, funcLit.String(), "function foo(x, y) {  }")
}

func TestListLiteral(t *testing.T) {
	listLit := &List{
		Lbrack: token.Position{Line: 1, Column: 1},
		Items: []Expr{
			&Int{ValuePos: token.Position{Line: 1, Column: 2}, Literal: "1", Value: 1},
			&Int{ValuePos: token.Position{Line: 1, Column: 5}, Literal: "2", Value: 2},
		},
		Rbrack: token.Position{Line: 1, Column: 6},
	}

	assert.Equal(t, listLit.Pos().Column, 1)
	assert.Equal(t, listLit.End().Column, 7)
	assert.Equal(t, listLit.String(), "[1, 2]")
}

func TestMapLiteral(t *testing.T) {
	mapLit := &Map{
		Lbrace: token.Position{Line: 1, Column: 1},
		Items: []MapItem{
			{
				Key:   &String{ValuePos: token.Position{Line: 1, Column: 2}, Literal: `"a"`, Value: "a"},
				Value: &Int{ValuePos: token.Position{Line: 1, Column: 6}, Literal: "1", Value: 1},
			},
		},
		Rbrace: token.Position{Line: 1, Column: 7},
	}

	assert.Equal(t, mapLit.Pos().Column, 1)
	assert.Equal(t, mapLit.End().Column, 8)
	assert.False(t, mapLit.HasSpread())

	// With spread
	mapLit.Items = append(mapLit.Items, MapItem{
		Key:   nil,
		Value: &Ident{NamePos: token.Position{Line: 1, Column: 10}, Name: "other"},
	})
	assert.True(t, mapLit.HasSpread())
}

// Statement tests
func TestVarStatement(t *testing.T) {
	varStmt := &Var{
		Let:  token.Position{Line: 1, Column: 1},
		Name: &Ident{NamePos: token.Position{Line: 1, Column: 5}, Name: "x"},
		Value: &Int{
			ValuePos: token.Position{Line: 1, Column: 9},
			Literal:  "42",
			Value:    42,
		},
	}

	assert.Equal(t, varStmt.Pos().Column, 1)
	assert.Equal(t, varStmt.String(), "let x = 42")
}

func TestMultiVarStatement(t *testing.T) {
	multiVar := &MultiVar{
		Let: token.Position{Line: 1, Column: 1},
		Names: []*Ident{
			{NamePos: token.Position{Line: 1, Column: 5}, Name: "x"},
			{NamePos: token.Position{Line: 1, Column: 8}, Name: "y"},
		},
		Value: &List{
			Lbrack: token.Position{Line: 1, Column: 12},
			Items: []Expr{
				&Int{ValuePos: token.Position{Line: 1, Column: 13}, Literal: "1", Value: 1},
				&Int{ValuePos: token.Position{Line: 1, Column: 16}, Literal: "2", Value: 2},
			},
			Rbrack: token.Position{Line: 1, Column: 17},
		},
	}

	assert.Equal(t, multiVar.Pos().Column, 1)
	assert.Equal(t, multiVar.String(), "let x, y = [1, 2]")
}

func TestObjectDestructure(t *testing.T) {
	objDest := &ObjectDestructure{
		Let:    token.Position{Line: 1, Column: 1},
		Lbrace: token.Position{Line: 1, Column: 5},
		Bindings: []DestructureBinding{
			{Key: "a", Alias: ""},
			{Key: "b", Alias: "renamed"},
			{Key: "c", Alias: "", Default: &Int{ValuePos: token.Position{Line: 1, Column: 20}, Literal: "42", Value: 42}},
		},
		Rbrace: token.Position{Line: 1, Column: 25},
		Value:  &Ident{NamePos: token.Position{Line: 1, Column: 29}, Name: "obj"},
	}

	assert.Equal(t, objDest.Pos().Column, 1)
	s := objDest.String()
	assert.True(t, s != "", "ObjectDestructure.String() should not be empty")
}

func TestArrayDestructure(t *testing.T) {
	arrDest := &ArrayDestructure{
		Let:    token.Position{Line: 1, Column: 1},
		Lbrack: token.Position{Line: 1, Column: 5},
		Elements: []ArrayDestructureElement{
			{Name: &Ident{NamePos: token.Position{Line: 1, Column: 6}, Name: "a"}},
			{Name: &Ident{NamePos: token.Position{Line: 1, Column: 9}, Name: "b"}, Default: &Int{ValuePos: token.Position{Line: 1, Column: 13}, Literal: "0", Value: 0}},
		},
		Rbrack: token.Position{Line: 1, Column: 14},
		Value:  &Ident{NamePos: token.Position{Line: 1, Column: 18}, Name: "arr"},
	}

	assert.Equal(t, arrDest.Pos().Column, 1)
	s := arrDest.String()
	assert.True(t, s != "", "ArrayDestructure.String() should not be empty")
}

func TestConstStatement(t *testing.T) {
	constStmt := &Const{
		Const: token.Position{Line: 1, Column: 1},
		Name:  &Ident{NamePos: token.Position{Line: 1, Column: 7}, Name: "PI"},
		Value: &Float{ValuePos: token.Position{Line: 1, Column: 12}, Literal: "3.14", Value: 3.14},
	}

	assert.Equal(t, constStmt.Pos().Column, 1)
	assert.Equal(t, constStmt.String(), "const PI = 3.14")
}

func TestReturnStatement(t *testing.T) {
	// With value
	retStmt := &Return{
		Return: token.Position{Line: 1, Column: 1},
		Value:  &Int{ValuePos: token.Position{Line: 1, Column: 8}, Literal: "42", Value: 42},
	}

	assert.Equal(t, retStmt.Pos().Column, 1)
	assert.Equal(t, retStmt.String(), "return 42")

	// Without value
	retStmt.Value = nil
	assert.Equal(t, retStmt.End().Column, 7) // 1 + len("return")
	assert.Equal(t, retStmt.String(), "return")
}

func TestBlockStatement(t *testing.T) {
	block := &Block{
		Lbrace: token.Position{Line: 1, Column: 1},
		Stmts: []Node{
			&Int{ValuePos: token.Position{Line: 1, Column: 3}, Literal: "1", Value: 1},
			&Int{ValuePos: token.Position{Line: 2, Column: 1}, Literal: "2", Value: 2},
		},
		Rbrace: token.Position{Line: 2, Column: 2},
	}

	assert.Equal(t, block.Pos().Column, 1)
	assert.Equal(t, block.End().Column, 3)

	// Test EndsWithReturn
	assert.False(t, block.EndsWithReturn())

	block.Stmts = append(block.Stmts, &Return{Return: token.Position{Line: 3, Column: 1}})
	assert.True(t, block.EndsWithReturn())

	// Empty block
	emptyBlock := &Block{Stmts: []Node{}}
	assert.False(t, emptyBlock.EndsWithReturn())
}

func TestAssignStatement(t *testing.T) {
	// Simple assignment
	assign := &Assign{
		Name:  &Ident{NamePos: token.Position{Line: 1, Column: 1}, Name: "x"},
		OpPos: token.Position{Line: 1, Column: 3},
		Op:    "=",
		Value: &Int{ValuePos: token.Position{Line: 1, Column: 5}, Literal: "42", Value: 42},
	}

	assert.Equal(t, assign.Pos().Column, 1)
	assert.Equal(t, assign.String(), "x = 42")

	// Index assignment
	assign.Name = nil
	assign.Index = &Index{
		X:      &Ident{NamePos: token.Position{Line: 1, Column: 1}, Name: "arr"},
		Lbrack: token.Position{Line: 1, Column: 4},
		Index:  &Int{ValuePos: token.Position{Line: 1, Column: 5}, Literal: "0", Value: 0},
		Rbrack: token.Position{Line: 1, Column: 6},
	}
	assert.Equal(t, assign.Pos().Column, 1)
	assert.Equal(t, assign.String(), "arr[0] = 42")
}

func TestPostfixStatement(t *testing.T) {
	postfix := &Postfix{
		X:     &Ident{NamePos: token.Position{Line: 1, Column: 1}, Name: "x"},
		OpPos: token.Position{Line: 1, Column: 2},
		Op:    "++",
	}

	assert.Equal(t, postfix.Pos().Column, 1)
	assert.Equal(t, postfix.End().Column, 4)
	assert.Equal(t, postfix.String(), "(x++)")
}

func TestSetAttrStatement(t *testing.T) {
	setAttr := &SetAttr{
		X:      &Ident{NamePos: token.Position{Line: 1, Column: 1}, Name: "obj"},
		Period: token.Position{Line: 1, Column: 4},
		Attr:   &Ident{NamePos: token.Position{Line: 1, Column: 5}, Name: "foo"},
		OpPos:  token.Position{Line: 1, Column: 9},
		Op:     "=",
		Value:  &Int{ValuePos: token.Position{Line: 1, Column: 11}, Literal: "42", Value: 42},
	}

	assert.Equal(t, setAttr.Pos().Column, 1)
	assert.Equal(t, setAttr.String(), "obj.foo = 42")
}

func TestTryStatement(t *testing.T) {
	tryStmt := &Try{
		Try: token.Position{Line: 1, Column: 1},
		Body: &Block{
			Lbrace: token.Position{Line: 1, Column: 5},
			Stmts:  []Node{},
			Rbrace: token.Position{Line: 1, Column: 7},
		},
	}

	assert.Equal(t, tryStmt.Pos().Column, 1)
	assert.Equal(t, tryStmt.End().Column, 8) // Body end

	// With catch
	tryStmt.Catch = token.Position{Line: 1, Column: 10}
	tryStmt.CatchIdent = &Ident{NamePos: token.Position{Line: 1, Column: 16}, Name: "e"}
	tryStmt.CatchBlock = &Block{
		Lbrace: token.Position{Line: 1, Column: 18},
		Stmts:  []Node{},
		Rbrace: token.Position{Line: 1, Column: 20},
	}
	assert.Equal(t, tryStmt.End().Column, 21)

	// With finally
	tryStmt.Finally = token.Position{Line: 1, Column: 23}
	tryStmt.FinallyBlock = &Block{
		Lbrace: token.Position{Line: 1, Column: 31},
		Stmts:  []Node{},
		Rbrace: token.Position{Line: 1, Column: 33},
	}
	assert.Equal(t, tryStmt.End().Column, 34)
}

func TestThrowStatement(t *testing.T) {
	// With value
	throwStmt := &Throw{
		Throw: token.Position{Line: 1, Column: 1},
		Value: &String{ValuePos: token.Position{Line: 1, Column: 7}, Literal: `"error"`, Value: "error"},
	}

	assert.Equal(t, throwStmt.Pos().Column, 1)
	assert.Equal(t, throwStmt.String(), `throw "error"`)

	// Without value
	throwStmt.Value = nil
	assert.Equal(t, throwStmt.End().Column, 6) // 1 + len("throw")
	assert.Equal(t, throwStmt.String(), "throw ")
}

// Bug reproduction tests - These test edge cases and nil handling

func TestVarNilValue(t *testing.T) {
	// Var with nil Value - tests inconsistency between End() and String()
	// String() handles nil Value, but End() should also handle it
	varStmt := &Var{
		Let:   token.Position{Line: 1, Column: 1},
		Name:  &Ident{NamePos: token.Position{Line: 1, Column: 5}, Name: "x"},
		Value: nil,
	}

	// String() handles nil Value gracefully
	s := varStmt.String()
	assert.Equal(t, s, "let x = ")

	// End() should not panic with nil Value
	// Currently this would panic: x.Value.End() when Value is nil
	// After fix, should return position after variable name or "="
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Var.End() panicked with nil Value: %v", r)
		}
	}()
	_ = varStmt.End()
}

func TestMultiVarNilValue(t *testing.T) {
	// MultiVar with nil Value
	multiVar := &MultiVar{
		Let: token.Position{Line: 1, Column: 1},
		Names: []*Ident{
			{NamePos: token.Position{Line: 1, Column: 5}, Name: "x"},
		},
		Value: nil,
	}

	// End() should not panic with nil Value
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("MultiVar.End() panicked with nil Value: %v", r)
		}
	}()
	_ = multiVar.End()
}

func TestConstNilValue(t *testing.T) {
	// Const with nil Value - tests inconsistency between End() and String()
	// String() handles nil Value, but End() should also handle it
	constStmt := &Const{
		Const: token.Position{Line: 1, Column: 1},
		Name:  &Ident{NamePos: token.Position{Line: 1, Column: 7}, Name: "X"},
		Value: nil,
	}

	// String() handles nil Value gracefully
	s := constStmt.String()
	assert.Equal(t, s, "const X = ")

	// End() should not panic with nil Value
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Const.End() panicked with nil Value: %v", r)
		}
	}()
	_ = constStmt.End()
}

func TestPipeEmptyExprs(t *testing.T) {
	// Pipe with empty Exprs slice - would cause index out of bounds
	pipe := &Pipe{
		Exprs: []Expr{},
	}

	// Pos() and End() should not panic with empty Exprs
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Pipe.Pos() or End() panicked with empty Exprs: %v", r)
		}
	}()

	// These currently panic: index out of range
	_ = pipe.Pos()
	_ = pipe.End()
}

func TestFuncNilBody(t *testing.T) {
	// Func with nil Body
	funcLit := &Func{
		Func:   token.Position{Line: 1, Column: 1},
		Lparen: token.Position{Line: 1, Column: 9},
		Params: []FuncParam{},
		Rparen: token.Position{Line: 1, Column: 10},
		Body:   nil,
	}

	// End() should not panic with nil Body
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Func.End() panicked with nil Body: %v", r)
		}
	}()
	_ = funcLit.End()
}

func TestDefaultValueNilDefault(t *testing.T) {
	// DefaultValue with nil Default
	dv := &DefaultValue{
		Name:    &Ident{NamePos: token.Position{Line: 1, Column: 1}, Name: "x"},
		Default: nil,
	}

	// End() should not panic with nil Default
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("DefaultValue.End() panicked with nil Default: %v", r)
		}
	}()
	_ = dv.End()
}

func TestObjectDestructureNilValue(t *testing.T) {
	// ObjectDestructure with nil Value
	objDest := &ObjectDestructure{
		Let:      token.Position{Line: 1, Column: 1},
		Lbrace:   token.Position{Line: 1, Column: 5},
		Bindings: []DestructureBinding{{Key: "a"}},
		Rbrace:   token.Position{Line: 1, Column: 8},
		Value:    nil,
	}

	// End() should not panic
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("ObjectDestructure.End() panicked with nil Value: %v", r)
		}
	}()
	_ = objDest.End()
}

func TestArrayDestructureNilValue(t *testing.T) {
	// ArrayDestructure with nil Value
	arrDest := &ArrayDestructure{
		Let:    token.Position{Line: 1, Column: 1},
		Lbrack: token.Position{Line: 1, Column: 5},
		Elements: []ArrayDestructureElement{
			{Name: &Ident{NamePos: token.Position{Line: 1, Column: 6}, Name: "a"}},
		},
		Rbrack: token.Position{Line: 1, Column: 8},
		Value:  nil,
	}

	// End() should not panic
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("ArrayDestructure.End() panicked with nil Value: %v", r)
		}
	}()
	_ = arrDest.End()
}

func TestIfNilConsequence(t *testing.T) {
	// If with nil Consequence (edge case - should be invalid AST, but End() should not panic)
	ifExpr := &If{
		If:          token.Position{Line: 1, Column: 1},
		Lparen:      token.Position{Line: 1, Column: 4},
		Cond:        &Bool{ValuePos: token.Position{Line: 1, Column: 5}, Value: true},
		Rparen:      token.Position{Line: 1, Column: 9},
		Consequence: nil,
	}

	// End() should not panic
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("If.End() panicked with nil Consequence: %v", r)
		}
	}()
	_ = ifExpr.End()
}

func TestPrefixNilX(t *testing.T) {
	// Prefix with nil X
	prefix := &Prefix{
		OpPos: token.Position{Line: 1, Column: 1},
		Op:    "!",
		X:     nil,
	}

	// End() should not panic
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Prefix.End() panicked with nil X: %v", r)
		}
	}()
	_ = prefix.End()
}

func TestInfixNilOperands(t *testing.T) {
	// Infix with nil X
	infix := &Infix{
		X:     nil,
		OpPos: token.Position{Line: 1, Column: 3},
		Op:    "+",
		Y:     &Int{ValuePos: token.Position{Line: 1, Column: 5}, Value: 2},
	}

	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Infix.Pos() panicked with nil X: %v", r)
		}
	}()
	_ = infix.Pos()

	// Infix with nil Y
	infix2 := &Infix{
		X:     &Int{ValuePos: token.Position{Line: 1, Column: 1}, Value: 1},
		OpPos: token.Position{Line: 1, Column: 3},
		Op:    "+",
		Y:     nil,
	}

	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Infix.End() panicked with nil Y: %v", r)
		}
	}()
	_ = infix2.End()
}

func TestCaseNilBody(t *testing.T) {
	// Case with nil Body
	caseExpr := &Case{
		Case:  token.Position{Line: 1, Column: 1},
		Exprs: []Expr{&Int{ValuePos: token.Position{Line: 1, Column: 6}, Value: 1}},
		Colon: token.Position{Line: 1, Column: 7},
		Body:  nil,
	}

	// End() should use Colon position when Body is nil
	pos := caseExpr.End()
	assert.Equal(t, pos.Column, 8) // Colon.Advance(1)
}

func TestGetAttrNilX(t *testing.T) {
	// GetAttr with nil X
	getAttr := &GetAttr{
		X:      nil,
		Period: token.Position{Line: 1, Column: 4},
		Attr:   &Ident{NamePos: token.Position{Line: 1, Column: 5}, Name: "foo"},
	}

	defer func() {
		if r := recover(); r != nil {
			t.Errorf("GetAttr.Pos() panicked with nil X: %v", r)
		}
	}()
	_ = getAttr.Pos()
}

func TestObjectCallNilX(t *testing.T) {
	// ObjectCall with nil X
	objCall := &ObjectCall{
		X:      nil,
		Period: token.Position{Line: 1, Column: 4},
		Call: &Call{
			Fun:    &Ident{NamePos: token.Position{Line: 1, Column: 5}, Name: "method"},
			Lparen: token.Position{Line: 1, Column: 11},
			Args:   []Node{},
			Rparen: token.Position{Line: 1, Column: 12},
		},
	}

	defer func() {
		if r := recover(); r != nil {
			t.Errorf("ObjectCall.Pos() panicked with nil X: %v", r)
		}
	}()
	_ = objCall.Pos()
}

func TestCallNilFun(t *testing.T) {
	// Call with nil Fun
	call := &Call{
		Fun:    nil,
		Lparen: token.Position{Line: 1, Column: 4},
		Args:   []Node{},
		Rparen: token.Position{Line: 1, Column: 5},
	}

	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Call.Pos() panicked with nil Fun: %v", r)
		}
	}()
	_ = call.Pos()
}

func TestIndexNilX(t *testing.T) {
	// Index with nil X
	index := &Index{
		X:      nil,
		Lbrack: token.Position{Line: 1, Column: 4},
		Index:  &Int{ValuePos: token.Position{Line: 1, Column: 5}, Value: 0},
		Rbrack: token.Position{Line: 1, Column: 6},
	}

	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Index.Pos() panicked with nil X: %v", r)
		}
	}()
	_ = index.Pos()
}

func TestSliceNilX(t *testing.T) {
	// Slice with nil X
	slice := &Slice{
		X:      nil,
		Lbrack: token.Position{Line: 1, Column: 4},
		Low:    nil,
		High:   nil,
		Rbrack: token.Position{Line: 1, Column: 6},
	}

	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Slice.Pos() panicked with nil X: %v", r)
		}
	}()
	_ = slice.Pos()
}

func TestInNilOperands(t *testing.T) {
	// In with nil X
	inExpr := &In{
		X:     nil,
		InPos: token.Position{Line: 1, Column: 3},
		Y:     &Ident{NamePos: token.Position{Line: 1, Column: 6}, Name: "list"},
	}

	defer func() {
		if r := recover(); r != nil {
			t.Errorf("In.Pos() panicked with nil X: %v", r)
		}
	}()
	_ = inExpr.Pos()

	// In with nil Y
	inExpr2 := &In{
		X:     &Ident{NamePos: token.Position{Line: 1, Column: 1}, Name: "x"},
		InPos: token.Position{Line: 1, Column: 3},
		Y:     nil,
	}

	defer func() {
		if r := recover(); r != nil {
			t.Errorf("In.End() panicked with nil Y: %v", r)
		}
	}()
	_ = inExpr2.End()
}

func TestNotInNilOperands(t *testing.T) {
	// NotIn with nil X
	notInExpr := &NotIn{
		X:        nil,
		NotInPos: token.Position{Line: 1, Column: 3},
		Y:        &Ident{NamePos: token.Position{Line: 1, Column: 10}, Name: "list"},
	}

	defer func() {
		if r := recover(); r != nil {
			t.Errorf("NotIn.Pos() panicked with nil X: %v", r)
		}
	}()
	_ = notInExpr.Pos()

	// NotIn with nil Y
	notInExpr2 := &NotIn{
		X:        &Ident{NamePos: token.Position{Line: 1, Column: 1}, Name: "x"},
		NotInPos: token.Position{Line: 1, Column: 3},
		Y:        nil,
	}

	defer func() {
		if r := recover(); r != nil {
			t.Errorf("NotIn.End() panicked with nil Y: %v", r)
		}
	}()
	_ = notInExpr2.End()
}

func TestTryNilBody(t *testing.T) {
	// Try with nil Body (edge case)
	tryStmt := &Try{
		Try:  token.Position{Line: 1, Column: 1},
		Body: nil,
	}

	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Try.End() panicked with nil Body: %v", r)
		}
	}()
	_ = tryStmt.End()
}

func TestAssignNilNameAndIndex(t *testing.T) {
	// Assign with both Name and Index nil (should not happen, but test resilience)
	assign := &Assign{
		Name:  nil,
		Index: nil,
		OpPos: token.Position{Line: 1, Column: 3},
		Op:    "=",
		Value: &Int{ValuePos: token.Position{Line: 1, Column: 5}, Value: 42},
	}

	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Assign.Pos() panicked with nil Name and Index: %v", r)
		}
	}()
	_ = assign.Pos()
}
