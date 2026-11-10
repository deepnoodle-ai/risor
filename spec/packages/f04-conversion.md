# F4. Checked conversion

This chapter is normative. It specifies follow-on package F4. It adds to Core:

1. Two operators, `x as T` and `x is T`, which test a value against a type
   written in the program, and the node `As` (2).
2. Named types: a type an environment declares under a name, used in type text
   everywhere (3).
3. The conformance walk: the order in which a value is tested, what each step
   charges, and the path and message of the first failure (4, 5).
4. The checker's rules for the two operators, and the facts `is` gives (6).
5. The conformance function a binding gives a host (7), and the named types in
   an environment's description (8).
6. Two optional fields of a case, `types` and `conform`, and the case of the
   second kind (9).

It adds no run code, no message template, no library entry, no reserved word
and no `declare` code. It raises the run code `not-conforming`, which Core
reserves (`01-language.md` 14.5 item 3), with the three templates Core's
catalog already holds for it. It reports the `parse` code `bad-type-text`, one
of Core's 14 (`01-language.md` 14.2), and the check codes `never-conforms` and
`untagged-union` (`05-types.md` 16.2).

The chapters of Core are cited by file and section: `05-types.md` 3.9 is
section 3.9 of `spec/core/05-types.md`. F1 is `spec/packages/f01-host-types.md`.
Section numbers without a file are of this chapter.

## 1. Conventions

### 1.1 Terms

| Term | Meaning |
| --- | --- |
| **Test** | `x as T` or `x is T`. `x` is the **subject**, `T` the **written type** |
| **Type text** | The text of `05-types.md` 3.1 and 3.2 |
| **Program type** | Type text that stands after `as` or `is` in a program (2.2) |
| **Type name** | The name of an opaque type or of a named type (3) |
| **Resolve** | To make a type from type text: 3.5 |
| **Conforms** | As `05-types.md` 18.1 says. This chapter adds no case to that table |
| **Walk** | The test of a value against a type, in the order of 4.2 |
| **Path** | `03-host.md` 3.7.8: a list of keys and indexes from the tested value to the place of a failure |
| **Surely fits, disjoint, without** | The helpers of 6.2 |

### 1.2 Requirements

1. An implementation that offers F4 offers every section of this chapter.
   Packages are offered in a fixed order, F3, F1, F2, F4 and on (rulings
   BE8), so it also offers the type checker and F1.
2. A compile with the static rules alone (`03-host.md` 3.2.1 item 4) applies
   2 to 5 and the static rules of 6.1: it resolves every named type, program
   type and type given to the conformance function (3.5), and runs the walk
   of 4. It applies no rule of 6.2 to 6.7.

### 1.3 Version

F4 is part of version `3.0`. It adds no version number. In an environment of
version `3.0` every form of this chapter exists.

### 1.4 What this package changes in Core's rules

| Core rule | With this package |
| --- | --- |
| `01-language.md` 3.1, the production `Rel` | Gains `as` and `is` followed by a program type (2.1) |
| `01-language.md` 3.3, level 6 | Gains `as` and `is` (2.4) |
| `01-language.md` 4.1 item 2 | Node 31, `As`, exists (2.7) |
| `01-language.md` 13.1 | `as` and `is` open a level, and so do the brackets and `[]` suffixes of a program type (2.8) |
| `01-language.md` 14.2, `bad-type-text` | Reported, by 2.6 |
| `01-language.md` 14.3, the row `x as T`, `x is T`, `x as const`, `x is not T` | No longer applies to `x as T` and `x is T`. `x as const` and `x is not T` keep the row (2.6) |
| `01-language.md` 14.3, the row `!x as T` | Stands (2.5) |
| `01-language.md` 14.5 item 3 | `as` raises `not-conforming` (5.1) |
| `02-evaluation.md` 5.2, 6.1, 12.4 | Gain the rows of 5.3 and 5.5 |
| `02-evaluation.md` 8.4 item 1 | A walk of this chapter is a walk (5.4) |
| `03-host.md` 3.3.1 | Gains the declaration of a named type (3.1) |
| `05-types.md` 3.5, 19.5 | A `Name` may be a named type (3.3) |
| `05-types.md` 4.5 | Gains surely fits, disjoint and without (6.2) |
| `05-types.md` 10.4, 10.5 | Gain the facts of `is` (6.4) |
| `05-types.md` 18.3 | An `as` is a `value` site (6.6) |
| `05-types.md` 22 | Gains the static rules of 6.1 |
| F2 3.2, 3.5 and 3.6, the analysis | The analysis walks the subject of an `As` node as it walks any operand. The program type names nothing: a type name in it is not a read, even when a variable has that name, and a key of a record in it is not a path. A subject's path stops at the `As` node, which ends the chain |

## 2. Syntax

### 2.1 Grammar

The production `Rel` of `01-language.md` 3.1 becomes:

```ebnf
Rel   = Add { ( "<" | "<=" | ">" | ">=" | "in" | "not" "in" ) Add
            | ( "as" | "is" ) PType } .
PType = PAlt { "|" PAlt } .
PAlt  = PPrim { "[" "]" } .
PPrim = "null" | "bool" | "int" | "float" | "string" | "any" | "never"
      | String | "map" "<" PType ">" | PRecord | Name | "(" PType ")" .
PRecord = "{" [ ( PField { "," PField } [ "," PRest ] | PRest ) [ "," ] ] "}" .
PField  = ( Word | String ) [ "?" ] ":" PType .
PRest   = "..." [ PType ] .
```

1. `PType` is the `Type` of `05-types.md` 3.1 without `Var`. A signature, an
   `Fn`, `unknown` and `condition` cannot stand there.
2. `Name` in `PPrim` is the type name of `05-types.md` 3.1: an upper-case
   letter, then one or more of `[A-Za-z0-9_]`. It is not the `Name` of
   `01-language.md` 2.4: a type name and a value name are two sets (3.3).
3. `String` is a one-line string or a raw string (F3). Its value is a literal
   set of one string.
4. Every other production of `01-language.md` 3.1 is unchanged. Every place an
   `Expr` stands may hold a test: a `const` value, an argument, a lambda body, a
   list item, a map value, an index, an interpolation, a `match` subject.

### 2.2 Program types

A program type is read with the tokens of `01-language.md` 2.3. White space and
comments may stand between its tokens.

| Matter | Rule |
| --- | --- |
| Type words | `null`, `bool`, `int`, `float`, `string`, `any`, `never` and `map` are words of a program type only where a `PPrim` is expected. Elsewhere `int` and `string` are names of the library, as before. Of the eight only `null` is reserved, as in Core |
| A word where a `PPrim` is expected | A type word; or a type name; or else `bad-type-text` (2.6). So a lower-case word that is not a type word, a reserved word, `_`, and one upper-case letter are errors there |
| `{` where a `PPrim` is expected | Starts a `PRecord`, never a map or a block |
| `<` | Belongs to a program type only directly after the word `map` |
| `>` | There is no `>>` token, so `map<map<int>>` ends with two `>`. `>=` is always one token |
| `\|` | Joins alternatives. `\|\|` is one token, and is not `\|` |
| A key of a `PField` | A `Word` (any word, a reserved word among them) or a `String` |
| `?` | Stands only after a field's key, before `:` |

