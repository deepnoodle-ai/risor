# Risor v3 in Rust: layout and plan

This crate is written from `spec/` and `suite/` only. Where the specification
was not enough, the question went to the lead, and the answer is in the
specification, not here. This file records the crate's own choices.

## Rules

- No dependencies, in the library or the tests. JSON is read by `src/json.rs`,
  which also refuses a repeated key (`03-host.md` 3.10.10).
- No `unsafe` (`#![forbid(unsafe_code)]`).
- Offsets are counted in code points. `Source` holds the text as `Vec<char>`;
  every `Range` (crate-private) is two indexes into it. The public `Span` is
  start and end as line and column (`03-host.md` 3.7.7, E.8); every span a
  host sees is one, converted at the boundary.
- **No input overflows the stack.** Recursion is allowed only where it is
  bounded by a number of the specification: the 100 levels of source nesting
  (`01-language.md` 13), the 64 brackets of type text (`05-types.md` 20.5),
  and, from S2, the depth limit. What opens no level is walked by a loop.

## The public surface

The crate root exports the surface of `05-host-api.md` Part E (ruling R201)
and nothing else. Every module is private; `lib.rs` re-exports the names,
grouped by the sections E.3 to E.8.

| Where | Holds |
| --- | --- |
| `src/api.rs` | `Expected`, `CheckResult`, `CompileOptions`; `Env::check`, `compile`, `complete`, `hover`; `Program::run`, `run_with`, `measure`, `measure_with`, `check_result`; `truthy`, `json_text` |
| `src/typ.rs` | The structural type API of R179: `Type`, `TypeKind`, `FieldType`, the constructors, `join`, `type_from_schema`, `conform`, `conform_all` |
| `src/config.rs` | `load_value`, `load::<T>`, `LoadOptions`, `LoadError` |
| `lib.rs` `__private` | Hidden: what `risor-derive` and the examples reach that is not part of the surface |
| `src/testing/` | The case-file runner, the one public module (E.2 item 2) |

- A `Type` wraps the checker's own type, so a type the host builds is the type
  the checker reads. A constructor never fails: a construction with no type
  gives an invalid type, which `err` reports and a declaration refuses with
  `bad-type`.
- A run gives `Result<Value, Error>` with one `run` diagnostic; inside the
  crate, `Program::outcome` keeps the raw failure, which the runner reads.
- Tests that reach internals are unit tests under `src/tests/`; the
  integration tests in `tests/` use the root alone.

## Layout

| Path | Stage | Holds |
| --- | --- | --- |
| `src/source.rs` | S1 | `Source`, `Range`, `Pos`, `Span` |
| `src/diag.rs` | S1 | `Diagnostic`, `Error`, `Phase`, the code names |
| `src/syntax/token.rs` | S1 | Tokens, the 26 reserved words |
| `src/syntax/lexer.rs` | S1 | The lexer. Templates nest through an explicit stack |
| `src/syntax/ast.rs` | S1 | The 30 node kinds, in an arena; F16's type lines and annotations are fields, not nodes |
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
| `src/eval/code.rs` | — | The compiled form of the hot node kinds: a tree of closures built with the program, which the walk tries first |
| `src/eval/lib.rs` | S3 | The library entries: the 53 of Core, the 6 queries of F7 and `walk` of F17; the entries of the groups (F6, F13) run in `eval/groups.rs`. The argument tests of every entry from its record's signature, and the table of formulas of all 79, which a test holds equal to the record files |
| `src/casemap.rs` | S3 | The case table, from `suite/casemap.json` |
| `src/check/ty.rs` | S4 | Types (`05-types.md` 2): Join, Fits, Merge, Truthy, Falsy, the helpers of 4.5, printing (3.9), and reading type text |
| `src/check/mod.rs` | S4 | The checker: the type rules of sections 5 to 18 over the resolution that `compile::resolve` gives, sites, classes, the expected result, and `compile_checked` |
| `src/library.rs` | S2 | The records of `suite/library.json`: names, receivers, signatures |
| `src/messages.rs` | S2 | The catalog of `suite/messages.json` and the building of a message |
| `src/bin/outcome.rs` | S2 | The outcome command of `suite/README.md` |
| `src/schema.rs` | F1 | The type of a JSON Schema (`f01-host-types.md` 3, 4): Pick, Meet, the mark *closable*, Close |
| `src/describe.rs` | F1 | Descriptions of types, signatures and environments; an environment from a description (5) |
| `src/hosttype.rs` | F1 | `HostType`, `TypeCx`, `Opaque<T>`: types from host types, and `Host` for the standard types (6.3) |
| `derive/` | F1 | The crate `risor-derive`: `#[derive(Host)]` for a struct, no dependency |

