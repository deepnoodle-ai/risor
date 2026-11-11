# F14. Host facts

This chapter is normative. It specifies follow-on package F14. A host sometimes
knows more about its variables than their types say: of two variables, each
declared `T | null`, at least one is not null in every run. A **host fact**
states it. The checker uses it through one fact, "is null", and one effect.

The package adds to Core and to packages F1, F4 and F9:

1. The host fact, its one kind `someNotNull`, its meaning as part of the
   precondition, and its declaration errors (2).
2. The call that declares one in each binding (3).
3. The fact null(p): which tests give it, and the facts in the right operand of
   `??` (4).
4. Its one effect: a member is not null where every other member of its host
   fact is null (5).
5. What a program that gains a host fact may newly report (6, 7), and the
   soundness of the rules (8).
6. The key `facts` of an environment's description (9).
7. The case field `facts`, and a fixture of the fixture test of F9 (10).

It adds no syntax, no node kind, no reserved word, no value, no library entry,
no run code, no check code, no `declare` code, no message template, no site, no
check class and no cost row. It reports the `declare` code `bad-name` at a new
place (2.5).

The chapters of Core are cited by file and section: `05-types.md` 10.5 is
section 10.5 of `spec/core/05-types.md`. F1 is
`spec/packages/f01-host-types.md`, F4 `spec/packages/f04-conversion.md`, F5
`spec/packages/f05-unions.md` and F9 `spec/packages/f09-testing.md`. Section
numbers without a file are of this chapter.

## 1. Conventions

### 1.1 Terms

| Term | Meaning |
| --- | --- |
| **Host fact** | A statement an environment declares about the values of its variables, beyond their types (2) |
| **Member** | A variable that a host fact names |
| **Member path** | An `Ident` that resolves (`05-types.md` 5.3) to an environment variable that is a member of some host fact of the environment (4.1) |
| **null(p)** | The fact "is null" about a member path p (4) |
| **{p}** | Core's fact "not null" (`05-types.md` 10.4) |
| **N(e)** | The null facts that hold where `e` gave null, for the left operand e of `??` (4.4) |
| **Path, facts in force, T(c), F(c)** | `05-types.md` 10.3, 6.2 and 10.5, with the facts of F4 6.4 and F5 5.2 |

### 1.2 Requirements

1. An implementation that offers F14 offers every section of this chapter but 3,
   whose sections bind the binding of their language.
2. Packages are offered in the order of `03-host.md` 3.11 item 2, with F14 after
   F13 and before F12. So an implementation that offers F14 has the type
   checker, F1, F4, F5 and F9, and its cases may use their forms.
3. A compile with the static rules alone (`03-host.md` 3.2.1 item 4) builds the
   host facts of the environment and reports their errors (2.5). It applies no
   rule of 4 to 7: the static rules read no type and no fact.

### 1.3 Version

F14 is part of version `3.0`. It adds no version number.

### 1.4 What this package changes in earlier rules

| Earlier rule | With this package |
| --- | --- |
| `03-host.md` 3.3.1, the table of declarations | Gains the host fact (2.1) |
| `03-host.md` 3.3.2 | A child keeps its parent's host facts, less those that name a variable it replaces (2.6) |
| `03-host.md` 3.3.4 | `bad-name` for an error in a host fact (2.5) |
| `03-host.md` 3.10.3 | Gains the field `facts` (10.1) |
| `05-types.md` 8.2, the row `??` | The right operand is checked under N of the left (4.4). The type of `??` is unchanged |
| `05-types.md` 10.4 | Gains the fact null(p) (4.1) |
| `05-types.md` 10.5 | Gains the rows of 4.2 and the place of 4.4 |
| `05-types.md` 11.2 | The arm of `null` alone gives null(p) (4.3) |
| `05-types.md` 5.3, 10.4 (the row Effect) | A member path may have nonnull of its type (5) |
| `05-types.md` 18.4 | The precondition gains the host facts (2.3) |
| F1 5.3, 5.4 | The description gains the key `facts` (9) |
| F4 6.4 | `p is T`, T the type `null`, also gives null(p) (4.2) |
| F9 2.1, 2.3 | A host fact of a case is a fixture (10.2) |

No other rule changes.

## 2. The host fact

### 2.1 The declaration

This row is added to the table of `03-host.md` 3.3.1.

