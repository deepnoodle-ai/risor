package vm

import (
	"context"
	"testing"

	"github.com/deepnoodle-ai/wonton/assert"
	"github.com/risor-io/risor/object"
)

// TestExceptionCrossingFunctionBoundaries tests that exceptions propagate
// correctly across function call boundaries.
func TestExceptionCrossingFunctionBoundaries(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected object.Object
	}{
		{
			name: "throw in called function caught by caller",
			input: `
			function inner() {
				throw "from inner"
			}
			let result = "uncaught"
			try {
				inner()
			} catch e {
				result = "caught: " + string(e)
			}
			result
			`,
			expected: object.NewString("caught: from inner"),
		},
		{
			name: "throw in deeply nested call chain",
			input: `
			function level3() { throw "deep error" }
			function level2() { level3() }
			function level1() { level2() }
			let result = "uncaught"
			try {
				level1()
			} catch e {
				result = "caught"
			}
			result
			`,
			expected: object.NewString("caught"),
		},
		{
			name: "exception in recursion",
			input: `
			function countdown(n) {
				if (n <= 0) {
					throw "reached zero"
				}
				countdown(n - 1)
			}
			let result = "uncaught"
			try {
				countdown(5)
			} catch e {
				result = "caught: " + string(e)
			}
			result
			`,
			expected: object.NewString("caught: reached zero"),
		},
		{
			name: "inner function has try/catch, outer catches what propagates",
			input: `
			function inner() {
				try {
					throw "inner error"
				} catch e {
					throw "rethrown: " + string(e)
				}
			}
			let result = "uncaught"
			try {
				inner()
			} catch e {
				result = "outer caught: " + string(e)
			}
			result
			`,
			expected: object.NewString("outer caught: rethrown: inner error"),
		},
		{
			name: "inner function catches and does not rethrow",
			input: `
			function inner() {
				try {
					throw "swallowed"
				} catch e {
					return "handled"
				}
			}
			let result = "outer"
			try {
				result = inner()
			} catch e {
				result = "outer caught"
			}
			result
			`,
			expected: object.NewString("handled"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := run(context.Background(), tt.input)
			assert.Nil(t, err, "unexpected error: %v", err)
			assert.Equal(t, result, tt.expected)
		})
	}
}

// TestNestedTryCatch tests deeply nested try/catch blocks.
func TestNestedTryCatch(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected object.Object
	}{
		{
			name: "nested try/catch - inner catches",
			input: `
			let result = []
			try {
				result = result + ["outer try"]
				try {
					result = result + ["inner try"]
					throw "inner error"
				} catch e {
					result = result + ["inner catch"]
				}
				result = result + ["after inner"]
			} catch e {
				result = result + ["outer catch"]
			}
			result
			`,
			expected: object.NewList([]object.Object{
				object.NewString("outer try"),
				object.NewString("inner try"),
				object.NewString("inner catch"),
				object.NewString("after inner"),
			}),
		},
		{
			name: "nested try/catch - inner rethrows to outer",
			input: `
			let result = []
			try {
				result = result + ["outer try"]
				try {
					result = result + ["inner try"]
					throw "original"
				} catch e {
					result = result + ["inner catch"]
					throw "rethrown"
				}
				result = result + ["should not reach"]
			} catch e {
				result = result + ["outer catch: " + string(e)]
			}
			result
			`,
			expected: object.NewList([]object.Object{
				object.NewString("outer try"),
				object.NewString("inner try"),
				object.NewString("inner catch"),
				object.NewString("outer catch: rethrown"),
			}),
		},
		{
			name: "triple nested - middle catches",
			input: `
			let result = []
			try {
				result = result + ["L1"]
				try {
					result = result + ["L2"]
					try {
						result = result + ["L3"]
						throw "deep"
					} catch e {
						result = result + ["L3 catch"]
					}
					result = result + ["L2 after"]
				} catch e {
					result = result + ["L2 catch"]
				}
				result = result + ["L1 after"]
			} catch e {
				result = result + ["L1 catch"]
			}
			result
			`,
			expected: object.NewList([]object.Object{
				object.NewString("L1"),
				object.NewString("L2"),
				object.NewString("L3"),
				object.NewString("L3 catch"),
				object.NewString("L2 after"),
				object.NewString("L1 after"),
			}),
		},
		{
			name: "triple nested - propagates all the way",
			input: `
			let result = []
			try {
				result = result + ["L1"]
				try {
					result = result + ["L2"]
					try {
						result = result + ["L3"]
						throw "deep"
					} finally {
						result = result + ["L3 finally"]
					}
				} finally {
					result = result + ["L2 finally"]
				}
			} catch e {
				result = result + ["L1 catch: " + string(e)]
			}
			result
			`,
			expected: object.NewList([]object.Object{
				object.NewString("L1"),
				object.NewString("L2"),
				object.NewString("L3"),
				object.NewString("L3 finally"),
				object.NewString("L2 finally"),
				object.NewString("L1 catch: deep"),
			}),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := run(context.Background(), tt.input)
			assert.Nil(t, err, "unexpected error: %v", err)
			assert.Equal(t, result, tt.expected)
		})
	}
}

