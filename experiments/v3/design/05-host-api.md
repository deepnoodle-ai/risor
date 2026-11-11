# Risor v3 design 05: host API and embedding

Scope: the contract every implementation of v3 makes with a host (Part A), the
Go API that binds it (Part B), and sketches of the TypeScript API (Part C) and
the Rust API (Part D). Part A is normative for the language (`O:P2`, `O:P7`): a
reader building another implementation needs nothing else. Parts B, C and D add
no rule.

Citations: `O:` is `00-overview.md`, by decision number. `risor:` is this
repository at `07efd88`, `expr:` is expr v1.4.1, `workflow:` is
`workflow@v0.1.0` in the Go module cache, `proposal:` is
`docs/proposals/workflow-expressions.md`. `02` to `06` are the sibling
documents, which were in revision while I wrote (section 6). `07` is
`07-checked-conversion.md`, the authority for `as`, `is` and named types. `08`
to `13` are the six plans the owner accepted as `O:D46` to `O:D51`: strings,
tagged unions, errors, testing, config files and queries. Each is the authority
for its feature, and a decision of a plan is cited by its own number, as
`10-3`. (I) marks an inference.

## 1. Summary

1. **The contract is an abstract embedding model:** six stages and eight kinds
   of declaration, with no Go type in it.
2. **An environment declares, in type text or JSON Schema, and variables are
   supplied per run.** Its portable description lets one implementation check
   against an environment another built.
3. **Four limits, each with a default, a code and a guarantee:** steps,
   allocation units, time and depth. A run reports what it used.
4. **Diagnostics are specified:** a code, a class, a span in lines and columns
   of code points, and a message built from one catalog, `messages.json`. A
   failed run also lists the calls in progress. Every binding renders a
   diagnostic in the same three forms (A.6; `10`).
5. **Host functions are synchronous and declared with a signature.** By contract
   they do not change or keep their arguments. They charge steps and allocation
   through the call handle.
6. **A namespace is a compile-time prefix,** and every name clash is an error
   when the environment is built.
7. **An environment states the specification version it wants.** An
   implementation that lacks it refuses, and never substitutes another.
8. **The Go binding keeps its shape,** with `Type` in the root package. Types
   from Go structs are that binding's convenience.
9. **The TypeScript binding differs in kind in seven places,** and none changes
   Part A. A run is synchronous, and there is no async run in 3.0. **The Rust
   binding differs in five more** (Part D): host values through a trait, values
   lent for the length of a run, a result that owns its memory, no collector,
   and a stack that cannot overflow safely. One of them sharpens Part A: "an
   object with named fields" must not assume reflection (`O:R2`).
10. **Editor support is completion, hover and the environment description,** in
    3.0. I still recommend against a standalone language server.
11. **A host makes two choices about checking.** Per check, the result it
    expects: nothing, a type, or a condition that it tests by truthiness (A.3).
    Per environment, the check classes it rejects (A.2). `constant-condition`
    is rejected by default, and a host takes it out to re-check stored programs
    after a schema tightens.
12. **Untyped data is tested, by the program or by the host** (`07`). An
    environment declares named types, which a program writes after `as` and
    `is` (A.2). Every binding gives the host the same test as a function of a
    value and a type (A.13). It is not a run, so it takes no limits. A failure
    is `not-conforming`, and the place of the first failure is a field of the
    diagnostic, not text in its message (A.6). A second function goes on after
    a failure and reports every one, up to a maximum (`10-8`).
13. **Tests are data** (`11`). The case format of the conformance suite is also
    the author's format: a case file declares an environment once, stubs host
    functions with tables, and lists cases (A.11). `risor test` runs a file
    that declares everything in data. A helper in each binding runs the same
    file against the host's real environment.
14. **A config file is a program, and a load is five steps in a fixed order:**
    parse, check against the host's type, run, conform, decode into the host's
    own type (A.14; `12`). Only the decode is new. Imports are not in 3.0.

## 2. Part A: the embedding model

### A.1 Stages

| Stage | Takes | Gives | Fails in phase |
| --- | --- | --- | --- |
| Parse | source text of a program or a template | a parsed program | `parse` |
| Analyze | a parsed program | names read, static paths, calls, literal value, template segments | never |
| Check | a parsed program, an environment and, when the host has one, the expected result (A.3) | the result type and every problem found | `check` |
| Compile | the same | a runnable program; fails exactly when check fails | `check` |
| Run | a runnable program, variables, a cancellation signal | a value or one error, and the usage | `run` |
| Assist | source text that need not parse, an environment, a position | completions, or the type under the cursor (A.12) | never |

- **Parse needs no environment,** so analysis is available before types exist.
  The engine reads names first and builds types from them
  (`workflow:validate.go:807-848`).
- **One parse entry point takes the language's one top-level construct.** I
  assume `02` folds the two forms into one (`O:E2`). If two survive, the entry
  point takes a form argument and nothing else changes.
- **Check is not optional,** and one program can be checked against many
  environments (`workflow:typecheck.go:350-389`). Data with no schema is
  declared `any`.
- **Every value of the model is immutable once built** and may be shared by
  concurrent runs. A run's state belongs to that run.
- **A load is not a stage.** Loading a config file is parse, check, run, the
  conformance function and the binding's decode, in that order (A.14).

### A.2 The environment

An environment is a set of declarations.

| Declaration | States | Portable form |
| --- | --- | --- |
| Version | the specification version, `major.minor` (A.10) | text |
| Variable | a name and its type | type text, or JSON Schema (`O:P5`) |
| Function | a name, a signature, an implementation, documentation | the signature in type text; the implementation is host code |
| Opaque type | a name for a host type that has behaviour, and its methods, each a function whose receiver is that type (`03`, 2.15) | the name, and each method's signature in type text. Which host values carry the name is binding-specific |
| Named type | a name and the type it abbreviates (`07`, 2.5) | the name, and the type as type text or JSON Schema |
| Library | the optional groups that are on, and entries removed | group and entry names |
| Rejected classes | the check classes whose findings are errors: classes added to the default set, and classes taken out | class names |
| Limits | the four limits of A.5 | numbers |

A **child environment** adds declarations to a parent without changing it. A
scope of a host document is a child.

**Namespaces** (`O:S1`). A function name may have one prefix, as in
`geo.distance`. The checker resolves it. It is not a value, so `geo` alone is an
error. Each optional library group is a namespace of the same name (`06`, 2.10).

| Clash, when the environment is built | Result |
| --- | --- |
| Two declarations of one name in one environment | error `name-clash` |
| A variable named like a namespace that exists in the environment | error `name-clash`. A host with a variable `time` cannot enable the group `time` |
| A host function named like a library entry that is present | error `name-clash`. The host removes the entry first, which makes the replacement visible |
| A host function inside a library group's namespace | error `reserved-namespace`, whether or not the group is on |
| A child declares a host name its parent has | the child's replaces it, in the child only |
| A raised version adds a library name the host declares | error `name-clash` at the next build. Never a silent change of meaning |
| A lambda parameter or `const` name equal to an environment name | not a clash: it shadows (`02`, 2.1) |
| Two type names equal in one environment, whether each is a named type or an opaque type | error `name-clash` |
| A type name that is not an upper-case letter followed by one or more of `[A-Za-z0-9_]` | error `bad-name`. One upper-case letter alone is a type variable of a signature (`03`, 2.2) |
| A variable, a function or a namespace named with a reserved word of `02`, 2.2 | error `bad-name`: no program could read it. `catch`, `throw`, `finally` and `import` joined the reserved words with `O:D48` and `O:D50`. A field of a variable may have such a name: after `.` it is a word like any other |
| A named type that uses a name not declared before it, or its own | error `bad-type` |
| A type text of any declaration in which two or more records are alternatives and have no tag | error `bad-type` (`09:2.3`). The same text after `as` or `is` in a program is the check code `untagged-union`. Schema branches that form no tagged union join, and nothing is reported (`09:2.8`) |
| A child declares a named type its parent has | the child's replaces it, in the child only. What the parent declared with the name keeps the type it had |
| A child gives an opaque type's name to a named type, or the reverse | error `name-clash`: values of the opaque type still reach the child |
| A type name equal to a variable, a function or a namespace | not a clash: a type name is read only where a type is written |

**Check classes** (`O:B7`). A check class is a named group of findings that an
environment either rejects or lets pass. The environment carries the set of
classes it rejects.

| Class | Covers | In the default set | Rejected | Not rejected |
| --- | --- | --- | --- | --- |
| `constant-condition` | a tested expression that can never be falsy, or can never be truthy (`03`, conditions) | **yes** | each is the diagnostic `always-truthy` or `always-falsy` | the two codes are not reported |
| `dynamic`, `index`, `divide`, `value`, `host` | the residual sites of `03`'s promise: places where a checked program can still fail on a value | no | each site is a diagnostic: `unchecked` for `dynamic`, `may-fail` for the rest | a site is not a diagnostic. It is listed in the check's result, as a class and a span (`03`, 2.13 and 2.14) |

- **The declaration changes the set.** It names classes to add and classes to
  take out. An environment with no such declaration has the default set.
  Rejecting `dynamic` is `03`'s strict mode.
- **A check gives three things: a type, diagnostics and sites.** The sites are
  those of the classes the environment does not reject, each with its class and
  its span. A host that rejects nothing can still show them, or count them. The
  conformance cases of `03` compare them. Every binding returns them: `Sites`
  in Go, the `sites` field in TypeScript and in Rust.
- **A name that is not a check class is the `declare` error `bad-class`.** The
  run-error classes `environment`, `range` and `limit` are not check classes: a
  checker cannot find them.
- **A child's declaration changes the set in the child only,** like every other
  declaration.
- **The set does not change what a program is.** A program that checks under
  two sets has the same type, the same value and the same usage under both.
- **It is on the environment and not on a check call,** so that the description
  carries it and every editor and server that shares the description gives the
  same answer. The expected result is the opposite case (A.3).

**Type names** (`O:V3`; `07`, 2.5). An environment has one set of type names.
It holds the opaque types and the named types.

- **A named type is an abbreviation.** `Order` and the type it names are the
  same type in every rule, and a type prints by its structure, never by the
  name (`07`, D44). So a named type adds no kind of value and nothing to the
  boundary of A.3.
- **Where a name may be written.** In the type text of any declaration of the
  same environment or of a child: a variable, a signature, a later named type.
  And in a program, after `as` or `is` (`07`, 2.4). A type from a JSON Schema
  cannot use one: a `$ref` names a schema in its own document (`03`, 2.15).
- **Order.** Named types are resolved in the order they are declared, the
  parent's first, and each may use the names before it. That is what makes a
  cycle impossible to write. The other declarations may use every type name of
  the environment, wherever they stand.
- **A named type from a JSON Schema takes no schema option by default** (`07`,
  2.9). The three options of `03` 2.15 say what a host did to a value before a
  run, and data a program meets inside a run was not normalized by anyone. A
  host that does normalize a value before it supplies it passes the options for
  that variable's type, as before.
- **It is not a schema.** The type keeps the shape and drops every keyword that
  constrains values (`07`, D43). A host that must enforce `minLength` or
  `pattern` does so with its own validator, before the run or in a host
  function.

*The case it exists for.* A host stores programs that were checked when they
were saved. Later a schema tightens a field from optional to required: `report`
was `{verdict: string} | null` and is now `{verdict: string}`. A stored
`report && report.verdict == "APPROVE"` then has a test that can never be falsy,
and a fresh check reports `always-truthy`. The program is not wrong: it gives
the value it always gave. Nothing else about tightening a type breaks a stored
program (`O:B7`). So the host re-checks its stored programs in a child
environment that takes `constant-condition` out, and a failure there is a real
break. It keeps the class in for a program an author is editing, where a dead
test is worth an error. B.8, case 4, shows it.

Alternative: a warning severity for the two codes. Why not: it needs a field in
every diagnostic, a rule for what a host does with a warning, and a column in
every conformance case (`03`, the constant conditions). A set on the environment
gives the host the same choice and adds no field.

**The description.** An environment can be written out as data: version,
variables as type text, function signatures and documentation, opaque type names
with their method signatures, named types in their order, each as a name and a
type text, groups, removed entries, the rejected check classes and limits. A
named type declared from a JSON Schema is written as the type text it became. A
tagged union is written as its variants joined with `|`, which reads back as the
same type (`09:2.3`). The classes are written as the whole resulting set,
`"reject": ["constant-condition"]`, so a reader need not know the default. Check
needs nothing else, so an environment built from a description can check but not
run, and a browser editor agrees with the server (A.12). A function typed by a
rule (A.4) is described by its declared signature only.

### A.3 Variables and values

- **Variables are supplied to each run, by name.** The run asks only for names
  the program reads on the path it takes, at most once each, so a host may
  compute values on demand. A declared variable that is not supplied is
  `missing-variable`, which `catch` does not catch: no value caused it, and a
  handler would hide a fault in how the host is wired (`10-3`).
- **Host values are read in place.** Nothing is copied or converted when a run
  starts, and host data is not charged to the allocation budget (`O:L4`).
- **Declared types are a promise that the evaluator does not trust.** A value
  that does not fit its type gives a run error of class `environment`. It never
  gives a crash or a wrong value. The run finds this only where it reads.
  `catch` catches it, because `catch` decides by code and the code is that of
  any null, wrong kind or missing field (A.6). A host can test the whole
  promise itself, before a run, with the conformance function (A.13).
- **A result may share memory with the inputs,** so the host treats it as
  read-only. The `data` of a `failed` diagnostic leaves a run the same way
  (A.6).
- **A result has one JSON text.** Every binding gives the JSON form of `02`
  2.13 for a value a run returned: the text `json.stringify` gives inside a
  program, with keys in code point order and numbers in the rendering of `02`
  2.5. It is not a run and charges nothing. It fails where `json.stringify`
  fails (`06`, 2.8), with the same code and no span: `wrong-kind` on an opaque
  value, and `depth-exceeded` on nesting deeper than the default depth limit of
  A.5, so that cyclic host data cannot make it loop (I). A host's own JSON
  encoder is not this function: it orders keys and writes numbers its own way.
- **A host may test a result by truthiness,** with the binding's own test, or
  require a type such as `bool`. It says which when it checks. "The expected
  result", below, has the rules.

| Language value | A host may supply | Rule at the boundary |
| --- | --- | --- |
| null | its own value for "nothing": `nil` in Go, `null` in JavaScript, `None` in Rust | Absence of a field is not null (`O:B2`). Each binding says what counts as absent |
| bool | its boolean | Any value may be a condition. What is falsy is the language's rule, not the binding's: null, `false`, zero and anything empty (`O:B5`). A binding must be able to tell an empty host list or map from a non-empty one in bounded work |
| number | any of its number types | A whole number outside ±(2^53 − 1), a NaN or an infinity cannot enter the language. Where a run reads one, it fails with the code `number-range` and the class `environment`, and `catch` catches it (`O:N2`, `O:N3`; `04`, A.2). The host passes such an identifier as a string |
| string | its text | Length and positions are in code points (`O:T1`) |
| list | an ordered sequence | Read through, not copied |
| map or record | a string-keyed mapping, or an object with named fields | Iteration is in key order by code point (`O:T2`). How a binding finds a field by name is its own matter: reflection in Go, own properties in TypeScript, a trait in Rust (`O:R2`). The model assumes only that a value can report its kind, its size, a field by name and its keys |
| opaque | any other host value | It can be passed through, given to a host function, and used through declared methods |
| function | never | No function value crosses the boundary in either direction (`O:S3`) |

`number-range` is raised at the read, since values are read lazily. An integer
that is only passed through fails too: an implementation with doubles cannot
carry it. The code is the one arithmetic gives for a result out of range, and
the class tells the two apart: `range` for arithmetic, `environment` for a host
value (A.6). Met inside the walk of `as`, `is` or the conformance function,
such a value does not conform, and nothing else is reported (A.13).

**The expected result** (`O:B8`; `03`, the expected result). Check and compile
take, with the program and the environment, the result the host expects. There
are three choices.

| Expected | Accepted | Reported | The host then |
| --- | --- | --- | --- |
| nothing | a result of any type | nothing | reads the type the check gives |
| a type, such as `bool` | a result that fits it through its form (`09:2.6`). Nothing is converted | `result-mismatch` at each place of failure: the result expression, or a place inside it | has a value of that type: with `bool`, a real boolean |
| `condition` | a result of any type | at the result expression, `always-truthy` when the result can never be falsy, and `always-falsy` when it can never be truthy | tests the value with the binding's truthiness test |

- **Where a mismatch is reported** (`O:D47`, `O:D50`). The result's type is
  tried first, as before. When it does not fit, the fit is tried through the
  result's form: into a list or map literal, the branches of an `if` or `? :`,
  the arms of a `match` and the result of a block. `09:2.6` is the one rule,
  and `03` holds it. So a config file with a wrong, unknown or missing field
  gets its error at that entry, that key or that map, and not on the whole
  file. A required field with no entry is reported on the whole map. No type
  changes: the expected type decides only where a diagnostic is placed. The
  same rule places `arg-mismatch` at an argument.

- **Where it is passed.** To each check and each compile, as an argument. It is
  not a declaration of the environment, because one environment serves fields
  of different kinds: a `when:` field wants a condition, and an input field
  wants a value of its declared type. Compile takes the same argument and fails
  exactly when that check fails (A.1).
- **`condition` is not a type.** Written as data it is the word `condition`
  where a type text would stand, as in the `returns` field of a case (A.11). It
  makes the result a tested position, under the rule `03` gives for the
  test of `? :` and of `if`. So `true`, a `bool`, a number, a string and a list
  all pass, because each can go both ways. A record with a required field, an
  opaque value, a literal set without `""` and null alone do not. A result of
  type `any` passes and reports nothing.
- **The two codes belong to the class `constant-condition`.** An environment
  that has taken the class out reports neither (A.2), and `condition` then
  accepts every result.
- **The run does not change.** The expected result is not kept by the compiled
  program, and the value is not converted to a boolean. The host tests it.
- **The truthiness test.** Every binding offers the language's own test as a
  function of one value. It is false for null, `false`, zero, the empty string,
  an empty list and an empty map, and true for everything else, an opaque value
  included. It takes any value a run can return, including one that shares
  memory with host data. It cannot fail and it is not part of a run, so it
  charges nothing. A host never writes its own list of falsy values, and never
  uses its own language's truthiness in place of this test (`O:B5`).
- **Which to choose.** `bool` is the strict choice, and its price is that the
  guard most authors write, `report && report.verdict == "APPROVE"`, has type
  `bool | null` and is rejected. `condition` accepts it. The workflow engine
  takes `condition` for `when:` (`O:B8`). The price of `condition` is that a
  list or a string in `when:` is accepted when it can be empty, and reported
  only when it never can.

The Go binding spells these `risor.Expect(t)`, `risor.Condition` and
`risor.Truthy(v)` (B.2, B.3). Parts C and D give the other two spellings.

Alternative: the expected result as a declaration of the environment. Why not:
a host would build one environment per kind of field, where today it builds one
per scope.

### A.4 Host functions

- **Declaration.** A name, a signature in type text such as `(s: string, n?:
  int) => string !`, and an implementation. `!` marks a function that can fail.
  A binding may derive the signature from the host language's types.
- **No callback parameter.** A host function or method takes values only. A
  lambda is passed only to a library function or method (`O:E9`), and no
  function value crosses the boundary (A.3). A signature with a function-typed
  parameter is `bad-signature` when the environment is built.
- **Type rule.** A function may carry host code that computes the result type at
  each call site from the argument types and expressions. The engine needs this
  for `coalesce` and `choose` (`workflow:typecheck.go:611-725`).
- **The call** is synchronous: the function returns before the run continues. It
  receives the arguments and a **call handle**, which gives the cancellation
  signal, `charge(steps)` and `allocate(units)`. Whether it may wait for I/O is
  the binding's matter (Part C).
- **Result.** A value of the declared type, or an error. An error is
  `function-error`, of class `host`, at the span of the call, with the host's
  error as the cause and its text as the message. `catch` catches it. A panic
  or a thrown non-error is `function-panic`, also of class `host`, which
  `catch` does not catch: the state of the host function is not known (`04`,
  A.1). A host function's error carries no `data` in 3.0 (`10`, section 7).
- **Never at compile time, never cached,** even with literal arguments.

What a host must do for its functions to stay within the contract:

| Duty | Why |
| --- | --- |
| Treat arguments as read-only and do not keep them after the call (`O:S2`) | Constant collections are shared between runs. This is a contract, not a check |
| Call `charge` in proportion to the work: one step per element visited, and text at the specification's rate | Otherwise the step budget does not bound the run (`O:L4`) |
| Call `allocate` before building a result whose size depends on its input | Otherwise the allocation budget does not bound the run |
| Stop when the signal says so, if the function can wait | The evaluator cannot interrupt it (`O:L5`) |

An uncharged function costs the one step of its call. The limits cover host
functions exactly as far as they charge (`O:L8`).

### A.5 Limits and usage

| Limit | Counts | Default | Code when exceeded | Deterministic |
| --- | --- | --- | --- | --- |
| Steps | work, by the specification's cost table (`O:L2`) | 1,000,000 | `steps-exceeded` | yes |
| Allocation | units of every value the language or library creates in the run, charged before the memory is taken (`O:L3`) | 1,000,000 units | `allocation-exceeded` | yes |
| Time | wall-clock time of one run | none | `deadline-exceeded`; `cancelled` when the host cancels | no |
| Depth | each call in progress as 1 plus its nesting depth within its body (`02` 2.16), and each level of a walk over data | 200 | `depth-exceeded` | yes |
| Source nesting | levels of nesting open at once in the program text, counted when it is parsed (`02` 2.16) | 100, fixed by the specification (`O:R5`) | `nesting-exceeded`, of phase `parse` | yes |

