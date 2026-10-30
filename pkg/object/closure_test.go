package object

import (
	"testing"

	"github.com/deepnoodle-ai/risor/v2/pkg/bytecode"
	"github.com/deepnoodle-ai/wonton/assert"
)

// NewClosure turns a null default into Nil and counts it as a default. A nil
// entry stays nil, because that parameter has no default.
func TestNewClosureNilDefault(t *testing.T) {
	fn := bytecode.NewFunction(bytecode.FunctionParams{
		Name:       "f",
		Parameters: []string{"a", "b", "c"},
		Defaults:   []any{nil, bytecode.NilDefault{}, int64(2)},
		Code:       bytecode.NewCode(bytecode.CodeParams{}),
	})
	closure := NewClosure(fn)
	assert.Equal(t, closure.RequiredArgsCount(), 1)
	assert.Nil(t, closure.Default(0))
	assert.Equal(t, closure.Default(1), Object(Nil))
	assert.Equal(t, closure.Default(2), Object(NewInt(2)))
	assert.Contains(t, closure.Inspect(), "(a, b=null, c=2)")
}
