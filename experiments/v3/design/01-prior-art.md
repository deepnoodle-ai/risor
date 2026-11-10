# Risor v3 design, part 1: prior art and positioning

Date: 2026-11-09, revised the same day for the owner's constraints C1 to C5 in
`00-overview.md`, and again for the confirmed set of three implementations: Go,
Rust and TypeScript. Status: survey and recommendations. This document designs
no feature. It reports what other languages in this space do, what their users
complain about, and what v3 should take from that.

How to read it:

- A claim about another language ends with a source key such as [cel-spec]. The
  keys and their URLs are at the end. I read each page on the date above.
  **(inference)** marks my conclusion, as opposed to something a source states.
- Claims about our own code use `file:line`. `risor:` is this repository,
  `expr:` is `~/git/deepnoodle/expr` at v1.4.1, and `workflow:` is
  `github.com/deepnoodle-ai/workflow@v0.1.0` in the Go module cache. "The
  proposal" is `risor:docs/proposals/workflow-expressions.md`.
- "expr" is the owner's library. "expr-lang" is the widely used
  `expr-lang/expr`.
- **(run)** marks a result I got by running a short program on the date above,
  on one machine, with rustc 1.99.0, Go 1.26.2 and Node 22.22.3. It shows what
  those versions do, not what any specification says.
- For a Rust project the page I read is often a source file, because that is
  where its documentation is written.
- Rule numbers such as N2, L6 or R5 refer to `00-overview.md`. Sections 2 to 5
  and 7 to 9 are about the language. Section 6 is a note on the Go
  implementation, and section 3.4 on the Rust one.

## 1. Summary: the eleven lessons that matter most

1. **Every checker in this space gives up on untyped data, and JSON-shaped data
   is untyped by default.** CEL types it `dyn`. expr-lang returns nil for an
   unknown key of a nested map. Rego's documentation says a misspelled field "is
   indistinguishable from" a passing policy ([k8s-cel], [exprlang-env],
   [opa-lang]). A structural type for JSON-shaped data, built from a schema, is
   the gap. expr already has one (`expr:docs/reference/spec.md:734-741`).
2. **A language with several implementations is defined by its test data, not
   its prose.** CEL calls its conformance tests "a complementary specification …
   in the form of executable software". JSONata, JMESPath and JSON Schema each
   ship a suite of plain data files that any implementation can run
   ([cel-tests], [jsonata-suite], [jmespath-test], [jsonschema-suite]). Whatever
   the suite does not pin, implementations decide differently.
3. **The same things diverge every time: number width, string units, regular
   expressions, number formatting and map order.** Starlark's `len("😀")` is 4 in
   Go and 2 in Java. gojq has arbitrary-precision integers where jq has doubles.
   A Java port of JSONata documents a list of regular expression differences
   ([starlark-lang-spec], [gojq], [jq-manual], [jsonata-java]). Section 3.3 has
   the list.
4. **"Host values read in place" rules out one core behind a boundary, so there
   are three native implementations: Go, Rust and TypeScript.** OPA's Wasm route
   makes the host serialize its data as JSON into the module's memory, and
   wasm-bindgen copies every string across the boundary ([opa-wasm],
   [wasm-bindgen-str]). A Rust core reached from Go copies too: Regorus's Go
   binding takes its input as a JSON string ([regorus-go]). The Rust
   implementation, built for WebAssembly or behind a C interface with data
   copied in, is the fallback for every other host (section 3.2).
5. **Work done in native library code is where limits fail.** Lua's count hook
   fires only "while Lua is executing a Lua function". JSONata's timeout "cannot
   protect against" a runaway regular expression. expr-lang's 2025 crash was in
   recursive builtins ([lua54], [jsonata-guardrails],
   [exprlang-ghsa-recursion]). A limit is complete only if every library
   function charges for its own work.
6. **Declared, cumulative allocation accounting has been built in Go and in
   Rust.** Canonical's Starlark fork has each builtin declare its allocations
   before making them and says of its guarantees: "The contract is upheld
   through testing" ([canonical-safety]). MiniJinja's fuel charges "roughly one
   fuel per 64 bytes", and a value of known size "is only created if the
   remaining fuel can pay for it", in its Rust and its Go implementation
   ([minijinja-env], [minijinja-changelog]). These are the closest prior art to
   L3.
7. **Costs fixed by a specification will need to change, and a change can break
   stored programs.** Ethereum raised gas prices after underpriced operations
   were used for denial of service, and a later repricing warned that contracts
   assuming a fixed cost "may cease to function" ([eip-150], [eip-1884]). Fix
   the costs per version of the cost table, and let a host pin the version.
8. **No operator may coerce, and "missing" must not be silent.** GitHub Actions
   coerces mismatched operands to numbers and treats the string `'false'` as
   true. JMESPath and jq return null for a missing key ([gha-expr-313],
   [gha-25645], [jmespath-spec], [jq-manual]).
9. **Austere expression languages grow the same things: local names, a multi-way
   conditional, and richer iteration.** CEL added `cel.bind` and two-variable
   comprehensions, JMESPath added `let`, GitHub Actions added `case`, and Excel
   added `IFS` and `LET` ([cel-ext], [jmespath-let], [gha-expr], [excel-ifs],
   [excel-let]). v3 should start with them.
10. **Stored expressions outlive library versions.** Kubernetes documents which
    CEL library version each release enables, so that stored expressions keep
    evaluating after a rollback ([k8s-cel]). v3's function set and cost table
    need versions a host can pin.
11. **Rust takes away three things the first drafts leaned on: reflection, a
    garbage collector, and a stack overflow that can be survived.** A Rust host
    hands data over through serde, which copies, or through a trait it
    implements, which reads in place ([cel-rust-context], [minijinja-value]). A
    stack overflow aborts the process ([rust-stack-overflow]), and Cedar's
    check of the remaining stack does not work in WebAssembly ([cedar-self]).
    Every depth limit has to be a count that the specification fixes (section
    3.4).

## 2. The languages

Each table gives what to adopt, what to avoid, and what users report.

### 2.1 CEL (cel-go)

A typed expression language with implementations in several languages, used for
Kubernetes admission rules and IAM conditions ([cel-overview]). Sections 3 and 4
cover its other implementations and its cost limits.

| | Point | Evidence |
| --- | --- | --- |
| Adopt | Separate phases, and a checker that only rejects | Parse, check and evaluate are distinct, and the program is "stateless, thread-safe, and cachable" [cel-go-pkg], [cel-go-readme]. Checking "does not affect the result of evaluation, it can only reject expressions as ill-typed" [cel-spec]. |
| Adopt | Termination by construction and a deterministic cost unit | "CEL programs cannot loop forever" [cel-spec]. Kubernetes relies on the same expression and input always costing the same [k8s-cel]. A timeout lacks this property, and the workflow engine cannot use one for its own state changes (the proposal, lines 297-301). |
| Avoid | Macros in place of lambdas | `all`, `exists`, `map` and `filter` are expanded at parse time, and there are no function values [cel-spec]. Two-variable forms came as an extension that Kubernetes enabled only in 1.33 [cel-ext], [k8s-cel]. |
| Avoid | Commutative `&&` and `\|\|` | `error && false` is `false` [cel-spec]. This hides errors and conflicts with nil narrowing in `x != nil && x.a > 3` (`expr:docs/reference/spec.md:778-779`). **(inference)** |
| Avoid | Three numeric types with no conversions | `1 + 1u` does not type-check [cel-spec]. Comparing a JSON number with an integer literal needed a cast. The maintainer answered "working as designed", and the issue is still open as "Support for union types" [cel-203]. |
| Avoid | `dyn` as the type of JSON | In a Kubernetes policy, `object.namex` passes the checker "even when the `namex` field is not defined" [k8s-cel]. |
| Avoid | Protobuf in the core | The AST is a protobuf message, and `go.mod` requires protobuf, with ANTLR and genproto as indirect dependencies [cel-overview], [cel-go-mod]. |
| Avoid | Escaping field names | A field named `namespace` is written `self.__namespace__`, and `x-prop` is `self.x__dash__prop` [k8s-cel]. |
| Pain | Static cost estimation | A rule on a string bounded by `maxLength` inside `allOf` is rejected with "estimated rule cost exceeds budget by factor of more than 100x" [k8s-134029]. The advice is to add size limits to every array, map and string [k8s-crd-blog]. A KubeCon 2024 abstract calls the budget "easy to violate" [kubecon-cost]. |
| Pain | The host API | The most upvoted cel-go issues include context propagation to functions, removing built-in functions, subsetting the standard environment, and reading Go structs by reflection [cel-issues-top]. |

### 2.2 expr-lang/expr

The most used Go-native expression library. It compiles to bytecode for a VM
whose stack is `[]any` and whose loop is a `switch` on the opcode
([exprlang-vm]).

| | Point | Evidence |
| --- | --- | --- |
| Adopt | The environment is a Go value | Pass a struct or a map to `expr.Env` and the checker infers types from it [exprlang-env]. Under P5 this is a convenience of the Go binding, and the lowest-friction one in the field. The library has no dependencies [exprlang-mod]. |
| Adopt | Result type assertions | `expr.AsBool()` rejects a non-boolean expression at compile time, and `WarnOnAny` makes an untyped result an error [exprlang-config]. The workflow engine needs this for `when:` (`workflow:llms.txt:318`). |
| Avoid | Types that stop at the first map | The docs give `object.unknown // nil (no error)` next to `struct.unknown // error` [exprlang-env]. |
| Avoid | Implicit variables | Predicates use `#` for the element, `.Field` as shorthand, and optional braces [exprlang-lang]. |
| Avoid | Reflection on the call path | A reflective call measured 545 ns against 126 ns for govaluate. The recommended form is now `func(params ...any) (any, error)` with a separate type [exprlang-300], [exprlang-func]. |
| Pain | Two advisories in 2025 | Memory exhaustion from an unbounded source string, fixed by a node budget, and a stack overflow in recursive builtins over nested or cyclic data, fixed by a depth limit [exprlang-ghsa-memory], [exprlang-ghsa-recursion]. |
| Pain | Limits and nil | "Sandboxing expression" is among the most discussed issues, and "Estimated expression cost" and "Nil safety" are open requests [exprlang-206], [exprlang-628], [exprlang-434]. |
| Pain | How Go values appear | "How to access pointer value?" is the second most discussed issue, and struct tag support is the most upvoted [exprlang-issues-comments], [exprlang-issues-top]. |

### 2.3 Starlark (go.starlark.net)

A Python dialect for configuration, embedded in Bazel and other tools.

| | Point | Evidence |
| --- | --- | --- |
| Adopt | Determinism as a stated property | "There are no sources of random numbers, clocks, or unspecified iterators" [starlark-spec]. |
| Adopt | The reasoning for leaving loops out | Unbounded loops "would encourage users to write more complex code" [starlark-design]. Recursion is an error unless a flag enables it [starlark-spec]. This supports E6. |
| Avoid | Freezing as the route to immutability | Values are mutable, then frozen when a module finishes, and a later mutation is a run-time error [starlark-spec], [starlark-impl]. v3 values are immutable from the start. |
| Avoid | Its own value universe | Integers are `big.Int`, and host data must be converted [starlark-impl]. |
| Pain | Getting host data in | A wrapper library exists because it is "actually quite difficult to get go data into a starlark-go script" [starlight]. |

### 2.4 HCL expressions and go-cty

Terraform's expression language and the typed value library under it.

| | Point | Evidence |
| --- | --- | --- |
| Adopt | Two kinds of key-value type | An object type gives each attribute its own type. A map type gives all elements one type [cty-types]. expr's closed objects and open objects with an element type are the same split (`expr:docs/reference/spec.md:734-741`, `:756-759`). |
| Adopt | Explicit, asymmetric conversion | Number to string is safe. String to number can fail. Number and bool do not convert [cty-convert]. HCL logic operators "apply only to boolean values" and equality requires "identical types" [hcl-spec]. |
| Adopt | Interpolation unwrapping | A template that is exactly one interpolation returns the value, not its text [hcl-spec]. The workflow engine has the same rule (`workflow:llms.txt:353-356`). |
| Avoid | Requiring branches to unify to one type | The conditional needs both results to "unify into a common type" [hcl-spec]. expr's checker joins branches into a union instead (`expr:docs/reference/spec.md:781-784`). |
| Avoid | Unknown values at run time | A cty unknown "carries a type but no value" [cty-concepts]. Terraform needs this to plan. The workflow engine instead passes "may not have settled" as an optional type (`workflow:llms.txt:311-315`). **(inference)** |
| Pain | Conditional types | On a Terraform issue, a maintainer wrote that "there probably cannot be a general solution in the Terraform language" without inline type conversions [tf-29809]. |

