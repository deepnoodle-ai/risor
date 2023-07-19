package object

import (
	"errors"
	"fmt"
	"testing"

	"time"
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
		// {NewBreak(), "break"},
		// {NewContinue(), "continue"},
		{NewError(errors.New("kaboom")), "error(kaboom)"},
		// {NewReturn(NewInt(42)), "return(int(42))"},
		{NewFloat(3.0), "3"},
		{NewInt(-3), "-3"},
		{NewString("foo"), "foo"},
		// {NewModule("my-scope"), "module(my-scope)"},
		{NewList([]Object{NewInt(1), NewInt(2)}), "list([1, 2])"},
		{NewSet([]Object{True, Nil}), "set(true, nil)"},
		{NewMap(map[string]Object{"foo": NewInt(1), "bar": NewInt(2)}), `map("bar": 2, "foo": 1)`},
		{NewTime(tm), "2009-11-10 23:00:00 +0000 UTC"},
	}

	for _, tt := range tests {
		str, ok := tt.input.(fmt.Stringer)
		if !ok {
			t.Errorf("object.String() not implemented for %T", tt.input)
			continue
		}
		if str.String() != tt.expected {
			t.Errorf("object.String() wrong. want=%q, got=%q", tt.expected, str.String())
		}
	}
}
