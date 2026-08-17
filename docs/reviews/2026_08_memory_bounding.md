# Memory Bounding

This came out of [issue #473](https://github.com/deepnoodle-ai/risor/issues/473), which
showed that a Risor script can allocate several gigabytes while staying well inside
`WithMaxSteps` and `WithTimeout`.

Memory bounding was never a Risor design goal, so this isn't a defect in ordinary
embedding. It's the reason that running fully untrusted scripts in-process, which is
what the issue asks for, is harder than the existing limits make it look. This document
maps out how much harder, how far you can get today, and what it would cost if we
decided to support the use case properly.

Every number below was measured against commit `a263ecc` using
`runtime.MemStats.TotalAlloc` deltas, on Go 1.26.2, darwin/arm64.

## The short version

Risor limits how many instructions run and how long they run for. It doesn't limit how
much work a single instruction does, and a single instruction can allocate gigabytes.
`list(range(50000000))` allocates 5.4 GB under a 1000-step limit. That's fine when you
trust the script. It's the whole problem when you don't.

You can get where the issue wants to be today without changing Risor, and for the
spreadsheet-mapping case it describes the answer is complete rather than partial. What
makes it work is that Risor v2 has no loop keywords. Turn off function definitions and
every script runs top to bottom exactly once, at which point capping each individual
operation caps the whole program. Measured: peak allocation on the repro drops from
572 MB to 8 MB, and to zero on `"a".repeat(1000000000)`, while ordinary field-mapping
expressions keep working. Section 3 has the recipe.

If we did want to support this in the library, the piece that covers the issue is
roughly 150 lines. Section 4 covers what it would take beyond that to offer a real
`WithMaxMemory`.

One genuine bug did fall out of the investigation, and it's unrelated to the design
question: `WithTimeout` can be escaped entirely. That's in §1.2.

---

## 1. Where the memory goes

The use case in the issue is a template engine where the script is untrusted but the
data is controlled: mapping spreadsheet cells to normalized values, like turning the
country name "Germany" into the code "DE". It has to run in-process next to a larger
pipeline, so sandboxing it in a container or a Lambda isn't available.

`WithMaxSteps` and `WithTimeout` bound instructions and wall-clock time, which is what
they claim to do. Neither says anything about allocation. The tables below are really a
measurement of how much room that leaves.

### 1.1 How much one script can allocate

Every row runs with `WithMaxSteps(1000)` and `risor.Builtins()`. Every one of them
returns a result instead of an error.

| What it does | Script | Allocated |
| --- | --- | ---: |
| Materialize a range | `len(list(range(50000000)))` | **5,468 MB** |
| Spread a list into itself | `[...a,...a,...a]` × 6 levels | 966 MB |
| Chain string concatenation | the script from the issue | 572 MB |
| Sort a big list | `sorted(list(range(5000000)))` | 555 MB |
| Repeat a string | `"a".repeat(500000000)` | 476 MB |
| Repeat bytes | `bytes("a").repeat(500000000)` | 476 MB |
| Interpolate a big string | `` `${a}${a}…` `` where `a` is 1 MB | multiplies |

Two things worth pulling out.

Lists cost about 100× more per element than strings do. Each element is an interface
header plus a boxed `*Int`, which measured at roughly 109 bytes per element against
1 byte per character for a string. That's why the range case dwarfs everything else.

And `list(range(50000000))` allocates 5.4 GB from one instruction. If you want a single
sentence for why step counting doesn't help here, that's it.

### 1.2 A real bug found along the way: `WithTimeout` can be escaped

Everything above is a limit Risor never promised. This one is different. `WithTimeout`
is documented as returning `context.DeadlineExceeded` when the timeout is exceeded, and
there are scripts where it doesn't return at all.

Risor v2 has no loop statements (see §3.3), but list literals can share structure, so a
few lines of source can build a huge tree in almost no memory:

```risor
let a = [1]
a = [a, a]   // repeated 30 times
// ...
len(string(a))
```

Building that costs about 0 MB and 60 instructions. `len(a)` is 2. But rendering it with
`string()` walks 2^30 elements inside one builtin call:

```
WithTimeout(2s) + ctx deadline 2s  ->  had not returned after 20s
```

The cause is structural. `start()` spawns a goroutine that sets `vm.halt`
(`pkg/vm/vm.go:285-290`), but the flag is only read at the top of the eval loop
(`pkg/vm/vm.go:501`), so Go code running inside a builtin can't be interrupted. Any
builtin whose runtime grows faster than its input escapes both `maxSteps` and
`WithTimeout`.

This deserves its own issue, since it's a divergence from documented behavior rather
than a design gap. It does share a root cause with #473, though: the VM counts
instructions, not work. Phase 4 in §4 fixes it.

---

## 2. Why you can't bound this from outside today

Four separate reasons, any one of which would be enough on its own.

**The operator interface has nowhere to put a budget.**

```go
// pkg/object/object.go:95
RunOperation(opType op.BinaryOpType, right Object) (Object, error)
```

No `context.Context`, no VM handle, no allocator. Nothing you can attach a per-execution
limit to, and so nothing that reaches `String.runOperationString`
(`pkg/object/string.go:239`), the four lines that produce the reported gigabyte.

**There's no single place allocation happens.** It's spread across ten VM opcodes
(`BinaryOp`, `BuildString`, `BuildList`, `BuildMap`, `ListAppend`, `ListExtend`,
`MapMerge`, `MapSet`, `Slice`, `BinarySubscr`), 82 registered type attributes
(`ls pkg/object/*.go | grep -v _test | xargs grep -h '\.Define(' | wc -l`), 25 builtins
and 3 modules. Each one calls `object.NewString`, `NewList`, `NewMap` or `NewBytes`
directly.

**You can't swap in your own string type.** `object.Object` is an open interface, but the
VM type-switches on the concrete `*object.String` for map keys (`vm.go:840`, `vm.go:930`)
and template joins (`vm.go:1022`). A size-aware replacement would break at those sites.

**Step counting is batched, and it looks backwards.** `maxSteps` is checked every
`contextCheckInterval` instructions, 1000 by default (`vm.go:509-537`). Even with a
correct cost model, the check happens after the allocation.

### One distinction that decides everything below

Every approach in this document either:

- **checks before allocating** — work out the output size from the inputs, refuse if it
  doesn't fit; or
- **allocates and then charges** — do the work, measure the result, fail afterwards.

Only the first bounds peak memory. Charging afterwards limits how much you accumulate
across many operations, but it can't stop one operation from spiking, and one spike is
enough to get the process OOM-killed. Keep this in mind for everything that follows.

---

## 3. What you can do today, with the library as-is

More than I expected going in. Risor already ships the pieces needed to run untrusted
scripts under a memory budget; they're just not labelled that way, and none of the
documentation connects them. For the use case in the issue this adds up to a real
answer, not a partial one.

### 3.1 What the shipped options give you

| Option | What it limits | Hard limit? |
| --- | --- | --- |
| `WithMaxSteps` | instruction count | batched ±1000, and no model of work |
| `WithTimeout` | wall clock | no, escapable inside a builtin (§1.2) |
| `WithMaxStackDepth` | value stack and frame depth | yes |
| `WithEnv` | what the script can reach | yes |
| `WithSyntax` | what grammar is allowed | yes |
| `WithValidator` | rejects any AST you don't like | yes |
| `WithTransform` | rewrites the AST before compiling | yes |

The bottom four do the real work. The top two are the ones people reach for first.

### 3.2 Cut down the environment

The environment is empty by default, and `risor.Builtins()` is opt-in. This is the
biggest win available for the least effort:

```go
env := map[string]any{
    "len": ..., "string": ..., "int": ..., "sprintf": ...,
}
risor.Eval(ctx, src, risor.WithEnv(env))  // not risor.Builtins()
```

Leaving out `list`, `range`, `sorted`, `chunk` and `encode` removes the 5,468 MB and
555 MB cases outright, along with several others.

One catch: this does not remove type *methods*. `"a".repeat(n)` still works no matter
what's in the environment, because methods resolve through `GetAttr` on the object
rather than through the environment. You need §3.4 for those.

### 3.3 Restrict the grammar, and the fact that makes this work

**Risor v2 has no loop statements.** The keyword table
(`internal/token/token.go:129`) has no `for`, `while` or `range`, and I confirmed that
`for`, `while`, `for-in` and `range` statements are all parse errors. Iteration only
exists through callbacks (`.map`, `.each`, `.filter`, `.reduce`) and recursion. Both of
those need a function definition.

So `DisallowFuncDef` makes every script run top to bottom exactly once. Instruction
count becomes proportional to the length of the source, and total allocation becomes the
sum over a fixed, source-bounded number of operations. That's what turns a leaky wrapper
into a real bound: once the program can't loop, capping each individual operation caps
the program. I verified that `.each(x => ...)`, `.map(x => ...)` and recursion are all
rejected at validation time under `DisallowFuncDef`.

`SyntaxConfig` also closes two holes that nothing else can reach, because they happen
inside VM instructions with no function call to wrap:

- `DisallowSpread` kills the 966 MB list-spread case
- `DisallowTemplates` kills the string-interpolation multiplier

Skip the presets, though. `ExpressionOnly` looks like it ought to help and does nothing
here. It still allows operators and calls, so `"aaaa".repeat(n) + "b"` sails straight
through. It restricts grammar, not cost.

### 3.4 Rewrite the AST and check sizes in the guards

`pkg/ast` is fully exported with plain settable struct fields, and `WithTransform` runs
between parsing and compiling. That's enough to route `+` and every method call through
functions you control.

```go
switch x := n.(type) {
case *ast.Infix:
    if x.Op == "+" {
        return &ast.Call{
            Fun:  &ast.Ident{NamePos: x.OpPos, Name: "__add"},
            Args: []ast.Node{x.X, x.Y},
        }
    }
case *ast.ObjectCall:   // note: NOT ast.Call{Fun: *ast.GetAttr}, which is easy to assume
    name := x.Call.Fun.(*ast.Ident)
    args := append([]ast.Node{x.X, &ast.String{Value: name.Name}}, x.Call.Args...)
    return &ast.Call{
        Fun:  &ast.Ident{NamePos: x.Period, Name: "__method"},
        Args: args,
    }
}
```

A reflection-based walker covers the whole tree in about 60 lines, since every field is
an exported `Expr`, `Node` or slice of them.

The guards need to check sizes *before* the allocation happens:

```go
env["__add"] = object.NewBuiltin("__add", func(ctx context.Context, args ...object.Object) (object.Object, error) {
    a, b := args[0], args[1]
    switch a.(type) {
    case *object.String, *object.Bytes, *object.List:
        if err := budget.reserve(sizeOf(a) + sizeOf(b)); err != nil {
            return nil, err          // refused BEFORE object.BinaryOp allocates
        }
    }
    return object.BinaryOp(op.Add, a, b)
})

env["__method"] = object.NewBuiltin("__method", func(ctx context.Context, args ...object.Object) (object.Object, error) {
    self, name, rest := args[0], args[1].(*object.String).Value(), args[2:]

    // methods that take a multiplier get checked up front
    if name == "repeat" && len(rest) == 1 {
        if n, ok := rest[0].(*object.Int); ok {
            if err := budget.reserve(sizeOf(self) * n.Value()); err != nil {
                return nil, err
            }
        }
    }

    attr, found := self.GetAttr(name)
    if !found {
        return nil, fmt.Errorf("attribute %q not found on %s", name, self.Type())
    }
    res, err := attr.(object.Callable).Call(ctx, rest...)
    if err != nil {
        return nil, err
    }
    if name != "repeat" {
        return res, budget.reserve(sizeOf(res))  // everything else charges afterwards
    }
    return res, nil
})
```

One `__method` wrapper covers all 82 type methods, because `GetAttr` hands back a
`*Builtin` that already satisfies `object.Callable`.

Worth knowing: plain builtin calls, meaning `ast.Call` with an `*ast.Ident`, are not
rewritten by this transform. You have to wrap those yourself when you build the
environment. Since you're building it anyway, that's a loop over a map rather than a
problem.

### 3.5 What the whole setup measures

Configuration: hand-built environment, `DisallowFuncDef` plus `DisallowSpread`,
`DisallowTemplates`, `DisallowPipe`, `DisallowTryCatch` and `DisallowReturn`, the `+`
and method transform with size checks, 10 MB budget.

| Case | Before | Peak after | What happens |
| --- | ---: | ---: | --- |
| The repro from #473 | 572 MB | **8 MB** | stopped at the budget |
| `"a".repeat(1000000000)` | ~950 MB | **0 MB** | refused before allocating |
| `bytes("a").repeat(…)` | 476 MB | 0 MB | `bytes` isn't in the environment |
| List spread | 966 MB | 0 MB | rejected at validation |
| Template string | multiplies | 0 MB | rejected at validation |
| `code + ": " + street + " " + num` | — | 0 MB | works, returns `DE: Hauptstrasse 42` |

The zero rows come from checking before allocating. This is a genuine cap on peak memory
for scripts that can't loop, with no changes to Risor.

### 3.6 What's still open

Four things I couldn't close from outside.

**Shared list structure.** `a = [a, a]` repeated 30 times is a handful of lines, about
0 MB allocated, and `len(a) == 2`. `op.BuildList` happens inside a VM instruction, so
there's no call to wrap. It stays harmless right up until something expands it, so don't
whitelist `string`, `sorted`, `encode` or `chunk` over lists. If you need `string()` on
arbitrary values, you need the library change.

**Your size function is itself a hazard.** A naive recursive `sizeOf` over the tree
above is 2^30 operations and hangs, which turns your guard into the vulnerability. Give
it a budget of nodes to visit and a depth cap, and have it return "too big" instead of a
number when it hits either.

**Methods that charge afterwards.** Anything whose output size isn't a cheap function of
its inputs still allocates first. In practice the inputs are already inside the budget,
so this costs you a constant factor rather than leaving a hole.

**The transform is coupled to AST node shapes.** `ast.ObjectCall` versus
`ast.Call{Fun: GetAttr}` already cost me a debugging cycle, and a node type added
upstream would be a silent gap. Worth a test that asserts the rewrite fires on every
construct that allocates.

`WithTimeout` also stays soft (§1.2), independently of any of this.

---

## 4. What it would cost to support this in the library

Everything here is optional. Risor works fine today for embedders who trust the scripts
they run, and each phase below is a step toward a different product: one that can also
host scripts you don't trust. They're ordered so that stopping after any of them leaves
something coherent.

### Phase 1: check sizes at the opcode boundary

About 150 lines, no break to the public API, and it covers the case in the issue.

```go
// vm.go:683
case op.BinaryOp:
    opType := op.BinaryOpType(vm.fetch())
    b := vm.pop()
    a := vm.pop()
    if err := vm.reserveForBinaryOp(opType, a, b); err != nil {   // new
        ...
    }
    result, err := object.BinaryOp(opType, a, b)
```

Add `maxBytes` and `bytesUsed` to `VirtualMachine` along with a `WithMaxMemory(n int64)`
option, add a shallow `object.Sizeof(Object) int64` with the visit cap from §3.6, and do
the same check at all ten allocating opcodes.

No interface changes and nothing ripples into the object package. It covers both the
concatenation and spread cases with no spike, and it's the workable form of the first
suggestion in the issue: charge the cost before doing the work, rather than converting
it into steps and noticing 1000 instructions later.

Of the four phases this is the one with the best ratio of coverage to cost, so it's the
one to weigh first.

### Phase 2: methods and builtins

`callObject` (`vm.go:1448`) is the one place every `object.Callable` goes through, and it
has a `ctx`. Put the budget in the context; the pattern already exists as
`object.WithCallFunc` (`pkg/object/context_values.go`). Then two levels:

Charge afterwards at `callObject` for all 82 methods and 25 builtins. One place, cheap,
catches accumulation.

Check up front inside the six operations that multiply their input: `string.repeat`,
`bytes.repeat`, `list()` over a range (`builtins.go:59`), `chunk`, `sorted` and `encode`.
Output size is a cheap function of the inputs in each of them, so the check is easy and
it removes the spike.

### Phase 3: decide what the number means

The plumbing is easy. Deciding what a memory limit actually measures is the hard part.

**Charge cumulatively and never give anything back.** Safe and deterministic. Too
restrictive for a script that creates and discards a lot of temporaries, except that
without loops the churn is bounded by source length anyway, so for straight-line scripts
this stays within a small constant factor of what's actually live. It only becomes a
problem once callbacks are back on the table. That makes it a much more reasonable
default for v2 than it would be in a language with `for`.

**Track what's actually live.** What you'd want, but Go's GC gives you no hook, and
refcounting every object is a non-starter.

**Walk the VM roots and measure.** The middle path, and the part of this that would take
actual design work. Risor's VM has an explicit set of roots: `vm.stack[:sp+1]`,
`vm.globals`, the locals and free cells in `vm.frames[:fp+1]`, `vm.tmp`, and the
constants in loaded code (`vm.go:40-115`). So charge cumulatively, and when the counter
hits the ceiling, walk those roots and reset the counter to what's genuinely retained.
The walk needs a visited set, both because closures and cells form cycles and because
shared structure would otherwise be counted twice. Since it only runs at the ceiling,
the cost doesn't show up in normal execution.

Call it 200 to 300 lines. This is the difference between shipping a `WithMaxMemory` that
means what it says and shipping one that's really a cap on total allocations. The
explicit root set is what makes it tractable in Risor and impractical in most languages.

### Phase 4: make long-running builtins interruptible

Fixes §1.2. Pass `ctx` into the recursive walkers that can run away (`Inspect`,
`MarshalJSON`, `Equals`, `Compare`, `Sizeof`) and check it periodically, or cap how many
nodes they'll visit. Without this, `WithTimeout` stays soft no matter what happens with
memory.

### What each phase gets you

| | as-is | +P1 | +P2 | +P3 | +P4 |
| --- | :-: | :-: | :-: | :-: | :-: |
| Bound `+`, spread, templates | via syntax config | ✅ | ✅ | ✅ | ✅ |
| Bound methods and builtins | wrapper, charged after | — | ✅ | ✅ | ✅ |
| Bound allocation inside opcodes | ❌ | ✅ | ✅ | ✅ | ✅ |
| Limit reflects what's live | ❌ | ❌ | ❌ | ✅ | ✅ |
| Safe to allow callbacks | ❌ | partial | partial | ✅ | ✅ |
| Hard wall-clock bound | ❌ | ❌ | ❌ | ❌ | ✅ |
| `string()` safe on shared structure | ❌ | ❌ | ❌ | ✅ | ✅ |

---

## 5. Recommendations

### 5.1 If you need this today

1. Build the environment by hand instead of calling `risor.Builtins()`. Whitelist what
   you need, something like `len`, `string`, `int`, `float`, `bool`, `type` and
   `sprintf`, and wrap each one so it charges the budget.
2. Set `WithSyntax` with `DisallowFuncDef`, `DisallowSpread`, `DisallowTemplates`,
   `DisallowPipe`, `DisallowTryCatch` and `DisallowReturn`. `DisallowFuncDef` is the one
   that matters most, since it's what stops the script from looping.
3. Add a `WithTransform` that rewrites `+` and `ObjectCall`, checking operand sizes in
   `__add` and the multiplier in `repeat` before the work happens.
4. Keep `WithMaxSteps` and `WithTimeout` as a backstop.
5. Cap `sizeOf` by visit count and depth.

Measured across every case I could find: peak allocation between 0 and 8 MB against a
10 MB budget, with normal field-mapping expressions unaffected. Nothing to fork, nothing
to merge upstream, and no change to how the surrounding pipeline is deployed.

### 5.2 For Risor

**Fix §1.2, and file it separately.** That one is a divergence from what `WithTimeout`
documents, so it stands on its own regardless of what we decide about memory. Phase 4
covers it.

**Say plainly what the limits do and don't cover.** The opening line of #473 was that
`WithMaxSteps` and `WithTimeout` suggest execution can be bounded. That's a fair reading
of the option names, and the answer is that they bound two specific things and not
allocation. Worth stating in the option docs whether or not anything else changes, since
the reading in #473 is the one most embedders will make.

**Phase 1 is worth considering on its own merits.** 150 lines and no API break, and it
would turn "Risor doesn't bound memory" into "Risor bounds memory at the operations
where it's cheap to know the answer." Whether that's worth doing is a scope question
about what Risor is for, not a bug to be fixed. I lean yes, because it's small and
because it makes the untrusted-script story go from "not really" to "yes, with care."

**The earlier "not feasible" answer holds up better than it might look.** It's right for
cumulative-only accounting, which is the obvious way in. Two things I didn't expect
change the picture: v2 has no loop statements, which makes cumulative accounting far
more workable than it would otherwise be, and the VM's explicit root set makes the
walk-and-measure approach practical. Neither is visible without going looking.

**The guide may help more than any of this.** The pieces already exist and compose well
(`WithEnv`, `WithSyntax`, `WithValidator`, `WithTransform`), but nothing told an embedder
that `DisallowFuncDef` is what makes resource limits tractable in the first place. Most
people who want what #473 asks for can have it today and don't know it. §3 of this
document is now written up as
[`docs/guides/memory-limits.md`](../guides/memory-limits.md), with working code and an
explicit list of what it doesn't cover.

---

## 6. Reproducing any of this

The measurement setup is easy to rebuild: a module with a `replace` directive pointing
at this repo, `runtime.ReadMemStats` around each `risor.Eval`, and the scripts from the
tables above. For the `DisallowFuncDef` finding, assert that
`[1,2,3].each(x => print(x))` fails validation while `len("a".repeat(100000000))` still
succeeds. That second half is exactly why the transform layer is needed too.
