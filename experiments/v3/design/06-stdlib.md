# Risor v3 design 06: the standard library

Status: design proposal, revised to `00-overview.md`. Decision numbers such as
C1, L3 and T4 refer to that overview. No code was changed.

**Revised for plans `08` to `13` (D46 to D51).** Seven entries of plan `13` are
in: `sortBy` takes a direction, and `minBy`, `maxBy`, `uniqueBy`, `indexBy`,
`countBy` and `mapValues` are new (2.8). `fail` takes data (2.8). Recovery is
written `try { e } catch { d }` everywhere (`10`). The messages of the library's
failures come from the catalog `messages.json` (2.3, 2.11). A case has no
tolerance field (2.8, `math`). The library now has 78 entries. A decision of a
plan is cited by the plan's own number, such as `13-2`.

**How to read this.** Section 2 defines the library. A rule there is normative:
any implementation must do it. A paragraph headed *Why*, *Rejected* or *Note* is
not normative, and the bullets under an entry table explain the table without
adding to it. Nothing in section 2 is defined by what Go does (P3). Notes for
the Go, TypeScript and Rust implementations are in section 7.

**Evidence.** **(R)** means I read the cited lines. **(X)** means I ran it: in
Risor v2, expr v1.4.1, Go 1.26.2 or Node 22, in throwaway programs outside the
repositories. **(I)** marks an inference. `v2:` is this repository at `07efd88`,
`expr:` is `~/git/deepnoodle/expr` at v1.4.1, `wf:` is
`github.com/deepnoodle-ai/workflow@v0.1.0`. `02:` to `05:` are the sibling
documents by section, and `07:` to `13:` are the plans.

## 1. Summary

1. **The 3.0 library has 78 entries.** 59 are in the default set: 15 functions
   and 44 methods. Four opt-in namespaces add 19 functions, and 2 constants that
   are not counted as entries. v2 ships 175 entries and expr 40 (section 3).
2. **67 entries are exact and 11 are implementation-defined (T3).** The 11 are
   the transcendental functions of `math`. Every other entry has one result in
   every implementation, stated in neutral terms: code points, a listed set of
   white space, a case table the specification owns, RFC texts.
3. **One shape per operation.** An operator if the language has one. Otherwise a
   method if the subject is a string, a list or a map. Otherwise a function.
4. **Names (E7): JavaScript's for string and list methods, expr's for the
   functions.** `some`, `every`, `flat`, `flatMap`, `toUpperCase` and
   `toLowerCase` replace the first draft's `any`, `all`, `flatten`, `upper` and
   `lower`. A familiar name that v3 does not use is a compile error with a hint.
5. **One failure rule.** Absent gives null and the type `T | null`. Invalid
   gives an error and the signature carries `!`. No sentinel, and no null turned
   into zero. Every such error has the code `invalid-argument`, and its message
   is a template of the catalog `messages.json` (D48).
6. **Every entry declares two formulas, in the runtime's units: steps and
   allocation units.** Both are charged before the work (L2, L3). An entry
   without them cannot be registered. A generic test scales each entry's input
   and checks the declaration against what really happened.
7. **`range`, `repeat`, `padStart` and `padEnd` are back (E8)**, with `fixed`
   for decimals. With allocation charged first they are safe, and with no
   `sprintf` they are needed. `toMap` and `fail` are new, and `fail` takes data:
   `fail(message, data?)` (D48).
8. **Regular expressions are left out of 3.0 (T4).** The namespace is reserved
   and a linear-time subset is written down as the candidate for 3.1.
9. **Nothing in the library is nondeterministic.** No clock, random source,
   locale or time zone. The current time is data the host supplies. Times are
   numbers: seconds since the Unix epoch.
10. **The library is one table of data in the specification.** Each entry is a
    record: name, receiver, signature text, the two formulas, failure cases,
    exactness and conformance cases. Every implementation and the documentation
    are generated from it or checked against it. It is versioned with the
    specification, and a released entry never changes.
11. **Queries (D51).** `sortBy` takes a direction. Six entries are new: `minBy`,
    `maxBy`, `uniqueBy`, `indexBy`, `countBy` and, on a map, `mapValues`.
    `filter` and `find` narrow the element type. The projection step `[*]` of
    `02` overlaps `map`, `flatMap` and `flat`, and no entry goes (2.1).

## 2. The library

### 2.1 Shape

**The rule, in order:**

1. If the language has an operator or a literal form for the operation, that is
   its only spelling.
2. Otherwise, if the operation has one subject and the subject is a string, a
   list or a map, it is a method on that kind.
3. Otherwise it is a function.

Strings, lists and maps have library methods. Numbers, booleans and null have
none. A host declares methods only on its own opaque types (`05:A.2`).

| Kind of operation | Shape | Written |
| --- | --- | --- |
| Membership, key test, substring | operator | `x in xs`, `k in m`, `sub in s` |
| Default for null | operator | `a ?? b` |
| Read that may miss | operator | `m?.k`, `m?.[k]`, `xs?.[i]` |
| Concatenate, append | spread | `[...a, ...b]`, `[...xs, x]` |
| Merge, set or remove a field | spread, computed key, rest pattern | `{...a, ...b}`, `{...m, [k]: v}`, `{ const {a, ...rest} = m; rest }` |
| Build text | template string | `` `${n} failed` `` |
| Choose | operator, `if`, `match` | `c ? a : b`, `if (c) { a } else { b }` |
| Test a value against a type | operator | `x is T`, `x as T` (`07`) |
| Recover from a failed call | syntax | `try { e } catch { fallback }` (`10:2.2`) |
| Power | operator | `x ** 2` |
| Read a path through each element of a list | the projection step (`02:2.8`) | `orders[*].lines[*].sku` |
| Transform, search, order or aggregate a list | method | `xs.filter(f)`, `xs.sortBy(f, "desc")`, `xs.sum()` |
| Operate on text | method | `s.trim()`, `s.split(",")` |
| Enumerate a map, or transform its values | method | `m.keys()`, `m.mapValues(f)` |
| Convert between kinds | function, named for the result | `int(s)`, `string(x)` |
| Arithmetic on numbers | function | `abs(a - b)`, `max(0, n - 1)` |
| Opt-in group | function in a namespace | `math.sqrt(x)`, `json.parse(s)` |

- **`len` is the one exception to rule 2.** Its subject is a string, a list or a
  map. It is a function because a property `.length` cannot exist (`.` reads a
  field), and `len(x)` is what `02` and the engine's documents already write.
- **`min` and `max` exist in both shapes because they are two operations.**
  `max(a, b)` compares its arguments and needs two. `xs.max()` aggregates a list
  and may be null. The language has no spread arguments.
- **Some methods are fused forms of others**: `count`, `find`, `some`, `every`,
  `flatMap`, `minBy`, `maxBy`, `indexBy`, `countBy` and `mapValues`. Each exists
  because it stops early, or skips a sort, a list or a map that the longer form
  would build and pay for. An optimizer may not stop early on the author's
  behalf (`04:A.8`), so the stopping forms must be entries.
- **The projection step is the one exception to rule 1** (D51). `xs[*].f` says
  what `xs.map(x => x.f)` says, `xs[*].ys[*]` what `xs.flatMap(x => x.ys)` says,
  and `xss[*][*]` what `xss.flat()` says. No entry goes. `map` and `flatMap` are
  the general forms: their body is any expression, and it has the index. The
  star is a fused form for a body that is only a path, as `count` is for
  `filter` and `len` (`13:5.2`). `flat` is the only entry that a projection
  replaces in every case. It stays, because it is the name a TypeScript author
  writes first, and `xss[*][*]` is not.
- **In call position a library method wins.** `m.keys()` is the method even if
  `m` has a field `keys`. `m.keys` and `m["keys"]` are always the data.
- **After a projection, a method receives the list** (13-2). The call ends the
  projection. `orders[*].lines[*].qty.sum()` is `sum` of the flat list. So
  `users[*].email.toLowerCase()` is `no-method`: a string method is not applied
  to each element, and the hint is `users.map(u => u.email.toLowerCase())`.
  `slice` is the one name that both a string and a list have: `names[*].slice(0,
  3)` is the first three names, not three characters of each.

*Why methods.* A chain reads in the order it runs. A method is found through its
receiver's type, so the checker and an editor list exactly what applies. A
method takes no name from the host's variables: the engine reserves 44 function
names against step names today (`wf:expr.go:74-87`) (R), and this library needs
15.

*Rejected.* A pipe operator: with methods it would be a second call syntax
(`02:6`). `xs.len()`: consistent, but against every habit the authors have.

### 2.2 Names

**Decision (E7).** The test is: what will a person or a model write first, and
what will they expect it to mean?

| Group | Names follow | Why |
| --- | --- | --- |
| String and list methods that JavaScript has | JavaScript | The syntax is JavaScript's, so its method names are what gets written first. I use the JavaScript name wherever v3's meaning is JavaScript's, or differs only as listed in 2.9. |
| String and list methods that JavaScript lacks | the most common name elsewhere: `sortBy`, `groupBy`, `minBy`, `maxBy`, `countBy`, `uniqueBy`, `indexBy`, `sum`, `count`, `unique`, `first`, `last` | These are expr's, lodash's or Underscore's names. There is no JavaScript spelling to be surprised by: none of them is a method of a JavaScript array (`13:5.6`) (I). `uniqueBy` follows `unique`, where lodash writes `uniqBy`, and `indexBy` says what the result is, where lodash writes `keyBy`. Both of those get a hint. |
| Map methods | `keys`, `values`, `entries`, `mapValues` | The first three are the same words in JavaScript, v2 and expr. `mapValues` is lodash's and Kotlin's name. (I): the names of other libraries in this row and the one above are from memory. |
| Functions | expr's: `len`, `string`, `int`, `float`, `min`, `max`, `abs`, `floor`, `ceil`, `round` | JavaScript spells these as statics on `Math` and as capitalized constructors. v3 has neither shape, `02` and the engine's documents already use these names, and they are also Python's and Go's. |
| Namespaces | lower-case; JavaScript's verbs where it has them: `json.parse`, `json.stringify` | `JSON.parse` is what gets written first. |

Each v3 name beside JavaScript's:

| Operation | JavaScript | v3 | Same meaning? |
| --- | --- | --- | --- |
| Keep matching elements | `xs.filter(f)` | `xs.filter(f)` | Yes |
| Transform elements | `xs.map(f)` | `xs.map(f)` | Yes |
| Transform and splice | `xs.flatMap(f)` | `xs.flatMap(f)` | Yes |
| Splice one level | `xs.flat()` | `xs.flat()` | Yes; no depth argument |
| Some or every element matches | `xs.some(f)`, `xs.every(f)` | the same | Yes |
| First match | `xs.find(f)` | `xs.find(f)` | Yes; null for `undefined` |
| Fold | `xs.reduce(f, init)` | `xs.reduce(f, init)` | Yes; `init` is required |
| Join text | `xs.join(sep)` | `xs.join(sep)` | `sep` is required; elements must be strings |
| Part of a list or string | `x.slice(a, b)` | `x.slice(a, b)` | Yes; a string counts code points |
| Reverse, sort | `xs.toReversed()`, `xs.toSorted()` | `xs.reverse()`, `xs.sort()` | Nothing is mutated, so the short names are free. `sort` differs: 2.9 |
| Sort with a comparator | `xs.sort((a, b) => ...)` | `xs.sortBy(x => key)`, `xs.sortBy(x => key, "desc")` | A key and a direction, not a comparator: 2.8 |
| Membership | `xs.includes(x)`, `s.includes(t)` | `x in xs`, `t in s` | The language's operator (`02:3.1`) |
| Length | `x.length` | `len(x)` | A property cannot exist |
| End elements | `xs.at(0)`, `xs.at(-1)` | `xs.first()`, `xs.last()` | `at` would be a second index beside `xs?.[i]` |
| Position of an element | `xs.indexOf(x)` | none | 2.8 |
| Concatenate | `xs.concat(ys)` | `[...xs, ...ys]` | Spread |
| Keys, values | `Object.keys(m)`, `Object.values(m)` | `m.keys()`, `m.values()` | Methods; in key order |
| Entries | `Object.entries(m)` | `m.entries()` | Records `{key, value}`, not pairs |
| Map from entries | `Object.fromEntries(ps)` | `es.toMap()` | From records |
| Group | `Object.groupBy(xs, f)` | `xs.groupBy(f)` | Yes |
| Map from a list, by key | `Object.fromEntries(xs.map(x => [x.id, x]))` | `xs.indexBy(x => x.id)` | A later element replaces an earlier one, as there |
| Transform the values of a map | `Object.fromEntries(Object.entries(m).map(([k, v]) => [k, f(v)]))` | `m.mapValues(f)` | The callback is called in key order |
| Case | `s.toUpperCase()`, `s.toLowerCase()` | the same | One code point to one: 2.9 |
| Trim | `s.trim()` | `s.trim()` | Yes: the same 25 characters (X) |
| Prefix, suffix test | `s.startsWith(p)`, `s.endsWith(p)` | the same | Yes; no position argument |
| Split | `s.split(sep)` | `s.split(sep)` | Yes; no limit argument |
| Replace | `s.replaceAll(a, b)` | `s.replaceAll(a, b)` | `b` is literal text: 2.9 |
| Repeat, pad | `s.repeat(n)`, `s.padStart(n, f)`, `s.padEnd(n, f)` | the same | Yes; lengths in code points |
| Number functions | `Math.abs(x)`, `Math.floor(x)`, `Math.max(a, b)` | `abs(x)`, `floor(x)`, `max(a, b)` | Yes |
| Round | `Math.round(x)` | `round(x)` | Halves differ: 2.9 |
| Truncate, parse | `Math.trunc(x)`, `parseInt(s)`, `Number(s)` | `int(x)`, `int(s)`, `float(s)` | Bad text fails; it is not `NaN` |
| To text | `String(x)`, `x.toString()` | `string(x)` | The rendering of `02:2.13` |
| Fixed decimals | `x.toFixed(d)` | `fixed(x, d)` | Yes, but no `-0.00` |
| Kind | `typeof x` | `type(x)` | v3's kind names. As a test: `x is string` (2.8) |
| Raise | `throw new Error(m)` | `fail(m)`, `fail(m, data)` | An expression. A handler reads `err.message` and `err.data` (`10:2.3`) |
| JSON | `JSON.stringify(x)`, `JSON.parse(s)` | `json.stringify(x)`, `json.parse(s)` | Keys sorted; a large integer fails |
| Base64 | `btoa(s)`, `atob(s)` | `base64.encode(s)`, `base64.decode(s)` | Text as UTF-8 |
| Time | `Date.parse(s)` | `time.parse(s)` | Seconds; RFC 3339 only |

Conventions for new names: camelCase and ASCII; no aliases; a transform is a
plain verb; a variant that takes a key function ends in `By`; a conversion is
named for its result; no name is a reserved word of `02:2.2`. v2's
`regexp.match` cannot be called at all, because `match` is a keyword (X).

**Wrong names are corrected, not accepted.** The table of 2.11 holds, for each
entry, the names people try. The checker uses them only in hints. This is how
"one way" and "familiar" are both kept. The diagnostic is the one `03` gives
anyway, with its message from the catalog: `undefined-name`, `no-method` or
`no-field` for a name, and `arity`, `arg-mismatch` or `not-a-value` for an
argument. The hint is the diagnostic's `help`, which is not specified and which
no case compares (`10:2.9`). So a hint may be added or reworded in any version.

