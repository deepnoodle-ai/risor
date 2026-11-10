# F7. Queries

This chapter is normative. It specifies follow-on package F7: the projection
step `[*]` and `?.[*]`, with the node `Each`; the list methods `minBy`, `maxBy`,
`uniqueBy`, `indexBy` and `countBy`; the map method `mapValues`; and the
direction argument of `sortBy`. It adds node 32, six library entries, one
optional parameter of a Core entry, the file `suite/library-queries.json`, the
star step of an analysis path, and three message templates. It adds no value,
no type, no run code, no check code, no reserved word and no case field.

The package only adds. In Core a projection is a parse error, the six names are
methods no kind has, and `sortBy` takes one argument (`01-language.md` 14.3,
`04-library.md` 5.4.13). Section 13 lists the Core cases that pinned these
texts as errors, and the cases of this package that replace them.

Other chapters: `01-language.md` (chains, nodes, spans, levels),
`02-evaluation.md` (order, steps, units, the depth count), `03-host.md`
(diagnostics, the catalog, case files), `04-library.md` (the record format, how
an entry is specified, the cost rules), `05-types.md` (chains, calls, facts,
sites), `spec/packages/f02-templates-analysis.md` (analysis paths),
`spec/packages/f04-conversion.md` (facts of `is`) and
`spec/packages/f05-unions.md` (tagged unions).

## 1. Terms and scope

### 1.1 Terms

| Term | Meaning |
| --- | --- |
| Star | The step `[*]` or `?.[*]` of a chain. An optional star is `?.[*]` |
| Read | The step `.w`, `?.w`, `[e]` or `?.[e]` of a chain |
| Projection | A star, and every read and star that follow it directly in the same chain, up to the first method call or the end of the chain |
| The first star | The star that begins a projection. A **later star** is any other star of it |
| Level | A star of a projection with the reads that follow it, up to the next star of the projection or the end of the projection. A projection with k stars has k levels, numbered from 1 |
| Element in hand | At level i, the element of the list of level i that the reads of level i are applied to |
| Step span | Of a step of a projection: from the first token of the chain to the last token of the step (2.4) |
| Star span | The step span of a star |

### 1.2 What this package changes in Core's rules

| Core rule | With this package |
| --- | --- |
| `01-language.md` 3.1, `Chain` | A star is a step of a chain (2.1) |
| `01-language.md` 4.1, node 32 | The node `Each` (2.3) |
| `01-language.md` 4.2, `OptChain` | Wraps a chain whose optional step stands outside a projection's reads (2.3) |
| `01-language.md` 13.1, levels | A star opens a level to the end of its projection (2.5) |
| `01-language.md` 14.3, the row of a projection | Replaced: a projection is a program (2.2) |
| `02-evaluation.md` 2.3, 5.2, 6.1, 8.4 | The evaluation of a projection, its rows of cost, and its place in the depth count (3, 4) |
| `03-host.md` 3.8.1, the catalog | A second catalog file, `suite/messages-queries.json`, with three templates: `star` of `null-operand` and of `wrong-kind`, `order` of `wrong-kind` (11). Core's catalog is unchanged |
| `04-library.md` 1.1, 1.3, 3.3, 3.6, 4.2, 4.4, 4.5 | Six entries, a direction for `sortBy`, a second file of records with the field `extends`, the test of a parameter whose type is a literal set (6) |
| `05-types.md` 6.3, 9, 16.2, 18.3, 22.4 | The check of a projection (5) |
| `05-types.md` 13.8 | Nothing: with F5, `f05-unions.md` 5.7 narrows `filter` and `find` by a tag; section 8 gives the cases with projections |
| `f02-templates-analysis.md` 3.5, 4.3, 4.6 | The star step of a path and its JSON form (9) |

### 1.3 Version

1. The projection and the entries of this package are part of version `3.0`.
   Every record has `since` `3.0`. This package adds no version number and no
   version marker.
2. With this package an implementation offers 59 library entries: the 53 of
   Core and the 6 that this package adds. The list methods are 27 and the map
   methods 4. With F6 it offers 67, and with F13 78.

### 1.4 Packages this one requires

Packages are offered in a fixed order, each only with every package before it:
F3, F1, F2, F4, F5, F6, F7, and then F8 to F13. So an implementation that offers
F7 offers F2, F4 and F5, and the cases of this package may use their forms and
their case fields: the result field `analysis` of F2 (section 9), `as` of F4,
and the tagged unions of F5 (section 8).

## 2. Syntax

### 2.1 Grammar

The production `Chain` of `01-language.md` 3.1 becomes:

```ebnf
Chain   = Primary { "." Word [ Args ] | "?." Word [ Args ]
                  | "[" Expr "]" | "?.[" Expr "]" | Star } .
Star    = ( "[" | "?.[" ) "*" "]" .
```

1. The parser chooses this way: a `[` or `?.[` of a chain whose next token is
   `*` begins a `Star`. Otherwise it begins an index, as in Core.
2. `*` and `]` are separate tokens. White space and comments may stand between
   them as between any two tokens: `xs[ * ]` and `xs?.[/* all */*]` are stars.
3. `**` is one token (`01-language.md` 2.3 rule 2), so `xs[**]` is not a star.
4. A star is a step of a chain and nothing else. No expression starts with
   one.

### 2.2 Where a star stands, and text that stays an error

1. A star may follow any primary and any step of a chain: a name, a literal, a
   call, a parenthesized expression, a block, a `match`, a read, a method call
   or another star.
2. These are errors, each `unexpected-token` at the span given, unless a lexical
   diagnostic comes first (`01-language.md` 2.11).

| Text | Span |
| --- | --- |
| A `[` or `?.[` and `*` not directly followed by `]`: `xs[*1]`, `xs[* + 1]`, `xs[*,]` | The token after the `*` |
| The same at the end of input: `xs[*` | The empty span at the end of input |
| A star where no chain is: `[*]`, `f(*)`, `[*].a` | The `*` |
| `xs.*.f`, `xs[].f`, `xs[?c]` (rows of `01-language.md` 14.3, for F8) | As 14.3 gives |
| `?.` followed by `[` with white space: `xs?. [*]` | The `[` |
| A star after an `if` or a `try` (`01-language.md` 3.4 rule 4) | The `[` or `?.[` |

3. The rows of `01-language.md` 14.3 for `xs[*]` and `xs?.[*]` do not apply. A
   projection is a program.

### 2.3 The node `Each`

| # | Node | Fields | Written |
| --- | --- | --- | --- |
| 32 | `Each` | `x` (or none), `reads`, `optional` | `xs[*].f`, `xs?.[*].f` |

1. **A projection is one `Each`.** Its `x` is the node of the chain before the
   first star. `optional` is true when the first star is `?.[*]`.
