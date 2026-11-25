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
		{"a limit failure: span, trace and usage are not compared",
			`{"name":"a","phase":"run","diagnostics":[{"code":"steps-exceeded","span":null,"class":"limit","message":"m"}]}`,
			`{"name":"a","phase":"run","diagnostics":[{"code":"steps-exceeded","span":[1,1,1,2],"class":"limit","message":"m","trace":[]}],"steps":5,"units":1}`, ""},
		{"a limit failure: which limit is compared",
			`{"name":"a","phase":"run","diagnostics":[{"code":"steps-exceeded","span":null,"class":"limit","message":"m"}]}`,
			`{"name":"a","phase":"run","diagnostics":[{"code":"depth-exceeded","span":null,"class":"limit","message":"m"}]}`, "diagnostics.code"},
		{"parse: a message on one side only differs",
			`{"name":"a","phase":"parse","diagnostics":[{"code":"bad-number","span":[1,1,1,4],"message":"x"}]}`,
			`{"name":"a","phase":"parse","diagnostics":[{"code":"bad-number","span":[1,1,1,4]}]}`, "diagnostics.message"},
		{"parse: messages on both sides are compared (F8)",
			`{"name":"a","phase":"parse","diagnostics":[{"code":"bad-number","span":[1,1,1,4],"message":"x"}]}`,
			`{"name":"a","phase":"parse","diagnostics":[{"code":"bad-number","span":[1,1,1,4],"message":"y"}]}`, "diagnostics.message"},
		{"run: traces are compared (F8)",
			`{"name":"a","phase":"run","diagnostics":[{"code":"wrong-kind","span":[1,1,1,2],"class":"dynamic","message":"m","trace":[]}],"steps":0,"units":0}`,
			`{"name":"a","phase":"run","diagnostics":[{"code":"wrong-kind","span":[1,1,1,2],"class":"dynamic","message":"m","trace":[{"span":[1,1,1,2],"name":"f","index":null}]}],"steps":0,"units":0}`, "diagnostics.trace"},
		{"run: trace on one side only differs",
			`{"name":"a","phase":"run","diagnostics":[{"code":"not-conforming","span":null,"class":"value","message":"m","path":["x"]}],"steps":0,"units":0}`,
			`{"name":"a","phase":"run","diagnostics":[{"code":"not-conforming","span":null,"class":"value","message":"m","path":["x"],"trace":[]}],"steps":0,"units":0}`, "diagnostics.trace"},
		{"run: diagnostic path differs",
			`{"name":"a","phase":"run","diagnostics":[{"code":"not-conforming","span":null,"class":"value","message":"m","path":["a",0]}],"steps":0,"units":0}`,
			`{"name":"a","phase":"run","diagnostics":[{"code":"not-conforming","span":null,"class":"value","message":"m","path":["a",1]}],"steps":0,"units":0}`, "diagnostics.path"},
		{"run: diagnostic path on one side only differs",
			`{"name":"a","phase":"run","diagnostics":[{"code":"not-conforming","span":[1,1,1,5],"class":"value","message":"m","trace":[]}],"steps":0,"units":0}`,
			`{"name":"a","phase":"run","diagnostics":[{"code":"not-conforming","span":[1,1,1,5],"class":"value","message":"m","path":["x"],"trace":[]}],"steps":0,"units":0}`, "diagnostics.path"},
		{"run: extra diagnostic field differs",
			`{"name":"a","phase":"run","diagnostics":[{"code":"failed","span":[1,1,1,5],"class":"value","message":"m","trace":[],"data":{"k":1}}],"steps":0,"units":0}`,
			`{"name":"a","phase":"run","diagnostics":[{"code":"failed","span":[1,1,1,5],"class":"value","message":"m","trace":[],"data":{"k":2}}],"steps":0,"units":0}`, "diagnostics.data"},
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
		{"a description on one side only differs",
			`{"name":"a","description":{"x":1},"type":"int","sites":[],"phase":null,"value":1,"steps":0,"units":0}`,
			`{"name":"a","type":"int","sites":[],"phase":null,"value":1,"steps":0,"units":0}`, "description"},
		{"descriptions that differ",
			`{"name":"a","description":{"x":1},"type":"int","sites":[],"phase":null,"value":1,"steps":0,"units":0}`,
			`{"name":"a","description":{"x":2},"type":"int","sites":[],"phase":null,"value":1,"steps":0,"units":0}`, "description"},
		{"an analysis on one side only differs",
			`{"name":"a","phase":"parse","diagnostics":[{"code":"bad-number","span":[1,1,1,4]}],"analysis":{"k":[1]}}`,
			`{"name":"a","phase":"parse","diagnostics":[{"code":"bad-number","span":[1,1,1,4]}]}`, "analysis"},
		{"analyses that differ",
			`{"name":"a","type":"int","sites":[],"phase":null,"value":1,"steps":0,"units":0,"analysis":{"k":[1]}}`,
			`{"name":"a","type":"int","sites":[],"phase":null,"value":1,"steps":0,"units":0,"analysis":{"k":[2]}}`, "analysis"},
		{"opaque values",
			`{"name":"a","phase":null,"value":{"$opaque":{"type":"Tok","id":1}},"steps":0,"units":0}`,
			`{"name":"a","phase":null,"value":{"$opaque":{"type":"Tok","id":2}},"steps":0,"units":0}`, "value"},
		// Conform cases (F4 9.5, F8 8) including failures lists (#688, #703).
		{"conform: identical with failures",
			`{"name":"a","phase":null,"conforms":false,"path":["a"],"message":"m1","steps":4,"agree":true,"failures":[{"path":["a"],"message":"m1"},{"code":"steps-exceeded","message":"limit"}]}`,
			`{"name":"a","phase":null,"conforms":false,"path":["a"],"message":"m1","steps":4.0,"agree":true,"failures":[{"path":["a"],"message":"m1"},{"code":"steps-exceeded","message":"limit"}]}`, ""},
		{"conform: conforms flag differs",
			`{"name":"a","phase":null,"conforms":true,"steps":1,"agree":true}`,
			`{"name":"a","phase":null,"conforms":false,"steps":1,"agree":true}`, "conforms"},
		{"conform: path differs",
			`{"name":"a","phase":null,"conforms":false,"path":["a",0],"message":"m","steps":2,"agree":true}`,
			`{"name":"a","phase":null,"conforms":false,"path":["a",1],"message":"m","steps":2,"agree":true}`, "path"},
		{"conform: message differs",
			`{"name":"a","phase":null,"conforms":false,"path":["a"],"message":"m1","steps":2,"agree":true}`,
			`{"name":"a","phase":null,"conforms":false,"path":["a"],"message":"m2","steps":2,"agree":true}`, "message"},
		{"conform: agree differs",
			`{"name":"a","phase":null,"conforms":true,"steps":2,"agree":true}`,
			`{"name":"a","phase":null,"conforms":true,"steps":2,"agree":false}`, "agree"},
		{"conform: failures length differs",
			`{"name":"a","phase":null,"conforms":false,"path":["a"],"message":"m1","steps":4,"agree":true,"failures":[{"path":["a"],"message":"m1"}]}`,
			`{"name":"a","phase":null,"conforms":false,"path":["a"],"message":"m1","steps":4,"agree":true,"failures":[{"path":["a"],"message":"m1"},{"code":"steps-exceeded","message":"limit"}]}`, "failures"},
		{"conform: failures order differs",
			`{"name":"a","phase":null,"conforms":false,"path":["a"],"message":"m1","steps":4,"agree":true,"failures":[{"path":["a"],"message":"m1"},{"path":["b"],"message":"m2"}]}`,
			`{"name":"a","phase":null,"conforms":false,"path":["a"],"message":"m1","steps":4,"agree":true,"failures":[{"path":["b"],"message":"m2"},{"path":["a"],"message":"m1"}]}`, "failures"},
		{"conform: failures code vs path differs",
			`{"name":"a","phase":null,"conforms":false,"path":["a"],"message":"m1","steps":4,"agree":true,"failures":[{"code":"steps-exceeded","message":"limit"}]}`,
			`{"name":"a","phase":null,"conforms":false,"path":["a"],"message":"m1","steps":4,"agree":true,"failures":[{"path":[],"message":"limit"}]}`, "failures"},
		{"conform: failures message differs",
			`{"name":"a","phase":null,"conforms":false,"path":["a"],"message":"m1","steps":4,"agree":true,"failures":[{"path":["a"],"message":"m1"}]}`,
			`{"name":"a","phase":null,"conforms":false,"path":["a"],"message":"m1","steps":4,"agree":true,"failures":[{"path":["a"],"message":"m2"}]}`, "failures"},
		{"conform: failures absent on one side",
			`{"name":"a","phase":null,"conforms":false,"path":["a"],"message":"m1","steps":4,"agree":true,"failures":[]}`,
			`{"name":"a","phase":null,"conforms":false,"path":["a"],"message":"m1","steps":4,"agree":true}`, "failures"},
		// Render cases (F8 8), no phase field.
		{"render: identical",
			`{"name":"a","line":"l","detail":"d","json":"{}"}`,
			`{"name":"a","line":"l","detail":"d","json":"{}"}`, ""},
		{"render: detail differs",
			`{"name":"a","line":"l","detail":"d1","json":"{}"}`,
			`{"name":"a","line":"l","detail":"d2","json":"{}"}`, "detail"},
		// Runner cases (F9 9.4), no phase field.
		{"runner: verdicts differ",
			`{"name":"a","verdicts":[{"name":"c","status":"pass"}],"report":"ok"}`,
			`{"name":"a","verdicts":[{"name":"c","status":"fail","differs":["value"]}],"report":"ok"}`, "verdicts"},
		{"runner: text differs",
			`{"name":"a","verdicts":[],"report":"ok","text":"a"}`,
			`{"name":"a","verdicts":[],"report":"ok","text":"b"}`, "text"},
		{"runner: invalid differs",
			`{"name":"a","invalid":true}`,
			`{"name":"a","verdicts":[],"report":""}`, "invalid"},
		// Tools cases (F12 7.7), no phase field on success.
		{"tools: formatted differs",
			`{"name":"a","formatted":"1 + 2\n"}`,
			`{"name":"a","formatted":"1+2\n"}`, "formatted"},
		{"tools: completion differs",
			`{"name":"a","completion":{"span":[1,1,1,2],"items":[{"label":"x","kind":"variable","detail":"int","insert":"x"}]}}`,
			`{"name":"a","completion":{"span":[1,1,1,2],"items":[{"label":"y","kind":"variable","detail":"int","insert":"y"}]}}`, "completion"},
		{"tools: hover null vs object",
			`{"name":"a","hover":null}`,
			`{"name":"a","hover":{"kind":"variable","span":[1,1,1,2],"type":"int","signature":null,"doc":null,"declaration":null}}`, "hover"},
		// Catch-all for unknown extra top-level fields.
		{"unknown extra field differs",
			`{"name":"a","phase":null,"value":1,"steps":0,"units":0,"entries":[{"name":"f"}]}`,
			`{"name":"a","phase":null,"value":1,"steps":0,"units":0,"entries":[{"name":"g"}]}`, "entries"},
		{"unknown extra field on one side only",
			`{"name":"a","phase":null,"value":1,"steps":0,"units":0,"entries":[]}`,
			`{"name":"a","phase":null,"value":1,"steps":0,"units":0}`, "entries"},
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
