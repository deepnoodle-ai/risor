# 04. Core library

This chapter is normative. It defines the 53 entries of the Core library: the
result of each for every input, when it fails, and the steps and allocation
units it charges. `suite/library.json` holds the same entries as data (1.3).

The other chapters are cited by file name: `01-language.md` (values, truthiness,
equality, order, rendering, the rules of numbers and text), `02-evaluation.md`
(order of evaluation, steps, units, limits), `03-host.md` (environments,
diagnostics, the catalog `suite/messages.json`, case files) and `05-types.md`
(type text, signatures, the checker).

## 1. Scope

### 1.1 The entries

| Group | Count | Entries |
| --- | ---: | --- |
| Conversion, inspection and errors | 7 functions | `len`, `string`, `int`, `float`, `bool`, `type`, `fail` |
| Numbers | 8 functions | `abs`, `floor`, `ceil`, `round`, `min`, `max`, `range`, `fixed` |
| Strings | 13 methods | `toUpperCase`, `toLowerCase`, `trim`, `removePrefix`, `removeSuffix`, `startsWith`, `endsWith`, `split`, `replaceAll`, `slice`, `repeat`, `padStart`, `padEnd` |
| Lists | 22 methods | `filter`, `map`, `flatMap`, `some`, `every`, `find`, `count`, `reduce`, `sum`, `min`, `max`, `sort`, `sortBy`, `groupBy`, `toMap`, `unique`, `flat`, `reverse`, `first`, `last`, `slice`, `join` |
| Maps | 3 methods | `keys`, `values`, `entries` |

1. Every entry is in the library group `core`. An environment has every entry of
   `core` unless it declares the entry removed (`03-host.md`).
2. Every entry is exact. For the same arguments, every implementation gives the
   same result or the same failure, and charges the same steps and units.
3. An entry is named by its name and, for a method, the kind of its receiver.
   `min`, `max` and `slice` each name two entries: `min` and `max` are a
   function and a list method; `slice` is a string method and a list method.
   Where one entry is named alone, as in the removed entries of an
   environment (`03-host.md` 3.3.1), a function is written by its name, `len`,
   and a method by its receiver's kind, a dot and its name, `string.slice`,
   `list.min`, `map.keys`.

### 1.2 Versions

Every entry has `since` 3.0. Within a major version:

| Part | Rule |
| --- | --- |
| The set of entries | Only grows |
| A signature | May gain an optional last parameter, and only if every existing call keeps its result, its steps and its units. Nothing else changes |
| The result and the failures of an entry, for any arguments | Never change |
| The steps and units of an entry | Never change |
| The rules of section 3, the case table and the white space set | Never change |
| The code of each failure, and the name and text of its template | Never change |
| `doc`, `wrong`, and cases added to `cases` | May change |

A change to a result or a count is a new entry with a new name. The old entry
stays.

### 1.3 The specification table

`suite/library.json` is a JSON array. It has one record for each entry, 53 in
all. The records and this chapter are written together and state the same rules.

| Field | Holds | Present |
| --- | --- | --- |
| `name` | The entry's name, as `filter` or `len` | Always |
| `receiver` | The receiver type in type text, as `T[]`, `string`, `string[]` or `map<T>` | For a method only |
| `signature` | The signature in type text (`05-types.md`), without the receiver | Always |
| `group` | `core` | Always |
| `since` | `3.0` | Always |
| `exact` | `true` | Always |
| `doc` | One sentence. Not normative | Always |
| `calls` | The names of the parameters that are callbacks. The entry calls each during the call and does not keep it. `[]` when there is none | Always |
| `narrows` | The parameter whose true facts narrow the element type of the result (section 6) | On `filter` and `find` only |
| `renders` | The names of the parameters whose argument the entry renders as text, by the walk a template part gets. The checker applies the rule of `05-types.md` 7.3 to such an argument: an opaque type there is the check error `kind`, and a type with `any` at any depth records a `dynamic` site. It changes nothing at run time | On `string` only, as `["x"]` |
| `tested` | The positions, from 0, of the arguments that are tested positions (`05-types.md`): their values are tested by truthiness, and the checker reports a constant condition there | On `bool` only, as `[0]` |
| `steps` | The steps formula, in the text of 4.3 | Always |
| `units` | The units formula, in the text of 4.3 | Always |
| `null` | When the result is null, in words. `null` when it never is | Always |
| `fails` | The failures particular to the entry, each a record (below). The failures of 2.6 that every entry shares are not listed | Always |
| `wrong` | Names people try, for hints. Not normative | Always |
| `cases` | The entry's conformance cases: a case file with no header (`03-host.md`) | Always |

A record of `fails` has these fields.

| Field | Holds |
| --- | --- |
| `when` | The condition, in words |
| `kind` | One of the kinds of failure (`05-types.md`): `null`, `kind`, `field`, `index`, `divide`, `value`, `host`, `range` or `limit`. The library's lines use `value`, `range`, `kind`, `field` and `limit` |
| `code` | The run code (`03-host.md`) |
| `template` | The name of the code's template in `suite/messages.json`, present when the code has several (`03-host.md`): for `invalid-argument`, one of 3.2; for `number-range`, `entry`; for `wrong-kind` and `missing-field`, the one whose condition the line describes. Absent for a code with one template |

1. A loader rejects a record with no `steps` or no `units`. Each
   implementation's registry has exactly the entries of the table, and an entry
   cannot be added in code alone.
2. Every `template` named in `fails` is a template of the line's code in
   `suite/messages.json`.
3. Every case of `cases` runs in the suite's base environment, which has the
   whole Core library (`03-host.md`).

## 2. Shape

### 2.1 Functions and methods

1. A **function** is called by name: `len(x)`. Its name is a name of the
   environment, resolved when the program is compiled.
2. A **method** is called on a receiver: `xs.filter(f)`. Strings, lists and maps
   have the library's methods. Numbers, booleans and null have none. An opaque
   value has the methods its host declares (`03-host.md`) and no method of the
   library.
3. A function or a method of the library is not a value (`01-language.md`). It
   can only be called.

### 2.2 How a method call is resolved

For a method call `r.name(a1, …)`, the evaluator does this, in order, at every
method call of every run (`02-evaluation.md`):

1. It evaluates `r`.
2. If `r` is null, the call fails with `null-operand` (2.4). Nothing more is
   evaluated.
3. It chooses the method by the kind of `r` and by `name`: the string methods
   for a string, the list methods for a list, the map methods for a map, the
   host's methods for an opaque value. Then it makes the **shape tests** below,
   in order. The first that fails raises `wrong-kind` at the method call, with
   the template shown. Nothing more is evaluated and nothing is charged.

| # | Test | Template |
| --- | --- | --- |
| 1 | The kind has a method `name` | `method` |
| 2 | The method accepts the number of arguments written (2.6) | `count` |
| 3 | Each argument at a callback parameter is a lambda: one written in place, or the name of a `const`-bound lambda | `callback` |
| 4 | Each argument at a parameter that is not a callback is not a lambda | `not-callback` |
| 5 | Each lambda declares no more parameters than the entry calls it with | `parameters` |

Tests 3 and 4 are one pass over the arguments, left to right: the first argument
that fails either gives its template. Test 5 is a second pass, left to right. A
method of an opaque type takes no callback, so for an opaque receiver test 3
never fails and test 4 fails for any lambda argument. All five need only the
program's text and the method's record: an implementation may prepare them when
compiling, but may not report them then, because the method is known only when
the receiver's kind is.

4. It evaluates the arguments, left to right.
5. It makes the tests of 2.6, then charges the call's step, then the entry runs
   (section 4).

Where the checker knows the type of `r`, the shape tests are also made when the
program is checked, as type rules (`05-types.md`): a method that the type does
not have is `no-method`; a count it does not accept, and a lambda with too many
parameters, are `arity`; a lambda where the method takes a value is
`not-a-value`; and a value where it takes a function is `arg-mismatch`. Where
the type of `r` is `any`, or where the program was compiled without the type
rules, nothing about the method is tested when compiling.

The run always selects the method by the kind of `r` and always makes the shape
tests, whatever was found when compiling. An implementation may resolve a method
when compiling, as an optimization, only if it guards the choice with a test of
the receiver's kind, so that it is never observable. So `x.slice(1)`, with `x`
declared `string` and supplied as the list `[1, 2, 3]`, runs the list method and
gives `[2, 3]` in every implementation.

### 2.3 A name that is not an entry

| Written | Outcome |
| --- | --- |
| A call of a function name that the environment does not have | `undefined-function`, a static rule (`05-types.md`). It does not depend on types |
| A function name used without a call | `not-a-value`, a static rule (`05-types.md`) |
| A method name used without a call, as `m.keys` | A field read, not a method (`01-language.md`): it reads the key of that name, as 2.5 says |
| A method call whose name is not a method of the receiver's type | `no-method`, a type rule (`05-types.md`) |
| The same, where the receiver's type is `any` | At run time, `wrong-kind` (2.2) |

The field `wrong` of a record holds names that people try, such as `upper` for
`toUpperCase`. A diagnostic may give the right name in its `help`, which is not
specified and which no case compares. A suggestion never names an entry that
the environment removed.

### 2.4 Null receivers

1. No method accepts a null receiver. A method call on a null receiver fails
   with `null-operand`, before its arguments are evaluated (2.2).
2. `r?.name(…)` on a null `r` gives null, and the rest of its chain is not
   evaluated or charged (`02-evaluation.md`).
3. The checker reports a receiver that may be null (`05-types.md`).
4. A parameter accepts null only when its type is `any` or a type variable with
   no bound (2.6). The parameters of type `any` are those of `string`, `bool`
   and `type`, and the `data` of `fail`. The one parameter outside a callback
   whose type is a type variable with no bound is the `init` of `reduce`.

### 2.5 Names that are also map keys

1. In a method call on a map, the name is a method of the library, whatever keys
   the map has. `m.keys()` calls `keys` even when `m` has the key `keys`.
2. Without a call, the same text reads the data: `m.keys` and `m["keys"]` read
   the key `keys`.

| Program | Value |
| --- | --- |
| `{keys: 5}.keys()` | `["keys"]` |
| `{keys: 5}.keys` | `5` |
| `{entries: 1}["entries"]` | `1` |

### 2.6 Arguments and the tests made at run time

1. **Count.** A call must have as many arguments as its signature allows: each
   required parameter, each optional parameter at most once, and for `...rest`
   zero or more. For a call of a function by name, another count is `arity`, a
   static rule. For a method call it is `arity` where the receiver's type is
   known, a type rule; where it is not, it is `wrong-kind` at run time by shape
   test 2 (2.2). `05-types.md` has the static and type rules.
2. **Callbacks.** The argument for a callback parameter is a lambda written in
   place, or the name of a `const`-bound lambda (`01-language.md`), and no other
   parameter takes a lambda. The entry calls it during the call only, with every
   parameter its signature shows. A lambda may declare fewer parameters
   (`02-evaluation.md`), and no more. In a call of a function by name these are
   static rules; at a method call they are type rules where the receiver's type
   is known, and otherwise shape tests 3 to 5 (2.2).
3. **Tests.** At run time the receiver and the arguments are tested against the
   signature. These tests are made on every call. A checked program whose values
   keep their declared types never fails one. The receiver is tested first, then
   each argument left to right. For one argument the test of its kind comes
   before the test for a whole number. The first failure is raised. The
   templates are `argument` for an argument of a kind its parameter does not
   take, null included, for a function and for a method alike; `whole` for a
   number that is not whole at an `int` parameter; and `unordered` for any
   failure at a parameter bound `ordered` (the arguments of the functions `min`
   and `max`), null and lists included. So `fixed("a", 2.5)` fails with
   `argument`, and `max(1, null)` with `unordered`. A parameter whose type is
   `any`, or a type variable with no bound (`A`, `T`, `U`), accepts every value,
   null included, and is not tested. So `[].reduce((acc, x) => 0, null)` gives
   null: the `init` of `reduce` has the type `A`.

| Found | Code |
| --- | --- |
| A null receiver | `null-operand` |
| A method call that fails a shape test (2.2) | `wrong-kind` |
| An argument of a kind its parameter does not allow | `wrong-kind` |
| A null argument, for a parameter whose type names a kind (`int`, `float`, `string`, a list, a map) | `wrong-kind` |
| A number that is not a whole number (3.10), for a parameter of type `int` | `wrong-kind` |
| Arguments of `min` or `max`, the functions, that are not ordered (3.6) | `wrong-kind` |

4. These are the top-level tests. They read the kind of each argument and no
   more, and charge nothing. Some entries make further tests of the elements of
   a list, of the results of a callback, or of keys. Each entry's record says
   which, and section 4 says when.
5. Every failure of a test is raised before the call's step is charged.
6. **Span.** Every failure that a library entry raises has the span of the whole
   call: the `Call` node for a function, and the `MethodCall` node for a method,
   from the first token of its receiver to the closing parenthesis. This holds
   whichever argument, element, key or callback result failed, for the tests of
   this section, the shape tests of 2.2, the element tests and value tests of
   section 4, the failures of each entry's work, and `fail`. A failure raised
   inside a callback's body is not the entry's: it keeps its own span (3.3).

