package builtins

import (
	"context"
	stderrors "errors"
	"fmt"
	"math"
	"runtime"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/deepnoodle-ai/risor/v2/pkg/object"
	"github.com/deepnoodle-ai/wonton/assert"
)

// formatTestArgs returns Go values as formatArgs produces them for a range
// of Risor values.
func formatTestArgs(t testing.TB) []any {
	t.Helper()
	inner := object.NewList([]object.Object{object.NewInt(1), object.NewString("two"), object.Nil})
	objs := []object.Object{
		object.NewString("hello"),
		object.NewString(""),
		object.NewString("héllo \"wörld\"\n\x00\xff 世界 🙂"),
		object.NewInt(42),
		object.NewInt(-7),
		object.NewInt(0),
		object.NewInt(math.MaxInt64),
		object.NewFloat(3.25),
		object.NewFloat(-1e300),
		object.NewFloat(math.Inf(1)),
		object.NewFloat(math.NaN()),
		object.True,
		object.False,
		object.Nil,
		object.NewByte(200),
		object.NewBytes([]byte("ab\x00\xffc")),
		object.NewBytes(nil),
		object.NewList(nil),
		object.NewList([]object.Object{
			object.NewInt(1), object.NewFloat(2.5), object.NewString("x y"),
			object.Nil, object.True, inner, object.NewBytes([]byte{1, 2}),
			object.NewMap(map[string]object.Object{"k": object.NewInt(1)}),
		}),
		object.NewMap(map[string]object.Object{
			"a": object.NewInt(1), "b": object.NewString("s"), "c": inner, "d": object.Nil,
		}),
		object.NewMap(nil),
		object.NewError(stderrors.New("boom")),
		object.NewRange(0, 5, 1),
		object.NewTime(time.Date(2026, 10, 31, 12, 0, 0, 0, time.UTC)),
	}
	args, err := object.ToInterfaces(context.Background(), objs)
	assert.Nil(t, err)
	return args
}

// actualFormat returns the text that Sprintf, or Errorf when wrapErrs is
// true, produces.
func actualFormat(wrapErrs bool, format string, args []any) string {
	if wrapErrs {
		return fmt.Errorf(format, args...).Error()
	}
	return fmt.Sprintf(format, args...)
}

// The count is the exact length of the formatted text for every directive
// applied to every kind of value that Risor passes to fmt.
func TestFormatCountExact(t *testing.T) {
	args := formatTestArgs(t)
	verbs := "vdsqxXcUeEfFgGtTpbowO%z"
	flags := []string{"", "#", "+", "-", " ", "0", "#+", "- 0", "# "}
	widths := []string{"", "1", "8", "*"}
	precs := []string{"", ".", ".0", ".3", ".*"}
	for _, arg := range args {
		for _, verb := range verbs {
			for _, flag := range flags {
				for _, wid := range widths {
					for _, prec := range precs {
						format := "a%" + flag + wid + prec + string(verb) + "b"
						var a []any
						if wid == "*" {
							a = append(a, -6)
						}
						if prec == ".*" {
							a = append(a, 2)
						}
						a = append(a, arg)
						for _, wrapErrs := range []bool{false, true} {
							want := len(actualFormat(wrapErrs, format, a))
							got := formatCount(math.MaxInt64, wrapErrs, format, a)
							if uint64(want) != got {
								t.Fatalf("format %q with %T %v (wrapErrs %v): count %d, text %d: %q",
									format, arg, arg, wrapErrs, got, want, actualFormat(wrapErrs, format, a))
							}
						}
					}
				}
			}
		}
	}
}

// The quoted length is counted without building the quoted text, so it is
// compared with strconv for every rune up to U+3400, a sample of higher
// runes, and invalid bytes.
func TestFormatCountQuoted(t *testing.T) {
	var sb strings.Builder
	for r := rune(0); r < 0x3400; r++ {
		sb.WriteRune(r)
	}
	for r := rune(0x3400); r <= utf8.MaxRune; r += 997 {
		sb.WriteRune(r)
	}
	sb.WriteString("\xff\xc0\xed\xa0\x80`")
	str := sb.String()
	for _, arg := range []any{str, []byte(str), "`no backquote`"} {
		for _, format := range []string{"%q", "%+q", "%#q", "%#+q", "%#v", "%-8000q", "%.500q", "%x", "% #X", "%.7x"} {
			want := len(fmt.Sprintf(format, arg))
			got := formatCount(math.MaxInt64, false, format, []any{arg})
			assert.Equal(t, got, uint64(want), "format %q with %T", format, arg)
		}
	}
}

