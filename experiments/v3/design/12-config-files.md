# Risor v3 design: configuration files

Status: plan accepted by the owner, with every decision of section 2 as
recommended (D50 in `00-overview.md`). The owner chose two by name: a config
file is data only, and imports come in a minor version after 3.0 (12-8), with
`import` reserved now. `02` to `06` are revised to match, without section 3.9.
No code was written and nothing was measured. Plans `08` (text blocks), `09` (tagged unions)
and `10` (trace, messages, every failure of a conformance test) are assumed as
the round's brief states them. Decision numbers such as D41 and R5 refer to
`00-overview.md`. This plan's own decisions are `12-1` to `12-9` (section 2).

**How to read this.** Section 3 is the proposal, written as rules so that it can
move into the specification. A paragraph headed *Why* is not a rule. The other
sections are not rules.

**Evidence.** **(R)** read in a sibling document at the place cited. **(I)**
inferred. **(ran)** run with TypeScript 5.9 in a throwaway directory. **(W)**
read on the web for this draft, at the source the last section lists. A claim
marked "from memory" was not checked.

## 1. Summary

1. **A config file is a program:** `const` lines, then one expression (`02:2.1`)
   (R). Its value is the configuration. There is no second grammar and no data
   mode.
2. **Data only.** The value is null, a bool, a number, a string, a list or a
   map. A function is not a value (`02:2.9`) (R), so nothing in a config file
   reaches the host to be evaluated later (section 4).
3. **Most of it exists:** comments, trailing commas, `const`, spread, `if`,
   templates, exact records that refuse an unknown key (D41), a type the host
   passes as the expected result (`05:A.3`), limits, and text blocks and tagged
   unions from plans `08` and `09`.
4. **Loading is five steps in a fixed order:** parse, check against the host's
   type, run, conform, decode into the host's own type. The first four exist.
   `05` lacks three small pieces: the decode, the JSON text of a result, and
   optional fields in a type taken from a host type.
5. **A wrong field must be reported at the field.** Today `result-mismatch` has
   the span of the result expression (`03:2.13`) (R), which in a config file is
   the whole file. 3.3 moves the report into list and map literals.
6. **Settings from outside are variables** the host declares and supplies. No
   library entry reads the process environment, so a load stays a function of
   its files and its variables.
7. **`import "path"`** is an expression whose value is the value of another
   file's program. The host supplies the resolver, and there is none by default.
   Files share the environment and no names. A helper function cannot be shared
   between files.
8. **Timing.** In 3.0: everything but imports, with the word `import` reserved.
   Imports in a minor version.

## 2. Decisions

| # | Decision | Recommendation | Strongest alternative |
| --- | --- | --- | --- |
| 12-1 | Is a config file the whole language? | Yes. A host that wants less already has two lower levels: the `Literal` analysis accepts a file that is one literal, with no run, and an environment without the `core` group has no library (`05:A.8`, `05:B.2`) (R). | A data mode a host can require: no lambdas and no calls. It is a second language to specify and to test three times. |
| 12-2 | Is `result-mismatch` reported inside a list or map literal? | Yes, by the table of 3.3. It moves a span and changes no type. One rule serves this plan and plan `09`. | Leave the span and rely on the path of `not-conforming` after the run. It needs no checker change, but it reports a place in the data and not in the text, and only after a run. |
| 12-3 | Does a load always conform the value before it decodes? | Yes. A result that came through `any` passes the check and may not conform (`03:2.3`) (R). One walk makes the decode's precondition a tested fact. | A loader's environment rejects the class `dynamic`, and the walk is skipped. Then every use of `json.parse` in a config file needs an `as`. |
| 12-4 | What does the decode do? | What the host's usual JSON decoder does with the JSON text of the value, with unknown fields refused. An implementation need not make the text. | No decode in 3.0: a binding gives the JSON text and the host calls its own decoder. It is the same result in two calls. |
| 12-5 | Is a type from a JSON Schema closed for a config file? | Yes: a loader sets the option *closed* (`03:2.15`). Otherwise a misspelled key passes, because most schemas do not say `additionalProperties: false`. | Follow the schema, as a named type used with `as` does (`07:2.9`). |
| 12-6 | How does a setting from outside get in? | As a declared variable. The library reads nothing (`06:2.6`) (R). | An opt-in entry `env.get(name)` backed by the host. It is a variable with a second mechanism, which `06:2.6` already rejected for the clock. |
| 12-7 | The form of an import | `import "path"`: an expression, a literal path, the same environment, no shared names, evaluated as written. | Evaluated once for each load and shared. It needs a table for each run, and the cost of a file then falls on whichever import runs first. |
| 12-8 | Timing of imports | A minor version. Reserve `import` in 3.0. A host layers files today by passing one file's value to the next as a variable (3.9). | In 3.0. Composition across files is the first thing a YAML author asks a config language for. The cost is one more feature that three implementations must finish before 3.0 (D27). |
| 12-9 | Does the specification fix how a path names a file? | Yes, for one **standard resolver** that every binding ships: relative paths only, written `./` or `../`, with the file's full name. A host may supply any other. | Leave every path rule to the host. Then a set of config files does not move between hosts, and the suite cannot test imports. |

