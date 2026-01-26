package token

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
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
	require.Equal(t, 3, tok.StartPosition.LineNumber())
	require.Equal(t, 1, tok.StartPosition.ColumnNumber())
}
