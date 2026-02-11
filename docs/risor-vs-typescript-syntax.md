# Where Risor Syntax Diverges from TypeScript

Risor is _almost_ a subset of TypeScript. Most Risor code looks like it could be
valid TypeScript — `let`, `const`, arrow functions, template literals,
destructuring, optional chaining, and nullish coalescing all work in both
languages. This document catalogs every place where valid Risor syntax is **not**
valid TypeScript syntax.

## Summary

| Category | Divergence | Severity |
|---|---|---|
| Keywords | `not` as prefix operator | No TS equivalent keyword |
| Operators | `not in` membership test | Not valid TS (`in` alone is shared) |
| Operators | `\|>` pipe operator | Not in TS (TC39 Stage 2) |
| Operators | `**` with Python semantics | Minor: `-2 ** 2` is a TS syntax error |
| Expressions | `if` as expression | TS `if` is a statement |
| Expressions | `try/catch` as expression | TS `try` is a statement |
| Expressions | `match` expression | Not in TS (no pattern matching) |
| Expressions | Block-as-expression | `{ let a = 1; a }` is not a TS expression |
| Statements | `let x, y = [1, 2]` multi-var | Not valid TS destructuring syntax |
| Literals | `052` octal format | TS strict mode requires `0o52` |
| Literals | Unquoted map keys are identifiers | TS object keys and Risor map keys have different semantics |
| Missing features | No `for`, `while`, `do` loops | TS has all three |
| Missing features | No `class`, `interface`, `enum`, `type` | Core TS constructs absent |
| Missing features | No `import`/`export` | TS module system absent |
| Missing features | No type annotations | TS's raison d'etre |
| Missing features | No `async`/`await` | TS async model absent |
| Missing features | No `switch`/`case` | Risor uses `match` instead |
| Missing features | No `void`, `undefined`, `never`, `unknown` | TS-specific types |
| Builtins | Snake_case methods on primitives | `.to_upper()` vs `.toUpperCase()` |

---

## 1. Keywords

### `not` prefix operator

Risor supports `not` as a keyword alternative to `!`. TypeScript has no `not`
keyword.

```
// Risor
if (not ready) { ... }

// TypeScript equivalent
if (!ready) { ... }
```

### `struct` reserved keyword

Risor reserves `struct` as a keyword. TypeScript does not have `struct` (it's not
a JS/TS keyword).

---

## 2. Operators

### `not in` membership test

Both Risor and TypeScript have the `in` binary operator (`x in y`), so `in`
itself is syntactically valid in both — only the runtime semantics differ (TS
checks object properties, Risor checks membership in lists/maps/sets/strings).

However, Risor also has `not in` as a combined operator. TypeScript has no
`not in` form.

```
// Risor
if (x not in list) { ... }

// TypeScript — no equivalent operator
if (!(x in obj)) { ... }
```

### `|>` pipe operator

Risor has a first-class pipe operator. TypeScript does not (the TC39 pipe
proposal is Stage 2 and not part of the language).

```
// Risor
5 |> double |> addOne

// TypeScript — not valid syntax
```

### `**` exponentiation with Python semantics

Both Risor and TypeScript support `**`. However, Risor follows Python's semantics
where `-2 ** 2` equals `-(2 ** 2)` = `-4`. TypeScript/JavaScript makes
`-2 ** 2` a **syntax error** — you must write `(-2) ** 2` or `-(2 ** 2)`
explicitly.

---

## 3. Expressions vs Statements

This is the largest category of divergence. Risor makes several constructs into
expressions that are statements in TypeScript.

### `if` as expression

In Risor, `if/else` returns a value. In TypeScript, `if` is a statement and
cannot appear on the right side of an assignment.

```
// Risor — valid
let label = if (x > 0) { "positive" } else { "non-positive" }

// TypeScript — syntax error
// Must use ternary: let label = x > 0 ? "positive" : "non-positive"
```

### `try/catch` as expression

Risor's `try/catch/finally` returns a value. TypeScript's `try` is a statement.

