# F3. String forms

This chapter is normative. It specifies follow-on package F3: the raw string,
`r"..."` and `r'...'`, and the text block, `"""` and `r"""`. Both are literal
forms of the language of `01-language.md`. Neither adds a node kind, a value, a
type or a run code. A literal of a new form is a `String` node or a `Template`
node, and is evaluated and charged as one (7.1).

The package only adds. In Core, every text that this chapter reads as a new form
is a `parse` error (`01-language.md` 2.3 rules 7 and 9, 14.3). Text that holds
no such form is read as before. Section 10 lists the Core cases whose text this
package reads differently.

Other chapters: `01-language.md` (tokens, escapes, templates, grammar, nodes,
spans, nesting), `02-evaluation.md` (cost), `03-host.md` (diagnostics,
positions, case files), `05-types.md` (types, the static rules).

## 1. Terms and scope

### 1.1 Terms

| Term | Meaning |
| --- | --- |
| Raw string | `r"..."` or `r'...'` (3) |
| Text block | `"""`, then lines, then `"""` (4) |
| Raw text block | `r"""`, then lines, then `"""` (4.7) |
| Opening delimiter | `"""` of a text block, or `r"""` of a raw text block |
| Closing delimiter | The `"""` that ends a text block (4.4) |
| Space or tab | U+0020 or U+0009. No other code point, U+00A0 and U+3000 among them |
| Blank | A sequence of code points is blank when each is a space or a tab. The empty sequence is blank |
| Line end | U+000A, U+000D followed by U+000A, or U+000D alone, as `01-language.md` 2.1 item 3 says |

### 1.2 What this package changes in Core's rules

| Core rule | With this package |
| --- | --- |
| `01-language.md` 2.3 rule 7, the block token | Applies to `'''` only. A token that starts with `"""` is a text block (4) |
| `01-language.md` 2.3 rule 9, the prefixed literal | Applies as before, except to the word `r` directly followed by `"`, or by `'` that is not followed by `''` (2) |
| `01-language.md` 2.11 item 3, lexical diagnostics | Unchanged. The three codes of 6.1 are not lexical |
| `01-language.md` 14.2, `text-block-open`, `text-block-close`, `text-block-indent` | Reported, by 6.1 |
| `01-language.md` 14.3, the rows of a text block and of a raw string | The row of `"""` no longer applies; 6 does. The row of the prefixed literal no longer applies to `r"` and `r'`. Every other row stands, the row of `r` or `R` followed by white space and a string among them. Where a text stays an error and no earlier case pins its span, this chapter's span replaces Core's (rulings, BA5) |
| `05-types.md` 3.2, the token `String` of type text | Includes the raw string (8.2) |

### 1.3 Version and case files

1. Both forms are part of version `3.0`. This package adds no version number, no
   declaration and no field of a case file.
2. The cases of this package are in `suite/strings/`. A runner runs them as it
   runs the cases of `suite/core-run/` (`03-host.md` 3.10.8). A case that states
   `type` or `sites` there needs the type checker, by the rules of 3.10.8.

## 2. Where a literal begins

At each place where a token starts, the lexer applies the first row of this
table that holds. "Directly" means with no code point between.

