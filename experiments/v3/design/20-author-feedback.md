# Risor v3 design: feedback to authors

Status: built, D56 in `00-overview.md`. The owner settled its direction on
2026-11-14, from the evidence of issues #640 and #641, and trimmed it on
2026-11-19: the brief rendering (20-8) and layouts for `time.format` (20-10)
are dropped (section 6). Decisions are numbered `20-1` to `20-11`. Each is
marked **settled** where the owner chose it, and **recommended** where this
plan chose a detail the owner has not seen. Built in all three
implementations, in track B: 20-1 and 20-2 (`15` R212, R215, R216, R260), and
the settled part of 20-9, the inner message with the inner diagnostic kept as
the cause (`15` R217). The rest is built in all three implementations, as
package F19, in track F: `spec/packages/f19-author-feedback.md`, with the
recommended details of 20-4, 20-5, 20-9 and 20-11 as recommended (`15` R351
to R359). Where the chapter differs from this plan, the chapter is right:
the row `indexOf` of 3.3 is left out (R355), and E28 also takes a `Branch`
(R356). Counts marked *ran* were taken from the suite files at `main`
`aeaaf1f` with a short script.

**Where it lands.** 20-1 and 20-2 (absence under `??`) and 20-9 (a nested
Risor error's message) change outcomes of Core, so they are Core amendments,
in track B of `14` 5. The rest is package F19, track F. `00` section 4,
"Where the plans meet", fixes the numbers and the order.

**How to read this.** Section 3 is the proposal, written as rules so that it can
move into the specification. A paragraph headed *Why* is not a rule. **(R)** is
read in the document at the place cited, **(I)** is inferred, and "(from
memory)" is not checked. `01:` to `05:` are the Core chapters in `spec/core/`,
`F6` and `F8` are `spec/packages/f06-groups.md` and
`spec/packages/f08-errors.md`, and `O:` is `00-overview.md`.

## 1. Summary of the decisions

| # | Decision | Recommendation | Strongest alternative |
| --- | --- | --- | --- |
| 20-1 | Absence under `??` | **Settled.** In a chain `e1 ?? e2 ?? ... ?? d`, when the last step of an operand other than the last is a plain field read, an absent key at that step gives null. The checker types that read as `?.` types it, except that a key a closed record cannot have stays `no-field` | Change B2 everywhere, so a plain `.f` gives null for an absent key |
| 20-2 | The reach of 20-1 | **Settled with 20-1 (2026-11-19).** Every operand of a `??` chain but the last. A `.f` step, and `["f"]` with a `String` node only. Not a computed key. Not `\|\|` | Include `m[k] ?? d` with any string key |
| 20-3 | `maybe-absent` names its fixes | **Settled** (P1 of #640). The templates name `?.`, `??`, a test, and a pattern default | Keep the message; rely on 20-1 alone |
| 20-4 | Where a hint lives | **Recommended.** In the message, through templates that carry a fixed fix taken from a table | A new `hint` field, written by every rendering |
| 20-5 | The `wrong` lists | **Settled** that they become normative. **Recommended:** the lookup rules of 3.3 | Edit distance over the names in scope |
| 20-6 | Foreign names | **Settled.** A suite table of JavaScript and v2 habits with their Risor form | Fold them into the `wrong` lists |
| 20-7 | `await`, `async`, and `;` between expressions | **Settled.** Two targeted parse codes. `await` and `async` are not reserved | Reserve `await` and `async` |
| 20-8 | A rendering for a model | **Dropped, 2026-11-19** (section 6) | — |
| 20-9 | A host error that is a Risor diagnostic | **Settled:** the `function-error` message is the inner message, and the inner diagnostic is the `cause`. **Recommended:** its `data` is carried, and JSON writes `cause` | Leave the message to the binding, as today |
| 20-10 | `time.format` with a layout | **Dropped, 2026-11-19** (section 6) | — |
| 20-11 | No suggestion by search | **Recommended.** Hints come from tables only | "Did you mean" by edit distance |

## 2. The problem

### 2.1 What #640 measured

The experiment of #640 wrote six agent tools in Risor (ran there, Go 1.26.0,
`main` `16cd6f8d`). Four findings concern what an author is told.

