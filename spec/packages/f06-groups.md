# F6. Opt-in groups

This chapter is normative. It specifies follow-on package F6: the four opt-in
library groups `json`, `time`, `math` and `base64`. A group is a namespace of
library functions that an environment turns on. The package adds 8 library
entries and 2 constants, a declaration of the environment, two fields of the
case format, and the file `suite/library-groups.json`. It adds no node kind, no
value, no type, no run code, no message template and no reserved word.

The package only adds. In Core no group can be on, so every program that names
a group fails to compile with `undefined-name` (`03-host.md` 3.3.3,
`05-types.md` 22.4). An environment that turns no group on behaves exactly as
in Core. Section 12 lists the Core cases this package touches: there are none
to replace.

Other chapters: `01-language.md` (calls, the JSON form, numbers and text),
`02-evaluation.md` (cost, limits, the depth count), `03-host.md` (environments,
diagnostics, the catalog, case files), `04-library.md` (the record format, the
cost rules, how an entry is specified), `05-types.md` (type text, the checker,
the static rules), and `spec/packages/f01-host-types.md` (the description of an
environment).

## 1. Terms and scope

### 1.1 Terms

| Term | Meaning |
| --- | --- |
| Group | One of the four opt-in groups: `json`, `time`, `math` and `base64` |
| Entry of a group | A library function whose name has the group as its namespace: `json.parse` |
| Constant | A number of a group that is read without a call: `math.pi` and `math.e` |
| On, off | A group is on in an environment that declares it (2.1). Otherwise it is off |
| UTF-8 | The encoding form of Unicode 16.0.0, chapter 3, definition D92 (4.1) |
| Correctly rounded | The binary64 value nearest to the exact mathematical value, ties to even |
| The default group | The group `core` of `04-library.md`, which is always on and is not an opt-in group |

### 1.2 The entries

| Group | Entries | Constants | Section |
| --- | --- | --- | --- |
| `json` | 2: `json.stringify`, `json.parse` | 0 | 6 |
| `time` | 3: `time.parse`, `time.format`, `time.duration` | 0 | 7 |
| `math` | 1: `math.sqrt` | 2: `math.pi`, `math.e` | 8 |
| `base64` | 2: `base64.encode`, `base64.decode` | 0 | 9 |
| **Total** | **8 entries** | **2 constants** | |

1. Every entry is a function. No group has a method.
2. Every entry and constant is exact: for the same arguments every
   implementation gives the same result or the same failure, and charges the
   same steps and units, on every platform.
3. `suite/library-groups.json` has 10 records: one for each entry and one for
   each constant (3).
4. With this package an implementation offers 61 library entries: the 53 of
   Core and these 8.
5. Package F13 adds eleven entries to the group `math` (8.1). With it an
   implementation offers 72.

### 1.3 What this package changes in Core's rules

| Core rule | With this package |
| --- | --- |
| `03-host.md` 3.3.1, the declarations | A declaration of the groups that are on (2.1) |
| `03-host.md` 3.3.4, `name-clash` | A variable or host function named like a group that is on clashes (2.2) |
| `01-language.md` 4.2, `ns.f(a)` | A group that is on is a namespace of the environment. A `Member` that reads a constant of a group is a constant (2.4) |
| `05-types.md` 22.2, what the static rules read | The groups that are on are namespaces, and their entries are library functions (2.5) |
| `04-library.md` 1.3, the specification table | A second file of records, `suite/library-groups.json`, with a record for a constant and two new symbols of formula text (3) |
| `04-library.md` 1.3, the field `renders` | Present on `json.stringify` too (6.1) |
| `02-evaluation.md` 5.2 and 6.1 | One row: the read of a constant (5.3) |
| `03-host.md` 3.10.3 and 3.10.6 | The case field `groups` and the result field `json` (11) |
| `f01-host-types.md` 5.3, the description | The key `groups` (2.7) |

### 1.4 Version

1. The groups and their entries are part of version `3.0`. Every record has
   `since` `3.0`. This package adds no version number.
2. An environment of version `3.0` may turn any of the four groups on.
3. A later minor version may add a group, or an entry or a constant to a group.
   It never changes an entry, a constant or a group (`04-library.md` 1.2).

### 1.5 Packages this one requires

Packages are offered in a fixed order, each only with every package before it:
F3, F1, F2, F4, F5, F6, F7, F8, F9, F10, F11, F13, F12. So an implementation
that offers F6 offers F1, and the cases of this package may use the forms and the case fields
of F1 to F5. The file `suite/groups/with-f01-description.json` uses F1's
result field `description` (2.7).

## 2. Turning a group on

### 2.1 The declaration

This row is added to the table of `03-host.md` 3.3.1.

| Declaration | States | Portable form |
| --- | --- | --- |
| Groups | The opt-in groups that are on | A list of group names, each `json`, `time`, `math` or `base64` |

1. An environment with no groups declaration has every group off. The suite's
   base environment (`03-host.md` 3.10.9) has every group off.
2. Naming a group once or several times turns it on. The order of the names
   does not matter.
3. A name that is not one of the four is the `declare` error `bad-name`: among
   them `core`, `regexp`, `JSON`, the empty string, and an entry's name such as
   `math.sqrt`.
4. A group that is on adds its namespace to the environment, its entries to the
   library functions, and its constants (2.4). A group that is off adds
   nothing.
5. Turning a group on is the only way to call its entries. A host cannot declare
   a function in a group's namespace, whether the group is on or off: that is
   `reserved-namespace` (`03-host.md` 3.3.4), as in Core.

### 2.2 Names and clashes

These rows are added to the table of `03-host.md` 3.3.4. Each error is reported
when the environment is built, in phase `declare`.

| Situation | Code |
| --- | --- |
| A variable named like a group that is on: a variable `json` with the group `json` on | `name-clash` |
| A host function with no namespace named like a group that is on: a function `math` with `math` on | `name-clash` |
| A group name that is not one of the four (2.1 item 3) | `bad-name` |

1. A variable or a host function named like a group that is off is allowed. So
   a host may keep a variable `time` while the group `time` is off, and a
   program reads it as any variable.
2. A host function named like an entry without its namespace does not clash: a
   function `sqrt` beside the entry `math.sqrt`. The two are different names
   (`03-host.md` 3.3.4).
3. A host namespace beside a group does not clash: `geo.parse` and
   `json.parse`.
4. An opaque type named like a group, such as `Json`, does not clash. A type
   name never clashes with a namespace (`03-host.md` 3.3.4 item 1).
5. A variable named like an entry without its namespace, such as `parse`, does
   not clash.

### 2.3 A group that is off

1. A program that names a group that is off reads the group's name as a plain
   name. It is bound by nothing, so the static rules report `undefined-name`
   on the identifier, in phase `check` (`05-types.md` 22.4).
2. So `json.parse("1")` with `json` off is `undefined-name` at `[1, 1, 1, 5]`,
   the word `json`. The call is a `MethodCall` whose receiver is that name
   (`01-language.md` 4.2). `math.pi` with `math` off is `undefined-name` on
   `math`.
3. Where the environment declares a variable of that name, the program reads
   the variable: `time.parse("x")`, with a variable `time` of type `any`
   supplied as a map, is a method call on the map, and the run fails with
   `wrong-kind`, template `method`.
4. No new code is reported for a group that is off. The text gives the same
   outcome as in Core.

### 2.4 Calls and constants