| Written | Hint |
| --- | --- |
| `xs.length`, `xs.len()`, `xs.size()` | `len(xs)` |
| `xs.includes(x)`, `xs.contains(x)`, `contains(xs, x)`, `has(m, k)` | `x in xs` |
| `xs.any(f)`, `xs.all(f)` | `xs.some(f)`, `xs.every(f)` |
| `s.upper()`, `s.to_upper()`, `upper(s)` | `s.toUpperCase()` |
| `s.has_prefix(p)`, `s.starts_with(p)`, `startsWith(s, p)` | `s.startsWith(p)` |
| `s.trimPrefix(p)`, `s.removeprefix(p)` | `s.removePrefix(p)` |
| `s.replace(a, b)`, `s.replace_all(a, b)` | `s.replaceAll(a, b)` |
| `s.substring(a, b)`, `s.substr(a, n)`, `x[a:b]` | `x.slice(a, b)` |
| `xs.flatten()` | `xs.flat()` |
| `xs.at(-1)`, `xs.at(0)` | `xs.last()`, `xs.first()` |
| `xs.push(x)`, `xs.append(x)`, `xs.concat(ys)`, `concat(xs, ys)` | `[...xs, x]`, `[...xs, ...ys]` |
| `xs.sort((a, b) => ...)`, `sorted(xs)`, `xs.sort_by(f)`, `xs.orderBy(f)`, `xs.toSorted(f)` | `xs.sortBy(x => key)`, `xs.sort()` |
| `xs.sortByDescending(f)`, `xs.orderByDescending(f)`, `xs.sortBy(f, "descending")`, `xs.sortBy(f, true)`, `xs.sortBy(f, -1)` | `xs.sortBy(f, "desc")` |
| `xs.thenBy(g)`, `xs.sortBy(f, g)`, `xs.sortBy([f, g])` | two sorts, the minor key first: `xs.sortBy(g).sortBy(f)` |
| `xs.keyBy(f)`, `xs.index_by(f)`, `xs.associateBy(f)`, `xs.toDictionary(f)` | `xs.indexBy(f)` |
| `xs.distinct()`, `xs.uniq()`, `xs.dedupe()` | `xs.unique()` |
| `xs.distinctBy(f)`, `xs.uniqBy(f)`, `xs.unique_by(f)` | `xs.uniqueBy(f)` |
| `xs.min_by(f)`, `xs.minByOrNull(f)`, `xs.max_by(f)`, `xs.maxByOrNull(f)` | `xs.minBy(f)`, `xs.maxBy(f)` |
| `xs.count_by(f)`, `xs.tally()`, `xs.frequencies()` | `xs.countBy(f)` |
| `xs.group_by(f)`, `Object.groupBy(xs, f)`, `groupBy(xs, f)` | `xs.groupBy(f)` |
| `xs.sumBy(f)`, `xs.sum(f)`, `xs.sumOf(f)` | `xs.map(f).sum()` |
| `xs.pluck("f")`, `xs.map("f")` | `xs[*].f` |
| `m.map_values(f)`, `m.transformValues(f)`, `m.map(f)` | `m.mapValues((value, key) => ...)` |
| `m[*].f` | `m.values()[*].f`: a star needs a list (`02:2.8`) |
| `error(m)`, `raise(m)`, `panic(m)` | `fail(m)` |
| `s.trimIndent()`, `s.stripIndent()`, `dedent(s)` | a text block, `"""`, which removes its indentation (`08:2.7`) |
| `Math.max(...xs)`, `max(xs)` | `xs.max()` |
| `m.get(k)`, `m.get(k, d)` | `m?.[k]`, `m?.[k] ?? d` |
| `Object.keys(m)`, `keys(m)`, `Object.fromEntries(es)` | `m.keys()`, `es.toMap()` |
| `coalesce(a, b)` | `a ?? b` |
| `Array.isArray(x)`, `Number.isInteger(x)` | `x is any[]`, `x is int` |
| `Math.floor(x)`, `Math.round(x)` | `floor(x)`, `round(x)` |
| `parseInt(s)`, `Number(s)`, `parseFloat(s)`, `String(x)` | `int(s)`, `float(s)`, `string(x)` |
| `x.toFixed(2)`, `sprintf("%.2f", x)` | `fixed(x, 2)` |
| `sprintf(...)`, `format(...)` | a template string, with `fixed` and `padStart` |
| `JSON.stringify(x)`, `encode(x, "json")` | `json.stringify(x)` |
| `filter(xs, f)`, `map(xs, f)` | the method |

*Why JavaScript's names where the first draft had expr's.* A model or a person
who sees arrow functions and method chains writes `.some(`, `.toLowerCase()` and
`.flat()`. With expr's names each of those is one failed attempt. The engine's
stored expressions must be rewritten in either case, because `any(xs, it.ok)`
changes shape whatever the method is called (3.3). (I): I did not measure what
authors write first.

### 2.3 Failure and null

**The rule.** Null means absent. An error means invalid.

| Situation | Result | In the signature |
| --- | --- | --- |
| The data asked for is not there: no matching element, an empty list | null | result type `T \| null` |
| An argument has a value the entry cannot use: `int("abc")`, invalid JSON, a negative count | an error of kind `value` (`03:2.14`) | `!` after the result |
| A whole-number result outside ±(2^53 − 1), or a result that is not finite | an error of kind `range`, code `number-range` | not marked: it is the language's arithmetic error (N6) |
| An argument of the wrong kind, or null | a compile error; on `any` data a run error | the parameter type |
| A limit is reached | an error of kind `limit` | not marked; `catch` does not catch it (`10:2.4`) |

- The checker tracks `T | null`, so an optional result must be handled:
  `xs.max() ?? 0`, `xs.find(f)?.name`.
- A fallback for an invalid value is written: `try { int(input.count) } catch {
  0 }`. As an operand the `try` takes parentheses: `(try { int(input.count) }
  catch { 0 }) > 10` (`10:2.2`).
- A failed `as` is a failure of the same kind, `value`, with the code
  `not-conforming` (`07:2.3`). So a `try` gives a fallback for it too: `try {
  input.config as Config } catch { defaults }`.
- **`catch` catches every failure of a library entry but a limit** (`10:2.4`):
  kind `value`, `number-range`, and a wrong kind or a null on `any` data. The
  limit codes, `depth-exceeded` among them, pass through.
- **No parameter accepts null unless its type is `any`.** That is `string`,
  `bool`, `type`, `json.stringify`, and the `data` of `fail`. A method has no
  null receiver. The author writes `len(xs ?? [])` or `xs?.filter(f)`.
- **No entry returns NaN, an infinity or negative zero** (N3, `02:2.5`).
- **A callback's failure is the call's failure**, at the position inside the
  callback. The diagnostic's trace has an entry for the library call, with the
  entry's name and the index the callback was given (`10:2.8`). The trace of
  each entry that takes a callback is in 2.7.
- **No entry changes its receiver or an argument.** A result may share memory
  with them. That cannot be observed, because no value can be changed.

**Codes.** Every `value` failure of the library has one code,
`invalid-argument`. `fail` has `failed`. Both are in the catalog of `05:A.6`.
Every other failure of an entry keeps the code the language gives it:
`number-range`, the code of a wrong kind or a null on `any` data (`02` names it,
`05:A.6` lists it), and the limit codes.

**The library in the message catalog** (`10:2.9`, D48). Before D48 a message was
free text and no case compared it. Now a program can read it as `err.message`,
and pays for its size, so it is specified.

| Matter | Rule |
| --- | --- |
| The entry | `messages.json` has one entry for `invalid-argument`: phase `run`, `caught: true`. It has one named template for each shape of failure in the table below |
| Which template | Each `fails` line of an entry in `library.json` names its code and its template (2.11). So the message of a failed call is fixed by the entry and its arguments |
| Wording | The text of each template is in `messages.json`, written with the specification. It is not in this document |
| Parameters | Of the kinds of `10:2.9`. The catalog renders each and cuts it at 64 code points. So a message is bounded, whatever the length of the argument it quotes |
| `failed` | Its entry has no template. The message is the first argument of `fail`, whole (`10:2.5`) |
| Cost | The `steps` and `units` of an entry do not include a message. A message is built only when a `catch (err)` binds the error, and it is charged there (`10:2.7`) |
| Cases | Each `fails` line has a case, and the case states the message whole (2.11) |
| Version | A run message does not change within a major version (`10:2.9`). The templates join the things that never change (2.12) |

| Template | Entries | The call fails when | Parameters |
| --- | --- | --- | --- |
| `not-a-number` | `int`, `float` | The text is not a number by the rule of text to number (2.5) | `text: string` |
| `not-finite` | `float` | The text reads as an infinity | `text: string` |
| `bad-digits` | `fixed` | `digits` is not in 0 to 20 | `digits: number` |
| `too-large` | `fixed` | The magnitude of `x` is 1e21 or more | `x: number` |
| `empty-search` | `replaceAll` | `old` is empty | none |
| `negative-count` | `repeat` | `count` is negative | `count: number` |
| `not-real` | The 11 entries of `math` marked `!` | The mathematical result is not a real number | `name: word`, the entry as written |
| `not-a-time` | `time.parse` | The text is not in the accepted form, or a field is out of range | `text: string` |
| `year-range` | `time.format` | The year is outside 0000 to 9999 | `t: number` |
| `not-a-duration` | `time.duration` | The text is not a duration text | `text: string` |
| `not-json` | `json.parse` | The text is not in the grammar of RFC 8259 | none |
| `integer-range` | `json.parse` | A number with no fraction and no exponent is not a whole number | none |
| `not-base64` | `base64.decode` | A character is outside both alphabets, or the padding is wrong | none |
| `not-utf8` | `base64.decode` | The decoded bytes are not UTF-8 | none |

- The 14 names are this document's proposal (I). They are names of templates,
  not codes: a program sees `invalid-argument` for all of them (open question
  8).
- `not-json` and `integer-range` name no position in the text. Three JSON
  scanners must agree that a text is bad, and need not agree where.
- When a text could fail in both ways, the first row wins. `integer-range` is
  reported only for a text that is in the grammar, and `not-utf8` only for a
  text that is base64.

*Rejected.* expr's rule, where `len`, `int`, `float` and the list forms accept
nil (`expr:check.go:882-916`) (R). It is two rules, and the lenient half hides
the null the checker exists to show. Open question 4.

### 2.4 Cost

Steps and allocation units are those of `04:A.3` to `04:A.5`. This section adds
no kind of charge. Symbols used in the entry tables:

| Symbol | Meaning |
| --- | --- |
| `n` | The number of elements of the receiver list, or entries of the receiver map |
| `k` | The number of elements of the result |
| `v` | The number of elements visited by a method that stops early |
| `w(x)` | The weight of string `x`: its code points, with one above U+FFFF counted twice (`04:A.5`) |
| `sz(x)` | The text size of `x`: `w(x)` divided by 64, rounded down. `sz(x, y)` and `sz(elements)` take several strings together: the sum of their weights, divided by 64, rounded down |
| `sz(keys)` | `sz` of the keys a callback gave, or of a map's keys, taken together. 0 when the keys are numbers |
| `g` | The number of distinct keys a callback gave |
| `L` | 0 when `n` is less than 2, else the smallest whole number with `2 ** L >= n` |

**How an entry charges.**

1. The evaluator charges 1 step for the call (`04:A.3`). The Steps column gives
   what the entry adds. Where `04:A.3` writes "1 + size", I read the 1 as the
   call's step.
2. A loop charges 1 step per element visited. A call of a callback costs 1 step
   plus its body. So a method that calls `f` once per element shows `2n`, and
   the bodies are charged as they run.
3. Text read is charged at `sz` of the text. Parsing JSON is charged at 8 times
   that rate.
4. A sort charges `L × (n + sz(the strings compared))`, whatever comparisons are
   made. An implementation must use a stable sort that makes at most `n × L`
   comparisons. A direction adds nothing: `sortBy(f, "desc")` charges what
   `sortBy(f)` charges.
5. Units are those of `04:A.4`, on the result as written: 1 + `sz` for a string,
   1 + `k` for a list, 1 + twice the entries for a map. They are charged whether
   or not an implementation shares storage with the receiver.
6. **The order is fixed: steps, then units, then the work** (`04:A.1`). Both
   formulas are computed from the sizes of the arguments.
7. **An entry that orders or compares keys calls its callback first**: once for
   each element, in order, before any comparison. That is `sortBy`, `minBy`,
   `maxBy` and `uniqueBy`. The term `sz(keys)`, and for `sortBy` the sort
   formula, is charged when the keys are known and before they are compared. The
   keys sit in scratch space until then.

**When the size of the output depends on the data**, the entry runs a pass that
creates no value, then charges, then builds:

| Entry | The pass that finds the size |
| --- | --- |
| `filter`, `unique`, `uniqueBy` | The callbacks run, or the elements or keys are compared, and the result of each is kept in scratch space of one bit per element. `k` is then known |
| `flatMap`, `flat` | The inner lengths are summed |
| `groupBy`, `countBy` | The keys are computed and the distinct keys counted. `g` is then known |
| `indexBy`, `mapValues`, `toMap` | None. The units are `1 + 2n`, from the length of the receiver, whatever keys repeat |
| `split` | The occurrences of the separator are counted. The parts number one more |
| `replaceAll` | The occurrences `c` are counted. The result's weight is `w(s) + c × (w(new) − w(old))` |
| `join` | The weights of the elements are summed: `w` is that sum plus `(n − 1) × w(sep)` |
| `repeat`, `padStart`, `padEnd`, `range` | Arithmetic on the arguments. No pass is needed |
| `string`, `json.stringify` | A walk of the value that measures its text. The walk is charged as steps |
| `json.parse` | A first pass that validates the text and counts the values it describes |
| `base64.decode` | A bound, not the size: the text decoded has weight at most `3 × w(s) / 4` |

Scratch space is not a value and is not charged. It is bounded by the units and
steps already charged (`04:A.4`).

*Why this is enough.* Every entry's work is at most a constant times its steps,
and every value it creates is covered by its units, before the memory is taken.
`range(1e9)` fails with `allocation-exceeded` and allocates nothing.

*Note.* `04:A.3` charges an ordered walk of a map (rendering) 1 step per entry.
The four map methods here charge the sort formula, because key order has to come
from somewhere. One of the two should change. Open question 5.

### 2.5 Shared exact rules

Every entry is **exact** unless its row says implementation-defined: for the
same arguments every implementation gives the same result or the same error
kind, and uses the same steps and units. These rules are used by several
entries.

