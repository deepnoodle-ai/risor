# Risor v3 design: error propagation and formatting

Status: plan accepted by the owner, with every decision of section 1 as
recommended (D48 in `00-overview.md`). `02` to `06` are revised to match. No
code was written and nothing was measured. Decisions are numbered `10-1` to
`10-8`. The owner decided two things by name: `try` becomes a block expression
with `catch`, as in Risor v2, and replaces `try(e, fallback)`; and the message
catalog covers every phase (10-6).

**How to read this.** Section 2 is the proposal, written as rules so that it can
move into the specification. A paragraph headed *Why* is not a rule, and the
other sections are not rules. **(R)** is read in a sibling document at the place
cited and **(I)** is inferred. In section 6, "checked" means read in that
project's own documentation for this draft, and "(from memory)" means not
checked. `02:` to `07:` are the sibling documents and `O:` is `00-overview.md`.

## 1. Summary of the decisions

| # | Decision | Recommendation | Strongest alternative |
| --- | --- | --- | --- |
| 10-1 | The form of `try` | `try { e } catch (err) { f }` and `try { e } catch { f }`. Both parts are branches, as in an `if`. It stands where an `if` stands. The name is a plain name. One `catch`, required. No `finally`. `try(` is a targeted error | A pattern in the parentheses, `catch ({code, message})`, which JavaScript allows. Later: it is an addition |
| 10-2 | The error value | An exact record: `code`, `message`, `path`, `data`. No new kind of value. **No `class` field**, which departs from the brief: the class of a failure depends on the checker's sites, and a program must not see what the checker inferred (`03:2.12`) | `class` as a fifth field. It needs a class that an evaluator can compute alone, which `03:2.14` does not have |
| 10-3 | What `catch` catches | It decides by code. Four things pass through: a limit, a cancellation, a crash (`function-panic`), and `missing-variable`. Every other run error is caught, including those a host sees as class `environment` | Catch `missing-variable` too, so that the list is "a limit, a cancellation and a crash", as `04:A.1` has it today |
| 10-4 | `fail` | `fail(message, data?)`. `data` is any value. It costs nothing and copies nothing. A caught error cannot be raised again unchanged in 3.0 | `fail(err)`, accepted only for the name a `catch` binds, raises the caught error again with its code, span and trace |
| 10-5 | Trace | A run diagnostic lists the calls in progress, from the outermost in. Specified and compared by the suite. A program cannot read it | No trace in the specification: each implementation gives what it has. Adding the field later is not a break |
| 10-6 | Messages | One catalog, `messages.json`, in the specification: a template for each code of every phase, with typed parameters. Implementations generate from it and the suite compares messages. `help` and `fix` stay unspecified | The run phase only in 3.0. It is the part a program can see and pay for. The other three phases follow |
| 10-7 | Rendering | Three forms, specified once: one line, detail, JSON | Specify the JSON form only, and leave the two texts to each binding |
| 10-8 | Every failure of a conformance test | A second host function that reports up to a maximum, in the walk's order. `as` in the language still reports the first | A maximum as a third argument of the function `05:A.13` already has |

## 2. The proposal

### 2.1 What it looks like

```ts
// A fallback for a bad value. The handler needs no name.
const n = try { int(input.count) } catch { 0 };

// The error as data.
const order = try {
  json.parse(msg.text) as Order
} catch (err) {
  match err.code {
    "not-conforming" => fail(`order: ${err.message}`, {path: err.path}),
    _                => null,
  }
};

// As an operand it needs parentheses, as an `if` does.
(try { int(input.count) } catch { 0 }) > 10
```

### 2.2 `try` and `catch`

```ebnf
Expr = If | Try | Binary [ "?" Expr ":" Expr ] .
Try  = "try" Branch "catch" [ "(" Name ")" ] Branch .
```

`Try` leaves `Primary`. `Branch` is the branch of an `if` (`02:2.2`). The part
after `try` is the **body** and the part after `catch` is the **handler**.

