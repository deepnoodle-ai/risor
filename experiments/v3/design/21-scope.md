# Risor v3 design: scope, and what 3.0 still needs

Status: revised on 2026-11-19, after the owner's adjustments of that day,
which `00-overview.md` section 4 and `14-implementation-plan.md` 5.3 now
hold. First drafted on 2026-11-15, from the discussion on PR #643.

| Part | State |
| --- | --- |
| The scope statement and its two tests (section 1) | **Not settled.** A recommendation the owner has not confirmed (D59) |
| Whether to keep the name "Risor v3" | **Open** (D60) |
| What #643 keeps and drops (section 3) | **Settled** by the owner on 2026-11-19, as `00` section 4 records it. Where this draft first said otherwise, the owner's choice stands |
| Explaining a result (2.5) and `glob` (2.6) | **Settled** as the next wave, in track F of `14` 5.3. Their designs below are sketches; each needs a plan of its own |
| A guide off expr (2.7) and a measurement against expr (2.8) | In track F. Not started |

## 1. Scope

Risor v3 is an **embedded expression language**. A host gives it data and
gets a value back: a condition, a template, a query over JSON, a config file.
A file may hold several named entry points (`17`), and a host calls one at a
time. Nothing outlives a run: no closure that the host keeps, no event loop,
no mutation, no loops. Its purpose is that of `deepnoodle-ai/expr`, made
portable to three implementations and with TypeScript's syntax.

Scripting stays on Risor v2, which is maintained: long-lived programs, an
event loop inside a program, loops and mutation. A host that lives long, such
as the GUI host of #641, may still use v3 by the pattern of `19-15`: the host
owns the state, and calls a named entry point for each event.

**The test for every proposal:** does it make writing, checking or running an
expression over host data better? If a host can do it, the host does it.

**The readability test:** a TypeScript programmer reads it without an
explanation. No fold or unfold patterns, and no implicit bindings.

| Follows from the scope | Why |
| --- | --- |
| No loops or reassignment in 3.0 (X3 stands) | An expression produces a value. `map`, `filter`, `find`, `sum` and recursion cover what a query needs |
| No regular expressions (D52 withdrawn, D9 stands) | Three hand-written matchers to keep identical, for a need that section 2.6 mostly meets |
| No closure kept by the host, no event loop (S3, `19-10` stand) | A host that lives long calls a named entry point per event (`19-15`). The GUI-host draft, which was never in the repository, is withdrawn |
| A function is passed, never kept (D57) | It reaches a user lambda or the library, and never a list, a map, a result or the host. It cannot outlive the call that received it |
| No `iterate` (dropped from `18`) | An unfold fails the readability test. Recursion covers cursors up to the depth limit |

## 2. What 3.0 needs, in order

### 2.1 Errors, hints and the editor

The people who write expressions are often not programmers, or are LLMs.
Workflow conditions are the main case. A good message is worth more to them
than a new feature. Plan `20` holds the hints and `foreign-names.json`, and
nested error messages (track B for the messages, track F for the hints):

```
order.items.length > 3
a value of type Item[] has no fields to read; use len(...)
```

The browser editor stays first-class. F12's completion and hover are built in
all three implementations (`suite/tools/`), and
`@deepnoodle/risor-codemirror`, `@deepnoodle/risor-react` and
`@deepnoodle/risor-mantine` are published. The remaining service and editor
work, such as worker deadlines in #635, goes on beside the tracks. Each track
keeps hover, completion and token highlighting in step (`14` 5.3).

### 2.2 `??` on absence

Data from JSON is often missing a field. `20` 3.1 lets `??` read a field that
may be absent, for every operand of a chain but the last:

```js
order.discount?.code ?? "none"
```

It is a Core amendment in track B, and needs the owner to confirm the
exception to `03-host.md` 3.4.8 item 2.

### 2.3 Recursion, `walk`, guards and function parameters

JSON of unknown depth is common: comment threads, nested config, a syntax
tree. Self-recursion reads plainly:

```js
const render = (c: Comment, indent: string): string =>
  `${indent}${c.author}: ${c.text}\n` +
  c.replies.map(r => render(r, indent + "  ")).join("");
render(thread, "")
```

`walk` gives every node of a tree: `walk(thread, c => c.replies)[*].author`.
Plan `18` holds these, with `match` guards and, since 2026-11-19, function
parameters (D57). `iterate` is dropped.

### 2.4 Analysis for hosts

