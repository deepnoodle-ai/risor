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
		// Inside lists, elements use the sort order: NaN is above every
		// number and equal to itself. Equality still follows IEEE 754.
		{`[math.nan] > [1.0]`, true},
		{`[1.0] < [math.nan]`, true},
		{`[math.nan] <= [math.nan]`, true},
		{`[math.nan] == [math.nan]`, false},
		{`"a".compare(bytes("b"))`, int64(-1)},
		{`[math.min(1, math.nan), math.max(math.nan, 1)].map(x => math.is_nan(x))`, []any{true, true}},
		// int ** float is a float
		{`2 ** 0.5`, 1.4142135623730951},
		{`2 ** -1.0`, 0.5},
		// int ** negative int is a float; int ** non-negative int is an int
		{`2 ** -1`, 0.5},
		{`2 ** -2`, 0.25},
		{`byte(2) ** -1`, 0.5},
		{`2 ** 3`, int64(8)},
		{`2 ** 0`, int64(1)},
		{`[0 ** -1 == math.inf, 0 ** -1.0 == math.inf]`, []any{true, true}},
		// int ** int is exact above 2^53, and wraps on overflow like *
		{`((1 << 53) + 1) ** 1 == (1 << 53) + 1`, true},
		{`3 ** 35`, int64(50031545098999707)},
		{`(-1) ** ((1 << 53) + 1)`, int64(-1)},
		{`byte(3) ** 35`, int64(50031545098999707)},
		{`[2 ** 62, 2 ** 63, 2 ** 64, (-2) ** 63]`, []any{int64(1 << 62), int64(-1 << 63), int64(0), int64(-1 << 63)}},
		{`[0 ** 0, 7 ** 0, 0 ** 5, (-3) ** 3]`, []any{int64(1), int64(1), int64(0), int64(-27)}},
		// byte ** byte wraps like byte * byte
		{`[byte(2) ** byte(7), byte(16) ** byte(2), byte(3) ** byte(5)]`, []any{byte(128), byte(0), byte(243)}},
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

func TestBytesSliceIsACopy(t *testing.T) {
	result, err := Eval(context.Background(), `
		let b = bytes("hello")
		let s = b[1:4]
		s[0] = byte(88)
		s[1] = "Y"
		[string(b), string(s)]
	`, WithEnv(Builtins()))
	assert.Nil(t, err)
	assert.Equal(t, result, []any{"hello", "XYl"})
}

func TestByteFromString(t *testing.T) {
	ctx := context.Background()
	cases := []struct {
		src  string
		want byte
	}{
		{`byte("0")`, 0},
		{`byte("127")`, 127},
		{`byte("128")`, 128},
		{`byte("255")`, 255},
		{`byte("0x80")`, 0x80},
		{`byte("0xff")`, 0xff},
		{`byte("0b101")`, 5},
	}
	for _, tc := range cases {
		result, err := Eval(ctx, tc.src, WithEnv(Builtins()))
		assert.Nil(t, err, tc.src)
		assert.Equal(t, result, tc.want, tc.src)
	}
	for _, src := range []string{`byte("256")`, `byte("-1")`, `byte("-128")`, `byte("0x100")`, `byte("x")`} {
		_, err := Eval(ctx, src, WithEnv(Builtins()))
		assert.NotNil(t, err, src)
		assert.Contains(t, err.Error(), "invalid literal for byte()", src)
	}
}
