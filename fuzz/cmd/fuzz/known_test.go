package main

import (
	"testing"

	"github.com/deepnoodle-ai/risor/fuzz/internal/gen"
	"github.com/deepnoodle-ai/risor/fuzz/internal/outcome"
)

func row(t *testing.T, lines ...string) []res {
	t.Helper()
	var rs []res
	for _, s := range lines {
		l, err := outcome.Parse(s)
		if err != nil {
			t.Fatal(err)
		}
		rs = append(rs, res{line: l})
	}
	return rs
}

func TestKnown(t *testing.T) {
	cases := []struct {
		src   string
		lines []string
		want  string
	}{
		{"a = 1", []string{
			`{"name":"a","phase":"parse","diagnostics":[{"code":"assignment","span":[1,3,1,4]}]}`,
			`{"name":"a","phase":"parse","diagnostics":[{"code":"unexpected-token","span":[1,3,1,4]}]}`,
			`{"name":"a","phase":"parse","diagnostics":[{"code":"unexpected-token","span":[1,3,1,4]}]}`,
		}, "package-parse"},
		{"a[*", []string{
			`{"name":"a","phase":"parse","diagnostics":[{"code":"unexpected-token","span":[1,4,1,4]}]}`,
			`{"name":"a","phase":"parse","diagnostics":[{"code":"unexpected-token","span":[1,3,1,4]}]}`,
			`{"name":"a","phase":"parse","diagnostics":[{"code":"unexpected-token","span":[1,3,1,4]}]}`,
		}, "package-form"},
		{"[].sortBy(() => 0, 0)", []string{
			`{"name":"a","phase":"check","diagnostics":[{"code":"arg-mismatch","span":[1,20,1,21]}]}`,
			`{"name":"a","phase":"check","diagnostics":[{"code":"arity","span":[1,1,1,22]}]}`,
			`{"name":"a","phase":"check","diagnostics":[{"code":"arity","span":[1,1,1,22]}]}`,
		}, "package-sortBy"},
		{"1 + x", []string{
			`{"name":"a","phase":"check","diagnostics":[{"code":"kind","span":[1,5,1,6]}]}`,
			`{"name":"a","phase":"check","diagnostics":[{"code":"undefined-name","span":[1,5,1,6]}]}`,
			`{"name":"a","phase":"check","diagnostics":[{"code":"undefined-name","span":[1,5,1,6]}]}`,
		}, ""},
		{"x.y", []string{
			`{"name":"a","phase":"parse","diagnostics":[{"code":"unexpected-token","span":[1,1,1,2]}]}`,
			`{"name":"a","phase":"parse","diagnostics":[{"code":"unexpected-token","span":[1,2,1,3]}]}`,
			`{"name":"a","phase":"parse","diagnostics":[{"code":"unexpected-token","span":[1,2,1,3]}]}`,
		}, ""},
	}
	for _, c := range cases {
		if got := known(&gen.Case{Source: c.src}, row(t, c.lines...)); got != c.want {
			t.Errorf("%s: got %q, want %q", c.src, got, c.want)
		}
	}
}
