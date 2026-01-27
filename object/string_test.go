package object

import (
	"fmt"
	"testing"

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
		{"", 0, "", "index error: index out of range: 0"},
		{"a", 0, "a", ""},
		{"a", -1, "a", ""},
		{"a", -2, "a", "index error: index out of range: -2"},
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

func TestStringHashKey(t *testing.T) {
	a := NewString("hello")
	b := NewString("hello")
	c := NewString("goodbye")
	d := NewString("goodbye")

	assert.Equal(t, b.HashKey(), a.HashKey())
	assert.Equal(t, d.HashKey(), c.HashKey())
	assert.NotEqual(t, c.HashKey(), a.HashKey())

	assert.Equal(t, a.HashKey(), HashKey{Type: STRING, StrValue: "hello"})
}
