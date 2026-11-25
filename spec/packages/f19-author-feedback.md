# F19. Feedback to authors

This chapter is normative. It specifies follow-on package F19
(`experiments/v3/design/20-author-feedback.md`, and
`experiments/v3/design/14-implementation-plan.md` 5.3, track F item 1). It
makes the first message an author gets name the Risor form of what they
wrote: the texts of `maybe-absent` name their fixes (2); four check messages
take a hint from the tables of the suite, the `wrong` lists of the library
and a new table of foreign names (3); two targeted `parse` codes catch
`await` and `;` between expressions (4); and a host error that is a Risor
diagnostic carries its `data`, and its JSON form writes the inner diagnostic
(5). It adds two `parse` codes, the kind of parameter `fix`, the files
`suite/messages-feedback.json` and `suite/foreign-names.json`, the directory
`suite/feedback/`, and the key `data` of a stub's diagnostic object. It adds
no node kind, no value, no type, no run code, no check code, no reserved
word, no library entry, no cost row and no limit.

Two parts of plan `20` are Core, not this package, and are already in the
Core chapters: absence under `??` (`20-1`, `20-2`: `01-language.md` 6.6,
`05-types.md` 9.2, `03-host.md` 3.4.8 item 2), and the message of a host
error that is a Risor diagnostic (`20-9`: `03-host.md` 3.5.3 items 5 to 7).

The package changes messages and nothing a program can see, except the
`data` of section 5. A `parse` or a `check` message may change in a minor
version (`03-host.md` 3.8.4, F8 3.6). Section 8 lists the earlier cases whose
outcome changes.

Other chapters: `01-language.md` (grammar, nodes, the error value),
`03-host.md` (diagnostics, the catalog, stubs, the order of packages),
`04-library.md` (the record format, removed entries), `05-types.md` (the
checker, the static rules), F6 (`spec/packages/f06-groups.md`), F7
(`spec/packages/f07-queries.md`), F8 (`spec/packages/f08-errors.md`), F13
(`spec/packages/f13-math.md`) and F17 (`spec/packages/f17-recursion.md`).

## 1. Terms and scope

### 1.1 Terms

| Term | Meaning |
| --- | --- |
| Fix | A clause that a template writes after `; `, taken from a table of this chapter |
| Hint | A diagnostic whose template carries a fix (3.5) |
| Library table | A file of library records: `suite/library.json`, and the files of F6, F7, F13 and F17 |
| Offered entry | A library entry the environment has: in a group that is on, and not removed (`03-host.md` 3.3.1 item 6) |
| Foreign row | A row of `suite/foreign-names.json` (3.3) |
| Receiver kind | The kind of a method's receiver: `string`, `list` or `map` (`04-library.md` 2.1). For a diagnostic, the one kind of the receiver's type at the diagnostic, null left out |

### 1.2 What this package changes in earlier rules

| Earlier rule | With this package |
| --- | --- |
| `03-host.md` 3.5.3 item 2 | A host error that is a Risor diagnostic carries the inner diagnostic's `data` (5.1) |
| `03-host.md` 3.8.3 item 3, F8 3.3 item 4 | A fix taken by exact name from a table of the specification is not a suggestion found by search (3.4) |
| `03-host.md` 3.10.4 item 5 | A stub's diagnostic object may have the key `data` (5.3) |
| `03-host.md` 3.11 item 2 | F19 is offered after F17 and before F12 (1.4) |
| `01-language.md` 11.2, the row `data` | For `function-error`, the data of 5.1 |
| `04-library.md` 1.3, the field `wrong` | Normative (3.2) |
| `04-library.md` 2.3, the last paragraph | A diagnostic names the right entry in its message, by 3.4 |
| `05-types.md` 16.1, the rows `undefined-name`, `undefined-function`, `no-method`, `kind`, `maybe-absent` | Their messages, by 2 and 3 |
| F8 2.7, 2.8, 2.11 | Rows E26 to E28, their spans, two more codes (4) |
| F8 3.1 item 2 | This package's catalog file (7) |
| F8 3.2 | The kind `fix` (3.1) |
| F8 3.5, `maybe-absent` | The texts of `field` and `index` (2) |
| F8 5.1 rule 3, 5.5 | The JSON form writes `cause` (5.2) |

