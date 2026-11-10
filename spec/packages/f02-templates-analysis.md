# F2. Templates and analysis

This chapter is normative. It specifies follow-on package F2: templates, which
are host texts with `${{ }}` parts, and analysis, which reads a program or a
template without an environment and reports the names it reads, the paths it
reads through them and the calls it makes. Both are host API. Like
`03-host.md`, this chapter names no type or function of any binding.

The package only adds. It changes no rule, cost row, run code or message of
Core, and no Core case. A program text is parsed as before: `${{ x }}` given as
a program is still `unexpected-character` at the `$`.

Other chapters: `01-language.md` (tokens, grammar, nodes, spans, scope,
rendering), `02-evaluation.md` (order and cost), `03-host.md` (stages,
diagnostics, positions, case files), `05-types.md` (the checker).

## 1. Terms

| Term | Meaning |
| --- | --- |
| Template | A host text in which `${{ }}` parts stand (2) |
| Part | `${{`, then one expression, then `}}` (2.2) |
| Text run | A maximal stretch of a template that is not in a part (2.1) |
| Single | A template that is exactly one part, with no text run before or after it |
| Mode | How a host takes the program of a template: `string` or `value` (2.5) |
| Analysis | The stage of section 3. It gives a report (3.3) |
| Free | A name at a place where no binding of the program is in scope (3.2) |
| Read | An `Ident` node whose name is free (3.2) |

## 2. Templates

### 2.1 Reading the text

Template parse is a stage of its own, beside the four of `03-host.md` 3.2.1.

| Stage | Takes | Gives | Fails with a list of diagnostics of phase |
| --- | --- | --- | --- |
| Template parse | A template text | A parsed template: its segments (2.4), whether it is single, and its program in each mode (2.5) | `parse` |

It needs no environment. A text gives the same parsed template, or the same
`parse` diagnostics, in every environment.

1. A template is a sequence of code points. A host passes it as it passes the
   source text of a program (`03-host.md` 3.4.5).
2. Nothing is removed from the text. A U+FEFF at its start is a code point of
   the text and is counted in positions.
3. The text is read from its start, left to right. At each place the first rule
   of this table that holds applies.

| # | The text at this place | Effect |
| --- | --- | --- |
| 1 | The four code points `$${{` | The text run goes on with the three code points `${{`. Reading goes on after the four |
| 2 | The three code points `${{` | A part opens (2.2) |
| 3 | Any other code point | The code point belongs to the text run |