## 3. The proposal

### 3.1 What it looks like

```ts
// app.risor
const mb = 1024 * 1024;
const backend = (name, port) => {name, url: `http://${name}.internal:${port}`};

{
  listen: `0.0.0.0:${env?.PORT ?? "8080"}`,   // a setting from outside (3.6)
  max_body: 8 * mb,
  backends: [backend("a", 9001), backend("b", 9002)],
  auth: {kind: "token", header: "X-Token"},   // one variant (plan 09)
  banner: """
    Maintenance on Sunday,
    02:00 to 03:00 UTC.
    """
}
```

The host declares the type, in type text, from a JSON Schema or from its own
type:

```ts
{listen: string, max_body?: int, backends: {name: string, url: string}[],
 auth: {kind: "none"} | {kind: "token", header: string}, banner?: string}
```

An author who writes `max_bod` on line 7 gets one check error at that key,
`7:3-7:10`, before anything runs. Plan `10` owns the layout and the message.

### 3.2 What a config file is

| Question | Answer |
| --- | --- |
| What text? | A program of `02:2.2`. Nothing is added and nothing is taken away. |
| What value? | Data: the six kinds of `02:2.4` that are not opaque. An opaque value in a result is refused by the decode and by the JSON text. |
| What does `const` give? | A name for a repeated value, and a helper for a repeated shape. It does the work of a YAML anchor, with a type. |
| Which names can it read? | Its own `const` names, the library, and what the host declares: variables for settings (3.6), functions, named types. Nothing by default. |
| Is JSON a config file? | Yes, with four exceptions, each an error and never a second meaning: a key written twice (`duplicate-key`), an integer outside ±(2^53 − 1) written with no fraction (`02:2.5`), an escape of a lone surrogate (`02:2.2`), and nesting past 100 levels (`02:2.16`) (R). |
| What does it cost? | By `04:A.3` and `04:A.4` (R): a literal costs 0 steps; a list of `n` items 1 + `n` units; a map of `n` entries 1 + 2`n` units; a template 1 + its size, in both. The default budgets admit a file of about 500,000 map entries (I). |
| What bounds it? | The four limits of `05:A.5`, and 100 levels of nesting. |

### 3.3 Where a wrong field is reported

Let E be the result expression, S its type by `03:2.5` and T the expected type.
S is found as today. When Fits(S, T) is false, the checker reports by this
table, starting at (E, T). Every report is `result-mismatch`.

| Node | Reports |
| --- | --- |
| A `Map` whose entries all have a word, a string or a shorthand name as key, when T has a record alternative R | For each entry in source order, with key `k` and value `v`: if R lists `k` with type F and the type of `v` does not fit F, apply this table to (`v`, F). If R does not list `k` and R's rest is `never`, the span of the key. If R does not list `k` and the type of `v` does not fit R's rest, apply this table to (`v`, the rest). Then, if a required field of R has no entry, one report whose span is the `{` that opens the map. |
| A `List`, when T has a list alternative `U[]` | For each item in source order: a plain item whose type does not fit U, apply this table to (item, U); a spread whose element type does not fit U, the span of the `Spread`. |
| Any other node; a `Map` with a spread or a computed key; no such alternative; or a row above that reported nothing | The span of the node. |

- **No type changes.** T decides only where a diagnostic that already exists is
  placed. This is not "expected types flowing into list and map literals", which
  `03` section 8 rejects (R).
- **With a tagged union,** R is the one alternative that plan `09` picks by the
  tag. `09` owns that choice and what is reported when no alternative has the
  tag. `03` should hold the two as one rule.
- **Not followed:** a `const` name, a call, an import. The report is on that
  node. Following them is later.
- **Plan `09` states this rule in a wider form, and `09:2.6` governs where the
  two differ.** There the fit also goes through the branches of an `if`, the
  arms of a `match` and the result of a block, it applies to an argument as
  well as to the result, and a required field with no entry is reported on the
  whole map and not on its `{`. `03` holds one rule, `09`'s.
- The reports are a set, ordered as `03:2.13` orders diagnostics.

### 3.4 Loading

| Step | Stage (`05:A.1`) | Fails with |
| --- | --- | --- |
| 1. Parse the file, with its name as the origin | Parse | `parse` diagnostics |
| 2. Check, with the host's type T as the expected result | Check, Compile | `check` diagnostics; a wrong, unknown or missing field by 3.3 |
| 3. Run, with the settings as variables | Run | one `run` diagnostic, with the trace of plan `10` |
| 4. Conform the value to T | The conformance function (`05:A.13`), in the form of plan `10` that reports every failure | `not-conforming`, each with its path |
| 5. Decode into the host's type | The binding | `not-conforming` with a path: a number too wide for the field, or text the field's type refuses |

- **Step 4 is not a run** and charges nothing (`05:A.13`). It is always made
  (12-3).
- **Step 5 copies.** A result may share memory with the inputs (`05:A.3`). What
  the decode gives does not.
- **Nothing is filled in** (D39). A default comes from the host, which fills its
  own value before step 5; or from the file, as `{...defaults, port: 1}` over a
  variable the host supplies; or from `??` where the value is used.

What `05` has for a value going out, and what is missing:

| Need | `05` has | Missing |
| --- | --- | --- |
| The value | `Run` gives the host's plain forms of the six kinds, with `AsInt` and `AsFloat` (B.3). Rust's output has `to()` and `into_owned()`, named with no rule (Part D) | The rule for `to()`. `Decode` in Go |
| The value as JSON text | The rule: the JSON form of `02:2.13`, which `json.stringify` gives inside a program (`06`). The build for other hosts assumes it (Part D) | A function of each binding that gives that text for a result. It is not a run, and it fails where `json.stringify` fails |
| A type from a host type | `TypeOf[T]` in Go (B.7); the derive in Rust (`03:6.2`). Both make every field required | Optional fields (3.5) |
| A type from a JSON Schema | `TypeFromSchema` and three options (`03:2.15`) | A place for the options in the TypeScript and Rust sketches |
| The type as the expected result | `Expect(t)` (A.3) | The span of 3.3 |
| The file's name in a diagnostic | `Origin` (A.6) | With imports, a file for each diagnostic (3.9) |

### 3.5 The type, and the three bindings

| The type comes from | Rule |
| --- | --- |
| Type text | As written. A record with no `...` is exact (D41), which is what a config file wants: a misspelled key is an error at the key. |
| A JSON Schema | The table of `03:2.15`, with the option *closed* (12-5), and without *absent-is-null* and *defaults-applied*: nobody normalized the value. Value constraints are dropped (D43). A host that wants them runs its validator on the JSON text. |
| A Go type | `TypeOf[T]` (`03` section 5), with two more rows. A field whose `json` tag has `omitempty` or `omitzero` is optional. A type that implements `encoding.TextUnmarshaler` and has no registered method is `string`, so a `time.Time` field takes RFC 3339 text. |
| A Rust type | The derive (`03:6.2`), with one more row: a field marked `#[serde(default)]` is optional (I). |
| A TypeScript type | Nothing: its types are erased (`05`, Part C). The host gives type text or a schema. `load<T>` is a claim that steps 2 and 4 make true. |

```go
// 6. Load a config file into a Go struct. Host case 6 of 05:B.8.
type Config struct {
	Listen   string    `json:"listen"`
	MaxBody  int       `json:"max_body,omitempty"` // optional in the file
	Backends []Backend `json:"backends"`
}
env, err := risor.NewEnv(risor.Declare(`{env: map<string>}`))
x, err := risor.Parse(src, risor.At(risor.Origin{Name: "app.risor"}))
cfg := Config{MaxBody: 1 << 20} // the host's defaults
err = risor.Load(ctx, env, x, risor.Map{"env": picked}, &cfg)

// Load is steps 2 to 5 with T's type: Compile with Expect(TypeOf[T]()), Run,
// Conform, Decode. A field the value lacks keeps what dst held.
func Load[T any](ctx context.Context, env *Env, x *Expr, vars Vars, dst *T) error
func Decode(v any, dst any) error // dst as for json.Unmarshal
func JSON(v any) ([]byte, error)  // the JSON form of 02:2.13
```

```ts
const env = createEnv({ vars: "{env: map<string>}", types: { Config: schema } });
const x = parse(src, { name: "app.risor" });
const cfg = load<Config>(env, x, "Config", { env: picked });

export function load<T>(env: Env, x: Expr, expected: TypeText, vars?: Vars): T;
/** Not JSON.stringify: keys by code point, numbers by 02:2.5. */
export function toJSON(value: unknown): string;
```

```rust
#[derive(serde::Deserialize, risor::Host)]
struct Config {
    listen: String,
    #[serde(default)] max_body: u32, // optional in the file
    backends: Vec<Backend>,
}
let cfg: Config = env.load(&parse(src)?, &vars)?;

impl Env {
    pub fn load<T>(&self, x: &Expr, vars: &impl Vars) -> Result<T, Error>
    where T: DeserializeOwned + Typed;
}
impl Output<'_> {
    pub fn to<T: DeserializeOwned>(&self) -> Result<T, Error>; // step 5
    pub fn to_json(&self) -> Result<String, Error>;
}
```

`Typed` is the trait the derive implements to give a struct's type text (I).

### 3.6 Settings from outside

| Matter | Rule |
| --- | --- |
| How | A variable that the host declares with a type and supplies to the run (`05:A.3`), as it does for the current time (`06:2.6`) (R). For example `env: map<string>`, or `env: {REGION: string, DEBUG?: string}`. |
| What the library reads | Nothing. No entry reads the process environment, a file or a clock (`05:A.9`). This plan adds none. |
| Determinism | The same files, version and variables give the same value or the same error, and the same usage (`05:A.9`). A host that records the variables it supplied can repeat a load. |
| Reading one | With the exact record, `env.REGION`, and a misspelled name is `no-field`. With `map<string>`, `env?.REGION ?? "us-east-1"` is a `string` and cannot fail (`03:2.7`) (R). |
| Which names | The host's choice. A host that passes the whole process environment makes every variable of the machine an input of the result. It should pass the names it documents. |
| Text | A process variable is text. `int(env.PORT)` converts and may fail. The text `"false"` is truthy, so write `env.DEBUG == "true"`. `01:2.6` has this mistake in another language (R). |

### 3.7 The CLI

| Command | Does |
| --- | --- |
| `risor eval FILE --format json` | Steps 1 and 3, and 2 and 4 with `--type`. Prints the JSON text of the value and a line feed. `--format text`, the default, prints the rendering of `02:2.13`. `--pretty` indents and is not specified. There is no YAML or TOML output: pipe the JSON to a converter. |
| `risor check FILE --type TEXT` | Steps 1 and 2. `--schema FILE` gives a JSON Schema in place of the text, with the option *closed*. `--description FILE` loads an environment description (`05:A.2`), and `--type` may then name one of its types. |
| `risor fmt FILE` | Rewrites the text. It keeps comments, the order of keys and the written form of every literal. It does not sort keys. |
| `--var NAME=TEXT`, `--vars FILE.json`, `--env NAME` | A variable that holds the text. One variable for each key of a JSON object. The process variable `NAME` as a key of the variable `env`, declared `map<string>`; one that is not set is absent. The CLI reads no process variable that a flag does not name. |
| `--root DIR` | With imports: the root of the standard resolver (3.9). The default is the directory of FILE. |
| Exit status | 0 with a value. 1 with diagnostics, in the layout of plan `10`, as JSON under `--format json`. |

### 3.8 What a YAML or JSON author meets

| Coming from YAML or JSON | Here | The answer |
| --- | --- | --- |
| Keys come out in the order written | A map has no order. JSON text is in key order by code point (T2) | `fmt` keeps the order in the source. Data whose order matters is a list. |
| 64-bit integers | An integer literal outside ±(2^53 − 1) is a compile error (`02:2.5`) | Write the identifier as a string (D3). |
| Dates and times as values | None (D26) | RFC 3339 text. The host's type decodes it (3.5), or `time.parse` gives seconds. |
| Anchors, aliases and merge keys | None | `const` and spread: `{...base, port: 9000}`. |
| A deep merge of two documents | Spread is one level deep | `{...base, db: {...base.db, port: 1}}`. A library `merge` is later (`06`). |
| A key written twice: the last one wins | `duplicate-key` | Remove one. |
| Bare strings, and `yes`, `no`, `on`, `off` as booleans | A bare word is a name, and an unknown one is `undefined-name` | Strings are quoted. `true` and `false` are the booleans. |
| A bare key such as `max-retries` | A bare key is a `Word` (`02:2.2`) | Quote it: `"max-retries": 3`. A targeted error says so. |
| `#` comments | `//` and `/* */` | A targeted error says so. |
| Block scalars, `\|` and `>` | A text block (plan `08`). No folding form | Join lines with a template, or write one long line. |
| `~`, an empty value | `null` only. An absent key is not a null one (D42) | Leave the key out, or write `null` where the type allows it. |
| `1.0` stays `1.0` | One kind of number: `1.0` is `1` and prints as `1` (N1) | A host field typed as a float decodes it. |
| Several documents in a file, `---` | One expression | A list. |
| A tool rewrites the file | A program is not a data format | A tool that writes config writes JSON, which is a config file (3.2). |
| Completion of keys from a schema | Not in this plan | Section 9. |

### 3.9 Imports

**Without imports (3.0).** A host layers two files with what exists. It loads
`base.risor`, and supplies the value as the variable `base` when it loads
`prod.risor`, which is `{...base, max_body: 16 * 1024 * 1024}`.

**With imports (a minor version).**

```ebnf
Primary = ... | Import .
Import  = "import" String .
```

```ts
const base = import "./base.risor";
{...base, backends: [...base.backends, ...import "./extra.json"]}
```

| Matter | Rule |
| --- | --- |
| Form | The word, then one string of the form `"..."` or `'...'` (`02:2.2`). Not a template, a text block or a raw string, and not an expression. So the files a program can read are fixed by its text. |
| Value | The value of the file's program, evaluated where the import stands: its `const` lines, then its result. |
| Names | Each file is its own program. It reads the environment (variables, functions, named types, version) and none of the importer's names. The importer gets a value and no name. |
| Type | The result type of the file, checked against the same environment with no expected result. Its rejected classes are the environment's. |
| What a file can give | Data. A function is not a value and cannot be the result of a program (`02:2.9`), so **a file cannot give a helper function.** A `const`-bound lambda is used in the file that writes it. Logic that two files need is a host function, or is written twice. |
| Each evaluation | As written. An import that is evaluated twice evaluates the file twice and charges twice. `const base = import "./base.risor";` evaluates it once. An implementation may keep the value of a file that calls no host function, and charge what the file charged (`04:A.8`). |
| Steps and units | 1 step for each evaluation, as for a call (`04:A.3`), plus what the file's program charges. Without that step a chain of files could do work in 0 steps. |
| Depth | An import in progress counts as a call does: 1 plus its nesting within its file (`02:2.16`). |
| Limits | A load is one run: one budget of steps, one of units, one deadline, one depth (`05:A.5`). |
| JSON | A JSON file is a program (3.2), so `import "./data.json"` needs no rule. |
| A program that is one import | `import "./x";` alone is also a TypeScript program, where it is a statement with no value (ran). Here it has the file's value. It joins the block's value in `02:3.1`. As an operand, `import "p"` is a syntax error in TypeScript (ran), so no other text is valid in both. |

| Resolving | Rule |
| --- | --- |
| The resolver | A ninth kind of declaration of an environment (`05:A.2` has eight): host code that takes the name of the importing file and the path as written, and gives a file's **name** and its source text, or fails. Two imports that get the same name are the same file. A description does not carry it. |
| None by default | With no resolver, every import is `import-failed`. A program reads no file that the host did not offer (`05:A.9`). |
| When | At check and at compile, never during a run. From the root, breadth first, each file's imports in source order. Every file is resolved, parsed and checked once, whether or not a run reaches its import. |
| No asynchronous resolver | As for a run (D18). A new analysis result, Imports, gives each import's path and span with no environment (`05:A.8`), so a host that must fetch does it first. |
| Order of checking | A file is checked after every file it imports, so the check does not recurse from file to file (R5). A run does, and the depth limit counts it. |
| A cycle | An import whose file can reach the file the import stands in is `import-cycle`, at the path string. A file that imports itself is the smallest case. No file is run. |
| A failure | The resolver fails, or there is none: `import-failed` at the path string, with the resolver's error as the cause. |
| A file that does not parse | Its `parse` diagnostics are the result of the check, and nothing is checked. So a check can fail in phase `parse`, which `05:A.1` does not allow today. |
| How many files | At most 1,000 in a load, the root included (a candidate number). The import that names file 1,001, in resolving order, is `check-limit`, and resolving stops. A chain is also bounded at run time, by the depth limit. |
| Diagnostics | Every diagnostic, site and trace entry (plan `10`) carries `file`: the name the resolver gave, and for the root the origin's name (`05:A.6`). It is specified, and the suite compares it. An import in progress is a trace entry, at the span of the import. |

**The standard resolver (12-9).** Every binding ships it, over a map from names
to texts or over a directory. The suite's harness uses it.

| Path | Rule |
| --- | --- |
| Form | It starts with `./` or `../`. Segments are separated by `/`. The file's name is written in full, with its extension. |
| Meaning | It is read against the directory of the importing file's name. A `.` segment is dropped, and `..` removes one segment. A result above the root fails. The file's name is the path that results, and names are compared code point by code point. |
| Refused | An empty segment, `\`, a leading `/`, a `:`, and a bare name such as `"base"`, which is kept for a later meaning. |

**The word.** `import` joins the reserved words of `02:2.2` in 3.0 (12-8). Today
it is an ordinary name, and reserving it later would break a program that binds
it. In 3.0 the word is a targeted error: imports are not in 3.0, and a host
passes shared data as a variable. `export` and `from` need no reservation:
`export const x`, `import x from "p"` and `import {x} from "p"` are syntax
errors today (`02:2.2`) (R), so each can be given a meaning later. Each gets a
targeted error, as do `import("p")` and `require("p")`: write `const x = import
"p";`.

## 4. What is not in this plan

| Left out | Why | When |
| --- | --- | --- |
| Expressions the host evaluates later: a rule or a callback as a config value | A function is not a value, and no function crosses the boundary (D34, `05:A.3`). The owner ruled it out of this round. | Not planned |
| Sharing a `const`-bound lambda between files: `import {backend} from "./lib"` | It needs a name that crosses files, and the argument for termination (`02:2.14`) made again over several files. It is what authors will ask for first. | Its own plan |
| Importing a text file as a string | A text block holds a prompt in place (plan `08`). | Later |
| A library `merge` that goes deep | Spread covers one level. | Later, `06` |
| Value constraints: a range, a pattern | D43. | Not planned |
| Completion of keys in a literal, from the expected type | Assist would take an expected result (`05:A.12`). | Later |
| YAML or TOML output | An emitter in the CLI of each implementation. | Not planned |

## 5. Changes to the other documents

| Document | In 3.0 | With imports |
| --- | --- | --- |
| `02` | 2.2: `import` is reserved. 3.2: targeted errors for `import`, `export`, `require`, a `#` comment and a bare key with a `-`. 3.3: JSON is valid with the four exceptions of 3.2 here. | 2.2 and 2.3: `Import` in `Primary`, and one node kind, `Import`, with a `path`. 2.14 and 2.16: an import creates no value and counts as a call. 3.1: one row. |
| `03` | 2.13: the span rule of 3.3. Sections 5 and 6.2: optional fields, and text types, from host types. | 2.5: a row for `Import`. 2.13: `import-failed` and `import-cycle`, so two more codes. 2.17: the file limit. |
| `04` | Nothing. | A.3: 1 step for an import. A.6: depth. A.8: keeping a file's value. |
| `05` | B.3: `Load`, `Decode`, `JSON`. B.7: the two tag rules. B.8: host case 6. B.11: the CLI's flags. Parts C and D: `load`, the JSON text, the rule for `to()`, a place for schema options. | A.1: a check may fail in phase `parse`. A.2: the resolver. A.6: `file`. A.8: Imports. B.5: an `Error` holds more than one source. |
| `06` | 2.6: one sentence, that the process environment is data like the clock. No entry. | Nothing. |
| Plans `10` and `11` | Nothing. | A trace entry for an import, and `file`. A test case with a `files` field. |

**What the conformance suite tests.**

| Area | Cases |
| --- | --- |
| The span of a mismatch | Check cases with `expected`: one for each row and each branch of 3.3, with every `result-mismatch` and its span. |
| JSON as a program | The four exceptions of 3.2, and a JSON document that is valid on both sides. |
| The JSON text of a result | For each run case with a value, the harness also takes the binding's JSON text and compares it with what `json.stringify` gives in the language. |
| Imports | A `files` field, a map from names to sources, and a `file` field that names the root. Expected: the value, steps and units; `file` in each diagnostic and trace entry; a cycle; a failure; the file limit; a file evaluated twice. |
| Not in the suite | The decode and a type taken from a host type. Each harness has cases of its own for its binding, as `05:A.11` says of host values (R). |

## 6. What it costs each implementation (I)

Lines without tests. None of this was built.

| Part | Go | TypeScript | Rust |
| --- | --- | --- | --- |
| The span rule of 3.3, in the checker | 80 | 80 | 80 |
| The JSON text of a result | 20: the renderer exists | 20 | 20 |
| The decode | 40 over `encoding/json`; 250 without the text | 0 | 300: a serde deserializer over the output |
| Optional fields and text types from host types | 40 | 0 | 30 in the derive |
| `Load` | 30 | 20 | 30 |
| CLI flags | 150 | its own CLI, if it has one | the same |
| Imports: node, resolver, graph, checker, evaluator, `file` | 450 | 450 | 450 |

## 7. Prior art

| Language | What it does | What I take from it |
| --- | --- | --- |
| Jsonnet | `import` is an expression with a literal path. The file is "parsed and statically checked in isolation" and cached by file name. A file may evaluate to a function. Values from outside come only as external variables or top-level arguments: it "does not allow unrestricted IO" (W). It is untyped and lazy (`01:2.7`). | The expression, the literal path, isolation, explicit outside values. Not a function as a file's value, and not laziness: an unused field with an error would pass a load. |
| CUE | Imports are declarations at the top of a file and name packages. An import cycle is illegal. The specification has no user functions, and field order is "irrelevant" (W). Types and values are one lattice (`01:2.7`). | A cycle is an error. A config language can do without shared functions. Not unification. |
| Pkl | A module amends another. An import clause brings a module in, and an `import()` expression imports "only a value". `read("env:NAME")` reads the process environment, behind an allowlist. Type constraints are "validated at runtime" (W). It generates classes for host languages (from memory). | The value-only import, and decoding into the host's types. Not a read of the environment from inside a file. Constraints stay out (D43). |
| HCL | The host gives variables and functions in an `EvalContext`, and `gohcl` decodes into a Go struct by struct tags, with diagnostics "written with the configuration author as the target audience" (W). The language has no import (from memory). `01:2.4` has its types. | The host case of 3.4 is this one: declared variables in, a tagged struct out. |
| Starlark | `load("module", "name")` is a statement that binds names, functions among them. The module "must be a literal string", and its meaning "is determined by the application". A module's values are frozen (W). `01:2.3` has its determinism. | The host's resolver and the literal path. Not shared functions: they need function values that outlive their file. |

## 8. Alternatives rejected

| Alternative | Why not |
| --- | --- |
| The expected type flows into literals and changes their types | `03` section 8 rejects it (R). 3.3 needs only the span. |
| A library entry that reads the process environment | The result would depend on something no host declared or recorded (`06:2.6`). |
| A path that is an expression | The files a program reads would not be fixed by its text. A cycle and a missing file would be run errors, the resolver would run during a run, and a TypeScript host could not fetch first (D18). |
| An imported file reads no variable | A base file could not read a setting, and there would be two kinds of file. |
| The call form, `import("p")` | It parses as TypeScript (ran), where it means a promise of a module. `import "p"` as an operand does not parse there (ran). |
| A decode with rules of its own | A second set of naming and conversion rules beside the JSON decoder's, in three bindings. |
| Keys in source order in the output | T2. Three implementations have three map orders. |
| A report on the whole map when a required field is missing | In a config file that span is the file. The opening `{` says the same in one column. |

## 9. Open questions

- **The file limit.** 1,000 is a candidate. It should be settled with the other
  numbers of D31.
- **How far 3.3 follows.** A `const` name and the branches of an `if` are the
  next two steps. Each is a rule three checkers must share.
- **The same table for `arg-mismatch`.** A literal argument that does not fit
  its parameter has the same problem on a smaller scale.
- **An opaque value in a result.** The Go decode could assign one to a field of
  its own type. This plan refuses it everywhere, to keep one rule.
- **The messages of the decode.** Plan `10`'s catalog has no entry for a number
  too wide for a host field. The code is `not-conforming` either way.
- **Imports once for each load,** if measurement shows that evaluating as
  written costs too much in real config trees (12-7).
- Prior art marked "from memory" is not checked.

## Sources

[jsonnet-ref]: https://jsonnet.org/ref/language.html
[jsonnet-spec]: https://jsonnet.org/ref/spec.html
[cue-spec]: https://cuelang.org/docs/reference/spec/
[pkl-ref]: https://pkl-lang.org/main/current/language-reference/index.html
[gohcl]: https://pkg.go.dev/github.com/hashicorp/hcl/v2/gohcl
[starlark-spec]: https://github.com/google/starlark-go/blob/master/doc/spec.md

Jsonnet: [jsonnet-ref], [jsonnet-spec]. CUE: [cue-spec]. Pkl: [pkl-ref]. HCL:
[gohcl]. Starlark: [starlark-spec].
