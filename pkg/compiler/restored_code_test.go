package compiler

import (
	"context"
	"strings"
	"testing"

	"github.com/deepnoodle-ai/risor/v2/pkg/parser"
	"github.com/deepnoodle-ai/wonton/assert"
)

// Compiling into code restored by UnmarshalCode must not reuse the IDs of
// the functions already in it.
func TestCompileIntoRestoredCodeKeepsFunctionIDsUnique(t *testing.T) {
	ctx := context.Background()

	c1, err := New(nil)
	assert.Nil(t, err)
	ast1, err := parser.Parse(ctx, "function f() { return function() { return 1 } }", nil)
	assert.Nil(t, err)
	_, err = c1.CompileAST(ast1)
	assert.Nil(t, err)
	data1, err := MarshalCode(c1.Code())
	assert.Nil(t, err)

	restored, err := UnmarshalCode(data1)
	assert.Nil(t, err)
	c2, err := New(&Config{Code: restored})
	assert.Nil(t, err)
	ast2, err := parser.Parse(ctx, "function g() { return 2 }", nil)
	assert.Nil(t, err)
	_, err = c2.CompileAST(ast2)
	assert.Nil(t, err)

	ids := map[string]bool{}
	for _, code := range c2.Code().Flatten() {
		if id := code.FunctionID(); id != "" {
			assert.False(t, ids[id], "duplicate function id %q", id)
			ids[id] = true
		}
	}
	assert.Len(t, ids, 3)

	data2, err := MarshalCode(c2.Code())
	assert.Nil(t, err)
	_, err = UnmarshalCode(data2)
	assert.Nil(t, err)
}

// UnmarshalCode rejects a symbol table whose indexed and by-name symbols
// disagree, and code that uses the wrong symbol table.
func TestUnmarshalCodeRejectsInconsistentSymbols(t *testing.T) {
	root := `{"id":"main","name":"main","symbol_table_id":""}`
	tests := []struct {
		name string
		data string
		want string
	}{
		{
			"by-name symbol out of range",
			`{"code":[` + root + `],"symbol_table":{"symbols":[],` +
				`"symbols_by_name":{"x":{"name":"x","index":10}}}}`,
			`symbol "x" does not match the symbol at index 10`,
		},
		{
			"by-name symbol names another symbol",
			`{"code":[` + root + `],"symbol_table":{"symbols":[{"name":"y","index":0}],` +
				`"symbols_by_name":{"x":{"name":"x","index":0}}}}`,
			`symbol "x" does not match the symbol at index 0`,
		},
		{
			"by-name key differs from symbol name",
			`{"code":[` + root + `],"symbol_table":{"symbols":[{"name":"y","index":0}],` +
				`"symbols_by_name":{"x":{"name":"y","index":0}}}}`,
			`symbol "x" does not match the symbol at index 0`,
		},
		{
			"indexed symbol at the wrong position",
			`{"code":[` + root + `],"symbol_table":{"symbols":[{"name":"x","index":3}],` +
				`"symbols_by_name":{}}}`,
			`symbol "x" at position 0 has index 3`,
		},
		{
			"root code uses a block table",
			`{"code":[{"id":"main","symbol_table_id":"b"}],"symbol_table":{"symbols":[],` +
				`"symbols_by_name":{},"children":[{"id":"b","is_block":true,"symbols":[],"symbols_by_name":{}}]}}`,
			"root code does not use the root symbol table",
		},
		{
			"root code uses a child table",
			`{"code":[{"id":"main","symbol_table_id":"f"}],"symbol_table":{"symbols":[],` +
				`"symbols_by_name":{},"children":[{"id":"f","symbols":[],"symbols_by_name":{}}]}}`,
			"root code does not use the root symbol table",
		},
		{
			"function code uses a block table",
			`{"code":[{"id":"main","symbol_table_id":"","constants":[{"type":"function","value":{"id":"1"}}]},` +
				`{"id":"main.0","parent_id":"main","function_id":"1","symbol_table_id":"b"}],` +
				`"symbol_table":{"symbols":[],"symbols_by_name":{},` +
				`"children":[{"id":"b","is_block":true,"symbols":[],"symbols_by_name":{}}]}}`,
			"code block 1 uses a block symbol table",
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

// Block scopes refer to the slots of the enclosing function's table, which
// UnmarshalCode accepts.
func TestUnmarshalCodeAcceptsBlockSymbols(t *testing.T) {
	code, err := compileSource(`
	let a = 1
	if (true) { let b = 2 }
	function f(x) { if (x) { let c = 3; return c }; if (true) { let d = 4; return d } }
	`)
	assert.Nil(t, err)
	data, err := MarshalCode(code)
	assert.Nil(t, err)
	_, err = UnmarshalCode(data)
	assert.Nil(t, err)
}
