package risor

import (
	"context"
	"fmt"
	"reflect"
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
		`"a,a,a,a,a,a,a,a,a,a,a".split(",")`,
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
		`"a,a,a,a,a,a,a,a,a,a".split(",")`,
		`"a a a a a a a a a a".fields()`,
		`regexp.split("", "aaaaaaaaaa")`,
		`regexp.find_all("a", "aaaaaaaaaaa", 10)`,
		`regexp.compile("a").find_all("aaaaaaaaaaa", 10)`,
		// len() keeps the result conversion, which counts the nested
		// lists too, within the limit
		`len(regexp.compile("(a)").find_all_groups("aaaaa"))`,
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

	// Ranges over the limit raise a value error instead of converting to nil.
	// A list counts its items too: [1, range(11)] has 13 elements.
	for _, src := range []string{
		`sprintf("%v", range(11))`,
		`sprintf("%v", [1, range(11)])`,
		`error("%v", {a: range(11)})`,
		`encode(range(11), "json")`,
		`encode({a: [range(11)]}, "json")`,
		`encode({a: range(11)}.values(), "json")`,
		`sprintf("%v", {a: range(11)}.values())`,
	} {
		_, err := Eval(ctx, src, env, limits)
		assert.NotNil(t, err, src)
		assert.True(t, strings.Contains(err.Error(), "value too large to convert"), src)
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
	// A range converts to []int64 for an any parameter
	envMap["count"] = func(v any) int {
		items, _ := v.([]int64)
		return len(items)
	}
	envMap["countInts"] = func(v []int64) int { return len(v) }
	envMap["countPtr"] = func(v *any) int {
		items, _ := (*v).([]int64)
		return len(items)
	}
	envMap["countNested"] = func(v []any) int {
		items, _ := v[0].([]int64)
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
		assert.True(t, strings.Contains(err.Error(), "value too large to convert"), src)
	}
}

// One conversion of a value to Go counts every element it produces against
// MaxListLength, including the elements of a list each time it is
// referenced. A list holding a 3-element list twice has 2 + 2*3 = 8
// elements; three times, it has 3 + 3*3 = 12.
func TestWithLimitsSharedReferences(t *testing.T) {
	ctx := context.Background()
	limits := WithLimits(Limits{MaxListLength: 10})
	envMap := Builtins()
	envMap["count"] = func(v any) int { return len(v.([]any)) }
	envMap["countNested"] = func(v [][]int64) int { return len(v) }
	env := WithEnv(envMap)

	under := `let l = [1, 2, 3]; `
	for src, want := range map[string]any{
		under + `sprintf("%v", [l, l])`:        "[[1 2 3] [1 2 3]]",
		under + `encode([l, l], "json")`:       "[[1,2,3],[1,2,3]]",
		under + `[l, l]`:                       []any{[]any{int64(1), int64(2), int64(3)}, []any{int64(1), int64(2), int64(3)}},
		under + `count([l, l])`:                int64(2),
		under + `countNested([l, l])`:          int64(2),
		`let r = range(6); sprintf("%v", [r])`: "[[0 1 2 3 4 5]]",
	} {
		result, err := Eval(ctx, src, env, limits)
		assert.Nil(t, err, src)
		assert.Equal(t, result, want, src)
	}

	over := `let l = [1, 2, 3]; `
	for _, src := range []string{
		over + `sprintf("%v", [l, l, l])`,
		over + `encode([l, l, l], "json")`,
		over + `[l, l, l]`,
		over + `count([l, l, l])`,
		over + `countNested([l, l, l])`,
		`let r = range(6); sprintf("%v", [r, r])`, // 2 + 2*6 = 14
		`let r = range(6); [r, r]`,
	} {
		_, err := Eval(ctx, src, env, limits)
		assert.NotNil(t, err, src)
		assert.True(t, strings.Contains(err.Error(), "value too large to convert (exceeds limit of 10 elements)"), src)
	}
}

// The arguments of one builtin or host call share one element budget, so a
// spread call cannot pass many references to a large list. Two references to
// a 4-element list cost 2*4 = 8 elements. Three references cost 12.
func TestWithLimitsSpreadArguments(t *testing.T) {
	ctx := context.Background()
	limits := WithLimits(Limits{MaxListLength: 10})
	envMap := Builtins()
	envMap["count2"] = func(a, b any) int { return len(a.([]any)) + len(b.([]any)) }
	envMap["count3"] = func(a, b, c any) int { return 3 }
	envMap["countAll"] = func(vs ...any) int { return len(vs) }
	env := WithEnv(envMap)

	under := `let l = [1, 2, 3, 4]; `
	for src, want := range map[string]any{
		under + `sprintf("%v %v", ...[l, l])`: "[1 2 3 4] [1 2 3 4]",
		under + `count2(...[l, l])`:           int64(8),
		under + `countAll(...[l, l])`:         int64(2),
	} {
		result, err := Eval(ctx, src, env, limits)
		assert.Nil(t, err, src)
		assert.Equal(t, result, want, src)
	}

	over := `let l = [1, 2, 3, 4]; `
	for _, src := range []string{
		over + `sprintf("%v %v %v", ...[l, l, l])`,
		over + `error("%v %v %v", ...[l, l, l])`,
		over + `count3(...[l, l, l])`,
		over + `countAll(...[l, l, l])`,
	} {
		_, err := Eval(ctx, src, env, limits)
		assert.NotNil(t, err, src)
		assert.True(t, strings.Contains(err.Error(), "value too large to convert"), src)
	}

	// 63 references to a 100-element list cost 6300 elements
	src := `let l = list(range(100)); sprintf("%v", ...list(range(63)).map(i => l))`
	_, err := Eval(ctx, src, env, WithLimits(Limits{MaxListLength: 1000}))
	assert.NotNil(t, err)
	assert.True(t, strings.Contains(err.Error(), "value too large to convert"), err.Error())
}

// Without WithLimits, a list that references one large list many times
// fails to convert instead of building billions of elements.
func TestDefaultLimitsSharedReferences(t *testing.T) {
	src := `let l = list(range(4096)); list(range(4096)).map(i => l)`
	_, err := Eval(context.Background(), src, WithEnv(Builtins()))
	assert.NotNil(t, err)
	assert.True(t, strings.Contains(err.Error(), "value too large to convert"), err.Error())
}

type limitsBox struct {
	Items []int64
	Grid  [][]int64
	Rows  [][]int64
}

// With MaxListLength 10, each list literal below is within the limit, so the
// script can build it. Converting it to Go counts every element: rowsOver
// has 3 rows and 9 items, 12 in total, and rowsAtLimit has 10.
const (
	rowsOver    = `box.Rows = [[1, 2, 3, 4], [1, 2, 3, 4], [1]]`
	rowsAtLimit = `box.Rows = [[1, 2, 3, 4], [1, 2, 3, 4]]`
)

// Setting a field of a Go struct converts the value with the limits from
// WithLimits (#497).
func TestWithLimitsStructField(t *testing.T) {
	ctx := context.Background()
	limits := WithLimits(Limits{MaxListLength: 10})

	// A list literal longer than the limit fails before the conversion,
	// so the tests use a range, which converts without becoming a list.
	box := &limitsBox{}
	boxEnv := func() Option {
		env := Builtins()
		env["box"] = box
		return WithEnv(env)
	}
	_, err := Eval(ctx, `box.Items = range(11)`, boxEnv(), limits)
	assert.NotNil(t, err)
	assert.True(t, strings.Contains(err.Error(), "value too large to convert"), err.Error())
	assert.Equal(t, len(box.Items), 0)

	// The error is catchable
	result, err := Eval(ctx, `
	let r = "none"
	try { box.Items = range(11) } catch e { r = e.message() }
	r
	`, boxEnv(), limits)
	assert.Nil(t, err)
	assert.True(t, strings.Contains(result.(string), "value too large to convert"), result)

	// A value within the limit is set
	_, err = Eval(ctx, `box.Items = [1, 2, 3, 4, 5, 6, 7, 8, 9, 10]`,
		WithEnv(map[string]any{"box": box}), limits)
	assert.Nil(t, err)
	assert.Equal(t, len(box.Items), 10)

	// Without WithLimits, the default limit applies: 1025 + 1025*4096
	// elements exceeds 4,194,304
	box = &limitsBox{}
	envMap := Builtins()
	envMap["box"] = box
	_, err = Eval(ctx, `let l = list(range(4096)); box.Grid = list(range(1025)).map(i => l)`,
		WithEnv(envMap))
	assert.NotNil(t, err)
	assert.True(t, strings.Contains(err.Error(), "value too large to convert"), err.Error())
	assert.Equal(t, len(box.Grid), 0)
}

// guardedBox embeds *object.GoStruct and overrides SetAttr to block a field.
type guardedBox struct {
	*object.GoStruct
}

func (g *guardedBox) SetAttr(name string, value object.Object) error {
	if name == "Grid" {
		return object.TypeErrorf("field %q is read-only", name)
	}
	return g.GoStruct.SetAttr(name, value)
}

func TestWithLimitsStructEmbedderKeepsSetAttr(t *testing.T) {
	box := &limitsBox{}
	guarded := &guardedBox{object.NewGoStruct(reflect.ValueOf(box), object.DefaultRegistry())}
	env := WithEnv(map[string]any{"box": guarded})
	limits := WithLimits(Limits{MaxListLength: 10})

	// The host override runs and blocks the write
	_, err := Eval(context.Background(), `box.Grid = [[1]]`, env, limits)
	assert.NotNil(t, err)
	assert.True(t, strings.Contains(err.Error(), "read-only"), err.Error())
	assert.Equal(t, len(box.Grid), 0)

	// Other fields go through the override to GoStruct.SetAttr
	_, err = Eval(context.Background(), `box.Items = [1, 2, 3]`, env, limits)
	assert.Nil(t, err)
	assert.Equal(t, len(box.Items), 3)

	// The override did not opt in to AttrAssigner, so the default limits
	// apply, not WithLimits
	_, err = Eval(context.Background(), rowsOver, env, limits)
	assert.Nil(t, err)
	assert.Equal(t, len(box.Rows), 3)
}

// plainBox embeds *object.GoStruct and defines neither SetAttr nor
// AssignAttr.
type plainBox struct {
	*object.GoStruct
}

// An embedder that does not opt in gets the promoted GoStruct.SetAttr, which
// uses the default limits (#499).
func TestWithLimitsStructEmbedderDefaultLimits(t *testing.T) {
	box := &limitsBox{}
	plain := &plainBox{object.NewGoStruct(reflect.ValueOf(box), object.DefaultRegistry())}
	env := WithEnv(map[string]any{"box": plain})
	limits := WithLimits(Limits{MaxListLength: 10})

	_, err := Eval(context.Background(), rowsOver, env, limits)
	assert.Nil(t, err)
	assert.Equal(t, len(box.Rows), 3)
}

// assignBox embeds *object.GoStruct, blocks a field in both SetAttr and
// AssignAttr, and opts in to the VM context with AssignAttr.
type assignBox struct {
	*object.GoStruct
	setAttrCalls int
}

var _ object.AttrAssigner = (*assignBox)(nil)

func (b *assignBox) SetAttr(name string, value object.Object) error {
	b.setAttrCalls++
	if name == "Grid" {
		return object.TypeErrorf("field %q is read-only", name)
	}
	return b.GoStruct.SetAttr(name, value)
}

func (b *assignBox) AssignAttr(ctx context.Context, name string, value object.Object) error {
	if name == "Grid" {
		return object.TypeErrorf("field %q is read-only", name)
	}
	return b.GoStruct.SetAttrContext(ctx, name, value)
}

// An embedder that implements AttrAssigner gets the limits from WithLimits
// on field assignment (#499).
func TestWithLimitsStructEmbedderAssignAttr(t *testing.T) {
	box := &limitsBox{}
	assigner := &assignBox{GoStruct: object.NewGoStruct(reflect.ValueOf(box), object.DefaultRegistry())}
	env := WithEnv(map[string]any{"box": assigner})
	limits := WithLimits(Limits{MaxListLength: 10})

	// A value over the limit fails and leaves the field unchanged
	_, err := Eval(context.Background(), rowsOver, env, limits)
	assert.NotNil(t, err)
	assert.True(t, strings.Contains(err.Error(), "value too large to convert"), err.Error())
	assert.Equal(t, len(box.Rows), 0)

	// A value at the limit is set
	_, err = Eval(context.Background(), rowsAtLimit, env, limits)
	assert.Nil(t, err)
	assert.Equal(t, len(box.Rows), 2)

	// AssignAttr blocks the field
	_, err = Eval(context.Background(), `box.Grid = [[1]]`, env, limits)
	assert.NotNil(t, err)
	assert.True(t, strings.Contains(err.Error(), "read-only"), err.Error())
	assert.Equal(t, len(box.Grid), 0)

	// The VM called AssignAttr, not SetAttr
	assert.Equal(t, assigner.setAttrCalls, 0)
}

// Throwing a value, or failing an assert with a message, that is not a string
// prints the value into the error message. A value that repeats shared
// references prints as a placeholder, as object.Errorf does (#583).
func TestThrowAndAssertSharedReferencesBounded(t *testing.T) {
	for _, levels := range []int{22, 40} {
		build := fmt.Sprintf("let a = list(range(%d)).reduce([1], (acc, i) => [acc, acc])\n", levels)
		for _, raise := range []string{`throw a`, `assert(false, a)`} {
			src := build + "try { " + raise + " } catch e { e.message() }"
			result, err := Eval(context.Background(), src, WithEnv(Builtins()))
			assert.Nil(t, err)
			assert.Equal(t, result, "[...]")
		}
	}

	// A small value still prints in full
	result, err := Eval(context.Background(),
		`try { throw [1, {a: 2}] } catch e { e.message() }`, WithEnv(Builtins()))
	assert.Nil(t, err)
	assert.Equal(t, result, `[1, {"a": 2}]`)
}
