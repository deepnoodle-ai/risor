package risor_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	risor "github.com/deepnoodle-ai/risor/go/v3"
	"github.com/deepnoodle-ai/wonton/assert"
)

// A host number that cannot enter fails where an entry reads the element
// (02-evaluation.md 3.3), with the span of the whole call; an entry that only
// copies or measures the list does not read it.
func TestEntriesReadHostNumbers(t *testing.T) {
	env, _ := risor.NewEnv(risor.Declare("{xs: any, es: any}"))
	big := int64(1) << 60
	vars := risor.Map{"xs": []any{1, big}, "es": []any{map[string]any{"key": "a", "value": big}}}
	for _, src := range []string{"xs.sum()", "xs.sort()", "xs.min()", "xs.unique()", "xs.map(x => 1)",
		"xs.filter(x => true)", "xs.last()", "string(xs)", "xs.find(x => false)"} {
		d := runErr(t, env, src, vars)
		assert.Equal(t, d.Code, "number-range", src)
		assert.Equal(t, d.Class, "environment", src)
		assert.Equal(t, d.Span.Array(), [4]int{1, 1, 1, len(src) + 1}, src)
	}
	v := run(t, env, "[len(xs), len(xs.reverse()), len(xs.slice(1)), es.toMap().keys(), len([...xs])]", vars)
	assert.Equal(t, v, any([]any{2.0, 2.0, 1.0, []any{"a"}, 2.0}))
}

// timed runs a program and returns how long it took and its steps.
func timed(t *testing.T, env *risor.Env, src string, vars risor.Map) (time.Duration, int64) {
	t.Helper()
	p := compile(t, env, src)
	start := time.Now()
	_, u, err := p.Measure(context.Background(), vars)
	assert.NoError(t, err, src)
	return time.Since(start), u.Steps
}

// The entries this implementation wrote itself do work in proportion to what
// they charge (02-evaluation.md 8.5): a search, a sort and a join on large
// inputs. Each case states a bound in nanoseconds for each step charged that
// a quadratic algorithm would pass by far.
func TestWorkIsBoundedByCharge(t *testing.T) {
	env, _ := risor.NewEnv(risor.Declare("{s: string, t: string, xs: any, ys: any}"),
		risor.Limits{Steps: risor.NoLimit, Alloc: risor.NoLimit})
	n := 1 << 20
	// A search whose pattern nearly matches everywhere: aaaa...ab in aaaa...a.
	s := strings.Repeat("a", n)
	tt := strings.Repeat("a", 4096) + "b"
	strs := make([]any, 1<<15)
	for i := range strs {
		strs[i] = strings.Repeat("x", 30)
	}
	nums := make([]any, 1<<17)
	for i := range nums {
		nums[i] = float64((i * 7919) % 100003)
	}
	sorted := make([]any, 1<<17)
	for i := range sorted {
		sorted[i] = float64(i)
	}
	vars := risor.Map{"s": s, "t": tt, "xs": nums, "ys": strs}
	for _, c := range []struct {
		src  string
		vars risor.Map
	}{
		{"t in s", vars},
		{"s.split(t)", vars},
		{"s.replaceAll(t, \"x\")", vars},
		{"xs.sort()", vars},
		{"xs.sortBy(x => -x)", vars},
		{"ys.join(\",\")", vars},
		{"xs.unique()", vars},
		{"xs.sort()", risor.Map{"xs": sorted, "s": "", "t": "", "ys": []any{}}},
	} {
		d, steps := timed(t, env, c.src, c.vars)
		perStep := float64(d.Nanoseconds()) / float64(steps)
		assert.True(t, perStep < 20000, "%s: %d steps in %v (%.0f ns a step)", c.src, steps, d, perStep)
	}
}

func TestLibraryLimits(t *testing.T) {
	env, _ := risor.NewEnv(risor.Limits{Steps: 50})
	// range charges before it builds: a huge range fails at once.
	_, err := compile(t, env, "range(1000000000000)").Run(context.Background(), nil)
	var e *risor.Error
	assert.True(t, errors.As(err, &e))
	assert.Equal(t, e.Diagnostics[0].Code, "steps-exceeded")
	env, _ = risor.NewEnv(risor.Limits{Alloc: 100, Steps: risor.NoLimit})
	_, err = compile(t, env, `"ab".repeat(1000000000000)`).Run(context.Background(), nil)
	assert.True(t, errors.As(err, &e))
	assert.Equal(t, e.Diagnostics[0].Code, "allocation-exceeded")
	_, err = compile(t, env, `"a".padStart(9007199254740991, "xy")`).Run(context.Background(), nil)
	assert.True(t, errors.As(err, &e))
	assert.Equal(t, e.Diagnostics[0].Code, "allocation-exceeded")
}