// TestFinallyBehavior tests that finally blocks run in all scenarios.
func TestFinallyBehavior(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected object.Object
	}{
		{
			name: "finally runs on normal completion",
			input: `
			let result = []
			try {
				result = result + ["try"]
			} finally {
				result = result + ["finally"]
			}
			result
			`,
			expected: object.NewList([]object.Object{
				object.NewString("try"),
				object.NewString("finally"),
			}),
		},
		{
			name: "finally runs after catch",
			input: `
			let result = []
			try {
				throw "error"
			} catch e {
				result = result + ["catch"]
			} finally {
				result = result + ["finally"]
			}
			result
			`,
			expected: object.NewList([]object.Object{
				object.NewString("catch"),
				object.NewString("finally"),
			}),
		},
		{
			name: "finally runs even when exception propagates",
			input: `
			let result = []
			try {
				try {
					throw "error"
				} finally {
					result = result + ["inner finally"]
				}
			} catch e {
				result = result + ["outer catch"]
			}
			result
			`,
			expected: object.NewList([]object.Object{
				object.NewString("inner finally"),
				object.NewString("outer catch"),
			}),
		},
		{
			name: "exception in finally replaces original",
			input: `
			let result = "initial"
			try {
				try {
					throw "original"
				} finally {
					throw "from finally"
				}
			} catch e {
				result = string(e)
			}
			result
			`,
			expected: object.NewString("from finally"),
		},
		{
			name: "exception in finally propagates",
			input: `
			let result = "uncaught"
			try {
				try {
					// no exception in try
				} finally {
					throw "finally error"
				}
			} catch e {
				result = "caught: " + string(e)
			}
			result
			`,
			expected: object.NewString("caught: finally error"),
		},
		{
			name: "multiple nested functions with finally - all finally blocks run",
			input: `
			let order = []
			function inner() {
				try {
					throw "inner"
				} finally {
					order = order + ["inner finally"]
				}
			}
			function outer() {
				try {
					inner()
				} finally {
					order = order + ["outer finally"]
				}
			}
			try {
				outer()
			} catch e {
				order = order + ["caught: " + string(e)]
			}
			order
			`,
			expected: object.NewList([]object.Object{
				object.NewString("inner finally"),
				object.NewString("outer finally"),
				object.NewString("caught: inner"),
			}),
		},
		{
			name: "function calls within finally block work correctly",
			input: `
			let order = []
			function helper() {
				order = order + ["helper"]
				return "helper result"
			}
			function test() {
				try {
					return "try"
				} finally {
					let x = helper()
					order = order + ["finally with " + x]
				}
			}
			let result = test()
			order = order + ["result: " + result]
			order
			`,
			expected: object.NewList([]object.Object{
				object.NewString("helper"),
				object.NewString("finally with helper result"),
				object.NewString("result: try"),
			}),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := run(context.Background(), tt.input)
			assert.Nil(t, err, "unexpected error: %v", err)
			assert.Equal(t, result, tt.expected)
		})
	}
}

// TestReturnInTryCatch tests return statements within try/catch/finally blocks.
func TestReturnInTryCatch(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected object.Object
	}{
		{
			name: "return in try block",
			input: `
			function test() {
				try {
					return "from try"
				} catch e {
					return "from catch"
				}
				return "after"
			}
			test()
			`,
			expected: object.NewString("from try"),
		},
		{
			name: "return in catch block",
			input: `
			function test() {
				try {
					throw "error"
				} catch e {
					return "from catch"
				}
				return "after"
			}
			test()
			`,
			expected: object.NewString("from catch"),
		},
		{
			name: "return in finally with no exception",
			input: `
			function test() {
				try {
					let x = "try body"
				} finally {
					return "from finally"
				}
				return "after"
			}
			test()
			`,
			expected: object.NewString("from finally"),
		},
		// Finally blocks run even when returning from try block (Python behavior)
		{
			name: "return in try with finally - finally runs",
			input: `
			let finallyRan = false
			function test() {
				try {
					return "from try"
				} finally {
					finallyRan = true
				}
			}
			let result = test()
			[result, finallyRan]
			`,
			expected: object.NewList([]object.Object{
				object.NewString("from try"),
				object.True, // Finally runs before the return completes
			}),
		},
		// Finally also runs when returning from catch block
		{
			name: "return in catch with finally - finally runs",
			input: `
			let finallyRan = false
			function test() {
				try {
					throw "error"
				} catch e {
					return "from catch"
				} finally {
					finallyRan = true
				}
			}
			let result = test()
			[result, finallyRan]
			`,
			expected: object.NewList([]object.Object{
				object.NewString("from catch"),
				object.True, // Finally runs before the return completes
			}),
		},
		// Return in finally overrides return in try (Python behavior)
		{
			name: "return in finally overrides return in try",
			input: `
			function test() {
				try {
					return "from try"
				} finally {
					return "from finally"
				}
			}
			test()
			`,
			expected: object.NewString("from finally"),
		},
		// Return in finally suppresses pending exception (Python behavior)
		{
			name: "return in finally suppresses exception",
			input: `
			function test() {
				try {
					throw "error"
				} finally {
					return "suppressed"
				}
			}
			test()
			`,
			expected: object.NewString("suppressed"),
		},
		// Throw in finally with pending return - exception propagates
		{
			name: "throw in finally with pending return",
			input: `
			let caught = ""
			function test() {
				try {
					return "from try"
				} finally {
					throw "finally error"
				}
			}
			try {
				test()
			} catch e {
				caught = string(e)
			}
			caught
			`,
			expected: object.NewString("finally error"),
		},
		// Nested try/finally with returns - outermost finally wins
		{
			name: "nested try/finally with returns",
			input: `
			function test() {
				try {
					try {
						return "inner"
					} finally {
						return "inner finally"
					}
				} finally {
					return "outer finally"
				}
			}
			test()
			`,
			expected: object.NewString("outer finally"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := run(context.Background(), tt.input)
			assert.Nil(t, err, "unexpected error: %v", err)
			assert.Equal(t, result, tt.expected)
		})
	}
}

