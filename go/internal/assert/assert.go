// Package assert provides small, zero-dependency test assertions built on the
// Go standard library. Every helper calls t.Helper and fails immediately via
// t.Fatalf.
package assert

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
)

// Equal fails the test if got and want are not deeply equal.
func Equal(t testing.TB, got, want any, msgAndArgs ...any) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("%sgot:  %#v\nwant: %#v", formatPrefix(msgAndArgs), got, want)
	}
}

// NoError fails the test if err is non-nil.
func NoError(t testing.TB, err error, msgAndArgs ...any) {
	t.Helper()
	if err != nil {
		t.Fatalf("%sunexpected error: %v", formatPrefix(msgAndArgs), err)
	}
}

// Error fails the test if err is nil.
func Error(t testing.TB, err error, msgAndArgs ...any) {
	t.Helper()
	if err == nil {
		t.Fatalf("%sexpected an error, got nil", formatPrefix(msgAndArgs))
	}
}

// True fails the test if v is false.
func True(t testing.TB, v bool, msgAndArgs ...any) {
	t.Helper()
	if !v {
		t.Fatalf("%sexpected true, got false", formatPrefix(msgAndArgs))
	}
}

// False fails the test if v is true.
func False(t testing.TB, v bool, msgAndArgs ...any) {
	t.Helper()
	if v {
		t.Fatalf("%sexpected false, got true", formatPrefix(msgAndArgs))
	}
}

// Nil fails the test if v is not nil (including typed nil pointers, slices,
// maps, channels, functions, and interfaces).
func Nil(t testing.TB, v any, msgAndArgs ...any) {
	t.Helper()
	if !isNil(v) {
		t.Fatalf("%sexpected nil, got %#v", formatPrefix(msgAndArgs), v)
	}
}

// NotNil fails the test if v is nil (including typed nil pointers, slices,
// maps, channels, functions, and interfaces).
func NotNil(t testing.TB, v any, msgAndArgs ...any) {
	t.Helper()
	if isNil(v) {
		t.Fatalf("%sexpected non-nil value", formatPrefix(msgAndArgs))
	}
}

// Contains fails the test if s does not contain substr.
func Contains(t testing.TB, s, substr string, msgAndArgs ...any) {
	t.Helper()
	if !strings.Contains(s, substr) {
		t.Fatalf("%sexpected %q to contain %q", formatPrefix(msgAndArgs), s, substr)
	}
}

// NotContains fails the test if s contains substr.
func NotContains(t testing.TB, s, substr string, msgAndArgs ...any) {
	t.Helper()
	if strings.Contains(s, substr) {
		t.Fatalf("%sexpected %q not to contain %q", formatPrefix(msgAndArgs), s, substr)
	}
}

func isNil(v any) bool {
	if v == nil {
		return true
	}
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return rv.IsNil()
	}
	return false
}

func formatPrefix(msgAndArgs []any) string {
	switch len(msgAndArgs) {
	case 0:
		return ""
	case 1:
		return fmt.Sprintf("%v: ", msgAndArgs[0])
	default:
		if format, ok := msgAndArgs[0].(string); ok {
			return fmt.Sprintf(format, msgAndArgs[1:]...) + ": "
		}
		return fmt.Sprint(msgAndArgs...) + ": "
	}
}
