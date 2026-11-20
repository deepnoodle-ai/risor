# F17. Recursion, function parameters and guards

This chapter is normative. It specifies follow-on package F17 as far as step
E3 of its plan (`experiments/v3/design/18-recursion.md`, and
`experiments/v3/design/14-implementation-plan.md` 5.3, track E). Step E0: a
guard on an arm of `match`, and the library function `walk` (sections 2 to
10). Step E1: typed self-recursion, a `const`-bound lambda whose parameters
and result carry types and that calls itself (section 11). Step E3: function
parameters, a parameter with a function type that its lambda may call and
give on (section 13). It adds one optional field of the node `Arm`, one
library entry, the files `suite/library-recursion.json` and
`suite/messages-recursion.json`, three message templates and the directory
`suite/recursion/`. It adds no node kind, no value, no type, no run code, no
check code, no `parse` code, no reserved word and no case field. It changes
the text of one template of Core, that of `recursion-needs-types` (9), and
retires one of F16, `fn-value` (13.5).

Step E2 of the plan is not in this chapter yet: recursive named types. It
adds section 12 when it lands.

The package only adds. In Core a guard is a `parse` error, `walk` is a name
that no function has, and a reference of a lambda to its own name is the
static error `recursion-needs-types`. With F16 alone a parameter with a
function type is a value: a call of it is `call-of-parameter`, and a lambda
given at it is `lambda-to-lambda`. Sections 10 and 13.9 list the earlier
cases whose outcome changes.

Other chapters: `01-language.md` (grammar, nodes, spans, levels, `match`),
`02-evaluation.md` (order, steps, units, the depth count), `03-host.md`
(diagnostics, case files, the order of packages), `04-library.md` (the record
format, how an entry is specified, the cost rules), `05-types.md` (`match`,
tested positions, facts, calls, the static rules), F2
(`spec/packages/f02-templates-analysis.md`), F5 (`spec/packages/f05-unions.md`),
F8 (`spec/packages/f08-errors.md`), F12 (`spec/packages/f12-tools.md`), F14
(`spec/packages/f14-host-facts.md`) and F16
(`spec/packages/f16-entry-points.md`).

## 1. Terms and scope

### 1.1 Terms

| Term | Meaning |
| --- | --- |
| Guard | The text `if (e)` between the literals, or the `_`, of a `match` arm and its `=>`. Also the expression `e` |
| Guarded arm | An arm that has a guard |
| Plain arm | An arm that has none |
| Node | A value that `walk` visits: its root, and each element of each list that its callback gives |
| Depth of a node | 0 for the root. For any other node, 1 more than the depth of the node whose callback gave it |
| Pre-order | The order of a walk: a node, then the nodes under its first child, then those under its second, and so on |
| Typed lambda, self-reference, self-call | Section 11.1 |
| Function parameter, function binding, higher-order lambda, bound lambda | Section 13.1 |

### 1.2 What this package changes in earlier rules

| Earlier rule | With this package |
| --- | --- |
| `01-language.md` 3.1, `Case` | A guard may follow the literals or the `_` (2.1) |
| `01-language.md` 4.1, node 25 `Arm` | The field `guard`, an expression or none (2.2) |
| `01-language.md` 9.3 items 3, 7 and 9 | An arm is chosen only when its guard is truthy (3). Guards change `duplicate-case` and `unreachable-arm` (4.1) |
| `01-language.md` 14.3, the row of a guard | Replaced: a guard is a program (2.4) |
| `02-evaluation.md` 2.2 and 5.2, the rows of `match` | A guard is evaluated after a literal of its arm matched, and charges what its expression charges (3) |
| `02-evaluation.md` 8.4 item 1 | The call that `walk` makes of its callback for a node also counts the node's depth (5.4) |
| `04-library.md` 1.1, 1.3 and 7 | One entry, `walk`, in a file of its own (5) |
| `04-library.md` 2.4 item 4 | The `root` of `walk` is a second parameter outside a callback whose type is a type variable with no bound (5.5) |
| `04-library.md` 2.6 item 3 | A value at the callback parameter of a function fails the tests of the arguments (5.5) |
| `05-types.md` 10.1 | A guard is a tested position (4.2) |
| `05-types.md` 11.2, and F5 5.3, F14 4.3 | A guarded arm: its facts, coverage and the literals it ignores (4.2, 4.3) |
| `05-types.md` 13.5 step 3a | How the seed rule reads for `walk`, whose result is not its type variable (5.6) |
| `05-types.md` 22.4 and 22.8 | The walk of a guard, `duplicate-case` and `unreachable-arm` (4.1) |
| F8 4.2 item 2 | The index of a trace entry of `walk` (5.4) |
| F12 2.8 | The document of a guard (7.1) |
| F12 3.11 item 1 | A string listed only by guarded arms stays a candidate (7.2) |
| `03-host.md` 3.11 item 2 | F17 is offered after F16, slice D1, and before F12 (1.4) |
| `01-language.md` 5 rule 6 and 14.4, `05-types.md` 22.5 and 22.7, F16 3.1 item 10 | A self-reference of a typed lambda is not `recursion-needs-types` (11.2) |
| `05-types.md` 13.9, F16 3.4 | A self-reference of a typed lambda is a use of a lambda checked once, and a self-call has the result type (11.3) |
| `02-evaluation.md` 8.4 item 10 | A checked program reaches the depth limit through self-calls (11.4) |
| `02-evaluation.md` 10.2 and 11.1 | Each call binds its own names; three facts of 11.1, and the fixed places that rest on them, hold only for a program with no self-reference (11.5) |
| `03-host.md` 3.9.1 | Three deepest cases of self-recursion in the stack test (11.6) |
| F12 3.13 | A hover on a self-call of a typed lambda gives the result type (11.7) |
| `suite/messages-check.json`, `recursion-needs-types` | Its text names the fix (9) |
| `01-language.md` 7.2 tests 3 to 5, 7.3 item 2, 8.1 rule 3, 8.2 rules 1, 2 and 4 and 14.4; `05-types.md` 13.2, 13.9, 22.4, 22.5 and 22.9; F16 3.1 items 9 and 11, 3.2 item 2 | A parameter with a function type is a function binding: it may be called and given on, and it is not a value (13.2) |
| `05-types.md` 13.2, 13.5 step 3 and 13.9; F16 3.3 and 3.4 | How a call of a function parameter, a lambda given at one and one given on are checked (13.3) |
| `02-evaluation.md` 8.4 item 10, 10.2, 11.1, 11.2 and 11.3 | A call of a function parameter, a lambda argument of a call by name, each call's own names (13.4) |
| F8 3.1 item 2, 3.5; F16 9.1 | Three templates in this package's catalog file; F16's `fn-value` retires (13.5) |
| F2 3.2; F12 3.6, 3.12 and 3.13 | Analysis, completion and hover of a function parameter (13.6, 13.7) |

No other rule changes. A program with no guard, no call of `walk`, no
self-reference of a typed lambda and no parameter with a function type has the
type, sites, diagnostics, value and usage it has without this package, but for
the text of the message of `recursion-needs-types` (9).

### 1.3 Version

1. The guard, `walk`, self-recursion and function parameters are part of
   version `3.0`. The record of `walk` and the entries of
   `suite/messages-recursion.json` have `since` `3.0`. This package adds no
   version number and no version marker.
2. With this package an implementation offers 79 library entries: the 53 of
   Core, the 8 of F6, the 6 of F7, the 11 of F13, and `walk`. 60 of them are
   in the group `core`: Core's 53, F7's 6 and `walk`.

### 1.4 Packages this one requires

1. Packages are offered in a fixed order, each only with every package before
   it (`03-host.md` 3.11 item 2): F3, F1, F2, F4, F5, F6, F7, F8, F9, F10, F11,
   F13, F14, F16 slice D1, F17, F12. F19 takes its place beside F17 when its
   chapter is written: F14, F16, F17, F19, F12 (`15-build-rulings.md` R211).
   F16's slice D1 stands before F17, and its later slices after it
   (`spec/packages/f16-entry-points.md` 1.4). So an implementation that
   offers F17 offers every package before
   it, and the cases of this package may use their forms and their case
   fields: the tagged unions of F5, the trace of F8, the host facts of F14.
2. **F17 is offered whole, and keeps one place.** As this chapter states it,
   F17 holds steps E0, E1 and E3, and an implementation that offers it offers
   all three. A later step extends this chapter, and every implementation
   offers it in the change that adds it (`15-build-rulings.md` R261). Step E0
   needs no part of F16. Steps E1 and E3 need the annotations of F16's slice
   D1: a lambda is typed only by them (11.1), and a function type is one
   (13.1).
3. **F12 was offered before this chapter was written,** and F17 stands before
   it in the order. So the cases of F17's forms in the formatter and in assist
   stand in this package's directory, in files whose names begin `tools-` (8),
   and not in `suite/tools/`: a runner runs them only when it offers F12. The
   one case of `suite/tools/` whose outcome F17 changes stood in a file named
   `until-f17-` while an implementation offered F12 and not F17 (10). Every
   implementation offered F17 in the change that added it, so no
   implementation offers F12 without it.

## 2. Guards: syntax

### 2.1 Grammar

The production `Case` of `01-language.md` 3.1 becomes:

```ebnf
Case    = ( Lit { "|" Lit } | "_" ) [ "if" "(" Expr ")" ] "=>" Expr .
```

```js
match order.status {
  "open" if (order.total > 1000) => "review",
  "open" => "ship",
  "held" | "late" if (order.priority) => "escalate",
  _ => "wait",
}
```

1. The guard is written after every literal of its arm. It belongs to the whole
   arm: `"held" | "late" if (c)` tests `c` after either literal matched.
2. The parentheses are required, as in `if (c)`. They end the guard before the
   `=>`, so the guard needs no rule about a lambda or an operator.
