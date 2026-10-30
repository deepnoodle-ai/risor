package regexp

import (
	"context"
	"strings"
	"testing"

	"github.com/deepnoodle-ai/risor/v2/pkg/errors"
	"github.com/deepnoodle-ai/risor/v2/pkg/object"
	"github.com/deepnoodle-ai/wonton/assert"
)

func replaceString(t *testing.T, args ...object.Object) string {
	t.Helper()
	result, err := Replace(context.Background(), args...)
	assert.Nil(t, err)
	return result.(*object.String).Value()
}

func str(v string) object.Object { return object.NewString(v) }

func num(v int64) object.Object { return object.NewInt(v) }

// Replace with a count matches the original string once and never
// re-matches its own replacement text.
func TestReplaceCountSinglePass(t *testing.T) {
	assert.Equal(t, replaceString(t, str("a"), str("ab"), str("aa"), num(3)), "aab")
	assert.Equal(t, replaceString(t, str("^"), str("abc"), str("X"), num(3)), "Xabc")
	assert.Equal(t, replaceString(t, str(""), str("abc"), str("-"), num(2)), "-a-bc")
	assert.Equal(t, replaceString(t, str("[0-9]+"), str("a1b2c3"), str("X"), num(2)), "aXbXc3")
	assert.Equal(t, replaceString(t, str("[0-9]+"), str("a1b2c3"), str("X")), "aXbXcX")
	assert.Equal(t, replaceString(t, str("[0-9]+"), str("a1b2c3"), str("X"), num(0)), "aXbXcX")
	assert.Equal(t, replaceString(t, str("[0-9]+"), str("a1b2c3"), str("X"), num(-1)), "a1b2c3")
	assert.Equal(t, replaceString(t, str(`(\w+)@(\w+)`), str("a@b c@d"), str("$2:$1"), num(1)), "b:a c@d")
	// Anchors are evaluated against the whole string, not the match alone.
	assert.Equal(t, replaceString(t, str(`b$`), str("abab"), str("X"), num(1)), "abaX")
	assert.Equal(t, replaceString(t, str(`\bx`), str("x xx x"), str("Y"), num(2)), "Y Yx x")
	assert.Equal(t, replaceString(t, str(`(?m)^`), str("a\nb\nc"), str("> "), num(2)), "> a\n> b\nc")
	// Literal replacements expand $$ and unknown names, like ReplaceAllString.
	assert.Equal(t, replaceString(t, str("a"), str("aaa"), str("$$${x}"), num(2)), "$$a")
	assert.Equal(t, replaceString(t, str("(a)"), str("aaa"), str("<$1>"), num(2)), "<a><a>a")

	re, err := Compile(context.Background(), str("a"))
	assert.Nil(t, err)
	method, ok := re.GetAttr("replace")
	assert.True(t, ok)
	result, err := method.(*object.Builtin).Call(context.Background(), str("ab"), str("aa"), num(3))
	assert.Nil(t, err)
	assert.Equal(t, result.(*object.String).Value(), "aab")
}

func TestReplaceLimits(t *testing.T) {
	ctx := context.Background()
	input := str(strings.Repeat("a", 1<<16))
	_, err := Replace(ctx, str("a"), input, str(strings.Repeat("x", 2048)))
	var ve *errors.ValueError
	assert.ErrorAs(t, err, &ve)
	assert.ErrorContains(t, err, "too large")

	// One match expanded many times via group references.
	_, err = Replace(ctx, str("(.*)"), input, str(strings.Repeat("$1", 2048)), num(1))
	assert.ErrorContains(t, err, "too large")

	re, err := Compile(ctx, str("a"))
	assert.Nil(t, err)
	method, _ := re.GetAttr("replace_all")
	_, err = method.(*object.Builtin).Call(ctx, input, str(strings.Repeat("x", 2048)))
	assert.ErrorContains(t, err, "too large")

	// Size checks are exact enough not to reject identity replacements.
	result, err := Replace(ctx, str("a"), input, str("$0"))
	assert.Nil(t, err)
	assert.Equal(t, len(result.(*object.String).Value()), 1<<16)
	result, err = Replace(ctx, str("(a)"), input, str("<$1>"), num(2))
	assert.Nil(t, err)
	assert.Equal(t, result.(*object.String).Value()[:7], "<a><a>a")

	canceled, cancel := context.WithCancel(ctx)
	cancel()
	_, err = Replace(canceled, str("a"), input, str("b"))
	assert.ErrorIs(t, err, context.Canceled)
}

// Replace with a count and a literal replacement streams over the matches, so
// a huge count needs no memory bound. With group references, the match
// indices are held in memory, so the count is bounded by the byte limit.
func TestReplaceCountBudget(t *testing.T) {
	ctx := object.WithLimits(context.Background(), object.Limits{MaxBytesSize: 1024})
	input := str(strings.Repeat("a", 1000))

	result, err := Replace(ctx, str("a"), input, str("b"), num(999))
	assert.Nil(t, err)
	assert.Equal(t, result.(*object.String).Value(), strings.Repeat("b", 999)+"a")

	// Each match of (a) holds 4 indices: 24 + 8*4 = 56 bytes, and 1024/56 = 18.
	result, err = Replace(ctx, str("(a)"), input, str("$1"), num(18))
	assert.Nil(t, err)
	assert.Equal(t, result.(*object.String).Value(), strings.Repeat("a", 1000))
	_, err = Replace(ctx, str("(a)"), input, str("$1"), num(19))
	var ve *errors.ValueError
	assert.ErrorAs(t, err, &ve)
	assert.ErrorContains(t, err, "regexp.replace: count too large (19 exceeds limit of 18 for this pattern)")
}
