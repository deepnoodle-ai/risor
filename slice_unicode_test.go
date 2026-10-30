package risor

import (
	"context"
	"testing"

	"github.com/deepnoodle-ai/wonton/assert"
)

func TestEmptySlices(t *testing.T) {
	tests := []struct {
		input    string
		expected any
	}{
		{`""[:]`, ""},
		{`"ab"[2:2]`, ""},
		{`"ab"[2:]`, ""},
		{`"é"[1:]`, ""},
		{`[][:]`, []any{}},
		{`[1, 2][2:2]`, []any{}},
		{`[1, 2][2:]`, []any{}},
		{`bytes("")[:]`, []byte{}},
		{`bytes("ab")[2:2]`, []byte{}},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result, err := Eval(context.Background(), tt.input, WithEnv(Builtins()))
			assert.Nil(t, err)
			assert.Equal(t, result, tt.expected)
		})
	}
}

func TestSliceOutOfRange(t *testing.T) {
	inputs := []string{`""[1:]`, `"ab"[3:]`, `[][1:]`, `[1, 2][3:3]`, `bytes("ab")[3:]`}
	for _, input := range inputs {
		t.Run(input, func(t *testing.T) {
			_, err := Eval(context.Background(), input, WithEnv(Builtins()))
			assert.NotNil(t, err)
		})
	}
}

// String index, last_index, s[i], slicing, and len all count runes.
func TestStringIndexesCountRunes(t *testing.T) {
	tests := []struct {
		input    string
		expected any
	}{
		{`"héllo".index("l")`, int64(2)},
		{`"héllo".last_index("l")`, int64(3)},
		{`"héllo".index("x")`, int64(-1)},
		{`"héllo".last_index("x")`, int64(-1)},
		{`"héllo".index("")`, int64(0)},
		{`"héllo".last_index("")`, int64(5)},
		{`let s = "日本語"; s[s.index("語")]`, "語"},
		{`let s = "héllo"; s[s.last_index("l"):]`, "lo"},
		{`let s = "héllo"; len(s) == s.last_index("")`, true},
		{`keys("héllo")`, []any{int64(0), int64(1), int64(2), int64(3), int64(4)}},
		{`let s = "héllo"; keys(s).map(i => s[i])`, []any{"h", "é", "l", "l", "o"}},
		{`bytes("héllo").contains_rune("é")`, true},
		{`bytes("héllo").contains_rune("日")`, false},
		{`bytes("héllo").index_rune("é")`, int64(1)},
		{`bytes("héllo").index_rune("l")`, int64(3)},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result, err := Eval(context.Background(), tt.input, WithEnv(Builtins()))
			assert.Nil(t, err)
			assert.Equal(t, result, tt.expected)
		})
	}
}
