# Risor v3 design: querying, sorting, grouping and aggregating data

Status: sketch, with its recommendation accepted by the owner (D51 in
`00-overview.md`): candidate L (5.1) and candidate P (5.2) are in 3.0, with
13-2 and 13-3 as recommended, and `02` to `06` are revised to match. Candidate
C may follow in a minor version. Candidate Q is not planned. This sets out a
workload, what the design did with it before, a control and three candidate
additions, and a recommendation. Sections 5.1 and 5.2 are the rules that were
applied; `02`, `03`, `04` and `06` now hold their exact form. Nothing was built
and no cost was measured.
Decision numbers such as D34 and B5 refer to `00-overview.md`.

**Evidence.** **(R)** read in a sibling document at the place cited. **(W)**
checked for this draft in the language's own documentation. **(ran)** run in
Node 22 for this draft, as a parse test; TypeScript itself was not run. **(M)**
from memory. **(I)** inferred. `02:` to `06:` are the sibling documents.

## 1. Summary

1. **The request.** Language-level support for queries, sorting, grouping and
   aggregation over data shaped like JSON, with Rego and JSONata as inspiration,
   in a form that composes with the rest of v3.
2. **What today's design does.** Section 2 writes twelve queries with only what
   `02` and `06` give. Four are good, three are fair and five are clumsy. None
   is impossible.
3. **Most of the trouble is in the library, not the syntax.** Sort has no
   direction. The only way from a map to a map is `entries` and `toMap`, so
   every aggregate over groups pays that twice. There is no `minBy`. And
   `filter` narrows nothing, so `xs.filter(x => x != null)` still has elements
   that may be null.
4. **One thing no library entry gives:** a path through nested lists with no
   lambda and no made-up name at each level, as in JSONata, JSONPath, JMESPath
   and Rego.
5. **The candidates.** L, the control: seven library entries and one checker
   rule, with no new syntax. P: L and one new step, `orders[*].lines[*].sku`. C:
   L and two comprehensions, `[for (o of orders) if (o.open) o.id]`. Q: a query
   expression, `from o in orders where ... order by ... select ...`.
6. **Every new form is a syntax error today,** in TypeScript and in v3. So no
   text changes meaning, and any of them can be added in any version without a
   break. Nothing here has to be reserved before 3.0.
7. **Recommendation.** L and P in 3.0. C in a minor version, if ported programs
   show queries that need two names in scope at once. Q is not planned. An
   implicit parameter, a pipe, quantifier words and a filter step: never.
8. **Size (I).** L is about 230 lines in each implementation and P about 170
   more. C would add about 360 and Q about 880.
9. **Three decisions for the owner** (6.2): the direction, what a projection
   covers, and the library set.

## 2. The workload, in v3 as it is designed today

The data. `body` is the result of `json.parse` and has no type.

```ts
orders:    {id: string, customer: string, status: "open" | "paid" | "void",
            placed: float, lines: {sku: string, qty: int, price: float}[]}[]
customers: {id: string, name: string}[]
events:    {kind: string, user: string | null, ms: int}[]
services:  map<{owner: string, deps: string[]}>
body:      any       // {"items": [{"name": "a", "tags": ["x", "y"]}, ...]}
```

Each query below uses only the grammar of `02:2.2` and the entries of `06:2.8`.

