# F1. Host types

This chapter specifies follow-on package F1. It adds three things to Core:

1. A variable's type declared by a JSON Schema, with three options (2 to 4).
2. The description of a type and of an environment, as data (5).
3. The contract for a type a binding takes from a host type (6).

It adds two optional fields to the case format (7). It adds no syntax, no
library entry, no run code, no message template and no cost row (8).

The chapters of Core are cited by file and section: `05-types.md` 3.9 is
section 3.9 of `spec/core/05-types.md`. Section numbers without a file are of
this chapter.

## 1. Conventions

### 1.1 Terms

| Term | Meaning |
| --- | --- |
| **Schema** | A JSON value given as a JSON Schema: an object, `true` or `false`. |
| **Root** | The schema the host gives. Every `$ref` is resolved in the root (3.7). |
| **Subschema** | A schema that stands inside another schema, under a keyword. |
| **Walked** | A subschema is walked when the mapping of 3 reads it. A keyword that 3 does not read is not walked, and nothing under it is looked at. |
| **JSON value** | null, `true`, `false`, a number, a string, an array or an object, as JSON gives them. A JSON number is read as a floating value by the rule of text to number in `01-language.md`. |
| **Valid** | A JSON value is valid by a schema when a JSON Schema validator accepts it, for any of the drafts in 2.2. |
| **M(S)** | The type the mapping gives schema S (3). |
| **Join, Fits, nonnull** | As `05-types.md` 4 defines them. |

### 1.2 Requirements

1. An implementation offers F1 only if it has the type checker of
   `05-types.md`. The mapping uses Join and Fits, and its result is a type.
2. An implementation that offers F1 offers all of sections 2 to 5 and 7. The
   rules of section 6 bind a binding that derives types from host types.

## 2. A variable declared by a JSON Schema

### 2.1 The declaration

This row is added to the table of `03-host.md` 3.3.1.

| Declaration | States | Portable form |
| --- | --- | --- |
| Variable by schema | A name, a schema, and a set of options (4) | The schema as a JSON value. The options as names |

1. The variable's type is M(S) with the options given (3, 4). The schema itself
   is not kept: after the environment is built, the variable is a variable of
   that type, the same as one declared in type text.
2. A variable declared by schema is a variable for every rule of `03-host.md`
   3.3: its name follows 3.3.3, and its clashes are those of 3.3.4. A variable
   by schema and a variable in type text with one name are `name-clash`.
3. A schema cannot name a type of the environment. It has no opaque type and
   no named type in it.
4. Any error in a schema, or in the type it gives, is the `declare` error
   `bad-type` (3.11). The `declare` diagnostic has no span (`03-host.md` 3.3.4
   item 5). Its message may say where in the schema the error is.
5. A run is not changed. A run uses a variable's name only (`03-host.md`
   3.3.1 item 3), and reads its value in place.

### 2.2 The drafts read

1. The mapping reads a schema the same way whatever draft it is written for.
   It does not read `$schema`.
2. The mapping is sound (2.3) for schemas of JSON Schema drafts 4, 6, 7,
   2019-09 and 2020-12, and for the Schema Object of OpenAPI 3.0 and 3.1.
   Where two drafts give one keyword different meanings, the mapping gives a
   type that is sound under each.

### 2.3 The promise of the mapping

| Part | Statement |
| --- | --- |
| Soundness | Every JSON value that is valid by S conforms (`05-types.md` 18.1) to M(S), with one exception: item 1. |
| Options | With options, every JSON value that is valid by S, and that the host then changed as the options state (4), conforms to M(S). |
| Wider, never narrower | Where the mapping cannot say what a schema says, it gives a wider type. A keyword it ignores only takes values away from the valid set, so the type is still sound. |

1. **The exception.** `"type": "integer"` gives `int` (3.3), and `int` holds
   whole numbers of magnitude at most 2^53 − 1 (`05-types.md` 2.1). A whole
   number of greater magnitude is valid by such a schema and does not conform
   to `int`. A host that supplies one breaks the precondition of
   `05-types.md` 18.4. An `enum` or a `const` has no such exception (3.4).
2. So the precondition of `05-types.md` 18.4 holds for a variable declared by
   schema whenever its value is valid by the schema, outside item 1.
3. The mapping checks types, not schemas. A value that conforms to M(S) need
   not be valid by S. A host that must enforce `minLength` or `pattern` runs
   its own validator.

| Schema | Type | A valid value, and why it conforms |
| --- | --- | --- |
| `{"type": "object", "properties": {"a": {"type": "string"}}}` | `{a?: string, ...}` | `{}`: `a` is optional. `{"a": "x", "b": 1}`: the rest is `any` |
| The same with `"required": ["a"]` | `{a: string, ...}` | `{"a": "x"}` |
| `{"type": ["string", "null"]}` as a required property | `a: string \| null` | `{"a": null}`: present, and null |
| The same as a property outside `required` | `a?: string \| null` | `{}` and `{"a": null}` |
| `{"type": "object", "properties": {"a": {"type": "string"}}, "additionalProperties": false}` | `{a?: string}` | `{}`. `{"b": 1}` is not valid, and does not conform |
| `{"type": "number", "multipleOf": 1}` | `float` | `2`, and also `1e300`, which is not `int` |