### 2.3 Where a program type ends

The parser reads the longest program type. After a complete `PPrim`, and after
each `[]` suffix, two tokens may continue the type, and no other:

| Token | Continues the type when |
| --- | --- |
| `[` | The token after it is `]`. The two are one suffix. Otherwise the type ends before the `[` |
| `\|` | Always. An alternative must follow it (2.6) |

1. Every other token ends the type: `?`, `:`, `,`, `)`, `]`, `}`, `{`, `;`,
   `=>`, `\|\|`, `&&`, `??`, `==`, `!=`, `<`, `<=`, `>`, `>=`, `in`, `not`,
   `as`, `is`, `+`, `-`, `*`, `/`, `%`, `**`, `.`, `?.`, `?.[`, `(`, a word, a
   literal, and the end of input.
2. A `?` always ends a type, because no type has a `?` suffix. So `x is T ? a :
   b` is a conditional.
3. A `{` after a complete type ends it. So `match x as "a" | "b" { ... }` needs
   no parentheses: the `{` opens the arms.
4. What follows the type is then read by `01-language.md` 3.1. The table of
   2.4 says what that allows.

### 2.4 Precedence

`as` and `is` stand at level 6 of `01-language.md` 3.3, with `<`, `<=`, `>`,
`>=`, `in` and `not in`, and associate to the left. Their right operand is a
program type, not an expression.

| Text | Reads as |
| --- | --- |
| `a + b as T` | `(a + b) as T` |
| `x as int > 3` | `(x as int) > 3` |
| `3 < n as int` | `(3 < n) as int` |
| `y == x as T` | `y == (x as T)` |
| `x as T == y` | `(x as T) == y` |
| `x is T && y` | `(x is T) && y` |
| `x is T ? a : b` | `(x is T) ? a : b` |
| `x as T \| null` | `x as (T \| null)` |
| `x as T \|\| d` | `(x as T) \|\| d` |
| `x as T as U` | `(x as T) as U` |
| `a in b as T` | `(a in b) as T` |
| `x as T in y` | `(x as T) in y` |
| `x as map<int> > y` | `(x as map<int>) > y` |
| `c ? x as T : y`, `{k: x as T, j: 1}`, `xs[i as int]`, `f(x as T)` | the `:`, `,`, `]` and `)` end the type |

Each of these is `unexpected-token` at the token in the second column, by the
productions. Package F8 gives them a targeted code at the same span.

| Text | Span | Write |
| --- | --- | --- |
| `x as int + 1`, and every other operator of levels 7 to 10 after a type | The operator | `(x as int) + 1` |
| `x as T ?? d` | The `??` | `(x as T) ?? d` |
| `a ?? b as T` | The `as` | `(a ?? b) as T` |
| `x as T.f`, `x as T?.f` | The `.` or `?.` | `(x as T).f` |
| `x as T[0]`, `x as T[][i]` | The `[` that is not followed by `]` | `(x as T[])[0]` |
| `x as T(1)` | The `(` | |
| `x as T { }` outside a `match` subject | The `{` | |

### 2.5 A `!` operand

The left operand of `as` or `is` may not be a `!` expression written without
parentheses, as for `in` (`01-language.md` 3.4 rule 2). `!x is T` would test
`!x`.

1. `!x as T`, `!x is T`, `a == !x is T` and `a && !x as T` are
   `unexpected-token` at the `!`: the first prefix operator of the left
   operand, the outermost. The parser finds it at the `as` or the `is`, before
   it reads the type (`01-language.md` 14.1).
