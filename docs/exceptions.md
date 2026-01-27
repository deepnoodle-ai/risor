# Exception Handling

Risor uses a Python-like exception model with `try`, `catch`, `finally`, and
`throw`. Unlike Python, `try` is an **expression** that returns a value
(Kotlin-style semantics).

## Quick Reference

```ts
// Basic try/catch
let result = try {
    riskyOperation()
} catch e {
    defaultValue
}

// With finally (for cleanup)
try {
    openFile()
} catch e {
    handleError(e)
} finally {
    closeFile()  // Always runs
}

// Throw an exception
throw "something went wrong"
throw error("detailed error message")
```

## Try as Expression

`try/catch` is an expression that evaluates to a value:

| Scenario | Expression Value |
|----------|------------------|
| Try succeeds | Try block's value |
| Exception caught | Catch block's value |
| Finally present | Finally does NOT affect the value |

### Examples

```ts
// Returns try value on success
let x = try { 42 } catch e { -1 }  // x == 42

// Returns catch value on exception
let x = try { throw "err"; 42 } catch e { -1 }  // x == -1

// Finally runs but doesn't affect result
let x = try { 42 } finally { 999 }  // x == 42 (not 999)
let x = try { 42 } catch e { -1 } finally { 999 }  // x == 42

// Use directly in expressions
let total = (try { a / b } catch e { 0 }) + offset

// In function returns
function safeParse(s) {
    return try { int(s) } catch e { 0 }
}

// In list literals
let values = [
    try { compute(a) } catch e { 0 },
    try { compute(b) } catch e { 0 },
]
```

### Block Values

The value of a block is its last expression:

```ts
let result = try {
    let a = 10
    let b = 20
    a + b  // This is the try block's value
} catch e {
    0
}
// result == 30
```

## Throwing Exceptions

Use `throw` to raise an exception:

```ts
throw "error message"              // String converted to error
throw error("detailed message")    // Explicit error value
throw existingError                // Re-throw an error
```

**Note:** Creating an error value does NOT throw it:

```ts
let err = error("not thrown")  // Just creates a value
print(err.message())           // "not thrown"
// No exception raised

throw err  // NOW it's thrown
```

## Catch Block

The catch block receives the error as a value:

```ts
try {
    throw "oops"
} catch e {
    print(e.message())  // "oops"
    print(e.kind())     // "runtime error"
}
```

### Error Attributes

| Attribute    | Type       | Description                    |
|--------------|------------|--------------------------------|
| `message()`  | string     | Error message                  |
| `kind()`     | string     | Error category                 |
| `line()`     | int        | Line number (1-based)          |
| `column()`   | int        | Column number (1-based)        |
| `filename()` | string/nil | Source filename                |
| `source()`   | string/nil | Source line text               |
| `stack()`    | list       | Stack frames as maps           |

### Error Kinds

| Kind | Description |
|------|-------------|
| `"type error"` | Type mismatch (e.g., `1 + "foo"`) |
| `"value error"` | Invalid value (e.g., division by zero) |
| `"name error"` | Undefined variable |
| `"runtime error"` | General runtime error |

### Catch Without Variable

You can omit the error variable if you don't need it:

```ts
try {
    riskyOperation()
} catch {
    // Error is discarded
    "default value"
}
```

## Finally Block

The `finally` block **always runs**, regardless of how the try/catch exits:

- Normal completion of try
- Exception caught by catch
- Exception propagating (no catch or re-thrown)
- Return statement in try or catch

```ts
function example() {
    try {
        return "from try"
    } finally {
        cleanup()  // This ALWAYS runs
    }
}
```

### Finally Does Not Affect Expression Value

In Kotlin-style semantics, finally is for side effects only:

```ts
let x = try { 42 } finally { 999 }
// x == 42, not 999
// finally block ran, but its value (999) was discarded
```

### Return/Throw in Finally

Return or throw in finally overrides any pending return or exception:

