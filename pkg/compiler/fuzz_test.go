package compiler

import (
	"context"
	"testing"
	"time"

	"github.com/deepnoodle-ai/risor/v2/pkg/parser"
)

// FuzzParseCompile checks that any input the parser accepts compiles to
// either bytecode or an error, never a panic.
func FuzzParseCompile(f *testing.F) {
	seeds := []string{
		// Misplaced spread and default values
		"let o = { add: function(a, b) { return a + b } }\no.add(...[1, 2])",
		"...x",
		"let a = ...x",
		"1 + ...x",
		"[......arr]",
		"[a = 1]",
		"{a = 1}",
		"([a = 1])",
		// Valid spread and destructuring
		"let xs = [1]; [...xs, 2]",
		"let m = {a: 1}; {...m, b: 2}",
		"let f = function(...xs) { return xs }; f(1, ...[2, 3])",
		"let o = {}; o?.f(...[1])",
		"let f = ([a = 1], {b = 2}) => a + b; f([], {})",
		"let { a, b = 2 } = {a: 1}",
		"let [x, y = 3] = [1]",
		// General syntax
		"function f(a, b = 10) { return a + b }; f(1)",
		"let x = 1; x |> string",
		"try { throw 1 } catch e { e } finally { 2 }",
		"match 1 { 1 => \"one\", _ => \"other\" }",
		"for (let i = 0; i < 3; i++) { if (i == 1) { continue } }",
		"let s = `a ${1 + 2} b`",
		"switch (1) { case 1: 1 default: 2 }",
		"let c = 0; c += 1; c *= 2",
		"let a = [1, 2, 3]; a[0:2]",
	}
	for _, seed := range seeds {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, input string) {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		prog, err := parser.Parse(ctx, input, nil)
		if err != nil {
			return
		}
		Compile(prog, &Config{GlobalNames: []string{"x", "arr", "string"}})
	})
}