```
// Risor — valid
let result = try { riskyOp() } catch (e) { defaultValue }

// TypeScript — syntax error
```

Note: Risor uses the same `catch (e) { }` syntax as TypeScript. The divergence
is only that `try/catch` is an expression (returns a value) in Risor.

### `match` expression

Risor has `match` with `=>` arms and a required `_` wildcard default. TypeScript
has no `match` keyword (it uses `switch/case` with entirely different syntax).

```
// Risor
let day_type = match day {
    "Saturday" => "weekend"
    "Sunday" => "weekend"
    _ => "weekday"
}

// TypeScript — no equivalent construct
// Closest: switch statement (which is a statement, not an expression)
```

Match arms can also have guard clauses (`pattern if condition => result`), which
have no TypeScript parallel.

### Block as expression

In Risor, a block `{ ... }` can be an expression whose value is the last
expression in the block. TypeScript blocks are statements.

```
// Risor — valid
let x = {
    let a = 1
    let b = 2
    a + b
}
// x == 3

// TypeScript — the {} would be parsed as a block statement, not an expression
```

---

## 4. Declaration Syntax

### Multi-variable `let`

Risor supports `let x, y = [1, 2]` to destructure a list into multiple
variables. TypeScript requires standard destructuring syntax.

```
// Risor
let x, y = [1, 2]

// TypeScript equivalent
let [x, y] = [1, 2]
```

The Risor form is not valid TypeScript (`let x, y = [1, 2]` would declare `x` as
undefined and `y` as an array in TS, if it compiled at all).

---

## 5. Literal Syntax

### Octal number format

Risor uses C-style octal literals (`052`). TypeScript strict mode (and all
TypeScript by default) forbids legacy octal literals and requires the `0o` prefix.

```
// Risor
let x = 052    // octal 42

// TypeScript
let x = 0o52   // octal 42
// 052 is a syntax error in strict mode
```

### Map literal key semantics

Risor map literals use `{key: value}` syntax that looks identical to TypeScript
object literals, but Risor also supports:

- **Shorthand with `=`**: `{a = 1, b = 2}` in destructuring contexts within map
  literals. This is not valid TypeScript object literal syntax.

- **Identifier keys are always strings**: In Risor, `{a: 1}` creates a map with
  string key `"a"`. This matches TypeScript's behavior for simple cases, but
  Risor maps have built-in methods (`.keys()`, `.values()`, `.entries()`,
  `.get()`, `.each()`) that shadow any keys with those names — a semantic
  difference that would cause different runtime behavior.

---

## 6. Features Risor Has That TypeScript Lacks

These are Risor features with no TypeScript syntax equivalent:

| Risor Feature | Notes |
|---|---|
| `error("msg")` builtin | TS uses `new Error("msg")` |
| `throw` without `new` | Risor: `throw error("msg")`, TS: `throw new Error("msg")` |
| `x++` / `x--` as statements only | Same syntax, but Risor restricts to same-line usage |
| Shebang `#!/usr/bin/env risor` | Not valid TS (though some runtimes strip it) |

---

## 7. Features TypeScript Has That Risor Lacks

The absence of these features means TypeScript code using them cannot run in
Risor, but this doesn't affect whether _Risor code_ is valid TS. Listed for
completeness:

- **Type annotations**: `let x: number = 5`
- **Loops**: `for`, `for...of`, `for...in`, `while`, `do...while`
- **Classes**: `class`, `extends`, `implements`, `super`, `this`
- **Modules**: `import`, `export`, `from`
- **Async**: `async`, `await`, `Promise`
- **Enums**: `enum`
- **Type constructs**: `interface`, `type`, `as`, `is`, `keyof`, `typeof` (type context), `infer`, `never`, `unknown`, `void`, `undefined`
- **Switch**: `switch`, `case`, `default`, `break`
- **Ternary operator**: `? :` — Risor uses `if` expressions instead
- **`new` keyword**: No constructor invocation
- **Generators**: `function*`, `yield`
- **Labels and goto**: `break label`, `continue label`
- **Comma operator**: `(a, b)` as expression (Risor parses this as arrow function params)