| Rule | Definition |
| --- | --- |
| Text | A string is a sequence of code points (T1). A position, a length and a count of characters are in code points. No entry normalizes text |
| Order of strings | Code point by code point; a proper prefix is smaller (T2, `02:2.6`) |
| Order of numbers | By value |
| `ordered` elements and keys | All numbers or all strings. A null is neither: a key that may be null is a check error, and the author writes `o.note ?? ""`. On `any` data a mix, a null or any other kind is an error of kind `kind`. Every element or key is tested, before any comparison, so a list of one fails too. No order across kinds is invented |
| Equality | The language's `==` (`02:2.7`) |
| White space | Exactly these 25 code points: U+0009 to U+000D, U+0020, U+00A0, U+1680, U+2000 to U+200A, U+2028, U+2029, U+202F, U+205F, U+3000, U+FEFF |
| Case table | A table in the specification, `casemap.json`: for each code point, at most one upper-case and one lower-case code point. It is the simple case mappings of the Unicode Character Database, version 16.0. A code point not in the table maps to itself |
| Text to number | The rule of `02:2.5`: the JSON number grammar, read as the nearest double, ties to even. Entries that use it say what they add |
| Number to text | The rule of `02:2.5` (N7) |
| Whole number | A number with no fraction in ±(2^53 − 1). An entry whose result type is `int` fails with `number-range` if its result is not one (N2) |
| Stable sort | Elements that compare equal keep their order, in both directions. With that, the result does not depend on the algorithm |
| Direction | `"asc"` puts the lesser key first and `"desc"` the greater. Under `"desc"` elements with equal keys still keep their order: it is not the reverse of the ascending result |
| Key order | Wherever an entry visits a map: by key, in the order of strings above |
| Depth | An entry that walks nested values (`string`, `json.stringify`, `json.parse`) fails with `depth-exceeded` beyond the depth limit (`05:A.5`) |

*Why a case table of the specification's own.* On one machine today, Go's tables
are Unicode 15.0 and Node's are 17.0, and the two languages disagree on `ß`, on
a final `Σ` and on `İ` (X). An ASCII-only rule is exact and cheap, but
`"café".toUpperCase()` giving `CAFé` surprises more than a small table costs.
About 2,900 code points have a mapping, and no mapping crosses U+FFFF, so the
weight of the result equals the receiver's (X, counted in Go's Unicode 15.0
tables; to be confirmed for 16.0 when the table is generated).

*Why this white space set.* It is the set JavaScript's `trim` removes (X). Go's
`unicode.IsSpace` differs in two code points: it has U+0085 and lacks U+FEFF
(X).

### 2.6 Determinism and time

No entry reads anything but its arguments. There is no clock, random source,
locale, time zone, environment or I/O in any group.

- **The current time is data.** The library has no `now`. A host that wants a
  program to see the time declares a variable and supplies it. The workflow
  engine reads no clock except its event's time (`wf:llms.txt:11-12`) (R), and
  can bind that.
- **A time is a number: seconds since 1970-01-01T00:00:00Z.** A duration is a
  number of seconds. Comparison and arithmetic are the ordinary operators. Whole
  seconds are exact far beyond year 9999. A fraction is the nearest double,
  which resolves about a quarter of a microsecond today, so nanoseconds do not
  survive (2.8, `time`).
- **Random values** come from the host, as a variable or as a host function.
- **A setting from outside is data, like the clock** (12-6). No entry reads the
  process environment or a file. A host that wants a program to see a setting
  declares a variable and supplies it: `env?.PORT ?? "8080"` (`12:3.6`).

*Rejected.* `time.now()` backed by a clock the host injects: the same thing as a
variable, with a second mechanism. A `time` type: two more kinds for the
checker, values that are not JSON, and no consumer. Milliseconds since the
epoch, JavaScript's unit: whole numbers, but Unix seconds are what the payloads
and tokens that workflows read carry.

### 2.7 Notation

Signatures are in the type text of `03:2.2`, which is TypeScript's spelling
wherever TypeScript has the same idea (overview Y4). A method row omits the
receiver when the table's heading gives it.

- `int` is a whole number and `float` is any number; `int` fits `float` (N5).
- `T[]` is a list of `T`, and `T[][]` is a list of lists. `map<T>` is a map
  whose values are `T`.
- `T | null` is `T` or null. `[]` binds tighter than `|`: `int[] | null` is a
  list or null, and a list whose elements may be null is `(int | null)[]`.
- `any` is unchecked. `never` is the type of an expression that does not return.
- `(item: T, index: int) => U` is a callback. Every parameter has a name, in a
  signature and in a callback. A callback's names are documentation only. A
  callback is called with every parameter shown, and a lambda may declare fewer
  (`02:2.9`).
- `(item: T, index: int) => unknown` is a predicate. `unknown` appears only
  there, as a predicate's result: the lambda's body may have any type, and its
  value is tested by truthiness (overview B6, `03:2.11`). "`f` is true" in a
  meaning column below means "the result of `f` is truthy".
- **The argument for a callback parameter is a lambda written in place, or the
  name of a `const`-bound lambda** (`xs.map(double)`). Nothing else fits (E9).
  Sixteen entries take a callback. Fifteen are list methods: `filter`, `map`,
  `flatMap`, `some`, `every`, `find`, `count`, `reduce`, `sortBy`, `minBy`,
  `maxBy`, `uniqueBy`, `groupBy`, `indexBy` and `countBy`. One is a map method,
  `mapValues`. Each calls it during the call and does not keep it, and no entry
  returns a function.
- **A callback in a trace** (`10:2.8`). While an entry runs its callback, the
  call is a call in progress. Its trace entry has the entry's name without a
  receiver (`map`, `mapValues`) and the index the callback was given. The
  callback of `mapValues` is given a key and no index: its trace entry has the
  position of the entry in key order, from 0 (I).
- `end?: int` may be omitted. `...rest: T[]` takes zero or more arguments, each
  a `T`: the type after `...` is the list that collects them.
- A parameter may be a set of string literals. There is one: `order`, the second
  parameter of `sortBy`, which is `"asc"` or `"desc"`. A literal argument is
  checked against the set, so `"descending"` is `arg-mismatch`. On `any` data a
  value outside the set is the run error of any argument that does not fit its
  parameter (2.3). It is found before a callback runs, and it is not a `value`
  failure, so `sortBy` has no `!`.
- `where T: ordered` and `where T: number` are the two bounds.
- `!` marks an entry that can fail with kind `value`.
- A method's full text starts with its receiver: `T[].map`, `string[].join`,
  `U[][].flat`, `map<T>.keys`.

The library needs no overloads. Three entries have a narrower receiver than
`string`, `T[]` or `map<T>`: `join` on `string[]`, `flat` on `U[][]` and `toMap`
on `{key: string, value: T}[]`. `03` 2.2 allows this (overview S17), and its
binding rule handles them.

### 2.8 The entries

In each table, Steps is what the entry adds to the call's 1 step, and Units is
what it allocates (2.4).

#### Conversion, inspection and errors (7 functions)

| Signature | Meaning | Null and failure | Steps | Units |
| --- | --- | --- | --- | --- |
| `len: (x: string \| any[] \| map<any>) => int` | The code points of a string, the elements of a list, the entries of a map | | `sz(x)` for a string, else 0 | 0 |
| `string: (x: any) => string` | The rendering of `02:2.13`. A string is returned as it is | Fails on an opaque value (kind `kind`) | 0 for a string. Else 1 per element and entry walked, plus `sz(result)` | 0 for a string. Else `1 + sz(result)` |
| `int: (x: float \| string) => int !` | A number is truncated toward zero. A string has white space removed from both ends, is read by text to number, and is then truncated | Fails on other text. `number-range` if the result is not whole | `sz(x)` for a string, else 0 | 0 |
| `float: (x: float \| string) => float !` | A number is returned. A string has white space removed from both ends and is read by text to number | Fails on other text, and on text that reads as an infinity | `sz(x)` for a string, else 0 | 0 |
| `bool: (x: any) => bool` | Whether `x` is truthy: false for null, `false`, zero, the empty string, the empty list and the empty map, and true for anything else (overview B5) | Never fails | 0 | 0 |
| `type: (x: any) => string` | The kind of `x`: `"null"`, `"bool"`, `"number"`, `"string"`, `"list"`, `"map"` or `"opaque"` | | 0 | 0 |
| `fail: (message: string, data?: any) => never !` | Always fails, with the message. `data` is any value, and null when it is left out | Kind `value`, code `failed`. `catch` catches it, and the handler reads `err.message` and `err.data` (`10:2.3`) | 0 | 0. Nothing is copied |

- `int("1.5")` is 1 and `int("1e3")` is 1000: `int` of a string is `int` of the
  number it reads as, so there is one rule. `int("0x10")`, `int("")` and
  `int("12 apples")` fail.
- `type` names the kind of a value, as a string. It is no longer the only test
  of a value of type `any`: `x is T` is (`07`), and the tables below set the two
  side by side. `type` is not a reserved word. Only `type Name =` at the start
  of a line is taken, for a declaration in a later version, and in 3.0 it is a
  targeted error (D45, `02`). `type(` is always the call.
- `type` has no name for a shape. A value of a tagged union (`09`) is a map, and
  so is the error a `catch` binds (`10:2.3`): `type` gives `"map"` for both. The
  seven names are also how a message names a kind (`10:2.9`), so they do not
  change within a major version.
- `fail` is what `02:2.12` asks for. Its type is `never`, so it joins with
  anything: `match kind { "a" => 1, _ => fail("unknown kind") }`. With `is` it
  gives a failed test the author's own message, where `as` gives the standard
  one: `if (body is Order) { body.total } else { fail("not an order") }`.
- **`fail(message, data?)`** (10-4). `data` carries what a handler or the host
  needs as data and not as text: `fail("over budget", {limit, total})`. A
  handler reads it as `err.data`, and the host's diagnostic has it as `data`
  (`05:A.6`). It is not copied and costs nothing: the message and the data are
  values that already exist. The message is the author's text, whole. It is not
  cut and not taken from the catalog (`10:2.5`).
- **A caught error cannot be raised again unchanged in 3.0.** `fail(err.message,
  err)` raises a new `failed` error that carries the old one as data. The host
  then sees the code `failed` and the span of that `fail` (`10:2.5`).
- **`fail` is the guard, and it is not a test.** `c ? x : fail(m)` stops a run
  on a bad value. A test of a program is a case in a case file (`11`). There is
  no `assert`.
- `bool` is the conversion an author writes for a host that requires a real
  boolean (overview B8): `bool(results.items)`. `!!x` means the same. `bool` is
  kept beside it because it reads as what it does, and because `int`, `float`
  and `string` are the other conversions. A host that tests its result by
  truthiness needs neither, and the workflow engine does so for `when:` (D33).
- `bool`'s argument is a tested position: `bool(x)` on a value that can never be
  falsy, or is always falsy, is reported under the `constant-condition` class of
  `03` (overview B7).

**A conversion is not a test.** `int`, `float`, `string` and `bool` each give a
new value of the kind they are named for. `x as T` gives `x` itself or fails
with `not-conforming`, and `x is T` gives a `bool`. Neither coerces anything
(`07:2.2`). With `x` of type `any`:

| `x` holds | Conversion | Test |
| --- | --- | --- |
| `"42"` | `int(x)` is `42` | `x as int` fails, and `x is int` is `false` |
| `3.0` | `int(x)` is `3` | `x as int` is `3`: a whole number conforms to `int`, however it was written |
| `3.5` | `int(x)` is `3` | `x as int` fails. Nothing is rounded |
| `42` | `string(x)` is `"42"` | `x as string` fails |
| `[]` | `bool(x)` is `false` | `x as bool` fails: only `true` and `false` conform to `bool` |

Where the checker knows the operand's type and no value could conform, as in
`"42" as int` written with the literal, the `as` is the compile error
`never-conforms` (`07:2.6`). The four names are also words of type text. They do
not clash: a type stands only after `as` or `is`, and there `int` is the type.

**`type(x)` against `x is T`.** Each kind name has a test that means the same:

| `type(x) ==` | The same test | Steps of the `is` test (`07:2.7`) |
| --- | --- | --- |
| `"null"` | `x is null`, or `x == null` | 1 |
| `"bool"`, `"string"` | `x is bool`, `x is string` | 1 |
| `"number"` | `x is float` | 1 |
| `"list"` | `x is any[]` | 1. The elements are not visited |
| `"map"` | `x is map<any>` | 1. The entries are not visited |
| `"opaque"` | None. `x is Name` tests one opaque type | |

- **Prefer `is`.** It narrows `x` where it passed, so `x is any[] ? len(x) : 0`
  has no unchecked site. `type(x) == "list"` is a `bool` that narrows nothing.
  `is` also says more: `x is string[]` tests the elements, and `x is int` tests
  for a whole number, which no kind name does.
- **`type` is for the kind as a value:** in a message (`` `got ${type(x)}` ``),
  or as the subject of a `match`.

#### Numbers (8 functions)

| Signature | Meaning | Null and failure | Steps | Units |
| --- | --- | --- | --- | --- |
| `abs: (x: T) => T where T: number` | The magnitude of `x` | | 0 | 0 |
| `floor: (x: float) => int` | The largest whole number not above `x` | `number-range` | 0 | 0 |
| `ceil: (x: float) => int` | The smallest whole number not below `x` | `number-range` | 0 | 0 |
| `round: (x: float) => int` | The whole number nearest to `x`. Of two equally near, the one with the larger magnitude: `round(2.5)` is 3, `round(-2.5)` is -3 | `number-range` | 0 | 0 |
| `min: (a: T, b: T, ...rest: T[]) => T where T: ordered` | The smallest argument. Of equal ones, the first | | 1 per argument after the first; strings add `sz(arguments)` | 0 |
| `max: (a: T, b: T, ...rest: T[]) => T where T: ordered` | The largest argument. Of equal ones, the first | | the same | 0 |
| `range: (a: int, b?: int) => int[]` | With one argument, the whole numbers from 0 up to but not including `a`. With two, from `a` up to but not including `b`. Empty if there are none | | `k` | `1 + k` |
| `fixed: (x: float, digits: int) => string !` | `x` in decimal with exactly `digits` digits after the point: the exact value of `x`, rounded at that digit, halves away from zero. No exponent. A result that is all zeros has no minus sign | Fails if `digits` is not in 0 to 20, or the magnitude of `x` is 1e21 or more | 0 | 1 |

- `floor`, `ceil` and `round` return `int`, because the reason to call them is
  to get a whole number. Two decimals as a number is `round(x * 100) / 100`
  (`02:5`, example 35). Two decimals as text is `fixed(x, 2)`.
- `round` rounds halves away from zero, as expr, v2, Go, C, SQL and spreadsheets
  do. JavaScript's `Math.round(-2.5)` is -2 (X). `fixed` uses the same rule,
  which is also `toFixed`'s (X: `(2.5).toFixed(0)` is `3`).
- `range` is back (E8). It is the only source of a sequence, because there are
  no loops in 3.0 (the words are reserved): `range(pages).map(p => ...)`, or
  batches by `range(ceil(len(xs) / 10)).map(i => xs.slice(i * 10, i * 10 +
  10))`. There is no step argument.
- `fixed` and `padStart` are the two things `sprintf` did that a template string
  does not: `%.2f` and `%05d`.

#### Strings (13 methods)

The receiver `s` is a `string`.

