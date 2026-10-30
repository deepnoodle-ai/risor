package vm

import (
	"context"
	"errors"
	"testing"

	"github.com/deepnoodle-ai/risor/v2/pkg/object"
	"github.com/deepnoodle-ai/wonton/assert"
)

// runAndCheckExcStack runs the source and asserts that no exception frames
// remain on the exception stack once execution completes.
func runAndCheckExcStack(t *testing.T, source string) object.Object {
	t.Helper()
	ctx := context.Background()
	vm, err := newVM(ctx, source)
	assert.Nil(t, err)
	assert.Nil(t, vm.Run(ctx))
	assert.Equal(t, vm.excStackSize, 0)
	result, ok := vm.TOS()
	assert.True(t, ok)
	return result
}

// The exception stack must grow beyond its initial capacity.
func TestExcStackGrows(t *testing.T) {
	t.Run("recursion through try", func(t *testing.T) {
		result := runAndCheckExcStack(t, `
		function recurse(n) {
			if (n <= 0) { return 42 }
			try { return recurse(n - 1) } catch e { return -1 }
		}
		recurse(200)
		`)
		assert.Equal(t, result, object.NewInt(42))
	})

	t.Run("deep static nesting", func(t *testing.T) {
		src := "let x = "
		for i := 0; i < 20; i++ {
			src += "try { "
		}
		src += `throw "boom"`
		for i := 0; i < 20; i++ {
			src += " } finally { 0 }"
		}
		src = "let r = try { " + src + ` } catch e { "caught" }; r`
		result := runAndCheckExcStack(t, src)
		assert.Equal(t, result, object.NewString("caught"))
	})
}

// The exception stack is bounded, and overflowing it is a clean error.
func TestExcStackBounded(t *testing.T) {
	recurse := `
	function recurse(n) {
		try { try { try { try { try { return recurse(n + 1) } catch e { throw e } } catch e { throw e } } catch e { throw e } } catch e { throw e } } catch e { throw e }
	}
	`

	t.Run("uncaught", func(t *testing.T) {
		ctx := context.Background()
		vm, err := newVM(ctx, recurse+"recurse(0)")
		assert.Nil(t, err)
		vm.maxFrameDepth = 64
		err = vm.Run(ctx)
		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), "exception handler depth")
	})

	// Overflowing the exception stack is a stack overflow, which ends the
	// run. A catch block in a caller cannot catch it.
	t.Run("not catchable", func(t *testing.T) {
		ctx := context.Background()
		vm, err := newVM(ctx, recurse+`try { recurse(0) } catch e { "caught: " + string(e) }`)
		assert.Nil(t, err)
		vm.maxFrameDepth = 64
		err = vm.Run(ctx)
		assert.True(t, errors.Is(err, ErrStackOverflow), "got %v", err)
		assert.Contains(t, err.Error(), "exception handler depth")
	})
}

// A throw mid-expression must restore the operand stack.
func TestExcRestoresStackPointer(t *testing.T) {
	result := runAndCheckExcStack(t, `
	function fail() { throw "x" }
	100 + (try { 1 + (2 + fail()) } catch e { 5 })
	`)
	assert.Equal(t, result, object.NewInt(105))

	result = runAndCheckExcStack(t, `
	100 + (try { 1 + (2 + (3 * [1,2][5])) } catch e { 5 })
	`)
	assert.Equal(t, result, object.NewInt(105))
}

// A return through nested finally blocks runs every finally.
func TestReturnRunsAllFinallyBlocks(t *testing.T) {
	result := runAndCheckExcStack(t, `
	let log = []
	function f() {
		try {
			try {
				try { return 1 } finally { log.append("a") }
			} catch e {
				log.append("never")
			}
		} finally {
			log.append("b")
		}
	}
	let r = f()
	[r, log]
	`)
	assert.Equal(t, result.Inspect(), `[1, ["a", "b"]]`)
}

