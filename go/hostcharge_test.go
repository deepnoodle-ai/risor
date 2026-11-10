package risor_test

import (
	"context"
	"errors"
	"math"
	"testing"

	risor "github.com/deepnoodle-ai/risor/v3"
	"github.com/deepnoodle-ai/wonton/assert"
)

// measure runs src and gives its first diagnostic, or nil, and its usage.
func measure(t *testing.T, env *risor.Env, src string) (*risor.Diagnostic, risor.Usage) {
	t.Helper()
	_, u, err := compile(t, env, src).Measure(context.Background(), nil)
	if err == nil {
		return nil, u
	}
	var e *risor.Error
	assert.True(t, errors.As(err, &e), "%s: %v", src, err)
	return &e.Diagnostics[0], u
}

// A charge too large for the room left fails; it does not wrap the count
// past the limit (03-host.md 3.5.2 item 3).
func TestHostChargeDoesNotWrap(t *testing.T) {
	var errs []error
	env, err := risor.NewEnv(
		risor.Function{Name: "alloc", Type: "() => int", Impl: func(c *risor.Call, args []any) (any, error) {
			errs = append(errs, c.Allocate(1), c.Allocate(math.MaxInt64))
			return 0.0, nil
		}},
		risor.Function{Name: "charge", Type: "() => int", Impl: func(c *risor.Call, args []any) (any, error) {
			errs = append(errs, c.Charge(1), c.Charge(math.MaxInt64))
			return 0.0, nil
		}},
	)
	assert.NoError(t, err)

	d, u := measure(t, env, "alloc()")
	assert.NoError(t, errs[0])
	assert.Error(t, errs[1])
	assert.Equal(t, errs[1].Error(), "allocation-exceeded")
	assert.Equal(t, d.Code, "allocation-exceeded")
	assert.Equal(t, u.Alloc, int64(1))

	errs = nil
	d, u = measure(t, env, "charge()")
	assert.NoError(t, errs[0])
	assert.Equal(t, errs[1].Error(), "steps-exceeded")
	assert.Equal(t, d.Code, "steps-exceeded")
	assert.Equal(t, u.Steps, int64(2))
}

// With no limit, a count stops at math.MaxInt64 rather than wrap negative.
func TestHostChargeNoLimitSaturates(t *testing.T) {
	env, err := risor.NewEnv(
		risor.Limits{Steps: risor.NoLimit, Alloc: risor.NoLimit},
		risor.Function{Name: "big", Type: "() => int", Impl: func(c *risor.Call, args []any) (any, error) {
			for range 3 {
				if err := c.Charge(math.MaxInt64); err != nil {
					return nil, err
				}
				if err := c.Allocate(math.MaxInt64); err != nil {
					return nil, err
				}
			}
			return 0.0, nil
		}},
	)
	assert.NoError(t, err)
	d, u := measure(t, env, "big()")
	assert.Nil(t, d)
	assert.Equal(t, u, risor.Usage{Steps: math.MaxInt64, Alloc: math.MaxInt64})
}

// After a charge that failed, the call handle charges nothing more and
// reports the first failure, which is the one the run ends with
// (02-evaluation.md 2.6).
func TestHostChargeAfterFailure(t *testing.T) {
	var errs []error
	env, err := risor.NewEnv(
		risor.Limits{Steps: 100, Alloc: 10},
		risor.Function{Name: "greedy", Type: "() => int", Impl: func(c *risor.Call, args []any) (any, error) {
			// The host ignores each error and goes on.
			errs = append(errs, c.Allocate(11), c.Charge(1000), c.Charge(1), c.Allocate(1))
			return 0.0, nil
		}},
	)
	assert.NoError(t, err)
	d, u := measure(t, env, "greedy()")
	assert.Equal(t, d.Code, "allocation-exceeded")
	for _, e := range errs {
		assert.Error(t, e)
		assert.Equal(t, e.Error(), "allocation-exceeded")
	}
	assert.Equal(t, u, risor.Usage{Steps: 1, Alloc: 0})
}

// A limit diagnostic has a span; its value is not specified (03-host.md
// 3.7.1).
func TestLimitDiagnosticHasSpan(t *testing.T) {
	env, err := risor.NewEnv(
		risor.Limits{Steps: 5, Alloc: 3},
		risor.Function{Name: "alloc", Type: "() => int", Impl: func(c *risor.Call, args []any) (any, error) {
			return 0.0, c.Allocate(100)
		}},
	)
	assert.NoError(t, err)
	for _, src := range []string{
		"range(100).map(x => x * 2).sum()",
		"[1, 2, 3, 4]",
		"alloc()",
	} {
		d, _ := measure(t, env, src)
		assert.True(t, d != nil, src)
		assert.Equal(t, d.Class, "limit", src)
		assert.True(t, d.Span != nil, src)
	}
}