The first three can be set or turned off; depth can only be set. Source nesting
cannot be set: whether a text is a program must not depend on the host. The five
codes of the first four rows have class `limit`, and `catch` does not catch them
(A.6). The specification also fixes the largest depth a host may set (candidate:
1,000), so that every implementation can state the stack it needs for 2D + 100
levels. The defaults:

- **Steps: 1,000,000,** the engine's value (`workflow:expr.go:16`): about 15 ms
  of language work at the spike's cost per node (I). v2 and expr default to
  unlimited (`risor:risor.go:218`, `expr:engine.go:159`).
- **Allocation: 1,000,000 units.** If a unit is near 16 bytes in Go, this is
  about 16 MB (I), twice the engine's 8 MiB cap on a run's state
  (`proposal:487-488`). The number is to be reset when `04` fixes the unit
  (`O:L7`).
- **Time: none.** It is the one limit that is not deterministic, and a host that
  replays cannot use it (`proposal:297-301`). Steps already bound the language's
  work, and a blocked host function is not interrupted, so a default adds
  machine dependence and no protection.
- **Depth: 200,** as `04` proposes. With no recursion (`O:E6`) it matters for
  walks over cyclic host data (`01`, lesson 6).

**Time** is enforced at a specified step interval: the run checks whether the
limit has passed or the host has cancelled. How an implementation learns that is
its own business. **Usage:** every run, successful or not, reports the steps
used and the units allocated. They are the same in every implementation and
version (`O:L6`), unless the run stopped on time or cancellation.

### A.6 Diagnostics

A failure of any stage is a list of diagnostics with one phase: `declare`,
`parse`, `check` or `run`. A run fails with exactly one diagnostic.

| Field | Meaning | Specified |
| --- | --- | --- |
| code | the kind of problem: `no-field`, `maybe-null`, `steps-exceeded`. "The codes", below, lists every one | yes (`O:P6`) |
| class | for a run error, its class in the promise of `03`, 2.14: `index`, `divide`, `value`, `host`, `dynamic`, `environment`, `range`, `limit` ("The class of a run error", below). For a check diagnostic that exists because its check class is rejected (A.2), that class: `constant-condition`, or the class of the site | yes |
| span | start and end position in the source text; the end is exclusive | yes |
| message | one sentence, built from the code's template in the catalog ("Messages", below) | yes (`10-6`) |
| path | for `not-conforming`: where in the tested value the failure is. A list of steps from the tested value, each a key, as a string, or a list index, as a number from 0. Empty when the tested value itself is of the wrong kind | yes |
| data | for `failed`: the second argument of `fail`, or null when it had none (`10:2.5`). No other code has it | yes |
| trace | for a run error: the calls in progress when the run failed, from the outermost in ("Trace", below) | yes (`10-5`) |
| help, expected, found | what to write instead; two types in type text | no |
| fix | a span and its replacement text | no |
| cause | the host's own error, for `function-error` | no |

**Positions** (`O:T5`). A position is a line and a column, both from 1. A line
ends at U+000A and at nothing else: a U+000D is an ordinary character. A column
counts code points, so a tab and an astral character are one column each. An
implementation may add an offset in its own unit. A span is the narrowest node
at fault (`03`).

The string forms of `08` keep these rules:

| Matter | Rule |
| --- | --- |
| A literal's span | From its first character, the `r` of a raw prefix if it has one, to the end of its closing delimiter (`08:2.5`) |
| Inside a text block | A span is its place in the source text as written. Removing the indentation moves no position, so an error inside `${ }` on the third line of a block has that line and that column |
| Line ends | In the value of a text block a U+000D U+000A and a lone U+000D each read as one U+000A (`08:2.3`). That is a rule for the value. A position still counts a line at U+000A only |
| The three codes | `text-block-open`: the first character after the opening `"""` that is not a space or a tab. `text-block-close`: the closing delimiter. `text-block-indent`: the first character of the line that differs from the indentation, once for each such line, in order (`08:2.7`) |
| A place in a value | Not given. A host that parses a string's value, as SQL for example, maps a place in it to the source itself |

**Origin.** The host may say where the source text sits in a larger document: a
name, the line and column of the first character, and the columns removed from
the start of each later line. A diagnostic then also carries the span in the
document. This is exact for YAML plain scalars and literal (`|`) blocks; for a
quoted or folded scalar the host maps the span itself (I). A text block needs
its line ends, so it fits a `|` block, where the origin stays exact: YAML
removes its own indentation first, and the block's indentation is counted in
the program text that is left. A folded scalar joins the lines, and the program
then fails with `text-block-open`. A host that reads programs from folded
scalars cannot offer text blocks (`08:2.9`).

**The codes.** Every code has a name, a phase and an entry in the catalog. A
code is never renamed or reused within a major version. One may stop being
reported: when a minor version gives a text a meaning, the targeted error for
that text goes, as `import-statement` will (`02`, 3.2). There are 115.

| Phase | Codes | Named in |
| --- | --- | --- |
| `declare` | 8: `name-clash`, `reserved-namespace`, `bad-name`, `bad-signature`, `bad-type`, `bad-class`, `unsupported-version`, `no-implementation` | this document |
| `parse` | 58, in the table below | `02`, 3.2; three of them in `08:2.7` |
| `check` | 30: the 29 of `03`'s catalog, which gained `untagged-union` (`09-2`), and `needs-version` | `03`, 2.13; A.10 |
| `run` | 19, in the table below | this document; `02`, 2.12; `06`, 2.3; `07` |

| `parse` codes | |
| --- | --- |
| The 13 that are not targeted errors | `unexpected-token`, `unexpected-character`, `unterminated-string`, `unterminated-comment`, `bad-escape`, `line-continuation`, `bad-number`, `literal-range`, `text-block-open`, `text-block-close`, `text-block-indent`, `nesting-exceeded`, `bad-type-text` |
| The 45 targeted errors, one for each mistake an author is known to make | `strict-equality`, `missing-else`, `if-form`, `branch-like-map`, `empty-branch`, `needs-parentheses`, `try-call`, `missing-catch`, `try-form`, `catch-name`, `finally-clause`, `throw-statement`, `switch-statement`, `function-declaration`, `return-statement`, `let-binding`, `loop-statement`, `assignment`, `import-statement`, `export-statement`, `block-without-const`, `lambda-in-pattern`, `reserved-word`, `slice-syntax`, `optional-index`, `star-form`, `filter-step`, `context-symbol`, `bitwise-operator`, `pipe-operator`, `nil-literal`, `type-annotation`, `satisfies-operator`, `instanceof-operator`, `typeof-operator`, `angle-cast`, `type-declaration`, `as-const`, `negated-is`, `text-block-quote`, `string-prefix`, `tagged-template`, `heredoc`, `hash-comment`, `unquoted-key` |

`02`, 3.2 says when each is reported and what its span is. It chose one code for
one mistake: rows that are the same mistake in another spelling share a code,
and the catalog then has a template for each row. So a case, and a host that
shows its own wording, can tell one mistake from another by the code alone.

**After the first `parse` error** (`02`, 3.2 leaves this here). Three parsers
recover from a bad text in three ways, so only the start of the list is
specified.

| Matter | Rule |
| --- | --- |
| The first diagnostic | Specified: its code, its span and its message. Every parser reports the same one |
| The lines of one text block | When the first diagnostic is a `text-block-indent`, the others of the same block follow it, one for each bad line, in order, and they are specified too (`08:2.7`) |
| After those | Not specified. A parser may stop, or go on and report more. A parser that goes on reports nothing more for a literal after `text-block-open` or `text-block-close`, and nothing at all after `nesting-exceeded` (`02`, 2.16) |
| A case of phase `parse` | Lists the specified diagnostics and no more. The runner compares that many, from the start of what the parser reported (A.11) |
| The other phases | A `check` failure is specified whole, in the order of `03`, 2.13. A run fails with one diagnostic |

| `declare` code | Reported when the environment is built |
| --- | --- |
| `name-clash`, `reserved-namespace`, `bad-name` | The rows of A.2 |
| `bad-signature` | A signature that is malformed, or has a function-typed parameter (A.4) |
| `bad-type` | A type text or schema that is malformed, or nested past the 64 levels of `03`, 2.17, or that uses a type name it cannot resolve, or in which records are alternatives with no tag (A.2) |
| `bad-class` | A rejected class that is not a check class (A.2) |
| `unsupported-version` | A version the implementation does not have (A.10) |
| `no-implementation` | A compile asked of an environment that has a function with no implementation, as one built from a description has (A.2) |

| `run` code | Raised when | Class | `catch` |
| --- | --- | --- | --- |
| `steps-exceeded`, `allocation-exceeded`, `depth-exceeded`, `deadline-exceeded`, `cancelled` | A limit is passed, or the host cancels (A.5) | `limit` | passes |
| `missing-variable` | A declared variable is read and the host did not supply it (A.3) | `environment` | passes |
| `function-panic` | A host function crashes (A.4) | `host` | passes |
| `internal-error` | The implementation itself fails (A.9) | none: the promise does not cover it | passes |
| `function-error` | A host function returns an error (A.4) | `host` | catches |
| `number-range` | A result of arithmetic or of a library entry is out of range or not finite (`02`, 2.5). A host number that cannot enter the language is read (A.3) | `range`. `environment` | catches |
| `not-conforming` | `x as T`, on a value that does not conform (`07`, 2.3) | `value` at a site; `environment` where the checker showed the subject fits | catches |
| `invalid-argument` | A library entry is given a value it cannot use (`06`, 2.3) | `value` | catches |
| `failed` | The program calls `fail` (`10:2.5`) | `value` | catches |
| `null-operand`, `wrong-kind`, `missing-field` | A null, a value of the wrong kind or a missing field stands where an operation needs something else (`02`, 2.12) | `dynamic` at a `dynamic` site; `environment` anywhere else | catches |
| `index-range` | A plain index is outside its list or string (`02`, 2.12) | `index` | catches |
| `divide-by-zero` | A zero divisor (`02`, 2.12) | `divide` | catches |
| `no-match` | A `match` by cases finds no arm (`02`, 2.12) | `environment` | catches |

`02` named six of these in round 8: `null-operand`, `wrong-kind`,
`missing-field`, `index-range`, `divide-by-zero` and `no-match`. This document
named `internal-error`. A run code is never also a `check` code: `maybe-null`
says a value may be null, and `null-operand` says one was.

**The class of a run error.** For most codes the table fixes it. Three points
need more.

- **A null, a wrong kind or a missing field on data** has the class of the
  place: `dynamic` at a `dynamic` site, and `environment` anywhere else, where
  the checker showed that it cannot happen unless the host broke the promise of
  A.3. So this class needs the checker's sites. `03`, 2.14 has the rule, and
  this document follows it. In a checked program `environment` always means
  that the promise was broken.
- **A program that was run with no check has no sites,** as in an evaluator
  that is built before its checker. There such a failure has class `dynamic`,
  and so has a `no-match`, and a failed `as` is `value` (`03`, 2.14).
  `environment` is then reported only where no site is needed to know it: a
  host number that cannot enter the language, and `missing-variable`. A case of
  the suite that compares a class always checks first.
- **A program never sees the class.** The error value that `catch (err)` binds
  has `code`, `message`, `path` and `data`, and no `class` (`10-2`): the class
  depends on what a checker inferred, and a value must not.

**What `catch` catches** is decided by the code and by nothing else (`10-3`;
`02`, 2.12). The last column above is the rule, and the catalog carries it as
the field `caught`. Eight codes pass through: the five limit codes,
`function-panic`, `internal-error` and `missing-variable`. The other eleven are
caught, and they are the literal set that is the type of `err.code`:
`divide-by-zero`, `failed`, `function-error`, `index-range`, `invalid-argument`,
`missing-field`, `no-match`, `not-conforming`, `null-operand`, `number-range`
and `wrong-kind`. So `catch` does catch a failure that a host sees as class
`environment`, a host number that cannot enter the language among them. A host
that wants to find a broken promise tests its values with the conformance
function before the run (A.13).

**Trace** (`10:2.8`). A span says where a run failed. Inside a helper or a
callback the author also needs to know which call, and which element.

| Matter | Rule |
| --- | --- |
| What | The diagnostic of a failed run has `trace`: a list of entries, from the outermost in. It may be empty. A diagnostic of another phase has none |
| A call in progress | As `02`, 2.16 defines it for the depth limit: a call by name of a `const`-bound lambda, or a call of a library entry that is running a lambda passed to it. Each has one entry |
| A projection in progress | A star whose reads are running on an element has one entry too, of the same shape (`13`, section 7). A projection with two stars can have two |
| An entry | `span`: the `Call` or `MethodCall` node, or the star, `[*]` or `?.[*]`. `name`: the `const` name, the library entry's name, such as `map`, or `[*]` for a star. `index`: for a library call, the index the callback was given, from 0; for a star, the index of the element; for `mapValues`, whose callback is given a key and no index, the position of the entry in key order, from 0; for a `const`-bound lambda, null. So an index is always a number or null |
| No entry | A `try`, a library call that takes no lambda, and a host function, whose own failure is the diagnostic's span |
| Length | At most the depth limit and the 100 levels of source nesting together: each call in progress counts against the first, and the stars around the failure against the second (A.5). It is never cut |
| Cost | Nothing when nothing fails. An entry is written as the failure leaves each call. It is not charged: a diagnostic is not a value of the language, and its size is bounded |
| A caught error | A program cannot read a trace. A failure raised in a handler has the trace of the place that raised it |
| A limit error | Has a trace. The suite does not compare it, or its span: the position of a limit error is not specified (`04`, A.1) |
| The conformance function | Its diagnostics have no span and an empty trace (A.13) |

**Messages** (`10:2.9`). A message is one value in every implementation,
because a program can read it as `err.message` and is charged for its size
(`10:2.7`).

- **One catalog, as data.** `messages.json` stands beside `library.json` (`06`,
  2.11) and is part of the specification. It has one entry for each code of
  every phase.
- **The wording is the file's.** No template is written in this document or in
  any sibling. A message shown in one of them is an illustration.
- **Each implementation generates its table from the file.** None writes a
  message by hand.

| An entry has | Rule |
| --- | --- |
| `code`, `phase`, `since` | The code, its phase, and the specification version that added it |
| `caught` | For a `run` code: whether `catch` catches it. The type of `err.code` is the literal set of the codes that have it true, generated from this field (`10:2.3`) |
| `params` | The parameters of the templates, each with a kind from the table below |
| `templates` | One template, or several, each named, where the failure has more than one shape. The entry says which condition takes which. `not-conforming` has three: a value of the wrong kind, a required field that is missing, a key the type does not allow. `number-range` has one for an operator, one for a library entry's result, which names the entry, and one for a host number. `invalid-argument` has the 14 of `06`, 2.3. A `parse` code that several rows of `02`, 3.2 share has one for each row |
| `cases` | Conformance cases for the entry, in the format of A.11. Each template has at least one |

| Matter | Rule |
| --- | --- |
| A template | One sentence with no full stop at its end. A hole is `{name}`. No other brace stands in it |
| A parameter | Has one kind, and each kind has one rendering. A rendering longer than 64 code points is cut to its first 61 and `...`. So a message is bounded by its entry |
| A long type | Is a `type` parameter and is cut by that rule: the literal set of a whole text block does not fill a message (`08`, section 9). The whole type may stand in `expected`, which is not specified |
| At run time | A parameter is taken from the value and from the text of the program or the environment. Never from an inferred type (`07`, 2.6) |
| Not in a message | The path, which is a field and is printed by the renderings below. The span. A suggestion, which belongs in `help` |
| Not from a template | `failed`: the first argument of `fail`, whole and not cut. `function-error`: the text of the host's error, as the binding gives it. A failure of the decode (A.14): the binding's text |
| Not specified | `help`, `fix`, `expected`, `found` and `cause`. A "did you mean" needs one search algorithm in three implementations, and nothing a program can see depends on it |
| Stability | The catalog belongs to the specification version, as the cost table does. A `run` message is seen and paid for by programs, so it does not change within a major version (A.10). A message of another phase may change in a minor version |

| Kind of parameter | Rendering |
| --- | --- |
| `word` | A name, a field word or an operator of the program, as written |
| `string` | A string of the data, in its JSON form (`02`, 2.13), with the quotes |
| `value` | A null, a bool, a number or a string in its JSON form. For anything else, the word `list`, `map` or `opaque` |
| `kind` | What `type(x)` gives (`06`, 2.8) |
| `type` | Type text as `03`, 2.2 prints it |
| `number` | The rendering of `02`, 2.5 |
| `token` | In the `parse` phase: the token at the span as written, or `end of input` |

Three parsers must agree on the first error in a bad text. The suite already
compared its code and span, and the catalog adds only parameters. So the
template of the general syntax error names the token found, and not what was
expected.

**Rendering** (`10-7`). Each form is a function of a diagnostic and the source
text. Every binding gives all three, and the CLI prints them (B.11).

| Form | Layout |
| --- | --- |
| One line | `error[`, the code, `] `, then the span as start line `:` start column `-` end line `:` end column, then `: ` and the message. With a path that has a step: a space, `at`, a space and the path. With an origin: the span is the one in the document, with the origin's name and `:` before it. With no span, as from the conformance function: `error[`, the code, `]: ` and the rest |
| Detail | Line 1: the one-line form. Line 2: four spaces, then the source line that holds the start of the span, with each tab written as one space. Line 3: four spaces, one space for each column before the start, then one `^` for each column of the span on that line, at least one. Then `  help: ` and the help, if there is one. Then one line for each trace entry, the innermost first: `  in `, the name, `, element `, the index, `, at ` and the span; the `element` part is left out when the index is null. No color. With no source text, line 1 alone |
| JSON | One object: `phase`, `code`, `class` (for a run error), `message`, `span` as `[1, 37, 1, 52]`, `path`, `data` and `trace`, a list of `{span, name, index}`. A run diagnostic always has `trace`, which may be empty. Any other field that has no value is left out. `help`, `fix`, `expected`, `found`, `cause` and an offset may be added. It is written in the JSON form of `02`, 2.13: no white space, keys in code point order. Where that form cannot write a part of `data`, such as an opaque value, that part is null |

- **A path** is written from its steps: `.` and the key when the key is a
  `Word` (`02`, 2.2), `[`, the key's JSON form and `]` for any other key, and
  `[3]` for an index. A key is cut to 64 code points, as a parameter is.
- **The word is always `error`.** There is no severity ("Check classes",
  below).
- **Several diagnostics** are rendered one after another in the order of `03`,
  2.13, each form ending in a line feed.
- **Columns are code points.** A wide character puts the carets out of line in
  a terminal. That is accepted.

```
error[number-range] 1:37-1:52: the result of * is not a finite number
    const total = o => o.lines.map(l => l.qty * l.price).sum();
                                        ^^^^^^^^^^^^^^^
  in map, element 1, at 1:20-1:53
  in total, at 2:17-2:25
  in map, element 2, at 2:1-2:26
```

That is the detail form for the program below, when a product in the second
line of the third order is not finite. The message is an illustration.

```ts
const total = o => o.lines.map(l => l.qty * l.price).sum();
orders.map(o => total(o))
```

**What a conformance test compares:** the phase, the code, the class of a run
error, the path of a `not-conforming`, the start and end line and column in the
source text, the message, whole, the `data` of a `failed`, with the language's
`==`, and the trace, entry by entry. It does not compare `help`, `fix`,
`expected`, `found`, `cause` or offsets, and for a limit error it compares
neither span nor trace. For `function-error` the message is the text the stub
gave, and for `failed` the text given to `fail` (A.11). A separate kind of case
compares the three renderings as text.

**`not-conforming`** (`07`, 2.3). A failed `x as T` is a run error with this
code and class `value`, at the span of the `as` node, and `catch` catches it.
The conformance function of A.13 returns the same diagnostic with no span.

- **Decision: the path is a field, as data.** `07` left open whether it is a
  field or part of the message. It is a field, `["lines", 3, "qty"]`, for three
  reasons. The walk's order is specified so that every implementation reports
  the same first failure (`07`, 2.3), and a field is compared exactly, where a
  message is cut at 64 code points for each parameter and would lose the end of
  a long path, which is the part that matters. A host maps the place onto its
  own document: a form field, a line of a request body. And a list of steps
  needs no quoting rule for a key that holds a dot, a bracket or a quote.
- **The last step names what failed.** A required field that is missing: its
  key. A key an exact record does not allow: that key, the least by code point
  when there are several. A value of the wrong kind: the step that reaches it.
  For a tagged union the tag is read first, and a value with no tag, or with a
  tag in no variant, fails at the tag (`09:2.7`).
- **The message does not hold the path.** It says what was expected and what
  was found, by one of the entry's three templates. The one-line rendering
  adds the path, as `at .lines[3].qty`. The fields `expected` and `found` hold
  the type wanted at the path and the kind met there, and they are not
  specified.
- **What a failure holds is bounded** (`04`, A.4). The path has at most 64
  steps, one for each level of the type, and each step is an index or a key
  that already exists, so the path copies nothing. A message is bounded by its
  catalog entry.
- **The class is `value` at a site.** An `as` whose subject already has the
  type is not a site, and it can fail only when a host broke the promise of
  A.3. That failure is `environment`. `07` left the case open, and `03`, 2.14
  now rules it.

**Check classes.** Every check diagnostic is an error, and there is no severity
field. What a host can vary is the set of check classes its environment rejects
(A.2). Four codes depend on it: `always-truthy` and `always-falsy` are reported
only while `constant-condition` is rejected, which it is by default, and
`unchecked` and `may-fail` only for a site class the host has added. The class
field says which class produced the diagnostic, so a host can tell an author
"this is an error here because this host rejects `index`". Every other check
code is reported under every set.

### A.7 Templates

