package bytecode

import (
	"testing"

	"github.com/risor-io/risor/op"
)

func TestNewCodeImmutability(t *testing.T) {
	// Create input slices
	instructions := []op.Code{op.LoadConst, 0, op.ReturnValue}
	constants := []any{42, "hello"}
	names := []string{"foo", "bar"}
	locations := []SourceLocation{{Line: 1, Column: 1}, {Line: 1, Column: 5}}
	handlers := []ExceptionHandler{{TryStart: 0, TryEnd: 10}}
	globalNames := []string{"global1"}

	code := NewCode(CodeParams{
		ID:                "test",
		Name:              "test_code",
		Instructions:      instructions,
		Constants:         constants,
		Names:             names,
		Locations:         locations,
		GlobalNames:       globalNames,
		ExceptionHandlers: handlers,
		LocalCount:        2,
		GlobalCount:       1,
	})

	// Modify the original slices
	instructions[0] = op.Nil
	constants[0] = 99
	names[0] = "modified"
	locations[0] = SourceLocation{Line: 999, Column: 999}
	handlers[0] = ExceptionHandler{TryStart: 999}
	globalNames[0] = "modified_global"

	// Verify the code was not affected by the modifications
	if code.InstructionAt(0) != op.LoadConst {
		t.Errorf("expected instruction 0 to be LoadConst, got %v", code.InstructionAt(0))
	}
	if code.ConstantAt(0) != 42 {
		t.Errorf("expected constant 0 to be 42, got %v", code.ConstantAt(0))
	}
	if code.NameAt(0) != "foo" {
		t.Errorf("expected name 0 to be 'foo', got %v", code.NameAt(0))
	}
	if code.LocationAt(0).Line != 1 {
		t.Errorf("expected location 0 line to be 1, got %v", code.LocationAt(0).Line)
	}
	if code.ExceptionHandlerAt(0).TryStart != 0 {
		t.Errorf("expected handler 0 TryStart to be 0, got %v", code.ExceptionHandlerAt(0).TryStart)
	}
	if code.GlobalNameAt(0) != "global1" {
		t.Errorf("expected global name 0 to be 'global1', got %v", code.GlobalNameAt(0))
	}
}

func TestCodeAccessors(t *testing.T) {
	code := NewCode(CodeParams{
		ID:           "test-id",
		Name:         "test_name",
		IsNamed:      true,
		Instructions: []op.Code{op.LoadConst, 0, op.ReturnValue},
		Constants:    []any{42, "hello", true},
		Names:        []string{"attr1", "attr2"},
		Source:       "let x = 42\nreturn x",
		Filename:     "test.risor",
		LocalCount:   5,
		GlobalCount:  2,
		MaxCallArgs:  3,
	})

	// Test basic accessors
	if code.ID() != "test-id" {
		t.Errorf("expected ID 'test-id', got %v", code.ID())
	}
	if code.Name() != "test_name" {
		t.Errorf("expected Name 'test_name', got %v", code.Name())
	}
	if !code.IsNamed() {
		t.Error("expected IsNamed to be true")
	}
	if code.Source() != "let x = 42\nreturn x" {
		t.Errorf("unexpected source: %v", code.Source())
	}
	if code.Filename() != "test.risor" {
		t.Errorf("expected filename 'test.risor', got %v", code.Filename())
	}
	if code.LocalCount() != 5 {
		t.Errorf("expected LocalCount 5, got %v", code.LocalCount())
	}
	if code.GlobalCount() != 2 {
		t.Errorf("expected GlobalCount 2, got %v", code.GlobalCount())
	}
	if code.MaxCallArgs() != 3 {
		t.Errorf("expected MaxCallArgs 3, got %v", code.MaxCallArgs())
	}

	// Test counts
	if code.InstructionCount() != 3 {
		t.Errorf("expected InstructionCount 3, got %v", code.InstructionCount())
	}
	if code.ConstantCount() != 3 {
		t.Errorf("expected ConstantCount 3, got %v", code.ConstantCount())
	}
	if code.NameCount() != 2 {
		t.Errorf("expected NameCount 2, got %v", code.NameCount())
	}
}