// Malformed formats, argument indexes, and missing and extra arguments are
// counted as fmt prints them.
func TestFormatCountMalformed(t *testing.T) {
	args := formatTestArgs(t)
	formats := []string{
		"", "%", "%!", "%[", "%[]d", "%[x]d", "%[0]d", "%[99]d", "%[2]d %d %[1]s",
		"%.", "%-", "%*d", "%.*d", "%[2]*[1]d", "%[1]2d", "%[1].2d", "%3[2]d",
		"%d %d %d", "no verbs", "%10000000d", "%.10000000d", "%[1]*d",
		"%d%%%d", "%\xffd", "%é", "%!d(x)", "%[1]d %[1]d %[1]d", "%w %w",
	}
	for _, format := range formats {
		for i := range args {
			for _, a := range [][]any{nil, {args[i]}, {args[i], 3, args[(i+1)%len(args)]}, {7, args[i]}} {
				for _, wrapErrs := range []bool{false, true} {
					want := len(actualFormat(wrapErrs, format, a))
					got := formatCount(math.MaxInt64, wrapErrs, format, a)
					if uint64(want) != got {
						t.Fatalf("format %q with %v (wrapErrs %v): count %d, text %d: %q",
							format, a, wrapErrs, got, want, actualFormat(wrapErrs, format, a))
					}
				}
			}
		}
	}
}

type hostInner struct {
	Name  string
	Count int
}

type hostStruct struct {
	Label   string
	Values  []int
	Inner   hostInner
	Ptr     *hostInner
	Any     any
	NilAny  any
	hidden  string
	private int
	items   []any
	attrs   map[string]any
	Data    map[string]int
	Arr     [2]byte
	Custom  hostFormatter
}

// hostFormatter is a host type with a Format method.
type hostFormatter struct{ n int }

func (h hostFormatter) Format(f fmt.State, verb rune) {
	wid, _ := f.Width()
	fmt.Fprintf(f, "<%c %d %v %d>", verb, h.n, f.Flag('#'), wid)
}

type panicStringer struct{ p *int }

func (s panicStringer) String() string { return strings.Repeat("s", *s.p) }

type nilStringer struct{ s string }

func (n *nilStringer) String() string { return n.s }

type panicFormatter struct{}

func (panicFormatter) Format(f fmt.State, verb rune) {
	f.Write([]byte("partial"))
	panic("oops")
}

// Host Go values that a GoStruct exposes are counted too, including
// unexported fields and nested pointers.
func TestFormatCountHostValues(t *testing.T) {
	v := &hostStruct{
		Label:   "x",
		Values:  []int{1, 2, 3},
		Inner:   hostInner{Name: "in", Count: 2},
		Ptr:     &hostInner{Name: "p"},
		Any:     []any{1, "a"},
		hidden:  "secret",
		private: 9,
		items:   []any{int64(1), "s", nil, []any{2.5}},
		attrs:   map[string]any{"k": int64(-3), "n": nil},
		Custom:  hostFormatter{n: 4},
		Data:    map[string]int{"a": 1},
		Arr:     [2]byte{'h', 'i'},
	}
	args := []any{
		v, *v,
		[]*hostInner{v.Ptr, nil},
		[3]int{1, 2, 3},
		[]string{"a", "b"},
		hostFormatter{n: 1},
		[]any{hostFormatter{n: 2}},
		panicFormatter{},
		panicStringer{},
		(*nilStringer)(nil),
		&nilStringer{s: "a\x00b"},
	}
	for _, arg := range args {
		for _, format := range []string{"%v", "%+v", "%#v", "%s", "%d", "%x", "%q", "%5v", "%-4.1v", "%T", "%p", "%*0"} {
			want := len(fmt.Sprintf(format, arg))
			got := formatCount(math.MaxInt64, false, format, []any{arg})
			if uint64(want) != got {
				t.Fatalf("format %q with %T: count %d, text %d: %q", format, arg, got, want, fmt.Sprintf(format, arg))
			}
		}
	}
}

func FuzzFormatCount(f *testing.F) {
	for _, seed := range []string{"%v", "%#v", "%[2]*[1]d", "%-+ #08.3x", "%!w(", "a%%b%q", "%[3]v %v %v"} {
		f.Add(seed)
	}
	args := formatTestArgs(f)
	f.Fuzz(func(t *testing.T, format string) {
		for _, a := range [][]any{args[:3], args[3:9], args[9:16], args[16:], {5, args[18], -3, args[19]}} {
			for _, wrapErrs := range []bool{false, true} {
				text := actualFormat(wrapErrs, format, a)
				got := formatCount(math.MaxInt64, wrapErrs, format, a)
				if got != uint64(len(text)) {
					t.Fatalf("format %q (wrapErrs %v): count %d, text %d: %q", format, wrapErrs, got, len(text), text)
				}
			}
		}
	})
}

