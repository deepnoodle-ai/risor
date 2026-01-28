package risor

import (
	"context"
	"strings"
	"testing"

	"github.com/deepnoodle-ai/wonton/assert"
)

// =============================================================================
// ISSUE 1: Run() with nil code should return error, not panic
// =============================================================================

func TestRunWithNilCode(t *testing.T) {
	ctx := context.Background()

	// Running with nil code should return ErrNilCode
	_, err := Run(ctx, nil)
	assert.NotNil(t, err)
	assert.ErrorIs(t, err, ErrNilCode)
}

// =============================================================================
// ISSUE 2: Documentation consistency - modules docs should match Builtins()
// =============================================================================

func TestDocumentedModulesMatchBuiltins(t *testing.T) {
	// Verify that all modules documented in Docs() are actually available
	docs := Docs(DocsCategory("modules"))
	data := docs.Data().(map[string]any)
	modules := data["modules"].([]docsModuleInfo)

	env := Builtins()

	for _, mod := range modules {
		t.Run(mod.Name, func(t *testing.T) {
			_, exists := env[mod.Name]
			assert.True(t, exists, "documented module %q should be available in Builtins()", mod.Name)
		})
	}
}

// =============================================================================
// Edge case tests for empty/whitespace input
// =============================================================================

func TestEvalWithEmptySource(t *testing.T) {
	ctx := context.Background()

	// Empty source should return nil with no error
	result, err := Eval(ctx, "")
	assert.Nil(t, err)
	assert.Nil(t, result)
}

func TestEvalWithWhitespaceOnly(t *testing.T) {
	ctx := context.Background()

	result, err := Eval(ctx, "   \n\t  ")
	assert.Nil(t, err)
	assert.Nil(t, result)
}

// =============================================================================
// Edge cases with resource limits
// =============================================================================

func TestNegativeResourceLimits(t *testing.T) {
	ctx := context.Background()

	t.Run("negative maxSteps treated as unlimited", func(t *testing.T) {
		result, err := Eval(ctx, "1 + 1", WithMaxSteps(-100))
		assert.Nil(t, err)
		assert.Equal(t, result, int64(2))
	})

	t.Run("negative maxStackDepth treated as default", func(t *testing.T) {
		result, err := Eval(ctx, "1 + 1", WithMaxStackDepth(-100))
		assert.Nil(t, err)
		assert.Equal(t, result, int64(2))
	})

	t.Run("negative timeout treated as no timeout", func(t *testing.T) {
		result, err := Eval(ctx, "1 + 1", WithTimeout(-1))
		assert.Nil(t, err)
		assert.Equal(t, result, int64(2))
	})
}

func TestZeroResourceLimits(t *testing.T) {
	ctx := context.Background()

	t.Run("zero maxSteps means unlimited", func(t *testing.T) {
		result, err := Eval(ctx, "1 + 1", WithMaxSteps(0))
		assert.Nil(t, err)
		assert.Equal(t, result, int64(2))
	})

	t.Run("zero maxStackDepth uses default", func(t *testing.T) {
		result, err := Eval(ctx, "1 + 1", WithMaxStackDepth(0))
		assert.Nil(t, err)
		assert.Equal(t, result, int64(2))
	})

	t.Run("zero timeout means no timeout", func(t *testing.T) {
		result, err := Eval(ctx, "1 + 1", WithTimeout(0))
		assert.Nil(t, err)
		assert.Equal(t, result, int64(2))
	})
}

// =============================================================================
// Edge cases with nil/empty options
// =============================================================================

func TestWithEnvNilMap(t *testing.T) {
	ctx := context.Background()

	// Passing nil to WithEnv should not panic
	result, err := Eval(ctx, "1 + 1", WithEnv(nil))
	assert.Nil(t, err)
	assert.Equal(t, result, int64(2))
}

func TestNilOptionIsIgnored(t *testing.T) {
	ctx := context.Background()

	// Nil options should be silently ignored
	result, err := Eval(ctx, "1 + 1", nil, WithEnv(map[string]any{}), nil)
	assert.Nil(t, err)
	assert.Equal(t, result, int64(2))
}

