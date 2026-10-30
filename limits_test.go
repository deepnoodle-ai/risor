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
		`[1, 2, 3, 4, 5, 6] + [1, 2, 3, 4, 5]`,
		`[1, 2, 3, 4, 5, 6].extend([1, 2, 3, 4, 5])`,
		`"aaaaaaaaaaa".split("")`,
		`"a a a a a a a a a a a".fields()`,
		`regexp.split("", "aaaaaaaaaaa")`,
		`regexp.find_all("a", "aaaaaaaaaaa")`,
		`regexp.compile("").split("aaaaaaaaaaa")`,
		`regexp.compile("a").find_all("aaaaaaaaaaa")`,
		`regexp.compile("(a)").find_all_groups("aaaaaa")`,
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
		`[1, 2, 3, 4, 5] + [1, 2, 3, 4, 5]`,
		`[1, 2, 3, 4, 5].extend([1, 2, 3, 4, 5])`,
		`"aaaaaaaaaa".split("")`,
		`"a a a a a a a a a a".fields()`,
		`regexp.split("", "aaaaaaaaaa")`,
		`regexp.find_all("a", "aaaaaaaaaaa", 10)`,
		`regexp.compile("a").find_all("aaaaaaaaaaa", 10)`,
		`regexp.compile("(a)").find_all_groups("aaaaa")`,
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

func TestWithLimitsRangeToHostFunc(t *testing.T) {
	ctx := context.Background()
	limits := WithLimits(Limits{MaxListLength: 10})
	envMap := Builtins()
	envMap["count"] = func(v any) int {
		items, _ := v.([]any)
		return len(items)
	}
	envMap["countInts"] = func(v []int64) int { return len(v) }
	envMap["countPtr"] = func(v *any) int {
		items, _ := (*v).([]any)
		return len(items)
	}
	envMap["countNested"] = func(v []any) int {
		items, _ := v[0].([]any)
		return len(items)
	}
	env := WithEnv(envMap)

	// Ranges within the limit reach the host as their values
	for src, want := range map[string]int64{
		`count(range(3))`:         3,
		`countInts(range(3))`:     3,
		`countNested([range(3)])`: 3,
		`countPtr(range(3))`:      3,
	} {
		result, err := Eval(ctx, src, env, limits)
		assert.Nil(t, err, src)
		assert.Equal(t, result, want, src)
	}

	// Ranges over the limit raise a value error instead of reaching the host
	// as nil
	for _, src := range []string{
		`count(range(11))`,
		`countInts(range(11))`,
		`countNested([range(11)])`,
		`countPtr(range(11))`,
	} {
		_, err := Eval(ctx, src, env, limits)
		assert.NotNil(t, err, src)
		assert.True(t, strings.Contains(err.Error(), "range too large"), src)
	}
}