1. **Optional fields.** "Every tool with an optional parameter failed to load on
   the first try." The failing text was `args.limit ?? 5`, which is correct
   TypeScript (C6). The message, `field "limit" may be absent from {...}`, names
   neither `args?.limit` nor `const {limit = 5} = args;`.
2. **JavaScript habits.** F8's targeted parse codes name the Risor form for
   `let`, `for`, `===`, `return` and `function`. The check errors for the next
   habits do not. The issue lists eleven: `xs.length`, `.includes`,
   `Math.round`, `JSON.stringify`, `Object.keys`, `parseInt`, `.toFixed`,
   `console.log`, `xs.push(1); xs`, `await f()`, `args.coupon ?? "none"`.
3. **Error text for a model.** A nested failure repeats its prefix:
   `error[function-error] 5:38-5:66: error[function-error] 8:11-8:32: unknown
   city "Atlantis"`. 20-9 answers it. The report also found that
   `RenderLine` writes spans, which a model cannot use; the rendering for that
   (20-8) is dropped.
4. **`time.format`.** `time.format(t, "2006-01-02")` is `arity`. The example is
   Go's layout, which P3 rules out as a definition (O: P3). The layout
   (20-10) is dropped.

### 2.2 Why it matters more than it did

Tool code and host scripts will often be written by a model, in a loop of write,
check and fix. The quality of the first message decides how many turns the loop
takes. A message that names the Risor form ends the loop in one turn. A message
that only states the fault makes the model guess (I).

### 2.3 What is already there

| Piece | State (R) |
| --- | --- |
| `wrong` lists in the library tables | `suite/library.json`: 53 of 53 entries, 137 names. `library-groups.json`: 8 of 10, 29 names. `library-math.json`: 11 of 11, 17 names. `library-queries.json`: 7 of 7, 26 names. In all, 79 entries and 209 names (ran). `04:` 1.3 marks the field "not normative" |
| A use for them | `04:` 2.3: "A diagnostic may give the right name in its `help`, which is not specified and which no case compares." No message uses them |
| The rule against search | `03:` 3.8.3 item 3 and F8 3.3 item 4: no suggestion found by search. F8 3.3 item 4: "A fixed fix written in a template is not a suggestion found by search" |
| `help` and `fix` | Unspecified (`10-6`), and never written by a rendering (F8 5.1 rule 3) |
| Message stability | A `parse` or `check` message may change in a minor version (`03:` 3.8.4, F8 3.6). A `run` message may not |

So most of the data exists. What is missing is the rule that turns it into a
message every implementation gives.

## 3. The proposal

### 3.1 Absence under `??` (20-1, 20-2)

**The rule.** These rows replace the row of `a ?? b` in `01:` 6.6 and the row in
`02:` 2.2.

| Form | Value |
| --- | --- |
| `a ?? b` | If `a` is not null, `a`, and `b` is not evaluated. Otherwise `b` |
| `a ?? b`, where `a` is a **coalesced read** | As above, except that when the key of the read's last step is absent from its receiver, `a` is null |

`??` is one flat chain, `Coal = Add "??" Add { "??" Add }` (`01:` 3.1). A
**coalesced read** is an operand of a chain, other than its last, whose node is
one of these, with or without an `OptChain` around it (`01:` 4.1). So in
`a.f ?? b.g ?? c` both `a.f` and `b.g` are coalesced reads, and `c` is not.
Parentheses have no node (`01:` 4.2), so `(a.f) ?? d` is one too.

| Node | Condition |
| --- | --- |
| `Member` | `optional` is false |
| `Index` | `optional` is false, and the index is a `String` node |

1. Only the last step is affected. `a.b.c ?? d` with no key `b` fails with
   `missing-field` at `a.b`, as it does today.
2. Only absence is affected. A null receiver of the last step still fails with
   `null-operand`, and a receiver of another kind with `wrong-kind` (`01:` 7.1
   items 1 and 2). This is what TypeScript does with `a.b.c` when `a.b` is null
   (from memory).
