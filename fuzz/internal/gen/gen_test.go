package gen

import (
	"bytes"
	"encoding/json"
	"testing"
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

func TestNumberJSON(t *testing.T) {
	for v, want := range map[float64]string{0: "0", 1e21: "1e+21", 1e-7: "1e-7", 5e-324: "5e-324", 0.1: "0.1", 9007199254740991: "9007199254740991", 123456.789: "123456.789", 1e-6: "0.000001", 1.5e300: "1.5e+300", 100: "100"} {
		if got := formatNumber(v); got != want {
			t.Errorf("%v: got %s, want %s", v, got, want)
		}
	}
}
