package object

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/deepnoodle-ai/wonton/assert"
)

func TestObjectString(t *testing.T) {
	tm, _ := time.Parse(time.RFC3339, "2009-11-10T23:00:00Z")
	tests := []struct {
		input    Object
		expected string
	}{
		{True, "true"},
		{False, "false"},
		{Nil, "nil"},
		{NewError(errors.New("kaboom")), "kaboom"},
		{NewFloat(3.0), "3"},
		{NewInt(-3), "-3"},
		{NewString("foo"), "foo"},
		{NewList([]Object{NewInt(1), NewInt(2)}), "[1, 2]"},
		{NewMap(map[string]Object{"foo": NewInt(1), "bar": NewInt(2)}), `{"bar": 2, "foo": 1}`},
		{NewTime(tm), "time(\"2009-11-10T23:00:00Z\")"},
	}
	for _, tt := range tests {
		str, ok := tt.input.(fmt.Stringer)
		if !ok {
			t.Errorf("String() not implemented for %T", tt.input)
			continue
		}
		result := str.String()
		if result != tt.expected {
			t.Errorf("String() wrong. want=%q, got=%q", tt.expected, result)
		}
	}
}

func TestComparisons(t *testing.T) {
	tests := []struct {
		left        Object
		right       Object
		expected    int
		expectedErr error
	}{
		{NewInt(1), NewInt(1), 0, nil},
		{NewInt(1), NewInt(2), -1, nil},
		{NewInt(2), NewInt(1), 1, nil},
		{NewFloat(1.0), NewFloat(1.0), 0, nil},
		{NewFloat(1.0), NewFloat(2.0), -1, nil},
		{NewFloat(2.0), NewFloat(1.0), 1, nil},
		{NewString("a"), NewString("a"), 0, nil},
		{NewString("a"), NewString("b"), -1, nil},
		{NewString("b"), NewString("a"), 1, nil},
		{True, True, 0, nil},
		{True, False, 1, nil},
		{False, True, -1, nil},
		{Nil, Nil, 0, nil},
		{Nil, True, 0, TypeErrorf("type error: unable to compare nil and bool")},
		{NewInt(1), NewFloat(1.0), 0, nil},
		{NewInt(1), NewFloat(2.0), -1, nil},
		{NewInt(1), NewFloat(0.0), 1, nil},
		{NewFloat(1.0), NewInt(1), 0, nil},
		{NewFloat(1.0), NewInt(2), -1, nil},
		{NewFloat(1.0), NewInt(0), 1, nil},
		{NewInt(1), NewString("1"), 0, TypeErrorf("type error: unable to compare int and string")},
		{NewString("1"), NewInt(1), 0, TypeErrorf("type error: unable to compare string and int")},
		{NewFloat(1.0), NewString("1"), 0, TypeErrorf("type error: unable to compare float and string")},
		{NewString("1"), NewFloat(1.0), 0, TypeErrorf("type error: unable to compare string and float")},
		{NewByte(1), NewByte(1), 0, nil},
		{NewByte(1), NewByte(2), -1, nil},
		{NewByte(2), NewByte(1), 1, nil},
		{NewByte(1), NewInt(1), 0, nil},
		{NewByte(1), NewInt(2), -1, nil},
		{NewByte(2), NewInt(1), 1, nil},
		{NewInt(1), NewByte(1), 0, nil},
		{NewInt(1), NewByte(2), -1, nil},
		{NewInt(2), NewByte(1), 1, nil},
		{NewByte(1), NewFloat(1.0), 0, nil},
		{NewByte(1), NewFloat(2.0), -1, nil},
		{NewByte(2), NewFloat(1.0), 1, nil},
		{NewFloat(1.0), NewByte(1), 0, nil},
		{NewFloat(1.0), NewByte(2), -1, nil},
		{NewFloat(1.0), NewByte(0), 1, nil},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("%s %s", tt.left.Type(), tt.right.Type()), func(t *testing.T) {
			comparable, ok := tt.left.(Comparable)
			assert.True(t, ok)
			cmp, cmpErr := comparable.Compare(tt.right)
			assert.Equal(t, cmp, tt.expected)
			if tt.expectedErr == nil {
				assert.Nil(t, cmpErr)
			} else {
				assert.Error(t, cmpErr)
				assert.Equal(t, cmpErr.Error(), tt.expectedErr.Error())
			}
		})
	}
}

func TestPrintableValue(t *testing.T) {
	type testCase struct {
		obj      Object
		expected any
	}

	testTime, err := time.Parse("2006-01-02", "2021-01-01")
	assert.NoError(t, err)

	builtin := func(ctx context.Context, args ...Object) (Object, error) {
		return nil, nil
	}

	cases := []testCase{
		{NewString("hello"), "hello"},
		{NewByte(5), byte(5)},
		{NewInt(42), int64(42)},
		{NewFloat(42.42), 42.42},
		{NewBool(true), true},
		{NewBool(false), false},
		{Errorf("error"), "error"}, // PrintableValue returns error message as string
		{obj: Nil, expected: "nil"},
		{obj: NewTime(testTime), expected: "2021-01-01T00:00:00Z"},
		{obj: NewBuiltin("foo", builtin), expected: "builtin(foo)"},
		{ // strings printed inside lists are quoted in Risor
			obj: NewList([]Object{
				NewString("hello"),
				NewInt(42),
			}),
			expected: `["hello", 42]`,
		},
		{ // strings printed inside maps are quoted in Risor
			obj: NewMap(map[string]Object{
				"a": NewInt(42),
				"b": NewString("hello"),
				"c": Nil,
			}),
			expected: `{"a": 42, "b": "hello", "c": nil}`,
		},
	}
	for _, tc := range cases {
		t.Run(fmt.Sprintf("%v", tc.expected), func(t *testing.T) {
			got := PrintableValue(tc.obj)
			// Handle error comparison specially since error pointers differ
			if gotErr, ok := got.(error); ok {
				expectedErr, ok := tc.expected.(string)
				assert.True(t, ok)
				assert.Equal(t, gotErr.Error(), expectedErr)
				return
			}
			assert.Equal(t, got, tc.expected)
		})
	}
}
