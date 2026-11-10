# Risor v3 in Rust: layout and plan

This crate is written from `spec/` and `suite/` only. Where the specification
was not enough, the question went to the lead, and the answer is in the
specification, not here. This file records the crate's own choices.

## Rules

- No dependencies, in the library or the tests. JSON is read by `src/json.rs`,
  which also refuses a repeated key (`03-host.md` 3.10.10).
- No `unsafe` (`#![forbid(unsafe_code)]`).
- Offsets are counted in code points. `Source` holds the text as `Vec<char>`;
  every `Span` is two indexes into it. A diagnostic carries both the offsets
  and the line and column of `03-host.md` 3.7.7.
- **No input overflows the stack.** Recursion is allowed only where it is
  bounded by a number of the specification: the 100 levels of source nesting
  (`01-language.md` 13), the 64 brackets of type text (`05-types.md` 20.5),
  and, from S2, the depth limit. What opens no level is walked by a loop.

## Layout

| Path | Stage | Holds |
| --- | --- | --- |
| `src/source.rs` | S1 | `Source`, `Span`, `Pos`, `LineSpan` |
| `src/diag.rs` | S1 | `Diagnostic`, `Error`, `Phase`, the code names |
| `src/syntax/token.rs` | S1 | Tokens, the 26 reserved words |
| `src/syntax/lexer.rs` | S1 | The lexer. Templates nest through an explicit stack |
| `src/syntax/ast.rs` | S1 | The 30 node kinds, in an arena |
| `src/syntax/parser.rs` | S1 | The parser: recursive descent, precedence climbing for binary operators |
| `src/types/text.rs` | S1 | The parser of type text: `Type`, `Sig`, `Method`, with the counts of `05-types.md` 22.3 |
| `src/json.rs` | S1 | The strict JSON reader |
| `src/testing/` | S1 | The case-file runner: `case.rs` reads and validates, `run.rs` runs and compares |
| `src/value.rs` | S2 | `Value`, `Str`, lists and maps behind `Rc`, weight and size, truthiness |
| `src/number.rs` | S2 | Arithmetic of 6.2, number to text and text to number |
| `src/host.rs` | S2 | `Host`, `View`, `HostList`, `HostMap`, `Vars`, `Data`, `OpaqueRef` |
| `src/env.rs` | S2 | `Env`, `EnvBuilder`, `Limits`, host functions and methods, the `declare` errors |
| `src/compile.rs` | S2 | Name resolution to fixed slots, the depth measure of each call, the static rules |
| `src/eval/mod.rs` | S2 | The evaluator, the meter, the call handle, failures and the error value |
| `src/eval/lib.rs` | S2, S3 | The library entries: `len`, `string`, `type`, `bool`, `fail`, `map`, `filter` in S2 |
| `src/library.rs` | S2 | The records of `suite/library.json`: names, receivers, signatures |
| `src/messages.rs` | S2 | The catalog of `suite/messages.json` and the building of a message |
| `src/bin/outcome.rs` | S2 | The outcome command of rulings round 10 AJ |

The data files of `suite/` (`messages.json`, `library.json`) are compiled into
the crate with `include_str!` and read once, at first use. Nothing in them is
retyped, and the table cannot fall out of date: a change to a file is a change
to the build.

## The syntax tree

The tree is an arena (`Ast::nodes`), and a node names its children by
`NodeId`. Three reasons:

1. A sum of 100,000 terms is a left-deep tree 100,000 nodes deep (13.2 rule
   6). Boxed nodes would drop it by recursion and overflow the stack. An
   arena drops it as one `Vec`.
2. A `Program` holds no `Rc`, so a compiled program is `Send + Sync` and can
   be shared by runs on several threads (`03-host.md` 3.2.2).
3. The checker (M8) and the static rules can keep facts per node in a side
   table indexed by `NodeId`.

Each node has `span` (4.3) and `outer`, the span with its parentheses, which a
node that holds it starts or ends at (4.3 rule 1). `Bad` exists for the
numbering; the parser stops at its first diagnostic and never builds it.

**Stack.** The parser's recursion is bounded by the 100 levels. Measured at
the worst 100-level form (`{a: {a: ...}}`): about 768 KiB in a debug build and
256 KiB optimized. The parser's error type is boxed for this: an unboxed
`Diagnostic` in every `Result` tripled the debug figure.

## Plan for values (S2)

### Values a run creates

```rust
enum Value<'h> {
    Null,
    Bool(bool),
    Num(f64),               // finite, never -0: every operation normalizes
    Str(Str<'h>),
    List(List<'h>),
    Map(Map<'h>),
    Host(HostRef<'h>),      // host data, read in place (below)
}
```

- **Shared, never changed.** A list, a map and a long string are behind `Rc`.
  A value cannot be changed (`01-language.md` 6.1), so sharing is never seen:
  `[...xs, x]` makes a new list that shares the elements of `xs`. `Rc`, not
  `Arc`: a run belongs to one thread (`03-host.md` 3.2.2), and what a run
  creates dies with its output.
