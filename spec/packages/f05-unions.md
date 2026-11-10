# F5. Tagged unions

This chapter is normative. It specifies follow-on package F5. It adds to Core
and to packages F1 and F4:

1. The tagged union: two or more record alternatives of one type, told apart by
   a key that each requires with its own strings (2).
2. Type text that writes one, and the error for records that form none (3).
3. The rows of Join, Fits, Merge, Truthy, Falsy and the helpers for a map
   alternative that is a tagged union (4).
4. Reads of a union, and narrowing by a test of a field (5).
5. A row of "fits through its form" for a map literal against a union (6).
6. The conformance of a value to a union, and the walk of `as`, `is` and the
   conformance function over it (7).
7. The tagged union that the branches of `anyOf` and `oneOf` give (8).

It adds no syntax, no node, no reserved word, no run code, no check code, no
`declare` code, no message template, no library entry and no case field. It
reports the check code `untagged-union` of package F4 only for records that form
no tagged union.

The chapters of Core are cited by file and section: `05-types.md` 3.9 is section
3.9 of `spec/core/05-types.md`. F1 is `spec/packages/f01-host-types.md` and F4
is `spec/packages/f04-conversion.md`. Section numbers without a file are of this
chapter.

## 1. Conventions

### 1.1 Terms

| Term | Meaning |
| --- | --- |
| **Record** | A record of `05-types.md` 2.2: fields and a rest. `map<T>` is a record |
| **Tag** | A key that tells some records apart (2.1) |
| **Tag set** | The literal set that types the tag in one record (2.1) |
| **Tagged union**, **union** | Two or more records that have a tag, held as one map alternative (2.2) |
| **Variant** | One record of a union |
| **The variants of a map alternative** | Of a union, its variants; of a record, that record alone |
| **flat(U)** | The one record that a union joins to (2.3) |
| **Same by structure** | 2.5 |
| **Path, fact, facts in force** | `05-types.md` 10.3, 10.4 and 6.2, with the two kinds of fact of F4 6.4 |
| **Surely fits, disjoint, without** | F4 6.2, with the rows of 4.5 |

### 1.2 Requirements

1. An implementation that offers F5 offers every section of this chapter.
   Packages are offered in the order F3, F1, F2, F4, F5 and on, so it also
   offers the type checker, F1 and F4.
2. A compile with the static rules alone resolves a program type as F4 1.2 item
   2 says. It forms the unions of 3, makes the test of 3.2, and runs the walk of
   7. It applies no rule of 4 to 6.

### 1.3 Version

F5 is part of version `3.0`. It adds no version number. In an environment of
version `3.0` every rule of this chapter holds.

### 1.4 What this package changes in earlier rules

| Earlier rule | With this package |
| --- | --- |
| `05-types.md` 2.1, 2.3 | The map alternative is a record or a tagged union (2.2) |
| `05-types.md` 2.5, 20.7 | The depth and the size of a union (2.6) |
| `05-types.md` 3.4, last row | Records that form a tagged union are its variants. Records that form none stay an error (3) |
| `05-types.md` 3.9 | Gains the printing of a union (2.4) |
| `05-types.md` 4.1 | Gains the map rows of 4.1 |
| `05-types.md` 4.2 | Gains the map row of 4.2 |
| `05-types.md` 4.3, 4.5 (record of T), 7.5, 7.7, 12.2, 13.3, 13.4 | The record of a union is flat(U) (4.3). bind gains a row (4.3) |
| `05-types.md` 4.4 | Gains the row of a union (4.4) |
| `05-types.md` 4.5 | has-opaque and has-any look into each variant. flat, Unite, hit and all are added (4.5) |
| `05-types.md` 9.2 | A read of a union is a read of flat(U) (5.1) |
| `05-types.md` 10.5, 11.2 | Gain the facts of a test of a field (5.2) and of a `match` on a field (5.3) |
| `05-types.md` 15.1 | Gains the row of 6 |
| `05-types.md` 16.2 | `untagged-union` is reported for records that form no union (3.2) |
| `05-types.md` 18.1 | Gains the row of 7.1 |
| `05-types.md` 19.5, the row "Two or more record alternatives in one `Type`" | Reads "that form no tagged union" (3.2) |
| F1 3.6 item 2, Pick | Step 2 meets a record with a union (8.3) |
| F1 3.9, 3.11 | Replaced by 8.1. The error of F1 3.9 is gone |
| F1 4, `closed` | Applies to each variant; its statement names the variant (8.2) |
| F4 3.2, 3.5 step 4, 6.1 | Records that form a union are accepted (3.2) |
| F4 4.2 step 8, 4.5 | Gains the rows of 7.2 |
| F4 6.2, disjoint and without | Gain the rows of 4.5 |
| F4 6.4 item 4 | Two conforms facts combine by Unite, not by Join (5.4) |

## 2. The tagged union

### 2.1 A tag

Let R1, ..., Rn be records, n at least 2.

1. A key k is a **tag** of them when all three hold:
   1. Each Ri lists k as a required field.
   2. The type of that field in each Ri is a literal set and nothing else: no
      other alternative, not `null`, not `any`, not `never`. That literal set
      is the **tag set** of Ri.
   3. No string is in the tag sets of two of them.
2. The records **form a tagged union** when they have a tag. **The tag** of the
   union is the least such key, by code point (`01-language.md` 6.7 item 5).
3. The tag is found from the records. It is never written.
4. Every other part of a variant is free. A variant may be exact or open, may
   have a typed rest, optional fields, and fields whose types are unions or
   lists of unions. Its tag set may hold several strings.
5. Two records that are the same by structure never form a union: their tag sets
   would share every string.
6. The tag is a function of the set of records. Two or more of the variants of a
   union form a union too, since the tag of the whole is a tag of them. Their
   tag may be a lesser key: a key under which two removed variants shared a
   string may be a tag of the variants left.