1. **A call.** In a `MethodCall` with a plain `.` whose receiver is an `Ident`
   that names a group that is on, name resolution makes a `Call` with that
   namespace, as for a host namespace (`01-language.md` 4.2). The callee is the
   entry of that full name. With `?.`, the name is read as a value, which is
   `not-a-value` (`05-types.md` 22.4).
2. **A constant.** A `Member` with a plain `.` whose receiver is an `Ident`
   that names a group that is on, and whose name is a constant of that group,
   is a read of the constant. No node is added: the `Member` is resolved, as a
   `MethodCall` is in item 1. Its span is the span of the `Member`.
3. A `Member` whose receiver names a group that is on and whose name is not a
   constant of the group, as `math.tau` or `math.sqrt`, is not resolved. Its
   receiver is read as an `Ident`, which is `not-a-value` on the identifier, as
   for any namespace (`05-types.md` 22.4).
4. A `MethodCall` whose name is a constant, as `math.pi()`, is a `Call` of a
   function the group lacks: `undefined-function` on `math.pi` (`05-types.md`
   22.5).
5. A namespace is never a value and a group is never bound. A name of a group
   that is on, used as the name of a binding, is `shadows-namespace`
   (`05-types.md` 22.7).
6. A key of a map literal, a field word after `.` on any other receiver, and
   the key of a pattern field are words, not names: `{json: 1}.json` is `1`, and
   `const {math: m} = {math: 2}; m` is `2`. A shorthand entry or a pattern field
   with no other name is a name: `{math}` is `not-a-value` on `math`, and
   `const {math} = ...` is `shadows-namespace`.
7. A call of an entry is a call of a library function, evaluated and charged by
   `02-evaluation.md` 11.3 and `04-library.md` 4.4.

### 2.5 The static rules

`05-types.md` 22.2 reads, with this package:

| Read | From |
| --- | --- |
| The library functions: name and the counts of 22.3 | Each entry of `library.json` that has no `receiver`, and each function record of `library-groups.json` whose group is on, by its full name with its namespace |
| The namespaces | The prefixes of the host functions' names, and the groups that are on |
| The constants | The constant records of `library-groups.json` whose group is on |

1. A `Call` with the namespace of a group that is on, whose name is an entry,
   has that entry as its callee (22.5, first row). With another name it is
   `undefined-function` on the namespace and the name.
2. The count of arguments is tested as for any function called by name: fewer
   than required or more than its parameters is `arity` on the whole `Call`. A
   lambda argument is `not-a-value` on the argument: no entry of a group takes a
   callback.
3. A `Member` that reads a constant (2.4 item 2) gives no diagnostic. Its
   receiver is not walked as an `Ident`.
4. Every other rule of 22 is unchanged. Its spans and its order hold.

| Program | Groups | Static diagnostics |
| --- | --- | --- |
| `json` | `json` | `not-a-value` 1:1-1:5 |
| `json.nothing(1)` | `json` | `undefined-function` 1:1-1:13 |
| `json?.parse("1")` | `json` | `not-a-value` 1:1-1:5 |
| `math.sqrt` | `math` | `not-a-value` 1:1-1:5 |
| `math.pi()` | `math` | `undefined-function` 1:1-1:8 |
| `math.sqrt()` | `math` | `arity` 1:1-1:12 |
| `math.sqrt(x => x)` | `math` | `not-a-value` 1:11-1:17 |
| `const json = 1; json` | `json` | `shadows-namespace` 1:7-1:11; `not-a-value` 1:17-1:21 |
| `json.nothing(math)` | `json`, `math` | `undefined-function` 1:1-1:13; `not-a-value` 1:14-1:18 |
| `[math.pi, time.format(0)]` | `math` | `undefined-name` 1:11-1:15 |

### 2.6 Child environments and removed entries

1. A child environment (`03-host.md` 3.3.2) has the groups of its parent and
   those it declares itself. A child cannot turn off a group its parent turned
   on.
2. **Naming for removal.** An entry or a constant of a group is named for
   removal (`03-host.md` 3.3.1) by its full name, with its namespace:
   `json.parse`, `math.pi`. A group's name alone (`json`), an entry's name
   without its namespace (`parse`) and a name the group does not have
   (`json.nope`) name no entry, and are `bad-name` (`03-host.md` 3.3.4).
3. A removed entry or constant of a group that is on is absent: a call of it is
   `undefined-function` on its full name, and a read of a removed constant is
   `not-a-value` on the namespace (2.4 item 3). The group stays on, with its
   other entries. A host function still may not take the name: the namespace
   stays reserved, so that is `reserved-namespace`.
4. Removing an entry of a group that is off is no error and changes nothing:
   the group stays off.
5. The cases are in `suite/groups/groups-removed.json`, with the field
   `removed` of `03-host.md` 3.10.3.

### 2.7 The description of an environment

For an implementation that offers F1 and this package, the description of an
environment (`f01-host-types.md` 5.3) gains one key.

| Key | Value |
| --- | --- |
| `groups` | An array of the names of the groups that are on, in code point order |

1. The key is present only when at least one group is on. So the description of
   an environment with every group off is the description F1 gives, unchanged.
2. A description with the key builds an environment with those groups on
   (`f01-host-types.md` 5.4). A name in it that is not a group is `bad-name`.

```json
{"functions":{},"groups":["json","math"],"limits":{"allocation":1000000,"depth":200,"steps":1000000},"opaque":{},"reject":["constant-condition"],"removed":[],"variables":{},"version":"3.0"}
```

## 3. The records: `suite/library-groups.json`

### 3.1 The file

1. `suite/library-groups.json` is a JSON array of 10 records: the 8 entries
   and the 2 constants of 1.2, in the order of that table, each group's
   constants after its functions.
2. It is a part of the specification table of `04-library.md` 1.3, kept in its
   own file. `suite/library.json` holds the 53 records of the default group and
   is not changed. An implementation that offers F6 loads both files. An
   implementation that does not offer F6 loads `library.json` only.
3. No name is in both files. The loader's rules of `04-library.md` 1.3 apply to
   both files together: every `template` a `fails` line names is a template of
   its code, and each of the 14 templates of `invalid-argument` is named by some
   `fails` line of one of the two files.

### 3.2 A function's record

A function's record has the fields of `04-library.md` 1.3, with these values.

| Field | Value |
| --- | --- |
| `name` | The full name, with its namespace: `json.parse` |
| `receiver` | Absent: every entry is a function |
| `signature` | The signature in type text |
| `group` | The group: `json`, `time`, `math` or `base64`. It is the namespace of `name` |
| `since` | `3.0` |
| `exact` | `true` |
| `calls` | `[]` |
| `renders` | `["x"]` on `json.stringify`; absent on the others |
| `steps`, `units` | The formulas, in the text of `04-library.md` 4.3 with the symbols of 3.4 |
| `null` | `null`: no entry gives null |
| `fails`, `wrong`, `cases` | As in `04-library.md` 1.3 |

### 3.3 A constant's record

| Field | Value |
| --- | --- |
| `name` | The full name: `math.pi` |
| `type` | The type in type text: `float` |
| `value` | The constant, as a JSON number. It is read by the rule of text to number (`01-language.md` 6.3) |
| `group` | `math` |
| `since`, `exact`, `doc` | As for a function |
| `steps`, `units` | `"0"` and `"0"`: what a read of the constant charges |
| `null` | `null` |
| `fails` | `[]` |
| `wrong`, `cases` | As for a function |

1. A constant's record has no `signature`, `receiver`, `calls`, `renders`,
   `narrows` or `tested`. A loader tells a constant from a function by the field
   `type`. A record with both `type` and `signature`, or neither, is rejected.