3. The rule depends on the node only, never on a type. So a checked and an
   unchecked run give the same value (R55).
4. It costs nothing more: the read charges what a read charges.
5. The suite pins these edges: a receiver whose type is a union, read by
   flat(U) as every union read is (R196); `a?.b.c ?? d` with no key `b`,
   which is `null-operand` at `.c` by item 2; and `xs[*].f ?? d`, whose
   operand is an `Each` node (F7), not a coalesced read.

**The checker.** A row joins `05:` 9.2.

| Receiver | Step | A coalesced read |
| --- | --- | --- |
| A record | Field `f`, or `["f"]` with a `String` node: f optional, or unlisted with a rest that is not `never` or `any` | As the optional column: the type with null added. No `maybe-absent` |
| | The same: f unlisted, rest `any` | `any`, and no site |
| | The same: f unlisted, rest `never` | `no-field`; `never`. A key that no type allows is a typo, not an absence |

Every other receiver and step reads by the plain column.

The third row keeps a typo a typo. `order.dicsount ?? 0` on a closed record
still fails the check with `no-field`, and does not give `0` silently.

**Why not change B2 everywhere.** B2 (O: B2) makes `.` fail on an absent key.
That failure carries load. A run does not trust a declared type (`03:` 3.4.8
item 1). Where host data breaks its type, the run must fail with the code of
what it found, and "never gives a wrong value" (`03:` 3.4.8 item 2). If `.` gave
null for any absent key, a field the type calls required but the data lacks
would read as null and flow on as a value. The narrow rule confines that to
one place, where the author has said that absence is expected and given the
value to use.

**An exception to 3.4.8 item 2.** The narrow rule does not keep 3.4.8 for a
field the type calls required. On a host record declared `{id: int}` that
lacks `id`, `x.id ?? 0` gives `0` and no `missing-field`. The rule cannot be
limited to optional and unlisted fields: it would then depend on a type, and
rule 3 above, with R55, forbids that. So 3.4.8 item 2 gains an exception:
"except a coalesced read under `??` (`01:` 6.6), which gives the
default for an absent key whatever the declared type says." The value given
is the author's own default, not a value made from data. The owner settled
absence under `??` (D56); this exception is what that ruling costs, and it is
named so the owner can confirm it (20-1).

**20-2, the reach.** A computed key, `m[k] ?? d`, is left out. Its type rule
(`05:` 9.2, second row) joins every field type, and the key is not known, so the
checker could not tell an absence from a typo. `m?.[k] ?? d` stays the form.
`||` is left out because `x.f || d` also replaces `0`, `""` and `false`, which
is JavaScript's own trap (O: D30). *Why the alternative was not taken:* it is
the same rule for one more node, and it could be added in a minor version
without a break, since it only turns failures into values.

### 3.2 Messages for `maybe-absent` (20-3)

The two templates of `maybe-absent` change text, and one is added. The code and
its parameters do not change.

| Template | Text | Chosen when |
| --- | --- | --- |
| `field` | `field {key} may be absent from {type}: read it with ?., add ?? and a default, or test for it first` | A plain read, as today |
| `index` | `the key may be absent from {type}: read it with ?.[ ], and add ?? and a default` | An index read of a record, as today |
| `pattern` | `field {key} may be absent from {type}: give it a default in the pattern, as {key} = ...` | A field of a map pattern with no default (`05:` 12.2) |

`{key}` is a `string` (`03:` 3.8.3), so it prints with its quotes: `as "limit" =
...`. *Why the quotes are kept:* a key need not be a `Word`, and a second kind
of parameter for one template is not worth it.

### 3.3 Hints from tables (20-4, 20-5, 20-6, 20-11)

**20-4. A hint is part of the message.** A hint is a fixed clause that a
template adds after `; `. It is chosen by a lookup in a table of the suite,
never by search. So `03:` 3.8.3 item 3 holds as amended in section 5: a table
lookup is not a search.

*Why in the message.* Every rendering already writes the message: one line,
detail and JSON. A model sees the hint in whatever form the host sends. A new
field would need a change to F8 5.1 rule 3, to all three renderings, to the
case format and to every host that reads diagnostics. The message may
change in a minor version (`03:` 3.8.4 item 2), so a better hint later is not a
break. *What the alternative gives:* a tool could show the hint apart from the
fault. That is a presentation choice. A host that wants it can split at the
template's `; `.

