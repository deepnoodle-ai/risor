package risor

import (
	"context"
	"errors"
	"testing"

	"github.com/deepnoodle-ai/risor/v2/pkg/object"
	"github.com/deepnoodle-ai/wonton/assert"
)

// End-to-end tests for audit findings C-02 (cyclic containers) and C-03 (map
// changes during iteration).

func TestEvalCyclicResult(t *testing.T) {
	ctx := context.Background()
	for _, src := range []string{
		`let a = []; a.append(a); a`,
		`let m = {}; m["self"] = m; m`,
		`let a = []; let m = {list: a}; a.append(m); a`,
	} {
		_, err := Eval(ctx, src)
		assert.True(t, errors.Is(err, object.ErrNestingDepth), src)
	}

	// The raw result is still available
	result, err := Eval(ctx, `let a = []; a.append(a); a`, WithRawResult())
	assert.Nil(t, err)
	assert.Equal(t, result.(object.Object).Inspect(), "[[...]]")
}

func TestEvalCyclicCompare(t *testing.T) {
	ctx := context.Background()

	result, err := Eval(ctx, `let a = []; a.append(a); a == a`)
	assert.Nil(t, err)
	assert.Equal(t, result, false)

	result, err = Eval(ctx, `let a = []; let m = {list: a}; a.append(m); a == a`)
	assert.Nil(t, err)
	assert.Equal(t, result, false)

	_, err = Eval(ctx, `let a = []; a.append(a); a < a`)
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "maximum nesting depth")
}

func TestEvalMapDeleteDuringIteration(t *testing.T) {
	ctx := context.Background()
	env := Builtins()

	result, err := Eval(ctx, `
		let m = {a: 1, b: 2}
		filter(m.entries(), function(pair) {
			m.clear()
			return true
		})`, WithEnv(env))
	assert.Nil(t, err)
	assert.Equal(t, result, []any{[]any{"a", int64(1)}})

	result, err = Eval(ctx, `
		let m = {a: 1, b: 2, c: 3}
		let seen = []
		m.each(function(k, v) {
			seen.append(v)
			m.pop("b")
		})
		seen`, WithEnv(env))
	assert.Nil(t, err)
	assert.Equal(t, result, []any{int64(1), int64(3)})
}
