package risor_test

import (
	"context"
	"errors"
	"sync"
	"testing"

	risor "github.com/deepnoodle-ai/risor/go/v3"
	"github.com/deepnoodle-ai/wonton/assert"
)

// A host function that keeps its call handle cannot use it after it returns
// (03-host.md 3.5.2 item 6): Charge and Allocate return
// ErrCallEnded and change no run, and Context is cancelled. That holds after
// the run, and while a second run of the same program, which may reuse the
// first run's state, is in progress. Run under -race.
func TestCallHandleAfterReturn(t *testing.T) {
	var mu sync.Mutex
	var kept *risor.Call
	gate := make(chan struct{})    // the second run waits here
	entered := make(chan struct{}) // the second run has reached its host call
	env, err := risor.NewEnv(
		risor.VarRecord("{n: float}"),
		risor.Function{Name: "keep", Type: "(x: float) => float", Impl: func(c *risor.Call, args []any) (any, error) {
			mu.Lock()
			if kept == nil {
				kept = c
			}
			mu.Unlock()
			if err := c.Charge(2); err != nil {
				return nil, err
			}
			return args[0], nil
		}},
		risor.Function{Name: "wait", Type: "(x: float) => float", Impl: func(c *risor.Call, args []any) (any, error) {
			if args[0].(float64) == 1 {
				close(entered)
				<-gate
			}
			return args[0], nil
		}},
	)
	assert.NoError(t, err)
	x, err := risor.Parse("keep(n) + wait(n) + n * 2")
	assert.NoError(t, err)
	p, err := env.Compile(x)
	assert.NoError(t, err)
	ctx := context.Background()

	_, want, err := p.Measure(ctx, risor.Map{"n": 0.0})
	assert.NoError(t, err)

	// After the run has returned.
	assert.True(t, kept != nil)
	assert.True(t, errors.Is(kept.Charge(5), risor.ErrCallEnded))
	assert.True(t, errors.Is(kept.Allocate(5), risor.ErrCallEnded))
	assert.Error(t, kept.Context().Err())

	// While a second run of the same program is in progress.
	done := make(chan risor.Usage)
	go func() {
		_, u, err := p.Measure(ctx, risor.Map{"n": 1.0})
		assert.NoError(t, err)
		done <- u
	}()
	<-entered
	for i := 0; i < 100; i++ {
		assert.True(t, errors.Is(kept.Charge(1000), risor.ErrCallEnded))
		assert.True(t, errors.Is(kept.Allocate(1000), risor.ErrCallEnded))
	}
	assert.Error(t, kept.Context().Err())
	close(gate)
	got := <-done
	assert.Equal(t, got, want)

	// A late use racing a run from another goroutine.
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 1000; i++ {
			_ = kept.Charge(1)
			_ = kept.Allocate(1)
			_ = kept.Context()
		}
	}()
	for i := 0; i < 50; i++ {
		_, u, err := p.Measure(ctx, risor.Map{"n": 0.0})
		assert.NoError(t, err)
		assert.Equal(t, u, want)
	}
	wg.Wait()
}
