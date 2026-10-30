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

func TestWithLimitsRangeConversion(t *testing.T) {
	ctx := context.Background()
	limits := WithLimits(Limits{MaxListLength: 10})
	env := WithEnv(Builtins())

	// Ranges within the limit convert to their values, also when nested
	for src, want := range map[string]string{
		`sprintf("%v", range(3))`:              "[0 1 2]",
		`sprintf("%v", [range(3)])`:            "[[0 1 2]]",
		`error("%v", {a: range(3)}).message()`: "map[a:[0 1 2]]",
		`encode({a: [range(3)]}, "json")`:      `{"a":[[0,1,2]]}`,
	} {
		result, err := Eval(ctx, src, env, limits)
		assert.Nil(t, err, src)
		assert.Equal(t, result, want, src)
	}

	// Ranges over the limit raise a value error instead of converting to nil
	for _, src := range []string{
		`sprintf("%v", range(11))`,
		`sprintf("%v", [1, range(11)])`,
		`error("%v", {a: range(11)})`,
		`encode(range(11), "json")`,
		`encode({a: [range(11)]}, "json")`,
	} {
		_, err := Eval(ctx, src, env, limits)
		assert.NotNil(t, err, src)
		assert.True(t, strings.Contains(err.Error(), "range too large"), src)
	}

	// A raised limit lets a range above the default limit convert
	src := `len(sprintf("%v", range(4194305)))` // DefaultMaxListLength + 1
	_, err := Eval(ctx, src, env)
	assert.NotNil(t, err)
	result, err := Eval(ctx, src, env, WithLimits(Limits{MaxListLength: 8 << 20}))
	assert.Nil(t, err)
	assert.True(t, result.(int64) > 4194305)
}
