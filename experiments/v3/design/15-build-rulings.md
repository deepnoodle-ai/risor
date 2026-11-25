# Risor v3: rulings made during the build

Status: a record. The owner gave the design lead the design and
implementation decisions on 2026-11-09 ("you call the shots"), so each ruling
below is in force. Any of them can be reversed: say which.

When the specification under `spec/` and a design document `00` to `13`
disagree, **the specification wins**. The design documents are the reasons;
the specification is the rule. This file lists each place where the
specification settles or changes what a design document says, so the two can
be compared without reading both.

A ruling that only confirms a design document is not listed. The full list of
questions and answers for a round is kept with the build's working files.

## 1. Decisions taken to start the build

Section 7 of `14` was taken as recommended.

| # | Ruling |
| --- | --- |
| D37 | An `if` must have an `else`. |
| D38 | `match` by conditions is not in 3.0. `match {` with no subject is a parse error. |
| Codes | Code names are as the design documents write them. |
| Line | Core and follow-on are as `14` sections 2, 3 and 5 say. |
| D8, D10 to D13, D15 to D17, D21, D25, D31 | As recommended in `00`. |
| D23 | Open. Go tests use wonton's `assert` until it is settled. |
| Layout | `spec/`, `suite/`, `go/`, `ts/`, `rust/`. v2's code stays on the branch for now. |
| Parser | Each lexer and parser is written by hand, a Pratt parser. Settled by the owner. |

## 2. Round 1: the five Core chapters

### 2.1 Changes to a design document

| # | Ruling | Changes | Why |
| --- | --- | --- | --- |
| R1 | `_ => e` is a lambda, the same as `(_) => e`. | `02` 2.2, the `Lambda` rule (edited) | `(_) => e` already parsed. An author from TypeScript writes `_ =>`. |
| R2 | `x in list` charges, for each element compared, 1 step, plus the size of the common prefix when both are strings, plus the steps of `==` when both are lists of one length or maps of one size. | `04` A.3, "1 per element compared" | Comparing long text or deep lists was work with no charge, against C5. A list of numbers costs what it cost. |
| R3 | `s[i]` on a string charges 1 + the size of `s`. | `04` A.3, "index: 1" | Finding position `i` reads the text; `02` 2.14 already lists it as work that grows. A string under 64 in weight costs what it cost. |
| R4 | A failed element test of a library entry (`sum`, `flat`, `join`, `toMap`, the ordered test) charges 1 step for each element tested. | `06`, silent | `wrong-kind` can be caught, so a free scan inside a callback was unbounded work. |
| R5 | `sum` has the signature `() => T \| int where T: number`. | `06` 2.8 | `[].sum()` had type `never` and the value `0`. Now its type is `int`. |
| R6 | `int("1e400")` fails with `invalid-argument`, form `not-finite`, as `float("1e400")` does. | `06`, which lists the form for `float` only | One text, one answer from both. |
| R7 | `string(x)` charges the walk that a template charges. | `06` open question 5, and `06` 2.8 "1 per element and entry walked" | One rendering, one cost, wherever it happens. |
| R8 | `03` example: `gate.failed \|\| "none"` has type `int \| "none"`. | `03` 2.9 (edited) | An error in the example. |

### 2.2 Rules the design documents did not have

| # | Ruling |
| --- | --- |
| R9 | **Every implementation runs the static rules**, with or without a type checker: unknown names, calls of what is not a function, lambdas where none may stand, duplicate names, keys and cases, the count of arguments of a call by name. They need no types. Otherwise the three evaluators would accept different programs until each had its checker. |
| R10 | Every implementation parses type text, since it needs the names of variables and the count of parameters of functions. |
| R11 | The suite has `core-run/`, which needs no type checker, and `core-check/`, which does. |
| R12 | A case may carry `precondition: false` from Core on. Core runners ignore it; F9 gives it its meaning. So no Core case is edited later. |
| R13 | The usage of a run that fails on a limit is not fixed, and a case must not state it. The usage of every other failed run is exact. |
| R14 | Null as a receiver, operand, index, key or spread operand is `null-operand`. Null as an argument or an element of a library entry is `wrong-kind`. |
| R15 | On `any` data, a method the receiver's kind lacks, and a count of arguments that method does not take, are `wrong-kind` at run time, raised before the arguments are evaluated. |
| R16 | `a ** b` with `b` below zero: an infinite `a ** -b` gives zero, a zero one gives `number-range`, and `a` zero gives `divide-by-zero`. |
| R17 | The first `parse` diagnostic: the whole text is tokenized, and a lexical diagnostic comes before any syntax diagnostic. It is the one rule that three hand-written parsers cannot read differently. |
| R18 | `null-operand` and `wrong-kind` are raised before the operation charges. Every other code is raised after. |
| R19 | Opaque types and their declared methods are in Core. |
| R20 | The largest depth a host may set is 1,000. Steps and allocation limits are whole numbers of 0 or more. The largest depth is replaced by R206: 400 (D58). |
| R21 | The run-phase catalog has 61 templates, not the "about 33" of `14` 2.3: one for each shape, so a message names what was found. |
| R22 | The case table for `toUpperCase` and `toLowerCase` is a file of the suite, `casemap.json`, made from Unicode 16.0.0. No implementation uses its platform's tables. |
| R23 | The message of a `parse` or `check` diagnostic is not specified in Core. Its code and span are. F8 brings the catalog for those phases. |

## 3. Round 2: work that was not charged

The limits promise that the work of a run is at most a constant times the
steps charged (`04` L2). An audit of the Core chapters found work that grew
with no charge. Each fix adds a term that is zero for a string under 64 code
points, so no ordinary program costs more.

| # | Ruling | Changes |
| --- | --- | --- |
| R24 | **The key rule.** Probing or building a map by a key charges the size of the key: `m[k]`, `m.name`, `x in m`, each entry of a map literal, a map spread, a map pattern, `==` on maps, `groupBy`, `toMap`. | `04` A.3, several rows; `06` 2.8 |
| R25 | **Searching text is linear.** A search for `x` in `s` does work of at most a constant times the two lengths. An implementation uses its platform's search only where that holds, and one of its own otherwise. Binds `in` on strings, `split` and `replaceAll`. | `04`, `06`: silent |
| R26 | `unique`, `toMap` and `groupBy` are done with a hash set or map, so their expected work is linear. | `06`: silent |
| R27 | R4 is narrowed: only a scan made before the entry's charges pays on failure. A callback's result or key that fails its test was already paid for with its element. | R4 |
| R28 | `a != b` charges what `a == b` charges, not the extra step of a `!`. | `02` 2.7, which defines it as `!(a == b)` |

## 4. Round 3: a lambda at a method call

A lambda at a method call needs the receiver's type, so its rules are type
rules and not static rules. With a receiver of type `any`, or in an
implementation with no type checker, nothing said what the run does.

| # | Ruling | Changes |
| --- | --- | --- |
| R29 | **The shape test.** At a method call that was not resolved when compiling, the run tests, before any argument is evaluated: the method exists for the receiver's kind; it takes that count of arguments; each callback parameter gets a lambda; no other parameter gets one; no lambda has more parameters than it is called with. The first failure is `wrong-kind` at the call. | R15, which had the first two |
| R30 | `wrong-kind` gains the templates `callback`, `not-callback` and `parameters`. The catalog has 64. | R21 |
| R31 | A method name written without a call is a field read, never `not-a-value`. `m.keys` reads the key `keys`. | `06` 2.3 |

## 5. Round 4: corners of lexing and parsing

Writing 1,028 syntax cases found places where two hand-written lexers could
differ. Each is now fixed by a rule and a case.

