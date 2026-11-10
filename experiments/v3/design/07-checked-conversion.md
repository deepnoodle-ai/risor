# Risor v3 design: checked conversion of untyped data

Status: plan accepted by the owner. No code was written and nothing was
measured. The owner settled the seven decisions of section 9 (D39 to D45 in
`00-overview.md`) as recommended, and `02` to `06` are revised to match. Those
revisions found errors in the first draft of this plan, and they are corrected
here: section 10 lists them. Decision numbers such as D29 and R5 refer to the
overview.

`T` and `U` in this document stand for any type. They are not program text: a
one-letter upper-case name is a type variable of a library signature and cannot
be written after `as` or `is` (`03:2.2`).

**How to read this.** Section 2 is the proposal, written as rules so that it can
move into the specification. A paragraph headed *Why* is not a rule. Sections 3
to 10 are not rules.

**Evidence.** **(R)** read in a sibling document at the place cited. **(I)**
inferred. **(M)** from memory of another language or library, not checked
against its documentation for this draft. `02:` to `06:` are the sibling
documents.

## 1. Summary

1. **The problem.** A program meets data whose type nobody declared: the result
   of `json.parse`, an HTTP body, the output of a model, an `any` field. Today
   it has two choices. It can use the data as it is, and every use is a
   `dynamic` site that may fail at run time (`03:2.12`) (R). Or the host can
   reject `dynamic`, and then the data cannot be used at all.
2. **The proposal.** Two operators that test a value against a type at run time.
   `x as T` gives `x` back with type `T`, or fails. `x is T` gives a `bool`, and
   narrows `x` to `T` where the test passed.
3. **It is a test, not a conversion.** The value does not change. Nothing is
   coerced, filled in or removed. The word "conversion" in the title is about
   the type: `any` goes in and `T` comes out.
4. **The rule already exists.** `03:2.14` defines when a value conforms to a
   type, because the checker's promise rests on it (R). This plan gives that
   rule to the program, and to the host as a function.
5. **Types come from three places:** type text written in place, a name the
   host declared (from type text or from a JSON Schema), and later a `type`
   line in a script.
6. **It checks types, not schemas.** A type from a JSON Schema carries the
   shape and drops `minLength`, `pattern`, `format` and the rest (`03:2.15`)
   (R). Full schema validation stays with the host.
7. **It makes strict mode usable.** A host that rejects `dynamic` gets a
   language in which untyped data may be held and passed on, and must go
   through `as` or `is` before it is used.
8. **Cost.** One step for each value the test looks at, no allocation, and a
   recursion bounded by the depth of the type, which is at most 64.
9. **Timing (D40).** In 3.0: the host function, `as`, `is`, type text in
   place and host-declared names. In a minor version: `type` lines and
   annotations. Not planned here: tagged unions, recursive types and value
   constraints.

## 2. The proposal

### 2.1 What it looks like

```ts
// An HTTP body. Without the `as`, every read below is a dynamic site.
const order = resp.body as {id: string, total: float, lines: {sku: string, qty: int}[], ...};
order.lines.map(l => l.qty * 2)

// A name the host declared, here from a JSON Schema.
const order = json.parse(msg.text) as Order;

// A fallback when the data is wrong.
const cfg = try(input.config as Config, defaults);

// A test. `body` is `Order[]` in the first branch and still `any` in the second.
if (body is Order[]) { len(body) } else { 0 }

// A string into a literal set, so that `match` can check the cases.
match (kind as "bug" | "feature") { "bug" => "fix", "feature" => "plan" }

// A float into an int, without rounding: it fails unless the number is whole.
xs[(len(xs) / 2) as int]
```

### 2.2 Conformance

The rule is `03:2.14`'s, restated here because it becomes something a program
can see. `03:2.14` is its normative home.

