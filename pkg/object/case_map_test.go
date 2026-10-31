package object

import (
	"context"
	"strings"
	"testing"
	"unicode"

	"github.com/deepnoodle-ai/wonton/assert"
)

func callStringMethod(t *testing.T, ctx context.Context, s, name string) (Object, error) {
	t.Helper()
	method, ok := NewString(s).GetAttr(name)
	assert.True(t, ok)
	return method.(*Builtin).Call(ctx)
}

// Case mapping can grow a string: "ȿ" (2 bytes) uppercases to "Ȿ" (3 bytes),
// and an invalid UTF-8 byte maps to U+FFFD (3 bytes). The result must not
// pass the byte limit even when the input is under it.
func TestStringCaseMapLimits(t *testing.T) {
	ctx := WithLimits(context.Background(), Limits{MaxBytesSize: 1000})

	tests := []struct {
		name   string
		method string
		input  string
	}{
		{"upper grows 2 to 3 bytes", "to_upper", strings.Repeat("ȿ", 400)},
		{"lower grows 2 to 3 bytes", "to_lower", strings.Repeat("Ⱥ", 400)},
		{"upper invalid utf8", "to_upper", strings.Repeat("\xff", 400)},
		{"lower invalid utf8", "to_lower", strings.Repeat("\xff", 400)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.True(t, len(tt.input) <= 1000)
			_, err := callStringMethod(t, ctx, tt.input, tt.method)
			assertValueError(t, err, "too large")
		})
	}

	// A result of exactly the limit is allowed.
	result, err := callStringMethod(t, ctx, strings.Repeat("ȿ", 333)+"a", "to_upper")
	assert.Nil(t, err)
	assert.Equal(t, result.(*String).Value(), strings.Repeat("Ȿ", 333)+"A")
	assert.Equal(t, len(result.(*String).Value()), 1000)

	// Strings that shrink or keep their length are not rejected.
	result, err = callStringMethod(t, ctx, strings.Repeat("Ȿ", 333), "to_lower")
	assert.Nil(t, err)
	assert.Equal(t, result.(*String).Value(), strings.Repeat("ȿ", 333))

	result, err = callStringMethod(t, ctx, strings.Repeat("a", 1000), "to_upper")
	assert.Nil(t, err)
	assert.Equal(t, result.(*String).Value(), strings.Repeat("A", 1000))
}

// assertCaseMapLen checks that caseMapLen counts exactly the bytes that
// strings.ToUpper and strings.ToLower return for s, and that the result is
// never more than maxCaseMapGrowth times as long as s.
func assertCaseMapLen(t *testing.T, s string) {
	t.Helper()
	upper := caseMapLen(s, unicode.ToUpper)
	lower := caseMapLen(s, unicode.ToLower)
	if upper != len(strings.ToUpper(s)) || lower != len(strings.ToLower(s)) {
		t.Fatalf("caseMapLen(%q) = (upper %d, lower %d), want (%d, %d)",
			s, upper, lower, len(strings.ToUpper(s)), len(strings.ToLower(s)))
	}
	assert.True(t, upper <= maxCaseMapGrowth*len(s))
	assert.True(t, lower <= maxCaseMapGrowth*len(s))
}

func TestCaseMapLenAllRunes(t *testing.T) {
	var all strings.Builder
	for r := rune(0); r <= unicode.MaxRune; r++ {
		s := string(r)
		assertCaseMapLen(t, s)
		assertCaseMapLen(t, "a"+s+"\xff")
		all.WriteString(s)
	}
	// Every byte value alone, which covers each invalid UTF-8 lead and
	// continuation byte.
	for b := 0; b < 256; b++ {
		assertCaseMapLen(t, string([]byte{byte(b)}))
		all.WriteByte(byte(b))
	}
	assertCaseMapLen(t, all.String())
}

func TestCaseMapLenSpecialCases(t *testing.T) {
	for _, s := range []string{
		"",
		"hello WORLD",
		"ȿȺⱥȾ",             // change between 2 and 3 bytes
		"ſıK",              // map to ASCII (U+212A is the Kelvin sign)
		"�",                // valid U+FFFD maps to itself
		"\xff\xfe",         // invalid bytes grow 1 to 3 bytes
		"\xe2\x82",         // truncated 3-byte sequence
		"\xed\xa0\x80",     // UTF-16 surrogate, invalid in UTF-8
		"\xf4\x90\x80\x80", // above unicode.MaxRune
		"\xc0\xaf",         // overlong encoding
		"abc\xffȿ",
		"ǅǈǋǲ", // title case letters
		"İ",    // dotted capital I
	} {
		assertCaseMapLen(t, s)
	}
}

func FuzzCaseMapLen(f *testing.F) {
	for _, s := range []string{"", "abc", "ȿȺ", "ſıK", "\xff", "\xe2\x82", "a�b", "İǅ"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		assertCaseMapLen(t, s)
	})
}
