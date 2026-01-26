package bytecode

import (
	"strings"
	"testing"
)

func TestNewFunctionImmutability(t *testing.T) {
	// Create input slices
	parameters := []string{"a", "b", "c"}
	defaults := []any{nil, 10, "default"}

	fn := NewFunction(FunctionParams{
		ID:         "test-fn",
		Name:       "testFunc",
		Parameters: parameters,
		Defaults:   defaults,
		RestParam:  "rest",
	})

	// Modify the original slices
	parameters[0] = "modified"
	defaults[1] = 999

	// Verify the function was not affected
	if fn.Parameter(0) != "a" {
		t.Errorf("expected parameter 0 to be 'a', got %v", fn.Parameter(0))
	}
	if fn.Default(1) != 10 {
		t.Errorf("expected default 1 to be 10, got %v", fn.Default(1))
	}
}

func TestFunctionAccessors(t *testing.T) {
	code := NewCode(CodeParams{
		ID:         "code-id",
		LocalCount: 5,
	})

	fn := NewFunction(FunctionParams{
		ID:         "fn-id",
		Name:       "myFunction",
		Parameters: []string{"x", "y", "z"},
		Defaults:   []any{nil, 42},
		RestParam:  "args",
		Code:       code,
	})

	// Test basic accessors
	if fn.ID() != "fn-id" {
		t.Errorf("expected ID 'fn-id', got %v", fn.ID())
	}
	if fn.Name() != "myFunction" {
		t.Errorf("expected Name 'myFunction', got %v", fn.Name())
	}
	if fn.RestParam() != "args" {
		t.Errorf("expected RestParam 'args', got %v", fn.RestParam())
	}
	if !fn.HasRestParam() {
		t.Error("expected HasRestParam to be true")
	}
	if fn.Code() != code {
		t.Error("expected Code to match")
	}

	// Test parameter accessors
	if fn.ParameterCount() != 3 {
		t.Errorf("expected ParameterCount 3, got %v", fn.ParameterCount())
	}
	if fn.Parameter(0) != "x" {
		t.Errorf("expected parameter 0 'x', got %v", fn.Parameter(0))
	}
	if fn.Parameter(1) != "y" {
		t.Errorf("expected parameter 1 'y', got %v", fn.Parameter(1))
	}
	if fn.Parameter(2) != "z" {
		t.Errorf("expected parameter 2 'z', got %v", fn.Parameter(2))
	}

	// Test default accessors
	if fn.DefaultCount() != 2 {
		t.Errorf("expected DefaultCount 2, got %v", fn.DefaultCount())
	}
	if fn.Default(0) != nil {
		t.Errorf("expected default 0 to be nil, got %v", fn.Default(0))
	}
	if fn.Default(1) != 42 {
		t.Errorf("expected default 1 to be 42, got %v", fn.Default(1))
	}

	// Test LocalCount (delegates to Code)
	if fn.LocalCount() != 5 {
		t.Errorf("expected LocalCount 5, got %v", fn.LocalCount())
	}
}

func TestFunctionRequiredArgsCount(t *testing.T) {
	tests := []struct {
		name     string
		params   []string
		defaults []any
		expected int
	}{
		{
			name:     "no defaults",
			params:   []string{"a", "b", "c"},
			defaults: []any{},
			expected: 3,
		},
		{
			name:     "all defaults",
			params:   []string{"a", "b"},
			defaults: []any{1, 2},
			expected: 0,
		},
		{
			name:     "partial defaults with nil",
			params:   []string{"a", "b", "c"},
			defaults: []any{nil, 10, nil}, // a=required, b=10, c=required
			expected: 2,
		},
		{
			name:     "only last param has default",
			params:   []string{"a", "b", "c"},
			defaults: []any{nil, nil, "default"},
			expected: 2,
		},
		{
			name:     "no params",
			params:   []string{},
			defaults: []any{},
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fn := NewFunction(FunctionParams{
				Parameters: tt.params,
				Defaults:   tt.defaults,
			})
			if fn.RequiredArgsCount() != tt.expected {
				t.Errorf("RequiredArgsCount() = %d, expected %d", fn.RequiredArgsCount(), tt.expected)
			}
		})
	}
}

func TestFunctionNoRestParam(t *testing.T) {
	fn := NewFunction(FunctionParams{
		Parameters: []string{"a", "b"},
	})

	if fn.HasRestParam() {
		t.Error("expected HasRestParam to be false")
	}
	if fn.RestParam() != "" {
		t.Errorf("expected RestParam to be empty, got %v", fn.RestParam())
	}
}

func TestFunctionLocalCountNilCode(t *testing.T) {
	fn := NewFunction(FunctionParams{
		Parameters: []string{"a"},
		Code:       nil,
	})

	if fn.LocalCount() != 0 {
		t.Errorf("expected LocalCount 0 for nil code, got %v", fn.LocalCount())
	}
}

func TestFunctionString(t *testing.T) {
	code := NewCode(CodeParams{
		Source: "return x + 1",
	})

	fn := NewFunction(FunctionParams{
		Name:       "add",
		Parameters: []string{"x"},
		Code:       code,
	})

	str := fn.String()

	// Check that the string contains expected parts
	if !strings.Contains(str, "func add") {
		t.Errorf("expected string to contain 'func add', got %v", str)
	}
	if !strings.Contains(str, "(x)") {
		t.Errorf("expected string to contain '(x)', got %v", str)
	}
}

func TestFunctionStringWithDefaults(t *testing.T) {
	fn := NewFunction(FunctionParams{
		Name:       "greet",
		Parameters: []string{"name", "greeting"},
		Defaults:   []any{nil, "Hello"},
	})

	str := fn.String()

	if !strings.Contains(str, "greeting=Hello") {
		t.Errorf("expected string to contain 'greeting=Hello', got %v", str)
	}
}

func TestFunctionAnonymous(t *testing.T) {
	fn := NewFunction(FunctionParams{
		Parameters: []string{"x"},
	})

	str := fn.String()

	// Anonymous function should not have a name after "func"
	if strings.Contains(str, "func ") && !strings.HasPrefix(str, "func(") {
		if !strings.HasPrefix(str, "func ") || str[5] != '(' {
			t.Errorf("unexpected format for anonymous function: %v", str)
		}
	}
}