| Type | A value conforms when |
| --- | --- |
| `any` | Always. |
| `null`, `bool`, `string` | It is of that kind. |
| `float` | It is a number of the language: finite. |
| `int` | It is a whole number: finite, with no fraction, within ±(2^53 − 1). `3.0` conforms: no operation can tell how a number is stored (`04:A.2`) (R). `1e20` does not. |
| A literal set | It is a string and a member of the set. |
| `T[]` | It is a list and every element conforms to `T`. |
| A record | It is a map; every required field is present; every field that is present conforms to its type; every other key conforms to the rest. With no rest, there is no other key. |
| `map<T>` | The record with no fields and rest `T`. |
| An opaque name | The binding says the value has that name. |
| A union | It conforms to the alternative of its kind. A type has at most one alternative per kind (`03:2.1`) (R), so there is never a choice to make. |

Four consequences an author will meet:

- **Nothing is coerced.** With `s` an untyped value that holds `"42"`, `s as
  int` fails at run time, and `int(s)` is the conversion. Written on the literal,
  `"42" as int` is already the check error `never-conforms` (2.6).
- **A record with no `...` is exact.** `{id: "a", note: "x"} as {id: string}`
  fails, and the message says to write `{id: string, ...}`. This is the largest
  hazard of the plan, and section 9 asks about it (D41).
- **Absent and null are different.** `{id: "a", note: null}` does not conform to
  `{id: string, note?: string}`. Write `note?: string | null`.
- **A test never looks under `any`.** `x as {meta: any, ...}` checks that `meta`
  is present and nothing more. `x is any[]` checks that `x` is a list and does
  not visit its elements.

### 2.3 `as` and `is`

| | `x as T` | `x is T` |
| --- | --- | --- |
| Value | `x` itself, when it conforms. The same value: nothing is copied. | `true` when `x` conforms, else `false`. |
| When it does not conform | A run error with code `not-conforming` and class `value`. The diagnostic carries the path of the first failure as a field, `["lines", 3, "qty"]`, which the conformance suite compares (`05:A.6`). The message may render it: `.lines[3].qty: expected int, found string`. | `false`. It never fails. |
| With `try` | `try(x as T, fallback)` gives the fallback. | Not needed. |
| Static type | `T`. | `bool`. |
| Narrowing | None is needed: the result has the type. | Where the test passed, `x` has type `T` as written, if `x` is a name or a path the checker can narrow (`03:2.8`). Where it failed, `x` loses each alternative that surely fits `T` (2.6): for `int \| string`, a failed `is int` leaves `string`. An `any` stays `any`. |

**The order of the walk is specified,** so that every implementation reports
the same first failure and charges the same steps.

| At | Order | Stops early |
| --- | --- | --- |
| A list | By index. | At the first element that does not conform. |
| A record's declared fields | In code point order of their keys. A required field that is missing is a failure at that key. | At the first failure. |
| A record's other keys, rest `any` | None is visited. | |
| A record's other keys, another rest type | Every one is tested. If any fails, the one reported is the least by code point. | No. The count then does not depend on an order, as for `==` on two maps (`04:A.3`) (R). |
| A record's other keys, no rest | Whether one exists is known from the map's size. If one does, the test fails and the key reported is the least by code point. | No, for the same reason. |

Declared fields are tested before other keys, and other keys are reached only
when every declared field passed.

**A host value that cannot enter the language** (a number outside the safe
range, a NaN, an infinity) conforms to no type. The walk reports
`not-conforming` at its path and never the error of an ordinary read. So `is`
never fails on host data, and the count does not depend on where in a map such
a value sits (`04:A.3`). At a place typed `any` it is not looked at.

### 2.4 Grammar

```ebnf
Rel  = Add { ( "<" | "<=" | ">" | ">=" | "in" | "not" "in" ) Add
           | ( "as" | "is" ) Type } .
```

`Type` is the type text of `03:2.2`, without `Sig`, `Fn`, `Var`, `unknown` and
`condition`. `02:2.17` has a table of how each text is read, set against
TypeScript.

