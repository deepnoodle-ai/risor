# Risor v3 benchmarks against expr

Reproducible Go benchmarks comparing Risor v3 (`github.com/deepnoodle-ai/risor/go/v3`)
against `github.com/deepnoodle-ai/expr` and `github.com/expr-lang/expr` on
representative workflow rules, arithmetic/branching expressions, templates, and
queries over lists of records.

Published results and analysis live in [`docs/benchmarks.md`](../../../docs/benchmarks.md).

## What is measured

- **Agreement (`TestAgree`)** — asserts that every engine produces the same
  result on every expression and template case, for both `map[string]any` and Go
  struct inputs, and that static type checking passes.
- **`BenchmarkCompileMaps`** — source text to a runnable program against a
  map-backed schema (`risor_v3` checked, `risor_v3_nocheck`, `deepnoodle_expr`,
  `deepnoodle_expr_check`, and `expr_lang`).
- **`BenchmarkCompileStructs`** — source text to a runnable program against Go
  structs (`risor_v3` with `TypeOf` + F11 typed layer, `risor_v3_nocheck`,
  `deepnoodle_expr` with `json` tags, and `expr_lang` with struct `Env`).
- **`BenchmarkRunMaps`** — evaluating a pre-compiled program over JSON-shaped
  `map[string]any` values.
- **`BenchmarkRunStructs`** — evaluating a pre-compiled program over Go structs.
- **`BenchmarkTemplateCompile` / `BenchmarkTemplateRun`** — host templates
  (`risor.ParseTemplate` `${{ ... }}` vs `deepnoodle-ai/expr.NewTemplate`
  `${...}` vs `expr-lang/expr` string concatenation).

## Reproducing

Run from this directory with `GOWORK=off` (this module is intentionally separate
from `go/` so `go/go.mod` carries no benchmark dependencies):

```bash
cd experiments/v3/bench-expr
GOWORK=off go test -run '^TestAgree$' -v ./...
GOWORK=off go test -run '^$' -bench . -benchmem -benchtime 150ms -count 5 ./... | tee results.txt
python3 summarize.py results.txt
```
