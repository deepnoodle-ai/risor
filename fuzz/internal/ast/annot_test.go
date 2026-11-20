package ast

import "testing"

// Package F16: type lines and annotations.
func TestPrintAnnotations(t *testing.T) {
	p := &Program{
		Types: []TypeLine{{Name: "Tl0", Type: "{a: int, ...}"}},
		Consts: []*Const{
			{Pat: &NamePat{Name: "c"}, Type: "Tl0", Value: id("v")},
			{Pat: &NamePat{Name: "f"}, Value: &Lambda{Params: []Param{{Pat: &NamePat{Name: "p"}, Type: "int", Default: n("1")}}, Result: "float", Body: id("p")}},
		},
		Result: id("c"),
	}
	want := "type Tl0 = {a: int, ...}; const c: Tl0 = v; const f = (p: int = 1): float => p; c"
	if got := Print(p); got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
	if got, _ := PrintSpans(p); got != want {
		t.Fatalf("PrintSpans: got %q, want %q", got, want)
	}
	if got := Print(Clone(p)); got != want {
		t.Fatalf("Clone: got %q, want %q", got, want)
	}
}