| Rule | Text |
| --- | --- |
| Branches | The body and the handler are each zero or more `const` lines, then one expression. A name bound in the body is not in scope in the handler. |
| The brace | Directly after `try`, after `catch` and after `catch (err)`, a `{` opens a branch and never a map. A branch written like a map, or left empty, is the targeted error it is for an `if` (`02:2.10`). |
| The name | A `Name`, in scope in the handler only. It may shadow as a `const` name may, and may not shadow a namespace. With no name, the parentheses are left out too. |
| Evaluation | The body is evaluated. If it gives a value, that is the value of the `try`, and the handler is not evaluated. If it fails with an error that `catch` catches (2.4), the handler is evaluated, with the name bound to the error value (2.3), and its value is the value of the `try`. Any other failure of the body, and any failure of the handler, is the failure of the `try`. |
| What stays charged | The steps and units the body charged before it failed, as today (`04:A.8`). |
| Place | A whole expression, at the level of `if` and `? :`. As an operand, a chain receiver or a `match` subject it needs parentheses. |
| Nesting | The braces of each branch and the parentheses of `catch (err)` each open a level (`02:2.16`). The two words open none. |
| Node | Node 26, `Try`, keeps its number. Its fields become `body`, `name` (or none) and `handler`. This plan adds no kind (there are 32 with the `Each` of `13`). |
| Words | `catch` is reserved. `throw` and `finally` are reserved for targeted errors, as `return` is. After `.` and as a bare map key each is a word like any other. |

Targeted errors, each of phase `parse`:

| Written | Message says |
| --- | --- |
| `try(e, d)` | write `try { e } catch { d }` |
| `try { e }` with no `catch` | a `try` needs a `catch` |
| `finally` | there is no `finally`: nothing in a program has an effect to undo; remove it |
| `throw e`, `throw new Error("m")` | write `fail("m")` |
| `catch (e: T)`, `catch ({message})`, `catch (_)` | the parentheses hold one name; write `catch { ... }` when the error is not used |
| `try e catch d` | the braces are required |
| `try { e } catch { d } + 1`, or a `.` or `[` after the handler | put the `try` in parentheses |

**Against TypeScript** (`02:3.1`). `try { f(x) } catch (err) { g(err) }` is
valid in both languages. There it is a statement whose value is discarded, and
here it has a value: the row of `02:3.1` about a block's value covers it. Two
differences are new rows. `catch` here does not catch a limit, a cancellation or
a crash. And `err` is a record and not an `Error`: `err.message` reads the same,
and `err.name` and `err.stack` are `no-field`.

### 2.3 The error value

In the handler the name is bound to a map with exactly these four keys. Its type
is an exact record.

| Field | Type | Holds |
| --- | --- | --- |
| `code` | The literal set of the codes that `catch` catches (2.4), taken from the catalog (2.9) | The diagnostic's code |
| `message` | `string` | The diagnostic's message (2.9). For `failed` it is the first argument of `fail`, the same value and not a copy |
| `path` | `(string \| int)[] \| null` | For `not-conforming`, the path of `05:A.6`. Otherwise null |
| `data` | `any` | For `failed`, the second argument of `fail`. Otherwise null |

- **It is an ordinary value.** `type(err)` is `"map"`. It may be compared,
  spread, rendered, returned and passed to a host function. There are still
  seven kinds of value (`02:2.4`).
- **A misspelled code is found by the checker.** `code` is a literal set, so
  `err.code == "not-conformin"` is `never-equal` and a `match` on it is checked
  for its cases (`03:2.9`). The set belongs to the specification version.
- **No span and no trace.** A position is for a person and for the host. A
  program that branches on a position breaks when a line is added above it.
- **No class.** `03:2.14` reports a null, kind or field failure as `dynamic` at
  a `dynamic` site and as `environment` anywhere else. Which it is depends on
  the types the checker inferred. If `err` showed it, replacing a declared type
  by `any` would change a value, against `03:2.12`, and an evaluator with no
  checker, which is how the Rust and TypeScript implementations start (`O:5.2`),
  could not fill the field. The code says what went wrong. The class stays on
  the host's diagnostic.

### 2.4 What `catch` catches

**`catch` decides by the code of the failure, and by nothing else.**

| Passes through | Codes | Why |
| --- | --- | --- |
| A limit, a cancellation | `steps-exceeded`, `allocation-exceeded`, `deadline-exceeded`, `depth-exceeded`, `cancelled` | As today (`02:2.12`). A handler would run on a budget that is spent, or after the host said stop |
| A crash | `function-panic`, and a failure inside the implementation | As `04:A.1` has it. The state of the host function is not known. `02:2.12` does not list it and must |
| A variable the host did not supply | `missing-variable` | New. No value caused it and nothing in the program can cure it. It is a fault in how the host is wired, and a handler would hide it on every run |

