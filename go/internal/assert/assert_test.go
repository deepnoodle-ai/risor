package assert

import (
	"errors"
	"testing"
)

func TestIsNil(t *testing.T) {
	var p *int
	var s []string
	var m map[string]int
	var f func()
	var ch chan int
	var err error
	n := 1

	for _, v := range []any{nil, p, s, m, f, ch, err} {
		if !isNil(v) {
			t.Fatalf("expected %#v to be nil", v)
		}
	}
	for _, v := range []any{0, "", false, &n, []string{}, map[string]int{}, errors.New("boom")} {
		if isNil(v) {
			t.Fatalf("expected %#v not to be nil", v)
		}
	}
}

func TestFormatPrefix(t *testing.T) {
	if got := formatPrefix(nil); got != "" {
		t.Fatalf("got %q, want empty", got)
	}
	if got := formatPrefix([]any{"10 % 3"}); got != "10 % 3: " {
		t.Fatalf("got %q, want %q", got, "10 % 3: ")
	}
	if got := formatPrefix([]any{"depth %d", 42}); got != "depth 42: " {
		t.Fatalf("got %q, want %q", got, "depth 42: ")
	}
}

func TestPassingAssertions(t *testing.T) {
	Equal(t, []int{1, 2}, []int{1, 2})
	NoError(t, nil)
	Error(t, errors.New("fail"))
	True(t, true)
	False(t, false)
	var p *int
	Nil(t, p)
	n := 1
	NotNil(t, &n)
	Contains(t, "hello world", "world")
	NotContains(t, "hello world", "mars")
}
