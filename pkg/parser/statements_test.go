package parser

import (
	"context"
	"testing"

	"github.com/deepnoodle-ai/risor/v2/pkg/ast"
	"github.com/deepnoodle-ai/wonton/assert"
)

// Tests for statement parsing (statements.go)
// - Variable declarations (let)
// - Constant declarations (const)
// - Multi-variable declarations
// - Destructuring patterns
// - Return statements
// - Throw statements
// - Assignment statements
// - Postfix operators
// - Try/catch/finally

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
		t.Run(tt.input, func(t *testing.T) {
			program, err := Parse(context.Background(), tt.input, nil)
			assert.Nil(t, err)
			assert.Len(t, program.Stmts, 1)

			stmt, ok := program.First().(*ast.Var)
			assert.True(t, ok)
			testVarStatement(t, stmt, tt.ident)
			testLiteralExpression(t, stmt.Value, tt.value)
			assert.Equal(t, tt.ident, stmt.Name.Name)
		})
	}
}

func TestVarAST(t *testing.T) {
	program, err := Parse(context.Background(), "let x = 42", nil)
	assert.Nil(t, err)

	varStmt, ok := program.First().(*ast.Var)
	assert.True(t, ok)

	// Verify AST node fields
	assert.NotNil(t, varStmt.Name)
	assert.Equal(t, "x", varStmt.Name.Name)
	assert.NotNil(t, varStmt.Value)

	val, ok := varStmt.Value.(*ast.Int)
	assert.True(t, ok)
	assert.Equal(t, int64(42), val.Value)
}

func TestDeclareStatements(t *testing.T) {
	input := `
	let x = foo.bar()
	let y = foo.bar()
	`
	program, err := Parse(context.Background(), input, nil)
	assert.Nil(t, err)

	statements := program.Stmts
	assert.Len(t, statements, 2)

	stmt1, ok := statements[0].(*ast.Var)
	assert.True(t, ok)
	assert.Equal(t, "x", stmt1.Name.Name)

	stmt2, ok := statements[1].(*ast.Var)
	assert.True(t, ok)
	assert.Equal(t, "y", stmt2.Name.Name)
}

func TestMultiDeclareStatements(t *testing.T) {
	input := `let x, y, z = [1, 2, 3]`
	program, err := Parse(context.Background(), input, nil)
	assert.Nil(t, err)

	statements := program.Stmts
	assert.Len(t, statements, 1)

	stmt1, ok := statements[0].(*ast.MultiVar)
	assert.True(t, ok)
	assert.Len(t, stmt1.Names, 3)
	assert.Equal(t, "x", stmt1.Names[0].Name)
	assert.Equal(t, "y", stmt1.Names[1].Name)
	assert.Equal(t, "z", stmt1.Names[2].Name)
	assert.Equal(t, "[1, 2, 3]", stmt1.Value.String())
}

func TestMultiVar(t *testing.T) {
	program, err := Parse(context.Background(), "let x, y = [1, 2]", nil)
	assert.Nil(t, err)
	assert.Len(t, program.Stmts, 1)

	mvar, ok := program.First().(*ast.MultiVar)
	assert.True(t, ok)
	assert.Equal(t, "x", mvar.Names[0].Name)
	assert.Equal(t, "y", mvar.Names[1].Name)
	assert.Equal(t, "[1, 2]", mvar.Value.String())
}

func TestMultiVarAST(t *testing.T) {
	program, err := Parse(context.Background(), "let a, b, c = values", nil)
	assert.Nil(t, err)

	multiVar, ok := program.First().(*ast.MultiVar)
	assert.True(t, ok)

	// Verify AST node fields
	assert.Len(t, multiVar.Names, 3)
	assert.Equal(t, "a", multiVar.Names[0].Name)
	assert.Equal(t, "b", multiVar.Names[1].Name)
	assert.Equal(t, "c", multiVar.Names[2].Name)
	assert.NotNil(t, multiVar.Value)
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
		t.Run(tt.input, func(t *testing.T) {
			program, err := Parse(context.Background(), tt.input, nil)
			assert.Nil(t, err)
			assert.Len(t, program.Stmts, 1)

			stmt, ok := program.First().(*ast.Const)
			assert.True(t, ok)
			testConstStatement(t, stmt, tt.expectedIdentifier)
			assert.Equal(t, tt.expectedIdentifier, stmt.Name.Name)
			testLiteralExpression(t, stmt.Value, tt.expectedValue)
		})
	}
}