Every other run error is caught: a failed `as`, a failed conversion, `fail`, an
index out of range, a zero divisor, a number out of range, an error a host
function returned, and a null, a wrong kind or a missing field on data.

*Why not let class `environment` pass.* The brief asks whether a failure that
means "the host broke its promise" should pass through. It cannot, as a class,
for the reason in 2.3: whether a wrong kind is `dynamic` or `environment` is not
known to an evaluator, so two runs of one program on the same values would
differ by what a checker inferred. `missing-variable` is the one such failure
that has its own code and no value behind it. A host that wants to find a broken
promise tests its values with the conformance function before the run
(`05:A.13`).

### 2.5 `fail`

`fail: (message: string, data?: any) => never !`

| Matter | Rule |
| --- | --- |
| The diagnostic | Code `failed`, at the span of the call. Its message is the first argument. Its `data` is the second, or null. |
| Cost | Nothing beyond the 1 step of the call. Nothing is copied: the message and the data are values that already exist. |
| Bound | `data` is a value of the run, so the allocation budget bounded it when it was built, or it is host data. It leaves the run as a result does (`05:A.3`): it may share memory with the inputs, and the host treats it as read-only. |
| The message | The author's text, whole. It is not cut and not taken from the catalog. |
| Raising again | Not in 3.0. `fail(err.message, err)` raises a new `failed` error that carries the old one as data. The host then sees the code `failed`, and the span and trace of the `fail` call, not of the first failure. |

### 2.6 Checking

| Matter | Rule |
| --- | --- |
| Type | The Join of the body's type and the handler's type. |
| Sites | Sites recorded in the body are dropped, of every class, as they are today for the first argument (`03:2.14`). Sites in the handler are recorded. |
| Diagnostics | Reported in both parts as anywhere else. `try` is for bad values and does not relax the checker (`02:2.12`). |
| The name | Has the type of 2.3. |
| Facts | A fact in force outside holds in both parts. Nothing learned in the body holds in the handler. |
| A body that cannot fail | Not reported. A range error can occur at any arithmetic node (promise 3 of `03:2.14`), so the checker cannot show that a handler never runs. |

No check code is added: there are still 28. The errors of 2.2 are the parser's.

### 2.7 Cost and limits (C5)

| Case | Steps | Units |
| --- | --- | --- |
| `try` whose body gives a value | 0, as today (`04:A.3`) | 0 |
| A failure caught by `catch { ... }` | 0 | 0. No message is built |
| A failure caught by `catch (err) { ... }` | 1 + the size of the message, as for any text created. 0 when the message already exists: `failed` and `function-error` | 9 for a map of four entries (`04:A.4`). 1 + the size of the message, when it is built. 1 + `n` for a path of `n` steps, when there is one |

- **Charged as written.** The value is charged whether or not the handler reads
  the name, as an unused `const` is (`04:A.8`). The charge is made before the
  value is built. If it passes a budget, the run ends with that limit error.
- **This is why the message must be specified** (2.9). Its size is charged, so
  two implementations with two wordings would report two counts (L6).
- **A code** is text of the specification, as a literal is text of the program.
  It costs nothing. **A path** holds keys that already exist and numbers.
- **The message is bounded** by its catalog entry (2.9), and the path by 64
  steps (`05:A.6`). So `try { int("x") } catch (err) { 0 }` costs 2 steps and 10
  units, and with `catch { 0 }` 1 step and none. The counts are illustrative.
- **Leaving the calls between the failure and the `try` is not charged.** Each
  call in progress is left at most once, and entering it cost a step.

### 2.8 Trace

A failed run reports where it failed, as a span. Inside a helper or a callback
that is not enough: the author also needs to know which call, and which element.

