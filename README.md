# Risor

[![CI](https://github.com/deepnoodle-ai/risor/actions/workflows/ci.yml/badge.svg)](https://github.com/deepnoodle-ai/risor/actions/workflows/ci.yml)
[![Apache-2.0 license](https://img.shields.io/badge/License-Apache%202.0-brightgreen.svg)](https://opensource.org/licenses/Apache-2.0)
[![Go.Dev reference](https://img.shields.io/badge/go.dev-reference-blue?logo=go&logoColor=white)](https://pkg.go.dev/github.com/deepnoodle-ai/risor)
[![Go Report Card](https://goreportcard.com/badge/github.com/deepnoodle-ai/risor?style=flat-square)](https://goreportcard.com/report/github.com/deepnoodle-ai/risor)
[![Releases](https://img.shields.io/github/release/deepnoodle-ai/risor/all.svg?style=flat-square)](https://github.com/deepnoodle-ai/risor/releases)

Risor is a fast, embedded scripting language for Go applications. Scripts
compile to bytecode and run on a lightweight virtual machine. Risor is written
in pure Go with minimal dependencies.

## Risor v2

Risor v2 is a major evolution of the language and library. This release
refocuses Risor on core language features, usability, and the embedded
scripting use case, while moving away from devops and system scripting.

Risor v2 draws more inspiration from TypeScript, with arrow functions,
optional chaining, destructuring, and functional iteration patterns. For loops
are removed in favor of `map`, `filter`, and `reduce`. Most I/O modules were
removed in favor of secure, isolated execution by default. Host applications
selectively expose functionality to scripts as needed.

These are breaking changes. Now is the time for feedback!

Risor v1 remains available at [tag v1.8.1](https://github.com/deepnoodle-ai/risor/releases/tag/v1.8.1).
Critical fixes will be made on a v1 branch as needed.

## Documentation

Documentation for v1 is available at [risor.io](https://risor.io). Updated
documentation for v2 is coming soon.

See `docs/` in this repository for v2 documentation including:
- [Migration Guide](docs/v1-to-v2-migration.md) — Upgrading from v1 to v2
- [Language Semantics](docs/semantics.md) — Type coercion, equality, and iteration
- [Exception Handling](docs/exceptions.md) — Try/catch/finally and error types

## Syntax Example

Risor v2 moves away from Go-inspired syntax toward TypeScript. For loops are
removed in favor of functional patterns like `map`, `filter`, and `reduce`:

```ts
// Arrow functions and list operations
let numbers = [1, 2, 3, 4, 5]
let doubled = numbers.filter(x => x > 2).map(x => x * 2)  // [6, 8, 10]

// Destructuring with defaults
let { name, age = 0 } = { name: "Alice", age: 30 }

// Optional chaining and nullish coalescing
let city = user?.address?.city ?? "Unknown"

// Closures
function makeCounter() {
    let count = 0
    return () => { count += 1; return count }
}
let counter = makeCounter()
counter()  // 1
counter()  // 2

// Try/catch error handling
try {
    riskyOperation()
} catch e {
    print("Error:", e.message())
}
```

## Go Interface

Embedding Risor in your Go program is straightforward. By default, the
environment is empty (secure by default). Use `risor.Builtins()` to get the
standard library:

```go
result, err := risor.Eval(ctx, "math.min([5, 2, 7])", risor.WithEnv(risor.Builtins()))
// result is 2
```

Provide input to the script using `WithEnv`:

```go
env := risor.Builtins()
env["input"] = 16
result, err := risor.Eval(ctx, "input | math.sqrt", risor.WithEnv(env))
// result is 4.0
```

Primitive types, slices, and maps are automatically converted between Go and
Risor. For more control, use custom builtins or modules.

## Adding Custom Modules

Risor is designed to have minimal external dependencies in its core. You can add
custom modules to the environment when embedding Risor:

```go
import (
    "github.com/deepnoodle-ai/risor"
    "github.com/deepnoodle-ai/risor/object"
)

func main() {
    env := risor.Builtins()
    env["custom"] = object.NewBuiltinsModule("custom", map[string]object.Object{
        "hello": object.NewBuiltin("hello", myHelloFunc),
    })
    result, err := risor.Eval(ctx, source, risor.WithEnv(env))
    // ...
}
```

## Contributing

Risor is a community project. You can lend a hand in various ways:

- Please ask questions and share ideas in [GitHub discussions](https://github.com/deepnoodle-ai/risor/discussions)
- Share Risor on any social channels that may appreciate it
- Open a GitHub issue or pull request for any bugs you find
- Star the project on GitHub

## Community Projects

These community projects were built for Risor v1 and may need updates for v2:

- [RSX: Package Risor Scripts into Go Binaries](https://github.com/rubiojr/rsx)
- [Awesome Risor](https://github.com/rubiojr/awesome-risor)
- [tree-sitter-risor](https://github.com/applejag/tree-sitter-risor)
- [bench_go_scripting](https://github.com/mna/bench_go_scripting)

## Discuss the Project

Please visit the [GitHub discussions](https://github.com/deepnoodle-ai/risor/discussions)
page to share thoughts and questions.

There is also a `#risor` Slack channel on the [Gophers Slack](https://gophers.slack.com).

## Credits

Check [CREDITS.md](./CREDITS.md).

## License

Released under the [Apache License, Version 2.0](./LICENSE).

Copyright Curtis Myzie / [github.com/myzie](https://github.com/myzie).
