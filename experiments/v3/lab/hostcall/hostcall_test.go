package hostcall

import (
	"context"
	"strings"
	"testing"

	"github.com/deepnoodle-ai/wonton/assert"
)

// The host's functions.
func hasPrefix(s, prefix string) (bool, error) { return strings.HasPrefix(s, prefix), nil }

func clamp(x, lo, hi int64) (int64, error) { return min(max(x, lo), hi), nil }

// clampInt takes Go's int, which the evaluator never produces.
func clampInt(x, lo, hi int) (int, error) { return min(max(x, lo), hi), nil }

// The functions as a host would write them against the native signature.
func hasPrefixNative(_ context.Context, args []any) (any, error) {
	if len(args) != 2 {
		return nil, argError(len(args), nil)
	}
	s, ok := args[0].(string)
	if !ok {
		return nil, argError(0, args[0])
	}
	prefix, ok := args[1].(string)
	if !ok {
		return nil, argError(1, args[1])
	}
	return strings.HasPrefix(s, prefix), nil
}

func clampNative(_ context.Context, args []any) (any, error) {
	if len(args) != 3 {
		return nil, argError(len(args), nil)
	}
	x, ok := args[0].(int64)
	if !ok {
		return nil, argError(0, args[0])
	}
	lo, ok := args[1].(int64)
	if !ok {
		return nil, argError(1, args[1])
	}
	hi, ok := args[2].(int64)
	if !ok {
		return nil, argError(2, args[2])
	}
	return min(max(x, lo), hi), nil
}

// Typed leaves, as the compiler would make them when the checker knows the
// type of a global.
func globString(i int) func(*frame) string {
	return func(fr *frame) string { return fr.globals[i].(string) }
}

func globInt(i int) func(*frame) int64 {
	return func(fr *frame) int64 { return fr.globals[i].(int64) }
}

func typedNode[R any](bound any, ok bool) func(*frame) R {
	if !ok {
		panic("typed binding failed")
	}
	return bound.(func(*frame) R)
}

func newFrame() *frame {
	return &frame{
		ctx: context.Background(),
		// clamp(5000, 1000, 9000) is 5000: the result is above 255, so
		// boxing it allocates.
		globals: []any{"refs/heads/main", "refs/heads/", int64(5000), int64(1000), int64(9000)},
		args:    make([]any, 0, 8),
	}
}

var (
	prefixCases = map[string]node{
		"native":  call(hasPrefixNative, glob(0), glob(1)),
		"adapter": call(Adapt2(hasPrefix), glob(0), glob(1)),
		"reflect": call(Reflect(hasPrefix), glob(0), glob(1)),
	}
	clampCases = map[string]node{
		"native":      call(clampNative, glob(2), glob(3), glob(4)),
		"adapter":     call(Adapt3(clamp), glob(2), glob(3), glob(4)),
		"convert_int": call(Convert3(clampInt), glob(2), glob(3), glob(4)),
		"reflect":     call(Reflect(clamp), glob(2), glob(3), glob(4)),
		"reflect_int": call(Reflect(clampInt), glob(2), glob(3), glob(4)),
	}
	prefixTyped = typedNode[bool](Typed2(hasPrefix)(globString(0), globString(1)))
	clampTyped  = typedNode[int64](Typed3(clamp)(globInt(2), globInt(3), globInt(4)))
)

func TestAgree(t *testing.T) {
	fr := newFrame()
	for name, n := range prefixCases {
		assert.Equal(t, n(fr), true, name)
	}
	for name, n := range clampCases {
		assert.Equal(t, n(fr), int64(5000), name)
	}
	assert.Equal(t, prefixTyped(fr), true)
	assert.Equal(t, clampTyped(fr), int64(5000))
}

var (
	sink     any
	sinkBool bool
	sinkInt  int64
)

func BenchmarkCall(b *testing.B) {
	fr := newFrame()
	for _, name := range []string{"native", "adapter", "reflect"} {
		n := prefixCases[name]
		b.Run("hasPrefix/"+name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				sink = n(fr)
			}
		})
	}
	b.Run("hasPrefix/typed", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			sinkBool = prefixTyped(fr)
		}
	})
	b.Run("hasPrefix/direct_go", func(b *testing.B) {
		s, p := fr.globals[0].(string), fr.globals[1].(string)
		b.ReportAllocs()
		for b.Loop() {
			sinkBool, _ = hasPrefix(s, p)
		}
	})
	for _, name := range []string{"native", "adapter", "convert_int", "reflect", "reflect_int"} {
		n := clampCases[name]
		b.Run("clamp/"+name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				sink = n(fr)
			}
		})
	}
	b.Run("clamp/typed", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			sinkInt = clampTyped(fr)
		}
	})
}
