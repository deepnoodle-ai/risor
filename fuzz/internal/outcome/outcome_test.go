package outcome

import "testing"

func mustParse(t *testing.T, s string) *Line {
	t.Helper()
	l, err := Parse(s)
	if err != nil {
		t.Fatalf("%s: %v", s, err)
	}
	return l
}

func TestDiff(t *testing.T) {
	cases := []struct {
		name, a, b, want string
	}{
		{"numbers spelled apart",
			`{"name":"a","phase":null,"value":[1,{"x":0.5}],"steps":2,"units":3}`,
			`{"name":"a","phase":null,"value":[1.0,{"x":5e-1}],"steps":2.0,"units":3}`, ""},
		{"key order does not matter",
			`{"name":"a","phase":null,"value":{"a":1,"b":2},"steps":0,"units":0}`,
			`{"name":"a","phase":null,"value":{"b":2,"a":1},"steps":0,"units":0}`, ""},
		{"steps differ",
			`{"name":"a","phase":null,"value":7,"steps":2,"units":0}`,
			`{"name":"a","phase":null,"value":7,"steps":3,"units":0}`, "steps"},
		{"units differ",
			`{"name":"a","phase":null,"value":7,"steps":2,"units":0}`,
			`{"name":"a","phase":null,"value":7,"steps":2,"units":1}`, "units"},
		{"a missing count",
			`{"name":"a","phase":null,"value":7,"steps":2,"units":0}`,
			`{"name":"a","phase":null,"value":7,"units":0}`, "steps"},
		{"values differ",
			`{"name":"a","phase":null,"value":"x","steps":0,"units":0}`,
			`{"name":"a","phase":null,"value":"y","steps":0,"units":0}`, "value"},
		{"phases differ",
			`{"name":"a","phase":null,"value":1,"steps":0,"units":0}`,
			`{"name":"a","phase":"run","diagnostics":[{"code":"wrong-kind","span":[1,1,1,2],"class":"dynamic","message":"m"}],"steps":0,"units":0}`, "phase"},
		{"run spans differ",
			`{"name":"a","phase":"run","diagnostics":[{"code":"wrong-kind","span":[1,1,1,2],"class":"dynamic","message":"m"}],"steps":0,"units":0}`,
			`{"name":"a","phase":"run","diagnostics":[{"code":"wrong-kind","span":[1,1,1,3],"class":"dynamic","message":"m"}],"steps":0,"units":0}`, "diagnostics.span"},
		{"run messages differ",
			`{"name":"a","phase":"run","diagnostics":[{"code":"wrong-kind","span":[1,1,1,2],"class":"dynamic","message":"m"}],"steps":0,"units":0}`,
			`{"name":"a","phase":"run","diagnostics":[{"code":"wrong-kind","span":[1,1,1,2],"class":"dynamic","message":"n"}],"steps":0,"units":0}`, "diagnostics.message"},
		{"a limit failure: span and usage are not compared",
			`{"name":"a","phase":"run","diagnostics":[{"code":"steps-exceeded","span":null,"class":"limit","message":"m"}]}`,
			`{"name":"a","phase":"run","diagnostics":[{"code":"steps-exceeded","span":[1,1,1,2],"class":"limit","message":"m"}],"steps":5,"units":1}`, ""},
		{"a limit failure: which limit is compared",
			`{"name":"a","phase":"run","diagnostics":[{"code":"steps-exceeded","span":null,"class":"limit","message":"m"}]}`,
			`{"name":"a","phase":"run","diagnostics":[{"code":"depth-exceeded","span":null,"class":"limit","message":"m"}]}`, "diagnostics.code"},
		{"parse: message is not compared",
			`{"name":"a","phase":"parse","diagnostics":[{"code":"bad-number","span":[1,1,1,4],"message":"x"}]}`,
			`{"name":"a","phase":"parse","diagnostics":[{"code":"bad-number","span":[1,1,1,4]}]}`, ""},
		{"check: a missing diagnostic",
			`{"name":"a","phase":"check","diagnostics":[{"code":"arity","span":[1,1,1,4]},{"code":"arity","span":[1,6,1,9]}]}`,
			`{"name":"a","phase":"check","diagnostics":[{"code":"arity","span":[1,1,1,4]}]}`, "diagnostics.count"},
		{"types differ",
			`{"name":"a","type":"int","sites":[],"phase":null,"value":1,"steps":0,"units":0}`,
			`{"name":"a","type":"float","sites":[],"phase":null,"value":1,"steps":0,"units":0}`, "type"},
		{"sites differ in order",
			`{"name":"a","type":"any","sites":[{"class":"dynamic","span":[1,1,1,2]},{"class":"index","span":[1,1,1,5]}],"phase":null,"value":1,"steps":0,"units":0}`,
			`{"name":"a","type":"any","sites":[{"class":"index","span":[1,1,1,5]},{"class":"dynamic","span":[1,1,1,2]}],"phase":null,"value":1,"steps":0,"units":0}`, "sites"},
		{"a type on one side only",
			`{"name":"a","type":"int","sites":[],"phase":null,"value":1,"steps":0,"units":0}`,
			`{"name":"a","phase":null,"value":1,"steps":0,"units":0}`, "type"},
		{"a description on one side only is ignored",
			`{"name":"a","description":{"x":1},"type":"int","sites":[],"phase":null,"value":1,"steps":0,"units":0}`,
			`{"name":"a","type":"int","sites":[],"phase":null,"value":1,"steps":0,"units":0}`, ""},
		{"descriptions that differ",
			`{"name":"a","description":{"x":1},"type":"int","sites":[],"phase":null,"value":1,"steps":0,"units":0}`,
			`{"name":"a","description":{"x":2},"type":"int","sites":[],"phase":null,"value":1,"steps":0,"units":0}`, "description"},
		{"an analysis on one side only is ignored",
			`{"name":"a","phase":"parse","diagnostics":[{"code":"bad-number","span":[1,1,1,4]}],"analysis":{"k":[1]}}`,
			`{"name":"a","phase":"parse","diagnostics":[{"code":"bad-number","span":[1,1,1,4]}]}`, ""},
		{"analyses that differ",
			`{"name":"a","type":"int","sites":[],"phase":null,"value":1,"steps":0,"units":0,"analysis":{"k":[1]}}`,
			`{"name":"a","type":"int","sites":[],"phase":null,"value":1,"steps":0,"units":0,"analysis":{"k":[2]}}`, "analysis"},
		{"opaque values",
			`{"name":"a","phase":null,"value":{"$opaque":{"type":"Tok","id":1}},"steps":0,"units":0}`,
			`{"name":"a","phase":null,"value":{"$opaque":{"type":"Tok","id":2}},"steps":0,"units":0}`, "value"},
	}
	for _, c := range cases {
		if got := Diff(mustParse(t, c.a), mustParse(t, c.b)); got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
}

func TestParseAllMissingLine(t *testing.T) {
	out := []byte("{\"name\":\"a\",\"phase\":null,\"value\":1,\"steps\":0,\"units\":0}\n\n")
	lines, err := ParseAll(out)
	if err != nil || len(lines) != 1 {
		t.Fatalf("got %d lines, %v", len(lines), err)
	}
	if _, err := ParseAll([]byte("{\"name\":\"a\"}\nnot json\n")); err == nil {
		t.Fatal("a line that is not JSON was accepted")
	}
	if _, err := Parse(`{"name":"a","value":1}`); err == nil {
		t.Fatal("a line with no phase was accepted")
	}
}

func TestMissingEntry(t *testing.T) {
	lib := map[string]bool{"sortBy": true, "trim": true}
	l := mustParse(t, `{"name":"a","phase":"check","diagnostics":[{"code":"undefined-function","span":[1,5,1,11]}]}`)
	if got := l.MissingEntry("1 + sortBy(x)", lib); got != "sortBy" {
		t.Errorf("got %q", got)
	}
	l = mustParse(t, `{"name":"a","phase":"run","diagnostics":[{"code":"wrong-kind","span":[1,1,1,9],"class":"dynamic","message":"a value of kind string has no method trim"}],"steps":0,"units":0}`)
	if got := l.MissingEntry(`"a".trim()`, lib); got != "trim" {
		t.Errorf("got %q", got)
	}
}