## 3. The mapping

### 3.1 The order of the rules

M(S) for a schema S is given by these steps, in order.

| Step | Rule |
| --- | --- |
| 1 | S is `true`: `any`. S is `false`: `never`. S is neither an object nor a boolean: `bad-type`. |
| 2 | S is an object below the root that has `$id`: `bad-type` (3.7). S has `$dynamicRef` or `$recursiveRef`: `bad-type`. |
| 3 | S has `$ref`: M(S) is M of the schema it names (3.7), with null added if S has `"nullable": true`. Every other keyword of S is ignored. Stop. |
| 4 | Make the list of **parts** of S, in this order, each that S has: the **kind part** (3.2), if S has `type`, `enum` or `const`; the `allOf` part (3.6); the `anyOf` part (3.6); the `oneOf` part (3.6). |
| 5 | The type is Pick(the parts) (3.6). With no part, it is `any`. |
| 6 | S has `"nullable": true`: Join(the type, `null`). |

1. A keyword is read only where a rule of this section reads it. A keyword
   that is not read is not checked: an error under it is no error.
2. Step 3 is sound for every draft of 2.2. Drafts 4 to 7 and OpenAPI 3.0
   ignore the siblings of `$ref`, and later drafts apply them as well, which
   only takes values away.
3. The schema `{}` has no part: `any`.

### 3.2 The kind part

| S has | The kind part |
| --- | --- |
| `const` | The values: the one value of `const`. Then the row "values" below. `enum` is ignored. |
| `enum`, and no `const` | The values: the elements of `enum`, which must be an array (`bad-type` if not). Then the row "values". An empty array gives `never`. |
| Values | The Join of the literal type (3.4) of each value that `type` allows (3.3). A value `type` does not allow is dropped. With no value left: `never`. |
| `type`, and neither `const` nor `enum` | The Join, over each name of `type`, of the type of that name (3.3). |

1. When S has no `type`, every value is allowed.
2. When S has `const` or `enum`, the keywords of 3.5 and 3.8 are not read: the
   values say more than they do.

### 3.3 `type`

`type` is a string, or a non-empty array of strings. Each string is one of the
seven names below. Anything else is `bad-type`. A name may be written twice.

| Name | Type | Allows a value that is |
| --- | --- | --- |
| `null` | `null` | null |
| `boolean` | `bool` | `true` or `false` |
| `integer` | `int` | a number with no fractional part, of any magnitude |
| `number` | `float` | a number |
| `string` | `string` | a string |
| `array` | E`[]`, with E by 3.5 | an array |
| `object` | a record by 3.8 | an object |

- `["integer", "number"]` is Join(`int`, `float`), which is `float`.
- `2.0` and `1e20` are allowed by `integer`. Names are case-sensitive:
  `"Integer"` is `bad-type`. `int`, `float` and `any` are not names here.
- The seven names together are not `any`: they give `bool | float | string |
  any[] | map<any> | null`.

### 3.4 The literal type of a JSON value

The literal type of a JSON value is the type the same literal has in a
program (`05-types.md` 7).

| JSON value | Literal type |
| --- | --- |
| null | `null` |
| `true`, `false` | `bool` |
| A number | `int` if it is whole and of magnitude at most 2^53 − 1, else `float`. By value, not by spelling: `2.0` is `int`, `9007199254740992` is `float` |
| A string | The literal set of that string |
| An array | The list of the Join of its elements' literal types. `[]` gives `never[]` |
| An object | The exact record with a required field for each key, of the literal type of its value. `{}` gives `{}` |

Every value conforms to its literal type, so the kind part is sound for `enum`
and `const`. A literal type deeper than 64 is `bad-type` (3.10).

### 3.5 Arrays

The element type E of an `array` is the Join of the types of each item below
that S has.

| S has | Joined into E |
| --- | --- |
| `prefixItems`, an array of schemas | M of each schema |
| `items`, a schema | M(`items`) |
| `items`, an array of schemas | M of each schema; then M(`additionalItems`) if S has it, else `any` |
| No `items` | `any` |

1. `prefixItems` that is not an array, `items` that is neither a schema nor an
   array, and an element of either that is not a schema are `bad-type`.
2. `additionalItems` is read only beside an array `items`. Beside a schema
   `items` it is ignored, as every draft does.
3. `items: false` with no `prefixItems` gives `never[]`: only the empty list.
4. Each draft constrains a subset of the elements by these keywords, so the
   Join is sound under every draft of 2.2.
5. `contains`, `minItems`, `maxItems`, `minContains`, `maxContains`,
   `uniqueItems` and `unevaluatedItems` are ignored.

### 3.6 `allOf`, `anyOf`, `oneOf`, and Pick

| Keyword | Its part |
| --- | --- |
| `allOf` | Pick(M of each branch) |
| `anyOf` | The Join of M of each branch, after the test of 3.9 |
| `oneOf` | The same as `anyOf` |

1. The value of each is a non-empty array of schemas. Anything else is
   `bad-type`.