2. The left operand is what a comparison's would be. In `a + !x as T` it is `a
   + !x`, and in `a < !x as T` it is `a < !x`. Neither is a `!` expression, so
   neither is this error.
3. `!(x is T)` and `(!x) is T` are valid.

### 2.6 Errors in a program type

**`bad-type-text`.** Text where a program type is read that 2.1 to 2.3 do not
accept is `bad-type-text`, phase `parse`, at the token at fault.

| Text | Span |
| --- | --- |
| A token that cannot begin a `PPrim` where one is expected: after `as` or `is`, after `\|`, after `(`, after `map<`, after a field's `:`, and after `...` when the token is not `,` or `}`. `x as` then the end of input, `x is 3`, `x is true`, `x as number`, `x as T`, `x is (` then `)` | That token. At the end of input, the empty span there |
| A word after `map` that is not followed by `<`; a `PType` after `map<` not followed by `>`, as in `x is map<int>=> 1` | The token that stands where `<` or `>` should |
| A `PType` after `(` not followed by `)` | The token that stands where `)` should |
| In a `PRecord`: a token that cannot begin a field, a rest or the `}`; a token after a key that is not `?` or `:`; a token after `?` that is not `:`; a token after a field or a rest that is not `,` or `}`; a field after a rest | That token |
| A key written twice in one `PRecord`, compared by its value after escapes | The later key |
| A template, a text block or a raw text block where a `PPrim` or a key is expected | The whole literal, from its opening delimiter to the end of its closing one |

1. A one-upper-case-letter word is a type variable of a signature and is
   `bad-type-text` here. A lower-case word that is not a type word, a reserved
   word among them, is `bad-type-text`, with these two exceptions.
2. **`x as const`** is `unexpected-token` at the `as`, and **`x is not T`** is
   `unexpected-token` at the `is`, as `01-language.md` 14.3 says. Package F8
   gives each a targeted code at that span. `x is const` and `x as not` are
   `bad-type-text` at `const` and at `not`.
3. A token of the last row of `01-language.md` 2.3, the block token `'''` and a
   prefixed literal stay `unexpected-token` wherever they stand, a program type
   included, with the spans of 2.3 rule 4.
4. A lexical diagnostic anywhere in the text comes first (`01-language.md`
   2.11). A text block's own code (F3 6.1) comes before `bad-type-text` for the
   same literal.
5. The first diagnostic is the first rule broken as the parser reads the
   tokens, by `01-language.md` 14.1. A `nesting-exceeded` at a token comes
   before `bad-type-text` at the same token.
6. Package F8 gives some of these texts a message that names the fix. It keeps
   the code `bad-type-text` and the span.

**Errors that need the environment.** A type name that the environment does not
declare, two records in one type, and a type deeper than 64 are found when the
type is resolved, by the static rules (6.1). They are not `parse` errors.

### 2.7 The node `As`

| # | Node | Fields | Written |
| --- | --- | --- | --- |
| 31 | `As` | `form` (`as` or `is`), `x`, `type` | `x as T`, `x is T` |

1. The language has 31 node kinds with this package: the 30 of
   `01-language.md` 4.1 and `As`. Number 32, `Each`, belongs to F7.
2. `type` is the program type, as text and as the type it resolves to (3.5). It
   is not a node, and nothing in it is evaluated.
3. **Spans.** The span of an `As` node runs from the start of `x` to the end of
   the last token of its program type. The span of the program type runs from
   its first token to its last. Parentheses around the subject are part of the
   `As` node, as for any operand (`01-language.md` 4.3 item 1): in `(x) as int`
   the node spans the whole text.
4. Unlike `form` of a `String`, `form` of an `As` is read: it decides what the
   node does (5.1, 5.2).

### 2.8 Nesting

These rows join the table of `01-language.md` 13.1.

| Opens a level | The level lasts |
| --- | --- |
| The binary operator `as` or `is` | To the end of its program type |
| In a program type: `(`, `{`, and the `<` of `map<` | To its closing bracket |
| In a program type: the `[` of each `[]` suffix | To the end of its `PAlt`. The levels of the suffixes of one `PAlt` add up: in `int[][][]` the third `[` opens the third level above the `int` |

1. The `\|` between alternatives, the `,` between fields, `?`, `:` and `...`
   open no level.
2. The token that would open level 101 is `nesting-exceeded`, at that token
   (`01-language.md` 13.2). A `[` that is not followed by `]` is not a suffix
   and opens no level.
3. So in `x as int[]`, with `x` at level 0, `int` is at level 1 and the `[`
   opens level 2. With 98 suffixes the last `[` opens level 99; with 100, the
   100th `[` is `nesting-exceeded`.

## 3. Named types

### 3.1 The declaration

This row is added to the table of `03-host.md` 3.3.1.

| Declaration | States | Portable form |
| --- | --- | --- |
| Named type | A name and a type | The type as type text, or, with F1, as a JSON Schema with the options of F1 4 |

1. **An abbreviation.** A named type stands for its type in every rule: Fits,
   Join, Merge, Truthy and Falsy, the helpers of 6.2, conformance, depth and
   printing. Nothing is nominal. Two names for one structure are one type. A
   type prints by its structure, never by a name (`05-types.md` 3.9).
2. **Declared in order.** The named types of an environment form a list. Each
   is resolved (3.5) in its turn, against the opaque types of the environment
   and the named types before it in the list.
3. **No recursive type.** A named type may use only names declared before it.
   So it cannot refer to itself, directly or through others.
4. **By schema.** A named type declared by schema has the type F1 3 gives, with
   the options given, each off unless given. A schema names no type of the
   environment (F1 2.1 item 3).
5. **Shared, not copied.** An implementation holds each named type once and
   refers to it. Replacing every name in place may double the size of a type
   for each named type in a chain, and must not be done by copying.

### 3.2 Errors

These are reported when the environment is built, whether the program is
then compiled with the check or with the static rules alone. Each is phase
`declare`. A `declare`
diagnostic has no span.

| Situation | Code |
| --- | --- |
| A name that is not an upper-case letter followed by one or more of `[A-Za-z0-9_]`. One upper-case letter alone is a type variable | `bad-name` |
| Two named types with one name, or a named type and an opaque type with one name | `name-clash` |
| Type text that the grammar of `05-types.md` 3.1 and the token rules of 3.2 do not accept as a `Type`; text deeper than 64 by `05-types.md` 20.5 | `bad-type` |
| A key written twice in one record of the text | `bad-type` |
| A `Name` that is not an opaque type of the environment or a named type declared before it: undeclared, declared later, or its own | `bad-type` |
| Two or more record alternatives in one `Type` of the text, after names are replaced (3.5) | `bad-type`. Package F5 accepts records that form a tagged union |
| A type deeper than 64 (`05-types.md` 2.5) once its names are replaced | `bad-type` |
| A type whose size is over 100,000 once its names are replaced, by the limit on the size of a type of `05-types.md` 20 | `bad-type` |
| A schema that F1 3 refuses | `bad-type` |

1. A cycle is reported as a name not declared before it: in the list `A = B`,
   `B = A`, the first entry is `bad-type`, and the second then names an `A`
   that failed. An implementation reports at least one `bad-type`. A case
   expects exactly one diagnostic, so the suite's cases of a cycle have one
   named type that names itself or a later one.
2. A named type with an error is not declared. Whether a later declaration
   that names it is reported as well is not specified, and no case of the
   suite has one.
3. A type name and a variable, function or namespace name never clash
   (`03-host.md` 3.3.4 item 1).

### 3.3 Where a type name may stand

| Place | Rule |
| --- | --- |
| The type of a variable, by type text | Any type name of the environment, wherever it stands in the list |
| A host function's signature, a method's signature | The same |
| The expected result (`returns`) | The same |
| A later named type | The opaque types, and the named types declared before it (3.1) |
| A method's receiver (`Recv`) | An opaque type only. A named type there is `bad-signature`, in every implementation (`05-types.md` 3.6) |
| A program type | Any type name of the environment (6.1) |
| The type given to the conformance function (7) | Any type name of the environment |

1. `05-types.md` 3.5 and 19.5 read "a declared opaque type or named type"
   wherever they read "a declared opaque type", but for a `Recv`.
2. In a variable's type, a signature and the expected result, a `Name` that
   is neither is `bad-type` or `bad-signature` (`05-types.md` 19.5).

### 3.4 Child environments

These are binding rules (`03-host.md` 3.3.2). No case of the suite has a child
of a child.

1. A child may declare a named type its parent declares. The child's replaces
   it in the child only. A type of the parent that used the name keeps the
   type it had.
2. A child that gives a parent's opaque type name to a named type, or a
   parent's named type name to an opaque type, is `name-clash`.

### 3.5 Resolving type text

To resolve type text against an environment:

1. Parse it by `05-types.md` 3.1 and 3.2, or by 2.1 for a program type.
2. Replace each `Name` by the type it names: an opaque type, or a named type's
   type.
3. Remove parentheses, and join the alternatives of each `Type` that have one
   kind, by `05-types.md` 3.4 and the Join of `05-types.md` 4.1. So `int \|
   float` is `float`, `"a" \| string` is `string`, and `{a: int}[] \| {b:
   int}[]` is `{a?: int, b?: int}[]`.
4. Two or more record alternatives of one `Type`, `map<T>` among them, after
   step 2, are an error: `bad-type` in a declaration (3.2), `untagged-union` in
   a program type (6.1). A named type that is a record counts as one record:
   `Order \| Item`, with both records, is two. Package F5 accepts records that
   form a tagged union.
5. The depth of the result is that of `05-types.md` 2.5. More than 64 is an
   error: `bad-type` in a declaration, `check-limit` in a program type.
6. The size of the result is that of the limit on the size of a type
   (`05-types.md` 20): the size of the type written out with no sharing. More
   than 100,000 is an error: `bad-type` in a declaration, `check-limit` in a
   program type. An implementation keeps the size with each named type,
   saturating above the limit, and never expands a type to find it.

Type text is resolved this way in every compile, the static rules alone
included. The run-time test of 4 is made on the resolved type.

## 4. The conformance walk

### 4.1 The rule

A value conforms to a type by `05-types.md` 18.1. The walk of 4.2 decides the
same answer, and fixes what that table leaves open: the order in which the
value is looked at, the first failure, its path and message, and the steps.

### 4.2 The walk

**Test**(v, T, path) tests a value v against a resolved type T. Its answer is
**pass**, or a **failure**: a path and one of the three templates of 4.4. A
test of the subject starts with the empty path.

| Step | Rule |
| --- | --- |
| 1. `any` | If T is `any`: pass. Nothing is charged, v is not looked at, and nothing inside it is |
| 2. Enter | If v is a list and T has a list alternative, or v is a map and T has a map alternative, the walk **enters** v: the depth test of 5.4 |
| 3. Charge | 1 step. For the value of a declared field, this step is the field's lookup of 4.3 step 1, already charged, and step 2 comes after it |
| 4. A value that cannot enter | If v is a host number that cannot enter the language (`03-host.md` 3.4.4): failure, template `wrong` |
| 5. The alternative | Take the alternative of T of v's kind: the `null`, `bool`, number, string, list or map alternative, or for an opaque value the opaque alternative with v's name. None: failure, template `wrong` |
| 6. Scalars | `null`, `bool`, `string`, `float`, an opaque name: pass. `int`: pass if v is a whole number (`01-language.md` 6.2), else failure, `wrong`. A literal set: charge the size of v (`02-evaluation.md` 4), then pass if v is a member, else failure, `wrong` |
| 7. A list, E`[]` | Test each element in order of index, the element at index i with path + [i]. At the first failure, stop: that failure is the answer. Otherwise pass |
| 8. A record | The rows of 4.3 |
| 9. Leave | If the walk entered v at step 2, it leaves v when the test of v ends, by pass or by failure |

1. **Nothing is converted.** The string `"42"` does not conform to `int`, and
   `3.0` conforms to `int`: no operation can tell how a number is stored.
2. **A test never looks under `any`.** `x is any[]` charges 1 step and visits
   no element. `{meta: any, ...}` looks up `meta` and does not look at its
   value.
3. **Absent is not null.** A field is present when the map has the key,
   whatever its value. So `{"id": "a", "note": null}` does not conform to
   `{id: string, note?: string}`, and fails at `["note"]`.
4. **A host number that cannot enter** conforms to no type. The walk reports
   `not-conforming` at its path, never `number-range`. At a place whose type is
   `any` it is not looked at.

### 4.3 A record

The value v is a map, and R is the record alternative of T.

| Step | Rule |
| --- | --- |
| 1. Declared fields | Take R's fields in code point order of their keys. For each field k: charge 1 + the size of k, whether or not v has the key (the key rule, `02-evaluation.md` 5.3 item 5). If v lacks k: when the field is required, failure at path + [k], template `missing`; when it is optional, go on. If v has k: Test(v[k], the field's type, path + [k]), whose step 3 is this lookup. A failure is the answer. A field of type `any` costs its lookup, and its value is not looked at |
| 2. Other keys | Reached only when every declared field passed. The **other keys** are the keys of v that R does not list. Their number m is the size of v less the number of declared keys v has, so finding it costs nothing |
| 3. Rest `any`, or m = 0 | Pass. No other key is visited |
| 4. Rest `never`: an exact record | Charge m + the size of the m other keys taken together (`02-evaluation.md` 4 item 3). Failure at path + [k], template `extra`, with k the least other key by code point |
| 5. Another rest U | Charge the size of the m other keys taken together. Then Test(v[k], U, path + [k]) for every other key k, in code point order, each to its end. Every one is tested, also after one fails. If any failed, the answer is the failure of the least failing key by code point. Otherwise pass |

1. Declared fields are tested before other keys, and other keys are reached
   only when every declared field passed.
2. Step 5 does not stop early, so its count does not depend on the order of a
   host map's keys. The key reported is the least, whatever the order.
3. The keys of a map are compared by code point (`01-language.md` 6.7 item 5).

### 4.4 The failure

A failure has a path and a template of the `not-conforming` entry of
`suite/messages.json`, with its parameters.

| Template | Text | When | Parameters |
| --- | --- | --- | --- |
| `wrong` | `expected {expected}, found {found}` | 4.2 steps 4 to 6 | `expected`: the type tested at the path, printed by `05-types.md` 3.9. `found`: the value at the path |
| `missing` | `required field {key} is missing` | 4.3 step 1 | `key`: the field's key |
| `extra` | `the type does not allow key {key}` | 4.3 step 4 | `key`: the least other key |

1. Each parameter is rendered by its kind (`03-host.md` 3.8.3) and cut to 64
   code points: `expected` is of kind `type`, `found` of kind `value`, `key` of
   kind `string`, in its JSON form with the quotes.
2. `expected` is the type T of the test that failed: the whole alternative
   set, not the alternative of v's kind. For `x as int \| null` with `x` the
   string `"a"`, the message is `expected int | null, found "a"`. A named type
   is printed by its structure.
3. For a host number that cannot enter, `found` is the word `number`.
4. For `missing` and `extra` the last step of the path is the key.
5. The path has at most 64 steps: each step enters a level, and a resolved
   type is at most 64 deep.

### 4.5 Where package F5 attaches

Package F5 lets the map alternative of a type be a tagged union. It adds a row
to 4.2 step 5 for a map alternative that is a tagged union: the walk reads the
tag and then tests one variant by the rows of 4.3. It changes no other row.
Every step of this section holds for a type whose map alternative is one
record.

## 5. Evaluation

### 5.1 `x as T`

1. `x` is evaluated, then Test(value of x, T, []) is made (4.2).
2. On pass, the value of the node is the value of `x` itself. Nothing is copied
   or changed, so `(x as T) == x` is `true`.
3. On failure, the node fails with `not-conforming`, at the span of the `As`
   node. The diagnostic's `path` is the failure's path, and its message is the
   failure's template with its parameters (4.4).
4. The class of the failure is that of `03-host.md` 3.7.6: `value` at a
   `value` site, `environment` elsewhere in a checked program, `value` in a
   program compiled without a check. 6.6 says where the sites are.
5. `catch` catches `not-conforming` (`01-language.md` 11.3).

### 5.2 `x is T`

1. `x` is evaluated, then Test(value of x, T, []) is made.
2. The value is `true` on pass and `false` on failure. `is` never fails but by
   a limit: `steps-exceeded`, `depth-exceeded`, `deadline-exceeded`,
   `cancelled`, or a failure while `x` is evaluated.
3. No message and no path is built.

### 5.3 Cost rows

These rows join the tables of `02-evaluation.md` 5.2 and 6.1.

| Form | Steps | Units | When |
| --- | --- | --- | --- |
| `x as T`, `x is T`: the operator | 0 | 0 | |
| The walk | By 4.2 and 4.3 | 0 | After `x`, charged as the walk goes (`02-evaluation.md` 2.5 item 3) |

1. The walk charges, for each value tested: 1 at step 3 of 4.2, and nothing
   for a value tested against `any`. Further: the size of a string tested
   against a literal set; 1 + the size of the key for each declared field
   looked up, which is also the step of the field's value; for an exact record
   with other keys, m + their size taken together; for another rest, the size
   of the other keys taken together.
2. **A failure is charged as the walk went.** What the walk charged before it
   stopped stays charged. The failure builds its message outside the usage
   (`02-evaluation.md` 12.4 item 4).
3. `as` and `is` charge the same steps for the same value and type.
4. The type is text of the program. Resolving it, and making the run-time
   form of the test, is done when the program is compiled, and charges
   nothing.
5. The walk creates no value. `as` gives back its subject.

### 5.4 Depth

1. A walk of this chapter is a walk of `02-evaluation.md` 8.4. It adds 1 to the
   depth count for each list or map it has entered (4.2 step 2) and not left.
2. The depth test is made on entering, before the step of 4.2 step 3, and
   `depth-exceeded` is raised when the count would be more than the depth
   limit. For the value of a declared field, the lookup of 4.3 step 1 is
   charged first, then the depth test.
3. An empty list or map is entered like any other. A value tested against
   `any` is not entered.
4. The walk enters a level only where the type has one, so it enters at most
   64 levels whatever the data: host data nested 10,000 deep, tested against
   `{meta: any, ...}`, is entered once.
5. The count at a test is the count of the calls in progress (`02-evaluation.md`
   8.4 item 1), plus the walk's levels. A test adds nothing else.
6. A failure that `catch` catches returns the count to its value when the
   `try` began. So a caught `not-conforming` leaves no level entered.

### 5.5 `try` and the error value

This row joins the table of `02-evaluation.md` 12.4.

| Part | Steps | Units |
| --- | --- | --- |
| `path` of a `not-conforming` failure, a path of n steps | 0 | 1 + n. A key in it is a string that exists, and costs nothing more |

1. The error value of a caught `not-conforming` has `code` `"not-conforming"`,
   the message of 4.4, `path` the failure's path as a list of strings and whole
   numbers, and `data` null.
2. So `try { x as T } catch (err) { err.path }` charges the walk, then 1 + the
   size of the message in steps, and 9 + (1 + the size of the message) + (1 +
   n) in units, before the handler runs.
3. `try { x as T } catch { d }` charges the walk and nothing for the failure.
4. The sites of an `as` in the body of a `try` are dropped (`05-types.md`
   14.1), so an `as` there is no diagnostic in an environment that rejects the
   class `value`.

### 5.6 Optimizing

This row joins the table of `02-evaluation.md` 10.2.

| An implementation may | Under this condition |
| --- | --- |
| Decide a test, or part of one, without the walk | The answer is proved from the value itself, as from the static type of a host field that the binding reads. The steps and the failure are those of the walk. A type the checker inferred is not a proof: a host value may break its declared type, and a run without a check behaves the same |

## 6. Checking

### 6.1 The static rules

These rules join `05-types.md` 22. They are applied in every compile, the
static rules alone included, and each is reported in phase `check`. They need
the program, the environment's type names and the named types' types. Unlike
the size of an expression's type (`05-types.md` 20), the size of a program type
is tested here: the type is resolved in every compile.

| Rule | Code | Span |
| --- | --- | --- |
| A `Name` in a program type that is not a type name of the environment | `undefined-name` | The name |
| Two or more record alternatives in one `PType`, after names are replaced (3.5 step 4) | `untagged-union` | That `PType`, from its first token to its last: the innermost `PType` whose own alternatives, with parentheses removed, hold the records |
| A program type whose resolved type is deeper than 64 (3.5 step 5), or whose size is over 100,000 (3.5 step 6) | `check-limit` | The program type |

1. `05-types.md` 22.2 gains a row: the rules read the names of the opaque
   types and the named types of the environment, and the named types' types.
2. The tests of a program type are made in this order: names; then records;
   then depth and size, one `check-limit` for both. When a name is undefined,
   the type is not tested for records, depth or size. When a `PType` has two
   records, its program type is not tested for depth or size.
3. Each `untagged-union` is reported once for each `PType` that has two
   records. Package F5 accepts records that form a tagged union, and keeps
   `untagged-union` for those that do not.
4. **Static codes.** `untagged-union` and `check-limit` at a program type are
   static diagnostics. For `03-host.md` 3.10.8, 3.10.10 item 17 and 3.10.13,
   in `suite/conversion/` the static codes are the eleven of Core and these
   two. A case there that expects either expects it at a program type.
   Elsewhere `check-limit` is a type rule, as before.
5. After a diagnostic of this section, the type rules give the `As` node the
   type `never` for `as` and `bool` for `is`, record no site on it, give it no
   fact, and report nothing more for it. The subject is checked as usual.
6. The static rules walk the subject of an `As` node as any operand.

### 6.2 Helpers

These join the helpers of `05-types.md` 4.5.

**Surely fits.** S surely fits T when Fits(S, T) holds (`05-types.md` 4.2) and
the fit did not use any. Every value that conforms to S then conforms to T.
`any` surely fits only `any`.

**Disjoint.** disjoint(A, B) is true when the checker can show that no value
conforms to both. It is decided by the first row that applies.

| Case | disjoint(A, B) |
| --- | --- |
| Either is `never` | True |
| Either is `any` | False |
| Otherwise | True unless some kind has an alternative in both, and those two share a value by the rows below |
| null, bool, an opaque name | They share a value |
| number | They share a value: a whole number conforms to `int` and to `float` |
| string | They share a value, unless both are literal sets with no string in common |
| list | They share a value, whatever the element types: the empty list conforms to both |
| map: two records | They share a value, unless some key that is required in either record has disjoint types in the two. The type of a key in a record is its field's type if the record lists the key, else the record's rest |

1. The rule is sound and not complete. When it says true, no value conforms to
   both. It may say false of two types that share no value, such as `any` and
   `{a: never}`.
2. It recurses on the types and goes no deeper than they do.

| A | B | disjoint |
| --- | --- | --- |
| `"42"` | `int` | yes: no kind in both |
| `int \| null` | `string \| null` | no: null |
| `int[]` | `string[]` | no: `[]` |
| `"a" \| "b"` | `"b" \| "c"` | no |
| `{id: int}` | `{id: string, ...}` | yes: `id` is required |
| `{id: int}` | `{name: string}` | yes: `name` is required in B, and A's rest is `never` |
| `{id?: int}` | `{name?: string}` | no: `{}` |

**Without.** without(X, T) is the type left of X when a value of type X is known
not to conform to T.

| X | without(X, T) |
| --- | --- |
| `any` | `any` |
| Otherwise | X less each alternative that surely fits T, taken as a type of one alternative. A literal set that does not surely fit T loses each string of T's literal set, and goes when none is left. Nothing else is split |

So without(`int \| string`, `int`) is `string`, without(`"a" \| "b" \| "c"`,
`"a" \| "c"`) is `"b"`, and without(`float`, `int`) is `float`: a number that is
not an `int` is still a `float`.

### 6.3 The node `As`

Let S be the type of the subject, and T the resolved type.

| Case, in this order | `x as T` | `x is T` |
| --- | --- | --- |
| A diagnostic of 6.1 on the type | `never` | `bool` |
| S is `never` | T. Nothing is reported. No site | `bool`. Nothing is reported |
| S surely fits T | T. Nothing is reported. No site: under the precondition the test cannot fail | `bool`. `always-truthy` on the `As` node |
| disjoint(S, T) | `never-conforms` on the `As` node. The type is T | `bool`. `always-falsy` on the `As` node |
| Otherwise | T. A `value` site on the `As` node | `bool`. No site |

1. **An untyped subject is the last case.** Fits(`any`, T) uses any, so `blob
   as Order` is a test that runs, and a `value` site.
2. **The subject may have any type.** Null is not refused, and an `any` subject
   is not a `dynamic` site.
3. **The type of `x as T` is T as resolved,** also where S says more: `n as
   float` with `n: int` is a `float`. `[] as string[]` widens `never[]`.
4. `always-truthy` and `always-falsy` on an `is` are of the class
   `constant-condition` (`05-types.md` 17), and are reported only when the
   environment rejects it. The node is a `bool`, so a tested position it
   stands in reports nothing more (`05-types.md` 10.2).
5. `never-conforms` has no class: it is always reported.
6. The run does not depend on the case. The test and its cost are fixed by the
   value and by T. The checker decides only what is reported, the type, and
   whether the node is a site.

### 6.4 Facts

`05-types.md` 10.4 gains two kinds of fact.

| Fact | Written | Says | An expression that is the path p then has |
| --- | --- | --- | --- |
| conforms | {p: T} | The value of p conforms to T | T. If the type of p already surely fits T, it is unchanged |
| does not conform | {p: not T} | The value of p does not conform to T | without(the type of p, T) |

These rows join the table of `05-types.md` 10.5. p is a path (`05-types.md`
10.3) and T a resolved type.

| c | T(c) | F(c) |
| --- | --- | --- |
| `p is T` | {p: T}. Also {p}, when T has no null alternative and is not `any` | {p: not T}. Also {p}, when T has the null alternative |

1. A test whose subject is not a path gives no fact. Bind the value with
   `const` to test it.
2. An `x is T` with a diagnostic of 6.1 gives no fact.
3. **Two facts are the same** when they are of one kind and about one path,
   and, for the two kinds of this section, about one type by structure. This
   is what ∩ compares in `05-types.md` 10.5.
4. **`||` joins two facts of `is`.** In T(d || e), when T(d) holds exactly
   one conforms fact about a path p, {p: A}, and T(e) exactly one about the
   same path, {p: B}, the two give one fact {p: Join(A, B)} (`05-types.md`
   4.1), which stands where {p: A} stood. It replaces, for these two, the ∩ of
   `05-types.md` 10.5; every other fact of the two sides is kept only if both
   have it, as before. `||` reads from the left, so a third operand joins the
   same way. When a side has no conforms fact about p, or more than one, there
   is no conforms fact about p. F(d || e) is F(d) ∪ F(e), as before: the
   failing side applies without for each type in turn.
5. Facts on one path are applied in the order of their tests in the source,
   as before. A fact never reaches a binding that shadows its path's name.
6. The {p} beside a fact of `is` carries the prefix rule of `05-types.md`
   10.4: where `input?.note is string` passed, `input` is not null and `note`
   is present, so `input.note.trim()` checks.
7. A fact about a field changes the type of no field of a record type, as
   before. {`x.a`: T} gives `x.a` the type T and changes nothing in the type of
   `x`.

| Written | With | Type |
| --- | --- | --- |
| `v is int ? v + 1 : v.trim()` | `v: int \| string` | `int \| string` |
| `role is "admin" \| "owner" ? 1 : role` | `role: "admin" \| "member" \| "owner"` | `"member" \| int` |
| `if (body is {id: string}[]) { len(body) } else { 0 }` | `body: any` | `int` |
| `input?.note is string ? input.note.trim() : ""` | `input: {note?: string}` | `string` |
| `if (u is int \|\| u is string) { u } else { "x" }` | `u: bool \| int \| string` | `int \| string` |
| `u is int \|\| u is string ? "x" : u` | the same | `bool \| "x"` |
| `role is "admin" \|\| role is "owner" ? role : "x"` | `role` as above | `"admin" \| "owner" \| "x"` |

### 6.5 `filter` and `find`

In `05-types.md` 13.8, the facts that reach the parameter x are {x}, {p} for a
longer path p that starts with x, and the two facts of 6.4 about x. So
`xs.filter(x => x is string)` with `xs: (int | string)[]` is `string[]`, and
`xs.find(x => x is int)` is `int | null`.

### 6.6 Sites, classes and the promise

**The site.** This row is added to the table of `05-types.md` 18.3.

| Class | Recorded on | Span |
| --- | --- | --- |
| `value` | An `As` node of form `as`, in the last case of 6.3 | The `As` node |

1. A run failure of an `as` is at its site when the site is on its `As` node.
2. An `is` is never a site.
3. **An environment that rejects `value`** makes each such site a `may-fail`
   diagnostic. Strict mode, which rejects `constant-condition` and `dynamic`,
   does not reject `value`: an `as` of untyped data is accepted there, and a
   `dynamic` use of the same data is not. A host that rejects `value` has its
   authors use `is`, or put the `as` in the body of a `try`, whose sites are
   dropped.

**The class.** `05-types.md` 18.5 and `03-host.md` 3.7.6 already give the class
of `not-conforming`: `value` at a `value` site, `environment` elsewhere in a
checked program, and `value` in a program compiled without a check. So a
failed `as` whose subject surely fits T, a node with no site, is `environment`:
a host value broke its declared type.

**The promise** of `05-types.md` 18.4 holds with this package.

1. Promise 2: an `as` fails only at its `value` site, or where its subject
   surely fits T, which the precondition rules out. An `is` does not fail.
2. Promise 4: `x as T` gives a value that conforms to T, because the run tests
   it.
3. The facts are sound. {p: T} holds because the run tested it, whatever the
   precondition. {p: not T} removes only alternatives that surely fit T: a
   value that conforms to the type of p, as the precondition says, and not to
   T, conforms to an alternative that is left. The fact {p: Join(A, B)} of
   `||` holds because a value that conforms to A or to B conforms to their
   Join. Where a host broke the
   precondition, a use in the failing branch fails with class `environment`,
   as every such failure does.
4. Types still change no result where they are inferred or declared. A
   program type is an operand of a test, and the result depends on it.

### 6.7 Check codes

| Code | Reported when | Span | Class | Type afterwards |
| --- | --- | --- | --- | --- |
| `never-conforms` | An `x as T` where disjoint(S, T) (6.3) | The `As` node | | T |
| `untagged-union` | Two records in one `PType` (6.1). A static rule | 6.1 | | `never` for `as`, `bool` for `is` |
| `undefined-name` | A type name the environment does not declare (6.1). A static rule | The name | | The same |
| `check-limit` | A program type deeper than 64, or over the size limit (6.1). A static rule | The program type | | The same |
| `always-truthy`, `always-falsy` | An `x is T` that the types decide (6.3) | The `As` node | `constant-condition` | `bool` |

With this package the catalog's check codes that an implementation can report
are 29: the 27 of `05-types.md` 16.1, `never-conforms` and `untagged-union`.

## 7. The conformance function

### 7.1 The function

Every binding gives the host the test that `as` and `is` make.

| Matter | Rule |
| --- | --- |
| Takes | A value and a type. The value is anything a host may supply (`03-host.md` 3.4) or a run may return. The type is resolved by an environment (3.5), which gives its names |
| Gives | Nothing when the value conforms. Otherwise one diagnostic: phase `run`, code `not-conforming`, class `value`, the path and message of 4.4, and no span |
| Agrees with | `x is T` for every value and type, and `x as T` with its path and message |
| Changes | Nothing. The value is not copied, filled in or trimmed |

1. **It is not a run.** It has no program, no usage, no limits and no
   cancellation signal. It charges nothing.
2. **Its work** is bounded by the value: it visits a value at most once for
   each way the type reaches it. **Its depth** is bounded by the type, at most
   64, whatever the data. Cyclic host data cannot make it loop.
3. A host that wants limits and a count runs the program `v as T`, with `v`
   declared `any`.
4. Type text given to it is resolved as declared type text (`05-types.md`
   19.5): an error is the `declare` error `bad-type`.
5. **It is the precondition of `05-types.md` 18.4, made testable.** A host
   tests a variable or a function's result against its declared type with the
   rule the checker assumed.
6. The function that reports every failure, `ConformAll`, is package F8.

### 7.2 Binding forms (binding rules)

| Binding | Form |
| --- | --- |
| Go | `t, err := env.ParseType(text)`, then `risor.Conform(v, t)`, which returns `nil` or an `error` that holds the diagnostic |
| TypeScript | `env.conform(value, typeText)`, which returns `null` or the diagnostic |
| Rust | `env.ty(text)?.conform(&value)`, which returns `Ok(())` or `Err(diagnostic)` |

## 8. The description

F1 5.3 gains one key.

| Key | Value |
| --- | --- |
| `types` | An object: each named type's name, and its type printed by `05-types.md` 3.9. Present only when the environment declares at least one named type |

1. A type prints by its structure, so the text of a named type holds no name
   but opaque ones, and each reads back alone.
2. The types of variables and signatures are printed by structure too: a name
   used in them is replaced.
3. An environment built from a description (F1 5.4) declares each entry of
   `types` as a named type by type text. Their order does not matter, since no
   text names another.
4. A description with no named type has no key `types`, so every description
   of an environment with no named type is as F1 gives it.

## 9. Additions to the case format

Each addition is optional, and its absence leaves every existing case file
valid with its meaning unchanged.

### 9.1 The case field `types`

This row is added to the table of `03-host.md` 3.10.3.

| Field | Holds | When absent |
| --- | --- | --- |
| `types` | A list of named types, in the order declared (3.1). Each is an object: `{"name": N, "type": text}`, or `{"name": N, "schema": S}` with the optional field `options`, an array of the option names of F1 4 | No named type is declared |

1. The declarations join those of `03-host.md` 3.10.3 item 3. A named type may
   use the case's `opaque` names.
2. The errors of 3.2 are `declare` failures of the case, not invalid files.
3. `types` may stand in the header. A case's `types` replaces the header's
   whole.
4. A `schema` is not a host value: a JSON object with the key `$opaque` in it
   is data of the schema. Its numbers are read by `03-host.md` 3.10.3 item 5.

These make the file invalid: `types` that is not an array; an entry that is not
an object; an entry with no `name`, or a `name` that is not a string; an entry
with both `type` and `schema`, or with neither; `type` that is not a string;
`options` beside `type`, or `options` that is not an array of the three names
with none twice; a field other than `name`, `type`, `schema` and `options`.

### 9.2 The case of the second kind: `conform`

A case with the field `conform` has no program. It gives a type and a value,
and expects an answer.

| Field | Holds |
| --- | --- |
| `conform` | An object with exactly two fields: `type`, a type text, and `value`, a host value as `03-host.md` 3.10.3 items 4 and 7 read one, opaque values included |

1. `source`, `file` and `conform` count as one field for the header rule
   (`03-host.md` 3.10.2 item 4). After the header rule a case has exactly one
   of them.
2. A case with `conform` may have, after the header rule, only `name`,
   `version`, `opaque`, `types`, `conform`, `skip` and `expect`. Any other field
   makes the file invalid.
3. Its `expect` is a **conform result**, or a failure of phase `declare`.

| Field of a conform result | Holds | Compared with |
| --- | --- | --- |
| `conforms` | `true` or `false`. Required | The answer of each of the three ways of 9.3 |
| `path` | The path of the failure | The path of the conformance function's diagnostic and of the run of `v as T`. Equal, step by step |
| `message` | The message | The same two. Equal, code point by code point |
| `steps` | The steps | The usage of the runs of `v as T` and `v is T`. Equal |

4. `path` and `message` stand only beside `conforms: false`. Either may be left
   out, and is then not compared.
5. A conform result in a case without `conform`, or an `expect` of a case with
   `conform` that is neither a conform result nor a failure of phase
   `declare`, makes the file invalid.
6. The differences of a conform case are named `outcome`, `conforms`, `path`,
   `message` and `steps`, in that order.

### 9.3 How a conform case is run

| Step | What happens | A failure here gives |
| --- | --- | --- |
| 1. Declare | The case's environment is built: `version`, `opaque`, `types` | The outcome: a failure of phase `declare` |
| 2. Resolve | `conform.type` is resolved in it as declared type text (7.1 item 4) | The outcome: a failure of phase `declare`, `bad-type` |
| 3. The function | The conformance function (7) on the value, supplied as a host value | |
| 4. Two runs | The programs `v as ` followed by the text of `conform.type`, and `v is ` followed by it, each compiled in a child of the case's environment that declares the variable `v` of type `any`, with the static rules alone, and each run with `v` supplied as the value, under the base environment's limits | |
| 5. Compare | Each of the three answers is compared with `expect` | The verdict `fail` |

1. The three agree: the function gives nothing, `v is T` gives `true` and `v as
   T` gives a value exactly when the value conforms. When they disagree, the
   verdict is `fail` with the difference `conforms`.
2. The run of `v as T` that passes gives a value `==` to the value supplied.
   Another value is the difference `conforms`.
3. The runs charge no unit. A conform result has no `allocated`.
4. A case with a `declare` failure stops at the step that failed.

### 9.4 The runner

1. A runner of an implementation that offers F4 runs `suite/conversion/`. A
   case there that states `type` or `sites`, or expects a `check` code that is
   not static by 6.1 item 4, is run once, checked (`03-host.md` 3.10.8 item
   5). Every other case is run checked and with the static rules alone, as a
   case of `suite/core-run/` is, a conform case by 9.3.
2. A runner of an implementation that does not offer F4 does not run
   `suite/conversion/`. For it the fields `types` and `conform` are unknown
   fields (`03-host.md` 3.10.10).
3. The cases that declare a named type by schema, and those that expect a
   description, are in `suite/conversion/conversion-schema-types.json`.
4. The analysis cases of F2 4.2 for programs that hold a test are in
   `suite/conversion/conversion-analysis.json`, run as F2 4.7 runs them.
5. The description of 8 is compared by the result field `description` of F1
   7.2.

### 9.5 The `outcome` command

The line of the `outcome` command (`suite/README.md`) changes so:

| Case | Line |
| --- | --- |
| A case with `types`, whose environment was built | The field `description` of F1 7.4 |
| A conform case | `name`; then `phase`, null, or `"declare"` with `diagnostics` as for any `declare` failure; then, when `phase` is null, `conforms`, `path` and `message` from the conformance function (absent when it conforms), `steps` from the run of `v as T`, and `agree`, `true` when the three ways of 9.3 agree in answer, path, message and steps |

### 9.6 The JSON Schema of the additions

These are added to the schema of `03-host.md` 3.10.11: the two properties below
in `$defs/fields`, `"types"` and `"conform"` in both lists of `propertyNames`,
`$defs/conformResult` as a third branch of `$defs/expect`, and `types` in the
`description` of F1 7.5.

```json
{
  "fields": {"properties": {
    "types": {
      "type": "array",
      "items": {
        "type": "object",
        "required": ["name"],
        "properties": {
          "name": {"type": "string"},
          "type": {"type": "string"},
          "schema": true,
          "options": {
            "type": "array",
            "uniqueItems": true,
            "items": {"enum": ["closed", "absent-is-null", "defaults-applied"]}}},
        "oneOf": [
          {"required": ["type"], "not": {"anyOf": [
            {"required": ["schema"]}, {"required": ["options"]}]}},
          {"required": ["schema"], "not": {"required": ["type"]}}],
        "additionalProperties": false}},
    "conform": {
      "type": "object",
      "required": ["type", "value"],
      "properties": {
        "type": {"type": "string"},
        "value": {"$ref": "#/$defs/hostValue"}},
      "additionalProperties": false}}},
  "conformResult": {
    "type": "object",
    "required": ["conforms"],
    "properties": {
      "conforms": {"type": "boolean"},
      "path": {"$ref": "#/$defs/diagnostic/properties/path"},
      "message": {"type": "string"},
      "steps": {"$ref": "#/$defs/count"}},
    "if": {"properties": {"conforms": {"const": true}}},
    "then": {"not": {"anyOf": [{"required": ["path"]}, {"required": ["message"]}]}},
    "additionalProperties": false},
  "description": {"properties": {
    "types": {"type": "object", "additionalProperties": {"type": "string"}}}}
}
```

### 9.7 Example

```json
{
  "env": "{body: any}",
  "types": [{"name": "Line", "type": "{sku: string, qty: int}"},
            {"name": "Order", "type": "{id: string, lines: Line[], ...}"}],
  "cases": [
    {"name": "a valid order", "source": "(body as Order).lines[0].qty",
     "vars": {"body": {"id": "a", "lines": [{"sku": "x", "qty": 2}]}},
     "expect": {"value": 2, "steps": 9}},
    {"name": "the first failure is in the second line",
     "conform": {"type": "Order",
                 "value": {"id": "a", "note": 1,
                           "lines": [{"sku": "x", "qty": 1}, {"sku": "y", "qty": "2"}]}},
     "expect": {"conforms": false, "path": ["lines", 1, "qty"],
                "message": "expected int, found \"2\"", "steps": 8}}
  ]
}
```

1. The first case charges 6 steps for the walk: the map, the lookups of `id`
   and `lines` (the second is also the step of the list), the line, and the
   lookups of `qty` and `sku`. `note` is under a rest of `any`. Then `.lines`,
   `[0]` and `.qty` charge 1 each: 9.
2. The second stops at `qty` of the second line, which sorts before `sku`, and
   never reaches `note`: the map, `id`, `lines`, 3 for the first line, then the
   second line and its `qty`, 8.

## 10. Core cases this package replaces

Core pinned these texts as `parse` errors in
`suite/core-run/parse-follow-on.json`. When this package is accepted they are
removed from that file (rulings BA4). The file
`suite/conversion/conversion-core-texts.json` holds each text with the outcome
of this chapter.

| Core case | Text | Core outcome | With this package |
| --- | --- | --- | --- |
| x as T | `x as int` | `unexpected-token` `[1, 3, 1, 5]` | `undefined-name` `[1, 1, 1, 2]`, phase `check`: no `x` is declared |
| x is T | `x is string` | `unexpected-token` `[1, 3, 1, 5]` | `undefined-name` `[1, 1, 1, 2]` |
| a number as T | `1 as int` | `unexpected-token` `[1, 3, 1, 5]` | The value `1`, with 1 step |
| a parenthesized operand as T | `(x) as T` | `unexpected-token` `[1, 5, 1, 7]` | `bad-type-text` `[1, 8, 1, 9]`, the one-letter `T` |
| a field read is T | `x.a is T` | `unexpected-token` `[1, 5, 1, 7]` | `bad-type-text` `[1, 8, 1, 9]` |
| T1: + then !x as T fails at the as | `a + !x as T` | `unexpected-token` `[1, 8, 1, 10]` | `bad-type-text` `[1, 11, 1, 12]` |
| T1: < then !x as T fails at the as | `a < !x as T` | `unexpected-token` `[1, 8, 1, 10]` | `bad-type-text` `[1, 11, 1, 12]` |

1. These Core cases keep their outcome, and are not replaced: `a == !x is T`,
   `a && !x as T`, `!x as T` and `!x is T` (`unexpected-token` at the `!`, by
   2.5); `x as const` and `x is not T` (`unexpected-token`, by 2.6 item 2);
   `!x as )` in `parse-first-diagnostic.json` (at the `!`, found at the `as`);
   and the cases of `parse-reserved.json` that use `as` or `is` as a name or
   an operand (`reserved-word`).
2. No valid Core program changes its meaning. This package gives a meaning only
   to text in which `as` or `is` follows an operand, which Core always refuses.

## 11. Examples

The environment declares `body: any`, `n: int`, `v: int | string`, `t: Time`
with the opaque type `Time`, and the named type `Order` = `{id: string, total:
float, ...}`.

| # | Text | Value or code |
| --- | --- | --- |
| 1 | `body as {id: string, ...}`, `body` `{"id": "a", "x": 1}` | the same map; 2 steps: the map and `id`, and nothing for `x` |
| 2 | `body as {id: string}`, the same `body` | `not-conforming`, path `["x"]`, `the type does not allow key "x"`; 3 steps |
| 3 | `body is any[]`, `body` `[1, "a"]` | `true`; 1 step |
| 4 | `body as int`, `body` `3.0` | `3`; 1 step |
| 5 | `body as int`, `body` `1.5` | `not-conforming`, `expected int, found 1.5`; 1 step |
| 6 | `body as {id: string, note?: string}`, `body` `{"id": "a", "note": null}` | `not-conforming`, path `["note"]`, `expected string, found null`; 3 steps |
| 7 | `try { body as Order } catch (err) { err.path }`, `body` `{"id": 1}` | `["id"]` |
| 8 | `body is Order ? body.total : 0` | `float`, checked, no site |
| 9 | `(body as Order).total` | `float`, a `value` site on `body as Order` |
| 10 | `n as string` | `never-conforms` on the whole text |
| 11 | `n is float` | `always-truthy` on the whole text |
| 12 | `v is int ? v + 1 : v.trim()` | `int \| string`, no site |
| 13 | `match body as "bug" \| "feature" { "bug" => 1, "feature" => 2 }` | `int`; the `match` covers its subject |
| 14 | `t is Time` | `always-truthy` |
| 15 | `body as Item` | `undefined-name` on `Item` |
| 16 | `body as {a: int} \| {b: int}` | `untagged-union` on `{a: int} \| {b: int}` |
| 17 | `body as T` | `bad-type-text` on `T` |
| 18 | `x as int + 1` | `unexpected-token` on `+` |
| 19 | `!body is int` | `unexpected-token` on `!` |
