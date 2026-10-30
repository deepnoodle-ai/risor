package risor

import (
	"context"
	"strings"
	"testing"

	"github.com/deepnoodle-ai/wonton/assert"
)

// Operations that build strings, bytes, or lists must respect WithLimits.
func TestWithLimitsBuildOperations(t *testing.T) {
	ctx := context.Background()
	limits := WithLimits(Limits{MaxBytesSize: 15, MaxListLength: 3})
	env := WithEnv(Builtins())

	for _, src := range []string{
		"let a = '1234567890'; `${a}${a}`",
		`sprintf("%20s", "")`,
		`string([1234567890, 1234567890])`,
		`encode("1234567890", "hex")`,
		`encode("12345678901234567890", "json")`,
		`[1, 2, 3, 4]`,
		`let a = [1, 2]; [...a, 3, 4]`,
		`let l = [1, 2, 3]; l.append(4)`,
		`let l = [1, 2, 3]; l.insert(0, 4)`,
		`decode("[1, 2, 3, 4]", "json")`,
		`decode("{\"a\": [1, 2, 3, 4]}", "json")`,
		`decode("a\nb\nc\nd", "csv")`,
		`decode("a,b,c,d", "csv")`,
	} {
		_, err := Eval(ctx, src, env, limits)
		assert.NotNil(t, err, src)
		assert.True(t, strings.Contains(err.Error(), "too large"), "%s: %v", src, err)
	}

	// Values at the limit are allowed
	for _, src := range []string{
		"let a = '12345'; `${a}${a}${a}`",
		`sprintf("%15s", "")`,
		`string([12345, 12345])`,
		`encode("1234567", "hex")`,
		`[1, 2, 3]`,
		`let a = [1, 2]; [...a, 3]`,
		`let l = [1, 2]; l.append(3)`,
		`let l = [1, 2]; l.insert(0, 3)`,
		`decode("[1, 2, 3]", "json")`,
		`len(decode("a,b,c", "csv")[0])`,
	} {
		_, err := Eval(ctx, src, env, limits)
		assert.Nil(t, err, src)
	}
}