| # | Query | v3 today | Verdict |
| --- | --- | --- | --- |
| 1 | Filter and project: the ids of the open orders | `orders.filter(o => o.status == "open").map(o => o.id)` | Good |
| 2 | Reach through nested lists: every sku ordered; every tag in `body` | `orders.flatMap(o => o.lines).map(l => l.sku)`; `body.items.flatMap(i => i.tags)` | Fair. A lambda and a made-up name for each level |
| 3 | Look up a key, with a default | `services?.billing?.owner ?? "nobody"`; with the key in a name, `services?.[name]?.owner ?? "nobody"` | Good |
| 4 | Sort by one key; top N: the three slowest events | `events.sortBy(e => -e.ms).slice(0, 3)` | Good for a number. A string key cannot be negated |
| 5 | Sort by two keys, mixed direction: customer descending, then `placed` ascending | `orders.sortBy(o => -o.placed).sortBy(o => o.customer).reverse()` | Clumsy. Correct, and a puzzle: the minor key is sorted the wrong way on purpose, because `reverse` also turns equal elements round |
| 6 | Distinct: the users seen, without null | `events.flatMap(e => e.user == null ? [] : [e.user]).unique()` | Clumsy. The obvious `events.map(e => e.user).filter(u => u != null).unique()` is `arg-mismatch`: the elements are still `string \| null`, which is not `ordered` |
| 7 | Group and count: events of each kind | `events.groupBy(e => e.kind).entries().map(({key, value}) => {key, value: len(value)}).toMap()` | Clumsy. Four calls, and every group is built in order to be counted |
| 8 | Group and sum: revenue for each customer | `orders.groupBy(o => o.customer).entries().map(({key, value}) => {key, value: value.flatMap(o => o.lines).map(l => l.qty * l.price).sum()}).toMap()` | Clumsy, in the same way |
| 9 | Min and max by a field: the slowest event | `events.sortBy(e => -e.ms).first()` | Fair. A whole sort to find one element: the sort formula in steps, and `1 + n` units |
| 10 | Any and every | `orders.some(o => o.lines.some(l => l.qty > 100))`; `services.values().every(s => s.owner)` | Good. Both stop early |
| 11 | Join two lists by key: each order with its customer's name | `{ const byId = customers.map(c => {key: c.id, value: c.name}).toMap(); orders.map(o => {id: o.id, customer: byId?.[o.customer] ?? "unknown"}) }` | Fair. Linear. The shorter form, `customers.find(...)` inside the `map`, is `n × m` steps |
| 12 | Reshape a map: each owner with the names of that owner's services | `services.entries().groupBy(e => e.value.owner).entries().map(({key, value}) => {key, value: value.map(e => e.key)}).toMap()` | Clumsy. Two round trips through `entries` |

What the baseline shows:

- **Filter, map, flatten, `some`, `every` and the null operators are already
  good.** They are what a TypeScript author writes first, and `?.` with `??` is
  a better lookup with a default than a `get(k, d)` method.
- **Nothing is impossible,** because `reduce`, `entries` and `toMap` reach
  everything. But the general route is costly: a map built by `reduce` and
  spread is about `n × n / 2` steps (`06:2.8`) (R).
- **The clumsy rows have three causes.** Sort takes one key and has no direction
  (5, and 4 for strings). A map is transformed only through `entries` and
  `toMap` (7, 8, 12). `filter` does not narrow the element type (6).
- **`06` gives advice that fails when combined.** "Descending order is
  `.reverse()` after the sort" and "two keys are two stable sorts, the minor key
  first" (`06:2.8`) (R) together give row 5 the wrong answer: `reverse` undoes
  the minor key. And `sortBy(f).first()` for the least element has no partner
  for the greatest: `sortBy(f).last()` takes the last of equal elements, where
  `xs.max()` takes the first.

## 3. What other languages do

| Language | Form | What it buys | Known surprise |
| --- | --- | --- | --- |
| JSONata | A path step that maps: `Account.Order.Product.Price` (W) | Nested lists in one path, with no names | The mapping is implicit in `.`. "A result sequence will never contain child sequences", and a sequence of one "is considered equivalent to that value itself" (W). So the shape of a result depends on its length, and authors append `[]` to force a list (`01:2.5`) (R) |
| JSONata | Predicate `Product[Price > 10]` (M). Order `Product^(>Price, <Quantity)` (W). Grouping `Product{Category: $sum(Price)}` (form W, example M). Context `$` (M) | Filter, a sort with several keys and directions, and group with aggregate, each in one token | A number in the brackets is an index and anything else is a filter (W). A name in a predicate is a field of the context, so a field and a variable compete. `orders[price > 10]` is valid TypeScript with another meaning (ran) |
| JSONPath (RFC 9535) | `$.store.book[*].author`, `$..book[?@.price<10]` (W) | An explicit wildcard, and one flat result | A missing member or a value of the wrong kind selects nothing, with no error (W) |
| JMESPath | `[*]`, `[?expr]`, `[]` to flatten, `\|` to end a projection, `sort_by(xs, &age)`, `max_by` (W) | An explicit projection, with no lambda | A projection goes on over every later step until a pipe. Null results are dropped from it. A wildcard on the wrong kind is null (W). It has no grouping (W) |
| jq | `.[]`, `map(select(f))`, `group_by(f)`, `sort_by(.a, .b)`, `unique_by`, `max_by`, `to_entries`, `with_entries` (W) | Everything is a filter in a pipe. One total order over all kinds (W) | An expression gives a stream of results, not a value. `01:2.5` quotes an author who gave up learning it (R) |
| Rego | `some x in xs`; `every x in xs { ... }`; `[t \| body]`, `{t \| body}`, `{k: v \| body}`; `sites[_].servers[_].hostname` (W) | Several generators with every name in scope. Sets and maps built in place | An undefined reference makes the whole expression undefined, silently (W; `01:2.7`). Whether `xs[i]` iterates or indexes depends on whether `i` is bound elsewhere (M). `[t \| x in xs, c]` is valid TypeScript (ran) |
| expr | `filter(tweets, len(.Content) > 240)`, `{# % 2 == 0}`, `sortBy(users, .Age, "desc")`, `groupBy(users, .Age)`, `reduce(1..9, #acc + #)` (W) | The shortest predicates | `#` is the innermost element, so a nested predicate cannot reach the outer one. `02:2.9` removed this form (R) |
| C# LINQ | `from c in countries where ... orderby c.Area, c.Population descending select c`; `group c by k into g`; `join p in products on cat equals p.Category` (W) | The name is bound first, so an editor knows its type. Sort keys with directions. Every range name in scope | Two syntaxes for one thing, query and method. A query is deferred: the variable "stores no actual result data" (W) |
| Python | `[o.id for o in orders if o.open]`, `{k: v for ...}` (M) | The best-known comprehension | The result is written before the name it uses (M) |
| Kotlin | `orders.filter { it.open }.map { it.id }`, `sortedWith(compareBy({ it.a }, { it.b }))`, `associateBy`, `maxByOrNull` (M) | An implicit `it`, and a whole family of `By` functions | A nested lambda's `it` hides the outer one (M) |