| Matter | Rule |
| --- | --- |
| What | The diagnostic of a failed run has a field `trace`: the calls in progress when it failed, from the outermost in. |
| A call in progress | As `02:2.16` defines it for the depth limit: a call by name of a `const`-bound lambda, or a call of a library entry that is running a lambda passed to it. Nothing else has an entry: not a `try`, not a library call that takes no lambda, and not a host function, whose own failure is the diagnostic's span. |
| A projection in progress | A star whose reads are running on an element has one entry too (plan `13`). |
| An entry | `span`: the `Call` or `MethodCall` node, or the star. `name`: the `const` name, the entry's name, such as `map`, or `[*]` for a star. `index`: for a library call, the index the callback was given, from 0; for a star, the index of the element; for `mapValues`, whose callback is given a key, the position of the entry in key order; for a `const`-bound lambda, null. So an index is a number or null (`05:A.6`). |
| Length | At most the depth limit and the 100 levels of source nesting together: each call in progress counts at least 1 against the first (`04:A.6`), and the stars around the failure count against the second. It is never cut. |
| Cost | Nothing when nothing fails. An entry is written as the failure leaves each call. It is not charged: a diagnostic is not a value of the language (`04:A.4`), and the entries are bounded as above, each of a fixed size. |
| A caught error | Has no trace that a program can see (2.3). A failure raised in a handler has the trace of the place that raised it. |
| A limit error | Has a trace, and the suite does not compare it, because the position of a limit error is not specified (`04:A.1`). |

```ts
const total = o => o.lines.map(l => l.qty * l.price).sum();
orders.map(o => total(o))
```

With a product that is not finite in the second line of the third order, the
diagnostic is `number-range` at `1:37-1:52`, and its trace is `map` at
`2:1-2:26` with index 2, `total` at `2:17-2:25`, and `map` at `1:20-1:53` with
index 1.

### 2.9 Messages

Today a message is "one sentence" and is not specified (`05:A.6`). Three
implementations would word one error three ways, `err.message` would differ
between them, and so would its charge.

**One catalog, as data.** `messages.json` stands beside `library.json`
(`06:2.11`). It has one entry for each code of every phase.

```json
{"code": "not-conforming", "phase": "run", "since": "3.0", "caught": true,
 "params": {"expected": "type", "found": "value", "key": "string"},
 "templates": {
   "wrong":   "expected {expected}, found {found}",
   "missing": "required field {key} is missing",
   "extra":   "key {key} is not in the type; write ... to allow other keys"},
 "cases": [{"conform": {"type": "{id: string, qty: int}",
                        "value": {"id": "a", "qty": "2"}},
            "expect": {"conforms": false, "path": ["qty"],
                       "message": "expected int, found \"2\""}}]}
```

| Matter | Rule |
| --- | --- |
| A template | One sentence with no full stop at its end. A hole is `{name}`. No other brace stands in it. |
| Several templates | A code may have more than one, each named, where the failure has more than one shape. The entry says which condition takes which. `not-conforming` has the three shapes of `05:A.6`. |
| A parameter | Has a kind from the table below. Each kind has one rendering. A rendering longer than 64 code points is cut to its first 61 and `...`. So a message is bounded by its entry. |
| At run time | A parameter is taken from the value and from the text of the program or the environment. Never from an inferred type (`07:2.6`). |
| What a message does not hold | The path, which is a field (2.10 prints it). The span. A suggestion, which belongs in `help`. |
| Not from a template | `failed`: the author's text (2.5). `function-error`: the text of the host's error, as the binding gives it. Neither is cut or built. |
| Not specified | `help`, `fix`, `expected`, `found` and `cause`, as today. A "did you mean" needs one search algorithm in three implementations, and nothing a program can see depends on it. |
| `caught` | For a run code, whether `catch` catches it (2.4). The type of `err.code` is generated from this field. |
| Version | The catalog belongs to the specification version, as the cost table does. A run message is seen and paid for by programs, so it does not change within a major version (`05:A.10`). A message of another phase may change in a minor version. |
| Generated | Each implementation generates its table from the file. None writes a message by hand. |

| Kind | Rendering |
| --- | --- |
| `word` | A name, a field word or an operator of the program, as written |
| `string` | A string of the data, in its JSON form (`02:2.13`), with the quotes |
| `value` | A null, a bool, a number or a string in its JSON form. For anything else, the word `list`, `map` or `opaque` |
| `kind` | What `type(x)` gives (`06:2.8`) |
| `type` | Type text as `03:2.2` prints it |
| `number` | The rendering of `02:2.5` |
| `token` | In the `parse` phase: the token at the span as written, or `end of input` |

*What it costs.*

- **About 110 templates** (I; `05:A.6` now lists 115 codes: 8 `declare`, 58
  `parse`, 30 `check`, 19 `run`): 28 check codes, about 20 run codes, 8 `declare`
  codes, and the `parse` phase, where each of the 50 or so targeted errors of
  `02:3.2` needs one.
