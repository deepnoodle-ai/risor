# F17. Recursion, function parameters and guards

This chapter is normative. It specifies follow-on package F17 as far as step
E0 of its plan (`experiments/v3/design/18-recursion.md`, and
`experiments/v3/design/14-implementation-plan.md` 5.3, track E): a guard on an
arm of `match`, and the library function `walk`. It adds one optional field of
the node `Arm`, one library entry, the file `suite/library-recursion.json` and
the directory `suite/recursion/`. It adds no node kind, no value, no type, no
run code, no check code, no `parse` code, no reserved word, no message template
and no case field.

Steps E1 to E3 of the plan are not in this chapter yet: typed self-recursion,
recursive named types and function parameters. Each adds to this chapter when
it lands.

The package only adds. In Core a guard is a `parse` error, and `walk` is a name
that no function has. Section 10 lists the one earlier case whose outcome
changes.

Other chapters: `01-language.md` (grammar, nodes, spans, levels, `match`),
`02-evaluation.md` (order, steps, units, the depth count), `03-host.md`
(diagnostics, case files, the order of packages), `04-library.md` (the record
format, how an entry is specified, the cost rules), `05-types.md` (`match`,
tested positions, facts, calls, the static rules), F2
(`spec/packages/f02-templates-analysis.md`), F5 (`spec/packages/f05-unions.md`),
F8 (`spec/packages/f08-errors.md`), F12 (`spec/packages/f12-tools.md`) and F14
(`spec/packages/f14-host-facts.md`).

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
| `03-host.md` 3.11 item 2 | F17 is offered after F14 and before F12 (1.4) |

No other rule changes. A program with no guard and no call of `walk` has the
type, sites, diagnostics, value and usage it has without this package.

### 1.3 Version

1. The guard and `walk` are part of version `3.0`. The record of `walk` has
   `since` `3.0`. This package adds no version number and no version marker.
2. With this package an implementation offers 79 library entries: the 53 of
   Core, the 8 of F6, the 6 of F7, the 11 of F13, and `walk`. 60 of them are
   in the group `core`: Core's 53, F7's 6 and `walk`.

### 1.4 Packages this one requires

1. Packages are offered in a fixed order, each only with every package before
   it (`03-host.md` 3.11 item 2): F3, F1, F2, F4, F5, F6, F7, F8, F9, F10, F11,
   F13, F14, F17, F12. F16 and F19 take their places beside F17 when their
   chapters are written: F14, F16, F17, F19, F12 (`15-build-rulings.md`
   R211). So an implementation that offers F17 offers every package before
   it, and the cases of this package may use their forms and their case
   fields: the tagged unions of F5, the trace of F8, the host facts of F14.
2. **How the steps of F17 are offered is open** (`14` 5.3). F17 as this
   chapter states it holds step E0 only, and an implementation that offers it
   offers that step. Step E0 needs no part of F16.
3. **An implementation that offered F12 before this chapter was written.** It
   offers F17 when it can. Until then it keeps passing the suite it passed:
   it does not run `suite/recursion/` or load `suite/library-recursion.json`,
   and it runs `suite/tools/until-f17-complete.json`, which holds the one
   case of `suite/tools/` whose outcome F17 changes (10). That is why the
   cases of F17's forms in the formatter and in assist stand in this
   package's directory, in files whose names begin `tools-` (8), and not in
   `suite/tools/`.

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
   other rows apply as anywhere: `match x { 1 if (c) | 2 => 3 }` is
   `bitwise-operator` by row E10, and the message of each code is F8's.

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

These replace the rows of `05-types.md` 22.4 and 22.8 for `match`.

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
   stops at `depth-exceeded`. A value with many paths to its nodes stops at
   the step limit.
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
   `a` and `b`.
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

## 9. Messages

This package adds no template. Its failures use Core's templates:

| Failure | Code | Template | Parameters |
| --- | --- | --- | --- |
| A result of `children` that is neither a list nor null | `wrong-kind` | `result` | `name` `walk`; `kind` the result's kind |
| A value at `children` in an unchecked run | `wrong-kind` | `argument` | `name` `walk`; `kind` the value's kind |
| A walk deeper than the limit | `depth-exceeded` | `default` | `limit` |
| No arm taken | `no-match` | `default` | `found` the subject |

## 10. Earlier cases

1. One case of `suite/tools/` lists every function of the environment, and
   `walk` joins that list: "every name at the start of an empty text" of
   `suite/tools/complete.json`. It moves, with the header of its file, to
   `suite/tools/until-f17-complete.json` (`03-host.md` 3.11 item 4).
   `suite/recursion/tools-complete.json` holds the same text with this
   package's outcome.
2. No other earlier case changes. No case of the suite calls `walk` or writes a
   guard: the texts of `match` with `if` in the suite have it in an arm's
   result.
