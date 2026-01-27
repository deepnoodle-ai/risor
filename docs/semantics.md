# Language Semantics

This document specifies the core semantics of Risor. Embedders can rely on these
behaviors as stable contracts.

## Numeric Types

Risor has three numeric types:

| Type    | Go Type   | Range           | Notes                              |
| ------- | --------- | --------------- | ---------------------------------- |
| `int`   | `int64`   | -2^63 to 2^63-1 | Small integers (-10 to 255) cached |
| `float` | `float64` | IEEE 754 double | No caching                         |
| `byte`  | `byte`    | 0 to 255        | All 256 values cached              |

### Numeric Coercion

Arithmetic operations follow these rules:

**Result type depends on operand types:**

| Left    | Right   | Result  | Notes                    |
| ------- | ------- | ------- | ------------------------ |
| `int`   | `int`   | `int`   | Integer arithmetic       |
| `int`   | `float` | `float` | Promotes to float        |
| `int`   | `byte`  | `int`   | Byte treated as int64    |
| `float` | `float` | `float` | Float arithmetic         |
| `float` | `int`   | `float` | Int promoted to float64  |
| `float` | `byte`  | `float` | Byte promoted to float64 |
| `byte`  | `byte`  | `byte`  | Pure byte operations     |
| `byte`  | `int`   | `int`   | Byte promoted to int64   |
| `byte`  | `float` | `float` | Byte promoted to float64 |

**Key principle:** When float is involved, the result is float (except power
with int base, which returns int).

**Supported operations:**

| Operation              | int | float | byte | Cross-type    |
| ---------------------- | --- | ----- | ---- | ------------- |
| `+` `-` `*` `/`        | Yes | Yes   | Yes  | Yes           |
| `%` (modulo)           | Yes | No    | Yes  | int/byte only |
| `**` (power)           | Yes | Yes   | Yes  | Yes           |
| `<<` `>>` (shift)      | Yes | No    | Yes  | int/byte only |
| `&` `\|` `^` (bitwise) | Yes | No    | Yes  | int/byte only |

**No implicit narrowing:** Float values are never implicitly converted to int.

## Equality

Every type implements `Equals(other Object) bool`. Equality is symmetric: if
`a.Equals(b)` is true, then `b.Equals(a)` is also true.

### Equality Rules by Type

**Numeric types** (cross-type equality allowed):

```risor
5 == 5.0        // true - int equals float
5 == byte(5)    // true - int equals byte
5.0 == byte(5)  // true - float equals byte
```

Numeric types are compared by value after converting to a common representation.

**Strings and bytes:**

```risor
"hello" == "hello"               // true
bytes("hello") == bytes("hello") // true
bytes("hello") == "hello"        // true - bytes can equal string
```

**Containers** (deep equality):

```risor
[1, 2, 3] == [1, 2, 3]          // true - element-wise comparison
[1, 2] == [1, 2, 3]             // false - different lengths
{a: 1, b: 2} == {b: 2, a: 1}    // true - key order doesn't matter
{a: 1} == {a: 1, b: 2}          // false - different keys
```

Lists require same length and element-wise equality. Maps require same keys
with equal values.

**Other types:**

| Type       | Equality Rule                     |
| ---------- | --------------------------------- |
| `bool`     | Same boolean value                |
| `nil`      | Only equal to nil                 |
| `time`     | Same time value                   |
| `error`    | Same error message (not location) |
| `function` | Identity only (same object)       |
| `builtin`  | Identity only                     |

## Comparison (Ordering)

Comparison operators (`<`, `>`, `<=`, `>=`) require types to be comparable.

### Comparable Types

| Type     | Comparison                 | Cross-type  |
| -------- | -------------------------- | ----------- |
| `int`    | Numeric order              | float, byte |
| `float`  | Numeric order              | int, byte   |
| `byte`   | Numeric order              | int, float  |
| `string` | Lexicographic (byte order) | No          |
| `bytes`  | Lexicographic              | string      |
| `bool`   | `false < true`             | No          |
| `list`   | Lexicographic by elements  | No          |
| `time`   | Chronological              | No          |
| `error`  | By message string          | No          |
| `nil`    | Only equal to nil          | No          |

**Not comparable:** `map`, `function`, `builtin`, `module`

Comparing incompatible types throws a type error:

```risor
"hello" < 5     // type error
{} < {}         // type error - maps not comparable
```

### List Comparison

Lists are compared lexicographically:

1. Compare elements pairwise from index 0
2. First unequal pair determines the result
3. If all compared elements are equal, shorter list is less

```risor
[1, 2] < [1, 3]     // true - second element differs
[1, 2] < [1, 2, 3]  // true - shorter is less
[1, 2, 3] < [1, 2]  // false
```

## Truthiness

Every type has a boolean interpretation used in conditionals and logical
operators.

### Falsy Values

| Type     | Falsy When                |
| -------- | ------------------------- |
| `nil`    | Always                    |
| `bool`   | `false`                   |
| `int`    | `0`                       |
| `float`  | `0.0`                     |
| `byte`   | `0`                       |
| `string` | `""` (empty)              |
| `bytes`  | `len == 0` (empty)        |
| `list`   | `len == 0` (empty)        |
| `map`    | `len == 0` (empty)        |
| `time`   | Zero time (uninitialized) |

