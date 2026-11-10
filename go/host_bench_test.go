package risor_test

// BenchmarkHostLoad stands for a host that loads a workflow: a parent
// environment with about 20 variables and the library; for each of 1,000
// expressions, a child that adds 3 variables declared from types, then a
// check and a compile of one small expression.
//
//	GOWORK=off go test -run xxx -bench HostLoad -benchmem

import (
	"fmt"
	"testing"

	risor "github.com/deepnoodle-ai/risor/v3"
)

var hostExprs = []string{
	`steps.a.status == "ok" && len(items) > 0`,
	`input.count + 1 > limit`,
	`items.filter(i => i.done).map(i => i.name)`,
	"`${input.name}: ${steps.a.status}`",
	`steps.a.output?.score ?? 0`,
}

func hostParent(b *testing.B) *risor.Env {
	decl := "{"
	for i := 0; i < 16; i++ {
		decl += fmt.Sprintf("v%d: {name: string, n: int, tags: string[]}, ", i)
	}
	decl += "input: {name: string, count: int}, limit: int, flag: bool, env: map<string>}"
	env, err := risor.NewEnv(risor.Declare(decl))
	if err != nil {
		b.Fatal(err)
	}
	return env
}

func hostTypes(b *testing.B, env *risor.Env) [3]risor.Type {
	var out [3]risor.Type
	for i, text := range []string{
		`{a: {status: string, output: {score: int} | null}}`,
		`{name: string, done: bool}[]`,
		`string`,
	} {
		t, err := env.ParseType(text)
		if err != nil {
			b.Fatal(err)
		}
		out[i] = t
	}
	return out
}

// BenchmarkHostLoad is the child, then Check, then Compile.
func BenchmarkHostLoad(b *testing.B) {
	parent := hostParent(b)
	ts := hostTypes(b, parent)
	xs := make([]*risor.Expr, len(hostExprs))
	for i, s := range hostExprs {
		x, err := risor.Parse(s)
		if err != nil {
			b.Fatal(err)
		}
		xs[i] = x
	}
	b.Run("child", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			if _, err := parent.With(risor.VarType("steps", ts[0]), risor.VarType("items", ts[1]), risor.VarType("label", ts[2])); err != nil {
				b.Fatal(err)
			}
		}
	})
	child, _ := parent.With(risor.VarType("steps", ts[0]), risor.VarType("items", ts[1]), risor.VarType("label", ts[2]))
	b.Run("check", func(b *testing.B) {
		b.ReportAllocs()
		i := 0
		for b.Loop() {
			if _, err := child.Check(xs[i%len(xs)]); err != nil {
				b.Fatal(err)
			}
			i++
		}
	})
	b.Run("compile", func(b *testing.B) {
		b.ReportAllocs()
		i := 0
		for b.Loop() {
			if _, err := child.Compile(xs[i%len(xs)]); err != nil {
				b.Fatal(err)
			}
			i++
		}
	})
	b.Run("load_one", func(b *testing.B) {
		b.ReportAllocs()
		i := 0
		for b.Loop() {
			c, err := parent.With(risor.VarType("steps", ts[0]), risor.VarType("items", ts[1]), risor.VarType("label", ts[2]))
			if err != nil {
				b.Fatal(err)
			}
			if _, _, err := c.CheckCompile(xs[i%len(xs)]); err != nil {
				b.Fatal(err)
			}
			i++
		}
	})
	b.Run("load", func(b *testing.B) {
		b.ReportAllocs()
		i := 0
		for b.Loop() {
			c, err := parent.With(risor.VarType("steps", ts[0]), risor.VarType("items", ts[1]), risor.VarType("label", ts[2]))
			if err != nil {
				b.Fatal(err)
			}
			x := xs[i%len(xs)]
			if _, err := c.Check(x); err != nil {
				b.Fatal(err)
			}
			if _, err := c.Compile(x); err != nil {
				b.Fatal(err)
			}
			i++
		}
	})
}
