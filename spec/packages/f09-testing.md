# F9. Testing for authors

This chapter is normative. It specifies follow-on package F9. Core's case format
(`03-host.md` 3.10) is the suite's format; this package makes it a tool for the
author of a host's programs. It adds to Core:

1. **The fixture test**: before a case runs, each value it supplies and each
   answer of its stubs is tested against its declared type (2).
2. **Runner cases**: a case that holds a case file and expects the verdicts a
   runner gives it, so that a runner's own behaviour is in the suite (3).
3. **Update mode**: a runner writes the outcome of a case into its `expect` (4).
4. **Case files written as programs** (5).
5. **The report** of a case as text (6), the helpers each binding gives a host's
   own test framework (7), and the command `risor test` (8).

It adds no syntax, no node kind, no value, no run code, no check code, no
message template, no library entry, no cost row and no reserved word. It changes
the outcome of no earlier case (10).

The chapters of Core are cited by file and section: `03-host.md` 3.10 is section
3.10 of `spec/core/03-host.md`. F1 is `spec/packages/f01-host-types.md`, F4
`spec/packages/f04-conversion.md`, F8 `spec/packages/f08-errors.md`. Section
numbers without a file are of this chapter.

## 1. Conventions

### 1.1 Terms

| Term | Meaning |
| --- | --- |
| **Program case** | A case with `source` or `file`, which with F2 may be a template's text (`template` gives its mode) |
| **Fixture** | A value a case gives the run: a value of `vars`, and the `result` or `error` of a row of a stub (2.1) |
| **Outcome** | What a case gave: a result or a failure, as `03-host.md` 3.10.6 names their fields |
| **Verdict** | `pass`, `fail`, `skip` or `invalid` (`03-host.md` 3.10.8) |
| **Inner file** | The case file a runner case holds (3) |
| **Update status** | In update mode, `updated` or one of the four verdicts (4.3) |
| **JSON form** | The JSON form of `01-language.md` 12: no white space, keys in code point order, numbers in the rendering of `01-language.md` 6.3. An opaque value is written `{"$opaque":{"id":N,"type":"T"}}`, as a case file writes it (`03-host.md` 3.10.3 item 7) |
| **Visible text** | As F8 1.1 defines it |
| **Path text** | A list of keys and indexes written as F8 5.1 item 5 writes a path: `.vars.m.a`, `.functions["geo.distance"].calls[0].error` |

### 1.2 Requirements

1. An implementation that offers F9 offers sections 2 to 7 and 9. Packages are
   offered in the order of `03-host.md` 3.11 item 2, so it also offers the type
   checker, F1, F4 and its conformance function, F5 and F8.
2. Section 8 binds an implementation that ships a command line tool.
3. The fixture test (2) is made in every run of a case file by such an
   implementation: by `risor test`, by the helpers of 7, and when it runs the
   suite.

### 1.3 Version

F9 is part of version `3.0`. It adds no version number.

### 1.4 What this package changes in Core's rules

| Core rule | With this package |
| --- | --- |
| `03-host.md` 3.10.3 item 6 | The runner tests the fixtures (2). `"precondition": false` turns the test off |
| `03-host.md` 3.10.5 | Gains the fixture test between steps 4 and 5 (2.2). Its failure is the verdict `invalid` of that case alone |
| `03-host.md` 3.10.8 item 1 | The four verdicts stand. `invalid` is also the verdict of one case whose fixture fails (2.5), and the file is not invalid |
| `03-host.md` 3.10.10 | Gains the rules of 9.2 |
| `03-host.md` 3.10.10 item 6 | Does not apply in update mode (4.2) |
| `03-host.md` 3.10.11 | Gains the schema of 9.5 |

## 2. The fixture test

### 2.1 The fixtures

| Fixture | Its place | Tested against |
| --- | --- | --- |
| The value of the key k of `vars` | `["vars", k]` | The declared type of the variable k in the case's environment |
| The `result` of row i of the stub of function f | `["functions", f, "calls", i, "result"]` | The result type of f's signature, each type variable replaced by `any` |
| The `error` of row i of the stub of function f | `["functions", f, "calls", i, "error"]` | The mark `!` of f's signature |
| The `result` of row i of the stub of method `T.m` | `["methods", "T.m", "calls", i, "result"]` | The result type of the method's signature, each type variable replaced by `any` |
| The `error` of row i of the stub of method `T.m` | `["methods", "T.m", "calls", i, "error"]` | The mark `!` of the method's signature |

1. A value is tested by the conformance function of F4 7, with the type resolved
   in the case's environment (F4 3.5): its opaque types, its named types (F4
   9.1) and, for a variable declared by schema, the type F1 2.1 gives with the
   options the case names (F1 7.1).
