# F10. Config files

This chapter is normative. It specifies follow-on package F10. A config file is
a program of `01-language.md`: `const` lines, then one expression, whose value
is the configuration. A host **loads** it with a type the result must have,
and gets data: checked before it runs, bounded while it runs, tested against
the type after it runs, and decoded into the host's own types.

The package adds to Core and to packages F1, F4, F5 and F6:

1. An expected result given by a JSON Schema (2).
2. What the check and a load promise about a result, and where a schema says
   more than its type, so that a host must still validate (3).
3. The JSON text of a result as the contract of a load (4).
4. The load: its steps, the phase of each failure, and what the author of a
   config file is shown (5).
5. The decode of a result into a host type: a portable contract, then binding
   rules (6).
6. What a config file may use: settings from outside, the library, limits,
   determinism (7).
7. Layering one config over another, without imports (8).
8. The commands `eval --format json` and `check --type` (9).
9. Two optional fields of a case, `returnsSchema` and `load` (10).

It adds no syntax, no node kind, no reserved word, no library entry, no run
code, no check code, no `declare` code, no message template and no cost row
(11).

The chapters of Core are cited by file and section: `05-types.md` 15.1 is
section 15.1 of `spec/core/05-types.md`. F1 is
`spec/packages/f01-host-types.md`, F4 `spec/packages/f04-conversion.md`, F5
`spec/packages/f05-unions.md` and F6 `spec/packages/f06-groups.md`. Section
numbers without a file are of this chapter.

## 1. Conventions

### 1.1 Terms

| Term | Meaning |
| --- | --- |
| **Config file** | The source text of a program that a host loads. Nothing in its text marks it as one |
| **Load** | The steps of 5.2, made by a binding for a host |
| **Expected type**, T | The type the result must have: the expected result of `03-host.md` 3.2.3, given as type text, as a schema (2), as a named type (F4 3), or as a type derived from a host type (F1 6) |
| **Host type**, H | A type of the host language that a load decodes into (6) |
| **Valid** | A JSON value is valid by a schema when a JSON Schema validator accepts it, as F1 1.1 says |
| **M(S)** | The type F1 3 maps schema S to, with the options given (F1 4) |
| **Conforms** | As `05-types.md` 18.1 says, with the row of F5 7.1 |
| **The conformance function** | F4 7: one value, one type, nothing or one `not-conforming` diagnostic |

### 1.2 Requirements

1. An implementation that offers F10 offers every section of this chapter but
   6.4 to 6.6, which bind the binding of their language, and 9, which binds an
   implementation that ships a command-line tool.
2. Packages are offered in the order F3, F1, F2, F4, F5, F6, F7, F8, F9, F10
   (`03-host.md` 3.11 item 2). So an implementation that offers F10 has the
   type checker, schemas (F1), named types and the conformance function (F4),
   tagged unions (F5), the groups (F6), queries (F7), the second part of
   errors (F8) and testing for authors (F9). The cases of `suite/config/` use
   text blocks (F3), unions (F5), and the fields `description` (F1), `groups`
   and `json` (F6).

### 1.3 Version

F10 is part of version `3.0`. It adds no version number.

### 1.4 What this package changes in earlier rules

| Earlier rule | With this package |
| --- | --- |
| `03-host.md` 3.2.3, the table of expected results | Gains the row "a type, by JSON Schema" (2.1) |
| `03-host.md` 3.3.4, the row of the expected result's type | Covers a schema given as the expected result, and two options refused there (2.1) |
| `03-host.md` 3.10.2 item 4 | `returns` and `returnsSchema` count as one field (10.1) |
| `03-host.md` 3.10.3 | Gains the fields `returnsSchema` and `load` (10) |
| `03-host.md` 3.10.5 | Gains step 5a for a case with `load` (10.2) |
| `03-host.md` 3.10.6 | In a case with `load`, an expected diagnostic may state `"span": null` (10.2) |

No other rule changes. A program's type, sites, diagnostics, value and usage
are those of Core and of the packages before this one.

## 2. The expected result by schema

### 2.1 The declaration

This row is added to the table of `03-host.md` 3.2.3.

| Expected result | Accepted results | Reported |
| --- | --- | --- |
| A type, by JSON Schema: a schema S and a set of options | A result whose type fits M(S) through its form, by `05-types.md` 15 | `result-mismatch`, at each place of failure that `05-types.md` 15.1 and F5 6 give |

1. The type is M(S), with the options given, by F1 3 and F1 4. The schema is
   not kept. After the type is made, the check is the check with an expected
   type in type text: nothing in `05-types.md` 15 changes.
2. **The options.** Only `closed` is accepted. `absent-is-null` and
   `defaults-applied` state what a host did to a value before it supplied it
   (F1 4). Nobody supplies a result, so for an expected result either option
   is the `declare` error `bad-type`, alone or beside `closed`.
3. Any error in the schema, or in the type it gives, is the `declare` error
   `bad-type`, as F1 2.1 item 4 says for a variable. The limits of F1 3.10
   apply: a schema deeper than 64, a type deeper than 64, and a type of size
   over 100,000 (`05-types.md` 20.7) are `bad-type`.
4. The expected result is not a declaration of the environment (`03-host.md`
   3.2.3 item 1). A schema given as the expected result has no name, clashes
   with nothing, and is not in the environment's description (F1 5.3).
5. A schema names no type of the environment (F1 2.1 item 3).
6. A compile without a check (`03-host.md` 3.2.1 item 4) still maps the schema
   and reports its errors. It makes no fit.
7. **A named type as the expected result** (F4 3.3) is the other way to give a
   schema: a named type declared by schema, with the options of its
   declaration, named by type text. Such a named type is a type like any
   other, and item 2 does not apply to it. A load given a schema uses this
   section's form (5.6).

### 2.2 What `closed` means for a result

| For | `closed` states |
| --- | --- |
| A variable (F1 4.1) | What the host did: it removed from each object every key the closed record does not list |
| A result | What the program must do: no object of the result whose record Close made exact may have a key that record does not list |

1. So for a result `closed` narrows the type: a result that fits M(S) under
   `closed` fits M(S) without it.
2. A key that the closed record does not list, written in a map literal that
   the fit reaches by `05-types.md` 15.1, is `result-mismatch` at the key: the
   record's rest is `never`. That is the error at the key a config file needs:
   a misspelled key fails before anything runs.
3. Where the fit stops at a node that it does not follow (a `const` name, a
   call, `05-types.md` 15.1), the report is on that node.