The data files of `suite/` (`messages.json`, `library.json`, `casemap.json`) are compiled into
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
the worst 100-level form (`{a: {a: ...}}`): about 611 KiB in a debug build and
83 KiB optimized (Stack, below). The parser's error type is boxed for this:
an unboxed `Diagnostic` in every `Result` tripled the debug figure.

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
  limit (`02-evaluation.md`), so their recursion is bounded by it. At the
  largest limit, 400, their stack is small beside that of calls (Stack).
- **Memory (`02-evaluation.md` 8.5 item 2).** On 64-bit targets (`Value<'h>` is
  24 bytes, `Str<'h>` is 24 bytes), the heap memory a run creates as values of
  the language is at most 208 bytes per allocation unit charged plus 0 bytes per
  step charged. The bound is derived from the layouts below, not measured; the
  test `value::layout` pins the two sizes:
  - string (`1 + floor(w/64)` units): `Rc<str>` carries 16 bytes of reference
    counts plus at most `3 * 64 = 192` UTF-8 bytes per 64 weight units (each
    4-byte scalar value has weight 2);
  - list (`1 + n` units): `Rc<ListVal>` is 16 bytes of counts, 24 bytes of `Vec`
    header, and `24 * n` bytes of `Value` slots, or up to twice that where the
    `Vec` grew by pushes;
  - map (`1 + 2n` units): `Rc<MapVal>` is 16 bytes of counts, 24 bytes of `Vec`
    header, and `48 * n` bytes of `(Str, Value)` entries (24 bytes per unit);
  - step: `Value::Num(f64)` is stored inline without heap boxing, so uncharged
    arithmetic and library steps allocate 0 value bytes on the heap.

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
  `environment` (3.4.4). A host float enters whenever it is finite.
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

**Coalesced reads.** `Ast::coalesced_reads` marks, once per program, the
last step of each coalesced read under `??` (`01-language.md` 6.6): every
operand of a `??` chain but the last, counted across the whole `??` tree,
whose node is a plain `Member` or a plain `Index` by a `String` node. The
mark depends on the nodes alone. The evaluator gives null for an absent key
at a marked step, and the checker reads a marked step on a record by the
column of `05-types.md` 9.2 for a coalesced read, which is the column
"Optional" after a plain null step. Both read the same mark, so a checked
and an unchecked run agree.

### The compiled form

`compile` also builds, for the hot node kinds, a closure per node
(`src/eval/code.rs`): literals, names, operators, conditionals, chains of
field reads, `const` lines, list literals and templates. A closure does
what the walk does for its node, in the same order and with the same
charges, but reaches its children directly; `Run::eval` tries a node's
closure first and walks the node when it has none. The walk stays the
definition of every form, and every failure is built by the walk's own
functions.

