# Risor v3 design: recursion, recursive types, function parameters and `match` guards

Status: plan, accepted by the owner on 2026-11-14 (D54 in `00-overview.md`),
and revised on 2026-11-19. The owner chose self-recursion over the two
independent reviews of #640 and #641, which both preferred bounded library
entries alone. The owner also accepted `match` guards. On 2026-11-19 the owner
dropped `iterate` and settled passing functions to user-defined functions
(#644a, D57), which section 7 adds. Sub-decisions 18-1 to 18-4, 18-6, 18-9 to
18-11 and 18-13 are settled as stated; section 10 marks the others as rulings
or recommendations. Nothing here is built. The spec text is in sections 3 to
8 in a form ready to move into the package chapter.

**Evidence.** **(R)** read in the specification or a sibling document at the
place cited. **(ran)** run with the Go CLI built from `main` at `aeaaf1f`
(Go 1.26.0). **(measured)** the stack figures of D58, measured on 2026-11-19
at `main` `aeaaf1fd`. **(M)** from memory. **(I)** inferred. Decision numbers
such as D34 and E6 refer to `00-overview.md`. R numbers refer to
`15-build-rulings.md`, except R3, R5 and R6, which are in the overview's
section 2.8.

## 1. Summary

1. **The problem.** A program cannot walk data of unknown depth: a comment
   thread, a file tree, a JSON document of any shape. #640 met it (friction
   5). And a program cannot write a reusable helper that takes a function,
   such as `sumBy(xs, f)`: a parameter cannot be called (#644). For a language
   whose main uses include a query over JSON, both are gaps in C4.
2. **Five additions, one package (F17).**
   - A `const`-bound lambda may call itself, when its parameters and its
     result have types (section 3).
   - A named type may refer to itself, through a list, a record field or a map
     (section 4).
   - One library entry: `walk` for trees (section 5).
   - A guard on a `match` arm: `"open" if (o.total > 100) => ...`
     (section 6).
   - A parameter with a function type may be called, and a lambda may be
     passed to it (section 7, D57).
3. **What it costs the language.** "Every program terminates by
   construction" (E6) becomes "every run ends at its value, a failure or a
   limit". The call graph may have a cycle, and every cycle passes through a
   name: a lambda that calls itself. Values stay acyclic, and a function is
   still never stored, returned or given to the host, so no implementation
   needs a collector (R3 stands). Each call gets its own frame (section 8).
4. **What it does not do.** No mutual recursion. No function values: a
   function is never stored, returned or passed to a host function (S3). No
   function that takes a function as a parameter type: function parameters are
   first-order. No loops or reassignment (X3 stands). No type parameters on a
   lambda yet (7.6). No `iterate` (5).
5. **Size (I).** About 300 lines in each checker: types checked once against
   annotations, coinductive Fits and Equal, function parameters, a guard rule.
   About 200 lines in each evaluator: a frame for each call, `walk`, guards.
6. **Dependency.** Self-recursion and function parameters need annotations on
   lambda parameters and results. Program-declared recursive types need
   `type Name = ...` lines. Both come from slice D1 of plan 17 (D53). This plan
   assumes TypeScript's spelling: `const f = (n: Node, depth: int = 0):
   string[] => ...` and `type Node = {name: string, children: Node[]}`.
7. **Four steps** (track E of `14` 5):

| Step | Holds | Needs |
| --- | --- | --- |
| E0 | `walk` and `match` guards | Nothing. It may go before D1 |
| E1 | Typed self-recursion, and per-call frames in every evaluator | D1 |
| E2 | Recursive named types, in programs, in F1 and in Go's `TypeOf` | D1, F4 |
| E3 | Function parameters (section 7) | D1, E1's frames |

## 2. The problem, in v3 as it is today

| Task | Today | Verdict |
| --- | --- | --- |
| Page through a cursor | `range(10).reduce((acc, _) => if (acc.done) { acc } else { ... }, {items: [], cursor: null, done: false})` (#640 friction 5; the shape ran) | Works. Not obvious. With this plan a recursive lambda does it, to the depth limit |
| Collect every node of a tree of unknown depth | No form | Impossible |
| Rename a key at every level of a document | No form | Impossible |
| Total the sizes of each directory of a file tree | No form | Impossible |
| Type a tree | `{name: string, children: any[]}`, losing every level below the first. A host's recursive schema is `bad-type` (R112). A Go struct that contains itself is `bad-type` (`go/hosttypes.go`, `TypeOf`) | Lossy |
| Call yourself | `const f = n => n == 0 ? 0 : f(n - 1);` is `undefined-function` on `f` (ran). `01-language.md` 5 rule 6: a name is not in scope in its own value | Error |
| Write a helper that takes a function | `const sumBy = (xs, f) => xs.map(f).sum();` is an error: `f` is a value, so `xs.map(f)` is `arg-mismatch` when checked and `wrong-kind` when run; `f(x)` would be `call-of-parameter` (`01-language.md` 8.2) | Error |

The host can supply a tree walker, but it cannot know the shape each program
wants back. A walk that only flattens covers the first task and none of the
others.

## 3. Self-recursion

### 3.1 The rules

| # | Rule | Replaces |
| --- | --- | --- |
| 1 | The name of a `const`-bound lambda is in scope in its own body, its parameter defaults included. A `const` whose value is not a lambda keeps rule 6: `const x = x + 1;` reads an outer `x` | `01-language.md` 5 rule 6, for lambdas only |
| 2 | A lambda is **recursive** when its body or a parameter default, or a lambda written inside either, calls its own name or passes its name as a lambda argument. So `const f = (n = f()): int => n;` is recursive | New term |
| 3 | A recursive lambda must give a type to every parameter and to its result. A parameter with a default still needs a type. Otherwise the static rule `recursion-needs-types` fails, on the first reference to its own name | New static rule, `01-language.md` 14.4 |
| 4 | A lambda can call only names bound before it or itself. So two lambdas cannot call each other | Follows from `01-language.md` 5 rule 3 |
| 5 | A recursive lambda may pass its own name to a library entry, `n.children.flatMap(walk)`, and to an `Fn` parameter of another lambda (7.1), unless it has an `Fn` parameter itself (7.1 rule 8) | — |
| 6 | Each call of a lambda has its own parameters and `const` names, even while an earlier call of it is in progress (section 8) | `02-evaluation.md` 11.1, third fact |

Rule 3 is a static rule. It reads only the text: whether each parameter and
the result carry type text. So every implementation applies it, with or
without a type checker, as `01-language.md` 14.4 requires of every static
rule.

Rule 1 changes the meaning of one program shape. `const len = xs => len(xs) *
2; len([1, 2])` gives 4 today (ran): the inner `len` is the library's. Under
rule 1 it is a self-call. With no types it is now `recursion-needs-types`,
not a silent change. The message names the fix: give the lambda another name,
or give it types.

**Rule 1 is a Core amendment, timed with slice D1.** Added by a package, it
would change the value of a program that runs today, which test T1 of `14`
2.1 forbids a package to do. So Core takes rules 1 and 3 before 3.0 is final.
They land in the same change as annotations (slice D1 of plan 17), in all
three implementations, and not earlier with the other Core amendments: a
build that had rule 3 and no annotations would tell an author to add types
that the parser refuses. In a build that offers Core and not F16, every
reference of a lambda to its own name is `recursion-needs-types`; no release
is such a build, because D1 lands in all three at once. E1 lands before the
alpha that carries D1 is tagged, so no release accepts the types of a
self-reference and then refuses the self-reference (my reading; `14` 5 track
E). TypeScript reads `const f = x => f(x)` as a self-reference (M), so rule 1
is the least surprise for its authors (C2, C6). The break costs little (C7).

### 3.2 Checking

| Matter | Rule |
| --- | --- |
| When the body is checked | A lambda with every parameter and its result typed is checked once, where it is written. Its parameters are bound to their types. The per-use rule of `05-types.md` 13.9 does not apply to it |
| The body against the result | The body must fit the result type, else `result-mismatch` at the body (`17` 3.2, `05-types.md` 15.1) |
| A call by name, the self-call included | Each argument must fit its parameter's type (`arg-mismatch`). The call's type is the result type. The body is not checked again |
| The name as a lambda argument at a library `Fn` | The `Fn`'s parameter types must fit the lambda's parameter types, and the lambda's result type must fit the `Fn`'s result after its `Var`s are solved (`arg-mismatch`, 13.5) |
| The name at an `Fn` parameter of a user lambda | 7.3 |
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
| Frames | Section 8 |

**Depth is counted in levels, not calls.** An author needs this to know how
deep a recursion may go. Each call counts 1 plus the levels of source open
around it in its body (`02-evaluation.md` 8.4 item 2), and the right operand of
a binary operator opens a level (`01-language.md` 13.1). So `1 + f(n - 1)`
costs 2 for each call, and `f(n - 1) + 1` costs 1. A typical tree recursion
through `map`, as `n.children.map(c => total(c))`, costs 2 to 3 for each
level of the tree. At the default limit of 200, that is roughly 66 to 100
levels of a tree (I, from the rules; not run). A host that needs more sets a
higher limit, up to 400 (D58).

`02-evaluation.md` 8.5 item 3 already bounds an evaluator's recursion by the
depth limit `D` plus 100 for the source. That bound does not change. What
changes is that a checked program can now reach it through calls. Today
`02-evaluation.md` 8.4 item 10 says calls add at most 100 to the count in a
checked program, so only a walk of data reaches the default limit of 200.
Item 10 is amended by `17` 3.4.

### 3.4 What happens to E6, D6, D34, E9 and R3

| Rule | Before | After |
| --- | --- | --- |
| E6 | No recursion and no loops; every program terminates; work is bounded by text and data | No loops. Recursion only by self-call, with types. Every run ends at its value, a failure or a limit |
| D6 | No recursion or loops in 3.0 | Partly reversed: the "no recursion" half. Loops stay out (X3, X4) |
| D34 and E9 | A lambda is passed only to the library; a parameter cannot be called | Revised by D57 (section 7): a lambda may also be passed to an `Fn` parameter of a user lambda, and that parameter may be called. E9's hole stays closed: an `Fn` takes no function, so `x(y => x(y))` cannot be written, and every cycle of calls passes through a name (7.5) |
| `02-evaluation.md` 11.1 | The call graph has no cycle; at most `k` calls in progress; no lambda re-entered | True only for a program with no recursive lambda and no `Fn` parameter. Even a lambda that is not recursive can be re-entered through an `Fn` parameter (section 8) |
| R3 | No collector, because no value refers to itself | Unchanged. A call cannot store a function or change a value, so a recursive call builds only acyclic values |
| R5 | Every recursion an implementation makes is bounded by a fixed number | Unchanged. The number is the depth limit, at most 400 (D58, `03-host.md` 3.6.1) |
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
| The same name | Fits(N, N), Equal(N, N) and Join(N, N) = N hold at once, with no unfolding. Sound, since a name has one definition |
| Fits(A, B) and Equal(A, B) | Coinductive. Each computation keeps a set of pairs in progress. A pair met again while in progress holds. A pair that has failed fails at once |
| Join(A, B) | If A fits B, B. If B fits A, A. Otherwise the structural Join of their unfoldings. A pair met again while in progress gives `any` |
| Merge, Truthy, Falsy, facts | Look at one unfolding. A recursive name in a field is kept as it stands |
| Conformance (F4 4) | The walk follows the data. Each list or map entered counts against the depth limit (F4 5.4), so a walk ends at the data's depth or at `depth-exceeded` |
| Depth of a type (`05-types.md` 2.5) | Measured on the unfolding, stopping at each recursive name's second occurrence. So `type Node = {children: Node[]}` has depth 2 |
| Size (`05-types.md` 20.7) | A recursive name counts 1 where it occurs inside its own definition |
| Order | **Normative.** A computation visits pairs depth first: alternatives in the order of printing (`05-types.md` 3.9), and within two records the fields in key order by code point, then the rests. So the pair that passes the bound is the same pair in every implementation. The exact order is fixed in the chapter (recommendation) |
| Bound | A computation of Fits, Equal or Join holds at most 4,096 pairs in progress. Passing it is `check-limit` on the node being checked |

The bound replaces the argument of `05-types.md` 20.6, "goes no deeper than
the deeper of its types, so 64 bounds it", which no longer holds for a type
with no finite depth. With the set of pairs, each computation visits at most
the product of the two types' sizes in distinct pairs (I). The fixed bound
and the fixed order make the cut-off the same in every implementation (R5).

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

## 5. `walk`

`walk` stays worth adding now that recursion exists.

- **No type needed.** `walk` works on `any` data with no declared type. A
  recursive lambda must declare one, if only `any`.
- **One line where a lambda takes three.** The common case, "every node",
  should not need a named lambda with annotations.
- **Cost.** A recursion that flattens writes `[...n, ...rest]` at each level.
  Each spread copies, and L6 charges what is written, so the units grow with
  the square of the nodes. `walk` charges each node once (I).
- **No native recursion.** It keeps its own stack (rule 5 below), so it costs
  no stack in any implementation.

| Entry | Signature | Group |
| --- | --- | --- |
| `walk` | `(root: T, children: (node: T, depth: int) => T[] \| null) => T[]` | core |

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

**`iterate` is dropped** (2026-11-19). It was `iterate(seed, next, max)`,
for cursors. It is an unfold, which fails the readability test of `21` 1: a
TypeScript programmer does not read it without an explanation. A recursive
lambda covers a cursor up to the depth limit.

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

## 7. Function parameters (D57)

#644a asks that a function be passed to a user-defined function. The owner
settled it on 2026-11-19, first-order only. It revises D34, E9 and
`01-language.md` 8.2 rules 2 and 4. A function still cannot be stored,
returned or passed to a host function: R3 and S3 hold.

```js
const sumBy = (xs: Order[], f: (o: Order) => float): float =>
  xs.map(f).sum();
const apply = (f: (x: int) => int, x: int): int => f(x) + x;
{total: sumBy(orders, o => o.qty * o.price), n: apply(n => n * 2, 3)}
```

The syntax is plan 17's (3.1): `Param = ... ":" PFn`. A `PFn`'s parameters
are `PType`, never `Fn`, so a function parameter cannot take a function.

### 7.1 Static rules

Every rule here is a static rule (`05-types.md` 22): it reads the text and
the annotations, never a type, and every compile applies it, with or without
the type rules. That is what keeps an unchecked run from calling null or a
lambda with the wrong arguments. So an `Fn` annotation is read by every
compile (`17` 3.2).

| # | Rule | Code | Replaces |
| --- | --- | --- | --- |
| 1 | A parameter may be called only when it is annotated with an `Fn`. An unannotated parameter that is called stays `call-of-parameter`, with a hint (7.2) | `call-of-parameter` | `01-language.md` 8.2 rule 4 |
| 2 | A lambda argument (a lambda written there, the name of a `const`-bound lambda, or an `Fn` parameter) may be passed to a `const`-bound lambda only at an `Fn` parameter | `lambda-to-lambda` elsewhere | 8.2 rule 2 |
| 3 | An argument at an `Fn` parameter of a `const`-bound lambda that is not a lambda argument is an error | Recommendation: `arg-mismatch`, static here as `arity` is static at a call by name. Code to fix in the chapter | `05-types.md` 13.2, which makes it a type rule |
| 4 | An `Fn` parameter is not a value: `[f]`, `{f}`, `f == g`, `f ?? g`, `` `${f}` ``, `const g = f;`, `type(f)`, and `f` as a lambda's body, so it cannot be returned | `not-a-value` | — |
| 5 | An `Fn` parameter is a name, never a pattern, and has no default. The grammar gives this (`17` 3.1) | `parse` | — |
| 6 | An `Fn` parameter is not passed to a host function or method (S3). A host signature has no `Fn` (`05-types.md` 13.1), so this is the existing rule for a lambda argument at a parameter that is not an `Fn` | `not-a-value` | — |
| 7 | **(b)** A call of an `Fn` parameter has the count of arguments its `Fn` lists, exactly | `arity` | — |
| 8 | **(a)** A lambda that has an `Fn` parameter is not passed as a lambda argument anywhere: not to a user lambda, not to a library entry | Recommendation: `lambda-to-lambda`, with a template of its own | — |
| 9 | **(c)** A lambda argument at an `Fn` parameter, and an `Fn` parameter forwarded to another `Fn` parameter, has no more parameters than the receiving `Fn` lists | `arity` | — |
| 10 | **(d)** At run time, the shape test of `xs.map(f)` with `f` an `Fn` parameter counts the parameters its `Fn` lists (`01-language.md` 7.2 test 5). Test 3 accepts an `Fn` parameter as a lambda | `wrong-kind` | 7.2 tests 3 and 5 |

Rules 7 to 10 are the review's (a) to (d) of 2026-11-19. Rule 8 is what
keeps every function parameter first-order at run time: without it, an
unchecked run could pass `apply` itself where an `(x: int) => int` is
expected. A lambda bound to an `Fn` with fewer parameters than the `Fn`
lists drops the extra arguments, as a lambda that a library calls does
(`01-language.md` 8.3 item 2).

### 7.2 Messages

| Written | Diagnostic | New template (recommendation) |
| --- | --- | --- |
| `(xs, f) => f(xs)` | `call-of-parameter` | `fn-hint`: `{name} is a parameter; to call it, give it a function type, as {name}: (x: int) => int` |
| `(xs, f) => xs.map(f)` | `arg-mismatch` when checked; `wrong-kind` when run (`05-types.md` 13.2) | `arg-mismatch` `parameter-at-fn`: `{name} is a value; to pass it as a function, give it a function type` |
| `xs.map(apply)`, `apply` with an `Fn` parameter | rule 8 | `lambda-to-lambda` `higher-order`: `{name} takes a function, so it cannot be passed as one` |

The adjustments of 2026-11-19 said the second row "stays
`call-of-parameter`". It does not: `f` is passed there, not called, and
`05-types.md` 13.2 gives `arg-mismatch` (my reading). Both rows get a hint.

### 7.3 Checking

| Matter | Rule |
| --- | --- |
| `f` in the body | Has its `Fn` type. A call `f(a1, ..., an)`: each argument must fit the `Fn`'s parameter (`arg-mismatch`); the call's type is the `Fn`'s result |
| A call of an `Fn` whose result is `unknown` | Type `any`. Such an `Fn` is for passing on to a library predicate, `xs.filter(keep)`, where `unknown` means what it means there (recommendation) |
| A lambda written at an `Fn` parameter, or an unannotated `const`-bound lambda named there | Checked against the `Fn` as `05-types.md` 13.5 step 3 checks a lambda against a library `Fn`, with no `Var` to solve |
| An annotated lambda, or an `Fn` parameter, passed at an `Fn` parameter | Fitted: each of the receiving `Fn`'s parameter types must fit the passed one's (contravariant), and the passed one's result must fit the receiving result (covariant). `arg-mismatch` otherwise |
| `xs.map(f)` with `f` an `Fn` parameter | 13.5 step 3 with `f`'s `Fn` as the lambda's annotated type, fitted as the row above |
| A lambda with an `Fn` parameter and other parameters unannotated | Checked at each use (13.9), with `f` bound to its `Fn` type |
| Sites | A lambda passed at an `Fn` is checked once for each argument where it stands, and its body's sites are recorded there |

### 7.4 Running

| Matter | Rule |
| --- | --- |
| A lambda argument in a call of a user lambda | Charges 1 step where it stands, as a lambda argument of a library call does (`02-evaluation.md` 11.3 item 1). Recommendation; the suite pins it |
| A call `f(x)` of an `Fn` parameter | Charges as a call by name (11.2): 1 step, then the patterns and body of the lambda bound to `f`. Its depth is measured at the call site (8.4 items 1 and 2) |
| `xs.map(f)` | The library measure (8.4 item 3) |
| What `f` is bound to | The lambda passed, with the frames where it was written (section 8) |

### 7.5 Why this adds no loop

A cycle of calls needs a function that reaches itself. A lambda reaches
itself by name only (3.1 rule 2, typed). Through an `Fn` it cannot: an `Fn`
takes no function, so self-application `f(f, x)` cannot be written, checked
or not (rules 7 to 9). So a program with no recursive lambda still ends by
construction, and every unbounded run passes through a name that the depth
limit bounds (I, from the rules). The call graph is still fixed by the text
up to the lambdas bound to `Fn` parameters, and those are lambdas written in
the text.

### 7.6 A known gap: functions of one type only

An `Fn` annotation has no type variable: the one-letter names stay reserved
(`05-types.md` 3.2, `17` 3.1). So a reusable `sumBy` over any list must be
written with `any`:

```js
const sumBy = (xs: any[], f: (x: any) => float): float => xs.map(f).sum();
```

Its sites are `dynamic`, and a host that rejects the class `dynamic` refuses
it.

**Planned follow-up, a recommendation the owner has not settled.** A lambda
that is not recursive may use type variables in its annotations. Each call
solves them by the rules that library signatures already use (`05-types.md`
13.4 to 13.6), and the body is checked at each use (13.9), as an unannotated
lambda is today. It only adds: a type variable in an annotation is an error
until then. It is not in track E.

## 8. Frames

Today's evaluators give each parameter and each `const` name one fixed place
for the whole run (`02-evaluation.md` 10.2, row "Bind each call to its callee
when compiling, keep each binding in one fixed place"; 11.1, third fact). The
Go, TypeScript and Rust evaluators all do (`go/internal/eval/compile.go`,
`ts/src/eval/compile.ts`, `rust/src/eval/code.rs`, as the adjustments of
2026-11-19 report). Both additions break the fact it rests on. Recursion
re-enters a lambda. So does an `Fn` parameter, even with no recursion:

```js
const apply = (f: (x: int) => int, x: int): int => f(x) + x;
apply(y => apply(z => z, y + 10), 1)  // 23
```

The inner call of `apply` binds `x` to 11 while the outer call, with `x` 1,
is still in progress. With one fixed place for `x`, the outer call reads 11
when `f` returns and gives 33.

| Rule | Text |
| --- | --- |
| The reference path | Each call of a lambda gets its own frame of parameters and `const` names. A lambda reads an outer name from the frame of the call where it was written, by a static link or a closure |
| The suite | Every implementation runs the whole suite in frame mode |
| Fixed places | An optimization only, for a program with no recursive lambda and no `Fn` parameter, under `02-evaluation.md` 10.2. An implementation keeps it only if benchmarks show it is needed |
| Values | Still acyclic. A frame holds values the run owns or lends; no function escapes the call that received it, so R3 holds, and Rust's borrowed frames hold |

**Ruling to record** (2026-11-19): per-call frames are the reference path.
The first draft said the reverse, frames only for recursive lambdas and the
lambdas inside them. The `apply` example shows that is not enough once `Fn`
parameters exist, and one path for every program is what three
implementations can keep alike.

## 9. Stack

The stack is the one claim that could fail C1. D58 settles the largest depth
a host may set at 400, from 1,000, on these measurements (measured
2026-11-19, `main` `aeaaf1fd`: chains of `const` lambdas compiled without the
type rules, depth limit 1,000; each figure is the deepest limit that ran with
no stack fault):

| Build | Direct call | Through `map` | Through `flatMap` |
| --- | --- | --- | --- |
| Go | 1,000 | 1,000 | 1,000 |
| Node 22, default stack | 1,000 | 643 | 604 |
| Chrome 157, main thread | 1,000 | 992 | 992 |
| Chrome 157, Web Worker | 1,000 | 435 | 497 |
| Rust release, 2 MiB thread | runs | aborts (about 2.7 MiB needed at 1,000) | aborts |
| Rust debug | — | about 5 MiB needed at depth 200 | — |

Safari and Firefox were not measured.

1. The fallback is 300, if the TypeScript binding cannot run the deepest
   forms at 400 with 1.5 times headroom in a browser Web Worker.
2. **Not met today, either way (I, from the table).** 1.5 times 400 is 600,
   and 1.5 times 300 is 450. The Web Worker ran `map` to 435. So the
   TypeScript binding must spend fewer JavaScript frames on a Risor call
   through a library entry before either number holds.
3. **The stack test of `03-host.md` 3.9.1** must cover calls that a library
   entry makes of a lambda (`map`, `flatMap`), not only direct calls, and
   pass with 1.5 times headroom.
4. The Rust binding runs the evaluator on a thread with a set stack size,
   from `std`, with no dependency, or documents its need. A debug build needs
   more than a 2 MiB thread even at the default limit (measured).

## 10. Decisions

| # | Decision | Recommendation | Strongest alternative |
| --- | --- | --- | --- |
| 18-1 | Self-recursion | **Settled (owner, 2026-11-14):** a `const`-bound lambda may call itself | Both reviewers: no recursion, only bounded library entries. It keeps termination by construction, but transforms of trees remain impossible |
| 18-2 | How a lambda names itself | **Settled (owner, 2026-11-14):** its own name is in scope in its body (3.1 rule 1), as in TypeScript. A Core amendment, timed with slice D1 (3.1) | A marker, such as `const rec f = ...`. It changes no current program, but adds a word nobody else writes |
| 18-3 | Types on a recursive lambda | **Settled:** required on every parameter and the result, as a static rule with a new code `recursion-needs-types` | Inference by a fixed point. No annotations, but the checker's result depends on its rounds |
| 18-4 | Mutual recursion | **Settled:** out. It falls out of scope order | Forward references among the `const` lambdas of one scope |
| 18-5 | Steps turned off | Allowed. `03-host.md` advises a deadline | Refuse a recursive program when steps are off (`bad-limit` at compile) |
| 18-6 | Recursive named types | **Settled:** through a list, a field, a rest or a map. Mutual recursion of types allowed (recommendation) | Self-reference only. Simpler to explain, but a file tree of `Dir` and `File` must be squeezed into one type |
| 18-7 | Equality and fit | Structural and coinductive, with the same-name shortcut, a normative order and 4,096 pairs. Join gives `any` on a repeated pair | Nominal recursive names: cheaper, but it breaks F4 3.1 item 1, "nothing is nominal" |
| 18-8 | Printing | A recursive type prints by its name. D44 stands for every other type | Print one unfolding, then the name inside it |
| 18-9 | `iterate` | **Dropped (owner, 2026-11-19):** an unfold, which fails the readability test | Keep it, as accepted on 2026-11-14 |
| 18-10 | `walk` | **Settled:** in, pre-order, with depth given to the callback | Leave it to recursion. One way to write each thing, but it costs a typed lambda and quadratic units |
| 18-11 | `match` guards | **Settled (owner, 2026-11-14):** in | No guards. `else if` covers it (D10). The reviewers' preference |
| 18-12 | Guard spelling | `if (c)` with parentheses | `if c` without them, as in Rust (M). Shorter, but then a guard needs a rule for where it ends before `=>` |
| 18-13 | Function parameters | **Settled (owner, 2026-11-19, D57):** first-order `Fn` parameters; never stored, returned or given to the host | #644's full proposal: functions passed to host functions too. Rejected for the reasons of `19` 4 |
| 18-14 | The static rules of 7.1, (a) to (d) among them | **Ruling (2026-11-19)**, with D57, so that an unchecked run stays safe | Leave them to the type rules. An unchecked run could then call null |
| 18-15 | The codes of 7.1 rules 3 and 8, and the templates of 7.2 | Recommendation | New static codes. Each costs a row in every count |
| 18-16 | A lambda argument to a user lambda charges 1 step | Recommendation (7.4) | Charge nothing. Then it differs from a library call |
| 18-17 | Per-call frames as the reference path | **Ruling (2026-11-19)** (section 8) | Frames only where the text needs them. Two paths in each evaluator, and the second is wrong once `Fn` parameters exist |
| 18-18 | Type variables in a non-recursive lambda's annotations | Recommendation, not settled (7.6). Additive; not in track E | Leave `Fn` monomorphic. Reusable helpers then use `any` |

## 11. What changes in other documents

| Document | Section | Change |
| --- | --- | --- |
| `00-overview.md` | E6, D6, E9, D34 | Reword as in 3.4. D6 partly reversed by D54; D34 and E9 revised by D57 |
| `00-overview.md` | R3, R5 | Note that both hold, and why; R5's largest depth is 400 (D58) |
| `00-overview.md` | D38 | Guards cover what `match` by conditions was for. Recommend dropping D38's form |
| `01-language.md` | 3.1 `Case`; 4.2 `Arm` and item 3; 5 rule 6; 7.2 tests 3 and 5; 8.1 rules 3 and 4; 8.2 rules 2 and 4 and its table; 9.3 rule 7; 14.4 | Grammar, the `guard` field, the scope rule for a lambda's own name, function parameters, guards, and the static rules |
| `02-evaluation.md` | 8.4 item 10 | Amended by `17` 3.4 |
| `02-evaluation.md` | 10.2, 11.1 | Frames are the reference path; the fixed-place row and the three facts hold only with no recursive lambda and no `Fn` parameter |
| `02-evaluation.md` | 11.2, 11.3; the `match` cost row | A call of an `Fn` parameter; a lambda argument to a user lambda; a guard charges as it runs |
| `03-host.md` | 3.6.1 | The largest depth is 400 (D58). Advise a deadline when steps are off |
| `03-host.md` | 3.9.1 "The stack a binding needs" | Depth limits 200 and 400; the deepest cases include calls through `map` and `flatMap`, with 1.5 times headroom |
| `04-library.md` and `suite/library.json` | Core list; counts | `walk`. One entry. Cost rows as in 5 |
| `05-types.md` | 2.5, 3.9, 11.2, 13.2, 13.9, 16.1, 20.6, 20.7, 22.1, 22.3 to 22.5, a new 20.8 | Recursive types, printing by name, guard rules, a typed lambda checked once, function parameters, the new code, the static rules, the bound of 4,096 pairs |
| `spec/packages/f01-host-types.md` | 3, 6.2 | A recursive `$ref` and a Go struct that contains itself |
| `spec/packages/f04-conversion.md` | 3.1 items 1 and 3; 3.2 the "declared later or its own" row and item 1; 5.4 item 4 | Recursive named types, the guard rule, and a walk that follows the data |
| `spec/packages/f08-errors.md` | 3.1 item 2; 3.5 | The template of `recursion-needs-types`, and those of 7.2 |
| `15-build-rulings.md` | R112; R20 and R76 | R112 superseded by 4.1 rule 5. R20 and R76 replaced by D58 |
| `suite/` | `core-check/`, `core-run/`, a package directory | Cases for each rule of 3 to 8: self-recursion to the depth limit, the scope break of 3.1, coinductive Fits, the bound of 4,096, a `$ref` cycle, guards and coverage, every rule of 7.1, the `apply` example of 8 |
| `fuzz/` | The generators | Recursive lambdas, `Fn` parameters, guards and `walk` |
| `go/hosttypes.go` | `TypeOf` | A struct that contains itself |

The counts that change: library entries (78 to 79, and the default group 59
to 60, as `00` 3 counts them), check codes (`recursion-needs-types`: Core 27
to 28, the catalog 29 to 30 before F16's `bad-export`), check templates (one
for `recursion-needs-types`, three by 7.2 if taken), and the static codes of
`05-types.md` 16.1 (11 to 12, and `arg-mismatch` joins the list if 7.1 rule
3's recommendation is taken). No reserved word, no run code, no node kind and
no `parse` code is added: `if` is already reserved, and a guard written in a
program without this package is a `parse` error today (ran:
`unexpected-token` on `if`).

**Package.** The five additions form one follow-on package, F17, built in
the four steps of section 1. The scope rule of 3.1 rule 1 is in Core, timed
with slice D1 (3.1).

## 12. Cost to each implementation

| Part | Go | TypeScript | Rust |
| --- | --- | --- | --- |
| Frames for every call | The closure compiler gives each call a frame instead of fixed slots | The same | The same. Borrowed host data is unaffected: a frame holds values the run owns or lends |
| Native stack at depth 400 through library calls | Goroutine stacks grow (M) | The risk (section 9): fewer JavaScript frames for each Risor call | A thread with a set stack size (section 9) |
| Checker | Coinductive Fits, Equal and Join with a pair set; a typed lambda checked once; function parameters; guards | The same | The same |
| Library | `walk` with an explicit stack | The same | The same |

## 13. What I did not verify

- The stack figures of section 9 are the owner's measurements of 2026-11-19,
  carried here; I did not run them. Safari and Firefox were not measured.
- That the error sites of `05-types.md` 18 need nothing more than "recorded
  once" for a lambda checked against its annotations.
- The number 4,096. It is a guess sized to the type size limit of 100,000,
  not a measurement. The suite's cases should probe it.
- That `any` from a repeated pair in Join is rare. It is inference from the
  fact that a recursive lambda's result is annotated.
- That 7.1's rules leave no way to call a function with the wrong count, or
  to call a value, in an unchecked run. Inference from the rules; the
  differential fuzzer is the test.
- That a program with no recursive lambda still ends by construction (7.5).
  Inference.
- TypeScript's behaviour for `const f = x => f(x)` and the need for a return
  annotation on a recursive function. From memory; TypeScript was not run.
