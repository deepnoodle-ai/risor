package value

import (
	"math"
	"testing"

	"github.com/deepnoodle-ai/wonton/assert"
)

func TestFormatNumber(t *testing.T) {
	for f, s := range map[float64]string{
		0: "0", 100: "100", 123456789012345680000: "123456789012345680000",
		123456.789: "123456.789", 4.35: "4.35", 0.5: "0.5", 0.000001: "0.000001",
		0.30000000000000004: "0.30000000000000004", 1e21: "1e+21", 1.5e-7: "1.5e-7",
		5e-324: "5e-324", -2.5: "-2.5", 1e20: "100000000000000000000",
		math.MaxFloat64: "1.7976931348623157e+308", 9007199254740991: "9007199254740991",
	} {
		assert.Equal(t, FormatNumber(f), s)
	}
}

func TestReplaceInvalid(t *testing.T) {
	for in, out := range map[string]string{
		"\xe2\x82A":        "�A",
		"\xff":             "�",
		"\xf0\x9f\x98":     "�",
		"\xed\xa0\x80":     "���",
		"\xe0\x80\x80":     "���",
		"a\xc0\xafb":       "a��b",
		"\xf4\x90\x80\x80": "����",
		"ok é 😀":           "ok é 😀",
	} {
		assert.Equal(t, ReplaceInvalid(in), out, "%q", in)
	}
}

func TestWeight(t *testing.T) {
	assert.Equal(t, Weight("a😀"), int64(3))
	assert.Equal(t, Size(string(make([]byte, 64))), int64(1))
	assert.Equal(t, CommonPrefix("héllo", "hélp"), "hél")
	assert.Equal(t, CommonPrefix("é", "è"), "")
	assert.Equal(t, Cut(string(make([]rune, 65))) != "", true)
}