4. A key that reaches the result through `any` is not seen by the check. Step
   4 of a load finds it (5.3).
5. Without `closed`, a schema with `properties` and no `additionalProperties`
   allows every other key, as the schema does.

### 2.3 Fitting through the form

The fit of `05-types.md` 15.1, with the row of F5 6, applies unchanged with a
type from a schema. In a config file it gives these places.

| Result text, against the closed schema of 13.1 | Reported |
| --- | --- |
| `{listen: ":8080", backends: [], max_bod: 1}` | `result-mismatch` on `max_bod` |
| `{listen: ":8080"}` | On the whole map, once: `backends` is required |
| `{listen: 8080, backends: []}` | On `8080` |
| `{listen: "x", backends: [{name: "a", url: "u", weight: 2}]}` | On `weight`: the record behind `$ref` is closed |
| `if (prod) { {...} } else { {..., debug: true} }` | On `debug`, in the branch |
| `const cfg = {...}; cfg` | On the last `cfg`: a `const` name is not followed |
| `{...base, extra: 1}`, `base` an exact record that fits | On `extra`: a written entry beside a spread is fitted as in a map with no spread (8.2) |
| `{listen: x, backends: []}`, `x: any` | Nothing. A `dynamic` site on the whole map, since its type fits by `any` |

1. The expected result changes no type and no site (`05-types.md` 15.3). The
   program's type is the inferred one.
2. Each place of failure is reported, in the order of `05-types.md` 6.1.

### 2.4 Defaults

1. **Nothing is filled in.** A load never adds a key to a result. The
   keyword `default` of a schema is not read for an expected result: the
   option that reads it is refused (2.1 item 2).
2. A default comes from one of three places:

   | Place | How |
   | --- | --- |
   | The program | `??` where a setting is read: `env?.PORT ?? "8080"`. Or a spread over a map of defaults the host supplies: `{...defaults, port: 9000}` (8) |
   | The host, before the decode | In Go, the host fills its own value of H, and the decode keeps every optional field the result lacks (6.4 item 3). In Rust, `#[risor(default)]` gives a field's `Default` (6.5 item 3) |
   | The host, after the load | The host reads the optional field and uses its own default |

3. A field the program may leave out is optional in T. A schema makes it
   optional by leaving it out of `required`.

### 2.5 Unions

1. Branches of `anyOf` or `oneOf` whose records have a tag give a tagged union
   (F5 8.1). It is the usual form of a config value with variants, such as
   `auth` of 13.1.
2. A map literal against the union fits the variant its tag's value picks
   (F5 6). A tag no variant has is reported at the tag's value; a map with no
   entry for the tag, on the map.
3. Under `closed` each variant is closed (F5 8.2): a key that the variant the
   tag picks does not list is reported at the key.
4. Branches whose records have no tag join (F1 3.6). Under `closed` the joined
   record is exact with the field types of the Join: `anyOf` of `{a: string}`
   and `{b: integer}`, each with `properties` only, is `{a?: any, b?: any}`.

## 3. What a result promises

### 3.1 The check and the load

Let T be the expected type, and let a program check against T and run to a
value v.

| Part | Statement |
| --- | --- |
| Promise 1 | v conforms to the program's inferred type (`05-types.md` 18.4, promise 4) |
| Promise 2 | v conforms to T, unless the check recorded a `dynamic` site, or the precondition of `05-types.md` 18.4 failed |
| Promise 3 | A load tests v against T at its step 4, by the conformance function. So a value a load returns conforms to T, without exception |

1. Promise 2 holds because the fit of `05-types.md` 15.1 succeeds only where
   each part of the result fits its part of T, and a fit that uses `any` is a
   `dynamic` site (15.1, "A fit that used any").
2. So step 4 of a load fails only when the check recorded a `dynamic` site,
   or the precondition failed. In an environment that rejects the class
   `dynamic` (`05-types.md` 17), step 4 fails only when the precondition
   failed.

### 3.2 A type from a schema says less than the schema

F1 2.3 promises, for a variable, that every value valid by S conforms to M(S):
the type is never narrower than the schema. For a result the host needs the
other direction: that a value which conforms to T is valid by S. In general it
does not hold, because M(S) is wider than S wherever F1 widens.

| Where the schema says more than its type | So a result that conforms may be invalid by S |
| --- | --- |
| A keyword F1 does not read (F1 3.12) that constrains a value: `minimum`, `maximum`, `exclusiveMinimum`, `exclusiveMaximum`, `multipleOf`, `minLength`, `maxLength`, `pattern`, `format`, `minItems`, `maxItems`, `uniqueItems`, `contains`, `minContains`, `maxContains`, `minProperties`, `maxProperties`, `propertyNames`, `dependentRequired`, `dependentSchemas`, `dependencies`, `not`, `if`, `then`, `else`, `unevaluatedItems`, `unevaluatedProperties`, `contentEncoding`, `contentMediaType`, `contentSchema` | `{"type": "integer", "minimum": 0}` is `int`, and `-1` conforms |
| An `enum` or `const` value that is a number, a bool, an array or an object | `{"enum": [1, 2]}` is `int`, and `3` conforms |
| `anyOf` and `oneOf` whose branches overlap or whose records do not form a tagged union | The Join of F1 3.6 admits a value no branch accepts, and `oneOf`'s "exactly one" is not kept |
| `allOf` where Pick takes the first type because none fits every other (F1 3.6 item 2.3) | `allOf` of `string` and `integer` is `string`, and no value is valid |
| `prefixItems`, an array `items`, `additionalItems` | The positions are joined into one element type |
| `patternProperties`, and a key of `properties` that a pattern also matches | The patterns are joined into the rest |
| Keywords beside `$ref` | F1 ignores them; drafts 2019-09 and later apply them |
| `nullable` | OpenAPI 3.0 reads it; the drafts of JSON Schema do not |

A host that needs a result to be valid by a schema of this table runs its own
validator on the JSON text of the result (4), after the load.

### 3.3 Faithful schemas

A schema is **faithful** when it is `true`, `false`, or an object in which
every keyword is allowed by this table and every subschema it walks (F1 3.10
item 2) is faithful.

