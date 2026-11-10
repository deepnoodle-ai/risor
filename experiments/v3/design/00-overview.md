# Risor v3 design: overview and cross-cutting decisions

Status: design proposal. It records the owner's constraints, the decisions that
span more than one of the six aspect documents (`01` to `06`), the decisions
still open for the owner, and the roadmap. The aspect documents have been
revised to agree with it. Where one still disagrees, this file wins. Seven
plans, `07` to `13`, each hold one feature that was designed after the aspect
documents. Each is the authority for its feature, and `02` to `06` carry its
rules.

Third revision. The owner has since said four things, recorded as C1 (now three
named implementations), C6, C7 and B1 (truthiness), and has confirmed the
headline decisions of section 4. Sections 2.3, 2.8 and 2.9 carry what follows
from them.

Fourth revision. The owner settled D28 to D30 and D32 to D34 as recommended. `null` is the one
spelling of the absent value, type text is in TypeScript's spelling, and a
lambda is passed only to a library function. The owner also said that a
language with no statements at all is more than C3 asked for. That was put
as D35 and D36, with section 2.10.

Fifth revision. The owner settled D35 and D36 as recommended: `if`, `else if`
and `else` form an expression, and a binding is written `const`. Loops and
reassignment are deferred, with their words reserved. Section 2.10 has the
rules. D37 and D38 are two details of `if` that were ruled here and not asked.

Sixth revision. A plan for checked conversion of untyped data is added as
`07-checked-conversion.md`, with section 2.11 and D39 to D45. The owner settled
all seven as recommended: `as` and `is` are in 3.0, with host-declared type
names and a conformance function for the host. `02` to `06` are revised to
match. That revision corrected the plan at sixteen points, none of which
changes a decision; `07` section 10 lists them.

Seventh revision. The owner asked for six more areas and accepted a plan for
each, with every decision as the plan recommends: string literal forms (`08`),
tagged unions (`09`), error propagation and formatting (`10`), testing (`11`),
configuration files (`12`) and queries (`13`). Section 2.12 and D46 to D51
record them. The owner decided four points directly: `try` is a block
expression with `catch`, as in v2; config files are data only; imports come in
a minor version after 3.0; and messages come from one catalog. `02` to `06` are
revised to match.

Each decision has a status:

- **Owner**: the owner said it. Do not reopen it.
- **Ruling**: decided here to reconcile the aspect documents. An aspect document
  may argue against a ruling in its own "Open questions" section, but must be
  written as if the ruling holds.
- **Candidate**: a direction the named aspect is asked to design and either
  adopt or reject with reasons.

## 1. The owner's constraints

These came after the first drafts of `01` to `06` and change several of them.

| # | Constraint | Status |
| --- | --- | --- |
| C1 | **The language is not tied to Go.** There are three implementations: Go, Rust and TypeScript. | Owner |
| C2 | **Least surprise**, all else equal. | Owner |
| C3 | **A preference for expressions** over many statements. | Owner |
| C4 | **Expressiveness and the author's experience must be high.** A design that is safe but austere misses the goal. | Owner |
| C5 | **Memory allocation and run duration can be controlled and limited.** | Owner |
| C6 | **Lean toward TypeScript's syntax, all else equal,** without JavaScript's historical baggage. | Owner |
| C7 | **Compatibility with expr and with Risor v1 and v2 carries little weight.** expr has no real outside users and Risor's user base is small. A more robust v3 is worth a break. | Owner |

The eight decisions given to every aspect at the start still hold:
expression-first; Risor's syntax on expr's semantics; host values read in place;
`.` always reads a field; immutable values; static checking as a first-class
feature; a budget that counts all work; no bytecode VM.

## 2. What the constraints change

### 2.1 A specification with several implementations (C1)

| # | Decision | Status |
| --- | --- | --- |
| P1 | v3 is defined by a written specification and a conformance suite. The Go code is one implementation of it. Cross-language portability is a goal; `01` said it was not. | Owner (C1) |
| P2 | Every aspect document separates what is **normative for the language** (any implementation must do it) from what is **a note on the Go implementation**. A reader building the TypeScript implementation should be able to skip the second kind. The same holds for the Rust implementation. | Ruling |
| P3 | No semantic rule may be defined by what Go does. This covers integer width and wrapping, UTF-8 byte lengths and byte order, `reflect`, struct tags, Go's `regexp`, Go's float formatting, and Go's Unicode tables. Each such rule is restated in neutral terms or marked implementation-defined, and the second kind is kept as small as possible. | Ruling |
| P4 | The conformance suite is data, not Go tests: source text, declared types, input values as JSON, and the expected type, value, diagnostic code or error class. Every implementation runs the same files. | Ruling |
| P5 | The portable ways to declare an environment's types are the type text notation of `03` and JSON Schema. Deriving types from Go structs by reflection is a convenience of the Go implementation, as deriving them from TypeScript types would be of that one. | Ruling |
| P6 | Diagnostic codes, error classes and the type text notation are part of the specification, so tools and tests work across implementations. | Ruling |
| P7 | The host API is specified as an abstract embedding model (parse, check, compile, run; environment; variables; limits; diagnostics). `05` gives the Go binding in full and sketches the TypeScript and Rust bindings, to show the model is not Go-shaped. | Ruling |

### 2.2 Numbers (C1, C2)

`02` wanted one kind of number, `03` wanted `int` and `float`, `01` wanted one
static `number`. A TypeScript implementation has IEEE doubles, exact for
integers only to 2^53.

