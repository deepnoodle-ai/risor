# 5. Core check: types and the checker

This chapter specifies the types of Risor and the static checker for the forms
of Core run. Every implementation that has a checker accepts and rejects the
same programs, infers the same types, records the same sites, and reports the
same diagnostics with the same codes and spans.

The language, its node kinds and its nesting count are in `01-language.md`.
Evaluation and the run codes are in `02-evaluation.md`. The environment, the
expected result, the shape of a diagnostic and of a case are in `03-host.md`.
The library and `library.json` are in `04-library.md`.

## 1. Conventions

### 1.1 Terms

| Term | Meaning |
| --- | --- |
| **Core** | The forms and rules of this chapter. A rule marked **follow-on** is not part of Core. Its package adds it later, and adds nothing that changes a result of this chapter. |
| **synth(e)** | Checking node e: it gives e a type, may report diagnostics, may record sites and may produce facts. |
| **Diagnostic** | A check code and a span (16). Every diagnostic is an error. A program with a diagnostic does not check. |
| **Site** | A class and a span: a place where a checked program can still fail on a value (18.3). |
| **Fact** | A statement about a path that holds where a test went one way (10.4). |
| **Tested position** | An expression whose value is tested by truthiness (10.1). |
| **Environment** | The declarations a program is checked against (19; `03-host.md`). |

### 1.2 Spans

A span is a start position and an end position in the source text. The end is
exclusive. A position is a line and a column, both from 1, as `03-host.md`
defines them. This chapter writes a span as `L:C-L:C`, so `1:1-1:7` is the first
six columns of line 1. A case file writes the same span as `[1, 1, 1, 7]`.

The span of a node is from its first character to the end of its last. The
parentheses around an expression are not part of its node. The span of a
`MethodCall` and of a `Call` runs from the start of the chain or the name to the
closing `)`.

### 1.3 "Must fit"

| Phrase | Meaning |
| --- | --- |
| e **must fit** T (code c) | If the type of e does not fit T (4.2), report c. If the fit used any (4.2), record a `dynamic` site on e. |
| e must fit T **through its form** | The same, with the place of each failure given by 15.1. |
| An operand **may be null** where null is not accepted | Report `maybe-null` on the operand, and continue with nonnull of its type (4.5). |

### 1.4 What Core does not have

These forms are parse errors in Core (`01-language.md`), so the checker never
meets them: `x as T` and `x is T` (node 31, `As`, follow-on F4), the projection
step `[*]` and `?.[*]` (node 32, `Each`, follow-on F7), and `match` with no
subject. Tagged unions are follow-on F5. Named types are follow-on F4. Types
from JSON Schema and from host types are follow-on F1.

## 2. Types

### 2.1 A type

A type is `any`, or a set of alternatives with at most one alternative of each
kind.

| Kind | Alternative | Values |
| --- | --- | --- |
| null | `null` | The one absent value. |
| bool | `bool` | `true` and `false`. |
| number | `int` or `float` | `int`: the whole numbers, with no fractional part and a magnitude of at most 2^53 − 1. `float`: every number. |
| string | `string`, or a literal set | A literal set is a finite, non-empty set of strings, such as `"fail" \| "pass"`. |
| list | `T[]` | Lists whose elements are of type T. |
| map | a record (2.2) | Maps. |
| opaque *N* | `N` | The host values that carry the name N (19.3). Each opaque type is a kind of its own. |

| Point | Rule |
| --- | --- |
| `never` | The empty set: the type of no value. It is the type of an expression that does not give a value. |
| `T \| null` | T with the null alternative added. A type that may be null has no other spelling. |
| `any` | The unchecked type. It is not an alternative and has no kind. It may be a type, an element type, a field type or a rest type. |
| Literal sets | Only strings have literal sets. No type holds a particular number or a particular boolean. |

### 2.2 Records

A record has **fields** and a **rest**.

| Part | Rule |
| --- | --- |
| A field | A key (any string), a type, and a mark: **required** or **optional**. |
| The rest | The type of the value under every key the record does not list. It is `never`, a type, or `any`. |
| Exact record | A record whose rest is `never`. It has no keys but its fields. |
| Open record | A record whose rest is `any`. |
| `map<T>` | The record with no fields and rest T. |
| `{}` | The record with no fields and rest `never`: the type of the empty map only. |

A record has no width subtyping: `{a: int, b: int}` does not fit `{a: int}`
(4.2).

### 2.3 How a type is held

A type is held as one optional slot for each kind, plus the marker `any`.

| Slot | Holds |
| --- | --- |
| null, bool | Present or absent. |
| number | Absent, `int` or `float`. |
| string | Absent, `string`, or a literal set. |
| list | Absent, or the element type. |
| map | Absent, a record, or a list of two or more records (the variants of a tagged union, follow-on F5). |
| opaque | A set of names. |

**In Core the map slot holds at most one record.** No type text, no Join (4.1)
and no other rule of this chapter puts two records in one type. A type text that
would need two is an error (3.4). The rows that F5 adds for a map slot with two
or more variants extend Join, Fits, Merge, Truthy, Falsy and the reads, and
change no result for a slot that holds one record.

### 2.4 What is not a type

| Not a type | Rule |
| --- | --- |
| A function | A lambda and a library or host function have no type. Function types exist only in signatures (3.6). |
| A namespace | It is a prefix of a function's name, not a value (5.4). |
| `unknown` | A word of signatures only (3.7). |
| `condition` | A word of the expected result only (3.7). |

### 2.5 Depth of a type

| Type | Depth |
| --- | --- |
| A type with no list and no record alternative, and `any` | 0 |
| A type with a list or a record alternative | 1 more than the deepest of its element, field and rest types, taken over all its alternatives. |

The limit is 64 (20).

## 3. Type text

Type text is how a type is written in an environment and in `library.json`. In
Core a program contains no type text.

### 3.1 Grammar

```ebnf
Type   = Alt { "|" Alt } .
Alt    = Prim { "[" "]" } .
Prim   = "null" | "bool" | "int" | "float" | "string" | "any" | "never"
       | String | "map" "<" Type ">" | Record
       | Name | Var | "(" Type ")" .
Record = "{" [ ( Field { "," Field } [ "," Rest ] | Rest ) [ "," ] ] "}" .
Field  = ( Word | String ) [ "?" ] ":" Type .
Rest   = "..." [ Type ] .
Name   = an upper-case letter, then one or more of [A-Za-z0-9_] .
Var    = one upper-case letter .

Sig    = "(" [ Param { "," Param } ] ")" "=>" Type [ "!" ]
         [ "where" Var ":" Bound { "," Var ":" Bound } ] .
Param  = [ "..." ] Word [ "?" ] ":" ( Fn | Type ) .
Fn     = "(" [ Word ":" Type { "," Word ":" Type } ] ")" "=>"
         ( Type | "unknown" ) .
Bound  = "ordered" | "number" .
Method = Recv "." Word ":" Sig .
Recv   = Alt .
```

### 3.2 Tokens

| Matter | Rule |
| --- | --- |
| Tokens | Type text is read with the lexer of `01-language.md`. White space and comments may stand between tokens. |
| `Word`, `String` | The tokens of `01-language.md`. In Core a `String` is `"..."` or `'...'`, with escapes. A backtick literal is not a `String` and may not stand in type text. |
| `>` | There is no `>>` token, so `map<map<int>>` ends with two `>` tokens. |
| Type words | `null`, `bool`, `int`, `float`, `string`, `any`, `never` and `map` are words of type text. Any other lower-case word where a `Prim` is expected is an error. |
| `Name` and `Var` | A `Var` is one upper-case letter. A `Name` has two characters or more. |
| `Var` | Only inside a `Sig` or a `Method`. Elsewhere it is an error. |
| `(` in a `Param` | `(` then `)`, or `(` then a `Word` then `:`, starts an `Fn`. Anything else is a `Type` in parentheses. |

### 3.3 Meaning

A `Type` is the set of its alternatives, with parentheses removed, joined by
3.4.

| Text | Type |
| --- | --- |
| `null`, `bool`, `int`, `float`, `string` | That alternative. |
| `any`, `never` | `any`; the empty set. |
| A `String` | The literal set of that one string, after escapes. |
| `T[]` | The list of T. `[]` binds tighter than `\|`: `int \| string[]` is an `int` or a list of strings. |
| `map<T>` | The record with no fields and rest T. |
| `{f: T}` | A record with a required field `f` of type T. |
| `{f?: T}` | A record with an optional field `f` of type T. |
| `{..., ...T}` | The rest is T. |
| `{..., ...}` | The rest is `any`. |
| A record with no `Rest` | The rest is `never`: the record is exact. |
| A key written twice in one `Record` | An error. |
| `T \| null` | T with the null alternative. |
| A `Name` | The opaque type of that name (3.5). |
| A `Var` | A type variable of the signature (13.4). |

`f?: T` and `f: T | null` are different types. The first is a field that may be
absent and is never null when present. The second is a field that is always
present and may be null.

### 3.4 Alternatives of one kind

When two or more alternatives of a `Type` have one kind, they are joined.

| Written | Type |
| --- | --- |
| `int \| float` | `float` |
| `"a" \| "b"` | The literal set of both strings. |
| `"a" \| string` | `string` |
| `int[] \| string[]` | `(int \| string)[]`: the elements are joined by Join (4.1). |
| `Time \| Time`, `null \| null` | `Time`, `null` |
| `any` among the alternatives | `any` |
| `never` among the alternatives | The other alternatives. `never` alone is `never`. |
| **Two or more records**, `map<T>` included | **An error in Core.** In a declaration it is `bad-type`, or `bad-signature` in a signature or a method's text (19.5). Follow-on F5 accepts two or more records that form a tagged union, and keeps the error for records that do not. |

### 3.5 Names

| Matter | Rule |
| --- | --- |
| A `Name` | In Core it must be an opaque type that the environment declares (19.3). Any other `Name` is an error: `bad-type`, or `bad-signature` in a signature or a method's text (19.5). Follow-on F4 adds named types, which share the set of names. |
| In a `Recv` | A `Name` must be an opaque type. |
| Type names and value names | Two separate sets. A type name is read only in type text. |

### 3.6 Signatures

| Part | Rule |
| --- | --- |
| `Sig` | The parameters, the result type, an optional `!`, and optional bounds. |
| `Param` | Every parameter has a name. The name is documentation only. |
| `?` after a parameter's name | The parameter may be omitted. Every optional parameter follows every required one. |
| `...` before a parameter's name | A rest parameter. It is the last, takes zero or more arguments, and its type must be a list: `...rest: T[]` takes arguments of type T. |
| `Fn` | A parameter that takes a lambda (13.2). Only a library signature may have one. Its parameter names are documentation only. |
| `!` | The entry can fail on a value. A call of it records a site (13.5, step 6). |
| `where V: B` | A bound on the type variable V (13.6). |
| `=>` | Binds loosest: `(x: T) => U \| null` has the result `U \| null`. |
| `Method` | A method of the receiver type `Recv`. The receiver is argument zero of every call (13.3). |
| `Recv` | `string`, a list type, `map<T>`, a record type, or an opaque type name. A receiver narrower than its kind, such as `string[]` or `T[][]`, makes the method apply only to receivers that fit it (13.3). |
| `Var` | The type variables of a signature are the `Var`s in its `Recv` and its `Sig`. They are local to the signature. |

### 3.7 `unknown` and `condition`

| Word | Where it may be written | Meaning |
| --- | --- | --- |
| `unknown` | Only as the result of an `Fn` | The lambda's body is a tested position: it may have any type, and its value is tested by truthiness (13.7). |
| `condition` | Only as the expected result of a program, in place of a type (`03-host.md`) | The program's result is a tested position (15.3). |

Neither is a type. Neither may stand as the type of a variable, a parameter or a
field, or as an operand of Join or Fits. `condition` is not type text.

### 3.8 Words that are errors

Each of these in type text is an error (19.5). The error names the form to
write.

| Written | Write |
| --- | --- |
| `number` (outside a `Bound`) | `int` or `float` |
| `boolean` | `bool` |
| `undefined`, `void`, `nil` | `null` |
| `object` | a record or `map<T>` |
| `Array<T>` | `T[]` |
| `T?` | `T \| null`, or `f?: T` for a field |
| `condition`, `unknown` outside an `Fn` result | 3.7 |

### 3.9 Printing

Every implementation prints a type the same way. A diagnostic's message and a
case's `type` use this text. The printed text reads back as the same type.

| Part | Printed as |
| --- | --- |
| `any`, `never` | `any`, `never` |
| Alternatives | In the order bool, number, string, list, map, opaque names in code point order, then `null`, joined with ` \| `. |
| Null | `null`, last: `int \| null`. |
| A literal set | Each string in its JSON form (`01-language.md` 12), in code point order, joined with ` \| `. |
| A list | The element, then `[]`. The element is in parentheses when its text has a ` \| ` at the top level: `(int \| string)[]`, `("a" \| "b")[]`, `(int \| null)[]`. |
| A record | `{`, the fields in code point order of their keys, each as key, `?` if optional, `: ` and its type, separated by `, `; then `, ...` for an `any` rest or `, ...T` for a typed rest; then `}`. A key that is a `Word` is bare, any other key is in its JSON form. A record with no fields and rest `never` is `{}`. |
| A record with no fields and a rest that is not `never` | `map<T>`, with T the rest: `map<any>`, `map<float>`. |
| An opaque type | Its name. |

| Type | Text |
| --- | --- |
| The empty list's type | `never[]` |
| A list of lists | `int[][]` |
| A literal set that may be null | `"fail" \| "pass" \| null` |
| A list that may be null | `int[] \| null` |
| A list of elements that may be null | `(int \| null)[]` |
| An open record | `{action: string, ...}` |
| A record with an optional field and a nullable one | `{note?: string, owner: string \| null}` |

## 4. Operations on types

### 4.1 Join

Join(A, B) is the type of a value that has type A or type B.

| Case | Join(A, B) |
| --- | --- |
| Either is `any` | `any` |
| A kind in one only | That alternative, unchanged. |
| null, bool, in both | That alternative. |
| number, in both | `int` if both are `int`, else `float`. |
| string, in both | The union of the sets if both are literal sets, else `string`. |
| list, in both | The list of Join(the two element types). |
| opaque *N*, in both | *N* |
| map, in both: two records | One record. A field in both: the Join of its two types, optional if it is optional in either. A field in one only: optional, and its type is joined with the other record's rest, unless that rest is `never`. The rest: the Join of the two rests. |

- Join is commutative and associative. Join(T, `never`) is T.
- **Join never puts two records in one type.** Two records join into one record
  by the last row, whatever their fields.
- The map rows that follow-on F5 adds for a tagged union apply only when a side
  holds two or more variants, which no Core type does.

### 4.2 Fits

Fits(S, T) is the one assignability relation. Some fits **use any**.

| Case | Fits when |
| --- | --- |
| S is `never`, or T is `any` | Always. |
| S is `any` (and T is not `any`) | Always. The fit **uses any**. |
| Otherwise | Each alternative of S fits the alternative of T of the same kind, by the rows below. An alternative of S whose kind T lacks does not fit. |
| null, bool | They fit. |
| number | `int` fits `int` and `float`. `float` fits `float`. |
| string | A literal set fits a literal set that holds all its strings, and fits `string`. `string` fits `string`. |
| list | Fits(element of S, element of T). |
| opaque | *N* fits *N*. |
| map: a record S against a record T | (1) For each field of T: S lists it, its type in S fits its type in T, and it is required in S or optional in T; or S does not list it, it is optional in T, and S's rest fits its type in T. (2) Each field of S that T does not list fits T's rest. (3) S's rest fits T's rest. |

- A fit **uses any** when any step of it reaches the row "S is `any`". A fit
  that holds and does not use any holds for every value of S.
- `{a: int}` fits `map<int>`, `{a: int, ...}` and `{a: int, b?: string}`. `{a:
  int, b: int}` does not fit `{a: int}`, and `{a?: int}` does not fit `{a:
  int}`.

### 4.3 Merge

Merge(R, M) is the record made by writing the entries of a map of record type M
over a map of record type R. It types a map spread and a computed key (7.5).