Three things follow.

1. Two features are liked wherever they exist: a path through nested lists
   (JSONata, JSONPath, JMESPath, Rego), and generators with every name in scope
   (Rego, LINQ, Python).
2. Each language's surprises come from what is implicit: mapping without a mark,
   names that mean "the current element", a result that is unwrapped, a mistake
   that becomes null or nothing. An explicit form can have the gain without
   them.
3. Sort direction and grouping are library functions in jq, JMESPath, expr and
   Kotlin, and syntax only in JSONata and LINQ.

## 4. Constraints every candidate must meet

| # | Constraint | What it means here |
| --- | --- | --- |
| K1 | Text valid in TypeScript means what it means there (`02:3.1`) | A new form must be a syntax error in TypeScript. That rules out `orders[status == "open"]`, `orders[o => o.open]` and Rego's `[t \| x in xs, c]`: all three parse as JavaScript (ran) |
| K2 | A lambda goes only to the library (D34). No loop, no recursion (E6) | No new place for a lambda. A form that repeats must make one pass over a list that exists, with nothing carried from one pass to the next |
| K3 | Truthiness decides what a filter keeps (B5) | In every candidate a filter on `l.qty` drops a line whose `qty` is `0`, and a filter on `e.user` drops null and `""`. To keep them, test `!= null`. A test that can only go one way is a constant condition (`03:2.8`). This is today's rule for `filter` (`06:2.9`) and no candidate changes it |
| K4 | Null and missing data | A mistake is an error, or null where the author wrote `?.`. Nothing is dropped. The shape of a result does not depend on its length: no value is unwrapped, and no list is flattened unless the text says so |
| K5 | The checker (`03`) | A type rule over typed data, and a stated set of `dynamic` sites over `any`. A test should narrow what it guards |
| K6 | The cost model (`04:A.3`, `A.4`) | Exact steps. Units charged before a value is built. A result no larger than the steps already charged allow |
| K7 | Order | A sort key is all numbers, by value, or all strings, by code point. A mix or a null is an error, as for `<` (`02:2.7`). Sorts are stable. A map has no order of its own and shows in key order (T2) |
| K8 | Three implementations | Lines added to the parser, the checker and the evaluator, three times |
| K9 | One way to do a thing | A form that says what a library entry says must name which one goes |

## 5. The candidates

### 5.1 L: the library only (the control)

No new syntax. Seven entries, in the notation of `06:2.7`. All are exact.