| Signature | Meaning | Null and failure | Steps | Units |
| --- | --- | --- | --- | --- |
| `toUpperCase: () => string` | Each code point replaced by its upper-case entry in the case table | | `sz(s)` | `1 + sz(s)` |
| `toLowerCase: () => string` | The same, with the lower-case entry | | `sz(s)` | `1 + sz(s)` |
| `trim: () => string` | Without the white space at both ends | | `sz(s)` | `1 + sz(result)` |
| `removePrefix: (prefix: string) => string` | Without `prefix` if `s` starts with it, else `s` | | `sz(s)` | `1 + sz(result)` |
| `removeSuffix: (suffix: string) => string` | Without `suffix` if `s` ends with it, else `s` | | `sz(s)` | `1 + sz(result)` |
| `startsWith: (prefix: string) => bool` | Whether the first code points of `s` are those of `prefix`. True for the empty string | | `sz(prefix)` | 0 |
| `endsWith: (suffix: string) => bool` | The same, for the last code points | | `sz(suffix)` | 0 |
| `split: (sep: string) => string[]` | The parts of `s` between occurrences of `sep`, found left to right without overlap. With `p` parts: an empty `sep` gives one string per code point, and `"".split("")` is `[]`. `"".split(",")` is `[""]` | | `sz(s) + p` | `1 + 2p + sz(s)` |
| `replaceAll: (old: string, new: string) => string !` | Each occurrence of `old`, found left to right without overlap, replaced by `new` as literal text | Fails if `old` is empty | `sz(s) + sz(result)` | `1 + sz(result)` |
| `slice: (start: int, end?: int) => string` | The code points from `start` up to but not including `end`, which defaults to the length. A negative position counts from the end. Positions are clamped to the string. Empty if `start` is not before `end` | | `sz(s)` | `1 + sz(result)` |
| `repeat: (count: int) => string !` | `s` repeated `count` times | Fails if `count` is negative | `sz(result)` | `1 + sz(result)` |
| `padStart: (length: int, fill?: string) => string` | If `s` has fewer than `length` code points, `fill` repeated and cut to the missing number, then `s`. `fill` defaults to one space. Unchanged if `fill` is empty | | `sz(result)` | `1 + sz(result)` |
| `padEnd: (length: int, fill?: string) => string` | The same, with the fill after `s` | | `sz(result)` | `1 + sz(result)` |

- `removePrefix` and `removeSuffix` have no JavaScript form. `ref.removePrefix(
  "refs/heads/")` is the everyday case, and the name cannot be mistaken for
  white-space trimming.
- `repeat` and padding are back (E8). Their size is arithmetic on the arguments,
  so the charge comes first.
- A character is `s[i]` (`02:2.6`). Substring test is `in`. Length is `len`.

#### Lists (27 methods)

The receiver is `T[]` unless the row says otherwise. Each callback is called
with the element and its index, named `item` and `index`. The callback of
`reduce` takes the accumulated value, `acc`, before them. In the Steps column
`sz(keys)` is 0 when the keys are numbers (2.4).

| Signature | Meaning | Null and failure | Steps | Units |
| --- | --- | --- | --- | --- |
| `filter: (f: (item: T, index: int) => unknown) => T[]` | The elements for which `f` is true, in order | | `2n` | `1 + k` |
| `map: (f: (item: T, index: int) => U) => U[]` | `f` of each element | | `2n` | `1 + n` |
| `flatMap: (f: (item: T, index: int) => U[]) => U[]` | The elements of `f` of each element, in order | | `2n + k` | `1 + k` |
| `some: (f: (item: T, index: int) => unknown) => bool` | Whether `f` is true for some element. Stops at the first. False for an empty list | | `2v` | 0 |
| `every: (f: (item: T, index: int) => unknown) => bool` | Whether `f` is true for every element. Stops at the first that fails. True for an empty list | | `2v` | 0 |
| `find: (f: (item: T, index: int) => unknown) => T \| null` | The first element for which `f` is true | Null if none | `2v` | 0 |
| `count: (f: (item: T, index: int) => unknown) => int` | How many elements `f` is true for | | `2n` | 0 |
| `reduce: (f: (acc: A, item: T, index: int) => A, init: A) => A` | `f(…f(f(init, x0, 0), x1, 1)…)`, left to right | | `2n` | 0 |
| `sum: () => T where T: number` | The elements added left to right with the language's `+`, starting from 0 | `number-range` | `n` | 0 |
| `min: () => T \| null where T: ordered` | The smallest element. Of equal ones, the first | Null if empty | `n`; strings add `sz(elements)` | 0 |
| `max: () => T \| null where T: ordered` | The largest element. Of equal ones, the first | Null if empty | the same | 0 |
| `sort: () => T[] where T: ordered` | Ascending, by a stable sort | | `n + L × (n + sz(elements))` | `1 + n` |
| `sortBy: (f: (item: T, index: int) => K, order?: "asc" \| "desc") => T[] where K: ordered` | The elements in the order of `f` of each, by a stable sort, in the direction `order` (2.5), which defaults to `"asc"`. `f` is called once per element, in order, before any comparison | | `2n + L × (n + sz(keys))`, in both directions | `1 + n` |
| `minBy: (f: (item: T, index: int) => K) => T \| null where K: ordered` | The element whose key, `f` of it, is the least. Of those with equal keys, the first. `f` is called once per element, in order, before any comparison | Null if empty | `2n + sz(keys)` | 0 |
| `maxBy: (f: (item: T, index: int) => K) => T \| null where K: ordered` | The same, for the greatest key. Of those with equal keys, the first | Null if empty | `2n + sz(keys)` | 0 |
| `groupBy: (f: (item: T, index: int) => string) => map<T[]>` | For each distinct key, the elements with that key, in their original order. `g` is the number of groups | | `2n` | `1 + 3g + n` |
| `countBy: (f: (item: T, index: int) => string) => map<int>` | For each distinct key, how many elements have it. Each count is 1 or more | | `2n` | `1 + 2g` |
| `indexBy: (f: (item: T, index: int) => string) => map<T>` | The map with each element under its key. A later element replaces an earlier one with the same key | | `2n` | `1 + 2n` |
| `toMap: () => map<T>`, receiver `{key: string, value: T}[]` | The map with each record's `value` under its `key`. A later record replaces an earlier one with the same key | | `n` | `1 + 2n` |
| `unique: () => T[] where T: ordered` | The first occurrence of each value, in order | | `n + sz(elements)` | `1 + k` |
| `uniqueBy: (f: (item: T, index: int) => K) => T[] where K: ordered` | The first element with each key, in order. Keys are compared with `==`. `f` is called once per element, in order, before any comparison | | `2n + sz(keys)` | `1 + k` |
| `flat: () => U[]`, receiver `U[][]` | The elements of the inner lists, one level, in order | On `any` data, kind `kind` if an element is not a list | `n + k` | `1 + k` |
| `reverse: () => T[]` | The elements in reverse order | | `n` | `1 + n` |
| `first: () => T \| null` | The first element | Null if empty | 0 | 0 |
| `last: () => T \| null` | The last element | Null if empty | 0 | 0 |
| `slice: (start: int, end?: int) => T[]` | As for strings, on elements | | `k` | `1 + k` |
| `join: (sep: string) => string`, receiver `string[]` | The elements with `sep` between them. Empty for an empty list | | `n + sz(result)` | `1 + sz(result)` |

- **`sort` takes no comparator.** A comparator's calls depend on the sorting
  algorithm, so the steps a program uses would differ between implementations
  (L6), and an inconsistent comparator would give different results. A key
  function is called exactly `n` times.
- **Descending order is the direction of `sortBy`** (D51): `events.sortBy(e =>
  e.ms, "desc")`. An earlier revision said to write `.reverse()` after the sort,
  or to negate a numeric key. Both are withdrawn as advice. `reverse` also turns
  round the elements with equal keys, so it is not a stable descending sort, and
  a string key cannot be negated. For `sort`, which has no key,
  `xs.sort().reverse()` is still right: equal numbers and equal strings cannot
  be told apart.
- **Two keys are two stable sorts, the minor key first, each with its own
  direction.** Customer descending, then `placed` ascending, is `orders.sortBy(o
  => o.placed).sortBy(o => o.customer, "desc")`. The earlier advice failed when
  its two halves were combined: `orders.sortBy(o => o.placed).sortBy(o =>
  o.customer).reverse()` gives `placed` descending, because `reverse` undoes the
  minor key (`13`, section 2). One call with several keys would need a key that
  is a list with a type for each position, and the checker has no tuples
  (`03:2.1`).
- **The direction is a word, and it is optional.** `"asc"` and `"desc"` are
  SQL's words. A boolean would not say which way at the call. An optional last
  parameter is the one change 2.12 allows to a signature, so `sortBy(f)` keeps
  its result and its counts.
- **`minBy` and `maxBy` replace `sortBy(f).first()`.** They need no sort: `2n`
  steps and no units, where the sort charges its formula and `1 + n` units. Both
  take the first of equal keys, as `xs.min()` and `xs.max()` do. So `maxBy(f)`
  is not `sortBy(f).last()`, which takes the last. The key alone is
  `events[*].ms.max()`.
- **`uniqueBy` keeps the first element with each key, and `indexBy` the last.**
  `uniqueBy` gives a list in the original order, and its key is `ordered`: a
  number or a string. `indexBy` gives a map, in key order, and its key is a
  string.
- **A key of `groupBy`, `countBy` and `indexBy` is a string**, because it
  becomes a key of a map. A number is grouped by `string(n)`: `orders.groupBy(o
  => string(o.year))`. The result is in key order by code point (2.5), each
  group in the order of the list.
- **`countBy` is the fused form of `groupBy` and a count.** It does not build
  the groups: `1 + 2g` units, where `groupBy` charges `1 + 3g + n`. Any other
  aggregate over groups is `groupBy(f).mapValues(g => ...)`: revenue for each
  customer is `orders.groupBy(o => o.customer).mapValues(g =>
  g[*].lines[*].map(l => l.qty * l.price).sum())`.
- **`filter` and `find` narrow** (D51, `13:5.1`). Their signatures are
  unchanged. Where the lambda's first parameter is a name, the checker gives the
  result's element the type of that parameter under the true facts of the body
  (`03:2.11`). So `xs.filter(x => x != null)` has no null element, and
  `xs.filter(x => x is Order)` is an `Order[]`. A fact about a field of the
  element changes nothing. `events[*].user.filter(u => u != null).unique()`
  therefore checks: the elements are `string`, which is `ordered`. Only the type
  changes. The value, the steps and the units are those of the table.
- **`flatMap` is back.** `xs.map(f).flat()` builds and pays for a list of lists
  first. expr's `flatMap` also accepts nil and non-list results
  (`expr:docs/reference/spec.md:584-595`) (R). v3's needs a list.
- **`toMap` is the linear way to build a map from data.** Computed keys and
  spread do it one entry at a time: `items.reduce((m, it) => {...m, [it.id]:
  it.status}, {})` (`02:5`, example 36) copies the map at every step, which is
  about `n × n / 2` steps and `n × n` units. For 1,000 items that is past both
  default budgets. With `toMap` it is `items.map(it => {key: it.id, value:
  it.status}).toMap()`. When the value is the element itself, `indexBy` says it
  in one call: `customers.indexBy(c => c.id)`. `toMap` is also the inverse of
  `entries`, so a map is filtered, or its keys are changed, through its entries:
  `m.entries().filter(({value}) => value > 0).toMap()`. To transform the values
  alone, `mapValues` is shorter and cheaper.
- **`reduce` is the general fold**, in JavaScript's argument order. A host
  cannot add a higher-order function (S3), and an author cannot write one: a
  lambda is passed only to a library entry (E9). So `reduce` is how an author
  writes an aggregate the library lacks.
- **`first` and `last`**: `02:7` asks for them, since negative indexes stay out.
- **`join` needs strings.** `[1, 2].join(",")` is a compile error. The hint is
  `xs.map(x => string(x)).join(",")`.
- **`unique` and `sort` need `ordered` elements.** Records are ordered by
  `sortBy` and deduplicated by `uniqueBy`.
- **Left out of the seven:** `sumBy`, because `xs.map(f).sum()` says it, and
  `xs[*].f.sum()` when the body is a path. `uniqueBy` and `countBy` are the
  first to cut (`13:5.1`).

#### Maps (4 methods)

The receiver is `map<T>`, or a record, where `T` is the join of its field types.
A tagged union is read as its flat record (`09:2.4`). The callback of
`mapValues` is called with the value and then its key, named `value` and `key`.

| Signature | Meaning | Steps | Units |
| --- | --- | --- | --- |
| `keys: () => string[]` | The keys, in key order | `n + L × (n + sz(keys))` | `1 + n` |
| `values: () => T[]` | The values, in the order of their keys | the same | `1 + n` |
| `entries: () => {key: string, value: T}[]` | One record per entry, in key order | the same | `1 + 6n` |
| `mapValues: (f: (value: T, key: string) => U) => map<U>` | The map with the same keys, and `f` of each value under its key. `f` is called once per entry, in key order | `2n + L × (n + sz(keys))` | `1 + 2n` |

- `mapValues` never fails by itself and never gives null. A failure of `f` is
  the call's failure (2.3).
- The value comes first, as the element does in a list callback, so
  `prices.mapValues(p => p * 2)` needs no second name. lodash has the same order
  (I).
- On a record the result is a `map<U>`: the field names are not kept in the
  type. The value has the same keys.

| Task | Written |
| --- | --- |
| Test for a key | `"k" in m` |
| Read a key that may be absent, with a default | `m?.k ?? d`, `m?.[key] ?? d` |
| Count entries | `len(m)` |
| Merge; set one key; set a computed key | `{...a, ...b}`; `{...m, k: v}`; `{...m, [key]: v}` |
| Remove a key | `{ const {k, ...rest} = m; rest }` |
| Loop over entries | `m.entries().map(({key, value}) => ...)` |
| Transform the values of a map | `m.mapValues(v => ...)` |
| Filter a map, or change its keys | `m.entries().filter(...).toMap()`, `m.entries().map(...).toMap()` |
| Read a field of every value | `m.values()[*].owner`. A star on a map is an error of kind `kind`, with this hint (`02:2.8`) |
| Build a map from a list | `xs.indexBy(f)`, `xs.groupBy(f)`, `xs.countBy(f)`, `xs.map(x => {key: ..., value: ...}).toMap()` |
| Aggregate each group | `xs.groupBy(f).mapValues(g => ...)` |

*Why four methods and no more.* Each map method is a name an author can confuse
with a data field. v2 has ten, and all ten shadow data
(`v2:pkg/object/map.go:217-226`) (R): `m.keys` returned `builtin(map.keys)` for
a map with a `keys` field (X). `entries` returns records because the checker has
no tuples, and with destructuring a record reads as well as a pair (E3).

*Why `mapValues` is the fourth* (13-3). Earlier revisions had three methods, and
listed `mapValues` as deliberately absent: `entries` with `toMap` covered it.
The workload of `13` showed the price. The only way from a map to a map was two
calls and a list of records. So every aggregate over groups paid for a sort, for
the records of `entries` at `1 + 6n` units, and for a second map. `mapValues`
charges one ordered walk and the result. It is a name that data rarely has, and
`m.mapValues` without a call is still the data (2.1).

*Rejected.* A second callback on `groupBy` in place of `mapValues` and
`countBy`: `events.groupBy(e => e.kind, g => len(g))`. It adds no map method,
but the result type would depend on the number of arguments, and the library has
no overloads (2.7).

#### `math` (opt-in: 12 functions, 2 constants)