**A new kind of parameter** joins F8 3.2.

| Kind | Rendering |
| --- | --- |
| `fix` | A clause from a table of this section, written as it stands. It is never cut: every clause in the tables is at most 64 code points |

**20-5. The `wrong` lists are normative.** The field `wrong` of each record of
every library table becomes normative. A name in it is a name an author may
write for that entry. The rule of `04:` 1.2 that `wrong` "may change" stands: a
list may change in a minor version, and so may the messages built from it.

**20-6. The foreign names.** A new suite file, `suite/foreign-names.json`, holds
the habits that no entry stands for. Each row has a written form, a place, and a
fix. The first rows:

| Written | Place | Fix |
| --- | --- | --- |
| `Math` | name | `Risor has no Math object: write round(...), floor(...) and the like` |
| `JSON` | name | `Risor has no JSON object` |
| `Object` | name | `Risor has no Object: use the map methods keys, values and entries` |
| `Object.assign` | name and field | `use spread: {...a, ...b}` |
| `Array.isArray` | name and field | `use type(x) == "list"` |
| `Date`, `Date.now` | name; name and field | `Risor has no clock: the host passes the time as a variable` |
| `console` | name | `Risor has no console: a program's only output is its value` |
| `print`, `println`, `printf` | function | `Risor has no output: a program's only output is its value` |
| `undefined` | name | `use null` |
| `await` | function | `remove await: a host call gives its value directly` |
| `includes`, `contains` | method of a list or a string | `use the operator in: x in xs` |
| `has` | method of a list or a map | `use the operator in: x in xs, or key in m` |
| `push` | method of a list | `values never change: build a new list with [...xs, x]` |
| `unshift` | method of a list | `values never change: build a new list with [x, ...xs]` |
| `pop`, `shift` | method of a list | `values never change: use xs.last() and xs.slice(...)` |
| `concat` | method of a list | `use spread: [...xs, ...ys]` |
| `indexOf` | method of a list or a string | `use in, or find(...)` |

A row of place "name" applies to an `undefined-name`; "name and field" to an
`undefined-name` followed by a `.f` step, matched as `N.f`; "function" to an
`undefined-function`; "method" to a `no-method` on a receiver of that kind.

**The lookup.** A diagnostic of one of these four kinds takes its hint by the
first row of this table that gives one. When none does, it keeps its present
template. For an `undefined-name` N followed by a `.f` step, row 1 looks for
the form `N.f` (place "name and field") first, then row 2 runs, and only then
does row 1 look for N alone (place "name"). So `Math.round(x)` gives `use
round(...)`, and `Math.foo` gives the `Math` row's fix.

| # | Diagnostic | Looks for | Gives |
| --- | --- | --- | --- |
| 1 | Any of the four | A row of `foreign-names.json` with that written form and place | Its fix |
| 2 | `undefined-name` N, followed by a `.f` step | `N.f` in a `wrong` list | The entry's form |
| 3 | `undefined-function` n | n in a `wrong` list; else n as the name of a method entry | The entry's form |
| 4 | `no-method` n on a list, a string, a map, an int or a float | n in the `wrong` list of a method of that receiver kind; else in the `wrong` list of a function; else n as the name of a function entry | The entry's form |
| 5 | `kind`, template `field-read`, field n | n in the `wrong` list of a function | The entry's form |

1. **The entry's form** is the clause `use f(...)` for a function entry `f`, and
   `use x.m(...)` for a method entry `m`. A namespaced function is written with
   its namespace: `use json.stringify(...)`.
2. **Several entries.** When more than one entry matches, each gives its form.
   The forms are sorted in code point order, the first three are kept, and they
   are joined with ` or `. So `Number(s)` gives `use float(...) or use
   int(...)`.
3. **Only what the environment has.** An entry the environment does not offer
   never matches: a group not enabled, or an entry removed (`04:` 2.3). So
   `JSON.stringify` gives `use json.stringify(...)` with the `json` group, and
   the `JSON` row's fix without it.
