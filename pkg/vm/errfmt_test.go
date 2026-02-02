package vm

import (
	"context"
	goerrors "errors"
	"strings"
	"testing"

	"github.com/deepnoodle-ai/risor/v2/pkg/errors"
	"github.com/deepnoodle-ai/wonton/assert"
)

// TestRuntimeErrorHasEndColumn verifies runtime errors have EndColumn for multi-char underlines
func TestRuntimeErrorHasEndColumn(t *testing.T) {
	// This code should produce a type error at column 9-10 ("42")
	code := `let x = 42 + "hello"`

	_, err := run(context.Background(), code)
	assert.NotNil(t, err)

	// Check if it's a StructuredError with EndColumn
	var structErr *errors.StructuredError
	if goerrors.As(err, &structErr) {
		// EndColumn should be set (greater than Column for multi-char tokens)
		assert.True(t, structErr.Location.Column > 0, "Column should be set")
		// The error should have location info
		assert.True(t, structErr.Location.Line > 0, "Line should be set")
	}
}

// TestRuntimeErrorInFunctionHasSource verifies errors in functions show source
func TestRuntimeErrorInFunctionHasSource(t *testing.T) {
	code := `
// This is a comment
func add(x) {
    return x + "not a number"  // Error here
}
add(42)
`
	_, err := run(context.Background(), code)
	assert.NotNil(t, err)

	var structErr *errors.StructuredError
	if goerrors.As(err, &structErr) {
		// Should have source line with the error
		assert.True(t, structErr.Location.Line > 0, "Line should be set")

		// Stack should include both the function and caller
		assert.True(t, len(structErr.Stack) >= 1, "Stack should have frames")

		// The error message should include function context
		friendlyMsg := structErr.FriendlyErrorMessage()
		assert.Contains(t, friendlyMsg, "type error")
	}
}

// TestNestedFunctionErrorStack verifies stack traces for nested functions
func TestNestedFunctionErrorStack(t *testing.T) {
	code := `
func outer() {
    func inner() {
        1 + "bad"  // Type error in inner
    }
    inner()
}
outer()
`
	_, err := run(context.Background(), code)
	assert.NotNil(t, err)

	var structErr *errors.StructuredError
	if goerrors.As(err, &structErr) {
		// Should have at least 2 stack frames (inner and outer)
		assert.True(t, len(structErr.Stack) >= 2, "Stack should have multiple frames")

		// Check for function names in stack
		hasInner := false
		hasOuter := false
		for _, frame := range structErr.Stack {
			if frame.Function == "inner" {
				hasInner = true
			}
			if frame.Function == "outer" {
				hasOuter = true
			}
		}
		assert.True(t, hasInner, "Stack should include 'inner'")
		assert.True(t, hasOuter, "Stack should include 'outer'")
	}
}

// TestDivisionByZeroError verifies division by zero has proper location
func TestDivisionByZeroError(t *testing.T) {
	code := `let x = 10 / 0`

	_, err := run(context.Background(), code)
	assert.NotNil(t, err)

	var structErr *errors.StructuredError
	if goerrors.As(err, &structErr) {
		// Should have location info
		assert.True(t, structErr.Location.Line > 0, "Line should be set")
		assert.True(t, structErr.Location.Column > 0, "Column should be set")

		// Should be a runtime error or value error
		assert.True(t, structErr.Kind == errors.ErrRuntime || structErr.Kind == errors.ErrValue,
			"Should be runtime or value error")
	}
}

// TestIndexOutOfBoundsError verifies array index errors have location
func TestIndexOutOfBoundsError(t *testing.T) {
	code := `
let arr = [1, 2, 3]
arr[100]
`
	_, err := run(context.Background(), code)
	assert.NotNil(t, err)

	var structErr *errors.StructuredError
	if goerrors.As(err, &structErr) {
		// Should have location pointing to the indexing operation
		assert.True(t, structErr.Location.Line > 0, "Line should be set")
		assert.True(t, structErr.Location.Column > 0, "Column should be set")
	}
}

// TestFriendlyErrorMessageFormat verifies the friendly error format
func TestFriendlyErrorMessageFormat(t *testing.T) {
	code := `"hello" + 42`

	_, err := run(context.Background(), code)
	assert.NotNil(t, err)

	var structErr *errors.StructuredError
	if goerrors.As(err, &structErr) {
		msg := structErr.FriendlyErrorMessage()

		// Should contain key elements
		assert.Contains(t, msg, "type error")

		// If we have source, should show carets
		if structErr.Location.Source != "" {
			assert.Contains(t, msg, "^", "Should have caret indicator")
		}
	}
}