| Signature | Meaning | Steps | Units |
| --- | --- | --- | --- |
| `sortBy: (f: (item: T, index: int) => K, order?: "asc" \| "desc") => T[] where K: ordered` | As today, with a direction. `"desc"` reverses the order of the keys. Elements with equal keys keep their order either way, so it is not `sortBy(f).reverse()` | as today | as today |
| `minBy`, `maxBy: (f: (item: T, index: int) => K) => T \| null where K: ordered` | The element with the least or greatest key. Of equal ones, the first. Null if empty | `2n`; strings add `sz(keys)` | 0 |
| `uniqueBy: (f: (item: T, index: int) => K) => T[] where K: ordered` | The first element with each key, in order | `2n + sz(keys)` | `1 + k` |
| `indexBy: (f: (item: T, index: int) => string) => map<T>` | Each element under its key. A later element replaces an earlier one, as in `toMap` | `2n` | `1 + 2n` |
| `countBy: (f: (item: T, index: int) => string) => map<int>` | For each distinct key, how many elements have it. `g` is the number of keys | `2n` | `1 + 2g` |
| `mapValues: (f: (value: T, key: string) => U) => map<U>`, receiver `map<T>` | The same keys, with `f` of each value. `f` is called in key order | `2n + L × (n + sz(keys))` | `1 + 2n` |

- **Two keys are still two sorts, the minor key first.** Each now has its own
  direction: row 5 is `orders.sortBy(o => o.placed).sortBy(o => o.customer,
  "desc")`. One call with several keys would need a key that is a list with a
  type for each position, and the checker has no tuples (`03:2.1`).
- **Order (K7).** A key that may be null does not fit `ordered`, so it is a
  check error, and the author writes `o.note ?? ""`. On `any` data a mix of
  kinds is an error of kind `kind` (`06:2.5`). No order across kinds is
  invented, as jq's is.
- **Grouping gives a map.** `groupBy` is unchanged: `map<T[]>`, shown in key
  order by code point, each group in the order of the list. `countBy` is its
  fused form. An aggregate over groups is `groupBy(f).mapValues(g => ...)`. A
  key is a string: a number is grouped by `string(n)`.
- **One checker rule: `filter` and `find` narrow.** When the lambda's first
  parameter is a name, the element type of the result is that parameter's type
  under the true facts of the body (`03:2.8`). So `xs.filter(x => x != null)`
  loses null, `xs.filter(x => x is Order)` is an `Order[]`, and a tag test on a
  tagged union keeps one alternative. A fact about a field of the element
  changes nothing: facts narrow paths, not the fields inside a record type.
  TypeScript infers the same for `filter` (M).
- `sumBy` is left out: `xs.map(f).sum()` says it. `uniqueBy` and `countBy` are
  the first to cut.

### 5.2 P: paths. L, and one new step

```ebnf
Chain = Primary { Read | Star | "." Word Args | "?." Word Args } .
Read  = "." Word | "?." Word | "[" Expr "]" | "?.[" Expr "]" .
Star  = "[" "*" "]" | "?.[" "*" "]" .
```

A **projection** is a `Star` and the reads and stars that follow it directly.
`orders[*].lines[*].sku` is one projection. It is JSONPath's and JMESPath's
spelling, with JSONPath's meaning: nested passes and one flat result.

| Question | Answer |
| --- | --- |
| What must `x` be in `x[*]`? | A list. Null is an error, as for any plain step, and `x?.[*]` makes the whole chain null (B4). Any other kind, a map and a string included, is an error of kind `kind`. Nothing is treated as a list of one |
| What does a star cover? | The reads that follow it: `.f`, `?.f`, `[e]`, `?.[e]`, and further stars. They are applied to each element in turn. The first method call ends the projection and receives the list. So does the end of the chain |
| What is the value? | Always one list: what the reads gave for each element, in order. With one star its length is the length of `x` |
| A second star | Iterates the list that the reads gave for each element, and its results go into the same list. So a projection is nested passes with one flat result. A list that no star iterated is never flattened: `orders[*].lines` is a list of lists |
| One result, or none | A list of one. An empty list |
| A missing field | An error with `.f`, as everywhere. With `?.f` that element gives null, and the null is kept |
| A `?.` or `?.[` inside a projection whose receiver is null | It skips the rest of the reads for that element, which gives one null. Other elements go on. Outside a projection B4 is unchanged |
| An index read | `orders[*].lines[0]` is the first line of each order. Each read is evaluated for each element, its index expression included |
| Type (`03:2.7`) | The receiver must be a `T[]` (`maybe-null`, `kind`). The reads are checked as a chain that starts at `T`. If they end at `U`, the type is `U[]`, with `U` gaining null where a `?.` can skip. An inner star over `V[]` whose reads end at `W` gives `W[]` |
| On `any` | The receiver is a `dynamic` site, each read after it is one as today, and the type is `any[]` |
| Narrowing | None. A projection has no test |

