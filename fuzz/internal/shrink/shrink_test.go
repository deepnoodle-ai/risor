package shrink

import (
	"strings"
	"testing"

	"github.com/deepnoodle-ai/risor/fuzz/internal/ast"
	"github.com/deepnoodle-ai/risor/fuzz/internal/gen"
)

// A fake difference: it shows whenever the text holds the name zz.
func fake(cands []*gen.Case) []string {
	out := make([]string, len(cands))
	for i, c := range cands {
		if strings.Contains(c.Text(), "zz") {
			out[i] = "a-b=steps"
		}
	}
	return out
}

func TestShrinkTree(t *testing.T) {
	p := &ast.Program{
		Consts: []*ast.Const{
			{Pat: &ast.NamePat{Name: "a"}, Value: &ast.List{Items: []ast.Item{{X: &ast.Num{Text: "1"}}, {X: &ast.Str{Val: "long text", Quote: '"'}}}}},
			{Pat: &ast.MapPat{Fields: []ast.Field{{Name: "b"}}}, Value: &ast.Map{Entries: []ast.Entry{{Kind: ast.EntryWord, Key: "b", X: &ast.Num{Text: "2"}}}}},
		},
		Result: &ast.Binary{Op: "+", X: &ast.Ident{Name: "a"},
			Y: &ast.Call{Name: "f", Args: []ast.Expr{&ast.Num{Text: "3"}, &ast.Binary{Op: "*", X: &ast.Ident{Name: "zz"}, Y: &ast.Num{Text: "4"}}}}},
	}
	c := &gen.Case{Name: "x", Prog: p, Vars: []gen.Var{{Name: "v", Type: "any", Value: []any{1.0, "two", map[string]any{"k": true}}}}}
	got := Shrink(c, "a-b=steps", fake, Options{})
	if src := got.Text(); src != "zz" {
		t.Errorf("shrunk to %q, want %q", src, "zz")
	}
	if len(got.Vars) != 0 {
		t.Errorf("a variable was kept: %+v", got.Vars)
	}
}

func TestShrinkText(t *testing.T) {
	c := &gen.Case{Name: "x", Mode: gen.Mangled, Source: "const a = [1, 2]; a.map(x => zz + x) /* tail"}
	got := Shrink(c, "a-b=steps", fake, Options{})
	if got.Source != "zz" {
		t.Errorf("shrunk to %q", got.Source)
	}
}

func TestShrinkKeepsSignature(t *testing.T) {
	// A test that never shows the difference leaves the case as it was.
	c := &gen.Case{Name: "x", Source: "1 + 2"}
	got := Shrink(c, "a-b=steps", func(cs []*gen.Case) []string { return make([]string, len(cs)) }, Options{})
	if got.Source != "1 + 2" {
		t.Errorf("changed to %q", got.Source)
	}
}