| Signature | Meaning | Exactness |
| --- | --- | --- |
| `math.pi`, `math.e` | The doubles nearest to π and e | Exact |
| `math.sqrt: (x: float) => float !` | The square root, correctly rounded as IEEE 754 defines it | Exact |
| `math.pow: (x: float, y: float) => float !` | `x` to the power `y`, for exponents that `**` does not take | Implementation-defined |
| `math.exp`, `math.log`, `math.log10`: `(x: float) => float !` | e to the power `x`; natural and base-10 logarithm | Implementation-defined |
| `math.sin`, `math.cos`, `math.tan`, `math.asin`, `math.acos`, `math.atan`: `(x: float) => float !` | Trigonometry, in radians | Implementation-defined |
| `math.atan2: (y: float, x: float) => float` | The angle of the point `(x, y)` | Implementation-defined |

- Each costs 0 steps and 0 units beyond the call.
- Each `!` entry fails with kind `value` when the mathematical result is not a
  real number (`math.sqrt(-1)`, `math.log(0)`), and with `number-range` when it
  is too large for a double.
- **Implementation-defined means the last digits.** An implementation should
  return a result within one unit in the last place of the true value. When
  these 11 fail and what they cost is exact.
- **How the suite tests them: the case rounds in its own program.** A case has
  no tolerance field, and `value` is compared with `==` like any other
  (`11:2.4`, D49). So a case of the suite never has the bare result of one of
  the 11 as its value. Its program rounds the result to at most 11 significant
  digits, and the case expects that number exactly: `round(math.sin(1) * 1e9)`
  is `841470985`, and `fixed(math.exp(1), 9)` is `"2.718281828"`. The suite
  picks arguments whose true result is not near a half at the digit kept, so
  that two results one unit apart in the last place round the same way. A case
  for a failure, or for steps and units, needs no rounding. An author's own case
  does the same.
- A host that needs identical results in every implementation, or on every
  platform of one, leaves `math` off. In Go and in Rust these functions may
  differ in the last digits between platforms (overview R12).

*Why not exact.* An exact `sin` needs one specified algorithm written in every
implementation, a few thousand lines each, for functions no consumer uses today.
*Why not out.* Scoring and geometry need them, and a host can only add them as
its own functions with the same problem. `02:7` asks that `sqrt` be
implementation-defined. I make it exact, because IEEE 754 requires a correctly
rounded square root and every engine I know provides it (I).

#### `time` (opt-in: 3 functions)

| Signature | Meaning | Failure | Steps | Units |
| --- | --- | --- | --- | --- |
| `time.parse: (s: string) => float !` | The seconds since the epoch of an RFC 3339 date-time | Fails on any other text | `sz(s)` | 0 |
| `time.format: (t: float) => string !` | The RFC 3339 text of `t` in UTC | Fails outside years 0000 to 9999 | 0 | 1 |
| `time.duration: (s: string) => float !` | The seconds a duration text describes | Fails on any other text | `sz(s)` | 0 |

| Rule | Definition |
| --- | --- |
| Accepted form | `YYYY-MM-DDTHH:MM:SS`, an optional `.` and 1 to 9 digits, then `Z` or `+HH:MM` or `-HH:MM`. `T` and `Z` are upper-case. No white space |
| Field ranges | Month 01 to 12; day valid for the month in the proleptic Gregorian calendar; hour 00 to 23; minute and second 00 to 59; offset hour 00 to 23 and minute 00 to 59 |
| Leap seconds | A second of 60 is an error |
| Value | The whole seconds are computed exactly. The result is the double nearest to the exact value with its fraction |
| `format` | `YYYY-MM-DDTHH:MM:SS`, then, if `t` is not whole, `.` and the fraction digits of `t`'s number-to-text form written without an exponent, then `Z`. So `time.format(time.parse(s))` is the same instant |
| Duration text | An optional `-`, then one or more terms with nothing between them. A term is digits, an optional `.` and digits, and a unit: `ms`, `s`, `m`, `h` or `d` (86,400 seconds). The result is the double nearest to the exact sum |

A date is `time.format(t).slice(0, 10)`. "Older than a week" is `time.parse(
pr.updated_at) < now - time.duration("7d")`.

#### `json` and `base64` (opt-in: 2 functions each)

| Signature | Meaning | Failure | Steps | Units |
| --- | --- | --- | --- | --- |
| `json.stringify: (x: any) => string` | The JSON form of `02:2.13` | Fails on an opaque value (kind `kind`) | 1 per element and entry walked, plus `sz(result)` | `1 + sz(result)` |
| `json.parse: (s: string) => any !` | The value that the JSON text describes | Fails on text that is not JSON | `8 × sz(s)`, plus 1 per value created | The units of `04:A.4` for every string, list and map created |
| `base64.encode: (s: string) => string` | RFC 4648 section 4, with `=` padding, of the UTF-8 encoding of `s` | | `sz(s) + sz(result)` | `1 + sz(result)` |
| `base64.decode: (s: string) => string !` | The text whose UTF-8 encoding `s` encodes. The alphabet of section 4 or of section 5; padding may be present or absent | Fails on any other character, on wrong padding, and if the bytes are not UTF-8 | `sz(s)` | `1 + ⌊3 × w(s) / 256⌋` |

| `json` rule | Definition |
| --- | --- |
| `stringify` text | Exactly the JSON form of `02:2.13`: no white space, keys in key order, the listed escapes and no others, numbers by number to text. It differs from `string(x)` for a string (quoted) and for null (the text `null`) |
| `parse` grammar | RFC 8259, and nothing more: no comments, no trailing comma, no leading U+FEFF. White space is its four characters |
| Numbers | A number with no fraction and no exponent must be a whole number, tested on its digits: `9007199254740993` fails, and is not rounded. Any other number is read by text to number |
| Duplicate keys | The last one wins |
| Escapes | A `\u` escape of a surrogate that is not half of a pair reads as U+FFFD |
| Result | Lists, maps, strings, numbers, booleans and null. Its type is `any` |

**The result of `json.parse` is given a type with `as`** (`07`): `const order =
json.parse(msg.text) as Order;`. After that line `order` is checked like any
typed value. The two can fail in two ways, both of kind `value`: text that is
not JSON (`invalid-argument`) and a value that does not conform
(`not-conforming`). `try { json.parse(s) as Order } catch { null }` covers both.
A handler that must tell them apart binds the error and reads its code: `catch
(err) { match err.code { "not-conforming" => ..., _ => ... } }` (`10:2.1`). `as`
tests the type and not a JSON Schema's value constraints (`07:2.9`). `Order` may
be a tagged union (`09`): the test reads the tag, then one variant.

*Note.* JavaScript's `JSON.parse` rounds `12345678901234567890` without an error
(X). v2's `encode(value, format)` chose the codec by a string
(`v2:pkg/builtins/codecs.go:64-71`) (R), which the checker cannot type. Its
`csv`, `base32`, `hex` and `urlquery` codecs have no consumer.

#### Regular expressions: out of 3.0 (T4)

**Recommendation: leave regular expressions out of the 3.0 specification.
Reserve the namespace `regexp`. Add them in a minor version, from the subset
below, when the TypeScript matcher exists and all three implementations pass one
set of cases.** Go's and Rust's engines both run in time linear in the input,
but their default syntax differs, so the subset must be pinned for them too
(overview R11, `01`).

| Reason | Detail |
| --- | --- |
| No consumer has them | expr has no pattern function. The engine's authors have never had one. v2's `regexp.match` cannot even be called (X) |
| They are the most expensive entries to make portable | JavaScript's engine backtracks, so the TypeScript implementation must ship its own matcher. Go's engine can run the subset, but its results must be proved equal on capture groups and empty matches, where engines differ |
| A released entry cannot change (2.12) | A wrong rule for a capture group, or a wrong cost rate, would be permanent. Additions are safe in any minor version |
| The gap is small | `in`, `startsWith`, `endsWith`, `split`, `slice`, `removePrefix` and patterns in `const` cover the cases I found. A host that must match text registers a function, and owns its dialect |

**The candidate subset**, if the owner wants them in 3.0, or for 3.1:

| | In | Out |
| --- | --- | --- |
| Matching unit | Code points | Bytes, UTF-16 units |
| Literals and escapes | Any code point; `\\` and escaped punctuation; `\n \r \t \f \v`; `\x{H...}` | Octal escapes, `\c`, `\Q...\E` |
| Classes | `[abc]`, `[^a-z]`, ranges by code point; `\d` is `[0-9]`, `\w` is `[0-9A-Za-z_]`, `\s` is `[\t\n\f\r ]`, and their negations; `.` is any code point but U+000A | `\p{...}`, POSIX `[[:alpha:]]`, Unicode-aware `\d`, `\w`, `\s` |
| Anchors | `^` and `$` at the ends of the text; `\b` and `\B` on the `\w` set | `\A`, `\z`, `\Z`, `\G` |
| Repetition | `*`, `+`, `?`, `{n}`, `{n,}`, `{n,m}` with counts to 1,000, and the lazy forms. The repeated part must not be able to match the empty string | Possessive forms; a repeat of something that can be empty, which is where engines disagree on groups |
| Groups | `(...)`, `(?:...)`, `(?<name>...)` | Backreferences, look-ahead, look-behind, atomic groups, conditionals |
| Flags | `(?i)` for ASCII letters only, `(?m)`, `(?s)`, at the start of the pattern | Flags inside the pattern; Unicode case folding |
| Semantics | The leftmost match. Among matches that start there, the one a backtracking matcher would find first: alternatives left to right, greedy before lazy | Leftmost-longest |

- **Entries:** `regexp.test`, `regexp.find`, `regexp.findAll`, `regexp.groups`,
  `regexp.replaceAll` and `regexp.escape`, with the pattern first.
- **Cost:** a match simulates all alternatives in one pass, so its time is the
  pattern's size times the text's. Steps would be `8 × (1 + sz(pattern)) ×
  sz(s)`, with the 8 to be calibrated before it is fixed. A pattern too large to
  compile is an error. A literal pattern that is outside the subset is a compile
  error.
- **What the TypeScript implementation must build:** a parser for the subset, a
  compiler to a small instruction list, and a matcher that advances a set of
  threads one code point at a time and keeps their capture positions in priority
  order. About 700 lines (I). It cannot use `RegExp`, which backtracks and
  matches UTF-16 units.
- **What the Go implementation must build:** the same parser, used to reject
  what is outside the subset and to rewrite `(?i)` as explicit classes, and then
  either Go's engine or a port of the same matcher. The port is the safer
  choice: one algorithm in both gives one result.

#### Deliberately absent

| Absent | Write instead, or why not |
| --- | --- |
| `includes`, `contains`, `has` | `in` |
| `indexOf`, `findIndex` | No consumer. JavaScript's `-1` would be a sentinel, and null would be the same text with another meaning. `find`, `in`, and `split` with a list pattern cover the uses |
| `at(i)` | `xs?.[i]`, `first`, `last` |
| A comparator for `sort` | `sortBy`, with a key and a direction: see the lists |
| One `sortBy` with several keys; `thenBy` | Two stable sorts, the minor key first. One call waits for a way to type a key with a kind for each position (`13`, section 7) |
| `sumBy`, `averageBy` | `xs.map(f).sum()`; `xs[*].f.sum()` when the body is a path |
| `sprintf`, `format` | A template string, `fixed` and `padStart`. Format verbs cannot be checked |
| `trimStart`, `trimEnd`, `capitalize`, `lines` | No consumer. `s[0].toUpperCase() + s.slice(1)`; `s.split("\n")` |
| `dedent`, `trimIndent`, `stripIndent` | A text block, `"""`, removes its indentation when the program is parsed, at no step and no unit. A call would run after interpolation and be charged at every evaluation (`08:2.8`) |
| `indent` | `v.replaceAll("\n", "\n    ")` for the later lines of a value put into a text block (`08`, section 3) |
| `chunk`, `zip`, `take`, `drop` | `slice(0, n)`, and `range` with `slice` |
| `pick`, `omit`, `mapKeys`, `filter` on a map | A rest pattern; `entries` with `toMap` |
| A `merge` that goes deep | Spread merges one level: `{...base, db: {...base.db, port: 1}}`. Later, if config files ask for it (`12:3.8`) |
| `each`, `forEach` | An expression has no effect to loop for |
| `assert` | `c ? x : fail(m)` inside a program. A test is a case in a case file (`11`) |
| Raising a caught error again unchanged: `rethrow`, `fail(err)` | Not in 3.0. `fail(err.message, err)` carries the old error as data (`10:2.5`) |
| `now`, `rand`, `env.get`, `getenv` | 2.6 |
| Anything that depends on a locale | It cannot be exact |
| URL escaping, hex, CSV | No consumer. A host function is a few lines |
| Regular expressions | Above |

Entries that are **in** without a consumer today: `groupBy`, `unique`, `toMap`,
`removePrefix`, `removeSuffix`, `repeat`, `padStart`, `padEnd`, `fixed`,
`range`, `type`, and the `time` and `base64` groups. Under C4 I keep them: a
host can add neither a method nor a higher-order function (S3), so a missing
operation on a list, a map or a string cannot be supplied from outside. An
author cannot supply one that takes a function either: a `const`-bound lambda
cannot take a lambda (E9). The library is the only place a lambda can go, so an
absent operation that takes a function is written out at each use.

That argument is why four rows of this table were reversed (13-3). `minBy`,
`maxBy`, `uniqueBy` and `mapValues` stood here, with `sortBy(f).first()` and
`entries` with `toMap` as what to write. Each takes a function, so no host and
no author could supply it, and the workload of `13` showed what writing it out
costs. They are entries now, with `indexBy`, `countBy` and the direction of
`sortBy`. The seven are in on the owner's request for queries, not for a
consumer of today.

`type` is the one entry that `as` and `is` make mostly redundant. It was in as
the only test of untyped data. It stays (D45), for the kind as a string. No
other entry exists only because untyped data could not be tested.

### 2.9 Where the same text means something else than in JavaScript

`02:3.1` lists the language's departures. These are the library's. Everything
else that is valid in both means the same, or is a compile error.

| Text | JavaScript | v3 | Why |
| --- | --- | --- | --- |
| `[10, 9, 1].sort()` | `[1, 10, 9]`: elements are compared as text (X) | `[1, 9, 10]` | JavaScript's result is its best-known library trap. Nobody writes this wanting it |
| `"ß".toUpperCase()`, a final `Σ` lower-cased, `"İ".toLowerCase()` | `SS`, `ς`, two code points (X) | `ß`, `σ`, `i` | One code point to one, from a table the specification owns, is the widest rule that is exact and keeps the length |
| `s.replaceAll("b", "$&")` | `$&`, `$1` and `$$` in the replacement are special (X) | The replacement is literal text | No patterns, so nothing to refer to |
| `s.replaceAll("", "-")` | `-a-b-` (X) | An error | Almost always a bug |
| `"a😀".split("")`, `.slice`, `.padStart`, `len` | UTF-16 units: three parts (X) | Code points: two parts | T1 |
| `xs.reduce(f)` with no start value | Uses the first element | A compile error | The start value types the result |
| `xs.join()` | Joins with `,` and converts elements | A compile error | Explicit, and nothing converts |
| `m.entries()` | Pairs, on a `Map` | Records `{key, value}` | No tuples in the checker |
| `m.keys()`, `json.stringify(m)` | Insertion order | Key order (T2) | A map has no order of its own |
| `json.parse(s) as Order` | TypeScript's `as` is trusted: `JSON.parse(s) as Order` tests nothing, and a wrong value fails later or never | Tested when it runs: the call fails with `not-conforming` unless the value conforms | An untested `as` would end the checker's promise. `02:3.1` has the row for `as` itself (`07`) |
| `xs.filter(x => x.tags)`, and the callbacks of `find`, `some`, `every` and `count` | Keeps an element whose `tags` is an empty array, because `[]` is truthy | Drops it: an empty list or map is falsy (overview B5) | The one point where v3's truthiness differs from JavaScript's (`02:3.1`) |