2. An `error` passes when the signature has `!`, and fails when it has none: the
   mark is a promise (`05-types.md` 18.4).
3. A stub with no `type` takes its signature from the base environment
   (`03-host.md` 3.10.4). A bound (`where`) is not read.
4. These are not fixtures, and are not tested: the `args` of a row; a variable
   the case declares and does not supply; a variable the host supplies through a
   helper's base (7.1); a stub given as a signature alone, or with no `calls`,
   which has no row.
5. **Type variables.** A result type is tested with each `Var` replaced by
   `any`. So with `(x: T) => T[]`, the result `1` fails (`expected any[], found
   1`) and the result `["a"]` passes for every call. A result that conforms for
   no choice of the variables fails; a result that conforms for some other
   choice than the call's passes, and the run then finds it where it is read.

### 2.2 When the test is made

The fixtures of a case are tested when all of these hold:

1. The file is valid, and the case has no `skip`.
2. The case is a program case.
3. Its `precondition` is not `false`, after the header rule.
4. The rule of step 5 of `03-host.md` 3.10.5 runs it: it has `vars`, or its
   `expect` names what only a run gives. In update mode, a case that 4.4 takes
   through every stage is run too.
5. One of its runs (`03-host.md` 3.10.8 item 5) reaches step 5: its environment
   was built and its program compiled.

The test is made once for a case, before the first of its runs that reaches step
5, and its answer holds for every run of the case. A case that is run twice,
checked and with the static rules alone, is tested once: the test does not
depend on the check. A case that expects a `declare`, `parse` or `check` failure
never runs, and its fixtures are not tested; nor are those of a case whose every
run stops before step 5.

### 2.3 Order

1. The values of `vars`, in code point order of their keys.
2. Then the stubs of `functions`, in code point order of their names; for each,
   its rows in order; for each row, its `result` or its `error`.
3. Then the stubs of `methods`, in code point order of their keys, likewise.
4. The first fixture that fails ends the test. No other is tested.

### 2.4 A failure

1. The verdict of the case is `invalid`. The case is not run, and its `expect`
   is not compared. The fixture is wrong, not the program.
2. The verdict carries two facts:
   - **fixture**: the fixture's place (2.1) followed by the path of the
     conformance function's diagnostic, a list of keys and indexes. For a
     variable `m` whose field `a` fails: `["vars", "m", "a"]`. For an
     `error`: the place alone.
   - **message**: the message of the conformance function's diagnostic
     (F4 4.4), or, for an `error`, the text `the signature has no !, so the
     stub may not return an error`.
3. The other cases of the file are run as usual. The file is not invalid.
4. A case with `skip` has the verdict `skip`, and is not tested.

### 2.5 `precondition`

1. `"precondition": false` turns the test off for the case: none of its fixtures
   is tested. It says that the case breaks the precondition on purpose
   (`03-host.md` 3.10.3).
2. `"precondition": true` is the same as no `precondition`.
3. It may stand in the header. A case's `precondition` replaces the header's, as
   every field does.

### 2.6 What the test does not change

1. It is not a run. It charges no step and no unit, and the usage of the case is
   what it is without the test.
2. A case whose fixtures pass runs as it would with no test, with the same
   outcome.
3. So in a checked run of a case that passed the test, a failure of class
   `environment` comes only from a variable the case did not supply
   (`missing-variable`).

### 2.7 Examples

| Case | Verdict |
| --- | --- |
| `env` `{n: int}`, `vars` `{"n": "a"}` | `invalid`, fixture `["vars", "n"]`, `expected int, found "a"` |
| `env` `{n: int}`, `vars` `{"n": 3.0}` | Tested, passes: `3.0` is a whole number |
| `env` `{n: int}`, `vars` `{"n": 9007199254740992}` | `invalid`, `expected int, found 9007199254740992` |
| `env` `{m: {a?: int}}`, `vars` `{"m": {"a": null}}` | `invalid`, fixture `["vars", "m", "a"]`, `expected int, found null` |
| `env` `{b: int, a: int}`, both values strings | `invalid`, fixture `["vars", "a"]`: names in code point order |
| A stub `() => int` with rows `{"result": 1}`, `{"result": "x"}`, the second never called | `invalid`, fixture `["functions", "f", "calls", 1, "result"]` |
| A stub `() => int` with a row `{"error": "no"}` | `invalid`, fixture `["functions", "f", "calls", 0, "error"]` |
| The same with `"precondition": false` | Not tested; the run fails with `function-error`, class `environment` |
| `vars` that break their type, and an `expect` of phase `parse` | Not tested: the case never runs |

## 3. Runner cases

A runner case holds a case file, the **inner file**, and expects the verdicts a
runner gives it. It tests the runner itself: the header rule, each way a file is
invalid, stubs, the fixture test, which stages run, the report and the text an
update writes.

