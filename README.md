# Risor

[![CI](https://github.com/deepnoodle-ai/risor/actions/workflows/ci.yml/badge.svg)](https://github.com/deepnoodle-ai/risor/actions/workflows/ci.yml)
[![Go.Dev reference](https://img.shields.io/badge/go.dev-reference-blue?logo=go&logoColor=white)](https://pkg.go.dev/github.com/deepnoodle-ai/risor/v2)
[![Apache-2.0 license](https://img.shields.io/badge/License-Apache%202.0-brightgreen.svg)](https://opensource.org/licenses/Apache-2.0)

Risor is a fast, embeddable scripting language for Go applications.

Add dynamic expressions, filters, and user-defined logic to your Go programs
without embedding a heavy runtime like V8 or Python. Expressions compile to
bytecode and run on a lightweight VM. Pure Go, minimal dependencies.

Risor fills the gap between hardcoded logic and a full language runtime. It's
for Go developers who need to evaluate user-provided expressions, rules, or
small scripts safely at runtime.

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

## Use Cases

Risor is designed for scenarios where a Go application needs to evaluate
user-provided or externally-defined logic at runtime:

- **Workflow engines** — Evaluate conditions and run small scripts at each step to decide branching, transform data, or trigger actions
- **Rules engines** — Express business rules like `order.total > 100 && customer.tier == "gold"` that change frequently without redeployment
- **Expression evaluation** — Dynamic filters, computed fields, validation rules, access control predicates
- **String templating** — Interpolated strings with embedded expressions for notifications, reports, emails
- **Configuration logic** — When static config (YAML, JSON) isn't enough but a full language is too much
- **Data transformation** — User-defined transforms, enrichment, and filtering in data pipelines
- **Plugin & extension systems** — Let users or tenants write small scripts that customize application behavior

The common pattern: a compiled Go host handles performance-critical work and
exposes an API surface to Risor for flexible, safe, easy-to-change logic.

## What Risor Isn't

Risor is not a general-purpose programming language. It's not trying to replace
Python or TypeScript for writing applications. There is no package manager,
no module imports, and no third-party ecosystem — by design.

Extension happens through Go code: you add builtin functions, pass data into the
script context, and read results back out. This keeps the core small and lets
each application tailor Risor to its needs.

## Documentation

- [Language Semantics](docs/guides/semantics.md) — Type coercion, equality, and iteration
- [Exception Handling](docs/guides/exceptions.md) — Try/catch/finally and error types
- [v1 to v2 Migration](docs/guides/migration-v2.md) — Upgrading from v1

## Risor v2

This is Risor v2, a major evolution focused entirely on the embedded scripting
use case. v1 also served DevOps scripting, cloud tooling, and CLI usage. v2
narrows the focus to doing one thing well: giving Go developers a safe, fast way
to evaluate user-provided expressions and scripts at runtime.

Implementations for TypeScript and Rust host programs are being prototyped, with
good initial results.

Risor v1 remains available at [tag v1.8.1](https://github.com/deepnoodle-ai/risor/releases/tag/v1.8.1).

## Contributing

- Questions and ideas: [GitHub Discussions](https://github.com/deepnoodle-ai/risor/discussions)
- Bugs and PRs: [GitHub Issues](https://github.com/deepnoodle-ai/risor/issues)
- Chat: `#risor` on [Gophers Slack](https://gophers.slack.com)

## License

[Apache License 2.0](./LICENSE)
