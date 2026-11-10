# The fuzzers

This directory holds two fuzzers of Risor v3, in one command.

- The **differential** fuzzer makes programs, runs each one in every
  implementation, and reports each difference.
- The **soundness** fuzzer (`-sound`) tests the promise of the checker,
  `spec/core/05-types.md` 18.4, in each implementation on its own.

The language promises exact agreement. Two implementations give the same value,
or the same failure with the same code, span, class and message. They charge the
same steps and allocation units, so a run reaches a limit at the same point
everywhere. The fuzzer tests that promise on programs that no one wrote by hand.

It is its own Go module, with no dependency outside the standard library and
none on any implementation. It is not in `go.work`. Run Go here with
`GOWORK=off`.

## How it works

1. **Generate.** For each round, the generator makes a case file of `03-host.md`
   3.10, with no `expect`. Each case has a program and an environment: `env`,
   `vars`, and sometimes an opaque type with stubbed methods, stubbed host
   functions, and small limits. The seed, the round and the case's index fix
   every choice, so a run can be repeated.
2. **Run.** Each implementation runs the file through its `outcome` command
   (`suite/README.md`). Each command writes one JSON line for each case.
3. **Compare.** The lines for one case are compared as `suite/README.md` says:
   equal apart from the spelling of a number. That covers the `type` and the
   `sites` of the check (in order, with their spans), the phase, the value or
   every diagnostic, and the counts. The fields `description` and `analysis`
   belong to packages an implementation may not offer: each is compared among
   the implementations that write it, and ignored where only one does. Numbers
   compare as doubles, so `2` and `2.0` agree. For a run failure of class
   `limit`, only the code, the class and the message are compared: the span and
   the usage are not fixed (`03-host.md` 3.6.1, 3.6.3). A crash, a timeout, a
   status other than 0 and a missing line are differences too.
4. **Shrink.** Each difference is made smaller while it stays the same
   difference, and written to `findings/`.

Each implementation compiles a case with the type check. A share of cases, 20%
by default, carries `"check": false` and is compiled with the static rules
alone, so that the unchecked path is compared too. A variable is declared with
the type of its value most often; sometimes `any`; sometimes a wider type (a
field made optional, `| null` added, an open record); and sometimes a type the
value does not conform to, with `"precondition": false`. About one case in
twelve removes one or two library entries with the field `removed` (`03-host.md`
3.3.1 item 6), and its program may still call them.

The generator has three modes, mixed by weight:

| Mode | What it makes | For |
| --- | --- | --- |
| typed | Programs that are well formed and mostly well kinded. The generator tracks a rough kind for each subexpression. | Values, deep evaluation and exact counts |
| loose | Any tree the grammar allows, kinds mostly ignored, with breaks of the static rules and bad declarations | Run failures of every code, `check` and `declare` |
| mangled | A program with a token deleted, doubled, swapped or replaced, a character changed, the text cut short, a byte order mark, deep nesting, or a comment | The lexer and the first `parse` diagnostic |

It also writes the string forms of package F3 (`spec/packages/f03-strings.md`):
raw strings `r"..."` and `r'...'`, text blocks and raw text blocks, with several
closing indentations. They stand as string operands, map keys, keys of map
patterns and literals of `match` arms, and inside the interpolations of
templates and text blocks, nested. The mangled mode adds their broken forms:
text on the opening or closing line, bad indentation, a missing close, `r'''`, a
raw string cut short, and line ends written CR or CR LF. `-nof3` leaves the
package out.

It also writes the forms of packages F4 and F5
(`spec/packages/f04-conversion.md`, `spec/packages/f05-unions.md`):

- `x as T` and `x is T`, with program types: the primitive types, literal sets,
  lists, exact and open records, `map<T>`, nullable types, tagged unions of
  records and unions with no tag, and named types;
- named types declared by the case field `types` (a union `Shape`, a record
  `Item`, a list `Items`), and, in loose cases, declarations that fail;
- variables whose declared type is a tagged union, nullable at times, with a
  value that conforms;
- tests of a tag with `==`, `!=`, `in`, `not in` and `match` that narrow a union
  and read a field of the variant; `v is T ? e : d`, where `e` uses `v` as `T`,
  and two tests of `is` joined by `||`;
- a map literal against a union, as the expected result (`returns`) and as the
  subject of `as`;
- in the mangled mode, their broken forms: an operator after a program type, a
  `!` operand, a word that is no type, a key twice, an unclosed type, records
  with no tag. `-nof4` leaves both packages out.