| Program | Data | Code | Span |
| --- | --- | --- | --- |
| `int("0x10")` | | `invalid-argument` | `[1, 1, 1, 12]` |
| `xs.filter(x => x).sum()` | `{xs: any}`, xs = `[1, "2"]` | `wrong-kind` | `[1, 1, 1, 24]` |
| `[1, 0].map(x => 10 / x)` | | `divide-by-zero` | `[1, 17, 1, 23]`: `10 / x`, in the callback |

### 2.7 Signatures

Signatures are type text (`05-types.md`). In this chapter:

1. `int` is a whole number and `float` is any number. An `int` fits `float`.
2. `T[]` is a list of `T`, and `map<T>` is a map whose values are `T`.
3. `T | null` is `T` or null.
4. `where T: ordered` requires the values of `T` to be ordered (3.6). `where T:
   number` requires numbers.
5. `(item: T, index: int) => U` is a callback. `(item: T, index: int) =>
   unknown` is a predicate: its result is tested by truthiness
   (`01-language.md`), and "`f` is true" means that the result is truthy.
6. `!` after the result type marks an entry that can fail with kind `value`.
7. `never` is the type of an expression that does not give a value.
8. A method's full text starts with its receiver: `T[].map`, `string[].join`.

## 3. Shared rules

### 3.1 Failure and null

| Situation | Result |
| --- | --- |
| The data asked for is not there: no element matches, or the list is empty | Null. The result type is `T \| null` |
| An argument has a value the entry cannot use | The failure `invalid-argument`, kind `value`, with a template (3.2). The signature has `!` |
| A result that must be a whole number is not one, or a number result is not finite | The failure `number-range`, kind `range` |
| The program calls `fail` | The failure `failed`, kind `value` |
| A test of 2.6, or a test that an entry's record names | `null-operand`, `wrong-kind` or `missing-field` |
| A walk passes the depth limit | `depth-exceeded` (`02-evaluation.md`) |
| A charge passes a limit | The limit code (`02-evaluation.md`) |
| A callback fails | The call fails with that failure (3.3) |

1. No entry gives NaN, an infinity or negative zero. A result that would be
   negative zero is zero.
2. `catch` catches every failure of an entry, except a limit and
   `depth-exceeded` (`03-host.md`).
3. A `number-range` failure of an entry has the template `entry` of
   `number-range` in `suite/messages.json`, with the entry's name
   (`03-host.md`). Its `fails` line names that template.

### 3.2 The forms of `invalid-argument`

Every failure of kind `value` of an entry, but `fail`'s, has the code
`invalid-argument`. Its message is one of the named templates of the entry
`invalid-argument` in `suite/messages.json`. The entry's `fails` line names the
template, so the message of a failed call is fixed by the entry and its
arguments. The catalog has 14 templates. Six are used by Core entries:

| Template | Entries | The call fails when | Parameters |
| --- | --- | --- | --- |
| `not-a-number` | `int`, `float` | The text, with white space removed from both ends, is not accepted by the grammar of text to number (3.10) | `text`, kind `string`: the argument as given |
| `not-finite` | `int`, `float` | The text, with white space removed from both ends, is accepted by the grammar and reads as an infinity | `text`, kind `string`: the argument as given |
| `bad-digits` | `fixed` | `digits` is not in 0 to 20 | `digits`, kind `number` |
| `too-large` | `fixed` | The magnitude of `x` is 1e21 or more | `x`, kind `number` |
| `empty-search` | `replaceAll` | `old` is the empty string | None |
| `negative-count` | `repeat` | `count` is below zero | `count`, kind `number` |

The other eight belong to the opt-in groups, which are not in Core: `not-real`
(`math`), `not-a-time`, `year-range` and `not-a-duration` (`time`), `not-json`
and `integer-range` (`json`), `not-base64` and `not-utf8` (`base64`).

1. A program that catches the failure sees the code `invalid-argument` for every
   template.
2. The steps and units of an entry do not include a message. A message is built
   only when a `catch (err)` binds the error, and it is charged there
   (`02-evaluation.md`).

### 3.3 Callbacks

1. An entry calls its callback once for each element, in the order of the
   elements, with the index from 0. `some`, `every` and `find` stop early (5.4).
2. A callback's failure is the failure of the call: the same code, span, message
   and data. The span is in the callback, where the failure was raised. The
   entry does nothing more: it visits no later element, builds nothing and
   charges nothing more.
3. Two entries test each result of their callback when that call returns, before
   the next element: `flatMap` tests that it is a list, and `groupBy` that it is
   a string. A result that fails the test, null included, fails with
   `wrong-kind`. It charges nothing more: the result was paid for with the 2
   steps of its element (4.5).
4. A bound on a type variable of a callback's result, as `K: ordered` of
   `sortBy`, is tested over all the results after the last call (3.6). A failure
   charges nothing more (4.5).
5. Core defines no trace for a diagnostic (`03-host.md`).

### 3.4 No mutation

No entry changes its receiver or an argument. A result may share storage with
them. That cannot be observed, because no value can be changed.

### 3.5 Text

1. A string is a sequence of code points. Every position, length and count of
   characters in this chapter is in code points. No entry normalizes text.
2. `s` **starts with** `p` when the first `len(p)` code points of `s` are the
   code points of `p`, in order. `s` **ends with** `p` when its last `len(p)`
   code points are. Every string starts and ends with the empty string.
3. An **occurrence** of a non-empty string `t` in `s` is a position `i` such
   that the code points of `s` from `i` start with `t`.
4. Occurrences are found **left to right without overlap** this way: start at
   position 0. At each position, if an occurrence starts there, take it and go
   on at the position after its end. Otherwise go on at the next position. Stop
   at the end of `s`.

| `s` | `t` | Occurrences |
| --- | --- | --- |
| `"aaa"` | `"aa"` | One, at 0 |
| `"aaaa"` | `"aa"` | Two, at 0 and 2 |
| `"abc"` | `"x"` | None |

5. **The work of a search.** A search for a string `x` in a string `s` does work
   of at most a constant times `1 + w(s)`. A search for text longer than the
   text searched is answered from the two lengths, before anything else is done
   with `x`. This binds `split`, `replaceAll`, `startsWith`, `endsWith`,
   `removePrefix`, `removeSuffix`, and the language's `x in s`
   (`02-evaluation.md`). The charges of the entries do not change.
6. An implementation may use its platform's search function only where that
   bound holds for it. Where it does not, it may still use it when `w(x)` is
   under 64, and otherwise uses a search of its own that meets the bound.

### 3.6 Equality and order

1. Equality is the language's `==` (`01-language.md`). Numbers compare by value,
   so `1` and `1.0` are equal.
2. Numbers are ordered by value. Strings are ordered code point by code point,
   and a proper prefix is less (`01-language.md`).
3. A set of values is **ordered** when every value is a number, or every value
   is a string. A null, a bool, a list, a map or an opaque value is neither. No
   order across kinds exists.
4. Where an entry requires an ordered set, every value of the set is tested
   before any two are compared, in order. The test fails at the first value that
   is neither a number nor a string, or that is not of the kind of the first
   value, whichever comes first; the scan stops there. The call then fails with
   `wrong-kind`, template `unordered`, whose parameter `kind` is the kind of
   that value. So a list of one null fails at its first element, with `null`;
   `[1, "a", 2]` fails at its second, with `string`; and `[1, "a", null]` also
   fails at its second, with `string`. An empty set is ordered. Every failure of
   kind at a parameter or an element bound `ordered` has the template
   `unordered`, null and lists included.

| Entry | The set |
| --- | --- |
| `min`, `max` (functions) | The arguments |
| `min`, `max`, `sort`, `unique` (list methods) | The elements of the receiver |
| `sortBy` | The keys its callback gave |

### 3.7 Stable sort

1. A **stable sort** of a list by keys gives the elements in ascending order of
   their keys (3.6). Elements whose keys are equal keep their order in the
   receiver.
2. That defines exactly one result. An implementation may use any algorithm that
   gives it, and must use one that makes at most a constant times `n × L`
   comparisons (4.2). A platform's stable sort may be used where that holds for
   it.
3. The steps of a sort come from its formula and never from the comparisons made
   (4.4).

### 3.8 The case table

1. The case table maps each code point `c` to an upper-case code point
   `upper(c)` and a lower-case code point `lower(c)`.
2. `upper(c)` is the Simple_Uppercase_Mapping of `c` in the Unicode Character
   Database, version 16.0.0 (field 12 of `UnicodeData.txt`), when that field is
   not empty. Otherwise it is `c`.
3. `lower(c)` is the Simple_Lowercase_Mapping (field 13), when that field is not
   empty. Otherwise it is `c`.
4. The titlecase mapping, `SpecialCasing.txt` and every rule that depends on
   context or language are not used. A mapping is always one code point to one
   code point.
5. In version 16.0.0 no mapping changes whether a code point is above U+FFFF. So
   a mapped string has the weight of the receiver.
6. **`suite/casemap.json` is this table,** generated from `UnicodeData.txt` of
   Unicode 16.0.0. Every implementation builds its case tables from this file,
   never from its own platform's tables or Unicode version.

The file is one JSON object.

| Field | Holds |
| --- | --- |
| `unicode` | `"16.0.0"` |
| `source` | The file and the two fields the table comes from |
| `sha256` | The SHA-256 of the `UnicodeData.txt` it was generated from |
| `upper` | An object: for each code point `c` with `upper(c)` not `c`, the key `c` and the value `upper(c)` |
| `lower` | The same for `lower(c)` |

A code point is written as upper-case hex digits, at least four, with no prefix:
`00E9`, `1E921`. A code point that is not a key of `upper` maps to itself, and
the same for `lower`. The 16.0.0 table has 1,477 keys in `upper` and 1,460 in
`lower`.

| `c` | `upper(c)` | `lower(c)` |
| --- | --- | --- |
| `é` U+00E9 | `É` U+00C9 | `é` |
| `ß` U+00DF | `ß` | `ß` |
| `İ` U+0130 | `İ` | `i` U+0069 |
| `Σ` U+03A3 | `Σ` | `σ` U+03C3 |
| `ς` U+03C2 | `Σ` U+03A3 | `ς` |
| `ǅ` U+01C5 | `Ǆ` U+01C4 | `ǆ` U+01C6 |
| `ﬁ` U+FB01 | `ﬁ` | `ﬁ` |

### 3.9 White space

White space is exactly these 25 code points. No other code point is white space.

| Code points | Count |
| --- | ---: |
| U+0009 to U+000D | 5 |
| U+0020 | 1 |
| U+00A0 | 1 |
| U+1680 | 1 |
| U+2000 to U+200A | 11 |
| U+2028, U+2029 | 2 |
| U+202F | 1 |
| U+205F | 1 |
| U+3000 | 1 |
| U+FEFF | 1 |

U+0085 and U+200B are not white space.

### 3.10 Numbers and positions

1. **Text to number** is the rule of `01-language.md`: the JSON number grammar,
   read as the nearest double, ties to even. `-0` reads as zero. Text that the
   grammar accepts and that reads as an infinity gives no number: `int` and
   `float` each say what they do with it. Both first remove white space (3.9)
   from both ends of the text.
2. **Number to text** is the rendering of a number in `01-language.md`.
3. A **whole number** is a number with no fraction whose magnitude is at most
   2^53 − 1 (9007199254740991). An entry whose result type is `int` fails with
   `number-range` when its result is not a whole number.
4. **Positions.** `slice` resolves a position `p` against a length `len` this
   way: if `p` is below zero, it becomes `p + len`. Then a value below zero
   becomes 0, and a value above `len` becomes `len`. `end`, when it is left out,
   is `len`. The result covers the positions from the resolved `start` up to but
   not including the resolved `end`, and is empty when `start` is not below
   `end`.

| `len` | `start`, `end` | Resolved | Positions |
| ---: | --- | --- | --- |
| 5 | 1, 3 | 1, 3 | 1, 2 |
| 5 | -3, none | 2, 5 | 2, 3, 4 |
| 5 | 2, -1 | 2, 4 | 2, 3 |
| 5 | -10, 2 | 0, 2 | 0, 1 |
| 5 | 10, none | 5, 5 | None |
| 5 | 3, 1 | 3, 1 | None |

### 3.11 Key order

1. Wherever an entry visits the entries of a map, it visits them in key order:
   ascending by the order of strings (3.6).
2. A map that an entry returns has no order of its own. Wherever its order can
   be seen, it is key order (`01-language.md`).

### 3.12 Determinism

1. No entry reads anything but its receiver and its arguments. No entry reads a
   clock, a random source, a locale, a time zone, the process environment or a
   file.
2. For the same receiver and arguments, an entry gives the same result or the
   same failure, and charges the same steps and units, in every implementation
   and on every platform. The order in which a map was built changes nothing.

## 4. Cost

### 4.1 What a formula counts

