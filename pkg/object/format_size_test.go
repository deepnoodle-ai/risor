package object

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/deepnoodle-ai/wonton/assert"
)

func TestCheckFormatSize(t *testing.T) {
	ctx := WithLimits(context.Background(), Limits{MaxBytesSize: 1024})
	s := NewString(strings.Repeat("a", 600))

	// Shared references count each time they are referenced
	assert.NotNil(t, CheckFormatSize(ctx, "string", NewList([]Object{s, s})))
	assert.Nil(t, CheckFormatSize(ctx, "string", NewList([]Object{s})))
	assert.NotNil(t, CheckFormatSize(ctx, "string",
		NewMap(map[string]Object{"a": s, "b": s})))
	assert.NotNil(t, CheckFormatSize(ctx, "string", s, s))

	// A partial counts its arguments
	assert.NotNil(t, CheckFormatSize(ctx, "string", NewPartial(NewInt(1), []Object{s, s})))

	// A list that doubles 60 times is rejected without walking 2^60 items
	l := NewList([]Object{NewInt(1)})
	for i := 0; i < 60; i++ {
		l = NewList([]Object{l, l})
	}
	assert.NotNil(t, CheckFormatSize(ctx, "string", l))

	// A cycle counts once, as Inspect prints a placeholder for it
	cyclic := NewList([]Object{NewInt(1)})
	cyclic.Append(cyclic)
	assert.Nil(t, CheckFormatSize(ctx, "string", cyclic))

	// A cell counts its value, as Inspect prints it
	var value Object = NewList([]Object{s, s})
	assert.NotNil(t, CheckFormatSize(ctx, "string", NewCell(&value)))
	value = l
	assert.NotNil(t, CheckFormatSize(ctx, "string", NewCell(&value)))
	value = NewList([]Object{s})
	assert.Nil(t, CheckFormatSize(ctx, "string", NewCell(&value)))
	assert.Nil(t, CheckFormatSize(ctx, "string", NewCell(nil)))

	// A cell that holds itself counts its placeholder, directly or through a
	// list: cell(cell(...)) and [cell([...])]
	var self Object
	selfCell := NewCell(&self)
	self = selfCell
	assert.Equal(t, formatSize(selfCell, nil, 1024), uint64(15))
	assert.Nil(t, CheckFormatSize(ctx, "string", selfCell))

	var held Object
	holder := NewList([]Object{NewCell(&held)})
	held = holder
	assert.Equal(t, formatSize(holder, nil, 1024), uint64(13))
	assert.Nil(t, CheckFormatSize(ctx, "string", holder))

	// The walk stops even when the limit is used up
	assert.Equal(t, formatSize(selfCell, nil, 0), uint64(15))
	assert.Equal(t, formatSize(holder, nil, 0), uint64(13))

	// An error counts its message, as Inspect prints it: error("...")
	e := NewError(errors.New(strings.Repeat("a", 600)))
	assert.Equal(t, formatSize(e, nil, 1024), uint64(609))
	assert.NotNil(t, CheckFormatSize(ctx, "string", NewList([]Object{e, e})))
	assert.Nil(t, CheckFormatSize(ctx, "string", NewList([]Object{e})))
}

// A list that repeats one error prints its message each time, so the bounds
// of Inspect, InspectWithLimits, and InspectMessage count the messages.
func TestInspectSharedError(t *testing.T) {
	// 2^30 copies of a 1 KiB message
	shared := NewList([]Object{NewError(errors.New(strings.Repeat("a", 1024)))})
	for range 30 {
		shared = NewList([]Object{shared, shared})
	}
	assert.Equal(t, shared.Inspect(), "[...]")
	assert.Equal(t, InspectMessage(shared), "[...]")
	assert.Equal(t, InspectWithLimits(context.Background(), shared), "[...]")
	assert.NotNil(t, CheckFormatSize(context.Background(), "string", shared))
}

