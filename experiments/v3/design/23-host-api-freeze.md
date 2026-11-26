# Risor v3 design 23: host API review and freeze (issue #706)

Status: **P1–P9 approved by the owner and applied**; P10 kept as-is; P11
(`~standard` on TypeScript `Type`, R202) **ratified by the owner on
2026-11-25 (#696)**. This document records
the public surface of the Go, TypeScript and Rust bindings, verifies option
defaults against `spec/core/03-host.md`, and tracks the pre-freeze cleanups
for 3.0.0.

**Provenance (measured).** Audited on branch `feat/v3-host-api-freeze` based on
`main` at `3dedfb3b` (which includes #722 F16 slice D2: `Entries`, `Call`,
`CallRecord`, `ExpectEntry`, and #723 hardening). Measured directly from:

- **Go**: `go list ./...`, `go doc -all .`, and `go doc -all ./risortest` with
  Go 1.26.2 (`darwin/arm64`).
- **TypeScript**: `ts/package.json` (`exports`), `ts/src/index.ts`,
  `ts/src/tools/tokens.ts`, `ts/src/testing.ts`, and `ts/src/host/*.ts` under
  Node v22.22.3 and TypeScript 5.9.3.
- **Rust**: `rust/src/lib.rs`, `rust/derive/src/lib.rs`, `rust/src/testing/`,
  and `RUSTFLAGS="-W missing_docs" cargo check --workspace --all-features` /
  `RUSTDOCFLAGS="-D warnings" cargo doc --workspace --no-deps --all-features`
  with Rust 1.99.0.

## 1. Summary of the six review dimensions

1. **Naming and idiom.** Following ruling R201 (`05-host-api.md` Part E), the
   three bindings align closely on canonical names and concepts while staying
   idiomatic in case, environment construction, error delivery and cancellation.
   Only minor idiomatic differences remain (for example, method vs. property
   getters in section 3, and the stray-helper and constant parity gaps in
   section 2, items P8 and P9).
2. **Parity across the three bindings (C1).** All Core stages, F1 to F14, F16
   slices D1–D3, F17 and F19 are present in all three bindings. Four genuine
   behavioural or capability divergences were found during this audit:
   - `Type.isNullable()` on `any` returns `true` in Go and Rust (explicitly
     documented as nullable), but `false` in TypeScript (explicitly documented
     as not nullable by that test) (P1).
   - `Type.equal` on two invalid types returns `false` in Go and TypeScript,
     but `true` in Rust when the messages match (P2).
   - `Expr.Text()` on `Template.part(i)` or `CallSite.args[i]` returns only the
     part's slice in Go, but the whole source text in TypeScript and Rust (P3).
   - `load` / `load_value` accepts a cancellation signal in Go (`ctx`) and
     TypeScript (`signal`), but not in Rust (P7).
3. **Public vs. should-be-internal exports.** Go's host embedding boundary
   (`risor` + `risortest`, everything else in `go/internal/`) is clean; #708
   also added a CLI-only companion package (`risor/risorcli`, outside the host
   API stability promise) so `go/cmd/risor` can live in its own module while
   `go/v3` stays zero-dependency. TypeScript and Rust both leak internals:
   - Rust `risor::testing` makes six submodules `pub mod` and re-exports ~35
     internal conformance-suite runner functions and types (P4).
   - Rust `Value<'h>` is a `pub enum` exposing internal representation variants
     (`Str(Str<'h>)`, `List(Rc<ListVal<'h>>)`, `Map(Rc<MapVal<'h>>)`, `BadNum`)
     and internal helpers (`owns`, `dup`, etc.) referencing private types (P5).
   - TypeScript classes (`Env`, `Program`, `Expr`, `Template`) declare public
     fields/methods marked `/** Internal */` so sibling source files can read
     them; they appear in generated `.d.ts` files and IDE autocomplete (P6).
4. **Error types and codes.** Every binding uses one stage-error type (`*Error`,
   `RisorError`, `Error`), one load-error type (`*LoadError`, `LoadError`,
   `LoadError`), and a plain/dedicated error for malformed descriptions (`error`,
   `Error`, `DescriptionError`). None exports public diagnostic code constants;
   all codes are stable strings (`Diagnostic.Code` / `code`). Two small
   differences: Rust `Diagnostic.cause` holds only nested Risor diagnostics
   (`Option<Box<Diagnostic>>`), not arbitrary host errors; Go exports
   `StepDeclare`..`StepDecode` while TypeScript and Rust use raw numbers.
5. **Options and defaults.** Every normative default of `spec/core/03-host.md`
   matches across all three bindings (section 5). One minor convenience gap:
   Go (`MaxSource`) and Rust (`SOURCE_LIMIT`) export the source-limit constant
   while TypeScript keeps `SOURCE_LIMIT` internal (P9).
6. **Documentation comments.** All missing and stale doc comments on exported
   items across Go, TypeScript and Rust (root crate + `risor-derive` +
   host-facing `risor::testing`) have been fixed on this branch. Notable stale
   comments fixed: Go's `Step` previously claimed `Star` was "never set by this
   package", and Go's `Env.Check` / `Program.CheckResult` omitted `Entries`.
   `RUSTDOCFLAGS="-D warnings" cargo doc --workspace --no-deps --all-features`
   passes with zero warnings, and `RUSTFLAGS="-W missing_docs"` reports zero
   warnings across the entire `risor` root API and `risor-derive`.

## 2. Pre-freeze changes (owner decisions)

The owner approved P1 through P9; all nine have been implemented across Go,
TypeScript and Rust. P10 is kept as-is and P11 (R202) was ratified on
2026-11-25 (#696).

| ID | Binding | Finding | Resolution | Status |
| --- | --- | --- | --- | --- |
| P1 | Go / TS / Rust | `typeAny.isNullable()` returned `false` in TS while Go and Rust returned `true`. | Aligned TS `typeAny.isNullable()` to `true` (matching Go and Rust: `null` is a value of `any`, `false` for invalid types) and documented the rule in `05-host-api.md` E.7. | Done |
| P2 | Rust | `Type::equal` returned `true` for two invalid types with the same message, disagreeing with Go (`false`) and TS (`false`). | Updated Rust `Type::equal` to return `false` whenever either type is invalid, with regression tests. | Done |
| P3 | Go / TS / Rust | `Expr.Text()` on `Template.part(i)` and `CallSite.args[i]` returned the part's slice in Go, but the whole source text in TS and Rust. | Aligned TS `Expr.text` and Rust `Expr::text()` to Go's part-slice behaviour (the program's own span without surrounding whitespace or comments), keeping full-source spans for diagnostics, and documented the rule in `05-host-api.md` E.5. | Done |
| P4 | Rust | `risor::testing` exported 6 `pub mod` submodules and ~35 internal suite-runner functions/types. | Narrowed `risor::testing` to the 8 host-facing F9 items (`load`, `from_text`, `CaseFile`, `Case`, `Report`, `Verdict`, `Difference`, `FileError`), made submodules and internal fields `pub(crate)`, and moved `outcome` binary hooks under `#[doc(hidden)] risor::__private`. | Done |
| P5 | Rust | `Value<'h>` was a `pub enum` exposing internal representation variants and methods naming private types. | Converted `Value<'h>` into an opaque `pub struct` backed by `pub(crate) enum ValueInner<'h>`, with public accessors (`is_null`, `is_list`, `is_map`, `as_bool`, `as_f64`, `as_i64`, `as_str`, `as_opaque`, `len`, `is_empty`, `elem`, `field`, `for_each_key`, `from_data`, `to_data`) and `pub(crate)` internals. | Done |
| P6 | TS | `Env`, `Program`, `Expr`, `Template` and `Analysis` exposed public fields, methods and constructors marked `/** Internal */` in `.d.ts` files. | Moved all internal state and constructors to `#` private fields, `private constructor` and `static { ... }` module-internal closures; `.d.ts` files expose zero `/** Internal */` members on those classes. | Done |
| P7 | Rust | `load_value`, `load` and `load_with` took no `RunOptions`, so Rust hosts could not cancel a config load. | Added `pub run: RunOptions` to `LoadOptions`, updated `load_with` to take `LoadOptions`, and wired `run` through `load_value`, `load` and `load_with`, with cancellation tests. | Done |
| P8 | Rust | `SchemaOptions::from_names`, `Diagnostic::bare`, `Error::one` and `Expr::has_result` were public in Rust without Go/TS counterparts. | Made all four `pub(crate)` (`Program::has_result` stays public, matching Go and TS). | Done |
| P9 | TS / Rust | Constant parity gaps: TS did not export `SOURCE_LIMIT`; TS and Rust did not export load-step constants (`StepDeclare`..`StepDecode`). | Exported `SOURCE_LIMIT` and `STEP_DECLARE`..`STEP_DECODE` from `@deepnoodle/risor`, and `STEP_DECLARE`..`STEP_DECODE` from `risor`. | Done |
| P10 | TS | `LoadOptions.expected` is required in TS, whereas Go `LoadValue` and Rust `LoadOptions::default()` allow omitting it. | Kept as-is: F10 config loads normally specify an expected type. | Kept |
| P11 | TS | Standard Schema `~standard` on `Type` (R202, listed in `22-ratification-brief.md` Part A). | Ratified by the owner on 2026-11-25 (#696): keep `~standard` as a TS ecosystem convenience. | Kept (ratified) |

## 3. Side-by-side parity matrix

All items below are measured from the current code. "—" means the binding has
no counterpart (either by E.1 row 5 or flagged in section 2).

| Area / Concept | Go (`risor`) | TypeScript (`@deepnoodle/risor`) | Rust (`risor`) |
| --- | --- | --- | --- |
| **Environment** | | | |
| Build / child | `NewEnv(decls...)`, `env.With(decls...)` | `createEnv(opts?)`, `env.with(opts?)` | `Env::builder()...build()`, `env.with()...build()`, `Env::default()` |
| Version | `Version(v)` | `version?: string` | `.version(v)` |
| Variables | `Var`, `VarRecord`, `VarSchema`, `VarType` | `vars`, `varRecord`, `varSchemas`, `varTypes` | `.var`, `.var_record`, `.var_schema`, `.var_type`, `.var_host::<T>()` |
| Named types | `NamedType`, `NamedTypeSchema`, `NamedTypeOf` | `namedTypes: NamedType[]` | `.named_type`, `.named_type_schema`, `.named_type_of` |
| Host functions | `Function{Name, Type, Impl, Doc}`, `Func(name, fn, opts...)` | `functions: Record<string, HostFunction>` | `.function(name, sig, impl)`, `.function_doc(name, doc)` |
| Opaque & methods | `OpaqueType{Name, Methods, Doc}`, `Method(t, m, f)`, `Methods[T](name, opts...)` | `opaque: Record<string, OpaqueType \| ChildOpaqueType>` | `.opaque(name)`, `.opaque_doc`, `.method`, `.method_doc` |
| Library control | `Groups(...)`, `Without(...)`, `env.LibraryNames()` | `groups`, `without`, `env.libraryNames()` | `.groups`, `.without`, `env.library_names()` |
| Rejected classes | `Reject(classes...)` (whole set) | `reject?: string[]` (whole set) | `.reject(&[...])` (whole set) |
| Host facts (F14) | `SomeNotNull(names...)` | `someNotNull?: string[][]` | `.some_not_null(&[...])` |
| Limits | `Limits{Steps, Allocation, Deadline, Depth}`, `NoLimit`, `ZeroLimit`, `Budget(s, u)` | `Limits{steps, allocation, deadlineMs, depth}` (`null` = off) | `Limits{steps, allocation, depth, deadline}` (`None` = off), `env.limits()` |
| Description | `env.Describe()` (`[]byte`), `FromDescription(data)` | `env.describe()` (`Description`), `descriptionText(d)`, `Env.fromDescription(d)` | `env.describe()` (`Data`), `env.description_text()`, `Env::from_description(s)` |
| Parse type text | `env.ParseType(text)` | `env.parseType(text)` | `env.parse_type(text)` |
| **Programs & Check** | | | |
| Parse & source limit | `Parse(src, opts...)`, `SourceLimit`, `MaxSource` | `parse(src, opts?)`, `SourceOptions{sourceLimit}`, `SOURCE_LIMIT` | `parse(src)`, `parse_with(src, opts)`, `ParseOptions`, `SOURCE_LIMIT` |
| Parsed program | `*Expr`: `Text()`, `Analyze()` | `Expr`: `text`, `analyze()` | `Expr`: `text()`, `analyze()` |
| Analysis | `Analysis` (`Names`, `Refs`, `Calls`, `Literal`, `IsLiteral`, `Segments`, `Template`), `CallsOf(...)`, `JSON()` | `Analysis` (`names`, `refs`, `calls`, `literal`, `segments`), `callsOf(...)`, `json()` | `Analysis` (`names`, `refs`, `calls`, `literal`, `segments`), `calls_of(&[...])`, `json()` |
| Expected result | `Expect`, `ExpectType`, `ExpectSchema`, `ExpectEntry`, `ExpectEntryType`, `ExpectEntrySchema`, `Condition`, `NoCheck` | `Expected`, `EntryExpected`, `CompileOptions{check, expected, entryExpected}` | `Expected::{None, Condition, text, schema, of}`, `CompileOptions{check, expected, entry_expected}` (`expect_entry*`) |
| Check & compile | `env.Check(x, opts...)`, `env.Compile(x, opts...)` | `env.check(x, expected?, entryExpected?)`, `env.compile(x, opts?)` | `env.check(&x, &exp)`, `env.check_with(&x, &opts)`, `env.compile(&x, opts)` |
| Check result | `CheckResult{Type, Sites, Entries}` | `CheckResult{type, sites, entries}` | `CheckResult{ty, sites, entries}` |
| Compiled program | `*Program`: `CheckResult()`, `Entries()`, `HasResult()` | `Program`: `checkResult`, `entries()`, `hasResult` | `Program`: `check_result()`, `entries()`, `has_result()` |
| **Run & Call** | | | |
| Run & measure | `p.Run(ctx, vars)`, `p.Measure(ctx, vars)` | `p.run(vars?, {signal}?)`, `p.measure(vars?, {signal}?)` | `p.run(&vars)`, `p.run_with`, `p.measure`, `p.measure_with` |
| Entry point calls | `Call`, `CallMeasure`, `CallRecord`, `CallRecordMeasure` | `call`, `callMeasure`, `callRecord`, `callRecordMeasure` | `call`, `call_with`, `call_measure`, `call_measure_with`, `call_record`, `call_record_with`, `call_record_measure`, `call_record_measure_with` |
| Variables | `Vars`, `Map` | `Vars` (object, `ReadonlyMap`, function) | `Vars`, `NoVars` (plus `BTreeMap<String, Data>`, `HashMap<String, T>`) |
| Usage | `Usage{Steps, Allocated}` | `Usage{steps, allocated}` | `Usage{steps, allocated}` |
| **Host calls & values** | | | |
| Call handle | `*Call`: `Context()`, `Charge`, `Allocate`, `Remaining()`, `ErrCallEnded` | `Call`: `signal`, `charge`, `allocate`, `remaining()` (`Remaining`) | `Call`: `cancelled()`, `charge`, `allocate`, `remaining()` (`Remaining`), `Stop` |
| Host error | any `error` | `HostError` (or any `Error`) | `HostError::{Text, Risor}` |
| Result helpers | `Truthy(v)`, `JSONText(v)`, `AsInt(v)`, `AsFloat(v)` | `truthy(v)`, `jsonText(v)` | `truthy(&v)`, `json_text(&v)` |
| **Templates** | | | |
| Parse & inspect | `ParseTemplate(s, opts...)`, `Template` (`Segments`, `IsSingle`, `Part`, `Program`, `Analyze`), `TemplateMode` (`ModeString`, `ModeValue`) | `parseTemplate(s, opts?)`, `Template` (`segments`, `isSingle`, `part`, `program`, `analyze`), `TemplateMode` | `parse_template`, `parse_template_with`, `Template` (`segments`, `is_single`, `part`, `program`, `analyze`), `TemplateMode` |
| Format template | `FormatTemplate(s, opts...)` | `formatTemplate(s, opts?)` | `format_template`, `format_template_with` |
| **Types, Schema & Config** | | | |
| Structural `Type` | `Type`, `TypeKind`, `FieldType`, `TypeNever`..`TypeString`, 9 constructors, `Join`, 16 methods | `Type<T>`, `TypeKind`, `FieldType`, `typeNever`..`typeString`, 9 constructors, `join`, 15 methods + `~standard`, `Infer<X>` | `Type`, `TypeKind`, `FieldType`, `Type::NEVER`..`STRING`, 9 constructors, `join`, 15 methods |
| JSON Schema | `TypeFromSchema(s, opts...)`, `SchemaOption` (`Closed`, `AbsentIsNull`, `DefaultsApplied`), `SchemaOf(t)` | `typeFromSchema(s, opts?)`, `SchemaOption`, `schemaOf(t)`, `schemaText(t)` | `type_from_schema(s, opts)`, `SchemaOptions`, `schema_of(&t)` |
| Conformance | `Conform(v, t, opts...)`, `ConformAll(v, t, max, opts...)`, `ConformSteps(n)` | `conform(v, t, opts?)`, `conformAll(v, t, max, opts?)`, `ConformOptions` | `conform`, `conform_with`, `conform_all`, `conform_all_with`, `ConformOptions` |
| Config load & decode | `Load[T]`, `LoadValue`, `Decode`, `LoadOption` (`LoadType`, `LoadSchema`, `LoadTypeOf`, `MaxFailures`), `LoadError`, `StepDeclare`..`StepDecode` | `loadValue`, `decode`, `LoadOptions`, `LoadError`, `STEP_DECLARE`..`STEP_DECODE` | `load::<T>`, `load_with::<T>`, `load_value`, `decode::<T>`, `Decode`, `DecodeCx`, `LoadOptions`, `LoadError`, `STEP_DECLARE`..`STEP_DECODE` |
| Host-type reflection / derive | `TypeOf[T]`, `Opaque`, `TypedLayer(bool)`, `Func` (`Params`, `Signature`, `Reflective`), `Methods[T]` (`ForMethod`, `Rename`, `Skip`) | `Infer<X>` (compile-time only) | `type_of::<T>`, `#[derive(Host)]`, `Host`, `HostList`, `HostMap`, `View`, `Data`, `OpaqueRef`, `HostType`, `Opaque`, `OpaqueType`, `OptionLike`, `TypeCx`, `RecordType`, `TypeError` |
| **Diagnostics & Tools** | | | |
| Errors & diagnostics | `Error`, `Diagnostic`, `Phase` (`PhaseDeclare`..`PhaseRun`), `Pos`, `Span`, `TraceEntry` | `RisorError`, `Diagnostic`, `Phase`, `Pos`, `Span`, `TraceEntry` | `Error`, `Diagnostic`, `Phase`, `Pos`, `Span`, `TraceEntry`, `PathStep` |
| Renderings | `RenderLine`, `RenderDetail`, `RenderJSON` | `renderLine`, `renderDetail`, `renderJSON` | `render_line`, `render_detail`, `render_json` |
| Format / Assist | `Format`, `env.Complete`, `env.Hover`, `Completion`, `Candidate`, `Hover` | `format`, `env.complete`, `env.hover`, `Completion`, `Candidate`, `Hover`, plus `@deepnoodle/risor/tokens` | `format`, `format_with`, `env.complete`, `env.complete_with`, `env.hover`, `env.hover_with`, `Completion`, `Candidate`, `Hover` |
| **Testing (separate entry point)** | `risortest`: `Run`, `Load`, `Parse`, `File`, `Case`, `Verdict`, `Diff`, `Origin`, `Option` (`Base`, `Filter`, `Sources`, `Update`, `Vars`) | `@deepnoodle/risor/testing`: `loadCaseFile`, `caseFile`, `caseFileText`, `defineCases`, `CaseFile`, `Case`, `Verdict`, `Difference`, `CaseOptions` | `risor::testing`: `load`, `from_text`, `CaseFile`, `Case`, `Report`, `Verdict`, `Difference`, `FileError` |

## 4. Error types and codes

| Aspect | Go | TypeScript | Rust |
| --- | --- | --- | --- |
| Stage failure type | `*Error` (`Diagnostics []Diagnostic`; `Error() string`, `Phase() Phase`) | `RisorError extends Error` (`phase`, `diagnostics`, `usage?`) | `Error` (`phase`, `diagnostics`; `first()`, `Display`, `std::error::Error`) |
| Load failure type | `*LoadError` (`Step int`, `Diagnostics`, `Usage`, `HasUsage`; `Unwrap() *Error`) | `LoadError extends RisorError` (`step: number`, inherited `phase`, `diagnostics`, `usage?`) | `LoadError` (`step: u8`, `diagnostics`, `usage: Option<Usage>`; `Display`, `std::error::Error`) |
| Description form error | Plain `error` (invalid JSON/shape); `*Error` (declare errors) | Plain `Error` (invalid shape); `RisorError` (declare errors) | `DescriptionError::Form(String)` or `DescriptionError::Declare(Error)` |
| Host function error | Any `error` (or `*Error` wrapped via `%w`) | `HostError` or any `Error` (or `RisorError`) | `HostError::Text(String)` or `HostError::Risor(Box<Error>)` |
| Budget-exceeded signal in host call | `*eval.LimitError` (unexported type) or `ErrCallEnded` | Internal throw (must not be caught by host) | `Result<(), Stop>` |
| `Diagnostic.cause` | `error` (any host error or `*Error`) | `unknown` (any thrown Error or inner `Diagnostic`) | `Option<Box<Diagnostic>>` (inner Risor diagnostic only) |
| Public code constants | None (strings on `Diagnostic.Code`) | None (strings on `Diagnostic.code`) | None (static strings on `Diagnostic.code`) |
| `conform` return shape | `error` (`nil` or `*Error`) | `Diagnostic \| null` | `Result<(), Diagnostic>` |
| `conformAll` return shape | `[]Diagnostic` | `Diagnostic[]` | `Vec<Diagnostic>` |

## 5. Verification of option defaults

Every default was verified directly in source code against
`spec/core/03-host.md` 3.6.1, `spec/packages/f04-conversion.md` 7.3,
`spec/packages/f08-errors.md` 6.1 and `spec/packages/f10-config.md` 5.2.

| Option | Spec default | Go (verified) | TypeScript (verified) | Rust (verified) | Match |
| --- | --- | --- | --- | --- | --- |
| `version` | `"3.0"` (newest) | `"3.0"` (`go/env.go:301`) | `"3.0"` (`ts/src/host/env.ts:23`) | `"3.0"` (`rust/src/env.rs:607`) | Yes |
| `limits.steps` | `1,000,000` | `1_000_000` (`go/internal/eval/run.go:39`) | `1_000_000` (`ts/src/host/env.ts:38`) | `Some(1_000_000)` (`rust/src/env.rs:62`) | Yes |
| `limits.allocation` | `1,000,000` | `1_000_000` (`go/internal/eval/run.go:39`) | `1_000_000` (`ts/src/host/env.ts:38`) | `Some(1_000_000)` (`rust/src/env.rs:63`) | Yes |
| `limits.depth` | `200` (max `400`) | `200`, max `400` (`go/internal/eval/run.go:39`, `go/internal/diag/diag.go:176`) | `200`, max `400` (`ts/src/host/env.ts:38,41`) | `200`, max `400` (`rust/src/env.rs:64,945`) | Yes |
| `limits.deadline` | Off | `0` / off (`go/internal/eval/run.go:39`) | `null` / off (`ts/src/host/api.ts:351`) | `None` / off (`rust/src/env.rs:65`) | Yes |
| `sourceLimit` | `1,048,576` (max same) | `1 << 20` (`go/source.go:13`) | `1_048_576` (`ts/src/syntax/source.ts:128`) | `1_048_576` (`rust/src/syntax/parser.rs:59`) | Yes |
| `reject` | `["constant-condition"]` | `["constant-condition"]` (`go/env.go:225`) | `["constant-condition"]` (`ts/src/host/env.ts:158`) | `["constant-condition"]` (`rust/src/env.rs:962`) | Yes |
| `compile` check | Checked (`true`) | Checked unless `NoCheck` (`go/check.go:311`) | `check !== false` (`ts/src/host/api.ts:1195`) | `check: true` (`rust/src/api.rs:201`) | Yes |
| `conform` step limit | `1,000,000` | `eval.DefaultLimits.Steps` (`go/conversion.go:302`) | `CONFORM_STEPS = 1_000_000` (`ts/src/eval/conform.ts:33`) | `Some(FREE_STEPS)` (`rust/src/typ.rs:592`) | Yes |
| `conform` depth limit | `200` | `DefaultLimits.Depth` (`go/internal/eval/conform.go:145`) | `CONFORM_DEPTH = 200` (`ts/src/eval/conform.ts:27`) | `FREE_DEPTH = 200` (`rust/src/conform.rs:30`) | Yes |
| `conformAll` `max < 1` | Read as `1` | Clamped to `1` (`go/conversion.go:352`) | `max >= 1 ? Math.floor(max) : 1` (`ts/src/host/api.ts:1595`) | `max.max(1)` (`rust/src/typ.rs:642`) | Yes |
| `load` `max` failures | `1` | `1` (`go/config.go:145`) | `options.max ?? 1` (`ts/src/host/api.ts:1636`) | `max: 1` (`rust/src/config.rs:52`) | Yes |
| `SchemaOptions` | All `false` | Empty `opts...` (`go/hosttypes.go:59`) | `options = []` (`ts/src/host/api.ts:388`) | All `false` (`rust/src/schema.rs:24`) | Yes |
| `TypedLayer` (Go F11) | On (`true`) | `untyped: false` (`go/env.go:281`) | — | — | Yes |
| `LoadOptions.decode` (TS) | `true` | — (separate `Load[T]` / `LoadValue`) | `options.decode !== false` (`ts/src/host/api.ts:1646`) | — (separate `load` / `load_value`) | Yes |

## 6. Full public API listing for owner approval

### 6.1 Go (`github.com/deepnoodle-ai/risor/go/v3`)

Two public host library packages (`risor` and `risor/risortest`); one CLI-only
companion package (`risor/risorcli`, outside the frozen host embedding API);
23 `internal/` packages; one in-module binary (`cmd/outcome`). The `risor` CLI
lives in its own module (`github.com/deepnoodle-ai/risor/go/cmd/risor` in
`go/cmd/risor/`, split out in #708 so `go/v3` has zero external dependencies).

#### Package `risor` (32 constants, 10 variables, 46 types, 15 standalone functions)

- **Constants (32):**
  - Load steps (6): `StepDeclare`, `StepParse`, `StepCheck`, `StepRun`,
    `StepConform`, `StepDecode`.
  - Phases (4): `PhaseDeclare`, `PhaseParse`, `PhaseCheck`, `PhaseRun`.
  - Limits (3): `MaxSource`, `NoLimit`, `ZeroLimit`.
  - `SchemaOption` (3): `Closed`, `AbsentIsNull`, `DefaultsApplied`.
  - `TemplateMode` (2): `ModeString`, `ModeValue`.
  - `TypeKind` (14): `KindNever`, `KindAny`, `KindNull`, `KindBool`, `KindInt`,
    `KindFloat`, `KindString`, `KindList`, `KindRecord`, `KindUnion`,
    `KindOpaque`, `KindVar`, `KindMixed`, `KindInvalid`.
- **Variables (10):**
  - Primitive types (7): `TypeNever`, `TypeAny`, `TypeNull`, `TypeBool`,
    `TypeInt`, `TypeFloat`, `TypeString`.
  - Check options (2): `Condition`, `NoCheck`.
  - Sentinel error (1): `ErrCallEnded`.
- **Standalone functions (15; constructors grouped under their return types below):**
  - Values & schema (5): `Truthy(v)`, `JSONText(v)`, `AsInt(v)`, `AsFloat(v)`,
    `SchemaOf(t)`.
  - Conformance & config (5): `Conform(v, t, opts...)`, `ConformAll(v, t, max, opts...)`,
    `Decode(v, dst)`, `Load[T](ctx, env, source, vars, dst, opts...)`,
    `LoadValue(ctx, env, source, vars, opts...)`.
  - Formatting & rendering (5): `Format(source, opts...)`, `FormatTemplate(source, opts...)`,
    `RenderLine(ds)`, `RenderDetail(ds, source...)`, `RenderJSON(ds)`.
- **Environment & declarations:**
  - `Env` (`NewEnv(decls...)`; methods: `With`, `Check`, `Compile`, `Complete`,
    `Hover`, `Describe`, `LibraryNames`, `ParseType`).
  - `Decl` constructors: `Budget`, `FromDescription`, `Groups`, `Method`,
    `NamedType`, `NamedTypeOf`, `NamedTypeSchema`, `Reject`, `SomeNotNull`,
    `TypedLayer`, `Var`, `VarRecord`, `VarSchema`, `VarType`, `Version`,
    `Without`, plus struct `Decl`s `Function`, `OpaqueType`, `Limits`.
  - `Function` (`Name`, `Type`, `Impl`, `Doc`; `Func(name, fn, opts...)`,
    `Reflective()`), `FuncOption` (`Params`, `Signature`), `Native`.
  - `OpaqueType` (`Name`, `Methods`, `Doc`; `Methods[T](name, opts...)`),
    `MethodOption` (`ForMethod`, `Rename`, `Skip`), `Opaque` (`OpaqueType() string`).
  - `Limits` (`Steps`, `Allocation`, `Deadline`, `Depth`).
  - `LibraryNames` (`Functions`, `Namespaces`, `GroupFunctions`, `Methods`).
- **Programs, analysis & entry points:**
  - `Expr` (`Parse(source, opts...)`; `Analyze()`, `Text()`), `ParseOption`,
    `SourceLimit`.
  - `Analysis` (`Names`, `Refs`, `Calls`, `Literal`, `IsLiteral`, `Segments`,
    `Template`; `CallsOf(names...)`, `JSON()`), `Ref`, `Step`, `CallSite`.
  - `CheckOption` (`Expect`, `ExpectType`, `ExpectSchema`, `ExpectEntry`,
    `ExpectEntryType`, `ExpectEntrySchema`, `Condition`, `NoCheck`).
  - `CheckResult` (`Type`, `Sites`, `Entries`), `Site`, `Entry`, `EntryParam`.
  - `Program` (`CheckResult`, `Entries`, `HasResult`, `Run`, `Measure`, `Call`,
    `CallMeasure`, `CallRecord`, `CallRecordMeasure`).
  - `Vars` (`Var(name) (any, bool)`), `Map`, `Usage` (`Steps`, `Allocated`),
    `Call` (`Context`, `Charge`, `Allocate`, `Remaining`).
- **Templates:**
  - `Template` (`ParseTemplate(text, opts...)`; `Segments`, `IsSingle`, `Part`,
    `Program`, `Analyze`), `TemplateMode`, `Segment`.
- **Types, conformance & config:**
  - `Type` constructors: `ExactRecordType`, `Join`, `ListType`, `LiteralType`,
    `MapType`, `Nullable`, `OpaqueTypeNamed`, `RecordType`, `TaggedUnionType`,
    `TypeFromSchema`, `TypeOf[T]`, `UnionType`.
  - `Type` methods: `Alternatives`, `Elem`, `Equal`, `Err`, `Field`, `Fields`,
    `Fits`, `IsExact`, `IsNullable`, `IsZero`, `Kind`, `Literals`, `NonNull`,
    `OpaqueName`, `Rest`, `String`, `Tag`, `Variants`.
  - `TypeKind`, `FieldType`, `SchemaOption`.
  - `ConformOption` (`ConformSteps`).
  - `LoadOption` (`LoadSchema`, `LoadType`, `LoadTypeOf`, `MaxFailures`),
    `LoadError` (`Step`, `Diagnostics`, `Usage`, `HasUsage`; `Error`, `Unwrap`).
- **Diagnostics & assist:**
  - `Error` (`Diagnostics`; `Error`, `Phase`), `Diagnostic` (`Phase`, `Code`,
    `Class`, `Span`, `Message`, `Path`, `Data`, `HasData`, `Trace`, `Cause`;
    `String`), `Phase`, `Pos` (`Line`, `Column`), `Span` (`Start`, `End`;
    `Array`, `String`), `TraceEntry` (`Span`, `Name`, `Index`).
  - `Completion` (`Span`, `Items`), `Candidate` (`Label`, `Kind`, `Detail`,
    `Insert`), `Hover` (`Kind`, `Span`, `Type`, `Signature`, `Doc`,
    `Declaration`).

#### Package `risor/risortest` (1 top-level function, 7 types)

- `Run(t, base, path, opts...)`.
- `File` (`Load(path, opts...)`, `Parse(text, program, opts...)`; `Cases()`,
  `Update(verdicts)`).
- `Case` (`Name()`, `Run(ctx)`).
- `Option` (`Base`, `Filter`, `Sources`, `Update`, `Vars`).
- `Verdict` (`Name`, `Status`, `Diffs`, `Fixture`, `Message`, `Reason`,
  `Expect`; `Report()`), `Diff` (`Name`, `Expected`, `Found`), `Origin`
  (`Name`, `Line`, `Column`, `Indent`).

#### Package `risor/risorcli` (CLI-only companion, outside frozen host API; 5 functions, 5 types)

Added by #708 so `github.com/deepnoodle-ai/risor/go/cmd/risor` can live in its
own module without importing `go/internal/` while `go/v3` stays
zero-dependency. It is not part of the host embedding API stability promise
(hosts embed `risor` and test with `risortest`) and exports only the minimal
surface `go/cmd/risor` calls:

- Strict JSON & value rendering: `DecodeJSON(data)`, `RenderText(env, v)`.
- REPL session: `Session` (`NewSession(env, vars)`; `Eval`, `Reset`,
  `EOFError`), `EntryResult` (`Value`, `Diagnostics`, `Err`, `More`).
- Case-file & suite runner: `RunCaseFile`, `CaseFileOutcome` (`Results`,
  `Report`, `UpdatedText`, `Written`, `UpdateErr`), `CaseResult` (`File`,
  `Case`, `Status`, `Diffs`, `Fixture`, `Message`, `Reason`), `RunSuite`,
  `SuiteSection` (`Rel`, `Results`, `Err`).

### 6.2 TypeScript (`@deepnoodle/risor`)

Three public subpath entry points in `ts/package.json`: `"."`, `"./tokens"`,
and `"./testing"`.

#### Entry point `@deepnoodle/risor` (51 value exports, 39 type exports)

- **Environment:**
  - Values: `createEnv`, `Env` (`static fromDescription`, `with`, `describe`,
    `libraryNames`, `parseType`, `check`, `compile`, `complete`, `hover`),
    `descriptionText`.
  - Types: `EnvOptions`, `WithOptions`, `LibraryNames`, `ChildOpaqueType`,
    `NamedType`, `Limits`, `Description`, `HostFunction`, `OpaqueType`.
- **Programs & analysis:**
  - Values: `parse`, `SOURCE_LIMIT`, `Expr` (`text`, `analyze`), `Analysis`
    (`names`, `refs`, `calls`, `literal`, `segments`, `callsOf`, `json`),
    `Program` (`checkResult`, `hasResult`, `entries`, `run`, `measure`, `call`,
    `callMeasure`, `callRecord`, `callRecordMeasure`).
  - Types: `CallSite`, `Ref`, `Expected`, `EntryExpected`, `Entry`,
    `EntryParam`, `CheckResult`, `Site`, `CompileOptions`, `SourceOptions`,
    `Usage`, `Vars`.
- **Templates:**
  - Values: `parseTemplate`, `Template` (`isSingle`, `segments`, `program`,
    `part`, `analyze`), `formatTemplate`.
  - Types: `TemplateMode`, `Segment`.
- **Host calls & values:**
  - Values: `HostError`, `truthy`, `jsonText`.
  - Types: `Call`, `Remaining`.
- **Types, conformance & config:**
  - Values: `Type` (`kind`, `elem`, `fields`, `field`, `rest`, `alternatives`,
    `variants`, `literals`, `tag`, `nonNull`, `isNullable`, `isExact`,
    `opaqueName`, `fits`, `equal`, `toString`, `err`, `~standard`),
    `typeNever`, `typeAny`, `typeNull`, `typeBool`, `typeInt`, `typeFloat`,
    `typeString`, `listType`, `mapType`, `nullable`, `literalType`,
    `recordType`, `exactRecordType`, `unionType`, `taggedUnionType`,
    `opaqueTypeNamed`, `join`, `typeFromSchema`, `schemaOf`, `schemaText`,
    `conform`, `conformAll`, `loadValue`, `decode`, `LoadError`,
    `STEP_DECLARE`, `STEP_PARSE`, `STEP_CHECK`, `STEP_RUN`, `STEP_CONFORM`,
    `STEP_DECODE`.
  - Types: `TypeKind`, `FieldType`, `Infer`, `ConformOptions`, `LoadOptions`,
    `SchemaOption`.
- **Diagnostics & tools:**
  - Values: `RisorError`, `renderLine`, `renderDetail`, `renderJSON`, `format`.
  - Types: `Diagnostic`, `Phase`, `Span`, `Pos`, `TraceEntry`, `Completion`,
    `Candidate`, `Hover`.

#### Entry point `@deepnoodle/risor/tokens` (1 function, 2 types)

- `tokens(text: string): ColourToken[]`, `ColourToken`, `TokenClass`.

#### Entry point `@deepnoodle/risor/testing` (4 functions, 5 types)

- `loadCaseFile`, `caseFile`, `caseFileText`, `defineCases`.
- `CaseOptions`, `Difference`, `Verdict`, `Case`, `CaseFile`.

### 6.3 Rust (`risor` + `risor-derive`)

Crate root (`rust/src/lib.rs`) exports 103 items (+ `#[derive(Host)]` behind
feature `derive`), one public module (`testing`), and `#[doc(hidden)] pub mod
__private`.

#### Crate root `risor` (103 items + `derive::Host`)

- **Environment (5):** `Env` (`builder`, `default`, `with`, `limits`,
  `parse_type`, `library_names`, `describe`, `description_text`,
  `from_description`, `check`, `check_with`, `compile`, `complete`,
  `complete_with`, `hover`, `hover_with`), `EnvBuilder` (`version`, `var`,
  `var_record`, `var_schema`, `var_type`, `var_host`, `function`,
  `function_doc`, `opaque`, `opaque_doc`, `method`, `method_doc`, `named_type`,
  `named_type_schema`, `named_type_of`, `groups`, `without`, `reject`,
  `some_not_null`, `limits`, `build`), `Limits`, `LibraryNames`,
  `DescriptionError`.
- **Programs & analysis (20):** `parse`, `parse_with`, `Expr` (`text`,
  `analyze`), `ParseOptions`, `SOURCE_LIMIT`, `Analysis` (`names`, `refs`,
  `calls`, `literal`, `segments`, `calls_of`, `json`), `CallSite`, `Ref`,
  `Step`, `Expected` (`None`, `Condition`, `text`, `schema`, `of`),
  `CompileOptions` (`check`, `expected`, `entry_expected`, `expect_entry`,
  `expect_entry_type`, `expect_entry_schema`), `CheckResult`, `Site`, `Entry`,
  `EntryParam`, `Program` (`has_result`, `entries`, `check_result`, `run`,
  `run_with`, `measure`, `measure_with`, `call`, `call_with`, `call_measure`,
  `call_measure_with`, `call_record`, `call_record_with`,
  `call_record_measure`, `call_record_measure_with`), `RunOptions`, `Usage`,
  `Vars`, `NoVars`.
- **Templates (7):** `parse_template`, `parse_template_with`, `Template`
  (`segments`, `is_single`, `part`, `program`, `analyze`), `TemplateMode`,
  `Segment`, `format_template`, `format_template_with`.
- **Host calls & values (9):** `Call` (`charge`, `allocate`, `remaining`,
  `cancelled`), `Remaining`, `Stop`, `Data`, `HostError`, `OpaqueRef`, `Value`
  (opaque `pub struct`: `from_data`, `to_data`, `is_null`, `is_list`,
  `is_map`, `as_bool`, `as_f64`, `as_i64`, `as_str`, `as_opaque`, `len`,
  `is_empty`, `elem`, `field`, `for_each_key`), `truthy`, `json_text`.
- **Types, conformance & config (35):** `Type` (`NEVER`, `ANY`, `NULL`, `BOOL`,
  `INT`, `FLOAT`, `STRING`; `err`, `kind`, `elem`, `fields`, `field`, `rest`,
  `alternatives`, `variants`, `literals`, `tag`, `non_null`, `is_nullable`,
  `is_exact`, `opaque_name`, `fits`, `equal`, `Display`), `TypeKind`,
  `FieldType`, `list_type`, `map_type`, `nullable`, `literal_type`,
  `record_type`, `exact_record_type`, `union_type`, `tagged_union_type`,
  `opaque_type_named`, `join`, `type_from_schema`, `schema_of`,
  `SchemaOptions`, `conform`, `conform_with`, `conform_all`,
  `conform_all_with`, `ConformOptions`, `load`, `load_with`, `load_value`,
  `LoadOptions`, `LoadError`, `STEP_DECLARE`, `STEP_PARSE`, `STEP_CHECK`,
  `STEP_RUN`, `STEP_CONFORM`, `STEP_DECODE`, `decode`, `Decode`, `DecodeCx`.
- **Host-type conveniences (12 + derive):** `Host`, `HostList`, `HostMap`,
  `View`, `type_of`, `HostType`, `Opaque`, `OpaqueType`, `OptionLike`,
  `RecordType`, `TypeCx`, `TypeError`, plus `#[cfg(feature = "derive")] Host`
  (from `risor-derive`, supporting `#[risor(rename = "...")]`, `skip`,
  `absent_if_none`, `default`).
- **Diagnostics & tools (15):** `Error` (`phase`, `diagnostics`, `first`),
  `Diagnostic` (`phase`, `code`, `class`, `span`, `message`, `data`, `path`,
  `trace`, `cause`; `trace()`), `Phase`, `Pos`, `Span`, `TraceEntry`,
  `PathStep`, `format`, `format_with`, `render_line`, `render_detail`,
  `render_json`, `Completion`, `Candidate`, `Hover`.

#### Module `risor::testing` (8 items)

- **Host-facing F9 API:** `load`, `from_text`, `CaseFile` (`cases`, `filter`,
  `run`), `Case` (`name`, `run`), `Report` (`verdicts`, `names`, `text`,
  `updated_text`, `assert_ok`), `Verdict` (`label`), `Difference`, `FileError`.