2. **`reads`** is the list of the steps after the first star, in order, up to
   the next star or the end of the projection. Each is a `Member` or an `Index`
   with no `x`: it is applied to the element in hand.
3. **A later star** is an `Each` with no `x`, the last item of the `reads` of
   the level before it. Its `optional` says whether it is `?.[*]`, and its own
   `reads` hold the steps of its level.
4. So `orders[*].lines[*].sku` is an `Each` with `x` the `Ident` `orders`,
   whose `reads` are the `Member` `.lines` and an `Each` with no `x` whose one
   read is the `Member` `.sku`.
5. **After the projection.** The first method call after a projection has the
   outer `Each` as its `x`, and the chain goes on as in Core. A star after a
   method call begins a new projection, whose `x` is that method call.
6. **`OptChain`.** A chain is wrapped in one `OptChain` (`01-language.md` 4.2)
   when it has a `?.`, a `?.[` or a `?.[*]` step outside the reads of every
   projection: a first star `?.[*]` among them. A `?.` or `?.[` among a
   projection's reads, and a later star `?.[*]`, do not wrap the chain. So
   `xs[*]?.a` is an `Each` with no `OptChain`, and `xs?.[*].a` is an `OptChain`
   around an `Each`.
7. Core has 30 node kinds. With F4 and this package there are 32.

### 2.4 Spans

1. The **step span** of a step of a projection runs from the first token of
   the chain to the last token of the step: the word of `.w` and `?.w`, and the
   `]` of `[e]`, `?.[e]` and a star. The first token of the chain is that of
   its primary, with the parentheses around the primary (`01-language.md` 4.3
   item 1).
2. A `Member` or `Index` among the `reads` has its step span. So does the
   "receiver" of a step (`05-types.md` 9.1): the receiver of a step of a
   projection is the chain from its start up to that step, and its span is the
   step span of the step before it, or the span of `x` for the first star.
   The span of `x` is that of its node, so it does not hold parentheses around
   it, as for any receiver in Core: in `(n)[*].f`, a `kind` on the first
   star's receiver spans `n`, the star span is `(n)[*]`, and the receiver of
   `.f` spans `(n)[*]`.
3. An `Each` spans from the first token of the chain to the last token of its
   projection. An `Each` with no `x` has the same start. Its star span is the
   step span of its own star.

| Text | Node or step | Span |
| --- | --- | --- |
| `orders[*].lines[*].sku` | The outer `Each`, and the inner `Each` | `[1, 1, 1, 23]` |
| | The first star | `[1, 1, 1, 10]` |
| | `.lines` | `[1, 1, 1, 16]` |
| | The later star | `[1, 1, 1, 19]` |
| | `.sku` | `[1, 1, 1, 23]` |
| `(xs)[*].a` | The `Each`, and `.a` | `[1, 1, 1, 10]` |
| `xs[*].a.sum()` | The `MethodCall`, and the `Each` | `[1, 1, 1, 14]`; `[1, 1, 1, 8]` |

### 2.5 Levels of nesting

This row joins the table of `01-language.md` 13.1.

| Opens a level | The level lasts |
| --- | --- |
| The `[` or `?.[` of a star | to the last token of its projection. The `*` and the `]` of the star are inside it, and it does not close at the `]` |

1. So the steps after one star are one level inside it, and those after a
   second star two. A projection with k stars holds k levels at its end.
2. The `[` of an index read in a projection opens its own level, to its `]`,
   inside the levels of the stars before it.
3. The levels of a projection close where the projection ends: before the `.`
   or `?.` of the method call that ends it, or at the end of the chain. A
   method call's argument list after a projection is one level, as in Core.
4. The token that would open level 101 is `nesting-exceeded` (`01-language.md`
   13.2), a star's `[` or `?.[` among them. So a projection may have 100 stars
   at the top of a program, and the `[` of the 101st is `nesting-exceeded`.

| Text | Levels open at `x` |
| --- | --- |
| `xs[*].a[x]` | 2: the star, and the `[` of the index |
| `xs[*][*].a[x]` | 3 |
| `f(xs[*].x)` | 2: the `(`, and the star |
| `xs[*].a.m(x)` | 1: the `(`. The projection ended before `.m` |
| `xs[*].a + x` | 1: the `+` |

## 3. Evaluation

### 3.1 The receiver

1. The chain before the first star is evaluated as in Core. Its value is the
   receiver of the first star.
2. A null receiver under `[*]` fails with `null-operand`, template `star`. A
   null receiver under `?.[*]` makes the whole chain null, as an optional step
   does in Core (`01-language.md` 7.4 rule 2): no later step runs, and nothing
   more is evaluated or charged.
3. A receiver that is not null and not a list fails with `wrong-kind`,
   template `star`, under `[*]` and under `?.[*]` alike. A map, a string, a
   number, a bool and an opaque value each fail. Nothing is treated as a list
   of one, and a map's values are not visited: write `m.values()[*]`.
4. The test is made before anything is charged for the star.

### 3.2 Levels and elements

A projection is evaluated level by level, from level 1.

1. **The elements of a level** are those of its star's receiver, visited in
   order by index, from 0.
2. **For each element in hand,** in order: 1 step is charged for the visit
   (4.1). Then the reads of the level are applied to the element in hand, left
   to right, as a chain of their own whose primary is the element in hand
   (3.3). Then:
   - at the last level, the value the reads gave is appended to the result;
   - at any other level, that value is the receiver of the next star, which is
     tested by 3.1 items 2 and 3 with one difference: a null receiver under a
     later `?.[*]` appends one null to the result and ends the work for the
     element in hand. Otherwise the next level visits the elements of that
     receiver.
3. The elements of a level are all visited before the work of the level
   before it goes on to its next element. So the result is in the order of a
   walk depth first: the values for the first element of level 1, in their
   order, then those for its second element, and so on.
4. **One flat list.** Every value appended at the last level, and every null of
   3.3 item 2 and of item 2 above, goes into one list, the value of the
   projection. A list that no star iterated is never flattened:
   `orders[*].lines` is a list of lists.
5. A star with no read after it gives a list of the elements it visits:
   `xs[*]` is a new list with the elements of `xs`, and `xs[*][*]` is the list
   of the elements of the elements of `xs`, each of which must be a list.
6. An empty receiver gives nothing for its element, and an empty receiver of
   the first star gives the empty list.

### 3.3 Reads in a projection

1. **A read is the read of Core** (`01-language.md` 7.1; `02-evaluation.md`
   2.3), applied to the value of the steps before it within the level, or to
   the element in hand for the first read of a level. Its receiver is tested
   first, then its index expression is evaluated and tested, then it charges
   and reads. An index expression is evaluated again for each element it is
   applied to. It binds no name and sees the names in scope where the chain
   stands.
