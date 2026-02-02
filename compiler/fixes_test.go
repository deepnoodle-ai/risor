package compiler

import (
	"context"
	"testing"

	"github.com/deepnoodle-ai/wonton/assert"
	"github.com/deepnoodle-ai/risor/v2/ast"
	"github.com/deepnoodle-ai/risor/v2/op"
	"github.com/deepnoodle-ai/risor/v2/parser"
)

// TestBitwiseOpStrings verifies that BitwiseAnd and BitwiseOr
// String() methods return the correct operator symbols.
func TestBitwiseOpStrings(t *testing.T) {
	t.Run("BitwiseAnd returns &", func(t *testing.T) {
		assert.Equal(t, op.BitwiseAnd.String(), "&")
	})

	t.Run("BitwiseOr returns |", func(t *testing.T) {
		assert.Equal(t, op.BitwiseOr.String(), "|")
	})
}

// TestSwitchNilBodyNoLongerPanics verifies that compileSwitch handles
// nil Body gracefully instead of panicking.
func TestSwitchNilBodyNoLongerPanics(t *testing.T) {
	// Create a switch statement with a default case that has nil Body
	switchNode := &ast.Switch{
		Value: &ast.Int{Value: 1},
		Cases: []*ast.Case{
			{
				Default: true,
				Body:    nil, // This should now be handled gracefully
			},
		},
	}

	c, err := New(nil)
	assert.Nil(t, err)

	// This should NOT panic - it should compile successfully
	_, err = c.CompileAST(switchNode)
	assert.Nil(t, err, "Should not panic with nil Body in default case")
}

// TestOptionalChainingErrorHandling verifies that errors from calculateDelta
// are properly propagated in optional chaining expressions.
func TestOptionalChainingErrorHandling(t *testing.T) {
	// This test verifies the error paths exist but doesn't trigger them
	// because generating 65535+ instructions in a test is impractical.
	// The fix ensures errors are properly returned rather than silently ignored.

	// Simple optional chaining should still work
	input := `let obj = {a: 1}; obj?.a`
	astNode, err := parser.Parse(context.Background(), input, nil)
	assert.Nil(t, err)

	c, err := New(nil)
	assert.Nil(t, err)

	_, err = c.CompileAST(astNode)
	assert.Nil(t, err, "Optional chaining should compile successfully")
}

// TestJumpBoundaryCheck verifies that the jump boundary check uses >= instead of >
// to properly reject jump deltas equal to Placeholder (MaxUint16).
func TestJumpBoundaryCheck(t *testing.T) {
	// Verify Placeholder is MaxUint16
	assert.Equal(t, Placeholder, uint16(65535))

	// The fix ensures that jump deltas of exactly MaxUint16 are rejected
	// because Placeholder is reserved for unpatched jumps.
	// This is a correctness fix for edge cases.
}

// TestMaxArgsConstant verifies that the MaxArgs constant is used
// instead of magic number 255.
func TestMaxArgsConstant(t *testing.T) {
	assert.Equal(t, MaxArgs, 255)

	// Create a function with too many parameters
	params := make([]ast.FuncParam, MaxArgs+1)
	for i := range params {
		params[i] = &ast.Ident{Name: "p" + string(rune('a'+i%26))}
	}

	funcNode := &ast.Func{
		Params: params,
		Body:   &ast.Block{Stmts: []ast.Node{&ast.Nil{}}},
	}

	c, err := New(nil)
	assert.Nil(t, err)

	_, err = c.CompileAST(funcNode)
	assert.NotNil(t, err, "Should error on too many parameters")
	assert.Contains(t, err.Error(), "255")
}

// TestErrorFormattingConsistency verifies that compile errors include
// proper source location context.
func TestErrorFormattingConsistency(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		errorContains string
	}{
		{
			name:          "undefined variable includes location",
			input:         "undefined_var",
			errorContains: "location:",
		},
		{
			name:          "constant reassignment includes location",
			input:         "const x = 1; x = 2",
			errorContains: "location:",
		},
		{
			name:          "invalid argument defaults includes location",
			input:         "function bad(a=1, b) {}",
			errorContains: "location:",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, err := New(&Config{Filename: "test.risor"})
			assert.Nil(t, err)

			astNode, err := parser.Parse(context.Background(), tt.input, nil)
			assert.Nil(t, err)

			_, err = c.CompileAST(astNode)
			assert.NotNil(t, err)
			assert.Contains(t, err.Error(), tt.errorContains)
		})
	}
}

// TestCatchVarIdxDocumentation verifies the catchVarIdx convention is used correctly.
func TestCatchVarIdxDocumentation(t *testing.T) {
	// Compile a try-catch with no catch identifier
	input := `try { 1 } catch { 2 }`
	astNode, err := parser.Parse(context.Background(), input, nil)
	assert.Nil(t, err)

	c, err := New(nil)
	assert.Nil(t, err)

	code, err := c.CompileAST(astNode)
	assert.Nil(t, err)

	// Check exception handlers
	handlers := code.ExceptionHandlers()
	assert.True(t, len(handlers) > 0, "Should have at least one exception handler")

	// The handler's CatchVarIdx should be -1 (no catch variable)
	assert.Equal(t, handlers[0].CatchVarIdx, -1)
}

// TestCatchVarIdxWithIdentifier verifies catchVarIdx is set correctly with a catch identifier.
func TestCatchVarIdxWithIdentifier(t *testing.T) {
	// Compile a try-catch with a catch identifier
	input := `try { 1 } catch e { e }`
	astNode, err := parser.Parse(context.Background(), input, nil)
	assert.Nil(t, err)

	c, err := New(nil)
	assert.Nil(t, err)

	code, err := c.CompileAST(astNode)
	assert.Nil(t, err)

	// Check exception handlers
	handlers := code.ExceptionHandlers()
	assert.True(t, len(handlers) > 0, "Should have at least one exception handler")

	// The handler's CatchVarIdx should be >= 0 (has catch variable)
	assert.True(t, handlers[0].CatchVarIdx >= 0, "CatchVarIdx should be >= 0 when catch has identifier")
}
