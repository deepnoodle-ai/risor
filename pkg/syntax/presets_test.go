package syntax

import (
	"testing"

	"github.com/deepnoodle-ai/wonton/assert"
)

func TestExpressionOnlyPreset(t *testing.T) {
	tests := []struct {
		source  string
		wantErr bool
	}{
		// Allowed
		{"1 + 2", false},
		{"x * y", false},
		{"foo(x)", false},
		{"obj.attr", false},
		{"arr[0]", false},
		{"true && false", false},
		{`"hello"`, false},

		// Disallowed
		{"let x = 1", true},
		{"x = 1", true},
		{"function foo() { 1 }", true},
		{"x => x + 1", true},
		{"if (true) { 1 }", true},
		{"try { 1 } catch { 2 }", true},
		{"let {a} = obj", true},
		{"[...arr]", true},
		{"x |> foo", true},
	}

	validator := NewSyntaxValidator(ExpressionOnly)

	for _, tt := range tests {
		t.Run(tt.source, func(t *testing.T) {
			program := parse(t, tt.source)
			errs := validator.Validate(program)
			if tt.wantErr {
				assert.True(t, len(errs) > 0, "expected error for: %s", tt.source)
			} else {
				assert.Equal(t, len(errs), 0, "unexpected error for: %s", tt.source)
			}
		})
	}
}

func TestBasicScriptingPreset(t *testing.T) {
	tests := []struct {
		source  string
		wantErr bool
	}{
		// Allowed - most language features
		{"1 + 2", false},
		{"let x = 1", false},
		{"x = 2", false},
		{"if (true) { 1 }", false},
		{"try { 1 } catch { 2 }", false},
		{"let {a} = obj", false},
		{"[...arr]", false},
		{"x |> foo", false},
		{"`hello ${name}`", false},

		// Disallowed - only function definitions and return
		{"function foo() { 1 }", true},
		{"x => x + 1", true},
		{"function foo() { return 1 }", true}, // both function and return
	}

	validator := NewSyntaxValidator(BasicScripting)

	for _, tt := range tests {
		t.Run(tt.source, func(t *testing.T) {
			program := parse(t, tt.source)
			errs := validator.Validate(program)
			if tt.wantErr {
				assert.True(t, len(errs) > 0, "expected error for: %s", tt.source)
			} else {
				assert.Equal(t, len(errs), 0, "unexpected error for: %s", tt.source)
			}
		})
	}
}

func TestFullLanguagePreset(t *testing.T) {
	sources := []string{
		"1 + 2",
		"let x = 1",
		"function foo() { return 1 }",
		"if (true) { 1 }",
		"try { 1 } catch { 2 }",
		"let {a} = obj",
		"[...arr]",
		"x |> foo",
		"`hello ${name}`",
	}

	validator := NewSyntaxValidator(FullLanguage)

	for _, source := range sources {
		t.Run(source, func(t *testing.T) {
			program := parse(t, source)
			errs := validator.Validate(program)
			assert.Equal(t, len(errs), 0, "unexpected error for: %s", source)
		})
	}
}