2. **Pick**(T1, ..., Tn), for a list of types in order:
   1. Drop every Ti that is `any`. With none left: `any`.
   2. If every type left is one record alternative and nothing else (an exact
      record, an open record, or `map<T>`): their Meet, folded from the left:
      Meet(Meet(T1, T2), T3) and so on.
   3. Otherwise: the first type left that fits every other (`05-types.md`
      4.2) by a fit that does not use any. If none: the first type left.
3. **Meet**(A, B) of two records:

   | Part | Rule |
   | --- | --- |
   | Keys | Every key that A or B lists |
   | A key k | Let a be A's field type for k if A lists k, else A's rest; b the same for B. The field's type is Pick(a, b), so records under a shared key meet |
   | Its mark | Required when A or B lists k as required, else optional |
   | The rest | Pick(the rest of A, the rest of B) |

4. **Soundness.** A valid value is valid by each part and by each branch of
   `allOf`, so each one's type is sound for it. Step 1 drops only `any`, which
   says nothing. In step 3 the type picked is sound alone. In a Meet, a value
   that conforms to A and to B is a map; each key it has conforms to both a
   and b, and so to Pick(a, b) by the same argument; a key required by either
   is present. So the value conforms to the Meet.
5. A type that is a record with other alternatives, such as `{a: int, ...} |
   null`, does not meet: step 3 applies. A part with no `type` is `any` and
   is dropped: the object keywords of a schema with no `type` are not read
   (3.1 item 1).
6. A key that an exact record lacks has the rest `never` on that side, so in
   the Meet it has type `never`: `{a?: int}` met with `{b: string, ...}` is
   `{a?: int, b: never}`, which no value conforms to, as no value is valid by
   both.
7. The difference between `anyOf` and `oneOf` is dropped. `discriminator` is
   ignored.
8. **Marks** (4.1). A record may carry the mark *closable*. A Meet is
   closable when every record met is closable. Step 3 passes the type it
   picks through with its marks. A Join of two records is closable when both
   are.

| Schema | Type | Why |
| --- | --- | --- |
| `{"allOf": [{"minLength": 1}, {"type": "string"}]}` | `string` | The `any` part is dropped |
| `{"allOf": [{"type": "string"}, {"type": "integer"}]}` | `string` | Neither fits the other: the first |
| `{"allOf": [{"type": "string"}, false]}` | `never` | `never` fits everything |
| `{"type": ["string", "integer"], "anyOf": [{"type": "string"}]}` | `string` | The `anyOf` part fits the kind part |
| `{"type": "object", "oneOf": [{"required": ["a"]}, {"required": ["b"]}]}` | `map<any>` | Each branch is `any`, and is dropped |
| `allOf` of a `$ref` to `{id: int, ...}` and of `{"type": "object", "properties": {"b": {"type": "string"}}, "required": ["b"]}` | `{b: string, id: int, ...}` | The two records meet |
| `allOf` of `{"type": "object", "additionalProperties": {"type": "number"}}` and of a record `{a: int, ...}` | `{a: int, ...float}` | `a`: Pick(`float`, `int`) is `int`; the rest: Pick(`float`, `any`) is `float` |

### 3.7 `$ref`

1. The value of `$ref` is a string. It must be `#`, or `#` followed by a JSON
   Pointer (RFC 6901). Any other value is `bad-type`: a reference to another
   document, an absolute URI, and a plain-name fragment such as `#node`.
2. The text after `#` is percent-decoded first (RFC 3986): each `%` followed
   by two hexadecimal digits is one byte, and the bytes must be UTF-8. A `%`
   not followed by two hexadecimal digits, or bytes that are not UTF-8, are
   `bad-type`.
3. The pointer is then read token by token from the root. In each token `~1`
   is `/` and `~0` is `~`; a `~` followed by any other character is
   `bad-type`. In an object a token names a member. In an array a token is an
   index: `0`, or a digit from 1 to 9 followed by digits, less than the length.
   A token that names nothing is `bad-type`.
4. `#` alone names the root.
5. The schema named must be an object or a boolean (3.1 step 1).
6. A pointer may name any place in the root: a member of `$defs` or of
   `definitions`, a property's schema, an element of an array. `$defs` and
   `definitions` are not read except through a `$ref`.
7. **A cycle is an error.** A `$ref` that names a schema whose mapping is in
   progress, the root included, is `bad-type`. A type cannot refer to itself
   (`05-types.md` 2). A schema named by two `$ref`s that are not nested is no
   cycle.
8. A `$ref` that is not walked is not resolved. A recursive definition that
   nothing walks is no error.
9. `$id` on the root is ignored. `$id` on a walked schema below the root is
   `bad-type`: the mapping resolves every pointer in the root and does not
   read embedded resources. `$anchor` and `$dynamicAnchor` are ignored.
   `$dynamicRef` and `$recursiveRef` on a walked schema are `bad-type`.

### 3.8 Objects

The record of an `object`:

| Part | Rule |
| --- | --- |
| R0, the open rest | If S has `additionalProperties`: Join of M(`additionalProperties`) and of M of each value of `patternProperties`. Otherwise `any`. |
| A key of `properties` | A field of type M(its schema). Required if `required` names it, else optional; the options of 4 change this. |
| A name in `required` that `properties` does not have | A required field of type R0. |
| The rest | R0. Under the option `closed`, the record may be closable, and Close later makes its rest `never` (4.1). |

