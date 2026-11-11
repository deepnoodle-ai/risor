# Risor v3 design: testing programs

Status: plan accepted by the owner, with every decision of section 8 as
recommended (D49 in `00-overview.md`). `02` to `06` are revised to match. Two
points of section 9 were ruled when the plan was applied: the field `expected`
of a case file is renamed `returns`, and a case has no tolerance field, so a
case that tests an implementation-defined `math` entry rounds in its own
program. Where this plan says `expected` for that field, read `returns`. A
header or a case also has a field `groups`, which enables opt-in library
groups such as `math` and `json`; this plan had none (`05:A.11`).

The owner decided in this round that tests are data, in the format of the
conformance files, and that the language gets no test construct. This plan works
that out. It assumes plan `08` (text blocks, raw strings), plan `10` (specified
messages, `data` and `trace` on a run diagnostic) and plan `12` (config files).
Where it depends on one, it says so.

**How to read this.** Section 2 is the proposal, written as rules so that it can
move into `05:A.11`. A paragraph headed *Why* is not a rule. Sections 3 to 9 are
not rules.

**Evidence.** **(R)** read in a sibling document at the place cited. **(I)**
inferred. **(C)** checked on 2026-11-09 against the project's own public
documentation or repository. A prior-art claim with no (C) says "(from memory)".
`02:` to `07:` are the sibling documents, and `risor:` is this repository.

## 1. Summary

1. **The problem.** v2 tested a script with a script: `test_` functions, a `t`
   object and `assert` (`risor:pkg/testing/runner.go:124-125`,
   `risor:pkg/testing/context.go:15-16`). v3 removes all three (`05:B.11`,
   `06:3.1`) (R). An author of a v3 program has nothing in their place.
2. **The proposal.** The case format of the conformance suite (`05:A.11`)
   becomes the author's format. A **case file** declares an environment once and
   lists cases. Each case gives inputs and what is expected.
3. **One runner, two bases.** `risor test` runs a file that declares its whole
   environment in data. A helper in each binding runs the same file against the
   host's real environment, inside the host language's own test runner.
4. **Host functions are stubbed with data:** a table from arguments to a result
   or a failure. It replaces the fixed functions every harness had to provide.
5. **Nothing is added to the language.** No node, no reserved word, no library
   entry. `assert` stays out.
6. **A file that declares everything in data gives the same verdicts in all
   three implementations.** That is the property the suite already needs, so the
   author's tests get it for nothing, step counts included.
7. **A user's case file may be a Risor program whose value is the data** (plan
   `12`). The suite's own files stay JSON.
8. **An update mode writes actual results into a JSON case file.**
9. **`risor conform` goes.** The suite is a directory of case files, and `risor
   test` runs it.

## 2. The proposal

### 2.1 What it looks like

```json
{
  "file": "discount.risor",
  "env": "{order: {total: float, coupon: string | null}, tier: \"gold\" | \"basic\"}",
  "functions": {
    "rate": {"type": "(code: string) => float !",
             "calls": [{"args": ["SAVE10"], "result": 0.1},
                       {"error": "unknown coupon"}]}
  },
  "expected": "float",
  "cases": [
    {"name": "gold with a coupon",
     "vars": {"order": {"total": 200, "coupon": "SAVE10"}, "tier": "gold"},
     "expect": {"value": 153, "steps": 14}},
    {"name": "an unknown coupon fails the run",
     "vars": {"order": {"total": 200, "coupon": "NOPE"}, "tier": "basic"},
     "expect": {"phase": "run",
                "diagnostics": [{"code": "function-error", "class": "host"}]}},
    {"name": "no coupon stays under 50 steps", "limits": {"steps": 50},
     "vars": {"order": {"total": 200, "coupon": null}, "tier": "basic"},
     "expect": {}},
    {"name": "a misspelled field is a check error",
     "source": "order.totl * 2",
     "expect": {"phase": "check",
                "diagnostics": [{"code": "no-field", "span": [1, 1, 1, 11]}]}}
  ]
}
```