| Work | Steps | Units |
| --- | --- | --- |
| A star | 1 for each element it visits, and nothing for the step itself | |
| A read in a projection | What the read costs today, once for each element it is applied to | What it creates today |
| The result | | `1 + k` for a list of `k` values. Charged when the reads have run and before the list is built, as `flatMap` does (`06:2.4`). Until then the values sit in scratch space, bounded by the steps charged |

With three orders of 2, 0 and 4 lines, `orders[*].lines[*].sku` is 18 steps and
7 units: 3 visits, 3 reads of `lines`, 6 visits and 6 reads of `sku`. Row 2 of
the baseline is 37 steps and 14 units for the same list.

- **Nesting.** A star opens a level that lasts to the end of its projection
  (`02:2.16`), so a projection has at most 100 stars and an evaluator may
  recurse once for each.
- **One node kind,** `Each`, with `x`, the reads and `optional`.
- **K9.** `xs[*].f` says what `xs.map(x => x.f)` says. `map` stays: it is the
  general form and the one a TypeScript author writes. The star is a fused form
  for a body that is only a path, as `count` is for `filter` and `len`.

### 5.3 C: comprehensions. L, and two bracket forms

```ebnf
Primary  = ... | ListComp | MapComp .
ListComp = "[" Gen { Gen | Filter } Expr "]" .
MapComp  = "{" Gen { Gen | Filter } "[" Expr "]" ":" Expr "}" .
Gen      = "for" "(" Pattern "of" Expr ")" .
Filter   = "if" "(" Expr ")" .
```

It is the loop header of TypeScript inside brackets, and the form of the array
comprehension once proposed for ECMAScript (M). `for` is reserved already and
`of` is a word only here (`02:2.2`).

| Question | Answer |
| --- | --- |
| Evaluation | Clauses run left to right, each inside the ones before it. A generator visits each element of its list and binds its pattern. A filter goes on only when its test is truthy (K3). The last expression is evaluated once for each pass that reaches it |
| Value | A list of those values, in order. For the map form, a map: the key must be a string, and a later entry replaces an earlier one |
| A source that is null or not a list | An error. Write `x ?? []` |
| Scope | A name is in scope in every later clause and in the result. This is what the method chain lacks: two generators, both names usable |
| Type and narrowing | The result is checked under the true facts of every filter before it. So `[for (e of events) if (e.user != null) e.user]` is a `string[]`, and any path fact works, a literal set on a field included |
| `if` | Directly after a clause, `if (c)` is a filter. A result that is an `if` expression is written in parentheses. An `else` after the result is a targeted error that says so |
| Cost | 1 step for each element a generator visits, and the steps of each clause as it runs. `1 + k` units for a list and `1 + 2k` for a map, charged as in 5.2 |
| Nodes and nesting | Two node kinds, `Comp` and `Gen`. A `{` followed by `for` is the map form, a third case beside a map and a block (`02:2.1`). Each `for` opens a level to the closing bracket |
| K2 | No lambda. Each generator is one pass over a list that exists. Step 1 of the termination argument of `02:2.14`, "there is no loop", must be restated for it, and for a star |
| K9 | It is a second way to write `filter`, `map`, `flatMap` and `toMap`. None of them can go: they are what a TypeScript author writes first |

### 5.4 Q: a query expression

```ebnf
Expr  = If | Query | Binary [ "?" Expr ":" Expr ] .
Query = From { From | "where" Expr | "order" "by" Key { "," Key }
             | "group" Expr "by" Expr "into" Name } "select" Expr .
From  = "from" Pattern "in" Expr .
Key   = Expr [ "asc" | "desc" ] .
```

| Question | Answer |
| --- | --- |
| Words | Nine words that are not reserved: `from`, `where`, `order`, `by`, `asc`, `desc`, `group`, `into`, `select`. Each is recognized where a word follows a complete expression, which is never valid today |
| Place | A whole expression, at the level of `if`. As an operand or a receiver it needs parentheses |
| Rows | A clause works on rows: one binding of every name so far. `from` multiplies rows and `where` drops them |
| `order by` | One stable sort of the rows by the keys in order, each with its direction. K7 holds for each key |
| `group e by k into g` | Ends the names before it. `g` is `{key: string, items: T[]}`, with one row for each key, in key order by code point |
| Value | The list of the `select` values. Never a map: a map needs `toMap` after it |
| Cost | `order by` and `group` must hold the rows before they can go on. A row is not a value, so `04:A.4` has no charge for it. This is a new kind of charge: about 1 unit for each name of each row (I) |
| Syntax and library | All of it is syntax. `unique`, `sum`, `slice` and `toMap` are still called on its result, and it has no clause for what `minBy`, `uniqueBy` and `indexBy` do |
| K9 | A second way to write `filter`, `map`, `flatMap`, `sortBy` and `groupBy` |