2. The value is written in the shortest form that reads back as the same
   double: `3.141592653589793` and `2.718281828459045`.

### 3.4 Symbols of formula text

Two symbols are added to `04-library.md` 4.2 and to `Symbol` in 4.3. Only
`json.parse` uses them.

| Symbol | Meaning |
| --- | --- |
| `c` | The number of values that the JSON text describes: each `value` of the grammar of 6.2.1, at every depth, the top one included. Keys are not values |
| `u` | The units of the values that the JSON text describes (6.2.6) |

The other symbols keep their meaning. `sz(result)` is the size of the string an
entry returns, and `walk(x)` is the rendering walk of `04-library.md` 4.2.

### 3.5 The cases of a record

1. The `cases` of a record are a case file with no header, as in
   `library.json`.
2. Every case of a record states `groups`, and its list holds the record's
   group. So each case runs as it stands, in any runner that offers F6.
3. A runner runs the cases of `library-groups.json` as it runs those of
   `library.json` (`03-host.md` 3.10.8 item 5).

## 4. Shared rules

### 4.1 UTF-8

1. The **UTF-8 encoding** of a string is the sequence of its code points, each
   encoded by the table below. The language's strings hold Unicode scalar
   values only, so no surrogate code point is encoded.

| Code point | Bytes |
| --- | --- |
| U+0000 to U+007F | 1 |
| U+0080 to U+07FF | 2 |
| U+0800 to U+FFFF | 3 |
| U+10000 to U+10FFFF | 4 |

2. A sequence of bytes is **well-formed UTF-8** when it is a concatenation of
   the well-formed byte sequences of Unicode 16.0.0 table 3-7. So an overlong
   form, an encoded surrogate (`ED A0 80`), a code point above U+10FFFF, a lone
   continuation byte and a sequence cut short are not well-formed.
3. A byte order mark is the code point U+FEFF like any other. No entry adds,
   removes or skips one.

### 4.2 Failures

| Failure | Code | Template | Parameters | Entries |
| --- | --- | --- | --- | --- |
| A value the entry cannot use | `invalid-argument` | `not-real` | `name`: the entry's full name, `math.sqrt` | `math.sqrt` |
| | `invalid-argument` | `not-a-time` | `text`: the argument | `time.parse` |
| | `invalid-argument` | `year-range` | `t`: the argument | `time.format` |
| | `invalid-argument` | `not-a-duration` | `text`: the argument | `time.duration` |
| | `invalid-argument` | `not-json` | None | `json.parse` |
| | `invalid-argument` | `integer-range` | None | `json.parse` |
| | `invalid-argument` | `not-base64` | None | `base64.decode` |
| | `invalid-argument` | `not-utf8` | None | `base64.decode` |
| A number result that is not finite | `number-range` | `entry` | `name`: the entry's full name | `json.parse`, `time.duration` |
| An opaque value written as JSON | `wrong-kind` | `json` | None | `json.stringify` |
| An argument of a kind its parameter does not take, null included | `wrong-kind` | `argument` | `name`: the entry's full name; `kind` | Every entry but `json.stringify` |
| A walk past the depth limit | `depth-exceeded` | | | `json.stringify`, `json.parse` |

1. The templates are those of Core's catalog, `suite/messages.json`. This
   package adds none and changes none.
2. Every failure of an entry has the span of the whole `Call`, from the
   namespace to the `)` (`04-library.md` 2.6 item 6).
3. A `not-json` message names no position in the text, and no other field of
   the diagnostic does. Its template has no parameter.
4. `catch` catches every failure of this table but `depth-exceeded`
   (`02-evaluation.md` 12.2).
5. No entry gives NaN, an infinity or negative zero. A result that would be
   negative zero is zero.

### 4.3 Tests of the arguments

The tests of `04-library.md` 2.6 apply to every entry, before the call's step.
Every parameter but the `x` of `json.stringify` names a kind, so a null argument
fails with `wrong-kind`, template `argument`. `json.stringify` takes every
value. No parameter is of type `int`, so the template `whole` is never raised.

### 4.4 Determinism and the clock

1. No entry reads anything but its arguments: no clock, random source, locale,
   time zone, process environment or file (`04-library.md` 3.12). Each group is
   as deterministic as the default group.
2. **The current time is data.** No group has a function that gives the
   current time. A host that wants a program to know the time declares a
   variable, such as `now: float`, and supplies the time as a number of seconds
   (7.1). A run is then deterministic for the value supplied, and a case states
   that value in `vars`.
3. A program in a group never depends on the platform: `math.sqrt` is
   correctly rounded (8.3), and so is every entry F13 adds to `math`, so that
   their results are the same doubles in every implementation.

## 5. Cost

### 5.1 The formulas

| Entry | Steps | Units |
| --- | --- | --- |
| `json.stringify` | `walk(x) + sz(result)` | `1 + sz(result)` |
| `json.parse` | `8*sz(s) + c` | `u` |
| `time.parse` | `sz(s)` | `0` |
| `time.format` | `sz(result)` | `1 + sz(result)` |
| `time.duration` | `sz(s)` | `0` |
| `math.sqrt` | `0` | `0` |
| `base64.encode` | `sz(s) + sz(result)` | `1 + sz(result)` |
| `base64.decode` | `sz(s)` | `1 + sz(result)` |

1. A formula is what the entry charges in addition to the call's 1 step
   (`04-library.md` 4.1).
2. The order of each entry's charges is in its section, under "Order". Each
   order is one of the rules of `04-library.md` 4.4 or is stated there in full.
3. Each entry does work of at most a constant times what it charges
   (`02-evaluation.md` 8.5). Where a platform's function does not meet this, an
   implementation writes its own.
4. A **failure keeps what was charged.** Where an entry charges steps for its
   text before it reads the text (`json.parse`, `time.parse`, `time.duration`,
   `base64.decode`), a failure found in the text comes after that charge.

### 5.2 Order of a call

As for every library call (`04-library.md` 4.4): the arguments are evaluated,
the tests of 2.6 are made, and the call's 1 step is charged. Then each entry
goes on by its "Order".

### 5.3 The read of a constant

This row is added to the tables of `02-evaluation.md` 5.2 and 6.1.

| Form | Steps | Units |
| --- | --- | --- |
| The read of a constant of a group, `math.pi` | 0 | 0. It is a number |

## 6. `json`

### 6.1 `json.stringify`

| Field | Value |
| --- | --- |
| Signature | `json.stringify: (x: any) => string` |
| Group | `json` |
| Steps | `walk(x) + sz(result)` |
| Units | `1 + sz(result)` |
| Null | Never |
| Fails | `wrong-kind`, template `json`: x is an opaque value or holds one at any depth; `depth-exceeded`: the walk passes the depth limit |
| Order | Tests; the rendering walk, which charges as it goes; steps `sz(result)`; units; build. |
| Renders | `x` (`04-library.md` 1.3) |

1. The result is the JSON form of `x` (`01-language.md` 12): no white space,
   the entries of every map in key order by code point, each string escaped as
   that section lists and in no other way, each number in its rendering
   (`01-language.md` 6.3). So `/`, `<`, `&`, U+007F and U+2028 are written as
   themselves, a code point above U+FFFF is written as itself, and a control
   character other than the five with short escapes is `\u00` and two
   lower-case hex digits.
2. It is the JSON text of `03-host.md` 3.2.4, made in the run: for the same
   value the two give the same text. They differ only in where a walk stops:
   3.2.4 stops at the default depth limit, and this entry at the run's depth
   count (`02-evaluation.md` 8.4).
