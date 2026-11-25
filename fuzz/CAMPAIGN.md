# Pre-3.0 Freeze Fuzz Campaign (#703)

This record documents the pre-freeze differential and checker-soundness fuzz
campaign run across all three Risor v3 implementations (Go, TypeScript, and
Rust) after extending `fuzz/internal/outcome` to compare every field emitted on
an outcome line.

## Environment

- **Base commit:** `3dedfb3b` (`origin/main`, including F16 slice D2 `export const` and #704 hardening)
- **Date:** 2026-11-25
- **Go:** `go1.26.2 darwin/arm64`
- **Node.js:** `v22.22.3`
- **Rust:** `rustc 1.99.0 (b940084d7 2026-09-28)`, `cargo 1.99.0 (5f94df478 2026-08-27)`
- **Outcome binaries:** compiled Go (`go build ./cmd/outcome`), compiled Rust (`cargo build --release --bin outcome`), TypeScript (`node scripts/outcome.ts`)

## Expanded Outcome Comparator

`fuzz/internal/outcome` now parses and compares every top-level and diagnostic
field written by any `outcome` command (`suite/README.md`, F1–F19), checking
both presence and value (with numbers compared as IEEE 754 doubles and map keys
as sets):

- **Program lines:** `name`, `description`, `type`, `sites`, `phase`, `value`,
  `diagnostics`, `steps`, `units`, `analysis` (presence mismatch on
  `description`, `analysis`, `diagnostics.message`, `diagnostics.trace`, and
  `diagnostics.path` is now a finding).
- **Conform lines (F4 9.5, F8 8):** `conforms`, `path`, `message`, `agree`,
  and full `failures` lists (length, order, `path`/`code`, and `message`,
  covering the limit entry added in #688).
- **Render lines (F8 8):** `line`, `detail`, `json` (no `phase` field).
- **Runner lines (F9 9.4):** `invalid`, `verdicts`, `report`, `text` (no `phase`
  field).
- **Tools lines (F12 7.7):** `formatted`, `completion`, `hover` (no `phase`
  field on success), plus parse/declare diagnostics on failure.
- **Diagnostics:** `code`, `span`, `class`, `message`, `path`, `trace`, plus
  any additional diagnostic key. For run failures of class `limit`, `span`,
  `trace`, `steps`, and `units` remain uncompared (`03-host.md` 3.6.1/3.6.3,
  `suite/README.md` item 2, `f08-errors.md` 8).
- **Catch-all:** any future or extra top-level field on an outcome line is
  compared in sorted key order so no field can be silently ignored.

## Findings Discovered and Fixed

Every discrepancy surfaced by the strict full-suite replay and the fuzz
campaign was investigated, fixed, and covered by conformance suite regression
cases before running the final clean campaign:

| # | Source | Implementation(s) | Root cause & fix | Regression test |
| --- | --- | --- | --- | --- |
| 1 | Suite replay (`core-run/parse-source-size.json`, `templates/source-size.json`) | TypeScript | `analysisOf` ignored `limits.source`, emitting `analysis` on `source-too-large` cases where Go and Rust omitted it (`ts/src/suite/runner.ts`). Passed `limits.source` and rejected invalid source limits. | Verified via full suite replay |
| 2 | Suite replay (`config/`, `conversion/`, `unions/`, `recursion/`, `entry-points/`) | Rust, TypeScript | `outcome` disagreed on `diagnostics[].path` and `diagnostics[].trace`: Rust emitted `path` on in-program `as` failures (non-null span) and `trace: []` on `load` step 5a failures; TypeScript and Rust emitted `trace: []` alongside `path` on `entry` argument conformance failures while Go omitted `trace`. Aligned all three to emit `path` (no `trace`) for host-boundary `not-conforming` (`span == null`) and `trace` (no `path`) for in-program non-limit run failures (`rust/src/testing/run.rs`, `ts/src/suite/outcome.ts`). Also aligned analysis parse-failure diagnostics to emit `message`. | Verified via full suite replay |
| 3 | Suite replay (`tools/format-*.json`, `tools/source-size.json`) | Rust | `tools::outcome` omitted `message` on `format`/`formatTemplate` parse diagnostics (`rust/src/testing/tools.rs`). Added `message` per F12 7.7 and F8 8. | Verified via full suite replay |
| 4 | Suite replay (`core-check/check-calls.json` etc.) | Rust | Unmatched stub call error omitted the stub name (`no row of the stub matches the call` vs `no row of the stub <name> matches the call`, `rust/src/testing/run.rs`). Included `{name}`. | Verified via full suite replay |
| 5 | Suite replay (`core-run/parse-reserved.json`) | Rust | Top-level `is => 1` used `reserved-word` template `operand` instead of `name` (`rust/src/syntax/parser.rs`). Checked for following `=>` in `operand()`. | `suite/errors/fuzz-round-7-check.json` |
| 6 | Suite replay (`config-layering.json`, `config-unions.json`, `check-spread-fit.json`, `unions-form.json`) | Rust, TypeScript | Rust reported outer record/union types instead of field/tag types in `result-mismatch`/`arg-mismatch` on spreads and tagged-union map literals (`rust/src/check/mod.rs`). On a spread giving an unlisted key against a closed record, TypeScript reported `expected never, found <T>` instead of `{expected} does not allow key {key}` (`ts/src/types/check.ts`) and did not sort keys in code-point order. Fixed both to match `f08-errors.md` 3.5. | `suite/errors/fuzz-round-7-check.json` |
| 7 | Differential seed 1 (`r0-c154`) | TypeScript | On an extra key in a variant of a recursive tagged union (`type Rt0 = {k: ""} \| {k: "a", l: Rt0}`), TypeScript printed `Rt0 does not allow key "r"` instead of `{k: ""} does not allow key "r"` (`ts/src/types/check.ts`). Restricted reference printing to non-union record aliases per F17 12.6 item 4. | `suite/recursion/e2-check.json` |
| 8 | Differential seeds 1 (`r2-c379`), 42 (`r6-c332`), 703 (`r2-c331`, `r5-c177`) | Go | For `export const` entry points with an explicit return annotation (`lam.Result != nil`), Go fitted `entryReturns` via `fitForm` descending into `lam.Body` instead of fitting the annotated return type directly at `lam.Body.Span()` (`go/internal/check/expr.go`). Fixed to match F16 12.3, TypeScript, and Rust. | `suite/entry-points/d2-check.json` |
| 9 | Soundness seed 3 (`s6-82-v0`, differential check) | Go | `base64.encode` on strings of >= 64 code points under-charged 1 step in Go because `fenv.Weight("s")` and `newQuick` read `c.recv` even for non-method functions whose parameter is named `s` (`go/internal/eval/lib.go`). Scoped the `s -> recv` shortcut to methods (`Receiver != ""`). | `suite/groups/groups-examples.json` |

### Spec-Allowed Difference (Ruling)

Four cases in `suite/core-run/declare-names.json` intentionally test
declarations with both a refused name (`reserved-namespace` or `bad-name`) and
bad type/signature text. By `spec/core/03-host.md` 3.3.4 item 4, whether the
secondary `bad-signature`/`bad-type` diagnostic is also reported is unspecified
(Go and TypeScript report only the name error; Rust reports both). This is
explicitly allowed by the specification; all other 305 of 306 suite files match
identically across every outcome field in Go, TypeScript, and Rust.

## Final Clean Campaign Results

All commands were executed from `fuzz/` against the built Go, TypeScript, and
Rust `outcome` commands with `-impls go,ts,rust` and the expanded all-field
comparator.

### 1. Differential — Default Full Mix (All Packages)

Command template:
```sh
GOWORK=off go run ./cmd/fuzz -seed <SEED> -cases 500 -rounds 10
```

| Seed | Typed | Loose | Mangled | Total Cases | Findings | Duration |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| `1` | 2,523 | 1,436 | 1,041 | 5,000 | 0 | 110s |
| `2` | 2,480 | 1,516 | 1,004 | 5,000 | 0 | 64s |
| `3` | 2,513 | 1,450 | 1,037 | 5,000 | 0 | 52s |
| `42` | 2,468 | 1,507 | 1,025 | 5,000 | 0 | 95s |
| `703` | 2,554 | 1,444 | 1,002 | 5,000 | 0 | 66s |
| `1001` | 2,522 | 1,479 | 999 | 5,000 | 0 | 23s |
| `2026` | 2,503 | 1,499 | 998 | 5,000 | 0 | 42s |
| **Subtotal** | **17,563** | **10,331** | **7,106** | **35,000** | **0** | **452s** |

### 2. Differential — Targeted Stress Modes

| Mode | Flags | Cases | Findings | Duration |
| --- | --- | ---: | ---: | ---: |
| Typed + limits (50%) | `-seed 101 -typed 1 -loose 0 -mangled 0 -limits 50 -nocheck 0 -cases 500 -rounds 10` | 5,000 | 0 | 137s |
| Loose + unchecked (50%) | `-seed 202 -typed 0 -loose 1 -mangled 0 -limits 25 -nocheck 50 -cases 500 -rounds 10` | 5,000 | 0 | 48s |
| Mangled (100%) | `-seed 303 -typed 0 -loose 0 -mangled 1 -cases 500 -rounds 10` | 5,000 | 0 | 25s |
| Long flat chains (50%, 1k–20k links) | `-seed 404 -typed 1 -loose 0 -mangled 0 -flat 50 -limits -1 -cases 50 -rounds 10` | 500 | 0 | 140s |
| **Subtotal** | | **15,500** | **0** | **350s** |

### 3. Checker Soundness (`-sound`, 5 Conforming Value Sets per Program)

Command template:
```sh
GOWORK=off go run ./cmd/fuzz -sound -seed <SEED> -cases 500 -rounds 10
```

| Seed | Programs | Checked Clean | Value-Set Runs | Undecided | Violations | Diff Disagreements | Duration |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| `1` | 5,000 | 4,512 | 25,000 | 0 | 0 | 0 | 94s |
| `2` | 5,000 | 4,504 | 25,000 | 0 | 0 | 0 | 56s |
| `3` | 5,000 | 4,518 | 25,000 | 0 | 0 | 0 | 42s |
| `42` | 5,000 | 4,458 | 25,000 | 0 | 0 | 0 | 51s |
| `703` | 5,000 | 4,471 | 25,000 | 0 | 0 | 0 | 63s |
| `1001` | 5,000 | 4,460 | 25,000 | 0 | 0 | 0 | 52s |
| `2026` | 5,000 | 4,491 | 25,000 | 0 | 0 | 0 | 29s |
| **Subtotal** | **35,000** | **31,414** | **175,000** | **0** | **0** | **0** | **387s** |

### 4. Conformance Suite Replay (`-replay`)

- **Files replayed:** 306 case files under `suite/*/*.json`
- **Clean agreement across every outcome field:** 305/306 files (only the 4
  spec-unspecified `declare-names.json` cases under `03-host.md` 3.3.4 item 4
  differ in `declare` diagnostic count)

### Grand Total

- **50,500** differential cases + **35,000** soundness programs (**175,000**
  value-set runs) = **225,500** evaluated case runs (**676,500** implementation
  executions across Go, TypeScript, and Rust)
- **0** differential findings, **0** soundness violations, **0** undecided runs,
  **0** crashes or timeouts.
