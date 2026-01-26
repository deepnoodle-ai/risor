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

func TestStructuredError_IsFatal(t *testing.T) {
	// Save and restore the global setting
	originalSetting := typeErrorsAreFatal
	defer func() { typeErrorsAreFatal = originalSetting }()

	tests := []struct {
		name             string
		kind             ErrorKind
		typeFatalSetting bool
		expected         bool
	}{
		{"type error when fatal=true", ErrType, true, true},
		{"type error when fatal=false", ErrType, false, false},
		{"runtime error always fatal", ErrRuntime, false, true},
		{"name error always fatal", ErrName, false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			typeErrorsAreFatal = tt.typeFatalSetting
			err := &StructuredError{Kind: tt.kind, Message: "test"}
			assert.Equal(t, err.IsFatal(), tt.expected)
		})
	}
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
	assert.True(t, err.IsFatal())

	// Test Unwrap
	underlying := err.Unwrap()
	assert.NotNil(t, underlying)
}

func TestArgsError(t *testing.T) {
	err := ArgsErrorf("expected %d args, got %d", 2, 3)

	assert.Contains(t, err.Error(), "expected 2 args, got 3")
	assert.True(t, err.IsFatal())
}

func TestTypeError(t *testing.T) {
	// Save and restore the global setting
	originalSetting := typeErrorsAreFatal
	defer func() { typeErrorsAreFatal = originalSetting }()

	typeErrorsAreFatal = false
	err := TypeErrorf("cannot compare %s and %s", "int", "string")

	assert.Contains(t, err.Error(), "cannot compare int and string")
	assert.False(t, err.IsFatal())

	typeErrorsAreFatal = true
	err2 := TypeErrorf("test")
	assert.True(t, err2.IsFatal())
}

func TestAreTypeErrorsFatal(t *testing.T) {
	originalSetting := typeErrorsAreFatal
	defer func() { typeErrorsAreFatal = originalSetting }()

	SetTypeErrorsAreFatal(true)
	assert.True(t, AreTypeErrorsFatal())

	SetTypeErrorsAreFatal(false)
	assert.False(t, AreTypeErrorsFatal())
}