func limitsCtx(maxBytes int64) context.Context {
	return object.WithLimits(context.Background(), object.Limits{MaxBytesSize: maxBytes})
}

// The reproduction from #594: padded verbs that reuse one argument. sprintf
// and error both fail before building the text, and allocate far less than
// the 32 MB it would take.
func TestFormatWidthAmplification(t *testing.T) {
	ctx := limitsCtx(1 << 20)
	format := object.NewString(strings.Repeat("%1000000[1]s", 32))
	for name, fn := range map[string]func(context.Context, ...object.Object) (object.Object, error){
		"sprintf": Sprintf,
		"error":   Error,
	} {
		t.Run(name, func(t *testing.T) {
			var before, after runtime.MemStats
			runtime.GC()
			runtime.ReadMemStats(&before)
			_, err := fn(ctx, format, object.NewString("x"))
			runtime.ReadMemStats(&after)
			assertValueError(t, err, name+" result too large (exceeds limit of 1048576 bytes)")
			// Before, each call allocated about 93 MiB.
			assert.Less(t, after.TotalAlloc-before.TotalAlloc, uint64(4<<20))
		})
	}
}

// %x and %q of one large string, bytes value, or error message fail before
// the hex or quoted text is built.
func TestFormatLargeStringVerbs(t *testing.T) {
	ctx := limitsCtx(1 << 16)
	str := object.NewString(strings.Repeat("\x00", 1<<20))
	bytes := object.NewBytes([]byte(str.Value()))
	errValue := object.NewError(stderrors.New(str.Value()))
	for _, format := range []string{"%x", "%X", "% #x", "%q", "%+q", "%#v"} {
		for _, arg := range []object.Object{str, bytes, errValue} {
			var before, after runtime.MemStats
			runtime.GC()
			runtime.ReadMemStats(&before)
			_, err := Sprintf(ctx, object.NewString(format), arg)
			runtime.ReadMemStats(&after)
			assertValueError(t, err, "sprintf result too large")
			assert.Less(t, after.TotalAlloc-before.TotalAlloc, uint64(4<<20), "format %q with %s", format, arg.Type())
		}
	}
}

// A precision that is not a minimum length does not count as one.
func TestFormatPrecisionNotMinimum(t *testing.T) {
	ctx := limitsCtx(100)
	for format, want := range map[string]string{
		"%.100000g": "1",
		"%.100000v": "1",
		"%.100000c": "A",
		"%.100000q": "'A'",
	} {
		var arg object.Object = object.NewFloat(1)
		if format == "%.100000c" || format == "%.100000q" {
			arg = object.NewInt(65)
		}
		result, err := Sprintf(ctx, object.NewString(format), arg)
		assert.Nil(t, err, format)
		assert.Equal(t, result.(*object.String).Value(), want)
	}
	_, err := Sprintf(ctx, object.NewString("%#.100000g"), object.NewFloat(1))
	assertValueError(t, err, "sprintf result too large")
}

// Every way a format can amplify its input is rejected before formatting.
func TestFormatAmplifiers(t *testing.T) {
	ctx := limitsCtx(1 << 16)
	big := object.NewString(strings.Repeat("y", 1000))
	shared := make([]object.Object, 1000)
	for i := range shared {
		shared[i] = big
	}
	ints := make([]object.Object, 1000)
	for i := range ints {
		ints[i] = object.NewInt(int64(i))
	}
	bytes := object.NewBytes(make([]byte, 1000))
	tests := []struct {
		name string
		args []object.Object
	}{
		{"star width", []object.Object{object.NewString(strings.Repeat("%[1]*[2]s", 100)), object.NewInt(1000000), object.NewString("x")}},
		{"negative star width", []object.Object{object.NewString("%*s%*s"), object.NewInt(-40000), object.NewString("x"), object.NewInt(-40000), object.NewString("x")}},
		{"precision", []object.Object{object.NewString("%.100000f"), object.NewFloat(1)}},
		{"star precision", []object.Object{object.NewString("%.*d"), object.NewInt(100000), object.NewInt(1)}},
		{"unicode precision", []object.Object{object.NewString("%.100000U"), object.NewInt(65)}},
		{"repeated index", []object.Object{object.NewString(strings.Repeat("%[1]s", 100)), big}},
		{"shared list", []object.Object{object.NewString("%v"), object.NewList(shared)}},
		{"shared list quoted", []object.Object{object.NewString("%q"), object.NewList(shared)}},
		{"width per element", []object.Object{object.NewString("%100v"), object.NewList(ints)}},
		{"width per map value", []object.Object{object.NewString("%100000v"), object.NewMap(map[string]object.Object{"a": object.NewInt(1)})}},
		{"width per byte", []object.Object{object.NewString("%100v"), bytes}},
		{"hex bytes", []object.Object{object.NewString(strings.Repeat("%[1]x", 40)), bytes}},
		{"extra arguments", []object.Object{object.NewString(""), object.NewList(shared)}},
		{"bad verb", []object.Object{object.NewString(strings.Repeat("%[1]d", 100)), big}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Sprintf(ctx, tt.args...)
			assertValueError(t, err, "sprintf result too large")
			_, err = Error(ctx, tt.args...)
			assertValueError(t, err, "error result too large")
		})
	}
}

