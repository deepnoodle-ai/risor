# Experiment: Risor without the VM

Risor compiles to bytecode and runs it on a virtual machine. The only reason for
that design is speed. This experiment asks whether it delivers speed for the way
Risor is mostly used: short expressions evaluated against data that a Go host
supplies.

**Result: it does not.** A closure compiler over Risor's existing AST, reading
host values in place, is faster than the VM on every case measured, including
compute-heavy programs.

This is a spike. Nothing here is used by Risor, and nothing here is an API.

## What is here

- `spike/` — the evaluator, about 1,200 lines. It parses with `pkg/parser`, then
  compiles each AST node to a Go closure. There is no bytecode and no object
  model. Values are the host's Go values: `nil`, `bool`, `int64`, `float64`,
  `string`, `[]any` and `map[string]any`.
- `bench/` — agreement tests and benchmarks against the Risor VM and against
  [expr](https://github.com/deepnoodle-ai/expr).
- `results/` — the raw benchmark output and the script that summarizes it.
- `design/` — the design of a Risor v3 language that followed from this result.
  Start with `design/00-overview.md`. It is a proposal: no specification and
  no implementation exists yet.
- `lab/` — small Go measurements that the design documents cite, such as the
  cost of counting steps and allocations, and of value representations.

The semantics differ from Risor v2 on purpose:

- A dot always reads a field. Methods exist only in call position, so `m.keys`
  on a map returns the value under `"keys"`.
- Host values are read in place. Nothing is converted on the way in or out.
- The step budget counts every operator, field read, call and iteration of a
  list method, so it bounds `filter` and `map` as well as user code.

## Running it

The module is not in `go.work`, because expr needs a newer Go than the workspace
declares. Run it with the workspace off:

```bash
cd experiments/v3
GOWORK=off go test ./bench -run 'Test' -v
GOWORK=off go test ./bench -run xxx -bench 'SpikeWorkflow|SpikeHeavy' \
  -benchmem -benchtime 400ms -count 6 | tee results/run.txt
python3 results/tables.py results/run.txt
```

## Results

Apple M4 Max, Go 1.26.2, median of 6 runs. Every implementation returns the same
value for every case (`TestSpikeAgree`).

The columns:

- **expr** — expr v1.4.1, where the case can be written in it.
- **risor** — `risor.Run` with Go values, as a host calls it today.
- **VM reused** — one VM kept between runs, values converted to Risor objects
  once, globals rebound on each run, result converted back to Go.
- **VM floor** — the same, but globals are never rebound and the result stays a
  Risor object. A host cannot use the VM this way, because its data changes
  between evaluations. It shows what running the bytecode costs with everything
  else removed.
- **spike** — this experiment.

### Expressions over a small scope

| Case | expr | risor | VM reused | VM floor | spike |
| --- | ---: | ---: | ---: | ---: | ---: |
| `input.event.pull_request.head.sha` | 79 ns | 35.1 µs | 4.7 µs | 1.3 µs | 76 ns |
| `gate.failed > 0` | 56 ns | 35.1 µs | 4.6 µs | 1.2 µs | 62 ns |
| `input.role in ["admin", "owner"]` | 130 ns | 35.3 µs | 4.7 µs | 1.3 µs | 65 ns |
| `coalesce(report?.status, approve?.status, "error")` | 123 ns | 35.3 µs | 5.2 µs | 1.5 µs | 78 ns |
| `!any(verify.map(v => v.confirmed))` | 566 ns | 36.1 µs | 5.6 µs | 2.2 µs | 314 ns |
| filter and map over 50 maps | 3.3 µs | 40.2 µs | 9.7 µs | 6.1 µs | 1.2 µs |
| `if (…) { … } else { … }` | 82 ns | 35.0 µs | 4.7 µs | 1.3 µs | 63 ns |
| string concatenation | 126 ns | 35.1 µs | 4.7 µs | 1.4 µs | 113 ns |

### Compute-heavy programs

| Case | expr | risor | VM reused | VM floor | spike | VM floor ÷ spike |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| filter and map over 10,000 ints | 418 µs | 1.61 ms | 954 µs | 926 µs | 238 µs | 3.9x |
| filter and map over 1,000 strings | 107 µs | 860 µs | 213 µs | 196 µs | 69 µs | 2.8x |
| nested closures, 100 × 100 | 1.04 ms | 1.95 ms | 1.32 ms | 1.26 ms | 359 µs | 3.5x |
| `reduce` over 10,000 ints | – | 1.48 ms | 857 µs | 807 µs | 236 µs | 3.4x |
| recursive `fib(22)` | – | 5.91 ms | 5.26 ms | 5.24 ms | 1.21 ms | 4.3x |
| four `let` statements and an `if` | – | 634 µs | 5.8 µs | 2.0 µs | 116 ns | 17x |

### Allocations

| Case | expr | VM floor | spike |
| --- | ---: | ---: | ---: |
| field read | 120 B, 2 allocs | 2,328 B, 12 allocs | 384 B, 1 alloc |
| filter and map over 50 maps | 3,032 B, 118 | 5,888 B, 95 | 952 B, 12 |
| filter and map over 10,000 ints | 274 KB, 6,590 | 486 KB, 19,964 | 269 KB, 6,604 |
| recursive `fib(22)` | – | 923 KB, 57,419 | 5 KB, 118 |

### Compile time

Source text to a runnable program: spike 3.4–7.6 µs, Risor 13–22 µs, expr 5–9 µs
(`results/compile.txt`).

## Where the VM's time goes

- **Per run.** A reused VM spends about 4.7 µs on each run before it executes an
  instruction. A CPU profile shows most of it is rebuilding the globals map on
  every `RunCode`. With that removed, the floor is still 1.3 µs and 12
  allocations.
- **`fib(22)`.** No single cause. The time is split between the dispatch loop, a
  slice allocated on every call and the collection it causes, observer dispatch,
  and interface calls for integer comparison and arithmetic.

## Limits of this evidence

- **Two things are measured together.** The comparison is bytecode plus Risor's
  object model against closures plus host values. A bytecode VM over host values
  was not built, so this does not say how much of the gap is dispatch and how
  much is the object model.
- **The spike does less.** It has no observer, no per-operation size limits, no
  destructuring, spread, template strings, `match` or `try`. It has no modules
  and only four builtins.
- **Only JSON-shaped values.** Reading Go structs through reflection and calling
  typed Go functions were not measured, and will cost more than a map read.
- **One machine.** All numbers are from one Apple M4 Max.
