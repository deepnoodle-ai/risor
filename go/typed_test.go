package risor_test

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"sync"
	"testing"

	risor "github.com/deepnoodle-ai/risor/go/v3"
	"github.com/deepnoodle-ai/risor/go/v3/internal/bridge"
	"github.com/deepnoodle-ai/wonton/assert"
)

// Package F11: the typed layer changes no result. Each test runs one
// program with the layer on and off and compares the whole outcome.

type tLeaf struct {
	Name  string  `json:"name"`
	Big   int64   `json:"big"`
	F     float64 `json:"f"`
	Small int8    `json:"small"`
	Tag   tText   `json:"tag"`
	Opt   string  `json:"opt,omitempty"`
}

type tText struct{ s string }

func (t tText) MarshalText() ([]byte, error) { return []byte("text:" + t.s), nil }

type tEmbedded struct {
	Z int `json:"z"`
}

type tNode struct {
	*tEmbedded
	Leaf  tLeaf    `json:"leaf"`
	Ptr   *tLeaf   `json:"ptr"`
	Items []tLeaf  `json:"items"`
	Ptrs  []*tLeaf `json:"ptrs"`
}

// typedPlans counts the reads the layer planned in these tests.
var typedPlans int

type outcome struct {
	v            string
	err          string
	steps, units int64
}

func runBoth(t *testing.T, src string, data any, limits ...risor.Limits) (outcome, outcome) {
	t.Helper()
	ty, err := risor.TypeOf[tNode]()
	assert.NoError(t, err)
	var out [2]outcome
	for i, on := range []bool{true, false} {
		decls := []risor.Decl{risor.VarType("n", ty), risor.TypedLayer(on)}
		for _, l := range limits {
			decls = append(decls, l)
		}
		env, err := risor.NewEnv(decls...)
		assert.NoError(t, err)
		x, err := risor.Parse(src)
		assert.NoError(t, err)
		p, err := env.Compile(x)
		assert.NoError(t, err, src)
		if on {
			typedPlans += bridge.TypedPlans(p)
		} else {
			assert.Equal(t, bridge.TypedPlans(p), 0)
		}
		v, u, err := p.Measure(context.Background(), risor.Map{"n": data})
		o := outcome{steps: u.Steps, units: u.Allocated}
		if err != nil {
			var e *risor.Error
			assert.True(t, errors.As(err, &e))
			d := e.Diagnostics[0]
			o.err = fmt.Sprintf("%s %s %s %v %s", d.Phase, d.Code, d.Class, d.Span, d.Message)
			if d.Code == "steps-exceeded" {
				o.steps = 0
			}
		} else {
			o.v = fmt.Sprintf("%T %#v", v, v)
		}
		out[i] = o
	}
	return out[0], out[1]
}

func sameBoth(t *testing.T, src string, data any, limits ...risor.Limits) outcome {
	t.Helper()
	on, off := runBoth(t, src, data, limits...)
	assert.Equal(t, on, off, src)
	return on
}

func tnode() *tNode {
	return &tNode{
		tEmbedded: &tEmbedded{Z: 7},
		Leaf:      tLeaf{Name: "a", Big: 1 << 60, F: math.Copysign(0, -1), Small: -3, Tag: tText{"x"}},
		Ptr:       &tLeaf{Name: "p"},
		Items:     []tLeaf{{Name: "i0"}, {Name: "i1", Opt: "o"}, {Name: "caf\xe9"}},
		Ptrs:      []*tLeaf{{Name: "q0"}, nil, {Name: "q2"}},
	}
}