### 2.5 JSONata, JMESPath and jq

Query languages whose input is one JSON document.

| | Point | Evidence |
| --- | --- | --- |
| Adopt | No implicit conversion | "jq never does implicit type conversions" [jq-manual]. |
| Adopt | The adoption signal | AWS Step Functions added JSONata in November 2024 so that workflows could format dates and do arithmetic without custom code [aws-sfn]. |
| Avoid | Null for everything that goes wrong | In JMESPath a missing identifier is null, a wildcard on the wrong type is null, and an ordering comparison on non-numbers is null [jmespath-spec]. |
| Avoid | Results that change shape with their length | In JSONata a one-element array and its element "are deemed to be equivalent", and authors append `[]` to force an array [jsonata-predicate], [jsonata-processing]. No static type describes that. |
| Avoid | Implicit mapping through `.` | JSONata's `.` maps over arrays and JMESPath has projections [jsonata-path], [jmespath-spec]. |
| Pain | Reach and learnability | JMESPath's own proposal calls the inability to reach an outer value "a serious limitation" [jmespath-let]. On jq, one blog author writes "I've given up on ever learning the jq language" [jvns-jq]. |

### 2.6 GitHub Actions expressions and other `${{ }}` languages

| | Point | Evidence |
| --- | --- | --- |
| Adopt | Only the placement | Expressions sit in string fields of a declarative document [gha-expr]. |
| Avoid | Coercion on comparison | When types differ, both sides become numbers: null is 0, `true` is 1, a non-numeric string is `NaN`. String comparison ignores case [gha-expr-313]. |
| Avoid | A conditional built from `&&` and `\|\|` | The docs present `cond && a \|\| b` as the substitute for a ternary [gha-expr-313]. When `a` is falsy the result is `b`. **(inference from the falsy list on the same page)** The current docs add a `case` function [gha-expr]. |
| Pain | Values that are strings without saying so | A user set `VARF: ${{ false }}` and found that `${{ !env.VARF }}` is `false`: environment values are strings, and a non-empty string is truthy [gha-25645]. Truthiness turned a wrong type into a wrong answer. This supports B1. |
| Pain | Lenient helpers and unreachable names | Argo Workflows embeds expr-lang and adds Sprig. Its docs warn that Sprig's `int` returns 0 for an invalid value, and that a hyphenated parameter name "will cause a parsing error" [argo-vars]. |

### 2.7 Briefly: Rego, Jsonnet, CUE, Dhall, Lua, goja

| Language | Relevant lesson | Source |
| --- | --- | --- |
| Rego | A reference to an undefined value is undefined, and builtin errors are undefined unless strict mode is on. Schemas were added so the checker can report a misspelled field. Adopt schemas, avoid undefined. | [opa-lang] |
| Jsonnet | Hermetic and pure, but dynamically typed by choice and lazy: an error appears only when a field is used. Laziness defeats a save-time promise. | [jsonnet-design] |
| CUE | Types and values in one lattice. It is constraint solving, not evaluation over host data. Nothing to adopt. **(inference)** | [cue-logic] |
| Dhall | Total and typed before evaluation, yet a finite program can take "longer than the heat death of the universe". Termination does not replace a budget. | [dhall-safety] |
| gopher-lua | Its own value type. An `LState` is not goroutine-safe. Passing a context for cancellation slowed its example from 5.3 s to 7.5 s. | [gopher-lua] |
| goja | A full ECMAScript engine. One runtime per goroutine. Its README describes interruption from another goroutine and no step or memory limit. | [goja] |

## 3. More than one implementation (C1)

### 3.1 How others did it, and what it cost

| Language | Definition and tests | Implementations | Where they diverge, and the cost |
| --- | --- | --- | --- |
| CEL | A language definition, plus conformance tests "driven by data files" that give parse, check and evaluation behavior. Reference runners exist for Go, C++ and Java. [cel-tests] | Go, C++ and Java from the project. Others include TypeScript (beta), JavaScript, Python and Rust. [cel-es], [cel-js], [cel-python], [cel-rust] | The definition leaves `string(double)` formatting, map order, evaluation order and which of several errors is reported unspecified [cel-spec]. The TypeScript one represents `int` as `bigint` and `uint` as a wrapper class, and ships its own port of an RE2 engine [cel-es], [cel-es-re2]. Another says it covers "Most of the CEL Spec" and expects host integers as `BigInt` [cel-js]. The Python one does no type check and documents a division difference [cel-python]. The Rust one runs the project's conformance data and keeps a list of the cases it ignores, 728 lines long on the date above [cel-rust-conformance]. |
| JSONata | A reference implementation in JavaScript and "a language neutral test suite": one JSON file per case with the expression, input, bindings, an optional `timelimit` and `depth`, and a result or an error code. [jsonata-suite] | Ports in several languages, with Java, Go and Rust among them. [jsonata-java], [jsonata-go], [jsonata-rs] | A Java port claims "100% reference test coverage" with exceptions kept in an override file, and still documents a list of regular expression differences, because it uses Java's engine [jsonata-java]. A product that changed Java ports found its depth limit now scaled with input size and one expression went quadratic [kestra-102]. The Go port claims "feature parity with jsonata-js 1.5.4" and most of what came later [jsonata-go]. The Rust port calls itself "(incomplete)": no regular expressions, no parent operator, and "Many built-in functions are missing" [jsonata-rs]. |
| JMESPath | A specification and compliance tests in JSON: `given`, `cases`, `expression`, and `result` or `error`. [jmespath-test] | Several. | The error type says what to raise "but it does not indicate **when**": at compile time or at run time is left open [jmespath-test]. |
| JSON Schema | A specification and a suite that "should require only a JSON parser", used by about 60 projects, with an `optional` directory. A separate tool runs implementations in containers and publishes which tests each fails. [jsonschema-suite], [bowtie] | Dozens. | Regular expressions: the dialect is ECMA 262, but "that complete syntax is not widely supported, therefore it is recommended that you stick to the subset" [jsonschema-regex]. |
| Starlark | A specification. Go, Java and Rust implementations. [starlark-lang-spec] | Three. | The size of a string element is "either 8 or 16, depending on the implementation", so lengths and indexes differ [starlark-lang-spec]. In the Go one, "the precise meaning of 'step' is not specified and may change" [starlark-go-pkg]. The Rust one says "We mostly follow the Starlark standard" and lists where it does not: extensions such as recursion and a top-level `for`, no `bytes` type, and "In some cases creating circular data structures may lead to stack overflows" [starlark-rust]. |
| Jsonnet | A C++ implementation and a Go one. They share the standard library source, the C API headers and some tests through a submodule. [go-jsonnet] | Two from the project. | Release notes still record fixes of the form "was already the case in go-jsonnet, now fixed in C++ jsonnet" [jsonnet-021]. |
| jq | An implementation and its manual [jq-manual]. gojq is an independent Go one. | Two in wide use. | gojq documents its differences: arbitrary-precision integers, no preserved key order, and no backreferences or look-around in regular expressions [gojq]. |
| Rego | One Go implementation. Policies also compile to WebAssembly. A Rust reimplementation offers bindings to eight languages. [opa-wasm], [regorus] | One, plus two routes to other hosts. | Wasm: the host writes data as a JSON string into the module's memory and parses it there, and supplies any builtin the module lacks [opa-wasm]. Rust: "mostly compliant", a list of missing builtins, and "we currently don't publish these bindings" [regorus]. |
| Cedar | A formal model in Lean, with proofs. The Rust implementation "is then tested against the model with differential random testing". The minimized fuzzing corpus is published as an integration test corpus. [cedar-self], [cedar-spec] | Rust from the project. An "independent Go implementation" that "also runs the integration test corpus". Java calls the Rust library through a foreign function interface. JavaScript gets a WebAssembly build of "JSON-in, JSON-out functions". [cedar-self], [cedar-java] | The Go one lacks the formatter, partial evaluation and policy templates, and its validator is experimental [cedar-go]. A nightly job tells its maintainers when the upstream corpus has changed [cedar-go-corpus]. In October 2026 an advisory reported that its JSON decoder accepted a malformed entity record and dropped the record's other keys, "turning a `deny` into an `allow`" [cedar-go-ghsa-entity]. The Java binding "typically lags behind the newest Cedar features" [cedar-java]. The Wasm file is 4.4 MB [cedar-wasm-npm]. |
| MiniJinja | Jinja2's behavior, a compatibility document, and the reference tests of the Rust implementation. [minijinja-readme], [minijinja-go] | Rust, and "a native Go port". JavaScript through a WebAssembly build of the Rust one, Python through an extension module. [minijinja-readme], [minijinja-go] | The Go port "passes the reference tests from the Rust implementation with minor adjustments", and is an AST walker where the Rust one is "a bytecode interpreter VM" [minijinja-go]. Release 3.1.0 still has seven entries headed "Go:" that bring it into line with Rust: integers outside 64 bits, map key order, and filters "silently accepting invalid values" [minijinja-changelog]. The project says of its Wasm route that "it would not be wise to use this for actual template rendering in the browser", because of its size and speed [minijinja-js]. |
| Dhall | A standard and a test suite. [dhall-integrate] | Native in five languages. Others wrap the Haskell or Rust one. | The project says a native implementation is "the only way to get complete support" [dhall-integrate]. |

Five patterns follow. **(inference)**

- **The suite is the specification that implementations actually obey.** Where
  it is silent, they differ: CEL's number formatting, JMESPath's error timing,
  Starlark's string units.
- **Ports reuse the host platform's library, and that is where behavior leaks
  in**: Java's regular expressions, Go's maps, JavaScript's numbers.
- **Second implementations lag.** "Most of the spec", "mostly compliant" and a
  list of missing builtins are the normal state. A public report of which tests
  each implementation fails, as JSON Schema has, makes the lag visible instead
  of surprising. The Rust and Go evidence says where the lag sits: in the
  checker and the library. cedar-go has the authorizer, and its validator is
  experimental. Regorus and jsonata-rs evaluate the language and list the
  functions they lack. This is the evidence for making 3.0 final only when all
  three pass the whole suite (D27).
- **A second implementation diverges at its edges before its core.** cedar-go's
  advisory was in how it decoded JSON, not in how it evaluated a policy.
  MiniJinja's Go fixes are about integer ranges, slice bounds and which
  arguments a filter accepts. The conformance suite needs cases for the binding
  and for bad input, not only for expressions.
- **The two projects I found with Rust, Go and JavaScript hosts do not write
  JavaScript natively.** Cedar and MiniJinja each have a Rust implementation, a
  native Go one, and a WebAssembly build of the Rust one for JavaScript. A
  native TypeScript implementation is the part of R1 that neither has. CEL has
  one ([cel-es]), and MiniJinja's own warning about its Wasm route in a browser
  is the argument for it.

### 3.2 Independent implementations, or one core compiled to WebAssembly

**Evidence on one core.**

- **Data must be copied in.** OPA's Wasm host calls `opa_malloc`, writes the
  JSON text, and calls `opa_json_parse` for every input ([opa-wasm]).
  wasm-bindgen "copies the string's contents back and forth between the
  JavaScript garbage-collected heap and the Wasm linear memory", converting
  between UTF-16 and UTF-8 ([wasm-bindgen-str]). An embedded QuickJS returns
  handles into the Wasm heap that "cannot be garbage-collected as usual in
  Javascript" and must be disposed by hand ([quickjs-emscripten]). Risor v2's
  cost was exactly this conversion: 25 µs of a 35 µs evaluation, growing with
  the size of the scope (the proposal, lines 481-486).
- **A Rust core copies as well.** Cedar's JavaScript package is a WebAssembly
  build of "JSON-in, JSON-out functions" ([cedar-self]). Regorus reaches Go
  through cgo, and its Go binding takes input as a JSON string and returns each
  result as a string ([regorus], [regorus-go]). MiniJinja's JavaScript binding
  converts plain objects, arrays and maps on the way in. Class instances "are
  not copied but accessed lazily" ([minijinja-js]), which is a call across the
  boundary for each read. **(inference)**
- **Size.** Go's own guidance puts the smallest Wasm binary at "around ~2MB",
  about 500 kB compressed, with "10MB+ is common" ([go-wasm]). A C interpreter
  is smaller: QuickJS is 500 kB as Wasm ([quickjs-emscripten]). A Rust policy
  interpreter is 1.9 MB to 6.3 MB as a native binary ([regorus]). A Rust engine
  as Wasm can come in under Go's floor. Rhai's book says a full engine "compiles
  to a single WASM32 file that is less than 400KB (non-gzipped)" ([rhai-wasm]).
  The Wasm file in MiniJinja's published JavaScript package is 808 kB, and the
  one in Cedar's is 4.4 MB ([minijinja-js-npm], [cedar-wasm-npm]). These are
  three different languages, so they give a range for v3, not a figure.
  **(inference)**
