package compiler

import (
	"context"
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/deepnoodle-ai/risor/v2/pkg/op"
	"github.com/deepnoodle-ai/risor/v2/pkg/parser"
	"github.com/deepnoodle-ai/wonton/assert"
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

func TestMarshalCodeRoundTripsAllFields(t *testing.T) {
	program, err := parser.Parse(context.Background(), `
	function f(a, ...rest) { return rest }
	let [_, b] = [1, 2]
	try { throw "x" } catch e { b = 3 } finally { b = 4 }
	f(1, 2)
	`, nil)
	assert.Nil(t, err)
	c, err := New(&Config{GlobalNames: []string{"host"}, Filename: "main.risor"})
	assert.Nil(t, err)
	codeA, err := c.CompileAST(program)
	assert.Nil(t, err)
	assert.True(t, len(codeA.exceptionHandlers) > 0)
	assert.Equal(t, codeA.envKeys, []string{"host"})

	data, err := MarshalCode(codeA)
	assert.Nil(t, err)
	codeB, err := UnmarshalCode(data)
	assert.Nil(t, err)
	assert.Equal(t, codeB, codeA)
	assert.Equal(t, codeB.filename, "main.risor")
	assert.Equal(t, codeB.GlobalNames(), codeA.GlobalNames())
	fn, ok := codeB.constants[0].(*Function)
	assert.True(t, ok)
	assert.Equal(t, fn.RestParam(), "rest")
}

func TestUnmarshalCodeMalformedReturnsError(t *testing.T) {
	valid := `{"id":"main","name":"main","symbol_table_id":""}`
	table := `"symbol_table":{"symbols":[],"symbols_by_name":{}}`
	tests := []struct {
		name string
		data string
		want string
	}{
		{"no code", `{"code":[],` + table + `}`, "no code blocks"},
		{"no symbol table", `{"code":[` + valid + `]}`, "missing symbol table"},
		{"null code block", `{"code":[null],` + table + `}`, "is null"},
		{"root block table", `{"code":[` + valid + `],"symbol_table":{"is_block":true}}`, "root table is a block"},
		{
			"null symbol by name",
			`{"code":[` + valid + `],"symbol_table":{"symbols_by_name":{"a":null}}}`,
			"null symbol",
		},
		{
			"null free variable",
			`{"code":[` + valid + `],"symbol_table":{"free":[{"symbol":null}]}}`,
			"null free variable",
		},
		{
			"root with parent",
			`{"code":[{"id":"main","parent_id":"x","symbol_table_id":""}],` + table + `}`,
			"root code has a parent",
		},
		{
			"second root",
			`{"code":[` + valid + `,{"id":"other","symbol_table_id":""}],` + table + `}`,
			"parent code not found",
		},
		{
			"duplicate code id",
			`{"code":[` + valid + `,{"id":"main","parent_id":"main","symbol_table_id":""}],` + table + `}`,
			"duplicate code id",
		},
		{
			"null function value",
			`{"code":[{"id":"main","symbol_table_id":"","constants":[{"type":"function"}]}],` + table + `}`,
			"missing value",
		},
		{
			"function without code",
			`{"code":[{"id":"main","symbol_table_id":"","constants":[{"type":"function","value":{"id":"f"}}]}],` + table + `}`,
			"has no code block",
		},
		{
			"function code with wrong parent",
			`{"code":[{"id":"main","symbol_table_id":"","function_id":"f",` +
				`"constants":[{"type":"function","value":{"id":"f"}}]}],` + table + `}`,
			"invalid code block",
		},
		{
			"truncated instruction",
			`{"code":[{"id":"main","symbol_table_id":"","instructions":[24]}],` + table + `}`,
			"truncated LOAD_CONST",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, err := UnmarshalCode([]byte(tt.data))
			assert.Nil(t, code)
			assert.NotNil(t, err)
			assert.True(t, strings.Contains(err.Error(), tt.want), err.Error())
		})
	}
}

func TestUnmarshalCodeRejectsTooManyNames(t *testing.T) {
	table := `"symbol_table":{"symbols":[],"symbols_by_name":{}}`
	build := func(n int) string {
		names := make([]string, n)
		for i := range names {
			names[i] = fmt.Sprintf("%q", fmt.Sprintf("n%d", i))
		}
		return `{"code":[{"id":"main","symbol_table_id":"","names":[` +
			strings.Join(names, ",") + `]}],` + table + `}`
	}

	code, err := UnmarshalCode([]byte(build(math.MaxUint16)))
	assert.Nil(t, err)
	assert.Len(t, code.names, math.MaxUint16)

	code, err = UnmarshalCode([]byte(build(math.MaxUint16 + 1)))
	assert.Nil(t, code)
	assert.NotNil(t, err)
	assert.True(t, strings.Contains(err.Error(), "65536 attribute names exceeds the limit of 65535"), err.Error())
}

func TestAddNameDeduplicates(t *testing.T) {
	code, err := compileSource(`
	let o = {a: 1, b: 2}
	o.a + o.b + o.a
	o.a = 3
	`)
	assert.Nil(t, err)
	assert.Equal(t, code.names, []string{"a", "b"})
}

func TestAddNameLimit(t *testing.T) {
	c := &Code{}
	for i := 0; i < math.MaxUint16; i++ {
		idx, err := c.addName(fmt.Sprintf("n%d", i))
		assert.Nil(t, err)
		assert.Equal(t, int(idx), i)
	}
	// A name already in the table is still found
	idx, err := c.addName("n7")
	assert.Nil(t, err)
	assert.Equal(t, idx, uint16(7))
	// A new name does not fit
	_, err = c.addName("one-too-many")
	assert.NotNil(t, err)
	assert.Equal(t, len(c.names), math.MaxUint16)
}

func TestCompilerInstructionIterInvalidInstructions(t *testing.T) {
	iter := NewInstructionIter(&Code{instructions: []op.Code{op.Nop, op.LoadConst}})
	assert.Equal(t, iter.All(), [][]op.Code{{op.Nop}})
	assert.Equal(t, iter.Err().Error(), "truncated LOAD_CONST instruction at offset 1")

	iter = NewInstructionIter(&Code{instructions: []op.Code{op.Code(256)}})
	assert.Equal(t, len(iter.All()), 0)
	assert.Equal(t, iter.Err().Error(), "unknown opcode 256 at offset 0")
}
