package object

import (
	"context"
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
}