3. It differs from `string(x)` for a string, which it quotes and escapes, and
   for null, which it writes as `null`. For a list, a map, a number and a bool
   the two give the same text, the same steps and the same units.
4. The walk is the rendering walk of `02-evaluation.md` 5.5, as `string` makes
   it (`04-library.md` 4.4): on entering each list or map it tests the depth
   limit, then charges `m` for a list of `m` elements, and `m + L(m) × (m +
   sz(its keys))` for a map of `m` entries. A number, a bool, a string or null
   is not walked. `walk(x)` is the total.
5. An opaque value, at any depth, fails with `wrong-kind`, template `json`,
   where the walk meets it. What the walk charged before stays charged.
6. There is no second argument: no indentation and no other key order. A later
   minor version may add an optional last parameter (`04-library.md` 1.2).

| Program | Data | Result |
| --- | --- | --- |
| `json.stringify(null)` |  | `"null"` |
| `json.stringify("a\"b")` |  | `"\"a\\\"b\""` |
| `json.stringify([1, "a", null, true])` |  | `"[1,\"a\",null,true]"` |
| `json.stringify({b: 1, a: [true]})` |  | `"{\"a\":[true],\"b\":1}"` |
| `json.stringify(x)` | `{x: float[]}`, x = `[1e21, 1.5e-7, 100, 0.30000000000000004]` | `"[1e+21,1.5e-7,100,0.30000000000000004]"` |
| `json.stringify(m)` | `{m: map<int>}`, m = `{"😀": 2, "～": 1}` | `"{\"～\":1,\"😀\":2}"`: U+FF5E is before U+1F600 |
| `json.stringify(s)` | `{s: string}`, s = `"abc"` | `"\"abc\""`; 1 step, 1 unit |
| `json.stringify(x)` | `{x: any}`, x = `{b: [1, 2], a: "x"}` | `"{\"a\":\"x\",\"b\":[1,2]}"`; 7 steps, 1 unit |
| `json.stringify(s)` | `{s: string}`, s = `"a" × 70` | `"a" × 70` quoted, weight 72; 2 steps, 2 units |
| `json.stringify(t)` | `{t: any}`, t an opaque value | Fails: `wrong-kind`, template `json`; 1 step, 0 units |
| `json.stringify(x)` | `{x: any}`, x = `[[[1]]]`, depth limit 2 | Fails: `depth-exceeded` |
| `[string("x"), json.stringify("x"), string(null), json.stringify(null)]` |  | `["x", "\"x\"", "", "null"]` |

### 6.2 `json.parse`

| Field | Value |
| --- | --- |
| Signature | `json.parse: (s: string) => any !` |
| Group | `json` |
| Steps | `8*sz(s) + c` |
| Units | `u` |
| Null | When the text is the JSON value `null` |
| Fails | `invalid-argument`, form `not-json`: s is not a JSON text by 6.2.1; `invalid-argument`, form `integer-range`: a number with no fraction and no exponent is not a whole number; `number-range`, template `entry`: a number with a fraction or an exponent reads as an infinity; `depth-exceeded`: the nesting of the text passes the depth limit |
| Order | Tests; steps `8*sz(s)`; the pass of 6.2.4; steps `c`; units `u`; build. |

#### 6.2.1 The grammar

A JSON text is the text of RFC 8259, section 2, and nothing more. In this
grammar a quoted character is the code point itself, and the grammar reads code
points, not bytes.

```ebnf
Text    = WS Value WS .
Value   = Object | Array | String | Number | "true" | "false" | "null" .
Object  = "{" WS [ Member { WS "," WS Member } WS ] "}" .
Member  = String WS ":" WS Value .
Array   = "[" WS [ Value { WS "," WS Value } WS ] "]" .
String  = '"' { Char } '"' .
Char    = any code point but '"', "\" and U+0000 to U+001F
        | "\" ( '"' | "\" | "/" | "b" | "f" | "n" | "r" | "t" | Unicode ) .
Unicode = "u" Hex Hex Hex Hex .
Number  = [ "-" ] Int [ Frac ] [ Exp ] .
Int     = "0" | Digit19 { Digit } .
Frac    = "." Digit { Digit } .
Exp     = ( "e" | "E" ) [ "+" | "-" ] Digit { Digit } .
WS      = { U+0020 | U+0009 | U+000A | U+000D } .
```

`Digit` is `0` to `9`, `Digit19` is `1` to `9`, and `Hex` is `0` to `9`, `a`
to `f` and `A` to `F`.

1. The whole of `s` must be one `Text`. Anything after the value but white space
   is not JSON.
2. White space is exactly the four code points of `WS`. U+00A0, U+FEFF, U+2028
   and every other code point are not white space. A byte order mark at the
   start is not JSON.
3. The value at the top may be of any kind: `42`, `"x"` and `null` are JSON
   texts.
4. Not JSON, among others: an empty text; a comment; a trailing comma; a key
   that is not a string; a single quote; a leading zero, `01`; `+1`; `.5` and
   `1.`; `NaN` and `Infinity`; `True`; a raw control character in a string,
   such as a tab or a line end; an escape not in `Char`, such as `\x41` or `\'`;
   a `\u` with fewer than four hex digits; an open string, list or object.

#### 6.2.2 The value

| Text | Value |
| --- | --- |
| `Object` | A map. Its keys are the members' keys. Where two members have equal keys, the later one's value is the value under that key |
| `Array` | A list of the values, in order |
| `String` | A string: the code points of its characters, each escape replaced by 6.2.3 |
| `Number` | A number, by 6.2.3 |
| `true`, `false`, `null` | `true`, `false`, null |

1. Two keys are equal when their code points are equal after escapes are
   replaced: in `{"a": 1, "\u0061": 2}` the second key replaces the first.
2. A map has no order of its own (`01-language.md` 6.1).

#### 6.2.3 Escapes and numbers

