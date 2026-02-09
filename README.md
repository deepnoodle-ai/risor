# Risor

[![CI](https://github.com/deepnoodle-ai/risor/actions/workflows/ci.yml/badge.svg)](https://github.com/deepnoodle-ai/risor/actions/workflows/ci.yml)
[![Go.Dev reference](https://img.shields.io/badge/go.dev-reference-blue?logo=go&logoColor=white)](https://pkg.go.dev/github.com/deepnoodle-ai/risor/v2)
[![Apache-2.0 license](https://img.shields.io/badge/License-Apache%202.0-brightgreen.svg)](https://opensource.org/licenses/Apache-2.0)

Risor is a fast, embeddable expression language for Go applications.

Add dynamic expressions, filters, and user-defined logic to your Go programs
without embedding a heavy runtime. Expressions compile to bytecode and run on a
lightweight VM. Pure Go, minimal dependencies.

```ts
// Expression evaluation: access control, validation rules
user.role == "admin" || resource.ownerId == user.id

// String templating: dynamic messages with embedded expressions
`Hello, ${user.name}! Your order ships ${order.shipDate}.`

// Configuration logic: safe navigation with fallbacks
config?.defaults?.theme ?? "light"

// Data transformation: filter, map, reduce
orders.filter(o => o.status == "pending").map(o => o.total).reduce((a, b) => a + b, 0)
```

## Install

```bash
go get github.com/deepnoodle-ai/risor/v2
```

## Go API

Embedding is straightforward. By default, the environment is empty (secure by
default). Add the standard library with `risor.Builtins()`:

```go
import "github.com/deepnoodle-ai/risor/v2"

env := risor.Builtins()
env["user"] = currentUser
env["resource"] = requestedResource

allowed, err := risor.Eval(ctx,
    `user.role == "admin" || resource.ownerId == user.id`,
    risor.WithEnv(env),
)
```

Go primitives, slices, and maps convert automatically.

## What Risor Isn't

Risor is not trying to compete with Python, Typescript, or other general purpose
scripting languages. Nor is it intended for writing large programs, hence there
is no package manager or module import mechanism in the language. That said,
there are several easy ways to customize and extend the environment and
built-in functions and types available to Risor expressions during execution.

## Documentation

- [Language Semantics](docs/guides/semantics.md) — Type coercion, equality, and iteration
- [Exception Handling](docs/guides/exceptions.md) — Try/catch/finally and error types
- [v1 to v2 Migration](docs/guides/migration-v2.md) — Upgrading from v1

## Risor v2

This is Risor v2, a major evolution focused on the embedded scripting use case.
Implementations of Risor v2 for use in Typescript and Rust programs is currently
being prototyped, with good initial results.

Risor v1 remains available at [tag v1.8.1](https://github.com/deepnoodle-ai/risor/releases/tag/v1.8.1).

## Contributing

- Questions and ideas: [GitHub Discussions](https://github.com/deepnoodle-ai/risor/discussions)
- Bugs and PRs: [GitHub Issues](https://github.com/deepnoodle-ai/risor/issues)
- Chat: `#risor` on [Gophers Slack](https://gophers.slack.com)

## License

[Apache License 2.0](./LICENSE)