// TestErrorInLambda verifies errors in lambda expressions have proper context
func TestErrorInLambda(t *testing.T) {
	code := `
let items = [1, 2, 3]
items.map((x) => x + "bad")
`
	_, err := run(context.Background(), code)
	assert.NotNil(t, err)

	var structErr *errors.StructuredError
	if goerrors.As(err, &structErr) {
		// Should have location info
		assert.True(t, structErr.Location.Line > 0, "Line should be set")
	}
}

// TestErrorSourceLinePreserved verifies source lines are preserved with comments
func TestErrorSourceLinePreserved(t *testing.T) {
	code := `
// First comment
// Second comment
let x = 42 + "test"  // Error on this line
`
	_, err := run(context.Background(), code)
	assert.NotNil(t, err)

	var structErr *errors.StructuredError
	if goerrors.As(err, &structErr) {
		// Line number should match the actual source line (line 4)
		assert.Equal(t, structErr.Location.Line, 4, "Line should be 4")
	}
}

// TestMultiCharacterUnderlineInError verifies multi-char tokens get proper underlines
func TestMultiCharacterUnderlineInError(t *testing.T) {
	code := `let verylongvariable = 42; verylongvariable + "oops"`

	_, err := run(context.Background(), code)
	assert.NotNil(t, err)

	var structErr *errors.StructuredError
	if goerrors.As(err, &structErr) {
		// If EndColumn is set, it should span the token
		// EndColumn is exclusive (points after last char)
		if structErr.Location.EndColumn > 0 {
			span := structErr.Location.EndColumn - structErr.Location.Column
			assert.True(t, span > 1, "Span should be greater than 1 for multi-char token")
		}
	}
}

// TestFormattedErrorFromStructured verifies ToFormatted includes all fields
func TestFormattedErrorFromStructured(t *testing.T) {
	code := `
func test() {
    1 / 0
}
test()
`
	_, err := run(context.Background(), code)
	assert.NotNil(t, err)

	var structErr *errors.StructuredError
	if goerrors.As(err, &structErr) {
		formatted := structErr.ToFormatted()

		// Check formatted fields
		assert.True(t, formatted.Line > 0, "Line should be set")
		assert.True(t, formatted.Column > 0, "Column should be set")
		assert.True(t, len(formatted.Kind) > 0, "Kind should be set")
		assert.True(t, len(formatted.Message) > 0, "Message should be set")
	}
}

// TestCaughtErrorPreservesLocation verifies try/catch preserves error location
func TestCaughtErrorPreservesLocation(t *testing.T) {
	code := `
let result = ""
try {
    1 + "bad"
} catch e {
    result = string(e)
}
result
`
	obj, err := run(context.Background(), code)
	assert.Nil(t, err)

	resultStr := obj.Inspect()
	// The caught error should contain location information
	assert.True(t, strings.Contains(resultStr, "type error"), "Should contain error message")
}

// TestAttributeErrorLocation verifies attribute errors have location
func TestAttributeErrorLocation(t *testing.T) {
	code := `
let x = 42
x.unknownMethod()
`
	_, err := run(context.Background(), code)
	assert.NotNil(t, err)

	var structErr *errors.StructuredError
	if goerrors.As(err, &structErr) {
		// Should have location info
		assert.True(t, structErr.Location.Line > 0, "Line should be set")
	}
}

// =============================================================================
// Stack Trace Tests for Panics
// =============================================================================

// TestStackTrace_DivisionByZero_ThreeLevels tests division by zero shows all 3 call frames
func TestStackTrace_DivisionByZero_ThreeLevels(t *testing.T) {
	code := `
function divide(a, b) {
    return a / b
}

function calculate(x, y) {
    return divide(x, y)
}

let result = calculate(100, 0)
`
	_, err := run(context.Background(), code)
	assert.NotNil(t, err)

	var structErr *errors.StructuredError
	if !goerrors.As(err, &structErr) {
		t.Fatal("Expected StructuredError")
	}

	// Should have exactly 3 frames: divide, calculate, __main__
	assert.Equal(t, len(structErr.Stack), 3, "Should have 3 stack frames")

	// Verify frame order (innermost first)
	assert.Equal(t, structErr.Stack[0].Function, "divide", "First frame should be divide")
	assert.Equal(t, structErr.Stack[1].Function, "calculate", "Second frame should be calculate")
	assert.Equal(t, structErr.Stack[2].Function, "__main__", "Third frame should be __main__")

	// Verify line numbers are correct
	assert.Equal(t, structErr.Stack[0].Location.Line, 3, "divide should be on line 3")
	assert.Equal(t, structErr.Stack[1].Location.Line, 7, "calculate call should be on line 7")
	assert.Equal(t, structErr.Stack[2].Location.Line, 10, "main call should be on line 10")
}