| Point | Rule |
| --- | --- |
| Precedence | That of a comparison, as in TypeScript (M). `a + b as T` tests the sum. `(x as Order).id` needs the parentheses. |
| Where the type ends | At the first token that cannot continue a type. A `?` always ends it, because no type has a `?` suffix (D29). So `x is T ? a : b` reads as a reader expects. A `[` after a type must be `[]`: `x as T[0]` is an error, and the index is written `(x as T[])[0]`. |
| What may follow | Another comparison, `as` or `is`, or an operator that binds looser. `x as int + 1` is an error that says to write `(x as int) + 1`. `x as T ?? d` needs parentheses by the rule for `??`. `!x is T` is an error that says to write `!(x is T)`. Where TypeScript and this grammar would differ on a text, the text is an error and never a second meaning (`02:2.17`). |
| Nesting | The brackets of a type count toward the 100 levels of source nesting (D31), and so does each `[]` suffix (`02:2.16`). |
| `{` after `as` or `is` | Starts a record type, never a map or a block. |
| `\|` | In a type it joins alternatives. It is not an operator of expressions, so there is no clash. `\|\|` and `??` end the type. |
| Words | `as` and `is` become reserved words. An upper-case name of two or more characters after them is a type name. |

### 2.5 Where a type comes from

| Source | Example | Version |
| --- | --- | --- |
| Type text in place | `x as {id: string, ...}` | With `as` |
| A name the host declares | `x as Order` | With `as` |
| A `type` line in a script | `type Order = {id: string, ...};` | A minor version |

**Names the host declares.** An environment gets an eighth kind of declaration
(`05:A.2` has seven) (R): a named type.

| Matter | Rule |
| --- | --- |
| Form | A name and a type. The portable forms of the type are type text and JSON Schema (P5), as for a variable. |
| Name | An upper-case letter, then one or more of `[A-Za-z0-9_]`. It shares one set of names with opaque types. A clash is `name-clash`. |
| Meaning | An abbreviation. `Order` and the type it names are the same type everywhere: in fits, in Join, in conformance. |
| Reference | A named type may use names declared before it. It cannot refer to itself, directly or through others. A cycle, or a name that does not resolve, is the `declare` error `bad-type`. The other declarations of an environment (a variable, a signature) may use any type name, wherever they stand. |
| Printing | A type is printed by its structure, as today. A message about a failed `as` also quotes the type as the program wrote it. Section 9 asks whether names should print (D44). |
| Description | The environment's description lists named types, so an editor can complete them. |

**`type` lines** (a minor version). `type Name = Type;` at the top level of a
program, among its `const` lines. The same rules as a host's named type: declare
before use, no cycle, no clash with an environment's type name. A type line has
no value and no cost. `type` stays a library function: `type Name =` at the
start of a line is the declaration and `type(` is the call, as TypeScript reads
it (M).

*Why a host-declared name comes first.* Most programs are one expression in a
field of a host document. A `type` line has nowhere to stand there, and no way
to be shared with the next field. The host's document is where such a type
belongs.

### 2.6 Checking

| Case | `x as T` | `x is T` |
| --- | --- | --- |
| The type of `x` surely fits `T` | Accepted, with nothing reported. It is how a type is widened or stated: `[] as string[]`. It is not a site: under the checker's precondition it cannot fail. | `always-truthy`, of the class `constant-condition` (B7). |
| The two types are disjoint | Error `never-conforms`, as `never-equal` is for `==`. | `always-falsy`, of the same class. |
| Anything else | The type is `T`. The node is a site of class `value`, as a call of a library entry marked `!` is (`03:2.14`) (R). Inside the first argument of `try` it is not a site. | The type is `bool`. Not a site. |

- **Surely fits** is Fits that did not use `any` (`03:2.3`). Plain Fits would
  be wrong here: `any` fits every type, so `blob as Order` would never be
  tested. An untyped subject is always the third case.
- **Disjoint** means the checker can show that no value conforms to both
  (`03:2.14`). The rule is sound and not complete. Two list types are never
  disjoint, because `[]` conforms to both.