- **Speed and startup.** esbuild, a Go program, says its Wasm build is "much,
  much slower than the native version", often by 10x, in part because Node
  recompiles the module on every run ([esbuild-wasm]).
- **Packaging and debugging.** esbuild's native route needs a binary per
  platform chosen by the package manager, and its docs list several ways that
  fails ([esbuild-wasm]). Go's Wasm guidance says to debug with `println`
  ([go-wasm]). Regorus does not publish its bindings ([regorus]).
- **A Rust core in WebAssembly is not the same program as the native one.**
  Cedar checks the remaining stack "only on platforms that the `stacker` crate
  supports, which excludes Wasm" ([cedar-self]). Sizes are 32 bits wide there:
  MiniJinja fixed a slice step of 4294967296 that panicked "on 32-bit targets
  such as WebAssembly" ([minijinja-changelog]). And "If the engine panics, the
  WASM runtime corrupts" ([minijinja-js]).

**Evidence on independent implementations.** Section 3.1 is the cost: divergence
wherever the suite is silent, and lag. The benefit is that each implementation
uses its host's values directly, which is the property v3 is built on, and that
each is ordinary code in its host's language for packaging and debugging.

**Recommendation, now rulings R1 and R8: three native implementations, in Go,
Rust and TypeScript, each written from the specification and one conformance
suite. The Rust implementation, built for WebAssembly or behind a C interface
with data copied in, is the fallback for a host in any other language.** The
reasons:

1. Reading host values in place is a standing decision, and no boundary-crossing
   design keeps it. A Go core compiled to Wasm would also be the largest and
   slowest option in a browser.
2. The Rust implementation is not a shared core for the other two. A Go or
   TypeScript host that reached it through cgo or a Wasm runtime would pay the
   copy that reason 1 rules out, which is how Regorus serves Go and how Cedar
   serves JavaScript ([regorus-go], [cedar-self]). So each of the three hosts
   gets a native implementation, and only a host in a fourth language pays the
   copy. **(inference)**
3. The language is small by ruling: expressions only, no loops, no recursion
   (E5, E6). CEL, a comparable language, has implementations in Go, C++ and Java
   from its project and several from outside it. Cedar and MiniJinja each have a
   native Go implementation beside the Rust one ([cedar-go], [minijinja-go]).
4. Two of CEL's costs in TypeScript do not arise in v3: one kind of number needs
   no `bigint` (N1), and regular expressions are either a specified subset or
   absent (T4).
5. The Rust implementation is the real test of the specification. The Go and
   TypeScript ones could share a design, because both have a garbage collector
   and both can look inside a host value at run time. Rust has neither, and it
   cannot survive a stack overflow, so it cannot be written by transcribing the
   other two (section 3.4). Where the specification leans on something Go and
   JavaScript both happen to provide, Rust is where it shows. **(inference)**
6. Rust is the right base for the fallback. It needs no runtime or collector
   inside the module, the smaller Rust engines above are under Go's 2 MB floor
   as Wasm, and Regorus offers C, C++, C#, Go, Java, Python and Ruby bindings
   and a Wasm build over one Rust library ([regorus]). The fallback is still a
   separate target with its own faults, as the last bullet above shows, so it
   has to pass the conformance suite in its own right before it is published.
   **(inference)**

What makes this work, from the prior art:

- **The suite is data** (P4), and it carries limits. JSONata's cases have
  `timelimit` and `depth` fields ([jsonata-suite]). Because L6 fixes step and
  allocation counts, a case can assert the exact count, which also catches an
  implementation that took a different path to the right answer. **(inference)**
- **Error classes and their timing are specified** (P6), which JMESPath left
  open.
- **Tables are generated once and shared**: the cost table, case mapping if any,
  and the number formatting test vectors. Jsonnet shares its standard library
  source between implementations ([go-jsonnet]).
- **A published conformance report per implementation**, in the manner of JSON
  Schema's ([bowtie]). The fallback build gets its own column.
- **Differential testing between the implementations.** Cedar generates
  "millions of diverse inputs", feeds them to its model and to its Rust code,
  and publishes the minimized corpus, which the Go implementation then runs
  ([cedar-drt-blog], [cedar-self]). The method found "corner cases where there
  were discrepancies between the model and the production code", one of them a
  bug in a Rust package used for IP addresses ([cedar-drt-blog]). v3 has three
  implementations to compare and a small grammar to generate programs from, and
  every disagreement becomes a suite case. **(inference)**

### 3.3 The portability traps the specification must pin down

| Trap | Prior art that hit it | What the specification must say |
| --- | --- | --- |
| Integer width against JavaScript doubles | CEL chose 64-bit integers, and its JavaScript implementations use `bigint` [cel-es], [cel-js]. Dart has 64-bit integers natively and doubles on the web, where "integers lose precision past 53 bits", overflow differs and numbers print differently [dart-numbers]. Lua 5.3 listed "integers (64-bit by default)" first among its main changes [lua53]. Jsonnet, doubles only, added safe-range checks to bitwise operations in 0.21 [jsonnet-021]. | N1 and N2 take the Jsonnet side and go further: every integer result outside the safe range is an error. I found no language that already does this for all arithmetic. See open question 2. |
| Large integers from the host | ProtoJSON writes 64-bit integers as decimal strings because many implementations "are silently lossy" above 2^53 [protojson]. CEL's JSON conversion does the same outside ±(2^53 − 1) [cel-spec]. RFC 8785 recommends strings [rfc8785]. | N2's "the host passes it as a string" is the established answer. |
| Checking a range after rounding | A reviewer of Jsonnet's check noted that the literal `9007199254740993` passes, because it was rounded when parsed [jsonnet-1217]. | Range checks on literals and on text-to-number conversion are made on the decimal text, not on the double. |
| String length and index units | Starlark: 8-bit or 16-bit elements by implementation, so `len("😀")` is 4 or 2 [starlark-lang-spec]. CEL: code points [cel-spec]. JSONata: "the number of characters", not defined further [jsonata-string]. | T1 says code points. Also say what a host string with an unpaired surrogate is. wasm-bindgen replaces them with U+FFFD [wasm-bindgen-str]. A Rust `String` cannot hold one at all: "`String`s are always valid UTF-8" [rust-string]. So a string with an unpaired surrogate cannot be a v3 value, and the TypeScript binding must reject or replace it on the way in. **(inference)** |
| String and key ordering | CEL orders by code point [cel-spec]. RFC 8785 sorts keys by UTF-16 code unit, because that is the native string of ECMAScript, Java and .NET [rfc8785]. | T2 says code point. A TypeScript implementation needs its own comparison. A host that wants RFC 8785 output must re-sort keys outside the basic plane. **(inference)** |
| Regular expression dialect | CEL requires RE2 syntax and linear time [cel-spec]. Its TypeScript implementation ships a fork of an RE2 port with match-only operations, and its Python one bundles an RE2 binding [cel-es-re2], [cel-python]. Ports that used the host engine diverged [jsonata-java], [gojq]. V8's linear engine is experimental, behind flags, and lacks backreferences and look-around [v8-linear-regex]. Rust's `regex` crate has "worst case `O(m * n)` time complexity" and no look-around or backreferences. Its defaults differ from Go's: in Rust `\w`, `\d` and `\s` "are all Unicode-aware by default" and `\b` uses "a Unicode definition", where Go's `\d` is `[0-9]`, its `\w` is `[0-9A-Za-z_]` and its `\b` is an "ASCII word boundary". Rust's bound also "does **not** apply" to iterating over all matches, which is `O(m * n^2)` in the worst case [rust-regex], [go-regexp-syntax]. | T4: a subset, with a matcher shipped by any implementation whose host engine backtracks. CEL shows the cost is one ported engine. With Rust in the set only TypeScript needs one (R11). The subset must define each class by its members, and "find every match" needs its own cost formula or it breaks L2. **(inference)** |
| Number to text | CEL leaves `string(double)` unspecified [cel-spec]. RFC 8785 requires the ECMAScript algorithm and tells other languages to use a Ryu-style algorithm and test against V8 [rfc8785]. For Rust, `ryu-js` is a fork of `ryu` "adjusted to comply to the ECMAScript number-to-string algorithm" [ryu-js]. I did not test it. The default formatters disagree: `1e21` prints as `1000000000000000000000` in Rust and as `1e+21` in Go and JavaScript, and `1e-7` prints as `0.0000001` in Rust, `1e-07` in Go and `1e-7` in JavaScript **(run)**. | N7's candidate is the choice RFC 8785 made. Ship test vectors in the suite. No implementation may call its host's default formatter. |
| Text to number | The host parsers accept different text **(run)**. Rust and Go accept `inf` and `nan`. Go accepts `1_000`. JavaScript's `Number` accepts `0x10`, a leading space, and the empty string as 0. For `1e400`, Rust and JavaScript give infinity and Go gives an error. | N7 covers this direction as well. The specification gives the grammar, and an implementation checks the text against it before it calls any host parser. |
| Case mapping | The CEL string extension that Kubernetes enables lists only `lowerAscii` and `upperAscii` for case [k8s-cel]. Go's tables follow one Unicode version (17.0.0 today) and have "no mechanism for full case folding" involving several runes [go-unicode]. CEL's TypeScript RE2 package carries its own Unicode 16.0 data [cel-es-re2]. Rust's tables follow `char::UNICODE_VERSION`, and an update "is *not* considered to be a breaking change": Unicode 15 came with Rust 1.66.0 and Unicode 17 with 1.94.0 [rust-char], [rust-releases]. Go made the same jump one release ago: 1.26 has the tables of Unicode 15.0.0 [go-unicode-126]. On the machine used here rustc 1.99.0 and Node 22 report 17.0 and Go 1.26.2 reports 15.0.0 **(run)**, so the version depends on the toolchain that built the host. The algorithm differs as well as the table. Rust documents that "One character can become multiple" [rust-str]. In upper case `tschüß` is `TSCHÜSS` in Rust and JavaScript and `TSCHÜß` in Go, and a final `Σ` is lowered to `ς` in Rust and JavaScript and to `σ` in Go **(run)**. | T3: ASCII-only is exact. Anything wider is implementation-defined or needs a table the specification owns (D25). That table must also fix the algorithm: whether one character may become several, and whether position in the word counts. |
| Map order | Starlark: insertion order [starlark-spec]. JavaScript objects: integer-like keys first in ascending order, then creation order [mdn-forin]. gojq "does not keep the order of object keys" [gojq]. CEL: unspecified [cel-spec]. | T2. A TypeScript implementation cannot use object order. |
| Math functions | Go "does not guarantee bit-identical results across architectures" [go-math]. For JavaScript, "even the same JavaScript engine on a different OS or architecture can give different results" [mdn-math]. Rust says the precision of `f64::sin` "varies by platform, Rust version, and can even differ within the same execution from one invocation to the next" [rust-f64]. | T3: implementation-defined, and the suite compares within a tolerance. |
| Evaluation order and which error | CEL: order "is not guaranteed", and which of several errors propagates is "not specified" [cel-spec]. | Left to right, first error wins. Otherwise error cases in the suite are not portable. |
| Division | Python changed `/` to true division because an integer finding its way into an expression silently changed the result [pep-238]. A CEL port had to document that its host's integer division rounds differently [cel-python]. | N4, with truncation as a named function. |
| The implementation's own stack | A stack overflow in Rust prints a message and aborts the process [rust-stack-overflow]. cel-rust's parser overflowed on nested brackets and now stops at 96 levels [cel-rust-212], [cel-rust-parser]. Cedar checks the remaining stack while it evaluates, but not while it parses Cedar syntax or loads entities, and not at all in WebAssembly [cedar-self], [cedar-2660]. MiniJinja overflowed on a value built by about 1,500 concatenations, with no deep source at all [minijinja-907]. Rhai's limits are lower in a debug build: 32 and 16 levels of nesting where a release build has 64 and 32 [rhai-expr-depth]. | R5 and R6. Limits on source nesting, call depth and value nesting are counts, with the numbers in the specification and cases in the suite. The remaining stack is not a limit: it differs by platform, build and thread. |
| Machine integers and word size | Rhai without its checks panics on overflow in a debug build and wraps in a release build [rhai-checked]. MiniJinja fixed a `range` that "panicked with overflow checks enabled" and gave a wrong answer without them, and a slice step of 4294967296 that panicked "on 32-bit targets such as WebAssembly". The same list has its Go port "panicking or giving wrong results for steps and bounds near the 64-bit integer limits" [minijinja-changelog]. | N2 lets an implementation store integers as machine integers. The suite then needs cases at 2^31, 2^32, 2^53 and 2^63 for arithmetic and for every index, count and repeat argument. The fallback build has 32-bit sizes. **(inference)** |

