# Risor v3 Language Guide

Risor v3 is a small, typed expression language embedded in Go, TypeScript, and
Rust hosts. A host gives a Risor program data, checks it before it runs, runs it
within strict CPU, memory, and depth budgets, and gets a value back.

If you write TypeScript or JavaScript, you can already read most of Risor. A
program is zero or more `type` and `const` lines followed by a single result
expression (or a set of `export const` entry points the host calls by name):

<!-- risor
{
  "env": "{fetch: {tests: {name: string, status: \"pass\" | \"fail\", ms: int}[], owner: any}}",
  "vars": {
    "fetch": {
      "tests": [
        {"name": "auth", "status": "pass", "ms": 12},
        {"name": "billing", "status": "fail", "ms": 180},
        {"name": "search", "status": "pass", "ms": 45},
        {"name": "checkout", "status": "fail", "ms": 95}
      ],
      "owner": "payments"
    }
  },
  "expect": {
    "verdict": "fail",
    "summary": "2 of 4 failed",
    "names": ["billing", "checkout"],
    "slowest": ["billing 180ms", "checkout 95ms"],
    "owner": "payments"
  }
}
-->
```js
// Summarize a test run. `fetch` is data supplied by the host.
const failed = fetch.tests.filter(t => t.status == "fail");
const rate = len(failed) / len(fetch.tests);
const verdict = if (!failed) { "pass" }
  else if (rate < 0.1) { "flaky" }
  else { "fail" };

{
  verdict,
  summary: `${len(failed)} of ${len(fetch.tests)} failed`,
  names: failed[*].name,
  slowest: failed
    .sortBy(t => t.ms, "desc")
    .slice(0, 3)
    .map(({name, ms}) => `${name} ${ms}ms`),
  owner: try { fetch.owner as string } catch { "unknown" },
}
```

Every example in this guide is parsed, type-checked, and executed against the
Risor v3 implementation in CI.

## Contents