// TestExceptionInCatch tests exception handling when exceptions occur in catch blocks.
func TestExceptionInCatch(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected object.Object
	}{
		{
			name: "throw in catch propagates",
			input: `
			let result = "uncaught"
			try {
				try {
					throw "original"
				} catch e {
					throw "from catch"
				}
			} catch e {
				result = "outer: " + string(e)
			}
			result
			`,
			expected: object.NewString("outer: from catch"),
		},
		{
			name: "runtime error in catch propagates",
			input: `
			let result = "uncaught"
			try {
				try {
					throw "original"
				} catch e {
					let x = nil
					x.foo // This will throw a runtime error
				}
			} catch e {
				result = "caught runtime error"
			}
			result
			`,
			expected: object.NewString("caught runtime error"),
		},
		{
			name: "function call in catch that throws",
			input: `
			function throwIt() {
				throw "from function"
			}
			let result = "uncaught"
			try {
				try {
					throw "original"
				} catch e {
					throwIt()
				}
			} catch e {
				result = "outer: " + string(e)
			}
			result
			`,
			expected: object.NewString("outer: from function"),
		},
		{
			name: "throw in catch with finally - finally runs and exception propagates",
			input: `
			let finallyRan = false
			let caught = ""
			function test() {
				try {
					throw "original"
				} catch e {
					throw "from catch"
				} finally {
					finallyRan = true
				}
			}
			try {
				test()
			} catch e {
				caught = string(e)
			}
			[caught, finallyRan]
			`,
			expected: object.NewList([]object.Object{
				object.NewString("from catch"),
				object.True,
			}),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := run(context.Background(), tt.input)
			assert.Nil(t, err, "unexpected error: %v", err)
			assert.Equal(t, result, tt.expected)
		})
	}
}

// TestExceptionWithClosures tests that closures properly interact with try/catch.
func TestExceptionWithClosures(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected object.Object
	}{
		{
			name: "closure captures exception variable",
			input: `
			let getter = nil
			try {
				throw "captured error"
			} catch e {
				getter = function() { return e }
			}
			string(getter())
			`,
			expected: object.NewString("captured error"),
		},
		{
			name: "closure called from try block throws",
			input: `
			let thrower = function() { throw "from closure" }
			let result = "uncaught"
			try {
				thrower()
			} catch e {
				result = "caught: " + string(e)
			}
			result
			`,
			expected: object.NewString("caught: from closure"),
		},
		{
			name: "nested closures and exceptions",
			input: `
			let outer = function() {
				let inner = function() {
					throw "deep in closures"
				}
				inner()
			}
			let result = "uncaught"
			try {
				outer()
			} catch e {
				result = "caught: " + string(e)
			}
			result
			`,
			expected: object.NewString("caught: deep in closures"),
		},
		{
			name: "exception in closure with own try/catch",
			input: `
			let handler = function() {
				try {
					throw "inner"
				} catch e {
					return "handled: " + string(e)
				}
			}
			let result = handler()
			result
			`,
			expected: object.NewString("handled: inner"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := run(context.Background(), tt.input)
			assert.Nil(t, err, "unexpected error: %v", err)
			assert.Equal(t, result, tt.expected)
		})
	}
}

// TestExceptionInBuiltinCallbacks tests exceptions in callbacks to built-in functions.
func TestExceptionInBuiltinCallbacks(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected object.Object
	}{
		{
			name: "exception in map callback",
			input: `
			let result = "uncaught"
			try {
				[1, 2, 3].map(function(x) {
					if (x == 2) { throw "found 2" }
					return x
				})
			} catch e {
				result = "caught: " + string(e)
			}
			result
			`,
			expected: object.NewString("caught: found 2"),
		},
		{
			name: "exception in filter callback",
			input: `
			let result = "uncaught"
			try {
				[1, 2, 3].filter(function(x) {
					if (x == 2) { throw "filter error" }
					return true
				})
			} catch e {
				result = "caught"
			}
			result
			`,
			expected: object.NewString("caught"),
		},
		{
			name: "exception in each callback",
			input: `
			let result = "uncaught"
			let processed = []
			try {
				[1, 2, 3].each(function(x) {
					processed = processed + [x]
					if (x == 2) { throw "stop at 2" }
				})
			} catch e {
				result = "caught"
			}
			[result, processed]
			`,
			expected: object.NewList([]object.Object{
				object.NewString("caught"),
				object.NewList([]object.Object{
					object.NewInt(1),
					object.NewInt(2),
				}),
			}),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := run(context.Background(), tt.input)
			assert.Nil(t, err, "unexpected error: %v", err)
			assert.Equal(t, result, tt.expected)
		})
	}
}

