# Risor v3 design: host calls and opaque values

This plan answers the host side of two reports. #640 used Risor v3 as the
language for writing tools that AI agents call. #641 is from the maintainer of
fynerisor: Risor v2 bindings for a GUI toolkit, with about 850 host
signatures and scripts that live as long as their window. Both reports are
public, in `deepnoodle-ai/risor`.

It is decision D55. Its parts are numbered `19-1` to `19-15`. The owner
settled the direction on 2026-11-14, and trimmed it on 2026-11-19. Six parts
are kept: 19-1, 19-11, 19-12 (a design track), 19-13, 19-14 (a note only)
and 19-15. Every other part is dropped. Section 8 lists each with the reason,
so the reasoning is not lost. `00` section 4 has the D row, and `14` section
5 the tracks.

**Where it lands.**

| Part | Where | Track (`14` 5) |
| --- | --- | --- |
| 19-1, 19-11, 19-14, 19-15 | Core amendments. Each restates or documents Core's contract; none adds a code, a node or a value | B |
| 19-13 | The Go binding only, as conveniences for the host's own types (R201). No specification text, no suite case, no package number | C, beside the others |
| 19-12 | A design track. Package number F18 is kept for it. It is built only when its design is settled | F |

Related plans: `17-entry-points.md` (D53), `18-recursion.md` (D54) and
`20-author-feedback.md` (D56). Entry points (`17`) are what an event-driven
host calls (19-15).

## 1. Summary

| # | Change | Where it lands |
| --- | --- | --- |
| 19-1 | Determinism covers the **sequence of host interactions**: each variable request and each host call, with its arguments, its charges and its result | `02-evaluation.md` 9; `03-host.md` 3.6.2, 3.9.1 |
| 19-10 | Lambdas still never reach a host function. Kept, not changed | S3, D34, `01-language.md` 8.2 rule 3 |
| 19-11 | **An opaque value's state belongs to the host.** A method may change it, and a host may keep it across runs. Two duties come with it | `03-host.md` 3.4.6, 3.5.4; F2 3.6 |
| 19-12 | **Host opaque interfaces**, declared by name, with `implements`. Design only in this round | `03-host.md` 3.3.1; `05-types.md`; F1 5.3 |
| 19-13 | The Go binding builds **S4**: `risor.Func`, `risor.Methods`, and optionally a generator | `05-host-api.md` Part E; Go only |
| 19-14 | Integers past 2^53 − 1 stay out, and the specification says so plainly | `03-host.md` 3.4.4 |
| 19-15 | An informative section for **event-driven hosts**, with the "poisoned run" pattern | `03-host.md`, new 3.2.5 |

Nothing here adds syntax, a node kind, a value, a reserved word or a run
code. Core stays at 19 run codes.

## 2. The problem

### 2.1 What a host can rely on (#640, "Two properties a host can build on")

The experiment journaled each host call's arguments and result. A tool asked
the user for approval through a host function `approve`. The function
returned a sentinel error. The host stopped the run, kept the journal, and on
a later run replayed it. `issues.list` ran once across both runs.

This works today by accident of three facts. The spec does not promise them
together:

| Fact | What the spec says now |
| --- | --- |
| The order of host calls is fixed | Implied by `02-evaluation.md` 2 and 10.1 item 3. Not stated as a property a host can use |
| The same calls give the same run | `02-evaluation.md` 9.1 says "the same results and charges of host functions". The experiment replayed results only, not charges. A host function that charges would make a replayed run drift (inference) |
| A sentinel stops the run | It does not. The sentinel is a host error, so it is `function-error`, which `catch` catches (`03-host.md` 3.5.3). `try { approve(m) } catch { false }` swallows the pause and goes on as if the user declined |

#640 question 3 asks whether determinism is stated strongly enough for this.
It is not. 19-1 states it. Replay itself stays the host's own work: the
language gives no journal, no replay key and no pause code (section 8). A
host that must stop a run at once uses the pattern of 7.1 item 7.

