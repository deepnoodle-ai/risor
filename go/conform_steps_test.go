package risor_test

import (
	"errors"
	"testing"

	risor "github.com/deepnoodle-ai/risor/go/v3"
	"github.com/deepnoodle-ai/risor/go/v3/internal/assert"
)

// F4 7.3 and F8 6.4: the conformance function and ConformAll count steps
// against a step limit. Host data that shares one list at every level has
// 2^levels places; each is charged, so the limit ends the walk (#678).

// sharedLists is a list whose two elements are the same inner list, at
// each of n levels, with leaf at the bottom.
func sharedLists(n int, leaf any) any {
	v := []any{leaf, leaf}
	for i := 1; i < n; i++ {
		v = []any{v, v}
	}
	return v
}

func nestedListType(n int) risor.Type {
	t := risor.TypeInt
	for i := 0; i < n; i++ {
		t = risor.ListType(t)
	}
	return t
}

// The walk over 2^31 - 1 shared places stops at the limit. An unmetered walk
// would not end, so the test's own timeout guards against a hang; no clock
// is needed. The limit is low so that the test is fast on any runner.
func TestConformSharedDataIsBounded(t *testing.T) {
	const levels = 30
	typ := nestedListType(levels)
	limit := risor.ConformSteps(10_000)
	v := sharedLists(levels, 1)
	err := risor.Conform(v, typ, limit)
	var e *risor.Error
	assert.True(t, errors.As(err, &e))
	d := e.Diagnostics[0]
	assert.Equal(t, d.Phase, risor.PhaseRun)
	assert.Equal(t, d.Code, "steps-exceeded")
	assert.Equal(t, d.Class, "limit")
	assert.True(t, d.Path == nil && d.Span == nil)
	assert.Equal(t, d.Message, "the run needed more than its limit of 10000 steps")
	all := risor.ConformAll(v, typ, 5, limit)
	assert.Equal(t, len(all), 1)
	assert.Equal(t, all[0].Code, "steps-exceeded")
	// A failing leaf: the first leaf fails, long before the limit.
	v = sharedLists(levels, "x")
	err = risor.Conform(v, typ, limit)
	assert.True(t, errors.As(err, &e))
	assert.Equal(t, e.Diagnostics[0].Code, "not-conforming")
	assert.Equal(t, len(e.Diagnostics[0].Path), levels)
	all = risor.ConformAll(v, typ, 5, limit)
	assert.Equal(t, len(all), 5)
	assert.Equal(t, all[4].Code, "not-conforming")
}

func TestConformStepsOption(t *testing.T) {
	typ := nestedListType(10)
	v := sharedLists(10, 1)
	// 2^11 - 1 places, each charged 1.
	const count = 2047
	assert.NoError(t, risor.Conform(v, typ, risor.ConformSteps(count)))
	err := risor.Conform(v, typ, risor.ConformSteps(count-1))
	var e *risor.Error
	assert.True(t, errors.As(err, &e))
	assert.Equal(t, e.Diagnostics[0].Code, "steps-exceeded")
	assert.Equal(t, e.Diagnostics[0].Message, "the run needed more than its limit of 2046 steps")
	// NoLimit turns it off; another number below 0 is read as 0.
	assert.NoError(t, risor.Conform(sharedLists(16, 1), nestedListType(16), risor.ConformSteps(risor.NoLimit)))
	err = risor.Conform(1, risor.TypeInt, risor.ConformSteps(-5))
	assert.True(t, errors.As(err, &e))
	assert.Equal(t, e.Diagnostics[0].Message, "the run needed more than its limit of 0 steps")
	assert.NoError(t, risor.Conform(1, risor.TypeAny, risor.ConformSteps(0)))
	// The environment's step limit plays no part (R322).
	env, err := risor.NewEnv(risor.Limits{Steps: 10})
	assert.NoError(t, err)
	big, err := env.ParseType("int[]")
	assert.NoError(t, err)
	assert.NoError(t, risor.Conform(make([]any, 100), risor.ListType(risor.UnionType(risor.TypeInt, risor.TypeNull))))
	assert.NoError(t, risor.Conform([]any{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12}, big))
}

func TestConformAllEndsWithTheLimit(t *testing.T) {
	// The first walk fails at [1] with 3 steps; All charges 4 and passes a
	// limit of 3 after its first failure (F8 6.4 step 3).
	typ := risor.ListType(risor.TypeInt)
	v := []any{1, "a", "b"}
	ds := risor.ConformAll(v, typ, 5, risor.ConformSteps(3))
	assert.Equal(t, len(ds), 2)
	assert.Equal(t, ds[0].Code, "not-conforming")
	assert.Equal(t, ds[0].Path, []any{1.0})
	assert.Equal(t, ds[1].Code, "steps-exceeded")
	assert.Equal(t, ds[1].Class, "limit")
	assert.True(t, ds[1].Path == nil)
	// With room for one failure, All stops before the limit.
	ds = risor.ConformAll(v, typ, 1, risor.ConformSteps(3))
	assert.Equal(t, len(ds), 1)
	assert.Equal(t, ds[0].Code, "not-conforming")
	// The maximum bounds the list, the limit diagnostic included.
	ds = risor.ConformAll([]any{"a", "b", "c", "d"}, typ, 2, risor.ConformSteps(2))
	assert.Equal(t, len(ds), 2)
	assert.Equal(t, ds[0].Path, []any{0.0})
	assert.Equal(t, ds[1].Code, "steps-exceeded")
}
