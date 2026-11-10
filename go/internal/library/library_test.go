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
	data, err = os.ReadFile("../../../suite/library-groups.json")
	if err == nil && string(data) != sourceGroupsJSON {
		t.Fatal("groups_gen.go is stale: run GOWORK=off go generate ./... in go/")
	}
}

func TestTable(t *testing.T) {
	assert.Equal(t, len(Entries), 59) // 53 of Core and 6 of F7
	assert.Equal(t, len(Functions), 15)
	assert.Equal(t, len(GroupEntries), 8)
	assert.Equal(t, len(Constants), 2)
	assert.Equal(t, *Constants["math.pi"].Value, 3.141592653589793)
	m, ok := Method(value.List, "map")
	assert.True(t, ok)
	assert.Equal(t, m.Params[0].Callback, 2)
}

// The generated copy of the F7 file must be the file of the suite.
func TestQueriesTableIsCurrent(t *testing.T) {
	data, err := os.ReadFile("../../../suite/library-queries.json")
	if err != nil {
		t.Skip("suite/library-queries.json is not there")
	}
	if string(data) != sourceQueriesJSON {
		t.Fatal("queries_gen.go is stale: run GOWORK=off go generate ./... in go/")
	}
}
