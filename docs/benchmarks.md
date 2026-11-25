# Benchmarks: Risor v3 against expr

Risor v3 is designed to replace `deepnoodle-ai/expr` (decision D1) for
embedding rules, templates and queries in a Go host. This page measures Risor
v3 (Go) against both [`deepnoodle-ai/expr`][dexpr] and
[`expr-lang/expr`][exprlang] on compile time, run time and heap allocations.

All numbers below are **measured** on one machine and are reproducible from
[`experiments/v3/bench-expr/`](../experiments/v3/bench-expr/README.md).

[dexpr]: https://github.com/deepnoodle-ai/expr
[exprlang]: https://github.com/expr-lang/expr

## Summary

- **Compile time (rules and short expressions).** Checked Risor v3 compiles
  small conditions in **2.6–4.8 µs** on maps (**3.0–6.6 µs** on structs) —
  **1.2x–2.2x faster** than `deepnoodle-ai/expr` and **1.4x–2.7x faster** than
  `expr-lang/expr`. A host compiling 1,000 small rules spends roughly **3–7 ms**
  in Risor v3.
- **Compile time (longer pipelines).** On multi-stage queries and nested
  branches, unchecked `deepnoodle-ai/expr` compiles 10–30% faster than checked
  Risor v3, and `expr-lang/expr` compiles the 4-stage `sort_top3_100` pipeline
  faster (12.1 µs vs 15.4 µs on maps) with fewer compile-time bytes. Roughly
  35–45% of Risor v3's compile time on complex expressions is the static type
  checker (`risor_v3` vs `risor_v3_nocheck`).
- **Run time on maps (`map[string]any`).** Risor v3 is faster on every case
  measured: **1.3x–3.2x faster** than `deepnoodle-ai/expr` and **1.8x–2.8x
  faster** than `expr-lang/expr`, with **0 B / 0 allocs** on simple rules and
  **10x–100x fewer allocations** on list queries.
- **Run time on Go structs.** With the F11 typed layer (`risor.TypeOf`), Risor
  v3 is **1.9x–4.1x faster** than `deepnoodle-ai/expr` across every case.
  Against `expr-lang/expr`'s bytecode VM, `expr-lang/expr` edges out Risor v3 on
  a 4-level field read (`63 ns` vs `71 ns`) and ties on optional coalescing
  (`81 ns` vs `82 ns`), while Risor v3 is **1.1x–1.6x faster** on rules,
  branching and every list query, with **3x–7x fewer allocations** on rules
  and filter queries (`projection_100` is about even: 103 vs 113 allocs).
- **Templates.** On pre-compiled host templates over maps, Risor v3 and
  `deepnoodle-ai/expr` run at parity (`478 ns` vs `479 ns` on a 5-field PR
  summary) and ~2.1x faster than `expr-lang/expr` string concatenation. On
  structs, Risor v3 leads both (`489 ns` vs `593 ns` `expr-lang/expr` and
  `860 ns` `deepnoodle-ai/expr`).

## Environment and method

| Item | Value |
| --- | --- |
| Machine | Apple M4 Max (`darwin/arm64`) |
| Go | `go1.26.2` |
| Risor v3 | `github.com/deepnoodle-ai/risor/go/v3` at commit `603f5b4c` |
| `deepnoodle-ai/expr` | `v1.4.1` |
| `expr-lang/expr` | `v1.18.1` |
| Command | `GOWORK=off go test -run '^$' -bench . -benchmem -benchtime 150ms -count 5 ./...` |
| Summary | Median of 5 runs via `experiments/v3/bench-expr/summarize.py` |
| Agreement | `TestAgree` asserts identical normalized results across all engines, on both maps and structs, before benchmarking |

### How each engine is configured

- **Risor v3 (`risor_v3`)** — `Env` is built once with full static types
  (`risor.VarRecord` for maps, `risor.VarType` with `risor.TypeOf[T]()` for
  structs, which enables the F11 typed layer). Compile benchmarks time
  `risor.Parse` plus `env.Compile` (full static checking + closure compile).
  `risor_v3_nocheck` passes `risor.NoCheck` to isolate the checker's cost. Runs
  enforce Risor's default limits (1,000,000 steps, 1,000,000 allocation units,
  depth 200).