func TestConstAST(t *testing.T) {
	program, err := Parse(context.Background(), "const PI = 3.14", nil)
	assert.Nil(t, err)

	constStmt, ok := program.First().(*ast.Const)
	assert.True(t, ok)

	// Verify AST node fields
	assert.NotNil(t, constStmt.Name)
	assert.Equal(t, "PI", constStmt.Name.Name)
	assert.NotNil(t, constStmt.Value)

	val, ok := constStmt.Value.(*ast.Float)
	assert.True(t, ok)
	assert.Equal(t, 3.14, val.Value)
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
		t.Run(tt.input, func(t *testing.T) {
			_, err := Parse(context.Background(), tt.input, nil)
			assert.NotNil(t, err)
			e, ok := err.(ParserError)
			assert.True(t, ok)
			assert.Equal(t, tt.err, e.Error())
		})
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
		t.Run(tt.input, func(t *testing.T) {
			program, err := Parse(context.Background(), tt.input, nil)
			assert.Nil(t, err)
			assert.Len(t, program.Stmts, 1)

			control, ok := program.First().(*ast.Return)
			assert.True(t, ok)
			assert.NotNil(t, control.Value)
		})
	}
}

func TestReturnAST(t *testing.T) {
	program, err := Parse(context.Background(), "return x + 1", nil)
	assert.Nil(t, err)

	ret, ok := program.First().(*ast.Return)
	assert.True(t, ok)

	// Verify AST node fields
	assert.NotNil(t, ret.Value)

	infix, ok := ret.Value.(*ast.Infix)
	assert.True(t, ok)
	assert.Equal(t, "+", infix.Op)
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
		t.Run(tt.input, func(t *testing.T) {
			result, err := Parse(context.Background(), tt.input, nil)
			assert.Nil(t, err)
			assert.Equal(t, tt.expected, result.String())
		})
	}
}

func TestObjectDestructure(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		bindings []struct {
			key        string
			alias      string
			hasDefault bool
		}
	}{
		{
			name:  "basic",
			input: `let { a, b } = obj`,
			bindings: []struct {
				key        string
				alias      string
				hasDefault bool
			}{
				{"a", "", false},
				{"b", "", false},
			},
		},
		{
			name:  "with alias",
			input: `let { a: x, b: y } = obj`,
			bindings: []struct {
				key        string
				alias      string
				hasDefault bool
			}{
				{"a", "x", false},
				{"b", "y", false},
			},
		},
		{
			name:  "with default",
			input: `let { a = 10, b = 20 } = obj`,
			bindings: []struct {
				key        string
				alias      string
				hasDefault bool
			}{
				{"a", "", true},
				{"b", "", true},
			},
		},
		{
			name:  "with alias and default",
			input: `let { a: x = 10 } = obj`,
			bindings: []struct {
				key        string
				alias      string
				hasDefault bool
			}{
				{"a", "x", true},
			},
		},
		{
			name:  "single binding",
			input: `let { name } = person`,
			bindings: []struct {
				key        string
				alias      string
				hasDefault bool
			}{
				{"name", "", false},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			program, err := Parse(context.Background(), tt.input, nil)
			assert.Nil(t, err)
			assert.Len(t, program.Stmts, 1)

			destruct, ok := program.First().(*ast.ObjectDestructure)
			assert.True(t, ok, "expected ObjectDestructure, got %T", program.First())
			assert.Len(t, destruct.Bindings, len(tt.bindings))

			for i, expected := range tt.bindings {
				binding := destruct.Bindings[i]
				assert.Equal(t, expected.key, binding.Key)
				assert.Equal(t, expected.alias, binding.Alias)
				if expected.hasDefault {
					assert.NotNil(t, binding.Default)
				} else {
					assert.Nil(t, binding.Default)
				}
			}
		})
	}
}