| Records | Tag |
| --- | --- |
| `{kind: "circle", r: float}`, `{kind: "rect", h: float, w: float}` | `kind` |
| `{kind: "a" \| "b", x: int}`, `{kind: "c"}` | `kind`. The first tag set has two strings |
| `{a: "x", b: "q"}`, `{a: "y", b: "p"}` | `a`. `b` qualifies too, and `a` is less |
| `{a: "x", k: "1"}`, `{a: "x", k: "2"}`, `{a: "y", k: "3"}` | `k`. `a` does not qualify: two records share `"x"`. The last two alone have the tag `a` |
| `{kind: "a", ...}`, `{kind: "b", x: int}` | `kind`. An open record may be a variant |
| `{a: int}`, `{b: string}` | None: no key in both |
| `{kind: string}`, `{kind: "a"}` | None: `string` is not a literal set |
| `{kind: "a"}`, `{kind?: "b"}` | None: not required in the second |
| `{kind: "a" \| "b"}`, `{kind: "b"}` | None: `"b"` is in both |
| `{kind: "a" \| null}`, `{kind: "b"}` | None: the first type has `null` |
| `{kind: "a"}`, `map<int>` | None: `map<int>` lists no field |

### 2.2 The union in a type

1. The map alternative of a type (`05-types.md` 2.1, 2.3) is **one record, or
   one tagged union**. A type still has at most one alternative of each kind.
   The other alternatives stand beside a union as beside a record. So the type
   `{kind: "a"} | {kind: "b"} | null` has a map alternative and a null
   alternative.
2. A union is held as the set of its variants. Their order means nothing.
3. A union has at least two variants. Where a rule leaves one variant, the map
   alternative is that record. Where it leaves none, the type has no map
   alternative.
4. A union may stand wherever a type stands: as the type of a field, of a rest,
   of the elements of a list, inside another union's variant. There are no
   recursive types (F4 3.1 item 3).

### 2.3 flat

**flat(U)** of a union U is the Join of its variants, taken as records by the
last row of `05-types.md` 4.1. flat of a record is the record.

1. Every value that conforms to U conforms to flat(U).
2. The tag is required in flat(U), with the literal set of every string of every
   tag set: the **tag type** of U.

flat(`{kind: "circle", r: float} | {kind: "rect", h: float, w: float}`) is `{h?:
float, kind: "circle" | "rect", r?: float, w?: float}`.

### 2.4 Printing

These rows join `05-types.md` 3.9.

| Part | Printed as |
| --- | --- |
| A union | Its variants, each printed as a record, joined with ` \| `, in code point order of the least string of each variant's tag set. They stand where the map alternative stands among the alternatives: after the list, before the opaque names and `null` |
| A list of a union | In parentheses, by the rule for a ` \| ` at the top level: `({kind: "a"} \| {kind: "b"})[]` |
| A union as a field's type, a rest or the type of `map<T>` | With no parentheses: `{s: {kind: "a"} \| {kind: "b"}}` |

1. The printed text reads back as the same type. Its alternatives are the
   variants, and the same records form the same union with the same tag.
2. The tag sets share no string, so the least strings are all different and the
   order is total.
3. So `{kind: "circle", r: float} | {kind: "rect", h: float, w: float} | null`
   prints as `{kind: "circle", r: float} | {h: float, kind: "rect", w: float} |
   null`, and `{a: "x", b: "q"} | {a: "y", b: "p"}` prints in that order,
   because its tag is `a`.

### 2.5 Same by structure

Two types are **the same by structure** when they have the same alternatives;
their element types are the same by structure; and their map alternatives have
the same variants: for each record of one, a record of the other with the same
keys, the same marks, field types the same by structure, and a rest the same by
structure. The order of variants does not matter.

