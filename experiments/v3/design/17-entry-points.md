# Risor v3 design: entry points, annotations, descriptions and schemas

This plan is D53. It answers friction 3 and 4, proposals P3, P4 and P7 of issue
#640, and question 1 of issue #641. The owner settled the direction on
2026-11-14: a host may call a top-level `const`-bound lambda by name, as one
bounded run, with arguments conformed to annotated parameter types. The
details below are recommendations unless a sub-decision says otherwise. The
owner settled 17-2, `export const`, the same day.

Plans `18` (recursion and recursive types), `19` (host calls and replay) and
`20` (author feedback) are written beside this one. Section 7 lists where they
meet.

## 1. Summary

1. **`export const`** marks a top-level `const`-bound lambda as an **entry
   point**. A host lists a program's entry points and calls one by name. Each
   call is one run, with its own limits and usage.
2. **Annotations.** A lambda's parameters and its result may carry a type:
   `(city: string, units: "metric" | "imperial" = "metric"): Weather => ...`.
   A `const` line may carry one: `const o: Order = x;`. A program may declare
   **type lines**: `type Order = {id: string, total: float};`. This brings the
   part of D40 that was timed for a minor version into the release that ships
   this plan.
3. **An entry call evaluates only what the entry needs.** It evaluates, in text
   order, the top-level `const` lines the entry depends on, then calls the
   lambda. Nothing is kept between calls.
4. **A program may have no result expression** when it has at least one entry
   point. A tool file is then a list of tools.
5. **Descriptions.** A doc comment, `/** ... */`, before a field of type text,
   a parameter, a `const` line, a type line, or in a host declaration, is a
   description. It is metadata. No rule of the checker reads it.
6. **Type → JSON Schema.** F1 gains `SchemaOf(T)`, one exact mapping tested by
   suite cases, with the law `TypeFromSchema(SchemaOf(T)) ≡ T`. This reverses
   R114.
7. **Host documentation.** `Function` and `OpaqueType` gain `Doc`. Hover gives
   it. This reverses R188.

The `@name`, `@description`, `@input`, `@output` and `@param` headers of the
#640 prototype disappear. The source says all of it.

## 2. The problem

### 2.1 Evidence from #640 (ran, Go at `16cd6f8d`, by the issue's author)