No other rule changes. A program's type, sites, value and usage are those it
has without this package, but for the `data` of 5.1.

### 1.3 Version

F19 is part of version `3.0`. It adds no version number. Each entry of
`suite/messages-feedback.json` has `"since": "3.0"`.

### 1.4 Packages this one requires

1. Packages are offered in a fixed order, each only with every package before
   it (`03-host.md` 3.11 item 2): F3, F1, F2, F4, F5, F6, F7, F8, F9, F10,
   F11, F13, F14, F16 slice D1, F17, F19, F12.
2. An implementation that offers F19 offers F8, whose templates it adds to,
   and F6, F7, F13 and F17, whose library tables the lookup of 3.4 reads.
3. F19 is offered whole. All three implementations offered it in the change
   that added it, so no implementation offers F12 without it.

## 2. Messages of `maybe-absent`

The texts of the two templates of `maybe-absent` in
`suite/messages-check.json` change, and this package's catalog file adds a
third. The code and its parameters do not change.

| Template | Text | Chosen when |
| --- | --- | --- |
| `field` | `field {key} may be absent from {type}: read it with ?., add ?? and a default, or test for it first` | A field or a `String` key, as before, but not a field of a map pattern |
| `index` | `the key may be absent from {type}: read it with ?.[ ], and add ?? and a default` | An index read of a record, as before |
| `pattern` | `field {key} may be absent from {type}: give the field a default in the pattern, with = and a value` | A field of a map pattern with no default (`05-types.md` 12.2). type is the record bound |

1. The text of `pattern` names no syntax: a key that is not a name needs
   `"key": name = d`, and a name needs `name = d` (`01-language.md` 3.1,
   `Field`), and one text cannot show both (`15-build-rulings.md` R358).
2. The texts of `field` and `index` are Core's templates, so they change in
   Core's catalog file, as F17 changed two (F17 9). A build without this
   package prints them too. Each is true without it.

## 3. Hints from tables

### 3.1 The kind `fix`

This row joins the table of F8 3.2.

| Kind | Rendering |
| --- | --- |
| `fix` | The fix of 3.4, written as it stands. It is never cut |

A clause of `suite/foreign-names.json` is at most 64 code points, and a fix
of 3.4 holds at most three entry forms, so the bound of F8 3.3 item 2 holds:
no message passes 400 code points.

### 3.2 The `wrong` lists

1. The field `wrong` of each record of every library table is normative. A
   name in it is a name an author may write for that entry. The lookup of
   3.4 reads it.
2. A list may change in a minor version, and so may the messages built from
   it (`04-library.md` 1.3; F8 3.6).
3. A name of a `wrong` list is a name as written: a word, or two words joined
   by `.`, as `Math.round`.

### 3.3 Foreign names

`suite/foreign-names.json` holds the habits of other languages that no
entry stands for. It is a JSON array of rows.

| Field | Holds |
| --- | --- |
| `written` | The text written: a name, or `N.f` for the place `name-field` |
| `place` | `name`, `name-field`, `function` or `method` |
| `kinds` | For `method` only: the receiver kinds the row applies to, a list of `string`, `list` and `map` |
| `entries` | When the fix names library entries: those entries, each named as a removal names it (`03-host.md` 3.3.1), as `round` or `list.last`. Absent when the fix names none |
| `fix` | The fix, at most 64 code points |

| Place | Applies to |
| --- | --- |
| `name` | An `undefined-name` of template `name` |
| `name-field` | An `undefined-name` of template `name` whose `Ident` is followed by a step `.f` or `?.f`, a field read or a method call, matched as `N.f` |
| `function` | An `undefined-function` |
| `method` | A `no-method` on a receiver of one of the row's kinds |

The rows:

| Written | Place | Fix |
| --- | --- | --- |
| `Math` | name | `Risor has no Math object: write round(...), floor(...) and so on` |
| `JSON` | name | `Risor has no JSON object` |
| `Object` | name | `Risor has no Object: use the methods keys, values and entries` |
| `Object.assign` | name-field | `use spread: {...a, ...b}` |
| `Array.isArray` | name-field | `use type(x) == "list"` |
| `Date` | name | `Risor has no clock: the host passes the time as a variable` |
| `Date.now` | name-field | `Risor has no clock: the host passes the time as a variable` |
| `console` | name | `Risor has no console: a program's only output is its value` |
| `undefined` | name | `use null` |
| `print`, `println`, `printf` | function | `Risor has no output: a program's only output is its value` |
| `await` | function | `remove await: a host call gives its value directly` |
| `includes`, `contains` | method of a list or a string | `use the operator in: x in xs` |
| `has` | method of a list or a map | `use the operator in: x in xs, or key in m` |
| `push` | method of a list | `values never change: build a new list with [...xs, x]` |
| `unshift` | method of a list | `values never change: build a new list with [x, ...xs]` |
| `pop` | method of a list | `values never change: use xs.last() and xs.slice(...)` |
| `shift` | method of a list | `values never change: use xs.first() and xs.slice(1)` |
| `concat` | method of a list | `use spread: [...xs, ...ys]` |

1. A row of the table that names several texts is one row of the file for
   each: 21 rows in all.
2. **A row names only offered entries.** A row applies only when the
   environment offers every entry of its `entries`, so a fix never names a
   removed entry (`03-host.md` 3.3.1 item 6). The rows with `entries` are
   `Math` (`round`, `floor`), `Object` (`map.keys`, `map.values`,
   `map.entries`), `Array.isArray` (`type`), `pop` (`list.last`, `list.slice`) and `shift`
   (`list.first`, `list.slice`). So with `map.keys` removed, `Object.keys(m)`
   is `Object is not defined`, with no fix.
3. The written text of a row of place `name` or `function` is never the name
   of an entry of a library table, and that of a row of place `method` never
   the name of a method of the row's kinds. So a row never hides an entry.

### 3.4 The lookup

A diagnostic of one of the kinds of this table takes its fix from the first
row that gives one. When none does, it keeps the template it has without
this package.

| # | Diagnostic | Looks for | Gives |
| --- | --- | --- | --- |
| 1 | `undefined-name` of template `name`, `undefined-function`, `no-method` | A foreign row with that written text that applies (3.3) | Its fix |
| 2 | `undefined-name` N of template `name`, followed by a step `.f` or `?.f` | `N.f` in the `wrong` list of an offered entry | The entry forms |
| 3 | `undefined-function` n | n in the `wrong` list of an offered entry; else an offered method named n | The entry forms |
| 4 | `no-method` n on a receiver of kind string, list, map or number | n in the `wrong` list of an offered method of the receiver's kind; else in the `wrong` list of an offered function; else an offered function named n | The entry forms |
| 5 | `kind`, template `field-read`, on a field named n | n in the `wrong` list of an offered function | The entry forms |

1. **The order for a name and a field.** For an `undefined-name` N followed
   by a step `.f`, row 1 looks first for the row `N.f` of place
   `name-field`, then row 2 runs, and only then does row 1 look for N of place
   `name`. So `Math.round(x)` gives `use round(...)`, and `Math.foo` the fix
   of the row `Math`.
2. **The entry form** is the clause `use f(...)` for a function `f`, and
   `use x.m(...)` for a method `m`. A function of a group is written with its
   namespace: `use json.stringify(...)`.
3. **The entry forms.** When more than one entry matches at a row, each gives
   its form. The distinct forms are sorted in code point order, the first
   three are kept, and they are joined with ` or `. So `Number(s)` gives
   `use float(...) or use int(...)`.
4. **Only offered entries.** An entry that is not offered never matches. So
   `JSON.stringify(x)` gives `use json.stringify(...)` with the group `json`
   on, and the fix of the row `JSON` without it.
5. **Never a host's name.** A `no-method` on a receiver of another kind, an
   opaque type among them, takes no hint. An `undefined-name` or an
   `undefined-function` is reported only for a name the environment does not
   declare, so a host that declares `Math` or `print` sees none.
