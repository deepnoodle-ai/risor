# Risor v3 design: overview and cross-cutting decisions

Status: design proposal. It records the owner's constraints, the decisions that
span more than one of the six aspect documents (`01` to `06`), the decisions
still open for the owner, and the roadmap. The aspect documents have been
revised to agree with it. Where one still disagrees, this file wins. Seven
plans, `07` to `13`, each hold one feature that was designed after the aspect
documents. Four more, `17` to `20`, come from the review of issues #640 and
#641 (D53 to D56). Each plan is the authority for its feature. `02` to `06`
carry the rules of `07` to `13`. They do not yet carry the rules of `17` to
`20`: each track of `14` 5.3 adds them when it lands. Plan `16`, regular
expressions (D52), was withdrawn.

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

Eighth revision. Issues #640 and #641 were reviewed on 2026-11-14, and plans
`16` to `20` were written from the review (D52 to D56). On 2026-11-19, after a
further review and issue #644, the owner withdrew D52, trimmed D54 to D56, and
settled D57 (passing functions to user-defined functions) and D58 (a lower
largest depth). D59 and D60 are open. Section 4 has the rows and "Where the
plans meet"; `14` section 5 orders the work as tracks A to F. `02` to `06`
are not yet revised for D53 to D58: each track revises the specification
when it lands, and the plans are the authority until then.

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
| B2 | A missing field is an error with `.` and null with `?.`. An optional field (`f?: T`) is distinct from a field whose value may be null. | Ruling (all documents agree). B2 holds except at the last step of a coalesced read: an operand of a `??` chain other than its last, where an absent key gives null (D56, `20` 3.1). A key that a closed record cannot have stays `no-field`. |
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
| E6 | **No recursion and no loops in 3.0**, so every program terminates and its work is bounded by its text and its data. Lambdas are arguments and `const` values, not general values. This is the one place where C5 is chosen over C4. | Ruling, confirmed by the owner. Partly reversed by D54: a `const`-bound lambda with typed parameters and result may call itself, and every run ends at its value, a failure or a limit (`18` 3.4). Loops stay out. D57 lets a function reach a user lambda's `Fn` parameter, and still not a list, a map, a result or the host. |
| E7 | **Names in the library follow least surprise for a JavaScript-shaped language.** `06` chose expr's names (`upper`, `any`, `all`) and gives hints for JavaScript's (`toUpperCase`, `some`, `every`, `includes`). `06` is to set the two columns side by side and choose again with C2 as the test. | Candidate for `06` |
| E9 | **E6's claim has a hole.** `02` allows a lambda as an argument to a `const`-bound lambda, and a parameter may be called. Then `const om = x => x(y => x(y)); om(z => om(w => z(w)))` never returns, although nothing refers to itself. Today such a program ends at the checker's limit on body checks or at the depth limit, so it ends by a limit and not by construction. The fix `02` recommends: a lambda may be passed only to a library function or method, and a parameter cannot be called. The call graph is then fixed by the text. The cost is a user-written helper such as `const twice = (f, x) => f(f(x))`. | Ruling, confirmed by the owner (D34): a lambda is passed only to a library function or method, and a parameter cannot be called. Revised by D57: a lambda may also be passed to a parameter typed with an `Fn`, which may then be called. The hole stays closed, because an `Fn` takes no function: `x(y => x(y))` cannot be written (`18` 7.5) |
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
| T4 | **Regular expressions** need a syntax subset that both Go's engine and a JavaScript implementation can run with the same results, and a guarantee of time linear in the input (L2). JavaScript's built-in engine backtracks, so the TypeScript implementation would have to ship its own matcher for the subset. `06` is to recommend: specify the subset, or leave regular expressions out of the 3.0 specification. | Candidate for `06`. Decided by D9: out of 3.0, with `regexp` reserved. D52 would have specified one matcher for all three; it is withdrawn. |
| T5 | **Diagnostic positions** are specified as line and column counted in code points. A byte or UTF-16 offset is an addition each implementation may give in its own unit. | Ruling; `05` to define |