- **A type name that nothing declares** is `undefined-name`. **A written type
  deeper than 64** is `check-limit` (`03:2.17`). Both are found when the type
  is compiled, so an evaluator with no checker refuses the program too.
- **A failed `as` where the subject surely fits** means a host value broke its
  declared type. It is reported with class `environment`, like every failure
  the precondition rules out (`03:2.14`).
- **No new check class and no new kind of failure.** A host that rejects the
  class `value` already means "nothing here may fail on a value". Under that
  host an `as` goes inside `try`, or the program uses `is`.
- **The constant-condition class fits `is` for the reason it exists.**
  Tightening a declared type can make a stored `x is T` always true. A host
  that re-checks stored programs after a schema change already takes that class
  out (D32).
- **`03:2.12` changes by one sentence.** It says types never change results.
  That stays true of inferred and declared types. A type written after `as` or
  `is` is an operand of a test.
- **A run without the checker behaves the same.** The test and its cost depend
  only on the value and on the type as written, never on an inferred type.

### 2.7 Cost and limits (C5)

| Limit | Rule |
| --- | --- |
| Steps | 1 for each value tested: the subject, each list element reached, each declared field looked up, each other key of a record. A value at a place whose type is `any` is not tested and costs nothing. So the elements of an `any[]`, and the other keys of a record whose rest is `any`, are not visited: `x is any[]` costs 1 step, as `type(x) == "list"` does. `04:A.3` has the exact count for every case, with worked examples. |
| The operator itself | 0. `x as any` costs nothing. |
| A declared field | 1 for the lookup, whether the key is present or not. When it is present, that step is also the step of its value. A field of type `any` still costs its lookup. |
| An exact record that has other keys | Finding that one exists is free. The failure then costs 1 for each other key, which pays for finding the least one. |
| When the implementation can prove the result | The same count. It may skip the work, not the charge (`04:A.8`) (R). A type the checker inferred is not a proof: a host's value may break the precondition, and a run without the checker must behave the same (2.6). |
| Allocation | None when the value conforms. A failure builds one bounded message. |
| Depth | The walk enters one level for each level of the type. A type is at most 64 deep (`03:2.17`) (R), so the recursion is bounded by the text, not by the data. This satisfies R5 without a new number. Each level entered also counts 1 against the run's depth limit, as in every other walk over a value (`04:A.6`). |
| Host values | The walk reads a host value through the binding's ordinary reads: length, element, field, keys. For lists and records it needs nothing new of a binding. For an opaque name the binding must know which host values carry it, so an opaque type is declared by name (`05:A.2`). |

*Why a test once is enough.* Values are immutable. A value that conformed keeps
conforming, so no wrapper and no later check is needed. Languages that check
deep structure at a boundary and allow mutation need both (M).

### 2.8 The host's function

Every binding exposes the same rule to the host. `05:A.13` owns the rule and
Parts B to D of `05` the exact forms.

| Binding | Form |
| --- | --- |
| Go | `func Conform(v any, t Type) error`, with `t` from `env.ParseType(text)` |
| TypeScript | `env.conform(value, typeText)` |
| Rust | `env.ty(text)?.conform(&value)` |

- **A type's names belong to an environment.** A named type or an opaque name
  can be resolved only there, so the type is resolved by an environment first.
  A type with no name needs none.
- The error is the `not-conforming` diagnostic of 2.3, with the same path and
  no span.
- **It is not a run.** It takes no limits and reports no steps. Its work is
  bounded by the value and its depth by the type. A host that wants a limit and
  a count runs the one-node program `v as T`.
- **This is the precondition of the promise, made testable.** `03:2.14` assumes
  that every variable conforms to its declared type (R). A host can now test
  that at its own boundary, in a test suite or in production, with the rule the
  checker assumed and not one of its own.
- It needs no parser change and no checker change, so it can ship before the
  operators.

