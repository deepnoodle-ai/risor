package risor

import (
	"context"
	"errors"
	"testing"

	"github.com/deepnoodle-ai/risor/v2/pkg/vm"
	"github.com/deepnoodle-ai/wonton/assert"
)

func TestMaxStepsBelowCheckInterval(t *testing.T) {
	_, err := Eval(context.Background(), `
	let sum = 0; list(range(100)).each(x => { sum += x }); sum
	`, WithEnv(Builtins()), WithMaxSteps(50))
	assert.True(t, errors.Is(err, vm.ErrStepLimitExceeded), "got %v", err)
}

// The step limit counts instructions exactly: a script that runs n
// instructions passes WithMaxSteps(n) and fails WithMaxSteps(n-1).
func TestMaxStepsExact(t *testing.T) {
	src := `let a = 1; let b = 2; a + b`
	// Find the smallest limit that the script runs under
	n := int64(1)
	for ; n < 100; n++ {
		if _, err := Eval(context.Background(), src, WithMaxSteps(n)); err == nil {
			break
		}
	}
	assert.True(t, n > 5 && n < 100, "steps: %d", n)
	_, err := Eval(context.Background(), src, WithMaxSteps(n-1))
	assert.True(t, errors.Is(err, vm.ErrStepLimitExceeded), "got %v", err)
	result, err := Eval(context.Background(), src, WithMaxSteps(n))
	assert.Nil(t, err)
	assert.Equal(t, result, int64(3))
}

func TestMaxStackDepthLimitsValueStack(t *testing.T) {
	_, err := Eval(context.Background(),
		`len([1,2,3,4,5,6,7,8,9,10,11,12,13,14,15,16,17,18,19,20])`,
		WithEnv(Builtins()), WithMaxStackDepth(5))
	assert.True(t, errors.Is(err, vm.ErrStackOverflow), "got %v", err)

	_, err = Eval(context.Background(),
		`let [a, b, c, d, e, f, g, h] = [1]; a`, WithMaxStackDepth(5))
	assert.True(t, errors.Is(err, vm.ErrStackOverflow), "got %v", err)

	result, err := Eval(context.Background(), `len([1, 2, 3])`,
		WithEnv(Builtins()), WithMaxStackDepth(5))
	assert.Nil(t, err)
	assert.Equal(t, result, int64(3))
}