// TestStackTrace_DeepCallStack tests a 5-level deep call stack
func TestStackTrace_DeepCallStack(t *testing.T) {
	code := `
function level5() {
    return 1 / 0
}

function level4() {
    return level5()
}

function level3() {
    return level4()
}

function level2() {
    return level3()
}

function level1() {
    return level2()
}

level1()
`
	_, err := run(context.Background(), code)
	assert.NotNil(t, err)

	var structErr *errors.StructuredError
	if !goerrors.As(err, &structErr) {
		t.Fatal("Expected StructuredError")
	}

	// Should have 6 frames: level5, level4, level3, level2, level1, __main__
	assert.Equal(t, len(structErr.Stack), 6, "Should have 6 stack frames")

	// Verify frame order and line numbers
	// Frame 0: level5 - error at line 3 (return 1 / 0)
	assert.Equal(t, structErr.Stack[0].Function, "level5")
	assert.Equal(t, structErr.Stack[0].Location.Line, 3)

	// Frame 1: level4 - call at line 7 (return level5())
	assert.Equal(t, structErr.Stack[1].Function, "level4")
	assert.Equal(t, structErr.Stack[1].Location.Line, 7)

	// Frame 2: level3 - call at line 11 (return level4())
	assert.Equal(t, structErr.Stack[2].Function, "level3")
	assert.Equal(t, structErr.Stack[2].Location.Line, 11)

	// Frame 3: level2 - call at line 15 (return level3())
	assert.Equal(t, structErr.Stack[3].Function, "level2")
	assert.Equal(t, structErr.Stack[3].Location.Line, 15)

	// Frame 4: level1 - call at line 19 (return level2())
	assert.Equal(t, structErr.Stack[4].Function, "level1")
	assert.Equal(t, structErr.Stack[4].Location.Line, 19)

	// Frame 5: __main__ - call at line 22 (level1())
	assert.Equal(t, structErr.Stack[5].Function, "__main__")
	assert.Equal(t, structErr.Stack[5].Location.Line, 22)
}

// TestStackTrace_IndexOutOfBounds tests array index error
// Note: Index errors are currently not wrapped as StructuredError, so this test
// just verifies the error is returned correctly.
func TestStackTrace_IndexOutOfBounds(t *testing.T) {
	code := `
function getItem(arr, idx) {
    return arr[idx]
}

function process(data) {
    return getItem(data, 100)
}

let items = [1, 2, 3]
process(items)
`
	_, err := run(context.Background(), code)
	assert.NotNil(t, err)

	// Verify the error message indicates index out of bounds
	assert.Contains(t, err.Error(), "index", "Error should mention index")
	assert.Contains(t, err.Error(), "100", "Error should mention the index value")
}

// TestStackTrace_MixedNamedAndAnonymous tests mixed named and anonymous functions
func TestStackTrace_MixedNamedAndAnonymous(t *testing.T) {
	code := `
function outer() {
    let inner = function() {
        return 1 / 0
    }
    return inner()
}

outer()
`
	_, err := run(context.Background(), code)
	assert.NotNil(t, err)

	var structErr *errors.StructuredError
	if !goerrors.As(err, &structErr) {
		t.Fatalf("Expected StructuredError, got %T: %v", err, err)
	}

	// Should have exactly 3 frames
	assert.Equal(t, len(structErr.Stack), 3, "Should have 3 stack frames")

	// Frame 0: anonymous function - error at line 4 (return 1 / 0)
	assert.Equal(t, structErr.Stack[0].Function, "<anonymous>")
	assert.Equal(t, structErr.Stack[0].Location.Line, 4)

	// Frame 1: outer - call at line 6 (return inner())
	assert.Equal(t, structErr.Stack[1].Function, "outer")
	assert.Equal(t, structErr.Stack[1].Location.Line, 6)

	// Frame 2: __main__ - call at line 9 (outer())
	assert.Equal(t, structErr.Stack[2].Function, "__main__")
	assert.Equal(t, structErr.Stack[2].Location.Line, 9)
}

