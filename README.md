# Risor

[![CI](https://github.com/deepnoodle-ai/risor/actions/workflows/ci.yml/badge.svg)](https://github.com/deepnoodle-ai/risor/actions/workflows/ci.yml)
[![Go.Dev reference](https://img.shields.io/badge/go.dev-reference-blue?logo=go&logoColor=white)](https://pkg.go.dev/github.com/deepnoodle-ai/risor)
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
user?.settings?.theme ?? config?.defaults?.theme ?? "light"

// Data transformation: filter, map, reduce
orders.filter(o => o.status == "pending").map(o => o.total).reduce((a, b) => a + b, 0)
```

## Go API

Embedding is straightforward. By default, the environment is empty (secure by
default). Add the standard library with `risor.Builtins()`:

```go
env := risor.Builtins()
env["user"] = currentUser
env["resource"] = requestedResource

allowed, err := risor.Eval(ctx, `user.role == "admin" || resource.ownerId == user.id`, risor.WithEnv(env))
```

Go primitives, slices, and maps convert automatically.

## What Risor Isn't

**Not a general-purpose language.** Risor isn't trying to replace Python or
TypeScript. It's a tool for adding flexibility to Go applications.

**Not for large programs.** Risor handles longer scripts fine, but the sweet
spot is expressions and small scripts.

**Not an ecosystem.** No package manager, no plugin system. Extension happens
through Go code, not Risor code.

## Documentation

- [Language Semantics](docs/semantics.md) — Type coercion, equality, and iteration
- [Exception Handling](docs/exceptions.md) — Try/catch/finally and error types
- [v1 to v2 Migration](docs/v1-to-v2-migration.md) — Upgrading from v1

## Risor v2

This is Risor v2, a major evolution focused on the embedded scripting use case.
The syntax draws from TypeScript: arrow functions, optional chaining,
destructuring, and functional iteration. For loops are gone in favor of `map`,
`filter`, and `reduce`.

Risor v1 remains available at [tag v1.8.1](https://github.com/deepnoodle-ai/risor/releases/tag/v1.8.1).

## Contributing

- Questions and ideas: [GitHub Discussions](https://github.com/deepnoodle-ai/risor/discussions)
- Bugs and PRs: [GitHub Issues](https://github.com/deepnoodle-ai/risor/issues)
- Chat: `#risor` on [Gophers Slack](https://gophers.slack.com)

## License

[Apache License 2.0](./LICENSE)
