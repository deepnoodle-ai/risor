# Risor v3 design: the type system and the checker

Status: design proposal, revised to the fifth revision of `00-overview.md`, to
the revised `02-language.md`, to the accepted plan `07-checked-conversion.md`
and to the six accepted plans `08` to `13` (D46 to D51). No code was written.
Decision numbers such as N5, B5, E6 and D29 refer to the overview. A decision of
a plan is cited by its own number, such as 09-3.

**How to read this.** Section 2 defines the type system. In it, a rule is
normative: every implementation must accept and reject the same programs, infer
the same types, and report the same diagnostic codes at the same positions. A
paragraph headed *Why* is not normative. Type text is in TypeScript's spelling
(D29) and 2.2 is its grammar. Section 2.18 maps the spelling of earlier
revisions to it, for a reader of older notes. Sections 3 to 11 are not
normative. Nothing in section 2 depends on Go, Rust or TypeScript; the Go
implementation is section 5, and the TypeScript and Rust ones are section 6.

**Fourth and fifth revisions.** They apply eight decisions. The absent value is
null (D28). Type text is TypeScript's (D29). The falsy set is confirmed (D30).
The two constant-condition codes form a check class a host may take out (D32,
2.13). A host may expect a condition and not a `bool` (D33, 2.14). A lambda is
passed only to a library function or method, and a parameter cannot be called
(D34, 2.11). `if`, `else if` and `else` form an expression, checked as the `? :`
chain it stands for (D35, 2.5). A binding is written `const` (D36).

**Checked conversion** (D39 to D45). A program can test a value against a type
with `x as T` and `x is T`, and an environment can name a type. `07` is the
authority for the feature. This document owns five parts of it: what may be
written as T (2.2), how the two operators are checked and how `is` narrows (2.5,
2.8), when a value conforms and when two types are disjoint (2.14), named types
(2.15), and the bound on the walk (2.17). The rest is cited from `07`.

**Six plans** (D46 to D51). Each plan is the authority for its feature. This
document owns the parts below, and cites the plan for the rest.

| Plan | Decision | What this document owns |
| --- | --- | --- |
| `08`, strings | D46 | The types of the five forms (2.5), and which may stand in type text (2.2). No rule changed. |
| `09`, tagged unions | D47 | The type (2.1), its text and printing (2.2), Join, Fits, flat and the helpers (2.3), reads (2.7), the tests that narrow (2.8, 2.9), the code `untagged-union` (2.13), conformance and disjoint (2.14), `anyOf` and `oneOf` (2.15). And the rule "e fits T through its form" (2.5), which is the one rule for where `result-mismatch` and `arg-mismatch` are reported. |
| `10`, errors | D48 | The typing of `try { } catch (err) { }` and of the error value (2.5), and the class of a run error (2.14). Messages are the catalog's, and none is written here (2.13). |
| `11`, testing | D49 | The fields of a check case (2.16). The field `expected` is now `returns`. |
| `12`, config files | D50 | Nothing of its own in the rules: its span rule is the rule of 2.5. The option a loader sets on a schema (2.15), and an optional field in a type taken from a host type (sections 5 and 6.2). Imports are not in 3.0 (`12:3.9`). |
| `13`, queries | D51 | The type of a projection (2.7), and the rule that `filter` and `find` narrow (2.11). |

One check code is new, `untagged-union`: 29 codes. One node is new, `Each`: 32
kinds. `Try` keeps its number, 26, with new fields. Every `try(e, d)` of earlier
revisions is now `try { e } catch { d }`.

**Evidence.** **(R)** read at the cited lines, **(X)** ran and observed, **(I)**
inferred. `expr:` is `~/git/deepnoodle/expr` at v1.4.1, `workflow:` is
`github.com/deepnoodle-ai/workflow@v0.1.0`, `02:` and `04:` to `13:` are the
sibling documents.

## 1. Summary

1. **A type is a set of alternatives, at most one per kind.** The kinds are
   null, bool, number, string, list, map, and one kind per opaque host type. A
   value that may be absent has the type `T | null`. `any` (unchecked) and
   `never` (no value) stand outside. Type text is in TypeScript's spelling:
   `string[]`, `int | null`, `(s: string) => int` (D29).
2. **Functions are not types.** A lambda is an argument to a library function or
   method, or a `const` value (E6, D34), so function types exist only inside
   library signatures. A parameter never holds a function and cannot be called.
   A `const`-bound lambda is checked at each use, as if its body were written
   there.
3. **`int` means a whole number and fits `float`** (N5). A literal is typed by
   its value, so `2.0` is an `int`. `/` gives `float`, so `xs[10 / 2]` is an
   error and the fix is `int(...)`. The checker folds no constants.
4. **Records are exact or open.** A field is required or optional (`f?: T`), and
   a plain `.` on an optional field is an error (B2).
5. **Any value can be a condition** (B1, B5). Every type has a truthy part and a
   falsy part. `a || b` has the truthy part of `a` joined with `b`, `a && b` has
   the falsy part of `a` joined with `b`, and `!a` is `bool` (B6, B7). A test
   that can only go one way is an error: `always-truthy` or `always-falsy`. The
   two codes are the check class `constant-condition`, which an environment
   rejects by default and a host may take out (D32). Narrowing comes from a bare
   path, from `== null` and `!= null`, and from their combinations. Each removes
   null. An `if` is checked as the `? :` chain it stands for (D35).
6. **A chain is typed as a unit** (B4): one `?.` on a receiver that may be null
   makes the whole chain nullable, and later plain steps need no `?`.
7. **`match` by cases is checked for coverage** against string literal sets,
   `bool` and null.
8. **Every rule names its diagnostic code, its span, and the type the node has
   afterwards,** so three implementations agree on every diagnostic.
9. **The promise:** with a conforming environment, a checked program fails on a
   null, a missing field or a wrong kind only at a listed `dynamic` site, and on
   an index, a zero divisor or a fallible call only at a listed site of that
   class. Number range and the limits can stop any run. Truthiness adds no way
   to fail. Its cost is in result types: `&&` and `||` give unions. A host that
   needs a `bool` checks for one, and a host that tests by truthiness expects a
   condition (B8, D33).
10. **Portable declarations are type text and JSON Schema** (P5). A host type
    with behaviour is an opaque type: a name and declared methods.
11. **Three checkers, written from this document and the cases: Go, Rust and
    TypeScript** (C1). Nothing in the checker needs a garbage collector or
    reflection, and every recursion it makes is bounded by a number in the
    specification (R5).
12. **Size:** about 2,900 lines per implementation, down from 3,250 in the first
    draft: the statement tier, function types and nominal records are gone.
    Truthiness is about 125 of those lines. D34 removes about 30, and check
    classes, `condition` and `if` add about 30. `as` and `is` add about 130.
    The six plans add about 405, most of it tagged unions (section 11).
13. **A program can test a value against a type** (D39). `x as T` gives the
    value back with type T or fails, and is a `value` site unless the type of
    `x` surely fits T. `x is T` is a `bool`, and narrows `x` to T where it
    passed and away from T where it failed. The test is the conformance rule
    the promise already rested on (2.14). T is written in place or is a name the
    environment declares, and a named type is an abbreviation, never a new kind
    of type. One code came with it, `never-conforms`, for an `as` that no value
    could pass.
14. **A map may have one of several shapes, told apart by a tag** (D47). The
    map kind of a type holds one record, or one tagged union: two or more
    records with a required field whose literal sets share no string. It is
    written `{...} | {...}`, with no new syntax. Only a written type makes one:
    Join never does. Before a test it reads as the joined record. A test of the
    tag, a `match` on it and `is` narrow it. Two records with no tag are an
    error in type text: the one new code, `untagged-union`. That makes 29.
15. **An expression fits a written type through its form** (D47, D50). Where a
    program's result meets the host's expected type, and where an argument
    meets its parameter, a list literal, a map literal, a conditional, a `match`
    and a block are fitted part by part. No type changes. The diagnostic lands
    on the element that is wrong, which in a config file is the difference
    between one key and the whole file.
16. **`try { e } catch (err) { f }` has the Join of its two parts** (D48). The
    name is an exact record of four fields, and its `code` is a literal set, so
    a misspelled code is `never-equal` and a `match` on it is checked for
    coverage. The value has no class: a program must not see what the checker
    inferred.
17. **A projection is typed as a chain over the element** (D51).
    `orders[*].lines[*].sku` is a `string[]`. And `filter` and `find` narrow:
    `xs.filter(x => x != null)` loses null.

## 2. The type system

### 2.1 Types

A type is `any`, or a set of alternatives with at most one of each kind.

| Kind | Alternative | Detail |
| --- | --- | --- |
| null | `null` | The one absent value (D28). |
| bool | `bool` | |
| number | `int` or `float` | `int` is the whole numbers of `02:2.5`. `float` is every number. |
| string | `string`, or a literal set | A literal set is a finite, non-empty set of strings: `"fail" \| "pass"`. |
| list | `T[]` | One element type. |
| map | a record, or a tagged union | A record has fields, each with a type and marked required or optional, and a **rest** type for every other key. A tagged union is two or more records that a tag tells apart (D47, below). |
| opaque *N* | `N` | Each opaque type an environment declares (2.15) is a kind of its own. |

- The empty set is `never`. `T | null` is T with the null alternative added.
  There is no shorter form: the `?` suffix of earlier revisions is gone (2.2).
- A record's rest is `never` (an **exact** record: no other keys), a type, or
  `any`. `map<T>` is the record with no fields and rest T.
- A **tagged union** is the one way the map kind holds more than one record, so
  a type still has at most one alternative of a kind. The table below defines
  it.
- `any` is the unchecked type, named `dynamic` before D29. It is not an
  alternative. It may be an element, field or rest type.
- A function and a namespace are not values and have no type (2.4, 2.11).
- A **named type** is an abbreviation that an environment declares (2.15):
  `Order` for `{id: string, total: float, ...}`. The name and the type it stands
  for are the same type in every rule. It is not a kind, and it has no identity
  of its own. A type is always printed by its structure (D44).
- A program can test a value against a type: `x as T` and `x is T` (2.5, 2.8,
  2.14; `07`). The type is written in place, or is a name. A program cannot
  declare a type in 3.0 (D40).

**Tagged unions** (D47; `09:2.3`). `09` is the authority. The running example is
its own: `Shape = {kind: "circle", r: float} | {kind: "rect", h: float, w:
float}`.

