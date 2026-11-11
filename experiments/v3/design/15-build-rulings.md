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
| R20 | The largest depth a host may set is 1,000. Steps and allocation limits are whole numbers of 0 or more. |
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
| R76 | A new `declare` code `bad-limit`: a limit that is not a whole number, or a depth limit outside 1 to 1,000. Core has 9 `declare` codes. |
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
| R112 | A `$ref` cycle is `bad-type`. `any` at the point of recursion could not be sharpened later without changing a result; an option that loads recursive schemas would only add. |
| R113 | Keywords the design did not list are given sound readings: `patternProperties`, `prefixItems`, `items` as an array, `nullable`, a `required` name with no property, siblings of `$ref` (ignored). References outside the document, `$id` below the root, `$dynamicRef` and `$recursiveRef` are `bad-type`. |
| R114 | An environment is described as a JSON object of seven keys, with every type and signature as printed text; an environment built from a description checks every program as the original does. There is no JSON Schema output. Printing a signature is defined. |
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
| R188 | Only a library record carries a `doc` in 3.0. Hover gives null for a host function and for a method of an opaque type; a binding does not invent one. Go built F12 against the cases and found two wrong cases, corrected. |
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