### 3.1 The fields

| Field | Holds | When absent |
| --- | --- | --- |
| `runner` | The inner file. An object or an array: the file as JSON data. A string: the text of the file | The case is not a runner case |
| `update` | `true` or `false`. `true`: the inner file is run in update mode (4) | `false` |
| `program` | `true` or `false`. `true`: the string is the text of a program case file (5) | `false` |

1. A case with `runner` has, after the header rule, only `name`, `runner`,
   `update`, `program`, `skip` and `expect`.
2. `runner`, `update` and `program` may not stand in a header.
3. `update: true` and `program: true` each need `runner` to be a string.
4. `source`, `file`, `conform`, `render` and `runner` count as one field for the
   header rule (`03-host.md` 3.10.2 item 4). `template` is not among them: it is
   the mode of a text that stands in `source` or `file`, and a header may give
   it for every case.

### 3.2 How a runner case is run

1. The inner file is read and run as a case file in the directory of the file
   that holds the runner case, in the base environment of that run. A `file` in
   it is a path relative to that directory.
2. A string is the file's text: its code points, read as the UTF-8 text of a
   file would be. Rules that only text can break (`03-host.md` 3.10.10 items 1,
   13 and 15) can be stated only in this form.
3. Each case of the inner file is run as `03-host.md` 3.10.8 item 5 says for a
   case outside `suite/core-check/`.
4. With `update: true`, the inner file is run in update mode. Nothing is written
   to disk: the text update mode would write is compared with `text`.
5. An inner file may hold runner cases. Each is run by this section.
6. A runner case is run once. It has no stages, no fixtures and no usage of its
   own.

### 3.3 What is expected

The `expect` of a runner case is one of two objects.

| Field | Holds |
| --- | --- |
| `invalid` | `true`: the inner file is invalid. No other field stands beside it |
| `verdicts` | A list: one entry for each case of the inner file, in the file's order (3.4) |
| `report` | With `verdicts` only. The lines of 6 for the inner file's cases, each ending with U+000A; the empty string when every case passed |
| `text` | With `verdicts`, and only when `update` is `true`. The text of the inner file after update mode: its text unchanged when nothing is written |

### 3.4 An entry of `verdicts`

| Field | Holds | Present |
| --- | --- | --- |
| `name` | The case's name | Always |
| `status` | `pass`, `fail`, `skip`, `invalid`, or with `update: true` also `updated` | Always |
| `differs` | The names of what differed, in the order of `03-host.md` 3.10.8 item 2 and of the packages that add names | Only with `fail`. Optional |
| `fixture` | The fixture of 2.4 item 2 | Only with `invalid`. Optional |
| `message` | The message of 2.4 item 2 | Only with `invalid`. Optional |

### 3.5 How it is compared

1. `invalid` is compared first: the inner file is invalid exactly when the case
   expects `invalid`. A difference is the difference `invalid`, and nothing else
   is compared.
2. `verdicts` is compared whole: the same number of entries, and each expected
   entry with the actual one at the same position. In each, `name` and `status`
   are compared, and `differs`, `fixture` and `message` when the entry gives
   them. A difference is the difference `verdicts`.
3. `report` and `text` are compared code point by code point. A difference is
   the difference `report` or `text`.
4. The differences of a runner case are named `invalid`, `verdicts`, `report`
   and `text`, in that order.
5. Why an invalid file is invalid is the runner's own report. No case compares
   it: a runner case expects `invalid` only, and each rule of `03-host.md`
   3.10.10 has a case whose valid twin differs from it in that rule alone.

## 4. Update mode

### 4.1 Asked for by

1. `risor test --update` (8.1).
2. A helper's option, or the variable `RISOR_UPDATE` of the process environment
   when its value is exactly `1` (7.1).
3. A runner case with `"update": true` (3.2 item 4).

Update mode is never on by default, and never implied by a missing `expect`.

### 4.2 The file

1. In update mode a case may have no `expect`. Item 6 of `03-host.md` 3.10.10
   does not apply. Every other rule does.
2. An invalid file is reported as in an ordinary run, and is not written.
3. Each case is run as in an ordinary run, except as 4.4 says, and gets an
   update status (4.3).
4. The file is written once, after all its selected cases have run (8.1, 7.1),
   and only when at least one case is `updated`. Otherwise it is left as it was.

### 4.3 The update status of a case