1. [Coming from JavaScript and TypeScript](#1-coming-from-javascript-and-typescript)
2. [Values and expressions](#2-values-and-expressions)
3. [`const`, `if`, `match`, and `try`](#3-const-if-match-and-try)
4. [Methods and queries](#4-methods-and-queries)
5. [Types, annotations, and checked conversions](#5-types-annotations-and-checked-conversions)
6. [Recursion and higher-order functions](#6-recursion-and-higher-order-functions)
7. [Templates](#7-templates)
8. [Errors, diagnostics, and limits](#8-errors-diagnostics-and-limits)

---

## 1. Coming from JavaScript and TypeScript

Risor borrows TypeScript's familiar surface syntax, then removes mutation,
statements, implicit coercions, and JavaScript's historical quirks. The table
below lists the departures that matter most on day one.

| JavaScript / TypeScript | Risor v3 | Why |
| --- | --- | --- |
| Statements, `let`, `var`, `for`, `while`, `return` | Expression-only: `const` lines + one final expression | Every form produces a value; no loops or mutation means runs stay bounded and predictable. |
| `xs.push(x)`, `obj.a = 1` | Immutable values: `[...xs, x]`, `{...obj, a: 1}` | Values never change; host data is read in place without copying. |
| `[]` and `{}` are truthy | `[]` and `{}` are **falsy** | `if (!failed) { ... }` naturally checks for an empty list or map. The six falsy values are `null`, `false`, `0`, `""`, `[]`, and `{}`. |
| `===` / `!==` vs loose `==` | Strict structural `==` and `!=` (no `===`) | No type coercion (`"1" == 1` is `false`), deep equality on lists and maps (`[1, 2] == [1, 2]` is `true`), `1 == 1.0` is `true`. |
| `a < b < c`, `a == b == c` | Parse error (`chained-comparison`) | Write `a < b && b < c` or parenthesize `(a == b) == c`. |
| `"n=" + 1` coerces to `"n=1"` | Check error (`kind`) | No implicit coercion. Write `` `n=${1}` `` or `"n=" + string(1)`. |
| `undefined` and `null` | Only `null` | There is no `undefined`. |
| `indexOf` / `findIndex` return `-1` | Return `null` | Every "not found" lookup returns `null` so `?.` and `??` handle it uniformly. |
| `o.opt ?? d` works because missing keys are `undefined` | `o.opt` alone is a check error (`maybe-absent`); `o.opt ?? d` and `o?.opt` work | Plain `.opt` rejects possibly absent fields; `??` directly over a field read coalesces absence to `null`. |
| `NaN`, `Infinity`, `-0` | None of these exist | Numbers are finite IEEE 754 binary64 values with a single `0`. Division by zero or overflow fails cleanly. |
| `7 / 2` is `number` | `/` always returns `float`; indexes require `int` | `int` is exact whole numbers (`\|x\| <= 2^53 - 1`). Use `floor(n / 2)` or `int(n / 2)` to index. |
| `xs.includes(x)`, `s.includes(sub)`, `k in m` | `x in xs`, `sub in s`, `k in m` (and `not in`) | One membership operator for lists, strings, and maps. |
| `xs.length`, `Object.keys(m)`, `Math.round(x)` | `len(xs)`, `m.keys()`, `round(x)` | Functions and methods are small, flat, and deterministic. |
| `console.log`, `Date.now`, `await`, `/regex/` | None of these exist | A program's only output is its value, host calls are synchronous, time comes from the host, and there is no regex engine. |

### Truthiness, structural equality, and `null` on miss

Empty lists and empty maps are falsy, `==` compares lists and maps by value, and
missing lookups return `null` instead of `-1` (note that writing `![]` or
`"2" == 2` with literals directly is caught at check time as `always-falsy` and
`never-equal`, so this example passes typed variables from the host):

<!-- risor
{
  "env": "{emptyList: int[], emptyMap: map<int>, raw: any}",
  "vars": {"emptyList": [], "emptyMap": {}, "raw": "2"},
  "expect": {
    "emptyListFalsy": true,
    "emptyMapFalsy": true,
    "deepListEq": true,
    "deepMapEq": true,
    "intFloatEq": true,
    "noCoercion": false,
    "missingIndex": null,
    "fallbackIndex": 0
  }
}
-->
```js
const xs = [10, 20, 30];
{
  emptyListFalsy: !emptyList,
  emptyMapFalsy: !emptyMap,
  deepListEq: [1, 2] == [1, 2],
  deepMapEq: {a: 1, b: 2} == {b: 2, a: 1},
  intFloatEq: 2 == 2.0,
  noCoercion: raw == 2,
  missingIndex: xs.indexOf(99),
  fallbackIndex: xs.findIndex(x => x > 100) ?? 0,
}
```

### Reading optional fields with `??` vs `?.`

When a record declares an optional field (`note?: string`), reading `order.note`
plainly is rejected by the checker because the key may be absent. Placing the
field read directly on the left of `??` tells Risor to treat an absent key as
`null`, and `?.` produces `string | null`:

<!-- risor
{
  "env": "{order: {id: string, note?: string}}",
  "vars": {"order": {"id": "ord-1"}},
  "expect": {
    "coalesced": "none",
    "optional": null,
    "formatted": "no note"
  }
}
-->
```js
{
  coalesced: order.note ?? "none",
  optional: order?.note,
  formatted: order?.note?.toUpperCase() ?? "no note",
}
```

---

## 2. Values and expressions

Risor has seven value kinds: `null`, `bool`, `number` (typed as `int` or
`float`), `string`, `list`, `map`, and opaque host values. Functions are not
values.

### Numbers: `int` and `float`

Every number at run time is a finite IEEE 754 binary64 value, with one zero
(`-0` normalizes to `0`). There is no `NaN` and no `Infinity`.

In the type system, numbers split into two types:

- **`int`** — exact whole numbers with `|x| <= 2^53 - 1` (`9_007_199_254_740_991`).
  Required for list/string indexes, exponents of `**`, and `int` parameters.
- **`float`** — any finite number. Every `int` fits `float`.

Number literals support `_` digit separators, hexadecimal (`0x...`), and
scientific notation (`1.5e3`). Binary (`0b`) and octal (`0o`) literals are not
supported.

<!-- risor
{
  "expect": {
    "million": 1000000,
    "hex": 255,
    "div": 3.5,
    "exactDiv": 3,
    "rem": -1,
    "pow": 8,
    "negPow": 4,
    "mid": "b"
  }
}
-->
```js
const items = ["a", "b", "c", "d"];
{
  million: 1_000_000,
  hex: 0xFF,
  div: 7 / 2,
  exactDiv: 6 / 2,          // value is 3, static type is float
  rem: -7 % 3,              // sign follows the dividend
  pow: 2 ** 3,
  negPow: (-2) ** 2,        // parentheses required: `-2 ** 2` is a parse error
  mid: items[int(len(items) / 2) - 1],
}
```

Because `/` always has type `float`, indexing with `items[len(items) / 2]` is
rejected at check time — convert with `int(...)` or `floor(...)` first. Notice
also that unary `-` or `!` cannot apply directly to `**`: write `(-2) ** 2` or
`-(2 ** 2)`.

### Strings, templates, raw strings, and text blocks

Strings are immutable sequences of Unicode code points. `len(s)` and `s[i]`
count **code points**, not UTF-16 code units or UTF-8 bytes, so `"a😀"` has
length `2` and `"a😀"[1]` is `"😀"`.

Risor offers four string literal forms:

1. **One-line strings** (`"..."` or `'...'`): support `\n`, `\r`, `\t`, `\"`,
   `\'`, `\\`, `\/`, `\b`, `\f`, `\uXXXX`, and `\u{X...}`.
2. **Template literals** (`` `...${expr}...` ``): may span lines and interpolate
   any expression. `null` renders as the empty string `""`; lists and maps render
   as compact JSON with sorted keys.
3. **Raw strings** (`r"..."` or `r'...'`): backslashes and `${...}` are plain
   characters with no escapes or interpolation.
4. **Text blocks** (`""" ... """`) and **raw text blocks** (`r""" ... """`):
   multi-line strings whose indentation is stripped based on the closing `"""`
   line. Opening `"""` must be followed by a newline, and closing `"""` must
   stand on a line with only leading spaces or tabs.

<!-- risor
{
  "env": "{user: {name: string, nick: string | null}}",
  "vars": {"user": {"name": "Ada", "nick": null}},
  "expect": {
    "codePoints": 2,
    "emoji": "😀",
    "greeting": "Hello, Ada ()",
    "rawPath": "C:\\builds\\new\\${not_interp}",
    "prompt": "Hello Ada,\n\n  - Review open PRs\n  - Check CI",
    "rawBlock": "line 1\n\\n stays literal ${x}"
  }
}
-->
```js
const prompt = """
  Hello ${user.name},

    - Review open PRs
    - Check CI
  """;

const rawBlock = r"""
  line 1
  \n stays literal ${x}
  """;

{
  codePoints: len("a😀"),
  emoji: "a😀"[1],
  greeting: `Hello, ${user.name} (${user.nick})`,
  rawPath: r"C:\builds\new\${not_interp}",
  prompt,
  rawBlock,
}
```

### Lists

List literals allow trailing commas and `...` spreads. Indexing is 0-based with
no negative indexes (use `xs.last()` or `xs.slice(-2)` instead). Out-of-bounds
plain indexing fails with `index-range`; optional indexing `xs?.[i]` returns
`null`.

<!-- risor
{
  "expect": {
    "combined": [0, 1, 2, 3, 4],
    "first": 1,
    "oob": null,
    "tail": [3, 4]
  }
}
-->
```js
const base = [1, 2, 3];
const combined = [0, ...base, 4];
{
  combined,
  first: base[0],
  oob: base?.[99],
  tail: combined.slice(-2),
}
```

### Maps

Map keys are always strings. Any word — even a reserved word such as `if` or
`in` — can be a bare key. Keys with punctuation use quotes, computed keys use
`[expr]`, shorthand `{a}` means `{a: a}`, and `...spread` copies entries (later
entries overwrite earlier ones). Maps have no insertion order: whenever keys are
iterated or rendered, they appear in ascending Unicode code-point order.

<!-- risor
{
  "env": "{envName: string}",
  "vars": {"envName": "prod"},
  "expect": {
    "merged": {"envName": "prod", "if": true, "retries": 5, "timeout": 30},
    "dynamic": {"env-prod": "active"}
  }
}
-->
```js
const defaults = {retries: 3, timeout: 30};
const merged = {
  ...defaults,
  retries: 5,       // overrides defaults.retries
  envName,          // shorthand for envName: envName
  if: true,         // reserved words are valid bare keys
};
const key = `env-${envName}`;
{
  merged,
  dynamic: {[key]: "active"},
}
```

### Operators, `in` / `not in`, `?.`, and `??`

Operator precedence follows JavaScript with three deliberate guardrails:

- **No chained comparisons**: `0 <= x <= 10` is a parse error; write
  `0 <= x && x <= 10`.
- **`??` never mixes with `&&`, `||`, or comparisons without parentheses**:
  `a ?? b || c` and `a ?? b > 0` are parse errors. Parenthesize to make intent
  explicit: `(a ?? 0) > 0`.
- **No `!` directly on the left of `in`**: write `x not in xs`, not `!x in xs`.

`&&` and `||` short-circuit and return the deciding operand (just like
JavaScript). `in` and `not in` work on lists (value equality), strings
(substring check), and maps (key presence).

<!-- risor
{
  "env": "{user: {role: string, team?: {name: string, lead?: string | null}}, tags: string[]}",
  "vars": {"user": {"role": "editor", "team": {"name": "core", "lead": null}}, "tags": ["v3", "docs"]},
  "expect": {
    "isStaff": true,
    "hasDocs": true,
    "notDeprecated": true,
    "hasSubstring": true,
    "hasKey": true,
    "lead": "unassigned"
  }
}
-->
```js
{
  isStaff: user.role in ["admin", "editor"],
  hasDocs: "docs" in tags,
  notDeprecated: "deprecated" not in tags,
  hasSubstring: "doc" in "v3-docs",
  hasKey: "team" in user,
  lead: user?.team?.lead ?? "unassigned",
}
```

Optional chaining short-circuits the **entire chain** when a `?.` or `?.[` step
meets `null`: in `a?.b.c`, if `a` is `null`, the whole chain evaluates to `null`
without touching `.c`. However, if `a` is a map without key `b`, `a?.b` produces
`null` and `.c` then fails unless written `a?.b?.c`.

---

## 3. `const`, `if`, `match`, and `try`

Risor has no statements. Every control-flow construct is an expression that
produces a value.

### `const` bindings, blocks, and destructuring

Every `const` line ends with `;` and is evaluated in order. Names cannot be
reassigned, though an inner block or branch may shadow an outer name. A block
`{ const ...; expr }` lets you introduce local bindings anywhere an expression
is expected — notice there is **no semicolon** after a block's final expression.

Destructuring works on maps and lists in `const` lines and lambda parameters.
Defaults trigger when a key or element is **absent or `null`**, and a default
can reference names bound earlier in the same pattern. Patterns are one level
deep (they do not nest).

<!-- risor
{
  "env": "{req: {method: string, path: string, password: string, timeout?: int | null}, scores: int[]}",
  "vars": {
    "req": {"method": "POST", "path": "/orders", "password": "secret", "timeout": null},
    "scores": [98, 91, 87, 72]
  },
  "expect": {
    "summary": "POST /orders (timeout 30s)",
    "safeKeys": ["method", "path", "timeout"],
    "top": 98,
    "third": 87,
    "restCount": 1
  }
}
-->
```js
const {password, ...safe} = req;          // drop `password` into a new map `safe`
const {method, path, timeout = 30} = req; // `timeout` is null, so default 30 applies
const [top, _, third = 0, ...others] = scores;

const summary = {
  const label = `${method} ${path}`;
  `${label} (timeout ${timeout}s)`
};

{
  summary,
  safeKeys: safe.keys(),
  top,
  third,
  restCount: len(others),
}
```

### `if` / `else if` / `else`

`if` always returns a value, so **`else` is always required**, and both
parentheses around conditions and braces around branches are required. Each
branch may start with local `const` lines followed by one result expression.

Two syntax details to remember:

- Because `{` right after `if (...)` or `else` always opens a branch, returning
  a map literal from a branch needs double braces: `if (ok) { {status: "ok"} } else { {status: "err"} }`.
- `if` is a whole expression, not an operand: wrap it in parentheses before
  applying an operator, e.g. `(if (vip) { 0.8 } else { 1.0 }) * total`.

<!-- risor
{
  "env": "{order: {total: float, vip: bool}}",
  "vars": {"order": {"total": 250, "vip": true}},
  "expect": {"tier": "gold", "payable": 200}
}
-->
```js
const tier = if (order.total >= 500) {
  "platinum"
} else if (order.total >= 100) {
  "gold"
} else {
  "standard"
};

const factor = if (order.vip) { 0.8 } else { 1.0 };

{
  tier,
  payable: (if (order.total > 0) { order.total } else { 0 }) * factor,
}
```

For short two-way expressions, the ternary `c ? a : b` is equivalent to
`if (c) { a } else { b }`.

### `match` and guards

`match` compares a subject expression with `==` against literal arms (strings,
numbers, booleans, or `null`) or the `_` wildcard arm. Multiple literals share
an arm with `|`.

When the subject has a finite type (such as a string literal union
`"open" | "held" | "closed"`, `bool`, or `T | null`), the checker verifies
exhaustiveness — if you miss a case and have no `_` arm, it reports
`non-exhaustive`.

Arms can also carry an **`if (condition)` guard**, evaluated only after a
literal in that arm matches. Guarded arms do not count toward exhaustiveness
because the guard may evaluate to false, so pair them with a plain fallback arm.

<!-- risor
{
  "env": "{order: {status: \"open\" | \"held\" | \"late\" | \"closed\", total: float, priority: bool}}",
  "vars": {"order": {"status": "open", "total": 1500, "priority": false}},
  "expect": "review"
}
-->
```js
match order.status {
  "open" if (order.total > 1000) => "review",
  "open" => "ship",
  "held" | "late" if (order.priority) => "escalate",
  "held" | "late" => "wait",
  "closed" => "done",
}
```

### `try`, `catch`, and `fail`

`try` is an expression that catches run-time failures in its body and returns
the `catch` handler's value instead. There is no `finally` and no `throw`
statement; raise application errors with `fail(message)` or `fail(message, data)`.

- Use `catch { fallback }` when you do not need the error details (it allocates
  no error map and costs zero extra steps).
- Use `catch (err) { ... }` to inspect the error map, which has four keys:
  `code`, `message`, `path` (non-null for `not-conforming`), and `data` (from
  `fail` or a nested Risor host error).

<!-- risor
{
  "env": "{raw: string, stock: int}",
  "vars": {"raw": "not-a-number", "stock": 0},
  "expect": {
    "parsed": 0,
    "caught": {
      "code": "failed",
      "message": "out of stock",
      "sku": "sku-9"
    }
  }
}
-->
```js
const parsed = try { int(raw) } catch { 0 };

const caught = try {
  if (stock <= 0) {
    fail("out of stock", {sku: "sku-9"})
  } else {
    {code: "ok", message: "reserved", sku: "sku-9"}
  }
} catch (err) {
  {
    code: err.code,
    message: err.message,
    sku: (err.data as {sku: string}).sku,
  }
};

{parsed, caught}
```

**Limits are never caught.** `catch` handles value and data errors
(`divide-by-zero`, `failed`, `function-error`, `index-range`,
`invalid-argument`, `missing-field`, `no-match`, `not-conforming`,
`null-operand`, `number-range`, `wrong-kind`), but **lets limit and host-crash
errors pass straight through**: `steps-exceeded`, `allocation-exceeded`,
`depth-exceeded`, `deadline-exceeded`, `cancelled`, `function-panic`,
`internal-error`, and `missing-variable`. A program cannot trap a step or
memory limit inside `try`/`catch`.

---

## 4. Methods and queries

Because Risor has no loops, you transform and aggregate data with list methods,
map methods, string methods, and projections. No method mutates its receiver.
See the full [Standard Library Reference](library/README.md) for every signature
and exact cost formula.

### Filtering, transforming, and finding

List callbacks receive `(item, index)` — you may omit `index` if you don't need
it. `filter` and `find` also **narrow the element type** based on checks like
`x != null`, `x is T`, or tagged-union checks like `x.kind == "circle"`.

<!-- risor
{
  "env": "{scores: (int | null)[], users: {name: string, active: bool, role: string}[]}",
  "vars": {
    "scores": [10, null, 25, null, 15],
    "users": [
      {"name": "Ada", "active": true, "role": "admin"},
      {"name": "Bob", "active": false, "role": "member"},
      {"name": "Cara", "active": true, "role": "member"}
    ]
  },
  "expect": {
    "validSum": 50,
    "activeNames": ["Ada", "Cara"],
    "firstAdmin": "Ada",
    "firstInactiveIdx": 1,
    "missingIdx": null,
    "allHaveNames": true,
    "anyAdmin": true
  }
}
-->
```js
const valid = scores.filter(s => s != null);   // narrowed from (int | null)[] to int[]
{
  validSum: valid.sum(),
  activeNames: users.filter(u => u.active).map(u => u.name),
  firstAdmin: users.find(u => u.role == "admin")?.name,
  firstInactiveIdx: users.findIndex(u => !u.active),
  missingIdx: users.map(u => u.name).indexOf("Zoe"),
  allHaveNames: users.every(u => len(u.name) > 0),
  anyAdmin: users.some(u => u.role == "admin"),
}
```

### Querying, sorting, grouping, and mapping values

Risor includes query methods built for JSON and workflow data:

- `sortBy(f)` / `sortBy(f, "desc")` — stable sort by a numeric or string key.
  Chain a minor sort before a major sort for multi-key ordering.
- `minBy(f)` / `maxBy(f)` — return the element with the smallest/largest key (or
  `null` on an empty list; ties keep the first element).
- `uniqueBy(f)` — deduplicate keeping the first element per key.
- `indexBy(f)` — build `map<T>` keyed by string (later element wins on duplicate
  keys).
- `countBy(f)` — build `map<int>` of counts per string key without allocating
  intermediate groups.
- `groupBy(f)` — build `map<T[]>` of elements grouped by string key.
- `mapValues((value, key) => ...)` — transform every value of a map in ascending
  key order.

<!-- risor
{
  "env": "{events: {id: string, service: string, ms: int}[]}",
  "vars": {
    "events": [
      {"id": "e1", "service": "api", "ms": 40},
      {"id": "e2", "service": "db", "ms": 120},
      {"id": "e3", "service": "api", "ms": 85},
      {"id": "e4", "service": "cache", "ms": 5}
    ]
  },
  "expect": {
    "slowestId": "e2",
    "fastestId": "e4",
    "topTwoIds": ["e2", "e3"],
    "uniqueServices": ["api", "db", "cache"],
    "counts": {"api": 2, "cache": 1, "db": 1},
    "byIdMs": 85,
    "totalsByService": {"api": 125, "cache": 5, "db": 120}
  }
}
-->
```js
const byId = events.indexBy(e => e.id);
const totalsByService = events
  .groupBy(e => e.service)
  .mapValues(group => group.map(e => e.ms).sum());

{
  slowestId: events.maxBy(e => e.ms)?.id,
  fastestId: events.minBy(e => e.ms)?.id,
  topTwoIds: events.sortBy(e => e.ms, "desc").slice(0, 2).map(e => e.id),
  uniqueServices: events.uniqueBy(e => e.service).map(e => e.service),
  counts: events.countBy(e => e.service),
  byIdMs: byId?.["e3"]?.ms,
  totalsByService,
}
```

### Projections with `[*]` and `?.[*]`

A projection `xs[*].field` extracts a field across every element of a list into
a new list, with less boilerplate and fewer steps than `.map(...)`. A second
`[*]` flattens nested lists in one pass, and a method call directly after a
projection operates on the resulting list. Use `?.[*]` or `?.field` when an
inner list or optional field may be absent or `null`:

<!-- risor
{
  "env": "{orders: {id: string, note?: string, lines: {sku: string, qty: int}[]}[], drafts: {id: string, lines: {sku: string, qty: int}[] | null}[]}",
  "vars": {
    "orders": [
      {"id": "o1", "note": "rush", "lines": [{"sku": "A", "qty": 2}, {"sku": "B", "qty": 1}]},
      {"id": "o2", "lines": [{"sku": "A", "qty": 4}]}
    ],
    "drafts": [
      {"id": "d1", "lines": [{"sku": "X", "qty": 1}]},
      {"id": "d2", "lines": null}
    ]
  },
  "expect": {
    "ids": ["o1", "o2"],
    "notes": ["rush", null],
    "skus": ["A", "B", "A"],
    "totalQty": 7,
    "draftSkus": ["X", null]
  }
}
-->
```js
{
  ids: orders[*].id,
  notes: orders[*]?.note,
  skus: orders[*].lines[*].sku,
  totalQty: orders[*].lines[*].qty.sum(),
  draftSkus: drafts[*].lines?.[*].sku,
}
```

Two rules to keep in mind with projections:

1. **A method call ends the projection and receives the list.** Writing
   `users[*].email.toLowerCase()` tries to call `toLowerCase` on `string[]` (a
   check error). Write `users.map(u => u.email.toLowerCase())` instead.
2. **Reads inside a projection are not coalesced by `??`.** Write
   `orders[*]?.note` or `orders.map(o => o.note ?? "")`, not
   `orders[*].note ?? ""`.

### String methods

String methods operate on Unicode code points, use a pinned Unicode 16.0.0
simple case table, and never accept regular expressions (`replaceAll` matches
literal text):

<!-- risor
{
  "env": "{ref: string, raw: string}",
  "vars": {"ref": "refs/heads/release-3.0", "raw": "  v3, docs,  guide  "},
  "expect": {
    "branch": "release-3.0",
    "parts": ["v3", "docs", "guide"],
    "padded": "0042",
    "replaced": "release_3.0",
    "pos": 7
  }
}
-->
```js
const branch = ref.removePrefix("refs/heads/");
{
  branch,
  parts: raw.trim().split(",").map(s => s.trim()),
  padded: "42".padStart(4, "0"),
  replaced: branch.replaceAll("-", "_"),
  pos: branch.indexOf("-"),
}
```

### Opt-in library groups

Four standard namespaces are disabled by default and turned on explicitly by the
host: [`json`](library/json.md), [`time`](library/time.md),
[`math`](library/math.md), and [`base64`](library/base64.md).

<!-- risor
{
  "groups": ["json", "time", "math", "base64"],
  "expect": {
    "parsed": {"ok": true},
    "encoded": "aGVsbG8=",
    "windowSec": 5400,
    "hypotenuse": 5
  }
}
-->
```js
{
  parsed: json.parse(`{"ok": true}`),
  encoded: base64.encode("hello"),
  windowSec: time.duration("1h30m"),
  hypotenuse: math.sqrt(3 ** 2 + 4 ** 2),
}
```

Browse the full [Standard Library Reference](library/README.md) for all 84
entries across `core`, `json`, `time`, `math`, and `base64`.

---

## 5. Types, annotations, and checked conversions

Risor's checker runs before evaluation so unknown names, typos in field names,
unhandled `null`s, absent optional fields, and kind mismatches fail at compile
time instead of in production.

### Core types and structural records

| Type syntax | Meaning |
| --- | --- |
| `null`, `bool`, `int`, `float`, `string` | Primitive types (`int` fits `float`). |
| `"open" \| "closed"` | String literal union (only strings have literal types). |
| `T[]` | List of `T`. Empty list `[]` has type `never[]`. |
| `map<T>` | Map with string keys and values of type `T`. |
| `{id: string, note?: string}` | **Exact record**: required `id`, optional `note`, no other keys allowed. |
| `{id: string, ...}` | **Open record**: required `id` plus any other keys of type `any`. |
| `{id: string, ...int}` | Record with required `id` and all other keys of type `int`. |
| `T \| null` | Nullable type (`null` is always explicit). |
| `any` | Unchecked type (recorded as a `dynamic` site; host may reject in strict mode). |
| `never` | Type of expressions that never return a value (such as `fail(...)`). |

Two differences from TypeScript are especially important:

- **Exact records have no width subtyping.** `{a: int, b: int}` does **not** fit
  `{a: int}`. Write `{a: int, ...}` when extra keys should be permitted.
- **Optional (`?`) and nullable (`| null`) are distinct.** `{note?: string}`
  means `note` may be absent and is a `string` when present (never `null`).
  `{note: string | null}` means `note` is always present and its value may be
  `null`.

### Tagged unions and narrowing

Two or more records can form a **tagged union** when every variant lists a
shared required field whose type is a disjoint string literal set (such as
`kind: "circle"` vs `kind: "rect"`). Untyped or untagged record unions like
`{a: int} | {b: string}` are rejected with `untagged-union`.

You narrow a tagged union with `match`, `==` / `!=`, `in` / `not in`, `is`, or
inside `filter` / `find`:

<!-- risor
{
  "env": "{shapes: ({kind: \"circle\", r: float} | {kind: \"rect\", w: float, h: float})[]}",
  "groups": ["math"],
  "vars": {
    "shapes": [
      {"kind": "circle", "r": 2},
      {"kind": "rect", "w": 3, "h": 4}
    ]
  },
  "expect": {
    "areas": [12.566370614359172, 12],
    "radii": [2]
  }
}
-->
```js
type Shape = {kind: "circle", r: float} | {kind: "rect", w: float, h: float};

const area = (s: Shape): float => match s.kind {
  "circle" => math.pi * s.r * s.r,
  "rect" => s.w * s.h,
};

{
  areas: shapes.map(area),
  radii: shapes.filter(s => s.kind == "circle").map(c => c.r),
}
```

### `type` lines and type annotations

At the top level of a program, `type Name = ...;` declares a structural type
alias. Type names must begin with an uppercase letter and be at least two
characters long (single uppercase letters like `T` are reserved for library
signatures).

You can annotate `const` bindings, lambda parameters, and lambda return types:

<!-- risor
{
  "env": "{orders: {id: string, lines: {sku: string, qty: int, price: float}[]}[]}",
  "vars": {
    "orders": [
      {"id": "o1", "lines": [{"sku": "A", "qty": 2, "price": 10.0}]},
      {"id": "o2", "lines": [{"sku": "B", "qty": 1, "price": 24.0}]}
    ]
  },
  "expect": {"count": 2, "totals": [25, 30]}
}
-->
```js
type Line = {sku: string, qty: int, price: float};
type Order = {id: string, lines: Line[]};

const totalWithTax = (order: Order, rate: float = 0.25): float =>
  order.lines.map((l: Line) => l.qty * l.price).sum() * (1 + rate);

const count: int = len(orders);
{
  count,
  totals: orders.map(o => totalWithTax(o)),
}
```

**Annotations are static claims with zero run-time cost.** The checker must be
able to prove that the expression surely fits the annotation — it will **not**
silently accept `any`:

<!-- risor
{
  "env": "{payload: any}",
  "checkError": "result-mismatch",
  "messageContains": "expected int, found any"
}
-->
```js
const count: int = payload;
count + 1
```

To validate untyped (`any`) data at run time, use `as` or `is`.

### Checked conversions: `as` and `is`

`x as T` and `x is T` walk a value at run time against any type `T` (including
nested records, lists, literal unions, and tagged unions).

- **`x is T`** returns `true` or `false` without failing, and narrows `x` in
  `if`, `?:`, `&&`, `||`, `filter`, and `find`.
- **`x as T`** returns `x` unchanged when it conforms, or fails with
  `not-conforming` and a precise `path` to the first bad field or element.
  **Nothing is coerced**: `"42" as int` fails.

<!-- risor
{
  "env": "{raw: any, bad: any}",
  "vars": {
    "raw": {"id": "u-1", "age": 30},
    "bad": {"id": "u-2", "age": "thirty"}
  },
  "expect": {
    "label": "u-1 (30)",
    "isUser": false,
    "errCode": "not-conforming",
    "errPath": ["age"]
  }
}
-->
```js
type User = {id: string, age: int};

const u = raw as User;
const errInfo = try {
  const _ = bad as User;
  {code: "ok", path: []}
} catch (err) {
  {code: err.code, path: err.path}
};

{
  label: `${u.id} (${u.age})`,
  isUser: bad is User,
  errCode: errInfo.code,
  errPath: errInfo.path,
}
```

Watch precedence with `as` and `is`: parenthesize `(x as int) + 1`,
`(x as User).id`, and `!(x is User)` (because `!x is User` is a parse error).

### Doc comments and `export const` entry points

A `/** ... */` doc comment placed directly before a `type` line, a record field,
a `const` line, an `export const` line, or a lambda parameter attaches a
description. Hosts read these descriptions for hover help, generated JSON
Schemas, and tool discovery.

A program can expose named entry points to the host with `export const`. Every
parameter of an exported lambda must carry a type annotation (and cannot be a
function type). A file with at least one `export const` does not need a trailing
result expression; when the host calls one entry point, Risor evaluates only the
top-level `const` lines that entry point actually uses.

<!-- risor
{
  "entries": [
    {
      "name": "triage",
      "doc": "Classify an incoming issue by label and age.",
      "inputType": "{ageHours?: int, labels: string[]}",
      "resultType": "\"normal\" | \"urgent\"",
      "params": [
        {"name": "labels", "type": "string[]", "doc": "Labels attached to the issue.", "optional": false},
        {"name": "ageHours", "type": "int", "doc": "Hours since the issue opened.", "optional": true}
      ]
    },
    {
      "name": "isUrgent",
      "doc": "Return true when the issue needs immediate attention.",
      "inputType": "{ageHours?: int, labels: string[]}",
      "resultType": "bool"
    }
  ],
  "calls": [
    {"entry": "triage", "args": [["p0", "billing"], 2], "expect": "urgent"},
    {"entry": "triage", "input": {"labels": ["docs"], "ageHours": 12}, "expect": "normal"},
    {"entry": "isUrgent", "input": {"labels": ["bug"], "ageHours": 72}, "expect": true}
  ]
}
-->
```js
const urgentLabels = ["p0", "security", "outage"];
const slaHours = 48;

/** Classify an incoming issue by label and age. */
export const triage = (
  /** Labels attached to the issue. */
  labels: string[],
  /** Hours since the issue opened. */
  ageHours: int = 0,
): "urgent" | "normal" =>
  if (labels.some(l => l in urgentLabels) || ageHours >= slaHours) {
    "urgent"
  } else {
    "normal"
  };

/** Return true when the issue needs immediate attention. */
export const isUrgent = (labels: string[], ageHours: int = 0): bool =>
  triage(labels, ageHours) == "urgent";
```

---

## 6. Recursion and higher-order functions

Functions in Risor are **not first-class values**: you cannot put a lambda in a
list or map, return one from a function, or pass one to a host function. Within
that first-order boundary, Risor supports reusable `const` helpers, tree
walking, typed self-recursion, recursive types, and function parameters.

### Walking trees with `walk`

For tree-shaped data, the built-in `walk(root, children)` function visits every
node in pre-order using an explicit bounded stack and returns a flat list of
visited nodes. The callback receives `(node, depth)` and returns a list of child
nodes (or `null` / `[]` for leaves).

<!-- risor
{
  "env": "{tree: any}",
  "vars": {
    "tree": {
      "name": "root",
      "kids": [
        {"name": "a", "kids": []},
        {"name": "b", "kids": [{"name": "b1", "kids": []}]}
      ]
    }
  },
  "expect": {
    "all": ["root", "a", "b", "b1"],
    "shallow": ["root", "a", "b"]
  }
}
-->
```js
{
  all: walk(tree, n => n.kids).map(n => n.name),
  shallow: walk(tree, (n, d) => d < 1 ? n.kids : null).map(n => n.name),
}
```

### Typed self-recursion and recursive types

A `const`-bound lambda can call itself (or pass itself to a library method like
`map` or `reduce`) **when every parameter and its return type are annotated**.
Without full annotations, self-reference is rejected with
`recursion-needs-types`.

Named `type` lines may also refer to themselves — or mutually to one another —
as long as every cycle passes through a list, map, or record field (a direct
cycle such as `type Loop = Loop | null;` is rejected with `circular-type`).

<!-- risor
{
  "env": "{root: any}",
  "vars": {
    "root": {
      "name": "src",
      "size": 0,
      "children": [
        {"name": "main.go", "size": 120, "children": []},
        {
          "name": "pkg",
          "size": 0,
          "children": [
            {"name": "util.go", "size": 80, "children": []}
          ]
        }
      ]
    }
  },
  "expect": {"totalBytes": 200, "fact5": 120}
}
-->
```js
type Node = {name: string, size: int, children: Node[]};

const totalSize = (n: Node): int =>
  n.size + n.children.map(totalSize).sum();

const factorial = (n: int): int =>
  if (n <= 1) { 1 } else { n * factorial(n - 1) };

{
  totalBytes: totalSize(root as Node),
  fact5: factorial(5),
}
```

Mutually recursive types and JSON values work the same way:

<!-- risor
{
  "env": "{doc: any}",
  "vars": {"doc": {"a": [1, true, {"b": "ok"}]}},
  "expect": true
}
-->
```js
type Json = null | bool | float | string | Json[] | map<Json>;
doc is Json
```

Forgetting a type annotation on a self-recursive lambda or writing an unguarded
type cycle is caught at check time:

<!-- risor
{
  "checkError": "recursion-needs-types",
  "messageContains": "give every parameter and the result a type"
}
-->
```js
const countdown = (n: int) => if (n <= 0) { 0 } else { countdown(n - 1) };
countdown(5)
```

<!-- risor
{
  "checkError": "circular-type",
  "messageContains": "outside any list, map or record"
}
-->
```js
type Loop = Loop | null;
null
```

### Function parameters

A `const`-bound lambda may accept a callback by giving its parameter a function
type `(x: T) => R` (or `=> unknown` for truthiness predicates). Inside the body,
that parameter may be called or forwarded to a library method or another helper.

<!-- risor
{
  "env": "{orders: {id: string, qty: int, price: float}[]}",
  "vars": {
    "orders": [
      {"id": "o1", "qty": 2, "price": 15.0},
      {"id": "o2", "qty": 3, "price": 10.0}
    ]
  },
  "expect": {"revenue": 60, "scaled": 12}
}
-->
```js
type Order = {id: string, qty: int, price: float};

const sumBy = (xs: Order[], f: (o: Order) => float): float =>
  xs.map(f).sum();

const applyTwice = (f: (x: int) => int, x: int): int =>
  f(f(x));

{
  revenue: sumBy(orders, o => o.qty * o.price),
  scaled: applyTwice(n => n * 2, 3),
}
```

**What cannot escape a call:**

- Functions remain non-values: a function parameter cannot be stored in a list
  or map, returned from a lambda, compared, or passed to a host function.
- A function parameter cannot have a default value.
- A higher-order lambda (one that takes a function parameter) can only be called
  by name — it cannot itself be passed as a callback.
- An `export const` entry point cannot take a function parameter, because a host
  cannot pass a Risor lambda across the boundary.

---

## 7. Templates

Risor has two complementary template forms:

- **Template literals** (`` `Hello, ${user.name}!` `` and `""" ... """`) live
  inside a Risor program.
- **Host templates** are plain text files or strings owned by the host — such as
  prompts, notifications, or YAML values — with **`${{ ... }}`** expression
  parts.

### Expressions inside `${{ ... }}`

Every `${{ ... }}` part holds a single Risor expression. All parts in a template
run together under one step, allocation, and depth budget. Bindings do not cross
parts (use a block `${{ { const n = len(items); n * 2 } }}` when a single part
needs local `const` bindings).

<!-- risor
{
  "template": "string",
  "env": "{pr: {number: int, title: string, branch: string}, gate: {passed: int, failed: int}}",
  "vars": {
    "pr": {"number": 42, "title": "Add guide", "branch": "docs/guide"},
    "gate": {"passed": 18, "failed": 0}
  },
  "expect": "PR #42 (Add guide) on docs/guide: 18 passed, 0 failed — ready"
}
-->
```text
PR #${{ pr.number }} (${{ pr.title }}) on ${{ pr.branch }}: ${{ gate.passed }} passed, ${{ gate.failed }} failed — ${{ gate.failed == 0 ? "ready" : "blocked" }}
```

### Escaping

`$${{` is the **only** escape in a host template's text runs: it produces the
literal characters `${{`. Backslashes, quotes, backticks, and line endings stay
completely untouched. To write a literal `$` immediately before an expression
part, put it inside the part: `${{ "$" + fixed(price, 2) }}`.

<!-- risor
{
  "template": "string",
  "env": "{price: float}",
  "vars": {"price": 19.5},
  "expect": "Total: $19.50 (syntax: ${{ expr }})"
}
-->
```text
Total: ${{ "$" + fixed(price, 2) }} (syntax: $${{ expr }})
```

### `string` mode vs `value` mode

A host compiles a template in one of two modes:

- **`string` mode** always produces a `string`, rendering every part (and
  rendering `null` as `""`).
- **`value` mode** preserves the exact typed value (number, boolean, list, map,
  or `null`) when the entire template is a **single `${{ expr }}`** with no
  surrounding text or whitespace. If any text surrounds the part (even a single
  space), `value` mode renders a string just like `string` mode.

<!-- risor
{
  "template": "value",
  "env": "{tags: string[]}",
  "vars": {"tags": ["alpha", "beta"]},
  "type": "string[]",
  "expect": ["alpha", "beta"]
}
-->
```text
${{ tags }}
```

---

## 8. Errors, diagnostics, and limits

### Three phases

Every Risor program passes through three stages (after the host builds an
environment in the `declare` phase):

| Phase | Catches | Example codes |
| --- | --- | --- |
| `parse` | Lexical and grammar errors, source size/nesting caps | `unexpected-token`, `chained-comparison`, `async-await`, `statement-sequence`, `nesting-exceeded` |
| `check` | Static rules and type errors before any code runs | `undefined-name`, `no-field`, `maybe-null`, `maybe-absent`, `kind`, `result-mismatch`, `non-exhaustive`, `recursion-needs-types` |
| `run` | Dynamic data failures, `fail(...)`, and resource limits | `divide-by-zero`, `index-range`, `not-conforming`, `failed`, `steps-exceeded`, `allocation-exceeded`, `depth-exceeded` |

### Reading diagnostics and hints

Diagnostics carry a stable `code`, a `line:col-line:col` span, an underlined
source snippet, and built-in hints when you reach for a JavaScript, TypeScript,
or Python habit by mistake.

For example, writing `.length` on a list fails at check time and tells you the
Risor function to use:

<!-- risor
{
  "env": "{xs: int[]}",
  "checkError": "kind",
  "messageContains": "use len(...)"
}
-->
```js
xs.length
```

<!-- risor {"matchesPreviousDiagnostic": "detail"} -->
```text
error[kind] 1:1-1:3: a value of type int[] has no fields to read; use len(...)
    xs.length
    ^^
```

Writing `await` or chaining statements with `;` fails at parse time with a
targeted diagnostic:

<!-- risor
{
  "env": "{xs: int[]}",
  "parseError": "statement-sequence",
  "messageContains": "bind earlier values with const"
}
-->
```js
xs.push(1); xs
```

<!-- risor {"matchesPreviousDiagnostic": "detail"} -->
```text
error[statement-sequence] 1:11-1:12: a program is const lines and then one expression; bind earlier values with const, and remember that values never change
    xs.push(1); xs
              ^
```

<!-- risor
{
  "env": "{user: {name: string}}",
  "parseError": "async-await",
  "messageContains": "remove await"
}
-->
```js
await user.name
```

<!-- risor {"matchesPreviousDiagnostic": "detail"} -->
```text
error[async-await] 1:1-1:6: there is no await: a host call gives its value directly; remove await
    await user.name
    ^^^^^
```

And reading an optional field without handling absence explains all three ways
to fix it:

<!-- risor
{
  "env": "{order: {id: string, note?: string}}",
  "checkError": "maybe-absent",
  "messageContains": "read it with ?."
}
-->
```js
order.note
```

<!-- risor {"matchesPreviousDiagnostic": "detail"} -->
```text
error[maybe-absent] 1:7-1:11: field "note" may be absent from {id: string, note?: string}: read it with ?., add ?? and a default, or test for it first
    order.note
          ^^^^
```

### Execution limits

Limits are part of the language specification, not an afterthought. Every
operation has an exact, documented cost in **steps** (CPU work) and **allocation
units** (values created), so a program hits a deterministic limit at the exact
same point in Go, TypeScript, and Rust.

| Limit | Default | Bounds | Code | Deterministic |
| --- | --- | --- | --- | --- |
| Steps | `1,000,000` | CPU work across operators, calls, and walks | `steps-exceeded` | Yes |
| Allocation | `1,000,000` units | Newly allocated strings, lists, and maps (host data costs `0`) | `allocation-exceeded` | Yes |
| Depth | `200` (max `400`) | Active calls, recursion, `walk`, and deep value comparisons/renders | `depth-exceeded` | Yes |
| Deadline | Off | Wall-clock run time (and host cancellation) | `deadline-exceeded` / `cancelled` | No |
| Source nesting | `100` (fixed) | Nested brackets and expressions in source text | `nesting-exceeded` (`parse`) | Yes |
| Source size | `1,048,576` code points (fixed) | Maximum program or template length | `source-too-large` (`parse`) | Yes |

Hosts can lower these limits per environment. If a program exceeds any budget,
the run stops immediately — even inside a `try` / `catch`:

<!-- risor
{
  "limits": {"steps": 100},
  "runError": "steps-exceeded"
}
-->
```js
try {
  range(0, 500)
} catch {
  []
}
```

<!-- risor {"matchesPreviousDiagnostic": "detail"} -->
```text
error[steps-exceeded] 1:1-5:2: the run needed more than its limit of 100 steps
    try {
    ^^^^^
```

Allocation and depth limits work the same way:

<!-- risor
{
  "limits": {"allocation": 20},
  "runError": "allocation-exceeded"
}
-->
```js
"a".repeat(2000)
```

<!-- risor
{
  "limits": {"depth": 10},
  "runError": "depth-exceeded"
}
-->
```js
const down = (n: int): int => if (n <= 0) { 0 } else { down(n - 1) };
down(20)
```

Because there are no unbounded loops, no mutation of host data, no hidden I/O,
and no way to catch limit failures, a host can safely compile and run
user-authored or LLM-generated Risor programs with predictable cost.

---

## Further reading

- [Standard Library Reference](library/README.md) — every function, method, opt-in group, and cost formula
- [Core language specification](../spec/core/01-language.md) — normative grammar and semantics
- [Types and checker specification](../spec/core/05-types.md) — normative type rules and narrowing
- [Host embedding specification](../spec/core/03-host.md) — environment setup, host functions, and limits
- [Benchmarks against expr](benchmarks.md) — compile-time and run-time measurements
- [Versioning policy](versioning.md) — compatibility promises for 3.x