func TestCodeWithChildren(t *testing.T) {
	child1 := NewCode(CodeParams{
		ID:   "child1",
		Name: "child1_name",
	})
	child2 := NewCode(CodeParams{
		ID:   "child2",
		Name: "child2_name",
	})

	parent := NewCode(CodeParams{
		ID:       "parent",
		Name:     "parent_name",
		Children: []*Code{child1, child2},
	})

	if parent.ChildCount() != 2 {
		t.Errorf("expected ChildCount 2, got %v", parent.ChildCount())
	}
	if parent.ChildAt(0).ID() != "child1" {
		t.Errorf("expected child 0 ID 'child1', got %v", parent.ChildAt(0).ID())
	}
	if parent.ChildAt(1).ID() != "child2" {
		t.Errorf("expected child 1 ID 'child2', got %v", parent.ChildAt(1).ID())
	}
}

func TestCodeGetSourceLine(t *testing.T) {
	code := NewCode(CodeParams{
		Source: "line one\nline two\nline three",
	})

	tests := []struct {
		lineNum  int
		expected string
	}{
		{1, "line one"},
		{2, "line two"},
		{3, "line three"},
		{0, ""},  // out of range
		{4, ""},  // out of range
		{-1, ""}, // negative
	}

	for _, tt := range tests {
		result := code.GetSourceLine(tt.lineNum)
		if result != tt.expected {
			t.Errorf("GetSourceLine(%d) = %q, expected %q", tt.lineNum, result, tt.expected)
		}
	}
}

func TestCodeStats(t *testing.T) {
	fn := NewFunction(FunctionParams{
		ID:   "fn1",
		Name: "testFunc",
	})

	code := NewCode(CodeParams{
		Instructions: []op.Code{op.LoadConst, 0, op.ReturnValue},
		Constants:    []any{42, fn, "hello"},
		GlobalCount:  5,
		Source:       "test source",
	})

	stats := code.Stats()

	if stats.InstructionCount != 3 {
		t.Errorf("expected InstructionCount 3, got %v", stats.InstructionCount)
	}
	if stats.ConstantCount != 3 {
		t.Errorf("expected ConstantCount 3, got %v", stats.ConstantCount)
	}
	if stats.GlobalCount != 5 {
		t.Errorf("expected GlobalCount 5, got %v", stats.GlobalCount)
	}
	if stats.FunctionCount != 1 {
		t.Errorf("expected FunctionCount 1, got %v", stats.FunctionCount)
	}
	if stats.SourceBytes != 11 {
		t.Errorf("expected SourceBytes 11, got %v", stats.SourceBytes)
	}
}

func TestLocationAt(t *testing.T) {
	code := NewCode(CodeParams{
		Locations: []SourceLocation{
			{Line: 1, Column: 1},
			{Line: 2, Column: 5},
		},
	})

	// Valid indices
	loc := code.LocationAt(0)
	if loc.Line != 1 || loc.Column != 1 {
		t.Errorf("expected {1, 1}, got {%d, %d}", loc.Line, loc.Column)
	}

	loc = code.LocationAt(1)
	if loc.Line != 2 || loc.Column != 5 {
		t.Errorf("expected {2, 5}, got {%d, %d}", loc.Line, loc.Column)
	}

	// Out of range - should return zero value
	loc = code.LocationAt(-1)
	if loc.Line != 0 || loc.Column != 0 {
		t.Errorf("expected zero location for -1, got {%d, %d}", loc.Line, loc.Column)
	}

	loc = code.LocationAt(100)
	if loc.Line != 0 || loc.Column != 0 {
		t.Errorf("expected zero location for 100, got {%d, %d}", loc.Line, loc.Column)
	}
}
