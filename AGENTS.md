# AGENTS.md

This file provides guidance to coding agents when working with code in this
repository.

## This is Risor v3

`main` holds Risor v3, published as an alpha and a request for comment. Risor
v2 is released and is maintained on the `v2` branch.

v3 is a new language and a new code base. It is not a revision of v2.

**Status: Core is built; the follow-on packages are being built.** The design
is written. The specification is in `spec/`: Core in `spec/core/`, and one
chapter for each follow-on package in `spec/packages/`. The conformance suite
is in `suite/`. All three implementations pass Core with the type checker:
Go in `go/`, TypeScript in `ts/` and Rust in `rust/`. Each then builds the
packages in a fixed order (`spec/core/03-host.md` 3.11); the Go implementation is the furthest along.
`experiments/v3/design/14-implementation-plan.md` is the plan: what Core is,
what follows it, and the order.

## What v3 is

Risor v3 is a small expression language that a host program embeds. A host
gives it data and gets a value back. Typical uses are a condition in a workflow
step, a template, a query over JSON, and a config file.

- **A specification, not one implementation.** The language is defined by a
  specification and a conformance suite of data files. There are three
  independent implementations: Go, Rust and TypeScript.
- **A program is `const` bindings followed by one expression.** There are no
  statements, no loops and no reassignment. `if`, `match` and `try` are
  expressions.
- **Values cannot be changed.** They are null, bool, number, string, list, map
  and opaque host values. Host data is read in place.
- **Checked before it runs.** A static checker is part of the language. A
  checked program cannot fail with an unknown name, a missing field, a null or
  a wrong kind.
- **Bounded.** Steps, allocation units, a deadline and a depth limit are part
  of the language. The counts are exact, so a program reaches a limit at the
  same point in every implementation.
- **No bytecode and no virtual machine.** The Go implementation compiles the
  AST to closures. `experiments/v3/README.md` has the measurement behind this.

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

This is not v2 syntax. v2 has `let`, `function`, statements and loops.

## Goals

The project owner set these constraints. Every design choice is judged against
them.

| # | Constraint |
| --- | --- |
| C1 | The language is not tied to Go. There are three implementations: Go, Rust and TypeScript. |
| C2 | Least surprise, all else equal. |
| C3 | A preference for expressions over many statements. |
| C4 | Expressiveness and the author's experience must be high. A design that is safe but austere misses the goal. |
| C5 | Memory allocation and run duration can be controlled and limited. |
| C6 | Lean toward TypeScript's syntax, all else equal, without JavaScript's historical baggage. |
| C7 | Compatibility with expr and with Risor v1 and v2 carries little weight. A more robust v3 is worth a break. |

C1 has a consequence for all work: a rule that only Go can implement cheaply is
a defect in the design. Do not define behavior by what a Go standard library
function does.

## Where the design is

The design is in `experiments/v3/design/`. Read `00-overview.md` first.

| File | Subject |
| --- | --- |
| `00-overview.md` | Constraints, every decision (`D1` and up), the roadmap. **It is the authority: where another file disagrees, this file wins.** |
| `01-prior-art.md` | Other embedded languages and what to take from them |
| `02-language.md` | Grammar, node kinds, values, semantics, parse errors |
| `03-types.md` | Types, the checker, check errors |
| `04-runtime.md` | Evaluation, step and allocation costs, limits |
| `05-host-api.md` | The API a host uses, in each of the three languages; error codes; the CLI |
| `06-stdlib.md` | The library of functions and methods |
| `07` to `13` | One feature each: checked conversion (`as`, `is`), strings, tagged unions, errors, testing, config files, queries |
| `14-implementation-plan.md` | Core and the follow-on packages, the milestones, the rules of the work |
| `15-build-rulings.md` | Every ruling made while building: decisions taken, design rules changed, new rules |

A plan (`07` to `13`) is the authority for its feature. `02` to `06` carry its
rules and cite it.

## The specification, the suite and the implementations

| Path | Holds |
| --- | --- |
| `spec/core/` | The Core specification, five chapters: language, evaluation, host, library, types. Normative text only. |
| `spec/packages/` | One chapter for each follow-on package. A package only adds: its chapter is the home of its rules, and the Core chapters point to it. |
| `suite/` | Data every implementation shares: the library's records, the message templates, the case mapping, and case files in `suite/core-run/`, `suite/core-check/` and one directory for each package. `suite/README.md` lists them. `spec/core/03-host.md` 3.10 defines the case format. |
| `go/` | The Go implementation: the module `github.com/deepnoodle-ai/risor/go/v3`, with `cmd/`, `internal/` and `risortest/`. Its release tags carry the directory: `go/v3.x.y`. |
| `ts/` | The TypeScript implementation. No run-time dependencies. |
| `rust/` | The Rust implementation. No library dependencies. |
| `fuzz/` | The fuzzers that compare the three implementations with one another and test the checker's promise. Its own Go module. |