- **Stack.** A closure recurses where the walk loops (an operator chain, a
  chain's steps, an `else` chain), so a node gets a closure only when its
  subtree is at most 48 levels deep. Above that, the walk runs the node as
  before.
- **Lambdas.** For a lambda whose parameters are all plain names,
  `Program::params` holds their slots, so a call binds them without the walk
  of patterns. Arguments are moved into the slots, not copied.
- **Per-run cost.** A value is 24 bytes (an opaque value is behind `Rc`). A
  run's slots and the variables it reads share one buffer; that buffer and
  the text buffer of templates are kept by the thread from one run to the
  next, empty. A run asks the host for each variable once, as before
  (`03-host.md` 3.4.1), with two bit sets for read and missing. A library
  call of up to three arguments holds them on the stack.
- **Numbers to text.** A whole number below 2^53 is written as an integer.
  The test for a tie between two shortest texts runs only in [2^30, 2^51),
  where `suite/core-run/number-text-ties.json` places every tie; a unit test
  compares both shortcuts with the full search over 200,000 doubles.

## Stack

A run recurses on the thread that calls it. The depth count bounds the
recursion: at most `D` units of calls in progress and walk levels, then at
most 100 levels of source nesting inside the last call (`02-evaluation.md`
8.4, 8.5 item 3). The stack a run needs is about `D` times the costliest unit,
plus 100 times the costliest level of nesting. A Rust stack overflow aborts
the process, so the figures below are part of the host contract (`03-host.md`
3.9.1).

**What a host gives a run.** At least 1.5 times the measured need of the worst
form (below), in whole MiB, with room for the test to pass on two thirds of
it. A thread that Rust spawns has 2 MiB unless it asks for more (the
documented default of `std::thread`, from memory); the main thread of Linux
and macOS usually has 8 MiB.

| Depth limit | Optimized | Unoptimized |
| --- | --- | --- |
| 200, the default | 1 MiB (2.6 times the need) | 2 MiB (2.0 times) |
| 400, the largest | 2 MiB (3.0 times) | 4 MiB (2.2 times) |

**Measured.** rustc 1.100.0 (the pinned toolchain), aarch64 macOS; optimized
is the release profile and unoptimized the dev profile. The least stack, to
4 KiB, of a thread on which the work succeeds; the program is compiled before
the thread starts. The worst form is a chain of lambdas that each take a
pattern parameter (`({v}) => ...`) and call the next through `mapValues`, the
last nesting 98 map literals (`{a: ...}.a`). `sortBy` and `some` cost 0 to 4%
less; `filter` and `reduce`, 4 to 17% less.

| Work | Optimized | Unoptimized |
| --- | --- | --- |
| Parse at the nesting limit (`{a: {a: ...}}`, 100 levels) | 83 KiB | 611 KiB |
| Run the worst form at depth limit 200 | 387 KiB | 1,043 KiB |
| Run the worst form at depth limit 400 | 691 KiB | 1,859 KiB |

The cost of one unit: the stack of a chain at the limit divided by the limit,
optimized at depth 400 and unoptimized at depth 200, so it carries a share of
the fixed cost of a run. The last row is the frames between one level and the
next, read with lldb.

| Unit | Optimized | Unoptimized |
| --- | --- | --- |
| A call through `mapValues` of a lambda with a pattern parameter | 1.5 KB | 4.2 KB |
| A call through `sortBy` or `some` of a lambda with a pattern parameter | 1.5 KB | 4.0 KB |
| A call through `flatMap` of a lambda by name | 1.3 KB | 3.4 KB |
| A call through `map` of a lambda by name | 1.2 KB | 3.0 KB |
| A call by name | 0.8 KB | 1.4 KB |
| A level of map literal (`{a: ...}.a`) inside a lambda, by frames | 1.0 KB | 2.3 KB |

Under x86_64 macOS (Rosetta, the same compiler), these forms used 0 to 4%
less stack than on aarch64. CI runs on x86_64 Linux; its figures are not
measured here.

**How the evaluator keeps it small.** An unoptimized build gives every local
and every temporary a slot of its own, and each `?` adds two. A frame that is
live while a lambda runs, or while the walk recurses, pays for all of them at
every level. So such a frame holds only its loop:

- each library entry that calls a lambda is a function of its own (`map`,
  `filter`, the entries of `list_each`, `reduce`, the queries, `mapValues`,
  `walk`);
  what the entry does after its loop, and every entry that calls no lambda,
  is in another function;
- a failure, a rare kind and a lambda with patterns or defaults go to
  functions of their own, marked `#[inline(never)]` or `#[cold]`;
- receivers are passed by reference, and the arguments of a library call are
  written to the caller's buffer, not handed to a closure;
- a chain's closure calls a method step or an index step directly, and the
  arms of a frame that stays live end in one result, not one `?` each;
- the small steps of each element (its two steps and read, the depth test
  and the binding of plain parameters, `filter`'s keep) are inlined only
  when `debug_assertions` is off. Inlined unoptimized, they added about
  1.5 KB to each unit; called optimized, they cost 4 to 20% more
  instructions on the benchmark programs that call a lambda for each of
  10,000 elements (counted with `/usr/bin/time -l`). `debug_assertions`
  stands for the build: an optimized build with debug assertions on gets
  the calls, and an unoptimized one with them off gets bigger frames than
  these figures.

**The test.** `the_deepest_programs_run_within_the_stated_stack`
(`src/tests/host.rs`) runs, at depth limits 200 and 400: walks to the limit
and one level past it; calls by name with the nesting limit before the first
and inside the last; and chains through `map` (a lambda by name and an inline
lambda), `flatMap`, and `filter`, `some`, `sortBy`, `mapValues` and `reduce`
with pattern parameters, each with 98 map literals inside the last call, then
one link past the limit. It also runs `walk` (F17): a chain of nodes whose
deepest callback runs at the limit, with the nesting limit in its body, and
one node past it; data nested to the limit, and one level past; and walks
inside walks, each link calling the one before from its walk's callback, to
the limit and one link past. It runs each limit on a thread of two thirds of the
stated figure, so the figure has a margin of 1.5. CI runs it unoptimized and
optimized. Unoptimized at depth 200, the test passes on 13/16 of that thread
and overflows on 12/16 (aarch64 and x86_64 macOS).

## Known departures from the work bound

`02-evaluation.md` 8.5 item 1 asks for work at most a constant times the
steps. Two places are within a factor of `log n`, not a constant: a map
literal sorts its entries once when it is built, and a host map's keys are
sorted each time their order shows (a walk, a spread, `==`). Both are charged
`n` steps or more, so a program pays for each, but the sort is `n log n`.

## The library and the work bound

| Work | How | Why it meets the bound |
| --- | --- | --- |
| Searches (`in`, `split`, `replaceAll`, `startsWith`, `endsWith`, `removePrefix`, `removeSuffix`) | The standard library's `str` searches. A needle longer than the text is answered from the lengths first | The standard library searches a `&str` pattern with the Two-Way algorithm, which is linear (inference from its documentation; `tests/library.rs` times a pattern that a naive search takes quadratic time on) |
| `sort`, `sortBy` | The standard library's stable `sort_by` | At most a constant times `n log n` comparisons; a comparison of two strings reads their common prefix, which the charge's `sz` term pays for |
| `join`, `repeat`, `padStart`, `padEnd`, `replaceAll` | The result's weight is found from the arguments, charged, then the text is built once | Building is linear in the result |
| String arguments | Read in place, never copied before the charge; `padStart` reads only the part of `fill` it uses | An argument the charge does not count is not walked |
| `unique`, `groupBy`, `toMap` | Hash sets and maps over the values or keys | Expected work linear in `n` and the keys' weights; building the result map sorts its keys (the `n log n` exception of `02-evaluation.md` 8.5) |
| Case mapping | `suite/casemap.json`, never `char::to_uppercase` | The table is Unicode 16.0.0 whatever the platform's version |
| `fixed` | Its own exact decimal rounding of the double | Rust's `{:.N}` formatting rounds a tie half to even (from memory), where `fixed` rounds it away from zero |

## The checker (S4)

- `compile::resolve` applies the static rules and gives every binding a slot.
  The checker keeps a type for each slot, so scope needs no structure of its
  own: two bindings of one name are two slots, and a fact never reaches a
  shadowing binding (10.4).
- A fact is a path (a slot or a variable, then field keys). The facts in
  force are a list that grows and shrinks with the walk.
- A use of a `const`-bound lambda checks its body again with the use's
  parameter types, under the facts where the lambda was written. The limits
  of 20.2 and 20.3: the parser records the
  greatest level open inside each lambda. A lambda that F16 checks once is
  the exception (below).
- **A lambda's own name** (`01-language.md` 5 rule 6, `05-types.md` 22.5).
  The resolver binds the name of a `const`-bound lambda before it walks the
  lambda, and keeps a stack of the lambdas whose text it is in. A call of
  one of them, or its name given as a lambda argument, resolves to
  `Res::SelfRef`, not to a call or a lambda reference: the checker counts no
  use of it, a call there has type `never` with its plain arguments
  synthesized, and a lambda argument there is not checked. The first such
  reference of each lambda, by start, is `recursion-needs-types`, reported
  after the walk.
- Signatures stay type text. Solving a call (13.5) binds and substitutes type
  variables in that text; `ty::substitute` gives the type with no depth test,
  and the checker tests depth where 20.4 says.
- The run of a checked program gives a failure of `null-operand`,
  `wrong-kind` or `missing-field` the class `dynamic` when a `dynamic` site is
  on the failing node or on its operand, receiver, index or argument (18.3),
  by comparing spans.
- `Env::compile` checks by default; `CompileOptions { check: false, .. }` is
  the explicit option, and `Env::check` the check alone (`03-host.md`
  3.2.1). Inside the crate they are `check::compile_checked`,
  `compile::compile_unchecked` and `check::check`.
- Stack: the checker's recursion follows the source nesting and the nested
  uses of lambdas, both bounded at 100. Measured at the worst: 256 KiB
  optimized, 768 KiB unoptimized, within the figures for running above.

## Package F3, string forms

- The lexer reads raw strings in its word rule and text blocks at `"""`. A
  block's content is held as positioned pieces (code points, escapes, line
  ends, interpolations) until its closing delimiter, where the indentation is
  known; then its value is built and the text parts are written into the
  tokens already emitted. Positions are never computed from the split lines.
- `text-block-open`, `text-block-close` and `text-block-indent` travel on the
  literal's token, and the parser reports one where it reaches the token
  (F3 6.2). They are not lexical.
- A raw string or a block with no interpolation is a `String` node: its value
  is fixed at parse time and costs nothing at run time.

## Call handles

A call handle is valid only until its host function returns, and holds by
construction here: `Call` borrows the run's meter mutably for the duration of
one call, and a host implementation is a `'static` closure that receives
`&mut Call<'_>`, so it cannot keep the handle past its return.

## Host errors

A host function returns `Result<Data, HostError>`. `HostError` is an enum of
two variants, so the binding tells a Risor failure from any other error by
its type, which `03-host.md` 3.5.3 item 5 leaves to the binding:

| Variant | Made by | The run's `function-error` |
| --- | --- | --- |
| `Text(String)` | `"no".into()`, or `HostError::Text` | The text is the message. No cause |
| `Risor(Box<Error>)` | `?` on any `Result<_, risor::Error>`, such as a nested `parse`, `compile` or `run` | The message of the error's first diagnostic, the inner diagnostic, whole. The inner diagnostic is `Diagnostic::cause` |

The cause is a public field, `cause: Option<Box<Diagnostic>>`, named as the
specification names it (3.7.1). `std::error::Error::source` gives the same
diagnostic, on `Diagnostic` and on `Error`, so a chain of nested runs walks
with the standard trait. The renderings never write it (`f08-errors.md` 6).
A plain text error has no cause: its text is already the message.

## Package F1, host types

- **Shared types.** A type's element and record slots are `Arc`, not `Box`,
  so a type is cheap to clone and parts are shared. The mapping of a schema
  maps each `$ref` target once, by its place in the document, and reuses the
  type (3.10 item 5). Close, the depth of a type and the walk of its opaque
  names visit each shared part once, keyed by its address. A unit test maps
  a doubling chain of 30 definitions, which as a tree has 2^30 nodes.
- **The mark.** `Rec` has a field `closable`. Only the mapping sets it, under
  `closed`; Join keeps it when both records have it; Close drops it. Every
  other type has it `false`, so the checker's equality is unchanged.
- **Walk level.** The mapping stops at a schema 65 levels below the root,
  since the root is then deeper than 64. This bounds the recursion before a
  depth is known. Literal values are cut at the same level.
- **Signatures printed.** A type variable is held, for printing only, as an
  opaque name that starts with U+10FFFF: it sorts after every opaque name and
  is printed without the mark.
- **Descriptions.** `Env::describe` gives a `Json` with sorted keys;
  `json::to_text` gives the text of 5.3 item 5. `Env::from_description`
  returns `DescriptionError::Form` for a description of the wrong form (the
  binding's own error) and `DescriptionError::Declare` for `declare` errors.
  This build has no removed library entries, so `removed` is always `[]`, and
  a description that names one is a form error.
- **Host types.** `HostType` gives the type; `Host` and `HostMap` present the
  value in place, and the two agree. The derive lives in `derive/` so that
  this crate keeps no dependency; it is a workspace member, built only with
  `-p risor-derive` or `--workspace`.

## Round 13

- **Removed entries.** `EnvData::removed` holds record indices. Every lookup
  of a library entry by a compile, a check or a run goes through
  `Env::lib_function` and `Env::lib_method`, which skip them; the `value`
  site of a method on an `any` receiver (`05-types.md` 13.3) skips them too.
- **Child environments** (`03-host.md` 3.3.2): `Env::child()` gives a builder
  whose declarations replace the parent's of the same name; removed entries
  accumulate; rejected classes and limits are the parent's unless given.
- **The parameter list** (`01-language.md` 3.2 rule 6): the table of matching closers treats a
  template with its interpolations as one token; each interpolation matches
  its own openers. A template where a pattern must stand is
  `unexpected-token` over the whole template.
- **Type size** (`05-types.md` 20.7): `Rec` keeps its size in a
  `SizeCache` (a `OnceLock`, emptied by a clone and ignored by equality), so
  `Ty::size` is constant work for each level of list nesting and saturates
  just above 100,000. The checker tests it at the nodes 20.7 lists
  (`size_check`, and inside `depth_check` before the depth); a declared type,
  a schema's type and a host type over it are `bad-type`.

## Package F2, templates and analysis

- **Template parse** (`src/template.rs`): the text is read left to right;
  each part is lexed by the Core lexer from after `${{` (`lex_part`), which
  counts the part's `{` tokens and stops at the closing brace. Every part is
  lexed before any is parsed, so a lexical error anywhere comes first. The
  parts' tokens are joined, each ended by an `Eof` with the span of `}}` (or
  the lone `}`, then an `Eof`), and `parse_parts` parses each as one `Expr`
  with the `${{` opening a level. The matching table of 3.2 rule 6 restarts
  at each part's end. `Template::program(mode)` adds the `Template` or
  `String` node and the `Program` node to a copy of the arena.
- **Analysis** (`src/analysis.rs`): one walk with an explicit task stack and
  a stack of scopes, so neither a long chain nor a long operator chain uses
  host stack. A chain is walked from its last step down to its primary; the
  path runs from the primary outward and stops at a parenthesis, a method
  call or an index that is not a `String` node.
- **The runner**: `PACKAGES` in `lib.rs` is the one list of packages offered;
  `testing::skipped_by_package` leaves out the `until-fNN-` files of those
  packages (`03-host.md` 3.11 item 4).

## Package F4, checked conversion

- **Parsing** (`src/syntax/parser.rs`): `as` and `is` stand in the loop of
  level 6; a program type is its own small tree (`PType`, `PAlt`, `PPrim` in
  `ast.rs`), kept with its spans for the static rules. `bad-type-text` is
  the parser's own helper.
- **Names** (`check::ty::Names`, `TypeNames`): type text is resolved
  against an environment's opaque types and named types; a named type's
  type is held once and shared, and the size limit is tested on it.
- **Static rules** (`compile.rs`): each `As` node's program type is
  resolved when names are, in every compile, into `Res::As`; names, then
  two records in one `PType`, then depth and size.
- **The walk** (`src/conform.rs`) is one function over a `WalkMeter`: the run
  charges steps and counts depth through it, and the conformance function
  (`Env::ty(text)?.conform(&data)`) uses one that counts nothing.
- **Facts of `is`** (`check/mod.rs`): a fact is a path with a kind,
  `NonNull`, `Is(T)` or `IsNot(T)`; `||` joins one `Is` fact per side about
  one path into one.

## Package F5, tagged unions

- **Representation** (`check/ty.rs`): `Ty::map` still holds one record; for
  a union it holds flat(U), so every read, Merge, spread and pattern sees
  the record `f05-unions.md` 4.3 names with no change. `Ty::variants` holds
  the union's variants, in the order of printing. Size, printing, Join,
  Fits, Truthy, disjoint and without read the variants; `union_of_alts`
  forms a union from the records of one `Type`, in type text, program types
  and schema branches alike.
- **Facts** (`check/mod.rs`): a test of a field gives facts of the kinds of
  `f04-conversion.md` 6.4, marked as from a field, when the receiver's map
  alternative is a union or a field fact narrowed it there; `match` on a
  field gives them arm by arm; `||` unites.
- **The walk** (`conform.rs`) reads the tag first, then tests the variant it
  selects without the tag.
- **Schemas** (`schema.rs`): branches whose records form a union give it;
  Pick meets a record with a union variant by variant; Close closes each
  variant.

## Package F6, opt-in groups

- **Records** (`library.rs`): `suite/library-groups.json` loads beside
  `library.json`; a function record keeps its full name (`json.parse`) and
  its group, and a constant is a record of its own. `Env::lib_function`
  gives a group's entry only while its group is on; a group that is on is a
  namespace, and resolution makes `json.parse(s)` a library call and
  `math.pi` a constant (`Res::Constant`, read at no cost).
- **The entries** (`eval/groups.rs`) are written by the chapter's rules: a
  JSON scanner of the grammar of 6.2.1 with an explicit stack that decides
  `not-json` over the whole text before any number or depth failure; the
  proleptic Gregorian calendar of 7.1 with exact decimal arithmetic for
  fractions and durations; RFC 4648 with the strict rules of 9.2. Decimal
  text is turned into a double by the standard library's `str::parse`,
  which is correctly rounded for any count of digits, as Core already uses
  it. `json.stringify` is `string`'s rendering walk, with its own template
  for an opaque value.

## Package F7, queries

- **Syntax** (`parser.rs`): a `[` or `?.[` followed by `*` begins a
  projection, parsed to an `Each` whose reads hang off an internal `Elem`
  node, the element in hand. A later star is the last read of its level.
- **Evaluation** treats an `Each` as a step of its chain: one loop per
  level, recursion once per later star, a scratch list charged once at the
  end. The checker walks a projection with *skipped-element* and *failed*.
- **Library** (`library.rs`): `suite/library-queries.json` loads after the
  others; `sortBy` replaces the Core record and keeps its cases, and the
  six new entries are methods. The three templates of
  `suite/messages-queries.json` join their codes in the catalog.


## Package F8, errors, second part

- **Targeted parse codes** (`parser.rs`): the parser finds the Core
  diagnostic as before and records, with it, the rule of Core that gave it
  (`Rule`). A few marks on tokens (map, map pattern, index step, paren, `if`
  test, operand word, lambda arrow) and the token where the last
  expression, `if`, program type, `const` pattern and parameter ended answer
  the conditions of 2.3 to 2.7. `finish` maps the diagnostic by the rows in
  order and builds its message; `parse_core` gives the Core diagnostic
  unmapped. Rows A1 to A5 are lexical and the lexer gives them.
- **Messages**: `messages.rs` reads `suite/messages-parse.json` and
  `suite/messages-check.json` with the other catalog files. The text at a
  span chooses a lexical template (`syntax/message.rs`); the checker and the
  resolver name the template and print each type with the 3.9 printer. A
  text parameter reads only the first 65 code points of its span.
- **Trace**: built on the failure path only. A call by name and a library
  entry's call of a lambda each add their entry as a failure leaves them;
  the diagnostic reverses the list.
- **Renderings** (`render.rs`) work on a `View` of a diagnostic, so the
  `render` case renders diagnostics it reads from JSON. `ConformAll` is a
  second walk in `conform.rs`, with no charges, that stops at the maximum.

## Package F9, testing for authors

- **The runner is the helper** (`testing/helper.rs`): `load(path, base)` and
  `from_text(text, program, base)` read a file against its base, and its
  cases run in a child of that base. A verdict carries each difference's E
  and F in the JSON form of `f09-testing.md` 6.2, so the report is the
  differences written out.
- **The fixture test** runs after the program compiles and before the run,
  in each run that gets there; both give the same answer.
- **Update mode** (`testing/update.rs`) captures the outcome of the checked
  run, builds the new `expect` by shape, and splices it into the text with a
  scanner of byte offsets: every other byte is kept.
- **Program case files** (`testing/program.rs`) are checked and run in the
  default environment, and their value read as JSON data.

## Package F10, config files

- An expected result by schema maps it with `closed` and checks as with a
  type; the runner's step 5a applies the conformance function after a run.
- `config.rs` makes the steps of a load and reports the step that failed;
  the value is copied into `Data`.
- `decode.rs`: the `Decode` trait. A decode walks every failure against the
  type derived from the host type, then copies; it fails beyond that only
  where the host type is narrower. The derive implements `Decode` for a
  struct that borrows nothing, with the mark `#[risor(default)]`.

## Package F13, math functions

- `math.rs` has its own big integers and computes each function in fixed
  point with an explicit error bound. The method of Ziv doubles the
  precision until both ends of the interval round to one double. The exact
  and midpoint results of `pow` are found first, by exact arithmetic on the
  odd part of the base. No platform function decides a result; one is used
  only to estimate a range with a wide margin.
- That exact path costs microseconds, so it is the second phase. The first,
  `math/fast.rs`, evaluates in double-double arithmetic (`two_sum`, and
  exact products with `mul_add`) with an error bound proved in a comment for
  each function, and returns when both ends of a much wider interval round
  to one double; otherwise the exact path decides. exp: 2^(i/64) ·
  2^(j/4096) tables and a short series; log: a table of 512 reciprocals and
  a log1p series; sin, cos, tan: Payne–Hanek reduction with 256 bits of 2/π,
  then sin and cos of j/128 and short series; atan and the rest: atan(j/128)
  and a short series; pow: the exact cases cheap to find (integer powers by
  `u128`, √x by `sqrt`), then exp(y log x). A call costs 10 to 45 ns.
- The tables (`math/tables.rs`) are literals, generated by `math/gen.rs`
  from the exact kernels; a test generates them again and compares.
  `math/fast/tests.rs` compares the fast phase with the exact path: a short
  run in `cargo test`, and an ignored long run of 1,000,000 arguments per
  function and the edge sets.

## Package F14, host facts

- **Declaration** (`env.rs`): `EnvBuilder::some_not_null` keeps the names;
  `build` tests them against every variable of the environment, a parent's
  among them, and keeps each distinct set once, its members in code point
  order, as variable indices. A child drops a parent's host fact that names
  a variable or function the child declares, whole. `Env::with` is
  `Env::child`.
- **The checker** (`check/mod.rs`): `FactKind::Null` is null(p), given only
  for a member path (an `Ident` that resolves to a member variable). `??`
  carries N of its left operand along a chain, as `&&` and `||` carry their
  facts. The effect is applied in `ident`, after the facts of Core, F4 and
  F5: some host fact of the variable has every other member under a null
  fact in force.

## Package F17, guards and `walk`

- **Guards** (`syntax/parser.rs`, `compile.rs`, `check/mod.rs`,
  `eval/mod.rs`): `Arm` has `guard: Option<NodeId>`. The parser reads
  `if (e)` after the literals or the `_`, and its `(` opens a level. The
  resolver and the checker keep the literals of earlier plain arms apart
  from those of the arm itself: a literal is compared with both, and only a
  plain arm adds its own to the first. So a guarded arm neither covers nor
  makes a later literal `duplicate-case`, and only a plain `_` makes the
  arms after it unreachable. In the checker a guarded arm gives later arms
  none of its facts; its guard is a tested position under the arm's facts,
  and its true facts join them for the result.
- **`walk`** (`eval/lib.rs`, `lib_walk`): the record is loaded from
  `suite/library-recursion.json` after those of F7, so its index is the
  last. The walk keeps a stack of its own, one entry for each list of
  children entered, each a list and the position of its next element, so a
  level of the walk is a loop turn and not a host call. The depth of a node
  is the length of that stack. Each call of the callback goes through
  `call2`, as `map`'s does, with the measure of the call plus the node's
  depth, and the node's place in pre-order as the trace index.
- **A value at a callback parameter.** `ArgTest::Callback` fails an argument
  that is a value with `wrong-kind`, template `argument`, among the tests of
  2.6 in their order (F17 5.5 item 3). Only a call of a function by name
  reaches it: a method's shape test comes first.
- **The seed rule** (`check/mod.rs`, `call`): it fires when B does not fit
  R with the `Var` replaced by A0, or fits it only by any; the new solution
  joins A0 with what `bind(R, B)` gathers for the `Var` (F17 5.6). For
  `reduce`, whose R is the `Var`, this is the rule as before.
- **The runner**: `PACKAGES` holds 17 before 12, so a file named
  `until-f17-` is not run; `recursion_package` runs
  `suite/recursion/`, its `tools-` files among them, and
  `library_recursion_cases` the record's cases.

## Package F16, slice D1: annotations and type lines

- **Nodes** (`syntax/ast.rs`): no node kind is added. `Program.types` holds
  the type lines (`TypeLine`: name, span of the name, `PType`, span), which
  are not nodes; `Const.ty`, `Lambda.result` and `Param.ty` hold a `PType`,
  and `Param.fn_` a `PFn` (its words and types, and its result, `None` for
  `unknown`). A walker that orders a program's lines merges `consts` and
  `types` by start.
- **Parsing** (`syntax/parser.rs`): `type` begins a type line only in the
  loop of a program's lines, when an upper-case word and `=` follow it; a
  block and a branch never look for one, so `type` there stays a name and E17
  reports it. A `(` begins a lambda when its partner is followed by `=>` or
  `:`. After the `:` of a parameter that is a name, `(` followed by `)`, or by
  a word and `:`, begins a `PFn`; it takes no default, so a `=` after it is
  the loop's `unexpected-token`. E9 keeps only its third case. In a repair
  (`parse_repair` sets `limit`), these choices read only the tokens before
  the end, and a `(` with no partner followed by a name or `_` and `:` begins
  a parameter list (F16 7.2 items 3, 4 and 7). There `type` and one word that
  ends those tokens begin a type line whatever the word's first letter, and
  also when the word is the hole with an empty prefix (R242).
- **Static rules** (`compile.rs`): the lines of a program are walked in the
  order of the text. A type name resolves against the environment's named
  and opaque types, then the type lines that declared so far
  (`Resolver::type_lines`). A later line of one name whose type fails
  removes the earlier line's entry: it stands and declares nothing (R240).
  Each annotation is resolved once, where it
  stands, and kept in `Resolution::annots` by the start of its span, `None`
  when a static rule refused it. The resolver also keeps each type name that
  resolved, and each declaring type line's name, with the type and the
  declaration (`TypeRef`), for hover.
- **The checker** (`check/mod.rs`): `fit_strict` is 15.1's fit through the
  form with `strict` set: a fit that uses `any` is followed to its places of
  failure and reported there, a spread with the types of its first key, and
  `maybe-null` only for a value that surely fits without null. A
  `const`-bound lambda whose parameters are all annotated, with at least one
  annotation, is checked once at its `const` line (`check_once`): one count
  against 20.3, its nesting where it is written against 20.2, its sites kept,
  and its result type kept in `once_result`. A use of it fits each plain
  argument (`fit_args`) and each value the library passes (`fit_passed`) and
  takes the result type; its body is not checked again. A lambda that is
  partly annotated is checked at each use as before, with each annotated
  parameter bound to its annotation and the result annotation fitted. The
  narrowing of `filter` and `find` reads Q, as before, not the annotation.
- **Evaluation**: nothing. A run never reads an annotation or a type line.
- **Tools**: the formatter merges type lines into the program's lines and
  prints every annotation token as a kept token; a `PFn` is one line.
  Completion is in the types context when the hole is in any annotation, a
  `PFn` or a type line's type, with the names of the type lines that end
  before the hole, less the line that holds the hole: the repair's `;` ends
  that line at the hole too (R243). Hover answers a word that is a `TypeRef`
  first, with the kind `type`. A repaired hover on a type line's name gives
  nothing, since the cut is at the word's end and the repair wrote the type
  (F16 7.3 item 5).
- **The runner**: `PACKAGES` holds 16 after 14 and before 17. The files
  named `until-f16-` were deleted when all three implementations offered the
  slice (`03-host.md` 3.11 item 5). `entry_points_package` runs
  `suite/entry-points/`, its `tools-` files among them;
  `catalog_entry_points_cases` the cases of
  `suite/messages-entry-points.json`, whose entries add `name-clash` and the
  template `fn-value` of `arg-mismatch` to the catalog. In that directory
  `name-clash`, `untagged-union` and `check-limit` are static codes beside
  Core's twelve. The place of D1 before F17 is the recommendation of R211,
  not yet confirmed (F16 1.5).

## Package F12, tools

- **Formatter** (`src/format.rs`): documents live in an arena (`Docs`), with
  the forced-break flag of each part computed when it is made. A document
  may be shared: the arguments of a call with a hugged last argument are
  built once and used by all three alternatives, so nesting costs no more.
  The layout and the test of fits use explicit stacks. Which tokens are kept
  is known only after the document is built, so the builder runs twice;
  comments are attached between the runs. Tokens are found from the spans of
  the nodes around them; a template with its interpolations is one token.
- **Assist** (`src/assist.rs`): the lexer has a cut mode that stops where the
  text ends inside an interpolation and reports which are open; the repair
  appends a closing tail for each after the inserted tokens. The hole is the
  word `parser::HOLE` (U+10FFFF), which the parser reads as a name, a word or
  a type name; it resolves to nothing, so the checker gives it `never`. The
  repair inserts one token, re-parses, and keeps it when the parser stands
  past it; `parse_repair` matches no bracket among the inserted tokens.
  `check::check_probe` gives the types of the nodes and of the slots where
  the check first reached the hole (F12 3.12). The resolver marks an `Ident`
  that names a `const`-bound lambda where a value is needed
  (`Res::NotValue`), as it marks a lambda argument (`Res::LambdaRef`), so a
  hover on either gives the kind `lambda` with no type (R245).
- **Records** (`library.rs`): a record keeps its `signature` text and `doc`
  verbatim, which completion and hover show.
- **Cases** (`testing/tools.rs`): a format, completion or hover case is read
  as a program case whose source is its text, so its environment is built by
  the same code. Read for the outcome command, such a case may have no
  `expect`; update mode does not write one for it.