2. **An optional step whose receiver is null.** When the receiver of a `?.w` or
   `?.[e]` among the reads is null, the rest of the projection's steps are
   skipped for the element in hand: no later read or star runs for it, and no
   index expression of theirs is evaluated or charged. One null is appended to
   the result. The other elements go on. Outside the projection nothing is
   skipped.
3. **A read that finds nothing.** `.w` and `[k]` on a map without the key, and
   `[i]` out of range, fail as in Core. `?.w` and `?.[e]` that find nothing give
   null for that step, and the reads go on (`01-language.md` 7.4 rule 3): in
   `xs[*]?.a.b`, an element without `a` fails on `.b`, and a null element gives
   one null.
4. A plain read on a null receiver fails with `null-operand`, and a read on a
   receiver of a kind it does not accept fails with `wrong-kind`, as in Core.

### 3.4 What follows a projection

1. The projection ends at the first method call after it or at the end of the
   chain. The method call receives the projection's list, and the chain goes on
   as in Core. So `orders[*].lines[*].qty.sum()` sums one list.
2. A method of the element is not reached: `users[*].email.toLowerCase()` calls
   `toLowerCase` on a list. Without a checker it fails with `wrong-kind`,
   template `method`. Write `users.map(u => u.email.toLowerCase())`.
3. A read cannot follow a projection in its chain: a read there is a step of
   the projection. To read the result, end the chain: `(xs[*].a)[0]` and
   `xs[*].a.first()`.
4. A star after a method call begins a new projection:
   `xs.filter(x => x.ok)[*].id`.

### 3.5 Failures

1. The first failure in the order of evaluation ends the projection, as it ends
   any expression. No partial list is given, and no unit is charged for the
   result.
2. Each failure has a run code of Core and a template of a catalog file
   (section 11): `suite/messages.json` or `suite/messages-queries.json`. Its
   `path` is null.

| Failure | Code and template | Span |
| --- | --- | --- |
| A star's receiver is null, under `[*]` | `null-operand`, `star` | The star span |
| A star's receiver is not null and not a list | `wrong-kind`, `star` | The star span |
| A read fails | The code and template of Core for the read (`01-language.md` 14.5) | The read's step span |
| An index expression fails | Its own failure | Its own span |

3. Message parameters are those of Core for a read. `wrong-kind` `star` has
   `kind`, the kind of the receiver. `null-operand` `star` has none.
4. The class of a failure is that of `05-types.md` 18.5, with the sites of
   section 5.

### 3.6 Host values

1. An element a star visits is read (`02-evaluation.md` 3.3) when a read or a
   later star is applied to it: it is then yielded as the receiver of that step.
   An element appended to the result with no step after its star is copied, as
   a spread copies, and is not read.
2. The value a read gives is read, as in Core.

### 3.7 The depth count

1. A projection is not a call. A star in progress adds nothing to the depth
   count (`02-evaluation.md` 8.4).
2. A call inside a read of a projection, in an index expression, has a nesting
   depth that counts the levels of the stars around it (2.5). So with `g` a
   `const`-bound lambda, `g(0)` in `xs[*].a[g(0)]` stands at nesting depth 2,
   and the depth count while it runs is 3.
3. An evaluator may recurse once for each star. 2.5 bounds the stars at 100.

### 3.8 A projection and `map`

1. **Values agree.** For the same values, `x[*]R`, where R is a sequence of
   reads, gives the value of `x.map(e => eR)`, with `e` a name that occurs
   nowhere else. A failure of a read has the same code and message in both.
2. A second star is `flatMap`: `x[*]R1[*]R2` gives the value of
   `x.flatMap(e => eR1.map(f => fR2))`, where every `?.` of R1 and R2 finds no
   null receiver. A `?.` that skips gives one null where the `flatMap` form
   would give nothing or fail.
3. **They differ** in these, and in nothing else:

| Matter | `x[*]R` | `x.map(e => eR)` |
| --- | --- | --- |
| Steps | 1 for each element visited, and the reads | 1 for the lambda, 1 for the call, 2 for each element visited, and the reads. So 2 + v more, where v is the number of elements visited |
| Units | 1 + n | 1 + n, the same |
| A failure of a read | At the read's step span | In the lambda's body, at the read's span there |
| A failure of the receiver | Template `star`, at the star span | The test of a method call (`null-operand` `method`, or `wrong-kind` `method` for a map or a string), at the `MethodCall` |
| The depth count | No call | Each call of the lambda is a call in progress |
| Types and sites | Section 5 | `05-types.md` 13 |

## 4. Cost

### 4.1 Rows

These rows join the tables of `02-evaluation.md` 5.2 and 6.1.

| Form | Steps | When |
| --- | --- | --- |
| The receiver of the first star | What its chain charges, once | Before the star |
| The test of a star's receiver | 0. A failure of the test charges nothing | Before the first visit of the star |
| A first star `?.[*]` whose receiver is null | 0. The rest of the chain is not charged | |
| Each element a star visits | 1 | Before the reads applied to that element |
| A read applied to the element in hand, or to the value of the reads before it | The row of the read in `02-evaluation.md` 5.2, with its index expression | At its turn, for each element |
| A `?.` or `?.[` among the reads whose receiver is null, and a later `?.[*]` whose receiver is null | 0. The steps skipped for the element in hand are not charged | |
| The result | 0 | |

| Value created | Units |
| --- | --- |
| The list a projection gives | 1 + `k`, where `k` is the number of values in it, the nulls of 3.2 and 3.3 included. Charged once, after the last step of the last element has run and before the list is built (`02-evaluation.md` 6.2 item 3) |
| A projection that fails | 0. Nothing is created for its result |

1. No list between the levels is charged, because none is created: the values
   of a later level go into the one result. Lists that the reads read already
   exist.
2. The scratch space that holds the values until the result is built is not a
   value and is not charged (`02-evaluation.md` 6.2 item 6). It is at most a
   constant times the steps charged.
3. A projection is not a walk over a value. It counts nothing against the
   depth limit (3.7).
4. **The work bound** (`02-evaluation.md` 8.5) holds: every element visited is
   charged 1, and each read charges by its row.

### 4.2 Worked counts

The names other than `const` names are variables, supplied by the host. Host
data costs no units. Each count is the usage of the whole run.