4. **Never a host's name.** A `no-method` on an opaque receiver takes no hint.
   An `undefined-name` or `undefined-function` takes a hint only because the
   name is not declared, so a host that declares `Math` or `print` sees none.
5. The lookup is by exact name. Case matters.

**Two hints that are not in the table.** Two more habits get a fixed clause,
and neither is a lookup of a name, so each is a template of the plan that
brings the syntax it names (my reading; the adjustments of 2026-11-19 listed
them with the foreign names):

| Habit | Diagnostic | Where its template is written |
| --- | --- | --- |
| `interface Order { ... }` | The `parse` code `type-declaration` (F8 2.7 E17) | `17` 3.1: once type lines exist, its text names `type Order = {...}` |
| `(xs, f) => f(xs)`, a parameter called with no `Fn` type | `call-of-parameter` | `18` 7.2: a template that names an `Fn` annotation |

**The templates** that carry a hint, in F8 3.5:

| Code | Template | Text |
| --- | --- | --- |
| `undefined-name` | `hint` | `{name} is not defined; {fix}` |
| `undefined-function` | `hint` | `no function is named {name}; {fix}` |
| `no-method` | `hint` | `a value of type {type} has no method {name}; {fix}` |
| `kind` | `field-read-hint` | `a value of type {type} has no fields to read; {fix}` |

So, by these rules (I, not run):

| Written | Message |
| --- | --- |
| `xs.length` | `a value of type int[] has no fields to read; use len(...)` |
| `s.includes("x")` | `a value of type string has no method includes; use the operator in: x in xs` |
| `Math.round(x)` | `Math is not defined; use round(...)` |
| `parseInt(s)` | `no function is named parseInt; use int(...)` |
| `x.toFixed(2)` | `a value of type float has no method toFixed; use fixed(...)` |
| `keys(m)` | `no function is named keys; use x.keys(...)` |
| `console.log(x)` | `console is not defined; Risor has no console: a program's only output is its value` |

**20-11. No search.** Edit distance ("did you mean `filter`" for `fitler`) is
left out. *Why:* it needs a fixed distance, a fixed tie rule and a fixed set of
candidates in scope, written three times, for a typo a model rarely makes. The
tables cover the habits, which are the common case. *The alternative* stays
possible: it is a new template in a minor version.

### 3.4 Targeted parse codes (20-7)

`xs.push(1); xs` and `await f()` fail in the parser, before the checker runs.
The program is `{ Const } Expr [ ";" ]` (`01:` 3.1), so the `;` after
`xs.push(1)` ends the program and `xs` is an unexpected token. `await` is not a
reserved word (`01:` 2.4), so `await` is a name, a complete operand, and `f`
after it is two operands side by side, which F8 2.10 leaves as
`unexpected-token` (R). Both need rows of F8 2.7, not check hints.

| # | Condition | Code and template |
| --- | --- | --- |
| E26 | The token before t is the operand word `await` or `async` | `async-await`, `await` or `async`, at the span of 2.8 |
| E27 | The token before t is the `;` that ends the `Expr` of a `Program`, and t is not the end of input | `statement-sequence`, `program`, at the span of 2.8 |
| E28 | t is `;`, and the parser reads a `Block` whose `Expr` has been read | `statement-sequence`, `block` |

| Row | Core's span | The span |
| --- | --- | --- |
| E26 | The token after the word | The word `await` or `async` |
| E27 | The token t | The `;` |

| Code | Template | Text |
| --- | --- | --- |
| `async-await` | `await` | `there is no await: a host call gives its value directly; remove await` |
| | `async` | `there is no async: a lambda gives its value directly; remove async` |
| `statement-sequence` | `program` | `a program is const lines and then one expression; bind earlier values with const, and remember that values never change` |
| | `block` | `a block is const lines and then one expression, with no ; after it` |

1. `await` and `async` stay names. A host may declare a variable `await`, and
   `await.x` then reads it. *Why not reserve them:* a reserved word cannot be
   unreserved, and C7 makes a break cheap but not free. E26 catches the text
   without taking the name.