1. `properties` must be an object, and each of its values a schema.
   `required` must be an array of strings. `patternProperties` must be an
   object whose values are schemas. Anything else is `bad-type`, where the
   keyword is read.
2. `patternProperties` is read only when S has `additionalProperties`. Without
   it, R0 is `any` and the patterns add nothing.
3. A key of `properties` that a pattern also matches is typed by `properties`
   alone. The value must be valid by both, so this is sound.
4. A name written twice in `required` is one name.
5. `minProperties`, `maxProperties`, `propertyNames`, `dependentRequired`,
   `dependentSchemas`, `dependencies` and `unevaluatedProperties` are ignored.
   So `unevaluatedProperties: false` does not make the record exact.
6. A record prints by `05-types.md` 3.9: no fields and rest R prints `map<R>`,
   no fields and rest `never` prints `{}`.

| Schema (each with `"type": "object"`) | Type |
| --- | --- |
| nothing more | `map<any>` |
| `"additionalProperties": false` | `{}` |
| `"additionalProperties": {"type": "integer"}` | `map<int>` |
| `"properties": {"a": {"type": "string"}}, "required": ["a"]` | `{a: string, ...}` |
| `"properties": {"a": {"type": "string"}}` | `{a?: string, ...}` |
| `"properties": {"a": false}` | `{a?: never, ...}` |
| `"required": ["id"]` | `{id: any, ...}` |
| `"required": ["id"], "additionalProperties": false` | `{id: never}` |
| `"patternProperties": {"^x": {"type": "integer"}}, "additionalProperties": {"type": "string"}` | `map<int \| string>` |

### 3.9 Records that would form a tagged union

Follow-on F5 gives a tagged union when the records of the branches of `anyOf`
or `oneOf` have a tag. Until F5 is present, such a schema is an error, so that
F5 only adds.

1. Take the record of each branch type of one `anyOf` or one `oneOf` that has
   a map alternative.
2. A key is a **tag** of these records when every one of them has it as a
   required field, its type in each is a literal set and nothing else, and no
   string is in the sets of two records.
3. Two or more records that have a tag are `bad-type`. Otherwise the branches
   join (3.6). The cases that pin this error are in
   `suite/host-types/until-f05-tagged-branches.json`, which F5 removes.
4. The test is made at each `anyOf` and `oneOf`, after its branches are
   mapped. A nested `anyOf` joins first, and its record then takes part in the
   test of the one outside it.

| Branches | Result |
| --- | --- |
| `{kind: "circle", r: float, ...}` and `{kind: "rect", w: float, ...}`, `kind` required in both | `bad-type` until F5 |
| `kind` outside `required` in one branch | The Join: `{kind?: "circle" \| "rect", r?: any, w?: any, ...}` |
| `kind` typed `string` in one branch | The Join: `{kind: string, ...}` |
| `kind` of `"a" \| "b"` and of `"b"` | The sets share `"b"`: the Join |
| `kind` of `"a" \| null` and of `"b"` | Not a literal set alone: the Join |
| `{a: int, ...}` and `{b: string, ...}` | No common key: the Join, `{a?: any, b?: any, ...}` |

### 3.10 Depth

1. The **depth** of a schema is 0 when it walks no subschema, and otherwise 1
   more than the greatest depth of the subschemas it walks. The schema a
   `$ref` names counts as a subschema of the schema that has the `$ref`.
2. The subschemas a schema walks are those 3.1 to 3.8 read: the schema its
   `$ref` names; the branches of `allOf`, `anyOf` and `oneOf`; and, in a kind
   part with no `const` and no `enum`, the schemas of `prefixItems`, `items`,
   `additionalItems`, `properties`, `additionalProperties` and
   `patternProperties` that 3.5 and 3.8 read.
3. A schema of depth more than 64 is `bad-type`. A cycle (3.7 item 7) is
   reported as `bad-type` before any depth.
4. A type that the mapping gives of depth more than 64 (`05-types.md` 2.5) is
   `bad-type`. Only a literal type (3.4) can pass 64 while its schema does not.
5. The depth of a schema does not depend on where it stands, so an
   implementation may map a schema named by many `$ref`s once and share the
   type. It must not copy it: two `$ref`s to one definition at each of k
   levels would make 2^k copies.
6. The type of a variable by schema may have depth 64. Type text in a case's
   `env` reaches only 63, since the braces of the record count one level of
   text depth (`05-types.md` 20.5). Each type text of a description is read
   alone (5.4), so a description holds a type of depth 64.

| Schema | Depth | Result |
| --- | --- | --- |
| 64 `array` schemas, each the `items` of the one outside, around `{"type": "integer"}` | 64 | `int` followed by 64 `[]` |
| 65 of them | 65 | `bad-type` |
| 64 `anyOf`, each with one branch, around `{"type": "integer"}` | 64 | `int` |
| A chain of 64 `$ref` to `{"type": "integer"}` | 64 | `int` |
| 63 `array` schemas around a `$ref` to an `array` of `integer` | 65 | `bad-type`: the `$ref` is a level, though the type has depth 64 |
| `{"const": v}`, v 64 nested arrays with `[]` innermost | 0 | `never` followed by 64 `[]` |
| The same with 65 | 0 | `bad-type`: the type has depth 65 |