| # | The text at this place | Begins |
| --- | --- | --- |
| 1 | A word (`01-language.md` 2.4) that is exactly `r`, directly followed by `"""` | A raw text block. Its opening delimiter is the four code points `r"""` |
| 2 | The word `r`, directly followed by `'''` | The prefixed literal of `01-language.md` 2.3 rule 9, as in Core: `r'''a'''` is `unexpected-token` at the `r` |
| 3 | The word `r`, directly followed by `"` or `'` | A raw string (3) |
| 4 | Any other word that is not reserved, directly followed by `"`, `'` or `` ` `` | The prefixed literal, as in Core. `R"a"`, `xr"a"`, `rr"a"`, `f"a"` and `` r`a` `` are each `unexpected-token` at the word |
| 5 | Any other word | A word. A reserved word followed by a quote is the word and then the literal, as in Core |
| 6 | `"""` | A text block. Its opening delimiter is the three code points `"""` |
| 7 | `'''` | The block token of `01-language.md` 2.3 rule 7, as in Core: `unexpected-token` at its first three code points |
| 8 | `"` or `'` | A one-line string (`01-language.md` 2.6) |
| 9 | `` ` `` | A template (`01-language.md` 2.8) |

1. By rows 1 and 6, three double quotes are always an opening delimiter where a
   token starts. `""` is the empty string only when no `"` follows it directly.
   `r""` is the empty raw string only when no `"` follows it directly.
2. `r` followed by anything other than a quote is the word `r`, a name: `r`,
   `r.x`, `r => r`, `{r: 1}`, `r("a")` and `r+"a"` read as in Core.
3. These rules apply only where a token starts. In a one-line string, in a raw
   string, in the text of a template or of a text block, and in a comment, `"""`
   and `r"` are code points like any other.

## 3. Raw strings

1. A **raw string** is `r`, a quote (`"` or `'`), content, and the same quote.
2. The content is every code point after the opening quote up to the next code
   point equal to that quote. Nothing in it is an escape or an interpolation:
   `\` is U+005C and `${` is two code points.
3. So `r"\"` is a complete raw string whose value is one `\`, and `r"a"b"` is
   the raw string `a` followed by the prefixed literal `b"`.
4. A raw string in `"` may hold `'`, and one in `'` may hold `"`. No raw string
   holds its own quote.
5. The content may not hold U+000A or U+000D. A line end, or the end of input,
   before the closing quote is `unterminated-string`. Its span runs from the `r`
   to the place where the literal stops: the start of the line end, or the end
   of input. A `\` before that place is content and changes nothing.
6. The value is the content, unchanged.

| Text | Value or error |
| --- | --- |
| `r"C:\new"` | The 6 code points `C:\new` |
| `r"\n"` | 2 code points, `\` and `n` |
| `r"\"`, `r'\'` | One `\` |
| `r"${x}"` | The 4 code points `${x}`. No name is read |
| `r"it's"`, `r'say "hi"'` | `it's`; `say "hi"` |
| `r""`, `r''` | The empty string |
| `r"abc` | `unterminated-string`, `[1, 1, 1, 6]` |
| `r"a` then a line end | `unterminated-string`, `[1, 1, 1, 4]` |

## 4. Text blocks

### 4.1 The opening line

1. The **opening line** is the code points after the opening delimiter up to the
   first line end, or up to the end of input if no line end follows.
2. When the opening line is blank and a line end follows it, the opening line
   and that line end are dropped. The **content** starts after that line end.
3. When the opening line holds a code point that is not a space or a tab, the
   literal is `text-block-open` (6.1). It then runs from its opening delimiter
   to the end of the next `"""` after the opening delimiter, or to the end of
   input if there is none. Nothing in it is read as an escape, an interpolation,
   a line end or a closing line, and nothing more is reported for it.
4. When the opening line is blank and the end of input follows it, the literal
   is `unterminated-string`, with the span from the first code point of the
   opening delimiter to the end of input.

### 4.2 Text and interpolations

The content of a text block is read from its start, left to right, as text parts
and interpolations. The first row of this table that holds applies.

| # | The content at this place | Effect |
| --- | --- | --- |
| 1 | `"""` | The closing delimiter (4.4). The literal ends after it |
| 2 | `\` | An escape of a template (`01-language.md` 2.7, 2.7 item 2): every escape of a one-line string, `` \` `` and `\$`. A `\` directly before a line end is `line-continuation`, with the span of the `\`. A `\` directly before the end of input is part of the literal: the literal is unterminated (item 2). Any other failure is `bad-escape` with the span of `01-language.md` 2.7 |
| 3 | `${` | An interpolation, exactly as in a template (`01-language.md` 2.8 rules 5 and 6). It may hold any expression, a text block or a raw string among them |
| 4 | A line end | A line end in text (4.3) |
| 5 | Any other code point | Text. `"`, `""`, `'`, `'''`, `` ` ``, a `$` not followed by `{`, `//`, `/*`, U+000B, U+000C, U+0085, U+00A0, U+2028 and U+3000 are each text |