| Case | Result |
| --- | --- |
| M is `any` | Every field of R, with type `any` and its mark kept; rest `any`. |
| A field required in M | That field, required, with M's type. |
| A field optional in M | The Join of M's type with R's type for that key (R's field, else R's rest). Required only if R lists it as required. |
| A field of R that M does not list | Unchanged if M's rest is `never`. Otherwise its type joined with M's rest, its mark kept. |
| The rest | The Join of the two rests. |

### 4.4 Truthy and Falsy

The falsy values are null, `false`, zero, the empty string, the empty list and
the empty map. Every other value is truthy (`01-language.md` 6.5). Truthy(T) is
the type of the truthy values of T, and Falsy(T) of the falsy values. Each is
the set of the parts of T's alternatives below. A dash is no part.

| Alternative | Truthy part | Falsy part |
| --- | --- | --- |
| `null` | – | `null` |
| `bool` | `bool` | `bool` |
| `int`, `float` | The same | The same |
| `string` | `string` | `""` |
| A literal set | The set without `""`; – if nothing is left | `""` if the set has it, else – |
| `T[]` | `T[]`; – if T is `never` | `never[]` |
| A record | The record; – if it has no fields and its rest is `never` | `{}` if it has no required field, else – |
| Opaque *N* | *N* | – |

| Outside the table | Truthy | Falsy |
| --- | --- | --- |
| `any` | `any` | `any` |
| `never` | `never` | `never` |

- A part with no alternative is `never`. Truthy(`null`) is `never`. Falsy(`{id:
  int}`) is `never`. Falsy(`Time`) is `never`.
- Join(Truthy(T), Falsy(T)) is T for every T.
- Truthy and Falsy look at alternatives and do not recurse into them.

### 4.5 Helpers

| Helper | Definition |
| --- | --- |
| nonnull(T) | T without its null alternative. nonnull(`any`) is `any`. nonnull(`null`) is `never`. |
| overlap(A, B) | False if either is `never`. True if either is `any`. Otherwise true when some kind has an alternative in both, except that two literal sets must share a string. It looks at kinds only and does not recurse. |
| has-opaque(T) | T has an opaque alternative, or an element, field or rest type of T has one, at any depth. `any` does not have one. |
| has-any(T) | T is `any`, or an element, field or rest type of T is `any`, at any depth. |
| finite(T) | T is not `any`, and every alternative of T is `null`, `bool` or a literal set. The **values** of a finite type are null if it has `null`, `true` and `false` if it has `bool`, and each string of its literal set. |
| record of T | For a type whose map slot holds a record, that record. (F5 defines it for a slot with variants.) |

Follow-on F4 adds the helpers disjoint, without and surely fits. Follow-on F5
adds flat, hit and all.

## 5. Scope and names

### 5.1 Scopes

Scopes nest. A name refers to the innermost binding of that name in scope.

| Scope | Holds | Encloses |
| --- | --- | --- |
| The environment | Its variables | The program |
| The program | Its `const` names | Its result |
| A `Block`, and a branch of an `if` | Its `const` names | Its result |
| A lambda | Its parameters' names | Its body |
| The body of a `try` | Its `const` names | Its result |
| The handler of a `try` | The name of `catch (err)`, then its `const` names | Its result |

| Rule | Text |
| --- | --- |
| Order | A `const` name is in scope from the end of its `const` line to the end of its scope. A `const` value is checked before its names exist, so a name is not in scope in its own value. |
| A default | A default in a pattern sees the names bound to its left in the same pattern (12). |
| The handler | A name bound in the body of a `try` is not in scope in its handler. |
| `_` | Binds nothing and cannot be read. |
| Unused names | Not an error. |
| Shadowing | A binding may shadow an outer binding or an environment variable. |
| `duplicate-name` | A name bound twice in one scope, or twice in one pattern or one parameter list. The later binding stands. A static rule: 22.7 reports it. |
| `shadows-namespace` | A binding whose name is a namespace of the environment (5.4). No binding is made. A static rule: 22.7 reports it. |

### 5.2 Two kinds of binding

| Binding | Made by | Has |
| --- | --- | --- |
| **const-bound lambda** | A `Const` whose pattern is a name and whose value is a `Lambda` | The lambda. It has no type. |
| **value binding** | An environment variable; any other `const` name; a parameter's name; a name in a pattern; the name of `catch (err)` | A type. |

A parameter is never bound to a lambda (13.2).

### 5.3 Resolving a name

| Use | Resolution |
| --- | --- |
| An `Ident` | The innermost binding. A value binding gives its type, narrowed by the facts in force (10.4). Anything else (a const-bound lambda used as a value, a namespace, a function of the environment, or no binding) gives `never`. |
| The name of a `Call` | The innermost binding. A const-bound lambda is called by 13.9. A value binding: the call has type `never`. With no binding: the function of the environment of that name, with its namespace (a host function or a library function), called by 13.5; with none, the call has type `never`. |
| A `MethodCall` with a plain `.` whose receiver is an `Ident` that names a namespace | It is a `Call` of the function of that namespace (5.4). With `?.`, as `geo?.distance(a, b)`, it is not: the `Ident` is read as a value, which is `not-a-value` (22.4). |

So the callee of every call is known from the scopes alone: a const-bound
lambda, a function of the environment, or an error. The errors of resolution
(`undefined-name`, `not-a-value`, `call-of-parameter`, `undefined-function`) are
static rules, reported by 22 and never by the type rules. A call whose callee is
not a function has type `never`: its plain arguments are synthesized, and its
lambda arguments are not checked by the type rules.

### 5.4 Namespaces

| Matter | Rule |
| --- | --- |
| A namespace | The prefix of a host function's name, as `geo` in `geo.distance` (19.2). Follow-on F6 adds the opt-in library groups as namespaces. |
| As a value | `not-a-value` on the identifier (22.4). |
| As a binding's name | `shadows-namespace` (22.7). |
| A namespace with no such function | `geo.nothing(x)` is `undefined-function` on `geo.nothing` (22.5). |

## 6. Checking

### 6.1 The walk

The checker has two parts. The **static rules** (22) need the text and the
environment's names, and no type. Every implementation applies them, whether or
not it has a type checker. The **type rules** are sections 4 to 15. A static
diagnostic (22.1) is reported by the static rules only, once for each place in
the text, and the type rules never report one. The full checker's diagnostics
are the union of the two parts.

The type rules check the `Program` node (6.3). They report every diagnostic they
find. A node with a diagnostic has the type that 16.1 gives, and checking goes
on. The result of a check is the program's type, its sites and its diagnostics.
When there is a diagnostic, the type and the sites are not part of the result.

Diagnostics are a set: two with the same code and the same span are one. They
are ordered by start position, then end position, then code in code point order.
Sites are a set in the same way, ordered by start, end, then class.

The result does not depend on the order in which an implementation visits nodes.

### 6.2 Facts in force

A fact holds in every node inside the node where it is in force, lambda bodies
included, except where this chapter says otherwise (`try`, 14). A fact never
reaches a binding that shadows its path's name (10.4).

### 6.3 Node kinds

The node kinds are those of `01-language.md`. In Core every kind but 31 and 32
can occur.

| # | Node | Rule |
| --- | --- | --- |
| 1 | `Program` | Check each `Const` in order, then the result. Then the expected result (15.3). The program's type is the result's type. |
| 2 | `Const` | 12.1 |
| 3 | `Block` | 11.3 |
| 4 | `Ident` | 5.3 |
| 5 | `Number` | 7.1 |
| 6 | `String` | 7.2 |
| 7 | `Template` | 7.3 |
| 8 | `Bool` | `bool` |
| 9 | `Null` | `null` |
| 10 | `List` | 7.4 |
| 11, 12 | `Map`, `Entry` | 7.5 |
| 13 | `Spread` | 7.7 |
| 14 | `Unary` | 8.1 |
| 15 | `Binary` | 8.2 to 8.7 |
| 16 | `Cond` | 11.1 |
| 17, 18, 20, 21 | `Member`, `Index`, `MethodCall`, `OptChain` | 9 |
| 19 | `Call` | 13 |
| 22 | `Lambda` | 13.2, 13.9. A lambda has no type. |
| 23 | `Param` | 12.4 |
| 24, 25 | `Match`, `Arm` | 11.2 |
| 26 | `Try` | 14 |
| 27, 28, 29 | `NamePat`, `MapPat`, `ListPat` | 12 |
| 30 | `Bad` | `never`. The parser reported it, and a program with a parse error is not checked. |
| 31 | `As` | Follow-on F4. Not in Core. |
| 32 | `Each` | Follow-on F7. Not in Core. |

## 7. Literals and building values

### 7.1 `Number`

`int` if the value is a whole number in the sense of 2.1, else `float`. The
spelling does not matter: `2.0` and `1e3` are `int`, `1e20` and `0.5` are
`float`. The checker folds no constants: `10 / 2` is a `float`.

### 7.2 `String`

The literal set of its one string: the value after escapes. A backtick literal
with no `${ }` is a `String` too.

### 7.3 `Template`

Synth each expression part. Each expression part is a rendered expression. The
type is `string`.

A **rendered expression** is an expression whose value the run renders as text
(`01-language.md` 12). The rendered expressions of Core are each expression part
of a `Template`, and each argument of a library entry at a parameter that its
record names in the field `renders` (13.10). For a rendered expression e of type
T:

| T | Rule |
| --- | --- |
| has-opaque(T) | `kind` on e |
| Otherwise, has-any(T) | A `dynamic` site on e |
| Otherwise | Nothing. Null is accepted. |

### 7.4 `List`

The type is `J[]`, where J is the Join of the type of every item and of the
element type of every spread's operand (7.7). With no items it is `never[]`. The
depth limit applies (20.4).

### 7.5 `Map`

Start with `{}`. Take the entries in order.

| Entry | The record becomes |
| --- | --- |
| `k: v` with k a `Word` or a `String`, or a shorthand name `k` | The record with field k set, required, to the type of v (for a shorthand, the type of the `Ident` k). |
| `[e]: v`, a computed key | e must fit `string` (`kind`; null is `maybe-null`). The record becomes Merge(record, `map<V>`), with V the type of v. |
| `...e`, a spread | Merge(record, M), with M the record of e's type (7.7). |

- A `Word` or `String` key equal to an earlier `Word`, `String` or shorthand key
  of the same map literal is `duplicate-key`, a static rule (22.8). For the
  type, the later entry stands.
- The depth limit applies (20.4).

### 7.6 Merge in a map literal

`{a: 1, ...m}` with `m: {b: string}` is `{a: int, b: string}`. `{a: 1, [k]:
"x"}` is `{a: int | "x", ..."x"}`. `{...gate, failed: "none"}` replaces the type
of `failed`.

### 7.7 `Spread`

| Place | Rule |
| --- | --- |
| In a list | The operand must be a list (`kind`; null is `maybe-null`). Its element type joins the list's element type. |
| In a map | The operand must be a map (`kind`; null is `maybe-null`). Its record is M in 7.5. |
| An operand of type `any` | A `dynamic` site on the operand. In a list the element type is `any`; in a map M is `any`. |
| After `kind` | The spread adds nothing to the list or map type. |
| An operand of type `never` | Nothing is reported, and the spread adds nothing: `[...fail("x")]` is `never[]`, and in a map the record is unchanged. |

## 8. Operators

### 8.1 `Unary`

| Form | Rule | Type |
| --- | --- | --- |
| `!x` | x is a tested position (10.1). | `bool` |
| `-x` | x must fit `float` (`kind` on x; null is `maybe-null`). An `any` operand records a `dynamic` site. | The type of x, after nonnull; `any` for an `any` operand. |

### 8.2 The table of binary operators

| Operator | Operands | Type | Diagnostics and sites |
| --- | --- | --- | --- |
| `a && b` | Any types. a is a tested position. b is checked under the true facts of a (10.5). | Join(Falsy(A), B) | From the tested position (10.2). |
| `a \|\| b` | Any types. a is a tested position. b is checked under the false facts of a. | Join(Truthy(A), B) | The same. |
| `==`, `!=` | Any types. | `bool` | `never-equal` on the whole `Binary` when overlap(A, B) is false, neither A nor B is `never`, and neither is exactly the type `null`. Never a site. |
| `<`, `<=`, `>`, `>=` | Both fit `float`, or both fit `string` (8.3). | `bool` | `kind` (8.3). |
| `in`, `not in` | 8.4 | `bool` | 8.4 |
| `??` | Any types. | Join(nonnull(A), B) | None. |
| `+` | Both fit `float`, or both fit `string` (8.3). | `int` if both fit `int`; `float` if both fit `float`; `string` if both fit `string` | `kind` (8.3). |
| `-`, `*`, `%` | Both fit `float`. | `int` if both fit `int`, else `float` | `kind` on each operand that does not fit `float`. `%` records a `divide` site on the `Binary` unless b is a `Number` node whose value is not 0. |
| `/` | Both fit `float`. | `float` | `kind` on each operand that does not fit `float`. `zero-divisor` on b when b is a `Number` node with value 0. A `divide` site on the `Binary` unless b is a `Number` node whose value is not 0. |
| `**` | a fits `float`, b fits `int`. | `int` if a fits `int` and b is a `Number` node, else `float` | `kind` on a if it does not fit `float`, on b if it does not fit `int`. A `divide` site on the `Binary` unless b is a `Number` node, or a is a `Number` node whose value is not 0. |

A `Number` node is never negative: `-1` is a `Unary` node.

### 8.3 Null, `any` and `kind` in the operators

These rules apply to every operator but `&&`, `||`, `==`, `!=` and `??`. For
`in` and `not in`, 8.4 says to which operand the null rule applies.

| Matter | Rule |
| --- | --- |
| Null | An operand that may be null is `maybe-null` on that operand. Checking continues with nonnull of its type. |
| `any` | An operand of type `any` records a `dynamic` site on the operand. The type of the node is then `any`, except that `<`, `<=`, `>`, `>=`, `in` and `not in` stay `bool`. |
| `kind` on one operand | When an operand, after nonnull, fits no type the operator takes in that position, `kind` is reported on that operand. For `<`, `<=`, `>`, `>=` and `+` an operand must fit `float` or fit `string`. |
| `kind` on the whole `Binary` | For `<`, `<=`, `>`, `>=` and `+`: when each operand fits `float` or `string`, and one fits only `float` while the other fits only `string`, `kind` is reported once, on the whole `Binary`. |
| After `kind` | The node has type `bool` for `<`, `<=`, `>`, `>=`, `in` and `not in`, and `never` for the others. |
| `never` | A `never` operand fits every type. `a - b` with a `never` operand has type `int` when the other fits `int`. |

### 8.4 `in` and `not in`

The null rule of 8.3 applies to the right operand always, and to the left
operand only when the right is a string or a record (the fourth row). Let A be
the type of the left operand, and B the type of the right after the null rule.
So `ni in [1, 2]`, with `ni: int | null`, is `bool` with no diagnostic, and `ns
in s`, with `ns: string | null` and `s: string`, is `maybe-null` on `ns`.

| B | Rule |
| --- | --- |
| `any` | A `dynamic` site on the right operand. The left operand may have any type. |
| `never` | Nothing more is checked. |
| A list `E[]` (and no other kind) | The left side is compared as by `==` with E: `never-equal` on the whole `Binary` when overlap(A, E) is false, neither A nor E is `never`, and neither is exactly `null`. So `"a" in []` is not reported: E is `never`. No site for the left side, and null on the left is accepted. |
| `string`, a literal set, or a record (and no other kind) | The left side must fit `string` (`kind` on the left operand; null is `maybe-null`; `any` records a `dynamic` site). |
| Two or more kinds, or another kind | `kind` on the right operand. |

### 8.5 Equality

`==` and `!=` accept every type and never fail. A comparison with the literal
`null`, or with a value whose type is exactly `null`, is never `never-equal`.
Nor is a comparison with a side of type `never`: `n == fail("x")` reports
nothing. Equality is not a test: neither operand is a tested position.

### 8.6 Order

The rows of 8.2 decide everything. `"a" < "b"` is `bool`. `1 < "b"` is `kind` on
the whole `Binary`. `[1] < [2]` is `kind` on each operand.

### 8.7 Numbers

| Point | Rule |
| --- | --- |
| `int` is sound | `+`, `-`, `*`, `%` and unary `-` on whole operands give a whole number or fail with `number-range` (`02-evaluation.md`). That failure has class `range` and is never a site. |
| Where `int` is required | A list or string index, the exponent of `**`, and a parameter of type `int`. A `float` there is `kind` (or `arg-mismatch` at an argument). |
| `/` | Always `float`. `xs[len(xs) / 2]` is `kind` on the index. |
| Truthiness | No type says a number is zero, so a number in a tested position is never a constant condition. |