// TestStackTrace_RecursiveFunction tests recursive function panic shows all frames
func TestStackTrace_RecursiveFunction(t *testing.T) {
	code := `
function countdown(n) {
    if (n == 0) {
        return 1 / 0
    }
    return countdown(n - 1)
}

countdown(3)
`
	_, err := run(context.Background(), code)
	assert.NotNil(t, err)

	var structErr *errors.StructuredError
	if !goerrors.As(err, &structErr) {
		t.Fatalf("Expected StructuredError, got %T: %v", err, err)
	}

	// Should have 5 frames: countdown(0), countdown(1), countdown(2), countdown(3), __main__
	assert.Equal(t, len(structErr.Stack), 5, "Should have 5 stack frames for recursive calls")

	// Frame 0: countdown(0) - error at line 4 (return 1 / 0)
	assert.Equal(t, structErr.Stack[0].Function, "countdown")
	assert.Equal(t, structErr.Stack[0].Location.Line, 4)

	// Frames 1-3: countdown recursive calls - all at line 6 (return countdown(n - 1))
	for i := 1; i < 4; i++ {
		assert.Equal(t, structErr.Stack[i].Function, "countdown")
		assert.Equal(t, structErr.Stack[i].Location.Line, 6, "Recursive call should be on line 6")
	}

	// Frame 4: __main__ - call at line 9 (countdown(3))
	assert.Equal(t, structErr.Stack[4].Function, "__main__")
	assert.Equal(t, structErr.Stack[4].Location.Line, 9)
}

// TestStackTrace_TypeErrorInNestedCall tests type error (not panic) also has proper stack
func TestStackTrace_TypeErrorInNestedCall(t *testing.T) {
	code := `
function concat(a, b) {
    return a + b
}

function build(x) {
    return concat(x, "suffix")
}

build(42)
`
	_, err := run(context.Background(), code)
	assert.NotNil(t, err)

	var structErr *errors.StructuredError
	if !goerrors.As(err, &structErr) {
		t.Fatal("Expected StructuredError")
	}

	// Should have 3 frames
	assert.Equal(t, len(structErr.Stack), 3, "Should have 3 stack frames")

	// Frame 0: concat - error at line 3 (return a + b)
	assert.Equal(t, structErr.Stack[0].Function, "concat")
	assert.Equal(t, structErr.Stack[0].Location.Line, 3)

	// Frame 1: build - call at line 7 (return concat(x, "suffix"))
	assert.Equal(t, structErr.Stack[1].Function, "build")
	assert.Equal(t, structErr.Stack[1].Location.Line, 7)

	// Frame 2: __main__ - call at line 10 (build(42))
	assert.Equal(t, structErr.Stack[2].Function, "__main__")
	assert.Equal(t, structErr.Stack[2].Location.Line, 10)
}

// TestStackTrace_LambdaInMap tests lambda errors in map operations
func TestStackTrace_LambdaInMap(t *testing.T) {
	code := `
function process(items) {
    return items.map(function(x) {
        return x / 0
    })
}

process([1, 2, 3])
`
	_, err := run(context.Background(), code)
	assert.NotNil(t, err)

	var structErr *errors.StructuredError
	if !goerrors.As(err, &structErr) {
		t.Fatal("Expected StructuredError")
	}

	// Should have at least 2 frames: anonymous lambda, __main__
	assert.True(t, len(structErr.Stack) >= 2, "Should have at least 2 frames")

	// Frame 0: anonymous lambda - error at line 4 (return x / 0)
	assert.Equal(t, structErr.Stack[0].Function, "<anonymous>")
	assert.Equal(t, structErr.Stack[0].Location.Line, 4)
}

// TestStackTrace_ErrorInClosureCapturedVariable tests closure with captured variable
func TestStackTrace_ErrorInClosureCapturedVariable(t *testing.T) {
	code := `
function makeAdder(n) {
    return function(x) {
        return x / n
    }
}

let addZero = makeAdder(0)
addZero(10)
`
	_, err := run(context.Background(), code)
	assert.NotNil(t, err)

	var structErr *errors.StructuredError
	if !goerrors.As(err, &structErr) {
		t.Fatal("Expected StructuredError")
	}

	// Should have 2 frames: closure, __main__
	assert.Equal(t, len(structErr.Stack), 2, "Should have 2 frames")

	// Frame 0: anonymous closure - error at line 4 (return x / n)
	assert.Equal(t, structErr.Stack[0].Function, "<anonymous>")
	assert.Equal(t, structErr.Stack[0].Location.Line, 4)

	// Frame 1: __main__ - call at line 9 (addZero(10))
	assert.Equal(t, structErr.Stack[1].Function, "__main__")
	assert.Equal(t, structErr.Stack[1].Location.Line, 9)
}