### 5.5 The workload in each candidate

"L" in a later column means the cell is the same as L.

| # | L | P | C | Q |
| --- | --- | --- | --- | --- |
| 1 | as today | `orders.filter(o => o.status == "open")[*].id` | `[for (o of orders) if (o.status == "open") o.id]` | `from o in orders where o.status == "open" select o.id` |
| 2 | as today | `orders[*].lines[*].sku`; `body.items[*].tags[*]` | `[for (o of orders) for (l of o.lines) l.sku]` | `from o in orders from l in o.lines select l.sku` |
| 3 | as today | L | L | L |
| 4 | `events.sortBy(e => e.ms, "desc").slice(0, 3)` | L | L | `(from e in events order by e.ms desc select e).slice(0, 3)` |
| 5 | `orders.sortBy(o => o.placed).sortBy(o => o.customer, "desc")` | L | L | `from o in orders order by o.customer desc, o.placed select o` |
| 6 | `events.map(e => e.user).filter(u => u != null).unique()` | `events[*].user.filter(u => u != null).unique()` | `[for (e of events) if (e.user != null) e.user].unique()` | `(from e in events where e.user != null select e.user).unique()` |
| 7 | `events.countBy(e => e.kind)` | L | L | `from e in events group e by e.kind into g select {kind: g.key, n: len(g.items)}` |
| 8 | `orders.groupBy(o => o.customer).mapValues(g => g.flatMap(o => o.lines).map(l => l.qty * l.price).sum())` | `orders.groupBy(o => o.customer).mapValues(g => g[*].lines[*].map(l => l.qty * l.price).sum())` | `orders.groupBy(o => o.customer).mapValues(g => [for (o of g) for (l of o.lines) l.qty * l.price].sum())` | `from o in orders group o by o.customer into g select {customer: g.key, revenue: (from o in g.items from l in o.lines select l.qty * l.price).sum()}` |
| 9 | `events.maxBy(e => e.ms)` | L. The number alone: `events[*].ms.max()` | L | `(from e in events order by e.ms desc select e).first()` |
| 10 | as today | L | L | L |
| 11 | `{ const byId = customers.indexBy(c => c.id); orders.map(o => {id: o.id, customer: byId?.[o.customer]?.name ?? "unknown"}) }` | L | `{ const byId = {for (c of customers) [c.id]: c.name}; [for (o of orders) {id: o.id, customer: byId?.[o.customer] ?? "unknown"}] }` | `from o in orders from c in customers where c.id == o.customer select {id: o.id, customer: c.name}`: `n × m` steps, and an order with no customer is dropped |
| 12 | `services.entries().groupBy(e => e.value.owner).mapValues(g => g.map(e => e.key))` | `services.entries().groupBy(e => e.value.owner).mapValues(g => g[*].key)` | `services.entries().groupBy(e => e.value.owner).mapValues(g => [for (e of g) e.key])` | `from {key, value} in services.entries() group key by value.owner into g select {owner: g.key, services: g.items}` |

L alone removes every "clumsy" verdict. P adds rows 2, 8 and 12, and the short
aggregates `orders[*].lines[*].qty.sum()` and `"urgent" in issue.labels[*].name`
(`02:5`, example 6). C adds rows 2, 6 and 11. Q writes rows 5 and 7 best, and
gives lists where the others give maps.

### 5.6 Scores

| | L | P adds | C adds | Q |
| --- | --- | --- | --- | --- |
| K1 TypeScript | No new text. None of the six new names is a method of a JavaScript array (I) | `x[*]` and `x?.[*]` are syntax errors (ran) | `[for (` and `{for (` are syntax errors (ran) | `from o in xs` is a syntax error (ran) |
| K2 lambdas, loops | Lambdas go to the library | No lambda. One pass for each star | No lambda. One pass for each generator | The same, and rows |
| K3 truthiness | As today | No test | `if (c)` | `where c` |
| K4 null, missing | As today | 5.2: an error, or null by `?.`. Nothing dropped or unwrapped | A source must be a list | The same |
| K5 checker | `filter` and `find` narrow the element itself | One row in `03:2.7`. No narrowing of its own | Every fact of a filter reaches the result | The same, and `group` changes the scope |
| K6 cost | Formulas in 5.1 | Exact. 18 steps against 37 in the example of 5.2 | Exact | Needs a new kind of charge, for rows |
| K7 order | Stable in both directions. A map in key order | Nothing to say | Nothing to say | One sort over several keys. Groups in key order |
| K8 lines in each implementation (I) | About 200 of library and 30 of checker | 30 parser, 50 checker, 90 evaluator | 90, 110, 160 | 220, 260, 400 |
| K9 one way | `minBy` and `maxBy` replace the advice `sortBy(f).first()` | A second way to write a `map` whose body is a path | A second way to write four entries | A second way to write five |