1. Steps and allocation units are those of `02-evaluation.md`.
2. The evaluator charges 1 step for the call of an entry (`02-evaluation.md`).
   An entry's `steps` formula is what the entry charges in addition to that
   step. Its `units` formula is what the entry allocates.
3. A formula does not count:
   - the evaluation of the receiver and of the arguments, a lambda argument
     included, which `02-evaluation.md` charges;
   - what a callback's body charges as it runs;
   - the message of a failure (3.2).
4. Units are charged for the result as written, whether or not an implementation
   shares storage with the receiver.
5. **The key rule.** An entry that builds or probes a map by a key charges the
   size of the key, in steps, because hashing or comparing a key reads its text
   (`02-evaluation.md`). In Core that is `groupBy` and `toMap`, whose formulas
   have the term `sz(keys)`. The map methods `keys`, `values` and `entries`
   already charge `sz(keys)` in their sort term, and `unique` charges
   `sz(elements)`. The keys `key` and `value` that `toMap` reads and `entries`
   writes are text of the specification, of size 0. No other Core entry builds
   or probes a map by a key. Units do not change: a key is not copied.

### 4.2 Symbols

| Symbol | Meaning |
| --- | --- |
| `s` | The receiver of a string method |
| `x`, `prefix`, `suffix`, … | The argument of that name |
| `n` | The number of elements of the receiver list, or of entries of the receiver map |
| `k` | The number of elements of the resulting list. For `flatMap` and `flat`, the total number of elements of the inner lists |
| `v` | The number of elements visited by `some`, `every` or `find` |
| `p` | The number of parts `split` gives |
| `g` | The number of distinct keys `groupBy`'s callback gave |
| `L` | 0 when `n` is less than 2. Otherwise the least whole number with `2 ** L >= n` |
| `w(a)` | The weight of string `a` (`02-evaluation.md`): its code points, with each code point above U+FFFF counted twice |
| `sz(a)` | The size of string `a`: `w(a)` divided by 64, rounded down. 0 when `a` is not a string |
| `sz(result)` | The size of the string the entry returns |
| `sz(elements)` | The weights of the elements of the receiver that are strings, added, divided by 64, rounded down. 0 when no element is a string |
| `sz(keys)` | For `sortBy` and `groupBy`: the same, over the keys the callback gave, one for each element. For `toMap`: over the `key` of every element. For a map method: over the keys of the receiver |
| `sz(arguments)` | The same, over the arguments that are strings |
| `len(arguments)` | The number of arguments of the call |
| `walk(x)` | What the rendering walk of `x` charges (`02-evaluation.md`), as `string` makes it (5.1.2) |

| `n` | 0 | 1 | 2 | 3 | 4 | 5 | 8 | 9 | 64 | 65 | 1000 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| `L` | 0 | 0 | 1 | 2 | 2 | 3 | 3 | 4 | 6 | 7 | 10 |

| String | `w` | `sz` |
| --- | ---: | ---: |
| `"a😀"` | 3 | 0 |
| 63 times `a` | 63 | 0 |
| 64 times `a` | 64 | 1 |
| 32 times `😀` | 64 | 1 |
| 130 times `a` | 130 | 2 |

`sz` of several strings adds their weights before it divides: two strings of
weight 40 have size 1 together, though each has size 0.

### 4.3 Formula text

A formula in `library.json` is text in this grammar.

```ebnf
Formula = Cond | Sum .
Cond    = Name "is" "string" "?" Sum ":" Sum .
Sum     = Term { ( "+" | "-" ) Term } .
Term    = Factor { "*" Factor } .
Factor  = Number | Symbol | Call | "(" Sum ")" .
Call    = ( "sz" | "w" | "len" | "walk" ) "(" Name ")" .
Number  = digit { digit } .
Symbol  = "n" | "k" | "v" | "p" | "g" | "L" .
Name    = the name of a parameter, "s", "result", "elements", "keys" or
          "arguments" .
```

1. `*` is multiplication. Every value is a whole number, and no formula is below
   zero.
2. `x is string ? A : B` is `A` when the argument `x` is a string, else `B`.
   Only `string` uses it.
3. The formulas in this chapter are the same text.

### 4.4 When an entry charges

Every call goes through these steps first (`02-evaluation.md`):

1. The receiver is evaluated and tested, then the arguments (2.2).
2. The tests of 2.6.
3. The call's 1 step.

Then the entry charges and works by one of the three rules below: `string` by
its own rule, an entry that takes a callback by the second, and every other
entry by the first. Each entry's record repeats its order under "Order".

**An entry with no callback.**

1. **Element tests.** The tests of the elements that the entry's record names:
   the ordered test of 3.6 for the list methods `min` and `max`, `sort` and
   `unique`; every element a number for `sum`, a list for `flat`, a string for
   `join`; the tests of `toMap`. They read the kind of each element, and for
   `toMap` the presence and kind of two keys. The elements are tested by index.
   A test that passes charges nothing. A test that fails charges by 4.5 and then
   raises its failure.
2. **Value tests that read no text.** `replaceAll` tests that `old` is not
   empty, `repeat` that `count` is not below zero, and `fixed` its `digits` and
   then the magnitude of `x`. A failure is raised here, before anything is
   charged.
3. **Sizes.** Each term of the formulas that depends on the data, `k`, `p`,
   `sz(result)` or `sz(elements)`, is found by a pass that creates no value,
   charges nothing and cannot fail.
4. **Steps**, the whole formula.
5. **Units**, the whole formula.
6. **The work**, which builds the result. Failures raised here come after both
   charges: `int` and `float` reading their text (`not-a-number`, `not-finite`),
   and `number-range` of `int`, `floor`, `ceil`, `round` and `sum`.

**An entry with a callback.** `filter`, `map`, `flatMap`, `some`, `every`,
`find`, `count`, `reduce`, `sortBy` and `groupBy`.

1. For each element in order: 2 steps, 1 for the visit and 1 for the call. Then
   the depth test (`02-evaluation.md`). Then the callback is called, and its
   body charges as it runs. Then its result is tested, where 3.3 says so.
2. `some`, `every` and `find` stop after the call whose result decides. The
   elements after it are not visited and not charged.
3. After the last call: the ordered test of the keys, for `sortBy`. Then the
   rest of the steps formula: `L*(n + sz(keys))` for `sortBy`, `k` for
   `flatMap`, `sz(keys)` for `groupBy`. Then the units. Then the result is
   built.

**`string`.** The rendering walk of `02-evaluation.md` charges as it goes: on
entering each list or map it tests the depth limit, then charges `m` for a list
of `m` elements, and `m + L(m) × (m + sz(its keys))` for a map of `m` entries,
where `L(m)` is `L` for `n` equal to `m`. After the walk, the steps
`sz(result)`, then the units, then the string is built. A failure met by the
walk keeps what the walk charged before it.

### 4.5 A failed test is paid for

1. This section covers the element tests of an entry with no callback (4.4 rule
   1): the ordered test of the list methods `min` and `max`, `sort` and
   `unique`, and the element tests of `sum`, `flat`, `join` and `toMap`. They
   are a scan made before the entry's charges. A test that passes charges
   nothing.
2. When one fails at an element, the entry charges 1 step for each element it
   tested, the failing one included, and then raises the failure.
3. If that charge passes the step budget, the failure is `steps-exceeded`
   (`02-evaluation.md`).
4. A failed test of a callback's result (`flatMap`, `groupBy`) or of `sortBy`'s
   keys charges nothing more. Each of those values was paid for with the 2 steps
   of its element.
5. The tests of 2.6 are not element tests: they charge nothing, pass or fail.
   That includes the ordered test of the arguments of the functions `min` and
   `max`: `min(1, "1")` fails at 0 steps.

| Program | Data | Steps of the failed run |
| --- | --- | --- |
| `xs.sort()` | `{xs: any}`, xs = `[1, "a", 2]` | 3: the call, and the two elements tested |
| `xs.min()` | `{xs: any}`, xs = `[null]` | 2: the call, and one element |
| `xs.sortBy(x => x)` | `{xs: any}`, xs = `[1, null]` | 6: the call, the lambda, and 4 for two elements. The key test adds nothing |
| `xs.flatMap(x => x)` | `{xs: any}`, xs = `[1, 2]` | 4: the call, the lambda, and 2 for the first element. The result test adds nothing |

### 4.6 What stays charged

A failure keeps every step and unit charged before it (`02-evaluation.md`). A
pass of rule 3 of 4.4 and the scratch space an entry uses are not values and are
not charged. Scratch space is at most a constant times the steps and units
charged.

## 5. The entries

Each entry has a record and its rules. The record's fields are those of 1.3.
"Order" is the sequence of 4.4 for the entry, after the receiver and the
arguments are evaluated. Its first word, "Tests", stands for the tests of 2.6,
then the call's step, then the entry's element tests where it has them.

In the examples, "Data" gives the case's environment as type text and the values
of its variables. A result with counts states the steps and units of the whole
run. A failure names its code, and for `invalid-argument` its form. Every
example is a case of `suite/library.json`.

### 5.1 Conversion, inspection and errors: 7 functions

#### 5.1.1 `len`

| Field | Value |
| --- | --- |
| Signature | `len: (x: string \| any[] \| map<any>) => int` |
| Receiver | None: a function |
| Steps | `sz(x)` |
| Units | `0` |
| Null | Never |
| Fails | Only as 2.6 says |
| Order | Tests; steps; result. |

1. For a string, the number of its code points. A code point above U+FFFF counts
   1, though its weight is 2 (4.2).
2. For a list, the number of its elements. For a map, the number of its entries.
   A record's entries are its fields that are present (`01-language.md`).
3. `x` null, or of another kind, fails with `wrong-kind` (2.6).

| Program | Data | Result |
| --- | --- | --- |
| `len("héllo")` |  | `5` |
| `len("a😀")` |  | `2` |
| `len([])` |  | `0` |
| `len({a: 1, b: null})` |  | `2` |
| `len(s)` | `{s: string}`, s = `"😀" × 32` | `32`; 2 steps, 0 units |
| `len(x)` | `{x: any}`, x = `5` | Fails: `wrong-kind`; 0 steps, 0 units |
| `len(x)` | `{x: any}`, x = `null` | Fails: `wrong-kind` |

#### 5.1.2 `string`

| Field | Value |
| --- | --- |
| Signature | `string: (x: any) => string` |
| Receiver | None: a function |
| Steps | `x is string ? 0 : walk(x) + sz(result)` |
| Units | `x is string ? 0 : 1 + sz(result)` |
| Null | Never |
| Fails | `wrong-kind`, template `text`: x is an opaque value or holds one at any depth; `depth-exceeded`: the walk passes the depth limit |
| Order | Tests; for a string, the result. Otherwise the rendering walk, which charges as it goes; steps `sz(result)`; units; build. |
| Renders | `x`: rendered as text (1.3) |

1. A string is returned as it is, with no charge.
2. The record's field `renders` is `["x"]`: the checker treats `x` as it treats
   a template part (`05-types.md` 7.3). This changes nothing at run time.
3. Any other value gives its rendering (`01-language.md`): null gives `""`; a
   number its number to text; a bool `true` or `false`; a list and a map their
   JSON form, with no white space and a map's entries in key order (3.11).
4. Before it charges for the text, the entry makes the rendering walk of
   `02-evaluation.md` over `x`. The walk tests the depth limit and charges on
   entering each list and map, as that chapter says; `walk(x)` is the total it
   charges. A number, a bool or null is not walked.
5. An opaque value, at any depth, fails with `wrong-kind` where the walk meets
   it. What the walk charged before stays charged. A list or map nested past the
   depth limit fails with `depth-exceeded`.

| Program | Data | Result |
| --- | --- | --- |
| `string(null)` |  | `""` |
| `string(1.5)` |  | `"1.5"` |
| `string(1e21)` |  | `"1e+21"` |
| `string(0.1 + 0.2)` |  | `"0.30000000000000004"` |
| `string(-0.0)` |  | `"0"` |
| `string([1, "a", null, true])` |  | `"[1,\"a\",null,true]"` |
| `string({b: 1, a: [true]})` |  | `"{\"a\":[true],\"b\":1}"` |
| `string(["a\"b"])` |  | `"[\"a\\\"b\"]"` |
| `string("x")` |  | `"x"` |
| `string(x)` | `{x: any}`, x = `{"$opaque": {type: "Token", id: 1}}` | Fails: `wrong-kind`, template `text`, message `"an opaque value cannot be rendered as text"`; 1 step, 0 units |
| `string(x)` | `{x: any}`, x = `"abc"` | `"abc"`; 1 step, 0 units |
| `string(x)` | `{x: any}`, x = `42` | `"42"`; 1 step, 1 unit |
| `string(x)` | `{x: any}`, x = `[]` | `"[]"`; 1 step, 1 unit |
| `string(x)` | `{x: any}`, x = `{b: [1, 2], a: "x"}` | `"{\"a\":\"x\",\"b\":[1,2]}"`; 7 steps, 1 unit |

#### 5.1.3 `int`