1. The Join of 4.1 uses it. So do F4 6.4 item 3 (two facts are the same) and
   `03-host.md` 3.10.6 (a case's `type` is compared by structure).

### 2.6 Depth and size

| Measure | Of a union |
| --- | --- |
| Depth (`05-types.md` 2.5) | 1 more than the deepest field or rest type of any variant: the depth of its deepest variant. A union adds no level of its own |
| Size (`05-types.md` 20.7) | The sum of the sizes of its variants, each counted as a record. flat(U) is not counted |

1. The limits are unchanged: 64 for depth, 100,000 for size.
2. A union of two variants `{kind: "a", v: T}` and `{kind: "b", v: T}`, with T
   of size s, has size 2 × (s + 2): 1 for each record, 1 for each tag set of one
   string, and s for each `v`.

## 3. Type text

### 3.1 Records of one Type

These rules replace the last row of `05-types.md` 3.4 and step 4 of F4 3.5. They
hold for declared type text and for a program type alike.

1. After names are replaced, take the **records of a `Type`**: each of its
   alternatives that is a record, `map<T>` included; each variant of an
   alternative that is a named type whose type has a union; and the records of
   an alternative in parentheses, by this rule applied to the `Type` inside
   them. A named type that is a record counts as one record.
2. One record: it is the map alternative, as in Core.
3. Two or more records that form a tagged union (2.1): the union of them is the
   map alternative.
4. Two or more records that form none: an error (3.2).
5. The other alternatives join by `05-types.md` 3.4, as before. So `({kind:
   "a"})[] | ({kind: "b"})[]` is a list whose element is the Join of the two
   records: `{kind: "a" | "b"}[]`. A Join never makes a union (4.1).
6. A named type that is a union gives its variants: with `Shape` a union of two,
   `Shape | {kind: "tri", a: float}` has three, and `Shape | Shape` has four,
   which share their strings, and form none.

### 3.2 Where the test is made, and the error

1. The test of 3.1 is made at each `Type` of the text, from the innermost out. A
   `Type` whose records form no union is an error. A `Type` that holds it, in
   parentheses or by a name, is not tested again.
2. In declared type text the error is `bad-type`, or `bad-signature` in a
   signature or a method's text (`05-types.md` 19.5). In a named type it is
   `bad-type` (F4 3.2).
3. In a program type it is `untagged-union`, a static rule (F4 6.1), on the
   innermost `PType` whose records form no union, from its first token to its
   last. Each such `PType` is reported once. A `PType` that holds one is not
   tested for depth or size.
4. Records that form a union are no error anywhere.
5. A schema never gives this error (8.1). A binding derives no union from a host
   type (9).

| Text | Result |
| --- | --- |
| `{kind: "a", x: int} \| {kind: "b"}` | A union of two |
| `{kind: "a"} \| {kind: "b"} \| null` | A union, and `null` |
| `({kind: "a"} \| {kind: "b"}) \| {kind: "c"}` | A union of three |
| `({kind: "a"} \| {kind: "b"}) \| {kind: "a"}` | An error at the whole text: three records share `"a"`. The inner `Type` was a union |
| `({kind: "a"} \| {kind: "a"}) \| {kind: "b"}` | An error at the inner `Type` only |
| `{a: int} \| {b: string}`, `{kind: string} \| {kind: "a"}`, `{kind: "a"} \| {kind?: "b"}`, `{kind: "a" \| "b"} \| {kind: "b"}`, `{kind: "a"} \| map<int>` | An error, as in Core |
| `Circle \| Rect`, two named records with a tag | A union of two |

## 4. Operations on types

In this section a record counts as a union of one variant wherever a rule speaks
of variants.

### 4.1 Join

This row replaces the map row of `05-types.md` 4.1 when either side's map
alternative is a union.

| Case | Join(A, B) |
| --- | --- |
| map, in both, and either is a union | The union, if the two map alternatives are the same by structure (2.5). Otherwise Join(flat(A), flat(B)) by the record row |

1. **Join never makes a union.** Two records join into one record, by Core's
   row, whatever their fields.
2. Join stays commutative and associative, and Join(T, `never`) is T. Once two
   map alternatives differ, the result is the record Join of their flats, and
   that Join is commutative, associative and idempotent.
3. So the Join of a union with one of its own variants is flat(U) joined with
   that variant, which is flat(U).

| A | B | Join |
| --- | --- | --- |
| `Shape` | `Shape` | `Shape` |
| `Shape` | `{kind: "circle", r: int}` | `{h?: float, kind: "circle" \| "rect", r?: float, w?: float}` |
| `{kind: "circle", r: float}` | `{kind: "rect", h: float, w: float}` | `{h?: float, kind: "circle" \| "rect", r?: float, w?: float}`: two records, as in Core |
| `Shape \| null` | `int` | `int \| Shape \| null`, printed with the union where the record stands |

Here and below, `Shape` is `{kind: "circle", r: float} | {kind: "rect", h:
float, w: float}`.

### 4.2 Fits and surely fits

This row replaces the map row of `05-types.md` 4.2.

| Case | Fits when |
| --- | --- |
| map: S's map alternative against T's | Each variant of S fits some variant of T by Core's record row |

1. The fit **uses any** when some variant of S fits no variant of T without
   using any. So S surely fits T (F4 6.2) when each variant of S surely fits
   some variant of T. When a variant of S fits several variants of T, the fit
   uses any only when it fits none of them without using any.

| S | T | Fits |
| --- | --- | --- |
| `{kind: "circle", r: int}` | `Shape` | Yes, surely: it fits the first variant |
| `flat(Shape)` | `Shape` | No: its `kind` is in no one tag set |
| `Shape` | `{kind: string, ...}` | Yes, surely: each variant fits |
| `Shape` | `{kind: "circle", r: float}` | No: the second variant does not fit |
| `{kind: "circle", r: any}` | `Shape` | Yes, by a fit that uses any |

### 4.3 The record of a type

The **record of** a type whose map alternative is a union is flat(U)
(`05-types.md` 4.5). So these see flat(U), with no rule added:

1. Merge, a spread in a map literal (`05-types.md` 4.3, 7.5, 7.7): `{...s, id:
   1}` with `s: Shape` is `{h?: float, id: int, kind: "circle" | "rect", r?:
   float, w?: float}`.
2. A map pattern and its rest (12.2): `const {kind, ...rest} = s` gives `rest`
   the type `{h?: float, r?: float, w?: float}`.
3. A method call (13.3). A union has the map methods. The receiver is fitted to
   its `Recv` by 4.2 and bound by the rows below.
4. bind (13.4). "A has a record" holds when A's map alternative is a union, and
   A's record is flat(A). When P's map alternative is a union, the row "P is
   another record" is applied to each variant of P in turn, against A's record.

### 4.4 Truthy and Falsy

| Alternative | Truthy part | Falsy part |
| --- | --- | --- |
| A union | The union | – |

Every variant requires its tag, so no value of a union is the empty map. A union
alone in a tested position is `always-truthy`, when the class
`constant-condition` is rejected (`05-types.md` 10.2).

### 4.5 The helpers

| Helper | With this package |
| --- | --- |
| overlap, nonnull, finite | Unchanged: they look at kinds only |
| has-opaque(T), has-any(T) | Look into the field and rest types of every variant |
| flat(U) | 2.3 |
| surely fits | 4.2 |
| disjoint(A, B), map in both | They share a value unless every variant of A is disjoint from every variant of B by the record row of F4 6.2 |
| without(X, T) | The map alternative of X loses each variant that surely fits T. One variant left is a record. None left: X has no map alternative. The other rows of F4 6.2 are unchanged |
| Unite(A, B) | Join(A, B), except for the map alternative: take the variants of A's map alternative and of B's, dropping each that is the same by structure as one taken before. When two or more are left and they form a tagged union (2.1), the map alternative is that union. Otherwise it is the map alternative of Join(A, B) |
| hit(M, f, S) | For a map alternative M, a key f and a set S of strings: the variants of M in which the type of f is not disjoint from the literal set of S |
| all(M, f, S) | The variants of M that list f as a required field whose type surely fits the literal set of S |

1. **The type of a key in a record** is its field's type when the record lists
   the key, else the record's rest (F4 6.2).
2. For a list of variants V, **the type of V** is the type whose only
   alternative is a map alternative: the record when V has one, the union of V
   when it has two or more (they form one by 2.1 item 6), and `never` when it
   has none.
3. Unite(A, B) is a type every value of A and every value of B conforms to: a
   map that conforms to a record of A or of B conforms to that record, and so to
   the union, whose tag sets share no string.

| X | T | without(X, T) |
| --- | --- | --- |
| `Shape` | `{kind: "circle", r: float}` | `{h: float, kind: "rect", w: float}` |
| `Shape \| null` | `Shape` | `null` |
| `Shape \| null` | `{kind: "circle", ...}` | `{h: float, kind: "rect", w: float} \| null` |

| A | B | disjoint |
| --- | --- | --- |
| `Shape` | `{kind: "tri", ...}` | Yes: `kind` is required and the sets share nothing, for each variant |
| `Shape` | `{kind: "circle", ...}` | No |
| `Shape` | `{r: string, ...}` | Yes: `r` is required in B. Its type is `float` in the first variant and `never` in the second, whose rest is `never`, and each is disjoint from `string` |

## 5. Reads and narrowing

### 5.1 Reads

**A read of a receiver whose map alternative is a union is a read of flat(U)**,
by `05-types.md` 9.2: a field step, a `String` key, a computed key, a map
pattern's field, with `.` or with `?.`. No rule is added. A key that a fact says
is present reads as required, as in Core.

| Read, `s: Shape` | Type and diagnostic |
| --- | --- |
| `s.kind` | `"circle" \| "rect"` |
| `s.r` | `float`, and `maybe-absent` on `r`: only some variants list it |
| `s?.r` | `float \| null` |
| `s.color` | `no-field` on `color`: no variant lists it, and every variant is exact |
| `s.color`, with one variant open | `any`, and a `dynamic` site on `s` |
| `s[k]`, `k: string` | `maybe-absent` on `k`; `float \| "circle" \| "rect"` |
| `const {r = 0} = s` | `r: float` |

### 5.2 A test of a field

These rows join the table of `05-types.md` 10.5.

| c | T(c) | F(c) |
| --- | --- | --- |
| `q == L`, `L == q` | {p: H} and {p} | {p: not A} |
| `q != L`, `L != q` | {p: not A} | {p: H} and {p} |
| `q in [L1, ..., Ln]` | {p: H} and {p} | {p: not A} |
| `q not in [L1, ..., Ln]` | {p: not A} | {p: H} and {p} |

1. **The forms.** q is a path (`05-types.md` 10.3) with at least one step. Its
   last step is `.f`, `?.f`, `["f"]` or `?.["f"]`, which names the key f. p is q
   without that step: an `Ident`, or a shorter path. L is a `String` node. The
   right operand of `in` and `not in` is a `List` node of one or more items,
   each a `String` node, with no spread. S is the set of strings of the test:
   {L}, or the strings L1 to Ln.
2. **`in` and `==`.** Where `q in [L1, ..., Ln]` held, its fact is the one the
   `||` of the tests `q == Li` gives (5.4): hit(M, f, S) holds the variants of
   each hit. Where it failed, its fact {p: not A} removes, as the `||` does,
   each variant whose type of f surely fits one Li, and also each whose type of
   f surely fits S without fitting one string: with `t: {kind: "a" | "b", x:
   int} | {kind: "c"}`, `t.kind in ["a", "b"]` failing leaves `{kind: "c"}`,
   where the `||` of two `==` tests leaves both. The rows of the table are the
   rule; `not in` swaps the two sides.
3. **When it gives facts.** Let M be the map alternative of the type of p as
   narrowed where the test stands. The test gives facts when M is a union; or
   when M is a record and a fact that a test of this section or of 5.3 gave
   about p is in force there. Otherwise it gives none, as in Core.
4. **H** is the type of hit(M, f, S), and **A** the type of all(M, f, S) (4.5
   item 2). The facts {p: H} and {p: not A} are the facts "conforms" and "does
   not conform" of F4 6.4, and have the effects it gives: p then has H, unless
   the type of p already surely fits H; and without(the type of p, A).
5. **{p}** is the fact "not null" of `05-types.md` 10.4, with its prefix rule.
   It stands beside {p: H}: where `q == L` or `q in [...]` held, p was a map.
6. **A fact with no variant** is not given. With hit empty, the test gives no
   {p: H} and no {p}. With all empty, it gives no {p: not A}.
7. **Any field narrows,** not only the tag. A field that only some variants
   list, or that one variant types `string`, narrows as the rows say.
8. **Diagnostics are Core's.** `q in [...]` compares q with the list's element
   type as `==` does (`05-types.md` 8.4): `s.kind in ["tri"]` is `never-equal`
   on the whole `Binary`, and gives no fact, since hit is empty. `s.kind in
   ["tri", "circle"]` reports nothing and narrows to the circle variant.

| Written, `s: Shape` | Type |
| --- | --- |
| `s.kind == "circle" ? s.r : s.w * s.h` | `float` |
| `if (s.kind != "rect") { s.r } else { s.h }` | `float` |
| `s.kind == "tri"` | `never-equal` on the whole `Binary` (`05-types.md` 8.2), and no fact |
| `"circle" == s.kind ? s.r : 0` | `float` |
| `s.kind in ["circle"] ? s.r : s.w` | `float` |
| `s["kind"] == "circle" ? s.r : 0` | `float` |

| Written, with `t: {kind: "a" \| "b", x: int} \| {kind: "c"}` | Facts |
| --- | --- |
| `t.kind == "a"` | True: {t: `{kind: "a" \| "b", x: int}`}. False: none, since `"a" \| "b"` does not surely fit `"a"` |
| `t.kind == "c"` | True: {t: `{kind: "c"}`}. False: {t: not `{kind: "c"}`}, so `t` is the first variant |

### 5.3 `match` on a field

When the subject of a `match` is a path q, as in 5.2 item 1, and the test of 5.2
item 3 holds for p where the `match` stands, the arms are checked under these
facts. Let M1 be M, and Mi the map alternative of the type of p as the facts of
the arms before arm i leave it.

1. **An arm whose literals are all strings,** S the strings of it that
   `05-types.md` 11.2 does not ignore: its result is checked under {p: the type
   of hit(Mi, f, S)} and {p}.
2. **Every later arm,** `_` included, is checked under {p: not the type of
   all(Mi, f, S)} as well, with S the strings of arm i. The facts of the arms
   before it hold in order.
3. **An arm that also lists a literal that is not a string** gives its own
   result no fact by this section. The later arms still get the fact of item 2
   for its strings.
4. Item 6 of 5.2 holds: a fact with no variant is not given.
5. **Coverage is Core's.** The subject's type is a literal set, and
   `non-exhaustive` is decided by `05-types.md` 11.2. With a third variant in
   `Shape`, `match s.kind { "circle" => s.r, "rect" => s.w }` is
   `non-exhaustive`.
6. Core's rule for `null` stands: after an arm that lists `null`, {q} holds, and
   with it {p} by the prefix rule.

| Written, `s: Shape` | Type |
| --- | --- |
| `match s.kind { "circle" => s.r, "rect" => s.w }` | `float` |
| `match s.kind { "circle" => s.r, _ => s.h }` | `float`: under `_`, `s` is the second variant |
| `match t.kind { "a" => t.x, "b" => t.x, "c" => 0 }`, `t` of 5.2 | `int` |

### 5.4 Combining facts

The facts of 5.2 and 5.3 are of F4's two kinds, so `!`, `&&`, `||`, `? :` and an
`if` chain combine them by `05-types.md` 10.5 and F4 6.4. One rule changes:

1. **F4 6.4 item 4 reads Unite for Join.** In T(d || e), one conforms fact about
   a path p on each side, {p: A} and {p: B}, give the one fact {p: Unite(A, B)}.
   So `s.kind == "circle" || s.kind == "rect"` with three variants gives `s` the
   union of the first two, and `v is Circle || v is Rect` gives `v` the union of
   the two records. Without a tag, Unite is the Join, as before.
2. **The failing side** of `d || e` is F(d) ∪ F(e), applied in order, as before:
   each does-not-conform fact removes its variants in turn.
3. A test checked under earlier facts computes its M under them: in `d && e` and
   in the right operand of `d || e`, M is the map alternative as the facts of d
   leave it.

| Written, `u: {kind: "a", x: int} \| {kind: "b", y: int} \| {kind: "c"}` | Type |
| --- | --- |
| `u.kind == "a" \|\| u.kind == "b" ? u : null` | `{kind: "a", x: int} \| {kind: "b", y: int} \| null` |
| `u.kind == "a" \|\| u.kind == "b" ? 0 : u` | `int \| {kind: "c"}` |
| `u.kind != "a" && u.kind != "b" ? u : null` | `{kind: "c"} \| null` |

### 5.5 Paths, `?.` and null

1. p is any path: `s`, `cfg.shape`, `cfg?.shape`, `m["shape"]`, or a `const`
   name. An expression that is the path p, also as the receiver of a longer
   chain, has the narrowed type. So `cfg.shape.kind == "circle" ? cfg.shape.r :
   0` checks.
2. A fact about `cfg.shape` changes the type of no field of the record of `cfg`
   (F4 6.4 item 7). `const {shape} = cfg` reads `cfg`'s record, not the narrowed
   path.
3. With `p: Shape | null`, `p?.kind == "circle"` gives {p: the circle variant}
   and {p} where it held, so `p?.kind == "circle" ? p.r : 0` checks, and `p.r`
   reports no `maybe-null`. Where it failed, `p` keeps `null`.
4. `?.` on a union reads flat(U) with null added: `p?.r` is `float | null`.

### 5.6 What gives no fact

| Written | Why |
| --- | --- |
| `const k = s.kind; k == "circle"` | `k` is a path with no step: nothing names `s` |
| `s.kind == k`, `k` a name of type `"circle"` | L is not a `String` node |
| `"r" in s` | A test of a key gives no fact |
| `s.kind in ks`, `ks` a name of a list | The right operand is not a `List` node |
| `s.kind in ["a", ...more]`, `s.kind in ["a", k]` | A spread, or an item that is not a `String` node |
| `f(s).kind == "a"`, `xs[0].kind == "a"` | The receiver is not a path |
| `r.kind == "a"`, `r` a record that no test of 5.2 narrowed | M is a record and no fact of 5.2 is in force: as in Core |
| `s.kind < "d"` | Order gives no fact |

### 5.7 `filter` and `find`

The facts that reach the parameter x of a predicate (`05-types.md` 13.8, F4 6.5)
include the facts of 5.2 about x. With `shapes: Shape[]`:

| Written | Type |
| --- | --- |
| `shapes.filter(x => x.kind == "circle")` | `{kind: "circle", r: float}[]` |
| `shapes.find(x => x.kind != "circle")` | `{h: float, kind: "rect", w: float} \| null` |

### 5.8 Soundness

Each fact holds under the precondition of `05-types.md` 18.4: the value of p
conforms to the type of p as narrowed, by the precondition and the facts already
in force.

1. **{p: H} where `q == L` held.** q was read and gave the string L, so p is a
   map, and conforms to some variant v of M. Its value under f is L, which
   conforms to the type of f in v. So that type is not disjoint from {L}, and v
   is in hit(M, f, {L}).
2. **{p} beside it.** A null p gives null at `?.f`, which is not L, and fails at
   `.f`. So p is not null.
3. **{p: not A} where `q == L` failed.** A value that conforms to a variant in
   all(M, f, {L}) has f present with a value that conforms to a type that surely
   fits {L}: it is L. So p conforms to none of them, and without leaves the
   alternative it conforms to.
4. **`!=`** swaps the two sides. **`in`** held means q equals one of the strings
   S, and failed means it equals none, so items 1 to 3 hold with S for {L};
   **`not in`** swaps them. **`match`** is a chain of `==` tests on one value.
   So items 1 to 3 cover them all.
5. **Unite** is sound by 4.5 item 3.
6. **The promise** of `05-types.md` 18.4 holds with this package. Promise 1: a
   read of a union is a read of flat(U), and every value of U conforms to
   flat(U); a narrowed type is sound by items 1 to 5. Promise 4: a union is
   inferred only from a declared type, a program type or a fact, and each is
   sound for the value. Promises 2 and 3 are unchanged: no site and no class is
   added.

## 6. An expression against a written type

This row joins the table of `05-types.md` 15.1, after the row of a `Map`. It
uses Core's table "A map literal against a record" and its terms (what an entry
gives, the contributors of a key).

