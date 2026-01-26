package bytecode

import (
	"testing"

	"github.com/risor-io/risor/op"
)

func TestMarshalUnmarshalRoundTrip(t *testing.T) {
	// Create a code structure with nested functions
	childCode := NewCode(CodeParams{
		ID:           "child-id",
		Name:         "childFunc",
		Instructions: []op.Code{op.LoadFast, 0, op.ReturnValue},
		Constants:    []any{100},
		Names:        []string{"inner_attr"},
		Source:       "return x",
		Filename:     "test.risor",
		LocalCount:   1,
	})

	childFn := NewFunction(FunctionParams{
		ID:         "fn-child",
		Name:       "childFunc",
		Parameters: []string{"x"},
		Code:       childCode,
	})

	rootCode := NewCode(CodeParams{
		ID:           "root-id",
		Name:         "main",
		Instructions: []op.Code{op.LoadConst, 0, op.Call, 1, op.ReturnValue},
		Constants:    []any{childFn, 42},
		Names:        []string{"outer_attr"},
		Source:       "childFunc(42)",
		Filename:     "test.risor",
		GlobalNames:  []string{"childFunc"},
		GlobalCount:  1,
		LocalCount:   0,
		Children:     []*Code{childCode},
	})

	// Marshal
	data, err := Marshal(rootCode)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	// Unmarshal
	restored, err := Unmarshal(data)
	if err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	// Verify root code
	if restored.ID() != "root-id" {
		t.Errorf("expected root ID 'root-id', got %v", restored.ID())
	}
	if restored.Name() != "main" {
		t.Errorf("expected root name 'main', got %v", restored.Name())
	}
	if restored.InstructionCount() != 5 {
		t.Errorf("expected 5 instructions, got %v", restored.InstructionCount())
	}
	if restored.Filename() != "test.risor" {
		t.Errorf("expected filename 'test.risor', got %v", restored.Filename())
	}

	// Verify the function constant was restored
	if restored.ConstantCount() != 2 {
		t.Errorf("expected 2 constants, got %v", restored.ConstantCount())
	}

	restoredFn, ok := restored.ConstantAt(0).(*Function)
	if !ok {
		t.Fatalf("expected constant 0 to be *Function, got %T", restored.ConstantAt(0))
	}
	if restoredFn.Name() != "childFunc" {
		t.Errorf("expected function name 'childFunc', got %v", restoredFn.Name())
	}
	if restoredFn.ParameterCount() != 1 {
		t.Errorf("expected 1 parameter, got %v", restoredFn.ParameterCount())
	}

	// Verify function's code was linked
	fnCode := restoredFn.Code()
	if fnCode == nil {
		t.Fatal("expected function to have code")
	}
	if fnCode.ID() != "child-id" {
		t.Errorf("expected child code ID 'child-id', got %v", fnCode.ID())
	}
}

func TestMarshalUnmarshalWithDefaults(t *testing.T) {
	fnCode := NewCode(CodeParams{
		ID:           "fn-code",
		Instructions: []op.Code{op.ReturnValue},
	})

	fn := NewFunction(FunctionParams{
		ID:         "fn-id",
		Name:       "withDefaults",
		Parameters: []string{"a", "b", "c"},
		Defaults:   []any{nil, 10, "hello"},
		RestParam:  "rest",
		Code:       fnCode,
	})

	code := NewCode(CodeParams{
		ID:        "root",
		Constants: []any{fn},
		Children:  []*Code{fnCode},
	})

	// Round-trip
	data, err := Marshal(code)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	restored, err := Unmarshal(data)
	if err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	restoredFn := restored.ConstantAt(0).(*Function)

	// Verify defaults
	if restoredFn.DefaultCount() != 3 {
		t.Errorf("expected 3 defaults, got %v", restoredFn.DefaultCount())
	}
	if restoredFn.Default(0) != nil {
		t.Errorf("expected default 0 to be nil, got %v", restoredFn.Default(0))
	}
	if restoredFn.Default(1) != int64(10) {
		t.Errorf("expected default 1 to be 10, got %v (%T)", restoredFn.Default(1), restoredFn.Default(1))
	}
	if restoredFn.Default(2) != "hello" {
		t.Errorf("expected default 2 to be 'hello', got %v", restoredFn.Default(2))
	}

	// Verify rest param
	if restoredFn.RestParam() != "rest" {
		t.Errorf("expected rest param 'rest', got %v", restoredFn.RestParam())
	}
}

func TestMarshalUnmarshalExceptionHandlers(t *testing.T) {
	code := NewCode(CodeParams{
		ID:           "test",
		Instructions: []op.Code{op.Nop, op.Nop, op.Nop},
		ExceptionHandlers: []ExceptionHandler{
			{TryStart: 0, TryEnd: 5, CatchStart: 6, FinallyStart: 10, CatchVarIdx: 0},
			{TryStart: 20, TryEnd: 30, CatchStart: 31, FinallyStart: 0, CatchVarIdx: -1},
		},
	})

	data, err := Marshal(code)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	restored, err := Unmarshal(data)
	if err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if restored.ExceptionHandlerCount() != 2 {
		t.Errorf("expected 2 exception handlers, got %v", restored.ExceptionHandlerCount())
	}

	h := restored.ExceptionHandlerAt(0)
	if h.TryStart != 0 || h.TryEnd != 5 || h.CatchStart != 6 || h.FinallyStart != 10 || h.CatchVarIdx != 0 {
		t.Errorf("handler 0 mismatch: %+v", h)
	}

	h = restored.ExceptionHandlerAt(1)
	if h.TryStart != 20 || h.TryEnd != 30 || h.CatchStart != 31 || h.FinallyStart != 0 || h.CatchVarIdx != -1 {
		t.Errorf("handler 1 mismatch: %+v", h)
	}
}

func TestMarshalUnmarshalConstantTypes(t *testing.T) {
	code := NewCode(CodeParams{
		ID: "test",
		Constants: []any{
			nil,
			true,
			false,
			int64(42),
			3.14,
			"hello",
		},
	})

	data, err := Marshal(code)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	restored, err := Unmarshal(data)
	if err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if restored.ConstantCount() != 6 {
		t.Fatalf("expected 6 constants, got %v", restored.ConstantCount())
	}

	// Verify each constant
	if restored.ConstantAt(0) != nil {
		t.Errorf("expected constant 0 to be nil, got %v", restored.ConstantAt(0))
	}
	if restored.ConstantAt(1) != true {
		t.Errorf("expected constant 1 to be true, got %v", restored.ConstantAt(1))
	}
	if restored.ConstantAt(2) != false {
		t.Errorf("expected constant 2 to be false, got %v", restored.ConstantAt(2))
	}
	if restored.ConstantAt(3) != int64(42) {
		t.Errorf("expected constant 3 to be 42, got %v", restored.ConstantAt(3))
	}
	if restored.ConstantAt(4) != 3.14 {
		t.Errorf("expected constant 4 to be 3.14, got %v", restored.ConstantAt(4))
	}
	if restored.ConstantAt(5) != "hello" {
		t.Errorf("expected constant 5 to be 'hello', got %v", restored.ConstantAt(5))
	}
}
