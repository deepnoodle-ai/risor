package risor

import (
	"context"
	"errors"
	"testing"

	"github.com/deepnoodle-ai/risor/v2/pkg/object"
	"github.com/deepnoodle-ai/wonton/assert"
)

// End-to-end tests for cyclic containers and for map changes during
// iteration.

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

// Builtins that convert their arguments to Go values report the nesting error
// instead of a misleading "not supported" error or a "<nil>" in the output.
func TestEvalCyclicBuiltinArgs(t *testing.T) {
	ctx := context.Background()
	for _, src := range []string{
		`let a = []; a.append(a); encode(a, "json")`,
		`let a = []; a.append(a); sprintf("%v", a)`,
		`let a = []; a.append(a); error("bad: %v", a)`,
	} {
		_, err := Eval(ctx, src, WithEnv(Builtins()))
		assert.True(t, errors.Is(err, object.ErrNestingDepth), src)
	}
}

// A map that holds an iterator over itself is a cycle when converted, encoded,
// or formatted. Printing and comparing it do not visit the iterator's values.
func TestEvalCyclicIterator(t *testing.T) {
	ctx := context.Background()
	env := Builtins()
	for _, src := range []string{
		`let m = {}; m["v"] = m.values(); m`,
		`let m = {}; m["e"] = m.entries(); m`,
		`let m = {}; m["v"] = m.values(); m["v"]`,
		`let m = {}; let a = [m.values()]; m["a"] = a; a`,
		`let m = {}; m["v"] = m.values(); encode(m, "json")`,
		`let m = {}; m["v"] = m.values(); sprintf("%v", m)`,
		`let m = {}; m["v"] = m.values(); error("bad: %v", m)`,
	} {
		_, err := Eval(ctx, src, WithEnv(env))
		assert.True(t, errors.Is(err, object.ErrNestingDepth), src)
	}

	result, err := Eval(ctx, `let m = {}; m["v"] = m.values(); m == m`)
	assert.Nil(t, err)
	assert.Equal(t, result, true)

	result, err = Eval(ctx, `let m = {}; m["v"] = m.values(); m`, WithRawResult())
	assert.Nil(t, err)
	assert.Equal(t, result.(object.Object).Inspect(), `{"v": iter(map.values)}`)
}