| Escape | Code point |
| --- | --- |
| `\"`, `\\`, `\/` | `"`, `\`, `/` |
| `\b`, `\f`, `\n`, `\r`, `\t` | U+0008, U+000C, U+000A, U+000D, U+0009 |
| `\u` with a value from U+D800 to U+DBFF, directly followed by a `\u` with a value from U+DC00 to U+DFFF | The one code point the pair encodes in UTF-16 |
| `\u` with a value from U+D800 to U+DFFF, in any other place | U+FFFD |
| `\u` with any other value | That code point |

1. So `"\uD83D\uDE00"` is `"😀"`, `"\uD800x"` is U+FFFD then `x`,
   `"\uDC00\uD800"` is two U+FFFD, and `"\uD800\uD800\uDC00"` is U+FFFD then
   U+10000.
2. A **number in integer form** has no `Frac` and no `Exp`. It must be a whole
   number (`04-library.md` 3.10), tested on its digits: the integer its digits
   name, without the `-`, must be at most 9007199254740991. `-0` is zero. A
   number in integer form that is not whole fails with `integer-range`; it is
   not rounded. So `9007199254740992` fails, and `9007199254740991` is read.
3. **Any other number** is read by the rule of text to number (`01-language.md`
   6.3): the nearest double, ties to even, exactly for any count of digits. One
   that reads as an infinity fails with `number-range`, template `entry`, name
   `json.parse`. One too small to represent is zero. So `9007199254740993.0` is
   `9007199254740992`, `1e20` is `1e20`, and `1e-400` is zero.
4. Negative zero is zero: `-0`, `-0.0` and `-1e-400` all give zero.

#### 6.2.4 The pass and the order of failures

After the charge `8*sz(s)`, the entry reads the whole text in one pass that
creates no value. The first rule below that the text breaks gives the failure:

1. The text is not a `Text` of 6.2.1: `invalid-argument`, form `not-json`.
2. Some number breaks 6.2.3 items 2 or 3. The first such number in the text
   decides: in integer form, `invalid-argument`, form `integer-range`; otherwise
   `number-range`, template `entry`.
3. The nesting of the text passes the depth limit (6.2.5): `depth-exceeded`.

So a text that is not JSON is `not-json` whatever else is wrong with it, and
`[9007199254740993, 1e400]` is `integer-range` while `[1e400,
9007199254740993]` is `number-range`. The pass also finds `c` and `u`. After it
the entry charges `c` steps, then `u` units, then builds the value.

#### 6.2.5 Depth

1. An `Array` or `Object` at the top of the text is at level 1. One directly
   inside a value at level `n` is at level `n + 1`. Let `d` be the greatest
   level in the text, or 0 when it has no `Array` and no `Object`.
2. The text passes the depth limit when the depth count at the call
   (`02-evaluation.md` 8.4) plus `d` is more than the depth limit. The count at
   the call is that of the calls in progress: `json.parse` calls no lambda, so
   it adds nothing of its own, as `string` does not.
3. So with the default limit of 200, a text nested 200 deep parses at the top of
   a program, and inside the callback of a list method called at the top it
   fails, since that call adds 1.
4. The parse itself never uses the host's stack in proportion to `d`
   (`03-host.md` 3.9.1). Rule 1 of 6.2.4 is decided over the whole text, at any
   nesting.

#### 6.2.6 The counts

1. `c` is the number of `Value`s in the text, at every depth, the top one
   included. A value whose key a later member replaces is counted.
2. `u` is the sum, over the values of the text, of: for an `Array` of `m`
   values, `1 + m`; for an `Object`, `1 + 2 × k`, where `k` is the number of
   distinct keys after replacement; for a `String`, `1 + sz` of its value. A
   number, `true`, `false` and `null` add 0. A key adds nothing, as a key of a
   map literal does not (`02-evaluation.md` 6.1). A value that a later member
   replaces is counted, as a replaced entry of a map literal was built and
   charged.
3. The term `8*sz(s)` pays for reading the text, keys and escapes included, and
   for probing each map by its keys (`02-evaluation.md` 5.3 item 5).

| Program | Data | Result |
| --- | --- | --- |
| `json.parse("{\"b\": [1, 2.5, -0], \"a\": null}")` |  | `{a: null, b: [1, 2.5, 0]}` |
| `json.parse(" 42\n")` |  | `42` |
| `json.parse("{\"a\": 1, \"a\": 2}")` |  | `{a: 2}` |
| `json.parse("{\"a\": 1, \"\\u0061\": 2}")` |  | `{a: 2}` |
| `json.parse("\"\\uD83D\\uDE00\"")` |  | `"😀"` |
| `json.parse("\"\\uD800x\"")` |  | `"\u{fffd}x"` |
| `json.parse("1e-400")` |  | `0` |
| `json.parse("[-0, -0.0, -1e-400]")` |  | `[0, 0, 0]` |
| `json.parse("[9007199254740991, -9007199254740991]")` |  | `[9007199254740991, -9007199254740991]` |
| `json.parse("9007199254740992")` |  | Fails: `invalid-argument`, form `integer-range` |
| `json.parse("9007199254740993.0")` |  | `9007199254740992` |
| `json.parse("1e20")` |  | `100000000000000000000` |
| `json.parse("1e400")` |  | Fails: `number-range` |
| `json.parse("[9007199254740993, 1e400]")` |  | Fails: `invalid-argument`, form `integer-range` |
| `json.parse("[9007199254740993, x]")` |  | Fails: `invalid-argument`, form `not-json` |
| `json.parse("")` |  | Fails: `invalid-argument`, form `not-json` |
| `json.parse("\u{feff}1")` |  | Fails: `invalid-argument`, form `not-json` |
| `json.parse("[1,]")` |  | Fails: `invalid-argument`, form `not-json` |
| `json.parse("'a'")` |  | Fails: `invalid-argument`, form `not-json` |
| `json.parse("01")` |  | Fails: `invalid-argument`, form `not-json` |
| `json.parse("\"a\tb\"")` |  | Fails: `invalid-argument`, form `not-json`: a raw tab in a string |
| `json.parse("[[[[1]]]]")` | depth limit 3 | Fails: `depth-exceeded` |
| `json.parse("[[[[1]]]")` | depth limit 3 | Fails: `invalid-argument`, form `not-json` |
| `json.parse(s)` | `{s: string}`, s = `{"a":[1,2],"b":"x"}` | `{a: [1, 2], b: "x"}`; 6 steps, 9 units |
| `json.parse(s)` | `{s: string}`, s = `{"a":[1],"a":2}` | `{a: 2}`; 5 steps, 5 units |
| `json.parse(s)` | `{s: string}`, s = a JSON string of 70 `a` | `"a" × 70`; 10 steps, 2 units |
| `json.parse(s)` | `{s: string}`, s = 70 spaces then `x` | Fails: `invalid-argument`, form `not-json`; 9 steps, 0 units |
| `json.parse(x)` | `{x: any}`, x = `5` | Fails: `wrong-kind`, template `argument`; 0 steps, 0 units |

## 7. `time`

### 7.1 Times and the calendar

1. A **time** is a number: the seconds since 1970-01-01T00:00:00Z, the Unix
   epoch, with no leap seconds. A time before the epoch is negative. A
   **duration** is a number of seconds.
2. Times and durations are compared and combined with the language's
   operators: `time.parse(a) < now - time.duration("7d")`.
3. The **calendar** is the proleptic Gregorian calendar, with years written as
   four digits from 0000 to 9999. Year 0000 is the year before 0001 and is a
   leap year. A year is a leap year when it is divisible by 4 and not by 100,
   or divisible by 400.
4. Every day has 86,400 seconds. The day `Y-M-D` begins at the time `86400 ×
   days(Y, M, D)`, where `days` counts the days from 1970-01-01 to that date in
   this calendar, negative before it.
5. The times of years 0000 to 9999 in UTC are those from `-62167219200`
   (0000-01-01T00:00:00Z) up to but not including `253402300800`
   (10000-01-01T00:00:00Z).

### 7.2 `time.parse`

| Field | Value |
| --- | --- |
| Signature | `time.parse: (s: string) => float !` |
| Group | `time` |
| Steps | `sz(s)` |
| Units | `0` |
| Null | Never |
| Fails | `invalid-argument`, form `not-a-time`: s is not of the accepted form, or a field is out of range |
| Order | Tests; steps; read the text; result. |

The accepted form, a profile of the `date-time` of RFC 3339 section 5.6:

```ebnf
DateTime = Year "-" Month "-" Day "T" Hour ":" Minute ":" Second
           [ "." Digit { Digit } ] Offset .