### 2.9 JSON Schema

| Question | Answer |
| --- | --- |
| How does a schema become a type? | By the table of `03:2.15`, unchanged. |
| Which options apply to a named type used with `as`? | None, by default. The three options of `03:2.15` describe what a host did to a value before the run. Data met inside a run was not normalized by anyone. So an absent property is absent, an undeclared key is allowed unless the schema forbids it, and a default is not applied. |
| What does `x as Order` check, when `Order` came from a schema? | The type. Not the schema. |
| What is dropped? | Every keyword that constrains values: `minimum`, `maxLength`, `pattern`, `format`, `minItems`, `uniqueItems` and the like. Also the difference between `oneOf` and `anyOf`, and any pairing of fields across branches: two object branches join into one record. |
| How does a host validate the whole schema? | With its own validator, in a host function. The language does not get one. |

*Why not a validator in the language.* It would be three implementations of
JSON Schema. `pattern` needs regular expressions, which are out of 3.0 (D9).
`format` is a list that grows. The drafts differ. And a host that holds schemas
already has a validator in its own language.

*What this costs.* An author who sees `as Order` may believe the schema was
enforced. The name of the feature and its documentation must say "type".

## 3. What is not in this plan

| Left out | Why | When |
| --- | --- | --- |
| Coercion: `"42"` accepted as `int`, a missing field filled from a default | It makes `as` a second set of conversion rules beside `int(x)` and `??`. Lax-by-default validators are a known source of surprise (M). | Not planned |
| Removing undeclared keys | The result would be a new value, so `as` would allocate, `(x as T) == x` could be false, and `is` could not agree with `as`. | A library function could do it later |
| Tagged unions: `{type: "a", x: int} \| {type: "b", y: string}` | A type has one alternative per kind (`03:2.1`), so the two records join. This plan gives the feature its first consumer. | Its own plan, since written and accepted: `09-tagged-unions.md` (D47) |
| Recursive types: a tree, a comment thread | They end the bound on the walk's depth and on the checker's type depth, and they are the first cyclic structure in the design (R3). Write `any` at the point of recursion. | Its own plan |
| Value constraints: a range, a length, a pattern | A value-level type system (`03:2.1`, "Refinements"). | Not planned |
| Annotations: `const o: Order = x` | Already reserved (`02:8.1`, question 8). An annotation is a claim checked statically. `as` is a test. They are different things and both are useful. | A minor version, with `type` lines |
| Generic named types: `Page<T>` | No user generics (`03:2.1`). | Not planned |

## 4. Changes to the other documents

| Document | Change |
| --- | --- |
| `02` | `as` and `is` in the grammar and the reserved words. One node, `As`, with a `form` field and a type: 31 kinds, from 30. Targeted errors for `satisfies`, `interface`, `enum`, `instanceof`, `typeof x` and `<T>x`. A row in 3.1: `as` is tested here and trusted in TypeScript. Later: the `type` line. |
| `03` | 2.5: the rules of 2.6 above. 2.8: `is` as a narrowing test. 2.12: the sentence about results. 2.13: `never-conforms`, so 28 codes. 2.14: `as` among the `value` sites. 2.15: named types. 2.16: conformance cases for the walk. |
| `04` | A.3: the step rule. A.6: the depth rule. B: a type compiles to a test once, when the program is compiled. Parts C and D: a note each. |
| `05` | A.2: named types as a declaration, and in the description. A.6: the run code `not-conforming` with its path field. The `Conform` function in all three bindings. |
| `06` | No new entry. `json.parse(s) as T` becomes the idiom in the examples. |
| Conformance suite | A new kind of case: a type, a value, whether it conforms, the path of the failure and the steps charged. The same files test the operators and the host's function. |

Every evaluator now needs the type text parser and a run-time form of a type.
Before this plan only a checker needed them. This matters for the Rust and
TypeScript implementations, whose checkers come last in the roadmap.

## 5. Size (I)

