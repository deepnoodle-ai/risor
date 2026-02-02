package compiler

import (
	"context"
	"fmt"
	"testing"

	"github.com/deepnoodle-ai/wonton/assert"
	"github.com/deepnoodle-ai/risor/v2/op"
	"github.com/deepnoodle-ai/risor/v2/parser"
)

func compileSource(source string) (*Code, error) {
	program, err := parser.Parse(context.Background(), source, nil)
	if err != nil {
		return nil, err
	}
	c, err := New(&Config{GlobalNames: []string{"len", "list", "string", "print"}})
	if err != nil {
		return nil, err
	}
	return c.CompileAST(program)
}

func TestMarshalCode1(t *testing.T) {
	codeA, err := compileSource(`
	let x = 1.0
	let y = 2.0
	x + y
	`)
	assert.Nil(t, err)
	data, err := MarshalCode(codeA)
	assert.Nil(t, err)
	codeB, err := UnmarshalCode(data)
	assert.Nil(t, err)
	assert.Equal(t, codeB, codeA)
}

func TestMarshalCode2(t *testing.T) {
	codeA, err := compileSource(`
	function test(a, b=2) {
		if (a > b) {
			return a
		} else {
			return b
		}
	}
	test(1) + test(2, 3)
	`)
	assert.Nil(t, err)
	data, err := MarshalCode(codeA)
	assert.Nil(t, err)
	codeB, err := UnmarshalCode(data)
	assert.Nil(t, err)
	assert.Equal(t, codeB, codeA)
}

func TestMarshalCode3(t *testing.T) {
	codeA, err := compileSource(`
	let start = 10
	function counter(a) {
		let current = a
		return function() {
			current++
			return current
		}
	}
	let c = counter(start)
	c()
	`)
	assert.Nil(t, err)
	data, err := MarshalCode(codeA)
	assert.Nil(t, err)
	fmt.Println(string(data))
	codeB, err := UnmarshalCode(data)
	assert.Nil(t, err)
	assert.Equal(t, codeB, codeA)
}

func TestSymbolTableDefinition(t *testing.T) {
	table := NewSymbolTable()
	table.InsertVariable("x")
	table.InsertConstant("c")

	def := definitionFromSymbolTable(table)
	symbols := def.Symbols
	assert.Len(t, symbols, 2)

	symbol := symbols[0]
	assert.Equal(t, symbol.Name, "x")
	assert.Equal(t, symbol.IsConstant, false)
	assert.Equal(t, symbol.Index, uint16(0))

	symbol = symbols[1]
	assert.Equal(t, symbol.Name, "c")
	assert.Equal(t, symbol.IsConstant, true)
	assert.Equal(t, symbol.Index, uint16(1))

	newTable, err := symbolTableFromDefinition(def)
	assert.Nil(t, err)
	assert.Equal(t, newTable, table)
}

func TestCodeConstants(t *testing.T) {
	c := Code{symbols: NewSymbolTable()}
	c.constants = append(c.constants, int64(1), 2.0, "three", true, nil)
	data, err := MarshalCode(&c)
	assert.Nil(t, err)
	c2, err := UnmarshalCode(data)
	assert.Nil(t, err)
	assert.Equal(t, c2.constants, c.constants)
}

func TestCompiledInstructions(t *testing.T) {
	code, err := compileSource(`1 + 2`)
	assert.Nil(t, err)
	instrs := NewInstructionIter(code).All()
	assert.Equal(t,

		instrs, [][]op.Code{
			{op.LoadConst, 0},
			{op.LoadConst, 1},
			{op.BinaryOp, op.Code(op.Add)},
		})

	data, err := MarshalCode(code)
	assert.Nil(t, err)

	code2, err := UnmarshalCode(data)
	assert.Nil(t, err)

	instrs = NewInstructionIter(code2).All()
	assert.Equal(t,

		instrs, [][]op.Code{
			{op.LoadConst, 0},
			{op.LoadConst, 1},
			{op.BinaryOp, op.Code(op.Add)},
		})
}
