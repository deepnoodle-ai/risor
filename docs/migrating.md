# Migrating to Risor v3

Risor v3 is a small, statically checked expression language with native
implementations in Go, TypeScript, and Rust. It is the successor to
`deepnoodle-ai/expr`, and a clean break from Risor v2.

This guide covers:

- [When to move — and when to stay on Risor v2](#when-to-move--and-when-to-stay-on-risor-v2)
- [Part 1: Moving from `expr`](#part-1-moving-from-expr) (`deepnoodle-ai/expr` and `expr-lang/expr`)
- [Part 2: Moving from Risor v2](#part-2-moving-from-risor-v2)
- [Migration checklist](#migration-checklist)

For related reference material, see the
[Standard Library Reference](library/README.md),
[Benchmarks against `expr`](benchmarks.md), and the
[Stability Promise and Versioning Policy](versioning.md).

---

## When to move — and when to stay on Risor v2

Risor v3 is an **embedded expression language**, not a general-purpose scripting
language. A host gives it data and gets a value back. A file may contain
`const` bindings, `type` definitions, and one result expression — or several
named `export const` entry points that the host calls one at a time. Nothing
outlives a single run: there are no loops, no reassignment, no mutation, no
stored callbacks, and no event loops inside a program.

| Use case | Recommendation |
| --- | --- |
| Workflow conditions, routing rules, feature flags, access policies | **Move to Risor v3.** Static checking catches typos and null errors before rules run, with exact CPU and memory limits. |
| Templates and JSON/record queries | **Move to Risor v3.** Native `${{ }}` host templates, `[*]` projections, and 29 list methods run in Go, TypeScript, and Rust with identical results. |
| Config files and data transforms | **Move to Risor v3.** Deterministic evaluation, `type` lines, `as`/`is` checked conversion, and `SchemaOf` JSON Schema export. |
| Replacing `deepnoodle-ai/expr` or `expr-lang/expr` | **Move to Risor v3.** See [benchmarks](benchmarks.md) and [Part 1](#part-1-moving-from-expr). |
| Event-driven plugins or UI handlers where the host owns state | **Move to Risor v3** using [`export const` entry points](#4-event-loops-stored-callbacks-and-state-across-events). |
| Long-lived scripts with `for`/`while` loops, in-place mutation, internal event loops, or v2 scripting modules (`os`, `http`, `exec`) | **Stay on Risor v2.** Existing imports of `github.com/deepnoodle-ai/risor/v2` continue to work on the `v2` branch. See the [v2 support policy](versioning.md#5-support-policy-for-risor-v2). |

---

## Part 1: Moving from `expr`

This section covers both [`deepnoodle-ai/expr`](https://github.com/deepnoodle-ai/expr)
and [`expr-lang/expr`](https://github.com/expr-lang/expr).

### Side-by-side examples

Every Risor v3 snippet below is checked and evaluated against real inputs.

#### Compound rule

```text
// deepnoodle-ai/expr
order.total > 100 && order.country == "US" && contains(["admin", "owner"], user.role) && !order.flagged

// expr-lang/expr
order.total > 100 and order.country == "US" and user.role in ["admin", "owner"] and not order.flagged
```

<!-- risor-verify: {"env":{"varRecord":"{order: {total: float, country: string, flagged: bool}, user: {role: string}}"},"vars":{"order":{"total":250,"country":"US","flagged":false},"user":{"role":"admin"}},"expect":true} -->
```js
order.total > 100 && order.country == "US" && user.role in ["admin", "owner"] && !order.flagged
```

#### Filtering, mapping, and aggregating a list

```text
// deepnoodle-ai/expr
fetch.tests | filter(it.status == "fail") | map(it.ms) | sum()

// expr-lang/expr
sum(filter(fetch.tests, .status == "fail"), .ms)
```

<!-- risor-verify: {"env":{"varRecord":"{fetch: {tests: {name: string, status: string, ms: int}[]}}"},"vars":{"fetch":{"tests":[{"name":"login","status":"pass","ms":12},{"name":"upload","status":"fail","ms":340},{"name":"export","status":"fail","ms":160}]}},"expect":500} -->
```js
fetch.tests.filter(t => t.status == "fail").map(t => t.ms).sum()
```

#### Top-N sorting and projection

```text
// expr-lang/expr
map(take(sortBy(filter(fetch.tests, .status == "fail"), .ms, "desc"), 3), .name)
```

<!-- risor-verify: {"env":{"varRecord":"{fetch: {tests: {name: string, status: string, ms: int}[]}}"},"vars":{"fetch":{"tests":[{"name":"a","status":"fail","ms":50},{"name":"b","status":"pass","ms":900},{"name":"c","status":"fail","ms":420},{"name":"d","status":"fail","ms":180},{"name":"e","status":"fail","ms":310}]}},"expect":["c","e","d"]} -->
```js
fetch.tests
  .filter(t => t.status == "fail")
  .sortBy(t => t.ms, "desc")
  .slice(0, 3)[*].name
```

#### Optional chaining, defaults, and string interpolation

```text
// deepnoodle-ai/expr
sprintf("%s <%s>", coalesce(u?.name, "anon"), lower(u.email))

// expr-lang/expr
sprintf("%s <%s>", u?.name ?? "anon", lower(u.email))
```

<!-- risor-verify: {"env":{"varRecord":"{u: {name?: string | null, email: string}}"},"vars":{"u":{"email":"ADA@EXAMPLE.COM"}},"expect":"anon <ada@example.com>"} -->
```js
`${u.name ?? "anon"} <${u.email.toLowerCase()}>`
```

### Syntax and operator mapping

| Concept | `deepnoodle-ai/expr` / `expr-lang/expr` | Risor v3 |
| --- | --- | --- |
| Logical operators | `and`, `or`, `not` (or `&&`, `\|\|`, `!`) | `&&`, `\|\|`, `!` (word forms are not accepted) |
| Membership | `x in xs`, `x not in xs`, `contains(xs, x)` | `x in xs`, `x not in xs` (`!x in xs` is a parse error) |
| Substring check | `s contains sub`, `contains(s, sub)` | `sub in s` |
| Map key check | `"k" in m` | `"k" in m` |
| Null coalescing | `a ?? b`, `coalesce(a, b)` | `a ?? b` (also handles absent last field: `m.k ?? d`) |
| Optional chaining | `a?.b`, `a?.[k]` | `a?.b`, `a?.[k]` |
| Conditional | `c ? a : b`, `if(c, a, b)` | `c ? a : b` or `if (c) { a } else { b }` |
| Local binding | `let x = 1; x + 2` | `const x = 1; x + 2` |
| Predicates & transforms | `filter(xs, # > 0)`, `filter(xs, it.ok)`, `filter(xs, .ok)` | `xs.filter(x => x.ok)` (explicit parameter; no `#`, `it`, or `.field`) |
| Map | `map(xs, .name)`, `xs \| map(it.name)` | `xs.map(x => x.name)` or `xs[*].name` |
| All / any | `all(xs, ...)`, `any(xs, ...)` | `xs.every(x => ...)`, `xs.some(x => ...)` |
| None / one | `none(xs, ...)`, `one(xs, ...)` | `!xs.some(x => ...)`, `xs.count(x => ...) == 1` |
| Count | `count(xs, ...)` | `xs.count(x => ...)` |
| Sum / min / max | `sum(xs)`, `sum(xs, .ms)`, `min(xs)`, `max(xs)` | `xs.sum()`, `xs[*].ms.sum()`, `xs.min()`, `xs.max()` |
| Find / findIndex | `find(xs, ...)`, `findIndex(xs, ...)` | `xs.find(x => ...)`, `xs.findIndex(x => ...)` (returns `null`, not `-1`) |
| Sort / group | `sortBy(xs, .ms, "desc")`, `groupBy(xs, .team)` | `xs.sortBy(x => x.ms, "desc")`, `xs.groupBy(x => x.team)` |
| Slice / take / drop | `xs[0:3]`, `take(xs, 3)`, `xs[1:]` | `xs.slice(0, 3)`, `xs.slice(1)` |
| First / last / negative index | `first(xs)`, `last(xs)`, `xs[-1]` | `xs.first()`, `xs.last()` (`xs[-1]` fails with `index-range`) |
| Flatten / unique / reverse | `flatten(xs)`, `uniq(xs)`, `reverse(xs)` | `xs.flat()`, `xs.unique()`, `xs.reverse()` |
| Join / split | `join(xs, ",")`, `split(s, ",")` | `xs.join(",")`, `s.split(",")` |
| Map keys & values | `keys(m)`, `values(m)` | `m.keys()`, `m.values()`, `m.entries()` (sorted by code point) |
| Pipes | `xs \| filter(...) \| map(...)` | Method chaining `xs.filter(...).map(...)` or `const` bindings |
| String formatting | `sprintf("%s: %.2f", k, v)` | `` `${k}: ${fixed(v, 2)}` `` |
| Case & trimming | `lower(s)`, `upper(s)`, `trim(s)` | `s.toLowerCase()`, `s.toUpperCase()`, `s.trim()` |
| Prefix & suffix | `hasPrefix(s, p)`, `s startsWith p`, `trimPrefix(s, p)` | `s.startsWith(p)`, `s.endsWith(p)`, `s.removePrefix(p)`, `s.removeSuffix(p)` |
| Replace | `replace(s, old, new)` | `s.replaceAll(old, new)` |
| String index | `indexOf(s, sub)` | `s.indexOf(sub)` (returns `int \| null`, not `-1`) |
| Host templates | `Hello, ${user.name}` (`deepnoodle-ai/expr`) | `Hello, ${{ user.name }}` (escape literal as `$${{`) |
| Projection | `map(orders, .id)` | `orders[*].id` (and `orders?.[*].id` when `orders` may be null) |

### Semantic differences to watch out for

1. **Static type checking runs on `Compile` by default.**
   In Risor v3, `env.Compile(expr)` rejects typos (`user.nmae`), missing fields,
   and unchecked nulls (`user.email.toLowerCase()` when `email: string | null`)
   before the expression ever runs. Fix nullable reads with `?.` and `??`:

   <!-- risor-verify: {"env":{"varRecord":"{user: {email: string | null}}"},"vars":{"user":{"email":null}},"expect":"none"} -->
   ```js
   user.email?.toLowerCase() ?? "none"
   ```

   If you are migrating a large set of untyped rules incrementally, you can
   declare variables as `any` or pass `risor.NoCheck` to `Compile` (static name
   and syntax checks still run), then tighten variable types over time.

2. **`??` handles an absent last field, and requires parentheses with other operators.**
   `order.coupon ?? "none"` returns `"none"` whether `coupon` is `null` or
   missing from `order`. Only the **last** step before `??` is read this way:
   for an optional intermediate field, use `?.` at that step too
   (`order?.discount?.code ?? "NONE"`). Also, `??` cannot be mixed with `||`,
   `&&`, or comparison operators without parentheses:

   <!-- risor-verify: {"env":{"varRecord":"{order: {coupon?: string, discount?: {code?: string} | null, total?: float | null}}"},"vars":{"order":{"total":null}},"expect":{"coupon":"none","code":"NONE","large":false}} -->
   ```js
   {
     coupon: order.coupon ?? "none",
     code: order?.discount?.code ?? "NONE",
     large: (order.total ?? 0) > 100,
   }
   ```

3. **Not-found returns `null`, never `-1`.**
   `s.indexOf(sub)`, `xs.indexOf(x)`, `xs.findIndex(f)`, `xs.find(f)`,
   `xs.first()`, `xs.last()`, `xs.min()`, `xs.max()`, `xs.minBy(f)`, and
   `xs.maxBy(f)` all return `T | null`. Check with `!= null` or `??`:

   <!-- risor-verify: {"env":{"varRecord":"{path: string}"},"vars":{"path":"reports/2026/q1.csv"},"expect":"reports"} -->
   ```js
   const slash = path.indexOf("/");
   if (slash != null) { path.slice(0, slash) } else { path }
   ```

4. **No chained comparisons.**
   `0 < x < 10` is a parse error (`chained-comparison`). Write `0 < x && x < 10`:

   <!-- risor-verify: {"env":{"varRecord":"{x: int}"},"vars":{"x":5},"expect":true} -->
   ```js
   0 < x && x < 10
   ```

5. **Equality is strict and deep.**
   `==` and `!=` never coerce across kinds (`"1" == 1` and `1 == true` are
   `false`), and lists and maps compare deeply by value. There is no `===` or
   `!==`.

6. **Truthiness treats empty lists and empty maps as falsy.**
   Falsy values are `null`, `false`, `0`, `""`, `[]`, and `{}`. Everything else
   — including `"0"`, `"false"`, `[0]`, `{a: null}`, and opaque values — is
   truthy. In particular, `if (items) { ... } else { ... }` checks whether
   `items` is non-empty, which differs from JavaScript (where `[]` and `{}` are
   truthy).

7. **Narrowing `any` uses `is` and `as`.**
   Values of type `any` (such as `json.parse(...)` results or loosely typed host
   inputs) can be tested with `is` (which narrows inside `if`/`&&`/`?:`) or
   asserted with `as` (which fails with `not-conforming` and can be caught with
   `try`/`catch`):

   <!-- risor-verify: {"env":{"varRecord":"{payload: any}"},"vars":{"payload":{"id":"ord_1","total":42}},"expect":"ord_1:42"} -->
   ```js
   const order = payload as {id: string, total: float};
   `${order.id}:${order.total}`
   ```

8. **Exact, deterministic resource limits.**
   Every run enforces step, allocation-unit, depth, and optional deadline
   limits. Default limits are 1,000,000 steps, 1,000,000 allocation units, and
   depth 200 (max 400). Limit errors (`steps-exceeded`, `allocation-exceeded`,
   `depth-exceeded`, `deadline-exceeded`, `cancelled`) cannot be caught by
   `try`/`catch`.

9. **No built-in `matches` / regex operator.**
   Risor v3 does not ship regular expressions in 3.0. See
   [Regular expressions](#5-regular-expressions-regexp) for built-in string
   alternatives and host-function registration.

### Host-side Go migration from `expr`

#### Compiling and running over maps

Before (`expr-lang/expr`):

```go
program, err := expr.Compile(`order.total > 100 && user.role in ["admin", "owner"]`,
    expr.Env(envSchema),
    expr.AsBool(),
)
out, err := expr.Run(program, data)
allowed := out.(bool)
```

Before (`deepnoodle-ai/expr`):

```go
program, err := dexpr.Compile(`order.total > 100 && contains(["admin", "owner"], user.role)`,
    dexpr.WithBuiltins(),
    dexpr.WithCollectionFuncs(),
    dexpr.WithEvalBudget(100_000),
)
out, err := program.Run(ctx, data)
```

After (Risor v3):

```go
import risor "github.com/deepnoodle-ai/risor/go/v3"

env, err := risor.NewEnv(
    risor.VarRecord(`{
        order: {total: float, country: string, flagged: bool},
        user: {role: string},
    }`),
    risor.Limits{Steps: 100_000, Allocation: 100_000},
)
expr, err := risor.Parse(`order.total > 100 && user.role in ["admin", "owner"]`)
prog, err := env.Compile(expr, risor.Condition) // or risor.Expect("bool")

out, err := prog.Run(ctx, risor.Map(data))
allowed := risor.Truthy(out)
```

Build `env` once and reuse it across compiles; compile each expression once and
share `*risor.Program` concurrently across goroutines.

#### Using Go structs (`TypeOf` and the typed layer)

Instead of passing a prototype struct value to `expr.Env(RootEnv{})`, derive a
Risor type directly from your Go struct with `risor.TypeOf[T]()`. Risor v3 reads
structs in place using `json` struct tags and activates the reflection-free
typed layer:

```go
type Order struct {
    Total   float64 `json:"total"`
    Country string  `json:"country"`
    Flagged bool    `json:"flagged"`
}

orderType, err := risor.TypeOf[Order]()
env, err := risor.NewEnv(risor.VarType("order", orderType))
expr, err := risor.Parse(`order.total > 100 && !order.flagged`)
prog, err := env.Compile(expr, risor.Expect("bool"))

out, err := prog.Run(ctx, risor.Map{"order": Order{Total: 250, Country: "US"}})
```

#### Analyzing dependencies before running

Hosts that inspect which variables, paths, or functions a rule reads before
fetching data use `expr.Analyze()` — it requires no environment and never runs
the program:

```go
expr, err := risor.Parse(`order.total > 100 && customer.tier == "gold"`)
report := expr.Analyze()
// report.Names -> ["customer", "order"]
// report.Refs  -> [{Name: "order", Path: [{Key: "total"}]}, {Name: "customer", Path: [{Key: "tier"}]}]
// report.Calls -> every free function or namespace call with argument spans
```

#### Host templates (`${{ }}`)

Replace `deepnoodle-ai/expr`'s `NewTemplate` (`${...}`) with
`risor.ParseTemplate` (`${{ ... }}`):

<!-- risor-verify: {"mode":"template","templateMode":"string","env":{"varRecord":"{user: {name: string, role: string}}"},"vars":{"user":{"name":"Ada","role":"admin"}},"expect":"Hello, Ada (admin)!"} -->
```text
Hello, ${{ user.name }} (${{ user.role }})!
```

```go
tmpl, err := risor.ParseTemplate(`Hello, ${{ user.name }} (${{ user.role }})!`)
prog, err := env.Compile(tmpl.Program(risor.ModeString))
text, err := prog.Run(ctx, risor.Map{"user": user})
```

For config fields where a single template part should keep its typed value
(for example `${{ order.total }}` returning a `float64` instead of `"250"`),
compile `tmpl.Program(risor.ModeValue)`.

#### Custom functions

Use `risor.Func` to expose Go functions without manually writing type wrappers:

```go
env, err := risor.NewEnv(
    risor.Var("title", "string"),
    risor.Func("slugify", func(s string) string {
        return strings.ReplaceAll(strings.ToLower(strings.TrimSpace(s)), " ", "-")
    }, risor.Params("s")),
)
```

---

## Part 2: Moving from Risor v2

Risor v2 is an imperative scripting language with `let`/`var`, `for` loops,
mutable lists and maps, `function` statements, `return`, and built-in modules
(`os`, `http`, `json`, `strings`, `math`, `time`). Risor v3 replaces that with a
pure, statically typed expression language.

### Language form mapping

| Risor v2 | Risor v3 | Notes |
| --- | --- | --- |
| `x := 1` / `let x = 1` / `var x = 1` | `const x = 1;` | Bindings are immutable and terminated by `;`. |
| `nil` | `null` | `nil` is reserved and gives a targeted error. |
| `function add(a, b) { return a + b }` | `const add = (a: int, b: int): int => a + b;` | Lambdas are expressions; use `export const` for host-callable entry points. |
| `if x > 0 { "pos" }` | `if (x > 0) { "pos" } else { "non-pos" }` | Parentheses around condition and `else` branch are required. |
| `switch status { case "ok": 1 default: 0 }` | `match status { "ok" => 1, _ => 0 }` | Supports `\|` multi-literals and `if (guard)` guards. |
| `try { ... } catch e { ... }` | `try { ... } catch (err) { ... }` | `err` is `{code, message, data}`. Limits cannot be caught. |
| `error("bad")` / `throw` | `fail("bad")` or `fail("bad", {field: "x"})` | Raises code `failed` with optional structured `data`. |
| `sprintf("%s: %.2f", k, v)` | `` `${k}: ${fixed(v, 2)}` `` | Template literals replace `printf`/`sprintf`. |
| `a \|> f \|> g` | `a.f().g()` or `const b = f(a); g(b)` | `\|>` is not in v3; chain methods or bind with `const`. |
| `keys(m)`, `values(m)` | `m.keys()`, `m.values()`, `m.entries()` | Methods on maps; always in ascending code-point key order. |
| `strings.to_lower(s)`, `strings.trim_space(s)` | `s.toLowerCase()`, `s.trim()` | String operations are methods on `string`. |
| `strings.contains(s, sub)` | `sub in s` | `in` works on strings, lists, and maps. |
| `strings.has_prefix(s, p)` | `s.startsWith(p)` | Also `s.endsWith(p)`, `s.removePrefix(p)`, `s.removeSuffix(p)`. |
| `strings.replace_all(s, a, b)` | `s.replaceAll(a, b)` | Literal replacement (no regex). |
| `json.marshal(x)` / `json.unmarshal(s)` | `json.stringify(x)` / `json.parse(s)` | Opt-in `json` group (`risor.Groups("json")`). |
| `math.sqrt(x)`, `math.abs(x)` | `math.sqrt(x)` (opt-in `math`), `abs(x)` (core) | `abs`, `floor`, `ceil`, `round`, `min`, `max` are core functions; trig/log/pow/sqrt are in opt-in `math`. |
| `time.parse(...)`, `time.now()` | `time.parse(s)`, `time.format(t)`, `time.duration(s)` | Opt-in `time` group; there is no `time.now()` — pass current time as a variable. |
| `base64.encode(s)` / `base64.decode(s)` | `base64.encode(s)` / `base64.decode(s)` | Opt-in `base64` group (`risor.Groups("base64")`). |
| `list.append(xs, x)` | `[...xs, x]` | Lists are immutable; use spread syntax. |
| `len(x)`, `int(x)`, `float(x)`, `string(x)`, `bool(x)`, `type(x)` | Same names in core | `type(x)` returns `"null"`, `"bool"`, `"number"`, `"string"`, `"list"`, `"map"`, or the opaque type name. |

#### Common foreign names and automatic hints

Risor v3 ships a built-in foreign-names table (`suite/foreign-names.json`) and
alias lists on every library entry, so writing a v2, `expr`, or JavaScript name
produces a compile diagnostic pointing straight to the v3 spelling:

| What you might write | Risor v3 equivalent |
| --- | --- |
| `xs.length` / `xs.size` | `len(xs)` |
| `xs.includes(x)` / `xs.contains(x)` / `m.has(k)` | `x in xs` / `k in m` |
| `xs.push(x)` / `xs.unshift(x)` / `xs.concat(ys)` | `[...xs, x]` / `[x, ...xs]` / `[...xs, ...ys]` |
| `xs.pop()` / `xs.shift()` | `xs.last()` + `xs.slice(0, -1)` / `xs.first()` + `xs.slice(1)` |
| `Object.keys(m)` / `Object.values(m)` / `Object.entries(m)` | `m.keys()` / `m.values()` / `m.entries()` |
| `Object.assign(a, b)` | `{...a, ...b}` |
| `Object.fromEntries(xs)` | `xs.toMap()` (on `{key: string, value: T}[]`) |
| `Array.isArray(x)` / `typeof x` | `type(x) == "list"` / `type(x)` |
| `Math.floor(x)` / `Math.round(x)` / `Math.abs(x)` | `floor(x)` / `round(x)` / `abs(x)` (core) |
| `Math.sqrt(x)` / `Math.pow(x, y)` | `math.sqrt(x)` / `math.pow(x, y)` (opt-in `math`) |
| `JSON.stringify(x)` / `JSON.parse(s)` | `json.stringify(x)` / `json.parse(s)` (opt-in `json`) |
| `parseInt(s)` / `parseFloat(s)` / `toString(x)` | `int(s)` / `float(s)` / `string(x)` |
| `s.replace(a, b)` / `s.substring(a, b)` | `s.replaceAll(a, b)` / `s.slice(a, b)` |
| `xs.any(f)` / `xs.all(f)` / `xs.flatten()` / `xs.uniq()` | `xs.some(f)` / `xs.every(f)` / `xs.flat()` / `xs.unique()` |
| `undefined` / `nil` | `null` |
| `print(...)` / `println(...)` / `console.log(...)` | No stdout — return the value you want the host to receive |
| `Date.now()` / `time.now()` | No clock — pass the timestamp from the host as a variable |
| `await f()` | `f()` — host calls are synchronous and return values directly |

### What has no direct form — and how to rewrite it

#### 1. `for` loops and accumulation

Risor v3 has no `for` or `while` loops. Most loops in v2 scripts fall into one
of four patterns:

**A. Filtering, transforming, and summing**

Before (Risor v2):

```text
total := 0
for _, item := range order.items {
    if item.active {
        total += item.price * item.qty
    }
}
total
```

After (Risor v3):

<!-- risor-verify: {"env":{"varRecord":"{order: {items: {price: float, qty: int, active: bool}[]}}"},"vars":{"order":{"items":[{"price":12.5,"qty":2,"active":true},{"price":99,"qty":1,"active":false},{"price":5,"qty":3,"active":true}]}},"expect":40} -->
```js
order.items
  .filter(item => item.active)
  .map(item => item.price * item.qty)
  .sum()
```

**B. Grouping, counting, indexing, and finding extremes**

Before (Risor v2):

```text
counts := {}
for _, ev := range events {
    counts[ev.kind] = (counts[ev.kind] ?? 0) + 1
}
```

After (Risor v3) — use `countBy`, `groupBy`, `indexBy`, `minBy`, and `maxBy`:

<!-- risor-verify: {"env":{"varRecord":"{events: {id: string, kind: string, ms: int}[]}"},"vars":{"events":[{"id":"e1","kind":"http","ms":40},{"id":"e2","kind":"db","ms":120},{"id":"e3","kind":"http","ms":15}]},"expect":{"byKind":{"db":1,"http":2},"slowest":"e2","byId":{"e1":{"id":"e1","kind":"http","ms":40},"e2":{"id":"e2","kind":"db","ms":120},"e3":{"id":"e3","kind":"http","ms":15}}}} -->
```js
{
  byKind: events.countBy(e => e.kind),
  slowest: events.maxBy(e => e.ms)?.id ?? "none",
  byId: events.indexBy(e => e.id),
}
```

For custom accumulation that does not fit a dedicated method, use `reduce`:

<!-- risor-verify: {"env":{"varRecord":"{nums: int[]}"},"vars":{"nums":[2,3,4]},"expect":24} -->
```js
nums.reduce((product, n) => product * n, 1)
```

**C. Walking trees of arbitrary depth (`walk`)**

Before (Risor v2):

```text
names := []
function collect(node) {
    names.append(node.name)
    for _, child := range node.children {
        collect(child)
    }
}
collect(root)
names
```

After (Risor v3) — `walk(root, children)` returns every node in pre-order,
bounded by the depth and step limits:

<!-- risor-verify: {"env":{"namedTypes":[{"name":"Node","type":"{name: string, children: Node[]}"}],"vars":{"root":"Node"}},"vars":{"root":{"name":"root","children":[{"name":"a","children":[{"name":"a1","children":[]}]},{"name":"b","children":[]}]}},"expect":["root","a","a1","b"]} -->
```js
walk(root, n => n.children)[*].name
```

**D. General recursive logic (typed self-recursion)**

When you need per-node state (such as indentation or a running total across a
tree), declare a `const` lambda with parameter and return types so it can call
itself:

<!-- risor-verify: {"env":{"vars":{"thread":"{author: string, text: string, replies: any[]}"}},"vars":{"thread":{"author":"Ada","text":"Ship it","replies":[{"author":"Grace","text":"LGTM","replies":[]}]}},"expect":"Ada: Ship it\n  Grace: LGTM\n"} -->
```js
type Comment = {author: string, text: string, replies: Comment[]};

const render = (c: Comment, indent: string): string =>
  `${indent}${c.author}: ${c.text}\n` +
  c.replies.map(r => render(r, indent + "  ")).join("");

render(thread as Comment, "")
```

#### 2. Mutation (`push`, `m[k] = v`, `delete`, `x++`)

Every value in Risor v3 is immutable. Build updated lists and maps with spread
syntax (`...`), `slice`, `mapValues`, and `entries().filter(...).toMap()`:

Before (Risor v2):

```text
tags.append("verified")
config["timeout"] = 30
delete(config, "debug")
```

After (Risor v3):

<!-- risor-verify: {"env":{"varRecord":"{tags: string[], config: map<int>}"},"vars":{"tags":["beta"],"config":{"retries":3,"debug":1,"timeout":10}},"expect":{"tags":["beta","verified"],"config":{"retries":3,"timeout":30}}} -->
```js
const nextTags = [...tags, "verified"];
const nextConfig = {
  ...config.entries().filter(e => e.key != "debug").toMap(),
  timeout: 30,
};
{tags: nextTags, config: nextConfig}
```

To transform every value in a map without mutating it in a loop, use
`mapValues`:

<!-- risor-verify: {"env":{"varRecord":"{prices: map<float>}"},"vars":{"prices":{"book":10,"pen":2}},"expect":{"book":11,"pen":2.2}} -->
```js
prices.mapValues(p => p * 1.1)
```

#### 3. Multiple statements and early `return`

A v3 program is a sequence of `const` (and `type`) lines followed by **one**
expression — or one or more `export const` entry points. Replace early `return`
branches with `if`/`else if`/`else` or `match` expressions (which may include
`if (guard)` guards):

Before (Risor v2):

```text
function grade(score) {
    if score >= 90 {
        return "A"
    }
    if score >= 80 {
        return "B"
    }
    return "C"
}
grade(input.score)
```

After (Risor v3):

<!-- risor-verify: {"env":{"varRecord":"{input: {score: int}}"},"vars":{"input":{"score":85}},"expect":"B"} -->
```js
const grade = (score: int): string => match score {
  _ if (score >= 90) => "A",
  _ if (score >= 80) => "B",
  _ => "C",
};
grade(input.score)
```

Inside any `if` branch, `try`/`catch` branch, or lambda body, you can open a
scoped block with local `const` bindings ending in one expression:
`{ const x = ...; x * 2 }`.

Error handling follows the same expression shape — `fail(message, data?)`
raises a value error, and `try`/`catch` produces a fallback value (note that
resource limit errors cannot be caught):

<!-- risor-verify: {"env":{"groups":["json"],"varRecord":"{raw: string}"},"vars":{"raw":"not-json"},"expect":{"ok":false,"code":"invalid-argument"}} -->
```js
try {
  {ok: true, value: json.parse(raw)}
} catch (err) {
  {ok: false, code: err.code}
}
```

#### 4. Event loops, stored callbacks, and state across events

Risor v3 programs never hold state between runs and cannot hand closures back to
the host. Instead, follow the **long-lived host pattern**
([`spec/core/03-host.md` 3.2.5](../spec/core/03-host.md#325-hosts-that-live-long-informative)):

1. The **host owns all state**.
2. Compile the program **once**, exposing named `export const` entry points.
3. On each event, call one entry point with the current state and event, get
   back the next state and a list of typed actions, and apply the actions in
   host code. Each event runs under its own step, allocation, and deadline
   limits.

<!-- risor-verify: {"mode":"entry","entry":"onEvent","args":[{"count":2,"paused":false},{"kind":"add","delta":3}],"expect":{"state":{"count":5,"paused":false},"actions":[{"kind":"notify","text":"count is 5"}]}} -->
```js
type State = {count: int, paused: bool};
type Event = {kind: "add", delta: int} | {kind: "toggle"};
type Action = {kind: "notify", text: string};
type Result = {state: State, actions: Action[]};

export const onEvent = (state: State, ev: Event): Result =>
  match ev.kind {
    "toggle" => {
      state: {...state, paused: !state.paused},
      actions: [],
    },
    "add" => if (state.paused) {
      {state, actions: []}
    } else {
      const next = {...state, count: state.count + ev.delta};
      {state: next, actions: [{kind: "notify", text: `count is ${next.count}`}]}
    },
  };
```

#### 5. Regular expressions (`regexp`)

Risor v3 reserves the `regexp` namespace for a future version and does not ship
regular expressions in 3.0. Most string checks rewrite directly with built-in
methods:

<!-- risor-verify: {"env":{"varRecord":"{filename: string, email: string}"},"vars":{"filename":"report-2026-04.pdf","email":"ada@deepnoodle.dev"},"expect":{"isReport":true,"domain":"deepnoodle.dev"}} -->
```js
const parts = email.split("@");
{
  isReport: filename.startsWith("report-") && filename.endsWith(".pdf"),
  domain: if (len(parts) == 2) { parts[1] } else { "" },
}
```

When you genuinely need regex matching, expose a Go `regexp` helper as a host
function (using a non-reserved namespace such as `re.match`) and charge work
proportional to input size through `*risor.Call`:

```go
reMatch := risor.Func("re.match", func(c *risor.Call, pattern, text string) (bool, error) {
    if err := c.Charge(int64(len(pattern) + len(text))); err != nil {
        return false, err
    }
    return regexp.MatchString(pattern, text)
}, risor.Params("pattern", "text"))
```

#### 6. Printing, clocks, and `await`

- **No `print` / `println` / `console`:** a program's only output is its return
  value. If you need both a verdict and diagnostic messages, return a record
  like `{ok: false, reasons: [...]}`.
- **No `time.now()` / `Date.now()`:** runs are deterministic. Pass the current
  timestamp as a variable (Unix seconds as `float`), then combine it with the
  opt-in `time` group (`time.parse`, `time.format`, `time.duration`):

  <!-- risor-verify: {"env":{"groups":["time"],"varRecord":"{now: float, expiresAt: string}"},"vars":{"now":1704067200,"expiresAt":"2024-01-02T00:00:00Z"},"expect":true} -->
  ```js
  time.parse(expiresAt) - now > time.duration("12h")
  ```

- **No `await`:** host functions are synchronous from the program's point of
  view (they receive `c.Context()` for cancellation). Remove `await` and call
  the function directly.

### Host-side Go migration from Risor v2

The Go module path changed from `github.com/deepnoodle-ai/risor/v2` to
`github.com/deepnoodle-ai/risor/go/v3`.

| Risor v2 | Risor v3 |
| --- | --- |
| `risor.Eval(ctx, code, risor.WithEnv(m))` | `env, _ := risor.NewEnv(...)`; `expr, _ := risor.Parse(code)`; `prog, _ := env.Compile(expr)`; `prog.Run(ctx, risor.Map(m))` |
| `risor.WithMaxSteps(n)` + `context.WithTimeout` | `risor.Limits{Steps: n, Allocation: u, Depth: d, Deadline: dur}` |
| `risor.WithRawResult()` / `object.Object` | Results are plain Go values (`nil`, `bool`, `float64`, `string`, `[]any`, `map[string]any`, or your opaque type). Helpers: `risor.AsInt`, `risor.AsFloat`, `risor.Truthy`, `risor.JSONText`. |
| Built-in `json`, `time`, `math`, `base64` modules | Opt-in library groups: `risor.Groups("json", "time", "math", "base64")`. Remove core entries with `risor.Without("string.repeat")`. |
| `object.NewBuiltin(...)` | `risor.Func("name", goFn, risor.Params(...))` or `risor.Function{Name, Type, Impl}` |
| Custom `object.Object` types | `risor.Methods[*MyType]("MyType")` or `risor.OpaqueType` (implement `OpaqueType() string`) |
| Calling one script with many globals | `export const` entry points: `prog.Entries()`, `prog.Call(ctx, name, args, vars)`, `prog.CallRecord(ctx, name, input, vars)` |
| Runtime type checks / tool schemas | `risor.Conform(val, typ)`, `risor.ConformAll(val, typ, max)`, `risor.SchemaOf(typ)`, `risor.VarSchema(...)` |

#### Before and after: embedding with limits, functions, and opaque values

Before (Risor v2):

```go
result, err := risor.Eval(ctx, code,
    risor.WithEnv(map[string]any{
        "account": accountObj,
        "amount":  500,
    }),
    risor.WithMaxSteps(50_000),
)
```

After (Risor v3):

```go
import (
    "context"
    "time"

    risor "github.com/deepnoodle-ai/risor/go/v3"
)

type Account struct {
    Owner string
    Cents int64
}

func (*Account) OpaqueType() string        { return "Account" }
func (a *Account) Name() string            { return a.Owner }
func (a *Account) Balance() int64          { return a.Cents }
func (a *Account) CanPay(cents int64) bool { return cents <= a.Cents }

env, err := risor.NewEnv(
    risor.Groups("json", "time"),
    risor.Limits{
        Steps:      50_000,
        Allocation: 50_000,
        Depth:      100,
        Deadline:   100 * time.Millisecond,
    },
    risor.Methods[*Account]("Account", risor.ForMethod("CanPay", risor.Params("cents"))),
    risor.Var("account", "Account"),
    risor.Var("amount", "int"),
    risor.Func("audit.tag", func(c *risor.Call, owner string) (string, error) {
        if err := c.Charge(10); err != nil {
            return "", err
        }
        return "acct:" + owner, nil
    }, risor.Params("owner")),
)

expr, err := risor.Parse(`{
    tag: audit.tag(account.name()),
    approved: account.canPay(amount),
}`)
prog, err := env.Compile(expr)
out, usage, err := prog.Measure(ctx, risor.Map{
    "account": &Account{Owner: "Ada", Cents: 1250},
    "amount":  500,
})
// usage.Steps and usage.Allocated report exact consumption
```

#### Calling `export const` entry points and exporting JSON Schema

```go
expr, err := risor.Parse(`
/** Decide whether to approve a transfer. */
export const approve = (
    /** Transfer amount in cents. */
    amount: int,
    /** Two-letter country code. */
    country: string,
): bool => amount <= 10_000 && country == "US";
`)
prog, err := env.Compile(expr)

// Inspect entry points and export JSON Schema for forms or LLM tool calls:
for _, entry := range prog.Entries() {
    if entry.InputType != nil {
        schema, err := risor.SchemaOf(*entry.InputType)
        _ = schema
    }
}

// Call positionally or with a JSON-like record (conformed automatically):
ok, err := prog.CallRecord(ctx, "approve", map[string]any{
    "amount":  2500,
    "country": "US",
}, nil)
```

To validate arbitrary host data against a Risor type outside a run, use
`risor.Conform(val, typ)` (or `risor.ConformAll(val, typ, 20)` to collect
multiple errors with paths).

---

## Migration checklist

1. **Audit your expressions and scripts.** Separate pure expressions, rules,
   templates, and stateless handlers (move to v3) from long-lived imperative
   scripts (keep on v2).
2. **Declare your environment types.** Use `risor.TypeOf[T]()` for Go structs,
   `risor.VarRecord` / `risor.Var` for maps, and `risor.VarSchema` if you
   already have JSON Schemas. Turn on any needed opt-in groups (`json`, `time`,
   `math`, `base64`).
3. **Compile once, check early.** Wire `risor.Parse` + `env.Compile` into your
   save/validation path (or CI via `risor check`) so typos, missing fields, and
   unchecked nulls fail before runtime.
4. **Update syntax and library calls.** Replace `and`/`or`/`not`, `let`/`var`,
   implicit `#`/`it`/`.field`, `|>` pipes, `for` loops, and mutating methods
   using the tables above — or let the v3 checker's built-in diagnostics point
   out each rename.
5. **Set explicit limits and pin the spec version.** Configure `risor.Limits`
   and `risor.Version("3.0")` when storing user-authored programs.
6. **Verify behavior with conformance tests.** Run your migrated expressions
   against golden inputs (or `risor test` case files) to confirm parity before
   switching traffic.
