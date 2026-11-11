# Risor v3: implementation sequence, Core and follow-on

Status: in force. The owner asked for this plan to be built on 2026-11-09.
The decisions of section 7 were taken as recommended, and `15-build-rulings.md`
records them and every ruling made since. The plan refines the order of work
in `00-overview.md` 5.2 and differs from it in three places, listed in section
8. Three things joined Core run after the plan was written (`15` R9, R10 and
R19): the static rules, the parser of type text, and opaque types.

It answers two questions. What is built first, and in which order. And where
the line is between **Core** functionality and **follow-on** functionality.

All sizes are (I). A package's figure is the estimate of the plan it comes
from. The split of the front end, the evaluator and the binding between the
tiers is my own apportionment of the totals in `00` section 3. Nothing here
was built.

## 1. Summary

1. **Core is the part of 3.0 that cannot be added later without a break.**
   Follow-on is everything that only adds. Section 2 gives one test for the
   line, and CI can enforce it.
2. **Core has two halves.** *Core run*: parse, evaluate, limits, errors and the
   library's default group as it stood before D51 (53 entries). *Core check*:
   types and the checker, for the forms of Core run.
3. **Core run is built three times before any follow-on work starts.** That is
   the test of C1, and it is the riskiest claim in the design. The Go checker
   is built at the same time, because it does not depend on the result.
4. **The first host is ported right after Core, in Go.** The workflow engine
   needs Core, types from JSON Schema, templates and the analysis calls. It
   does not need the six areas of D46 to D51. Porting it early tests the host
   API before twelve packages are built on it.
5. **Follow-on work is twelve packages in three waves.** Each package is
   additive, has its own directory of suite cases, and lands in all three
   implementations.
6. **Every follow-on package could move to 3.1 without a break,** because Core
   reserves its syntax and its words. Nothing is proposed to move. D46 to D51
   put all six areas in 3.0, and this plan keeps them there.

Go size, by tier: Core run about 10,000 lines, Core check about 2,400,
follow-on about 7,000. So Core is nearly two thirds of the Go implementation.
The split is clean; it is not a small Core.

## 2. The line

### 2.1 The test

A piece of the design is **Core** if any of these holds:

| # | Test | Examples |
| --- | --- | --- |
| T1 | **Adding it later would change the expectation of a suite case that already exists:** a value, a code, a span, a step count or a unit count | The cost table. Truthiness. The number rules. Key order. The run codes. Where a mismatch is reported |
| T2 | **Other pieces cannot be built or tested without it** | The parser. The value model. The case-file format and a runner. The type representation |
| T3 | **It is a rule that might not hold the same way in three implementations** | Number formatting. Text size. Sorting. Case mapping. The depth limits |

A piece is **follow-on** if none holds. Then it only adds: text that was an
error becomes a program, or a new library entry, host function or tool appears.
That is the same rule a minor version obeys (`05` A.10).

T1 is mechanical. **A pull request for a follow-on package adds case files and
edits none.** CI can refuse one that does.

### 2.2 What Core must do for follow-on work to be additive

| Rule | Detail |
| --- | --- |
| **Reserve in Core, implement in follow-on** | Every word of 3.0 and of later versions is reserved from the first Core parser: `as`, `is`, `import`, `throw`, `finally`, `let`, `var`, the loop words. Text that a follow-on package gives meaning to is a parse error in Core: `"""`, `r"..."`, `xs[*]`, `x as T` |
| **All run codes exist in Core** | The 19 run codes and their classes are frozen with Core, since a program can read `err.code`. A follow-on package may raise an existing code at a new place. It does not add or rename one |
| **Core cases do not pin what a package will sharpen** | A Core case for a parse error states its code only when the code is in Core: the 13 general ones and `reserved-word`. Where a targeted error of `02` 3.2 will cover the text, the case states the phase and the span |
| **The type representation is the final one** | Core check holds a type as `09` describes it, with room for several record alternatives, although the rules for tagged unions come later. Otherwise the checker is written twice |
| **A cost row is never changed** | A package adds rows to the cost table for its own forms |