| Field | Value |
| --- | --- |
| Signature | `int: (x: float \| string) => int !` |
| Receiver | None: a function |
| Steps | `sz(x)` |
| Units | `0` |
| Null | Never |
| Fails | `invalid-argument`, form `not-a-number`: x is a string that, with white space removed from both ends, is not accepted by the grammar of text to number; `invalid-argument`, form `not-finite`: x is a string that reads as an infinity; `number-range`, template `entry`: the truncated value is not a whole number |
| Order | Tests; steps; read the text; truncate; result. |

1. For a number: `x` truncated toward zero. If the result is not a whole number
   (3.10), the call fails with `number-range`.
2. For a string: white space (3.9) is removed from both ends. The rest is read
   by the rule of text to number (3.10). Text that the rule's grammar does not
   accept fails with `invalid-argument`, form `not-a-number`, whose parameter
   `text` is `x` as given. Text that the grammar accepts but that reads as an
   infinity fails with form `not-finite`, as in `float`. Otherwise the number
   read is truncated as above.
3. No base other than 10, no sign `+`, no leading zero, no separator and no hex
   is accepted: they are outside the grammar.

| Program | Data | Result |
| --- | --- | --- |
| `int(3.99)` |  | `3` |
| `int(-3.99)` |  | `-3` |
| `int(" 42\n")` |  | `42` |
| `int("\u{a0}7")` |  | `7` |
| `int("1.5")` |  | `1` |
| `int("1e3")` |  | `1000` |
| `int("-0.5")` |  | `0` |
| `int(s)` | `{s: string}`, s = `" 42 "` | `42`; 1 step, 0 units |
| `int("0x10")` |  | Fails: `invalid-argument`, form `not-a-number` |
| `int("")` |  | Fails: `invalid-argument`, form `not-a-number` |
| `int("+1")` |  | Fails: `invalid-argument`, form `not-a-number` |
| `int("1e20")` |  | Fails: `number-range` |
| `int("1e400")` |  | Fails: `invalid-argument`, form `not-finite` |
| `int(1e16)` |  | Fails: `number-range` |

#### 5.1.4 `float`

| Field | Value |
| --- | --- |
| Signature | `float: (x: float \| string) => float !` |
| Receiver | None: a function |
| Steps | `sz(x)` |
| Units | `0` |
| Null | Never |
| Fails | `invalid-argument`, form `not-a-number`: x is a string that, with white space removed from both ends, is not accepted by the grammar of text to number; `invalid-argument`, form `not-finite`: x is a string that reads as an infinity |
| Order | Tests; steps; read the text; result. |

1. For a number: `x`.
2. For a string: white space (3.9) is removed from both ends, and the rest is
   read by the rule of text to number (3.10). Text that the rule's grammar does
   not accept fails with form `not-a-number`. Text that the grammar accepts but
   that reads as an infinity fails with form `not-finite`. In both the parameter
   `text` is `x` as given.
3. Text that reads as a value too small to represent gives zero. `-0` gives
   zero.

| Program | Data | Result |
| --- | --- | --- |
| `float(" 2.50 ")` |  | `2.5` |
| `float("-0")` |  | `0` |
| `float(7)` |  | `7` |
| `float("1e-400")` |  | `0` |
| `float("1e400")` |  | Fails: `invalid-argument`, form `not-finite` |
| `float("NaN")` |  | Fails: `invalid-argument`, form `not-a-number` |
| `float(".5")` |  | Fails: `invalid-argument`, form `not-a-number` |
| `float("   ")` |  | Fails: `invalid-argument`, form `not-a-number` |

#### 5.1.5 `bool`

| Field | Value |
| --- | --- |
| Signature | `bool: (x: any) => bool` |
| Receiver | None: a function |
| Steps | `0` |
| Units | `0` |
| Null | Never |
| Fails | Only as 2.6 says |
| Order | Result. |
| Tested | Argument positions [0]: tested positions (1.3) |

1. `false` when `x` is falsy: null, `false`, zero, the empty string, the empty
   list or the empty map. `true` for every other value, an opaque value included
   (`01-language.md`).
2. It never fails. It reads no element and measures no string.
3. Its argument is a tested position: an argument whose type can never be falsy,
   or never truthy, is reported under the check class `constant-condition`
   (`05-types.md`). The record's field `tested` is `[0]`.

| Program | Data | Result |
| --- | --- | --- |
| `bool(x)` | `{x: any}`, x = `[]` | `false` |
| `bool(x)` | `{x: any}`, x = `[[]]` | `true` |
| `bool(x)` | `{x: any}`, x = `"0"` | `true` |
| `bool(x)` | `{x: any}`, x = `0` | `false` |
| `bool(x)` | `{x: any}`, x = `{a: null}` | `true` |
| `bool(x)` | `{x: any}`, x = `null` | `false` |

#### 5.1.6 `type`

| Field | Value |
| --- | --- |
| Signature | `type: (x: any) => string` |
| Receiver | None: a function |
| Steps | `0` |
| Units | `0` |
| Null | Never |
| Fails | Only as 2.6 says |
| Order | Result. |

1. One of seven strings: `"null"`, `"bool"`, `"number"`, `"string"`, `"list"`,
   `"map"` or `"opaque"`.
2. A record, and the error value a `catch (err)` binds, are maps. A whole number
   and a fraction are both `"number"`.
3. It never fails. The seven strings are text of the specification and cost no
   unit.

| Program | Data | Result |
| --- | --- | --- |
| `type(x)` | `{x: any}`, x = `null` | `"null"` |
| `type(true)` |  | `"bool"` |
| `type(1.5)` |  | `"number"` |
| `type("")` |  | `"string"` |
| `type([])` |  | `"list"` |
| `type({})` |  | `"map"` |
| `try { fail("x") } catch (err) { type(err) }` |  | `"map"` |

#### 5.1.7 `fail`

| Field | Value |
| --- | --- |
| Signature | `fail: (message: string, data?: any) => never !` |
| Receiver | None: a function |
| Steps | `0` |
| Units | `0` |
| Null | Never |
| Fails | `failed`: always |
| Order | Tests; the failure. |

1. The call always fails, with the code `failed`, at the span of the call. The
   message is `message`, whole: it is not cut and not taken from the catalog.
   The diagnostic's `data` is `data`, or null when it is left out.
2. A `catch (err)` that catches it binds `err.message` to `message` and
   `err.data` to `data`, the same values and not copies (`02-evaluation.md`).
3. `message` must be a string. On `any` data another kind, null included, fails
   with `wrong-kind` (2.6), and then no `failed` failure is raised.
4. Nothing is copied, so nothing is charged beyond the call's step.

| Program | Data | Result |
| --- | --- | --- |
| `try { fail("no stock") } catch (err) { err.message }` |  | `"no stock"` |
| `try { fail("over", {limit: 3}) } catch (err) { err.data }` |  | `{limit: 3}` |
| `try { fail("x") } catch (err) { [err.code, err.data] }` |  | `["failed", null]` |
| `ok ? 1 : fail("never")` | `{ok: bool}`, ok = `true` | `1` |
| `fail("")` |  | Fails: `failed`, message `""` |
| `fail("stop", [1])` |  | Fails: `failed`, message `"stop"`, data `[1]` |

### 5.2 Numbers: 8 functions

#### 5.2.1 `abs`

| Field | Value |
| --- | --- |
| Signature | `abs: (x: T) => T where T: number` |
| Receiver | None: a function |
| Steps | `0` |
| Units | `0` |
| Null | Never |
| Fails | Only as 2.6 says |
| Order | Tests; result. |

1. `x` if `x` is zero or more, else `-x`. The magnitude of a whole number is a
   whole number, so the call never fails.

| Program | Data | Result |
| --- | --- | --- |
| `abs(-3)` |  | `3` |
| `abs(2.5)` |  | `2.5` |
| `abs(0)` |  | `0` |
| `abs(-9007199254740991)` |  | `9007199254740991` |
| `abs(x)` | `{x: any}`, x = `"3"` | Fails: `wrong-kind` |

#### 5.2.2 `floor`

| Field | Value |
| --- | --- |
| Signature | `floor: (x: float) => int` |
| Receiver | None: a function |
| Steps | `0` |
| Units | `0` |
| Null | Never |
| Fails | `number-range`, template `entry`: the result is not a whole number |
| Order | Tests; result, or `number-range`. |

1. The largest integer not greater than `x`. If it is not a whole number (3.10),
   the call fails with `number-range`.

| Program | Data | Result |
| --- | --- | --- |
| `floor(2.7)` |  | `2` |
| `floor(-0.5)` |  | `-1` |
| `floor(-2)` |  | `-2` |
| `floor(x)` | `{x: float}`, x = `2.5` | `2`; 1 step, 0 units |
| `floor(1e300)` |  | Fails: `number-range` |

#### 5.2.3 `ceil`

| Field | Value |
| --- | --- |
| Signature | `ceil: (x: float) => int` |
| Receiver | None: a function |
| Steps | `0` |
| Units | `0` |
| Null | Never |
| Fails | `number-range`, template `entry`: the result is not a whole number |
| Order | Tests; result, or `number-range`. |

1. The smallest integer not less than `x`. If it is not a whole number (3.10),
   the call fails with `number-range`. A result of zero is zero, never negative
   zero.

| Program | Data | Result |
| --- | --- | --- |
| `ceil(2.1)` |  | `3` |
| `ceil(-0.5)` |  | `0` |
| `ceil(5)` |  | `5` |
| `ceil(-1e300)` |  | Fails: `number-range` |

#### 5.2.4 `round`

| Field | Value |
| --- | --- |
| Signature | `round: (x: float) => int` |
| Receiver | None: a function |
| Steps | `0` |
| Units | `0` |
| Null | Never |
| Fails | `number-range`, template `entry`: the result is not a whole number |
| Order | Tests; result, or `number-range`. |

1. The integer nearest to the exact value of `x`. Of two integers equally near,
   the one of larger magnitude: `round(2.5)` is 3 and `round(-2.5)` is -3.
2. The rounding is of the exact value, so `0.49999999999999994` gives 0.
3. If the result is not a whole number (3.10), the call fails with
   `number-range`. A result of zero is zero.

| Program | Data | Result |
| --- | --- | --- |
| `round(2.5)` |  | `3` |
| `round(-2.5)` |  | `-3` |
| `round(1.4999)` |  | `1` |
| `round(0.49999999999999994)` |  | `0` |
| `round(-0.4)` |  | `0` |
| `round(1e16)` |  | Fails: `number-range` |

#### 5.2.5 `min`

| Field | Value |
| --- | --- |
| Signature | `min: (a: T, b: T, ...rest: T[]) => T where T: ordered` |
| Receiver | None: a function |
| Steps | `len(arguments) - 1 + sz(arguments)` |
| Units | `0` |
| Null | Never |
| Fails | `wrong-kind`, template `unordered`: the arguments are not all numbers or all strings |
| Order | Tests, the ordered test over the arguments among them; steps; result. |

1. The arguments must be ordered (3.6): all numbers or all strings. Otherwise
   the call fails with `wrong-kind`.
2. The result is the least argument by the order of 3.6. Of arguments that are
   equal, the first; equal values cannot be told apart.
3. `len(arguments)` is the number of arguments, 2 or more. A call with fewer is
   a compile error (2.6).

| Program | Data | Result |
| --- | --- | --- |
| `min(3, 1, 2)` |  | `1` |
| `min("b", "a", "ab")` |  | `"a"` |
| `min("é", "z")` |  | `"z"` |
| `min(-1, -1.5)` |  | `-1.5` |
| `min(a, b, c)` | `{a: int, b: int, c: int}`, a = `3`, b = `1`, c = `2` | `1`; 3 steps, 0 units |
| `min(a, b)` | `{a: any, b: any}`, a = `1`, b = `"1"` | Fails: `wrong-kind`, template `unordered`, message `"min needs all numbers or all strings, found string"`; 0 steps, 0 units |
| `min(a, b)` | `{a: any, b: any}`, a = `[1]`, b = `2` | Fails: `wrong-kind`, template `unordered`, message `"min needs all numbers or all strings, found list"`; 0 steps, 0 units |

#### 5.2.6 `max`

| Field | Value |
| --- | --- |
| Signature | `max: (a: T, b: T, ...rest: T[]) => T where T: ordered` |
| Receiver | None: a function |
| Steps | `len(arguments) - 1 + sz(arguments)` |
| Units | `0` |
| Null | Never |
| Fails | `wrong-kind`, template `unordered`: the arguments are not all numbers or all strings |
| Order | Tests, the ordered test over the arguments among them; steps; result. |

1. As `min`, with the greatest argument. Of arguments that are equal, the first.

| Program | Data | Result |
| --- | --- | --- |
| `max(1, 5, 3)` |  | `5` |
| `max("a", "a😀", "b")` |  | `"b"` |
| `max("", " ")` |  | `" "` |
| `max(0.1, 0.2)` |  | `0.2` |
| `max(a, b)` | `{a: any, b: any}`, a = `1`, b = `null` | Fails: `wrong-kind`, template `unordered`, message `"max needs all numbers or all strings, found null"`; 0 steps, 0 units |

#### 5.2.7 `range`