| Declaration | States | Portable form |
| --- | --- | --- |
| Host fact | A kind, and the variables it names | The kind's name, and a list of variable names |

1. 3.0 has one kind, `someNotNull` (2.2).
2. An environment may declare any number of host facts. Their order does not
   matter.
3. A member is named, not typed: a host fact adds no variable and changes no
   variable's type.
4. A host fact may name a variable of the same environment or of a parent (2.6).
   The variables an environment declares and its host facts are given together;
   a binding does not require a host fact to come after the variables it names.

### 2.2 The kind `someNotNull`

`someNotNull` names one or more variables of the environment, and states: **in
every run, the value of at least one of them is not null.**

1. Members are variables, not longer paths: `report`, never `input.report`.
2. A member's declared type may lack null. The host fact then holds in every run
   that keeps the precondition, and states nothing new about that variable.
3. A host fact of one member states that this variable is not null.
4. The list is a set: the order of its members does not change the meaning. Two
   host facts with the same members are one statement.

### 2.3 Its meaning

A host fact is part of the precondition of `05-types.md` 18.4, as a declared
type is. The row "Precondition" of that table reads:

| Part | Statement |
| --- | --- |
| Precondition | Every variable's value conforms to its declared type; **for every host fact `someNotNull`, the value of at least one of its members is not null;** every host function and method returns a value that conforms to its result type; and a host function or method whose signature has no `!` returns no error. |

The host promises it for every run, as it promises the types of its variables.

### 2.4 A run

1. A run does not read host facts. Nothing tests one at a run, and a program's
   value, usage, code, span and message do not depend on them.
2. Where a host breaks a host fact, the run goes on. A read that meets null
   where the checker relied on the fact fails with the code of what it found, as
   for a value that breaks its declared type (`03-host.md` 3.4.8 item 2):
   `null-operand` for a field read or a method call on null.
3. The class of such a failure is given by `05-types.md` 18.5. In a checked
   program it is `environment`, unless the failing node is at a `dynamic` site
   (a member of type `any`). In a program compiled without a check it is the
   class of the column "not checked".

### 2.5 Errors

These are errors of phase `declare`, reported when the environment is built,
whether the program is then compiled with the check or with the static rules
alone. They need the names of the environment's variables and no type, so every
implementation that offers F14 reports them.

| Situation | Code |
| --- | --- |
| A host fact with no member | `bad-name` |
| A member that names no variable of the environment: a misspelled name, a host function, a namespace, a library entry, an opaque type, a named type, text that is not a `Name` of `01-language.md`, a reserved word, the empty string | `bad-name` |
| A member that names a variable an earlier member of the same host fact names | `bad-name` |

1. `bad-name` is the code of a list of names that names nothing, as for a
   removed entry (`03-host.md` 3.3.4). The message says which of the three is
   wrong.
2. The members of a host fact are tested in order. Each member that names no
   variable is one error. Each member equal to an earlier member that names a
   variable is one error. An implementation reports every error it finds, in no
   specified order (`03-host.md` 3.3.4 item 4).
3. Names are compared code point by code point. `Report` is not `report`.
4. A host fact with an error is not declared.
5. Listing a host fact twice, or two host facts that share members, is no error.

### 2.6 Child environments

These are binding rules (`03-host.md` 3.3.2). The suite's cases have one level
of child, the case's environment under a base environment that declares no host
fact (`03-host.md` 3.10.9), so no case tests them. Each binding tests them with
tests of its own.

1. A child keeps every host fact of its parent.
2. A host fact of the parent that names a variable the child replaces is dropped
   in the child, whole. It is not narrowed to its other members.
3. A child may declare host facts of its own. They may name the child's
   variables and the parent's variables that the child does not replace. They
   are tested against those variables (2.5).
4. A host fact the child declares that names a variable the child replaces names
   the child's variable, and is kept.
5. The parent is not changed.

## 3. The bindings (binding rules)

Each binding declares a host fact the way it declares a variable.

### 3.1 Go

```go
func SomeNotNull(names ...string) Decl

env, err := risor.NewEnv(
    risor.VarRecord("{report: {ok: bool} | null, approve: {ok: bool} | null}"),
    risor.SomeNotNull("report", "approve"))
child, err := env.With(risor.SomeNotNull("approve"))
```