1. So `\"""` is an escaped quote followed by two quotes of text, and does not
   close the block.
2. **Unterminated.** If the end of input comes inside a text block, in its text
   or in an interpolation, the code is `unterminated-string`. Its span runs from
   the first code point of the innermost template or text block that is still
   open to the end of input. A literal that is itself open inside an
   interpolation is innermost (`01-language.md` 2.11 item 2).

### 4.3 Lines

1. A **content line** is a sequence of the content that starts at the start of
   the content or directly after a line end in text, and that runs to the next
   line end in text or to the closing delimiter. A content line may hold
   interpolations. A line end inside an interpolation is white space of the
   interpolation's expression and does not end a content line.
2. The line end that ends a content line is not part of it.
3. The **closing line** is the last content line: the one that runs to the
   closing delimiter.
4. A content line that is not the closing line is **blank** when its code points
   are blank (1.1): it holds only spaces and tabs, or nothing, and no escape and
   no interpolation.

### 4.4 The closing line and the indentation

1. The **closing delimiter** is the first `"""` met by row 1 of 4.2: in text,
   not inside an interpolation, and with none of its quotes part of an escape.
2. The closing line must be blank. When it holds any other code point, an
   interpolation among them, the literal is `text-block-close` (6.1). Then no
   `text-block-indent` is reported for the literal.
3. The **indentation** is the sequence of code points of the closing line. It
   may be empty.
4. Anything may follow the closing delimiter on its line: `;`, `,`, `)`, `.`, an
   operator. A `"` that follows it directly begins a one-line string.

### 4.5 Removing the indentation

1. Each content line other than the closing line is taken in order.
2. A blank line becomes the empty line, whatever spaces and tabs it holds. It
   needs no indentation.
3. A line that is not blank must start with the indentation, code point for code
   point. Its first code points are compared with the indentation's, one by one;
   a tab equals only a tab and a space only a space. When they are equal for the
   whole length of the indentation, those code points are removed. When they
   differ, the literal is `text-block-indent` at the line's first code point
   that differs (6.1).
4. Everything after the removed code points is content: spaces, tabs and other
   white space after the indentation, and at the end of the line.
