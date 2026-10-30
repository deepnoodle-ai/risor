package compiler

import (
	"testing"

	"github.com/deepnoodle-ai/risor/v2/pkg/bytecode"
	"github.com/deepnoodle-ai/wonton/assert"
)

// A null default compiles to bytecode.NilDefault, which counts as a default
// and survives MarshalCode and UnmarshalCode.
func TestCompileNilDefault(t *testing.T) {
	code, err := compileSource(`function f(a, b = nil) { return b }`)
	assert.Nil(t, err)
	fn := code.constants[0].(*Function)
	assert.Nil(t, fn.Default(0))
	assert.Equal(t, fn.Default(1), any(bytecode.NilDefault{}))
	assert.Equal(t, fn.RequiredArgsCount(), 1)
	assert.Contains(t, fn.String(), "(a, b=null)")

	data, err := MarshalCode(code)
	assert.Nil(t, err)
	restored, err := UnmarshalCode(data)
	assert.Nil(t, err)
	restoredFn := restored.constants[0].(*Function)
	assert.Nil(t, restoredFn.Default(0))
	assert.Equal(t, restoredFn.Default(1), any(bytecode.NilDefault{}))
}