- **Every code must first have a name.** `02:7.1` (item 22) leaves the names of
  the run codes open. No document names a `parse` code other than
  `nesting-exceeded`, or the run codes of the failures that `02:2.12` gives in
  prose: a null operand, an index out of range, a `match` with no arm. That work
  is owed under P6 whatever is decided here. The catalog makes it due.
- **Three parsers must agree on the first error in a bad text.** The suite
  already compares its code and span (`05:A.6`). The catalog adds only the
  parameters, so the template of the general syntax error names the token found
  and not what was expected.
- **Messages get plainer.** `03:3` shows `input.role is never null or empty, so
  the right side of || is never used`. A template cannot follow each case that
  closely. The closer wording moves to `help`.
- **A poor run message stays for a major version.**

*What it buys.* `err.message` is one value everywhere and its charge is exact. A
user's tests can expect a message (plan `11`). The reference page of errors is
generated. A host can show its own wording for a code.

### 2.10 Rendering

Each form is a function of a diagnostic and the source text. Every binding gives
all three, and `risor` prints them.

| Form | Layout |
| --- | --- |
| One line | `error[` code `] ` start line `:` start column `-` end line `:` end column `: ` message. With a path that has a step: a space, `at`, a space and the path. With an origin (`05:A.6`): the positions are those in the document, with the origin's name and `:` before them. With no span, as from the conformance function: `error[` code `]: ` and the rest. |
| Detail | Line 1: the one-line form. Line 2: four spaces, then the source line that holds the start of the span, with each tab written as one space. Line 3: four spaces, one space for each column before the start, then one `^` for each column of the span on that line, at least one. Then `  help: ` and the help, if there is one. Then one line for each trace entry, the innermost first: `  in ` name `, element ` index `, at ` span, without the `element` part when the index is null. No color. With no source text, line 1 alone. |
| JSON | One object: `phase`, `code`, `class` (run), `message`, `span` as `[1, 37, 1, 52]`, `path`, `data` and `trace`, a list of `{span, name, index}`. A run diagnostic always has `trace`, which may be empty. Any other field that has no value is left out. `help`, `fix`, `expected`, `found`, `cause` and an offset may be added. It is written in the JSON form of `02:2.13`: no white space, keys in code point order. Where that form cannot write a part of `data`, such as an opaque value, the part is null. |

- **A path** is written from its steps: `.` and the key when the key is a
  `Word`, `[` and the key's JSON form `]` for any other key, `[3]` for an index.
  A key is cut to 64 code points, as a parameter is.
- **The word is always `error`.** There is no severity (`05:A.6`).
- **Several diagnostics** are rendered one after another in the order of
  `03:2.13`, each form ending in a line feed.
- **Columns are code points** (`O:T5`). A wide character puts the carets out of
  line in a terminal. That is accepted.

```
error[number-range] 1:37-1:52: the result of * is not a finite number
    const total = o => o.lines.map(l => l.qty * l.price).sum();
                                        ^^^^^^^^^^^^^^^
  in map, element 1, at 1:20-1:53
  in total, at 2:17-2:25
  in map, element 2, at 2:1-2:26
```

### 2.11 Every failure of a conformance test

`x as T` reports the first failure, and its cost is exact because it stops
(`07:2.3`). A host that loads a config file or checks a request body wants them
all. The host's function charges nothing (`05:A.13`), so it can go on.

| Binding | Form |
| --- | --- |
| Go | `func ConformAll(v any, t Type, max int) []Diagnostic` |
| TypeScript | `env.conformAll(value, typeText, max): Diagnostic[]` |
| Rust | `env.ty(text)?.conform_all(&value, max) -> Vec<Diagnostic>` |

| Matter | Rule |
| --- | --- |
| Order | The walk of `07:2.3`, going on after a failure. A list by index. A record's declared fields in code point order of their keys, then its other keys in code point order. A value is finished before the next is started. |
| A failure | The three shapes of `05:A.6`: a required field that is missing; a key an exact record does not allow, each one and not only the least; a value that does not conform where it stands. Nothing inside a value of the wrong kind is visited. |
| Other keys | Always reached. The first-failure walk reaches them only when every declared field passed. |
| Maximum | `max` is 1 or more. The walk stops when it has that many. A host that must say "and more" asks for one more than it shows. |
| Result | A list of `not-conforming` diagnostics, each with a path and a message and no span. Empty when the value conforms. |
| Agreement | The first one is the failure that `Conform` and `x as T` report, for every value and type. The suite holds the three together. |
| Cost | It is not a run and charges nothing. Its work is bounded by the value and its depth by the type, as in `05:A.13`. |

