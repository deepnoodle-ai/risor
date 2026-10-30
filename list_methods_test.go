package risor

import (
	"context"
	"testing"

	"github.com/deepnoodle-ai/wonton/assert"
)

func TestFilterBuiltinCallbackShrinksList(t *testing.T) {
	// The callback pops from the list being filtered. filter() follows the
	// same iteration rules as list.filter(): it reads the item currently at
	// each index and stops once the list is shorter than the index.
	result, err := Eval(context.Background(), `
		let ls = [1, 2, 3]
		filter(ls, x => {
			ls.pop(0)
			return true
		})
	`, WithEnv(Builtins()))
	assert.Nil(t, err)
	assert.Equal(t, result, []any{int64(1), int64(3)})
}

func TestListMapCallbackArity(t *testing.T) {
	ctx := context.Background()
	cases := []struct {
		src  string
		want any
	}{
		// Optional parameters don't make map() pass the index
		{`[10, 20, 30].map(function(x, mult = 2) { return x * mult })`, []any{int64(20), int64(40), int64(60)}},
		{`[1, 2].map(function(x, y = 10, z = 20) { return x + y + z })`, []any{int64(31), int64(32)}},
		{`range(1, 3).map(function(x, mult = 2) { return x * mult })`, []any{int64(2), int64(4)}},
		// Rest parameters receive the value
		{`[1, 2].map(function(...args) { return args[0] * 2 })`, []any{int64(2), int64(4)}},
		{`range(2).map(function(...args) { return len(args) })`, []any{int64(1), int64(1)}},
		{`[5, 6].map(function(x, ...rest) { return [x, len(rest)] })`, []any{[]any{int64(5), int64(0)}, []any{int64(6), int64(0)}}},
		// Exactly two required parameters receive (index, value)
		{`["a", "b"].map((i, x) => [i, x])`, []any{[]any{int64(0), "a"}, []any{int64(1), "b"}}},
		{`range(5, 7).map((i, x) => [i, x])`, []any{[]any{int64(0), int64(5)}, []any{int64(1), int64(6)}}},
		{`["a"].map(function(i, x, ...rest) { return [i, x] })`, []any{[]any{int64(0), "a"}}},
	}
	for _, tc := range cases {
		result, err := Eval(ctx, tc.src, WithEnv(Builtins()))
		assert.Nil(t, err, tc.src)
		assert.Equal(t, result, tc.want, tc.src)
	}

	for _, src := range []string{
		`[1].map(function() { return 1 })`,
		`[1].map(function(a, b, c) { return 1 })`,
		`range(1).map(function() { return 1 })`,
		`range(1).map(function(a, b, c) { return 1 })`,
	} {
		_, err := Eval(ctx, src, WithEnv(Builtins()))
		assert.NotNil(t, err, src)
		assert.Contains(t, err.Error(), "incompatible function", src)
	}
}

func TestListIndexErrorsAreValueErrors(t *testing.T) {
	result, err := Eval(context.Background(), `
		let ls = [1, 2]
		[
			try { ls[5] } catch e { e.kind() },
			try { ls[5] = 1 } catch e { e.kind() },
			try { ls[-3] = 1 } catch e { e.kind() },
		]
	`, WithEnv(Builtins()))
	assert.Nil(t, err)
	assert.Equal(t, result, []any{"value error", "value error", "value error"})
}
