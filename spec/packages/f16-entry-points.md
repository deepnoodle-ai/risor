# F16. Entry points: annotations, type lines, descriptions and export

This chapter is normative. It specifies follow-on package F16 across its three
slices (`experiments/v3/design/17-entry-points.md`, and
`experiments/v3/design/14-implementation-plan.md` 5.3, track D):

- **Slice D1** (sections 1 to 10): type lines, annotations on `const` lines, on
  the parameters of a lambda and on its result, and the syntax of a function
  type on a parameter. It adds fields to the nodes `Program`, `Const`, `Lambda`
  and `Param`, the check code `name-clash`, the catalog file
  `suite/messages-entry-points.json`, the hover kind `type` of F12, and the
  directory `suite/entry-points/`.
- **Slice D3** (section 11): doc comments as descriptions, `SchemaOf`, and the
  `Doc` of host declarations.
- **Slice D2** (section 12): `export const` entry points, programs with no
  result expression, the check code `bad-export`, the `declare` codes
  `no-result` and `no-input-record`, the Call stage (positional and by record),
  per-entry expected results, and the `Entries` call.

The package adds no node kind, no value, no type, no run code, no `parse` code
and no reserved word.

The package only adds. In Core an annotation and a type line are `parse`
errors. A Core amendment lands with this slice (section 5): the name of a
`const`-bound lambda is in scope in its own body, and a reference to it there
is the static error `recursion-needs-types`. That rule is Core's, and lives in
the Core chapters. Section 10 lists the earlier cases whose outcome changes.

Other chapters: `01-language.md` (grammar, nodes, spans, levels, scope),
`02-evaluation.md` (steps, units, the depth count), `03-host.md` (diagnostics,
case files, the order of packages), `05-types.md` (the checker, fits, the
static rules), F2 (`spec/packages/f02-templates-analysis.md`), F4
(`spec/packages/f04-conversion.md`), F8 (`spec/packages/f08-errors.md`), F12
(`spec/packages/f12-tools.md`) and F17 (`spec/packages/f17-recursion.md`).

## 1. Terms and scope

### 1.1 Terms

| Term | Meaning |
| --- | --- |
| Program type | A type written in a program, by F4 2.1: `PType` |
| Annotation | A program type written after a `:` on a `const` line, on a parameter or after a lambda's parameter list; or a function type on a parameter |
| Type line | `type Name = T;` at the top level of a program |
| Function type, `Fn` annotation | `(a: T, b: U) => R` after the `:` of a parameter that is a name. R may be `unknown` |
| Annotated parameter | A parameter that has an annotation, a program type or a function type |
| Claim | What an annotation states about a value. The checker proves it (3.3); a run never tests it (4) |
| Surely fits | Fits without using `any` (F4 6.2; `05-types.md` 4.2) |
| Checked once | A `const`-bound lambda that 3.4 checks where it is written, and not at each use |

### 1.2 What this package changes in earlier rules

| Earlier rule | With this package |
| --- | --- |
| `01-language.md` 3.1, `Program`, `Const`, `Lambda`, `Param` | Type lines and annotations (2.1) |
| `01-language.md` 3.2 rule 5 | A `(` also begins a `Lambda` when the token after its matching `)` is `:` (2.2) |
| `01-language.md` 4.1, nodes 1, 2, 22 and 23 | The fields `types`, `type`, `result` and `fn` (2.3) |
| `01-language.md` 14.3, the rows of `(x: int) => x` in an argument, `const n: int = 1;` and `type Name = T` | Replaced: these are programs (2.5) |
| `05-types.md` 13.5, 13.8 and 13.9 | Annotated lambdas: checked once, or at each use with their annotations; the values the library passes (3.4) |
| `05-types.md` 15.1 | The fit of a value against an annotation does not use `any` (3.3) |
| `05-types.md` 22 | The static rules of type lines, `name-clash` (3.1) |
| `02-evaluation.md` 8.4 item 10 | Holds only for a program whose `const`-bound lambdas are all checked at each use (3.4) |
| F4 6.1 | Its static rules apply to every program type of an annotation and of a type line (3.1) |
| F8 2.7, E9 and E17 | Narrowed to the text that stays an error (2.5). The texts of `type-annotation` and `type-declaration` change (9) |
| F12 2.8 | The documents of a type line and of annotations (7.1) |
| F12 3.3 item 7, 3.5, 3.10 | The repair of an unclosed parameter list; types in annotations and type lines (7.2) |
| F12 3.13 | The hover kind `type` (7.3) |
| `03-host.md` 3.11 item 2 | F16, slice D1, is offered after F14 and before F17 (1.4) |

No other rule changes. A program with no type line and no annotation has the
type, sites, diagnostics, value and usage it has without this package.

### 1.3 Version

Type lines and annotations are part of version `3.0`. This package adds no
version number and no version marker.

### 1.4 Packages this one requires

1. Packages are offered in a fixed order, each only with every package before
   it (`03-host.md` 3.11 item 2): F3, F1, F2, F4, F5, F6, F7, F8, F9, F10, F11,
   F13, F14, F16 slice D1, F17, F16 slice D3, F19, F16 slice D2, F12. Slice D1
   stands before F17, and each later slice takes a place of its own after F17,
   in the order the slices are built (`15-build-rulings.md` R211, R261, R371).
2. An implementation that offers this package offers F4: an annotation is a
   program type of F4, resolved by F4's rules. It offers F8, so a case of this
   package may state messages.
3. The cases of each slice stand in their own files. Those of slice D1 are the
   files of `suite/entry-points/` whose names begin `d1-` or `tools-d1-` (8),
   those of slice D3 the files whose names begin `d3-` or `tools-d3-` (11.8),
   and those of slice D2 the files whose names begin `d2-` or `tools-d2-`
   (12.10).
4. **An implementation that offers F17 or F12 and not yet this slice** does
   not run `suite/entry-points/` or load `suite/messages-entry-points.json`,
   and it does not pass the new Core cases of section 5: the Core amendment
   lands with this slice. The cases it would still pass stood in files named
   `until-f16-` (`03-host.md` 3.11 item 4). All three implementations offered
   this slice in the change that added it, so those files were deleted
   (3.11 item 5; section 10).

### 1.5 Settled points

These four points were written to the recommendation and settled by the owner
(point 2 on 2026-11-19, R261; points 1, 3 and 4 on 2026-11-25, #696, R232 and
R238).

| # | Point | Settled rule | Strongest alternative |
| --- | --- | --- | --- |
| 1 | Whether a narrowed `parse` code retires | None retires: `type-annotation` and `type-declaration` each still cover text that stays an error (2.5), so the count of `parse` codes does not change (R238) | Retire a code whose main text became a program |
| 2 | The place of slice D1 in the order of 1.4 | Before F17 (R261) | After F17, beside the other slices of F16 |
| 3 | The new texts of `type-annotation` and `type-declaration` | Changed in place in `suite/messages-parse.json`, so the count of templates does not change (9.2, R238). A build with F8 and without F16 prints them too | New templates in this package's catalog file. That changes the message of two catalog cases of `messages-parse.json` in an implementation that offers F16, and 3.11 item 4 has no file named `until-` for a catalog |
| 4 | A fit at an annotation that uses `any` (3.3) | It does not hold: the value must surely fit (R232) | It holds with a `dynamic` site, as at an argument of Core; then the name has the annotation's type while its value may be anything, and a later failure is at no site (`05-types.md` 18.4, promises 1 and 4) |

## 2. Syntax

### 2.1 Grammar

The productions `Program`, `Const`, `Lambda` and `Param` of `01-language.md`
3.1 become these. `PType` and `TName` are those of F4 2.1.

```ebnf
Program  = { Const | TypeLine } Expr [ ";" ] .
TypeLine = "type" TName "=" PType ";" .
Const    = "const" Pattern ( "=" ( Lambda | Expr ) | ":" PType "=" Expr ) ";" .
Lambda   = ( Name | "_"
           | "(" [ Param { "," Param } [ "," ] ] ")" [ ":" PType ] ) "=>" Expr .
Param    = Pattern [ ":" PType ] [ "=" Expr ] | Name ":" PFn .
PFn      = "(" [ Word ":" PType { "," Word ":" PType } ] ")"
           "=>" ( PType | "unknown" ) .
```

```js
type Line = {sku: string, qty: int, price: float};
type Order = {id: string, lines: Line[], ...};
const total = (order: Order, rate: float = 0.2): float =>
  order.lines.map((l: Line) => l.qty * l.price).sum() * (1 + rate);
const n: int = len(orders);
{n, totals: orders.map(o => total(o))}
```

1. `type` is not a reserved word. It is a type line's first word only where
   2.2 rule 1 says; elsewhere it is a name. So `type(x)` calls the library
   function `type`, and `const type = 1; type` reads a name.
2. Type lines and `const` lines may stand in any order before the result.
   Blocks and branches have no type lines.
3. A parameter written as a bare name, `x => e`, has no annotation. A lambda
   with an annotation is written with parentheses.
4. A `PFn` stands only after the `:` of a parameter whose pattern is a name. It
   has no default, and is never a result's annotation, a `const` line's, or a
   type line's type. Its words are documentation: they bind nothing.
5. A `PFn` takes no comma after its last parameter, as an `Fn` of type text
   does not (`05-types.md` 3.2).

### 2.2 How the parser chooses

These rules add to the table of `01-language.md` 3.2.

| # | Place | Rule |
| --- | --- | --- |
| 1 | The start of a line at the top level of a program, where `const` may stand | The word `type` begins a `TypeLine` exactly when the next token is a `Word` whose first code point is an upper-case letter `A` to `Z`, and the token after that is `=`. Otherwise `type` is a name, and the place holds the program's result |
| 2 | Rule 5 of `01-language.md` 3.2: where an `Arg` or the value of a `Const` may stand | A `(` begins a `Lambda` when the token after its matching `)` (rule 6) is `=>` or `:` |
| 3 | The `:` after the pattern of a `Param` that is a name | It begins a `PFn` exactly when the next token is `(` and the token after it is `)`, or a `Word` followed by `:`. Otherwise a `PType` follows |
| 4 | The value of a `Const` that has an annotation | An `Expr`. A `Lambda` is not read there: a function is not a value of a program type |