The value, the counts and the span are illustrative.

### 2.2 The file

A case file is one JSON object: a **header**, and `cases`, a list. A JSON array
is a file with no header, so every file of `05:A.11` and `03:2.16`, and the
`cases` of an entry of `library.json` (`06:2.11`), stay valid (R).

**The rule of the header.** The header may hold any field of a case but `name`,
`skip` and `expect`. A case is the header with the case's own fields laid over
it. A field of the case replaces the header's field of that name, whole. Nothing
is merged. `source`, `file`, `template` and `conform` count as one field here.

| Field | Meaning | When absent |
| --- | --- | --- |
| `name` | The case's name. Unique in the file | Required |
| `source` | The program text, in place | Exactly one of these four is present |
| `file` | A path to the program text, relative to the directory of the case file, with `/` between its parts. Spans are positions in that text | |
| `template` | A template text (`05:A.7`) | |
| `conform` | A type and a value, with no program: a conformance case of the second kind (`05:A.11`). An author uses it to hold a sample payload against a named type | |
| `version` | The specification version (`05:A.10`) | The base's (2.6) |
| `env` | One record type in type text. Its fields are variables | No variable is added |
| `types`, `opaque`, `methods` | Named types, opaque type names and method signatures, as in `05:A.11` and `03:2.16` | None is added |
| `functions` | Host functions, stubbed (2.3) | None is added |
| `reject` | The whole set of rejected check classes (`05:A.2`) | The base's |
| `expected` | The expected result: a type text, or `condition` (`05:A.3`) | Nothing is expected |
| `vars` | A JSON object: a value for each variable the case supplies | No variable is supplied |
| `limits` | `steps`, `allocation` and `depth`. Each one given replaces the base's. Time cannot be given: a case must not depend on a clock | The base's |
| `precondition` | `false`: do not test the fixtures (2.5, step 5). For the suite's cases that break a declared type on purpose | `true` |
| `skip` | A reason. The case is not run, and its verdict is `skip` | The case is run |
| `expect` | What is expected (2.4) | The file is invalid, except in update mode (2.8) |

A field that this table does not have makes the file invalid. So `expcet` is
reported and never ignored, which is the rule of an exact record (D41).

### 2.3 Stubs

| Matter | Rule |
| --- | --- |
| Form | `functions` maps a name to a signature text, or to `{"type": signature, "calls": [row, ...]}`. A signature alone is a stub with no rows, which is enough for a case that is only checked (`03:2.16`) (R) |
| A row | `args`, a list of JSON values, and exactly one of `result`, a JSON value, and `error`, a string. Optional: `charge` and `allocate`, two whole numbers the stub passes to the call handle before it answers (`05:A.4`) |
| Matching | Rows are tried in order. A row with no `args` matches every call. A row with `args` matches when the call has that many arguments and each is `==` to the row's (`02:2.7`). The first row that matches answers |
| `result` | The stub returns the value |
| `error` | The stub returns an error with that text. The run sees `function-error`, class `host` (`05:A.4`), and the text is the cause |
| No row matches | The stub returns an error, and the case's verdict is `fail` with the difference `functions`, whatever the run then gives. A `try` in the program cannot hide it |
| Where it is declared | In the case's environment, a child of the base (2.6). A name the base declares is replaced in the child only (`05:A.2`) (R). With no `type`, the base's signature and type rule are kept and only the implementation is replaced |
| Cost | Matching is the runner's work and charges nothing. A stubbed call costs the 1 step of any call (`04:A.3`) plus its row's `charge`. So the counts of a case do not depend on the implementation |

*Why a table.* A stub is then a pure function that every implementation reads
the same way, and `risor test` needs no host code. It also removes a gap:
`05:A.11` gives each harness three fixed functions and does not say how a name
in `functions` picks one (R). `result`, `error` and `charge` are those three,
written as data. Each harness keeps its own cases for what data cannot say: a
function that panics, and one that blocks until it is cancelled.

### 2.4 What is expected

`expect` is a **result** or a **failure**. A failure has `phase`.

