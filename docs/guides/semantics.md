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

```ts
5 == 5.0        // true - int equals float
5 == byte(5)    // true - int equals byte
5.0 == byte(5)  // true - float equals byte
```

Numeric types are compared by value after converting to a common representation.

**Strings and bytes:**

```ts
"hello" == "hello"               // true
bytes("hello") == bytes("hello") // true
bytes("hello") == "hello"        // true - bytes can equal string
```

**Containers** (deep equality):

```ts
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
| `null`     | Only equal to null                |
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
| `null`   | Only equal to null         | No          |

**Not comparable:** `map`, `function`, `builtin`, `module`

Comparing incompatible types throws a type error:

```ts
"hello" < 5     // type error
{} < {}         // type error - maps not comparable
```

### List Comparison

Lists are compared lexicographically:

1. Compare elements pairwise from index 0
2. First unequal pair determines the result
3. If all compared elements are equal, shorter list is less

```ts
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
| `null`   | Always                    |
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
- `match` guard expressions
- Logical `!` (not) operator
- Logical `&&` (and) and `||` (or) operators

**Logical operators return values, not booleans:**

```ts
"" || "default"     // "default" - first truthy value
"hello" && "world"  // "world" - last value if all truthy
null && expensive()  // null - short-circuits, expensive() not called
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

```ts
keys({c: 3, a: 1, b: 2})  // ["a", "b", "c"] - sorted order
```

### Range

The `range` builtin creates a lazy sequence of integers (like Python 3):

```ts
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

```ts
// Lists spread as values
[...[1, 2, 3]]  // [1, 2, 3]

// Maps spread as keys (sorted)
[...{b: 2, a: 1}]  // ["a", "b"]
```

## Map Methods

Maps have methods accessible via dot syntax. Methods take priority over keys
(Python-style shadowing).

### Available Methods

| Method | Signature | Returns | Description |
|--------|-----------|---------|-------------|
| `keys()` | `() → iter` | Iterator | Iterate over keys (sorted) |
| `values()` | `() → iter` | Iterator | Iterate over values |
| `entries()` | `() → iter` | Iterator | Iterate over [key, value] pairs |
| `each(fn)` | `(fn) → null` | null | Call fn(key, value) for each entry |
| `get(key, default?)` | `(key, default?) → any` | Value | Safe access with optional default |
| `pop(key, default?)` | `(key, default?) → any` | Value | Remove and return |
| `setdefault(key, val)` | `(key, value) → any` | Value | Set if missing, return final value |
| `update(other)` | `(map) → null` | null | Merge another map into this one |
| `clear()` | `() → null` | null | Remove all entries |
| `copy()` | `() → map` | Map | Shallow copy |

### Method Shadowing

Methods take priority over map keys. Use bracket syntax to access keys that
shadow method names:

```ts
let m = {keys: "my data", name: "Alice"}

m.keys()     // Method - returns iterator over ["keys", "name"]
m["keys"]    // Data - returns "my data"
m.name       // Data - returns "Alice" (no method named "name")
```

### Iterators

Methods like `keys()`, `values()`, and `entries()` return lazy iterators.
Iterators implement `Enumerable` and can be:

- Collected to a list with `list()`
- Spread into a list with `[...]`
- Iterated with `each()`

```ts
let m = {a: 1, b: 2, c: 3}

// Collect to list
list(m.keys())      // ["a", "b", "c"]
[...m.values()]     // [1, 2, 3]

// Iterate with callback
m.each((k, v) => print(k + "=" + string(v)))

// Chain operations
m.entries().each(([k, v]) => {
    print(k, v)
})
```

### Safe Access

Use `get()` for safe access with a default value:

```ts
let config = {host: "localhost"}

config.get("host")           // "localhost"
config.get("port")           // null (key missing)
config.get("port", 8080)     // 8080 (default used)

// Versus direct access
config.port                  // Error: attribute "port" not found
config["port"]               // Error: key "port" not found
```

## Map Shorthand Syntax

Maps support shorthand syntax when keys match variable names:

```ts
let name = "Alice"
let age = 30

// Shorthand: {name, age} is equivalent to {name: name, age: age}
let person = {name, age}  // {name: "Alice", age: 30}

// Mixed shorthand and explicit
let x = 1
let data = {x, y: 2, z: 3}  // {x: 1, y: 2, z: 3}
```

