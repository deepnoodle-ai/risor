# Risor v3 design: string literal forms

Status: plan accepted by the owner, with every decision of section 8 as
recommended (D46 in `00-overview.md`). `02` to `06` are revised to match. No
code was written and nothing was measured. This plan gives the exact rules of
the text block and the raw prefix. Its decisions are numbered `08-1` to `08-11`
(section 8).

**How to read this.** Section 2 is the proposal, written as rules so that it can
move into the specification. A paragraph headed *Why* is not a rule. Sections 3
to 9 are not rules.

**Evidence.** **(R)** read in a sibling document at the place cited. **(ran)** I
ran it for this draft, in Node 22 or PyYAML 6, in throwaway programs outside the
repository. **(web)** checked against that language's own documentation for this
draft. **(from memory)** not checked. **(I)** inferred. `02:` to `07:` are the
sibling documents.

## 1. Summary

1. **The problem.** Text of several lines (a prompt, a message, SQL, a shell
   command) has one form today, the backtick template (`02:2.2`) (R). Every
   character between the backticks is the value, so the text either starts at
   column 1 or carries the indentation of the program around it. And text with
   backslashes (a Windows path, a pattern, shell) must double each one.
2. **The proposal.** Two additions. A **text block**, `"""`, holds lines and
   removes their indentation when the program is parsed. A **raw prefix**, `r`,
   turns off escapes and interpolation.
3. **The three existing forms do not change:** `"..."`, `'...'` and `` `...` ``.
4. **No text changes meaning.** Both additions are syntax errors today, in
   TypeScript and in v3 (2.6).
5. **The form is spelling.** There is no new node kind, value, type or cost. A
   literal with no interpolation is a `String` node and one with an
   interpolation is a `Template`, whichever form wrote it. A `form` field keeps
   the spelling for the formatter.
6. **The closing delimiter sets the indentation.** A line with less is an error,
   not a guess.
7. **All of it happens when the program is parsed.** A run pays no step and no
   allocation unit for a form. A library `dedent` would pay both on every
   evaluation, and it gives another answer when an interpolated value has
   several lines.
8. **Kept small.** A raw string cannot hold its own delimiter, `\` does not join
   lines, and no raw form interpolates. Each of these is an error in 3.0, so
   each can arrive later without a break.
9. **Timing.** 3.0. Plan `12` (config files) assumes the text block.

## 2. The proposal

### 2.1 What it looks like

```ts
// A prompt. The four spaces before the closing delimiter are removed from
// every line. The value starts at "You" and has no line feed at its end.
const prompt = """
    You are reviewing pull request #${pr.number}.
    Reply with one of: ${verdicts.join(", ")}.

      - Quote the line you object to.
    """;

// Raw: a backslash is a backslash, and `${` is two characters.
const path = r"C:\builds\new";
const step = r"""
    run: echo "${{ secrets.TOKEN }}" \
      | tr -d '\n'
    """;
