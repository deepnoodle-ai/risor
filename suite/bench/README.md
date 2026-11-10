# Benchmark programs

Programs with their data, shared by the benchmark harnesses of the three
implementations. This directory is not normative and is not part of the
conformance suite: no runner reads it, and no rule is stated here.

Each file is a case file in the format of `spec/core/03-host.md` 3.10, using
only the fields `env`, `limits` and `vars` in the header, and `name`, `source`
and `expect` in each case. Every `expect` holds the value, `steps` and
`allocated` of a run, so a harness checks its result before it times anything.
The three implementations agree on all of them; the `outcome` command of each
(see `suite/README.md`) runs these files as it runs any case file.

| File | Programs | Data |
| --- | --- | --- |
| `workflow.json` | The eight workflow expressions of `bench_test.go` (`workflowCases`) | `workflowData`: a pull-request event, 50 test records, 10 findings. Every variable is declared `any`, as there |
| `heavy.json` | The five compute-heavy programs of `bench_test.go` (`heavyCases`) | `heavyData`: 10,000 numbers, 1,000 names, 100 rows of 100 numbers. Declared `any`. The Go benchmark gives the numbers as `int64`; here they are JSON numbers, so a host reads them as floating values |
| `typical.json` | Small typed programs: a field path, a comparison, `??`, a template, string building, a `match`, an order total with `[*]` and `reduce` | A typed pull-request event, a gate, an order of 8 lines |
| `lists-50.json` | Five list programs: `filter` and `map`, `sortBy` with `slice`, a query with `[*]`, `countBy`, and a summary with a template | 50 test records |
| `lists-10k.json` | The same five programs | 10,000 test records |
| `mid.json` | The middle-sized program of `compile_bench_test.go` (`midProgram`), mostly a measure of compile time | An input with 50 items |

Each file sets limits of 100,000,000 steps and units, so no program reaches a
limit.

## The harnesses

Each harness uses its implementation's host API only. For each program it
builds the environment once, checks the outcome against `expect`, and then
measures:

- `compile_ns`: parse, check and compile of the source text;
- `run_ns`: one run of the compiled program on the file's `vars`.

Each is the median of 15 samples taken after a warm-up of 200 ms. A sample is
a batch of calls that takes at least 10 ms, divided by the number of calls.
Each harness prints one JSON line per program, with the fields `impl`,
`program` (file and case name), `compile_ns`, `run_ns`, `steps` and
`allocated`.

| Implementation | Command, from its directory |
| --- | --- |
| Go | `GOWORK=off go test -run xxx -bench BenchmarkSuite -benchmem` |
| TypeScript | `node scripts/bench.ts [FILTER]` |
| Rust | `cargo run --release --example bench [-- FILTER]` |

Steps and units are the same in every implementation, so a comparison of run
times is a comparison of the cost of a step.