## 3. What it costs each implementation (I)

| Part | Go, lines without tests | TypeScript | Rust |
| --- | --- | --- | --- |
| Parser: `Try`, three words, seven targeted errors | 70 | The same | The same |
| Evaluator: the test by code, building `err` | 60 | One `try` in each `Try` node, as `04` Part C has it, testing the code of the thrown object | The caught and uncaught errors are two variants already (`04` Part D) |
| Trace | 80. A call node adds its entry on the path that returns an error. That path exists | A `catch` at each call of a lambda adds the entry and throws again. What a `try` with no throw costs in the engines is to be measured | `map_err` at each call. Free on success |
| Checker: the type of `err`, the Join | 40 | The same | The same |
| Catalog loader and generator | 200, and the data | The same file | The same file |
| Renderer: three forms | 150 | The same | The same |
| `ConformAll` over the walk that exists | 80 | The same | The same |
| Total new | about 680 | | |

Removed: the call form of `try`, and each implementation's hand-written message
strings.

## 4. Changes to the other documents

| Document | Change |
| --- | --- |
| `00` | Section 3: the `try` form in "Syntax". A section for this plan, and its decisions under 4.3 and 4.5. |
| `02` | 2.2: the `Try` production moves from `Primary` to `Expr`; `catch`, `throw` and `finally` join the reserved words; a `match` subject that starts with `try` needs parentheses. 2.3: the fields of node 26. 2.12: rewritten from 2.2 to 2.5 above. 2.14: `catch (err)` joins the constructs that create a value. 2.16: the braces of a `try` in the bracket row. 3.1: two rows. 3.2: seven rows. 3.3: a line on `try` in YAML, to be run. 3.4: the `try` row. Examples 23 and 57. 7.1 item 19 and 8.1 question 7. |
| `03` | 2.5 row 26. 2.14: "the first argument of `try`" becomes "the body of a `try`", in three places. The type of the error value, generated from the catalog. Examples 33 and 63. No new code from this plan (29 with the `untagged-union` of `09`). |
| `04` | A.1: the two rows on what `try` catches, by code. A.3 and A.4: the rows of 2.7. B.10: the trace on the error path. Parts C and D: a note each. Benchmark group E: a failure under 1, 10 and 100 calls, with the trace. |
| `05` | A.6: `message` is specified; the fields `data` and `trace`; what a conformance test compares; the complete lists of run codes and `parse` codes. A.11: `message`, `data` and `trace` in `expect`; `max` and `failures` in a conformance case. A.13: the second function. B.5, and Parts C and D: `Data` and `Trace` on a diagnostic, the three forms, `ConformAll`. |
| `06` | The signature of `fail`. Every `try(e, d)` in 2.1, 2.3, 2.8 and section 3. 2.11: `messages.json` beside `library.json`, and each `fails` line names its code and template. |
| `07` | The examples of 2.1, 2.3 and 2.6 that use `try(x as T, d)`. 2.8: the second function. |

**What the conformance suite tests.**

- **`try`:** for each run code, that `catch` catches it or lets it pass. The
  four fields of `err` for each. The steps and units with a name and with none.
  A failure in a handler. A `try` inside a callback, and around one.
- **The parser:** each targeted error of 2.2, with its span.
- **Messages:** every case that expects an error also expects its message. Each
  template of the catalog has a case, and each parameter kind has a case at the
  cut: 64 and 65 code points, and a code point above U+FFFF.
- **Trace:** spans, names and indexes, for a `const`-bound lambda, for each of
  the ten entries that take a callback (`06:2.7`), and for the two nested.
- **Rendering:** a new kind of case, with no program: a diagnostic as JSON and a
  source text, and the one-line, detail and JSON texts expected.
- **`ConformAll`:** a conformance case of the second kind (`05:A.11`) with a
  `max`, expecting the list of paths and messages, and that its first entry is
  the path the three first-failure tests give.

## 5. Alternatives rejected