### 3.4 What a Rust implementation adds

Go and JavaScript both have a garbage collector, and both let a library look
inside a value it was handed. Rust has neither, and it cannot recover from a
stack overflow. This section is the evidence for rulings R2 to R8, R11 and R12.

**Host data without reflection.** Serde's own description is the starting point:
"Where many other languages rely on runtime reflection for serializing data,
Serde is instead built on Rust's powerful trait system" ([serde]). An engine in
Rust learns the shape of a host type from code the host compiles in.

| Project | How host data gets in | Read in place, or copied | What the host writes |
| --- | --- | --- | --- |
| MiniJinja | `Value::from` for the standard collections. The `Object` trait for the host's own types. A `Serde` wrapper, which has been optional and off by default since 3.0.0, when `Value::from_serialize` was removed. | In place for collections and objects: "The values in the collections are lazily converted into value when accessed or iterated over", and "the types are boxed unchanged". The engine must own the value: `from_object` asks for `Send + Sync + 'static`. | For a struct, `get_value` to read a field by name and `enumerate` to list the names, by hand. [minijinja-value], [minijinja-object], [minijinja-changelog] |
| cel (Rust) | `add_variable` takes any type that implements `serde::Serialize`. `add_variable_as_val` takes a boxed `Val`. A `VariableResolver` is asked for each name. | The first converts to a `Value` whose lists and maps "hold eagerly-materialized contents". The second resolves "its contents *on access*", and "may borrow data for `'v`". The resolver's documentation has an example headed "Borrowing instead of copying". The evaluator's result "borrows where it can". | Nothing for the copy. To read in place: `Val`, and `Indexer`, `Iterable` and `Sizer` as needed, for each type. [cel-rust-context], [cel-rust-objects] |
| Rhai | Every value is a `Dynamic`. Any type that is `Clone` can be registered. Serde converts to and from `Dynamic`. | By value: `Clone` is required because "this allows the `Engine` to pass by value". A host that wants sharing wraps its data in `Rc` or `Arc`. | Getters, methods and indexers, registered one at a time or generated by `#[derive(CustomType)]`. [rhai-custom-types], [rhai-serde] |
| starlark-rust | Host types "live in the Starlark heap", which is garbage collected. | Moved into that heap. | The `StarlarkValue` trait, an attribute macro and several derives. The crate says "Such types are relatively complex". [starlark-rust-lib], [starlark-rust] |
| Regorus | `Value::from_json_str`, then `set_input` and `add_data`. | Copied into its own `Value`. **(inference)** | JSON text. [regorus] |
| Cedar | Entities and the request context, from JSON or built through the API. | Copied into its own entity store. **(inference)** | JSON or entity objects. [cedar-self] |
| wirefilter | A `Scheme` declares each field and its type. `set_field_value` supplies each one for a run. | A value may borrow from the host for as long as the context lives (`'v: 'e`). | One call per field. [wirefilter], [wirefilter-ctx] |

What follows for v3. Each point is an **(inference)**.

- **Reading in place is possible in Rust, and it is opt-in.** MiniJinja,
  cel-rust and wirefilter do it. In each the host implements a trait or sets
  each field. The path that needs no host code is serde, and serde copies. A
  Rust host that takes the easy path loses the property v3 is built on, so the
  binding should make the trait as easy as the copy: a derive macro, and
  implementations for the standard collections and for `serde_json::Value` (R2).
- **The trait is small.** MiniJinja's asks for two things, a member by key and
  the list of keys, and it has a kind that says map or sequence. That is the
  "object with named fields" of the embedding model, and it is the same question
  Go answers by reflection and JavaScript by property lookup.
- **Rust has two forms of "in place", and they are not the same promise.**
  MiniJinja owns the host value for as long as the engine wants it. cel-rust and
  wirefilter borrow it for a stated lifetime. Only the second lets a host lend a
  value it does not want to give away or wrap in `Arc`. It is what lending
  values for the length of a run needs, and it makes S2 and S3 something the
  compiler checks (R7).
- **Types cannot come from reflection either.** P5 already makes type text and
  JSON Schema the portable ways to declare an environment. Deriving types from
  Rust structs would be a second derive macro, not a library call.

**No garbage collector.** MiniJinja's values are "immutable objects which are
internally reference counted", with the warning that "Special care must be taken
so that cycles are not created to avoid causing memory leaks"
([minijinja-value]). starlark-rust built its own collected heap
([starlark-rust]). jsonata-rs takes an arena from the caller: "you must provide
a `bumpalo` arena" ([jsonata-rs]). v3 needs none of the three workarounds for
cycles, because its values are immutable and nothing in the language can make a
value refer to itself (R3). **(inference)** Rust does have something Go lacks, a
hook on allocation. Regorus counts "the requested size of every successful
allocation" through an allocator shim ([regorus-memory]), where Go offers only a
limit on the whole runtime ([go-memlimit]). A Rust test harness can therefore
compare charged units with real bytes exactly. **(inference)**

**Stack overflow.** The Rust runtime's handler for a stack overflow exists
"purely to print an informative message", and then "We then abort"
([rust-stack-overflow]). The stack of the main thread "is *not* determined by
Rust", and the default for other threads is "platform-dependent and subject to
change" ([rust-thread]).

| Project | Depth of source | Depth of evaluation | Walks over values | Source |
| --- | --- | --- | --- | --- |
| Rhai | Nesting is limited to 64 levels at the top and 32 in a function, because "it is possible to overflow the `Engine`'s stack when it tries to recursively parse". | Call depth 64, and 8 in a debug build, "to fit into most platforms' default stack sizes". The book warns that a call inside a deep expression "may still overflow the stack". | Not on the pages I read. | [rhai-expr-depth], [rhai-call-stack] |
| MiniJinja | "the maximum recursion at parsing time" is "always limited". | A counter weighted by operation, 500 by default. Going higher needs the `stacker` feature, and "in WASM the maximum limits are additionally enforced by the runtime". | A chain of lazy concatenations overflowed in `len` at about 1,500 links. The fix builds a flat value. | [minijinja-lib], [minijinja-env], [minijinja-907] |
| cel (Rust) | 96 levels, added in October 2025 after a report of a stack overflow. | None found. | None found. | [cel-rust-212], [cel-rust-parser] |
| starlark-rust | — | A maximum call stack size, 50 by default. | A thread-local counter for equality and similar walks: 3,000 levels, and 200 in a debug build. A comment beside it asks: "If we build a deep structure so deep equals dies, won't we just die in `drop` anyway?" | [starlark-rust-eval], [starlark-rust-guard] |
| Cedar | No check for Cedar syntax "on any platform". Parse functions with a depth limit are in the unreleased changes. JSON stops at serde_json's 128 levels. | `stacker::remaining_stack()` compared with 100 kB, giving a `RecursionLimit` error. "Wasm has no such check." | "Long entity parent chains can also overflow the stack while entities load", an open issue filed in October 2026. | [cedar-self], [cedar-eval], [cedar-changelog], [cedar-2660] |
| serde_json | 128 levels unless the caller turns the limit off. | — | With the limit off, the caller must watch "Display and Debug and Drop impls". The example frees a nested value with an explicit stack. | [serde-json-depth] |
| regex-syntax | A nest limit, kept for the sake of "consumers that do structural induction on an `Ast` using explicit recursion". The crate itself works by "moving the call stack to the heap". | — | — | [regex-syntax-nest] |

What follows for v3. Each point is an **(inference)**.

- **Count, do not probe.** Cedar probes the remaining stack. That gives no
  protection in WebAssembly, and the depth at which it fails depends on the
  platform and the thread. A count fails at the same depth everywhere, which is
  what a conformance case needs (R5).
- **A number sized to the stack is not portable.** Rhai's limits differ between
  debug and release builds. A limit in the specification is one number, so the
  Rust implementation must fit it in its least favorable build, by keeping
  frames small or by running on a stack it sizes itself. D17's depth of 200
  should be tried in a Rust debug build and in WebAssembly before it is fixed.
- **Source depth is not the only depth.** MiniJinja's overflow came from a value
  built one step at a time by a loop. v3 has no loops, but a `reduce` whose step
  wraps its accumulator in a list nests a value as deep as its input is long.
  Every walk over a value needs a bound or an explicit stack: equality,
  ordering, rendering, encoding, and in Rust the release of the value (R6).
- **Parsers are where the reports are.** cel-rust and Cedar each crashed or ran
  out of memory in the parser, not the evaluator ([cel-rust-212],
  [cel-rust-ghsa-memory], [cedar-ghsa-has]). D31 asks for a maximum nesting of
  source. Parse-time memory needs a rule as well (section 8, runtime).

**Execution strategy.** The Rust evidence is thinner than the Go evidence of
section 6, and it does not all point one way.

- Cloudflare's filter engine moved from interpreting the AST to a tree of boxed
  closures and saw "an immediate ~10-15% runtime improvement in benchmarks and
  on real examples". It rejected generated native code: "No native code is
  generated at runtime" ([wirefilter-blog]).
- Rhai walks its AST and says it is "roughly 2-3x slower than Python 3". Its
  newer bytecode VM is "roughly 2x faster" than its AST walker ([rhai-bench]).
- MiniJinja is a bytecode VM in Rust and an AST walker in its Go port
  ([minijinja-go]). cel-rust walks its AST ([cel-rust-objects]). starlark-rust
  compiles to bytecode ([starlark-rust-bc]).
- Regorus has an interpreter and a newer VM. Its README reports that the VM
  "achieved 1.35-2.15x faster median authorization than Cedar" on benchmarks
  adapted from the Cedar paper ([regorus]).

No bytecode VM is a standing decision, and nothing here overturns it: the one
measured comparison of closures in Rust favors them. The gain is smaller than
Vitess reported in Go, and Rhai found bytecode about twice as fast as walking
its tree. Evaluation strategy is not normative (L6 counts over the program as
written), so the Rust implementation can measure before it commits.
**(inference)**

**What the Rust standard library and crates supply.** The details are in the
table of section 3.3.

| Need | Rust has | Usable as it is? |
| --- | --- | --- |
| A linear-time regular expression engine | The `regex` crate [rust-regex] | For time, yes. For results, only with classes pinned and "find every match" charged for (R11). |
| ECMAScript number to text | `ryu-js` [ryu-js] | Probably. It claims the algorithm, and the suite's vectors will show. The standard formatter is not usable. |
| Text to number | `str::parse` | No. It accepts `inf` and `nan` **(run)**. |
| Case mapping | The standard library, at the compiler's Unicode version [rust-char] | No. The version moves with the compiler (R12, D25). |
| Transcendental functions | The standard library, with unspecified precision [rust-f64] | Only as implementation-defined (T3). |
| Strings as code points | `String` is UTF-8 and always valid [rust-string] | Yes. A count of UTF-16 units (L7) is a scan. |

## 4. Limits on work, memory and time (C5)

### 4.1 What others built

