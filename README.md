# Risor

**A small, typed expression language you can embed in Go, TypeScript and
Rust. It is checked before it runs, and it cannot run away with your CPU or
memory.**

[![CI](https://github.com/deepnoodle-ai/risor/actions/workflows/ci.yml/badge.svg)](https://github.com/deepnoodle-ai/risor/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/deepnoodle-ai/risor/go/v3.svg)](https://pkg.go.dev/github.com/deepnoodle-ai/risor/go/v3)
[![License](https://img.shields.io/badge/license-Apache--2.0-blue.svg)](LICENSE)

> **Risor v3 is an alpha and a request for comment.** The language, the
> specification and the APIs will change before 3.0.
> Looking for Risor v2? It is released and maintained on the
> [`v2` branch](https://github.com/deepnoodle-ai/risor/tree/v2). Existing
> imports of `github.com/deepnoodle-ai/risor/v2` keep working.

Your application has data. Your users, your operators or an LLM want to
write a little logic over it. Risor is the language for that logic. Your
program gives it data and gets a value back.

```js
// A rule
order.total > 100 && order.country == "US"

// A template
`Hello, ${user.name}!`

// A query
users.filter(u => u.age >= 18).map(u => u.name)
```

If you know JavaScript or TypeScript, you can read Risor already. A longer
program names its values with `const` and ends with one expression:

```js
const adults = users.filter(u => u.age >= 18);
`${len(adults)} of ${len(users)} users are adults`
```

## Why Risor

**Native in three languages.** Risor has three independent
implementations, one each in Go, TypeScript and Rust. None of them is a
binding to another language, and none uses WebAssembly or cgo. Each is
idiomatic in its own language, and none of the three libraries has a
run-time dependency. The same program gives the same result in all three.

**Bounded CPU and memory.** Each run has a limit on steps, on allocation,
on wall-clock time and on call depth. These limits are part of the
language. The step and allocation counts are exact, so a successful run
reports the same usage in Go, TypeScript and Rust. You can run code that you
did not write, and meter it.

**Checked before it runs.** A static checker is part of the language. A
program that passes the checker cannot fail with an unknown name, a missing
field, an unexpected null or a value of the wrong kind.

**Small.** No statements, no loops, no reassignment. `if`, `match` and
`try` are expressions. Values cannot be changed, and host data is read in
place, with no copy.

**Defined by a specification.** The language is defined by
[`spec/`](spec) and by a conformance suite of data files in
[`suite/`](suite). All three implementations pass the Core suite.

## Catch mistakes before they run

The checker knows the shape of your data. Say `user` is
`{name: string, age: int, email: string | null}`:

```text
user.nmae

error[no-field] 1:6-1:10: {age: int, email: string | null, name: string} has no field "nmae"
```

```text
user.email.toLowerCase()

error[maybe-null] 1:1-1:11: user.email may be null; its type is string | null
```

Write `user.email?.toLowerCase() ?? "none"` and the checker accepts it.
You find the mistake when the rule is saved, not when it runs.

## Run code you did not write

A run that passes a limit stops with an error that names the limit:

```text
range(0, 10000000)

error[steps-exceeded] 1:1-1:19: the run needed more than its limit of 1000000 steps
```

| Limit | Default | Bounds |
| --- | --- | --- |
| Steps | 1,000,000 | CPU: the work a run does |
| Allocation | 1,000,000 units | Memory: the values a run makes |
| Deadline | Off | Wall-clock time |
| Depth | 200 | Recursion and nesting |

Steps, allocation and depth are deterministic: whether a program passes one
of these limits is the same in every implementation, on every run. The
deadline is not, because wall-clock time is not. A host can also cancel a
run. A `catch` in the program cannot catch a limit.

## Embed it

Declare what the program can see, compile it once, run it as often as you
like. The three APIs have the same concepts and the same names.

### Go

```bash
go get github.com/deepnoodle-ai/risor/go/v3@latest
```

```go
type User struct {
    Name string `json:"name"`
    Age  int    `json:"age"`
}

userType, err := risor.TypeOf[User]() // {age: int, name: string}
env, err := risor.NewEnv(risor.VarType("user", userType))
expr, err := risor.Parse(`user.age >= 18`)
prog, err := env.Compile(expr) // checks the program
out, err := prog.Run(ctx, risor.Map(map[string]any{
    "user": User{Name: "Ada", Age: 36},
}))
// true
```

### TypeScript

TypeScript's types do not exist at run time, so a TypeScript host declares
the type in Risor's type syntax. It looks like a TypeScript type.

```ts
import { createEnv, parse } from "risor";

const env = createEnv({ vars: { user: "{name: string, age: int}" } });
const prog = env.compile(parse("user.age >= 18")); // checks the program
prog.run({ user: { name: "Ada", age: 36 } });
// true
```

### Rust

```toml
[dependencies]
risor = { git = "https://github.com/deepnoodle-ai/risor", features = ["derive"] }
```

```rust
use std::collections::HashMap;
use risor::{parse, CompileOptions, Env};

#[derive(risor::Host)]
struct User { name: String, age: i64 }

let env = Env::builder().var_host::<User>("user").build()?;
let prog = env.compile(&parse("user.age >= 18")?, CompileOptions::default())?;
let vars = HashMap::from([("user".to_string(), User { name: "Ada".into(), age: 36 })]);
let out = prog.run(&vars)?;
// true
```

`#[derive(risor::Host)]` lets a program read your structs in place. It needs
the `derive` feature, which adds one dependency: the project's own
`risor-derive` crate, which has none of its own.

### Setting limits

```go
risor.NewEnv(risor.Limits{Steps: 10_000, Allocation: 10_000, Deadline: time.Second})
```

```ts
createEnv({ limits: { steps: 10_000, allocation: 10_000, deadlineMs: 1000 } });
```

```rust
Env::builder().limits(Limits { steps: Some(10_000), allocation: Some(10_000),
    deadline: Some(Duration::from_secs(1)), ..Limits::default() })
```

| Implementation | Where | Package |
| --- | --- | --- |
| Go | [`go/`](go) | `github.com/deepnoodle-ai/risor/go/v3` |
| TypeScript | [`ts/`](ts) | not yet on npm |
| Rust | [`rust/`](rust) | not yet on crates.io |

## The CLI

```bash
go install github.com/deepnoodle-ai/risor/go/v3/cmd/risor@latest

risor eval program.risor --vars data.json   # run a program
risor check program.risor --vars data.json  # check it without running it
risor fmt --write .                         # format programs
risor test                                  # run case files
risor                                       # the REPL
```

## Status

All three implementations pass the Core specification with the type
checker. Each is now building the follow-on packages in
[`spec/packages/`](spec/packages): JSON Schema, config files, templates,
queries and others. Go is the furthest along.

## Request for comment

We want to hear what works and what does not before 3.0. Open an
[issue](https://github.com/deepnoodle-ai/risor/issues) about the language,
the specification or an implementation.

| To read | Start with |
| --- | --- |
| The language | [`spec/core/01-language.md`](spec/core/01-language.md) |
| How a host embeds it | [`spec/core/03-host.md`](spec/core/03-host.md) |
| The design and its decisions | [`experiments/v3/design/00-overview.md`](experiments/v3/design/00-overview.md) |
| The conformance suite | [`suite/README.md`](suite/README.md) |

## License

[Apache-2.0](LICENSE)