6. The lookup is by exact text. Case matters, and a namespace is part of the
   text: an `undefined-function` named `json.dumps` matches only
   `json.dumps`.
7. **Not a search.** Each step looks a text up in a table, by exact text. That
   is not a suggestion found by search (`03-host.md` 3.8.3 item 3, F8 3.3
   item 4). There is no suggestion by edit distance.
8. The lookup does not depend on the type rules but at rows 4 and 5, whose
   diagnostics are the checker's. So an `undefined-name` or an
   `undefined-function` has the same message in a compile with the static
   rules alone.
9. Building a fix charges nothing: neither phase is a run.

### 3.5 Templates

This package's catalog file adds these templates to codes of
`suite/messages-check.json` (F8 3.1 item 8), each with the parameter `fix` of
kind `fix` (3.1). Each is chosen when the lookup gives a fix.

| Code | Template | Text |
| --- | --- | --- |
| `undefined-name` | `hint` | `{name} is not defined; {fix}` |
| `undefined-function` | `hint` | `no function is named {name}; {fix}` |
| `no-method` | `hint` | `a value of type {type} has no method {name}; {fix}` |
| `kind` | `field-read-hint` | `a value of type {type} has no fields to read; {fix}` |

The other parameters are those of the template it stands for: `name` of
`undefined-name` `name`, of `undefined-function` `default` and of `no-method`
`default`; `type` of `no-method` `default` and of `kind` `field-read`.

### 3.6 Examples

| Written | Message |
| --- | --- |
| `xs.length`, xs a list of int | `a value of type int[] has no fields to read; use len(...)` |
| `s.includes("x")`, s a string | `a value of type string has no method includes; use the operator in: x in xs` |
| `Math.round(x)` | `Math is not defined; use round(...)` |
| `parseInt(s)` | `no function is named parseInt; use int(...)` |
| `x.toFixed(2)`, x a float | `a value of type float has no method toFixed; use fixed(...)` |
| `keys(m)` | `no function is named keys; use x.keys(...)` |
| `console.log(x)` | `console is not defined; Risor has no console: a program's only output is its value` |
| `Number(s)` | `no function is named Number; use float(...) or use int(...)` |
| `xs.at(0)`, xs a list | `a value of type int[] has no method at; use x.first(...) or use x.last(...)` |

## 4. Targeted parse codes

### 4.1 The rows

These rows join the table of F8 2.7.

| # | Condition | Code and template |
| --- | --- | --- |
| E26 | The token before t is the operand word `await` or `async` | `async-await`, `await` or `async`, at the span of 4.2 |
| E27 | The token before t is the `;` that ends the `Expr` of a `Program`, and t is not the end of input | `statement-sequence`, `program`, at the span of 4.2 |
| E28 | t is `;`, and the parser reads a `Block` or a `Branch` whose `Expr` has been read | `statement-sequence`, `block` |

1. `await` and `async` are not reserved words. A host may declare a variable
   `await`, and `await.x` reads it. E26 catches the text without taking the
   name.
2. `await(f())` is a call, which the static rules report as
   `undefined-function`, with the fix of the foreign row `await` (3.3).
3. E26 is tried with E16 to E18, which have the same shape. E27 and E28 are
   tried after E25. Each is tried before D4 (F8 2.1 item 6).
4. A lambda body is neither a `Block` nor a `Branch`: `x => { f(); g }` stays
   E23. A `Branch` is the body of an `if` branch, or the body or the handler
   of a `try`, with or without `const` lines.
5. `async (x) => x` is a call of `async`, then `=>`: no row holds, and it
   stays `unexpected-token`.

### 4.2 Spans

These rows join the table of F8 2.8.

| Row | Core's span | The span |
| --- | --- | --- |
| E26 | The token after the word | The word `await` or `async` |
| E27 | The token t | The `;` |

### 4.3 Codes and templates

With this package the `parse` phase has 62 codes: the 60 of F8 2.11 and these
two.

| Code | Rows |
| --- | --- |
| `async-await` | E26 |
| `statement-sequence` | E27, E28 |

