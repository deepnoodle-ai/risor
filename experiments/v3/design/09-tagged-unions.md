# Risor v3 design: tagged unions

Status: plan accepted by the owner, with every decision of section 8 as
recommended (D47 in `00-overview.md`). `02` to `06` are revised to match. No
code was written and nothing was measured. Decision numbers such as D13 and D41 refer to
`00-overview.md`. This plan's own decisions are `09-1` to `09-8` (section 8).

**How to read this.** Section 2 is the proposal, written as rules so that it can
move into the specification. A paragraph headed *Why* is not a rule. Sections 3
to 9 are not rules.

**Evidence.** **(R)** read in a sibling document at the place cited. **(I)**
inferred. **(C)** checked for this draft against the published documentation of
the language or library named. "(from memory)" marks a claim not checked.
`02:` to `07:` are the sibling documents.

**The running example.** A host declared one named type (`03:2.15`) and three
variables: `s: Shape`, `shapes: Shape[]` and `blob: any`.

```
Shape = {kind: "circle", r: float} | {kind: "rect", h: float, w: float}
```

## 1. Summary

1. **The problem.** Data often has one of several shapes, told apart by one
   field: an event, a part of a message, a step of a pipeline, a backend in a
   config file. Today a type has at most one alternative per kind, so two record
   types join into one record with optional fields (`03:2.1`, `03:2.3`) (R). The
   checker then cannot say that a circle has `r` and a rectangle does not.
2. **The proposal.** The map kind of a type may hold several records when one
   required field tells them apart: its type in each record is a string literal
   set, and no string is in two of the sets. That field is the tag. Nothing else
   may hold two records. There are no untagged unions of records.
3. **No new syntax.** Type text already reads `{...} | {...}` (`03:2.2`). No
   node, no word and no operator is added.
4. **Enums are not part of this.** They are string literal sets already (2.2).
5. **Use.** Before a test, a tagged union reads as the joined record it is
   today. `s.kind == "circle"`, `!=`, `match s.kind` and `s is T` narrow it to
   the variants that remain.
6. **Only a written type makes one.** Join never builds a tagged union from two
   records. Where a literal meets a type someone wrote, the fit goes through the
   literal, so a config file gets its error at the element that is wrong.
7. **The test.** `as`, `is` and the host's function read the tag, then test one
   variant. A value that conforms costs exactly what it costs against its
   variant alone.
8. **JSON Schema.** `anyOf` and `oneOf` give a tagged union when their object
   branches form one, and the Join otherwise, as today.
9. **Counts.** One new check code, `untagged-union`: 29 codes, from 28. Still 31
   node kinds, six check classes and three kinds of fact. No new run code,
   reserved word, declaration or limit.
10. **Later, not now:** tags that are numbers or booleans, variants told apart
    only by two fields together, structural patterns in `match`, recursive
    types.

## 2. The proposal

### 2.1 What it looks like

```ts
// A test of the tag narrows. `s.r` reads only where `s` is a circle.
s.kind == "circle" ? 3.14159 * s.r ** 2 : s.w * s.h

// `match` on the tag is checked for coverage. A third variant added to
// Shape makes this a compile error until it has an arm.
match s.kind { "circle" => s.r * 2, "rect" => s.w + s.h }

// Untyped data. One test: it reads the tag, then one variant.
const e = json.parse(msg.text) as Shape;

// A config file whose host expects Shape[]. The second element has no `h`,
// and the error is on the second element.
[{kind: "circle", r: 1}, {kind: "rect", w: 2}]
```

### 2.2 Enums

An enum is a string literal set (`03:2.1`, D13) (R). This plan changes nothing
here. The table is for a reader who looks for the word.

| Want | Write |
| --- | --- |
| A type of named cases | `"fail" \| "pass"`, in type text or from a schema's `enum` (`03:2.15`) |
| A name for it | A named type the host declares: `Status = "fail" \| "pass"` |
| A choice by case, checked for coverage | `match status { "fail" => 0, "pass" => 1 }` (`03:2.9`) |
| A string of unknown origin as a case | `x as "fail" \| "pass"`, or `x is "fail"`, which narrows both ways (`03:2.8`) |
| A numeric enum | `int`. A `match` on it needs `_` (D13) |
| `enum Name { ... }` | A targeted error (`02:3.2`) |

