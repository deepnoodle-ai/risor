//go:build !race

package risor_test

import (
	"context"
	"fmt"
	"testing"

	risor "github.com/deepnoodle-ai/risor/go/v3"
	"github.com/deepnoodle-ai/wonton/assert"
)

// The race detector adds allocations, so these counts hold only without it.

func TestTypedLayerAllocations(t *testing.T) {
	ty, _ := risor.TypeOf[tNode]()
	env, _ := risor.NewEnv(risor.VarType("n", ty))
	n := tnode()
	n.Items = make([]tLeaf, 100)
	for i := range n.Items {
		n.Items[i] = tLeaf{Name: fmt.Sprint(i)}
	}
	for _, c := range []struct {
		src string
		max float64
	}{
		{`n.items.some(x => x.name == "zz")`, 4},
		{`n.items.count(x => x.name == "5")`, 4},
		{`n.items.filter(x => x.name == "5")`, 6},
	} {
		x, _ := risor.Parse(c.src)
		p, err := env.Compile(x)
		assert.NoError(t, err)
		vars := risor.Map{"n": n}
		allocs := testing.AllocsPerRun(20, func() { _, _ = p.Run(context.Background(), vars) })
		assert.True(t, allocs <= c.max, fmt.Sprintf("%s: %v allocations", c.src, allocs))
	}
}