// TestExceptionInIteration tests try/catch behavior with iteration patterns.
// Note: Risor uses recursion and higher-order functions instead of loops.
func TestExceptionInIteration(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected object.Object
	}{
		{
			name: "try/catch inside each - catch and continue",
			input: `
			let results = []
			[0, 1, 2, 3, 4].each(function(i) {
				try {
					if (i == 2) { throw "skip 2" }
					results = results + [i]
				} catch e {
					results = results + ["caught"]
				}
			})
			results
			`,
			expected: object.NewList([]object.Object{
				object.NewInt(0),
				object.NewInt(1),
				object.NewString("caught"),
				object.NewInt(3),
				object.NewInt(4),
			}),
		},
		{
			name: "exception breaks out of iteration",
			input: `
			let result = []
			try {
				[0, 1, 2, 3, 4].each(function(i) {
					result = result + [i]
					if (i == 2) { throw "stop" }
				})
			} catch e {
				result = result + ["caught"]
			}
			result
			`,
			expected: object.NewList([]object.Object{
				object.NewInt(0),
				object.NewInt(1),
				object.NewInt(2),
				object.NewString("caught"),
			}),
		},
		{
			name: "try/catch in recursive iteration",
			input: `
			function processWithCatch(items, idx, results) {
				if (idx >= len(items)) { return results }
				let item = items[idx]
				try {
					if (item == 2) { throw "skip" }
					results = results + [item]
				} catch e {
					results = results + ["skipped"]
				}
				return processWithCatch(items, idx + 1, results)
			}
			processWithCatch([0, 1, 2, 3, 4], 0, [])
			`,
			expected: object.NewList([]object.Object{
				object.NewInt(0),
				object.NewInt(1),
				object.NewString("skipped"),
				object.NewInt(3),
				object.NewInt(4),
			}),
		},
		{
			name: "iteration over list with exceptions",
			input: `
			let results = []
			["a", "b", "c"].each(function(item) {
				try {
					if (item == "b") { throw "skip b" }
					results = results + [item]
				} catch e {
					results = results + ["caught"]
				}
			})
			results
			`,
			expected: object.NewList([]object.Object{
				object.NewString("a"),
				object.NewString("caught"),
				object.NewString("c"),
			}),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := run(context.Background(), tt.input)
			assert.Nil(t, err, "unexpected error: %v", err)
			assert.Equal(t, result, tt.expected)
		})
	}
}

// TestRuntimeErrorsAreCaught tests that various runtime errors can be caught.
func TestRuntimeErrorsAreCaught(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected object.Object
	}{
		{
			name: "nil attribute access",
			input: `
			let result = "uncaught"
			try {
				let x = nil
				x.foo
			} catch e {
				result = "caught nil error"
			}
			result
			`,
			expected: object.NewString("caught nil error"),
		},
		{
			name: "index out of bounds",
			input: `
			let result = "uncaught"
			try {
				let arr = [1, 2, 3]
				arr[10]
			} catch e {
				result = "caught index error"
			}
			result
			`,
			expected: object.NewString("caught index error"),
		},
		{
			name: "type error in operation",
			input: `
			let result = "uncaught"
			try {
				let x = "hello" - 5
			} catch e {
				result = "caught type error"
			}
			result
			`,
			expected: object.NewString("caught type error"),
		},
		{
			name: "division by zero",
			input: `
			let result = "uncaught"
			try {
				let x = 10 / 0
			} catch e {
				result = "caught div zero"
			}
			result
			`,
			expected: object.NewString("caught div zero"),
		},
		{
			name: "modulo by zero",
			input: `
			let result = "uncaught"
			try {
				let x = 10 % 0
			} catch e {
				result = "caught mod zero"
			}
			result
			`,
			expected: object.NewString("caught mod zero"),
		},
		{
			name: "calling non-callable",
			input: `
			let result = "uncaught"
			try {
				let x = 5
				x()
			} catch e {
				result = "caught call error"
			}
			result
			`,
			expected: object.NewString("caught call error"),
		},
		{
			name: "missing map key with subscript",
			input: `
			let result = "uncaught"
			try {
				let m = {a: 1}
				let x = m["nonexistent"]
				result = "got value"
			} catch e {
				result = "caught key error"
			}
			result
			`,
			// Map access throws a key error for missing keys (strict mode)
			expected: object.NewString("caught key error"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := run(context.Background(), tt.input)
			assert.Nil(t, err, "unexpected error: %v", err)
			assert.Equal(t, result, tt.expected)
		})
	}
}

// TestDocumentedRuntimeErrors tests the exact runtime error examples from docs/exceptions.md
// to ensure the documentation is accurate.
func TestDocumentedRuntimeErrors(t *testing.T) {
	tests := []struct {
		name            string
		input           string
		expectedKind    string
		expectedMsgPart string
	}{
		// Type errors
		{
			name:            "type error: 1 + string",
			input:           `1 + "foo"`,
			expectedKind:    "type error",
			expectedMsgPart: "unsupported",
		},
		{
			name:            "type error: string - int",
			input:           `"hello" - 5`,
			expectedKind:    "type error",
			expectedMsgPart: "unsupported",
		},
		// Value errors
		{
			name:            "value error: division by zero",
			input:           `1 / 0`,
			expectedKind:    "value error",
			expectedMsgPart: "division by zero",
		},
		{
			name:            "value error: modulo by zero",
			input:           `10 % 0`,
			expectedKind:    "value error",
			expectedMsgPart: "division by zero",
		},
		// Attribute errors
		{
			name:            "type error: nil attribute access",
			input:           `nil.foo`,
			expectedKind:    "type error",
			expectedMsgPart: "attribute",
		},
		// Index errors
		{
			name:            "value error: index out of bounds",
			input:           `[1, 2, 3][10]`,
			expectedKind:    "value error",
			expectedMsgPart: "out of range",
		},
		// Additional documented errors
		{
			name:            "type error: calling non-callable",
			input:           `let x = 5; x()`,
			expectedKind:    "type error",
			expectedMsgPart: "not callable",
		},
		{
			name:            "value error: negative index out of bounds",
			input:           `[1, 2, 3][-10]`,
			expectedKind:    "value error",
			expectedMsgPart: "out of range",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// First verify the error is thrown
			_, err := run(context.Background(), tt.input)
			assert.NotNil(t, err, "expected error but got none")

			// Now verify it can be caught and error attributes are correct
			catchCode := `
			let caught = false
			let errKind = ""
			let errMsg = ""
			try {
				` + tt.input + `
			} catch e {
				caught = true
				errKind = e.kind()
				errMsg = e.message()
			}
			[caught, errKind, errMsg]
			`
			result, err := run(context.Background(), catchCode)
			assert.Nil(t, err, "try/catch should not propagate error")

			list, ok := result.(*object.List)
			assert.True(t, ok, "expected list result")

			caught, _ := list.GetItem(object.NewInt(0))
			assert.Equal(t, caught, object.True, "error should be caught")

			kindObj, _ := list.GetItem(object.NewInt(1))
			kind := kindObj.(*object.String).Value()
			assert.Contains(t, kind, tt.expectedKind, "error kind mismatch")

			msgObj, _ := list.GetItem(object.NewInt(2))
			msg := msgObj.(*object.String).Value()
			assert.Contains(t, msg, tt.expectedMsgPart, "error message should contain expected text")
		})
	}
}