| Field of a result | Compared with | How |
| --- | --- | --- |
| `type` | The type the check gives | Both are parsed and compared by structure (`03:2.16`) |
| `sites` | The sites of the check, each a class and a span (`03:2.16`) | The whole list |
| `names` | The names analysis gives (`05:A.8`) | The whole list |
| `value` | The value of the run | With the language's `==` (`02:2.7`). So `2` equals `2.0`, and the order of keys does not matter |
| `truthy` | The truthiness test on the value (`05:A.3`) | `true` or `false`. For a host that checks with `condition` |
| `steps`, `allocated` | The usage of the run (`05:A.5`) | Equal |

| Field of a failure | Meaning |
| --- | --- |
| `phase` | `declare`, `parse`, `check` or `run` |
| `diagnostics` | The whole list, in the order the stage reports it (`03:2.13`). A run failure has one. In each: `code`, required, and any of `class`, `span`, `path`, `message`, `data` and `trace` |
| `steps`, `allocated` | For phase `run`: the usage of the failed run |

- **A field that is present is compared. A field that is absent is not.** So
  `"expect": {}` means: the case checks and gives a result, and nothing more is
  asked. The suite's own cases state everything; that is a rule of the suite,
  not of the format.
- **A failure has one shape.** `03:2.16` writes `{"phase", "diagnostics"}` and
  `05:A.11` writes `{"phase", "code", "span"}` (R). The first is the rule.
- **`span` is in the source text:** `[line, column, end line, end column]`,
  never a position in a host document (`05:A.6`).
- **`message` is compared whole,** once plan `10` specifies messages. For
  `function-error` it is compared with the stub's `error` text, and for `failed`
  with the text given to `fail`. `help` and `fix` are never compared. `data` is
  compared with `==`, and `trace` entry by entry (plan `10`).
- **A ceiling on steps or allocation is the case's `limits`.** A run that passes
  it fails with `steps-exceeded` or `allocation-exceeded`, where the case
  expected a result. No field such as `maxSteps` exists. *Why:* it is the
  mechanism a host uses, and the failure is the one the host would see. An exact
  count in `expect` changes whenever the program does, and for no other reason
  (D16), so an author who wants only a bound uses `limits`.

### 2.5 How a case is run

| Step | What happens | A failure here is |
| --- | --- | --- |
| 1. Read | The file is read, and each case is laid over the header (2.2) | The verdict `invalid` |
| 2. Declare | The case's environment is built: a child of the base (2.6) | The outcome, phase `declare` |
| 3. Parse | The program is parsed | The outcome, phase `parse` |
| 4. Check | The program is checked, with `expected` | The outcome, phase `check`. Otherwise the check gives `type` and `sites` |
| 5. Test the fixtures | Only when the case will run, and `precondition` is not `false`. Each value of `vars` is tested against its declared type, and each `result` of a stub against its signature's result type when that has no type variable, with the function of `05:A.13` | The verdict `invalid`, with the path. The fixture is wrong, not the program |
| 6. Run | Only when the case has `vars`, or `expect` names what only a run gives: `value`, `truthy`, `steps`, `allocated`, or a failure of phase `run`. So a case with neither is only checked (`03:2.16`) | The outcome, phase `run`, with its usage |
| 7. Compare | The outcome is compared with `expect` (2.4) | The verdict `fail` |

**The verdict** of a case is `pass`, `skip`, `invalid`, or `fail` with the list
of what differed. The names in that list are `outcome` (a result where a failure
was expected, the reverse, or another phase), then the fields of 2.4, with
`diagnostics` as one name, then `functions` (2.3). They stand in that order. A
`conform` case has no stages: it is run as `05:A.11` says.

**Also `invalid`:** text that is not JSON or repeats a key, a field 2.2 does not
have, a name used twice, a `file` that cannot be read, a key of `vars` that no
declaration names, a stub row with both or neither of `result` and `error`, and
a stub with no `type` for a name the base does not declare.