| Part | Go, lines without tests |
| --- | --- |
| Type text parser moved from the checker to the core | 0 new; about 300 moved |
| (The aspect documents now carry their own figures: the checker grows by 130 lines (`03`) and the evaluator by 350 (`04`). They replace the rows below where they differ.) | |
| The walk, with paths and step charges | 250 |
| Parser and node | 80 |
| Checker rules and `is` narrowing | 150 |
| Named types in the environment and its description | 120 |
| `Conform` in the binding | 60 |
| Total new | about 660 |

The Rust and TypeScript figures should be close. None of this was built.

## 6. Order of work

| Step | Work | Depends on |
| --- | --- | --- |
| 1 | Reserve `as` and `is` in `02`, and the `type Name =` form. Decide D39 to D41. | Nothing. Do it before 3.0 whatever else is decided |
| 2 | The walk and `Conform` in Go, with conformance cases | The value model of `04` |
| 3 | Named types in the environment | Step 2 |
| 4 | `as` and `is` in the parser, the evaluator and the checker | Steps 2 and 3 |
| 5 | The same in TypeScript and Rust, against the same cases | Their evaluators |
| 6 | `type` lines and annotations | A minor version |

## 7. Prior art (M)

All of this is from memory. It should be checked before the plan is final.

| Language or library | What it does | What I take from it |
| --- | --- | --- |
| TypeScript | `as` is trusted and not tested. `satisfies` is a static check. `x is T` appears only in the signature of a guard the author writes by hand. | The words. Not the meaning of `as`: an untested cast would break the promise of `03:2.14`. |
| Dart, Kotlin, C#, Swift | `as` is tested and fails. `is` tests and narrows. Kotlin's `as?` and Swift's `as?` give null. | Tested `as` with narrowing `is` is ordinary. Their test reads one tag. Ours walks the value, so its cost grows with the data. |
| Zod and similar | `parse` returns a typed value or fails. `safeParse` returns a result. Unknown keys are removed by default. Coercion is opt-in. | The two forms. The default for unknown keys goes against this plan (D41). |
| Pydantic | Lax by default: `"42"` becomes `42`. A strict mode exists. | Strict only. |
| Rust serde | Unknown fields are ignored unless the type says to deny them. | Again against D41's recommendation. |
| JSON Schema | Undeclared properties are allowed unless `additionalProperties` is false. `1.0` is a valid `integer`. | A named type from a schema is open by default, and `3.0 as int` passes. |
| Gradual typing research | A cast at a boundary needs a wrapper when data can change after the cast. | Immutable values make one eager test enough (2.7). |
| Go | `v, ok := x.(T)` | The two forms again. |

## 8. Alternatives rejected

| Alternative | Why not |
| --- | --- |
| `as` as in TypeScript: trusted, free | The checker's promise would end at the first `as`. It is the wart C6 says to leave out. |
| A function with the type in a string: `conform(x, "Order[]")` | Type text inside a string literal: no highlighting, no completion, and the checker must read a constant string as a type. |
| The type's name as a function: `Order(x)` | It reads like `int(x)`, which converts. It cannot take type text in place. |
| An annotation that tests: `const o: Order = x` runs a test when `x` is `any` | A cost and a failure with no word in the text that shows them. A TypeScript author reads an annotation as free. |
| `as` only, with `try` for the other case | `try(x as T, null)` loses the difference between "did not conform" and "conformed and is null", and nothing narrows in an `if`. |
| `is` only | The common case, "this is an `Order` or the run is wrong", would need an `if` and a `fail` every time. |
| Stop at the first failure everywhere, in the map's own order | Three implementations have three map orders. The first failure and the step count would differ. |
| A JSON Schema validator in the language | Section 2.9. |

## 9. Decisions, all settled by the owner as recommended