- **`deepnoodle-ai/expr` (`deepnoodle_expr`)** — configured as a workflow host
  uses it: `Builtins`, `StringFuncs`, `MathFuncs`, `CollectionFuncs`, a
  `coalesce` helper (it has `?.` but no `??`), `WithEvalBudget(1_000_000)`, and
  `WithStructTags("json")` for struct workloads. `deepnoodle_expr_check` adds
  `Program.Check(typeEnv)` on map compiles for a checked-to-checked comparison.
  `sort_top3_100` is omitted because `deepnoodle-ai/expr` has no `sortBy`.
- **`expr-lang/expr` (`expr_lang`)** — compiled with `expr.Env(schema)` (a
  lightweight prototype map for map workloads and `RootEnv{}` for struct
  workloads, with matching `expr:"..."` struct tags) and evaluated via
  `expr.Run`. `expr-lang/expr` always type-checks against `Env` during
  `Compile`, and does not enforce step or allocation budgets at run time.
  Because it has no built-in host template engine, the template benchmarks
  compare it using string concatenation.

### Workloads

All 11 expression cases and 2 host template cases live in
[`experiments/v3/bench-expr/cases.go`](../experiments/v3/bench-expr/cases.go):

| Case | Category | Risor v3 source |
| --- | --- | --- |
| `field_read` | Rule | `input.event.pull_request.head.sha` |
| `simple_rule` | Rule | `gate.failed > 0` |
| `compound_rule` | Rule | `order.total > 100 && order.country == "US" && user.role in ["admin", "owner"] && !order.flagged` |
| `optional_coalesce` | Rule | `approve?.status ?? report?.status ?? "error"` |
| `branch_arith` | Arithmetic | `if (order.total >= 500) { order.total * 0.85 } else if (order.total >= 100) { order.total * 0.95 } else { order.total }` |
| `string_interp` | Template | `` `${err.step} failed (${err.type}): ${err.message}` `` |
| `any_check_10` | Query | `!verify.some(v => v.confirmed)` (10 items, all false) |
| `filter_map_50` | Query | `fetch.tests50.filter(t => t.status == "fail").map(t => t.name)` (50 items, 5 match) |
| `projection_100` | Query | `fetch.tests100[*].name` (100 items) |
| `filter_sum_100` | Query | `fetch.tests100.filter(t => t.status == "fail").map(t => t.ms).sum()` (100 items, 10 match) |
| `sort_top3_100` | Query | `fetch.tests100.filter(t => t.status == "fail").sortBy(t => t.ms, "desc").slice(0, 3).map(t => t.name)` |
| `greeting` | Host template | `Hello, ${{ user.name }} (${{ user.role }})!` |
| `pr_summary` | Host template | `PR #${{ input.event.pull_request.number }} (${{ input.event.pull_request.title }}) on ${{ input.event.pull_request.head.ref }}: ${{ gate.failed }} failed, ${{ gate.passed }} passed` |

## Compile time

Hosts with thousands of small rules often pay more for compiling expressions
than for running them. Ratios above `1.00x` mean Risor v3 (`risor_v3`, checked)
is faster.

### Compile time — map schema

| Case | `risor_v3` | `risor_v3_nocheck` | `deepnoodle_expr` | `deepnoodle_expr_check` | `expr_lang` | `dexpr` / `v3` | `expr-lang` / `v3` |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| `field_read` | 3.05 µs | 2.31 µs | 5.38 µs | 5.50 µs | 6.41 µs | 1.76x | 2.10x |
| `simple_rule` | 2.55 µs | 1.85 µs | 5.53 µs | 5.53 µs | 5.75 µs | 2.16x | 2.25x |
| `compound_rule` | 9.02 µs | 5.05 µs | 8.37 µs | 11.36 µs | 10.76 µs | 0.93x | 1.19x |
| `optional_coalesce` | 4.48 µs | 2.72 µs | 9.41 µs | 9.86 µs | 6.98 µs | 2.10x | 1.56x |
| `branch_arith` | 9.79 µs | 6.64 µs | 8.74 µs | 9.19 µs | 10.61 µs | 0.89x | 1.08x |
| `string_interp` | 4.14 µs | 3.24 µs | 6.29 µs | 6.64 µs | 8.39 µs | 1.52x | 2.03x |
| `any_check_10` | 4.82 µs | 3.13 µs | 5.91 µs | 6.13 µs | 6.69 µs | 1.23x | 1.39x |
| `filter_map_50` | 8.52 µs | 5.30 µs | 7.14 µs | 7.79 µs | 9.45 µs | 0.84x | 1.11x |
| `projection_100` | 3.05 µs | 2.24 µs | 6.59 µs | 6.46 µs | 6.84 µs | 2.16x | 2.25x |
| `filter_sum_100` | 9.50 µs | 5.50 µs | 7.99 µs | 8.25 µs | 9.65 µs | 0.84x | 1.02x |
| `sort_top3_100` | 15.39 µs | 8.70 µs | — | — | 12.05 µs | — | 0.78x |

