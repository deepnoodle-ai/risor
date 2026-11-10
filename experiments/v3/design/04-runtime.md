# Risor v3 design 04: the evaluator and runtime

Status: design proposal, one of six, revised for `00-overview.md`. Rulings are
cited by their numbers there (C1, L3, N2). The only code written is the set of
small benchmark packages under `experiments/v3/lab/`.

- **Part A** is what the specification requires of any evaluator. It contains no
  Go. **Part B** is the Go implementation, one implementation's choices. **Part
  C** is notes for a TypeScript implementation and **Part D** for a Rust one.
- **(M)** is a number I measured, with its table. **(I)** is an inference or an
  estimate that I did not run. All of Part C is (I), and all of Part D but the
  text-size figures.
- **Fourth revision.** D28: the absent value is null. Go's own `nil` keeps its
  name in Part B. D34: a function literal (a lambda, in `02`'s word) is passed
  only to a library function or method, and a parameter cannot be called. A.9
  says what an evaluator may rely on, and B.5 is rewritten for it. No benchmark
  was run again. B.5 marks what is still measured and what is now inferred.
- **Fifth revision.** D36: a binding is written `const`. D35: `if`, `else if`
  and `else` form an expression. It costs what `? :` costs (A.3), its brackets
  nest as any bracket does (A.6), and it compiles to the closure of `? :`
  (B.5).
- **Sixth revision.** Checked conversion is in 3.0 (overview 2.11, D39 to D45,
  `07-checked-conversion.md`). `x as T` and `x is T` test a value against a
  type at run time. A.3 fixes the steps of the walk, A.4 and A.6 its allocation
  and depth, and B.14 says how the Go implementation compiles a type to a test.
  None of it was built or measured.
- **Seventh revision.** Plans `08` to `13` are in 3.0 (D46 to D51). A decision
  of a plan is cited by its own number (`08-8`, `10-3`). `try { } catch { }`
  replaces the call form of `try`, and `catch` decides by the code of a failure
  (A.1). A.3 and A.4 give the exact cost of each new form: a string literal of
  every form, a projection `[*]`, the walk of a tagged union, a caught failure
  and its error value, and `fail` with data. A.10 is new: how a failure
  unwinds, what it may hold, and how its trace is built at no cost to a run
  that does not fail. B.10 has the Go design for `Try` and the trace, and B.15
  for the new node `Each`. A host number that cannot enter the language has the
  code `number-range` and the class `environment` (A.2). None of it was built
  or measured, and no benchmark was run again.
- `spike:` is `experiments/v3/spike/spike.go`. `expr:` is
  `~/git/deepnoodle/expr` at v1.4.1. `v2:` is this repository's `pkg/`.
- Measurements: Apple M4 Max, Go 1.26.2, darwin/arm64, median of 6 or 8 runs.
  Other work was running on the machine, so the numbers are indicative. The lab
  packages (`valuerep`, `hostread`, `hostcall`, `lambda`, `budget`, `errors`,
  `runstate`, `alloc`, `textsize`, `counts`) run hand-built closure trees, not a
  compiler's output, and each has an agreement test. Each table comes from:

```bash
cd experiments/v3
GOWORK=off go test ./lab/<package> -run xxx -bench . -benchmem \
  -benchtime 200ms -count 6 | GOWORK=off go run ./lab/median
```

## 1. Summary

1. **The cost model is part of the language (L2, L3, L6).** Two short tables
   define steps and allocation units over the program as written (A.3, A.4).
   Every implementation charges the same numbers at the same points.
2. **Allocation is charged in units, before the memory is taken (L3).** A list
   costs 1 + its length, a map 1 + twice its entries, a string 1 + its size. The
   exact size is found first. This costs 0% to 5% on the hot path, and 29% on a
   join that must measure its pieces (M, B.7).
3. **Text is measured in blocks of 64 units of weight (L7).** Weight is the
   number of code points, with one above U+FFFF counted twice. That is the
   UTF-16 length, so JavaScript has it for free. Go counts it at 0.05 to 0.11 ns
   per byte and never counts a string under 64 bytes (M, B.8).
4. **Fixed counts cost almost nothing at run time (L6).** Folding, fused paths
   and set lookups that charge what the unoptimized evaluation charges measured
   the same as ones that do not, but for 1.1 ns in one case. The real cost is
   that stopping early is forbidden and the cost table is frozen (M, B.9).
5. **The number range check is free (N2).** On typed integer arithmetic it
   measured no slower than no check, and faster than the 64-bit overflow check
   it replaces (16.0 ns against 19.4 ns) (M, B.2).
6. **Go: closures over the AST, values as plain `any`, typed closures as a
   second layer.** A tagged value struct is 1.5x faster on arithmetic and 29%
   slower on reads. Typed closures are 1.3x to 1.7x faster on arithmetic, and
   make a struct field read 4.6 ns with no allocation, against 13 to 17 ns with
   one (M, B.2, B.3).
7. **Go: errors are returned, not panicked.** The success path was never slower,
   and a failure costs 38 ns in place of 184 ns (M, B.10). The trace of a
   failure is written on that error path, so a run that does not fail does no
   work for it (I, A.10, B.10).
8. **Go: struct fields resolved at compile time, adapters for host functions,
   one fixed slot per binding with no frames, pooled run state.** A fused
   four-field path is 11 ns against 24 ns through maps. An adapter costs what
   native code costs, and reflection 15x more. A pooled run is 24 ns against 71
   ns (M, B.3 to B.11). A function literal run in place was 1.8x faster than
   one called through a pooled frame (M, before D34). D34 lets every function
   literal run in place and removes frames, which is derived and not measured
   (I, A.9, B.5).
9. **The step counter counts down in chunks of 1,024, and the deadline is
   checked when a chunk runs out (L5):** 0.25 ns per operation, and 0.1 ns per
   step more when the check reads the clock (M, B.6).
10. **The reference evaluator is about 2,130 lines of Go, of which about 1,330
    are language semantics** that port line for line (I, B.1). D46 to D51 added
    about 230 of them.
11. **A type test (`as`, `is`) charges 1 step for each value it tests, creates
    no value, and enters at most 64 levels (`07`).** A place whose type is `any`
    costs nothing. The count is exact and does not depend on a map's order
    (A.3). A tagged union is tested by its tag first and then by one variant,
    so a value that conforms costs what it costs against that variant alone
    (`09`, A.3). A type compiles to a test once, and the walk reads host values
    in place (I, B.14). Nothing was measured.
12. **`catch` decides by the code of a failure (D48).** A limit, a
    cancellation, a crash and `missing-variable` pass through (A.1). A `try`
    costs nothing until it catches. `catch { }` costs nothing then either.
    `catch (err)` costs a map of four entries and the size of the message:
    9 units, and 1 step and 1 unit more for a short message built from the
    catalog (A.10).
13. **A failure is not a value, and what it holds is bounded by the limits and
    not by the data:** a path of at most 64 steps and a trace of at most D +
    100 entries. The trace charges nothing and is written only as the failure
    leaves each call (A.10).
14. **A string literal of any form costs nothing in a run (D46).** A projection
    charges 1 step for each element a star visits, each read as it is charged
    today, and 1 + `k` units for its one flat result (D51, A.3, A.4). It
    compiles to one closure that fills a scratch buffer and allocates once (I,
    B.15).

## Part A. What the specification requires of any evaluator

### A.1 Evaluation order and errors

| Rule | Detail |
| --- | --- |
| Order | Operands are evaluated left to right, before their operator. A receiver is evaluated before its arguments, and arguments left to right. `&&`, `\|\|`, `??`, `? :`, `if` and `try` evaluate only the operands they need. An `if` evaluates its tests in order up to the first truthy one, then the one branch taken. A `try` evaluates its body, and its handler only when the body fails with a failure that `catch` catches (`10` 2.2) |
| Optional chains (B4) | When `?.` or `?.[` finds null, the rest of its chain is not evaluated and not charged |
| A projection (D51, `13` 5.2) | The receiver of the first star is evaluated once. The reads after a star are applied to each element in turn, by index, each read with its index expression, and one element is finished before the next is started. A later star iterates the list the reads gave for that element. Inside a projection a `?.` or `?.[` that finds null skips the rest of the reads and stars for that element only: the element gives one null, which is kept, and the next element goes on. `x?.[*]` on a null `x` at the first star makes the whole chain null, as B4 has it |
| `x as T` and `x is T` (`07`) | `x` is evaluated, then tested against `T` by the walk of `07` 2.3: a list by index, a record's declared fields in code point order of their keys, then its other keys. `as` gives back the value it was given, or fails with `not-conforming`, class `value`, which `catch` catches. `is` gives a bool and raises nothing but a limit. The type is not evaluated: it is text of the program |
| Charge before work | An operation charges its steps, then its allocation units, then does its work. If a charge fails, the work is not done and the run ends |
| Which failure wins | The first failure raised in evaluation order. The outcome of a run is a value or one failure, and a failure has one code |
| How `catch` decides (D48, `10` 2.4) | By the code of the failure, and by nothing else. Not by its class: whether a null, a wrong kind or a missing field is `dynamic` or `environment` depends on the checker's sites (`03` 2.14), which an evaluator does not have, and a program compiled with no check has none |
| Failures `catch` catches | Every run failure whose code is not in the next row. Among them: `not-conforming`, a failed conversion, `failed` (the code of `fail`), an index out of range, a zero divisor, `number-range` from arithmetic (N6) or from a host number that cannot enter the language (A.2), `function-error` (an error a host function returned), and a null, a wrong kind or a missing field on data, whatever its class |
| Failures `catch` does not catch | Three groups, each a list of codes. A limit or a cancellation: `steps-exceeded`, `allocation-exceeded`, `deadline-exceeded`, `depth-exceeded` and `cancelled`. A crash: `function-panic`, which has class `host`, and a failure inside the implementation, under the code `05` A.6 gives it. A variable the host did not supply: `missing-variable` (10-3). Each passes through every `try` in progress and ends the run. A failure of a handler is not caught by its own `try` |
| Position of a limit error | Not specified. An implementation may report any position in the operation that was in progress. This is what lets charges be merged (A.8). Its trace is not fixed either (A.10) |
| What a run reports | The steps and allocation units it used. For a run that ends without a limit error, both are fixed by this specification. A failed run also reports one diagnostic, with the trace of A.10 |

`02` 2.12 and this table said different things about a crash. This table
governs: `function-panic` and a failure inside the implementation are not
caught, because the state of the host function, or of the evaluator, is not
known (`10` 2.4). The catalog of `10` 2.9 carries the answer for each run code
in its `caught` field, and an implementation generates its test from that.

### A.2 Numbers at run time

`02` owns the definition. The evaluator needs it to say this, and assumes it:

| Rule | Detail |
| --- | --- |
| Value (N1) | A number has the value of an IEEE 754 double. A **whole number** is a finite value with no fraction in ±(2^53 − 1) |
| `+`, `-`, `*` (N2, N3) | On two whole numbers: a whole number, or `number-range` if the exact result is outside the range. On any other operands: the double result, or `number-range` if it is not finite |
| `/` (N4) | True division. A zero divisor is an error |
| Comparison | Equality and ordering are by value. No operation can tell how a number is stored |
| Host numbers | An integer outside the range, a NaN or an infinity cannot enter the language. Reading one fails with the code `number-range` and the class `environment`: the code says what is wrong with the number, and the class says the host supplied it. `05` A.3 gave the code and this row gave the class, and both now give both. `catch` catches it, as it catches `number-range` from arithmetic (A.1). Met inside the walk of `as` or `is`, or by the host's conformance function, such a value does not conform and raises nothing (A.3) |
| Rounding | Each operator rounds once. An implementation must not fuse two operators into one rounding, or compute in a wider format |

### A.3 Steps

A step is the unit of work. The intent is that one step is a bounded amount of
work, so work is proportional to steps. An operation that breaks that is a
defect in this table (L2).

| Work | Steps |
| --- | --- |
| Literal, variable read, `const` binding, `&&`, `\|\|`, `??`, `? :`, `try` | 0 |
| A string literal with no `${ }`, in any of the five forms of `08` 2.2 (D46) | 0. It is a `String` node whichever delimiter wrote it: `"..."`, `'...'`, a backtick, a text block, or a form with the prefix `r` (08-8). Line ends, the indentation of a text block and escapes are settled when the program is parsed, and a run does none of that work. The earlier revisions could be read both ways for a backtick with no `${ }`. It is a literal |
| `try { } catch { }`, with or without a name (D48) | 0 for the `try`, and nothing more when the body gives a value. The body charges as it runs, and what it charged before it failed stays charged (A.10) |
| A failure caught by `catch { }` | 0. No message is built |
| A failure caught by `catch (err) { }` | 1 + size of the message, when the message is built from the catalog. 0 when the message is text that already exists, which is so for `failed` and `function-error`. A.10 has the rows and worked counts |
| `if`, `else if`, `else` (D35) | 0, as for `? :`. Each test is a truthiness test. The tests are evaluated in order up to the first truthy one, and only the branch taken is evaluated and charged. A `const` line in a branch is a binding |
| Operator on numbers or bools. `==` on two scalars that are not both strings | 1 |
| Field read, index, key lookup with `in` | 1 |
| Function literal evaluated | 1 |
| Call of a library function or method, of a host function, or of a `const`-bound function literal by name | 1, plus what the callee charges. No other call exists: a name bound by a parameter or a pattern cannot be called (D34, A.9) |
| `fail(message, data?)` (`10` 2.5) | 1, as the call of a library function. Nothing for the message or the data: both are values that already exist, and neither is copied |
| Call of a host function that a case file stubs (`11` 2.3) | 1, as any call, plus the `charge` of the row that answers, and its `allocate` in units. Matching the rows is the runner's work and charges nothing, so the counts of a case do not depend on the implementation |
| Element visited by a library function | 1. A callback per element therefore costs 2 plus its body |
| Element or entry copied by a spread | 1 |
| Field read by a destructuring pattern | 1 |
| Element visited by a star, `x[*]` or `x?.[*]` (D51, `13` 5.2) | 1, charged before the reads are applied to the element. The star adds nothing of its own, so a star over an empty list costs 0, and so does a star whose receiver is null or is not a list, which fails or gives null with nothing charged |
| A read in a projection | What the read costs outside one, once for each element it is applied to: 1 for `.f`, `?.f`, `[e]` or `?.[e]`, plus what `e` charges each time. A read that a `?.` skipped for an element is not charged for that element |
| Arm tested by a `match` | 1 per arm, in order, up to the one taken |
| Type test, `x as T` or `x is T` (`07` 2.7) | 1 for each value tested: the subject, each list element reached, each declared field of a record looked up, and each other key of a record. The operator adds no step of its own, and `as` and `is` charge the same. Nothing is charged for what the walk does not reach after it stops |
| The same, at a place whose type is `any` | 0. The value there is not tested, read or entered. So the elements of an `any[]` and the other keys of a record whose rest is `any` cost nothing, `x is any[]` costs 1, and `x as any` costs 0 |
| Truthiness test (B5), in `!`, `&&`, `\|\|`, `? :`, `if`, a condition arm, or on a callback's result | 0. It reads the kind and at most a length. An implementation must be able to tell an empty host list or map from a non-empty one in bounded work |
| `in` on a list | 1 per element compared, up to the first match |
| `==` on two lists | 1 if the lengths differ. Otherwise 1 per position compared, up to the first difference, plus the cost of each comparison made |
| `==` on two maps | 1 if the sizes differ. Otherwise 1 per entry, for every entry, plus the cost of each comparison made. There is no early exit, so the count does not depend on an order |
| Text compared (`==`, `<`) | 1 + size of the longest common prefix |
| Text read (search, case mapping, length, slice) | 1 + size of the text read, at the rate `06` gives for the kind of work |
| Text created (`+`, template string, conversion to text) | 1 + size of the result |
| A `Template` node: a backtick template or a text block with at least one `${ }` (D46) | Text created, once for the whole template: 1 + size of the result. Each part's expression charges what it charges. A part that is a list or a map adds the walk of its rendering (`02` 2.13), as `06` charges it for `string(x)`: 1 for each element of a list, and the key-order row below for each map. A part that is a string, a number, a bool or null adds nothing: its rendering is not a second string. A text block and a backtick template with the same text and the same parts charge the same (`08` 2.8) |
| Sort | A formula over the length (`06`), never the comparisons made |
| Entries of a map visited in key order (`keys`, `values`, `entries`, rendering, encoding) | The sort formula of `06` over the number of entries, plus 1 per entry (overview S16) |

"Size" is the text size of A.5. The library's entries are in `06`. They apply
these rules and do not add kinds of charge.

**The type test, exactly.** `07` 2.2 says when a value conforms, and `07` 2.3
gives the order of the walk. The rows below fix the count where `07` leaves a
choice, so that three implementations agree. Each charge is made before the
value is looked at.

| At | Charged |
| --- | --- |
| A value tested against a scalar type, `int`, a literal set, an opaque name or `null` | 1, and no more. The same for a value whose kind the type does not allow, which fails there. A union adds nothing: the value's kind picks the alternative. Where the map alternative is a tagged union, the tag picks the variant, by the three rows for one below |
| A literal set | Nothing for the text. The comparison reads no more of the value than the longest literal, which is text of the program |
| A list against `T[]` | 1 for the list. Then its elements by index, each a value tested against `T`. The walk stops after the first element that does not conform. That element is charged, with whatever its own walk charged before it stopped. Later elements are not |
| A record | 1 for the map |
| Its declared fields | In code point order of their keys, 1 for each lookup, whether the key is present or not, and whatever the field's type is. When the key is present, that step is also the step of the field's value, and the value's own contents are charged after it. A field of type `any` costs its lookup and no more: the key must be found, and the value is not looked at. The walk stops at the first failure: a required key that is absent, or a value that does not conform. Later fields are not looked up, and no other key is visited |
| Its other keys, when the rest is `any` | 0. They are not visited |
| Its other keys, when the rest is another type | Reached only when every declared field passed. The value of every other key is a value tested against the rest: 1, plus what its own walk charges, which stops early by these rows. All are tested even after one fails, so the sum does not depend on an order. The key reported is the least failing one by code point |
| Its other keys, when there is no rest | Reached only when every declared field passed. Finding that one exists is free: the size of the map is compared with the number of declared keys present. With none, the record passes. With any, the test fails and 1 is charged for each other key, which is that difference. Every key was visited to find the least one, which is the one reported |
| A value against a tagged union (D47, `09` 2.7) | 1 for the value. A value that is not a map fails there, and the path ends at the value |
| Its tag | 1 for the lookup of the tag's key, whether the key is present or not. When it is present, that step is also the step of its value. The walk stops here when the key is absent, or its value is not a string, or is a string in no variant's tag set. The path ends at the tag. Finding the variant costs nothing more, however many variants there are: as for a literal set, the comparison reads no more of the value than the longest literal |
| Its variant | The rows of a record above, over the variant's other declared fields in code point order of their keys, then its other keys. The map and the tag are not charged again, and the tag is not an other key. So a value that conforms costs exactly what it costs against its variant written alone. A value that fails may cost 1 more than against the variant alone, because the tag is read before a field that sorts ahead of it |
| Choosing the least key to report | Nothing more. It is at most one key comparison for each other key and is covered by that key's step, as the sort formula covers the comparisons of a sort |
| A value that cannot enter the language (A.2): an integer out of range, a NaN, an infinity | Its 1. It does not conform to any type, so the test fails there with `not-conforming` and never with `number-range`. At a place whose type is `any` it is not looked at |
| A test the implementation can decide without the walk | The same count (A.8) |

Three cases, with `T` as `{id: string, lines: {sku: string, qty: int}[], ...}`
and a value with `id`, three lines and one more key, `note`:

| Case | Steps |
| --- | --- |
| The value conforms | 12: the subject, `id`, `lines`, and 3 for each line (the line, `qty`, `sku`). `note` is under a rest of `any` and costs nothing |
| `qty` of the second line is a string | 8: the subject, `id`, `lines`, 3 for the first line, and the second line with its `qty`. The path is `.lines[1].qty` |
| `{id: "a", note: "x", z: 1}` against `{id: string}` | 4: the subject, `id`, and 1 for each of the two other keys. The path is `.note` |

Five more, against the tagged union of `09`, `Shape`, which is `{kind:
"circle", r: float} | {kind: "rect", h: float, w: float}`. Its tag is `kind`.
`09` 2.7 has the whole table.

| Value | Steps |
| --- | --- |
| `{kind: "rect", w: 2, h: 3}` conforms | 4: the map, `kind`, `h`, `w`. Against the second variant alone it is the same 4 |
| `{kind: "rect", w: 2}` | 3: the map, `kind`, `h`. The path is `.h` |
| `{kind: "oval", r: 1}`, `{r: 1}` or `{kind: 7}` | 2: the map, `kind`. The path is `.kind` |
| `{kind: "circle", r: 1, note: "x"}` | 4: the map, `kind`, `r`, and 1 for the one other key. The path is `.note` |
| A circle, then a rectangle, against `Shape[]` | 8: the list, 3 and 4 |

**A projection, exactly (D51).** `13` 5.2 says what a projection is and what it
gives: a star and the reads and stars that follow it directly, ended by the
first method call or the end of the chain, with one flat list as its value.
The rows below fix the count.

| At | Charged |
| --- | --- |
| The receiver of the first star | What its expression charges, once. Null under `[*]`, or a value that is not a list, a map and a string among them, fails with nothing more charged. Null under `?.[*]` gives null for the whole chain with nothing more charged |
| Each element a star visits | 1, before its reads. Elements are visited by index |
| Each read applied to an element | Its cost outside a projection (the rows above), index expression included |
| A `?.` or `?.[` in the reads that finds null | Nothing. The reads and stars left for that element are not evaluated and not charged. The element gives one null, which is kept and counts in the result |
| A later star | Its receiver is the value the reads gave for the element in hand. It visits that list's elements at 1 each. Null under `[*]`, or a value that is not a list, fails. Null under `?.[*]` gives one null for the element, as the row above |
| The result | 0 steps. In units, 1 + `k` for a list of `k` values, charged once, when every read has run and before the list is built (A.4). A projection that fails part way is charged no units for a result |
| A method call after the projection | It receives the list, and charges as it does for any list |

No intermediate list is charged, because none is written: `orders[*].lines` is
a list of lists that already exist, and a second star puts its values into the
one result. A projection is not a walk over a value. Its nesting is that of its
text, and it counts nothing against the depth limit beyond the level each star
opens in the source (A.6).

Two cases, with three orders:

| Case | Steps | Units |
| --- | --- | --- |
| `orders[*].lines[*].sku`, with 2, 0 and 4 lines | 18: 3 visits, 3 reads of `lines`, 6 visits, 6 reads of `sku` | 7: a list of 6 |
| `orders[*].lines?.[*].sku`, with 2 lines, null and 4 lines | 18: the same visits and reads. The second order's `lines` is read, found null, and gives one null | 8: a list of 7, the null among the skus |

`13` 5.2 counts the first case written with `flatMap` and `map` at 37 steps and
14 units.

### A.4 Allocation units

A unit is the measure of what a run creates. The budget is on the total created
during the run, not on what is live, because the total is fixed by the program
and its data and live memory depends on a collector (L3).

| Value created | Units |
| --- | --- |
| Number, bool, null | 0 |
| String literal with no `${ }`, in any form (D46, `08` 2.8) | 0. The string is text of the program, and the run does not create it. The form of a literal has no cost |
| String, from `+`, a template string, a conversion or a library function | 1 + size of the string. A template string is a `Template` node in either form, a backtick or a text block: one string, charged once, with no charge for the rendering of a part as a string of its own |
| List of `n` elements, from a literal, a spread or a library function | 1 + `n` |
| Map or record of `n` entries, from a literal, a spread or a library function | 1 + 2`n` |
| Result of a projection, a list of `k` values (D51) | 1 + `k`, charged once, when the reads have run and before the list is built. Nothing for a star or a read. Until the charge, the values wait in scratch space, at most one slot for each step the projection has charged |
| `try` whose body gives a value, and a failure caught by `catch { }` | 0 |
| The error value that `catch (err)` binds (D48, `10` 2.7) | 9 for a map of four entries. 1 + size of the message more, when the message is built from the catalog, and nothing for the message of `failed` or `function-error`, which exists. 1 + `n` more for the path of a `not-conforming` failure, a list of `n` steps, with `n` from 0 to 64. Nothing for `code`, which is text of the specification, or for `data`, which is the value `fail` was given, or null. Charged as written: whether or not the handler reads the name (A.10) |
| `fail(message, data?)` | 0. Nothing is copied |
| A failure and its diagnostic: code, span, message, path, data, trace | 0. A diagnostic is not a value of the language. What it holds is bounded by the limits and not by the data (A.10) |
| Result of `x as T` or `x is T` | 0. `as` gives back the value it was given, not a copy, and `is` gives a bool. A failure creates no value either: its diagnostic is not a value of the language. What an implementation holds for a failure is bounded by the type and not by the data: at most 64 path segments, one for each level of the type (`03` 2.17), each an index or a key that already exists, and the expected type, which is text of the program or the environment. A message that copies a key cuts it to 64 code points, as the catalog cuts every parameter (`10` 2.9). The same holds against a tagged union, which adds no level and no value (`09` 2.7) |
| Function literal | 0. It cannot be stored or returned (E6), and every use of it is a call whose callee is known from the text (D34, A.9). So no implementation needs to create anything for one. One that does is bounded by the 1 step of A.3 |

| Rule | Detail |
| --- | --- |
| Containers and new text, not contents | A list is charged for its slots, not for the values in them. A string that already exists is not charged again when a list or map refers to it |
| Before the memory is taken | An implementation charges a value before it creates it. When the size depends on the work (a filter, a split), it finds the size first |
| Scratch space | Working memory that is not a value is not charged. It must be bounded by a constant times the values that exist or the units charged. The values of a projection that wait for its one charge are scratch space, bounded by the steps it charged, which the guarantee of A.6 allows for |
| Host data | A value the host supplied is never charged (L4) |
| As written | A constant collection costs its units each time it is evaluated, even if an implementation builds it once. An intermediate value costs its units even if an implementation never builds it |

### A.5 Text size

**The rule.** The weight of a string is its number of code points, counting a
code point above U+FFFF twice. The size of a string is its weight divided by 64,
rounded down. The size of several strings taken together is the sum of their
weights divided by 64, rounded down.

- Weight is defined on code points and names no encoding. It equals the UTF-16
  length. For text with no code point above U+FFFF it is the length the author
  sees (T1).
- **Rounding down makes short text free to measure.** A string's weight is never
  more than its UTF-8 length or its UTF-16 length. So a string under 64 bytes,
  or under 64 UTF-16 units, has size 0, and no implementation needs to scan it.
- A unit of text size covers at most 192 bytes of UTF-8 and exactly 128 bytes of
  UTF-16.
- Comparison charges the common prefix, not the length, so two long strings that
  differ at once cost one step.

B.8 argues for weight over UTF-8 bytes and code points, with measurements.

### A.6 The limits

| Limit | Guarantee | Does not bound |
| --- | --- | --- |
| Steps (L2) | The work the language and its library do is at most a constant times the steps charged. The run fails with the step error at the first charge the budget cannot cover | Time inside a host function, beyond what it charges |
| Allocation (L3) | The memory the run creates as values is at most a constant times the units charged, plus a smaller constant times the steps. Each implementation publishes its constants. The run fails before it takes memory the budget does not cover | Live memory, which is lower. Host data. Memory a host function takes without charging. The fixed state of a run |
| Deadline (L5) | An implementation checks the deadline at least once in every 1,024 steps charged. The check reads a cancellation signal or the clock, whichever it can observe during a run. A run whose deadline has passed at a check fails there. It overruns by at most 1,024 steps of work plus the host function in progress | A host function that blocks. It is the one limit that is not deterministic |
| Depth (L1) | Each call in progress counts 1 plus the nesting depth of the call within the body that makes it (`02` 2.16). The parentheses of an `if` and the braces of each branch open a level, as any bracket does. An `else if` opens none, so an implementation evaluates a chain of them in a loop and not by recursion, as it does a run of `const` lines. The total is at most the limit, D. So is the nesting any walk over a value reaches (`==`, rendering, encoding). An evaluator's recursion is then at most D + 100 levels and a whole run's 2D + 100, which is 500 at the default. A plain count of calls would give the product, 20,100. The calls in progress are calls of different function literals, so the largest total the calls of one program can reach is fixed by its text (A.9). An implementation may compute it when it compiles, and make no test at a call when it is within D. The nesting a walk reaches depends on the data and is always counted. The walk of `as` and `is` is a walk over a value and counts the same way: 1 for each list or record it enters. It enters a level only where the type has one, and a type is at most 64 deep (`03` 2.17), so it enters at most 64 levels whatever the data. A place whose type is `any` is not entered. A tagged union is entered as a record is: one level, for the variant (`09` 2.7). The braces of the body and of the handler of a `try`, and the parentheses of `catch (err)`, each open a level, and a star opens one that lasts to the end of its projection (`02` 2.16). So an evaluator may recurse once for each star, at most 100 times, and a projection counts nothing more: it is not a walk over a value. A failure that `catch` catches returns the count to what it was at the `try`. `depth-exceeded` is not caught (A.1). 2D + 100 stands | Nesting of the source, which has its own specified maximum of 100 levels (overview R5, `02` 2.16). Releasing a deeply nested value, which is not a walk the language makes (overview R6) |

The limits are complete for what the language and its library do (L8). They do
not cover host functions, which charge for themselves through the call handle.
They are not a sandbox: the constants are loose, and nothing here defends
against a host function, a hostile host value or a timing channel. An
implementation with no signal to poll during a run, such as one in JavaScript,
reads the clock at each deadline check.

### A.7 Determinism

The same program, variable values, host function results, step limit and
allocation limit give the same outcome, and the same steps and units used. The
outcome is the same value, or the same failure: its code, span, message, path,
data and trace (A.10). Two things are outside that. A limit error has no fixed
position and no fixed trace (A.1). And the class of a null, kind or field
failure follows from the checker's sites (`03` 2.14), so it is the same only
for the same check, and `03` says what it is with no check. A program can see
neither. An implementation must:

- evaluate in the order of A.1 and report the first failure;
- build the message of a failure from the catalog of `10` 2.9 and from nothing
  else. A program can read `err.message` and is charged for its size, so a
  second wording would be a second value and a second count (L6);
- visit, render and encode a map in key order by code point (T2), wherever order
  can be seen. Lookup, size, `in` and `==` need no order. The walk of `as` and
  `is` reads the tag of a tagged union first, takes declared fields in key
  order, and needs no order for other keys (A.3);
- give the number results of A.2 on every platform;
- charge by A.3 and A.4, the same in every version of the specification (L6);
- use no clock, random source or shared state. The deadline is the only input
  that is not a value.

### A.8 What an implementation may do when optimizing

The reference is the evaluation of the program as written, charging by A.3 and
A.4. An implementation may evaluate in any other way that gives:

1. the same value, or the same failure as A.7 counts it, with its trace;
2. the same steps and units used, at the end of the run, at every call of a host
   function, and whenever `catch` catches a failure, before the error value is
   charged;
3. a limit error exactly when the reference has one, and never a host function
   call that the reference would not reach.

| Allowed | Why it is safe |
| --- | --- |
| Merge consecutive charges into one | When nothing between them can fail or call the host, and a limit error has no fixed position |
| Fold a constant expression | The fold's cost is known at compile time and is charged where the expression stood |
| Fuse a chain of reads, or inline a function literal | The charges are the same numbers at the same points. A call that is inlined still has its entry in the trace of a failure (A.10) |
| Charge several operations at once and correct on failure | When a failure part way gives back what was not done, and a budget too small for the whole falls back to charging one at a time. The visits and reads of one star are such a group |
| Skip building an intermediate value | When its units and steps are charged as if it were built. The list of a projection that feeds `sum` is one. So is the error value of a `catch (err)` whose handler never reads the name, or reads one field of it |
| Write no trace entry for a failure that a `try` in progress will catch | Whether `catch` catches is fixed by the code, which is known when the failure is raised, and the trace of a caught failure reaches nobody (A.10) |
| Find the variant of a tagged union from a table | Tag sets share no string, so the reference makes no search either. The tag's step is charged (A.3) |
| Replace a scan by a lookup | When the lookup yields what the scan would have charged |
| Bind a call to its callee when compiling, keep each binding in one fixed place, and create nothing for a function literal | A.9: the callee is known from the text, and no function literal has two calls in progress |
| Make no depth test at a call | When the largest total the program's calls can reach is within the limit (A.6, A.9) |
| Skip the test of `as` or `is`, or a part of it | When the implementation can prove the result, and it charges the steps the walk would have charged (A.3, `07` 2.7). The count often comes from lengths alone: a list of `n` strings against `string[]` is 1 + `n`. The work may go. The charge never does. A type the checker inferred is not a proof: the test depends only on the value and on the type as written (`07` 2.6), and a host's value may break the precondition of `03` 2.14 |

Not allowed: stopping early where the reference goes on, such as turning
`xs.filter(f)[0]` into a search that stops at the first match. What the
remaining calls of `f` would have charged is not known without making them. The
same holds for skipping an unused `const`, and for `(xs[*].f)[0]`, where every
element is visited and read before the index. An author who wants the early
stop writes the library function that has it.

### A.9 Calls: what the text fixes (D34)

A function literal is passed only to a library function or method, and a name
bound by a parameter or a pattern cannot be called (overview E9, `02` 2.9). So
the calls a program can make are fixed by its text. An evaluator may rely on the
facts below. They are derived from the rule. Nothing here is measured.

| Fact | Why |
| --- | --- |
| The callee of every call is known when the program is compiled | A call names a library function or method, a host function, or a name that a `const` binds to a function literal. A library function calls only the function literals passed to it, and each of those is written at the call or named there |
| A function literal runs only while every body around it is in progress | It cannot be returned, stored or passed to a host function (E6, S3), and a library function calls a function literal only before it returns |
| No function literal is entered while an earlier call of it is in progress | The body of a literal `L` can enter a literal written inside `L`, or one bound by a `const` that ends before `L` starts, and no other, because a name is not in scope in its own value. Either one ends earlier in the text than `L` ends. So along the calls in progress the end positions fall, and no literal appears twice |
| A program with `k` function literals has at most `k` calls in progress | The same argument |
| The largest total the depth limit can count for a program's calls is fixed by its text | The calls form a graph with no cycle. The total is its longest path, with each call weighted as in A.6. One pass over the literals, in the order they end, finds it |

| An implementation may | Because |
| --- | --- |
| Keep each parameter, each `const` name and the name of each `catch (err)` in one fixed place for the whole run | A body has one call in progress at a time, and a handler runs in the body that holds its `try` |
| Create nothing when a function literal is evaluated | Every use of it is a call the compiler has already bound |
| Let a body read an outer name from the place where the outer body wrote it | The outer body is still in progress, in the same call that evaluated the literal |
| Know before a run whether its calls can reach the depth limit | The longest path above |

One rule goes with a fixed place for parameters: every argument of a call is
evaluated before any parameter is written. In `f(a, f(b, c))` the inner call
writes the same parameters as the outer one.

None of this depends on a binding being constant. Loops, reassignment and early
`return` are not in 3.0; the words are reserved (overview 2.10, X3 to X6). If a
loop over a list with reassignable locals arrives, a body still has one call in
progress, a slot is written more than once, and an inner body reads the current
value. An early `return` would be a new way to leave a body, which each
evaluator would have to carry.

Two forms of D48 and D51 touch these facts and break none.

- **A caught failure leaves bodies early.** It leaves every call in progress
  between the place that raised it and its `try`. A call that was left is no
  longer in progress, so the facts above hold for the handler and for what
  follows. Nothing has to be put back but the depth count (A.10): a fixed
  place has no state to restore, and a name bound in the body of a `try` is
  not in scope in its handler (`10` 2.2).
- **A projection is not a call and runs no function literal.** Each star is
  one pass over a list that exists, with nothing carried from one element to
  the next (`13`, K2). It binds no name, so it has no place to keep. The index
  expression of a read may make calls, and those are calls of the text like
  any other.

### A.10 Failures: unwinding, the error value and the trace (D48)

`10` is the authority for the form of `try`, the fields of the error value, the
catalog of messages and the fields of a diagnostic. This section fixes what an
evaluator does, charges and may hold between the place that raises a failure
and the `try` or the host that receives it.

**Unwinding.**

| Rule | Detail |
| --- | --- |
| What a failure leaves | Every operation and every call in progress between the place that raised it and the innermost `try` whose body is in progress. When there is none, or when `catch` does not catch its code (A.1), it leaves the run. Nothing is evaluated on the way: there is no `finally` |
| What stays charged | Every step and unit charged before the failure, by the body of the `try` and by everything else. A charge made ahead for work that was not done is given back before the handler runs and before the usage is reported (A.8) |
| Leaving is free | No step and no unit is charged for leaving a call, a star or a `try`. Each call in progress is left at most once, and entering it cost a step (`10` 2.7) |
| What is put back | The depth count returns to its value at the `try` (A.6). Nothing else of the language has state to restore: every name has a fixed place (A.9), and a name bound in the body is not in scope in the handler |
| The handler | Evaluated in place of the body, with the name, if there is one, bound to the error value below. Its value is the value of the `try`. A failure of the handler is the failure of the `try`. Its own `try` does not catch it, an outer one may, and the first failure is gone: 3.0 has no way to raise it again unchanged (`10` 2.5) |

**The error value, exactly.** `catch (err)` binds an exact record of four
fields (`10` 2.3). It is the one thing a failure creates in the language.

| Part | Steps | Units |
| --- | --- | --- |
| The map of four entries | 0 | 9 |
| `code` | 0 | 0. A code is text of the specification, as a literal is text of the program |
| `message`, built from a template of the catalog | 1 + size of the message | 1 + size of the message |
| `message` of `failed` and of `function-error` | 0 | 0. It is the string `fail` was given, the same value and not a copy, or the text of the host's error as the binding gives it, which is host data (L4) |
| `path` of a `not-conforming` failure, `n` steps | 0 | 1 + `n`, with `n` from 0 to 64. A key in it is a string that exists |
| `path` of any other failure | 0 | 0. It is null |
| `data` | 0 | 0. It is the value `fail` was given, the same value and not a copy, or null |

- **Charged as written.** The whole value is charged whether or not the handler
  reads the name, as an unused `const` is (A.8). With `catch { }` there is no
  value: nothing is built and nothing is charged.
- **Charged at once, before it is built:** the steps, then the units (A.1). If
  either passes its budget the run ends with that limit error and the handler
  does not run.
- **The size of a message is fixed by the specification.** It is the size of
  A.5 of the text the catalog's template gives with its parameters filled in
  (`10` 2.9). A message under 64 units of weight has size 0, so it costs 1
  step and 1 unit.
- **Building a message is bounded work.** The catalog cuts the rendering of
  each parameter to 64 code points, so an implementation reads no more of a
  value than the cut needs, whatever the size of the value. The step is a fair
  charge for it.
- **A message is built only here.** A failure that no `catch (err)` receives
  has its message built for the host's diagnostic, outside the run's counts.

The counts below assume a message of size 0. The wording is in
`messages.json`, not in these documents, so the suite's counts come from that
file.

| Program | Steps | Units |
| --- | --- | --- |
| `try { 1 / d } catch { 0 }`, with `d` zero | 1: the `/` | 0 |
| `try { 1 / d } catch (err) { 0 }` | 2: the `/`, and 1 for the message | 10: 9 for the map and 1 for the message |
| `try { body as Order } catch (err) { err.path }`, the second case of A.3, which fails at `.lines[1].qty` | 10: 8 for the walk, 1 for the message, 1 for the read of `err.path` | 14: 9, 1, and 4 for a path of three steps |
| `try { fail("no stock", {sku}) } catch (err) { err.data }` | 2: the call, and the read of `err.data` | 12: 3 for the map `{sku}`, which the body built, and 9 |

**What a failure may hold.** A failure is not a value of the language, so it is
charged nothing (A.4). The run must still bound it, and it does:

| Part | Bound |
| --- | --- |
| Code, class, span | A fixed size |
| What the message needs | The name of a template and its parameters. Each parameter is a reference to a value or a text that exists, or a rendering of at most 64 code points. For `failed` and `function-error`, a reference to the text |
| Path | At most 64 segments, one for each level of the type (`03` 2.17), each an index or a key that exists |
| Data | One reference to a value of the run, which the allocation budget bounded when it was built, or to host data. No copy (`10` 2.5) |
| Trace | At most D + 100 entries, each of a fixed size (below) |
| The cause of a `function-error` | The host's own error. It is memory a host function took, which no limit bounds (A.6) |

So a failure holds a constant, plus 64, plus D + 100 small records, whatever
the data. A caught failure is charged only for the error value above, and the
rest of what it held is dropped.

**The trace.** The diagnostic of a failed run lists the calls in progress when
it failed (`10` 2.8, 10-5). A program cannot read it.

| Matter | Rule |
| --- | --- |
| A call by name of a `const`-bound function literal | One entry: the span of the call, the name, and a null index |
| A library function or method that is running a function literal passed to it | One entry: the span of the call, the entry's name, and the index of the element in hand, from 0, which is the index the callback was given. A literal passed by name has this entry and no second one |
| A star with an element in hand | One entry of the same shape: the span of the star and the index of the element in the list that star iterates (`13` section 7). Its name is the one `05` A.6 gives a star |
| No entry | A `try`, a library call that takes no function literal, a library function that fails in its own work and not in a callback, and a host function, whose failure has the span of its call |
| Order | From the outermost in. The detail rendering of `10` 2.10 prints the innermost first |
| When it is written | Only as the failure leaves each call or star, so the innermost entry is written first. Everything an entry needs is at hand there: the span and the name are text of the program, and the index is the position of the pass that was running |
| A run that does not fail | Pays nothing for the trace: no step, no unit, and no work. An implementation keeps no list of calls during a run |
| A run that fails | Is charged nothing for the trace either. Writing an entry is part of leaving, which is free |
| Length | At most D + 100 entries, and it is never cut. Each call in progress counts at least 1 against the depth limit D. A star that has a call in progress inside it is counted in that call's nesting (A.6). The stars of the innermost body are bounded by the 100 levels of the source. `10` 2.8 says at most D, which was written before a star had an entry |
| A caught failure | Has no trace a program can see. An implementation need not write one (A.8) |
| A failure raised in a handler | Has the trace of the place that raised it: the calls in progress there, those around the `try` among them |
| A limit error | Has a trace, and it is not fixed, because the position of a limit error is not (A.1). The suite does not compare it |

In `orders[*].lines[*].sku`, with no `sku` in the second line of the third
order, the trace has two entries: the first star with index 2, then the second
star with index 1. `10` 2.8 has an example with calls.

## Part B. The Go implementation

### B.1 Execution strategy

**Decision: compile each AST node to a Go closure, in two layers.**

Layer 1 is the reference evaluator of A.8. Every node is a `func(*run) (any,
error)`. Its argument is the run object, because there is one set of slots per
run and no frame (B.5). Each operator tests the common operand types first and
falls back to a general function, as the spike's `+` does (`spike:533-551`). It
needs only name resolution from the checker.

Layer 2 adds typed forms `boolNode`, `intNode`, `floatNode` and `strNode`, which
return `(bool, error)`, `(int64, error)` and so on. The compiler picks the form
from the checker's type and from what the consumer asks for, and wraps a node in
a checked boxing or unboxing where the two differ. A condition is tested by
truthiness (B1, B5). Where the checker's type is exactly `bool` the condition is
a `boolNode`. Elsewhere it is `truthy(v)`: a switch on the kind that reads at
most a length. The binding exports the same function as `risor.Truthy` (`05`),
so a host that tests a result by truthiness (D33) and a condition inside a
program cannot disagree. A typed leaf does not trust the checker:
`asInt` accepts an `int64` or a whole `float64` in the range, and returns an
error of class `environment` for anything else: `number-range` for a number
that cannot enter the language (A.2), and for any other value the failure that
`03` 2.14 gives a value that breaks its declared type. Layer 2 charges at the
same points as layer 1. A differential test runs every conformance case
through both and compares value, steps and units, and for a failing case the
code, the span and the trace.

**How much of layer 1 is the language.** I estimate 1,900 lines for layer 1 over
JSON-shaped values with the limits: about 1,100 of semantics (operator and
number rules, equality and ordering, chains, the cost tables, error codes,
scoping) and 800 of Go mechanics (closure plumbing, error returns, slots, the
pool, type switches) (I). The basis is the spike, where 618 of 1,240 lines are
semantics by my count of line ranges. The semantic part is written as plain
functions over values, such as `add(a, b)` and `equal(a, b, charge)`, so it
ports line for line.

D46 to D51 add about 230 lines to layer 1, nearly all of them semantics: 60 for
`Try` and the error value, 80 for the trace (`10` section 3), and 90 for
`Each` (`13` 5.6). So layer 1 is about 2,130 lines, of which about 1,330 are
semantics (I). The string forms of D46 add nothing here: the lexer settles
them (B.12).

### B.2 Value representation and numbers

**Decision: a value is a Go `any`, in seven shapes: `nil`, `bool`, `int64`,
`float64`, `string`, `[]any` and `map[string]any`.** Go's `nil` is the
language's null. A host's `map[string]any`
already holds what the evaluator works in, which is what reading in place
requires. Lists and maps the evaluator builds have the same types.

`lab/valuerep` runs the same expressions in three representations, each charging
a step and checking the number range per operator. "Tagged" is a three-word
struct `{ref any; num uint64}` with numbers and bools inline. The ints are above
255, so boxing one allocates.

| Case (M) | `any` | tagged struct | typed closures |
| --- | ---: | ---: | ---: |
| `a * b + c * d - e` | 27.2 ns, 4 allocs | 18.4 ns, 1 | 16.0 ns, 1 |
| `a < b && c + d > e` | 14.6 ns, 1 | 13.7 ns, 0 | 11.6 ns, 0 |
| `price * 1.08 + ship > limit` | 15.9 ns, 2 | 11.6 ns, 0 | 10.4 ns, 0 |
| `gate.failed > 0`, JSON float | 9.9 ns, 0 | 13.1 ns, 0 | 10.0 ns, 0 |
| `input.event.pull_request.head.sha` | 22.3 ns, 0 | 28.8 ns, 0 | as `any` |
| `reduce` over 10,000, `acc + x * x` | 156 µs, 19,975 | 83 µs, 1 | 111 µs, 9,992 |
| the same, with typed slots | – | – | 70 µs, 1 |

The tagged struct wins on numbers and loses on reads, and it would make two list
representations. Typed closures get the gain on numbers at no cost on reads.
They still box once per loop iteration. Typed slots remove that, and wait.

**Numbers (N1 to N6).** `int64` and `float64` are two storages of one kind of
number. An `int64` the evaluator holds is always in ±(2^53 − 1).

| Rule | Detail |
| --- | --- |
| Integer `+` and `-` | The operands are in range, so the result cannot wrap. One unsigned comparison checks the range |
| Integer `*` | Multiply as floats and compare with the range, then multiply as ints. Both operands convert exactly and rounding is monotonic, so the float product is in range exactly when the true product is |
| Float results | One comparison: a result within the range passes. Beyond it, an infinity is an error, and so is a whole number made from two whole operands |
| Mixed operands | Convert the `int64` to `float64`, which is exact. `==` between an `int64` and a `float64` compares as floats. A library function that hashes numbers hashes the float value |
| `/` | Always a `float64` |

| Typed integer arithmetic (M) | `a * b + c * d - e` | `reduce` over 10,000 |
| --- | ---: | ---: |
| range check (N2) | 16.0 ns | 70.0 µs |
| 64-bit overflow check (the first draft) | 19.4 ns | 85.3 µs |
| no check | 17.4 ns | 76.8 µs |

The range check is within noise of no check. The overflow check was slower
because checking a multiply needs a divide.

**Numbers in a result.** A host can rely on this:

| Where | Go type |
| --- | --- |
| Top level, checked type `int` | `int64` |
| Top level, checked type `float` | `float64`, even for a whole value |
| Top level, any other type | `int64` for a whole number, `float64` for any other |
| Inside a list or map | `int64` or `float64` with the right value. Which one is not specified. A number the host supplied keeps the host's type. The binding offers a helper that reads either |

### B.3 Host values

**Decision: one read barrier, accessors built at compile time for Go types, and
reflection behind both.**

**The read barrier.** A value that leaves a host container (a variable, a field,
an element, a function result) passes one type switch:

| Go value | Becomes |
| --- | --- |
| `nil`, `bool`, `string`, `[]any`, `map[string]any` | itself |
| `int64` in ±(2^53 − 1), finite `float64` | itself |
| `int64` or any integer outside the range, NaN, an infinity | the failure `number-range`, class `environment` (N2, N3, A.2). The host passes such a value as a string |
| Other integer and float types, named scalar types | `int64`, `float64`, `string` or `bool`, with the same range rule |
| nil pointer, nil interface | `nil` |
| pointer to a scalar | the scalar |
| struct, pointer to struct | unchanged, read through an accessor |
| other slices, arrays and string-keyed maps | unchanged, read through a list or map view |
| anything else (`time.Time`, a channel) | unchanged and opaque |

The barrier adds 0.9 ns to a map read (5.9 ns against 5.0 ns), and the range
check adds nothing measurable to that: 6.0 ns with it against 6.2 ns without for
an int, and 6.7 ns for a float against 6.2 ns for the bare read (M,
`Field/*/map*` in `lab/hostread`). Alternative: normalize where a value is used.
Why not: one missed site gives a wrong answer.

**Structs.** `lab/hostread` reads one field of a `*Input`, and the path
`input.Event.PullRequest.Head.SHA`, which crosses an embedded struct, a pointer
and another embedded struct.

| Read (M) | Time | Allocs |
| --- | ---: | ---: |
| `map[string]any`, one field | 5.2 ns | 0 |
| struct, plan per type in a `sync.Map` (expr, v2) | 26.6 ns | 1 |
| struct, index resolved at compile time, boxed result | 17.0 ns | 1 |
| struct, index, typed result (`string` or `int64`) | 4.6 ns | 0 |
| four maps, one closure per field | 24.3 ns | 0 |
| four struct fields, fused, reflection by index | 11.3 ns | 0 |
| four struct fields, fused, unsafe offsets | 4.5 ns | 0 |
| expr `Run` of the path on the struct, whole run | 158 ns | 2 |

The cost of a struct read is the boxing, not the reflection: a `string` or `int`
field is not an `any` in memory, so making it one allocates.

| Rule | Detail |
| --- | --- |
| Accessors are built at compile time | The checker's hint gives the struct's `reflect.Type` and field index path. The closure compares the value's type with the promised one, then reads by index. expr finds its plan in a `sync.Map` on every read (`expr:struct_tags.go:57-80`). v2 converts the field's value on every read (`v2:object/go_struct.go:244-255`) |
| Paths are fused | A chain of field reads on Go types is one closure that stays in `reflect.Value`s until the leaf. This is expr's `evalSelectorChainRV` (`expr:program.go:896-944`) with the lookups moved to compile time. A `?.` in the chain is a test for null with an exit (B4) |
| Type unknown at compile time | The closure caches the last `reflect.Type` it saw and its field index in an `atomic.Pointer`. A miss uses a per-type plan in a `sync.Map` |
| A struct is never copied | An element of a `[]User` is passed as a `*User`. A struct in a result is a pointer into the host's data |
| Unsafe offsets wait | They make a fused path 2.5x faster, a gain of 7 ns. The module's first `unsafe` deserves its own decision |

**Typed slices and maps.** A list function works on a view with `len()` and
`at(i)` that holds a `[]any` directly or a `reflect.Value`. Counting the
elements of 1,000 that pass a predicate takes 1.6 µs for a `[]any` of strings,
10.5 µs and 1,000 allocations for a `[]string` with each element boxed, 5.9 µs
for a `[]any` of maps, and 7.2 µs with no allocation for a `[]User` with a
pointer to each element (M, `Slice` in `lab/hostread`). Passing a pointer to
each element of a `[]string` (1.0 µs) waits for typed slots.

**Library types.** An opaque Go type takes part only through a per-type entry
that the host or the library registers: fields, methods and an ordering.

### B.4 Host functions

**Decision: one native signature, adapters chosen at registration, reflection as
a fallback (S4).** The native form takes the call handle and `args []any` and
returns `(any, error)`. `args` is a window of the run's argument stack, valid
only during the call. Arguments are read-only by contract (S2).

| The call handle | |
| --- | --- |
| `Context()` | The run's context, for the function's own cancellation |
| `Charge(steps)` | Spends steps. A function that loops charges per element (L4) |
| `Alloc(units)` | Spends allocation units, before the function takes the memory (L4). A helper gives the units for a string or a list |

A function literal cannot be passed to a host function, and the handle has no
way to call one (S3, D34). A function registered by its Go signature has no
handle, so the adapter charges its result after the call, by A.4, one level
deep. L8 allows that for a host function.

`lab/hostcall` calls `hasPrefix(s, prefix string) (bool, error)` and `clamp(x,
lo, hi int64) (int64, error)`:

| Call (M) | `hasPrefix` | `clamp` |
| --- | ---: | ---: |
| native, written by hand | 8.9 ns | 14.4 ns, 1 alloc |
| generic adapter | 9.4 ns | 15.2 ns, 1 |
| adapter with a converter per parameter, for `int` | – | 18.2 ns, 1 |
| adapter bound to typed argument closures | 3.8 ns | 3.7 ns, 0 |
| `reflect.Value.Call`, types cached | 146 ns, 4 allocs | 165 ns, 5 |

Registration accepts any Go function. It picks a reflection-free adapter where
the signature has one, a converter for a parameter type the evaluator does not
produce, and reflection for the rest, and it reports which path a function got.
expr uses reflection for every function that is not native
(`expr:prepared.go:103-156`), and v2 for every Go function
(`v2:object/go_func.go:120-149`).

### B.5 Frames, slots and closures

This section is rewritten for D34 (A.9). The earlier revision kept a frame
stack, because a function value passed to another could be entered while an
earlier call of it was still running. That cannot happen now. `lab/lambda` was
run under the earlier rule and has not been run again, so each claim below is
marked (M) or (I).

The spike gives each activation a frame with six inline slots (`spike:53-60`),
pools the frames of functions that no inner function can capture (`spike:285`,
`:842-876`), and has a closure hold a pointer to its defining frame
(`spike:287`).

**Decision: one array of slots per run. No frame, no frame stack and no closure
object (I).**

1. **Every binding has a fixed slot.** The compiler gives each name bound by a
   `const`, a parameter, a pattern or a `catch (err)` one index in the run's
   slot array. The array's length is the number of bindings in the text, and
   it is sized once (B.12).
2. **A function literal is compiled once, to its body.** Evaluating the literal
   charges its step (A.3) and creates nothing.
3. **A library function runs the body in place.** It writes each element to the
   parameter's slot and calls the body. This holds for a literal written as the
   argument and for a `const`-bound one passed by name.
4. **A call by name is a direct call.** The call node evaluates its arguments,
   writes them to the callee's parameter slots and calls the body. The argument
   count was checked when the program was compiled.
5. **A body reads an outer name from its slot.** Nothing is captured (A.9).

| Rule | Detail |
| --- | --- |
| Arguments before slots | Every argument of a call by name is evaluated before a parameter slot is written (A.9). A call with one argument needs no temporary. A call with more keeps the earlier arguments on the argument stack until the last is evaluated |
| Slots are not numbered by depth in the text | In `const f = a => a + 1; xs.map(x => f(x))`, `a` and `x` stand at the same depth and are both in use during a call of `f`. Frames could give them the same number. One array cannot. One slot per binding is always right. Two bindings may share a slot only when the call graph shows their bodies are never in progress together, and that waits |
| Nothing is saved or restored at a call | No earlier call of the callee is waiting for its slots |
| A slot keeps its last value until the run ends | A run can therefore keep one dead value alive per binding. Slots are cleared when the run object goes back to the pool (B.11) |

| Earlier revision | Now |
| --- | --- |
| A function literal that is an argument of a library function is compiled inline, with its parameters in slots of the enclosing frame | Kept. It now covers every function literal a library function is given, written in place or passed by name (3) |
| A function value bound by `const` captures by value | Gone. The body reads the slots (5) |
| Frames for calls to function values come from a per-run stack | Gone. A call by name writes fixed slots (4) |

| `lab/lambda` (M, under the earlier rule) | Time | Allocs |
| --- | ---: | ---: |
| `nums.filter(x => x > limit)`, 10,000, pooled frames | 128 µs | 17 |
| the same, a new frame per call | 294 µs | 10,017 |
| the same, inline | 72 µs | 16 |
| `rows.map(r => r.vals.filter(v => v > r.min))`, pooled | 195 µs | 961 |
| the same, inline | 143 µs | 760 |

**What the measurements still decide.**

- (M) The two inline rows are this design, for a literal written as the
  argument: the parameter is a slot of the one frame and the body runs in place.
  D34 does not change those two programs, so the figures stand, on hand-built
  trees.
- (M) The two pooled rows and the new-frame row measure frames, which nothing
  needs now. Under the earlier rule they chose between two ways to run a literal
  argument, and frames stayed for function values. Now they record what is
  avoided: 1.8x on the flat case, 1.4x on the nested one, and an allocation per
  call where a frame could be captured.
- (I) A `const`-bound literal passed by name, as in `nums.filter(isBig)`, runs
  through the same code as a written one, so it should cost the same. Under the
  earlier rule it was a function value and took a frame. The lab has no such
  case.
- (I) A call by name should cost less than the pooled call. The pooled call
  tests the callee's type and argument count, takes a frame from the pool,
  clears it and puts it back. The direct call writes the slots and calls the
  body. The lab has no call by name.
- (I) The lab's nodes take a frame and reach the counters through it. Nodes that
  take the run object save that load.
- Not measured at all: a call with several arguments, which uses the argument
  stack; a slot array as long as a large program's bindings, which the run
  object clears (B.11); and a run that makes no depth test at a call (B.6).

So the decision no longer rests on the 1.8x. It rests on A.9: frames, their
pool and captured values are code that no program can need. The measurement
says that the simpler design is also the faster one where it was measured.
Section 2 lists the cases to add before the (I) marks here can go.

There is no recursion (E6), and no function literal has two calls in progress
(A.9). So parameters are fixed slots and there is no frame stack. A function
literal never leaves its run (E6, S3), and in this implementation it is not a
value at all. Fixed slots also make the typed slots of B.2 a layout the
compiler picks for each binding (I). They still wait.

| Construct `02` may add | How it compiles |
| --- | --- |
| `match` (E1) | The subject goes to a slot. Arms are tested in order. When every pattern is a literal, a constant table maps a value to its arm, and the charge is the arm's position (A.8) |
| `const` in a block expression (E2) | A slot write for each `const` line, in a loop, then the body. The spike nests each binding inside the one before (`spike:213-220`), which recurses once for each binding; `02` 2.16 requires that a block of any length costs no stack, and the same for an operator chain, a method chain and an `else if` chain |
| `if`, `else if`, `else` (D35) | The closure of `? :`: the test is a `boolNode` or `truthy(v)` (B.1), then one branch runs. A chain compiles to one closure that holds its tests and branches in a list, with the `else` branch last, and walks the list in a loop, so a chain of any length costs no stack. A branch with `const` lines is a block, compiled as the row above. A branch with none is its expression. No step is charged (A.3) |
| Destructuring (E3) | One field read per name into a slot, through the same accessors as `.` |
| `**` (E4) | An operator with the range check of B.2 |
| `try { } catch (err) { }` (D48) | Two branches, each compiled as a block, and one slot for the name. B.10 has the node |
| A projection, `x[*].f` (D51) | One closure for the node `Each`, which binds no name and takes no slot. B.15 has it |

None of the first five needs new machinery. `try` needs the test by code and
the error value, and a projection needs a scratch buffer.

### B.6 Charging steps, and the deadline

The run's state holds a counter that counts down from a chunk of at most 1,024
steps. When it passes zero, a slow path polls `ctx.Done()` without blocking,
reads the clock if the limits carry a deadline, then takes the next chunk or
returns the error. `lab/budget` builds one expression and one loop six ways:

| Charge per operator (M) | `a + b + c + d < e` | count over 10,000 |
| --- | ---: | ---: |
| none | 12.9 ns | 50.3 µs |
| count up, compare with the limit (`spike:112-117`) | 14.1 ns | 49.7 µs |
| count down, compare with zero | 13.8 ns | 50.6 µs |
| count down in chunks, poll the context per chunk | 13.6 ns | 52.8 µs |
| the same, and read the clock per chunk | 13.8 ns | 54.8 µs |
| `ctx.Err()` and a counter on every node (`expr:program.go:292-303`) | 18.6 ns | 74.2 µs |

The counter costs about 0.25 ns per operation. Reading the clock once per chunk
adds 2 µs to 20,000 steps. Checking the context on every node costs 35% to 47%.
Exhaustion is exact whatever the chunk size: the test fails at 19,999 steps and
passes at 20,000. The evaluator reads the clock only when the host sets a
deadline. Depth is one counter. A call adds 1 plus its nesting depth within its
body, a number the compiler stores on the call node, and a walk over a value
adds 1 for each level it enters (A.6). The compiler can also store the largest
total the program's calls can reach (A.9). A run whose depth limit is at least
that total needs no test at a call. In Go this saves one comparison per call, so
it is worth little here. It is not built or measured (I).

### B.7 Charging allocation

The run's state holds a second counter of units. Each place that makes a list, a
map or a string calls `alloc(n)` first: one subtraction and one test, next to an
allocation that costs 20 ns or more.

| Output | How its size is known before it is made |
| --- | --- |
| Literal, `map`, spread, `+` | From the operands |
| `filter` | The predicate's results go into a bitmap, one bit per input element, in the run's scratch space. Then the exact size is charged and allocated once |
| `join`, `split`, `flatten` | A counting pass, then the charge, then the allocation |
| Text built piece by piece (a template that renders a list) | The builder charges each block of 64 before it grows into it |

`lab/alloc` builds each expression charging steps only, and charging steps and
units:

| Case (M) | Steps only | Steps and units |
| --- | ---: | ---: |
| `[a, b, c]` | 28.3 ns | 28.1 ns |
| filter and map over 10,000, growing the output | 126.4 µs, 362 KB | 128.7 µs, 362 KB |
| the same, exact size from a bitmap | 115.6 µs, 204 KB | 115.2 µs, 204 KB |
| `"Hello, " + first + " " + last` | 74.2 ns | 77.7 ns |
| `names.map(n => n + "@example.com")`, 1,000 | 27.2 µs | 28.2 µs |
| `names.join(", ")`, 1,000 names, 9,890 characters | 7.2 µs | 9.3 µs |

- An expression that creates nothing has no charge site, so it pays nothing.
- Finding the exact size first beats growing: 9% less time and 44% less memory
  than the spike's `append` (`spike:951-956`), with the charge included.
- Lists and short strings cost 0% to 5%. The join costs 29%, all of it the pass
  that measures its pieces.
- The test checks each expression's units against A.4, and that one unit fewer
  fails and exactly enough passes.

**What a unit is in Go.** A list slot is 16 bytes. A map entry, 2 units, is
about 50. A unit of text is at most 192 bytes, and 64 for ASCII. So the
published constant is about 200 bytes per unit at worst and 16 to 64 typically.
What is not charged is bounded by steps: a boxed number is 8 bytes. A function
literal creates nothing (B.5).

### B.8 Measuring text, and why weight (L7)

A Go string knows its UTF-8 length, a JavaScript string its UTF-16 length, and
neither its code point count. Any unit gives the same number everywhere. The
question is who scans:

| Unit | Go | Rust | JavaScript | Others |
| --- | --- | --- | --- | --- |
| UTF-8 bytes | free | free | a scan: 1.2 to 1.4 ns per unit in a loop, 0.02 to 0.3 ns through `TextEncoder.encodeInto`, with a floor near 20 ns a call (M) | free in Swift |
| Code points | a scan | a scan, 0.03 ns per byte with the standard library (M) | a scan, about 1 ns per unit (M) | free in Python |
| Weight (the UTF-16 length) | a scan, 0.05 to 0.11 ns per byte (M) | a scan, 0.005 ns per byte for ASCII, 0.14 for other text with a plain count (M) | free | free in Java, C#, Kotlin and Dart |

- Code points make all three implementations scan, so they lose on cost.
- With weight, Go and Rust scan. With bytes, JavaScript scans. Since the owner
  named Rust, two of the three implementations store UTF-8, and this is a closer
  call than the first revision made it (overview R10, D15).
- **Weight still wins, on short text.** A string's weight is at most its length
  in either encoding, so a string under 64 bytes or 64 UTF-16 units has size 0
  in every implementation without a scan. With bytes as the unit, a JavaScript
  string of 22 to 63 units may reach 64 bytes and must be measured, at about 20
  ns a time (M). Names, states, identifiers, addresses and short sentences fall
  in that band, and they are most of what expressions handle.
- On long text the two units cost about the same in total. For one 4,096-unit
  string: weight costs Go 222 to 466 ns and Rust 20 to 566 ns; bytes cost
  JavaScript 85 to 1,269 ns (M).
- The first revision said a JavaScript scan was about ten times slower than
  Go's. That holds for a loop in JavaScript. `TextEncoder.encodeInto` is as fast
  as Go's count on ASCII text, at the cost of writing the bytes to a scratch
  buffer three times the string's length.
- P3 names UTF-8 byte lengths as a Go habit to keep out of the rules. With Rust
  that argument is weaker; the one above does not depend on it.

The JavaScript and Rust figures are from `lab/textsize/other` (`node.txt`,
`rust.txt`), on the same machine as the Go figures below.

`lab/textsize` measures what it costs Go to know the size of a string it only
reads:

| Size of one string (M) | 16 B | 256 B | 4,096 B | 4,096 B, one third non-ASCII |
| --- | ---: | ---: | ---: | ---: |
| UTF-8 bytes | 1.6 ns | 1.6 ns | 1.8 ns | 1.9 ns |
| code points, `utf8.RuneCountInString` | 5.4 ns | 76.5 ns | 986 ns | 2.1 µs |
| code points, eight bytes at a time | 2.4 ns | 15.3 ns | 234 ns | 477 ns |
| weight, eight bytes at a time | 2.3 ns | 13.6 ns | 222 ns | 466 ns |
| size by the rule of A.5 | 1.8 ns | 14.5 ns | 234 ns | 463 ns |

The count reads a word, skips it if it is all ASCII, and otherwise counts
continuation bytes and four-byte leads with two population counts. The lab reads
words through `unsafe.StringData`. Without `unsafe` the count takes 608 ns for 4
KB of ASCII, still under the standard library's (M, `utf16_units_by_word_safe`).

| Operation (M) | One step | By weight | By code points, standard library | By bytes |
| --- | ---: | ---: | ---: | ---: |
| `==`, 12 bytes, equal | 1.9 ns | 2.0 ns | 2.0 ns | – |
| `==`, 256 bytes, equal | 4.6 ns | 32.6 ns | 157 ns | – |
| `==`, 4,096 bytes, equal | 45.8 ns | 483 ns | 2.0 µs | – |
| `==`, 4,096 bytes, differ in the first byte | 2.4 ns | 2.8 ns | 1.9 µs | – |
| `+`, two of 10 bytes | 12.4 ns | 13.5 ns | 12.9 ns | 12.4 ns |
| `+`, two of 48 bytes | 16.1 ns | 23.1 ns | 40.0 ns | 17.6 ns |
| `+`, two of 512 bytes | 90.0 ns | 126 ns | 345 ns | 94.8 ns |

- Text under 64 bytes costs nothing to measure. That covers the names, states
  and identifiers that expressions compare.
- Above that, `+` costs about 40% more, and `==` on long equal strings 7x to
  10x, because Go's comparison is far faster than any count. `==` on strings
  that differ early costs nothing, because the charge is the common prefix.
  Charging by the shorter string's length was rejected: 427 ns against 2.8 ns
  for two 4 KB strings that differ in the first byte (M,
  `Equal/4096_bytes_differ_at_start/by_length`).
- This is the price Go pays so that JavaScript pays none. UTF-8 bytes would
  reverse it, at a higher price per character.

### B.9 Optimizing without changing the counts (L6)

Each optimization in the Go implementation, and how it keeps the counts of A.3
and A.4:

| Optimization | How it charges what the reference charges |
| --- | --- |
| Constant folding | The compiler runs the fold with counters. The folded steps are added to the charge of the operator above. A folded constant with no operator above it is a closure that charges and returns. Units are charged per evaluation, though the value is built once |
| Constant collections | Built once, shared, and charged 1 + `n` units at each evaluation |
| Function literals run in place (B.5) | 1 step where the literal stood, and 2 per element in one subtraction. A call by name charges its 1 step at the call |
| Fused paths | The whole path is charged at once. A read that fails part way gives back the steps of the reads that did not happen. A budget too small for the whole path takes a slow path that charges one read at a time |
| `x in [constants]` as a set | The set maps a value to the number of comparisons the scan makes before finding it |
| `match` on literals as a table | The same, with the arm's position |
| `a + b + c` in one buffer | The sizes of the intermediate strings are sums, charged as if each were built |
| Typed closures | The same charge points as layer 1 |
| A type test decided from a Go type (B.14) | The count the walk would have made, found from lengths. A `[]string` of `n` elements against `string[]` charges 1 + `n` in one subtraction |
| The variant of a tagged union from a table (B.14) | The tag's 1 step, as the reference charges. There is no scan to replace |
| A projection of plain reads (B.15) | One star's visits and reads are charged at once, `n` times 1 plus the reads. A read that fails at element `i` gives back what the later elements would have charged, and a budget too small for the whole takes the slow path |
| A string literal held by the program (B.12) | 0 steps and 0 units, which is what the reference charges a `String` node (A.3) |

`lab/counts` builds three of these as the reference, as an optimization free to
charge less, and as one that preserves the counts. A test runs the reference and
the preserving form at every budget from 0 to 12 and requires the same value,
error and steps left.

| Case (M) | Reference | Free | Preserving |
| --- | ---: | ---: | ---: |
| `x * (60 * 60 * 24)`, folded | 22.0 ns | 8.1 ns | 8.1 ns |
| `flag && 60 * 60 * 24`, folded with no operator above | 17.5 ns | 3.6 ns | 4.7 ns |
| four-field path, fused | 22.7 ns | 21.9 ns | 22.1 ns |
| the same, third field missing | 15.0 ns | 12.1 ns | 12.9 ns |
| `role in [8 constants]`, as a set | 9.1 ns | 8.3 ns | 8.2 ns |

**The cost of L6.** At run time: nothing measurable, except 1.1 ns for a bare
folded constant. The costs are elsewhere:

| Cost | Detail |
| --- | --- |
| Stopping early is forbidden (A.8) | expr turns `filter(xs, p)[k]` into a search that stops (`expr:program.go:1285-1356`). v3 cannot. I recommend accepting that: `find`, `any` and `all` stop by definition |
| The cost table is frozen | A rate that turns out wrong, or a faster algorithm with a different formula, is a change to the language. See open question 4 |
| Tests | Every optimization needs the budget sweep of `lab/counts`, and the conformance suite pins steps and units for every case |
| Go scans long text | B.8 |

### B.10 Runtime errors, `try` and the trace

**Decision: closures return errors. No evaluator path panics.** `lab/errors`
builds the same trees with panic and recover (`spike:99-105`, `:1226-1234`) and
with error returns. "Net" is error returns plus one deferred recover in `Run`.

| Case (M) | panic | error | error + net |
| --- | ---: | ---: | ---: |
| `a + b + c + d < e` | 17.1 ns | 12.9 ns | 16.0 ns |
| the same, typed closures | 14.5 ns | 10.6 ns | 12.9 ns |
| count over 10,000 maps, reading a field | 132.9 µs | 118.1 µs | 116.4 µs |
| a function called 21,891 times | 323 µs | 297 µs | 294 µs |
| `try { a.b.c } catch { "none" }`, no failure | 17.1 ns | 12.7 ns | 15.1 ns |
| error raised 4 closures deep | 184 ns | 38.4 ns | 41.2 ns |
| error raised 40 closures deep | 564 ns | 92.6 ns | 107 ns |
| the same `try`, catching a failure | 173 ns | 33.1 ns | 35.5 ns |

Error returns were never slower on the success path and are 5x cheaper on
failure. The 4 ns that panic loses on short expressions is its deferred recover.
The cost is code: about 1.6x the lines per operator, 600 lines in all (I). The
function called 21,891 times is a recursive one, which the language does not
admit (E6). It stands for call-heavy work, and it was run with the frames that
B.5 has since removed.

The two `try` rows were run with the call form of `try` that the earlier
revisions had, with the fallback as a second argument. The tree is the one
`try { e } catch { d }` compiles to: a body closure, a test of the returned
error, and a second closure. The lab tested
the error's class, and the design now tests its code, which is one comparison
either way (I). So the rows stand for a `try` with no name. Not measured: a
`catch (err)`, which builds a map of four entries, and a failure that writes
a trace. Section 2 has the cases.

| Rule | Detail |
| --- | --- |
| The net | `Run` has one deferred recover, 3 ns. A panic in a host function becomes `function-panic`, class `host`, and a bug in the evaluator becomes the failure `05` A.6 names for one. `catch` catches neither (A.1). The run records which host function it is inside, so the error names it. A run object that saw a panic does not go back to the pool |
| Positions | Each closure that can fail captures its node's span, two `uint32`s. An error is allocated only on failure. Line and column, counted in code points (T5), are computed when the error is formatted |
| Host function errors | The call node wraps the returned `error` with the call's span and the function's name, and keeps the original as the cause. The code is `function-error`, class `host`. Its message is the text of the host's error, which a `catch (err)` gets at no charge (A.10) |
| The error object | One allocation, only on failure. It holds an index into the table of codes, the span, the name of a template with its parameters as references, a reference to the data of a `fail`, and the cause. The table of codes is generated from `messages.json` (`10` 2.9) and gives each code its templates and whether `catch` catches it. No message is written by hand in the evaluator |
| Messages | Built when an error is formatted for the host, or when a `catch (err)` binds it. Never on the way |
| `not-conforming` (`07` 2.3) | Class `value`, and `catch` catches it. The walk of B.14 returns a small failure record: the path, the test that failed, which gives what was expected, and the kind that was found. The path is at most 64 segments, each an index or a key, held in the run's scratch space: the walk writes a segment before it enters a level. `as` wraps the record in an error with its node's span, allocated only on failure. `is` drops it and allocates nothing. The path is made into text when the error is formatted, as line and column are. The path is also a field of the diagnostic (`05` A.6), and of the error value when a `catch (err)` binds it |

**`Try`, node 26 (D48).** Decision: one closure that holds the body, the
handler and the slot of the name, and tests the error the body returns. No
deferred function (I).

| Step | Detail |
| --- | --- |
| Enter | The closure keeps four words in locals: the depth count, the heights of the argument stack and of the scratch buffer, and the mark of the host function in progress. It adds 1 to `run.tries`, calls the body, and takes the 1 off again |
| The body gives a value | It is returned. The cost over the bare body is one test for a nil error and the two writes of `tries` |
| The failure is not caught | One test of a bit on the error object, set from the table of codes when the object was made. The error is returned as it is, and the calls above add their trace entries |
| Caught by `catch { }` | The four locals are written back, and the handler runs. That is all of unwinding: the closures in between returned one after another, and each call took its own count off the depth as it returned. The error object is garbage, and nothing else was built |
| Caught by `catch (err)` | First the error value. The message is built from its template, or taken from the `fail` or from the host's error. `charge` and `alloc` take the sums of A.10, one call each. One `map[string]any` of four entries is made, with the path as a `[]any` copied from the segments in scratch, and it is written to the slot. Then the locals are written back and the handler runs |
| A failure in the handler | Returned like any other. `tries` is already back, so the error is quiet only if an outer `try` is in progress (below) |
| Layer 2 | A `try` whose body and handler have the same typed form is a typed node. Any other is boxed |

The body and the handler are branches, compiled as blocks (B.5). What the body
wrote to its slots stays there, and the handler cannot name it.

**The trace (10-5).** Decision: entries are written into a buffer of the run,
on the error path only. Nothing is kept while a run succeeds (I).

| Matter | Detail |
| --- | --- |
| Where | A call by name, a library function that runs a body in place (B.5) and the loop of an `Each` (B.15) each have a branch for a returned error already. In it they call one function that appends an entry: the node's span, a pointer to its name, and the index the loop is at. The success path gains no instruction |
| Storage | A slice of fixed-size entries in the run object, with room for D + 100 (A.10), kept between runs. An append allocates nothing |
| A failure that will be caught | Gets no entries. An error object is marked quiet when it is made, if `catch` catches its code and `run.tries` is above zero: the innermost `try` whose body is in progress will then catch it. The append tests the mark. So a `try` that gives a fallback inside a loop pays for no trace |
| Leaving the run | `Run` copies the entries into the diagnostic in one allocation, reversed so that they run from the outermost in, and empties the buffer before the run object goes back to the pool (B.11) |
| A limit error | Takes the same path. Its entries are those of the place where the chunked counter stopped (B.6), which A.1 leaves free |

What this should cost, none of it measured: a failure that leaves `n` calls
pays `n` appends of one small entry each on top of the 38 to 93 ns of the table
above, and a run that does not fail pays nothing. A caught failure with a name
pays one map of four entries and at most one string and one list.

### B.11 Concurrency and run state

A compiled program is a tree of closures over immutable data. Nothing in it is
written after compilation except the one-entry accessor caches of B.3, which are
atomic. Per run: the two counters, the depth, the bound variables, the slots,
the argument stack, the count of `try` bodies in progress, the trace buffer
(B.10) and scratch space (sorted keys, the filter bitmap, the path of a type
test, the values of a projection). There is no frame stack (B.5). The spike
allocates its run state as one 384-byte object (`spike:1190-1200`).

**Decision: the run object comes from a package-level `sync.Pool`.**
`lab/runstate` runs `gate.failed > 0`:

| Run state (M) | Serial | 16 goroutines, per op | Allocs |
| --- | ---: | ---: | ---: |
| new 384-byte object per run (the spike) | 71.1 ns | 79.5 ns | 1 |
| `sync.Pool` | 23.8 ns | 1.8 ns | 0 |
| object owned by the caller | 14.4 ns | 1.1 ns | 0 |

Before an object goes back, its variables, slots, argument stack, scratch
buffer and trace buffer are cleared, so the pool keeps no host data alive. An
object whose buffers grew past a small bound is dropped. Under B.5 the slot
array has one slot per binding in the text, so the clear grows with the size
of the program and not with the calls it made.
`lab/runstate` ran a program with no bindings, so that cost is not measured
(I). A variable is fetched from the host on its first read in a run
and kept in the run object. Alternative: a `Runner` the caller owns. It is 9 ns
faster and adds an object to the API that must not be shared.

### B.12 Compile-time work and map order

| Work | Detail |
| --- | --- |
| Names | Locals to slots, one per binding in the program (B.5). Variables to indexes. Library functions and methods to direct references with arity checked (`spike:363-392`, `:792-797`). A call by name to its callee's body and parameter slots |
| Folding and constants | By calling the function the run would call, as in expr (`expr:program.go:160-168`), with counters (B.9). A fold that fails is left to fail at run time |
| Constant operands | `x > 0` gets a closure that holds the constant as an int and as a float. In the spike a JSON float compared with an int literal takes the general path (`spike:658-673`) |
| Library functions with constant arguments | A function may supply a binder that runs at compile time, such as one that prepares a constant pattern |
| Accessors and forms | Struct field indexes, fused paths, typed forms, typed bindings (B.1, B.3, B.4) |
| Type tests | The type text after `as` or `is` is parsed, each name is replaced by the type the environment gives it, and the result becomes a test (B.14). No type text is read during a run |
| String literals (D46, `08`) | The lexer settles line ends, removes the indentation of a text block and reads escapes, or reads none under the prefix `r`. Nothing of this is done in a run. A `String` node of any form compiles to a closure that returns one Go string the program holds: no charge site. A `Template` node compiles to the same closure whichever delimiter wrote it. The compiler does not read `form` |
| Failures | The table of codes and templates, generated from `messages.json`, is linked in. Each node that can fail holds its span, and a call node holds its name for the trace (B.10) |
| Projections | The reads after each star are fused as a path is (B.3), one path for each star (B.15) |
| Sizes | Slot count, variable count and argument depth, so the run object's buffers are sized once. With no frame stack, the slots and the argument stack do not grow during a run (I). Also the largest total the calls can reach against the depth limit (B.6) |

**Map order (T2).** Go compares strings by byte, and byte order of UTF-8 is code
point order, so a sort of the keys in the run's scratch space is enough. For a
map of 8, 64 and 512 keys, an unordered walk takes 32 ns, 334 ns and 2.6 µs, and
a sorted one 141 ns, 1.9 µs and 32.6 µs with no allocation (M, `MapOrder` in
`lab/hostread`). Each arithmetic operator is its own closure, so the Go compiler
cannot fuse a multiply and an add (A.2).

### B.13 Observability

v2's observer is called from the dispatch loop on every instruction
(`v2:vm/vm.go:667`, `v2:vm/observer.go:78-94`). **Decision: remove it.** A run
reports the steps and units it used. Tracing, if wanted later, is a compile
option that wraps each node's closure, so a program compiled without it has no
hook and no check for one (about 80 lines, I). This is not the trace of a
failure (A.10, B.10), which every run has and which costs a run that succeeds
nothing.

### B.14 Type tests: `as` and `is`

Nothing in this section was built or measured. All of it is (I).

**Decision: a type compiles to a test once, when the program is compiled. One
function walks a value against a test, for the two operators and for
`risor.Conform`.**

| Matter | Detail |
| --- | --- |
| When | The compiler parses the type text after `as` or `is` and replaces each name by the type the environment declares for it. A named type is an abbreviation and never refers to itself (`07` 2.5), so this ends. A name that nothing declares is `undefined-name`, a type deeper than 64 is `check-limit` (`03` 2.17), and two or more records in one union that have no tag are `untagged-union` (`09` 2.3). The compiler makes the three tests itself, so a program compiled without the checker is refused in the same way |
| The run-time form | A tree of small immutable structs, one for each node of the type. A node holds the kinds it allows as a bit set. A record node also holds its declared fields sorted by key, each with a required flag and a test, and its rest: a test, `any` or none. A list node holds its element test. A literal set holds its strings and the length of the longest. An opaque node holds its name. A place whose type is `any` is a nil test, which the walk does not call |
| A tagged union (D47, `09` 2.7) | The map kind of a node holds a record or a union. A union node holds the tag's key and a Go map from each tag string to its variant. A variant is a record node whose sorted list of declared fields leaves the tag out. The walk charges 1 for the map, charges 1 and looks the tag up, finds the variant in the table, and runs the record walk over it from its first field. The record walk is reused unchanged, and the tag is counted with the declared keys present when the size of the map is compared (A.3) |
| Sharing | A named type compiles once for an environment, and every program that names it points at the same test. Tests are part of the compiled program and are safe to share between runs (B.11) |
| The walk | `conform(run, v, test)` charges 1 through the counter of B.6, then looks at the value. It loops over elements and fields and recurses only into a list or a record, adding 1 to the depth count as it enters (A.6). Its order is `07` 2.3 and its charges are A.3 |
| The result | `as` returns the `any` it was given: the same map, slice or struct pointer. `is` returns a bool, and in layer 2 it is a `boolNode`. `x as int` in layer 2 is an `intNode`: it is the `asInt` of B.1 with `not-conforming` as its error |
| What the evaluator now carries | The type text parser and this form. Before `07` only the checker needed a type at all. The parser, about 300 lines, moves from the checker's package to the core, so that a program compiles and runs without the checker (`07` section 5) |

**Host values are read through B.3, and nothing is converted.**

| Value | How the walk reads it |
| --- | --- |
| `map[string]any`, `[]any` | In place: `len`, an index, a lookup for each declared field. Other keys are visited by ranging over the map only when the rest is a type other than `any`, or when an exact record has failed. Go ranges over a map in no fixed order, which is why A.3 makes the count free of order |
| A scalar | One type switch, the read barrier's. A value the barrier would refuse, such as an `int64` out of range or a NaN, fails the test and raises no `number-range` failure (A.3). Nothing is boxed and no number is rewritten |
| A struct, or a pointer to one | Through a plan for the pair of Go type and test, built the first time the pair is seen and kept as B.3 keeps its plans for types. The plan maps each declared field to a struct field index, lists the struct fields that are other keys, and marks each sub-test the Go type already settles: a `string` field conforms to `string`, and an `int32` field to `int`. The walk reads a field as a `reflect.Value` and tests it there, so a scalar field is never made into an `any`. The steps are those of A.3, whatever the plan skips (A.8). Against a tagged union the tag field is read first, as a string, and the plan is for the pair of Go type and variant (`09` section 5). A struct type whose tag field is not a string settles the test at once: it fails at the tag |
| A typed slice or map | Through the view of B.3. Where the element type settles the test, as `[]string` against `string[]`, no element is visited and the count comes from the length. A `[]int64` against `int[]` still reads each element, for the range. An element of a `[]User` is tested through a pointer to it and not copied |
| A nil pointer or nil interface | Null, as in B.3 |
| An opaque value | The per-type entry of B.3 gives its name |
| A value at a place whose type is `any` | Not read at all |

**Every failure (`10` 2.11).** `risor.ConformAll(v, t, max)` is the same walk
in a second mode: it goes on after a failure, reaches the other keys of a
record whether or not its declared fields passed, takes them in code point
order, and stops at `max` diagnostics. It is not a run and charges nothing, so
it has no count to keep exact. Its first diagnostic is the one `Conform` and
`as` report, which a test holds for every case of the suite. About 80 lines
over the walk (`10` section 3).

The cost per value is not known. It should be near the cost of the read that
finds the value, 5 ns for a map entry and 5 to 17 ns for a struct field (B.3),
since the test itself is a switch on the kind. Section 2 has the cases to run.

### B.15 Projections: the node `Each`

Nothing in this section was built or measured. All of it is (I).

**Decision: a projection compiles to one closure. It holds one level for each
star, runs the levels as nested loops, pushes each result onto the run's
scratch buffer, and allocates the list once, after one charge.**

| Matter | Detail |
| --- | --- |
| The node | `Each` has `x`, the reads and `optional` (`13` 5.2). The compiler cuts the reads at each later star into levels. A level holds its star's span, whether the star is `?.[*]`, and its reads fused into one path, as B.3 fuses a chain of reads: map lookups, struct accessors and indexes, with a null exit at each `?.` |
| The receiver | Evaluated once. Nil under `?.[*]` leaves through the null exit of the chain that holds the projection (B4). A `[]any`, or a slice or array behind the list view of B.3, is iterated by `len()` and `at(i)`. Anything else is the failure of A.3, with a hint when the value is a map (`13` section 7) |
| A level | A loop over the elements by index. For each: the element is passed to the fused path. A null exit pushes one nil and goes on to the next element. On the last level the path's result is pushed. On any other level the result is the receiver of the next level, which is a call of the same function one level down. So Go recurses once for each star, at most 100 deep (A.6), and not at all for a projection with one star |
| Charging | A level charges its visits and reads together: `n` times 1 plus the reads of its path, in one subtraction when the path has no index expression. A read that fails at element `i` gives back what the elements after `i` would have charged, and what the failed path did not reach. A null exit gives back the reads it skipped. A budget too small for the whole takes the loop that charges one element at a time (A.8, B.9) |
| The scratch buffer | One `[]any` in the run object, used as a stack. The closure notes its height, the levels push, and at the end the closure calls `alloc(1 + k)`, makes a `[]any` of exactly `k`, copies, and cuts the buffer back. A projection inside an index expression of another, or inside a body that a read calls, stacks above it. The buffer grows by doubling and is kept between runs up to a bound (B.11) |
| No binding | An element is passed as an argument of a Go function, not written to a slot. That is the saving over `map(o => o.f)`: no slot write and no call of a body closure for each element |
| Failure | A read returns an error, and the loop appends the trace entry of its star with its index (B.10). The buffer is cut back by the `Try` that catches, or by the run's end |
| Host values | An element of a `[]User` is passed as a `*User`, and the path reads it through accessors (B.3). The result is always a `[]any`, so a scalar field read from a struct is boxed, one allocation for each element. A typed result list waits with typed slots (B.2) |
| A method after it | `orders[*].lines[*].qty.sum()` builds the list and calls `sum`. Fusing the two is allowed when 1 + `k` units are still charged (A.4, A.8). It waits for a measurement |

`xs[*].f` and `xs.map(x => x.f)` must give the same list for the same steps
less the 1 of the call, the 1 of the literal and 1 for each element, so the
differential test of B.1 runs each projection of the suite against its `map`
form as well. I expect the projection to cost less than the inline literal of
B.5, which took 7.2 ns for each element. Section 2 has the cases.

## Part C. Notes for a TypeScript implementation

All of this is inference. Nothing here was built or measured.

| Topic | Note |
| --- | --- |
| Closure compilation | Carries over. A node is a function of the run state. Engines compile such closures well, but a call site that sees many different closures is not inlined, so expect less gain from fusing than in Go. Generating source text is faster and is blocked by content security policies, so it cannot be the only path |
| Typed closures | Not needed. A JavaScript number is not boxed the way a Go `float64` in an `any` is. One layer |
| Errors | A function returns one value, and a result object per node allocates. Throw instead: one JavaScript `try` in `run`, and one in each `Try` node, which tests the code of the thrown object and throws it again when `catch` does not catch it (A.1). Throw a plain object, not an `Error`, to skip the stack capture. A.1 fixes which failure wins, not how it travels. The error value of `catch (err)` is a plain object with the four keys always in one order, so it has one hidden class |
| Trace (A.10) | A JavaScript `catch` adds the entry and throws again. Put one `try` around the whole pass of a library function or of a star, with the index in a variable outside it, and not one around each call of the body: then a pass enters one `try` whatever its length. A call by name needs one of its own. What entering a `try` that does not throw costs in the engines is not measured (`10` section 8). If it is not free, the alternative of 10-5 stands for this implementation. The quiet mark of B.10 carries over |
| Projections (A.3, B.15) | The same levels and the same scratch array, which is cut back by setting its `length`. The receiver must pass `Array.isArray`. A missing property and one that holds `undefined` are told apart in a read, as everywhere (B2) |
| Tagged unions (A.3, B.14) | The tag is one own-property read, and the table from tag string to variant is a `Map` (`09` section 5) |
| String forms (`08`) | Nothing at run time. The delimiters and the indentation are ASCII, so the lexer needs no code point care for them beyond the columns of a span |
| Reading in place | Plain objects, arrays and `Map`s with string keys. Read own properties only, so that `constructor` and `toString` are not fields. JavaScript's `undefined` and `null` both read as null. A missing property and a property holding `undefined` must be told apart (B2). Getters, proxies, class instances and symbols are for the binding to rule in or out |
| Records | A record the evaluator builds is an object with a stable set of keys, which engines give a hidden class and read through an inline cache. This takes the place of Go's struct accessors |
| Numbers | Every number is a double. The range rule is `Number.isSafeInteger` on both operands and on the result of `+`, `-` and `*`, and a finite test otherwise. `-0` must render as `0`. A `bigint` cannot enter |
| Text | `length` is the weight of A.5, free. Length, slicing and indexing by code point are scans that step over surrogate pairs. Ordering by code point is not the built-in `<`, which orders by UTF-16 unit. A lone surrogate in a host string reads as U+FFFD, one for each such unit (`02` 2.6) |
| Map order | `Object.keys` gives insertion order with integer-like keys first. Sort with a code point comparison. A cache of sorted keys per object per run is worth having |
| Deadline | Evaluation is synchronous, and an `AbortSignal` cannot change during it unless a host function changes it. Read `performance.now()` at each 1,024-step check. Stopping a run from outside needs a worker |
| Function literals (A.9) | One array of slots on the run object, indexed by numbers the compiler assigns, as in B.5. A function literal is a node compiled once, and evaluating it creates no JavaScript closure. The other way is to make each literal a JavaScript function and let the engine keep the frames. That allocates a closure each time a literal is evaluated, and A.9 says nothing needs it |
| Type tests (A.3, B.14) | The same tree of tests, built when the program is compiled, and one walk shared with `conform`. `int` is `Number.isSafeInteger`, and `float` is a finite number. A NaN or an infinity fails the test. A list is `Array.isArray`. Other keys come from `Object.keys`, which allocates an array of names: that is scratch space and is not charged (A.4), and it is needed only for a rest type other than `any` or a failed exact record. The size of a plain object is not stored, so finding that an exact record has another key means counting its own keys. The count stops as soon as it passes the number of declared keys present, so on success it is covered by the declared lookups, and on failure every other key is charged (A.3). The evaluator ships with the type text parser though the checker comes last (`07` section 4) |
| Run state | No pool is needed: one thread. Keep one run object per program and allocate another only if a host function re-enters |
| Limits | The two counters and the charge points are the same code. Units are counted, not enforced by the engine, as in Go |
| Does not carry over | Reflection, the read barrier's Go kinds, struct accessors, generic adapters, `sync.Pool`, error returns, typed closures |

The hard parts, in order: code point operations on UTF-16 strings, sorted key
iteration, the integer range rule on every arithmetic node, and a deadline
without preemption.

## Part D. Notes for a Rust implementation

All of this is inference, except the text-size figures of B.8. No evaluator was
built in Rust.

| Topic | Note |
| --- | --- |
| Execution | Two shapes are workable. One is a tree of boxed closures, `Box<dyn Fn(&mut Run) -> Result<Value, Stop>>`, as in Go. The other is a `match` over an enum of nodes held in one vector. In Go the second is the slow one, which is why Part B compiles to closures. In Rust a `match` on a dense tag is a jump table and the nodes sit together in memory, so the answer is not known. Build the reference evaluator as the enum walk, which is the simplest, and measure closures against it before choosing |
| Values | An enum: null, bool, number, string, list, map, host value. A number is an `f64` held in the enum, not behind a pointer. The cost that layer 2 removes in Go, a number boxed in an interface, does not exist. One layer |
| Whole numbers | As in A.2. An implementation may carry an `i64` beside the `f64` for whole numbers, as Go's does, if no result differs. Rust does not fuse or reorder floating-point operations on its own, which is what A.2 requires |
| Function literals (A.9) | Slots are one `Vec` of values on the run, sized when the program is compiled and indexed by numbers. No closure is built, so nothing borrows a frame and no lifetime ties a function to the scope that defines it. The frame stack was the one place where the evaluator would have lent a frame across a call. A program with `k` function literals has at most `k` calls in progress, and the largest total its calls can reach against the depth limit is known when it is compiled. So an implementation can state the stack one program needs before it runs it, and it is never more than the 2D + 100 of the stack row below |
| `if` (D35) | In the enum walk, the node holds its tests and branches as a list and the walker loops along it. An `else if` chain, like a run of `const` lines and an operator chain, must not recurse (`02` 2.16) |
| Type tests (A.3, B.14) | A test is an enum tree owned by the compiled program. The walk reads through the trait for host values: a kind, a length, an element by index, a field by name, and the keys of a map. So the trait must give the keys and the size of a record, which rendering needs as well. The walk recurses at most 64 levels and counts each against the depth limit, as every recursion here does. The path of a failure is a fixed array of 64 segments in the run's state, so a failed test allocates nothing until its message is formatted. A derived record knows its own field types, so it can settle a test of its scalar fields without reading them, as Go's plan does. The core crate carries the type text parser before the checker exists (`07` section 4) |
| Borrowing | A value either belongs to the run or is lent by the host for the lifetime of the variables passed to `run`. The compiler checks that a run and its result do not outlive those variables. This is "read in place" stated as a type |
| Memory | No collector is needed (overview R3): nothing a run creates can refer to itself. Take every value the run creates from one region and release the region when the result is dropped. Allocation is a pointer bump, release is one call, and the region is at most the allocation budget times a constant (overview R4). Reference counting is the alternative. It frees earlier, costs a count on every copy, and needs a release that does not recurse (overview R6) |
| The result | The run returns an output that owns the region and borrows the variables. The host reads it in place, or copies it into an owned value. The copy is a walk, so it is iterative or bounded by the depth limit |
| Host records | There is no reflection. A host value is read through a trait (`05` Part D): a kind, a length, an element by index, a field by name. The trait is implemented for the standard collections, for `Option`, and for JSON values, and derived for a struct. A derived field read is a `match` on the name, and the compiler of a program can resolve a name to a field number once. That is the speed of Go's struct accessors without a typed layer |
| Map order | Byte order of UTF-8 is code point order, so `<` on `str` is the specified order and a `BTreeMap<String, T>` already iterates in it. A `HashMap` is sorted for each ordered walk. The charge is the same either way (overview S16) |
| Text | A `str` is always valid UTF-8, so the rule for invalid host text (`05`) has nothing to check. Length and indexing by code point are scans. The implementation owns the type of the strings it creates, so it can store a string's weight and code point count when it builds it, and an "all ASCII" mark that makes indexing constant time. Then only host strings of 64 bytes or more are ever scanned |
| Errors | `Result` and `?`. A failure that `catch` catches and one that it does not are different variants, so a `Try` node matches one and passes the other. The second variant holds the limits, a cancellation, `function-panic`, a failure inside the implementation and `missing-variable` (A.1). A language error is never a panic. The error value of `catch (err)` is a map of four entries in the run's region, charged before it is taken |
| Trace (A.10) | `map_err` at each call by name, around each pass of a library function and around each star adds the entry. It is free on success. The entries go into a vector of the run's state with room for D + 100, so a failure allocates nothing for its trace until the diagnostic is made |
| Projections (A.3, B.15) | One more node of the enum, holding its levels. The scratch is a `Vec` of values in the run's state, and the result list is taken from the region after the charge. The walker recurses once for each star, which the 100 levels of the source bound, so the stack row below needs nothing new |
| Tagged unions (A.3, B.14) | One more case of the test enum: the tag's key and a table from tag string to variant (`09` section 5). The path and the depth are as for a record |
| A host function that panics | Caught at the call and reported as `function-panic`, where the build unwinds. In a build where a panic aborts, it ends the process, and the binding must say so |
| Stack (overview R5, R6) | An overflow ends the process. Every recursion carries a counter checked against a specified limit: the parser, the checker, the compiler, the evaluator, `==`, rendering and encoding. Release is not recursive. A spawned thread has 2 MiB of stack by default. With `02` 2.16 a run recurses at most 2D + 100 levels, 500 at the default depth. A small walker written for `02` used 287 bytes a level (ran, `02` 2.16), so there is room, but the real figure is the evaluator's. The implementation publishes the stack it needs at the specified limits and tests the deepest legal program on a 2 MiB stack, in a debug build as well |
| Deadline | Read `Instant::now()` at each 1,024-step check, or load a flag another thread sets. Rust can observe a signal during a run, as Go can |
| Run state and threads | A compiled program is immutable and `Send + Sync`. A run's state is a value on the caller's stack, or an object the host keeps and reuses. No pool is needed |
| Host functions | A closure that takes the call handle and a slice of lent values and returns a `Result`. Typed adapters are trait implementations over function arities, resolved when the host is compiled. S4's choice of adapter at registration is a Go matter |
| Other hosts (overview R8) | Keep the core free of threads and of a clock it reads directly, so it builds for WebAssembly and behind a C interface. The host passes data in as JSON there, and the copy is the host's cost, not charged (L4) |
| Does not carry over | Reflection, the read barrier's Go kinds, the typed layer for scalars, `sync.Pool`, adapters chosen at run time |

The hard parts, in order: the lifetimes of lent values across the whole API,
the trait for host records and its derive, the stack bounds, and code point
operations on UTF-8 text.

## 2. Benchmark plan

Implementations: expr v1.4.1, Risor v2 (`risor.Run`), the spike, v3 layer 1 and
v3 layer 2. The harness is `experiments/v3/bench`, with an agreement test before
every benchmark.

| Group | Cases |
| --- | --- |
| A. Workflow | The eight expressions of `bench/bench_test.go`, with a small scope and with 105 KB extra |
| B. Compute | The programs of `bench/spike_test.go` that the language still admits |
| C. Go types | One field, a four-field path, `u.Age > 18 && u.Role == "admin"`, filter over 1,000 `[]User`, index of `[]string`, lookup in `map[string]int` |
| D. Calls | Native, adapter and reflection |
| E. Failure | A failing read. A `try` whose body gives a value. A `try` that catches with `catch { }` and with `catch (err)`, for a failure with a built message, for `fail` with data, and for `not-conforming` with a path of 3 and of 64 steps. A failure that leaves the run under 1, 10 and 100 calls in progress, with its trace, and the same failure caught by a `try` at the top. A `try` that gives a fallback for each of 10,000 elements of a `map`. Each limit reached in a loop |
| F. Overhead | A constant, one variable read, and group A on 16 goroutines |
| G. Compile | Source to program with checking, for groups A and B. A program that is one text block of 4 KB and of 64 KB, against the same text as one backtick template, for the lexer's pass over the indentation (`08` 2.8) |
| H. Limits | Groups A and B with the allocation budget on and off. `+`, `==`, `join` and a template on text of 16, 256 and 4,096 bytes |
| I. Counts | Every conformance case in layer 1, in layer 2 and with each optimization off |
| J. Function literals (B.5) | `nums.filter(x => x > limit)` over 10,000 with the literal written in place, and with a `const`-bound literal passed by name. A `const`-bound literal called by name 10,000 times, with one argument and with three. The nested case of B.5. A program with 200 bindings that takes a short path, for the cost of clearing the slots. Each against the pooled frames of the earlier revision |
| K. Type tests (B.14) | `body as Order` and `body is Order`, where `Order` is a record with a list of line records, on a parsed JSON body (`map[string]any` and `[]any`) of 10, 1,000 and 100,000 values, and on host structs (`[]Order` holding `[]Line`). `xs as string[]` on a `[]any` and on a `[]string` of 10,000. A failure at the first line and at the last, under `as`, under `try` and under `is`. An exact record with one other key among 1,000. A field of type `any` that holds 100,000 values. A list of 10,000 values against a tagged union of 2 variants and of 50, with the values spread over the variants, and against one variant alone. Baselines: the program's own reads of every field without the test, and the same check written by hand in Go |
| L. Projections (B.15) | `orders[*].id`, `orders[*].lines[*].sku` and `orders[*].lines[*].qty.sum()` over 10 and 10,000 orders of 4 lines, on parsed JSON and on a `[]Order` of structs. Each against the same query written with `map` and `flatMap`. A projection with a `?.` that skips half the elements. A failure at the last element, for the charge given back |

| Target | Measure |
| --- | --- |
| Against expr | No case in A or B slower, with every limit on. A field read from maps at or under 1.0x, with 0 allocations |
| Go types | A typed struct field read within 1.2x of the same read from a map |
| Against v2 | Group B at least 3x faster than the VM floor |
| Step budget | Under 10% on every case |
| Allocation budget | Under 5% on every case in A and B |
| Text | Under 50% on creating text of any length. No scan of a string under 64 bytes, checked by a counter in a test build. `==` on long equal strings has no target and is reported (7x to 10x in the lab) |
| Counts | Steps and units equal across layer 1, layer 2 and every optimization setting, for every conformance case. For each case, a run at every budget from 0 to its full cost gives the reference's outcome |
| Deadline | A passed deadline is noticed within 1,024 steps, tested with a stub clock |
| Scaling | Throughput on 16 goroutines at least 12x one goroutine |
| Failure | A failing short expression under 200 ns. A `try` whose body gives a value within 2 ns of the bare body. A failure caught by `catch { }` no slower than the 33 ns of B.10. A failure that leaves the run pays for its trace in proportion to the calls it leaves, with one allocation for the whole trace. No case of groups A to D is slower for the trace, to be checked against a build with the trace compiled out |
| Function literals | A `const`-bound literal passed by name within 1.1x of the same literal written in place. A call by name no slower than a call through a pooled frame. A pooled run of the 200-binding program within 1.5x of the pooled run of B.11 |
| Type tests | On JSON-shaped data, a test costs no more per value than the read that finds the value, and allocates nothing when it passes. On host structs, no allocation for a scalar field. `[]string` against `string[]` takes the same time at any length. A field of type `any` costs nothing whatever it holds. A failure under `is` allocates nothing, and under `as` one error. The time for each value against a tagged union does not grow with the number of variants, and a value that conforms costs no more than 1.2x what it costs against its variant alone |
| Projections | No slower for each element than the same query with a literal run in place (B.5), and one allocation for the result on JSON-shaped data. Steps as A.3 gives them, checked against the `map` form |

Group J does not exist yet in any form. `lab/lambda` has only a literal written
in place, run under the earlier rule (B.5). It should gain a call by name and a
literal passed by name before the (I) marks of B.5 are removed. The three
targets in its row are guesses to be corrected by the first run (I).

Group K does not exist either, and no lab package covers it. `07` section 10
lists the walk's cost per value, on a host struct and on a parsed JSON body, as
a measurement still to make. A lab package for the walk over hand-built tests
should come before B.14 is built into the compiler. The first target in its row
is a guess (I).

Group L does not exist, and neither do the new cases of group E. `lab/errors`
has a `try` that tests a class and builds nothing. Before B.10 is built it
should gain three things: the test by code with the quiet mark, a
`catch (err)` that builds the map, and a failure that writes its trace under
1, 10 and 100 calls. A lab package for B.15 over hand-built levels should
stand beside `lab/lambda`, since the two answer the same question. Every
figure in the two new target rows is a guess (I).

## 3. Alternatives rejected

| Alternative | Why not |
| --- | --- |
| Count iterations only, and list size as a gap (the first draft) | Overruled by L2 and L3. Charging size costs 0% to 5% outside long text (B.7) |
| Counts free to change between versions (the first draft) | Overruled by L6. Fixing them costs nothing measurable at run time (B.9) |
| Limit live memory | It depends on the collector, so it is not deterministic (L3) |
| Count bytes | Go has no allocator hook, and bytes differ between implementations |
| Text in UTF-8 bytes, or in code points | With bytes the TypeScript implementation measures every string of 22 units or more. With code points all three scan (B.8) |
| Charge `==` by the shorter string | Scans strings that differ at once (B.8) |
| Charge a filter by its input length | Charges for memory never taken (B.7) |
| One Go type for every number in a result | Copies every result (B.2) |
| 64-bit integers | N2 |
| Bytecode VM over host values | Nobody has built one, so the split between dispatch cost and object-model cost is unmeasured. Closures already run the compute-heavy cases 2.8x to 4.3x faster than the VM (`experiments/v3/README.md`), and a VM needs a compiler, an instruction set and a disassembler: 7,030 lines today in `v2:compiler`, `bytecode`, `op`, `dis` |
| Tagged value struct | 29% slower on reads, two list representations (B.2) |
| Normalize numbers at each use | One missed site is a wrong answer (B.3) |
| Reflection for every host function | 15x slower, 4 to 8 allocations (B.4) |
| Frames for calls, pooled or on a per-run stack | No program can need them: no function literal has two calls in progress (A.9). A pooled frame was also 1.8x slower than a slot of the one frame, measured before D34 (B.5) |
| A closure object for a `const`-bound function literal, capturing by reference or by value | Nothing takes a function literal as a value, and its body can read the slots (A.9, B.5). Capture by value was the earlier revision's choice |
| Read the type text at each `as` or `is` | A type is text of the program, so it is parsed once (B.14) |
| Convert a host value to maps and slices, then test it | It copies what `as` promises to give back unchanged, and it breaks reading in place (`07` 2.3, B.3) |
| Stop at the first failing other key of a record | Three implementations have three map orders, so the steps and the path would differ (`07` section 8, A.3) |
| A `number-range` failure when the walk meets a value that cannot enter the language | Among other keys it would end the walk at a point that depends on the map's order, and a `catch` would then see a count that differs by implementation. `is` could also fail. The value does not conform, and that is all (A.3) |
| `catch` that decides by the class of a failure | The class of a null, kind or field failure needs the checker's sites. Two runs of one program on the same values would differ by what a checker inferred, and an evaluator built before its checker could not decide at all (`10` 2.4) |
| A list of the calls in progress, kept during the run, so that a trace is always at hand | Every call of every run that succeeds would pay for it. Each call already has an error path, and the entry is written there (A.10, B.10) |
| The trace or the span as a field of the error value | It would be built and charged for every caught failure, and a program would branch on positions (`10` section 5) |
| A fixed charge for a message, so that its wording could change | A program can still compare the text, so the wording is fixed anyway. The size is the honest charge (`10` section 5) |
| A copy of the message and the data of `fail` into the error value | Both exist already. A copy would charge a caught `fail` by the size of its data |
| Trying the variants of a union one after another | The steps and the first failure would depend on the order and the number of the variants. The tag decides in one step (`09` section 7) |
| Charging a list for each star of a projection | No such list is written: the result is one flat list (`13` 5.2). A charge for lists nobody can see would make `[*]` cost what `flatMap` costs |
| Charging the result of a projection before its reads, from the lengths | With a second star or a `?.` the length is not known until the reads have run, and a read may call a host function, so it cannot run twice. The values wait in scratch space, which the steps bound (A.4) |
| A cost for the form of a string literal, or a `dedent` in the library | The form is spelling, settled by the lexer. A library function would be charged as text read and text created at every evaluation (`08` 2.8) |
| `ctx.Err()` on every node | 35% to 47% slower (B.6) |
| Panic and recover | Never faster, 5x slower on failure. It buys fewer lines and nothing else (B.10) |
| An error value that flows through operators, as in CEL | A typed closure that returns `int64` has no room for it |
| v2's interval check with a goroutine that watches the context (`v2:vm/vm.go:404-429`) | The chunk does the same with no goroutine (B.6) |
| A `Runner` object owned by the caller | 9 ns for an API burden (B.11) |

## 4. Rulings applied, and what is still assumed

| Ruling | Effect here |
| --- | --- |
| C1, P1 to P4 | Parts A, B, C and D. The conformance suite carries steps and units for each case |
| R2 to R6 | Part D. A.3 and A.6 say what an implementation without reflection, a collector or a recoverable stack must still guarantee |
| C5, L1 to L8 | A.3 to A.6, B.6 to B.9. The first draft's "iterations, not bytes" is gone |
| N1 to N6 | A.2, B.2, and the read barrier of B.3 |
| B1, B5, B6 | A condition is a truthiness test of 0 steps. `&&` and `\|\|` return an operand. A condition the checker types as `bool` is a `boolNode` |
| B8 (D33) | A host that tests a result by truthiness calls `risor.Truthy`, which is the evaluator's own test (B.1) |
| B4 | A chain compiles as one unit with a null exit, and skipped accesses are not charged. Inside a projection the exit is for one element (A.1, A.3) |
| E6 | No recursion. A function literal is a `const` value or an argument of a library function or method, never a result |
| E9 (D34) | The callee of every call is known from the text, and no function literal has two calls in progress (A.9). One fixed slot per binding, no frames and no closure objects (B.5). The depth row of A.6 says what the text now fixes |
| X1 (D35) | `if` costs what `? :` costs (A.3), nests as brackets do (A.6) and compiles to the closure of `? :`, with a chain walked by a loop (B.5) |
| X2 (D36) | A binding is written `const`. Nothing in the evaluator changes |
| X3 to X6 | Loops, reassignment and early `return` are not in 3.0; the words are reserved. A.9 says what they would and would not disturb |
| V1 to V5 (D39 to D45) | `as` and `is`: steps in A.3, no allocation in A.4, depth in A.6, what may be skipped in A.8, and the compiled test in B.14. The rule of conformance and the order of the walk are `07`'s. This document owns the count |
| D46 (`08`) | A string literal of any form is a `String` node at 0 steps and 0 units, the backtick with no `${ }` included (08-8). A text block with a part is a `Template` and charges as one (A.3, A.4). The lexer does all the work (B.12) |
| D47 (`09`) | The walk of a tagged union: the tag first, then one variant (09-7). Steps and worked counts in A.3, depth in A.6, the union node in B.14 |
| D48 (`10`) | `catch` decides by code, and what passes through (A.1, 10-3). The cost of a `try`, of a caught failure and of the error value, `fail` with data, unwinding, what a failure may hold, and the trace (A.3, A.4, A.10, 10-4, 10-5). `Try` and the trace in Go (B.10). `ConformAll` over the walk (B.14, 10-8). Every use of the call form of `try` in this document is now `try { e } catch { d }` |
| D48, on two points the documents disagreed on | A crash is not caught, and `function-panic` has class `host` (A.1). A host number that cannot enter the language is `number-range`, class `environment`, and is caught (A.2, B.3) |
| D49 (`11`) | A stubbed host function costs the step of a call plus its row's `charge` (A.3). Nothing else here: the runner is `05`'s |
| D50 (`12`) | Nothing in 3.0. A config file is a program and costs what A.3 and A.4 say. Imports are not in 3.0 (`12` 3.9) |
| D51 (`13`) | A projection: order in A.1, steps and units in A.3 and A.4, nesting in A.6, its trace entry in A.10, the node `Each` in B.15. The new library entries are `06`'s and add no kind of charge |
| S2 | Shared constant collections and the argument stack rely on read-only arguments |
| S3 | A function literal cannot be passed to a host function, and the call handle has no way to call one |
| S4 | Registration picks the adapter (B.4) |
| T1, T2, T5 | Weight for cost, code points for meaning. Map order by code point. Positions in code points |

| Still assumed | Of |
| --- | --- |
| The definition of a whole number and of `number-range` in A.2 | `02` |
| A function literal is a `const` value or an argument of a library function or method. A `const`-bound one is called by name or passed to a library function or method, and used in no other way: it is not put in a list or a map, compared, returned, bound to a second name or passed to a host function. A name bound by a parameter or a pattern cannot be called (D34) | `02` |
| A name that refers to a function literal is bound by a `const` whose value is that literal, and a name is not in scope in its own value. So name resolution gives the callee of a call by name | `02` |
| An `if` holds its tests and branches in order, with the `else` branch last, so that a chain is one node and not a nest of them. `else` is required (D37). The braces of a branch and the parentheses of a test each open one nesting level, and an `else if` opens none | `02` |
| Node 26, `Try`, has `body`, `name` or none, and `handler`, and both parts are branches. The braces of each and the parentheses of `catch (err)` open a level each. The node `Each` has `x`, the reads and `optional`. A projection is a star and the reads and stars that follow it directly, and a star opens a level to the end of its projection. A literal with no `${ }` is a `String` node in every form. There are 32 node kinds | `02` |
| Every run failure has a named code, among them the null operand, the index out of range, the zero divisor and the failure of a star on a value that is not a list | `02`, `05` |
| Per node: each name's resolution (required). As hints: whether the node is exactly `bool`, `int`, `float` or `string`, the `reflect.Type` and index path of a field read on a Go type, the resolved function of a call | `03` |
| A type has at most one alternative for each kind, so a union needs no search. Type depth counts lists and records only, and is at most 64. The type text grammar after `as` and `is` | `03` |
| The rule of conformance (2.2), the order of the walk (2.3), and that a place whose type is `any` is not tested and costs nothing (2.7) | `07` |
| `Conform` is the walk of B.14 with a run state of its own. The `not-conforming` diagnostic carries the path. `ConformAll` is the same walk going on after a failure, and charges nothing. A message cuts a copied key to 64 code points (`10` 2.9). What "present" means for a field of a host record, and whether a nil slice is a list | `05` |
| `messages.json` gives each run code its templates, its parameters and its `caught` field, and cuts each parameter at 64 code points, so the size of every message is fixed. A diagnostic carries `data` and `trace`. The name of a star's trace entry. The code of a failure inside the implementation | `10`, `05` |
| A tagged union has a tag that every variant requires, and no string is in two tag sets, so the tag's value picks at most one variant. `untagged-union` is refused by the compiler as well as the checker | `09`, `03` |
| `fail` is `(message: string, data?: any) => never`. Each entry that takes a callback gives the callback an index, or says what its trace entry holds in its place. The entries of D51 charge by the rules of A.3 to A.5 | `06` |
| `Limits` carries steps, units, a deadline and a depth. A run reports steps and units used. A limit error has no fixed position | `05` |
| The call handle of B.4, and the result rule of B.2 | `05` |
| The cost column uses the rules of A.3 to A.5 and adds no kind of charge. Each entry says which arguments are functions it calls. An entry calls a function literal only before it returns, and never keeps one (A.9 needs this). A sort charges by formula. Pattern matching is linear in its input | `06` |

## 5. Open questions for the owner

1. **Error returns or panics in the Go implementation?** I recommend returns:
   the measurements favor them or tie. The cost is about 600 lines.
2. **Is weight the unit of text?** I recommend it (A.5, B.8). It is the UTF-16
   length, which may look like a JavaScript habit in the rules. The alternative
   is UTF-8 bytes: Go and Rust pay nothing, and the TypeScript implementation
   measures every string of 22 characters or more. With Rust named, two of the
   three implementations would prefer bytes. I still recommend weight, because
   it is the unit that makes short strings free in all three (B.8).
3. **N2 and Go hosts.** A Go `int64` above 2^53 is common: a nanosecond
   timestamp, a 64-bit identifier. Under N2, reading one is an error. I
   recommend keeping N2 and letting the Go binding present such a field as a
   string, declared with its type. The owner should confirm N2 knowing this.
4. **Are counts fixed for ever, or per version of the specification?** I
   recommend per version: a host names the version it runs, and a new version
   may change a rate. Fixed for ever means a wrong rate in 3.0 stays wrong.
5. **Does layer 2 ship in 3.0?** I recommend layer 1 first and layer 2 before
   the release. Without it, a host that passes Go structs pays one allocation
   per scalar field read.
6. **Does the walk of `as` and `is` count against the depth limit?** `07` 2.7
   says only that the type bounds it at 64 levels. I have it count 1 for each
   level it enters, as every other walk over a value does (A.6), so 2D + 100
   stands for any D. The alternative is that it never counts. Then an `as`
   cannot fail on depth, and the bound is D + 100 plus the larger of D and 64.
   The revised `07` 2.7 and `02` 2.16 now say it counts.
7. **A value that cannot enter the language, met by the walk.** `07` does not
   say. I have it fail the test (A.3), so that `is` never raises and the count
   does not depend on a map's order. It also makes `Conform` report a NaN in a
   `float` variable, which is the precondition a host wants tested. The
   revised `07` 2.3 and `05` A.13 now say the same.
8. **Which key does a failed exact record report?** `07` 2.3 says the least
   other key, so every key is visited and each other key costs 1 (A.3). The
   cheaper rule is to report the record itself and let the message name any
   other key. Then the failure costs nothing more and no key is compared.
9. **May a test be skipped on the checker's word?** The first draft of `07`
   2.7 let the work go when the type of `x` already fits. That holds only while
   every host value conforms to its declared type. A.8 asks for a proof, so
   that a run with the checker and a run without it cannot differ (`07` 2.6).
   The revised `07` 2.7 now says the same, and `03` agrees (its open
   question 7).
10. **How long may a trace be?** `10` 2.8 says at most the depth limit. A
    star now has an entry, and the stars of the innermost body count against
    the 100 levels of the source and not against D. I have the bound at D +
    100 (A.10). The alternative is that a star with an element in hand counts
    1 against the depth limit, which keeps the bound at D and makes a
    projection able to fail on depth. Settled as D + 100: `05` A.6 says the
    same, and `10` 2.8 is corrected.
11. **Two things a trace entry does not yet say.** The name of a star's
    entry: a library callback's entry has the entry's name, and a star has
    none. And the index for `mapValues`, whose callback is given a key and no
    index (`13` 5.1): `10` 2.8 defines the index as the one the callback was
    given. I assume `05` and `06` settle both. For `mapValues` I would give
    the position of the entry in key order, from 0. Settled: `05` A.6 names a
    star's entry `[*]`, and `05` and `06` both give `mapValues` the position
    in key order, so an index is always a number or null.
12. **Is a part of a template charged as a string of its own?** `08` 2.8 says
    a template charges "the text created, and the rendering of each part". I
    read that as the walk of a list or a map and not as a second 1 + size for
    each part, so `` `${a} of ${b}` `` creates one string (A.3, A.4). `02` 2.14
    says "one string". The suite's counts depend on it. Settled as read here:
    `08` 2.8 defers to this document's tables and adds no charge.
13. **The worked counts of A.10 assume short messages.** The size of a
    message is known only when `messages.json` is written. If a common
    message passes 64 units of weight, a caught failure costs 2 steps and 2
    units for it, not 1 and 1. Nothing in the design changes.

## 6. Implementation notes

**Size (I).** About 3,900 lines without tests. The spike is 1,240. The figures
were not estimated again for D34 and D35. D34 removes the frame stack, capture
and the closure object from layer 1, and D35 adds one node, so the total should
not grow. `07` adds the type test: about 250 lines for the walk with its paths
and charges (`07` section 5), about 100 for the struct plans of B.14, and the
type text parser of about 300 lines, which moves in from the checker. That
made 4,250 lines, and 4,550 with the parser.

D46 to D51 add about 560 lines, by the plans' own estimates, which I did not
make again. `Try` with the test by code and the error value is 60, the trace
80, and the table of codes with the filling of a template 200 (`10` section
3). `Each` is 90 (`13` 5.6). The union node in the walk is 50 (`09` section
5), and `ConformAll` 80 (`10` section 3). The string forms add nothing to the
evaluator: their 230 lines are in the lexer, the parser and the formatter
(`08` section 5). The three renderings of a diagnostic, 150 lines, and the
test runner are `05`'s. The test for a tag adds 40 lines to the type text
parser (`09` section 5). So the evaluator is about 4,810 lines, and 5,150
with the parser. Removed: the call form of `try`, and every message written
by hand.

| Part | Lines |
| --- | ---: |
| Layer 1: compiler and closures over the seven shapes, with errors returned. `Try` (60), the trace (80) and `Each` (90) are in it | 1,880 |
| Limits: counters, chunking, deadline, weight, charge points | 250 |
| Host values: barrier, accessors, views, per-type entries | 500 |
| Host functions: native call, adapters, converters, reflection | 350 |
| Count-preserving optimizations of B.9 | 300 |
| Run state, errors and spans. The table of codes and templates (200) is in it | 450 |
| Layer 2: typed nodes and typed bindings | 600 |
| Type tests: the run-time form of a type, the walk, struct plans, the union node (50) and `ConformAll` (80) (B.14) | 480 |

**Order of work.**

| Step | Work |
| --- | --- |
| 1 | Layer 1 over the seven shapes with both counters, error returns, spans and the pooled run object, with `Try`, the error value, the trace and `Each`. The table of codes is generated from `messages.json` before the first failure is written, so no message is ever written by hand. No optimization. This is the reference, and the conformance suite's steps and units are recorded from it |
| 2 | One fixed slot per binding, function literals run in place by library functions, direct calls by name (B.5) |
| 3 | The read barrier with the number rules, list and map views, sorted iteration. Then the type test over those values, tagged unions included, with `risor.Conform`, `risor.ConformAll` and the conformance cases of `07` and `09`, and the `as` and `is` nodes (B.14) |
| 4 | Struct accessors, fused paths, per-type entries, and the struct plans of the type test |
| 5 | Host functions: native, adapters, converters, the reflection fallback |
| 6 | The optimizations of B.9, each with its budget sweep test |
| 7 | Layer 2, behind the differential test |
| 8 | Benchmark groups C to L, and the targets as a report in CI |

**From the spike.** Kept: scope resolution (`spike:121-184`), the fast-path
shape of each operator (`spike:494-605`), binding functions at compile time
(`spike:792-797`), the argument stack (`spike:813-825`) and the single run
object (`spike:1190-1195`). Changed: panics become returns, slots are numbered
per program and not per frame, the frames, their pool and the closure object go
(B.5), the counter becomes a chunked countdown with a second counter for units,
truthiness takes the falsy set of B5, `reflect.DeepEqual` (`spike:1075`) becomes
a counted comparison, wrapping arithmetic becomes the range rule, the index that
requires an `int64` (`spike:749-752`) accepts a whole float, `append` in
`filter` becomes the bitmap, and allocation per run becomes the pool.

**Deleted from v2** (lines without tests): `pkg/vm` (3,119), `pkg/compiler`
(4,749), `pkg/bytecode` (1,809), `pkg/op` (272), `pkg/dis` (200) and the object
model in `pkg/object` (8,749), with the type registry and conversion in
`typeconv.go`, the `GoFunc` and `GoStruct` proxies, and the observer: about
18,900 lines. v2's per-operation size limits go with them, replaced by the
allocation budget.