- **Form.** Text with `${{ expr }}` segments. `$${{` writes a literal `${{`. The
  end of a segment is found with the language's lexer, because a map literal can
  end in `}}` and a string can contain it. That lexer reads raw strings
  (`08:2.4`), in which nothing escapes a delimiter: the segment `${{ r"\" }}`
  ends after the second quote, and the segment `${{ r"}}" }}` holds a string of
  two braces. A splitter that only steps over `\"` is wrong.
- **The string forms of `08` belong to segments, not to the text.** Outside a
  segment the text is the host's, and `"""` and `r"` there are characters. In a
  segment stands an expression, so every form may be written. A text block
  needs line ends, so it fits only where the host's text has lines (A.6,
  "Origin"). These templates are what `02` and `08` call host strings. The
  backtick literal and the text block of the language are another thing: there
  `${{` is `${` and then a map.
- **A template is a program whose value is a string.** It is analyzed, checked,
  compiled and run like any other, and its spans are positions in the template
  text.
- **Exactly one segment and no text** is available to the host as that segment's
  own program, so the host can keep the value's type (`01`, lesson 9).
- **Rendering** is the rule of `string(v)` in `02`, 2.13: a string as it is, a
  number in its shortest form, null as empty, a list or map as compact JSON in
  key order. Building the text is charged to both budgets.

### A.8 Analysis

| Result | Definition |
| --- | --- |
| Names | the free variables the program reads. Lambda parameters and `const` names are not free. A name in call position is a call |
| References | each read of a free variable with its static path: `input.event["id"]` gives `input`, `event`, `id`. A step of the path is a key or a star. The path stops at a computed index or a call |
| Calls | each call of a free function name, with its arguments as programs that can be analyzed, checked and compiled alone, and whether it sits inside a lambda |
| Literal | the value, when the whole program is a literal: null, a bool, a number, a string, or a list or map of literals |
| Segments | for a template: the literal runs and the segment programs, each with its span |

Analysis is syntactic. It gives the same answer in every environment.

**A star is a step of a static path** (`O:D51`; `13`, 5.2). The projection
step, `[*]` or `?.[*]`, does not end a path, as a computed index does. The
reads after it are reads of each element, and the path goes on through them.

| Program | Path of the reference to `orders` |
| --- | --- |
| `orders[*].lines[*].sku` | star, `lines`, star, `sku` |
| `orders[*].lines` | star, `lines` |
| `orders[*].lines[0].sku` | star, `lines`. It stops at the index |
| `orders[*].lines[*].qty.sum()` | star, `lines`, star, `qty`. The method call ends the projection and the path |
| `orders.map(o => o.id)` | empty, as before. The path stops at the call |
| `orders["*"]` | the key `*`, which is not a star |

- **A star is not a key.** Every binding gives a step a form in which the two
  cannot be confused (B.1, Parts C and D).
- **`.` and `?.` give the same step,** and so do `[*]` and `?.[*]`: a path says
  what is read, not what happens on null.
- **What a host gains.** A host that reads the paths to learn which fields of
  its data a program uses now sees through nested lists. Before, it saw
  `orders` and nothing after it, because the fields were read inside a lambda.
- **A call in an index read after a star is not inside a lambda.** In
  `orders[*].lines[pick(n)]` the call of `pick` is evaluated once for each
  order. No name is bound there, so its arguments are the same each time, and a
  host can answer it beforehand.

**A string literal is a literal in every form** (`O:D46`; `08:2.5`): `"..."`,
`'...'`, a raw string, a text block and a backtick literal with no `${ }` all
give the Literal result, with the value the parser fixed. A text block's value
has its indentation removed and its line ends as U+000A. A literal with a
`${ }` is not a literal, in either form.

**The calls are complete** (`O:E9`). A function is not a value: it cannot be
bound, passed or returned, and a lambda is passed only to a library function or
method. So a host function runs only where the text calls it by name, and a run
calls no host function that Calls does not list. "Inside a lambda" covers a
callback's body and the body of a `const`-bound helper. Such a call may run many
times or not at all, with arguments that exist only during the run, so the host
cannot answer it beforehand (Part C, "no async run").

### A.9 The safety contract

A program can read the variables its environment declares, call the functions
and methods its environment declares, and build new values. It can do nothing
else.

| Property | Guaranteed | Not guaranteed |
| --- | --- | --- |
| No I/O | The language and its library read no file, network, clock, environment or random source | What a host function does |
| No mutation | The evaluator never writes to a value it did not create. The language has no mutating operation. It has no assignment in 3.0: the words are reserved (`O:X3`, `O:X6`), and a reassigned local would not change this row | A host function is bound by contract not to mutate its arguments (`O:S2`); nothing enforces it. A result may alias host data |
| Determinism | The same program, version, values and host function results give the same value, or the same error code, span, message and trace, and the same usage. A config file is a program, so a load is a function of its files and its variables (`12:3.6`) | A run stopped by time or cancellation. The span and the trace of a limit error |
| Steps | The work the language and its library do is proportional to the steps charged. A run stops with `steps-exceeded` before the step that would pass the budget, at the same point in every implementation and version | The time one step takes. Work inside a host function beyond what it charges |
| Allocation | The total size in units of every value the language and its library create in a run never passes the budget. The charge comes before the memory is taken | Live memory, which depends on a collector. The bytes behind one unit, which each implementation documents as a constant. Host data, and what a host function builds without charging |
| Time | A run that passes its time limit, or is cancelled, stops within one check interval of steps | That a blocked host function returns. That two runs stop at the same point |
| Depth | The implementation's own stack does not overflow on any program or data | – |
| No crash | A run does not crash or hang on any program or any values. An unexpected failure in host code is returned as `function-panic`, and one in the implementation itself as `internal-error`. `catch` catches neither (A.6) | – |

The limits make accidents cheap. They are not a sandbox for hostile code
(`O:L8`).

### A.10 Versions

- **One number,** `major.minor`, for grammar, semantics, cost table and library.
  A minor version only adds. It never changes the result or the usage of a
  program that was valid before.
- **An environment states the version it wants.** Entries and syntax added later
  do not exist in it. Check reports newer syntax as `needs-version`, so parsing
  stays independent of the environment.
- **An implementation offers every minor version of its major, up to its
  newest.** Hiding entries by the version that added them is cheap (`06`, 2.11).
- **An unknown version is refused** with `unsupported-version`. Falling back
  would let a host store programs it wrongly believes are checked.
- **The default is the newest.** A host that stores programs states a version
  and stores it with them. A host with more than one implementation states the
  newest they all have.

Alternative: separate versions for language and library. Why not: a host would
pin two numbers and test their combinations (`01`, lesson 10).

### A.11 Case files

The suite is data (`O:P4`). Since `O:D49` its format is also the author's
format: a program is tested with a file of cases, and the language has no test
construct (`11`). A case file is JSON, so no implementation needs a second
parser.

```json
{
  "file": "discount.risor",
  "env": "{order: {total: float, coupon: string | null}, tier: \"gold\" | \"basic\"}",
  "functions": {
    "rate": {"type": "(code: string) => float !",
             "calls": [{"args": ["SAVE10"], "result": 0.1},
                       {"error": "unknown coupon"}]}
  },
  "returns": "float",
  "cases": [
    {"name": "gold with a coupon",
     "vars": {"order": {"total": 200, "coupon": "SAVE10"}, "tier": "gold"},
     "expect": {"value": 153, "steps": 14}},
    {"name": "an unknown coupon fails the run",
     "vars": {"order": {"total": 200, "coupon": "NOPE"}, "tier": "basic"},
     "expect": {"phase": "run",
                "diagnostics": [{"code": "function-error", "class": "host",
                                 "message": "unknown coupon"}]}},
    {"name": "no coupon stays under 50 steps", "limits": {"steps": 50},
     "vars": {"order": {"total": 200, "coupon": null}, "tier": "basic"},
     "expect": {}},
    {"name": "a misspelled field is a check error",
     "source": "order.totl * 2",
     "expect": {"phase": "check",
                "diagnostics": [{"code": "no-field", "span": [1, 7, 1, 11]}]}}
  ]
}
```

The value, the counts and the span are illustrative.

**The file.** A case file is one JSON object: a **header**, and `cases`, a
list. A JSON array is a file with no header. So every file written as an array
stays valid, and so do the `cases` of an entry of `library.json` (`06`, 2.11)
and of `messages.json` (A.6).

**The header rule** (`11-2`). The header may hold any field of a case but
`name`, `skip` and `expect`. A case is the header with the case's own fields
laid over it. A field of the case replaces the header's field of that name,
whole. Nothing is merged. `source`, `file`, `template` and `conform` count as
one field here.

| Field | Meaning | When absent |
| --- | --- | --- |
| `name` | The case's name. Unique in the file | Required |
| `source` | The program text, in place | Exactly one of these four is present, or the field of a kind only the suite has (below) |
| `file` | A path to the program text, relative to the directory of the case file, with `/` between its parts. Spans are positions in that text | |
| `template` | A template text (A.7) | |
| `conform` | A type and a value, with no program: a case of the second kind (below) | |
| `version` | The specification version (A.10) | The base's |
| `env` | One record type in type text. Its fields are the variables | No variable is added |
| `types` | Named types (A.2): a list, in order, of `{"name": "Order", "type": "..."}` or `{"name": "Order", "schema": {...}}` | None is added |
| `opaque`, `methods` | Opaque type names and method signatures (`03`, 2.16) | None is added |
| `functions` | Host functions, stubbed (below) | None is added |
| `groups` | The optional library groups that are on (A.2), as a list of names: `["math", "json"]`. Without it no case of an opt-in group could run under `risor test` | The base's |
| `reject` | The whole set of rejected check classes (A.2), as in the description. With `"source": "r && r.ok"` and `"env": "{r: {ok: bool}}"`, a case expects `always-truthy` at `[1, 1, 1, 2]`; with `"reject": []` added, the same case checks and gives `bool` | The base's |
| `returns` | The expected result of A.3: a type text, or the word `condition` | Nothing is expected |
| `vars` | A JSON object: a value for each variable the case supplies | No variable is supplied |
| `limits` | `steps`, `allocation` and `depth`. Each one given replaces the base's. Time cannot be given: a case must not depend on a clock | The base's |
| `precondition` | `false`: do not test the fixtures (step 5 below). For the suite's cases that break a declared type on purpose | `true` |
| `skip` | A reason. The case is not run, and its verdict is `skip` | The case is run |
| `expect` | What is expected (below) | The file is invalid, except in update mode |

- **A field this table does not have makes the file invalid.** So `expcet` is
  reported and never ignored, which is the rule of an exact record.
- **`returns` was `expected`** until `O:D49`. `expect` and `expected` differed
  by two letters and meant different things. The host API's term is still "the
  expected result" (A.3).
- **No field states a tolerance.** `value` is compared with `==`. A case that
  tests an implementation-defined `math` entry rounds in its own program (`06`,
  2.8).

**Stubs** (`11-3`). A host function in a case file is a table from arguments to
a result or a failure. It replaces the three fixed functions that every harness
had to provide.

| Matter | Rule |
| --- | --- |
| Form | `functions` maps a name to a signature text, or to `{"type": signature, "calls": [row, ...]}`. A signature alone is a stub with no rows, which is enough for a case that is only checked |
| A row | `args`, a list of JSON values, and exactly one of `result`, a JSON value, and `error`, a string. Optional: `charge` and `allocate`, two whole numbers the stub passes to the call handle before it answers (A.4) |
| Matching | Rows are tried in order. A row with no `args` matches every call. A row with `args` matches when the call has that many arguments and each is `==` to the row's (`02`, 2.7). The first row that matches answers |
| `result` | The stub returns the value |
| `error` | The stub returns an error with that text. The run sees `function-error`, class `host`, and the text is the message and the cause (A.4) |
| No row matches | The stub returns an error, and the case's verdict is `fail` with the difference `functions`, whatever the run then gives. A `try` in the program cannot hide it |
| Where it is declared | In the case's environment, a child of the base. A name the base declares is replaced in the child only (A.2). With no `type`, the base's signature and type rule are kept and only the implementation is replaced |
| Cost | Matching is the runner's work and charges nothing. A stubbed call costs the 1 step of any call plus its row's `charge` (`04`, A.3). So the counts of a case do not depend on the implementation |
| What data cannot say | A function that panics, and one that blocks until it is cancelled. Each harness keeps cases of its own for those |

**What is expected** (`11-4`). `expect` is a **result** or a **failure**. A
failure has `phase`.

| Field of a result | Compared with | How |
| --- | --- | --- |
| `type` | The type the check gives | Both are parsed and compared by structure (`03`, 2.16) |
| `sites` | The sites of the check, each a class and a span (A.2) | The whole list |
| `names` | The names analysis gives (A.8) | The whole list |
| `value` | The value of the run | With the language's `==` (`02`, 2.7). So `2` equals `2.0`, and the order of keys does not matter |
| `truthy` | The truthiness test on the value (A.3) | `true` or `false`. For a host that checks with `condition` |
| `steps`, `allocated` | The usage of the run (A.5) | Equal |

| Field of a failure | Meaning |
| --- | --- |
| `phase` | `declare`, `parse`, `check` or `run` |
| `diagnostics` | The whole list, in the order the stage reports it (`03`, 2.13). A run failure has one. For phase `parse` the list is the specified start of what the parser reports, and what follows it is not compared (A.6). In each: `code`, required, and any of `class`, `span`, `path`, `message`, `data` and `trace` |
| `steps`, `allocated` | For phase `run`: the usage of the failed run |

- **A field that is present is compared. A field that is absent is not.** So
  `"expect": {}` means: the case checks and gives a result, and nothing more is
  asked. The suite's own cases state everything A.6 says a conformance test
  compares. That is a rule of the suite, not of the format.
- **A failure has one shape,** the one above. The earlier form of this section,
  `{"phase", "code", "span"}`, is gone.
- **`span` is in the source text:** `[line, column, end line, end column]`,
  never a position in a host document (A.6).
- **`message` is compared whole** (A.6). `help` and `fix` are never compared.
  `data` is compared with `==`. `trace` is compared entry by entry, each a
  `{"span", "name", "index"}`.
- **A ceiling on steps or allocation is the case's `limits`.** A run that
  passes it fails with `steps-exceeded` or `allocation-exceeded`, where the
  case expected a result. No field such as `maxSteps` exists. An exact count in
  `expect` changes whenever the program does, so an author who wants only a
  bound uses `limits`.
- **The JSON text of a result is tested by every run case.** For a case with a
  `value`, the harness also takes the binding's JSON text of the result (A.3)
  and compares it with what `json.stringify` gives in the language.

**How a case is run.**

| Step | What happens | A failure here is |
| --- | --- | --- |
| 1. Read | The file is read, and each case is laid over the header | The verdict `invalid` |
| 2. Declare | The case's environment is built: a child of the base | The outcome, phase `declare` |
| 3. Parse | The program is parsed | The outcome, phase `parse` |
| 4. Check | The program is checked, with `returns` | The outcome, phase `check`. Otherwise the check gives `type` and `sites` |
| 5. Test the fixtures | Only when the case will run, and `precondition` is not `false`. Each value of `vars` is tested against its declared type, and each `result` of a stub against its signature's result type when that has no type variable, with the function of A.13 | The verdict `invalid`, with the path. The fixture is wrong, not the program |
| 6. Run | Only when the case has `vars`, or `expect` names what only a run gives: `value`, `truthy`, `steps`, `allocated`, or a failure of phase `run`. A case with neither is only checked | The outcome, phase `run`, with its usage |
| 7. Compare | The outcome is compared with `expect` | The verdict `fail` |

- **The verdict** of a case is `pass`, `skip`, `invalid`, or `fail` with the
  list of what differed. The names in that list are `outcome` (a result where a
  failure was expected, the reverse, or another phase), then the fields above,
  with `diagnostics` as one name, then `functions`. They stand in that order.
- **Also `invalid`:** text that is not JSON or repeats a key, a field the table
  does not have, a name used twice, a `file` that cannot be read, a key of
  `vars` that no declaration names, a stub row with both or neither of `result`
  and `error`, and a stub with no `type` for a name the base does not declare.
- **Why step 5.** A fixture that does not fit its declared type gives a run
  error of class `environment` where it is read, and a passing case where it is
  not (A.3). Either way the test says nothing about the program.

**A case of the second kind** has no program (`07`, section 4). It gives a type
and a value, and expects an answer. It has no stages. An author uses one to
hold a sample payload against a named type.

```json
{
  "name": "conform: the first failure is in the second line",
  "version": "3.0",
  "types": [{"name": "Line", "type": "{sku: string, qty: int}"}],
  "conform": {"type": "{id: string, lines: Line[], ...}",
              "value": {"id": "a", "note": 1,
                        "lines": [{"sku": "x", "qty": 1}, {"sku": "y", "qty": "2"}]}},
  "expect": {"conforms": false, "path": ["lines", 1, "qty"], "steps": 8}
}
```

- **The harness asks twice.** Once through the binding's conformance function.
  Once by running the programs `v as T` and `v is T`, with `v` declared `any`
  and supplied as the value. All three must agree with `conforms`, and the two
  that fail must give `path`, and `message` when the case has one.
- **`steps` is what the walk charges** by `07`, 2.7. Only a run has a count, so
  the harness reads it from the usage of the two runs, less the fixed cost of
  the program's other nodes. The count above is illustrative. It stops at
  `qty`, which is before `sku` in code point order, and never reaches `note`,
  because declared fields are tested before other keys.
- **With `max`, every failure** (A.13). `"max": 3` in `conform` asks the second
  function for at most three, and `expect` then has `failures`: a list in the
  walk's order, each a `path` and, when given, a `message`. The first entry
  must be the `path` the three first-failure tests give. With the type above
  made exact and `"id": 7`, a case with `"max": 3` expects the paths `["id"]`,
  `["lines", 1, "qty"]` and `["note"]`.
- **The value is JSON,** so these cases cannot hold an opaque value, an absent
  variable or a number outside the safe range. Each harness adds cases of its
  own for the host values of its binding.

**Kinds of case that only the suite has.** The same runner runs them. An author
never writes one, and they are documented with the suite.

| Field | The case holds | It expects |
| --- | --- | --- |
| `join`, `fits`, `merge`, `truthy`, `falsy`, `disjoint`, `without`, `flat`, `print`, `schema` | Types, with no program (`03`, 2.16; `09`, section 4) | `03`'s answers |
| `runner` | A case file, as data (`11:2.5`) | `verdicts`: for each case its `name`, its `status` and, for a `fail`, the list `differs` |
| `render` | A `diagnostic` in the JSON form of A.6, a `source` text or none, and an `origin` or none | The three texts of A.6: `line`, `detail` and `json` |

The `runner` cases cover the header rule, each way a file is invalid, stub
matching and its cost, an unmatched call inside a `try`, the fixture test,
which stages run, and the text an update writes. Every binding's helper runs
them, so the three runners agree as the three evaluators do.

**Two entry points** (`11-6`). A case runs in a child of a **base
environment**. The two entry points differ only in the base.

| | The command `risor test` (B.11) | The helper of a binding (B.13, Parts C and D) |
| --- | --- | --- |
| Base | The default environment: the newest version, `core` on, the default limits, no variable | The environment the host passes: its real one |
| Host functions | Stubs only | The host's own, except those the file stubs |
| Variables | `vars` only | `vars`, then the host's variables for a name the case does not give. That is how an opaque value reaches a case |
| `file` | A path | A path, or whatever the host's reader accepts, such as a place in its own document, with the origin of A.6 |
| The same in every implementation | Yes, step counts included | As far as the host's functions and values are |
| Reports to | Standard output and the exit status | The host language's test runner |

The helper is the harness of this section, made public. Each implementation
runs the suite through it, so it is tested by every case the suite has.

**A case file as a program** (`11-7`).

| Question | Answer |
| --- | --- |
| May a case file be a Risor program? | A user's file, yes: `*.test.risor`. Its value must be the object described above. The format is the value, not the syntax |
| How is it loaded? | As a config file is (A.14): parsed, checked with the type of a case file as the expected result, and run in the default environment. The value is then read as the JSON would be. The specification carries the type of a case file as type text |
| May the suite's files be programs? | No. A file that tests the parser cannot need the parser |
| What does it give? | Comments. A raw string or a text block for `source`, so a quote needs no `\"` (`08`). `const` and spread for a fixture that cases share, which is the answer to "nothing is merged". A wrong field is a check error with a position |
| Is a JSON case file a program too? | Yes: JSON with unique keys is valid program text with the same value (`02`, 3.3). So a file can be renamed and then edited |
| May a stub be a lambda? | No. A file's value is data, and no function reaches a host (A.3). A stub stays a table |

**Update mode** (`11-8`) writes actual results into a JSON case file.

| Matter | Rule |
| --- | --- |
| Asked for by | `risor test --update`. In a helper, the environment variable `RISOR_UPDATE=1`. Never on by default, and never implied by a missing `expect`: in an ordinary run that file is invalid |
| A case with no `expect` | It goes through every stage and gets the whole outcome. A result: `type`, `value`, `steps`, `allocated`. A failure: `phase`, and for each diagnostic `code`, `class`, `span`, `path` and `message` |
| A case whose `expect` has the outcome's shape | Each field present is replaced with the actual. A field that is absent stays absent. So an update never makes a case stricter |
| A case whose `expect` has the other shape | As a case with no `expect` |
| The text written | The JSON form of `02`, 2.13. Only the text of that `expect` member changes. Every other byte of the file stays |
| Not written | A file that is invalid, a case that is skipped, a case that failed on `functions`, and a file that is a program |
| Who provides it | The command and the Go helper in 3.0. The TypeScript and Rust helpers may follow |