And the forms of packages F6 and F7 (`spec/packages/f06-groups.md`,
`spec/packages/f07-queries.md`), from their records in
`suite/library-groups.json` and `suite/library-queries.json`:

- the groups `json`, `time`, `math` and `base64`, turned on by the case field
  `groups` in about one case in three: calls of their functions with valid and
  invalid text, and the constants `math.pi` and `math.e`. The correctly rounded
  functions of F13 are not generated. Loose cases call a group that is off, name
  a group that does not exist, and declare a variable named like a group that is
  on;
- the projection `xs[*]`, `xs[*].f` and `xs?.[*]`;
- the six methods `minBy`, `maxBy`, `uniqueBy`, `indexBy`, `countBy` and
  `mapValues`, and `sortBy` with a direction.

The generator reaches all of Core: every operator and precedence level, `const`
lines with patterns and defaults, lambdas and calls, `if`, `match`, blocks,
`try`, `catch` and `fail`, nested templates, list and map literals with spreads
and computed keys, chains with `?.` and indexes, `in` and `not in`, and calls of
every entry of `suite/library.json`. It reads the entries, their parameters,
callbacks and types from that file.

A share of cases gets a small step, unit or depth limit. At most one of the step
limit and the unit limit is small in one case, so that a finding points at one
count.

## Running it

From this directory:

```
GOWORK=off go run ./cmd/fuzz -seed 1 -cases 500 -rounds 40
```

This makes 40 case files of 500 cases each. It builds the Go and Rust `outcome`
programs once, then runs the built programs. TypeScript runs with `node
scripts/outcome.ts`. Each command runs in its own directory.

At the end it prints, for each mode and implementation, the share of cases that
gave a value, a run failure, a check failure, a parse failure and a declare
failure. Then it lists each distinct difference with its count, and the totals.
The status is 1 when there was any finding, else 0.

| Flag | Meaning |
| --- | --- |
| `-seed N` | The seed. The same seed and flags give the same cases, byte for byte |
| `-cases K`, `-rounds R` | Cases in a round, and rounds |
| `-impls go,ts,rust` | The implementations to compare |
| `-typed`, `-loose`, `-mangled` | The weight of each mode: 5, 3 and 2 by default |
| `-limits P` | The percent of cases with a small limit, 25 by default; -1 for none |
| `-nocheck P` | The percent of cases with `"check": false`, 20 by default; -1 for none |
| `-sound` | The soundness mode (below). `-cases` is then programs a round |
| `-nof3` | Leave out the string forms of package F3 |
| `-nof4` | Leave out the forms of packages F4 and F5 |
| `-nof6`, `-nof7` | Leave out the opt-in groups of F6, or the projection and methods of F7 |
| `-known=false` | Report the known differences (packages offered) as findings too |
| `-exclude a,b` | Library entries not to call |
| `-cmd name=command` | The `outcome` command of an implementation. The file is added at the end |
| `-build name=command` | A command run once before the first file |
| `-nobuild` | Build nothing. Go runs with `go run`; Rust runs the program built before |
| `-timeout D` | The time one command may take on one file, 60s by default |
| `-noshrink`, `-max N` | Do not shrink; write at most N findings for each distinct difference |
| `-noprobe` | Do not probe which library entries each implementation has |
| `-emit DIR` | Write the case files to DIR and run nothing |
| `-replay FILE` | Run one finding, or one case file, and print each line |
| `-case R:I` | Make and run only case I of round R |
| `-v` | Print each round, and the count of outcomes by code |