*Why step 5.* A fixture that does not fit its declared type gives a run error of
class `environment` where it is read, and a passing case where it is not
(`05:A.3`) (R). Either way the test says nothing about the program. `05:A.13`
exists so that a host can test this promise, and a test run is where it is
cheap.

**What the conformance suite tests.** A new kind of case holds a case file and
the verdicts it must give:

```json
{"name": "runner: nothing is merged, and a difference is named",
 "runner": {"vars": {"a": 4, "b": 2}, "env": "{a: int, b: int}",
            "cases": [{"name": "replaced", "source": "a", "vars": {"b": 1},
                       "expect": {"phase": "run",
                                  "diagnostics": [{"code": "missing-variable"}]}},
                      {"name": "wrong", "source": "a / b", "expect": {"value": 3}}]},
 "expect": {"verdicts": [{"name": "replaced", "status": "pass"},
                         {"name": "wrong", "status": "fail", "differs": ["value"]}]}}
```

These cases cover each row of 2.2 to 2.5: the header rule, each way a file is
invalid, stub matching and its cost, an unmatched call inside a `try`, the
fixture test, which stages run, and the text an update writes (2.8). Every
binding's helper runs them, so the three runners agree as the three evaluators
do.

### 2.6 Two entry points

A case runs in a child of a **base environment**. The two entry points differ
only in the base.

| | `risor test` | The helper of a binding |
| --- | --- | --- |
| Base | The default environment: the newest version, `core` on, the default limits, no variable | The environment the host passes: its real one |
| Host functions | Stubs only | The host's own, except those the file stubs |
| Variables | `vars` only | `vars`, then the host's variables for a name the case does not give. That is how an opaque value reaches a case |
| `file` | A path | A path, or whatever the host's reader accepts, such as a place in its own document, with the origin of `05:A.6` |
| The same in every implementation | Yes | As far as the host's functions and values are |
| Reports to | Standard output and the exit status | The host language's test runner |

```
risor test [path ...] [--run regexp] [--update] [--format text|json]
```

A path that is a file is run. A directory is searched for `*.test.json` and
`*.test.risor`. The exit status is 0 when every case passed or was skipped, 1
when one failed, and 2 when a file is invalid. `--format json` writes each
verdict as data.

```go
package risortest // github.com/deepnoodle-ai/risor/go/v3/risortest

// Run runs each case of the file at path as a subtest of t, named by the
// case, in a child of base. A nil base is the default environment.
func Run(t *testing.T, base *risor.Env, path string, opts ...Option)

// Load and Case.Run are Run without *testing.T, for another runner.
func Load(path string, opts ...Option) (*File, error) // an error: invalid
func (f *File) Cases() []*Case
func (c *Case) Run(ctx context.Context, base *risor.Env) Verdict

// A Verdict has Status (Pass, Fail, Invalid, Skip), Differs (each a Field,
// what was wanted and what was got) and Actual, the expect of 2.8.
func Vars(v risor.Vars) Option // variables a case does not give
func Sources(read func(ref string) (string, Origin, error)) Option
```

```ts
// "risor/testing". It imports no runner: the host passes its own `test`.
export function defineCases(
  test: (name: string, fn: () => void) => void,  // vitest, jest, node:test
  base: Env | undefined, file: CaseFile): void;
// loadCaseFile reads a path. caseFile takes parsed data, as in a browser.
export function loadCaseFile(path: string, options?: CaseOptions): CaseFile;
export function caseFile(data: unknown, options?: CaseOptions): CaseFile;
export interface CaseFile { readonly cases: readonly Case[] }
export interface Case { readonly name: string; run(base?: Env): Verdict }
// Verdict and CaseOptions hold what Go's Verdict, Vars and Sources do.
```

```rust
// risor::testing
pub fn load(path: impl AsRef<Path>) -> Result<CaseFile, Error>;
impl CaseFile {
    pub fn cases(&self) -> &[Case];
    pub fn run(&self, base: &Env, vars: &dyn Vars) -> Report; // every case
}
impl Case   { pub fn run(&self, base: &Env, vars: &dyn Vars) -> Verdict; }
impl Report { pub fn assert_ok(&self); } // panics, and lists the cases that failed

#[test]
fn discount() {
    let file = risor::testing::load("tests/discount.test.json").unwrap();
    file.run(&env(), &NoVars).assert_ok();
}
```