// A return inside a finally overrides the pending return and pops the frame.
func TestReturnInsideFinally(t *testing.T) {
	result := runAndCheckExcStack(t, `
	let log = []
	function f() {
		try {
			try { return 1 } finally { return 2 }
		} finally {
			log.append("outer")
		}
	}
	let r = f()
	[r, log]
	`)
	assert.Equal(t, result.Inspect(), `[2, ["outer"]]`)

	result = runAndCheckExcStack(t, `
	function g() { try { return 1 } finally { return 2 } }
	let total = 0
	list(range(50)).each(function(i) { total = total + g() })
	total
	`)
	assert.Equal(t, result, object.NewInt(100))
}

// A catch falling through to finally runs finally exactly once.
func TestCatchFallthroughToFinally(t *testing.T) {
	result := runAndCheckExcStack(t, `
	let count = 0
	function f() {
		try { throw "a" } catch e { 1 } finally { count++; return 3 }
	}
	let r = f()
	[r, count]
	`)
	assert.Equal(t, result.Inspect(), `[3, 1]`)

	result = runAndCheckExcStack(t, `
	let count = 0
	let r = try {
		try { throw "a" } catch e { 1 } finally { count++; throw "b" }
	} catch e2 {
		string(e2)
	}
	[r, count]
	`)
	assert.Equal(t, result.Inspect(), `["b", 1]`)

	result = runAndCheckExcStack(t, `
	let r = 0
	list(range(50)).each(function(i) {
		r = r + try { throw "a" } catch e { 1 } finally { 2 }
	})
	r
	`)
	assert.Equal(t, result, object.NewInt(50))
}

// A try that completes normally inside an outer catch or finally must not
// consume the outer handler's frame.
func TestNestedTryInsideHandlers(t *testing.T) {
	result := runAndCheckExcStack(t, `
	let log = []
	try {
		try {
			throw "a"
		} catch e {
			try { 1 } finally { log.append("inner") }
			throw "b"
		} finally {
			log.append("outer")
		}
	} catch e {
		log.append("caught " + string(e))
	}
	log
	`)
	assert.Equal(t, result.Inspect(), `["inner", "outer", "caught b"]`)

	result = runAndCheckExcStack(t, `
	let log = []
	try {
		try {
			throw "a"
		} finally {
			try { 1 } finally { log.append("inner") }
			log.append("after")
		}
	} catch e {
		log.append("caught " + string(e))
	}
	log
	`)
	assert.Equal(t, result.Inspect(), `["inner", "after", "caught a"]`)
}

// A throw inside finally (after catch threw) must not re-enter the catch.
func TestThrowInFinallyAfterCatchThrow(t *testing.T) {
	result := runAndCheckExcStack(t, `
	let log = []
	let r = try {
		try {
			throw "a"
		} catch e {
			log.append("catch")
			throw "b"
		} finally {
			log.append("finally")
			throw "c"
		}
	} catch e {
		string(e)
	}
	[r, log]
	`)
	assert.Equal(t, result.Inspect(), `["c", ["catch", "finally"]]`)
}

// Repeated execution of every try shape must leave the exception stack empty.
func TestExcStackNoLeak(t *testing.T) {
	result := runAndCheckExcStack(t, `
	function retTry() { try { return 1 } catch e { return 0 } }
	function retTryFinally() { try { return 1 } finally { 0 } }
	function retFinally() { try { return 1 } finally { return 2 } }
	function retCatchFinally() { try { throw "x" } catch e { return 3 } finally { 0 } }
	function catchFallthrough() { try { throw "x" } catch e { 4 } finally { 0 } }
	function catchFallthroughReturn() { try { throw "x" } catch e { 4 } finally { return 5 } }
	function nested() {
		try { try { return 6 } finally { 0 } } finally { 0 }
	}
	function thrower() { try { throw "x" } finally { 0 } }
	let total = 0
	list(range(100)).each(function(i) {
		total = total + retTry() + retTryFinally() + retFinally() + retCatchFinally()
		total = total + catchFallthrough() + catchFallthroughReturn() + nested()
		total = total + try { thrower() } catch e { 1 }
		total = total + try { 1 } finally { 0 }
	})
	total
	`)
	assert.Equal(t, result, object.NewInt(100*(1+1+2+3+4+5+6+1+1)))
}