## 6. Recommendation

### 6.1 What, and when

| Building block | Verdict | When |
| --- | --- | --- |
| The seven entries of L, and `filter` and `find` that narrow | Yes. They are needed under every candidate, because the syntax feeds them | 3.0 |
| The projection step `[*]` (P) | Yes. It is the one thing asked for that a library cannot give, it is the smallest syntax here, and its spelling is already known from JSONPath | 3.0 |
| Comprehensions (C) | Later, on evidence: ported programs that need two names in scope, or that build maps. `[for` and `{for` stay errors until then, so it is an addition | A minor version |
| A query expression (Q) | No. It is five times the size of P, it needs nine words and a new kind of charge, and it is a second language beside the method chain, which is what LINQ became in C# | Not planned |
| A filter step, `xs[?c]` or `xs[c]` | No. `xs[c]` is valid TypeScript (K1). `xs[?c]` needs a name for the element, and with one it is `filter` | Never |
| Quantifier words, `some x in xs { c }` and `every` | No. `xs.some(x => c)` and `xs.every(x => c)` say it and stop early | Never |
| A lambda with an implicit parameter: `it`, `#`, a leading `.f` | No. `02:2.9` decided it: a nested one cannot reach the outer element, and on `any` data a bare name could be a field or a variable | Never |
| A pipe operator | No. Method chains are the pipe (`02:6`) | Never |
| Mapping implied by `.`, unwrapping a list of one, dropping nulls, descendants (`..`) | No. They are the surprises of section 3 | Never |

### 6.2 Decisions for the owner

