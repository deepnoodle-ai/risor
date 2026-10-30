package parser

import (
	"math"
	"testing"

	"github.com/deepnoodle-ai/risor/v2/pkg/ast"
	"github.com/deepnoodle-ai/wonton/assert"
)

// math.MinInt64 has no positive literal, so a negated literal of its
// magnitude parses as one negative literal.
func TestMinInt64Literal(t *testing.T) {
	for _, input := range []string{
		"-9223372036854775808",
		"-0x8000000000000000",
		"-01000000000000000000000",
		"-0b1000000000000000000000000000000000000000000000000000000000000000",
	} {
		t.Run(input, func(t *testing.T) {
			program, err := Parse(t.Context(), input, nil)
			assert.Nil(t, err)
			lit, ok := program.First().(*ast.Int)
			assert.True(t, ok)
			assert.Equal(t, lit.Value, int64(math.MinInt64))
			assert.Equal(t, lit.Literal, input)
			assert.Equal(t, lit.Pos().Column, 0)
			assert.Equal(t, lit.End().Column, len(input))
		})
	}

	// It is also an operand of a lower-precedence operator
	program, err := Parse(t.Context(), "-9223372036854775808 + 1", nil)
	assert.Nil(t, err)
	infix := program.First().(*ast.Infix)
	assert.Equal(t, infix.X.(*ast.Int).Value, int64(math.MinInt64))
}

// The magnitude of math.MinInt64 is out of range everywhere else.
func TestMinInt64MagnitudeOutOfRange(t *testing.T) {
	for _, input := range []string{
		"9223372036854775808",
		"-9223372036854775809",
		"-9223372036854775808 ** 2",
		"-9223372036854775808[0]",
		"-9223372036854775808(1)",
	} {
		t.Run(input, func(t *testing.T) {
			_, err := Parse(t.Context(), input, nil)
			assert.NotNil(t, err)
			assert.Contains(t, err.Error(), "invalid integer")
		})
	}
}

// The end of a nil literal depends on its spelling.
func TestNilLiteralEnd(t *testing.T) {
	for _, input := range []string{"nil", "null"} {
		program, err := Parse(t.Context(), input, nil)
		assert.Nil(t, err)
		lit := program.First().(*ast.Nil)
		assert.Equal(t, lit.Literal, input)
		assert.Equal(t, lit.End().Column, len(input))
	}
}