5. An escape is never indentation and is never a line end: it begins with `\`,
   which is not a space or a tab. `\u0020` at the start of a line is a space of
   content, and `\n` gives a U+000A that no rule of this section sees.
6. The text of a line that begins after an interpolation's `}`, on a source line
   that began inside the interpolation, is not at the start of a content line.
   It is not compared with the indentation.

### 4.6 The value

1. The value is the content lines other than the closing line, each after 4.5,
   joined with one U+000A between each two. With no such line, the value is the
   empty string.
2. So each line end in text becomes one U+000A, whether it was written U+000A,
   U+000D U+000A or U+000D, and the line end directly before the closing line is
   not part of the value. A blank line directly before the closing line gives a
   value that ends with U+000A.
3. In each line, an escape is replaced by the code point it names, as in a
   template.
4. An interpolation is replaced by the rendering of its value (`01-language.md`
   12), as in a template. Line feeds in the rendering are inserted as they are.
   They receive no indentation and are not line ends of the block. What is
   removed is fixed by the source text, never by a value.
5. The value never holds a U+000D that the source wrote as a code point. `\r`
   still gives U+000D.

### 4.7 Raw text blocks

A raw text block follows 4.1 to 4.6, with these differences.

1. Its opening delimiter is `r"""`, four code points.
2. Its content has no escape and no interpolation. Rows 2 and 3 of 4.2 do not
   apply: `\` and `${` are text. A `\` directly before a line end is text, and
   the line end ends the line.
3. Its closing delimiter is the first `"""` after the opening line. A raw text
   block cannot hold `"""`. It holds `"` and `""`.
4. A raw text block with no closing delimiter is `unterminated-string`, with the
   span from the `r` to the end of input.

### 4.8 Examples

`⏎` is U+000A, `·` a space and `→` a tab.

| Source | Value or error |
| --- | --- |
| `"""⏎··a⏎····b⏎··"""` | `"a\n  b"` |
| `"""⏎··a⏎⏎··"""` | `"a\n"` |
| `"""⏎"""`, `"""⏎⏎"""` | `""` |
| `"""⏎⏎⏎"""` | `"\n"` |
| `"""·⏎→a⏎→"""`, with each `⏎` written U+000D U+000A | `"a"` |
| `"""⏎··a·⏎······⏎··"""` | `"a \n"`: the space after `a` stays, and the line of six spaces is empty |
| `"""⏎··a⏎····b⏎"""` | `"  a\n    b"`: a closing delimiter at column 1 removes nothing |
| `"""⏎····a⏎··b⏎··"""` | `"  a\nb"`: the first line may be indented more |
| `"""⏎··say·\"""hi\"""⏎··"""` | `"say \"\"\"hi\"\"\""` |
| `"""⏎··a·${⏎1·+⏎······2⏎}·b⏎··"""` | `"a 3 b"`: lines 3 to 5 begin inside the interpolation |
| `r"""⏎··\n${x}⏎··"""` | The six code points `\n${x}` |
| `"""abc"""` | `text-block-open` at `[1, 1, 1, 4]` |
| `"""⏎··a"""` | `text-block-close` at `[2, 4, 2, 7]` |
| `"""⏎··a⏎·b⏎··"""` | `text-block-indent` at `[3, 2, 3, 3]`, the `b` |
| `"""⏎→a⏎··"""` | `text-block-indent` at `[2, 1, 2, 2]`, the tab |
| `"""⏎··a` | `unterminated-string` at `[1, 1, 2, 4]` |

## 5. Grammar, nodes and spans

### 5.1 Tokens

| Token of the grammar (`01-language.md` 3.1) | With this package |
| --- | --- |
| `String` | A one-line string or a raw string |
| `Template` | A template, a text block or a raw text block |

1. A raw string may stand wherever a `String` may: as an operand, a map key, the
   key of a field of a map pattern, a literal of a `match` arm, and a literal in
   type text.
2. A text block and a raw text block stand where a template may: where a
   `Primary` may. They are not a map key, not the key of a field of a pattern,
   not a literal of a `match` arm and not a literal in type text. There each is
   `unexpected-token`, or `bad-type` in a declaration, as a template is.
3. Where 3.5 or 3.2 rules 7 and 8 of `01-language.md` report a template with the
   span of the whole template, a text block or raw text block is reported with
   the span of the whole literal, from the first code point of its opening
   delimiter to the end of its closing delimiter. A raw string that does not fit
   is reported with the span of its token, from the `r` to the closing quote, as
   a one-line string is. Where this differs from Core's span for the same text,
   no earlier case pins that span, so this chapter's span holds (BA5).

### 5.2 Nodes

| Literal | Node |
| --- | --- |
| A raw string | `String` |
| A raw text block | `String` |
| A text block with no interpolation | `String` |
| A text block with at least one interpolation | `Template`. Its parts are its text parts after 4.5 and 4.6 and its interpolations, in order |

1. The node kinds stay those of `01-language.md` 4.1: 30 in Core, numbered 1 to
   30. This package adds none.
2. `form` of a `String` is one of `"`, `'`, `` ` ``, `"""`, `r"`, `r'` and
   `r"""`. `form` of a `Template` is `` ` `` or `"""`. As in Core, evaluation
   and the checker never read `form`: two nodes that differ only in `form` have
   the same meaning.
3. The value of a `String` node, and the text parts of a `Template` node, are
   fixed when the program is parsed.

### 5.3 Spans

1. The span of a literal of a new form, and of its node, runs from its first
   code point, the `r` when there is one, to the end of its closing delimiter.
   It may cover several lines.
2. The span of a node inside an interpolation is its place in the source text.
   Removing indentation moves no position.
3. Positions are those of `03-host.md` 3.7.7. A line ends at U+000A only. So in
   a block whose lines end with U+000D alone, every position is on one line, and
   the U+000D of a U+000D U+000A is a column of the line it ends.

## 6. Diagnostics

### 6.1 The three codes

| Code | When | Span |
| --- | --- | --- |
| `text-block-open` | The opening line holds a code point that is not a space or a tab (4.1 item 3) | The opening delimiter: `"""`, or `r"""` with its `r` |
| `text-block-close` | The closing line holds a code point that is not a space or a tab, or an interpolation (4.4 item 2) | The closing delimiter |
| `text-block-indent` | A content line that is not blank does not start with the indentation (4.5 item 3) | The first code point of the line that differs from the indentation |

1. A literal has at most one of `text-block-open` and `text-block-close`. A
   literal with either has no `text-block-indent`.
2. A literal may have several `text-block-indent`, one for each line that does
   not start with the indentation. The first in the text is its first.
3. Each of the three codes stands for the whole literal. The literal is a `Bad`
   node.
4. `text-block-close` and `text-block-indent` point at the fault, not at the
   opening delimiter where Core reported the same text: no earlier case pins
   that span, so this chapter's span holds (BA5).

### 6.2 When they are reported

1. The three codes are not lexical diagnostics (`01-language.md` 2.11). The
   lexer finds them and keeps them with the literal's token.
2. The parser reports the literal's first code when it reaches the token, before
   it tests whether the token fits where it stands and before it reads the
   tokens of any interpolation in the literal. This is the place at which Core
   reports the block token (`01-language.md` 2.3 rule 4).
3. So the order of `01-language.md` 2.11 and 14.1 holds unchanged:
   - A lexical diagnostic anywhere in the text comes first: `"""x""" @` is
     `unexpected-character` at the `@`. A `bad-escape`, `line-continuation` or
     `unterminated-string` in the content of a text block is lexical, and comes
     before a `text-block-close` or a `text-block-indent` of the same literal.
   - A syntax error at an earlier token comes first: `) """x"""` is
     `unexpected-token` at the `)`.
   - A literal's code comes before every diagnostic inside its interpolations,
     so an outer literal's `text-block-indent` comes before an inner literal's.
   - `1 """x"""` is `text-block-open` at `[1, 3, 1, 6]`, not
     `unexpected-token`.
4. After `text-block-open` nothing in the literal is read (4.1 item 3), so its
   content gives no lexical diagnostic.

### 6.3 Other errors of the new forms

| Text | Code | Span |
| --- | --- | --- |
| A raw string with a line end or the end of input before its closing quote | `unterminated-string` | 3 item 5 |
| A text block or raw text block with no closing delimiter, or whose opening line is blank and ends at the end of input | `unterminated-string` | 4.1 item 4, 4.2 item 2, 4.7 item 4 |
| A `\` before a line end in a text block | `line-continuation` | The `\` |
| An escape that fails in a text block | `bad-escape` | `01-language.md` 2.7 |
| A text block or raw text block as a map key, as the key of a field of a pattern, or where no token may stand | `unexpected-token` | The whole literal (5.1 item 3) |
| A text block or raw text block as a literal of a `match` arm | `unexpected-token` | Not pinned by the suite, as for a template (`01-language.md` 14.3) |

### 6.4 Text that stays an error

These keep the code and the span that Core gives them. A later package may give
some of them a targeted code at the same span.

| Text | Code | Span |
| --- | --- | --- |
| `'''`, and `r'''` | `unexpected-token` | The first three code points of `'''`; the `r` |
| `R"..."`, `f"..."`, any other word directly before a quote, and `` r`...` `` | `unexpected-token` | The word |
| `r "a"`, `R 'a'`: the word `r` or `R`, white space or a comment, then a one-line string or a raw string that does not fit | `unexpected-token` | The word |
| A line end in a one-line string | `unterminated-string` | `01-language.md` 2.6 |
| A `\` before a line end in a template | `line-continuation` | The `\` |

## 7. Cost and limits

### 7.1 Cost rows

These rows join the tables of `02-evaluation.md` 5.2 and 6.1.

| Form | Steps | Units |
| --- | --- | --- |
| A raw string | 0 | 0. It is text of the program |
| A text block or raw text block with no interpolation | 0 | 0. It is text of the program |
| A text block with at least one interpolation | As a template: 1 + the size of the result, once, plus the rendering walk of each part (`02-evaluation.md` 5.5), charged as the template row says | As a template: 1 + the size of the result |

1. The result whose size is charged is the value of 4.6: after the indentation
   is removed, each line end is one U+000A, the line end before the closing line
   is dropped, and each escape is the code point it names. Removed indentation,
   the opening line and the delimiters are not counted.
2. A text block and a backtick template with the same text parts and the same
   interpolations charge the same steps and the same units.
3. Removing indentation, reading line ends and reading escapes are done when the
   program is parsed. A run does none of this work and charges nothing for it.
4. A failure inside an interpolation of a text block is charged and reported as
   in a template: what the parts charged stays charged, and the template's own
   step and units are not charged.
5. These forms create no value that `02-evaluation.md` does not already charge.
   They add nothing to the depth count.

### 7.2 Nesting

1. Each `${` of a text block opens one level of `01-language.md` 13.1, as the
   `${` of a template does, and the level lasts to its matching `}`.
2. Nothing else in the new forms opens a level: not the delimiters, not the
   text, and not `${` or a bracket in a raw string or a raw text block.
3. The `${` that would open level 101 is `nesting-exceeded`, with the span of
   its two code points (`01-language.md` 13.2).

### 7.3 Source size

1. A literal of a new form has no limit of its own on its length or its number
   of lines, as a one-line string has none.
2. An implementation reads a text block in time and memory linear in the length
   of its source text, its interpolations excluded. The indentation is known
   only at the closing delimiter, so a lexer holds the content lines of a block
   until then.

## 8. Types

### 8.1 Nodes

The checker types the nodes of 5.2 as `05-types.md` 7.2 and 7.3 type a `String`
and a `Template`.

1. A raw string, a raw text block and a text block with no interpolation have
   the literal set of their value.
2. A text block with an interpolation has the type `string`, and each of its
   interpolations is a rendered expression (`05-types.md` 7.3).
3. Every rule of `05-types.md` that names a `String` node applies to a `String`
   node of a new form: a read `m["f"]` whose index is such a node reads the
   field `f`.

### 8.2 Type text

1. In type text (`05-types.md` 3.1, 3.2), a `String` may be a raw string. Its
   literal set is that of its value: `r"a\b"` is the set of the three code
   points `a\b`.
2. A text block or raw text block in type text is an error, as a backtick
   literal is: `bad-type` in a declaration, or `bad-signature` in a signature.

## 9. Static rules

The static rules of `01-language.md` 14.4 compare keys and literals by value.

1. A raw string as a key of a map literal is a `String` key. It is compared with
   the other word, string and shorthand keys by its value: in `{"a\\b": 1,
   r"a\b": 2}` the second key is `duplicate-key`.
2. A raw string as a `match` literal is compared by its value: `match s { "a" =>
   1, r"a" => 2, _ => 3 }` is `duplicate-case` at `r"a"`.
3. A name inside a raw string or a raw text block is text and is never read.
   `r"${x}"` reads no name.

## 10. Core cases this package replaced

Core pinned these texts as `parse` errors in
`suite/core-run/parse-follow-on.json`. Those cases were removed from that file
when this package was accepted. The file `strings-core-texts.json` of
`suite/strings/` holds each text with the outcome of this chapter.

| Removed Core case | Core outcome | With this package |
| --- | --- | --- |
| a text block | `unexpected-token`, `[1, 1, 1, 4]` | `text-block-open`, the same span |
| a text block after an operator | `unexpected-token`, `[1, 5, 1, 8]` | `text-block-open`, the same span |
| nothing in a text block is an escape | `unexpected-token`, `[1, 1, 1, 4]` | `text-block-open`, the same span |
| a text block with no close runs to the end of input | `unexpected-token`, `[1, 1, 1, 4]` | `text-block-open`, the same span |
| four double quotes | `unexpected-token`, `[1, 1, 1, 4]` | `text-block-open`, the same span |
| a text block after an empty string | `unexpected-token`, `[1, 6, 1, 9]` | `text-block-open`, the same span |
| T2: r, a space, then a text block: the text block row | `unexpected-token`, `[1, 3, 1, 6]` | `text-block-open`, the same span |
| a raw string r"..." | `unexpected-token`, `[1, 1, 1, 2]` | The value `"a"` |
| a raw string r'...' | `unexpected-token`, `[1, 1, 1, 2]` | The value `"a"` |
| a prefixed string after an operator | `unexpected-token`, `[1, 5, 1, 6]` | Parses; `1 + "a"` then fails with `wrong-kind` |
| nothing in a prefixed string is an escape | `unexpected-token`, `[1, 1, 1, 2]` | The value `a\q` |
| a prefixed string runs to the end of its line | `unexpected-token`, `[1, 1, 1, 2]` | `unterminated-string`, `[1, 1, 1, 5]` |

1. No valid Core program changes its meaning. This package changes how the lexer
   reads a place only where a token starts with `"""`, or with the word `r`
   directly followed by `"` or `'`. In Core each of these is the block token or
   the prefixed literal, and is always a `parse` error.