// TestRuntimeErrorsReturnValues tests that runtime errors work correctly with try-as-expression.
func TestRuntimeErrorsReturnValues(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected object.Object
	}{
		{
			name:     "division by zero returns catch value",
			input:    `try { 1 / 0 } catch e { -1 }`,
			expected: object.NewInt(-1),
		},
		{
			name:     "type error returns catch value",
			input:    `try { 1 + "foo" } catch e { "type error handled" }`,
			expected: object.NewString("type error handled"),
		},
		{
			name:     "index error returns catch value",
			input:    `try { [1, 2, 3][100] } catch e { 0 }`,
			expected: object.NewInt(0),
		},
		{
			name:     "nil access returns catch value",
			input:    `try { nil.foo } catch e { "nil handled" }`,
			expected: object.NewString("nil handled"),
		},
		{
			name:     "chained error handling",
			input:    `(try { 1/0 } catch e { 10 }) + (try { 2/0 } catch e { 20 })`,
			expected: object.NewInt(30),
		},
		{
			name: "error in complex expression",
			input: `
			let items = [1, 2, 3]
			let result = try {
				items[0] + items[1] + items[100]
			} catch e {
				items[0] + items[1]
			}
			result
			`,
			expected: object.NewInt(3),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := run(context.Background(), tt.input)
			assert.Nil(t, err, "unexpected error: %v", err)
			assert.Equal(t, result, tt.expected)
		})
	}
}

// TestComplexControlFlow tests complex interactions between try/catch and control flow.
func TestComplexControlFlow(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected object.Object
	}{
		{
			name: "multiple sequential try blocks",
			input: `
			let result = []
			try {
				result = result + ["try1"]
				throw "err1"
			} catch e {
				result = result + ["catch1"]
			}
			try {
				result = result + ["try2"]
				throw "err2"
			} catch e {
				result = result + ["catch2"]
			}
			result
			`,
			expected: object.NewList([]object.Object{
				object.NewString("try1"),
				object.NewString("catch1"),
				object.NewString("try2"),
				object.NewString("catch2"),
			}),
		},
		{
			name: "try in if-else branches",
			input: `
			function testBranch(condition) {
				if (condition) {
					try {
						throw "from if"
					} catch e {
						return "if caught"
					}
				} else {
					try {
						throw "from else"
					} catch e {
						return "else caught"
					}
				}
			}
			[testBranch(true), testBranch(false)]
			`,
			expected: object.NewList([]object.Object{
				object.NewString("if caught"),
				object.NewString("else caught"),
			}),
		},
		{
			name: "recursive function with try/catch at top level",
			input: `
			function fib(n) {
				if (n < 0) { throw "negative" }
				if (n <= 1) { return n }
				return fib(n - 1) + fib(n - 2)
			}
			let r1 = 0
			let r2 = 0
			try { r1 = fib(6) } catch e { r1 = -1 }
			try { r2 = fib(-1) } catch e { r2 = -1 }
			[r1, r2]
			`,
			expected: object.NewList([]object.Object{
				object.NewInt(8),
				object.NewInt(-1),
			}),
		},
		{
			name: "exception in dispatch function",
			input: `
			function dispatch(op, a, b) {
				try {
					if (op == "add") { return a + b }
					if (op == "sub") { return a - b }
					if (op == "div") { return a / b }
					throw "unknown op: " + op
				} catch e {
					return "error: " + string(e)
				}
			}
			[dispatch("add", 2, 3), dispatch("bad", 1, 1)]
			`,
			expected: object.NewList([]object.Object{
				object.NewInt(5),
				object.NewString("error: unknown op: bad"),
			}),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := run(context.Background(), tt.input)
			assert.Nil(t, err, "unexpected error: %v", err)
			assert.Equal(t, result, tt.expected)
		})
	}
}

