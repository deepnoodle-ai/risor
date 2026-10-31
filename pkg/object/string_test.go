package object

import (
	"context"
	"fmt"
	"math"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/deepnoodle-ai/wonton/assert"
)

func TestStringBasics(t *testing.T) {
	value := NewString("abcd")
	assert.Equal(t, value.Type(), STRING)
	assert.Equal(t, value.Value(), "abcd")
	assert.Equal(t, value.String(), "abcd")
	assert.Equal(t, value.Inspect(), `"abcd"`)
	assert.Equal(t, value.Interface(), "abcd")
	assert.True(t, value.Equals(NewString("abcd")))
}

func TestStringCompare(t *testing.T) {
	a := NewString("a")
	b := NewString("b")
	A := NewString("A")
	tests := []struct {
		first    Comparable
		second   Object
		expected int
	}{
		{a, b, -1},
		{b, a, 1},
		{a, a, 0},
		{a, A, 1},
		{A, a, -1},
	}
	for _, tc := range tests {
		result, err := tc.first.Compare(tc.second)
		assert.Nil(t, err)
		assert.Equal(t, result, tc.expected,
			"first: %v, second: %v", tc.first, tc.second)
	}
}

func TestStringReverse(t *testing.T) {
	tests := []struct {
		s        string
		expected string
	}{
		{"", ""},
		{"a", "a"},
		{"ab", "ba"},
		{"abc", "cba"},
	}
	for _, tc := range tests {
		result := NewString(tc.s).Reversed().Value()
		assert.Equal(t, result, tc.expected, "s: %v", tc.s)
	}
}

func TestStringGetItem(t *testing.T) {
	tests := []struct {
		s           string
		index       int64
		expected    string
		expectedErr string
	}{
		{"", 0, "", "index out of range: 0"},
		{"a", 0, "a", ""},
		{"a", -1, "a", ""},
		{"a", -2, "a", "index out of range: -2"},
		{"012345", 5, "5", ""},
		{"012345", -1, "5", ""},
		{"012345", -2, "4", ""},
	}
	for _, tc := range tests {
		msg := fmt.Sprintf("%v[%d]", tc.s, tc.index)
		result, err := NewString(tc.s).GetItem(NewInt(tc.index))
		if tc.expectedErr != "" {
			assert.NotNil(t, err, msg)
			assert.Equal(t, err.Message().value, tc.expectedErr, msg)
		} else {
			resultStr, ok := result.(*String)
			assert.True(t, ok, msg)
			assert.Equal(t, resultStr.Value(), tc.expected, msg)
		}
	}
}

func TestStringEnumerateRuneIndexes(t *testing.T) {
	s := NewString("héllo")
	var keys []int64
	var values []string
	s.Enumerate(context.Background(), func(key, value Object) bool {
		keys = append(keys, key.(*Int).Value())
		values = append(values, value.(*String).Value())
		return true
	})
	assert.Equal(t, keys, []int64{0, 1, 2, 3, 4})
	assert.Equal(t, values, []string{"h", "é", "l", "l", "o"})
}

func TestStringIndexInvalidUTF8MatchesReference(t *testing.T) {
	// The reference maps each byte offset to the rune index that a range
	// loop starts there, or -1 inside a rune. Index and LastIndex must give
	// the first and last match whose both ends are rune starts.
	reference := func(s, substr string) (int64, int64) {
		starts := make([]int, len(s)+1)
		for i := range starts {
			starts[i] = -1
		}
		n := 0
		for i := range s {
			starts[i] = n
			n++
		}
		starts[len(s)] = n
		first, last := int64(-1), int64(-1)
		for off := 0; off+len(substr) <= len(s); off++ {
			if s[off:off+len(substr)] != substr || starts[off] < 0 || starts[off+len(substr)] < 0 {
				continue
			}
			if first < 0 {
				first = int64(starts[off])
			}
			last = int64(starts[off])
		}
		return first, last
	}
	// These bytes form valid 2-, 3- and 4-byte runes ("\xc3\xa9", "\xe2\x80\x80",
	// "\xf0\x9f\x80\x80") and invalid sequences.
	alphabet := []byte{'a', 0xc3, 0xa9, 0xe2, 0xf0, 0x9f, 0x80, 0xff}
	var build func(prefix []byte, depth int, out *[]string)
	build = func(prefix []byte, depth int, out *[]string) {
		*out = append(*out, string(prefix))
		if depth == 0 {
			return
		}
		for _, b := range alphabet {
			build(append(append([]byte{}, prefix...), b), depth-1, out)
		}
	}
	var hay, needles []string
	build(nil, 4, &hay)
	build(nil, 2, &needles)
	for _, h := range hay {
		s := NewString(h)
		for _, sub := range needles {
			if utf8.ValidString(sub) {
				continue
			}
			wantFirst, wantLast := reference(h, sub)
			first, err := s.Index(NewString(sub))
			assert.Nil(t, err)
			assert.Equal(t, first, NewInt(wantFirst), "index %q in %q", sub, h)
			last, err := s.LastIndex(NewString(sub))
			assert.Nil(t, err)
			assert.Equal(t, last, NewInt(wantLast), "last_index %q in %q", sub, h)
		}
	}
}