1. `SomeNotNull` is a `Decl` like `Var`. `NewEnv` and `With` take it in any
   position among the others.
2. An error of 2.5 makes `NewEnv` or `With` return an `*Error` of phase
   `declare` with the code `bad-name`.

### 3.2 TypeScript

```ts
export interface EnvOptions {
  // the fields of the binding, and:
  someNotNull?: string[][];
}

const env = createEnv({
  varRecord: "{report: {ok: bool} | null, approve: {ok: bool} | null}",
  someNotNull: [["report", "approve"]],
});
const child = env.with({ someNotNull: [["approve"]] });
```

1. `env.with(options)` adds the child's host facts to those it keeps (2.6).
2. An error of 2.5 makes `createEnv` or `with` throw a `RisorError` of phase
   `declare` with the code `bad-name`.

### 3.3 Rust

```rust
let env = Env::builder()
    .var_record("{report: {ok: bool} | null, approve: {ok: bool} | null}")
    .some_not_null(&["report", "approve"])
    .build()?;
let child = env.with().some_not_null(&["approve"]).build()?;
```

1. `some_not_null` takes a slice of names and returns the builder. It does not
   fail: the names are tested by `build`, when every variable is known.
2. An error of 2.5 makes `build` return an `Error` of phase `declare` with the
   code `bad-name`.

## 4. The null fact

### 4.1 The fact

`05-types.md` 10.4 gains one kind of fact.

| Fact | Written | Says | Kept for |
| --- | --- | --- | --- |
| is null | null(p) | The value of p is null | A member path p only |

1. **Member paths.** p is a member path when it is an `Ident`, with no step,
   that resolves to an environment variable that is a member of some host fact
   of the environment. A path with a step (`report.verdict`, `report?.x`), a
   `const` name, a parameter, a name bound by a pattern and the name of `catch
   (err)` are never member paths, whatever they are bound to.
2. **Lifetime.** As every fact, null(p) is about a binding, never reaches a
   binding that shadows the name, and is never ended (`05-types.md` 10.4).
3. **Same.** Two null facts are the same when they are about the same path.
   null(p) and {p} are not the same fact. This is what ∩ compares in
   `05-types.md` 10.5.
4. **It reports nothing.** A null fact is never a diagnostic, and changes the
   type of no expression but by 5. Where null(p) holds, an expression that is p
   keeps the type it has without it.

### 4.2 True and false facts

These rows join the table of `05-types.md` 10.5. p is a member path, and `null`
is a `Null` node.

| c | T(c) | F(c) |
| --- | --- | --- |
| `p == null`, `null == p` | null(p) | {p}, as in Core |
| `p != null`, `null != p` | {p}, as in Core | null(p) |
| `p is T`, where T resolves to exactly the type `null` (F4) | null(p), beside the facts of F4 6.4 | The facts of F4 6.4 |

1. The rows of `!d`, `d && e` and `d || e` are unchanged, and carry null facts
   as they carry {p}: `!` swaps T and F, `&&` unites the true facts and keeps
   the false facts both sides give, `||` keeps the true facts both sides give
   and unites the false facts.
2. A bare path p gives no null fact. Where p was falsy it may be null, `false`,
   zero or empty (`05-types.md` 10.5).
3. A test of a path that is not a member path gives what Core and the packages
   give, and no null fact.
4. The `if` chain is checked as nested `? :` (`05-types.md` 11.1): the false
   facts of every earlier test, null facts among them, hold in each later test
   and branch.

### 4.3 `match`

When the subject of a `match` is a member path p, the result of an arm whose
literals, less those `05-types.md` 11.2 ignores, are the one literal `null` is
checked under null(p).

1. An arm that lists `null` and another literal gives its result no null fact:
   the subject may be the other literal.
2. Core's rule stands: {p} holds in every arm after an arm that lists `null`.
3. The arm `_` gives no null fact.

### 4.4 `??`

In `a ?? b`, b is checked under the facts in force where the `??` stands and
N(a). b runs only where a gave null.

| a | N(a) |
| --- | --- |
| A member path p | null(p) |
| `a1 ?? a2` | N(a1) ∪ N(a2) |
| Anything else | None |

1. `??` reads from the left (`01-language.md` 3.3), so `a ?? b ?? c` is `(a ??
   b) ?? c`: b is checked under N(a), and c under N(a) ∪ N(b).