### 2.10 Groups and the default environment

| Group | Entries | Default | Why here |
| --- | --- | --- | --- |
| `core` | 15 functions, 44 methods | on | Exact, and what every consumer uses |
| `math` | 12 functions, 2 constants | off | The only implementation-defined entries |
| `time` | 3 functions | off | No consumer today |
| `json` | 2 functions | off | `parse` returns `any`. Under a strict host its result is used only after `as` or `is` (`07`) |
| `base64` | 2 functions | off | No consumer today |
| `regexp` | none in 3.0 | reserved | 2.8 |

- **A group is a namespace** (S1). Every opt-in entry is written `group.name`.
  An author can tell from the text which entries are opt-in, and a group adds
  one name to the scope. The namespace is a compile-time prefix, not a value.
  Declaring, enabling and clashes are `05:A.2`.
- **`core` is on by default.** A pure library cannot widen what a program does
  to its host.
- **A host can remove a group or a single entry** (`05:A.2`). A host that wants
  no general fold removes `reduce`. A host with a step named `range` removes
  `range`.
- A reserved namespace cannot be declared by a host (`05:A.2`), so `regexp` can
  be added later without breaking one.

*Rejected.* expr's flat groups (`expr:builtin_groups.go:25`, `:46`, `:70`) (R):
bare `log`, `e` and `exp` are names a host's data wants, and nothing in the text
says which functions are opt-in.

### 2.11 The specification table

A library entry is a record of data. The table of all entries is part of the
specification, in one JSON file, `library.json`, beside the conformance files of
`05:A.11` and the message catalog, `messages.json` (`10:2.9`). JSON, because
every implementation already reads it for those files.

| Field | Content |
| --- | --- |
| `name` | `filter`, `len`, `math.sqrt` |
| `receiver` | The receiver type for a method, in type text; absent for a function |
| `signature` | The `Sig` text of `03:2.2` |
| `group`, `since` | The group, and the specification version that added the entry |
| `exact` | `true`, or `false` for implementation-defined. There is no tolerance: the cases of such an entry round in their own programs (2.8, `math`) |
| `doc` | One sentence. Not normative |
| `calls` | The parameters that are functions the entry calls. They are never kept (`04:4`) |
| `narrows` | The predicate whose true facts narrow the element type of the result (`03:2.11`). Present on `filter` and `find` only |
| `steps`, `units` | The two formulas of 2.4, as text over `n`, `k`, `v`, `g`, `w(p)`, `sz(p)`, `len(p)` and `L`, with `+`, `×` and rounding down. **Required** |
| `null` | When the result is null |
| `fails` | Each way the call fails, as a record: `when`, the condition; `kind`, as in 2.3; `code`; and `template`, the name of that code's template in `messages.json`, when the code has more than one. A `value` failure always has the code `invalid-argument` and a template of 2.3 |
| `wrong` | Names people try, for hints. Not normative. A hint is `help`, which no case compares |
| `cases` | The conformance cases for the entry: a case file with no header (`11:2.2`) |

```json
{
  "name": "filter", "receiver": "T[]",
  "signature": "(f: (item: T, index: int) => unknown) => T[]",
  "group": "core", "since": "3.0", "exact": true,
  "doc": "The elements for which f is true, in order.",
  "calls": ["f"], "narrows": "f", "steps": "2*n", "units": "1 + k",
  "null": null, "fails": [],
  "wrong": ["select", "where"],
  "cases": [{
    "name": "filter keeps order",
    "source": "xs.filter(x => x > 1)",
    "env": "{xs: int[]}", "vars": {"xs": [3, 1, 2]},
    "expect": {"type": "int[]", "value": [3, 2], "steps": 11, "allocated": 3}
  }]
}
```

The case uses 1 step for the call, 1 for the lambda, 6 for the three visits and
calls, and 3 for the comparisons (`04:A.3`, as I read it).

An entry that can fail names the code and the template of each failure. These
are the two lines of `repeat` and of `floor`:

```json
"fails": [{"when": "count < 0", "kind": "value",
           "code": "invalid-argument", "template": "negative-count"}]

"fails": [{"when": "the result is not a whole number", "kind": "range",
           "code": "number-range"}]
```

- **The `cases` of an entry are a case file with no header** (`11:2.2`): a JSON
  array. `risor test` runs it as it runs any case file. `risor conform` is gone
  (D49). The cases of an opt-in group need the group on. `11:2.2` lists no field
  of a case that does that, so this document assumes one, `groups`, as an
  environment has (`05:A.2`, section 5).
- **A case that expects a failure states its message** (`10`, section 4), as
  `{"phase": "run", "diagnostics": [{"code": "invalid-argument", "class":
  "value", "span": [...], "message": "..."}]}`. The text is the template of
  `messages.json` with its parameters filled in.
- A case that needs a type the host expects writes the field `returns`. The
  field was named `expected` before D49.

**What is generated from the table, and what is checked against it.**

| Artifact | Relation |
| --- | --- |
| The checker's signatures, in each implementation | Generated |
| The run-time argument checks for `any` values | Generated from the signature. No entry checks its own argument count or kinds |
| The reference page, completion and hover text, the hint table | Generated |
| The message of each `value` failure | Generated from `messages.json`, by the template its `fails` line names. No implementation writes a message by hand (`10:2.9`) |
| The loader's cross-check | Every `template` in a `fails` line is a template of `invalid-argument`, and every template of 2.3 is named by a `fails` line |
| A compact listing for a host's own documentation, limited to what its environment enables | Generated. The engine keeps such a list by hand today (`wf:documentation/definitions.md:348-370`) (R) |
| Each implementation of an entry | Hand-written, and checked by the cases and by the accounting test |

**Examples are conformance cases (P4).** Each entry has at least one case for
the normal result, one for each `null` and `fails` line, and one at an edge: an
empty list, a code point above U+FFFF, a number at the end of the whole range.
The cases are in the format of `05:A.11`, with the result's steps and units, so
they run unchanged in every implementation. The case table, `casemap.json`, has
a case for every code point in it.

The entries of D51 have these cases beside the usual ones (`13:6.4`): equal keys
under both directions of `sortBy`, and two sorts for two keys; equal keys in
`minBy`, `maxBy`, `uniqueBy` and `indexBy`, to fix which element wins; string
keys above U+FFFF; on `any` data a mix of kinds, a null key and a direction
outside the set; an empty list and an empty map; a callback that fails, with its
trace; and `filter` and `find` with a test that narrows, compared by `type`.

**A declared charge is a condition of registering.** The table's loader rejects
an entry with no `steps` or no `units`. Each implementation's own registry does
the same when it starts, so an entry cannot be added in code alone.

**The accounting test.** Every implementation runs it for every entry. It
follows the harness of Canonical's Starlark fork (`01:4.2`).

| Step | Check |
| --- | --- |
| 1. Generate | For each entry, arguments of size N, for N of 0, 1, 2, 63, 64, 65, 1,000 and 100,000. Size means elements for a list, entries for a map and code points for a string. Generators come from the parameter types, in several shapes: all equal, all distinct, already sorted, reversed, every position a match, no match, text above U+FFFF. Callbacks are fixed lambdas whose cost is known, and for an entry that takes a key they give keys in the same shapes, as numbers and as strings. An optional parameter is tried present and absent |
| 2. Declared against charged | The steps and units the run reports must **equal** the formulas evaluated on those arguments and that result, at every N and shape |
| 3. Charged before work | With a budget one short of each formula, the run fails with the limit error, and the implementation's allocation counter shows the result was not built |
| 4. Charged against real work | Time per step at N of 100,000 is at most a fixed factor above time per step at 1,000. Memory created per unit is under the implementation's published bytes per unit (`04:A.6`). This is what catches an entry whose formula is linear and whose code is not |
| 5. No mutation | The arguments are deeply equal before and after |
| 6. Determinism | A second run, with maps built in another insertion order, gives the same result, steps and units |

Steps 2, 3, 5 and 6 are exact and run in the shared suite. Step 4 measures, so
each implementation runs it with its own thresholds.

### 2.12 Versions

The library is versioned with the specification: one number, `major.minor`
(`05:A.10`). An environment states the version it wants, and an entry whose
`since` is later does not exist in it.

| Part of the table | Within a major version |
| --- | --- |
| The set of entries | Only grows. A minor version may add entries and groups |
| An entry's signature | May gain an optional last parameter, if every existing call keeps its result and its counts. Nothing else changes |
| An entry's result and failures, for any arguments | Never change |
| An entry's `steps` and `units` | Never change (L6) |
| The shared rules of 2.5, the case table, the white space set, the rates of 2.4 | Never change |
| The result of an implementation-defined entry | May differ between implementations and their releases, in the last digits (2.8, `math`). Its failures and counts do not |
| The code of each failure, and the text of its template in `messages.json` | Never change. A program reads both, as `err.code` and `err.message`, and pays for the message's size (`10:2.9`) |
| `doc`, `wrong`, hints, and added cases | Free to change |

**Changing a released entry is not possible. Replacing it is.** A fix that
changes a result or a count ships as a new entry with a new name, and the old
one stays. A newer Unicode version for case mapping would be a new pair of
entries, or the next major version. A minor version that adds a name a host
already declares is an error when that host next builds its environment
(`05:A.2`), never a silent change.

*Why so strict.* A host stores programs and replays them. The engine promises
the same states from the same events (`wf:llms.txt:520-521`). A changed count
moves the point where a stored program reaches its limit.

## 3. Inventory

### 3.1 Risor v2: 175 entries

25 builtin functions, 86 methods and 8 properties on 9 types, 51 module
functions and 5 module constants. All (R) unless marked.

| Disposition | Count | Meaning |
| --- | ---: | --- |
| Keep | 37 | Same name, though behavior may change as noted |
| Rename | 15 | Same operation under a new name or on a new receiver |
| Merge | 12 | An operator or another entry does the job |
| Drop | 111 | No form in 3.0. 20 of these are regular expression entries, deferred |

**Builtin functions** (`v2:pkg/builtins/registry.go:18-219`)

| v2 | v3 | Reason |
| --- | --- | --- |
| `all(xs)` | merge: `xs.every(f)` | Takes a callback and stops early. `xs.every(x => x)` is the old meaning |
| `any(xs)` | merge: `xs.some(f)` | The same |
| `assert(cond, msg)` | drop | `cond ? x : fail(msg)` inside a program. A test is a case in a case file (`11`), which replaces v2's `test_` functions too |
| `bool(x)` | keep | The same falsy set as v2 (overview B5) |
| `byte(x)`, `bytes(x)` | drop (2) | No byte types |
| `call(f, ...)` | drop | Functions are not values |
| `chunk(xs, n)` | drop | No consumer; `range` with `slice` |
| `coalesce(...)` | merge: `a ?? b` | The operator exists and is lazy |
| `decode(x, format)` | rename: `json.parse`, `base64.decode` | A format chosen by a string cannot be typed. The parsed value gets its type from `as` |
| `encode(x, format)` | rename: `json.stringify`, `base64.encode` | The same |
| `error(msg, ...)` | drop | An error is not a kind of value. `fail(message, data?)` raises, and a `catch` binds a map (`10:2.3`) |
| `filter(xs, f)` | merge: `xs.filter(f)` | One spelling |
| `float(x)` | keep | |
| `getattr(x, name, d)` | merge: `x?.[name] ?? d` | Operators |
| `int(x)` | keep | Base 10 only; trims white space |
| `keys(x)` | merge: `m.keys()` | One spelling |
| `len(x)` | keep | |
| `list(x)` | drop | No iterators to convert |
| `range(...)` | keep | Returns a list; no step argument |
| `reversed(x)` | rename: `xs.reverse()` | Nothing mutates, so the copy takes the short name |
| `sorted(x, cmp)` | rename: `xs.sort()`, `xs.sortBy(f)` | A key and a direction replace the comparator |
| `sprintf(f, ...)` | drop | Template strings, `fixed`, `padStart` |
| `string(x)` | keep | null is `""`; lists and maps are compact JSON |
| `type(x)` | keep | Returns kind names; `"null"` for null, as v2 does (X). A v2 test such as `type(x) == "list"` still works; `x is any[]` is preferred, because it narrows (2.8) |

**List methods** (`v2:pkg/object/list.go:15-151`)

| v2 | v3 | Reason |
| --- | --- | --- |
| `append(x)`, `extend(ys)` | drop (2) | Mutate. Spread |
| `clear()`, `insert(i, x)`, `pop(i)`, `remove(x)` | drop (4) | Mutate |
| `copy()` | drop | An immutable value needs no copy |
| `count(x)` | keep | Now takes a predicate. v2 counted equal items, so `xs.count(x => x > 1)` returned 0 (X) |
| `each(f)` | drop | No effects to loop for |
| `filter(f)` | keep | |
| `index(x)` | drop | `-1` sentinel; no consumer |
| `map(f)` | keep | The callback gets `(item, index)`. v2 passes `(index, value)` (X) |
| `reduce(init, f)` | keep | Argument order is `(f, init)` |
| `reverse()`, `sort()` | keep (2) | Return a new list |

**Map methods** (`v2:pkg/object/map.go:18-154`)

| v2 | v3 | Reason |
| --- | --- | --- |
| `keys()`, `values()` | keep (2) | Return a list, not an iterator |
| `entries()` | keep | A list of `{key, value}` records, not pairs |
| `get(k, d)` | merge: `m?.[k] ?? d` | Operators |
| `update(other)` | merge: `{...m, ...other}` | Spread |
| `each(f)`, `pop(k, d)`, `setdefault(k, v)`, `clear()`, `copy()` | drop (5) | Effects or mutation |

**String methods** (`v2:pkg/object/string.go:18-160`)

| v2 | v3 | Reason |
| --- | --- | --- |
| `compare(other)` | drop | `<` and `==` |
| `contains(sub)` | merge: `sub in s` | Operator |
| `count(sub)`, `fields()`, `index(sub)`, `last_index(sub)` | drop (4) | No consumer; two return `-1` |
| `has_prefix(p)`, `has_suffix(p)` | rename (2): `startsWith`, `endsWith` | JavaScript's names |
| `join(xs)` | rename: `xs.join(sep)` | The list is the subject |
| `repeat(n)` | keep | |
| `replace_all(old, new)` | rename: `replaceAll` | |
| `split(sep)` | keep | |
| `to_lower()`, `to_upper()` | rename (2): `toLowerCase`, `toUpperCase` | |
| `trim(chars)` | drop | The cutset form has no consumer; the name now means white space |
| `trim_prefix(p)`, `trim_suffix(p)` | rename (2): `removePrefix`, `removeSuffix` | |
| `trim_space()` | rename: `trim` | |

