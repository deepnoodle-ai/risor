# Risor v3 design: recursion, recursive types and `match` guards

Status: plan, accepted by the owner on 2026-11-14 (D54 in `00-overview.md`).
The owner chose self-recursion over the two independent reviews of #640 and
#641, which both preferred bounded library entries alone. The owner also
accepted `match` guards. Sub-decisions 18-1, 18-2, 18-3, 18-4, 18-6, 18-9,
18-10 and 18-11 are settled as stated. The others are recommendations the
owner has not yet confirmed. Nothing here is built. The spec text is in sections 3 to 6 in
a form ready to move into the package chapter.

**Evidence.** **(R)** read in the specification or a sibling document at the
place cited. **(ran)** run with the Go CLI built from `main` at `aeaaf1f`
(Go 1.26.0). **(M)** from memory. **(I)** inferred. Decision numbers such as
D34 and E6 refer to `00-overview.md`. R numbers refer to
`15-build-rulings.md`, except R3, R5 and R6, which are in the overview's
section 2.8.

## 1. Summary

1. **The problem.** A program cannot walk data of unknown depth: a comment
   thread, a file tree, a JSON document of any shape. It can page through a
   cursor only with a `reduce` over a `range` that carries a flag. #640 met
   both (friction 5). For a language whose main uses include a query over
   JSON, this is a gap in C4.
2. **Four additions, one package.**
   - A `const`-bound lambda may call itself, when its parameters and its
     result have types (section 3).
   - A named type may refer to itself, through a list, a record field or a map
     (section 4).
   - Two library entries: `iterate` for cursors and `walk` for trees
     (section 5).
   - A guard on a `match` arm: `"open" if (o.total > 100) => ...`
     (section 6).
3. **What it costs the language.** "Every program terminates by
   construction" (E6) becomes "every run ends at its value, a failure or a
   limit". The call graph of a program is still fixed by its text (D34
   stands), but it may have a cycle: a lambda that calls itself. Values stay
   acyclic, so no implementation needs a collector (R3 stands).
4. **What it does not do.** No mutual recursion. No function values. No
   loops or reassignment (X3 stands). No type parameters on a lambda.
5. **Size (I).** About 250 lines in each checker: types checked once against
   annotations, coinductive Fits and Equal, a guard rule. About 120 lines in
   each evaluator: a frame for each call of a recursive lambda, `iterate`,
   `walk`, guards.
6. **Dependency.** Self-recursion needs annotations on lambda parameters and
   results. Program-declared recursive types need `type Name = ...` lines.
   Both come from plan 17 (D53). This plan assumes TypeScript's spelling:
   `const f = (n: Node, depth: int = 0): string[] => ...` and
   `type Node = {name: string, children: Node[]}`.

## 2. The problem, in v3 as it is today

