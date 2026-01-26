package compiler

import (
	"testing"

	"github.com/deepnoodle-ai/wonton/assert"
)

func TestTable(t *testing.T) {
	table := NewSymbolTable()

	assert.Nil(t, table.Parent())
	assert.Equal(t, table.Count(), uint16(0))

	a, err := table.InsertVariable("a")
	assert.Nil(t, err)
	assert.Equal(t, a.Index(), uint16(0))
	assert.Equal(t, a.Name(), "a")
	assert.Nil(t, a.Value())

	b, err := table.InsertVariable("b")
	assert.Nil(t, err)
	assert.Equal(t, b.Index(), uint16(1))
	assert.Equal(t, b.Name(), "b")
	assert.Nil(t, b.Value())

	c, err := table.InsertVariable("c")
	assert.Nil(t, err)
	assert.Equal(t, c.Index(), uint16(2))
	assert.Equal(t, c.Name(), "c")
	assert.Nil(t, c.Value())

	// The size is the count of variables
	assert.Equal(t, table.Count(), uint16(3))

	assert.True(t, table.IsDefined("a"))
	assert.True(t, table.IsDefined("b"))
	assert.True(t, table.IsDefined("c"))
}

func TestBlock(t *testing.T) {
	table := NewSymbolTable()
	block := table.NewBlock()

	block.InsertVariable("a", 42)

	assert.Equal(t, table.Count(), uint16(1))
	assert.Equal(t, table.Symbol(0).Value(), 42)
}

func TestFunctionID(t *testing.T) {
	table := NewSymbolTable()  // root
	block := table.NewBlock()  // root.0
	fn1 := block.NewChild()    // root.0.0
	fn1Block := fn1.NewBlock() // root.0.0.0
	fn2 := fn1Block.NewChild() // root.0.0.0.0
	fn2Block := fn2.NewBlock() // root.0.0.0.0.0

	assert.Equal(t, fn2Block.ID(), "root.0.0.0.0.0")

	// The function ID of a block corresponds to its enclosing function
	fnID, ok := fn2Block.GetFunctionID()
	assert.True(t, ok)
	assert.Equal(t, fnID, "root.0.0.0.0")

	fnID, ok = fn1Block.GetFunctionID()
	assert.True(t, ok)
	assert.Equal(t, fnID, "root.0.0")
}

func TestFreeVar(t *testing.T) {
	main := NewSymbolTable()
	outerFunc := main.NewChild()
	innerFunc := outerFunc.NewChild()

	outerFunc.InsertVariable("a", 42)

	_, found := innerFunc.Resolve("whut")
	assert.False(t, found)

	res, found := innerFunc.Resolve("a")
	assert.True(t, found)

	exp := &Resolution{
		symbol: &Symbol{
			name:  "a",
			index: 0,
			value: 42,
		},
		scope: Free,
		depth: 1,
	}
	assert.Equal(t, res, exp)

	assert.Equal(t, innerFunc.FreeCount(), uint16(1))
	assert.Equal(t, innerFunc.Free(0), exp)
	assert.Equal(t, outerFunc.FreeCount(), uint16(0))
}

func TestFreeVarWithBlocks(t *testing.T) {
	// Tests that nesting within blocks does not affect the depth of free
	// variables, and that blocks do not allocate free variables.
	main := NewSymbolTable()
	outerFunc := main.NewChild()
	outerBlock := outerFunc.NewBlock()
	innerFunc := outerBlock.NewChild()
	innerBlock := innerFunc.NewBlock()

	outerFunc.InsertVariable("a", 42)

	_, found := innerBlock.Resolve("whut")
	assert.False(t, found)

	res, found := innerBlock.Resolve("a")
	assert.True(t, found)

	exp := &Resolution{
		symbol: &Symbol{
			name:  "a",
			index: 0,
			value: 42,
		},
		scope: Free,
		depth: 1,
	}
	assert.Equal(t, res, exp)
	assert.Equal(t, innerFunc.FreeCount(), uint16(1))
	assert.Equal(t, innerFunc.Free(0), exp)
	assert.Equal(t, outerFunc.FreeCount(), uint16(0))
	assert.Equal(t, outerBlock.FreeCount(), uint16(0))
	assert.Equal(t, innerBlock.FreeCount(), uint16(0))
}

func TestConstant(t *testing.T) {
	main := NewSymbolTable()
	outerFunc := main.NewChild()
	innerFunc := outerFunc.NewChild()

	outerFunc.InsertConstant("a", 42)
	outerFunc.InsertVariable("b", 42)

	resolution, found := innerFunc.Resolve("a")
	assert.True(t, found)
	assert.True(t, resolution.symbol.isConstant)

	resolution, found = innerFunc.Resolve("b")
	assert.True(t, found)
	assert.False(t, resolution.symbol.isConstant)
}