| System | Work | Memory | Time | What went right or wrong |
| --- | --- | --- | --- | --- |
| starlark-go | A step counter with a maximum. "The precise meaning of 'step' is not specified and may change." | No limit in the API index. **(inference)** | `Cancel` from any goroutine. "There may be a delay … if the thread is currently in a call to a built-in function." | Steps are "incremented by the interpreter". Nothing says a builtin adds to them. **(inference)** [starlark-go-pkg] |
| Canonical's Starlark fork | One step per builtin call, and a builtin calls `AddSteps` for significant work. Steps are "not the actual CPU usage". | Each builtin calls `AddAllocs` or `CheckAllocs` with a byte estimate, and the check can come before the allocation. Freed values are not subtracted. | A `TimeSafe` flag: stop "within a reasonable amount of time" when cancelled. | Builtins declare safety flags. "The contract is upheld through testing", with a harness that scales a workload by N and bounds steps and allocations per N. Caveats: "Don't be surprised if you've missed something", and size estimates undercount maps. [canonical-starlark], [canonical-safety] |
| MiniJinja (Rust and Go) | Fuel. "every instruction executed: usually one fuel", plus "the items that built-in filters and operators iterate over, and the comparisons made while sorting". | The same counter: "roughly one fuel per 64 bytes" of output and of the strings and lists a template creates. "Where the size of a value is known ahead of time (for instance for `"x" * n`), the value is only created if the remaining fuel can pay for it." And "fuel only ever goes down". | None on the pages I read. | Until 3.1.0 fuel counted instructions only. A host filter is "only charged for the value they return" unless it calls `consume_fuel`. "The exact costs are not part of the stability guarantees and can change between releases". The Rust and the Go implementation both have it. [minijinja-env], [minijinja-changelog] |
| Rhai (Rust) | A maximum number of operations, off by default. One operation "roughly equals one expression node, loading one variable/constant, one operator call, one iteration of a loop, or one function call". A call into Rust is "counted as only one operation". | No budget. Caps on the length of one string and the size of one array or map, summed over what it contains. "a registered function may grow a string's length without Rhai noticing until the very end", so a result "may be almost *twice* the maximum length". For memory as a whole the book says to ask the operating system from a progress callback. | A progress callback can stop the run. | The book calls the meaning of an operation "volatile". The caps are per value, not cumulative. Every check can be compiled out with the `unchecked` feature. [rhai-ops], [rhai-string-size], [rhai-array-size], [rhai-memory], [rhai-checked] |
| starlark-rust | A maximum tick count. A tick is "either one function call or one loop backedge", chosen because it is "deterministic, cheap to implement, and most importantly is *stable*". It is "not very "fair"". | A maximum heap size, "not enforced on allocation, but instead checked once every so often". It counts the Starlark heap only, so "Even a large dict costs only constant memory". | A cancellation callback "called periodically". | Both limits are "best-effort". For a guarantee: "Use OS-level APIs in a subprocess". [starlark-rust-eval] |
| Regorus (Rust) | Work units, counted by `tick`. | A budget per run on requested bytes, counted by an allocator shim. "Frees never refund the total." "Enforcement is cooperative, not an allocation-time peak-memory cap", so one builtin "can temporarily overshoot before the next check". | A wall-clock limit, with the clock read once per `check_interval` work units. | The memory budget is cumulative, like L3, but it is measured in bytes after the fact, so it is not the same number on two platforms. **(inference)** The time limit is the design of L5. [regorus-memory], [regorus-time] |
| cel (Rust) | None in the sources I read. | None in the sources I read. | None in the sources I read. | A limit on parser depth only. An advisory in 2026 reported that 125 bytes of input made the parser use more than 1 GiB. [cel-rust-parser], [cel-rust-ghsa-memory] |
| expr-lang | None in the VM source I read. | A budget counted in elements at range, array, map and sort. A node limit at compile time. | None in the pages I read. | The 2025 crash was in builtins outside the accounting. [exprlang-vm], [exprlang-ghsa-memory], [exprlang-ghsa-recursion] |
| CEL in Kubernetes | A runtime cost budget in deterministic units. Regular expressions cost pattern length times input length. | Through cost only. | Not in the pages I read. | Halts with an error. When library costs changed, a reviewer asked whether they needed a version bump and whether stored rules would still be accepted. The answer was that version bumps had been used before. The thread did not settle it. [k8s-cel], [k8s-138227] |
| Tengo (Go) | Not on the page I read. | `SetMaxAllocs`: "a cumulative metric that tracks only the object creations". String and byte length caps that are process-wide. | Not on the page I read. | A count of objects, not of their size. [tengo-interop] |
| Lua and Luau | A hook "called after the interpreter executes every `count` instructions", which "only happens while Lua is executing a Lua function". Luau calls an interrupt at function calls and loop iterations. | All allocation goes through a host function, so the host can refuse. Luau has no limit by default. | The host's hook can read a clock. | A long-running C function is not interrupted. [lua54], [luau-sandbox] |
| QuickJS | An interrupt handler "regularly called by the engine". | `JS_SetMemoryLimit`, "a global memory allocation limit". | The handler reads a clock. | Possible because the engine owns its allocator. **(inference)** [quickjs-doc], [quickjs-emscripten] |
| Wasmtime | Fuel: "completely deterministic", with more overhead. | Not on the page I read. | Epochs: about 10% overhead, "non-deterministic". | Both exist because neither does the other's job. [wasmtime-interrupt] |
| JSONata | A `stack` limit that is really a count of evaluation passes, and a `sequence` length limit. | The sequence limit only. | A `timeout` in milliseconds. | The timeout "cannot protect against" a regular expression, which runs outside the evaluator. A port's depth counter scaled with input size. [jsonata-guardrails], [kestra-102] |
| Ethereum | Gas: a cost per operation fixed by the protocol. Changing one "requires a hardfork". | Charged through gas. | — | Underpriced operations enabled denial of service and were repriced in 2016. A later repricing warned that contracts assuming fixed costs "may cease to function". [eip-150], [eip-1884] |
| Go itself | — | `SetMemoryLimit` is a soft limit on "the total amount of memory used by the Go runtime", not on a part of a program. | — | A Go-hosted interpreter has to count for itself. **(inference)** [go-memlimit] |

### 4.2 The closest prior art, and what it teaches

**Canonical's Starlark fork is the closest design to L2 to L4.** It is in Go, it
charges declared amounts before allocating, it counts cumulative allocation and
not live memory, and it charges abstract steps for work. Tengo also counts
cumulatively. **MiniJinja 3.1.0 is the second.** It has one counter for work and
memory, it only goes down, it is charged before a value of known size is
created, and it exists in a Rust and a Go implementation of the same language.
The lessons, each an **(inference)** from the sources in the table:

1. **The design works in Go, and its weak point is coverage.** The fork's own
   document calls safety a best-effort contract and warns that something has
   been missed. v3 has an easier job: every value is created by the evaluator or
   a small library, and values are immutable. Even so, make "declares its
   charges" a condition of registering a library function, as the fork's safety
   flags do.
2. **Test the accounting, not only the results.** The fork's harness runs each
   builtin with input scaled by N and fails if steps or allocation per N leave a
   declared range. v3's suite should do the same in every implementation: for
   each library function, real allocation per charged unit stays under a fixed
   bound.
3. **Abstract units avoid the fork's estimation problem and create a different
   one.** It estimates bytes by reflection and undercounts maps. Units per slot
   and per run of characters (L3) are exact by definition. The host then needs a
   stated bound on bytes per unit for each implementation, or the budget cannot
   be turned into a memory figure.
4. **Charge library work inside the library.** Lua's hook, Luau's interrupt,
   Starlark's `Cancel` and JSONata's timeout all stop at the edge of native
   code. L2's rule, that work not proportional to steps is a defect in the cost
   table, is the right one. Regular expressions are the hard case everywhere,
   which is a second argument for T4's linear-time subset.
5. **Deterministic counts and a deadline are both needed.** Wasmtime ships both.
   L1 does too.
6. **Fixed costs will be wrong somewhere, and changing them breaks stored
   programs.** Starlark avoids the problem by promising nothing. Ethereum fixed
   costs and had to reprice more than once, at the risk of breaking programs
   tuned to the old numbers. Kubernetes reviewers ask whether each cost change
   needs a version. L6's "every version" should become "every implementation of
   one cost-table version", with the version pinned by the host like the
   function set.
7. **Accounting bugs are ordinary bugs.** A depth counter that scaled with input
   size, and a cost estimate that ignored a length bound, both reached users
   ([kestra-102], [k8s-134029]). Counts asserted in the conformance suite are
   the defense.
8. **No project I found promises the same counts in two implementations.**
   MiniJinja charges in Rust and in Go and says the exact costs "can change
   between releases". starlark-rust chose a coarse unit, a call or a loop
   backedge, because it is stable. L6 asks for more than either: one count, in
   three implementations, asserted by the suite. That is new, so the cost table
   should stay as coarse as L2 allows.
9. **A cap on each value is not a budget.** Rhai caps one string or one array
   and has to add up nested contents to make the cap mean anything, and a
   concatenation can still reach almost twice the cap. L3's running total has
   neither problem.
10. **A limit checked afterwards overshoots.** starlark-rust and Regorus compare
    usage with the limit from time to time, and both say a single operation can
    pass it. L3 charges before the memory is taken, and MiniJinja shows that
    rule working in Rust.
11. **Rust can measure what Go has to estimate.** Regorus reads requested bytes
    from an allocator shim. The harness of lesson 2, in Rust, can check real
    bytes per charged unit exactly, and so give the bound that lesson 3 asks
    for.

For hosted use I found library guidance, not product documentation. JSONata's
documentation tells hosts that take expressions from users to set both its depth
and time guardrails ([jsonata-guardrails]), and gojq tells library users to "use
`RunWithContext` when you want to limit the execution time" ([gojq]).

## 5. Expressiveness (C2 to C4)

**Languages that started austere and had to grow.**

- CEL added local binding, two-variable comprehensions and optional access as
  extensions ([cel-ext], [cel-go-pkg]). JMESPath added `let` ([jmespath-let]).
  GitHub Actions added `case` ([gha-expr]). expr-lang has `let`, `??`, a pipe
  and a multi-line `if`, and requests for variables are among its most upvoted
  and most discussed issues ([exprlang-lang], [exprlang-issues-top],
  [exprlang-issues-comments]).
- Terraform added `try` for "complex data structures whose shape is not
  well-known", with a warning that overuse makes configuration "hard to
  understand and maintain" ([tf-try]).
- Excel added `IFS` because it "can take the place of multiple nested IF
  statements, and is much easier to read", and `LET` to assign "names to
  calculation results", for performance and for "easy reading and composition"
  ([excel-ifs], [excel-let]).

The additions are the same each time: names for intermediate values, a multi-way
conditional, handling of missing data, and richer iteration. Each language that
bolted them on got a second syntax for them. E1 to E3 put them in at the start.

**A multi-way conditional expression (E1).**

- C#'s `switch` expression is the nearest model: arms of the form `pattern =>
  expression`, `_` as the default, optional `when` guards, a compiler warning
  when the arms do not cover the input, and a run-time exception if none matches
  ([csharp-switch]).
- JavaScript has none. The pattern matching proposal has been at stage 1 since
  May 2018, and its motivation is that `switch` "may not appear in expression
  position" ([tc39-match]). Least surprise therefore cannot mean "what
  JavaScript does" here. Arms written with `=>` at least reuse a shape the
  author already knows. **(inference)**
- Excel's `IFS` returns `#N/A` when no condition holds, and a default is written
  as a final `TRUE` ([excel-ifs]). A `match` with no default and no proof of
  coverage should be a check-time problem, not a run-time null. **(inference)**

**Block expressions with local bindings (E2).**

- JSONata has them: a parenthesized block of expressions separated by
  semicolons, with `$x := value` bindings scoped to the block, whose value is
  its last expression ([jsonata-programming]). expr-lang has `let x = 42; x * 2`
  ([exprlang-lang]). Both support the shape E2 proposes.
- CEL's `cel.bind` carries the note "Local bindings are not guaranteed to be
  evaluated before use" ([cel-ext]). Under L6 the specification must say when a
  binding is evaluated, because the step count depends on it. Eager, in order,
  is the unsurprising answer. **(inference)**

**Destructuring (E3).**

- Kotlin allows it in lambda parameters, `map.mapValues { (key, value) -> ...
  }`, and its documentation has to spell out the difference between two
  parameters and one destructured pair ([kotlin-destructuring]). Risor v2 hit
  the same confusion without destructuring: a two-parameter callback received
  `(index, value)` (the proposal, line 336).
- Python removed tuple parameters. The reasons were tooling, error messages that
  "in no way" say which tuple failed to unpack, and use in only 0.18% of
  functions. It also noted that lambdas used them precisely because a lambda is
  a single expression ([pep-3113]).
- The lesson for E3: destructure records by field name, not by position, so the
  checker can name the missing field, and keep a destructured parameter visibly
  different from two parameters. **(inference)**

**Recursion (E6).** JSONata allows a function to call itself, and so it needs a
depth guardrail and a timeout ([jsonata-programming], [jsonata-guardrails]). CEL
has no user functions, and Starlark makes recursion an error by default
([cel-spec], [starlark-spec]). The prior art supports E6.

## 6. Implementation techniques (a note on the Go implementation)

**Closures against bytecode.** The published Go evidence points one way.

- Vitess's SQL expression engine moved from an AST interpreter to a slice of Go
  closures. The post reports a geometric mean of 56.30 ns falling to 28.94 ns,
  zero allocations in most benchmarks, and dispatch overhead under 20%
  ([vitess-vm]).
- starlark-go's implementation notes say bytecode could not beat tree-walking,
  because "the Go compiler does not generate a 'computed goto' for a switch
  statement" ([starlark-impl]). The notes are out of date: the repository now
  has an `internal/compile` package ([starlark-compile]).
- GoAWK moved from tree-walking to bytecode and gained 18% on microbenchmarks.
  Its author was "not entirely sure it was worth the additional 2500 lines of
  code" ([goawk-vm]). Go 1.19 added jump tables for large switches, worth about
  20% ([go119]).