| The case | Status | Written |
| --- | --- | --- |
| Has `skip` | `skip` | No |
| Its fixture test fails (2) | `invalid` | No |
| Passes | `pass` | No: an `expect` that passes is kept as it is spelled |
| Fails on `functions` (`03-host.md` 3.10.4) | `fail` | No |
| Its outcome is a `declare` failure, and its `expect` is absent or not a failure of phase `declare` | `fail` | No |
| Its new `expect` would hold a map with the key `$opaque`, or would make the file nest deeper than `03-host.md` 3.10.1 item 4 allows, or would be a `declare` failure with more than one diagnostic | `fail` | No |
| A conform case (F4 9.2) whose three ways disagree | `fail` | No |
| A runner case, a render case (F8 7.4), or a case not selected (8.1) | As in an ordinary run, or not run | No |
| Any other case | `updated` | Yes |

1. A case that is not written and has the status `fail` has the differences of
   an ordinary run. A case with no `expect` differs in `outcome`.
2. A map with the key `$opaque` means a map value of the language. An opaque
   value is written as the case file writes one, and is not such a map.

### 4.4 The new `expect`

**Which run.** The new `expect` is made from the outcome of the checked run of
the case, or of the run with the static rules alone when the case has `"check":
false`.

**Same shape or written whole.** The new `expect` keeps the author's choice of
fields when the outcome has the shape of the old `expect`: both results, both
failures of the same phase, or both conform results. Otherwise, and when the
case has no `expect`, it is written whole. A case written whole is taken through
every stage: it is run when its program compiles, whether or not it has `vars`.

| The new `expect` | Fields |
| --- | --- |
| A result, same shape | Each field of the old `expect`, with the outcome's value: `type`, `sites`, `value`, `truthy` (the truthiness test of `03-host.md` 3.2.4 on the value), `steps`, `allocated`, and the fields of packages: `description` (F1), `analysis` (F2), `json` (F6). An absent field stays absent |
| A result, written whole | `type`, when the case was checked; `value`, `steps` and `allocated` |
| A failure, same phase | `phase`; `diagnostics`, each diagnostic with the columns below; `steps` and `allocated` when the old `expect` had them and the code is not a limit code; F8's `rendered` when the old `expect` had it, with the forms it named, and not for a limit code |
| A failure, written whole | `phase`, and `diagnostics` with every column the phase may write |
| A conform result, same shape | Each of `conforms`, `path`, `message`, `steps` and F8's `failures` the old `expect` had. `path` and `message` are left out when `conforms` is `true`; `failures` is left out when `conform` has no `max` |
| A conform result, written whole | `conforms`; `path` and `message` when it is `false`; `steps` |

**The columns of a diagnostic.** For a failure of the same phase, the columns
are `code` and every field that some diagnostic of the old list has.

| Phase | Diagnostics written | Columns it may write | Written whole |
| --- | --- | --- | --- |
| `declare` | The one | `code` | `code` |
| `parse` | The first | `code`, `span`, `message` | `code`, `span`, `message` |
| `check` | Every one, in order | `code`, `class`, `span`, `message` | `code`, `class`, `span`, `message` |
| `run` | The one | `code`, `class`, `span`, `path`, `message`, `data`, `trace` | `code`, `class`, `span`, `path`, `message`, `data` |

A column is written for a diagnostic only when the diagnostic has the field:
`class` when it has a class; `span` and `trace` never for a limit code; `path`
for `not-conforming` only; `data` for `failed` only. The messages of `parse` and
`check` are F8's (F8 1.2 item 2); the message of a `declare` diagnostic is not
specified, and is never written.

### 4.5 The text written

1. The new `expect` is written in its JSON form (1.1).
2. A case object that has the member `expect`: the text of that member's value,
   from its first code point to its last, is replaced by the JSON form. The key,
   the colon and the white space around them stay.
3. A case object with no `expect`: the text `, "expect": ` followed by the JSON
   form is put directly after the last code point of the value of the object's
   last member, in the order of the text. White space after that value stays
   after the new member.
4. Every other code point of the file stays as it was: the header, the other
   cases, the other members of the case, white space, line ends, escapes in
   strings and the spelling of numbers.
5. A file that is an array is written the same way: its elements are the case
   objects.
6. Three implementations write the same text for the same file and outcomes.

Example: the case `{"name": "sum", "source": "1 + 2"}` with no `expect` becomes
`{"name": "sum", "source": "1 + 2", "expect":
{"allocated":0,"steps":1,"type":"int","value":3}}`.

### 4.6 A second update

A case written by update mode passes in the next run: its new `expect` holds the
outcome's values, which 3.10.7 compares equal, and a run is deterministic. So a
second update writes nothing, and the file keeps the same text.

### 4.7 Never written

An invalid file. A program case file (5). A case of 4.3 that is not written. An
update never makes a case stricter: a field the author left out stays out.

### 4.8 Reports

In update mode, the report (6) has the line ` UPDATED ` and the name for each
case written. A case not written has the lines of its verdict.

## 5. Case files written as programs

### 5.1 The form

1. A **program case file** is a file whose name ends in `.test.risor`, or the
   inner file of a runner case with `"program": true`.