### 2.7 Smaller conflicts between the drafts

| # | Conflict | Ruling |
| --- | --- | --- |
| S1 | `06` needs namespaces (`math.sqrt`); `05` dropped modules and read `math.sqrt(x)` as a method call on a variable. | A namespace is a compile-time prefix on a function name, resolved by the checker. It is not a value. Declaring a variable with a namespace's name is an error when the environment is built. `05` adds declaring a namespaced function and enabling an optional group. |
| S2 | `05` said a host function may mutate its arguments; `04` shares constant collections between runs. | A host function must treat its arguments as read-only and must not keep them after the call. This is a contract, not an enforced check. |
| S3 | `04` lets a host function receive a function value; `05` rules it out; `06` notes hosts then cannot add higher-order operations. | Not in 3.0: a lambda cannot be passed to a host function. This keeps a closure from outliving its run in every implementation. Revisit after 3.0. Kept by D55 (`19-10`), with the reasons recorded there, and by D57, which passes a function to a user lambda only. |
| S4 | `05` types a host function by its Go signature; `04` needs a call path with no reflection. | Go binding detail. Registration accepts any Go function, chooses a reflection-free adapter for the signatures that have one, falls back to reflection for the rest, and can report which path a function got. Planned by D55 (`19-13`), Go only (track C): `risor.Func`, `risor.Methods`, and a generator as a follow-up. Not built yet. |
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
| R3 | **No implementation needs a garbage collector, and the specification keeps it that way.** Values are immutable, a function value cannot be stored (E6), and nothing is recursive, so no value a run creates can refer to itself. Reference counting, or one region of memory released when the run ends, is complete. A later feature that could make a cycle (a recursive `const`, a lazy value, a mutable cell) must say how an implementation without a collector handles it. This is a second reason for E6. | Ruling. Kept by D54 and D57: a recursive call and a call through an `Fn` parameter build only acyclic values, and no function is stored, returned or given to the host (`18` 3.4, 8) |
| R4 | **The allocation budget fits a region allocator exactly.** MiniJinja, in Rust and in Go, is the closest precedent for L3: one cumulative counter, about one unit per 64 bytes, charged before a value of known size is made. No project found fixes its counts across implementations, so L6 is new (`01` 4.1). L3 counts what a run creates, not what is live. An implementation that takes every value from one region and frees it at the end holds at most the budget times its constant. | Ruling (no change to L3) |
| R5 | **Every recursion an implementation performs is bounded by a number the specification fixes, and passing it is the same error everywhere.** A stack overflow in Rust ends the process. In JavaScript it can be caught, but the depth at which it comes depends on the engine. Neither can be the limit. There are three recursions: over the source (parser, checker, compiler), over calls, and over values (`==`, rendering, encoding). The documents bound the second and third with the depth limit and say only that "the parser limits" the first. Missing, and now required: a maximum nesting of source in the specification, tested by the suite; and a statement of how the limits combine, because call depth times the nesting of each body is the stack an evaluator really uses. `02` 2.16 now supplies both. Source may open 100 levels at once, counted on the text, where a bracket, a prefix operator and the right operand of a binary operator each open one. A long sum, a long method chain, an else chain and a long run of `const` lines open none, and an implementation must loop along them. The depth limit counts each call as 1 plus its nesting within its body, so the stack is a sum and not a product: 2D + 100 levels for a run, 500 at the default, against 20,100. `03` 2.17 bounds the checker's own recursion the same way, and type depth at 64. | Ruling. The numbers are D31. D58 sets the largest depth a host may set at 400, from 1,000 (R20) |
| R6 | **Releasing a value is not a walk the language makes, so no limit covers it.** MiniJinja overflowed its stack on a value built by about 1,500 concatenations, with no deep source (`01` 3.4). `xs.reduce((acc, x) => [acc], [])` builds a value nested as deep as `xs` is long, and that is legal. An implementation that frees values by recursion must not: it frees iteratively or from a region. This is a note for Rust; Go and JavaScript are unaffected. | Ruling |
| R7 | **A host function cannot keep or change its arguments (S2, S3). Rust enforces this at compile time,** by lending the arguments for the length of the call. It stays a contract in Go and TypeScript. | Note |
| R8 | **The fallback for a host in any other language is the Rust implementation,** built for WebAssembly or behind a C interface, with data copied in. It replaces "offer a WebAssembly build" in `01`. A Go core built for WebAssembly is the largest and slowest option. | Ruling |
| R9 | **The stages that take text and give data need not be written three times.** Parse, analyze, check, format and editor assistance never touch a host value, so the argument against one shared core does not apply to them. The target is still a native checker in each implementation. If the third checker lags, the TypeScript package may carry the Rust checker as WebAssembly. The two projects found with Rust, Go and JavaScript implementations, Cedar and MiniJinja, both serve JavaScript that way for everything, evaluation included; MiniJinja warns against it in browsers (`01` 3.2). A Go host has no such route without a WebAssembly runtime as a dependency. | Ruling; a fallback, not the plan |
| R10 | **The text unit stays the weight of `04` A.5 (L7),** now measured in all three. Two of the three implementations store UTF-8, which is the argument for bytes. The argument for weight is the short string: a string under 64 units is free to measure in every implementation, and with bytes a JavaScript string of 22 to 63 units is not. See the table below. | Ruling; owner's decision D15 |
| R11 | **Regular expressions (T4).** Go's and Rust's engines both run in time linear in the input, but their default syntax is not the same. Only the TypeScript implementation would need its own matcher. This does not change the recommendation to leave them out of 3.0: three engines must agree on one subset before an entry can be released. | Ruling; details in `01`. D52 proposed one specified matcher instead, and is withdrawn; this ruling stands. |
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
| Library | 79 functions and methods and 2 constants. 60 are in the default core and all 60 are exact across implementations. Optional groups: `math`, `time`, `json`, `base64`. One specification table defines every entry, with its cost formulas and its examples as conformance cases. | `06` |