### 3.11 Errors

Every error of a schema is the `declare` error `bad-type`, where the keyword
is read (3.1 item 1).

| Error | Section |
| --- | --- |
| A schema that is not an object or a boolean: null, a number, a string, an array | 3.1 |
| `type` that is not a string or a non-empty array of strings, or a name that is not one of the seven | 3.3 |
| `enum` that is not an array | 3.2 |
| `properties`, `patternProperties` or `required` of the wrong form | 3.8 |
| `items`, `prefixItems` or `additionalItems` of the wrong form | 3.5 |
| `allOf`, `anyOf` or `oneOf` that is not a non-empty array | 3.6 |
| `$ref` that is not a string, not local, malformed, naming nothing, or naming what is not a schema | 3.7 |
| A cycle of `$ref` | 3.7 |
| `$id` below the root; `$dynamicRef`; `$recursiveRef` | 3.7 |
| Records of `anyOf` or `oneOf` that have a tag, until F5 | 3.9 |
| A schema deeper than 64, or a type deeper than 64 | 3.10 |

### 3.12 Keywords

Each keyword of JSON Schema and of the OpenAPI Schema Object, and what the
mapping does with it. A keyword not in this table is ignored, and ignoring it
is sound: a keyword of a validator only takes values away.

| Keyword | Read as | Ignored when |
| --- | --- | --- |
| `type` | 3.3 | S has `$ref` |
| `enum`, `const` | 3.2, 3.4. `const` wins over `enum` | S has `$ref`. `enum` when S has `const` |
| `nullable` | `true` adds `null` (3.1 step 6). OpenAPI 3.0 | Any value but `true` |
| `items`, `prefixItems`, `additionalItems` | 3.5 | Not in a kind part with `array` |
| `properties`, `required`, `additionalProperties`, `patternProperties` | 3.8 | Not in a kind part with `object` |
| `default` | The option *defaults-applied* (4.3) | Without that option |
| `allOf`, `anyOf`, `oneOf` | 3.6 | S has `$ref` |
| `$ref` | 3.7 | Never on a walked schema |
| `$defs`, `definitions` | Read only through `$ref` | Always, as keywords |
| `$id` | `bad-type` below the root | On the root |
| `$dynamicRef`, `$recursiveRef` | `bad-type` | Never on a walked schema |
| `$schema`, `$anchor`, `$dynamicAnchor`, `$vocabulary`, `$comment` | — | Always |
| `title`, `description`, `examples`, `example`, `deprecated`, `readOnly`, `writeOnly` | — | Always |
| `format`, `contentEncoding`, `contentMediaType`, `contentSchema` | — | Always. `format` gives no type: a `date-time` is a `string` |
| `multipleOf`, `minimum`, `maximum`, `exclusiveMinimum`, `exclusiveMaximum` | — | Always. `multipleOf: 1` does not give `int` |
| `minLength`, `maxLength`, `pattern` | — | Always |
| `minItems`, `maxItems`, `uniqueItems`, `contains`, `minContains`, `maxContains`, `unevaluatedItems` | — | Always |
| `minProperties`, `maxProperties`, `propertyNames`, `dependentRequired`, `dependentSchemas`, `dependencies`, `unevaluatedProperties` | — | Always |
| `not`, `if`, `then`, `else` | — | Always |
| `discriminator`, `xml`, `externalDocs` (OpenAPI) | — | Always. The tag of a union is found from the branches (F5) |
| Any other keyword, `x-` keywords among them | — | Always |

## 4. The three options

A host gives options with each schema. Each is off unless given. They state
what the host did to a value before a run. They apply to every object schema
the mapping meets in that schema: nested, in `items`, in a branch, and through
`$ref`.

| Option | The host has | Effect on the record of 3.8 |
| --- | --- | --- |
| `closed` | removed, from every object whose mapped record the option made exact, each key that record does not list | By 4.1: the record is closable, and Close makes its rest `never` |
| `absent-is-null` | put null under every key of `properties` that `required` does not name and the value lacks | Such a field is required, of type Join(T, `null`) |
| `defaults-applied` | put the `default` under every key of `properties` that has one, that `required` does not name and the value lacks | Such a field is required, of type Join(T, the literal type of the `default`) |

### 4.1 `closed`

1. Under `closed`, the record of an object schema that has `properties` and
   has neither `additionalProperties` nor `patternProperties` is
   **closable**: it is mapped as 3.8 says, with its open rest R0 (`any`), and
   carries the mark *closable*. No other record is closable.
2. Records meet and join as they are (3.6 items 3 and 8): a Meet or a Join is
   closable when every record in it is closable. A closable record met with
   one that is not closable, such as one with an explicit
   `additionalProperties`, gives a record that is not closable.
3. **Close**(T) makes the rest of each closable record of T `never` and drops
   its mark, at every depth: in T's record alternative, in its field types and
   rest, and in element types.
4. Close is applied once, to the type of the root, after the whole mapping.
   It is not applied to a property, an element, a branch or the target of a
   `$ref` on its own: those are closed when the root's type is.