| Field | Value |
| --- | --- |
| Signature | `range: (a: int, b?: int) => int[]` |
| Receiver | None: a function |
| Steps | `k` |
| Units | `1 + k` |
| Null | Never |
| Fails | Only as 2.6 says |
| Order | Tests; steps; units; build. |

1. With one argument: the whole numbers from 0 up to but not including `a`,
   ascending.
2. With two: the whole numbers from `a` up to but not including `b`, ascending.
3. Empty when there are none: `a` of 0 or less with one argument, `b` not above
   `a` with two. There is no step argument.
4. `k` is the number of elements of the result. It is found from the arguments,
   so both charges come before anything is built: `range(1e9)` fails with a
   limit and builds nothing.

| Program | Data | Result |
| --- | --- | --- |
| `range(3)` |  | `[0, 1, 2]` |
| `range(0)` |  | `[]` |
| `range(-2)` |  | `[]` |
| `range(2, 5)` |  | `[2, 3, 4]` |
| `range(5, 2)` |  | `[]` |
| `range(-2, 1)` |  | `[-2, -1, 0]` |
| `range(n)` | `{n: int}`, n = `3` | `[0, 1, 2]`; 4 steps, 4 units |
| `range(100)` |  | Fails: `steps-exceeded` (limit steps 50) |
| `range(x)` | `{x: any}`, x = `2.5` | Fails: `wrong-kind` |

#### 5.2.8 `fixed`

| Field | Value |
| --- | --- |
| Signature | `fixed: (x: float, digits: int) => string !` |
| Receiver | None: a function |
| Steps | `0` |
| Units | `1` |
| Null | Never |
| Fails | `invalid-argument`, form `bad-digits`: digits is not in 0 to 20; `invalid-argument`, form `too-large`: the magnitude of x is 1e21 or more |
| Order | Tests; `digits` tested, then the magnitude of `x`; steps; units; build. |

1. If `digits` is not in 0 to 20, the call fails with form `bad-digits`,
   parameter `digits`. This test comes first. Then, if the magnitude of `x` is
   1e21 or more, it fails with form `too-large`, parameter `x`.
2. Otherwise the result is the exact value of `x` (the double, not its shortest
   text), rounded to `digits` digits after the point, with a half rounded away
   from zero. It is written in decimal with no exponent: a `-` if the rounded
   value is below zero; the digits of its integer part, with no leading zero, or
   `0` when the integer part is zero; then, if `digits` is more than 0, a `.`
   and exactly `digits` digits.
3. A result whose digits are all zero has no minus sign.
4. The result has at most 43 code points, so its size is 0 and it costs 1 unit.

| Program | Data | Result |
| --- | --- | --- |
| `fixed(2.5, 0)` |  | `"3"` |
| `fixed(-2.5, 0)` |  | `"-3"` |
| `fixed(1.005, 2)` |  | `"1.00"` |
| `fixed(-0.001, 2)` |  | `"0.00"` |
| `fixed(1e20, 2)` |  | `"100000000000000000000.00"` |
| `fixed(0.1, 20)` |  | `"0.10000000000000000555"` |
| `fixed(7, 0)` |  | `"7"` |
| `fixed(x, 2)` | `{x: float}`, x = `3.14159` | `"3.14"`; 1 step, 1 unit |
| `fixed(1e21, 0)` |  | Fails: `invalid-argument`, form `too-large` |
| `fixed(1, 21)` |  | Fails: `invalid-argument`, form `bad-digits` |
| `fixed(1e21, 21)` |  | Fails: `invalid-argument`, form `bad-digits` |
| `fixed(x, y)` | `{x: any, y: any}`, x = `"a"`, y = `2.5` | Fails: `wrong-kind`, template `argument`, message `"fixed cannot take a value of kind string"`; 0 steps, 0 units |
| `fixed(1, y)` | `{y: any}`, y = `2.5` | Fails: `wrong-kind`, template `whole`, message `"expected a whole number, found 2.5"`; 0 steps, 0 units |

### 5.3 Strings: 13 methods

The receiver is a string, named `s` in the formulas.

#### 5.3.1 `toUpperCase`

| Field | Value |
| --- | --- |
| Signature | `string.toUpperCase: () => string` |
| Receiver | `string` |
| Steps | `sz(s)` |
| Units | `1 + sz(s)` |
| Null | Never |
| Fails | Only as 2.6 says |
| Order | Tests; steps; units; build. |

1. Each code point `c` of `s` is replaced by `upper(c)` of the case table (3.8).
   The result has as many code points as `s`, and the same weight.
2. No mapping depends on context or language: `ß` has no upper-case mapping in
   the table and stays.

| Program | Data | Result |
| --- | --- | --- |
| `"café".toUpperCase()` |  | `"CAFÉ"` |
| `"ß".toUpperCase()` |  | `"ß"` |
| `"".toUpperCase()` |  | `""` |
| `"\u{3c2}".toUpperCase()` |  | `"Σ"` |
| `"\u{1c6}".toUpperCase()` |  | `"Ǆ"` |
| `"\u{fb01}".toUpperCase()` |  | `"ﬁ"` |
| `s.toUpperCase()` | `{s: string}`, s = `"a" × 130` | `"A" × 130`; 3 steps, 3 units |

#### 5.3.2 `toLowerCase`

| Field | Value |
| --- | --- |
| Signature | `string.toLowerCase: () => string` |
| Receiver | `string` |
| Steps | `sz(s)` |
| Units | `1 + sz(s)` |
| Null | Never |
| Fails | Only as 2.6 says |
| Order | Tests; steps; units; build. |

1. Each code point `c` of `s` is replaced by `lower(c)` of the case table (3.8).
   The result has as many code points as `s`, and the same weight.
2. Capital sigma always gives `σ`, wherever it stands. `İ` gives `i`.

| Program | Data | Result |
| --- | --- | --- |
| `"ÀB".toLowerCase()` |  | `"àb"` |
| `"\u{130}".toLowerCase()` |  | `"i"` |
| `"\u{391}\u{3a3}".toLowerCase()` |  | `"ασ"` |
| `"\u{1c5}".toLowerCase()` |  | `"ǆ"` |
| `"😀A".toLowerCase()` |  | `"😀a"` |

#### 5.3.3 `trim`

| Field | Value |
| --- | --- |
| Signature | `string.trim: () => string` |
| Receiver | `string` |
| Steps | `sz(s)` |
| Units | `1 + sz(result)` |
| Null | Never |
| Fails | Only as 2.6 says |
| Order | Tests; steps; units; build. |

1. The longest run of white space (3.9) at the start of `s` and the longest at
   its end are removed. The code points between are kept, white space among
   them.
2. A string of white space only gives the empty string.

| Program | Data | Result |
| --- | --- | --- |
| `"  a b  ".trim()` |  | `"a b"` |
| `"\u{a0}\u{feff}x\u{3000}".trim()` |  | `"x"` |
| `"\u{85}x".trim()` |  | `"\u{85}x"` |
| `"\u{200b}x".trim()` |  | `"\u{200b}x"` |
| `"\t\n \r".trim()` |  | `""` |
| `s.trim()` | `{s: string}`, s = `"  x  "` | `"x"`; 1 step, 1 unit |

#### 5.3.4 `removePrefix`

| Field | Value |
| --- | --- |
| Signature | `string.removePrefix: (prefix: string) => string` |
| Receiver | `string` |
| Steps | `sz(s)` |
| Units | `1 + sz(result)` |
| Null | Never |
| Fails | Only as 2.6 says |
| Order | Tests; steps; units; build. |

1. If `s` starts with `prefix` (3.5), the code points of `s` after it. Otherwise
   `s`. The prefix is removed once.
2. An empty `prefix` gives `s`.

| Program | Data | Result |
| --- | --- | --- |
| `"refs/heads/main".removePrefix("refs/heads/")` |  | `"main"` |
| `"aaa".removePrefix("a")` |  | `"aa"` |
| `"abc".removePrefix("")` |  | `"abc"` |
| `"ab".removePrefix("abc")` |  | `"ab"` |
| `s.removePrefix("refs/heads/")` | `{s: string}`, s = `"refs/heads/main"` | `"main"`; 1 step, 1 unit |

#### 5.3.5 `removeSuffix`

| Field | Value |
| --- | --- |
| Signature | `string.removeSuffix: (suffix: string) => string` |
| Receiver | `string` |
| Steps | `sz(s)` |
| Units | `1 + sz(result)` |
| Null | Never |
| Fails | Only as 2.6 says |
| Order | Tests; steps; units; build. |

1. If `s` ends with `suffix` (3.5), the code points of `s` before it. Otherwise
   `s`. The suffix is removed once.
2. An empty `suffix` gives `s`.

| Program | Data | Result |
| --- | --- | --- |
| `"file.tar.gz".removeSuffix(".gz")` |  | `"file.tar"` |
| `"x".removeSuffix("x")` |  | `""` |
| `"abc".removeSuffix("")` |  | `"abc"` |
| `"ab".removeSuffix("xab")` |  | `"ab"` |

#### 5.3.6 `startsWith`

| Field | Value |
| --- | --- |
| Signature | `string.startsWith: (prefix: string) => bool` |
| Receiver | `string` |
| Steps | `sz(prefix)` |
| Units | `0` |
| Null | Never |
| Fails | Only as 2.6 says |
| Order | Tests; steps; result. |

1. `true` if `s` starts with `prefix` (3.5), else `false`. `true` for an empty
   `prefix`. `false` when `prefix` is longer than `s`.
2. The comparison is of code points, so case matters and nothing is normalized.

| Program | Data | Result |
| --- | --- | --- |
| `"abc".startsWith("ab")` |  | `true` |
| `"abc".startsWith("")` |  | `true` |
| `"ab".startsWith("abc")` |  | `false` |
| `"😀a".startsWith("😀")` |  | `true` |
| `"Abc".startsWith("a")` |  | `false` |
| `s.startsWith("ab")` | `{s: string}`, s = `"abc"` | `true`; 1 step, 0 units |

#### 5.3.7 `endsWith`

| Field | Value |
| --- | --- |
| Signature | `string.endsWith: (suffix: string) => bool` |
| Receiver | `string` |
| Steps | `sz(suffix)` |
| Units | `0` |
| Null | Never |
| Fails | Only as 2.6 says |
| Order | Tests; steps; result. |

1. `true` if `s` ends with `suffix` (3.5), else `false`. `true` for an empty
   `suffix`. `false` when `suffix` is longer than `s`.

| Program | Data | Result |
| --- | --- | --- |
| `"abc".endsWith("bc")` |  | `true` |
| `"".endsWith("")` |  | `true` |
| `"c".endsWith("abc")` |  | `false` |
| `"a😀".endsWith("😀")` |  | `true` |

#### 5.3.8 `split`

| Field | Value |
| --- | --- |
| Signature | `string.split: (sep: string) => string[]` |
| Receiver | `string` |
| Steps | `sz(s) + p` |
| Units | `1 + 2*p + sz(s)` |
| Null | Never |
| Fails | Only as 2.6 says |
| Order | Tests; count the parts; steps; units; build. |

1. When `sep` is not empty: the occurrences of `sep` in `s` are found left to
   right without overlap (3.5). The parts are the runs of code points before the
   first occurrence, between each two, and after the last, in order. With `c`
   occurrences there are `c + 1` parts, empty parts included. So `"".split(",")`
   is `[""]`.
2. When `sep` is empty: one string for each code point of `s`, in order. So
   `"".split("")` is `[]`.
3. `p` is the number of parts. It is found by counting before anything is
   charged.
4. There is no limit argument.

| Program | Data | Result |
| --- | --- | --- |
| `"a,b,,c".split(",")` |  | `["a", "b", "", "c"]` |
| `",".split(",")` |  | `["", ""]` |
| `"aaa".split("aa")` |  | `["", "a"]` |
| `"".split(",")` |  | `[""]` |
| `"a😀b".split("")` |  | `["a", "😀", "b"]` |
| `"".split("")` |  | `[]` |
| `"abc".split("x")` |  | `["abc"]` |
| `s.split(",")` | `{s: string}`, s = `"a,b,,c"` | `["a", "b", "", "c"]`; 5 steps, 9 units |

#### 5.3.9 `replaceAll`

| Field | Value |
| --- | --- |
| Signature | `string.replaceAll: (old: string, new: string) => string !` |
| Receiver | `string` |
| Steps | `sz(s) + sz(result)` |
| Units | `1 + sz(result)` |
| Null | Never |
| Fails | `invalid-argument`, form `empty-search`: old is the empty string |
| Order | Tests; `old` tested; count the occurrences; steps; units; build. |

1. If `old` is empty, the call fails with form `empty-search`, which has no
   parameter. This test comes before any charge.
2. Otherwise the occurrences of `old` in `s` are found left to right without
   overlap (3.5), and each is replaced by `new`. `new` is literal text: no
   character in it is special.
3. With `c` occurrences the result's weight is `w(s) + c × (w(new) − w(old))`.
   It is found by counting before anything is charged.

