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
> [`v2` branch](https://github.com/deepnoodle-ai/risor/tree/v2).

Your application has data. Your users, your operators or an LLM want to
write the logic over it: a rule in a workflow, a filter, a template, a
transform of an API response, a config file. Risor is the language for that
logic. Your program gives it data and gets a value back.

```js
// Summarize a test run. `fetch` is data from the host.
const failed = fetch.tests.filter(t => t.status == "fail");
const rate = len(failed) / len(fetch.tests);
const verdict = if (!failed) { "pass" }
  else if (rate < 0.1) { "flaky" }
  else { "fail" };
{
  verdict,
  summary: `${len(failed)} of ${len(fetch.tests)} failed`,
  names: failed[*].name,
  slowest: failed.sortBy(t => t.ms, "desc").slice(0, 3)
    .map(({name, ms}) => `${name} ${ms}ms`),
  owner: try { fetch.owner as string } catch { "unknown" },
}
```

If you know TypeScript, you can read this already.

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
field, an unexpected null or a value of the wrong kind. You find the error
when the user saves the rule, not at 3 a.m.

**Small enough to learn in an afternoon.** A program is `const` bindings
followed by one expression. There are no statements, no loops and no
reassignment. `if`, `match` and `try` are expressions. Values cannot be
changed, and host data is read in place, with no copy.

**Defined by a specification.** The language is defined by
[`spec/`](spec) and by a conformance suite of data files in
[`suite/`](suite). All three implementations pass the Core suite.

## Catch errors before they run

The checker knows the shape of your data. Here, a test that failed has no
owner:

```bash
$ risor check rule.risor --vars data.json
error[maybe-null] 2:17-2:24: t.owner may be null; its type is {team: "web"} | null
    failed.map(t => t.owner.team)
                    ^^^^^^^
```

Change the program to `t.owner?.team ?? "unowned"` and the checker accepts
it. It infers the type of the result: `("unowned" | "web")[]`.

## Run code you did not write

A run that passes a limit stops with an error that names the limit. A
`catch` in the program cannot catch it.

```bash
$ risor eval runaway.risor
error[steps-exceeded] 1:1-2:32: the run needed more than its limit of 1000000 steps
    const xs = range(0, 100000);
    ^^^^^^^^^^^^^^^^^^^^^^^^^^^^
  in map, element 2, at 2:1-2:32
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
run.
The rules are in [`spec/core/03-host.md`](spec/core/03-host.md) 3.6.

## Embed it

The three APIs have the same concepts and the same names. They differ only
where each language has its own idiom: options, errors and cancellation.

### Go

```bash
go get github.com/deepnoodle-ai/risor/go/v3@latest
```

```go
env, err := risor.NewEnv(
    risor.Var("tests", "{name: string, status: string, ms: int}[]"),
    risor.Limits{Steps: 100_000, Allocation: 50_000, Deadline: 50 * time.Millisecond},
)
expr, err := risor.Parse(`
const failed = tests.filter(t => t.status == "fail");
{
  verdict: if (!failed) { "pass" } else { "fail" },
  names: failed[*].name,
}`)
prog, err := env.Compile(expr) // checks the program against the declarations
out, err := prog.Run(ctx, risor.Map(map[string]any{"tests": tests}))
// map[names:[upload] verdict:fail]
```

A compiled program does not change, and many goroutines can run it at the
same time. The full example is
[`go/example_test.go`](go/example_test.go).

### TypeScript

```ts
import { createEnv, parse, RisorError } from "risor";

const env = createEnv({
  vars: { tests: "{name: string, status: string, ms: int}[]" },
  limits: { steps: 100_000, allocation: 50_000, deadlineMs: 50 },
});
const prog = env.compile(parse(`
const failed = tests.filter(t => t.status == "fail");
{
  verdict: if (!failed) { "pass" } else { "fail" },
  names: failed[*].name,
}`));
const { value, usage } = prog.measure({ tests });
// value: Map { verdict => "fail", names => ["upload"] }
// usage: { steps, allocated }
```

A failure throws a `RisorError` that holds its diagnostics. A run takes an
`AbortSignal`.

### Rust

```rust
use risor::{parse, CompileOptions, Env, Limits};

#[derive(risor::Host)]
struct Test { name: String, status: String, ms: i64 }

let env = Env::builder()
    .var_host::<Vec<Test>>("tests") // the type comes from the struct
    .limits(Limits { steps: Some(100_000), allocation: Some(50_000), ..Limits::default() })
    .build()?;
let expr = parse(r#"
const failed = tests.filter(t => t.status == "fail");
{
  verdict: if (!failed) { "pass" } else { "fail" },
  names: failed[*].name,
}"#)?;
let prog = env.compile(&expr, CompileOptions::default())?;
let out = prog.run(&HashMap::from([("tests".to_string(), tests)]))?;
// {"names":["upload"],"verdict":"fail"}
```

The crate has no dependencies and no `unsafe`. Host structs are read in
place through `#[derive(risor::Host)]`.

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