// TestUnhandledExceptions tests that unhandled exceptions propagate correctly.
func TestUnhandledExceptions(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		errorContains string
	}{
		{
			name:          "simple throw without catch",
			input:         `throw "unhandled"`,
			errorContains: "unhandled",
		},
		{
			name: "throw in function without handler",
			input: `
			function bad() { throw "error from function" }
			bad()
			`,
			errorContains: "error from function",
		},
		{
			name: "throw escapes catch that rethrows",
			input: `
			try {
				throw "original"
			} catch e {
				throw "rethrown"
			}
			`,
			errorContains: "rethrown",
		},
		{
			name: "throw escapes finally",
			input: `
			try {
				// nothing
			} finally {
				throw "from finally"
			}
			`,
			errorContains: "from finally",
		},
		{
			name: "runtime error without handler",
			input: `
			let x = nil
			x.foo
			`,
			errorContains: "attribute",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := run(context.Background(), tt.input)
			assert.NotNil(t, err)
			assert.Contains(t, err.Error(), tt.errorContains)
		})
	}
}

// TestErrorObjectAttributes tests that caught error objects have expected attributes.
func TestErrorObjectAttributes(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected object.Object
	}{
		{
			name: "error message access",
			input: `
			let msg = ""
			try {
				throw "test message"
			} catch e {
				msg = e.message()
			}
			msg
			`,
			expected: object.NewString("test message"),
		},
		{
			name: "error to string conversion",
			input: `
			let msg = ""
			try {
				throw "hello"
			} catch e {
				msg = string(e)
			}
			msg
			`,
			expected: object.NewString("hello"),
		},
		{
			name: "error equality check by message",
			input: `
			let same = false
			try {
				throw "specific error"
			} catch e {
				same = e.message() == "specific error"
			}
			same
			`,
			expected: object.True,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := run(context.Background(), tt.input)
			assert.Nil(t, err, "unexpected error: %v", err)
			assert.Equal(t, result, tt.expected)
		})
	}
}

// TestThrowVariousTypes tests throwing different types of values.
func TestThrowVariousTypes(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected object.Object
	}{
		{
			name: "throw string",
			input: `
			let msg = ""
			try { throw "str" } catch e { msg = string(e) }
			msg
			`,
			expected: object.NewString("str"),
		},
		{
			name: "throw error object",
			input: `
			let msg = ""
			try { throw error("custom error") } catch e { msg = string(e) }
			msg
			`,
			expected: object.NewString("custom error"),
		},
		{
			name: "throw computed expression",
			input: `
			let msg = ""
			try { throw "error " + string(1 + 2) } catch e { msg = string(e) }
			msg
			`,
			expected: object.NewString("error 3"),
		},
		{
			name: "throw number (converted to string)",
			input: `
			let msg = ""
			try { throw 42 } catch e { msg = string(e) }
			msg
			`,
			expected: object.NewString("42"),
		},
		{
			name: "throw nil (converted to string)",
			input: `
			let msg = ""
			try { throw nil } catch e { msg = string(e) }
			msg
			`,
			expected: object.NewString("nil"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := run(context.Background(), tt.input)
			assert.Nil(t, err, "unexpected error: %v", err)
			assert.Equal(t, result, tt.expected)
		})
	}
}

// TestStressExceptionHandling tests exception handling under stress.
func TestStressExceptionHandling(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected object.Object
	}{
		{
			name: "many sequential throws via recursion",
			input: `
			function throwMany(remaining, count) {
				if (remaining <= 0) { return count }
				try {
					throw "error"
				} catch e {
					return throwMany(remaining - 1, count + 1)
				}
			}
			throwMany(100, 0)
			`,
			expected: object.NewInt(100),
		},
		{
			name: "nested try/catch with rethrow",
			input: `
			function nest(depth) {
				if (depth <= 0) {
					throw "bottom"
				}
				try {
					return nest(depth - 1)
				} catch e {
					throw "level " + string(depth)
				}
			}
			let result = ""
			try {
				nest(3)
			} catch e {
				result = string(e)
			}
			result
			`,
			expected: object.NewString("level 3"),
		},
		{
			name: "exception in long call chain",
			input: `
			function chain(n) {
				if (n <= 0) { throw "chain end" }
				return chain(n - 1)
			}
			let result = ""
			try {
				chain(50)
			} catch e {
				result = string(e)
			}
			result
			`,
			expected: object.NewString("chain end"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := run(context.Background(), tt.input)
			assert.Nil(t, err, "unexpected error: %v", err)
			assert.Equal(t, result, tt.expected)
		})
	}
}

// TestExceptionEdgeCases tests edge cases in exception handling.
func TestExceptionEdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected object.Object
	}{
		{
			name: "empty try block",
			input: `
			let result = "ok"
			try {
			} catch e {
				result = "caught"
			}
			result
			`,
			expected: object.NewString("ok"),
		},
		{
			name: "empty catch block",
			input: `
			let result = "ok"
			try {
				throw "ignored"
			} catch e {
			}
			result
			`,
			expected: object.NewString("ok"),
		},
		{
			name: "exception via function call in expression",
			input: `
			function throwIt() { throw "expr error" }
			let result = ""
			try {
				let x = 1 + throwIt()
			} catch e {
				result = "caught: " + string(e)
			}
			result
			`,
			expected: object.NewString("caught: expr error"),
		},
		{
			name: "exception in list literal",
			input: `
			let result = ""
			try {
				let x = [1, 2, (function() { throw "list error" })()]
			} catch e {
				result = "caught"
			}
			result
			`,
			expected: object.NewString("caught"),
		},
		{
			name: "exception in map literal",
			input: `
			let result = ""
			try {
				let x = {a: 1, b: (function() { throw "map error" })()}
			} catch e {
				result = "caught"
			}
			result
			`,
			expected: object.NewString("caught"),
		},
		{
			name: "catch block does not shadow outer variable",
			input: `
			let e = "outer"
			try {
				throw "inner"
			} catch e {
				// e is shadowed here
			}
			e
			`,
			expected: object.NewString("outer"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := run(context.Background(), tt.input)
			assert.Nil(t, err, "unexpected error: %v", err)
			assert.Equal(t, result, tt.expected)
		})
	}
}