### Always Truthy

- `error` (errors are significant values)
- `function` / `closure`
- `builtin`
- `module`

### Usage in Control Flow

Truthiness is evaluated in:

- `if` / `else` conditions
- `while` loop conditions
- Logical `!` (not) operator
- Logical `&&` (and) and `||` (or) operators

**Logical operators return values, not booleans:**

```risor
"" || "default"     // "default" - first truthy value
"hello" && "world"  // "world" - last value if all truthy
nil && expensive()  // nil - short-circuits, expensive() not called
```

## Enumeration

Risor has no loop constructs (`for`, `while`). Use recursion or higher-order
functions like `map()`, `filter()`, and `each()`.

Types implementing `Enumerable` can be used with spread expressions and
enumeration builtins like `keys()`, `values()`, `list()`, `sorted()`, and
`reversed()`.

### Enumeration Order

| Type     | Order                              | Key          | Value              |
| -------- | ---------------------------------- | ------------ | ------------------ |
| `list`   | Index order (0, 1, 2, ...)         | Index (int)  | Element            |
| `map`    | **Sorted by key** (alphabetically) | Key (string) | Value              |
| `string` | Byte order                         | Index (int)  | Character (string) |
| `bytes`  | Byte order                         | Index (int)  | Byte value         |
| `range`  | Arithmetic sequence                | Index (int)  | Generated integer  |

**Map enumeration is deterministic:** Keys are sorted alphabetically, not in
insertion order. This ensures reproducible behavior across runs.

```risor
keys({c: 3, a: 1, b: 2})  // ["a", "b", "c"] - sorted order
```

### Range

The `range` builtin creates a lazy sequence of integers (like Python 3):

```risor
range(5)           // 0, 1, 2, 3, 4
range(1, 5)        // 1, 2, 3, 4
range(0, 10, 2)    // 0, 2, 4, 6, 8
range(5, 0, -1)    // 5, 4, 3, 2, 1
```

Range objects are lazy - they don't allocate memory for all values upfront.
Convert to a list with `list(range(...))` when needed.

Attributes: `start`, `stop`, `step`

### Spread Expressions

Spread (`...`) uses enumeration order:

```risor
// Lists spread as values
[...[1, 2, 3]]  // [1, 2, 3]

// Maps spread as keys (sorted)
[...{b: 2, a: 1}]  // ["a", "b"]
```

## Error Handling

Risor uses a Python-like exception model where errors are values that can be
created, inspected, and thrown.

### Error Values vs Exceptions

**Errors are values:**

```risor
let err = error("file not found")  // Creates error value, does NOT throw
print(err.message())               // "file not found"
```

**Only `throw` triggers exception handling:**

```risor
throw error("something went wrong")  // Throws exception
throw "also works"                   // String converted to error
```

### Try/Catch/Finally

```risor
try {
    might_fail()
} catch e {
    print(e.message())  // e is an error value
} finally {
    cleanup()           // Always runs
}
```

**Catch block receives the error as a value.** You can inspect its attributes:

| Attribute    | Type       | Description                                       |
| ------------ | ---------- | ------------------------------------------------- |
| `message()`  | string     | Error message                                     |
| `kind()`     | string     | Error category (type, name, value, runtime, etc.) |
| `line()`     | int        | Line number (1-based, 0 if unknown)               |
| `column()`   | int        | Column number (1-based, 0 if unknown)             |
| `filename()` | string/nil | Source filename                                   |
| `source()`   | string/nil | Source line text                                  |
| `stack()`    | list       | Stack frames as maps                              |

### Error Propagation

1. **Operations that fail throw automatically:**

   ```risor
   let x = 1 + "foo"  // Throws type error
   ```

2. **Uncaught errors propagate up the call stack:**

   ```risor
   func inner() {
       throw "oops"
   }
   func outer() {
       inner()  // Error propagates through here
   }
   outer()      // Error reaches top level
   ```

3. **Finally blocks always run:**

   ```risor
   try {
       throw "error"
   } finally {
       print("runs")  // This prints
   }
   // Error re-thrown after finally
   ```

### Stack Traces

Stack traces capture the call chain from deepest frame to root:

```risor
try {
    func a() { b() }
    func b() { throw "error" }
    a()
} catch e {
    for frame in e.stack() {
        print(frame.function, frame.line)
    }
}
// Output:
// b 2
// a 1
// __main__ 4
```

Each stack frame contains:

- `function` - Function name (`"__main__"` for top level, `"<anonymous>"` for
  lambdas)
- `line` - Line number
- `column` - Column number
- `filename` - Source filename (if known)

### For Embedders

```go
result, err := risor.Eval(ctx, source, opts...)
if err != nil {
    switch e := err.(type) {
    case *errors.CompileError:
        // Compilation failed (syntax, etc.)
    case *errors.StructuredError:
        // Runtime error with location info
        fmt.Println(e.Message)
        fmt.Println(e.Kind)      // "type error", "runtime error", etc.
        fmt.Println(e.Location)  // File, line, column
        for _, frame := range e.Stack {
            fmt.Println(frame.Function, frame.Location.Line)
        }
    default:
        // Other error
    }
}
```
