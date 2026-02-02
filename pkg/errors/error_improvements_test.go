package errors

import (
	"strings"
	"testing"

	"github.com/deepnoodle-ai/wonton/assert"
)

// TestEndColumn verifies that EndColumn is properly used for multi-character underlines
func TestEndColumn_MultiCharacterUnderlines(t *testing.T) {
	tests := []struct {
		name          string
		loc           SourceLocation
		expectedLen   int
		expectedCaret string
	}{
		{
			name: "single character when EndColumn not set",
			loc: SourceLocation{
				Line:   1,
				Column: 5,
				Source: "let x = 42",
			},
			expectedLen:   1,
			expectedCaret: "^",
		},
		{
			name: "single character when EndColumn equals Column",
			loc: SourceLocation{
				Line:      1,
				Column:    5,
				EndColumn: 5,
				Source:    "let x = 42",
			},
			expectedLen:   1,
			expectedCaret: "^",
		},
		{
			name: "multi-character underline (3 chars)",
			loc: SourceLocation{
				Line:      1,
				Column:    5,
				EndColumn: 8, // Exclusive: 5 + 3 = 8
				Source:    "let foo = 42",
			},
			expectedLen:   3,
			expectedCaret: "^^^",
		},
		{
			name: "multi-character underline (10 chars)",
			loc: SourceLocation{
				Line:      1,
				Column:    5,
				EndColumn: 15, // Exclusive: 5 + 10 = 15
				Source:    "let verylongname = 42",
			},
			expectedLen:   10,
			expectedCaret: "^^^^^^^^^^",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := &StructuredError{
				Kind:     ErrType,
				Message:  "test error",
				Location: tt.loc,
			}

			msg := err.FriendlyErrorMessage()
			assert.Contains(t, msg, tt.expectedCaret)

			// Count actual carets in the line
			lines := strings.Split(msg, "\n")
			var caretLine string
			for _, line := range lines {
				if strings.Contains(line, "^") {
					caretLine = line
					break
				}
			}
			if tt.loc.Column > 0 {
				caretCount := strings.Count(caretLine, "^")
				assert.Equal(t, caretCount, tt.expectedLen)
			}
		})
	}
}

// TestSourceLocationWithEndColumn verifies EndColumn field works correctly
func TestSourceLocationWithEndColumn(t *testing.T) {
	loc := SourceLocation{
		Filename:  "test.risor",
		Line:      10,
		Column:    5,
		EndColumn: 15,
		Source:    "let x = someLongIdentifier",
	}

	// String representation should still use start position
	assert.Equal(t, loc.String(), "test.risor:10:5")
	assert.False(t, loc.IsZero())

	// EndColumn should be accessible
	assert.Equal(t, loc.EndColumn, 15)
}

// TestFormattedErrorIncludesEndColumn verifies ToFormatted includes EndColumn
func TestFormattedErrorIncludesEndColumn(t *testing.T) {
	loc := SourceLocation{
		Filename:  "test.risor",
		Line:      5,
		Column:    10,
		EndColumn: 20,
		Source:    "let x = someVariable",
	}

	err := NewStructuredError(ErrType, "type mismatch", loc, nil)
	formatted := err.ToFormatted()

	assert.Equal(t, formatted.EndColumn, 20)
}

// TestErrorFormattingWithSourceAndComments verifies source lines are preserved
func TestErrorFormattingWithSourceAndComments(t *testing.T) {
	// When we have source with a comment, the error should show the full line
	loc := SourceLocation{
		Line:   3,
		Column: 10,
		Source: "let x = 42  // this is a comment",
	}

	err := &StructuredError{
		Kind:     ErrType,
		Message:  "test error",
		Location: loc,
	}

	msg := err.FriendlyErrorMessage()

	// Should contain the full source line including comment
	assert.Contains(t, msg, "let x = 42  // this is a comment")
}

// TestStackFrameWithLocation verifies stack frames preserve location info
func TestStackFrameWithLocation(t *testing.T) {
	frame := StackFrame{
		Function: "myFunction",
		Location: SourceLocation{
			Filename:  "test.risor",
			Line:      25,
			Column:    10,
			EndColumn: 20,
		},
	}

	str := frame.String()
	assert.Contains(t, str, "myFunction")
	assert.Contains(t, str, "test.risor:25:10")
}