| Keyword | Allowed when |
| --- | --- |
| `title`, `description`, `$comment`, `examples`, `example`, `default`, `deprecated`, `readOnly`, `writeOnly`, `$schema`, `$anchor`, `$dynamicAnchor`, `$vocabulary`, `$defs`, `definitions`, `discriminator`, `xml`, `externalDocs`, and every keyword that begins with `x-` | Always |
| `$id` | On the root only |
| `type` | Always |
| `enum`, `const` | Every value that `type` allows is a string or null |
| `items` | It is a schema, beside `type`, and S has no `prefixItems` and no `additionalItems` |
| `properties`, `required`, `additionalProperties` | Beside `type`, and S has no `patternProperties` |
| `$ref` | The other keywords of S are of the first row |
| `allOf` | No Pick that the mapping of S makes, at any depth, takes the first type in step 3 because no type fits every other (F1 3.6 item 2) |
| `anyOf`, `oneOf` | No two branch types have an alternative of one kind, where `int` and `float` are one kind, a literal set and `string` are one kind, and `any` has an alternative of every kind; except map alternatives, which may stand in two or more branches when the records of the branches form a tagged union (F5 8.1) |

1. **The promise.** When S is faithful, every value that conforms to M(S),
   with or without `closed`, is valid by S under every draft of F1 2.2.
2. So for a faithful schema, a value a load returns (3.1, promise 3) is valid
   by S, and the host needs no validator. Under `closed` the value also has no
   key that a closed record does not list.
3. **Why.** By induction on the walk. `true` gives `any` and accepts every
   value; `false` gives `never`, to which no value conforms. A value that
   conforms to the Join of the types of `type`'s names is of one of those
   kinds; an `int` is whole, so `integer` accepts it. A string or null in a
   literal type is a value of the `enum`. An element of `E[]` conforms to E =
   M(`items`). In a record, a present listed key conforms to M of its
   property, a required key is present, and another key conforms to
   M(`additionalProperties`), or to `any` when the schema has none and allows
   it. A value that conforms to a Pick conforms to each type picked from:
   `any` says nothing, a Meet is made of Picks, and a type that fits every
   other is narrower than each. For `anyOf` and `oneOf`, a value that
   conforms to the part conforms to the branch type that holds its kind, or,
   for a map, to the branch whose variant its tag picks; by F1 2.3 it is
   valid by no other branch, since no other branch type has its kind or its
   tag. `closed` only narrows the type.
4. Faithful is a sufficient condition. A schema that is not faithful may still
   have no value that conforms and is not valid. A binding need not offer a
   test of it.

## 4. The JSON text of a result

### 4.1 The contract

The JSON text of a result is the function of `03-host.md` 3.2.4. This package
adds no rule to it. As a contract of a load it is:

1. The JSON form of `01-language.md` 12: no white space anywhere; the entries
   of every map in key order by code point; each string escaped as that
   section lists and in no other way; each number in the rendering of
   `01-language.md` 6.3.
2. Text, not bytes. Where a binding gives bytes, they are the UTF-8 encoding
   of the text, with no byte order mark and no line end after it.
3. It is not a run. It charges nothing and has no limits but its depth.
4. It fails as `03-host.md` 3.2.4 says, with no span: `wrong-kind` on an opaque
   value at any depth; `depth-exceeded` on nesting deeper than 200, the
   default depth limit, whatever the environment's depth limit is.
5. A result may share memory with host data (`03-host.md` 3.4.2). The JSON
   text reads host data as a run does: a map's keys in code point order
   whatever the host's order, a host `-0.0` as `0`, text that is not valid with
   U+FFFD (`03-host.md` 3.4.5). A host number that cannot enter the language
   (`03-host.md` 3.4.4) fails the function with `number-range` and no span.
6. For one value it is the same text as `json.stringify` gives in a program
   (F6 6.1 item 2), and the same in every implementation.

| Value | JSON text |
| --- | --- |
| `{max_body: 16 * 1024 * 1024, listen: ":8080", tls: null}` | `{"listen":":8080","max_body":16777216,"tls":null}` |
| `{"a_b": 1, ab: 2, "a-b": 3, Z: 4, _: 5, a: 6, "1": 7, "": 8}` | `{"":8,"1":7,"Z":4,"_":5,"a":6,"a-b":3,"a_b":1,"ab":2}` |
| `[1e21, 0.1 * 3, 1e20, 123.0, -0, 1e-7]` | `[1e+21,0.30000000000000004,100000000000000000000,123,0,1e-7]` |
| `"/srv/<app>&"` | `"/srv/<app>&"` |

### 4.2 What the suite tests

1. The result field `json` of F6 11.2 compares a binding's JSON text with a
   case's text, code point by code point. `suite/config/config-json-text.json`
   states it for values a config gives: whole configs, key order, numbers,
   escapes, host data read in place.
2. A case that states `json` beside `load` compares the JSON text of the value
   a load returns.

### 4.3 No pretty form

1. The JSON text has one form. This package specifies no indented form.
2. A tool may offer one (9.2 item 6). Its text is not specified, and no case
   states it.

## 5. The load

### 5.1 What a load takes

| Input | Rule |
| --- | --- |
| The source text | As for parse (`03-host.md` 3.4.5 item 3). Reading a file is the host's work, not the binding's: a load takes text |
| The origin | The name a binding gives the text, for its diagnostics. Not specified |
| An environment | As for compile (`03-host.md` 3.3) |
| The expected type T | Type text, a schema (2.1), a named type, or a type derived from a host type (F1 6) |
| Variables | As for a run (`03-host.md` 3.4.1) |
| A cancellation signal | As for a run |
| A host type H | Optional. With it, step 5 is made |

### 5.2 The steps

A load makes these steps in this order. Each is made only when every step
before it succeeded. The load stops at the first step that fails, and gives
its diagnostics.

| Step | What is made | A failure gives |
| --- | --- | --- |
| 0. Declare | The environment is built, and T is resolved or mapped | Phase `declare`: `bad-type` for T, and any error of `03-host.md` 3.3.4 |
| 1. Parse | Parse (`03-host.md` 3.2.1) | Phase `parse` |
| 2. Check and compile | Compile with the check, T as the expected result. A load never compiles without the check | Phase `check`: every diagnostic, `result-mismatch` among them at the places of 2.3 |
| 3. Run | Run, with the variables and the signal | Phase `run`: one diagnostic, with its span |
| 4. Conform | The function that reports every failure (F8 6) on v and T, with a maximum the host gives | Phase `run`: one or more diagnostics, each with code `not-conforming`, class `value`, the path and message of F4 4.4, and no span |
| 5. Decode | The decode of v into H (6), when H is given | Phase `run`: the codes of 6.1, each with a path and no span |

