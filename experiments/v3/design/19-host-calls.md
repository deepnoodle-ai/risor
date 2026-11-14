# Risor v3 design: host calls, replay and opaque values

This plan answers the host side of two reports. #640 used Risor v3 as the
language for writing tools that AI agents call. #641 is from the maintainer of
fynerisor: Risor v2 bindings for a GUI toolkit, with about 850 host
signatures and scripts that live as long as their window. Both reports are
public, in `deepnoodle-ai/risor`.

It is decision D55. Its parts are numbered `19-1` to `19-15`. The owner
settled the direction on 2026-11-14. Section 7 marks each part as settled or as
a recommendation.

**Where it lands.** 19-1, 19-4 and 19-11, with the plain statement of 19-14 in
`03-host.md` 3.4.4, are Core amendments: a run code cannot be added later, and
the others restate Core's contract. The rest is package F18. `00` section 4,
"Where the five plans meet", fixes the numbers and the order.

Related plans written at the same time: `16-regular-expressions.md` (D52),
`17-entry-points.md` (D53), `18-recursion.md` (D54) and
`20-author-feedback.md` (D56). Entry points (`17`) are the mechanism that
section 6 builds on.

## 1. Summary

| # | Change | Where it lands |
| --- | --- | --- |
| 1 | Determinism covers the **sequence of host interactions**: each variable request and each host call, with its arguments, its charges and its result | `02-evaluation.md` 9; `03-host.md` 3.6.2, 3.9.1 |
| 2 | Two case fields, `calls` and `reads`, test that sequence | `03-host.md` 3.10.6 |
| 3 | A **replay identity**: the key under which a host may journal a run | `03-host.md`, new 3.6.4 |
| 4 | A run code **`suspended`**, which `catch` never catches, so a host function can pause a run and the host can resume it by replay | `03-host.md` 3.7.4; `02-evaluation.md` 12.2; `05-types.md` 18.5 |
| 5 | **Nested runs** share the caller's budget: `remaining()` on the call handle, and `runWithin` | `03-host.md` 3.5.2; `02-evaluation.md` 8 |
| 6 | **Effect marks**, `read` and `write`, on host functions and methods. A binding may prefetch `read` calls. A helper `Batch` | `03-host.md` 3.3.1, 3.5; `02-evaluation.md` 10.1; F1 5.3; F2 3.6 |
| 7 | Lambdas still never reach a host function | Kept: S3, D34, `01-language.md` 8.2 rule 3 |
| 8 | **An opaque value's state belongs to the host.** A method may change it, and a host may keep it across runs | `03-host.md` 3.4.6, 3.5.4 |
| 9 | **Opaque interfaces**, declared by name, with `implements` | `03-host.md` 3.3.1, 3.4.6; `05-types.md` 2.3, 4, 9.1, 13.3; F1 5.3 |
| 10 | The Go binding builds **S4**: `risor.Func`, `risor.Methods`, and a generator | `05-host-api.md` Part E; Go only |
| 11 | Integers past 2^53 − 1 stay out. `json.parse` gains `{bigInts: "string"}`. F1 gains a `,string` row | F6 6.2; F1 6.2 to 6.4 |
| 12 | A section for **event-driven hosts** | `03-host.md`, new 3.2.5 (informative) |

Nothing here adds syntax, a node kind, a value or a reserved word. It adds one
run code and one declaration kind.

## 2. The problem

### 2.1 Durable runs (#640, "Two properties a host can build on")

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
It is not.

### 2.2 Nested runs (#640, "Nested tool calls escape the caller's budget")

A tool that calls another tool through a host function starts a second run.
That run has a full budget of its own. Measured in the experiment (ran: Go
1.26.0, `main` at `16cd6f8d`), with 20,000 steps for each run:
`outer({calls: 40})` succeeded and reported 205 steps while about 180,000 were
done. Billing after the call with `Call.Charge` closes most of the gap, but
the callee may still spend its whole budget before the caller learns of it. A
tool that calls itself through the host passes the depth limit unseen.

### 2.3 Calls one at a time (#640, friction 6 and question 2)

`cities.map(c => weather.current(c))` makes its calls in sequence, so the
latency adds up. The report asks for `parallel`, `retry` and `withTimeout`,
which need a lambda passed to a host function.

### 2.4 Read-only from the source (#640)