```

The third line of `prompt`'s value is empty and the fourth starts with two
spaces.

### 2.2 The five forms

| Form | Lines | Escapes | `${ }` | Indentation | Use it for |
| --- | --- | --- | --- | --- | --- |
| `"..."`, `'...'` | One | Yes | No | Nothing to remove | A name, a key, a short text |
| `` `...` `` | One or more | Yes | Yes | Kept: every character between the backticks is the value | One line built from values |
| `"""` ... `"""` | Any number, starting on the line after the opening `"""` | Yes | Yes | Removed: what stands before the closing `"""` | Text of several lines: a prompt, a message, SQL |
| `r"..."`, `r'...'` | One | No | No | Nothing to remove | A path, a pattern, text with backslashes |
| `r"""` ... `"""` | As a text block | No | No | Removed, by the same rule | Text in another language: shell, code, a template with its own `${` |

An author asks two questions. One line or several? Values and escapes, or
neither?

### 2.3 The text block

| Matter | Rule |
| --- | --- |
| Opening | `"""`. Only spaces and tabs may follow it on its line, and they are ignored. Anything else there is the error `text-block-open`. |
| Line ends | In a text block a line ends at U+000A, at U+000D U+000A, or at a U+000D alone, and each is read as one U+000A, as in a template (`02:2.2`) (R). So the value does not depend on how the file's lines end. No other character ends a line: U+2028 and U+0085 are content. Positions keep their own rule (`05:A.6`). |
| Content | Everything after the line end of the opening line, up to the start of the closing line. |
| Content line | A line of the content that begins in text. A line that begins inside an interpolation is program text and is not a content line. |
| Closing | The first `"""` that stands in the text of the content: not inside an interpolation, and with none of its quotes escaped. Only spaces and tabs may stand before it on its line. Anything else there is `text-block-close`. Anything may follow it: `;`, `,`, `.trim()`. |
| Indentation | The spaces and tabs before the closing delimiter, as a sequence of characters. |
| Removal | Every content line that is not blank must start with the indentation, character for character. That prefix is removed. A line that does not start with it is `text-block-indent`. |
| Tabs | The comparison is of characters. A tab is not a number of spaces and has no width. A mix is accepted when every line starts with the same mix. |
| Blank line | A content line with only spaces and tabs, or with nothing. It is an empty line, whatever white space it has, and it needs no indentation. |
| Other white space | White space after the indentation, and at the end of a line that is not blank, is content. |
| Last line end | Each content line ends with a line end. That of the last content line is not part of the value. A blank line before the closing delimiter therefore gives a value that ends with a line feed. With no content line the value is the empty string. |
| Escapes and `${ }` | Read after the removal, with the escapes of a template (`02:2.2`). So an escape is never indentation and never a line end: `\n` adds a line feed that no rule above sees, and `\u0020` at the start of a line is a space of content. |
| `\` before a line end | An error, as in a template today, where it is "any other escape" (`02:2.2`) (R). It does not join lines (08-6). |
| A quote in the content | `"` and `""` are content. Three in a row close the block, so one is escaped: `\"""`. |
| Interpolation | `${ Expr }`, as in a template: any expression, another text block among them. It opens one level of nesting (`02:2.16`). |
| An interpolated value with line feeds | Inserted as `02:2.13` renders it. Its later lines get no indentation. What is removed is fixed by the text, never by a value (08-7). |

Examples. `⏎` is a line end, `·` a space and `→` a tab. A span is `[line,
column, line, column]`, as in `05:A.11`.

| Source | Value or error |
| --- | --- |
| `"""⏎··a⏎····b⏎··"""` | `"a\n  b"` |
| `"""⏎··a⏎⏎··"""` | `"a\n"` |
| `"""⏎"""` and `"""⏎⏎"""` | `""` |
| `"""⏎⏎⏎"""` | `"\n"` |
| `"""·⏎→a⏎→"""`, with each `⏎` a U+000D U+000A | `"a"` |
| `"""⏎··a·⏎······⏎··"""` | `"a \n"`: the space after `a` stays, and the line of six spaces is empty |
| `"""⏎··say·\"""hi\"""⏎··"""` | `"say \"\"\"hi\"\"\""` |
| `r"""⏎··\n${x}⏎··"""` | The six characters `\n${x}` |
| `"""⏎··a⏎·b⏎··"""` | `text-block-indent` at `[3, 2, 3, 3]`, the `b` |
| `"""⏎→a⏎··"""` | `text-block-indent` at `[2, 1, 2, 2]`, the tab |
| `"""a⏎··"""` | `text-block-open` at `[1, 4, 1, 5]` |
| `"""⏎··a"""` | `text-block-close` at `[2, 4, 2, 7]` |

*Why the closing delimiter.* The margin is visible: it is the column of the
closing `"""`. The first line may be indented more than the rest, which YAML
needs an indicator for. Moving the delimiter left keeps indentation in the
value.

*Why an error.* A line to the left of the margin is a mistake in that line or in
the delimiter, and the parser cannot know which. Java removes the least
indentation of all lines (web), so one line moved left changes every other line
of the value and nothing is reported.

*Why a blank line is empty.* White space at the end of a line cannot be seen,
and editors remove it. Under this rule, removing it from a blank line never
changes a value and never makes an error.

### 2.4 The raw prefix

| Matter | Rule |
| --- | --- |
| Form | The letter `r` directly before the opening quote of `"..."`, `'...'` or `"""`. Lower case. No white space between the two. |
| Content | Every character up to the closing delimiter is the value. `\` is a backslash. `${` is two characters. |
| End | `r"..."` ends at the next `"`, `r'...'` at the next `'`, and a raw text block at the next `"""`. Nothing escapes a delimiter. So `r"\"` is a complete string of one backslash. |
| Lines | A one-line raw string may not hold a line feed or a carriage return, as a string may not (`02:2.2`). A raw text block follows 2.3 for its delimiters, line ends, indentation, blank lines and last line end. |
| What it cannot hold | Its own delimiter. `r"..."` holds `'`. `r'...'` holds `"`. A raw text block holds both, and `""`. No raw form holds `"""` or a carriage return. Such text is written in a plain form with an escape, or the piece is interpolated: `${'"""'}`. |
| A raw piece in a plain form | An interpolation may hold a raw string: `` `${r"\d+"} matched ${n} times` ``. |
| The name `r` | Not reserved. `r` followed by anything but a quote is a name, as today. |
| The backtick | `` r`...` `` is an error. In TypeScript it calls `r` as a tag **(ran)**, so here it cannot mean raw text (Y1). |

*Why a prefix, and why `r`.* It is one character, it is Python's and Rust's
(from memory), and it is an error in TypeScript today. TypeScript's own
spelling, `` String.raw`...` ``, needs tagged templates, and a function is not a
value here (D34).

### 2.5 Tokens and nodes

The two rows of `02:2.2` become:

| Element | Rule |
| --- | --- |
| String | `"..."` or `'...'`, with the same meaning, each with an optional prefix `r`. One line. |
| Template | `` `...` ``, or a text block, which may have the prefix `r`. May span lines. |

| Point | Rule |
| --- | --- |
| Longest match | `"""` is always the delimiter of a text block. `""` is the empty string only when no `"` follows it directly. The same holds for `r"""` against `r""`. |
| The prefix | The lexer reads a `Word`. When the word is exactly `r` and the next character is `"` or `'`, the two begin a raw literal. `xr"a"` is the word `xr` and then a string, an error as today. |
| Where a `String` may stand | Unchanged: an expression, a map key, a pattern key, a `match` case, a literal in type text (`03:2.2`). A one-line raw string is a `String` and may stand in each. |
| Where a text block may stand | Where a template may: as an expression. It is not a key, not a case and not a literal in type text. Those stay on one line (08-9). |
| Node | `Template` when the literal has at least one `${ }`. Otherwise `String`. This holds for every form, the backtick included, which `02:2.3` leaves unsaid (08-8). A raw literal is always a `String`. |
| `form` | `String` and `Template` each gain a field `form`: the delimiter (`"`, `'`, `` ` `` or `"""`) and whether `r` was written. It is for a formatter and for messages. Neither the checker nor an evaluator reads it, as for `Cond` and `As` (`02:2.3`) (R). |
| Count | 31 node kinds, as today. |
| Span | From the first character of the literal, the `r` if there is one, to the end of the closing delimiter. A part's span is its place in the source text. Removing indentation moves no position (`05:A.6`). |
| Type | By the node (`03:2.5`): a `String` has the literal set of its value, and a `Template` has `string`. |
| Analysis | A literal with no interpolation is a string literal to `05:A.8`, in every form. |

**The formatter.** It writes a literal back from its source text, because the
value does not record which escapes the author chose. It never changes one form
into another. In a text block it may replace the indentation: on each content
line that is not blank, and on the closing line. The value is the same before
and after. A backtick template cannot be moved this way, and that is the reason
to prefer a text block for text of several lines.

### 2.6 No text changes meaning

| Text | TypeScript | v3 today | With this plan |
| --- | --- | --- | --- |
| `"""`, then a line end | An error: the third `"` opens a string that its line does not close **(ran)** | An error: `""` and then a string. Two values never stand side by side (`02:2.2`) (I) | A text block |
| `"""abc"""`, `""""` | An error **(ran)** | An error | `text-block-open` |
| `r"a"`, `r'a'`, `r"""` | An error: a name and then a string **(ran)** | An error, for the same reason | A raw literal |
| `` r`a` `` | A call of `r` as a tag **(ran)** | An error | An error, with a message of its own (2.7) |
| `r`, `r.x`, `r => r`, `{r: 1}`, `r("a")` | A name | A name | A name |
| `""`, `"" + x`, `f("", "a")` | The empty string | The same | The same |
| `"a ${b}"` | The text as written **(ran)** | The same | The same (08-10) |
| `` `a\⏎b` `` | `ab`: the `\` joins the lines **(ran)** | An error | An error (08-6) |

- In the grammar of `02:2.2` no `String` directly follows a `String` or a
  `Name`, and the same holds in type text (`03:2.2`) (I). So no valid v3 program
  has `""` before a `"`, or the name `r` before a quote.
- The text of a template, of a comment and of a host string outside its segments
  is not lexed, so `"""` and `r"` there are characters as before.
- TypeScript adds no string form to JavaScript (from memory), so what Node
  rejects, TypeScript rejects.
- This plan adds no row to the list of `02:3.1`. Plan `10` adds two, so the
  list has nine.

### 2.7 Errors

`02` has no catalog of `parse` codes yet: it names `nesting-exceeded` and no
other. This plan names three. Its other errors are targeted errors in the manner
of `02:3.2`, and take whatever code those rows take when the catalog is written.

| Code | When | Span |
| --- | --- | --- |
| `text-block-open` | A character other than a space or a tab stands between the opening `"""` and the end of its line | That character |
| `text-block-close` | A character other than a space or a tab stands before the closing `"""` on its line | The closing delimiter |
| `text-block-indent` | A content line that is not blank does not start with the indentation | The first character of the line that differs from the indentation |

- After `text-block-open` or `text-block-close` the lexer reads on to the end of
  the literal and reports nothing more for it.
- `text-block-indent` is reported once for each such line, in order.
- A text block with no closing delimiter is the error of a template that is not
  closed.
- The node of a literal with an error is `Bad` (`02:2.3`).

| Written | Message says |
| --- | --- |
| `'''` | a text block is written `"""` |
| `"""text` on the opening line | the content starts on the next line |
| `text"""` | the closing `"""` stands on its own line, and what stands before it is the indentation removed |
| a content line to the left of the closing delimiter | indent the line, or move the closing `"""` left. When the difference is a tab against a space, it says so |
| a line end inside `"..."`, `'...'` or `r"..."` | a string ends on its line; write a text block, or `\n` |
| `\` before a line end, in a template or a text block | `\` does not join lines in 3.0; write `\\` for a backslash, or `r"""` |
| `f"..."`, `$"..."` | a template or a text block interpolates with `${ }`, and has no prefix |
| `` r`...` ``, `` String.raw`...` ``, any `` name`...` `` | there are no tagged templates; raw text is `r"..."` or `r"""` |
| `R"..."`, `r "..."` | the prefix is a lower-case `r` directly before the quote |
| `r#"..."#`, `#"..."#` | guards are not in 3.0; what a raw string cannot hold is written as 2.4 says |
| `<<EOF`, `<<-EOF`, `<<~EOF` | write a text block |
| `s.trimIndent()`, `s.stripIndent()`, `dedent(s)` | a text block removes its indentation (a hint of `06`, from the checker) |

**No diagnostic for `${` in a plain string (08-10).** `"cost: ${n}"` is valid
TypeScript with the same value **(ran)**. Such text is often meant: a shell
variable, a host's template. And a check diagnostic is always an error
(`05:A.6`) (R), so a rule here would refuse correct programs. An editor may mark
it.

### 2.8 Cost and limits (C5)

| Matter | Rule |
| --- | --- |
| Line ends, indentation, escapes | Settled when the program is parsed. A run does none of this work. |
| A `String` node, in every form | 0 steps: it is a literal (`04:A.3`). 0 units: the string is text of the program and the run does not create it. `04:A.4` charges a string from `+`, a template, a conversion or a library function, and lists no literal (I). |
| A `Template` node, in either form | What `04:A.3` and `04:A.4` charge a template string: the text created, and the rendering of each part. A text block and a backtick template with the same text and the same parts charge the same steps and the same units. |
| Nesting | Each `${` opens a level, as in a template (`02:2.16`). The text opens none. |
| The parser | The indentation is known only at the closing delimiter, so a lexer holds the block until then. Its time and memory are linear in the length of the literal, which is source text. |
| Size of a literal | No limit of its own, as for a long string today. |

A `dedent` function would be charged as text read and as text created on every
evaluation (`04:A.3`), and a constant would spend allocation units.

### 2.9 Host strings, YAML and origins

| Question | Answer |
| --- | --- |
| Are the forms part of a host string's text (`05:A.7`)? | No. Outside `${{ }}` the text is the host's, and `"""` and `r"` there are characters. Inside a segment stands an expression, so every form may be written. A text block needs line ends, so it fits only when the host's text has lines. |
| Where does a segment end? | Where the language's lexer says (`05:A.7`) (R). That lexer now reads raw strings: `${{ r"\" }}` ends after the second quote. A splitter that only steps over `\"` is wrong. |
| What is `${{` inside a template or a text block of a program? | `${` and then a map, as in a template today: `` `${{ a }}` `` renders the map `{a}`. Text for another template language (`${{ secrets.X }}`, `${HOME}`) goes in a raw text block, or is written `\${`. |
| A program in a YAML `\|` block | Works **(ran)**. YAML removes its own indentation first, and the text block's indentation is counted in the program text that is left. YAML gives line feeds whatever the file has **(ran)**. The origin of `05:A.6` stays exact, because a text block moves no position. |
| A program in a YAML `>-` block, or in a plain scalar of several lines | Lines of equal indentation are joined **(ran)**, so the opening delimiter no longer ends its line: `text-block-open`. The failure is loud. A folded block keeps the line ends of lines indented more than the first **(ran)**, and nobody should rely on that. A host that reads programs from folded scalars cannot offer text blocks (`02:3.3`). |
| A YAML plain scalar that starts with `"""` | A YAML error **(ran)**. One that starts with `r"` is read as written **(ran)**. |
| A field of one line | No text block. `r"..."` and `r'...'` both work, so a value in YAML's single quotes can hold `r"..."`. The reverse does not hold: inside YAML's double quotes a backslash is YAML's own escape, so `"r'\d+'"` is a YAML error (`02:3.3`). |
| How does a YAML author read a text block? | As a `\|-` block: lines kept, no line feed at the end. A blank line before the closing delimiter gives `\|`. The margin is the closing delimiter and not the first line. |
| A position inside the value | Not given. A host that parses the value, as SQL for example, maps a place in it to the source by itself. |

## 3. What is not in this plan

| Left out | Why | When |
| --- | --- | --- |
| A raw string that holds its own delimiter: a longer delimiter (`""""`) or guards (`r#"..."#`) | A second rule for a rare need. 2.4 has what to write. Both are errors in 3.0 | Later, without a break |
| `\` before a line end joins the lines | 08-6 | Later, without a break |
| A raw form that interpolates (C#'s `$$"""`) | A third choice on top of two. `${r"..."}` in a plain form does it | Not planned |
| An escape for a space at the end of a line (Java's `\s`) | `\u0020` says it | Not planned |
| Indenting each line of an interpolated value | `${v.replaceAll("\n", "\n    ")}` says it (`06`) | A library method could do it later |
| Tagged templates | A second call syntax, for one use | Not planned |
| A word after the opening delimiter that names the content's language (`"""sql`) | Only editors would read it. `text-block-open` keeps the place free | Later, without a break |
| A map from a position in the value to a position in the source | 2.9 | Later |

## 4. Changes to the other documents

| Document | Change |
| --- | --- |
| `02` | 2.2: the two lexical rows and the longest-match rule of 2.5; the rules of 2.3 and 2.4. 2.3: `form` on nodes 6 and 7, and the node rule; no new kind from this plan (32 with the `Each` of `13`). 2.13: a text block with parts renders as a template. 2.16: the `${` of a text block opens a level. 3.2: the rows of 2.7. 3.3: the YAML rows of 2.9. 3.4: two rows, `"""` and `r`, and `\` before a line end among the TypeScript forms left out. 4.2: expr's raw string, dropped, returns as `r"..."`. 7.1, item 1: the new forms. |
| `03` | No rule changes. 2.5 types nodes 6 and 7 as today. 2.2: a literal in type text may be a one-line raw string and may not be a text block. |
| `04` | A.3 and A.4: one sentence each, that the form of a literal has no cost and a text block with parts is a template string. Part B: the lexer removes indentation, and nothing is done in a run. |
| `05` | A.6: the three `parse` codes. A.7: the two segment rows of 2.9. A.8: a literal in every form. A.12 and B.11: the editor grammar learns both forms. A.11 needs nothing: `source` is a JSON string and holds line ends already. |
| `06` | No entry. `dedent`, `trimIndent` and `stripIndent` join the hints and the list of what is deliberately absent. Examples that hold a prompt use a text block. |
| `07` | Nothing. |

**What the conformance suite tests.**

- Every row of the example table of 2.3, with its value or its code and span.
- Each form against the one-line string of the same value: `==` is true, and the
  type is the same literal set.
- One block with its line ends as U+000A, as U+000D U+000A and as U+000D: one
  value.
- Raw: `r"\n"` has length 2; `r"\"`; `r"${x}"` checks with no `x` declared.
- Longest match: `""`, `"" + "a"`, `r""`, and each against a text block.
- Each code of 2.7 with its span; nothing more reported after `text-block-open`;
  one `text-block-indent` for each bad line.
- Cost: a text block with parts and the backtick template with the same parts
  give the same steps and units. A literal with no part gives 0 and 0.
- Analysis: the literal value of a text block, and the names its interpolations
  read.
- Positions: a check error inside `${ }` on the third line of a block carries
  that line and column.
- Nesting: 100 levels through nested text blocks, and the 101st.
- A host string with segments `r"}}"` and `r"\"`.

## 5. Size (I)

| Part | Go, lines without tests |
| --- | --- |
| Lexer: the text block, the prefix, three codes, the targeted errors | 150 |
| `form` on two nodes, and the parser | 20 |
| Formatter: copy a literal from the source, replace a block's indentation | 60 |
| Checker and evaluator | 0 |
| Total | about 230, and a few rules in the editor grammar |

The TypeScript and Rust figures should be close. Every delimiter and every
character of indentation is ASCII, so neither needs code point care here beyond
the columns of a span (T5). None of this was built.

## 6. Prior art

| Language | What it does | What I take from it |
| --- | --- | --- |
| Swift | `"""`: the first line end is dropped, and the white space in front of the closing delimiter is removed from each line (SE-0168, web). A line with less is an error, and `\` at a line end joins (from memory). | The rule of 2.3. Not the joining. |
| C# 11 raw strings | The opening quotes end their line and the closing quotes start theirs. White space left of the closing quotes is removed from all lines. A space does not match a tab. The line end before the closing quotes is not in the value. A longer delimiter holds `"""` (web). A blank line keeps its white space past the margin (from memory). | The same rule, found twice. The tab rule (08-3). Longer delimiters are "later". |
| Java text blocks | The least indentation of all lines is removed, the closing line among them. White space at line ends is removed. Line ends become `\n`. A space and a tab count the same, with a lint warning. `\` at a line end joins, and `\s` is a space. No interpolation (web). | Line ends as U+000A. Not the least-indentation rule (2.3, *Why an error*). |
| Kotlin | `"""` has templates and no escapes. Indentation is removed by `trimIndent()`, a call on the value (web). | The call runs after interpolation (I), which is the defect of `dedent`. |
| JavaScript | `` String.raw`...` `` is a tag (ran). `String.dedent` is a stage 2 proposal: a tag that removes the common indentation and empties lines of only white space (web). | The blank-line rule. No tags. |
| Python | `"""` keeps indentation, and `textwrap.dedent` is a library call. `r"\"` is an error, because a backslash still guards the quote (from memory). | The prefix. Not that exception: 2.4 has none. |
| Rust, Go | `r"..."` and `r#"..."#`. Backquotes with no escapes (from memory). | The prefix. Guards are "later". |
| YAML | Block scalars: the indentation is that of the first line, and a first line indented more than the rest needs an indicator. `\|` keeps one line end at the end and `\|-` drops it (ran). | The reading of 2.9. Not the first-line rule. |
| Risor v2, expr | v2 has no raw string: a backtick is its template (`risor:internal/lexer/lexer.go:364-372`) (R). expr's backtick is a raw string, which `02:4.2` dropped (R). | The need is known. |

## 7. Alternatives rejected

| Alternative | Why not |
| --- | --- |
| A library `dedent(s)` | It runs after interpolation, so a value with several lines changes what is removed. It is charged on every evaluation (2.8). Its errors have no position in the source. |
| Remove indentation from a backtick template of several lines | It changes the meaning of text valid in TypeScript (Y1), and of v3 programs. |
| `'''` as a second delimiter | One way to write a thing. |
| The prefix on a backtick | 2.4: in TypeScript it is a call. |
| Removing white space at the end of every line, as Java does | The value then differs from the text, and `\s` is needed to undo it. |
| The margin from the first content line, as YAML has it | The first line cannot then be indented more than the rest without an indicator. |
| A heredoc with a word the author picks | It solves the delimiter problem, but `<<` reads as an operator and nothing in TypeScript looks like it (C6). |
| A node kind for each form | The checker and every evaluator would gain rules for a spelling. |

## 8. Decisions for the owner

| # | Decision | Recommendation | Strongest alternative |
| --- | --- | --- | --- |
| 08-1 | How is the indentation of a text block found, and what is a line with less? | The closing delimiter sets it. A line with less is `text-block-indent`. Nothing but white space may follow the opening delimiter. | Java's rule: remove the least indentation of all lines. Nothing is ever an error, and a mistake changes the value silently. |
| 08-2 | Blank lines, and white space at the end of a line | A blank line is empty. White space at the end of any other line is content. | A blank line is a line like any other: it must start with the indentation and keeps the rest. Then a value, or an error, depends on characters nobody sees. |
| 08-3 | Tabs against spaces | Character for character, with no tab width. A mix is accepted when it is the same on every line. | Refuse a tab in the indentation of a text block. Simpler to say, and it refuses files indented with tabs. |
| 08-4 | The raw prefix | A lower-case `r`, on `"`, `'` and `"""`, never on a backtick. | Also `R`. A second spelling for nothing. |
| 08-5 | How does a raw string hold its own delimiter? | It does not. The other quote, then a raw text block, then a plain form or an interpolated piece. | C#'s longer delimiters: `""""` holds `"""`. Small, and the first thing to add if this proves too tight. |
| 08-6 | Does `\` before a line end join lines? | No. It is an error, in a text block and in a template. Shell text ends lines with `\`, and joining them silently would change what the author pasted. The message names `\\` and `r"""`. | Join, as Java, Swift and JavaScript do. Long lines of prose could then be wrapped in the source. |
| 08-7 | An interpolated value with several lines | Inserted as it is. | Indent its later lines to the column of the `${`. The result's size would then depend on a column, and a value meant to be exact would change. |
| 08-8 | Nodes | No new kind. `form` on `String` and `Template`. A literal with no `${ }` is a `String`, in every form. | A text block is always a `Template`. Its type is then `string`, never a long literal set (section 9), and a constant block is charged as text created on each evaluation. |
| 08-9 | Where may a text block stand? | As an expression only. Keys, `match` cases and type text stay on one line. A one-line raw string may stand wherever a string may. | A text block wherever a string may. One rule fewer, for a key of several lines that nobody wants. |
| 08-10 | A diagnostic for `${` in a plain string? | No (2.7). | A check class that a host may reject, off by default. It is a class for one mistake. |
| 08-11 | Timing | Both forms in 3.0. | A minor version. Possible, because both are errors today, but `12` needs the text block. |

## 9. Open questions

- **A long literal has a long type.** A text block with no `${ }` has the
  literal set of its value as its type, so a message that prints the type may
  print a whole prompt. A long one-line string does this today. Messages are not
  specified (`05:A.6`), so an implementation may cut a literal in a message as
  it cuts a key at 64 code points. Should `03` say so? The alternative of 08-8
  removes the question at a price.
- **`02` has no catalog of `parse` codes** (2.7). The suite compares codes, so
  it needs one. Plan `10` may supply it, and the targeted errors of 2.7 then get
  their codes there.
- **The node of a backtick with no `${ }`** is unsaid in `02:2.3`, and `04:A.3`
  can be read both ways: a literal costs 0, and a template string costs 1 and
  its size. 08-8 says `String`. It should be confirmed for the backtick.
- **One `text-block-indent` for each bad line, or one for the block?** This plan
  says each line, so that an author fixes them in one pass.
- **Should the formatter rewrite a backtick template of several lines as a text
  block?** This plan says no: the indentation there is content, and a formatter
  cannot know that it was not meant.
- The rows of section 6 marked "from memory" are not yet checked.