// Formats under the limit still work, including ones whose width or
// precision is large but whose text is small.
func TestFormatUnderLimit(t *testing.T) {
	ctx := limitsCtx(100)
	result, err := Sprintf(ctx, object.NewString("%.1000000s|%5d|%v"),
		object.NewString("abc"), object.NewInt(42), object.NewList([]object.Object{object.NewInt(1)}))
	assert.Nil(t, err)
	assert.Equal(t, result.(*object.String).Value(), "abc|   42|[1]")

	// The limit is inclusive
	result, err = Sprintf(ctx, object.NewString("%100s"), object.NewString("x"))
	assert.Nil(t, err)
	assert.Equal(t, len(result.(*object.String).Value()), 100)
	_, err = Sprintf(ctx, object.NewString("%101s"), object.NewString("x"))
	assertValueError(t, err, "sprintf result too large")

	result, err = Error(ctx, object.NewString("%100s"), object.NewString("x"))
	assert.Nil(t, err)
	assert.Equal(t, len(result.(*object.Error).Message().Value()), 100)
	_, err = Error(ctx, object.NewString("%101s"), object.NewString("x"))
	assertValueError(t, err, "error result too large")
}

// error() still wraps an error argument formatted with %w.
func TestErrorWrapsWithLimit(t *testing.T) {
	ctx := limitsCtx(100)
	cause := object.NewError(stderrors.New("cause"))
	result, err := Error(ctx, object.NewString("outer: %w"), cause)
	assert.Nil(t, err)
	errObj := result.(*object.Error)
	assert.Equal(t, errObj.Message().Value(), "outer: cause")
	assert.True(t, stderrors.Is(errObj.Value(), cause.Value()))
}

// Counting a large list stops at the limit, so it does not walk the rest.
func BenchmarkFormatCountSharedList(b *testing.B) {
	big := object.NewString(strings.Repeat("y", 1<<20))
	shared := make([]object.Object, 1<<12)
	for i := range shared {
		shared[i] = big
	}
	ctx := limitsCtx(1 << 20)
	format := object.NewString("%v")
	list := object.NewList(shared)
	b.ReportAllocs()
	for b.Loop() {
		_, err := Sprintf(ctx, format, list)
		if err == nil {
			b.Fatal("expected an error")
		}
	}
}

// Counting adds a pass over the format and arguments to every call.
func BenchmarkSprintf(b *testing.B) {
	ctx := context.Background()
	ints := make([]object.Object, 100_000)
	for i := range ints {
		ints[i] = object.NewInt(int64(i))
	}
	cases := []struct {
		name string
		args []object.Object
	}{
		{"small", []object.Object{
			object.NewString("%s=%d %v %.2f"), object.NewString("key"), object.NewInt(42),
			object.NewList([]object.Object{object.NewInt(1), object.NewString("a")}), object.NewFloat(3.14159),
		}},
		{"list", []object.Object{object.NewString("%v"), object.NewList(ints)}},
	}
	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				if _, err := Sprintf(ctx, tc.args...); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkSprintfWidthAmplification(b *testing.B) {
	ctx := limitsCtx(1 << 20)
	format := object.NewString(strings.Repeat("%1000000[1]s", 32))
	x := object.NewString("x")
	b.ReportAllocs()
	for b.Loop() {
		_, err := Sprintf(ctx, format, x)
		if err == nil {
			b.Fatal("expected an error")
		}
	}
}