// TestExceptionInArrowFunctions tests exception handling with arrow functions.
func TestExceptionInArrowFunctions(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected object.Object
	}{
		{
			name: "throw in arrow function",
			input: `
			let thrower = x => { throw "arrow error" }
			let result = ""
			try {
				thrower(1)
			} catch e {
				result = "caught: " + string(e)
			}
			result
			`,
			expected: object.NewString("caught: arrow error"),
		},
		{
			name: "arrow function in try/catch",
			input: `
			let result = ""
			try {
				let fn = x => x * 2
				result = string(fn(5))
			} catch e {
				result = "error"
			}
			result
			`,
			expected: object.NewString("10"),
		},
		{
			name: "arrow in map with exception",
			input: `
			let result = ""
			try {
				[1, 2, 3].map(x => {
					if (x == 2) { throw "stop" }
					return x
				})
			} catch e {
				result = "caught"
			}
			result
			`,
			expected: object.NewString("caught"),
		},
		{
			name: "arrow with try/catch inside",
			input: `
			let handler = x => {
				try {
					if (x < 0) { throw "negative" }
					return x * 2
				} catch e {
					return -1
				}
			}
			[handler(5), handler(-3)]
			`,
			expected: object.NewList([]object.Object{
				object.NewInt(10),
				object.NewInt(-1),
			}),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := run(context.Background(), tt.input)
			assert.Nil(t, err, "unexpected error: %v", err)
			assert.Equal(t, result, tt.expected)
		})
	}
}

// TestTryAsExpression tests that try/catch is an expression that returns a value
// (Kotlin-style semantics).
func TestTryAsExpression(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected object.Object
	}{
		{
			name: "try succeeds - returns try value",
			input: `
			let result = try { 42 } catch e { -1 }
			result
			`,
			expected: object.NewInt(42),
		},
		{
			name: "try throws - returns catch value",
			input: `
			let result = try { throw "error"; 42 } catch e { -1 }
			result
			`,
			expected: object.NewInt(-1),
		},
		{
			name: "try with complex expression",
			input: `
			let x = 10
			let result = try { x * 2 + 5 } catch e { 0 }
			result
			`,
			expected: object.NewInt(25),
		},
		{
			name: "catch receives error and returns value",
			input: `
			let result = try {
				throw "test error"
			} catch e {
				"caught: " + string(e)
			}
			result
			`,
			expected: object.NewString("caught: test error"),
		},
		{
			name: "try with finally - returns try value, finally runs",
			input: `
			let sideEffect = false
			let result = try { 42 } catch e { -1 } finally { sideEffect = true; 999 }
			[result, sideEffect]
			`,
			expected: object.NewList([]object.Object{
				object.NewInt(42),
				object.True,
			}),
		},
		{
			name: "catch with finally - returns catch value, finally runs",
			input: `
			let sideEffect = false
			let result = try { throw "err" } catch e { -1 } finally { sideEffect = true; 999 }
			[result, sideEffect]
			`,
			expected: object.NewList([]object.Object{
				object.NewInt(-1),
				object.True,
			}),
		},
		{
			name: "try/finally without catch - returns try value",
			input: `
			let sideEffect = false
			let result = try { 42 } finally { sideEffect = true; 999 }
			[result, sideEffect]
			`,
			expected: object.NewList([]object.Object{
				object.NewInt(42),
				object.True,
			}),
		},
		{
			name: "nested try expressions",
			input: `
			let result = try {
				try {
					throw "inner"
				} catch e {
					100
				}
			} catch e {
				-1
			}
			result
			`,
			expected: object.NewInt(100),
		},
		{
			name: "try expression in function return",
			input: `
			function safeParse(s) {
				return try {
					int(s)
				} catch e {
					0
				}
			}
			[safeParse("42"), safeParse("invalid")]
			`,
			expected: object.NewList([]object.Object{
				object.NewInt(42),
				object.NewInt(0),
			}),
		},
		{
			name: "try expression used directly",
			input: `
			(try { 10 } catch e { 0 }) + (try { throw "x"; 5 } catch e { 20 })
			`,
			expected: object.NewInt(30),
		},
		{
			name: "try expression in list literal",
			input: `
			[try { 1 } catch e { 0 }, try { throw "x" } catch e { 2 }, try { 3 } catch e { 0 }]
			`,
			expected: object.NewList([]object.Object{
				object.NewInt(1),
				object.NewInt(2),
				object.NewInt(3),
			}),
		},
		{
			name: "try expression with block returning last expression",
			input: `
			let result = try {
				let a = 10
				let b = 20
				a + b
			} catch e {
				0
			}
			result
			`,
			expected: object.NewInt(30),
		},
		// Tests for try/finally propagation behavior
		{
			name: "try/finally where try throws - exception propagates, no value assigned",
			input: `
			let x = "default"
			try {
				x = try { throw "inner" } finally { "finally-value" }
			} catch e {
				// x should still be "default" - the inner assignment never happened
			}
			x
			`,
			expected: object.NewString("default"),
		},
		{
			name: "try/finally propagation - finally runs before propagating",
			input: `
			let finallyRan = false
			try {
				try {
					throw "error"
				} finally {
					finallyRan = true
				}
			} catch e {
				// exception was caught here
			}
			finallyRan
			`,
			expected: object.True,
		},
		{
			name: "partial expression evaluation with throw - stack stays balanced",
			input: `
			function thrower() { throw "boom" }
			let result = "ok"
			try {
				try {
					1 + thrower()  // 1 gets pushed before throw
				} finally {
					"cleanup"
				}
			} catch e {
				result = "caught"
			}
			result
			`,
			expected: object.NewString("caught"),
		},
		{
			name: "multiple try/finally with throws - stack stays balanced",
			input: `
			function thrower() { throw "boom" }
			let results = []
			let i = 0

			function test() {
				i = i + 1
				try {
					try {
						1 + 2 + thrower()
					} finally {
						results = results + ["finally " + string(i)]
					}
				} catch e {
					results = results + ["caught " + string(i)]
				}
				return i
			}

			// Run multiple times to verify stack doesn't corrupt
			[test(), test(), test(), results]
			`,
			expected: object.NewList([]object.Object{
				object.NewInt(1),
				object.NewInt(2),
				object.NewInt(3),
				object.NewList([]object.Object{
					object.NewString("finally 1"),
					object.NewString("caught 1"),
					object.NewString("finally 2"),
					object.NewString("caught 2"),
					object.NewString("finally 3"),
					object.NewString("caught 3"),
				}),
			}),
		},
		{
			name: "try/finally without catch - value when no exception",
			input: `
			let finallyRan = false
			let result = try {
				42
			} finally {
				finallyRan = true
				999  // This value is discarded
			}
			[result, finallyRan]
			`,
			expected: object.NewList([]object.Object{
				object.NewInt(42),
				object.True,
			}),
		},
		{
			name: "deeply nested try/finally with exception propagation",
			input: `
			let order = []
			try {
				try {
					try {
						throw "deep"
					} finally {
						order = order + ["inner finally"]
					}
				} finally {
					order = order + ["middle finally"]
				}
			} catch e {
				order = order + ["caught: " + string(e)]
			}
			order
			`,
			expected: object.NewList([]object.Object{
				object.NewString("inner finally"),
				object.NewString("middle finally"),
				object.NewString("caught: deep"),
			}),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := run(context.Background(), tt.input)
			assert.Nil(t, err, "unexpected error: %v", err)
			assert.Equal(t, result, tt.expected)
		})
	}
}