**Other types**

| v2 | v3 | Reason |
| --- | --- | --- |
| bytes: `clone`, `contains`, `contains_any`, `contains_rune`, `count`, `equals`, `has_prefix`, `has_suffix`, `index`, `index_any`, `index_byte`, `index_rune`, `repeat`, `replace`, `replace_all` (`v2:pkg/object/bytes.go:16-127`) | drop (15) | No bytes type |
| time: `after`, `before` (`v2:pkg/object/time.go:21-27`) | merge (2): `>`, `<` | Times are numbers |
| time: `format(layout)` (`:33`) | rename: `time.format(t)` | RFC 3339 in UTC only |
| time: `add_date`, `unix`, `utc` (`:15`, `:39`, `:44`) | drop (3) | No consumer; the value is already seconds in UTC |
| error: `error`, `message`, `line`, `column`, `filename`, `source`, `stack`, `kind` (`v2:pkg/object/error.go:17-90`) | drop (8) | An error is not a kind of value. The error a `catch` binds is a map with `code`, `message`, `path` and `data`, and no method (`10:2.3`). A position and a trace are the host's, on the diagnostic |
| range: `start`, `stop`, `step` (properties), `map`, `filter`, `each` (`v2:pkg/object/range.go:13-50`) | drop (6) | `range` returns a list |
| color: `rgba` (`v2:pkg/object/color.go:15`) | drop | No color type |
| builtin: `__name__`, `__module__`; module: `__name__` (`v2:pkg/object/builtin.go:15-22`, `v2:pkg/object/module.go:14`) | drop (3) | Functions and modules are not values |
| regexp object: 10 methods and 2 properties (`v2:pkg/modules/regexp/regexp_object.go:83-325`) | drop (12) | Deferred with the group |

**Modules**

| v2 | v3 | Reason |
| --- | --- | --- |
| `math.pi`, `math.e` (`v2:pkg/modules/math/math.go:528-529`) | keep (2) | |
| `math.tau`, `math.inf`, `math.nan` (`:530-532`) | drop (3) | There is no infinity or NaN |
| `math.abs`, `floor`, `ceil`, `round` (`:535-539`) | keep (4): core | The last three return a whole number |
| `math.min`, `math.max` (`:544-545`) | keep (2): core `min`, `max`, and `xs.min()`, `xs.max()` | v2 takes arguments or one list in the same function (X) |
| `math.sum(xs)` (`:546`) | rename: `xs.sum()` | The list is the subject |
| `math.trunc` (`:540`) | merge: `int(x)` | The same operation |
| `math.sign`, `math.clamp` (`:536`, `:541`) | drop (2) | `clamp` is `min(max(x, lo), hi)` |
| `math.sqrt`, `pow`, `exp`, `log`, `log10` (`:549-554`) | keep (5) | In the `math` group |
| `math.cbrt`, `math.log2` (`:550`, `:555`) | drop (2) | No consumer |
| `math.sin`, `cos`, `tan`, `asin`, `acos`, `atan`, `atan2` (`:558-564`) | keep (7) | In the `math` group |
| `math.hypot`, `sinh`, `cosh`, `tanh`, `degrees`, `radians` (`:565-574`) | drop (6) | No consumer |
| `math.is_finite`, `is_inf`, `is_nan` (`:577-579`) | drop (3) | Nothing to test for |
| `rand`: 10 functions (`v2:pkg/modules/rand/rand.go:267-276`) | drop (10) | Nondeterministic |
| `regexp`: 8 functions (`v2:pkg/modules/regexp/regexp.go:330-337`) | drop (8) | Deferred: 2.8 |

### 3.2 expr: 40 entries

12 builtins, 18 functions in three opt-in groups, and 10 special forms
(`expr:builtins.go:41-56`, `expr:builtin_groups.go:25-80`,
`expr:docs/reference/spec.md:556-577`) (R). 10 keep their name and shape, 19
become methods of the same name, 5 become methods under JavaScript's name, 4
merge into an operator or syntax, and 2 are dropped.

| expr | v3 | Reason |
| --- | --- | --- |
| `len(v)`, `int(v)`, `float(v)` | keep | null is an error, where expr's nil is 0 |
| `string(v)` | keep | Lists and maps render as JSON |
| `bool(v)` | keep | The same falsy set as expr (overview B5) |
| `contains(h, n)`, `has(m, k)` | merge: `n in h`, `k in m` | Operator |
| `keys(m)`, `entries(m)` | method | Shape rule |
| `lower(s)`, `upper(s)` | rename: `s.toLowerCase()`, `s.toUpperCase()` | 2.2 |
| `sprintf(f, ...)` | drop | Template strings, `fixed`, `padStart` |
| `min`, `max`, `abs` | keep | `min` and `max` need two arguments |
| `floor`, `ceil`, `round` | keep | Return a whole number |
| `trim(s)`, `split(s, sep)`, `startsWith(s, p)`, `endsWith(s, p)` | method | Shape rule |
| `join(xs, sep)` | method: `xs.join(sep)` | Shape rule |
| `replace(s, old, new)` | rename: `s.replaceAll(old, new)` | JavaScript's `replace` means the first occurrence |
| `first(xs)`, `last(xs)`, `sum(xs)`, `sort(xs)`, `reverse(xs)` | method | Shape rule |
| `slice(xs, i, j)` | method: `xs.slice(i, j)`, `s.slice(i, j)` | Shape rule |
| `map`, `filter`, `find`, `count`, `flatMap`, `sortBy` | method with an arrow function | No `it`; `(x, i) => ...` gives the index |
| `any`, `all` | rename: `xs.some(f)`, `xs.every(f)` | 2.2 |
| `try(v, fallback)` | merge: `try { v } catch { fallback }` | Language syntax (`10:2.2`, D48). Lazy, so it could never be a function. The call form is a targeted error that names the new form |
| `if(c, a, b)` | merge: `c ? a : b`, `if (c) { a } else { b }`, `match` | Language syntax. `if` is a keyword, so the call form is an error |

### 3.3 The workflow engine

What an author writes today (`wf:documentation/definitions.md:348-370`,
`wf:expr.go:19-66`) (R), and in v3. Calls counted in the engine's documentation,
examples and `llms.txt` (X): `coalesce` 14, `len` 8, `filter` 6, `try` 5, `map`
4, `concat` 3, `string`, `int`, `if` and `any` 2 each, `max` and `all` 1 each,
and no other function.

| Today | v3 | Note |
| --- | --- | --- |
| `len(v)` | `len(v)` | For a value that may be null: `len(v ?? [])` |
| `any(xs, it.ok)`, `all(xs, it.ok)` | `xs.some(x => x.ok)`, `xs.every(x => x.ok)` | |
| `filter(xs, it.ok)`, `map(xs, it.name)` | `xs.filter(x => x.ok)`, `xs.map(x => x.name)` | A body that is only a path may be a projection: `xs[*].name` (`02:2.8`) |
| `map(reviews, r, r.author)` | `reviews.map(r => r.author)` | Or `reviews[*].author` |
| `find(xs, it.ok)`, `count(xs, it.ok)` | `xs.find(x => x.ok)`, `xs.count(x => x.ok)` | |
| `flatMap(xs, it.items)` | `xs.flatMap(x => x.items)` | The result must be a list. Or `xs[*].items[*]` |
| `sortBy(xs, it.score)` | `xs.sortBy(x => x.score)` | Descending is new: `xs.sortBy(x => x.score, "desc")` |
| `index` inside a form | `(x, i) => ...` | The `each` step's `index` is no longer hidden |
| A nil list in any form | `xs?.filter(f)`, or `(xs ?? []).filter(f)` | Forms treat nil as empty today |
| `coalesce(a, b)` | `a ?? b` | Or the engine keeps its own `coalesce` for its type rule |
| `concat(a, b)` | `[...a, ...b]` | Null: `...(a ?? [])`. A single item: `[...a, x]` |
| `try(e, fallback)` | `try { e } catch { fallback }` | The call form is a targeted error that names this one. As an operand it takes parentheses: `(try { e } catch { 0 }) > 10`. The 5 uses must be rewritten |
| `if(c, a, b)` | `c ? a : b`, `if (c) { a } else { b }`, or `match` | `if` is a keyword, so the call form is an error |
| `decide(q, s)`, `choose(q, s, opts)` | the same | Host functions |
| `string(v)`, `int(v)`, `float(v)` | the same | `int(null)` and `float(null)` fail |
| `bool(v)` | the test itself | `len(v) > 0`, `v != null` |
| `contains(h, n)`, `has(m, k)` | `n in h`, `k in m` | |
| `keys(m)`, `entries(m)` | `m.keys()`, `m.entries()` | |
| `lower(s)`, `upper(s)`, `trim(s)` | `s.toLowerCase()`, `s.toUpperCase()`, `s.trim()` | |
| `split(s, sep)`, `join(xs, sep)` | `s.split(sep)`, `xs.join(sep)` | |
| `replace(s, old, new)` | `s.replaceAll(old, new)` | |
| `startsWith(s, p)`, `endsWith(s, p)` | `s.startsWith(p)`, `s.endsWith(p)` | |
| `sprintf("%s: %d", a, n)` | `` `${a}: ${n}` `` | `%.2f` is `fixed(x, 2)`; `%05d` is `string(n).padStart(5, "0")` |
| `min(a, b)`, `max(a, b)`, `abs(n)` | the same | `min(a)` with one argument is an error |
| `floor(v)`, `ceil(v)`, `round(v)` | the same | The result is a whole number |
| `first(xs)`, `last(xs)`, `sum(xs)` | `xs.first()`, `xs.last()`, `xs.sum()` | |
| `slice(xs, i, j)` | `xs.slice(i, j)` | |
| `sort(xs)`, `reverse(xs)` | `xs.sort()`, `xs.reverse()` | |
| `xs \| filter(it.ok) \| map(it.name)` | `xs.filter(x => x.ok).map(x => x.name)` | |

The step names reserved because of functions fall from 44 to at most 15, the
core functions, and a host can remove any of those. The six entries of D51 are
methods, so they reserve no name.

## 4. Alternatives rejected