This table is the design as built on `main`. D53 to D58 change it as `14`
section 5 schedules, and the Counts table of section 4 lists what moves.

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
D52 to D56 are five plans from the review of issues #640 and #641; the owner
settled the direction of each on 2026-11-14, then withdrew D52 and trimmed D54
to D56 on 2026-11-19. D57 and D58 were put and settled the same day. D59 is a
recommendation the owner has not confirmed, and D60 is open. D62 and D63 were
put and settled on 2026-11-24.

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
| D34 | Close the hole in E6: a lambda is passed only to library functions, and a parameter cannot be called (E9) | **Settled:** yes. It is what makes "every program terminates" true, which is the reason for D6 | Helpers that take a function, such as `const twice = (f, x) => f(f(x))`, cannot be written. Revised by D57: a parameter typed with an `Fn` may be called, so `const twice = (f: (x: int) => int, x: int): int => f(f(x))` can |
| D6 | No recursion or loops in 3.0 (E6) | **Settled: yes** | A v2 script that uses a loop, a statement or mutation has no direct v3 form. It is rewritten with `map`, `filter` and `reduce`, moved into a host function, or left on v2. This is the one break that C7 does not make cheap. Partly reversed by D54: self-recursion with types is in. Loops stay out. |
| D7 | JavaScript's names for string and list methods (`toUpperCase`, `some`, `every`), expr's for functions (`len`, `int`, `round`) | **Settled: yes** (and C6) | Longer names than expr's. The other spellings give a compile error that names the fix |
| D8 | `len`, `int` and `float` reject null | Yes | `len(x ?? [])` where expr accepts nil today (v3: null) |
| D9 | Regular expressions are out of 3.0 and reserved for a minor version | **Settled: yes** | Three engines would have to agree on one subset, the TypeScript implementation would need its own matcher of about 700 lines, and a released entry cannot change. D52 reversed it on 2026-11-14 and was withdrawn on 2026-11-19, so D9 stands. `regexp` is reserved in Core (PR #642). |

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
| D40 | Timing | **Settled:** the host's function, `as`, `is` and host-declared names in 3.0. `type` lines and annotations in a minor version. Moved forward by D53: annotations and `type` lines come in slice D1 of F16 (`17`), ahead of entry points. |
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

Hosts and agents. Issues #640 (Risor as the language of AI agent tools) and
#641 (a stateful GUI host, fynerisor) were reviewed on 2026-11-14 with two
independent reviews. The owner asked for the design to take in what the
language should have, not deferrals, and settled the direction of each plan
below. On 2026-11-19, after a further review and issue #644 (recursion and
functions as arguments), the owner withdrew D52, trimmed D54 to D56, and
settled D57 and D58. Each plan's own table marks its parts as settled, as
rulings to record, or as recommendations. `14` section 5 orders the work as
tracks A to F.