Before the first round the fuzzer runs the cases of `library.json` in each
implementation. An entry that one implementation reports it does not have
(`undefined-function`, or a method the receiver's kind lacks) is left out of
generation, and the fuzzer says so. A difference of that kind found later is
counted apart, not as a finding.

The implementations may offer different packages (`03-host.md` 3.11), and the
`outcome` command cannot turn a package off. A package gives some texts that
Core rejects a meaning, or a targeted parse code of its own. A difference that
comes only from that is counted apart, by name, so that anything new stands out.
Every implementation offers F1 to F8, and some offer more. The generator writes
only forms of F1 to F8, so no difference is filtered: `cmd/fuzz/known.go` is
where a form of a package that not every implementation offers would be counted
apart. With F8 a parse or check diagnostic carries a message, and a run
diagnostic a trace; each is compared among the implementations that write it
(`spec/packages/f08-errors.md` 8).

If a build fails and a program built before exists, the fuzzer says so and runs
the old program.

## The soundness mode

```
GOWORK=off go run ./cmd/fuzz -sound -seed 1 -cases 300 -rounds 40
```

Each program has an environment of declared types, made at random: the primitive
types, literal sets, `| null`, lists, records with optional fields, open
records, `map<T>`, `any` at the top and at depth, tagged unions (F5), named
types (F4), and an opaque type. For each program there are five value sets, each
of which conforms to the declared types (18.1, and for a union `f05-unions.md`
7.1). They favour the edges: null where the type allows it, optional keys
absent, the empty list, map and string, zero, `false`, the ends of the whole
range, extra keys on an open record. Host functions and methods are stubbed,
with results that conform to their result types; those that can fail are marked
`!`.

Half of the programs are careful: they use a value only as its type allows. The
other half are careless: they use an `any` value, a nullable value, an optional
field, one of several kinds or one variant of a union as if it were known. Both
write `x as T` (a `value` site), `x is T ? e : d` (a fact that narrows), and
tests of a union's tag (facts that narrow to variants). The checker decides
which programs check clean. A clean program is run on each value set, and the
fuzzer decides from the outcome line alone whether the promise holds:

| Outcome | Holds when |
| --- | --- |
| `null-operand`, `wrong-kind`, `missing-field` | At a `dynamic` site (promise 1) |
| `index-range`, `divide-by-zero` | At an `index` or a `divide` site (promise 2) |
| `invalid-argument`, `failed` | At a `value` site (promise 2) |
| `not-conforming` (a failed `as`) | At the `value` site of that `as` (promise 2) |
| `function-error` | At a `host` site (promise 2) |
| `number-range`, a limit, `function-panic` | Always (promise 3) |
| Any other run code | Never: the values conform, so nothing else can fail |
| A value | It conforms to the reported `type` (promise 4); for a union, by its tag first |

"At a site" is 18.3: a site of that class on the failing node, or on its
operand, receiver, index or argument; for a pattern, on its key or name or on
the value bound. The fuzzer knows the span of every node of the program it
printed, and compares spans: no two nodes of a Core program that could differ in
the answer share one span. It also reports a failure at a site whose `class` in
the line is not the site's class. A failure whose span is no node of the program
is counted as undecided.

At the end it prints, for each implementation, how many programs checked clean
(careful and careless apart), the sites of each class those programs recorded,
and the outcomes of their runs. Each violation is shrunk, with every value still
conforming to its type, and written to `findings/sound-<seed>-<n>.json`. A
violation that every implementation shares points at the chapter or at this
fuzzer; a violation in one implementation points at that implementation. The
summary keeps the two apart.

## Reading a finding

A finding is `findings/<seed>-<n>.json`. The directory is ignored by git.

| Field | Holds |
| --- | --- |
| `seed`, `round`, `index`, `mode` | Where the case came from |
| `difference` | For each pair of implementations, the first field where their lines differ, or nothing when they agree |
| `case` | The shrunk case, as a case of a case file |
| `lines` | Each implementation's line for the shrunk case, or `{"failure": ...}` |
| `original` | The case as it was generated |

The fields of a difference are `phase`, `value`, `diagnostics.count`,
`diagnostics.code`, `diagnostics.span`, `diagnostics.class`,
`diagnostics.message`, `steps` and `units`. A failure is `fail:status`,
`fail:timeout`, `fail:missing line` or `fail:bad-line`.

The one-line summary names the file, the mode, the start of the program, and
what each implementation gave: `value`, or the phase and the code.

## Replaying

To run a finding again in every implementation:

```
GOWORK=off go run ./cmd/fuzz -replay findings/7-3.json
```

To make one generated case again, give the seed and the mode weights of the
first run, and the round and index of the finding:

```
GOWORK=off go run ./cmd/fuzz -seed 7 -case 19:354
```

The library entries left out change what is generated. A replay by seed needs
the same `-exclude` list, and the same probe result: the fuzzer prints the
entries the probe left out.

## Tests

```
GOWORK=off go test ./...
```

The tests cover the printer (hand-made trees, 20,000 random trees read back by a
small parser written from the grammar, and the spans of nodes), the comparer on
canned lines, the reducer on a fake difference, that one seed gives one file,
type text read and printed and the conformance of values (18.1), that every
value set conforms, and the judge of the promise on canned lines.
