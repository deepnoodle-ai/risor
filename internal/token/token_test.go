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

func TestPositionAdvance(t *testing.T) {
	pos := Position{
		Char:      10,
		LineStart: 5,
		Line:      1,
		Column:    5,
		File:      "test.risor",
	}

	// Advance by 3 bytes
	advanced := pos.Advance(3)

	// Check that Char and Column are advanced
	assert.Equal(t, advanced.Char, 13)
	assert.Equal(t, advanced.Column, 8)

	// Check that other fields are preserved
	assert.Equal(t, advanced.LineStart, 5)
	assert.Equal(t, advanced.Line, 1)
	assert.Equal(t, advanced.File, "test.risor")
}

func TestPositionAdvanceZero(t *testing.T) {
	pos := Position{
		Char:      10,
		LineStart: 5,
		Line:      1,
		Column:    5,
		File:      "test.risor",
	}

	// Advance by 0 should return equivalent position
	advanced := pos.Advance(0)
	assert.Equal(t, advanced.Char, pos.Char)
	assert.Equal(t, advanced.Column, pos.Column)
}

func TestPositionIsValid(t *testing.T) {
	// Zero position is invalid
	zero := Position{}
	assert.False(t, zero.IsValid())

	// Position with File is valid
	withFile := Position{File: "test.risor"}
	assert.True(t, withFile.IsValid())

	// Position with Line is valid
	withLine := Position{Line: 1}
	assert.True(t, withLine.IsValid())

	// Position with Column is valid
	withColumn := Position{Column: 1}
	assert.True(t, withColumn.IsValid())

	// Position with Char is valid
	withChar := Position{Char: 1}
	assert.True(t, withChar.IsValid())
}

func TestNoPos(t *testing.T) {
	// NoPos should be invalid
	assert.False(t, NoPos.IsValid())

	// NoPos should equal zero Position
	assert.Equal(t, NoPos, Position{})
}

func TestPositionFields(t *testing.T) {
	// Test that Position struct has expected fields and no Value field
	pos := Position{
		Char:      100,
		LineStart: 50,
		Line:      5,
		Column:    10,
		File:      "example.risor",
	}

	assert.Equal(t, pos.Char, 100)
	assert.Equal(t, pos.LineStart, 50)
	assert.Equal(t, pos.Line, 5)
	assert.Equal(t, pos.Column, 10)
	assert.Equal(t, pos.File, "example.risor")

	// Verify 1-indexed accessors
	assert.Equal(t, pos.LineNumber(), 6)    // Line + 1
	assert.Equal(t, pos.ColumnNumber(), 11) // Column + 1
}