func TestObjectDestructureAST(t *testing.T) {
	program, err := Parse(context.Background(), "let { x, y } = point", nil)
	assert.Nil(t, err)

	destruct, ok := program.First().(*ast.ObjectDestructure)
	assert.True(t, ok)

	// Verify AST node fields
	assert.Len(t, destruct.Bindings, 2)
	assert.Equal(t, "x", destruct.Bindings[0].Key)
	assert.Equal(t, "y", destruct.Bindings[1].Key)
	assert.NotNil(t, destruct.Value)

	val, ok := destruct.Value.(*ast.Ident)
	assert.True(t, ok)
	assert.Equal(t, "point", val.Name)
}

func TestObjectDestructureErrors(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{`let { } = obj`, "destructuring pattern cannot be empty"},
		{`let { 123 } = obj`, "expected identifier in destructuring pattern"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			_, err := Parse(context.Background(), tt.input, nil)
			assert.NotNil(t, err)
			assert.Contains(t, err.Error(), tt.expected)
		})
	}
}

func TestObjectDestructureTrailingComma(t *testing.T) {
	program, err := Parse(context.Background(), `let { a, } = obj`, nil)
	assert.Nil(t, err)
	assert.Len(t, program.Stmts, 1)

	destruct, ok := program.First().(*ast.ObjectDestructure)
	assert.True(t, ok)
	assert.Len(t, destruct.Bindings, 1)
	assert.Equal(t, "a", destruct.Bindings[0].Key)
}

func TestObjectDestructureWithNewlines(t *testing.T) {
	input := `let {
		a,
		b: c = 2,
	} = obj`
	program, err := Parse(context.Background(), input, nil)
	assert.Nil(t, err)
	assert.Len(t, program.Stmts, 1)

	destruct, ok := program.First().(*ast.ObjectDestructure)
	assert.True(t, ok)
	assert.Len(t, destruct.Bindings, 2)
	assert.Equal(t, "a", destruct.Bindings[0].Key)
	assert.Equal(t, "b", destruct.Bindings[1].Key)
	assert.Equal(t, "c", destruct.Bindings[1].Alias)
	assert.NotNil(t, destruct.Bindings[1].Default)
}

func TestArrayDestructure(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		elements []struct {
			name       string
			hasDefault bool
		}
	}{
		{
			name:  "basic",
			input: `let [a, b, c] = arr`,
			elements: []struct {
				name       string
				hasDefault bool
			}{
				{"a", false},
				{"b", false},
				{"c", false},
			},
		},
		{
			name:  "with defaults",
			input: `let [a = 1, b = 2] = arr`,
			elements: []struct {
				name       string
				hasDefault bool
			}{
				{"a", true},
				{"b", true},
			},
		},
		{
			name:  "mixed",
			input: `let [x, y = 10] = arr`,
			elements: []struct {
				name       string
				hasDefault bool
			}{
				{"x", false},
				{"y", true},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			program, err := Parse(context.Background(), tt.input, nil)
			assert.Nil(t, err)
			assert.Len(t, program.Stmts, 1)

			destruct, ok := program.First().(*ast.ArrayDestructure)
			assert.True(t, ok, "expected ArrayDestructure, got %T", program.First())
			assert.Len(t, destruct.Elements, len(tt.elements))

			for i, expected := range tt.elements {
				elem := destruct.Elements[i]
				assert.Equal(t, expected.name, elem.Name.Name)
				if expected.hasDefault {
					assert.NotNil(t, elem.Default)
				} else {
					assert.Nil(t, elem.Default)
				}
			}
		})
	}
}

func TestArrayDestructureWithNewlines(t *testing.T) {
	input := `let [
		first,
		second = 2,
	] = items`
	program, err := Parse(context.Background(), input, nil)
	assert.Nil(t, err)
	assert.Len(t, program.Stmts, 1)

	destruct, ok := program.First().(*ast.ArrayDestructure)
	assert.True(t, ok)
	assert.Len(t, destruct.Elements, 2)
	assert.Equal(t, "first", destruct.Elements[0].Name.Name)
	assert.Equal(t, "second", destruct.Elements[1].Name.Name)
	assert.NotNil(t, destruct.Elements[1].Default)
}

