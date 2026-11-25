# Risor v3 design: ratification brief for 3.0 (issue #696)

Status: brief for the owner. Nothing in `00-overview.md`, `15-build-rulings.md`
or `spec/` has been marked settled here. Every item below is built as
recommended in all three implementations unless noted otherwise.

**How to use this brief.** Part A lists the 13 items genuinely worth a second
look before 3.0 freezes the language and APIs. Part B batches every remaining
unconfirmed decision and ruling (all built and tested by the suite) so they can
be confirmed in one "yes". Items marked *(added)* were found in the design docs
or spec outside the list in issue #696 so nothing slips past 3.0.

## Part A: Items worth a second look

| ID | Built behavior | Why it matters / strongest alternative | Recommendation |
| --- | --- | --- | --- |
| D59 (`00` 4.3, `21` 1) | Scope statement: v3 is an embedded expression language with named entry points; nothing outlives a run (no kept closures, no event loop, no mutation, no loops). Two tests for every proposal: improves expressions over host data, and readable by a TypeScript programmer without explanation. | Sets the guardrail written into `AGENTS.md` and `00-overview.md` to turn away scripting requests. Alternative: keep scope informal. | Confirm, and add to `AGENTS.md` and `00-overview.md`. |
| D60 *(added)* (`00` 4.3, `21` 4) | Project is named "Risor v3" (`go/v3`, `@deepnoodle/risor`, `risor` crate). | Renaming as expr's successor is practically impossible once 3.0 packages ship. Keeping "Risor v3" may keep drawing v2 scripting requests. | Owner's call (no recommendation in `00`). |
| D27 (`00` 4.1) | 3.0 is final only when Go, TypeScript and Rust all pass the full suite (checker and library included). | All three already pass Core and built packages, but Rust is still `publish = false` at `0.1.0`. Alternative: ship Go and TypeScript 3.0 first and let Rust follow (prior art says a third implementation then drifts). | Confirm: require all three at 3.0.0. |
| D23 (`00` 4.5, `15` 1) | `00` recommends dropping `wonton/assert` so the Go module has zero dependencies. **Actual state:** Go library code imports no `wonton` package, but `go/go.mod` still requires `wonton v0.14.0` (plus 4 indirect deps) for `wonton/assert` in tests **and** `wonton/cli`, `tui`, `color`, `env`, `tty` in `go/cmd/risor`. | Dropping `wonton/assert` alone does not make `go/go.mod` dependency-free unless `cmd/risor` splits into its own module or drops `wonton` too. Alternative: keep `wonton` in `go/go.mod` (zero deps in the library import graph, CLI/tests use `wonton`), or split `cmd/risor` into `go/cmd/risor/go.mod`. | Decide between (1) split `cmd/risor` + drop `wonton/assert` for a 0-dep `go.mod`, or (2) keep `wonton` and reverse D23. |
| D16 (`00` 4.4) | Step and allocation unit counts are fixed per specification version (`04-library.md` 1.2, `03-host.md` 3.6.3). | Prohibits optimizations that change counts and freezes any miscalibrated cost rate until 4.0 (or a new entry name). Directly governs the stability policy (#713). Alternative: leave counts implementation-defined or allow repricing in minor versions. | Confirm: deterministic metering is a headline v3 promise. |
| D37 (`00` 4.3) | `if` requires an `else` branch in 3.0. | Requiring `else` now allows adding optional `else` (giving `null`) later without a break; the reverse is breaking. Alternative: allow `if (c) { a }` as `c ? a : null` in 3.0. | Confirm: keep `else` required in 3.0. |
| D38 (`00` 4.3, `01` 4.1) | Subjectless `match` by conditions (`match { c => e }`) was **dropped** in Core (`01-language.md` 4.1 item 3), while `00` 4.3 still says "stays in the documents for now; I would drop it". `match` by subject cases (+ F17 guards) and `if`/`else if` both stay. | Reconciles `00-overview.md` with Core as built. Alternative: restore condition-form `match` before 3.0. | Confirm dropping it (and update `00` 4.3 to match Core). |
| R374 (`15` 28, `F16` 11.4) | Every `Join` of two records drops all field descriptions, including `Join(T, T)` (so `c ? a : a` drops `a`'s field descriptions). Only operands returned unchanged (`never`, single-side alternative, F17 identity steps, equal unions) keep theirs. | Uniform rule that guarantees three checkers never diverge on descriptions; Go's `Join` shortcut was removed for types with descriptions. Alternative: keep descriptions when both sides are the exact same type (`Join(T, T)`). | Confirm as built, or restore `Join(T, T)` identity preservation. |
| R381 (`15` 28, `F16` 11) | Completion does not show doc-comment or host `Doc` descriptions; hover does. `17` 3.6 originally listed completion's detail as a reader, but F12 3.12 puts the printed type in `detail` and completion candidates have no `doc` field. | Adding `doc` to completion candidates changes every binding's completion record — cheap before 3.0, awkward (or additive field) after. Alternative: add an optional `doc: string \| null` field to completion candidates before 3.0. | Confirm hover-only for 3.0, or add `doc` to completion now. |
| R202 *(added)* (`15` 21, `05` E) | TypeScript `Type` implements Standard Schema v1 (`~standard` runtime property, `ts/src/host/type.ts`), alongside `Infer<X>`. Marked open in `15` R202 because Go and Rust have no counterpart. | Already built and exported in `ts/`. Lets TS libraries (forms, tRPC, etc.) accept Risor types directly, at the cost of a TS-only runtime member. Alternative: remove `~standard` before 3.0 and keep only type-level `Infer<X>`. | Confirm keeping `~standard` as a TS ecosystem convenience. |
| R232 / F16 1.5 #4 *(added)* (`15` 22, `F16` 3.3) | A value of type `any` does **not** fit an annotation (`const x: int = blob` fails with `expected int, found any`; author writes `blob as int`). | Preserves the checker's soundness promise without adding runtime checks to annotations, but is stricter than TypeScript. Alternative: allow `any` with a `dynamic` site (weakens soundness promise) or check annotations at runtime (rejected by `17-5`). | Confirm as built: annotations require a sure fit. |
| R268 & R272 (`15` 22, `F17` 12.5–12.6) | Recursive types: (R268) the 4,096 bound counts pairs **entered** (not pairs "in progress" as `18` 4.2 wrote), with failed-pair caching; (R272) a recursive reference has depth 0 and size 1, and each definition is tested against depth 64 where declared. | Both prevent exponential checker blowup on mutually recursive types (`18` 4.2's wording did not bound work or allow constant-time depth storage). No practical downside (`Tree` still has depth 2). | Confirm both as built. |
| R346 *(added)* (`15` 26) | Go `risor.Func` does not auto-derive signatures for variadic Go functions (`...T`); callers pass `risor.Signature(...)` with a rest parameter instead. | Left as a question because auto-deriving `...T` would change the settled row in `19` 6.1. Adding auto-derivation later is backward-compatible. | Confirm as built (revisit if Go hosts hit friction). |

## Part B: Routine items already built and proven (batch approval)

Every item below is implemented in Go, TypeScript and Rust, covered by the
conformance suite, and has no compelling reason to reopen. A single "yes"
settles the whole batch.

### B.1 Overview decisions (`00-overview.md` section 4)

| ID | Built behavior (all confirmed by suite) |
| --- | --- |
| D2 *(note)* | Build order (Go -> TS/Rust reference evaluators -> checker) is historical; all three implementations are built. |
| D8 *(added)* | `len`, `int` and `float` reject `null` (built in Core; omitted from `00` settled list). |
| D10 | `match` has no binding or structural patterns in 3.0 (guards were added by D54/F17; conditions dropped by D38). Additive later. |
| D11 | `??` beside a comparison or logical operator requires parentheses. |
| D12 | `in` on a list tests membership; parameter/pattern defaults replace `null` as well as absent. |
| D13 | Literal sets are strings only (a `match` on numbers needs a default arm). |
| D14 | Template delimiters are fixed at `${{ }}`. |
| D15 | Text size unit is UTF-16 length in 64-unit blocks (free for short strings in all three languages). |
| D17 | Default limits: 1,000,000 steps, 1,000,000 allocation units, depth 200, no deadline. |
| D18 | No async `run` in TypeScript in 3.0; hosts prefetch using the analysis API. |
| D19 | A host value is either a record or opaque, never both. |
| D20 | A JSON Schema property outside `required` is an optional field unless the host opts into filling absent properties. |
| D21 | Go implementation returns errors and does not use `panic`/`recover` for control flow. |
| D22 | Go typed layer (F11) ships in 3.0. |
| D24 | No standalone LSP server in 3.0; core bindings expose `complete`, `hover` and `describe` directly. |
| D25 | Case mapping uses a spec-owned table pinned to Unicode 16.0.0 (`suite/casemap.json`). |
| D26 | Times in `time` group are seconds since Unix epoch as a number (not nanoseconds). |
| D31 | Source nesting limit is fixed at 100 levels; depth counts each call as 1 + expression nesting (max depth 400 already settled by D58; verified across all three bindings in R214). |

### B.2 `SchemaOf` and doc-comment rulings (`15-build-rulings.md` section 28)

| ID | Built behavior |
| --- | --- |
| R371 | F16 slice D3 is built and offered before slice D2 (`..., F17, F16 slice D3, F19, F12`). |
| R372 | Case format uses result field `schema` (`null` for refusal) rather than a separate `schemaOf` case kind; runner checks the round-trip law automatically. |
| R373 | Round-trip law accepts a recursive type that F1 reads back inlined when not re-entered along a cycle. |
| R375 | Empty doc comments (`/** */`, `/***/`), empty `Doc`, and empty JSON Schema `description` attach nothing; `SchemaOf` never emits `"description":""`. |
| R376 | Hover shows descriptions on single-name `const` bindings and parameters (not destructured pattern names), at binding, read, call and lambda-arg sites. |
| R377 | Field hover reads the receiver's record alternative (null stripped, reference unfolded); union flat records give no field description. |
| R378 | `SchemaOf` writes field descriptions only (not type-line, `const`, or parameter descriptions, which F1 would not round-trip). |
| R379 | `SchemaOf` refuses type variables and colliding recursive type names (`bad-type`); `anyOf` follows type-printing order. |
| R380 | Case field `docs` maps function names, `Type.method`, and opaque type names to host `Doc` strings. |
| R382 | Bindings do not derive descriptions from host types (Go struct tags / Rust doc comments) in 3.0; signature parameter doc comments attach for slice D2. |
| R383 | Types of depth 64 whose `anyOf` pushes schema depth to 65 are emitted by `SchemaOf` and sit outside F1's round-trip law. |
| R384 | Case field `docs` is valid only where `functions` is valid (header, program, completion and hover cases; not format or render cases). |

### B.3 Other open rulings found in `15-build-rulings.md` and `spec/` *(added)*

| ID | Built behavior |
| --- | --- |
| R238 / F16 1.5 #1, #3 | Narrowed parse codes (`type-annotation`, `type-declaration`) do not retire, and their messages were updated in place in `suite/messages-parse.json`. (F16 1.5 #2 was already closed by R261.) |
| R288 | Unreachable `arg-mismatch` template `fn-value` retired from `suite/messages-entry-points.json` once F17 step E3 landed. |
| R303 | Recursive JSON Schema mapping visits `properties` in code-point order and `type` arrays in written order so all three implementations name the same cycle target. |
| R352–R354, R357 | F19 recommended details (`20-4`, `20-5`, `20-9`, `20-11`): hints live in the message (no separate `hint` field), exact table lookup with no edit-distance search, nested host Risor error carries `data` and writes `cause` in JSON. |
| `18-18` (`18` 7.6) | Type variables in non-recursive lambda annotations remain deferred past 3.0 (additive follow-up; `Fn` stays monomorphic in 3.0). |

## Next step once the owner replies

For each confirmed item, update `00-overview.md` section 4 (marking settled and
aligning D10/D38 wording with Core), `15-build-rulings.md`, `F16` section 1.5,
and `AGENTS.md` (for D59 and D23). Any item the owner reverses gets a targeted
follow-up issue before 3.0.0 is tagged.