- Our spike agrees: closures are 2.8x to 4.3x faster than the VM's floor on
  compute-heavy cases (`risor:experiments/v3/README.md:77-84`).

**Type-specialized closures.** Vitess's gain comes from static types as much as
from closures: the planner knows column types, so the compiler emits
instructions specialized to their operand types ([vitess-vm]). v3 has checker
types at compile time, so `a > b` over two known numbers can compile to a
closure that returns a Go `bool`, not an `any`. **(inference)** Vitess keeps its
old interpreter as a fallback, because SQL arithmetic can change type by value.
v3 needs none: with one kind of number and out-of-range results as errors (N1,
N2), a result type never depends on a value. **(inference)** A read from
`map[string]any` still needs a kind switch, and the spike did not measure struct
reads (`risor:experiments/v3/README.md:119-120`).

**Avoiding interface boxing.** The spike's node type is `func(fr *frame) any`
(`risor:experiments/v3/spike/spike.go:27`), so a number result becomes an
interface value, which usually allocates. **(inference)** Two remedies are
known: typed node families chosen from checker types, and a tagged value struct
such as `log/slog.Value`, which "can represent most small values without an
allocation" ([slog-value]). Typed node families fit gradual typing best: where
the checker says `any`, compile the dynamic node. **(inference)**

**What published benchmarks show.** The one cross-library table I found is the
expr-lang author's: expr-lang 70 ns, cel-go 91 ns, goja 146 ns and Starlark
2,653 ns for one expression ([expr-bench]). Our spike measured 62 to 113 ns on
short expressions (`risor:experiments/v3/README.md:64-73`). The machines and
expressions differ, and the spike charged no allocation units, so this shows
only that v3 can be in the same class as the two leaders. **(inference)**

## 7. Positioning

**The niche.** Risor v3 is a small, specified expression language for
JSON-shaped data: checked when an expression is saved, bounded when it runs, and
the same in every implementation. CEL is the incumbent among specified
languages, and it is built around protobuf, 64-bit integers and macros.
expr-lang is the incumbent in Go and exists only there. Neither types the data
that a workflow engine, a rules table or an automation product actually holds,
which is JSON described by a schema: CEL calls it `dyn`, and expr-lang stops
checking at the first nested map. v3 takes a structural type for that data from
the host, reads the host's values where they are, gives authors the
JavaScript-shaped syntax they already write, and runs the same way in a Go
service, in a Rust service and in the TypeScript editor in front of them. The
owner's workflow engine is the first consumer and already makes the central
promise: a definition that loads does not fail on an expression at run time,
outside a short documented list (`workflow:llms.txt:337-340`).

**Why a developer would choose it over CEL or expr-lang**

1. **It type-checks JSON-shaped data.** Closed and open objects, optional
   fields, literal sets and unions, with null narrowing and misspelling hints,
   and a checker tested to agree with the evaluator
   (`expr:docs/reference/spec.md:767-770`). CEL and expr-lang treat this data as
   dynamic ([k8s-cel], [exprlang-env]).
2. **One language in the service and in the browser.** A specification, a
   conformance suite of data files, and native implementations in Go, Rust and
   TypeScript. An editor can check an expression with the same rules the server
   will apply. expr-lang is a Go library, and CEL's TypeScript implementation
   pays for 64-bit integers with `bigint` ([cel-es]). Cedar and MiniJinja reach
   JavaScript only through WebAssembly ([cedar-self], [minijinja-js]).
3. **Expressive, in a syntax authors know.** Arrow functions, method calls,
   `?.`, `??`, `in`, and the forms austere languages end up adding: `if` and
   `match` as expressions, a named value inside an expression (`const`), and
   destructuring (section 5).
4. **Complete, deterministic limits.** Steps and allocation units defined by the
   specification and charged by every library function, plus a deadline. cel-go
   has runtime cost, which I did not find in CEL's language definition.
   **(inference)** expr-lang has a memory budget, and I saw no step count in its
   VM source ([exprlang-vm]).
5. **The host's values and the host's control.** Values are read in place with
   no conversion, the core has no third-party dependencies, and the names an
   expression reads, its calls and a template's segments are public API (the
   proposal, lines 70-79). In Rust, "no conversion" holds for a host that
   implements or derives the binding's trait. A host that goes through serde
   gets a copy (section 3.4).

**What v3 should not claim**

- **A sandbox for hostile code.** The limits are complete for the language and
  its library, not for host functions (L8).
- **Portability before it is shown.** "The same in every implementation" is a
  claim about implementations that pass the suite. Until all three do, it is a
  goal (D27). CEL's ecosystem is years ahead here.
- **Identical results for entries marked implementation-defined** (T3), such as
  math functions.
- **More speed than expr-lang or cel-go.** Section 6 shows parity at best,
  before allocation accounting is added.

## 8. Recommendations to the other five aspects

Each list is in line with `00-overview.md`. Where a ruling overrules what this
document said before, the item says so.

### Language surface (`02`)

- **Keep arrow functions as the only callback form.** No macros, no `#`, no `it`
  ([cel-ext], [jmespath-let]).
- **Make every JSON key reachable.** Accept keywords after `.` and `?.`, and use
  `x["my-key"]` for the rest. Add no escape scheme ([k8s-cel], [argo-vars]).
- **E1:** model `match` on C#'s `switch` expression, with `=>` arms and a
  default arm. A `match` that is not proved exhaustive and has no default is a
  check-time problem ([csharp-switch], [excel-ifs]).
- **E2:** a block whose value is its last expression has precedent in JSONata
  and expr-lang. Specify that bindings are evaluated eagerly and in order
  ([jsonata-programming], [exprlang-lang], [cel-ext]).
- **E3:** destructure by field name. Keep one destructured parameter distinct
  from two parameters ([kotlin-destructuring], [pep-3113]).
- **Keep `&&` and `||` strictly left to right**, and say so in the
  specification, along with "the first error wins" ([cel-spec]).
- **N4 stands on good evidence:** Python moved to true division for the reason
  N4 gives ([pep-238]).
- **B1: the owner chose truthiness, against this document's advice.** The
  evidence for strict booleans was that HCL requires them and that truthiness
  is behind a well-known GitHub Actions complaint ([hcl-spec], [gha-25645]).
  That complaint is about one case, and it survives under B5: the string
  `"false"` is not empty, so it is truthy. What limits the harm in v3 is the
  checker, which knows the value is a string, and B8: a host that takes a
  condition from text-like data checks that the program's type is `bool`.
  **(inference)**
- **No operator converts between string, number and bool, and `==` across kinds
  is false** ([gha-expr-313], [hcl-spec], [jq-manual]).
- **A dot reads one field of one value.** No implicit mapping, projections or
  singleton flattening ([jsonata-predicate], [jmespath-spec]).
- **Check numeric literals for range on their text** ([jsonnet-1217]).
- **Specify templates in the language.** The host chooses delimiters, and a
  template that is exactly one interpolation yields the value ([hcl-spec],
  `workflow:llms.txt:353-356`).

### Type system (`03`)

- **Start from expr's model**: any, null (expr's nil), bool, number, string, list, object
  (closed, or open with an element type), optional, literal sets and unions
  (`expr:docs/reference/spec.md:734-741`). cty's object and map split and the
  open cel-go request for union types for JSON both support it ([cty-types],
  [cel-203]).
- **Numbers. Overruled:** this document recommended one static `number`. N5
  keeps `int` and `float` in the checker, with `int` fitting `float`. That meets
  the concern behind my recommendation, which was CEL's refusal to compare a
  JSON number with an integer literal ([cel-203]). Make sure no rule ever asks
  an author for a cast between the two.
- **Join branches into a union. Never fail to unify** ([tf-29809]).
- **`any` turns checking off, and the host can ask where.** Offer the equivalent
  of expr-lang's `WarnOnAny` and of result assertions such as `AsBool`
  ([exprlang-config]). Without it, v3 repeats CEL's `object.namex` ([k8s-cel]).
- **Checking must not change results** ([cel-spec]), and the checker must agree
  with the evaluator. Under P4 the agreement tests are suite data, so every
  implementation inherits them.
- **Specify when an error is reported**, at check time or at run time, for every
  error class. JMESPath left it open ([jmespath-test]).
- **Do not build static cost estimation** ([k8s-134029], [kubecon-cost]). The
  runtime budgets of L1 are enough.

### Runtime (`04`)

- **Charging. Overruled:** this document said "one step per element, no cost per
  function". S8 rules for charging by size, with a small table of rates by kind
  of work. The prior art supports the ruling: every limit that ignored work
  inside library code failed there (section 4.2, lesson 4).
- **L3 has a working precedent in Go.** Copy its discipline: no library function
  is registered without declared charges, and a harness checks real allocation
  against charged units with scaled inputs ([canonical-safety]).
- **L6: version the cost table.** I found no precedent for the same counts in
  "every version", and several for one versioned cost table that a host pins
  ([eip-1884], [k8s-138227], [starlark-go-pkg]).
- **State a bound on bytes per allocation unit** for each implementation, so a
  host can turn a budget into a memory figure. **(inference)**
- **Bound the depth of every recursive walk over host values**: equality,
  flatten, rendering and encoding. Host data can be cyclic
  ([exprlang-ghsa-recursion]).
- **Cap source length or node count at compile time** ([exprlang-ghsa-memory]).
  A cap on length is not enough by itself. cel-rust and Cedar each had an input
  of under 300 bytes that made the parser ask for gigabytes, one in a generated
  parser's prediction and one on an error path ([cel-rust-ghsa-memory],
  [cedar-ghsa-has]). Bound what the parser may allocate per byte of source, and
  put pathological inputs in the suite. **(inference)**
- **Keep the compiled program immutable and safe for concurrent runs**
  ([cel-go-readme]). The spike works this way
  (`risor:experiments/v3/spike/spike.go:29-31`).
- **L5: poll the deadline on the step counter.** gopher-lua's example ran about
  40% slower once a context was checked, and Wasmtime measures its wall-clock
  mechanism at about 10% ([gopher-lua], [wasmtime-interrupt]).
- **Go note:** compile to closures and specialize them by checker type (section
  6).
- **Rust note (R5, R6):** make every depth limit a count, never a probe of the
  remaining stack, and give the suite cases at each limit. Make every walk over
  a value iterative or counted, and release values without recursion, or from
  one region freed when the run ends ([cedar-self], [minijinja-907],
  [serde-json-depth]). Run the suite in a debug build and in the WebAssembly
  build, where the stack runs out soonest ([rhai-expr-depth], [cedar-self]).
- **Rust note:** closures are a reasonable first choice and not a proven best
  one. Measure them against a compact tree before the typed layer is designed
  ([wirefilter-blog], [rhai-bench]).
- **Rust note (N2):** do index and count arithmetic in 64 bits with checked
  operations, so that a debug build, a release build and a 32-bit WebAssembly
  build give one answer ([rhai-checked], [minijinja-changelog]).

### Host API (`05`)

- **P5 matches the prior art.** CEL declares variables with types, and the
  workflow engine already derives types from JSON Schema ([cel-go-pkg],
  `workflow:llms.txt:309-311`). Keep reflection over Go values as a Go
  convenience: expr-lang is built around it ([exprlang-env]).
- **P7:** the TypeScript binding should take plain objects, arrays and numbers.
  CEL's takes `bigint`, wrapper classes and its own map type, which is the
  conversion v3 exists to avoid ([cel-es]).
- **Say what the binding does with a host integer outside the safe range** (N2):
  an error naming the field, never rounding. Offer a documented way to pass it
  as a string ([protojson]). Nanosecond timestamps and 64-bit identifiers in Go
  structs will hit this. **(inference)**
- **Go note:** decide struct field naming once, follow pointers, and treat a
  typed nil as nil. These are among the most discussed and most upvoted
  expr-lang and cel-go binding issues ([exprlang-issues-top],
  [exprlang-issues-comments], [cel-issues-comments]).
- **Register functions with a type, a declared charge, and no reflection on the
  call path where a typed adapter exists** (S4) ([exprlang-300],
  [canonical-safety]). Pass the run's cancellation signal to functions that ask
  for it, which is the most upvoted cel-go request ([cel-issues-top]).
- **Return every problem with a position and a code** (P6, T5) ([cel-go-pkg]).
- **Make it easy to remove or subset the standard functions**
  ([cel-issues-top]).
- **Let a host pin the function-set version and the cost-table version**
  ([k8s-cel]).
- **Publish the analysis API**: identifiers read, calls with literal arguments,
  and template segments (the proposal, lines 70-79).