### 2.3 The type

| Matter | Rule |
| --- | --- |
| What may hold two records | The map kind of a type holds one record, or one **tagged union**: two or more records, its **variants**. So a type still has at most one alternative of a kind (`03:2.1`). |
| A tag | A key is a tag of some records when every one of them requires it, its type in each is a string literal set (that record's **tag set**), and no string is in two of the sets. |
| The tag | Records form a tagged union when they have a tag. **The tag** of the union is the least such key by code point. It is found from the variants and never written. |
| A variant | Any record with the tag: exact or open, with optional fields, with a field that is itself a tagged union. |
| Type text | No new grammar. `Type = Alt { "\|" Alt }` reads it (`03:2.2`). Take the alternatives of one `Type` with names replaced and parentheses removed. Alternatives of any other kind join as today. If two or more are records, those records must form a tagged union. A named type that is a tagged union gives its variants, so `Shape \| {kind: "tri", a: float}` has three. |
| Not tagged | An error where the text is read. In a declaration it is `bad-type` (`05:A.6`). After `as` or `is` it is the new check code `untagged-union`, on the union: from the start of its first alternative to the end of its last. The node then has type `never` for `as` and `bool` for `is`, and gives no fact, as for `undefined-name` (`03:2.5`). The compiler makes the test when it reads the type, so an evaluator with no checker refuses the program too (`03:2.17`). |
| Examples of not tagged | `{a: int} \| {b: string}`: no common key. `{kind: string} \| {kind: "a"}`: a tag type that is not a literal set. `{kind: "a"} \| {kind?: "b"}`: not required. `{kind: "a" \| "b"} \| {kind: "b"}`: a shared string. `Shape \| Shape`: the same. `{kind: "a"} \| map<int>`: no tag in the second. |
| Printing | The variants stand where the record stood, each printed as a record, in code point order of the least string of each tag set, joined with `\|`: `{kind: "circle", r: float} \| {kind: "rect", h: float, w: float} \| null`. As a list element it takes parentheses, by today's rule for a `\|` at the top level. |
| Depth | That of its deepest variant (`03:2.17`). A tagged union adds no level of its own, and the limit of 64 stands. |
| flat(U) | The Join of the variants of U, taken as plain records by today's map row of Join (`03:2.3`). It is one record, and every value that conforms to U conforms to it. flat of a record is the record. |

flat(`Shape`) is `{h?: float, kind: "circle" | "rect", r?: float, w?: float}`.
It is the type the text of `Shape` has today.

*How a checker holds it.* The map slot of a type holds a record or a sorted list
of variants. flat(U) is computed once for each union and kept.

*Why the tag is found and not written (09-1).* The text is then TypeScript's,
with nothing to learn (C6). A tag named in the text would be new grammar, and
the variants already say which key it is.

### 2.4 Join, Fits and the helpers

Each rule below replaces or extends one row of `03:2.3`, `03:2.8` or `03:2.14`.
A record counts as a union of one variant.

| Operation | Rule |
| --- | --- |
| Join, map kind in both | Two records: one record, as today. **Join never makes a tagged union** (09-3). When either side is a tagged union: that union if the two are the same by structure, else Join(flat(A), flat(B)). Join stays associative and commutative (I). |
| Fits(S, T), map kind in both | Each variant of S fits some variant of T by today's record row. |
| S surely fits T | Each variant of S surely fits some variant of T. A fit that holds, and is not a sure fit, used any. |
| disjoint(A, B), map kind in both | They share a value unless every variant of A is disjoint from every variant of B by today's record row. |
| without(X, T) | The map kind of X loses each variant that surely fits T. One variant left is a record. None left: X has no map alternative. |
| Truthy(U), Falsy(U) | U, and –. Every variant requires its tag, so no value of U is an empty map, and a test of one alone is `always-truthy`. |
| Merge, bind, a pattern, a spread, a map method | Each sees flat(U). |
| overlap, nonnull | Unchanged: they stop at the kind. has-opaque(U) holds when it holds of a variant. |

| Written | Result | Why |
| --- | --- | --- |
| Join(`Shape`, `Shape`) | `Shape` | The same by structure. |
| Join(`{kind: "circle", r: int}`, `{kind: "rect", h: int, w: int}`) | `{h?: int, kind: "circle" \| "rect", r?: int, w?: int}` | Two records. As today. |
| Join(`Shape`, `{kind: "circle", r: int}`) | flat(`Shape`) | Not the same, so both are flattened. The cost is in section 9. |
| Fits(`{kind: "circle", r: int}`, `Shape`) | yes | It fits the first variant. |
| Fits(flat(`Shape`), `Shape`) | no | Its `kind` is within no one tag set. |
| Fits(`Shape`, `{kind: string, ...}`) | yes | Each variant fits. |
| disjoint(`Shape`, `{kind: "tri", ...}`) | yes | `kind` is required and its sets share nothing, for each variant. |
| without(`Shape`, `{kind: "circle", r: float}`) | `{kind: "rect", h: float, w: float}` | |

### 2.5 Reads and narrowing

**A read is a read of flat(U)** (09-5), by the rules of `03:2.7`. No rule is
added.

| Written | Result |
| --- | --- |
| `s.kind` | `"circle" \| "rect"`: the tag is required in flat(U), with the union of the tag sets. |
| `s.r` | `maybe-absent` on `r`: only some variants have it. A help text can name the test of `s.kind`. |
| `s?.r ?? 0` | `float`. |
| `s.color` | `no-field`: no variant has it and every variant is exact. |
| `{...s, id: 1}`, `const {kind, ...rest} = s` | Typed from flat(`Shape`). |

**Narrowing** (09-6). Let f be a key and S a set of strings. For a tagged union
U:

- hit(U, f, S) is the variants in which the type of key f is not disjoint from
  the literal set S;
- all(U, f, S) is the variants that require f with a type that surely fits S.

The type of a key in a record is its field's type if the record lists it, else
the rest (`03:2.14`). These rows join the table of T(c) and F(c) in `03:2.8`. In
them p is a path, L is a `String` node, the last step is `.f`, `?.f`, `["f"]` or
`?.["f"]`, and U is the tagged union in the type of p as narrowed where the test
stands. With no tagged union there, no fact is given, as today.

| c | T(c) | F(c) |
| --- | --- | --- |
| `p.f == L`, `L == p.f` | {p: hit(U, f, {L})} | {p: not all(U, f, {L})} |
| `p.f != L`, `L != p.f` | {p: not all(U, f, {L})} | {p: hit(U, f, {L})} |

| Matter | Rule |
| --- | --- |
| The facts | They are the two kinds `is` gives (`03:2.8`): "conforms" and "does not conform". So `!`, `&&`, `\|\|`, `? :`, an `if` chain and a condition arm combine them with no new rule. A fact that names no variant is no fact. |
| `match p.f { ... }` by cases | An arm whose literals are the strings S is checked under {p: hit(U, f, S)}. Every later arm, `_` included, is checked under {p: not all(U, f, S)}, with U as the earlier arms left it. An arm that also lists a literal that is not a string narrows nothing in its own result. Coverage is unchanged: the subject is a literal set (`03:2.9`). |
| `p is T` | As today: T where it passed, without(type of p, T) where it failed. |
| Null | A "conforms" fact for a type with no null alternative says p is not null (`03:2.8`), so `p?.kind == "circle" && p.r` checks. |
| Which field | f is usually the tag. The rule does not ask: any field that tells variants apart narrows, as in TypeScript (C). |
| Not narrowed | A tag copied to a name: `const k = s.kind; k == "circle"`. A test with `in`: `s.kind in ["circle"]`, `"r" in s`. A predicate was not narrowed when this plan was written; plan `13` since made `filter` and `find` narrow, so `shapes.filter(x => x.kind == "circle")` is a list of the circle variant (`03:2.11`). A plain literal set under `==`, as today. |

| Written | Result |
| --- | --- |
| `s.kind == "circle" ? s.r : s.w * s.h` | `float`, with no site. |
| `match s.kind { "circle" => s.r, "rect" => s.w }` | `float`. With a third variant in `Shape`: `non-exhaustive`. |
| `match s.kind { "circle" => s.r, _ => s.h }` | `float`: under `_`, `s` is the other variant. |
| `if (s.kind != "rect") { s.r } else { s.h }` | `float`. |
| `s.kind == "tri"` | `never-equal`, as today, and no fact. |
| `s \|\| d` | `always-truthy` on `s`. |
| `s is {kind: "circle", r: float} ? s.r : s.h` | `float`: `is` narrows both ways. |

*Why the true fact is sound.* The value of p conforms to the type of p: that is
the precondition of `03:2.14`, or the result of an earlier test. So it conforms
to one variant. `p.f` was read and equals L, so in that variant the type of f is
not disjoint from {L}, and the variant is in hit. The false fact needs no
precondition: a value in a variant of all has `f` in S.

### 2.6 A literal against a written type

Join never makes a tagged union, so the last example of 2.1 has the type
`{kind: "circle" | "rect", r?: int, w?: int}[]`, which does not fit `Shape[]`.
A config file is that expression. So where an expression must fit a type that
someone wrote, the fit is tried through the expression's form (09-4).

**e fits T through its form** when the type of e fits T, or, failing that, by
one row. The type of e is the one synth gave it (`03:2.5`): nothing is checked
twice.

| e | It fits when | Place of a failure |
| --- | --- | --- |
| A `List`, where T has a list alternative `E[]` | Each item fits E through its form. The type of each spread's operand fits `E[]`. | The item, or inside it by these rows. The operand. |
| A `Map` with no spread and no computed key, where the map alternative of T is a record R | Each entry whose key R lists fits that field's type through its form. Each other entry fits R's rest through its form. Each required field of R has an entry. | The entry's value, or inside it. The entry's key when R has no rest. e when a field is missing. |
| The same, where the map alternative of T is a tagged union | The type of the tag's entry is a literal set within the tag set of one variant. e then fits that variant by the row above. | The value of the tag's entry. e when it has none. |
| A `Cond` of either form, a `Match`, a `Block` | Each branch, each arm's result, the block's result fits T through its form. | By these rows. |
| Anything else, or a row whose condition T does not meet | It does not fit. | e |

| Where | Today | With this plan |
| --- | --- | --- |
| The program's result against an expected type (`03:2.5`, row 1) | Must fit. `result-mismatch` on the result. | Must fit through its form. `result-mismatch` at each place of failure. |
| A plain argument against its parameter (`03:2.11`, step 5) | Must fit. `maybe-null`, else `arg-mismatch`, on the argument. | Must fit through its form. The same code, at each place of failure. |

- **A program accepted today is accepted, with the same type and sites.**
  Today's fit is tried first. A fit that used any is a `dynamic` site on the
  expression whose fit used it.
- **The spans of two codes move inward,** with or without a tagged union: `flag
  ? 1 : "a"` against `int` is reported on `"a"`.
- **No type changes.** The program's type is still the inferred one. That the
  value also conforms to the expected type is what the check showed.
- **`x as T` is not one of the places.** `[...] as Shape[]` is a `value` site
  and is tested when it runs, as today. An annotation, `const x: Shape[] =
  [...]`, is a third place when annotations come (D40).

*Why it is sound.* A list's value has the values of its items and the elements
of its spreads. A map literal with no spread and no computed key has exactly the
keys written. A conditional's value is the value of one branch. So each row
shows that every value the expression can have conforms to T.

### 2.7 Conformance and its cost

The row for the table of `03:2.14`: **a value conforms to a tagged union when it
is a map, it has the tag, the tag's value is a string in the tag set of one
variant, and it conforms to that variant.** Tag sets share no string, so there
is never a choice, and `07:2.2` and `03:2.14` keep that sentence. `as`, `is` and
the host's function (`05:A.13`) share the walk, as today.

| The walk, at a value tested against a tagged union | Steps (`04:A.3`) | A failure here has a path that ends at |
| --- | --- | --- |
| 1. The value | 1. It is not a map: the test fails. | The value itself. |
| 2. The tag | 1 for the lookup, whether the key is present or not. When it is present, that step is also the step of its value. It is absent, or not a string, or a string in no tag set: the test fails. | The tag. |
| 3. The variant | The walk of a record (`07:2.3`) over the variant's other declared fields, in code point order of their keys, then its other keys. The map and the tag are not charged again, and the tag is not an other key. | As for a record. |

| Point | Rule |
| --- | --- |
| A value that conforms | Costs exactly what it costs against its variant written alone: one step for the map and one for each declared field, the tag among them. |
| A value that fails | May cost one step more than against the variant alone, because the tag is read before a field that sorts ahead of it. |
| Finding the variant | Nothing beyond the tag's step, however many variants there are. As for a literal set, the comparison reads no more of the value than the longest literal (`04:A.3`). An implementation keeps a table from tag string to variant. |
| Depth | The walk enters one level, as for a record (`04:A.6`). |
| Allocation | None. A failure holds its path, as today (`04:A.4`). |
| The diagnostic | `not-conforming`, class `value`, with the path (`05:A.6`). Nothing new. |
| A proof | `04:A.8` stands: the work may go, the charge never does. |

| Value, against `Shape` | Conforms | Path | Steps |
| --- | --- | --- | --- |
| `{kind: "rect", w: 2, h: 3}` | yes | | 4: the map, `kind`, `h`, `w` |
| `{kind: "rect", w: 2}` | no | `["h"]` | 3: the map, `kind`, `h` |
| `{kind: "oval", r: 1}`, `{r: 1}`, `{kind: 7}` | no | `["kind"]` | 2: the map, `kind` |
| `{kind: "circle", r: "1"}` | no | `["r"]` | 3 |
| `{kind: "circle", r: 1, note: "x"}` | no | `["note"]` | 4: the map, `kind`, `r`, and 1 for the one other key |
| `"circle"` | no | `[]` | 1 |
| A circle, then a rectangle, against `Shape[]` | yes | | 8: the list, 3, 4 |

### 2.8 JSON Schema

| Question | Answer |
| --- | --- |
| What do `anyOf` and `oneOf` give today? | The Join of the branches (`03:2.15`) (R). |
| And with this plan (09-8)? | Each branch is mapped as today. Take the records the branches give, where a branch that is itself a tagged union gives its variants. Two or more records that form a tagged union (2.3) give that union, joined with what the other branches give. Two or more that do not form one give the Join, as today. |
| What makes a branch a variant? | Nothing new. A property in `required` whose schema is a `const` string or an `enum` of strings is already a required field with a literal set. |
| `oneOf` against `anyOf` | The same for a tagged union: no value matches two branches. D43 dropped the difference, and here there is none to drop. |
| A tag property that is not in `required`, or is typed `string` | Not a tag. The branches join, and nothing is reported. A schema is written for another tool, so what it cannot say maps to the wider type, as in every other row of `03:2.15`. Type text is written for this language, so there the same mistake is an error. |
| OpenAPI's `discriminator` | Ignored, like any keyword outside the table. The tag is found from the branches. |
| The three options | They apply to each variant as to any record. *closed* makes each variant exact. |
| Back to text | A tagged union prints as type text that reads back as the same type, so the environment's description carries it (`05:A.2`). |

## 3. What is not in this plan

| Left out | Why | When |
| --- | --- | --- |
| Tags that are numbers or booleans | Literal sets are strings only (D13). | With number literal sets, if they come |
| Variants told apart only by two fields together | The walk would read two keys before it knew the variant. | Later |
| Structural patterns in `match`: `{kind: "circle", r} => r` | `02:8.1`, question 3. This plan supplies the types. Binding in a pattern is the rest. | Its own plan |
| Recursive types: a tree of variants | `03:2.17`, R3. Write `any` at the point of recursion. | Its own plan |
| Untagged unions of records | The owner's ruling. Section 7 has the cost. | Not planned |
| A Join that keeps a union when one side surely fits the other | It is not associative (section 7). | Later, with an order rule |
| An annotation, `const x: Shape[] = [...]` | D40. It reuses 2.6. | A minor version |
| `in` as a narrowing test | A new kind of fact. | Later. (A predicate that narrows an element type came with plan `13`, for `filter` and `find`.) |
| A Rust `enum` or a Go interface derived into a tagged union | A convenience of one binding (P5). Type text and JSON Schema are enough. | Later |

## 4. Changes to the other documents

| Document | Change |
| --- | --- |
| `02` | No grammar, node or word. 2.17: one row, `x as {kind: "a"} \| {kind: "b"}` reads as one type. 3.1: a union of records with no tag is an error here and a type in TypeScript. 8.1, question 3: the types now exist. |
| `03` | 2.1: the map kind's alternative, and tagged unions leave the left-out table. 2.2: the rules of 2.3 above, printing, and one sentence it lacks today: alternatives of one kind written with `\|` join. 2.3: the rows of 2.4 above, and flat. 2.5 and 2.11: "fits through its form" at the result and at an argument. 2.7: a read sees flat. 2.8 and 2.9: the rows of 2.5 above. 2.13: `untagged-union`, 29 codes; the spans of `result-mismatch` and `arg-mismatch`. 2.14: the conformance row, and the map row of disjoint. 2.15: `anyOf` and `oneOf`. 2.16: the cases below. 2.17: the depth of a union. Section 11: the size. |
| `04` | A.3: the three rows of 2.7 above, with the worked counts. B.14: a union node holds the tag and a table from tag string to variant, and a variant lists its declared fields without the tag. Parts C and D: a note each. |
| `05` | A.6: `untagged-union` in the check catalog, and `bad-type` for a declared union with no tag. A.11: the cases. B.2: `Type` gains accessors for the tag and the variants, and its record accessors answer for flat. Nothing for A.13: `Conform` is the same walk. |
| `06` | No entry. `type(s)` is `"map"`. |
| `07` | 2.2: the union row gains the tag. 2.9: "two object branches join into one record" holds only when they form no tagged union. Section 3: the row for tagged unions points here. |
| Conformance suite | Below. |

**What the suite tests.** All in the formats of `03:2.16` and `05:A.11`.

- Type text: each example of "not tagged" in 2.3, in a declaration and after
  `as`; which key is the tag when two qualify; print cases; depth.
- The second file: `join`, `fits` (with whether it used any), `disjoint`,
  `without`, `truthy`, `falsy`, and a new `flat`, on the rows of 2.4.
- Check cases: every row of the tables of 2.5 and 2.6, with spans.
- Schema cases: branches that form a union, and each way they fail to.
- Conformance cases of the second kind: the table of 2.7, with path and steps,
  through `as`, `is` and the host's function.

```json
{"name": "tagged: a field of one variant, before a test",
 "source": "s.r", "env": "{s: Shape}", "types": [{"name": "Shape", "type":
   "{kind: \"circle\", r: float} | {kind: \"rect\", h: float, w: float}"}],
 "expect": {"phase": "check",
            "diagnostics": [{"code": "maybe-absent", "span": [1, 3, 1, 4]}]}}

{"name": "tagged: a list literal fits through its form",
 "source": "[{kind: \"circle\", r: 1}, {kind: \"rect\", w: 2}]",
 "expected": "Shape[]", "types": "as above",
 "expect": {"phase": "check", "diagnostics": [
   {"code": "result-mismatch", "span": [1, 26, 1, 46]}]}}

{"name": "tagged: a union of records with no tag",
 "source": "blob as {a: int} | {b: string}", "env": "{blob: any}",
 "expect": {"phase": "check",
            "diagnostics": [{"code": "untagged-union", "span": [1, 9, 1, 31]}]}}

{"name": "conform: the tag is read first, then one variant",
 "types": "as above",
 "conform": {"type": "Shape", "value": {"kind": "rect", "w": 2}},
 "expect": {"conforms": false, "path": ["h"], "steps": 3}}
```

`"as above"` stands for the `types` list of the first case. The step count is
the one 2.7 gives.

## 5. Size (I)

| Part | Lines per implementation, without tests |
| --- | ---: |
| Types: variants in the map slot, flat, the rows of 2.4, printing | 110 |
| Type text: the test for a tag, in the core with the parser | 40 |
| hit, all, and the rows for `==`, `!=` and `match` | 60 |
| Fits through a form (2.6) | 60 |
| JSON Schema | 15 |
| The walk: a union node, tag first, in the evaluator | 50 |
| Total | about 335 |

The checker goes from about 2,530 lines to about 2,800 (`03`, section 11). I
expect about 80 new cases. None of this was built.

| Implementation | What it costs beyond the lines |
| --- | --- |
| All three | Every evaluator carries the test for a tag and the union node, because the type text parser is in the core (`07`, section 4). The checker's part comes with each checker. |
| Go | A plan for a struct walked against a union is keyed by the variant as well (`04:B.14`). `TypeOf` cannot derive a tagged union: Go has no such type. The host writes type text or a schema. |
| TypeScript | The tag is one own-property read, and the table is a `Map`. A TypeScript union type is erased, so the host writes type text, as for every type (`05`, Part C). |
| Rust | The union node is one more case of the test enum (`04`, Part D). The path and the depth are as for a record. |

## 6. Prior art

| Language or library | What it does | What I take from it |
| --- | --- | --- |
| TypeScript (C) | "When every type in a union contains a common property with literal types", the union is discriminated, and `===`, `!==` and `switch` on that property narrow. A property that only some members have is an error before narrowing. `in` narrows too. A union of object types needs no tag. | The type text, the tests that narrow, and an error on `s.r`. Not the untagged union, and not `in`. `s?.r` gives null here and is still an error there. |
| Zod (C) | `z.discriminatedUnion("status", [...])` names the key. A plain union will "check the input against each option in order and return the first one that passes", which "can be slow for large unions". | Tag first, then one variant. The key is found here, not named. |
| Rust serde (C) | Four forms. Internally tagged, `#[serde(tag = "type")]`, puts the tag beside the fields in one object. Untagged tries each variant in order and returns "the first one that deserializes successfully". | The internally tagged form is the one this plan has. It is also what a derive would map, later. |
| OpenAPI 3.1 (C) | `discriminator` has `propertyName` and `mapping`, is legal only beside `oneOf`, `anyOf` or `allOf`, and may act as a hint that shortcuts validation and the selection of the matching schema. | It is a hint over what the branches say, so 2.8 reads the branches. |
| JSON Schema | `oneOf` with a `const` property in each branch is the usual way to write this (from memory). | 2.8. |
| Pydantic | A discriminated union names a field whose type in each model is a literal (from memory). | The same shape again. |
| Rust, Swift, Kotlin, OCaml | Sum types with named constructors (from memory). | Not this. A variant here is a plain map, so JSON goes in and out with no mapping. |

## 7. Alternatives rejected

| Alternative | Why not |
| --- | --- |
| Untagged unions of records, tested one after another | The first failure and the step count would depend on the order and the number of the variants. A value could conform to two. Narrowing would need a test of presence. It is how Zod's plain union and serde's untagged form work (C). |
| A tag named in the type text | New grammar that TypeScript does not have, for something the variants already say (09-1). |
| Join makes a tagged union when two records form one | A `String` node has the type of its one string (`03:2.5`). So `[{name: "a", role: "x"}, {name: "b", role: "y"}]` would have one variant for each element: a type that grows with the data, where today's one record is what the author means. |
| Join keeps a union when one side surely fits the other | `[...shapes, a, b]` would be a `Shape[]` and `[a, b, ...shapes]` would not, because `a` and `b` join first. Join would stop being associative, and every rule that joins several types would need an order that three checkers share. |
| `no-field` for a field that not every variant has, as in TypeScript | A second rule for reads. flat gives `maybe-absent` for `.` already, and the answer for `?.` is sound. |
| A fact "this path equals this string" that also narrows a plain literal set | Wider than asked, and `is` does it today. |
| Expected types pushed into every literal | `03` removed that in an earlier revision (its section 11). It changes inferred types. 2.6 changes none, and runs only where today's fit fails. |
| Reading `discriminator` | A hint, and its `mapping` renames tag values, which would need a rule of its own. |
| A new kind of value with a constructor, `Circle(r)` | A seventh kind in `02:2.4`, and a mapping to and from JSON in every binding. |

## 8. Decisions for the owner

| # | Decision | Recommendation | Strongest alternative |
| --- | --- | --- | --- |
| 09-1 | How is the tag known? | Found from the variants: the least key by code point that qualifies. No syntax. | Name it in the type text. One more form to learn, and a second way to be wrong. |
| 09-2 | A union of records with no tag, in type text | An error: `bad-type` in a declaration, the new check code `untagged-union` after `as` or `is`. 29 codes. | Join the records silently, as today. No new code, and the author never learns why `s.r` may be absent. |
| 09-3 | Does Join ever make a tagged union? | No. Only a written type does. A tagged union survives a Join only with itself. | Keep it when one side surely fits the other (section 7). I would take this next. |
| 09-4 | A literal against a written type | The fit goes through `List`, `Map`, `Cond`, `Match` and `Block`, at the program's expected result and at an argument. The error is at the element. | Nothing: the author writes `as Shape[]`, the test runs, and a wrong element is a run error with a path. Smaller, and a config file loses its check errors. |
| 09-5 | A field of only some variants, before a test | Read through flat: `maybe-absent` with `.`, `T \| null` with `?.`. No new rule. | `no-field` for both, as in TypeScript. |
| 09-6 | What narrows | `p.f == L`, `p.f != L`, `match p.f` and `p is T`, through the two kinds of fact `is` has. Any field may be f. | The tag only. One sentence shorter, and it needs "the tag of a type as narrowed", which can differ from the tag as written. |
| 09-7 | The walk | The tag first, then the variant with no second lookup of the tag. A value that conforms costs what its variant costs. | The tag first, then the variant as a plain record, tag included. The record walk is reused unchanged, and every value costs one step more than its variant does. |
| 09-8 | JSON Schema | `anyOf` and `oneOf` give a tagged union when the branches form one, else the Join. `discriminator` is ignored. | An error for object branches with no tag. It would refuse schemas that load today. |

## 9. Open questions, and where the documents disagree

- **The cost of 09-3.** A value built from different variants in different
  branches has the joined record type: `shapes.map(x => x.kind == "circle" ?
  {...x, r: 2} : x)` is a list of flat(`Shape`), and it does not fit `Shape[]`.
  The author writes `as Shape[]`, which is a test with a cost and a `value`
  site. 2.6 covers literals only. Annotations would cover the rest without a
  run-time test. This is the largest hazard of the plan.
- **2.6 reaches past tagged unions.** It moves the spans of `result-mismatch`
  and `arg-mismatch` inward for every program, and it brings back a small part
  of what `03` section 11 lists as removed ("expected types in literals").
  `03:2.13` says `result-mismatch` is on the result expression. The brief for
  plan `12` says a wrong field is already a check error with a position. It is
  not: the position is the whole result until 2.6 is applied.
- **`03:2.2` does not say what `int | float` or two records mean in type text.**
  `07` section 3 says two records join. 2.3 above needs the sentence.
- **`p is T` takes T as written** (`03`, open question 8). `s is {kind:
  "circle", ...}` then forgets `r`. A tag test does not, so the help for that
  form should name it.
- **Two tags.** When two keys qualify, the walk reads the lesser. An author
  cannot choose. I know no case where it matters beyond the path of a failure.
- **Type size.** A union of n variants holds n records and one flat record.
  `03`, open question 6, already asks for types to be shared and not copied.
- **Is `untagged-union` worth a code?** It could be `bad-type` in both phases,
  if `05` lets a `declare` code be reported for a program.
- The (from memory) rows of section 6 are not yet checked.