1. **Rule 2 changes no program.** At an `Arg` or at a `const` value, a
   parenthesized expression directly followed by `:` is never the start of an
   `Expr`: a `:` there could only be the `:` of `? :`, and no `?` can stand
   before it outside the parentheses. So every text that parses without this
   package parses the same with it. A text that was an error stays one, with a
   code or a span that may change: `f((a): b)` is now a lambda whose result
   annotation `b` is `bad-type-text`, and `f((x, y): int)` is `unexpected-token`
   at the `)`.
2. The places where a lambda may stand are those of `01-language.md` 8.1 rule
   2, and rule 2 above holds at each: the value of a `const` line at the top, in
   a block and in a branch; an argument of a `Call`, of a `MethodCall` and of a
   call of a namespace.
3. In a type line, the name is read as a word: a name of one letter is
   `bad-type-text` at the name, with the template `letter` (2.5).

### 2.3 The nodes

| Node | Field added | Holds |
| --- | --- | --- |
| 1 `Program` | `types` | The type lines, in the order of the text. Each has a `name`, the span of the name, and a `type`, a program type. A type line is not a node, as a program type is not one: its place among the `const` lines is its span |
| 2 `Const` | `type` | The annotation, a program type, or none |
| 22 `Lambda` | `result` | The result annotation, a program type, or none |
| 23 `Param` | `type` | The annotation, a program type, or none |
| 23 `Param` | `fn` | The function type, or none: its parameters, each a word and a program type, and its result, a program type or `unknown`. At most one of `type` and `fn` is set |

1. The span of a `Param` runs from its pattern to the end of its default, or
   to the end of its annotation when it has no default.
2. The span of a type line runs from `type` to its `;`.
3. No node kind is added, so the walkers of `01-language.md` 4 and of every
   package see the same nodes. The strongest alternative was a node kind 33,
   `TypeLine`, in one list of top-level lines; it would change every walker
   for a node that holds no expression.

### 2.4 Levels of nesting

The `:` of an annotation, the word `type` and the `=` of a type line open no
level (`01-language.md` 13.1). The brackets of a program type open levels as F4
2.8 says. The `(` of a `PFn` opens one level, to its `)`.

### 2.5 Text that stays an error

| Text | Core code | Span | With F8 |
| --- | --- | --- | --- |
| A type line in a block or a branch: `{ const a = 1; type Foo = int; a }` | `unexpected-token` | The name after `type` | `type-declaration` at `type` (E17) |
| `type` followed by a lower-case word, or by a name and no `=`: `type foo = int;`, `type Foo int;` | `unexpected-token` | The word after `type` | `type-declaration` at `type` |
| A type line whose name has one letter: `type T = int;` | `bad-type-text` | The name | Template `letter` |
| A type line with no `;`, or as the whole text | `unexpected-token` | The token after the type, or the end | The same |
| An annotated `const` line whose value is a lambda: `const f: int = x => x;` | `unexpected-token` | The `=>` | The same |
| The same with a parameter list that has an annotation: `const f: int = (x: int) => x;` | `unexpected-token` | The `:` | `type-annotation` (E9) |
| An annotation in a parenthesized expression: `f((x: int))`; and a lambda where none may stand: `(x: int) => x` at the top of a program | `unexpected-token` | The `:` | `type-annotation` |
| A bare parameter with an annotation: `xs.map(x: int => x)` | `unexpected-token` | The `:` | The same |
| A `PFn` with a default | `unexpected-token` | The `=` | The same |
| A function type on a pattern, on `_`, as a result annotation, on a `const` line, in a type line | `bad-type-text` | The first word after its `(` | Template `lower` for a lower-case word |
| A `PFn` with a comma after its last parameter | `unexpected-token` | The `)` | The same |
| `unknown` anywhere but a `PFn`'s result; a name of one letter in an annotation | `bad-type-text` | The word | Templates `lower` and `letter` |

1. **E9 narrows** to its third case (F8 2.7): `:` after a `Name` that follows
   the `(` of a parenthesized expression, where no lambda may stand.
2. **E17 narrows** for `type`: only where 2.2 rule 1 does not begin a type
   line. `interface` and `enum` keep it everywhere.
3. A Core build without this package reports these texts by `01-language.md`
   14.3; the rows it names stay true of Core.

## 3. Meaning and the checker

### 3.1 The static rules

These rules apply in every compile, with or without the type rules, in phase
`check` (`05-types.md` 22).

1. **Order.** The type lines and the `const` lines are walked in the order of
   the text, then the result.
2. **A type line's type** is resolved by F4 6.1 in the type names in scope
   where the line stands: the environment's named types and opaque types, and
   the type lines before it. Its diagnostics are F4 6.1's: `undefined-name`
   with the template `type`, `untagged-union` (without F5, or for records that
   form no union), and `check-limit`.
3. **`name-clash`.** A type line whose name is a named type or an opaque type
   of the environment is `name-clash` on its name. The environment's type
   stands, and the line declares nothing.