**Shorthand with defaults** (destructuring contexts only):

```ts
// In destructuring, = provides a default value
let {a, b = 10} = {a: 1}  // a == 1, b == 10
```

## Destructuring

Destructuring extracts values from maps and lists into variables.

### Object Destructuring

Extract properties from maps by key name:

```ts
let user = {name: "Alice", age: 30, city: "NYC"}

// Basic destructuring
let {name, age} = user  // name == "Alice", age == 30

// With alias (rename variable)
let {name: userName} = user  // userName == "Alice"

// With default value
let {role = "guest"} = user  // role == "guest" (not in user)

// Combined alias and default
let {status: userStatus = "active"} = user  // userStatus == "active"
```

### Array Destructuring

Extract elements from lists by position:

```ts
let pair = [10, 20]

// Basic destructuring - must match element count
let [x, y] = pair  // x == 10, y == 20

// Use _ to ignore elements
let coords = [10, 20, 30]
let [first, _, third] = coords  // first == 10, third == 30
let [head, _, _] = coords       // head == 10, ignore rest

// With default value (for missing elements)
let [a, b, c = 0] = pair  // a == 10, b == 20, c == 0
```

### Destructuring in Function Parameters

Functions can destructure arguments directly in the parameter list:

```ts
// Object destructuring parameter
function greet({name, age}) {
    return "Hello " + name + ", age " + string(age)
}
greet({name: "Alice", age: 30})  // "Hello Alice, age 30"

// With defaults
function connect({host = "localhost", port = 8080}) {
    return host + ":" + string(port)
}
connect({})                    // "localhost:8080"
connect({port: 3000})          // "localhost:3000"

// Array destructuring parameter
function sum([a, b]) {
    return a + b
}
sum([1, 2])  // 3

// Arrow functions with destructuring
let getX = ({x}) => x
getX({x: 42, y: 10})  // 42

let first = ([a, b]) => a
first([1, 2])  // 1
```

### Destructuring Semantics

**Missing keys:** Accessing a missing key yields `null` (or the default if provided).

```ts
let {missing} = {}           // missing == null
let {missing = "default"} = {}  // missing == "default"
```

**Extra keys:** Extra keys in the source are ignored.

```ts
let {a} = {a: 1, b: 2, c: 3}  // a == 1, b and c ignored
```

**Type requirements:**

- Object destructuring requires a `map` value
- Array destructuring requires a `list` value
- Type mismatch throws a runtime error

```ts
let {a} = [1, 2]     // runtime error: expected map
let [x] = {a: 1}     // runtime error: expected list
```

**Array count matching:** Array destructuring requires patterns to match list
length (use `_` to ignore elements, or defaults for missing elements).

```ts
let [a, b] = [1, 2, 3]      // runtime error: count mismatch
let [a, b, _] = [1, 2, 3]   // ok: a == 1, b == 2
let [a, b, c] = [1, 2]      // runtime error: count mismatch
let [a, b, c = 0] = [1, 2]  // ok: c gets default
```

## Error Handling

Risor uses a Python-like exception model with `try`, `catch`, `finally`, and
`throw`. Unlike Python, **`try` is an expression** that returns a value
(Kotlin-style semantics).

> **See [exceptions.md](exceptions.md) for comprehensive documentation.**

### Quick Overview

```ts
// Try is an expression - returns a value
let result = try { riskyOperation() } catch (e) { defaultValue }

// Returns try value on success, catch value on exception
let x = try { 42 } catch (e) { -1 }           // x == 42
let y = try { throw "err" } catch (e) { -1 }  // y == -1

// Finally runs but doesn't affect the return value
let z = try { 42 } finally { 999 }          // z == 42 (not 999)
```

### Error Attributes

| Attribute    | Type       | Description                                       |
| ------------ | ---------- | ------------------------------------------------- |
| `message()`  | string     | Error message                                     |
| `kind()`     | string     | Error category (type, name, value, runtime, etc.) |
| `line()`     | int        | Line number (1-based, 0 if unknown)               |
| `column()`   | int        | Column number (1-based, 0 if unknown)             |
| `filename()` | string/null | Source filename                                   |
| `source()`   | string/null | Source line text                                  |
| `stack()`    | list       | Stack frames as maps                              |

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