**It exercises all of Part A:** stages (A.1), clashes, check classes, type
names and the description (A.2), boundary values, the expected result and the
JSON text (A.3), host function typing, errors and charging through stubs (A.4),
limits and usage (A.5), codes, positions, messages, traces and renderings
(A.6), templates through the `template` field (A.7), analysis (A.8), versions
(A.10), the runner itself, and both conformance functions (A.13). The decode
of A.14 and a type taken from a host type are not in the suite: each harness
has cases of its own for its binding.

### A.12 Editor support

The author's experience must be high (`O:C4`), and authors write inside host
documents: a YAML file, or a field in a web form.

| Need | Delivered by | In the model above |
| --- | --- | --- |
| Problems underlined as the author types, with a fix | Check, and spans in the host document | yes |
| The same answer in a browser editor as on the server | The environment description, loaded by the browser's implementation | yes (A.2) |
| Names after a dot, names in scope, and type names after `as` and `is` | **Completion**: an environment, a source text and a position give the candidates | **no**: it needs a parser that recovers from an unfinished program, and the type of the receiver before the cursor |
| The type of the thing under the cursor | **Hover**: the same three inputs give a type and documentation | **no** |
| Colors for a program inside a host document | An editor grammar (B.11). It learns the two string forms of `08`: a text block, and the prefix `r` | outside the model |

Not in 3.0: completion of the keys of a map literal from the expected result,
which a config file would use. Assist would take an expected result as check
does (`12`, section 4).

**Decision: completion and hover are the sixth stage of the model, in 3.0.**
Both work on text that does not parse, so check cannot supply them, and each
host editor would otherwise write its own recovering parser. A web editor calls
them in the TypeScript implementation. A host's language server for its own
document format calls them in Go. A generic server for `.risor` files serves
neither (section 7, question 2).

### A.13 The conformance function

Every binding gives the host the test that `as` and `is` make (`O:V2`; `07`,
2.8). It has two forms: one stops at the first failure, as `as` does, and one
reports every failure.

| Matter | Rule |
| --- | --- |
| Takes | A value, and a type whose names are an environment's. The value is anything a host may supply (A.3) or a run may return |
| Gives | Nothing when the value conforms. Otherwise one diagnostic: code `not-conforming`, class `value`, the path and the message of A.6, no span and an empty trace |
| Rule | "A value conforms to a type" of `03`, 2.14, walked in the order of `07`, 2.3. No coercion. An exact record refuses other keys. Absent is not null. Nothing under `any` is looked at. Against a tagged union the walk reads the tag, then tests one variant (`09:2.7`) |
| Agrees with | `x is T` for every value and type, and `x as T` with its path. The conformance cases hold the three together (A.11) |
| Changes | Nothing. The value is not copied, filled in or trimmed |

- **It is the checker's precondition, made testable.** The promise of `03`
  assumes that every variable conforms to its declared type, and that every
  host function returns a value that conforms to its result type. Until now a
  host could only meet that by care. Now it can test it with the rule the
  checker assumed and not with a rule of its own: on a step's output before it
  is stored, on a function's result in a test suite, on every variable in
  production if it wants to pay for it.
- **The type must describe the value as it is at that moment.** A type built
  from a JSON Schema with the options of `03` 2.15 describes a value after the
  host filled absent properties and applied defaults. The raw value does not
  conform to it. A host tests after it normalizes, or builds the type with no
  option.
- **It needs an environment only for names.** A type with no opaque name and no
  named type can be tested with no environment at all. An opaque name can be
  answered only by the binding that knows which host values carry it.
- **A host value that cannot enter the language does not conform.** A number
  outside the safe range, a NaN or an infinity conforms to no type (`04`, A.3).
  Met by the walk, it is a `not-conforming` failure at its path, not the
  `number-range` of an ordinary read (A.3). So the function has one diagnostic,
  `x is T` never fails, and a host that tests a `float` variable is told of a
  NaN. At a place typed `any` the value is not looked at, and a later read of
  it fails as before.

**Decision: it takes no limits, and it charges nothing.** `07` did not say.

| Question | Answer |
| --- | --- |
| Is it a run? | No. It has no program, no usage and no cancellation signal. The environment's limits are limits of a run and do not apply |
| What bounds its work? | The value. It visits a value at most once for each way the type reaches it, so on data shaped as a tree, as all parsed JSON is, the work is at most the size of the data. It builds nothing when the value conforms, and one bounded message when it does not |
| What bounds its depth? | The type. A type is at most 64 levels deep (`03`, 2.17), so the walk's recursion is bounded by text the host wrote, whatever the data. Cyclic host data cannot make it loop |
| What is not bounded? | A host value in which one list or map is reached by many paths is visited once for each path. The walk keeps no memory of what it has seen. (I) |
| A host that wants the limits and a count | Runs the test as a program: `v as T`, compiled once, with `v` declared `any`. That is a run, with steps, a budget and a deadline, and the same answer |

Alternative: a step limit as an argument, with the count returned. Why not: it
gives the function a second result and a second way to fail, for a bound the
one-node program already provides. The data is in the host's memory and the
host has usually just parsed it, which cost more than the walk will.

**Every failure** (`10-8`). `x as T` reports the first failure, and its cost is
exact because it stops (`07`, 2.3). A host that loads a config file or checks a
request body wants them all. The host's function charges nothing, so it can go
on. Every binding gives a second function for this: `ConformAll` in Go,
`conformAll` in TypeScript and `conform_all` in Rust.

| Matter | Rule |
| --- | --- |
| Takes | A value, a type, and a maximum, which is 1 or more |
| Order | The walk of `07`, 2.3, going on after a failure. A list by index. A record's declared fields in code point order of their keys, then its other keys in code point order. A value is finished before the next is started |
| A failure | One of three shapes: a required field that is missing; a key an exact record does not allow, each one and not only the least; a value that does not conform where it stands. Nothing inside a value of the wrong kind is visited |
| Other keys | Always reached. The first-failure walk reaches them only when every declared field passed |
| Maximum | The walk stops when it has that many failures. A host that must say "and more" asks for one more than it shows |
| Gives | A list of `not-conforming` diagnostics, each as the first function gives it. Empty when the value conforms |
| Agrees with | Its first diagnostic is the one that the first function and `x as T` report, for every value and type. The suite holds them together (A.11) |
| Cost | It is not a run and charges nothing. Its work is bounded by the value and its depth by the type, as above. What it builds is bounded by the maximum |

Alternative: a maximum as a third argument of the first function. Why not: the
first function gives one diagnostic or none, and its result would become a
list for every caller. Alternative: an `as` that reports every failure. Why
not: its cost is exact because it stops.

### A.14 Loading a config file

A config file is a program: `const` lines, then one expression, and its value
is the configuration (`O:D50`; `12`). There is no second grammar and no data
mode. The host reads the value into a type of its own. That is five steps in a
fixed order, and the first four are the stages and functions above.

| Step | Is | Fails with |
| --- | --- | --- |
| 1. Parse the file, with its name as the origin | Parse (A.1) | `parse` diagnostics |
| 2. Check, with the host's type T as the expected result | Check and compile, with the rule of A.3 for where a mismatch is reported | `check` diagnostics. A wrong, unknown or missing field is `result-mismatch` at that entry, that key or that map |
| 3. Run, with the settings as variables | Run | one `run` diagnostic, with its trace |
| 4. Conform the value to T | The second conformance function (A.13) | `not-conforming` diagnostics, each with its path |
| 5. Decode into the host's type | The binding | `not-conforming` with a path: a number too wide for the field, text the field's type refuses, or an opaque value |

- **Step 4 is always made** (`12-3`). A result that came through `any` passes
  the check and may not conform. One walk makes the decode's precondition a
  tested fact. It is not a run and charges nothing.
- **The decode** (`12-4`) does what the host language's usual JSON decoder does
  with the JSON text of the value (A.3), with unknown fields refused. An
  implementation need not make the text. The decode copies: what it gives
  shares no memory with the inputs. Its failures have the code
  `not-conforming` and a path, and their messages are the binding's (A.6). It
  is the binding's own work, as reflection is, and the suite does not test it
  (A.11).
- **Nothing is filled in.** A default comes from the host, which fills its own
  value before step 5; from the file, as `{...defaults, port: 1}` over a
  variable the host supplies; or from `??` where the value is used.
- **The type T** is type text, a JSON Schema, or a type taken from a host type
  where the binding can do that (B.7, Part D). A record with no `...` is exact,
  which is what a config file wants: a misspelled key is an error at the key.
  A type from a JSON Schema is built with the option *closed* of `03` 2.15, and
  without the other two, because nobody normalized the value (`12-5`). This
  differs from a named type meant for `as`, which takes no option (A.2).
- **Settings from outside are variables** that the host declares and supplies
  (`12-6`). No library entry reads the process environment, a file or a clock
  (A.9). A process variable is text, so a host declares, for example, `env:
  map<string>`, and passes the names it documents.
- **Less than the whole language.** A host that wants a file to be data alone
  has two lower levels already: the Literal result of A.8 accepts a file that
  is one literal, with no run, and an environment without the `core` group has
  no library (`12-1`).
- **Imports are not in 3.0.** `import` is a reserved word with a targeted error
  (`12:3.9`). A host layers two files by loading the first and supplying its
  value to the second as a variable.

## 3. Part B: the Go binding

Import path `github.com/deepnoodle-ai/risor/go/v3`, package `risor`. Rules not
repeated here are Part A's.

### B.1 Parse and analysis

```go
// Parse parses one program and reports every syntax error it finds.
func Parse(src string, opts ...ParseOption) (*Expr, error)

// At records where src sits in a host document (A.6).
func At(o Origin) ParseOption

type Origin struct{ Name string; Line, Column, Indent int } // the origin of A.6

// Expr is a parsed program or a part of one. It is immutable.
func (x *Expr) Source() string                   // the exact source text
func (x *Expr) Span() Span                       // its place in the text given to Parse
func (x *Expr) Names() []string                  // sorted and unique
func (x *Expr) Refs() []Ref                      // in source order
func (x *Expr) Calls(names ...string) []CallSite // all calls when no name is given
func (x *Expr) Literal() (any, bool)             // a string literal in any form of 08

// A Ref has Name, Path (the static steps after the name), Partial and Span.
// A CallSite has Name, Args ([]*Expr), Span and InLambda.

// A Step is one step of a static path (A.8): a key, or the star of a
// projection. Star is true for `[*]` and `?.[*]`, and Key is then empty.
type Step struct {
	Key  string
	Star bool
}
```

`Ref.Path` is a `[]Step`. It was a list of field names before `O:D51`; a star
needs a form that the key `"*"` cannot take.

The AST is not public in 3.0. These methods cover the engine's uses
(`workflow:validate.go:827-842`, `workflow:decision.go:120-172`), and a public
AST would freeze a tree `02` is still changing.

### B.2 Types and the environment

```go
// Type is an immutable value. Its String method prints the type text of 03.
type Type struct{ /* unexported */ }

func ParseType(text string) (Type, error) // the portable form
func MustType(text string) Type
func TypeFromSchema(jsonSchema []byte, opts ...SchemaOption) (Type, error)
func Join(a, b Type) Type
func Fits(s, t Type) bool

var Null, Bool, Int, Float, String, Any Type // Null is the type of the language's null

// NewEnv builds an immutable Env. With returns a child (A.2).
func NewEnv(decls ...Decl) (*Env, error)
func (e *Env) With(decls ...Decl) (*Env, error)
func (e *Env) Check(x *Expr, opts ...CheckOption) (Type, error)
func (e *Env) Compile(x *Expr, opts ...CheckOption) (*Program, error)

// Sites lists where a program that passes Check can still fail on a value,
// for the classes e does not reject (A.2). A Site has Class and Span.
func (e *Env) Sites(x *Expr) []Site

// A CheckOption states the expected result (A.3). With none, any result is
// accepted. Check and Compile take the same options and fail alike.
// Expect: the result must fit t through its form, else result-mismatch at
// each place of failure (09:2.6).
func Expect(t Type) CheckOption
var Condition CheckOption       // the result will be tested with Truthy (B.3)

func (e *Env) Lookup(name string) (Symbol, bool)
func (e *Env) Symbols() []Symbol
func (e *Env) Describe() ([]byte, error) // the description of A.2, as JSON

// ParseType on an Env resolves type names (A.2): a named type becomes the
// type it abbreviates, and an opaque name is bound to its Go type.
func (e *Env) ParseType(text string) (Type, error)

// Complete and Hover (A.12) accept text that does not parse.
func (e *Env) Complete(src string, line, column int) []Completion
func (e *Env) Hover(src string, line, column int) (Hover, bool)

// A Symbol has Name, Kind, Type, Doc, Since (a version) and Path (B.4).
// A Decl is one declaration: each value below, Function (B.4) and Limits.
func Version(v string) Decl                  // "3.0"; the newest when absent
func Var(name string, t Type) Decl           // one variable
func Declare(recordType string) Decl         // one variable per field, in type text
func NamedType(name string, t Type) Decl     // "Order": a name for t, for `as` and `is`
func Func(name string, impl any) Decl        // Function{Name: name, Impl: impl}
func Groups(names ...string) Decl            // optional library groups: "math"
func Without(names ...string) Decl           // remove entries or groups: "reduce", "core"
func Reject(classes ...Class) Decl           // add check classes to the rejected set (A.2)
func Allow(classes ...Class) Decl            // take them out: ClassConstantCondition
func Description(data []byte) (Decl, error)  // everything Describe wrote; check only

type Limits struct { // a Decl
	Steps   int64         // 0 is the default, 1,000,000. NoLimit turns it off.
	Alloc   int64         // units. 0 is the default, 1,000,000. NoLimit turns it off.
	Timeout time.Duration // for one run. 0 is none.
	Depth   int           // 0 is the default, 200.
}

const NoLimit = -1
```

- **Decision: `Type` lives in the root package** (`O:S5`), with `03`'s
  constructors and accessors (`ListOf`, `RecordOf`, `Elem`, `Fields`) beside it
  under names that do not collide with `Map` and `Func`. Alternative: a `types`
  package, as `03` drafted. Why not: `Env.Check` returns a `Type`, so every host
  would import both.
- **A tagged union** (`O:D47`). `Type` gains two accessors: `Tag() (string,
  bool)`, the tag of the union in its map kind, and `Variants() []Type`, its
  records in printing order. For a type whose map kind is one record, `Tag`
  reports false and `Variants` gives that record alone. The record accessors,
  `Fields` among them, answer for the flat record of `09:2.3`, which is what a
  read sees. No constructor makes a tagged union from records: a host writes
  type text or a schema, and `ParseType` returns `bad-type` when the records
  have no tag. `TypeOf` (B.7) cannot derive one, because Go has no such type.
- **Schema options.** A `SchemaOption` is one of the three options of `03`
  2.15: `Closed`, `AbsentIsNull` and `DefaultsApplied`. A loader passes
  `Closed` (A.14). A named type for `as` passes none (A.2).
- **Decision: declarations are values, and `Env` is immutable.** One `Env`
  serves every goroutine. `NewEnv` returns an `*Error` of phase `declare`; expr
  defers such errors to `Compile` (`expr:engine.go:130-146`). Alternative: a
  mutable builder, where a program compiled before a later declaration disagrees
  with its `Env`.
- **Defaults.** The `core` group is on unless `Without("core")` is given; v2
  starts empty (`risor:risor.go:74-75`). `Timeout` and the context combine: the
  run stops at whichever comes first. The rejected set is
  `ClassConstantCondition` alone unless `Reject` or `Allow` is given.
- **Decision: a named type is `NamedType(name, t)`, and takes a `Type`.** The
  type comes from `MustType` or from `TypeFromSchema`, so one declaration
  covers both portable forms. `TypeFromSchema` with no `SchemaOption` is the
  default `07` asks for: a host that builds a variable's type with options, as
  the engine does, builds the named type from the same schema without them.
  Alternative: `TypeName`. Why not: it reads as a question about a type, and
  `Var` and `Func` already name the thing declared. A `Type` from the package's
  own `ParseType` may hold a name. `NewEnv` resolves it, in declaration order
  for named types, and a name it cannot resolve is `bad-type`. A host that
  wants a resolved `Type` in its hand, for `Fits` or `Conform`, calls
  `env.ParseType`.
- **Decision: the expected result is an option of `Check` and `Compile`, and
  the rejected classes are declarations.** That is the split of Part A: the
  first varies by field and the second by host. `Condition` is a `CheckOption`
  and not a `Type`, because `condition` is not a type (A.3): it cannot be given
  to `Var`, `Fits` or `Join`. `Reject` and `Allow` mirror `Groups` and
  `Without`, and `env.With(risor.Allow(...))` is the child for a re-check (B.8,
  case 4). A `Class` that is not a check class makes `NewEnv` fail with
  `bad-class`.

### B.3 Values and running

```go
// Vars supplies variable values to one Run (A.3).
type Vars interface{ Var(name string) (value any, ok bool) }

type Map map[string]any // a Vars; the conversion risor.Map(m) is free

type Program struct{ /* unexported; safe to run from many goroutines */ }

func (p *Program) Run(ctx context.Context, vars Vars) (any, error)
func (p *Program) Measure(ctx context.Context, vars Vars) (any, Usage, error)
func (p *Program) Type() Type
func (p *Program) Expr() *Expr

type Usage struct{ Steps, Alloc int64 } // also on *Error for a failed run

// Eval parses, checks, compiles and runs src once. Each key of vars is
// declared any, and the core library is on.
func Eval(ctx context.Context, src string, vars map[string]any) (any, error)

func AsInt(v any) (int64, bool)     // v is a whole number in the safe range
func AsFloat(v any) (float64, bool) // v is a number

// Truthy is the language's truthiness test (A.3), for a host that checked
// with Condition. It takes any value Run can return and cannot fail.
func Truthy(v any) bool

// Conform tests v against t by the rule of 03 2.14 (A.13). It returns nil
// when v conforms. Otherwise it returns an *Error of PhaseRun that holds one
// diagnostic: not-conforming with its Path. It is not a run: it takes no
// context and no Limits, and reports no Usage.
func Conform(v any, t Type) error

// ConformAll goes on after a failure (A.13). It returns at most max
// diagnostics, in the walk's order, and none when v conforms. The first is
// the one Conform reports. max must be 1 or more.
func ConformAll(v any, t Type, max int) []Diagnostic

// JSON gives the JSON text of a result (A.3): the JSON form of 02 2.13, which
// is not what encoding/json writes. It fails on an opaque value.
func JSON(v any) ([]byte, error)

// Decode is step 5 of a load (A.14). dst is as for json.Unmarshal. It does
// what json.Unmarshal does with JSON(v), with unknown fields refused, and
// need not make the text. A field the value lacks keeps what dst held.
func Decode(v any, dst any) error

// Load is steps 2 to 5 of a load with T's type: Compile with
// Expect(TypeOf[T]()), Run, ConformAll, Decode.
func Load[T any](ctx context.Context, env *Env, x *Expr, vars Vars, dst *T) error
```

- **Decision: `Vars` is a one-method interface.** It covers a map, a struct
  (`Bind`, B.7) and computed values, at the spike's cost of one lookup per name
  (`risor:experiments/v3/spike/spike.go:1211-1217`). Alternative: `Run(ctx, env
  any)` as in expr (`expr:program.go:274`), which cannot be lazy.
- **Decision: `Run` returns the value, and `Measure` also returns the usage.**
  Alternative: one method with a result struct. Why not: the common call wants
  one value.
- **Numbers in.** Every Go integer and float type is read. An `int64` or
  `uint64` outside the safe range is `number-range` (A.3). The struct tag option
  `,string`, as in `encoding/json`, presents such a field as a string.
- **Numbers out.** A number arrives as `int64` or `float64`. The language does
  not tell them apart (`O:N1`), so a host calls `AsInt` or `AsFloat` and does
  not assert a Go type. I assume `04` keeps this rule: a selected number keeps
  its kind, and a computed one is an `int64` exactly when its operation is typed
  `int` (`O:N5`), so `4 / 2` is `float64(2)`.
- **Other values.** A selected value is the host's own. A built value is Go's
  `nil` for the language's null, `bool`, a number, `string`, `[]any` or
  `map[string]any`.
- **`Truthy`.** It is false for Go's `nil`, `false`, a zero of any number type,
  `""`, and a slice, array or string-keyed map of length 0, whether built or
  the host's own. Go's four kinds of nil are all falsy: a nil pointer and a nil
  interface are null, and a nil slice and a nil map have no elements. So the
  answer does not depend on whether a host left a slice nil or made it empty
  (`O:B5`). A struct is a record, and one with an exported field is never
  falsy. Any other value is opaque and truthy. The cost is one type switch,
  and reflection only for the host's own types.
- **`Conform`.** It reads `v` as a run would, through the one type switch of
  `04`, so a struct is tested by its fields and a typed slice needs no
  conversion first. `t` must have its names resolved: a type from
  `env.ParseType`, from `TypeOf`, or one with no type name in it. An unresolved
  name gives an `*Error` of `PhaseDeclare` with `bad-type`, because the fault
  is the host's and not the value's. `3.0` conforms to `int` and stays a
  `float64`, so a host still reads a number with `AsInt` (`O:N1`).
- **`ConformAll`** returns diagnostics and not an `error`, because a list of
  failures is the answer and not a fault. Each has `Path` and `Message`, and no
  span. It panics on a `max` below 1 and on a type with an unresolved name,
  which are mistakes in the host's code (I).
- **`Decode` and `Load`** (`12-4`). `Decode` follows `encoding/json`: field
  names by the `json` tag, a number into any numeric field it fits, text into
  a type that implements `encoding.TextUnmarshaler`, so a `time.Time` field
  takes RFC 3339 text. A number too wide for its field, text the field's type
  refuses and an opaque value are each `not-conforming` with a `Path`, in an
  `*Error` of `PhaseRun`. `Load` returns the first `*Error` of steps 2 to 5.
  For step 4 it holds every diagnostic `ConformAll` gave, up to 20 (I). A host
  that wants another type than `TypeOf[T]()`, such as one from a schema with
  `Closed`, calls the four steps itself (B.8, case 6).

