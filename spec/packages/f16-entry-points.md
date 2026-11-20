# F16. Entry points, slice D1: annotations and type lines

This chapter is normative. It specifies follow-on package F16 as far as slice
D1 of its plan (`experiments/v3/design/17-entry-points.md`, and
`experiments/v3/design/14-implementation-plan.md` 5.3, track D): type lines,
annotations on `const` lines, on the parameters of a lambda and on its
result, and the syntax of a function type on a parameter. It adds fields to
the nodes `Program`, `Const`, `Lambda` and `Param`, one check code,
`name-clash`, one template of `arg-mismatch`, the catalog file
`suite/messages-entry-points.json`, the hover kind `type` of F12, and the
directory `suite/entry-points/`. It adds no node kind, no value, no type, no
run code, no `parse` code, no reserved word and no case field.

Slices D2 and D3 of the plan are not in this chapter yet: `export`, entry
points, the stage that calls one, programs with no result; doc comments, and
the description of a type. Each adds to this chapter when it lands.

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
   F13, F14, F16 slice D1, F17, F12. **The place of slice D1 is open:** it is
   the recommendation of `15-build-rulings.md` R211, which put F16 before
   F17, and the owner has not confirmed how the slices of F16 and the steps
   of F17 are offered (`14` 5.3).
2. An implementation that offers this package offers F4: an annotation is a
   program type of F4, resolved by F4's rules. It offers F8, so a case of this
   package may state messages.
3. The cases of each slice stand in their own files. Those of slice D1 are the
   files of `suite/entry-points/` whose names begin `d1-` or `tools-d1-` (8).
4. **An implementation that offers F17 or F12 and not yet this slice** does
   not run `suite/entry-points/` or load `suite/messages-entry-points.json`,
   and it does not pass the new Core cases of section 5: the Core amendment
   lands with this slice. The cases it would still pass stood in files named
   `until-f16-` (`03-host.md` 3.11 item 4). All three implementations offered
   this slice in the change that added it, so those files were deleted
   (3.11 item 5; section 10).

### 1.5 Open points

These are written to the recommendation, and wait for the owner.

| # | Point | Recommendation | Strongest alternative |
| --- | --- | --- | --- |
| 1 | Whether a narrowed `parse` code retires | None retires: `type-annotation` and `type-declaration` each still cover text that stays an error (2.5), so the count of `parse` codes stays 58 | Retire a code whose main text became a program |
| 2 | The place of slice D1 in the order of 1.4 | Before F17 | After F17, beside the other slices of F16 |
| 3 | The new texts of `type-annotation` and `type-declaration` | Changed in place in `suite/messages-parse.json`, so the count of templates stays 97 (9.2). A build with F8 and without F16 prints them too | New templates in this package's catalog file. That changes the message of two catalog cases of `messages-parse.json` in an implementation that offers F16, and 3.11 item 4 has no file named `until-` for a catalog |
| 4 | A fit at an annotation that uses `any` (3.3) | It does not hold: the value must surely fit | It holds with a `dynamic` site, as at an argument of Core; then the name has the annotation's type while its value may be anything, and a later failure is at no site (`05-types.md` 18.4, promises 1 and 4) |

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
   inner `Tree`. Recursive types are step E2 of F17.
7. **An annotation's type** is resolved the same way, in the type lines before
   the `const` line or the result that holds it. So in `const f = (x: Late)
   => x; type Late = int; f(1)`, `Late` is `undefined-name`.
8. A type name and a value name are in two scopes, and never clash:
   `type Order = {id: string}; const Order = 1; Order` reads the value.
9. **An `Fn` parameter keeps Core's static rules** until step E3 of F17: it is
   a value binding (`05-types.md` 22.7). A call of it is `call-of-parameter`, a
   lambda given at it in a call of its lambda is `lambda-to-lambda`, and the
   name given as a lambda argument is a value.
10. **The reference of a lambda to its own name** is Core's rule
    `recursion-needs-types` (section 5). With this package alone every
    reference is that error, whether or not the lambda's parameters and
    result are annotated. Package F17, which follows this slice in the order
    (1.4), accepts a reference of a lambda whose every parameter and result
    are annotated (`spec/packages/f17-recursion.md` 11.2).
11. **Static codes.** For `03-host.md` 3.10.8, 3.10.10 item 17 and 3.10.13, in
    `suite/entry-points/` the static codes are Core's twelve, `untagged-union`
    and `check-limit` at a program type (F4 6.1 item 4), and `name-clash`.

### 3.2 A compile without the type rules

1. An annotation is a claim. A compile without the type rules applies 3.1 and
   ignores every claim: it binds what the value is, and runs no test. So
   `const x: int = "a"; x` gives `"a"`.
2. An `Fn` annotation is read by every compile (`15-build-rulings.md` R208).
   In this slice it changes no rule: its parameter keeps Core's static rules
   (3.1 item 9). So `const g = (f: (x: int) => int) => f; g(1)` gives 1 in a
   run without the type rules; the checker reports `arg-mismatch` at the `1`
   (3.3).

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
| An argument of a call by name at `(x: T)` | The argument | T; with a default, `T \| null` | `arg-mismatch`: `argument`, `missing`, `extra`; or `maybe-null` when it fits but for null | The same |
| A plain argument of a call by name at `(f: (...) => R)` | | | `arg-mismatch`, template `function`: `param` the parameter's name, `name` the lambda's | The argument |
| A value the library passes to `(x: T)` (3.4) | Q | T | `arg-mismatch`, template `argument` | The annotation |
| A value the library passes to `(f: (...) => R)` | Q | | `arg-mismatch`, template `fn-value`: `name` the function, `found` Q | The function type |

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
| **Checked once**: a `const`-bound lambda with at least one annotation, and every parameter annotated | Checked once, where it is written, under the facts there. Each parameter is bound to its annotation; a parameter with a function type, or with an annotation that has a static diagnostic, is bound to `never`. Each default and the body's fit to the result annotation are 3.3's. Its sites are kept, also when it has no use. The check counts as one check against `05-types.md` 20.3, and its nesting where it is written against 20.2; a `check-limit` of either is on the lambda, the whole of it (2.3). Its **result type** is its result annotation, or the body's type |
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
2. A type line's type is written on one line, however long (F12 2.13).
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
| `arg-mismatch` | `check` | `fn-value` | `{name} passes {found} to a parameter that takes a function` | `name`: word, `found`: type |

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