2. In `a ?? (b ?? c)`, the operand `b ?? c` is checked under N(a), so c is
   checked under N(a) ∪ N(b) as well.
3. Parentheses are not a node: N(`(report)`) is null(`report`).
4. N of a chain with a step, as `report?.verdict`, of a conditional, of a call
   and of every other form is empty.
5. `a ?? b` as a tested expression gives no fact, as in Core. a itself is
   checked under the facts in force, with nothing added.

The second table of `05-types.md` 10.5 gains a row.

| Where | Under |
| --- | --- |
| b in `a ?? b` | N(a) |

### 4.5 Beside the other facts

| Test or form | Facts of Core and the packages | A null fact |
| --- | --- | --- |
| A bare path p | T: {p} | None |
| `p == null`, `p != null` | {p} on one side | null(p) on the other side, for a member path (4.2) |
| The arm of `null` alone in a `match` on p | None | null(p) in its result (4.3) |
| The arms after an arm that lists `null` | {p} | None |
| `p is T`, T exactly `null` (F4) | T: {p: `null`}. F: {p: not `null`} and {p} | null(p) in T |
| `p is T`, T with `null` and another alternative | T: {p: T}. F: {p: not T} and {p} | None: where it held, p may be another value of T |
| `p is T`, T without `null` | T: {p: T} and {p}. F: {p: not T} | None: where it failed, p may be null or another value |
| A test of a field (F5 5.2), a `match` on a field (F5 5.3) | {p: H}, {p}, {p: not A} | None: the tested path has a step |
| The predicate of `filter` and `find` (`05-types.md` 13.8, F4 6.5, F5 5.7) | Facts about the parameter | None: a parameter is never a member path |
| `try` | None (`05-types.md` 14.1) | None. Facts in force outside hold in both parts |
| The right operand of `??` | None in Core | N of the left operand (4.4) |

### 4.6 Bindings and lambdas

1. **A `const` bound to a member** is not a member path: in `const r = report; r
   == null ? approve.verdict : r.verdict`, the test of r gives no null fact, and
   `approve.verdict` is `maybe-null`. The `const` name has its value's type as
   narrowed where the `const` stands (`05-types.md` 10.3), which 5 may have made
   non-null: `const r = report ?? approve; r.verdict` checks.
2. **A binding that shadows a member** (a `const`, a parameter, a name in a
   pattern, the name of `catch (err)`) is not a member: an `Ident` that resolves
   to it is no member path, and a test of it gives no null fact.
