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
	s := NewString(string(make([]byte, 600)))

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

	// A cell that holds itself counts once, directly or through a list
	var self Object
	selfCell := NewCell(&self)
	self = selfCell
	assert.Equal(t, formatSize(selfCell, nil, 1024), uint64(2))
	assert.Nil(t, CheckFormatSize(ctx, "string", selfCell))

	var held Object
	holder := NewList([]Object{NewCell(&held)})
	held = holder
	assert.Equal(t, formatSize(holder, nil, 1024), uint64(4))
	assert.Nil(t, CheckFormatSize(ctx, "string", holder))

	// The walk stops even when the limit is used up
	assert.Equal(t, formatSize(selfCell, nil, 0), uint64(2))
	assert.Equal(t, formatSize(holder, nil, 0), uint64(4))

	// An error counts its message, as Inspect prints it
	e := NewError(errors.New(strings.Repeat("a", 600)))
	assert.Equal(t, formatSize(e, nil, 1024), uint64(600))
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
	// A list counts 2 for its brackets, and a string counts its length
	s := NewString(strings.Repeat("a", inspectMaxSize-2))
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

	// A list counts 2 for its brackets, and a string counts its length
	s := NewString(strings.Repeat("a", errorfMaxTextSize-2))
	atBound := NewList([]Object{s})
	assert.Equal(t, InspectMessage(atBound), atBound.Inspect())
	assert.Equal(t, InspectMessage(NewList([]Object{s, NewInt(1)})), "[...]")

	// A cycle prints as Inspect prints it
	cyclic := NewList([]Object{NewInt(1)})
	cyclic.Append(cyclic)
	assert.Equal(t, InspectMessage(cyclic), "[1, [...]]")
}