- **Go** gives one subtest for each case, so `go test -run` selects a case.
- **TypeScript** names no runner: every common one has `test(name, fn)` (from
  memory), and the core package keeps no dependency.
- **Rust** gives one `#[test]` for each file, because the standard harness
  cannot add a test at run time (from memory). The report names the cases.
- **The helper is the harness of `05:A.11`, made public.** Each implementation
  runs the suite through it, so it is tested by every case the suite has.

### 2.7 A case file as a program

| Question | Answer |
| --- | --- |
| May a case file be a Risor program? | A user's file, yes: `*.test.risor`. Its value must be the object of 2.2. The format is the value, not the syntax |
| How is it loaded? | As plan `12` loads a config file: parsed, checked against the type of a case file as the expected result, and run in the default environment. The value is then read as the JSON would be |
| May the suite's files be programs? | No. A file that tests the parser cannot need the parser, and `05:A.11` promises that no implementation needs a second one (R) |
| What does it give? | Comments. A raw string or a text block for `source`, so a quote needs no `\"` (plan `08`). `const` and spread for a fixture that cases share, which is the answer to "nothing is merged". Cases built with `map`. A wrong field is a check error with a position |
| What does it cost? | No update mode (2.8). And the test's data now passes through the evaluator under test |
| Is a JSON case file a program too? | Yes: JSON with unique keys is valid program text with the same value (`02:3.3`) (R). So a file can be renamed and then edited |
| May a stub be a lambda? | No. A file's value is data, and no function reaches a host (the owner's decision 1 of this round, D34). A stub stays a table |

```ts
// discount.test.risor
const base = {order: {total: 200, coupon: null}, tier: "basic"};
{
  file: "discount.risor",
  env: '{order: {total: float, coupon: string | null}, tier: "gold" | "basic"}',
  cases: [
    {name: "no coupon", vars: base, expect: {value: 200}},
    // Gold takes 15% off.
    {name: "gold", vars: {...base, tier: "gold"}, expect: {value: 170}},
  ],
}
```

The specification carries the type of a case file as type text, so that the
loader of plan `12`, an editor and a reader have one definition.

### 2.8 Update mode

| Matter | Rule |
| --- | --- |
| Asked for by | `risor test --update`. In a helper, the environment variable `RISOR_UPDATE=1`. Never on by default, and never implied by a missing `expect`: in an ordinary run that file is invalid |
| A case with no `expect` | It goes through every stage, and gets the whole outcome. A result: `type`, `value`, `steps`, `allocated`. A failure: `phase`, and for each diagnostic `code`, `class`, `span` and `path` |
| A case whose `expect` has the outcome's shape | Each field present is replaced with the actual. A field that is absent stays absent. So an update never makes a case stricter: an author who compares only `value` chose not to pin the count |
| A case whose `expect` has the other shape | As a case with no `expect` |
| The text written | The JSON form of `02:2.13`: compact, keys in code point order. Only the text of that `expect` member changes. Every other byte of the file stays |
| Not written | A file that is invalid, a case that is skipped, a case that failed on `functions`, and a file that is a program: the tool says to run it on a JSON file |
| Who provides it | `risor test` and the Go helper in 3.0. The TypeScript and Rust helpers may follow |

### 2.9 `risor conform`

| Question | Answer |
| --- | --- |
| What does `risor conform` become? | Nothing. The suite is a directory of case files, and `risor test <directory>` runs it |
| The CLI of `05:B.11` | `eval`, `check`, `fmt`, `test` and the REPL: still four commands |
| The kinds of case an author never writes: `join`, `fits`, `print`, `schema`, the limit cases, `runner` (`03:2.16`, 2.5) | The same runner runs them. They are documented with the suite, not with `risor test` |
| The public conformance report (`00:5.4`) | The output of `--format json` over the suite |