1. **Step 4 is always made.** It charges nothing and takes no limit but the
   maximum (F4 7.1, F8 6.1). It reports up to the maximum failures, in the
   order of F8 6.2. A maximum below 1 is read as 1. Its first diagnostic is
   the one the conformance function of F4 7 gives, so a load with a maximum of
   1 reports what `v as T` would report.
2. A failure of step 4 is not a run failure: `catch` in the program cannot
   catch it, since the run has ended. An `as` in the program that fails is a
   failure of step 3, with a span (F4 5.1).
3. **Usage.** Only step 3 charges. A load's usage is the usage of its run. A
   load that fails at step 4 or 5 reports the usage of its run, which
   succeeded.
4. **What a load gives.** On success: the value v, or with H the decoded host
   value; and the usage. On failure: the diagnostics of the step that failed,
   the number of that step, and the usage when step 3 was reached.
5. Steps 3, 4 and 5 all give phase `run`. A binding tells the host which step
   failed. A failure of step 4 or 5 has no span and has a path.

### 5.3 When step 4 fails

| Cause | Example |
| --- | --- |
| A part of the result came through `any`: a variable typed `any`, `json.parse`, a field of an open record's rest | `{auth: a}`, `a: any`, with `a` = `{kind: "token"}` against the union of 13.1: path `["auth", "header"]`, `required field "header" is missing` |
| A host value broke its declared type (the precondition) | `x`, declared `{port: int}`, supplied `{port: "80"}`: path `["port"]`, `expected int, found "80"` |

1. By 3.1, nothing else makes step 4 fail.
2. Under `closed`, a key that reaches the result through `any` and that the
   closed record does not list fails at step 4 with the template `extra`.

### 5.4 What the author of a config file sees

| Step | The author sees |
| --- | --- |
| 0 | An error of the host's declarations. Not the author's: a bad schema or type |
| 1 | The first `parse` diagnostic, at its place in the file |
| 2 | Every `check` diagnostic, each at its place in the file: a misspelled key at the key, a missing field at its map, a value of the wrong kind at the value |
| 3 | One run diagnostic at its place in the file, with the message of the catalog |
| 4 | Each failure at a path in the value, with the message of F4 4.4 |
| 5 | Each failure at a path in the value, with the binding's message |

1. The rendering of a diagnostic for a person (a line, a detail form, color)
   is package F8's and F12's.

### 5.5 Sharing

1. The value of step 3 may share memory with the variables (`03-host.md`
   3.4.2).
2. The decode copies: what it gives shares no memory with the variables.
3. A load with no H gives v as the run gave it. A binding states whether it
   copies it.

### 5.6 A load given a schema

1. A load given T as a schema maps it with the option `closed` and no other
   option (2.1). A host that wants another mapping makes the steps itself:
   compile with its own expected type, run, conform, decode.
2. The decode refuses a key that H has no field for (6.1). Without `closed`
   such a key would pass the check and fail only at step 5; with it, the
   check reports it at the key.

### 5.7 The check classes

A load checks with the environment's rejected classes (`03-host.md` 3.3.1
item 4). It adds none. A host that wants every part of a result typed before
it runs rejects the class `dynamic`; then a value from `json.parse` must pass
through `as` (F4) to reach the result.

## 6. The decode

### 6.1 The portable contract

The decode takes a value v that a run returned and a host type H, and gives a
value of H. Let D(H) be the type the binding derives from H (F1 6).

1. **The rule.** A decode fails when v does not conform to D(H). Its failures
   are those of the walk of every failure (F8 6.2) of v against D(H), each at
   its path. It fails also where a host type is narrower than its part of
   D(H): items 7 and 8.
2. **What it gives,** when it does not fail: the value of H that holds v, field
   by field, under the naming rules by which F1 6 presents H. A binding need
   not make the JSON text of v (4) to give it.
3. **It is not a run.** It charges nothing, has no limits but the depth of 4.1
   item 4, and cannot be cancelled.
4. **It copies.** What it gives shares no memory with v or with host data.
5. So, by item 1:

   | v holds | Because D(H) | The decode |
   | --- | --- | --- |
   | A key for which H has no field | Is an exact record (F1 6.1) | Fails at that key, template `extra` |
   | No key for a field H presents always | Has the field required | Fails at the key, template `missing` |
   | No key for a field H may present as absent | Has the field optional | Gives the field the binding's value for an absent key (6.4, 6.5) |
   | Null where H's type cannot hold null | Has no `null` there | Fails at the path, template `wrong`. Never a zero value left in place |
   | A value of another kind than H's type there | Has no alternative of that kind | Fails at the path, template `wrong` |

6. Keys are matched code point for code point, by the key under which F1 6
   presents the field.
7. **Narrower than D(H).** A number that the host number type there cannot
   hold fails at its path: for an integer type, one outside the type's range
   (D(H) is `int` for every integer type); for a floating type, one of
   greater magnitude than its largest finite value. Text that a text type
   refuses, by its own parsing, fails at its path.
8. **Never decoded.** An opaque value fails at its path, even where D(H) names
   its opaque type: a decode copies, and an opaque value is not data. A host
   number that cannot enter the language fails at its path with `number-range`
   (`03-host.md` 3.4.4 item 4). Nesting deeper than 200 fails with
   `depth-exceeded` and no path.
9. Every failure has phase `run`, no span, and the code `not-conforming` but
   where item 8 says. A failure by item 1 has the path and the message of F8
   6.2. A failure by items 7 and 8 has a message of the binding's: no template
   of the catalog words it.
10. **The order.** A decode reports at least its first failure, in the order
    of F8 6.2. When it fails, what it gives is not specified.
11. A number decoded into a host floating type of 64 bits is the same number.
    Into a narrower floating type it is rounded to nearest, ties to even.

### 6.2 What step 4 guarantees the decode

1. When T is the type the binding derives from H (F1 6), every value that
   conforms to T decodes into H, with three exceptions: a number outside the
   range of a host integer type narrower than `int` (an 8-bit field and
   `300`) or a floating type's range; text that a text type refuses (a Go
   `time.Time` and text that is not RFC 3339); an opaque value, which no
   decode accepts.
2. When T is not D(H), the decode fails wherever v does not conform to D(H)
   (6.1 item 1). The load reports it at step 5, with a path.
3. So a host type with an opaque field cannot be loaded. An opaque value in a
   result is refused by the JSON text and by the decode alike.