| # | Program and variables | Steps | Units |
| --- | --- | --- | --- |
| Q1 | `xs[*].a`, with `xs` `[{a: 1}, {a: 2}, {a: 3}]` | 6: 3 visits, 3 reads | 4: a list of 3 |
| Q2 | `xs.map(x => x.a)`, the same | 11: the lambda 1, the call 1, `2n` = 6, three reads | 4 |
| Q3 | `xs[*]`, with `xs` `[1, 2, 3]` | 3: 3 visits | 4 |
| Q4 | `orders[*].lines[*].sku`, with 3 orders of 2, 0 and 4 lines | 18: 3 visits, 3 reads of `lines`, 6 visits, 6 reads of `sku` | 7: a list of 6 |
| Q5 | `orders[*].lines?.[*].sku`, with 2 lines, `null` and 4 lines | 18: the same visits and reads. The second order's `lines` is read, is null, and gives one null | 8: a list of 7 |
| Q6 | `xs?.[*].a`, with `xs` null | 0 | 0 |
| Q7 | `xs[*].a.sum()`, with `xs` as in Q1 | 10: the projection 6, the call 1, `sum`'s `n` = 3 | 4 |
| Q8 | `xs[*].a`, with `xs` `[{a: 1}, {}]` | 4: 2 visits, 2 reads, then `missing-field` at the second | 0 |
| Q9 | `orders[*].lines[*].sku` written `orders.flatMap(o => o.lines).map(l => l.sku)`, as Q4 | 37 | 14 |

## 5. Types

### 5.1 The walk of a projection

The checker walks a chain by `05-types.md` 9.1, with *cur* and *skipped*. A
projection adds a second flag, *skipped-element*, and a third, *failed*, both
false at the first star.

| At | Rule |
| --- | --- |
| The first star | Steps 2 to 4 of 9.1 apply to *cur*, the receiver's type: before `[*]`, a receiver that may be null is `maybe-null` on the receiver, and *cur* becomes nonnull(*cur*); before `?.[*]`, *skipped* is set. Then by the kind of *cur*: |
| | `any`: a `dynamic` site on the receiver. The element type T is `any`. A `?.[*]` on `any` sets *skipped*: the receiver may be null, and the projection's type is a list. So `body?.[*]` is `any[] \| null` |
| | `never`: the projection has the type `never` (5.2). Its reads report nothing, and their index expressions are synthesized (9.4) |
| | a list `T[]`: the element type is T |
| | any other kind, a string and a record among them: `kind` on the receiver. *failed* is set, and the projection has the type `never` |
| The reads of a level | Checked by steps 2 to 5 of 9.1 and by 9.2 and 9.3, as a chain whose *cur* starts at the element type of the level, with one change: step 3, an optional step on a *cur* that has null, sets *skipped-element*, not *skipped*. A read on a receiver of type `any` records its `dynamic` site, and a plain index read records an `index` site on its `Index` node, once, however many elements it is applied to |
| A later star | As the first star, on the *cur* of the reads before it, except that `?.[*]` on a *cur* that has null, or that is `any`, sets *skipped-element*. On `never`, the element type is `never` and nothing is reported. A `kind` sets *failed*, and *cur* becomes `never` |
| A rejected `dynamic` site inside the projection | `unchecked` on its receiver, which takes the type `never` (`05-types.md` 17): *cur* becomes `never` there, and *failed* is set |
| A diagnostic of a read that gives *cur* the type `never` | `no-field`, `kind`, or `unchecked` (17) on a receiver inside the projection: *failed* is set |
| The end of the projection | The type of 5.2. Then *cur* is that type, and the chain goes on by 9.1: a method call receives it |

### 5.2 The type of a projection

| Case | Type |
| --- | --- |
| The first star's receiver is `never` after steps 2 to 4, or *failed* is set | `never` |
| Otherwise, the reads of the last level end at U | `U[]`, or `(U \| null)[]` when *skipped-element* is set |

1. With one star or several, the type is one list: a later star's elements
   take the place of its receiver's.
2. An element type that is `never` with no diagnostic, as in `[][*].a`, gives
   `never[]`.
3. The chain's *skipped*, set by a first star `?.[*]` or by an optional step
   before it, adds null to the type of the whole chain at its end, as in Core.
4. A null that an optional read gives for an absent key is part of that read's
   type (9.2), not of *skipped-element*: `users[*]?.note` with `note?: string`
   is `(string | null)[]` by the read alone.

### 5.3 Sites

1. Sites are recorded in a projection as in any chain (`05-types.md` 18.3):
   a `dynamic` site on each receiver of type `any`, at its span (2.4), and an
   `index` site on each plain `Index` among the reads, at its step span.
2. A run failure of a star's test is at a `dynamic` site when the site is on
   its receiver. A failure of a read is at a site by 18.3, with the receiver of
   2.4.
3. A projection adds no class of site.

### 5.4 Facts

A projection gives no fact, and a fact applies to none of its steps. A path
(`05-types.md` 10.3) has field steps only, so a chain with a star is not a path
from the star on. A fact about the chain before the first star applies there
as in Core: in `xs != null ? xs[*].a : []` the receiver `xs` is not null.

### 5.5 Depth and size

A projection's type is never deeper than the type of its first star's
receiver, or it is `any[]`: U is a part of the element type, and the levels of
the lists its stars iterated are removed. Its size is never larger than the
receiver's: `U[]` and `(U | null)[]` are no larger than the list that holds U.
So the limits of `05-types.md` 20.4 and 20.7 make no test at an `Each`.

### 5.6 Diagnostics

No check code is added. The codes of Core are reported in a projection at
these spans.

| Code | In a projection | Span |
| --- | --- | --- |
| `maybe-null` | A plain star or read whose receiver may be null | The receiver (2.4) |
| `kind` | A star's receiver that is not a list; a read's receiver of the wrong kind; a union at a step (9.5) | The receiver |
| `no-field`, `maybe-absent` | A read of a record (9.2) | The field's name or key, or the index expression |
| `unchecked`, `may-fail` | A rejected `dynamic` or `index` site (17) | The site |
| `no-method` | A method call after a projection, on a list type that has no such method | The method's name |

### 5.7 The static rules

The walk of `05-types.md` 22.4 visits an `Each` by the row "every other node":
`x`, then each read in order, the index expression of each `Index` among them,
and a later `Each` with its reads. A projection binds no name and is subject to
no static rule of its own.

### 5.8 Examples

The environment: `orders: {id: string, lines: {qty: int, sku: string}[],
note?: string}[]`, `services: map<{owner: string}>`, `body: any`, `users:
{email: string}[]`, `ol: {id: string, lines: {qty: int, sku: string}[] |
null}[]`, `xn: {a: int}[] | null`.