2. Its text is a program (`01-language.md`). Its value is the case file: a map
   (a header and `cases`) or a list (cases with no header).

### 5.2 Loading

| Step | What happens | A failure makes the file invalid, with |
| --- | --- | --- |
| 1. Parse | The text is parsed | The `parse` diagnostics |
| 2. Check | The program is compiled with the type check in the default environment of the newest version (`03-host.md` 3.10.9 item 2), with no expected result. It may use the forms of every package the implementation offers | The `check` diagnostics |
| 3. Run | The program is run with the default limits and no deadline | The `run` diagnostic |
| 4. Read | The value is read as the JSON of the same case file would be, by every rule of `03-host.md` 3.10 and of the packages | The rule broken |

1. The base environment of the cases is not the environment of steps 2 and
   3. A helper's base and variables (7.1) do not reach the program.
2. A diagnostic of steps 1 to 3 has positions in the program file's text.
3. The value of step 4 is never a function: a program cannot give one. So a stub
   stays a table.

### 5.3 From the value to the JSON

| Value | Read as |
| --- | --- |
| A map | An object with the same keys. No key is repeated: a map literal cannot repeat a key (`duplicate-key`) |
| A list | An array |
| A string | A string, code point for code point |
| A number | A number. It is already a double; no infinity can occur |
| `true`, `false`, null | The same |

1. A map whose only key is `$opaque` stands for an opaque value where a host
   value stands, as in JSON (`03-host.md` 3.10.3 items 7 to 9).
2. The depth of the value is counted as for JSON text (`03-host.md` 3.10.1 item
   4).
3. A `file` in it is relative to the directory of the program case file; for a
   runner case, to the directory of the file that holds the runner case.

### 5.4 What a program case file does not give

1. Update mode never writes one (4.7). It is run as in an ordinary run.
2. In 3.0 the value is not checked against a type of a case file. A later minor
   version may add that check: it refuses only values that step 4 refuses.

### 5.5 In 3.0

Program case files are part of F9, and so of 3.0. The suite's own files are
JSON; its program case files stand inside runner cases.

## 6. The report

The report is the text an author sees for the cases of one file. `risor test`
writes it (8.3), a helper gives it to the host's test framework (7), and a
runner case may compare it (3.3).

### 6.1 The lines of a case

A case that passes has no line. For every other case, in the file's order:

| Status | Lines |
| --- | --- |
| `fail` | `  FAIL ` and the name. Then one line for each difference, in order (6.2) |
| `invalid`, by its fixture | `  INVALID ` and the name. Then `    fixture `, the path text of the fixture (2.4), `: ` and the message |
| `skip` | `  SKIP `, the name, `: ` and the reason |
| `updated` | `  UPDATED ` and the name |

1. Each line ends with U+000A.
2. The name, the reason and the message are written as visible text.

### 6.2 A line for a difference

` ` (four spaces), the name of the difference, `: expected `, E, `, found `, F.
For `functions` the line is ` functions: no row of `, the stub's name, ` matches
`, and the arguments of the first call that no row matched, as a list in its
JSON form. For a method the name is `T.m`, and the receiver is the first
argument, as in a row (`03-host.md` 3.10.4 item 2).

| Difference | E | F |
| --- | --- | --- |
| `outcome` | The JSON form of the whole `expect`. With no `expect` (update mode), the word `nothing` | The outcome with the fields of an `expect` written whole (4.4), those it has: a result that did not run has no `value`, `steps` or `allocated` |
| `type` | The JSON form of the type text as written | The JSON form of the type the check gives, printed by `05-types.md` 3.9 |
| `diagnostics` | The JSON form of the expected list | The outcome's diagnostics with the columns of the expected list (4.4) |
| Any other | The JSON form of the expected field | The JSON form of the outcome's field |

1. A value read from the case file is written by its JSON form: `2.0` is written
   `2`.
2. F comes from the first run that differs in that name: the checked run, then
   the run with the static rules alone.
3. E and F are written whole, never cut.

Example, for `{"name": "a", "source": "1 + 2", "expect": {"value": 4}}`:

```
  FAIL a
    value: expected 4, found 3