4. `$${{` is the only escape of a text run. Every other code point stands for
   itself: `\`, `` ` ``, `"`, `'`, `$`, `{`, `}`, `}}`, `"""`, `r"`, a line end
   and U+000D among them. So `\${{ x }}` is the text `\` and then a part, and
   `$$` not followed by `{{` is two `$`.
5. A text run's value is its code points, with each `$${{` read as `${{`.
   Line ends are kept as they are written: U+000D is not changed.
6. A `$` directly before a part cannot be written by rule 1. A host writes it
   inside the part: `${{ "$" + price }}`.

### 2.2 Parts

1. After `${{`, the text is divided into tokens, white space and comments by
   the lexer of `01-language.md` section 2, as the text of a program is.
2. The `{` and `}` tokens of the part are counted. A `${` of a template literal
   inside the part, and that literal's own `}`, are inside that literal and are
   not counted, as in `01-language.md` 2.8 rule 5. A string, a text part of a
   template literal and a comment hold no token: `}}` there does not close the
   part.
3. The part's tokens end at the first `}` token that is not matched by an
   earlier `{` token of the part. This `}` is the **closing brace**.
4. When the code point directly after the closing brace is `}`, the two are the
   **closing delimiter** `}}`, and the part ends after it. Where a diagnostic
   names the closing delimiter, its span is both code points.
5. Otherwise the part ends after the closing brace. The closing brace is then
   `unexpected-token`, with the span of that `}`, met where the parser reaches
   it (2.3). The text run goes on after it.
6. When the end of the text comes before the closing brace, the part is
   unterminated: `unterminated-string`, with the span from the `$` of its
   `${{` to the end of the text. It is a lexical diagnostic, met at the end of
   the text. A literal or comment of the part that is itself unterminated is
   met first, since it is inner (`01-language.md` 2.11): `${{ "abc` is
   `unterminated-string` from the `"`, and `${{ /* x` is
   `unterminated-comment` at the `/*`.
7. A line comment runs to the next line end or to the end of the text. On one
   line, `${{ x // c }}` is an unterminated part.
8. The tokens of a part must form exactly one `Expr` (`01-language.md` 3.1), as
   the tokens of an interpolation do. A part holds no `const` line and no `;`:
   `${{ const a = 1; a }}` is `reserved-word` at `const` (`01-language.md`
   14.2), and `${{ a; }}` is `unexpected-token` at the `;`. A block may stand in
   a part, and may hold `const` lines: `${{ { const a = 2; a * 3 } }}`.
9. A part with no token, or whose tokens end before an `Expr` is complete, is
   `unexpected-token` at its closing delimiter, or at its closing brace when the
   part ended there. So `${{ }}` and `${{ a + }}` are each an error at the
   `}}`.
10. A part is not a nested template. Outside a string, `$` is an unexpected
    character in a part, so `${{ a ${{ b }} }}` is `unexpected-character` at the
    second `$`.
11. **Levels.** The `${{` of a part opens one level of `01-language.md` 13.1,
    which lasts to its closing delimiter or closing brace. So the first token
    of a part's expression is at nesting depth 1, as after `${`, and a part
    holds at most 99 more levels. `nesting-exceeded` is reported by `01-language.md` 13.2
    at the token that would open level 101.
12. Every code point a part's lexer reads that `01-language.md` 2.10 says
    begins no token is `unexpected-character` there, a U+FEFF included.

### 2.3 Diagnostics and spans

1. Every position of a template's diagnostics, nodes, sites and report is a
   position in the whole template text, by `03-host.md` 3.7.7. A line ends at
   U+000A only. It is never a position within a part.
2. Template parse is one stage. It fails with a list of `parse` diagnostics.
   Only the first is specified: its code and its span (`03-host.md` 3.7.2).
3. **The first diagnostic.** The whole text is read by 2.1 and 2.2 first,
   every part's tokens included. If that finds a lexical diagnostic of
   `01-language.md` 2.11 item 3, or an unterminated part (2.2 item 6), the
   first diagnostic is the first one met, reading from the start of the text.
   Otherwise the parts are parsed in order, and the first diagnostic is the
   first that the parser meets (`01-language.md` 14.1), a closing brace of 2.2
   item 5 included.
4. A template that has a `parse` diagnostic has no program and no report.
5. The codes are those of Core (`03-host.md` 3.7.3). This package adds no code.

### 2.4 The segments

1. The **segments** of a template are its text runs and its parts, in the
   order of the text.
2. A text run has its value (2.1 item 5) and its span: the code points it was
   read from, `$${{` included as four.
3. A part has two spans: the span of its expression (`01-language.md` 4.3), and
   the span of the whole part, from the `$` of its `${{` to the end of its
   closing delimiter.
4. Two parts with nothing between them are two segments with no text run
   between. The empty text has no segment.

### 2.5 The program of a template

A template gives a program, a parsed program in the sense of `03-host.md` 3.2.
It is checked, compiled and run by the stages of `03-host.md` like any other.
The host chooses the mode.

| Mode | The template | Its program |
| --- | --- | --- |
| `string` | Has at least one part | `Program` with no `const`, whose result is a `Template` node. Its parts are the segments, in order: each text run as a text part with its value, and each part as an expression part |
| `string` | Has no part | `Program` with no `const`, whose result is a `String` node whose value is the whole text's value |
| `value` | Is single | `Program` with no `const`, whose result is the part's expression |
| `value` | Is not single | As in mode `string` |

1. In mode `string` the value of a template is always a string.
2. In mode `value` a single template gives the value of its expression, as it
   is, with no rendering: `${{ pr.number }}` gives the number, `${{ tags }}`
   gives the list and `${{ null }}` gives null.
3. A template that is not single gives the same program in both modes. So
   ` ${{ pr.number }}`, with one space before the part, and `${{ a }}${{ b }}`
   give strings in mode `value`.
4. The `form` of the `Template` and the `String` node of a template is `${{`.
   Evaluation never reads it (`01-language.md` 4.2).
5. **Spans.** In mode `string`, the span of the `Program` node and of its
   `Template` or `String` node is the whole text, from its first code point to
   the end of its last. The empty text gives the empty span at line 1,
   column 1. In mode `value`, the span of a single template's `Program` node is
   the span of the part's expression.
6. **Scope.** Each part is an expression of its own. No name is bound across
   parts, and no binding of the host's text exists. A name bound in a block in
   one part is not in scope in another. Each part sees the environment only.
7. **Levels.** The nesting depth of every expression of a template, used by the
   depth limit (`02-evaluation.md` 8.4), is counted on the template text by 2.2
   item 11, in both modes.

### 2.6 Check

The checker of `05-types.md` checks the program of 2.5 as it checks any
program. This package adds no rule of the checker.

1. Each expression part of a `Template` is a rendered expression
   (`05-types.md` 7.3). The type of a template with a part is `string` in mode
   `string`.
2. A template with no part is a `String`. Its type is the literal set of its
   value (`05-types.md` 7.2).
3. In mode `value`, the type of a single template is the type of its
   expression. Its expression is not rendered, so it is not a rendered
   expression: an opaque value is accepted there, and an `any` value records no
   site of 7.3.
4. **Expected result.** The expected result (`03-host.md` 3.2.3) is applied to
   the program by `05-types.md` 15.3. A `Template` is not followed by
   `05-types.md` 15.1, so a `result-mismatch` on a template with a part has the
   span of the whole text.
5. The static rules (`01-language.md` 14.4) apply to the program, with or
   without a check.

### 2.7 Run and cost

1. **One run.** A template's program is one program, run once. Its parts share
   one step budget, one allocation budget, one deadline and one depth count. A
   run reports one usage for the whole template.
2. Parts are evaluated left to right, by `02-evaluation.md` 2.2. A failure in
   one part ends the run: no later part is evaluated.
3. **Cost.** This package adds no row to the cost table. A template is charged
   as the node of 2.5 that it gives:

| Program of 2.5 | Steps | Units |
| --- | --- | --- |
| A `Template` | The row "Template" of `02-evaluation.md` 5.2: 1 + size of the result, once, plus the rendering walk of each part (5.5), plus what each part's expression charges | 1 + size of the result (`02-evaluation.md` 6.1), plus what each part's expression charges |
| A `String`, from a template with no part | 0 | 0 |
| A single template in mode `value` | What its expression charges | What its expression charges |

4. The size of the result is computed from the value, after `$${{` is read as
   `${{`.
5. A rendering failure fails with `wrong-kind` at the `Template` node
   (`01-language.md` 14.5), which has the span of the whole text.
6. A run failure has the span its node has, in the template text.

### 2.8 The string forms of F3

F2 depends on F3 (`f03-strings.md`): an implementation that offers F2 offers
F3.

1. A part is read by the lexer of F3. A raw string, a text block and a raw text
   block are each one token, and their text holds no token, so `}}` in them
   does not close the part. `${{ r"}}" }}` holds the raw string `}}`, and
   `${{ r"\" }}` ends after the second quote.
2. In a text run these forms are characters (2.1 item 4).
3. Where a part ends is not always where Core's lexer would end it. A text block
   that holds `\"""` goes on past it (F3 4.2 item 1), and so does a text block
   that holds a text block in an interpolation. Core reads the first `"""` as
   the end of its block token. Both texts are errors in Core.
4. A text block needs line ends, so it stands only in a part that spans lines.
5. A raw string, a raw text block and a text block with no interpolation are
   `String` nodes: each is a literal (3.7) and a key of a path (3.5). A text
   block with an interpolation is a `Template` node: its interpolations are
   rendered expressions, and their reads and calls are reported. A raw text
   block reads no name: `${x}` in it is text.
6. F3's diagnostics apply in a part, at positions in the template text. Its
   three codes are not lexical (F3 6.2), so they take their place in the order
   of 2.3 item 3 as parse diagnostics.

### 2.9 Examples

`pr` is `{number: 42}`, `xs` is `[1, 2, 3]` and `name` is `"Ann"`.

| # | Template | Mode | Value or first diagnostic | Steps | Units |
| --- | --- | --- | --- | ---: | ---: |
| T1 | `Review of #${{ pr.number }}` | `string` | `"Review of #42"` | 2 | 1 |
| T2 | `${{ pr.number }}` | `value` | `42` | 1 | 0 |
| T3 | `${{ pr.number }}` | `string` | `"42"` | 2 | 1 |
| T4 | ` ${{ pr.number }}` | `value` | `" 42"` | 2 | 1 |
| T5 | `${{ xs }} items` | `string` | `"[1,2,3] items"` | 4 | 1 |
| T6 | `${{ {a: 1}}}` | `string` | `"{\"a\":1}"` | 2 | 4 |
| T7 | `${{ "}}" }}` | `string` | `"}}"` | 1 | 1 |
| T8 | `$${{ name }}` | `value` | `"${{ name }}"` | 0 | 0 |
| T9 | `Hello ${{ name }}!` | `string` | `"Hello Ann!"` | 1 | 1 |
| T10 | `${{ null }}` | `string` | `""` | 1 | 1 |
| T11 | `${{ null }}` | `value` | `null` | 0 | 0 |
| T12 | `${{ }}` | either | `unexpected-token` 1:5-1:7 | | |
| T13 | `a ${{ b` | either | `unterminated-string` 1:3-1:8 | | |
| T14 | `${{ a } b }}` | either | `unexpected-token` 1:7-1:8 | | |
| T15 | `${{ const a = 1; a }}` | either | `reserved-word` 1:5-1:10 | | |
| T16 | `\${{ 1 }}` | `string` | `"\1"`, the two code points `\` and `1` | 1 | 1 |

## 3. Analysis

### 3.1 The stage

| Stage | Takes | Gives | Fails |
| --- | --- | --- | --- |
| Analysis | A parsed program; or a parsed template and a mode | The report (3.3) | Never |

1. Analysis needs no environment. It reads the parsed tree only. A text gives
   the same report in every environment and every implementation.
2. Analysis applies no static rule and no type rule. A program that a static
   rule rejects, such as `const a = 1; const a = 2; a`, has a report.
3. A text that does not parse has no parsed program, so it has no report: the
   host gets the `parse` diagnostics of the parse.
4. Analysis charges no steps and no units, and calls no host function.
5. Analysis keeps to the rule of `01-language.md` 13.2 item 6: what does not
   open a level costs it no stack.
6. The analysis of a template in a mode is the analysis of its program in that
   mode (2.5), with the template's segments (3.8).

### 3.2 Free names and reads

1. The bindings of a program are those of `01-language.md` section 5: the names
   bound by a `const` line, by a pattern, by a lambda's parameters and by a
   `catch`, each in the scope that section gives. The environment is not
   consulted.
2. A name is **free** at a place when no binding of the program is in scope
   there. These rules of `01-language.md` decide it:

| Rule | Example | Free |
| --- | --- | --- |
| A name is not in scope in its own value (5 rule 6) | `const x = x + 1; x` | The `x` after `=` |
| A block's and a branch's names end with it (5 rule 3) | `{ const a = 1; a } + a` | The last `a` |
| The body and the handler of a `try` are two scopes (5 rule 9) | `try { const a = 1; a } catch { a }` | The `a` of the handler |
| A default sees only the names to its left (8.3 rule 3, 8.4 rule 1) | `const {b = a, a} = m; b` | The `a` of the default, and `m` |
| A lambda's parameters are bound in its defaults and body | `xs.map(x => x.id + y)` | `xs` and `y` |
| A `catch` name is bound in the handler | `try { a } catch (err) { err.code }` | `a` |

3. A **read** is an `Ident` node (`01-language.md` 4.1, node 4) whose name is
   free where it stands. A shorthand entry `{a}` holds an `Ident`, so it is a
   read of `a` when `a` is free. A key of a map literal, a field word after `.`
   or `?.`, and the name of a `Call` are not `Ident` nodes.
4. The receiver of `n.m(...)` is an `Ident` node. It is a read of `n` when `n`
   is free, whether or not a host declares a namespace `n` (3.6 item 3).
5. An `Ident` that names a `const`-bound lambda, as in `xs.map(double)`, is not
   free. An `Ident` that names nothing the program binds is a read whatever it
   names: in `xs.map(len)`, `len` is a read.

### 3.3 The report

The report is portable data: the JSON object below, with its fields in this
order. A binding gives the same contents in its own types.

| Field | Holds |
| --- | --- |
| `names` | The names of the reads, each once, in code point order (3.4) |
| `refs` | One reference for each read, in source order (3.5) |
| `calls` | One call for each call of a free function name, and each call `n.m(...)` with a free `n` (3.6) |
| `literal` | `false`, or `{"value": v}` when the whole program is a literal with the value `v` (3.7) |
| `segments` | For a template, its segments (3.8). For a program, `null` |

### 3.4 Names

1. `names` is a list of strings: the name of every read, with duplicates
   removed, ordered by code point.
2. A name is in `names` exactly when it has a reference in `refs`.
3. A name only called, as `f` in `f(x)`, is not in `names`.

### 3.5 References

1. A reference is `{"name": n, "path": [...], "span": s}`, one for each read.
2. **The path** is the list of the read's static steps. It is found by walking
   the chain (`01-language.md` 7.4) of which the read is the primary, from the
   read outward, one step at a time:

| Step | Effect on the path |
| --- | --- |
| `.w` or `?.w`, a field read | The key `w`, the word as written |
| `[e]` or `?.[e]`, where `e` is a `String` node | The key that is the string's value, after escapes |
| `[e]` or `?.[e]`, where `e` is any other node | The path stops before this step |
| A method call, `.m(...)` or `?.m(...)` | The path stops before this step |
| The end of the chain | The path stops |

3. A key is a JSON string. A `String` node includes a template literal with no
   interpolation and a string in parentheses, so `` m[`k`] `` and `m[("k")]`
   give the key `k`. `m[0]`, `m[i]`, `m["a" + "b"]` and `` m[`k${i}`] `` stop
   the path.
4. `.` and `?.` give the same step. A path says what is read, not what happens
   on null.
5. The path follows the chain only. It does not follow a binding:
   `const e = input.event; e.id` has one reference, `input` with the path
   `event`. It does not go past a parenthesis, which ends a chain:
   `(input.a).b` gives `input` with the path `a`. A pattern is not a step:
   `const {event} = input;` gives `input` with an empty path.
6. **The span** of a reference runs from the start of the read to the end of
   the last step of its path: the word of a field read, or the `]` of an index.
   With an empty path, it is the span of the read.
7. A reference says that the program may read the value at its path, and any
   part of that value. A shorter path claims more of the value, never less.

### 3.6 Calls

1. A **call** is reported for each of these nodes:

| Node | When | Its `name` |
| --- | --- | --- |
| `Call` (node 19) | Its name is free at the call: no `const`-bound lambda and no other binding of that name is in scope | The name |
| `MethodCall` (node 20) whose `optional` is false | Its `x` is an `Ident` node whose name is free | The receiver's name, `.`, and the method's name, as `geo.distance` |

2. A call of a library function is a call: `len(xs)`, `string(x)` and
   `fail("no")` are each reported. A method call whose receiver is not a free
   `Ident`, such as `user.name.trim()` or `xs.map(f).sort()` after its first
   step, is not.
3. The second row exists because a call of a host function in a namespace,
   `geo.distance(a, b)`, has the form of a method call until an environment
   names `geo` (`01-language.md` 4.2). Analysis has no environment, so it
   reports every such form, with its receiver's name in the field
   `namespace`. A host keeps a call of this row whose `namespace` its
   environment declares and drops the others, and it drops the reads whose
   name is a namespace it declares. `xs.map(x => x.id)` with `xs` free reports
   the call `xs.map` with the `namespace` `xs`, which the host drops.
   `ns?.f(a)` is not a call of a namespace, and is not reported
   (`01-language.md` 4.2).
4. A call is `{"name": n, "namespace": ns, "span": s, "args": [...],
   "lambda": b}`.

| Field | Holds |
| --- | --- |
| `name` | As the table of item 1 gives |
| `namespace` | For a `MethodCall` of the second row: the receiver's name, as `geo`. For a `Call`: `null` |
| `span` | The span of the `Call` or `MethodCall` node: from the name, or from the receiver, to the `)` |
| `args` | The span of each argument, in order. A lambda argument's span is the lambda's |
| `lambda` | `true` when the call stands inside a `Lambda` node: in its body or in a default of its parameters. This covers the body of a `const`-bound lambda and of a callback. Otherwise `false` |

5. A binding gives each argument as a parsed program of the argument's text,
   with spans in the whole text, so a host can analyze, check or compile it
   alone. An argument that reads a name the program binds does not compile
   alone. A lambda argument is not a program.
6. **The calls are complete.** A run calls a host function only at a `Call`
   whose name is free, or at a `MethodCall` of the second row that resolution
   made a `Call` (`01-language.md` 4.2, 7.3). Each is reported. A call with
   `lambda` true may run many times or not at all.
7. A call of a `const`-bound lambda by its name is not reported. The calls in
   its body are reported once, with `lambda` true, whether the lambda is called
   or not.

### 3.7 Literal

1. `literal` is `{"value": v}` when the program has no `const` line and its
   result is a literal expression with the value `v`. Otherwise it is `false`.
2. These are the literal expressions:

| Node | Value |
| --- | --- |
| `Null`, `Bool`, `Number`, `String` | The literal's value. A template literal with no interpolation is a `String` |
| `Unary` `-` whose operand is a `Number` node | The number negated. `-0` is zero (`01-language.md` 6.2) |
| `List` whose every item is a literal expression, with no spread | The list of their values |
| `Map` whose every entry has a key written as a word or a string, and a value that is a literal expression, with no spread, no computed key and no shorthand entry | The map of those keys and values. A key written twice takes its later value, as a run does (`01-language.md` 10.2) |

3. Parentheses have no node, so `(5)` and `-(5)` are literals. `--5`, `"a" +
   "b"`, a `Template` node, `{a}`, `{["k"]: 1}` and `[...xs]` are not.
4. A template in mode `string` that has a part is a `Template`, so it is not a
   literal. A template with no part is a `String`, whose literal is its text's
   value. A single template in mode `value` is a literal when its expression is
   one.

### 3.8 Segments

1. For a template, `segments` is the list of its segments in order (2.4).
2. A text run is `{"text": t, "span": s}`, where `t` is its value and `s` its
   span.
3. A part is `{"part": e, "span": s}`, where `e` is the span of its expression
   and `s` the span of the whole part, delimiters included.
4. The segments are the same in both modes.
5. For a program, `segments` is `null`. The empty template's `segments` is the
   empty list.

### 3.9 Order and duplicates

| List | Order | Duplicates |
| --- | --- | --- |
| `names` | Code point order | Merged: each name once |
| `refs` | By the start position of the read | Kept: `a.b + a.b` has two references |
| `calls` | By the start position of the call's span. No two calls start at one position | Kept: each call site is one call |
| `args` | The order of the arguments | Kept |
| `segments` | The order of the text | Not possible |

In a template, every part's reads and calls are in these lists, in the order
of the text.

### 3.10 Examples

| # | Program | `names` | `refs`: name and path | `calls`: name, `namespace`, `lambda` |
| --- | --- | --- | --- | --- |
| A1 | `input.event["id"]` | `input` | `input` `event`, `id` | none |
| A2 | `a[i].b` | `a`, `i` | `a` empty; `i` empty | none |
| A3 | `a?.b?.["c"]` | `a` | `a` `b`, `c` | none |
| A4 | `const e = input.event; e.id` | `input` | `input` `event` | none |
| A5 | `xs.map(x => lookup(x.id) + y)` | `xs`, `y` | `xs` empty; `y` empty | `xs.map` `xs` false; `lookup` null true |
| A6 | `geo.distance(a, b) > limit` | `a`, `b`, `geo`, `limit` | `geo`, `a`, `b`, `limit`, each empty | `geo.distance` `geo` false |
| A7 | `const double = x => x * 2; double(n) + rate(n)` | `n` | `n` empty, twice | `rate` null false |
| A8 | `const f = x => f(x); f(1)` | none | none | `f` null true: the `f` in the body is free (`01-language.md` 5 rule 6) |
| A9 | `{a: 1, b: [true, null], c: -2.5}` | none | none | none; `literal` is the map |

## 4. Additions to the case format

These are additions to `03-host.md` 3.10. Each has a default that leaves every
existing case file valid and its meaning unchanged.

### 4.1 The case field `template`

| Field | Holds | When absent |
| --- | --- | --- |
| `template` | `"string"` or `"value"`. The text of `source` or `file` is a template, and the case's program is its program in that mode (2.5) | The text is a program |

1. `template` may stand in the header (`03-host.md` 3.10.2).
2. Step 3 of `03-host.md` 3.10.5, Parse, parses the text as a template, and
   takes its program in the case's mode. Every other step is unchanged. Spans
   are positions in the template text.

### 4.2 The result field `analysis`

| Field of a result | Compared with | How |
| --- | --- | --- |
| `analysis` | The report of the analysis of the case's program, or of its template in its mode (3.1) | An object with any of the fields `names`, `refs`, `calls`, `literal` and `segments`. Each field it has is compared whole, by 4.4. A field it lacks is not compared |

1. A case whose `expect` has `analysis` is an **analysis case**. Its `expect`
   has no other field.
2. An analysis case is run in three steps: Read, Parse, Analyze. No
   environment is built, and the program is not checked, compiled or run. When
   Parse fails, that failure is the outcome, and the verdict is `fail` with the
   difference `outcome`.
3. An analysis case has none of the fields `env`, `opaque`, `methods`,
   `functions`, `reject`, `returns`, `vars`, `limits`, `check`, `precondition`
   and `version`, after the header rule.
4. A case that expects a `parse` failure is run as before. Since analysis takes
   a parsed program (3.1 item 3), such a case also states what analysis gives
   for its text.

### 4.3 The forms in an expected `analysis`

| Field | JSON form |
| --- | --- |
| `names` | A list of strings |
| `refs` | A list of objects `{"name", "path", "span"}`: a string, a list of keys, a span |
| `calls` | A list of objects `{"name", "namespace", "span", "args", "lambda"}`: a string, a string or `null`, a span, a list of spans, a boolean |
| `literal` | `false`, or an object `{"value": v}`, where `v` is a JSON value |
| `segments` | `null`, or a list of objects, each `{"text", "span"}` or `{"part", "span"}` |

Each object of `refs`, `calls` and `segments` has exactly the fields shown.
A span is written as in `03-host.md` 3.7.7.

### 4.4 How an expected `analysis` is compared

1. `names`, `refs`, `calls` and `segments` are compared whole: the same length,
   and each element equal to the element at the same position.
2. Two strings are equal when they have the same code points. Two spans, and
   two paths, are equal step by step. Two booleans are equal when they are the
   same.
3. `literal` is equal when both are `false`, or both are objects whose values
   are equal by the language's `==` (`01-language.md` 6.7).
4. The verdict `fail` lists the difference `analysis`. Its place in the order of
   `03-host.md` 3.10.8 item 2 is directly after `outcome`.

### 4.5 When a file is invalid

These are added to `03-host.md` 3.10.10.

1. A `template` that is not `"string"` or `"value"`.
2. An `expect` with `analysis` and any other field.
3. An analysis case with a field of 4.2 item 3, after the header rule.
4. An `analysis` with a field 4.3 does not list, or a field, an element or an
   object of `refs`, `calls` or `segments` not of the form 4.3 gives.

### 4.6 Schema

These are additions to the schema of `03-host.md` 3.10.11. `template` is added
to the properties of `fields` and to both `propertyNames` lists. `analysis` is
added to the properties of `result`.

```json
{
  "template": {"enum": ["string", "value"]},
  "analysis": {
    "type": "object",
    "properties": {
      "names": {"type": "array", "items": {"type": "string"}},
      "refs": {"type": "array", "items": {
        "type": "object", "required": ["name", "path", "span"],
        "properties": {
          "name": {"type": "string"},
          "path": {"type": "array", "items": {"type": "string"}},
          "span": {"$ref": "#/$defs/span"}},
        "additionalProperties": false}},
      "calls": {"type": "array", "items": {
        "type": "object",
        "required": ["name", "namespace", "span", "args", "lambda"],
        "properties": {
          "name": {"type": "string"},
          "namespace": {"type": ["string", "null"]},
          "span": {"$ref": "#/$defs/span"},
          "args": {"type": "array", "items": {"$ref": "#/$defs/span"}},
          "lambda": {"type": "boolean"}},
        "additionalProperties": false}},
      "literal": {"oneOf": [
        {"const": false},
        {"type": "object", "required": ["value"],
         "properties": {"value": true}, "additionalProperties": false}]},
      "segments": {"oneOf": [
        {"type": "null"},
        {"type": "array", "items": {"oneOf": [
          {"type": "object", "required": ["text", "span"],
           "properties": {"text": {"type": "string"},
                          "span": {"$ref": "#/$defs/span"}},
           "additionalProperties": false},
          {"type": "object", "required": ["part", "span"],
           "properties": {"part": {"$ref": "#/$defs/span"},
                          "span": {"$ref": "#/$defs/span"}},
           "additionalProperties": false}]}}]}
    },
    "additionalProperties": false
  }
}
```

The rules of 4.2 items 1 and 3 hold beside the schema, as the rules of
`03-host.md` 3.10.10 that the schema cannot express do.

### 4.7 The directories

| Directory | Holds |
| --- | --- |
| `suite/templates/` | Cases with `template` |
| `suite/analysis/` | Analysis cases |

1. A runner runs a case of these directories by `03-host.md` 3.10.8 item 5 as
   a case outside `suite/core-check/`: once, checked, when its `expect` has
   `type` or `sites` or a `check` failure with a code that is not static;
   otherwise twice, checked and with the static rules alone.
2. An analysis case is run once. It has no check.
3. A case that is run twice meets the conditions of `03-host.md` 3.10.13 for
   `suite/core-run/`.
4. A runner of an implementation that does not offer F2 does not run these
   directories. For it a file with `template` or an expected `analysis` is
   invalid, as a file with any field it does not know is (`03-host.md`
   3.10.10, 3.11).

### 4.8 The `outcome` command

The `outcome` command (`suite/README.md`) gains one field. Only an
implementation that offers F2 writes it.

| Field | Value |
| --- | --- |
| `analysis` | The report (3.3), with every field, when the case's text parses. Absent when it does not |

1. It is the last field of the line. Within the report the fields are in the
   order of 3.3, and the fields of each reference, call and segment in the order
   of 4.3.
2. It is written for every case whose text parses, a template or a program,
   whatever the later stages give. For an analysis case the line has `name`,
   `phase` (`null`, or `"parse"` with its `diagnostics`) and `analysis`.

## 5. What later packages add

| Package | Adds here |
| --- | --- |
| F7, queries | The star step `[*]` and `?.[*]` in a path. A star is a step of the path and does not stop it (`experiments/v3/design/05-host-api.md` A.8). In the JSON form a star is the object `{"star": true}`, which no key can equal. A method call after a star ends the path |
| F8, errors | The targeted `parse` codes, at the spans this chapter gives for Core's codes |
