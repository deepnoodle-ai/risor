package object

import (
	"testing"

	"github.com/deepnoodle-ai/wonton/assert"
)

func TestIntCompare(t *testing.T) {
	one := NewInt(1)
	two := NewFloat(2.0)
	thr := NewInt(3)

	tests := []struct {
		first    Comparable
		second   Object
		expected int
	}{
		{one, two, -1},
		{two, one, 1},
		{one, one, 0},
		{two, thr, -1},
		{thr, two, 1},
		{two, two, 0},
	}
	for _, tc := range tests {
		result, err := tc.first.Compare(tc.second)
		assert.Nil(t, err)
		assert.Equal(t, result, tc.expected,
			"first: %v, second: %v", tc.first, tc.second)
	}
}

func TestIntEquals(t *testing.T) {
	oneInt := NewInt(1)
	twoFlt := NewFloat(2.0)
	twoInt := NewInt(2)

	tests := []struct {
		first    Object
		second   Object
		expected bool
	}{
		{oneInt, twoFlt, false},
		{oneInt, twoInt, false},
		{oneInt, oneInt, true},
		{twoInt, twoFlt, true},
		{twoFlt, twoFlt, true},
	}
	for _, tc := range tests {
		result, ok := tc.first.Equals(tc.second).(*Bool)
		assert.True(t, ok)
		assert.Equal(t, result.Value(), tc.expected,
			"first: %v, second: %v", tc.first, tc.second)
	}
}

func TestIntBasics(t *testing.T) {
	value := NewInt(-3)
	assert.Equal(t, value.Type(), INT)
	assert.Equal(t, value.Value(), int64(-3))
	assert.Equal(t, value.String(), "-3")
	assert.Equal(t, value.Inspect(), "-3")
	assert.Equal(t, value.Interface(), int64(-3))
}
