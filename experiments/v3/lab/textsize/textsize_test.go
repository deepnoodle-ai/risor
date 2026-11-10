package textsize

import (
	"fmt"
	"strings"
	"testing"
	"unicode/utf16"

	"github.com/deepnoodle-ai/wonton/assert"
)

// text returns n bytes of text. The mixed kind is about one third non-ASCII:
// Latin letters with accents, CJK and emoji.
func text(kind string, n int) string {
	unit := "the quick brown fox jumps over the lazy dog "
	if kind == "mixed" {
		unit = "naïve café 東京タワー 😀 jumps über "
	}
	s := strings.Repeat(unit, n/len(unit)+1)
	s = s[:n]
	for !strings.HasSuffix(s, " ") && len(s) > 0 && s[len(s)-1] >= 0x80 {
		s = s[:len(s)-1] // do not cut a code point
	}
	return strings.ToValidUTF8(s, "")
}

func TestSizes(t *testing.T) {
	for _, kind := range []string{"ascii", "mixed"} {
		s := text(kind, 4096)
		assert.Equal(t, UTF16(s), len(utf16.Encode([]rune(s))), kind)
		assert.Equal(t, CodePoints(s), len([]rune(s)), kind)
		assert.Equal(t, CodePointsFast(s), len([]rune(s)), kind)
		assert.Equal(t, Weight(s), len(utf16.Encode([]rune(s))), kind)
		assert.Equal(t, Blocks(s), int64(len(utf16.Encode([]rune(s)))/Block), kind)
		for n := range 40 { // every tail length
			assert.Equal(t, Weight(s[:len(s)-n]), UTF16(s[:len(s)-n]), kind)
			assert.Equal(t, WeightSafe(s[:len(s)-n]), UTF16(s[:len(s)-n]), kind)
		}
	}
	assert.Equal(t, Blocks(strings.Repeat("😀", 15)), int64(0)) // 60 bytes: not scanned
	assert.Equal(t, Blocks(strings.Repeat("😀", 16)), int64(0)) // 64 bytes, weight 32: scanned
	assert.Equal(t, Blocks(strings.Repeat("😀", 32)), int64(1)) // weight 64
	assert.Equal(t, Blocks(strings.Repeat("é", 64)), int64(1)) // 128 bytes, weight 64
	assert.Equal(t, Blocks(strings.Repeat("a", 64)), int64(1))
}

func TestEqual(t *testing.T) {
	a := text("mixed", 1024)
	for _, b := range []string{a, a + "x", a[:len(a)-1] + "y", "z" + a, text("ascii", 1024), ""} {
		for _, eq := range []func(*runtime, string, string) bool{EqualPlain, EqualByLength, EqualByLengthStdlib, EqualByPrefix} {
			assert.Equal(t, eq(&runtime{}, a, b), a == b)
		}
	}
	// The prefix charge is in weight, so it is the same number whatever the
	// encoding. 100 emoji, then a difference: 400 bytes, weight 200.
	x := strings.Repeat("😀", 100) + "a" + strings.Repeat("b", 100)
	y := strings.Repeat("😀", 100) + "c" + strings.Repeat("b", 100)
	rt := &runtime{}
	EqualByPrefix(rt, x, y)
	assert.Equal(t, -rt.steps, int64(1+200/Block))
	// A difference inside a code point does not count that code point.
	rt = &runtime{}
	EqualByPrefix(rt, strings.Repeat("a", 63)+"é"+strings.Repeat("b", 64), strings.Repeat("a", 63)+"è"+strings.Repeat("b", 64))
	assert.Equal(t, -rt.steps, int64(1))
}

var (
	sinkInt  int
	sinkBool bool
	sinkStr  string
)

// BenchmarkSize is the cost of knowing the size of a string that is only
// read, in each unit.
func BenchmarkSize(b *testing.B) {
	for _, kind := range []string{"ascii", "mixed"} {
		for _, n := range []int{16, 64, 256, 4096} {
			s := text(kind, n)
			for _, u := range []struct {
				name string
				size func(string) int
			}{
				{"utf8_bytes", Bytes},
				{"code_points_stdlib", CodePoints},
				{"code_points_by_word", CodePointsFast},
				{"utf16_units_by_byte", UTF16},
				{"utf16_units_by_word", Weight},
				{"utf16_units_by_word_safe", WeightSafe},
				{"blocks", func(s string) int { return int(Blocks(s)) }},
			} {
				b.Run(fmt.Sprintf("%s/%d/%s", kind, n, u.name), func(b *testing.B) {
					for b.Loop() {
						sinkInt = u.size(s)
					}
				})
			}
		}
	}
}

// BenchmarkEqual compares two strings of the same length.
func BenchmarkEqual(b *testing.B) {
	for _, c := range []struct {
		name string
		a, b string
	}{
		{"12_bytes_equal", "user_id_0042", strings.Clone("user_id_0042")},
		{"256_bytes_equal", text("ascii", 256), strings.Clone(text("ascii", 256))},
		{"256_bytes_differ_at_start", text("ascii", 256), "X" + text("ascii", 256)[1:]},
		{"4096_bytes_equal", text("ascii", 4096), strings.Clone(text("ascii", 4096))},
		{"4096_bytes_differ_at_start", text("ascii", 4096), "X" + text("ascii", 4096)[1:]},
	} {
		for _, eq := range []struct {
			name string
			fn   func(*runtime, string, string) bool
		}{{"one_step", EqualPlain}, {"by_length_stdlib", EqualByLengthStdlib}, {"by_length", EqualByLength}, {"by_prefix", EqualByPrefix}} {
			b.Run(c.name+"/"+eq.name, func(b *testing.B) {
				rt := &runtime{}
				for b.Loop() {
					rt.steps = 1 << 40
					sinkBool = eq.fn(rt, c.a, c.b)
				}
			})
		}
	}
}

// BenchmarkConcat joins two strings of n bytes each.
func BenchmarkConcat(b *testing.B) {
	for _, n := range []int{10, 48, 512} {
		x, y := text("ascii", n), text("ascii", n)
		b.Run(fmt.Sprintf("2x%d_bytes/one_step", n), func(b *testing.B) {
			rt := &runtime{}
			b.ReportAllocs()
			for b.Loop() {
				rt.steps = 1 << 40
				sinkStr = ConcatPlain(rt, x, y)
			}
		})
		b.Run(fmt.Sprintf("2x%d_bytes/charged_code_points_stdlib", n), func(b *testing.B) {
			rt := &runtime{}
			b.ReportAllocs()
			for b.Loop() {
				rt.steps, rt.units = 1<<40, 1<<40
				sinkStr, sinkBool = ConcatChargedStdlib(rt, x, y)
			}
		})
		b.Run(fmt.Sprintf("2x%d_bytes/charged_by_weight", n), func(b *testing.B) {
			rt := &runtime{}
			b.ReportAllocs()
			for b.Loop() {
				rt.steps, rt.units = 1<<40, 1<<40
				sinkStr, sinkBool = ConcatCharged(rt, x, y)
			}
		})
		b.Run(fmt.Sprintf("2x%d_bytes/charged_by_bytes", n), func(b *testing.B) {
			rt := &runtime{}
			b.ReportAllocs()
			for b.Loop() {
				rt.steps, rt.units = 1<<40, 1<<40
				sinkStr, sinkBool = ConcatChargedBytes(rt, x, y)
			}
		})
	}
}