## 9. Reads and chains

### 9.1 The walk of a chain

A chain is a primary and the steps that follow it (`01-language.md`). An
`OptChain` node holds a chain that has an optional step. The checker walks a
chain from left to right with a current type *cur* and a flag *skipped*, false
at the start.

| Step | Rule |
| --- | --- |
| 1. Start | *cur* is the type of the primary. |
| 2. Before a plain step (`.f`, `[e]`, `.m(args)`) | If *cur* has null: `maybe-null` on the receiver, and *cur* becomes nonnull(*cur*). |
| 3. Before an optional step (`?.f`, `?.[e]`, `?.m(args)`) | If *cur* has null: *skipped* is set, and *cur* becomes nonnull(*cur*). |
| 4. One kind | *cur* must now have exactly one kind, or be `never`, or be `any`. Otherwise `kind` on the receiver, and *cur* becomes `never`. |
| 5. The step | On `any`: the step records a `dynamic` site on the receiver, and *cur* stays `any` (9.3, 13.3). On `never`: 9.4. Otherwise *cur* becomes the result of the read (9.2) or of the method call (13.3). |
| 6. End | The chain's type is *cur*, with the null alternative added if *skipped* is set. |

The **receiver** of a step is the chain from its start up to that step. Its span
is the span of a diagnostic or site "on the receiver". A step's index expression
and a method call's arguments are expressions of their own.

### 9.2 Reads

| Receiver | Step | Plain | Optional |
| --- | --- | --- | --- |
| A record | Field `f`, or `["f"]` with a `String` node: f required | Its type | Its type |
| | The same: f optional, or unlisted with a rest that is not `never` or `any` | `maybe-absent`; continues with the field's or the rest's type | That type with null added |
| | The same: f unlisted, rest `any` | `any`, and a `dynamic` site on the receiver | `any` |
| | The same: f unlisted, rest `never` | `no-field`; `never` | `no-field`; `never` |
| A record | `[e]` with any other e: e must fit `string` (`kind` on e; null is `maybe-null`) | `maybe-absent`; or, if the rest is `any`, a `dynamic` site on the receiver and no diagnostic; continues with V. When e does not fit `string`: `kind` on e and nothing else; the read has type `never` | V with null added, and no site. When e does not fit `string`: as for plain |
| A list `T[]` | `[e]`: e must fit `int` (`kind`; null is `maybe-null`) | T, and an `index` site on the `Index` node | T with null added |
| `string` or a literal set | `[e]`: e must fit `int` | `string`, and an `index` site on the `Index` node | `string \| null` |
| Anything else | A field or an index | `kind` on the receiver; `never` | `kind` on the receiver; `never` |

- A receiver of type `any` is read by 9.3, which also records an `index` site
  for a plain index step.
- V is the Join of every field type of the record and its rest.
- A key that a fact says is present (10.4) reads as required: its type is the
  field's type, or for an unlisted key nonnull of the rest. No `maybe-absent` is
  reported, and for a rest of `any` no site is recorded. A key unlisted in a
  record whose rest is `never` is still `no-field`.
- So `scores[1]`, with `scores: map<float>`, is `kind` on `1` only. `op?.[s]` on
  a record whose rest is `any` has type `any` and records no site; the plain
  `op[s]` records its `dynamic` site.
- The span of `maybe-absent` is the field's name, the `String` key, or the index
  expression. The span of `no-field` is the field's name or the `String` key.
- An absent optional field read with `?.` gives null for that step only. The
  chain goes on. So with `b?: {c: int}`, `a?.b.c` is `maybe-null` on `a?.b`, and
  `a?.b?.c` is `int | null`.

### 9.3 `any` in a chain

Every step on an `any` receiver records a `dynamic` site on that receiver, and
gives `any`. An optional step on `any` records the same site: the kind is still
unknown. An index expression is synthesized, and it need not fit anything.

| Step on an `any` receiver | Sites besides the `dynamic` site on the receiver |
| --- | --- |
| A plain index step `[e]` | An `index` site on the `Index` node: the value may be a list or a string, and the index may be out of range. |
| An optional index step `?.[e]` | None: an index out of range gives null. |
| A method call, plain or optional | 13.3: a `value` site or a `host` site on the call, by the method's name. |

The receiver is `any` here when *cur* is `any` after step 2 or 3 of 9.1.

### 9.4 `never` in a chain

A step on a `never` receiver gives `never` and reports nothing for the step. Its
index expression is synthesized. A method call's plain arguments are synthesized
and its lambda arguments are not checked.

### 9.5 Unions in a chain

Step 4 is where a union from `&&`, `||`, `? :`, `if` or `match` shows.
`(gate.failed || "none").trim()` is `kind` on the receiver: the author must say
which kind is meant.

## 10. Conditions and narrowing

### 10.1 Tested positions

A value is tested by truthiness in these places. The test cannot fail, so a
tested position accepts every type, and an `any` value there records no site.

| Construct | The tested expression |
| --- | --- |
| `c ? a : b` | c |
| `if (c) { a } else if (d) { b } else { e }` | c and d |
| `a && b`, `a \|\| b` | a. b is a result, not a test. |
| `!a` | a |
| A lambda argument where the signature's `Fn` result is `unknown` (13.7) | The lambda's body |
| An argument of a library function at a position that its `library.json` record lists in the field `tested` (`04-library.md`). In Core: the argument of `bool` | That argument |
| A program whose expected result is `condition` (15.3) | The program's result |

### 10.2 Constant conditions

Let C be the type of a tested expression, as narrowed.

| Code | Reported when | Span |
| --- | --- | --- |
| `always-truthy` | Falsy(C) is `never` and C is not `never` | The tested expression |
| `always-falsy` | Truthy(C) is `never` and C is not `never` | The tested expression |

- `any` and `never` report neither.
- Both codes are of the check class `constant-condition` (17). They are reported
  only when the environment rejects that class. With the class not rejected,
  nothing is reported and nothing changes.
- The node keeps its type, and checking goes on.
- The checker looks at types and not at values. `true ? a : b` and `1 || x` are
  not reported: `bool` and `int` have both parts. `"x" || y`, `null || y` and
  `[] || y` are reported, because the types of those literals have one part.
- A tested expression that is itself `a && b` or `a || b` is tested by its own
  type as well as its operand a.
- `x != null ? a : b`, `x ?? d` and `x?.f` with x never null are never reported.
  Neither is a `null` case of `match`, nor a `_` arm after full coverage.

### 10.3 Paths

A **path** is an `Ident` followed by zero or more field steps: `.f`, `?.f`,
`["f"]` or `?.["f"]` with a `String` node. Two paths are the same path when
their `Ident`s refer to the same binding and their steps name the same keys in
the same order. Whether a step is written `.f`, `?.f`, `["f"]` or `?.["f"]` does
not matter. A call, an index with any other expression, and a method call are
not paths. A `const` name is a path, with its value's type as narrowed where the
`const` stands.

### 10.4 Facts

In Core there is one kind of fact: **not null**, written {p}, for a path p.
Follow-on F4 adds the facts "conforms" and "does not conform", and F5 gives them
for tests of a field.

| Matter | Rule |
| --- | --- |
| Meaning | {p}: the value of p is not null. |
| Effect | An expression that is the path p has nonnull of its type. |
| Prefixes | {p} for p = `x.a.b` also says that `x` and `x.a` are not null, and that the keys `a` and `b` are present: a read of either reads it as required (9.2), whether or not the record lists the key. |
| Lifetime | A fact is about a binding. It does not reach an inner binding of the same name. Nothing ends a fact: no value changes. |
| Several facts | They are applied in the order of their tests in the source. |
| Two facts | Are the same when they are about the same path (10.3). The ∩ of 10.5 compares by this. |

### 10.5 True and false facts

Each tested expression c has **true facts** T(c), which hold where c was truthy,
and **false facts** F(c), which hold where c was falsy.

| c | T(c) | F(c) |
| --- | --- | --- |
| A path p | {p} | none |
| `p != null`, `null != p` | {p} | none |
| `p == null`, `null == p` | none | {p} |
| `!d` | F(d) | T(d) |
| `d && e` | T(d) ∪ T(e) | F(d) ∩ F(e) |
| `d \|\| e` | T(d) ∩ T(e) | F(d) ∪ F(e) |
| Anything else | none | none |

Here `null` is a `Null` node. The facts apply:

| Where | Under |
| --- | --- |
| e in `d && e` | T(d) |
| e in `d \|\| e` | F(d) |
| The two branches of `c ? a : b` | T(c) and F(c) |
| The branches of an `if` chain | 11.1 |
| The arms of a `match` | 11.2 |
| The result of `filter` and `find` | 13.8 |

A program's result tested under `condition` has facts that apply nowhere. A
false branch of a bare path concludes nothing: where x was falsy it may be null,
`false`, zero or empty, and its type already allows each.

## 11. Choice and blocks

### 11.1 `Cond`

`c ? a : b`: c is a tested position. a is checked under T(c) and b under F(c).
The type is Join(A, B).

The `if` form, `if (c1) { b1 } else if (c2) { b2 } else { d }`, is checked
exactly as `c1 ? b1 : (c2 ? b2 : d)`, with each branch a `Block` (11.3). So each
test is a tested position, a test's true facts hold in its branch, the false
facts of every earlier test hold in each later test and branch, each branch is a
scope for its `const` names, and the type is the Join of the branches.

### 11.2 `match`

A `match` has a subject and arms. Each arm is one or more literals joined by
`|`, or `_`, and a result. Let S be the type of the subject. The subject is not
a tested position.

| Step | Rule |
| --- | --- |
| Literal types | A literal has the type of the same literal expression: a `Number` by 7.1 (with `-`, the same by its value), a `String` its literal set, `true` and `false` `bool`, `null` `null`. |
| `never-equal` | A literal L whose type does not overlap S (4.5), under the conditions of `==` (8.2): S is not `never`, S is not exactly the type `null`, and L is not `null`. Reported on the literal, its `-` included. So a `match` on a subject of type `never` or of type `null` reports no `never-equal`, and `match null { 0 \| null \| "" => 0 }` checks clean. A literal equal to an earlier one of the same `match` is ignored here too: it is `duplicate-case` only (22.8). |
| `duplicate-case`, `unreachable-arm` | Static rules (22.8). A literal equal to an earlier one is ignored for every type rule, `never-equal` and coverage included. An arm after `_` is still checked and joined. |
| Coverage | Without an `_` arm, S must be finite (4.5) and every value of S must be a literal of some arm. Otherwise `non-exhaustive` on the whole `Match`. |
| `_` after full coverage | Accepted, with no diagnostic. |
| Narrowing | If the subject is a path p, then {p} holds in every arm after an arm that lists `null`. |
| Type | The Join of the arm results, in order. |

A case is compared with `==`, never by truthiness: `match n { true => 1, _ => 0
}` on an `int` is `never-equal` on `true`. An arm can be unreachable in three
ways, each with its own code: `never-equal`, `duplicate-case`,
`unreachable-arm`.

`match` with no subject is not part of the language. Follow-on F5 adds narrowing
of a tagged union by a `match` on its tag.

### 11.3 `Block`

A `Block` is a new scope. Its `const` lines are checked in order (12.1), then
its result. Its type is the result's type. Facts in force outside hold inside. A
branch of an `if` or a `try` with no `const` line is its expression, and is
checked as that expression.

## 12. Bindings and patterns

### 12.1 `Const`

| Form | Rule |
| --- | --- |
| `const f = Lambda;` (pattern is a name) | f is bound to the lambda (5.2, 13.9). |
| `const p = e;` otherwise | Synth e to V, then bind the pattern p against V (12.2). |

### 12.2 Patterns

A pattern is bound against a type V: a `const` value's type, or the type a
parameter receives.

| Pattern | Rule |
| --- | --- |
| `x` | x has type V. |
| `_` | Binds nothing. |
| `{...}`, a `MapPat` | V may be null: `maybe-null` on the value bound (12.3), continuing with nonnull(V). V must then be a record (`kind` on the value bound), or `any` (a `dynamic` site on the value bound; every name, the rest included, is `any`). Each field is read as the plain read `V.key` by 9.2, with its diagnostic or site on the key of the field in the pattern: a field unlisted in a record whose rest is `any`, as `const {zz} = event;`, has type `any` and records a `dynamic` site on its key. A field with a default is read by 12.4. |
| `[...]`, a `ListPat` | V may be null as above. V must then be a list (`kind`), or `any` (a `dynamic` site on the value bound; every name, the rest included, is `any`). Each element name has the element type (`any` for an `any` V), and records an `index` site on the name, for a list and for `any` alike. An element `_` binds nothing and records the `index` site on the `_` too: the run needs its position. An element with a default is bound by 12.4 and records no `index` site. |
| `...rest` in a map pattern | The record V without the fields the pattern names. An exact record stays exact, and an open record keeps its rest. |
| `...rest` in a list pattern | V. No site. |

After `kind` on a pattern, every name of the pattern has type `never`. When V is
`never`, or becomes `never` after `maybe-null` because the value's type is
`null`, nothing more is reported for the pattern, and every name of it has type
`never`.

**A default is always checked**, whatever happened to the value bound: after
`kind` on the pattern, and when V is `null` or `never`. Each default is an
expression of its own (12.4). The names to its left have the types the pattern
gave them, `never` after a failure. So `const {b = 0[""]} = ""; 0` reports
`kind` on `""` and `kind` on `0`.

### 12.3 The value bound

The span of a diagnostic or site "on the value bound" is the `const` value
expression for a `const`. For a parameter it is the parameter's pattern.

### 12.4 Defaults

| Pattern part | Rule |
| --- | --- |
| A field `name = d` | d is checked, always (12.2), with the names to its left in scope. The field is read as by `?.`, so `maybe-absent` is not reported. With T that read's type and D the type of d, the name has Join(nonnull(T), D). |
| An element `name = d` | The same with T the element type. No `index` site is recorded. |
| A parameter `p = d` | When the argument is given with type A, the name has Join(nonnull(A), D). When the argument is omitted, the name has D. |

A default replaces null as well as absence. It does not replace another falsy
value. `{note = null}` gives `note` the type `T | null`.

## 13. Signatures and calls

### 13.1 Where signatures come from

| Source | Rule |
| --- | --- |
| The library | Each entry of `library.json` (`04-library.md`) gives a name, a `receiver` for a method (a `Recv` in type text) and a `signature` (a `Sig`). The `Var`s of the receiver and the signature are one set. An entry with the field `narrows` narrows by 13.8. An entry with the field `renders` renders the arguments it names (13.10). An entry with the field `tested` tests the arguments it names (10.1). |
| The environment | Each host function has a name, with at most one namespace prefix, and a `Sig`. Each method of an opaque type has a `Method` text (19). A host signature has no `Fn` parameter. |

Only the library entries of the environment exist. An entry that Core does not
have, such as `minBy`, is `no-method` or `undefined-function` like any other
missing name.

### 13.2 Lambda arguments

A **lambda argument** is a `Lambda` node, or an `Ident` that names a const-bound
lambda, given as an argument.

| A lambda argument | Result |
| --- | --- |
| At a parameter whose type is an `Fn`, in a call of a library function or method | Checked by 13.5, step 3. An `Ident` is checked as if the lambda were written there (13.9). |
| In a `Call` of a const-bound lambda | `lambda-to-lambda` on the argument, a static rule (22.5). The parameter it would bind has type `never`. The lambda argument is not checked by the type rules. |
| In a `Call` of a host or library function, at a parameter that is not an `Fn` | `not-a-value` on the argument, a static rule (22.5). It binds nothing, is not fitted, and is not checked by the type rules. |
| In a `MethodCall`, at a parameter that is not an `Fn` (any parameter of a host method, or a parameter of a library method that is not an `Fn`) | `not-a-value` on the argument. This is a **type rule**: the method is known only from the receiver's kind. It binds nothing, is not fitted, and is not checked by the type rules. |

A plain argument at an `Fn` parameter is `arg-mismatch` on the argument, a type
rule in a `Call` and in a `MethodCall` alike. It is synthesized and binds
nothing.