| Program | Type, or diagnostic |
| --- | --- |
| `orders[*].id` | `string[]` |
| `orders[*].lines` | `{qty: int, sku: string}[][]`: no star iterated `lines` |
| `orders[*].lines[*].sku` | `string[]` |
| `orders[*].lines[0].qty` | `int[]`, with one `index` site on `orders[*].lines[0]` |
| `orders[*].lines[*].qty.sum()` | `int` |
| `users[*].email.toLowerCase()` | `no-method` on `toLowerCase` |
| `xn?.[*].a` | `int[] \| null` |
| `xn[*].a` | `maybe-null` on `xn` |
| `ol[*].lines?.[*].sku` | `(string \| null)[]` |
| `ol[*].lines[*].sku` | `maybe-null` on `ol[*].lines` |
| `orders[*]?.note` | `(string \| null)[]` |
| `orders[*].note` | `maybe-absent` on `note` |
| `services[*].owner` | `kind` on `services` |
| `services.values()[*].owner` | `string[]` |
| `body.items[*].tags[*]` | `any[]`, with four `dynamic` sites: `body`, `body.items`, `body.items[*]` and `body.items[*].tags` |

## 6. The library

### 6.1 The records: `suite/library-queries.json`

1. `suite/library-queries.json` is a JSON array of 7 records, in this order:
   `sortBy`, `minBy`, `maxBy`, `uniqueBy`, `indexBy`, `countBy`, `mapValues`.
   It is a part of the specification table of `04-library.md` 1.3, kept in a
   file of its own. An implementation that offers F7 loads it after
   `suite/library.json`, and after `suite/library-groups.json` when it offers
   F6. One that does not offer F7 does not load it.
2. Each record has the fields of `04-library.md` 1.3, with `group` `core` and
   `since` `3.0`. Six are new entries: no record of `library.json` has the
   same name and kind of receiver.
3. **A record that extends a Core entry** has one more field, `extends`: the
   entry's name as `04-library.md` 1.1 item 3 writes it, `list.sortBy`. In an
   implementation that offers F7 it takes the place of that entry's record of
   `library.json`. These rules bind it, and a loader rejects a record that
   breaks one:
   - its `name`, its `receiver`, its `calls`, its `steps`, its `units` and its
     `null` are those of the Core record;
   - its `signature` is that of the Core record with optional parameters added
     at the end, and nothing else changed (`04-library.md` 1.2);
   - its `fails` holds every line of the Core record's, and may add lines;
   - the Core record's `cases` remain cases of the entry. A runner of an
     implementation that offers F7 runs them against the extended entry, and
     each keeps its outcome and its counts. The extended record's own `cases`
     are added to them.
4. Only `sortBy` is extended. In an environment that removes `list.sortBy`
   (`03-host.md` 3.3.1) the extended entry is removed: there is one entry of
   that name.
5. A loader applies the rules of `04-library.md` 1.3 to the files together.

### 6.2 Shared rules

These join the rules of `04-library.md` section 3 and section 4.

| Rule of `04-library.md` | Added |
| --- | --- |
| 3.3 item 3, a test of each result | `indexBy` and `countBy` test that each result of their callback is a string, when that call returns. A result that is not a string, null included, fails with `wrong-kind`, template `result` |
| 3.3 item 4, a bound on a callback's result | `minBy`, `maxBy` and `uniqueBy`, with `sortBy`: `K: ordered` is tested over all the keys after the last call |
| 3.6, the ordered sets | The keys that the callbacks of `minBy`, `maxBy` and `uniqueBy` gave |
| 3.11, key order | `mapValues` visits the receiver's entries in key order |
| 4.2, symbols | `g`: also the number of distinct keys `countBy`'s callback gave. `sz(keys)`: for `minBy`, `maxBy`, `uniqueBy`, `indexBy` and `countBy`, the keys the callback gave, one for each element; for `mapValues`, a map method, the keys of the receiver |
| 4.4, an entry with a callback | The six entries of this package, each in the order its record gives |
| 4.5 item 4, a failed test of a result or a key | Also the tests of `indexBy`, `countBy`, `minBy`, `maxBy` and `uniqueBy`: each charges nothing more |
| 4.1 item 5, the key rule | `uniqueBy`, `indexBy` and `countBy` build a map or a set by a key, and charge `sz(keys)`. `minBy` and `maxBy` compare keys, and charge `sz(keys)` |

**A parameter whose type is a literal set.** `order` of `sortBy` is the one
such parameter. `04-library.md` 2.6 item 3 gains this test, made with the other
tests of the arguments, in their order: the argument is a string of the set.
Any other value, a string outside the set, null and every other kind included,
fails with `wrong-kind`, template `order`, before the call's step. The
template's parameters are `name`, the entry's name, and `found`, the argument.

### 6.3 `sortBy`, extended

| Field | Value |
| --- | --- |
| Signature | `T[].sortBy: (f: (item: T, index: int) => K, order?: "asc" \| "desc") => T[] where K: ordered` |
| Receiver | `T[]` |
| Extends | `list.sortBy` |
| Steps | `2*n + L*(n + sz(keys))` |
| Units | `1 + n` |
| Null | Never |
| Fails | `wrong-kind`, template `unordered`: the keys are not all numbers or all strings; `wrong-kind`, template `order`: `order` is not `"asc"` or `"desc"` |
| Order | Tests, `order` among them; for each element: 2 steps, then `f`. Then the ordered test over the keys; steps `L*(n + sz(keys))`; units; sort. |

1. Without `order`, and with `"asc"`, the entry is that of `04-library.md`
   5.4.13: the elements in ascending order of their keys, by a stable sort.
2. With `"desc"`, the elements are in descending order of their keys, by a
   stable sort: elements with equal keys keep their order in the receiver. So
   `xs.sortBy(f, "desc")` is not `xs.sortBy(f).reverse()`, which turns equal
   elements round.
3. The steps and units are the same in both directions. `order` is evaluated
   with the other arguments, before the call's step, and costs what its
   expression costs.
4. Two keys are two sorts, the minor key first, each with its direction:
   `orders.sortBy(o => o.placed).sortBy(o => o.customer, "desc")`.

| Program | Data | Result |
| --- | --- | --- |
| `xs.sortBy(s => len(s), "desc")` | `{xs: string[]}`, xs = `["bb", "a", "cc", "d"]` | `["bb", "cc", "a", "d"]`; 22 steps, 5 units |
| `xs.sortBy(s => len(s), "asc")` | the same | `["a", "d", "bb", "cc"]`; 22 steps, 5 units |
| `[{n: "a", k: 1}, {n: "b", k: 2}, {n: "c", k: 1}].sortBy(p => p.k, "desc").map(p => p.n)` | | `["b", "a", "c"]` |
| `["\u{ffff}", "😀"].sortBy(s => s, "desc")` | | `["😀", "\u{ffff}"]` |
| `xs.sortBy(x => x, d)` | `{xs: int[], d: any}`, xs = `[2, 1]`, d = `"up"` | Fails: `wrong-kind`, template `order`, message `sortBy takes "asc" or "desc", found "up"`; 1 step, 0 units |
| `xs.sortBy(x => x, "up")` | `{xs: int[]}` | `arg-mismatch` on `"up"` |

### 6.4 `minBy`