// TestToFormattedPreservesAllFields verifies ToFormatted preserves all location fields
func TestToFormattedPreservesAllFields(t *testing.T) {
	loc := SourceLocation{
		Filename:  "test.risor",
		Line:      10,
		Column:    5,
		EndColumn: 15,
		Source:    "let x = value",
	}
	stack := []StackFrame{
		{Function: "inner", Location: SourceLocation{Line: 10, Column: 5, EndColumn: 15}},
		{Function: "outer", Location: SourceLocation{Line: 20, Column: 1}},
	}

	err := NewStructuredError(ErrType, "test", loc, stack)
	formatted := err.ToFormatted()

	// Verify all fields are preserved
	assert.Equal(t, formatted.Kind, "type error")
	assert.Equal(t, formatted.Message, "test")
	assert.Equal(t, formatted.Filename, "test.risor")
	assert.Equal(t, formatted.Line, 10)
	assert.Equal(t, formatted.Column, 5)
	assert.Equal(t, formatted.EndColumn, 15)

	// Verify stack frames
	assert.Equal(t, len(formatted.Stack), 2)
	assert.Equal(t, formatted.Stack[0].Function, "inner")
}

// TestNestedFunctionErrorLocation verifies errors in functions have correct locations
func TestNestedFunctionErrorLocation(t *testing.T) {
	// This simulates an error in a nested function
	innerFrame := StackFrame{
		Function: "innerFunc",
		Location: SourceLocation{
			Filename:  "test.risor",
			Line:      5,
			Column:    20,
			EndColumn: 25,
		},
	}
	outerFrame := StackFrame{
		Function: "outerFunc",
		Location: SourceLocation{
			Filename: "test.risor",
			Line:     10,
			Column:   10,
		},
	}

	err := &StructuredError{
		Kind:    ErrType,
		Message: "cannot add string and int",
		Location: SourceLocation{
			Filename:  "test.risor",
			Line:      5,
			Column:    20,
			EndColumn: 25,
			Source:    "    return x + 42",
		},
		Stack: []StackFrame{innerFrame, outerFrame},
	}

	msg := err.FriendlyErrorMessage()

	// Should show both frames in stack trace
	assert.Contains(t, msg, "innerFunc")
	assert.Contains(t, msg, "outerFunc")
	// Should show the source line
	assert.Contains(t, msg, "return x + 42")
}

// TestZeroEndColumnDefaultsToSingleCaret verifies default behavior
func TestZeroEndColumnDefaultsToSingleCaret(t *testing.T) {
	err := &StructuredError{
		Kind:    ErrType,
		Message: "test",
		Location: SourceLocation{
			Line:      1,
			Column:    5,
			EndColumn: 0, // Not set
			Source:    "let x = 42",
		},
	}

	msg := err.FriendlyErrorMessage()
	lines := strings.Split(msg, "\n")

	var caretLine string
	for _, line := range lines {
		if strings.Contains(line, "^") {
			caretLine = line
			break
		}
	}

	// Should have exactly one caret
	assert.Equal(t, strings.Count(caretLine, "^"), 1)
}

// TestCaretPositionAccuracy verifies caret appears at correct column
func TestCaretPositionAccuracy(t *testing.T) {
	tests := []struct {
		name   string
		column int
		source string
	}{
		{"column 1", 1, "let x = 42"},
		{"column 5", 5, "let x = 42"},
		{"column 10", 10, "let x = 42"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := &StructuredError{
				Kind:    ErrType,
				Message: "test",
				Location: SourceLocation{
					Line:   1,
					Column: tt.column,
					Source: tt.source,
				},
			}

			msg := err.FriendlyErrorMessage()
			lines := strings.Split(msg, "\n")

			// Find the line with the caret
			var caretIdx int
			for _, line := range lines {
				if strings.Contains(line, "^") {
					// Find position of caret after " | " prefix
					idx := strings.Index(line, "^")
					// The " | " prefix is 3 chars
					caretIdx = idx - 3 + 1 // Convert to 1-based
					break
				}
			}

			assert.Equal(t, caretIdx, tt.column)
		})
	}
}