| Program | Data | Result |
| --- | --- | --- |
| `"a-b-c".replaceAll("-", "+")` |  | `"a+b+c"` |
| `"aaa".replaceAll("aa", "b")` |  | `"ba"` |
| `"x".replaceAll("y", "z")` |  | `"x"` |
| `"a.b".replaceAll(".", "$&")` |  | `"a$&b"` |
| `"😀x😀".replaceAll("😀", "")` |  | `"x"` |
| `s.replaceAll("-", "+")` | `{s: string}`, s = `"a-b-c"` | `"a+b+c"`; 1 step, 1 unit |
| `"abc".replaceAll("", "-")` |  | Fails: `invalid-argument`, form `empty-search` |

#### 5.3.10 `slice`

| Field | Value |
| --- | --- |
| Signature | `string.slice: (start: int, end?: int) => string` |
| Receiver | `string` |
| Steps | `sz(s)` |
| Units | `1 + sz(result)` |
| Null | Never |
| Fails | Only as 2.6 says |
| Order | Tests; steps; units; build. |

1. `start` and `end` are resolved against the length of `s` by the rule of
   positions (3.10). `end` defaults to the length.
2. The result is the code points from the resolved `start` up to but not
   including the resolved `end`. It is empty when the resolved `start` is not
   before the resolved `end`.
3. A position never fails: it is clamped.

| Program | Data | Result |
| --- | --- | --- |
| `"hello".slice(1, 3)` |  | `"el"` |
| `"hello".slice(-3)` |  | `"llo"` |
| `"hello".slice(2, -1)` |  | `"ll"` |
| `"hello".slice(10)` |  | `""` |
| `"hello".slice(-10, 2)` |  | `"he"` |
| `"hello".slice(3, 1)` |  | `""` |
| `"a😀b".slice(1, 2)` |  | `"😀"` |
| `s.slice(x)` | `{s: string, x: any}`, s = `"abc"`, x = `1.5` | Fails: `wrong-kind` |
| `x.slice(y => y)` | `{x: any}`, x = `"abc"` | Fails: `wrong-kind`, template `not-callback`; 0 steps, 0 units |

#### 5.3.11 `repeat`

| Field | Value |
| --- | --- |
| Signature | `string.repeat: (count: int) => string !` |
| Receiver | `string` |
| Steps | `sz(result)` |
| Units | `1 + sz(result)` |
| Null | Never |
| Fails | `invalid-argument`, form `negative-count`: count is below zero |
| Order | Tests; `count` tested; steps; units; build. |

1. If `count` is below zero, the call fails with form `negative-count`,
   parameter `count`. This test comes before any charge.
2. Otherwise `count` copies of `s`, one after another. A `count` of 0, or an
   empty `s`, gives the empty string.
3. The result's weight is `count × w(s)`, found from the arguments.

| Program | Data | Result |
| --- | --- | --- |
| `"ab".repeat(3)` |  | `"ababab"` |
| `"x".repeat(0)` |  | `""` |
| `"".repeat(5)` |  | `""` |
| `s.repeat(40)` | `{s: string}`, s = `"ab"` | `"ab" × 40`; 2 steps, 2 units |
| `"a".repeat(-1)` |  | Fails: `invalid-argument`, form `negative-count` |

#### 5.3.12 `padStart`

| Field | Value |
| --- | --- |
| Signature | `string.padStart: (length: int, fill?: string) => string` |
| Receiver | `string` |
| Steps | `sz(result)` |
| Units | `1 + sz(result)` |
| Null | Never |
| Fails | Only as 2.6 says |
| Order | Tests; steps; units; build. |

1. `fill` defaults to one space, U+0020.
2. If `s` has `length` code points or more, or `fill` is empty, the result is
   `s`. A `length` of zero or below gives `s`.
3. Otherwise, with `m` the number of code points missing, `length − len(s)`:
   `fill` repeated as often as needed and cut to its first `m` code points, then
   `s`.
4. The result is charged even when it is `s`.

| Program | Data | Result |
| --- | --- | --- |
| `"5".padStart(3, "0")` |  | `"005"` |
| `"abc".padStart(6, "12")` |  | `"121abc"` |
| `"abc".padStart(2)` |  | `"abc"` |
| `"a".padStart(3)` |  | `"  a"` |
| `"a".padStart(3, "")` |  | `"a"` |
| `"x".padStart(3, "😀")` |  | `"😀😀x"` |
| `s.padStart(3, "0")` | `{s: string}`, s = `"5"` | `"005"`; 1 step, 1 unit |

#### 5.3.13 `padEnd`

| Field | Value |
| --- | --- |
| Signature | `string.padEnd: (length: int, fill?: string) => string` |
| Receiver | `string` |
| Steps | `sz(result)` |
| Units | `1 + sz(result)` |
| Null | Never |
| Fails | Only as 2.6 says |
| Order | Tests; steps; units; build. |

1. As `padStart`, with the cut fill after `s`.

| Program | Data | Result |
| --- | --- | --- |
| `"5".padEnd(3, "0")` |  | `"500"` |
| `"abc".padEnd(6, "12")` |  | `"abc121"` |
| `"abc".padEnd(-1)` |  | `"abc"` |
| `"a".padEnd(3)` |  | `"a  "` |
| `"😀".padEnd(2, "ab")` |  | `"😀a"` |

### 5.4 Lists: 22 methods

The receiver is a list of `n` elements, of type `T[]` unless the record says
otherwise. A callback is called with the element, `item`, and its index,
`index`; the callback of `reduce` takes the accumulated value, `acc`, before
them.

#### 5.4.1 `filter`

| Field | Value |
| --- | --- |
| Signature | `T[].filter: (f: (item: T, index: int) => unknown) => T[]` |
| Receiver | `T[]` |
| Steps | `2*n` |
| Units | `1 + k` |
| Null | Never |
| Fails | Only as 2.6 says |
| Order | For each element: 2 steps, then `f`. Then units; build. |
| Narrows | Through `f` (section 6) |

1. `f` is called once for each element, in order. The result holds the elements
   for which `f` is truthy, in their order.
2. `k` is the number of elements kept. Units are charged after the last call.
3. An empty or a non-empty list is tested by truthiness like any value, so
   `xs.filter(x => x)` drops empty lists.
4. The checker narrows the result through `f` (section 6).

| Program | Data | Result |
| --- | --- | --- |
| `xs.filter(x => x > 1)` | `{xs: int[]}`, xs = `[3, 1, 2]` | `[3, 2]`; 11 steps, 3 units |
| `[[], [1], [], [2, 3]].filter(x => x)` |  | `[[1], [2, 3]]` |
| `xs.filter(x => x > 1)` | `{xs: int[]}`, xs = `[]` | `[]` |
| `xs.filter(x => x != null).sum()` | `{xs: (int \| null)[]}`, xs = `[1, null, 3]` | `4`, type `int` |
| `["a", "b", "c"].filter((x, i) => i != 1)` |  | `["a", "c"]` |
| `xs.filter(x => x)` | `{xs: any}`, xs = `null` | Fails: `null-operand`; 0 steps, 0 units |

#### 5.4.2 `map`

| Field | Value |
| --- | --- |
| Signature | `T[].map: (f: (item: T, index: int) => U) => U[]` |
| Receiver | `T[]` |
| Steps | `2*n` |
| Units | `1 + n` |
| Null | Never |
| Fails | Only as 2.6 says |
| Order | For each element: 2 steps, then `f`. Then units; build. |

1. `f` is called once for each element, in order. The result holds the results,
   in order.
2. Units are charged after the last call.

| Program | Data | Result |
| --- | --- | --- |
| `xs.map((x, i) => x * i)` | `{xs: int[]}`, xs = `[1, 2, 3]` | `[0, 2, 6]`; 11 steps, 4 units |
| `xs.map(x => x * 2)` | `{xs: int[]}`, xs = `[]` | `[]` |
| `[[1, 2], []].map(x => len(x))` |  | `[2, 0]` |
| `[1, 0].map(x => 10 / x)` |  | Fails: `divide-by-zero` |
| `x.map(1)` | `{x: any}`, x = `[1]` | Fails: `wrong-kind`, template `callback`; 0 steps, 0 units |
| `x.map((a, b, c) => a)` | `{x: any}`, x = `[1]` | Fails: `wrong-kind`, template `parameters`; 0 steps, 0 units |
| `x.map(1, (a, b, c) => a)` | `{x: any}`, x = `[1]` | Fails: `wrong-kind`, template `count`; 0 steps, 0 units |

#### 5.4.3 `flatMap`

| Field | Value |
| --- | --- |
| Signature | `T[].flatMap: (f: (item: T, index: int) => U[]) => U[]` |
| Receiver | `T[]` |
| Steps | `2*n + k` |
| Units | `1 + k` |
| Null | Never |
| Fails | `wrong-kind`, template `result`: a result of f is not a list |
| Order | For each element: 2 steps, then `f`, then its result is tested. Then steps `k`; units; build. |

1. `f` is called once for each element, in order. Each result must be a list. A
   result that is not a list, null included, fails with `wrong-kind` when that
   call returns, before the next element.
2. The result holds the elements of each result list, in order: those of the
   first element's list, then the second's, and so on.
3. `k` is the total number of elements of the result lists. Steps `k` and units
   are charged after the last call.

| Program | Data | Result |
| --- | --- | --- |
| `xs.flatMap(x => x)` | `{xs: int[][]}`, xs = `[[1], [2, 3]]` | `[1, 2, 3]`; 9 steps, 4 units |
| `[1, 2, 3].flatMap(x => range(x))` |  | `[0, 0, 1, 0, 1, 2]` |
| `[1, 2].flatMap(x => [])` |  | `[]` |
| `xs.flatMap(x => x)` | `{xs: any}`, xs = `[1, 2]` | Fails: `wrong-kind`; 4 steps, 0 units |

#### 5.4.4 `some`

| Field | Value |
| --- | --- |
| Signature | `T[].some: (f: (item: T, index: int) => unknown) => bool` |
| Receiver | `T[]` |
| Steps | `2*v` |
| Units | `0` |
| Null | Never |
| Fails | Only as 2.6 says |
| Order | For each element visited: 2 steps, then `f`. Stops at the first truthy result. |

1. `f` is called for each element in order, until a result is truthy. Then the
   result is `true` and no later element is visited. If no result is truthy,
   `false`.
2. `false` for an empty list.
3. `v` is the number of elements visited.

| Program | Data | Result |
| --- | --- | --- |
| `xs.some(x => x > 1)` | `{xs: int[]}`, xs = `[1, 5, 0]` | `true`; 8 steps, 0 units |
| `xs.some(x => x > 1)` | `{xs: int[]}`, xs = `[]` | `false` |
| `[1, 0].some(x => 10 / x > 0)` |  | `true` |
| `["", 0].some(x => x)` |  | `false` |

#### 5.4.5 `every`

| Field | Value |
| --- | --- |
| Signature | `T[].every: (f: (item: T, index: int) => unknown) => bool` |
| Receiver | `T[]` |
| Steps | `2*v` |
| Units | `0` |
| Null | Never |
| Fails | Only as 2.6 says |
| Order | For each element visited: 2 steps, then `f`. Stops at the first falsy result. |

1. `f` is called for each element in order, until a result is falsy. Then the
   result is `false` and no later element is visited. If every result is truthy,
   `true`.
2. `true` for an empty list.
3. `v` is the number of elements visited.

| Program | Data | Result |
| --- | --- | --- |
| `xs.every(x => x)` | `{xs: int[]}`, xs = `[1, 0, 2]` | `false`; 6 steps, 0 units |
| `xs.every(x => x > 1)` | `{xs: int[]}`, xs = `[]` | `true` |
| `[2, 4].every(x => x % 2 == 0)` |  | `true` |
| `[5, 0].every(x => 10 / x > 5)` |  | `false` |

#### 5.4.6 `find`

| Field | Value |
| --- | --- |
| Signature | `T[].find: (f: (item: T, index: int) => unknown) => T \| null` |
| Receiver | `T[]` |
| Steps | `2*v` |
| Units | `0` |
| Null | No element makes f true, which includes an empty list |
| Fails | Only as 2.6 says |
| Order | For each element visited: 2 steps, then `f`. Stops at the first truthy result. |
| Narrows | Through `f` (section 6) |

1. `f` is called for each element in order, until a result is truthy. The result
   is that element, and no later element is visited.
2. Null if no result is truthy, and for an empty list. An element that is null
   gives null too: the two cannot be told apart.
3. `v` is the number of elements visited.
4. The checker narrows the result through `f` (section 6).

| Program | Data | Result |
| --- | --- | --- |
| `xs.find(x => x % 2 == 0)` | `{xs: int[]}`, xs = `[1, 4, 6]` | `4`; 10 steps, 0 units |
| `[1, 3].find(x => x > 5)` |  | `null` |
| `[0, 5, 7].find(x => x)` |  | `5` |
| `[null, 1].find(x => x == null)` |  | `null` |
| `xs.find(x => x != null)` | `{xs: (int \| null)[]}`, xs = `[null, 2]` | `2`, type `int \| null` |

#### 5.4.7 `count`