### 6.3 The order in a load

The decode is step 5. It is made only after step 4, so its input conforms to
T. It is the only step whose result depends on the host language.

### 6.4 Go (binding rules)

`risor.Decode(v, dst)` decodes; `dst` is a non-nil pointer, as for
`json.Unmarshal`. D(H) is `TypeOf` of the destination's type. `risor.Load`
makes steps 0 to 5 with T = D(H).

1. A struct field is matched by the key F1 6.2 gives it: its `risor` tag,
   else its `json` tag name, else its Go name. The match is exact. Unlike
   `encoding/json`, no key matches a field by case folding.
2. A field tagged `json:"-"` or `risor:"-"` is not decoded. A key of that name
   is a key with no field.
3. A field whose `json` tag has `omitempty` or `omitzero` is optional in
   D(H) (F1 6.2 item 3). A field whose key v lacks keeps what `dst` held: so a
   host fills its defaults into `dst` before the decode. Every other field is
   required, and its key missing is a failure (6.1 item 5).
4. Embedded structs are decoded as `encoding/json` decodes them, by the
   presentation of F1 6.2 items 4 and 5.
5. Numbers: into an integer type, the number must be whole and within the
   type's range; into `float32`, 6.1 item 11; into `float64`, exactly; into an
   interface, as a `float64`.
6. Null: into a pointer or an interface, nil. Into any other type, the
   failure of 6.1 item 5, though `encoding/json` would leave the field as it
   was: D(H) has `null` only for a pointer and an interface (F1 6.2).
7. A type that implements `encoding.TextUnmarshaler` takes a string through
   `UnmarshalText`; an error from it is `not-conforming` at the path. A type
   that implements `json.Unmarshaler` takes the JSON text (4) of the value at
   that place through `UnmarshalJSON`.
8. A type registered as opaque (F1 6.2, first row) refuses every value
   (6.1 item 8).
9. A value decoded into an interface is `nil`, `bool`, `float64`, `string`,
   `[]any` or `map[string]any`.
10. Where these rules and `encoding/json` differ (names, case folding, null,
    a missing required field), these rules hold.

### 6.5 Rust (binding rules)

The Rust crate has no library dependency (rulings BB16, BB26). Its decode is
its own, by a trait the derive of F1 6.3 also implements. These rules mirror
F1 6.3, read in the other direction.

| Rust | Decodes |
| --- | --- |
| `bool`, `String` | A bool, a string |
| Every integer type | A whole number within the type's range |
| `f64`, `f32` | A number; 6.1 item 11 |
| `Vec<T>`, `VecDeque<T>` | A list, each element as T |
| `[T; N]` | A list of exactly N elements; any other length is `not-conforming` at the list |
| `HashMap<String, T>`, `BTreeMap<String, T>` | A map, each value as T |
| `Option<T>` | Null as `None`; any other value as T |
| `Box<T>`, `Rc<T>`, `Arc<T>` | As T |
| `risor::Data` | Any value, copied |
| `Opaque<T>` | Nothing: every value is `not-conforming` |
| A struct with `#[derive(risor::Host)]` | A map, by the rules below |
| A borrowed type (`&str`, `&[T]`, `&T`, `Cow<str>`) | Not decodable: the trait is not implemented, and the host's build fails |

1. A field's key is its `#[risor(rename = "k")]`, else its Rust name with
   `r#` dropped, as F1 6.3 says.
2. A field marked `#[risor(skip)]` is not decoded and takes
   `Default::default()`. A key of that name is a key with no field.
3. **`#[risor(default)]`.** This mark adds a row to F1 6.3. On a field whose
   type implements `Default`, it makes the field optional in D(H): `f?: T`. A
   value presents the field always (F1 6.3), which is sound for an optional
   field. A decode gives `Default::default()` for an absent key. On a type
   that does not implement `Default`, the build fails. It may share an
   attribute with `rename` (`#[risor(rename = "k", default)]`), and not with
   `skip` or `absent_if_none`.
4. An absent key of a field marked `absent_if_none` gives `None`. An absent
   key of any other field that is neither `default` nor `skip` is the failure
   `missing` of 6.1 item 5.
5. The decode makes a new value. A host's defaults come from the `Default` of
   a field's type, by `#[risor(default)]`, or from an `Option` the host reads
   with its own default.

### 6.6 TypeScript (binding rules)

TypeScript types are erased (F1 6.4). There is no H to decode into.

1. The decode is a copy into plain data: null as `null`, a bool, a number, a
   string, a list as an `Array`, a map as an object.
2. A map becomes an object whose own enumerable properties are exactly its
   keys. A key such as `__proto__` or `constructor` is an own property, and no
   prototype is changed.
3. The order of an object's keys is JavaScript's, not specified here: integer
   keys come first in that language. The JSON text (4) has the specified
   order.
4. An opaque value is `not-conforming` at its path.
5. `load<T>` asserts the type T of the host's code. Steps 2 and 4 make the
   assertion true of the type the host gave as text or schema.

## 7. What a config file may use

### 7.1 The language

1. A config file is any program. Nothing of the language is off for it:
   `const` lines, lambdas bound by `const`, templates, text blocks (F3),
   `if`, `match`, `try`, spreads, the library.
2. A function is not a value (`01-language.md` 8.2), so no function reaches the
   host in a result.
3. A host that wants less has the means of Core and the packages: an
   environment that declares nothing, removed entries (`03-host.md` 3.3.1), and
   the analysis of F2, which tells a text that is one literal.

### 7.2 Settings from outside

| Matter | Rule |
| --- | --- |
| How | A variable the host declares with a type and supplies to the run (`03-host.md` 3.4.1). For example `env: map<string>`, or `env: {REGION: string, DEBUG?: string}` |
| What the library reads | Nothing. No entry reads the process environment, a file or a clock (`03-host.md` 3.9.1) |
| A setting read with `map<string>` | `env?.REGION ?? "us-east-1"` is a `string`, and cannot fail |
| A setting read with an exact record | `env.REGION`. A misspelled name is `no-field` at check time |
| A declared setting not supplied | `missing-variable` where it is read (`03-host.md` 3.4.1 item 3) |
| Text | A setting from a process is text. `int(env.PORT)` converts and may fail with `invalid-argument`. The text `"false"` is truthy, so a flag is compared: `env.DEBUG == "true"` |

### 7.3 The library and the groups

1. A config file has the Core library and the groups the environment turns on
   (F6 2.1). A load turns on no group by itself.