| Code | Template | Text |
| --- | --- | --- |
| `async-await` | `await` | `there is no await: a host call gives its value directly; remove await` |
| | `async` | `there is no async: a lambda gives its value directly; remove async` |
| `statement-sequence` | `program` | `a program is const lines and then one expression; bind earlier values with const, and remember that values never change` |
| | `block` | `a block is const lines and then one expression, with no ; after it` |

Neither code has a parameter.

| Text | Code | Span | Message |
| --- | --- | --- | --- |
| `await f()` | `async-await` | `[1, 1, 1, 6]` | there is no await: a host call gives its value directly; remove await |
| `async x => x` | `async-await` | `[1, 1, 1, 6]` | there is no async: a lambda gives its value directly; remove async |
| `xs.push(1); xs` | `statement-sequence` | `[1, 11, 1, 12]` | a program is const lines and then one expression; bind earlier values with const, and remember that values never change |
| `if (c) { f(); 1 } else { 2 }` | `statement-sequence` | `[1, 13, 1, 14]` | a block is const lines and then one expression, with no ; after it |

## 5. A host error that is a Risor diagnostic

### 5.1 Its data

`03-host.md` 3.5.3 item 2 says that a host function's error carries no
`data`. With this package, one case changes.

| The function | `data` of the `function-error` |
| --- | --- |
| Returns an error that is a Risor diagnostic (`03-host.md` 3.5.3 item 5) | The `data` of the inner diagnostic: for `failed`, the second argument of `fail`, or null; for a `function-error` that carries data by this row, that data; for any other code, null |
| Returns any other error | Null, as before |

1. So a nested run that ends with `fail(message, data)` gives the caller's
   `catch` the same error value it would give at the top, but for its code:
   `try { lookup(x) } catch (e) { e.data }` is the `data` that the nested
   run gave `fail`. The code stays `function-error`.
2. A chain of nested runs keeps the innermost data, as it keeps the innermost
   message (`03-host.md` 3.5.3 item 7).
3. The data is read as a host value (`03-host.md` 3.4), as the binding reads
   any. The class,
   the span and the message of the `function-error` do not change.

### 5.2 `cause` in the JSON form

1. The JSON form of a diagnostic (F8 5.5) gains the key `cause`, present when
   the diagnostic's cause is a Risor diagnostic (`03-host.md` 3.5.3 item 5).
   Its value is the JSON object of the inner diagnostic, by F8 5.5 and this
   rule.
2. A cause is written to a depth of 8: the cause of the diagnostic rendered
   is at depth 1, and the object at depth 8 has no key `cause`.
3. F8 5.1 rule 3 holds for every other key and for the other two forms: the
   one-line and the detail forms do not write a cause.
4. A host error that is not a Risor diagnostic writes no `cause`.

So `weather("Atlantis")`, when `weather` fails with the diagnostic
`{"code": "failed", "message": "unknown city \"Atlantis\""}`, has this JSON
form, on one line:

```
{"cause":{"code":"failed","data":null,"message":"unknown city \"Atlantis\"","phase":"run","trace":[]},"class":"host","code":"function-error","message":"unknown city \"Atlantis\"","phase":"run","span":[1,1,1,20],"trace":[]}
```

### 5.3 A stub's diagnostic object

`03-host.md` 3.10.4 item 5 gives a stub's diagnostic object exactly two keys.
With this package it may have a third, `data`, a JSON value, read as `vars`
are, when its `code` is `failed`.

1. The diagnostic the stub builds has phase `run`, the object's `code` and
   `message`, no span, no class and an empty trace. When `code` is `failed`,
   its `data` is the object's `data`, or null when the object has none.
2. `data` with another code makes the file invalid.
3. In the JSON Schema of `03-host.md` 3.10.11, the object of `error` gains
   the property `"data": {"$ref": "#/$defs/hostValue"}`.

## 6. The cases