- **The specification wins.** Where `spec/` and a design document `00` to `13`
  disagree, the specification is right and `15` records why.
- **An implementer of one language does not read another implementation's
  source.** The specification and the suite are the contract. This is how the
  project tests that the language is not tied to one platform (C1).
- **Behaviour that matters gets a suite case,** not only a unit test.
- **A case you believe is wrong is a question, not a case to skip or edit.**
- **Lexers and parsers are written by hand,** as Pratt parsers. No parser
  generator and no parsing library.

`experiments/v3/lab/` holds the small Go measurements that the documents cite.
`experiments/v3/spike/` and `bench/` are the first experiment: an evaluator
with no VM, measured against v2 and expr.

## Decisions

The project owner decides. Section 4 of the overview lists each decision and
its state: settled by the owner, or a recommendation that the owner has not
yet confirmed.

- Do not settle an open decision as a side effect of other work. Write to the
  recommendation, say that it is open, and put the question to the owner.
- Do not reverse a settled decision. If you find a reason to, state the reason
  and ask.
- When two documents disagree, do not pick one silently. Find which document
  owns the rule, fix the other, and report it.
- A new decision gets the next `D` number and a row in the overview.

## Editing the design documents

- **Keep the counts in step.** The documents state the number of node kinds,
  error codes, library entries and reserved words in several places. A change
  to one of them is a change to every file that states it. Search for the old
  number before you finish.
- **Say how you know.** The documents mark a claim as measured, as run
  (`ran`, with the tool and version), as inference, or as "from memory". Keep
  that practice. Do not state an untested claim about TypeScript, Rust or
  another language as fact.
- **Cite exactly.** A rule is cited by document and section (`02` 2.8), a
  decision by its number (`D47`, or `09-4` inside a plan).
- **Style.** Plain, exact English. Short sentences. Tables for rules. Prose
  wrapped at 80 columns.
- **This repository is public.** Cite only public repositories.

## Order of work

Section 5.2 of the overview has the full list. In short:

1. The owner settles the decisions that are still open.
2. The conformance format and first cases; a Go lexer, parser and AST.
3. A Go reference evaluator with all limits and a core subset of the library.
4. A TypeScript lexer, parser and evaluator, written from the specification
   and the suite only.
5. A Rust lexer, parser and evaluator, the same way.
6. The checker in Go.
7. The full host binding and library in Go.
8. Acceptance: a real host ported to v3.
9. The checker and library in TypeScript and Rust; tools.

Steps 4 and 5 are early on purpose. They test C1 before the checker and the
library are built on rules that might not hold in three languages.

## Branches

| Branch | Holds |
| --- | --- |
| `main` | Risor v3, the alpha. |
| `v2` | Risor v2. v2 releases are cut from it. |
| `v1` | Risor v1, kept for reference. |

- **v3 work lands on `main`.** Branch from `main` and open the pull request
  against `main`.
- **A fix for v2 goes to `v2`,** on a branch from `v2`, with `--base v2`. v2's
  rules (compatibility, resource limits, `make test`) are in the `AGENTS.md`
  on that branch.
- **Pull requests are squash-merged.** The title becomes the commit message.
- **Do not rebase or force-push `main` or `v2`.**

`.github/workflows/ci.yml` runs the three implementations and the fuzzer's own
tests on every push and pull request that touches Go code, `ts/`, `rust/`,
`fuzz/` or `suite/`.

## Go code

The Go implementation is the module in `go/`. `fuzz/` and `experiments/v3/`
are modules of their own. There is no `go.work`. Run these in `go/`:

```bash
go build ./...
go vet ./...
go test ./...            # includes the conformance suite in suite/
go generate ./internal/... # after a change to suite/library*.json, messages*.json or casemap.json
```

All Go tests use the `assert` package of
[Wonton](https://github.com/deepnoodle-ai/wonton). Decision D23 in the
overview proposes that the v3 module have no dependencies, which would end
this rule. It is not settled. Until it is, use `assert`.

## Changelog

Pull requests do not edit `CHANGELOG.md`. Every PR edits the same `Unreleased`
section, so each merge makes the other open PRs conflict.

Instead, describe the user-visible change in the PR title and description.

## Releasing

v3 alpha releases are tagged `go/v3.0.0-alpha.N` on `main`: the Go module in
`go/` takes its versions from tags with that prefix (R200). **Never push a bare
`v3.*` tag.** With no `go.mod` at the root, Go lists such a tag as a broken
version of the old path `github.com/deepnoodle-ai/risor/v3`, and `go list -m
-versions` and `@latest` fail there. The GitHub release is made from the
`go/` tag. v2 releases are cut
from the `v2` branch; see `docs/guides/releasing.md` there.
