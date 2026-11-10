package library

import (
	"os"
	"testing"

	"github.com/deepnoodle-ai/risor/v3/internal/value"
	"github.com/deepnoodle-ai/wonton/assert"
)

// The generated copy must be the file of the suite: run go generate.
func TestTableIsCurrent(t *testing.T) {
	data, err := os.ReadFile("../../../suite/library.json")
	if err != nil {
		t.Skip("suite/library.json is not there")
	}
	if string(data) != sourceJSON {
		t.Fatal("table_gen.go is stale: run GOWORK=off go generate ./... in go/")
	}
}

func TestTable(t *testing.T) {
	assert.Equal(t, len(Entries), 53)
	assert.Equal(t, len(Functions), 15)
	m, ok := Method(value.List, "map")
	assert.True(t, ok)
	assert.Equal(t, m.Params[0].Callback, 2)
}