| # | Ruling |
| --- | --- |
| R32 | A U+FEFF at the start of the text is removed and not counted: the next code point is line 1, column 1. |
| R33 | In `\u{...}` every hex digit is read, and the span of a bad one runs to the last of them. |
| R34 | A `\` directly before the end of input is part of an unterminated literal. The code is `unterminated-string`, never `bad-escape`. |
| R35 | When two literals stop at the same place, the innermost is reported. |
| R36 | The nesting level that `match` opens has closed when the `{` of its arms opens. |
| R37 | In `!!a in b` and `- -2 ** 2`, the span is the first prefix operator in the text. For `a ?? b not in c` it is both words, `not in`. |
| R38 | `\$` and `` \` `` are escapes of a template only. In a one-line string each is `bad-escape`. |
| R39 | The control characters are U+0000 to U+001F and U+007F to U+009F. |

## 6. Round 5: questions from the first front end

| # | Ruling |
| --- | --- |
| R40 | Until F4 gives them a meaning, `as` and `is` are read at the level of a comparison, so the error for `!x as T` is at the `!` and for `a + !x as T` at the `as`. |
| R41 | A reserved word where a `match` case needs a literal is `reserved-word`: `match x { nil => 1 }`. |
| R42 | Environment text that is a type but not a record (`"int"`) is `bad-type` in every implementation. |
| R43 | Text to number is correctly rounded for any count of digits. An implementation whose platform does not promise that does it itself. |
| R44 | What cannot enter the language is a value of a host **integer** type over 2^53 − 1 in magnitude, a NaN and an infinity. A finite host float always enters: `1e20` is a number that is not whole. |

## 7. Round 6: questions from the control-flow cases

| # | Ruling |
| --- | --- |
| R45 | A call is in progress, for the depth count, from its depth test to its end. Its arguments run before that. |
| R46 | A `match` arm that compares a string subject with a string literal charges the size of their common prefix, as `==` does, on top of the arm's step. |
| R47 | Every failure a library entry raises has the span of the whole call, whichever argument or element failed. |
| R48 | The `missing-field` message of a pattern names the key the data lacks, not the name the pattern binds. |

## 8. Round 7: questions from the evaluation cases

| # | Ruling |
| --- | --- |
| R49 | **Null first.** A binary operator that rejects null tests both operands for null before it tests their kinds. `"a" * null` is `null-operand`, not `wrong-kind`. |
| R50 | A case file can hold an opaque value: `{"$opaque": {"type": "Token", "id": 1}}`. Two are the same host value when type and id are equal. Without this, opaque types were in Core and no case could test them. |
| R51 | A message names an operator by its canonical text (`not in`), and every parameter of a message is cut at 64 code points, names included. |

## 9. Rounds 8 and 9: the Rust front end and an independent review

Three front ends, written apart, agree on every syntax case. An independent
reviewer then read the five chapters: it recomputed every worked count and
example and found none wrong, and reported 28 places where two readers could
differ. The rulings that change or add a rule:

| # | Ruling |
| --- | --- |
| R52 | The limit of 64 on type text is on its depth: a `[]` suffix counts 1, and a bracketed form counts 1 over its deepest content. |
| R53 | A case file nests JSON at most 1,024 deep. |
| R54 | A binding documents the stack it needs, tests at that stack, and never overflows it within the limits. |
| R55 | **The run always selects a method by the receiver's kind** and always makes the shape test. Resolving a method when compiling is an optimization that is never observable. So a checked and an unchecked run cannot differ. |
| R56 | Every element of a list pattern needs its position, `_` included, and the checker records the site on `_`. |
| R57 | The failed-scan charge (R4, R27) is for list methods. The functions `min(a, b)` and `max(a, b)` test their arguments for nothing, as every function does. |
| R58 | A library call tests its receiver, then its arguments left to right, kind before whole number. For an `ordered` bound every kind failure uses one template. |
| R59 | Core does not test the arguments of a host function at run time, beyond their count. |
| R60 | A call gives every parameter up to the last one with no default: with `(a = 1, b) => b`, `f(5)` is `arity`. |
| R61 | A search is done in work of a constant times `1 + w(s)`: a needle longer than the text is answered from the lengths. Tightens R25. |
| R62 | A namespace is called only with a plain `.`: `geo?.f()` is `not-a-value`. |
| R63 | **A variable may be named like a library function** (`type`, `max`, `keys`) and shadows it. A host function named like one is still `name-clash`. |
| R64 | A host number is read when an operation yields it as a value; copying or measuring a container does not read its elements. |
| R65 | Ill-formed UTF-8 from a host gives one U+FFFD for each maximal subpart, as Unicode recommends, not one for each byte. |
| R66 | A case that expects a `parse` or a `declare` failure states exactly one diagnostic. |
| R67 | The record of an environment has no optional field and no rest. An `env` key written twice is `name-clash`. |

## 10. Round 10: questions from the first evaluators

The TypeScript evaluator was the first to run the whole Core suite: 2,744 of
2,744 runnable cases, with one case found wrong (an expected value written
with a five-digit JSON escape) and fixed.

| # | Ruling |
| --- | --- |
| R68 | **Handing the result to the host is not a read** (changes R64). The run makes no walk of its result, because that walk would be work with no charge. A number inside host data that a program passes through untouched goes back as the host supplied it. A binding's own conversion of a result reports a number that cannot enter as its own error. |
| R69 | An empty list or map is entered by a walk like any other, and an empty pair by `==`: the depth test is made for it. There is no special case for an empty container. |
| R70 | In TypeScript a thrown `Error` of any class is `function-error`, which `catch` catches; any other thrown value is `function-panic`. JavaScript's own functions throw `TypeError` for failures a caller expects, so no class can mark a bug. |
| R71 | In TypeScript a plain object and a `Map` are maps. A property that holds `undefined` is absent and an `undefined` in an array is null, as in JSON. |
| R72 | The work bound has one stated exception: ordering the keys of a map of `n` entries may take a constant times `n` × log2(`n`) comparisons, against a charge of `n`. Key order by code point is part of the language and every implementation pays it. |
| R73 | Each implementation has a test command `outcome` that writes what it found for each case of a case file, one line of JSON each. The differential fuzzer in `fuzz/` compares the lines. A limit failure's line has no span and no usage, since the specification leaves both open. |
| R74 | Every runner compares the class of a run failure. A runner with no checker leaves out only the five codes whose class depends on the check. |
| R75 | `==` on two maps visits the left map's entries in key order whenever a value of it is a list or a map, because the order then decides which limit a run passes first. Otherwise any order. |
| R76 | A new `declare` code `bad-limit`: a limit that is not a whole number, or a depth limit outside 1 to 1,000. Core has 9 `declare` codes. The range becomes 1 to 400 by R206 (D58). |
| R77 | **The Core checker is written in all three implementations at once,** not in Go first with TypeScript and Rust after wave 1 (plan `14` M5 and M8). The three evaluators, written apart, agreed on every one of 2,763 hand-derived cases and found the faults in the specification early; the checker's cases are hand-derived too, and three readers of the types chapter test it better than one. The cost is that tagged unions (F5) may change the type representation in three places, not one. |

## 11. Round 11: the differential fuzzer and the checker

The differential fuzzer (`fuzz/`) ran 60,000 generated programs in the three
implementations. They agreed on every value, count, span and message except
in three places: one fault in TypeScript, one in Rust, and one difference
that the checker ends.

| # | Ruling |
| --- | --- |
| R78 | A library parameter whose type is a type variable with no bound accepts every value, null included, like `any`: `[].reduce(f, null)` is `null`. The null test is for a parameter whose type names a kind. |
| R79 | A binding's own conversion of a result is outside the run and must not overflow the host stack on deep data. (A result nested past the depth limit is still handed over, by R68.) |
| R80 | The rules of type text beyond the grammar belong to the checker. An implementation reports them as `bad-type` when it has the checker, which after R77 is every implementation. |
| R81 | **A lambda body is always fitted** against its `Fn` result, with each type variable replaced by its solution. Before, a result with a type variable was never fitted, so `[1].flatMap(x => x)` checked and then failed at run time. |
| R82 | **One rule for every rendered expression:** a template part, and an argument that a library record marks with the new field `renders` (in Core, `string`'s). A type that has an opaque is `kind`; a type that has `any` at any depth records a `dynamic` site. Before, `string(x)` on an opaque value, and a template part with `any` inside a list or map, checked clean and could fail. |
| R83 | The seed rule of `reduce` also fires when the body fits the seed only by a fit that used any. The accumulator is then `any`, as the run will have it. |
| R84 | A `declare` code says where the text stands: `bad-signature` for every error in a function's signature or a method's text, `bad-type` for every error in a variable's type or the expected result's type. |
| R85 | `never-equal` is not reported when a side is `never`: `role in []` is not a mistake. |
| R86 | Sites of patterns: an unlisted key of an open record has its `dynamic` site on the key in the pattern; a list pattern on `any` records its `index` sites too. A pattern's failure is at a site on the value bound, or on the key or element that failed. |
| R87 | Gaining `never` binds a type variable: `[].map(x => x + 1)` is `int[]`, not `cannot-infer`. |
| R88 | The two limits of the checker are exact. Uses of const-bound lambdas are counted in source order as their body checks begin. A use is `check-limit` when the expanded nesting of the use, plus 1, plus the deepest nesting inside the lambda's text, passes 100. |
| R89 | A fact about a path holds for a key the record does not list: under `scores?.x != null`, `scores.x` reads as present. |
| R90 | **A new case field `check`.** `"check": false` marks a case whose program the type rules reject on purpose; it pins what a program compiled without a check does. A runner with a checker runs every other case of the run suite twice, checked and with the static rules alone, and both must pass. |
| R91 | In every binding, compile checks by default; compiling without a check is an explicit option; the check alone is its own call. |
| R92 | In a checked program the nesting of calls never passes 100, so the default depth limit of 200 is met only by a walk of data. |
| R93 | A method call fits its receiver with the type the chain gives it at that step, so with null removed after `?.`: `report?.keys()` reports nothing for the receiver. |
| R94 | When a const-bound lambda is passed by name, `xs.map(f)`, the use is the name: the `arity` diagnostic is on it, and the limit of R88 counts its nesting. |
| R95 | A call handle is valid until its host function returns. A later use charges nothing and changes no run. The Go evaluator reuses run state between runs of one program, so without this rule a handle kept by mistake could charge another run. |

The Go checker, the first of the three, passed all 1,441 cases of
`suite/core-check/` (hand-derived before any checker existed) once rulings R81
to R89 were applied, and found nine older run cases whose programs the type
rules reject. Those nine were changed or marked by R90.

The TypeScript and Rust checkers were then written apart, each from the
chapter and the cases alone. Both pass the same 1,441 cases, and the whole run
suite checked and unchecked. Between them they raised R93 and R94 and no
other question that changed a rule.

## 12. Round 12: the first follow-on packages

Core passed in three implementations, each with the checker. The first
packages of `14` section 5 were then written on paper: a chapter in
`spec/packages/` and cases in a suite directory of their own.

### 12.1 Rules for every package

| # | Ruling |
| --- | --- |
| R96 | **A package's chapter is the home of its rules:** its cost rows, case fields and overrides of Core rows. A Core chapter gains only a pointer, a table "What packages add". |
| R97 | **A runner runs only the directories of the packages its implementation offers.** For it a field of a package it lacks is an unknown field, and the file is invalid as before. No runner needs to know a package it does not have. |
| R98 | **Core cases did pin follow-on text as errors,** against `14` 2.2. A package lists each such case and writes a case with the same text and its own outcome; the listed cases are removed when the package is accepted. A removal needs such a replacement. This replaces "a package edits no case" with "a package changes no outcome but those of the texts it gives meaning to". |
| R99 | A case that pins an error which a named later package turns into a result stands in a file `until-fNN-` and what it holds. The later package removes that file. |
| R100 | For a text that is an error before and after a package, the package may change the code and span where no earlier case states them. |
| R101 | An implementation offers F3 before F2, and a package only with the packages it depends on, so a package's cases may use those forms. |

### 12.2 F3, string forms

The chapter is `spec/packages/f03-strings.md`, with 336 cases in
`suite/strings/`. Plan `08` is its source. Twelve cases were removed from
`suite/core-run/parse-follow-on.json` under R98.

| # | Ruling |
| --- | --- |
| R102 | `text-block-close` is reported at the closing delimiter and `text-block-indent` at the first code point that differs, as `08` 2.7 says: a diagnostic points at the fault. `text-block-open` keeps the opening delimiter, which Core's cases stated. |
| R103 | The three text-block codes are kept with the token and reported when the parser reaches it, before the tokens of its interpolations. So a lexical error anywhere is still reported first, and an outer block's code is reported before an inner block's. |
| R104 | A text block whose opening line holds text runs to the next `"""` or the end of input, and nothing in it is read. A blank opening line followed by the end of input is `unterminated-string`. |
| R105 | Only U+0020 and U+0009 are white space for the opening line, a blank line and the indentation. Indentation is compared code point for code point. |
| R106 | `r'''`, `'''`, `R"` and a prefixed template stay the errors Core gives them. |

Go and TypeScript implemented F3 apart and each found the same two wrong
cases, both in what follows a literal that is already in error. The cases
were corrected and the chapter now states the rule (its 6.2 item 5).

### 12.3 F1, host types

The chapter is `spec/packages/f01-host-types.md`, with 382 cases in
`suite/host-types/`. Its sources are `03` 2.15 and `05`.

| # | Ruling |
| --- | --- |
| R107 | **`allOf` meets records.** The design gave `allOf` no rule. Parts and branches are combined by Pick: `any` is dropped; records meet field by field, a field required when either requires it; otherwise the part that fits the others. The meet is sound and keeps every branch's fields, which is how schemas are composed in practice. |
| R108 | Branches of `anyOf` or `oneOf` that would form a tagged union are `bad-type` until F5, as two records in type text are in Core. Their cases stand in an `until-f05-` file (R99). |
| R109 | `integer` gives `int`, with one stated exception to soundness: a whole number past 2^53 − 1. |
| R110 | `defaults-applied` gives Join(T, the default's literal type), not `nonnull(T)` as `03` 2.15 has it: a property's schema may allow null, and a default need not be valid by its schema. |
| R111 | **The option `closed` closes once, deeply, at the root, after every meet and Join.** Closing each branch or property first is unsound; the lane's fuzzer showed it against a host that strips keys by the mapped record. The option's statement is that stripping, exactly. |
| R112 | A `$ref` cycle is `bad-type`. `any` at the point of recursion could not be sharpened later without changing a result; an option that loads recursive schemas would only add. Reversed by D54: a `$ref` cycle through a list, a field, a rest or a map gives a recursive named type (`18` 4.1 rule 5), from step E2 of track E (`14` 5.3). Built in step E2 by R275. |
| R113 | Keywords the design did not list are given sound readings: `patternProperties`, `prefixItems`, `items` as an array, `nullable`, a `required` name with no property, siblings of `$ref` (ignored). References outside the document, `$id` below the root, `$dynamicRef` and `$recursiveRef` are `bad-type`. |
| R114 | An environment is described as a JSON object of seven keys, with every type and signature as printed text; an environment built from a description checks every program as the original does. There is no JSON Schema output. Printing a signature is defined. Its "no JSON Schema output" is reversed by D53: F1 gains `SchemaOf` (`17` 3.7), in slice D3 (`14` 5.3). |
| R115 | Presenting a host record (a Go struct) as a map is a binding rule of F1. The Rust rules are provisional until its derive is built, and name no other crate. TypeScript derives no type. |

### 12.4 F2, templates and analysis

The chapter is `spec/packages/f02-templates-analysis.md`, with 171 cases in
`suite/templates/` and 138 in `suite/analysis/`. Its source is `05` A.7 and
A.8.

| # | Ruling |
| --- | --- |
| R116 | A host template has two modes. In `string` it is always a string. In `value`, a template that is exactly one part gives that part's value. It is evaluated as Core's template node: one run, one budget, no new cost row, no new code. |
| R117 | `$${{` is the only escape in a template's text. A part ends where Core's rule for an interpolation says, with `}}` as one delimiter. An unclosed part is `unterminated-string`. |
| R118 | **Analysis reports `n.m(...)` with a free `n` as a call named `n.m`,** since without an environment a namespaced host function looks like a method call. Each call carries `namespace`, the receiver's name or null, so a host keeps the calls of the namespaces it declares. |
| R119 | A path follows the chain only: not a `const` binding, a pattern or a parenthesis. A shorter path claims more of the value, so it is always sound. The design's `Partial` flag is left out. |
| R120 | The report has five fields: `names`, `refs`, `calls`, `literal`, `segments`. One result field `analysis` states it in a case, and such a case is analysis only. |

### 12.5 The comparison command

Each implementation has an `outcome` command that the fuzzers and the package
lanes use to compare the three. `suite/README.md` is the home of its line
format.

## 13. Round 13: the fuzzers, with three checkers

The differential fuzzer ran 65,000 cases, and then 40,000 more with the
string forms of F3. Wherever all three checkers accepted a program, the
values, counts, run diagnostics, types and sites agreed. A new soundness
fuzzer (`fuzz/`, `-sound`) made 24,000 programs against random declared types,
ran each checked program on five sets of conforming values, and judged the
promise of `05-types.md` 18.4 from the outcome alone.

### 13.1 Where the promise broke in all three

These were faults of the chapter: 1,441 hand-written cases and three
implementations shared them.

| # | Ruling |
| --- | --- |
| R121 | **A plain index read on an `any` receiver records an `index` site** as well as the `dynamic` site. Before, `x[2]` on an `any` value could fail `index-range` at no `index` site, so a host that rejected `index` sites could still meet that failure. |
| R122 | **A method call on an `any` receiver records a `value` site** when some library method of that name is marked `!`, and a `host` site when a declared opaque type's method of that name is. |
| R123 | A failure of a library entry's test of what a callback gave is at a site when the site is on the lambda's body. Before, `[0].flatMap(() => x)` with `x` an `any` failed with the class `environment`, which blames the host. The same for a computed key and the site on its key. |
| R124 | **The mark `!` is a promise, like a declared type.** A host function not marked `!` returns no error. If it does, the failure's class in a checked program is `environment`. Before, it was `host` at a call with no `host` site. |

### 13.2 Where the three checkers differed

| # | Ruling |
| --- | --- |
| R125 | `never-equal` in a `match` has the conditions of `==`: none when the subject is exactly `null`. A literal listed twice is `duplicate-case` only. |
| R126 | A const-bound lambda whose name is only read as a value has no use: its body is checked once with `any` parameters. A named lambda passed to a method on an `any` receiver is a use, and is checked there. |
| R127 | A pattern's defaults are always checked, whatever the value bound. |
| R128 | `cannot-infer` is about the parameters a lambda declares, not every parameter the callback could take. |
| R129 | Whether a `(` opens a parameter list is decided before what is inside it is parsed. |

### 13.3 A gap in Core

| # | Ruling |
| --- | --- |
| R130 | **Removed library entries had no test.** Core let an environment remove an entry, and the case format could not say so. An entry is named `len`, `string.repeat`, `list.map` or `map.keys`. A removed entry is absent in every compile, checked or not; removing a name that is no entry is `bad-name`; a child cannot bring an entry back. The case format gains the field `removed`. |

## 14. Round 12, continued: the groups and the functions of `math`

### 14.1 F6, opt-in groups

The chapter is `spec/packages/f06-groups.md`. Its records are
`suite/library-groups.json` and its other cases `suite/groups/`: 390 cases.
Its source is `06` 2.8.

| # | Ruling |
| --- | --- |
| R131 | **Every entry of `math` is correctly rounded.** The design made the transcendental functions implementation-defined (overview R12). That contradicts the promise of one value and one count everywhere: a result that differs in its last digit can take the other branch of an `if`. R12 is overruled for `math`. |
| R132 | **The eleven transcendental entries are their own package, F13,** built last: `pow`, `exp`, `log`, `log10`, `sin`, `cos`, `tan`, `asin`, `acos`, `atan`, `atan2`. 3.0 does not wait for three correctly rounded implementations of them; adding entries is additive. F6 keeps `math.sqrt`, `math.pi` and `math.e`, which are exact: 8 entries and 2 constants. F13's chapter is `spec/packages/f13-math.md`, with 114 cases. |
| R133 | Packages are offered in a fixed order, each only with all before it: F3, F1, F2, F4, F5, F6, F7, F8, F9, F10, F11, F12, F13. This replaces R101. |
| R134 | `time.parse` takes any number of fraction digits, so that `parse(format(t))` is `t`; `time.format` charges for its result. Upper-case `T` and `Z` only. |
| R135 | `base64.decode`: one alphabet in a text, leftover bits zero, no white space, padding absent or complete, units of the result as written. |
| R136 | `json.parse`: a number that reads as an infinity is `number-range`; failures in the order grammar, first bad number, depth; the text's nesting counts as a walk. A `not-json` failure gives no position, since Core's template is fixed. |
| R137 | A group is turned on by a declaration. A name of a group that is on clashes with a variable or a host function; of a group that is off, it does not. A group's entry is removed by its full name. A constant is a record with `type` and `value`. The result field `json` compares a binding's JSON text of a result. |

### 14.2 F4, checked conversion

The chapter is `spec/packages/f04-conversion.md`, with 617 cases in
`suite/conversion/`. Plan `07` is its source. Seven cases were removed from
`suite/core-run/parse-follow-on.json` under R98.

| # | Ruling |
| --- | --- |
| R138 | `untagged-union` arrives with F4, as a static diagnostic for a program type with two records; F5 turns the tagged ones into results. `untagged-union` and `check-limit` are static at a program type, so a compile with the static rules alone refuses them too. |
| R139 | The conformance walk charges by Core's key rule: a declared field's lookup costs 1 plus the size of its key, a string tested against a literal set costs its size. Plan `04` A.3 charged 1 for each value and nothing for text. A list or a map is always entered, an empty one too, and the depth test comes before the value's step. |
| R140 | **Under `\|\|`, two `is` tests of one path combine:** after `v is int \|\| v is string`, `v` is `int \| string`. Plan `03` gave no fact there. |
| R141 | A compile with the static rules alone still resolves every named type and program type in full. |
| R142 | The description gains a key `types` only when an environment has a named type. A conform case may hold opaque values. `x as const`, `x is not T` and the texts that need parentheses stay Core's errors, for F8 to replace. |

### 14.3 A hole in Core found on the way

| # | Ruling |
| --- | --- |
| R143 | **The size of a type is at most 100,000,** counted as the type written out with no sharing. A type is held with shared parts, and forty `const` lines that each make a record of two copies of the one before give a type of 2^40 nodes; joining two such types did not return. Past the limit an expression is `check-limit` at a node the chapter names for each construct, and a declared type is `bad-type`. The test is constant work: the size is kept with the type. F1 (a schema's `$ref`) and F4 (a named type) share parts in declarations and cite the same limit. |
| R144 | A site of class `index`, `divide`, `value` or `host` covers a failure only on its own node; the wider reach (an operand, a receiver, an argument, a callback's body) is for `dynamic` sites. |
| R145 | **Child environments.** There are two spaces of names, values and types. A child's declaration replaces the inherited declaration of the same name in the same space, whatever kind it is, and whole: a redeclared opaque type has only the child's methods. The clash rules then apply to the child's whole environment. |

### 14.4 F7, queries

The chapter is `spec/packages/f07-queries.md`, with 329 cases in
`suite/queries/`, seven records in `suite/library-queries.json` and three
templates in `suite/messages-queries.json`. Plan `13` is its source. Ten Core
cases were removed under R98: three in `parse-follow-on.json`, seven in
`check-library.json`.

| # | Ruling |
| --- | --- |
| R146 | **A package's run-phase templates stand in a catalog file of their own,** as its library records do. Core's catalog keeps its 64 templates. |
| R147 | A record may extend a Core entry (`extends`): `sortBy` gains a direction, and every Core case of it keeps its outcome and its counts. A bad direction is `wrong-kind`, tested before the call's step, so that no `!` is added to `sortBy` and no Core site changes. |
| R148 | A star's level lasts to the end of its projection. A step inside a projection spans from the chain's first token. A map, a string and a number fail under `[*]`, also under `?.[*]`. |
| R149 | The five entries that build or probe a map by key charge the size of the keys, by Core's key rule; plan `13` charged it for three of them. `mapValues` charges for ordering its keys before its first call. |

### 14.5 What happens to an earlier case (replaces R98 and R99)

| # | Ruling |
| --- | --- |
| R150 | **An earlier case whose outcome a package changes is moved, not removed,** into a file named `until-fNN-` in its own directory. A runner whose implementation offers package `NN` does not run that file; every other runner does. The package holds a case with the same text. The file is deleted when all three implementations offer the package. The errors package, F8, changes the code of 317 earlier parse cases; removing them when it was accepted on paper would have left every implementation without them until it reached the eighth package. The cases removed for F4 and F7 were restored as such files. |

### 14.6 F5, tagged unions

The chapter is `spec/packages/f05-unions.md`, with 423 cases in
`suite/unions/`. Plan `09` is its source.

| # | Ruling |
| --- | --- |
| R151 | A test of a field gives facts when the type at the path is a union, or a record while a fact about that path from a tag test is in force. Read literally, plan `09` stopped narrowing once one variant was left, which broke `else if` chains and the second arm of a `match`. |
| R152 | **`kind in ["a", "b"]` narrows,** as `kind == "a" \|\| kind == "b"` does, when the right side is a list of string literals. Plan `09` 2.5 said `in` gives no fact, and D47 took that plan as written; this one line of it is changed, because an author expects the two forms to mean the same. |
| R153 | With F5, two `is` tests of one path under `\|\|` combine to a union when their types have a tag. |
| R154 | **`allOf` of a base record and a `oneOf` of variants keeps the union:** the base is met with each variant. This is how a schema shares fields among variants. The lane's fuzzer found no value that breaks it. |

### 14.7 F8, errors, second part

The chapter is `spec/packages/f08-errors.md`, with 638 cases in
`suite/errors/` and 221 in the catalogs `suite/messages-parse.json` and
`suite/messages-check.json`. Plan `10` is its source. 317 earlier cases moved
to `until-f08-` files under R150.

| # | Ruling |
| --- | --- |
| R155 | **A targeted parse code is a mapping of the diagnostic Core already reports,** tried in a fixed order of rows. So it never changes which diagnostic comes first, and it keeps Core's span except in seven rows that no earlier case stated. There are 44 targeted codes, 58 parse codes in all. |
| R156 | The 13 reserved words that plan `02` gave codes of their own get them where a value may start; where a name is needed every reserved word stays `reserved-word`. |
| R157 | The message of a `check` diagnostic prints inferred types, so a case that states a message also tests the types of subexpressions. `declare` messages stay each binding's own. |
| R158 | **The renderings are narrower than plan `10`,** so that three implementations give the same bytes: no `help`, `fix` or origin; one column for each code point; a control character in the excerpt is a space; a long line is cut to a window of 120 columns. |
| R159 | A projection has no entry in a trace. A trace is built only on the path of a failure and is bounded by the depth limit. |
| R160 | A package's entry in a catalog file, for a code Core's catalog has, adds templates to that code. |

### 14.8 F9, testing for authors

The chapter is `spec/packages/f09-testing.md`, with 291 cases in
`suite/testing/`. Plan `11` is its source. Its lane tested the fixtures of
every case of the suite that runs: 2,314 fixtures in 1,752 cases, and each
conforms or is marked as breaking its type on purpose.

| # | Ruling |
| --- | --- |
| R161 | A fixture that does not conform to its declared type makes that one case `invalid`, with the place and the path; the other cases of the file run. Fixtures are tested in code point order of their names, before the first run, whether or not the program is checked. A type variable in a stub's result is tested as `any`. |
| R162 | **Update mode never rewrites a case that passes,** so a second update writes nothing. It writes compact JSON with sorted keys in place of the value of `expect` and keeps every other byte of the file. It never writes a `declare` failure the author did not expect. |
| R163 | A case can test a runner: it holds a case file, as data or as text, and expects verdicts or an invalid file. Every invalid-file rule of the case format then has a case in the suite. |
| R164 | Case files written as programs are in 3.0. `risor test --run` matches a substring, since the three languages share no pattern language. The report text is normative. |

### 14.9 F10, config files

The chapter is `spec/packages/f10-config.md`, with 166 cases in
`suite/config/`. Plan `12` is its source.

| # | Ruling |
| --- | --- |
| R165 | For an expected result only the option `closed` applies; the two fill options are refused. A load fills nothing in: defaults come from the program, or from the host before the decode. |
| R166 | **The fit through a form follows a map literal with spreads** (a change to Core, `05-types.md` 15.1). A config is layered `{...base, port: 8080}`; a misspelled or mistyped key beside the spread is reported at that key, and a failing key that the base gives on the base. Every entry that may give a key is fitted: taking only the last one was unsound, since an optional field does not hide an earlier entry. |
| R167 | **A decode fails when the value does not conform to the type the host type derives.** Null into a field that cannot hold null is an error in every binding, never a zero value left in place; so is a missing required key. Go's decode therefore differs from `encoding/json` on purpose. |
| R168 | A config's value is checked, so its literal nesting is bounded by the type depth of 64, although text parses to 100 levels. |
| R169 | The case format gains `returnsSchema` and `load`. A load reports up to a given number of failures by F8's function. |

| R170 | A key that a host fills, with null or with a default, is no tag: a value that lacks it gives the host no way to choose the variant. Found by the F5 lane's fuzzer under the option `defaults-applied`. |
| R171 | On its failing side `in` also removes a variant whose tag set lies within the list, so there it says more than the chain of `==` (R152). The fit of a map literal against a union follows spreads, the variant chosen from the entries that give the tag. |

### 14.10 F13, math functions, as built

| # | Ruling |
| --- | --- |
| R172 | **Each call of an F13 entry charges 200 steps.** The chapter gave 0. The first implementation is correctly rounded on 100,000 random arguments for each function, against an independent reference at 320 bits, and takes 3 to 12 microseconds a call; an ordinary step takes about 50 nanoseconds. With a cost of 0 the step limit would not bound the time of a run. The charge is made after the argument tests and before the computation. |
| R173 | F13 is offered before F12 in the order of packages: it depends only on F6, and the tools package may then assume the whole library. This changes the order of R133. |
| R174 | The typed layer of Go (F11) changes no result, and the Go type a host gets back is a result: a host that receives struct values keeps receiving them, never pointers into its own slice. An element that a callback only reads fields of is not made a value at all. |

### 14.11 F12, tools

The chapter is `spec/packages/f12-tools.md`, with 399 cases in `suite/tools/`.

| # | Ruling |
| --- | --- |
| R175 | **The formatter's output and the results of completion and hover are data:** specified exactly, a call in every binding, tested by the suite. **The command line program is a contract,** byte for byte, built from those calls and from the renderings and report text of F8, F9 and F10; one implementation ships it in 3.0, in Go. The REPL and the editor grammar are products with their own tests. |
| R176 | The formatter lays a program out by an algorithm, at width 80 and indent 2, with no options, and does not keep the author's line breaks: it is the way to one output in three implementations. Parentheses follow from the tree. A string literal is copied as written. A comment never crosses a token. |
| R177 | For completion and hover the text is cut at the position and closed by a fixed rule, so a text that does not parse still gives a result. |
| R182 | In completion and hover, the detail of a library entry is the signature text of its record, verbatim. `--color auto` gives no color when standard output is not a terminal or `NO_COLOR` is set and not empty. In the REPL a later `const` of a held name shadows it; what is already held keeps its binding. |
| R187 | **The repair closes before it fills.** At the end of a cut text, where the parser accepts the closing bracket of the innermost open bracket, the repair inserts it; otherwise it inserts as before. An unfinished call is closed: hover on `first(` shows the type of `first()`, and on `len(` shows `never` with the signature, an honest arity error. **An unclosed `(` is never a parameter list:** the lambda decision stays the one rule of `01-language.md` 3.2, so `(a, b = a` does not parse and completion falls back to the environment. A second lambda rule for the repair alone was refused. |
| R188 | Only a library record carries a `doc` in 3.0. Hover gives null for a host function and for a method of an opaque type; a binding does not invent one. Go built F12 against the cases and found two wrong cases, corrected. Reversed by D53: `Function` and `OpaqueType` gain `Doc`, and hover gives it (`17` 3.8), in slice D3 (`14` 5.3). |
| R189 | **The empty string is a key like any other.** The fuzzers found the Go implementation reading the key `""` of a record type as "no key": `b as {"": string}` gave `{}` and `b is {"": null}` an internal error. They also found Rust printing a number one digit off where two shortest texts tie; the tie takes the even digit. 100 cases pin both. A tie exists only for a double in [2^30, 2^51). |

## 15. Round 14: the first host

The workflow engine was ported from expr to Risor v3 on a local branch (plan
`14`, M6). Its own tests pass: 479, with 3 skipped. Its soundness fuzz test
passes at full size: no definition that loads fails with an expression
error. Nothing in Risor gave a wrong result or crashed. Running a workflow
costs the same as with expr; loading one took 1.9 times as long.

| # | Ruling |
| --- | --- |
| R178 | **A new package, F14, host facts.** Where alternative steps converge the engine knows that at least one of them ran, and nothing a host could declare said so: `report ?? approve` stayed nullable. An environment may now declare a group of variables of which at least one is not null in every run. It is part of the precondition, and it only adds. |
| R179 | **The Go binding exports a structural type API.** `Type` was opaque and a check result's type was text, so the engine rebuilt the type model to build and join types. Constructors, accessors, Join, Fits, parse and print are exported, and a check result gives a `Type`. |
| R180 | A child environment in Go shares its parent's tables, a variable declared from a `Type` costs no parse, and a checked program compiles without a second check. The target is the engine's load within 1.2 times of expr's. |
| R181 | No literal types for numbers: a `match` on a numeric enum is not checked for coverage in 3.0. No fourth schema option for a default that replaces an explicit null: that is the engine's own rule. |
| R183 | **The shape of F14.** A host fact is part of the precondition, as a declared type is; nothing tests it at a run. Its one kind in 3.0, `someNotNull`, names variables of which at least one is not null. The checker uses it through one mechanism, a fact "is null" for such a variable: a test against null gives it, and in `a ?? b` the right operand is checked under it. Its one effect: a member is not null where every other member is known to be null. So `report ?? approve` is not null, and so is `approve` where `report == null`. F14 is offered after F13 and before F12. |
| R184 | F14 as written: 176 cases in `suite/facts/`. `p is null` and a `match` arm of `null` alone also give the fact. Declaration errors are `bad-name`. An environment with no host fact is unchanged; one that gains a host fact may get the reports a narrower type gives (a constant condition, `never-equal`), as one whose declared type loses its null does. `report?.verdict ?? approve.verdict` does not narrow in 3.0: a sound rule for it exists and can be added later. |
| R185 | The re-port on the new Go API: the engine's type model (546 lines) is deleted, and its load takes 0.99 times expr's, from 1.95. **A type constructor of the Go binding returns a type and no error:** a result past the limits is an invalid type that every declaration refuses with `bad-type`. |
| R186 | **The first host, at the end.** With host facts declared for converging steps the engine's tests pass, 486 with one skip (a `match` on a numeric enum, R181). Its load takes 1.12 times expr's and a run 1.08 times; two thirds of the load is the engine's own code. Of 1,500 generated workflow definitions expr loads 747 and Risor 567. The gap is one rule: `len(x)` and `x.filter(...)` where `x` may be null, which expr answered with 0 and an empty list and Risor reports as `maybe-null` (about 170 definitions); the rest are constant conditions, a class a host may choose not to reject. The rule stays: null is not an empty list, and the author writes `len(x ?? [])`. Nothing in Risor gave a wrong result or crashed in any pass of the port. |

## 16. Round 15: the fuzzers on the packages

The fuzzers were extended to the forms of F4 to F7 once all three
implementations offered them. Each finding was a difference between two
implementations that no suite case pinned. None broke the checker's promise.
R189 records the first two. All three implementations offer F8 at the end of
this round, and Go offers every package.

| # | Ruling |
| --- | --- |
| R190 | **The edges of the step `[*]`.** The receiver of a first star in parentheses has the span of its node, without the parentheses, for `kind`, `maybe-null` and a `dynamic` site; a later step's receiver includes them. A read after a star whose receiver carries a `dynamic` site fails with class `dynamic`. A projection of a field of type `never` is `never[]`, a list, and a method call on it is checked. 47 cases. |
| R191 | **Which targeted parse code wins.** Where one diagnostic meets rows in more than one section of the errors chapter, the first row in order wins. Two exceptions, on which all three implementations already agreed and which give the better message: a token after a complete `if` or `try` is tried after every row of 2.7, and a token after a program type is tried before `a?[i]`. A condition on what the parser is reading means the innermost open production. 59 cases, one for each pair of rows that can both apply. |
| R192 | **`=` after a map key is a wrong separator, not a block.** A lambda body `{ a = 1 }` reads like a statement and is `block-without-const`. `{ "a" = 1 }` and `{ [k] = 1 }` do not: they are `unexpected-token`, inside a lambda body or not. So that the message helps, `unexpected-token` gains a second template, "a map key is followed by :, found …", and no new code. |
| R193 | **What F11 is.** F11 is a binding's typed layer: a faster path for the binding's own typed data. It changes no result, so it has no chapter, no table row and no cases. A binding that reads its host's data in place offers F11 when it offers F10 (`spec/core/03-host.md` 3.11). |
| R194 | **Four points of the testing package, raised by a second binding.** A stub with no `type` is checked against the base at load, so every call that loads a file takes the base. A file built from parsed data has no updated text. In update mode the file is written once, when every selected case has run. A runner case runs once, and is never `skip` for lack of a static-rules pass (`spec/packages/f09-testing.md` 7.1 to 7.4). |
| R195 | **A case runs in the base of its load.** The case-level `run` of each binding took a base that had to equal the base of the load, and a different one could only be an error that no verdict status names. The parameter is gone: Go `Case.Run(ctx)`, TypeScript `defineCases(test, file)` and `Case.run()`, Rust `CaseFile::run(vars)` and `Case::run(vars)` (`spec/packages/f09-testing.md` 7.2 to 7.4). Go's update mode writes only when every selected case has run. |

## 17. Round 16: the fuzzers with every error code

The fuzzers' filter of F8's targeted parse codes was removed once all three
implementations offered F8, and messages and traces were compared. Nine
differences, all in a parse code or a message; no value, count, type, site
or trace differed, and the soundness run found no violation. 29 cases pin
them, in `suite/errors/fuzz-round-7.json` and `fuzz-round-7-check.json`.

| # | Ruling |
| --- | --- |
| R196 | **Errors on a union read the joined record.** A field read, or a field of a map pattern, on a tagged union reads flat(U), so `no-field` and `maybe-absent` print that one record and never the union (`f08-errors.md` 3.5). A star on a receiver of two kinds reports `kind` with the `receiver` template; `star` is for one kind that is not a list. |
| R197 | **Where a targeted parse code applies.** The start of a map entry is not a place where an operand must begin, so a prefixed string, `<` or a tagged template there is `unexpected-token`. A `match` literal is not an operand for `assignment`; a pattern default is. `block-without-const` follows a lambda's `=>` only, and not inside an open computed key. An unclosed interpolation names the innermost literal still open (`f08-errors.md` 2.2, 2.7, 3.4). |
| R198 | **Facts apply only to paths.** An index read on a value that is not a path took facts about the first `const`, and so lost its type in a branch; it found no unsound result, but it hid an `arity` error. A fact applies to a path (`05-types.md` 10.4), and to nothing else. |

## 18. The alpha on `main`

The owner chose to publish v3 as an alpha and request for comment on `main`.
v2 moved to the `v2` branch, cut from `main` before the switch. The Go module
path `github.com/deepnoodle-ai/risor/v3` resolves only from a `go.mod` at the
repository root or in a `v3/` directory, so the Go implementation moved out
of `go/`.

| # | Ruling |
| --- | --- |
| R199 | **The repository layout.** The Go implementation is the module at the repository root, path `github.com/deepnoodle-ai/risor/v3`, tagged `v3.x.y`. `ts/`, `rust/`, `fuzz/`, `spec/` and `suite/` stay where they are. v2's code is removed from this line; it lives on the `v2` branch. This replaces the layout row of section 1. Replaced by R200. |

## 19. The three implementations side by side

The owner ruled that Go, TypeScript and Rust are laid out alike: no
implementation sits at the repository root (C1). Go is the only one whose
package path follows the repository's layout, and the owner chose a
`github.com` path over a custom domain. A module in a subdirectory takes that
directory into its path, and its version tags carry the directory as a
prefix (the Go modules reference, "Mapping versions to commits"; checked
in a scratch clone with Go 1.26 in direct mode). The same check found that a
bare `v3.0.0-alpha.2` tag at the root breaks the old path: with no root
`go.mod`, Go lists the tag as an invalid version of
`github.com/deepnoodle-ai/risor/v3`, and `@latest` and `go list -m -versions`
fail for it.

| # | Ruling |
| --- | --- |
| R200 | **Each implementation in its own directory.** The Go implementation is the module in `go/`, path `github.com/deepnoodle-ai/risor/go/v3`, tagged `go/v3.x.y`; no bare `v3.*` tag is pushed again. `ts/` and `rust/` are unchanged. `v3.0.0-alpha.1`, tagged at the root path `github.com/deepnoodle-ai/risor/v3`, is the only release at that path; it is not retracted and gets no successor there. This replaces R199, and the layout answer of `14` section 7, question 6. |

## 20. One public surface

The three bindings had grown three public APIs: the same concepts under
different names, the same step in different shapes, and, in TypeScript and
Rust, internals exported beside the host API.

| # | Ruling |
| --- | --- |
| R201 | **The bindings are idiomatic and otherwise aligned.** A binding differs from the others only in the case of names, how an environment is declared, how a failure reaches the host, how a run is cancelled, and conveniences for the host's own types. Everything else is the same: the concepts, their names, where each belongs, what it takes and what it gives. Each root exports the aligned surface and nothing else. `05-host-api.md` Part E is the authority, and replaces Parts B to D where they disagree. |

## 21. The TypeScript type of a Risor type

A TypeScript host wrote each variable's type twice: as a TypeScript type for
its own code, and as type text for Risor, with nothing to keep the two in
step (#627). Go and Rust derive the Risor type from the host type. TypeScript
cannot, because its types do not exist at run time.

| # | Ruling |
| --- | --- |
| R202 | **TypeScript infers the host type from the Risor type.** `Infer<X>` gives the TypeScript type of a `Type<T>` or of type text, which the TypeScript checker reads. `createEnv` and `env.with` give `Env<V>`, and `env.compile` gives `Program<V>`, whose `run` takes V's variables. These are conveniences of E.1 row 5 in `05-host-api.md`, with no new concept, in the types only. **Open, a recommendation the owner has not confirmed:** a `Type` is also a Standard Schema (version 1), so the libraries that take one take a Risor type. That part adds the run-time member `~standard` to `Type`, which Go and Rust have no counterpart of, so whether it is a convenience of row 5 is the owner's decision (AGENTS.md, Decisions). Type text stays the one way to write a type; ArkType shows that a type-level reader of such text is practical (`ran`: an 80-field record adds about 64,000 instantiations and no measurable check time, TypeScript 5.9.3). Part E, after E.7, has the rules. |

## 22. The adjustments of 2026-11-19

Plans 16 to 20 were written on 2026-11-14 from the review of #640 and #641.
On 2026-11-19, after a further review and #644, the owner withdrew D52,
trimmed D54 to D56 and settled D57 and D58. These rulings record what follows.
Each lands with the track of `14` 5.3 that it names. R214 to R221 record
track B as built, R222 to R231 step E0 of track E, R232 to R246 slice D1, and
R247 to R257 step E1. R240 to R246 were made when the three implementations
of slice D1 were compared, and R255 to R257 when those of step E1 were. R258
and R259 were made when the pull request of slice D1 was reviewed. R260 and
R261 close the two questions that tracks B and D1 left for the owner; they
were made on 2026-11-19, when the plan's status was brought up to date. The
owner was shown both questions that day and answered that the work proceeds
as intended. R262 to R279 are kept for step E2. R280 to R297 record step E3 as
written and built in the three implementations, and R293 to R297 the changes
that the review of its implementation asked for.

| # | Ruling |
| --- | --- |
| R203 | **One plan, held in `00` and `14`.** The decisions are rows of `00` section 4 (D52 to D60) and the work order is `14` 5.3, tracks A to F. No other document overrides them. Plan 16 is removed. Plans 19 and 20 keep a table of what was dropped and why (`19` 8, `20` 6). |
| R204 | **Function parameters (D57) come with static rules.** A parameter typed with an `Fn` may be called and may take a lambda argument; an `Fn` takes no function, so every parameter of a function type is first-order. Every compile applies the rules of `18` 7.1, the review's (a) to (d) among them: a lambda that has an `Fn` parameter is never a lambda argument; a call of an `Fn` parameter has the `Fn`'s count; a lambda at an `Fn` has no more parameters than the `Fn` lists; the shape test counts the `Fn`'s parameters. So an unchecked run never calls a value or a lambda with the wrong count. The codes of `18` 7.1 rules 3 and 8 are recommendations. Track E, step E3. |
| R205 | **Per-call frames are the reference path** in every evaluator, and every implementation runs the whole suite in frame mode. One fixed place per name stays only as an optimization, for a program with no recursive lambda and no `Fn` parameter, and only if benchmarks show it is needed (`18` 8). Even a lambda that is not recursive is re-entered through an `Fn` parameter: `apply(y => apply(z => z, y + 10), 1)` is 23, and fixed places give 33. Track E, step E1. |
| R206 | **The largest depth a host may set is 400 (D58),** with the default at 200. It replaces R20 and the range of R76. The fallback is 300, if the TypeScript binding cannot run the deepest forms at 400 with 1.5 times headroom in a browser Web Worker. If 300 fails the same test, the limit is the largest multiple of 50 that passes it, and not below the default of 200. Each binding's stack test (`spec/core/03-host.md` 3.9.1) covers calls that a library entry makes of a lambda, not only direct calls, with 1.5 times headroom. The measurements are in `00` section 4. Track B. |
| R207 | **The scope of a lambda's own name lands with annotations.** It and `recursion-needs-types` are a Core amendment (`18` 3.1), and they land in the same change as slice D1, not with the other Core amendments of track B, so that no build asks an author for types it cannot parse. Recommendation, not a ruling: step E1 lands before the alpha that carries D1 is tagged. |
| R208 | **An `Fn` annotation is read by every compile.** It changes the static rules, so a compile without the type rules reads it, where it ignores every other annotation's claim (`17` 3.2). An `Fn` stands on a parameter only, as a name with no default; a result annotation and a `const` line's are never an `Fn`, and an `export const` has no `Fn` parameter (`bad-export`) (`17` 3.1, 3.3). Slices D1 and D2. |
| R209 | **Beside an `export const`, a top-level `const` that binds no name is `bad-export`.** No entry point can need it (`17` 3.5), so an entry call would skip it in silence, and a guard written as `const _ = ...;` would never run (`17` 3.3, 17-15). Slice D2. |
| R210 | **Descriptions travel through identity only.** A description stays on a field that is passed on unchanged; a Join, a Meet, a Pick or a Merge of two sources builds fields with none (`17` 3.6). It replaces the first draft's rules for Join and Meet. Slice D3. |
| R211 | **Package numbers are not renumbered.** F15 stays unused, as F11 keeps its number with no chapter (R193). F16 is built in three slices, D1 to D3, with F17 between D1 and D2. F18 is kept for host opaque interfaces, a design track. F19 is author feedback. The Go helpers of `19-13` have no number. The order of offering is F3, F1, F2, F4, F5, F6, F7, F8, F9, F10, F11, F13, F14, F16, F17, F19, F12; F18 joins it when designed. How the slices of F16 and the steps of F17 are offered was left open here; R261 settles it. This extends R173 and R183. |
| R212 | **Absence under `??` reaches every operand of a chain but the last,** `a.f ?? b.g ?? c` coalescing both reads, and a key that a closed record cannot have stays `no-field` (`20` 3.1). The exception to `spec/core/03-host.md` 3.4.8 item 2 stands (R260). Track B. |
| R213 | **No `go/` tag before the Core amendments.** There is no `go/v3.0.0-alpha.N` tag yet (R200). The first is cut after track B, once the workflow engine, ported onto the amended Core, passes its own tests and its soundness fuzz test (`14` 5.3). |
| R214 | **400 stands; the fallback of D58 was not needed.** After the stack work, the deepest form of each binding runs at 600 levels (1.5 times 400): TypeScript in a Chrome 158 Web Worker reaches 771 for the worst of 28 forms (136 before), Firefox 159 reaches 1,000 and WebKit 27.2 reaches 697 (ran: Playwright's builds, Apple M4 Max); Rust needs 691 KiB optimized and 1,859 KiB unoptimized at 400 (ran: rustc 1.100.0, aarch64 macOS); Go grows its stack. `03-host.md` 3.9.1 names the deepest cases and the stack divided by 1.5. |
| R215 | **A coalesced read is found from the node alone.** An operand of a `??` chain other than its last is one when its node is a non-optional `Member`, or a non-optional `Index` with a `String` key, with or without an `OptChain`. So a checked and an unchecked run agree, and a projection is never one (`01-language.md` 6.6). Only an absent key at the last step is forgiven: a null receiver is still `null-operand`, and an absent key at an earlier step is still `missing-field`. Steps and units do not change. |
| R216 | **Under `??`, a field with rest `never` stays `no-field`.** The column of `05-types.md` 9.2 for a coalesced read gives an optional field, or an unlisted one with a typed rest, its type with null added and no `maybe-absent`; rest `any` gives `any` and no site. The text of the `maybe-absent` template is not changed here: plan `20` owns it (20-3). |
| R217 | **A nested failure gives its first diagnostic's message, whole.** `function-error` takes it with no code or span in front, and a chain of nested runs gives the innermost message. The inner code, class, span, path and data do not reach the run, and `data` stays null (`03-host.md` 3.5.3). The inner failure is kept as the cause, which no rendering writes (F8 5.1 rule 3). Each binding names the cause its own way: `Diagnostic.Cause` in Go, `cause` in TypeScript, `Diagnostic::cause` with `Error::source` in Rust. |
| R218 | **A stub row's `error` may be a diagnostic object,** `{code, message}`, with exactly those two keys. It stands for a nested run's failure, and the runner turns it into the binding's own failure type (`03-host.md` 3.10.4 item 5). Any other object is an invalid case. |
| R219 | **The poisoned run cuts a run off from the host; it does not stop it.** A `catch` may catch the first error and the program may go on computing. Cancellation stops the run, and its limits bound it (`03-host.md` 3.2.5, informative). |
| R220 | **Methods of opaque values are not in the report of calls.** F2 3.6 item 6 no longer claims them; item 8 says how they show, and that a host caching on the report treats a read of an opaque value as volatile. |
| R221 | **No helper that spawns a thread with a stack.** A binding documents the stack a run needs and shows the lines a host writes. A helper would add to the host API of all three bindings (`05` part E). |
| R222 | **Step E0 went before D1, and F17 is offered with it alone.** The order of 3.11 item 2 is `..., F14, F17, F12` (R211). `spec/packages/f17-recursion.md` holds guards and `walk`; steps E1 to E3 add to it. How the slices of F16 and the later steps of F17 are offered stays open (`14` 5.3): this ruling does not settle it. The alternative, in the recommendation of `14` 5.3, was to hold E0's cases until all of F17 is built, which leaves a built feature with no shared cases. |
| R223 | **`walk` has a record file of its own,** `suite/library-recursion.json`, not a row of `suite/library.json` as `18` 9 listed. A package only adds, so Core keeps 53 entries; with F17 the library has 79, 60 of them in the group `core`. |
| R224 | **F17's formatter and assist cases stand in `suite/recursion/`,** in files named `tools-`, which a runner runs only when it offers F12. The one earlier case that lists every function moved to `suite/tools/until-f17-complete.json` while an implementation offered F12 and not F17; the file was deleted when all three offered F17 (`03-host.md` 3.11 item 5). |
| R225 | **A guarded arm is invisible to the rules about other arms.** Its literals are compared for `duplicate-case` only with the earlier literals of the same arm and of earlier plain arms; a later literal equal to one of them is no duplicate, is not ignored by `05-types.md` 11.2, and keeps `never-equal` and its facts (F17 4.1, 4.2). `18` 6 rules 5 to 8 give the rest. |
| R226 | **A guard's false facts hold nowhere, also after a guarded `_` arm.** `18` 6 rule 8. After `_ if (c)` a later arm is reached only when `c` was falsy, so its false facts could hold there; that would accept more programs and can be added later without breaking one. Not done: `if` and `else if` already write that chain. |
| R227 | **A value at the callback parameter of a function fails as an argument.** `walk(t, 5)` in a run compiled without the type rules is `wrong-kind`, template `argument`, before the call's step (F17 5.5 item 3). The template `callback` says "method", and a new static rule would add a code and settle `18` 7.1 rule 3, which is open. A checked program has `arg-mismatch`. |
| R228 | **`walk` makes one depth test for each call of its callback,** at the measure of its own call plus the node's depth. A list of children is not tested when it is entered (F17 5.4). So at nesting depth 0 with the limit L the deepest node whose callback runs has depth L − 1. A child is read as a host value after its 2 steps; a node given twice is visited twice. |
| R229 | **The seed rule reads with R in place of the type variable.** `05-types.md` 13.5 step 3a was written for `reduce`, whose R is its `Var`. For `walk`, whose R is `T[] \| null`, it fires when the body's type does not fit R under the first solution, or fits only by any, and the new solution is the Join of the first and what bind(R, B) gathers (F17 5.6). Read as written, `walk(0, x => [x + 1])` would get `T` = `int \| int[]`. For `reduce` the two readings are the same rule. |
| R230 | **In assist, a guard is in the names context with no expected type, and a string that only guarded arms list stays a candidate** for a later arm's literal (F17 7.2). It changes F12 3.11 item 1, which left out every string another arm lists. The formatter writes a guard as it writes the test of an `if` (F17 7.1). |
| R231 | **A string that an earlier literal of the hole's own arm lists is no candidate.** F12 3.11 item 1 said only "another arm", and the bindings differed: Go left `a` out of `match s { "a" \| "‸`, TypeScript and Rust listed it. Written again it is always `duplicate-case`, so it is left out, for a plain arm and for a guarded one. Found when the three were compared for F17. |
| R232 | **A value of type `any` does not fit an annotation.** `17` 3.2 gave an annotation Core's rule for `any`: the fit holds with a `dynamic` site. But an annotation runs no test, and the name then has the annotation's type: in `const x: int = blob; x + 1` a wrong `blob` would fail at `x + 1`, which is no site, against promises 1 and 4 of `05-types.md` 18.4. So the value must surely fit, the message is "expected int, found any", and `blob as int` is how such a value is tested (F16 3.3). The owner is asked to confirm; the alternatives are a run test at the annotation, which `17` 17-5 ruled out, or a weaker promise. |
| R233 | **Until step E1, every reference of a `const`-bound lambda to its own name is `recursion-needs-types`,** with or without types. The rule and its code are Core's; step E1 of F17 will accept the reference when every parameter and the result carry a type. No release is cut between slice D1 and E1 (`18` 3.1), so no author is told to add types that are then refused. A self-reference is a call by the name, or the name as a lambda argument; it is reported once for each lambda, at its first reference, and the self-call has the type `never`. |
| R234 | **A lambda is checked once when it has at least one annotation and every parameter is annotated.** `17` 3.4 said "every parameter annotated", which read alone takes in `() => e` and would change Core's results for it: a lambda that is never used would keep its sites. With a result annotation, `(): R => e` is checked once (F16 3.4). |
| R235 | **A type line is not a node, and an annotated `const` line takes no lambda.** `Program` gains the list `types`; `Const`, `Lambda` and `Param` gain fields; no node kind is added, so no walker changes (`17` 6 recommended this). `const f: T = x => x;` is a `parse` error: a function is not a value of a program type (F16 2.2 rule 4). |
| R236 | **A type line that fails declares nothing,** so a later use of its name is `undefined-name` too. `name-clash` is a new check code, in F16's catalog file, and the environment's type stands; a second type line of one name is `duplicate-name`, and the later line stands. Type names and value names are two scopes. A type line names only the lines before it until step E2 (F16 3.1). |
| R237 | **The finding of a lambda is exact** (`17` 9 item 1, confirmed): where an argument or a `const` value may stand, a parenthesized expression directly followed by `:` begins no expression, so `(` … `)` then `=>` or `:` is a lambda and no program changes its meaning. Two texts that were errors change their code or span (F16 2.2 item 1). |
| R238 | **The two narrowed parse texts change in place** in `suite/messages-parse.json`, so a build with F8 and without F16 prints them too; a catalog file has no `until-` form. The text of `type-declaration` says "write type Name = and then the type": a template holds no literal brace (`03-host.md` 3.8.2 item 6), so it cannot print `{...}` as `17` 3.1 asked. No narrowed code retires; the count stays 58. The first point is open (F16 1.5). |
| R239 | **The Core amendment's cases stand in Core's files,** `suite/core-run/static-recursion.json`, `suite/core-check/check-recursion.json` and `suite/analysis/scope.json`, and the six earlier cases it changes moved to files named `until-f16-`, which were deleted when all three implementations offered the slice (`03-host.md` 3.11 item 5). The rule is Core's, and an implementation has it exactly when it offers slice D1 (R207). Slice D1 is offered after F14 and before F17; its cases are the files `d1-` and `tools-d1-` of `suite/entry-points/`. The order of the slices stays open (`14` 5.3). |
| R240 | **A later type line of one name stands also when its type fails.** It is `duplicate-name`, it declares nothing, and the earlier line's declaration ends there, so a later use of the name is `undefined-name` (F16 3.1 item 4). The bindings differed: Rust kept the earlier declaration. "The later line stands" already said it; the text now says it for a line that fails. |
| R241 | **A `check-limit` of a lambda checked once is on the lambda, the whole of it.** The check has no use to report on (`05-types.md` 20.3 says "on the use"). The bindings differed: TypeScript reported it on the lambda's name (F16 3.4). |
| R242 | **At the end of the text, `type` and one word begin a type line,** whatever the word's first letter, and also when the word is empty. No expression goes on with a word after the name `type`, so the hole can only be a type line's name, which has no context: the list is empty (F16 7.2 item 7). The text said "an upper-case word", and the bindings differed: for `type ‸` and `type ty‸` TypeScript and Rust fell back to the environment's names, and listed the function `type`. |
| R243 | **In assist, a type line declares its name only to what follows it, and only when its type is written.** Its own name is no candidate in its own type, unless an earlier line has the name (F16 7.2 item 1; Go and Rust listed it). A line whose type the repair supplied declares nothing, so `type ‸Foo` has no hover (F16 7.3 item 5; Go and Rust printed the type `null`). With F17 step E2 a line's own name is a candidate in its type (R274). |
| R244 | **Comments at an annotation.** A comment after the `:` stays there, since the `:` is a kept token. An annotation is part of its parameter, so the trailing comments of its last token go after the parameter's comma. A parameter of a function type is as a field of a program type: its trailing comments go after its comma (F16 7.1 items 1, 3 and 4; F12 2.12 item 5). Go broke the first two, TypeScript the third, and TypeScript also kept the comment before the comma of a record type's field, against F12 2.12 item 5; `suite/tools/format-comments.json` now has that case. |
| R245 | **A hover on the name of a `const`-bound lambda that is not a call's name has the kind `lambda` and no type.** F12 3.13 had a row only for the name of a `Call`. As a lambda argument, `[1].map(f)`, and where a value is needed, `[f]`, the bindings gave three answers: `name`, `lambda` and no hover. A lambda is not a value, so the type is null; the declaration is the name in its `const` line. |
| R246 | **The `outcome` command writes `description` for a case with `types` too.** F4 9.5 says so; `suite/README.md` named only `schemas`, and Rust followed the README. The README is corrected. |
| R247 | **A lambda is typed when it has a result annotation and every parameter has one,** of a program type or of a function type, read from the text alone (F17 11.1). `18` 3.1 rule 3 did not say whether an `Fn` counts or whether an annotation whose type fails counts. Both do: an `Fn` is a parameter's type, and a rule that read the resolved types would add a second diagnostic to every misspelt type name, so `const f = (n: Nope): int => f(n); 1` has `undefined-name` alone. A default is no type. A typed lambda is always checked once (F16 3.4), since it has an annotation and every parameter has one. Step E1. |
| R248 | **A self-reference of a typed lambda passes `recursion-needs-types`, and every other static rule tests it as any reference** (F17 11.2): `arity` on a self-call, `lambda-to-lambda` at a self-call's lambda argument, `f(f)` included, `not-a-value` and `arity` at its name given to a function. `18` 3.1 rule 5 lets a recursive lambda give its name to an `Fn` of another lambda; that is step E3, and until then it is `lambda-to-lambda`, as for any lambda. So a compile without the type rules accepts and runs exactly the programs a checked compile accepts for this rule. Step E1. |
| R249 | **A self-call is a use of a lambda checked once, and nothing more** (F17 11.3). Its arguments are fitted by F16 3.3, its type is the result annotation, known before the body is checked, and it counts against no limit of `05-types.md` 20. A result annotation with a static diagnostic gives `never`, as F16 3.4 already gives for every call. `18` 3.2 said "a call records no site of its own beyond the call's own class", which reads as a site the call does not have; the text now says it records what any call of a lambda checked once records. Step E1. |
| R250 | **The message of `recursion-needs-types` names the fix,** in Core's catalog: `{name} is this lambda's own name; give every parameter and the result a type so that it may refer to itself, or give the lambda another name` (F17 9). It changes in place, so a build with Core and slice D1 and without F17 prints it too; no build is such a build, since F17 follows D1 in every implementation (`18` 3.1). The code, its parameter and its span do not change. Step E1. |
| R251 | **Step E1 changes earlier cases in place, with no `until-` file.** The one case it changes is the catalog case of `recursion-needs-types` in `suite/messages-check.json`, and a catalog file has no `until-` form (R238). No earlier case writes a typed self-reference, so no other outcome changes (F17 10 item 3). TypeScript and Rust take step E1 from the chapter and the cases before it merges, as D1 landed in all three at once; until then their runners fail the files `e1-` and that one case, and the fuzzer counts their differences apart (`-noe1` leaves the lambdas out). Step E1. |
| R252 | **Go's evaluator gives each call its own frame, on one path for every program.** The values of all frames are one slice; a call pushes a frame record of its base and its static link, and a name is compiled to the number of links to follow and an index. A lambda never outlives the call where it was evaluated (`02-evaluation.md` 11.1), so frames are freed in order, and a lambda argument reaches the frame where it was written by following links from the call that gives it. The calls a library entry makes of its lambda never overlap, so they share one frame: the first pushes it, each binds its names again, and it is dropped when the entry returns. No fixed-place path is kept (`18` 8, 18-17). Measured (`go/bench_test.go`, 59 benchmarks, six interleaved rounds, medians, darwin/arm64, Go 1.26.2): a frame for every call cost 10.3% in the geometric mean against fixed places; the shared frame of a library entry brings that to 2.9%, with no change in allocation. The largest costs left are 10% to 14%, in small lists given to `reduce`, `sortBy` and `any` and in `nested_100x100`. `fib(22)`, 57,313 calls, runs in 1.77 ms. Saving and restoring the fixed places around a self-call would also give E1's results, but it breaks once step E3 lets a lambda reach a parameter of another call of its own lambda (`18` 8, the `apply` example). Step E1. |
| R253 | **Depth through a library entry is the library call's measure alone.** `18` 3.3 says `n.children.flatMap(walk)` "adds the flatMap call's measure and then 1 for each level", which can be read as two for each level. `02-evaluation.md` 8.4 item 3 is the rule: the call that `flatMap` makes of the lambda adds the measure of the call of `flatMap`, and nothing more, so `[n - 1].flatMap(d)` at nesting depth 0 costs 1 for each call (F17 11.4 item 1; measured in Go, `suite/recursion/e1-run.json`). Step E1. |
| R254 | **F17's step E1 is section 11 of its chapter,** appended, so that the citations of sections 1 to 10 stand. Its cases are the files `e1-` and `tools-e1-` of `suite/recursion/`. The chapter states that an implementation offering F17 offers steps E0 and E1; how later steps are offered stays open (`14` 5.3). Step E1. |
| R255 | **In a completion list, a lambda's own name is a name of the scope that holds its `const` line.** F12 3.6 item 1 orders the candidates "from the innermost scope out, and in one scope in the order they are bound", and did not say where the own name of a `const`-bound lambda stands when the hole is in its body. Two implementations listed it with the names of its line's scope, at its place in the order bound; one listed it in a scope of its own, between the parameters and that scope. The `const` line binds the name (`01-language.md` 5 rules 4 and 6), so it belongs to that line's scope: in `const a = 1; const f = n => ‸` the order is `n`, `a`, `f`. F12 3.6 item 1 says so, and three cases of `suite/recursion/tools-e1-assist.json` hold it. Found when the implementations of step E1 were compared; the difference dates from slice D1. |
| R256 | **Where D4 of F8 2.6 is given.** Two differences in the code of a `parse` error after the last branch of an `if` or the handler of a `try` (`01-language.md` 3.4 rule 4), both older than step E1. (1) In the first branch of `? :`, one implementation gave `unexpected-token` for `c ? try { 1 } catch { 2 } (1) : 3`, because it met the missing `:` before the rule; two gave `needs-parentheses`. The `try` is an `Expr` there, and rule 4 says what may follow one, so it is D4. (2) For a `not` that `in` does not follow, `try { 1 } catch { 2 } not x`, one implementation gave `needs-parentheses` and two `unexpected-token`. The operator is `not in`; a lone `not` continues no operand, so it stays `unexpected-token`. F8 2.6 items 3 and 4 say both, with the tokens at which D4 is given, and nine cases of `suite/errors/parse-precedence.json` hold them. The first was found by the differential fuzzer, the second by probes around it. |
| R257 | **A field named by a reserved word is an operand like any other, for the targeted `parse` codes.** `a.if` parses (`01-language.md` 14.2), but two implementations read the word as the start of a form when choosing a code of F8 2.7. One gave `unexpected-token` for `a.if = 1`, because it did not count a reserved word as the end of an operand; the rule is E7, `assignment`. Another gave `try-form` for `a.try = 1` and `a.try x`, reading the field's name as the word of a `Try`; E6 names that word, so the first is E7 and the second stays `unexpected-token`. F8 2.7 item 8 says so, and seven cases of `suite/errors/parse-targeted.json` hold it. Found by the differential fuzzer and by probes around its finding; older than step E1. |
| R258 | **A predicate whose first parameter has a default narrows nothing** (`05-types.md` 13.8). A default takes the place of a null element, so the body tests the default and not the element: `xs.filter((x = 1) => x != null)` keeps a null, and its type is that of `xs`. 13.8 named only a pattern, `_` and no parameter, and read to the letter it narrowed here. Go did not narrow; TypeScript and Rust did, also for an annotated parameter, so a checked `xs.filter((x = 1) => x != null).map(y => y + 1)` failed with `null-operand` at no site. The pull request's review found it. The number follows R257, which step E1 took first. |
| R259 | **A hover on the name of a `name-clash` type line gives the environment's type also when the repair supplied the line's type** (F16 7.3 items 2 and 5). Item 5 said only that such a line declares nothing and has no hover; its examples have no type of that name in the environment. Rust gave no hover. |
| R260 | **The exception to `spec/core/03-host.md` 3.4.8 item 2 stands.** A coalesced read gives the default for an absent key, whatever the declared type says. Absence under `??` is in the owner's adjustments of 2026-11-19 (`20-1`, `20-2`), and the rule cannot be had without the exception (`20` 3.1). It is built in all three implementations (R212, PR #654). This closes the question R212 left. |
| R261 | **The slices of F16 take places of their own, in the order they are built; F17 keeps one place.** The order of offering is `..., F14, F16 slice D1, F17, F16 slice D2, F16 slice D3, F19, F12`, the recommendation of `14` 5.3. As built it ends `F14, F16 slice D1, F17, F12` (`spec/core/03-host.md` 3.11 item 2). A later step of F17 extends the package in its place, and every implementation offers the step in the change that adds it, so no runner offers a part of F17. This closes the question R211 left. |
| R262 | **Step E2 is section 12 of F17, and a list of named types is resolved together.** The type lines of a program, the named types an environment declares, the targets of one schema and the Go types of one `TypeOf` are each a list. A name may stand for itself and for any item of its list; the items on a cycle of uses are recursive, and each gets a reference, an alternative of a type with the item's name. In a type line's type a name stands for the line of that name at or before it, else the first after it. The components are resolved one at a time, each after those it names, so the order of the lines does not change a type (F17 12.1, 12.2). |
| R263 | **At a top place of a recursive item a name stands for a definition; at a guarded place, for a reference.** So a definition holds no reference at its top, and one level of unfolding is always enough. At the top of an item that is not recursive a recursive name stays a reference, so `type Maybe = Tree \| null` prints as `Tree \| null`. A cycle of top places only is the error of `18` 4.1 rule 2 (F17 12.2 items 3 and 4). |
| R264 | **In a program the errors of a list have a check code of their own, `circular-type`.** `18` 4.1 rule 2 says `bad-type`, which is a `declare` code with no span; a type line's error is a `check` diagnostic on the line's name, as F16's are. `circular-type` is static, as `name-clash` is, with two templates, `direct` and `union`, in `suite/messages-recursion.json`. An environment, a schema and `TypeOf` keep `bad-type`. The catalog then has 32 check codes (F17 9). |
| R265 | **A reference beside a type of its own kind is an error** (F17 12.2 item 6), new beside `18` 4. While a component is resolved its definitions are not known, so the Join of one of its references with a record, a list or another kind it shares cannot be made then: `type Node = {next: Node \| {end: bool}}`. Resolving twice would change the definitions the first pass gave. The error names the fix, a type line of its own for the union, where the name stands at a top place and counts as its definition. A complete reference beside a type of its kind is no error: it counts as its definition. |
| R266 | **A component fails whole, and silently but for its own errors.** A component with an undefined name or a cycle with no guard is not resolved, so the other texts of its lines give no diagnostic; when a line's text has a diagnostic, every line of the component fails, and only that line reports it. A name of a failed line of another component is `undefined-name`, as F16 3.1 item 5 says. So one mistake gives one diagnostic (F17 12.2 item 7). |
| R267 | **Null beside a reference stays as written.** `Json \| null` holds `null` even when Json's definition has it, and Join with `null` adds it, so `m?.tail` of `type Maybe = Cell \| null` prints `Maybe \| null`. Dropping it would need the definition, which a component being resolved does not have, and two spellings of one type would then arise from the order of resolution (F17 12.2 item 5). |
| R268 | **The bound counts the pairs a computation enters, 4,096 of them, not the pairs in progress at once,** and a pair that has failed fails at once when met again. `18` 4.2 says "holds at most 4,096 pairs in progress". That bounds nothing: two families of twelve types, each holding the next twice, take 4,095 entries to fit with twelve pairs in progress at most, and thirty levels take a billion. Counting entries bounds both the work and the native recursion of a computation; the number stays. Two pairs are one when their types are identical as written, so the count needs no structural comparison. A question for the owner: this reads "in progress" in `18` 4.2 as "entered" (F17 12.5 items 2, 7). |
| R269 | **Past the bound the check ends, with one `check-limit` on the innermost node in progress.** The template is `pairs`, in F17's catalog file. A fit made by a `const` line after its value is checked is on the value, and the fit of the result to the expected result is on the result. The check's other diagnostics are dropped, since its state when a computation stops in the middle is no state the rules define; the static rules' diagnostics stand. A program type, an environment and a schema report it as their other limits (F17 12.5 item 7). |
| R270 | **The order of visits is normative, and the chapter fixes it** (`18` 4.2 left it to the chapter). Fits: the alternatives of S in the order of printing, stopping at the first that fails; two records by T's fields in key order, the mark before the type, then S's other fields against T's rest, then the rests; F5's variants in the order of printing. Join: the elements, then the keys of both records in key order, then the rests. Equal and disjoint: the alternatives in the order of printing, the keys in key order. The suite holds a case for each choice whose other order would pass the bound (F17 12.5 item 6). |
| R271 | **Join with references, in steps.** Null alone adds null; a reference not yet complete is kept beside the other side, unfolded nowhere; two types identical as written give the first; `any` in an unfolding gives `any`; then "A fits B gives B, B fits A gives A", in that order, with no `any` used; a pair in progress gives `any`; else the references that share a kind with the other side are unfolded and the two are joined by kind. So `[a, b]` of two names with one structure has the second's name, and `[b, a]` the first's. A Join of many types with a reference folds from the left, each step a computation of its own (F17 12.5 item 4). |
| R272 | **A reference has depth 0 and size 1, and each definition is tested where it is declared.** `18` 4.2 measures depth "on the unfolding, stopping at each recursive name's second occurrence". That measure depends on the path to the name, so it cannot be kept with a type, as the constant-work rule of `05-types.md` 20.7 requires, and with names that refer to one another it takes work exponential in their number. A definition is at most 64 deep where it is declared, and the bound of 4,096 pairs, not depth, bounds the computations that follow references. `type Tree = {kids: Tree[]}` has depth 2 by both measures. A question for the owner (F17 12.6). |
| R273 | **Truthy keeps a reference whose definition cannot be falsy,** with no `null`, no `""`, no `never[]` and no record that can be empty; otherwise it, and Falsy always, work on the unfolding. nonnull unfolds a reference whose definition has `null`, and without(X, T) unfolds the references that share a kind with T. So `c ? 1 : 2` with `c` a recursive record is `always-truthy`, as for any record with a required field (F17 12.5 item 8). |
| R274 | **A recursive type prints by its name everywhere, and assist shows its unfolding.** A reference prints after the opaque names and before `null`. `no-field` and `maybe-absent` on a receiver that is a reference print the reference, so the message names `Node` (use 2 of `18` 4). A hover on a recursive type's name gives its unfolding, and in a type line's type the line's own name is a completion candidate: it replaces R243 for a type line's own name. A case's `type` compares a name that is no opaque type as an atom (F17 12.6, 12.8). |
| R275 | **A schema's recursive type is named by the last token of its pointer,** the first letter upper-cased, and the root by the named type it declares, the variable's name with its first letter upper-cased, or `Root`. A name that is no type name is `bad-type`, as is a second recursive type of one schema with one name. Only a target met again while it is mapped is named. While a schema is mapped no definition of its own is known: Pick, Meet and a tag test that would look into one are `bad-type`, and in `anyOf` the reference stands beside the other branches, so a linked list's `anyOf` of `$ref` and `null` maps. This reverses R112 (F17 12.4). |
| R276 | **Go's `TypeOf` names a recursive type after the Go type,** its first letter upper-cased. A struct field, an element and a map value are guards; a pointer is none, so `type P *P` is `bad-type`. A function whose signature `Func` derives cannot take or give a recursive type: the signature is text, and the host writes it in `Type` with a type declared by `NamedTypeOf`. Rust's derive keeps `bad-type`, as `18` 4.1 rule 6 names Go alone (F17 12.4). |
| R277 | **An environment's recursive types share one space of names with its named types.** Two recursive types the declarations reach with one name and two structures, or one named as a named type that it is not, are `name-clash`; two with one name and one structure are one type. The description lists each by its unfolding under `types`, and an environment built from a description resolves `types` as one list, so a description reads back (F17 12.4). |
| R278 | **The conformance function stops past the default depth limit, 200,** with `depth-exceeded`, class `limit`, no path and no span, as the JSON text of `03-host.md` 3.2 does. With a recursive type the type no longer bounds its depth (F4 7.1 item 2), and the function has no run whose limit could. `ConformAll` ends with the same diagnostic. A run's walk keeps the run's depth limit (F17 12.7). |
| R279 | **Step E2 changes earlier cases in place, and the fuzzer knows the step as one difference.** Eleven cases change and one is added (F17 10 item 4), with no `until-` file, since every implementation offers the step in the change that adds it (R261). Two schema cases would still fail only because a target's pointer token is one letter, which names no type; their targets were renamed, so the cases test the cycle, and the added case keeps the one-letter token, so the name rule has a case of its own. The fuzzer generates recursive type lines behind `-noe2`, and counts a difference in such a case as the known difference `e2` while TypeScript and Rust lack the step. |
| R280 | **Step E3 is section 13 of F17's chapter,** appended so that the citations of sections 1 to 11 stand; section 12 is kept for step E2, which is written beside it. Its cases are the files `e3-` and `tools-e3-` of `suite/recursion/` and the catalog file `suite/messages-recursion.json`. As with step E1 (R251, R261), TypeScript and Rust take the step from the chapter and the cases before it merges; until then their runners fail those files, and the fuzzer counts their differences apart as `e3-ts-rust` (`-noe3` leaves the forms out). Eight earlier cases moved to `suite/recursion/e3-earlier.json` with their new outcomes, with no file named `until-`. Step E3. |
| R281 | **The template of rule 8 names the callee.** `18` 7.2 proposed `{name} takes a function, so it cannot be passed as one`, with `name` the lambda passed. A higher-order lambda written in place, `xs.map((g: (x: int) => int) => g(1))`, has no name, so the template is `a function that takes a function cannot be passed to {name}`, with `name` the callee as written: the function, the method or the lambda (F17 13.5). Step E3. |
| R282 | **The codes of `18` 7.1 rules 3 and 8 are `arg-mismatch` and `lambda-to-lambda`,** recommendation 18-15 taken. A value at a function parameter of a call by name is a static `arg-mismatch`, as the count of a call by name is a static `arity`: the parameter is read from the text. So in `suite/recursion/` and `suite/messages-recursion.json` the static codes are Core's twelve and `arg-mismatch` (F17 8 item 5, 13.8); Core's count of twelve does not change, as F4's and F16's additions did not change it. A case there that expects `arg-mismatch` from a type rule stands in a file named `-check.json`. Step E3. |
| R283 | **The hints are chosen by the form of the binding.** `fn-hint` and `parameter-at-fn` name the fix "give it a function type". They are chosen when the binding is a parameter written as a name with no default, annotated with a program type or not: the form a function parameter takes. A `const` name, a pattern's name, a parameter with a default and the name of `catch` keep the texts of Core and F16, whose fix differs. `parameter-at-fn` is also chosen for a plain argument at the `Fn` of a library entry, a type rule (F17 13.3), since `(xs, f) => xs.map(f)` is the text `18` 7.2 names. Step E3. |
| R284 | **A lambda argument in a call of a function parameter is `not-a-value`,** template `argument`, with `param` the listed word: each listed parameter takes a value, as a parameter of a function that is not an `Fn` does. `lambda-to-lambda`'s template says "a const-bound lambda", which a function parameter is not (F17 13.2 rule 7). Step E3. |
| R285 | **Rule 8 decides its argument alone, at every call.** A higher-order lambda given anywhere is `lambda-to-lambda` `higher-order`, also after `arity` on the whole call and at an unknown callee, and no other rule tests that argument: not `lambda-to-lambda` `default`, not `not-a-value`, not `arity` for its count (F17 13.2 item 1). One diagnostic for one mistake, wherever it stands. Step E3. |
| R286 | **Every fit against a function type is F16 3.3's: a fit that uses `any` does not hold.** That covers the arguments of a call of a function parameter, the values a function type passes to a lambda's annotated parameters, a lambda's body against the result, and a function parameter given on. A function type is a claim no run tests, so a `dynamic` site would let a call of the parameter have its result type while its value is anything, and a later failure would be at no site (F16 3.3 item 3). The one exception is the fit of `05-types.md` 13.5 step 3 at a library entry, where the entry takes the result and the site is on the argument, as F16 3.4 says of a lambda's result annotation. A consequence: `18` 7.6's `sumBy` over `any` with `o => o.qty * o.price` and the result `float` is `arg-mismatch`, since arithmetic on `any` has the type `any` (`05-types.md` 8.3). The author writes `as float`, or the result `any` (F17 13.3 item 2). Inside a lambda with no use, a call of a function parameter is fitted the same way, as a call by name of an annotated lambda already is there. Step E3. |
| R287 | **A call of a function type whose result is `unknown` has the type `any`,** the recommendation of `18` 7.3. A function parameter given to `filter` or `find` narrows nothing: there is no body whose facts to read (F17 13.3). Step E3. |
| R288 | **F16's template `fn-value` retires.** It was chosen for a value that the library passes to an `Fn` parameter of a lambda argument. With rule 8 no lambda that has an `Fn` parameter is passed, so no build that offers F17 can choose it, and no build offers slice D1 without F17 (R261). Its entry and its case leave `suite/messages-entry-points.json`, and F16's own chapter says so (F16 9.1). F8 3.1 item 9 says that no package's file changes or removes a template of another file; my reading is that it bars one package's catalog file from overriding another's, and that a step may change the file that owns a template when it amends that file's chapter, as R250 changed Core's template in Core's file. The alternative is to keep `fn-value` with a case that F17 can no longer reach. Put to the owner. The check templates go from 74 to 76: 72 in `suite/messages-check.json`, 1 in F16's file, 3 in F17's. R293 and R294 add two more in F17's file, so the step ends at 78. Step E3. |
| R289 | **Costs of a function parameter,** recommendation 18-16 taken. A lambda written at a function parameter of a call by name charges 1 step where it stands, as at a library call; the name of a `const`-bound lambda and the name of a function parameter given on charge nothing (`02-evaluation.md` 5.6 item 3). A call of a function parameter is a call by name of its bound lambda: 1 step, the depth test, extra arguments dropped at no charge. Its trace entry has the parameter's name and the `Call`'s span (F17 13.4). The `apply` case gives 23 in 10 steps with a depth count of 4. Step E3. |
| R290 | **Hover and completion of a function parameter.** The kind is `lambda`, and the detail or signature is the printed function type, `(x: int) => int`, with `unknown` printed as written and a type with a static diagnostic printed `never`. A hover on a call of it gives the call's type and the signature; on the name where declared or as an argument, no type (F17 13.7). A `const`-bound lambda keeps a null detail and signature: it has no function type to print. Step E3. |
| R291 | **Go keeps one frame stack for function parameters.** A lambda written at a function parameter is a closure over the index of the frame where it was evaluated, and a call of the parameter runs it on that frame. No function outlives the call where it was evaluated, so that frame is on the stack whenever the closure runs (R252, F17 13.4 item 2). `02-evaluation.md` 8.4 item 10's bound is not claimed for a program with a function parameter: a call of one runs a lambda that was checked where it was written, not expanded at the call. Step E3. |
| R292 | **Four readings of step E3 that the TypeScript and Rust builds asked about,** each now a case. (1) Rule 8 and a lambda that names itself: an untyped higher-order lambda that gives its own name as an argument is `lambda-to-lambda` and `recursion-needs-types`. The second is a rule of the lambda, not of the argument, so 13.2 item 1 does not remove it. Go and Rust read it so; TypeScript read item 1 to remove it. (2) A function parameter given to a library entry: where the seed rule checks the body once more, the fits of each Q are made once more. (3) A lambda checked once and named at a function parameter whose result is `unknown`: its body is a tested position, as on the library path (F16 3.4). (4) The `name` of `higher-order` at a function in a namespace is the whole name, `json.parse`. On (2) to (4) the three builds agreed with no case (ran, 2026-11-20). The cases are in `suite/recursion/e3-static.json` and `e3-check.json`. |
| R293 | **The message at a lambda given to a `const`-bound lambda names the fix that step E3 makes possible.** Core's `lambda-to-lambda` `default` said `a function cannot be passed to {name}, which is a const-bound lambda; pass the data instead`; with step E3 its reason is false, and `sumBy(orders, o => o.qty * o.price)` with `sumBy = (xs, f) => ...` is fixed by a function type, not by data. The implementation review found it. Rule 2 now chooses by the parameter that receives the argument. (1) A parameter written as a name with no default, with a program type or with no annotation: `lambda-to-lambda` `fn-hint`, a new template of F17's file, `{name} is a parameter; to pass a function to it, give it a function type, as {name}: (x: int) => int`, with `name` the parameter's name. The code has only the parameter `name`, and F8 3.1 item 8 lets a package's template use only the code's parameters, so the template names the parameter and not the callee; the span shows the call. A parameter with a program type takes the hint too, as at rules 1 and 3 (R283): `f: any` is a common way to ask for a callback, and the fix is to write its type. (2) A pattern, `_` or a parameter with a default: `default`, whose text changes in Core's catalog to `a function cannot be passed to {name} at a parameter that takes a value`. The form shows a value, and the parameter cannot become a function parameter without a different form, so the text states the fact and offers no fix; it is true with and without step E3, as R250's change is. Core's catalog case of `default` (`const ap = (v) => v; ap(inc)`) now takes `fn-hint`: it is that template's case, and Core's case is changed in place to `(v = 0)` (F17 9, 13.5, 13.9 item 7). Step E3. |
| R294 | **The name of a higher-order lambda read as a value has a message of its own.** Core's `not-a-value` `lambda` says `call it, or pass it to a library entry that takes a function`, and rule 8 refuses the second for a lambda that has a function parameter. F17's file adds `not-a-value` `higher-order`: `{name} is a const-bound lambda, not a value; it takes a function, so it can only be called`, chosen where Core gives `lambda` and the lambda is higher-order (F17 13.2 item 1, 13.5). The implementation review found it. Step E3. |
| R295 | **The hints are given only at a parameter of a `const`-bound lambda** (issue #668). R283 chose `fn-hint` and `parameter-at-fn` by the form of the parameter alone. Only a `const`-bound lambda may have a function parameter: a higher-order lambda written as an argument is `lambda-to-lambda` (rule 8). So `[1].map(x => x(1))` said "give it a function type", and the author who did so met rule 8. A **hint parameter** is now a parameter of a `const`-bound lambda written as a name with no default, with a program type or with no annotation (F17 13.1, 13.2 item 4); a parameter of a lambda written as an argument keeps `call-of-parameter` `default` and `arg-mismatch` `function`, at rule 1, at rule 3 and at a library's `Fn`. No earlier case stated the old outcome; new cases in `suite/recursion/e3-static.json` and `e3-check.json` state the new one. The implementation review found it. Revises R283. Step E3. |
| R296 | **The stack test has two deepest cases of step E3, the same in every implementation** (`03-host.md` 3.9.1; F17 13.4 item 6). The TypeScript and Rust builds each tested forms of their own, and the suite had no E3 case at 400. (1) A recursion through a call of a function parameter at every level: `const down = (f: (x: int) => int, n: int): int => n <= 0 ? 0 : f(n - 1); const d = (n: int): int => down(d, n); d(N)`, a depth count of 2N + 2, so `d(L/2 - 1)` reaches the limit L. (2) The chain of lambdas the review named: `const r = (f: (x: int) => int, n: int): int => n == 0 ? f(0) : r(y => f(y), n - 1); r(x => x, N)`, a count of 2N + 3, whose stack holds a call of `r` and a lambda of the chain for each level. Its count is odd, so the deepest run within L reaches L − 1; a form that reached L exactly would need a call that serves no purpose, and one level matters nothing to a test of the stack. Eight cases of `suite/recursion/e3-run.json` run both within and past the limit at 200 and at 400, with their steps (measured in Go, and counted by hand: 4N + 5 and 5N + 5). The implementation review found it. Step E3. |
| R297 | **A function parameter given to a method of an opaque type has a case.** `01-language.md` 7.2 says a method of an opaque type takes no callback, so test 4 of the shape test fails for any lambda argument, and F17 13.4 item 1 takes a function parameter's name as a lambda there. The three builds gave `wrong-kind` for `tok.put(f)` in a run without the type rules (ran by the review), but no case held it, and Rust's path for an opaque receiver changed in step E3. The case format states it with `opaque`, `methods` and an opaque value in `vars` (`03-host.md` 3.10.3 item 7): `suite/recursion/e3-run.json` holds the unchecked run, `wrong-kind` `not-callback` with the trace of the call of `g`, and `e3-check.json` the checked compile, `not-a-value` `argument`. The implementation review found it. Step E3. |
| R300 | **The rule for flat forms binds every pass.** `spec/core/01-language.md` 13.2 item 6 said that a long flat form parses and runs. It now names every part that reads a program: the parser, the static rules, the checker, the evaluator, analysis, the formatter, completion and hover. A chain of `as` or `is` is flat (F4 2.8 item 4), and so is a chain of projections each ended by a method call (F7 2.5 item 5). TypeScript threw a `RangeError` at 3,000 links of `as` and Rust aborted at 10,000 (ran, 2026-11-19, issue #649); both now pass 100,000 links through the parser, the checker and the evaluator, and their tools do not overflow the stack on the long forms their tests give them. The rule is about the stack only. It does not state time or memory: the formatter's text for a `? :` chain grows with the square of its length (F12 2.8, issue #665; ran in Go, 2,000 branches give 8 MB of text), and a run of more than 100 `==` formats to text that is nested too deeply to parse (F12 2.6, issue #666). The numbers R262 to R299 are kept for steps E2 and E3 of track E. |
| R301 | **An array's `items`, `prefixItems` and `additionalItems` each guard a `$ref` cycle** (F17 12.4 item 1). Each gives the element type (F1 3.5), so a cycle through it passes through a list's element, as 12.2 item 3 asks. All three implementations read it so (ran); three cases of `suite/recursion/e2-declare.json` hold it. The TypeScript implementer's question T1 of step E2. |
| R302 | **A schema's target is a place of the root, and a place has one type however the mapping meets it** (F17 12.4 items 1 and 2). The pointer names the place once decoded (F1 3.7 items 2 and 3), so two pointers reach one target only when their tokens are the same, and a target has one name; a place that holds a `$ref` is a target of its own. A place is being mapped from when the mapping meets it, through a `$ref` or as a part of the schema that holds it, as F1 3.7 item 7 said of "a schema whose mapping is in progress, the root included". Ran: Go and TypeScript mapped a place reached inside its parent apart from the same place reached by a pointer, and gave `{kids: Kids[]}` where Rust gave `{kids: Kids}`; Go now follows the text. The question T2; the orchestrator ruled that the rule be stated from the text of the schema, with no knowledge of an implementation's order of work. |
| R303 | **The order in which the mapping meets the schemas inside a schema is normative** (F17 12.4 item 3). Only a target met again is named (R275), so the order decides which. Ran: Go took the keys of `properties` in code point order, TypeScript and Rust in the order written; Rust took the names of `type` in an order of its own, Go and TypeScript in the order written. Keys go in code point order: JSON gives an object's members no order (RFC 8259), Go's standard decoder keeps none, and the description and 12.5 already order keys so. The names of `type` go in the order written, as an array has one. A rule with no order, naming every target on a cycle as type lines do, would change R275 in every implementation and make more schemas `bad-type` through Pick; it is a question for the owner. Step E2, question T2. |
| R304 | **A reference at a schema definition's top, replaced by its definition, joins nothing** (F17 12.4 item 5). A definition that then has two alternatives of one kind other than null is `bad-type`, as R265 makes a reference beside a type of its kind. Type lines differ: a reference of another component is complete there, and counts as its definition (12.2 item 5). The orchestrator's ruling on T3. Ran: Go and TypeScript gave `bad-type`; Rust joined the two lists. |
| R305 | **An Equal of two recursive types of one name that passes the bound counts as not equal, and a name is `name-clash` once** (F17 12.4, 12.5 item 7). Ran: all three gave `name-clash` for two schemas whose types of one name need 8,191 pairs to compare, and one type for 4,095, so the `bad-type` of 12.5 item 7 for an environment gains this exception. The count: for three types of one name and three structures Go and TypeScript gave two `name-clash` and Rust one; for a named type, a variable of it and a schema's type of that name and another structure, TypeScript gave two. One for each name does not depend on the order the declarations are met in, and one mistake gives one diagnostic (R266). Go now reports once for each name. Step E2, question T4. |
| R306 | **A name that is no recursive type hovers and is described as its type prints** (F17 12.4, 12.8 item 2): `type Alias = Tree` gives `Tree`, not the unfolding. The orchestrator's ruling on T5, as TypeScript built it. Ran: Go gave the unfolding for every alias, Rust for an alias with nothing beside the reference; all three described an environment's alias as `"Alias": "Tree"`. |
| R307 | **A reference that reaches a definition's top through a type that is not recursive is replaced by its definition** (F17 12.2 item 4), so no definition holds a reference at its top (R263). Ran: Go and Rust did so; TypeScript kept the reference, and then refused `const x: Xs = {k: []}` with `result-mismatch` and failed `as Xs` in a run. The Rust implementer's question R1 of step E2. |
| R308 | **For disjoint, a pair has failed when it is shown disjoint, and the computation keeps the normative order** (F17 12.5 items 5 and 6). A pair in progress is not disjoint, so the outcome kept is the opposite one, as for Fits and Equal. The order: the alternatives as they print, so the map before null; two records' keys once each, in code point order. Ran: TypeScript and Rust did both. Go kept no failed pair, tested null first, and tested each record's required keys apart, so that a key in both entered its pair twice; Go now follows the text. Step E2, question R2. |
| R309 | **A component with an undefined name and a cycle with no guard reports both** (F17 12.2 item 7): `undefined-name` and `circular-type`. Ran: all three. Step E2, question R3. |
| R310 | **Two unions of one structure join to the first's union, and the rule that joins fixes the order of Join** (F17 12.5 item 4, F5 4.1 item 4). F5 4.1 keeps the union when the two are the same by structure, and with references that is Equal. Ran: Go and TypeScript kept A's union; Rust flattened the two unless they were identical as written. Also ran: for `c ? a : b` with two names of one structure, TypeScript and Rust gave a's name, a Join(B, A); `05-types.md` 11.1 says Join(A, B), as Go does, and every other construct agreed. Step E2, question R4. |
| R311 | **A record that can be empty is one with no required field** (F17 12.5 item 8). Ran: TypeScript and Rust unfolded a reference whose definition has such a record, so `x \|\| 0` prints the record; Go unfolded only for `{}`. The text names a record that can be empty, and Go now follows it. Step E2, question R5. |
| R312 | **Pick's step 2 reads the kinds of each type, and step 3 looks into a reference only as far as its Fits does** (F17 12.4 item 4). Ran: Go and TypeScript picked `Node[]` from two `Node[]` and `string` from `string` and `Node[]`; Rust gave `bad-type` whenever a reference stood anywhere inside. For two records each with a field of the reference, Go and Rust gave `bad-type`, since Pick of the two fields reads the reference's kinds (12.5, first paragraph); TypeScript picked it. Step E2, question R6. |
| R313 | **The template `extra` prints as `expected` the record, and a record of a reference as the reference** (F17 12.6 item 4), whatever stands beside it. The catalog already says "expected is the record". Ran: all three gave `Rt0 does not allow key "x_1"` for `Rt0`; for `Rt0 \| null` Rust gave `Rt0`, Go and TypeScript `Rt0 \| null`. Go also printed the whole expected type for a record that is not recursive, `{a: int} \| null`, where TypeScript and Rust printed the record; Go now prints the record. Step E2, question R7. |
| R314 | **An opaque name in a definition is a kind of the reference** (F17 12.1). Ran: the three agree on reads, `is`, `as`, fits, joins, narrowing and `circular-type` with an opaque type in a recursive one, and no program found misses one. Go walks the opaque names of a type through definitions only to test a type a host gives by value (`TypeOf`), which a case cannot hold. Two cases of `suite/recursion/e2-check.json` hold the kinds. Step E2, question R8. |
| R315 | **A recursive type's name clashes with an opaque type's, and with a named type that is only an alias** (F17 12.4 item 2). The item named only the named types of the environment, so an opaque `Node` and a schema whose recursive type is named `Node` declared, and the description listed `Node` in both `opaque` and `types`, which no environment can be built from. F4 3.2 already makes a named type with an opaque type's name `name-clash`; a recursive type now follows it. And a named type `Tree = Other`, where `Other = Other[]`, is not the recursive type `Tree = Tree[]` that a schema names, though the two have one structure: the exception is for the named type that is that recursive type. Ran: Go declared both, and the review of PR #677 found the first in all three implementations; two cases of `suite/recursion/e2-declare.json` hold them. F4 8 item 4 also says that a description lists a recursive type in `types` with no named type. Step E2. |
| R316 | **A schema's own type follows 12.2 item 6** (F17 12.4 item 5). The item applied the rule only to definitions, so an `anyOf` of a reference to a recursive object and an inline object declared, and its description printed `{end: int, ...} \| Node`, which an environment built from the description refuses. The schema's own type is now tested as a definition is: a reference beside a type of its kind is `bad-type`; `Node` alone and `Node \| null` stand. The TypeScript build found it from the review of PR #677; a case of `suite/recursion/e2-declare.json` holds it. Step E2. |
| R317 | **The pair bound in a host signature is `bad-signature`** (F17 12.5 item 7). The table said `bad-type` for an environment; a signature's errors are all `bad-signature` (`03-host.md` 3.3.4), so a Join of a signature that passes the bound is too. The TypeScript build asked. Step E2. |
| R318 | **A target met again has depth 0 for the depth of a schema** (F17 12.4 item 1, F1 3.10). F1 3.10 item 1 counts the schema a `$ref` names as a subschema, which with a cycle has no end. A target met again is not mapped again (12.4 item 1), so it walks no subschema there: the closing `$ref` counts one level and its target none. Only that back edge counts 0: a target whose mapping has ended keeps its measured depth wherever it is met again, since the depth of a schema does not depend on where it stands (F1 3.10 item 5); two cases of `suite/recursion/e2-declare.json` hold this too. Ran: Go and TypeScript both give `Tree` for `{"type": "array", "items": {"$ref": "#"}}` inside 62 one-branch `allOf` objects and `bad-type` inside 63. Two cases of `suite/recursion/e2-declare.json` hold it. Step E2. |
| R319 | **A map literal with no tag against a union is reported with the template `missing`, the tag as its key** (F5 6). The row gave the place, the map, once, but not the template. The tag is a field every variant requires, so the catalog's `missing` ("a map with no entry for a required field") applies: `required field "kind" has no entry`. Ran: Go and TypeScript gave it; Rust gave the template `result`, `expected Shape, found {}`. The differential fuzzer found it on a recursive tagged union of step E2. Cases in `suite/unions/unions-form.json` and `suite/recursion/e2-check.json` hold it. |
| R320 | **The size of a source text (D63): its unit, its place and how it is tested.** (1) The unit is the code point, not the UTF-8 byte: `01-language.md` 6.4 and `03-host.md` 3.7.7 already count text in code points, and a UTF-8 count would favour Go and Rust (C1). The count is of the decoded text, a leading U+FEFF and each code point of a line end included, so it needs no lexing. A binding bounds the work: a text of at most N bytes or UTF-16 units is within N, and one of more than 4N bytes or 2N units is over. (2) `source-too-large` is a `parse` code with no span: the problem is the whole text, not a place in it, and a whole-text span would make a binding read to the end of an adversarial text. (3) The host's lower limit is an option of each call that takes text, not a declaration of the environment, because parse needs no environment (`03-host.md` 3.2.1 item 1); so an environment's description, and every case that states one, is unchanged. Its range is 0 to 1,048,576; outside it is `bad-limit`, except in completion and hover, which give their empty result. (4) The case format gains `limits.source`, read by the parse, by a template's parse, and by format, completion and hover cases (F12 7.1, 7.2). A format case's laws read the formatted text with the default limit, since `1+2` at a limit of 3 formats to `1 + 2`, which is longer. (5) The exact default limit is tested by unit tests in each binding, not by the suite: a case file would need 1 MiB of text or a repeat field, and the lowered limit tests the same code. (6) The flat forms of 13.2 item 6 drop from 100,000 to 10,000: an `if` with 100,000 `else if` parts is about 1.8 million code points. Tests that built such texts were cut to fit, to 40,000 forms where the text allows it (ran: Go `TestFlatPrograms`, `TestCheckFlatPrograms`, `TestManyTypeLines`; TypeScript to 10,000; Rust kept each form as long as fits, from 40,000 to 100,000, and its `check-limit` forms of 100,000 fields fit with three-letter keys). The quadratic `? :` form is formatted at 500 branches, since its formatted text at 2,000 is past the limit and could not be formatted again. Issue #635 closes: check, completion, hover and format are bounded by the size of the text, so the web service needs no deadline for them. |
| R321 | **The layout of a broken `? :` chain** (F12 2.8 item 5, D62). The owner chose one line for each link (`test ? branch`, then `: test ? branch`, then `: branch`) over a first draft that put every `?` and `:` on a line of its own. Rulings within it: a single `? :` is a chain of one link, so one that does not fit is `test ? branch` and then `: branch`, when that fits; a link that does not fit breaks alone, so the chain's group and each link's group are separate; a `? :` in a test or a then branch keeps its own group, and its depth is bounded by the nesting limit (`01-language.md` 13.2), so it cannot make the output grow with the square of the source; a then branch that is a `? :` is one level further in, since its `:` lines at the `?` would read as `:` lines of the outer chain. Ran: Go, TypeScript and Rust give the cases of `suite/tools/format-conditional.json`. |

## 23. The conformance function counts steps (D64)

Issue #678: host data that holds one list or map at many places made the
conformance function walk each place, charging nothing, so 24 levels of
sharing took 26 s in Go. The owner settled D64 on 2026-11-24: meter the walk,
remember nothing. These rulings record what the build had to decide.

| # | Ruling |
| --- | --- |
| R322 | **The limit is the default step limit, 1,000,000, or one the host gives, not the environment's** (F4 7.3). The brief recommended the environment's step limit. The function takes a value and a type, and no environment: a type may be made by `TypeOf`, from a schema or by the type algebra of F1, with no environment, and a type resolved in one environment can be used with another. Carrying a limit in each type would give one type two behaviours by how it was made, and would change the equality of types in Go and Rust. The depth limit of the function is already the default, not the environment's (F17 12.7 item 3), so the step limit follows it. A host gives another limit through an option, a natural place in each binding: `risor.ConformSteps(n)` in Go (variadic, so no call changes), `{steps}` in TypeScript (an optional last argument), and `conform_with` and `conform_all_with` with `ConformOptions` in Rust, which has no optional arguments. `NoLimit`, `null` and `None` turn it off, as for an environment's limits. A number below 0 is read as 0, as a maximum below 1 is read as 1 |
| R323 | **`ConformAll` first makes the walk of F4 4.2, and a limit there is its whole answer** (F8 6.4). The walk of 4.2 tests every other key of a record with a rest, also after a failure, so it can reach the limit after its first failure; All alone would then report that failure first, and break the agreement of F8 6.1. Go already made this walk first for the depth limit; the rule now covers both limits in all three builds. All then counts from 0 with the same limit, charging at the same places as 4.2, and appends the limit's diagnostic when it passes it. So the host's work is at most about twice the limit |
| R324 | **A config load's step 4 counts steps against the default limit; the decode of F10 6 and the JSON text of `03-host.md` 3.2.4 do not count** (F10 5.2 item 1). Step 4 is `ConformAll`, and a run can build shared data cheaply (`const a = [b, b]` at each level), so it is metered like the function. The decode copies, and the JSON text writes, a value whose size is its tree size; both stay unmetered. Step 4 bounds only the places that have a type other than `any`: it does not look under `any`, so a place typed `any`, and a load whose expected type is `any`, reaches the decode with no bound. The owner accepted on 2026-11-25 that the decode and the JSON text stay unmetered |
| R325 | **A conform case pins the limit with `limit` in `conform`, and the runs keep the base limits** (F4 9.2, 9.3). The function's count is the `steps` a conform case already states, so `limit` equal to it passes and one less fails. The runs of `v as T` and `v is T` keep the base environment's limits, so a case that pins the limit still compares their steps. A function that ends with `steps-exceeded` makes no run, and the case expects a failure of phase `run`; with `max`, `ConformAll` must give that one diagnostic. `depth-exceeded` has no such case yet (F17 8 item 6) |

## 24. Comparisons do not chain (D61)

The owner settled D61 on 2026-11-24: a comparison is not an operand of a
comparison without parentheses. Issue #666, the formatter's parentheses past
the nesting limit, is fixed with it. These rulings record what the build had
to decide.

| # | Ruling |
| --- | --- |
| R326 | **Comparisons do not chain, and the rule takes in every comparison operator** (D61, `01-language.md` 3.4 rule 13). `in` and `not in` stand at level 6 with `<`, so they are comparisons too: `a in b == c` and `a in b not in c` are refused. A mix of levels 5 and 6, `a < b == c` and `a == b < c`, is refused as well, so the two levels no longer decide anything between comparisons. `as` and `is` (F4) are not comparisons: `x as int > 3` and `x as T == y` stay valid. The rule follows the parser, which reads from the left: no comparison operator may stand in the right operand of a comparison outside parentheses, even under an `as` or `is`, so `x == a < b as bool` is refused. On the left an `as` or `is` ends the comparison, so `a < b as bool == x` is valid; refusing it too would break chains such as `c as any in [true] as any in [true]`, which the flat-form tests rely on. The formatter writes the pair on the right (found in review: `x == (a < b as bool)` had formatted to text that did not parse). The span is the second comparison operator in the text, `not in` whole. Five cases of `suite/core-run/parse-precedence.json` and one of `eval-compare.json` that chained comparisons now state the error, or keep their run-time check with parentheses. This is a Core amendment. |
| R327 | **The order of `chained-comparison`** (`01-language.md` 14.1 item 5). It is found at the second operator before that operator opens a level, so at a token that would also open level 101 it comes first. At one `in` where 3.4 rule 2 also holds, `a == !b in c`, rule 2 comes first: the rule about the left operand is tested before the operator is taken. Each implementation already tested rule 2 there. |
| R328 | **The formatter's pairs for the reader fit the nesting limit** (F12 2.6 items 3 and 4, #666). The 100 levels are the author's: every text that parses formats to a text that parses, and the pairs the formatter adds are its own to fit. The pairs of the second table are written only in a run of at most 3 operators that no such pair of another run encloses, through any brackets. So a token is inside at most K = 2 of them, those of one run of `*`, `/` and `%` (`((a * b) / c) % d`); an `\|\|` run adds 1. The second condition is what makes K a constant: without it, brackets let the pairs repeat, as in `(((a && b) \|\| c) && d) \|\| e`, one level for each bracket. The parser keeps one number, the highest level the parse reached, internal to each implementation, and the formatter makes one decision for the whole text: a source above 98 levels gets none of these pairs (2.6 item 4). This replaces two rules built first: one kept each node's deepest level and formatted twice, and one parsed the plain text of a deep source again to win its pairs back. The owner judged both more machinery than the edge case was worth. **Idempotence near the limit: settled by the owner, 2026-11-25.** Deciding on the source's level alone leaves two kinds of text that a second format changes. A source 97 or 98 levels deep whose deepest point is inside a short run gets pairs that take it to 99 or 100 levels, and that text formats without them. A source 99 or 100 levels deep only because of parentheses the formatter drops formats without pairs to a shallower text, which formats with them. Both reach a level above 96, and no source at 96 or less does: its pairs take it to at most 98. The owner accepted the exception over a second parse: F12 2.2 item 4 lets the pairs differ on a second format for a source deeper than 96 levels, and the runner's idempotent law skips such a source (7.1 item 4). The text still parses and keeps its tree. The suite pins the 97-, 98- and 99-level outputs. The `==` row of the reader's table is gone: a comparison inside a comparison is now in the first table, as the parser requires. |
| R329 | **A block comment after an opening bracket stands where the bracket's first token would** (F12 2.12 item 4, #655). The rule had named only the brackets of a sequence, so the test of an `if`, an index, a `match` guard and the empty parameters of a function type were left to each build: Go and Rust wrote `( /* why */c)`, TypeScript `(/* why */c)`, and for a branch TypeScript wrote `{/* why */ 1 }`. The rule now covers every opening bracket, the `<` of a map type included: no space before the comment, and one space after the last comment unless the closing bracket follows, so `if (/* why */ c)`, `a[/* why */ 0]` and `(/* none */) => int`. The `{` of a block, a branch and a `match` keeps a space before the comment, `{ /* why */ 1 }`, since a space or a break always follows that brace: a comment written against it would make the braces lopsided. The same change fixes a double space all three builds wrote between two block comments after a bracket, `f(/* a */  /* b */c)`. Ran: Go, TypeScript and Rust give the cases of `suite/tools/format-comments.json`. |

## 25. Parser and checker differences (issues #650, #659, #663, #672, #673)

The differential fuzzer and the work on recursive named types found five
places where the implementations differed or the text read oddly. These
rulings record what was decided.

| # | Ruling |
| --- | --- |
| R335 | **An `as` or `is` test ending in its type is a complete operand** (F8 2.2, E7). So `1 as string = 1` is `assignment` at the `=`. Ran: Go and Rust gave `assignment`; TypeScript gave `unexpected-token`, and now follows. The first part of #650, an `if` followed by `(` or `.` in the middle of `? :`, was already `needs-parentheses` in all three; three cases of `suite/core-run/parse-errors.json` pin it, and five of `suite/conversion/conversion-syntax.json` pin the `=`. |
| R336 | **An argument of exactly `null` is `arg-mismatch`, not `maybe-null`** (`05-types.md` 13.5 step 5, 15.2). `maybe-null` at a place of failure needs a type with null and something else. `rate(null)` read `null may be null; its type is null`; it now reads `expected string, found null`, as a `const` annotation already did. The rule is the same for a branch (`rate(c ? null : "a")` at the `null`), a library argument and an annotated parameter (F16 3.3). Operands are unchanged: `null + 1` stays `maybe-null` by 1.3, since the issue and the fix hints concern arguments. Ran: all three. |
| R337 | **A rejected `dynamic` site on a method's receiver ends the call** (`05-types.md` 13.3, 17). The receiver takes `never`, so the call records no `value` or `host` site; its arguments are still checked. Of the two readings of #663, this is the simpler, and TypeScript and Rust already gave it. Go gave a second diagnostic, `may-fail` on the call, and now follows. Three cases of `suite/core-check/check-sites.json` hold it. |
| R338 | **A named type's name is tested before its text** (F4 3.2 item 4). A `bad-name` declaration gets that one diagnostic; its text or schema is not read. Ran: for `X = X[]`, TypeScript and Rust gave `bad-name`; Go parsed the text first and gave `bad-type`, and now follows. A later declaration that names the bad one stays unspecified (item 2). |
| R339 | **An empty `types` is still the case's `types`** (F4 9.1 item 5, 9.5). It declares nothing, but it replaces the header's, so it cannot mean the same as an absent field; the `outcome` command writes the description for it. Ran: Go and TypeScript wrote it; Rust did not, and now does. |

## 26. Host API and conformance follow-ups (#656, #684)

Issue #656 came from the port of the workflow engine: three rough spots in
the Go host API. Issue #684 came from the review of #681: a conform case
could not state a list of `ConformAll` that ends with a limit.

| # | Ruling |
| --- | --- |
| R345 | **A `failures` entry states the limit diagnostic by its `code`** (F8 7.5, #684). With `max`, `ConformAll` can collect failures and then end with a pathless `steps-exceeded` (F8 6.4 step 3), which an entry with a required `path` could not state. An entry now has `path` or `code`, not both, and optionally `message`. An entry with `code` is the last, its code is of class `limit`, and it is the same as a diagnostic with no path and that code. The `outcome` line writes that entry as `{"code", "message"}` (F8 8), and F9's update mode writes it the same way, where before it wrote nothing for such a case. Before, the bindings wrote the entry differently on the `outcome` line (Go `"path":null`, TypeScript and Rust `"path":[]`); the fuzzer does not compare `failures`, so nothing caught it. Ran: Go, TypeScript and Rust give the new case of `suite/errors/conform-all-steps.json`. |
| R346 | **`risor.Func` takes a written signature, and declares a variadic Go function with one** (19-13, #656). `risor.Signature(text)` is an option of `Func`, and through `ForMethod` of `Methods`. It sets `Type` as assigning it after the call does, so a `var` initializer needs no helper. A variadic Go function still has no derived signature: deriving `...T` as a rest parameter would change the settled row of `19` 6.1, so it is left as a question for the owner. Instead the error is one of the derived signature only, as for an interface type: a written signature whose last parameter is rest declares the function. The rest arguments become one list, converted to the Go `[]T` as any list argument is, and an argument that does not convert is named by its place in the call (`argument 3 of join`). A rest parameter for a Go function that is not variadic, and a variadic Go function whose last parameter is not rest, are `bad-signature`. Go only: TypeScript and Rust have no `Func` (`19` 6.1 item 4). |
| R347 | **The analysis gives the calls of some names** (F2 3.6, #656). `CallsOf(names...)` in Go, `callsOf(...names)` in TypeScript and `calls_of(&names)` in Rust give the calls whose name is one of the names, in source order. A name is matched whole, as the call's `name` holds it, so a call in a namespace is `geo.distance`. With no names the list is empty. It is a convenience of the bindings, not a rule of F2, and it is in all three to keep them in step (`05-host-api.md` E.4). |

## 27. `indexOf` and `findIndex` (D66)

Issue #640, friction 7: with no regular expressions, a tool author who takes a
value out of text has only chains of `split`. The owner settled D66: three
Core entries that give null on a miss. These rulings record how they were
built.

| # | Ruling |
| --- | --- |
| R386 | **The three entries are Core, in the group `core`,** beside `find`, `startsWith` and `slice`, not in a package: they pass test T1 of `14` 2.1 as the other text and list methods do. Each is added at the end of its group, `04-library.md` 5.3.14, 5.4.23 and 5.4.24, so no section number moves. Core has 56 entries: 15 functions and 41 methods (14 of strings, 24 of lists, 3 of maps). With the packages the library has 82, 63 of them in the group `core`. `indexOf` names two entries, as `slice` does (`04-library.md` 1.1 item 3). |
| R387 | **`s.indexOf(search)` charges `sz(s)` steps and 0 units,** wherever the occurrence is and when there is none. The charge is fixed by the receiver before the search, as for `split` and `slice`, so it does not depend on how far a search went. The search obeys the bound of `04-library.md` 3.5 rule 5. The position counts code points, as `len` and `slice` do, so `s.slice(s.indexOf(t) ?? 0)` is exact on any text. The empty string is found at 0. |
| R388 | **`xs.indexOf(x)` compares by `==` and charges what `x in xs` charges** (`02-evaluation.md` 5.3), as each comparison is made. A formula symbol is added for it, `e` (`04-library.md` 4.2, 4.3): the formula grammar could not state the common prefix of two strings or the walk of `==` on two lists. The parameter is `x: T`, a type variable with no bound, so null is accepted at run time (2.4 item 4, 2.6). In the checker `x` binds `T` as any plain argument does (`05-types.md` 13.5 step 2), so `ints.indexOf("a")` checks, with the type `int \| null`, and gives null. `in` reports `never-equal` for the same mistake; giving `indexOf` that diagnostic would need a rule of its own in the checker, and is left for a later change. |
| R389 | **No start position argument.** `04-library.md` 1.2 lets a later version add an optional last parameter to an entry, so leaving it out closes nothing. Today `s.slice(i).indexOf(t)`, plus `i`, searches after a position. A start position would raise questions with no case behind them yet: whether `sz(s)` still counts the skipped text, and what a position past the end means for the empty string. |
| R390 | **`findIndex` follows `find` exactly:** the same callback, called with the element and its index; 2 steps for each element visited, then the call; it stops at the first truthy result; a callback's failure is the call's. It does not narrow, as its result is an index. Unlike `find`, it tells a null element that matches from no match. With D66 the row of `06` 2.8, "Deliberately absent", that left out `indexOf` and `findIndex` is removed, and so is the row `indexOf` of the foreign names in `20` 3.3 (`20-6`): `indexOf` is now an entry, so a `no-method` hint for it cannot arise. The `wrong` lists of the three records name `index`, `find` (of a string, as Python writes it), `strpos`, `search`, `position`, `indexWhere`, `findFirstIndex` and `find_index`. |
