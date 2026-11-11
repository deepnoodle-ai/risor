package risor_test

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"

	risor "github.com/deepnoodle-ai/risor/go/v3"
	"github.com/deepnoodle-ai/risor/go/v3/internal/casefile"
	"github.com/deepnoodle-ai/risor/go/v3/internal/runner"
	"github.com/deepnoodle-ai/wonton/assert"
)

// TestReusedRunState runs each case of suite/core-run that needs no host
// function three times on one compiled program. A program keeps its run
// state between runs, so the second and third runs start from the state the
// first left, which may have failed at any point. Every run must give the
// same value, usage and failure.
func TestReusedRunState(t *testing.T) {
	needSuite(t)
	files, err := runner.Files(filepath.Join("..", "suite", "core-run"))
	assert.NoError(t, err)
	ran := 0
	for _, path := range files {
		f, err := casefile.Load(path)
		if err != nil {
			continue
		}
		for _, c := range f.Cases {
			if len(c.Functions) > 0 || len(c.Methods) > 0 || len(c.Opaque) > 0 || c.Skip != nil {
				continue
			}
			var decls []risor.Decl
			if c.Env != nil {
				decls = append(decls, risor.Declare(*c.Env))
			}
			if l := c.Limits; l != nil {
				lim := risor.Limits{}
				if l.Steps != nil {
					lim.Steps = *l.Steps
					if lim.Steps == 0 {
						lim.Steps = risor.ZeroLimit
					}
				}
				if l.Allocation != nil {
					lim.Alloc = *l.Allocation
					if lim.Alloc == 0 {
						lim.Alloc = risor.ZeroLimit
					}
				}
				if l.Depth != nil {
					lim.Depth = int(*l.Depth)
				}
				decls = append(decls, lim)
			}
			env, err := risor.NewEnv(decls...)
			if err != nil {
				continue
			}
			x, err := risor.Parse(c.Source)
			if err != nil {
				continue
			}
			p, err := env.Compile(x, risor.NoCheck)
			if err != nil {
				continue
			}
			var first string
			for i := 0; i < 3; i++ {
				v, u, err := p.Measure(context.Background(), risor.Map(c.Vars))
				got := fmt.Sprintf("%#v %v %v", v, u, err)
				if i == 0 {
					first = got
				} else {
					assert.Equal(t, got, first, c.Name)
				}
			}
			ran++
		}
	}
	assert.True(t, ran > 1000)
}
