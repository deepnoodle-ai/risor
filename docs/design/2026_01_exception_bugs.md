# Exception Handling Bugs

This document tracks bugs in Risor's try/catch/finally implementation that differ from expected behavior in Python and other languages.

## Bug 1: Division by Zero Causes Go Panic

**Status:** ✅ FIXED

**Severity:** Critical

**Description:**
Integer division by zero causes a Go runtime panic instead of a catchable Risor error. This crashes the VM instead of allowing the error to be handled.

**Expected Behavior (Python):**
```python
try:
    x = 10 / 0
except ZeroDivisionError:
    print("caught")  # This executes
```

**Fix Applied:**
Added zero checks in `object/int.go` and `object/byte.go` for both division and modulo operations:
- `object/int.go:115-124` - runOperationInt Divide and Modulo
- `object/byte.go:113-122` - runOperationByte Divide and Modulo
- `object/byte.go:148-157` - runOperationInt Divide and Modulo

Division/modulo by zero now returns `"value error: division by zero"` which is catchable:
```javascript
try {
    let x = 10 / 0
} catch e {
    // e.message == "value error: division by zero"
}
```

**Tests:** `object/div_test.go`, `vm/exception_test.go:TestRuntimeErrorsAreCaught`

---

## Bug 2: Finally Block Doesn't Run on Return from Try

**Status:** ✅ FIXED

**Severity:** Critical

**Description:**
When a `return` statement executes inside a try or catch block, the finally block was NOT executed. In Python and virtually all other languages with try/finally, the finally block ALWAYS runs regardless of how control leaves the try block.

**Expected Behavior (Python):**
```python
def test():
    try:
        return "from try"
    finally:
        print("finally ran")  # This ALWAYS executes

test()  # Prints "finally ran", returns "from try"
```

**Fix Applied:**
Multiple changes to correctly handle return-through-finally:

1. Added `pendingReturn` and `inCatch` fields to `exceptionFrame` struct (`vm/vm.go:107-108`)
2. Modified `ReturnValue` opcode handler to detect finally blocks and save the return value (`vm/vm.go:732-752`)
3. Modified `handleException` to keep exception frame when entering catch with finally (`vm/vm.go:1762-1769`)
4. Modified `EndFinally` to complete pending returns after finally runs (`vm/vm.go:1163-1200`)
5. Fixed compiler to set `FinallyStart = 0` when no finally block exists (`compiler/compiler.go:2181-2185`)
6. Added `evalLoop` label to main eval loop for proper control flow (`vm/vm.go:487`)

Finally now runs for:
- Return from try block
- Return from catch block
- Normal completion of try/catch flowing to finally

**Tests:** `vm/exception_test.go:TestReturnInTryCatch`
- "return in try with finally - finally runs"
- "return in catch with finally - finally runs"

**Example (now works correctly):**
```javascript
let finallyRan = false
function test() {
    try {
        return "from try"
    } finally {
        finallyRan = true  // Now executes!
    }
}
let result = test()
// result = "from try"
// finallyRan = true  // Fixed!
```

---

**Original Root Cause:**
The `ReturnValue` opcode handler in the VM didn't check for pending finally blocks before returning. It immediately restored the caller's frame without running finally cleanup.

**Original Fix Required:**
Before returning from a function, check the exception stack for any finally blocks that need to run. The finally block should execute, then the return should proceed with the original return value.

This required:
1. Detecting pending finally blocks on return
2. Saving the return value
3. Executing the finally block
4. Restoring and returning the saved value

---

## Bug 2b: Exception in Catch Block with Finally (Related Fix)

**Status:** ✅ FIXED

**Description:**
When an exception is thrown inside a catch block that has an associated finally block, the finally block wasn't running and the exception wasn't propagating correctly. This caused an infinite loop because `handleException` would repeatedly try to enter the same catch block.

**Fix Applied:**
Modified `handleException` in `vm/vm.go` to:
1. Track when we're inside a catch block (`inCatch` flag)
2. If already in catch and an exception occurs, skip re-entering catch and go to finally
3. Store the new exception as `pendingError` for re-raising after finally completes

**Test:** `vm/exception_test.go:TestExceptionInCatch/"throw_in_catch_with_finally_-_finally_runs_and_exception_propagates"`

---

## Bug 3: Break/Continue in Try Block Skip Finally

**Severity:** High (if loops are added)

**Description:**
This is a related issue to Bug 2. If Risor adds loop constructs (for, while), break and continue statements inside try blocks would similarly skip finally blocks.

**Note:** Currently Risor has no loop constructs, so this is a future concern. When loops are added, they need to respect finally blocks.

---

## Enhancement: Try as Expression (Kotlin-style)

**Status:** ✅ IMPLEMENTED

**Description:**
`try/catch` is now an expression that returns a value, following Kotlin semantics:
- If try block succeeds → returns try block's value
- If exception is caught → returns catch block's value
- Finally block runs for side effects but does NOT affect the return value

**Example:**
```javascript
// Try as expression - returns value
let result = try { parseInt(input) } catch e { -1 }

// Use directly in expressions
let total = (try { x / y } catch e { 0 }) + offset

// In function returns
function safeParse(s) {
    return try { int(s) } catch e { 0 }
}

// Finally doesn't affect return value
let x = try { 42 } finally { cleanup() }  // x == 42
```

**Implementation:**
Modified `compiler/compiler.go:compileTry()`:
1. Removed `PopTop` after try block - value stays on stack
2. Removed `PopTop` after catch block - value stays on stack
3. Kept `PopTop` after finally block - finally value is discarded
4. Removed final `LoadNil` - try/catch value is already on stack

**Tests:** `vm/exception_test.go:TestTryAsExpression` (12 test cases)

---

## Test Coverage

These bugs were discovered through comprehensive testing in `vm/exception_test.go`. The test file includes:

- 80+ test cases covering exception handling
- Tests for nested try/catch (up to triple nesting)
- Tests for exceptions crossing function boundaries
- Tests for closures capturing exception variables
- Tests for exceptions in builtin callbacks (.map, .filter, .each)
- Tests for try-as-expression (Kotlin-style semantics)
- Stress tests with 100+ sequential throws
- Tests for division by zero (`object/div_test.go`)

All tests now pass with the fixes applied.

---

## Priority

1. ~~**Bug 2 (finally on return)**~~ - ✅ Fixed
2. ~~**Bug 1 (division by zero)**~~ - ✅ Fixed

---

## References

- Python try/finally semantics: https://docs.python.org/3/reference/compound_stmts.html#the-try-statement
- JavaScript try/finally semantics: https://developer.mozilla.org/en-US/docs/Web/JavaScript/Reference/Statements/try...catch
- Test file: `vm/exception_test.go`
- Division test file: `object/div_test.go`