3. **A null fact about the variable still holds** inside a scope where its name
   is shadowed, since it is about the variable's binding: in `report == null ?
   xs.map(report => approve.verdict) : []`, `approve` is not null.
4. **A lambda's body** is checked under the facts in force where the lambda is
   written (`05-types.md` 6.2, 13.9). So a lambda written in a branch sees the
   branch's null facts, and a `const`-bound lambda written outside the branch
   does not, wherever it is called.
5. **`filter` and `find`** narrow by facts about their parameter (`05-types.md`
   13.8), and a null fact is never about a parameter. They narrow nothing more.

## 5. The effect

### 5.1 The rule

Let p be a member path. **Where an expression is p, and some host fact that has
p as a member has every other member q under null(q) in the facts in force
there, the expression has nonnull of its type** (`05-types.md` 4.5).

1. **Its type** is the type `05-types.md` 5.3 gives the `Ident`, narrowed by the
   facts in force of Core, F4 and F5. The effect is applied after them.
2. **An expression that is p** is p wherever it stands, as for the row Effect of
   `05-types.md` 10.4: an operand, the primary of a chain, the subject of a
   `match`, the value bound by a pattern, an argument, a rendered expression.
3. **A host fact of one member** has no other member, so the effect applies at
   every expression that is that member.
4. **A member whose declared type lacks null** keeps its type. **A member of
   type `null`** has `never` where the effect applies, as under {p}.
5. **The effect is not a fact.** It adds nothing to T(c) or F(c), takes no part
   in ∩, and gives nothing to `filter` and `find`. A tested expression that is p
   gives {p} as Core says.

### 5.2 Several host facts

A member of several host facts has nonnull of its type where any one of them has
every other member under a null fact. With the host facts `someNotNull(a, b)`
and `someNotNull(a, c)`, `a ?? b` and `a ?? c` lack null, and `b ?? c` does not:
neither host fact has all its other members null there.

### 5.3 The types it gives

`05-types.md` 8.2 gives `a ?? b` the type Join(nonnull(A), B). With 4.4 and 5.1,
B is the type of b under N(a).

| Written | Environment | Type |
| --- | --- | --- |
| `report ?? approve` | E | `{verdict: string}` |
| `(report ?? approve).verdict` | E | `string` |
| `(report ?? approve)?.verdict` | E | `string` |
| `const {verdict} = report ?? approve; verdict` | E | `string` |
| `report == null ? approve.verdict : report.verdict` | E | `string` |
| `if (report != null) { report.verdict } else { approve.verdict }` | E | `string` |
| `match report { null => approve.verdict, _ => report.verdict }` | E | `string` |
| `report == null && approve.verdict == "ok"` | E | `bool` |
| `report is null ? approve.verdict : report.verdict` | E | `string` |
| `report == null ? xs.map(i => approve.verdict) : []` | E | `string[]` |
| `report ?? approve.verdict` | E | `string \| {verdict: string}` |
| `report == null ? report : approve` | E | `{verdict: string} \| null` |
| `approve.verdict` | E | `maybe-null` on `approve` |
| `report ?? report.verdict` | E | `maybe-null` on the second `report` |
| `report ? report.verdict : approve.verdict` | E | `maybe-null` on `approve`: a falsy path gives no null fact |
| `(report ?? third).verdict` | E | `maybe-null` on `report ?? third` |
| `const r = report; r == null ? approve.verdict : r.verdict` | E | `maybe-null` on `approve` |
| `(a ?? b ?? c).v` | E3 | `int` |
| `(a ?? b).v` | E3 | `maybe-null` on `a ?? b` |
| `a == null && b == null ? c.v : 0` | E3 | `int` |

E declares `{report: {verdict: string} | null, approve: {verdict: string} |
null, third: {verdict: string} | null, xs: int[]}` and the host fact
`someNotNull(report, approve)`. E3 declares `{a: {v: int} | null, b: {v: int}
| null, c: {v: int} | null}` and `someNotNull(a, b, c)`.

## 6. Sites, classes and diagnostics

1. F14 records no site, adds no check class and reports no check code of its
   own. A null fact is never a diagnostic.
2. Where 5.1 applies, a diagnostic that reads null in a type is not reported for
   null: `maybe-null` on that expression, or on an operand, receiver, argument
   or value bound that it is or that flows from it.
3. Where 5.1 applies, a rule that reports a type that is too narrow may report
   where it did not: 7.2 lists them.
4. The class of a run failure is given by `05-types.md` 18.5 and `03-host.md`
   3.7.6, unchanged (2.4).

## 7. It only adds

### 7.1 An environment with no host fact

With no host fact, no path is a member path. No null fact is given, N of every
operand is empty, and 5.1 never applies. Every program has the type, sites,
diagnostics, value, usage, code, span, message and class it has without this
package. So no earlier case changes: none declares a host fact, since the field
`facts` (10.1) is new.

### 7.2 A program that gains a host fact

Let a program check in an environment E, and let E' be E with one or more host
facts added. Then in E':

1. Every node has the type it has in E, or a type that fits it: nonnull of it at
   the expressions of 5.1, and what Join, Truthy, Falsy and the other rules give
   from that.
2. The sites are those of E, less any `value` site of an `as` (F4 6.3) whose
   subject now surely fits its type.
3. **No diagnostic appears, except from these rules,** each of which reports a
   type that is too narrow. Each reports here what it reports for the same
   narrowing written as a test, `p != null && ...`.

| Rule | May newly report | Example in E of 5.3, with `other: int \| null` |
| --- | --- | --- |
| `05-types.md` 10.2, constant conditions | `always-truthy`, where null was the only falsy part of a tested type. `always-falsy` only where a member of type `null` became `never` inside a tested type, as `[z]` is then `never[]` | `report == null ? (approve ? 1 : 2) : 0` |
| `05-types.md` 8.2 and 8.4, `never-equal` | Where the two types shared only null | `report == null ? approve == other : false` |
| F4 6.3, `x is T` | `always-truthy` or `always-falsy` (class `constant-condition`) | `report == null ? approve is null : false` |
| F4 6.3, `x as T` | `never-conforms` | `report == null ? approve as null : null` |

The two codes of `05-types.md` 10.2 and the two of F4 on `is` are reported only
when the environment rejects `constant-condition`, as always. `never-equal` and
`never-conforms` have no class. A member of type `null` is `never` where 5.1
applies (5.1 item 4), and every rule treats it as it treats `never` under Core's
{p}.

### 7.3 Every rule that reads a narrowed type

| Rule | No host fact (7.1) | Gaining a host fact (7.2) |
| --- | --- | --- |
| `05-types.md` 9.1, 9.2, 8.3, 8.4, 12.2, 15.1: `maybe-null` | Unchanged | Fewer |
| `05-types.md` 9.1 step 4, 8.3: `kind` | Unchanged | Unchanged: null is removed before the kind test |
| `05-types.md` 10.2, constant conditions | Unchanged | May report (7.2) |
| `05-types.md` 10.2, a needless `x != null`, `x ?? d`, `x?.f` | Unchanged | Never reported, as in Core |
| `05-types.md` 8.2, 8.4, `never-equal` on `==`, `!=`, `in` | Unchanged | May report (7.2) |
| `05-types.md` 11.2, `never-equal` on a `match` literal | Unchanged | Unchanged: the literal is not null, and overlap through another kind does not depend on null. A subject of type `never` reports none |
| `05-types.md` 11.2, coverage | Unchanged | No new `non-exhaustive`: a type without null has fewer values to cover |
| `05-types.md` 11.2, {p} after the arm of `null` | Unchanged | Unchanged |
| `05-types.md` 13.5 to 13.8, calls, bounds and `filter` | Unchanged | Types that fit: an argument that fitted still fits, a bound that held still holds |
| `05-types.md` 15, `result-mismatch` | Unchanged | Fewer |
| `05-types.md` 18.3, sites | Unchanged | 7.2 item 2 |
| `05-types.md` 20, limits | Unchanged | A type is no larger |
| F4 6.3, `is` and `as` | Unchanged | May report (7.2); may drop a `value` site |
| F4 6.4, facts of `is` | Unchanged | Facts of the same tests; `without` and conforms on a type that fits |
| F5 5.2 to 5.4, narrowing by a field | Unchanged | Unchanged: M, the map alternative, does not depend on null |
| F5 5.3, coverage of a `match` on a field | Unchanged | Unchanged |

### 7.4 Earlier cases

1. No earlier case is contradicted: no case declares a host fact (7.1).
2. No `until-f14-` file exists, and this package removes none and writes none.

## 8. Soundness

**Claim.** Promises 1 to 4 of `05-types.md` 18.4 hold with the precondition of
2.3.

1. **A null fact holds.** Where null(p) is in force at a node, p is null each
   time the node is evaluated. T(`p == null`) holds where the test was true;
   F(`p != null`) where it was false; T(`p is null`) where p conformed to
   `null`; the arm of `null` alone where the subject equalled null. `!`, `&&`
   and `||` combine them as they combine {p}, which is sound for every fact that
   is a statement about values, since values do not change. b in `a ?? b` runs
   only where a gave null, and `a1 ?? a2` gives null only where both a1 and a2
   did, so N is sound. A lambda's body runs only inside the node where it is
   written, so the facts in force there hold when it runs. A fact is about a
   binding, so shadowing does not mislead it.
2. **The effect holds.** Where 5.1 applies, every other member of the host fact
   is null by item 1, and by the precondition one member is not null, so p is
   not null. Its value conforms to its type as narrowed by sound facts, and so
   to nonnull of it.
3. **Promise 1.** Every type the checker gives is a sound type of the value
   (items 1 and 2), so Core's argument for promise 1 holds unchanged.
4. **Promises 2 and 3.** No site and no class is added. A `value` site is
   dropped only where an `as` cannot fail under the precondition (7.2 item 2).
5. **Promise 4.** By items 1 to 3.
6. **A host that breaks a host fact** breaks the precondition. A read the effect
   allowed may then meet null, and fails as 2.4 says, with class `environment`.

## 9. Descriptions, analysis and tools

### 9.1 The description

The description of an environment (F1 5.3) gains one key.

| Key | Value |
| --- | --- |
| `facts` | An array of objects, one for each distinct host fact of the environment. Each is `{"someNotNull": [names]}`, its members in code point order. The objects are ordered by their lists of members, compared name by name in code point order, a list that is a prefix of another first. Present only when the environment has at least one host fact |

1. A child's description holds the host facts it keeps (2.6) and its own.
2. Two host facts with the same members are described once (2.2 item 4).
3. A description with no host fact has no key `facts`, so every description of
   an environment with none is as F1 and the packages before F14 give it.
4. Example: an environment of 5.3's E has, beside F1's keys,
   `"facts":[{"someNotNull":["approve","report"]}]`.

### 9.2 An environment from a description

An environment built from a description (F1 5.4) declares each entry of `facts`
as a host fact. An entry that names no variable of the description, an empty
list and a name listed twice are `bad-name` (2.5). An entry that is not an
object of the form of 9.1 is refused by the binding with an error of its own, as
F1 5.4 item 2 says. The round trip of F1 5.4 item 3 holds.

### 9.3 Analysis and tools

The analysis of F2 does not change. The results of F12 (completion and hover) do
not change, except that a type they show may lack null where 5.1 applies.

## 10. Additions to the case format

Each addition is optional, and its absence leaves every existing case file valid
with its meaning unchanged.

### 10.1 The case field `facts`

This row is added to the table of `03-host.md` 3.10.3.

| Field | Holds | When absent |
| --- | --- | --- |
| `facts` | A list of host facts. Each is an object with exactly one key, the kind `someNotNull`, whose value is a list of variable names | No host fact is declared |

1. **Beside the variables.** `facts` is a declaration of the case's environment,
   and joins the list of `03-host.md` 3.10.3 item 3, after `env`. Its members
   name variables of the case's environment: those of `env`, and those of the
   base environment that the case does not replace.
2. `facts` may stand in the header. A case's `facts` replaces the header's whole
   (`03-host.md` 3.10.2 item 3). A case that replaces the header's `env` and not
   its `facts` keeps the header's host facts, which must then name variables of
   the new `env`.
3. `"facts": []` declares no host fact. A case uses it to remove the header's.
4. The errors of 2.5 are `declare` failures of the case, not invalid files: a
   list with no name, a member that names no variable, a member named twice.
5. Wherever `env` may stand, `facts` may stand. A conform case of F4 9.2 and a
   runner case of F9 3.1 have no environment of their own: `facts` there makes
   the file invalid, as any field they do not list does.

These make the file invalid: `facts` that is not an array; an entry that is not
an object, or that has a key other than `someNotNull`, or no key; a value of
`someNotNull` that is not an array of strings.

### 10.2 The fixture test of F9

F9 2.1 gains a fixture.

| Fixture | Its place | Tested against |
| --- | --- | --- |
| The host fact at index i of the case's `facts` | `["facts", i]` | 2.2: at least one member's value in `vars` is not null |

1. A host fact is tested only when `vars` supplies every one of its members. A
   host fact with a member the case does not supply is not tested (F9 2.1 item
   4).
2. It fails when every member's value in `vars` is null.
3. **Order.** F9 2.3 reads: the values of `vars`; then the host facts of
   `facts`, in order; then the stubs of `functions`; then those of `methods`.
4. **A failure** has the verdict `invalid` (F9 2.4), with the fixture `["facts",
   i]` and the message `at least one of ` followed by the members in the order
   the case lists them, separated by `, `, followed by ` must not be null`: `at
   least one of report, approve must not be null`.
5. `"precondition": false` turns it off with every other fixture (F9 2.5).

### 10.3 The runner

1. A runner of an implementation that offers F14 runs `suite/facts/`. A file
   whose name ends `-check.json` is run checked only (`03-host.md` 3.11 item 7).
   Every other file is run as `suite/core-run/` is: each case checked and with
   the static rules alone, both compared, `class` as `03-host.md` 3.10.7 item 8
   says.
2. A runner of an implementation that does not offer F14 does not run
   `suite/facts/`. For it the field `facts` is unknown (`03-host.md` 3.10.10).
3. In `suite/facts/` the static codes are the eleven of Core, and
   `untagged-union` and `check-limit` at a program type of F4 (F4 6.1 item 4).
   No case of the directory expects either of the two.
4. The result field `description` of F1 7.2 compares the description of 9.1.

### 10.4 The `outcome` command

The line of the `outcome` command (`suite/README.md`) changes so:

| Case | Line |
| --- | --- |
| A case with `facts`, whose environment was built | The field `description` of F1 7.4, after `name` |

A case with `facts` is a case where the field is present, in the case or in
the header, even as an empty list.

### 10.5 The JSON Schema of the additions

These are added to the schema of `03-host.md` 3.10.11: the property below in
`$defs/fields`, and `"facts"` in both lists of `propertyNames`.

```json
{
  "fields": {"properties": {
    "facts": {
      "type": "array",
      "items": {
        "type": "object",
        "required": ["someNotNull"],
        "properties": {
          "someNotNull": {"type": "array", "items": {"type": "string"}}},
        "additionalProperties": false}}}}
}
```

### 10.6 Example

```json
{
  "env": "{report: {verdict: string} | null, approve: {verdict: string} | null}",
  "facts": [{"someNotNull": ["report", "approve"]}],
  "cases": [
    {"name": "the chain reads a field", "source": "(report ?? approve).verdict",
     "expect": {"type": "string", "sites": []}},
    {"name": "the same with no host fact", "source": "(report ?? approve).verdict",
     "facts": [],
     "expect": {"phase": "check",
                "diagnostics": [{"code": "maybe-null", "span": [1, 2, 1, 19]}]}},
    {"name": "a member that names nothing", "source": "1",
     "facts": [{"someNotNull": ["report", "reprt"]}],
     "expect": {"phase": "declare", "diagnostics": [{"code": "bad-name"}]}},
    {"name": "a host that breaks the host fact", "source": "(report ?? approve).verdict",
     "vars": {"report": null, "approve": null}, "precondition": false,
     "expect": {"phase": "run",
                "diagnostics": [{"code": "null-operand", "class": "environment",
                                 "span": [1, 1, 1, 28]}],
                "steps": 0, "allocated": 0}}
  ]
}
```

## 11. Costs and what is not added

1. A host fact charges no step and no unit. A program's counts do not depend on
   the host facts of its environment.
2. No syntax, no node kind, no reserved word, no value, no library entry, no run
   code, no check code, no `declare` code, no parse code, no message template,
   no site, no check class and no cost row is added.
3. Not in this package: a member that is a longer path; other kinds of host
   fact; a null fact from a test of a path with a step, of a `const` bound to a
   member, or of an expression that is not a path; a call that tests a host fact
   against values (a host tests it with its own code); `match` by conditions.

## 12. What F14 adds

The rows the pointer tables of Core and of F1 take for this package.

| Table | Row |
| --- | --- |
| `03-host.md` 3.11 | F14, host facts: the declaration of a host fact (a row of 3.3.1), its errors, child environments. The case field `facts`, and the host fact as a fixture of F9. `spec/packages/f14-host-facts.md` |
| `05-types.md` 23 | F14, host facts: the fact null(p), its true and false facts, N of the left operand of `??`, and the effect on a member path; the precondition with host facts. `spec/packages/f14-host-facts.md` 4 to 8 |
| F1 5.3 item 4 | F14 adds the key `facts` |
| `03-host.md` 3.11 item 2 | The order of packages: F13, F14, F12 |

## 13. Cases

The cases are in `suite/facts/`.

| File | Run | Cases | Holds |
| --- | --- | --- | --- |
| `facts-declare.json` | Checked and static | 21 | Each error of 2.5, and valid host facts beside them |
| `facts-declare-header.json` | Checked and static | 4 | `facts` in a header, a case that replaces `env`, `"facts": []` (10.1) |
| `facts-run.json` | Checked and static | 19 | Runs with values, steps and units; runs of a host that breaks a host fact, with class `environment` (2.4) |
| `facts-check.json` | Checked only | 97 | 4 to 7: each test, `match`, `??` and N, F4 and F5 beside the null fact, bindings and lambdas, several host facts, members of every kind, what a gained host fact newly reports; and the same texts with no host fact |
| `facts-examples-check.json` | Checked only | 20 | Every row of the table of 5.3 |
| `facts-description-check.json` | Checked only | 7 | The key `facts` of 9.1 |
| `facts-fixtures.json` | Runner cases of F9 | 8 | The fixture of 10.2 |