### B.4 Host functions

```go
// Function declares a host function. It is a Decl.
type Function struct {
	Name string // "slug", or "geo.distance" in a namespace
	// Impl is a Go function. Its signature gives the type. The first
	// parameter may be context.Context or *Call. A Native needs Type or Rule.
	Impl any
	Type Type                             // optional: replaces the derived type
	Rule func(c *CallCheck) (Type, error) // optional: types each call site
	Doc  string
}

// Native is the unconverted form.
type Native func(c *Call, args []any) (any, error)

// Call is the call handle of A.4.
type Call struct{ /* unexported */ }

func (c *Call) Context() context.Context
func (c *Call) Charge(steps int64) error   // return the error unchanged
func (c *Call) Allocate(units int64) error // before building the result

// CallCheck is what a Rule sees. An Arg has a Type and an Expr.
type CallCheck struct{ Args []Arg; Span Span }
```

- **Typing.** The Go signature needs nothing else; it may be variadic, and `(T,
  error)` is `!`. `Type` states what Go types cannot, such as a result that may
  be null. `Rule` sees argument expressions, so the engine need not compare
  `*Type` pointers to tell steps apart (`workflow:typecheck.go:613-628`).
- **Call paths** (`O:S4`). Registration accepts any Go function. It picks a
  generic adapter with no reflection for the signatures that have one, and falls
  back to `reflect.Call` for the rest. `Symbol.Path` reports which, so a host
  can test that a hot function avoided reflection: 9 ns against 146 ns in `04`.
- **Blocking and scopes.** A Go host function may wait for I/O and should watch
  `c.Context()`. Scope-dependent typing is a redeclaration in a child `Env`.

### B.5 Diagnostics

```go
// Error is the only error type this package returns.
type Error struct {
	Phase       Phase        // PhaseDeclare, PhaseParse, PhaseCheck, PhaseRun
	Source      string       // the text given to Parse or ParseTemplate
	Origin      Origin
	Diagnostics []Diagnostic // at least one; a failed run has exactly one
	Usage       Usage        // for PhaseRun
}

// The three renderings of A.6. Each gives every diagnostic in order.
func (e *Error) Error() string   // the one-line form, joined by line feeds
func (e *Error) Detail() string  // the detail form; no color
func (e *Error) JSON() []byte    // the JSON form, one object to a line
func (e *Error) Unwrap() []error // the causes, for errors.Is and errors.As

// Diagnostic has the fields of A.6, under those names in JSON: Code, Class,
// Message, Help, Expected, Found, Path, Data, Trace, Span (in Source), Doc (in
// the host document), Fix and Cause. Path is a []any of string keys and int
// indexes, and is nil except for not-conforming. Data is the second argument
// of fail, read-only like a result, and nil for every other code. Trace is
// empty except for a failed run. MarshalJSON writes the JSON form of A.6.
type Diagnostic struct{ /* ... */ }

func (d Diagnostic) Line() string // the one-line form of this diagnostic

// A TraceEntry is one call or one projection in progress (A.6). Index is -1
// for a call of a const-bound lambda, which the JSON form writes as null.
type TraceEntry struct {
	Span  Span
	Name  string // "total", "map", or "[*]" for a star
	Index int
}

type Span struct{ Start, End Position }
type Position struct{ Line, Column, Offset int } // Offset is Go's addition, in bytes

type Code string  // CodeStepsExceeded Code = "steps-exceeded", and so on
type Class string // ClassLimit Class = "limit", and so on. The check classes of
                  // A.2 are Classes too: ClassConstantCondition, ClassIndex
```

**Decision: one `*Error` holding a list, for all four phases.** A host writes
one `errors.AsType` and one loop. v2 has four unrelated error shapes
(`risor:pkg/errors/compile.go:9`, `risor:pkg/errors/structured.go:48`, and the
parser's and validator's own), and expr's type errors are strings with no
position (`expr:check.go:18-20`, `:185-191`). v2's lexer counts runes
(`risor:internal/token/token.go:8-14`) and gains a byte offset.

- **`Error()` is the one-line form** (`10-7`), so a log line or a `%v` shows
  what every other implementation shows. The third revision gave the first
  diagnostic and a count. With several diagnostics the string has several
  lines, and it ends without a line feed, as a Go error should.
- **The messages are generated.** A `go generate` step reads `messages.json`
  and writes the table of templates and the `Code` constants. No message is
  written by hand, and `err.message` inside a program is the same string as
  `Diagnostic.Message` (A.6).
- **A failed run gives exactly one diagnostic. The conformance functions are
  not runs,** but their errors have `PhaseRun`, because `as` gives the same
  diagnostic in a run. `Conform` gives one. `Load` gives the list of
  `ConformAll` when step 4 of a load fails (B.3), so that `*Error` may hold
  several. Neither has a `Usage` or a trace.
- **Color** is the CLI's (B.10). The detail form is plain text everywhere.

### B.6 Templates

```go
func ParseTemplate(text string, opts ...ParseOption) (*Template, error)

func (t *Template) Segments() []Segment
func (t *Template) Expr() *Expr   // the whole template, as a string-valued program
func (t *Template) Single() *Expr // non-nil when it is exactly one segment

// A Segment has Text or Expr, and a Span that includes the delimiters.
```

expr has a separate `Template.Check` and `Template.Render` (`expr:check.go:76`,
`expr:template.go:242`); here `env.Compile(t.Expr())` gives an ordinary
`*Program`. The engine's typed-value rule (`workflow:expr.go:150-175`) becomes
`Single()`. v2's splitter is internal and accepts only `${`
(`risor:internal/tmpl/string_template.go:57`).

### B.7 Conveniences of the Go binding

These derive declarations from Go types by reflection. They are outside the
model (`O:P5`); `Describe` writes their result as type text.

```go
func TypeOf[T any]() Type               // the type of a Go type, by the table in 03
func Fields[T any]() Decl               // one variable per field of struct T
func Bind[T any](p *T) Vars             // reads those fields in place
func Opaque[T any](name string) Decl    // Go type T is the opaque type name
func Method(name string, impl any) Decl // impl's first parameter is the receiver
```

- **Field names.** A field's name is its `risor` tag, else its `json` tag name,
  else the Go name: a host's schema and its struct describe the same document.
  expr needs an option for this (`expr:engine.go:178`).
- **Optional fields** (`12:3.5`). A field whose `json` tag has `omitempty` or
  `omitzero` is optional in the type `TypeOf` gives: `max_body?: int`. Every
  other field is required. Before `O:D50` every field was required, and a
  config file had to write each one.
- **Text types** (`12:3.5`). A Go type that implements
  `encoding.TextUnmarshaler` and has no registered method is `string` in the
  type `TypeOf` gives, so a `time.Time` field takes RFC 3339 text and `Decode`
  parses it (B.3). The same type registered with `Opaque` stays opaque. These
  two rows are `03`'s table for Go types, section 5.
- **How those two are read going in,** when a struct is supplied as a variable
  (`03`, section 5 asks). A field under `omitempty` or `omitzero` that holds
  what `encoding/json` would leave out is absent, and any other value is
  present. So the struct reads as the document it marshals to, and
  `cfg?.max_body ?? 1024` gives the default for a field the host left at zero.
  The price is the one `omitempty` has in JSON: a zero the host meant is absent
  too. A text type is presented as the string its `MarshalText` gives, made when
  the field is read. It is host data and is not charged (A.3). A type that has
  `UnmarshalText` and no `MarshalText` can be decoded into and cannot be read: a
  read of it is `wrong-kind`, of class `environment`. (I)
- **An opaque type is named, never guessed.** `Opaque[time.Time]("Time")` puts
  `Time` in the environment's set of type names (A.2) and tells the binding
  which Go values carry it, for a read, for `Method` and for `Conform`. An
  environment built from a description has the name and no Go type, which is
  enough to check.
- **Methods are registered, never discovered.** v2 and expr expose every
  exported method (`risor:pkg/object/go_struct.go:46-47`,
  `expr:program.go:1546`), so a value in scope grants whatever its methods can
  do. Strings, lists and maps take no host methods.

### B.8 Six host cases

```go
// 1. Evaluate one expression once.
ok, err := risor.Eval(ctx, `user.age >= 18 && "admin" in user.roles`,
	map[string]any{"user": user})

// 2. Compile at save time, run many times against changing data.
env, err := risor.NewEnv(
	risor.Version("3.0"),
	risor.Declare(`{input: {items: {status: string}[]}, limit: int}`),
	risor.Var("user", risor.TypeOf[User]()), // Go convenience
	risor.Func("geo.distance", distance),    // func(a, b Point) (float64, error)
	risor.Groups("math"),
	risor.Limits{Steps: 100_000, Timeout: 50 * time.Millisecond},
)
x, err := risor.Parse(src, risor.At(risor.Origin{Name: "rules.yaml", Line: 41, Column: 11}))
prog, err := env.Compile(x) // every problem, with document positions
v, usage, err := prog.Measure(ctx, risor.Map{"input": payload, "limit": 3, "user": &u})

// 3. Check a `when:` field without running. The host has two choices (A.3).
// (a) Truthiness, which is what the workflow engine does. Any type is
// accepted. A result that can never be falsy, or never truthy, is the
// diagnostic always-truthy or always-falsy at the result expression.
_, err = env.Check(x, risor.Condition)
// (b) A real boolean. A result that does not fit bool is result-mismatch,
// so `report && report.verdict == "APPROVE"`, a `bool | null`, is rejected.
_, err = env.Check(x, risor.Expect(risor.Bool))
// Either way the problems arrive as diagnostics, with document positions.
if rerr, ok := errors.AsType[*risor.Error](err); ok {
	for _, d := range rerr.Diagnostics {
		report(d.Doc.Start.Line, d.Doc.Start.Column, d.Code, d.Message, d.Help)
	}
}
// At run time, choice (a) tests the value with the language's own rule.
when, err := env.Compile(x, risor.Condition)
v, err = when.Run(ctx, vars)
if err == nil && risor.Truthy(v) {
	takeEdge()
}
// Choice (b) asserts instead: ok := v.(bool).

// 4. Re-check stored programs after a schema tightens a field from optional
// to required (A.2). `next` is the environment built from the new schema.
// It rejects constant-condition, as every environment does by default, so
// an author who edits a program is told that a test is now dead.
recheck, err := next.With(risor.Allow(risor.ClassConstantCondition))
for _, p := range stored {
	if _, err := recheck.Check(p.Expr, risor.Condition); err != nil {
		flag(p, err) // a real break, such as a field the new schema removed
	}
	// `report && report.verdict == "APPROVE"` passes here. Under `next`
	// it is always-truthy, because `report` can no longer be null.
}

// 5. An HTTP step whose body has no declared schema (A.13; `07`). The step's
// output is {status: int, body: any}. The engine declares the names an
// author may test against. No schema option: nobody normalized this body.
orderT, err := risor.TypeFromSchema(orderSchema)
scope, err := env.With(
	risor.NamedType("Order", orderT),
	risor.Declare(`{fetch: {status: int, body: any}}`),
)
// The author tests the body where it is used:
//   when: fetch.body is Order && fetch.body.total > 100
//   with: (fetch.body as Order).lines.map(l => l.sku)
// Without the test each read of the body is a dynamic site, and a host that
// declares Reject(ClassDynamic) reports it as `unchecked`.
prog, err = scope.Compile(x)
v, err = prog.Run(ctx, vars)
if rerr, ok := errors.AsType[*risor.Error](err); ok {
	if d := rerr.Diagnostics[0]; d.Code == risor.CodeNotConforming {
		failStep("fetch", d.Path, d.Message) // Path is ["lines", 3, "qty"]
	}
}
// The host can ask the same question itself, of any value and before any
// program runs. Here: a step that does declare an output schema, tested
// after the engine has completed its output and before the output is stored.
if err := risor.Conform(output, declaredT); err != nil {
	failStep(step, err)
}

// 6. Load a config file into a Go struct (A.14; `12`). The file is a program
// whose value is the configuration. A setting from outside is a variable.
type Config struct {
	Listen   string    `json:"listen"`
	MaxBody  int       `json:"max_body,omitempty"` // optional in the file
	Backends []Backend `json:"backends"`
	Started  time.Time `json:"started,omitzero"`   // optional; RFC 3339 text
}
env, err = risor.NewEnv(risor.Declare(`{env: map<string>}`))
x, err = risor.Parse(src, risor.At(risor.Origin{Name: "app.risor"}))
cfg := Config{MaxBody: 1 << 20} // the host's defaults
err = risor.Load(ctx, env, x, risor.Map{"env": picked}, &cfg)
// An author who writes `max_bod` gets one check error at that key, before
// anything runs: result-mismatch, in the one-line form
//   error[result-mismatch] app.risor:7:3-7:10: ...
if rerr, ok := errors.AsType[*risor.Error](err); ok {
	fmt.Fprint(os.Stderr, rerr.Detail())
}
// The same load, step by step, with the type from a JSON Schema. A loader
// closes the schema, so a misspelled key cannot pass (A.14).
cfgT, err := risor.TypeFromSchema(configSchema, risor.Closed)
prog, err = env.Compile(x, risor.Expect(cfgT))                // step 2
v, err = prog.Run(ctx, risor.Map{"env": picked})              // step 3
if ds := risor.ConformAll(v, cfgT, 21); len(ds) > 0 {         // step 4
	reportUpTo(20, ds) // the 21st says "and more"
}
err = risor.Decode(v, &cfg)                                   // step 5
```

Case 3 no longer calls `Fits(t, risor.Bool)` and writes its own message, as the
third revision had it. The expected result gives the same answer as a diagnostic
with a code and a span. `Fits` stays for a host that compares types for its own
reasons.

Case 6 is HCL's host case: declared variables in, a tagged struct out (`12`,
section 7). The `picked` map holds the process variables the host documents,
and not the whole process environment: a host that passes everything makes
every variable of the machine an input of the result (`12:3.6`). A seventh
case, testing a program with a case file, is in B.13.

### B.9 What survives from v2

| v2 (`risor:risor.go`) | v3 | Reason |
| --- | --- | --- |
| `WithEnv` (`:146`) | `Var`, `Declare`, `Func`; `Vars` at `Run` | Declaring and supplying are separate. v2 must cross-check the two at run time (`:409`) |
| `WithFilename` (`:154`) | `At(Origin{Name: ...})` | Also carries line and column |
| `WithMaxSteps` (`:226`), `WithMaxStackDepth` (`:242`) | `Limits.Steps`, `Limits.Depth` | On the `Env`, default on |
| `WithLimits` (`:282`) | `Limits.Alloc` | Caps on single values are replaced by one budget on everything the run creates (`O:L3`) |
| `WithTimeout` (`:259`) | `Limits.Timeout` | Kept (`O:S9`) |
| `WithObserver` (`:164`) | `Measure` | It reported VM instructions |
| `WithTypeRegistry` (`:179`), `WithRawResult` (`:210`) | gone | Nothing is converted, and there is no object model |
| `WithSyntax` (`:298`), `WithValidator` (`:324`), `WithTransform` (`:342`) | gone | See below |
| `Builtins()` (`:385`) | `core` on by default; `Groups`, `Without` | The library is pure |

**The syntax allow-list, validators and transforms go.** The allow-list tried to
carve an expression language out of a scripting language and failed
(`proposal:216-239`); in v3 the grammar is the expression language. A
validator's job is done by not declaring a name, or by reading `Names()` and
`Calls()`. A transform would break spans.

**Extension points.** Kept, as declarations the checker sees: host functions,
type rules, registered methods, `Vars` implementations and `Origin`. Dropped, as
hooks a checker cannot type: custom object types and the `Attr*` interfaces
(`risor:pkg/object/object.go:183-242`), `RisorValuer`
(`risor:pkg/object/typeconv.go:205`), the observer, and callables in the
run-time map.

### B.10 Packages and dependencies

Two public packages: `risor`, and `risor/risortest` for case files (B.13),
which imports only `risor` and the standard library's `testing`. The lexer,
parser and AST are in `internal/syntax`, and the generated message table in
`internal/messages`; v2 keeps its own branch. The specification's data files,
`library.json`, `messages.json` and the suite, are read by `go generate` and by
the tests, never at run time. **No dependencies in the core module, tests
included.**
The engine's root package depends only on the standard library and expr
(`proposal:624-626`), so a replacement must add nothing. Today
`risor:pkg/errors/format.go:8` imports wonton for color, and tests use its
`assert`. Color moves to the CLI, a separate module that keeps wonton
(`risor:cmd/risor/go.mod:9`). The tests are question 1 of section 7.

### B.11 The rest of the product

| Part | v2 | Recommendation | Reason |
| --- | --- | --- | --- |
| CLI | 10 commands, about 9,200 lines with tests (`risor:cmd/risor/main.go:55-154`) | **Shrink** to `eval`, `check`, `fmt`, `test` and a REPL that loads JSON as variables | Its job becomes trying a program against data, reading diagnostics, loading a config file and running case files. `dis` has no bytecode; `bench`, `examples` and `lint` serve whole programs. `test` keeps its name and changes its meaning: v2's ran `test_` functions of a script with a `t` object and `assert` (`risor:pkg/testing/runner.go:124-125`), and v3's runs case files (A.11) |
| Script test runner | `pkg/testing` (927 lines) | **Replace** with `risortest` (B.13) | Tests are data (`O:D49`) |
| Language server | about 2,700 lines with tests; reports parse errors only (`risor:cmd/risor-lsp/server.go:66-91`) | **Drop** the server; **add** `Complete`, `Hover` and `Describe` | A.12, and question 2 |
| VS Code extension | grammar and LSP client (`risor:vscode/package.json`) | **Shrink** to the grammar | An injection grammar for `${{ }}` in YAML is the useful addition. The grammar learns the text block and the prefix `r` (`08`) |
| Docs generator | `cmd/risor-docs`, `pkg/docs` (512 lines) | **Shrink** to a `go generate` step over `Env.Symbols()` | One source, as `06` wants |
| Modules | `math`, `rand`, `regexp` as objects (1,643 lines) | **Replace** with namespaces and groups (`O:S1`) | `rand` breaks determinism. The entries are `06`'s |

**The CLI** (`11:2.6`, `11:2.9`; `12:3.7`). Four commands and the REPL. An
implementation that ships a CLI of its own gives it these commands, flags and
exit statuses.

```
risor eval FILE [--type TEXT | --schema FILE] [--format text|json] [--pretty]
risor check FILE [--type TEXT | --schema FILE] [--description FILE]
risor fmt FILE
risor test [PATH ...] [--run REGEXP] [--update] [--format text|json]
risor
```

| Command | Does |
| --- | --- |
| `risor eval FILE` | Parses and runs the file: steps 1 and 3 of A.14. With `--type` or `--schema` it also checks against that type and conforms the value: steps 2 and 4. It prints the value. `--format text`, the default, prints the rendering of `02` 2.13. `--format json` prints the JSON text of A.3 and a line feed. `--pretty` indents and is not specified. There is no YAML or TOML output: pipe the JSON to a converter |
| `risor check FILE` | Steps 1 and 2. `--type TEXT` is the expected result. `--schema FILE` gives a JSON Schema in place of the text, with the option *closed*. `--description FILE` loads an environment description (A.2), and `--type` may then name one of its types |
| `risor fmt FILE` | Rewrites the text. It keeps comments, the order of keys and the written form of every literal, and it does not sort keys. It never changes one string form into another. In a text block it may replace the indentation, which leaves the value as it was (`08:2.5`) |
| `risor test [PATH ...]` | Runs case files on the default base (A.11). A path that is a file is run. A directory is searched for `*.test.json` and `*.test.risor`. `--run` selects cases by name. `--update` is update mode. `--format json` writes each verdict as data |
| `risor` | The REPL. It loads JSON as variables, with the flags below |
| `--var NAME=TEXT`, `--vars FILE.json`, `--env NAME` | For `eval`, `check` and the REPL. A variable that holds the text. One variable for each key of a JSON object. The process variable `NAME` as a key of the variable `env`, declared `map<string>`; one that is not set is absent. The CLI reads no process variable that a flag does not name |
| Diagnostics | Written in the detail form of A.6, and in the JSON form under `--format json`. Color on a terminal is the CLI's addition |
| Exit status | `eval`, `check` and `fmt`: 0 with no diagnostic, 1 with any. `test`: 0 when every case passed or was skipped, 1 when one failed, 2 when a file is invalid |

- **`risor conform` is gone** (`11-9`). The suite is a directory of case files,
  and `risor test <directory>` runs it. The public conformance report is the
  output of `--format json` over the suite. The kinds of case an author never
  writes run through the same command (A.11).
- **`risor test` needs no host code.** Every function a file uses is a stub,
  so its verdicts are the same in every implementation, step counts included.
  A host that tests against its real environment uses the helper (B.13).

### B.12 Migration

From expr, for every call the workflow engine makes:

| expr | v3 |
| --- | --- |
| `Compile(src, opts...)` | `risor.Parse(src)`, then `env.Compile(x)` |
| `Program.Run(ctx, env)` | `prog.Run(ctx, risor.Map(env))` |
| `WithFunctions(map)`, `expr.Func` | `risor.Func`, `risor.Function`, `risor.Native` |
| `Builtins`, `StringFuncs`, `MathFuncs`, `CollectionFuncs` | `core` by default; `risor.Groups("math")`, whose entries are then written `math.sqrt` |
| `WithEvalBudget(n)` | `risor.Limits{Steps: n}`; allocation is now bounded too |
| `NewTemplate` with `WithTemplateDelimiters`, `Template.Segments`, `TemplateSegment.Program`, `Template.Render` | `risor.ParseTemplate`, `Template.Segments`, `Segment.Expr`, `Template.Single`; `env.Compile(t.Expr())`, then `Run` |
| `Program.Identifiers`, `Calls`, `Literal`, `Source` | `Expr.Names`, `Calls`, `Literal`, `Source`. `Call.Bound` becomes `CallSite.InLambda` |
| `Program.Check(env, opts...)`, `Template.Check`, `WithFuncType` | `scope, err := base.With(decls...)`, then `scope.Check(x)`; `Function.Rule` |
| The engine's own test that a `when:` has type boolean, and its assertion `v.(bool)` at run time (`workflow:typecheck.go:784-786`, `workflow:apply.go:1030-1031`) | `scope.Check(x, risor.Condition)`, then `risor.Truthy(v)` (`O:B8`). The engine's rule that a bare `decide(...)` is not a condition (`workflow:typecheck.go:781-783`) stays the engine's: a number passes `condition` |
| `expr.IsTruthy(v)` (`expr:truthy.go:20`) | `risor.Truthy(v)`. The falsy set is the same for the values a run builds. For host values it differs on a pointer to a scalar, which is read as the scalar (`04`): a `*int` holding 0 is falsy, where expr calls every non-nil pointer truthy (`expr:truthy.go:67-68`) |
| At a step boundary the engine validates params, output and input with its own `Schema.validate`, which returns up to ten messages with paths, and then rewrites the value with `Schema.complete`: absent properties become nil, defaults are applied, undeclared keys of a closed object are dropped (`workflow:apply.go:586`, `:637-647`, `:1941-1944`; `workflow:schema.go:561-699`) | Both stay the engine's: they enforce and normalize, and `Conform` does neither. `risor.Conform(v, t)` after `complete`, with `t` the type the checker was given, tests that the two agree. That is the precondition the engine's soundness test probes by fuzzing (`proposal:93-97`), now one call. It differs from `validate` in two ways: it reports a path as data, and it reads absent and null as different. `Conform` stops at the first failure. `risor.ConformAll(v, t, 10)` gives up to ten, as `validate` does (A.13) |
| A step with no `output:` schema: nothing is tested, and its output has expr's `any` type, so every read of it is unchecked | The output is declared `any`, and the author writes `as` or `is` with a type in place or a named type (host case 5) |
| The definition's `schemas:`, named schemas that may refer to each other and may not form a cycle (`workflow:definition.go:27-28`, `workflow:schema.go:165-172`) | One `risor.NamedType` each, built with no schema option and declared in dependency order. A name that is not an upper-case letter and at least one more character is not declared, or is renamed |
| `TypeError.Problems`, `ErrCompile`, `ErrEvaluate`, `ErrType` | `(*risor.Error).Diagnostics` and `Error.Phase` |
| `Type` and its fields, `Join`, `ObjectOf`, `ListOf`, `EnumOf`; `Compile("[]").Check(nil)` for the empty-list element type (`workflow:typecheck.go:1033-1043`) | `risor.Type` with accessors, `risor.ParseType`; `never[]` for the empty list. A type expr printed as `list<string>`, `string?` or `nil` (`expr:types.go:116-144`) prints as `string[]`, `string \| null` or `null` |
| The engine's schema table (`workflow:schema.go:463-545`); a result asserted as `int64` or `float64` | `risor.TypeFromSchema`; `risor.AsInt`, `risor.AsFloat` |

From v2 (options are in B.9):

| v2 | v3 |
| --- | --- |
| `risor.Eval(ctx, src, risor.WithEnv(m))` | `risor.Eval(ctx, src, m)` |
| `risor.Compile`, then `risor.Run` | `risor.Parse`, `env.Compile`, `prog.Run` |
| `WithEnv(risor.Builtins())`; a module such as `math` | nothing: `core` is on; `risor.Groups("math")` |
| A Go function in the env map | `risor.Func(name, fn)` |
| `errors.As` against four error types | `errors.AsType[*risor.Error]` |
| `ErrStepLimitExceeded`, `ErrStackOverflow` | `CodeStepsExceeded`, `CodeDepthExceeded` |
| A script of `test_` functions with `t` and `assert`, run by `risor test` | A case file, run by `risor test` or by `risortest.Run` (A.11, B.13). A guard inside a program is `cond ? x : fail(msg)` (`06`, 3.1) |
| A message matched as text in a host's own tests | The code, which is specified. The message is now specified too, and a `run` message does not change within a major version (A.6) |

### B.13 Case files: the `risortest` package

The helper of A.11 for Go. It is the harness that runs the conformance suite,
made public (`11-6`).

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
// what was wanted and what was got) and Actual, the expect that update mode
// would write.
func Vars(v risor.Vars) Option // variables a case does not give
func Sources(read func(ref string) (string, risor.Origin, error)) Option
```

```go
// 7. Test the programs of a host against its real environment. The file
// stubs `rate`; every other function is the host's own.
func TestDiscount(t *testing.T) {
	risortest.Run(t, hostEnv, "testdata/discount.test.json")
}
```

- **One subtest for each case,** so `go test -run TestDiscount/gold` selects a
  case, and a failure prints the fields that differed and the diagnostic in its
  detail form.
- **`Sources`** lets `file` name something other than a path: a place in the
  host's own document, with its origin, so that a span in a failure is a
  position in that document (A.6).
- **Update mode** is `RISOR_UPDATE=1 go test`. It rewrites the `expect` members
  of JSON case files in place and never makes a case stricter (A.11).
- **The base is never changed.** Each case runs in a child, so one `*Env`
  serves every test of a package, in parallel.
- **`*.test.risor` files** are loaded with `risor.Load` against the type of a
  case file (A.11), so a wrong field is a check error with a position.

## 4. Part C: the TypeScript binding, in sketch

This is a sketch to test Part A, not a final API.

```ts
/** The type notation of 03. The portable way to state a type. */
export type TypeText = string;
// Origin, Span and Diagnostic have the fields of A.6. A Position is
// { line, column, offset }; offset is this binding's addition, in UTF-16 units.
// A Diagnostic's path is (string | number)[], present for "not-conforming".
// Its data is the second argument of fail, present for "failed". Its trace is
// { span, name, index: number | null }[], present for phase "run".
// CallSite, Template, Completion and Hover mirror Part B.

/** A step of a static path (A.8). `star` is a unique symbol, so no key is it. */
export const star: unique symbol;
export interface Ref { name: string; path: (string | typeof star)[]; partial: boolean; span: Span }

/** Thrown by parse, createEnv, compile, run and load. Never by check. */
export class RisorError extends Error {
  readonly phase: "declare" | "parse" | "check" | "run";
  readonly diagnostics: readonly Diagnostic[];
  readonly usage?: Usage;             // for phase "run"
  // The three renderings of A.6. `message` is the one-line form.
  detail(): string;
  toJSON(): unknown;                  // the JSON form, one object per diagnostic
}
/** The same three forms for diagnostics that were returned, not thrown. */
export function render(diagnostics: readonly Diagnostic[], source?: string,
                       form?: "line" | "detail" | "json"): string;

export function parse(source: string, origin?: Origin): Expr;
export function parseTemplate(text: string, origin?: Origin): Template; // as Go's

export interface Expr {
  readonly source: string;
  readonly span: Span;
  names(): string[];
  refs(): Ref[];
  calls(...names: string[]): CallSite[];        // args are Expr values
  /** undefined when the program is not a literal. null is a literal value. */
  literal(): { value: unknown } | undefined;
}

export interface HostFunction {
  /** "(s: string, n?: int) => string !". Required: there is no reflection.
   *  No parameter is a function (A.4). */
  type: TypeText;
  /** Synchronous. A returned Promise is a function-error. */
  call(handle: CallHandle, ...args: readonly unknown[]): unknown;
  rule?(site: { args: { type: TypeText; expr: Expr }[]; span: Span }): TypeText;
  doc?: string;
}

/** charge and allocate throw when a budget is passed; do not catch it. */
export interface CallHandle {
  readonly signal?: AbortSignal;
  charge(steps: number): void;
  allocate(units: number): void;
}

export interface Limits { steps?: number; allocation?: number; timeoutMs?: number; depth?: number }
export interface Usage { steps: number; allocated: number }

/** The check classes of A.2. "constant-condition" is rejected by default. */
export type CheckClass =
  | "constant-condition" | "dynamic" | "index" | "divide" | "value" | "host";

/** The expected result of A.3: a type text such as "bool" or "string[]", or
 *  "condition" for a result the host tests with truthy(). */
export type Expected = TypeText | "condition";

export interface EnvOptions {
  version?: `${number}.${number}`;
  /** One record type, "{user: {age: int}}", or a type text or JSON Schema per name. */
  vars?: TypeText | { [name: string]: TypeText | object };
  /** Named types (A.2): a type text or a JSON Schema per name, taken in key
   *  order, so a later one may use an earlier name. */
  types?: { [name: string]: TypeText | object };
  /** Opaque type names, each with the test that says a value carries it. */
  opaque?: { [name: string]: (value: unknown) => boolean };
  /** Keys may be namespaced: "geo.distance". */
  functions?: { [name: string]: HostFunction };
  groups?: string[];
  without?: string[];
  /** Classes added to the rejected set, and classes taken out of it. */
  reject?: CheckClass[];
  allow?: CheckClass[];
  limits?: Limits;
}

export function createEnv(options?: EnvOptions): Env;
/** An environment built from another implementation's description. Check only. */
export function envFromDescription(description: unknown): Env;

/** The three schema options of 03 2.15. Each is off unless set. */
export interface SchemaOptions {
  closed?: boolean; absentIsNull?: boolean; defaultsApplied?: boolean;
}
/** The type a JSON Schema gives, as type text. A schema passed as an object
 *  in EnvOptions takes no option; a host that wants one calls this first. */
export function typeFromSchema(schema: object, options?: SchemaOptions): TypeText;

export interface Env {
  with(options: EnvOptions): Env;
  /** Problems are data here: an editor calls this on every keystroke. */
  check(expr: Expr, expected?: Expected):
    { type: TypeText; diagnostics: Diagnostic[]; sites: { class: CheckClass; span: Span }[] };
  compile(expr: Expr, expected?: Expected): Program;
  /** The conformance function of A.13. Not a run: no limits, no usage.
   *  null when the value conforms. Throws only for a type text that is bad. */
  conform(value: unknown, type: TypeText): Diagnostic | null;
  /** Every failure, in the walk's order, at most max. [] when it conforms. */
  conformAll(value: unknown, type: TypeText, max: number): Diagnostic[];
  describe(): unknown;                // JSON
  complete(source: string, line: number, column: number): Completion[];
  hover(source: string, line: number, column: number): Hover | undefined;
}

export type Vars =
  | { readonly [name: string]: unknown }
  | ReadonlyMap<string, unknown>
  | ((name: string) => unknown);

export interface Program {
  readonly type: TypeText;
  /** Synchronous. There is no async run in 3.0. */
  run(vars?: Vars, options?: { signal?: AbortSignal }): unknown;
  measure(vars?: Vars, options?: { signal?: AbortSignal }): { value: unknown; usage: Usage };
}

/** The language's truthiness test (A.3). Not JavaScript's: [] and {} are falsy. */
export function truthy(value: unknown): boolean;

/** The JSON text of a result (A.3). Not JSON.stringify: keys by code point,
 *  numbers by 02 2.5. Throws on a class instance that is opaque. */
export function toJSON(value: unknown): string;

/** Steps 2 to 5 of a load (A.14): compile with `expected`, run, conformAll,
 *  and a copy of the value. T is the host's claim, which steps 2 and 4 make
 *  true: TypeScript's types are erased, so `expected` is text or a name. */
export function load<T>(env: Env, x: Expr, expected: TypeText, vars?: Vars): T;
```

The helper for case files is a module of its own, `risor/testing` (A.11). It
imports no test runner: the host passes its own `test`.

```ts
export function defineCases(
  test: (name: string, fn: () => void) => void,  // vitest, jest, node:test
  base: Env | undefined, file: CaseFile): void;
/** loadCaseFile reads a path. caseFile takes parsed data, as in a browser. */
export function loadCaseFile(path: string, options?: CaseOptions): CaseFile;
export function caseFile(data: unknown, options?: CaseOptions): CaseFile;
export interface CaseFile { readonly cases: readonly Case[] }
export interface Case { readonly name: string; run(base?: Env): Verdict }
export interface Verdict {
  status: "pass" | "fail" | "invalid" | "skip";
  differs: { field: string; wanted: unknown; got: unknown }[];
  actual: unknown;                    // the expect an update would write
}
/** vars: variables a case does not give. sources: a reader for `file`. */
export interface CaseOptions {
  vars?: Vars;
  sources?: (ref: string) => { source: string; origin?: Origin };
}
```

Where the two bindings differ in kind, and what Part A says so that both
conform:

| Point | Go | TypeScript | Part A |
| --- | --- | --- | --- |
| Waiting in a host function | It may block on I/O | It cannot: JavaScript has no blocking wait, and a run is one synchronous call | "returns before the run continues" (A.4) |
| Time and cancellation | The context is polled at the interval | During a synchronous run no timer or abort event can fire, so a signal never changes. The evaluator reads the clock at the interval, and checks the signal at the start and after each host function | "how an implementation learns that is its own business" (A.5) |
| Declaring types | Type text, JSON Schema, and reflection over Go types | Type text and JSON Schema only. TypeScript types are erased before the program runs | type text and JSON Schema are the portable forms (A.2) |
| Host values | `map[string]any`, slices, structs by reflection | Plain objects, arrays, `Map` with string keys. Only own enumerable string-keyed properties are fields: the prototype chain is never read, so `x.constructor` is a missing field. A class instance is opaque unless the host declares its fields | "a string-keyed mapping, or an object with named fields"; "opaque" (A.3) |
| Absent and null | A missing key is absent. A nil pointer, a nil interface and a key whose value is `nil` are null | A property that is missing, or that holds `undefined`, is absent. JavaScript's `null` is the language's null, and so is `undefined` anywhere but in a property: as an array element or as a host function's result. A result never contains `undefined` | "each binding says what counts as absent" (A.3) |
| Errors | Returned `*Error` values | A thrown `RisorError`; `check` returns diagnostics as data | a list of diagnostics with one phase (A.6) |
| Numbers | Integer and float kinds in; `int64` or `float64` out | `number` in and out. A `bigint` in the safe range is read as a number, and one outside it is `number-range`. A result is never a `bigint` | the safe-range rule (A.3) |

Smaller differences: the offset unit of a position; sharing, where a program
cannot be sent to a worker but a description can; and invalid text (bad UTF-8
against lone surrogates), which `02` 2.6 settles: each malformed unit reads as
U+FFFD.

**Testing a result** (A.3). This is the binding where the host's own language
has a truthiness rule, and it is not the language's: in JavaScript `[]` and `{}`
are truthy, and here an empty list and an empty map are falsy (`O:B5`). So a
host that checked with `"condition"` writes `if (truthy(v))` and never `if
(v)`. The two agree on null, `false`, `0` and `""`, which is why the mistake
would pass most tests. Go and Rust have no truthiness to confuse it with.

```ts
const env = createEnv({ vars: "{report: {verdict: string} | null}" });
const when = parse('report && report.verdict == "APPROVE"');
env.check(when, "condition").diagnostics;        // []: `report` may be null
env.check(when, "bool").diagnostics[0].code;     // "result-mismatch": bool | null
if (truthy(env.compile(when, "condition").run(vars))) takeEdge();

// The schema now requires `report`. Stored programs are re-checked with the
// class taken out; an editor keeps it in (A.2).
const next = createEnv({ vars: "{report: {verdict: string}}" });
next.check(when, "condition").diagnostics[0].code;                       // "always-truthy"
next.with({ allow: ["constant-condition"] }).check(when, "condition");   // no diagnostics
```

**Named types and `conform`** (A.2, A.13). A type is text in this binding, so
`conform` is a method of the environment: text such as `"Order[]"` means
nothing without the environment that declared `Order`. An implementation parses
a type text once and keeps the result. The `types` option relies on the order of
an object's keys, which JavaScript keeps as written for every key that is not
an integer, and a type name never is one. `conform` reads a value as a run
does, so a property holding `undefined` is absent and fails a required field,
and a class instance conforms only to an opaque name whose test accepts it.

```ts
const env = createEnv({
  types: { Line: "{sku: string, qty: int}", Order: "{id: string, lines: Line[], ...}" },
  vars: "{fetch: {status: int, body: any}}",
});
env.conform(JSON.parse(text), "Order")?.path;    // ["lines", 3, "qty"], or undefined
env.conformAll(JSON.parse(text), "Order", 10).map(d => d.path);   // every failure
env.compile(parse("(fetch.body as Order).lines.map(l => l.sku)"));
```

**Loading a config file** (A.14). The host gives the type as text or as a
schema, and `load<T>` is a claim that steps 2 and 4 make true. There is no
decode to write: the value is plain objects and arrays already, and step 5 is a
copy, so that what `load` gives shares no memory with the variables. A schema
goes through `typeFromSchema` with `closed`, which is the place this binding
has for a schema option.

```ts
const env = createEnv({
  vars: "{env: map<string>}",
  types: { Config: typeFromSchema(configSchema, { closed: true }) },
});
const x = parse(src, { name: "app.risor" });
const cfg = load<Config>(env, x, "Config", { env: picked });
```

**Open for the TypeScript binding: the size of a plain object.** The table above
makes a property holding `undefined` absent. Then an object's size is not its
key count, and "is it empty", which truthiness asks at 0 steps (`O:B5`), must
look past such properties. `truthy()` on a result asks the same question. `len`
of a plain object is a count of its keys in any case, which is work in
proportion to the object, where Go and Rust read a stored length. Two ways out,
to settle when the reference evaluator is built: treat a property holding
`undefined` as present and null, so that emptiness is one probe; and cache an
object's key count for the length of a run. (I)

**Decision: no async run in 3.0.** A host that needs remote data fetches it
before the run, and analysis exists for this: `refs()` and `calls()` say what a
program reads and which functions it calls with which literal arguments. The
engine already works this way for `decide` and `choose`: it finds the calls,
asks the questions, and the functions read saved answers
(`workflow:decision.go:59`, `:120-172`). Alternative: `runAsync`, where a host
function may return a promise. Why not now: it needs a second evaluator in which
every node may suspend. It can be added later as a new kind of program without
touching Part A.

**Types without reflection** (I). Type text is the first route, and the
conformance suite uses it. JSON Schema is the second, and it admits any schema
library that can emit JSON Schema at run time. A third is static: a type-level
parser over a literal type text could type the `vars` parameter of `run`. It is
a convenience of this binding, as `TypeOf[T]` is of Go's.

## 4a. Part D: the Rust binding, in sketch

This is a sketch to test Part A a second time, against a language with no
reflection and no garbage collector. Nothing here was built. (I)

```rust
/// Returned by parse, build, compile, run and load. `check` returns
/// diagnostics as data.
pub struct Error { pub phase: Phase, pub diagnostics: Vec<Diagnostic>, pub usage: Option<Usage> }

/// The three renderings of A.6. `Display` is the one-line form.
impl Error {
    pub fn detail(&self) -> String;
    pub fn to_json(&self) -> String;
}
/// A Diagnostic has the fields of A.6. `data` is an Owned copy of the second
/// argument of fail, because a diagnostic outlives the run. `trace` is a
/// Vec<TraceEntry>, each a span, a name and an Option<usize> index.

pub fn parse(source: &str) -> Result<Expr, Error>;
pub fn parse_template(text: &str) -> Result<Template, Error>;

impl Expr {
    pub fn names(&self) -> Vec<&str>;
    pub fn refs(&self) -> Vec<Ref<'_>>;
    pub fn calls(&self, names: &[&str]) -> Vec<CallSite<'_>>;
    pub fn literal(&self) -> Option<Owned>;
}
pub enum RefStep<'a> { Key(&'a str), Star }       // Ref::path is Vec<RefStep> (A.8)

/// What the evaluator may ask of a host value. There is no reflection, so the
/// host's types answer for themselves. Implemented here for bool, the number
/// types, str and String, Option<T>, Vec<T>, slices, HashMap<String, T>,
/// BTreeMap<String, T> and serde_json::Value. Derived for a struct.
pub trait Host {
    fn view(&self) -> View<'_>;
}