| # | Decision | Recommendation |
| --- | --- | --- |
| D52 | Regular expressions | **Withdrawn by the owner, 2026-11-19.** D9 stands. Only the reservation of the namespace `regexp` ships, in Core (PR #642). Plan `16` is not kept: three hand-written matchers would have to stay identical for a released entry, and `glob` (`21` 2.6) covers the common matching. The number F15 stays unused |
| D53 | Entry points, annotations, descriptions and schemas (`17-1` to `17-17`) | **Settled:** a host calls an `export const` lambda by name, as one bounded run with conformed arguments. Parameter and result annotations, `type` lines and the syntax of an `Fn` parameter, moving D40 forward. Doc comments are descriptions, kept through identity only. F1 gains `SchemaOf`, reversing R114; host docs reach hover, reversing R188. Kept on 2026-11-19 and built in three slices, D1 to D3 (`17`, `14` 5). An `export const` takes no `Fn` parameter. A ruling to record: beside an `export const`, a top-level `const` that binds no name is `bad-export` (17-15) |
| D54 | Recursion, recursive types, `walk` and `match` guards (`18-1` to `18-12`) | **Settled:** a `const`-bound lambda may call itself when its parameters and result are typed, partly reversing E6 and D6; no mutual recursion; R3 stands. The scope of a lambda's own name is a Core amendment that lands with annotations (`18` 3.1). Recursive named types, reversing R112. `walk`. Guards on `match` arms. The owner chose self-recursion over both reviews. **`iterate` dropped, 2026-11-19:** an unfold fails the readability test (`21` 1). Per-call frames are the reference path in every evaluator (`18` 8, a ruling to record) |
| D55 | Host calls and opaque values (`19-1` to `19-15`) | **Settled, and trimmed on 2026-11-19.** Kept: the sequence of host interactions is part of determinism (`19-1`); the host owns an opaque value's state, with a duty not to change memory a run has read (`19-11`); host opaque interfaces as a design track, with no source-level `interface` (`19-12`); S4's helpers in Go (`19-13`); a plain statement that integers past 2^53 − 1 enter as strings (`19-14`); a section for event-driven hosts, with the poisoned-run pattern (`19-15`). **Won't do:** the run code `suspended`, so Core stays at 19 run codes; replay and journals; `remaining()`, `runWithin` and `callWithin`; effect marks and `readOnly`; prefetch; `Batch`; `bigInts`. `19` 8 keeps the reasons. Lambdas still never reach a host function (S3) |
| D56 | Feedback to authors (`20-1` to `20-11`) | **Settled, and trimmed on 2026-11-19.** An absent key at the last step of a read gives null when the read is an operand of a `??` chain other than its last; a key a closed record cannot have stays `no-field`. Hints from tables in the message, the `wrong` lists made normative, a table of foreign names. Parse codes for `await` and `;` between expressions. A nested Risor error keeps its message. **Dropped:** the brief rendering and layouts for `time.format` |
| D57 | Passing functions to user-defined functions (#644a, `18` 7) | **Settled by the owner, 2026-11-19.** A parameter typed with an `Fn` may be called, and a lambda may be passed to it. First-order only: an `Fn` takes no function. Revises D34, E9 and `01-language.md` 8.2 rules 2 and 4. A function still cannot be stored, returned or passed to a host function: R3 and S3 hold. Static rules keep an unchecked run safe (`18` 7.1). `Fn` types have no type variable yet; `18` 7.6 recommends an additive follow-up. Specified as step E3 of F17 (`spec/packages/f17-recursion.md` 13) and built in all three implementations (R280 to R297) |
| D58 | The largest depth limit a host may set | **Settled by the owner, 2026-11-19:** 400, down from 1,000 (R20). The default stays 200. The fallback of 300 was not needed: after the stack work each binding runs its deepest forms at 400 with 1.5 times headroom, the TypeScript binding in a browser Web Worker among them (R214). With it goes a rule for `03-host.md` 3.9.1: each binding's stack test covers calls that a library entry makes of a lambda (`map`, `flatMap`), not only direct calls, and passes with 1.5 times headroom. The evidence is below |
| D59 | Scope: what v3 is (`21` 1) | **A recommendation the owner has not confirmed.** Write it into `AGENTS.md` and this file as a constraint: v3 is an embedded expression language; a file may hold several named entry points; nothing outlives a run (no closure kept by the host, no event loop, no mutation, no loops). The two tests of `21` 1 come with it: does a proposal make writing, checking or running an expression over host data better; and does a TypeScript programmer read it without an explanation |
| D60 | Keep the name "Risor v3", or rename v3 as expr's successor | **Open.** No recommendation. `21` 4 notes the case for a rename: if the Risor name keeps bringing requests for scripting |
| D61 | Comparisons do not chain | **Settled by the owner, 2026-11-24.** A comparison (`==`, `!=`, `<`, `<=`, `>`, `>=`, `in`, `not in`) is not an operand of another comparison without parentheses: `a == b == c`, `a < b < c` and `a < b == c` are the new Core `parse` code `chained-comparison`, at the second operator (`01-language.md` 3.4 rule 13). The reason is least surprise (C2): `1 == 1 == true` compared a bool with `true`, and `1 < 2 < 3` read like a range test and failed at run time. Mixing levels 5 and 6 is refused too, and so is `in`, which stands at level 6. `as` and `is` are not comparisons: `x as int > 3` stays valid. Core has 16 `parse` codes, F8 60 with 99 templates. The formatter writes its parentheses for the reader only in a run of at most 3 operators that no other such pair encloses, so they add at most 2 levels, and leaves them out of a source that nests deeper than 98 levels, so its output always parses (F12 2.6 items 3 and 4), which fixes #666. Existing Core cases that chained comparisons changed |
| D62 | How the formatter writes a broken `? :` chain (#665) | **Settled by the owner, 2026-11-24:** flat. A `? :` whose `else` branch is a `? :` is one chain. When it does not fit, each link starts a line at the chain's indentation: `test ? branch`, then `: test ? branch`, then `: branch`, as an `if` with `else if` parts stands. A link that does not fit breaks alone, with `? branch` on the next line one level in. Each `else` branch used to be indented two more than the one before, so the output grew with the square of the chain's length: 22 KB of source gave 8 MB. F12 2.8 item 5; R321 |
| D64 | Host data that shares a list or map makes the conformance function take exponential time (#678) | **Settled by the owner, 2026-11-24:** meter the walk. The conformance function and `ConformAll` charge the steps a run's walk charges (F4 5.3), against the default step limit, 1,000,000, or one the host gives through an option of each binding; past it they fail with `steps-exceeded`, class `limit`, no path. Shared values are not remembered and no identity of a host value is defined. The count is exact, so a conform case pins it with the field `limit` (F4 7.3, 9.2; F8 6.4). The environment's step limit does not apply: the function takes no environment (R322). A host that walks very large data through `Conform` must raise or turn off the limit |
| D65 | What the suite pins, and two rules relaxed to match | **Settled by the owner, 2026-11-24.** The suite pins what an author or a host sees: values, the first diagnostic's code and span, and run failures. It does not pin every combination of errors, and no implementation does extra work to agree on one nobody meets. Two rules are relaxed. (1) A declaration whose name is refused and whose type text is also bad: whether the text is parsed and its error reported is unspecified. `reserved-namespace` and `bad-name` are always reported; with `name-clash`, at least one of the two errors is (`03-host.md` 3.3.4 item 4). A `declare` case that expects a name error is compared with any one reported diagnostic; one that expects another code still needs exactly one (3.10.7 item 6). (2) A library entry whose result creates no new text and is no larger than a constant times its receiver, such as `groupBy`, may build its result before it charges the units, with the same charges, failure and callback order. An entry whose result can outgrow its input, such as `range` or `repeat`, still charges first (`02-evaluation.md` 6.2 item 3). Fixes #648 and #653 |
| D67 | A host call can read the run's remaining budget (#640) | **Settled by the owner, 2026-11-25.** The call handle gives `remaining()`: each of the step and allocation limits less what the run has charged, with the call's own step, or no count for a limit that is off, in the form each binding turns a limit off (`03-host.md` 3.5.2 items 7 to 9). It gives no time: the deadline and cancellation travel on the signal. A host function that runs another program runs it with what is left and charges back its usage, so a nested run cannot pass the caller's budget (`03-host.md` 3.2.5). Go also gets `risor.Budget`, since a zero field of `Limits` keeps the default. Host callbacks stay out. Rulings R391 to R395 |

**The evidence for D58** (measured 2026-11-19, `main` `aeaaf1fd`). Chains of
`const` lambdas, compiled without the type rules, at depth limit 1,000. Each
figure is the deepest limit that ran with no stack fault:

| Build | Direct call | Through `map` | Through `flatMap` |
| --- | --- | --- | --- |
| Go | 1,000 | 1,000 | 1,000 |
| Node 22, default stack | 1,000 | 643 | 604 |
| Chrome 157, main thread | 1,000 | 992 | 992 |
| Chrome 157, Web Worker | 1,000 | 435 | 497 |
| Rust release, 2 MiB thread | runs | aborts the process (about 2.7 MiB needed at 1,000) | aborts the process |

Rust debug needs about 5 MiB for a chain through `map` at depth 200. Safari
and Firefox were not measured. By these figures neither 400 nor 300 has 1.5
times headroom in a Web Worker today: that needs 600 or 450, and `map` ran to
435. So the TypeScript binding must spend fewer frames on a call through a
library entry before either number holds (inference; `18` 9).

**Where the plans meet.** These are fixed here, so that no plan needs the
others' text:

| Matter | Ruling |
| --- | --- |
| Package numbers | Not renumbered. F15 is not used: D52 is withdrawn, and its number is not given to another package, as F11 keeps its number with no chapter (R193). F16 entry points and annotations (`17`), built in three slices; F17 recursion, recursive types, function parameters, `walk` and guards (`18`); F18 host opaque interfaces (`19-12`), a design track; F19 author feedback (`20`). F16, F17 and F19 are offered after F14 and before F12, which stays last. The order as built is F3, F1, F2, F4, F5, F6, F7, F8, F9, F10, F11, F13, F14, F16 slice D1, F17, F12 (`03-host.md` 3.11 item 2). The planned order ends F16 slice D1, F17, F16 slice D2, F16 slice D3, F19, F12: the slices of F16 take places of their own in the order they are built, and F17 keeps one place (R261, `14` 5.3). F18 joins the order when its design is settled. The Go helpers of `19-13` are a convenience of the Go binding, with no package number |
| Core before 3.0 is final | The parts that pass test T1 of `14` 2.1 change Core, not a package. In track B: the `regexp` reservation (PR #642); absence under `??` (`20-1`, `20-2`), with its exception to `03-host.md` 3.4.8 item 2 (`20` 3.1), which stands (R260); the interactions in the determinism guarantee (`19-1`); the host's ownership of opaque state, with the aliasing duty and the note on F2 (`19-11`); the plain statement on big integers (`19-14`); hosts that live long (`19-15`); a nested Risor error's message (`20-9`); the largest depth of D58; and no change to the description of an environment, so that interfaces can come later as new keys (`19` 5.2). In track D1: the scope of a lambda's own name and `recursion-needs-types` (`18` 3.1), with annotations, so that no build asks for types it cannot parse. Core stays at 19 run codes |
| The diagnostic for a body that does not fit its annotated result | `result-mismatch` at the body, in `17` and `18` alike |
| A recursive type in `SchemaOf` | It is written once under `$defs`, by its name, and referred to with `$ref` (`17` 3.7) |
| An entry point and a function | An `export const` has no `Fn` parameter (`bad-export`, `17` 3.3): a host cannot pass a function (S3) |
| Frames | Per-call frames are the reference path in every evaluator, from step E1 (`18` 8) |
| Counts | The additions are listed below. The totals are fixed when each chapter is written. Two counts are already in doubt at `main`: `suite/messages-parse.json` holds 97 templates (ran), where F8 3.1 says 96, and track B corrects the chapter; and the package chapters each count library entries as if the others were absent |

| Count | Additions |
| --- | --- |
| Library entries | 1 (F17: `walk`, in the default group). The totals of section 3, 78 and 59, become 79 and 60 |
| Run codes and run templates | None: 19 codes, 64 templates |
| Check codes | `recursion-needs-types` (Core: 27 become 28), `bad-export` (F16) and `circular-type` (F17, step E2, R264). The catalog's 29 become 32 |
| Static codes (`05-types.md` 16.1, 22.1) | The first two: 11 become 13. 14 if `arg-mismatch` gains a static use (`18` 7.1 rule 3, a recommendation). `circular-type` is a static code of its package, as `name-clash` is of F16, and is not in Core's count |
| Check templates | 1 for `recursion-needs-types`, 4 for `bad-export` (F16), 5 in F19 (four hints and `maybe-absent` `pattern`): 71 become 81. F17 adds 3 for step E2 (`circular-type` `direct` and `union`, `check-limit` `pairs`) and 5 for step E3 (`spec/packages/f17-recursion.md` 13.5), and F16's `fn-value` retires. That row counts planned templates. As built with steps E2 and E3, the catalogs hold 81: 72 in Core's, 1 in F16's and 8 in F17's (`14` 5.3 rows E2 and E3) |
| Parse codes | `source-too-large` (D63, Core: 14 become 15), with 1 template: 58 become 59, and 97 templates 98. `chained-comparison` (D61, Core: 15 become 16), with 1 template: 59 become 60, and 98 templates 99. Then 2 (F19: `async-await`, `statement-sequence`), with 4 templates: 60 become 62, and 99 templates 103. `type-annotation`, `type-declaration` and `export-statement` narrow and change their text, by my reading with no change of count (`17` 3.1) |
| Declare codes | 2 (F16: `no-result`, `no-input-record`), beside the 9 of Core |
| Reserved namespaces | 1 (`regexp`) |
| The largest depth a host may set | 1,000 becomes 400 (D58): `03-host.md` 3.3.4, 3.6.1, 3.9.1 and the schema of 3.10.11 |
| Keys of an environment's description (F1 5.3) | 7, unchanged. 9 when F18 lands (`interfaces`, `implements`) |
| Reserved words, node kinds | None |

### 4.4 Limits

| # | Decision | Recommendation |
| --- | --- | --- |
| D15 | The unit of text size is the UTF-16 length in blocks of 64 (L7, R10) | Yes, but it is closer than it was. JavaScript has it for free. Go and Rust scan strings of 64 bytes or more: in Go, comparing two equal 4 KB strings goes from 46 ns to 483 ns. UTF-8 bytes would be free in Go and Rust and would make JavaScript measure every string of 22 units or more, at 20 ns and up. Short strings are the common case, and weight is the unit that makes them free everywhere |
| D16 | Step and unit counts are fixed per specification version (L6) | Yes. Optimizations that stop early are then forbidden, and a wrong rate stays until the next version |
| D17 | Defaults: 1,000,000 steps, 1,000,000 allocation units, depth 200, no deadline | Yes. A deadline is the one limit that is not deterministic |
| D31 | Source nesting is at most 100 levels, fixed by the specification and not settable by a host, and the depth limit counts a call as 1 plus its nesting (R5, `02` 2.16) | Yes. Without it a deep program is an error in one implementation, a crash in another and a success in the third. Settle the numbers after the Rust reference evaluator has run the limit cases in its least favorable build (`01`, question 7). D58 settles the largest depth a host may set: 400 |
| D63 | A limit on the size of a program's source text | **Settled by the owner, 2026-11-24.** A source text has at most 1,048,576 code points, counted as positions are, in the decoded text. A host may set a lower source limit, from 0, on each call that takes source text (parse, a template's parse, the formatter, completion, hover, the load); it cannot raise it. A longer text is the `parse` code `source-too-large`, with no span, found before the text is lexed. Completion and hover give their empty result. The flat forms of `01-language.md` 13.2 item 6 are cut from 100,000 to 10,000 so that they fit, and the time and memory of every tool are bounded through the size of the text. The source limit is not a declaration of the environment, so a description does not change (R320) |

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
the library 79 entries, each written three times and kept in step by the suite.
R9 is the fallback if that proves too much for the third checker.

### 5.3 What happens to v2 and expr

| Thing | Fate |
| --- | --- |
| Risor v2 | Kept in maintenance on its own branch. Its users are expected to move (C7), except for scripts with statements, loops or mutation, which have no v3 form (D6; recursion gains one with D54). v3 is a new module path |
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
| Three of everything is too much work | A checker and a 79-entry library, three times, for a project with few maintainers | The language is small by ruling; the suite and the library table are shared data; R9 as a fallback |
| A deep program crashes a Rust host | A stack overflow cannot be caught there | R5, R6, D31 and D58: every recursion has a specified bound, release is not recursive, and each binding's stack test covers calls through the library |
| The whole-number range hurts Go hosts | 64-bit ids and nanosecond times are common in Go data | The error is loud and at the read; the binding can present a field as a string. D3 should be confirmed knowing this |
| The cost table is wrong and frozen | A released rate cannot change within a specification version | Calibrate the rates on the real evaluator before 3.0; the accounting test bounds real time and memory per unit |
| The break is large for existing users | Division, method names and the loss of statements all change | The owner judges the user base small and willing (C7). Truthiness now matches v2 and expr. Errors name the fix. The loss of statements is the part with no mechanical answer (D6) |
| Three checkers to keep in step | The checker is the largest part to write three times, and truthiness adds union types to it | `03` puts it at about 2,935 lines with truthiness and type tests; the diagnostic catalog and conformance cases pin its behavior |
| The evidence is thinner than the documents sound | The allocation and count measurements are from hand-built trees, and the TypeScript and Rust notes are inference | Steps 3, 4 and 5 replace all of it with real measurements before much else is built |