### 2.3 Four places where the test decides against the obvious choice

| Piece | Obvious tier | Tier by the test | Why |
| --- | --- | --- | --- |
| `try` / `catch`, `fail`, the error value | Follow-on: it is plan `10` | **Core** | T1 and T3. How an error unwinds is in every evaluator. `err.message` is visible to a program, so the run messages are results. The workflow engine uses `try` today (`06` 3.3) |
| The run-phase part of `messages.json` | Follow-on | **Core** | It follows from the row above: 61 templates as built (`15` R21; the first estimate was about 33), for the 19 run codes and the 14 forms of `invalid-argument`. Rendering a value in a message is also a T3 rule. The parse and check templates stay follow-on |
| `filter` and `find` narrow (`13` 5.1) | Follow-on: it is plan `13` | **Core check** | T1. A more exact result type can change a diagnostic of a program that already checks. 30 lines |
| A mismatch is reported at the element (`09` 2.6) | Follow-on: it is plan `09` | **Core check** | T1. It moves the span of `result-mismatch` and `arg-mismatch`, and cases state spans. 60 lines |

The reverse case: **`as` and `is` are follow-on,** although `07` moved the type
text parser into the core package. They pass the test: `x as T` is a parse
error until the package lands. They are the first follow-on package, because
four others depend on them.

A note on words. `06` calls the library's default group "the core". In this
plan that is **the default group**, and **Core** is always the tier.

## 3. Core

### 3.1 Core run

| Area | In Core | Source |
| --- | --- | --- |
| Program | `const` lines and one expression; blocks; scope; comments | `02` 2.1 |
| Literals | null, bool, numbers, `"..."` and `'...'` with escapes, backtick templates with `${ }`, lists, maps, spread, shorthand and computed keys | `02` 2.2, 2.11 |
| Operators | Arithmetic with `**`, comparison, equality, `&&`, `\|\|`, `!`, `??`, `in`, `not in`, `? :` | `02` 2.5, 2.7 |
| Reads and calls | `.f`, `[i]`, `?.`, `?.[`, calls, method calls, chains as a unit | `02` 2.8 |
| Lambdas | Arrow functions, list and map patterns, defaults, rest; a `const`-bound lambda; the rules of D34 | `02` 2.9 |
| Choice | `if` / `else if` / `else`. `match` by literal cases. `match` by conditions if D38 keeps it | `02` 2.10 |
| Errors | `try { } catch (err) { }`, `fail(message, data?)`, the error value, which codes `catch` passes through. Every run code, with span and path. The 13 general parse codes, and `reserved-word` | `02` 2.12, `10` 2.1 to 2.6 |
| Semantics | Truthiness, the number rules, text as code points, equality, order, key order, evaluation order | `02` 2.4 to 2.7, `04` A.1 |
| Limits | Steps, allocation units, deadline, depth, source nesting of 100. The cost of every form above | `04` Part A |
| Library | The default group without the seven entries of D51: 15 functions, 13 string methods, 22 list methods, 3 map methods. `library.json` and its loader | `06` 2.8, 2.11 |
| Messages | The format of `messages.json`, its loader, and the run-phase templates | `10` 2.9 |
| Host | Parse, compile, run. An environment of variables, host functions and limits. Usage. One error type. Host values read in place | `05` A.1 to A.6 |
| Tests | The case-file format, and a runner in each implementation: program, variables, stubs for host functions, `expect` | `05` A.11 |

Not in Core run: every row of section 5.

### 3.2 Core check

| Area | In Core | Lines (`03` 11) |
| --- | --- | ---: |
| Types | Sets of alternatives, Join, Fits, Merge, printing, in the representation of `09` | 400 |
| Type text | The parser for type text and signatures | 270 |
| Checking | Every node of Core run: operators, chains, narrowing of null | 750 |
| Truthiness | Tested positions, `&&` and `\|\|` as Joins, constant conditions, `condition` | 125 |
| Choice and patterns | `match`, `if`, patterns, blocks | 265 |
| Calls | Signatures from `library.json`, solving, `const`-bound lambdas | 270 |
| Diagnostics | Codes with exact spans, classes of failure site | 160 |
| Limits | The checker's own depth limits (`03` 2.17) | 30 |
| `try` | Scopes, the Join, the type of the error value | 40 |
| The two rows of 2.3 | `filter` and `find` narrow; a mismatch at the element | 90 |
| **Total** | | **2,400** |