5. So records are closed after every Meet and Join. `allOf` of a schema with
   property `a` and one with property `b` gives `{a?: string, b?: int}`.
   `anyOf` of the same two gives `{a?: any, b?: any}`: each branch is open
   when they join, so a key that only one branch lists may hold any value
   that the other branch allows.
6. An explicit `additionalProperties` (a schema, `true` or `false`) or
   `patternProperties` keeps a record from being closable. A schema with no
   `properties` is not closable: `{"type": "object"}` stays `map<any>`.
7. A name of `required` that `properties` lacks keeps the type R0 of 3.8: `{a:
   string, z: any}` for `properties` `a` and `required` `["a", "z"]`.
8. **Soundness.** Take a value valid by S, and remove from each object whose
   record in M(S) Close made exact each key that record does not list. Every
   key left is listed in that record, and its value is valid by the schemas
   whose open records were met or joined into it, so it conforms by 3.6 item
   4. Closing a record before it is met or joined would not be sound: a
   branch closed before a Join gives a key the other branch's type, though
   the first branch, open, allows any value there.

| Schema, under `closed` | Type |
| --- | --- |
| `allOf` of `properties` `a` (string) and of `properties` `b` (integer) | `{a?: string, b?: int}` |
| The same with `additionalProperties: {"type": "integer"}` in the second | `{a?: string, b?: int, ...int}` |
| The same with `additionalProperties: false` in the second | `{a?: never, b?: int}`: the second forbids `a` |
| `anyOf` of the same two | `{a?: any, b?: any}` |
| `allOf` of two schemas whose property `p` is an object, with `x` and with `y` | `{p: {x?: int, y?: string}}` |

### 4.2 `absent-is-null`

1. It changes only a key of `properties` outside `required`. A field already
   nullable stays as it is. A property whose schema is `false` gives `{a:
   null, ...}`.

### 4.3 `defaults-applied`

1. The `default` is read from the property's own schema object, also when
   that object has a `$ref`.
2. The literal type of the `default` is joined because a default need not be
   valid by its schema. A `default` of null gives `a: T | null`.
3. With both `absent-is-null` and `defaults-applied`, a property with a
   `default` takes the row of `defaults-applied`, and the others the row of
   `absent-is-null`.
4. A property in `required` is not changed by either option.

## 5. Describing types and environments

### 5.1 The description of a type

1. The description of a type is its printed text, by `05-types.md` 3.9. It
   reads back as the same type.
2. A type from a schema or from a host type is described the same way. The
   schema and the host type are not kept.
3. **No schema is given back.** This package has no mapping from a type to a
   JSON Schema.

### 5.2 Printing a signature

A signature (`05-types.md` 3.6) is printed so:

| Part | Printed as |
| --- | --- |
| The signature | `(`, the parameters separated by `, `, `) => `, the result type; then ` !` if it has `!`; then ` where ` and the bounds separated by `, ` if it has any |
| A parameter | `...` if it is a rest parameter, its name, `?` if it is optional, `: `, its type |
| A bound | The `Var`, `: `, the bound. The bounds in code point order of their `Var` |
| A type | By `05-types.md` 3.9, with the row below |
| A type variable | Its letter. Among the alternatives of a type, type variables stand after the opaque names, in code point order, and before `null`: `int \| T`, `(T \| null)[]` |

A method is printed as the signature of its text without the receiver. The
parameter names are kept. `(code:string)=>float!` prints as `(code: string) =>
float !`.

### 5.3 The description of an environment

The description of an environment is a JSON object with exactly these keys.

| Key | Value |
| --- | --- |
| `version` | The environment's version, `major.minor` as a string |
| `variables` | An object: each variable's name, and its type printed (5.1). Variables declared in type text and by schema alike |
| `functions` | An object: each host function's name, with its namespace prefix, and its signature printed (5.2) |
| `opaque` | An object: each opaque type's name, and an object of its methods: each method's name and its signature printed (5.2). A type with no method has `{}` |
| `removed` | An array of the names of the removed library entries (`03-host.md` 3.3.1), in code point order |
| `reject` | An array of the rejected check classes: the whole set, in code point order, so a reader need not know the default |
| `limits` | An object with the keys `steps`, `allocation` and `depth`. Each is a whole number, and `steps` and `allocation` are null when the limit is off |

1. It describes the whole environment: a child's description holds what its
   parents declare, with the child's replacements.
2. The deadline is not described. It bounds a run on one host, and a
   description is for check.
3. Documentation of a function is not in the description.
4. A later package adds keys: F4 adds named types, F6 the groups that are on.
5. **The text of a description** is its JSON form by `01-language.md` 12: no
   white space, the keys of every object in code point order, each string
   escaped as that section says, and each number in its rendering. Two
   implementations give the same text for one environment.

This environment, with `order` declared by the schema of 2.3's second row and
a function `rate`:

```json
{"functions":{"rate":"(code: string) => float !"},"limits":{"allocation":1000000,"depth":200,"steps":1000000},"opaque":{},"reject":["constant-condition"],"removed":[],"variables":{"order":"{a: string, ...}"},"version":"3.0"}
```

### 5.4 An environment from a description

1. Every binding that offers F1 builds an environment from a description. Each
   type text in it is read as declared type text (`05-types.md` 19.5) on its
   own, not inside a record: a variable's type, a signature, a method's text.