3. A `_` arm may have a guard: `_ if (c) => e`.
4. `if` is a reserved word already (`01-language.md` 2.4). No word is added.

### 2.2 The node

| # | Node | Fields | Written |
| --- | --- | --- | --- |
| 25 | `Arm` | `literals` (list), or default; `guard` (an expression, or none); `result` | `"a" \| "b" => e`, `_ => e`, `"a" if (c) => e` |

1. No node kind is added. Core still has 30 node kinds.
2. The guard's expression is a child of the `Arm`, between its literals and its
   result. A walk of the tree visits the literals, then the guard, then the
   result.
3. **Spans.** The span of an `Arm` is unchanged: from its first literal, or its
   `_`, to the end of its result. The guard's node has its own span. The
   word `if` and the parentheses are tokens of the `Arm` and of no node
   inside it (`01-language.md` 4.3).

### 2.3 Levels of nesting

The `(` of a guard is a bracket. It opens one level, to its `)`, as the
parentheses of the test of an `if` do (`01-language.md` 13.1). The word `if`
opens no level. So in `match x { 1 if (f(y)) => 0 }` the call `f(y)` stands
inside two levels: the `{` of the arms and the `(` of the guard.

### 2.4 Text that stays an error

1. In Core, a guard is `unexpected-token` at the `if`. `01-language.md` 14.3
   has the row. With this package the text is a program.
2. These stay errors. Each is `unexpected-token` at the token named, by
   `01-language.md` 3.5 rule 1, unless the row says otherwise.

| Written | Error |
| --- | --- |
| `match x { 1 if c => 2 }` | At `c`: the parentheses are required |
| `match x { 1 if => 2 }` | At `=>` |
| `match x { 1 if () => 2 }` | At `)`: a guard holds an expression |
| `match x { 1 if (c) 2 }` | At `2` |
| `match x { 1 if (c, d) => 2 }` | At `,` |
| `match x { 1 if (c) if (d) => 2 }` | At the second `if`: one guard to an arm |
| `match x { if (c) => 2 }` | `reserved-word` at `if`: a case starts with a literal or `_` (`01-language.md` 14.2) |
| `match x { 1 if (c) = 2 }` | At `=` |