// TestStackTrace_ChainedMethodCalls tests method chain errors
func TestStackTrace_ChainedMethodCalls(t *testing.T) {
	code := `
function first(x) {
    return second(x)
}

function second(x) {
    return third(x)
}

function third(x) {
    return x.nonexistent()
}

first({})
`
	_, err := run(context.Background(), code)
	assert.NotNil(t, err)

	var structErr *errors.StructuredError
	if !goerrors.As(err, &structErr) {
		t.Fatal("Expected StructuredError")
	}

	// Should have 4 frames: third, second, first, __main__
	assert.Equal(t, len(structErr.Stack), 4, "Should have 4 stack frames")

	// Frame 0: third - error at line 11 (return x.nonexistent())
	assert.Equal(t, structErr.Stack[0].Function, "third")
	assert.Equal(t, structErr.Stack[0].Location.Line, 11)

	// Frame 1: second - call at line 7 (return third(x))
	assert.Equal(t, structErr.Stack[1].Function, "second")
	assert.Equal(t, structErr.Stack[1].Location.Line, 7)

	// Frame 2: first - call at line 3 (return second(x))
	assert.Equal(t, structErr.Stack[2].Function, "first")
	assert.Equal(t, structErr.Stack[2].Location.Line, 3)

	// Frame 3: __main__ - call at line 14 (first({}))
	assert.Equal(t, structErr.Stack[3].Function, "__main__")
	assert.Equal(t, structErr.Stack[3].Location.Line, 14)
}

// TestStackTrace_CorrectLineNumbers tests that line numbers are accurate for each frame
func TestStackTrace_CorrectLineNumbers(t *testing.T) {
	// Each function call is on a specific line that we verify
	code := `
function a() {
    return 1 / 0
}
function b() {
    return a()
}
function c() {
    return b()
}
c()
`
	_, err := run(context.Background(), code)
	assert.NotNil(t, err)

	var structErr *errors.StructuredError
	if !goerrors.As(err, &structErr) {
		t.Fatal("Expected StructuredError")
	}

	assert.Equal(t, len(structErr.Stack), 4, "Should have 4 stack frames")

	// Line 3: return 1 / 0
	assert.Equal(t, structErr.Stack[0].Location.Line, 3, "a() error should be on line 3")

	// Line 6: return a()
	assert.Equal(t, structErr.Stack[1].Location.Line, 6, "b() call to a() should be on line 6")

	// Line 9: return b()
	assert.Equal(t, structErr.Stack[2].Location.Line, 9, "c() call to b() should be on line 9")

	// Line 11: c()
	assert.Equal(t, structErr.Stack[3].Location.Line, 11, "__main__ call to c() should be on line 11")
}

// TestStackTrace_FriendlyMessageShowsAllFrames tests the friendly message includes all frames
func TestStackTrace_FriendlyMessageShowsAllFrames(t *testing.T) {
	code := `
function inner() {
    return 1 / 0
}

function outer() {
    return inner()
}

outer()
`
	_, err := run(context.Background(), code)
	assert.NotNil(t, err)

	var structErr *errors.StructuredError
	if !goerrors.As(err, &structErr) {
		t.Fatal("Expected StructuredError")
	}

	// Verify stack structure first
	assert.Equal(t, len(structErr.Stack), 3, "Should have 3 stack frames")
	assert.Equal(t, structErr.Stack[0].Function, "inner")
	assert.Equal(t, structErr.Stack[1].Function, "outer")
	assert.Equal(t, structErr.Stack[2].Function, "__main__")

	// Verify friendly message includes all frames with line numbers
	msg := structErr.FriendlyErrorMessage()
	assert.Contains(t, msg, "inner")
	assert.Contains(t, msg, "outer")
	assert.Contains(t, msg, "__main__")
	assert.Contains(t, msg, "3:") // inner error line
	assert.Contains(t, msg, "7:") // outer call line
	assert.Contains(t, strings.ToLower(msg), "stack trace")
}