// Inspect prints the function of a partial as well as its arguments, so a
// partial whose function is another partial is counted through both.
func TestInspectSharedPartial(t *testing.T) {
	// Each level prints the one below it twice, as the function and as the
	// argument, so the full text would repeat the innermost partial 2^40
	// times. Its 1 KiB strings make the walk stop after about 2^16 of them.
	s := NewString(strings.Repeat("a", 1024))
	p := NewPartial(s, []Object{s})
	for range 40 {
		p = NewPartial(p, []Object{p})
	}
	// Check the count first, so a broken count fails here instead of
	// building the full text below
	counted := formatSize(p, nil, inspectMaxSize) > inspectMaxSize
	assert.True(t, counted)
	if !counted {
		return
	}
	list := NewList([]Object{p})
	assert.Equal(t, list.Inspect(), "[...]")
	assert.Equal(t, p.Inspect(), "partial(...)")
	assert.Equal(t, InspectMessage(p), "partial(...)")
	assert.NotNil(t, CheckFormatSize(context.Background(), "string", p))

	// A small partial counts its function and its argument
	small := NewPartial(NewPartial(NewInt(1), []Object{NewInt(2)}), []Object{NewInt(3)})
	assert.Equal(t, formatSize(small, nil, 100), uint64(len("partial(partial(1, 2), 3)")))
	assert.Equal(t, NewList([]Object{small}).Inspect(), "[partial(partial(1, 2), 3)]")
}

// A partial goes on the path when it is printed or measured, so a cycle
// through its arguments or its function prints as a placeholder (#583).
func TestPartialCycle(t *testing.T) {
	raised := WithLimits(context.Background(), Limits{MaxBytesSize: 2 * DefaultMaxBytesSize})

	// A list that holds a partial whose argument is that list
	argList := NewList(nil)
	argPartial := NewPartial(NewInt(1), []Object{argList})
	argList.Append(argPartial)

	// A list that holds a partial whose function is that list
	fnList := NewList(nil)
	fnPartial := NewPartial(fnList, []Object{NewInt(2)})
	fnList.Append(fnPartial)

	// A map and a cell that hold a partial that holds them
	m := NewMap(nil)
	m.Set("a", NewPartial(NewInt(1), []Object{m}))
	var value Object
	cell := NewCell(&value)
	value = NewPartial(cell, nil)

	for _, tt := range []struct {
		obj  Object
		want string
	}{
		{argList, "[partial(1, [...])]"},
		{argPartial, "partial(1, [partial(...)])"},
		{fnList, "[partial([...], 2)]"},
		{fnPartial, "partial([partial(...)], 2)"},
		{m, `{"a": partial(1, {...})}`},
		{cell, "cell(partial(cell(...), ))"},
	} {
		assert.True(t, formatSize(tt.obj, nil, inspectMaxSize) < 100, tt.want)
		assert.Nil(t, CheckFormatSize(context.Background(), "string", tt.obj), tt.want)
		assert.Equal(t, tt.obj.Inspect(), tt.want)
		assert.Equal(t, InspectMessage(tt.obj), tt.want)
		assert.Equal(t, InspectWithLimits(context.Background(), tt.obj), tt.want)
		assert.Equal(t, InspectWithLimits(raised, tt.obj), tt.want)
	}
}

// Inspect and String have no context, so they bound the text of a list, map,
// or cell with the default byte limit and print a placeholder past it (#583).
func TestInspectSharedReferencesBounded(t *testing.T) {
	// Each level references the previous one twice, so the full text would
	// repeat the 1 KiB string 2^40 times. The walk stops after about 2^16.
	shared := NewList([]Object{NewString(strings.Repeat("a", 1024))})
	for range 40 {
		shared = NewList([]Object{shared, shared})
	}
	assert.Equal(t, shared.Inspect(), "[...]")
	assert.Equal(t, shared.String(), "[...]")
	assert.Equal(t, fmt.Sprintf("%v", shared), "[...]")

	m := NewMap(map[string]Object{"a": shared})
	assert.Equal(t, m.Inspect(), "{...}")
	assert.Equal(t, m.String(), "{...}")

	var value Object = shared
	cell := NewCell(&value)
	assert.Equal(t, cell.Inspect(), "cell(...)")
	assert.Equal(t, cell.String(), "cell(...)")

	// A container that holds the large one is bounded as a whole
	outer := NewList([]Object{NewInt(1), shared})
	assert.Equal(t, outer.Inspect(), "[...]")

	// A small value with shared references still prints in full
	small := NewList([]Object{NewInt(1)})
	pair := NewList([]Object{small, small})
	assert.Equal(t, pair.Inspect(), "[[1], [1]]")
	assert.Equal(t, NewMap(map[string]Object{"a": pair}).Inspect(), `{"a": [[1], [1]]}`)
}

