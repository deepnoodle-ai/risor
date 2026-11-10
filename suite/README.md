# The conformance suite

The data every implementation of Risor v3 shares. The specification in
`spec/` says what the language is; these files test that an implementation
does it. A case file is JSON, in the format of `spec/core/03-host.md` 3.10.

## Layout

| Path | Holds | Specified by |
| --- | --- | --- |
| `core-run/` | Core cases that need no type checker: values, counts, spans, failures | `spec/core/` |
| `core-check/` | Core cases of the type checker | `spec/core/05-types.md` |
| `library.json` | The library's entries, with cases for each | `spec/core/04-library.md` |
| `messages.json` | The templates of the run-phase messages, with cases | `spec/core/03-host.md` |
| `casemap.json` | The case mapping of text, taken from the Unicode data | `spec/core/04-library.md` |
| `*/until-fNN-*.json` | Earlier cases whose outcome package `NN` changes; not run by an implementation that offers it | `spec/core/03-host.md` 3.11 |
| `strings/` | Package F3: raw strings and text blocks | `spec/packages/f03-strings.md` |
| `host-types/` | Package F1: types from JSON Schema, descriptions | `spec/packages/f01-host-types.md` |
| `templates/`, `analysis/` | Package F2: host templates, and the analysis of a program | `spec/packages/f02-templates-analysis.md` |
| `conversion/` | Package F4: `as`, `is`, named types, the conformance function | `spec/packages/f04-conversion.md` |
| `unions/` | Package F5: tagged unions, narrowing by the tag | `spec/packages/f05-unions.md` |
| `library-groups.json`, `groups/` | Package F6: the groups `json`, `time`, `math` and `base64`, as records with their cases, and cases that need more than one entry | `spec/packages/f06-groups.md` |
| `library-queries.json`, `messages-queries.json`, `queries/` | Package F7: the step `[*]`, six list and map methods, a direction for `sortBy`; their records, their three message templates, their cases | `spec/packages/f07-queries.md` |
| `messages-parse.json`, `messages-check.json`, `errors/` | Package F8: the templates of the parse and check phases with their cases; targeted parse codes, messages, traces, renderings, `ConformAll` | `spec/packages/f08-errors.md` |
| `testing/` | Package F9: fixtures tested against their types, cases that test a runner, update mode, case files written as programs | `spec/packages/f09-testing.md` |
| `config/` | Package F10: a program as a config file: the expected result by schema, the load, the decode, layering by spread | `spec/packages/f10-config.md` |
| `library-math.json`, `math/` | Package F13: the correctly rounded functions of `math` | `spec/packages/f13-math.md` |

A runner runs the Core paths and the directories of the packages its
implementation offers, and reports each by itself (`03-host.md` 3.11).

## Rules for cases

1. A case you believe is wrong is a question, not a case to skip or edit.
2. A package adds files. It changes the outcome of no earlier case, except
   for a text it gives a new outcome to: that case moves to a file named
   `until-fNN-...` in its own directory, and the package holds a case with
   the same text.
3. A runner whose implementation offers package `NN` does not run a file
   named `until-fNN-...`. Every other runner does. Such a file is deleted
   when every implementation offers the package.

## The `outcome` command

Each implementation has a small command, `outcome`, used to compare the
implementations with one another: by the fuzzers in `fuzz/`, and by anyone
writing cases. It is a test tool, not part of the host API.

| Implementation | Run from its directory |
| --- | --- |
| Go | `GOWORK=off go run ./cmd/outcome FILE` |
| TypeScript | `node scripts/outcome.ts FILE` |
| Rust | `cargo run --quiet --release --bin outcome -- FILE` |

1. `FILE` is a case file in which a case may have no `expect`. The command
   ignores every `expect`.
2. It writes to standard output one line of JSON for each case, in the file's
   order. It ends with status 0 when it has written a line for every case,
   whatever the cases found, and with status 2 and a message on standard
   error when the file is invalid.
3. It compiles each case with the type check, and runs every case that
   compiles. A case with no `vars` is run with no variable supplied. A case
   with `"check": false` is compiled with the static rules alone.
4. It sets no deadline and never reads the clock, so its output for a file is
   the same every time.

A line is one JSON object with these fields, in this order.

| Field | Value |
| --- | --- |
| `name` | The case's name |
| `description` | Package F1 only, when the case has `schemas` and its environment was built: the environment's description |
| `type`, `sites` | The inferred type as printed text, and the sites of the check, when the check passed. Absent for a case with `"check": false` |
| `phase` | `null` when the run gave a value. Otherwise the phase that failed: `"parse"`, `"declare"`, `"check"` or `"run"` |
| `value` | The result as JSON, when `phase` is null. An opaque value is written as in a case file: `{"$opaque": {"type": T, "id": N}}` |
| `diagnostics` | When `phase` is not null, a list of objects with `code`, `span`, `class` and `message`. For `parse`: the first diagnostic, with `code` and `span`. For `declare`: each with `code` only, sorted by code. For `check`: every diagnostic in the order the types chapter gives, with `code` and `span`. For `run`: the one failure, with all four fields |
| `steps`, `units` | The usage of the run, when the case got as far as a run |
| `analysis` | Package F2 only, when the text parses: the analysis report |

1. A number is written in the shortest form that reads back as the same
   double. A comparison reads numbers as doubles, not as text.
2. For a run failure whose class is `limit`, `span` is null and the line has
   no `steps` and no `units`: the specification does not fix them. The code,
   the class and the message are fixed.
3. Two implementations must write the same line for the same case, apart from
   the spelling of a number. A crash, a hang or an overflow of the host stack
   is a fault of the implementation.