```

### 6.3 An invalid file

An invalid file has the line ` INVALID`, then one or more lines that say why,
each starting with four spaces. Their text is the runner's own, and is not
specified.

## 7. The bindings

### 7.1 The portable contract

Every binding gives a helper that runs a case file inside the host language's
own test framework. It is the runner the implementation runs the suite with.

| Matter | Rule |
| --- | --- |
| A file | Loaded from a path: a program case file when the name ends in `.test.risor`, else JSON. Or from text, with a flag for a program. A binding may also take parsed data |
| An invalid file | Loading gives an error that holds the runner's reason. No case is run. A file is reported invalid once, when it is loaded, before any case runs |
| The base | The host's environment, or the default environment when none is given. **It is given to every call that loads a file.** The rules of `03-host.md` 3.10.10 that read the base (item 11: a stub with no `type`) are applied against it at the load, and every case of the file runs in a child of it. The base is never changed (`03-host.md` 3.10.9). A binding that also takes a base when a case is run accepts only the base the file was loaded with, and gives its own error for another |
| Variables | The host may give variables for the names a case does not supply (`03-host.md` 3.10.9 item 3). They are not fixtures (2.1 item 4) |
| Sources | The host may give a reader for `file`, which returns a text and an origin (`03-host.md` 3.7). Spans are positions in the text it returns |
| A filter | A text: only the cases whose names contain it, code point for code point, are run (8.1) |
| Update | An option, or `RISOR_UPDATE=1` (4.1). A file loaded from a path is written in place. A file loaded from text gives its new text to the host. A file built from parsed data has no text: update mode gives the update statuses (4.3) and the new `expect` of each case, and no new text; the binding writes nothing and does not make a text from the data |
| When it is written | Once, when every case the helper selects has run in that update (4.2 item 4). The selected cases are those its filter keeps, all of them when it has none. When the host's framework runs only some of them, by a filter of its own, the file is left as it was |
| A verdict | For each case: its status, its differences (each a name, E and F of 6.2), its fixture and message (2.4), its skip reason, and the `expect` that update mode would write |
| The framework | One test for each case, named by the case's name. `pass`: the test passes. `skip`: the test is skipped, with the reason. `fail` and `invalid`: the test fails, and its message is the lines of 6.1 for that case. An invalid file: one test that fails with the reason |
| Agreement | With the default environment as the base, and no variables or reader of its own, a helper gives every case the verdict `risor test` gives it, in every implementation |

### 7.2 Go (binding rules)

```go
package risortest // github.com/deepnoodle-ai/risor/go/v3/risortest

func Run(t *testing.T, base *risor.Env, path string, opts ...Option)
func Load(path string, opts ...Option) (*File, error) // an error: invalid
func Parse(text []byte, program bool, opts ...Option) (*File, error)
func Base(env *risor.Env) Option // the base of the load and of every case
func (f *File) Cases() []*Case
func (c *Case) Run(ctx context.Context) Verdict // in the base of the load
func (f *File) Update(verdicts []Verdict) ([]byte, error) // the new text
func Vars(v risor.Vars) Option
func Sources(read func(ref string) (string, risor.Origin, error)) Option
```

1. `Run` gives one subtest for each case with `t.Run(name, ...)`. Go's own
   rewriting of a subtest's name applies, so `go test -run 'TestX/gold'` selects
   cases.
2. `skip` calls `t.Skip` with the reason. `fail` and `invalid` call `t.Error`
   with the lines of 6.1. An invalid file calls `t.Fatal` on the parent test.
3. With `RISOR_UPDATE=1`, `Run` writes the file after its subtests end.
4. A nil base is the default environment. `Run` loads the file with its base.
   `Load` and `Parse` take it by `Base`. A case runs in the base of its load,
   so `Case.Run` takes no base.

### 7.3 TypeScript (binding rules)

```ts
// "risor/testing". It imports no test framework.
export function defineCases(test: TestFn, file: CaseFile): void;
export function loadCaseFile(path: string, options?: CaseOptions): CaseFile; // Node
export function caseFile(data: unknown, options?: CaseOptions): CaseFile;
export function caseFileText(text: string, program: boolean, options?: CaseOptions): CaseFile;
export interface CaseFile { readonly cases: readonly Case[]; updatedText(): string | null }
export interface Case { readonly name: string; run(): Verdict } // in the base of the load
type TestFn = ((name: string, fn: () => void) => void) & {skip?: (name: string, fn: () => void) => void};
```

1. `defineCases` calls `test(name, fn)` for each case. `fn` throws an `Error`
   whose message is the lines of 6.1 when the case fails or is invalid.
2. A skipped case is given to `test.skip(name, fn)` when `test.skip` is a
   function, and is not registered otherwise. vitest, jest and `node:test` each
   have `test.skip` (from memory).
3. An invalid file throws from `loadCaseFile`, `caseFile` or `caseFileText`.
4. Update: `options.update`, or `process.env.RISOR_UPDATE === "1"` where
   `process` exists. `loadCaseFile` writes the file in place once every selected
   case has run; `updatedText()` gives the new text, or `null` when nothing is
   written. A file from `caseFile(data)` has no text: its `updatedText()` is
   always `null`.
5. `CaseOptions.base` is the base of 7.1: the file is checked against it when it
   is loaded, and its cases run in children of it. A case runs in the base of
   its load, so `defineCases` and `Case.run` take no base.

### 7.4 Rust (binding rules)

```rust
// risor::testing
pub fn load(path: impl AsRef<Path>, base: &Env) -> Result<CaseFile, Error>;
pub fn from_text(text: &str, program: bool, base: &Env) -> Result<CaseFile, Error>;
impl CaseFile {
    pub fn cases(&self) -> &[Case];
    pub fn filter(self, text: &str) -> CaseFile;
    pub fn run(&self, vars: &dyn Vars) -> Report;
}
impl Case   { pub fn run(&self, vars: &dyn Vars) -> Verdict; }
impl Report {
    pub fn verdicts(&self) -> &[Verdict];
    pub fn text(&self) -> String;           // the lines of 6.1
    pub fn updated_text(&self) -> Option<String>;
    pub fn assert_ok(&self);                // panics with text() unless every case passed or skipped
}
```

1. The standard harness cannot add a test at run time, so a host writes one
   `#[test]` for each file. The report names the cases.