// =============================================================================
// WithEnv behavior tests
// =============================================================================

func TestWithEnvDuplicateKeysBehavior(t *testing.T) {
	ctx := context.Background()

	// Last value should win (documented behavior)
	result, err := Eval(ctx, "x",
		WithEnv(map[string]any{"x": int64(1)}),
		WithEnv(map[string]any{"x": int64(2)}),
		WithEnv(map[string]any{"x": int64(3)}),
	)
	assert.Nil(t, err)
	assert.Equal(t, result, int64(3))
}

// =============================================================================
// Global validation tests
// =============================================================================

func TestGlobalValidationEmptyEnvAtRuntime(t *testing.T) {
	ctx := context.Background()

	// Compile with env
	code, err := Compile(ctx, "x + y", WithEnv(map[string]any{
		"x": int64(1),
		"y": int64(2),
	}))
	assert.Nil(t, err)

	// Run with empty env should fail with clear error
	_, err = Run(ctx, code)
	assert.NotNil(t, err)
	assert.True(t, strings.Contains(err.Error(), "missing required globals"))
}

// =============================================================================
// Context cancellation tests
// =============================================================================

func TestCompileWithCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err := Compile(ctx, "1 + 2")
	assert.NotNil(t, err)
	assert.ErrorIs(t, err, context.Canceled)
}

// =============================================================================
// Result conversion tests
// =============================================================================

func TestResultConversionEdgeCases(t *testing.T) {
	ctx := context.Background()

	t.Run("module returns inspect string", func(t *testing.T) {
		// Modules have no Go equivalent, should return Inspect() string
		result, err := Eval(ctx, "math", WithEnv(Builtins()))
		assert.Nil(t, err)
		s, ok := result.(string)
		assert.True(t, ok, "module should be converted to string")
		assert.True(t, strings.Contains(s, "module"))
	})

	t.Run("closure returns inspect string", func(t *testing.T) {
		result, err := Eval(ctx, "function() { return 1 }")
		assert.Nil(t, err)
		s, ok := result.(string)
		assert.True(t, ok, "closure should be converted to string")
		assert.True(t, strings.Contains(s, "func"))
	})

	t.Run("error object returns go error", func(t *testing.T) {
		// Error objects return the underlying Go error via Interface()
		result, err := Eval(ctx, `error("test")`, WithEnv(Builtins()))
		assert.Nil(t, err)
		goErr, ok := result.(error)
		assert.True(t, ok, "error object should return Go error")
		assert.Equal(t, goErr.Error(), "test")
	})

	t.Run("nil returns nil", func(t *testing.T) {
		result, err := Eval(ctx, "nil")
		assert.Nil(t, err)
		assert.Nil(t, result)
	})

	t.Run("empty list returns empty slice", func(t *testing.T) {
		result, err := Eval(ctx, "[]")
		assert.Nil(t, err)
		list, ok := result.([]any)
		assert.True(t, ok)
		assert.Len(t, list, 0)
	})

	t.Run("empty map returns empty map", func(t *testing.T) {
		result, err := Eval(ctx, "{}")
		assert.Nil(t, err)
		m, ok := result.(map[string]any)
		assert.True(t, ok)
		assert.Len(t, m, 0)
	})
}

// =============================================================================
// Compile/Run separation tests
// =============================================================================

func TestCompileRunSeparation(t *testing.T) {
	ctx := context.Background()

	// Compile once, run multiple times with different envs
	code, err := Compile(ctx, "value * 2", WithEnv(map[string]any{"value": int64(0)}))
	assert.Nil(t, err)

	tests := []struct {
		value    int64
		expected int64
	}{
		{1, 2},
		{5, 10},
		{100, 200},
	}

	for _, tt := range tests {
		result, err := Run(ctx, code, WithEnv(map[string]any{"value": tt.value}))
		assert.Nil(t, err)
		assert.Equal(t, result, tt.expected)
	}
}