2. Every other Core case keeps its outcome, the other cases of
   `parse-follow-on.json` that hold `'''`, `R`, `` r` `` or `r` followed by
   white space among them.

## 11. Examples

| # | Text | Value or code |
| --- | --- | --- |
| 1 | `r"C:\builds\new"` | the 13 code points `C:\builds\new` |
| 2 | `len(r"\n")` | `2` |
| 3 | `r"\" + "x"` | `\x` |
| 4 | `{r"a-b": 1}` | `{"a-b": 1}` |
| 5 | `const r = 2; r` | `2` |
| 6 | `R"a"` | `unexpected-token` at `R` |
| 7 | `r'''a'''` | `unexpected-token` at `r` |
| 8 | `"""⏎····You·are·${role}.⏎⏎······-·Be·brief.⏎····"""`, `role` `"terse"` | `"You are terse.\n\n  - Be brief."`; 1 step and 1 unit |
| 9 | `"""⏎··${xs}⏎··"""`, `xs` `[1, 2]` | `"[1,2]"`; 3 steps: the walk 2, the template 1; 1 unit |
| 10 | `"""⏎··abc⏎··"""` | `"abc"`; 0 steps and 0 units |
| 11 | `"""⏎··a⏎··"""·+·"b"` | `"ab"` |
| 12 | `"""⏎··${"""⏎····a⏎····"""}⏎··"""` | `"a"` |
| 13 | `"""a"""` | `text-block-open` at `[1, 1, 1, 4]` |
| 14 | `"""x"""·@` | `unexpected-character` at the `@` |
| 15 | `"""⏎··a⏎·b"""` | `text-block-close` at `[3, 3, 3, 6]` |
| 16 | `"""` | `unterminated-string` at `[1, 1, 1, 4]` |