pub enum View<'a> {
    Null,
    Bool(bool),
    Int(i64),
    UInt(u64),                 // read as a number; number-range above 2^53 - 1
    Float(f64),
    Str(&'a str),
    List(&'a dyn HostList),
    Map(&'a dyn HostMap),
    Opaque(&'a dyn std::any::Any),
}

pub trait HostList {
    fn len(&self) -> usize;
    fn get(&self, index: usize) -> Option<View<'_>>;
}

pub trait HostMap {
    fn len(&self) -> usize;
    /// None is "absent" (O:B2). A field that is present and null is Some(View::Null).
    fn get(&self, key: &str) -> Option<View<'_>>;
    /// Keys in any order. The evaluator sorts them when order can be seen.
    fn keys(&self, each: &mut dyn FnMut(&str));
    /// A derived struct answers by field number, resolved once per program.
    fn field(&self, _number: usize) -> Option<View<'_>> { None }
}

#[derive(risor::Host)]            // this binding's convenience, as TypeOf[T] is Go's
struct User { name: String, age: u32, #[risor(absent_if_none)] nickname: Option<String> }

/// What the derive also implements: the type text of the struct. Every field
/// is required, but one marked #[serde(default)] or #[risor(absent_if_none)],
/// which is optional (12:3.5).
pub trait Typed { fn type_text() -> String; }

/// Lent values, a way to charge, and a way to see cancellation.
pub struct Call<'c> { /* ... */ }
impl Call<'_> {
    pub fn charge(&mut self, steps: u64) -> Result<(), Stop>;
    pub fn allocate(&mut self, units: u64) -> Result<(), Stop>;
    pub fn cancelled(&self) -> bool;
}

pub struct Limits { pub steps: Option<u64>, pub allocation: Option<u64>,
                    pub timeout: Option<std::time::Duration>, pub depth: u32 }
pub struct Usage { pub steps: u64, pub allocated: u64 }

/// The check classes of A.2. ConstantCondition is rejected by default.
pub enum CheckClass { ConstantCondition, Dynamic, Index, Divide, Value, Host }

/// The expected result of A.3.
pub enum Expect<'a> { Anything, Type(&'a str), Condition }

/// The three schema options of 03 2.15. `none()` has each one off.
pub struct SchemaOptions { pub closed: bool, pub absent_is_null: bool, pub defaults_applied: bool }

/// A type whose names an environment resolved. Parsed once, then cheap to use.
pub struct Type { /* ... */ }
impl Env  {
    pub fn ty(&self, text: &str) -> Result<Type, Error>;
    pub fn ty_schema(&self, schema: &serde_json::Value, options: SchemaOptions)
        -> Result<Type, Error>;
}
impl Type {
    /// The conformance function of A.13. Not a run: no limits, no usage.
    /// The diagnostic is not-conforming with its path.
    pub fn conform(&self, value: &dyn Host) -> Result<(), Diagnostic>;
    /// Every failure, in the walk's order, at most max. Empty when it conforms.
    pub fn conform_all(&self, value: &dyn Host, max: usize) -> Vec<Diagnostic>;
}
pub enum PathStep { Key(String), Index(usize) }   // Diagnostic::path is Vec<PathStep>

let env: Env = Env::builder()
    .version("3.0")
    .declare("{input: {items: {status: string}[]}, limit: int}")?       // type text
    .var_schema("order", &json_schema, SchemaOptions::none())?          // JSON Schema
    .var_type::<User>("user")                                           // derive
    .opaque::<Point>("Point")                                           // an opaque type name
    .named_type("Line", "{sku: string, qty: int}")?                     // a named type (A.2)
    .named_schema("Order", &order_schema)?                              // no schema option
    .function("geo.distance", "(a: Point, b: Point) => float !",        // no fn parameter (A.4)
              |call: &mut Call, args: &[Value]| -> Result<Out, HostError> { /* ... */ })?
    .group("math")
    .reject(&[CheckClass::Index])                                       // added to the set
    .limits(Limits { steps: Some(100_000), ..Limits::default() })
    .build()?;                    // Env is Send + Sync and cheap to clone

let checked = env.check(&expr, Expect::Condition);   // { ty, diagnostics, sites }; never fails
let program: Program = env.compile(&expr, Expect::Condition)?;   // Send + Sync

// The child for re-checking stored programs after a schema tightens (A.2).
let recheck: Env = env.with().allow(&[CheckClass::ConstantCondition]).build()?;

/// Variables by name. Implemented for maps of Host values, for a derived
/// struct, and for a closure, so a host may compute a value on demand.
pub trait Vars { fn get(&self, name: &str) -> Option<View<'_>>; }

let order: Type = env.ty("Order[]")?;         // names are env's
order.conform(&body)?;                        // body: serde_json::Value, read in place

let out: Output<'_> = program.run(&vars)?;    // borrows vars; owns what the run created
out.value();                                  // a lent Value: read it in place
out.value().truthy();                         // the language's test (A.3); cannot fail
out.usage();
let owned: Owned = out.into_owned();          // a copy that outlives vars
let n: i64 = out.to()?;                       // or convert to a Rust type

impl Output<'_> {
    /// Step 5 of a load (A.14): what serde_json gives for the JSON text of
    /// the value, with unknown fields refused. It need not make the text.
    pub fn to<T: DeserializeOwned>(&self) -> Result<T, Error>;
    /// The JSON text of the result (A.3).
    pub fn to_json(&self) -> Result<String, Error>;
}
impl Env {
    /// Steps 2 to 5 of a load, with the type text T's derive gives.
    pub fn load<T>(&self, x: &Expr, vars: &impl Vars) -> Result<T, Error>
    where T: DeserializeOwned + Typed;
    /// The same with another type, such as one from a closed schema.
    pub fn load_as<T: DeserializeOwned>(&self, x: &Expr, ty: &Type, vars: &impl Vars)
        -> Result<T, Error>;
}

#[derive(serde::Deserialize, risor::Host)]
struct Config {
    listen: String,
    #[serde(default)] max_body: u32,          // optional in the file
    backends: Vec<Backend>,
}
let cfg: Config = env.load(&parse(src)?, &vars)?;
```

The helper for case files is the module `risor::testing` (A.11).

```rust
pub fn load(path: impl AsRef<Path>) -> Result<CaseFile, Error>;   // an error: invalid
impl CaseFile {
    pub fn cases(&self) -> &[Case];
    pub fn run(&self, base: &Env, vars: &dyn Vars) -> Report;     // every case
}
impl Case   { pub fn run(&self, base: &Env, vars: &dyn Vars) -> Verdict; }
impl Report { pub fn assert_ok(&self); }  // panics, and lists the cases that failed

#[test]
fn discount() {
    let file = risor::testing::load("tests/discount.test.json").unwrap();
    file.run(&env(), &NoVars).assert_ok();
}
```

It gives one `#[test]` for each file, because the standard harness cannot add
a test at run time. The report names the cases. Go gives one subtest for each
case, and TypeScript one `test` call.

Where this binding differs in kind from the other two, and what Part A says so
that all three conform:

| Point | Go | TypeScript | Rust | Part A |
| --- | --- | --- | --- | --- |
| Finding a field by name | Reflection over any struct | The object's own properties | The `Host` trait, written or derived. A type without it cannot be passed | "an object with named fields"; how is the binding's matter (A.3) |
| Declaring types | Type text, JSON Schema, reflection | Type text, JSON Schema | Type text, JSON Schema, and the derive, which emits the type text of the struct | the portable forms (A.2) |
| How long a value lives | The collector decides | The collector decides | A host value is lent for the lifetime of the variables. The compiler rejects a run or a result that outlives them | "a result may share memory with the inputs" (A.3) |
| Memory the run creates | Collected | Collected | Owned by the output and released with it, in one step. No collector (`O:R3`, `O:R4`) | the allocation budget is on what is created (A.5) |
| Read-only arguments (`O:S2`) | A contract | A contract | Enforced: arguments are lent immutably and cannot be kept | A.4 |
| Absent and null | A missing key is absent. A nil pointer, a nil interface and a key whose value is `nil` are null | A missing or `undefined` property is absent. JavaScript's `null` is null, and so is `undefined` outside a property | A missing key is absent. `None` is null, or absent where the field is marked so | "each binding says what counts as absent" (A.3) |
| Identity of an opaque value (`O:S11`) | Pointer or interface equality | `===` | The address of the lent value | "the binding supplies the test" |
| Invalid text | Bad UTF-8 in a string | A lone surrogate | Cannot occur: a `str` is valid UTF-8 | each malformed unit reads as U+FFFD (`02` 2.6); Rust has nothing to check |
| Errors | Returned `*Error` | A thrown `RisorError` | `Result<_, Error>` | one phase, a list of diagnostics (A.6) |
| A host function that crashes | A recovered panic is `function-panic` | A thrown non-error is `function-panic` | A caught unwind is `function-panic`. In a build where a panic aborts, the process ends | A.4 |
| Waiting in a host function | It may block | It cannot | It may block. An async host runs the program on a blocking thread | "returns before the run continues" (A.4) |
| Time and cancellation | The context is polled | The clock is read | The clock is read, and a flag another thread sets is seen | A.5 |
| Deep nesting | The stack grows | A `RangeError` at a depth the engine chooses | The process ends. Every recursion is counted against a specified limit, and release does not recurse (`O:R5`, `O:R6`) | depth and source nesting are specified limits (A.5) |
| Sharing between threads | A program is safe to share | One thread | A program and an environment are `Send + Sync`. A run is not shared | "immutable once built" (A.1) |

These are the same in kind in all three bindings and differ only in
spelling:

| Point | Go | TypeScript | Rust | Part A |
| --- | --- | --- | --- | --- |
| The expected result | `Expect(t)` or `Condition`, an option of `Check` and `Compile` | `"bool"` or `"condition"`, the second argument of `check` and `compile` | `Expect::Type`, `Expect::Condition`, an argument of `check` and `compile` | per check and per compile (A.3) |
| The truthiness test | `risor.Truthy(v)` | `truthy(v)`, and never `if (v)` | `value.truthy()` | "every binding offers the language's own test" (A.3) |
| The rejected classes | `Reject` and `Allow`, declarations | `reject` and `allow`, options of `createEnv` and `with` | `reject` and `allow` on the builder | a declaration of the environment, carried by the description (A.2) |
| A named type | `NamedType(name, t)`, with `t` from type text or `TypeFromSchema` | the `types` option: a type text or a schema per name | `named_type` and `named_schema` on the builder | a name and a type, in order (A.2) |
| An opaque type's name | `Opaque[T](name)`: a Go type | the `opaque` option: a test of a value | `opaque::<T>(name)`: a `TypeId` | "which host values carry the name is binding-specific" (A.2) |
| The conformance function | `Conform(v, t)`, with `t` from `env.ParseType` | `env.conform(value, typeText)` | `env.ty(text)?.conform(&value)` | a value, and a type whose names are an environment's (A.13) |
| The path of a failure | `Diagnostic.Path`, a `[]any` | `path`, a `(string \| number)[]` | `path`, a `Vec<PathStep>` | a list of keys and indexes (A.6) |
| Every failure | `ConformAll(v, t, max)` | `env.conformAll(value, typeText, max)` | `env.ty(text)?.conform_all(&value, max)` | the second function (A.13) |
| The three renderings | `Error()`, `Detail()`, `JSON()` | `message`, `detail()`, `toJSON()`, and `render` for diagnostics that `check` returned | `Display`, `detail()`, `to_json()` | one line, detail, JSON (A.6) |
| The trace and the data of a run error | `Diagnostic.Trace`, `Diagnostic.Data` | `trace`, `data` | `trace`, `data`, the second an owned copy | fields of the diagnostic (A.6) |
| A step of a static path | `Step{Key, Star}` | `string \| typeof star` | `RefStep::Key`, `RefStep::Star` | a key or a star (A.8) |
| The JSON text of a result | `JSON(v)` | `toJSON(value)` | `out.to_json()` | the JSON form of `02` 2.13 (A.3) |
| The decode | `Decode(v, &dst)`, by the rules of `encoding/json` | none: the value is the host's plain data already, and `load` copies it | `out.to::<T>()`, by the rules of serde | the binding's, with unknown fields refused (A.14) |
| A load | `Load(ctx, env, x, vars, &dst)` | `load<T>(env, x, expected, vars)` | `env.load::<T>(&x, &vars)` | steps 2 to 5 (A.14) |
| A type from a host type | `TypeOf[T]`; `omitempty` and `omitzero` make a field optional | none: types are erased | the derive; `#[serde(default)]` makes a field optional | outside the model (A.14) |
| Schema options | `SchemaOption` values given to `TypeFromSchema` | `typeFromSchema(schema, options)` | `SchemaOptions`, given with each schema | given with each schema, each off unless set (`03`, 2.15) |
| The helper for case files | `risortest.Run`, one subtest for each case | `defineCases`, one `test` for each case, with the host's runner | `risor::testing`, one `#[test]` for each file | the harness, made public (A.11) |

**Where `07` does not fit the bindings.** Its section 2.8 gives the function as
`Conform(v, t)` in Go, `conform(value, type)` in TypeScript and `conform(view,
&Type)` in Rust, each free of any environment. Four corrections follow from
Parts B to D.

| `07` says | What a binding needs |
| --- | --- |
| A free function of a value and a type | A type may hold a named type or an opaque name, and only an environment can resolve either. Go keeps the free function and resolves the type first. TypeScript and Rust reach the function through the environment |
| Rust takes `&Type`, TypeScript takes a type | Neither sketch had a type value: a type was text. Rust gains `Type` so that a type is parsed once. TypeScript keeps text and caches |
| "It needs nothing new of a binding" (2.7) | True for lists and records. For an opaque name the binding must know which values carry it, and this document had no declaration that said so. A.2 now has one |
| The error is `not-conforming`; `x is T` never fails | `07` did not say what the walk does with a host number outside the safe range. Ruled with `04` (A.3): the value conforms to no type, so the walk reports `not-conforming` there and both statements hold (A.13) |

**What Part A had to change.** One row. A.3 said "an object with named fields"
and Part B read that as "anything reflection can open". It now says what the
model needs from a value: its kind, its size, a field by name, its keys. The
depth row of A.5 gains source nesting, because Rust cannot leave it to the
platform.

**Two routes for host data.** Reading in place needs the `Host` trait on the
host's types. A host whose types only implement `serde::Serialize` can convert
them to an owned value before the run. That is a copy, made by the host and not
charged (`O:L4`). It is the convenience route, and it gives up the property the
design is built on, so the documentation should name the trait first.

**No async run in 3.0,** as in TypeScript and for the same reason. A Rust host
function may block, so an async host calls `run` from a blocking thread.

**The build for other hosts** (`O:R8`). The same crate built for WebAssembly or
behind a C interface serves a host with no implementation of its own. Variables
go in as JSON text and the result comes out as JSON text. The stages that never
touch a host value (parse, analyze, check, assist) lose nothing by this route
(`O:R9`).

**Open in this sketch:** whether `Value` is 16 or 24 bytes; whether the run's
memory is a region or reference counts (`04` Part D recommends a region); the
exact shape of `Vars` for values computed on demand, which must stay alive until
the output is dropped; and how a derived struct exposes a field holding an
integer above the safe range (as text, by an attribute).

## 4b. Part E: one surface in three bindings

Parts B, C and D were sketches, and each binding grew its own public API as it
was built. In November 2026 the three surfaces had the same concepts under
different names, different shapes for the same step, and different amounts of
internals exported. The owner ruled (R201) that the bindings stay idiomatic and
align in everything else. This part is the authority for the public surface of
each binding. Where Parts B to D disagree with it, this part wins.

### E.1 What may differ

A binding may differ from the others only in these five ways.

| # | What | Go | TypeScript | Rust |
| --- | --- | --- | --- | --- |
| 1 | Case of names | `PascalCase` exported | `camelCase`; types `PascalCase` | `snake_case`; types `PascalCase` |
| 2 | How an environment is declared | Option values: `NewEnv(decls...)` | One options object: `createEnv({...})` | A builder: `Env::builder()...build()` |
| 3 | How a failure reaches the host | A returned `error` | A thrown `RisorError` | A returned `Result` |
| 4 | How a run is cancelled | `context.Context` | `AbortSignal` | `RunOptions` |
| 5 | Conveniences for the host's own types | `TypeOf[T]`, `Load[T]`, `AsInt`, `AsFloat`, `TypedLayer` (F11) | none | `type_of::<T>()`, `#[derive(Host)]`, `var_host::<T>()`, `load::<T>()` |

Everything else is the same: the concepts, their names, which object a
function belongs to, what it takes, and what it gives. A name in the tables
below is the canonical name. Each binding writes it in its own case (row 1).

### E.2 The boundary

1. The root of each package exports exactly the surface of E.3 to E.8, and
   nothing else.
2. The case-file runner is a separate entry point: Go `risortest`, TypeScript
   `risor/testing`, Rust `risor::testing`.
3. Everything else is private: Go `internal/`; TypeScript not exported from
   `index.ts`; Rust `pub(crate)`, or `#[doc(hidden)]` where a binary or an
   integration test of the crate needs it.
4. One concept has one way to reach it. An alias or a second spelling of the
   same step is removed.

### E.3 The environment

| Concept | Canonical name | Go | TypeScript | Rust |
| --- | --- | --- | --- | --- |
| Build | (E.1 row 2) | `NewEnv(decls...)` | `createEnv(options)` | `Env::builder()...build()` |
| Child environment | `with` | `env.With(decls...)` | `env.with(options)` | `env.with()...build()` |
| Version | `version` | `Version(v)` | `version` | `.version(v)` |
| One variable, by type text | `var` | `Var(name, text)` | `vars: {name: text}` | `.var(name, text)` |
| All variables, as one record type | `varRecord` | `VarRecord(text)` | `varRecord: text` | `.var_record(text)` |
| One variable, by JSON Schema | `varSchema` | `VarSchema(name, schema, opts...)` | `varSchemas: {name: {schema, options}}` | `.var_schema(name, schema, opts)` |
| One variable, by a `Type` | `varType` | `VarType(name, t)` | `varTypes: {name: t}` | `.var_type(name, t)` |
| Host function | `function` | `Function{Name, Type, Impl}` | `functions: {name: {type, call}}` | `.function(name, sig, impl)` |
| Opaque type | `opaque` | `OpaqueType{...}` | `opaque: {Name: {test, methods}}` | `.opaque(name)` |
| Method of an opaque type | `method` | `Method(type, name, f)` | `opaque.Name.methods` | `.method(type, name, sig, impl)` |
| Named type, by type text | `namedType` | `NamedType(name, text)` | `namedTypes: [{name, type}]` | `.named_type(name, text)` |
| Named type, by JSON Schema | `namedTypeSchema` | `NamedTypeSchema(name, schema, opts...)` | `namedTypes: [{name, schema, options}]` | `.named_type_schema(name, schema, opts)` |
| Named type, by a `Type` | `namedTypeOf` | `NamedTypeOf(name, t)` | `namedTypes: [{name, typeOf: t}]` | `.named_type_of(name, t)` |
| Opt-in groups | `groups` | `Groups(names...)` | `groups: [...]` | `.groups(&[...])` |
| Library entries removed | `without` | `Without(names...)` | `without: [...]` | `.without(&[...])` |
| Check classes rejected | `reject` | `Reject(classes...)` | `reject: [...]` | `.reject(&[...])` |
| Host fact (F14) | `someNotNull` | `SomeNotNull(names...)` | `someNotNull: [[...]]` | `.some_not_null(&[...])` |
| Limits | `limits` | `Limits{Steps, Allocation, Deadline, Depth}` | `limits: {steps, allocation, deadlineMs, depth}` | `.limits(Limits{steps, allocation, deadline, depth})` |
| The description (F1 5) | `describe` | `env.Describe()` | `env.describe()` | `env.describe()` |
| The description as text | `descriptionText` | (`Describe` gives bytes) | `descriptionText(d)` | `env.description_text()` |
| An environment from a description | `fromDescription` | `FromDescription(data)`, a declaration | `Env.fromDescription(json)` | `Env::from_description(json)` |
| A type from type text | `parseType` | `env.ParseType(text)` | `env.parseType(text)` | `env.parse_type(text)` |
| The library's names | `libraryNames` | `env.LibraryNames()` | `env.libraryNames()` | `env.library_names()` |

`Function` in Go keeps its struct form, which is the Go idiom for a declaration
with three parts. A host function with no implementation is declared the same
way with the implementation left out; it is not a second name.

### E.4 Programs

| Concept | Canonical name | Go | TypeScript | Rust |
| --- | --- | --- | --- | --- |
| A parsed program | `Expr` | `*Expr` | `Expr` | `Expr` |
| Parse | `parse` | `Parse(text)` | `parse(text)` | `parse(text)` |
| Its text | `text` | `x.Text()` | `x.text` | `x.text()` |
| Analyze (F2) | `analyze` | `x.Analyze()` | `x.analyze()` | `x.analyze()` |
| The report of an analysis | `Analysis` | `Analysis` | `Analysis` | `Analysis` |
| An analysis as JSON text | `json` | `a.JSON()` | `a.json()` | `a.json()` |
| What the host expects of a result | `Expected` | `Expect(text)`, `ExpectSchema(...)`, `ExpectType(t)`, `Condition`, `NoCheck` | `expected: text \| {schema} \| {type} \| "condition"`, `check: false` | `Expected::text(..)`, `::schema(..)`, `::of(t)`, `::Condition`, `::None`, and `check: false` in `CompileOptions` |
| Check | `check` | `env.Check(x, opts...)` | `env.check(x, expected)` | `env.check(&x, &expected)` |
| The result of a check | `CheckResult` | `{Type, Sites}` | `{type, sites}` | `{ty, sites}` |
| Compile | `compile` | `env.Compile(x, opts...)` | `env.compile(x, options)` | `env.compile(&x, options)` |
| A compiled program | `Program` | `*Program` | `Program` | `Program` |
| Its check result | `checkResult` | `p.CheckResult()` | `p.checkResult` | `p.check_result()` |
| Run | `run` | `p.Run(ctx, vars)` | `p.run(vars, {signal})` | `p.run(&vars)`, `p.run_with(&vars, opts)` |
| Run and report usage | `measure` | `p.Measure(ctx, vars)` | `p.measure(vars, {signal})` | `p.measure(&vars)`, `p.measure_with(&vars, opts)` |
| What a run used | `Usage` | `{Steps, Allocated}` | `{steps, allocated}` | `{steps, allocated}` |
| The variables of a run | `Vars` | `Vars`, `Map` | an object, a `Map` or a function | `Vars`, `NoVars` |

1. `check` and `compile` take an `Expr`, never source text. A host parses first.
2. A check result's type is a `Type` (E.7). Its printed form is the type's
   text form.
3. `run` gives the value or the failure. `measure` gives the usage as well,
   whether the run succeeds or fails.
4. Removed: TypeScript `execute`, `tryCompile`, `parseProgramText`,
   `ParsedProgram`, `reportJSON`, and the AST-level `parse`; Rust the free
   functions `compile`, `compile_unchecked`, `check` and `analyze`; Go
   `CheckCompile`, `WalkCalls`, `Expr.Names`, `Expr.Refs`, `Expr.Calls`,
   `Expr.Literal` and `Expr.Source`, each of which `Analyze` or `Text` gives.

### E.5 Templates

| Concept | Canonical name | Go | TypeScript | Rust |
| --- | --- | --- | --- | --- |
| Parse | `parseTemplate` | `ParseTemplate(text)` | `parseTemplate(text)` | `parse_template(text)` |
| A parsed template | `Template` | `*Template` | `Template` | `Template` |
| The mode | `TemplateMode` | `TemplateMode` | `TemplateMode` | `TemplateMode` |
| Its segments | `segments` | `t.Segments()` | `t.segments` | `t.segments()` |
| Exactly one part | `isSingle` | `t.IsSingle()` | `t.isSingle` | `t.is_single()` |
| One part, as a program | `part` | `t.Part(i)` | `t.part(i)` | `t.part(i)` |
| The whole, as a program | `program` | `t.Program(mode)` | `t.program(mode)` | `t.program(mode)` |
| Analyze | `analyze` | `t.Analyze(mode)` | `t.analyze(mode)` | `t.analyze(mode)` |
| Format | `formatTemplate` | `FormatTemplate(text)` | `formatTemplate(text)` | `format_template(text)` |

### E.6 Host functions and values

| Concept | Canonical name | Go | TypeScript | Rust |
| --- | --- | --- | --- | --- |
| The call handle | `Call` | `*Call` | `Call` | `Call` |
| Charge steps | `charge` | `c.Charge(n)` | `c.charge(n)` | `c.charge(n)` |
| Charge allocation units | `allocate` | `c.Allocate(n)` | `c.allocate(n)` | `c.allocate(n)` |
| A host function's error | `HostError` | any `error` | `HostError` | `HostError` |
| Truthiness | `truthy` | `Truthy(v)` | `truthy(v)` | `truthy(&v)` |
| A value as JSON text | `jsonText` | `JSONText(v)` | `jsonText(v)` | `json_text(&v)` |

### E.7 Types, conformance and config files

| Concept | Canonical name | Go | TypeScript | Rust |
| --- | --- | --- | --- | --- |
| A type | `Type` | `Type` | `Type` | `Type` |
| A type from JSON Schema | `typeFromSchema` | `TypeFromSchema(schema, opts...)` | `typeFromSchema(schema, options)` | `type_from_schema(schema, opts)` |
| Building and reading a type (R179) | | constructors, accessors, `Join`, `Fits`, `Equal`, `String` | the same names | the same names |
| Test a value against a type | `conform` | `Conform(v, t)` | `conform(v, t)` | `conform(&v, &t)` |
| Every failure, up to a maximum | `conformAll` | `ConformAll(v, t, max)` | `conformAll(v, t, max)` | `conform_all(&v, &t, max)` |
| Load a config file into a value | `loadValue` | `LoadValue(ctx, env, text, vars, opts...)` | `loadValue(env, text, options)` | `load_value(&env, text, &vars, opts)` |
| Load into a host type | `load` | `Load[T](...)` | none (E.1 row 5) | `load::<T>(...)` |
| Decode a result into a host type | `decode` | `Decode(v, dst)` | `decode(v)` | `decode::<T>(&v, max)` |
| A failed load | `LoadError` | `*LoadError` | `LoadError` | `LoadError` |

The structural type API of R179 is offered by all three, with the names of the
Go binding cased per E.1 row 1: the `Type` constants, `ListType`, `MapType`,
`Nullable`, `LiteralType`, `RecordType`, `ExactRecordType`, `UnionType`,
`TaggedUnionType`, `OpaqueTypeNamed`, `Join`, and the methods `kind`,
`elem`, `fields`, `field`, `rest`, `alternatives`, `variants`, `literals`,
`tag`, `nonNull`, `isNullable`, `isExact`, `opaqueName`, `fits`, `equal` and
the text form.

### E.8 Diagnostics and tools

| Concept | Canonical name | Go | TypeScript | Rust |
| --- | --- | --- | --- | --- |
| A diagnostic | `Diagnostic` | `Diagnostic` | `Diagnostic` | `Diagnostic` |
| A failure with its diagnostics | `Error` | `Error` | `RisorError` | `Error` |
| Phase, span, position | `Phase`, `Span`, `Pos` | the same | the same | the same |
| One line | `renderLine` | `RenderLine(ds)` | `renderLine(ds)` | `render_line(&ds)` |
| Detail, with source | `renderDetail` | `RenderDetail(ds, source)` | `renderDetail(ds, source)` | `render_detail(&ds, source)` |
| JSON | `renderJSON` | `RenderJSON(ds)` | `renderJSON(ds)` | `render_json(&ds)` |
| Format a program (F12) | `format` | `Format(text)` | `format(text)` | `format(text)` |
| Completion (F12) | `complete` | `env.Complete(text, pos, opts...)` | `env.complete(text, pos, expected)` | `env.complete(text, pos, &expected)` |
| Hover (F12) | `hover` | `env.Hover(text, pos, opts...)` | `env.hover(text, pos, expected)` | `env.hover(text, pos, &expected)` |
| A completion | `Completion`, `Candidate` | the same | the same | the same |
| A hover | `Hover` | `Hover` | `Hover` | `Hover` |

A position is a line and a column (`spec/core/03-host.md` 3.7.7) in all three,
not an offset. `RisorError` keeps its name because `Error` is a global of
JavaScript.

### E.9 What is not part of the surface

The Go REPL's `Env.RunEntry` and `Held` serve `cmd/risor` alone. They move out
of the root package, to a package under `internal/` that `cmd/risor` imports.

## 5. Alternatives rejected

- **expr's shape: `Compile(src, opts)`, then `Program.Check(types)`.** Checking
  becomes a step a host can skip, and functions and their types go in two lists
  (`workflow:expr.go:68-71`, `workflow:typecheck.go:687-692`).
- **A model written as the Go API with notes.** A TypeScript reader would have
  to subtract the context, reflection and `(value, error)` to find the contract.
- **Reserving every library group's name against variables,** whether or not the
  group is on. `time` and `json` are names a host's data wants.
- Rejected where they arise: compiling without a separate parse (A.1), a warning
  severity for constant conditions (A.2), functions as run-time values (A.3;
  `expr:program.go:256-264`), the expected result as a declaration (A.3), a
  default time limit (A.5), the path of a failure as message text (A.6), two
  version numbers (A.10), limits on the conformance function (A.13), a `types`
  package (B.2), a result struct from `Run` (B.3), automatic Go methods (B.7),
  validators and transforms (B.9), an async run (Part C).
- Rejected in the plans, and not argued again here: messages left to each
  implementation, the path inside a message, a trace kept during the run or
  readable by a program, and `help` specified (`10`, section 5); a test
  construct, YAML case files, `maxSteps` in `expect`, and `risor conform` as a
  second command (`11`, section 7); a data mode for config files, a decode
  with rules of its own, and a library entry that reads the process
  environment (`12`, section 8).
- Rejected here: a maximum as a third argument of the first conformance
  function, and an `as` that reports every failure (A.13); a star written as
  the key `"*"` in a static path (A.8, B.1).

## 6. Rulings applied, and what I still assume

| Ruling | Where it lands here |
| --- | --- |
| `O:P2`, `O:P4`, `O:P7` | Parts A, B and C; A.11 |
| `O:P5` | A.2; `TypeOf`, `Fields`, `Bind` and struct tags moved to B.7 |
| `O:P6`, `O:T5` | A.6, B.5 |
| `O:L1` to `O:L8`, `O:S9` | A.5, A.9; `Limits`, `Usage`, `Call.Allocate` |
| `O:S1` | A.2; `Func("geo.distance", ...)`, `Groups`, `Without` |
| `O:S2`, `O:S3`, `O:S4`, `O:S5` | A.4, A.9; B.4 (`Symbol.Path`); B.2 |
| `O:B1`, `O:B5`, `O:B8`, `O:N1` to `O:N6` | The language has truthiness. A host states the result it expects when it checks and compiles: `condition`, tested with `Truthy`, which the workflow engine takes for `when:`, or a type such as `bool` (A.3, B.2, B.3, host case 3) |
| `O:B7` | The rejected check classes are a declaration of the environment, with `constant-condition` in the default set (A.2, A.6; `Reject`, `Allow`; host case 4) |
| `O:E9` | A host function or method has no callback parameter (A.4), and the calls analysis lists are complete (A.8) |
| `O:Y3`, `O:Y4` | "null" throughout, and every type text in TypeScript's spelling. Go's `nil` keeps its name where Go is meant. `risor.Nil` and `risor.Dynamic` are now `risor.Null` and `risor.Any`, and Rust's `View::Nil` is `View::Null` |
| `O:X1`, `O:X2` | A binding is `const` (A.2, A.8). The result of an `if` is a result like any other, and its test is a tested position (A.3) |
| `O:V1` to `O:V5` | Named types are the eighth kind of declaration, in one set of names with opaque types (A.2). The conformance function, with no limits (A.13). `not-conforming` with a path field (A.6). The second kind of conformance case (A.11). `NamedType`, `Conform`, `Opaque` and their equivalents (Parts B to D; host case 5) |
| `O:R1` to `O:R9` | Part D. A.3 no longer assumes a binding can inspect a host value by reflection |
| `O:D46` (`08`) | Positions and origins for the text block and the raw string, and the three `parse` codes (A.6). The lexer that ends a template segment reads raw strings (A.7). A string literal is a literal in every form (A.8). The editor grammar (A.12, B.11) |
| `O:D47` (`09`) | `bad-type` for records with no tag, and `untagged-union` in the check phase (A.2, A.6). "Fits through its form" decides where `result-mismatch` is reported (A.3). The walk reads the tag first (A.13). `Tag` and `Variants` on `Type` (B.2) |
| `O:D48` (`10`) | A.6: the message is specified, the fields `data` and `trace`, every code by phase, the rules of the catalog, three renderings, and what a conformance test compares. What `catch` lets pass (A.3, A.4, A.5). The second conformance function (A.13). `Error`, `Detail`, `JSON`, `TraceEntry`, `ConformAll` and their equivalents (Parts B to D) |
| `O:D49` (`11`) | A.11 is rewritten: the header, stubs as data, one shape of a failure, verdicts, `returns` in place of `expected`, the three kinds of case only the suite has, program files, update mode. `risortest` (B.13), `defineCases` (Part C), `risor::testing` (Part D). `test` in place of `conform` (B.11) |
| `O:D50` (`12`) | A.14: the five steps of a load. The JSON text of a result (A.3). `Load`, `Decode`, `JSON` (B.3), optional fields and text types from Go types (B.7), host case 6 (B.8), the CLI's flags (B.11). `load`, `toJSON`, `typeFromSchema`, `to`, `to_json`, `SchemaOptions` (Parts C and D). Of imports, only that `import` is reserved |
| `O:D51` (`13`) | A star is a step of a static path (A.8; `Step`, `star`, `RefStep`). A projection in progress has a trace entry (A.6) |

**One ruling needs an amendment.** `O:L5` has the TypeScript implementation poll
an `AbortSignal`. In a synchronous run no abort event can be delivered, so the
poll would see nothing. A.5 therefore specifies the check and leaves the
mechanism open, and the TypeScript implementation reads the clock.

**Round 8: plans `08` to `13` applied** (`O:D46` to `O:D51`). The last six rows
of the table say where each plan lands. The points below were ruled with the
plans, or decided here because a plan left them open or could not be applied
as written.

| Point | What this document now says |
| --- | --- |
| The case-file field `expected` | Renamed `returns` (A.11). `expect` keeps its name, and so does the term "the expected result" (A.3) |
| A tolerance in a case | None. A case rounds in its own program (A.11) |
| A host number that cannot enter the language | Code `number-range`, class `environment`, caught (A.3, A.6). `04` A.2 and A.3 here now agree |
| `function-panic` | Class `host`, not caught (A.4, A.6). It had no class |
| A failure inside the implementation | A code of its own, `internal-error`, with no class and not caught (A.6, A.9). No plan names one, and every code must have a name |
| The class of a run error with no checker | `dynamic` for a null, a kind or a field failure (A.6). `03` states the rule |
| A projection in a trace | An entry with the span of the star, the name `[*]` and the index of the element (A.6). The name is this document's choice |
| The length of a trace | The depth limit and the 100 levels of source nesting together. `10:2.8` says the depth limit alone, which was written before a star had an entry |
| `needs-version` | A `check` code that `03`'s catalog does not hold. So the `check` phase has 30 codes, and `03` has 29 |
| The count of codes | 115: 8 of `declare`, 58 of `parse`, 30 of `check` and 19 of `run` (A.6). `10:2.9` estimated about 110 templates. There are more templates than codes, because a code may have several |
| After the first `parse` error | Only the first diagnostic is specified, and with it the other `text-block-indent`s of the same block (A.6). `02`, 3.2 left the rule to this document, and `10:2.9` asks only that three parsers agree on the first error |
| The messages of the decode | The binding's own (A.6, A.14). `12` asks the question. The catalog has no entry, because the suite does not test the decode |
| The JSON text of a result, outside a run | Bounded by the default depth limit, so that cyclic host data ends (A.3). `12` gives no bound |
| Update mode | Also writes `message` (A.11). `11:2.8` lists `code`, `class`, `span` and `path`, from before messages were specified |
| A load in Rust | `load_as` takes a `Type`, for a type from a schema. `12:3.5` gives only the form that needs the derive |
| The case kind for renderings | The field `render` with `diagnostic`, `source` and `origin`, and `line`, `detail` and `json` in `expect` (A.11). `10` asks for the kind and names no field |
| A conformance case that asks for every failure | `max` in `conform`, and `failures` in `expect`, each a `path` and a `message` (A.11). `10` names the two fields and not their shape |
| A star in a static path | A step of its own kind, never the key `"*"` (A.8). `13` marks the change (I) and gives no form |
| The case field `groups` | Added (A.11). `11:2.2` has no field that turns an opt-in group on, and `06`, 2.11 needs one for the cases of `math`, `time`, `json` and `base64` |
| The trace entry of `mapValues` | Its index is the position of the entry in key order (A.6), as `04` and `06` propose. `10:2.8` was written when every callback had an index |
| A variable or a function named with a reserved word | `bad-name` (A.2). `catch`, `throw`, `finally` and `import` are new reserved words, so a host that declared one of those names must rename it |
| `Error()` in Go | The one-line form of every diagnostic (B.5). It was the first diagnostic and a count |

Still assumed, because the sibling documents were in revision:

- **`02`:** one top-level construct (A.1); lambda parameters and `const` names
  shadow environment names; the lexer can start inside a larger string, for
  templates, and reads raw strings there (A.7); the parser can recover from an
  unfinished program, for A.12; the names of the `parse` codes and of the run
  codes it owns, as A.6 lists them; that a star's span, for a trace entry, is
  the step `[*]` or `?.[*]` and not the whole `Each` node.
- **`03`:** the type text and its JSON Schema table; the code catalog and the
  classes of its promise, with `budget` renamed `limit` now that it has five
  codes; that the site class of unchecked data keeps the name `dynamic` though
  the type is now `any`; that `condition` as the expected result makes the
  result a tested position; that its conformance cases take `reject` as the
  whole set of A.2 and `returns` in place of `expected`; that it holds "fits
  through its form" as one rule for `result-mismatch` and `arg-mismatch`; and
  that it states the class of a run error when nothing was checked.
- **`07` and `03`:** the walk's order and its step rule; that a failed `as`
  whose subject already had the type is classed by `03`; that `03` 2.16 takes
  the `types` field and the second kind of case as A.11 writes them.
- **`04`:** the allocation unit and the check interval; `number-range` raised at
  the read; the `int64` or `float64` rule of B.3; one rule for invalid text;
  that a trace entry is written on the error path and costs a run that succeeds
  nothing; that a stubbed call costs the step of a call and its row's `charge`.
- **`06`:** groups are namespaces, and each entry records the version that added
  it; `messages.json` stands beside `library.json`, and each `fails` line of an
  entry names its code and template; `json.stringify` fails on an opaque value
  with a code the JSON text of A.3 can share.
- **`10`:** the wording of every template is the file `messages.json`, which is
  written with the specification and not in these documents.

## 7. Open questions for the owner

1. **Should the core module's tests stop using wonton's `assert`?** (`O:S10`)
   Recommendation: yes, with an `internal/assert` of the same names. It is the
   only way the module has no `require` line, and the conformance files carry
   most behavior tests anyway.
2. **Drop the language server from 3.0?** (`O:S10`) Recommendation: drop the
   server for `.risor` files, and ship `Complete`, `Hover` and `Describe` in
   every implementation (A.12). They are what a host's web editor or its own
   language server needs, and the part only this project can write. A generic
   server that finds expressions in YAML through a mapping file can follow 3.0.
3. **Is no default time limit right?** Recommendation: yes (A.5). A default of
   one second would protect nothing the step budget does not.
4. **Do struct fields take their `json` tag name by default in the Go binding?**
   Recommendation: yes, with a `risor` tag to override.
5. **No async run in the TypeScript implementation?** Recommendation: none in
   3.0 (Part C); wait for a host that cannot hoist its I/O. The same holds for
   Rust (Part D).
6. **Are the template delimiters fixed at `${{ }}`?** Recommendation: fixed, so
   that the specification has one template grammar. `02` drafted them as
   configurable, and expr defaults to `${ }` (`expr:template.go:107`).
7. **Should a case be able to expect the other analysis results?** `expect`
   has `names` and no field for references, calls, the literal or the segments
   (`11:2.4`), so the suite cannot hold three implementations to A.8, the star
   step included. Recommendation: add `refs`, `calls`, `literal` and
   `segments` to the fields of a result before the first suite file is
   written. This document does not add them, because `11` fixed the list.
8. **Is `internal-error` the right code for a failure inside the
   implementation, with no class?** Recommendation: yes. The other choice is to
   report it as `function-panic`, which names a function that does not exist.
9. **Does `needs-version` move into `03`'s catalog?** It is reported by check,
   and it is the one `check` code `03` does not list. Recommendation: yes, so
   that one document holds the phase. Then `03` has 30 codes.
10. **Must the TypeScript and Rust helpers have update mode in 3.0?**
    Recommendation: no (`11`, section 9). It is a tool, and the command and the
    Go helper cover it.
11. **May the Go decode assign an opaque value to a field of its own Go type?**
    Recommendation: no in 3.0, to keep one rule in three bindings: an opaque
    value in a result is refused by the decode and by the JSON text (`12`,
    section 9).
12. **Is the name of a star's trace entry `[*]`?** The ruling gives the entry
    a span and an index. The detail form prints a name, so it needs one.
    Recommendation: `[*]`, whether the star was written `[*]` or `?.[*]`.

## 8. Implementation notes

**Rough size of the Go binding (I):** about 3,000 non-test lines beyond the
checker, evaluator and library, of which completion and hover are 500. `07`
adds about 180 for named types and `Conform`; the walk itself is the
evaluator's. Round 8 adds about 1,300 to the binding and 300 to the CLI, by the
plans' own estimates:

| Part | Lines without tests (I) | From |
| --- | ---: | --- |
| The catalog's loader and generator, and the data | 200 | `10`, section 3 |
| The three renderings | 150 | `10`, section 3 |
| `ConformAll`, over the walk that exists | 80 | `10`, section 3 |
| Case files: reading, the header rule, stubs, stages, verdicts | 500 | `11`, section 5. About 450 of it is the runner step 1 below needed anyway |
| Update mode, with a JSON scanner that keeps offsets | 120 | `11`, section 5 |
| `risortest` | 80 | `11`, section 5 |
| The JSON text, `Decode`, `Load`, optional fields and text types | 130 | `12`, section 6. `Decode` is 40 lines over `encoding/json`, and 250 if it never makes the text |
| `Step`, the trace fields, `Tag` and `Variants` | 40 | this document |
| The CLI: the `test` command, and the flags of `eval` and `check` | 300 | `11`, section 5; `12`, section 6 |

So the binding is about 4,500 lines. None of this was built.

**Order of work.** (1) The case-file format of A.11, `messages.json`, and the
runner, which is `risortest`, with cases written before the code they test. (2)
Diagnostics, positions, the generated message table and the three renderings.
(3) `Parse` and analysis, tested against expr's cases
(`expr:identifiers_test.go`, `expr:calls_test.go`). (4) `Env` with versions,
namespaces and clashes; limits; `Run` and `Measure`; then `Conform`,
`ConformAll` and named types, which need no parser or checker change (`07`,
section 6). (5) Host functions, then templates. (6) `Describe`, then the
TypeScript checker against the same files. (7) `Complete` and `Hover`. (8) The
conveniences of B.7, `Load` and `Decode`, and the CLI. (9) Port the workflow
engine as the acceptance test; its soundness fuzz test (`proposal:93-97`) must
pass.

**The TypeScript implementation** can ship in two steps: first parse, analyze,
check, complete and hover, which is all an editor needs; then compile and run.
Its helper for case files and Rust's are about 600 lines each, with no command
and no update mode (`11`, section 5). It has no decode to write.

**Reused.** From v2: the parser and AST (`pkg/parser`, `pkg/ast`, about 5,400
lines), `pkg/errors/suggest.go`, and the caret rendering in
`pkg/errors/format.go:344-379`. From expr: template splitting
(`expr:template.go:357-451`), function preparation (`expr:prepared.go`) and the
struct tag plan (`expr:struct_tags.go`). **Deleted:** `pkg/bytecode`,
`compiler`, `vm`, `op`, `dis` and `object` (about 18,900 lines), `pkg/syntax`,
`testing`, `modules` and `errors`, `internal/tmpl`, `cmd/risor-lsp`,
`cmd/risor-docs`, and most of `cmd/risor`.

**Rough size of the Rust binding (I):** about the same as Go's beyond the
checker, evaluator and library, plus a derive macro in a crate of its own, about
400 lines. The trait implementations for the standard collections replace Go's
reflection code. The decode is the one part that is larger than Go's: a serde
deserializer over the output, about 300 lines (`12`, section 6).
