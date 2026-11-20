package gen

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/deepnoodle-ai/risor/fuzz/internal/ast"
)

func loadLib(t *testing.T) *Library {
	t.Helper()
	p, err := FindLibrary(".")
	if err != nil {
		t.Fatal(err)
	}
	lib, err := LoadLibrary(p)
	if err != nil {
		t.Fatal(err)
	}
	return lib
}

func TestLibrary(t *testing.T) {
	lib := loadLib(t)
	if len(lib.Entries) != 53 || len(lib.Functions) != 15 || len(lib.Methods) != 38 {
		t.Fatalf("entries %d, functions %d, methods %d", len(lib.Entries), len(lib.Functions), len(lib.Methods))
	}
	for _, e := range lib.Entries {
		if e.Name == "reduce" {
			if len(e.Params) != 2 || e.Params[0].Fn == nil || len(e.Params[0].Fn.Params) != 3 || e.Required() != 2 {
				t.Errorf("reduce parsed as %+v", e.Params)
			}
		}
		if e.Name == "min" && e.Receiver == "" && (e.Max() != -1 || e.Required() != 2 || e.Where["T"] != "ordered") {
			t.Errorf("min parsed as %+v", e)
		}
	}
}

func TestSameSeedSameFile(t *testing.T) {
	lib := loadLib(t)
	make := func(seed uint64) []byte {
		var cs []*Case
		for i := range 300 {
			cs = append(cs, Generate(lib, Options{}, seed, 3, i))
		}
		return File(cs)
	}
	a, b := make(42), make(42)
	if !bytes.Equal(a, b) {
		t.Fatal("the same seed gave two files")
	}
	if bytes.Equal(a, make(43)) {
		t.Fatal("two seeds gave one file")
	}
	var v []map[string]any
	if err := json.Unmarshal(a, &v); err != nil || len(v) != 300 {
		t.Fatalf("not a case file: %v", err)
	}
}

func TestModesAndForms(t *testing.T) {
	lib := loadLib(t)
	modes := map[Mode]int{}
	for i := range 2000 {
		c := Generate(lib, Options{}, 9, 0, i)
		modes[c.Mode]++
		if c.Mode != Mangled && c.Prog == nil {
			t.Fatal("a typed or loose case has no tree")
		}
	}
	for _, m := range []Mode{Typed, Loose, Mangled} {
		if modes[m] < 200 {
			t.Errorf("mode %s: %d cases of 2000", m, modes[m])
		}
	}
}

// The generator writes typed recursive lambdas (F17 step E1), and -noe1
// leaves them out.
func TestSelfRecursion(t *testing.T) {
	lib := loadLib(t)
	with := 0
	for i := range 2000 {
		c := Generate(lib, Options{}, 9, 0, i)
		if c.E1 {
			with++
		}
		if d := Generate(lib, Options{NoE1: true}, 9, 0, i); d.E1 {
			t.Fatalf("case %d has a recursive lambda with NoE1", i)
		}
	}
	if with < 100 {
		t.Errorf("%d of 2000 cases have a recursive lambda", with)
	}
}

func TestNumberJSON(t *testing.T) {
	for v, want := range map[float64]string{0: "0", 1e21: "1e+21", 1e-7: "1e-7", 5e-324: "5e-324", 0.1: "0.1", 9007199254740991: "9007199254740991", 123456.789: "123456.789", 1e-6: "0.000001", 1.5e300: "1.5e+300", 100: "100"} {
		if got := formatNumber(v); got != want {
			t.Errorf("%v: got %s, want %s", v, got, want)
		}
	}
}

// The generator writes function parameters (F17 step E3), and -noe3 leaves
// them out.
func TestFunctionParameters(t *testing.T) {
	lib := loadLib(t)
	with := 0
	for i := range 2000 {
		c := Generate(lib, Options{}, 9, 0, i)
		if c.E3 {
			with++
		}
		if d := Generate(lib, Options{NoE3: true}, 9, 0, i); d.E3 {
			t.Fatalf("case %d has a function parameter with NoE3", i)
		}
	}
	if with < 100 {
		t.Errorf("%d of 2000 cases have a function parameter", with)
	}
}

// A flat chain has 1,000 links or more, and prints as text with no level of
// nesting deeper than its lambda's argument list.
func TestFlatChain(t *testing.T) {
	lib := loadLib(t)
	seen := 0
	for i := 0; i < 40; i++ {
		c := Generate(lib, Options{Typed: 1, FlatShare: 100, LimitShare: -1}, 7, 0, i)
		if c.Returns != "" {
			continue
		}
		seen++
		src := c.Source
		if c.Prog != nil {
			src = ast.Print(c.Prog)
		}
		links := strings.Count(src, " as any") + strings.Count(src, ".map(")
		if links < flatMin {
			t.Fatalf("case %d: %d links, want %d or more", i, links, flatMin)
		}
	}
	if seen == 0 {
		t.Fatal("no case had a flat chain")
	}
	c := Generate(lib, Options{Typed: 1, FlatShare: -1}, 7, 0, 0)
	if n := strings.Count(ast.Print(c.Prog), ".map("); n >= flatMin {
		t.Fatalf("a flat chain with FlatShare -1: %d calls of map", n)
	}
}
