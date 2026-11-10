package messages

import (
	"os"
	"strings"
	"testing"

	"github.com/deepnoodle-ai/wonton/assert"
)

// The generated copy must be the file of the suite: run go generate.
func TestCatalogIsCurrent(t *testing.T) {
	data, err := os.ReadFile("../../suite/messages.json")
	if err != nil {
		t.Skip("suite/messages.json is not there")
	}
	if string(data) != sourceJSON {
		t.Fatal("catalog_gen.go is stale: run go generate ./internal/... at the repository root")
	}
}

func TestBuild(t *testing.T) {
	assert.Equal(t, Build("divide-by-zero", "divisor", Word("op", "/")), "the divisor of / is zero")
	assert.Equal(t, Build("steps-exceeded", "", Word("limit", "5")), "the run needed more than its limit of 5 steps")
	assert.Equal(t, Build("no-match", "", Val("found", "c")), `the match has no arm for "c"`)
	assert.Equal(t, Build("no-match", "", Val("found", []any{})), `the match has no arm for list`)
	long := strings.Repeat("a", 100)
	assert.Equal(t, Build("no-match", "", Val("found", long)), `the match has no arm for "`+strings.Repeat("a", 60)+`...`)
	assert.True(t, Caught("divide-by-zero"))
	assert.False(t, Caught("steps-exceeded"))
	// 19 run codes, and with package F8 the 58 parse and 29 check codes.
	assert.Equal(t, len(Codes()), 106)
}

// The generated copy of the F7 file must be the file of the suite.
func TestQueriesTableIsCurrent(t *testing.T) {
	data, err := os.ReadFile("../../suite/messages-queries.json")
	if err != nil {
		t.Skip("suite/messages-queries.json is not there")
	}
	if string(data) != sourceQueriesJSON {
		t.Fatal("queries_gen.go is stale: run go generate ./internal/... at the repository root")
	}
}