| Alternative | Why not |
| --- | --- |
| Several `catch` clauses, or a clause with a test: `catch (err is T)` | `match err.code` in the handler says it, with the cases checked |
| An error as an eighth kind of value | Every walk over a value in three implementations would gain a case. A map needs none |
| The trace as a field of `err` | A program would branch on positions. It would be built, and charged, for every caught error |
| A list of calls kept during the run, so that the trace is always at hand | It costs every call of every run that succeeds |
| Messages left to each implementation, with `err.message` implementation-defined | Two runs of one program would differ in a value and in a count (L6) |
| A fixed charge for a message, so that wording could change | A program can still compare the text, so the wording is fixed anyway |
| The path inside the message | It is already a field. A cut would remove its end, which is the part that matters |
| `as` that reports every failure | Its cost is exact because it stops (`07:2.3`). The host's function has no cost to keep exact |
| `help` specified | One search for the nearest name, three times, kept in step by the suite. Nothing a program sees depends on it |

## 6. Prior art

| Language or tool | What it does | What I take from it |
| --- | --- | --- |
| Risor v2 | `try { ... } catch (e) { ... }` is an expression with a value (`risor:docs/risor-vs-typescript-syntax.md:115-127`) (R). It also has `finally` and `throw` | The form. Not `finally` |
| Kotlin | `try` is an expression. Its value comes from the `try` block or the `catch` block, and `finally` does not change it (checked) | A `try` with a value is ordinary |
| JavaScript | The `catch` binding and its parentheses may be left out. The binding may be a destructuring pattern (checked, MDN) | `catch { ... }`. The pattern is later |
| rustc | Its JSON diagnostics give lines and columns from 1, the end exclusive, columns counted in Unicode scalar values (checked). Its text form starts `error[E0308]:` (from memory) | The one-line form. Positions already agree with `05:A.6` |
| TypeScript | Every compiler message is a row of one data file, with numbered holes (from memory) | The catalog. Named holes, because three implementations fill them |
| Zod | A failed parse reports every issue, each with a code and a path (checked) | 2.11 |
| Swift, Zig | `try?` gives null on failure; `a catch b` is an operator (both from memory) | A short form exists elsewhere. Not in 3.0 (section 7) |
| Python, against JavaScript, Go and Java | Python prints the outermost call first. The others print the innermost first (from memory) | The data runs from the outside in. The detail form prints the innermost first, under the source line |

## 7. Left out

| Left out | Why | When |
| --- | --- | --- |
| Raising a caught error again unchanged | `fail(err)` would need a rule that its argument is a `catch` name, or a check that a map was not forged with a limit's code | Later. It is an addition |
| A pattern in `catch ( )` | A name and `const {code} = err;` say the same | Later. It is an addition |
| Data on a host function's error | It changes the call contract of `05:A.4` in three bindings | Later. The first addition I would make |
| The parameters of a message as a field of the diagnostic | A host that translates needs them. No program does | Later |
| A short fallback, such as `e catch d` | `(try { int(s) } catch { 0 })` is 14 characters longer than `try(int(s), 0)`. That is the price of the owner's decision, and one way to write a thing is worth it | Not planned |
| Every failure, inside the language | 2.11 | Not planned |

## 8. Open questions, and where the documents disagree

1. **`missing-variable` passes through `catch`** (10-3). It is the one new
   exception, and the one I am least sure of.
2. **`02:2.12` and `04:A.1` disagree on a crash.** `02` says `try` catches every
   run error but a limit and a cancellation. `04` also excludes a host function
   that crashes and a failure inside the implementation. This plan follows `04`.
   `05:A.6` gives `function-panic` no class.
3. **`04:A.2` and `05:A.3` disagree on a host number that cannot enter the
   language.** `04` calls it an `environment` error and `05` gives it the code
   `number-range`. With `catch` deciding by code, the code must be settled.
   Either way this plan catches it.
4. **The class of a run error needs the checker's sites** (`03:2.14`). An
   evaluator built before its checker cannot report `dynamic` against
   `environment`, and the suite compares the class (`05:A.6`). That is not
   caused by this plan, but 2.3 and 2.4 rest on it.
5. **`throw` is not a reserved word or a targeted error today.** `02:2.12` says
   only that there is none, and `02:3.2` has no row. The brief says it "stays"
   one. This plan adds it.
6. **Sites in the body.** The brief says `value` sites are dropped. `03:2.14`
   drops sites of every class, and this plan keeps that.
7. **The cost of the trace in the TypeScript implementation** is not measured.
   If a `catch` at each call is not free there, the alternative of 10-5 stands.
8. **Is 64 code points the right cut** for a `type` parameter? A record type is
   often longer. The whole type is in `expected`, which is not specified.