4. **`duplicate-name`.** A type line whose name an earlier type line has, and
   that is not `name-clash`, is `duplicate-name` on its name. The later line
   stands, also when its type has a diagnostic: it then declares nothing
   (item 5), and the earlier line's declaration ends there. So in `type Aa =
   int; type Aa = Nope; const x: Aa = 1; x` the `Aa` of the annotation is
   `undefined-name`.
5. **What a line declares.** A type line with no `name-clash` and no
   diagnostic in its type declares its name, for the rest of the program. A
   line whose type has a diagnostic declares nothing, so a later use of its
   name is `undefined-name` too. The alternative was to declare the name as
   `never`, which would avoid the second diagnostic and give a mismatch at
   each use instead.
6. A type line names only the type lines before it: it cannot name itself or
   a later line. `type Tree = {kids: Tree[]}` is `undefined-name` at the
   inner `Tree`. Package F17, step E2, replaces this item: the type lines of a
   program are resolved as one list, and a line may name itself and any line
   (`spec/packages/f17-recursion.md` 12.2, 12.3).
7. **An annotation's type** is resolved the same way, in the type lines before
   the `const` line or the result that holds it. So in `const f = (x: Late)
   => x; type Late = int; f(1)`, `Late` is `undefined-name`. This stays with
   package F17: only a type line's own type may name a later line
   (`spec/packages/f17-recursion.md` 12.3).
8. A type name and a value name are in two scopes, and never clash:
   `type Order = {id: string}; const Order = 1; Order` reads the value.
9. **An `Fn` parameter keeps Core's static rules** with this slice alone: it
   is a value binding (`05-types.md` 22.7). A call of it is
   `call-of-parameter`, a lambda given at it in a call of its lambda is
   `lambda-to-lambda`, and the name given as a lambda argument is a value.
   Package F17, which follows this slice in the order (1.4), makes it a
   function binding that may be called and given on
   (`spec/packages/f17-recursion.md` 13.2).
10. **The reference of a lambda to its own name** is Core's rule
    `recursion-needs-types` (section 5). With this package alone every
    reference is that error, whether or not the lambda's parameters and
    result are annotated. Package F17, which follows this slice in the order
    (1.4), accepts a reference of a lambda whose every parameter and result
    are annotated (`spec/packages/f17-recursion.md` 11.2).
11. **Static codes.** For `03-host.md` 3.10.8, 3.10.10 item 17 and 3.10.13, in
    `suite/entry-points/` the static codes are Core's twelve, `untagged-union`
    and `check-limit` at a program type (F4 6.1 item 4), and `name-clash`.
    F17's static `arg-mismatch` is a static code in its own directory
    (`spec/packages/f17-recursion.md` 8 item 5), not here: only
    `d1-check.json`, which runs checked, expects `arg-mismatch`.

### 3.2 A compile without the type rules

1. An annotation is a claim. A compile without the type rules applies 3.1 and
   ignores every claim: it binds what the value is, and runs no test. So
   `const x: int = "a"; x` gives `"a"`.
2. An `Fn` annotation is read by every compile (`15-build-rulings.md` R208).
   In this slice alone it changes no rule: its parameter keeps Core's static
   rules (3.1 item 9). Package F17 gives it its own (3.1 item 9), so with F17
   `const g = (f: (x: int) => int) => f; g(1)` is `not-a-value` on the body
   and `arg-mismatch` on the `1`, both static.

### 3.3 The fit of an annotation

An annotation is a claim the checker proves. A value is fitted to an
annotation by `05-types.md` 15.1, "fits through its form", with one change:
**a fit that uses `any` does not hold.** At each place of failure the value
must surely fit (F4 6.2). A place where it fits only by using `any` is a place
of failure, and the message prints the type found: `expected int, found any`.

| Place | Value | Against | Code and template | Span |
| --- | --- | --- | --- | --- |
| A `const` line, `const p: T = e;` | e | T | `result-mismatch`: `result`, `missing`, `extra` | Each place of failure |
| A default, `(x: T = d)` | d | T | The same | The same |
| A result annotation, `(...): R => b` | b | R | The same | The same |
| An argument of a call by name at `(x: T)` | The argument | T; with a default, `T \| null` | `arg-mismatch`: `argument`, `missing`, `extra`; or `maybe-null` when it fits but for null, by `spec/core/05-types.md` 15.2 | The same |
| A plain argument of a call by name at `(f: (...) => R)` | | | `arg-mismatch`, template `function`: `param` the parameter's name, `name` the lambda's. With F17 a static rule, and the template may be `parameter-at-fn` (`spec/packages/f17-recursion.md` 13.2 rule 3) | The argument |
| A value the library passes to `(x: T)` (3.4) | Q | T | `arg-mismatch`, template `argument` | The annotation |
| A value the library passes to `(f: (...) => R)` | | | None: with F17 a lambda that has an `Fn` parameter is never passed (`spec/packages/f17-recursion.md` 13.2 rule 8) | |
| With F17: a lambda or a function parameter given at `(f: (...) => R)`, and a call of `f` | | | `spec/packages/f17-recursion.md` 13.3. Every fit there is this section's | |

1. After the fit, the name of an annotated `const` line is bound to T, by
   `05-types.md` 12: `const x: int | null = 1;` gives `x` the type
   `int | null`. A parameter is bound to its annotation.
2. An annotation that has a static diagnostic (3.1) is fitted by nothing, and
   what it would bind has the type `never`.
3. **Why not `any`.** Where Core fits a value of type `any`, it records a
   `dynamic` site at the value, and the run tests it there. An annotation has
   no run test (4), and the name then has the annotation's type: in
   `const x: int = blob; x + 1` a non-number `blob` would fail at `x + 1`,
   which is no site. That breaks promises 1 and 4 of `05-types.md` 18.4.
   `x as int` is the way to test such a value (F4).

### 3.4 Lambdas with annotations

| Lambda | Rule |
| --- | --- |
| **Checked once**: a `const`-bound lambda with at least one annotation, and every parameter annotated | Checked once, where it is written, under the facts there. Each parameter is bound to its annotation; a parameter with an annotation that has a static diagnostic is bound to `never`, and a parameter with a function type to that type, which F17 lets the body call (`spec/packages/f17-recursion.md` 13.3). Each default and the body's fit to the result annotation are 3.3's. Its sites are kept, also when it has no use. The check counts as one check against `05-types.md` 20.3, and its nesting where it is written against 20.2; a `check-limit` of either is on the lambda, the whole of it (2.3). Its **result type** is its result annotation, or the body's type |
| A use of a lambda checked once | Its body is not checked again, and the use counts against no limit. A call by name: `arity` (static); each plain argument synthesized and fitted to its parameter by 3.3; a lambda argument is `lambda-to-lambda` (static). The call's type is the result type. As a lambda argument of a library function or method (13.5), or of a method on `any` (13.3): each value passed is fitted to its parameter by 3.3, and the result type stands for the body's type |
| **Partly annotated**: a parameter has no annotation | Checked at each use, by `05-types.md` 13.9. An annotated parameter is bound to its annotation, and the argument at it is fitted by 3.3; the others are bound as 13.9 says. A result annotation is fitted at each use, and the call's type is the annotation, or the body's type |
| No use, and not checked once | The row "No use" of 13.9: an annotated parameter is bound to its annotation, any other to `any`. Its defaults and its result annotation are fitted by Fits (`05-types.md` 4.2): a fit that uses `any` holds there, as the sites of that check are dropped |
| No annotation | As 13.9 |
| A lambda written as an argument of a library function or method (13.5) | Each value the library passes to an annotated parameter, the parameter type Q of step 3 with the solution so far, must surely fit the annotation (3.3). The parameter is bound to its annotation. A result annotation R′: the body must fit it by 3.3, and R′ is the body's type for step 3 |
| A method on an `any` receiver (13.3) | The value passed is `any`, which surely fits only `any`: an annotation of another type is `arg-mismatch` |

1. **Narrowing.** For `filter` and `find` (`05-types.md` 13.8), the narrowed
   type N is computed from Q, the element type, under the body's true facts,
   and not from the annotation. So with `ms` of type `(int | null)[]`,
   `ms.filter((m: int | null) => m != null)` has type `int[]`. A parameter
   with a default narrows nothing, annotated or not (13.8):
   `ms.filter((m: int | null = 1) => m != null)` has type `(int | null)[]`.
2. A predicate's body is a tested position as before (`05-types.md` 10.1).
3. A lambda with no parameter and no annotation is not checked once: it keeps
   13.9, so a lambda that is never used keeps dropping its sites. The plan
   says "every parameter annotated"; read alone that would check such a lambda
   once too, and change Core's results. With a result annotation, `() : R => e`
   is checked once.
4. **The depth bound.** A lambda checked once is not expanded at its uses, so
   `02-evaluation.md` 8.4 item 10 holds only for a program whose `const`-bound
   lambdas are all checked at each use.

```js
const f = (x: int[]) => x[0];   // checked once: the site of x[0] is kept
const g = (x: int, y) => [x, y]; // checked at each use
[f([1]), g(1, "a")]              // f(...) is int; g(1, "a") is (int | "a")[]
```

## 4. Evaluation

A type line and an annotation are not evaluated. They charge no step and no
unit, add nothing to the depth count, and open no call. A call of an
annotated lambda costs what the same call of the lambda without its
annotations costs. A run never tests a claim: a value that does not conform is
passed, bound and returned as it is.

## 5. The Core amendment timed with this slice

`15-build-rulings.md` R207 times a Core amendment with this slice, so that no
build asks an author for types it cannot parse:

1. The name of a `const`-bound lambda is in scope in its own body and in the
   defaults of its parameters (`01-language.md` 5 rule 6, 8.1 rule 3).
2. A reference of the lambda to its own name there is the static error
   `recursion-needs-types` (`01-language.md` 14.4; `05-types.md` 16.1, 22.5,
   22.7, 22.9). It is reported once for each lambda, on its first reference.
3. Such a reference is no use for the checker (`05-types.md` 13.9).
4. F2 3.2 reads the amended scope: the name is bound in its own body (6).
5. Package F17, step E1, lifts item 2 for a lambda whose every parameter and
   result are annotated, and makes such a reference a use of a lambda checked
   once (`spec/packages/f17-recursion.md` 11.2, 11.3). Items 1 and 4 do not
   change.

These are Core's rules, and their cases are Core's (8). The static codes go
from 11 to 12, Core's check codes from 27 to 28.

## 6. Analysis

1. A type line and an annotation hold no read and no call (F2 3.2). A type
   name is not an `Ident`: in `const f = (x: Order) => Order; 1`, the second
   `Order` is a read, and the first is not.
2. With the amendment of section 5, the name of a `const`-bound lambda is
   bound in its own body: `const f = x => f(x); f(1)` has no call (F2 3.10,
   A8).

## 7. Tools

### 7.1 The formatter

These rows add to F12 2.8.

| Node | Document |
| --- | --- |
| A type line | `"type" sp name sp "=" sp type ";"`, the type by F12 2.13. A type line is a line of the program, as a `const` line is: the document of it, then `H`, and an empty line before it as F12 2.9 says |
| `Const` with an annotation | `"const" sp ‹pattern› ":" sp type sp "=" assigned(value) ";"` |
| `Param` with an annotation | `‹pattern› ":" sp type`, then the default as before. A function type: `"(" w ":" sp type { "," sp w ":" sp type } ")" sp "=>" sp result`, on one line |
| `Lambda` with an annotation | The rule of one bare parameter does not apply: params is `seq("(", params, ")")`. With a result annotation: params `":" sp type sp "=>" assigned(body)` |

1. The `type`, the name, the `=` and the `;` of a type line, and the `:` and
   every token of an annotation, are kept tokens (F12 2.12 item 1). So a
   comment after the `:` stays there: `(x: /* t */ int)` is written as it
   stands.
2. A type line's type is written by F12 2.13: on one line however long, except
   that a record type breaks over lines when a comment between its `{` and `}`
   is a leading comment, a line comment or a block comment that holds a line
   end (such as a field's doc comment on its own line or spanning lines).
3. An annotation is part of its parameter, which is an element of its
   sequence (F12 2.12 item 5): the trailing comments of the annotation's last
   token are written after the parameter's comma. So `(x: int /* c */, y)`
   gives `(x: int, /* c */ y)`.
4. A parameter of a function type is as a field of a program type for F12
   2.12 item 5: the trailing comments of its last token are written after the
   comma that follows it. So `(f: (a: int /* a */, b: string) => bool, n)`
   gives `(f: (a: int, /* a */ b: string) => bool, n)`.

| Source | Formatted |
| --- | --- |
| `type   Order={id:string,qty:int};o` | `type Order = {id: string, qty: int};⏎o⏎` |
| `const f=(x:int=1,y:string="a"):string=>y;f()` | `const f = (x: int = 1, y: string = "a"): string => y;⏎f()⏎` |
| `const g=(f:(a:int,b:string)=>bool,n)=>n;0` | `const g = (f: (a: int, b: string) => bool, n) => n;⏎0⏎` |

### 7.2 Completion

1. **Types.** A hole in the program type of an annotation, of a `PFn`'s
   parameter or result, or of a type line is in the types context (F12 3.10).
   The candidates of 3.10 then include the names of the type lines that end
   before the hole. A type line's own name is not one in its own type, unless
   an earlier type line has that name: `type Pair = ‸` does not list `Pair`.
   Package F17 changes this for a type line's type: every type line of the
   repaired program is a candidate there, its own name included
   (`spec/packages/f17-recursion.md` 12.8).
2. A hole as a type line's name, or as a word of a `PFn`'s parameter, has no
   context.
3. **The repair** (F12 3.3 item 7). At a place where an `Arg` or a `const`
   value may stand, a `(` whose matching `)` is not among the tokens before the
   end begins a parameter list when the token after it is a `Name` or `_` and
   the token after that is `:`, both before the end. `(x:` cannot begin an
   expression there (2.2 item 1), so no text is read two ways. So in
   `items.map((i: {id: string, qty: int‸` the hole is in the types context.
4. At the end of the text, a `PFn` begins after the `:` of a parameter only
   when 2.2 rule 3 holds before the end. So `const g = (f: (Or‸` reads a
   parenthesized program type, and the hole is in the types context.
5. With the amendment of section 5, the name of a `const`-bound lambda is in
   scope in its body: it is a candidate there, kind `lambda` (F12 3.6 item 1).
6. An annotated parameter is bound to its annotation, so its detail in the
   names context is the type of its annotation, printed (F12 3.12): with
   `type Order = {id: string}`, a parameter `(order: Order)` has the detail
   `{id: string}`.
7. At the end of the text, at a place where 2.2 rule 1 applies, `type`
   followed by one word and nothing more begins a type line, and the hole is
   that word, which may be empty: 2.2 rule 1 is decided from the tokens before
   the end, and no expression goes on with a word after the name `type`. The
   word need not yet begin with an upper-case letter. So in `type ‸`, in
   `type or‸` and in `type Or‸` the hole is a type line's name (item 2), and
   the list is empty.

### 7.3 Hover

This row adds to the table of F12 3.13 item 3:

| The word is | `kind` | `type` | `signature` | `doc` | `declaration` |
| --- | --- | --- | --- | --- | --- |
| A type name in a program type, of an annotation, a type line, or `as` and `is` (F4); or the name of a type line | `type` | The printed type the name stands for there | null | null | The span of the name in its type line; null for a type of the environment |

1. The type a name stands for at a word is that of the last type line before
   the word with that name, when that line declares it (3.1 item 5); else the
   environment's named type, printed, or its opaque type, printed as its name.
   With package F17 a name in a type line's type stands for the line 12.2 item
   1 gives, and a recursive type is printed by its unfolding
   (`spec/packages/f17-recursion.md` 12.8).
2. A type line whose name is `name-clash` declares nothing: a hover on its name
   gives the environment's type, with the declaration null.
3. A name that nothing declares, and a type word such as `int`, has no hover
   (F12 3.13 item 6).
4. A hover on an annotated parameter gives the type it is bound to: its
   annotation.
5. A type line whose type the repair supplied, whole or in part (F12 3.13
   item 2), declares nothing: in `type ‸Foo`, in `type ‸Foo = ` and in
   `type ‸Foo = {a: ` nothing declares `Foo`, and there is no hover. With a
   type `Foo` in the environment, each of the three is `name-clash` too, and
   the hover gives the environment's type (item 2).

## 8. The cases

| Path | Holds |
| --- | --- |
| `suite/entry-points/d1-syntax.json` | The grammar of 2.1, the choices of 2.2, the lambda at each place it may stand, the errors of 2.5 with F8's messages |
| `suite/entry-points/d1-run.json` | The static rules of 3.1, claims in a run without the type rules (3.2), costs (4), the Core rules of an `Fn` parameter |
| `suite/entry-points/d1-check.json` | The checker of 3.3 and 3.4: fits, the lambdas checked once, at each use and with no use, the values the library passes |
| `suite/entry-points/d1-earlier.json` | The seven texts of 10 item 1, with this package's outcome |
| `suite/entry-points/d1-analysis.json` | Analysis of type lines and annotations (6) |
| `suite/entry-points/tools-d1-format.json` | The formatter of 7.1 (F12) |
| `suite/entry-points/tools-d1-assist.json` | Completion and hover of 7.2 and 7.3 (F12) |
| `suite/messages-entry-points.json` | The catalog entries of 9 and their cases |

1. A runner of an implementation that offers this package runs the files of
   `suite/entry-points/` as it runs `suite/core-run/` and `suite/core-check/`
   (`03-host.md` 3.10.8, 3.11 item 7): a file whose name ends `-check.json`
   once, checked, and the others by their cases. The static codes there are
   those of 3.1 item 11.
2. **The files named `tools-`** use the case fields of F12. A runner runs them
   only when its implementation offers F12, as it runs `suite/tools/` (F12
   7.6).
3. It loads `suite/messages-entry-points.json` (9) and runs its cases checked,
   with the static codes of 3.1 item 11.
4. A runner of an implementation that does not offer this package does not run
   `suite/entry-points/` and does not load `suite/messages-entry-points.json`.
5. The Core cases of section 5 stand in `suite/core-run/static-recursion.json`,
   `suite/core-check/check-recursion.json` and `suite/analysis/scope.json`, and
   the case of `recursion-needs-types` in `suite/messages-check.json`.

## 9. Messages

### 9.1 This package's catalog file

`suite/messages-entry-points.json` has the format of `03-host.md` 3.8.2. An
entry of a code that another catalog file has adds templates to it, as F7 11
says: the same code, params of the same kinds, new template names. An entry of
a code no other file has is added whole.

| Code | Phase | Template | Text | Params |
| --- | --- | --- | --- | --- |
| `name-clash` | `check` | `default` | `{name} is a type of the environment, and a type line cannot declare it again` | `name`: word |

The file had a second entry, the template `fn-value` of `arg-mismatch`, for a
value that the library passes to an `Fn` parameter of a lambda argument. With
F17 no lambda that has an `Fn` parameter is passed, so no build that offers
F17 chooses it, and no build offers this slice without F17 (1.4). It retired
with step E3 of F17 (`spec/packages/f17-recursion.md` 13.5;
`15-build-rulings.md` R288).

1. `recursion-needs-types` is a code of Core (section 5). Its entry is in
   `suite/messages-check.json` (F8 3.1), with the template `default`. Its text
   is F17's (`spec/packages/f17-recursion.md` 9): `{name} is this lambda's own
   name; give every parameter and the result a type so that it may refer to
   itself, or give the lambda another name`.
2. With F4, F5, F8 and this package, the check codes an implementation can
   report are 31: the 28 of `05-types.md` 16.1, `never-conforms`,
   `untagged-union` and `name-clash`.
3. Every other failure of this package uses a template that exists:
   `result-mismatch` `result`, `missing`, `extra`; `arg-mismatch` `argument`,
   `missing`, `extra`, `function`; `maybe-null`; `undefined-name` `type`;
   `duplicate-name`; `bad-type-text` `letter` and `lower`.

### 9.2 The narrowed `parse` codes

The texts of two templates of `suite/messages-parse.json` change, and their
number does not (1.5 point 3). The plan asks that the text of
`type-declaration` name `type Name = {...}`. A template holds no brace but its
holes (`03-host.md` 3.8.2 item 6), so the text says "and then the type". The
alternative is an escape for a literal brace in 3.8.2, a change to Core's
message format.

| Code | Template | Text |
| --- | --- | --- |
| `type-annotation` | `default` | `a lambda stands only as the value of a const line or as an argument, so this type annotates nothing` |
| `type-declaration` | `default` | `{token} does not declare a type here; at the top level of a program, write type Name = and then the type` |

## 10. Earlier cases

1. **Texts this package makes programs.** Seven cases of `suite/errors/`
   expected a `parse` diagnostic for an annotation or a type line, and moved
   to `suite/errors/until-f16-texts.json` (`03-host.md` 3.11 item 4):
   "core-run/parse-follow-on: a typed parameter in an argument",
   "core-run/parse-follow-on: a typed const line" and "core-run/parse-follow-on:
   text a later package covers: type Name = T" of `errors-core-texts.json`;
   and "type-annotation: a parameter in an argument", "type-annotation: a
   const line", "type-annotation: a second parameter" and "type-declaration:
   type" of `parse-targeted.json`. `suite/entry-points/d1-earlier.json` holds
   the same texts with this package's outcome.
2. **Texts the Core amendment changes** (section 5). Four cases of
   `suite/core-run/` moved to `suite/core-run/until-f16-scope.json`: "lambda:
   an inner lambda of the same name calls the outer one" and "lambda: no
   recursion, the name is not in scope in its own body" of `flow-lambda.json`,
   "a const-bound lambda cannot call itself" of
   `static-undefined-function.json`, and "a const-bound lambda's body does not
   see its own name" of `static-undefined-name.json`. Two cases of
   `suite/analysis/` moved to `suite/analysis/until-f16-scope.json`: "A8" of
   `examples.json` and "a lambda's own name is free in its body" of
   `calls.json`. The same texts, with the amended outcome, stand in the Core
   files of 8 item 5, and not in this package's directory: the rule is Core's,
   and an implementation that has it offers this slice (R207).
3. **The three `until-f16-` files were deleted** when every implementation
   offered this slice (`03-host.md` 3.11 item 5), which was in the change
   that added it.
4. **Messages.** The cases whose message is a text of 9.2 carry the new text:
   in `suite/messages-parse.json`, `suite/errors/errors-core-texts.json`,
   `suite/errors/parse-targeted.json` and `suite/errors/parse-precedence.json`.
   Their codes and spans do not change.
4. No other earlier case changes. Every other text of the suite with `type`
   uses it as a name, a key or a call, and no other text writes `:` after a
   parameter or a `const` pattern.

## 11. Slice D3: descriptions, `SchemaOf` and host documentation

Slice D3 of the plan (`17` 3.6 to 3.8). A doc comment, `/** ... */`, before a
record field, a parameter, a `const` line or a type line is a description. A
description is metadata: no rule of the checker or of a run reads it. Two
things read it in this slice: `SchemaOf`, which F1 gains here (11.5), and
hover (11.7). The Entries call of slice D2 will read it when it lands. A host
function, a method and an opaque type gain a `Doc` (11.6).

The slice adds no syntax, node kind, value, type, code, message template,
reserved word or cost row. It adds a field `doc` to the nodes `Const` and
`Param` and to a type line, a description to a field of a record type, the
function `SchemaOf` and a field `Doc` in every binding, the result field
`schema` and the case field `docs` (11.8), and the files of
`suite/entry-points/` whose names begin `d3-` or `tools-d3-`.

It is offered after F17, as F16 slice D3 (1.4 item 1). Slice D2 is not built
yet; it takes its place after slice D3 when it is (`15-build-rulings.md`
R371).

### 11.1 Terms

| Term | Meaning |
| --- | --- |
| Doc comment | A block comment (`01-language.md` 2.2) whose text begins `/**` and that is not `/**/` |
| Description | The text of a doc comment that attaches (11.2), when that text is not empty. A holder has at most one |
| Holder | What a description belongs to: a field of a record type, a `const` line, a parameter, a type line |
| Doc | The documentation a host gives a host function, a method or an opaque type (11.6) |

### 11.2 The doc comment

| Rule | Text |
| --- | --- |
| Attaches to | The next token, when only white space stands between them, and that token begins one of: the key of a field in type text; a parameter; a `const` line; a type line. Otherwise the doc comment is an ordinary comment |
| Type text | A program type (F4 2.1): of an annotation, of a type line, after `as` and `is`. Declared type text (`05-types.md` 3): a variable's type, a named type's type (F4 3), a signature, a method's text, an expected result. At any depth |
| A parameter | Of a lambda, at the first token of its pattern. Of a signature, at its name or its `...`; nothing reads such a description in this slice |
| A `const` line | At the word `const` |
| A type line | At the word `type` |
| Two in a row | The last one attaches. The earlier one is an ordinary comment |
| A line comment, or another block comment, between | The doc comment does not attach |
| Its text | Remove `/**` and `*/`. Split the rest into lines at each line end (`01-language.md` 2.1). From each line remove the leading spaces and tabs, then one `*` if one begins it, then one space if one follows. Remove the trailing spaces and tabs of each line. Drop the empty lines at the start and at the end. Join the lines with U+000A |
| Empty text | No description: `/** */` and `/***/` attach nothing |

1. A doc comment is white space for the grammar, as every comment is. It
   changes no parse, no diagnostic and no span. A text that parses without
   this slice parses the same with it.
2. The formatter keeps a doc comment as it keeps any block comment (F12 2.12).
3. A doc comment in a block or a branch attaches by the same rule: a `const`
   line in a block has a description.

```js
/** A city's weather. */
type Weather = {
  /** The city name, such as "Paris". */
  city: string,
  /**
   * Temperature in degrees.
   * Celsius unless asked otherwise.
   */
  temp: float,
};
```

The field `temp` has the description `Temperature in degrees.⏎Celsius unless
asked otherwise.`, and the type line the description `A city's weather.`

### 11.3 The nodes

| Node | Field added | Holds |
| --- | --- | --- |
| 2 `Const` | `doc` | Its description, or none |
| 23 `Param` | `doc` | Its description, or none |
| A type line (2.3) | `doc` | Its description, or none |

A field of a program type holds its description beside its key. No walker
reads `doc` (`01-language.md` 4).

### 11.4 Descriptions are metadata

A description travels with a field that is passed on unchanged, and with
nothing else. No rule compares two descriptions or combines them, so the three
checkers cannot keep different ones (`17` 3.6, 17-11).

| Rule | Descriptions |
| --- | --- |
| Fits, Equal, size, depth, the walk of F4, conformance, every site and every diagnostic | Not read. Two types that differ only in descriptions are one type |
| Printing (`05-types.md` 3.9), a message, the description of an environment (F1 5.3) | Not written |
| A field passed on unchanged from one source: a read (`05-types.md` 9), narrowing by a fact (10), `filter` and `find` (13.8), Close (F1 4.1), a named type or a type line replaced by its type, a reference replaced by its definition (F17 12.1) | Kept |
| A field of a record that a Join of two records builds (`05-types.md` 4.1; F5 4.1 for the flat record of a union), and a field of a Meet (F1 3.6) | None, even when both sources had the same description and even when the two are one type: in `c ? a : a` no field of the result has one |
| A Join that gives one of its operands as it is: `never` and the other, an alternative that one side has alone, two unions the same by structure (A's union, F5 4.1), and the steps of F17 12.5 item 4 that give A or B (steps 2, 4 and 6) | That operand's |
| Pick (F1 3.6 item 2): a type that steps 1 and 3 give as it is | Kept. A Meet in step 2: none |
| A spread, Merge (`05-types.md` 4.3) | A field that one source gives alone is kept: a required field of the right operand; an optional field of the right operand that the left does not list, beside a left rest of `never`; a field of the left operand that the right does not list, beside a right rest of `never`. A field whose type joins the two sources has none |
| `x as T`, `x is T` narrowing to T, an annotation | The type is T, with T's descriptions |
| A literal's type (`05-types.md` 7): a map literal, a shorthand | None for its own fields. A field's type keeps the descriptions inside it: in `{w: x.w}` the field `w` has none, and the fields of `w`'s type keep theirs |
| A type from a JSON Schema (F1 3) | 11.5 item 5 |
| A type a binding derives from a host type (F1 6) | None in this slice |

1. A description is not part of a type for any rule that compares types. A
   type of the environment and a type of the program that differ only in
   descriptions are Equal (F17 12.4 item 1 among them).
2. A description of a `const` line or a parameter is not part of the type of
   the name it binds.

### 11.5 `SchemaOf`: a type as a JSON Schema

This section replaces F1 5.1 item 3 ("No schema is given back"). Every binding
that offers this slice offers `SchemaOf(T)`, in its own case of names: it
gives a JSON value, a schema, or refuses T. Its text is its JSON form by
`01-language.md` 12: no white space, the keys of every object in code point
order. So two implementations give the same text for one type.

| T | Schema |
| --- | --- |
| `any` | `{}` |
| `never` | `false` |
| `null` | `{"type":"null"}` |
| `bool` | `{"type":"boolean"}` |
| `int` | `{"type":"integer"}` |
| `float` | `{"type":"number"}` |
| `string` | `{"type":"string"}` |
| A literal set of one string s | `{"const":s,"type":"string"}` |
| A literal set of several strings | `{"enum":[...],"type":"string"}`, the strings in code point order |
| `E[]` | `{"items":SchemaOf(E),"type":"array"}` |
| A record | `{"type":"object"}`, with `properties` when it has a field: an object of each field's schema, by key; `required` when a field is required: the required keys, in code point order; and `additionalProperties` by its rest: `false` for `never`, absent for `any`, SchemaOf(R) for any other rest R. So `map<int>` is `{"additionalProperties":{"type":"integer"},"type":"object"}` and `{}` is `{"additionalProperties":false,"type":"object"}` |
| A tagged union (F5) | Each variant is a record, and an alternative of its own for the next row, in the order F5 prints the variants |
| Two or more alternatives | `{"anyOf":[...]}`: the schema of each alternative, in the order of printing (`05-types.md` 3.9; F17 12.6 item 3): bool, number, string, list, the record or the variants of a union, the references in code point order of their names, `null` |
| A reference to a recursive type (F17 12) | `{"$ref":"#/$defs/N"}`, N its name (item 2) |
| An opaque type, anywhere in T | Refused: `bad-type`, "an opaque type has no JSON Schema" |
| A type variable, anywhere in T | Refused: `bad-type`, "a type variable has no JSON Schema" |

1. **A field's description** is the key `"description"` in that field's
   schema, beside its other keys: `{"description":"The city.","type":"string"}`,
   and beside a `$ref`: `{"$ref":"#/$defs/Node","description":"The parent."}`.
   When the field's schema is `false`, a field of type `never`, the description
   is dropped: `false` has no keys. A description of a type line, of a `const`
   line or of a parameter is not written.
2. **Recursive types.** Each recursive type that T reaches, at any depth and
   through the definitions of the references it reaches, is written once,
   under the key N of a top-level `"$defs"`: SchemaOf of its definition. The
   root holds `$defs` beside its own keys when T reaches one. So `Tree`, with
   `type Tree = {kids: Tree[]}`, gives
   `{"$defs":{"Tree":{"additionalProperties":false,"properties":{"kids":{"items":{"$ref":"#/$defs/Tree"},"type":"array"}},"required":["kids"],"type":"object"}},"$ref":"#/$defs/Tree"}`.
3. Two recursive types that T reaches with one name and that are not one
   reference are refused: `bad-type`. A type a checked program gives cannot
   reach two, since its type lines and its environment name their recursive
   types apart (F16 3.1, F17 12.4).
4. **No bounds.** `int` gives no `minimum` or `maximum`. A value outside the
   safe range fails conformance (`not-conforming`), so the host learns of it.
5. **F1 reads `description`.** F1 3.8 gains a row: the `description` of a
   property's own schema object, when it is a string that is not empty, is the
   field's description; also when that object has a `$ref`, as F1 4.3 item 1
   reads `default`. A `description` anywhere else, the target of a `$ref`
   among those places, is not read. So F1 3.12's row for `description` reads:
   read on a property's own schema object, ignored elsewhere.
6. **The law.** For every type T that SchemaOf does not refuse and that holds
   no fill and no mark of F1 4, M(SchemaOf(T)) of F1 3 with no option is a type
   Equal to T (F17 12.5 item 5) with the same descriptions, but for the
   description of a field of type `never`. The checker gives no type with a
   fill or a mark: those come only from a schema mapped with options. A
   recursive type of T that the mapping does not meet again (F17 12.4 item 3)
   comes back written out, not as a reference: in `type Dir = {files: File[]};
   type File = {parent: Dir | null}` both are recursive, and the mapping of
   SchemaOf(Dir) gives Dir with File written out. The two are Equal.
7. **Bounded.** The text of SchemaOf(T) grows with the size of T (`05-types.md`
   20.7), which is at most 100,000, and with the text of its keys, strings and
   descriptions. A recursive type is written once.
8. **Depth.** The law needs F1 to read the schema, and F1 reads a schema of
   depth 64 at most (F1 3.10). A type of depth 64 can give a deeper schema: an
   `anyOf` adds a level the type does not count. Such a type is outside the
   law. SchemaOf does not refuse it, and no case holds one (R383).
9. SchemaOf reads a type and nothing else. It charges no step and no unit, and
   needs no environment.

```js
type Weather = {
  /** The city name, such as "Paris". */
  city: string,
  /** Temperature units. */
  units?: "metric" | "imperial",
};
```

`SchemaOf(Weather)` is
`{"additionalProperties":false,"properties":{"city":{"description":"The city name, such as \"Paris\".","type":"string"},"units":{"description":"Temperature units.","enum":["imperial","metric"],"type":"string"}},"required":["city"],"type":"object"}`.

### 11.6 Host documentation

| Declaration | Gains | Text |
| --- | --- | --- |
| Function (`03-host.md` 3.3.1) | `Doc` | Its documentation, a string. A method is a function, so it has one |
| Opaque type | `Doc` | Its documentation, a string |

1. An empty `Doc` is no documentation.
2. What a `Doc` says is not normative. Where it goes is: hover shows it as
   given (11.7), and so will the Entries call of slice D2.
3. A `Doc` is not in the description of an environment (F1 5.3 item 3 stands):
   a description is for check, and check reads no documentation. An
   environment built from a description has no `Doc`.
4. A child environment keeps the `Doc` of what it inherits, and a replacement
   brings its own.

### 11.7 Hover

These rows change the column `doc` of F12 3.13 item 3, and replace its item 5.
Every other column stays.

| The word is | `doc` |
| --- | --- |
| The name of a `Call` of a host function | The function's `Doc`, or null |
| The word of a `MethodCall` that the check resolved to a method of an opaque type | The method's `Doc`, or null |
| A name a `const` line binds, where its pattern is that name; an `Ident` of it; the name of a `Call` of it; any other `Ident` that names it as a const-bound lambda | The `const` line's description, or null |
| A name a parameter binds, where its pattern is that name; an `Ident` of it; with F17, the name of a `Call` of it | The parameter's description, or null |
| The word of a `Member` | The description of the field the check read: the field of that key in the receiver's record alternative (`05-types.md` 9.2), null taken out and a reference unfolded. Null when there is none |
| A type name, or the name of a type line | The type line's description, when the name stands for a type line (7.3 item 1); the `Doc` of an opaque type of the environment; else null |

1. A name bound inside a pattern that is not a name, `const {a, b} = x;`, has
   no description: its `doc` is null.
2. The description of a field is that of the receiver's type as the check
   gives it, facts in force: a receiver of type `c ? a : b` reads a field of a
   Join, and gives null (11.4).
3. A library record's `doc` is given as before.

### 11.8 The cases

**The result field `schema`.** This row is added to the result table of
`03-host.md` 3.10.6.

| Field of a result | Compared with | How |
| --- | --- | --- |
| `schema` | SchemaOf of the type the check gives (11.5) | null: SchemaOf refuses the type. Otherwise as JSON data, as F1 7.2 compares a description. Then the law (11.5 item 6) |

1. A case with `schema` is run checked, once, as one with `type` is
   (`03-host.md` 3.10.8 item 5). `schema` in an expected failure, or in a case
   with `"check": false`, makes the file invalid.
2. **The law.** When the schema found is not refused, the runner maps it by F1
   3 with no option. The type it gives must be Equal to the type the check
   gave. And SchemaOf of it must be the schema found, once each `$ref` to an
   entry of `$defs` that SchemaOf of it does not hold is replaced by that
   entry (with the `description` beside the `$ref` kept), and those entries
   are dropped (11.5 item 6). The alternatives of each `anyOf` are compared in
   any order there, since a `$ref` written out can change their order of
   printing. Otherwise the case fails.
3. The difference is named `schema`. It stands after `type` in the order of
   3.10.8 item 2.

**The case field `docs`.** This row is added to the table of `03-host.md`
3.10.3.

| Field | Holds | When absent |
| --- | --- | --- |
| `docs` | An object. Each key is the name of a host function the case declares in `functions`, a `Type.method` it declares in `methods`, or an opaque type it declares in `opaque`. Each value is a string, that declaration's `Doc` (11.6) | No declaration has a `Doc` |

1. `docs` may stand where `functions` may: in the header, in a program case,
   and in a completion and a hover case (F12 7.2 item 1). A format case and a
   render case take none. Only hover reads it.
2. A key that names no declaration of the case, and a value that is not a
   string, make the file invalid.

| Path | Holds |
| --- | --- |
| `suite/entry-points/d3-schema.json` | SchemaOf (11.5) of the types of programs and of declared type text, with the law, and its refusals |
| `suite/entry-points/d3-descriptions.json` | Doc comments (11.2) and the travel of descriptions (11.4), seen through SchemaOf |
| `suite/entry-points/tools-d3-hover.json` | Hover (11.7), with `docs` (F12) |

The runner of 8 runs them. A runner that offers F17 and not this slice does
not run the files named `d3-` or `tools-d3-`, and accepts neither `schema`
nor `docs` (`03-host.md` 3.11 item 3).

### 11.9 What this slice changes in earlier rules

| Earlier rule | With this slice |
| --- | --- |
| `01-language.md` 4.1, nodes 2 and 23; F16 2.3 | The field `doc` (11.3) |
| `03-host.md` 3.3.1 | `Doc` on a function and an opaque type (11.6) |
| `03-host.md` 3.10.3, 3.10.6 | The case field `docs`, the result field `schema` (11.8) |
| F1 3.8, 3.12 | `description` is read (11.5 item 5) |
| F1 5.1 item 3 | `SchemaOf` (11.5) |
| F12 3.13 item 3, the column `doc`, and item 5 | 11.7 |
| `15-build-rulings.md` R114, R188 | Reversed, as D53 planned |

No other rule changes. A program with no doc comment has the type, sites,
diagnostics, value and usage it has without this slice.

## 12. Slice D2: `export const` entry points and the Call stage

Slice D2 of the plan (`17` 3.3, 3.5). A top-level `const`-bound lambda marked
`export` is an **entry point**. A program that has at least one `export const`
may omit its result expression. A host lists a program's entry points with
`Entries` and calls one by name — positionally or by record — through the Call
stage, as one bounded run with conformed arguments and nothing kept between
calls.

This slice adds the field `exported` to node 2 (`Const`), allows `Program`'s
`result` to be absent, adds the static check code `bad-export` (four templates
in `suite/messages-entry-points.json`), narrows the parse code
`export-statement` (updating its text in `suite/messages-parse.json`), adds the
`declare` codes `no-result` and `no-input-record`, adds the Call stage and
`Entries` to every binding, adds the case fields `entry`, `args`, `input`,
`entryReturns` and the result field `entries` (12.10), and adds the files of
`suite/entry-points/` whose names begin `d2-` or `tools-d2-`.

It is offered after F19 and before F12, as F16 slice D2 (1.4 item 1).

### 12.1 Terms

| Term | Meaning |
| --- | --- |
| Export line | `export const ...;` at the top level of a program |
| Entry point | An export line whose pattern is a name other than `_`, that has no `const` annotation, and whose value is a lambda |
| Needs | The top-level `const` lines an entry point refers to, directly or through other top-level `const` lines (12.4) |
| Input type | The record type that a call by record conforms its input value to, or absent when no input record exists (12.5) |
| Call stage | Calling one entry point of a compiled program, positionally or by record (12.5) |

### 12.2 Syntax

The productions `Program`, `Top` and `Export` extend 2.1:

```ebnf
Program  = { Top } [ Expr [ ";" ] ] .
Top      = Const | Export | TypeLine .
Export   = "export" Const .
```

1. **`export` is contextual.** It is not a reserved word (`01-language.md` 2.4
   item 3). At the start of a top-level line of a `Program`, where `const` and
   `TypeLine` may stand, the word `export` begins an `Export` **exactly when
   the next token is the word `const`**. Elsewhere `export` is an ordinary
   name: `const export = 1; export`, `export(1)` and `export.a` remain valid
   programs, and `export default 1` or `export const` inside a block or branch
   stays `export-statement` (E18).
2. **Optional result expression.** A `Program` may omit `Expr` (reaching end of
   input directly after `{ Top }`) **only when `{ Top }` contains at least one
   `Export`**. Without an `Export`, reaching end of input where `Expr` must
   begin reports `unexpected-token` at end of input, as before. When `Expr` is
   omitted, no trailing `;` may follow `{ Top }`: `export const f = (x: int) =>
   x;;` reports `unexpected-token` at the second `;`.
3. **Nodes.** Node 2 (`Const`) gains the boolean field `exported` (`true` for
   an `Export`, `false` for a plain `Const`). Node 1 (`Program`)'s `result`
   field may be absent. No node kind is added.
4. **Spans.** The span of an exported `Const` runs from `export` to its `;`.
   When a `Program` has no `result`, its span runs from its first token to the
   `;` of its last top-level line.
5. **Doc comments.** For an `export const` line, a doc comment (11.2) attaches
   at the word `export`, not at the word `const`. A doc comment between
   `export` and `const` is an ordinary comment and does not attach. The `Const`
   node's `doc` holds the description attached to `export`.
6. **Levels of nesting.** `export` opens no level (`01-language.md` 13.1).
7. **Text that stays an error.**

| Text | Core code | Span | With F8 |
| --- | --- | --- | --- |
| `export` followed by something other than `const` at the top level: `export default 1`, `export foo = 1` | `unexpected-token` | The token after `export` | `export-statement` at `export` (E18), except `export = 1` which stays `assignment` (E7 before E18) |
| `export const` in a block or a branch: `{ const x = 1; export const a = () => x; a() }`, `if (c) { export const a = () => 1; a() } else { 0 }` | `unexpected-token` | `const` | `export-statement` at `export` (E18); note `{ export const a = 1; a }` starts a map literal (not a block) and stays `unexpected-token` at `const` |
| `export const _ = x => x;` or `export const {a} = x => x;` | `unexpected-token` | `=>` | `lambda-in-pattern` (3.4 rule 9) |
| `export const f: int = x => x;` | `unexpected-token` | `=>` | The same (2.5) |
| `const a = 1;` or `type Foo = int;` with no `export` and no result | `unexpected-token` | End of input | The same |
| `export const f = (x: int) => x;;` | `unexpected-token` | The second `;` | The same |

### 12.3 Static rules and the checker

#### Static rule `bad-export`

`bad-export` is a static rule (`05-types.md` 22): it is reported in phase
`check` in every compile, with or without the type rules (`NoCheck`).

| Condition | Template | Span | Params |
| --- | --- | --- | --- |
| An `export const` line whose pattern is not a `NamePat` (or is `_`), or that carries a `const` annotation (`const x: T = ...`), or whose value is not a `Lambda` | `not-lambda` | The `Const` node | `name`: the bound name when the pattern is a `NamePat` other than `_`, else `"this line"` |
| A parameter of an exported `const`-bound lambda has neither a program type (`type`) nor a function type (`fn`) annotation | `unannotated` | The `Param` node | `name`: the entry point's name; `param`: the parameter's name when its pattern is a `NamePat` other than `_`, else its 1-based position (`"1"`, `"2"`, ...) |
| A parameter of an exported `const`-bound lambda has a function type (`fn`) annotation | `fn-parameter` | The `Param` node | `name`: the entry point's name; `param`: the parameter's name |
| In a program that has at least one `export const` line, a top-level `const` line that is not marked `export` and whose pattern binds no name (`_`, `{}`, `[]`, `[_, _]`, etc.) | `binds-nothing` | The `Const` node | None |

1. An `export const` line that fails `not-lambda` (such as `export const _ =
   1;`) does not also report `binds-nothing`. Its value and pattern are still
   walked by the other static rules (`undefined-name`, `duplicate-name`,
   `shadows-namespace`, and so on).
2. `binds-nothing` applies only at the top level of a program that has at least
   one `export const` line. A `const _ = ...;` inside a block or branch of an
   entry point or helper is allowed: that block runs when called.
3. A parameter whose annotation has a static error (such as `undefined-name`
   for an unknown type name) still carries an annotation and does not report
   `bad-export` `unannotated`.
4. An entry point with zero parameters (`export const ping = () => "pong";`)
   satisfies the annotation rule: every parameter carries a type.
5. **Static codes.** For `03-host.md` 3.10.8, 3.10.10 item 17 and 3.10.13, in
   `suite/entry-points/` and `suite/messages-entry-points.json`, `bad-export`
   joins the static codes of 3.1 item 11: `untagged-union`, `check-limit`,
   `name-clash` and `bad-export`.

#### Type checking

1. **Checked once.** Every entry point whose parameters all carry program type
   annotations — including one with zero parameters and no result annotation,
   `export const ping = () => "pong";` — is **checked once** (3.4), where it is
   written, under the top-level facts (empty), with its sites kept, even when
   nothing in the program calls it. Its result type is its result annotation
   when present, else its body's type. It counts as one check against
   `05-types.md` 20.3 and its nesting against 20.2.
2. **Still a `const`-bound lambda.** Other `const` lines, other entry points,
   and the result expression may call an entry point by name or pass it to a
   library entry, as any `const`-bound lambda checked once (3.4).
3. **Program with no result expression.** When `Program.result` is absent, the
   checker checks all type lines and `const` lines as usual and gives the
   program's result type as `never`.
4. **Expected result of the program.** Calling `Check` or `Compile` (checked or
   with `NoCheck`) on a program with no result expression with an expected
   result (`Expect`, `ExpectType`, `ExpectSchema` or `Condition`; in a case
   file, `returns` or `returnsSchema`) fails with phase `declare`, code
   `no-result` and no span. Invalid type text or schema in the option is still
   `declare` `bad-type`.
5. **Expected result of an entry point.** `Check` and `Compile` accept an
   expected result type for each named entry point (in type text, as a `Type`,
   or by JSON Schema with the options of F10 2.1; not `condition`).
   - Invalid type text or schema in the option is `declare` `bad-type`.
   - A name that is not an entry point of the program (not an exported
     `const`-bound lambda, including a private `const` lambda or an unknown
     name) is `declare` `bad-name`, in both checked and unchecked (`NoCheck`)
     compiles.
   - When checked, after the entry point is checked once (giving its result
     type T), T is fitted through its form (`05-types.md` 15.1, allowing `any`
     with a `dynamic` site as `returns` does) against the expected type at the
     entry point's body (`lam.Body`), reporting `result-mismatch` (`result`,
     `missing`, `extra`) at each place of failure. Passing the pair bound of
     F17 12.5 reports `check-limit` `pairs` at `lam.Body`.
   - When compiled with `NoCheck`, the option's type text and entry name are
     still validated (`bad-type`, `bad-name`), and the fit is skipped.

### 12.4 Needs and evaluation of an entry call

This section extends `02-evaluation.md` for the Call stage.

1. **Needs.** For an entry point `e`, its **direct references** are the
   top-level `const` lines whose bound name(s) or `const`-bound lambda are
   referred to (by lexical resolution, `01-language.md` 5) anywhere in `e`'s
   lambda (its body, its parameter defaults, and defaults inside its parameter
   patterns). For any top-level `const` line `k`, `k`'s direct references are
   the top-level `const` lines referred to anywhere in `k`'s value (for a
   lambda, its body and defaults) or defaults in `k`'s pattern. The **needs**
   of `e` are the transitive closure of direct references starting from `e`
   (excluding `e` itself).
   - A `const` line whose pattern binds several names is needed when any of its
     names is referred to.
   - A shadowed local binding inside a block or lambda does not refer to a
     top-level `const` line.
   - Type lines and annotations are never evaluated and create no needs.
   - Needs are determined purely from the source text, so every implementation
     evaluates the exact same lines in the exact same order.
2. **Evaluation order.** A call of entry point `e` executes as one bounded run
   under the compiled program's environment limits:
   1. Evaluate each top-level `const` line in `e`'s needs **that is not a
      lambda**, in text order, as a program evaluates it (`02-evaluation.md`).
      Evaluate no other top-level `const` line, no top-level lambda `const`
      line (their definitions charge 0 steps), and not the result expression.
   2. Charge **1 step** for calling the entry point, and add **1** to the depth
      count (testing `depth + 1 > maxDepth`, which fails with `depth-exceeded`
      if exceeded).
   3. Bind the arguments to `e`'s parameters in parameter order, as a call by
      name does (`01-language.md` 8.3): each supplied host argument is read
      (`03-host.md` 3.4.4, 3.4.5); an omitted argument, or a `null` argument
      for a parameter with a default, evaluates that default in parameter order
      with earlier parameters in scope.
   4. Evaluate `e`'s body. Its value is the call's value.
   5. Subtract 1 from the depth count when `e` returns.
3. **Trace.** The host's call of the entry point adds no entry to `trace` (F8
   4.1): there is no `Call` node in the source text for it. Calls made inside
   `e` or inside its needed lines add trace entries as usual.
4. **Isolation.** Nothing is kept between calls. Each call evaluates `e`'s
   needs again with its own limits, usage counters, variables and cancellation
   signal. Several calls on one compiled program may run concurrently
   (`03-host.md` 3.2.2).
5. **Running a program with no result expression.** Calling `Run` or `Measure`
   on a compiled program whose `result` is absent fails with phase `declare`,
   code `no-result`, no span, and usage of 0 steps and 0 allocation units.

### 12.5 The Call stage and the input record

`03-host.md` 3.2.1 gains the fifth stage, **Call**:

| Stage | Takes | Gives | Fails with a list of diagnostics of phase |
| --- | --- | --- | --- |
| Call | A runnable program, an entry point's name, arguments (positional list or one input record), a supply of variables, and a cancellation signal | A value, or diagnostics; and the usage (3.6.3) in both cases | `declare` (`bad-name`, `arity`, `no-input-record`), or `run` (argument `not-conforming` failures or a failure of the run) |

#### The input record

Every entry point has an **input type**, or none, determined from its parameter
list:

| The entry point has | Its input type |
| --- | --- |
| Exactly one parameter, whose resolved type (unfolding recursive references) is a record with no other alternative (not a tagged union, not nullable, not mixed; `map<T>` and a recursive record type `Node` qualify) | That parameter's type (keeping a recursive reference `Node` as written so `SchemaOf` writes `$defs`/`$ref`). The single object passed to a record call is that parameter's argument, even when the parameter's pattern is a destructuring pattern or `_` |
| Any other list of parameters (0 parameters, 2 or more parameters, or 1 parameter whose type is not a single record alternative), where every parameter's pattern is a name other than `_` | An exact record (`rest = never`) with one field per parameter, keyed by the parameter's name, with the parameter's type, marked optional when the parameter has a default, and carrying the parameter's description (11.2) as the field's description. For 0 parameters, `{}` |
| Any other list of parameters where at least one parameter's pattern is not a name (a destructuring pattern or `_`) | None (no input record) |

#### Positional call

Calling entry point `name` with positional argument list `args`:

1. If `name` is not an entry point of the program: fail with phase `declare`,
   code `bad-name`, no span, usage 0 steps / 0 units.
2. Let `min` be the highest 1-based index of a parameter with no default (or 0
   if every parameter has a default), and `max` be the number of parameters. If
   `len(args) < min` or `len(args) > max`: fail with phase `declare`, code
   `arity`, no span, usage 0 steps / 0 units.
3. **Argument conformance.** For each `i` from `0` to `len(args) - 1` in order,
   conform `args[i]` against `T_i` — where `T_i` is parameter `i`'s annotated
   type, or `T_i | null` when parameter `i` has a default — using F8's
   `ConformAll` (default step limit 1,000,000, default depth limit 200),
   collecting at most **100** diagnostics across all arguments in order.
   - Each `not-conforming` diagnostic has phase `run`, class `value`, no span,
     empty trace, and its `path` prefixed with `i` (the 0-based integer index
     of the argument).
   - A limit diagnostic (`steps-exceeded` or `depth-exceeded`) has phase `run`,
     class `limit`, no span, no path, empty trace, and stops further
     conformance immediately.
   - If any diagnostic is produced, end the call with those diagnostics and
     usage 0 steps / 0 units.
4. Otherwise evaluate the entry point by 12.4, with parameters `0..len(args)-1`
   present and `len(args)..max-1` omitted.

#### Call by record

Calling entry point `name` with one input value `input`:

1. If `name` is not an entry point of the program: fail with phase `declare`,
   code `bad-name`, no span, usage 0 steps / 0 units.
2. If the entry point has no input record: fail with phase `declare`, code
   `no-input-record`, no span, usage 0 steps / 0 units.
3. **Input conformance.** Conform `input` against the entry point's input type
   using F8's `ConformAll` with maximum **100** (default step limit 1,000,000,
   default depth limit 200). Each `not-conforming` diagnostic has phase `run`,
   class `value`, no span, empty trace, and its `path` from `ConformAll`
   directly; a limit diagnostic has class `limit` and no path. If any
   diagnostic is produced, end the call with those diagnostics and usage 0
   steps / 0 units.
4. **Argument extraction.**
   - Row 1 (single record parameter): one argument, `input`, present.
   - Row 2 (exact record by parameter name): for each parameter `i` with name
     `k`, if `input` has key `k`, pass its value as present; if key `k` is
     absent from `input`, mark parameter `i` omitted (so its default runs).
5. Evaluate the entry point by 12.4.

#### Unchecked compiles

Parameter annotations are resolved by the static rules (3.1), so an entry
point's parameter types and input type are known even in a program compiled
with `NoCheck`. Argument conformance (step 3 of positional call and call by
record) runs in both checked and unchecked compiles.

### 12.6 `Entries` and the binding surface

Every binding offers `Entries` on a compiled `Program` (and on a `CheckResult`)
returning one record per entry point in text order:

| Field | Holds |
| --- | --- |
| `name` | The entry point's `const` name |
| `doc` | The entry point's description (from the doc comment before `export`), or empty/null when none |
| `params` | For each parameter, in written order: `name` (its name, or empty/null when its pattern is a destructuring pattern or `_`), `type` (its annotated `Type`), `doc` (its description, or empty/null when none), and `optional` (`true` when it has a default) |
| `inputType` | Its input `Type` (12.5), or absent/null when it has no input record |
| `resultType` | Its checked result `Type`, or absent/null for a program compiled with `NoCheck` |

Every binding also offers:
- Positional call (`call` and `callMeasure`, returning value, or value and
  usage).
- Call by record (`callRecord` and `callRecordMeasure`).
- Per-entry expected result options on `Check` and `Compile`.

### 12.7 Analysis (F2)

1. An `export const` line is analyzed as a `const`-bound lambda (section 6):
   its name is bound in its own body and defaults, and in later lines and the
   result.
2. A program with no result expression is never a literal (`literal` is null);
   its `names`, `refs` and `calls` are collected from its top-level `const`
   lines in order.

### 12.8 Tools (F12)

#### Formatter

These rules add to F12 2.8 and section 7.1:

| Node | Document |
| --- | --- |
| `Const` with `exported` true | `"export" sp` followed by the document of the `Const` (`"const" sp ...`) |
| `Program` with no `result` | The top-level lines (type lines, `const` lines and `export const` lines in text order) separated by `H`, preserving blank lines (F12 2.9) and comments (F12 2.12), followed by `H` (one trailing line feed) |

1. The word `export` is a kept token (F12 2.12 item 1): comments before
   `export` or between `export` and `const` stay in place.
2. Every formatting law of F12 2.2 holds for programs with `export const` and
   programs with no result expression.

#### Completion

1. Completion repairs and analyzes programs with `export const` and programs
   with no result expression by the rules of F12 3.3 and section 7.2.
2. An exported `const`-bound lambda is in scope in its own body and in later
   lines and the result as kind `lambda` (7.2 item 5).
3. A hole in the name of an `export const` line has no context (empty list).
4. A hole in an annotation of an `export const` line is in the `types` context.

#### Hover

1. Hovering on an `export const` entry point's name — at its declaration, at a
   call, or as a lambda argument — follows 11.7, with `doc` equal to the
   description attached to `export` (12.2 item 5), or null.
2. Hovering on a parameter of an `export const` entry point follows 7.3 item 4
   and 11.7.
3. Hovering on the word `export` gives no hover (null).
4. Hover works on programs with no result expression.

### 12.9 Messages and codes

#### `suite/messages-entry-points.json`

This entry is added to `suite/messages-entry-points.json` (9.1):

| Code | Phase | Template | Text | Params |
| --- | --- | --- | --- | --- |
| `bad-export` | `check` | `not-lambda` | `export marks a const-bound lambda; {name} is not one` | `name`: word |
| | | `unannotated` | `parameter {param} of the entry point {name} needs a type` | `param`: word, `name`: word |
| | | `fn-parameter` | `parameter {param} of the entry point {name} is a function, which a host cannot pass` | `param`: word, `name`: word |
| | | `binds-nothing` | `a const line that binds no name never runs in a program with an export; bind a name an entry point uses, or remove the line` | |

With this slice, the check codes an implementation can report are 33 (Core's
28, `never-conforms`, `untagged-union`, `name-clash`, `circular-type`,
`bad-export`), with 90 check templates in all.

#### Narrowed `parse` code

The text of `export-statement` in `suite/messages-parse.json` changes in place
(as 9.2 changed `type-annotation` and `type-declaration`):

| Code | Template | Text |
| --- | --- | --- |
| `export-statement` | `default` | `export marks only a top-level const-bound lambda; write export const at the top level of a program` |

The count of `parse` codes stays 62, with 103 templates.

#### `declare` codes

This slice adds two `declare` codes: `no-result` and `no-input-record`. Unknown
entry point names use the existing `declare` code `bad-name`; wrong positional
argument counts in the Call stage use phase `declare` with code `arity`. As
with all `declare` diagnostics (`03-host.md` 3.7.2), their messages are not in
the catalog.

### 12.10 The cases

#### Case fields

These rows add to `03-host.md` 3.10.3:

| Field | Holds | When absent |
| --- | --- | --- |
| `entry` | The name of the entry point to call, a string | The case runs the program's result expression (3.10.5) |
| `args` | A JSON array of host values (supporting `{"$opaque": ...}`): positional arguments for `entry` | With `entry` and no `input`, an empty argument list `[]` |
| `input` | One JSON host value (supporting `{"$opaque": ...}`): the input record for `entry` | Positional call |
| `entryReturns` | An object mapping entry point names to expected result type texts (strings) | No per-entry expected result |

1. Each of the four fields may appear in the header or in a case. `args` and
   `input` count as one field for the header rule (`03-host.md` 3.10.2): a case
   that gives either replaces the header's `args` or `input`.
2. `args` or `input` without `entry`, or both `args` and `input` in one case,
   makes the file invalid.
3. None of these four fields may appear in a `conform`, `render`, `runner` or
   F12 tools case.
4. When `entry` is present, step 5 of `03-host.md` 3.10.5 always runs (calling
   the entry point with `args` or `input`, instead of running the program's
   result). Fixtures of `vars` and stub `result`s are still tested by F9 2.2;
   `args` and `input` are not fixtures and are tested by the Call stage itself.
5. In a case with `entry`, `"span": null` is allowed in an expected diagnostic
   of phase `run` (for argument conformance failures, as F10 10.2 allows it for
   a load).

#### Result field `entries`

This row adds to the result table of `03-host.md` 3.10.6:

| Field of a result | Compared with | How |
| --- | --- | --- |
| `entries` | `Entries` of the checked program (12.6) | Exact length and order; each element compares the fields present |

Each element of `expect.entries` is an object with:

| Field | Required | Compared with |
| --- | --- | --- |
| `name` | Yes | Entry `name`, equal |
| `doc` | No | Entry `doc` (`null` means no description), equal |
| `params` | No | Exact list of `{name (string or null), type (type text, compared by structure), doc (optional string or null), optional (bool)}` |
| `input` | No | Entry `inputType` (`null` means no input record; string compared by structure as `expect.type`) |
| `inputSchema` | No | `SchemaOf(inputType)` (`null` means no input record or SchemaOf refusal; otherwise JSON data and the round-trip law of 11.8 item 2) |
| `result` | No | Entry `resultType` (string compared by structure as `expect.type`) |

1. A case with `expect.entries` is run once, checked (as `type` and `schema`
   are). `entries` in an expected failure, or in a case with `"check": false`,
   makes the file invalid.
2. The difference is named `entries`. It stands after `schema` in the order of
   `03-host.md` 3.10.8 item 2.

#### Suite files

| Path | Holds |
| --- | --- |
| `suite/entry-points/d2-syntax.json` | Grammar of `export const`, contextual `export`, optional result expression, narrowed `export-statement`, parse errors |
| `suite/entry-points/d2-earlier.json` | The earlier text `export const a = 1; a` with slice D2's outcome (`bad-export`) |
| `suite/entry-points/d2-static.json` | All four templates of `bad-export`, scope and shadowing with `export const`, static checking with `NoCheck` |
| `suite/entry-points/d2-check.json` | Type checking of entry points (checked once, sites kept, zero-param entry points, `entryReturns`, `no-result`, `bad-name`) |
| `suite/entry-points/d2-run.json` | The Call stage: positional and record calls, defaults, needs evaluation and order, unused lines skipped, steps/units/depth, traces, conformance failures, `no-result`, `no-input-record`, `bad-name`, `arity`, opaque arguments |
| `suite/entry-points/d2-entries.json` | `Entries`: names, descriptions, parameters, input types, result types, and `SchemaOf(inputType)` with parameter and nested field descriptions |
| `suite/entry-points/d2-analysis.json` | Analysis (F2) of programs with `export const` and programs with no result expression |
| `suite/entry-points/tools-d2-format.json` | Formatter (F12) for `export const`, comments, and programs with no result expression |
| `suite/entry-points/tools-d2-assist.json` | Completion and hover (F12) for `export const`, descriptions, and programs with no result expression |

#### Earlier cases

1. One case of `suite/errors/errors-core-texts.json` (`"core-run/parse-follow-on:
   text a later package covers: export const a = 1; a"`) is replaced by
   `suite/entry-points/d2-earlier.json` with slice D2's outcome (`bad-export`,
   template `not-lambda`).
2. `suite/messages-parse.json` (`export-statement`) and
   `suite/errors/parse-targeted.json` (`export-statement: export default`)
   carry the new `export-statement` template text of 12.9, and the catalog case
   of `export-statement` uses `export default 1`.

### 12.11 What this slice changes in earlier rules

| Earlier rule | With this slice |
| --- | --- |
| `01-language.md` 3.1, 4.1, 4.3, 14.3; F16 2.1, 2.3 | `Export`, `Const.exported`, optional `Program.result`, `export` contextual word (12.2) |
| `02-evaluation.md` | Needs and evaluation of an entry call (12.4) |
| `03-host.md` 3.2.1, 3.7.3, 3.10, 3.11 | The Call stage, `Entries`, `no-result`, `no-input-record`, case fields and package order (12.5, 12.6, 12.10) |
| `05-types.md` 16.2, 22; F16 3.1, 3.4 | `bad-export` static rule, entry points checked once, per-entry expected results (12.3) |
| F2 3 | Analysis of `export const` and result-less programs (12.7) |
| F8 2.7 E18, 3.1, 3.4 | Narrowed `export-statement`, new text, 90 check templates (12.2, 12.9) |
| F12 2.8, 3 | Formatting, completion and hover for `export const` and result-less programs (12.8) |
| F16 11.2, 11.7 | Doc comment attaches at `export`; `Entries` reads descriptions (12.2, 12.6, 12.8) |

No other rule changes. A program with no `export const` has the type, sites,
diagnostics, value and usage it has without this slice.