Compile allocations (bytes / allocs per compile, map schema):

| Case | `risor_v3` | `risor_v3_nocheck` | `deepnoodle_expr` | `deepnoodle_expr_check` | `expr_lang` |
| --- | ---: | ---: | ---: | ---: | ---: |
| `field_read` | 9,224 B / 86 | 8,232 B / 66 | 14,496 B / 86 | 14,816 B / 89 | 12,912 B / 100 |
| `simple_rule` | 6,800 B / 84 | 5,104 B / 60 | 14,520 B / 77 | 15,048 B / 83 | 11,744 B / 81 |
| `compound_rule` | 23,456 B / 229 | 15,392 B / 146 | 18,176 B / 157 | 22,743 B / 232 | 16,288 B / 159 |
| `optional_coalesce` | 11,904 B / 124 | 8,424 B / 82 | 22,382 B / 171 | 23,448 B / 185 | 13,008 B / 106 |
| `branch_arith` | 29,936 B / 208 | 23,768 B / 151 | 18,504 B / 163 | 19,896 B / 181 | 17,112 B / 149 |
| `string_interp` | 10,440 B / 115 | 9,224 B / 90 | 15,336 B / 104 | 15,880 B / 111 | 14,480 B / 125 |
| `any_check_10` | 11,576 B / 127 | 8,832 B / 92 | 15,264 B / 97 | 15,872 B / 103 | 12,840 B / 96 |
| `filter_map_50` | 21,472 B / 202 | 15,336 B / 137 | 16,656 B / 127 | 17,584 B / 138 | 15,408 B / 138 |
| `projection_100` | 7,024 B / 88 | 5,824 B / 65 | 15,360 B / 102 | 15,872 B / 107 | 12,968 B / 102 |
| `filter_sum_100` | 23,368 B / 216 | 15,872 B / 142 | 17,016 B / 135 | 17,952 B / 147 | 16,808 B / 138 |
| `sort_top3_100` | 38,344 B / 296 | 26,456 B / 191 | — | — | 18,814 B / 181 |

### Compile time — Go struct schema

| Case | `risor_v3` | `risor_v3_nocheck` | `deepnoodle_expr` | `expr_lang` | `dexpr` / `v3` | `expr-lang` / `v3` |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| `field_read` | 3.55 µs | 2.38 µs | 5.50 µs | 7.98 µs | 1.55x | 2.24x |
| `simple_rule` | 2.98 µs | 1.89 µs | 5.53 µs | 6.53 µs | 1.86x | 2.19x |
| `compound_rule` | 11.53 µs | 5.66 µs | 8.62 µs | 12.31 µs | 0.75x | 1.07x |
| `optional_coalesce` | 5.01 µs | 3.18 µs | 11.05 µs | 11.23 µs | 2.21x | 2.24x |
| `branch_arith` | 14.28 µs | 9.13 µs | 11.18 µs | 15.24 µs | 0.78x | 1.07x |
| `string_interp` | 6.09 µs | 4.21 µs | 8.52 µs | 12.20 µs | 1.40x | 2.00x |
| `any_check_10` | 6.61 µs | 4.08 µs | 7.76 µs | 8.42 µs | 1.17x | 1.27x |
| `filter_map_50` | 10.11 µs | 5.00 µs | 7.11 µs | 11.01 µs | 0.70x | 1.09x |
| `projection_100` | 3.09 µs | 2.06 µs | 6.09 µs | 8.36 µs | 1.97x | 2.71x |
| `filter_sum_100` | 10.38 µs | 5.71 µs | 7.66 µs | 11.33 µs | 0.74x | 1.09x |
| `sort_top3_100` | 16.13 µs | 8.30 µs | — | 14.65 µs | — | 0.91x |

Compile allocations (bytes / allocs per compile, struct schema):