2. The group `time` has no clock: there is no entry that gives the current
   time, and `time.now()` is `undefined-function`. A host that wants the time
   in a config supplies it as a variable.
3. `json.parse` of a setting gives `any`. Its parts reach T through `any`, so
   step 4 tests them (5.3).
4. Host functions may be declared and called. Their results are inputs of the
   load, as variables are.

### 7.4 Limits

1. A load is one run: one step budget, one allocation budget, one depth limit
   and one deadline, the environment's (`03-host.md` 3.6.1). Their defaults are
   Core's: 1,000,000 steps, 1,000,000 units, depth 200, no deadline.
2. A load sets no limit of its own. A host that loads files it does not trust
   to be small sets lower limits, or a deadline.
3. Steps 0, 1, 2, 4 and 5 are bounded by the source limits, the checker's
   limits (`05-types.md` 20) and the limits of F4 7.1 and 6.1.

### 7.5 Determinism

1. The same text, version, declarations, variables, host function results and
   limits give the same outcome of a load: the same value, or the same step
   with the same diagnostics, and the same usage. A deadline or a
   cancellation is the exception (`03-host.md` 3.9.1).
2. Step 4 is deterministic. Step 5 is as deterministic as the host's own
   decoding.
3. A host that records the variables it supplied can repeat a load.

### 7.6 A JSON document as a config file

A JSON document is a program whose value is the document, with these
exceptions. Each is an error, never a second meaning.

| JSON text | Outcome | Rule |
| --- | --- | --- |
| A key written twice in one object | `duplicate-key` on the later key, phase `check` | `05-types.md` 22.8 |
| An integer with no fraction and no exponent outside ±(2^53 − 1) | `literal-range` on the number, phase `parse` | `01-language.md` 2.5 |
| A number whose value is not finite, such as `1e400` | `literal-range` on the number | `01-language.md` 2.5 |
| An escape of a lone surrogate, such as `"\ud800"` | `bad-escape` on the escape | `01-language.md` 2.7 |
| Arrays and objects nested deeper than 100 levels | `nesting-exceeded`, phase `parse` | `01-language.md` 13.2 |
| Nesting whose type is deeper than 64 | `check-limit`, phase `check`, on the innermost node whose type is too deep. A load always checks, so such a document does not load | `05-types.md` 20.4 |

1. Everything else a JSON document holds means what it means in JSON: `\/`
   and `\u` escapes, an escaped surrogate pair, `1E3`, `-0` (which is zero),
   white space with U+000D, and quoted keys of any text.
2. A tool that writes configuration writes JSON, which is a config file.

## 8. Layering

### 8.1 A config over a base

There is no layering mechanism. A host layers with what the language has:

1. It loads the base file, and supplies its value as a variable of the next
   load, declared with a type: the base's T, or a type of its own.
2. The overlay spreads the base and sets the keys it changes:
   `{...base, max_body: 16 * 1024 * 1024}`.
3. Spread is one level deep. A nested override spreads the nested map too:
   `{...base, db: {...base.db, port: 5433}}`. A list is extended with a spread
   in a list: `{...base, hosts: [...base.hosts, "c"]}`.
4. Several layers are several spreads, later ones winning:
   `{...base, ...region, listen: ":9000"}`.
5. The type of the overlay is the Merge of `05-types.md` 7.6, and it is fitted
   to T as any result is.

### 8.2 What the author of an overlay sees

`05-types.md` 15.1 fits a map literal with spreads by its table "A map literal
against a record": for each key, the entries that may give it last are its
**contributors**, and only they are fitted. So a layer's mistake is reported
where it was written.

| Overlay, against an exact T, `base` of type T | Reported |
| --- | --- |
| `{...base, max_bdy: 1}` | `result-mismatch` on `max_bdy`: a written key T does not list |
| `{...base, max_body: "big"}` | On `"big"`: a written value that does not fit its field |
| `{...base, listen: 8080}` | On `8080`: the written entry overrides `base`'s `listen`, so it alone is fitted |
| `{...base, max_body: 1}`, `base` declared with `listen: int` | On `base`: the key `listen` comes from the spread, whose type for it does not fit |
| `{...base, listen: ":80"}`, the same `base` | Nothing: the written entry overrides the failing key, and `base` is not a contributor of it |
| `{listen: 1, ...base}` | Nothing: the later spread surely gives `listen`, so the written entry is overridden and not fitted |
| `{...base, ...region, listen: ":9000"}`, `region: {max_body?: string}` | On `region`, the layer that may give the key last |
| `{...base, ...maybe}`, `maybe: map<string>` | On `maybe`, once, though it may give several keys that do not fit |
| `{...over, hosts: []}`, `over` with `listen` optional | On the whole map: a required key no entry surely gives |

1. A spread is reported on its operand, once for each spread, whatever the
   number of keys of it that fail.
2. A spread of a value of type `any` may give every key with type `any`: its
   fit is a `dynamic` site on its operand, and step 4 of the load tests the
   value (5.3).
3. A map with a computed key is still not followed: the report is on the
   whole map.

### 8.3 Without imports

1. Imports are not in version 3.0. `import` is a reserved word:
   `import "./base.risor"` is `reserved-word` on `import` (`01-language.md`
   14.2), in every place.
2. A host that composes files loads each one, in an order it chooses, and
   supplies the earlier values as variables of the later loads. A file reads
   no other file.

## 9. The commands

### 9.1 Scope

These rules bind an implementation that ships a command-line tool. They fix
what a config needs: the steps each command makes, the text of a value on
standard output, and the exit status. Package F12 fixes the rest: the layout
of diagnostics, errors in the command line, `--pretty`, `--description`, the
REPL and `fmt`.

### 9.2 `eval`

```
risor eval FILE [--type TEXT | --schema FILE] [--format text|json]
```

1. It reads FILE as UTF-8 and makes the steps of a load (5.2) without step 5.
2. With `--type TEXT`, T is the type text. With `--schema FILE`, T is the
   schema read from that file as JSON, mapped with `closed` (5.6). With
   neither, the check has no expected result, and step 4 is not made.
3. `--format text`, the default, writes the rendering of the value
   (`01-language.md` 12) and one U+000A.
4. `--format json` writes the JSON text of the value (4) and one U+000A, in
   UTF-8, and nothing else, to standard output.
5. When the JSON text fails (4.1 item 4), the command writes no value and
   fails with that diagnostic.