| Field | Value |
| --- | --- |
| Signature | `T[].minBy: (f: (item: T, index: int) => K) => T \| null where K: ordered` |
| Receiver | `T[]` |
| Steps | `2*n + sz(keys)` |
| Units | `0` |
| Null | The list is empty |
| Fails | `wrong-kind`, template `unordered`: the keys are not all numbers or all strings |
| Order | For each element: 2 steps, then `f`. Then the ordered test over the keys; steps `sz(keys)`; result. |

1. `f` is called once for each element, in order, before any comparison. Its
   result is the element's key.
2. The keys must be ordered (`04-library.md` 3.6). They are tested after the
   last call. A null key, a key of another kind, or a mix of numbers and
   strings fails with `wrong-kind`, and charges nothing more.
3. The result is the element whose key is the least by the order of 3.6. Of
   elements with equal keys, the first. Numbers compare by value, so `1` and
   `1.0` are equal keys; strings by code point.
4. Null for an empty list. An element that is null gives null too.
5. The result is the element, not its key. The key alone is
   `xs[*].k.min()`.

| Program | Data | Result |
| --- | --- | --- |
| `xs.minBy(p => p.a)` | `{xs: {a: int, n: string}[]}`, xs = `[{n: "b", a: 2}, {n: "a", a: 1}, {n: "c", a: 1}]` | `{a: 1, n: "a"}`; 11 steps, 0 units |
| `xs.minBy(x => x)` | `{xs: int[]}`, xs = `[]` | `null` |
| `xs.minBy(x => x)` | `{xs: any}`, xs = `[1, null]` | Fails: `wrong-kind`, template `unordered`; 6 steps, 0 units |

### 6.5 `maxBy`

| Field | Value |
| --- | --- |
| Signature | `T[].maxBy: (f: (item: T, index: int) => K) => T \| null where K: ordered` |
| Receiver | `T[]` |
| Steps | `2*n + sz(keys)` |
| Units | `0` |
| Null | The list is empty |
| Fails | `wrong-kind`, template `unordered`: the keys are not all numbers or all strings |
| Order | For each element: 2 steps, then `f`. Then the ordered test over the keys; steps `sz(keys)`; result. |

1. As `minBy`, with the greatest key. Of elements with equal keys, the first,
   as the list method `max` takes the first. So `xs.maxBy(f)` is not
   `xs.sortBy(f).last()`, which takes the last.

| Program | Data | Result |
| --- | --- | --- |
| `xs.maxBy(p => p.a)` | as for `minBy` | `{a: 2, n: "b"}`; 11 steps, 0 units |
| `[{n: "x", a: 2}, {n: "y", a: 2}].maxBy(p => p.a)?.n` | | `"x"` |

### 6.6 `uniqueBy`

| Field | Value |
| --- | --- |
| Signature | `T[].uniqueBy: (f: (item: T, index: int) => K) => T[] where K: ordered` |
| Receiver | `T[]` |
| Steps | `2*n + sz(keys)` |
| Units | `1 + k` |
| Null | Never |
| Fails | `wrong-kind`, template `unordered`: the keys are not all numbers or all strings |
| Order | For each element: 2 steps, then `f`. Then the ordered test over the keys; steps `sz(keys)`; find `k`; units; build. |

1. `f` is called once for each element, in order. The keys must be ordered,
   tested after the last call.
2. The result holds each element whose key is not equal to the key of an
   earlier element, in order: the first element with each key. Keys are equal
   by `04-library.md` 3.6: `1` and `1.0` are one key.
3. `k` is the number of elements kept. It is found before the units are
   charged.
4. The keys already met are found with a hash set over the keys, so that the
   expected work is at most a constant times the steps charged.

| Program | Data | Result |
| --- | --- | --- |
| `xs.uniqueBy(s => len(s))` | `{xs: string[]}`, xs = `["bb", "a", "cc", "d"]` | `["bb", "a"]`; 14 steps, 3 units |
| `xs.uniqueBy(x => x)` | `{xs: any}`, xs = `[1, "1"]` | Fails: `wrong-kind`, template `unordered` |

### 6.7 `indexBy`

| Field | Value |
| --- | --- |
| Signature | `T[].indexBy: (f: (item: T, index: int) => string) => map<T>` |
| Receiver | `T[]` |
| Steps | `2*n + sz(keys)` |
| Units | `1 + 2*n` |
| Null | Never |
| Fails | `wrong-kind`, template `result`: a result of f is not a string |
| Order | For each element: 2 steps, then `f`, then its result is tested. Then steps `sz(keys)`; units; build. |

1. `f` is called once for each element, in order. Each result must be a
   string, the element's key, tested when that call returns. A number is keyed
   by `string(n)`.
2. The result maps each key to the last element with that key: a later element
   replaces an earlier one, as in `toMap`.
3. Units are `1 + 2*n` whatever keys repeat, as for `toMap`.
4. The map is built with a hash map over the keys. Like any map, it shows its
   entries in key order.

| Program | Data | Result |
| --- | --- | --- |
| `xs.indexBy(u => u.id)` | `{xs: {id: string, v: int}[]}`, xs = `[{id: "a", v: 1}, {id: "b", v: 2}, {id: "a", v: 3}]` | `{a: {id: "a", v: 3}, b: {id: "b", v: 2}}`; 11 steps, 7 units |
| `xs.indexBy(x => x)` | `{xs: any}`, xs = `[1]` | Fails: `wrong-kind`, template `result`; 4 steps, 0 units |

### 6.8 `countBy`

| Field | Value |
| --- | --- |
| Signature | `T[].countBy: (f: (item: T, index: int) => string) => map<int>` |
| Receiver | `T[]` |
| Steps | `2*n + sz(keys)` |
| Units | `1 + 2*g` |
| Null | Never |
| Fails | `wrong-kind`, template `result`: a result of f is not a string |
| Order | For each element: 2 steps, then `f`, then its result is tested. Then steps `sz(keys)`; units; build. |

1. As `indexBy` for the keys. The result maps each distinct key to the number
   of elements with it. Each count is 1 or more.
2. `g` is the number of distinct keys. It does not build the groups: it is
   `groupBy` and a count in one, at `1 + 2*g` units where `groupBy` charges
   `1 + 3*g + n`.

| Program | Data | Result |
| --- | --- | --- |
| `xs.countBy(x => x)` | `{xs: string[]}`, xs = `["b", "a", "b"]` | `{a: 1, b: 2}`; 8 steps, 5 units |
| `xs.countBy(x => x)` | `{xs: string[]}`, xs = `[]` | `{}` |

### 6.9 `mapValues`