An environment declares a variable's type in type text. Types from JSON Schema
and from host types are package F1.

## 4. The sequence

### 4.1 Milestones

| # | Milestone | Work | Exit |
| --- | --- | --- | --- |
| M0 | **Decisions and ground** | The owner settles section 7. CI runs on `v3`. The repository layout is chosen | Nothing in section 7 blocks M1 |
| M1 | **Core on paper** | The Core chapters of the specification, taken from the normative parts of `02`, `04`, `05` Part A, `06` and `10`. The case-file format, fixed. `library.json` for the 53 entries. The run-phase templates. Core cases for values and codes | An implementer can start from these files alone |
| M2 | **Core run, Go** | Lexer, parser, AST, the reference evaluator, limits, errors, the 53 entries, the minimal host API, the runner | The Core run cases pass. The spike's benchmarks run through the real API and meet the targets of `04` |
| M3 | **Core run, TypeScript** | The same, written from the specification and the suite | The Core run cases pass with the same steps and units |
| M4 | **Core run, Rust** | The same, with host records through a trait and values lent for a run. The differential fuzzer starts here | The same. The limit cases pass in the least favorable build |
| M5 | **Core check, Go** | Section 3.2, with its cases and the soundness fuzzer | The Core check cases pass |
| — | **The Core gate** | Every disagreement between the three is settled in the specification. The rates are calibrated on real evaluators. D31's numbers are settled | **Core is stable.** From here a change to a Core rule needs the owner's decision |
| M6 | **First host** | Packages F1 and F2 in Go. The workflow engine ported on a branch | Its own tests and its soundness fuzz tests pass. `3.0.0-alpha.1` of the Go module |
| M7 | **Wave 1: types on data** | F3 strings, as the pilot. F4 checked conversion. F5 tagged unions. F6 opt-in groups | Each package's cases pass in all three, on the run side |
| M8 | **The checker, TypeScript and Rust** | Core check and the check side of wave 1, ported against finished cases | All check cases pass in all three |
| M9 | **Wave 2: authoring** | F7 queries. F8 errors, second part | Cases pass in all three, run and check |
| M10 | **Wave 3: hosts and tools** | F9 testing for authors. F10 config files. F11 the typed layer. F12 tools. F1 and F2 in TypeScript and Rust | Cases pass in all three. `3.0.0-beta.1` |
| M11 | **3.0** | The whole suite in all three. A public conformance report for each. The cost table and the run messages frozen | `3.0.0` (D27) |

### 4.2 What runs at the same time

| Work | May run beside | Must wait for |
| --- | --- | --- |
| M3 and M4 | Each other, and M5 | M2's suite cases, not M2's code |
| M5 | M3 and M4 | M2 |
| M6 | The pilot package F3 | The Core gate |
| A package in TypeScript or Rust | The next package in Go | The package's cases being final |
| M8 | Wave 2 in Go | Wave 1 in Go: tagged unions complete the type representation's rules |

Go leads. The rule that bounds the lead: **Go does not start a wave until the
other two pass the run side of the wave before it.**

### 4.3 Rules for the work

| Rule | Why |
| --- | --- |
| **The TypeScript and Rust implementers do not read the Go source.** They get the specification, the suite and the data files | A rule that only Go implements cheaply must show up as a question about the specification, not be copied |
| **A question from M3 or M4 is answered by a change to the specification,** and a case | Otherwise the answer lives in one implementation |
| **Cases come before code for values, codes and spans.** Step and unit counts are filled in from the Go evaluator, and each is reviewed against the cost table | A count by hand for every case is slow and wrong often. The other two implementations, working from the table, are the check on Go's counts |
| **A package is done when** its specification text, its cases, its records in `library.json` and `messages.json`, and three implementations are merged | Section 5 has the exception for the check side before M8 |
| **Each implementation publishes a conformance report by suite directory** | The lag is visible per package |
| **Each parser is written by hand,** a Pratt parser as in v2, with no parser generator and no run-time library. Settled by the owner | The parse codes and their spans are specified, the nesting limit is counted on the text, and no one generator serves Go, Rust and TypeScript. A generated grammar is used only where no implementation depends on it: the editor grammar (F12) and the programs the differential fuzzer makes (M4) |

