package object

import (
	"context"
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

func TestStringEnumerateRuneIndexes(t *testing.T) {
	s := NewString("héllo")
	var keys []int64
	var values []string
	s.Enumerate(context.Background(), func(key, value Object) bool {
		keys = append(keys, key.(*Int).Value())
		values = append(values, value.(*String).Value())
		return true
	})
	assert.Equal(t, keys, []int64{0, 1, 2, 3, 4})
	assert.Equal(t, values, []string{"h", "é", "l", "l", "o"})
}

func TestStringIndexInvalidUTF8(t *testing.T) {
	// "\xc3\xa9" is "é". An invalid UTF-8 substring can match inside it.
	// Index and LastIndex skip those matches, so s[i] accepts each result.
	tests := []struct {
		s, substr   string
		first, last int64
	}{
		{"\xc3\xa9", "\xa9", -1, -1},
		{"\xc3\xa9", "\xc3", -1, -1},
		{"\xc3\xa9\xa9", "\xa9", 1, 1},
		{"\xa9\xc3\xa9\xa9", "\xa9", 0, 2},
		{"a\xffb\xff", "\xff", 1, 3},
		{"a\xffb", "\xffb", 1, 1},
	}
	for _, tc := range tests {
		s := NewString(tc.s)
		first, err := s.Index(NewString(tc.substr))
		assert.Nil(t, err)
		assert.Equal(t, first, NewInt(tc.first), "index %q in %q", tc.substr, tc.s)
		last, err := s.LastIndex(NewString(tc.substr))
		assert.Nil(t, err)
		assert.Equal(t, last, NewInt(tc.last), "last_index %q in %q", tc.substr, tc.s)
		for _, i := range []int64{tc.first, tc.last} {
			if i < 0 {
				continue
			}
			item, err := s.GetItem(NewInt(i))
			assert.Nil(t, err)
			assert.Equal(t, item.(*String).Value(), string([]rune(tc.substr)[0]))
		}
	}
}