### 2.2 The GUI host (#641)

| Question | The state today | After this plan |
| --- | --- | --- |
| 1. Is the pattern host-owned state, a run per event, actions as data? | The owner said yes on the issue. The spec has no example | 19-15 writes it down |
| 2. May an opaque method change its host object? May a host keep an opaque value across runs? | Read literally, no. `03-host.md` 3.5.1 item 4 makes the receiver an argument, and 3.5.4 row 1 says to treat the arguments as read-only and not keep them | Yes (19-11) |
| 3. Can several opaque types satisfy one parameter type? | Half. The opaque part of a type is a set of names (`05-types.md` 2.3), so `Button \| Label` is a parameter type. But a method call on it is `kind`: 9.1 step 4 needs one kind, and each opaque type is a kind of its own | Designed in 19-12, not built in this round |
| 4. Is S4 still planned? | Ruled (overview S4, `05-host-api.md` B.4) and not built. `go/env.go` has only `Function{Name, Type, Impl}`, with the signature written by hand | Yes, Go only (19-13) |
| 5. Integers above 2^53? | Strings, by `03-host.md` 3.4.4 and D3. Stated only in passing | Stated plainly (19-14) |

## 3. Determinism (19-1)

A run's **interactions** are, in the order the run makes them:

| Interaction | Holds |
| --- | --- |
| A read | The name of a variable the run asks the host for (`03-host.md` 3.4.1) |
| A call | The host function's name, with its namespace, or `Type.method` for a method; the argument values, the receiver first for a method; each `charge` and `allocate` the function made, in order; and its outcome: a value, an error with its text, or a crash |

**The rule.** Given the same program, version, environment description,
limits and variable values, the interactions of a run are fixed by the
outcomes and charges of the calls before them. Two implementations make the
same reads and the same calls, with `==` arguments, in the same order, up to
the first that differs in outcome or charge.

| Holds | Does not hold |
| --- | --- |
| For every run, up to the first failure of class `limit` | Past a deadline or a cancellation. Their position is not fixed (`02-evaluation.md` 9.3) |
| With the optimizations of `02-evaluation.md` 10.2 | — |

`02-evaluation.md` 10.1 item 2 already compares usage at every host call, and
item 3 forbids a call the reference would not make. So the rule adds no work
to a conforming implementation (inference). It makes a promise of what was a
consequence, so a host may build on it: a journal of its own calls, replayed
with their charges, gives the same run.

## 4. Why lambdas still do not reach host functions (19-10)

#640 question 2 asks for `retry(() => ...)`, `parallel([...])` and
`withTimeout(...)`. #644 asks for a function passed to a host function. They
stay out. S3, D34 and `01-language.md` 8.2 rule 3 are kept. D57 lets a
function reach a user-defined lambda, and not a host function.

| Reason | Detail |
| --- | --- |
| The checker's sites | A host function that runs a lambda and catches its failure is a `try` the checker cannot see. A failure in the lambda would then not leave the run, and the error sites of `05-types.md` 18.3 and 18.4 would no longer describe what passes out of a call |
| Synchronous TypeScript | A TypeScript host function cannot wait for anything (D18). `parallel` over lambdas cannot be built there, and a feature in two implementations of three breaks C1 |
| Exact limits | Two lambdas of one run evaluated at once would charge in an order the program does not fix, so a limit would be reached at a different point in each run |
| What `retry` is for | Retrying a network call is the host function's own policy, done inside the call with the call handle's cancellation signal. Retrying Risor code that has no effect gives the same result |
| What `parallel` is for | Latency of host calls. A host function that takes a list, `weather.currentAll(cities)`, may do its own work at once inside one call. The run sees one call, so 19-1 holds |
| Long-lived callbacks | A GUI callback outlives its run, which S3 forbids for any reason above. The answer is 19-15: the host names a handler and calls it per event |

## 5. Opaque values

### 5.1 The host owns an opaque value's state (19-11)

Row 1 of `03-host.md` 3.5.4 becomes:

| Duty | Effect when it is not met |
| --- | --- |
| Treat the arguments that are language values (null, bool, number, string, list, map) as read-only, and do not keep them after the call returns | Values shared between runs may change. Nothing checks this duty |

And a new item in `03-host.md` 3.4.6:

> An opaque value stands for host state. A method or host function may change
> that state. A host may keep an opaque value, from a variable, an argument or
> a result, across runs. The run never changes it (3.4.2 item 3); the host
> does, through its own code.

**The aliasing duty.** A new row of `03-host.md` 3.5.4:

| Duty | Effect when it is not met |
| --- | --- |
| Do not change, until the run ends, host memory that the run has already read in place as a language value, or received as a result | A `const` changes under the program. `const xs = w.items(); const _ = w.add(1); xs` must give the list as `w.items()` returned it |

19-11 lets a method change its object. Without this duty, a result that
shares memory with that object (`03-host.md` 3.9.1: "A result may share
memory with host data") would change with it.

**Analysis and caching.** F2 3.6 lists calls of host functions, and its item
6 says the list is complete for them. A method of an opaque value is a host
call too, and F2 does not list it as one: a `MethodCall` on a free `Ident` is
reported only as a possible call of a namespace, which the host drops, and
one on any other receiver is not reported. F2 3.6 gains a note: a host that
caches results on F2's report treats a program that reads an opaque value as
volatile.

| Concern | Why it holds |
| --- | --- |
| Determinism | A method that reads changed state gives a host result, which 19-1 already counts as an input |
| Two runs share one object | The host's matter, as with any host state |
| Caching | A host call is never cached (`03-host.md` 3.5.2 item 5), so `const a = w.text(); const _ = w.setText("x"); w.text()` reads twice |
| The checker | Unchanged. It reasons about types; an opaque type has no fields to go stale |

### 5.2 Host opaque interfaces (19-12), a design track

Not built in this round. This section is the design to advance, with the
fynerisor scene prototype of #641 as its test. Package number F18 is kept for
it.

**Scope.** An interface is declared by the host, for opaque types only.
There is no source-level `interface`: a program writes `type Name = {...}`
(`17`) for exact structural records. A program that writes `interface` gets
a hint that names `type` (`20` 3.3).

A new declaration kind, **opaque interface**:

| Declaration | States | Portable form |
| --- | --- | --- |
| Opaque interface | A name, a list of methods, and the interfaces it extends | The name. Each method is a name and a signature in type text. The extended interfaces by name |
| Opaque type (changed) | Gains `implements`: a list of interface names | The names |

**The description of an environment** (F1 5.3) gains two top-level keys,
`interfaces` and `implements`, when this lands. Today's flat method map on
each `opaque` entry is kept as it is. Separate keys do not collide with a
method named `implements`, and do not change what current consumers read.
The count of keys in F1 5.3 then goes from 7 to 9.

**Rules at environment build** (`03-host.md` 3.3.4):

1. An interface name is a `Name` (`05-types.md` 3.1). It shares the set of
   names with opaque types and F4's named types. A name declared twice is
   `duplicate-name`.
2. `implements` and `extends` name declared interfaces. The graph they form
   has no cycle. A cycle, or an undeclared name, is `bad-signature`.
3. An opaque type that implements `I`, directly or through extension, has
   each of `I`'s methods. It declares the method with the same printed
   signature (F1 5.2), or it takes an implementation the interface gives.
   A missing method, or one with another signature, is `bad-signature`.
4. Two interfaces that a type implements and that declare one method name with
   different signatures are `bad-signature`.

**Types** (`05-types.md`):

| Matter | Rule |
| --- | --- |
| Type text | An interface name is a type, as an opaque name is (2.3, 3.2) |
| Values | No value carries an interface name. A value carries its opaque type's name (`03-host.md` 3.4.6 item 3) |
| Fits | Opaque `N` fits interface `I` when `N` implements `I`, directly or through extension. Interface `J` fits `I` when `J` extends `I`. `I` fits no opaque type |
| Join and printing | In a set of names, a name that fits another name of the set is dropped. So `Button \| CanvasObject` prints as `CanvasObject` |
| Method lookup (9.1 step 4) | A receiver whose type has only opaque alternatives is one kind for this step. The method is found when every name in the set has a method of that name with the same printed signature. The interface's methods count for an interface name |
| `is` and `as` (F4) | `v is I` is true when `v`'s opaque type implements `I` |
| Run | A method is selected by the receiver's own opaque type, as now (R55) |

So fynerisor declares `CanvasObject` once, with its 30 methods, and 84 widget
types that implement it. A container takes `children: CanvasObject[]`, and
`child.refresh()` checks.

**The strongest alternative**, from the Opus reviewer: no new declaration. A
method call resolves on any union of opaque types that all declare the method
with the same signature, and an F4 named type abbreviates the union. It needs
no new kind. It was not chosen: every new widget type must be added to the
union by hand, the 30 shared methods are declared 84 times, and a message
names an 84-way union where it could name `CanvasObject`. The method-lookup
row above still accepts that case, so the alternative is a subset of this
design.

## 6. The Go binding and big integers

### 6.1 S4, built (19-13)

S4 says registration accepts any Go function. R201 allows conveniences for the
host's own types. These are added to the Go root. They live in `go/` only:
no specification text, no suite case, nothing in TypeScript or Rust.

| Helper | Gives |
| --- | --- |
| `risor.Func(name string, fn any, opts ...FuncOption) Function` | A `Function` whose signature is derived from `fn`'s Go type |
| `risor.Methods[T](name string, opts ...MethodOption) OpaqueType` | An opaque type from `T`'s exported method set |
| `cmd/risor-bind` (`go generate`), an optional follow-up | Go source that declares the same `Function` and `OpaqueType` values, with adapters that use no reflection |

**Deriving a signature:**

| Go | Signature |
| --- | --- |
| A first parameter `*risor.Call` or `context.Context` | Not a parameter. It receives the call handle, or its context |
| Each other parameter | A parameter, its type by F1 6.2 |
| A variadic `...T` | `bad-type` |
| Results `()`, `(R)` | `=> null`, `=> R` |
| Results `(error)`, `(R, error)` | `=> null !`, `=> R !` |
| Any other results | `bad-type` |
| A Go interface type, such as `fyne.CanvasObject`, as a parameter or result | `bad-type`, unless the host registered it. This keeps 19-12 additive |
| Parameter names | `risor.Func`: from `opts` (`risor.Params("city", "units")`), else `a1`, `a2` and so on, because reflection has no names. These names appear in `Describe`, so a host that wants stable names passes `risor.Params` or uses the generator. The generator: the names in the source |
| Method names | The Go name with its leading initialism or first letter in lower case: `SetText` gives `setText`, `URL` gives `url`, `ID` gives `id`, `HTTPStatus` gives `httpStatus`. A method tag in the options renames |
| Two Go methods that give one name | `bad-signature` |

1. `risor.Func` uses an adapter with no reflection for a fixed set of common
   shapes, and reflection for the rest. `Function.Reflective()` reports which.
   This is the S4 ruling as written.
2. The generator reads the package with `go/types`. It writes each signature as
   text, each parameter's name from the source, and each doc comment as the
   function's documentation (`17` uses it as a description). Its output has
   no reflection and is ordinary code a host can read and edit.
3. The written signature is the portable form, as for any `Function` (S5).
4. TypeScript and Rust do not get these helpers. Rust already derives types
   with `#[derive(risor::Host)]` (F1 6.3); a function macro is its own matter.

**Stale text.** `05-host-api.md` B.4 to B.7 still describe `Impl any`,
`Rule`, `Opaque[T]` and `Fields[T]`. Part E replaced them (R201). #641 read B.4
as the current plan. Each of those sections has a note at its top: superseded
by Part E, with this plan for the S4 helpers.

### 6.2 Integers past 2^53 − 1 (19-14)

**The rule stays.** A number is a double. A whole number has a magnitude of at
most 2^53 − 1 (`01-language.md` 6.2, N2, D3). A host integer outside it cannot
enter (`03-host.md` 3.4.4). An int64 SQL id or a nanosecond timestamp is
passed as a string. The reason is C1 and C6: every implementation, TypeScript
among them, has the same numbers.

`03-host.md` 3.4.4 states it in passing. It gains a first paragraph that says
it plainly, with the two examples above. Nothing else changes: the option
`bigInts` of `json.parse` and the `,string` rows of F1 are dropped (section
8).

**Why `9007199254740993.0` reads and `9007199254740993` fails.** This is
already specified (F6 6.2.3 items 2 and 3) and is kept. A number in integer
form says the writer meant an exact integer. Rounding it would change an id
silently, so it fails. A number with a fraction or an exponent says the writer
meant a floating value, and the nearest double is its meaning.

## 7. Decisions

Settled means settled by the owner, on 2026-11-14 or 2026-11-19. A
recommendation is a detail inside a settled direction that was not put to
the owner.

| # | Decision | State | Strongest alternative |
| --- | --- | --- | --- |
| 19-1 | Determinism covers reads and calls, with charges and outcomes | **Settled** | Leave it implied by 10.1. A host could not rely on it |
| 19-10 | No lambda reaches a host function | **Settled** (kept; S3 holds) | Synchronous callbacks through `Call.Invoke`. Rejected for the reasons of 4 |
| 19-11 | An opaque value's state belongs to the host, with the aliasing duty and the note on F2 | **Settled** | — |
| 19-12 | Host nominal opaque interfaces with `implements` and `extends`; no source-level `interface` | **Settled** as a design track; the design of 5.2 is a recommendation | The structural rule of 5.2 alone |
| 19-13 | `risor.Func`, `risor.Methods`, and `cmd/risor-bind` as a follow-up, with the rules of 6.1 | **Settled** | Amend S4 and keep hand-written signatures |
| 19-14 | The number rule stays, stated plainly | **Settled** | A `bigint` kind. It would change every number rule in three implementations |
| 19-15 | An informative section for event-driven hosts | **Settled** | — |

### 7.1 The event-driven section (19-15)

A new informative section, `03-host.md` 3.2.5, **Hosts that live long**:

1. The host owns all state. A program, or an entry point (`17`), holds none
   between runs.
2. Each event is one run, or one call of an entry point, with the state and
   the event as arguments: `onTap(state: State, ev: Tap)`.
3. The result is data: the next state, and a list of actions. A tagged union
   (F5) types the actions: `{kind: "openDialog", title: string} | {kind:
   "setText", widget: Label, text: string}`.
4. The host applies the actions, through its own code. Opaque handles in an
   action are widgets the host gave the program (5.1).
5. Limits apply to each event, so a slow handler cannot freeze the host.
6. The compiled program is shared by every event (3.2.2), so a run per event
   costs no compile (measured in #640: 6.0 µs a call for a small tool, against
   62.8 µs for the compile; ran: Go 1.26.0, darwin/arm64).
7. **The poisoned run.** A host that must stop a run at once, without waiting
   for the next cancellation check, keeps a flag for the run. A host function
   that must stop it sets the flag and returns an error. Every later host call
   of that run returns an error at once, and the host discards the run's
   outcome, whatever it is, when the flag is set. A `catch` may catch the first
   error, but nothing the program does after it reaches the host. The host may
   also cancel the run, which then stops within one test interval (`03-host.md`
   3.6.1). This needs no run code.

## 8. Dropped on 2026-11-19, and why

Each was settled or recommended on 2026-11-14. None is in 3.0. Each could be
added later without a break unless the row says otherwise.

| Part | What it was | Why it is dropped |
| --- | --- | --- |
| 19-2 | Case fields `calls` and `reads` to test the sequence of 19-1 | Their main use was replay. 19-1 is a promise each binding tests in its own tests. A suite field can be added later if the suite must test 19-1 directly |
| 19-3 | A replay identity: SHA-256 of source and description, limits, entry, and a host key | Replay is the host's own work. A host that journals knows its own key |
| 19-4 | A run code `suspended` that `catch` never catches | Core stays at 19 run codes. A run code cannot be added after 3.0 (`03-host.md` 3.7.4 item 4), so this is the one item here that cannot come back later. A host that must stop a run uses the poisoned run of 7.1 item 7 |
| 19-5 | Journals record charges, and opaque values by host ids | Replay is the host's own work. 19-1 tells a host that a journal must replay the charges |
| 19-6 | `remaining()` on the call handle; `runWithin` and `callWithin`, runs nested in the caller's budget | A nested run is a host's own run. A host that calls a tool from a tool charges the caller with `Call.Charge`, after the fact (#640 measured the gap). Within one file, entry points call each other in one run and budget (`17` 4.1) |
| 19-7 | Effect marks `read` and `write`; `readOnly` in F2 | No language rule reads them once prefetch is gone. A host keeps its own list of read-only functions, as #640 did |
| 19-8 | Prefetch of `read` calls inside per-element entries | The one part whose bug would break determinism silently, and the TypeScript binding could not build it (D18). A host function over a list (19-10) covers the latency case |
| 19-9 | `Batch`, a helper that makes a host function over a list | A binding helper with no language rule. A host writes it in a few lines |
| 19-14, in part | `json.parse` with `{bigInts: "string"}`; F1 rows for an integer field presented as text (`json:",string"`, `#[risor(string)]`, `bigint`) | Only the plain statement is kept. Both are additive if a host asks |

## 9. What this changes in other documents

| Document | Section | Change |
| --- | --- | --- |
| `spec/core/02-evaluation.md` | 9 | Item 1 covers interactions (3). New item: the interactions are fixed as 3 says |
| `spec/core/03-host.md` | 3.2.5 (new) | Hosts that live long (7.1). Informative |
| | 3.4.4 | A plain first paragraph on big integers |
| | 3.4.6 | Item 6: an opaque value's state belongs to the host (5.1) |
| | 3.5.4 | Row 1 scoped to language values. A row for the aliasing duty |
| | 3.6.2 | The sequence of interactions |
| | 3.9.1 | The determinism row names the interactions |
| `spec/packages/f02-templates-analysis.md` | 3.6 | The note on methods of opaque values (5.1) |
| `spec/packages/f01-host-types.md` | 5.3 | When 19-12 lands: keys `interfaces` and `implements`, 7 keys become 9 |
| `spec/core/05-types.md`, `03-host.md` 3.3.1, 3.3.4 | — | When 19-12 lands: the rules of 5.2 |
| `experiments/v3/design/05-host-api.md` | B.4 to B.7 | Superseded notes (6.1) |
| | Part E | `Func`, `Methods` and `Reflective` in the Go column, as conveniences of E.1 row 5 |

## 10. Cost

Estimates, by inference from the size of comparable parts. Not measured.

| Part | Go | TypeScript | Rust |
| --- | --- | --- | --- |
| 19-1, 19-11, 19-14, 19-15 | Specification text and cases where a case can show it | Same | Same |
| `Func`, `Methods` | About 400 lines; about 600 more for the generator | — | — |
| Opaque interfaces, when built | Checker: about 200 lines in Fits, Join and method lookup. Environment build: about 100 | Same | Same |

## 11. What I did not verify

- That the method-lookup change of 5.2 keeps the checker within its bounds of
  `05-types.md` 20. Inference: the lookup is one pass over a set of at most
  the declared names.
- That `go/types` gives parameter names and doc comments for every shape the
  generator meets. Inference: it does for declared functions and methods.
- Whether fynerisor's 84 widget types fit one interface, or need several.
  From #641 only.
- That the poisoned run of 7.1 item 7 is enough for #640's approval flow.
  Inference: the host discards the run and resumes by its own replay.
