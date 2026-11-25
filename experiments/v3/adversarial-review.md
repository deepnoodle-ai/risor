# Adversarial Review of Limits and the Host Boundary

This report records the pre-3.0 adversarial review of Risor v3's safety
promises across all three implementations: Go (`go/`), TypeScript (`ts/`) and
Rust (`rust/`). It addresses issue #704.

The review tests whether an untrusted program, hostile host data, or malformed
editor text can exhaust CPU or memory past configured limits, panic a host
process, leak host internals, or bypass accounting across nested runs.

## Environment and tools

Every claim marked `ran` below was measured on macOS (`darwin/arm64`) with:

- Go `go1.26.2 darwin/arm64`
- Node.js `v22.22.3` (`npm 10.9.8`)
- Rust `rustc 1.99.0 (b940084d7 2026-09-28)`,
  `cargo 1.99.0 (5f94df478 2026-08-27)`

Regression tests added by this review live in:

- `go/adversarial_test.go` (`go test -count=1 -run TestAdversarial .`)
- `ts/test/adversarial.test.ts` (`node --test test/adversarial.test.ts`)
- `rust/tests/adversarial.rs` (`cargo test --test adversarial`)

## Summary by area

| Area | Attack vectors tested | Verdict |
| --- | --- | --- |
| 1. Limits (steps, allocation, depth, deadline, source size) | Exponential string/list/map blowups, `flatMap`/`json.parse`/`split`/`replaceAll`/`slice`/`padStart`/`Rendering` OOM-before-charge, `time.duration` and `time.parse` quadratic blowups, parser nesting (>100), type depth (>64), checker DAG type size (>100,000), long flat `as` chains (130,000), recursion & `walk` depth, 25 ms deadlines, cancellation, source size (>1,048,576) | **Fixed** (8 bugs across Go, TS, Rust; remaining checks **Pass**) |
| 2. Opaque values and host functions | Uncomparable Go opaque structs (`SameOpaque`), cyclic Go pointers (`KindOf`/`Read`), Rust `fail()` DAG expansion inside `Program::run`, TS prototype keys (`__proto__`, `constructor`, `toString`, `hasOwnProperty`), TS `call.charge`/`allocate` non-integer and `Infinity`, host panics, nested cause depth (8), host data immutability | **Fixed** (4 bugs across Go, TS, Rust; remaining checks **Pass**) |
| 3. Nested runs with `remaining()` (#689) | Budget amplification across loops and `try/catch`, zero remaining budget (`0` steps / `0` units), failed child accounting, post-return handle use, non-integer charges breaking `remaining()`, depth and deadline non-sharing (`03-host.md` 3.2.5) | **Fixed** (TS `charge`/`allocate` rounding; design bounds **Ruled** / **Pass**) |
| 4. Conformance of host data (F4, F8) | Cyclic host maps/objects against recursive types, huge lists/maps with step limits and error caps, `NaN`/`Inf`/`-Inf`/`MaxInt64`/`BigInt`/`-0`, tagged union bad host number in Go `ConformAll`, invalid UTF-8 and lone surrogates, DAG materialization outside a run | **Fixed** (Go `ConformAll` tag error and double key sort; outside-run DAGs **Ruled**) |
| 5. Editor tools on hostile text | Repeated unclosed `/*\n`, `` `${\n ``, `"""\n`, thousands of unclosed `(`, `{`, `[`, `if (1) {`, `const a = ; `, `a + `, null bytes, invalid UTF-8, >1 MiB inputs on `format`, `complete`, `hover`, `tokens` | **Fixed** (TS `tokens()` O(N^2)/O(N^3) hang; remaining tools **Pass**) |

## Findings and fixes

### Area 1: Limits, library charges, and OOM-before-charge

1. **Rust `list.flatMap` allocated output before charging (Fixed).**
   - *Location:* `rust/src/eval/lib.rs` (`Each::FlatMap` in `each_result` and
     `each_done`).
   - *Before (`ran`, `cargo test`):* Each callback return list was flattened
     eagerly into `g.out` before `self.charge(k)` and `self.alloc(1 + k)` ran,
     violating `spec/core/02-evaluation.md` 6.2 item 3. On `xs.flatMap(_ => xs)`
     with a 2,000-element host list and `allocation: 200`, Rust allocated
     4,000,000 `Value`s (147.8 ms) before returning `allocation-exceeded`. A
     10,000-element host list would allocate ~2.4 GB before failing.
   - *Fix:* Store each returned list `Value` in `g.out`, accumulate the total
     element count `k` with checked arithmetic (failing with
     `allocation-exceeded` on overflow), charge `k` steps and checked `1 + k`
     units, and flatten only if the budget covers `k`.
   - *Verification (`ran`, `cargo test --test adversarial`):*
     `limits_exponential_and_oom_before_charge` passes in <15 ms.

2. **TS and Rust `time.duration` had O(N^2) CPU and memory blowup (Fixed).**
   - *Location:* `ts/src/library/groups.ts` (`time.duration`),
     `rust/src/eval/groups.rs` (`parse_duration`).
   - *Before (`ran`, Node v22.22.3 & `cargo test`):* Input
     `"0." + "0".repeat(10000) + "1s" + "1s".repeat(10000)` (~30 KB, ~470 steps)
     took **1,633.5 ms** in TypeScript (repeated `BigInt` scaling
     `10n ** BigInt(k - t.k)` per term) and **872.0 ms** in Rust (repeated
     `format!("{ip}{fp}{}", "0".repeat(f - fp.len()))` per term). Go's column
     addition handled the same input in **0.3 ms**.
   - *Fix:* Replaced both TS and Rust `time.duration` summation loops with a
     single-pass, fixed-width O(N) column accumulator (1,120 fractional columns
     plus 320 integer columns, with a fixed-window exact midpoint tiebreaker).
     This keeps scratch memory to O(1) for the fixed columns and windows, plus
     O(k) term records on the exact-comparison path (where `k` is the number of
     terms with nonzero truncated tails), while preserving exact IEEE 754
     round-to-nearest-even and normative `units = 0`.
   - *After (`ran`):* 10,000 terms dropped from 1,633.5 ms to **5.2 ms** in TS
     (50,000 terms: 23.6 ms) and from 872.0 ms to **<5 ms** in Rust. Verified by
     `time.duration and time.parse stay linear on pathological inputs` (TS) and
     `time_duration_is_linear_on_many_terms_with_long_fraction` (Rust).

3. **TS `time.parse` built unbounded `BigInt` powers on long fractions
   (Fixed).**
   - *Location:* `ts/src/library/groups.ts` (`wholePlusFraction`).
   - *Before (`ran`, Node v22.22.3):* Computed `BigInt(w) * 10n ** BigInt(k)` on
     50,000 fractional digits (12.7 ms and proportional BigInt memory).
   - *Fix:* Format the decimal text directly in O(k) time (using linear borrow
     subtraction when `w < 0`) and pass it to `parseDecimal`, which already
     truncates past 800 significant digits with a sticky bit.
   - *After (`ran`):* 50,000 fractional digits ran in **1.2 ms**.

4. **TS and Rust `json.parse` built the full value tree before charging
   (Fixed).**
   - *Location:* `ts/src/library/groups.ts` (`scanJson`),
     `rust/src/eval/groups.rs` (`json_parse`, `JsonScan`).
   - *Before (`ran`):* With `steps: 10_000_000, allocation: 5`, parsing a
     100,000-element JSON array in TS took **81.1 ms** and allocated every array
     and element before `alloc(units)` rejected it. Rust additionally copied the
     input string (`to_string()`) before `charge(8 * size(s))` and collected a
     `Vec<char>` (4x input size) before building 50,000 `Value`s (**39.3 ms**).
   - *Fix:* Split both scanners into a measure-only pass (validating grammar,
     finding bad numbers, depth, value count and allocation units without
     building values) followed by depth/step/allocation checks, then a build
     pass. Rust now scans `&str`/`&[u8]` directly and charges `size(s)` before
     any copy in `json.parse`, `time.parse`, `time.duration` and
     `base64.decode`.
   - *After (`ran`):* TS 100k-element array rejected with `allocation-exceeded`
     in **24.1 ms** with zero JS values allocated; Rust passes all JSON suite
     cases and `limits_exponential_and_oom_before_charge`.

5. **TS `Rendering` buffered unbounded output before allocation check (Fixed).**
   - *Location:* `ts/src/eval/ops.ts` (`Rendering`, `renderedText`, `jsonForm`),
     `ts/src/eval/compile.ts` (`template`), `ts/src/library/core.ts` (`string`),
     `ts/src/library/groups.ts` (`json.stringify`).
   - *Before (`ran`):* Unlike Go (`renderKeep = 64 KiB`) and Rust (`64 KiB`), TS
     concatenated all rendered output into `Rendering.last` before
     `r.alloc(1 + sz)`. Rendering 20,000 strings under `allocation: 5` took
     **21.9 ms** and built the full string before failing.
   - *Fix:* Added a 64 KiB (`RENDER_KEEP = 65536`) cap to TS `Rendering`. Past
     64 KiB, pass 1 drops buffered chunks and measures length only; if the
     allocation budget accepts `1 + sz`, pass 2 builds the string with
     `paid = true` (charging nothing).
   - *After (`ran`):* Same 20,000-string probe rejected in **5.2 ms** without
     allocating the output buffer.

6. **TS string methods and Go/Rust list pre-allocations before charge (Fixed).**
   - *Locations:* `ts/src/library/core.ts` (`split`, `replaceAll`, `slice`,
     `padStart`/`padEnd`), `ts/src/eval/ops.ts` (`order`),
     `go/internal/eval/lib.go` (`list.map`, `list.sum`, `list.sortBy`,
     `list.toMap`, `list.flat`, `list.join`, `orderedElems`, `pad`),
     `rust/src/eval/lib.rs` (`padStart`/`padEnd`, `map_values`, `list_map`,
     `toMap`, `join`, `ordered_scan`).
   - *Before (`ran`):* TS `string.split` built all 250,000 parts (**40.2 ms**)
     before charging `alloc`; TS `slice`, `replaceAll`, `padStart`/`padEnd`, and
     string `order` allocated intermediate arrays/substrings before charging. Go
     and Rust pre-allocated `make([]any, n)` / `Vec::with_capacity(n)` for huge
     host lists before visiting or charging elements, and `pad` in Rust
     collected `Vec<char>` of `fill` before charging (while Go scanned all of
     `fill` even when `m` was small).
   - *Fix:* TS string methods now charge input size and count results without
     allocating before `alloc`; TS `order` uses `div64(lcpWeight(...))`; Go and
     Rust cap upfront container capacities at `min(n, 256)` (preserving exact
     failure precedence and step/unit counts) and scan at most `m` code points
     of `fill` in `pad` with zero pre-charge allocation.
   - *After (`ran`):* TS `string.split` on 500 KB under `allocation: 5` dropped
     from 40.2 ms to **6.5 ms**.

7. **Limits verified without bugs (Pass).**
   - *Source-size limit (`1,048,576` code points):* `ran` across `parse`,
     `parseTemplate`, `format`, `formatTemplate`, `complete`, and `hover` in Go,
     TS, and Rust. All rejected `1,048,577`-code-point inputs cleanly in <75 ms.
   - *Parser nesting (100), type depth (64), checker type size (100,000), and
     long flat chains:* `ran` in all three implementations. 500 unclosed `(`
     rejected with `nesting-exceeded`; 65 `[]` suffixes rejected with
     `bad-type`; 25-level DAG record type rejected with `check-limit` in <7 ms;
     130,000-link `as int` chain (~910 KB) compiled, formatted, and ran without
     stack overflow in Go (213 ms run / 536 ms format), TS (369 ms), and Rust
     (518 ms).
   - *Depth, deadline, and cancellation:* `ran` in all three implementations.
     Self-recursion and `walk` stopped at depth 20; `range(10000)` nested `map`
     stopped within ~25 ms of a 25 ms deadline and immediately on cancellation;
     `range(1e15)`, `"a".repeat(1e15)`, and `2 ** 10000` failed cleanly with
     `steps-exceeded` / `number-range`.

### Area 2: Opaque values and host functions

8. **Go `value.SameOpaque` panicked on comparable structs holding uncomparable
   interface values (Fixed).**
   - *Location:* `go/internal/value/value.go` (`SameOpaque`).
   - *Before (`ran`):* A Go struct `type Box struct { Data any }` has
     `ta.Comparable() == true`, so `a == b` panicked at runtime when `Data` held
     a slice (`[]int`). The evaluator caught the panic and surfaced
     `internal-error`, violating `spec/core/03-host.md` 3.4.6 item 4 ("The test
     cannot fail").
   - *Fix:* Guard `a == b` in `SameOpaque` with `defer recover()` returning
     `false` on runtime comparison panics.
   - *Verification (`ran`, `go test`):* `TestAdversarialOpaqueAndHostBoundary`
     confirms `a == b` returns `false` with no error.

9. **Go `value.KindOf` and `value.Read` stack-overflowed on cyclic Go pointers
   (Fixed).**
   - *Location:* `go/internal/value/value.go` (`KindOf`, `Read`).
   - *Before (`ran`, standalone subprocess):* Passing `var x any; x = &x` as a
     host variable caused `reflect.ValueOf(x).Elem().Interface()` to recurse
     infinitely, killing the Go process with an unrecoverable
     `fatal error: stack overflow`.
   - *Fix:* Bounded `reflect.Pointer` dereferencing in `kindOf` and `readAt` to
     `maxPtrDepth = 64` indirections, wrapping deeper/cyclic pointer chains in
     a dedicated opaque `deepPointer` value whose `KindOf` is immediately
     `OpaqueKind` (preventing kind/value mismatch panics on subsequent `Equal`
     or typed dispatch).
   - *Verification (`ran`, `go test`):* `TestAdversarialOpaqueAndHostBoundary`
     confirms `type(x)` returns `"opaque"` and `x == x` / `x == 1` do not panic
     on both self-referential pointers and 65-level pointer chains.

10. **Rust `Fail::diagnostic()` expanded shared DAGs exponentially inside
    `Program::run` (Fixed).**
    - *Location:* `rust/src/eval/mod.rs` (`Fail::diagnostic`),
      `rust/src/value.rs` (`to_data_bounded`).
    - *Before (`ran`, `cargo test`):* A 22-line program constructing a 20-level
      shared list DAG (`a0 = [1, 1]`, `a{i} = [a{i-1}, a{i-1}]`) and calling
      `fail("boom", a20)` used 1 step and 63 allocation units, yet
      `Fail::diagnostic()` called `v.to_data()` unconditionally inside
      `Program::run`, taking **963.1 ms** to materialize 2^20 nodes (and OOMing
      on depth 30 = 10^9 nodes). Go and TS share references in `Diagnostic.data`
      and returned in ~0.1 ms.
    - *Fix:* Added `Value::to_data_bounded(max_nodes)` and bounded
      `Fail::diagnostic()` at 100,000 total emitted `Data` nodes and container
      slots. Before iterating a list or copying a map's entries, `to_data_at`
      reserves all `n` child slots from the remaining budget; any container
      whose length exceeds the remaining budget collapses as a whole subtree to
      `Data::Null` (so emitted lists never silently lose elements, emitted maps
      never drop keys, and total emitted nodes and slots stay at most 100,000).
    - *Verification (`ran`, `cargo test --test adversarial`):*
      `fail_with_shared_dag_is_bounded_inside_program_run` tests depth 30 (2^30
      logical nodes), shared wide-container DAGs, and >100,000-element host
      lists and maps, verifying total emitted nodes stay <= 100,000 and finish
      in milliseconds.

11. **TS `call.charge` and `call.allocate` accepted fractional and non-finite
    numbers (Fixed).**
    - *Location:* `ts/src/eval/compile.ts` (`callHost`), `ts/src/eval/run.ts`
      (`charge`, `alloc`).
    - *Before (`ran`, Node v22.22.3):* Calling `h.charge(1.5)` or
      `h.allocate(1.5)` left fractional remaining counts (`98.5`), causing
      `env.with({ limits: h.remaining() })` to throw `bad-limit`. Calling
      `h.charge(Infinity)` when `steps: null` set `this.steps` and
      `this.nextCheck` to `Infinity`.
    - *Fix:* Round positive finite host charges up with `Math.ceil(n)` so a host
      function can never undercharge or produce non-integer `remaining()`
      counts; saturate `steps`/`units` and `nextCheck` at
      `Number.MAX_SAFE_INTEGER` when a limit is off, while keeping `Infinity`
      as an immediate limit failure when the limit is finite.
    - *Verification (`ran`, `node --test test/adversarial.test.ts`):* Verified
      negative, `NaN`, fractional, and `Infinity` charges under finite and
      unlimited budgets.

12. **Prototype isolation, panics, and cause depth verified (Pass).**
    - *TS prototype safety (`ran`):* Maps created by map literals, `json.parse`,
      and `toMap` are `RMap` (`Map`) instances; `decode` uses
      `Object.defineProperty`; host object reads use `hasOwnProperty` /
      `propertyIsEnumerable`. Keys `__proto__`, `constructor`, `toString`, and
      `hasOwnProperty` neither leak prototype methods nor pollute
      `Object.prototype`.
    - *Host panics and cause depth (`ran`):* Host panics produce uncatchable
      `function-panic` in Go, TS, and Rust; 12-level nested `RisorError` cause
      chains cap at depth 8 in `RenderJSON` / `renderJSON` (F19 5.2).

### Area 3: Nested runs with `remaining()` (#689)

13. **Budget sharing verified across loops, `try/catch`, and zero budgets
    (Pass / Fixed via Finding 11).**
    - *Tested (`ran` in Go, TS, Rust):* A host function running a child program
      with `Budget(c.Remaining())` / `{ limits: h.remaining() }` inside
      `try { range(10).map(_ => sub()) } catch { [] }` stops with uncatchable
      `steps-exceeded` without amplifying steps or allocation units. Post-return
      handle calls reject and report `{ steps: 0, allocation: 0 }`; zero
      remaining budgets (`0`) remain zero rather than reverting to defaults.

14. **Depth and environment deadlines are not shared across nested runs
    (Ruled — by specification).**
    - *Basis:* `spec/core/03-host.md` 3.2.5 and 3.5.2 item 9 explicitly specify
      that depth is counted from 0 in each nested run against its own
      environment's limit, and that `remaining()` carries no time. Hosts that
      allow recursive host-to-Risor-to-host re-entry must bound nesting depth
      themselves and propagate their cancellation signal (`c.Context()` /
      `h.signal` / `cancel`) with any wall-clock deadline attached to it.

### Area 4: Conformance of host data (F4, F8)

15. **Go `ConformAll` misreported bad host numbers in union tags and sorted
    map keys twice (Fixed).**
    - *Location:* `go/internal/eval/conformall.go` (lines 174 and 217–229).
    - *Before (`ran`):* `tv, _ = value.Read(tv)` discarded `rerr`. Passing
      `{"kind": math.NaN()}` or `{"kind": math.MaxInt64}` to `ConformAll`
      against `{kind: "a"} | {kind: "b"}` reported `expected "a" | "b", found
      null` instead of `found number` (disagreeing with `Conform`). Also,
      `allWalker.record` called `value.Keys(v)` twice, sorting all map keys
      twice.
    - *Fix:* Check `rerr != value.ReadOK` and emit `number: true` (reporting
      `found number`, matching `Conform`), and reuse the single `value.Keys(v)`
      slice.
    - *Verification (`ran`, `go test`):*
      `TestAdversarialRemainingAndHostConformance` confirms `Conform` and
      `ConformAll` agree on `NaN`, `+Inf`, and `MaxInt64`.

16. **Cyclic data, step limits, bad numbers, and invalid text (Pass).**
    - *Tested (`ran` in Go, TS, Rust):* Cyclic maps/objects against recursive
      `Node = { self: Node }` hit `depth-exceeded` cleanly in `as`, `Conform`,
      `ConformAll`, and `JSONText`/`jsonText`; huge non-conforming lists stop at
      step and error caps; `NaN`, `Inf`, `MaxInt64`/`BigInt > 2^53-1` fail with
      `number-range`; `-0` normalizes to `+0`; invalid UTF-8 maximal subparts
      (Go) and lone UTF-16 surrogates (TS) decode to `U+FFFD`.

### Area 5: Editor tools on hostile text

17. **TS `tokens()` had O(N^2) and O(N^3) hangs on repeated unclosed multiline
    constructs (Fixed).**
    - *Location:* `ts/src/tools/tokens.ts`, `ts/src/syntax/lexer.ts`
      (`lexPrefix`).
    - *Before (`ran`, Node v22.22.3):*
      - `"/*\n".repeat(3000)` (9 KB) took **506.1 ms** (O(N^2), re-scanning to
        EOF on every line).
      - `` "`${\n".repeat(3000) `` (12 KB) hung past **10,000 ms** (O(N^3):
        each line peeled 3,000 nested templates one by one from the right, then
        re-lexed the suffix on every line).
    - *Fix:* Added single-pass `lexPrefix(src)` in `lexer.ts` that returns the
      already-completed top-level tokens and jumps directly to the outermost
      open template/text-block start on nested errors (eliminating the inner
      O(N^2) retry loop). In `tokens.ts`, tracked cumulative recovery scan work
      and switched to line-by-line lexing once recovery exceeds `max(4096,
      4 * cps.length)` code points, bounding total work to O(N).
    - *Choice made:* Once the scan budget trips, single-line tokens keep exact
      colour, but multi-line constructs (`/* ... */`, multi-line templates, text
      blocks) after that point are lexed one line at a time and lose multi-line
      colour (their opener line is marked `invalid`). This trade-off is pinned
      by `tokens() pins colours after the line-by-line recovery fallback trips`.
    - *After (`ran`, Node v22.22.3):*
      - `"/*\n".repeat(10000)` (30 KB): **75.6 ms**
      - `` "`${\n".repeat(10000) `` (40 KB): **102.8 ms**
      - `"const x = 1; /*\n".repeat(10000)` (160 KB): **88.4 ms**
      All existing `tokens.test.ts` cases (including 1,000+ suite programs and
      20,000-deep valid nested templates) pass unchanged.

18. **Formatter, completion, and hover on hostile text (Pass).**
    - *Tested (`ran` in Go, TS, Rust):* 5,000 unclosed `(`, `{`, `[`, 5,000
      `const a = ; `, 10,000 `a + `, 500 `if (1) { `, 20 KB unclosed `"""`,
      5,000 `/*\n`, embedded NUL bytes, and raw invalid UTF-8 bytes (Go)
      completed in <100 ms per suite with zero panics and zero hangs.

## Residual assumptions and rulings for hosts

The specification leaves four boundary conditions to the host. They are not
bugs in the implementations, but hosts running untrusted programs should uphold
them:

1. **DAG amplification outside a run (Ruled).** A short program can build a
   shallow shared DAG (`const a0 = [1, 1]; ...; const a28 = [a27, a27]`) in <100
   allocation units with depth 28 (well below the 200 depth limit). Operations
   *inside* a run (`==`, `json.stringify`, template rendering, `as`) charge
   steps per visited element and stop at `steps-exceeded`. Conversions *outside*
   a run (`JSONText`/`jsonText`/`json_text`, `Decode`/`decode`, Rust
   `Value::owned()`, and Rust `load_value` with expected type `any`) are
   specified as uncharged (`03-host.md` 3.2.4, `f10-config.md` 6.1, R324) and
   bound only depth (200), not expanded node count. Hosts serializing untrusted
   results to tree formats should either call `json.stringify` inside the run or
   validate against a non-recursive schema before decoding.
2. **Nested run re-entry depth and deadlines (Ruled).** Per `03-host.md` 3.2.5,
   `remaining()` transfers step and allocation budgets only. Hosts exposing a
   function that invokes Risor recursively must cap host re-entry depth and
   propagate a deadline-bearing cancellation signal.
3. **Host function duties (`03-host.md` 3.5.4).** A host function that performs
   unbounded CPU or memory work without calling `charge`/`allocate`, or mutates
   host containers already exposed to the run, breaks the language's bounds and
   immutability guarantees by design.
4. **Go `Opaque` `OpaqueType()` method.** As documented in `go/env.go`,
   `TypeOf` calls `OpaqueType()` on a zero value (or nil pointer); a host type
   whose `OpaqueType()` dereferences a nil receiver will panic at environment
   declaration time.

## Full verification commands

All commands below were run after the final edits and exited with code 0:

```bash
(cd go && go build ./... && go vet ./... && go test -count=1 ./...)
(cd ts && npm run typecheck && npm run build && npm test && npm run suite)
(cd rust && cargo fmt --check && cargo clippy --all-targets -- -D warnings \
  && cargo test)
(cd fuzz && GOWORK=off go test ./...)
```