2. `await(f())` is a call, which the checker reports as `undefined-function`
   with the hint of the `await` row (3.3).
3. E26 is tried with E16 to E18, which have the same shape. E27 and E28 come
   after E25.
4. A lambda body is not a block (`01:` 3.1), so `x => { f(); g }` stays E23.

### 3.5 Counts

What F19 adds, counted from the suite files at `aeaaf1f` (ran):

| Catalog | Codes | Templates |
| --- | --- | --- |
| `parse` (F8 2.11, 3.1) | 58 → 60: the 14 of Core and 46 targeted | 97 → 101 |
| `check` (F8 3.1) | 29, unchanged by F19 | 71 → 76: four `hint` templates and `maybe-absent` `pattern` |
| `run` (`suite/messages.json`) | 19, unchanged | 64, unchanged |

**A discrepancy found while counting.** F8 3.1 item 2 says `messages-parse.json`
has 96 templates. The file has 97 (ran). The file is right: the specification's
number is fixed in the Core amendments (track B), not here. Other plans add to
the `check` catalog too (`recursion-needs-types`, `bad-export`); `00` section
4 lists every addition.

### 3.6 A host error that is a Risor diagnostic (20-9)

The row "Returns an error" of `03:` 3.5.3 gains a case.

| The function | The run sees |
| --- | --- |
| Returns an error that is a Risor diagnostic: a failure of a nested run, or one the binding builds | `function-error`, at the span of the call. **The message is the inner diagnostic's message.** The inner diagnostic is the `cause`. When the inner diagnostic has `data`, that is the `data` of this one |
| Returns any other error | As today: the message is the text of the host's error, as the binding gives it |

1. The code stays `function-error`, and `catch` catches it as before (`10-3`).
   Only the message, the `cause` and the `data` change.
2. Unwrapping goes one level, so a chain of nested runs keeps the innermost
   message: each level copies the message of the level below.
3. How a binding tells that an error is a Risor diagnostic is the binding's
   matter (`03:` 3.5.3 item 3): in Go an `errors.As` to its error type, in
   TypeScript `instanceof`, in Rust a downcast (I).
4. **Recommended:** the JSON form (F8 5.5) gains a key `cause`, present when the
   cause is a Risor diagnostic, holding that diagnostic's JSON object. A cause
   is written to a depth of 8, and deeper causes are left out. F8 5.1 rule 3 is
   amended for this one key.

*Why carry `data`.* A nested tool that ends with `fail(message, data)` gives its
caller's `catch` the same record it would give at the top. Today `03:` 3.5.3
item 2 drops it. *The alternative,* leaving `data` out, keeps that item as it is
and loses the structured part of the failure.

So the example of 2.1 becomes `error[function-error] 5:38-5:66: unknown city
"Atlantis"` in one line.

## 4. What it costs each implementation (I)

| Part | Go | TypeScript | Rust |
| --- | --- | --- | --- |
| 20-1 in the evaluator | A flag on the read node when it is an operand of a `??` chain other than the last; about 20 lines | The same | The same |
| 20-1 in the checker | One row of 9.2; about 20 lines | Checker not yet built: written once | The same |
| 20-3 to 20-6 | Generated from the tables: the lookup, about 120 lines; the templates are data | The same, when its checker exists | The same |
| 20-7 | Three rows in the targeted parse mapping; about 40 lines | The same | The same |
| 20-9 | Unwrapping in the host-call path; about 30 lines | The same | The same |

The largest cost is the suite: cases for every template and every foreign row.

## 5. Changes to the other documents