| # | Decision | Status |
| --- | --- | --- |
| N1 | **One kind of number at run time, with the value semantics of an IEEE 754 double.** An author never sees a difference between "an int" and "a float holding a whole number". | Ruling |
| N2 | **Integers are exact in the safe range, ±(2^53 − 1).** An integer result outside that range is an error, not a silent loss of precision. A host integer outside it cannot enter the language as a number; the host passes it as a string. An implementation may store integers as machine integers, as long as no result differs. | Ruling, confirmed by the owner (the alternatives were 64-bit integers, which cost the TypeScript implementation a second number representation, and JavaScript's silent rounding) |
| N3 | **No operator returns NaN or an infinity.** Such a result is an error. | Ruling (both `02` and `06` already say so) |
| N4 | **`/` is true division**: `7 / 2` is `3.5`. Truncation is written with a function. | Ruling, confirmed by the owner (`02` and `03` agree; v2 and expr both truncate) |
| N5 | **The checker has `int` and `float`, and `int` fits `float`,** as in `03`. `int` means "a whole number", not a storage format. It is what an index and an integer parameter require. To the author this reads as one kind of number with places that need a whole one. | Ruling |
| N6 | Out-of-range arithmetic is an error class that can occur at any arithmetic node, like the budget. It is not listed as a residual site on every `+`. | Ruling |
| N7 | Number to text, and text to number, follow one exact algorithm in the specification. The ECMAScript `Number::toString` rule is the candidate, because JSON tools already agree with it. | Candidate for `02` and `06` |

### 2.3 Booleans and null (C2)

| # | Decision | Status |
| --- | --- | --- |
| B1 | **The language has truthiness.** Any value may be a condition. The second revision ruled for strict booleans; the owner reversed it. | Owner |
| B5 | **The falsy values are null, `false`, zero, the empty string, the empty list and the empty map.** Every other value is truthy, including every opaque value. This is the rule of v2 and of expr (`pkg/object/list.go:538`, expr `truthy.go`), of Python and of Go's templates. JavaScript differs on one point: there an empty array and an empty object are truthy. Three reasons to keep "empty is falsy": an author who writes `when: results.items` means "there are items"; one sentence states the whole rule; and the test gives the same answer whether a Go host supplied a nil slice or an empty one, which a rule that separates nil from empty would not (C1). | Ruling, confirmed by the owner (D30). The alternative was JavaScript's set exactly |
| B6 | **`a && b` and `a \|\| b` return an operand, not a bool,** as in JavaScript: `a \|\| b` is `a` when `a` is truthy, else `b`. `!a` is a bool. A condition in `? :`, in a `match` arm, and the result of a callback to `filter`, `find`, `some` and `every` are tested by truthiness. `\|\|` replaces any falsy value and `??` replaces null only; both stay. | Ruling |
| B7 | **The checker types truthiness.** `a \|\| b` has the type of `a` without its null alternative, joined with `b`. `a && b` has the part of `a` that can be falsy, joined with `b`. A true branch narrows null away, as before. A condition whose type can never be falsy, or is always falsy, is a diagnostic: it is the same class of mistake TypeScript reports as "this condition will always return true". `03` defines the two codes, `always-truthy` and `always-falsy`, and makes them errors. Tightening a declared type from `T \| null` to `T` would then break a stored `x && x.f`, and nothing else about tightening a type breaks a program. So the two codes form a check class, `constant-condition`. An environment carries the set of classes its check rejects. This class is in the default set, and a host may take it out, for example to re-check stored programs after a schema changes. When it is out, the two codes are not reported (D32). | Ruling, confirmed by the owner |
| B8 | **A host that needs a real boolean checks the result type.** The workflow engine rejects `when: verify` when `verify` is a list. That stays possible: the host checks that the program's type is `bool`. Truthiness is a rule of the operators, not of what a host accepts. The price is real: the guard most authors will write, `report && report.verdict == "APPROVE"`, has type `bool \| null` and such a host rejects it. So a host has a second choice: it may test the result by truthiness, with the binding's own test, and ask the checker for a condition and not a `bool` (`05` A.3). The workflow engine takes the second choice for `when:` (D33). | Ruling, confirmed by the owner |
| B2 | A missing field is an error with `.` and null with `?.`. An optional field (`f?: T`) is distinct from a field whose value may be null. | Ruling (all documents agree) |
| B3 | **The optional index is spelled `?.[`**, as in JavaScript. `03` wrote `?[`. | Ruling |
| B4 | **`?.` short-circuits the rest of its chain, as in JavaScript, TypeScript, C# and Swift.** `a?.b.c` is null when `a` is null. `02` chose per-access, following v2 and expr. The syntax is JavaScript's, so its meaning should be too (C2). | Ruling; `02` to specify exactly what "the rest of the chain" covers |

### 2.4 Expressiveness (C3, C4)

`02` cut the language hard: the ternary as the only conditional, no `match`, no
destructuring, `let` only at the top of a script. Those cuts were made for the
checker's and the parser's convenience. Under C4 that is not enough reason. `02`
owns these and is asked to redesign them. The other aspects should assume the
directions below unless the revised `02` says otherwise.

| # | Direction | Status |
| --- | --- | --- |
| E1 | **A multi-way conditional expression.** A ternary chain and a map lookup with `??` are poor ways to write "choose by case". Design a `match` expression: literal patterns, a default arm, typed as the join of its arms, and checked for exhaustiveness against literal sets (`03` already has them). Decide whether arms bind names or take guards. | Candidate for `02` |
| E2 | **`const` usable inside an expression**, not only at the top of a script, so that a lambda body or a branch can name an intermediate value. One shape to consider: a block expression that starts with `const`, as in `x => { const y = f(x); y * y }`, which stays distinct from a map literal. If this works, the "script form" is the same construct without braces and the two forms become one. | Candidate for `02` |
| E3 | **Destructuring of records in lambda parameters and `const`**: `entries.map(({key, value}) => ...)`. `06` makes `entries` return records, which makes this the natural way to use them. | Candidate for `02` |
| E4 | **The `**` operator.** `02` removed it because its result type depended on a value. With one kind of number that reason is gone. | Candidate for `02` |
| E5 | **Everything stays an expression.** No statement that exists only for its effect. No assignment. | Ruling |
| E6 | **No recursion and no loops in 3.0**, so every program terminates and its work is bounded by its text and its data. Lambdas are arguments and `const` values, not general values. This is the one place where C5 is chosen over C4. | Ruling, confirmed by the owner |
| E7 | **Names in the library follow least surprise for a JavaScript-shaped language.** `06` chose expr's names (`upper`, `any`, `all`) and gives hints for JavaScript's (`toUpperCase`, `some`, `every`, `includes`). `06` is to set the two columns side by side and choose again with C2 as the test. | Candidate for `06` |
| E9 | **E6's claim has a hole.** `02` allows a lambda as an argument to a `const`-bound lambda, and a parameter may be called. Then `const om = x => x(y => x(y)); om(z => om(w => z(w)))` never returns, although nothing refers to itself. Today such a program ends at the checker's limit on body checks or at the depth limit, so it ends by a limit and not by construction. The fix `02` recommends: a lambda may be passed only to a library function or method, and a parameter cannot be called. The call graph is then fixed by the text. The cost is a user-written helper such as `const twice = (f, x) => f(f(x))`. | Ruling, confirmed by the owner (D34): a lambda is passed only to a library function or method, and a parameter cannot be called |
| E8 | **Sequence builders may return.** `06` dropped `range`, `repeat` and padding because nothing bounded their output. With allocation charged before the work (section 2.5), that reason is gone. | Candidate for `06` |

### 2.5 Limits on work, memory and time (C5)

`04` counted iterations only and listed size as a documented gap, citing the
rule in `AGENTS.md` against new limit machinery. `06` charged by input size
before doing the work. That rule was written for v2, where values live in a host
object model and Go gives no allocator hook. In v3 every value the language
creates is created by the evaluator or by a small library, and values are
immutable, so complete accounting is practical. C5 makes it a goal.

| # | Decision | Status |
| --- | --- | --- |
| L1 | **Three limits, each a first-class part of the host API:** a step budget (work), an allocation budget (memory), and a deadline (time). A depth limit stays as a fourth, for the implementation's own stack. | Owner (C5), shape by ruling |
| L2 | **Steps bound all work the language does.** A step is charged for each operator, field read, index, call and element visited, and for text in proportion to its size. An operation whose work is not proportional to its step count is a defect in the cost table, not a gap to document. `06`'s size-based charging stands; `04`'s "iterations, not bytes" does not. | Ruling |
| L3 | **Allocation is counted in abstract units and charged before the memory is taken.** Every value the evaluator or the library creates has a size in units (for example one per list slot or map entry, and one per fixed number of characters of text). The budget is on the total created during a run, not on live memory, because total creation is deterministic and live memory depends on a collector. An operation whose output size is known only after the work computes a bound first. | Ruling |
| L4 | **Host data read in place is not charged.** A host function charges for its own work and allocation through the call handle (`Charge` and its allocation counterpart). | Ruling |
| L5 | **The deadline is wall-clock time**, checked at a fixed step interval. The specification fixes the interval and leaves the mechanism to each implementation: Go polls a `context.Context`; a synchronous JavaScript run cannot observe a signal changing, so it reads the clock. `Limits` carries the deadline as a field. It is the one limit that is not deterministic. A host function that blocks is not interrupted. | Ruling (mechanism corrected after `05`'s revision) |
| L6 | **Steps and allocation units are defined by the specification, over the program as written.** The same program and data reach a limit at the same point in every implementation of one specification version. The cost table is part of that version, and a host pins the version (`05`), so a stored program keeps its counts until the host chooses to move. An optimization such as constant folding must charge what the unoptimized evaluation would have charged. `04` wanted counts free to change between releases; the cost of fixing them is to be measured, not assumed. | Ruling; `04` to report the cost. Narrowed from "every version" after `01` found no precedent for counts that never change (starlark-go leaves a step's meaning unspecified, and Ethereum had to reprice) |
| L7 | The unit of text size for L2 and L3 must be computable cheaply in Go and in JavaScript and give the same number in both. `04` and `06` are to propose it together (UTF-8 bytes, UTF-16 units and code points each cost O(n) somewhere). | Candidate for `04` and `06` |
| L8 | The limits are complete for what the language and its library do. They do not cover host functions, and they are still not a sandbox for hostile code. | Ruling |

`AGENTS.md` describes v2. Its "Resource Limits" section does not govern v3.

### 2.6 Text, order and the library (C1)

| # | Decision | Status |
| --- | --- | --- |
| T1 | **Strings are sequences of Unicode code points** for length, slicing and ordering. Byte and UTF-16 views are not visible to the author. | Ruling (`02` already says so) |
| T2 | **Maps iterate, render and encode in key order by code point.** JavaScript's default sort is by UTF-16 unit, so an implementation must not use it unchanged. | Ruling |
| T3 | Every library entry is marked **exact** (every implementation gives the same result) or **implementation-defined**. `06` is to keep the second set as small as it can. Known risks: case mapping (Unicode table versions), transcendental math functions, regular expressions. | Ruling |
| T4 | **Regular expressions** need a syntax subset that both Go's engine and a JavaScript implementation can run with the same results, and a guarantee of time linear in the input (L2). JavaScript's built-in engine backtracks, so the TypeScript implementation would have to ship its own matcher for the subset. `06` is to recommend: specify the subset, or leave regular expressions out of the 3.0 specification. | Candidate for `06` |
| T5 | **Diagnostic positions** are specified as line and column counted in code points. A byte or UTF-16 offset is an addition each implementation may give in its own unit. | Ruling; `05` to define |

### 2.7 Smaller conflicts between the drafts

| # | Conflict | Ruling |
| --- | --- | --- |
| S1 | `06` needs namespaces (`math.sqrt`); `05` dropped modules and read `math.sqrt(x)` as a method call on a variable. | A namespace is a compile-time prefix on a function name, resolved by the checker. It is not a value. Declaring a variable with a namespace's name is an error when the environment is built. `05` adds declaring a namespaced function and enabling an optional group. |
| S2 | `05` said a host function may mutate its arguments; `04` shares constant collections between runs. | A host function must treat its arguments as read-only and must not keep them after the call. This is a contract, not an enforced check. |
| S3 | `04` lets a host function receive a function value; `05` rules it out; `06` notes hosts then cannot add higher-order operations. | Not in 3.0: a lambda cannot be passed to a host function. This keeps a closure from outliving its run in every implementation. Revisit after 3.0. |
| S4 | `05` types a host function by its Go signature; `04` needs a call path with no reflection. | Go binding detail. Registration accepts any Go function, chooses a reflection-free adapter for the signatures that have one, falls back to reflection for the rest, and can report which path a function got. |
| S5 | `03` puts types in a `types` package; `05` assumes the root package. | Go binding detail, owned by `05`. The portable form of a type is its text. |
| S6 | `03` checks assignment, `return`, `throw`, destructuring `let` and blocks; `02` removed most of them. | `03` covers exactly the constructs in the revised `02`. |
| S7 | `03` wanted integer overflow to promote to float; `02` wanted an error. | An error (N2, N6). |
| S8 | `06` charged strings by size; `04` did not. `01` advised against a cost per function. | By size (L2). A small table of rates by kind of work, as in `06`, is not a cost per function. |
| S9 | `05` removed the timeout option; the owner wants duration controlled. | `Limits` carries a deadline (L5). |
| S10 | `05` recommends dropping wonton's `assert` from the core module's tests and dropping the language server from 3.0. | Owner's decisions. Carried to the final list of open decisions, not ruled here. |
| S11 | `02` made `==` between two opaque host values a run-time error; `03` objected that this makes `==` able to fail, which spreads into `in`, templates and strict checking. | `==` never fails. An opaque value equals only itself, by an identity test the binding supplies. `02` is edited to match. |
| S12 | `02` and `03` wrote library calls with the first draft's names (`.upper()`). | `06` owns names. The other documents use its names (`.toUpperCase()`). |
| S13 | `03` marked `round` as fallible on a value; `06` says it fails only when the result is out of range. | `round` carries no `!`. Its only failure has class `range`. |
| S14 | `02` let a host choose template delimiters; `05` fixed them at `${{ }}`. | Fixed in 3.0, so the specification has one template grammar. The owner may reopen it. |
| S15 | `02` asked for `math.sqrt` to be implementation-defined; `06` marks it exact. | Exact: IEEE 754 requires a correctly rounded square root. |
| S16 | `04` charged a walk of a map in key order 1 step per entry; `06` charges its three map methods a sort formula. | Both charge the sort formula over the number of entries, plus 1 per entry. Ordering keys is work. |
| S17 | `03`'s method grammar allowed only `string`, `list<T>` and `map<T>` as receivers; `06` needs narrower ones (`list<string>.join`). | A receiver may be any list or map type, and the method applies only to receivers that fit it. |

### 2.8 Three implementations: what Rust adds (C1)

The documents were written for Go and TypeScript. Both have a garbage
collector, and both can look inside a host value at run time. Rust has neither,
and it cannot recover from a stack overflow. Each rule below is one the
specification needs anyway. Rust is what shows it.

| # | Decision | Status |
| --- | --- | --- |
| R1 | **Three native implementations, each written from the specification and the suite: Go, Rust and TypeScript.** 3.0 is final when all three pass the suite. | Owner (C1) |
| R2 | **The embedding model must not assume reflection.** "An object with named fields" (`05` A.3) is supplied through an interface the binding defines. In Go that is reflection. In TypeScript it is the object's own properties. In Rust it is a trait, which the host implements by hand or by a derive macro, with implementations supplied for the standard collections and for JSON values. Reading in place stays the rule in all three. | Ruling |
| R3 | **No implementation needs a garbage collector, and the specification keeps it that way.** Values are immutable, a function value cannot be stored (E6), and nothing is recursive, so no value a run creates can refer to itself. Reference counting, or one region of memory released when the run ends, is complete. A later feature that could make a cycle (a recursive `const`, a lazy value, a mutable cell) must say how an implementation without a collector handles it. This is a second reason for E6. | Ruling |
| R4 | **The allocation budget fits a region allocator exactly.** MiniJinja, in Rust and in Go, is the closest precedent for L3: one cumulative counter, about one unit per 64 bytes, charged before a value of known size is made. No project found fixes its counts across implementations, so L6 is new (`01` 4.1). L3 counts what a run creates, not what is live. An implementation that takes every value from one region and frees it at the end holds at most the budget times its constant. | Ruling (no change to L3) |
| R5 | **Every recursion an implementation performs is bounded by a number the specification fixes, and passing it is the same error everywhere.** A stack overflow in Rust ends the process. In JavaScript it can be caught, but the depth at which it comes depends on the engine. Neither can be the limit. There are three recursions: over the source (parser, checker, compiler), over calls, and over values (`==`, rendering, encoding). The documents bound the second and third with the depth limit and say only that "the parser limits" the first. Missing, and now required: a maximum nesting of source in the specification, tested by the suite; and a statement of how the limits combine, because call depth times the nesting of each body is the stack an evaluator really uses. `02` 2.16 now supplies both. Source may open 100 levels at once, counted on the text, where a bracket, a prefix operator and the right operand of a binary operator each open one. A long sum, a long method chain, an else chain and a long run of `const` lines open none, and an implementation must loop along them. The depth limit counts each call as 1 plus its nesting within its body, so the stack is a sum and not a product: 2D + 100 levels for a run, 500 at the default, against 20,100. `03` 2.17 bounds the checker's own recursion the same way, and type depth at 64. | Ruling. The numbers are D31 |
| R6 | **Releasing a value is not a walk the language makes, so no limit covers it.** MiniJinja overflowed its stack on a value built by about 1,500 concatenations, with no deep source (`01` 3.4). `xs.reduce((acc, x) => [acc], [])` builds a value nested as deep as `xs` is long, and that is legal. An implementation that frees values by recursion must not: it frees iteratively or from a region. This is a note for Rust; Go and JavaScript are unaffected. | Ruling |
| R7 | **A host function cannot keep or change its arguments (S2, S3). Rust enforces this at compile time,** by lending the arguments for the length of the call. It stays a contract in Go and TypeScript. | Note |
| R8 | **The fallback for a host in any other language is the Rust implementation,** built for WebAssembly or behind a C interface, with data copied in. It replaces "offer a WebAssembly build" in `01`. A Go core built for WebAssembly is the largest and slowest option. | Ruling |
| R9 | **The stages that take text and give data need not be written three times.** Parse, analyze, check, format and editor assistance never touch a host value, so the argument against one shared core does not apply to them. The target is still a native checker in each implementation. If the third checker lags, the TypeScript package may carry the Rust checker as WebAssembly. The two projects found with Rust, Go and JavaScript implementations, Cedar and MiniJinja, both serve JavaScript that way for everything, evaluation included; MiniJinja warns against it in browsers (`01` 3.2). A Go host has no such route without a WebAssembly runtime as a dependency. | Ruling; a fallback, not the plan |
| R10 | **The text unit stays the weight of `04` A.5 (L7),** now measured in all three. Two of the three implementations store UTF-8, which is the argument for bytes. The argument for weight is the short string: a string under 64 units is free to measure in every implementation, and with bytes a JavaScript string of 22 to 63 units is not. See the table below. | Ruling; owner's decision D15 |
| R11 | **Regular expressions (T4).** Go's and Rust's engines both run in time linear in the input, but their default syntax is not the same. Only the TypeScript implementation would need its own matcher. This does not change the recommendation to leave them out of 3.0: three engines must agree on one subset before an entry can be released. | Ruling; details in `01` |
| R12 | **Case mapping and the `math` group.** Each standard library follows its own Unicode version, which changes with its releases (Go 1.26 has 15; current Go, Rust and Node report 17), and they already disagree: Go upper-cases `ß` and lower-cases a final sigma differently from Rust and JavaScript (`01` 3.4). This strengthens D25: the table is the specification's. `math` stays implementation-defined, and in Rust and Go the result of a transcendental function may also differ by platform. | Ruling |

What it costs to know the size of one 4,096-unit string that an implementation
only reads (measured, Apple M4 Max, `lab/textsize`):

| Unit | Go | Rust | JavaScript (Node 22) |
| --- | --- | --- | --- |
| Weight (the UTF-16 length) | 222 ns ASCII, 466 ns mixed | 20 ns ASCII, 566 ns mixed with a plain count | free |
| UTF-8 bytes | free | free | 85 ns ASCII, 1,269 ns mixed with `TextEncoder.encodeInto`; 5 µs with a loop |
| The same, 16 units | free either way | free either way | free by weight; 20 ns by bytes |

The Rust count is a first attempt. The standard library counts code points in
117 ns for the same string, so the mixed figure can come down.

### 2.9 The TypeScript lean (C6) and the lighter break (C7)

The syntax is already JavaScript's, so C6 confirms more than it changes. It
reaches two places where the documents kept an older spelling. C7 removes a
reason that several rulings leaned on.

| # | Decision | Status |
| --- | --- | --- |
| Y1 | **Where a construct is valid in both languages it means what it means in TypeScript.** The listed exceptions (`02` 3.1) each remove a wart: `in` on a list, `==` without coercion, template rendering, a block's value, and a default that replaces null. B5 adds one: an empty list or map is falsy. | Ruling |
| Y2 | **The warts that stay out:** `==` coercion and `===`, `undefined` beside `null`, `var` and reassignment, silent loss of integer precision, NaN, ordering by UTF-16 unit, `[object Object]`, automatic semicolon rules, `this`, prototypes. | Ruling (all already in `02`) |
| Y3 | **One spelling for the absent value.** `02` accepts `nil` and `null` "because each has a large body of existing text". Under C7 that body is small, and `02`'s own rule is one way to write each thing. `null` is the keyword, `nil` gets a targeted error, and the documents say "null". | Owner (D28) |
| Y4 | **Type text in TypeScript's spelling where TypeScript has the same idea.** `03` writes `list<T>`, `T?`, `->` and `dynamic`. TypeScript writes `T[]`, `T \| null`, `=>` and `any`. Type text is what an author reads in every diagnostic. `int` and `float` have no TypeScript spelling and stay, as do `bool`, `map<T>`, the `!` mark and `where`. A callback's parameters are named, and a predicate's result is `unknown`. `03` 2.2 has the grammar. | Owner (D29) |
| Y5 | **A break with expr or v2 is not, by itself, a cost** (C7). The rulings that gave "v2 and expr both do this" as the reason to hesitate (N4, E7, D8) stand on their other reasons. The converter for stored expr programs becomes a small tool for the workflow engine's own data, not a product. | Ruling |

### 2.10 Statements (C3)

C3 asked for "some preference for expressions generally (rather than lots of
statements)". E5 turned that into "no statements". The owner has said that is
more than was asked. E5 was a ruling made here, not the owner's. The owner has
now decided X1 to X3.

| # | Decision | Status |
| --- | --- | --- |
| X1 | **`if`, `else if` and `else` form an expression with block bodies:** `if (n > 3) { "big" } else { "small" }`. Its value is the value of the branch taken. The rules are below. | Owner (D35) |
| X2 | **A binding is written `const`.** In TypeScript `let` means "this will be reassigned", and nothing in v3 can be. `let` and `var` are reserved, and each gets a targeted error that says `const`. | Owner (D36) |
| X3 | **Loops and reassignment are not in 3.0.** They come as a package: a loop with nothing to change does nothing. With immutable values, a loop that builds a list copies it on every pass, and L6 charges that as written, so a 10,000-element loop costs 50 million units against a budget of one million. Collections that can be changed in place would undo three foundations: no cycles (R3), limits that are complete (L8), and narrowing that stays sound when two names share one value. | Owner: deferred |
| X4 | **A bounded middle, for a later version:** `for (const x of xs)` with reassignable locals, no `while`, values still immutable. It keeps termination and needs no collector. It is good for sums and counts, which `reduce` already does, and bad for building collections, for the reason in X3. Ported v2 scripts are the evidence that decides it. | Deferred with X3 |
| X5 | **Early `return` from a block or a lambda,** for guard clauses. It harms no foundation, but it is a non-local exit in three evaluators and a second way to give a block its value. | Deferred with X3 |
| X6 | **The words are reserved now,** so that X4 and X5 can arrive without a break: `let`, `var`, `for`, `while`, `do`, `break`, `continue`, `return`, `function`. Each is a targeted error in 3.0. `of` is not reserved: it has a meaning only after `for (const x`. Earlier revisions said `for` and `while` were reserved already. They were not. | Ruling |

**The rules of `if` (X1).**

| Rule | Text |
| --- | --- |
| Shape | `if (c) { ... } else if (d) { ... } else { ... }`. The parentheses and the braces are required. |
| Branch | A branch is a block body: zero or more `const` lines, then one expression. After `if (c)` and after `else`, a `{` always opens a branch, never a map. A map as a branch's value is written inside the branch: `{ {id: 1} }`. A branch written like a map (`{ a: 1 }`) or left empty is a targeted error that says so. |
| Elsewhere | Outside a branch the rule of `02` 2.1 stands with the new word: a `{` starts a block exactly when the next token is `const`. |
| `else` | Required in 3.0 (D37). Nothing has an effect, so an `if` with no `else` could only mean "this value or null", and `c ? a : null` says that in the open. Allowing the short form later is an addition, not a break. |
| Test | By truthiness (B5), as for `? :`. The check class `constant-condition` (B7) covers it. |
| Value and type | The value of the branch taken. No other branch is evaluated. The type is the join of the branch types. The test narrows names in its own branch, and its negation narrows them in every later branch, as for `? :`. |
| Place in the grammar | An `if` is a whole expression, at the level of `? :`, not an operand. It may stand wherever an expression may: a `const` value, an argument, a lambda body, a branch, a list item, a map value, an interpolation. As an operand or a chain receiver it needs parentheses: `(if (c) { 1 } else { 2 }) + 3`. So no reader has to ask whether `+ 3` or `.map(f)` belongs to the last branch or to the whole. |
| Cost and nesting | Steps as for `? :`. The parentheses and each branch's braces open a nesting level as brackets do (`02` 2.16). An `else if` opens none, so a long chain is handled by a loop in each implementation. |
| `? :` and `match` | Both stay. `? :` is the short form inside a larger expression. `match` by cases checks that every case is covered. `match` by conditions now says what an `else if` chain says (D38). |

### 2.11 Checked conversion of untyped data

The owner asked for a plan for one form of user-defined types: a tested
conversion from untyped data to a type, with its overlap with JSON Schema. The
plan is `07-checked-conversion.md`, and the owner accepted it (D39 to D45).

| # | Proposal | Status |
| --- | --- | --- |
| V1 | **`x as T` tests a value against a type at run time** and gives it back with that type, or fails with a `value` error. `x is T` gives a `bool` and narrows. The value never changes: nothing is coerced, filled in or removed. | Owner (D39, D41, D42) |
| V2 | **The rule is the one `03` 2.14 already defines** for "a value conforms to a type". Every binding also gives it to the host as a function, so a host can test the checker's precondition. | Owner |
| V3 | **A type comes from type text written in place, or from a name the host declares,** in type text or JSON Schema. A `type` line in a script comes in a minor version, with annotations. | Owner (D40, D45) |
| V4 | **It checks types, not schemas.** A JSON Schema's value constraints are dropped, as they are today. Full validation stays with the host. | Owner (D43) |
| V5 | **Not planned:** classes or records with methods, coercion, removing unknown keys, value constraints. **Its own plan later:** recursive types. Tagged unions now have theirs (`09`, D47). | Owner |

### 2.12 Six more areas: strings, unions, errors, tests, config files, queries

The owner named six areas where the language should offer more. Each has a
plan, and the owner accepted all six (D46 to D51). A plan's own decisions are
cited by its numbers, such as `08-4`.

| # | Proposal | Plan | Status |
| --- | --- | --- | --- |
| W1 | **Five string forms, chosen by two questions: one line or several, and values and escapes or neither.** A text block, `"""`, holds text of several lines, and the column of its closing delimiter is the indentation removed. A raw prefix, `r`, turns off escapes and interpolation. Both are syntax errors in TypeScript today, so no shared text changes meaning. | `08` | Owner (D46) |
| W2 | **Tagged unions.** A type may hold several record alternatives when one required field with string literal values tells them apart. The checker finds that field. A test of it narrows, `match` on it is checked for coverage, and a conformance test reads it first, so a failure names one variant. A union of records with no tag is an error. Enums stay string literal sets. | `09` | Owner (D47) |
| W3 | **A type mismatch is reported at the element that is wrong.** Where a list or map literal must fit a written type, the fit goes through the literal's form. This moves the span of two existing codes inward for every program and changes no type. | `09` 2.6 | Owner (D47) |
| W4 | **`try { e } catch (err) { f }` is the one error-handling construct,** an expression placed as `if` is. It replaces `try(e, fallback)`. `err` is an ordinary map with `code`, `message`, `path` and `data`. A limit, a cancellation, a crash and a missing variable are not caught. There is no `finally`, and raising is `fail(message, data?)`. | `10` | Owner (D48) |
| W5 | **A run diagnostic carries a trace, and every message comes from one catalog.** The trace lists the calls in progress. The catalog, `messages.json`, is data in the specification, so the three implementations word an error the same way and the suite compares messages. The layout of a printed diagnostic is specified once. | `10` | Owner (D48) |
| W6 | **Tests are data.** The conformance suite's case format is the user's format: an environment, stubs for host functions as rows of data, and cases with what is expected. No test construct enters the language. `risor test` replaces `risor conform`, and each binding has a helper that runs a case file in the host's own test runner. | `11` | Owner (D49) |
| W7 | **A config file is a program whose value is data.** The host declares the type, checks the file against it, runs it and decodes the value into its own type. No data mode and no function that reaches the host. Imports are designed and come in a minor version; `import` is reserved now. | `12` | Owner (D50) |
| W8 | **Queries are method chains, with one new step.** `orders[*].lines[*].sku` reads through nested lists and gives one flat list. Seven library entries close the gaps in sorting, grouping and aggregating, and `filter` and `find` narrow. Comprehensions may follow in a minor version. A query expression, a filter step, an implicit parameter and a pipe are not planned. `01` lists projections under "avoid"; that line is about the implied kind, and this one is written. | `13` | Owner (D51) |

## 3. The design in brief

Each line points at the document that owns the detail.

| Area | What v3 is | Owner |
| --- | --- | --- |
| Shape | A language defined by a specification and a data conformance suite, with independent implementations in Go, Rust and TypeScript. One core compiled to WebAssembly was rejected for evaluation because no such design keeps "host values read in place". The Rust implementation is the base for hosts in other languages. | `01`, sections 2.1 and 2.8 |
| Program | `const` bindings followed by one expression. The same thing in braces is a block expression, usable anywhere. A `when:` field, a script and a config file are the same language. 32 node kinds, against 43 in v2. | `02` |
| Syntax | JavaScript's expression syntax: arrow functions, method calls, `?.`, `?.[`, `??`, `? :`, spread, template strings, destructuring, default parameters, `**`. Text blocks and raw strings. `if` and `try` as expressions. A projection step, `xs[*].f`. A `match` expression by literal cases or by conditions. Text valid in both languages means the same, with a short listed set of exceptions. | `02` |
| Values | null, bool, number, string, list, map, and opaque host values. Nothing can be changed; spread builds the updated copy. | `02` |
| Numbers | One kind, with double semantics. Whole numbers are exact to ±(2^53 − 1) and an out-of-range result is an error. `/` is true division. | `02`, section 2.2 |
| Conditions | Truthiness: null, `false`, zero and anything empty are falsy. `&&` and `\|\|` return an operand. `\|\|` replaces a falsy value, `??` replaces null. | `02`, section 2.3 |
| Types | A type is a set of alternatives, at most one per kind, but for records that a tag tells apart. `int` (a whole number) fits `float`. Records are exact or open, with optional fields. String literal sets. `any` for unchecked data. No user generics, no function types outside library signatures. | `03` |
| Checking | Bidirectional and local. Lambda parameters take their types from the signature they are passed to. Null is narrowed by flow. A 29-code diagnostic catalog with specified spans. | `03` |
| The promise | A checked program cannot fail with an unknown name, a missing field, a null, a wrong kind or a wrong argument count. Every other failure is at a site the checker lists by class, or is a range or limit error. A host chooses which classes of site to reject. | `03` |
| Evaluation | In Go: closures compiled from the AST, values as plain Go values, host data read in place through one read barrier. A simple reference layer, and a typed layer over it for speed on Go structs. In Rust: no collector, values lent from the host for the length of a run, host records through a trait. | `04` |
| Limits | Steps, allocation units and a deadline, plus a depth limit. Steps and units are fixed by the specification version, so a program reaches a limit at the same point in every implementation. | `04`, section 2.5 |
| Host model | Parse, analyze, check, compile, run. An immutable environment declares names and types; values arrive per run. One error type with stable codes, a trace, and messages from one catalog. Templates with `${{ }}`. Tests as data. Analysis of names, paths and calls without an environment. | `05` |
| Library | 78 functions and methods and 2 constants. 59 are in the default core and all 59 are exact across implementations. Optional groups: `math`, `time`, `json`, `base64`. One specification table defines every entry, with its cost formulas and its examples as conformance cases. | `06` |

### Evidence, and its limits

| Claim | Basis |
| --- | --- |
| Closures beat the v2 VM by 2.8x to 4.3x on compute-heavy programs | Measured, with real parsed programs (`experiments/v3/README.md`) |
| Allocation accounting costs 0% to 5% outside long text building | Measured on hand-built closure trees, on a busy machine: indicative (`04`) |
| Fixing counts across implementations costs nothing measurable at run time | Measured the same way (`04`) |
| The design can be implemented in TypeScript and in Rust | Inference. Nobody has written an evaluator in either. The notes in `04`, `05` and `06` are reasoning, not experiment. The one thing measured in all three is the text-size scan (section 2.8) |
| Size of the Go implementation: about 19,000 lines without tests | Sum of each document's estimate after D46 to D51: front end 4,500, checker 2,935, evaluator 4,810, host binding 4,500, library 2,600. Before D39 the sum was about 15,000; checked conversion and the six areas of section 2.12 added the rest, most of it in the host binding (the catalog, case files, the load of a config file). The parts overlap, so the sum is an upper figure. v2's core is about 32,000 and expr about 9,000 |

## 4. Decisions for the owner

Each is written to in the documents as recommended. Reversing one means a
further revision of the documents named.

**Settled by the owner since the second revision:** D1, D2 (which
implementations), D3, D5, D6, D7 and D9 as recommended, and D4 against the
recommendation. The owner was shown those and agreed with all but D4.
D10 to D26 were not put to the owner one by one and are still recommendations.
D28 to D34, except D31, were put one by one and are settled as recommended.
D27 and D31 were not asked. D35 and D36 were put and are settled as recommended. D37 and D38 are
new details of `if`, ruled here and not asked. D39 to D45 were put and are
settled as recommended. D46 to D51 are six plans, each accepted with every
decision as it recommends; within them the owner chose the form of `try`, the
timing of imports, the message catalog and the direction for queries by name.

### 4.1 Strategy

| # | Decision | Recommendation | Touches |
| --- | --- | --- | --- |
| D1 | Is v3 the successor to expr, with the workflow engine moving to it, or a sibling? | **Settled: successor.** Users of expr and of v2 are expected to move (C7) | all |
| D2 | Which implementations, and in what order? | **Settled: Go, Rust and TypeScript.** Order is still a recommendation: Go first, then a reference evaluator in TypeScript and one in Rust, all before the checker is built (section 5) | all |
| D27 | Is 3.0 final only when all three implementations pass the whole suite, checker and library included? | Yes. The alternative is to ship Go and TypeScript and let Rust follow, which the prior art says is how a third implementation stays "mostly compliant" | all |

### 4.2 Breaks with v2 and expr

| # | Decision | Recommendation | Cost of the recommendation |
| --- | --- | --- | --- |
| D3 | Whole numbers exact only to ±(2^53 − 1) (N2) | **Settled: yes** | A Go host holding 64-bit ids or nanosecond timestamps above the range gets an error where the value is read, even if the program only passes it on. Such fields are presented as strings |
| D4 | Strict booleans | **Settled: no. The language has truthiness** (B1, B5 to B8) | Operand-returning `&&` and `\|\|` give union types the checker must carry. A host that wants a real boolean checks the result type |
| D32 | Are `always-truthy` and `always-falsy` errors, or a class a host may turn off, like the residual sites of `03`? | **Settled:** a class a host may turn off, on by default. `03` recommends plain errors. The case for the class: a host that re-checks stored programs after a schema changes a field from optional to required should not see `x && x.f` start to fail | A second kind of host-selected strictness in the checker |
| D33 | Does the workflow engine keep requiring a real `bool` for `when:`, or test by truthiness (B8)? | **Settled:** truthiness, with `condition` as the expected type. It is what the language now does everywhere else, and it accepts `report && report.verdict == "APPROVE"` | A list or a string in `when:` is accepted when it can be empty, and reported only when it never can |
| D30 | The falsy set: null, `false`, zero and anything empty (B5), or JavaScript's set, where an empty list and an empty map are truthy | **Settled:** the first | `xs \|\| ys` with an empty `xs` gives `ys`, where JavaScript gives `xs`. It joins the list of differences in `02` 3.1 |
| D5 | `/` is true division (N4) | **Settled: yes** | `7 / 2` was `3` in v2 and expr. An index from a division needs `int(...)` |
| D34 | Close the hole in E6: a lambda is passed only to library functions, and a parameter cannot be called (E9) | **Settled:** yes. It is what makes "every program terminates" true, which is the reason for D6 | Helpers that take a function, such as `const twice = (f, x) => f(f(x))`, cannot be written |
| D6 | No recursion or loops in 3.0 (E6) | **Settled: yes** | A v2 script that uses a loop, a statement or mutation has no direct v3 form. It is rewritten with `map`, `filter` and `reduce`, moved into a host function, or left on v2. This is the one break that C7 does not make cheap |
| D7 | JavaScript's names for string and list methods (`toUpperCase`, `some`, `every`), expr's for functions (`len`, `int`, `round`) | **Settled: yes** (and C6) | Longer names than expr's. The other spellings give a compile error that names the fix |
| D8 | `len`, `int` and `float` reject null | Yes | `len(x ?? [])` where expr accepts nil today (v3: null) |
| D9 | Regular expressions are out of 3.0 and reserved for a minor version | **Settled: yes** | Three engines would have to agree on one subset, the TypeScript implementation would need its own matcher of about 700 lines, and a released entry cannot change |

### 4.3 Language details

| # | Decision | Recommendation |
| --- | --- | --- |
| D10 | `match` ships with literal cases and conditions only: no binding, guards or structural patterns | Yes; they can be added later without a break |
| D11 | `??` beside a comparison or logical operator needs parentheses | Yes; it is the only option with no silent misreading |
| D12 | Two departures from JavaScript: `in` on a list means membership, and a default replaces null as well as "absent" | Keep both |
| D13 | Literal sets are strings only, so a `match` on a numeric enum needs a default arm | Accept |
| D14 | Template delimiters fixed at `${{ }}` (S14) | Fixed |
| D35 | `if` / `else if` / `else` as an expression (X1) | **Settled:** yes |
| D36 | `const` in place of `let` for a constant binding (X2) | **Settled:** `const`, with `let` and `var` reserved. Loops, reassignment and early `return` are deferred (X3 to X5) |
| D37 | Must an `if` have an `else`? (2.10) | Yes in 3.0. The other choice is that a missing `else` gives null. That can be allowed later without a break. The reverse cannot |
| D38 | Does `match` by conditions stay, now that `else if` says the same thing? (2.10) | It stays in the documents for now. I would drop it: two spellings of one thing, and `if` is the one a TypeScript author knows. `match` by cases stays either way |
| D28 | One spelling for the absent value: `null`, with `nil` a targeted error (Y3) | **Settled:** yes. It follows C6 and "one way to write each thing". The cost is that Risor has said `nil` since v1 |
| D29 | Type text in TypeScript's spelling: `T[]`, `T \| null`, `=>`, `any` (Y4) | **Settled:** yes, where TypeScript has the same idea. `int`, `float` and `map<T>` stay |

Checked conversion (`07`, section 9). All settled by the owner as recommended:

| # | Decision | Recommendation |
| --- | --- | --- |
| D39 | `as` and `is` as the words, with `as` tested and not trusted as in TypeScript | **Settled:** yes. Reserve both in 3.0 whatever the timing |
| D40 | Timing | **Settled:** the host's function, `as`, `is` and host-declared names in 3.0. `type` lines and annotations in a minor version |
| D41 | `x as {id: string}` fails when the value has any other key, because a record with no `...` is exact | **Settled:** keep, with a message that names the fix. The least sure of the seven: most prior art allows other keys |
| D42 | `note?: string` does not accept `note: null` | **Settled:** keep |
| D43 | A named type from a JSON Schema carries no value constraint into `as` | **Settled:** types only |
| D44 | A named type prints by its name in a diagnostic | **Settled:** not in the first version |
| D45 | `type` is a contextual word for a later `type Name = ...`, and `type(x)` stays | **Settled:** yes |

Six more areas (section 2.12). Each row is one plan, and the plan's own table
lists its decisions with the strongest alternative to each. All settled by the
owner as recommended:

| # | Decision | Recommendation |
| --- | --- | --- |
| D46 | String forms: a text block and a raw prefix (`08-1` to `08-11`) | **Settled:** both in 3.0. A line indented less than the closing delimiter is an error. A raw string cannot hold its own delimiter |
| D47 | Tagged unions, and a literal's fit through its form (`09-1` to `09-8`) | **Settled:** the simplest form. The tag is found, not written. Only a written type makes a tagged union, so a value rebuilt in branches needs `as` |
| D48 | Errors (`10-1` to `10-8`) | **Settled:** `try { } catch (err) { }` replaces `try(e, fallback)`, by the owner's own choice. No `class` on the error value. No raising again unchanged in 3.0. The full message catalog, by the owner's own choice |
| D49 | Testing (`11-1` to `11-9`) | **Settled:** one case format, `risor test`, and no test construct in the language |
| D50 | Config files (`12-1` to `12-9`) | **Settled:** data only, and imports in a minor version after 3.0, both by the owner's own choice |
| D51 | Queries (`13-1` to `13-3`) | **Settled:** library entries and the projection step in 3.0. Comprehensions in a minor version, on evidence. A query expression is not planned |


### 4.4 Limits

| # | Decision | Recommendation |
| --- | --- | --- |
| D15 | The unit of text size is the UTF-16 length in blocks of 64 (L7, R10) | Yes, but it is closer than it was. JavaScript has it for free. Go and Rust scan strings of 64 bytes or more: in Go, comparing two equal 4 KB strings goes from 46 ns to 483 ns. UTF-8 bytes would be free in Go and Rust and would make JavaScript measure every string of 22 units or more, at 20 ns and up. Short strings are the common case, and weight is the unit that makes them free everywhere |
| D16 | Step and unit counts are fixed per specification version (L6) | Yes. Optimizations that stop early are then forbidden, and a wrong rate stays until the next version |
| D17 | Defaults: 1,000,000 steps, 1,000,000 allocation units, depth 200, no deadline | Yes. A deadline is the one limit that is not deterministic |
| D31 | Source nesting is at most 100 levels, fixed by the specification and not settable by a host, and the depth limit counts a call as 1 plus its nesting (R5, `02` 2.16) | Yes. Without it a deep program is an error in one implementation, a crash in another and a success in the third. Settle the numbers after the Rust reference evaluator has run the limit cases in its least favorable build (`01`, question 7) |

### 4.5 Hosts and the repository

| # | Decision | Recommendation |
| --- | --- | --- |
| D18 | No asynchronous run in the TypeScript implementation in 3.0; a host fetches what a program needs first, using the analysis API | Yes |
| D19 | A host value is a record or opaque, never both | Accept for 3.0 |
| D20 | A JSON Schema property outside `required` is an optional field unless the host says it fills absent properties | Yes |
| D21 | The Go implementation returns errors and does not panic | Returns: never slower on the success path, 38 ns against 184 ns on failure, about 600 more lines |
| D22 | The typed layer ships in 3.0 | Yes, built after the reference layer. Without it a host that passes Go structs pays one allocation per scalar field read |
| D23 | The core module's tests stop using wonton's `assert`, so the module has no dependencies | Yes. This changes a rule in `AGENTS.md` |
| D24 | The language server is dropped and the core gains completion, hover and describe calls | Yes |
| D25 | Case mapping comes from a table the specification owns, pinned to one Unicode version | Yes. The alternative is ASCII only |
| D26 | Times are seconds since the epoch as a number | Yes. Nanoseconds do not survive either way |

## 5. Scope, migration and roadmap

### 5.1 What is built

| Deliverable | Source |
| --- | --- |
| The specification | The normative parts of `02`, `03`, `04` Part A, `05` Part A and `06` |
| The conformance suite and the library table | Data files, shared by every implementation (P4, `05` A.11, `06`) |
| The Go implementation | `04` Part B, `05` Part B |
| The TypeScript implementation | `04` Part C, `05` Part C |
| The Rust implementation, and from it a build for hosts in other languages (R8) | `04` Part D, `05` Part D |
| The message catalog | `messages.json`, a data file beside the library table: a template for each code of every phase (`10` 2.9) |
| Tools | A small CLI (`eval`, `check`, `fmt`, `test`), a REPL that loads JSON as variables, an editor grammar |

### 5.2 Order of work

The order puts the riskiest claim first. That claim is C1: nobody has yet shown
that the design can be built twice, let alone three times. The two other
implementations test different rules. TypeScript tests numbers, text and order.
Rust tests the embedding model, memory without a collector and the depth rules.

| Step | Work | Proves |
| --- | --- | --- |
| 1 | The owner settles what is left of section 4. D37 and D38 shape the grammar | |
| 2 | Conformance format and first cases. Go lexer, parser and AST | The grammar of `02` |
| 3 | Go reference evaluator with all three limits and a core subset of the library. The spike's benchmarks run through the real API | The performance and limits claims, on real programs and not hand-built trees |
| 4 | **A TypeScript lexer, parser and reference evaluator, from the specification and the suite only** | C1. Every Go-shaped rule that survived the documents shows up here, before the specification is fixed |
| 5 | **A Rust lexer, parser and reference evaluator, the same way,** with host records read through a trait and values lent for the length of a run. From this step on, a differential test generates programs and data and compares value, error class, steps and units across the implementations that exist (`01`, question 9) | R2 to R6. Whether the embedding model of `05` Part A holds without reflection or a collector |
| 6 | The checker in Go: type text, JSON Schema mapping, diagnostics, the soundness fuzzer | The promise of `03` |
| 7 | The host binding in full, the whole library from the table, the accounting tests, and suite cases for what a binding must reject: an integer outside the safe range, malformed text, a value nested past the limit (`01`, question 10) | `05`, `06` |
| 8 | Acceptance: the workflow engine ported on a branch, passing its own soundness fuzz tests, and the benchmark targets of `04` | That v3 can replace expr |
| 9 | The checker and library in TypeScript and in Rust; the typed layer in Go; tools; the build of R8 if a host needs it | 3.0 (D27) |

Steps 4 and 5 are early on purpose. The cheapest time to find that a rule cannot
be implemented the same way three times is before the checker and the library
are built on it. Both toolchains are on the development machine (Node 22, Rust
1.99), so neither step waits on anything.

Three of everything is the main cost of C1. The checker is about 2,935 lines and
the library 78 entries, each written three times and kept in step by the suite.
R9 is the fallback if that proves too much for the third checker.

### 5.3 What happens to v2 and expr

| Thing | Fate |
| --- | --- |
| Risor v2 | Kept in maintenance on its own branch. Its users are expected to move (C7), except for scripts with statements, loops, recursion or mutation, which have no v3 form (D6). v3 is a new module path |
| Removed from v2 for v3 | The compiler, bytecode, VM, object model, syntax allow-list, modules as objects, script test runner and language server: about 18,900 lines of core, plus tools |
| Kept from v2 | The Pratt parser's structure and depth limit, the error list with positions, the suggestion code, the caret rendering |
| expr | Replaced by v3 (D1). Its type model, narrowing, template splitting and test tables are reused |
| Stored expr programs | Changed mechanically: `it` forms to arrow functions, pipes to method chains, `if()` to `? :`, `?[` to `?.[`, function calls to methods. `\|\|` defaults keep their meaning now that v3 has truthiness with the same falsy set (B5). A small converter built on expr's parser does this for the workflow engine's own data; it is not a product (Y5) |
| v2 scripts | No automatic path. A script that uses statements stays on v2 |

Three defects in v2 as released turned up during this work. They are worth
fixing in v2 whatever happens to v3:

- `&&` and `||` share one precedence level, so `true || false && false` is
  `false` (`pkg/parser/precedence.go:51-52`).
- The registry documents `encode` and `decode` with the arguments in the wrong
  order, and both documented examples fail (`pkg/builtins/registry.go:95`,
  `:103`).
- `regexp.match(...)` cannot be called, because `match` is a keyword.

### 5.4 Risks

| Risk | Why it matters | What reduces it |
| --- | --- | --- |
| The three implementations drift | The prior art says this is the normal state of second and third implementations, that the lag sits in the checker and the library, and that the bindings diverge more than the evaluators. One Go port of a Rust policy language had a decoder that turned a deny into an allow (`01` 3.1) | Step and unit counts asserted in suite cases; a public conformance report per implementation; steps 4 and 5 early; D27 |
| Three of everything is too much work | A checker and a 78-entry library, three times, for a project with few maintainers | The language is small by ruling; the suite and the library table are shared data; R9 as a fallback |
| A deep program crashes a Rust host | A stack overflow cannot be caught there | R5, R6 and D31: every recursion has a specified bound, and release is not recursive |
| The whole-number range hurts Go hosts | 64-bit ids and nanosecond times are common in Go data | The error is loud and at the read; the binding can present a field as a string. D3 should be confirmed knowing this |
| The cost table is wrong and frozen | A released rate cannot change within a specification version | Calibrate the rates on the real evaluator before 3.0; the accounting test bounds real time and memory per unit |
| The break is large for existing users | Division, method names and the loss of statements all change | The owner judges the user base small and willing (C7). Truthiness now matches v2 and expr. Errors name the fix. The loss of statements is the part with no mechanical answer (D6) |
| Three checkers to keep in step | The checker is the largest part to write three times, and truthiness adds union types to it | `03` puts it at about 2,935 lines with truthiness and type tests; the diagnostic catalog and conformance cases pin its behavior |
| The evidence is thinner than the documents sound | The allocation and count measurements are from hand-built trees, and the TypeScript and Rust notes are inference | Steps 3, 4 and 5 replace all of it with real measurements before much else is built |