| Field | Value |
| --- | --- |
| Signature | `T[].count: (f: (item: T, index: int) => unknown) => int` |
| Receiver | `T[]` |
| Steps | `2*n` |
| Units | `0` |
| Null | Never |
| Fails | Only as 2.6 says |
| Order | For each element: 2 steps, then `f`. Then the result. |

1. `f` is called once for each element, in order. The result is the number of
   truthy results. It does not stop early.
2. 0 for an empty list.

| Program | Data | Result |
| --- | --- | --- |
| `xs.count(s => s)` | `{xs: string[]}`, xs = `["a", "", "b"]` | `2`; 8 steps, 0 units |
| `xs.count(x => x > 1)` | `{xs: int[]}`, xs = `[]` | `0` |
| `[1, 2, 3].count(x => x > 5)` |  | `0` |

#### 5.4.8 `reduce`

| Field | Value |
| --- | --- |
| Signature | `T[].reduce: (f: (acc: A, item: T, index: int) => A, init: A) => A` |
| Receiver | `T[]` |
| Steps | `2*n` |
| Units | `0` |
| Null | Never |
| Fails | Only as 2.6 says |
| Order | For each element: 2 steps, then `f`. Then the result. |

1. `f` is called once for each element, in order. The first call is given
   `init`, the first element and 0. Each later call is given the result of the
   call before it, the element and its index. The result is the result of the
   last call.
2. `init` for an empty list. `init` is required. Its type is the type variable
   `A`, which has no bound, so it accepts every value, null included (2.6).
3. The entry creates nothing. What `f` creates, `f` charges.

| Program | Data | Result |
| --- | --- | --- |
| `xs.reduce((acc, x) => acc + x, 0)` | `{xs: int[]}`, xs = `[1, 2, 3]` | `6`; 11 steps, 0 units |
| `xs.reduce((acc, x) => acc + x, 10)` | `{xs: int[]}`, xs = `[]` | `10` |
| `["a", "b"].reduce((acc, x, i) => acc + x + string(i), "")` |  | `"a0b1"` |
| `[].reduce((acc, x) => 0, null)` |  | `null`; 2 steps, 1 unit |
| `[1, 2].reduce((acc, x) => (acc ?? 0) + x, null)` |  | `3`; 8 steps, 3 units |

#### 5.4.9 `sum`

| Field | Value |
| --- | --- |
| Signature | `T[].sum: () => T \| int where T: number` |
| Receiver | `T[]` |
| Steps | `n` |
| Units | `0` |
| Null | Never |
| Fails | `number-range`, template `entry`: an addition gives a result out of range or not finite; `wrong-kind`, template `element`: an element is not a number |
| Order | Tests, every element a number; steps; the additions. |

1. Every element must be a number. On `any` data an element of another kind,
   null included, fails with `wrong-kind`. All elements are tested before any is
   added.
2. The result is `0 + x0 + x1 + …`, each `+` the language's, left to right
   (`01-language.md`). An addition that leaves the whole range on whole
   operands, or that is not finite, fails with `number-range` there, even when a
   later element would bring the total back.
3. 0 for an empty list. The result type is `T | int`, so the sum of an empty
   list is typed `int`, which is what it gives.

| Program | Data | Result |
| --- | --- | --- |
| `xs.sum()` | `{xs: int[]}`, xs = `[1, 2, 3]` | `6`; 4 steps, 0 units |
| `xs.sum()` | `{xs: int[]}`, xs = `[]` | `0` |
| `[0.1, 0.2].sum()` |  | `0.30000000000000004` |
| `[1, 2.5].sum()` |  | `3.5` |
| `[9007199254740991, 1, -1].sum()` |  | Fails: `number-range` |
| `[1e308, 1e308].sum()` |  | Fails: `number-range` |
| `xs.sum()` | `{xs: any}`, xs = `[1, "2"]` | Fails: `wrong-kind`; 3 steps, 0 units |
| `xs.filter(x => x).sum()` | `{xs: any}`, xs = `[1, "2"]` | Fails: `wrong-kind` |

#### 5.4.10 `min`

| Field | Value |
| --- | --- |
| Signature | `T[].min: () => T \| null where T: ordered` |
| Receiver | `T[]` |
| Steps | `n + sz(elements)` |
| Units | `0` |
| Null | The list is empty |
| Fails | `wrong-kind`, template `unordered`: the elements are not all numbers or all strings |
| Order | Tests, the ordered test over the elements among them; steps; result. |

1. The elements must be ordered (3.6). Every element is tested before any is
   compared, so a list of one null fails with `wrong-kind`. A failed test
   charges 1 step for each element tested (4.5).
2. The least element by the order of 3.6. Of equal elements, the first.
3. Null for an empty list.

| Program | Data | Result |
| --- | --- | --- |
| `xs.min()` | `{xs: string[]}`, xs = `["b", "a", "c"]` | `"a"`; 4 steps, 0 units |
| `[3, 1, 2].min()` |  | `1` |
| `xs.min()` | `{xs: int[]}`, xs = `[]` | `null` |
| `[5].min()` |  | `5` |
| `xs.min()` | `{xs: any}`, xs = `[null]` | Fails: `wrong-kind`; 2 steps, 0 units |

#### 5.4.11 `max`

| Field | Value |
| --- | --- |
| Signature | `T[].max: () => T \| null where T: ordered` |
| Receiver | `T[]` |
| Steps | `n + sz(elements)` |
| Units | `0` |
| Null | The list is empty |
| Fails | `wrong-kind`, template `unordered`: the elements are not all numbers or all strings |
| Order | Tests, the ordered test over the elements among them; steps; result. |

1. As `min`, with the greatest element. Of equal elements, the first.

| Program | Data | Result |
| --- | --- | --- |
| `["b", "a"].max()` |  | `"b"` |
| `[3, 1, 2].max()` |  | `3` |
| `xs.max()` | `{xs: int[]}`, xs = `[]` | `null` |
| `["\u{ffff}", "😀"].max()` |  | `"😀"` |
| `xs.max()` | `{xs: any}`, xs = `[1, "a"]` | Fails: `wrong-kind`; 3 steps, 0 units |

#### 5.4.12 `sort`

| Field | Value |
| --- | --- |
| Signature | `T[].sort: () => T[] where T: ordered` |
| Receiver | `T[]` |
| Steps | `n + L*(n + sz(elements))` |
| Units | `1 + n` |
| Null | Never |
| Fails | `wrong-kind`, template `unordered`: the elements are not all numbers or all strings |
| Order | Tests, the ordered test over the elements among them; steps; units; sort. |

1. The elements must be ordered (3.6). Every element is tested before any is
   compared.
2. The result is the elements in ascending order by 3.6, by a stable sort (3.7).
   Numbers compare by value and strings by code point, never as text of numbers.
3. There is no comparator and no direction. `xs.sort().reverse()` gives
   descending order: equal numbers and equal strings cannot be told apart.

| Program | Data | Result |
| --- | --- | --- |
| `xs.sort()` | `{xs: int[]}`, xs = `[3, 1, 2]` | `[1, 2, 3]`; 10 steps, 4 units |
| `[10, 9, 1].sort()` |  | `[1, 9, 10]` |
| `["b", "a", "B"].sort()` |  | `["B", "a", "b"]` |
| `["é", "z", "e"].sort()` |  | `["e", "z", "é"]` |
| `["😀", "\u{ffff}"].sort()` |  | `["\u{ffff}", "😀"]` |
| `xs.sort()` | `{xs: int[]}`, xs = `[]` | `[]` |
| `xs.sort()` | `{xs: any}`, xs = `[1, "a", 2]` | Fails: `wrong-kind`; 3 steps, 0 units |
| `xs.sort()` | `{xs: any}`, xs = `[1, "a", null]` | Fails: `wrong-kind`, template `unordered`, message `"sort needs all numbers or all strings, found string"`; 3 steps, 0 units |

#### 5.4.13 `sortBy`

| Field | Value |
| --- | --- |
| Signature | `T[].sortBy: (f: (item: T, index: int) => K) => T[] where K: ordered` |
| Receiver | `T[]` |
| Steps | `2*n + L*(n + sz(keys))` |
| Units | `1 + n` |
| Null | Never |
| Fails | `wrong-kind`, template `unordered`: the keys are not all numbers or all strings |
| Order | For each element: 2 steps, then `f`. Then the ordered test over the keys; steps `L*(n + sz(keys))`; units; sort. |

1. `f` is called once for each element, in order, before any comparison. Its
   result is the element's key.
2. The keys must be ordered (3.6). They are tested after the last call and
   before any comparison. A null key, a key of another kind, or a mix of numbers
   and strings fails with `wrong-kind`.
3. The result is the elements in ascending order of their keys, by a stable sort
   (3.7): elements with equal keys keep their order.
4. `sz(keys)` is the size of the keys that are strings, taken together. The sort
   term is charged after the keys are known and tested.
5. In Core `sortBy` takes one argument.

| Program | Data | Result |
| --- | --- | --- |
| `xs.sortBy(s => len(s))` | `{xs: string[]}`, xs = `["bb", "a", "cc", "d"]` | `["a", "d", "bb", "cc"]`; 22 steps, 5 units |
| `[{n: "b", a: 2}, {n: "a", a: 1}, {n: "c", a: 2}].sortBy(p => p.a).map(p => p.n)` |  | `["a", "b", "c"]` |
| `xs.sortBy(x => x)` | `{xs: int[]}`, xs = `[]` | `[]` |
| `["😀", "\u{ffff}"].sortBy(s => s)` |  | `["\u{ffff}", "😀"]` |
| `xs.sortBy(x => x)` | `{xs: any}`, xs = `[1, null]` | Fails: `wrong-kind`; 6 steps, 0 units |
| `xs.sortBy(x => x)` | `{xs: any}`, xs = `["a", 1]` | Fails: `wrong-kind` |
| `[1, 0].sortBy(x => 10 / x)` |  | Fails: `divide-by-zero` |

#### 5.4.14 `groupBy`

| Field | Value |
| --- | --- |
| Signature | `T[].groupBy: (f: (item: T, index: int) => string) => map<T[]>` |
| Receiver | `T[]` |
| Steps | `2*n + sz(keys)` |
| Units | `1 + 3*g + n` |
| Null | Never |
| Fails | `wrong-kind`, template `result`: a result of f is not a string |
| Order | For each element: 2 steps, then `f`, then its result is tested. Then steps `sz(keys)`; units; build. |

1. `f` is called once for each element, in order. Each result must be a string,
   the element's key. A result that is not a string, null and numbers included,
   fails with `wrong-kind` when that call returns.
2. The result has one entry for each distinct key. Its value is the list of the
   elements with that key, in their order. To group by a number, the callback
   gives `string(n)`.
3. `g` is the number of distinct keys. `sz(keys)` is the size of the keys the
   callback gave, one for each element, taken together: the key rule of 4.1. It
   and the units are charged after the last call.
4. The groups are found with a hash map over the keys, so that the expected work
   is at most a constant times the steps charged (3.5).
5. Like any map, the result shows its entries in key order (3.11).

| Program | Data | Result |
| --- | --- | --- |
| `xs.groupBy(x => x)` | `{xs: string[]}`, xs = `["b", "a", "b"]` | `{a: ["a"], b: ["b", "b"]}`; 8 steps, 10 units |
| `xs.groupBy(x => x)` | `{xs: string[]}`, xs = `["k" × 64, "k" × 64]` | `{kkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkk: ["k" × 64, "k" × 64]}`; 8 steps, 6 units |
| `["apple", "avocado", "banana"].groupBy(s => s.slice(0, 1))` |  | `{a: ["apple", "avocado"], b: ["banana"]}` |
| `["b", "a"].groupBy(x => x).keys()` |  | `["a", "b"]` |
| `xs.groupBy(x => x)` | `{xs: string[]}`, xs = `[]` | `{}` |
| `xs.groupBy(x => x)` | `{xs: any}`, xs = `[1]` | Fails: `wrong-kind`; 4 steps, 0 units |

#### 5.4.15 `toMap`

| Field | Value |
| --- | --- |
| Signature | `{key: string, value: T}[].toMap: () => map<T>` |
| Receiver | `{key: string, value: T}[]` |
| Steps | `n + sz(keys)` |
| Units | `1 + 2*n` |
| Null | Never |
| Fails | `wrong-kind`, template `element`: an element is not a map; `wrong-kind`, template `key`: the key field of an element is not a string; `missing-field`, template `field`: an element has no key named key or value |
| Order | Tests, every element; steps; units; build. |

1. Each element must be a map with the keys `key` and `value`, and `key` must
   hold a string. The elements are tested in order, before any charge, and each
   element in this order: it is a map, else `wrong-kind`; it has `key`, else
   `missing-field`; its `key` is a string, else `wrong-kind`; it has `value`,
   else `missing-field`. Its other keys are not read.
2. The result maps each element's `key` to its `value`. When two elements have
   the same key, the later one's value is kept.
3. `sz(keys)` is the size of the `key` of every element, taken together: the key
   rule of 4.1. Units are `1 + 2*n` whatever keys repeat.
4. The map is built with a hash map over the keys, so that the expected work is
   at most a constant times the steps charged (3.5).