2. Its errors are those of `03-host.md` 3.3.4 for what the description
   declares: `bad-type`, `bad-signature`, `bad-name`, `name-clash`,
   `bad-class`, `bad-limit`, `unsupported-version`. A description that is not
   a JSON object of the form of 5.3 is refused by the binding with an error of
   its own.
3. **Round trip.** An environment E2 built from the description of E1 has the
   same description as E1, and checks every program as E1 does: the same
   type, sites and diagnostics.
4. E2 has no implementation of any host function or method. A compile against
   it fails with `no-implementation` (`03-host.md` 3.3.4) while it declares
   one.

## 6. Types from host types

### 6.1 The portable contract

A binding may give a host a type derived from a type of the host language.
The rules of 6.2 to 6.4 are binding rules. Every binding that derives types
meets these.

| Matter | Rule |
| --- | --- |
| Soundness | The derived type is one that every value of the host type conforms to (`05-types.md` 18.1), as the binding presents that value to a run (`03-host.md` 3.4). Then the precondition of `05-types.md` 18.4 holds by construction, except for a host integer that cannot enter (the next row). |
| Numbers | A host integer type gives `int`. A host floating type gives `float`. A host integer outside ±(2^53 − 1) cannot enter (`03-host.md` 3.4.4), and the run fails where it reads it. |
| A record | A host type presented as a map gives a record. Its rest is `never` when the binding presents no key but its fields. |
| A field's name | The key under which the binding presents the field. The binding states how it chooses it. |
| A field's mark | Optional exactly when the binding may present the field as absent. Required otherwise. |
| Null | `\| null` exactly when the binding may present null for the value. |
| Opaque | A host type the host registered as opaque gives that opaque type's name. It is never guessed. |
| Unsupported | A host type that no rule of the binding covers is an error when the type is derived: the `declare` error `bad-type`, or an error of the host's own build where the binding derives at build time. |
| Limits | A derived type deeper than 64 (`05-types.md` 2.5) is `bad-type`. A host type that contains itself, directly or through others, is `bad-type`: a type cannot refer to itself. |
| No union | No binding derives a tagged union in this package. |
| Description | A derived type is described by its printed text (5.1), like any other. |

### 6.2 Go (binding rules)

The Go binding derives a type with `TypeOf`. The rows are tried in order: the
first that applies gives the type.

| Go | Type |
| --- | --- |
| A type that implements the binding's `Opaque` interface, a struct type among them | Its opaque name. This row is tried before every other. The name is asked of the type's zero value, so the method that gives it must not read its receiver |
| A type that implements `encoding.TextMarshaler` | `string`, presented as the text its `MarshalText` gives. A value whose `MarshalText` returns an error is presented as null, which breaks the declared type (`03-host.md` 3.4.8) |
| `bool`, `string` | `bool`, `string` |
| Every integer type | `int` |
| `float32`, `float64` | `float` |
| `[]T`, `[N]T` | `T[]`. A nil slice is presented as an empty list |
| `map[string]T` | `map<T>`. A nil map is presented as an empty map |
| `*T` | `T \| null` |
| Every interface type, `any` among them | `any`. The value an interface holds is presented by the rules of its own type, and a nil interface as null |
| A struct | An exact record of the fields it presents, by the rules below |
| Anything else | `bad-type` |

1. A struct presents its exported fields. A field tagged `json:"-"` or
   `risor:"-"` is not presented and is not in the record.
2. A field's name is its `risor` tag, else its `json` tag name, else its Go
   name.
3. A field whose `json` tag has `omitempty` or `omitzero` is optional, and is
   presented as absent when it holds what `encoding/json` would leave out.
   Every other field is required.
4. The fields of an embedded struct are presented as `encoding/json` presents
   them.
5. One shape is the exception to item 4: an exported field reached through
   an embedded pointer to an unexported struct type, `struct{ *inner }`. It
   takes part in deciding which field a name means, but it is not presented
   and is not in the record.

### 6.3 Rust (binding rules, provisional)

The Rust binding derives a type with `#[derive(risor::Host)]`, at build time.
The crate has no library dependency, so these rules name no other crate. This
section is provisional: the Rust binding fixes its details when it builds the
derive.

| Rust | Type |
| --- | --- |
| A type registered as opaque | Its opaque name |
| `bool`, `String`, `&str` | `bool`, `string` |
| Every integer type | `int` |
| `f32`, `f64` | `float` |
| `Vec<T>`, `[T]`, `[T; N]` | `T[]` |
| `HashMap<String, T>`, `BTreeMap<String, T>` | `map<T>` |
| `Option<T>` | `T \| null` |
| The crate's own owned value type | `any` |
| A struct with the derive | An exact record of the fields it presents, by the rules below |
| A Rust `enum`, and anything else | An error of the host's build: the trait is not implemented |

1. Fields are named and marked with `#[risor(...)]` only.
2. `#[risor(rename = "k")]` presents the field under the key `k`. Without it,
   the key is the field's Rust name.
3. `#[risor(skip)]`: the field is not presented and is not in the record.
4. `#[risor(absent_if_none)]` on a field of type `Option<T>`: an optional
   field `f?: T`, presented as absent when it is `None`. Every other field is
   required.

