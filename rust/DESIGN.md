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
| `src/eval/code.rs` | — | The compiled form of the hot node kinds: a tree of closures built with the program, which the walk tries first |
| `src/eval/lib.rs` | S3 | The 53 library entries, their argument tests from the records' signatures, and the table of formulas a test holds equal to `library.json` |
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
  greatest level open inside each lambda.
- Signatures stay type text. Solving a call (13.5) binds and substitutes type
  variables in that text; `ty::substitute` gives the type with no depth test,
  and the checker tests depth where 20.4 says.
- The run of a checked program gives a failure of `null-operand`,
  `wrong-kind` or `missing-field` the class `dynamic` when a `dynamic` site is
  on the failing node or on its operand, receiver, index or argument (18.3),
  by comparing spans.
- `compile` checks by default; `compile_unchecked` is the explicit option, and
  `check::check` the check alone (`03-host.md` 3.2.1).
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
  the check first reached the hole (F12 3.12).
- **Records** (`library.rs`): a record keeps its `signature` text and `doc`
  verbatim, which completion and hover show.
- **Cases** (`testing/tools.rs`): a format, completion or hover case is read
  as a program case whose source is its text, so its environment is built by
  the same code.