| Alternative | Why not |
| --- | --- |
| Functions for everything, with a pipe (expr's shape) | Every function is a name taken from the host's variables, and the pipe is a second call syntax |
| Both shapes for common operations, as in v2 (`filter(xs, f)` and `xs.filter(f)`) | Two spellings of one thing |
| expr's names for methods (the first draft): `any`, `all`, `upper`, `lower`, `flatten` | 2.2. Kept for the functions, where JavaScript has no shape to copy |
| snake_case (v2) | The syntax is JavaScript's |
| `includes` beside `in`; `at` beside `?.[`; `length()` beside `len` | Two spellings. The hint costs one round trip |
| ASCII-only case mapping | Exact and cheap, and wrong for `é`. 2.5 |
| JavaScript's full case mapping | It changes lengths, needs context rules for `Σ`, and differs by engine version |
| A comparator for `sort` | The number of calls would depend on the algorithm (L6) |
| Descending order as `.reverse()` after `sortBy` (an earlier revision's advice) | It turns round elements with equal keys, so two sorts for two keys give the wrong order. `sortBy(f, "desc")` is stable (2.8) |
| A boolean, or a second entry, for the direction: `sortBy(f, true)`, `sortByDesc(f)` | A boolean does not say which way at the call. A second entry doubles `sortBy` for one word |
| One `sortBy` with several keys | The key would be a list with a type for each position, and the checker has no tuples (`03:2.1`) |
| A second callback on `groupBy` in place of `mapValues` and `countBy` | The result type would depend on the number of arguments. The library has no overloads (13-3) |
| Removing `map`, `flatMap` or `flat` now that a projection says the same for a path | They are what a TypeScript author writes first, and the first two are the general forms (2.1) |
| A `tolerance` field in a case, for the implementation-defined entries | A second way to compare a value. The case rounds in its own program (2.8, D49) |
| A library `dedent` | It runs after interpolation and is charged at every evaluation. A text block is settled when the program is parsed (`08`, section 7) |
| `fail(err)`, to raise a caught error again unchanged | Not in 3.0 (10-4). `fail(err.message, err)` carries it as data |
| Charging after building (the first draft, for `replaceAll`) | L3. Every size is now found first: 2.4 |
| Counting the real work (comparisons made, bytes scanned) | The counts would depend on each implementation's algorithms |
| Leaving `range`, `repeat` and padding out (the first draft) | Their reason was unbounded output. Allocation charged first removes it (E8) |
| `sprintf` in an opt-in group | Its verbs are untyped, and its exact output is one language's formatting rules |
| Regular expressions in 3.0, on each implementation's own engine | The results differ, and JavaScript's engine is not linear (T4) |
| Transcendental functions specified exactly | Thousands of lines in every implementation, for no consumer |
| A `time` type; milliseconds | 2.6 |
| Lazy iterators | A second kind of sequence for the checker, and not a value a program can return |
| A full map API (`get`, `has`, `merge`, `filter`, `pick`, `omit`) | Each name can be confused with a field. Operators, `entries` and `toMap` cover them. `mapValues` was in this row and is now an entry (13-3): 2.8 |
| A registry defined in Go, with the specification derived from it | C1: the table is the definition, and each implementation is checked against it |
| Null-tolerant functions (expr's rule) | 2.3 |

## 5. Rulings applied, and what is still assumed

| Ruling | Where it lands |
| --- | --- |
| C1, P1 to P4 | Section 2 is normative and names no implementation. The table of 2.11 is the definition. Examples are conformance cases |
| P3, T1, T2 | 2.5: code points, the white space set, the case table, key order |
| T3 | 67 exact, 11 implementation-defined, all in `math` |
| T4 | Out of 3.0; the candidate subset is in 2.8 |
| C2, E7 | 2.2 and 2.9 |
| C4, E1 to E4 | `toMap`, `fail`, `flatMap`, `fixed`. Nothing became redundant: patterns, computed keys, `match`, blocks and string indexing replace operators and idioms, not entries. `math.pow` stays for exponents that are not whole |
| E8 | `range`, `repeat`, `padStart`, `padEnd` are in |
| C5, L1 to L4, L6, L7, S8 | 2.4: two formulas per entry in `04`'s units, charged first; the accounting test of 2.11 |
| N1 to N7 | One kind of number. `int` in a signature is a whole number. `int`, `floor`, `ceil`, `round` and `sum` fail with `number-range`. Text and numbers convert by `02:2.5` |
| B1, B5 to B8, B3 | A callback's result is tested by truthiness. `bool(x)` is in the core, and its argument is a tested position for the `constant-condition` class. `?.[` in hints and examples |
| R1, R11, R12 | Notes for a Rust implementation (7.3). Three engines to pin before regular expressions can ship. `math` may differ by platform |
| S1, S3, E9 | Namespaces for opt-in groups. Every higher-order operation is in this document: a host cannot add one (S3) and an author cannot write one (E9). A callback argument is a lambda in place or a `const`-bound lambda by name (2.7) |
| X1 to X3, X6 | `if (c) { a } else { b }` beside `? :` and `match` in 2.1 and in the migration rows for expr's `if(c, a, b)`. `const` in every example. No entry is named by a reserved word. `range` and `reduce` stand in for loops, which are not in 3.0 |
| V1 to V5 | No new entry. 2.8: conversion against `as`; `type(x)` against `is`, with `type` kept (D45); `json.parse(s) as T` as the idiom; `try` and `fail` with a failed test (2.3). No entry is named `as` or `is` |
| Y3, Y4 | "null" throughout, and `type(x)` returns `"null"`. Every signature is in the spelling of `03:2.2` (2.7) |
| W1 (D46, `08`) | No entry. `dedent`, `trimIndent` and `stripIndent` are hints (2.2) and are deliberately absent (2.8). No example here holds a prompt |
| W2, W3 (D47, `09`) | No entry. `type` of a variant is `"map"`, and a map method reads a tagged union as its flat record (2.8) |
| W4, W5 (D48, `10`) | `fail(message, data?)`. Every `try(e, d)` is rewritten as `try { e } catch { d }`: 2.1, 2.3, 2.8, 3.2, 3.3. `invalid-argument` has 14 named templates, and each `fails` line names its code and template (2.3, 2.11). Sixteen entries can stand in a trace (2.7). A message and a code never change (2.12) |
| W6 (D49, `11`) | The `cases` of an entry are a case file with no header, run by `risor test`. No tolerance: a case for one of the 11 rounds in its own program (2.8). `assert` stays out |
| W7 (D50, `12`) | No entry. The process environment is data, like the clock (2.6). A deep `merge` is absent |
| W8 (D51, `13`) | `sortBy` with a direction, and `minBy`, `maxBy`, `uniqueBy`, `indexBy`, `countBy`, `mapValues`, with their steps and units (2.8). The advice on descending order and on two keys is corrected, and four rows of "deliberately absent" are reversed. `filter` and `find` narrow. The projection step removes no entry (2.1). 78 entries |

| Still assumed | Of |
| --- | --- |
| `03:2.11` holds the rule by which `filter` and `find` narrow. Whether a test of a tag in the predicate narrows a tagged union is its to say: `13:5.1` says it does, and `09:2.5` lists a predicate under "not narrowed". This document shows only the cases both agree on | `03` |
| The run code of an argument of the wrong kind on `any` data has a name, and it covers a string outside the literal set of `sortBy`'s `order`, a mix of kinds under `ordered`, and an opaque value given to `string` (ruling 8) | `02`, `05` |
| The catalog entry of `number-range` has a template that fits a library entry's result, with the entry's name as a parameter | `05` |
| A trace entry for `mapValues` carries the position of the entry in key order, where a list method's carries the index. `10:2.8` was written when every callback had an index | `04`, `05` |
| A case can enable an opt-in group, by a field `groups`. Without it no case of `math`, `time`, `json` or `base64` runs under `risor test` | `05`, `11` |
| A `Method` receiver may be narrower than `T[]` (2.7). `never` is a result type. `!` means kind `value` only. Its example list writes `string.upper` and marks `round` with `!`; here it is `toUpperCase`, and `round` fails only with `number-range` | `03` |
| The 1 in "1 + size" is the call's step. A library entry may read text at 8 times the rate. Scratch space of one bit per element is free. Either an ordered walk of a map charges the sort formula, or the map methods here do not | `04` |
| `Groups`, `Without`, reserved namespaces and versions work as in `05:A.2` and `05:A.10`. The environment description carries the table's fields. Its migration table writes `math.abs`; `abs` is in `core` here | `05` |
| The codes `invalid-argument` and `failed` are in the catalog (P6), both with `caught: true`. `messages.json` has the 14 templates of 2.3 under `invalid-argument`, and no template under `failed` | `05`, `10` |
| Example 38 of `02:5` writes `.upper()`; it becomes `.toUpperCase()` | `02` |

## 6. Open questions for the owner

| # | Question | Recommendation |
| --- | --- | --- |
| 1 | **JavaScript's names for string and list methods**, so `some`, `every`, `flat`, `toUpperCase`, with expr's names for the functions. | Accept. The alternative is expr's names throughout, which the engine's authors know and which are shorter |
| 2 | **Regular expressions out of 3.0**, with the subset of 2.8 as the candidate for 3.1. | Accept. If they must be in 3.0, specify that subset and have all three implementations pass one set of cases |
| 3 | **Case mapping from a table the specification owns**, one code point to one, pinned to Unicode 16.0. So `ß` has no upper case. | Accept. The alternative is ASCII only |
| 4 | **Strict null in `len`, `int` and `float`.** expr and the engine's documents accept nil there. | Strict. It costs `len(x ?? [])` at each call that may see null, and gives one rule |
| 5 | **Should key order be paid for?** The four map methods charge a sort. Rendering, in `04`, charges 1 step per entry. | Charge the sort formula in both, so that work stays proportional to steps (L2) |
| 6 | **Times as seconds, with a fraction that is the nearest double.** | Accept. Whole milliseconds are the alternative |
| 7 | **`time` and `base64` in 3.0?** Each is small and exact, and neither has a consumer. | In, as opt-in groups. They are the first to cut. Of the entries of D51, `uniqueBy` and `countBy` are the first to cut (`13:5.1`) |
| 8 | **One code, `invalid-argument`, for every `value` failure of the library.** Since D48 a program reads `err.code` and may `match` on it, and a code cannot change within a major version. With one code, a handler cannot tell bad JSON from a negative count except by the message. | Keep one code. A `try` wraps one expression, so the handler knows what failed, and 14 more codes in the type of `err.code` help nobody. If codes are wanted, they must come before 3.0: the 14 template names of 2.3 are the candidates |
| 9 | **The trace entry of `mapValues`.** Its callback has a key and no index (`10:2.8`). | The position in key order, as a number, so that `index` has one type. The alternative is the key, which says more and makes `index` a number or a string |
| 10 | **`flat` beside `xss[*][*]`.** A projection of two stars says exactly what `flat` says, at the same cost less the call's step. | Keep `flat`. It is the familiar name, and D51 kept `map` on the same ground. The alternative is to drop it and give `flat()` a hint |

**Rulings and sibling decisions I would change.**

- **`math.sqrt` as implementation-defined (`02:7`).** It can be exact, so it
  should be (T3 asks for the smallest set).
- **`04:A.3`'s ordered walk at 1 step per entry.** Question 5.

No ruling in the overview seems wrong to me. L6 has a price here that should be
seen: the strict rule of 2.12, and a `sort` with no comparator.

## 7. Implementation notes

Nothing in this section is normative.

### 7.1 The Go implementation

**The registry.** The table is embedded and loaded at start-up. Each entry in
code is one value that names its table record and adds the implementation:

```go
// entry binds one record of library.json to its Go implementation.
type entry struct {
	Name  string // must match a record of the table
	Impl  any    // the Go function
	Steps func(a *args) int64 // the record's steps formula, hand-written
	Units func(a *args) int64 // the record's units formula, hand-written
	Bind  func(c *CallCheck) (any, error) // optional: prepare a constant argument
}
```

Start-up fails if a record has no entry, an entry has no record, or `Steps` or
`Units` is nil. The checker's signatures are generated from the table by `go
generate`, and the existing `generate` job in CI verifies them.

**What to keep from v2.** The principle of `AttrRegistry`, that definition and
documentation live in one place, checked when the program starts
(`v2:pkg/object/attr_registry.go:12-21`, `:78-86`, `:101-110`) (R). What v2
shows about the cost of not checking: its registry documents `decode("json",
'{"a": 1}')` and `encode("json", {a: 1})`
(`v2:pkg/builtins/registry.go:95-105`), the implementation reads the format from
the second argument (`v2:pkg/builtins/codecs.go:68`, `:227`), and both
documented examples fail (X). v2 has three mechanisms: `builtins.Entry`,
`AttrRegistry`, and module maps with documentation in a second file
(`v2:pkg/modules/math/docs.go:15-61`). v3 has one.

**Go habits that must not leak into results (P3).**

| Rule | Do not use | Use |
| --- | --- | --- |
| Case | `strings.ToUpper`, `unicode.ToUpper`: Go's own Unicode version | The specification's table |
| White space | `strings.TrimSpace`: it has U+0085 and lacks U+FEFF (X) | `strings.TrimFunc` with the 25 code points |
| Sort | `sort.Stable`, `slices.SortStableFunc`: their swaps are not bounded by `n × L` | A merge sort with a scratch slice |
| `sortBy(f, "desc")` | Sorting ascending and then reversing: elements with equal keys come out reversed | The same merge sort with the comparison turned round |
| Messages | A message written in Go, or `fmt.Errorf` text passed through | The template its `fails` line names, filled by the generated catalog code (`10:2.9`) |
| `mapValues` | Ranging over a Go map to call the callback | The receiver's keys sorted first. The callback's order, and so its first failure and its trace, are in key order |
| `fixed` | `strconv.FormatFloat(x, 'f', d, 64)` alone: halves go to even, so 2.5 gives `2` (X) | The exact decimal expansion, rounded by hand |
| JSON text | `encoding/json`: it escapes `<`, `>`, `&` and U+2028 (`02:8.3`), and reads a large integer as a rounded double | The renderer shared with templates; a two-pass scanner for `parse` |
| Time | `time.Parse(time.RFC3339, s)` | A parser for exactly the form of 2.8 |
| Length, positions | `len(s)`, byte offsets | Code points; weight for cost (`04:B.8`) |
| `math` | Nothing to avoid | Go does not promise the same bits on every architecture, which is why the 11 are implementation-defined |

**Sharing.** `slice`, `trim` and `removePrefix` may return a sub-slice or a
substring of the receiver. Two rules keep that safe: never `append` to a slice
the entry did not allocate, and clip the capacity of a returned sub-slice
(`xs[i:j:j]`). The units of 2.4 are charged regardless. Host slices of any
element type (`[]string`, `[]User`) reach an entry through the evaluator's list
accessor, not as `[]any`.

**Reused.** From expr: the conversion rules (`expr:builtins.go:193-232`), the
ordering rules of `sort` and `sortBy` (`expr:docs/reference/spec.md:597-611`),
`slice`'s clamping, the cycle guard of its formatter (`expr:format.go:16`), and
its tests as case data. From v2: code point access to strings without copying
(commit `fc7a78a`), and the math tests (`v2:pkg/modules/math/math_test.go`) as
case data. The spike's `filter`, `map` and `reduce`
(`experiments/v3/spike/spike.go:947-985`) are the starting point for the list
methods, with `reduce`'s arguments reordered.

### 7.2 The TypeScript implementation

All (I). Nothing here was built.

| Rule | Native feature that is right | Must be written |
| --- | --- | --- |
| Weight | `s.length` | |
| Code points | | Length, `slice`, `split("")`, padding and index walk surrogate pairs |
| White space | `s.trim()` removes the same 25 code points (X) | |
| Case | | A lookup in the specification's table; `toUpperCase` is right only for ASCII |
| Order | | A code point comparison; `<` and the default sort compare UTF-16 units |
| `sort`, `sortBy` | `Array.prototype.sort` is stable, with the comparison above. For `"desc"` the comparison is negated; `reverse()` is wrong for equal keys | |
| Numbers | `String(x)`; `Number(s)` after a grammar check; `Number.isSafeInteger` | `round`: `Math.round` rounds a negative half upward |
| `fixed` | `x.toFixed(d)` (X) | Remove the sign of a zero result |
| JSON | | The first pass that validates, counts and tests integers on their digits. `JSON.parse` may build the value afterwards |
| Base64 | | UTF-8 and both alphabets; `atob` is neither |
| Time | `Date.UTC` for the calendar | The parser: `Date.parse` accepts lower-case `t` and drops digits past milliseconds (X) |
| Maps | | Sorted keys for `keys`, `values`, `entries` and `mapValues`, whose callback runs in that order |

### 7.3 The Rust implementation

All (I). Nothing here was built. Each "is right" below must be confirmed by the
conformance cases before it is relied on.

| Rule | Native feature that is right | Must be written |
| --- | --- | --- |
| Weight | | A count. A string the run creates stores it (`04` Part D) |
| Code points | `chars()` and `char_indices()` | Slicing and indexing by code point are scans, unless the string is marked ASCII |
| White space | | A test for the specification's 25 code points. `str::trim` uses Unicode's White_Space property, which is a different 25: it has U+0085 and lacks U+FEFF |
| Case | | A lookup in the specification's table. `to_uppercase` follows the Unicode version of the compiler's standard library and maps `ß` to two code points |
| Order | `<` on `str` is code point order. `slice::sort` is stable | |
| `sort`, `sortBy` | The stable sort, with the charge from the formula and not from the comparisons made. For `"desc"`, `sort_by` with the comparison reversed, not `reverse()` | |
| `minBy`, `maxBy` | | A loop that keeps the first of equal keys. `Iterator::max_by_key` returns the last of equal elements, and `min_by_key` the first |
| Numbers | `f64::round` rounds a half away from zero, which is the rule | Number to text: the standard formatter gives the shortest digits and never an exponent, so the layout of `02:2.5` is written over it. Text to number: a grammar check, then `str::parse` |
| `fixed` | | The digits of the exact value, with a tie rounded as `02` says. The standard formatter rounds an exact tie to even, so `fixed(2.5, 0)` would differ |
| JSON | A JSON crate may build the value | The first pass that validates, counts and tests integers on their digits, and output in key order |
| Base64 | | Both alphabets, about 80 lines, or a crate |
| Time | | The calendar arithmetic. The standard library has none |
| Maps | A `BTreeMap` is already in key order, for `mapValues` too | |
| `math` | The `f64` methods | Nothing, but results may differ between platforms (2.8) |

The core group needs no crate outside the standard library. That keeps the
implementation buildable for WebAssembly (overview R8).

### 7.4 Size and order of work

v2's library is about 5,700 lines without tests (R, `wc`), and expr's about
1,900 (R). Estimates for the Go implementation (I):

| Part | Lines |
| --- | ---: |
| Table loader, entry binding, generated signatures and argument checks | 300 |
| Core functions, with the renderer shared by `string`, templates and `json.stringify` | 400 |
| String methods, with the case table and code point helpers | 350 |
| List methods, with the merge sort | 770 |
| Map methods | 110 |
| `json` 250, `time` 150, `math` 60, `base64` 80 | 540 |
| Hint table and listing generator | 150 |
| **Total** | **about 2,600** |

The seven entries of D51 are about 200 of those lines: 170 in the list methods
and 30 in the map methods (`13`, section 1, counts the same 200 and adds 30 in
the checker for `filter` and `find`). `fail` with data adds none. The catalog
loader and the generated message code are plan `10`'s lines, not these.

The table itself is about 80 records: 78 entries and 2 constants. The cases are
data: I expect 450 to 550, plus one per code point of the case table. Each case
for a failure now states its message. The accounting test is about 500 lines per
implementation. The TypeScript and Rust implementations are each about the same
size. The regular expression subset, if wanted, adds about 700 lines to each.

Order:

1. `library.json` with every record, and the cases for the core group. This
   needs no implementation, and it is what the checker (`03:11`) loads. With it,
   the entry of `invalid-argument` in `messages.json`: its 14 templates must be
   worded before the first case for a failure is written, because the case
   states the message.
2. Core functions and the renderer. List methods. String and map methods.
3. The accounting test, and the benchmark that confirms the rates of 2.4 before
   they are frozen.
4. `json`, `time`, `base64`, `math`.
5. The hint table, the reference page and the compact listing.
6. The same order in TypeScript and in Rust, against the same table and cases.