The experiment derived `readOnly` from `Analyze().Calls` with a list kept by
hand. Nothing in an environment says which functions are read-only.

### 2.5 The GUI host (#641)

| Question | The state today |
| --- | --- |
| 1. Is the pattern host-owned state, a run per event, actions as data? | The owner said yes on the issue. The spec has no example |
| 2. May an opaque method change its host object? May a host keep an opaque value across runs? | Read literally, no. `03-host.md` 3.5.1 item 4 makes the receiver an argument, and 3.5.4 row 1 says to treat the arguments as read-only and not keep them |
| 3. Can several opaque types satisfy one parameter type? | Half. The opaque part of a type is a set of names (`05-types.md` 2.3), so `Button \| Label` is a parameter type. But a method call on it is `kind`: 9.1 step 4 needs one kind, and each opaque type is a kind of its own (`05-types.md` 70) |
| 4. Is S4 still planned? | Ruled (overview S4, `05-host-api.md` B.4) and not built. `go/env.go` has only `Function{Name, Type, Impl}`, with the signature written by hand |
| 5. Integers above 2^53? | Strings, by `03-host.md` 3.4.4 and D3. Stated only in passing. `json.parse` fails the whole text on one large id |

## 3. Determinism and replay

### 3.1 The sequence of host interactions (19-1)

A run's **interactions** are, in the order the run makes them:

| Interaction | Holds |
| --- | --- |
| A read | The name of a variable the run asks the host for (`03-host.md` 3.4.1) |
| A call | The host function's name, with its namespace, or `Type.method` for a method; the argument values, the receiver first for a method; each `charge` and `allocate` the function made, in order; and its outcome: a value, an error with its text, a crash, or a suspension |

**The rule.** Given the same program, version, environment description,
limits and variable values, the interactions of a run are fixed by the
outcomes and charges of the calls before them. Two implementations make the
same reads and the same calls, with `==` arguments, in the same order, up to
the first that differs in outcome or charge.

| Holds | Does not hold |
| --- | --- |
| For every run, up to the first failure of class `limit` | Past a deadline or a cancellation. Their position is not fixed (`02-evaluation.md` 9.3) |
| With the optimizations of `02-evaluation.md` 10.2 | With the prefetch of 4.3, the binding makes extra `read` calls. They are not interactions: the run never sees them, and they are never journaled |

`02-evaluation.md` 10.1 item 2 already compares usage at every host call, and
item 3 forbids a call the reference would not make. So the rule adds no work
to a conforming implementation (inference). It makes a promise of what was a
consequence, so a host may build on it.

### 3.2 Testing it: `calls` and `reads` (19-2)

Two optional fields of a result or of a `run` failure in `expect`
(`03-host.md` 3.10.6):

| Field | Compared with | How |
| --- | --- | --- |
| `calls` | The calls the run made, each an object `{"fn": name, "args": [...]}` | The whole list, in order. Each `args` element with the language's `==`. `fn` is equal |
| `reads` | The names the run asked for | The whole list, in order |

1. Neither field is allowed with a diagnostic of a limit code.
2. A stub's row already gives the outcome and the charges (3.10.4), so `calls`
   need not repeat them.
3. `reads` makes `03-host.md` 3.4.1 item 2 testable: a name on a branch not
   taken is never asked for, and a name read twice is asked for once.

### 3.3 The replay identity (19-3)

A journal is only good for the run it came from. The **replay identity** of a
run is:

| Part | Form |
| --- | --- |
| Specification version | `major.minor` (`03-host.md` 3.9.2) |
| Source | SHA-256 of the program text, as UTF-8 |
| Environment | SHA-256 of the text of its description (F1 5.3) |
| Limits | Steps, allocation and depth, as in the description |
| Entry | For a call of an entry point (`17`): its name. Absent for a whole program |
| Invocation | A key the host gives each run: the same key on a replay of that run, a new one for any other run |

The first five parts name the program and its setting. They do not name its
inputs: two calls of one entry with different arguments, or two runs with
different variables, have the same first five parts. The invocation key
tells them apart. The host chooses it, because arguments and variables can
hold opaque values that have no text to hash. A host that keeps its inputs
with the journal may also compare them on replay.

The hash is named so that two implementations agree on the key. It is not a
security claim. A host without F1 may use any key that changes when the
environment does.