## 3. What is not in this plan

| Left out | Why | When |
| --- | --- | --- |
| A test construct, `assert`, or a `test` word | The owner's decision. `cond ? x : fail(msg)` stays the guard inside a program (`06:3.1`) | Not planned |
| A report of the branches and `match` arms that no case ran | It needs an evaluator that records the nodes it visits, the same in three implementations | Later |
| Inputs generated from the declared types, to find run errors | `03` already generates values from types for its own fuzzing (R). Offering it to authors needs a specified generator and a way to report the input found | Later |
| Merging a case into the header by key | A second rule. A program file has spread (2.7) | Later, if JSON authors ask |
| Expecting that a function was called, or how often. A stub that answers differently on a second call | A stub is a pure table. A host that needs more writes a function and uses the helper | Later |
| An environment description as the base of `risor test` (`05:A.2`) | It would let the CLI check against a host's real declarations with no host code. Nothing blocks it | A minor version |
| Update for a program file, a watch mode, parallel runs | Tools, not format | Later |
| A tolerance for `value` | Section 9 | Open |

## 4. Changes to the other documents

| Document | Change |
| --- | --- |
| `00` | P4: the suite's format is also the author's. 5.1: the tools are `eval`, `check`, `fmt`, `test`. 5.3: what replaces v2's script test runner. The decisions of section 8 |
| `02` | No change to the grammar: 31 node kinds, no new reserved word. 3.2: the hint for `assert(` names `fail` for a guard and a case file for a test |
| `03` | 2.16: the header, `precondition: false` on the cases that break a declared type, and `runner` among the kinds of case. Its form of a failed `expect` is already the rule. 28 check codes, unchanged |
| `04` | A.3: a stubbed call costs the step of a call plus its row's `charge`, and matching costs nothing |
| `05` | A.11 is rewritten from section 2: the file, the header, stubs in place of the fixed functions, the one shape of a failure, verdicts. A.6: `message` joins what a case may compare (plan `10`). B.11: `test` in place of `conform`. The helpers in Parts B to D. Section 8: the runner of step 1 is public |
| `06` | `assert` stays dropped. 2.11: the `cases` of an entry are a file with no header. 2.8: how a case states a tolerance (section 9) |
| Conformance suite | Every file written as an array stays valid. Cases that used a fixed function get rows. A new file of `runner` cases (2.5) |

## 5. Size and order of work (I)

| Part | Go, lines without tests |
| --- | --- |
| Reading a file, the header rule, the invalid cases | 200 |
| Stubs | 100 |
| Stages, comparison and verdicts | 200 |
| Update, with a JSON scanner that keeps offsets | 120 |
| `risortest` | 80 |
| The `test` command | 150 |
| Total | about 850. About 450 of it is the runner every implementation needed anyway (`05`, section 8) |

The TypeScript and Rust helpers should be about 600 lines each, with no command
and no update. None of this was built.

**Order.** The file, the stubs and the one shape of a failure are fixed in
`05:A.11` before the first suite case is written: that is step 2 of `00:5.2`.
Each implementation's runner is then built with its evaluator, as its harness.
Fixture tests need `Conform` (`07`, step 2). Program files wait for plan `12`,
and `message`, `data` and `trace` for plan `10`.

## 6. Prior art

| Project | What it does | What I take from it |
| --- | --- | --- |
| JMESPath compliance tests | JSON: `given` once, then `cases`, each an `expression` and a `result` or an `error` (C; `01:3.1`) | A header and a list of cases |
| CEL conformance tests | Data files: a test has `expr`, `bindings`, a `value` or an `eval_error`, and `check_only` and `disable_check` (C) | A case states which stages matter. Here `expect` says it, with no flag |
| Prometheus `promtool test rules` | Tests for rules of an expression language are YAML: input series, an `expr`, expected samples (C) | Tests as data are ordinary for a language embedded in configuration |
| OPA `opa test` | Tests are rules named `test_` written in Rego. `with` replaces data and functions for one expression (C) | The other road: tests in the language. It needs `with`, which is mocking as syntax |
| Terraform `terraform test` | Test files in the configuration language, with `mock_provider` and values given as data (C) | Stubs as data. Tests in the language's own syntax, as 2.7 allows |
| Go's golden files | `gofmt`'s own test has an `-update` flag that rewrites its `.golden` files (C) | Update is asked for, never implied |
| expect-test (Rust) | `UPDATE_EXPECT=1` patches the expectation in the source file (C) | An environment variable for the helpers, and a rewrite in place |
| Jest snapshots | `--updateSnapshot`. A snapshot is not written in CI without it (C) | A missing `expect` fails an ordinary run |

