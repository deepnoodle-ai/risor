package math

import (
	"context"
	"math"
	"testing"

	"github.com/deepnoodle-ai/risor/v2/pkg/errors"
	"github.com/deepnoodle-ai/risor/v2/pkg/object"
	"github.com/deepnoodle-ai/wonton/assert"
)

// L-03: abs(MinInt64) is not representable and must not wrap to negative.
func TestAuditAbsMinInt64(t *testing.T) {
	ctx := context.Background()
	_, err := Abs(ctx, object.NewInt(math.MinInt64))
	var ve *errors.ValueError
	assert.ErrorAs(t, err, &ve)
	assert.ErrorContains(t, err, "overflow")

	result, err := Abs(ctx, object.NewInt(math.MinInt64+1))
	assert.Nil(t, err)
	assert.Equal(t, result.(*object.Int).Value(), int64(math.MaxInt64))
}
