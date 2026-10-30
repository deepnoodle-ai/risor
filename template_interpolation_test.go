package risor

import (
	"context"
	"strings"
	"testing"

	"github.com/deepnoodle-ai/risor/v2/pkg/ast"
	"github.com/deepnoodle-ai/risor/v2/pkg/bytecode"
	"github.com/deepnoodle-ai/risor/v2/pkg/compiler"
	"github.com/deepnoodle-ai/risor/v2/pkg/parser"
	"github.com/deepnoodle-ai/wonton/assert"
)

// An interpolation may hold braces and strings that contain braces.
func TestTemplateInterpolationWithBraces(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"`${ {a: 1}[\"a\"] }`", "1"},
		{"`${ if (true) { \"yes\" } else { \"no\" } }`", "yes"},
		{"`${ \"}\" }`", "}"},
		{"`${ '{' }`", "{"},
		{"`${ \"\\\"}\" }`", `"}`},
		{"`<${ [1, 2].map(x => { return x * 2 }) }>`", "<[2, 4]>"},
		{"`${ match 2 { 2 => \"two\", _ => \"other\" } }`", "two"},
		{"`${ try { throw \"e\" } catch e { \"caught\" } }`", "caught"},
		{"`${ {a: {b: 1}}.a.b }-${ 2 }`", "1-2"},
		{"`${ 1 // it's }`", "1"},
		{"`${ 1 /* { */ }`", "1"},
		{"`${ 1 // }`", "1"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result, err := Eval(context.Background(), tt.input, WithEnv(Builtins()))
			assert.Nil(t, err)
			assert.Equal(t, result, tt.expected)
		})
	}
}

// Errors inside an interpolation point at the interpolation in the source,
// not at the start of the file.
func TestTemplateInterpolationErrorPosition(t *testing.T) {
	src := "let a = 1\nlet s = `x\n  ${ a + }`\n"
	_, err := parser.Parse(context.Background(), src, nil)
	assert.NotNil(t, err)
	errs := err.(*parser.Errors).Errors()
	assert.Len(t, errs, 1)
	assert.True(t, strings.HasPrefix(errs[0].Message(), "in template interpolation: "))
	assert.Equal(t, errs[0].StartPosition().Line, 2)
	assert.Equal(t, errs[0].SourceCode(), "  ${ a + }`")

	// A compile error inside an interpolation reports its own line
	_, err = Eval(context.Background(), "let a = 1\n\n`${ nope }`")
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "undefined variable \"nope\"")
	assert.Contains(t, err.Error(), ":3:5")
}

// The AST of an interpolation has source positions.
func TestTemplateInterpolationASTPositions(t *testing.T) {
	src := "// c\nlet s = `é ${ xy }`"
	prog, err := parser.Parse(context.Background(), src, nil)
	assert.Nil(t, err)
	str := prog.Stmts[0].(*ast.Var).Value.(*ast.String)
	assert.Len(t, str.Exprs, 1)
	pos := str.Exprs[0].Pos()
	assert.Equal(t, pos.Line, 1)
	assert.Equal(t, pos.Column, 14)
	assert.Equal(t, string([]rune(src)[pos.Char:pos.Char+2]), "xy")
}

// A function defined in an interpolation gets its own body as its source.
func TestTemplateInterpolationFunctionSource(t *testing.T) {
	src := "// header line 1\n// header line 2\nlet s = `${ [1, 2].map(x => x + 1) }`\n"
	prog, err := parser.Parse(context.Background(), src, nil)
	assert.Nil(t, err)
	code, err := compiler.Compile(prog, &compiler.Config{Source: src})
	assert.Nil(t, err)
	var found bool
	for i := 0; i < code.ConstantCount(); i++ {
		if fn, ok := code.ConstantAt(i).(*bytecode.Function); ok {
			found = true
			assert.Equal(t, fn.String(), "func(x) { x + 1 }")
		}
	}
	assert.True(t, found)
}