// TestClosureCreationWithTryCatch verifies that closures work correctly with
// exception handling. This tests that the LoadClosure opcode properly continues
// to evalLoop (not the inner for loop) when an error is handled.
func TestClosureCreationWithTryCatch(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected object.Object
	}{
		{
			name: "closure with captured variable in try block",
			input: `
			let outer = 10
			let fn = nil
			try {
				fn = function() { return outer }
			} catch e {
				fn = function() { return -1 }
			}
			fn()
			`,
			expected: object.NewInt(10),
		},
		{
			name: "nested closures in try block",
			input: `
			let x = 5
			let fn = nil
			try {
				fn = function() {
					let inner = function() { return x * 2 }
					return inner()
				}
			} catch e {
				fn = function() { return -1 }
			}
			fn()
			`,
			expected: object.NewInt(10),
		},
		{
			name: "multiple closures capturing different variables",
			input: `
			let a = 1
			let b = 2
			let c = 3
			let fns = []
			try {
				fns = [
					function() { return a },
					function() { return b },
					function() { return c }
				]
			} catch e {
				fns = [function() { return -1 }]
			}
			[fns[0](), fns[1](), fns[2]()]
			`,
			expected: object.NewList([]object.Object{
				object.NewInt(1),
				object.NewInt(2),
				object.NewInt(3),
			}),
		},
		{
			name: "closure creation followed by exception",
			input: `
			let x = 10
			let fn = nil
			let result = ""
			try {
				fn = function() { return x }
				throw "after closure"
			} catch e {
				result = "caught: " + string(e)
			}
			[fn(), result]
			`,
			expected: object.NewList([]object.Object{
				object.NewInt(10),
				object.NewString("caught: after closure"),
			}),
		},
		{
			name: "closure factory in try block",
			input: `
			let makeAdder = function(n) {
				return function(x) { return x + n }
			}
			let add5 = nil
			let add10 = nil
			try {
				add5 = makeAdder(5)
				add10 = makeAdder(10)
			} catch e {
				add5 = function(x) { return -1 }
				add10 = function(x) { return -1 }
			}
			[add5(3), add10(3)]
			`,
			expected: object.NewList([]object.Object{
				object.NewInt(8),
				object.NewInt(13),
			}),
		},
		{
			name: "closure in iteration with try/catch",
			input: `
			let funcs = []
			let values = [1, 2, 3, 4, 5]
			values.each(function(v) {
				try {
					funcs = funcs + [function() { return v * 2 }]
				} catch e {
					funcs = funcs + [function() { return -1 }]
				}
			})
			[funcs[0](), funcs[2](), funcs[4]()]
			`,
			expected: object.NewList([]object.Object{
				object.NewInt(2),
				object.NewInt(6),
				object.NewInt(10),
			}),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := run(context.Background(), tt.input)
			assert.Nil(t, err, "unexpected error: %v", err)
			assert.Equal(t, result, tt.expected)
		})
	}
}