func TestArrayDestructureAST(t *testing.T) {
	program, err := Parse(context.Background(), "let [first, second] = items", nil)
	assert.Nil(t, err)

	destruct, ok := program.First().(*ast.ArrayDestructure)
	assert.True(t, ok)

	// Verify AST node fields
	assert.Len(t, destruct.Elements, 2)
	assert.Equal(t, "first", destruct.Elements[0].Name.Name)
	assert.Equal(t, "second", destruct.Elements[1].Name.Name)
	assert.NotNil(t, destruct.Value)

	val, ok := destruct.Value.(*ast.Ident)
	assert.True(t, ok)
	assert.Equal(t, "items", val.Name)
}

func TestArrayDestructureErrors(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{`let [] = arr`, "array destructuring pattern cannot be empty"},
		{`let [123] = arr`, "expected identifier in array destructuring pattern"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			_, err := Parse(context.Background(), tt.input, nil)
			assert.NotNil(t, err)
			assert.Contains(t, err.Error(), tt.expected)
		})
	}
}

func TestAssign(t *testing.T) {
	tests := []struct {
		input string
		op    string
		name  string
	}{
		{"x = 1", "=", "x"},
		{"x += 1", "+=", "x"},
		{"x -= 1", "-=", "x"},
		{"x *= 2", "*=", "x"},
		{"x /= 2", "/=", "x"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			program, err := Parse(context.Background(), tt.input, nil)
			assert.Nil(t, err)
			assert.Len(t, program.Stmts, 1)

			assign, ok := program.First().(*ast.Assign)
			assert.True(t, ok, "expected Assign, got %T", program.First())
			assert.Equal(t, tt.op, assign.Op)
			assert.Equal(t, tt.name, assign.Name.Name)
		})
	}
}

func TestAssignAST(t *testing.T) {
	program, err := Parse(context.Background(), "x = 42", nil)
	assert.Nil(t, err)

	assign, ok := program.First().(*ast.Assign)
	assert.True(t, ok)

	// Verify AST node fields
	assert.NotNil(t, assign.Name)
	assert.Equal(t, "x", assign.Name.Name)
	assert.Equal(t, "=", assign.Op)
	assert.NotNil(t, assign.Value)
	assert.Nil(t, assign.Index) // Not an index assignment
}

func TestIndexAssignment(t *testing.T) {
	tests := []struct {
		input string
		op    string
	}{
		{`arr[0] = 1`, "="},
		{`arr[0] += 1`, "+="},
		{`arr[0] -= 1`, "-="},
		{`m["key"] = "value"`, "="},
		{`m["key"] += "!"`, "+="},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			program, err := Parse(context.Background(), tt.input, nil)
			assert.Nil(t, err)
			assert.Len(t, program.Stmts, 1)

			assign, ok := program.First().(*ast.Assign)
			assert.True(t, ok, "expected Assign, got %T", program.First())
			assert.Equal(t, tt.op, assign.Op)
			assert.NotNil(t, assign.Index)
		})
	}
}

func TestIndexAssignmentAST(t *testing.T) {
	program, err := Parse(context.Background(), "arr[0] = 42", nil)
	assert.Nil(t, err)

	assign, ok := program.First().(*ast.Assign)
	assert.True(t, ok)

	// Verify AST node fields
	assert.Nil(t, assign.Name) // Index assignment has no direct name
	assert.NotNil(t, assign.Index)
	assert.Equal(t, "=", assign.Op)
	assert.NotNil(t, assign.Value)

	// Verify the index
	assert.Equal(t, "arr", assign.Index.X.String())
	idx, ok := assign.Index.Index.(*ast.Int)
	assert.True(t, ok)
	assert.Equal(t, int64(0), idx.Value)
}

