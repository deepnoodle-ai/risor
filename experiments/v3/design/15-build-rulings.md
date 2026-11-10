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