| Case | `risor_v3` | `risor_v3_nocheck` | `deepnoodle_expr` | `expr_lang` |
| --- | ---: | ---: | ---: | ---: |
| `field_read` | 9,480 B / 91 | 8,232 B / 66 | 14,592 B / 88 | 14,530 B / 116 |
| `simple_rule` | 6,960 B / 87 | 5,104 B / 60 | 14,616 B / 79 | 12,118 B / 82 |
| `compound_rule` | 24,608 B / 246 | 15,392 B / 146 | 18,272 B / 159 | 18,454 B / 206 |
| `optional_coalesce` | 12,224 B / 130 | 8,424 B / 82 | 22,478 B / 173 | 14,368 B / 132 |
| `branch_arith` | 30,736 B / 223 | 23,768 B / 151 | 18,600 B / 165 | 16,980 B / 192 |
| `string_interp` | 10,920 B / 124 | 9,224 B / 90 | 15,432 B / 106 | 15,466 B / 156 |
| `any_check_10` | 11,744 B / 131 | 8,832 B / 92 | 15,360 B / 99 | 14,161 B / 112 |
| `filter_map_50` | 22,336 B / 216 | 15,336 B / 137 | 16,752 B / 129 | 17,373 B / 168 |
| `projection_100` | 7,304 B / 94 | 5,824 B / 65 | 15,456 B / 104 | 14,817 B / 125 |
| `filter_sum_100` | 24,232 B / 230 | 15,872 B / 142 | 17,112 B / 137 | 17,420 B / 168 |
| `sort_top3_100` | 39,208 B / 310 | 26,456 B / 191 | — | 21,137 B / 215 |

## Run time

Each program is compiled once and evaluated repeatedly against the same input
data. Ratios above `1.00x` mean Risor v3 is faster.

### Run time — maps (`map[string]any`)

| Case | `risor_v3` | `deepnoodle_expr` | `expr_lang` | `dexpr` / `v3` | `expr-lang` / `v3` |
| --- | ---: | ---: | ---: | ---: | ---: |
| `field_read` | 70 ns | 91 ns | 170 ns | 1.30x | 2.44x |
| `simple_rule` | 44 ns | 109 ns | 79 ns | 2.45x | 1.78x |
| `compound_rule` | 132 ns | 292 ns | 293 ns | 2.21x | 2.22x |
| `optional_coalesce` | 56 ns | 161 ns | 106 ns | 2.86x | 1.90x |
| `branch_arith` | 86 ns | 206 ns | 229 ns | 2.39x | 2.65x |
| `string_interp` | 168 ns | 227 ns | 316 ns | 1.35x | 1.88x |
| `any_check_10` | 237 ns | 593 ns | 454 ns | 2.50x | 1.91x |
| `filter_map_50` | 1.48 µs | 3.58 µs | 2.86 µs | 2.42x | 1.93x |
| `projection_100` | 1.63 µs | 5.25 µs | 4.51 µs | 3.22x | 2.76x |
| `filter_sum_100` | 2.92 µs | 7.19 µs | 5.82 µs | 2.46x | 1.99x |
| `sort_top3_100` | 3.45 µs | — | 6.36 µs | — | 1.84x |

Run allocations (bytes / allocs per run, maps):

| Case | `risor_v3` | `deepnoodle_expr` | `expr_lang` |
| --- | ---: | ---: | ---: |
| `field_read` | 0 B / 0 | 120 B / 2 | 96 B / 5 |
| `simple_rule` | 0 B / 0 | 120 B / 2 | 48 B / 2 |
| `compound_rule` | 0 B / 0 | 208 B / 5 | 96 B / 5 |
| `optional_coalesce` | 0 B / 0 | 168 B / 3 | 48 B / 2 |
| `branch_arith` | 8 B / 1 | 128 B / 3 | 88 B / 5 |
| `string_interp` | 136 B / 5 | 296 B / 10 | 320 B / 13 |
| `any_check_10` | 0 B / 0 | 504 B / 23 | 360 B / 13 |
| `filter_map_50` | 256 B / 4 | 3,032 B / 118 | 1,392 B / 63 |
| `projection_100` | 1,817 B / 2 | 5,200 B / 205 | 8,048 B / 112 |
| `filter_sum_100` | 680 B / 7 | 5,792 B / 230 | 3,008 B / 130 |
| `sort_top3_100` | 1,217 B / 13 | — | 4,232 B / 133 |

### Run time — Go structs