F2 3 lists the names, paths and calls of a program without running it. Hosts
use it to fetch only the data a rule reads, to know when to evaluate again,
to show "this rule depends on `order.total`", and to check a rule when it is
saved. expr added the same report (expr #32), which is a sign of demand.

F2 3 is built in Go, TypeScript and Rust (`suite/analysis/`). It is kept in
step as new forms land: analysis is part of every track's definition of done
(`14` 5.3). A method of an opaque value is not in its list of calls, and a
host that caches on it says so (`19` 5.1). No language change.

### 2.5 Explaining a result

A rules engine or a workflow screen is asked "why was this false?" all the
time. F8 4's trace covers a failed run: the calls in progress. Nothing covers
a run that succeeds (searched `spec/packages/`; I).

Proposed: a host option that records the value of each operand of `&&`,
`||`, `!`, a comparison and `??`, and returns it beside the result.

```
order.total > 100 && customer.tier == "gold"        false
  order.total > 100           true    order.total = 250
  customer.tier == "gold"     false   customer.tier = "silver"
```

| Rule | Why |
| --- | --- |
| Off by default; a host turns it on per run | It costs memory |
| It changes no result and no count of steps or units | A run with it on reaches a limit at the same point as a run with it off |
| Only operands that were evaluated appear | `&&` stops early; the explanation shows what ran |
| Its own size limit; past it, the explanation is cut, not the run | A large `map` must not fail a run because it is explained |

This is a new feature. It needs a plan of its own, short, with suite cases.

### 2.6 Plain string matching in place of regexp

Most conditions that would use a regex test a shape: a file type, a prefix
and a suffix, a path.

```js
glob(file.name, "*.pdf") && glob(file.path, "reports/*/2026-*")
```

| Rule | Why |
| --- | --- |
| `*` is any run of code points, `?` is one code point; nothing else is special | Small enough to be identical in three implementations |
| No `/` rule, no `**`, no classes | Each is a question with several answers in other tools. They can come later without a break |
| Cost is bounded up front by pattern length × text length | Exact in every implementation, whatever the matcher does |

Extraction (#640's `"Order #1234 shipped"`) waits. If it comes, a scan
pattern such as `text.scan("Order #{id:int} shipped")` gives typed output and
fails the readability test less than a regex does.

### 2.7 Moving off expr

If v3 is expr's successor (D1), expr's users need a path:

| expr | Risor v3 |
| --- | --- |
| `filter(user.roles, it != "internal")` | `user.roles.filter(r => r != "internal")` |
| `sprintf("%s <%s>", u.name, u.email)` | `` `${u.name} <${u.email}>` `` |
| `lower(s)` | `s.toLowerCase()` |

A guide first. A converter if the workflow engine's own rules are enough to
justify it. The workflow engine is the first user.

### 2.8 Measure against expr

Hosts with thousands of small expressions care about compile time more than
about loop speed. Measure compile time and run time on condition-sized
programs against `deepnoodle-ai/expr` and `expr-lang/expr`. Not measured
yet. The spike of 2026-11-09 compared closures with the v2 VM, not with expr.

## 3. What became of #643

The first draft of this document proposed cutting more of #643 than the
owner did. On 2026-11-19 the owner kept `export const` entry points, the
host's ownership of opaque state, the Go S4 helpers and the design of host
opaque interfaces, and kept this draft's cuts of regular expressions,
`iterate`, replay and its parts, and `time.format` layouts. The table is the
outcome; `00` section 4 is its authority.

| Plan | Kept | Dropped |
| --- | --- | --- |
| 16 regexp (D52) | The `regexp` reservation (PR #642) | The plan |
| 17 entry points (D53) | All of it, in three slices: annotations and type lines; entry points; `SchemaOf` and doc comments | Descriptions through Join and Meet: they travel through identity only |
| 18 recursion (D54, D57) | Self-recursion, recursive named types, `walk`, guards; function parameters added | `iterate` |
| 19 host calls (D55) | Host calls in source order as part of determinism; host-owned opaque state; S4 in Go; host opaque interfaces as a design; a plain note on big integers; event-driven hosts | `suspended`, replay, `remaining()` and nested runs, effect marks, prefetch, `Batch`, `bigInts` |
| 20 feedback (D56) | `??` on absence, hints, `foreign-names.json`, the two parse codes, nested messages | The brief rendering; `time.format` layouts |

The three CodeRabbit threads that were open on #643 concerned prefetch,
`Batch` and `time.format`; each part is dropped.

## 4. Decisions for the owner

| # | Decision | Recommendation |
| --- | --- | --- |
| D59 | Write the scope of section 1 into `AGENTS.md` and `00-overview.md` as a constraint | Yes. Not yet confirmed |
| D60 | Keep the name "Risor v3", or rename v3 as expr's successor | Open. A rename has a case if the Risor name keeps bringing requests for scripting |
| — | Plans for the explanation (2.5) and `glob` (2.6) | Write them in track F, the explanation first |
