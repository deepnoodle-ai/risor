package token

import (
	"strings"
	"testing"

	"github.com/deepnoodle-ai/wonton/assert"
)

// Test looking up values succeeds, then fails
func TestLookup(t *testing.T) {
	for key, val := range keywords {

		// Obviously this will pass.
		if LookupIdentifier(string(key)) != val {
			t.Errorf("Lookup of %s failed", key)
		}

		// Once the keywords are uppercase they'll no longer
		// match - so we find them as identifiers.
		if LookupIdentifier(strings.ToUpper(string(key))) != IDENT {
			t.Errorf("Lookup of %s failed", key)
		}
	}
}

func TestPosition(t *testing.T) {
	tok := Token{
		Type:    IDENT,
		Literal: "foo",
		StartPosition: Position{
			Line:   2,
			Column: 0,
		},
	}
	// Switches to 1-indexed
	assert.Equal(t, tok.StartPosition.LineNumber(), 3)
	assert.Equal(t, tok.StartPosition.ColumnNumber(), 1)
}