| Program | Data | Result |
| --- | --- | --- |
| `es.toMap()` | `{es: {key: string, value: int}[]}`, es = `[{key: "a", value: 1}, {key: "a", value: 2}]` | `{a: 2}`; 3 steps, 5 units |
| `es.toMap()` | `{es: {key: string, value: int}[]}`, es = `[{key: "k" × 64, value: 1}]` | `{kkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkk: 1}`; 3 steps, 3 units |
| `[{key: "a", value: 1}, {key: "b", value: 2}].toMap()` |  | `{a: 1, b: 2}` |
| `{b: 1, a: 2}.entries().toMap()` |  | `{a: 2, b: 1}` |
| `es.toMap()` | `{es: {key: string, value: int}[]}`, es = `[]` | `{}` |
| `es.toMap()` | `{es: any}`, es = `[{key: "a"}]` | Fails: `missing-field`; 2 steps, 0 units |
| `es.toMap()` | `{es: any}`, es = `[{key: "a", value: 1}, {key: 1, value: 2}]` | Fails: `wrong-kind`; 3 steps, 0 units |

#### 5.4.16 `unique`

| Field | Value |
| --- | --- |
| Signature | `T[].unique: () => T[] where T: ordered` |
| Receiver | `T[]` |
| Steps | `n + sz(elements)` |
| Units | `1 + k` |
| Null | Never |
| Fails | `wrong-kind`, template `unordered`: the elements are not all numbers or all strings |
| Order | Tests, the ordered test over the elements among them; steps; find `k`; units; build. |

1. The elements must be ordered (3.6). Every element is tested before any is
   compared.
2. The result holds each element that is not `==` to an earlier element, in
   order. So the first occurrence of each value is kept.
3. `k` is the number of elements kept. It is found before the units are charged.
4. The elements already kept are found with a hash set over the values, in which
   `1` and `1.0` are one value, so that the expected work is at most a constant
   times `n + w(elements)` (3.5).

| Program | Data | Result |
| --- | --- | --- |
| `xs.unique()` | `{xs: int[]}`, xs = `[3, 1, 3, 2, 1]` | `[3, 1, 2]`; 6 steps, 4 units |
| `["a", "A", "a"].unique()` |  | `["a", "A"]` |
| `[1, 1.0].unique()` |  | `[1]` |
| `xs.unique()` | `{xs: int[]}`, xs = `[]` | `[]` |
| `xs.unique()` | `{xs: any}`, xs = `[[1]]` | Fails: `wrong-kind`; 2 steps, 0 units |

#### 5.4.17 `flat`

| Field | Value |
| --- | --- |
| Signature | `U[][].flat: () => U[]` |
| Receiver | `U[][]` |
| Steps | `n + k` |
| Units | `1 + k` |
| Null | Never |
| Fails | `wrong-kind`, template `element`: an element is not a list |
| Order | Tests, every element a list; steps; units; build. |

1. Every element must be a list. On `any` data an element of another kind, null
   included, fails with `wrong-kind`, before any charge.
2. The result holds the elements of each element, in order. Only one level is
   removed. There is no depth argument.
3. `k` is the total number of elements of the inner lists.

| Program | Data | Result |
| --- | --- | --- |
| `xs.flat()` | `{xs: int[][]}`, xs = `[[1, 2], [], [3]]` | `[1, 2, 3]`; 7 steps, 4 units |
| `[[[1]], [2]].flat()` |  | `[[1], 2]` |
| `xs.flat()` | `{xs: int[][]}`, xs = `[]` | `[]` |
| `xs.flat()` | `{xs: any}`, xs = `[[1], 2]` | Fails: `wrong-kind`; 3 steps, 0 units |

#### 5.4.18 `reverse`

| Field | Value |
| --- | --- |
| Signature | `T[].reverse: () => T[]` |
| Receiver | `T[]` |
| Steps | `n` |
| Units | `1 + n` |
| Null | Never |
| Fails | Only as 2.6 says |
| Order | Tests; steps; units; build. |

1. The last element first, then the one before it, and so on. The receiver is
   not changed.

| Program | Data | Result |
| --- | --- | --- |
| `xs.reverse()` | `{xs: int[]}`, xs = `[1, 2, 3]` | `[3, 2, 1]`; 4 steps, 4 units |
| `xs.reverse()` | `{xs: int[]}`, xs = `[]` | `[]` |
| `["a"].reverse()` |  | `["a"]` |

#### 5.4.19 `first`

| Field | Value |
| --- | --- |
| Signature | `T[].first: () => T \| null` |
| Receiver | `T[]` |
| Steps | `0` |
| Units | `0` |
| Null | The list is empty |
| Fails | Only as 2.6 says |
| Order | Tests; result. |

1. The element at index 0. Null for an empty list. A first element that is null
   also gives null.

| Program | Data | Result |
| --- | --- | --- |
| `xs.first()` | `{xs: int[]}`, xs = `[7, 8]` | `7`; 1 step, 0 units |
| `xs.first()` | `{xs: int[]}`, xs = `[]` | `null` |
| `[null, 1].first()` |  | `null` |

#### 5.4.20 `last`

| Field | Value |
| --- | --- |
| Signature | `T[].last: () => T \| null` |
| Receiver | `T[]` |
| Steps | `0` |
| Units | `0` |
| Null | The list is empty |
| Fails | Only as 2.6 says |
| Order | Tests; result. |

1. The element at index `n − 1`. Null for an empty list. A last element that is
   null also gives null.

| Program | Data | Result |
| --- | --- | --- |
| `[7, 8].last()` |  | `8` |
| `xs.last()` | `{xs: int[]}`, xs = `[]` | `null` |
| `["a"].last()` |  | `"a"` |

#### 5.4.21 `slice`

| Field | Value |
| --- | --- |
| Signature | `T[].slice: (start: int, end?: int) => T[]` |
| Receiver | `T[]` |
| Steps | `k` |
| Units | `1 + k` |
| Null | Never |
| Fails | Only as 2.6 says |
| Order | Tests; steps; units; build. |

1. `start` and `end` are resolved against the length of the list by the rule of
   positions (3.10). `end` defaults to the length.
2. The result is the elements from the resolved `start` up to but not including
   the resolved `end`. It is empty when the resolved `start` is not before the
   resolved `end`.
3. `k` is the number of elements of the result, found from the arguments and the
   length.

| Program | Data | Result |
| --- | --- | --- |
| `xs.slice(1, 4)` | `{xs: int[]}`, xs = `[1, 2, 3, 4, 5]` | `[2, 3, 4]`; 4 steps, 4 units |
| `[1, 2, 3, 4, 5].slice(1, -1)` |  | `[2, 3, 4]` |
| `[1, 2, 3, 4, 5].slice(-2)` |  | `[4, 5]` |
| `[1, 2, 3].slice(2, 1)` |  | `[]` |
| `[1, 2, 3].slice(0, 100)` |  | `[1, 2, 3]` |
| `xs.slice(0)` | `{xs: int[]}`, xs = `[]` | `[]` |

#### 5.4.22 `join`

| Field | Value |
| --- | --- |
| Signature | `string[].join: (sep: string) => string` |
| Receiver | `string[]` |
| Steps | `n + sz(result)` |
| Units | `1 + sz(result)` |
| Null | Never |
| Fails | `wrong-kind`, template `element`: an element is not a string |
| Order | Tests, every element a string; steps; units; build. |

1. Every element must be a string. On `any` data an element of another kind,
   null included, fails with `wrong-kind`, before any charge. Nothing is
   converted.
2. The result is the elements in order with `sep` between each two. The empty
   string for an empty list, and the element for a list of one.
3. `sep` is required. The result's weight is the sum of the elements' weights
   plus `(n − 1) × w(sep)`, found before any charge.

| Program | Data | Result |
| --- | --- | --- |
| `xs.join(", ")` | `{xs: string[]}`, xs = `["a", "b", "c"]` | `"a, b, c"`; 4 steps, 1 unit |
| `xs.join(",")` | `{xs: string[]}`, xs = `[]` | `""` |
| `["x"].join("-")` |  | `"x"` |
| `["a", "", "b"].join("-")` |  | `"a--b"` |
| `xs.join(",")` | `{xs: any}`, xs = `["a", 1]` | Fails: `wrong-kind`; 3 steps, 0 units |

### 5.5 Maps: 3 methods

The receiver is a map of `n` entries, of type `map<T>`, or a record, whose `T`
is the join of its field types (`05-types.md`).

#### 5.5.1 `keys`

| Field | Value |
| --- | --- |
| Signature | `map<T>.keys: () => string[]` |
| Receiver | `map<T>` |
| Steps | `n + L*(n + sz(keys))` |
| Units | `1 + n` |
| Null | Never |
| Fails | Only as 2.6 says |
| Order | Tests; steps; units; build. |

1. The keys, in key order (3.11).
2. The empty list for an empty map.

| Program | Data | Result |
| --- | --- | --- |
| `m.keys()` | `{m: map<int>}`, m = `{b: 1, a: 2, c: 3}` | `["a", "b", "c"]`; 10 steps, 4 units |
| `m.keys()` | `{m: map<int>}`, m = `{}` | `[]` |
| `{"é": 1, z: 2}.keys()` |  | `["z", "é"]` |
| `{keys: 5}.keys()` |  | `["keys"]` |

#### 5.5.2 `values`

| Field | Value |
| --- | --- |
| Signature | `map<T>.values: () => T[]` |
| Receiver | `map<T>` |
| Steps | `n + L*(n + sz(keys))` |
| Units | `1 + n` |
| Null | Never |
| Fails | Only as 2.6 says |
| Order | Tests; steps; units; build. |

1. The values, in the key order (3.11) of their keys.
2. The empty list for an empty map.

| Program | Data | Result |
| --- | --- | --- |
| `m.values()` | `{m: map<int>}`, m = `{b: 1, a: 2, c: 3}` | `[2, 1, 3]`; 10 steps, 4 units |
| `m.values()` | `{m: map<int>}`, m = `{}` | `[]` |
| `{b: null, a: 1}.values()` |  | `[1, null]` |

#### 5.5.3 `entries`

| Field | Value |
| --- | --- |
| Signature | `map<T>.entries: () => {key: string, value: T}[]` |
| Receiver | `map<T>` |
| Steps | `n + L*(n + sz(keys))` |
| Units | `1 + 6*n` |
| Null | Never |
| Fails | Only as 2.6 says |
| Order | Tests; steps; units; build. |

1. One record for each entry, in key order (3.11). Each record has exactly two
   keys: `key`, the entry's key, and `value`, its value.
2. The empty list for an empty map.
3. Each record costs `1 + 2 × 2` units, so the units are `1 + 6*n`.

| Program | Data | Result |
| --- | --- | --- |
| `m.entries()` | `{m: map<int>}`, m = `{b: 1, a: 2, c: 3}` | `[{key: "a", value: 2}, {key: "b", value: 1}, {key: "c", value: 3}]`; 10 steps, 19 units |
| `m.entries()` | `{m: map<int>}`, m = `{}` | `[]` |
| `{b: 1, a: 2}.entries().map(({key, value}) => key + string(value))` |  | `["a2", "b1"]` |
| `x.entries()` | `{x: any}`, x = `[1]` | Fails: `wrong-kind`; 0 steps, 0 units |
| `x.entries(1 / n)` | `{x: any, n: int}`, x = `{a: 1}`, n = `0` | Fails: `wrong-kind`, template `count`; 0 steps, 0 units |
## 6. Narrowing

1. `filter` and `find` narrow. Their records have `narrows: "f"`.
2. Where the lambda passed as `f` has a name as its first parameter, the checker
   gives the element of the result the type that parameter has under the true
   facts of the lambda's body. `05-types.md` has the rule.
3. Narrowing changes only the type. The value, the steps and the units are those
   of the entry.

| Program | Data | Result type |
| --- | --- | --- |
| `xs.filter(x => x != null)` | `{xs: (int \| null)[]}` | `int[]` |
| `xs.find(x => x != null)` | `{xs: (int \| null)[]}` | `int \| null` |

## 7. What packages add

A package of the specification adds entries and changes none. Its chapter is
the home of its entries, written in the form of section 5, and its records are
in a file of their own beside `suite/library.json`, in the same format.

| Package | Adds | Chapter | Records |
| --- | --- | --- | --- |
| F6, opt-in groups | The groups `json`, `time`, `math` and `base64`, each off until an environment turns it on: 8 entries and the constants `math.pi` and `math.e` | `spec/packages/f06-groups.md` | `suite/library-groups.json` |
| F13, math functions | Eleven entries of the group `math`, each correctly rounded: `pow`, `exp`, `log`, `log10`, `sin`, `cos`, `tan`, `asin`, `acos`, `atan`, `atan2` | `spec/packages/f13-math.md` | `suite/library-math.json` |
| F7, queries | `minBy`, `maxBy`, `uniqueBy`, `indexBy`, `countBy` and `mapValues`, and a second parameter of `sortBy`, in a record that extends Core's | `spec/packages/f07-queries.md` | `suite/library-queries.json` |
