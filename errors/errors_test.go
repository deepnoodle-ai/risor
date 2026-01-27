package errors

import (
	"strings"
	"testing"

	"github.com/deepnoodle-ai/wonton/assert"
)

func TestSourceLocation_String(t *testing.T) {
	tests := []struct {
		name     string
		loc      SourceLocation
		expected string
	}{
		{
			name:     "with filename",
			loc:      SourceLocation{Filename: "main.risor", Line: 10, Column: 5},
			expected: "main.risor:10:5",
		},
		{
			name:     "without filename",
			loc:      SourceLocation{Line: 10, Column: 5},
			expected: "10:5",
		},
		{
			name:     "zero location",
			loc:      SourceLocation{},
			expected: "0:0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.loc.String(), tt.expected)
		})
	}
}

func TestSourceLocation_IsZero(t *testing.T) {
	tests := []struct {
		name     string
		loc      SourceLocation
		expected bool
	}{
		{
			name:     "zero location",
			loc:      SourceLocation{},
			expected: true,
		},
		{
			name:     "with line only",
			loc:      SourceLocation{Line: 1},
			expected: false,
		},
		{
			name:     "with column only",
			loc:      SourceLocation{Column: 1},
			expected: false,
		},
		{
			name:     "with both",
			loc:      SourceLocation{Line: 1, Column: 1},
			expected: false,
		},
		{
			name:     "filename doesn't affect IsZero",
			loc:      SourceLocation{Filename: "test.risor"},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.loc.IsZero(), tt.expected)
		})
	}
}

func TestStackFrame_String(t *testing.T) {
	tests := []struct {
		name     string
		frame    StackFrame
		expected string
	}{
		{
			name: "with function name",
			frame: StackFrame{
				Function: "calculate",
				Location: SourceLocation{Filename: "math.risor", Line: 25, Column: 10},
			},
			expected: "at calculate (math.risor:25:10)",
		},
		{
			name: "without function name",
			frame: StackFrame{
				Location: SourceLocation{Filename: "main.risor", Line: 5, Column: 1},
			},
			expected: "at main.risor:5:1",
		},
		{
			name: "anonymous function",
			frame: StackFrame{
				Function: "",
				Location: SourceLocation{Line: 10, Column: 5},
			},
			expected: "at 10:5",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.frame.String(), tt.expected)
		})
	}
}

func TestFormatStackTrace(t *testing.T) {
	tests := []struct {
		name     string
		frames   []StackFrame
		contains []string
	}{
		{
			name:     "empty stack",
			frames:   nil,
			contains: nil,
		},
		{
			name: "single frame",
			frames: []StackFrame{
				{Function: "main", Location: SourceLocation{Filename: "test.risor", Line: 1, Column: 1}},
			},
			contains: []string{"Stack trace:", "at main (test.risor:1:1)"},
		},
		{
			name: "multiple frames",
			frames: []StackFrame{
				{Function: "inner", Location: SourceLocation{Line: 10, Column: 5}},
				{Function: "outer", Location: SourceLocation{Line: 20, Column: 1}},
			},
			contains: []string{"Stack trace:", "at inner (10:5)", "at outer (20:1)"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatStackTrace(tt.frames)
			if len(tt.contains) == 0 {
				assert.Equal(t, result, "")
			} else {
				for _, s := range tt.contains {
					assert.Contains(t, result, s)
				}
			}
		})
	}
}

func TestErrorKind_String(t *testing.T) {
	tests := []struct {
		kind     ErrorKind
		expected string
	}{
		{ErrSyntax, "syntax error"},
		{ErrType, "type error"},
		{ErrName, "name error"},
		{ErrValue, "value error"},
		{ErrRuntime, "runtime error"},
		{ErrImport, "import error"},
		{ErrorKind(999), "error"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.kind.String(), tt.expected)
		})
	}
}

func TestStructuredError_Error(t *testing.T) {
	tests := []struct {
		name     string
		err      *StructuredError
		expected string
	}{
		{
			name: "with location",
			err: &StructuredError{
				Kind:     ErrType,
				Message:  "cannot add int and string",
				Location: SourceLocation{Line: 5, Column: 10},
			},
			expected: "type error: cannot add int and string (5:10)",
		},
		{
			name: "without location",
			err: &StructuredError{
				Kind:    ErrRuntime,
				Message: "division by zero",
			},
			expected: "runtime error: division by zero",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.err.Error(), tt.expected)
		})
	}
}