func TestStringIndexInvalidUTF8(t *testing.T) {
	// "\xc3\xa9" is "é". An invalid UTF-8 substring can match inside it.
	// Index and LastIndex skip those matches, so s[i] accepts each result.
	tests := []struct {
		s, substr   string
		first, last int64
	}{
		{"\xc3\xa9", "\xa9", -1, -1},
		{"\xc3\xa9", "\xc3", -1, -1},
		{"\xc3\xa9\xa9", "\xa9", 1, 1},
		{"\xa9\xc3\xa9\xa9", "\xa9", 0, 2},
		{"a\xffb\xff", "\xff", 1, 3},
		{"a\xffb", "\xffb", 1, 1},
	}
	for _, tc := range tests {
		s := NewString(tc.s)
		first, err := s.Index(NewString(tc.substr))
		assert.Nil(t, err)
		assert.Equal(t, first, NewInt(tc.first), "index %q in %q", tc.substr, tc.s)
		last, err := s.LastIndex(NewString(tc.substr))
		assert.Nil(t, err)
		assert.Equal(t, last, NewInt(tc.last), "last_index %q in %q", tc.substr, tc.s)
		for _, i := range []int64{tc.first, tc.last} {
			if i < 0 {
				continue
			}
			item, err := s.GetItem(NewInt(i))
			assert.Nil(t, err)
			assert.Equal(t, item.(*String).Value(), string([]rune(tc.substr)[0]))
		}
	}
}

// runeGetItem and runeGetSlice are the original implementations of
// String.GetItem and GetSlice, which convert the whole string to []rune.
// The current implementations must give the same results and errors.
func runeGetItem(s string, key Object) (string, string) {
	indexObj, ok := key.(*Int)
	if !ok {
		return "", TypeErrorf("string index must be an int (got %s)", key.Type()).Message().value
	}
	runes := []rune(s)
	index, err := ResolveIndex(indexObj.value, int64(len(runes)))
	if err != nil {
		return "", NewError(err).Message().value
	}
	return string(runes[index]), ""
}

func runeGetSlice(s string, slice Slice) (string, string) {
	runes := []rune(s)
	start, stop, err := ResolveIntSlice(slice, int64(len(runes)))
	if err != nil {
		return "", NewError(err).Message().value
	}
	return string(runes[start:stop]), ""
}

func stringResult(obj Object, err *Error) (string, string) {
	if err != nil {
		return "", err.Message().value
	}
	return obj.(*String).Value(), ""
}

// stringIndexSamples returns ASCII, multi-byte, and invalid UTF-8 strings.
func stringIndexSamples() []string {
	samples := []string{
		"",
		"a",
		"hello",
		"héllo",
		"日本語テキスト",
		"a😀b🎉c",
		"\xff",
		"a\xffb",
		"\xc3",             // truncated 2-byte rune
		"\xe2\x82",         // truncated 3-byte rune
		"x\xf0\x9f\x80",    // truncated 4-byte rune
		"\xed\xa0\x80z",    // UTF-16 surrogate, invalid in UTF-8
		"\xc0\xafq",        // overlong encoding
		"\xef\xbf\xbd\xff", // a real U+FFFD next to an invalid byte
		"é\x80\x80日",
	}
	// Every string of up to 4 bytes from an alphabet that forms valid 2-, 3-,
	// and 4-byte runes and invalid sequences.
	alphabet := []byte{'a', 0xc3, 0xa9, 0xe2, 0xf0, 0x9f, 0x80, 0xff}
	var build func(prefix []byte, depth int)
	build = func(prefix []byte, depth int) {
		samples = append(samples, string(prefix))
		if depth == 0 {
			return
		}
		for _, b := range alphabet {
			build(append(append([]byte{}, prefix...), b), depth-1)
		}
	}
	build(nil, 4)
	return samples
}