### 4.4 The suite, by tier

One directory for each tier and package: `core-run/`, `core-check/`, then one
for each package of section 5. The report of an implementation lists them. The
check of 2.1 is on these paths.

## 5. Follow-on packages

Lines are Go, without tests, from the plan named. "Check side" is the part that
needs a checker. Before M8 it lands in Go only.

| # | Package | What it adds | Depends on | Lines | Check side |
| --- | --- | --- | --- | ---: | --- |
| F1 | **Host types** | Types from JSON Schema (`03`), from Go types by `TypeOf`, from Rust by the derive. `Describe` | Core check | 110, and 440 in Go, 180 in Rust | all of it |
| F2 | **Templates and analysis** | `${{ }}` templates (`05` A.7). Names, paths and calls of a program without an environment (`05` A.8) | Core run | in `05`'s total | none |
| F3 | **String forms** (`08`) | The text block `"""`. The raw prefix `r`. Three parse codes | Core run | 170, and 60 in `fmt` | none |
| F4 | **Checked conversion** (`07`) | `as`, `is`. Named types in an environment. `Conform` | Core; F1 for named types from a schema | 660 | 150 |
| F5 | **Tagged unions** (`09`) | Several record alternatives told apart by a tag. Narrowing by the tag. `untagged-union` | F4 | 275 | 225 |
| F6 | **Opt-in groups** (`06`) | `json`, `time`, `math`, `base64`: 19 entries, 2 constants | Core run | 540 | signatures only |
| F7 | **Queries** (`13`) | The step `[*]` and the node `Each`. `minBy`, `maxBy`, `uniqueBy`, `indexBy`, `countBy`, `mapValues`, and a direction for `sortBy` | Core; F5 for narrowing by a tag in `filter` | 370 | 50 |
| F8 | **Errors, second part** (`10`) | The trace. The targeted parse errors that no other package brings. The parse and check templates. The three renderings. `ConformAll` | Core; F4 for `ConformAll` | 310, and the targeted errors | none |
| F9 | **Testing for authors** (`11`) | Fixtures tested against their types. Update mode. `risortest`, `defineCases`, `risor::testing`. `risor test` | F4 | 400 beyond Core's runner | none |
| F10 | **Config files** (`12`) | The JSON text of a result. `Decode`, `Load`. A closed type from a schema. `eval --format json`, `check --type` | F1, F4, F5 | 280 | the closed type |
| F11 | **The typed layer** (`04` Part B, D22) | Speed on Go structs. No change in any result | Core check, F1 | 600 | none |
| F12 | **Tools** | `eval`, `check`, `fmt`, the REPL, the editor grammar, completion and hover | F8 for the renderings | 500 for completion and hover, and the commands | none |

### 5.1 Why this order

| Choice | Reason |
| --- | --- |
| F1 and F2 first, in Go only | They are what the first host needs. The port tests the host API, which the prior art says diverges more than evaluators do (`00` 5.4) |
| F3 as the pilot | It is the smallest package, it touches only the lexer, and it exists in all three. It proves the process of 4.3 before a large package depends on it |
| F4 before F5, F9 and F10 | Tagged unions are tested by the walk of `as`. A fixture and a config file are both conformed to a type |
| The checker is ported after F5, not before | F5 completes the rules over the type representation. A port before it is a port twice |
| F7 and F8 after the port | Their check sides are small, and by then they land in three checkers at once |
| F11 late | It changes speed and no result. It needs real programs from the first host to measure against |
| F12 last | Tools show what exists. `fmt` must know every string form |

### 5.2 If three of everything proves too much