func TestSetAttrCompound(t *testing.T) {
	tests := []struct {
		input string
		op    string
	}{
		{`obj.x = 1`, "="},
		{`obj.x += 1`, "+="},
		{`obj.x -= 1`, "-="},
		{`obj.x *= 2`, "*="},
		{`obj.x /= 2`, "/="},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			program, err := Parse(context.Background(), tt.input, nil)
			assert.Nil(t, err)
			assert.Len(t, program.Stmts, 1)

			setAttr, ok := program.First().(*ast.SetAttr)
			assert.True(t, ok, "expected SetAttr, got %T", program.First())
			assert.Equal(t, tt.op, setAttr.Op)
		})
	}
}

func TestSetAttrAST(t *testing.T) {
	program, err := Parse(context.Background(), "obj.field = 42", nil)
	assert.Nil(t, err)

	setAttr, ok := program.First().(*ast.SetAttr)
	assert.True(t, ok)

	// Verify AST node fields
	obj, ok := setAttr.X.(*ast.Ident)
	assert.True(t, ok)
	assert.Equal(t, "obj", obj.Name)

	assert.Equal(t, "field", setAttr.Attr.Name)
	assert.Equal(t, "=", setAttr.Op)
	assert.NotNil(t, setAttr.Value)
}

func TestMutators(t *testing.T) {
	inputs := []string{
		"let w = 5; w *= 3;",
		"let x = 15; x += 3;",
		"let y = 10; y /= 2;",
		"let z = 10; y -= 2;",
		"let z = 1; z++;",
		"let z = 1; z--;",
		"let z = 10; let a = 3; y = a;",
		"let arr = [1, 2, 3]; arr[0]++;",
		"let arr = [1, 2, 3]; arr[0]--;",
		`let m = {a: 1}; m["a"]++;`,
		"let obj = {x: 5}; obj.x++;",
		"let obj = {x: 5}; obj.x--;",
	}
	for _, input := range inputs {
		t.Run(input, func(t *testing.T) {
			_, err := Parse(context.Background(), input, nil)
			assert.Nil(t, err)
		})
	}
}

func TestPostfix(t *testing.T) {
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
		t.Run(tt.input, func(t *testing.T) {
			program, err := Parse(context.Background(), tt.input, nil)
			assert.Nil(t, err, "failed to parse: %s", tt.input)
			assert.Len(t, program.Stmts, 1)
			assert.Equal(t, tt.expected, program.Stmts[0].String())
		})
	}
}

func TestPostfixAST(t *testing.T) {
	program, err := Parse(context.Background(), "x++", nil)
	assert.Nil(t, err)

	postfix, ok := program.First().(*ast.Postfix)
	assert.True(t, ok)

	// Verify AST node fields
	x, ok := postfix.X.(*ast.Ident)
	assert.True(t, ok)
	assert.Equal(t, "x", x.Name)
	assert.Equal(t, "++", postfix.Op)
}

func TestPostfixErrors(t *testing.T) {
	errorCases := []string{
		"1++;",
		"(1 + 2)++;",
		`"hello"++;`,
		"true++;",
		"nil++;",
		"[1, 2, 3]++;",
		"func() {}++;",
	}
	for _, input := range errorCases {
		t.Run(input, func(t *testing.T) {
			_, err := Parse(context.Background(), input, nil)
			assert.NotNil(t, err, "expected error for: %s", input)
		})
	}
}

func TestTryCatchFinally(t *testing.T) {
	validInputs := []string{
		`try { throw "err" } catch e { e }`,
		`try { throw "err" }
catch e { e }`,
		`try { throw "err" }
catch e { e }
finally { "done" }`,
		`try { "ok" }
finally { "cleanup" }`,
		`try { "ok" }


catch e { e }`,
		`try { "ok" }
catch e { e }


finally { "done" }`,
		`try { 1 }
catch e { 2 }
let x = 3`,
		`try { 1 }
finally { 2 }
let x = 3`,
		`try {
	try { throw "inner" }
	catch e { throw "outer" }
}
catch e { e }`,
		`function foo() {
	try { throw "err" }
	catch e { return e }
}`,
		`try { throw "err" }
catch { "handled" }`,
	}
	for _, input := range validInputs {
		t.Run(input, func(t *testing.T) {
			_, err := Parse(context.Background(), input, nil)
			assert.Nil(t, err, "failed to parse: %s", input)
		})
	}

	errorCases := []string{
		`try { 1 }`,
		`catch e { 1 }`,
		`finally { 1 }`,
	}
	for _, input := range errorCases {
		t.Run(input, func(t *testing.T) {
			_, err := Parse(context.Background(), input, nil)
			assert.NotNil(t, err, "expected error for: %s", input)
		})
	}
}

