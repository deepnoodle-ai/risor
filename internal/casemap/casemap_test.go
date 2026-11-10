package casemap

import (
	"os"
	"testing"

	"github.com/deepnoodle-ai/wonton/assert"
)

func TestTableIsCurrent(t *testing.T) {
	data, err := os.ReadFile("../../suite/casemap.json")
	if err != nil {
		t.Skip("suite/casemap.json is not there")
	}
	if string(data) != sourceJSON {
		t.Fatal("casemap_gen.go is stale: run go generate ./internal/... at the repository root")
	}
}

func TestMapping(t *testing.T) {
	u, l := Sizes()
	assert.Equal(t, u, 1477)
	assert.Equal(t, l, 1460)
	assert.Equal(t, Upper("café ß ς ǆ ﬁ"), "CAFÉ ß Σ Ǆ ﬁ")
	assert.Equal(t, Lower("ÀB İ ΑΣ ǅ 😀A"), "àb i ασ ǆ 😀a")
}
