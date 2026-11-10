package sound

import (
	"testing"

	"github.com/deepnoodle-ai/risor/fuzz/internal/ast"
	"github.com/deepnoodle-ai/risor/fuzz/internal/outcome"
)

func line(t *testing.T, s string) *outcome.Line {
	t.Helper()
	l, err := outcome.Parse(s)
	if err != nil {
		t.Fatal(err)
	}
	return l
}

func TestJudge(t *testing.T) {
	// x.a + 1, with x: any. The Binary spans 1:1-1:8; x.a spans 1:1-1:4.
	p := &ast.Program{Result: &ast.Binary{Op: "+", X: &ast.Member{X: &ast.Ident{Name: "x"}, Name: "a"}, Y: &ast.Num{Text: "1"}}}
	_, nodes := ast.PrintSpans(p)
	cases := []struct {
		name, line string
		promise    int
		violation  bool
	}{
		{"a failure at a dynamic site on the operand",
			`{"name":"a","type":"any","sites":[{"class":"dynamic","span":[1,1,1,2]},{"class":"dynamic","span":[1,1,1,4]}],"phase":"run","diagnostics":[{"code":"wrong-kind","span":[1,1,1,8],"class":"dynamic","message":"m"}],"steps":1,"units":0}`, 0, false},
		{"a failure at no site",
			`{"name":"a","type":"any","sites":[{"class":"dynamic","span":[1,1,1,2]}],"phase":"run","diagnostics":[{"code":"wrong-kind","span":[1,1,1,8],"class":"environment","message":"m"}],"steps":1,"units":0}`, 1, true},
		{"an index failure at a dynamic site is not at an index site",
			`{"name":"a","type":"any","sites":[{"class":"dynamic","span":[1,1,1,4]}],"phase":"run","diagnostics":[{"code":"index-range","span":[1,1,1,8],"class":"index","message":"m"}],"steps":1,"units":0}`, 2, true},
		{"a range failure anywhere",
			`{"name":"a","type":"int","sites":[],"phase":"run","diagnostics":[{"code":"number-range","span":[1,1,1,8],"class":"range","message":"m"}],"steps":1,"units":0}`, 0, false},
		{"a value that conforms",
			`{"name":"a","type":"int | null","sites":[],"phase":null,"value":null,"steps":0,"units":0}`, 0, false},
		{"a value that does not conform",
			`{"name":"a","type":"int","sites":[],"phase":null,"value":1.5,"steps":0,"units":0}`, 4, true},
		{"a class that disagrees with the site",
			`{"name":"a","type":"any","sites":[{"class":"dynamic","span":[1,1,1,4]}],"phase":"run","diagnostics":[{"code":"null-operand","span":[1,1,1,8],"class":"environment","message":"m"}],"steps":1,"units":0}`, 0, true},
	}
	for _, c := range cases {
		v := Judge(line(t, c.line), nodes)
		if !v.Clean || (v.Violation != "") != c.violation || v.Promise != c.promise || v.Undecided != "" {
			t.Errorf("%s: got %+v", c.name, v)
		}
	}
	if v := Judge(line(t, `{"name":"a","phase":"check","diagnostics":[{"code":"kind","span":[1,1,1,2]}]}`), nodes); v.Clean {
		t.Error("a check failure counted as clean")
	}
}

// 05-types.md 18.3: a failed test of a callback's result is at a site on
// the body of the lambda argument.
func TestJudgeCallbackBody(t *testing.T) {
	p := &ast.Program{Result: &ast.MethodCall{X: &ast.List{Items: []ast.Item{{X: &ast.Num{Text: "0"}}}}, Name: "flatMap",
		Args: []ast.Expr{&ast.Lambda{Body: &ast.Ident{Name: "x"}}}}}
	src, nodes := ast.PrintSpans(p)
	if src != "[0].flatMap(() => x)" {
		t.Fatalf("got %q", src)
	}
	l := line(t, `{"name":"a","type":"any[]","sites":[{"class":"dynamic","span":[1,19,1,20]}],"phase":"run","diagnostics":[{"code":"wrong-kind","span":[1,1,1,21],"class":"dynamic","message":"m"}],"steps":4,"units":2}`)
	if v := Judge(l, nodes); v.Violation != "" || v.Undecided != "" {
		t.Errorf("got %+v", v)
	}
}

// 12.3: the value bound is the const's value expression, inside any
// parentheses.
func TestJudgePatternValueInParens(t *testing.T) {
	p := &ast.Program{
		Consts: []*ast.Const{{Pat: &ast.MapPat{Fields: []ast.Field{{Name: "a"}}}, Value: &ast.Paren{X: &ast.Ident{Name: "xs"}}}},
		Result: &ast.Ident{Name: "a"},
	}
	src, nodes := ast.PrintSpans(p)
	if src != "const {a} = (xs); a" {
		t.Fatalf("got %q", src)
	}
	l := line(t, `{"name":"a","type":"any","sites":[{"class":"dynamic","span":[1,14,1,16]}],"phase":"run","diagnostics":[{"code":"wrong-kind","span":[1,7,1,10],"class":"dynamic","message":"m"}],"steps":0,"units":0}`)
	if v := Judge(l, nodes); v.Violation != "" || v.Undecided != "" {
		t.Errorf("got %+v", v)
	}
}