| Case | `risor_v3` | `deepnoodle_expr` | `expr_lang` | `dexpr` / `v3` | `expr-lang` / `v3` |
| --- | ---: | ---: | ---: | ---: | ---: |
| `field_read` | 71 ns | 190 ns | 63 ns | 2.67x | 0.89x |
| `simple_rule` | 46 ns | 115 ns | 63 ns | 2.49x | 1.37x |
| `compound_rule` | 142 ns | 463 ns | 154 ns | 3.26x | 1.09x |
| `optional_coalesce` | 82 ns | 282 ns | 81 ns | 3.44x | 0.99x |
| `branch_arith` | 89 ns | 364 ns | 114 ns | 4.10x | 1.28x |
| `string_interp` | 211 ns | 391 ns | 225 ns | 1.85x | 1.06x |
| `any_check_10` | 256 ns | 675 ns | 403 ns | 2.63x | 1.57x |
| `filter_map_50` | 1.68 µs | 3.61 µs | 2.32 µs | 2.15x | 1.38x |
| `projection_100` | 2.85 µs | 5.38 µs | 4.48 µs | 1.88x | 1.57x |
| `filter_sum_100` | 3.33 µs | 7.03 µs | 4.97 µs | 2.11x | 1.49x |
| `sort_top3_100` | 3.85 µs | — | 5.36 µs | — | 1.39x |

Run allocations (bytes / allocs per run, structs):

| Case | `risor_v3` | `deepnoodle_expr` | `expr_lang` |
| --- | ---: | ---: | ---: |
| `field_read` | 16 B / 1 | 296 B / 5 | 144 B / 3 |
| `simple_rule` | 0 B / 0 | 224 B / 4 | 136 B / 3 |
| `compound_rule` | 16 B / 1 | 352 B / 10 | 176 B / 6 |
| `optional_coalesce` | 16 B / 1 | 280 B / 5 | 144 B / 3 |
| `branch_arith` | 8 B / 1 | 248 B / 7 | 160 B / 6 |
| `string_interp` | 184 B / 8 | 440 B / 14 | 352 B / 13 |
| `any_check_10` | 0 B / 0 | 520 B / 14 | 536 B / 14 |
| `filter_map_50` | 520 B / 10 | 3,792 B / 60 | 3,256 B / 64 |
| `projection_100` | 3,442 B / 103 | 6,920 B / 107 | 11,352 B / 113 |
| `filter_sum_100` | 1,185 B / 18 | 7,192 B / 112 | 6,632 B / 131 |
| `sort_top3_100` | 1,721 B / 24 | — | 7,808 B / 131 |

## Host templates

Risor v3 (`risor.ParseTemplate`, `${{ ... }}`) and `deepnoodle-ai/expr`
(`expr.NewTemplate`, `${...}`) both provide first-class template compilers.
`expr-lang/expr` is measured on the equivalent string concatenation expression.

### Template compile time (struct schema)

| Case | `risor_v3` | `risor_v3_nocheck` | `deepnoodle_expr` | `expr_lang` | `dexpr` / `v3` | `expr-lang` / `v3` |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| `greeting` | 4.57 µs | 3.27 µs | 6.71 µs | 8.29 µs | 1.47x | 1.81x |
| `pr_summary` | 13.22 µs | 8.70 µs | 11.44 µs | 20.02 µs | 0.87x | 1.51x |

Compile allocations (bytes / allocs):

| Case | `risor_v3` | `risor_v3_nocheck` | `deepnoodle_expr` | `expr_lang` |
| --- | ---: | ---: | ---: | ---: |
| `greeting` | 10,056 B / 121 | 8,728 B / 94 | 17,224 B / 122 | 14,521 B / 137 |
| `pr_summary` | 31,040 B / 258 | 26,632 B / 192 | 23,976 B / 254 | 26,015 B / 356 |

### Template run time

| Case | `risor_v3` | `deepnoodle_expr` | `expr_lang` | `dexpr` / `v3` | `expr-lang` / `v3` |
| --- | ---: | ---: | ---: | ---: | ---: |
| `greeting_maps` | 125 ns | 134 ns | 256 ns | 1.07x | 2.05x |
| `greeting_structs` | 155 ns | 245 ns | 200 ns | 1.58x | 1.29x |
| `pr_summary_maps` | 478 ns | 479 ns | 1.00 µs | 1.00x | 2.10x |
| `pr_summary_structs` | 489 ns | 860 ns | 593 ns | 1.76x | 1.21x |

Run allocations (bytes / allocs):