| Field | Value |
| --- | --- |
| Signature | `map<T>.mapValues: (f: (value: T, key: string) => U) => map<U>` |
| Receiver | `map<T>` |
| Steps | `2*n + L*(n + sz(keys))` |
| Units | `1 + 2*n` |
| Null | Never |
| Fails | Only as 2.6 says |
| Order | Tests; steps `L*(n + sz(keys))`; for each entry, in key order: 2 steps, then `f`. Then units; build. |

1. The receiver is a map of `n` entries, or a record, whose `T` is the join of
   its field types (`04-library.md` 5.5).
2. The keys are put in key order first, and that is charged first. Then `f` is
   called once for each entry, in key order, with the entry's value and its
   key. The callback has no index.
3. The result has the same keys, each with the result of `f` for its entry. It
   never gives null by itself.
4. A callback that declares more than two parameters fails the shape test 5 of
   `04-library.md` 2.2.

| Program | Data | Result |
| --- | --- | --- |
| `m.mapValues(v => v * 2)` | `{m: map<int>}`, m = `{b: 1, a: 2, c: 3}` | `{a: 4, b: 2, c: 6}`; 17 steps, 7 units |
| `{b: 1, a: 2}.mapValues((v, k) => k + string(v))` | | `{a: "a2", b: "b1"}` |
| `m.mapValues(v => v)` | `{m: map<int>}`, m = `{}` | `{}` |

### 6.10 Types

1. The checker reads the signatures from the records (`05-types.md` 13.1). A
   new entry is checked by 13.5 like any other.
2. `order`'s parameter type is the literal set `"asc" | "desc"`. An argument
   fits it through its form (15.1): `"desc"` and `c ? "asc" : "desc"` fit; a
   `string` and `"up"` are `arg-mismatch` on the argument; a type with null is
   `maybe-null`; an `any` argument records a `dynamic` site on the argument.
3. `K: ordered` of `minBy`, `maxBy` and `uniqueBy` is tested by 13.5 step 4: a
   key that may be null, or a list, is `arg-mismatch` on the call; a key of
   type `any` records a `dynamic` site on the call.
4. The callbacks of `indexBy` and `countBy` give `string`: a body that does not
   fit is `arg-mismatch` on the body, and a body of type `any` records a
   `dynamic` site on the body. A run failure of the test of a result is then at
   a `dynamic` site (`05-types.md` 18.3).
5. On a receiver of type `any`, 13.3 applies. None of the six entries is
   marked `!`, so a call of one of their names on `any` records no `value`
   site.

## 7. Calls that stay errors

| Written | Outcome |
| --- | --- |
| `minBy(xs)`, the name called as a function | `undefined-function`: the six are methods only |
| `m.minBy(f)`, `m` a map | `no-method`; at run time on `any`, `wrong-kind`, template `method` |
| `xs.mapValues(f)`, `xs` a list | The same |
| `xs.sortBy(f, "asc", 1)` | `arity`; at run time on `any`, `wrong-kind`, template `count` |
| `xs.sortBy(f, g => 1)` | `not-a-value` on the lambda; at run time on `any`, `wrong-kind`, template `not-callback` |

## 8. Narrowing by a tag in `filter` and `find`, and projections of unions

This section needs F5, and adds no rule to it. A test of a field of a tagged
union gives facts of the two kinds of `f04-conversion.md` 6.4 about the path
before the field (`f05-unions.md` 5.2), and `f05-unions.md` 5.7 lets them reach
the parameter of the predicate of `filter` and `find`.

1. So the element type of the result of `filter` and `find` is narrowed to the
   variants the test leaves, and a projection after it reads those variants.
2. A fact about a longer path that starts with the parameter,
   `x.a.kind == "b"`, narrows `x.a` within the body, and not the element type.
3. **A projection over a union** reads each element as `f05-unions.md` 5.1
   reads one value: a read of flat(U). So `shapes[*].kind` is a list of the
   union of the tag sets, `shapes[*].r` is `maybe-absent` on `r`, and
   `shapes[*]?.r` is `(float | null)[]`.
4. Nothing else changes: the values, the steps and the units are those of the
   entries and of chapter 4.

| Program | `shapes: ({kind: "circle", r: float} \| {kind: "rect", h: float, w: float})[]` |
| --- | --- |
| `shapes.filter(s => s.kind == "circle")` | `{kind: "circle", r: float}[]` |
| `shapes.find(s => s.kind != "circle")` | `{h: float, kind: "rect", w: float} \| null` |
| `shapes.filter(s => s.kind == "circle").map(s => s.r)` | `float[]` |
| `shapes.filter(s => s.kind == "circle")[*].r` | `float[]` |
| `shapes[*].kind` | `("circle" \| "rect")[]` |
| `shapes[*].r` | `maybe-absent` on `r` |
| `shapes[*]?.r` | `(float \| null)[]` |

## 9. Analysis

This section adds to `f02-templates-analysis.md` section 3, as its section 5
reserves.

### 9.1 The star step of a path

1. A star is a step of a static path. In the table of
   `f02-templates-analysis.md` 3.5 item 2 this row is added: `[*]` or `?.[*]`,
   a star: the step star, and the path goes on.
2. The reads after a star are reads of each element, and the path goes on
   through them by the rows of 3.5. It stops before an index whose expression
   is not a `String` node, before a method call, and at the end of the chain,
   as before. So a method call after a projection ends the path.
3. `[*]` and `?.[*]` give the same step, as `.` and `?.` do.
4. The span of a reference whose path ends with a star runs to that star's `]`.
5. **What a reference through a star claims.** The program may read the list
   at the path before the star, its length, and at each element of it the value
   at the rest of the path and any part of that value. A shorter path claims
   more of the value, never less.

| Program | Path of the reference to `orders` |
| --- | --- |
| `orders[*].lines[*].sku` | star, `lines`, star, `sku` |
| `orders[*].lines` | star, `lines` |
| `orders[*].lines[0].sku` | star, `lines`. It stops at the index |
| `orders[*].lines["x"]` | star, `lines`, `x` |
| `orders[*].lines[*].qty.sum()` | star, `lines`, star, `qty` |
| `orders?.[*]?.id` | star, `id` |
| `orders["*"]` | the key `*`, which is not a star |
| `orders.map(o => o.id)` | empty: the path stops at the call |

### 9.2 Calls in a projection

A call in an index expression of a projection's read is reported as any call
is. Its `lambda` is `false`: it stands inside no `Lambda` node. It is evaluated
once for each element its read is applied to, zero times or more, and no name
is bound there.

### 9.3 The JSON form

1. In a reference's `path`, the step star is the object `{"star": true}`. A key
   is a string, so no key equals it.
2. In `f02-templates-analysis.md` 4.3 the form of `path` becomes: a list of
   steps, each a string or the object `{"star": true}`. 4.4 compares them step
   by step: a star equals a star, and never a key.
3. In the schema of 4.6, the `items` of `path` become:

```json
{"oneOf": [
  {"type": "string"},
  {"type": "object", "required": ["star"],
   "properties": {"star": {"const": true}}, "additionalProperties": false}]}
```

4. The `outcome` command writes a star step in this form.

## 10. Additions to the case format

None. The cases of this package use the fields of `03-host.md` 3.10 and of F2
and F4. A runner of an implementation that does not offer F7 does not run
`suite/queries/` and does not load `suite/library-queries.json` or
`suite/messages-queries.json`.

## 11. Messages

This package has three run-phase templates. They stand in a catalog file of
their own, `suite/messages-queries.json`, in the format of `suite/messages.json`
(`03-host.md` 3.8.2), each with cases of its own. Each is a template of a run
code of Core: no code is added.

| Code | Template | Text | When |
| --- | --- | --- | --- |
| `null-operand` | `star` | `cannot apply [*] to null` | The receiver of a star `[*]` is null |
| `wrong-kind` | `star` | `cannot apply [*] to a value of kind {kind}` | The receiver of a star, `[*]` or `?.[*]`, is not null and not a list. `kind` is the receiver's kind |
| `wrong-kind` | `order` | `{name} takes "asc" or "desc", found {found}` | An argument at a parameter whose type is the literal set `"asc" \| "desc"` is not one of its two strings, null and every other kind included. `name` is the entry's name; `found` is the argument |

1. `suite/messages.json` is not changed: Core's catalog keeps its 64
   templates. An implementation that offers F7 reads both files and generates
   its messages from both. One that does not offer F7 reads `messages.json`
   only.
2. An entry of `suite/messages-queries.json` has a code that `messages.json`
   has, and adds templates to that code's entry. Its `params` are params of
   that entry, with the same kinds, and its template names are not names of
   that entry. Its `caught` and `since` are those of the entry. The rules of
   `03-host.md` 3.8.2 and 3.8.3 apply to the two files together.
3. The file has two entries: `null-operand`, with `star` and 2 cases, and
   `wrong-kind`, with `star` and `order` and 4 cases.
4. **Core's templates for this package's entries.** The template `result` of
   `wrong-kind` is also the failure of the test of a result of the callback of
   `indexBy` and of `countBy` (6.2): `name` is the entry's name and `kind` the
   kind of the result. The template `unordered` is also the failure of the
   ordered test of the keys of `minBy`, `maxBy` and `uniqueBy`: the scan and
   `kind` are those of `04-library.md` 3.6.

## 12. The cases

| Path | Holds |
| --- | --- |
| `suite/library-queries.json` | The 7 records of 6.1, each with its cases |
| `suite/messages-queries.json` | The 3 templates of section 11, with 6 cases |
| `suite/queries/` | The cases of the projection, its cost, its types, its analysis, the entries in use together, the Core texts of section 13, and the cases that need F5 |

1. A runner runs `suite/library-queries.json` as it runs `library.json`, the
   cases of `suite/messages-queries.json` as it runs those of `messages.json`,
   and the files of `suite/queries/` as it runs `suite/core-run/` and
   `suite/core-check/` (`03-host.md` 3.10.8 item 5). A case whose `expect` has
   `type` or `sites`, or a `check` failure with a code that is not static, is
   run once, checked. An analysis case is run once (F2 4.2).
2. `suite/queries/queries-unions.json` uses the tagged unions of F5.
3. No file of this package is named `until-`. No earlier `until-f07-` file
   exists.

## 13. Earlier cases

1. These Core cases pin as errors texts this package turns into programs
   (rulings BA4). Each text is in `suite/queries/queries-core-texts.json`, with
   the outcome of this package. The lead removes them when the package is
   accepted.

| File | Case | Core outcome | With F7 |
| --- | --- | --- | --- |
| `suite/core-run/parse-follow-on.json` | "a projection", `xs[*]` | `unexpected-token` at `*` | `undefined-name` on `xs`: the text parses |
| | "an optional projection", `xs?.[*]` | `unexpected-token` at `*` | The same |
| | "a projection then a field", `xs[*].a` | `unexpected-token` at `*` | The same |
| `suite/core-check/check-library.json` | `xs.sortBy(x => x, "desc")` | `arity` | `int[]` |
| | `xs.minBy(x => x)` | `no-method` | `int \| null` |
| | `xs.maxBy(x => x)` | `no-method` | `int \| null` |
| | `xs.uniqueBy(x => x)` | `no-method` | `int[]` |
| | `xs.indexBy(x => x)` | `no-method` | `arg-mismatch` on the body `x`: an `int` does not fit `string` |
| | `xs.countBy(x => x)` | `no-method` | The same |
| | `scores.mapValues(x => x)` | `no-method` | `map<float>` |

2. These Core texts keep their outcome: `minBy(xs)`, `undefined-function`
   (`suite/core-run/static-undefined-function.json`); `xs.*.f`, `xs[?c]` and
   `xs[].f` (`suite/core-run/parse-follow-on.json`); every case of the Core
   record of `sortBy`.
3. No Core case states the spans this package moves by `01-language.md` 14A
   item 2: `xs[*1]` and the other rows of 2.2 whose span is the token after the
   `*`.

## 14. Examples

Each example is a case of `suite/queries/queries-examples.json`.

| # | Program | Data | Result |
| --- | --- | --- | --- |
| 1 | `orders.filter(o => o.status == "open")[*].id` | `orders` of 3 with statuses open, paid, open | `["a", "c"]` |
| 2 | `orders[*].lines[*].sku` | 3 orders of 2, 0 and 4 lines | the 6 skus; 18 steps, 7 units |
| 3 | `events.sortBy(e => e.ms, "desc").slice(0, 2)[*].ms` | `ms` 5, 9, 1 | `[9, 5]` |
| 4 | `orders.sortBy(o => o.placed).sortBy(o => o.customer, "desc")[*].id` | | customer descending, then `placed` ascending |
| 5 | `events[*].user.filter(u => u != null).unique()` | users `"a"`, null, `"b"`, `"a"` | `["a", "b"]`, type `string[]` |
| 6 | `events.countBy(e => e.kind)` | | a count for each kind |
| 7 | `orders.groupBy(o => o.customer).mapValues(g => g[*].lines[*].qty.sum())` | | the quantity for each customer |
| 8 | `events.maxBy(e => e.ms)` | | the slowest event |
| 9 | `{ const byId = customers.indexBy(c => c.id); orders.map(o => byId?.[o.customer]?.name ?? "unknown") }` | | each order's customer name |
| 10 | `services.entries().groupBy(e => e.value.owner).mapValues(g => g[*].key)` | | each owner's services |
| 11 | `"urgent" in issue.labels[*].name` | | `true` |
| 12 | `events[*].ms.max()` | | the greatest `ms` |