| Task | Today | Verdict |
| --- | --- | --- |
| Page through a cursor | `range(10).reduce((acc, _) => if (acc.done) { acc } else { ... }, {items: [], cursor: null, done: false})` (#640 friction 5; the shape ran) | Works. Not obvious. Visits every spare element of the range |
| Collect every node of a tree of unknown depth | No form | Impossible |
| Rename a key at every level of a document | No form | Impossible |
| Total the sizes of each directory of a file tree | No form | Impossible |
| Type a tree | `{name: string, children: any[]}`, losing every level below the first. A host's recursive schema is `bad-type` (R112). A Go struct that contains itself is `bad-type` (`go/hosttypes.go`, `TypeOf`) | Lossy |
| Call yourself | `const f = n => n == 0 ? 0 : f(n - 1);` is `undefined-function` on `f` (ran). `01-language.md` 5 rule 6: a name is not in scope in its own value | Error |

The host can supply a tree walker, but it cannot know the shape each program
wants back. A walk that only flattens covers the first task and none of the
others.

## 3. Self-recursion

### 3.1 The rules

| # | Rule | Replaces |
| --- | --- | --- |
| 1 | The name of a `const`-bound lambda is in scope in its own body, its parameter defaults included. A `const` whose value is not a lambda keeps rule 6: `const x = x + 1;` reads an outer `x` | `01-language.md` 5 rule 6, for lambdas only |
| 2 | A lambda is **recursive** when its body, or a lambda written inside its body, calls its own name or passes its name as a lambda argument | New term |
| 3 | A recursive lambda must give a type to every parameter and to its result. A parameter with a default still needs a type. Otherwise the static rule `recursion-needs-types` fails, on the first reference to its own name | New static rule, `01-language.md` 14.4 |
| 4 | A lambda can call only names bound before it or itself. So two lambdas cannot call each other | Follows from `01-language.md` 5 rule 3 |
| 5 | D34 stands. A parameter cannot be called, and a lambda cannot be passed to a `const`-bound lambda. A recursive lambda may pass its own name to a library entry: `n.children.flatMap(walk)` | — |
| 6 | Each call of a recursive lambda has its own parameters and `const` names, even while an earlier call of it is in progress | `02-evaluation.md` 11.1, third fact |

Rule 3 is a static rule. It reads only the text: whether each parameter and
the result carry type text. So every implementation applies it, with or
without a type checker, as `01-language.md` 14.4 requires of every static
rule.

Rule 1 changes the meaning of one program shape. `const len = xs => len(xs) *
2; len([1, 2])` gives 4 today (ran): the inner `len` is the library's. Under
rule 1 it is a self-call. With no types it is now `recursion-needs-types`,
not a silent change. The message names the fix: give the lambda another name.

**Rule 1 is a Core amendment.** Added later, it would change the value of a
program that runs today, which test T1 of `14` 2.1 forbids a package to do.
So Core takes rule 1 and rule 3 before 3.0 is final. Core has no annotations,
so in Core every reference of a lambda to its own name is
`recursion-needs-types`. F16 and F17 then make the typed form a program: text
that was an error becomes one, which is what a package may do (`00` section 4,
"Where the five plans meet").
TypeScript reads `const f = x => f(x)` as a self-reference (M), so rule 1 is
the least surprise for its authors (C2, C6). The break costs little (C7).

### 3.2 Checking

| Matter | Rule |
| --- | --- |
| When the body is checked | A lambda with every parameter and its result typed is checked once, where it is written. Its parameters are bound to their types. The per-use rule of `05-types.md` 13.9 does not apply to it |
| The body against the result | The body must fit the result type. Otherwise the diagnostic plan 17 gives for an annotated result. This plan assumes `result-mismatch` on the body, by `05-types.md` 15.1 |
| A call by name, the self-call included | Each argument must fit its parameter's type (`arg-mismatch`). The call's type is the result type. The body is not checked again |
| The name as a lambda argument at an `Fn` | The `Fn`'s parameter types must fit the lambda's parameter types, and the lambda's result type must fit the `Fn`'s result after its `Var`s are solved (`arg-mismatch`, 13.5) |
| Sites (`05-types.md` 18) | The body's sites are recorded once, at the body check. A call records no site of its own beyond the call's own class |
| Limits | A self-call is not a body check, so it adds nothing to the counts of `05-types.md` 20.2 and 20.3. The checker never recurses through a self-call |

The checker's work on a recursive lambda is one body check. Inference by a
fixed point would avoid the annotations. It would also make the type of a
call depend on how many rounds the checker ran, and an error would appear far
from its cause. Annotations are what a TypeScript author writes on a
recursive function anyway, since TypeScript cannot infer the return type of
a function that calls itself without one (M).

### 3.3 Running

| Matter | Rule |
| --- | --- |
| Steps | A self-call charges as any call by name: 1 step, then its patterns and body (`02-evaluation.md` 11.2) |
| Depth | As any call by name (`02-evaluation.md` 8.4): 1 plus its nesting depth within the body, while it is in progress. So `n.children.flatMap(walk)` inside `walk` adds the `flatMap` call's measure and then 1 for each level |
| Limit reached | `depth-exceeded` at the call that passes the limit, after its step and before its parameters are bound (`02-evaluation.md` 8.4 item 7) |
| Frames | An implementation may give a fixed place to the parameters and names of a lambda that is not recursive, as today. A recursive lambda needs a place for each call in progress |

`02-evaluation.md` 8.5 item 3 already bounds an evaluator's recursion by the
depth limit `D` plus 100 for the source. That bound does not change. What
changes is that a checked program can now reach it through calls. Today
`02-evaluation.md` 8.4 item 10 says calls add at most 100 to the count in a
checked program, so only a walk of data reaches the default limit of 200.

### 3.4 What happens to E6, D6, D34, E9 and R3

| Rule | Before | After |
| --- | --- | --- |
| E6 | No recursion and no loops; every program terminates; work is bounded by text and data | No loops. Recursion only by self-call, with types. Every run ends at its value, a failure or a limit |
| D6 | No recursion or loops in 3.0 | Partly reversed: the "no recursion" half. Loops stay out (X3, X4) |
| D34 and E9 | A lambda is passed only to the library; a parameter cannot be called | Unchanged. E9's hole needs a call of a function that arrives as a value. With D34, every callee is still fixed by the text. Self-recursion adds an edge from a lambda to itself, and nothing else |
| `02-evaluation.md` 11.1 | The call graph has no cycle; at most `k` calls in progress; no lambda re-entered | True for every lambda that is not recursive. A recursive lambda is found from the text (3.1 rule 2), so an implementation still knows before the run which lambdas need frames |
| R3 | No collector, because no value refers to itself | Unchanged. A call cannot store a function or change a value, so a recursive call builds only acyclic values |
| R5 | Every recursion an implementation makes is bounded by a fixed number | Unchanged. The number is the depth limit, at most 1,000 (`03-host.md` 3.6.1) |
| R6 | Releasing a value is not a walk any limit covers | Unchanged. A deep value could already be built with `reduce`. Recursion builds one no deeper than the depth limit allows |

**Turning the step limit off.** A host may turn steps off (`03-host.md`
3.6.1). Today a program still ends then, though perhaps slowly. A recursive
lambda that calls itself twice at each level, to depth 200, makes 2^200 calls,
which no deadline-free run finishes. 18-5 keeps steps optional, and the host
chapter says that a host that turns steps off should set a deadline.

## 4. Recursive types

### 4.1 The declaration

| # | Rule | Replaces |
| --- | --- | --- |
| 1 | A named type may refer to itself and to named types declared after it in the same list. A set of named types that refer to one another is resolved together | F4 3.1 item 3 and 3.2 item 1 |
| 2 | **Guarded.** Every cycle must pass through a list element, a record field, a record's rest or a map's value. `type A = A \| null` and `type A = B; type B = A` are `bad-type` | New |
| 3 | A record alternative per type, as F4 3.2 requires, unless F5 makes it a tagged union | Unchanged |
| 4 | **Structural.** Two types with the same infinite unfolding are one type, whatever their names. Nothing becomes nominal | F4 3.1 item 1, extended |
| 5 | A schema with a `$ref` cycle that meets rule 2 gives a recursive named type. One that does not is `bad-type` | Reverses R112 |
| 6 | In Go, `TypeOf` gives a struct that contains itself through a slice, a map or a pointer a recursive type | `go/hosttypes.go` `TypeOf`; F1 6.2 |
| 7 | In a program, `type` lines (plan 17) follow rules 1 to 4. A `type` line may refer to a later `type` line of the same program | Plan 17 |

Rule 1 allows types that refer to each other, such as a `Dir` holding
`File[]` and `Dir[]`. Mutual recursion is out for lambdas, because it would
need forward references to names and a fixed point in the checker. For types
it costs nothing more than self-reference: the coinductive rules of 4.2 handle
any cycle.

### 4.2 The checker's rules

| Rule | How it treats a recursive name N |
| --- | --- |
| Unfold | N stands for its definition. A rule that needs N's structure looks one level into the definition |
| Fits(A, B) and Equal(A, B) | Coinductive. Each computation keeps a set of pairs in progress. A pair met again while in progress holds. A pair that has failed fails at once |
| Join(A, B) | If A fits B, B. If B fits A, A. Otherwise the structural Join of their unfoldings. A pair met again while in progress gives `any` |
| Merge, Truthy, Falsy, facts | Look at one unfolding. A recursive name in a field is kept as it stands |
| Conformance (F4 4) | The walk follows the data. Each list or map entered counts against the depth limit (F4 5.4), so a walk ends at the data's depth or at `depth-exceeded` |
| Depth of a type (`05-types.md` 2.5) | Measured on the unfolding, stopping at each recursive name's second occurrence. So `type Node = {children: Node[]}` has depth 2 |
| Size (`05-types.md` 20.7) | A recursive name counts 1 where it occurs inside its own definition |
| Bound | A computation of Fits, Equal or Join holds at most 4,096 pairs in progress. Passing it is `check-limit` on the node being checked |

The bound replaces the argument of `05-types.md` 20.6, "goes no deeper than
the deeper of its types, so 64 bounds it", which no longer holds for a type
with no finite depth. With the set of pairs, each computation visits at most
the product of the two types' sizes in distinct pairs (I). The fixed bound
makes the cut-off the same in every implementation (R5).

`any` from a Join of two recursive types that neither fit nor unify in one
step is sound, since `any` is the top type. It is rarely met (I): a
recursive lambda's result is annotated, so its branches are checked against
that type, not joined freely.

### 4.3 Printing

A recursive named type prints by its name, everywhere. A type that is not
recursive prints by its structure, as `05-types.md` 3.9 and D44 say. A
recursive type has no finite structure to print, and printing one level and
then the name would read as a different type. So the diagnostic is
`expected Node, found {name: string, children: int}`.

### 4.4 An example

```js
type Node = {name: string, size: int, children: Node[]};
const total = (n: Node): int =>
  n.size + n.children.map(total).sum();
const rename = (n: Node): Node =>
  {...n, name: n.name.toLowerCase(), children: n.children.map(rename)};
{total: total(tree), tree: rename(tree)}
```

## 5. `iterate` and `walk`

Both entries remain worth adding now that recursion exists.

- **Cost.** A recursive pagination writes `[...p.items, ...rest]` at each
  level. Each spread copies, and L6 charges what is written, so N pages of M
  items cost about N²M/2 units. `iterate` with `flatMap` costs about NM (I).
- **No type needed.** `walk` works on `any` data with no declared type. A
  recursive lambda must declare one, if only `any`.
- **One line where a lambda takes three.** The common case, "every node",
  should not need a named lambda with annotations.

| Entry | Signature | Group |
| --- | --- | --- |
| `iterate` | `(seed: T, next: (state: T, index: int) => T \| null, max: int) => T[]` | core |
| `walk` | `(root: T, children: (node: T, depth: int) => T[] \| null) => T[]` | core |

**`iterate`.**

1. The result starts as `[seed]`. While its length is less than `max`, `next`
   is called with the last element and the length less 1. A null result
   stops. Any other result is appended.
2. `max` of 0 gives `[]`, and `next` is not called. A negative `max` fails
   with `number-range`.
3. Steps: `2*c`, where `c` is the number of calls of `next`. Units: `1 + k`,
   where `k` is the length of the result. Each unit is charged before its
   element is appended, so a large `max` costs nothing until it is used.
4. The seed is evaluated before the call, as every argument is. With a host
   call as the seed, the first page is fetched once.

```js
const pages = iterate(issues.page(repo, null),
  p => p.next == null ? null : issues.page(repo, p.next), 10);
pages.flatMap(p => p.items)
```

**`walk`.**

1. The result is the nodes in pre-order: a node, then the walk of each of its
   children in order. `children` is called once for each node, with the node
   and its depth below the root (the root is 0).
2. A result of `children` that is null is treated as `[]`. One that is not a
   list fails with `wrong-kind`, template `result`, as `flatMap`.
3. Steps: `2*k`, where `k` is the number of nodes. Units: `1 + k`.
4. Depth: entering the children of a node is a walk level (`02-evaluation.md`
   8.4 item 5). A call of `children` is a library call of a lambda (8.4 item
   3), measured from `walk`'s call, plus the levels entered. So a cycle in
   host data stops at `depth-exceeded`. A wide graph stops at the step limit.
5. An implementation keeps its own stack of positions. It does not recurse
   natively for each level (R5).

```js
walk(doc, n => n.replies)[*].author.unique()
```

## 6. `match` guards

### 6.1 Syntax

```
Case  = ( Lit { "|" Lit } | "_" ) [ "if" "(" Expr ")" ] "=>" Expr .
```

```js
match order.status {
  "open" if (order.total > 1000) => "review",
  "open" => "ship",
  "held" | "late" if (order.priority) => "escalate",
  _ => "wait",
}
```

The parentheses follow `if (c)`, which a TypeScript author already knows
(C2, C6). They also end the guard before `=>` with no rule about lambdas. No
new node kind: the `Arm` node gains an optional `guard` field. The arm's span
runs to the end of its result as before. `01-language.md` 4.2 item 3 says
"An `Arm` with a test is not in Core". That text concerns a `match` with no
subject (D38) and is reworded to name both.

### 6.2 The rules

| # | Rule |
| --- | --- |
| 1 | The subject is evaluated once. The arms are tried in order. An arm is chosen when one of its literals equals the subject (or it is `_`) and then its guard, if any, is truthy |
| 2 | The guard is evaluated only after a literal of its arm has matched. If it is falsy, the next arm is tried |
| 3 | No arm chosen fails with `no-match`, as today |
| 4 | Steps: as today, 1 for each arm tested, and the literal comparisons. A guard charges what its expression charges, when it runs. No other charge |
| 5 | **Coverage.** An arm with a guard covers nothing. A `match` whose only `_` arm has a guard is not exhaustive (`non-exhaustive`) |
| 6 | **`unreachable-arm`.** Only an arm after an unguarded `_` is unreachable |
| 7 | **`duplicate-case`.** A literal is a duplicate only when an earlier arm without a guard lists it. `1 if (a) => x, 1 => y` is accepted |
| 8 | **Narrowing.** The facts of the guard being truthy hold in that arm's result. Its falsy facts do not carry to later arms, because the literal may not have matched. The arm's own narrowing (11.2 for `null`, F5 for a tag) holds in the guard |
| 9 | **`always-truthy`, `always-falsy`.** A guard is a tested position (`05-types.md` 10.2) |

A guard sees only names already in scope, since an arm binds no name. That is
enough: the subject is usually a path, as in `order.status`, and the guard
reads the rest of `order`.

## 7. Decisions

| # | Decision | Recommendation | Strongest alternative |
| --- | --- | --- | --- |
| 18-1 | Self-recursion | **Settled (owner, 2026-11-14):** a `const`-bound lambda may call itself | Both reviewers: no recursion, only `iterate` and `walk`. It keeps termination by construction, but transforms of trees remain impossible |
| 18-2 | How a lambda names itself | **Settled (owner, 2026-11-14):** its own name is in scope in its body (3.1 rule 1), as in TypeScript. This part is a Core amendment (below) | A marker, such as `const rec f = ...`. It changes no current program, but adds a word nobody else writes |
| 18-3 | Types on a recursive lambda | **Settled:** required on every parameter and the result, as a static rule with a new code `recursion-needs-types` | Inference by a fixed point. No annotations, but the checker's result depends on its rounds |
| 18-4 | Mutual recursion | **Settled:** out. It falls out of scope order | Forward references among the `const` lambdas of one scope |
| 18-5 | Steps turned off | Allowed. `03-host.md` advises a deadline | Refuse a recursive program when steps are off (`bad-limit` at compile) |
| 18-6 | Recursive named types | **Settled:** through a list, a field, a rest or a map. Mutual recursion of types allowed (recommendation) | Self-reference only. Simpler to explain, but a file tree of `Dir` and `File` must be squeezed into one type |
| 18-7 | Equality and fit | Structural and coinductive, with 4,096 pairs. Join gives `any` on a repeated pair | Nominal recursive names: cheaper, but it breaks F4 3.1 item 1, "nothing is nominal" |
| 18-8 | Printing | A recursive type prints by its name. D44 stands for every other type | Print one unfolding, then the name inside it |
| 18-9 | `iterate` | **Settled:** in, with the signature of 5 | `unfold(seed, step)`, where step gives `{value, next}`. More general, but every caller builds a record |
| 18-10 | `walk` | **Settled:** in, pre-order, with depth given to the callback | Leave it to recursion. One way to write each thing, but it costs a typed lambda and quadratic units |
| 18-11 | `match` guards | **Settled (owner, 2026-11-14):** in | No guards. `else if` covers it (D10). The reviewers' preference |
| 18-12 | Guard spelling | `if (c)` with parentheses | `if c` without them, as in Rust (M). Shorter, but then a guard needs a rule for where it ends before `=>` |

## 8. What changes in other documents

| Document | Section | Change |
| --- | --- | --- |
| `00-overview.md` | E6, D6 | Reword as in 3.4. D6 is partly reversed by D54 |
| `00-overview.md` | R3, R5 | Note that self-recursion keeps both, and why |
| `00-overview.md` | Library row (about line 315), 5.2 (about line 471) | 78 entries become 80. 59 in the default core become 61 |
| `00-overview.md` | D38 | Guards cover what `match` by conditions was for. Recommend dropping D38's form |
| `01-language.md` | 3.1 `Case`; 4.2 `Arm` and item 3; 5 rule 6; 8.1 rule 3; 9.3 rule 7; 14.4 | Grammar, the `guard` field, the scope rule for a lambda's own name, guards, and the static rule `recursion-needs-types` |
| `02-evaluation.md` | 8.4 item 10 | A checked program's calls may now reach the depth limit |
| `02-evaluation.md` | 11.1 | The three facts hold for lambdas that are not recursive |
| `02-evaluation.md` | The `match` cost row | A guard charges as it runs |
| `03-host.md` | 3.6.1 | Advise a deadline when steps are off |
| `03-host.md` | 3.9 "The stack a binding needs" | The deepest cases include self-recursion to depth 1,000, not only walks |
| `04-library.md` and `suite/library.json` | Core list; counts | `iterate`, `walk`. Two entries. Cost rows as in 5 |
| `05-types.md` | 2.5, 3.9, 11.2, 13.9, 16.1, 20.6, 20.7, a new 20.8 | Recursive types, printing by name, guard rules, a typed lambda checked once, the new code, the bound of 4,096 pairs |
| `spec/packages/f01-host-types.md` | 3, 6.2 | A recursive `$ref` and a Go struct that contains itself |
| `spec/packages/f04-conversion.md` | 3.1 items 1 and 3; 3.2 the "declared later or its own" row and item 1; 5.4 item 4 | Recursive named types, the guard rule, and a walk that follows the data |
| `spec/packages/f08-errors.md` | 3.1 item 2; 3.5 | Check codes 29 become 30, templates 71 become 72: the template for `recursion-needs-types` |
| `15-build-rulings.md` | R112 | Superseded by 4.1 rule 5 |
| `suite/` | `core-check/`, `core-run/`, a package directory | Cases for each rule of 3 to 6: self-recursion to the depth limit, the scope break of 3.1, coinductive Fits, the bound of 4,096, a `$ref` cycle, guards and coverage |
| `go/hosttypes.go` | `TypeOf` | A struct that contains itself |

The counts that change are library entries (78 to 80), check codes (29 to 30,
counting F4's), check templates (71 to 72) and the static codes of
`05-types.md` 16.1 (11 to 12). No reserved word, no run code, no node kind
and no `parse` code is added: `if` is already reserved, and a guard written
in a program without this package is a `parse` error today (ran:
`unexpected-token` on `if`).

**Package.** The four additions form one follow-on package, F17. It requires
F4 (named types) and F16 (plan 17: annotations, `type` lines). The scope rule
of 3.1 rule 1 is in Core (3.1). `iterate`, `walk` and guards need neither F4
nor plan 17. They could go first if plan 17 is late.

## 9. Cost to each implementation

| Part | Go | TypeScript | Rust |
| --- | --- | --- | --- |
| Frames for recursive lambdas | The closure compiler gives a recursive lambda a frame per call instead of fixed slots | The same | The same. Borrowed host data is unaffected: a frame holds values the run owns or lends |
| Native stack at depth 1,000 through calls | Goroutine stacks grow (M) | The risk. One Risor call is several JavaScript frames, and the default V8 stack holds on the order of 10,000 frames (M). 1,000 levels may not fit | A thread's default stack is 2 MiB, and the main thread's 8 MiB (M). The binding documents its need, or runs the evaluator on a thread with a set stack size, from `std`, with no dependency |
| Checker | Coinductive Fits, Equal and Join with a pair set; a typed lambda checked once; guards | The same | The same |
| Library | `iterate` and `walk` with an explicit stack | The same | The same |

The stack is the one claim that could fail C1. Before 18-1 is built, each
binding should run `const d = (n: int): int => n == 0 ? 0 : 1 + d(n - 1);
d(990)` at depth limit 1,000 on its documented stack. If the TypeScript
implementation cannot, the options are, in order: fewer JavaScript frames for
each Risor call; an explicit stack for calls of recursive lambdas only; a
lower maximum depth limit for every implementation, which `03-host.md` 3.6.1
would then state.

## 10. What I did not verify

- The stack of the TypeScript and Rust implementations at 1,000 levels of
  calls. Not measured. Section 9 says how to measure it.
- That the error sites of `05-types.md` 18 need nothing more than "recorded
  once" for a lambda checked against its annotations. That needs a reading
  of 18 against plan 17's typed lambdas.
- The exact code plan 17 gives for a body that does not fit its annotated
  result. Section 3.2 assumes `result-mismatch`.
- The number 4,096. It is a guess sized to the type size limit of 100,000,
  not a measurement. The suite's cases should probe it.
- That `any` from a repeated pair in Join is rare. It is inference from the
  fact that a recursive lambda's result is annotated.
- TypeScript's behaviour for `const f = x => f(x)` and the need for a return
  annotation on a recursive function. From memory; TypeScript was not run.
