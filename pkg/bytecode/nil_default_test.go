package bytecode

import (
	"testing"

	"github.com/deepnoodle-ai/risor/v2/pkg/op"
	"github.com/deepnoodle-ai/wonton/assert"
)

func nilDefaultFunction() (*Function, *Code) {
	fnCode := NewCode(CodeParams{ID: "main.0", Instructions: []op.Code{op.Nil, op.ReturnValue}})
	fn := NewFunction(FunctionParams{
		ID:         "1",
		Name:       "f",
		Parameters: []string{"a", "b", "c"},
		Defaults:   []any{nil, NilDefault{}, int64(2)},
		Code:       fnCode,
	})
	return fn, fnCode
}

// A null default counts as a default, unlike a nil entry, which means the
// parameter has none.
func TestNilDefaultCountsAsDefault(t *testing.T) {
	fn, _ := nilDefaultFunction()
	assert.Equal(t, fn.RequiredArgsCount(), 1)
	assert.Nil(t, fn.Default(0))
	assert.Equal(t, fn.Default(1), any(NilDefault{}))
	assert.Contains(t, fn.String(), "(a, b=null, c=2)")
}

func TestNilDefaultMarshalRoundTrip(t *testing.T) {
	fn, fnCode := nilDefaultFunction()
	code := NewCode(CodeParams{ID: "main", Constants: []any{fn}, Children: []*Code{fnCode}})
	data, err := Marshal(code)
	assert.Nil(t, err)
	assert.Contains(t, string(data), `"nil_default"`)

	restored, err := Unmarshal(data)
	assert.Nil(t, err)
	restoredFn := restored.ConstantAt(0).(*Function)
	assert.Nil(t, restoredFn.Default(0))
	assert.Equal(t, restoredFn.Default(1), any(NilDefault{}))
	assert.Equal(t, restoredFn.RequiredArgsCount(), 1)
}

// Validate accepts NilDefault as a default, and only there.
func TestValidateNilDefault(t *testing.T) {
	fn, fnCode := nilDefaultFunction()
	code := NewCode(CodeParams{
		ID:           "main",
		Children:     []*Code{fnCode},
		Instructions: []op.Code{op.LoadConst, 0, op.PopTop},
		Constants:    []any{fn},
	})
	assert.Nil(t, Validate(code))

	err := Validate(NewCode(CodeParams{Constants: []any{NilDefault{}}}))
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "unsupported type bytecode.NilDefault")
}