// stringIndexes returns every index in range for s, a few out of range on
// each side, and the extreme int64 values.
func stringIndexes(s string) []int64 {
	n := int64(utf8.RuneCountInString(s))
	indexes := []int64{math.MinInt64, math.MinInt64 + 1, math.MaxInt64}
	for i := -n - 2; i <= n+2; i++ {
		indexes = append(indexes, i)
	}
	return indexes
}

func TestStringGetItemMatchesRunes(t *testing.T) {
	for _, s := range stringIndexSamples() {
		for _, i := range stringIndexes(s) {
			want, wantErr := runeGetItem(s, NewInt(i))
			got, gotErr := stringResult(NewString(s).GetItem(NewInt(i)))
			assert.Equal(t, gotErr, wantErr, "%q[%d]", s, i)
			assert.Equal(t, got, want, "%q[%d]", s, i)
		}
	}
	want, wantErr := runeGetItem("abc", NewFloat(1))
	got, gotErr := stringResult(NewString("abc").GetItem(NewFloat(1)))
	assert.Equal(t, gotErr, wantErr)
	assert.Equal(t, got, want)
	assert.Equal(t, gotErr, "string index must be an int (got float)")
}

func TestStringGetSliceMatchesRunes(t *testing.T) {
	for _, s := range stringIndexSamples() {
		bounds := []Object{nil}
		for _, i := range stringIndexes(s) {
			bounds = append(bounds, NewInt(i))
		}
		for _, start := range bounds {
			for _, stop := range bounds {
				slice := Slice{Start: start, Stop: stop}
				want, wantErr := runeGetSlice(s, slice)
				got, gotErr := stringResult(NewString(s).GetSlice(slice))
				assert.Equal(t, gotErr, wantErr, "%q[%v:%v]", s, start, stop)
				assert.Equal(t, got, want, "%q[%v:%v]", s, start, stop)
			}
		}
	}
	for _, slice := range []Slice{
		{Start: NewFloat(1)},
		{Stop: NewString("x")},
		{Start: NewFloat(1), Stop: NewInt(-1)},
		{Start: NewInt(-1), Stop: NewFloat(1)},
		{Start: NewInt(1), Stop: NewFloat(1)},
	} {
		want, wantErr := runeGetSlice("héllo", slice)
		got, gotErr := stringResult(NewString("héllo").GetSlice(slice))
		assert.NotEqual(t, wantErr, "")
		assert.Equal(t, gotErr, wantErr)
		assert.Equal(t, got, want)
	}
}

func TestStringGetItemDoesNotCopyString(t *testing.T) {
	// Indexing or slicing a large string allocates only the result, not a
	// []rune copy of the whole string.
	big := NewString(strings.Repeat("é", 100_000))
	index := NewInt(5)
	allocs := testing.AllocsPerRun(10, func() {
		_, _ = big.GetItem(index)
	})
	assert.Equal(t, allocs, 1.0, "GetItem allocs")
	slice := Slice{Start: NewInt(1), Stop: NewInt(3)}
	allocs = testing.AllocsPerRun(10, func() {
		_, _ = big.GetSlice(slice)
	})
	assert.Equal(t, allocs, 1.0, "GetSlice allocs")
}

func BenchmarkStringGetItem(b *testing.B) {
	for _, bc := range []struct {
		name string
		s    string
	}{
		{"ascii_1MB", strings.Repeat("a", 1<<20)},
		{"utf8_1MB", strings.Repeat("é", 1<<19)},
	} {
		s := NewString(bc.s)
		n := int64(utf8.RuneCountInString(bc.s))
		first, middle, last := NewInt(0), NewInt(n/2), NewInt(-1)
		slice := Slice{Start: NewInt(10), Stop: NewInt(20)}
		b.Run(bc.name+"/first", func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				_, _ = s.GetItem(first)
			}
		})
		b.Run(bc.name+"/middle", func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				_, _ = s.GetItem(middle)
			}
		})
		b.Run(bc.name+"/last", func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				_, _ = s.GetItem(last)
			}
		})
		b.Run(bc.name+"/slice", func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				_, _ = s.GetSlice(slice)
			}
		})
	}
}