The line makes a cut possible without a break. If one is needed, I would cut
in this order, last built first: F12's editor support, F11, F10, F9, the
`time`, `math` and `base64` groups of F6. R9 is the other fallback: one
checker shared as WebAssembly (`00` 2.8). Neither is proposed.

## 6. After 3.0

Already decided or ruled in the other documents. Listed so that the line has
three sides.

| Feature | Where it is ruled | Reserved in Core |
| --- | --- | --- |
| Imports | `12` 3.9, D50: a minor version | `import` |
| `type` lines and annotations | D40: a minor version | `type` as a contextual word |
| Comprehensions | D51: a minor version, on evidence | `[for` and `{for` are errors |
| Regular expressions | D9 | the `regexp` namespace |
| Loops, reassignment, early `return` | D36: deferred | `let`, `var`, the loop words, `return` |
| Raising a caught error again unchanged; a pattern in `catch ( )` | `10` | `throw` |
| Structural patterns in `match`, bindings and guards | D10 | — |
| Recursive types | not planned yet | — |
| An asynchronous run in TypeScript | D18 | — |

## 7. What the owner settles at M0

| # | Question | Recommendation | Blocks |
| --- | --- | --- | --- |
| 1 | D37: must an `if` have an `else`? | Yes in 3.0 | The grammar, M1 |
| 2 | D38: does `match` by conditions stay? | Drop it | The grammar, M1 |
| 3 | The names of the run codes and the general parse codes (`02`, question 12) | As written | M1. Frozen at the Core gate |
| 4 | The line of section 2, with `try` and the run messages in Core, and `as` and `is` as the first package | As written | M1 |
| 5 | D27 and the release names: an alpha of the Go module at M6, a beta at M10, 3.0 when all three pass | Yes | M6 |
| 6 | The repository layout: the Go module at the root; `ts/`, `rust/`, `spec/`, `suite/` beside it; the design moved out of `experiments/` | Yes. Replaced by R200 (`15`): the Go module is in `go/` | M1 |
| 7 | When v2's code leaves this branch | When M2 passes. Merges from `main` stop then | M2 |
| 8 | D23: the Go core module has no dependencies, so its tests do not use `assert` | Yes | M2 |
| 9 | The unconfirmed recommendations that shape Core: D8, D10 to D13, D15 to D17, D21, D25, D31 | As recommended. D15 and D31 are settled finally at the Core gate, on measurements | M1; the gate |

The other unconfirmed recommendations belong to packages and can wait for
them: D14 (F2), D18 to D20 and D24 (F1, F12), D22 (F11), D26 (F6).

## 8. Differences from `00` 5.2

| `00` 5.2 | This plan | Why |
| --- | --- | --- |
| The Go checker (step 6) follows the TypeScript and Rust evaluators (steps 4, 5) | It is built beside them. The Core gate still waits for all three evaluators | The checker does not depend on what steps 4 and 5 find about the evaluator. Waiting costs time and buys nothing |
| Acceptance is step 8, after the whole binding and library | The first host is M6, directly after the gate | The host API is the least tested part of the design. A port finds its faults while one implementation carries them, not three |
| The other two checkers are step 9, the last | They are M8, in the middle | A late checker is how a third implementation stays "mostly compliant" (`01` 3.1). After F5 the type rules are complete, so the port is done once |

## 9. Risks of this plan

| Risk | What reduces it |
| --- | --- |
| Core is large: about 12,400 lines of Go before the first follow-on package, and Core run twice more | It is the part that cannot be added later. M3 and M4 need Core run only, about 10,000 lines each (I), and no checker |
| The gate stalls on one implementation, and Go work stops | M5 and M6's preparation do not wait. If Rust alone lags, the owner may pass the gate on two, with the limit cases of D31 still open |
| Counts filled in from Go carry Go's mistakes into the suite | The other two derive counts from the cost table. A disagreement is examined against the table, never settled by vote |
| A package turns out not to be additive | The check of 2.1 fails in CI. The piece then moves into Core, as the four of 2.3 did, and the owner is told |
| The first host shapes the API to one host | `05` B.8 has six host cases. The other five are checked on paper at M6 |