Offset   = "Z" | ( "+" | "-" ) Hour ":" Minute .
Year     = Digit Digit Digit Digit .
Month = Day = Hour = Minute = Second = Digit Digit .
```

`Digit` is `0` to `9`: ASCII digits only.

| Field | Range |
| --- | --- |
| Month | 01 to 12 |
| Day | 01 to the length of the month in that year: 28 or 29 for 02, 30 for 04, 06, 09 and 11, 31 for the others |
| Hour | 00 to 23, in the time and in the offset |
| Minute | 00 to 59, in the time and in the offset |
| Second | 00 to 59. A leap second, 60, is not accepted |

1. The text must be exactly a `DateTime`: no white space anywhere, `T` and `Z`
   upper-case, the colon in the offset, four digits of year.
2. The fraction has one digit or more, any number of them.
3. The offset `-00:00` is accepted and means zero.
4. **The value.** Let `W` be the whole seconds `86400 × days(Year, Month, Day) +
   3600 × Hour + 60 × Minute + Second`, less the offset in seconds (`+` east of
   UTC). Let `F` be `0.` followed by the fraction's digits, or zero. The result
   is the double nearest to the exact value `W + F`, ties to even. `W` is a
   whole number, so a text with no fraction gives a whole number exactly.
5. An offset may move the time out of years 0000 to 9999 in UTC:
   `0000-01-01T00:00:00+23:59` is accepted. `time.format` of that time fails.
6. A text not of this form, or with a field out of its range, fails with form
   `not-a-time`, whose parameter `text` is `s`.

| Program | Data | Result |
| --- | --- | --- |
| `time.parse("1970-01-01T00:00:00Z")` |  | `0` |
| `time.parse("2024-02-29T12:30:45Z")` |  | `1709209845` |
| `time.parse("2024-02-29T12:30:45.5+01:00")` |  | `1709206245.5` |
| `time.parse("2000-01-01T00:00:00.123456789Z")` |  | `946684800.1234568` |
| `time.parse("1969-12-31T23:59:59.9Z")` |  | `-0.1` |
| `time.parse("0000-01-01T00:00:00Z")` |  | `-62167219200` |
| `time.parse("2023-02-29T00:00:00Z")` |  | Fails: `invalid-argument`, form `not-a-time` |
| `time.parse("2024-12-31T23:59:60Z")` |  | Fails: `invalid-argument`, form `not-a-time` |
| `time.parse("2024-01-01t00:00:00z")` |  | Fails: `invalid-argument`, form `not-a-time` |
| `time.parse("2024-01-01T00:00:00")` |  | Fails: `invalid-argument`, form `not-a-time` |
| `time.parse("2024-01-01")` |  | Fails: `invalid-argument`, form `not-a-time` |

### 7.3 `time.format`

| Field | Value |
| --- | --- |
| Signature | `time.format: (t: float) => string !` |
| Group | `time` |
| Steps | `sz(result)` |
| Units | `1 + sz(result)` |
| Null | Never |
| Fails | `invalid-argument`, form `year-range`: the year of t in UTC is outside 0000 to 9999 |
| Order | Tests; `t` tested; find the result's weight; steps; units; build. |

1. Let `W` be the greatest whole number not above `t`. If `W` is not a time of
   years 0000 to 9999 (7.1 item 5), the call fails with form `year-range`,
   parameter `t`. This test comes before any charge of the entry.
2. The text is `YYYY-MM-DDTHH:MM:SS` for the second `W` in UTC, each field with
   its leading zeros.
3. If `t` is not whole, then `.` and the fraction digits follow. Let `V` be the
   exact value of the number-to-text form of `t` (`01-language.md` 6.3). The
   fraction digits are the digits of `V − W`, written with as many digits after
   the point as `V` has. So `0.5` gives `.5`, `-0.25` gives `.75` of the second
   before, and `1e-7` gives `.0000001`. The last digit is never zero.
4. Then `Z`.
5. **Round trip.** `time.parse(time.format(t))` is `t` for every `t` of years
   0000 to 9999, because the text names `V` exactly and `V` reads back as `t`.
   And `time.format(time.parse(s))` names the same instant as `s`, in UTC.
6. The result has weight 20 when `t` is whole. It is longer only when `t` has a
   long fraction, as a tiny `t` has.

| Program | Data | Result |
| --- | --- | --- |
| `time.format(t)` | `{t: float}`, t = `0` | `"1970-01-01T00:00:00Z"`; 1 step, 1 unit |
| `time.format(t)` | `{t: float}`, t = `1709209845` | `"2024-02-29T12:30:45Z"` |
| `time.format(t)` | `{t: float}`, t = `0.5` | `"1970-01-01T00:00:00.5Z"` |
| `time.format(t)` | `{t: float}`, t = `-0.25` | `"1969-12-31T23:59:59.75Z"` |
| `time.format(t)` | `{t: float}`, t = `-0.1` | `"1969-12-31T23:59:59.9Z"` |
| `time.format(t)` | `{t: float}`, t = `1e-7` | `"1970-01-01T00:00:00.0000001Z"` |
| `time.format(t)` | `{t: float}`, t = `-62167219200` | `"0000-01-01T00:00:00Z"` |
| `time.format(t)` | `{t: float}`, t = `253402300799.5` | `"9999-12-31T23:59:59.5Z"` |
| `time.format(t)` | `{t: float}`, t = `1e-70` | `"1970-01-01T00:00:00."`, 69 zeros, `"1Z"`: weight 91; 2 steps, 2 units |
| `time.format(t)` | `{t: float}`, t = `253402300800` | Fails: `invalid-argument`, form `year-range`; 1 step, 0 units |
| `time.format(t)` | `{t: float}`, t = `-62167219200.5` | Fails: `invalid-argument`, form `year-range` |

### 7.4 `time.duration`

| Field | Value |
| --- | --- |
| Signature | `time.duration: (s: string) => float !` |
| Group | `time` |
| Steps | `sz(s)` |
| Units | `0` |
| Null | Never |
| Fails | `invalid-argument`, form `not-a-duration`: s is not a duration text; `number-range`, template `entry`: the duration is too large for a number |
| Order | Tests; steps; read the text; result, or `number-range`. |

```ebnf
Duration = [ "-" ] Term { Term } .
Term     = Digit { Digit } [ "." Digit { Digit } ] Unit .
Unit     = "ms" | "s" | "m" | "h" | "d" .
```

| Unit | Seconds |
| --- | --- |
| `ms` | 1/1000 |
| `s` | 1 |
| `m` | 60 |
| `h` | 3,600 |
| `d` | 86,400 |

1. The text must be exactly a `Duration`: no white space, no sign but one `-`
   at the start, ASCII digits, lower-case units. A term needs digits before its
   unit, and digits on both sides of a point. `ms` is read as one unit, since a
   term cannot begin with `s`.
2. Terms may come in any order, and a unit may repeat: `30m1h` and `1h1h` are
   accepted. Leading zeros are accepted.
3. The value is the exact sum of each term's number times its unit, negated if
   the text starts with `-`. The result is the double nearest to it, ties to
   even: one rounding, of the sum. A result of zero is zero.
4. A sum whose nearest double is not finite fails with `number-range`, template
   `entry`, name `time.duration`.
5. No other unit is accepted: not `us`, `ns`, `w` or `y`.

| Program | Data | Result |
| --- | --- | --- |
| `time.duration("90s")` |  | `90` |
| `time.duration("1h30m")` |  | `5400` |
| `time.duration("30m1h")` |  | `5400` |
| `time.duration("1.5h")` |  | `5400` |
| `time.duration("250ms")` |  | `0.25` |
| `time.duration("-1h")` |  | `-3600` |
| `time.duration("1m1ms")` |  | `60.001` |
| `time.duration("-0s")` |  | `0` |
| `time.duration("0.1s0.2s")` |  | `0.3`: the exact sum 0.3, rounded once |
| `time.duration("1")` |  | Fails: `invalid-argument`, form `not-a-duration` |
| `time.duration("1 h")` |  | Fails: `invalid-argument`, form `not-a-duration` |
| `time.duration("1us")` |  | Fails: `invalid-argument`, form `not-a-duration` |
| `time.duration(s)` | `{s: string}`, s = `1`, 400 × `0`, `d` | Fails: `number-range`; 7 steps, 0 units |

## 8. `math`

### 8.1 What the group holds in this package

1. In this package the group `math` holds one entry, `math.sqrt`, and the two
   constants `math.pi` and `math.e`.
2. Package F13, math functions (`spec/packages/f13-math.md`), adds eleven
   entries to the group: `math.pow`, `math.exp`, `math.log`, `math.log10`,
   `math.sin`, `math.cos`, `math.tan`, `math.asin`, `math.acos`, `math.atan` and
   `math.atan2`. Every entry of the group, `math.sqrt` and those eleven, is
   correctly rounded: of the exact mathematical value, the nearest binary64
   value, ties to even.
3. **Without F13.** In an implementation that offers this package and not F13,
   each of the eleven is a name the group does not have. With `math` on,
   `math.sin(x)` is `undefined-function` on `math.sin`, in phase `check`
   (2.5 item 1), and the removal of `math.sin` is `bad-name` (2.6). These cases
   stand in `suite/groups/until-f13-math-functions.json`, which F13 removes.

### 8.2 The constants

| Constant | Type | Value | The double nearest to |
| --- | --- | --- | --- |
| `math.pi` | `float` | `3.141592653589793` | π |
| `math.e` | `float` | `2.718281828459045` | e |

1. A read of a constant charges 0 steps and 0 units (5.3).

| Program | Data | Result |
| --- | --- | --- |
| `math.pi` |  | `3.141592653589793`; 0 steps, 0 units |
| `math.e` |  | `2.718281828459045`; 0 steps, 0 units |

### 8.3 `math.sqrt`

| Field | Value |
| --- | --- |
| Signature | `math.sqrt: (x: float) => float !` |
| Group | `math` |
| Steps, units | `0`, `0` |
| Null | Never |
| Fails | `invalid-argument`, form `not-real`, parameter `name` `math.sqrt`: x is below zero |
| Order | Tests; result, or the failure. |

1. The square root of `x`, correctly rounded. The square root of IEEE 754
   gives it on every platform.
2. A value below zero fails with form `not-real`. The square root of zero is
   zero.
3. It is marked `!`, so a call records a `value` site (10).

| Program | Data | Result |
| --- | --- | --- |
| `math.sqrt(2)` |  | `1.4142135623730951` |
| `math.sqrt(5e-324)` |  | `2.2227587494850775e-162` |
| `math.sqrt(x)` | `{x: float}`, x = `-1` | Fails: `invalid-argument`, form `not-real` |

## 9. `base64`

### 9.1 `base64.encode`

| Field | Value |
| --- | --- |
| Signature | `base64.encode: (s: string) => string` |
| Group | `base64` |
| Steps | `sz(s) + sz(result)` |
| Units | `1 + sz(result)` |
| Null | Never |
| Fails | Only as 4.3 says |
| Order | Tests; find the result's weight; steps; units; build. |

1. The result is the base64 encoding of RFC 4648 section 4, with the alphabet
   of its table 1 (`A` to `Z`, `a` to `z`, `0` to `9`, `+`, `/`) and padding
   `=`, of the UTF-8 encoding of `s` (4.1).
2. With `b` bytes, the result has `4 × ⌈b / 3⌉` code points. It is found from
   the code points of `s` before anything is charged.
3. No line breaks are written. The URL alphabet is not written.

| Program | Data | Result |
| --- | --- | --- |
| `base64.encode("")` |  | `""` |
| `base64.encode("a")` |  | `"YQ=="` |
| `base64.encode("abc")` |  | `"YWJj"` |
| `base64.encode("é")` |  | `"w6k="` |
| `base64.encode("😀")` |  | `"8J+YgA=="` |
| `base64.encode("????")` |  | `"Pz8/Pw=="` |
| `base64.encode(s)` | `{s: string}`, s = `"a" × 48` | `"YWFh" × 16`; 2 steps, 2 units |

### 9.2 `base64.decode`

| Field | Value |
| --- | --- |
| Signature | `base64.decode: (s: string) => string !` |
| Group | `base64` |
| Steps | `sz(s)` |
| Units | `1 + sz(result)` |
| Null | Never |
| Fails | `invalid-argument`, form `not-base64`: s is not base64 text of one alphabet with valid padding; `invalid-argument`, form `not-utf8`: the decoded bytes are not well-formed UTF-8 |
| Order | Tests; steps; read the text; decode; test the bytes; units; build. |

1. **The characters.** Let `p` be the number of `=` at the end of `s`, and `t`
   the text before them. Every code point of `t` must be in the alphabet of RFC
   4648 section 4 (table 1) or of section 5 (table 2, which has `-` and `_` in
   place of `+` and `/`). `t` must not hold both one of `+` and `/` and one of
   `-` and `_`. There is no `=` in `t`. No white space and no line break is
   accepted anywhere.
2. **The length.** Let `n` be the number of code points of `t`. `n mod 4` must
   not be 1. `p` must be 0, or exactly `4 − (n mod 4)` when `n mod 4` is 2 or 3.
   So padding may be present or absent, and when present it is complete.
3. **The bits.** Each group of 4 characters gives 3 bytes. A last group of 2
   characters gives 1 byte, and its last character's low 4 bits must be zero. A
   last group of 3 gives 2 bytes, and its last character's low 2 bits must be
   zero. So each byte string has one text in each alphabet.
4. A text that breaks items 1 to 3 fails with form `not-base64`. The empty text
   gives the empty string.
5. **The text.** The bytes must be well-formed UTF-8 (4.1). If they are not, the
   call fails with form `not-utf8`. This test comes only for a text that is
   base64. The result is the code points they encode, a byte order mark
   included.
6. The language has no value of bytes, so bytes that are not UTF-8 cannot be
   given back. A host that needs them registers a function of its own.

| Program | Data | Result |
| --- | --- | --- |
| `base64.decode("YQ==")` |  | `"a"` |
| `base64.decode("YQ")` |  | `"a"` |
| `base64.decode("8J+YgA==")` |  | `"😀"` |
| `base64.decode("8J-YgA")` |  | `"😀"` |
| `base64.decode("77u/YQ==")` |  | `"\u{feff}a"` |
| `base64.decode("YQ=")` |  | Fails: `invalid-argument`, form `not-base64` |
| `base64.decode("Pj4+Pz8_")` |  | Fails: `invalid-argument`, form `not-base64`: two alphabets |
| `base64.decode("YQ==\n")` |  | Fails: `invalid-argument`, form `not-base64` |
| `base64.decode("YR==")` |  | Fails: `invalid-argument`, form `not-base64`: bits left over |
| `base64.decode("/w==")` |  | Fails: `invalid-argument`, form `not-utf8` |
| `base64.decode("7aCA")` |  | Fails: `invalid-argument`, form `not-utf8`: an encoded surrogate |
| `base64.decode(s)` | `{s: string}`, s = the 64 characters that encode `"a" × 48` | `"a" × 48`; 2 steps, 1 unit |
| `base64.decode(s)` | `{s: string}`, s = `"*" × 64` | Fails: `invalid-argument`, form `not-base64`; 2 steps, 0 units |

## 10. Types

1. The checker reads the signature of each function record of a group that is
   on, as it reads `library.json` (`05-types.md` 19), and calls a `Call` with
   that namespace by 13.5.
2. A read of a constant has the record's `type`: `float` for both.
3. A call of an entry marked `!` records a `value` site on the call
   (`05-types.md` 18.3). An entry not marked records none.
4. `json.stringify` renders its argument (`05-types.md` 7.3): an argument whose
   type has an opaque type is the check error `kind` on it, and one whose type
   has `any` at any depth records a `dynamic` site on it.
5. `json.parse` gives `any`. A program gives its result a type with `as`, which
   is package F4.
6. No entry takes a callback, narrows, or has a tested position.

| Program | Data | Type | Sites |
| --- | --- | --- | --- |
| `math.pi` | | `float` | None |
| `math.sqrt(x)` | `{x: float}` | `float` | `value` 1:1-1:13 |
| `json.parse(s)` | `{s: string}` | `any` | `value` 1:1-1:14 |
| `json.stringify(x)` | `{x: any}` | `string` | `dynamic` 1:16-1:17 |
| `json.stringify(t)` | `{t: Token}`, `Token` opaque | | Check error `kind` 1:16-1:17 |
| `time.parse(x)` | `{x: any}` | `float` | `value` 1:1-1:14; `dynamic` 1:12-1:13 |
| `math.sqrt("4")` | | | Check error `arg-mismatch` 1:11-1:14 |

## 11. Additions to the case format

Each addition is optional, and its absence leaves every existing case file
valid with its meaning unchanged.

### 11.1 The case field `groups`

This row is added to the table of `03-host.md` 3.10.3.

| Field | Holds | When absent |
| --- | --- | --- |
| `groups` | A list of strings: the groups the case's environment turns on (2.1) | The base environment's: every group off |

1. Its declaration joins those of 3.10.3 item 3. A case's environment has the
   groups the field names and those of the base environment.
2. `groups` may stand in the header (`03-host.md` 3.10.2). A case's `groups`
   replaces the header's whole.
3. A name that is not a group is the `declare` failure `bad-name` of the case,
   not an invalid file.
4. These make the file invalid: `groups` that is not an array, an element that
   is not a string, and a name written twice.

### 11.2 The result field `json`

This row is added to the result table of `03-host.md` 3.10.6.

| Field of a result | Compared with | How |
| --- | --- | --- |
| `json` | The JSON text of the run's value (`03-host.md` 3.2.4), made by the binding's own function after the run | Code point by code point |

1. The runner applies the binding's JSON text function to the value. When that
   function fails, the difference is `json`.
2. A case whose `expect` has `json` reaches step 5 of 3.10.5: it is run.
3. The difference is named `json`. It stands after `value` in the order of
   3.10.8 item 2.
4. `json` in an expected failure makes the file invalid.
5. So the suite can test, for one value, that the binding's JSON text and
   `json.stringify` agree: a case of `suite/groups/groups-json-text.json` states
   `json` for a value, and the next states the same text as the value of
   `json.stringify`.

### 11.3 JSON Schema of the additions

These are added to the schema of `03-host.md` 3.10.11: the property below in
`$defs/fields`, `"groups"` in both lists of `propertyNames`, and `json` in
`$defs/result`.

```json
{
  "fields": {"properties": {
    "groups": {"type": "array", "items": {"type": "string"}, "uniqueItems": true}}},
  "result": {"properties": {
    "json": {"type": "string"}}}
}
```

### 11.4 The directories and the runner

1. The cases of this package are `suite/library-groups.json` and the files of
   `suite/groups/`.
2. A runner of an implementation that offers F6 runs both, as it runs
   `library.json` and `suite/core-run/` (`03-host.md` 3.10.8). A case that
   states `type` or `sites`, or expects a `check` code that is not static, needs
   the type checker by the rules of 3.10.8.
3. A runner of an implementation that does not offer F6 runs neither. For it a
   file with these fields is invalid (`03-host.md` 3.11 item 3).
4. **`suite/groups/with-f01-description.json`** states the result field
   `description` of F1 beside `groups`. An implementation that offers F6
   offers F1 (1.5), so every runner that runs this directory knows the field.
5. **`suite/groups/until-f13-math-functions.json`** pins the outcome of the
   eleven entries of F13 in an implementation that offers F6 without F13
   (8.1 item 3). F13 removes the file when its cases of the same texts are
   added.

### 11.5 The `outcome` command

The `outcome` command (`suite/README.md`) reads the field `groups` and builds
each case's environment with it. Its line gains no field: an implementation that
offers F6 writes the value of a case as before.

## 12. Earlier cases

1. No earlier case is contradicted. In Core every group is off, and the base
   environment keeps every group off, so these Core cases stand as they are:
   `suite/core-run/static-undefined-name.json`, "json is not a namespace in
   Core" and "json.parse in Core is a method call on the undefined name json";
   and `suite/core-run/declare-names.json`, the four cases of a host function in
   a reserved namespace.
2. No earlier case declares a variable named `json`, `time`, `math` or `base64`.
3. No `until-f06-` file exists, and this package removes none.
4. This package writes one file under that rule:
   `suite/groups/until-f13-math-functions.json`, 12 cases (11.4 item 5).

## 13. Examples

Each example is a case of `suite/groups/groups-examples.json`. The groups
`json`, `time`, `math` and `base64` are on.

| # | Program | Data | Result |
| --- | --- | --- | --- |
| 1 | `time.parse(pr.updated_at) < now - time.duration("7d")` | `{pr: {updated_at: string}, now: float}`, pr = `{updated_at: "2024-02-20T09:00:00Z"}`, now = `1709209845` | `true`; 5 steps: `.updated_at` 1, two calls 2, `-` 1, `<` 1. 0 units |
| 2 | `try { json.parse(msg).count ?? 0 } catch { -1 }` | `{msg: string}`, msg = `{"count": 3}` | `3`; 4 steps: the call 1, `c` 2, `.count` 1. 3 units: the map 1 + 2 |
| 3 | The same | msg = `oops` | `-1`; 2 steps: the call 1, the `-` 1. 0 units |
| 4 | `json.stringify({id: 7, tags: ["a", "b"]})` | | `"{\"id\":7,\"tags\":[\"a\",\"b\"]}"`; 7 steps: the call 1, the walk of the map 2 + 1 × 2 and of the list 2. 9 units: the list 3, the map 5, the result 1 |
| 5 | `json.parse(base64.decode(part))` | `{part: string}`, part = `"eyJzdWIiOiJhbmEiLCJleHAiOjE3MDkyMDk4NDV9"` | `{exp: 1709209845, sub: "ana"}`; 5 steps: decode 1, parse 1 + `c` 3. 7 units: the decoded text 1, the map 5, `"ana"` 1 |
| 6 | `math.sqrt(dx ** 2 + dy ** 2)` | `{dx: float, dy: float}`, dx = `3`, dy = `4` | `5`; 4 steps. 0 units |
| 7 | `time.format(time.parse("2024-02-29T12:30:45.500+01:00"))` | | `"2024-02-29T11:30:45.5Z"`; 2 steps, 1 unit |
| 8 | `try { json.parse(s) } catch (err) { err.message }` | `{s: string}`, s = `"9007199254740993"` | `"the JSON text has a number with no fraction that is not a whole number"`; 4 steps: the call 1, the message of weight 70 and size 1, 1 + 1, `.message` 1. 11 units: 9, and the message 1 + 1 |
