package object

import (
	"context"
	"errors"
	"image/color"
	"math"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/deepnoodle-ai/risor/v2/pkg/bytecode"
	"github.com/deepnoodle-ai/wonton/assert"
)

// longLeaf is a host type with no case in formatSize, so it counts the
// length of its Inspect text.
type longLeaf struct{ NilType }

func (l *longLeaf) Type() Type      { return "long_leaf" }
func (l *longLeaf) Inspect() string { return strings.Repeat("x", 5000) }

func testClosure(source string, defaults []any) *Closure {
	code := bytecode.NewCode(bytecode.CodeParams{Source: source})
	return NewClosure(bytecode.NewFunction(bytecode.FunctionParams{
		Name:       "f",
		Parameters: []string{"a", "b", "c"},
		Defaults:   defaults,
		Code:       code,
	}))
}

// formatSize counts exactly the length of the text that Inspect prints, for
// every Object type. A bound measured with it is a bound on the text, so a
// type whose text the count missed could make shared references print
// exponential text (#583). A type with no case in formatSize counts its
// Inspect text, so a new type keeps the invariant.
func TestFormatSizeMatchesInspect(t *testing.T) {
	cyclic := NewList([]Object{NewInt(1)})
	cyclic.Append(cyclic)
	cyclicMap := NewMap(map[string]Object{"k": NewInt(1)})
	cyclicMap.Set("self", cyclicMap)
	var self Object
	selfCell := NewCell(&self)
	self = selfCell
	partialList := NewList(nil)
	partialList.Append(NewPartial(partialList, []Object{partialList}))
	var held Object = NewString("v")
	builtin := NewBuiltin("fn", nil).WithModule(NewBuiltinsModule("mod", nil))
	gs := NewGoStruct(reflect.ValueOf(&struct{ A, B int }{1, 2}), DefaultRegistry())
	gf := NewGoFunc(reflect.ValueOf(strings.ToUpper), "to_upper", DefaultRegistry())
	escapes := "tab\t quote\" backslash\\ nul\x00 del\x7f bell\a é \u00a0 \U000e0001 bad\xff\xfe 日本"
	longSource := strings.Repeat("let x = 1\n  \t", 200)

	objs := []Object{
		NewInt(0),
		NewInt(math.MinInt64),
		NewInt(math.MaxInt64),
		NewFloat(1e308),
		NewFloat(-0.000123),
		NewFloat(math.Inf(-1)),
		NewFloat(math.NaN()),
		True,
		False,
		Nil,
		NewByte(255),
		NewString(""),
		NewString(escapes),
		NewString(`"quoted"`),
		NewString(`"a"b"`),
		NewBytes([]byte(escapes)),
		NewBytes(nil),
		NewError(errors.New(escapes)),
		NewList(nil),
		NewList([]Object{NewInt(1), NewString(escapes), Nil, NewList(nil)}),
		NewMap(nil),
		NewMap(map[string]Object{escapes: NewInt(1), "b": NewList([]Object{True}), "": Nil}),
		NewCell(nil),
		NewCell(&held),
		NewPartial(NewInt(1), nil),
		NewPartial(builtin, []Object{NewInt(1), NewString("a"), NewList(nil)}),
		cyclic,
		cyclicMap,
		selfCell,
		partialList,
		testClosure("", nil),
		testClosure("return a", []any{nil, int64(2), "x"}),
		testClosure(longSource, []any{nil, 1.5, bytecode.NilDefault{}}),
		builtin,
		NewBuiltin("plain", nil),
		gf,
		gs,
		NewBuiltinsModule("mod", nil),
		NewDynamicAttr("attr", nil),
		NewMapKeyIter(NewMap(nil)),
		NewRange(-5, 100, 3),
		NewColor(color.RGBA{R: 255, G: 128, B: 1, A: 255}),
		NewTime(time.Date(2026, 10, 31, 1, 2, 3, 0, time.UTC)),
		&longLeaf{},
		NewList([]Object{&longLeaf{}, &longLeaf{}}),
	}
	for _, obj := range objs {
		text := obj.Inspect()
		assert.Equal(t, formatSize(obj, nil, math.MaxUint64), uint64(len(text)), text)

		// Inside a list, the text gains 2 brackets. CheckFormatSize accepts
		// a limit of exactly that size and rejects one byte less.
		wrapped := NewList([]Object{obj})
		exact := WithLimits(context.Background(), Limits{MaxBytesSize: int64(len(text) + 2)})
		assert.Nil(t, CheckFormatSize(exact, "x", wrapped), text)
		short := WithLimits(context.Background(), Limits{MaxBytesSize: int64(len(text) + 1)})
		assert.NotNil(t, CheckFormatSize(short, "x", wrapped), text)
	}
}

// quotedSize matches the length of strconv.Quote for every byte value and
// for runes on each escaping path.
func TestQuotedSize(t *testing.T) {
	var all []byte
	for i := 0; i < 256; i++ {
		all = append(all, byte(i))
	}
	for _, s := range []string{
		"", "a", string(all), "\u00a0\u2028\ufeff\U000e0001\U0010ffff", "日本語",
		"\xed\xa0\x80", // a surrogate, invalid in UTF-8
	} {
		assert.Equal(t, quotedSize(s), uint64(len(NewString(s).Inspect())), s)
		assert.Equal(t, quotedBytesSize([]byte(s)), uint64(len(NewBytes([]byte(s)).Inspect())-7), s)
	}
}

// A closure prints its whole source, so a list that repeats one closure counts
// the source each time (#583).
func TestInspectSharedClosure(t *testing.T) {
	shared := NewList([]Object{testClosure(strings.Repeat("a", 1024), nil)})
	for range 22 {
		shared = NewList([]Object{shared, shared})
	}
	assert.Equal(t, shared.Inspect(), "[...]")
	assert.Equal(t, InspectMessage(shared), "[...]")
	assert.NotNil(t, CheckFormatSize(context.Background(), "string", shared))
}