```ts
// Return in finally overrides return in try
function test() {
    try {
        return "from try"
    } finally {
        return "from finally"  // This wins
    }
}
test()  // "from finally"

// Return in finally suppresses exception
function test() {
    try {
        throw "error"
    } finally {
        return "suppressed"  // Exception is suppressed
    }
}
test()  // "suppressed" (no exception)

// Throw in finally replaces pending return
function test() {
    try {
        return "from try"
    } finally {
        throw "finally error"  // This propagates
    }
}
// Throws "finally error"
```

## Exception Propagation

Uncaught exceptions propagate up the call stack:

```ts
function inner() {
    throw "error"
}

function middle() {
    inner()  // Exception passes through
}

function outer() {
    try {
        middle()
    } catch e {
        print("caught: " + e.message())
    }
}

outer()  // Prints: caught: error
```

### Finally Blocks Run During Propagation

When an exception propagates through multiple functions, all finally blocks run:

```ts
function inner() {
    try {
        throw "error"
    } finally {
        print("inner finally")
    }
}

function outer() {
    try {
        inner()
    } finally {
        print("outer finally")
    }
}

try {
    outer()
} catch e {
    print("caught")
}

// Output:
// inner finally
// outer finally
// caught
```

## Try/Finally Without Catch

Use `try/finally` when you need cleanup but want exceptions to propagate:

```ts
function processFile(path) {
    let f = open(path)
    try {
        return parse(f.read())
    } finally {
        f.close()  // Always closes, even if parse() throws
    }
}
```

When try throws with no catch:
- Finally block runs
- Exception continues propagating
- **No expression value** (execution doesn't continue)

```ts
let x = "default"
try {
    x = try { throw "inner" } finally { cleanup() }
} catch e {
    // x is still "default" - assignment never completed
}
```

## Runtime Errors

Many operations can throw runtime errors:

```ts
// Type errors
1 + "foo"           // type error: unsupported operation

// Value errors
1 / 0               // value error: division by zero
10 % 0              // value error: division by zero

// Attribute errors
nil.foo             // type error: attribute "foo" not found on nil

// Index errors
[1, 2, 3][10]       // value error: index out of bounds
```

All runtime errors are catchable:

```ts
let result = try {
    1 / 0
} catch e {
    "division failed: " + e.message()
}
// result == "division failed: value error: division by zero"
```

## Stack Traces

Errors include stack traces for debugging:

```ts
try {
    function a() { b() }
    function b() { c() }
    function c() { throw "deep error" }
    a()
} catch e {
    for frame in e.stack() {
        print(frame.function + " at line " + string(frame.line))
    }
}
// Output:
// c at line 4
// b at line 3
// a at line 2
// __main__ at line 5
```

Each stack frame contains:
- `function` - Function name (`"__main__"` for top level)
- `line` - Line number
- `column` - Column number
- `filename` - Source filename (if known)

## Best Practices

### Always Provide a Fallback Value

Since try is an expression, always provide a meaningful catch value:

```ts
// Good - explicit fallback
let config = try { loadConfig() } catch e { defaultConfig }

// Avoid - catch returns nil implicitly
let config = try { loadConfig() } catch e { }  // config could be nil
```

### Use Finally for Cleanup

```ts
let conn = connect()
try {
    conn.query("...")
} finally {
    conn.close()  // Always clean up
}
```

### Don't Suppress Errors Silently

```ts
// Bad - silently ignores errors
try { riskyOperation() } catch e { }

// Better - at least log it
try { riskyOperation() } catch e { log("error: " + e.message()) }

// Best - handle or re-throw
try {
    riskyOperation()
} catch e {
    if canRecover(e) {
        recover()
    } else {
        throw e  // Re-throw if can't handle
    }
}
```

### Prefer Specific Error Handling

```ts
try {
    let data = fetchData()
    let parsed = parse(data)
    process(parsed)
} catch e {
    // Check error kind for specific handling
    if e.kind() == "type error" {
        print("Invalid data format")
    } else {
        throw e  // Re-throw unknown errors
    }
}
```
