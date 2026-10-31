# Allocation Budget (`WithMaxAllocatedBytes`)

**Status:** Rejected (2026-10-31)
**Author:** Claude, for Curtis Myzie
**Date:** 2026-10-31
**Workflow:** Spec → build in parallel. The design is mostly settled. The cost
constants are not, so the first step of the build is the invariant test that
measures them.
**Related:** [#473](https://github.com/deepnoodle-ai/risor/issues/473),
[`docs/reviews/2026_08_memory_bounding.md`](../reviews/2026_08_memory_bounding.md),
[`docs/guides/memory-limits.md`](../guides/memory-limits.md)

## Decision

We are not building an allocation budget. This section records why. The
proposal below is kept as it was written for review. Its safety argument does
not hold, for the reasons given here.

### What the review found

Three independent reviewers each wrote scripts that charge a few megabytes
under this design and allocate 0.4 to 7 GB. The bound in Goal 2 (allocation
under 2 × budget + one per-operation cap) missed by 100× to 1000×. The
reasons fall into four groups:

- **The per-operation caps have holes**, so charging after the work is not
  safe. `sprintf` builds its result before it checks the size, `error()`
  never checks, `decode` builds the whole value before it checks, and string
  indexing allocates 4 bytes per character of the source string on every
  access. Each of these is now an issue (listed below).
- **Allocation that is not charged is not O(1) per step.** Frames with more
  than 256 locals allocate on every call, every caught error captures the
  whole call stack, and Go-side loops box scalars without running an
  instruction. `all(range(1, 5000001))` allocates 76 MiB under
  `WithMaxSteps(1000)`.
- **Conversions sit outside the VM.** `risor.Run` converts the result after
  the run ends, and host-function arguments are converted before the call.
  Returning a list that holds the same list many times allocates 424 MiB to
  convert after charging nothing.
- **The coverage test cannot enforce coverage.** Some callables are outside
  `AttrRegistry` (the `Regexp` methods), opcodes are not enumerated, and one
  fixture cannot exercise allocation that depends on the arguments.

### Why we stopped

The root cause is that Go has no allocator hook. Languages that bound memory
in-process own the allocator: Lua routes every allocation through a function
the host supplies, and V8 isolates have heap limits because V8 runs its own
GC. In Go, every path that allocates has to be found and charged by hand,
and every new method can open a new one.

A version that is good enough would be a large build with a permanent cost
for every method added after it. It still would not be a guarantee, so a host
with real untrusted input would still need a container, Lambda, or an
rlimit. The hosts left over, whose scripts are buggy rather than hostile, are
mostly served by the per-operation caps, steps, and a timeout, once the caps
hold.

### What we did instead

We filed the cases where today's limits did not do what the docs said, and
fixed the ones with small fixes:

- Fixed: [#593](https://github.com/deepnoodle-ai/risor/issues/593) string
  indexing copied the whole string to `[]rune`,
  [#595](https://github.com/deepnoodle-ai/risor/issues/595) conversion errors
  quoted the whole input,
  [#597](https://github.com/deepnoodle-ai/risor/issues/597) case mapping could
  pass `MaxBytesSize`,
  [#598](https://github.com/deepnoodle-ai/risor/issues/598) `list.insert(0, x)`
  copied the whole list, and
  [#599](https://github.com/deepnoodle-ai/risor/issues/599) Go-to-Risor
  conversions ignored `WithLimits`.
- Closed without a fix:
  [#594](https://github.com/deepnoodle-ai/risor/issues/594) (`sprintf` and
  `error()`) and [#596](https://github.com/deepnoodle-ai/risor/issues/596)
  (`decode`). The fixes needed a copy of `fmt`'s format handling and a
  hand-written JSON decoder. That was more complexity than the gaps were
  worth.

That experience settled the wider question. Making the per-operation limits
complete runs into the same problem as the budget, one allocation path at a
time. So Risor stopped adding limit machinery. `WithLimits` is documented as
a guard against accidents, not a security boundary, and the known gaps are
listed in [`docs/guides/memory-limits.md`](../guides/memory-limits.md). To
bound memory, run the script in a process the operating system can kill.

### When to revisit

Revisit when several embedders ask for an in-process bound and give use
cases that a subprocess with an rlimit cannot serve. Start from the review
findings above, not from this proposal. A new design would need charges
inside the limit helpers, a fixed charge per instruction, conversions inside
the budget, and coverage measured per opcode with `runtime/metrics` across
the whole test suite.

## Context

Issue #473 asks for a way to bound the memory an untrusted script can use
inside the host process. Today Risor bounds four things: instructions
(`WithMaxSteps`), wall-clock time (`WithTimeout`), value stack and call depth
(`WithMaxStackDepth`), and the size of the value one operation may produce
(`WithLimits`, added in #491 and #568).

The per-operation caps fixed the case in the issue. The `+` chain that built a
1 GB string now fails at the 64 MiB cap after 18 ms. They do not bound a whole
run, because a script can make many values that are each under the cap and
keep all of them. Measured on `main` (56f38eb):

| Script | Limits | Result | Go `TotalAlloc` |
| --- | --- | --- | --- |
| `range(16).map(n => s + string(n))`, `s` is 60 MiB | defaults | succeeds in 68 ms | 1,020 MB |
| `range(1000).map(n => s + string(n))`, `s` is 1 MB | 1 MiB cap, 10k steps | succeeds in 83 ms | 962 MB |

The reporter's use case is small field-mapping scripts in a long-lived
pipeline process. They expect each script to allocate a few kilobytes and
would set a 100 MB budget. They said the bookkeeping does not need to be exact
to be useful. The memory-bounding review (§4) describes an allocation budget
as the low-cost way in.

The limits work makes this much cheaper than when the review was written.
Every allocating opcode and every method that amplifies its input already
checks a size, and the limits already reach the `object` package through the
context. The budget can charge at those same places.

## Goals

1. A host can set a total allocation budget for a run with
   `risor.WithMaxAllocatedBytes(n)`. When a run charges more than `n` bytes,
   it fails with an error that matches
   `errors.Is(err, risor.ErrAllocationLimitExceeded)`.
2. Both scripts in the table above fail with a budget of 100 MB, and the Go
   `TotalAlloc` for the run stays under **2 × budget + one per-operation
   cap**. The invariant test checks the constant factor, and the PR reports
   the measured value.
3. Every registered method, builtin, module function, and allocating opcode
   is covered by the invariant test. A new method that has no test fixture
   fails the test.
4. The same script with the same inputs always charges the same total and
   fails at the same instruction. The count does not depend on the GC or on
   timing.
5. A script cannot catch the error, and `finally` blocks do not run, the same
   as `ErrStepLimitExceeded`.
6. With no budget set, `make bench` shows no change beyond noise. With a
   budget set, the overhead is under 5%.

## Non-goals

- **Bounding live memory or peak RSS.** This counts what a run allocates, not
  what it keeps. `WithMaxMemory` is a different feature (see Alternatives).
- **Bounding allocation inside host Go functions.** A function the host
  exposes is charged for the value it returns, not for its internal work.
- **A hard wall-clock bound.** The `WithTimeout` gap in the review's §1.2 is a
  separate fix.
- **Exact accounting of Go runtime overhead.** The cost model is an estimate
  with a constant factor that has been measured.
- **Changing `WithLimits`.** The per-operation caps stay as they are. The
  budget depends on them (see Proposal).

## Proposal

### API

```go
// WithMaxAllocatedBytes sets the maximum number of bytes a run may allocate
// for Risor values, counted from the start of the run. Bytes are charged when
// a value is created and are never given back, so values that the script
// discards still count. If the limit is exceeded, the run returns
// ErrAllocationLimitExceeded. A value of 0 (the default) means unlimited.
//
// The count is an estimate of what Risor values cost. It does not count
// allocations inside host functions, only the values they return.
func WithMaxAllocatedBytes(n int64) Option

var ErrAllocationLimitExceeded = object.ErrAllocationLimitExceeded
```

`vm.WithMaxAllocatedBytes` mirrors it, the same way `vm.WithMaxSteps` mirrors
`risor.WithMaxSteps`. The sentinel lives in `object`, because the methods that
charge are there. `vm` and `risor` re-export it.

It is a separate option, not a field of `Limits`. `Limits` bounds one
operation. This option bounds a whole run, which is what `WithMaxSteps` does,
and the docs should present the two together: steps bound work, allocated
bytes bound allocation.

The error message gives the limit, for example
`allocation limit exceeded (allocated more than 104857600 bytes)`.

### The budget

```go
// pkg/object/budget.go
type Budget struct {
    limit int64
    used  int64
}

func WithBudget(ctx context.Context, b *Budget) context.Context
func GetBudget(ctx context.Context) *Budget   // nil when unlimited

// Charge adds n bytes to the budget in ctx. It returns
// ErrAllocationLimitExceeded if the total passes the limit. It does nothing
// when ctx has no budget.
func Charge(ctx context.Context, n int64) error

// ChargeSlots, ChargeEntries, and ChargeBytes charge n list slots, n map
// entries, or n bytes. They are wrappers over Charge that apply the cost model.
func ChargeSlots(ctx context.Context, n int) error
func ChargeEntries(ctx context.Context, n int) error
func ChargeBytes(ctx context.Context, n int) error
```

The VM owns one `*Budget`. It resets `used` in `start`, next to `stepCount`,
so that each `Run` and each `Call` has its own budget, the same as steps.
`initContext` puts the pointer into the context next to the limits. Opcodes
charge `vm.budget` directly, without a context lookup. Methods and builtins
look it up with `GetBudget(ctx)`. They already call `GetLimits(ctx)`, so this
adds one more lookup on paths that already do one.

Like the step limit, once the budget is exceeded every later charge fails, so
a host that keeps running after the error cannot get past the limit.

A VM runs on one goroutine, and callbacks from Go builtins run on that same
goroutine, so `used` does not need to be atomic.

### Why charging after the work is safe

> **This argument does not hold.** The review found operations that
> allocate far past the cap before they check it. See [Decision](#decision).

No single operation can produce more than `MaxBytesSize` bytes or
`MaxListLength` elements. So where the output size is known only after the
work, such as `decode`, a regexp result, or a host function result, charging
afterward overshoots the budget by at most one cap. Where the size is known
first, which covers every opcode and every method that already checks its
size, the charge happens before the allocation, and there is no overshoot.

The budget therefore depends on the per-operation caps. Without them, every
method would need a check up front to avoid a spike. The guarantee in Goal 2
comes from both of them together.

### Cost model

| Value | Charge |
| --- | --- |
| String or bytes | its length |
| List | 16 bytes per new slot |
| Map | 64 bytes per new entry |
| Closure, cell, partial, error, iterator | 64 bytes each |
| Int, float, bool, nil | not charged |

Scalars are not charged on their own. Charging every arithmetic result would
turn the budget into a second step counter. A scalar that is kept lives in a
list or map slot, which is charged, and the scalars that are not kept are
already bounded by `WithMaxSteps`. Whether the slot cost needs to rise to
cover a boxed scalar is the first open question below.

The rule for every site: **charge what this operation allocates, not the size
of the result.** `list.append` charges one slot, not the length of the list.
`MapSet` copies the whole map, so it charges `len + 1` entries. A substring
that shares the parent's bytes (`trim`, `split`, slicing a string) charges
only the new slots or headers.

### Where charges go

**Opcodes in `pkg/vm/vm.go`**, next to the existing size checks:

| Opcode | Charge |
| --- | --- |
| `BuildList` | `count` slots |
| `BuildMap` | `count` entries |
| `BuildString` | the joined length, which the check already computes |
| `ListAppend` | 1 slot |
| `ListExtend` | `len(existing) + len(items)` slots (it copies) |
| `MapMerge` | `len(target) + len(source)` entries (it copies) |
| `MapSet` | `len(target) + 1` entries (it copies) |
| `StoreSubscr` | 1 entry when a map gains a key |
| `Slice` | the new slots or bytes, by container type |
| `Partial`, `CallSpread` | argument slots |
| `LoadClosure`, `MakeCell` | the fixed cost |

`BinaryOp` charges inside `object.BinaryOp`, which already calls
`checkConcatSize` for string, bytes, and list concatenation.

**Methods and builtins.** There are about 80 methods across the `AttrRegistry`
definitions in `pkg/object`, 25 builtins in `pkg/builtins/registry.go`, and
the `math`, `rand`, and `regexp` modules. Each one that allocates calls a
`Charge*` helper. Most of them already compute the size for a `Check*` call:
the 48 call sites from #491 and #568. The rest produce output no larger than
their input, such as `to_upper` or `list.copy`, and charge that size. Methods
that only read or that return stored values, such as `map.get`, `contains`,
`len`, and `index`, charge nothing.

`decode` and host function results build new nested values, so they charge
after the work with `object.SizeOf(obj)`. That is a deep walk that uses the
cost model. It keeps a visited set, so shared structure is counted once, and
it stops when it passes the remaining budget, so its cost is bounded. A host
function result that is already a Risor object was not created by the call,
so it is not charged.

Callbacks need nothing special. `list.map(f)` calls `f` through
`vm.callFunction`, so the opcodes inside `f` charge for themselves, and `map`
charges for the list it returns.

### The invariant test

Coverage is the real risk. A method added next year that allocates without
charging silently reopens the hole. The test that guards against this follows
`format_size_invariant_test.go`:

1. For every type registry (`Specs()`), every builtin, and every module
   function, look up a fixture: a receiver and arguments chosen so that the
   call allocates a lot if it allocates at all, such as a 1 MiB string or a
   100k-element list. **A callable that has no fixture fails the test**, so
   adding a method means adding a fixture.
2. Run each fixture in a VM with an unlimited budget. Record the charged
   bytes and the Go `TotalAlloc` delta, with `GOGC=off` for the measurement.
3. Assert `TotalAlloc ≤ k × charged + slack`, where `k` is the factor from
   Goal 2 and `slack` covers fixed per-call overhead.
4. Run a set of abusive scripts, the two from Context and the `+` chain from
   #473, plus spread, `MapSet`, and callback variants, under a 100 MB budget.
   Assert that each one fails with `ErrAllocationLimitExceeded` and that
   `TotalAlloc ≤ 2 × budget + MaxBytesSize`.

The test runs once without a budget to measure, and that run is how the cost
constants get tuned.

## Alternatives considered

**Charge the result of every call in `callObject`.** This is one place in
`pkg/vm/vm.go` that every Go method and builtin goes through, so it covers
methods automatically. It was the first idea. It fails because it cannot tell
a new value from an existing one. `list.append` returns its receiver, so a
loop of `n` appends would charge `1 + 2 + … + n` slots. `map.get` returns a
stored value, so reading a 1 MB string 1,000 times would charge 1 GB. Without
a way to know which results are new, the count is wrong by an unbounded
factor in both directions. Charging at each site, with the invariant test for
coverage, is more work but gives a number that means something.

**Weight steps by work, as the issue suggests.** `+` on two large strings
would cost many steps. That uses the same charge sites, so the cost is about
the same. But it changes what `WithMaxSteps` means for every embedder. Budgets
that are tuned for CPU today would start to fail on scripts that allocate,
and a host could not limit CPU and memory separately. A separate counter is
no more expensive and keeps the meaning of each option clear.

**Track live memory (`WithMaxMemory`).** This is the review's phase 3: charge
over time, and at the ceiling, walk the VM roots (stack, globals, frames,
cells, constants) to find out what is still alive. That is what most people
mean by a memory limit, and the explicit root set makes it practical. It is
deferred, not rejected. It needs real design work on what to count during the
walk, the reporter's use case does not need it, and it can be built later on
the same charge sites: the charges would trigger the walks.

**Use the Go runtime (`runtime.ReadMemStats`, `debug.SetMemoryLimit`).** Both
are process-wide. They cannot tell one VM from another when several run in
parallel, which the concurrency guide supports. `ReadMemStats` stops the
world, and `SetMemoryLimit` only makes the GC work harder. It does not stop a
script.

## Tradeoffs and consequences

- **Discarded values count.** A script that loops over 50k rows with callbacks
  and makes temporary strings uses up a budget that its live memory never
  reaches. That is fine for short scripts. For long-running scripts it is a
  poor fit, and the docs have to say so plainly. The name says "allocated" for
  this reason.
- **The number is an estimate.** It does not see the extra capacity that Go's
  `append` reserves, map bucket overhead, or allocations inside host
  functions. The invariant test measures the factor, and the docs give it.
  Hosts should set the budget below the memory they can actually spare.
- **Every new allocating method costs more to write.** It needs a charge and a
  fixture. The test enforces this, but it is still a cost for contributors.
- **Charges spread across many files.** About 60 call sites. The charge sits
  next to an existing size check at most of them, so a reviewer reads them
  together.
- **Not a hard guarantee.** It bounds what a script can request through Risor
  values. It does not bound the process. The external boundary at the top of
  the memory-limits guide is still the only hard bound.

## Security considerations

The threat is an untrusted script that tries to exhaust host memory. With the
budget set, what the script can still do:

- **Allocate up to about `k × budget + MaxBytesSize`.** Hosts size the budget
  for this.
- **Make a host function allocate.** A host function is charged only for its
  result. The guide already tells hosts to wrap the functions they expose.
- **Find a method the invariant test misses.** The fixture rule makes this a
  test failure instead of a silent gap, but only for callables the test can
  list. A host type that has its own `GetAttr`, outside `AttrRegistry`, is
  not listed.
- **Use CPU without allocating.** That is what `WithMaxSteps` and
  `WithTimeout` are for.

The budget does not make an untrusted script safe without the other limits. A
host should set the budget, steps, a timeout, and tight `Limits` together.

## Rollout

- Additive and off by default. No current behavior changes.
- One implementation PR: budget, charge sites, invariant test, docs. It is
  about 300 to 400 lines of production code and a similar amount of tests.
  The opcodes and the `object` package could be split into stacked PRs if
  review needs smaller pieces.
- Rewrite the in-process part of `docs/guides/memory-limits.md` to lead with
  the three options together. The AST-transform section becomes a fallback
  for hosts that need more than this option gives.
- Document the option next to `WithMaxSteps` in `risor.go` and the README.
- Reply on #473 with the before and after numbers, and close it when this
  ships.

## Open questions

1. **Cost constants.** Is 16 bytes per slot enough when the slot holds a boxed
   scalar? `range(n).map(i => i * 2)` allocates an `Int` per element that is
   not charged. Leaning toward 32 bytes per slot if the invariant test shows a
   factor above 2 for this case. The first build step settles it.
2. **Report the total to the host.** A host picking a budget needs to know
   what normal scripts use. Leaning toward exposing the charged total after a
   run, as a VM accessor or through the observer, because without it hosts
   will guess. This adds API, so it is decided before the PR.
3. **Budget across `Call`s.** Resetting per `Call` matches steps and suits a
   host that calls one function per row. A host that wants one budget across
   many calls could not have it. Leaning toward per call, the same as steps,
   until someone asks.
