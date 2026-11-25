package risor_test

import (
	"context"
	"errors"
	"math"
	"testing"

	risor "github.com/deepnoodle-ai/risor/go/v3"
	"github.com/deepnoodle-ai/wonton/assert"
)

// Call.Remaining gives each limit less what the run has charged, with the
// call's own step and the function's charges (03-host.md 3.5.2 item 3).
func TestRemainingGivesWhatIsLeft(t *testing.T) {
	var got [][2]int64
	probe := risor.Function{Name: "probe", Type: "() => int", Impl: func(c *risor.Call, args []any) (any, error) {
		s, u := c.Remaining()
		got = append(got, [2]int64{s, u})
		if err := c.Charge(5); err != nil {
			return nil, err
		}
		if err := c.Allocate(3); err != nil {
			return nil, err
		}
		s, u = c.Remaining()
		got = append(got, [2]int64{s, u})
		return 0, nil
	}}
	env, err := risor.NewEnv(risor.Limits{Steps: 100, Allocation: 50}, probe)
	assert.NoError(t, err)

	// The call's one step is charged before the function runs.
	d, u := measure(t, env, "probe()")
	assert.Nil(t, d)
	assert.Equal(t, got, [][2]int64{{99, 50}, {94, 47}})
	assert.Equal(t, u, risor.Usage{Steps: 6, Allocated: 3})

	// Literals cost no step; the list's 4 units are charged after its
	// elements, so after the call.
	got = nil
	d, u = measure(t, env, "[1, 2, probe()]")
	assert.Nil(t, d)
	assert.Equal(t, got, [][2]int64{{99, 50}, {94, 47}})
	assert.Equal(t, u, risor.Usage{Steps: 6, Allocated: 7})

	// A limit that is off gives NoLimit.
	off, err := env.With(risor.Limits{Steps: risor.NoLimit, Allocation: risor.NoLimit})
	assert.NoError(t, err)
	got = nil
	d, _ = measure(t, off, "probe()")
	assert.Nil(t, d)
	assert.Equal(t, got, [][2]int64{{risor.NoLimit, risor.NoLimit}, {risor.NoLimit, risor.NoLimit}})
}

// A charge that failed charged nothing, so Remaining does not change; a use
// of the handle after the function returned gives 0 and 0 (item 6).
func TestRemainingAfterFailureAndEnd(t *testing.T) {
	var kept *risor.Call
	var got [][2]int64
	env, err := risor.NewEnv(
		risor.Limits{Steps: 10, Allocation: 10},
		risor.Function{Name: "f", Type: "() => int", Impl: func(c *risor.Call, args []any) (any, error) {
			kept = c
			err := c.Charge(20)
			s, u := c.Remaining()
			got = append(got, [2]int64{s, u})
			return nil, err
		}},
	)
	assert.NoError(t, err)
	d, _ := measure(t, env, "f()")
	assert.Equal(t, d.Code, "steps-exceeded")
	assert.Equal(t, got, [][2]int64{{9, 10}})
	s, u := kept.Remaining()
	assert.Equal(t, [2]int64{s, u}, [2]int64{0, 0})
}

// Budget takes what Remaining gives: 0 is a limit of zero, NoLimit is off.
func TestBudget(t *testing.T) {
	env, err := risor.NewEnv(risor.Var("xs", "int[]"))
	assert.NoError(t, err)
	vars := risor.Map{"xs": []any{1, 2, 3}}
	zero, err := env.With(risor.Budget(0, risor.NoLimit))
	assert.NoError(t, err)
	_, _, err = compile(t, zero, "xs[0]").Measure(context.Background(), vars)
	assert.Equal(t, limitCode(err), "steps-exceeded")
	zero, err = env.With(risor.Budget(risor.NoLimit, 0))
	assert.NoError(t, err)
	_, _, err = compile(t, zero, "xs.map(x => x)").Measure(context.Background(), vars)
	assert.Equal(t, limitCode(err), "allocation-exceeded")
	_, err = env.With(risor.Budget(-2, 0))
	assert.Error(t, err)
}

// limitCode is the code of a run failure, or "".
func limitCode(err error) string {
	var e *risor.Error
	if errors.As(err, &e) && len(e.Diagnostics) > 0 {
		return e.Diagnostics[0].Code
	}
	return ""
}

// The inner program a tool runs, and its variables.
const innerSrc = "xs.map(x => x * 2)"

var innerVars = risor.Map{"xs": []any{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}}

// nestedTool runs the inner program with what the caller has left, then
// charges back what it used: the pattern of 03-host.md 3.2.5.
func nestedTool(t *testing.T, calls *int) risor.Function {
	t.Helper()
	toolEnv, err := risor.NewEnv(risor.Var("xs", "int[]"))
	assert.NoError(t, err)
	inner, err := risor.Parse(innerSrc)
	assert.NoError(t, err)
	return risor.Function{Name: "tool", Type: "() => int !", Impl: func(c *risor.Call, args []any) (any, error) {
		*calls++
		steps, units := c.Remaining()
		child, err := toolEnv.With(risor.Budget(steps, units))
		if err != nil {
			return nil, err
		}
		p, err := child.Compile(inner)
		if err != nil {
			return nil, err
		}
		_, used, runErr := p.Measure(c.Context(), innerVars)
		if err := c.Charge(used.Steps); err != nil {
			return nil, err
		}
		if err := c.Allocate(used.Allocated); err != nil {
			return nil, err
		}
		// A nested run that stopped at a limit stops the caller with the
		// same code: more than is left cannot be charged.
		switch limitCode(runErr) {
		case "steps-exceeded":
			return nil, c.Charge(math.MaxInt64)
		case "allocation-exceeded":
			return nil, c.Allocate(math.MaxInt64)
		}
		if runErr != nil {
			return nil, runErr
		}
		return 1, nil
	}}
}

