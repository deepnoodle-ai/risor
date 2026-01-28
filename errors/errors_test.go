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

// Tests for ValueError

func TestValueError(t *testing.T) {
	err := ValueErrorf("division by zero")
	assert.Contains(t, err.Error(), "value error: division by zero")

	// Test Unwrap
	underlying := err.Unwrap()
	assert.NotNil(t, underlying)
	assert.Contains(t, underlying.Error(), "division by zero")
}

func TestNewValueError(t *testing.T) {
	cause := EvalErrorf("bad value")
	err := NewValueError(cause)

	assert.Contains(t, err.Error(), "bad value")
	assert.Equal(t, err.Unwrap(), cause)
}

// Tests for IndexError

func TestIndexError(t *testing.T) {
	err := IndexErrorf("list index %d out of range [0, %d)", 5, 3)
	assert.Contains(t, err.Error(), "index error: list index 5 out of range [0, 3)")

	// Test Unwrap
	underlying := err.Unwrap()
	assert.NotNil(t, underlying)
}

func TestNewIndexError(t *testing.T) {
	cause := EvalErrorf("out of bounds")
	err := NewIndexError(cause)

	assert.Contains(t, err.Error(), "out of bounds")
	assert.Equal(t, err.Unwrap(), cause)
}

// Tests for CompileError

func TestCompileError_Error(t *testing.T) {
	tests := []struct {
		name     string
		err      *CompileError
		contains []string
		excludes []string
	}{
		{
			name: "with filename and location",
			err: &CompileError{
				Code:     E2001,
				Message:  "undefined variable 'foo'",
				Filename: "test.risor",
				Line:     10,
				Column:   5,
			},
			contains: []string{
				"compile error:",
				"undefined variable 'foo'",
				"location:",
				"test.risor:10:5",
			},
			// Should NOT have duplicate location info
			excludes: []string{"(line 10, column 5)"},
		},
		{
			name: "without filename",
			err: &CompileError{
				Code:    E2001,
				Message: "undefined variable 'bar'",
				Line:    5,
				Column:  3,
			},
			contains: []string{
				"compile error:",
				"undefined variable 'bar'",
				"location:",
				"5:3",
			},
		},
		{
			name: "without location",
			err: &CompileError{
				Message: "general compile error",
			},
			contains: []string{
				"compile error:",
				"general compile error",
			},
			excludes: []string{"location:"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.err.Error()
			for _, s := range tt.contains {
				assert.Contains(t, result, s)
			}
			for _, s := range tt.excludes {
				assert.False(t, strings.Contains(result, s), "should not contain %q", s)
			}
		})
	}
}

func TestCompileError_FriendlyErrorMessage(t *testing.T) {
	err := &CompileError{
		Code:       E2001,
		Message:    "undefined variable 'foo'",
		Filename:   "test.risor",
		Line:       10,
		Column:     5,
		SourceLine: "let x = foo + 1",
		Suggestions: []Suggestion{
			{Value: "food", Distance: 1},
		},
	}

	result := err.FriendlyErrorMessage()

	// Check formatted output contains key elements
	assert.Contains(t, result, "E2001")
	assert.Contains(t, result, "undefined variable 'foo'")
	assert.Contains(t, result, "test.risor:10:5")
	assert.Contains(t, result, "let x = foo + 1")
	assert.Contains(t, result, "hint:")
	assert.Contains(t, result, "food")
}

func TestCompileError_ToFormatted(t *testing.T) {
	err := &CompileError{
		Code:       E2001,
		Message:    "test message",
		Filename:   "test.risor",
		Line:       10,
		Column:     5,
		EndColumn:  10,
		SourceLine: "some code",
		Note:       "additional note",
		Suggestions: []Suggestion{
			{Value: "suggestion1", Distance: 1},
		},
	}

	formatted := err.ToFormatted()

	assert.Equal(t, formatted.Code, E2001)
	assert.Equal(t, formatted.Kind, "error")
	assert.Equal(t, formatted.Message, "test message")
	assert.Equal(t, formatted.Filename, "test.risor")
	assert.Equal(t, formatted.Line, 10)
	assert.Equal(t, formatted.Column, 5)
	assert.Equal(t, formatted.EndColumn, 10)
	assert.Equal(t, formatted.Note, "additional note")
	assert.Equal(t, len(formatted.SourceLines), 1)
	assert.Equal(t, formatted.SourceLines[0].Text, "some code")
	assert.True(t, formatted.SourceLines[0].IsMain)
	assert.Contains(t, formatted.Hint, "suggestion1")
}

// Tests for CompileErrors

func TestCompileErrors_Error(t *testing.T) {
	tests := []struct {
		name     string
		errors   []*CompileError
		expected string
	}{
		{
			name:     "empty",
			errors:   nil,
			expected: "",
		},
		{
			name: "single error",
			errors: []*CompileError{
				{Message: "first error", Line: 1, Column: 1},
			},
			expected: "compile error: first error",
		},
		{
			name: "multiple errors",
			errors: []*CompileError{
				{Message: "first error", Line: 1, Column: 1},
				{Message: "second error", Line: 2, Column: 1},
				{Message: "third error", Line: 3, Column: 1},
			},
			expected: "(and 2 more errors)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := &CompileErrors{Errors: tt.errors}
			result := errs.Error()
			if tt.expected == "" {
				assert.Equal(t, result, "")
			} else {
				assert.Contains(t, result, tt.expected)
			}
		})
	}
}