At run time, at every `MethodCall`, the run selects the method by the receiver's
kind and makes the shape test of `04-library.md` 2.2 before any argument is
evaluated: the method's name, the count of arguments, a lambda at each callback
parameter, no lambda at another parameter, and the lambda's count of parameters.
The first that fails is `wrong-kind` at the `MethodCall`. It does so whether or
not the program was checked: a method the checker resolved changes nothing at
run time.

### 13.3 Resolving a method

| Matter | Rule |
| --- | --- |
| By kind | A method is found by the one kind of its receiver (9.1, step 4): the library's methods for string, list and map, and the declared methods of an opaque type. No method of that name for that kind is `no-method` on the method's name. |
| A record receiver | Has the map methods. |
| Argument zero | The receiver is argument zero. It is bound against `Recv` in step 2 of 13.5, before the other arguments, and fitted in step 5 like any plain argument. Its type is the type the chain gives the receiver at that step: *cur* after steps 2 to 4 of 9.1, so with null removed after `?.` or after `maybe-null`. `report?.keys()` reports nothing for the receiver. |
| An `any` receiver | The call records a `dynamic` site on the receiver. It also records a `value` site on the call when some library method of that name, for any receiver kind, is marked `!`, and a `host` site on the call when a method of that name of some opaque type that the environment declares is marked `!`. Each plain argument is synthesized. Each lambda argument's parameters are bound to `any` and its body is synthesized. The type is `any`. So `blob.repeat(2)` records a `dynamic` site on `blob` and a `value` site on the call. |
| `no-method` | The call has type `never`. Its plain arguments are synthesized. Its lambda arguments are not checked. |

### 13.4 Type variables

A `Var` of a signature stands for a type that each call solves. Each call starts
with every `Var` unbound and with no bounds gathered.

| Term | Rule |
| --- | --- |
| bind(P, A) | Gathers bounds for the `Var`s of a parameter type P from an argument type A, by the table below. |
| The solution of a `Var` | The Join of its gathered types. A `Var` is unbound only when bind gathered nothing for it. A `Var` that gained `never` is bound, with the solution `never`: `[].map(x => x + 1)` gives `x` the type `never`, and the call the type `int[]`, with no diagnostic. |