3. **With F8.** The rows of F8 2.6 and 2.7 read the `if` of an `If`, and a
   guard is not one. So F8's rows E4 and E5 do not apply to a guard: `match x {
   1 if c => 2 }` stays `unexpected-token`. A guard is not an operand, so F8's
   row E7 does not apply after it, as it does not after a literal of an arm
   (F8 2.7 item 1): `match x { 1 if (c) = 2 }` stays `unexpected-token`. The
   `(` of a guard is not the `(` of a parenthesized expression, so F8's row E9
   does not apply inside it, as it does not in the test of an `if`: `match x {
   1 if (c: int) => 2 }` is `unexpected-token` at the `:`. The other rows
   apply as anywhere: `match x { 1 if (c) | 2 => 3 }` is `bitwise-operator`
   by row E10, and the message of each code is F8's.

## 3. Guards: evaluation

The row of `match` in `02-evaluation.md` 2.2 becomes:

| Form | Order |
| --- | --- |
| `match` | The subject, once. Then each arm in order: the subject is compared with `==` to each literal of the arm, left to right, and the arm stops at its first equal literal. When a literal is equal, or the arm is `_`, the arm's guard, if it has one, is evaluated. The arm is taken when it has no guard, or when its guard is truthy. When the guard is falsy, the next arm is tried. Then the result of the arm taken. No other result is evaluated. If no arm is taken, the failure `no-match` |

1. A guard is evaluated only after a literal of its arm matched, or at a `_`
   arm. A guard whose arm did not match is not evaluated and charges nothing.
2. A guard is evaluated at most once for each run of its `match`.
3. **Steps.** The row of `match` in `02-evaluation.md` 5.2 stands: 1 step for
   each arm tested, in order, up to and including the arm taken, or for every
   arm when none is taken, charged before the arm's comparisons. A guarded arm
   counts as any arm. A guard charges what its expression charges, when it
   runs. Its truthiness test charges nothing (5.1 rule 4). A guard adds no step
   of its own.
4. **Units.** A guard adds none of its own.
5. **Failures.** A failure in a guard is the failure of the `match`, with its
   own code and span. `no-match` is raised as in Core, with the subject as its
   `found`.
6. **Depth.** A call in a guard is measured as any call is (`02-evaluation.md`
   8.4). The `{` of the arms and the `(` of the guard are levels (2.3).

| Program | Data | Result |
| --- | --- | --- |
| `match x { 1 if (y > 0) => "a", 1 => "b", _ => "c" }` | `{x: int, y: int}`, x = `1`, y = `0` | `"b"`; 3 steps: the first arm 1, its guard 1, the second arm 1 |
| The same | x = `1`, y = `1` | `"a"`; 2 steps |
| The same | x = `2`, y = `1` | `"c"`; 3 steps: the guard is not evaluated |
| `match x { _ if (x > 5) => "big", _ => "small" }` | `{x: int}`, x = `1` | `"small"`; 3 steps |
| `match x { 1 if (1 / y > 0) => "a", _ => "b" }` | `{x: int, y: int}`, x = `1`, y = `0` | Fails: `divide-by-zero` at `1 / y`; 2 steps, 0 units |

## 4. Guards: the static rules and the checker

### 4.1 The static rules

These rows are those of `05-types.md` 22.4 and 22.8 for `match`, with what a
guard adds. For a `match` with no guard they say what Core's rows say, so the
package adds to the chapter's rules and changes none (`05-types.md` 23).

| Rule | Text |
| --- | --- |
| The walk (22.4) | The subject, then each arm in order: its guard, if it has one, then its result |
| `duplicate-case` (22.8) | A literal equal by `==` to an earlier literal of the same arm, or to a literal of an earlier plain arm. A literal of an earlier guarded arm is not compared with it. The span is the later literal, its `-` included |
| `unreachable-arm` (22.8) | An arm after the first plain `_` arm. The span is the arm |

1. So `match x { 1 if (c) => "a", 1 => "b", _ => "c" }` is accepted: the
   second arm is reached when `c` is falsy. `match x { 1 => "a", 1 if (c) =>
   "b", _ => "c" }` is `duplicate-case` on the second `1`, and `match x { 1 | 1
   if (c) => "a", _ => "b" }` on the second `1` too.
2. `match x { _ if (c) => 1, 2 => 3, _ => 4 }` is accepted. `match x { _ => 1,
   2 if (c) => 3 }` is `unreachable-arm` on `2 if (c) => 3`.
3. A guard is an expression of the arm's scope, the scope where the `match`
   stands: an arm binds no name (`01-language.md` 9.3 item 7). Every static
   rule applies inside it.
4. The static codes do not change. The guarantee of `05-types.md` 22.11 holds.

### 4.2 The checker

These join the rules of `05-types.md` 11.2.

| Matter | Rule |
| --- | --- |
| A tested position | The guard is a tested position (10.1). With the class `constant-condition` rejected, `always-truthy` or `always-falsy` is reported on the guard's expression. An `any` guard records no site |
| The guard's facts | The guard is checked under the facts in force at its arm and the facts of its literals (4.3) |
| The result of a guarded arm | Checked under the facts of its arm and then the true facts of its guard (10.5) |
| The literals 11.2 ignores | A literal that is `duplicate-case` by 4.1, and no other. A literal equal to a literal of an earlier guarded arm is not ignored: it has `never-equal` and its facts as any literal has |
| Coverage | A guarded arm covers nothing: its literals do not count, and a guarded `_` arm is not an `_` arm. Without a plain `_` arm, S must be finite and every value of S must be a literal of a plain arm. Otherwise `non-exhaustive` on the whole `Match` |
| `_` after full coverage | Accepted, with no diagnostic |
| Type | The Join of the results of every arm, guarded or not, in order |

1. So with `b: bool`, `match b { true if (c) => 1, false => 2 }` is
   `non-exhaustive`, and `match b { true if (c) => 1, true => 3, false => 2 }`
   checks, with the type `int`.
2. `match x { _ if (c) => 1 }` is `non-exhaustive` whatever the type of `x`.
3. A checked program never fails with `no-match` at a `match` whose values
   keep their declared types: such a `match` has a plain `_` arm, or its plain
   arms cover S.

### 4.3 Facts

| Matter | Rule |
| --- | --- |
| The facts in force at an arm | Those in force at the `match`, then those that earlier plain arms give later arms: {p} after a plain arm that lists `null` (11.2), the fact of F5 5.3 item 2 |
| The facts of an arm's literals | null(p) for an arm whose one literal is `null` (F14 4.3); the fact of F5 5.3 item 1 for an arm whose literals are strings. They hold in the arm's guard and in its result, as in a plain arm's result |
| The guard's true facts | Hold in the arm's result, after the facts of the arm |
| The guard's false facts | Hold nowhere. A later arm may be reached because no literal of the guarded arm matched |
| What a guarded arm gives later arms | Nothing. Not {p} of `05-types.md` 11.2 when it lists `null`, and not the fact of F5 5.3 item 2. A literal of it may have matched while its guard was falsy |

| Program | Data | Result |
| --- | --- | --- |
| `match x { 1 if (u != null) => u + 1, _ => 0 }` | `{x: int, u: int \| null}` | `int` |
| `match x { 1 if (u != null) => 0, _ => u + 1 }` | The same | `maybe-null` on `u` in `u + 1` |
| `match u { null if (c) => 0, _ => u + 1 }` | `{u: int \| null, c: bool}` | `maybe-null` on `u` in `u + 1`: the guarded arm gives no {u} |
| `match u { null => 0, _ => u + 1 }` | The same | `int`, as in Core |
| `match s.kind { "circle" if (s.r > 1) => s.r, "circle" => 0, "rect" => s.w }` | `s: {kind: "circle", r: float} \| {kind: "rect", h: float, w: float}` | `float`: `s` is a circle in the guard |
| `match s.kind { "circle" if (s.r > 1) => s.r, _ => s.w }` | The same | `maybe-absent` on `w`: under `_`, `s` is not narrowed |

## 5. `walk`

### 5.1 The record: `suite/library-recursion.json`

1. `suite/library-recursion.json` is a JSON array of 1 record, `walk`. It is a
   part of the specification table of `04-library.md` 1.3, kept in a file of
   its own. An implementation that offers F17 loads it after the record files
   of the packages before it. One that does not offer F17 does not load it.
2. The record has the fields of `04-library.md` 1.3, with `group` `core` and
   `since` `3.0`. It has no `receiver`: `walk` is a function. No record of an
   earlier file has the name `walk`.
3. It is in the group `core`, so every environment has it unless it declares
   it removed by the name `walk` (`03-host.md` 3.3.1). It is the first
   function of the library that takes a callback.

### 5.2 The entry

| Field | Value |
| --- | --- |
| Signature | `walk: (root: T, children: (node: T, depth: int) => T[] \| null) => T[]` |
| Receiver | None: a function |
| Steps | `2*k` |
| Units | `1 + k` |
| Null | Never |
| Fails | `wrong-kind`, template `result`: a result of `children` is neither a list nor null; `depth-exceeded`: a node is deeper than the depth limit allows (5.4) |
| Order | Tests; for each node, in pre-order: 2 steps, then `children`, then its result is tested. Then units; build. |

1. **The nodes.** The first node is `root`, at depth 0. `children` is called
   once for each node, in pre-order, with the node and its depth. Its result
   gives the node's children, in order. Each child is a node, at the depth of
   its parent plus 1.
2. **The result.** A new list of every node, in pre-order. `k` is the number
   of nodes, which is the length of the result (`04-library.md` 4.2).
3. **The result of `children`.** Null gives the node no child. A list gives
   its elements as the node's children, in order; an empty list gives none.
   Any other value fails with `wrong-kind`, template `result`, when that call
   returns, before any later node. Its `name` is `walk` and its `kind` the
   kind of the value. It charges nothing more: the call was paid for with the
   2 steps of its node (`04-library.md` 4.5 item 4).
4. **Each node** is charged 2 steps, 1 for the visit and 1 for the call, then
   the depth test (5.4), then `children` is called. Its body charges as it
   runs (`02-evaluation.md` 5.6).
5. **Host values.** A child is read as a host value (`02-evaluation.md` 3.3)
   when it is handed to `children`, after its 2 steps and before the depth
   test, as an element handed to a callback is. The result holds each node as
   it was handed to `children`. The root is the argument, read where the
   argument was evaluated.
6. **Units** `1 + k` are charged after the last call of `children`, and then
   the result is built. So a walk whose callbacks pass the step limit, or the
   depth limit, charges no unit of its own.
7. A list that `children` gives is not copied: each of its elements is visited
   once for each time a node gives that list. A node given twice is visited
   twice, so a value with many paths to one node costs a visit for each path.

### 5.3 An explicit stack

1. An implementation keeps the place of the walk in a stack of its own: for
   each level, a list of children and a position in it. It does not recurse
   natively for a level of the walk, so a walk costs no native stack for its
   levels (requirement R5 of `experiments/v3/design/00-overview.md`).
2. The stack holds at most one entry for each depth that the walk reaches, so
   at most the depth limit plus one entries (5.4). It is scratch space
   (`04-library.md` 4.6).
3. Each call of `children` runs its body, which may make calls of its own. That
   recursion is bounded by `02-evaluation.md` 8.5 item 3. The walk adds no
   native recursion to it.

### 5.4 Depth

1. **The count.** The call that `walk` makes of `children` for a node of depth
   d is a call that a library entry makes of a lambda (`02-evaluation.md` 8.4
   item 3). While it is in progress it adds the measure of `walk`'s call, 1
   plus the nesting depth of the call within its body, plus d. This is the
   level of the walk that the plan names (`18` 5 rule 4): each list of children
   entered adds 1.
2. **The test** is the test of `02-evaluation.md` 8.4 item 7 for a call that a
   library entry makes of a lambda: after the node's 2 steps, before the
   parameters are bound. When the count would be more than the depth limit,
   `depth-exceeded`. No other test is made: a list of children is not a walk of
   8.4 item 5, and is not tested when it is met or entered. So a node whose
   result is an empty list costs no test beyond its own.
3. So with `walk` at nesting depth 0 in the program and the depth limit L, the
   deepest node whose `children` runs has depth L − 1. A node of depth L fails
   at its test.
4. A value of host data that holds itself, as a list that is its own element,
   cannot be walked for ever: the walk stops at `depth-exceeded`, unless the
   step limit, the deadline or a failure of the callback stops it first. A
   value with many paths to its nodes stops at the step limit.
5. **The trace** (F8 4). The call of `walk` is in progress while a call of
   `children` is, as any library call of a lambda is. The `index` of its entry
   is the position in pre-order of the node that call was made for, from 0:
   its index in the result.
6. The depth of a node counts as the levels of a walk count in
   `02-evaluation.md` 8.4 item 10 and 8.5 item 3. It is at most the depth
   limit, and it adds no native recursion (5.3). So the bounds there hold.

### 5.5 Arguments

1. **The tests of `04-library.md` 2.6.** `root` has the type `T`, a type
   variable with no bound, and is not tested. `children` is a callback
   parameter.
2. **The static rules** of a call by name apply (`05-types.md` 22.5): a count
   other than 2 is `arity` on the whole `Call`; a lambda argument at `root` is
   `not-a-value` on the argument; a lambda argument with more than 2
   parameters at `children` is `arity` on the argument.
3. **A value at `children`.** An argument at `children` that is not a lambda
   argument, written in place or the name of a `const`-bound lambda, is
   `arg-mismatch` on the argument when the program is checked (`05-types.md`
   13.2). In a run compiled without the type rules it is evaluated with the
   other arguments, and then fails the tests of `04-library.md` 2.6 item 3,
   in their order, before the call's step: `wrong-kind`, template
   `argument`, with the `name` `walk` and the `kind` of the value. Null
   included. It charges nothing; the arguments charged what they charge.
4. A lambda passed to `walk` is called with 2 arguments, the node and its
   depth. A lambda that declares fewer parameters drops the others
   (`02-evaluation.md` 5.6 item 4).

### 5.6 Types

1. The checker reads the signature from the record (`05-types.md` 13.1), and
   checks a call by 13.5, as any call of a library function.
2. **The seed rule** (13.5 step 3a) applies to `walk`: its `Var` `T` occurs in
   its R, `T[] | null`, and in the Q of `node`, and it has a solution A0 from
   `root` before the body is checked. R is not `T` itself, so the rule reads
   with R in place of the `Var`:
   - it fires when B does not fit R with `T` replaced by A0, or fits it only
     by a fit that used any;
   - the new solution of `T` is the Join of A0 and what bind(R, B) gathers for
     `T`: the element type of B's list alternative, or `any` when B is `any`;
     nothing when B has no list alternative, and then A0;
   - the body is checked once more under it, binds nothing, and its type must
     fit R under the new solution (`arg-mismatch` on the body; a fit that used
     any records a `dynamic` site on the body).

   For `reduce`, whose R is its `Var`, this reading is the rule as 13.5 states
   it.
3. When the rule does not fire, step 3 goes on: bind(R, B), then B must fit R
   under the solution. The result type is `T[]` with `T` replaced.
4. `depth` has the type `int`.
5. A run failure of the test of a result is at a `dynamic` site when the body
   of the lambda has one (`05-types.md` 18.3).

| Program | Data | Result |
| --- | --- | --- |
| `walk(1, x => null)` | | `int[]` |
| `walk(0, x => [x + 1])` | | `int[]`, and fails with `depth-exceeded` when run |
| `walk(0, x => x)` | | `arg-mismatch` on the body `x`: `int` does not fit `int[] \| null` |
| `walk(d, x => x.subs)` | `{d: {name: string, subs: {name: string, subs: any[]}[]}}` | `{name: string, subs: any[]}[]`, and a `dynamic` site on the body: the seed rule fires, since `any[]` fits the first level's `subs` only by any |
| `walk(t, x => x.kids)` | `{t: any}` | `any[]` |
| `walk(t, 5)` | `{t: any}` | `arg-mismatch` on `5` |

### 5.7 Examples

In each, `t` is declared `any` and has this value, a tree of four nodes:

```json
{"n": "a", "kids": [{"n": "b", "kids": []},
                    {"n": "c", "kids": [{"n": "d", "kids": []}]}]}
```

| Program | Data | Result |
| --- | --- | --- |
| `walk(t, x => x.kids).map(x => x.n)` | | `["a", "b", "c", "d"]` |
| `walk(t, x => x.kids)` | | the four nodes, `a` `b` `c` `d`; 14 steps, 5 units: the lambda 1, the call 1, and for each node 2 and its body's read of `.kids` 1 |
| `walk(t, (x, d) => d < 1 ? x.kids : null).map(x => x.n)` | | `["a", "b", "c"]` |
| `walk(1, x => null)` | | `[1]`; 4 steps, 2 units |
| `walk(t, x => x.n)` | | Fails: `wrong-kind`, template `result`, message `the callback of walk gave a value of kind string, which it cannot take`; 5 steps, 0 units |
| `walk(0, x => [x + 1])` | | Fails: `depth-exceeded` |
| `walk(0, (x, d) => d < 4 ? [x + 1] : null)` | depth limit 5 | `[0, 1, 2, 3, 4]`; 21 steps, 14 units |
| `walk(0, (x, d) => d < 5 ? [x + 1] : null)` | depth limit 5 | Fails: `depth-exceeded` at the node of depth 5 |
| `walk(t, 5)`, compiled without the type rules | | Fails: `wrong-kind`, template `argument`, message `walk cannot take a value of kind number`; 0 steps, 0 units |

## 6. Analysis

F2's analysis does not change: its rules read the tree, and a guard is an
expression of it (2.2). So:

1. The reads of a guard are reads, and its calls are calls, in source order
   with the rest (F2 3.9). A call in a guard has `lambda` `false` unless the
   `match` stands inside a `Lambda` node.
2. A call of `walk` is a call of a free function name (F2 3.6): its `name` is
   `walk`, its `namespace` null, and its `args` the spans of its two arguments,
   the lambda's span for a lambda. A call in the body of its callback has
   `lambda` `true`.

## 7. Tools

### 7.1 The formatter

F12 2.8 gives the document of an arm with a guard: the literals, or `_`, then
`sp "if" sp "(" g(i(S ‹guard›) S) ")"`, then `sp "=>" assigned(result)`. A run
of one level as the guard is unindented, as the test of an `if` is (F12 2.8
item 4). The `if` and the two parentheses of a guard are kept tokens (F12 2.12
item 1): they are not the parentheses of a parenthesized expression, and the
formatter writes them for every guard. Inside them 2.6 adds no parentheses: a
guard is a place of the last row of its table.

| Source | Formatted |
| --- | --- |
| `match x{1 if(c)=>2,_=>3}` | `match x { 1 if (c) => 2, _ => 3 }⏎` |
| `match x { 1 if ((a + b) > c) => 2, _ => 3 }` | `match x { 1 if (a + b > c) => 2, _ => 3 }⏎` |

### 7.2 Assist

1. `walk` is a function of the library. In the names context (F12 3.6) it is a
   candidate of kind `function`, with its record signature as its detail. A
   hover on the name of a `Call` of `walk` gives the kind `function`, the type
   of the call, the record signature and the record's `doc` (F12 3.13).
2. A guard is a place where an `Ident` may stand, so a hole in it is in the
   names context. A guard has no expected type (F12 3.9).
3. The strings context of a literal of an arm (F12 3.11 item 1) does not leave
   out a string because a guarded arm other than the hole's own arm lists it.
   A guarded arm covers nothing (4.2), so the string may be written again. So
   with a subject of type `"a" | "b"`, `match s { "a" if (c) => 1, "‸` lists
   `a` and `b`. A string that an earlier literal of the hole's own arm lists
   is left out, as F12 3.11 item 1 says, whether or not that arm will have a
   guard: written again it is `duplicate-case` (4.1).
4. The token classes of the web packages do not change: `if` is a reserved
   word, and `walk` is a name.

## 8. The cases

| Path | Holds |
| --- | --- |
| `suite/library-recursion.json` | The record of `walk`, with the cases of 5.7 and more |
| `suite/recursion/guards-run.json` | Guards: values, steps, failures, the static rules |
| `suite/recursion/guards-syntax.json` | The grammar of 2.1, the errors of 2.4 with F8's messages, levels |
| `suite/recursion/guards-check.json` | The checker of 4.2 and 4.3: types, sites, coverage, facts, with F5 and F14 |
| `suite/recursion/walk-run.json` | `walk`: values, steps, units, failures, limits, depth, traces, the static rules |
| `suite/recursion/walk-check.json` | `walk`: types and sites, the seed rule |
| `suite/recursion/analysis.json` | Analysis of guards and of `walk` (F2) |
| `suite/recursion/tools-format.json` | The formatter of 7.1 (F12) |
| `suite/recursion/tools-assist.json` | Completion and hover of 7.2 (F12) |
| `suite/recursion/tools-complete.json` | The earlier case of section 10, with `walk` |
| `suite/recursion/e1-static.json` | Self-recursion: the static rule of 11.2 and the other static rules at a self-reference |
| `suite/recursion/e1-run.json` | Self-recursion: values, steps, units, each call's own names (11.5), the depth count and its limit at 10, 200 and 400, the step limit, traces, a run without the type rules |
| `suite/recursion/e1-check.json` | Self-recursion: the checker of 11.3, types and sites |
| `suite/recursion/e1-analysis.json` | Analysis of a typed recursive lambda (F2) |
| `suite/recursion/tools-e1-format.json` | The formatter on a typed recursive lambda (F12) |
| `suite/recursion/tools-e1-assist.json` | Hover and completion at a self-reference (11.7, F12) |
| `suite/recursion/e3-*.json`, `tools-e3-*.json`, `suite/messages-recursion.json` | Function parameters (13.8) |

1. A runner of an implementation that offers F17 loads
   `suite/library-recursion.json` and runs its cases as it runs those of
   `library.json`. It runs the files of `suite/recursion/` as it runs
   `suite/core-run/` and `suite/core-check/` (`03-host.md` 3.10.8 item 5,
   3.11 item 7): a file whose name ends `-check.json` once, checked, and the
   others by their cases.
2. **The files named `tools-`** use the case fields of F12. A runner runs them
   only when its implementation offers F12, and runs them as it runs
   `suite/tools/` (F12 7.6).
3. A runner of an implementation that does not offer F17 does not run
   `suite/recursion/`, and does not load `suite/library-recursion.json`.
4. This package adds no case field. Its cases use those of `03-host.md` 3.10,
   and of F2, F5, F8, F12 and F14.
5. **Static codes.** For `03-host.md` 3.10.8, 3.10.10 item 17 and 3.10.13, in
   `suite/recursion/` and in `suite/messages-recursion.json` the static codes
   are Core's twelve and `arg-mismatch`, which step E3 makes static at one
   place (13.2 item 5). So a case there that expects `arg-mismatch` from a type
   rule stands in a file whose name ends `-check.json`.

## 9. Messages

Steps E0 and E1 add no template. They change the text of one: the template
`default` of `recursion-needs-types` in `suite/messages-check.json` (F8 3.1)
names the fix that step E1 makes possible. Step E3 adds three templates in this
package's catalog file, and retires one of F16 (13.5).

| Code | Template | Text before | Text with this package |
| --- | --- | --- | --- |
| `recursion-needs-types` | `default` | `{name} is this lambda's own name, and a lambda cannot refer to itself` | `{name} is this lambda's own name; give every parameter and the result a type so that it may refer to itself, or give the lambda another name` |

The template is Core's, so the text changes in Core's catalog file, and a
build without this package prints it too. No build offers Core and slice D1
without F17 (1.4), so no build names a fix it refuses.

Its failures use Core's templates:

| Failure | Code | Template | Parameters |
| --- | --- | --- | --- |
| A result of `children` that is neither a list nor null | `wrong-kind` | `result` | `name` `walk`; `kind` the result's kind |
| A value at `children` in an unchecked run | `wrong-kind` | `argument` | `name` `walk`; `kind` the value's kind |
| A walk deeper than the limit | `depth-exceeded` | `default` | `limit` |
| No arm taken | `no-match` | `default` | `found` the subject |
| A recursion past the depth limit | `depth-exceeded` | `default` | `limit` |
| A recursion past the step limit | `steps-exceeded` | `default` | `limit` |

## 10. Earlier cases

1. One case of `suite/tools/` lists every function of the environment, and
   `walk` joins that list: "every name at the start of an empty text" of
   `suite/tools/complete.json`. It moved, with the header of its file, to
   `suite/tools/until-f17-complete.json` (`03-host.md` 3.11 item 4), and that
   file was deleted when every implementation offered F17 (3.11 item 5).
   `suite/recursion/tools-complete.json` holds the same text with this
   package's outcome.
2. No other earlier case changes with step E0. No case of the suite calls
   `walk` or writes a guard: the texts of `match` with `if` in the suite have
   it in an arm's result.
3. **Step E1** changes one earlier case: the case of `recursion-needs-types`
   in `suite/messages-check.json` expects the text of 9. It changed in place,
   with no file named `until-`: a catalog file has no `until-` form, and step
   E1 lands in every implementation in one change (`15-build-rulings.md`
   R251). No earlier case writes a self-reference of a typed lambda: every
   case of `suite/core-run/static-recursion.json`,
   `suite/core-check/check-recursion.json` and `suite/analysis/scope.json`
   has a parameter or a result with no type, so its code, span and type stand.
   Those cases expect no message.
4. **Step E3** changes eight earlier cases and retires one catalog case
   (13.9).

## 11. Self-recursion (step E1)

Step E1 of the plan (`18` 3, 8 and 9): a `const`-bound lambda whose
parameters and result all carry a type may refer to its own name. Its body may
call it, and may give its name to a library entry as a lambda argument. The
name is in scope there already (`01-language.md` 5 rule 6), and the types are
F16's annotations. The step adds no syntax, node, value, type, code or case
field. It changes one static rule (11.2), how the checker reads a reference of
a lambda to its own name (11.3), the facts of `02-evaluation.md` 11.1 (11.5),
the stack test (11.6) and one message (9).

### 11.1 Terms

| Term | Meaning |
| --- | --- |
| Typed lambda | A `Lambda` that has a result annotation and whose every parameter has an annotation, of a program type or of a function type (F16 2.1). A lambda with no parameter is typed when it has a result annotation |
| Self-reference | A reference of a `const`-bound lambda L to its own name, as `05-types.md` 22.5 defines it: a `Call` whose callee is L, or an `Ident` lambda argument whose innermost binding is L, that stands in L's body or in a default of one of its parameters, also inside a lambda written there |
| Self-call | A self-reference that is a `Call` |
| Recursive lambda | A `const`-bound lambda that has a self-reference |

1. Whether a lambda is typed is read from its text alone, by every compile,
   with or without the type rules. An annotation counts whatever its type
   resolves to: in `const f = (n: Nope): int => f(n); 1` the parameter is
   typed, and the program has one diagnostic, `undefined-name` on `Nope`.
2. A default is no type. `(n = 3): int => e` is not typed; `(n: int = 3): int
   => e` is.
3. A parameter with a function type is typed. It is a function parameter
   (13.1): a self-call gives a lambda at it, and a value there is
   `arg-mismatch` (13.2 rule 3). A lambda that calls itself may pass a lambda
   written in its body to its own name: `const r = (f: (x: int) => int, n:
   int): int => n == 0 ? f(0) : r(y => f(y) + n, n - 1); r(x => x, 3)` gives 6.

### 11.2 The static rule

1. A self-reference of a typed lambda is not `recursion-needs-types`. A
   self-reference of any other `const`-bound lambda is, as `05-types.md` 22.5
   says: once for each lambda, on its first reference.
2. Every other static rule tests a self-reference as it tests any reference of
   the name (`05-types.md` 22.4, 22.5). The rules do not change:

| Text in the body of a typed lambda `f` | Static rule |
| --- | --- |
| A self-call with fewer arguments than `f` requires, or more than it has parameters: `f(1, 2)` for one parameter | `arity` on the `Call` |
| A lambda argument of a self-call at a parameter with no function type: `f(x => 1)` | `lambda-to-lambda` on the argument |
| Its own name given to a self-call, when `f` has a function parameter: `f(f)` | `lambda-to-lambda`, template `higher-order` (13.2 rule 8) |
| Its name at a parameter of a function that takes no function: `len(f)` | `not-a-value` on the name |
| Its name at an `Fn` of a function with fewer parameters than `f` has: `walk(a, f)`, with three parameters | `arity` on the name |
| Its name read as a value: `[f]` | `not-a-value` on the name. A read is no self-reference |
| A self-reference of a lambda written inside `f` that is not typed, to its own name | `recursion-needs-types`, for that lambda |

3. So a compile without the type rules accepts a typed recursive lambda as a
   compile with them does, and runs it.

### 11.3 The checker

A typed lambda is checked once, where it is written: it has an annotation, and
every parameter has one (F16 3.4). Its result type is its result annotation.
That type is known before the body is checked, so a self-call in the body has
it.

| Matter | Rule |
| --- | --- |
| A self-call | A use of a lambda checked once (F16 3.4). Each plain argument is synthesized and fitted to its parameter's annotation by F16 3.3: it must surely fit, else `arg-mismatch` on the argument. The call's type is the result type. The body is not checked again |
| Its name as a lambda argument at an `Fn` of a library function or method, or of a method on an `any` receiver | As F16 3.4 says for a lambda checked once. Each value the library passes must surely fit its parameter's annotation, else `arg-mismatch` on the annotation. The result type stands for the body's type |
| The body | Checked once, fitted to the result annotation by F16 3.3 (`result-mismatch` on the body) |
| A result annotation with a static diagnostic | The result type is `never` at every call, a self-call included (F16 3.4) |
| Sites (`05-types.md` 18) | Those of the one check of the body. A self-call records what any call of a lambda checked once records |
| The limits of `05-types.md` 20.2 and 20.3 | The one check of the body counts, as F16 3.4 says. A self-reference is no check of the body, and counts against neither limit |
| A self-reference of a lambda that is not typed | As `05-types.md` 13.9 says: no use, the type `never` for a self-call, and nothing checked at a lambda argument |

1. The checker never follows a self-call into the body. Its work on a
   recursive lambda is one check of the body, however deep the recursion runs.
2. The checker does not test that a recursion ends. One that does not end
   reaches the depth limit or the step limit when it runs (11.4).
3. In `const f = (n: int): int | null => n <= 0 ? null : f(n - 1) + 1;` the
   self-call has the type `int | null`, and `+` is `maybe-null` on it. The
   annotation, not what the body gives, is the call's type.

### 11.4 Evaluation

A self-call is a call by name (`02-evaluation.md` 11.2). Its name given to a
library entry is a lambda argument (`02-evaluation.md` 11.3). No cost
changes:

| Matter | Rule |
| --- | --- |
| Steps | A self-call charges 1 step after its arguments, then what its parameters' patterns, its defaults and its body charge (`02-evaluation.md` 5.2, 11.2). Its name as a lambda argument charges no step where it stands (`02-evaluation.md` 5.6 item 3). The library entry charges for each call it makes of it, as for any lambda (5.6 item 1) |
| Units | None of a self-call's own |
| Depth | A self-call adds 1 plus its nesting depth within the body while it is in progress (`02-evaluation.md` 8.4 items 1 and 2). A call that a library entry makes of the lambda adds the measure of the library call (8.4 item 3) |
| The limit | `depth-exceeded` at the call that would pass it: a self-call after its step and before its parameters are bound, a call by a library entry after the element's two steps (`02-evaluation.md` 8.4 item 7). `catch` does not catch it |
| A failure | As at any call. Its trace (F8 4) has an entry for each call in progress, each self-call among them, so at most as many entries as the depth limit |

1. **Depth is counted in levels, not calls.** Where a self-call stands in the
   body sets what each call adds. The call `d(n)` in the program stands at
   nesting depth 0 and adds 1. Each row has cases in
   `suite/recursion/e1-run.json`.

| Body of `d`, with `n: int` | Each self-call adds | `d(n)` runs within the depth limit L when |
| --- | --- | --- |
| `n <= 0 ? 0 : d(n - 1)` | 1: the `else` branch of `? :` opens no level | n + 1 ≤ L |
| `n <= 0 ? 0 : [n - 1].map(d)[0]` | 1: the measure of the call of `map` | n + 1 ≤ L |
| `n <= 0 ? [] : [n - 1].flatMap(d)` | 1: the measure of the call of `flatMap` | n + 1 ≤ L |
| `n <= 0 ? 0 : 1 + d(n - 1)` | 2: `d` is the right operand of `+` | 2n + 1 ≤ L |
| `if (n <= 0) { 0 } else { d(n - 1) }` | 2: `d` is inside the braces of the branch | 2n + 1 ≤ L |
| `n <= 0 ? 0 : [n - 1].map(k => d(k))[0]` | 2: 1 for `map`, 1 for `d` by name in the lambda's body | 2n + 1 ≤ L |

2. **Steps, worked.** `const sum = (n: int): int => n == 0 ? 0 : sum(n - 1) +
   n; sum(4)` gives 10 and charges 19 steps and 0 units: 1 where the `const`
   line evaluates the lambda; for each of the four calls with `n` above 0, 1
   for the call, 1 for `==`, 1 for `-` and 1 for `+`, 16 in all; for `sum(0)`,
   1 for the call and 1 for `==`. With a step limit of 18 the run is
   `steps-exceeded`.
3. **Through `map`, worked.** `const d = (n: int): int => n <= 0 ? 0 : [n -
   1].map(d)[0]; d(9)` charges 57 steps: 1 for the `const` line and 1 for the
   call `d(9)`; for each `n` from 9 to 1, 1 for `<=`, 1 for `-`, 1 for the
   call of `map`, 2 for its element and 1 for `[0]`, 54 in all; and 1 for `<=`
   at `n` 0. It makes 10 calls of `d`, each adding 1, so it runs with a depth
   limit of 10, and `d(10)` does not.
4. **The bound of an evaluator's recursion** does not change:
   `02-evaluation.md` 8.5 item 3 bounds it by the depth limit plus 100 levels
   of source, through every call in progress. A checked program can now reach
   the depth limit through calls (`02-evaluation.md` 8.4 item 10).
5. **Steps turned off.** A host may turn the step limit off (`03-host.md`
   3.6.1). A lambda that calls itself twice at each level then makes up to
   2^L calls at the depth limit L, which no run finishes. Such a host should
   set a deadline. This is recommendation 18-5 of `18` 10, which the owner
   has not confirmed; the alternative is to refuse a recursive program when
   steps are off.

### 11.5 Each call's own names

`02-evaluation.md` 11.1 lists facts that the text fixes for every program
without a self-reference. A self-reference breaks three of them:

| Fact of `02-evaluation.md` 11.1 | In a program with a self-reference |
| --- | --- |
| The callee of every call is known when the program is compiled | Holds |
| A lambda runs only while every body around it is in progress | Holds |
| No lambda is entered while an earlier call of it is in progress | Does not hold: a self-call enters its lambda while the call that made it is in progress |
| A program with `k` lambdas has at most `k` calls in progress | Does not hold: the depth limit bounds the calls in progress |
| The calls form a graph with no cycle | Does not hold: a self-reference makes a cycle. The cycle is of one lambda, or of that lambda and lambdas written inside it, as the lambda given to `map` in the first example below. Every cycle passes through a self-reference: a lambda names only itself, lambdas bound before it and the lambdas around it (`01-language.md` 5), so two `const`-bound lambdas of one scope never call each other |

The rules of the reference evaluation that take their place:

1. **Each call binds its own names.** A call of a lambda binds its parameters,
   the names of each `const` line of its body and the name of each `catch
   (err)` in its body, for that call alone. A self-call binds them again, for
   itself. When it ends, the names of the call that made it have the values
   they had before it.
2. **A name is read from the call it belongs to.** A lambda written in a body
   is evaluated in a call of that body's lambda, and reads that body's names
   from that call, whichever deeper calls of the same lambda are in progress
   when it reads them. A lambda never outlives the call in which it was
   evaluated (`02-evaluation.md` 11.1, second fact), so that call is in
   progress whenever it runs.
3. **`catch`** ends every call that the failure passed through. The names of
   the call that holds the `try` keep their values, as the depth count goes
   back to its value when the `try` began (`02-evaluation.md` 8.4 item 8).
4. **How an implementation keeps the names is not fixed.** A frame for each
   call, linked to the frame of the call in which its lambda was evaluated, is
   one way; a closure is another. No frame is a value, so values stay acyclic
   (R3 of `00`).
5. **Fixed places.** One fixed place for each name for the whole run
   (`02-evaluation.md` 10.2) is an optimization that an implementation may make
   only for a program with no self-reference.

In `const f = (n: int): int => n == 0 ? 0 : [1].map(x => f(n - 1) + n)[0];
f(3)` the lambda `x => f(n - 1) + n` reads `n` after `f(n - 1)` has returned.
By item 2 it reads the `n` of the call in which it was evaluated, and the
program gives 6. With one fixed place for `n`, the deeper calls would have left
0 there, and the program would give 0. In `const sum = (n: int): int => n == 0
? 0 : sum(n - 1) + n; sum(4)` the `n` of `+ n` is read after the self-call
ends: 10 by item 1, 0 with one fixed place.

### 11.6 The stack

The stack test of `03-host.md` 3.9.1 has three more deepest cases with this
step. Each is a typed lambda that calls itself, each call adding 1 to the
depth count:

```js
const d = (n: int): int => n <= 0 ? 0 : d(n - 1); d(N)
const d = (n: int): int => n <= 0 ? 0 : [n - 1].map(d)[0]; d(N)
const d = (n: int): int[] => n <= 0 ? [] : [n - 1].flatMap(d); d(N)
```

At each depth limit L of the test, 200 and 400, `d(L - 1)` makes L calls and
gives its value, and `d(L)` is `depth-exceeded`. The suite holds both at 200
and at 400 (`suite/recursion/e1-run.json`), so a runner on the documented
stack runs them; the stack test runs them on the stack divided by 1.5.

### 11.7 Tools

1. **The formatter** has no rule of its own for this step. A typed recursive
   lambda is formatted as F16 7.1 formats any annotated lambda.
2. **Hover.** The row of F12 3.13 for the name of a `Call` of a `const`-bound
   lambda gives the printed type of the `Call`. For a self-call of a typed
   lambda that is the result type (11.3); for a self-call of a lambda that is
   not typed it is `never`. Its name as a lambda argument has the kind
   `lambda` and no type, as every such name has (F12 3.13; R245).
3. **Completion** does not change. A lambda's own name is a candidate of kind
   `lambda` in its body and its defaults (F12 3.6 item 1), typed or not. It
   is listed with the names of the scope that holds its `const` line, in the
   order bound.
4. **Analysis** (F2) does not change. A self-reference is bound to the lambda,
   so it is neither a free name nor a call of the environment (F2 3.2).

### 11.8 Examples

```js
// A tree of any depth, in data typed as any.
const total = (t: any): int =>
  t is any[] ? t.map(total).reduce((a, b) => a + b, 0) : t as int;
total([1, [2, [3, 4]], 5])        // 15

// Its own name given to reduce, which passes the accumulator and the element.
const f = (a: int, b: int): int => b <= 0 ? a : [b - 1].reduce(f, a + b);
f(0, 4)                           // 10

// recursion-needs-types: the result has no type.
const g = (n: int) => n <= 0 ? 0 : g(n - 1);
```

## 12. Recursive types (step E2)

Step E2 of the plan (`18` 4) adds this section when it lands.

## 13. Function parameters (step E3)

Step E3 of the plan (`18` 7 and 8; D57, settled by the owner on 2026-11-19): a
parameter whose annotation is a function type may be called in its lambda's body
and given on as a lambda argument, and a call of the lambda gives a lambda at
it. Functions stay first-order. A function parameter takes no function, a lambda
that has one is never passed, and a function is still not a value: it is not
stored, returned, compared or given to a host function (R3 and S3 of `00`).

```js
const sumBy = (xs: Order[], f: (o: Order) => float): float => xs.map(f).sum();
const apply = (f: (x: int) => int, x: int): int => f(x) + x;
{total: sumBy(orders, o => o.qty * o.price), n: apply(n => n * 2, 3)}
```

The syntax is F16's (2.1): `Param = ... | Name ":" PFn`. The step adds no
syntax, node, value, type, run code, check code or case field. It adds the
static rules of 13.2, one of which makes `arg-mismatch` a static code of this
package; the checker's rules of 13.3; the evaluation of 13.4; and three message
templates in a catalog file of its own, `suite/messages-recursion.json` (13.5).
It retires the template `fn-value` of F16 (13.5).

### 13.1 Terms

| Term | Meaning |
| --- | --- |
| Function parameter | A `Param` whose field `fn` is set (F16 2.3): a name, `:` and a function type. By F16 2.1 its pattern is a name, and it has no default |
| Function type | The `fn` of a function parameter: its listed parameters, each a word and a program type, and its result, a program type or `unknown` |
| Listed count | The number of parameters a function type lists |
| Function binding | The binding that a function parameter makes in its lambda's scope. It is neither a value binding nor a `const`-bound lambda |
| Lambda argument | As `05-types.md` 22.4 says, and also an `Ident` argument whose innermost binding is a function binding. The count of parameters of such an argument is the listed count of its function type |
| Higher-order lambda | A `Lambda` that has a function parameter |
| Bound lambda | In a call of a lambda, the lambda that a function parameter holds: the lambda argument given at it, or, for a function parameter given on, the lambda that one holds |
| Printed function type | `(`, each listed parameter as its word, `: ` and its type printed by `05-types.md` 3.9, separated by `, `, then `) => ` and the result printed, or `unknown`. A type with a static diagnostic prints as `never` |

1. A function binding is bound as a parameter is (`05-types.md` 22.7): in the
   lambda's scope, left to right, with `duplicate-name` and `shadows-namespace`
   as for any name. A binding of the same name in an inner scope hides it, as
   any binding hides an outer one (`01-language.md` 5).
2. Whether a parameter is a function parameter is read from the text alone, by
   every compile (`15-build-rulings.md` R208). A function type with a static
   diagnostic, such as `(x: Nope) => int`, still makes a function parameter: the
   static rules read only its listed count. The checker reads each type with a
   diagnostic as `never` (13.3).
3. A function binding is not a value binding. So the Core rules that test a
   value binding do not apply to it: a call of it is no `call-of-parameter`, and
   its name as an argument is a lambda argument, not a value.

### 13.2 The static rules

These rules apply in every compile, with or without the type rules, in phase
`check` (`05-types.md` 22). They replace, for a function binding, the rows of
`05-types.md` 22.4 for an `Ident` and of 22.5 for a `Call`, `01-language.md` 8.2
rules 2 and 4, and F16 3.1 item 9. Rules 1 to 10 are those of `18` 7.1.

| # | Rule | Code | Template | Span |
| --- | --- | --- | --- | --- |
| 1 | A `Call` whose name's innermost binding is a function binding calls it. A `Call` whose name's innermost binding is a value binding is `call-of-parameter`, as in Core | `call-of-parameter` | `fn-hint` when that binding is a parameter written as a name with no default, with a program type or with no annotation; else `default` | The name |
| 2 | In a `Call` of a `const`-bound lambda with a right count, a lambda argument at a function parameter is allowed. A lambda argument at any other parameter is `lambda-to-lambda`, as in Core; a function parameter's name is one | `lambda-to-lambda` | `default` | The argument |
| 3 | In a `Call` of a `const`-bound lambda with a right count, an argument at a function parameter that is not a lambda argument is an error | `arg-mismatch` | `parameter-at-fn` when the argument is an `Ident` whose innermost binding is a parameter written as a name with no default; else `function`, with `param` the parameter's name and `name` the callee | The argument |
| 4 | An `Ident` whose innermost binding is a function binding, and that is not a lambda argument, is not a value: `[f]`, `{f}`, `f == g`, `f ?? g`, `` `${f}` ``, `const k = f;`, `type(f)`, `f.x`, `f ? 1 : 2`, `[...f]`, `match f { ... }`, and `f` as the body of a lambda, so a function is never returned | `not-a-value` | `function` | The identifier |
| 5 | A function parameter is a name with no default. F16 2.1 gives this, and other text is a `parse` error (F16 2.5) | | | |
| 6 | A function is never given to the host (S3): a lambda argument in a `Call` of a host function, or of a library function at a parameter that is not an `Fn`, is `not-a-value` by `05-types.md` 22.5, and a function parameter's name is one. At a `MethodCall` the type rules (13.3) and the shape test (13.4) decide | `not-a-value` | `argument` | The argument |
| 7 | A `Call` of a function binding has exactly the listed count of arguments. A lambda argument in it is `not-a-value`: each listed parameter takes a value | `arity`; `not-a-value` | `arity` as in 22.5, `name` the parameter's name; `argument`, `param` the listed word | The `Call`; the argument |
| 8 | A lambda argument that is a higher-order lambda, written there or named, is `lambda-to-lambda` in every `Call` and every `MethodCall`, whatever the callee: a `const`-bound lambda, a function of the library or of the host, a function that does not exist, a method | `lambda-to-lambda` | `higher-order`, `name` the callee as written | The argument |
| 9 | A lambda argument at a function parameter has no more parameters than the listed count. At an `Fn` of a library function in a `Call`, 22.5 tests the count as before, and a function parameter given there counts its listed count | `arity` | `parameters`, `most` the listed count, or the `Fn`'s count | The argument |
| 10 | At run time the shape test counts a function parameter given to a method by its listed count, and takes it as a lambda (13.4) | `wrong-kind` | | The `MethodCall` |

1. **Rule 8 decides its argument alone.** An argument that rule 8 reports is
   tested by no other rule of this section or of 22.5. Rule 8 is tested at every
   call, also after `arity` on the whole call and at an unknown callee:
   `nope(apply)`, with `apply` a higher-order lambda, is `undefined-function`
   and `lambda-to-lambda`. A higher-order lambda may be written only as the
   value of a `const` line, and its name may only be called.
2. **The other tests of an argument.** In a `Call` with a right count, each
   argument that rule 8 does not report is tested by its callee, left to right.
   A `const`-bound lambda: at a function parameter, rule 3, then rule 9; at
   another parameter, rule 2. A function binding: rule 7. A host function or a
   library function: 22.5, with a function parameter's name as a lambda
   argument. A wrong count (`arity` on the whole `Call`) turns these tests off,
   as in Core.
3. **A function parameter's name as a lambda argument** is walked as Core walks
   an `Ident` that is a lambda argument (22.5, last paragraph): it is no read,
   and rule 4 does not report it.
4. **Hints.** `fn-hint` and `parameter-at-fn` name the fix for the form that a
   function parameter takes: a parameter written as a name, with no default. A
   `const` name, a name that a pattern binds, a parameter with a default and the
   name of `catch` keep `default` and `function`.
5. **`arg-mismatch` is static at one place.** Rule 3 is the only static rule
   with that code. Core makes a plain argument at an `Fn` a type rule
   (`05-types.md` 13.2), because the parameter there is a library's. A function
   parameter is read from the text, as the count of a call by name is, so the
   test is static (`18` 10, 18-15). Everywhere else `arg-mismatch` stays a type
   rule; at the `Fn` of a library entry its template may be `parameter-at-fn`
   (13.3).
6. **What the rules guarantee.** In a run compiled without the type rules, a
   value is never called, a function parameter is never called with another
   count than its function type lists, a function never reaches a parameter that
   takes a value of a `const`-bound lambda or of a function known by name, and a
   lambda bound to a function parameter never has more parameters than the
   arguments it is called with. At a method, the shape test of 13.4 gives
   `wrong-kind` (inference from the rules; the differential fuzzer is the test,
   `18` 13).

| Text | Static diagnostics |
| --- | --- |
| `const apply = (f: (x: int) => int, x: int): int => f(x) + x; apply(n => n * 2, 3)` | None. The value is 9 |
| `const twice = (f, x) => f(f(x)); 1` | `call-of-parameter`, `fn-hint`, on each `f` (rule 1) |
| `const each = (xs, g) => xs.map(g); 1` | None. Checked, `arg-mismatch` `parameter-at-fn` on `g` at each use (13.3); run without the type rules, `wrong-kind` (13.4) |
| `const g = (f: (x: int) => int) => [f]; 1` | `not-a-value` on `f` (rule 4) |
| `apply(5, 1)` | `arg-mismatch`, `function`, on `5` (rule 3) |
| `const g = (f: (a: int, b: int) => int): int => f(1); 1` | `arity` on `f(1)` (rule 7) |
| `xs.map(apply)` | `lambda-to-lambda`, `higher-order`, on `apply` (rule 8) |
| `apply((a, b) => a, 1)` | `arity` on the lambda (rule 9) |

### 13.3 The checker

These are type rules. They add to `05-types.md` 13 and to F16 3.3 and 3.4. A
function type is a claim: the checker proves it, and a run never tests it
(13.4). **Every fit against a function type is F16 3.3's:** a fit that uses
`any` does not hold, and a type with a static diagnostic is fitted by nothing
(F16 3.3 item 2).

| Matter | Rule |
| --- | --- |
| A function parameter | Bound to its function type. It is never read as a value (13.2 rule 4) |
| A call `f(a1, ..., an)` of a function parameter, with the listed count | Each plain argument is synthesized where the call stands and fitted to the listed type at its position by F16 3.3: through its form, with `arg-mismatch` or `maybe-null` at each place of failure, templates `argument`, `missing` and `extra`. The call's type is the result: `any` for `unknown`, and `never` for a result with a static diagnostic. The call records no site of its own |
| The same with another count | `arity` (13.2 rule 7). The call has the type `never`, and each plain argument is synthesized |
| A `Lambda` written at a function parameter p in a `Call` of a `const`-bound lambda; or there, the name of a `const`-bound lambda that is not checked once | Checked against p's function type as `05-types.md` 13.5 step 3 checks a lambda against a library `Fn`, with no `Var` to solve. Each parameter of the lambda is bound to the listed type at its position by 12.2 and 12.4, `never` for a type with a static diagnostic. An annotated parameter is bound to its annotation, and the listed type must surely fit it (`arg-mismatch` `argument` on the annotation, F16 3.4). The body is synthesized in the scope, and under the facts, where the lambda is written. Its type B is the body's type, or the lambda's result annotation, to which the body is fitted by F16 3.3. When p's result is `unknown`, the body is a tested position (13.7) and nothing is fitted. Otherwise B must surely fit p's result R, else `arg-mismatch`, template `body`, on the body |
| The name there of a `const`-bound lambda | Its check is a use (13.9), and counts against the limits of 20.2 and 20.3 |
| The name there of a lambda checked once (F16 3.4) | Its body is not checked again. Each listed type must surely fit the lambda's annotation at its position, else `arg-mismatch` `argument` on the annotation. Unless R is `unknown`, its result type must surely fit R, else `arg-mismatch` `body` on its body |
| A function parameter g given on at a function parameter p | Fitted, with no body to check. Each type that p lists must surely fit the type that g lists at the same position, else `arg-mismatch` `argument` on g's listed type. Unless p's result is `unknown`, g's result must surely fit it, else `arg-mismatch` `body` on the argument; a result `unknown` is `any` there, which surely fits only `any` |
| A lambda argument that a static rule reports (13.2 rules 2, 8, 9) | Not checked. The parameter it would bind has the type `never` (`05-types.md` 13.2) |
| A function parameter given to a library function or method | `05-types.md` 13.5 step 3, with the function type in place of the lambda. A listed count above n is `arity` on the argument: static in a `Call` (13.2 rule 9), a type rule in a `MethodCall`, template `parameters`. Each Q, its `Var`s replaced by their solutions, must surely fit the listed type at its position, else `arg-mismatch` `argument` on the listed type. A `Var` still unbound in a Q at a position that the function type lists is `cannot-infer` on the argument. B is the function type's result: `any` for `unknown`, `never` for a result with a static diagnostic. Then step 3 goes on as for a lambda: the seed rule (3a), bind(R, B), and the fit of B to R with `arg-mismatch` `body` on the argument. That fit is step 3's, so a fit that uses `any` holds there and records a `dynamic` site on the argument, as F16 3.4 says of a lambda's result annotation. With R `unknown` nothing more is done |
| `filter` and `find` (`05-types.md` 13.8) | A function parameter narrows nothing: there is no body whose facts to read |
| A function parameter given to a method on an `any` receiver (13.3) | Each value passed is `any`, which surely fits only `any`: a listed type that is not `any` is `arg-mismatch` `argument` on that type. The call has the type `any` |
| A plain argument at an `Fn` of a library function or method | `arg-mismatch` on the argument, as in Core (13.2). The template is `parameter-at-fn` when the argument is an `Ident` whose innermost binding is a parameter written as a name with no default, and `function` otherwise |
| A lambda with a function parameter whose every parameter is annotated | Checked once (F16 3.4): a function parameter is annotated. It is bound to its function type, not to `never` |
| A lambda with a function parameter and a parameter with no annotation | Checked at each use (`05-types.md` 13.9), with each function parameter bound to its function type |
| A lambda with no use | The row "No use" of 13.9 and of F16 3.4, with each function parameter bound to its function type. A call of a function parameter in its body is fitted as above, so an argument of type `any`, from a parameter bound to `any`, is `arg-mismatch`, as an argument at an annotated parameter in a call by name there is |
| Sites | A lambda written at a function parameter is checked where it stands, once for each call that holds it, and its body's sites are recorded there. A `const`-bound lambda named there records its sites as at any use |

1. **Why every fit is strict.** No run tests a function type (13.4). A fit that
   used `any` and recorded a `dynamic` site would leave a call of the function
   parameter with the result type while its value may be anything, and a later
   failure would be at no site, as F16 3.3 item 3 says of an annotation. The one
   exception is the fit of step 3 at a library entry, where the entry, not the
   function type, takes the result, and the site is at the argument.
2. **A consequence.** `18` 7.6 writes `sumBy` over `any` data as `const sumBy =
   (xs: any[], f: (x: any) => float): float => xs.map(f).sum();` and calls it
   with `o => o.qty * o.price`. The body's type is `any` (`05-types.md` 8.3),
   which does not surely fit `float`, so the call is `arg-mismatch` on the body.
   The author writes `(o.qty * o.price) as float` (F4), or gives the function
   type the result `any`. `suite/recursion/e3-check.json` holds the three.
3. **`unknown`.** A function type whose result is `unknown` is for a predicate:
   given on to `filter`, `find` or another function type with `unknown`, it
   means what it means there (13.7). A call of it has the type `any` (`18` 7.3,
   recommendation taken).

### 13.4 Evaluation

A call of a function parameter is a call by name of its bound lambda
(`02-evaluation.md` 11.2). A lambda argument at a function parameter is
evaluated where it stands, in the call that holds the argument.

| Matter | Rule |
| --- | --- |
| A lambda argument in a `Call` of a `const`-bound lambda | A `Lambda` written there charges 1 step where it stands, as a lambda argument of a library call does (`02-evaluation.md` 11.3 item 1). The name of a `const`-bound lambda charges nothing (5.6 item 3), and so does the name of a function parameter given on. None allocates a unit |
| A call `f(a1, ..., an)` of a function parameter | As 11.2: the arguments left to right; 1 step; the depth test; then the bound lambda's parameters are bound left to right and its body is evaluated. Its value is the value of the call |
| More arguments than the bound lambda has parameters | The extra arguments are dropped, at no charge, as a library's are (`01-language.md` 8.3 item 2; `02-evaluation.md` 5.6 item 4). A null argument takes the parameter's default |
| Depth | A call of a function parameter adds 1 plus its nesting depth within the body that makes it (`02-evaluation.md` 8.4 items 1 and 2), and is tested after its step and before the parameters are bound (8.4 item 7). The bound lambda's body runs at that count, wherever the lambda was written |
| A function parameter given to a library entry | The entry calls the bound lambda by 5.6 and 8.4 item 3: the library call's measure, and the two steps of each element |
| A failure | Its trace (F8 4) has an entry for the call of a function parameter: `span` the `Call`, `name` the parameter's name, `index` null. `catch` ends the calls the failure passed through, bound lambdas among them, as `02-evaluation.md` 12.1 says |

1. **The shape test** (`01-language.md` 7.2) takes a function parameter's name
   as a lambda: at a callback parameter it passes test 3, and at another
   parameter it fails test 4. Test 5 counts its listed count, not the count of
   its bound lambda. So in a run without the type rules `[1].map(f)`, with `f` a
   function parameter that lists three parameters, is `wrong-kind`, and
   `"ab".repeat(f)` is `wrong-kind`.
2. **Each call's own names.** The rules of 11.5 hold for every program with a
   function parameter, as for one with a self-reference. A bound lambda reads
   its outer names from the call in which it was evaluated, whichever call of
   whichever lambda calls it. That call is in progress whenever it runs: the
   lambda was given to a call made within it, and no function outlives the call
   that received it.
3. In `const apply = (f: (x: int) => int, x: int): int => f(x) + x; apply(y =>
   apply(z => z, y + 10), 1)`, the inner call of `apply` binds `x` to 11 while
   the outer call, with `x` 1, waits for `f`. Each call has its own `x`, so the
   program gives 23, with 10 steps, no unit and a depth count of 4 at the call
   of `z => z`. With one place for `x` the outer call would read 11 when `f`
   returns, and give 33 (`18` 8).
4. **The facts of `02-evaluation.md` 11.1.** A program with a function parameter
   keeps the first two facts, but for the callee of a call of a function
   parameter, which is its bound lambda. The third and fourth do not hold, as
   the example shows. The fifth does not hold as a graph of lambdas: `apply`
   calls the lambda bound to `f`, and that lambda calls `apply`. Every run of a
   program with no self-reference still ends, because no lambda can be given to
   itself (`18` 7.5; inference, which the differential fuzzer tests). One fixed
   place for each name (10.2) is an optimization only for a program with no
   self-reference and no function parameter.
5. **The depth bound** of `02-evaluation.md` 8.4 item 10 is not claimed for a
   program with a function parameter: a call of one runs a lambda that the
   checker checked where it was written, not at the call.

### 13.5 Messages

The catalog file `suite/messages-recursion.json` has the format of `03-host.md`
3.8.2. Its three entries add templates to codes that `suite/messages-check.json`
has, as F8 3.1 item 8 says of a package's file: the same code and phase, params
of kinds that code already has, new template names.

| Code | Template | Text | Chosen when |
| --- | --- | --- | --- |
| `call-of-parameter` | `fn-hint` | `{name} is a parameter; to call it, give it a function type, as {name}: (x: int) => int` | 13.2 rule 1: the binding is a parameter written as a name with no default |
| `arg-mismatch` | `parameter-at-fn` | `{name} is a value; to pass it as a function, give it a function type` | 13.2 rule 3, and the row of 13.3 for a plain argument at a library's `Fn`: the argument is an `Ident` whose binding is a parameter written as a name with no default |
| `lambda-to-lambda` | `higher-order` | `a function that takes a function cannot be passed to {name}` | 13.2 rule 8. `name` is the callee as written: the function, the method or the lambda the argument is given to |

1. Each has one parameter, `name`, of kind `word`: the name as written (F8 3.2
   item 3).
2. `18` 7.2 proposed `{name} takes a function, so it cannot be passed as one`
   for the third, with `name` the lambda passed. A higher-order lambda written
   in place has no name, so the template names the callee (`15-build-rulings.md`
   R281).
3. **`fn-value` retires.** F16's file had the template `fn-value` of
   `arg-mismatch` for a value that the library passes to a function parameter of
   a lambda argument. With rule 8 no higher-order lambda is passed, so no build
   that offers F17 can choose it, and no build offers slice D1 without F17
   (1.4). It is removed from `suite/messages-entry-points.json` with its case,
   by a change to F16's own chapter (F16 9.1; `15-build-rulings.md` R288).
4. Every other diagnostic of this step uses a template that exists:
   `call-of-parameter` `default`; `arg-mismatch` `function`, `argument`,
   `missing`, `extra` and `body`; `lambda-to-lambda` `default`; `not-a-value`
   `function` and `argument`; `arity` `parameters` and the count templates;
   `maybe-null`; `cannot-infer`; `always-truthy` and `always-falsy`;
   `result-mismatch`. Its run failures use Core's: `wrong-kind` and its
   templates of the shape test, `depth-exceeded`, `steps-exceeded`.
5. **Counts.** This step adds no check code. With F4, F5, F8, F16 and F17 the
   check codes are the 31 of F16 9.1 item 2, and their templates are 76: the 72
   of `suite/messages-check.json`, the 1 of `suite/messages-entry-points.json`
   and the 3 of this file.

### 13.6 Analysis

F2's analysis reads the amended scope (F2 3.2):

1. A function parameter is bound in its lambda's body. A call of it is no call
   of the environment, and its name given as a lambda argument is no read.
2. A function parameter hides a function of the environment of its name only in
   its lambda's body: in `const apply = (f: (x: int) => int): int => f(1);
   [f(2), apply(y => y)]` the call `f(2)` is a call of the environment, and
   `f(1)` is not.
3. A call in the body of a lambda written at a function parameter has `lambda`
   `true`, as a call in any lambda has.

### 13.7 Tools

1. **The formatter** has no rule of its own for this step. A function type is
   written by F16 7.1, and a lambda argument of a call by name as any argument
   (F12 2.8).
2. **Completion.** A function parameter is a candidate in the names context (F12
   3.6 item 1), in its lambda's body, with the kind `lambda` and its printed
   function type as its detail (13.1). F12 3.12's row for `lambda` gives that
   detail for a function parameter, and null for a `const`-bound lambda.
3. **Hover.** These rows add to the table of F12 3.13 item 3:

| The word is | `kind` | `type` | `signature` | `doc` | `declaration` |
| --- | --- | --- | --- | --- | --- |
| The name of a function parameter, where it is declared | `lambda` | null | The printed function type | null | Its own span |
| The name of a `Call` of a function parameter | `lambda` | The printed type of the `Call` (13.3): the result, `any` for `unknown` | The printed function type | null | The span of the parameter's name |
| Any other `Ident` whose binding is a function parameter: a lambda argument, or a place that needs a value | `lambda` | null | The printed function type | null | The span of the parameter's name |

4. A parameter of a lambda written at a function parameter is a parameter like
   any other: its hover gives the type it is bound to, the listed type (13.3).
5. **Token classes.** The web packages' token classes do not change: a function
   parameter's name is a name, and the words of a function type are those of
   F16.

### 13.8 The cases

| Path | Holds |
| --- | --- |
| `suite/recursion/e3-static.json` | Each static rule of 13.2, with its templates, and the rules of Core beside a function parameter |
| `suite/recursion/e3-run.json` | Values, steps, units, depth and its limit, traces, `catch`, each call's own names (the `apply` example of 13.4 item 3), runs without the type rules and the shape test |
| `suite/recursion/e3-check.json` | The checker of 13.3: types, sites, fits, each lambda argument, the library entries, `any` |
| `suite/recursion/e3-earlier.json` | The eight texts of 13.9, with this step's outcome |
| `suite/recursion/e3-analysis.json` | Analysis of 13.6 (F2) |
| `suite/recursion/tools-e3-format.json` | The formatter on function parameters (F12) |
| `suite/recursion/tools-e3-assist.json` | Hover and completion of 13.7 (F12) |
| `suite/messages-recursion.json` | The entries of 13.5 and a case for each template |

1. A runner runs these as section 8 says. It loads
   `suite/messages-recursion.json` and runs its cases checked.
2. **Static codes.** For `03-host.md` 3.10.8, 3.10.10 item 17 and 3.10.13, in
   `suite/recursion/` and in `suite/messages-recursion.json` the static codes
   are Core's twelve and `arg-mismatch` (13.2 item 5). A case there that expects
   `arg-mismatch` from a type rule stands in a file whose name ends
   `-check.json`.
3. Step E3 lands in every implementation in one change (`15-build-rulings.md`
   R261). Until an implementation offers it, its runner fails the files `e3-`
   and `tools-e3-`, the cases of `suite/messages-recursion.json`, and the cases
   of 13.9, as with step E1 (R251).

### 13.9 Earlier cases

The step changes the outcome of eight earlier cases and retires one catalog
case. Each moved, with no file named `until-`, as for step E1 (R251): the step
lands in every implementation in one change.

1. **`suite/entry-points/d1-check.json`:** "arg-mismatch: fn-value, the library
   passes a value to an Fn parameter" and "an Fn parameter is bound to never in
   the body".
2. **`suite/entry-points/d1-run.json`:** "an Fn parameter read as a value runs
   in a compile without the type rules", "an Fn parameter called in the body is
   call-of-parameter" and "a lambda given at an Fn parameter is
   lambda-to-lambda".
3. **`suite/recursion/e1-check.json`:** "a parameter with a function type takes
   no value".
4. **`suite/recursion/e1-static.json`:** "lambda-to-lambda: a lambda given to a
   call of its own name" and "lambda-to-lambda: its own name given to a call of
   its own name".
5. `suite/recursion/e3-earlier.json` holds the same eight texts with this step's
   outcome, each named by its old file and what it now shows.
6. **`suite/messages-entry-points.json`:** the entry of `arg-mismatch` with the
   template `fn-value` and its case "arg-mismatch: fn-value" are removed (13.5
   item 3).
7. No other earlier case changes. Every other text of the suite with a function
   type on a parameter neither calls it nor gives a value or a lambda at it.

### 13.10 Examples

```js
// A helper that takes a function, and a function given on.
const apply = (f: (x: int) => int, x: int): int => f(x);
const twice = (g: (x: int) => int, x: int): int => apply(g, apply(g, x));
twice(n => n + 1, 5)                       // 7

// A function type whose result is unknown, given on to filter.
const keep = (ys: int[], p: (x: int) => unknown): int[] => ys.filter(p);
keep([1, 2, 3, 4], x => x % 2 == 0)        // [2, 4]

// call-of-parameter, fn-hint: f has no function type.
const each = (xs, f) => f(xs);
```
