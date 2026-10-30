package vm

import (
	"context"
	"testing"

	"github.com/deepnoodle-ai/risor/v2/pkg/bytecode"
	"github.com/deepnoodle-ai/risor/v2/pkg/compiler"
	"github.com/deepnoodle-ai/risor/v2/pkg/object"
	"github.com/deepnoodle-ai/risor/v2/pkg/parser"
	"github.com/deepnoodle-ai/wonton/assert"
)

// A parameter whose default is null is optional, and defaults to null.
func TestNilDefaultParameters(t *testing.T) {
	runTests(t, []testCase{
		{`function g(a = nil) { return a }; g()`, object.Nil},
		{`function g(a = null) { return a }; g()`, object.Nil},
		{`function g(a = nil) { return a }; g(3)`, object.NewInt(3)},
		{`function f(a = 1, b = nil, c = 2) { return [a, b, c] }; f(10)`, object.NewList([]object.Object{
			object.NewInt(10), object.Nil, object.NewInt(2),
		})},
		{`function f(a, b = nil) { return [a, b] }; f(1)`, object.NewList([]object.Object{
			object.NewInt(1), object.Nil,
		})},
		{`function f(a, b = nil, ...rest) { return [a, b, rest] }; f(1)`, object.NewList([]object.Object{
			object.NewInt(1), object.Nil, object.NewList([]object.Object{}),
		})},
		{`let f = (a = nil) => a; f()`, object.Nil},
	})
}

func TestNilDefaultStillRequiresEarlierArgs(t *testing.T) {
	_, err := run(context.Background(), `function f(a, b = nil) { return a }; f()`)
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "takes 2 arguments (0 given)")
}

// A null default survives marshaling, both of compiled bytecode and of the
// compiler's own code.
func TestNilDefaultMarshalRoundTrip(t *testing.T) {
	ctx := context.Background()
	source := `function f(a = 1, b = nil) { return [a, b] }; f()`
	ast, err := parser.Parse(ctx, source, nil)
	assert.Nil(t, err)
	c, err := compiler.New(nil)
	assert.Nil(t, err)
	code, err := c.CompileAST(ast)
	assert.Nil(t, err)

	want := object.NewList([]object.Object{object.NewInt(1), object.Nil})

	fn, ok := code.ToBytecode().ConstantAt(0).(*bytecode.Function)
	assert.True(t, ok)
	assert.Equal(t, fn.RequiredArgsCount(), 0)
	assert.Contains(t, fn.String(), "(a=1, b=null)")
	assert.Contains(t, object.NewClosure(fn).Inspect(), "(a=1, b=null)")

	data, err := bytecode.Marshal(code.ToBytecode())
	assert.Nil(t, err)
	restored, err := bytecode.Unmarshal(data)
	assert.Nil(t, err)
	assert.Nil(t, bytecode.Validate(restored))
	result, err := Run(ctx, restored)
	assert.Nil(t, err)
	assert.Equal(t, result, want)

	data, err = compiler.MarshalCode(code)
	assert.Nil(t, err)
	restoredCode, err := compiler.UnmarshalCode(data)
	assert.Nil(t, err)
	result, err = Run(ctx, restoredCode.ToBytecode())
	assert.Nil(t, err)
	assert.Equal(t, result, want)
}