| # | Decision | Recommendation | Strongest alternative |
| --- | --- | --- | --- |
| 13-1 | Which direction, and when | L and P in 3.0. C in a minor version. Q not planned | C in place of P, the Rego lean. It covers more: filters, joins, narrowing by any test, maps built in linear time. It costs about twice the lines and is a second way to write `filter` and `map`. A second alternative is L alone in 3.0: every form here can be added later without a break |
| 13-2 | What a projection covers, and its shape | The reads after the star, and no more: a method call ends it and receives the list. The result is one flat list (JSONPath's rule). So `orders[*].lines[*].qty.sum()` needs no parentheses, and `users[*].email.toLowerCase()` is the check error `no-method`, with a hint to write `map` | The rest of the chain, as `?.` covers it (B4) and as JMESPath does. One scope rule for both marks, and a method call applies to each element. The price is `(orders[*].lines[*].qty).sum()`. Either rule has the same few silent cases: a method that both the element and the list have, such as `slice` on a list of strings |
| 13-3 | The library set | The seven entries of 5.1, with direction as `sortBy(f, "desc")`. This reverses four lines of `06:2.8`: `minBy`, `maxBy`, `uniqueBy` and `mapValues` are "deliberately absent" there, and a map has "three methods and no more" | A second callback on `groupBy` in place of `mapValues` and `countBy`: `events.groupBy(e => e.kind, g => len(g))`. No new map method, but the result type then depends on the number of arguments, and the library has no overloads (`06:2.7`) |

### 6.3 The gain, side by side

| # | v3 today | Recommended: L and P |
| --- | --- | --- |
| 1 | `orders.filter(o => o.status == "open").map(o => o.id)` | `orders.filter(o => o.status == "open")[*].id` |
| 2 | `orders.flatMap(o => o.lines).map(l => l.sku)` | `orders[*].lines[*].sku` |
| 3 | `services?.billing?.owner ?? "nobody"` | the same |
| 4 | `events.sortBy(e => -e.ms).slice(0, 3)` | `events.sortBy(e => e.ms, "desc").slice(0, 3)` |
| 5 | `orders.sortBy(o => -o.placed).sortBy(o => o.customer).reverse()` | `orders.sortBy(o => o.placed).sortBy(o => o.customer, "desc")` |
| 6 | `events.flatMap(e => e.user == null ? [] : [e.user]).unique()` | `events[*].user.filter(u => u != null).unique()` |
| 7 | `events.groupBy(e => e.kind).entries().map(({key, value}) => {key, value: len(value)}).toMap()` | `events.countBy(e => e.kind)` |
| 8 | `orders.groupBy(o => o.customer).entries().map(({key, value}) => {key, value: value.flatMap(o => o.lines).map(l => l.qty * l.price).sum()}).toMap()` | `orders.groupBy(o => o.customer).mapValues(g => g[*].lines[*].map(l => l.qty * l.price).sum())` |
| 9 | `events.sortBy(e => -e.ms).first()` | `events.maxBy(e => e.ms)` |
| 10 | `orders.some(o => o.lines.some(l => l.qty > 100))` | the same |
| 11 | `{ const byId = customers.map(c => {key: c.id, value: c.name}).toMap(); orders.map(o => {id: o.id, customer: byId?.[o.customer] ?? "unknown"}) }` | `{ const byId = customers.indexBy(c => c.id); orders.map(o => {id: o.id, customer: byId?.[o.customer]?.name ?? "unknown"}) }` |
| 12 | `services.entries().groupBy(e => e.value.owner).entries().map(({key, value}) => {key, value: value.map(e => e.key)}).toMap()` | `services.entries().groupBy(e => e.value.owner).mapValues(g => g[*].key)` |

On untyped data the forms compose with `as` (`07`): `(body as {items: {tags:
string[], ...}[], ...}).items[*].tags[*]` is a `string[]` with no `dynamic`
site.

### 6.4 What would change in the other documents

| Document | Change, if 13-1 to 13-3 are accepted as recommended |
| --- | --- |
| `02` | 2.2: `Star` in `Chain`. 2.3: one node, `Each`. 2.8: the rules of 5.2, and B4 inside a projection. 2.14: a star among the constructs that create a value, and step 1 of the termination argument restated. 2.16: a star opens a level to the end of its projection. 3.2: targeted errors for `xs.*.f`, `xs[].f`, `xs[?c]` and `$.a`. Section 5: example 6 |
| `03` | 2.7: the row for a star. 2.11: `filter` and `find` narrow. No new code |
| `04` | A.3 and A.4: the rows of 5.2 |
| `05` | A.8: a star as a step of a static path (I) |
| `06` | 2.7: sixteen entries take a callback, one of them a map method. 2.8: the entries of 5.1; the bullets on `sort` and the rows of "deliberately absent". 2.2: hints for `orderBy`, `keyBy`, `distinct` and `sort_by` |
| Conformance suite | For a star: an empty list, a null receiver with and without `?.`, a map and a string as receivers, a missing field with `.` and with `?.`, two stars with an empty inner list, a method call that ends the projection, each with its steps and units. For the entries: equal keys under both directions, strings above U+FFFF, a mix of kinds, a null key, an empty list |

## 7. Open questions, and what I did not check

- **A star on a map.** `services[*].owner` could visit the values in key order,
  as `values()` does. It is left an error here, with a hint to write
  `services.values()[*]`, to keep one rule.
- **A null inside a flat result.** `orders[*].lines?.[*].sku` gives one null for
  an order whose `lines` is null, among the skus. It follows from "nothing is
  dropped". An author may expect nothing there, as in JSONPath.
- **`01:2.5` says to avoid projections.** It lists "JMESPath has projections"
  beside JSONata's implicit map. P is explicit, and gives an error where
  JMESPath gives null. The owner's request reopens that line of `01`.
- **The index of an element.** A star gives none. A comprehension could bind
  one, as Rego's `some i, x in xs` does. `map((x, i) => ...)` has it today.
- **A trace entry.** Plan `10` lists the calls in progress, with the index of
  the element for a library callback. A failure inside a projection has an index
  and no call. It needs an entry of its own, or none.
- **Narrowing a field through `filter`.** `orders.filter(o => o.status ==
  "open")` leaves `status` as the whole literal set. Only a comprehension
  carries that fact to the next step.
- **`const` in a generator.** `for (o of xs)` is written here as `catch (err)`
  is, without the word. TypeScript's own header has `const`.
- **One sort with several keys** in the library waits for a way to type a key
  with a kind for each position.
- **Not checked.** No grammar here was run through a parser, and no step count
  was produced by code: the counts in 5.2 are by hand from `04:A.3`. The parse
  tests ran in Node 22 and not in TypeScript. I assume TypeScript accepts no
  expression that JavaScript rejects among these forms (I). Plan `09` was not
  yet written, so "a tag test narrows" in 5.1 is its brief and not its text. The
  line counts are estimates. The rows marked (M) in section 3 were not looked
  up.