2. With `RISOR_UPDATE=1`, `run` of a file loaded by `load` writes it in place
   once its selected cases have run.
3. A case runs in the base given to `load` or `from_text`, so `run` and
   `Case::run` take no base.

## 8. The command `risor test`

### 8.1 Arguments

```
risor test [PATH ...] [--run TEXT ...] [--update] [--format text|json]
risor test --suite DIR [--format text|json]
```

| Argument | Meaning |
| --- | --- |
| `PATH` | A file or a directory (8.2). With none, the current directory |
| `--run TEXT` | Runs only the cases whose names contain TEXT, code point for code point. Given more than once, a case named by any is run. A case not run is not reported, not counted and not written |
| `--update` | Update mode (4) |
| `--format` | `text`, the default (8.3), or `json` (8.4) |
| `--suite DIR` | Runs the conformance suite at DIR (8.6) |

1. The base environment is the default environment (`03-host.md` 3.10.9 item 2).
   Every host function is a stub, so the verdicts are the same in every
   implementation, step counts included.
2. A filter is a text, not a pattern: the three implementations share no pattern
   language.

### 8.2 Finding files

1. A `PATH` that is a file is run, whatever its name: as a program case file
   when its name ends in `.test.risor`, else as JSON.
2. A `PATH` that is a directory is searched, through its subdirectories, for
   files whose names end in `.test.json` or `.test.risor`. A file or a directory
   whose name begins with `.` is passed over. A symbolic link is not followed.
3. The files found in one directory are run in code point order of their paths
   relative to it, with `/` between the parts, compared part by part. Paths
   given as arguments are run in the order given.
4. A `PATH` that does not exist is an error: a line that names it on standard
   error, and the status 2.

### 8.3 Text

For each file that has at least one line of 6, in the order run: a line with the
file's path as given or found, then the lines of 6 for its cases, or of 6.3 for
an invalid file. Then one summary line:

```
{F} files, {C} cases: {P} pass, {X} fail, {S} skip, {I} invalid
```

1. F counts the files run. C counts the cases of valid files that the filter
   selects. I counts the cases whose verdict is `invalid` (2.4).
2. When a file is invalid, `; {B} files invalid` follows, with B the number of
   invalid files.
3. In update mode, `, {U} updated` follows `{I} invalid`.
4. Everything is written to standard output. A terminal may color the words
   `FAIL`, `INVALID`, `SKIP` and `UPDATED`; no other change is allowed.

### 8.4 JSON

One line for each case, in the order run: an object in its JSON form with the
keys of this table that the verdict has.

| Key | Value |
| --- | --- |
| `differs` | For `fail`: the names of the differences |
| `file` | The file's path, as in 8.3 |
| `fixture` | For `invalid` by its fixture: the list of 2.4 |
| `message` | For `invalid`: the message of 2.4, or, for an invalid file, the runner's reason |
| `name` | The case's name. Absent for an invalid file |
| `reason` | For `skip`: the reason |
| `status` | The verdict, or the update status |

An invalid file has one line, with `file`, `message` and `status`. No summary
line is written.

### 8.5 Exit status

| Status | When |
| --- | --- |
| 2 | A file is invalid, a case is `invalid`, or a `PATH` does not exist |
| 1 | Otherwise, a case is `fail` |
| 0 | Otherwise: every case passed, was skipped, or was written by update mode |

### 8.6 The conformance suite

`risor test --suite DIR` runs the suite at DIR: the paths of `03-host.md`
3.10.13, `library.json`, `messages.json`, and each directory and catalog file of
the packages the implementation offers (`03-host.md` 3.11). Each case is run as
`03-host.md` 3.10.8 item 5 says. The text output is one line for each path, in
code point order of the paths: the path, `: `, then `{P} pass, {X} fail, {S}
skip, {I} invalid`. With `--format json` it is the lines of 8.4. This is the
report of `03-host.md` 3.10.8 item 7. The exit status is that of 8.5.