func TestTypedLayerSameOutcomes(t *testing.T) {
	for _, src := range []string{
		`n.leaf.name`,
		`n.leaf.small`,
		`n.leaf.big`,            // number-range at the read
		`n.leaf.f`,              // -0
		`n.leaf.tag`,            // a text type
		`n.leaf?.opt ?? "none"`, // omitempty and empty: absent
		`n.leaf`,                // a struct leaf
		`n.ptr?.name ?? "none"`,
		`n.items.map(x => x.name)`,
		`n.items.filter(x => x?.opt == "o").map(x => x.name)`,
		`n.items.map(x => x?.opt)`, // an absent field in a lambda
		`n.items[*].name`,
		`n.items[*]?.opt`,
		`n.ptrs[*]?.name`, // a nil element
		`n.ptrs.map(p => p?.name)`,
		`n.items.reduce((a, x) => a + x.name, "")`,
		// Elements by reference, and the string comparison in place.
		`n.items.some(x => x.name == "i1")`,
		`n.items.every(x => x.name != "zz")`,
		`n.items.count(x => x.name == "i0")`,
		`n.items.find(x => x.name == "i1")`,
		`n.items.find(x => x.name == "none")`,
		`n.items.filter(x => x.name == "i1")`,
		`n.items.filter(x => x.name == "café")`, // invalid text: the general path
		`n.items.filter(x => x.small == 0).map(x => x.name)`,
		`n.items.flatMap(x => [x.name, x.small])`,
		`n.items.map(x => x)`,           // the element escapes
		`n.items.map(x => [x.name, x])`, // a whole use beside a read
		`n.items.sortBy(x => x.name, "desc")`,
		`n.ptrs.filter(p => p?.name == "q2")`, // a nil element
		`n.ptrs.map(p => p?.name ?? "nil")`,
		`n.ptrs.count(p => p?.name != "q0")`,
		`n.items.map(x => n.items.count(y => y.name == x.name))`,
		`n.leaf.name == "a"`,
		`n.leaf.name != "b"`,
	} {
		before := typedPlans
		sameBoth(t, src, tnode())
		assert.True(t, typedPlans > before, "no read planned: "+src)
	}
}

type tOther struct {
	Name string `json:"name"`
}

func TestTypedLayerOtherSlices(t *testing.T) {
	// A host slice whose elements are of another Go type than declared.
	for _, items := range []any{[]tOther{{"i1"}, {"x"}}, []any{tLeaf{Name: "i1"}, map[string]any{"name": "i1"}}} {
		data := map[string]any{"items": items}
		for _, src := range []string{`n.items.some(x => x.name == "i1")`, `n.items.filter(x => x.name == "i1")`,
			`n.items.map(x => x.name)`, `n.items[*].name`, `n.items.reduce((a, x) => a + x.name, "")`} {
			sameBoth(t, src, data)
		}
	}
}

func TestTypedLayerOtherValues(t *testing.T) {
	// The host breaks the declared Go type: a map, a struct of another
	// type, a nil pointer, a nil embedded pointer.
	m := map[string]any{"leaf": map[string]any{"name": "m"}, "items": []any{map[string]any{"name": "x"}}}
	for _, data := range []any{m, struct{ Leaf tLeaf }{}, (*tNode)(nil), &tNode{}} {
		for _, src := range []string{`n.leaf.name`, `n.items.map(x => x.name)`, `n.items[*].name`, `n.ptr?.name`, `n.leaf?.opt`} {
			sameBoth(t, src, data)
		}
	}
}

func TestTypedLayerLimits(t *testing.T) {
	for steps := int64(1); steps < 12; steps++ {
		o := sameBoth(t, `n.items.map(x => x.name)`, tnode(), risor.Limits{Steps: steps})
		_ = o
		sameBoth(t, `n.leaf.name + (n.ptr?.name ?? "")`, tnode(), risor.Limits{Steps: steps})
	}
}

func TestTypedLayerConcurrent(t *testing.T) {
	ty, _ := risor.TypeOf[tNode]()
	env, _ := risor.NewEnv(risor.VarType("n", ty))
	x, _ := risor.Parse(`n.items.filter(x => x.name != "i0").map(x => x.name)`)
	p, err := env.Compile(x)
	assert.NoError(t, err)
	var wg sync.WaitGroup
	for g := 0; g < 8; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			for i := 0; i < 200; i++ {
				var data any = tnode()
				if (g+i)%3 == 0 {
					data = map[string]any{"items": []any{map[string]any{"name": "m"}}}
				}
				v, err := p.Run(context.Background(), risor.Map{"n": data})
				assert.NoError(t, err)
				assert.True(t, len(v.([]any)) >= 1)
			}
		}(g)
	}
	wg.Wait()
}

func TestTypedLayerOnlyWhenChecked(t *testing.T) {
	// Unchecked, any, type text: nothing changes, and every outcome is the
	// general path's.
	ty, _ := risor.TypeOf[tNode]()
	for _, decl := range []risor.Decl{risor.VarType("n", ty), risor.Var("n", "any")} {
		env, _ := risor.NewEnv(decl)
		x, _ := risor.Parse(`n.leaf.name`)
		p, err := env.Compile(x, risor.NoCheck)
		assert.NoError(t, err)
		v, err := p.Run(context.Background(), risor.Map{"n": tnode()})
		assert.NoError(t, err)
		assert.Equal(t, v, any("a"))
	}
	_ = strings.TrimSpace
}