A host journals, for each call: the name, the arguments, the charges in order,
and the outcome. It replays a call when the name and arguments match the next
entry: it makes the same charges through the call handle, then gives the same
outcome. A mismatch is the host's own error; the run did not change, so the
journal or the identity is wrong.

**Journals record charges (19-5).** By 3.1, a replayed call that charges less
than the original moves every later step count. The run can then reach a limit
at a different point. So a replayed call must make the recorded charges.

**Opaque values in a journal (19-5).** An opaque value's identity
(`03-host.md` 3.4.6 item 4) cannot be written down. A host that journals a
call whose arguments or result hold an opaque value journals its own id for
that value, and maps it back on replay. This is the host's matter. The case
format already does the same with `$opaque` ids (3.10.3 item 7).

### 3.4 Suspension (19-4)

A new run code:

| Code | Raised when | Class | `catch` |
| --- | --- | --- | --- |
| `suspended` | A host function or method returns the binding's suspension sentinel | `host` | Passes |

1. Each binding offers one sentinel: `risor.Suspend` (Go), a `Suspend` error
   class (TypeScript), `Error::Suspend` (Rust). A host function returns it as
   it returns any error.
2. The run fails with `suspended` at the span of the call. Its message is
   from the catalog: `the run was suspended by {name}`. It has no `data`.
3. `catch` does not catch it. It passes every `try` in progress, as
   `function-panic` does (`02-evaluation.md` 12.2).
4. **Its usage is exact.** Unlike a limit code, a suspension happens at a
   fixed interaction, so the steps and units are fixed (`02-evaluation.md`
   8.6 item 2 covers it with no change).
5. It can happen at any host call, marked `!` or not. The class `host` already
   has that reach: `05-types.md` 18.4 promise 3 says a crash can occur at any
   host call. Add `suspended` to that sentence.