func TestStructuredError_FriendlyErrorMessage(t *testing.T) {
	err := &StructuredError{
		Kind:    ErrType,
		Message: "undefined variable \"foo\"",
		Location: SourceLocation{
			Filename: "test.risor",
			Line:     3,
			Column:   5,
			Source:   "let x = foo + 1",
		},
		Stack: []StackFrame{
			{Function: "calculate", Location: SourceLocation{Line: 3, Column: 5}},
			{Function: "main", Location: SourceLocation{Line: 10, Column: 1}},
		},
	}

	result := err.FriendlyErrorMessage()

	// Check all parts are present
	assert.Contains(t, result, "type error: undefined variable \"foo\" (3:5)")
	assert.Contains(t, result, "let x = foo + 1")
	assert.Contains(t, result, "    ^") // caret at column 5
	assert.Contains(t, result, "Stack trace:")
	assert.Contains(t, result, "at calculate (3:5)")
	assert.Contains(t, result, "at main (10:1)")
}

func TestStructuredError_FriendlyErrorMessage_NoSource(t *testing.T) {
	err := &StructuredError{
		Kind:     ErrRuntime,
		Message:  "something went wrong",
		Location: SourceLocation{Line: 5, Column: 10},
	}

	result := err.FriendlyErrorMessage()
	assert.Contains(t, result, "runtime error: something went wrong (5:10)")
	// Should not have source snippet or caret
	assert.False(t, strings.Contains(result, "^"))
}

func TestStructuredError_FriendlyErrorMessage_ZeroLocation(t *testing.T) {
	err := &StructuredError{
		Kind:    ErrRuntime,
		Message: "something went wrong",
	}

	result := err.FriendlyErrorMessage()
	assert.Contains(t, result, "runtime error: something went wrong")
	// Should not have line:column
	assert.False(t, strings.Contains(result, "(0:0)"))
}

func TestStructuredError_Unwrap(t *testing.T) {
	cause := NewEvalError(EvalErrorf("underlying error"))
	err := &StructuredError{
		Kind:    ErrRuntime,
		Message: "wrapper",
		Cause:   cause,
	}

	assert.Equal(t, err.Unwrap(), cause)
}

func TestStructuredError_WithCause(t *testing.T) {
	cause := EvalErrorf("the cause")
	err := NewStructuredError(ErrRuntime, "test", SourceLocation{}, nil)
	err.WithCause(cause)

	assert.Equal(t, err.Cause, cause)
}

func TestNewStructuredError(t *testing.T) {
	loc := SourceLocation{Filename: "test.risor", Line: 5, Column: 3}
	stack := []StackFrame{{Function: "main", Location: loc}}

	err := NewStructuredError(ErrType, "test message", loc, stack)

	assert.Equal(t, err.Kind, ErrType)
	assert.Equal(t, err.Message, "test message")
	assert.Equal(t, err.Location, loc)
	assert.Equal(t, err.Stack, stack)
}

func TestNewStructuredErrorf(t *testing.T) {
	loc := SourceLocation{Line: 10, Column: 1}

	err := NewStructuredErrorf(ErrValue, loc, nil, "invalid value: %d", 42)

	assert.Equal(t, err.Kind, ErrValue)
	assert.Equal(t, err.Message, "invalid value: 42")
	assert.Equal(t, err.Location, loc)
}

func TestEvalError(t *testing.T) {
	err := EvalErrorf("something bad: %s", "details")

	assert.Contains(t, err.Error(), "something bad: details")

	// Test Unwrap
	underlying := err.Unwrap()
	assert.NotNil(t, underlying)
}

func TestArgsError(t *testing.T) {
	err := ArgsErrorf("expected %d args, got %d", 2, 3)

	assert.Contains(t, err.Error(), "expected 2 args, got 3")
}

func TestTypeError(t *testing.T) {
	err := TypeErrorf("cannot compare %s and %s", "int", "string")

	assert.Contains(t, err.Error(), "cannot compare int and string")
}

// Tests for codes.go

func TestErrorCode_Description(t *testing.T) {
	tests := []struct {
		code     ErrorCode
		expected string
	}{
		{E1001, "unexpected token"},
		{E1002, "unterminated string literal"},
		{E2001, "undefined variable"},
		{E2006, "duplicate parameter name"},
		{E3001, "type error"},
		{E3002, "division by zero"},
		{ErrorCode("E9999"), "unknown error"},
	}

	for _, tt := range tests {
		t.Run(string(tt.code), func(t *testing.T) {
			assert.Equal(t, tt.code.Description(), tt.expected)
		})
	}
}

func TestErrorCode_String(t *testing.T) {
	assert.Equal(t, E1001.String(), "E1001")
	assert.Equal(t, E2001.String(), "E2001")
	assert.Equal(t, E3001.String(), "E3001")
}