6. A tool may offer `--pretty`, which writes an indented JSON text. Its text
   is not specified (4.3).

### 9.3 `check`

```
risor check FILE [--type TEXT | --schema FILE]
```

1. It makes steps 0, 1 and 2 of a load. `--type` and `--schema` are as for
   `eval`. It does not run the program.

### 9.4 Variables

| Flag | Declares and supplies |
| --- | --- |
| `--var NAME=TEXT` | A variable NAME of type `string`, holding TEXT |
| `--vars FILE.json` | One variable for each key of the JSON object in the file. Its type is the literal type of its value (F1 3.4) |
| `--env NAME` | The key NAME of the variable `env`, declared `map<string>`, holding the process variable NAME. A process variable that is not set is an absent key. Repeated, it adds keys |

1. The tool reads no process variable that a flag does not name.
2. The same variable named by two flags is an error of the command line
   (9.1).

### 9.5 Exit status

| Status | When |
| --- | --- |
| 0 | `eval` wrote a value; `check` found no diagnostic |
| 1 | A step failed, with one or more diagnostics |

Other statuses are F12's (9.1).

## 10. Additions to the case format

Each addition is optional, and its absence leaves every existing case file
valid with its meaning unchanged.

### 10.1 The case field `returnsSchema`

This row is added to the table of `03-host.md` 3.10.3.

| Field | Holds | When absent |
| --- | --- | --- |
| `returnsSchema` | The expected result by schema (2.1): an object with the field `schema`, any JSON value, and the optional field `options`, an array of option names of F1 4 | No expected result by schema |

1. `returns` and `returnsSchema` count as one field for the header rule
   (`03-host.md` 3.10.2 item 4). A case that gives either replaces the
   header's `returns` or `returnsSchema`. After the header rule a case has at
   most one of them.
2. The option `absent-is-null` or `defaults-applied` is the `declare` failure
   `bad-type` of the case (2.1 item 2), not an invalid file. A schema that F1
   refuses is the `declare` failure `bad-type`.
3. The `schema` is not a host value: a JSON object with the key `$opaque` in it
   is data of the schema. Its numbers are read by `03-host.md` 3.10.3 item 5.
4. A case with `returnsSchema` is run as a case with `returns` is
   (`03-host.md` 3.10.8). With `"check": false`, the schema is mapped and no
   fit is made (2.1 item 6).

These make the file invalid: `returnsSchema` that is not an object; one with
no `schema`, or with a field other than `schema` and `options`; `options` that
is not an array of the three names of F1 4, or that names one twice; a case
with both `returns` and `returnsSchema` after the header rule.

### 10.2 The case field `load`

| Field | Holds | When absent |
| --- | --- | --- |
| `load` | `true` or `false`. `true` says that the case is run as a load: steps 0 to 4 of 5.2 | `false` |

1. A case with `load` true has an expected type after the header rule: a
   `returnsSchema`, or a `returns` that is type text. Otherwise the file is
   invalid. `returns` equal to `condition` makes it invalid.
2. A case with `load` true and `"check": false` makes the file invalid: a load
   always checks (5.2 step 2).
3. Such a case is run once, checked, and always reaches step 5 of
   `03-host.md` 3.10.5.
4. **Step 5a.** After a run that gives a value, the runner makes step 4 with
   a maximum of 1: it applies the conformance function (F4 7) to the value
   and the expected type, resolved in the case's environment, with its
   options. When it gives a diagnostic, the
   outcome is a failure of phase `run` with that one diagnostic: code
   `not-conforming`, class `value`, its path and message, no span. Its usage is
   the usage of the run.
5. In a case with `load` true, an expected diagnostic may state `"span": null`.
   It says that the diagnostic has no span, and it is compared: a diagnostic
   with a span is the difference `diagnostics`. In any other case `"span":
   null` makes the file invalid.
6. A case with `load` true whose `expect` states `json` compares the JSON text
   of the value the load returned (F6 11.2).
7. A case cannot state a decode: it is the binding's (6). The JSON text is its
   portable part.

### 10.3 The runner

1. A runner of an implementation that offers F10 runs `suite/config/`. A case
   there is run as `03-host.md` 3.10.8 item 5 says, with 10.1 item 4 and 10.2
   item 3.
2. A runner of an implementation that does not offer F10 does not run
   `suite/config/`. For it the fields `returnsSchema` and `load` are unknown
   fields (`03-host.md` 3.10.10).
3. In `suite/config/` the static codes are those of `suite/unions/` (F5 11.2
   item 3). No file of the directory ends `-check.json`: every case whose
   expectation needs the type rules states `type`, `sites`, a `check` code
   that is not static, or `load`.
4. **Every failure of step 4.** A load case compares the first failure of
   step 4 (10.2 item 4). The walk of every failure is tested by conform cases
   in the form of F8 7.5, with `max` and `failures`, against config types
   declared by schema under `closed` (`config-conform-all.json`).

### 10.4 The `outcome` command

The line of the `outcome` command (`suite/README.md`) changes so:

| Case | Line |
| --- | --- |
| A case with `returnsSchema` | The schema is the expected result. The line gains no field |
| A case with `load` true that fails at step 5a | `phase` `"run"`, and `diagnostics` one object with `code`, `span` null, `class`, `message`, and `path` |

### 10.5 The JSON Schema of the additions

These are added to the schema of `03-host.md` 3.10.11: the two properties below
in `$defs/fields`, `"returnsSchema"` and `"load"` in both lists of
`propertyNames`, and `null` as a second type of `span` in
`$defs/diagnostic` (allowed only where 10.2 item 5 says).

```json
{
  "fields": {"properties": {
    "returnsSchema": {
      "type": "object",
      "required": ["schema"],
      "properties": {
        "schema": true,
        "options": {
          "type": "array",
          "uniqueItems": true,
          "items": {"enum": ["closed", "absent-is-null", "defaults-applied"]}}},
      "additionalProperties": false},
    "load": {"type": "boolean"}}},
  "diagnostic": {"properties": {
    "span": {"type": ["array", "null"]}}}
}
```

### 10.6 Example

