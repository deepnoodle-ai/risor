package gen

import (
	"math"
	"math/rand/v2"
)

const maxWhole = 9007199254740991

// numLit is a number literal: its text and its kind.
type numLit struct {
	text string
	k    Kind
}

// numLits are the number literals of programs. Edges first: zero, the
// largest whole number, the smallest and largest doubles, values that print
// in exponent form.
var numLits = []numLit{
	{"0", KInt}, {"1", KInt}, {"2", KInt}, {"3", KInt}, {"7", KInt},
	{"10", KInt}, {"42", KInt}, {"100", KInt}, {"0xFF", KInt}, {"0x1f", KInt},
	{"1_000", KInt}, {"0.5", KNum}, {"0.1", KNum}, {"1.5", KNum}, {"2.5", KNum},
	{"1e3", KNum}, {"1e308", KNum}, {"5e-324", KNum}, {"1e20", KNum},
	{"1e21", KNum}, {"1.5e-7", KNum}, {"0.000001", KNum},
	{"9007199254740991", KNum}, {"123456.789", KNum}, {"3.14159", KNum},
	{"1e-7", KNum}, {"4.35", KNum}, {"0.30000000000000004", KNum},
	{"9007199254740993.0", KNum}, {"1e-400", KNum}, {"2.2250738585072014e-308", KNum},
	{"1.7976931348623157e308", KNum}, {"1E2", KNum}, {"6.02e+23", KNum},
	{"4503599627370496.5", KNum}, {"0.1e1", KNum},
}

// numValues are the numbers of host values.
var numValues = []float64{
	0, math.Copysign(0, -1), 1, -1, 2, 3, 7, 10, 42, -5, 100, 1000,
	0.5, 0.1, -0.1, 1.5, 2.5, -2.5, 1e308, -1e308, 5e-324, 1e20, 1e21, 1.5e-7,
	0.000001, maxWhole, -maxWhole, maxWhole + 1, 123456.789, 0.30000000000000004,
	4.35, 1.7976931348623157e308, 2.2250738585072014e-308, 1e-7, 0.2,
}

// strValues are the strings of programs and host values: code points above
// U+FFFF, combining marks, case mapping that changes length, controls, and
// text that looks like syntax.
var strValues = []string{
	"", "a", "b", "abc", "ab", "héllo", "😀", "a😀b", "é", "ﬃ", "ß",
	"İ", "Σ", "ǅ", "x y z", "a,b,c", ",", "  pad  ", "\t tab", "line\nbreak",
	"quote\"s", "it's", "back\\slash", "\u0000", "\u001f", "\u007f", "\u2028",
	"\ufeff", "${not}", "`tick`", "{}", "[1]", "1.5", "42", "-0", "true",
	"null", "ABC", "MiXeD", "𝄞music", "\U0010FFFF", "a\r\nb", "ÅÉÎ", "ﬁ",
	"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	"😀😀😀😀😀😀😀😀😀😀😀😀😀😀😀😀😀😀😀😀😀😀😀😀😀😀😀😀😀😀😀😀😀",
	"x", "y", "-", "/", "a/b", "ǈ", "ŉ", "ΐ",
}

// numericStrings are strings that int and float may accept.
var numericStrings = []string{
	"0", "42", "-3", "1.5", "-2.5", "1e3", "007", " 1", "1e400", "0x10",
	"9007199254740993", "-0", "abc", "", "1.", ".5", "1_0",
}

// fieldNames are keys of maps: words, a reserved word, text that is not a
// word.
var fieldNames = []string{
	"a", "b", "c", "id", "name", "n", "tags", "if", "x_1", "keys", "a-b", "",
	"😀", "B", "aa",
}

// varNames are names of variables.
var varNames = []string{"a", "b", "n", "s", "xs", "m", "t", "u", "x", "y", "data", "items"}

func pick[T any](r *rand.Rand, xs []T) T { return xs[r.IntN(len(xs))] }

// hostValue makes a random host value of nesting at most depth.
func (g *Gen) hostValue(depth int) any {
	r := g.r
	n := r.IntN(10)
	if depth <= 0 && n >= 7 {
		n = r.IntN(7)
	}
	switch n {
	case 0, 1, 2:
		return pick(r, numValues)
	case 3, 4:
		return pick(r, strValues)
	case 5:
		return r.IntN(2) == 0
	case 6:
		if g.opaque != "" && r.IntN(3) == 0 {
			return g.opaqueValue()
		}
		return nil
	case 7, 8:
		k := r.IntN(5)
		out := make([]any, k)
		// A list is homogeneous more often than not, so that typed
		// programs can use it.
		if r.IntN(3) > 0 && k > 0 {
			kind := r.IntN(3)
			for i := range out {
				switch kind {
				case 0:
					out[i] = float64(r.IntN(20) - 5)
				case 1:
					out[i] = pick(r, strValues)
				default:
					out[i] = g.hostValue(depth - 1)
				}
			}
			return out
		}
		for i := range out {
			out[i] = g.hostValue(depth - 1)
		}
		return out
	default:
		k := r.IntN(5)
		out := map[string]any{}
		for range k {
			out[pick(r, fieldNames)] = g.hostValue(depth - 1)
		}
		return out
	}
}

func (g *Gen) opaqueValue() any {
	return map[string]any{"$opaque": map[string]any{"type": g.opaque, "id": float64(g.r.IntN(3) + 1)}}
}

// deepValue is a list nested n levels, for the depth limit of walks.
func deepValue(n int) any {
	var v any = []any{1.0}
	for range n - 1 {
		v = []any{v}
	}
	return v
}
