package risor

import (
	"context"
	"strings"
	"testing"

	"github.com/deepnoodle-ai/risor/v2/pkg/object"
	"github.com/deepnoodle-ai/wonton/assert"
)

func TestWithLimitsLowered(t *testing.T) {
	ctx := context.Background()
	limits := WithLimits(Limits{MaxBytesSize: 10, MaxListLength: 10})
	env := WithEnv(Builtins())

	for _, src := range []string{
		`"a".repeat(11)`,
		`"aaaaaa" + "aaaaa"`,
		`",".join(["aaaa", "aaaa", "aaaa"])`,
		`"aaaaaa".replace_all("a", "xx")`,
		`bytes("a").repeat(11)`,
		`list(range(11))`,
		`[...range(11)]`,
		`range(11).map(x => x)`,
		`rand.bytes(11)`,
		`regexp.replace("a", "aaaaaaaaaaa", "xx")`,
	} {
		_, err := Eval(ctx, src, env, limits)
		assert.NotNil(t, err, src)
		assert.True(t, strings.Contains(err.Error(), "too large"), src)
	}

	// Values at the limit are allowed
	for _, src := range []string{
		`"a".repeat(10)`,
		`"aaaaa" + "aaaaa"`,
		`list(range(10))`,
		`rand.bytes(10)`,
	} {
		_, err := Eval(ctx, src, env, limits)
		assert.Nil(t, err, src)
	}
}

func TestWithLimitsRaised(t *testing.T) {
	ctx := context.Background()
	src := `len("a".repeat(67108865))` // DefaultMaxBytesSize + 1

	_, err := Eval(ctx, src, WithEnv(Builtins()))
	assert.NotNil(t, err)
	assert.True(t, strings.Contains(err.Error(), "too large"))

	result, err := Eval(ctx, src, WithEnv(Builtins()), WithLimits(Limits{MaxBytesSize: 128 << 20}))
	assert.Nil(t, err)
	assert.Equal(t, result, int64(object.DefaultMaxBytesSize+1))
}

func TestWithLimitsZeroFieldsUseDefaults(t *testing.T) {
	ctx := context.Background()
	limits := WithLimits(Limits{MaxBytesSize: 10})

	// MaxListLength was not set, so the default applies
	result, err := Eval(ctx, `len(list(range(1000)))`, WithEnv(Builtins()), limits)
	assert.Nil(t, err)
	assert.Equal(t, result, int64(1000))

	_, err = Eval(ctx, `list(range(4194305))`, WithEnv(Builtins()), limits)
	assert.NotNil(t, err)
	assert.True(t, strings.Contains(err.Error(), "list too large"))
}

func TestWithLimitsCatchable(t *testing.T) {
	result, err := Eval(context.Background(), `
	let r = "none"
	try { "a".repeat(11) } catch e { r = e.kind() }
	r
	`, WithLimits(Limits{MaxBytesSize: 10}))
	assert.Nil(t, err)
	assert.Equal(t, result, "value error")
}