- **Rust note (R2):** give a Rust host three ways in, and say which of them
  copy. First, a trait for reading in place: a member by name, the list of
  names, a kind. Ship a derive macro for it and implementations for the standard
  collections and `serde_json::Value` ([minijinja-object], [rhai-custom-types]).
  Second, values borrowed for the length of a run, with a lifetime on the run's
  inputs ([cel-rust-context], [wirefilter-ctx]). Third, conversion through
  serde, documented as a copy. **(inference)**
- **Rust note (S11):** an opaque value needs an identity test. In Rust that is
  pointer equality on a shared pointer, or a method on the trait, as in
  cel-rust's `equals` ([cel-rust-value]). **(inference)**
- **Fallback build (R8):** data goes in as JSON text and results come back as
  JSON text, which is what Cedar's interface does, and what Regorus's Go binding
  does for input ([cedar-self], [regorus-go]). N2 then applies to the JSON: an
  integer outside the safe range is an error, not a rounded double. Catch a
  panic at the boundary and report it. A WebAssembly module that has panicked is
  not safe to call again ([minijinja-js]). **(inference)**

### Standard library (`06`)

- **Pure and deterministic by default.** No clock, random numbers or local time
  zone in the default set ([starlark-spec]). expr-lang has `now()` and defaults
  to `time.Local`, and cel-go needed a `DefaultUTCTimeZone` option
  ([exprlang-config], [cel-go-pkg]).
- **T3: prefer functions that can be exact.** The CEL string extension that
  Kubernetes enables offers only ASCII case functions ([k8s-cel]), which any
  implementation can get exactly right. **(inference)** Mark math functions
  implementation-defined ([go-math], [mdn-math], [rust-f64]). If case mapping
  goes wider than ASCII, the specification's table (D25) must settle the three
  cases where Go differs from Rust and JavaScript today: a character that
  becomes several, the final sigma, and the Unicode version (section 3.3).
- **T4: either specify an RE2-style subset or leave regular expressions out of
  3.0.** If they are in, the TypeScript implementation ships a matcher, as CEL's
  does ([cel-es-re2]). Do not rely on the host engine ([jsonata-java], [gojq],
  [v8-linear-regex]). Go's and Rust's engines are both linear, so they can run
  the subset once it defines `\d`, `\w`, `\s` and `\b` by their members, and
  once "find every match" has a cost that covers its quadratic worst case
  ([rust-regex], [go-regexp-syntax]).
- **N7: adopt the ECMAScript number-to-text algorithm** and put test vectors in
  the suite ([rfc8785]). The Rust implementation can start from `ryu-js`
  ([ryu-js]). Specify the text-to-number grammar as well: the three host parsers
  accept three different sets of strings (section 3.3).
- **E7:** GitHub Actions uses `startsWith`, `endsWith` and `contains`, and CEL's
  string extension uses `indexOf`, `lastIndexOf`, `charAt` and `substring`
  ([gha-expr], [k8s-cel]). Neither is JavaScript. That is weak evidence for
  JavaScript-style names where one exists. **(inference)**
- **E8: sequence builders can return.** expr-lang charges its range operator
  against a memory budget, which is the same idea ([exprlang-vm]).
- **Predicate forms that stop early**: the equivalents of `any`, `all`, `find`
  and `count`, and a sort by key ([cel-ext]).
- **A conversion that cannot convert is an error.** No zero on failure
  ([argo-vars], [gha-expr-313]).
- **Every function has a type signature, declared step and allocation charges,
  and, if it recurses, a depth bound** ([exprlang-ghsa-recursion],
  [canonical-safety]).
- **Keep it small**, and let hosts add the rest. Every entry is work in every
  implementation, and missing builtins are what "mostly compliant" means in
  practice ([regorus]).

## 9. Open questions for the owner

1. **In what order are the three built, and what is the fallback build?** The
   set is settled: Go, Rust and TypeScript, each native (R1). Two things are
   not. *Order.* In every project of section 3.1 the later implementations lag,
   and the lag sits in the checker and the library ([cedar-go], [regorus],
   [jsonata-rs]). *Recommendation:* Go first, then a parser and a reference
   evaluator in Rust and in TypeScript before any checker is finished. Rust
   should not be last, because it is the one that tests the embedding model, the
   depth rules and memory without a collector (section 3.4). Hold 3.0 until all
   three pass the whole suite (D27). *Fallback.* *Recommendation:* build it from
   the Rust implementation as WebAssembly first, with JSON text in and JSON text
   out, and add a C interface when a host asks for one. Treat it as a fourth
   target: it has 32-bit sizes, no stack probe, and a module that cannot be
   reused after a panic ([cedar-self], [minijinja-changelog], [minijinja-js]).
   Publish it only when it passes the suite itself. Regorus has bindings for
   eight languages and publishes none, "To avoid operational overhead"
   ([regorus]).
2. **N2 is confirmed by the owner. This records what it costs.** The prior art supports passing large
   integers as strings ([protojson], [rfc8785]). I found no precedent for making
   every out-of-range integer result an error, and Lua added 64-bit integers to
   a language that had done without them ([lua53]). The alternative has a
   visible price too: `bigint` in every JavaScript host ([cel-es]), or two
   platforms that disagree silently ([dart-numbers]). *What is left:* have `05`
   make the failure for 64-bit identifiers and nanosecond
   timestamps loud and well explained.
3. **Should L6 promise the same counts in every version?** *Recommendation:* no.
   Promise the same counts in every implementation of one cost-table version,
   and let the host pin the version (section 4.2, lesson 6).
4. **Regular expressions are out of 3.0 (D9, settled by the owner).** This
   document argued for them as an optional group with a specified linear-time
   subset, since CEL shows the cost in TypeScript is one ported matcher
   ([cel-es-re2]). The risk the owner has accepted is that leaving them out
   sends hosts to their own engines, which is how the divergences in section
   3.3 began. *What is left:* reserve the namespace, and say in the
   documentation that a host function wrapping the platform's engine is not
   portable.
5. **v3 is the successor to expr (D1, settled by the owner).** With C1 it is
   the only one of the two that can run in a browser. expr has few users (C7),
   so a converter from its call forms, such as `filter(xs, it.ok)`, is a small
   tool for the workflow engine's own data and not a product.
6. **A public conformance report?** *Recommendation:* yes, generated from the
   suite for each implementation and version, as JSON Schema's ecosystem has
   ([bowtie]). With three implementations it is also the working tool for D27.
7. **Is every depth limit a number in the specification, value nesting
   included?** R5 and D31 say yes for source and calls. The Rust evidence adds
   two things. A value can be nested deeper than any source, so the walks over
   values need the same treatment ([minijinja-907]). And a number that is safe
   in a release build may not be safe in a debug build or in WebAssembly
   ([rhai-expr-depth], [cedar-self]). *Recommendation:* yes to all three counts.
   Choose the numbers after the Rust reference evaluator has run the limit cases
   in its least favorable build, not before.
8. **What does "read in place" promise a Rust host?** In Go and TypeScript the
   host passes a value and the engine looks inside it. In Rust the host must
   implement or derive a trait first, and the path with no host code is serde,
   which copies (section 3.4). R2 already rules for the trait. What the prior
   art adds is the price of lending values for the length of a run: a lifetime
   on the public types, as in cel-rust's `Context<'p, 'v>` and wirefilter's
   `ExecutionContext<'e>` ([cel-rust-context], [wirefilter-blog]). MiniJinja
   avoids the lifetime by owning every value it is given ([minijinja-value]).
   *Recommendation:* lend, and accept the lifetime, because it is the only form
   in which a host keeps its data and nothing is copied. Also accept shared
   owned values, for a host that keeps an environment across runs. Say in the
   documentation that the serde path is a copy. **(inference)**
9. **Differential testing between the three implementations?** A hand-written
   suite pins what its authors thought of. Cedar's generated inputs found
   disagreements between its model and its code ([cedar-drt-blog]). cedar-go
   runs the corpus those inputs produced, and its decoder still accepted a
   malformed record ([cedar-go-ghsa-entity]), so a corpus is only as wide as the
   generator behind it. **(inference)** *Recommendation:* yes, from the day two
   implementations can run a program. Generate programs, data and host input,
   compare values, error classes and step and unit counts, and turn every
   disagreement into a suite case.
10. **Does the conformance suite cover the binding?** The Go ports in section
    3.1 diverged in how they decode input and which arguments they accept, more
    than in how they evaluate. **(inference)** P4 defines suite cases as source,
    types and JSON input. *Recommendation:* add cases for what each binding must
    reject: an integer outside the safe range, a string with an unpaired
    surrogate, a value nested past the limit, and malformed JSON for the
    fallback build.

## Sources

