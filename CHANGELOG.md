# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/).

## [Unreleased]

### Added

- Method calls accept spread arguments: `obj.method(...args)` and
  `obj?.method(...args)` (#489).
- **`risor.WithLimits`** caps the string, bytes, or list value that one
  size-amplifying operation can produce, such as `repeat`, `join`, `+`,
  `split`, spread, or `list(range(...))`. Operations that add one item, such as
  `append`, are not capped. Defaults: 64 MiB and 4,194,304 elements. Exceeding a
  cap raises a catchable `value error` (#491).
  The list cap also bounds the total elements of one conversion to Go, such as
  a script result, the arguments of a host function call, or `sprintf` and
  `encode` input, counting a shared list each time it appears (#496).

### Changed

- Release archives and the Homebrew cask now include `risor-lsp`, the Risor
  language server, next to `risor`. The language server writes its log to the
  user cache directory instead of the working directory.
- `vm.Get` returns `object.Nil` for a global that is not yet assigned, instead
  of a Go nil (#487).
- A spread call to a builtin with more than 256 arguments now fails with a
  catchable "max args limit" error, as calls to Risor functions already did
  (#487).
- The parser now reports more inputs as parse errors: an operator chain (such
  as `a.b.c`, `f()()` or `1 + 2 + 3`) or an `else if` chain longer than the
  depth limit, a guard on the default match arm `_`, and an empty template
  interpolation `${}`. The depth limit is 500 by default; raise it with
  `parser.Config.MaxDepth`. Before, a long chain could crash the host with a Go
  stack overflow (#488).
- Go API: to apply `WithLimits`, these now take a `context.Context` as their
  first argument: `object.BinaryOp`, `String.Join`, `Repeat`, `ReplaceAll`,
  `Split`, and `Fields`, and `Bytes.Repeat`, `Replace`, and `ReplaceAll`.
  `String.Fields` also returns an error (#491).
- `object.ToInterface` and `TypeRegistry.ToGo` take a `context.Context`, whose
  limits bound the conversion. `ToGo` converts a range to a slice or array.
  New `object.ToInterfaces` converts several values with one shared budget
  (#496).
- Numeric operators and comparisons:
  - A negative shift count (`1 << -1`, `byte(1) >> -1`) raises a catchable
    `value error`. Before, an int shift returned 0 and a byte shift crashed
    with a Go runtime error.
  - `int ** float` returns a float: `2 ** 0.5` is `1.4142135623730951`. Before,
    the result was cut to an int (`1`).
  - `int == float` and int/float ordering compare exactly. Before, the int was
    converted to float64 first, which rounds above 2^53, so
    `9007199254740993 == 9007199254740992.0` was true.
  - Every ordered comparison with NaN (`<`, `<=`, `>`, `>=`) is false, as in
    IEEE 754. Before, `math.nan < 1.0` and `1.0 < math.nan` were both true.
    `sorted` and `list.sort` put NaN after all other numbers.
  - `math.min` and `math.max` return NaN when any argument is NaN. Before,
    the result depended on the argument order.
- A string and bytes compare the same way in both directions:
  `"a" == bytes("a")` is now true, like `bytes("a") == "a"`, and `<`, `>`,
  `in`, and `list.index` work with a string on either side.

### Fixed

- `vm.Get`, `vm.GlobalNames`, `Code.GlobalNames`, the globals that `RunCode`
  carries over, and `object.NewModule` attribute lookup now use the root
  (top-level) variable when a variable declared in a block, such as in an `if`
  or `try` block, has the same name.
  Before, `vm.Get("foo")` could return the block value, and so could the REPL.
  A block variable is not a global, so `vm.Get` does not find it, and a block
  variable at the top level of a module does not resolve as `module.name`.
  `Code.GlobalNames` no longer has one entry per slot, so its indexes do not
  always match `GlobalNameAt`. To read every slot name, use `GlobalCount` and
  `GlobalNameAt`. An env key of the form `name#digits`, such as `step#1`, is
  not visible to `vm.Get` or `GlobalNames` (#492).

- The VS Code extension no longer runs a `go install` command that always
  failed. When it cannot find `risor-lsp`, it shows install steps and a link to
  the `risor.languageServerPath` setting.
- `try`/`catch`/`finally` now unwinds correctly at any depth. An exception
  thrown from a deep call is caught by the right handler, with the operand stack
  restored. A `return` now runs every enclosing `finally` block, and an error
  thrown from `finally` replaces an error thrown from `catch`. Active exception
  handlers are capped at 4 per allowed call frame (4096 by default); past the
  cap, the script gets a catchable stack overflow error (#486).
- A value stack overflow now returns `ErrStackOverflow` instead of an "index
  out of range" panic. Reading a global before it is assigned, a function with
  255 parameters and a rest parameter, and `vm.Call` before `vm.Run` no longer
  crash the host (#487).
- A misplaced `...` (such as `let a = ...x` or `[...a + b]`) is now a syntax
  error. So is a default in a list or map literal (such as `[a = 1]`), except
  in an arrow function parameter like `([a = 1]) => a`. Before, these caused a
  compiler panic. Write `[...(a + b)]` to spread an expression (#489).
- Cyclic lists and maps no longer crash the host with a stack overflow.
  Printing one shows `[...]` or `{...}` at the cycle. Converting it to a Go
  value, encoding it as JSON, or formatting it with `sprintf` returns a nesting
  error, as does any value nested more than 128 levels deep. `==` on a cyclic
  value returns `false` (#490).
- `repeat`, `range`, `list(range(...))`, spread, `+`, `rand.bytes`, and similar
  operations no longer crash the host with huge allocations. `range`,
  `rand.int`, and `math.abs` no longer overflow int64 (#491).
- Setting a field of a Go struct from a script now converts the value with the
  limits from `risor.WithLimits`, not the defaults. This applies only to a
  value whose type is exactly `*object.GoStruct`. Any other type, including one
  that embeds `*object.GoStruct`, uses its `SetAttr` (its own or the promoted
  one) and the default limits, as before (#497).

## [2.2.0] - 2026-08-17

### Added

- **Variadic method definitions** — Go embedders can use
  `AttrBuilder.Variadic(name)` to define object methods that accept zero or more
  trailing arguments (#467).

### Changed

- The VS Code extension now requires VS Code 1.91 or newer and uses the current
  language client and TypeScript toolchain (#475).

### Fixed

- Error equality (`==`) now matches a wrapped error against its underlying
  sentinel, so `err == fs.err_not_exist` works when `err` was returned from a
  module that wraps an inner error. The previous behavior compared only error
  message strings, so wrapped sentinels never matched (#463).
- Closures with more than eight local variables now preserve their captured
  values when a VM frame is reused for another function call (#462).

### Documentation

- Added a practical guide to memory limits, including isolation boundaries,
  in-process safeguards, and the limits of timeout and step-count controls
  (#474).

### Dependencies

- Updated Go and VS Code extension dependencies, clearing the repository's
  known dependency security alerts at release preparation time (#475).

### Notes

- Error message wrapping (e.g. via `%w` in the `error()` builtin) is an
  implementation detail and not part of the stable scripting API. The
  underlying chain semantics may change in a future major release.

## [2.1.0] - 2026-03-10

### Added

- **null keyword** — first-class null literal for explicit null values (#445)
- **TypeScript-style catch syntax** — `catch (e)` binds the error variable (#445)
- **Range iteration methods** — `.each()`, `.map()`, `.filter()` on range objects (#444)
- **--var CLI flag** — pass variables to scripts from the command line (#454)

### Fixed

- Fix JSON output formatting in CLI (#454)
- Fix `string(byte)` type conversion (#444)

### Dependencies

- Bump minimatch to 3.1.5 in VSCode extension (#451, #452)
- Bump @tootallnate/once and @vscode/test-electron in VSCode extension (#453)

## [2.0.0] - 2026-02-09

Risor v2 is a major release focused on the embedded scripting use case. It
introduces an isolated-by-default sandbox, TypeScript-aligned syntax, and a
streamlined Go API.

See [v1 to v2 Migration Guide](docs/guides/migration-v2.md) for upgrade details.

### Added

- **Arrow functions** — concise lambdas: `x => x * 2`, `(a, b) => a + b`
- **Optional chaining** — safe property access: `user?.profile?.name`
- **Nullish coalescing** — defaults: `value ?? "fallback"`
- **try/catch/finally** — keyword-based exception handling as expressions
- **throw** — explicit exception throwing
- **Match expressions** — pattern matching with guard expressions
- **Spread operator** — `{...a, ...b}`, `[...a, ...b]`
- **Destructuring** — `let {name, age} = obj`, `let [a, b] = list`
- **Template strings** — `` `Hello, ${name}!` ``
- **range() builtin** — lazy integer sequences: `range(10)`, `range(1, 10, 2)`
- **error() builtin** — create error values: `error("not found: %s", id)`
- **Map methods** — Python-style methods: `.keys()`, `.values()`, `.entries()`, `.get(key, default)`
- **GoFunc and GoStruct** — Go interop types for embedding
- **TypeRegistry** — explicit type conversion for Go types
- **RisorValuer interface** — automatic Go-to-Risor conversion
- **Resource limits** — step limits, stack depth, and timeouts
- **Execution observer** — hook into VM execution for profiling and debugging
- **Pipe expressions** — `data |> transform |> filter`

### Changed

- **Isolated by default** — empty environment unless explicitly configured
- **Parentheses required for if** — `if (condition)` instead of `if condition`
- **Callable returns `(Object, error)`** — explicit error returns throughout
- **BuiltinFunction returns `(Object, error)`** — explicit error returns
- **Object.Equals returns `bool`** — instead of Object
- **Parser/Compiler use Config structs** — replaces functional options
- **byte_slice renamed to bytes**
- **Only 3 built-in modules** — math, rand, regexp

### Removed

- **For loops** — use functional iteration: `.each()`, `.map()`, `.filter()`, `.reduce()`
- **Defer** — use `try/finally`
- **Concurrency primitives** — channels, `go` keyword, `spawn`
- **Import statements** — all functionality comes from the environment
- **Switch/case** — use `match` expressions or if/else
- **Set literals** — use lists
- **Hash comments** — use `//` (shebang still supported)
- **I/O modules** — os, http, exec, ssh, dns, net, bcrypt, filepath, errors, fmt
- **Proxy type** — use TypeRegistry or RisorValuer
- **buffer, set, float_slice types** — use bytes and list
- **try() builtin** — use try/catch
- **delete() builtin** — removed, no replacement

## [1.8.1] - 2025-01-15

Final v1 release. See [v1.8.1 on GitHub](https://github.com/deepnoodle-ai/risor/releases/tag/v1.8.1).