- **Strings** hold UTF-8 text with its count of code points and a flag for
  ASCII, so length is O(1) and `s[i]` on ASCII text is O(1). UTF-8 byte order
  is code point order, so comparison and key order are byte comparisons. A
  string read from the host is `Str::Lent(&'h str)`, not copied.
- **Maps** are a vector of entries sorted by key, so key order (6.7 rule 5)
  costs nothing when it is shown, and a read is a binary search. A literal is
  built in written order, with later keys replacing earlier ones, then sorted
  once. The cost of a key is charged by the cost table, not by what this
  representation does.
- **Release does not recurse.** A program can build a list nested a million
  deep with `reduce` (building walks nothing, so no limit stops it). Dropping
  such a value by `Rc`'s own recursion would overflow the stack. The payloads
  of `List` and `Map` get a `Drop` that moves each child it owns alone onto a
  work list and drops them in a loop.
- **Walks** (`==`, rendering, the JSON text) count nesting against the depth
  limit (`02-evaluation.md`), so their recursion is bounded by it; if the
  measured stack at depth 1,000 is too large, they move to an explicit stack.

### Host data, read in place

As design `05` Part D sketches, with no derive yet:

```rust
trait Host { fn view(&self) -> View<'_>; }
enum View<'a> { Null, Bool(bool), Int(i64), UInt(u64), Float(f64),
                Str(&'a str), List(&'a dyn HostList), Map(&'a dyn HostMap),
                Opaque(&'a dyn Any, &'a str /* declared name */) }
trait HostList { fn len(&self) -> usize; fn get(&self, i: usize) -> Option<View<'_>>; }
trait HostMap  { fn len(&self) -> usize; fn get(&self, k: &str) -> Option<View<'_>>;
                 fn keys(&self, each: &mut dyn FnMut(&str)); }
trait Vars     { fn get(&self, name: &str) -> Option<View<'_>>; }
```

- A run borrows the variables for `'h`, and every `Value<'h>` may point into
  them. Nothing is copied when a run starts (`03-host.md` 3.4.2), and nothing
  host-supplied is charged to the allocation budget.
- A value is read when the program reads it. A host integer outside
  ±(2^53 − 1), a NaN or an infinity fails there with `number-range`, class
  `environment` (3.4.4, ruling T7). A host float enters whenever it is finite.
- A host map's keys are sorted when their order can be seen. Empty or not is
  `len() == 0`, bounded work (3.4.3 item 1).
- Opaque identity is the address of the lent value (`std::ptr::addr_eq`).
- The runner implements `Host` for `json::Json`, so a case's `vars` are host
  data read in place, as the suite intends.
- An `Output<'h>` owns what the run created and borrows the rest. A host that
  wants a result to outlive its variables asks for an owned copy.

### Scopes and lambdas

- Compile resolves every name once: to a variable of the environment, to a
  slot of a scope, to a library entry or host function, or to a `const`-bound
  lambda. Static rules are reported from the same walk.
- At run time a scope is a frame of slots. A lambda is never a value
  (`01-language.md` 8.1): it is called by name or passed to the library, and it
  cannot outlive the scope it was written in. So a lambda is its `NodeId` and
  a pointer to the frame it was written in, and a call pushes one frame for
  its parameters whose parent is that frame. No lambda has two calls in
  progress, since a name is not in scope in its own value.
- Frames are `Rc` so that a lambda handed to a library entry can hold its
  defining frame while the entry calls it.

### The meter

One struct per run holds steps used, units used, the limits, the deadline and
the cancellation flag. Every charge goes through it, in the order the cost
table gives, so a limit is reached at the same point as in every other
implementation. The deadline and the flag are tested at the interval of steps
that `02-evaluation.md` gives. A host function receives a handle with
`charge`, `allocate` and `cancelled`.

### The evaluator

A walk over the arena. What opens no level (a left-associative chain of
operators, the steps of a chain, the `else` of `? :` and of `if`, `const`
lines, `match` arms) is evaluated by a loop over its spine, so the evaluator's
recursion is bounded by the source nesting times the call depth, which the
depth limit bounds.

## Stack (ruling AA7)

Measured with a thread of a given stack, release build, on the deepest forms
the limits allow:

| Work | Optimized | Unoptimized |
| --- | --- | --- |
| Parse at the nesting limit (`{a: {a: ...}}`, 100 levels) | 256 KiB | 768 KiB |
| Run at depth limit 200 (walks, and calls whose bodies nest 98 levels) | 512 KiB | 1 MiB |
| Run at depth limit 1,000 | 2 MiB | 4 MiB |

A host that runs at depth 1,000 on a thread of its own gives it at least
3 MiB (optimized). The main thread's default (8 MiB on common platforms) is
enough. `tests/host.rs` runs the deepest cases on a thread of 3 MiB
(optimized) or 6 MiB (unoptimized).

## Known departures from the work bound

`02-evaluation.md` 8.5 item 1 asks for work at most a constant times the
steps. Two places are within a factor of `log n`, not a constant: a map
literal sorts its entries once when it is built, and a host map's keys are
sorted each time their order shows (a walk, a spread, `==`). Both are charged
`n` steps or more, so a program pays for each, but the sort is `n log n`.
