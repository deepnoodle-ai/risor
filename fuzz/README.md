# The differential fuzzer

This directory holds the differential fuzzer of Risor v3. It makes programs,
runs each one in every implementation, and reports each difference.

The language promises exact agreement. Two implementations give the same
value, or the same failure with the same code, span, class and message. They
charge the same steps and allocation units, so a run reaches a limit at the
same point everywhere. The fuzzer tests that promise on programs that no one
wrote by hand.

It is its own Go module, with no dependency outside the standard library and
none on any implementation. It is not in `go.work`. Run Go here with
`GOWORK=off`.

## How it works

1. **Generate.** For each round, the generator makes a case file of
   `03-host.md` 3.10, with no `expect`. Each case has a program and an
   environment: `env`, `vars`, and sometimes an opaque type with stubbed
   methods, stubbed host functions, and small limits. The seed, the round
   and the case's index fix every choice, so a run can be repeated.
2. **Run.** Each implementation runs the file through its `outcome` command
   (ruling AJ of round 10). Each command writes one JSON line for each case.
3. **Compare.** The lines for one case are compared as AJ item 6 says.
   Numbers compare as doubles, so `2` and `2.0` agree. For a run failure of
   class `limit`, only the code, the class and the message are compared:
   the span and the usage are not fixed (ruling AK3). A crash, a timeout, a
   status other than 0 and a missing line are differences too.
4. **Shrink.** Each difference is made smaller while it stays the same
   difference, and written to `findings/`.

The generator has three modes, mixed by weight:

| Mode | What it makes | For |
| --- | --- | --- |
| typed | Programs that are well formed and mostly well kinded. The generator tracks a rough kind for each subexpression. | Values, deep evaluation and exact counts |
| loose | Any tree the grammar allows, kinds mostly ignored, with breaks of the static rules and bad declarations | Run failures of every code, `check` and `declare` |
| mangled | A program with a token deleted, doubled, swapped or replaced, a character changed, the text cut short, a byte order mark, deep nesting, or a comment | The lexer and the first `parse` diagnostic |

The generator reaches all of Core: every operator and precedence level,
`const` lines with patterns and defaults, lambdas and calls, `if`, `match`,
blocks, `try`, `catch` and `fail`, nested templates, list and map literals
with spreads and computed keys, chains with `?.` and indexes, `in` and
`not in`, and calls of every entry of `suite/library.json`. It reads the
entries, their parameters, callbacks and types from that file.

A share of cases gets a small step, unit or depth limit. At most one of the
step limit and the unit limit is small in one case, so that a finding points
at one count.

## Running it

From this directory:

```
GOWORK=off go run ./cmd/fuzz -seed 1 -cases 500 -rounds 40
```

This makes 40 case files of 500 cases each. It builds the Go and Rust
`outcome` programs once, then runs the built programs. TypeScript runs with
`node scripts/outcome.ts`. Each command runs in its own directory.

At the end it prints, for each mode and implementation, the share of cases
that gave a value, a run failure, a check failure, a parse failure and a
declare failure. Then it lists each distinct difference with its count, and
the totals. The status is 1 when there was any finding, else 0.

| Flag | Meaning |
| --- | --- |
| `-seed N` | The seed. The same seed and flags give the same cases, byte for byte |
| `-cases K`, `-rounds R` | Cases in a round, and rounds |
| `-impls go,ts,rust` | The implementations to compare |
| `-typed`, `-loose`, `-mangled` | The weight of each mode: 5, 3 and 2 by default |
| `-limits P` | The percent of cases with a small limit, 25 by default; -1 for none |
| `-exclude a,b` | Library entries not to call |
| `-cmd name=command` | The `outcome` command of an implementation. The file is added at the end |
| `-build name=command` | A command run once before the first file |
| `-nobuild` | Build nothing. Go runs with `go run`; Rust runs the program built before |
| `-timeout D` | The time one command may take on one file, 60s by default |
| `-noshrink`, `-max N` | Do not shrink; write at most N findings for each distinct difference |
| `-noprobe` | Do not probe which library entries each implementation has |
| `-emit DIR` | Write the case files to DIR and run nothing |
| `-replay FILE` | Run one finding, or one case file, and print each line |
| `-case R:I` | Make and run only case I of round R |
| `-v` | Print each round, and the count of outcomes by code |

Before the first round the fuzzer runs the cases of `library.json` in each
implementation. An entry that one implementation reports it does not have
(`undefined-function`, or a method the receiver's kind lacks) is left out of
generation, and the fuzzer says so. A difference of that kind found later is
counted apart, not as a finding.

If a build fails and a program built before exists, the fuzzer says so and
runs the old program.

## Reading a finding

A finding is `findings/<seed>-<n>.json`. The directory is ignored by git.

| Field | Holds |
| --- | --- |
| `seed`, `round`, `index`, `mode` | Where the case came from |
| `difference` | For each pair of implementations, the first field where their lines differ, or nothing when they agree |
| `case` | The shrunk case, as a case of a case file |
| `lines` | Each implementation's line for the shrunk case, or `{"failure": ...}` |
| `original` | The case as it was generated |

The fields of a difference are `phase`, `value`, `diagnostics.count`,
`diagnostics.code`, `diagnostics.span`, `diagnostics.class`,
`diagnostics.message`, `steps` and `units`. A failure is `fail:status`,
`fail:timeout`, `fail:missing line` or `fail:bad-line`.

The one-line summary names the file, the mode, the start of the program, and
what each implementation gave: `value`, or the phase and the code.

## Replaying

To run a finding again in every implementation:

```
GOWORK=off go run ./cmd/fuzz -replay findings/7-3.json
```

To make one generated case again, give the seed and the mode weights of the
first run, and the round and index of the finding:

```
GOWORK=off go run ./cmd/fuzz -seed 7 -case 19:354
```

The library entries left out change what is generated. A replay by seed
needs the same `-exclude` list, and the same probe result: the fuzzer prints
the entries the probe left out.

## Tests

```
GOWORK=off go test ./...
```

The tests cover the printer (hand-made trees, and 20,000 random trees read
back by a small parser written from the grammar), the comparer on canned
lines, the reducer on a fake difference, and that one seed gives one file.