| e | It fits when | Place of a failure |
| --- | --- | --- |
| A `Map` with no computed key, where T's map alternative is a union U | Let t be U's tag. Some entry of e surely gives t, and the Join of the types the contributors of t give for it is a literal set within the tag set of one variant V. e then fits V by Core's table "A map literal against a record" | e, once, when no entry surely gives t. Each contributor of t, when that Join is not within one tag set: a written entry's value (a shorthand's name), a spread's operand. Otherwise the places Core's table gives for V |

1. As in Core, the row is tried only when the type of e does not fit T. A `Map`
   with a computed key is not followed.
2. **The variant is chosen by the tag only;** the other fields are then checked
   against V. The tag may come from a written entry, as in `{...common, kind:
   "tcp", port: 1}`, or from a spread whose operand's type gives it within one
   tag set, as in `{...tcpBase, port: 1}` with `tcpBase: {host: string, kind:
   "tcp"}`.
3. **Soundness.** Core's table is sound for a record (`05-types.md` 15.1): when
   e fits V by it, every value of e conforms to V, and so to U, of which V is a
   variant. The test of the tag only chooses V.
4. So a list literal against `Shape[]` is reported at its element that is wrong,
   and inside that element at the entry that is wrong.

| e, against `Shape[]` | Place of failure |
| --- | --- |
| `[{kind: "circle", r: 1}, {kind: "rect", w: 2}]` | The second map: it has no `h` |
| `[{kind: "oval", r: 1}]` | `"oval"` |
| `[{r: 1}]` | The map: it has no `kind` |
| `[{kind: "rect", w: 2, h: "3"}]` | `"3"` |
| `[c ? {kind: "circle", r: 1} : {kind: "rect", w: 1, h: 1}]` | None: each branch fits |
| `[{...s, r: 1}]`, `s: Shape` | `s`: it is the one contributor of `kind`, and gives `"circle" \| "rect"` |

## 7. Conformance and the walk

### 7.1 Conformance

This row joins the table of `05-types.md` 18.1.

| Type | A value conforms when |
| --- | --- |
| A tagged union | It is a map. It has the tag. The tag's value is a string in the tag set of one variant. It conforms to that variant |

The tag sets share no string, so there is never a choice of variant.

### 7.2 The walk

This row joins step 8 of F4 4.2. F4 4.3 is unchanged for a map alternative that
is one record.

| Map alternative | The walk |
| --- | --- |
| A tagged union U | The rows below |

| Step | Rule |
| --- | --- |
| 1. The tag | Let t be U's tag. Charge 1 + the size of t, whether or not v has the key (the key rule, `02-evaluation.md` 5.3 item 5). If v lacks t: failure at path + [t], template `missing`. Otherwise Test(v[t], the tag type of U, path + [t]), whose step 3 is this lookup. A failure is the answer |
| 2. The variant | The variant V whose tag set holds v[t]. There is exactly one |
| 3. The record | The rows of F4 4.3 for v and V, with two changes. Step 1 passes over the field t: it is not charged and not tested again. In step 2, t counts as a declared key that v has |

1. **A value that conforms** costs what it costs against V written alone: 1 for
   the map, 1 + the size of each declared key, the tag's among them, the size of
   the tag's value, and what the other fields' values cost.
2. **A value that fails** may cost a different count from V alone, because the
   tag is read first: a field whose key sorts before the tag's is looked up
   after it.
3. **Finding V** reads nothing beyond the tag's value. The test of that value
   against the tag type charges the size of the string once (F4 4.2 step 6),
   however many variants there are.
4. **Depth.** The walk enters the map once, at F4 4.2 step 2. The tag's value is
   tested against a literal set and is never entered.
5. **The failure.** A failure at the tag has `expected` the tag type: `expected
   "circle" | "rect", found "oval"`. A failure at the value itself, which is not
   a map, has `expected` the whole type, the union printed by 2.4. No template
   is added.
6. `as`, `is` and the conformance function (F4 7) share this walk.

### 7.3 Costs

No form is added, and no cost row. The walk's charges of F4 5.3 item 1 gain: for
a value tested against a union, 1 + the size of the tag for its lookup, which is
also the step of its value; the size of the tag's value when it is a string;
then the record rows of its variant, without the tag. The walk creates no value
and charges no unit.

### 7.4 Worked counts

| Value, against `Shape` | Conforms | Path | Steps |
| --- | --- | --- | --- |
| `{kind: "rect", w: 2, h: 3}` | yes | | 4: the map, `kind`, `h`, `w` |
| `{kind: "rect", w: 2}` | no, `missing` | `["h"]` | 3: the map, `kind`, `h` |
| `{kind: "oval", r: 1}`, `{kind: 7}` | no, `wrong` | `["kind"]` | 2: the map, `kind` |
| `{r: 1}` | no, `missing` | `["kind"]` | 2 |
| `{kind: "circle", r: "1"}` | no, `wrong` | `["r"]` | 3 |
| `{kind: "circle", r: 1, note: "x"}` | no, `extra` | `["note"]` | 4: the map, `kind`, `r`, and 1 for the one other key |
| `"circle"` | no, `wrong` | `[]` | 1 |
| A circle, then a rectangle, against `Shape[]` | yes | | 8: the list, 3, 4 |

| Value, against `{r: float, type: "circle"} \| {type: "rect", w: float}` | Conforms | Path | Steps |
| --- | --- | --- | --- |
| `{type: "circle", r: 1}` | yes | | 3, as against the first variant alone |
| `{type: "circle", r: "1"}` | no | `["r"]` | 3: the map, `type`, `r`. Against the first variant alone, 2 |

## 8. JSON Schema

### 8.1 Branches that form a union

These rules replace F1 3.9 and its row of F1 3.11.

1. The test is made at each `anyOf` and each `oneOf`, after its branches are
   mapped. A nested `anyOf` or `oneOf` is mapped first.
2. Take **the records of the branches**: of each branch type that has a map
   alternative, its variants (a record is one).
3. When two or more are taken and they form a tagged union (2.1), the part of F1
   3.6 is a type whose map alternative is that union, and whose other
   alternatives are the Join of the other alternatives of the branch types.
4. Otherwise the part is the Join of the branch types, by F1 3.6, as before. A
   branch that is a union is then flattened by 4.1.
5. A property is a tag when it is in `required` and its schema gives a literal
   set and nothing else: a `const` string, or an `enum` of strings only. A tag
   property outside `required`, typed `string`, or with `null` in its `enum`, is
   no tag. The branches join, and nothing is reported.
6. **A filled key is no tag.** A key that any of the records fills (F1 4.4), by
   `absent-is-null` or by `defaults-applied`, is no tag of them, whatever its
   type, Join(base, fill). The host fills a key the value lacked, so it would
   have to fill the tag before any variant could be chosen.
7. `discriminator` is ignored, as F1 3.12 says. The tag is found from the
   branches.
8. **Soundness.** A value valid by the `anyOf` or `oneOf` is valid by some
   branch i, and has the tag, which no record fills. If it is a map, it conforms
   to the record of branch i (F1 2.3), so its tag's value is in that record's
   tag set, which no other record shares: it conforms to the union. Any other
   value conforms to the joined other alternatives.

| Branches | Type |
| --- | --- |
| Two object branches, `kind` a `const` and required in each: `"circle"` with `r` required, `"rect"` with `w` required | `{kind: "circle", r: float, ...} \| {kind: "rect", w: float, ...}` |
| The same two, and a third branch `{"type": "null"}` | The same union, and `null` |
| A nested `anyOf` of two records that share `t: "a"`, beside a branch with `t: "b"` | The nested records join to one record, which forms a union with the outer one |
| `kind` outside `required` in one branch | The Join, as F1 3.9 gave it |

### 8.2 The options

1. The options of F1 4 apply to the object schema of each branch as to any
   object schema, and a field of a variant keeps its base and its fill apart (F1
   4.4). The test of 8.1 is made on the records as the options map them: a
   `kind` outside `required` is filled, by `null` or by its `default`, and is no
   tag (8.1 item 6).
2. **`closed`.** A variant keeps the mark *closable* of F1 4.1. Close (F1 4.1
   item 3) closes each variant of a union, at every depth. So under `closed`
   each variant whose schema has `properties` and neither `additionalProperties`
   nor `patternProperties` is exact.
3. A Join of a union with another map alternative is closable when every record
   joined is closable (F1 3.6 item 8).
4. The statements of the options read, for an object whose type is a union, by
   the variant whose tag set holds its tag's value: the host has removed from it
   each key that variant does not list, when Close made that variant exact
   (`closed`), and has filled each key that variant marks as filled and the
   object lacked (`absent-is-null`, `defaults-applied`, F1 4.4). The tag itself
   is never filled (8.1 item 6).

### 8.3 Pick, and the meet of a record with a union

This replaces step 2 of Pick (F1 3.6 item 2). Steps 1 and 3 are unchanged.

1. **Step 2.** If every type left has a map alternative and nothing else, a
   record or a union, the result is their meet, folded from the left: Meet(T1,
   T2), then its meet with T3, and so on.
2. **Meet of two records** is F1's Meet (F1 3.6 item 3).
3. **Meet of a record R and a union U**, in either order, is the list of the
   meets of R with each variant V of U: Meet(R, V) when R stands first, Meet(V,
   R) when U stands first. When the records of that list form a tagged union
   (2.1), the meet is that union. Otherwise the meet fails.
4. **Meet of two unions** fails.
5. **When a meet fails,** at any step of the fold, Pick applies step 3 to the
   whole list: the first type that fits every other by a fit that does not use
   any, else the first type.
6. **Marks.** Meet(R, V) is closable when R and V both are (F1 3.6 item 8).
   Close, applied once at the root, then closes each variant (8.2).
7. **Soundness.** A value valid by both parts is valid by R and by one branch of
   U, so it conforms to R and to the variant V of that branch (F1 2.3). By F1's
   argument (F1 3.6 item 4, and F1 4.4 item 5 for a key the host filled) it
   conforms to Meet(R, V). Each Meet(R, V) types the tag by a literal set alone,
   its tag set, and the tag sets share no string, so the value's tag picks
   Meet(R, V), and it conforms to the union. Under `closed`, the argument of F1
   4.1 item 8 holds variant by variant.
8. The tag of the meet is found again (2.1), reading each field's type as
   Join(base, fill) (F1 4.4). A record part that types the tag key narrows each
   tag set by Pick, and the meet fails when the narrowed sets share a string or
   one is not a literal set. A tag key that a fill reaches, from either side, is
   no tag (8.1 item 6), so that meet fails too, and step 3 decides.

| `allOf` of | Type |
| --- | --- |
| A `$ref` to `{"type": "object", "properties": {"id": {"type": "integer"}}, "required": ["id"]}`, and a `oneOf` of the circle and rectangle objects of 8.1 | `{id: int, kind: "circle", r: float, ...} \| {h: float, id: int, kind: "rect", w: float, ...}` |
| The same, with the `oneOf` first | The same union |
| A record with `kind` of type `string` required, and the same `oneOf` | The same union without `id`: Pick(`string`, `"circle"`) is `"circle"` |
| A record with `kind` an `enum` of `"a"` and `"b"`, and a `oneOf` with tags `"a"`, `"b"` and `"c"` | The meets type the third tag `"a" \| "b"`, which shares its strings: the meet fails, and step 3 gives the record, which stands first |
| An exact record `{id: int}`, and a `oneOf` of open variants with tags | Each meet types `kind` as `never`, which is no tag: the meet fails, and step 3 gives `{id: int}`, which stands first |
| Under `absent-is-null`, a record with an optional `note`, and the same `oneOf` | Each meet has `note: string \| null`, required and filled |
| Under `defaults-applied`, a record whose optional `kind` has a `default` | The tag key is filled: the meet fails, and step 3 decides |
| Two `oneOf` parts | Two unions: step 3 |

9. No other rule of F1 3 changes. Depth and the limits of F1 3.10 are those of
   2.6.

## 9. Descriptions and host types

1. The description of a type (F1 5.1) is its printed text, by 2.4. It reads back
   as the same union. An environment built from a description (F1 5.4) accepts
   it as declared type text.
2. The key `types` of F4 8 prints a named union by its structure, as any named
   type.
3. **No binding derives a union** from a host type in this package. F1 6.1, "No
   union", stands. A host writes type text or a schema.

## 10. Checking

### 10.1 Diagnostics

No check code is added. These change where they are reported.

| Code | With this package |
| --- | --- |
| `bad-type`, `bad-signature` | Two or more records in one `Type` only when they form no union (3.2) |
| `untagged-union` | Only for records that form no union (3.2) |
| `maybe-absent` | A plain read of a field that only some variants list (5.1) |
| `no-field` | A read of a field that no variant lists, when every variant is exact (5.1) |
| `never-equal` | A test of a field with a literal that its type in flat(U) cannot hold, by `05-types.md` 8.2 and 11.2. Such a test gives no fact |
| `non-exhaustive` | A `match` on the tag that misses a string of the tag type, by `05-types.md` 11.2 |
| `always-truthy` | A union in a tested position, under `constant-condition` (4.4) |
| `result-mismatch`, `arg-mismatch`, `maybe-null` | At the places of 6 for a map literal against a union |
| `never-conforms`, `always-truthy`, `always-falsy` | On `as` and `is` with a union, by F4 6.3 and the helpers of 4.2 and 4.5 |

### 10.2 `as` and `is`

F4 6.3 and 6.4 apply with the helpers of 4.5. So, with `s: Shape`:

| Text | Result |
| --- | --- |
| `s as Shape` | `Shape`, no site |
| `s as {kind: "circle", r: float}` | That record, and a `value` site |
| `s as {kind: "tri"}` | `never-conforms` |
| `s is Shape` | `always-truthy` |
| `s is {kind: "circle", r: float} ? s.r : s.h` | `float`: `is` narrows both ways |
| `blob as Shape`, `blob: any` | `Shape`, and a `value` site |

### 10.3 Sites and the promise

No site and no class is added. An `as` whose subject does not surely fit its
union is a `value` site by F4 6.6. 5.8 gives what the promise rests on.

## 11. Cases

### 11.1 The case format

This package adds no field to the case format. Its cases use the fields of
`03-host.md` 3.10, of F1 7 (`schemas`, `description`) and of F4 9 (`types`,
`conform`).

### 11.2 The runner

1. A runner of an implementation that offers F5 runs `suite/unions/`. An
   implementation that does not offer F5 does not.
2. A case there that has `schemas`, or that states `type`, `sites` or
   `description`, or that expects a `check` code that is not static, is run
   once, checked. A conform case is run by F4 9.3. Every other case is run
   checked and with the static rules alone.
3. In `suite/unions/` the static codes are the eleven of Core, `untagged-union`,
   and `check-limit` at a program type, as in `suite/conversion/` (F4 6.1 item
   4).

## 12. Earlier cases

1. **Files removed.** Two files pinned this package's texts as errors, for
   an implementation that did not offer it (`03-host.md` 3.11 item 4): one in
   `suite/host-types/` (6 cases) and one in `suite/conversion/` (5 cases).
   They were deleted when every implementation offered the package. Each of
   their cases has a case of the same name and text in
   `suite/unions/unions-until-texts.json`, with this chapter's outcome.
2. **Core and earlier cases.** No case of `suite/core-run/`,
   `suite/core-check/`, `suite/host-types/` or `suite/conversion/` outside the
   two files pins records that form a tagged union as an error. Every two-record
   type they pin has no tag, and stays an error.

## 13. Examples

The environment declares the named type `Shape`, which is `{kind: "circle", r:
float} | {kind: "rect", h: float, w: float}`, and the variables `s: Shape`,
`shapes: Shape[]`, `ps: Shape | null` and `blob: any`.

| # | Text | Type, value or code |
| --- | --- | --- |
| 1 | `s` | `{kind: "circle", r: float} \| {h: float, kind: "rect", w: float}` |
| 2 | `s.kind` | `"circle" \| "rect"` |
| 3 | `s.r` | `maybe-absent` on `r` |
| 4 | `s?.r ?? 0` | `float` |
| 5 | `s.kind == "circle" ? s.r : s.w * s.h` | `float`; with `s` = `{kind: "rect", w: 2, h: 3}`, the value 6 in 5 steps |
| 6 | `match s.kind { "circle" => s.r, "rect" => s.w }` | `float` |
| 7 | `s.kind == "tri"` | `never-equal` |
| 8 | `shapes.filter(x => x.kind == "circle")` | `{kind: "circle", r: float}[]` |
| 9 | `ps?.kind == "rect" ? ps.w : 0` | `float` |
| 10 | `blob as Shape`, `blob` = `{kind: "rect", w: 2}` | `not-conforming`, path `["h"]`, `required field "h" is missing`; 3 steps |
| 11 | `blob is Shape`, `blob` = `{kind: "oval"}` | `false`; 2 steps |
| 12 | `[{kind: "circle", r: 1}, {kind: "rect", w: 2}]`, expected `Shape[]` | `result-mismatch` on the second map |
| 13 | `blob as {a: int} \| {b: string}` | `untagged-union` on `{a: int} \| {b: string}` |
| 14 | `{...s, id: 1}` | `{h?: float, id: int, kind: "circle" \| "rect", r?: float, w?: float}` |