| Case | `risor_v3` | `deepnoodle_expr` | `expr_lang` |
| --- | ---: | ---: | ---: |
| `greeting_maps` | 72 B / 4 | 288 B / 5 | 272 B / 12 |
| `greeting_structs` | 104 B / 6 | 416 B / 8 | 304 B / 12 |
| `pr_summary_maps` | 272 B / 8 | 784 B / 14 | 953 B / 39 |
| `pr_summary_structs` | 304 B / 10 | 1,128 B / 23 | 848 B / 32 |

## Analysis: where each engine wins

### Where Risor v3 is faster

- **Short-rule compilation.** Risor v3's hand-written Pratt parser avoids Go's
  `go/parser` setup cost (`deepnoodle-ai/expr` starts around 14.5 KB per
  compile even for `gate.failed > 0`), and `risor.NewEnv` resolves variable
  schemas and Go struct metadata once instead of per compile (`expr-lang/expr`
  inspects `expr.Env(...)` on each `Compile`).
- **Map evaluation and low allocations.** Risor v3 compiles directly to Go
  closures, reads `map[string]any` values in place, and pools per-program run
  state. Simple rules allocate `0 B / 0 allocs` per run, and queries avoid the
  per-element boxing overhead that `deepnoodle-ai/expr` and `expr-lang/expr`
  pay inside `filter` and `map`.
- **Projection (`[*]`).** `fetch.tests100[*].name` avoids lambda call overhead
  and allocates only 2 objects (`1,817 B`) on maps, running 2.8x–3.2x faster
  than `map(...)` in either `expr` engine.
- **Queries over Go structs.** The F11 typed layer plans struct field index
  paths at compile time, keeping struct list queries 1.4x–1.6x faster than
  `expr-lang/expr` and 1.9x–2.2x faster than `deepnoodle-ai/expr`.

### Where `expr` is faster

- **Compiling longer expressions.** Risor v3's static checker tracks record
  shapes, unions, nullability and dynamic sites, which adds roughly 35–45% to
  compile time and allocates more compile-time memory on longer expressions
  (`branch_arith`, `filter_map_50`, `filter_sum_100`, `sort_top3_100`,
  `pr_summary`). Unchecked `deepnoodle-ai/expr` compiles those 10–30% faster;
  `expr-lang/expr` compiles `sort_top3_100` ~10–22% faster with roughly half
  the compile-time bytes. When type checking is disabled (`risor.NoCheck`),
  Risor v3 compiles faster than both on every case.
- **Pure nested struct field reads.** On `input.event.pull_request.head.sha`
  over Go structs, `expr-lang/expr` (`63 ns`) beats Risor v3 (`71 ns`) because
  its VM emits specialized field-index opcodes without step-budget accounting
  and without boxing the intermediate result. Risor v3 pays for deterministic
  step accounting on every operation and 1 allocation (`16 B`) to box a string
  field read from a Go struct (which also shows up as 103 allocs on
  `projection_100` over structs vs 2 allocs on maps).
- **Multi-placeholder map templates at run time.** On `pr_summary_maps`,
  `deepnoodle-ai/expr`'s `NewTemplate` matches Risor v3 (`479 ns` vs `478 ns`).

## Limits of this evidence

- **One machine and Go only.** All numbers are from one Apple M4 Max running
  Go 1.26.2. Risor v3's TypeScript and Rust implementations are not measured
  here.
- **Different safety guarantees.** Risor v3 enforces exact step, allocation
  and depth limits on every run. `deepnoodle-ai/expr` enforces a step budget
  (no allocation budget). `expr-lang/expr` enforces neither at run time.
- **Run-to-run spread.** On the captured run, max `(max-min)/median` spread
  was 7.2% on `RunStructs`, 7.5% on `TemplateRun` and 19.4% on
  `TemplateCompile`. `CompileMaps` (44%), `RunMaps` (84%, driven by one sample
  on `simple_rule`) and `CompileStructs` (138%, driven by one sample on
  `branch_arith`) saw transient background scheduling jitter; reporting the
  median of 5 runs mitigates single-sample spikes, and allocation counts
  (`B/op`, `allocs/op`) are deterministic.

## Reproducing

```bash
cd experiments/v3/bench-expr
GOWORK=off go test -run '^TestAgree$' -v ./...
GOWORK=off go test -run '^$' -bench . -benchmem -benchtime 150ms -count 5 ./... | tee results.txt
python3 summarize.py results.txt
```