func TestCompileErrors_FriendlyErrorMessage(t *testing.T) {
	errs := &CompileErrors{
		Errors: []*CompileError{
			{Message: "first error", Line: 1, Column: 1, SourceLine: "code1"},
			{Message: "second error", Line: 2, Column: 1, SourceLine: "code2"},
		},
	}

	result := errs.FriendlyErrorMessage()

	assert.Contains(t, result, "first error")
	assert.Contains(t, result, "second error")
	assert.Contains(t, result, "found 2 errors")
}

func TestCompileErrors_FriendlyErrorMessage_Empty(t *testing.T) {
	errs := &CompileErrors{}
	result := errs.FriendlyErrorMessage()
	assert.Equal(t, result, "")
}

func TestCompileErrors_Add(t *testing.T) {
	errs := &CompileErrors{}
	assert.Equal(t, errs.Count(), 0)
	assert.False(t, errs.HasErrors())

	errs.Add(&CompileError{Message: "error 1"})
	assert.Equal(t, errs.Count(), 1)
	assert.True(t, errs.HasErrors())

	errs.Add(&CompileError{Message: "error 2"})
	assert.Equal(t, errs.Count(), 2)
}

func TestCompileErrors_ToError(t *testing.T) {
	tests := []struct {
		name       string
		errors     []*CompileError
		expectNil  bool
		expectType string
	}{
		{
			name:      "empty returns nil",
			errors:    nil,
			expectNil: true,
		},
		{
			name: "single error returns CompileError",
			errors: []*CompileError{
				{Message: "only error"},
			},
			expectType: "*errors.CompileError",
		},
		{
			name: "multiple errors returns CompileErrors",
			errors: []*CompileError{
				{Message: "error 1"},
				{Message: "error 2"},
			},
			expectType: "*errors.CompileErrors",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := &CompileErrors{Errors: tt.errors}
			result := errs.ToError()
			if tt.expectNil {
				assert.Nil(t, result)
			} else {
				assert.NotNil(t, result)
				// Check type by examining error message behavior
				if tt.expectType == "*errors.CompileError" {
					_, ok := result.(*CompileError)
					assert.True(t, ok, "expected *CompileError")
				} else {
					_, ok := result.(*CompileErrors)
					assert.True(t, ok, "expected *CompileErrors")
				}
			}
		})
	}
}

// Edge case tests

func TestSuggestSimilar_Unicode(t *testing.T) {
	// Test with Unicode characters
	candidates := []string{"日本語", "日本", "にほんご"}
	suggestions := SuggestSimilar("日本", candidates)

	// Should find "日本語" as close match
	assert.True(t, len(suggestions) >= 1)
}

func TestSuggestSimilar_CaseInsensitive(t *testing.T) {
	candidates := []string{"Print", "PRINTF", "println"}
	suggestions := SuggestSimilar("print", candidates)

	// Should match case-insensitively
	assert.True(t, len(suggestions) >= 1)
}

func TestLevenshteinDistance_Unicode(t *testing.T) {
	// Test Unicode handling
	tests := []struct {
		a, b     string
		expected int
	}{
		{"hello", "hëllo", 1}, // Substitution with accented character
		{"café", "cafe", 1},   // Remove accent
		{"日本", "日本語", 1},      // Japanese characters
	}

	for _, tt := range tests {
		t.Run(tt.a+"_"+tt.b, func(t *testing.T) {
			result := levenshteinDistance(tt.a, tt.b)
			assert.Equal(t, result, tt.expected)
		})
	}
}

func TestFormatter_EdgeCases(t *testing.T) {
	f := NewFormatter(false)

	t.Run("empty message", func(t *testing.T) {
		err := &FormattedError{
			Kind:    "error",
			Message: "",
		}
		result := f.Format(err)
		assert.Contains(t, result, "error:")
	})

	t.Run("zero column with source", func(t *testing.T) {
		err := &FormattedError{
			Kind:    "error",
			Message: "test",
			Line:    1,
			Column:  0, // Zero column
			SourceLines: []SourceLineEntry{
				{Number: 1, Text: "some code", IsMain: true},
			},
		}
		result := f.Format(err)
		// Should not crash and should not show caret when column is 0
		assert.Contains(t, result, "some code")
	})

	t.Run("very long source line", func(t *testing.T) {
		longLine := strings.Repeat("x", 1000)
		err := &FormattedError{
			Kind:    "error",
			Message: "test",
			Line:    1,
			Column:  500,
			SourceLines: []SourceLineEntry{
				{Number: 1, Text: longLine, IsMain: true},
			},
		}
		result := f.Format(err)
		assert.Contains(t, result, longLine)
	})
}

func TestStructuredError_GetStack(t *testing.T) {
	stack := []StackFrame{
		{Function: "func1", Location: SourceLocation{Line: 1, Column: 1}},
		{Function: "func2", Location: SourceLocation{Line: 2, Column: 1}},
	}
	err := NewStructuredError(ErrRuntime, "test", SourceLocation{}, stack)

	result := err.GetStack()
	assert.Equal(t, len(result), 2)
	assert.Equal(t, result[0].Function, "func1")
	assert.Equal(t, result[1].Function, "func2")
}

func TestStructuredError_GetLocation(t *testing.T) {
	loc := SourceLocation{
		Filename:  "test.risor",
		Line:      10,
		Column:    5,
		EndColumn: 15,
		Source:    "let x = 42",
	}
	err := NewStructuredError(ErrType, "test", loc, nil)

	result := err.GetLocation()
	assert.Equal(t, result.Filename, "test.risor")
	assert.Equal(t, result.Line, 10)
	assert.Equal(t, result.Column, 5)
	assert.Equal(t, result.EndColumn, 15)
	assert.Equal(t, result.Source, "let x = 42")
}