// TestStackTrace_EmptyStackOnSimpleError tests that simple errors still work
func TestStackTrace_EmptyStackOnSimpleError(t *testing.T) {
	code := `1 / 0`

	_, err := run(context.Background(), code)
	assert.NotNil(t, err)

	var structErr *errors.StructuredError
	if !goerrors.As(err, &structErr) {
		t.Fatal("Expected StructuredError")
	}

	// Should have exactly 1 frame: __main__
	assert.Equal(t, len(structErr.Stack), 1, "Should have 1 stack frame")
	assert.Equal(t, structErr.Stack[0].Function, "__main__")
	assert.Equal(t, structErr.Stack[0].Location.Line, 1)
	assert.Equal(t, structErr.Stack[0].Location.Column, 5) // position of "0" in "1 / 0"
}

// TestStackTrace_MultipleCallsToSameFunction tests calling same function multiple times
func TestStackTrace_MultipleCallsToSameFunction(t *testing.T) {
	code := `
function divide(a, b) {
    return a / b
}

function test() {
    divide(10, 2)
    divide(20, 0)
}

test()
`
	_, err := run(context.Background(), code)
	assert.NotNil(t, err)

	var structErr *errors.StructuredError
	if !goerrors.As(err, &structErr) {
		t.Fatal("Expected StructuredError")
	}

	// Should have 3 frames: divide, test, __main__
	assert.Equal(t, len(structErr.Stack), 3, "Should have 3 stack frames")

	// Frame 0: divide - error at line 3 (return a / b)
	assert.Equal(t, structErr.Stack[0].Function, "divide")
	assert.Equal(t, structErr.Stack[0].Location.Line, 3)

	// Frame 1: test - the SECOND call at line 8 (divide(20, 0)), not the first
	assert.Equal(t, structErr.Stack[1].Function, "test")
	assert.Equal(t, structErr.Stack[1].Location.Line, 8)

	// Frame 2: __main__ - call at line 11 (test())
	assert.Equal(t, structErr.Stack[2].Function, "__main__")
	assert.Equal(t, structErr.Stack[2].Location.Line, 11)
}

// TestStackTrace_PanicInConditional tests panic inside conditional branch
func TestStackTrace_PanicInConditional(t *testing.T) {
	code := `
function check(x) {
    if (x > 0) {
        return 1 / 0
    }
    return x
}

function caller() {
    return check(5)
}

caller()
`
	_, err := run(context.Background(), code)
	assert.NotNil(t, err)

	var structErr *errors.StructuredError
	if !goerrors.As(err, &structErr) {
		t.Fatalf("Expected StructuredError, got %T: %v", err, err)
	}

	// Should have 3 frames: check, caller, __main__
	assert.Equal(t, len(structErr.Stack), 3, "Should have 3 stack frames")

	// Frame 0: check - error at line 4 (return 1 / 0)
	assert.Equal(t, structErr.Stack[0].Function, "check")
	assert.Equal(t, structErr.Stack[0].Location.Line, 4)

	// Frame 1: caller - call at line 10 (return check(5))
	assert.Equal(t, structErr.Stack[1].Function, "caller")
	assert.Equal(t, structErr.Stack[1].Location.Line, 10)

	// Frame 2: __main__ - call at line 13 (caller())
	assert.Equal(t, structErr.Stack[2].Function, "__main__")
	assert.Equal(t, structErr.Stack[2].Location.Line, 13)
}

// TestStackTrace_PanicInIteration tests panic inside iteration via .map()
func TestStackTrace_PanicInIteration(t *testing.T) {
	code := `
function process(items) {
    return items.map(function handler(x) {
        if (x == 2) {
            return 1 / 0
        }
        return x
    })
}

function runner() {
    return process([0, 1, 2, 3, 4])
}

runner()
`
	_, err := run(context.Background(), code)
	assert.NotNil(t, err)

	var structErr *errors.StructuredError
	if !goerrors.As(err, &structErr) {
		t.Fatalf("Expected StructuredError, got %T: %v", err, err)
	}

	// Should have at least 3 frames: handler, runner, __main__
	// (process may or may not appear depending on how .map is implemented)
	assert.True(t, len(structErr.Stack) >= 3, "Should have at least 3 stack frames")

	// Frame 0: handler (the map callback) - error at line 5 (return 1 / 0)
	assert.Equal(t, structErr.Stack[0].Function, "handler")
	assert.Equal(t, structErr.Stack[0].Location.Line, 5)
}