| Path | Holds |
| --- | --- |
| `suite/foreign-names.json` | The rows of 3.3 |
| `suite/messages-feedback.json` | The catalog entries of 7, with their cases |
| `suite/feedback/hints.json` | The hints of `undefined-name` and `undefined-function` (3.4 rows 1 to 3): every foreign row of places `name`, `name-field` and `function`, the order of 3.4 item 1, several entries, groups and removed entries, a declared name |
| `suite/feedback/hints-check.json` | The hints of `no-method` and `kind` (3.4 rows 1, 4 and 5): every foreign row of place `method`, receivers of each kind, an opaque receiver |
| `suite/feedback/maybe-absent-check.json` | The three templates of 2 |
| `suite/feedback/parse.json` | Rows E26 to E28, and the texts of 4.1 that stay as they were |
| `suite/feedback/host-errors.json` | The data of 5.1, and the key `cause` of 5.2 |

1. A runner of an implementation that offers F19 runs the files of
   `suite/feedback/` as it runs `suite/core-run/` and `suite/core-check/`
   (`03-host.md` 3.10.8, 3.11 item 7): a file whose name ends `-check.json`
   once, checked, and the others by their cases. It loads
   `suite/messages-feedback.json` and runs its cases checked.
2. A runner of an implementation that does not offer F19 does not run
   `suite/feedback/`, and does not load `suite/messages-feedback.json` or
   `suite/foreign-names.json`.
3. This package adds no case field. Its cases use those of `03-host.md`
   3.10, F6 and F8, and the key of 5.3.

## 7. Messages

`suite/messages-feedback.json` has the format of `03-host.md` 3.8.2. Its
entries for `undefined-name`, `undefined-function`, `no-method`, `kind` and
`maybe-absent` add templates to entries of `suite/messages-check.json`, as F8
3.1 item 8 says, with one more parameter, `fix`. Its entries for
`async-await` and `statement-sequence` are new codes of phase `parse`.

1. **Counts.** With this package the catalog has 62 `parse` codes with 103
   templates: F8's 60 and 99, and the 2 and 4 of 4.3. It has 32 check codes,
   unchanged, with 86 templates: the 81 of F17 13.5 item 6, and the 5 of 2
   and 3.5. The `run` phase is unchanged: 19 codes and 64 templates.
2. Section 3.4 chooses the hint templates. F8 3.5 chooses every other.

## 8. Earlier cases

The package lands in every implementation in one change, so each case of a
package's file changed in place, with no file named `until-`
(`15-build-rulings.md` R251).

1. **Core cases.** Two cases of `suite/core-run/parse-errors.json` expected
   `unexpected-token` for a text of E27 and E28: "two semicolons after the
   result" (`1;;`) and "a semicolon after a block's result" (`{ const a = 1;
   a; }`). By `03-host.md` 3.11 items 4 and 5 they would stand in a file named
   `until-f19-`, deleted when every implementation offers F19, which was in
   the change that added it; so they were removed. `suite/feedback/parse.json`
   holds the same texts with this package's outcome, named after the file and
   the case they stand for.
2. **`maybe-absent`.** The cases that state a message of `field` or `index`
   carry the texts of 2: the three cases of the entry in
   `suite/messages-check.json`, "R7-2: maybe-absent on a union prints flat(U)"
   of `suite/errors/fuzz-round-7.json`, "maybe-absent names the type" of
   `suite/recursion/e2-check.json`, and "under _ beside a guarded tag arm the
   union is not narrowed" of `suite/recursion/guards-check.json`.
3. **A type line after the result.** "a type line after the result" of
   `suite/entry-points/d1-syntax.json`, `1; type Foo = int;`, is E27: it
   expects `statement-sequence` at the `;`.
4. **The JSON form.** "rendered: a host error that is a Risor diagnostic
   repeats no prefix" of `suite/errors/rendered.json` expects the key `cause`
   of 5.2.
5. **A stub's diagnostic object.** "10: an error object with a third key" of
   `suite/testing/testing-invalid.json` gave its object the key `data`, which
   5.3 allows. It is now "10: an error object with a key other than data",
   with the key `class`, and two cases beside it test 5.3.
6. No other earlier case changes. No earlier case states the message of a
   diagnostic to which 3.4 gives a fix, and no other case writes `await` or
   `async` before an operand, or `;` after an expression.