// inlineTool charges what the inner program costs, as if its work were done
// in the caller.
func inlineTool(t *testing.T, calls *int) risor.Function {
	t.Helper()
	env, err := risor.NewEnv(risor.Var("xs", "int[]"))
	assert.NoError(t, err)
	_, cost, err := compile(t, env, innerSrc).Measure(context.Background(), innerVars)
	assert.NoError(t, err)
	return risor.Function{Name: "tool", Type: "() => int !", Impl: func(c *risor.Call, args []any) (any, error) {
		*calls++
		if err := c.Charge(cost.Steps); err != nil {
			return nil, err
		}
		if err := c.Allocate(cost.Allocated); err != nil {
			return nil, err
		}
		return 1, nil
	}}
}

// A nested run gets exactly what is left; after the charge back the
// caller's counts are exact; and the caller fails with the limit code at the
// same call as when the work is charged inline.
func TestNestedRunWithRemainingBudget(t *testing.T) {
	toolEnv, err := risor.NewEnv(risor.Var("xs", "int[]"))
	assert.NoError(t, err)
	_, cost, err := compile(t, toolEnv, innerSrc).Measure(context.Background(), innerVars)
	assert.NoError(t, err)
	assert.Equal(t, cost, risor.Usage{Steps: 32, Allocated: 11})

	const src = "[tool(), tool(), tool()]"
	outcome := func(fn func(*testing.T, *int) risor.Function, limits risor.Limits, src string) (string, risor.Usage, int) {
		calls := 0
		env, err := risor.NewEnv(limits, fn(t, &calls))
		assert.NoError(t, err)
		d, u := measure(t, env, src)
		code := ""
		if d != nil {
			code = d.Code
		}
		return code, u, calls
	}

	// Unlimited, the caller's usage is its own plus three inner runs.
	off := risor.Limits{Steps: risor.NoLimit, Allocation: risor.NoLimit}
	code, nested, n := outcome(nestedTool, off, src)
	assert.Equal(t, code, "")
	assert.Equal(t, n, 3)
	code, inline, _ := outcome(inlineTool, off, src)
	assert.Equal(t, code, "")
	assert.Equal(t, nested, inline)
	assert.Equal(t, nested, risor.Usage{Steps: 3*cost.Steps + 3, Allocated: 3*cost.Allocated + 4})

	// The one call of `tool()` costs 1 step: a limit of 1 + cost leaves the
	// nested run exactly its cost, and it succeeds; one less, and it stops.
	code, u, _ := outcome(nestedTool, risor.Limits{Steps: 1 + cost.Steps}, "tool()")
	assert.Equal(t, code, "")
	assert.Equal(t, u, risor.Usage{Steps: 1 + cost.Steps, Allocated: cost.Allocated})
	code, _, _ = outcome(nestedTool, risor.Limits{Steps: cost.Steps}, "tool()")
	assert.Equal(t, code, "steps-exceeded")
	code, u, _ = outcome(nestedTool, risor.Limits{Allocation: cost.Allocated}, "tool()")
	assert.Equal(t, code, "")
	assert.Equal(t, u.Allocated, cost.Allocated)
	code, _, _ = outcome(nestedTool, risor.Limits{Allocation: cost.Allocated - 1}, "tool()")
	assert.Equal(t, code, "allocation-exceeded")

	// At every step limit, the nested and the inline tool stop at the same
	// call with the same code, and a run that succeeds has the same usage.
	// `catch` catches neither.
	for _, s := range []string{src, "try { [tool(), tool(), tool()] } catch { [] }"} {
		for limit := int64(0); limit <= nested.Steps; limit++ {
			lim := risor.Limits{Steps: limit}
			if limit == 0 {
				lim.Steps = risor.ZeroLimit
			}
			c1, u1, n1 := outcome(nestedTool, lim, s)
			c2, u2, n2 := outcome(inlineTool, lim, s)
			assert.Equal(t, c1, c2, "limit %d", limit)
			assert.Equal(t, n1, n2, "limit %d", limit)
			if c1 == "" {
				assert.Equal(t, u1, u2, "limit %d", limit)
			}
		}
		for limit := int64(0); limit <= nested.Allocated; limit++ {
			lim := risor.Limits{Allocation: limit}
			if limit == 0 {
				lim.Allocation = risor.ZeroLimit
			}
			c1, u1, n1 := outcome(nestedTool, lim, s)
			c2, u2, n2 := outcome(inlineTool, lim, s)
			assert.Equal(t, c1, c2, "units %d", limit)
			assert.Equal(t, n1, n2, "units %d", limit)
			if c1 == "" {
				assert.Equal(t, u1, u2, "units %d", limit)
			}
		}
	}
}