[cel-spec]: https://github.com/google/cel-spec/blob/master/doc/langdef.md
[cel-overview]: https://cel.dev/overview/cel-overview
[cel-tests]: https://github.com/cel-expr/cel-spec/blob/master/tests/README.md
[cel-go-readme]: https://github.com/google/cel-go/blob/master/README.md
[cel-go-pkg]: https://pkg.go.dev/github.com/google/cel-go/cel
[cel-go-mod]: https://github.com/google/cel-go/blob/master/go.mod
[cel-ext]: https://github.com/google/cel-go/blob/master/ext/README.md
[cel-es]: https://github.com/bufbuild/cel-es/tree/main/packages/cel
[cel-es-re2]: https://github.com/bufbuild/cel-es/tree/main/packages/re2
[cel-js]: https://github.com/marcbachmann/cel-js
[cel-python]: https://github.com/cloud-custodian/cel-python
[cel-rust]: https://github.com/cel-rust/cel-rust
[cel-rust-context]: https://github.com/cel-rust/cel-rust/blob/master/cel/src/context.rs
[cel-rust-value]: https://github.com/cel-rust/cel-rust/blob/master/cel/src/common/value.rs
[cel-rust-objects]: https://github.com/cel-rust/cel-rust/blob/master/cel/src/objects.rs
[cel-rust-parser]: https://github.com/cel-rust/cel-rust/blob/master/cel/src/parser/parser.rs
[cel-rust-conformance]: https://github.com/cel-rust/cel-rust/blob/master/conformance/src/bin/ignored.txt
[cel-rust-212]: https://github.com/cel-rust/cel-rust/issues/212
[cel-rust-ghsa-memory]: https://github.com/cel-rust/cel-rust/security/advisories/GHSA-grff-fjh9-2rc7
[cel-203]: https://github.com/cel-expr/cel-go/issues/203
[cel-issues-top]: https://github.com/cel-expr/cel-go/issues?q=is%3Aissue+sort%3Areactions-%2B1-desc
[cel-issues-comments]: https://github.com/cel-expr/cel-go/issues?q=is%3Aissue+sort%3Acomments-desc
[k8s-cel]: https://kubernetes.io/docs/reference/using-api/cel/
[k8s-crd-blog]: https://kubernetes.io/blog/2022/09/23/crd-validation-rules-beta/
[k8s-134029]: https://github.com/kubernetes/kubernetes/issues/134029
[k8s-138227]: https://github.com/kubernetes/kubernetes/pull/138227
[kubecon-cost]: https://kccncna2024.sched.com/event/1i7nv/exceeded-your-validation-cost-budget-now-what-joel-speed-red-hat
[exprlang-lang]: https://expr-lang.org/docs/language-definition
[exprlang-env]: https://expr-lang.org/docs/environment
[exprlang-config]: https://expr-lang.org/docs/configuration
[exprlang-func]: https://expr-lang.org/docs/functions
[exprlang-vm]: https://github.com/expr-lang/expr/blob/master/vm/vm.go
[exprlang-mod]: https://github.com/expr-lang/expr/blob/master/go.mod
[exprlang-ghsa-memory]: https://github.com/expr-lang/expr/security/advisories/GHSA-93mq-9ffx-83m2
[exprlang-ghsa-recursion]: https://github.com/expr-lang/expr/security/advisories/GHSA-cfpf-hrx2-8rv6
[exprlang-206]: https://github.com/expr-lang/expr/issues/206
[exprlang-300]: https://github.com/expr-lang/expr/issues/300
[exprlang-434]: https://github.com/expr-lang/expr/issues/434
[exprlang-628]: https://github.com/expr-lang/expr/issues/628
[exprlang-issues-top]: https://github.com/expr-lang/expr/issues?q=is%3Aissue+sort%3Areactions-%2B1-desc
[exprlang-issues-comments]: https://github.com/expr-lang/expr/issues?q=is%3Aissue+sort%3Acomments-desc
[expr-bench]: https://github.com/antonmedv/golang-expression-evaluation-comparison
[starlark-spec]: https://github.com/google/starlark-go/blob/master/doc/spec.md
[starlark-lang-spec]: https://github.com/bazelbuild/starlark/blob/master/spec.md
[starlark-impl]: https://github.com/google/starlark-go/blob/master/doc/impl.md
[starlark-compile]: https://github.com/google/starlark-go/tree/master/internal/compile
[starlark-design]: https://github.com/bazelbuild/starlark/blob/master/design.md
[starlark-go-pkg]: https://pkg.go.dev/go.starlark.net/starlark
[starlight]: https://github.com/starlight-go/starlight
[starlark-rust]: https://github.com/facebook/starlark-rust/blob/main/README.md
[starlark-rust-lib]: https://github.com/facebook/starlark-rust/blob/main/starlark/src/lib.rs
[starlark-rust-eval]: https://github.com/facebook/starlark-rust/blob/main/starlark/src/eval/runtime/evaluator.rs
[starlark-rust-guard]: https://github.com/facebook/starlark-rust/blob/main/starlark/src/values/stack_guard.rs
[starlark-rust-bc]: https://github.com/facebook/starlark-rust/tree/main/starlark/src/eval/bc
[canonical-starlark]: https://github.com/canonical/starlark
[canonical-safety]: https://github.com/canonical/starlark/blob/main/doc/safety.md
[cty-concepts]: https://github.com/zclconf/go-cty/blob/main/docs/concepts.md
[cty-types]: https://github.com/zclconf/go-cty/blob/main/docs/types.md
[cty-convert]: https://github.com/zclconf/go-cty/blob/main/docs/convert.md
[hcl-spec]: https://github.com/hashicorp/hcl/blob/main/hclsyntax/spec.md
[tf-29809]: https://github.com/hashicorp/terraform/issues/29809
[tf-try]: https://developer.hashicorp.com/terraform/language/functions/try
[jsonata-processing]: https://docs.jsonata.org/next/processing
[jsonata-path]: https://docs.jsonata.org/next/path-operators
[jsonata-predicate]: https://docs.jsonata.org/predicate
[jsonata-programming]: https://docs.jsonata.org/programming
[jsonata-string]: https://docs.jsonata.org/string-functions
[jsonata-guardrails]: https://docs.jsonata.org/guardrails
[jsonata-suite]: https://github.com/jsonata-js/jsonata/blob/master/test/test-suite/TESTSUITE.md
[jsonata-java]: https://github.com/dashjoin/jsonata-java
[jsonata-go]: https://github.com/blues/jsonata-go
[jsonata-rs]: https://github.com/Stedi/jsonata-rs
[kestra-102]: https://github.com/kestra-io/plugin-transform/issues/102
[jmespath-spec]: https://jmespath.org/specification.html
[jmespath-let]: https://github.com/jmespath-community/jmespath.spec/blob/main/jep-011a-lexical-scope.md
[jmespath-test]: https://github.com/jmespath/jmespath.test
[jsonschema-suite]: https://github.com/json-schema-org/JSON-Schema-Test-Suite
[jsonschema-regex]: https://json-schema.org/understanding-json-schema/reference/regular_expressions
[bowtie]: https://docs.bowtie.report/en/stable/
[jq-manual]: https://jqlang.org/manual/
[gojq]: https://github.com/itchyny/gojq
[jvns-jq]: https://jvns.ca/til/fx--a-jq-replacement/
[aws-sfn]: https://aws.amazon.com/about-aws/whats-new/2024/11/aws-step-functions-variables-jsonata-transformations
[gha-expr]: https://docs.github.com/en/actions/reference/workflows-and-actions/expressions
[gha-expr-313]: https://docs.github.com/en/enterprise-server@3.13/actions/reference/evaluate-expressions-in-workflows-and-actions
[gha-25645]: https://github.com/orgs/community/discussions/25645
[argo-vars]: https://argo-workflows.readthedocs.io/en/latest/variables/
[opa-lang]: https://www.openpolicyagent.org/docs/policy-language
[opa-wasm]: https://www.openpolicyagent.org/docs/wasm
[regorus]: https://github.com/microsoft/regorus
[regorus-go]: https://github.com/microsoft/regorus/blob/main/bindings/go/pkg/regorus/mod.go
[regorus-memory]: https://github.com/microsoft/regorus/blob/main/docs/limits/memory_budget.md
[regorus-time]: https://github.com/microsoft/regorus/blob/main/docs/limits/execution_time.md
[cedar-self]: https://github.com/cedar-policy/cedar/blob/main/self-assessment.md
[cedar-spec]: https://github.com/cedar-policy/cedar-spec
[cedar-eval]: https://github.com/cedar-policy/cedar/blob/main/cedar-policy-core/src/evaluator.rs
[cedar-changelog]: https://github.com/cedar-policy/cedar/blob/main/cedar-policy/CHANGELOG.md
[cedar-2660]: https://github.com/cedar-policy/cedar/issues/2660
[cedar-ghsa-has]: https://github.com/cedar-policy/cedar/security/advisories/GHSA-m84h-6f26-p3p7
[cedar-drt-blog]: https://www.amazon.science/blog/how-we-built-cedar-with-automated-reasoning-and-differential-testing
[cedar-go]: https://github.com/cedar-policy/cedar-go
[cedar-go-corpus]: https://github.com/cedar-policy/cedar-go/blob/main/.github/workflows/corpus.yml
[cedar-go-ghsa-entity]: https://github.com/cedar-policy/cedar-go/security/advisories/GHSA-vrg5-pj65-m4wr
[cedar-java]: https://github.com/cedar-policy/cedar-java
[cedar-wasm-npm]: https://unpkg.com/@cedar-policy/cedar-wasm@4.14.0/?meta
[minijinja-readme]: https://github.com/mitsuhiko/minijinja/blob/main/README.md
[minijinja-go]: https://github.com/mitsuhiko/minijinja/blob/main/minijinja-go/README.md
[minijinja-js]: https://github.com/mitsuhiko/minijinja/blob/main/minijinja-js/README.md
[minijinja-js-npm]: https://unpkg.com/minijinja-js@3.1.0/?meta
[minijinja-changelog]: https://github.com/mitsuhiko/minijinja/blob/main/CHANGELOG.md
[minijinja-lib]: https://github.com/mitsuhiko/minijinja/blob/main/minijinja/src/lib.rs
[minijinja-env]: https://github.com/mitsuhiko/minijinja/blob/main/minijinja/src/environment.rs
[minijinja-value]: https://github.com/mitsuhiko/minijinja/blob/main/minijinja/src/value/mod.rs
[minijinja-object]: https://github.com/mitsuhiko/minijinja/blob/main/minijinja/src/value/object.rs
[minijinja-907]: https://github.com/mitsuhiko/minijinja/pull/907
[rhai-custom-types]: https://github.com/rhaiscript/book/blob/master/src/rust/custom-types.md
[rhai-serde]: https://github.com/rhaiscript/book/blob/master/src/rust/serde.md
[rhai-ops]: https://github.com/rhaiscript/book/blob/master/src/safety/max-operations.md
[rhai-call-stack]: https://github.com/rhaiscript/book/blob/master/src/safety/max-call-stack.md
[rhai-expr-depth]: https://github.com/rhaiscript/book/blob/master/src/safety/max-stmt-depth.md
[rhai-string-size]: https://github.com/rhaiscript/book/blob/master/src/safety/max-string-size.md
[rhai-array-size]: https://github.com/rhaiscript/book/blob/master/src/safety/max-array-size.md
[rhai-memory]: https://github.com/rhaiscript/book/blob/master/src/safety/memory.md
[rhai-checked]: https://github.com/rhaiscript/book/blob/master/src/safety/checked.md
[rhai-wasm]: https://github.com/rhaiscript/book/blob/master/src/start/builds/wasm.md
[rhai-bench]: https://github.com/rhaiscript/book/blob/master/src/about/benchmarks.md
[wirefilter]: https://github.com/cloudflare/wirefilter/blob/master/engine/README.md
[wirefilter-ctx]: https://github.com/cloudflare/wirefilter/blob/master/engine/src/execution_context.rs
[wirefilter-blog]: https://blog.cloudflare.com/building-fast-interpreters-in-rust/
[serde]: https://serde.rs/
[serde-json-depth]: https://github.com/serde-rs/json/blob/master/src/de.rs
[rust-stack-overflow]: https://github.com/rust-lang/rust/blob/master/library/std/src/sys/pal/unix/stack_overflow/handler_signal.rs
[rust-thread]: https://github.com/rust-lang/rust/blob/master/library/std/src/thread/mod.rs
[rust-string]: https://github.com/rust-lang/rust/blob/master/library/alloc/src/string.rs
[rust-str]: https://github.com/rust-lang/rust/blob/master/library/alloc/src/str.rs
[rust-char]: https://github.com/rust-lang/rust/blob/master/library/core/src/char/methods.rs
[rust-f64]: https://github.com/rust-lang/rust/blob/master/library/std/src/num/f64.rs
[rust-releases]: https://github.com/rust-lang/rust/blob/master/RELEASES.md
[rust-regex]: https://github.com/rust-lang/regex/blob/master/src/lib.rs
[regex-syntax-nest]: https://github.com/rust-lang/regex/blob/master/regex-syntax/src/parser.rs
[go-regexp-syntax]: https://github.com/golang/go/blob/master/src/regexp/syntax/doc.go
[go-unicode-126]: https://github.com/golang/go/blob/release-branch.go1.26/src/unicode/tables.go
[ryu-js]: https://github.com/boa-dev/ryu-js
[jsonnet-design]: https://jsonnet.org/articles/design.html
[go-jsonnet]: https://github.com/google/go-jsonnet
[jsonnet-021]: https://github.com/google/jsonnet/releases/tag/v0.21.0
[jsonnet-1217]: https://github.com/google/jsonnet/pull/1217
[cue-logic]: https://cuelang.org/docs/concept/the-logic-of-cue/
[dhall-safety]: https://docs.dhall-lang.org/discussions/Safety-guarantees.html
[dhall-integrate]: https://docs.dhall-lang.org/howtos/How-to-integrate-Dhall.html
[gopher-lua]: https://github.com/yuin/gopher-lua/blob/master/README.md
[goja]: https://github.com/dop251/goja/blob/master/README.md
[lua53]: https://www.lua.org/manual/5.3/readme.html
[lua54]: https://www.lua.org/manual/5.4/manual.html
[luau-sandbox]: https://luau.org/sandbox
[quickjs-doc]: https://bellard.org/quickjs/quickjs.html
[quickjs-emscripten]: https://github.com/justjake/quickjs-emscripten
[tengo-interop]: https://github.com/d5/tengo/blob/master/docs/interoperability.md
[wasmtime-interrupt]: https://docs.wasmtime.dev/examples-interrupting-wasm.html
[wasm-bindgen-str]: https://rustwasm.github.io/docs/wasm-bindgen/reference/types/str.html
[esbuild-wasm]: https://esbuild.github.io/getting-started/
[go-wasm]: https://go.dev/wiki/WebAssembly
[go-memlimit]: https://go.googlesource.com/proposal/+/HEAD/design/48409-soft-memory-limit.md
[go-math]: https://pkg.go.dev/math
[go-unicode]: https://pkg.go.dev/unicode
[mdn-math]: https://developer.mozilla.org/en-US/docs/Web/JavaScript/Reference/Global_Objects/Math
[mdn-forin]: https://developer.mozilla.org/en-US/docs/Web/JavaScript/Reference/Statements/for...in
[v8-linear-regex]: https://v8.dev/blog/non-backtracking-regexp
[protojson]: https://protobuf.dev/programming-guides/json/
[rfc8785]: https://www.rfc-editor.org/rfc/rfc8785
[dart-numbers]: https://dart.dev/resources/language/number-representation
[eip-150]: https://eips.ethereum.org/EIPS/eip-150
[eip-1884]: https://eips.ethereum.org/EIPS/eip-1884
[excel-let]: https://support.microsoft.com/en-us/office/let-function-34842dd8-b92b-4d3f-b325-b8b8f9908999
[excel-ifs]: https://support.microsoft.com/en-us/office/ifs-function-36329a26-37b2-467c-972b-4a39bd951d45
[csharp-switch]: https://learn.microsoft.com/en-us/dotnet/csharp/language-reference/operators/switch-expression
[tc39-match]: https://github.com/tc39/proposal-pattern-matching
[kotlin-destructuring]: https://kotlinlang.org/docs/destructuring-declarations.html
[pep-3113]: https://peps.python.org/pep-3113/
[pep-238]: https://peps.python.org/pep-0238/
[vitess-vm]: https://planetscale.com/blog/faster-interpreters-in-go-catching-up-with-cpp
[goawk-vm]: https://benhoyt.com/writings/goawk-compiler-vm/
[slog-value]: https://github.com/golang/go/blob/master/src/log/slog/value.go
[go119]: https://go.dev/doc/go1.19
