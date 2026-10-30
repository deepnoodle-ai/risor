package builtins

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/deepnoodle-ai/risor/v2/pkg/errors"
	"github.com/deepnoodle-ai/risor/v2/pkg/object"
	"github.com/deepnoodle-ai/wonton/assert"
)

func hugeRange() *object.Range {
	return object.NewRange(1, 1<<60, 1)
}

func assertValueError(t *testing.T, err error, substr string) {
	t.Helper()
	var ve *errors.ValueError
	assert.ErrorAs(t, err, &ve)
	assert.ErrorContains(t, err, substr)
}

// C-10: materializing a huge range fails fast with a value error.
func TestAuditListKeysRangeLimit(t *testing.T) {
	ctx := context.Background()
	_, err := List(ctx, hugeRange())
	assertValueError(t, err, "list too large")

	_, err = Keys(ctx, hugeRange())
	assertValueError(t, err, "list too large")

	result, err := List(ctx, object.NewRange(0, 3, 1))
	assert.Nil(t, err)
	assert.Equal(t, result.Inspect(), "[0, 1, 2]")

	result, err = Keys(ctx, object.NewRange(5, 8, 1))
	assert.Nil(t, err)
	assert.Equal(t, result.Inspect(), "[0, 1, 2]")
}

// H-21: builtins that enumerate stop and report ctx errors.
func TestAuditBuiltinsContextCancellation(t *testing.T) {
	truthy := object.NewBuiltin("truthy", func(ctx context.Context, args ...object.Object) (object.Object, error) {
		return object.True, nil
	})
	falsy := object.NewBuiltin("falsy", func(ctx context.Context, args ...object.Object) (object.Object, error) {
		return object.False, nil
	})
	for name, fn := range map[string]func(ctx context.Context) (object.Object, error){
		"all":    func(ctx context.Context) (object.Object, error) { return All(ctx, hugeRange()) },
		"filter": func(ctx context.Context) (object.Object, error) { return Filter(ctx, hugeRange(), falsy) },
	} {
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
			defer cancel()
			start := time.Now()
			_, err := fn(ctx)
			assert.ErrorIs(t, err, context.DeadlineExceeded)
			assert.Less(t, time.Since(start), 5*time.Second)
		})
	}

	// A truthy filter over a huge range is stopped by the deadline or by the
	// list length limit, whichever comes first.
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	_, err := Filter(ctx, hugeRange(), truthy)
	assert.Error(t, err)

	ctx, cancel = context.WithCancel(context.Background())
	cancel()
	_, err = All(ctx, hugeRange())
	assert.ErrorIs(t, err, context.Canceled)
	_, err = Any(ctx, hugeRange())
	assert.ErrorIs(t, err, context.Canceled)
	_, err = Keys(ctx, object.NewRange(0, 10000, 1))
	assert.ErrorIs(t, err, context.Canceled)
	_, err = List(ctx, object.NewRange(0, 10000, 1))
	assert.ErrorIs(t, err, context.Canceled)
}

// M-14: sorted() stops calling the comparator after the first error and
// does not call it at all once the context is canceled.
func TestAuditSortedShortCircuit(t *testing.T) {
	items := make([]object.Object, 1000)
	for i := range items {
		items[i] = object.NewInt(int64(i))
	}
	calls := 0
	failing := object.NewBuiltin("cmp", func(ctx context.Context, args ...object.Object) (object.Object, error) {
		calls++
		return nil, fmt.Errorf("boom")
	})
	_, err := Sorted(context.Background(), object.NewList(items), failing)
	assert.ErrorContains(t, err, "boom")
	assert.Equal(t, calls, 1)

	calls = 0
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	counting := object.NewBuiltin("cmp", func(ctx context.Context, args ...object.Object) (object.Object, error) {
		calls++
		return object.True, nil
	})
	_, err = Sorted(ctx, object.NewList(items), counting)
	assert.ErrorIs(t, err, context.Canceled)
	assert.Equal(t, calls, 0)
}