func TestInspectBound(t *testing.T) {
	// A list counts 2 for its brackets, and a string counts its length and 2
	// quotes
	s := NewString(strings.Repeat("a", inspectMaxSize-4))
	assert.Equal(t, formatSize(NewList([]Object{s}), nil, inspectMaxSize), uint64(inspectMaxSize))
	assert.Equal(t, NewList([]Object{s, NewInt(1)}).Inspect(), "[...]")

	// A value far over the bound of InspectMessage still prints in full
	big := NewList([]Object{NewString(strings.Repeat("a", errorfMaxTextSize))})
	assert.Equal(t, len(big.Inspect()), errorfMaxTextSize+4)
}

// InspectWithLimits bounds a list, map, or cell with the byte limit in ctx
// instead of the default bound of Inspect (#583).
func TestInspectWithLimits(t *testing.T) {
	s := NewString(strings.Repeat("a", 600))
	list := NewList([]Object{s, s})
	m := NewMap(map[string]Object{"a": s, "b": s})
	var value Object = list
	cell := NewCell(&value)

	// A limit below the text prints a placeholder
	low := WithLimits(context.Background(), Limits{MaxBytesSize: 1024})
	assert.Equal(t, InspectWithLimits(low, list), "[...]")
	assert.Equal(t, InspectWithLimits(low, m), "{...}")
	assert.Equal(t, InspectWithLimits(low, cell), "cell(...)")

	// Other values print as Inspect prints them, whatever their size
	assert.Equal(t, InspectWithLimits(low, NewString(strings.Repeat("a", 2000))),
		NewString(strings.Repeat("a", 2000)).Inspect())
	assert.Equal(t, InspectWithLimits(low, NewInt(1)), "1")

	// A limit above the text prints it in full
	high := WithLimits(context.Background(), Limits{MaxBytesSize: 4096})
	assert.Equal(t, InspectWithLimits(high, list), list.Inspect())
	assert.Equal(t, InspectWithLimits(high, m), m.Inspect())
	assert.Equal(t, InspectWithLimits(high, cell), cell.Inspect())

	// A limit above the default bound of Inspect prints text that Inspect
	// prints as a placeholder
	big := NewList([]Object{NewString(strings.Repeat("a", inspectMaxSize))})
	assert.Equal(t, big.Inspect(), "[...]")
	raised := WithLimits(context.Background(), Limits{MaxBytesSize: 2 * inspectMaxSize})
	assert.Equal(t, len(InspectWithLimits(raised, big)), inspectMaxSize+4)
}

// InspectMessage bounds the text of a value for an error message, as Errorf
// bounds its arguments (#583).
func TestInspectMessage(t *testing.T) {
	assert.Equal(t, InspectMessage(NewInt(1)), "1")
	assert.Equal(t, InspectMessage(NewFloat(1e21)), "1000000000000000000000")
	assert.Equal(t, InspectMessage(NewString("a")), `"a"`)
	assert.Equal(t, InspectMessage(NewList([]Object{NewInt(1), NewString("a")})), `[1, "a"]`)

	// The full text would repeat the innermost list 2^40 times
	shared := NewList([]Object{NewInt(1)})
	for range 40 {
		shared = NewList([]Object{shared, shared})
	}
	assert.Equal(t, InspectMessage(shared), "[...]")
	assert.Equal(t, InspectMessage(NewMap(map[string]Object{"a": shared})), "{...}")

	// A list counts 2 for its brackets, and a string counts its length and 2
	// quotes
	s := NewString(strings.Repeat("a", errorfMaxTextSize-4))
	atBound := NewList([]Object{s})
	assert.Equal(t, InspectMessage(atBound), atBound.Inspect())
	assert.Equal(t, InspectMessage(NewList([]Object{s, NewInt(1)})), "[...]")

	// A cycle prints as Inspect prints it
	cyclic := NewList([]Object{NewInt(1)})
	cyclic.Append(cyclic)
	assert.Equal(t, InspectMessage(cyclic), "[1, [...]]")
}