| bind(P, A) | Rule |
| --- | --- |
| A is `any` | Every `Var` in P gains `any`. |
| P is a `Var` | It gains A. |
| P has the null alternative | bind(nonnull(P), nonnull(A)). |
| P is `P'[]` and A has a list alternative | bind(P', A's element type). |
| P is `map<P'>` and A has a record | bind(P', the Join of A's field types and A's rest). |
| P is another record and A has a record | For each field of P that A lists, bind(P's field type, A's field type). |
| Anything else | Gathers nothing. |

The rows are tried in this order, and the first that applies is used.

### 13.5 Solving one call

The steps run in this order for a call of a library entry or a host function or
method.

| Step | Rule |
| --- | --- |
| 1. Count | The arguments must cover every required parameter, and must not be more than the parameters unless the last is a rest parameter. Otherwise `arity` on the whole call: the call has type `never`, each plain argument is synthesized, no lambda argument is checked, and the call is not checked further. For a `Call` this `arity` is a static rule (22.5) and the type rules do not report it. For a `MethodCall` (argument zero not counted) it is a type rule. At run time the shape test makes the same test at every `MethodCall` (13.2; `04-library.md` 2.2). |
| 2. Plain arguments | Argument zero first for a method, then left to right: synth the argument to A and bind(P, A). An argument taken by a rest parameter `...rest: P[]` is bound, and in step 5 fitted, against P. |
| 3. Lambda arguments | Left to right, each against its parameter's `Fn`, `(q1: Q1, ..., qn: Qn) => R`. A lambda with more than n parameters is `arity` on the lambda argument (for a const-bound lambda named as the argument, as `xs.map(f)`, on the `Ident` `f`), and the lambda is not checked; this `arity` is a static rule in a `Call` (22.5) and a type rule in a `MethodCall`. Replace each bound `Var` in the Qs by its solution. A `Var` still unbound in the Q of a parameter that the lambda declares is `cannot-infer` on the lambda, and the lambda is not checked. A lambda may declare fewer than n parameters; a `Var` unbound only in a Q for which the lambda declares no parameter is no error, and the body is checked. So `[1].reduce((a, x) => a, y => y)` is `cannot-infer` on the first lambda (it declares `a`, whose Q is the unbound `A`), and `[].reduce(() => {} - 0, () => 0)` checks the first lambda's body, which reports `kind` on `{}`. Bind each parameter of the lambda to its Q by 12.2 and 12.4. Synth the body to B. If R is `unknown`: 13.7. Otherwise bind(R, B); then, unless the seed rule fires (3a), B must fit R with every `Var` replaced by its solution, an unbound `Var` by `never` (`arg-mismatch` on the body; a fit that used any records a `dynamic` site on the body, 1.3). So `[1].flatMap(x => x)` is `arg-mismatch` on the body `x`: `int` does not fit `never[]`. For `map`, R is `U`: U gains B, and the fit holds. |
| 3a. The seed rule | In step 3, when a `Var` X in R also occurs in a Q and already had a solution A0 before the body was checked, and B does not fit A0, or fits A0 only by a fit that used any: discard the diagnostics and sites of that check of the body, set the solution of X to Join(A0, B), and check the body once more under that solution. The second check binds nothing. Its body's type must fit the solution of R as it stands after the seed (`arg-mismatch` on the body; a fit that used any records a `dynamic` site on the body). When B fits A0 without using any, the rule does not fire, and step 3 goes on with its fit. So `xs.reduce((a, x) => [a], 0)` is `arg-mismatch` on `[a]`, and in `xs.reduce((a, x) => c ? blob : a + x, 0)` the second check gives `a` the type `any`, so `a + x` records its `dynamic` site. |
| 4. Bounds | For each `where V: B`, the solution of V must fit `float` or `string` for `ordered`, and must fit `float` for `number`. An unbound V has solution `never`. A failure is `arg-mismatch` on the whole call. A bound that holds only by a fit that used any records a `dynamic` site on the call. |
| 5. Fit | Each plain argument, argument zero included, must fit its parameter type, with every `Var` replaced by its solution (an unbound `Var` by `never`), **through its form** (15.1). At each place of failure the code is `maybe-null` if the type there has null and fits once null is removed, else `arg-mismatch`. |
| 6. Result | The result type, with each `Var` replaced by its solution, and an unbound `Var` by `never`. If the signature has `!`: a `value` site on the call for a library entry, a `host` site on the call for a host function or method. After `arg-mismatch` the call keeps this type. |

### 13.6 Bounds

| Bound | Holds when the solution |
| --- | --- |
| `ordered` | fits `float`, or fits `string` |
| `number` | fits `float` |

A solution that may be null fits neither: `ids.sum()` with `ids: (int | null)[]`
is `arg-mismatch` on the call.

### 13.7 Predicates

An `Fn` whose result is `unknown` is a **predicate**. In step 3 of 13.5, when R
is `unknown`:

| Matter | Rule |
| --- | --- |
| The body | May have any type. Nothing is bound or fitted. |
| A tested position | The body is a tested position (10.1): `always-truthy` or `always-falsy` on the body. An `any` body records no site. |
| Facts | The true facts of the body apply only by 13.8. |

A host signature has no `Fn`, so `unknown` never appears in a host signature. A
host function that wants a condition takes a `bool`.

### 13.8 `filter` and `find` narrow

This rule applies to the library entries whose `library.json` record has the
field `narrows`. In Core these are the list methods `filter` and `find`.

| Matter | Rule |
| --- | --- |
| When | The lambda argument's first parameter is a name x. With a pattern there, with `_`, or with no parameter, nothing is narrowed. |
| The narrowed type N | The type that an `Ident` x has under the true facts of the body, with x bound to the element type. It is the element type when no fact is about x. |
| Which facts reach x | {x}, and by the prefix rule (10.4) {p} for a longer path p that starts with x. |
| Which do not | A fact about a field changes the type of no field of a record type: `orders.filter(o => o.note != null)` is still a list of orders whose `note` may be null. |
| The result | The result type with the element `Var` of the receiver replaced by N: `N[]` for `filter`, `N \| null` for `find`. |
| A const-bound lambda by name | Narrowed as if the lambda were written there. |
| An `any` receiver | 13.3. The result is `any`. |
| Depth | The result is a call result, where the depth limit applies (20.4). |
| A `Block` body | A `Block` gives no facts (10.5), so nothing is narrowed. |

`xs.filter(x => x != null)` with `xs: (int | null)[]` is `int[]`.
`items.filter(t => t.owner)` keeps the element type: the fact is about
`t.owner`, and `t` was never null.

### 13.9 const-bound lambdas

A lambda has no type. Each use of a const-bound lambda is checked on its own,
because only a use gives the parameter types. A **use** is a call by name, or
the name given as a lambda argument: at an `Fn` parameter, or to a method on an
`any` receiver (13.3). A read of the name as a value is not a use: it is
`not-a-value` (22.4), and it does not count as a reference for the row "No use"
below.

| Use | Rule |
| --- | --- |
| A call `f(a1, ..., an)` | It must give at least the required count of 22.3 and no more arguments than parameters (`arity`, a static rule, 22.5; the call has type `never`, and each plain argument is synthesized). Each plain argument is synthesized where the call stands. A lambda argument is `lambda-to-lambda` (13.2), and its parameter has type `never`. Each parameter's pattern is bound to its argument's type, with defaults by 12.4. The body is then checked in the scope, and under the facts, where the lambda was written. The call's type is the body's type. |
| f as a lambda argument at an `Fn` | Checked as if the lambda were written there, by 13.5 step 3, in the scope and under the facts where the lambda was written. |
| f as an argument of a method on an `any` receiver | Checked as if the lambda were written there, by 13.3: every parameter is bound to `any` and the body is synthesized, in the scope and under the facts where the lambda was written. Its sites are kept. So in ``const f = () => ``.l; m.r(f)`` with `m` of type `any`, the body is checked and `.l` on a string is `kind`. |
| No use | A const-bound lambda whose name no `Call` and no argument in its scope refers to (a read of the name as a value does not count) is checked once, where it is written, with every parameter bound to `any`. The sites of that check are dropped. Its diagnostics stand. |
| Static diagnostics in the body | Reported once, where the text stands, by 22, whether the lambda has no use, one use or many. A check of the body by the type rules reports none. |
| Limits | Each use counts against the limits of 20.2 and 20.3. Passing either is `check-limit` on the use, which then has type `never`. |

A use is checked by the lambda and its parameter types alone. An implementation
may keep the outcome of a body's check under that key, and counts a use whose
check it reuses as a check (20.3).

### 13.10 Rendered arguments

This rule applies to the library entries whose `library.json` record has the
field `renders`: a list of parameter names. Each argument at a named parameter
is a rendered expression (7.3). In Core the one such entry is the function
`string`, with `"renders": ["x"]`. So `string(started)`, `started` opaque, is
`kind` on `started`; `string(blob)` records a `dynamic` site on `blob`; and
`string(5)` records nothing. The argument is also fitted to its parameter, of
type `any`, by 13.5, which reports nothing.

## 14. `try` and `catch`

### 14.1 The rule

`try { body } catch (err) { handler }` and `try { body } catch { handler }`.

| Matter | Rule |
| --- | --- |
| Type | The Join of the body's type and the handler's type. |
| The two parts | Each is checked as a branch of an `if` is: a scope for its `const` lines, then its result (11.3). |
| The name | In scope in the handler only, with the type of 14.2. With no name, nothing is bound. A name that is a namespace is `shadows-namespace` (22.7), and nothing is bound. |
| Sites | Sites recorded in the body are dropped, of every class. Sites in the handler are recorded. |
| Diagnostics | Reported in both parts, as anywhere else. |
| Facts | A fact in force outside holds in both parts. No fact from the body holds in the handler. A `try` gives no fact. |
| A body that cannot fail | Not reported. |

### 14.2 The error value

In the handler the name has this exact record type:

```
{code: C, data: any, message: string, path: (int | string)[] | null}
```

| Field | Type |
| --- | --- |
| `code` | C: the literal set of the codes of phase `run` whose entry in `messages.json` has `caught: true`. For the specification version 3.0 these are the eleven codes `"divide-by-zero"`, `"failed"`, `"function-error"`, `"index-range"`, `"invalid-argument"`, `"missing-field"`, `"no-match"`, `"not-conforming"`, `"null-operand"`, `"number-range"` and `"wrong-kind"`. |
| `data` | `any` |
| `message` | `string` |
| `path` | `(int \| string)[] \| null` |

- It is an ordinary record, and every rule of this chapter applies to it.
  `err.stack` is `no-field`. A test of `err` alone is `always-truthy`.
- `err.code == "steps-exceeded"` is `never-equal`: the code is not in C.
- `match err.code { ... }` without `_` must list every code of C.
- The type has depth 2.

## 15. An expression against a written type

### 15.1 e fits T through its form

Two places set an expression against a type that someone wrote: the program's
result against an expected type (15.3), and a plain argument against its
parameter (13.5, step 5). There the fit is tried **through the form** of the
expression.

e fits T through its form when the type of e fits T. Failing that, one row of
this table decides. The type of e is the one synth gave it. Nothing is checked
twice, and no type changes.

| e | It fits when | Place of a failure |
| --- | --- | --- |
| A `List`, where T has a list alternative `E[]` | Each item fits E through its form, and the type of each spread's operand fits `E[]`. | The item, or a place inside it by these rows; the spread's operand. |
| A `Map` with no computed key, where T has a record R | By the table "A map literal against a record" below. With no spread: each entry whose key R lists fits that field's type through its form; each other entry fits R's rest through its form; each required field of R has an entry. | The entry's value, or a place inside it by these rows. The entry's key, when R does not list it and R's rest is `never`. A spread's operand. The whole of e, once, when one or more required fields have no entry. |
| A `Cond` of either form | Each branch fits T through its form. | By these rows, in each branch. |
| A `Match` | Each arm's result fits T through its form. | By these rows, in each arm. |
| A `Block` | Its result fits T through its form. | By these rows. |
| Anything else, or a row whose condition T does not meet | It does not fit. | e |

| Matter | Rule |
| --- | --- |
| Several failures | Each is reported. |
| A fit that used any | A `dynamic` site on the expression whose fit used it: e itself where the first test passed, or the part a row fitted. |
| Not followed | An `Ident`, a call, a chain, a `try`, an operator, a template. The report is on that node: `const cfg = {...}; cfg` is reported on `cfg`. |
| A shorthand entry | Its value is the `Ident`; its place is the shorthand name. |
| A computed key | A `Map` with a computed key is not followed: the place is e. |
| Follow-on | F5 adds a row for a `Map` with no spread against a tagged union. |

**A map literal against a record.** Let e be a `Map` with no computed key, and R
the record of T. The entries are taken in written order.

| Term | Definition |
| --- | --- |
| What a written entry gives | `k: v`, or a shorthand `k`: it surely gives the key k, with the type of v. |
| What a spread gives | Let M be the type of its operand, with null removed (7.7 reported it). For a record M: it may give each key M lists, with that field's type, and surely gives each key M lists as required; when M's rest is not `never`, it may also give every key M does not list, with the rest's type. So an exact record may give its fields only, an open record and `map<T>` may give any key, and `map<T>` surely gives none. For M of type `any`: it may give every key, with type `any`, and surely gives none. For M of type `never`, or a spread that was `kind` (7.7): it gives nothing. |
| The contributors of a key k | The last entry that surely gives k, and every entry after it that may give k. With no entry that surely gives k, every entry that may give k. |

The keys taken are those R lists, those written, and those any spread's record
lists; and one **other key**, standing for every key none of these name, whose
contributors are the spreads that may give every key.

| Check | Place of a failure |
| --- | --- |
| A written entry that is a contributor of its key: its value fits R's type for that key (R's field, or R's rest) through its form. A written entry that is not a contributor was overridden and is not fitted. | The value, or a place inside it by these rows. The entry's key, when R does not list it and R's rest is `never`. |
| A spread that is a contributor of a key: the type it gives for that key fits R's type for that key. For the other key, R's type is R's rest. | The spread's operand, once for each spread. |
| Each required field of R is surely given by some entry. | The whole of e, once. |

With no spread this is the row of a `Map` above, and gives the same results. A
spread's fit that used any records a `dynamic` site on its operand; for an
operand of type `any`, 7.7 has recorded it already. So in `{...base, prot:
8080}` against an exact record, the misspelled key is reported at `prot`, and a
field of `base` whose type does not fit is reported on `base`.

### 15.2 The code at a place of failure

| Place | Code |
| --- | --- |
| The program's result | `result-mismatch` at each place of failure. |
| A plain argument | At each place of failure: `maybe-null` if the type there has null and fits once null is removed, else `arg-mismatch`. |

### 15.3 The expected result

The host may give the checker an expected result (`03-host.md`).

| Expected | Rule |
| --- | --- |
| None | The result may have any type. |
| A type T | The result must fit T through its form (15.1), with `result-mismatch`. Nothing is converted: with an expected `bool`, a list does not fit. |
| `condition` | The result is a tested position (10.1). Every type is accepted. `always-truthy` or `always-falsy` on the result expression when its type allows one outcome only, under the class `constant-condition`. |

The expected result changes no type and no site. The program's type is the
inferred one.

## 16. Diagnostics

### 16.1 The codes of Core

The check codes of Core are 27. Every one is an error. The column "Class" names
the check class for a code that is reported only when that class is rejected
(17). The other codes are always reported. "Type afterwards" is the type of the
node whose rule reports the code, unless the row names another; checking goes on
with it.

Eleven codes are **static codes**: `undefined-name`, `undefined-function`,
`not-a-value`, `call-of-parameter`, `lambda-to-lambda`, `duplicate-name`,
`shadows-namespace`, `duplicate-key`, `arity`, `duplicate-case` and
`unreachable-arm`. Section 22 reports them, and its rules decide when and where.
Two of them, `arity` and `not-a-value`, are also reported by the type rules at
the arguments of a `MethodCall` (13.2, 13.5). The other 16 codes are reported by
the type rules only.

| Code | Reported when | Span | Class | Type afterwards |
| --- | --- | --- | --- | --- |
| `undefined-name` | An `Ident` that resolves to nothing (22.4) | The identifier | | `never` |
| `undefined-function` | A `Call` whose name resolves to nothing (22.5) | The name, with its namespace | | `never`. Plain arguments are synthesized; lambda arguments are not checked. |
| `not-a-value` | A namespace, a function of the environment, or a const-bound lambda used as a value (22.4); a lambda argument where it may not go (22.5; at a `MethodCall`, 13.2) | The identifier; for a lambda argument, the argument | | `never` |
| `call-of-parameter` | A `Call` whose name resolves to a value binding (22.5) | The name | | As `undefined-function` |
| `lambda-to-lambda` | A lambda argument in a call of a const-bound lambda (22.5) | The argument | | The parameter has type `never`. The call is checked. |
| `duplicate-name` | A name bound twice in one scope, pattern or parameter list (22.7) | The later name | | The later binding stands. |
| `shadows-namespace` | A binding named like a namespace (22.7) | The name | | No binding is made. |
| `duplicate-key` | A literal key written twice in one map literal (22.8) | The later key | | The later entry stands. |
| `no-field` | A read of a field that a record cannot have (9.2, 12.2) | The field's name or key | | `never` |
| `no-method` | No method of that name for the receiver's kind (13.3) | The method's name | | As `undefined-function` |
| `maybe-absent` | A plain read of an optional or unlisted field, or an index read of a record (9.2, 12.2) | The field's name, the key, or the index expression | | The field's type (9.2) |
| `maybe-null` | An operand that may be null where null is not accepted (1.3) | The operand; for a read, the receiver; for a pattern, the value bound; for an argument, each place of failure (15.1) | | nonnull of the operand's type |
| `kind` | An operand of the wrong kind (7, 8, 9, 12); a rendered expression whose type has an opaque type (7.3) | The operand; for two operands that do not go together, the whole `Binary` (8.3); for a read or a method call, the receiver; for a computed index of a record, the index expression (9.2); for a rendered expression, that expression | | `bool` for `<`, `<=`, `>`, `>=`, `in` and `not in`; else `never` |
| `never-equal` | An equality or a `match` literal that can never hold, neither side being `never` nor exactly `null`, and the literal not being a duplicate (8.2, 8.4, 11.2) | The whole `Binary`; in a `match`, the literal, its `-` included | | `bool`; the literal is ignored |
| `always-truthy` | A tested expression that can never be falsy (10.2) | The tested expression; for a predicate, the lambda's body; for an expected `condition`, the result expression | `constant-condition` | Unchanged |
| `always-falsy` | A tested expression that can never be truthy (10.2) | The same | `constant-condition` | Unchanged |
| `zero-divisor` | `/` with a `Number` node 0 as divisor (8.2) | The divisor | | `float` |
| `arity` | Too few or too many arguments, or a lambda with too many parameters (22.5; at a `MethodCall`, 13.5) | The whole call; or the lambda argument: the `Lambda`, or the `Ident` that names a const-bound lambda | | `never` for the call; the lambda is not checked |
| `arg-mismatch` | An argument that does not fit its parameter; a plain argument at an `Fn`; a lambda body that does not fit R with its `Var`s solved (13.5 step 3, 3a); a bound that does not hold (13.2, 13.5) | For a plain argument, each place of failure (15.1); else the lambda's body, or the whole call | | The signature's result type (13.5, step 6) |
| `cannot-infer` | A `Var` still unbound in the Q of a parameter that the lambda declares (13.5) | The lambda | | The lambda is not checked. |
| `check-limit` | A limit of 20 is passed | The use of a const-bound lambda; the node whose type is too deep (20.4); for a type over the size limit, the node of the table of 20.7: the expression, the call, or for a name bound by a pattern with a default, the value bound | | `never` |
| `duplicate-case` | A `match` literal equal to an earlier one (22.8) | The later literal | | The literal is ignored. |
| `unreachable-arm` | An arm after `_` (22.8) | The arm | | The arm is checked and joined. |
| `non-exhaustive` | A `match` without `_` that does not cover its subject (11.2) | The whole `Match` | | The Join of the arms |
| `result-mismatch` | The result does not fit the expected type (15) | Each place of failure (15.1) | | Unchanged |
| `unchecked` | A `dynamic` site, when the class `dynamic` is rejected | The site | `dynamic` | `never` for the node on which the site is recorded |
| `may-fail` | A site of class `index`, `divide`, `value` or `host`, when that class is rejected | The site | The site's class | Unchanged |

The message of a `check` diagnostic is not specified in Core: its code and its
span are. The catalog's templates for the `check` phase are follow-on F8. Where
a message prints a type, it prints it by 3.9.

### 16.2 Codes that are not in Core

The catalog of 29 check codes has two that Core cannot report:

| Code | Raised by | Package |
| --- | --- | --- |
| `untagged-union` | Two or more records in the type after `as` or `is` that do not form a tagged union | F5 |
| `never-conforms` | `x as T` where no value could pass | F4 |

These uses of Core codes also come only with a follow-on form, and are not in
Core: `undefined-name` for a type name after `as` or `is`; `check-limit` for a
type after `as` or `is`; `always-truthy` and `always-falsy` on an `x is T`;
`kind`, `maybe-null` and `no-method` at a projection step.

## 17. Check classes

A check class is a named group of findings that the checker makes on every
program, and that an environment either rejects or does not. The environment
carries the set of classes it rejects (`03-host.md`).

| Class | Its findings | Rejected | Not rejected |
| --- | --- | --- | --- |
| `constant-condition` | A tested expression that can go one way only (10.2) | Each is a diagnostic, `always-truthy` or `always-falsy` | Not reported, and not in the result |
| `dynamic` | The `dynamic` sites (18.3) | Each is a diagnostic, `unchecked` | Listed in the result's sites |
| `index`, `divide`, `value`, `host` | The sites of that class (18.3) | Each is a diagnostic, `may-fail` | Listed in the result's sites |

| Matter | Rule |
| --- | --- |
| The default set | `constant-condition` and nothing else. |
| Strict mode | The set `constant-condition` and `dynamic`. |
| A diagnostic of a rejected class | An error like any other. It carries the class's name (`03-host.md`). |
| What the set changes | Only which findings are diagnostics. A program that checks under two sets has the same type and the same facts under both, and the same sites less those of a rejected class. No diagnostic outside the classes changes. |
| `unchecked` and the type | The node on which a rejected `dynamic` site is recorded takes the type `never`, so later steps on it record nothing more. |
| Dropped sites | A site is dropped (18.3) before the rejected set is applied. A site in the body of a `try`, or in the check of an unused const-bound lambda, is never a diagnostic, whatever the environment rejects, and the node it is on keeps its type. |

## 18. The promise

### 18.1 Conformance

A value **conforms** to a type by this table.

| Type | A value conforms when |
| --- | --- |
| `any` | Always. |
| `never` | Never. |
| A set of alternatives | The value's kind is the kind of one alternative, and the value conforms to that alternative. |
| `null`, `bool`, `string` | It is of that kind. |
| `float` | It is a number. |
| `int` | It is a number with no fractional part and a magnitude of at most 2^53 − 1, however it was written. |
| A literal set | It is a string in the set. |
| `T[]` | It is a list, and every element conforms to T. |
| A record | It is a map. Every required field is present. Every listed field that is present conforms to its type. The value of every key the record does not list conforms to the rest. |
| Opaque *N* | The binding says the value carries the name N (19.3). |

A field is present when the map has the key, whatever its value. So `{"id": "a",
"note": null}` does not conform to `{id: string, note?: string}`. Nothing is
converted: the string `"42"` does not conform to `int`. A host number that
cannot enter the language conforms to no type. The order in which a test walks a
value, and its cost, are follow-on F4.

### 18.2 The kinds of failure

A run-time failure has one of nine kinds: `null`, `kind` and `field` (the
mistakes the checker looks for), `index`, `divide`, `value` and `host` (failures
on a value), `range` and `limit`. This is the one list of kinds. The field
`kind` of a `fails` line of `library.json` (`04-library.md` 1.3) names one of
them; a library entry fails only with `value`, `range`, `kind`, `field` or
`limit`.

### 18.3 Sites

| Class | Recorded on | Span |
| --- | --- | --- |
| `dynamic` | Each use of an `any` value that needs a kind, and each rendered expression whose type has `any` (below) | The `any` operand, receiver or expression; the rendered expression; for a map pattern's unlisted field, the field's key (12.2) |
| `index` | A plain index read of a list or a string (9.2), or of a receiver of type `any` (9.3); a list pattern element without a default, `_` included (12.2) | The `Index` node; the element's name or `_` |
| `divide` | `/`, `%` and `**`, by 8.2 | The `Binary` node |
| `value` | A call of a library entry marked `!`; a method call on a receiver of type `any` when some library method of that name is marked `!` (13.3) | The call |
| `host` | A call of a host function or method marked `!`; a method call on a receiver of type `any` when a method of that name of some declared opaque type is marked `!` (13.3) | The call |

A run failure is **at a site** of class `index`, `divide`, `value` or `host`
only when the site is on the failing node itself. The wider reach below is for
`dynamic` sites only. So in `geo.distance(string(rate("x")), "b")`, with `rate`
marked `!` and `geo.distance` not, an error from `geo.distance` is not at the
`host` site of `rate("x")`: its class is `environment` (18.5).

A run failure is **at a `dynamic` site** when the site is on the failing node,
or on a node that is the failing node's operand, receiver, index, computed key
or argument. A failure of a library entry's test of a callback's result, or of a
key or a value a callback gave, is at a `dynamic` site when the site is on the
body of a lambda that is an argument of the failing call, or that such an
argument names (a const-bound lambda). A failure of a pattern is at a `dynamic`
site when the site is on the value bound (12.3), or on the key of the field or
the name of the element that failed. A rendering failure (an opaque value met
while rendering) is at a `dynamic` site when the site is on the rendered
expression (7.3). The class of a failure (18.5) uses this definition.

| An `any` value | Is a `dynamic` site when it is |
| --- | --- |
| A site | The receiver of a read or a method call; an operand of `-`, arithmetic, order or `+`; the right operand of `in`, or the left when the right is a string or a record; a spread's operand; a computed key; a destructured value; a map pattern's field unlisted in a record whose rest is `any`; a rendered expression whose type has `any` at any depth (7.3); an index expression; an argument, a result or a part of one whose fit used any (15.1); a bound that held by any (13.5). |
| Not a site | Passed on as a result, an item, an entry's value, an argument to an `any` parameter that is not rendered, or through `??`; tested in a tested position, so under `!`, `&&` and `\|\|`; compared with `==` or `!=`; the left operand of `in` with a list on the right; read with `?.` at an unlisted key of an open record, by a field step or by a computed key. |

Sites inside the body of a `try` are dropped. Sites of the check of an unused
const-bound lambda are dropped (13.9). A dropped site is never a diagnostic
(17). The result lists each remaining site once, in the order of 6.1.

### 18.4 The precondition and the promise

| Part | Statement |
| --- | --- |
| Precondition | Every variable's value conforms to its declared type; every host function and method returns a value that conforms to its result type; and a host function or method whose signature has no `!` returns no error. The mark `!` is a promise, as a declared type is. |
| Promise 1 | If the checker reports no diagnostic, then in every run a failure of kind `null`, `kind` or `field` occurs only at a `dynamic` site. |
| Promise 2 | A failure of kind `index`, `divide`, `value` or `host` occurs only at a site of that class. |
| Promise 3 | `range` and `limit` failures, and a cancellation, can occur anywhere. A crash of a host function (`function-panic`, class `host`) can occur at any call of a host function or method, marked `!` or not. A failure inside the implementation has no class. |
| Promise 4 | A run that succeeds gives a value that conforms to the inferred type. |

Truthiness adds no kind of failure and no class of site. A tested position
accepts every value.

### 18.5 The class of each run code

The class of a run failure (`03-host.md`) depends on whether the program was
checked. A program compiled with no check has no sites.

| Run code | Class, checked | Class, not checked |
| --- | --- | --- |
| `steps-exceeded`, `allocation-exceeded`, `depth-exceeded`, `deadline-exceeded`, `cancelled` | `limit` | `limit` |
| `missing-variable` | `environment` | `environment` |
| `function-panic` | `host` | `host` |
| `internal-error` | none | none |
| `function-error` | `host` at a `host` site on the failing call itself (18.3); `environment` anywhere else: a function or method not marked `!` returned an error | `host` |
| `number-range` | `range` from arithmetic or a library entry; `environment` for a host number that cannot enter the language | The same |
| `not-conforming` | `value` at a `value` site; `environment` elsewhere. No Core form raises it (follow-on F4). | `value` |
| `invalid-argument`, `failed` | `value` | `value` |
| `null-operand`, `wrong-kind`, `missing-field` | `dynamic` at a `dynamic` site; `environment` anywhere else | `dynamic` |
| `index-range` | `index` | `index` |
| `divide-by-zero` | `divide` | `divide` |
| `no-match` | `environment` | `dynamic` |

In a checked program the class `environment` means that the precondition failed:
a value broke its declared type, a declared variable was not supplied, or a host
function or method not marked `!` returned an error. A program never sees a
class: the error value has no class field (14.2), and `catch` decides by code. A
checked and an unchecked run of one program on the same values give the same
value, the same usage, and the same code and span on failure.

## 19. Declaring an environment in type text

`03-host.md` owns how an environment is built, its clashes, its version, its
limits and its rejected classes. This section says how its type text is read.

### 19.1 Variables

A variable has a name and a type, written as a `Type` (3.1). A case file's `env`
field is one record type whose fields are the variables: `{order: {total:
float}, tier: "gold" | "basic"}`. That text must be a `Record` by the grammar of
3.1, with no optional field and no rest: a variable that may be absent is
declared `T | null` and supplied as null. Parentheses do not make a record, and
a key written twice is `name-clash`. Text that the grammar accepts as a `Type`
but that is not a `Record`, such as `"int"`, is `bad-type` in every
implementation (19.5).

### 19.2 Host functions

A host function has a name, with at most one namespace prefix (`geo.distance`),
and a signature written as a `Sig`.

| Rule | Text |
| --- | --- |
| No `Fn` | A host signature with an `Fn` parameter is `bad-signature`. |
| No `unknown` | So `unknown` cannot appear in a host signature. |
| `Var`s and bounds | Allowed, and solved by 13.5. |
| `!` | The function can fail on a value. A call records a `host` site. |

### 19.3 Opaque types and their methods

| Matter | Rule |
| --- | --- |
| Declaration | An environment may declare opaque types by name. The name is a `Name` of 3.1, and is the whole type: no fields, no structure. It fits only itself and `any`. |
| Methods | An environment may declare methods on an opaque type, each as a `Method` text whose `Recv` is that type's name: `Time.format: (layout: string) => string`. |
| Use | A value of an opaque type can be bound, passed, put in a list or a map, compared with `==`, tested, and used through its declared methods. It is always truthy. It cannot be rendered: a rendered expression whose type has an opaque type at any depth, a template part or the argument of `string`, is `kind` (7.3, 13.10). |
| The binding | It tells, for each host value, whether it is opaque and under which declared name (`03-host.md`). |

### 19.4 Library signatures

A library entry's `receiver` and `signature` are read by 3.1 as a `Recv` and a
`Sig`. They may use `Var`s, `Fn` parameters, `unknown` as an `Fn` result, bounds
and `!`.

### 19.5 Errors in declared type text

Each of these is an error of phase `declare` when the environment is built.
**The code says where the text stands:** every error in a host function's
signature or in a method's text is `bad-signature`, and every error in a
variable's type or in the expected result's type is `bad-type`, whatever is
wrong. What is wrong is the message's business.

**Every implementation** parses the type text of its environment by the grammar
of 3.1 and the token rules of 3.2, whether or not it has a type checker. It
needs the names of the variables and functions, and the count of parameters of
each function (22.2). These errors need no type checker, and every
implementation reports them:

| Error | Code |
| --- | --- |
| A variable's type, or the expected result's type, that the grammar of 3.1 and the token rules of 3.2 do not accept as a `Type`. This covers the words of 3.8. | `bad-type` |
| An environment's text (19.1) that the grammar accepts as a `Type` but that is not a `Record`, such as `int` or `({a: int})` | `bad-type` |
| An environment's record (19.1) with an optional field or a rest, such as `{a?: int}` or `{a: int, ...}` | `bad-type` |
| An environment's record (19.1) that writes one key twice, such as `{a: int, a: int}`: two variables of one name | `name-clash` |
| A host function's signature or a method's text that the grammar of 3.1 and the token rules of 3.2 do not accept as a `Sig` or a `Method`, wherever in it the error lies | `bad-signature` |
| A `Sig` with an optional parameter before a required one, or with a rest parameter that is not the last (3.6) | `bad-signature` |
| A host signature with an `Fn` parameter (19.2) | `bad-signature` |
| Type text whose depth (20.5) is over 64. A `[]` suffix counts, and a bracketed form counts once over its deepest content | `bad-type` in a variable's or the expected result's type, `bad-signature` in a signature or a method's text |
| A method whose `Recv` is not a declared opaque type (19.3). It needs names only. | `bad-signature` |

**An implementation with a type checker** also reports these. They are rules
beyond the grammar:

| Error | In a variable's or the expected result's type | In a signature or a method's text |
| --- | --- | --- |
| A key written twice in one `Record` (3.3), deeper in a type than the environment's record | `bad-type` | `bad-signature` |
| Two or more record alternatives in one `Type` (3.4) | `bad-type` | `bad-signature` |
| A `Name` that is not a declared opaque type (3.5) | `bad-type` | `bad-signature` |
| A rest parameter whose type is not a list (3.6) | | `bad-signature` |
| A type deeper than 64 (2.5) | `bad-type` | `bad-signature` |
| A type whose size (20.7) is over 100,000 | `bad-type` | `bad-signature` |

A type deeper than 64 by 2.5 cannot occur in Core: the depth of type text (20.5)
is never less than the depth of the type it writes, and type text deeper than 64
is already an error of the first table. The row is for follow-on F4, whose named
types make a short text stand for a deep type.

## 20. Limits of the checker

Every recursion of the checker is bounded by a number here. Passing a number
gives the same result in every implementation.

### 20.1 Source nesting

A program may open at most 100 levels of nesting, counted as `01-language.md`
counts them. The parser enforces it, so the checker never meets deeper text.

### 20.2 Expanded nesting

A use of a const-bound lambda is checked by checking its body (13.9). The
counts:

| Term | Definition |
| --- | --- |
| n(X) | The nesting depth of a node X: the number of levels open at its first token (`01-language.md` 13.2 item 5). |
| d(L) | For a lambda L: the greatest number of levels open at once inside the text of L, its parameter list included, less n(L). |
| E(X), the **expanded nesting** of X | For a node outside every body checked for a use: n(X). For a node X of the body of L checked for a use U: E(U) + 1 + n(X) − n(L). |

A use U of L is `check-limit` on U when E(U) + 1 + d(L) is more than 100. The
use then has type `never`, and the body of L is not checked for it.

Take `const f0 = x => x; const f1 = x => f0(x);` and so on to `const f100 = x =>
f99(x); f100(1)`. d(f0) is 0, and every other d is 1: the `(` of the call opens
one level. `f100(1)` has E 0, the use of f99 in the body of f100 has E 1, and
the use of f(k) has E 100 − k. The use of f1, in the line of f2, has E 99, and
99 + 1 + 1 is more than 100: it is `check-limit` on `f1(x)` in the line of f2,
and nothing else is reported, because the body of f1 is then not checked. The
same program ending with `const f99 = x => f98(x); f99(1)` checks.

### 20.3 Lambda body checks

A program may check at most 1,000 lambda bodies by 13.9. Each use counts 1. Uses
are counted in the order their body checks begin in the walk in source order:
the `const` lines in order, the children of a node left to right, a call's plain
arguments before its body, and a lambda argument at step 3 of 13.5. A use met
while a body is checked is counted at that point, each time that body is
checked, also when an implementation reuses a kept check. A check that the seed
rule discards (13.5 step 3a) keeps its counts. Every use past the 1,000th is
`check-limit` on the use, and has type `never`. The check of an unused
const-bound lambda is not a use, but the uses met in it count. So 1,001 calls of
one const-bound lambda written one after another give `check-limit` on the last
call, and on nothing else.

### 20.4 Type depth

| Type | Limit 64 | Passing it |
| --- | --- | --- |
| A declared type | 2.5 | `bad-type` or `bad-signature`, phase `declare` (19.5). In Core the limit on type text (20.5) always meets it first. |
| A type the checker builds | Tested at a `List`, a `Map`, and the result of a call | `check-limit` on that node, which has type `never` |

No other rule makes a type deeper than its inputs: Join, Merge, Truthy, Falsy
and facts never do. The type of a `catch` name has depth 2.

### 20.5 Type text

The **depth of type text** is defined on the text as 3.1 parses it:

| Text | Depth |
| --- | --- |
| A `[]` suffix of an `Alt` | 1 more than the depth of what it follows: the `Prim` and the suffixes before it. So `P` with k suffixes has the depth of `P` plus k. |
| A bracketed form: `( ... )` (a parenthesized `Type`, a `Sig`'s or an `Fn`'s parameter list), `{ ... }` (a `Record`), `< ... >` (of `map`) | 1 more than the deepest of its contents |
| A sequence of parts: the alternatives of a `Type`, the parameters of a list, the fields of a `Record`, a `Sig` with its result and bounds, a `Method` with its `Recv` | The deepest of its parts |
| Any other text: a word, a `Name`, a `Var`, a `String`, punctuation | 0 |

Type text deeper than 64 is `bad-type`, or `bad-signature` in a `Sig` or a
`Method` (19.5). Every implementation applies this limit, with or without a type
checker. It is a limit on text and is not the depth of a type (2.5), which an
implementation with a type checker tests after names are replaced.

| Text | Depth | Result |
| --- | --- | --- |
| `int` followed by 64 `[]` | 64 | Accepted |
| `int` followed by 65 `[]` | 65 | `bad-type` |
| 64 `(`, then `int`, then 64 `)` | 64 | Accepted |
| 65 `(`, then `int`, then 65 `)` | 65 | `bad-type` |
| `{a: ` then `int` followed by 63 `[]`, then `}` | 64 | Accepted |
| `{a: ` then `int` followed by 64 `[]`, then `}` | 65 | `bad-type` |
| `(int[])[]` | 3 | Accepted |
| `(x: (item: T) => U[]) => T`, a `Sig` | 2: the `Fn` has depth 1 (its `( )` over `T`, and `U[]`), and the outer `( )` adds 1 | Accepted |

### 20.6 Loops, not recursion

What opens no level is checked by a loop: the operands of a left-associative
chain of operators, the steps of a chain, the `else` branch of `? :`, an `else
if` chain, the arms of a `match` and the `const` lines of a block. Along an
`else` chain the checker carries the false facts of the tests so far and the
Join of the branches so far. Recursion over types (Join, Fits, Merge, bind,
printing, the type text parser and the conformance walk) goes no deeper than the
deeper of its types, so 64 bounds it.

### 20.7 Type size

A type is held with shared parts, so a short program can make a type whose
expansion doubles at each line. Its size is bounded.

| Type | Size |
| --- | --- |
| `any`, `never`, `null`, `bool`, `int`, `float`, `string`, an opaque type's name, a type variable | 1 |
| A literal set | 1 for each string in it |
| `T[]` | 1 plus the size of T |
| A record | 1, plus the size of each field's type, plus the size of its rest. A rest of `never` counts 0. |
| A type with several alternatives | The sum of the sizes of its alternatives |

The size is that of the type written out by structure, with no sharing. A
field's mark (required or optional) does not count. **The limit is 100,000.**
The specification fixes it, and a host cannot change it.

**Where it is tested.** A type is tested when it becomes the type of one of
these nodes or names. A type over the limit is `check-limit` on the node in the
last column, and the node has type `never` afterwards, as for the other limits
of 20.

| Construct | The type tested | `check-limit` on |
| --- | --- | --- |
| A list literal, its spreads included (7.4, 7.7) | The list's type | The `List` |
| A map literal, its spreads and computed keys included (7.5, 7.7) | The record it builds | The `Map` |
| `c ? a : b`, and an `if` chain (11.1) | The Join of the branches | The `Cond`. An `else if` is a `Cond` of its own (`01-language.md` 4.2), whose type is the Join of the branches from it on, so the innermost `Cond` whose type passes the limit is the one reported |
| `match` (11.2) | The Join of the arms | The `Match` |
| `try` (14) | The Join of the body and the handler | The `Try` |
| `a ?? b`, `a && b`, `a \|\| b` (8.2) | The node's type | The `Binary` |
| A call of a library entry or a host function or method (13.5) | Each solution of a type variable, as step 2, step 3 or the seed rule makes it; then the result type | The `Call` or the `MethodCall`. The test of a solution is made when the solution is made; when it fails, the call is not checked further and has type `never` |
| A name bound by a pattern with a default (12.4) | Join(nonnull(T), D) | The value bound (12.3): the `const` value expression, or the parameter's pattern |

No other construct makes a type larger than its parts: a read gives a part of
its receiver's type; a narrowed type (10.4) and the `...rest` of a pattern
(12.2) are no larger than the type before; the other operators give `bool`, a
number type or `string`; a `Block` and a call of a const-bound lambda have the
type of an expression that was tested where it stands; a name bound by `const`
to a value, and a parameter bound to an argument, take a type that was tested.

| Matter | Rule |
| --- | --- |
| Innermost only | An expression built from one that failed sees `never`, so only the innermost expression over the limit is reported. |
| Constant work | An implementation keeps the size with each type as it builds it, saturating above the limit, so each test is constant work and nothing is expanded to find a size. No operation of the checker may take work that grows with the expansion of a type past the limit: an operation whose result would pass the limit stops, and the test above reports it. |
| Declarations | A declared type over the limit is `bad-type` or `bad-signature`, by where its text stands (19.5). In Core's type text, which has no sharing, the size of a type is at most the count of its tokens. |
| Without a type checker | Not tested: the static rules (22) have no types. |

The doubling chain `const a0 = {l: 1, r: 1};` and, for each k from 1, `const ak
= {l: a(k−1), r: a(k−1)};` gives sizes s0 = 3 and s(k) = 2 × s(k−1) + 1, so s(k)
= 2^(k+2) − 1:

| Line | k | s(k) | |
| --- | --- | --- | --- |
| 15 | 14 | 65,535 | Under the limit |
| 16 | 15 | 131,071 | Over: `check-limit` on the map literal of line 16 |

Worked examples 130 to 134 (21.15) show it.

## 21. Worked examples

### 21.1 The environment

Every example is checked against this environment, with the default rejected set
`["constant-condition"]` and no expected result, unless it says otherwise.

```
env:
  {input:   {role: "admin" | "member" | "owner", note?: string,
             tags: string[], priority: int | null,
             options: {retries?: int, backoff?: string} | null},
   gate:    {failed: int, score: float,
             items: {name: string, status: "fail" | "pass",
                     owner: string | null, ms: float}[]},
   report:  {verdict: "APPROVE" | "REJECT", body: string} | null,
   scores:  map<float>,
   event:   {action: string,
             pull_request?: {number: int, head: {sha: string}}, ...},
   blob:    any,
   started: Time,
   ids:     (int | null)[]}
opaque:    Time
methods:   Time.format: (layout: string) => string
functions: rate: (code: string) => float !
           geo.distance: (a: string, b: string) => float
```

Each row gives the program, then its type and sites, or its diagnostics. A
program with diagnostics has no type in the result. Spans are on line 1.

### 21.2 Reads, chains and null

| # | Program | Result |
| --- | --- | --- |
| 1 | `gate.failed > 0` | `bool` |
| 2 | `input.role in ["admin", "owner"]` | `bool` |
| 3 | `report.verdict` | `maybe-null` 1:1-1:7 |
| 4 | `report?.verdict ?? "REJECT"` | `"APPROVE" \| "REJECT"` |
| 5 | `report?.verdict.toLowerCase()` | `string \| null`: one `?.` covers the chain. |
| 6 | `input.note.toUpperCase()` | `maybe-absent` 1:7-1:11 |
| 7 | `input?.note.toUpperCase()` | `maybe-null` 1:1-1:12: an absent `note` is null for that step only. |
| 8 | `input?.note?.toUpperCase() ?? ""` | `string` |
| 9 | `event?.pull_request?.head.sha` | `string \| null` |
| 10 | `gate.items[0].name` | `string`; site `index` 1:1-1:14 |
| 11 | `gate.items?.[0]?.name` | `string \| null`; no site |
| 12 | `gate.items[len(gate.items) / 2]` | `kind` 1:12-1:31: an index must fit `int`. |
| 13 | `scores[input.role]` | `maybe-absent` 1:8-1:18 |
| 14 | `blob.x.y + 1` | `any`; sites `dynamic` 1:1-1:5, 1:1-1:7, 1:1-1:9 |
| 15 | `blob.x`, with reject `["constant-condition", "dynamic"]` | `unchecked` 1:1-1:5, class `dynamic` |
| 16 | `gate.items[0].name`, with reject `["constant-condition", "index"]` | `may-fail` 1:1-1:14, class `index` |
| 17 | `started.format("iso")` | `string` |
| 18 | `inptu.role` | `undefined-name` 1:1-1:6 |
| 19 | `len` | `not-a-value` 1:1-1:4 |

### 21.3 Operators and numbers

| # | Program | Result |
| --- | --- | --- |
| 20 | `gate.score / len(gate.items)` | `float`; site `divide` 1:1-1:29 |
| 21 | `gate.failed / 0` | `zero-divisor` 1:15-1:16 |
| 22 | `gate.score % 2` | `float`; no site: the divisor is a non-zero `Number` node. |
| 23 | `2 ** 10` | `int` |
| 24 | `2 ** gate.failed` | `float`; no site: the left side is a non-zero `Number` node. |
| 25 | `2 ** gate.score` | `kind` 1:6-1:16 |
| 26 | `gate.failed + " failures"` | `kind` 1:1-1:26: the two sides do not go together. |
| 27 | `-input.role` | `kind` 1:2-1:12 |
| 28 | `input.priority + 1` | `maybe-null` 1:1-1:15 |
| 29 | `gate.failed == "0"` | `never-equal` 1:1-1:19 |
| 30 | `` `at ${started}` `` | `kind` 1:7-1:14: an opaque value cannot be rendered. |
| 31 | `{a: 1, a: 2}` | `duplicate-key` 1:8-1:9 |
| 32 | `{...gate, failed: "none"}` | `{failed: "none", items: {ms: float, name: string, owner: string \| null, status: "fail" \| "pass"}[], score: float}` |

### 21.4 Truthiness and constant conditions

| # | Program | Result |
| --- | --- | --- |
| 33 | `report != null && report.verdict == "APPROVE"` | `bool` |
| 34 | `report && report.body` | `string \| null` |
| 35 | `report && report.verdict == "APPROVE"` | `bool \| null` |
| 36 | The same, with expected result `bool` | `result-mismatch` 1:1-1:38 |
| 37 | The same, with expected result `condition` | `bool \| null` |
| 38 | `input?.note \|\| "none"` | `string` |
| 39 | `input.priority \|\| 3` | `int` |
| 40 | `gate.failed \|\| "none"` | `int \| "none"` |
| 41 | `input.options && input.options?.retries` | `int \| {} \| null` |
| 42 | `gate.items && gate.items[0].name` | `string \| never[]`; site `index` 1:15-1:28 |
| 43 | `started ? started.format("iso") : ""` | `always-truthy` 1:1-1:8 |
| 44 | `input.role \|\| "member"` | `always-truthy` 1:1-1:11 |
| 45 | `null \|\| gate.failed` | `always-falsy` 1:1-1:5 |
| 46 | `!started` | `always-truthy` 1:2-1:9 |
| 47 | `started \|\| started`, with reject `[]` | `Time` |
| 48 | `started`, with expected result `condition` | `always-truthy` 1:1-1:8 |
| 49 | `gate.items`, with expected result `condition` | `{ms: float, name: string, owner: string \| null, status: "fail" \| "pass"}[]` |
| 50 | `blob \|\| "x"` | `any`; no site |
| 51 | `if (report) { report.verdict } else { "none" }` | `"APPROVE" \| "REJECT" \| "none"` |
| 52 | `if (!report) { "none" } else if (report.verdict == "APPROVE") { "go" } else { "stop" }` | `"go" \| "none" \| "stop"` |
| 53 | `if (gate.failed > 0) { const n = gate.failed; n * 2 } else { 0 }` | `int` |
| 54 | `if (started) { 1 } else { 2 }` | `always-truthy` 1:5-1:12 |

### 21.5 `match`

| # | Program | Result |
| --- | --- | --- |
| 55 | `match input.role { "admin" \| "owner" => "write", "member" => "read" }` | `"read" \| "write"` |
| 56 | `match input.role { "admin" => 1, "guest" => 0 }` | `non-exhaustive` 1:1-1:48; `never-equal` 1:34-1:41 |
| 57 | `match input.role { "admin" => 1, _ => 0, "owner" => 2 }` | `unreachable-arm` 1:42-1:54 |
| 58 | `match input.role { "admin" => 1, "admin" => 2, _ => 0 }` | `duplicate-case` 1:34-1:41 |
| 59 | `match input.priority { null => "normal", 1 \| 2 => "high", _ => "low" }` | `"high" \| "low" \| "normal"`. `_` is required: `int` is not finite. |

### 21.6 Bindings and patterns

| # | Program | Result |
| --- | --- | --- |
| 60 | `{ const {retries = 3, backoff = "1s"} = input.options ?? {}; {retries, backoff} }` | `{backoff: string, retries: int}` |
| 61 | `{ const {role, ...rest} = input; rest }` | `{note?: string, options: {backoff?: string, retries?: int} \| null, priority: int \| null, tags: string[]}` |
| 62 | `{ const {action, ...rest} = event; rest }` | `{pull_request?: {head: {sha: string}, number: int}, ...}` |
| 63 | `{ const {note, ...rest} = input; rest }` | `maybe-absent` 1:10-1:14 |
| 64 | `{ const [first, second = ""] = input.tags; first }` | `string`; site `index` 1:10-1:15 |
| 65 | `const a = 1; const a = 2; a` | `duplicate-name` 1:20-1:21 |
| 66 | `const geo = 1; geo` | `shadows-namespace` 1:7-1:10; `not-a-value` 1:16-1:19 |
| 67 | `const t = gate.failed; t(1)` | `call-of-parameter` 1:24-1:25 |

### 21.7 Calls and lambdas

| # | Program | Result |
| --- | --- | --- |
| 68 | `gate.items.filter(t => t.status == "fail").map(t => t.name)` | `string[]` |
| 69 | `gate.items.map(t => { const slow = t.ms > 500; {name: t.name, slow} })` | `{name: string, slow: bool}[]` |
| 70 | ``scores.entries().map(({key, value}) => `${key}=${value}`)`` | `string[]` |
| 71 | `gate.items.reduce((m, t) => {...m, [t.name]: t.status}, {})` | `map<"fail" \| "pass">`: the seed rule widens `{}` once. |
| 72 | `gate.items.reduce((a, t) => a + t.ms, 0)` | `float`: the seed rule widens `int`. |
| 73 | `gate.items.filter(t => t.owner).map(t => t.name)` | `string[]` |
| 74 | `gate.items.filter(t => t.status)` | `always-truthy` 1:24-1:32 |
| 75 | `gate.items.map(t => t.owner).filter(o => o != null)` | `string[]` |
| 76 | `ids.filter(x => x != null).sum()` | `int` |
| 77 | `ids.sum()` | `arg-mismatch` 1:1-1:10: `int \| null` is not a `number`. |
| 78 | `gate.items.map()` | `arity` 1:1-1:17 |
| 79 | `gate.items.map((t, i, z) => t)` | `arity` 1:16-1:30 |
| 80 | `lenn(gate.items)` | `undefined-function` 1:1-1:5 |
| 81 | `len(5)` | `arg-mismatch` 1:5-1:6 |
| 82 | `rate(input.role)` | `float`; site `host` 1:1-1:17 |
| 83 | `rate(report?.body)` | `maybe-null` 1:6-1:18 |
| 84 | `rate(x => x)` | `not-a-value` 1:6-1:12 |
| 85 | `const pct = (n, digits = 0) => round(n * 10 ** digits); pct(gate.score) + pct(gate.failed, 2)` | `int`; no site |
| 86 | `const name = t => t.nmae; gate.items.map(name)` | `no-field` 1:21-1:25 |
| 87 | `const twice = (f, x) => f(f(x)); twice(n => n + 1, 2)` | `call-of-parameter` 1:25-1:26 and 1:27-1:28; `lambda-to-lambda` 1:40-1:50 |
| 88 | `const inc = n => n + 1; [1, 2].map(inc)` | `int[]` |

### 21.8 `try`

| # | Program | Result |
| --- | --- | --- |
| 89 | `try { int(report?.body ?? "") } catch { 0 }` | `int`; no site: the body's `value` site is dropped. |
| 90 | `(try { int(blob) } catch { 0 }) > 10` | `bool`; no site |
| 91 | `try { 1 } catch (err) { fail(err.message, err.data) }` | `int`; site `value` 1:25-1:52 in the handler |
| 92 | `try { int(blob) } catch (err) { err.code == "not-conformin" ? 0 : fail(err.message) }` | `never-equal` 1:33-1:60 |
| 93 | `try { blob.x } catch (err) { err.stack }` | `no-field` 1:34-1:39 |
| 94 | `try { 1 } catch (err) { match err.code { "failed" => 1 } }` | `non-exhaustive` 1:25-1:57 |
| 95 | `try { blob.x } catch (err) { err.path }` | `any`: the Join with `any` is `any`. No site: the body's `dynamic` site is dropped. |

### 21.9 The expected result through its form

| # | Expected | Program | Result |
| --- | --- | --- | --- |
| 96 | `int` | `gate.failed > 0 ? 1 : "a"` | `result-mismatch` 1:23-1:26 |
| 97 | `{listen: string, max_body?: int}` | `{listen: ":80", max_bod: 1}` | `result-mismatch` 1:17-1:24, on the key |
| 98 | `{listen: string, max_body?: int}` | `{max_body: "8"}` | `result-mismatch` 1:1-1:16 (no `listen`) and 1:12-1:15 |
| 99 | `int[]` | `[1, 2.5, "x"]` | `result-mismatch` 1:5-1:8 and 1:10-1:13 |
| 100 | `bool` | `gate.items` | `result-mismatch` 1:1-1:11 |

### 21.10 Declarations

| # | Declaration | Result |
| --- | --- | --- |
| 101 | `env: {x: {a: int} \| {b: string}}` | `bad-type`, phase `declare`: two records in one type. Reported by an implementation with a type checker (19.5). |
| 102 | `env: {x: number}` | `bad-type`, phase `declare` |
| 103 | `functions: apply: (f: (x: int) => int) => int` | `bad-signature`, phase `declare`: a host function takes no lambda. |
| 104 | `env: {x: Order}`, with no opaque type `Order` | `bad-type`, phase `declare`, by an implementation with a type checker (19.5). Follow-on F4 lets `Order` be a named type. |
| 105 | `env: {x: {a: int, ...}}` | Accepted. `x.b` is `any` with a `dynamic` site 1:1-1:2, and `x?.b` is `any` with no site. |

### 21.11 Library signatures

| # | Program | Result |
| --- | --- | --- |
| 106 | `[].sum()` | `int`: `sum` has the result `T \| int` (`04-library.md`), and T is `never`. |
| 107 | `[1, 2].join(",")` | `arg-mismatch` 1:2-1:3 and 1:5-1:6: the receiver is argument zero and is fitted through its form. |

### 21.12 Bodies, rendering and the seed

| # | Program | Result |
| --- | --- | --- |
| 108 | `[1].flatMap(x => x)` | `arg-mismatch` 1:18-1:19: the body `x` is an `int`, and R is `U[]` with U unbound, so `never[]` (13.5 step 3). |
| 109 | `[].map(x => x + 1)` | `int[]`: U gains `int`; x has type `never` (13.4). |
| 110 | `string(started)` | `kind` 1:8-1:15: the argument of `string` is rendered, and `started` is opaque (13.10). |
| 111 | `string(blob)` | `string`; site `dynamic` 1:8-1:12 |
| 112 | `` `${ {a: blob} }` `` | `string`; site `dynamic` 1:5-1:14: the part's type has `any` at depth 1. |
| 113 | `gate.items.reduce((a, t) => [a], 0)` | `arg-mismatch` 1:29-1:32: the seed makes A `int \| int[]`, and the second check's `(int \| int[])[]` does not fit it (13.5 step 3a). |
| 114 | `gate.items.reduce((a, t) => gate.failed > 0 ? blob : a + t.ms, 0)` | `any`; site `dynamic` 1:54-1:55, on `a`: B is `any`, which fits `int` only by using any, so the seed fires and the second check gives `a` the type `any`. |
| 115 | `"a" in []` | `bool`, with no diagnostic: the element type is `never` (8.4). |

### 21.13 Sites where a kind is not known, and readings made exact

| # | Program | Result |
| --- | --- | --- |
| 116 | `match null { 2 => 0 }` | `non-exhaustive` 1:1-1:22 only: the subject is exactly `null`, so no `never-equal` (11.2). |
| 117 | `match null { 0 \| null \| "" => 0 }` | `int` |
| 118 | `const f = () => gate.nope; f` | `no-field` 1:22-1:26; `not-a-value` 1:28-1:29: a read as a value is not a use, so the body is checked once (13.9). |
| 119 | `const {b = 0[""]} = ""; 0` | `kind` 1:12-1:13; `kind` 1:21-1:23: the default is checked after `kind` on the pattern (12.2). |
| 120 | `const {k = 0[""]} = null; 0` | `kind` 1:12-1:13; `maybe-null` 1:21-1:25 |
| 121 | `match ({}) { 5 \| 5 => 0 }` | `non-exhaustive` 1:1-1:26; `never-equal` 1:14-1:15; `duplicate-case` 1:18-1:19, and no `never-equal` there (11.2). |
| 122 | `[].reduce(() => {} - 0, () => 0)` | `kind` 1:17-1:19; `not-a-value` 1:25-1:32: the first lambda declares no parameter, so `A` unbound is no error (13.5 step 3). |
| 123 | `blob[2]` | `any`; sites `dynamic` 1:1-1:5 and `index` 1:1-1:8 (9.3) |
| 124 | `blob?.[2]` | `any`; site `dynamic` 1:1-1:5 |
| 125 | `blob.repeat(2)` | `any`; sites `dynamic` 1:1-1:5 and `value` 1:1-1:15: the library method `repeat` is marked `!` (13.3). |
| 126 | `blob.format("iso")` | `any`; site `dynamic` 1:1-1:5: neither a library method nor the method of `Time` named `format` is marked `!`. |
| 127 | `[0].flatMap(() => blob)` | `any[]`; site `dynamic` 1:19-1:23. Run with `blob` = `2`: `wrong-kind` at the call, class `dynamic`, by the site on the body (18.3). |

### 21.14 A host function that returns an error

The host function returns an error in each run. A case states it as a stub with
`error` (`03-host.md`).

| # | Program | Check | Run, the stub returning an error |
| --- | --- | --- | --- |
| 128 | `rate(input.role)` | `float`; site `host` 1:1-1:17: `rate` is marked `!` | `function-error` 1:1-1:17, class `host` |
| 129 | `geo.distance("a", "b")` | `float`; no site: `geo.distance` is not marked `!` | `function-error` 1:1-1:23, class `environment`: the precondition failed (18.4). Without a check, class `host`. |
| 135 | `geo.distance(string(rate("x")), "b")`, `geo.distance` returning an error and `rate` a value | `float`; site `host` 1:21-1:30, on `rate("x")` only | `function-error` 1:1-1:37, class `environment`: the `host` site is on an argument, not on the failing call (18.3). |

### 21.15 Type size

In these programs each `const` line stands on a line of its own. **A(n)** is the
n + 1 lines `const a0 = {l: 1, r: 1};` to `const an = {l: a(n−1), r: a(n−1)};`,
which are lines 1 to n + 1, with s(k) = 2^(k+2) − 1 (20.7). **B(n)** is the same
with the keys `x` and `y` and the names `b0` to `bn`, standing after A(14) on
lines 16 to n + 16. Spans are written `line:column`.

| # | Program | Result |
| --- | --- | --- |
| 130 | A(14), then `0` on line 16 | `int`. `a14` has size 65,535. |
| 131 | A(15), then `0` on line 17 | `check-limit` 16:13-16:29, on `{l: a14, r: a14}`: size 131,071. |
| 132 | A(16), then `a16.l` on line 18 | `check-limit` 16:13-16:29 only: `a15` has type `never`, so `a16` is `{l: never, r: never}`, of size 3. |
| 133 | A(14), B(14), then `gate.failed > 0 ? a14 : b14` on line 31 | `check-limit` 31:1-31:28, on the `Cond`. The Join is `{l?: …, r?: …, x?: …, y?: …}` with four fields of size s(13) = 32,767: 1 + 4 × 32,767 = 131,069. |
| 134 | A(14), B(14), then `const m = gate.failed > 0 ? a14 : null; m != null ? [m, b14] : []` on line 31 | `check-limit` 31:53-31:61, on the list `[m, b14]`, reached through `m` narrowed to the type of `a14`: its element type has size 131,069. `m` itself has size 65,536 and is not reported. |

### 21.16 A map literal with spreads against a record

The environment is `{base: {db: {host: string, port: int}, hosts: string[],
listen: string}, bad: {listen: int}, maybe: {listen?: int}, part: {db: {host:
string, port: int}, hosts: string[]}, nums: map<int>, extra: {listen: string,
...}, blob: any}`, and the expected result is `{db: {host: string, port: int},
hosts: string[], listen: string, max_body?: int}` (15.1).

| # | Program | Result |
| --- | --- | --- |
| 136 | `{...base, max_bdy: 1}` | `result-mismatch` 1:11-1:18, on the key |
| 137 | `{...base, max_body: "big"}` | `result-mismatch` 1:21-1:26, on the value |
| 138 | `{...base, ...bad}` | `result-mismatch` 1:14-1:17, on `bad`: it surely gives `listen`, an `int` |
| 139 | `{...bad, ...base}` | Checks: `base` overrides `listen` |
| 140 | `{listen: 1, ...base}` | Checks: `base` surely gives `listen`, so the written `listen` is overridden |
| 141 | `{...base, ...maybe}` | `result-mismatch` 1:14-1:19, on `maybe`: it may give `listen`, an `int`, after `base` |
| 142 | `{...base, ...nums}` | `result-mismatch` 1:14-1:18, once, on `nums`: it may give every key, as an `int` |
| 143 | `{...base, ...extra}` | Checks, with a `dynamic` site 1:1-1:20: the type of e fits, its rest `any` fitting by using any |
| 144 | `{...base, ...blob}` | Checks, with `dynamic` sites 1:1-1:19 and 1:14-1:18 |
| 145 | `{...part, max_body: 1}` | `result-mismatch` 1:1-1:23: no entry surely gives `listen` |
| 146 | `{...base, db: {...base.db, prot: 1}}` | `result-mismatch` 1:28-1:32, on the key `prot` of the inner map |

## 22. The static rules alone

### 22.1 Static diagnostics

The static rules are the rules of `01-language.md` 14.4, made exact here. They
need the syntax tree of the program (`01-language.md` 4), the scopes of 5.1 and
the names of the environment (22.2). They need no type.

| Matter | Rule |
| --- | --- |
| Who applies them | Every implementation, when it compiles a program, whether or not it has a type checker. A program compiled "without a check" is compiled without the type rules, never without these. |
| Phase | `check` |
| Static diagnostic | A diagnostic that this section gives. Its code is one of the eleven static codes: `undefined-name`, `undefined-function`, `not-a-value`, `call-of-parameter`, `lambda-to-lambda`, `duplicate-name`, `shadows-namespace`, `duplicate-key`, `arity`, `duplicate-case`, `unreachable-arm`. |
| Not static | `arity` and `not-a-value` that the type rules give at the arguments of a `MethodCall` (13.2, 13.5). The method of a `MethodCall` is known only from the receiver's kind, so its count of arguments and the place of its lambda arguments are type rules. |
| Message | Not specified in Core. The code and the span are. |

### 22.2 What the rules read from the environment

| Read | From |
| --- | --- |
| The names of the variables | The environment (19.1). Each is a value binding of the outermost scope. |
| The host functions: name, namespace, and the counts of 22.3 | Each host function's `Sig` (19.2) |
| The library functions: name and the counts of 22.3 | Each entry of `library.json` that has no `receiver` (`04-library.md`) |
| The namespaces | The prefixes of the host functions' names (5.4) |

Nothing else is read: no type, no opaque type, no method, no library method. The
type text is parsed by 3.1 and 3.2, and its grammar errors are reported in phase
`declare` (19.5).

### 22.3 Counts of a signature

| Count | Rule |
| --- | --- |
| Required | The parameters with no `?` and no `...` |
| At most | The number of parameters. No bound when the last parameter is a rest parameter. |
| The parameter at argument i | The i-th parameter, from 0. When i is at or past a rest parameter, that rest parameter. |
| An `Fn` parameter | Whether the parameter is an `Fn`, and if so its number of parameters |

A `const`-bound lambda has these counts: required, the position of its last
parameter that has no default, counted from 1, or 0 when every parameter has a
default; at most, the number of its parameters. A parameter written `_` counts.
So `const f = (a = 1, b) => b;` requires 2: `f(5)` is `arity`, and `f(null, 5)`
gives `a` its default.

### 22.4 The walk

The rules walk the tree once. They visit every node exactly once, in the scopes
of 5.1, whatever diagnostics they find. **Nothing is skipped after a static
diagnostic**: the walk goes into every child, the bodies of lambda arguments
included, also when a call's callee is unknown or its count is wrong. The tests
that a diagnostic turns off are listed in 22.5, and they are the only ones.

| Node | The walk |
| --- | --- |
| `Program` | Each `Const` in order (22.7), then the result. |
| `Const` | 22.7 |
| `Block`; a branch of an `if`; the body of a `try` | A new scope. Each `Const` in order, then the result. |
| The handler of a `try` | A new scope. The name of `catch (err)`, if any, is bound in it first (22.7). Then each `Const` in order, then the result. |
| `Ident` | Resolved by the table below. |
| `Call` | 22.5 |
| `MethodCall` | When its step is a plain `.` and its receiver is an `Ident` that names a namespace, it is a `Call` of that namespace (5.4), walked by 22.5. Otherwise 22.6: with `?.`, the receiver `Ident` is walked as an `Ident`, so a namespace there is `not-a-value`. |
| `Lambda` | A new scope for its parameters. Each `Param` left to right: its default first, if any, then its pattern is bound (22.7). Then the body. |
| `Map` | Each entry in order. 22.8 for its keys. A shorthand entry's name is an `Ident`. |
| `Match` | The subject, then each arm's result in order. 22.8 for its literals and arms. |
| Every other node | Its children, in order. |

An `Ident` is resolved to the innermost binding of its name (5.1):

| The `Ident` | Diagnostic |
| --- | --- |
| Its innermost binding is a value binding | None |
| Its innermost binding is a `const`-bound lambda | `not-a-value` on the identifier, unless the `Ident` is a lambda argument (22.5, 22.6) |
| No binding, and the name is a namespace | `not-a-value` on the identifier |
| No binding, and the name is a host function with no namespace or a library function | `not-a-value` on the identifier |
| Otherwise | `undefined-name` on the identifier |

A **lambda argument** is an argument of a `Call` or a `MethodCall` that is a
`Lambda` node, or an `Ident` whose innermost binding is a `const`-bound lambda.
An `Ident` that is a lambda argument gets no diagnostic from the table above.

### 22.5 Calls

The callee of a `Call` is found from its name:

| Name | Callee | Diagnostic |
| --- | --- | --- |
| With a namespace, and the environment has that host function | The host function | None |
| With a namespace, and the environment has no such function | Unknown | `undefined-function` on the namespace and the name |
| Its innermost binding is a `const`-bound lambda | That lambda | None |
| Its innermost binding is a value binding | Unknown | `call-of-parameter` on the name |
| No binding, and the environment has a host function or a library function of that name | That function | None |

An environment variable is a binding (22.2). A variable may have the name of a
library function, and then it shadows the function as a `const` of that name
would: with a variable `type`, `type(x)` is `call-of-parameter` on `type`, and
`type` alone reads the variable.
| No binding, and no such function | Unknown | `undefined-function` on the name |

Then, by the callee:

| Callee | Tests |
| --- | --- |
| Unknown | None. No lambda argument is tested. |
| A `const`-bound lambda, or a function | The count of arguments (22.3): fewer than required, or more than at most, is `arity` on the whole `Call`. After `arity`, no lambda argument is tested. |
| A `const`-bound lambda, with a right count | Each lambda argument is `lambda-to-lambda` on the argument. |
| A function, with a right count | Each lambda argument whose parameter (22.3) is not an `Fn` is `not-a-value` on the argument. A lambda argument whose parameter is an `Fn` with n parameters, and that has more than n parameters, is `arity` on the argument. |

Every argument is walked by 22.4: a `Lambda` node by its row, an `Ident` that is
a lambda argument not at all, any other argument as itself.

### 22.6 Method calls

A `MethodCall` that is not a call of a namespace (22.4): the receiver is walked,
then each argument by the last paragraph of 22.5. No count is tested and no
lambda argument is tested. Where the receiver's type is known, the type rules
test them (13.2, 13.5). At run time, at every `MethodCall`, checked or not, the
run selects the method by the receiver's kind and tests them by the shape test
of `04-library.md` 2.2, before any argument is evaluated. A failure is
`wrong-kind` at the `MethodCall`.

### 22.7 Bindings

**Binding a name** n, written at token t, in scope S:

| Case | Result |
| --- | --- |
| n is `_` | Nothing is bound. |
| n is a namespace (22.2) | `shadows-namespace` on t. Nothing is bound. |
| S already binds n: an earlier `const` name of S, an earlier parameter's name, an earlier name of the same pattern, or the name of `catch` in a handler | `duplicate-name` on t. n is bound again, and the later binding stands. |
| Otherwise | n is bound in S. |

| Binding form | Order |
| --- | --- |
| `const n = Lambda;` | The `Lambda` is walked in the scope as it is before the line. Then n is bound, as a `const`-bound lambda. |
| `const p = e;`, any other | e is walked. Then the pattern p is bound, as value bindings. |
| A `NamePat` | Its name is bound. |
| A `MapPat` | Each field in order: its default first, if any, then its name is bound. Then the rest's name. |
| A `ListPat` | Each element in order: its default first, if any, then its name is bound. Then the rest's name. |
| A `Param` | Its default first, if any, then its pattern. Its names are value bindings of the lambda's scope. |
| `catch (err)` | `err` is bound in the handler's scope, as a value binding. |

So a default sees the names to its left in the same pattern and the parameters
to its left, and a name is not in scope in its own value.

### 22.8 Map literals and `match`

| Rule | Code | Span |
| --- | --- | --- |
| An entry whose key is a `Word`, a `String` or a shorthand name, equal as a string to the key of an earlier such entry of the same `Map`. A `String` key is compared by its value after escapes. A computed key and a spread are not compared. | `duplicate-key` | The later key |
| A literal of a `match` equal by `==` (`01-language.md` 6.7) to an earlier literal of the same `match`, in an earlier arm or earlier in the same arm. `1` and `1.0` are equal. A literal with `-` is the negative number. | `duplicate-case` | The later literal, its `-` included |
| An arm after the first `_` arm of a `match`, `_` arms included | `unreachable-arm` | The arm: from its first literal or `_` to the end of its result |

The literals of an arm after `_` are compared too.

### 22.9 Spans

| Code | Span |
| --- | --- |
| `undefined-name` | The identifier |
| `undefined-function` | The name of the `Call`, from the namespace if there is one |
| `not-a-value` | The identifier; for a lambda argument, the argument |
| `call-of-parameter` | The name of the `Call` |
| `lambda-to-lambda` | The argument |
| `arity` | The whole `Call`, from its name or namespace to its `)`; for a lambda argument, the argument |
| `duplicate-name` | The later name |
| `shadows-namespace` | The name |
| `duplicate-key` | The later key |
| `duplicate-case` | The later literal |
| `unreachable-arm` | The arm |

### 22.10 Order

The static diagnostics are a set: two with the same code and span are one. They
are ordered by start position, then end position, then code in code point order,
as every `check` diagnostic is (6.1).

### 22.11 The guarantee

The full checker reports exactly the static diagnostics of this section, and to
them the type rules add their own (6.1). So:

**For a program in which the full checker reports no diagnostic other than
static diagnostics, the static rules alone report the same list of diagnostics,
in the same order.**

A static diagnostic in the body of a `const`-bound lambda is reported once,
where the text stands, whether the lambda has no use, one use or many.

### 22.12 Compiling with no type checker

| Step | What happens |
| --- | --- |
| Declare | The type text of the environment is parsed. A grammar error is `bad-type` or `bad-signature` (19.5). |
| Parse | `01-language.md` |
| Static rules | This section. With a diagnostic, the compile fails with phase `check` and the list of 22.10. |
| Run | `02-evaluation.md`. A failure has the class of the column "not checked" of 18.5. |

A runner with no type checker does not run `suite/core-check/`. Elsewhere it
gives `skip` to a case that states `type` or `sites`, or expects a `check` code
that is not a static code (`03-host.md`). A case of `suite/core-run/` expects no
`arity` or `not-a-value` at a `MethodCall`: those are type diagnostics (22.1).

### 22.13 Examples

The environment is that of 21.1. Each row gives what the static rules report.
For every row, the full checker reports the same list and nothing more, except
where the row says otherwise.

| # | Program | Static diagnostics |
| --- | --- | --- |
| S1 | `inptu.role` | `undefined-name` 1:1-1:6 |
| S2 | `len` | `not-a-value` 1:1-1:4 |
| S3 | `geo` | `not-a-value` 1:1-1:4 |
| S4 | `lenn(gate.items)` | `undefined-function` 1:1-1:5 |
| S5 | `geo.nowhere(1)` | `undefined-function` 1:1-1:12 |
| S6 | `const t = gate.failed; t(1)` | `call-of-parameter` 1:24-1:25 |
| S7 | `const twice = (f, x) => f(f(x)); twice(n => n + 1, 2)` | `call-of-parameter` 1:25-1:26 and 1:27-1:28; `lambda-to-lambda` 1:40-1:50 |
| S8 | `const inc = n => n + 1; const ap = (v) => v; ap(inc)` | `lambda-to-lambda` 1:49-1:52 |
| S9 | `const inc = n => n + 1; [inc]` | `not-a-value` 1:26-1:29 |
| S10 | `const inc = n => n + 1; inc()` | `arity` 1:25-1:30 |
| S11 | `len(gate.items, 1)` | `arity` 1:1-1:19 |
| S12 | `fail("x", 1, 2)` | `arity` 1:1-1:16 |
| S13 | `geo.distance("a")` | `arity` 1:1-1:18 |
| S14 | `rate(x => x)` | `not-a-value` 1:6-1:12 |
| S15 | `nope(x => y)` | `undefined-function` 1:1-1:5; `undefined-name` 1:11-1:12: the lambda's body is walked. |
| S16 | `const a = 1; const a = 2; a` | `duplicate-name` 1:20-1:21 |
| S17 | `const a = 1; const a = b; a` | `duplicate-name` 1:20-1:21; `undefined-name` 1:24-1:25 |
| S18 | `gate.items.map((t, t) => 1)` | `duplicate-name` 1:20-1:21 |
| S19 | `const geo = 1; geo` | `shadows-namespace` 1:7-1:10; `not-a-value` 1:16-1:19: no binding was made. |
| S20 | `try { 1 } catch (geo) { 2 }` | `shadows-namespace` 1:18-1:21 |
| S21 | `{a: 1, a: 2}` | `duplicate-key` 1:8-1:9 |
| S22 | `match input.role { "admin" => 1, "admin" => 2, _ => 0 }` | `duplicate-case` 1:34-1:41 |
| S23 | `match input.role { "admin" => 1, _ => 0, "owner" => 2 }` | `unreachable-arm` 1:42-1:54 |
| S24 | `const bad = x => y; 1` | `undefined-name` 1:18-1:19, once. The lambda has no use. |
| S25 | `const f = x => y; f(1) + f(2)` | `undefined-name` 1:16-1:17, once, although the type rules check the body at each of the two uses. |
| S26 | `gate.items.map()` | None. The full checker reports `arity` 1:1-1:17, a type diagnostic (22.1), so the guarantee does not apply. |
| S27 | `gate.items.map(t => t.nmae)` | None. The full checker reports `no-field` 1:23-1:27, a type diagnostic. |
| S28 | `const inc = n => n + 1; [1, 2].map(inc)` | None: a lambda argument of a `MethodCall` is not tested by the static rules. |

## 23. What packages add

A package of the specification adds rules to this chapter's and changes none.
Its chapter is the home of its rules.

| Package | Adds | Chapter |
| --- | --- | --- |
| F1, host types | The type a JSON Schema gives, with its options. The printing of a signature. The description of an environment | `spec/packages/f01-host-types.md` |
| F2, templates and analysis | How a host template is checked, and its type in each mode | `spec/packages/f02-templates-analysis.md` 2 |
| F3, string forms | The types of the new string nodes, and raw strings in type text | `spec/packages/f03-strings.md` 8 |
| F6, opt-in groups | The signatures of the groups' entries and the types of the constants; `renders` on `json.stringify` | `spec/packages/f06-groups.md` 10 |
| F13, math functions | The signatures of its eleven entries | `spec/packages/f13-math.md` |
| F4, checked conversion | The types of `x as T` and `x is T`, the facts of `is`, the site of `as`, named types in type text, `untagged-union` for a program type with two records | `spec/packages/f04-conversion.md` 3 and 6 |
| F7, queries | The type of a projection and its sites; the signatures of its entries | `spec/packages/f07-queries.md` 4 and 6 |
| F5, tagged unions | Several record alternatives told apart by a tag: the definition, Join, Fits, Merge, printing, the facts of a test of the tag, `match` over the tag, `in` with a list of literals. `untagged-union` is then for records that form no union | `spec/packages/f05-unions.md` |
| F8, errors, second part | A message template for every `check` code, with the types it prints | `spec/packages/f08-errors.md` 3 |
| F10, config files | What a checked result promises against a type mapped from a schema, and where a host must still validate | `spec/packages/f10-config.md` 2 and 3 |
| F14, host facts | The fact "is null" for a variable a host fact names, the tests that give it, the facts in the right operand of `??`, and its one effect: a member is not null where every other member is null | `spec/packages/f14-host-facts.md` 4 and 5 |