## 7. Alternatives rejected

| Alternative | Why not |
| --- | --- |
| A test construct in the language | Decided by the owner. It is a statement run for its effect, three times over, in a language of expressions |
| A friendlier format for authors, apart from the suite's | Two formats and two runners. The suite's format is only as good as its use, and authors are its second user |
| YAML case files | A second parser in every implementation (`05:A.11`), and YAML's own surprises. A program file gives comments and multi-line text (2.7) |
| Tests only in the host language, as table tests around `Eval` | Each host writes its own, none is portable, and a browser editor cannot run the server's tests |
| Stubs as host code only | `risor test` then runs nothing with a function, and the suite keeps its fixed functions |
| `expect` compared whole, absent fields included | Every case would pin its step count, and every edit of a program would break every case |
| `maxSteps` and `maxAllocated` in `expect` | A second mechanism beside `limits`, with a different failure |
| Expectations in a snapshot file beside the case file | The reader of a case cannot see what it expects |
| An unmatched stub call as a plain `function-error` | A `try` in the program would catch it, and the case would pass on a path nobody wrote |
| `risor conform` as a second command over the same runner | Two names for one thing |

## 8. Decisions for the owner

| # | Decision | Recommendation | Strongest alternative |
| --- | --- | --- | --- |
| 11-1 | Is the suite's case format the author's format, with a header? | Yes. One object: header fields, then `cases`. An array is a file with no header | Keep the array, and repeat `env` in every case |
| 11-2 | How does a case combine with the header? | A case's field replaces the header's, whole | Merge `vars` and `functions` by name. More convenient in JSON, and a second rule |
| 11-3 | How is a host function stubbed? | A table of rows in data. An unmatched call fails the case | The fixed functions of `05:A.11`, with real functions only through a helper |
| 11-4 | What does `expect` compare? | The fields present. One shape for a failure. An exact count in `expect`, a ceiling in `limits` | `maxSteps` in `expect`, which reports the actual count when it fails |
| 11-5 | Are fixtures tested against their declared types? | Yes, with `05:A.13`, unless the case says `precondition: false` | No test. One field fewer, and a wrong fixture passes silently |
| 11-6 | The entry points | `risor test` on the default base, and `risortest.Run`, `defineCases` and `risor::testing` on the host's | The helper only, with no command |
| 11-7 | May a case file be a Risor program? | A user's file, yes, once plan `12` lands. The suite's, never | JSON only. Then authors escape every quote and have no comments |
| 11-8 | Update mode | Yes: on request only, `expect` only, JSON only, never stricter | Rewrite the whole file in a canonical layout: simpler, and a noisy first diff |
| 11-9 | `risor conform` | Remove it. `risor test` runs the suite | Keep it as a name for running the suite a binary carries |

## 9. Open questions

- **A tolerance for `value`.** `06:2.8` says the suite compares the results of
  the implementation-defined `math` entries to 11 or 12 significant digits (R).
  No field of a case says so. A case can round in its own program. A `tolerance`
  field is the other way.
- **`expect` and `expected` differ by two letters** and mean different things.
  Authors now write both. I would rename `expected` to `returns` before any file
  exists.
- **`message` waits on plan `10`.** Until messages are specified, a `message` in
  `expect` makes the file invalid, so that no case passes in one implementation
  only.
- **Should update mode be required of the TypeScript and Rust helpers for 3.0?**
  I say no: it is a tool, and D27 is about the language.