| Matter | Rule |
| --- | --- |
| A tag | A key is a tag of some records when every one of them requires it, its type in each is a string literal set (that record's **tag set**), and no string is in two of the sets. |
| A tagged union | Two or more records that have a tag. They are its **variants**. **The tag** of the union is the least such key by code point. It is found from the variants and never written (09-1). |
| A variant | Any record with the tag: exact or open, with optional fields, with a field that is itself a tagged union. |
| What makes one | A written type, and nothing else: type text (2.2) or a JSON Schema (2.15). A test may narrow one that exists to fewer variants (2.8). Join never makes one (2.3; 09-3). |
| Fewer variants | Any two or more variants of a tagged union form a tagged union, because the tag still tells them apart. Its tag is found again from the variants that are left, and may be a lesser key. One variant alone is a record. |
| flat(U) | The Join of the variants of U, taken as plain records by the map row of Join (2.3). It is one record, and every value that conforms to U conforms to it. flat of a record is the record. flat(`Shape`) is `{h?: float, kind: "circle" \| "rect", r?: float, w?: float}`. |
| A record as a union | In the rules of 2.3, 2.8 and 2.14 a record counts as a union of one variant. |
| How it is held | The map slot of a type holds a record or a sorted list of variants. flat(U) is computed once for each union and kept. |

An enum is not part of this. It is a string literal set, as before (`09:2.2`).

| Left out | Why |
| --- | --- |
| Function types as types | E6, S3 and D34: a lambda is never stored, returned, passed to a host or passed to another lambda. |
| Literal sets of numbers and booleans | Only string sets have a consumer (the engine's `switch`, `workflow:typecheck.go:868-903`) (R). `bool` counts as two values where that matters (2.9). A type for `true` alone would make `flag \|\| d` exact; that is not worth a second kind of literal. So a tag is a string, and never a number or a boolean. |
| Untagged unions of records | D47. A walk would try the records one after another, so its first failure and its cost would depend on their order and number, and a value could conform to two (`09`, section 7). Two records in type text with no tag are an error (2.2). |
| Tuples, user generics, annotations, recursive types, `type` lines in a program | Type tests and aliases left this row with D39 and D40, and tagged unions with D47. The rest stay out. A recursive type ends the bound on type depth (2.17) and is the first cyclic structure in the design (R3); write `any` at the point of recursion. Annotations and `type` lines are for a minor version (D40, D45). Structural patterns in `match` can be added later (`02:8.1`): the types they need now exist. |
| Refinements (non-empty list, non-zero number) | They would remove the `index` and `divide` sites, and sharpen the truthy parts of 2.3, at the cost of a value-level type system. |
| Records with methods | A host value is a record or an opaque value, never both (2.15). |

### 2.2 Type text

Type text is in TypeScript's spelling wherever TypeScript has the same idea (C6,
Y4, D29). It is what an author reads in every diagnostic.

```ebnf
Type   = Alt { "|" Alt } .
Alt    = Prim { "[" "]" } .
Prim   = "null" | "bool" | "int" | "float" | "string" | "any" | "never"
       | String | "map" "<" Type ">" | Record
       | Name | Var | "(" Type ")" .
Record = "{" [ ( Field { "," Field } [ "," Rest ] | Rest ) [ "," ] ] "}" .
Field  = ( Word | String ) [ "?" ] ":" Type .
Rest   = "..." [ Type ] .
Name   = upper-case letter, then one or more of [A-Za-z0-9_] .
Var    = one upper-case letter .                     (* signatures only *)

Sig    = "(" [ Param { "," Param } ] ")" "=>" Type [ "!" ]
         [ "where" Var ":" Bound { "," Var ":" Bound } ] .
Param  = [ "..." ] Word [ "?" ] ":" ( Fn | Type ) .
Fn     = "(" [ Word ":" Type { "," Word ":" Type } ] ")" "=>"
         ( Type | "unknown" ) .
Bound  = "ordered" | "number" .
Method = Recv "." Word ":" Sig .
Recv   = Alt .               (* string, a list, a map or an opaque type *)
```

`Word` and `String` are the tokens of `02:2.2`. So a literal in type text is
`"..."` or `'...'`, and either may have the raw prefix: `r"C:\tmp"` is a literal
set of one string (D46; `08:2.5`). A text block and a backtick are not a
`String` and may not stand in type text.

**Alternatives of one kind** (ruling 11 of round 8; `09`, section 9). A `Type`
is its alternatives, with names replaced and parentheses removed. Earlier
revisions did not say what two of one kind mean. They join by the rows of Join
(2.3), except two records.

| Written | Is the type |
| --- | --- |
| `int \| float` | `float` |
| `"a" \| "b"`; `"a" \| string` | The literal set of both; `string` |
| `int[] \| string[]` | `(int \| string)[]`. The printer never writes the first form. |
| `Time \| Time`, `null \| null` | `Time`, `null` |
| `any` among the alternatives | `any` |
| `never` among the alternatives | The others. `never` alone is `never`. |
| Two or more records | They must form a tagged union (2.1), and are then that union. A named type that is a tagged union gives its variants, so `Shape \| {kind: "tri", a: float}` has three. `map<T>` is a record here. |

**Records with no tag** (09-2). Two or more records among the alternatives of
one `Type` that do not form a tagged union are an error where the text is read.

| Where | Error |
| --- | --- |
| In a declaration | The `declare` error `bad-type` (`05:A.6`). |
| After `as` or `is` | The check code `untagged-union`, on the union: from the start of its first alternative to the end of its last (2.13). |
| In a JSON Schema | No error. `anyOf` and `oneOf` branches with no tag join into one record, as before (2.15). |

| Text | Why it is not tagged |
| --- | --- |
| `{a: int} \| {b: string}` | No common key. |
| `{kind: string} \| {kind: "a"}` | A tag's type must be a literal set. |
| `{kind: "a"} \| {kind?: "b"}` | A tag must be required. |
| `{kind: "a" \| "b"} \| {kind: "b"}`, `Shape \| Shape` | A string in two tag sets. |
| `{kind: "a"} \| map<int>` | No tag in the second. |

The compiler makes the test when it reads the type, with the depth test of 2.17.
So an evaluator with no checker refuses the program too.

**Names.** A `Name` is an opaque type or a named type (2.15). The two share one
set of names, so a `Name` means one thing wherever it is written. A `Var` is one
letter and a `Name` is two or more, so they cannot be confused. A named type is
replaced by the type it stands for when the text is read. A `Name` that nothing
declares is an error: `bad-type` in a declaration (`05:A.6`), `undefined-name`
in a program (2.13). In a `Recv` a `Name` must be an opaque type: a named type
stands for a string, a list or a map, and the library owns the methods of those.

**Type text in a program** (D39; `02:2.2`, `07:2.4`). A program writes a type in
one place: after `as` or `is`. What may stand there is the production `Type` and
everything under it. The rest of this grammar is for declarations only.

| After `as` or `is` | |
| --- | --- |
| Allowed | Every `Prim` but `Var`: the seven words, a literal set, `map<T>`, a record, a `Name`, parentheses; `[]`; `\|`. So `any` and `never` may be written: `x as any` forgets a type, and `x as never` is always `never-conforms`. |
| Not allowed | `Var`, `Sig`, `Fn`, `Method`, `where`, `!`, `unknown` and `condition`. A function is not a value, so no value can be tested against a signature. Each is a parse error (`02`). |
| Where the type ends, and what `{` and `\|` mean there | `02:2.2` and `07:2.4`. This grammar is the same in both places. |

| Part | Rule |
| --- | --- |
| `T[]` | A list of T. `[]` binds tighter than `\|`. |
| `T \| null` | A T that may be null. `null` is an alternative like any other. No suffix on a type means "or null". |
| `?` | Only after a field's key and after a parameter's name: `f?: T` is a field that may be absent, or a parameter that may be omitted. An optional parameter follows every required one. |
| A record's rest | `...T` types every key the record does not list. A bare `...` is `...any`. |
| `Recv` | Usually `T[]` or `map<T>`. A narrower one, as in `string[].join` or `T[][].flat`, makes the method apply only to receivers that fit it (overview S17). |
| A `...` parameter | It is last and takes zero or more arguments. |
| `Fn` | Only as a parameter of a library entry. |
| `!` | Marks an entry that can fail on a value. |

Three points follow TypeScript beyond the four forms Y4 names:

- **Every parameter has a name,** in a `Sig` and in an `Fn`. TypeScript reads
  `(T, int) => U` as two parameters named `T` and `int`. In an `Fn` the names
  are documentation only. They also remove a lookahead: after `(`, a `Word` and
  a `:` start an `Fn`.
- **A rest parameter is typed as the list it collects,** `...rest: T[]`, as in
  TypeScript. The type after `...` must be a list.
- **A predicate's result is `unknown`.** That is how TypeScript's own library
  declares it: `filter(predicate: (value: T, index: number) => unknown)`.

**`unknown` and `condition`.** `unknown` is not a type. It appears only as the
result of an `Fn`, and says that the callback's result is tested by truthiness:
the lambda's body may have any type (2.11). `T[].filter: (f: (item: T, index:
int) => unknown) => T[]` is the form. A host says the same thing about a whole
program when it gives **`condition`** as the program's expected result (2.5, row
1; 2.14; D33). The two words name one rule in two places: the expression is a
tested position (2.8), every type fits, and a result that can only go one way is
a constant condition.

| Word | Where it is written | By whom |
| --- | --- | --- |
| `unknown` | In type text, as the result of an `Fn`, and nowhere else | The library's signatures (`06`) |
| `condition` | As a program's expected result, in place of a type: in the embedding model (`05:A.3`; the Go binding spells it `risor.Condition`) and in the `returns` field of a case (2.16) | A host |

`condition` is not type text, and `unknown` is not an expected result. Neither
may stand where a type is wanted: not as the type of a variable, a parameter or
a field, and not as an operand of Join or Fits. Earlier revisions wrote
`condition` in both places.

Every implementation prints a type the same way:

| Part | Printed as |
| --- | --- |
| Alternatives | In the order bool, number, string, list, map, opaque names by code point, then `null`, joined with `\|`. |
| Null | An alternative like any other, printed last: `int \| null`, `"a" \| "b" \| null`. No parentheses and no suffix. |
| List | The element type, then `[]`. The element is in parentheses when its text has a `\|` at the top level: `(int \| string)[]`, `("a" \| "b")[]`, `(int \| null)[]`. |
| Literal set | Each string in its JSON form (`02:2.13`), in code point order. |
| Record | `{`, fields in code point order of their keys, then the rest, `}`. A key that is a `Word` is bare; any other is in its JSON form. `...` for an `any` rest, `...T` for a type. No fields and a rest that is not `never`: `map<T>`. |
| Tagged union | The variants stand where the record stands, each printed as a record, in code point order of the least string of each tag set, joined with `\|`: `{kind: "circle", r: float} \| {kind: "rect", h: float, w: float} \| null`. As a list element it takes parentheses, by the rule for a `\|` at the top level. The text reads back as the same type. |
| Literal | Always in the JSON form, whatever form wrote it: a raw string prints as a plain one. |
| Named type | Never by its name: the type it stands for is printed (D44). An opaque type is printed by its name, which is all it has. |

| Meaning | Type text |
| --- | --- |
| A list | `string[]` |
| A list of lists | `int[][]` |
| The empty list's type | `never[]` |
| A string that may be null | `string \| null` |
| A literal set that may be null | `"fail" \| "pass" \| null` |
| A list that may be null | `int[] \| null` |
| A list of elements that may be null | `(int \| null)[]` |
| Null alone | `null` |
| Unchecked | `any`, `any[]` |
| A record with an optional field and a nullable one | `{note?: string, owner: string \| null}` |
| A tagged union | `{kind: "circle", r: float} \| {kind: "rect", h: float, w: float}` |
| A list of a tagged union that may be null | `({kind: "none"} \| {header: string, kind: "token"})[] \| null` |
| A signature | `(s: string, n?: int) => string !` |
| A callback | `(item: T, index: int) => U` |
| A predicate | `(item: T, index: int) => unknown` |
| A rest parameter | `...rest: T[]` |
| A method | `T[].map: (f: (item: T, index: int) => U) => U[]` |

**What does not map** to TypeScript, and the rule for each:

| Here | TypeScript | Rule |
| --- | --- | --- |
| `int`, `float` | `number` | Kept (N5). `number` in type text is an error that names both. |
| `bool` | `boolean` | `bool` is kept (Y4). It is the kind's name in `02:2.4` and what `type(x)` returns, and it sits beside `int` and `float`. `boolean` in type text is an error that names `bool`. |
| `map<T>` | `Record<string, T>`, or `{[key: string]: T}` | Kept. Both TypeScript forms are longer, and the first collides in name with "record" here. |
| A rest, `...T`, and an open record, `...` | An index signature, `[key: string]: T` | Kept. They do not mean the same: an index signature constrains the listed fields too, and a rest covers only the other keys. `...` reads as "and more". |
| An exact record, `{a: int}` | No spelling. Every TypeScript object type is open. | Exact stays the default. This is the largest hazard of looking like TypeScript: a reader takes `{a: int}` for "at least `a`". The messages for `no-field` and `arg-mismatch` on a record should say "exactly these fields". |
| `{}`, the empty map | `{}` means any value that is not null | Kept. It appears as a falsy part (2.3), so a message that prints it should say "an empty map". |
| `{...} \| {...}` needs a tag | Any two object types form a union | Kept (D47). The text of a tagged union is TypeScript's, and means what it means there. Two records with no tag are a type there and an error here (`02:3.1`). |
| A field that only some variants have reads as absent: `s?.r` is `float \| null` | An error before narrowing, with `.` and with `?.` | Kept (09-5). The read is a read of the joined record (2.7). |
| `!` after a result | Nothing: TypeScript does not type failure | Kept. |
| `where T: ordered` | `<T extends ...>` in a parameter list | `where` is kept. A `Var` is one upper-case letter and needs no list. |
| `f?: T` against `f: T \| null` | The same two forms, and the same difference under `exactOptionalPropertyTypes` | Kept. |
| `undefined`, `void`, `object`, `Array<T>`, `T?`, `nil` | | Each is an error in type text that names the form to write. |
| `x as T`, trusted and free | `x as T`, tested (D39) | Not type text, but the nearest hazard to it: here `as` is a run-time test with a cost and a failure (2.14). |

**Ambiguities,** and how each is resolved:

| Text | Resolution |
| --- | --- |
| `int \| string[]` | `[]` binds tighter than `\|`, as in TypeScript: `int`, or a list of strings. A list of either is `(int \| string)[]`. The printer never writes the first form for a list of a union. |
| `"a" \| "b"[]` | The same rule: `"a"`, or a list of `"b"`. A literal set as an element is always printed in parentheses. |
| `int[]?`, `int?[]` | Not valid. The `?` suffix on a type is removed, not kept as a short form: it would be two spellings of one type, and `int?[]` beside `int[]?` is the pair most easily misread. `?` stays on a field and on a parameter, where TypeScript has it. |
| `int[] \| null` and `(int \| null)[]` | Both valid, and different. Print cases cover both. |
| `(` in a parameter | `(` then `)`, or `(` then a `Word` and `:`, starts an `Fn`. Anything else is a type in parentheses. A record cannot start with `(`. |
| `(x: T) => U \| null` | `=>` binds loosest: the result is `U \| null`. An `Fn` is never an alternative, so the other reading does not exist. |
| `=> int[] !` | `!` follows a whole type and can start nothing. |

*Why, and what it costs.* Type text is what an author reads most, and the author
is expected to know TypeScript (C6). Two costs came with the spelling. A type
that may be null is longer in the place authors read most (`expected string,
found string | null`), and a long record that is a list is known to be a list
only at its end. And the closer the text looks to TypeScript, the more the two
real differences mislead: exact records and `int`.

### 2.3 Join, fits, merge, the two parts and the helpers

**Join(A, B)** is the type of a value that has type A or type B.

| Case | Result |
| --- | --- |
| Either is `any` | `any` |
| A kind in one only | That alternative, unchanged |
| number, in both | `int` if both are `int`, else `float` |
| string, in both | The union of the sets if both are literal sets, else `string` |
| list, in both | The list of Join(elements) |
| opaque *N*, in both | *N* |
| map, in both: two records | A field in both: Join of the types, optional if either is. A field in one only: optional, and its type is joined with the other's rest unless that rest is `never`. Rest: Join of the rests. The result is one record, also when the two would form a tagged union. |
| map, in both: either is a tagged union | That union if the two are the same by structure. Otherwise Join(flat(A), flat(B)), by the row above (2.1). |

**Join never makes a tagged union** (09-3). Only a written type does, and a
tagged union survives a Join only with itself. A `String` node has the type of
its one string (2.5), so a Join that made unions would give `[{name: "a", role:
"x"}, {name: "b", role: "y"}]` one variant for each element: a type that grows
with the data. Join stays commutative and associative (I). The price is in open
question 9.

| Written | Result |
| --- | --- |
| Join(`Shape`, `Shape`) | `Shape` |
| Join(`{kind: "circle", r: int}`, `{kind: "rect", h: int, w: int}`) | `{h?: int, kind: "circle" \| "rect", r?: int, w?: int}` |
| Join(`Shape`, `{kind: "circle", r: int}`) | flat(`Shape`): the two are not the same, so both are flattened |
| Join(`Shape`, `null`) | `Shape \| null`: the kinds differ, so nothing is joined |

**Fits(S, T)** is the one assignability relation.

| Case | Fits when |
| --- | --- |
| S is `never`, or T is `any` | always |
| S is `any` | always. If T is not `any`, the check **used any**. |
| otherwise | each alternative of S fits the alternative of T of the same kind |
| number | `int` fits `int` and `float`; `float` fits `float` |
| string | a literal set fits a superset and `string`; `string` fits `string` |
| list | Fits(element of S, element of T). Sound, because no value changes. |
| map: a record against a record | (1) Each field of T: S lists it, S's type fits, and it is required in S or optional in T; or S does not list it, it is optional in T, and S's rest fits its type. (2) Each field of S that T does not list fits T's rest. (3) S's rest fits T's rest. |
| map: either is a tagged union | Each variant of S fits some variant of T by the row above. A record is a union of one variant. |

So `{a: int}` fits `map<int>` and `{a: int, ...}`, and `{a: int, b: int}` does
not fit `{a: int}`: there is no width subtyping.

| Written | Fits | Why |
| --- | --- | --- |
| Fits(`{kind: "circle", r: int}`, `Shape`) | yes | It fits the first variant. |
| Fits(flat(`Shape`), `Shape`) | no | Its `kind` is within no one tag set. This is why a value built from two variants in two branches does not fit (open question 9). |
| Fits(`Shape`, `{kind: string, ...}`) | yes | Each variant fits. |
| Fits(`Shape`, flat(`Shape`)) | yes | So does every tagged union into its flat record. |

**Merge(R, M)** is the record built by writing M's entries over R's. It types a
map spread and a computed key.

| Case | Result |
| --- | --- |
| A field required in M | That field, required, with M's type |
| A field optional in M | Join of M's type with R's type for that key (R's field, else R's rest); required only if R requires it |
| A field of R that M does not list | Unchanged if M's rest is `never`, else its type joined with M's rest |
| Rest | Join of the rests |
| M is `any` | Every field of R becomes `any`, and the rest is `any` |
| R or M is a tagged union | Its flat record is used (2.1). So is it wherever a rule asks for the record of a type that holds a union: a read (2.7), a pattern (2.10), bind and a map method (2.11). |

**Truthy(T) and Falsy(T)** are the two parts of a type. The falsy values are
null, `false`, zero, the empty string, the empty list and the empty map, and
every other value is truthy (B5, `02:2.7`). Truthy(T) is the type of the truthy
values of T, and Falsy(T) of the falsy ones. Each is the set of the parts of T's
alternatives. A dash is no alternative.

| Alternative | Its truthy part | Its falsy part |
| --- | --- | --- |
| `null` | – | `null` |
| `bool` | `bool` | `bool` |
| `int`, `float` | the same type | the same type |
| `string` | `string` | `""` |
| a literal set | the set without `""`; – if nothing is left | `""` if the set has it, else – |
| `T[]` | `T[]`; – if T is `never` | `never[]` |
| a record | the record; – if it has no fields and its rest is `never` | `{}` if no field is required, else – |
| a tagged union | the union | –. Every variant requires its tag, so no value of it is an empty map. |
| opaque *N* | *N* | – |

| Outside the table | Truthy | Falsy |
| --- | --- | --- |
| `any` | `any` | `any` |
| `never` | `never` | `never` |

- A part with no alternative left is `never`. So `Truthy(null)` is `never`:
  nothing of type `null` is truthy. `Falsy({id: int})` is `never`: a record with
  a required field is never empty. `Falsy(Time)` is `never`: an opaque value is
  never falsy.
- Join(Truthy(T), Falsy(T)) is T, for every T.
- No type says a number is zero or a boolean is `false` (2.1), so those parts
  are the type itself. Strings, lists and records do better at no cost, because
  `""`, `never[]` and `{}` are already types: they are the types of the literals
  `""`, `[]` and `{}`.

| Helper | Definition |
| --- | --- |
| nonnull(T) | T without its null alternative. `any` stays `any`. |
| overlap(A, B) | False if either is `never`. True if either is `any`. Otherwise true when some kind is in both; for string, two literal sets must share a string. |
| has-opaque(T) | T has an opaque alternative, or an element, field or rest type of T does. For a tagged union, when it holds of a variant. |
| S **surely fits** T | Fits(S, T) holds and the check did not use any. Every value that conforms to S then conforms to T (2.14). Plain Fits does not say that: `any` fits every type, and a value of type `any` may be anything. For a tagged union: each variant of S surely fits some variant of T. |
| flat(U) | The one record a tagged union joins into. Defined in 2.1. |
| disjoint(A, B) | No value conforms to both. Defined in 2.14, beside conformance. |
| without(X, T) | The part of X left when a value is known not to conform to T. Defined in 2.8. |
| hit(U, f, S), all(U, f, S) | The variants of a tagged union that a test of field f against the strings S may select, and those it must select. Defined in 2.8. |

nonnull and overlap stop at the kind, so a tagged union changes neither.

### 2.4 Scope and names

| Matter | Rule |
| --- | --- |
| Scopes | They nest: the environment's variables, the program's `const` names, each block's and each `if` branch's, the body and the handler of a `try`, each on its own, a lambda's parameters. The name of `catch (err)` is in scope in the handler only, and a name bound in the body is not in scope in the handler (`10:2.2`). A name refers to the innermost binding. A `const` value is checked before its names exist. `_` binds nothing. An unused name is not an error. |
| Rebinding | A name bound twice in one block or one pattern is `duplicate-name`. Shadowing an outer name or an environment variable is allowed. A binding named like a namespace is `shadows-namespace` and is not made (S1). |
| Namespaces | A `MethodCall` whose receiver is an `Ident` that names a namespace is a `Call` of that namespace's function (S1). |
| Type names | The names of opaque types and named types (2.15) are a set apart from the names of values. A program reads one only after `as` or `is` (2.2), and a value name nowhere else, so neither hides the other. A type name that nothing declares is `undefined-name`. |
| Two kinds of binding | A `const` whose pattern is a name and whose value is a `Lambda` binds that name to the lambda. Every other binding holds a value and has a type: a variable, any other `const` name, a parameter, a pattern name, the name of a `catch`. A parameter is never bound to a lambda, because no call passes one to it (D34, 2.11). |
| Not values | A namespace, an environment function or a `const`-bound lambda used as a value is `not-a-value`. The one place a `const`-bound lambda's name may stand without a call is as an argument where a library signature has an `Fn` (2.11). |
| Calls | A `Call` resolves its name to the innermost binding. A `const`-bound lambda is called by 2.11. A binding that holds a value cannot be called: `call-of-parameter` on the name (D34). With no binding it is the environment's function, or `undefined-function`. |

So the call graph is fixed by the text. The callee of every `Call` is known from
the scopes alone: a `const`-bound lambda, an environment function, or an error.
No type and no argument decides it.

### 2.5 Checking rules

`synth(node)` gives a type and may report diagnostics and record sites. "Must
fit T" means: report the stated code if it does not, and record a `dynamic` site
if the fit used any. "Must fit T **through its form**" is the same, with the
report placed by the table at the end of this section. An operand that may be
null where null is not accepted is `maybe-null`, and checking continues with
nonnull of its type. A **tested position** takes any type and is defined in 2.8.

The nodes are those of `02:2.3`, by the names this revision gives them: `Const`
(before D36, `Let`) and `Null` (before D28, `Nil`). D35 adds no node: an `if` is
the second form of `Cond`, node 16. D39 adds one, `As`, node 31, with a `form`
(`as` or `is`), a subject `x` and a type. D46 adds none: `String` and `Template`
gain a `form` that the checker does not read. D48 adds none: `Try` keeps its
number, and its fields are now `body`, `name` (or none) and `handler`. D51 adds
one, `Each`, the projection step `[*]`. That is 32 kinds.

| # | Node | Rule |
| --- | --- | --- |
| 1 | `Program` | Check each `Const`, then the result. The host may give an **expected result**: a type, or `condition` (2.2). With a type, the result must fit it through its form (`result-mismatch`, at each place of failure). Nothing is converted: with an expected `bool`, a list does not fit (B8). With `condition`, the result is a tested position (2.8): every type fits, and a result that can only go one way is a constant condition (D33). 2.14 sets the two side by side. |
| 2 | `Const` | If the pattern is a name and the value is a `Lambda`, bind the name to the lambda (2.4, 2.11). Otherwise synth the value and bind the pattern (2.10). |
| 3 | `Block` | A new scope. Its type is its result's type. Facts in force outside hold inside (2.8). |
| 4 | `Ident` | 2.4. Its type is the binding's type, narrowed by 2.8. |
| 5 | `Number` | `int` if the value is a whole number, else `float`. Spelling does not matter: `2.0` and `1e3` are `int`, `1e20` is `float`. |
| 6 | `String` | The literal set of that one string: the value, after escapes and after a text block's indentation is removed. Every literal with no `${ }` is this node, in each of the five forms, the backtick and the text block included (D46; `08:2.5`). So a constant text block has a literal set as its type, however long. |
| 7 | `Template` | A backtick or a text block with at least one `${ }`. Synth each expression part. A part with has-opaque is `kind`. An `any` part is a site. The type is `string`. |
| 8, 9 | `Bool`, `Null` | `bool`, `null`. |
| 10 | `List` | `J[]`, where J is the Join of the item types and of the element type of each spread. No items: `never[]`. |
| 11, 12 | `Map`, `Entry` | Start with `{}`. For each entry in order: a word or string key, or a shorthand name, sets that field, required, to the value's type (a key written twice: `duplicate-key`). A computed key must fit `string` (`kind`), and the record becomes Merge(record, `map<V>`) for its value type V. A spread is Merge(record, M). |
| 13 | `Spread` | In a list the operand must be a list; in a map, a map (`kind`, `maybe-null`). An `any` operand is a site. |
| 14 | `Unary` | `!x`: x is a tested position; the type is `bool`. `-x`: x must fit `float` (`kind`); the type is x's. |
| 15 | `Binary` | The table below. |
| 16 | `Cond` | The test is a tested position. `then` is checked under the test's true facts and `else` under its false facts. The type is their Join. The node has two forms (`02:2.3`). In the `if` form, `if (c1) { b1 } else if (c2) { b2 } else { d }` is checked exactly as `c1 ? b1 : (c2 ? b2 : d)`, with each branch a `Block` (row 3). So each test is a tested position, a test narrows its own branch, the false facts of every earlier test hold in each later branch, each branch is a scope for its `const` names, and the type is the Join of the branches. No rule is particular to `if`. An `else` is always there (D37): its absence is the parser's error. |
| 17, 18, 20, 21 | `Member`, `Index`, `MethodCall`, `OptChain` | 2.7. |
| 19 | `Call` | 2.11. |
| 22, 23 | `Lambda`, `Param` | 2.11 and 2.10. A lambda has no type. |
| 24, 25 | `Match`, `Arm` | 2.9. |
| 26 | `Try` | `try { body } catch (err) { handler }`: the Join of the body and the handler. The table after `As`. |
| 27-29 | patterns | 2.10. |
| 30 | `Bad` | `never`. The parser has reported it. |
| 31 | `As` | `x as T` and `x is T`: the table after the operators. |
| 32 | `Each` | The projection step, `x[*]` and `x?.[*]`: 2.7. |

| Operator | Operands | Type | Diagnostic or site |
| --- | --- | --- | --- |
| `a && b` | any types. `a` is a tested position. `b` is checked under the true facts of `a` (2.8). | Join(Falsy(A), B) | `always-truthy` or `always-falsy` on `a` (2.8) |
| `a \|\| b` | any types. `a` is a tested position. `b` is checked under the false facts of `a`. | Join(Truthy(A), B) | the same |
| `==`, `!=` | any | `bool` | `never-equal` if overlap is false and neither side is the type `null`. Never a site: `==` cannot fail (S11). |
| `<`, `<=`, `>`, `>=` | both fit `float`, or both fit `string` | `bool` | `kind` |
| `in`, `not in` | the right side has one kind: a list (the `==` row then applies to the left side and the element type), or a string or map (the left side must fit `string`) | `bool` | `kind` |
| `??` | any | Join(nonnull(left), right) | |
| `+` | both fit `float`, or both fit `string` | `int` if both fit `int`, `float` for other numbers, `string` for strings | `kind` |
| `-`, `*`, `%` | both fit `float` | `int` if both fit `int`, else `float` | `kind`. `%` records a `divide` site unless the right side is a `Number` node that is not 0. |
| `/` | both fit `float` | `float` | `kind`; `zero-divisor` if the right side is a `Number` node with value 0; a `divide` site as for `%` |
| `**` | the left fits `float`, the right fits `int` | `int` if the left fits `int` and the right is a `Number` node, else `float` | `kind`. A `divide` site unless the right side is a `Number` node or the left is a `Number` node that is not 0. |
| all but `&&`, `\|\|`, `==`, `!=`, `??` | | | Null is refused (`maybe-null`). An `any` operand is a site, and the type is then `any`, except that the `bool` rows stay `bool`. |

The `&&` and `||` rows are applied as written when a part is `never`, because
Join with `never` gives the other type:

| Written | With | Type | Note |
| --- | --- | --- | --- |
| `a \|\| b` | `a: string \| null`, `b: string` | `string` | The default idiom. Truthy(`string \| null`) is `string`. |
| `a \|\| b` | `a: null` | B | Truthy(A) is `never`. Also `always-falsy` on `a`. |
| `a \|\| b` | `a: Time` | Join(`Time`, B) | Falsy(A) is `never`, so `b` never runs. `always-truthy` on `a`. The type still joins B, so that the rule has no special case. |
| `a && b` | `a: {id: int} \| null`, `b: int` | `int \| null` | The guard idiom. Falsy(A) is `null`. |
| `a && b` | `a: {id: int}` | B | Falsy(A) is `never`. Also `always-truthy` on `a`. |
| `a && b`, `a \|\| b` | either is `any` | `any` | No site: a test cannot fail. |

**`As`** (D39; `07:2.6`). Let X be the type of the subject `x`, as narrowed, and
T the type written, with its names replaced (2.2). The cases are tried in this
order.

| Case | `x as T` | `x is T` |
| --- | --- | --- |
| X surely fits T (2.3) | The type is T. Nothing is reported and no site is recorded: under the precondition of 2.14 the test cannot fail. This is how a type is stated or widened: `[] as string[]`. | The type is `bool`. `always-truthy` on the node (2.8). |
| disjoint(X, T) (2.14) | `never-conforms` on the node. The type is T. | The type is `bool`. `always-falsy` on the node (2.8). |
| Anything else | The type is T. The node is a site of class `value` (2.14). | The type is `bool`. Not a site: `is` cannot fail. Its facts are in 2.8. |

- **An untyped subject is the third case.** Fits(`any`, T) holds only by using
  any, so `any` surely fits nothing but `any`. `07:2.6` says "the type of `x`
  fits `T`", and with the Fits of 2.3 that would make `blob as Order` a test
  that is never run and `blob is Order` a constant condition. Surely fits is the
  rule.
- **The subject may have any type.** Null is not refused, and an `any` subject
  is not a `dynamic` site: the test accepts every value.
- **A type that cannot be used.** A `Name` that nothing declares is
  `undefined-name` on the name, and a type deeper than 2.17 allows is
  `check-limit` on the type. The node then has type `never` for `as` and `bool`
  for `is`, gives no fact, and reports nothing else.
- **The result of `as` is T as written,** also where X says more. `n as float`
  with `n: int` is a `float`.
- **The run does not depend on the case.** The test and its cost are fixed by
  the value and by T (`07:2.6`, `07:2.7`). The checker decides only what is
  reported and whether the node is a site.
- **A type with two records and no tag** is `untagged-union` on the union (2.2),
  and the node is treated as for a type that cannot be used.

**`Try`** (D48; `10:2.2`, `10:2.6`). `try { body } catch (err) { handler }`, and
the same with no name: `try { body } catch { handler }`. `10` is the authority
for the form, for what `catch` catches and for the cost.

| Matter | Rule |
| --- | --- |
| Type | The Join of the body's type and the handler's type. |
| The two parts | Each is checked as a branch of an `if` is: a scope for its `const` lines, then its result (row 3). |
| The name | It is in scope in the handler only (2.4), and has the type of the error value, below. With no name, nothing is bound. |
| Sites | Sites recorded in the body are dropped, of every class: a failure at a site is one that `catch` catches (2.14). Sites in the handler are recorded. |
| Diagnostics | Reported in both parts as anywhere else. `try` is for bad values and does not relax the checker (`02:2.12`). |
| Facts | A fact in force outside holds in both parts. Nothing learned in the body holds in the handler. A `try` is not a path and gives no fact. |
| A body that cannot fail | Not reported. A range failure can occur at any arithmetic node (promise 3 of 2.14), so the checker cannot show that a handler never runs. |
| Codes | None is new. The mistakes of form (`try(e, d)`, a missing `catch`, `finally`, `throw`) are the parser's targeted errors (`10:2.2`, `02`). |

**The error value** (10-2; `10:2.3`). In the handler the name holds a map with
exactly four keys. Its type is this exact record:

```
{code: C, data: any, message: string, path: (int | string)[] | null}
```

| Field | Type | Note |
| --- | --- | --- |
| `code` | C, a string literal set: the run codes that `catch` catches | Generated from the catalog: the codes of phase `run` whose entry in `messages.json` has `caught: true` (`10:2.9`). `05:A.6` lists them. The set belongs to the specification version a program is compiled against (`05:A.10`), as a library signature does. |
| `message` | `string` | The text is the catalog's, or the author's for `failed`, or the host's for `function-error`. |
| `path` | `(int \| string)[] \| null` | `10:2.3` writes the element as `string \| int`. It is the same type, printed by the order of 2.2. |
| `data` | `any` | The second argument of `fail`, or null. A use that needs a kind is a `dynamic` site, or goes through `as` or `is`. |

- **It is an ordinary record.** Every rule of this section applies to it, and
  none is particular to it. `err.stack` and `err.name` are `no-field`. A test of
  `err` alone is `always-truthy`, because a record with a required field is
  never empty.
- **A misspelled code is found by the checker.** `err.code == "not-conformin"`
  is `never-equal`, and `match err.code { ... }` is checked for coverage by 2.9:
  without `_` it must list every code of C.
- **A code that passes through `catch` is not in C.** `err.code ==
  "steps-exceeded"` is `never-equal`. So the type says what `10:2.4` says: a
  limit, a cancellation, a crash and `missing-variable` never reach a handler.
- **No field holds the class.** The class of a failure depends on the sites the
  checker recorded (2.14). If a program could read it, replacing a declared type
  with `any` would change a value, against 2.12, and an evaluator with no
  checker could not fill the field. The class stays on the host's diagnostic.
- **No span and no trace** are in the value either (`10:2.3`). A fifth field
  would be a change to this type, and to every `match` that lists the four.

**An expression against a written type** (D47, D50; `09:2.6`). Two places set an
expression against a type that someone wrote: the program's result against the
host's expected type (row 1), and a plain argument against its parameter (2.11,
step 5). There the fit is tried through the form of the expression. This is the
one rule for where `result-mismatch` and `arg-mismatch` are reported. `12:3.3`
states a narrower form of it for a config file and defers to this one.

**e fits T through its form** when the type of e fits T, or, failing that, by
one row of this table. The type of e is the one synth gave it. Nothing is
checked twice, and no type changes.

| e | It fits when | Place of a failure |
| --- | --- | --- |
| A `List`, where T has a list alternative `E[]` | Each item fits E through its form. The type of each spread's operand fits `E[]`. | The item, or inside it by these rows. The spread's operand. |
| A `Map` with no spread and no computed key, where the map alternative of T is a record R | Each entry whose key R lists fits that field's type through its form. Each other entry fits R's rest through its form. Each required field of R has an entry. | The entry's value, or inside it by these rows. The entry's key, when R does not list it and R's rest is `never`. The whole of e, once, when one or more required fields have no entry. |
| The same, where the map alternative of T is a tagged union | The type of the tag's entry is a literal set within the tag set of one variant. e then fits that variant by the row above. | The value of the tag's entry. The whole of e when it has none. |
| A `Cond` of either form, a `Match`, a `Block` | Each branch, each arm's result, the block's result fits T through its form. | By these rows. |
| Anything else, or a row whose condition T does not meet | It does not fit. | e |

| Matter | Rule |
| --- | --- |
| The code | At the program's result, `result-mismatch` at each place of failure. At an argument, at each place of failure: `maybe-null` if the type there has null and fits once null is removed, else `arg-mismatch`. |
| Several failures | Each is reported. The reports are a set, in the order of 2.13. |
| A missing field | Reported on the whole map, not on its `{` (ruling 1 of round 8). |
| A fit that used any | A `dynamic` site on the expression whose fit used it: e itself where the first test passed, or the part a row fitted. |
| What is not followed | A name, a call, a chain, a `try`, an operator: the report is on that node. So `const cfg = {...}; cfg` is reported on `cfg`. |
| `x as T` | Not one of the places. `[...] as Shape[]` is a `value` site and is tested when it runs. An annotation would be a third place when annotations come (D40). |

- **A program accepted before this rule is accepted,** with the same type and
  the same sites. The fit of the type is tried first.
- **A program rejected before may now be accepted.** A list of map literals of
  two variants has the joined record as its element type, which does not fit a
  tagged union (2.3). Through its form each element fits one variant. This is
  what lets a literal, and so a config file, have a tagged union as its expected
  type.
- **The spans of two codes move inward,** with or without a tagged union:
  `gate.failed > 0 ? 1 : "a"` against `int` is reported on `"a"`.
- **No type changes.** The program's type is still the inferred one. That every
  value it can have also conforms to T is what the check showed. So this is not
  an expected type flowing into a literal, which section 8 still rejects.
- **It is sound.** A list's value has the values of its items and the elements
  of its spreads. A map literal with no spread and no computed key has exactly
  the keys written. A conditional's value is the value of one branch. So each
  row shows that every value the expression can have conforms to T.

| Expected | Result expression | Reported |
| --- | --- | --- |
| `Shape[]` | `[{kind: "circle", r: 1}, {kind: "rect", w: 2}]` | `result-mismatch` on the second element: it has no `h`. |
| `Shape[]` | `[{kind: "circle", r: 1}, {kind: "oval", r: 2}]` | `result-mismatch` on `"oval"`. |
| `{listen: string, max_body?: int}` | `{listen: ":80", max_bod: 1}` | `result-mismatch` on the key `max_bod`. |
| `{listen: string, max_body?: int}` | `{max_body: "8"}` | Two: on `"8"`, and on the whole map, which has no `listen`. |
| `int` | `gate.failed > 0 ? 1 : "a"` | `result-mismatch` on `"a"`. |
| `Shape[]` | `shapes.map(x => x.kind == "circle" ? {...x, r: 2} : x)` | `result-mismatch` on the whole call: a call is not followed, and its type is a list of one joined record (example 78). |

### 2.6 Numbers

There is one kind of number at run time (N1). The two static types say what is
known about a value, not how it is stored.

| Point | Rule |
| --- | --- |
| `int` | The value is a whole number: no fractional part, magnitude at most 2^53 − 1 (N2). |
| `float` | The value is any number. |
| Where `int` is required | A list or string index, the exponent of `**`, and an `int` parameter. A `float` there is `kind`, and the help is `int(x)`. |
| Soundness | `+ - * %` and unary `-` on whole operands give a whole number or fail (`02:2.5`), so `int` is true of every result a run reaches. The failure is class `range` and is never a site (N6). |
| Truthiness | Zero is falsy (B5). No type says a number is or is not zero, so both parts of `int` and of `float` are the type itself (2.3), and a number in a tested position is never a constant condition. |
| JSON | Schema `integer` is `int` and `number` is `float`. Untyped JSON is `any`. |
| Host functions | An `int` parameter receives a whole number, which always fits a 64-bit integer. A binding whose parameter is narrower (32-bit, unsigned) marks the function `!`, and a value that does not fit is a `value` failure at the call. A host number that cannot enter the language (an integer outside the whole range, a NaN, an infinity), as a result or in a variable, is the run error `number-range` where it is read, with class `environment` (2.14; `04:A.2`, `05:A.3`). |
| Literals are typed by value | `02:2.5` says so, and it is right: with one kind of number, `2` and `2.0` are one value, so `xs[2.0]` must check exactly when `xs[2]` does. My first draft typed `2.0` as `float`. This document changes and `02` stands. |
| `xs[10 / 2]` is an error, and should stay one | `/` has one result type (N4). Folding constants would accept it, but then every checker must carry the arithmetic of `02:2.5`, bit for bit, to type an expression nobody writes. What people do write is `xs[len(xs) / 2]`, which no folding helps. Both get the same help: `int(len(xs) / 2)`. The only places the checker looks at a number's value are the `Number` node tests of 2.5. |

### 2.7 Reads and chains

A **chain** is a primary and the steps that follow it (`02:2.8`). Check it left
to right with a current type *cur* and a flag *skipped*, false at the start.

| Step | Rule |
| --- | --- |
| 1. Start | *cur* is the type of the primary. |
| 2. Before a plain step (`.f`, `[e]`, `.m(args)`, `[*]`) | If *cur* has null, report `maybe-null` on the receiver and remove null. |
| 3. Before an optional step (`?.f`, `?.[e]`, `?.m(args)`, `?.[*]`) | If *cur* has null, set *skipped* and remove null. |
| 4. One kind | *cur* must now have exactly one kind, or be `never`, or be `any` (the step is then a site and *cur* stays `any`). Otherwise `kind` on the receiver. |
| 5. The step | *cur* becomes the result of the read below, or of the method call (2.11), or of the projection that a star starts (below). |
| 6. End | The chain's type is *cur*, plus null if *skipped*. |

The receiver of a step is the chain from its start up to that step, and that is
the span of a diagnostic or a site "on the receiver".

| Receiver | Step | Plain | Optional |
| --- | --- | --- | --- |
| record | field `f`, or `["f"]` with a `String` node: required | T | T |
| | the same: optional, or unlisted with a typed rest | `maybe-absent`; continues with T | T plus null |
| | the same: unlisted, rest `any` | `any`, and a site | `any` |
| | the same: unlisted, rest `never` | `no-field` | `no-field` |
| record | `[e]`, any other e, which must fit `string` | `maybe-absent`, or a site if the rest is `any`; continues with V | V plus null |
| list | `[e]`, e must fit `int` | the element type, and an `index` site | the element type plus null |
| string | `[e]`, e must fit `int` | `string`, and an `index` site | `string \| null` |
| a tagged union U | field or index | as the record flat(U) | as the record flat(U) |
| anything else | field or index | `kind` | `kind` |

V is the Join of every field type and the rest. A field that 2.8 knows is
present reads as required.

**A read of a tagged union is a read of its flat record** (09-5; `09:2.5`). No
rule is added. A field that only some variants have is optional in the flat
record, so the rows above give the answer.

| Written, with `s: Shape` | Result |
| --- | --- |
| `s.kind` | `"circle" \| "rect"`: the tag is required in flat(`Shape`), with the union of the tag sets. |
| `s.r` | `maybe-absent` on `r`: only one variant has it. The help can name the test of `s.kind`. |
| `s?.r ?? 0` | `float` |
| `s.color` | `no-field`: no variant has it, and every variant is exact. |
| `s.kind == "circle" ? s.r : s.w * s.h` | `float`: the test narrows `s` to one variant in each branch (2.8). |

With `a: {b: {c: int}} | null`, `a?.b.c` is `int | null`: one `?.` covers the
chain. With `b?: {c: int}` and `a` never null, `a?.b.c` is `maybe-null` on
`a?.b`: an absent `b` gives null for that step only, and the chain goes on to
`.c`. The fix is `a?.b?.c`, which is `int | null`.

Step 4 is where a union from `&&` or `||` shows. `(gate.failed || "none")` is
`int | "none"`, and a read or method call on it is `kind`: the author must say
which was meant (2.14).

**Projections** (D51; `13:5.2`, with 13-2 as recommended). A **projection** is a
star, `[*]` or `?.[*]`, and the reads and stars that follow it directly: `.f`,
`?.f`, `[e]`, `?.[e]`, `[*]`, `?.[*]`. It ends before the first method call, or
at the end of the chain. `orders[*].lines[*].sku` is one projection. `13` is the
authority for what it evaluates to and what it costs. Its type is this.

| Matter | Rule |
| --- | --- |
| The receiver of the first star | Steps 2 to 4 apply to it as to any step. So a plain star on a receiver that may be null is `maybe-null`, and `x?.[*]` sets *skipped*: the whole chain is then nullable (B4). *cur* must then be a list, `T[]`. |
| A receiver that is not a list | `kind` on the receiver, whatever the kind: a string and a map included. Nothing is treated as a list of one. For a map the help says to write `.values()[*]` (ruling 10 of round 8). |
| An `any` receiver | A site, as in step 4, and T is `any`. Each read after it is a site as it would be in any chain. |
| A `never` receiver | T is `never`, as after any error, and the projection has type `never` and not `never[]`, so that nothing more is reported. |
| The reads | They are checked by steps 2 to 5 as a chain whose *cur* starts at T. The projection has a flag of its own, *skipped-element*, false at the start. An optional step inside the projection that finds a receiver that may be null sets that flag, and not *skipped*. |
| A later star | Its receiver must be a list, `V[]`, by the rows above, and the reads go on from V. |
| The type | When the reads end at U, the projection has type `U[]`, or `(U \| null)[]` if *skipped-element* is set. It is one flat list, with one star or with several. A null that a `?.` gives for an element is kept (ruling 10). |
| After it | *cur* is that list type, and the chain goes on. A method call receives the list. A star after a method call starts a new projection. |
| Sites | Recorded as in any chain: a plain index read is an `index` site on its `Index` node, once, however many elements it is applied to. |
| Facts | None. A chain with a star is not a path (2.8), so a projection gives no fact, and no fact applies to its reads. |
| Depth | A projection's type is never deeper than its receiver's, or it is `any[]`. So it needs no depth test (2.17). |

| Written | Type, or diagnostic |
| --- | --- |
| `orders[*].id`, with `orders: {id: string, lines: {sku: string, qty: int}[]}[]` | `string[]` |
| `orders[*].lines` | `{qty: int, sku: string}[][]`: a list that no star iterated is not flattened. |
| `orders[*].lines[*].sku` | `string[]` |
| `orders[*].lines[0].qty` | `int[]`, with one `index` site, on `orders[*].lines[0]` |
| `orders[*].lines[*].qty.sum()` | `int`: the method call ends the projection and receives the `int[]`. |
| `users[*].email.toLowerCase()`, with `users: {email: string}[]` | `no-method` on `toLowerCase`: the receiver is a `string[]`. The help says to write `users.map(u => u.email.toLowerCase())`. |
| `orders?.[*].id`, with `orders` as above or null | `string[] \| null`: the star's `?.` covers the chain. |
| `orders[*].lines?.[*].sku`, with `lines` a list or null | `(string \| null)[]`: one null for an order with no lines, among the skus. |
| `users[*]?.note`, with `note?: string` | `(string \| null)[]`, by the read table: an absent field reads as null. |
| `services[*].owner`, with `services: map<{owner: string}>` | `kind` on `services`. Write `services.values()[*].owner`. |
| `body.items[*].tags[*]`, with `body: any` | `any[]`, with four `dynamic` sites: `body`, `body.items`, `body.items[*]` and `body.items[*].tags`. |

*Why a method call ends it* (13-2). The common aggregate then needs no
parentheses: `orders[*].lines[*].qty.sum()`. The other rule, in which a star
covers the rest of the chain as `?.` does, would apply `sum` to each number. The
price is the sixth row: a method of the element is not reached, and the checker
says so with `no-method`. One case is silent under either rule: a method that
both the element and the list have, such as `slice` on a list of strings.

### 2.8 Conditions, null and narrowing

**Tested positions.** A value is tested by truthiness in seven places. The
run-time test cannot fail (`02:2.7`), so a tested position accepts every type,
and an `any` value there is not a site.

| Construct | The tested expression |
| --- | --- |
| `c ? a : b` | `c` |
| `if (c) { a } else if (d) { b } else { e }` | `c` and `d` (2.5) |
| A condition arm of `match` (2.9) | the condition |
| `a && b`, `a \|\| b` | `a`. `b` is a result, not a test. |
| `!a` | `a` |
| A lambda passed where the signature says `unknown` (2.11) | the lambda's body |
| A program whose host expects `condition` (2.5, row 1) | the program's result |

**Constant conditions** (B7). Let C be the type of a tested expression, as
narrowed.

| Code | Reported when | Reads as |
| --- | --- | --- |
| `always-truthy` | Falsy(C) is `never` and C is not `never` | No value of this type is null, `false`, zero or empty, so the test always passes. |
| `always-falsy` | Truthy(C) is `never` and C is not `never` | Every value of this type is null or empty, so the test never passes. |

- The span is the tested expression. The node keeps its type, and checking goes
  on.
- The two codes are the check class `constant-condition` (2.13). An environment
  rejects the class unless its host takes it out, and then both are errors, like
  every code in 2.13. With the class out, neither is reported. There is no
  weaker severity (D32).
- `any` and `never` report neither.
- The checker looks at types, not values. `true ? a : b` and `1 || x` are not
  reported, because `bool` and `int` have both parts. `"x" || y`, `null || y`
  and `[] || y` are reported, because the types of those literals have one part.
- When a tested expression is itself `a && b` or `a || b`, the rule applies to
  its type as well: `(x || started) ? 1 : 2` reports the test, because a value
  that is `x` when truthy and else a `Time` is never falsy.
- **An `x is T` whose answer the types decide is a constant condition too,**
  wherever it stands (2.5). It is `always-truthy` when the type of `x` surely
  fits T, and `always-falsy` when the two are disjoint. The span is the whole
  `As` node. The node is a `bool`, so the position it stands in reports nothing
  more. The two codes are of the same class for the same reason: tightening a
  declared type can make a stored `x is T` always true.

| Written | With | Result |
| --- | --- | --- |
| `x \|\| d`, `x && x.id`, `x ? a : b` | `x: {id: int} \| null` | Accepted. `x` may be null. |
| the same | `x: {id: int}` | `always-truthy` on `x`. A record with a required field is never empty, and `x` is never null. |
| `x \|\| d`, `x ? a : b`, `!x` | `x: {id?: int}`, `map<int>`, `int[]`, `string`, `int` or `bool` | Accepted. Each may be empty, zero or `false`. |
| `t \|\| d` | `t: Time` | `always-truthy` on `t`. An opaque value is never falsy. |
| `t \|\| d` | `t: Time \| null` | Accepted. |
| `s \|\| d`, `s ? a : b` | `s: Shape`, a tagged union | `always-truthy` on `s`. Every variant requires its tag (2.3). |
| `role \|\| "member"` | `role: "admin" \| "member"` | `always-truthy` on `role`. |
| `xs.filter(t => t.status)` | `status: "fail" \| "pass"` | `always-truthy` on `t.status`. The filter would keep everything. |
| `x != null ? a : b`, `x ?? d`, `x?.f` | `x` is never null | Accepted, and never reported. |

*Why errors, and why a class.* The mistake truthiness invites is a test that
cannot do what its author thinks: `gate.status ? "ok" : "bad"` on a status that
is `"fail" | "pass"`, or a default after a value that is always there.
TypeScript reports the same class of mistake. The price is that tightening a
declared type from `T | null` to `T` turns a stored `x && x.f` into an error,
where a stored `x != null && x.f` stays valid. For an author that is the
intended split. A truthiness test says the value can go both ways, and the
checker holds the author to it. A comparison with null, `??`, `?.`, a `null`
case and a `_` arm after full coverage (2.9) are the defensive forms, and no
rule reports them when there is nothing to defend against. For a host that
changes a schema under stored programs it is a cost with no author present to
pay it. That host takes `constant-condition` out of the set its check rejects
for the re-check (2.13, D32).

**Paths and facts.** A **path** is an `Ident` followed by zero or more field
steps (`.f`, `?.f`, `["f"]`, `?.["f"]` with a `String` node). A **fact** is
about a path. A truthiness test and a null test give one kind of fact. `is`
gives two more (D39). A test of a field of a tagged union gives the same two
(D47), so there are still three kinds of fact.

| Fact | Written here | Says | An expression that is that path then has |
| --- | --- | --- | --- |
| not null | {p} | The value of p is not null. | nonnull of its type |
| conforms | {p: T} | The value of p conforms to T. | T. If its type surely fits T already, nothing changes: a fact never widens a type. |
| does not conform | {p: not T} | The value of p does not conform to T. | without(its type, T) |

| Matter | Rule |
| --- | --- |
| Prefixes | A not-null fact about `x.a.b` also says that `x` and `x.a` are not null and that fields `a` and `b` are present. |
| without(X, T) | `any` if X is `any`. Otherwise X less every alternative that surely fits T. A literal set of X also loses the strings of T's literal set, and goes when none is left. A tagged union of X loses each variant that surely fits T: one variant left is a record, and none left is no map alternative. Nothing else is split: a number that is not an `int` is still a `float`. |
| hit(U, f, S) | For a tagged union U, a key f and a set of strings S: the variants of U in which the type of key f is not disjoint from the literal set S (2.14). The type of a key in a record is its field's type if the record lists it, else the rest. |
| all(U, f, S) | The variants of U that require f with a type that surely fits S. |
| hit and all as types | Each is a set of variants of U: a tagged union, one record, or nothing (2.1). A fact that names no variant is no fact. |
| Several facts on one path | They are applied in the order of their tests in the source. |
| The same fact | Two facts are the same when they are of one kind, about one path and, for the two kinds from `is`, about one type by structure. This is what ∩ compares below. |
| Lifetime | A fact is about a binding, so it does not reach an inner name that shadows it. Nothing ends a fact: no value changes (E5). |
| `const` | A `const` name has its value's type as narrowed where the `const` stands, and is itself a path. |
| Calls | A call is not a path. Bind its result with `const` to test it. |

Each tested expression c has true facts T(c), which hold where c was truthy, and
false facts F(c), which hold where it was falsy:

| c | T(c) | F(c) |
| --- | --- | --- |
| a path `p` | {p} | none |
| `p != null`, `null != p` | {p} | none |
| `p == null`, `null == p` | none | {p} |
| `p is T` | {p: T}. Also {p}, when T has no null alternative and is not `any` | {p: not T}. Also {p}, when T has the null alternative |
| `p.f == L`, `L == p.f` | {p: hit(U, f, {L})}, and {p} | {p: not all(U, f, {L})} |
| `p.f != L`, `L != p.f` | {p: not all(U, f, {L})} | {p: hit(U, f, {L})}, and {p} |
| `!d` | F(d) | T(d) |
| `d && e` | T(d) ∪ T(e) | F(d) ∩ F(e) |
| `d \|\| e` | T(d) ∩ T(e) | F(d) ∪ F(e) |
| anything else | none | none |

**The two rows for a field test** (D47; `09:2.5`, 09-6). In them p is a path, L
is a `String` node, and `p.f` is p with one more field step: `.f`, `?.f`,
`["f"]` or `?.["f"]`. U is the tagged union in the type of p, as narrowed where
the test stands. With no tagged union there, the rows give no fact, as before: a
plain literal set is not narrowed by `==`. Where hit or all names no variant,
that fact is not given, and neither is the {p} beside it.

They apply to `e` in `d && e` under T(d), to `e` in `d || e` under F(d), to the
branches of `c ? a : b` under T(c) and F(c), to an `if` as to the `? :` chain it
stands for (2.5), and to `match` by 2.9. The result of a program is tested, but
its facts apply nowhere. The facts of a predicate's body apply in one place: the
true facts of the lambda given to `filter` or `find` narrow the element type of
the result (2.11, D51).

- **Truthiness and the null tests are one mechanism.** A truthy value is not
  null, so a bare path gives the same fact as `p != null`. `report &&
  report.verdict`, `report != null && report.verdict`, `!report ||
  report.verdict` and `report == null || report.verdict` all read `verdict` from
  a `report` that is known not to be null. They mix freely: `a && b != null &&
  a.x + b.y`.
- **A path with `?.` steps is a path.** `input?.note && input.note.trim()`
  checks: where `input?.note` was truthy, the field `note` is present and not
  null.
- **The `&&` and `||` rows hold for operands of any type,** not only for `bool`.
  `d && e` is truthy only if both were, and falsy only if one was.
- **A true branch of a truthiness test removes null and nothing else.** A truthy
  string is also not empty and a truthy number not zero, but no type says so.
  Where a type could say more (a literal set that has `""`), the fact still
  removes only null, so that truthiness gives one kind of fact.
- **A false branch of a bare path concludes nothing.** Where `x` was falsy it is
  null, or `false`, or zero, or empty, and `x`'s type already allows each. The
  one conclusion a type could hold is "`x` is null", for a record with a
  required field or an opaque value. Nothing can be done with a value known to
  be null except not to use it, and the declared type already forces that. So
  there are no "is null" facts, and F of a path is empty. A false branch does
  conclude something from `!p`, from `p == null` and from `||`, by the table.
- **`is` narrows both ways.** Where `x is T` passed, `x` has type T. Where it
  failed, `x` loses what T would have accepted: with `v: int | string`, the else
  branch of `if (v is int)` has a `string`, and with `role: "admin" | "member" |
  "owner"`, the else branch of `role is "admin" | "owner"` has `"member"`. `07`
  left the failing side open (its section 10). It is decided here, for least
  surprise: TypeScript narrows both branches, and an author who writes the test
  expects the other branch to know what is left. It is sound because it removes
  only what surely fits T. An `any` stays `any`, since nothing can be taken from
  it.
- **The narrowed type is T as written, not the common part of T and the type
  before.** There is no operation here that intersects two types, and this is
  not a reason to add one. With `x: {a: int, ...}`, `x is {b: string, ...}`
  gives a branch where `x.b` reads and `x.a` no longer does. The author who
  wants both writes both fields in T.
- **`is` and the not-null fact.** A value that conformed to a type with no null
  alternative is not null, and a value that failed a type with the null
  alternative is not null either. So `input?.note is string &&
  input.note.trim()` checks, as the truthy form does.
- **A test of a tag narrows a tagged union,** through the two kinds of fact that
  `is` gives. So `!`, `&&`, `||`, `? :`, an `if` chain and a condition arm
  combine a tag test with every other test, and no rule is added for them. With
  `s: Shape | null`, `s?.kind == "circle" && s.r` checks: where the test passed,
  `s` is the circle and is not null.
- **Any field may be f.** It is usually the tag. The rule does not ask: any
  field that tells variants apart narrows, as in TypeScript. That also spares a
  rule for "the tag of a type as narrowed", which can differ from the tag as
  written (2.1).
- **Why the two facts are sound.** The value of p conforms to the type of p: by
  the precondition of 2.14, or by an earlier test. So it conforms to one
  variant. `p.f` was read and equals L, so in that variant the type of f is not
  disjoint from {L}, and the variant is in hit. The false fact needs no
  precondition: a value in a variant of all has f in S, so it would have been
  equal. The true fact is the first fact that rests on the precondition and not
  on the test alone. Where a host broke it, a read in the narrowed branch fails
  with class `environment`, as every such failure does (2.14).
- **`p is T` still takes T as written.** `s is {kind: "circle", ...}` gives a
  branch where `s.r` no longer reads, and `s.kind == "circle"` gives one where
  it does. The help for the first form names the second (open question 8).

| Written, with `s: Shape` | Result |
| --- | --- |
| `s.kind == "circle" ? s.r : s.w * s.h` | `float`, with no site. |
| `if (s.kind != "rect") { s.r } else { s.h }` | `float` |
| `s.kind == "tri"` | `never-equal`, as before, and no fact. |
| `s is {kind: "circle", r: float} ? s.r : s.h` | `float`: `is` narrows both ways, and without leaves the rectangle. |
| `const k = s.kind; k == "circle" ? s.r : 0` | `maybe-absent` on `r`. A tag copied to a name narrows nothing: the test is not of a path with a field step. |
| `s.kind in ["circle"] ? s.r : 0`, `"r" in s ? s.r : 0` | `maybe-absent` on `r`. `in` gives no fact. |
| `input.role == "admin" ? input.role : "x"` | `"admin" \| "member" \| "owner" \| "x"`: a plain literal set is not narrowed by `==`, as before. `is` does that. |

### 2.9 `match`

**By conditions.** `match { c1 => e1, c2 => e2, _ => d }` is checked exactly as
`c1 ? e1 : (c2 ? e2 : d)`, which is also how an `if` chain is checked (2.5).
Each condition is a tested position, so an arm whose condition is
`always-truthy` or `always-falsy` is reported there. No `_` arm is
`non-exhaustive`. An arm after `_` is `unreachable-arm`.

**By cases.** Let S be the subject's type. A case is compared with `==`, never
by truthiness: `match n { true => 1, _ => 0 }` on an `int` is `never-equal`.

| Step | Rule |
| --- | --- |
| Literals | Each literal L has the type of the same literal expression. If overlap(S, L) is false and L is not `null`: `never-equal` on the literal. A `null` case is never reported, as `== null` is not (2.5): it is a defensive form (2.8). A literal equal to an earlier one in the same `match`: `duplicate-case` on the later one. |
| Order | An arm after `_` is `unreachable-arm`. |
| Coverage | S is **finite** when it is not `any` and each alternative is null, `bool` or a string literal set. Its values are then null, `true` and `false`, and the strings. Without `_`, S must be finite and every value listed, else `non-exhaustive`. |
| `_` when the cases already cover S | Accepted, with no diagnostic (`02:2.10`). At run time it catches data that broke its type, and rejecting it would make tightening a schema break stored programs. |
| Narrowing | If the subject is a path, it is not null in every arm after an arm that lists `null`. |
| Narrowing a tagged union | If the subject is `p.f`, a path p with one more field step, and the type of p as narrowed holds a tagged union U (2.8): an arm whose literals are the strings S is checked under {p: hit(U, f, S)}. Every later arm, `_` included, is checked under {p: not all(U, f, S)}, with U as the earlier arms left it. An arm that also lists a literal that is not a string narrows nothing in its own result. Coverage is unchanged: the subject is a literal set. |
| Type | The Join of the arm results, in order. |

So an arm can be unreachable in three ways, each with its own code:
`never-equal` (the subject is never that literal), `duplicate-case` (an earlier
arm took it), `unreachable-arm` (it follows `_`).

| Written, with `s: Shape` | Result |
| --- | --- |
| `match s.kind { "circle" => s.r, "rect" => s.w }` | `float`. With a third variant added to `Shape`: `non-exhaustive`, until it has an arm. |
| `match s.kind { "circle" => s.r, _ => s.h }` | `float`: under `_`, `s` is the other variant. |
| `match err.code { "not-conforming" => 1, "failed" => 2, _ => 0 }` | `int`. The subject is the literal set C of 2.5, so a misspelled case is `never-equal`. |

### 2.10 Patterns

A pattern is bound against a type V: a `const` value's type, or a parameter's.

| Pattern | Rule |
| --- | --- |
| `x`, `_` | x has type V. `_` binds nothing. |
| `{...}` | V must not have null (`maybe-null`) and must be a record (`kind`), or `any` (a site; every name is `any`). Each field is read as `V.key` by 2.7, with the diagnostic on the key. A tagged union is bound as its flat record (2.3): `const {kind, ...rest} = s` is typed from flat(`Shape`). |
| `[...]` | V must be a list (`maybe-null`, `kind`), or `any` as above. Each element name has the element type, and records an `index` site on the name, because the list may be too short. `_` records nothing. |
| a default, `name = d` | d is checked with the names to its left in scope. The name has Join(nonnull(T), D). For a field, the read is as by `?.`, so `maybe-absent` is not reported. For an element, no site is recorded. |
| a parameter default | The same. When a call omits the argument, the name has D. |
| `...rest` in a map pattern | V without the fields the pattern names: an exact record stays exact, and an open record keeps its rest. |
| `...rest` in a list pattern | V. No site: the rest may be empty. |

A default replaces null as well as "absent" (`02:2.9`), which is why the name's
type is built from nonnull(T). It does not replace a falsy value: `{retries =
3}` keeps a `retries` of 0, and `opts?.retries || 3` does not. To accept an
absent field and keep null, write `{note = null}`: the name is then `T | null`.

### 2.11 Signatures and calls

Every library function and method, and every host function and method, has a
signature in the text of 2.2. A host signature has no `Fn` parameter (S3).

**Where a lambda may go** (E6, D34). A **lambda argument** is a `Lambda` node,
or an `Ident` that names a `const`-bound lambda. It is legal in one place: where
the signature of a library function or method has an `Fn`.

| A lambda argument | Result |
| --- | --- |
| Where a library signature has an `Fn` | Checked by step 3 below. |
| In a call of a `const`-bound lambda | `lambda-to-lambda` on the argument. The parameter it would have bound has type `never`, and the body is checked. |
| Anywhere else: any parameter of a host function or method, and a parameter of a library entry that is not an `Fn` | `not-a-value` on the argument. |

A plain argument where the signature has an `Fn` is `arg-mismatch`. Three things
follow, and the rest of this section relies on them:

- **A parameter always has a type.** No rule binds a parameter or a pattern name
  to a lambda, so a call of one is `call-of-parameter` (2.4) whatever was
  passed. `const twice = (f, x) => f(f(x));` is that error twice.
- **The lambdas a body can run are fixed by its text:** the ones it passes to
  library entries, and the `const`-bound ones it calls by name. Each of those
  was bound before the body or inside it. So no lambda is entered while an
  earlier call of it is in progress, and every program terminates by
  construction (overview E9).
- **A use of a `const`-bound lambda is the lambda and its parameter types.**
  Nothing else about the place of use reaches the body.

**Resolving.** A function by 2.4. A method by the one kind of its receiver
(2.7): the library's methods for string, list and map, and the declared methods
of an opaque type. No such method is `no-method`. The receiver is argument zero,
matched against the receiver type of the `Method` text. A receiver that is a
tagged union has the map methods, and is matched as its flat record (2.3). A
method call ends a projection, and its receiver is then the list the projection
gives (2.7). On an `any` receiver the call is a site, each plain argument is
synthesized, each lambda argument is checked with `any` parameters, and the type
is `any`.

**Solving one call.** Each `Var` starts unbound. The steps run in this order.

| Step | Rule |
| --- | --- |
| 1. Count | The arguments must cover every required parameter and no more than are declared, unless the last is `...` (`arity`). |
| 2. Plain arguments | Left to right: synth the argument to A and bind(P, A). An argument taken by a rest parameter `...rest: P[]` is bound, and in step 5 fitted, against P. |
| 3. Lambda arguments | Left to right, against `(q1: Q1, ..., qn: Qn) => R`. A lambda with more than n parameters is `arity`. Substitute each bound `Var` in the Qs; one that is still unbound is `cannot-infer`. Bind each parameter pattern to its Q (2.10), synth the body to B, and bind(R, B). If R has no `Var`, B must fit R (`arg-mismatch` on the body). |
| 3a. An `unknown` result | When R is `unknown`, the body may have any type and nothing is bound or fitted. The body is a tested position (2.8): `always-truthy` or `always-falsy` on the body. An `any` body is not a site. |
| 4. Bounds | `T: ordered` holds when T's solution fits `float` or fits `string`. `T: number` holds when it fits `float`. Otherwise `arg-mismatch` on the call. |
| 5. Fit | Each plain argument must fit its parameter, with every `Var` replaced by its solution, through its form (2.5): `maybe-null`, else `arg-mismatch`, at each place of failure. For an argument that is not a literal, a conditional, a `match` or a block, that place is the argument, as before. |
| 6. Result | The result type with each `Var` replaced. An unbound `Var` is `never`. An entry marked `!` records a `value` site (library) or a `host` site (host) on the call. |
| bind(P, A) | P is a `Var`: its bounds gain A. P has null: continue with nonnull(A). P is `P'[]` and A has a list: bind(P', its element). P is `map<P'>` and A has a record: bind(P', the Join of A's field types and rest). P is a record and A has one: field by field. A tagged union in A is taken as its flat record. A is `any`: every `Var` in P gains `any`. Anything else binds nothing. |
| solution | The Join of a `Var`'s bounds. |
| the seed rule | In step 3, if a `Var` in R is also in a Q and already has solution A0, and B does not fit A0: discard the diagnostics and sites of that body check, set the solution to Join(A0, B), and check the body once more. B must then fit (`arg-mismatch`). This types `reduce` from a seed of `0`, `[]` or `{}`. |

**Predicates.** The library entries that test each element (`filter`, `find`,
`some`, `every`, `count` in `06`) take `(item: T, index: int) => unknown`. So
`items.filter(t => t.owner)` keeps the items with an owner, and `items.filter(t
=> t.status)` on a status that is never empty is `always-truthy`. A host
function has no `Fn` parameter, so `unknown` never appears in a host signature.
A host function that wants a condition takes a `bool`, and the author writes the
test.

**`filter` and `find` narrow** (D51; `13:5.1`). It is the one rule of this
section that is about two entries by name: the list methods `T[].filter` and
`T[].find` of `06`.

| Matter | Rule |
| --- | --- |
| When | The lambda argument's first parameter is a name, x. With a pattern there, or no parameter, nothing is narrowed. |
| The narrowed type N | The type an `Ident` x has under the true facts of the body (2.8), with x bound to the element type. It is the element type itself when no such fact is about x. |
| The result | For `filter`, `N[]` in place of `T[]`. For `find`, `N \| null` in place of `T \| null`. |
| Which facts reach x | A fact about the path x: not null, conforms, does not conform. A not-null fact about a longer path, such as `x?.note`, says that x is not null, by the prefix rule. |
| Which do not | A fact about a field changes no field of a record type: facts narrow paths. `orders.filter(o => o.note != null)` is still a list of orders whose `note` may be null. |
| A `const`-bound lambda by name | Narrowed as if the lambda were written there. |
| Everything else | Unchanged: the solving above, `always-truthy` and `always-falsy` on the body, an `any` receiver, whose result is `any`. |
| Depth | N may be a type from `is`, and `N[]` is one level deeper. A call result is already a place where depth is tested (2.17). |

| Written | Type |
| --- | --- |
| `gate.items.map(t => t.owner).filter(o => o != null)` | `string[]`. Before this rule: `(string \| null)[]`, which `unique()` then refused. |
| `shapes.filter(x => x.kind == "circle")`, with `shapes: Shape[]` | `{kind: "circle", r: float}[]`: a tag test is a fact about x (2.8). |
| `blobs.filter(x => x is Order)`, with `blobs: any[]` | `{id: string, total: float, ...}[]` |
| `shapes.find(x => x.kind != "rect")` | `{kind: "circle", r: float} \| null` |
| `gate.items.filter(t => t.owner)` | The same list type as `gate.items`: the fact is about `t.owner`, and `t` was never null. |

It is sound because `filter` keeps, and `find` returns, only an element for
which the body was truthy, and no value changes between the test and the result
(E5). The other predicates (`some`, `every`, `count`) return no element, so
there is nothing to narrow. `09:2.5` lists a predicate among the things that do
not narrow; it was written before `13` was accepted, and this rule is the later
one. What stays out is a fact about a field (open question 10).

**`const`-bound lambdas.** A lambda has no type. Each use is checked on its own,
because only a use supplies the parameter types.

| Use | Rule |
| --- | --- |
| A call `f(a1, ..., an)` | It must supply every parameter without a default and no more than are declared (`arity`). Each argument is synthesized where the call stands. Each parameter pattern is bound to its argument's type, with defaults by 2.10. The body is then checked in the scope and under the facts where the lambda was written. The call's type is the body's. |
| `f` as an argument where a library signature has an `Fn` | Checked as if the lambda were written there. |
| No use at all | The body is checked once with every parameter `any`. The sites of that check are dropped, and its diagnostics stand. |
| Limit | A program may check at most 1,000 lambda bodies this way, and a use may not nest too deep. Both are in 2.17, and passing either is `check-limit`. |

*Why per use.* `02:2.15` asks for it, and it is the only rule that types a
helper fully without annotations. `const pct = (n, digits = 0) => round(n * 10
** digits);` is called with a `float` and with an `int` in example 34, and
nothing at the `const` says which. One function type synthesized there would
need parameter types from nowhere, so every helper would be unchecked inside. A
conformance test sees the difference: `const name = t => t.nmae;
items.map(name)` is `no-field` at `nmae` under this rule, and under the other
passes the check and fails at run time. A constant condition is per use too:
`const label = x => x || "none";` is reported at `x` when one call passes a
value that is never falsy.

*What D34 simplified.* Before it, an argument of a `const`-bound lambda could be
a lambda. A parameter was then a typed value in one check of a body and a lambda
in the next, a call inside a body could name either, and `const om = x => x(y =>
x(y)); om(z => om(w => z(w)))` was checked until a limit stopped it.

| Question | Answer |
| --- | --- |
| Can a parameter have a function type? | No. There is one kind of parameter, and no function type outside a library signature. |
| Is checking per use still needed? | Yes. `pct` above has two uses and two parameter types. |
| Does the check end without a limit? | Yes. A `const`-bound lambda can use only one bound before it or inside it, so the uses form a graph with no cycle, and expanding them is finite. |
| May a check be reused? | Yes. The body's type, diagnostics and sites depend only on the lambda and its parameter types, so an implementation may keep them under that key. Before D34 there was no such key. |
| Are the limits of 2.17 still reachable? | Yes. They are now bounds on work and on stack, not on termination. `const a = x => x; const b = x => a(x) + a(x); const c = x => b(x) + b(x);` and so on doubles the body checks with each line, and the tenth passes 1,000. A chain of helpers, each using the one before, nests one level deeper with each. |

### 2.12 `any`: unchecked values

| Question | Answer |
| --- | --- |
| What is allowed? | Everything. Each use that needs a particular kind is a `dynamic` site, on the operand: a read, a star, a method call, an arithmetic, ordering or `in` operator, a spread, a destructuring pattern, a template part, and an argument, a result or a part of one whose fit used any (2.5). |
| What is not a site? | Passing the value on: as a result, into a list or map literal, to an `any` parameter, through `??`. Testing it: a tested position, and so `!`, `&&` and `\|\|` (2.8). Comparing it with `==` or `!=` (S11). An optional read of an unlisted key of an open record. Being the subject of `as` or `is` (2.5). |
| What is checked at run time? | Each operation checks its own operands (`02:2.12`). Nothing is wrapped or checked at a boundary. |
| How does untyped data get a type? | `x as T` tests it and gives it back with type T, or fails. `x is T` tests it and narrows `x` (2.5, 2.8; `07`). The value is not changed. |
| Can a host require static typing? | Yes: it rejects the class `dynamic` (2.13). A program can still hold untyped data, pass it on and test it, and must take it through `as` or `is` before any use that needs a kind. That is what makes strict mode usable (`07`, section 1). |
| Do types change results? | An inferred type and a declared type never do. Replacing a declared type with `any` changes no value. It removes diagnostics, and it turns an `as` that could not fail into a site. A type written after `as` or `is` is not of that sort: it is an operand of a test, and the result depends on it. This is why the error value of a `catch` has no `class` field (2.5): the class of a failure depends on the sites, and so on the declared types. |

The type is `any` and the class of its sites is `dynamic`. The two names are
kept apart on purpose (2.14).

### 2.13 Diagnostics

A diagnostic is a code and a span (`05:A.6`). Its message is specified since
D48: one catalog, `messages.json`, has a template for each code of every phase,
these 29 among them, and every implementation generates its messages from it
(`10:2.9`). The wording of a template is in that file and not in this document.
A type in a message is printed by 2.2 and cut by the catalog's rule for a `type`
parameter, so a long literal set needs no rule here (ruling 9 of round 8). The
help and the fix are not specified. This document owns the check codes: every
one has a name, and the table below is the whole list. Every code is an error: a
program with a diagnostic does not check. The checker reports every diagnostic
it finds. The result is a set: two with the same code and span are one, and the
order is by start position, then end position, then code. A node with an error
has the type in the last column, and checking goes on.

| Code | Reported when | Span | Type afterwards |
| --- | --- | --- | --- |
| `undefined-name` | 2.4; a type name after `as` or `is` that nothing declares (2.5) | the identifier; the type name | `never` |
| `undefined-function` | 2.4 | the name, with its namespace | `never`; plain arguments are synthesized, lambdas are not checked |
| `not-a-value` | 2.4, 2.11 | the identifier; for a lambda argument, the argument | `never` |
| `call-of-parameter` | 2.4: a call of a name that holds a value | the name | as `undefined-function` |
| `lambda-to-lambda` | 2.11: a lambda argument in a call of a `const`-bound lambda | the argument | the parameter has type `never`; the call is checked |
| `duplicate-name` | 2.4 | the later name | the later binding stands |
| `shadows-namespace` | 2.4 | the name | no binding |
| `duplicate-key` | 2.5, map literal | the later key | the later entry stands |
| `no-field` | 2.7 | the field name or key | `never` |
| `no-method` | 2.11 | the method name | as `undefined-function` |
| `maybe-absent` | 2.7, 2.10 | the field name, key or index expression | the field's type |
| `maybe-null` | an operand that may be null | the operand; for a read, the receiver | nonnull of it |
| `kind` | an operand of the wrong kind; a star on a value that is not a list (2.7) | the operand. For an operator whose two sides do not go together, the whole `Binary`. For a star, its receiver. | `bool` for the comparison and `in` operators, else `never` |
| `never-equal` | 2.5, 2.9 | the whole `Binary`; in `match`, the literal | `bool` |
| `never-conforms` | 2.5: `x as T` where the type of `x` and T are disjoint (2.14) | the whole `As` node | T |
| `untagged-union` | 2.2: a type written after `as` or `is` in which two or more records are alternatives and do not form a tagged union | the union: from the start of its first alternative to the end of its last | `never` for `as`, `bool` for `is`; no fact |
| `always-truthy` | 2.8: a tested expression that can never be falsy, or an `x is T` that always passes, when the class `constant-condition` is rejected | the tested expression; for a predicate, the lambda's body; for an expected `condition`, the program's result; for `is`, the whole `As` node | unchanged |
| `always-falsy` | 2.8: a tested expression that can never be truthy, or an `x is T` that never passes, when the class is rejected | the same | unchanged |
| `zero-divisor` | 2.5 | the divisor | `float` |
| `arity` | 2.11 | the whole call, or the lambda | `never` |
| `arg-mismatch` | 2.11 | for a plain argument, each place of failure by 2.5: the argument, or a part of a literal, a conditional, a `match` or a block inside it. Otherwise the lambda body, or the call | the signature's result |
| `cannot-infer` | 2.11 | the lambda | the lambda is not checked |
| `check-limit` | 2.17 | the use of a `const`-bound lambda, the node whose type is too deep, or the type after `as` or `is` that is too deep | `never` |
| `duplicate-case` | 2.9 | the later literal | ignored |
| `unreachable-arm` | 2.9 | the arm | the arm is checked and joined |
| `non-exhaustive` | 2.9 | the whole `Match` | the Join of the arms |
| `result-mismatch` | 2.5, `Program` | each place of failure by 2.5: the result expression, or a part of a literal, a conditional, a `match` or a block inside it. One program may have several | unchanged |
| `unchecked` | a `dynamic` site, when the class `dynamic` is rejected | the site | `never` |
| `may-fail` | a site of any other class, when that class is rejected | the site | unchanged |

That is 29 codes. Two came with D34: `call-of-parameter` and `lambda-to-lambda`.
Both are found by resolving names, with no type, and both belong to the `check`
phase, with `undefined-name` and `not-a-value`. `if` (D35) and `const` (D36) add
none: a missing `else`, a branch written like a map and the reserved words are
the parser's targeted errors (`02`). One came with D39: `never-conforms`. It is
to `as` what `never-equal` is to `==`, and like it belongs to no check class. An
unknown type name reuses `undefined-name`, and a failed `as` at run time is the
run code `not-conforming` (`05:A.6`), which is not in this list.

One came with D47: `untagged-union` (09-2). It is found when a type is read,
with no inferred type, and belongs to no check class. The same mistake in a
declaration is the `declare` code `bad-type`. `09`, section 9, asks whether it
is worth a code; the owner took the recommendation, so it is one. The other five
plans add no check code. `try` and `catch` (D48), the string forms (D46) and
`import` (D50) have only the parser's targeted errors, whose codes `02` names. A
star (D51) uses `kind`, `maybe-null` and `no-method`. The rule of 2.5 adds none
either: it moves the spans of `result-mismatch` and `arg-mismatch` inward, and
lets one program have several of each.

| Hint that the help may carry, with no code of its own | On |
| --- | --- |
| Write `.values()[*]` | `kind`, for a star on a map (2.7) |
| Write `xs.map(x => x.f.m())` | `no-method`, for a method call after a projection that the element type has and the list does not (2.7) |
| Test the tag: `s.kind == "circle"` | `maybe-absent`, for a field that only some variants of a tagged union have (2.7) |
| A text block removes its indentation | `no-method` or `undefined-function`, for `trimIndent`, `stripIndent` and `dedent` (`08:2.7`; a hint of `06`) |

**Check classes** (D32). A check class is a named group of findings that the
checker makes on every program, and that an environment either rejects or does
not. There are six.

| Class | Its findings | When the class is rejected | When it is not |
| --- | --- | --- | --- |
| `constant-condition` | A tested expression that can only go one way, and an `x is T` that the types decide (2.8) | Each is a diagnostic: `always-truthy` or `always-falsy` | Not reported: no diagnostic, and nothing in the result |
| `dynamic` | The `dynamic` sites of 2.14 | Each is a diagnostic: `unchecked` | Listed in the result, as sites |
| `index`, `divide`, `value`, `host` | The sites of that class (2.14) | Each is a diagnostic: `may-fail` | Listed in the result, as sites |

| Matter | Rule |
| --- | --- |
| One mechanism | An environment carries one set: the classes its check rejects (`05:A.2`). A host puts a class in or takes it out, each on its own. There is no severity, no warning and no second switch. |
| The default set | `constant-condition`, and nothing else. So by default a constant condition is an error and a site is not. Rejecting `dynamic` as well is strict mode. |
| A finding of a rejected class | A diagnostic, and an error like every other. The program does not check. The diagnostic carries the name of its class (`05:A.6`), so a host can say which of its choices made it an error. |
| A finding of a class that is not rejected | For a site class: the site is in the result's list, because the promise is stated over that list (2.14) and a host or an editor may read it. For `constant-condition`: nothing. A constant condition is not a place where a run can fail, so nothing downstream needs it, and a host that takes the class out wants silence. |
| What the set does not change | Any type, fact or site, or any diagnostic outside the class. With `constant-condition` out, a program has the same type and the same sites as with it in, less the two codes. |
| Codes outside every class | The other 25. They are always reported. |

*Why a host takes `constant-condition` out.* It re-checks stored programs after
a schema changes a field from `T | null` to `T`. A stored `x && x.f` then tests
a value that is never null. It still runs and still gives the right answer, and
no author is there to delete the test. With the class out, tightening a type
this way breaks no stored program that guards with truthiness, with `!= null`,
with `??`, with `?.`, with a `null` case or with `is`. Two forms still break. `a
== b` with `a: int | null` and `b: string | null` checks only because both may
be null, and is `never-equal` once either cannot be. And `x as null` with `x:
int | null` is `never-conforms` once `x` cannot be null. Both are contrived. A
host that authors new programs keeps the class in.

### 2.14 Conformance and the promise

A run-time failure has a **kind**: `null`, `kind`, `field` (the mistakes the
checker looks for), `index`, `divide`, `value`, `host` (failures on a value),
`range` (N2, N3) and `limit`. Every failure has a named run code since round 8:
`null-operand`, `wrong-kind` and `missing-field` for the first three kinds,
`index-range`, `divide-by-zero`, and `no-match` for a `match` that finds no arm.
`02:2.12` names them and `05:A.6` lists every run code with its class. This
document owns the classes and the check codes, and no run code. A **site** is a
class and a span:

| Class | Recorded on | Span |
| --- | --- | --- |
| `dynamic` | 2.12 | the `any` operand |
| `index` | a plain list or string index; a list pattern element without a default | the `Index` node; the element name |
| `divide` | `/`, `%`, `**`, by 2.5 | the `Binary` node |
| `value` | a call of a library entry marked `!`; an `x as T`, unless the type of `x` surely fits T (2.5) | the call; the `As` node |
| `host` | a call of a host entry marked `!` | the call |

Sites inside the body of a `try` are dropped, of every class, and sites inside
its handler are kept (2.5). The result lists each site once, in the order of
2.13. The five site classes are check classes (2.13): a host may **reject** any
of them, and each remaining site of a rejected class is then a diagnostic and
not a listed site. None is rejected by default. Rejecting `dynamic` is strict
mode.

**The class is `dynamic` and the type is `any`.** D29 renamed the type. The
class keeps its name, as does the code `unchecked`. The class says when the
operand's kind is checked, which is at run time, and that is still the right
word for a site. A class named `any` would also read badly where classes are
listed: "reject `any`" and "a run error of class `any`" both say something else.

| Part | Statement |
| --- | --- |
| A value conforms to a type | By the table below. |
| Precondition | Every variable's value conforms to its declared type, and every host function and method returns a value that conforms to its result type. A host can test this itself: every binding gives it the rule below as a function (`05`; `Conform` in Go). |
| Promise 1 | If the checker reports no diagnostic, then in every run a failure of kind `null`, `kind` or `field` occurs only at a `dynamic` site. |
| Promise 2 | A failure of kind `index`, `divide`, `value` or `host` occurs only at a site of that class. |
| Promise 3 | `range` and `limit` failures can occur anywhere, and so can a cancellation. A crash of a host function (`function-panic`, class `host`) can occur at any call of a host function or method, marked `!` or not: it is outside promise 2, and `catch` does not catch it (`04:A.1`). A failure inside the implementation has no class: no promise covers it. |
| Promise 4 | A run that succeeds gives a value that conforms to the inferred type. For `x as T` this holds because the run tests it. |

**Conformance.** This table is the rule, and it is normative. Four things use
it: the precondition and promise 4 above, the operators `as` and `is`, the
function a binding gives a host, and the tests. Before D39 only the first and
the last did, and a program could not see it.

| Type | A value conforms when |
| --- | --- |
| `any` | Always. The value is not looked at, and nothing inside it is. |
| `never` | Never. |
| A set of alternatives | The value's kind is the kind of one alternative, and the value conforms to that alternative. A type has at most one alternative of a kind (2.1), so there is never a choice. |
| `null`, `bool`, `string` | It is of that kind. |
| `float` | It is a number. |
| `int` | It is a number, and whole in the sense of 2.6: no fractional part, and a magnitude of at most 2^53 − 1. How it was written or is stored does not matter: `3.0` conforms, and `1e20` does not. |
| A literal set | It is a string, and a member of the set. |
| `T[]` | It is a list, and every element conforms to T. |
| A record | It is a map. Every required field is present. Every listed field that is present conforms to its type. The value of every key the record does not list conforms to the rest: with a rest of `never` there is no such key, and with a rest of `any` those keys are not looked at. |
| A tagged union | It is a map, it has the tag, the tag's value is a string in the tag set of one variant, and it conforms to that variant. Tag sets share no string, so here too there is never a choice (D47; `09:2.7`). |
| An opaque type N | The binding says the value has that name (2.15). |

- **A field is present when the map has the key,** whatever the value. A key
  whose value is null is present, and null conforms only to a type with the null
  alternative. So `{id: "a", note: null}` does not conform to `{id: string,
  note?: string}` (B2, D42).
- **Nothing is converted.** A string that holds `"42"` does not conform to `int`
  (D39).
- **A record with no `...` is exact** here as everywhere (D41).
- **A named type conforms as the type it stands for.** A type from a JSON Schema
  carries none of the schema's constraints on values (2.15, D43).
- **What this table does not say** is the order in which a walk looks, which
  failure it reports first, the path it gives and the steps it charges. Those
  are `07:2.3` and `07:2.7`, `04` and `05:A.6`. They are needed because a
  program can now see the result. The one rule about cost that belongs here is
  the first row: where the type is `any`, nothing is visited and nothing is
  charged, so `x is any[]` asks only whether `x` is a list.
- **A tagged union is tested by its tag, then by one variant** (09-7). The walk
  reads the tag and tests the one variant whose tag set has that string. It
  never tries variants in turn, so a value that conforms costs what it costs
  against its variant alone, however many variants there are. The steps and the
  path of a failure are `09:2.7`.
- **A host can ask for every failure** (10-8; `10:2.11`). A second function of
  each binding goes on after the first failure, up to a maximum. It applies this
  same table in the same order, and its first failure is the one that `as` and
  the first function report. A config loader uses it (`12:3.4`).
- **A host number that cannot enter the language conforms to no type** (round 6,
  and ruling 5 of round 8). Met by a walk, it simply does not conform.

**Disjoint types.** disjoint(A, B) is the checker's answer to "can no value
conform to both?". 2.5 uses it for `never-conforms` and for `always-falsy` on an
`is`. `07:2.6` names the idea and does not define it.

| Case, in this order | disjoint(A, B) |
| --- | --- |
| Either is `never` | True. |
| Either is `any` | False. |
| Otherwise | True unless some kind has an alternative in both, and those two alternatives share a value by the rows below. |
| null, bool, an opaque type N | They share a value. |
| number | They share a value: a whole number conforms to `int` and to `float`. |
| string | They share a value, unless both are literal sets with no string in common. |
| list | They share a value, whatever the element types: the empty list conforms to every list type. |
| map: two records | They share a value, unless some key that is required in either record has disjoint types in the two. The type of a key in a record is its field's type if the record lists the key, else the record's rest. |
| map: either is a tagged union | They share a value, unless every variant of A is disjoint from every variant of B by the row above. |

| A | B | disjoint | Why |
| --- | --- | --- | --- |
| `"42"` | `int` | yes | No kind in both. `"42" as int` is `never-conforms` (`07:2.2`). |
| `int \| null` | `string \| null` | no | Null conforms to both. |
| `int[]` | `string[]` | no | `[]` conforms to both. |
| `"a" \| "b"` | `"b" \| "c"` | no | `"b"`. |
| `{id: int}` | `{id: string, ...}` | yes | `id` is required, and `int` and `string` are disjoint. |
| `{id: int}` | `{name: string}` | yes | `name` is required in B, and A, being exact, gives it the type `never`. |
| `{id?: int}` | `{name?: string}` | no | `{}` conforms to both. |
| `Shape` | `{kind: "tri", ...}` | yes | `kind` is required, and its sets share nothing, for each variant. |
| `Shape` | `{kind: "circle", ...}` | no | A circle conforms to both. |
| `any` | `{id: never}` | no | See below. |

- **The rule is sound and not complete.** When it says true, no value conforms
  to both. It says false for a type that no value has at all, such as a record
  with a required field of type `never`, set against `any`. Such a test is
  accepted and fails, or gives `false`, at run time.
- **It is `overlap` of 2.3, made exact for two records.** `overlap` asks whether
  two values could be equal and stops at the kind. `never-equal` keeps using it,
  unchanged.
- Like Fits, it recurses on the types and goes no deeper than they do (2.17).

**Classes reported to a host** (`05:A.6`). A failure's class is its kind, except
as this table says. It is the rule for a program that was checked.

| Failure | Class |
| --- | --- |
| Of kind `null`, `kind` or `field`, at a `dynamic` site | `dynamic` |
| Of kind `null`, `kind` or `field`, anywhere else | `environment` |
| A `match` that finds no arm | `environment` |
| A failed `as` | `value` at a site, `environment` anywhere else (below) |
| A host number that cannot enter the language: an integer outside the whole range, a NaN or an infinity, read from a variable or from a host function's result | `environment`, with the code `number-range` (ruling 5 of round 8; `04:A.2`, `05:A.3`). The same code from the language's own arithmetic has class `range`. |
| A host function that crashes: `function-panic` | `host` (ruling 6; `04:A.1`). It needs no site (promise 3). |

So in a checked program `environment` always means the precondition failed, and
a test can check the promise mechanically.

**A program compiled with no check** (ruling 7 of round 8; `10`, section 8, item
4). The first two rows need the sites, and only a check records them. An
evaluator that is built before its checker has none, and neither has a host that
compiles without a check.

| Failure, in a program that was not checked | Class |
| --- | --- |
| Of kind `null`, `kind` or `field` | `dynamic`, always. With no check, every operand is unchecked. |
| A `match` that finds no arm | `dynamic`. No check showed that the arms cover the subject, so the failure cannot be laid on the host. |
| A failed `as` | `value`. No check showed that it could not fail. |
| Every other failure | As in a checked program. `environment` is then reported only where no site is needed to know it: a host number that cannot enter the language, and a variable the host did not supply (`05:A.6`). |

- The ruling states the first row. The second and third follow from its reason
  and are this document's: `environment` must mean that the precondition failed,
  and without a check nothing shows that.
- **A program cannot see the difference.** `catch` decides by code, and the
  error value has no class (2.5). So a checked and an unchecked run of one
  program on the same values give the same value, the same usage, and the same
  code and span on failure. Only the class on the host's diagnostic may differ.
- **A case of the suite that compares a class always checks first** (2.16). An
  implementation with no checker yet runs such a case without comparing the
  class.
- `05` follows this rule in A.6.

**What `catch` does with each class** is not a rule of classes at all: it
decides by code (`10:2.4`). A failure of class `environment` is caught like any
other, except `missing-variable`, which passes through with the limits, a
cancellation and a crash.

**A failed `as`** has the run code `not-conforming` and the kind `value`
(`07:2.3`). At a site its class is `value`. An `as` is not a site only where the
subject's type surely fits T, and a failure there means a value did not conform
to its declared type, so it is reported as `environment`, like every other
failure that the precondition rules out. `07` left this open (its section 10).
Inside the body of a `try` the failure is caught whichever it is. So the kinds
of failure and the classes of site are the same as before D39: an `as` is one
more `value` site, as `07:2.6` says.

**Truthiness and the promise.** Truthiness adds no kind of failure and no class
of site. A test accepts every value, so there is nothing to record. Promise 4
holds for `&&` and `||` because the parts of 2.3 are sound: a truthy value of
type A conforms to Truthy(A), and a falsy one to Falsy(A). What truthiness costs
is precision. `&&` and `||` return an operand (B6), so their types are unions,
and a union has to be resolved before it is used as one kind (2.7, step 4). This
is the reason the second revision removed truthiness. In practice:

| Written | Type | What happens |
| --- | --- | --- |
| `input?.note \|\| "none"` | `string` | Nothing is lost. Most defaults join two types of one kind, and the truthy part drops the null. |
| `gate.failed \|\| "none"` | `int \| string` | Usable in a template or as a result. `+ 1` or `.trim()` on it is `kind`: the author must choose. |
| `input.options && input.options?.retries` | `int \| {} \| null` | The guard returns itself when it is falsy, and an `options` with no fields is an empty map. The author meant `input.options?.retries`, which is `int \| null`. |
| `report && report.verdict == "APPROVE"` | `bool \| null` | Null when there is no report. As a condition it works. As a result that must be `bool` it does not fit. |

**What a host expects of the result** (B8, D33). A host gives the checker an
expected result (2.5, row 1), and has two choices.

| The host | Passes | At check time | At run time |
| --- | --- | --- | --- |
| needs a real boolean | the type `bool` | The result must fit `bool`, or it is `result-mismatch`. Truthiness does not weaken this. | The value is `true` or `false`. |
| tests the result by truthiness | `condition` | The result is a tested position (2.8). Every type fits. A result that can never be falsy is `always-truthy`, and one that can never be truthy is `always-falsy`, on the result expression. Both are of the class `constant-condition` (2.13). | The host applies the language's own test with the binding's function (`05:A.3`; in Go, `risor.Truthy(v)`). It never writes its own list of falsy values. |

`condition` is this document's name, and `05`'s, for the second choice. It is
not a type and is not written in type text. It is the same rule that `unknown`
marks on a predicate's result (2.2). The workflow engine passes `condition` for
`when:` (D33). A host that passes nothing gets the inferred type and no test.

| `when:` | Type | Expected `bool` | Expected `condition` |
| --- | --- | --- | --- |
| `gate.items` | a list | `result-mismatch`. The host asked for a `bool`. | Accepted: true when there are items. |
| `len(gate.items) > 0` | `bool` | Accepted. | Accepted. |
| `!!gate.items` | `bool` | Accepted. | Accepted. |
| `report && report.verdict == "APPROVE"` | `bool \| null` | `result-mismatch`. | Accepted. This is the guard most authors write first. |
| `report != null && report.verdict == "APPROVE"` | `bool` | Accepted: the left side is a `bool`, so its falsy part is `bool`. | Accepted. |
| `report?.verdict == "APPROVE"` | `bool` | Accepted, and shorter: `==` takes null. | Accepted. |
| `started` | `Time` | `result-mismatch`. | `always-truthy`: the step would always run. |
| `gate` | a record with required fields | `result-mismatch`. | `always-truthy`. |

So with `condition`, a list or a string is accepted when it can be empty and
reported only when it never can.

**`bool(x)` and `!!x` both turn a value into a `bool`.** `06:2.8` has `bool` in
the core, beside `int`, `float` and `string`, because it reads as what it does.
`!!x` exists whatever the library does, and it is what a JavaScript author
writes (C6). The argument of `bool` is a tested position, as the operand of `!`
is, so `bool(started)` is reported as `always-truthy` just as `!!started` is.
This document argued for `!!x` alone, on the ground that one way to write a
thing is enough; the overview ruled for both. A comparison that says which
meaning is wanted, `len(xs) > 0` or `x != null`, is often the better text, and
the help for `result-mismatch` offers both.

| Compared | expr `Check` | Workflow `Load` | v3 |
| --- | --- | --- | --- |
| Stated promise | "A passing check does not prove Run succeeds" (`expr:docs/reference/spec.md:793-794`) (R) | No missing name or field, nil, or wrong kind (`workflow:documentation/definitions.md:785-787`) (R) | The four points above |
| Untyped values | Unchecked, silently | Outside, in prose (`:792-797`) | `dynamic` sites |
| Index, `1 / 0`, `int("abc")` | Unchecked | Outside, in prose (`:798-801`) | `index`, `divide`, `value` sites |
| A fractional index | Passes, fails at run time (X) | The same | `kind` |
| A number for an `integer` parameter | n/a | Outside (`:802-807`) | `arg-mismatch` |
| Calls in null tests (expr: nil) | Narrowed by text (`expr:check.go:131-135`) (R) | Inherited | Not narrowed |
| `a && b`, `a \|\| b` | Join(A, B), and Join(A without nil, B) (`expr:check.go:316-325`) (R) | Inherited | Join(Falsy(A), B), and Join(Truthy(A), B) |
| A test that can only go one way | Accepted: `check.go` has no such rule (R) | Accepted, except that `when:` "must be a boolean, statically and at run time" (`workflow:documentation/definitions.md:210-212`) (R) | `always-truthy`, `always-falsy`; an expected `bool` is `result-mismatch` |

| Remaining class | How to narrow it | Worth it? |
| --- | --- | --- |
| `dynamic` | Declare types; reject the class | Yes, built in |
| `index` | `xs?.[i]`, `first`, `last` and a pattern default are not sites | Yes, as host policy (open question 1) |
| `divide` | A non-zero literal divisor is not a site. A guard such as `n != 0 &&` or `n &&` is not tracked. | No further |
| `value` | Library entries that return `T \| null` where "not there" is meant; `try` | Yes, in `06` |
| `host` | An entry without `!` is not a site | Nothing more |
| `range` | Cannot be ruled out without range types | No (N6) |

### 2.15 Declaring an environment

An environment gives each variable a type, each function and method a signature,
the names of its opaque types, its named types, and the set of check classes its
check rejects (2.13; `05:A.2`). The portable forms of a type are type text and
JSON Schema (P5).

**Opaque types.** This is all the specification says about a host type that has
behaviour, so that a Go struct, a Rust type and a TypeScript class are all
covered:

| Matter | Rule |
| --- | --- |
| Declaration | An environment may declare opaque types by name. The name is the whole type: no fields, no structure. It fits only itself and `any`. |
| Methods | An environment may declare methods on an opaque type, each with a signature. Methods are declared, never discovered. |
| Use | A value of an opaque type can be bound, passed, put in a list or map, compared with `==`, tested, and used through its methods. Nothing else (`02:2.4`). It is always truthy (B5). |
| The binding's duty | It must be able to tell, for any host value, whether it is opaque and under which declared name. How (a Go type, a Rust type's id, a class, a brand) is its business. |
| Record or opaque | A host value is a record, read by its fields, or opaque, used by its methods. The specification has no value that is both. A binding presents each host type as one or the other. |

A function typed by host code (`05:A.4`) is outside the specification: its
description carries its declared signature, and only that is portable.

**Named types** (D40; `07:2.5`). A named type is a declaration of its own kind
in an environment. To this document it is an abbreviation and nothing more.

| Matter | Rule |
| --- | --- |
| Declaration | A name and a type. The type is given as type text or as a JSON Schema, as a variable's is (P5). |
| Name | A `Name` of 2.2. Named types and opaque types share one set of names, and a clash is the `declare` error `name-clash` (`05:A.2`). |
| Meaning | The name stands for the type. Fits, Join, Merge, the two parts, disjoint, conformance and printing all see the type and never the name. Two names for one structure are one type. Nothing is nominal: an opaque type is the only type with an identity. |
| A tagged union | A named type may be one: `Shape = {kind: "circle", r: float} \| {kind: "rect", h: float, w: float}`. It is the usual way to have one, since a program cannot declare a type. Written beside another record, the name gives its variants (2.2). A declared type with two records and no tag is `bad-type`. |
| Where it may be written | In the type text of any later declaration of the environment: a variable, a signature, another named type. And in a program, after `as` and `is` (2.2). Not as a `Recv`. |
| Order | A named type may use only names declared before it. So it cannot refer to itself, directly or through others. A reference that would close a cycle is the `declare` error `bad-type`, as a `$ref` cycle is. |
| No recursive type | It follows from the order. Replacing every name therefore ends, the depth of a type (2.17) is the depth with its names replaced, and every walk over a type or along a value ends at that depth. A tree or a thread of comments is written with `any` at the point of recursion. |
| Replacing is not copying | A name is a reference to one type. `B = {l: A, r: A}`, `C = {l: B, r: B}` and so on doubles the size of the tree form at each line and adds one to the depth. An implementation holds each named type once and shares it (open question 6). |
| From a JSON Schema | By the table below. The three options are given with each schema, not once for the environment, and each is off unless set. For a named type meant for `as` they stay off (`07:2.9`): the options describe what a host did to a value before the run, and data met inside a run was normalized by nobody. So an absent property is absent, a key the schema does not declare is allowed unless the schema forbids it, and no default is applied. |
| What a schema's name does not carry | The schema. `x as Order` tests the type of this table. Every keyword that constrains a value is dropped, as the last row of the table says, and so is the difference between `oneOf` and `anyOf` (D43). A host that wants the schema enforced runs its own validator in a host function. |
| Description | The environment's description lists named types by name, each as type text (`05:A.2`). |

**JSON Schema.** The mapping follows the engine's (`workflow:schema.go:439-545`)
(R). Differences from the engine are in bold. Three options exist because some
hosts normalize values before a run and some do not. A host sets them schema by
schema.

| Schema | Type |
| --- | --- |
| `{}`, `true`, or no `type`, `enum` or `const` | `any` |
| `string`, `boolean`, `null` | `string`, `bool`, `null` |
| `integer` | `int` **(engine: number)** |
| `number` | `float` **(engine: number)** |
| `type: [A, B]` | The Join; `"null"` adds null |
| `enum`, `const` | The Join of the types of the values `type` allows, each typed as the same literal expression would be. Strings give a literal set. **Numbers give `int` or `float`, with no set.** |
| `array` | `S[]` for `items: S`; without `items`, `any[]` |
| `object`, no `properties`, no `additionalProperties` | `map<any>` |
| `object` with `properties` | A record. Rest: `never` under `additionalProperties: false` or the option *closed*; S under `additionalProperties: S`; else `any`. |
| A property in `required` | `f: T` |
| A property not in `required` | `f?: T` **(engine: `f: T \| null`)**. Under the option *absent-is-null*: `f: T \| null`. |
| A property with a `default` | Under the option *defaults-applied*: `f: nonnull(T)`. |
| `$ref` | The schema it names in the same document. A cycle is an error. |
| `anyOf`, `oneOf` | Each branch is mapped by this table. If two or more branches give records that form a tagged union (2.1), the result is that union, joined with what the other branches give. Otherwise the Join of the branches, as before. A branch that is itself a tagged union gives its variants (09-8). |
| Any other keyword | Ignored: it constrains values, not types. OpenAPI's `discriminator` is one of these: the tag is found from the branches. |

**A tagged union from a schema** (`09:2.8`). Nothing new makes a branch a
variant: a property in `required` whose schema is a `const` string or an `enum`
of strings is already a required field with a literal set. `oneOf` and `anyOf`
give the same union, because no value matches two variants. Object branches with
no tag (a tag property outside `required`, or typed `string`) join into one
record and nothing is reported. A schema is written for another tool, so what it
cannot say maps to the wider type, as in every other row. Type text is written
for this language, so there the same mistake is an error (2.2). The three
options apply to each variant as to any record: *closed* makes each variant
exact.

**Who sets which option.**

| The schema types | *closed* | *absent-is-null* | *defaults-applied* | Why |
| --- | --- | --- | --- | --- |
| A variable that the host normalizes before a run, as the workflow engine does | set | set | set | The engine drops undeclared keys, fills absent properties with Go's nil, which a program reads as null, and applies defaults (`workflow:schema.go:636-698`) (R). It keeps today's types, except that integers are now `int`. |
| A named type that programs use with `as` | off | off | off | Data met inside a run was normalized by nobody (`07:2.9`). |
| The expected result of a config file | set | off | off | 12-5; `12:3.5`. Most schemas do not say `additionalProperties: false`, so without *closed* a misspelled key would pass. Nobody normalized the value, so the other two stay off, and an absent key is absent. The loader sets the option; the author of the schema changes nothing. |

A schema says more about truthiness than its author may expect. An `enum` of
strings without `""` and an object with a `required` property are never falsy,
so a test of one alone is `always-truthy`.

### 2.16 Conformance files

The checker's cases use the format of `05:A.11`: the same files for every
implementation (P4). Since D49 that format is also an author's format. A file is
a JSON array of cases, or one object with a header and a list `cases`, and a
case is the header with its own fields laid over it (`11:2.2`). `11` and
`05:A.11` own the format, the runner and its verdicts, and `risor test` runs the
files. This section says which fields a check case uses and what the checker's
files hold. The cases below are written without a header.

```json
{"name": "chain: an absent optional field does not skip the chain",
 "source": "input?.note.toUpperCase()",
 "env": "{input: {note?: string}}",
 "expect": {"phase": "check",
            "diagnostics": [{"code": "maybe-null", "span": [1, 1, 1, 12]}]}}

{"name": "truthiness: && gives its left side when that is falsy",
 "source": "report && report.verdict == \"APPROVE\"",
 "env": "{report: {verdict: \"APPROVE\" | \"REJECT\"} | null}",
 "expect": {"type": "bool | null", "sites": []}}

{"name": "expected result: a host that needs a bool",
 "source": "report && report.verdict == \"APPROVE\"",
 "env": "{report: {verdict: \"APPROVE\" | \"REJECT\"} | null}",
 "returns": "bool",
 "expect": {"phase": "check",
            "diagnostics": [{"code": "result-mismatch", "span": [1, 1, 1, 38]}]}}

{"name": "expected result: a host that tests by truthiness",
 "source": "report && report.verdict == \"APPROVE\"",
 "env": "{report: {verdict: \"APPROVE\" | \"REJECT\"} | null}",
 "returns": "condition",
 "expect": {"type": "bool | null", "sites": []}}

{"name": "truthiness: an opaque value is never falsy",
 "source": "started || fallback",
 "env": "{started: Time, fallback: Time}", "opaque": ["Time"],
 "expect": {"phase": "check",
            "diagnostics": [{"code": "always-truthy", "span": [1, 1, 1, 8]}]}}

{"name": "as: an untyped subject is a value site",
 "source": "(blob as Order).total",
 "env": "{blob: any}",
 "types": [{"name": "Order", "type": "{id: string, total: float, ...}"}],
 "expect": {"type": "float",
            "sites": [{"class": "value", "span": [1, 2, 1, 15]}]}}

{"name": "is: the failing side loses what the type accepts",
 "source": "v is int ? v + 1 : v.trim()",
 "env": "{v: int | string}",
 "expect": {"type": "int | string", "sites": []}}

{"name": "check classes: constant-condition taken out",
 "source": "started || fallback",
 "env": "{started: Time, fallback: Time}", "opaque": ["Time"],
 "reject": [],
 "expect": {"type": "Time", "sites": []}}
```

More cases, as one file with a header (D49). The header declares the environment
once:

```json
{"env": "{s: Shape, blob: any, xs: (int | null)[], orders: {lines: {qty: int}[]}[]}",
 "types": [
   {"name": "Shape", "type":
     "{kind: \"circle\", r: float} | {kind: \"rect\", h: float, w: float}"},
   {"name": "Order", "type": "{id: string, total: float, ...}"}],
 "cases": [
  {"name": "tagged: a field of one variant, before a test",
   "source": "s.r",
   "expect": {"phase": "check",
              "diagnostics": [{"code": "maybe-absent", "span": [1, 3, 1, 4]}]}},

  {"name": "tagged: a test of the tag narrows both branches",
   "source": "s.kind == \"circle\" ? s.r : s.w * s.h",
   "expect": {"type": "float", "sites": []}},

  {"name": "form: a list literal fits a tagged union through its form",
   "source": "[{kind: \"circle\", r: 1}, {kind: \"rect\", w: 2}]",
   "returns": "Shape[]",
   "expect": {"phase": "check", "diagnostics": [
     {"code": "result-mismatch", "span": [1, 26, 1, 46]}]}},

  {"name": "tagged: two records with no tag, after as",
   "source": "blob as {a: int} | {b: string}",
   "expect": {"phase": "check",
              "diagnostics": [{"code": "untagged-union", "span": [1, 9, 1, 31]}]}},

  {"name": "try: the error value is a record, and the body's site is dropped",
   "source": "try { blob as Order } catch (err) { err.path }",
   "expect": {"type": "(int | string)[] | {id: string, total: float, ...} | null",
              "sites": []}},

  {"name": "projection: one flat list, and an index read is a site once",
   "source": "orders[*].lines[0].qty",
   "expect": {"type": "int[]",
              "sites": [{"class": "index", "span": [1, 1, 1, 19]}]}},

  {"name": "filter narrows the element",
   "source": "xs.filter(x => x != null)",
   "expect": {"type": "int[]", "sites": []}}
 ]}
```

| Field | Meaning |
| --- | --- |
| `source`, `env`, `functions` | As in `05:A.11`. A check case needs signatures only, so a function is a signature text with no rows (`11:2.3`). |
| `opaque`, `methods` | Opaque type names, and method signatures in the `Method` text: `{"Stamp.format": "(layout: string) => string"}`. |
| `types` | Named types (2.15), as in `05:A.11`: a list, in the order declared, each a name with its type text or its schema. |
| `returns` | The expected result of 2.5, row 1: type text, or the word `condition`. Absent: the host expects nothing. Earlier revisions named this field `expected`. It was renamed because `expect` and `expected` differ by two letters and mean different things (ruling 3 of round 8; `11`, section 9). The term "expected result" is unchanged. |
| `reject` | The whole set of check classes the environment rejects (2.13). Absent: the default set, `["constant-condition"]`. A case that gives it gives every class, so strict mode is `["constant-condition", "dynamic"]` and `[]` rejects nothing. |
| `precondition` | `false` on a case that breaks a declared type on purpose, such as a run case that expects class `environment`. The runner then does not test the fixtures against their types first (`11:2.5`). Absent: it does. |
| `expect` | Every diagnostic, in the order of 2.13; or `{"type": "string \| null", "sites": [{"class": "index", "span": [1, 1, 1, 14]}]}`. A failure has one shape: `phase` and `diagnostics` (`11:2.4`). |

- **A field that is present is compared, and an absent one is not** (`11:2.4`).
  The suite's own cases state everything: for a check failure every diagnostic,
  each with its code, its span and its message, and for a result its type and
  its sites. The message is the catalog's (`10:2.9`), and it is compared whole.
  The cases shown in this section leave `message` out, because the wording is in
  `messages.json` and not in this document.
- A case need not state the class set. Most do not, and run under the default.
- A type is compared by parsing the expected text and comparing structure, so a
  case does not depend on printing. Separate cases test printing.
- A second file holds cases with no program: `{"join": [A, B], "is": C}`, and
  the same for `fits` (with whether it used any), `merge`, `truthy`, `falsy`,
  `disjoint`, `without`, `flat` and `print`, and `{"schema": {...}, "options":
  [...], "is": A}`. With D47 it covers each row of 2.3 for a tagged union, type
  text that is not tagged, in a declaration and after `as`, which key is the tag
  when two qualify, and schema branches that form a union and each way they fail
  to (`09`, section 4).
- A run case of `05:A.11` that also gives `sites` tests the promise: its error
  must match a site, or have class `range`, `limit` or `environment`. Run cases
  for `&&` and `||` supply each falsy value in turn (null, `false`, 0, `""`,
  `[]`, `{}`) and compare the result with the inferred type (promise 4).
- **A case that compares a class always checks first** (2.14; ruling 7 of round
  8). The class of a null, kind or field failure is defined by the sites, so a
  case that gives `class` in a run diagnostic is run only after its program was
  checked. A case that expects `environment` breaks a declared type on purpose
  and says `"precondition": false`.
- Check cases cover every row of the table of 2.5 for a fit through a form, at a
  result and at an argument, with every diagnostic and its span (`09`, section
  4; `12`, section 5); every row of the two tables of 2.7 for a projection
  (`13:6.4`); the rows of 2.8 and 2.9 for a tag test; each code of C as a case
  of a `match` on `err.code`, and a code that is not in C.
- The kinds of case an author never writes (`join`, `fits`, `print`, `schema`,
  the limit cases, and the `runner` cases of `11:2.5`, which hold a case file
  and the verdicts it must give) are run by the same runner and are documented
  with the suite (`11:2.9`).
- A third file holds the limit cases of 2.17: the deepest legal program and
  type, and one past each.
- A fourth file holds **conformance cases** (D39): a type, a value, whether the
  value conforms, the path of the first failure and the steps charged. `05:A.11`
  owns their format, as "a conformance case of the second kind", and says how a
  harness runs each one through the host's function, through `as` and through
  `is`. What they test is the table of 2.14.

```json
{"name": "conform: nothing is looked at under any",
 "conform": {"type": "any[]", "value": [1, "a", null, [[[]]]]},
 "expect": {"conforms": true, "steps": 1}}

{"name": "conform: a key whose value is null is present",
 "conform": {"type": "{id: string, note?: string}",
             "value": {"id": "a", "note": null}},
 "expect": {"conforms": false, "path": ["note"], "steps": 3}}

{"name": "conform: the tag is read first, then one variant",
 "types": [{"name": "Shape", "type":
   "{kind: \"circle\", r: float} | {kind: \"rect\", h: float, w: float}"}],
 "conform": {"type": "Shape", "value": {"kind": "rect", "w": 2}},
 "expect": {"conforms": false, "path": ["h"], "steps": 3}}
```

- `type` is the `Type` of 2.2, as it may stand after `as`. A whole number in the
  JSON of `value` is a whole number here, however it is written. The step counts
  of the first two are illustrative, as in `05:A.11`. The third is the count
  `09:2.7` gives: the map, the tag, `h`.
- A conformance case may give a `max` and expect a list of failures, for the
  host's second function (`10:2.11`, `05:A.11`). Its first entry must be the
  path the first-failure tests give.
- Each row of the conformance table has cases on both sides: a value that
  conforms and one that does not.
- Each row of the disjoint table has a pair that is disjoint and a pair that is
  not, in the second file. For every pair the suite says is disjoint, no
  conformance case may have a value that conforms to both types.
- A check case and a conformance case may share a `types` list, so that a named
  type is tested by structure in both: in Fits and printing, and in the walk.

### 2.17 Limits of the checker

A stack overflow ends a Rust process, and in JavaScript it comes at a depth the
engine chooses (R5). So every recursion the checker makes is bounded by a number
here, and passing it is the same result in every implementation. The Go checker
enforces the same numbers, though its stack would grow.

A **use** is a use of a `const`-bound lambda: a call by name, or its name as an
argument to a library function or method (2.11). There is no other kind since
D34.

| Limit | Number | What is counted | Passing it |
| --- | --- | --- | --- |
| Source nesting | 100 levels, counted as `02` 2.16 counts them (overview D31) | The levels open at a node in the program text. The parser enforces it, so the checker never sees deeper text. | a `parse` diagnostic |
| Expanded nesting | the same number | A use is checked by checking the lambda's body (2.11). The expanded nesting of a node in that body is its own nesting within the lambda, plus the expanded nesting of the use, plus 1 for the use itself, which is how `02` 2.16 counts a call against the depth limit. Elsewhere it is the node's nesting. | `check-limit` on the use that would pass it; the use has type `never` |
| Lambda body checks | 1,000 per program | Each use counts 1, and so does each use met while its body is checked. An implementation that keeps a check and reuses it (2.11) counts as if it had checked again. | `check-limit` on the use; the use has type `never` |
| Type depth | 64 | A type with no list and no record has depth 0. A list or a record adds 1 to the deepest of its element, field and rest types. A tagged union has the depth of its deepest variant, and adds no level of its own. A named type has the depth of the type it stands for. | For a declared type: an error of the `declare` phase (`05:A.6`). For a type written after `as` or `is`: `check-limit` on the type (2.5). For a type the checker builds: `check-limit` on the node, which then has type `never`. |
| Type text and schemas | 64 | Brackets of any kind, nested, in type text. Schemas nested in a JSON Schema, a `$ref` counting as one level. | an error of the `declare` phase |

- **What the two limits on uses are for, since D34.** They no longer end the
  check. Uses form a graph with no cycle (2.11), so a program with k
  `const`-bound lambdas has no chain of uses longer than k, and expanding every
  use is finite. Both numbers are still too large to leave alone. The expanded
  nesting can reach k plus the nesting at each use, which is more than a stack
  holds when k is in the thousands. The body checks can reach 2^k, when each
  helper uses the one before it twice. So the two limits stay, as the bound on
  the checker's stack and the bound on its work.
- **One counter covers the source and the uses.** The checker's stack is as deep
  as the expanded nesting, times a constant an implementation knows. A chain of
  helpers, each calling the next from inside a nested expression, adds up
  against the one number. Use depth times body nesting, which is what an
  unbounded checker would use, cannot occur.
- **What opens no level is checked by a loop** (`02` 2.16): the operands of a
  left-associative chain, the steps of a chain, the else branch of `? :`, an
  `else if` chain, the arms of a `match` and the `const` lines of a block. `if`
  (D35) adds the fourth of these and changes no bound. Along an else chain the
  checker carries two things forward and recurses on neither: the false facts of
  the tests so far, and the Join of the branches so far. Each test and each
  branch body is one level, as `02` 2.16 counts the parentheses and the braces.
- **`try` and a star change no bound.** The braces of the body and of the
  handler, and the parentheses of `catch (err)`, each open a level as `02` 2.16
  counts them (`10:2.2`). A star opens a level that lasts to the end of its
  projection (`13:5.2`), so a projection has at most 100 stars. The checker
  walks the reads of a projection in the loop that walks any chain.
- **A fit through a form** (2.5) recurses on the expression and on the type
  together, one level of each at a time. The source nesting bounds the first and
  the type depth the second.
- **Recursion over types** is in Join, Fits, Merge, flat, bind, disjoint,
  printing, the type text parser, the JSON Schema mapping and the conformance
  walk of 2.14. Each goes no deeper than the deeper of its two types, so 64
  bounds them all. A tagged union adds breadth and no depth: Fits and disjoint
  compare each variant of one with each variant of the other, so their work on
  two unions is the product of the two counts of variants. Nothing bounds the
  number of variants but the size of the type text (open question 6).
- **A conformance walk is as deep as its type, not as its value** (`07:2.7`). It
  enters a value only where the type has a list or a record, one level of the
  value for one level of the type, and it stops where the type is `any`. So an
  `as`, an `is` and the host's function recurse at most 64 times whatever the
  data. A body nested 10,000 deep, tested against `{meta: any, ...}`, is entered
  once. R5 is met with no new number. This rests on there being no recursive
  type (2.15).
- **The depth of a written type is tested where the type is read, in every
  evaluator.** `int[][]...` opens no bracket, so the source nesting of `02` 2.16
  does not bound it, and a short name may stand for a deep type. Replacing the
  names of a type after `as` or `is` and testing its depth belong to reading the
  type, not to checking the program. So does the test that two records in it
  form a tagged union (2.2). An implementation does all three whenever it
  compiles a program against an environment, with the rest of the checker or
  without it. `07` does not say this, and its bound on the walk needs it.
- **A type the checker builds gets deeper only at a list literal, a map literal
  and a call result.** Join, Merge and flat never give a type deeper than their
  inputs. A fact from `is` or from a tag test gives a path a type that was
  already tested. A projection gives a list no deeper than its receiver, or
  `any[]` (2.7). The narrowed result of `filter` is a call result. The type of a
  `catch` name has depth 2. So the depth test is in four places: those three,
  and the type of an `As` node.
- **Truthy and Falsy do not recurse.** They look at alternatives, not inside
  them.
- An implementation publishes the stack it needs at these numbers, and the limit
  cases of 2.16 run at them.

### 2.18 The previous spelling of type text

Earlier revisions of these documents wrote type text another way. D29 replaced
it with the grammar of 2.2, and D28 renamed what was built on the word "nil". No
implementation accepts the old forms. This table is for a reader of older notes.
Every rule of section 2 is stated on the structure of types, not on their text,
so no rule changed with the spelling.

| Meaning | Before | Now |
| --- | --- | --- |
| A list | `list<string>` | `string[]` |
| A list of lists | `list<list<int>>` | `int[][]` |
| The empty list's type | `list<never>` | `never[]` |
| A type that may be null | `string?` | `string \| null` |
| A literal set that may be null | `("fail" \| "pass")?` | `"fail" \| "pass" \| null` |
| A list that may be null | `list<int>?` | `int[] \| null` |
| A list of elements that may be null | `list<int?>` | `(int \| null)[]` |
| The absent value alone | `nil` | `null` |
| Unchecked | `dynamic`, `list<dynamic>` | `any`, `any[]` |
| A signature | `(s: string, n?: int) -> string !` | `(s: string, n?: int) => string !` |
| A callback | `(T, int) -> U` | `(item: T, index: int) => U` |
| A predicate | `(T, int) -> condition` | `(item: T, index: int) => unknown` |
| A rest parameter | `...rest: T` | `...rest: T[]` |
| A method | `list<T>.map: (f: (T, int) -> U) -> list<U>` | `T[].map: (f: (item: T, index: int) => U) => U[]` |
| The same in both | | `bool`, `int`, `float`, `string`, `never`, literal sets, records, `f?: T`, `...`, `map<T>`, opaque names, `Var`, `where`, `!` |

Names that changed with D28 and D29:

| Thing | Before | Now |
| --- | --- | --- |
| Diagnostic code | `maybe-nil` | `maybe-null` |
| The helper of 2.3 | nonnil(T) | nonnull(T) |
| The kind, the node of `02:2.3`, the failure kind of 2.14 | nil, `Nil`, `nil` | null, `Null`, `null` |
| The JSON Schema option of 2.15 | *absent-is-nil* | *absent-is-null* |
| The phrase in Fits | "used dynamic" | "used any" |
| The site class `dynamic` and the code `unchecked` | | Unchanged (2.14) |

Names and forms that changed with D46 to D51. Type text itself did not change: a
tagged union is written with the grammar of 2.2 as it was.

| Thing | Before | Now |
| --- | --- | --- |
| A fallback for a failure | `try(e, d)` | `try { e } catch { d }`, in parentheses where it is an operand (D48) |
| The part of a `try` whose sites are dropped | "the first argument of `try`" | "the body of a `try`" |
| The fields of node 26, `Try` | `x`, `fallback` | `body`, `name`, `handler` |
| The case field for the expected result | `expected` | `returns` (D49). `expect` keeps its name. |
| The command that runs the suite | `risor conform` | `risor test` (D49) |
| Two record types in one type text | Unsaid; `07` read them as one joined record | A tagged union, or an error (D47) |
| The map row of Join, Fits and disjoint | One row | Two: a record against a record, and the row for a tagged union |
| The span of `result-mismatch` and `arg-mismatch` | The result expression; the argument | Each place of failure (2.5) |
| A diagnostic's message | Not specified | The catalog's, `messages.json` (D48) |

## 3. Worked examples

```
{input:   {role: "admin" | "member" | "owner", note?: string,
           tags: string[], priority: int | null,
           options: {retries?: int, backoff?: string} | null},
 gate:    {failed: int, score: float,
           items: {name: string, status: "fail" | "pass",
                   owner: string | null, ms: float}[]},
 report:  {verdict: "APPROVE" | "REJECT", body: string} | null,
 scores:  map<float>,
 event:   {action: string, pull_request?: {number: int, head: {sha: string}}, ...},
 blob:    any,
 s:       Shape,
 shapes:  Shape[],
 started: Time}                  Time.format: (layout: string) => string
                                 Order = {id: string, total: float, ...}
                                 Shape = {kind: "circle", r: float}
                                       | {kind: "rect", h: float, w: float}
```

Library signatures are `06`'s as I understand them: `len`, `int` (`!`), `round`,
`math.sqrt` (`!`), `fail` (`!`, with result `never`), `string.toUpperCase`,
`string.toLowerCase`, `T[].filter` (a predicate), `T[].map`, `T[].reduce`,
`T[].sum`, `map<T>.entries`, `map<T>.values`.

| # | Expression | Type, or diagnostics | Sites |
| --- | --- | --- | --- |
| 1 | `gate.failed > 0` | `bool` | |
| 2 | `input.role in ["admin", "owner"]` | `bool` | |
| 3 | `gate.items.filter(t => t.status == "fail").map(t => t.name)` | `string[]` | |
| 4 | `report.verdict` | `maybe-null` 1:1-1:7 | |
| 5 | `report?.verdict ?? "REJECT"` | `"APPROVE" \| "REJECT"` | |
| 6 | `report != null && report.verdict == "APPROVE"` | `bool` | |
| 7 | `report && report.body` | `string \| null`: null when there is no report, else the body. The left side narrows `report`. | |
| 8 | `report?.verdict.toLowerCase()` | `string \| null`: the one `?.` skips the rest of the chain | |
| 9 | `input.note.toUpperCase()` | `maybe-absent` 1:7-1:11 | |
| 10 | `input?.note.toUpperCase()` | `maybe-null` 1:1-1:12: an absent `note` is null for that step, and the chain goes on | |
| 11 | `input?.note?.toUpperCase() ?? ""` | `string` | |
| 12 | `event?.pull_request?.head.sha` | `string \| null` | |
| 13 | `gate.items[0].name` | `string` | `index` 1:1-1:14 |
| 14 | `gate.items?.[0]?.name` | `string \| null` | |
| 15 | `gate.items[len(gate.items) / 2]` | `kind` 1:12-1:31: an index must be a whole number; write `int(len(gate.items) / 2)` | |
| 16 | `gate.score / len(gate.items)` | `float` | `divide` 1:1-1:29 |
| 17 | `scores[input.role]` | `maybe-absent` 1:8-1:18. `scores?.[input.role] ?? 0` is `float`. | |
| 18 | `gate.failed > 0 ? gate.items : []` | `{ms: float, name: string, owner: string \| null, status: "fail" \| "pass"}[]`: `never[]` joins away | |
| 19 | `match input.role { "admin" \| "owner" => "write", "member" => "read" }` | `"read" \| "write"`: every role is listed, so no `_` | |
| 20 | `match input.role { "admin" => 1, "guest" => 0 }` | `non-exhaustive` 1:1-1:48 (no "member", "owner") and `never-equal` 1:34-1:41 | |
| 21 | `match input.role { "admin" => 1, _ => 0, "owner" => 2 }` | `unreachable-arm` 1:42-1:54 | |
| 22 | `match input.priority { null => "normal", 1 \| 2 => "high", _ => "low" }` | `"high" \| "low" \| "normal"`. `_` is required: `int` is not finite. | |
| 23 | `match { report == null => "none", _ => report.verdict }` | `"APPROVE" \| "REJECT" \| "none"`: the false fact of the first condition narrows `report` | |
| 24 | `gate.items.map(t => { const slow = t.ms > 500; {name: t.name, slow} })` | `{name: string, slow: bool}[]` | |
| 25 | `{ const {retries = 3, backoff = "1s"} = input.options ?? {}; {retries, backoff} }` | `{backoff: string, retries: int}` | |
| 26 | `{ const {role, ...rest} = input; rest }` | `{note?: string, options: {backoff?: string, retries?: int} \| null, priority: int \| null, tags: string[]}`: exact stays exact | |
| 27 | `{ const {action, ...rest} = event; rest }` | `{pull_request?: {head: {sha: string}, number: int}, ...}`: open stays open | |
| 28 | `{ const {note, ...rest} = input; rest }` | `maybe-absent` 1:10-1:14; write `note = null` or another default | |
| 29 | `{ const [first, second = ""] = input.tags; first }` | `string` | `index` 1:10-1:15 |
| 30 | ``scores.entries().map(({key, value}) => `${key}=${value}`)`` | `string[]` | |
| 31 | `gate.items.reduce((m, t) => {...m, [t.name]: t.status}, {})` | `map<"fail" \| "pass">`: the seed rule widens `{}` once | |
| 32 | `2 ** 10`, `2 ** gate.failed`, `2 ** gate.score` | `int`; `float`; `kind` 1:6-1:16 | |
| 33 | `try { int(report?.body ?? "") } catch { 0 }` | `int` | none: the body's `value` site is dropped |
| 34 | `const pct = (n, digits = 0) => round(n * 10 ** digits); pct(gate.score) + pct(gate.failed, 2)` | `int`: the body is checked once per use | none: `round` can fail only with class `range` |
| 35 | `const name = t => t.nmae; gate.items.map(name)` | `no-field` 1:21-1:25 | |
| 36 | `math.sqrt(gate.score)` | `float`. `math` in any other position is `not-a-value`. | `value` 1:1-1:22 |
| 37 | `started.format("iso")`, `started == started` | `string`; `bool`: an opaque value equals itself (S11) | |
| 38 | `blob.x.y + 1` | `any`. Rejecting the class `dynamic`: `unchecked` 1:1-1:5 only, since the type is then `never`. | `dynamic` 1:1-1:5, 1:1-1:7, 1:1-1:9 |
| 39 | `gate.failed + " failures"` | `kind` 1:1-1:26 | |
| 40 | `gate.failed / 0` | `zero-divisor` 1:15-1:16 | |
| 41 | `inptu.role` | `undefined-name` 1:1-1:6 | |
| 42 | `report && report.verdict == "APPROVE"` | `bool \| null`. With an expected `bool`: `result-mismatch` 1:1-1:38, and example 6 is the `bool`. With an expected `condition`: accepted (2.14). | |
| 43 | `report?.verdict == "APPROVE"`, `!!report`, `!!gate.items` | `bool`, three times | |
| 44 | `input?.note \|\| "none"` | `string`: the truthy part of `string \| null` is `string` | |
| 45 | `input.priority \|\| 3` | `int`. A priority of 0 is replaced too; `input.priority ?? 3` keeps it. | |
| 46 | `gate.failed \|\| "none"` | `int \| string` | |
| 47 | `input.options && input.options?.retries` | `int \| {} \| null`: the falsy part of `options` is null or an empty map. `input.options?.retries` is `int \| null`. | |
| 48 | `gate.items && gate.items[0].name` | `string \| never[]`: the empty list comes back as itself | `index` 1:15-1:28 |
| 49 | `gate.items.filter(t => t.owner).map(t => t.name)` | `string[]`: the predicate is a `string \| null` | |
| 50 | `gate.items.filter(t => t.status)` | `always-truthy` 1:24-1:32: a status is never empty | |
| 51 | `started ? started.format("iso") : ""` | `always-truthy` 1:1-1:8 | |
| 52 | `input.role \|\| "member"` | `always-truthy` 1:1-1:11 | |
| 53 | `null \|\| gate.failed`, `!started` | `always-falsy` 1:1-1:5; `always-truthy` 1:2-1:9 | |
| 54 | `match { !report => "none", report.verdict == "APPROVE" => "go", _ => "stop" }` | `"go" \| "none" \| "stop"`: where `!report` was falsy, `report` is not null | |
| 55 | `blob \|\| "x"`, `blob ? 1 : 2` | `any`; `int` | none: a test cannot fail |
| 56 | `if (report) { report.verdict } else { "none" }` | `"APPROVE" \| "REJECT" \| "none"`: the test narrows `report` in its branch, as in `report ? report.verdict : "none"` | |
| 57 | `if (!report) { "none" } else if (report.verdict == "APPROVE") { "go" } else { "stop" }` | `"go" \| "none" \| "stop"`: checked as example 54 is. Where `!report` was falsy, `report` is not null in every later branch | |
| 58 | `if (gate.failed > 0) { const n = gate.failed; n * 2 } else { 0 }` | `int`: a branch is a scope, and `n` ends with it | |
| 59 | `if (started) { 1 } else { 2 }` | `always-truthy` 1:5-1:12 | |
| 60 | `const twice = (f, x) => f(f(x)); twice(n => n + 1, 2)` | `call-of-parameter` 1:25-1:26 and 1:27-1:28; `lambda-to-lambda` 1:40-1:50 (D34) | |
| 61 | `const inc = n => n + 1; gate.items.map(t => inc(t.ms))`, `[1, 2].map(inc)` | `float[]`; `int[]`: a `const`-bound lambda called by name, and passed to a library method | |
| 62 | `(blob as Order).total * 2` | `float`: `Order` is the named type above | `value` 1:2-1:15 |
| 63 | `try { blob as Order } catch { null }` | `{id: string, total: float, ...} \| null`: printed by structure, never as `Order` (D44) | none: the body's site is dropped |
| 64 | `if (blob is Order[]) { len(blob) } else { 0 }` | `int`: `blob` is a list of orders in the first branch and still `any` in the second | none: `is` cannot fail, and `len(blob)` is not a `dynamic` site there |
| 65 | `gate.score as int` | `int`: a `float` into an `int` without rounding. It fails unless the number is whole | `value` 1:1-1:18 |
| 66 | `gate.failed as float`, `[] as string[]` | `float`; `string[]`: the type surely fits, so a type is stated and nothing can fail | none |
| 67 | `"42" as int`, `gate.failed as string` | `never-conforms` 1:1-1:12; `never-conforms` 1:1-1:22 | |
| 68 | `gate.failed is float`, `if (gate.failed is string) { 1 } else { 2 }` | `always-truthy` 1:1-1:21; `always-falsy` 1:5-1:26 | |
| 69 | `const v = gate.failed \|\| "none"; if (v is int) { v + 1 } else { v.trim() }` | `int \| string`: where the test failed, `v` is a `string` | |
| 70 | `input.role is "admin" \| "owner" ? "write" : input.role` | `"member" \| "write"`: the failing side loses the two strings | |
| 71 | `blob is {x: {y: int}, ...} && blob.x.y + 1` | `bool \| int`: `false` when the test fails, else the sum. The reads are checked, and are not sites | |
| 72 | `blob as Ordr` | `undefined-name` 1:9-1:13 | |
| 73 | `s.kind == "circle" ? 3.14159 * s.r ** 2 : s.w * s.h` | `float`: the test of the tag narrows `s` to one variant in each branch (D47) | |
| 74 | `s.r`, `s?.r ?? 0`, `s.color` | `maybe-absent` 1:3-1:4; `float`; `no-field` 1:3-1:8: before a test, `s` reads as its flat record | |
| 75 | `match s.kind { "circle" => s.r * 2, "rect" => s.w + s.h }` | `float`: every tag is listed, so no `_`, and each arm has its variant | |
| 76 | `s \|\| started` | `always-truthy` 1:1-1:2: every variant requires its tag | |
| 77 | `blob as {a: int} \| {b: string}` | `untagged-union` 1:9-1:31 | |
| 78 | `shapes.map(x => x.kind == "circle" ? {...x, r: 2} : x)` | `{h?: float, kind: "circle" \| "rect", r?: int, w?: float}[]`: two records join into one. It does not fit `Shape[]` (open question 9) | |
| 79 | With an expected `Shape[]`: `[{kind: "circle", r: 1}, {kind: "rect", w: 2}]` | `result-mismatch` 1:26-1:46: the second element has no `h`. With `h: 3` added it is accepted, though its inferred type is one joined record: the fit goes through the form (2.5) | |
| 80 | With an expected `int`: `gate.failed > 0 ? 1 : "a"` | `result-mismatch` 1:23-1:26, on `"a"` and not on the whole result | |
| 81 | `try { blob as Order } catch (err) { err.path }` | `(int \| string)[] \| {id: string, total: float, ...} \| null` (D48) | none: the body's site is dropped |
| 82 | `try { int(blob) } catch (err) { err.code == "not-conformin" ? 0 : fail(err.message) }` | `never-equal` 1:33-1:60: `code` is a literal set, and that string is not in it | |
| 83 | `try { blob.x } catch (err) { err.stack }` | `no-field` 1:34-1:39: the error value is an exact record of four fields | |
| 84 | `(try { int(blob) } catch { 0 }) > 10` | `bool`. As an operand a `try` is in parentheses | none: the `dynamic` and the `value` site of the body are dropped |
| 85 | `try { 1 } catch (err) { fail(err.message, err.data) }` | `int`: the Join of `int` and `never`. A body that cannot fail is not reported | `value` 1:25-1:52: a site in the handler is kept |
| 86 | `shapes.filter(x => x.kind == "circle").map(c => c.r).sum()` | `float`: `filter` narrows the element to the circle (D51) | |
| 87 | `gate.items.map(t => t.owner).filter(o => o != null)` | `string[]`. Before D51: `(string \| null)[]` | |
| 88 | `shapes[*].kind`, `gate.items[*].ms.sum()` | `("circle" \| "rect")[]`; `float`: the method call ends the projection and receives the list | |
| 89 | `gate.items[*].name.toUpperCase()` | `no-method` 1:20-1:31: the receiver is a `string[]`. Write `gate.items.map(t => t.name.toUpperCase())` | |
| 90 | `scores[*]` | `kind` 1:1-1:7: a star needs a list. Write `scores.values()[*]` | |
| 91 | `blob.items[*].tags[*]` | `any[]` | `dynamic` 1:1-1:5, 1:1-1:11, 1:1-1:14, 1:1-1:19 |
| 92 | `r"\d+"`, `` `none` `` | `"\\d+"`; `"none"`: each is a `String` node, whatever form wrote it (D46). A literal set is printed in its JSON form | |

**How these are shown.** Since D48 a message is the catalog's (`messages.json`;
`10:2.9`) and the layout is one of three specified forms (`10:2.10`). The block
below is the detail form. Its message lines are stand-ins: the wording of each
template is in the catalog and not in this document. They are written as a
template can be, with typed holes and no knowledge of the one case, so they are
plainer than the sentences earlier revisions showed here. The closer wording is
in `help`, which is not specified. A type in a message is cut at 64 code points
by the catalog's rule, as the third one shows.

```
error[always-truthy] 1:1-1:11: this test always passes
    input.role || "member"
    ^^^^^^^^^^
  help: input.role is "admin" | "member" | "owner", so || "member" is never used
error[always-truthy] 1:24-1:32: this test always passes
    gate.items.filter(t => t.status)
                           ^^^^^^^^
  help: t.status is never empty. Compare it with a value: t.status == "pass"
error[result-mismatch] 1:1-1:11: expected bool, found {ms: float, name: string, owner: string | null, status: "fail...
    gate.items
    ^^^^^^^^^^
  help: write len(gate.items) > 0, or !!gate.items
error[maybe-null] 1:21-1:28: this may be null
    gate.items.map(t => t.owner.toUpperCase())
                        ^^^^^^^
  help: write t.owner?.toUpperCase(), or (t.owner ?? "").toUpperCase()
error[non-exhaustive] 1:1-1:48: no arm for "member", "owner"
    match input.role { "admin" => 1, "guest" => 0 }
    ^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^
  help: add the cases, or a default arm: _ => ...
error[maybe-absent] 1:10-1:14: field note may be absent
    { const {note, ...rest} = input; rest }
             ^^^^
  help: give it a default in the pattern: {note = null, ...rest}
error[maybe-absent] 1:3-1:4: field r may be absent
    s.r
      ^
  help: only the variant with kind "circle" has r. Test it: s.kind == "circle"
error[no-field] 1:6-1:12: no field fialed
    gate.fialed
         ^^^^^^
  help: did you mean failed?
error[call-of-parameter] 1:25-1:26: f holds a value and cannot be called
    const twice = (f, x) => f(f(x));
                            ^
  help: a function is passed only to a library function such as map
error[result-mismatch] 3:3-3:10: key max_bod is not in the type
      max_bod: 8 * mb,
      ^^^^^^^
  help: did you mean max_body?
```

The last is the case D50 was written for: a config file whose host expects an
exact record with the fields `listen` and `max_body`. Before the rule of 2.5 its
span was the whole file.

## 4. Limits (C5)

Static cost estimation stays out. The narrower question: can the checker cheaply
tell a host "this program's work is bounded by a constant" from "it depends on
the size of these inputs"?

| Point | Answer |
| --- | --- |
| Can it? | Yes. With no loops and no recursion in 3.0 (E6, X3), work is constant unless the program has a node whose cost grows with a value (`02:2.14`): a library call that visits elements, a star, `==`, `in`, `<` on strings, a spread, a rest pattern or a template part, applied to a list, map or string, and an `as` or `is` whose type has a list or a record. A walk that flags those nodes by their static types is about 40 lines, given a "grows" mark on each entry of `06`'s cost table. |
| Should it, in 3.0? | No. The budget runs in any case, because only it is exact, so the flag removes no mechanism. It would add a column to the library's specification and a field to every conformance case. The one use I can name is an editor hint ("this depends on the size of `fetch.tests`"), which can be added later without changing anything here. |

The checker's own work and stack are bounded by 2.17.

## 5. The Go implementation

Everything here is a note on one implementation.

**Where the types live.** `05:B.2` puts `Type`, `ParseType`, `Join` and `Fits`
in the root package (S5). The engine reads expr's `Kind`, `Optional`,
`Variants`, `Enum`, `Elem`, `Fields`, `Absent` and `Open` fields directly
(`workflow:typecheck.go:464`, `:625`, `:707-712`, `:849-868`, `:1085-1140`) (R).
`Type` needs an accessor for each (`Kinds`, `WithoutNull`, `Truthy`, `Falsy`,
`Strings`, `Elem`, `Fields`, `Rest`), so the engine's port rewrites field reads,
not logic. With D47 it gains two, for the tag and the variants of a tagged
union, and `Fields` and `Rest` answer for the flat record (`05:B.2`). So code
that reads a type as one record, as the engine does, keeps working on a union.

**What the evaluator now needs of this section** (D40). Before `as` and `is`, a
type existed only while a program was checked. Now three things belong to the
core that every evaluator carries, with no dependency on the checker: `Type`,
`ParseType` with the lookup of named types, and the conformance walk of 2.14. An
`as` or an `is` compiles its type to a test once, when the program is compiled
(`04`, Part B), and `Conform` (`05`) is the same walk called from the host.
Join, Fits, Merge, the two parts and disjoint stay with the checker. The walk
reads a host value through the evaluator's read barrier, so it has no reflection
of its own.

D47 adds two things to that core: the test that the records of a written type
have a tag, which is part of `ParseType`, and a union node in the compiled test,
which holds the tag and a table from tag string to variant (`04:B.14`). flat,
hit, all and the fit through a form stay with the checker. D48 adds one input to
the checker and none to the core: the literal set C of the error value (2.5) is
generated from `messages.json` with the messages, and an evaluator builds the
value with no type.

**Types from Go types** (`TypeOf`, `05:B.7`): a convenience outside the
specification (P5). Its result prints as type text.

| Go | Type | Note |
| --- | --- | --- |
| `bool`, `string` | `bool`, `string` | |
| every integer type | `int` | A value outside the whole range is the run error `number-range`, of class `environment`, where it is read (2.14). |
| `float32`, `float64` | `float` | A NaN or an infinity, the same. |
| `[]T`, `[N]T`, `map[string]T` | `T[]`, `map<T>` | A nil slice or map should read as empty, not null, or every Go slice field becomes nullable. `05` decides. A test does not depend on the answer: null and empty are both falsy (B5). `len(xs)` and `xs.map(f)` do. |
| `struct` | An exact record of its exported fields | Field names by `05:B.7`. A field is required, except by the next row. A struct with a required field is never falsy, and a test of one alone is `always-truthy`. |
| A field whose `json` tag has `omitempty` or `omitzero` | An optional field, `f?: T` | New with D50 (`12:3.5`). Before it every field was required, so a type taken from a Go struct could not say that a config file may leave a key out. The tag is the one a Go host already writes to say so. |
| A type that implements `encoding.TextUnmarshaler` and has no registered method | `string` | New with D50 (`12:3.5`). A `time.Time` field then takes RFC 3339 text, and the decode gives the Go value. With a registered method the type is opaque, by the row below. |
| `*T` | `T \| null` | |
| `any` | `any` | |
| A type with a registered method | An opaque type | The host names it: `time.Time` becomes `Time`. |
| Anything else | An error | The host declares the type itself. |

- **How an optional field is read going in** is `05:B.7`'s to say: whether a
  zero value under `omitempty` is presented as an absent key. The type is sound
  either way, because a key that is present conforms to `f?: T` when its value
  conforms to T.
- **`TypeOf` gives no tagged union.** Go has no such type. A host that wants one
  writes type text or a JSON Schema (`09`, section 5). An interface whose
  implementations are the variants is a later convenience of this binding (P5).
- **A struct is exact,** which is what a config file wants: a misspelled key is
  a check error at the key (D41, 2.5).

**What the checker hands to the evaluator:** side tables keyed by AST node, as
in `go/types.Info`. The checker always runs and is the only pass that resolves
names.

```go
type Info struct {
    Result Type
    Types  map[ast.Node]Type      // a lambda body: the Join over its checks
    Uses   map[*ast.Ident]Binding // variable, const name, parameter, lambda
    Calls  map[ast.Node]Callee    // function, method or lambda, resolved
    Hints  map[ast.Node]Hint      // "both operands are int", "struct field 3"
    Sites  []Site
}
```

| Table | Contract |
| --- | --- |
| `Uses`, `Calls` | Binding: namespaces and lambdas are resolved here and nowhere else. |
| `Hints` | Optional. An evaluator that ignores them is the reference. A hint only picks the first path to try, because a declared type is a promise the evaluator does not trust (`05:A.3`). A type made by `TypeOf` remembers its `reflect.Type`, so a hint can carry a struct field index; that is not a nominal type and changes no rule. The gain (I) is in field plans for structs, callees resolved once, and tests whose operand is known to be `bool`, which skip the switch on the value's kind. It is not in arithmetic on JSON data. |

**Stack.** A goroutine's stack grows, so nothing here can crash the process. The
Go checker still counts expanded nesting and type depth (2.17), because the same
programs must be rejected everywhere.

## 6. The TypeScript and Rust implementations

All (I). Nobody has written a checker in either. Each table says what that
checker faces that the Go one does not.

### 6.1 TypeScript

| Matter | Consequence |
| --- | --- |
| No reflection | An environment comes from type text, JSON Schema or a description (`05:A.2`). The type-text parser is on the main path, not a convenience. |
| No structural equality or hashing of objects | Memo keys and type equality need the printed text or interned types. The printing rule of 2.2 serves. |
| Strings are UTF-16 | Spans must be converted to code point columns (T5). Literal sets and record keys must be sorted by code point, not with the default comparison (`02:8.2`). |
| One number type | `Number.isSafeInteger` types a literal. Nothing else in the checker computes. |
| Truthiness | The parts of 2.3 are the specification's, not the host language's. `Boolean([])` and `Boolean({})` are true in JavaScript and both values are falsy here, so no rule of the checker or the evaluator may lean on a JavaScript condition. |
| Opaque types | A value is recognized by a class or a brand the host registers when it creates the environment. Without that the binding cannot answer "is this a `Time`". |
| It runs in the browser | `05:A.12` puts completion and hover there, so the checker ships to editors. Its size is download size, and it must give types for a program with `Bad` nodes. |
| Stack depth | A deep expression overflows the stack sooner than in Go, at a depth the engine chooses, so the overflow cannot be the limit (R5). The checker counts expanded nesting and type depth as 2.17 says, and its limit cases run in the smallest stack a supported engine gives. |
| A function typed by host code | The rule returns type text (`05`, Part C), which is parsed at each call site. |
| Types at run time | The evaluator is written long before the checker (overview 5.2, steps 4 and 9), and since D40 it needs the type text parser, a form of a type and the walk of 2.14 from its first day. They are a module of the evaluator that the checker later imports, not the reverse. `Number.isSafeInteger` is the whole test for `int`. Keys are visited in code point order, not in the order a JavaScript object gives (`07:2.3`). |
| Tagged unions | The test for a tag is in that module, with the parser. In the walk the tag is one own-property read and the table from tag string to variant is a `Map`. A TypeScript union type is erased, so a host writes type text or a schema, as for every type. |
| The class of a run error | Until the checker exists, a null, kind or field failure is class `dynamic`, by the rule of 2.14 for a program that was not checked. The suite's cases that compare a class wait for the checker. |
| The error value's type | Generated from `messages.json` with the message table, as a constant list of the caught codes. The checker ships to editors, so the list is download size: a few hundred bytes. |

### 6.2 Rust

| Matter | Consequence |
| --- | --- |
| How a type is held | A type is a set with at most one alternative per kind, so the natural shape is a struct with a slot per kind, inside a two-case enum: `Any`, or `Set { null, bool, number, string, list, map, opaque }`. `number` is `None`, `Int` or `Float`. `string` is `None`, `Any` or a sorted set. `list` holds the element type. `map` is `None`, a record, or a tagged union: a sorted list of variants with the tag and the flat record, computed once (2.1). `never` is the set with every slot empty. |
| Interned, not reference-counted | Recommended: each distinct type is stored once in a table, and a type is a 32-bit index into it. Equality is then one integer comparison. Join and Fits memoize on a pair of indexes. A type is `Copy`, so the checker carries no lifetimes and clones nothing. `Info` is vectors indexed by node number. The environment owns a frozen table and is `Send + Sync` (`05`, Part D). Each check adds its own types in a table that is dropped when the check ends. The alternative is `Rc` (`Arc` in the environment): less code, but equality walks the structure and every Join clones. |
| No garbage collector is needed | A type cannot refer to itself: there are no aliases and no recursive types, and a `$ref` cycle is an error (2.15). Scopes are a stack. Facts are a small set of paths copied at a branch. Nothing the checker builds outlives the check except the result, which is data (overview R3). |
| No reflection is needed | Nothing in section 2 looks inside a host value. The checker takes source text and an environment's declarations, and gives diagnostics, a type and sites. The one rule that reads a value, "a value conforms to a type" (2.14), is used by tests and by the evaluator's read barrier, and goes through the `Host` trait (`05`, Part D). |
| Recursion over the source | A stack overflow ends the process. One counter in the checker's state is raised on entry to `synth` and to a lambda body check, and compared with the maximum of 2.17. The crate states the stack it needs at that number, and a test checks the deepest legal program on a thread with the default 2 MiB stack. |
| Recursion over types | Join, Fits, Merge, bind, the printer, the type text parser and the schema mapping recurse on types. Type depth is at most 64 (2.17), tested where a type is built, so none of them needs a counter of its own. Dropping a table of interned types is a loop, not a recursion (overview R6). |
| Sorting | Byte order of UTF-8 is code point order, so literal sets and record keys sort with the built-in comparison. This is free in Rust and in Go, and is work in TypeScript. |
| Spans | Source is UTF-8, so a column in code points (T5) is a count from the start of the line. Keep line starts, and count only for a diagnostic that is reported. |
| Number literals | An `f64`. A literal is `int` when it has no fraction and its magnitude is at most 2^53 − 1. |
| Opaque types | The host registers a Rust type under a name when it builds the environment, and the binding tells values apart by `TypeId` (`05`, Part D, `View::Opaque`). |
| Errors | Diagnostics are values pushed to a list. `check` never fails and never panics on any input (`05`, Part D). |
| Types at run time | The same split as in TypeScript: the parser of type text, the table of types and the walk are in the evaluator's crate, and the checker builds on them. The types a program writes after `as` and `is` are interned in the compiled program's own table, beside the environment's frozen one. The walk takes a `View` and a type index, recurses at most 64 deep (2.17), and allocates only for the message of a failure. It is the function `conform` of `05`, Part D. |

**Types from Rust types.** `#[derive(risor::Host)]` (`05`, Part D) also gives
the type of the struct, as type text. It is this binding's convenience, as
reflection is Go's (P5). The portable forms stay type text and JSON Schema, and
a Rust host may use only those.

| Rust | Type | Note |
| --- | --- | --- |
| `bool`, `String`, `&str` | `bool`, `string` | |
| every integer type | `int` | A value outside the whole range is the run error `number-range`, of class `environment`, where it is read (2.14), or the field is presented as text by an attribute (`05`, Part D, open). |
| `f32`, `f64` | `float` | A NaN or an infinity, the same. |
| `Vec<T>`, `[T]`, `[T; N]` | `T[]` | |
| `HashMap<String, T>`, `BTreeMap<String, T>` | `map<T>` | |
| `Option<T>` | `T \| null`; with the attribute `absent_if_none`, an optional field `f?: T` | |
| a struct with the derive | An exact record of its fields | A field is required, except by the next row. Never falsy if it has a required field (2.3). |
| a field marked `#[serde(default)]` | An optional field, `f?: T` | New with D50 (`12:3.5`) (I). It is the mark a Rust host already writes for a key that a file may leave out. Before it every field was required. |
| a Rust `enum` | Not derived in 3.0 | A serde enum with an internal tag is the shape of a tagged union (`09`, section 6). Deriving one is a later convenience of this binding (P5). The host writes type text or a schema. |
| `serde_json::Value` | `any` | |
| a type registered with methods | An opaque type | |
| anything else | A compile error in the host's own build | The trait is not implemented. Go finds the same mistake when the environment is built. |

Two differences from Go follow from doing this at compile time. The derive costs
nothing at run time. And for a derived type the precondition of 2.14 holds by
construction, because the type and the way the value is read come from the same
macro.

### 6.3 One checker, shared (overview R9)

The plan is three native checkers. The fallback exists because of what a checker
is: it takes text and gives data.

| Point | Detail |
| --- | --- |
| Why the checker can be shared and the evaluator cannot | Parse, check, format and editor assistance never touch a host value. The argument against one shared core is "host values read in place", and it does not apply to them. |
| The route | The Rust checker built for WebAssembly, carried by the TypeScript package. In goes the source text and the environment's description (`05:A.2`). Out come the diagnostics, the type as text and the sites. The conformance format of 2.16 is already that interface. |
| What it needs that 3.0 does not have | The evaluator takes name resolution from the checker (`Uses`, `Calls`). A shared checker returns those as data keyed by node number, so the specification would have to fix how nodes are numbered. Nothing else in `Info` is more than data. Since D40 a shared checker no longer spares an evaluator the type text parser: `as` and `is` need it at run time. |
| What it costs | Download size in a browser editor, against about 2,900 lines of TypeScript. A second toolchain in the TypeScript package's build. |
| Who has no such route | A Go host, without a WebAssembly runtime as a dependency. |
| What keeps it open | Every input and output of the checker is text or plain data in this document, and no rule depends on a host value. That is true for other reasons (P4, P5), so the fallback costs nothing to keep. |

It is the one large part that need not be written three times. If the third
checker lags, this is the way to ship (D27). It is not the plan.

## 7. Testing

| Layer | What |
| --- | --- |
| Conformance files (2.16) | The rules of section 2, at least one case per table row. Every implementation runs them. They replace expr's tables in `check_test.go` (501 lines) (R), whose cases are rewritten to v3 syntax. |
| Soundness fuzzing, in Go | Generate an environment type, values that conform (null, missing keys, empty lists, empty maps, empty strings, zero, `false`, whole numbers at the range edge), and a program, some of it careless. If it checks, run it on every value: the four points of the promise must hold. The generator is type-directed, like the engine's (`workflow:soundness_fuzz_test.go:346`) (R). |
| The parts | For a generated type T and each conforming value v: if v is truthy it conforms to Truthy(T), and if falsy to Falsy(T). So no value is truthy when Truthy(T) is `never`, and none is falsy when Falsy(T) is `never`. This is the test that `always-truthy` and `always-falsy` never lie. |
| `as`, `is` and conformance | For a generated type T and generated values, some conforming and some broken in one place: the walk agrees with a second, plain reading of the table of 2.14. If disjoint(A, B), no generated value conforms to both. If S surely fits T, every value that conforms to S conforms to T. Where `x is T` passed, the value conforms to the narrowed type of `x`, and where it failed, to without(X, T). These are what make `never-conforms`, the two constant-condition codes on `is` and both sides of the narrowing sound. |
| Tagged unions | The generator makes tagged unions, and values of each variant, with a wrong tag, with no tag and with a field missing. The walk agrees with a plain reading of the row of 2.14. A value that conforms to U conforms to flat(U). Each variant fits U. Where a test of a field passed, the value conforms to the narrowed type of the path, and where it failed, to the type the false fact gives. Any two or more variants of U form a tagged union. Join(U, U) is U, and Join of two generated types is never a tagged union that neither was. |
| A fit through a form | For a generated type T and a generated literal, with conditionals and blocks inside it: if it fits T through its form and the checker reports nothing, every value a run gives conforms to T. If the type of the expression fits T, nothing is reported. Every report's span is the expression or lies inside it. |
| Projections, `filter` and `find` | The value of a projection conforms to its type. `xs[*].f` equals `xs.map(x => x.f)`, and `xs[*].f[*].g` equals `xs.flatMap(x => x.f).map(y => y.g)`, in value and in type. Every element that `filter` keeps, and the one `find` gives, conforms to the narrowed element type. |
| `try` and the error value | For each run code, a program that raises it inside a body: the handler runs exactly when the catalog says the code is caught, the value bound conforms to the type of 2.5, and its `code` is a member of C. So C and the evaluator's test by code are held together by the suite, not by hand. |
| Export | Every generated program, with its diagnostics, type and sites, is written out as a conformance case, and the Rust and TypeScript checkers must agree on all of them. This is the main test that the three checkers are the same. |
| Metamorphic | Replace a declared type with `any`: no new diagnostics under the default set of classes, and the same value. Add null to a type or make a field optional: no diagnostic disappears except `always-truthy`, and no `always-falsy` appears. Take `constant-condition` out of the rejected set: exactly the `always-truthy` and `always-falsy` diagnostics go, and the type and the sites are the same. Join is commutative, associative and idempotent. A fits Join(A, B). Join(Truthy(T), Falsy(T)) is T. Parsing the printed text of a type gives that type. disjoint is symmetric. If S surely fits T, then S and T are not disjoint unless S is `never`. A run of a program that was checked and a run of the same program compiled with no check give the same value and usage, or the same code and span: only the class may differ, and only as 2.14 says. |
| Native fuzzing | Over source bytes, in Go and in Rust: the checker ends and does not crash on anything the parser accepts, `Bad` nodes included. |
| Limits (2.17) | The deepest legal program, helper chain and type, and one past each, in every implementation. A program of 1,000 body checks and one of 1,001. An `else if` chain, a `? :` else chain and a sum of 100,000 links each, which must check without deep recursion. In Rust on a 2 MiB stack. A stack overflow in any of them is a failed test, not a crash to live with. |
| The engine | Its `soundness_test.go` (543 lines) and `soundness_fuzz_test.go` (1,363 lines) (R) test through `Load` and `Apply`. On a v3 port only the generator's expression strings change. Passing them is the acceptance test. `TestOutsideThePromise` (`workflow:soundness_test.go:505`) (R) should lose its numeric case. |

## 8. Alternatives rejected

| Alternative | Why not |
| --- | --- |
| Strict booleans: `!`, `&&`, `\|\|` and conditions require `bool` (the second revision of this document) | B1: the owner chose truthiness. What strict booleans bought was that `&&` and `\|\|` always give `bool`, with no rule. What they cost was `!= null` and `len(xs) > 0` at every test, and an error on the text most authors write first. The typing here keeps the part that mattered: a host that wants a `bool` still gets a check for one (2.14), and a test that cannot work is still an error (2.8). |
| JavaScript's falsy set, where an empty list and an empty map are truthy | B5. For the checker it is the worse set. A list or a record that is not null would then never be falsy, so `gate.items ? a : b` would be `always-truthy` and the author would have to learn that the test means "is null". With B5's set the falsy parts of a list and a record are types that already exist (`never[]`, `{}`), and the test is the same whether a Go host supplied a nil slice or an empty one. |
| `&&` and `\|\|` that test by truthiness and give `bool` (C, PHP) | B6. `x \|\| default` is then gone, and `??` does not replace an empty string or a zero. The price of B6 is the unions of 2.14. |
| A second kind of fact, "is falsy", for the false branch of a bare path | 2.8: the only thing a type could then say is "`x` is null", and nothing can be done with that. |
| Narrowing a true branch to the truthy part, not only to nonnull | It would remove `""` from a literal set and nothing else an author sees. Two kinds of fact for that. |
| The falsy part of `float` as `int`, since zero is whole | True, and `score && n` would be an `int`. One more case in the table, for no consumer. |
| A weaker severity for `always-truthy` and `always-falsy` | D32 settled this another way: the two codes are a check class, rejected by default, that a host may take out (2.13). A warning would be a third state between reported and not. A host that reads a field from YAML has nowhere to show a warning, so the test that never works would pass in silence for an author, which is the outcome B7 is there to prevent. It would also add a severity to every diagnostic in three implementations and in `05`. A class is in the set or out of it, by the one mechanism hosts already use for sites. |
| A predicate typed `=> any` | It accepts the same programs, but says "unchecked" where it means "tested", and the checker could not tell a predicate from a callback that returns untyped data. `unknown` costs one word in the grammar, and it is the word TypeScript's library uses there. |
| Typing `2.0` as `float` (my first draft) | 2.6. |
| Folding constants, or typing `int / int` as `int` when it divides | 2.6: arithmetic in every checker, for an expression nobody writes. |
| One function type per `const`-bound lambda | 2.11: unchecked helpers. |
| Function types as ordinary types | Nothing can hold one (E6, S3, D34). |
| A lambda passed to a `const`-bound lambda, and a parameter that can be called (earlier revisions) | D34. A parameter was a value in one check of a body and a lambda in the next, no check could be reused, and the check of some programs ended only at a limit (2.11). |
| Two mechanisms: rejected site classes, and a separate switch for constant conditions | 2.13: one set of rejected classes does both. The classes differ only in what is kept when a class is not rejected. |
| Renaming the site class `dynamic` to `any` with the type | 2.14: the class says when the check is made, and `any` reads badly as a class. |
| `unknown` as the word a host passes for a result it tests by truthiness | 2.2: `unknown` is type text for a callback's result. An expected result of "unknown" would read as "the host does not know". `condition` says what the host will do with the value. |
| Rules of its own for `if` | 2.5: it is checked as the `? :` chain it stands for, with a block in each branch. One set of rules for tests, facts and joins. |
| Literal sets of numbers and booleans | No consumer needs coverage over numbers, and without them no rule widens anything. |
| Records with methods (my first draft's nominal records) | A second kind of record, with identity rules, for a binding feature. 2.15 gives a host two clear choices. |
| Expected types flowing into list and map literals | `never[]` and exact records already fit where they are passed, and a flowing type changes inferred types. The rule of 2.5 is not this: it changes no type, and runs only where the fit of the inferred type fails. It decides where a diagnostic lands, and lets a literal meet a tagged union (`09:2.6`, `12:3.3`). |
| A fit through a form that also follows a `const` name, a call or a `try` | Each is one more rule that three checkers must share (`12`, section 9). A name would need the expression it was bound to, kept past its check. Open question 11. |
| Reporting a missing required field on the `{` that opens the map (`12:3.3`) | Ruling 1 of round 8: one rule, `09:2.6`'s, which reports on the whole map. The one-line rendering gives the start of the span in either case. |
| Untagged unions of records | D47, and 2.1. |
| A tag named in the type text | New grammar that TypeScript does not have, for something the variants already say (09-1). |
| Join makes a tagged union when two records form one | 2.3: a type that grows with the data. |
| Join keeps a union when one side surely fits the other | `[...shapes, a, b]` would be a `Shape[]` and `[a, b, ...shapes]` would not, because `a` and `b` join first. Join would stop being associative, and every rule that joins several types would need an order that three checkers share (`09`, section 7). It is the first thing to revisit (open question 9). |
| `no-field` for a field that only some variants have, as in TypeScript | A second rule for reads. The flat record gives `maybe-absent` for `.` already, and the answer for `?.` is sound (09-5). |
| A fact "this path equals this string" that also narrows a plain literal set | Wider than D47 asked, and `is` does it already. |
| `in` as a test that narrows a tagged union | A new kind of fact (`09`, section 3). |
| A `class` field on the error value | 2.5, 2.12: a value would then depend on what the checker inferred, and an evaluator with no checker could not fill it (10-2). |
| The class `environment` passes through `catch` | The same reason: whether a wrong kind is `dynamic` or `environment` is not known to an evaluator. `catch` decides by code (10-3). |
| `environment` as the class of a null, kind or field failure in a program that was not checked | Ruling 7 of round 8. It would blame the host for what may be the program's own mistake. |
| A star that covers the rest of the chain, method calls included | 13-2: `(orders[*].lines[*].qty).sum()` would need parentheses. |
| A star on a map, visiting its values | One rule for a star: a list. The help names `.values()[*]` (ruling 10). |
| Dropping the nulls of a projection, or unwrapping a list of one | The shape of a result would depend on the data (`13`, K4). |
| `filter` that narrows a field of the element | It needs a type for "this record, with this field narrowed", built per call. A fact narrows a path (2.11; open question 10). |
| A rule here for cutting a long literal set in a message | Ruling 9: the catalog's rule for a `type` parameter does it (`10:2.9`). |
| A text block that is always a `Template`, so that its type is `string` (08-8, the alternative) | A constant block would be charged as text created on each evaluation (`08`, section 8). The long literal set is cut in a message by the catalog. |
| Leniency inside `try` (expr, and my first draft) | `02:2.12`: `try` is for bad values. One fewer mode in the checker. |
| Narrowing on calls or on list indexes | Unsound for host functions; `const` covers the second. |
| Width subtyping for records | Unsound with spread and rest patterns. |
| A separate `invalid` type for error recovery | `never` already fits everything and reports nothing. |
| Overloads in signatures | `06` needs none (`06:2.8`). |
| Hindley-Milner inference | Nothing is polymorphic except library signatures, which are solved per call. |
| Plain Fits as the test for "this `as` cannot fail" (`07:2.6` as written) | 2.5: `any` fits every type, so the one case the operators exist for would be the case that is never tested. Surely fits is Fits that did not use any. |
| Narrowing `x is T` to the common part of T and the type before | 2.8: it needs an operation that intersects two types, which nothing else here needs. T as written is sound, and the author controls it. |
| No narrowing where `x is T` failed (`07:2.3`) | 2.8: an author expects the other branch to know what is left, as in TypeScript, and the rule is one line over alternatives. |
| A code of its own for an unknown type name | `undefined-name` says it, and its span says where. |
| Named types with an identity: `Order` distinct from a record of the same shape | 2.15, D44. Two kinds of record, and a rule for when one fits the other, for a feature whose point is to test shape. An opaque type is the nominal type. |
| Keeping expr's `Type` struct | Three encodings of "or" and a sentinel pointer for the empty list (`expr:types.go:49-56`, `:252-257`) (R). |
| One checker in Rust, shipped to all three as WebAssembly | 6.3: a fallback. A Go host would need a WebAssembly runtime, and a browser editor a larger download. |
| Leaving the checker's stack to each platform | R5: an error in Go, a `RangeError` at an unknown depth in JavaScript, the end of the process in Rust. 2.17 gives the numbers. |

## 9. Rulings applied, and what is left to others

| Ruling | Where it lands |
| --- | --- |
| P1-P6 | Section 2 is the specification; 2.2, 2.13, 2.14 and 2.15 are its notation, codes, classes and portable declarations; 2.16 is the data. |
| C1, R1-R3, R5, R9 | Three checkers: sections 5, 6.1 and 6.2. No rule needs reflection or a collector. 2.17 bounds every recursion. 6.3 is the shared-checker fallback. |
| N1-N6, S7 | 2.6. Overflow is `range`, never a site. |
| B1, B5-B8 | 2.3 (the two parts), 2.5 (`!`, `&&`, `\|\|`, `? :`), 2.8 (tested positions, constant conditions, narrowing), 2.11 (`unknown`), 2.13 (two codes and their class), 2.14 (what it costs, and what a host expects of the result). |
| B2, B3, B4 | 2.7. |
| E1-E4 | 2.9; 2.5 rows 2 and 3; 2.10; `**` in 2.5. |
| E5, E6, S3 | No assignment, `return`, `throw`, loop, pipe or slice rules. Loops, reassignment and early `return` are not in 3.0, and the words are reserved (X3 to X6): a later version that adds them adds rules here. Lambdas in 2.11. |
| E9, D34 | 2.4 (two kinds of binding, calls), 2.11 (where a lambda may go, what it simplified), 2.13 (`call-of-parameter`, `lambda-to-lambda`), 2.17 (the limits bound work and stack, not termination). |
| X1, D35, D37 | 2.5: an `if` is the second form of `Cond`, checked as a `? :` chain with block branches. 2.8: its tests are tested positions. 2.17: an `else if` chain is checked by a loop. No new node and no new code: a missing `else`, a branch written like a map and an empty branch are `02`'s parse errors. |
| X2, D36 | `const` in every rule and example. The node is `Const`. `let` and the other reserved words are `02`'s parse errors. |
| S1, S5, S6 | 2.4; section 5; 2.5 covers the nodes of `02:2.3`, `As` included, and nothing else. |
| S11 | 2.5: `==` never fails, so it is never a site and two opaque values may be compared. |
| S12, S13, S17 | Names are `06`'s. `round` has no `!`. A receiver is any list or map type (2.2). |
| T5 | Spans are in lines and code point columns, end exclusive (`05:A.6`). |
| C7, Y5 | No rule here is kept, or softened, for the sake of v2 or expr. |
| Y3, D28 | Null throughout: the kind, the type `null`, `maybe-null`, nonnull, the failure kind `null`, the option *absent-is-null*. "nil" is kept only for Go's nil and expr's. |
| Y4, D29 | 2.2 is the grammar, and every type in this document is written in it. 2.18 records the previous spelling. |
| D30 | The falsy set of 2.3 is the confirmed one. |
| D32 | 2.13: check classes, one set of rejected classes, `constant-condition` in it by default. 2.16: the `reject` field. |
| D33 | 2.5 row 1, 2.8 and 2.14: `condition` as an expected result. 2.2: how it relates to `unknown`. |
| V1 to V5, D39 to D45 | 2.1: named types, and what leaves the left-out table. 2.2: `Name`, and what may follow `as` and `is`. 2.5: the node `As`, with surely fits where `07` says fits. 2.8: `is` as a constant condition and as a test that narrows both ways. 2.12: which types never change results. 2.13: `never-conforms`, the 28th code. 2.14: conformance as the normative table, disjoint, `as` as a `value` site, the class of a failed `as`. 2.15: named types. 2.16: conformance cases. 2.17: the walk is as deep as the type. Sections 5 and 6: what moves from the checker to the core. |
| D46 (`08`) | 2.2: a one-line raw string may be a literal in type text, and a text block may not. 2.5 rows 6 and 7: a literal with no `${ }` is a `String` in every form. No rule changed. |
| D47 (`09`) | 2.1: the type. 2.2: two alternatives of one kind, records with no tag, printing. 2.3: the two map rows of Join and Fits, flat, the parts, the helpers. 2.5: the fit through a form. 2.7: a read sees the flat record. 2.8 and 2.9: the tests that narrow. 2.13: `untagged-union`, 29 codes. 2.14: the conformance row and the map rows of disjoint. 2.15: `anyOf` and `oneOf`. 2.16 and 2.17: cases and depth. |
| D48 (`10`) | 2.4 and 2.5: the scopes and the type of `try`, the error value and its literal set C. 2.12: why it has no class. 2.13: messages are the catalog's. 2.14: sites in the body, the class of a run error, a crash. Section 3: every `try(e, d)` rewritten, and the examples shown in the detail form. |
| D49 (`11`) | 2.16: `returns`, the header, `precondition`, the `runner` cases. |
| D50 (`12`) | 2.5: its span rule is the fit through a form. 2.15: a loader sets *closed*. Sections 5 and 6.2: an optional field, and a text type, from a host type. Imports are not in 3.0, so no code, node or limit for them is here (`12:3.9`). |
| D51 (`13`) | 2.7: a projection. 2.11: `filter` and `find` narrow. 2.5: the node `Each`. No code. |
| Rulings 1, 5, 6, 7, 9, 10 and 11 of round 8 | 1: 2.5, a missing field is reported on the whole map. 5 and 6: the class table of 2.14. 7: 2.14, a program that was not checked. 9: 2.13, no rule here for a long type in a message. 10: 2.7, a star on a map and a kept null. 11: 2.2, alternatives of one kind. Ruling 13: D37 and D38 are still open, and nothing here settles them. |

| To | Request |
| --- | --- |
| `02` | 2.7 is the run-time rule the parts of 2.3 rest on: the six falsy values, and a test that cannot fail. 2.15 should list what it expects of this document: the types of `&&`, `\|\|` and `!`, a bare path as a null test, the two constant-condition codes and their class, and `if` checked as a `? :` chain. The maximum source nesting (D31) is the number 2.17 uses for expanded nesting; say how a node's nesting is counted. Literals are typed by value, as it says. This document names the nodes `Const` and `Null` as `02:2.3` does, and treats an `if` as the second form of `Cond`. `call-of-parameter` and `lambda-to-lambda` are defined here as `check` codes (2.13), because the checker is the pass that resolves names. `call-of-parameter` covers every name that holds a value, a variable and a non-lambda `const` name included, where earlier revisions said `kind`. For D39: give `As` its number; make `Var`, `Sig`, `Fn`, `unknown` and `condition` after `as` or `is` parse errors (2.2); say whether the parentheses around a subject are inside the span of an `As` node, which three codes are reported on. For D46 to D51: `Each` is node 32 here; give it its number, its fields and its span, and say that the receiver of a step, a star included, spans the chain from its start up to that step (2.7). The span of a union after `as` or `is` runs from the start of its first alternative to the end of its last (2.2). The name of `catch (err)` is a binding like a `const` name, and may not be named like a namespace (2.4). Name the `parse` codes of the targeted errors that 2.13 leaves to the parser: the call form `try(e, d)`, a `try` with no `catch`, `throw`, `finally`, `import`. |
| `04` | Report each failure with its kind (2.14) and node. Resolve calls and lambdas from the checker's tables. A test is not a failure point, so a tested position needs no kind check. For D39: the walk follows the table of 2.14 and is at most as deep as the type (2.17). Names are replaced and the depth of a written type is tested when the type is compiled, in an evaluator with no checker too. A failed `as` is class `value` at a site and `environment` elsewhere (2.14). Do not skip the walk where the checker says the type fits: `04:A.8` allows a skip only on a proof from the value itself (open question 7). For D47 to D51: the class of a run error follows 2.14, also for a program that was not checked. `number-range` from a host number is class `environment`, and `function-panic` is class `host` and is not caught. The test that the records of a written type have a tag is made where the type is compiled (2.17), and the union node reads the tag first (`09:2.7`). |
| `05` | Add opaque type declarations, and method declarations in the `Method` text, to the environment and its description. Decide the nil slice question of section 5. The codes of 2.13 are the check catalog: 29 since D47, all errors, so no severity field is needed. The environment and its description carry the set of rejected check classes (2.13): six class names, default `constant-condition` alone. `condition` as an expected result is in 2.5 and 2.14, no longer an open question here; the binding supplies the value and the run-time test (A.3). The site class and the run error class stay `dynamic`. Name the `declare` code for a type that is malformed or passes a limit of 2.17. A check failure in a conformance case is a list of diagnostics. For D39: named types as declarations, in the order declared, sharing names with opaque types; `bad-type` for a cycle, an unknown name or a depth past 64; the three JSON Schema options given per schema (2.15); the form of the path of `not-conforming`, which the conformance cases of 2.16 quote; `Conform` as the walk of 2.14. For D46 to D51: the check catalog is 29 codes, with `untagged-union`, and `bad-type` covers a declared type with two records and no tag. A.6 follows 2.14 for the class of a run error, also in a program that was not checked, and lists the caught run codes, from which the type of `err.code` is generated (2.5). `Type` gains accessors for the tag and the variants, and its record accessors answer for the flat record (section 5). B.7: a field with `omitempty` or `omitzero` is optional and a `TextUnmarshaler` with no registered method is `string` (section 5); say how such a field is presented going in. Give the three schema options a place in the TypeScript and Rust sketches (2.15). The case field is `returns`, and a case that compares a class checks first (2.16). |
| `06` | Write every signature in the text of 2.2, with `!` by the rule of `06:2.4`. A predicate is `(item: T, index: int) => unknown`: `filter`, `find`, `some`, `every` and `count`. `bool(x)` is in the core, and its argument is a tested position (2.14). Give each `Fn` its full parameter list, with names, so step 3 of 2.11 knows n. A rest parameter is `...rest: T[]`. No entry's `Fn` takes an `Fn`: a callback is never handed a function (D34). For D48 and D51: `fail` is `(message: string, data?: any) => never !`, so a call of it is a `value` site with type `never`. `filter` and `find` are the two entries that narrow, by name (2.11); no other entry of 5.1 of `13` needs a rule here, and `order?: "asc" \| "desc"` is an ordinary literal set. The new map method `mapValues` applies to a tagged union through its flat record (2.3). |
| `07` | Five places where its rules need the text given here. (1) 2.6, "the type of `x` fits `T`": with Fits, `any` fits everything; it must be surely fits (2.5). (2) 2.6, "no value conforms to both" is named and not defined: 2.14 defines disjoint, under which two list types are never disjoint. (3) 2.2, `int`: "a whole number" must be the whole number of 2.6, so `1e20` does not conform. (4) 2.3, narrowing to T: it must not widen a type that already fits, and the failing side is decided (2.8). (5) An unknown type name and a written type deeper than 64 have no code and no stage: here they are `undefined-name` and `check-limit`, found when the type is read (2.5, 2.17). Also: a named type is shared, not copied (2.15), and its 2.7 lets an implementation skip a walk (open question 7). For D47: its 2.2 says there is never a choice among alternatives, which a tagged union keeps by its tag; its 2.9 and section 3 say two object branches join into one record, which now holds only when they form no tagged union (2.2, 2.15). |
| `09` | Four places where this document says more than the plan, or differs. (1) Its 2.5 lists a predicate among the things that do not narrow. With D51 `filter` and `find` narrow, and a tag test in one keeps one variant (2.11). (2) Its 2.6 says an argument keeps "the same code" at each place of failure. An argument has two codes: here it is `maybe-null` where the type at that place fits once null is removed, else `arg-mismatch` (2.5). (3) Its 2.6 was written against the call form of `try`. The fit does not go through a `Try` here (open question 11). (4) Its 2.3 does not say that a subset of the variants is again a tagged union, with a tag that may be a lesser key. Narrowing relies on it (2.1). |
| `10` | (1) Its 2.3 writes the type of `path` as `(string \| int)[] \| null`. Printed by 2.2 it is `(int \| string)[] \| null`. (2) Its 2.6 says there are still 28 check codes. With `09` there are 29. (3) A crash has class `host` by ruling 6 and can occur at a call that is not a site, so promise 3 of 2.14 now names it. (4) The class of `cancelled` is `05`'s to give. This document does not name one. |
| `12` | Its 3.3 reports a missing field on the `{` that opens the map, and a spread on the `Spread` node. Ruling 1 and `09:2.6` put them on the whole map and on the spread's operand (2.5). Its 3.5 says a `TextUnmarshaler` is `string`: section 5 has the row, and `05:B.7` owes how such a value is read going in. |
| `13` | Its 5.2 gives the type of a projection in one row. Three things it leaves unsaid are decided in 2.7: the span of a receiver inside a projection is the chain up to that step; one flag for the whole projection makes the element type nullable, at any star; and a `never` receiver gives `never`. Its 5.1 narrows only when the first parameter is a name; a `const`-bound lambda passed by name is narrowed as if written there (2.11). |

## 10. Open questions

| # | Question | Recommendation |
| --- | --- | --- |
| 1 | Should the workflow engine reject `index` sites? | Yes for the engine, with `?.[`, `first` or a pattern default as the fix. No as the default. An out-of-range index is the first case the engine pins outside its promise (`workflow:soundness_test.go:517-519`) (R). |
| 2 | Literal sets are strings only, so a numeric `enum` is `int` and a `match` on it needs `_`. | Accept. The engine's `switch` on a numeric enum (`workflow:typecheck.go:859`) (R) needs a default. |
| 3 | A `const`-bound lambda is checked at each use, with a limit of 1,000 body checks per program, counted by use. An error caused by one use shows inside the helper, a constant condition included. Since D34 a check can be reused for the same lambda and parameter types (2.11), so the limit could count distinct checks instead. The doubling chain of 2.11 would then cost one check a helper. | Accept counting by use for 3.0. It needs no table in any implementation, and three checkers agree on it with no further rule. Counting distinct checks would make the reuse table part of the specification. Ask again if real programs meet the limit. The message should name the use. |
| 4 | A host value is a record or opaque, never both. A Go struct with fields and methods must pick, or expose its fields as methods. | Accept for 3.0. A named record with methods can be added later without changing a rule here. |
| 5 | A JSON Schema property outside `required` is `f?: T` unless the host says it fills absent properties. | Keep. It is what is true of raw JSON. |
| 6 | Type depth is bounded (2.17). Type size is not: `const b = {l: a, r: a}; const c = {l: b, r: b}; ...` doubles the printed size of a type on each line. | An implementation must intern or memoize so that Join and Fits do not walk the tree form. A message cuts a long type by the catalog's rule for a `type` parameter (`10:2.9`), so that part is settled. A tagged union adds a second way to grow: n variants hold n records and one flat record, and Fits of two unions compares each variant with each (2.17). If that proves fragile, add a limit on the number of distinct types a check may build, reported as `check-limit`. Named types make the same doubling available to a host, one declaration a level (2.15), and an evaluator now holds types too, so the rule to share and not copy applies to it as well. |
| 7 | `07:2.7` lets an implementation skip the walk of an `as` or `is` whose subject already fits, and charge for it all the same. When a host value breaks its declared type, the skipped test passes and the walked one fails, so two implementations give two results, and `05:A.3` says a declared type is a promise the evaluator does not trust. | No implementation skips the walk on the checker's word in 3.0. Then "a run without the checker behaves the same" (`07:2.6`) is true without a condition, and a failed `as` at a place the checker thought safe is reported as `environment` (2.14). The price is a walk the checker could have removed. A hint may bring it back later for values the evaluator itself built. `04:A.8` and the revised `07:2.7` now say this: a skip needs a proof from the value's own representation, such as a Go `[]string` against `string[]`, and an inferred type is not one. |
| 8 | `is` narrows both ways, and the passing side takes T as written (2.8). `07` asked for the first and left the second unsaid. Both are decided here. | Accept. The failing side removes only what surely fits T, plus the strings of a literal set, so it is sound and never surprising. Taking T as written can lose what was known before, as in `x: {a: int, ...}` tested with `{b: string, ...}`. The fix for that would be an intersection of types, and I would not add one for it. With D47 the loss has a common case: `s is {kind: "circle", ...}` forgets `r`, where `s.kind == "circle"` keeps the variant. The help for the first form should name the second (`09`, section 9). |
| 9 | **The cost of 09-3.** Join never makes a tagged union, so a value built from different variants in different branches has the joined record: `shapes.map(x => x.kind == "circle" ? {...x, r: 2} : x)` is a list of one joined record and does not fit `Shape[]` (example 78). The author writes `as Shape[]`, a test with a cost and a `value` site. The fit through a form covers literals only. | Accept for 3.0. It is the largest hazard of D47. Two repairs exist, and neither is for 3.0: annotations (D40), which reuse 2.5 with no run-time test, and a Join that keeps a union when one side surely fits the other, which needs an order rule that three checkers share. Watch for `as` on a tagged union in ported programs. |
| 10 | **`filter` narrows the element, not a field of it.** `orders.filter(o => o.status == "open")` leaves `status` the whole literal set, and `xs.filter(x => x.note != null)` leaves `note` nullable (`13`, section 7). | Accept. A type for "this record with one field narrowed" would be built per call, and facts narrow paths. An author who needs it maps to the field first: `xs.map(x => x.note).filter(n => n != null)`. |
| 11 | **How far the fit through a form follows.** It goes through `List`, `Map`, `Cond`, `Match` and `Block`. It does not follow a `const` name, a call or a `try`. `09:2.6` was written when `try` was a call, and `10` does not say. The two branches of a `try` are branches as an `if`'s are. | Leave `try` out for 3.0, as the plan is written, and add it with the `const` name if config files show the need. Each is an addition: it only moves a report inward or accepts more. |
| 12 | **Two keys that both qualify as the tag.** The tag is the lesser, and an author cannot choose. After narrowing, the tag of what is left can be a lesser key still (2.1). | Accept. It shows only in the path of a failed conformance test and in the order variants print. No rule of the checker asks which key is the tag. |
| 13 | **A `match` on `err.code` with no `_`** must list every caught code, and the set grows when a version adds a run code. A program is compiled against one version (`05:A.10`), so a stored program does not break. An author who moves to a newer version may have to add an arm. | Accept. It is the price of a checked `code`, and the same holds for a `match` on any literal set a host later widens. The help for `non-exhaustive` on `err.code` should suggest `_`. |

Two questions of the third revision are settled and are now text. Question 6,
whether `always-truthy` and `always-falsy` are plain errors, is D32: a check
class, rejected by default (2.13). Question 7, a host that tests its result by
truthiness, is D33: `condition` as the expected result (2.5, 2.14). Question 8
of that revision is question 6 here. Questions 1 to 5 keep their numbers.
Questions 7 and 8 are new with `as` and `is`. Questions 9 to 13 are new with the
plans of round 8, and question 6 lost its part about messages to the catalog.

Three things the plans left open were ruled in round 8 and are now text, not
questions: what two alternatives of one kind mean in type text (2.2), the class
of a run error with no checker (2.14), and whether this document needs a rule
for a long literal type in a message (it does not; 2.13). D37 and D38 are still
open and are not this document's to settle.

**The cost of B6, stated plainly.** I do not argue against the ruling. Its cost
is example 42: the guard most authors will write in a `when:` field, `report &&
report.verdict == "APPROVE"`, has type `bool | null` and is rejected by a host
that asks for a `bool`. D33 removes the problem for a host that tests by
truthiness: it passes `condition`, the guard is accepted, and the workflow
engine does this for `when:`. For a host that still asks for a `bool`, the
messages can teach `report?.verdict == "APPROVE"`.

No other ruling in the overview seems wrong to me. N5 with value-typed literals
has the consequence in 2.6, which I think is right.

## 11. Size and order of work

Lines without tests, (I). The first column is what each of the three
implementations writes.

| Part | Per implementation | Go only | Rust only | expr today (R) |
| --- | ---: | ---: | ---: | ---: |
| Types: sets, Join, Fits, Merge, print | 400 | | | 353 (`types.go`) |
| Type text and signature parser, with named types. In the core since D40 | 270 | | | none |
| Checker: nodes, operators, chains, narrowing | 750 | | | 1,035 (`check.go`, all of it) |
| Truthiness: the two parts, `&&` and `\|\|`, tested positions, constant conditions, `unknown` and `condition` | 125 | | | in `check.go` |
| `match`, `if`, patterns, blocks | 265 | | | none |
| Calls, solving, `const`-bound lambdas | 270 | | | in `check.go` |
| Diagnostics, sites, check classes | 160 | | | 249 (`suggest.go`) |
| JSON Schema mapping | 110 | | | none |
| The limits of 2.17 | 30 | | | none |
| `as` and `is`: the three cases, disjoint, two kinds of fact, without | 150 | | | none |
| Tagged unions: variants in the map slot, flat, the rows of 2.3, printing (110); the test for a tag, in the core with the parser (40); hit, all, and the rows for `==`, `!=` and `match` (60); `anyOf` and `oneOf` (15) | 225 | | | none |
| A fit through a form (2.5) | 60 | | | none |
| `try` and `catch`: the scopes, the Join, the type of the error value from the catalog | 40 | | | in `check.go`, as a call |
| A projection (2.7) | 50 | | | none |
| `filter` and `find` narrow (2.11) | 30 | | | none |
| `TypeOf`, hints, accessors, with the two rows of D50 | | 440 | | none |
| The type half of the derive macro, with `#[serde(default)]` | | | 180 | none |
| **Total** | **2,935** | **440** | **180** | **1,388**, plus `suggest.go` |

Three implementations are about 8,800 lines of checker, plus the two columns
that are not shared.

**What truthiness adds: about 125 lines in each implementation.** The two parts
are a table of eight rows (40). `&&` and `||` become two Joins where they were
two fits (20). A bare path joins the fact table as one row, and the rule that
recovered from a non-`bool` operand goes (net 0). The constant-condition rule is
one function called from each tested position (30). `unknown` in the signature
parser and step 3a of 2.11 (20). `condition` as an expected result (5). Two
messages with their help (10). That is 5% of the checker. It is small because
the first draft's truthiness came with number and bool literal sets and their
widening, and this one adds no form of type. The cost that does not show in
lines is in the cases: about 50 more, and unions reach more of the rest of them.

The first draft estimated 3,250 for Go. Removed since: the statement tier,
function types, number and bool literal sets with their widening, nominal
records, expected types in literals, and leniency in `try`. Added: `match`,
patterns, chains, Merge, opaque types, per-use lambdas, the limits of 2.17, and
an exact span and recovery type for every code. Tests are mostly data shared by
all implementations: about 650 conformance cases, and 800 lines of fuzzing in
Go. Three to four weeks for Go, more than half of it the cases (I). The Rust and
TypeScript checkers are written against finished cases, so I expect about two
weeks each (I). Round 8 adds about 405 lines and about 150 cases: 80 for tagged
unions (`09`, section 5), and about 70 more for the fit through a form, `try`,
projections and the two entries that narrow (I). I would add a week for Go and
two or three days for each of the others (I).

**What the fourth and fifth revisions change: nothing in the total, which they
leave at 2,400. `as` and `is` then take it to 2,530.**

| Decision | Lines | Why |
| --- | ---: | --- |
| D34: a lambda only to a library entry, no call of a parameter | −30 | Gone: a parameter bound to a lambda, the table of those bindings that each body check carried, and a call resolved through it. Added: two codes, each a test on a resolved name. Checking per use, the count of body checks and the count of expanded nesting all stay (2.11, 2.17). |
| D32: check classes | +10 | One set of class names on the environment, and a test of it where a constant condition or a site is reported. |
| D33: `condition` as an expected result | +5 | One more call of the constant-condition function. |
| D35: `if` | +15 | A loop over the branches that carries facts and a Join. Every rule it uses is the `? :` rule or the block rule. |
| D29: type text | 0 | About 60 of the 250 lines of the type text parser and printer are different lines: `[]` after an element, parentheses around a union element, `=>`, named parameters. No rule of the checker reads type text. |
| D28, D36: `null`, `const` | 0 | Names. |
| D39, D40: `as`, `is`, named types | +130 | +150 for the rules of 2.5 and 2.8 and for disjoint, which is `07`'s figure. +20 for names in type text. −40 for the conformance of values, which was a helper for tests here and is now the walk in the core: about 250 lines with paths and step charges, counted by `07` and `04`. The type text parser also moves to the core, and is counted once, here. |

**What round 8 changes: 2,530 becomes about 2,935.** The figures are the plans'
own (I). None of this was built.

| Decision | Lines | Why |
| --- | ---: | --- |
| D46: string forms | 0 | The checker types a node, not a spelling (`08`, section 5). |
| D47: tagged unions | +225 | `09`, section 5, less its 60 for the fit through a form, counted below, and less its 50 for the union node of the walk, which is the evaluator's (`04`). `09` gives "about 2,800" for the checker after it, which is this row and the next. |
| D47, D50: a fit through a form | +60 | `09`'s figure. `12`, section 6, counts 80 for its span rule; it is the same rule, counted once. |
| D48: `try`, `catch`, the error value | +40 | `10`, section 3. The call form's row in the checker goes. The catalog loader and the renderer are `05`'s. |
| D49: tests as data | 0 | Nothing in the checker. The runner is `05`'s and `11`'s. |
| D51: a projection | +50 | `13`, 5.6. |
| D51: `filter` and `find` narrow | +30 | `13`, 5.6. |
| D50: host types | Go +40, Rust +30 | `12`, section 6: optional fields and text types. Not in the shared column. |

| Order | Work |
| --- | --- |
| 1 | Types, type text, named types, tagged unions with the test for a tag, the conformance walk with its cases (`07`, step 2: the evaluator needs these first), Join, Fits, Merge, flat, the two parts, disjoint, printing, with the algebra cases of 2.16. |
| 2 | Names, literals, operators, reads and chains, projections, tested positions, `as` and `is`, narrowing, the tag tests, with their codes. Port expr's accept and reject tables as cases. |
| 3 | Blocks, patterns, `match`, `if`, `try` with the error value's type from the catalog. |
| 4 | Signatures, solving, lambdas, the fit through a form at an argument and at the result, `filter` and `find`, with `06`'s table. |
| 5 | Sites, check classes, the promise harness against the evaluator. |
| 6 | JSON Schema, value conformance, `TypeOf`, hints. |
| 7 | The fuzzer and its export. Then a trial port of the engine's `typecheck.go`. |
| 8 | The Rust and TypeScript checkers, against the exported cases and the limit cases (overview 5.2, step 9). |

**Reused from expr:** the Join of records with optional fields from one side
(`expr:types.go:316-353`); the shape of narrowing (`expr:check.go:109-137`), now
on resolved paths; the typing of `&&` and `||` (`expr:check.go:316-325`),
sharpened from "without nil" to the two parts; the enum comparison check
(`expr:check.go:401-407`); the test tables. **Not reused:** the per-builtin Go
tables (`expr:check.go:857-916`), the `it` forms (`:683-744`), leniency in `try`
(`:621-629`), and narrowing of calls.