| Friction | What the prototype had to do |
| --- | --- |
| A program cannot declare its inputs | `@input` holds type text in a comment. The bridge declares it as the variable `args` and builds the schema from it |
| A program cannot declare its output | `@output` in a comment, checked with `ExpectType` |
| A parameter cannot be described | `@param` comments. They reach top-level fields only. A nested field, or a field of a list element, cannot be described |
| No schema from a type | About 80 lines of Go over the structural type API. Each host of each binding writes it again, and may write it differently |
| One tool per file | A tool that calls another goes through a host function, `tools.get_weather`, which starts a new run with a full budget (#640, "Nested tool calls escape the caller's budget") |

### 2.2 Evidence from #641 (the issue's text; not run here)

fynerisor scripts register handlers that outlive the run: button taps, entry
submits, dialogs. The owner confirmed the pattern for v3 on the issue: the host
owns the state, dispatches events, runs a program per event and applies the
data it returns. What is missing is a way for one file to hold every handler.
Today a host needs one program per handler, or one program that switches on an
event tag.

### 2.3 Why not the alternatives

| Alternative | Why not |
| --- | --- |
| A program returns a record of lambdas | A function is not a value (`01-language.md` 8.1 rule 4). Lifting that for one place adds a value kind that the host keeps, and every rule about values must then say what it does with one |
| Closures that outlive the run | They break R3 and E6, and S3 was made to prevent them |
| Keep the headers, and specify a tool-file format (P7) | Out-of-band text that duplicates what the language should say. The checker cannot see it |

## 3. The design

### 3.1 Syntax

The productions of `01-language.md` 3.1 change so:

```ebnf
Program  = { Top } [ Expr ] [ ";" ] .
Top      = Const | Export | TypeLine .
Export   = "export" Const .
TypeLine = "type" TName "=" PType ";" .
Const    = "const" Pattern [ ":" PType ] "=" ( Lambda | Expr ) ";" .
Lambda   = ( Name | "_" | "(" [ Param { "," Param } [ "," ] ] ")"
             [ ":" PType ] ) "=>" Expr .
Param    = Pattern [ ":" PType ] [ "=" Expr ] .
```

`PType` is F4 2.1's program type. `TName` is F4's type name: an upper-case
letter, then one or more of `[A-Za-z0-9_]`.

| Matter | Rule |
| --- | --- |
| `export` | A contextual word. It is a word of the grammar only at the start of a top-level line, directly before `const`. Elsewhere it is a name, as today. It stays unreserved (`01-language.md` 2.4 item 3) |
| `type` | A contextual word, as D45 planned. It starts a type line only at the start of a top-level line, followed by a `TName` and `=`. `type(x)` stays a call |
| Where `Export` and `TypeLine` stand | At the top level of a program only. In a block or a branch, `export` keeps the parse code `export-statement`, and `type Name = T` keeps `unexpected-token` |
| What `export` may mark | A `const` line whose pattern is a name and whose value is a lambda. Anything else is the check code `bad-export` (3.3) |
| A parameter's annotation | After the pattern, before the default. A lambda written without parentheses, `x => e`, has none, as in TypeScript |
| A result annotation | After `)`, before `=>`. The type ends at `=>` (F4 2.3 item 1) |
| Finding a lambda | At the start of a `Const` value or an `Arg`, a `(` starts a lambda when the token after its matching `)` is `=>` or `:`. In those two places no expression can have `:` after a parenthesized operand, so the test is exact (inference from the productions; to be confirmed by the parser work) |
| A program with no result expression | Allowed only when it has at least one `Export`. Otherwise it is the parse code that an empty program gives today |
| Comments | Doc comments are block comments (3.6). They stay white space for the grammar |

### 3.2 Meaning of annotations and type lines

| Form | Meaning |
| --- | --- |
| `type N = T;` | Declares the named type `N` for the rest of the program, as F4 3.1 declares one in an environment. An abbreviation, never nominal. It may use the environment's type names and the type lines before it. Whether it may name itself is plan `18`'s matter |
| A type line and an environment's name | A type line may not reuse a type name the environment declares: `name-clash`, phase `check`. Two type lines with one name: `duplicate-name` |
| `(x: T) => e` | The parameter has type T in the body. The body is checked once, where it is written, with that type (3.4). It is a claim checked statically. It runs no test |
| `(...): R => e` | The body's type must fit R, else `result-mismatch` at the body. The lambda's result type is R |
| `const p: T = e;` | `e`'s type must fit T. The names `p` binds take their types from T. No test runs. A cost or a failure with no word in the text would surprise a TypeScript author (07, "Left out") |
| A value of type `any` against an annotation | The rule of `05-types.md` for `any` where a type is expected, with the same sites. An annotation adds no new kind of site |

Annotations are checked by the type rules only. A compile without the type
rules (`03-host.md` 3.2.1 item 4) resolves the type text, so bad text is still
`bad-type-text`, and ignores the claim.

### 3.3 Entry points

| Rule | Text |
| --- | --- |
| Entry point | A top-level `const` line marked `export`, whose pattern is a name and whose value is a lambda |
| Annotated | Every parameter of an entry point carries a type. A parameter that does not is `bad-export`. A result annotation is optional: without one, the result type is the body's type |
| The name | The `const` name. Two entry points cannot share a name, by `01-language.md` 5 rule 7 |
| An entry point is still a `const`-bound lambda | Other code in the program may call it by name. That call is an ordinary call, in the same run and budget |
| Check | Each entry point's body is checked once, with its parameters at their annotated types |
| `bad-export` | A static rule (`05-types.md` 22): reported in every compile, with or without the type rules. Templates: `not-lambda` ("export marks a const-bound lambda; {name} is not one"), `unannotated` ("parameter {param} of the entry point {name} needs a type") |

### 3.4 Annotated lambdas in the checker

`05-types.md` 13.9 checks a `const`-bound lambda at each use, because only a
use gives the parameter types. A lambda whose parameters are all annotated has
its types already.

| Lambda | Rule |
| --- | --- |
| Every parameter annotated | Checked once, where written, with the annotated types. At a call by name, each argument must fit its parameter: `arg-mismatch`. The call's type is the result annotation, or the body's type |
| Some annotated, some not | Checked at each use, as 13.9 says, with the annotated parameters bound to their types and each argument checked against them |
| None annotated | As today |
| Passed to the library (`xs.map(f)`) | Each argument the library passes must fit the annotated parameter type, at the solved type of 13.5 |

### 3.5 Calling an entry point

**The stage.** `03-host.md` 3.2.1 gains a fifth stage.

| Stage | Takes | Gives | Fails with |
| --- | --- | --- | --- |
| Call | A runnable program, an entry point's name, its arguments, a supply of variables, and a cancellation signal | A value, or exactly one diagnostic; and the usage | `run`; or `declare` for an unknown name; or the `not-conforming` failures of the arguments |

**The evaluation.** `02-evaluation.md` gains this section.

1. The **needs** of an entry point are the top-level `const` lines whose names
   its body refers to, and the needs of each `const`-bound lambda among them,
   to any depth. A `const` line whose pattern binds several names is needed
   when any of them is. Type lines are never evaluated.
2. A call evaluates the needed `const` lines that are not lambdas, in text
   order, as a program evaluates them. It evaluates no other `const` line and
   not the result expression.
3. It then binds the arguments to the parameters, as a call by name does
   (`01-language.md` 8.3), and evaluates the body. The value of the body is
   the value of the call.
4. The call charges the steps and units of what it evaluates, as a run does,
   and 1 step for the call of the entry point. It is bounded by the
   environment's limits.
5. Nothing is kept between calls. Each call evaluates its needs again.

The needs are decided from the text alone, so every implementation evaluates
the same lines in the same order.

**Arguments.** The binding conforms each argument to its parameter's type with
F8's `ConformAll` before the run. Any failure ends the call with those
diagnostics, and no step is charged. A checked entry point then cannot fail by
a wrong argument, which is the promise the checker gives a variable.

**The input record.** A tool host receives one JSON object. Every binding
offers a second way to call, by record:

| The entry point has | Its input type |
| --- | --- |
| One parameter, whose type is a record with no other alternative | That type. The object is the argument |
| Any other list of parameters | An exact record with a field for each parameter, by name. A parameter with a default is optional. A parameter's description (3.6) is its field's description |

A parameter whose pattern is not a name, in the second row, has no name to
use: calling by record such an entry point is the `declare` error
`no-input-record`. The input type is what the host gives to `SchemaOf` (3.7).

**The program API.** Every binding offers these, in its own case of names
(R201).

| Call | Gives |
| --- | --- |
| Entries | For each entry point, in text order: its name, its description, each parameter (name or null, type, description, optional), its input type, and its result type |
| Call | The value and usage of one call, by name with an argument list |
| Call by record | The same, with one value conformed to the input type |
| Expected result of an entry point | An option of compile, one for each entry point: a type its result must fit, reported as `result-mismatch` at that entry point's body |

**A program with no result expression.** Run gives the `declare` code
`no-result` and charges nothing. Compile with an expected result is
`no-result` too.

### 3.6 Descriptions

**The doc comment.** A block comment that starts with `/**` and is not `/**/`.

| Rule | Text |
| --- | --- |
| Attaches to | The next token, when only white space stands between them, and that token starts one of: a field key in type text (any depth, in a program type, a type line, a declared type, a signature); a parameter; a `const` or `export` line; a type line. Otherwise it is an ordinary comment |
| Two in a row | The last one attaches. The earlier one is an ordinary comment |
| Its text | Remove `/**` and `*/`. Split into lines at each line end. From each line remove the leading white space, then one `*` if one starts it, then one space if one follows. Remove trailing white space of each line. Drop empty lines at the start and the end. Join with U+000A |
| A host declaration | A variable's type text, a signature and a method's text may hold doc comments by the same rule. `Function` and `OpaqueType` gain a field `Doc`. A method is a `Function`, so it has one |

**Descriptions are metadata.** A description belongs to a record field, a
parameter, a `const` line, an entry point, a type line or a host declaration.

| Rule | Descriptions |
| --- | --- |
| Fits, Equal, size, depth, the walk, conformance, every site and diagnostic | Not read |
| Printing (`05-types.md` 3.9), the description of an environment (F1 5.3) | Not written |
| A field taken from one source field: narrowing, a fact, `filter`, spread, Close, a named type replaced | Kept |
| Join of two records, a key in both | Kept when both fields have the same description, or neither has one. Otherwise none |
| Join, a key in one record only | Its description is kept |
| Meet and Pick (F1 3.6), a key in several parts | The description of the first part, in the order F1 3.6 meets them, that has one |
| `x as T`, an annotation | The type is T, with T's descriptions |
| A literal's type | None |

The rule for Join keeps a description only where both sides agree, so three
checkers cannot keep different ones. Meet follows F1's fixed order for the
same reason.

**Who reads descriptions.** `SchemaOf` (3.7), hover (F12 3), completion's
detail (F12 3), and the Entries call (3.5). Nothing else.

### 3.7 Type → JSON Schema

F1 5.1 item 3 ("No schema is given back") is replaced by this section.
`SchemaOf(T)` gives a JSON value. Its text is its JSON form by
`01-language.md` 12: keys in code point order. Two implementations give the
same text for one type.

| T | Schema |
| --- | --- |
| `any` | `{}` |
| `never` | `false` |
| `null` | `{"type":"null"}` |
| `bool` | `{"type":"boolean"}` |
| `int` | `{"type":"integer"}` |
| `float` | `{"type":"number"}` |
| `string` | `{"type":"string"}` |
| A literal set of one string | `{"type":"string","const":s}` |
| A literal set of several | `{"type":"string","enum":[...]}`, the strings in code point order |
| `E[]` | `{"type":"array","items":SchemaOf(E)}` |
| A record | `{"type":"object"}`, with `properties` (each field's schema) when it has a field, `required` (the required keys, in code point order) when one is required, and `additionalProperties` by its rest: `false` for `never`, absent for `any`, `SchemaOf(R)` otherwise. So `map<T>` is `{"type":"object","additionalProperties":SchemaOf(T)}` |
| Several alternatives | `{"anyOf":[...]}`, each alternative's schema in the order of printing (`05-types.md` 3.9, `null` last). A tagged union (F5) is this row: each record has its tag as a required field with a `const` or an `enum` |
| An opaque type, anywhere in T | Refused: `bad-type`, "an opaque type has no JSON Schema" |
| A field's description | `"description"` in that field's schema. When the schema is `false`, the description is dropped |
| A recursive named type (`18` 4) | Written once, by its name, under a top-level `$defs`, and referred to as `{"$ref":"#/$defs/Name"}` at each occurrence, the root included. F1 already reads such a schema back (`18` 4.1 rule 5), so the law holds |

1. **No bounds.** `int` gives no `minimum` or `maximum`. A value outside the
   safe range fails conformance (`not-conforming`), so the host learns of it.
2. **The law.** For every type T made from type text, with no opaque type,
   `TypeFromSchema(SchemaOf(T))` with no option (F1 4) is T, field
   descriptions included. A type with a fill or a closable mark (F1 4) comes
   only from a schema with options and is outside the law.
3. **F1 3.8 reads `description`.** The `description` of a property's own
   schema object becomes the field's description. A description under a
   `$ref` target is not read, as F1 4.3 item 1 reads `default`.
4. **Suite cases.** A case field `schemaOf`: the type text, and the expected
   schema text or the error. Each row of the table has a case, and each case
   also tests the law.

### 3.8 Host documentation and hover

| Change | Text |
| --- | --- |
| `03-host.md` 3.3.1 | A function's documentation is its `Doc`. It stays non-normative in what it says, and normative in where it goes: hover and the Entries call show it as given |
| F12 3, hover | A host function, a method of an opaque type, an entry point, a `const`-bound lambda with a doc comment, and a field with a description give a hover record with that text as its doc. This reverses R188 |
| F1 5.3 | Item 3 stands: no documentation in an environment's description. A description is for check, and check does not read documentation |

## 4. Examples

### 4.1 A tool file from #640

```js
/** Current weather for a city, with a one-line summary. */
export const get_weather = ({city, units = "metric"}: {
  /** The city name, such as "Paris". */
  city: string,
  /** Temperature units. Defaults to metric. */
  units?: "metric" | "imperial",
}): {city: string, temp: float, unit: string, summary: string} => {
  const w = weather.current(city);
  const imperial = units == "imperial";
  const temp = if (imperial) { w.tempC * 9 / 5 + 32 } else { w.tempC };
  const unit = if (imperial) { "°F" } else { "°C" };
  {city: w.city, temp: round(temp), unit,
   summary: `${w.city}: ${fixed(temp, 1)}${unit}, ${w.conditions}`}
};

/** A short brief for a trip: weather at each stop. */
export const trip_brief = (stops: string[]): string =>
  stops.map(c => get_weather({city: c}).summary).join("\n");
```

The input type of `get_weather` is its parameter's record. `SchemaOf` gives the
schema of #640's "generated definition", with the descriptions in it, from the
source alone. `trip_brief` calls `get_weather` by name, in its own run and
budget: the nested-budget gap of #640 does not arise inside one file.

### 4.2 Event handlers from #641

```js
type State = {count: int, name: string};
type Action = {type: "toast", text: string} | {type: "close"};

export const onTap = (s: State, ev: {id: string}):
    {state: State, actions: Action[]} =>
  match ev.id {
    "inc" => {state: {...s, count: s.count + 1}, actions: []},
    "done" => {state: s, actions: [{type: "close"}]},
    _ => {state: s, actions: [{type: "toast", text: `unknown ${ev.id}`}]},
  };

export const view = (s: State) => {
  type: "button", label: `Clicked ${s.count}`, onTap: "onTap",
};
```

The host keeps the state. On each event it calls the entry point the scene
names, here by the string `"onTap"`, and applies the result. A handler is
named, never stored. The host checks the names a scene uses against the
Entries call when it loads the file. Opaque widget handles passed as arguments
are plan `19`'s matter.

## 5. Decisions

| # | Decision | Recommendation | Strongest alternative |
| --- | --- | --- | --- |
| 17-1 | Entry points by name, as bounded runs, with conformed arguments | **Settled by the owner, 2026-11-14** | — |
| 17-2 | How an entry point is marked | **Settled by the owner, 2026-11-14: `export const`.** TypeScript's spelling (C6). Helpers stay private. A host lists what the author chose to offer. It prepares imports (D50) | Every top-level `const`-bound lambda whose parameters are all annotated. No new word, but a helper becomes callable by accident, and adding a type to a helper changes the host's tool list |
| 17-3 | Parameter and result annotations, now | **Settled with 17-1:** entry points need them. Plan `18` needs them for recursion. Brings D40's annotations forward | — |
| 17-4 | Type lines in a program, now | **Yes, top level only.** Entry points need names for their types, and plan `18` needs a place for recursive types | Named types from the environment only. A tool file could then not name its own types |
| 17-5 | `const p: T = e` | **Yes, static only.** Once parameters have annotations, a `const` without one is the odd case | Leave it for later. Small cost either way |
| 17-6 | What an entry call evaluates | **Its needs, in text order, again on each call** | Every top-level `const` before it, as a run does. Simpler to state, but calling one tool would make the host calls of another tool's setup |
| 17-7 | Sharing evaluated `const` lines between calls | **No.** Usage must be exact for each call, a `const` may read variables that differ between calls, and a shared value would outlive its run | Evaluate once per program and share. Faster, but which run's limits pay, and replay (plan `19`) must journal a run that no call owns |
| 17-8 | A program with no result expression | **Allowed when it has an entry point** | Require a result. A tool file would end with `null` |
| 17-9 | Calling by record | **Yes, by the input record of 3.5** | Positional calls only. Every tool host would write the mapping again |
| 17-10 | Descriptions as doc comments, at any depth | **Settled direction, 2026-11-14.** `/** */`, as JSDoc. The text rule of 3.6 | `@param` paths outside the type (Hydrogen's reviewer). It keeps type text unchanged, but a nested field is described far from where it stands |
| 17-11 | Descriptions are metadata, with the rules of 3.6 | **Settled direction.** Join keeps only an agreed description | Make them part of the type. Then two types that differ only in a comment would not be equal |
| 17-12 | Type → JSON Schema in F1 | **Settled direction, reverses R114.** The table of 3.7, `anyOf` for every union, no bounds on `int` | `type: [t, "null"]` for a nullable primitive. Shorter, but a second rule for one idea |
| 17-13 | F1 reads `description` from a schema | **Yes**, so the law holds with descriptions | Ignore it. Then the law holds only without descriptions |
| 17-14 | Host documentation in hover | **Settled direction, reverses R188** | — |

## 6. What this changes in other documents

| Document | Change |
| --- | --- |
| `00-overview.md` | Row D53. D40: annotations and type lines move from "a minor version" to the release with this plan |
| `15-build-rulings.md` | Rulings for 17-2 to 17-14. R114 and R188 marked reversed by D53 |
| `07-checked-conversion.md` 3 | The row "Annotations" points here |
| `02-language.md` | The productions; `export` and `type` as contextual words; the row for `export const x` in the targeted errors now covers only a non-top-level `export` |
| `spec/core/01-language.md` | 2.2: the doc comment. 3.1: the productions of 3.1. 5: a program with no result expression; type lines in scope. 8.1: an entry point; annotations. 14.x: the condition of `export-statement` narrows |
| `spec/core/02-evaluation.md` | A section for the call of an entry point (3.5) and its cost row |
| `spec/core/03-host.md` | 3.2.1: the Call stage. 3.3.1: `Doc` on functions and opaque types. 3.7: the `declare` codes `no-result` and `no-input-record`. 3.10: case fields for an entry call (`entry`, `args` or `input`) |
| `spec/core/05-types.md` | 3.1, 3.2: doc comments in type text and signatures. 13.9: annotated lambdas (3.4). 16: the check code `bad-export`, so Core's count goes from 27 to 28 and the catalog's from 29 to 30, before plans 16, 18, 19 and 20 add theirs. 22: `bad-export` as a static rule |
| `spec/packages/f01-host-types.md` | 3.8: `description`. 5.1 item 3: replaced by `SchemaOf`. 7: the case field `schemaOf` |
| `spec/packages/f04-conversion.md` | 2: `PType` in parameters, results and `const` lines. 3: named types from type lines, with `name-clash` and `duplicate-name`. 3.3: a row for a type line |
| `spec/packages/f08-errors.md` | Templates for `bad-export`, `no-result`, `no-input-record`. The new condition of `export-statement` (E18). Counts in 3.1 item 2 |
| `spec/packages/f12-tools.md` | 2: the formatter prints annotations, type lines, `export` and doc comments. 3: hover gives descriptions |
| `05-host-api.md` Part E | Entries, Call, Call by record, the per-entry expected result, `SchemaOf`, `Doc` on `Function` and `OpaqueType`, in all three bindings |
| `suite/` | `messages-check.json`: `bad-export`. `messages.json`: `no-result`, `no-input-record`. Case files for entry calls, needs, annotations, type lines, doc comments, `SchemaOf` |
| `go/env.go` | `Function.Doc`, `OpaqueType.Doc` |

New codes: one check code (`bad-export`) and two `declare` codes
(`no-result`, `no-input-record`). No new run code, no new reserved word. Node
kinds: `Const` gains an export mark and an annotation, `Lambda` and `Param`
gain annotations, `Program` gains its type lines and an optional result. I
recommend no new node kind; a `TypeLine` kind is the alternative if the
formatter needs one (inference, to settle when `02`'s node list is edited).

## 7. Where this meets plans 18, 19 and 20

| Plan | Meeting point |
| --- | --- |
| `18` | Self-recursion needs the annotations of 3.1 and 3.4. Recursive named types need type lines (3.2). This plan says only that a type line may name itself as plan `18` allows |
| `19` | Replay journals one entry call as it journals one run, and the replay identity names the entry (`19-3`). `callWithin` (`19-6`) lets a host run an entry call inside a caller's budget. Effect marks let Entries report whether an entry point only reads. Opaque values cannot pass through JSON, so a host passes them in a positional call |
| `20` | The brief rendering is the form a tool host shows a model for a failed conformance of arguments |

## 8. Cost

| Part | Go | TypeScript | Rust |
| --- | --- | --- | --- |
| Parser: annotations, type lines, `export`, optional result | Small. F4's program type parser is reused | Same | Same |
| Doc comments: kept by the lexer and attached | Small. The formatter of F12 already keeps comments | Same | Same |
| Checker: annotated lambdas, type lines, `bad-export`, descriptions through Join and Meet | Medium | Medium | Medium |
| Needs and the Call stage | Small | Small | Small |
| `SchemaOf` | About 150 lines (inference, from the prototype's 80 lines without descriptions and exact text) | Same | Same |
| Suite | About 60 cases (inference) | Shared | Shared |

## 9. What I did not verify

1. That the lambda test of 3.1 ("`(` then `)` then `=>` or `:`") has no
   ambiguity in any place a lambda may stand. Inference from the productions.
2. That every type made from type text round-trips through F1 3 at the depth
   and size limits of F1 3.10. Inference; the suite cases are the test.
3. The cost estimates in section 8. Inference.
4. How Rust and TypeScript keep comments in their lexers today. I did not read
   their source, as AGENTS.md requires; F12's formatter cases imply that each
   keeps them.
