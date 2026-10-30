package regexp

import (
	"context"
	"strings"
	"testing"

	"github.com/deepnoodle-ai/risor/v2/pkg/errors"
	"github.com/deepnoodle-ai/risor/v2/pkg/object"
	"github.com/deepnoodle-ai/wonton/assert"
)

func auditReplace(t *testing.T, args ...object.Object) string {
	t.Helper()
	result, err := Replace(context.Background(), args...)
	assert.Nil(t, err)
	return result.(*object.String).Value()
}

func str(v string) object.Object { return object.NewString(v) }

func num(v int64) object.Object { return object.NewInt(v) }

// H-20: replace with a count matches the original string once and never
// re-matches its own replacement text.
func TestAuditReplaceCountSinglePass(t *testing.T) {
	assert.Equal(t, auditReplace(t, str("a"), str("ab"), str("aa"), num(3)), "aab")
	assert.Equal(t, auditReplace(t, str("^"), str("abc"), str("X"), num(3)), "Xabc")
	assert.Equal(t, auditReplace(t, str(""), str("abc"), str("-"), num(2)), "-a-bc")
	assert.Equal(t, auditReplace(t, str("[0-9]+"), str("a1b2c3"), str("X"), num(2)), "aXbXc3")
	assert.Equal(t, auditReplace(t, str("[0-9]+"), str("a1b2c3"), str("X")), "aXbXcX")
	assert.Equal(t, auditReplace(t, str("[0-9]+"), str("a1b2c3"), str("X"), num(0)), "aXbXcX")
	assert.Equal(t, auditReplace(t, str("[0-9]+"), str("a1b2c3"), str("X"), num(-1)), "a1b2c3")
	assert.Equal(t, auditReplace(t, str(`(\w+)@(\w+)`), str("a@b c@d"), str("$2:$1"), num(1)), "b:a c@d")
	// Anchors are evaluated against the whole string, not the match alone.
	assert.Equal(t, auditReplace(t, str(`b$`), str("abab"), str("X"), num(1)), "abaX")

	re, err := Compile(context.Background(), str("a"))
	assert.Nil(t, err)
	method, ok := re.GetAttr("replace")
	assert.True(t, ok)
	result, err := method.(*object.Builtin).Call(context.Background(), str("ab"), str("aa"), num(3))
	assert.Nil(t, err)
	assert.Equal(t, result.(*object.String).Value(), "aab")
}

func TestAuditReplaceLimits(t *testing.T) {
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