| Document | Section | Change |
| --- | --- | --- |
| `spec/core/01-language.md` | 6.6 | The `??` rows of 3.1, and the term coalesced read |
| | 7.1 | Note on the `missing-field` column: not for the last step of a coalesced read |
| | 14.5, the `missing-field` rows | The same exception |
| `spec/core/02-evaluation.md` | 2.2 | The row of `a ?? b` |
| `spec/core/05-types.md` | 9.2 | The column for a coalesced read (3.1) |
| | 16.1, row `maybe-absent` | Not reported for a coalesced read |
| | 21 (worked examples) | Cases for 20-1, with and without a rest |
| `spec/core/03-host.md` | 3.4.8 item 2 | The exception of 3.1: a coalesced read gives the default for an absent key, whatever the declared type says |
| | 3.5.3 | The case of 3.6; item 2 amended for `data` |
| | 3.8.3 item 3 | "A fix taken by exact name from a table of the specification is not a suggestion found by search" |
| `spec/core/04-library.md` | 1.3, row `wrong` | Normative: "Names people try. The hints of F8 3.7 use them" |
| | 2.3 | The last paragraph: a diagnostic gives the right name in its message, by F8 |
| `spec/packages/f08-errors.md` | 1 (scope) | The hints |
| | 2.7 | Rows E26 to E28 |
| | 2.8 | Spans of E26 and E27 |
| | 2.11 | 58 → 60 codes, 44 → 46 targeted; rows for `async-await` and `statement-sequence` |
| | 3.1 item 2 | Parse templates 97 → 101, check templates 71 → 76 (3.5), after track B corrects 96 to 97 |
| | 3.2 | The kind `fix` |
| | 3.3 item 4 | A table lookup is not a search |
| | 3.4, 3.5 | The templates of 3.2 to 3.4 |
| | 3.7 (new) | The lookup of 3.3 and the file `foreign-names.json` |
| | 5.1 rule 3 | `cause` written in JSON |
| | 5.5 | The key `cause` |
| `suite/` | `messages-parse.json`, `messages-check.json` | The templates and cases |
| | `foreign-names.json` (new) | The table of 3.3 |
| | `README.md` | The new file |
| `00-overview.md` | B2 | "B2 holds everywhere but the last step of a coalesced read (`20-1`)" |
| | 4 | Row D56 |
| `06-stdlib.md` | "Deliberately absent", row `includes` | Points to the foreign-name hint |
| `03-types.md`, `10-errors.md` | Where they describe `??` and the renderings | Cite this plan |

The failure tables of `01:` 14 count codes, not templates, so no count there
changes (R).

## 6. Dropped on 2026-11-19, and why

| Part | What it was | Why it is dropped |
| --- | --- | --- |
| 20-8 | A fourth rendering, brief: `error[code]: message at path`, with no span and no trace, for a model | The owner dropped it. The reason recorded here is inference: the one-line and JSON forms already carry the message and the path, and a host that shows a model the error can leave the span out itself. A rendering can be added later without a break |
| 20-10 | An optional layout for `time.format`, in tokens taken from Unicode LDML, UTC only | The owner dropped it from 3.0: a layout belongs to a package after 3.0, not to feedback. Its token set was never checked against UTS #35, and the default RFC 3339 form already serves hosts. The alternatives it weighed, Go's reference layout and `strftime`, stay rejected for the reasons it gave: one is defined by a Go library (O: P3), and the other's letters differ by platform (from memory) |

## 7. Alternatives rejected

| Alternative | Why not |
| --- | --- |
| A plain `.f` gives null for any absent key (TypeScript's rule) | Breaks `03:` 3.4.8 item 2: a value that breaks its type would read as null (3.1) |
| Reserve `await` and `async` | A reserved word cannot be freed, and E26 catches the text without it |
| A `hint` field in every rendering | Three renderings, the case format and every host change for what a template does (3.3) |
| Suggestions by edit distance | Three implementations of a distance and a tie rule, for typos a model rarely makes (20-11) |

## 8. What I did not verify

1. Every message example in 3.3 is worked by hand from the rules, not run.
2. That TypeScript fails on `a.b.c` with `a.b` null is from memory.
3. Whether the case format of `03:` 3.10 can declare a host function that
   returns a Risor diagnostic, which 20-9 needs for a suite case. If it cannot,
   the case format needs a host function of that kind (I).
4. The line counts of section 4 are estimates.
5. The parse template count: the spec says 96 and the file has 97 (3.5). Track
   B fixes the spec.
6. Whether any existing suite case pins a `missing-field` at the last step of a
   `??` operand. Such a case would change outcome under 20-1 and must be listed
   in F8 9, or in the chapter that takes 20-1 (I).