```json
{
  "groups": ["json"],
  "env": "{s: string}",
  "returnsSchema": {"schema": {"type": "object",
                               "properties": {"port": {"type": "integer"}},
                               "required": ["port"]},
                    "options": ["closed"]},
  "load": true,
  "cases": [
    {"name": "a key the schema does not list", "source": "{port: 1, prot: 2}",
     "expect": {"phase": "check",
                "diagnostics": [{"code": "result-mismatch", "span": [1, 11, 1, 15]}]}},
    {"name": "the same key through json.parse", "source": "json.parse(s)",
     "vars": {"s": "{\"port\": 1, \"prot\": 2}"},
     "expect": {"phase": "run",
                "diagnostics": [{"code": "not-conforming", "class": "value", "span": null,
                                 "path": ["prot"],
                                 "message": "the type does not allow key \"prot\""}],
                "steps": 4, "allocated": 5}}
  ]
}
```

## 11. Costs and what is not added

1. Mapping a schema, the fit, step 4, the JSON text and the decode charge no
   step and no unit. A program's counts do not depend on whether it is loaded,
   or on how T was given.
2. No syntax, no node kind, no reserved word, no library entry, no run code,
   no check code, no `declare` code, no parse code, no message template and no
   cost row is added.
3. Not in this package: imports (after 3.0); an indented JSON text; a JSON
   Schema produced from a type; YAML or TOML output; a library `merge` that
   goes deep; completion of keys from T (F12); the layout of diagnostics
   (F8, F12).

## 12. Earlier cases

1. No earlier case is contradicted. This package gives no text a new meaning:
   `import` stays reserved, and every program has the outcome it had.
2. No `until-f10-` file exists, and this package removes none and writes none.

## 13. Examples

### 13.1 The schema of the examples

The schema of `suite/config/config-schema-result.json`, under `closed`:

```json
{"type": "object",
 "properties": {
   "listen": {"type": "string"},
   "max_body": {"type": "integer", "minimum": 0},
   "log": {"enum": ["debug", "info", "warn"]},
   "backends": {"type": "array", "items": {"$ref": "#/$defs/backend"}},
   "tls": {"type": "object",
           "properties": {"cert": {"type": "string"}, "key": {"type": "string"}},
           "required": ["cert", "key"]}},
 "required": ["listen", "backends"],
 "$defs": {"backend": {"type": "object",
                       "properties": {"name": {"type": "string"},
                                      "url": {"type": "string", "format": "uri"}},
                       "required": ["name", "url"]}}}
```

M(S) under `closed` is `{backends: {name: string, url: string}[], listen:
string, log?: "debug" | "info" | "warn", max_body?: int, tls?: {cert: string,
key: string}}`. It is not faithful (3.3): `minimum` and `format` say more than
the type, so `max_body: -1` and `url: "not a uri"` check and load.

The union of `suite/config/config-unions.json` is the field `auth`, a `oneOf`
of `{kind: "none"}` and `{kind: "token", header: string}`, each with `kind` a
`const` and required. Under `closed` it is `{kind: "none"} | {header: string,
kind: "token"}`.

### 13.2 A config file

Each row is a case of `suite/config/config-examples.json`. The environment
declares `env: map<string>`; the expected type is `{auth: {kind: "none"} |
{header: string, kind: "token"}, backends: {name: string, url: string}[],
banner?: string, listen: string, max_body?: int}`.

```ts
// app.risor
const mb = 1024 * 1024;
const backend = (name, port) => {name, url: `http://${name}.internal:${port}`};

{
  listen: `0.0.0.0:${env?.PORT ?? "8080"}`,
  max_body: 8 * mb,
  backends: [backend("a", 9001), backend("b", 9002)],
  auth: {kind: "token", header: "X-Token"},
  banner: """
    Maintenance on Sunday,
    02:00 to 03:00 UTC.
    """
}
```

| # | Variation | Outcome |
| --- | --- | --- |
| 1 | `env` = `{PORT: "9000"}` | The value, with `listen` `"0.0.0.0:9000"` and `max_body` 8388608; 9 steps, 32 units. Its JSON text has the keys `auth`, `backends`, `banner`, `listen`, `max_body` in that order |
| 2 | `max_body` written `max_bod` | `result-mismatch` at `[7, 3, 7, 10]`, the key, before anything runs |
| 3 | The expected type given as the schema of the chapter, under `closed` | The same value; the same diagnostic for row 2 |
| 4 | `{...base, max_body: 16 * 1024 * 1024}`, with `base` the value of row 1 | The same value with `max_body` 16777216; 7 steps: 5 entries copied, 2 multiplications; 11 units |

### 13.3 Loads that fail after the run

Each row is a case of `suite/config/config-load.json` or
`config-load-unions.json`. The expected type is `{hosts?: string[], port:
int}` from a schema under `closed`, with `json` on and `s: string`.

| # | Program and data | Outcome |
| --- | --- | --- |
| 1 | `json.parse(s)`, `s` = `{"port": "80"}` | Step 4: path `["port"]`, `expected int, found "80"`, no span; the run's 3 steps and 4 units |
| 2 | The same, `s` = `{"port": 1, "debug": true}` | Step 4: path `["debug"]`, `the type does not allow key "debug"` |
| 3 | The same, `s` = `[1]` | Step 4: path `[]`, `expected {hosts?: string[], port: int}, found list` |
| 4 | `try { json.parse(s) } catch { {port: 0} }`, `s` = `{"port": "x"}` | Step 4: the `try` cannot catch it |
| 5 | `json.parse(s) as {port: int, hosts?: string[]}`, `s` = `{"port": "80"}` | Step 3: `not-conforming` with the span of the `as`; 6 steps |
| 6 | `{auth: a}`, `a: any` = `{kind: "oauth"}`, against the union of 13.1 | Step 4: path `["auth", "kind"]`, `expected "none" \| "token", found "oauth"` |

### 13.4 Every failure at step 4

A case of `suite/config/config-conform-all.json`. The type is M(S) of 13.1
under `closed`, and the value `{"listen": 8080, "backends": [{"name": "a"},
{"name": "b", "url": "u", "weight": 1}], "log": "loud", "max_bod": 1}`. With a
maximum of 10, step 4 reports five failures, in the order of F8 6.2:

| # | Path | Message |
| --- | --- | --- |
| 1 | `["backends", 0, "url"]` | `required field "url" is missing` |
| 2 | `["backends", 1, "weight"]` | `the type does not allow key "weight"` |
| 3 | `["listen"]` | `expected string, found 8080` |
| 4 | `["log"]` | `expected "debug" \| "info" \| "warn", found "loud"` |
| 5 | `["max_bod"]` | `the type does not allow key "max_bod"` |

With a maximum of 1, the first, which `v as T` also reports after 5 steps.
Without `closed`, failures 2 and 5 are not failures.