---

## 8. Standard Library / Method Names

Risor uses snake_case for built-in methods on primitives. These are valid method
call syntax in both languages, but the method names don't exist in TypeScript's
standard library:

| Risor | TypeScript |
|---|---|
| `"hello".to_upper()` | `"hello".toUpperCase()` |
| `"hello".to_lower()` | `"hello".toLowerCase()` |
| `"  hi  ".trim_space()` | `"  hi  ".trim()` |
| `"hello".has_prefix("he")` | `"hello".startsWith("he")` |
| `"hello".has_suffix("lo")` | `"hello".endsWith("lo")` |
| `"ab:cd".split(":")` | `"ab:cd".split(":")` (same!) |
| `list.each(fn)` | `list.forEach(fn)` |
| `list.filter(fn)` | `list.filter(fn)` (same!) |
| `list.map(fn)` | `list.map(fn)` (same!) |
| `list.reduce(init, fn)` | `list.reduce(fn, init)` (arg order swapped) |

This is a runtime/semantic concern rather than a syntax concern — the code
_parses_ the same way, it just wouldn't have the right methods at runtime.

---

## 9. What IS the Same

For reference, this extensive set of syntax is identical in both languages:

- `let` and `const` declarations
- Arrow functions: `(x) => x * 2`, `x => x + 1`, `() => 42`
- Template literals with `${expr}` interpolation
- Object/array destructuring: `let { a, b } = obj`, `let [x, y] = arr`
- Spread operator: `[...a, ...b]`, `{...a, ...b}`, `f(...args)`
- Rest parameters: `function f(...args) { }`
- Default parameters: `function f(x = 10) { }`
- Optional chaining: `obj?.prop`, `obj?.method()`
- Nullish coalescing: `a ?? b`
- All arithmetic: `+`, `-`, `*`, `/`, `%`, `**`
- All comparison: `==`, `!=`, `<`, `<=`, `>`, `>=`
- Logical operators: `&&`, `||`, `!`
- Bitwise operators: `&`, `|`, `^`, `<<`, `>>`
- Compound assignment: `+=`, `-=`, `*=`, `/=`
- Postfix: `x++`, `x--`
- Property access: `obj.prop`, `obj["prop"]`, `arr[0]`
- Function declarations: `function name(a, b) { return a + b }`
- Anonymous functions: `let f = function(x) { return x }`
- Single and double quoted strings
- Comments: `//` and `/* */`
- `true`, `false`, `null`
- `return`, `throw`
- `try/catch/finally` with `catch (e) { }` syntax (structure, not expression-ness)

---

## 10. Assessment: How Close Is Risor to a TypeScript Subset?

Risor is remarkably close. The core expression and declaration syntax is
intentionally TypeScript-compatible. Risor uses `null` (same as TypeScript) and
`catch (e) { }` (same as TypeScript). The remaining divergences fall into a few
categories:

1. **Expression-oriented design** (4 items): `if`, `try`, `match`, and blocks as
   expressions. This is a fundamental design choice — Risor is expression-oriented
   while TypeScript is statement-oriented. This is the deepest divergence.

2. **Missing constructs** (2 items): `not` keyword and `not in` operator. Python
   influence.

3. **Pipe operator** (1 item): `|>` is not in TypeScript. However, it's a
   popular TC39 proposal and widely understood.

4. **Minor syntax differences** (1 item): legacy octal literals.

If the goal were to make Risor a strict TypeScript subset, the changes would be:
- Remove `not` keyword (keep `!`)
- Remove `not in` (use `!(x in y)`)
- Remove `|>` pipe operator
- Use `0o` prefix for octal
- Make `if`/`try`/`match`/blocks into statements (biggest impact — would require
  adding ternary `? :` as compensation)
- Remove `let x, y = expr` multi-var syntax