func TestTryAST(t *testing.T) {
	program, err := Parse(context.Background(), `try { risky() } catch e { handle(e) } finally { cleanup() }`, nil)
	assert.Nil(t, err)

	tryStmt, ok := program.First().(*ast.Try)
	assert.True(t, ok)

	// Verify AST node fields
	assert.NotNil(t, tryStmt.Body)
	assert.Len(t, tryStmt.Body.Stmts, 1)

	assert.NotNil(t, tryStmt.CatchIdent)
	assert.Equal(t, "e", tryStmt.CatchIdent.Name)

	assert.NotNil(t, tryStmt.CatchBlock)
	assert.Len(t, tryStmt.CatchBlock.Stmts, 1)

	assert.NotNil(t, tryStmt.FinallyBlock)
	assert.Len(t, tryStmt.FinallyBlock.Stmts, 1)
}

func TestTryWithoutCatchIdent(t *testing.T) {
	program, err := Parse(context.Background(), `try { risky() } catch { handle() }`, nil)
	assert.Nil(t, err)

	tryStmt, ok := program.First().(*ast.Try)
	assert.True(t, ok)

	// No catch identifier
	assert.Nil(t, tryStmt.CatchIdent)
	assert.NotNil(t, tryStmt.CatchBlock)
}

func TestThrow(t *testing.T) {
	program, err := Parse(context.Background(), `throw "error"`, nil)
	assert.Nil(t, err)
	assert.Len(t, program.Stmts, 1)

	throwStmt, ok := program.First().(*ast.Throw)
	assert.True(t, ok)
	assert.NotNil(t, throwStmt.Value)
}

func TestThrowAST(t *testing.T) {
	program, err := Parse(context.Background(), `throw error`, nil)
	assert.Nil(t, err)

	throwStmt, ok := program.First().(*ast.Throw)
	assert.True(t, ok)

	// Verify AST node fields
	val, ok := throwStmt.Value.(*ast.Ident)
	assert.True(t, ok)
	assert.Equal(t, "error", val.Name)
}

func TestThrowError(t *testing.T) {
	// throw without value should error
	_, err := Parse(context.Background(), `throw`, nil)
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "throw statement requires a value")
}

func TestContinueBreak(t *testing.T) {
	tests := []struct {
		input   string
		keyword string
	}{
		{`function f() { continue }`, "continue"},
		{`function f() { break }`, "break"},
	}

	for _, tt := range tests {
		t.Run(tt.keyword, func(t *testing.T) {
			program, err := Parse(context.Background(), tt.input, nil)
			assert.Nil(t, err)
			assert.Len(t, program.Stmts, 1)
		})
	}
}

func TestEmptyBlock(t *testing.T) {
	tests := []struct {
		input string
	}{
		{`if (true) {}`},
		{`function f() {}`},
		{`if (true) {} else {}`},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			_, err := Parse(context.Background(), tt.input, nil)
			assert.Nil(t, err)
		})
	}
}

func TestBlockAST(t *testing.T) {
	program, err := Parse(context.Background(), "if (true) { x; y; z }", nil)
	assert.Nil(t, err)

	ifExpr, ok := program.First().(*ast.If)
	assert.True(t, ok)

	// Verify block AST
	block := ifExpr.Consequence
	assert.NotNil(t, block)
	assert.Len(t, block.Stmts, 3)

	for i, name := range []string{"x", "y", "z"} {
		ident, ok := block.Stmts[i].(*ast.Ident)
		assert.True(t, ok)
		assert.Equal(t, name, ident.Name)
	}
}