| # | Decision | Settled as |
| --- | --- | --- |
| D39 | Are `as` and `is` the words, with `as` tested? | Yes. The trap is that a TypeScript author expects `as` to be free. The surprise is on the safe side: a cost and a clear error, where TypeScript gives a wrong value later. Reserve both words in 3.0 whatever the timing. |
| D40 | Timing | The host's `Conform`, `as`, `is`, type text in place and named types in 3.0. `type` lines and annotations in a minor version. A workflow meets untyped data on its first day, and without this the choice is `dynamic` sites everywhere or a strict mode nobody can use. The cost is that every evaluator carries type text. |
| D41 | An exact record and unknown keys. `x as {id: string}` fails on a body with any other key, and a service that adds a field next month breaks every such program. | Keep the rule and make the message name the fix: write `...`. One rule for a type everywhere, and `is` agrees with `as`. Most prior art goes the other way, so this is the decision I am least sure of. The alternative I would take is: a record written after `as` or `is` allows other keys, and the result's type is the open record. The alternative I would not take is to remove the keys. |
| D42 | Absent against null. `note?: string` does not accept `note: null`. | Keep. The author writes `note?: string \| null`. It matches JSON Schema and TypeScript. |
| D43 | Does a named type from a JSON Schema carry any value constraint into `as`? | No. Types only (2.9). |
| D44 | Should a named type print by its name in a diagnostic? | Not in the first version. Printing by structure is what every implementation already agrees on. Long records in messages are the cost. |
| D45 | `type` as a contextual word for a later `type Name = ...`, with `type(x)` kept | Yes. The other choice is to rename the function. |

## 10. What the revision of `02` to `06` corrected, and what is still open

The first draft of this plan was wrong or silent at these points. Each is now
fixed above, and the document named owns the detail.

| Point | First draft | Now | Owner |
| --- | --- | --- | --- |
| The first case of 2.6 | "The type of `x` fits `T`", which `any` always does | Surely fits | `03:2.5` |
| "No value conforms to both" | Not defined | Disjoint, defined, sound and not complete | `03:2.14` |
| `int` | "A whole number", which read alone admits `1e20` | Within the safe range | `03:2.14` |
| Values under `any` | Charged a step each | Not visited, not charged | `04:A.3` |
| The step count | One sentence | Exact, with worked counts | `04:A.3` |
| An exact record with other keys | Unsaid | Free to detect; 1 for each other key on failure | `04:A.3` |
| A host number that cannot enter the language | Unsaid | Conforms to no type; never a second error | `04:A.3`, `05:A.13` |
| Depth | Bounded by the type only | Also counted against the depth limit | `04:A.6` |
| Skipping the walk | Allowed where the checker's type fits | Allowed only on a proof from the value itself, never on the checker's word | `04:A.8` |
| The path of a failure | Field or message, open | A field, as data | `05:A.6` |
| The host's function | A free function in every binding | Types are resolved by an environment; it is not a run and takes no limits | `05:A.13` |
| Opaque names | "Nothing new of a binding" | An opaque type is declared by name | `05:A.2` |
| Where a type ends | "The first token that cannot continue a type" | Plus the rules for `[`, `+`, `??` and `!` | `02:2.17` |
| The failing side of `is` | Open | It narrows | `03:2.8` |
| The class of a failed `as` at a place that is not a site | Open | `environment` | `03:2.14` |
| An unknown type name; a written type that is too deep | No code | `undefined-name`; `check-limit` | `03:2.13`, `03:2.17` |

Still open:

- **The passing side of `is` takes `T` as written.** It can lose what was
  known: `x: {a: int, ...}` tested with `{b: string, ...}` no longer has `a`.
  The fix would be an intersection of types (`03`, open question 8).
- **Which key a failed exact record reports.** The least other key, at a cost
  of 1 for each other key. Reporting the record itself would make the failure
  free (`04`, open question 8).
- Whether a host may give a limit on the size of a type written in place.
- A `match` over types, once `is` exists.
- Measurements: the walk's cost per value in Go, on a host struct and on a
  parsed JSON body (`04`, benchmark group K).
- Section 7 is from memory and is not yet checked.