func TestErrorCode_Category(t *testing.T) {
	tests := []struct {
		code     ErrorCode
		expected string
	}{
		{E1001, "parse"},
		{E1010, "parse"},
		{E2001, "compile"},
		{E2010, "compile"},
		{E3001, "runtime"},
		{E3010, "runtime"},
		{ErrorCode("E"), "unknown"},     // Too short
		{ErrorCode("E0001"), "unknown"}, // Invalid category
		{ErrorCode("E4001"), "unknown"}, // Unknown category
	}

	for _, tt := range tests {
		t.Run(string(tt.code), func(t *testing.T) {
			assert.Equal(t, tt.code.Category(), tt.expected)
		})
	}
}

// Tests for suggest.go

func TestSuggestSimilar(t *testing.T) {
	candidates := []string{"print", "printf", "println", "sprint", "sprintf"}

	tests := []struct {
		name        string
		target      string
		candidates  []string
		wantAtLeast int // Min number of expected suggestions
		wantFirst   string
	}{
		{
			name:        "close match",
			target:      "prin",
			candidates:  candidates,
			wantAtLeast: 1,
			wantFirst:   "print",
		},
		{
			name:        "exact match excluded",
			target:      "print",
			candidates:  candidates,
			wantAtLeast: 2, // printf and sprint at least
			wantFirst:   "printf",
		},
		{
			name:        "no close matches",
			target:      "xyz",
			candidates:  candidates,
			wantAtLeast: 0,
		},
		{
			name:        "empty target",
			target:      "",
			candidates:  candidates,
			wantAtLeast: 0,
		},
		{
			name:        "empty candidates",
			target:      "print",
			candidates:  []string{},
			wantAtLeast: 0,
		},
		{
			name:        "short word threshold",
			target:      "at",
			candidates:  []string{"as", "is", "it", "an"},
			wantAtLeast: 2, // At least "as" and "an" are 1 edit away
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			suggestions := SuggestSimilar(tt.target, tt.candidates)
			assert.True(t, len(suggestions) >= tt.wantAtLeast, "expected at least %d suggestions, got %d", tt.wantAtLeast, len(suggestions))
			if tt.wantAtLeast > 0 && tt.wantFirst != "" {
				assert.Equal(t, suggestions[0].Value, tt.wantFirst)
			}
		})
	}
}

func TestSuggestSimilar_MaxSuggestions(t *testing.T) {
	// Many similar candidates
	candidates := []string{"foo1", "foo2", "foo3", "foo4", "foo5"}
	suggestions := SuggestSimilar("foo", candidates)

	// Should be limited to MaxSuggestions
	assert.True(t, len(suggestions) <= MaxSuggestions)
}

func TestFormatSuggestions(t *testing.T) {
	tests := []struct {
		name        string
		suggestions []Suggestion
		expected    string
	}{
		{
			name:        "empty",
			suggestions: nil,
			expected:    "",
		},
		{
			name:        "single suggestion",
			suggestions: []Suggestion{{Value: "print", Distance: 1}},
			expected:    "Did you mean 'print'?",
		},
		{
			name: "multiple suggestions",
			suggestions: []Suggestion{
				{Value: "print", Distance: 1},
				{Value: "printf", Distance: 2},
			},
			expected: "Did you mean one of: 'print', 'printf'?",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatSuggestions(tt.suggestions)
			assert.Equal(t, result, tt.expected)
		})
	}
}

func TestLevenshteinDistance(t *testing.T) {
	tests := []struct {
		a        string
		b        string
		expected int
	}{
		{"", "", 0},
		{"", "abc", 3},
		{"abc", "", 3},
		{"abc", "abc", 0},
		{"abc", "abd", 1},
		{"abc", "adc", 1},
		{"abc", "abcd", 1},
		{"kitten", "sitting", 3},
	}

	for _, tt := range tests {
		t.Run(tt.a+"_"+tt.b, func(t *testing.T) {
			result := levenshteinDistance(tt.a, tt.b)
			assert.Equal(t, result, tt.expected)
		})
	}
}

// Tests for format.go

func TestNewFormatter(t *testing.T) {
	f := NewFormatter(true)
	assert.True(t, f.UseColor)

	f = NewFormatter(false)
	assert.False(t, f.UseColor)
}

func TestFormatter_Format(t *testing.T) {
	f := NewFormatter(false) // No color for easier testing

	err := &FormattedError{
		Code:     E2001,
		Kind:     "compile error",
		Message:  "undefined variable 'foo'",
		Filename: "test.risor",
		Line:     10,
		Column:   5,
		SourceLines: []SourceLineEntry{
			{Number: 10, Text: "let x = foo + 1", IsMain: true},
		},
	}

	result := f.Format(err)

	// Check key parts are present
	assert.Contains(t, result, "compile error")
	assert.Contains(t, result, "[E2001]")
	assert.Contains(t, result, "undefined variable 'foo'")
	assert.Contains(t, result, "test.risor:10:5")
	assert.Contains(t, result, "let x = foo + 1")
	assert.Contains(t, result, "^") // Caret
}