6. The call that suspended is not journaled with an outcome. On resume it is
   called again, live. That is how the host asks again ("is it approved
   yet?").

**How a host resumes.** It runs the same program, with the same replay
identity, and replays the journal call by call. When the journal ends, calls
are live again. By 3.1 the replayed run reaches the call that suspended with
the same usage.

**Why now.** `03-host.md` 3.7.4 item 4 says no run code is added in a later
minor version. 3.0 is an alpha, so this is the last time a code can join the
list. The list goes from 19 codes to 20, and the codes that pass `catch` from
8 to 9.

## 4. Nested runs, effects and concurrency

### 4.1 `remaining()` and `runWithin` (19-6)

**The call handle gains a fourth thing** (`03-host.md` 3.5.2 item 3):

| Part | Gives |
| --- | --- |
| `remaining()` | The steps and units left in the run's budgets (null for a limit that is off), and the depth left: the depth limit minus the current depth count. It charges nothing |

The values are exact at the moment of the call, so `remaining()` is
deterministic (3.1). It is valid only while the call is in progress, like the
rest of the handle (R95).

**A nested run.** Every binding gives `runWithin(call, program, vars)`, and
`callWithin(call, program, entry, args)` for an entry point (`17`).

| Matter | Rule |
| --- | --- |
| Budget | The child charges the parent's step and unit counters directly. Its limits are, for each, the smaller of the parent's remaining amount and the child's own environment's limit |
| Depth | The child counts its depth from 0, as a run does. Its depth limit is the smaller of its environment's limit and the parent's remaining depth from `remaining()`, which already counts the host call in progress. So a parent at depth 6 of 10 gives a child a limit of 4, measured from the child's own 0 |
| Deadline and cancellation | Shared with the parent. A cancelled parent cancels the child |
| A child limit | When the child passes a limit of its own environment, it fails with that code and the parent goes on. The host function sees the failure |
| A parent limit | When the child passes the parent's remaining amount, the child fails with that code, and the parent run fails with the same code as soon as the host function returns, whatever it returns |
| Usage | The child reports its own usage. That usage is also in the parent's, because it was charged there |
| Interactions | The child's interactions are its own (3.1). They are not interactions of the parent |

So a tool that calls itself through the host stops at the parent's depth
limit, and 40 calls of `inner` stop at the parent's step limit, at the same
point in every implementation.

### 4.2 Effect marks (19-7)

A host function or method is declared with an **effect**:

| Effect | Promise of the host |
| --- | --- |
| `read` | A call changes nothing that matters to anyone: not host state, not the outside world. Calling it more often than the program does, or with arguments the program never passes, is harmless |
| `write` | No promise. This is the default |

1. The effect is a field of the declaration, beside the signature
   (`03-host.md` 3.3.1). It is not in the signature text, so no type text
   changes.
2. It is a promise, like `!`. Nothing checks it.
3. The description of an environment (F1 5.3) carries it: a key `effects`,
   an object from each function and `Type.method` marked `read` to `"read"`.
   Unmarked names are absent.
4. Analysis (F2 3.6) gives, beside the calls, a field `readOnly`: true when
   every host function and method the program can call is marked `read`.
   Library entries do not count against it.

### 4.3 Prefetch of `read` calls (19-8)

`02-evaluation.md` 10.1 item 3 forbids "a call of a host function that the
reference would not make". The relaxed rule:

> ... and never a call of a host function marked `write` that the reference
> would not make. A binding may make extra calls of a host function marked
> `read`, under 10.4.

New `02-evaluation.md` 10.4, **Prefetch**:

1. Inside a library entry that calls a lambda once for each element (`map`,
   `flatMap`, `filter`, and the others `04-library.md` marks so), a binding
   may call a `read` function ahead, for later elements, at the same time as
   it evaluates earlier ones.
2. A prefetched call's charges are recorded, not made. Its outcome is kept.
   Its charges are still bounded: the handle refuses a charge, and the
   prefetched call is dropped, when the recorded charges of every kept and
   running prefetch, with this one, would pass what the run has left. So
   prefetch cannot build a result that the reference call could not pay for,
   and many prefetches at once cannot pass the run's budget together.
3. **A write ends every prefetch.** When any call of a function or method
   marked `write` starts, in the reference evaluation or in a nested run,
   every kept outcome is dropped and every running prefetch is cancelled and
   dropped. No prefetch starts while a `write` call is in progress. A `read`
   function may give a result that depends on host state, and a write may
   change that state. So a kept outcome is used only when no write started
   between its start and its use.
4. When the reference evaluation reaches a call of the same function with
   `==` arguments, and a kept outcome for it survives rule 3, the binding
   charges the 1 step of the call, makes the recorded charges in order, and
   gives the kept outcome. Then 10.1 items 1 and 2 hold as if the call were
   made there.
5. A kept outcome that no reference call uses is dropped, with its charges.
   This happens when the arguments differ, when a failure or a limit ends the
   entry earlier, or when the run ends.
6. A prefetched call gets the run's cancellation signal. A call handle used
   for prefetch refuses `remaining()`, `runWithin` and the suspension
   sentinel: a `read` function that needs them is not prefetched. A
   prefetched call that returns the sentinel is dropped.
7. Prefetch never changes the value, the usage or the interactions of a run.
   It may change only the wall-clock time, and the count of calls the host
   sees. A suite case tests rule 3: `xs.map(x => { const _ = w.setText(x);
   w.text() })` with `xs = ["a", "b"]` gives `["a", "b"]` in every binding.

Not every binding can prefetch. A TypeScript host function is synchronous
(D18), so the TypeScript binding does not (inference: it would need an
asynchronous host function, which 3.5.2 item 1 rules out).

### 4.4 `Batch` (19-9)

A binding helper, not a language rule. It turns a function of one item into a
host function over a list.

| Matter | Rule |
| --- | --- |
| Go form | `risor.Batch(name, fn, opts...)`, where `fn` is `func(ctx context.Context, x T) (R, error)`. It gives a `Function` typed `(xs: T[]) => R[] !` |
| Running | At most `opts.Parallel` items run at once. Each gets its own record of charges. The default is 8 for `read` and 1 for `write`: with 1, the items run one by one in list order |
| Parallel writes | `opts.Parallel` above 1 with `write` is a promise, like `read`, that no item's result or effect depends on another's. Nothing checks it. Without that promise, a counter that each item increments gives values in the order the scheduler picks |
| Charges | After every item ends, the records are made through the call handle in list order |
| Errors | The first error in list order is the call's error. Items after it still ran |
| Effect | The effect given in the options, `write` by default |

With `Batch` a host writes `weather.currentAll(cities)` once, for any number
of functions. The charges and the result are in list order. That keeps the
run deterministic only when the items are: `read` items, `write` items run one
by one, or `write` items under the promise above. The run sees one host call,
so its interactions (3.1) hold either way.

### 4.5 Why lambdas still do not reach host functions (19-10)

#640 question 2 asks for `retry(() => ...)`, `parallel([...])` and
`withTimeout(...)`. They stay out. S3, D34 and `01-language.md` 8.2 rule 3 are
kept.

| Reason | Detail |
| --- | --- |
| The checker's sites | A host function that runs a lambda and catches its failure is a `try` the checker cannot see. A failure in the lambda would then not leave the run, and the error sites of `05-types.md` 18.3 and 18.4 would no longer describe what passes out of a call |
| Synchronous TypeScript | A TypeScript host function cannot wait for anything (D18). `parallel` over lambdas cannot be built there, and a feature in two implementations of three breaks C1 |
| Exact limits | Two lambdas of one run evaluated at once would charge in an order the program does not fix, so a limit would be reached at a different point in each run |
| What `retry` is for | Retrying a network call is the host function's own policy, done inside the call with the call handle's cancellation signal. Retrying Risor code that has no effect gives the same result |
| What `parallel` is for | Latency of host calls. 4.3 and 4.4 give that without a lambda leaving the run |

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

| Concern | Why it holds |
| --- | --- |
| Determinism | A method that reads changed state gives a host result, which 3.1 already counts as an input. A journal records it |
| Two runs share one object | The host's matter, as with any host state |
| Caching | A host call is never cached (3.5.2 item 5), so `const a = w.text(); const _ = w.setText("x"); w.text()` reads twice |
| The checker | Unchanged. It reasons about types; an opaque type has no fields to go stale |

### 5.2 Opaque interfaces (19-12)

A new declaration kind, **opaque interface**:

| Declaration | States | Portable form |
| --- | --- | --- |
| Opaque interface | A name, a list of methods, and the interfaces it extends | The name. Each method is a name and a signature in type text. The extended interfaces by name |
| Opaque type (changed) | Gains `implements`: a list of interface names | The names |

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
host's own types. Three are added to the Go root:

| Helper | Gives |
| --- | --- |
| `risor.Func(name string, fn any, opts ...FuncOption) Function` | A `Function` whose signature is derived from `fn`'s Go type |
| `risor.Methods[T](name string, opts ...MethodOption) OpaqueType` | An opaque type from `T`'s exported method set |
| `cmd/risor-bind` (`go generate`) | Go source that declares the same `Function` and `OpaqueType` values, with adapters that use no reflection |

**Deriving a signature:**

| Go | Signature |
| --- | --- |
| A first parameter `*risor.Call` or `context.Context` | Not a parameter. It receives the call handle, or its context |
| Each other parameter | A parameter, its type by F1 6.2 |
| A variadic `...T` | `bad-type` |
| Results `()`, `(R)` | `=> null`, `=> R` |
| Results `(error)`, `(R, error)` | `=> null !`, `=> R !` |
| Any other results | `bad-type` |
| Parameter names | `risor.Func`: from `opts` (`risor.Params("city", "units")`), else `a1`, `a2` and so on, because reflection has no names. The generator: the names in the source |
| Method names | The Go name with its first letter in lower case: `SetText` gives `setText`. A method tag in the options renames |

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
`Rule`, `OPAQUE[T]` and `Fields[T]`. Part E replaced them (R201). #641 read B.4
as the current plan. Each of those sections gets a note at its top: superseded
by Part E, with this plan for the S4 helpers.

### 6.2 Integers past 2^53 − 1 (19-14)

**The rule stays.** A number is a double. A whole number has a magnitude of at
most 2^53 − 1 (`01-language.md` 6.2, N2, D3). A host integer outside it cannot
enter (`03-host.md` 3.4.4). An int64 SQL id or a nanosecond timestamp is
passed as a string. The reason is C1 and C6: every implementation, TypeScript
among them, has the same numbers.

`03-host.md` 3.4.4 states it in passing. It gains a first paragraph that says
it plainly, with the two examples above.

**`json.parse` gains an option** (F6 6.2):

| Field | Value |
| --- | --- |
| Signature | `json.parse: (s: string, opts?: {bigInts?: "fail" \| "string"}) => any !` |
| `bigInts: "fail"` | Today's rule, and the default |
| `bigInts: "string"` | A number in integer form whose digits name more than 9007199254740991 gives a string: its text as written, `-` included. Its units are those of the string |

1. Steps are unchanged: `8*sz(s) + c`. The units of each such string are
   counted in `u`.
2. A bad value of `bigInts` is `invalid-argument`, tested before the call's
   step, as R147 does for `sortBy`'s direction.
3. Only the integer form changes. A number with a fraction or an exponent is
   read as now.

**Why `9007199254740993.0` reads and `9007199254740993` fails.** This is
already specified (F6 6.2.3 items 2 and 3) and is kept. A number in integer
form says the writer meant an exact integer. Rounding it would change an id
silently, so it fails. A number with a fraction or an exponent says the writer
meant a floating value, and the nearest double is its meaning. 6.2.3 gains one
sentence that gives this reason.

**F1 gains a row for host integers as text:**

| Binding | Row |
| --- | --- |
| Go (6.2) | An integer field tagged `json:",string"` or `risor:",string"` gives `string`, presented as its decimal text. Tried before "Every integer type" |
| Rust (6.3) | `#[risor(string)]` on an integer field does the same |
| TypeScript (6.4) | A `bigint` gives `string`, presented as its decimal text |

The F1 6.1 row "Numbers" gains: a binding may present a host integer as a
string when the host asks, and the type is then `string`.

## 7. Decisions

Settled means settled by the owner in the session of 2026-11-14. A
recommendation is a detail inside a settled direction that was not put to the
owner.

| # | Decision | State | Strongest alternative |
| --- | --- | --- | --- |
| 19-1 | Determinism covers reads and calls, with charges and outcomes | **Settled** | Leave it implied by 10.1. A host could not rely on it |
| 19-2 | Case fields `calls` and `reads` | Recommendation | Test the sequence in each binding's own tests. The suite would not hold it |
| 19-3 | A replay identity with SHA-256 of source and description | **Settled** (the key); recommendation (the hash) | Leave the key to the host |
| 19-4 | `suspended`: a run code that `catch` never catches, class `host`, exact usage, added in 3.0 | **Settled** (the code); recommendation (the class) | A new kind `suspend` in `05-types.md` 18.2. More exact, but the tenth kind for one code. Or cancellation: the host cancels and the run fails with `cancelled`. Then the usage is not fixed and replay cannot check it |
| 19-5 | Journals record charges, and opaque values by host ids | **Settled** | — |
| 19-6 | `remaining()`, `runWithin`; a child's limits are the smaller of both | **Settled** (the API); recommendation (the smaller of both) | The child takes the parent's remaining amounts and ignores its own environment's. Simpler, but a host cannot cap one tool |
| 19-7 | Effect marks `read` and `write`, `write` by default, outside the signature text | **Settled** (the marks); recommendation (outside the text) | In the signature text, `=> T ! read`. Visible in messages, but every type text grammar changes |
| 19-8 | Prefetch of `read` calls inside per-element entries | **Settled** | Prefetch anywhere a later call can be predicted. Harder to state, and the gain is in `map` |
| 19-9 | `Batch` helper | **Settled** | — |
| 19-10 | No lambda reaches a host function | **Settled** (kept) | Synchronous callbacks through `Call.Invoke`. Rejected for the reasons of 4.5 |
| 19-11 | An opaque value's state belongs to the host | **Settled** | — |
| 19-12 | Nominal opaque interfaces with `implements` and `extends` | **Settled** | The structural rule of 5.2 alone |
| 19-13 | `risor.Func`, `risor.Methods`, `cmd/risor-bind` | **Settled** | Amend S4 and keep hand-written signatures |
| 19-14 | The number rule stays; `bigInts`; `,string` rows | **Settled** | A `bigint` kind. It would change every number rule in three implementations |
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

## 8. What this changes in other documents

| Document | Section | Change |
| --- | --- | --- |
| `spec/core/02-evaluation.md` | 8.4 | A nested run's depth starts at the parent's count plus 1 (4.1) |
| | 9 | Item 1 covers interactions (3.1). New item: the interactions are fixed as 3.1 says |
| | 10.1 | Item 3 relaxed for `read` functions (4.3) |
| | new 10.4 | Prefetch (4.3) |
| | 12.2 | `suspended` joins the codes `catch` does not catch |
| `spec/core/03-host.md` | 3.2.5 (new) | Hosts that live long (7.1). Informative |
| | 3.3.1 | Function and method: an effect. New declaration: opaque interface. Opaque type: `implements` |
| | 3.3.4 | The errors of 5.2 |
| | 3.4.4 | A plain first paragraph on big integers |
| | 3.4.6 | Item 6: an opaque value's state belongs to the host (5.1) |
| | 3.5.2 | Item 3: four things, with `remaining()`. New item: `runWithin` and `callWithin` |
| | 3.5.4 | Row 1 scoped to language values |
| | 3.6.2 | The sequence of interactions |
| | new 3.6.4 | The replay identity; journals record charges and opaque ids |
| | 3.7.4 | `suspended`. "Eight codes pass" becomes nine; "the other eleven" stays eleven; 19 codes become 20 |
| | 3.9.1 | The determinism row names the interactions |
| | 3.10.6, 3.10.11 | `calls` and `reads`, and their JSON Schema |
| `spec/core/05-types.md` | 2.3, 3.2 | An interface name is a type |
| | 4 (Fits, Join) | The rows of 5.2 |
| | 9.1 step 4, 13.3 | Method lookup on a set of opaque names |
| | 18.4 | Promise 3 names `suspended` |
| | 18.5 | A row for `suspended`: `host`, checked or not |
| `spec/packages/f01-host-types.md` | 5.3 | Keys `interfaces` and `effects`. The count "exactly these keys" goes from 7 to 9. Opaque types list `implements` |
| | 6.1 to 6.4 | The `,string` rows (6.2) |
| `spec/packages/f02-templates-analysis.md` | 3.6, 4.2 | `readOnly` in analysis |
| `spec/packages/f04-conversion.md` | `is`, `as` | An interface name tested by `implements` |
| `spec/packages/f06-groups.md` | 6.2, 6.2.3 | `bigInts`; the reason for the two number forms |
| `suite/messages.json` | — | An entry for `suspended`, `caught: false`, one template |
| `suite/` | core-run, core-check, packages | Cases: `suspended` through `try`; `calls` and `reads`; prefetch cannot be tested by a case (it changes no outcome); interfaces; `bigInts` |
| `experiments/v3/design/00-overview.md` | 4 | D55 row; S4 row notes it is built by 19-13 |
| | — | Every place that counts run codes: 20, nine that pass `catch` |
| `experiments/v3/design/05-host-api.md` | B.4 to B.7 | Superseded notes (6.1) |
| | Part E | `remaining`, `runWithin`, `callWithin`, effects, interfaces, `Suspend`, `Batch`, `Func`, `Methods` in each binding's aligned surface |
| `experiments/v3/design/15-build-rulings.md` | — | A ruling for each of 19-4, 19-8 and 19-12, when built |

## 9. Cost

Estimates, by inference from the size of comparable parts. Not measured.

| Part | Go | TypeScript | Rust |
| --- | --- | --- | --- |
| `suspended`, `calls`, `reads` | Small: a code, a sentinel, two runner fields | Small | Small |
| `remaining`, `runWithin` | About 150 lines: the run's counters become a shared budget | Same | Same; the borrow of the parent's counters is the delicate part (inference) |
| Effects and `readOnly` | Small | Small | Small |
| Prefetch | About 300 lines and a concurrency test suite | Not built | About 300 lines, behind a feature |
| `Batch` | About 80 lines | Not built | Optional |
| Opaque interfaces | Checker: about 200 lines in Fits, Join and method lookup. Environment build: about 100 | Same | Same |
| `Func`, `Methods`, `risor-bind` | About 400 lines, and about 600 for the generator | — | — |
| `bigInts`, `,string` | Small | Small | Small |

The largest risk is prefetch: a concurrency bug there would break
determinism silently. It is optional for every binding, and the Go binding
should ship without it first and add it behind an option.

## 10. What I did not verify

- That every implementation's evaluator can expose `remaining()` with exact
  counts mid-run. Go's can (inference from `Call.Charge`, which already reads
  the counters). TypeScript's and Rust's: not checked.
- That `runWithin` can share counters in Rust without a lock or a copy. Not
  checked.
- That the method-lookup change of 5.2 keeps the checker within its bounds of
  `05-types.md` 20. Inference: the lookup is one pass over a set of at most
  the declared names.
- The cost of prefetch against plain sequential calls. Not measured.
- That `go/types` gives parameter names and doc comments for every shape the
  generator meets. Inference: it does for declared functions and methods.
- Whether fynerisor's 84 widget types fit one interface, or need several.
  From #641 only.
