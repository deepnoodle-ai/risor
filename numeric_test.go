package risor

import (
	"context"
	"testing"

	"github.com/deepnoodle-ai/wonton/assert"
)

func TestNumericAndBytesOperators(t *testing.T) {
	ctx := context.Background()
	cases := []struct {
		src  string
		want any
	}{
		// Bytes and strings compare symmetrically
		{`bytes("a") == "a"`, true},
		{`"a" == bytes("a")`, true},
		{`"a" < bytes("b")`, true},
		{`bytes("a") < "b"`, true},
		{`let ls = [bytes("a")]; ["a" in ls, ls.index("a")]`, []any{true, int64(0)}},
		// Every ordered comparison with NaN is false
		{`math.nan < 1.0`, false},
		{`1.0 < math.nan`, false},
		{`1 >= math.nan`, false},
		{`math.nan <= math.nan`, false},
		{`math.nan == math.nan`, false},
		{`math.nan != math.nan`, true},
		{`string(sorted([3.0, math.nan, 1, math.nan, 2.0]))`, "[1, 2, 3, NaN, NaN]"},
		// int ** float is a float
		{`2 ** 0.5`, 1.4142135623730951},
		{`2 ** -1.0`, 0.5},
		// int == float is exact above 2^53
		{`[9007199254740992 == 9007199254740992.0, 9007199254740992.0 == 9007199254740993, 9007199254740993 > 9007199254740992.0]`, []any{true, false, true}},
	}
	for _, tc := range cases {
		t.Run(tc.src, func(t *testing.T) {
			result, err := Eval(ctx, tc.src, WithEnv(Builtins()))
			assert.Nil(t, err)
			assert.Equal(t, result, tc.want)
		})
	}
}

func TestNegativeShiftIsValueError(t *testing.T) {
	ctx := context.Background()
	for _, src := range []string{`1 << -1`, `1 >> -1`, `byte(1) << -1`, `byte(1) >> -1`} {
		t.Run(src, func(t *testing.T) {
			_, err := Eval(ctx, src, WithEnv(Builtins()))
			assert.NotNil(t, err)
			assert.Contains(t, err.Error(), "negative shift count")
		})
	}
	result, err := Eval(ctx, `try { 1 << -1 } catch e { e.kind() }`, WithEnv(Builtins()))
	assert.Nil(t, err)
	assert.Equal(t, result, "value error")
}
