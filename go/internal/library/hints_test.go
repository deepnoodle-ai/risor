package library

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/deepnoodle-ai/wonton/assert"
)

// F19 3.3: a fix is at most 64 code points, and a row never hides an entry.
func TestForeignRows(t *testing.T) {
	assert.Equal(t, len(foreign), 21)
	for _, r := range foreign {
		assert.True(t, utf8.RuneCountInString(r.Fix) <= 64, r.Written)
		switch r.Place {
		case "name", "function":
			for _, e := range AllEntries {
				assert.True(t, e.Name != r.Written, r.Written)
			}
		case "name-field":
			assert.True(t, strings.Contains(r.Written, "."), r.Written)
		case "method":
			assert.True(t, len(r.Kinds) > 0, r.Written)
			for _, k := range r.Kinds {
				for _, e := range AllEntries {
					assert.False(t, e.IsMethod() && e.Name == r.Written && KindWord(e.Kind) == k, r.Written)
				}
			}
		default:
			t.Fatalf("%s: place %q", r.Written, r.Place)
		}
	}
}