func TestFormatter_FormatWithHint(t *testing.T) {
	f := NewFormatter(false)

	err := &FormattedError{
		Kind:    "error",
		Message: "undefined variable 'prnt'",
		Line:    5,
		Column:  1,
		Hint:    "Did you mean 'print'?",
	}

	result := f.Format(err)
	assert.Contains(t, result, "hint: Did you mean 'print'?")
}

func TestFormatter_FormatWithNote(t *testing.T) {
	f := NewFormatter(false)

	err := &FormattedError{
		Kind:    "error",
		Message: "type mismatch",
		Line:    5,
		Column:  1,
		Note:    "expected int, got string",
	}

	result := f.Format(err)
	assert.Contains(t, result, "note: expected int, got string")
}

func TestFormatter_FormatWithStack(t *testing.T) {
	f := NewFormatter(false)

	err := &FormattedError{
		Kind:    "runtime error",
		Message: "division by zero",
		Line:    10,
		Column:  5,
		Stack: []StackFrame{
			{Function: "divide", Location: SourceLocation{Line: 10, Column: 5}},
			{Function: "main", Location: SourceLocation{Line: 20, Column: 1}},
		},
	}

	result := f.Format(err)
	assert.Contains(t, result, "stack trace:")
	assert.Contains(t, result, "at divide (10:5)")
	assert.Contains(t, result, "at main (20:1)")
}

func TestFormatter_FormatNoLocation(t *testing.T) {
	f := NewFormatter(false)

	err := &FormattedError{
		Kind:    "error",
		Message: "something went wrong",
	}

	result := f.Format(err)
	assert.Contains(t, result, "something went wrong")
	// Should not have location arrow
	assert.False(t, strings.Contains(result, "-->"))
}

func TestFormatter_FormatMultiple(t *testing.T) {
	f := NewFormatter(false)

	// Empty
	result := f.FormatMultiple(nil)
	assert.Equal(t, result, "")

	// Single error - no numbering
	single := []*FormattedError{{Kind: "error", Message: "test"}}
	result = f.FormatMultiple(single)
	assert.False(t, strings.Contains(result, "[1/1]"))

	// Multiple errors - with numbering
	multiple := []*FormattedError{
		{Kind: "error", Message: "first error"},
		{Kind: "error", Message: "second error"},
	}
	result = f.FormatMultiple(multiple)
	assert.Contains(t, result, "[1/2]")
	assert.Contains(t, result, "[2/2]")
	assert.Contains(t, result, "found 2 errors")
}

func TestFormatter_FormatWithColor(t *testing.T) {
	f := NewFormatter(true) // With color

	err := &FormattedError{
		Code:    E2001,
		Kind:    "error",
		Message: "test error",
		Line:    1,
		Column:  1,
	}

	result := f.Format(err)
	// Just verify it doesn't panic and produces output
	assert.True(t, len(result) > 0)
}

func TestFormatter_FormatMultiCharUnderline(t *testing.T) {
	f := NewFormatter(false)

	err := &FormattedError{
		Kind:      "error",
		Message:   "undefined identifier",
		Line:      5,
		Column:    5,
		EndColumn: 10, // Multi-char underline
		SourceLines: []SourceLineEntry{
			{Number: 5, Text: "let hello = undefined", IsMain: true},
		},
	}

	result := f.Format(err)
	// Should have multiple carets
	assert.Contains(t, result, "^^^^^")
}

func TestFormatter_FormatLargeLineNumber(t *testing.T) {
	f := NewFormatter(false)

	err := &FormattedError{
		Kind:     "error",
		Message:  "test",
		Filename: "test.risor",
		Line:     1000,
		Column:   5,
		SourceLines: []SourceLineEntry{
			{Number: 1000, Text: "some code", IsMain: true},
		},
	}

	result := f.Format(err)
	assert.Contains(t, result, "1000")
}

// Tests for NewEvalError and NewArgsError

func TestNewEvalError(t *testing.T) {
	cause := TypeErrorf("underlying cause")
	err := NewEvalError(cause)

	assert.Contains(t, err.Error(), "underlying cause")
	assert.Equal(t, err.Unwrap(), cause)
}

func TestNewArgsError(t *testing.T) {
	cause := TypeErrorf("bad args")
	err := NewArgsError(cause)

	assert.Contains(t, err.Error(), "bad args")
	assert.Equal(t, err.Unwrap(), cause)
}

func TestNewTypeError(t *testing.T) {
	err := NewTypeError(EvalErrorf("test"))
	assert.Contains(t, err.Error(), "test")
	assert.Equal(t, err.Unwrap().Error(), "test")
}