### 6.4 TypeScript (binding rules)

TypeScript types are erased before a program runs. The TypeScript binding
derives no type. A host gives type text, a schema, or a description.

## 7. Additions to the case format

Each addition is optional, and its absence leaves every existing case file
valid with its meaning unchanged.

### 7.1 The case field `schemas`

This row is added to the table of `03-host.md` 3.10.3.

| Field | Holds | When absent |
| --- | --- | --- |
| `schemas` | An object. Each key is a variable's name. Each value is an object with the field `schema`, any JSON value, and the optional field `options`, an array of option names (4) | No variable is declared by schema |

1. Each key declares a variable by schema (2.1), with the options given. The
   variables of a case are those of `env` and those of `schemas` together.
2. A name in both is `name-clash`. A name that is not a `Name`, or that is a
   reserved word, is `bad-name` (`03-host.md` 3.3.4). These are `declare`
   failures of the case, not invalid files.
3. A schema that 3 refuses is the `declare` failure `bad-type`.
4. `schemas` may stand in the header (`03-host.md` 3.10.2). A case's
   `schemas` replaces the header's whole.
5. A key of `vars` may name a variable of `schemas`. Item 9 of 3.10.10 reads:
   a key of `vars` that is neither a field of `env` nor a key of `schemas`.
6. Its declarations join those of 3.10.3 item 3.
7. The `schema` is not a host value. A JSON object with the key `$opaque`
   inside it is data of the schema, and item 7 of 3.10.3 does not apply. Its
   numbers are read by item 5 of 3.10.3.

These make the file invalid:

1. `schemas` that is not an object, or a value of it that is not an object.
2. A value with no `schema`, or with a field other than `schema` and
   `options`.
3. `options` that is not an array of the three names, or that names one twice.

### 7.2 The result field `description`

This row is added to the result table of `03-host.md` 3.10.6.

| Field of a result | Compared with | How |
| --- | --- | --- |
| `description` | The description of the case's environment (5.3) | As JSON data: objects with the same keys and equal values, in any key order; arrays element by element; strings code point by code point; numbers by value |

1. The description is taken after step 2 of 3.10.5. It needs no check of the
   program, and no run.
2. The difference is named `description`. It stands after `outcome` in the
   order of 3.10.8 item 2.
3. `description` in an expected failure makes the file invalid.

### 7.3 The runner

1. A runner of an implementation that offers F1 runs `suite/host-types/`.
   Every case there is run checked, once (3.10.8 item 5).
2. A runner of an implementation that does not offer F1 does not run
   `suite/host-types/`. For it a file with these fields is invalid, as a file
   with any unknown field is (`03-host.md` 3.10.10). No runner needs to know
   the fields of a package it lacks.

### 7.4 The `outcome` command

The line of the `outcome` command (`suite/README.md`) gains one field.

| Field | Value |
| --- | --- |
| `description` | When the case has `schemas` and its environment was built: the description (5.3), as a JSON object in its text (5.3 item 5). It stands after `name`, before `type` |

### 7.5 The JSON Schema of the additions

These are added to the schema of `03-host.md` 3.10.11: the property below in
`$defs/fields`, `"schemas"` in both lists of `propertyNames`, and
`description` in `$defs/result`.

```json
{
  "fields": {"properties": {
    "schemas": {
      "type": "object",
      "additionalProperties": {
        "type": "object",
        "required": ["schema"],
        "properties": {
          "schema": true,
          "options": {
            "type": "array",
            "uniqueItems": true,
            "items": {"enum": ["closed", "absent-is-null", "defaults-applied"]}}},
        "additionalProperties": false}}}},
  "result": {"properties": {
    "description": {
      "type": "object",
      "required": ["version", "variables", "functions", "opaque", "removed",
                   "reject", "limits"],
      "additionalProperties": false}}}
}
```

### 7.6 Example

```json
{
  "schemas": {"order": {"schema": {
    "type": "object",
    "properties": {"id": {"type": "integer"}, "note": {"type": "string"}},
    "required": ["id"]}}},
  "cases": [
    {"name": "the mapped type", "source": "order",
     "expect": {"type": "{id: int, note?: string, ...}", "sites": []}},
    {"name": "an optional property is maybe-absent", "source": "order.note",
     "expect": {"phase": "check",
                "diagnostics": [{"code": "maybe-absent", "span": [1, 7, 1, 11]}]}},
    {"name": "a valid value", "source": "order?.note ?? \"none\"",
     "vars": {"order": {"id": 1}}, "expect": {"value": "none"}}
  ]
}
```

## 8. Costs and what is not added

1. The mapping and the description are made when the environment is built or
   described. They charge no step and no allocation unit, and add no row to
   the cost table of `02-evaluation.md`. A program's counts do not depend on
   whether its variables were declared in type text or by schema.
2. No run code, no check code, no `declare` code, no parse code, no message
   template, no reserved word and no library entry is added.
3. Not in this package: a named type declared by schema (F4); the tagged union
   a schema's branches give (F5, which turns the error of 3.9 into a type); an
   expected result given by schema, and a closed schema for a config file
   (F10); a JSON Schema produced from a type.
