package compiler

import (
	"context"
	"encoding/json"
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

// nestedIDsSource has sibling functions, a function declared in a block, and
// nested arrows, so code IDs, parents and symbol tables are not all on one
// path.
const nestedIDsSource = `function a() { function b() { return 1 }; return b() }
function c() { return 2 }
let r = 0
if (true) {
	function d(x) {
		let g = y => z => x + y + z
		return g(1)(2)
	}
	r = d(3)
}
a() + c() + r
`

// editState decodes data from MarshalCode, calls edit on it, and encodes the
// result.
func editState(t *testing.T, data []byte, edit func(codes []map[string]any, table map[string]any)) []byte {
	t.Helper()
	var state struct {
		Code        []map[string]any `json:"code"`
		SymbolTable map[string]any   `json:"symbol_table"`
	}
	assert.Nil(t, json.Unmarshal(data, &state))
	edit(state.Code, state.SymbolTable)
	edited, err := json.Marshal(state)
	assert.Nil(t, err)
	return edited
}

// MarshalCode writes the ID of the root code and the root symbol table only.
// Codes refer to their parent and symbol table by index. UnmarshalCode
// derives the same IDs, and a second round trip gives the same data.
func TestMarshalCodeWritesIndexesForIDs(t *testing.T) {
	codeA, err := compileSource(nestedIDsSource)
	assert.Nil(t, err)
	data, err := MarshalCode(codeA)
	assert.Nil(t, err)

	editState(t, data, func(codes []map[string]any, table map[string]any) {
		assert.Equal(t, len(codes), 7)
		for i, def := range codes {
			_, hasID := def["id"]
			assert.Equal(t, hasID, i == 0)
			_, hasParent := def["parent_index"]
			assert.Equal(t, hasParent, i > 0)
			assert.NotNil(t, def["symbol_table_index"])
			assert.Nil(t, def["parent_id"])
			assert.Nil(t, def["symbol_table_id"])
		}
		assert.Equal(t, codes[0]["id"], "__main__")
		assert.Equal(t, table["id"], "root")
		var hasChildID func(table map[string]any) bool
		hasChildID = func(table map[string]any) bool {
			children, _ := table["children"].([]any)
			for _, child := range children {
				child := child.(map[string]any)
				if _, ok := child["id"]; ok || hasChildID(child) {
					return true
				}
			}
			return false
		}
		assert.False(t, hasChildID(table))
	})

	codeB, err := UnmarshalCode(data)
	assert.Nil(t, err)
	assert.Equal(t, codeB, codeA)
	again, err := MarshalCode(codeB)
	assert.Nil(t, err)
	assert.Equal(t, string(again), string(data))
}

// Older versions wrote every code and symbol table ID, and referred to the
// parent and the symbol table by ID. UnmarshalCode still reads them.
func TestUnmarshalCodeOldFormatIDs(t *testing.T) {
	codeA, err := compileSource(nestedIDsSource)
	assert.Nil(t, err)
	data, err := MarshalCode(codeA)
	assert.Nil(t, err)

	all := codeA.Flatten()
	old := editState(t, data, func(codes []map[string]any, table map[string]any) {
		for i, def := range codes {
			def["id"] = all[i].ID()
			if i > 0 {
				def["parent_id"] = all[i].parent.ID()
			}
			def["symbol_table_id"] = all[i].symbols.ID()
			delete(def, "parent_index")
			delete(def, "symbol_table_index")
		}
		var setIDs func(def map[string]any, table *SymbolTable)
		setIDs = func(def map[string]any, table *SymbolTable) {
			def["id"] = table.ID()
			children, _ := def["children"].([]any)
			for j, child := range children {
				setIDs(child.(map[string]any), table.children[j])
			}
		}
		setIDs(table, codeA.symbols)
	})
	assert.True(t, strings.Contains(string(old), `"parent_id":"__main__.2.0"`))
	assert.True(t, strings.Contains(string(old), `"symbol_table_id":"root.2.0.0.0"`))
	assert.False(t, strings.Contains(string(old), `"parent_index"`))

	codeB, err := UnmarshalCode(old)
	assert.Nil(t, err)
	assert.Equal(t, describeCodes(codeB), describeCodes(codeA))
	assert.Equal(t, describeTables(codeB.symbols), describeTables(codeA.symbols))
	assert.Equal(t, codeB.ToBytecode().Flatten()[6].ID(), "__main__.2.0.0")
}

// describeCodes describes each code in code, in Flatten order, by its ID, its
// parent's ID, its function ID, and its symbol table's ID.
func describeCodes(code *Code) []string {
	var out []string
	for _, c := range code.Flatten() {
		parent := "-"
		if c.parent != nil {
			parent = c.parent.ID()
		}
		out = append(out, fmt.Sprintf("%s parent=%s fn=%q table=%s", c.ID(), parent, c.functionID, c.symbols.ID()))
	}
	return out
}

// describeTables describes each table in table, in pre-order, by its ID and
// whether it is a block.
func describeTables(table *SymbolTable) []string {
	var out []string
	for _, t := range flattenSymbolTables(table, nil) {
		out = append(out, fmt.Sprintf("%s block=%v", t.ID(), t.isBlock))
	}
	return out
}

// Compiled code stores the ID of the root code and the root symbol table
// only. Every other ID is derived from the parent's ID and the position, so
// IDs do not use memory that grows with the square of the nesting depth.
func TestCompiledChildIDsAreDerived(t *testing.T) {
	code, err := compileSource(nestedIDsSource)
	assert.Nil(t, err)
	assert.Equal(t, describeCodes(code), []string{
		`__main__ parent=- fn="" table=root`,
		`__main__.0 parent=__main__ fn="1" table=root.0`,
		`__main__.0.0 parent=__main__.0 fn="2" table=root.0.0.0`,
		`__main__.1 parent=__main__ fn="3" table=root.1`,
		`__main__.2 parent=__main__ fn="4" table=root.2.0`,
		`__main__.2.0 parent=__main__.2 fn="5" table=root.2.0.0.0`,
		`__main__.2.0.0 parent=__main__.2.0 fn="6" table=root.2.0.0.0.0.0`,
	})
	for i, c := range code.Flatten() {
		assert.Equal(t, c.id == "", i > 0)
	}
	for i, table := range flattenSymbolTables(code.symbols, nil) {
		assert.Equal(t, table.id == "", i > 0)
	}
	var bytecodeIDs []string
	for _, c := range code.ToBytecode().Flatten() {
		bytecodeIDs = append(bytecodeIDs, c.ID())
	}
	assert.Equal(t, bytecodeIDs, []string{
		"__main__", "__main__.0", "__main__.0.0", "__main__.1",
		"__main__.2", "__main__.2.0", "__main__.2.0.0",
	})

	// Converting a child gives a root with the child's full ID.
	child := code.Flatten()[5].ToBytecode()
	assert.Equal(t, child.ID(), "__main__.2.0")
	assert.Equal(t, child.ChildAt(0).ID(), "__main__.2.0.0")
}

// UnmarshalCode rejects a parent or symbol table index that does not refer to
// an earlier code or an existing table, a code with both an index and an ID
// for the same reference, and duplicate stored code IDs.
func TestUnmarshalCodeRejectsBadIndexes(t *testing.T) {
	code, err := compileSource(nestedIDsSource)
	assert.Nil(t, err)
	data, err := MarshalCode(code)
	assert.Nil(t, err)
	tableCount := len(flattenSymbolTables(code.symbols, nil))

	set := func(i int, key string, value any) func([]map[string]any, map[string]any) {
		return func(codes []map[string]any, table map[string]any) {
			codes[i][key] = value
		}
	}
	tests := []struct {
		name string
		edit func([]map[string]any, map[string]any)
		want string
	}{
		{"root with parent index", set(0, "parent_index", 0), "root code has a parent index"},
		{"negative parent index", set(2, "parent_index", -1), "want a code block before it"},
		{"parent index of itself", set(2, "parent_index", 2), "want a code block before it"},
		{"parent index after it", set(2, "parent_index", 3), "want a code block before it"},
		{"parent index past the end", set(2, "parent_index", 100), "want a code block before it"},
		{"parent index and parent id", set(2, "parent_id", "__main__.0"), "both parent index and parent id"},
		{"no parent", func(codes []map[string]any, table map[string]any) {
			delete(codes[2], "parent_index")
		}, "parent code not found"},
		{"function code under the wrong parent", set(2, "parent_index", 0), "invalid code block"},
		{"negative symbol table index", set(1, "symbol_table_index", -1), "symbol table index -1"},
		{"symbol table index past the end", set(1, "symbol_table_index", tableCount), "symbol table index"},
		{"symbol table index and id", set(1, "symbol_table_id", "root.0"), "both symbol table index and symbol table id"},
		{"id of the root", set(3, "id", "__main__"), "duplicate code id"},
		{"id of another child", func(codes []map[string]any, table map[string]any) {
			codes[1]["id"] = "x"
			codes[3]["id"] = "x"
		}, "duplicate code id"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, err := UnmarshalCode(editState(t, data, tt.edit))
			assert.Nil(t, code)
			assert.NotNil(t, err)
			assert.True(t, strings.Contains(err.Error(), tt.want), err.Error())
		})
	}
}