### 8.7 What this section leaves to others

1. The renderings of a diagnostic of an invalid program case file are F8's (F8
   5). Which form `risor test` uses for them, and every further command, flag,
   progress output and watch mode, are F12's.
2. F12 may add flags. It does not change what this section fixes.

## 9. Additions to the case format

Each addition is optional, and its absence leaves every existing case file valid
with its meaning unchanged.

### 9.1 The fields

| Field | Where | Section |
| --- | --- | --- |
| `runner`, `update`, `program` | A case | 3.1 |
| `invalid`, `verdicts`, `report`, `text` | The `expect` of a runner case | 3.3 |

### 9.2 When a file is invalid

These join the list of `03-host.md` 3.10.10.

1. `runner` that is not a string, an object or an array.
2. `update` or `program` that is not `true` or `false`; either in a case with no
   `runner`; `true` beside a `runner` that is not a string.
3. `runner`, `update` or `program` in a header.
4. A runner case with a field other than those of 3.1 item 1, after the header
   rule.
5. A runner case whose `expect` is not of 3.3: `invalid` that is not `true`;
   `invalid` beside another field; neither `invalid` nor `verdicts`; `verdicts`
   that is not a list of entries of 3.4; `report` or `text` that is not a
   string; `text` when `update` is not `true`.
6. An entry of `verdicts` with a field other than those of 3.4, a `status` not
   among them, `updated` when `update` is not `true`, `differs` that is not a
   list of strings or beside a status other than `fail`, `fixture` that is not a
   path (`03-host.md` 3.7.8) or `message` that is not a string, or either beside
   a status other than `invalid`.
7. `invalid`, `verdicts`, `report` or `text` in the `expect` of a case that is
   not a runner case.

The inner file of a runner case is not part of the file that holds it for these
rules: an inner file that is invalid is the verdict the case expects.

### 9.3 The runner

1. A runner of an implementation that offers F9 runs `suite/testing/`. Each case
   there is run as `03-host.md` 3.10.8 item 5 says, but a runner case.
2. **A runner case is run once.** It has no program, so it has no checked run
   and no run with the static rules alone. A runner that makes a second pass
   with the static rules alone does not run it there and takes the verdict of
   its one run; it is never `skip` for want of that pass. Each case of its inner
   file is run as item 1 says, checked and with the static rules alone where
   `03-host.md` 3.10.8 item 5 runs it both ways.
3. A runner of an implementation that does not offer F9 does not run
   `suite/testing/`. For it the fields of 9.1 are unknown fields (`03-host.md`
   3.11 item 3).
4. `suite/testing/programs/` holds program texts that cases read with `file`. It
   holds no case file.

### 9.4 The `outcome` command

The `outcome` command (`suite/README.md`) writes, for a runner case, `name`,
then either `invalid`, `true`, or `verdicts`, then `report`, then `text` when
the case has `update: true`. The fuzzers do not make runner cases.

### 9.5 The JSON Schema of the additions

These are added to the schema of `03-host.md` 3.10.11: `"runner"`, `"update"`
and `"program"` in the list of `propertyNames` of a case (not of the file), the
three properties below in `$defs/case`, and `$defs/runnerResult` as a branch of
`$defs/expect`.

```json
{
  "case": {"properties": {
    "runner": {"oneOf": [{"type": "string"}, {"type": "object"}, {"type": "array"}]},
    "update": {"type": "boolean"},
    "program": {"type": "boolean"}}},
  "runnerResult": {
    "oneOf": [
      {"type": "object", "required": ["invalid"],
       "properties": {"invalid": {"const": true}},
       "additionalProperties": false},
      {"type": "object", "required": ["verdicts"],
       "properties": {
         "verdicts": {"type": "array", "items": {"$ref": "#/$defs/verdict"}},
         "report": {"type": "string"},
         "text": {"type": "string"}},
       "additionalProperties": false}]},
  "verdict": {
    "type": "object",
    "required": ["name", "status"],
    "properties": {
      "name": {"type": "string"},
      "status": {"enum": ["pass", "fail", "skip", "invalid", "updated"]},
      "differs": {"type": "array", "items": {"type": "string"}},
      "fixture": {"$ref": "#/$defs/diagnostic/properties/path"},
      "message": {"type": "string"}},
    "additionalProperties": false}
}
```

## 10. Costs, and earlier cases

1. Nothing in this chapter charges a step or a unit. The fixture test, a runner
   case, update mode and the report are the runner's work.
2. No earlier case changes its outcome. Every earlier case that will run either
   passes the fixture test or carries `"precondition": false`. The fields of 9.1
   appear in no earlier case.
3. No earlier case pins an error that this package turns into a result.
