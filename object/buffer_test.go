package object

import (
	"bytes"
	"context"
	"testing"

	"github.com/deepnoodle-ai/wonton/assert"
)

func TestBuffer(t *testing.T) {
	t.Run("Inspect", func(t *testing.T) {
		b := &Buffer{value: bytes.NewBufferString("hello")}
		assert.Equal(t, b.Inspect(), "buffer(\"hello\")")
	})

	t.Run("Type", func(t *testing.T) {
		b := &Buffer{value: bytes.NewBufferString("hello")}
		assert.Equal(t, b.Type(), BUFFER)
	})

	t.Run("Value", func(t *testing.T) {
		b := &Buffer{value: bytes.NewBufferString("hello")}
		assert.Equal(t, b.Value(), bytes.NewBufferString("hello"))
	})

	t.Run("Interface", func(t *testing.T) {
		b := &Buffer{value: bytes.NewBufferString("hello")}
		assert.Equal(t, b.Interface(), bytes.NewBufferString("hello"))
	})

	t.Run("String", func(t *testing.T) {
		b := &Buffer{value: bytes.NewBufferString("hello")}
		assert.Equal(t, b.String(), "buffer(\"hello\")")
	})

	t.Run("IsTruthy", func(t *testing.T) {
		b := &Buffer{value: bytes.NewBufferString("hello")}
		assert.True(t, b.IsTruthy())

		b = &Buffer{value: bytes.NewBuffer([]byte{})}
		assert.False(t, b.IsTruthy())
	})
}

func TestBufferMethods(t *testing.T) {
	testCase := []struct {
		name   string
		b      *bytes.Buffer
		args   []Object
		result Object
	}{
		{
			name:   "bytes",
			b:      bytes.NewBufferString("hello"),
			args:   []Object{},
			result: NewByteSlice([]byte("hello")),
		},
		{
			name:   "string",
			b:      bytes.NewBufferString("hello"),
			args:   []Object{},
			result: NewString("hello"),
		},
		{
			name:   "write",
			b:      bytes.NewBufferString("hello"),
			args:   []Object{NewString(" world")},
			result: NewInt(6),
		},
		{
			name:   "read",
			b:      bytes.NewBufferString("hello"),
			args:   []Object{NewInt(3)},
			result: NewByteSlice([]byte("hel")),
		},
		{
			name:   "cap",
			b:      bytes.NewBufferString("hello"),
			args:   []Object{},
			result: NewInt(8),
		},
		{
			name:   "len",
			b:      bytes.NewBufferString("hello"),
			args:   []Object{},
			result: NewInt(5),
		},
		{
			name:   "reset",
			b:      bytes.NewBufferString("hello"),
			args:   []Object{},
			result: Nil,
		},
	}
	for _, tc := range testCase {
		t.Run(tc.name, func(t *testing.T) {
			buf := NewBuffer(tc.b)
			methodObj, ok := buf.GetAttr(tc.name)
			assert.True(t, ok, "method not found")
			method, ok := methodObj.(*Builtin)
			assert.True(t, ok, "method is not a builtin")
			result := method.Call(context.Background(), tc.args...)
			assert.Equal(t, result, tc.result)
		})
	}
}
