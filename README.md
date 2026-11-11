# Risor

> **This is Risor v3, an alpha and a request for comment.** The language, the
> specification and the APIs will change before 3.0.
>
> **Looking for Risor v2?** It is released and maintained on the
> [`v2` branch](https://github.com/deepnoodle-ai/risor/tree/v2). Existing
> imports of `github.com/deepnoodle-ai/risor/v2` keep working.

Risor is a small expression language that a host program embeds. The host
gives it data and gets a value back. Typical uses are a condition in a
workflow step, a template, a query over JSON, and a config file.

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

- **A program is `const` bindings followed by one expression.** No
  statements, no loops, no reassignment. `if`, `match` and `try` are
  expressions.
- **Values cannot be changed.** Host data is read in place.
- **Checked before it runs.** A checked program cannot fail with an unknown
  name, a missing field, a null or a wrong kind.
- **Bounded.** Steps, allocation, a deadline and a depth limit are part of
  the language, with exact counts, so a program reaches a limit at the same
  point in every implementation.
- **A specification, not one implementation.** The language is defined by
  [`spec/`](spec) and a conformance suite of data files in [`suite/`](suite).
  There are three independent implementations: Go, TypeScript and Rust.

## Status

All three implementations pass the Core specification with the type checker.
Each is building the follow-on packages (`spec/packages/`); Go is the
furthest along.

| Implementation | Where | Package |
| --- | --- | --- |
| Go | [`go/`](go) | `github.com/deepnoodle-ai/risor/go/v3` |
| TypeScript | [`ts/`](ts) | not yet on npm |
| Rust | [`rust/`](rust) | not yet on crates.io |

## Go

```bash
go get github.com/deepnoodle-ai/risor/go/v3@latest
```

The first alpha, `v3.0.0-alpha.1`, was published at
`github.com/deepnoodle-ai/risor/v3`. Later releases use the path above.

```go
env, err := risor.NewEnv(
    risor.Var("tests", "{name: string, status: string, ms: int}[]"),
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

A compiled program does not change and may be run by many goroutines at once.
The full example is [`go/example_test.go`](go/example_test.go).

## CLI

```bash
go install github.com/deepnoodle-ai/risor/go/v3/cmd/risor@latest

risor eval program.risor --var 'n=int' --vars data.json
risor check program.risor
risor fmt --write .
risor test
risor            # the REPL
```

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
