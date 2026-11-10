# F8. Errors, second part

This chapter is normative. It specifies follow-on package F8. It adds to Core:

1. **Targeted `parse` codes**: 44 codes whose message names the fix, each
   reported in place of the general code that Core reports for the same text
   (2).
2. **The `parse` and `check` templates** of the message catalog, with two kinds
   of parameter, so that the message of every `parse` and `check` diagnostic is
   the same in every implementation (3).
3. **The trace** of a `run` diagnostic: the calls in progress when the run
   failed (4).
4. **Three renderings** of a diagnostic as text: one line, detail and JSON (5).
5. **`ConformAll`**: the conformance function that reports every failure, up to
   a maximum (6).
6. Additions to the case format and to the `outcome` command (7, 8).

It adds no run code, no run template, no node kind, no reserved word, no library
entry, no cost row and no limit. It changes no value, no count, no class, no
type and no site. It changes the code of a `parse` diagnostic only where 2 says,
and keeps its span except where 2.8 says. A trace, a rendering and `ConformAll`
charge nothing.

The chapters of Core are cited by file and section: `03-host.md` 3.8 is section
3.8 of `spec/core/03-host.md`. F3 is `spec/packages/f03-strings.md`, F2
`spec/packages/f02-templates-analysis.md` and F4
`spec/packages/f04-conversion.md`. Section numbers without a file are of this
chapter.

## 1. Conventions

### 1.1 Terms

| Term | Meaning |
| --- | --- |
| **Core diagnostic** | The first `parse` diagnostic of a text, with its code and span, as Core and the packages before F8 give it (`01-language.md` 14.1, F3 6.2, F4 2.6) |
| **Failing token** | The token at which the Core diagnostic is reported, or the end of input |
| **Targeted code** | A `parse` code of 2.11 that is not one of Core's 14 |
| **Operand word** | A word read as a `Name` where a `Primary` stands, not followed by `(`: not after `.` or `?.`, not a key, not a pattern |
| **Directly**, of code points | With no code point between |
| **The token before, the token after** | The previous or the next token. White space and comments may stand between |
| **Catalog** | `suite/messages.json` (the `run` phase, Core) and the two files of 3.1 |
| **Trace** | The list of 4 |
| **Visible text** | A text in which each code point U+0000 to U+001F and U+007F to U+009F is written as an escape: U+0009 as `\t`, U+000A as `\n`, U+000D as `\r`, every other one as `\u00` and two lower-case hex digits. Every other code point, `\` and `"` among them, is written as itself |

### 1.2 Requirements

1. An implementation that offers F8 offers every section of this chapter.
   Packages are offered in the order F3, F1, F2, F4, F5, F6, F7, F8 and on, each
   only with all before it (`03-host.md` 3.11 item 2). So it offers the forms of
   F3, F4 and F7, the type checker and F4's conformance function.
2. With F8, the message of every `parse` and `check` diagnostic is specified.
   The message of a `declare` diagnostic is still not specified.

### 1.3 Version

F8 is part of version `3.0`. It adds no version number. Each entry of 3.1 has
`"since": "3.0"`.

### 1.4 What this package changes in Core's rules

| Core rule | With this package |
| --- | --- |
| `01-language.md` 14.1 item 1 | The first `parse` diagnostic has a specified message (3) |
| `01-language.md` 14.2, the 14 codes | 44 more codes (2.11). Each is reported only where 2 says |
| `01-language.md` 14.3 | Each row is reported with the targeted code of 2, at the span the row gives, or at the span of 2.8 where no earlier case pins it (rulings BA5) |
| `03-host.md` 3.7.1, `span` and `message` | The message of a `parse` and of a `check` diagnostic is specified. A `run` diagnostic has the field `trace` (4) |
| `03-host.md` 3.7.2, the sentence on messages of `parse` and `check` | Those messages are built from the catalog (3) |
| `03-host.md` 3.7.3 | The `parse` phase has 58 codes (2.11) |
| `03-host.md` 3.8.1 item 2 | The catalog also has the entries of 3.1 |
| `03-host.md` 3.8.3 | Two more kinds of parameter, and the kind `token` made exact (3.2) |
| `03-host.md` 3.10 | The additions of 7 |
| `05-types.md` 16.1, the sentence on messages | The message of each `check` code is built from the catalog (3.5) |
| F4 7.1 item 6 | `ConformAll` is 6 |

## 2. Targeted parse errors

### 2.1 The rule

1. The Core diagnostic of the text is found first, by the rules of Core and of
   F3, F4 and F7, exactly as an implementation without F8 finds it.
2. Then the rows of 2.3 to 2.7 are tried in order. The first row whose condition
   holds gives the code, and the template of 3.4. Its span is the span of the
   Core diagnostic, except where the row names another (2.8).
3. When no row holds, the Core diagnostic stands, with its template of 3.4.
4. So a targeted code takes the place of the Core diagnostic and never of
   another: the order of lexical diagnostics, of syntax errors, of
   `reserved-word`, of the three codes of F3 and of `nesting-exceeded` is Core's
   (2.9).
5. The rows are conditions on the Core diagnostic: its code, the rule of Core
   that gives it, the failing token, the tokens next to it, and the production
   the parser is reading at the failing token. That production is fixed by the
   grammar of `01-language.md` 3.1 and the choices of 3.2, so it is the same in
   every implementation.

### 2.2 Terms of the conditions

| Term | Meaning |
| --- | --- |
| **A complete operand has been read** | The parser reads an `Expr` and the tokens before the failing token form a whole operand, after which a binary operator, `?`, a step or the end of the expression could stand |
| **Where an operand must begin** | The parser needs the start of an `Expr`, an operand or a `Lambda` there |
| **The index step** | The production `"[" Expr "]"` or `"?.[" Expr "]"` of a `Chain` (`01-language.md` 3.1) |
| **The pattern of a `Const`** | The `Pattern` between `const` and `=` |
| **A program type** | F4 2.2 |
| **The prefixed literal** | `01-language.md` 2.3 rule 9 |

### 2.3 Lexical diagnostics

These rows apply when the Core diagnostic is `unexpected-character` at a code
point p.

| # | Condition | Code and template |
| --- | --- | --- |
| A1 | p is `#`, and the code point after it is `"` or `'`, or it directly follows a word that is exactly `r` | `string-prefix`, `guard` |
| A2 | p is `#` | `hash-comment` |
| A3 | p is `$`, and the code point after it is `"`, `'` or `` ` `` | `string-prefix`, `prefix` |
| A4 | p is `$`, and the code point after it is not `{` | `context-symbol` |
| A5 | p is `&` | `bitwise-operator`, `and` |

1. A `$` directly followed by `{` stays `unexpected-character`: `${x}` outside a
   template, and a `${{` inside a part of F2.
2. Every other `unexpected-character` stands: `@`, `\`, `^`, `~`, a control
   character, a code point that is not ASCII, a U+FEFF after the start.
3. These are lexical diagnostics still. Their order among lexical diagnostics is
   Core's: the first met, reading from the start (`01-language.md` 2.11).

### 2.4 Reserved words

These rows apply when the Core diagnostic is `reserved-word` at a word w.

| # | Condition | Code |
| --- | --- | --- |
| B1 | w is reported by `01-language.md` 14.2 rule 2, where an `Expr` or an operand must begin (the start of a `match` case among those places), or w is a shorthand entry of a map literal (`01-language.md` 3.2 rule 7), and w is `let` or `var` | `let-binding` |
| B2 | The same places, and w is `for`, `while`, `do`, `break` or `continue` | `loop-statement` |
| B3 | The same places, and w is `return` | `return-statement` |
| B4 | The same places, and w is `function` | `function-declaration` |
| B5 | The same places, and w is `throw` | `throw-statement` |
| B6 | The same places, and w is `finally` | `finally-clause` |
| B7 | The same places, and w is `import` | `import-statement` |
| B8 | The same places, and w is `nil` | `nil-literal` |
| B9 | w is any other reserved word, and it is the first entry of a map literal whose `{` is the token after the `=>` of a lambda | `block-without-const`, at the span of 2.8 |

1. Where the grammar needs a `Name` (14.2 rule 1: the pattern of a `const`, a
   parameter, a field or element of a pattern, a rest, the name of a `catch`),
   the code stays `reserved-word`, whatever the word: `const let = 1;`, `(for)
   => 1`, `const {nil} = m;` and `catch (throw)`. A word whose token after is
   `=>`, where an `Arg` or a `Const` value may stand, is a parameter:
   `[1].map(do => 1)` stays `reserved-word`.
2. A shorthand entry of a map literal reads a value, so it is a place of B1 to
   B8: `{for}` is `loop-statement`, and `xs.map(x => { return x })` is
   `return-statement` at `return`. A shorthand field of a map pattern is not.
3. `true`, `false`, `null`, `const`, `if`, `else`, `in`, `not`, `try`, `catch`,
   `match`, `as` and `is` stay `reserved-word` at every place, but for B9.

### 2.5 Tokens that no rule accepts

These rows apply when the Core diagnostic is `unexpected-token` at a token of
the last row of `01-language.md` 2.3 (2.3 rule 4), or by the row of
`01-language.md` 14.3 for the word `r` or `R` before a string.

| # | Condition | Code and template |
| --- | --- | --- |
| C1 | The token `===` | `strict-equality`, `equal` |
| C2 | The token `!==` | `strict-equality`, `not-equal` |
| C3 | The token `+=`, `-=`, `*=`, `/=`, `%=`, `**=`, `??=`, `\|\|=` or `&&=` | `assignment`, `compound` |
| C4 | The token `++` | `assignment`, `increment` |
| C5 | The token `\|>` | `pipe-operator` |
| C6 | The block token `'''` (`01-language.md` 2.3 rule 7) | `text-block-quote` |
| C7 | The prefixed literal of the word `r` directly followed by `'''` (F3 2, row 2) | `text-block-quote` |
| C8 | The prefixed literal of any other word and a backtick, where an operand must begin, or where the token before it is `.` or `?.` | `tagged-template` |
| C9 | The prefixed literal of any other word and a quote, at the same places | `string-prefix`, `prefix` |
| C10 | The row of `01-language.md` 14.3 and F3 6.4: the word `r` or `R`, white space or a comment, then a one-line string or a raw string that does not fit | `string-prefix`, `space` |

1. These tokens are `unexpected-token` wherever they stand in Core, a program
   type included (F4 2.6 item 3). C1 to C7 apply wherever they stand: `x as ===`
   is `strict-equality`, and `x as '''a'''` is `text-block-quote`.
2. A prefixed literal that stands where neither an operand nor a field word may
   begin stays `unexpected-token`: after a complete operand, as the `b"` of
   `r"a"b"`, and where a type is expected, as in `x as R"a"`.
3. The word `r`, white space, then a text block is the text block's error (F3
   5.1 item 3), and stays `unexpected-token`.

### 2.6 Rules of the grammar

These rows apply when the Core diagnostic is `unexpected-token` by a rule of
`01-language.md` 3.4, or by F4 2.5. The rule that gives it is fixed by 14.1.

| # | Rule | Code and template |
| --- | --- | --- |
| D1 | 3.4 rule 1: a prefix operator before a power | `needs-parentheses`, `power` |
| D2 | 3.4 rule 2: a `!` operand of `in` or `not in` | `needs-parentheses`, `in` |
| D3 | 3.4 rule 3: `??` mixed with `\|\|`, `&&`, `==`, `!=`, `<`, `<=`, `>`, `>=`, `in`, `not in`, and with F4 `as` and `is` | `needs-parentheses`, `coalesce` |
| D4 | 3.4 rule 4: a token after the last branch of an `if` or the handler of a `try` | `needs-parentheses`, `after-branch` |
| D5 | 3.4 rule 5: an `if` or a `try` where an operand is needed | `needs-parentheses`, `operand` |
| D6 | 3.4 rule 6: a `match` subject that starts with `{`, `if` or `try`, and `match {` with no subject | `needs-parentheses`, `match-subject` |
| D7 | 3.4 rule 7: a branch written like a map | `branch-like-map` |
| D8 | 3.4 rule 8: an empty branch | `empty-branch` |
| D9 | 3.4 rule 9: a lambda as the value of a `const` whose pattern is not a name | `lambda-in-pattern` |
| D10 | 3.4 rule 10: `try` followed by `(` | `try-call` |
| D11 | 3.4 rule 11: no `else` after the last branch of an `if` | `missing-else` |
| D12 | 3.4 rule 12, at the token where `catch` should stand, when that token is the word `finally` | `finally-clause` |
| D13 | 3.4 rule 12, at the token where `catch` should stand | `missing-catch` |
| D14 | 3.4 rule 12, at the token after `catch` that is neither `(` nor `{`, or at the token after `catch (` `Name` `)` that is not `{` | `try-form` |
| D15 | 3.4 rule 12, inside the parentheses of `catch`: a token where the `Name` should stand, or a token after the `Name` that is not `)` | `catch-name` |
| D16 | F4 2.5: a `!` operand of `as` or `is` | `needs-parentheses`, `not-test` |

1. 3.4 rule 3 holds for `as` and `is` as for the other operators of level 6 (F4
   2.4): `a ?? b as T` is D3 at the `as`, and `x as T ?? d` is D3 at the `??`.
2. A reserved word where the `Name` of `catch` should stand is `reserved-word`
   by 14.2 rule 1 before rule 12 is reached: `catch (if)` stays `reserved-word`.

### 2.7 Other text that does not parse

These rows apply when the Core diagnostic is `unexpected-token` by
`01-language.md` 3.5 at the failing token t, or by the rows of 14.3 that F4 2.6
item 2 keeps. They are tried in this order.

| # | Condition | Code and template |
| --- | --- | --- |
| E1 | t is the `as` of `x as const` (F4 2.6 item 2) | `as-const` |
| E2 | t is the `is` of `x is not T` (F4 2.6 item 2) | `negated-is` |
| E3 | t is the word `finally` | `finally-clause` |
| E4 | The token before t is the word `if` of an `If`, and t is not `(`; or the token before t is the `)` of the test of an `If`, or the word `else`, and t is not `{`, nor `if` after `else` | `if-form`, `form` |
| E5 | The parser reads the test of an `If`, and t is `,` | `if-form`, `call` |
| E6 | The token before t is the word `try` of a `Try`, and t is neither `{` nor `(` | `try-form` |
| E7 | t is `=`, and a complete operand has been read | `assignment`, `assign` |
| E8 | t is `:`, and the parser reads the `Expr` of an index step | `slice-syntax` |
| E9 | t is `:`, and the token before it is the end of the pattern of a `Const`; or of the pattern of a parameter, in a parameter list (`01-language.md` 3.5 rule 3); or a `Name` whose token before is the `(` of a parenthesized expression | `type-annotation` |
| E10 | t is `\|` | `bitwise-operator`, `or` |
| E11 | t is `*` and the token before it is `.` or `?.`; or t is `]` and the token before it is the `[` or `?.[` of an index step | `star-form` |
| E12 | t is `?`, and the token before it is the `[` or `?.[` of an index step | `filter-step` |
| E13 | t is the word `satisfies` | `satisfies-operator` |
| E14 | t is the word `instanceof` | `instanceof-operator` |
| E15 | The token before t is the operand word `switch` or `case`; or t is `{` and the token before it is the `)` of a `Call` named `switch` with no namespace; or t is the word `case` at the start of a `match` case | `switch-statement`, at the span of 2.8 |
| E16 | The token before t is the operand word `typeof` | `typeof-operator`, at the span of 2.8 |
| E17 | The token before t is the operand word `interface`, `enum` or `type` | `type-declaration`, at the span of 2.8 |
| E18 | The token before t is the operand word `export` | `export-statement`, at the span of 2.8 |
| E19 | t is `<` where an operand must begin, and the code point directly after it is `<` | `heredoc` |
| E20 | t is `<` where an operand must begin | `angle-cast` |
| E21 | t stands where the `:` of a conditional should stand, and the `?` of that conditional is directly followed by `[` | `optional-index`, at the span of 2.8 |
| E22 | t is `-` in a map literal or a map pattern, in an entry or a field that begins with a word w; w is followed by one or more pairs of `-` and a word or a number token, each token directly after the one before it, and the token after them is `:` | `unquoted-key`, at the span of 2.8 |
| E23 | t stands in a map literal whose `{` is the token after the `=>` of a lambda, and no `:`, `,` or `...` stands before t at the level of that map | `block-without-const`, at the span of 2.8 |
| E24 | The token before t is the last token of a program type, and t is `+`, `-`, `*`, `/`, `%`, `**`, `.`, `?.`, `?.[`, `[`, `(` or `{` (F4 2.4) | `needs-parentheses`, `after-type` |
| E25 | t is `!`, a complete operand has been read, and the token after t is the word `is` | `negated-is` |

1. E7: `x = 1`, `f(a = 1)` and `const a = 1 = 2; a` are `assignment`. A `=`
   where an operand must begin, as in `2 > = 1`, or in a pattern, as in `const
   [...r = 1] = x;`, stays `unexpected-token`.
2. E9: `(x: int) => x` at the top of a program, where `(` cannot begin a lambda,
   is the third case; `[1].map((x: int) => x)` is the second; `const n: int = 1;
   n` is the first.
3. E11 and E12 read the forms of F7 as F7 leaves them: `xs[*]` is a projection,
   and `xs[]`, `xs.*` and `xs[?c]` are errors.
4. E15 to E18: an operand word is a whole operand, so `typeof x` fails at `x`. A
   call is not an operand word: `typeof(x)` is a call, and the checker reports
   `undefined-function` when nothing declares `typeof`.
5. E21: in `a?[i]` the conditional's `:` should stand at the end of input. In `c
   ? [1] : [2]`, with the `:`, nothing fails.
6. E23 covers `x => { x + 1 }`. B9 covers `x => { if (c) { 1 } else { 2 } }`. B3
   covers `x => { return x }`.
7. E22 covers `{max-retries: 3}` and `const {max-retries: r} = m;`, and comes
   before E23, so `x => {max-retries: 1}` is `unquoted-key`. `{a - b: 1}`, with
   spaces, stays `unexpected-token`.

### 2.8 Spans

1. A targeted code keeps the span of the Core diagnostic, but in the rows of
   this table. Each of them changes a span that no earlier case of the suite
   pins (rulings BA5), so that the diagnostic points at the fault.

| Row | Core's span | The span |
| --- | --- | --- |
| B9, E23 | A token in the map | The `{` of the map |
| E15 | The token after the word, or the `{` | The word `switch` or `case` |
| E16 | The token after `typeof` | The word `typeof` |
| E17 | The token after the word | The word `interface`, `enum` or `type` |
| E18 | The token after `export` | The word `export` |
| E21 | The token where the `:` should stand | The `?` of the conditional |
| E22 | The first `-` | From the first code point of w to the end of the last word or number token of the key |

2. Every other span of a Core diagnostic stands. Where `01-language.md` 3.4 or
   14.3 states the span, it is that span: `missing-else` at the token where
   `else` should stand or at the end of input, `branch-like-map` and
   `empty-branch` at the `{` of the branch, `try-call` at the `try`,
   `needs-parentheses` at the token or the prefix operator of the rule,
   `string-prefix` and `tagged-template` at the word, `text-block-quote` at the
   first three code points of `'''` or at the `r` of `r'''`.

### 2.9 Order

1. A lexical diagnostic comes first, as in Core. A1 to A5 are lexical, so `#` at
   the end of a text still comes before a syntax error earlier in it.
2. Among the rest, the first rule the text breaks as the parser reads it comes
   first (`01-language.md` 14.1), and only then is it mapped. So `) x === 1` is
   `unexpected-token` at the `)`, and `x === 1 )` is `strict-equality`.
3. A text block's own code (F3 6.2) and `nesting-exceeded` are never mapped:
   `nesting-exceeded` at a token comes before any row of this section at that
   token.
4. A diagnostic after the first is not specified (`03-host.md` 3.7.2). An
   implementation may map the diagnostics it reports after the first by the same
   rows, and need not.

### 2.10 What stays `unexpected-token`

Every Core diagnostic for which no row holds stands. Among them: a text that
ends too soon (`1 +`, `(1`); two operands side by side (`1 2`, `{a 1}`); a stray
closing bracket; a comma where none may stand; a call of a call; a lambda where
none may stand (`x => x` at the top, `[x => x]`); a template or a text block
where no token may stand, as a key or as a `match` literal; a projection or a
literal of a package where it does not fit; a name at the start of a `match`
case other than `case`; a `?` with no `:` that is not directly followed by `[`;
and `_` read as a value.

### 2.11 The parse codes

With this package the `parse` phase has 58 codes: the 14 of `01-language.md`
14.2 and these 44.

| Code | Rows |
| --- | --- |
| `strict-equality` | C1, C2 |
| `missing-else` | D11 |
| `if-form` | E4, E5 |
| `branch-like-map` | D7 |
| `empty-branch` | D8 |
| `needs-parentheses` | D1 to D6, D16, E24 |
| `try-call` | D10 |
| `missing-catch` | D13 |
| `try-form` | D14, E6 |
| `catch-name` | D15 |
| `finally-clause` | B6, D12, E3 |
| `throw-statement` | B5 |
| `switch-statement` | E15 |
| `function-declaration` | B4 |
| `return-statement` | B3 |
| `let-binding` | B1 |
| `loop-statement` | B2 |
| `assignment` | C3, C4, E7 |
| `import-statement` | B7 |
| `export-statement` | E18 |
| `block-without-const` | B9, E23 |
| `lambda-in-pattern` | D9 |
| `slice-syntax` | E8 |
| `optional-index` | E21 |
| `star-form` | E11 |
| `filter-step` | E12 |
| `context-symbol` | A4 |
| `bitwise-operator` | A5, E10 |
| `pipe-operator` | C5 |
| `nil-literal` | B8 |
| `type-annotation` | E9 |
| `satisfies-operator` | E13 |
| `instanceof-operator` | E14 |
| `typeof-operator` | E16 |
| `angle-cast` | E20 |
| `type-declaration` | E17 |
| `as-const` | E1 |
| `negated-is` | E2, E25 |
| `text-block-quote` | C6, C7 |
| `string-prefix` | A1, A3, C9, C10 |
| `tagged-template` | C8 |
| `heredoc` | E19 |
| `hash-comment` | A2 |
| `unquoted-key` | E22 |

1. `text-block-open`, `text-block-close` and `text-block-indent` are F3's, and
   `bad-type-text` is F4's. This package gives them templates (3.4).
2. A minor version that gives one of these texts a meaning stops reporting its
   code. A code is never renamed or reused (`03-host.md` 3.7.3).

## 3. Messages of the parse and check phases

### 3.1 The catalog

1. The entries of the `parse` phase are in `suite/messages-parse.json`, and
   those of the `check` phase in `suite/messages-check.json`. Each file has the
   format of `03-host.md` 3.8.2, and every rule of 3.8.2 holds for it.
2. `messages-parse.json` has one entry for each of the 58 codes of 2.11, with 96
   templates. `messages-check.json` has one entry for each of the 29 check codes
   (`05-types.md` 16.1, F4 6.7), with 71 templates.
3. An entry of either file has no field `caught`.
4. The `cases` of an entry are case files of `03-host.md` 3.10 with the
   additions of 7, with no header. Each template has at least one case that
   states its message.
5. Each implementation generates its table from the files, as for the run phase.
   None writes a message by hand.

**How the catalog files combine.**

6. The catalog is `suite/messages.json` and the catalog files of packages:
   `suite/messages-parse.json` and `suite/messages-check.json` of this package,
   and a file `suite/messages-<name>.json` of another package, such as F7's
   `suite/messages-queries.json`. An implementation reads the files of the
   packages it offers and no other.
7. An entry for a code that no other file has is the entry of that code. Every
   entry of this package's two files is of this kind: the `parse` and `check`
   phases have no entry in `suite/messages.json`.
8. An entry for a code that `suite/messages.json` has, in a package's file, adds
   templates to that code (rulings BA10): its `phase` and `caught` are those of
   Core's entry, its `params` are among Core's, its template names are names
   Core's entry lacks, and its `when` and `cases` are for those templates. So
   F7's entry for `wrong-kind` adds the template `star`. It is not a second
   entry for the code: the code has the templates of every file read, and a
   template name appears in one file only.
9. No package's file changes or removes a template of another file.

### 3.2 Kinds of parameter

These rows join the table of `03-host.md` 3.8.3.

| Kind | Rendering |
| --- | --- |
| `token` | Made exact: the source text at the span of the diagnostic, as visible text (1.1). For the empty span at the end of input, the words `end of input`. For a span from `not` to the end of `in`, the text `not in` with one space, whatever stands between |
| `char` | `U+`, then the number of the code point in upper-case hex, with at least four digits. For a code point from U+0021 to U+007E, then a space, `(`, the code point, and `)`: `U+0040 (@)`, `U+00A0` |
| `text` | In the `check` phase: the source text at the span the template names, as visible text |

1. Each rendering is cut by `03-host.md` 3.8.3 item 1: one longer than 64 code
   points is its first 61 code points and `...`. A `token` and a `text` are cut
   after they are made visible.
2. **Types in a `check` message.** `03-host.md` 3.8.3 item 2, that a parameter
   is never taken from an inferred type, is a rule of `run` messages. A
   parameter of kind `type` in a `check` message is a type the checker gives at
   that place by `05-types.md`, printed by `05-types.md` 3.9. Each template of
   3.5 names which type.
3. A `word` is the name as written; for a call with a namespace, the namespace,
   `.` and the name: `geo.nowhere`. An operator is in its canonical text
   (3.8.3).
4. A `string` is the key after escapes, in its JSON form. A `number` is a whole
   number in the rendering of `01-language.md` 6.3.

### 3.3 Building a message

1. The template of the diagnostic is chosen by its condition (3.4, 3.5). Each
   hole `{name}` is replaced by the rendering of that parameter. Nothing else
   changes.
2. A message is one sentence with no full stop at its end (`03-host.md` 3.8.1
   item 4). It is at most the length of its template less its holes, plus 64
   code points for each hole: no `parse` or `check` message passes 400 code
   points.
3. A `parse` message is built for the first diagnostic. A `check` message is
   built for each diagnostic of the list.
4. A message holds no path, no span, and no suggestion found by search
   (`03-host.md` 3.8.3 item 3). A fixed fix written in a template is not a
   suggestion found by search.
5. Building a message charges nothing: neither phase is a run.

### 3.4 Parse templates

Each code's templates, with the condition that chooses each. For a targeted
code, the row of 2 that gives the code also names the template. Where a code has
one template, its name is `default`.

| Code | Template | Text | Chosen when |
| --- | --- | --- | --- |
| `unexpected-token` | `default` | `unexpected {token}` |  |
| `unexpected-character` | `default` | `unexpected character {char}` |  |
| `unterminated-string` | `string` | `the string is not closed on its line` | A one-line string or a raw string has a line end or the end of input before its closing quote |
|  | `template` | `the template has no closing backtick` | The end of input comes inside a template, the innermost literal still open |
|  | `text-block` | `the text block has no closing delimiter` | The end of input comes inside a text block or a raw text block, or directly after a blank opening line, the innermost literal still open |
|  | `part` | `the part has no closing delimiter` | The end of the text of a host template comes inside a part (package F2) |
| `unterminated-comment` | `default` | `the block comment has no closing */` |  |
| `bad-escape` | `unknown` | `{token} is not an escape` | A \ followed by a code point that begins no escape of the literal it stands in. token is the \ and that code point |
|  | `unicode` | `{token} is not a complete Unicode escape` | \u not followed by four hex digits or by {, or \u{ with no hex digit, more than six, or no } after them. token is the span of the diagnostic |
|  | `range` | `{token} names no code point` | A \u{X...} whose number is above U+10FFFF or in U+D800 to U+DFFF. token is the escape |
|  | `surrogate` | `{token} is half of a surrogate pair` | A \uXXXX in U+D800 to U+DBFF with no partner after it, or one in U+DC00 to U+DFFF that completes no pair. token is that escape |
| `line-continuation` | `default` | `a \ before a line end does not join lines; write \\ for a backslash` |  |
| `bad-number` | `default` | `{token} is not a number` |  |
| `literal-range` | `whole` | `{token} is larger than the largest whole number, 9007199254740991` | A number with no fraction and no exponent, or a hex number, that is not a whole number |
|  | `finite` | `{token} is too large to be a finite number` | Any other number token whose nearest binary64 value is not finite |
| `text-block-open` | `default` | `only spaces and tabs may follow the opening delimiter of a text block on its line` |  |
| `text-block-close` | `default` | `only the indentation may stand before the closing delimiter of a text block on its line` |  |
| `text-block-indent` | `tab` | `the line has a tab where the indentation has a space` | The first code point that differs is a tab, and the indentation has a space there |
|  | `space` | `the line has a space where the indentation has a tab` | The first code point that differs is a space, and the indentation has a tab there |
|  | `default` | `the line does not start with the indentation of the closing delimiter` | Every other difference |
| `nesting-exceeded` | `default` | `{token} opens a level past the limit of 100 levels of nesting` |  |
| `bad-type-text` | `value` | `{token} is a value, not a type; compare with == to test a value` | Where a type is expected: a number, true, false or - |
|  | `number-word` | `number is not a type; write int or float` | Where a type is expected: the word number |
|  | `boolean-word` | `boolean is not a type; write bool` | Where a type is expected: the word boolean |
|  | `null-word` | `{token} is not a type; write null` | Where a type is expected: the word undefined, void or nil |
|  | `object-word` | `object is not a type; write a record type or map<T>` | Where a type is expected: the word object |
|  | `letter` | `a type name has two characters or more, and {token} has one` | Where a type is expected: one upper-case letter |
|  | `lower` | `{token} is not a type; a type name starts with an upper-case letter` | Where a type is expected: any other lower-case word, a reserved word, or _ |
|  | `literal` | `a template or a text block cannot stand in a type` | A template, a text block or a raw text block where a type or a key is expected |
|  | `expected` | `a type is expected, found {token}` | Where a type is expected: any other token, or the end of input |
|  | `split` | `write a space between the > that closes map< and the = after it` | The token >= stands where the > of map< is expected |
|  | `map-open` | `map is followed by <, found {token}` | A token other than < after the word map |
|  | `map-close` | `map< is closed by >, found {token}` | Any other token where the > of map< is expected |
|  | `paren` | `a ( in a type is closed by ), found {token}` | A token where the ) of a parenthesized type is expected |
|  | `record` | `{token} cannot stand here in a record type` | In a record type: a token that cannot begin a field, a rest or the }; a token after a key that is not ? or :; a token after ? that is not :; a token after a field or a rest that is not , or }; a field after a rest |
|  | `duplicate` | `key {key} is written twice in the record type` | A key written twice in one record type. key is that key, after escapes |
| `reserved-word` | `name` | `{token} is a reserved word and cannot be a name` | Where the grammar needs a Name (01-language.md 14.2 rule 1) |
|  | `operand` | `{token} is a reserved word and cannot stand here` | Where the grammar needs the start of an Expr or an operand (01-language.md 14.2 rule 2) |
| `strict-equality` | `equal` | `=== is not an operator; write ==, which converts nothing` | The token === |
|  | `not-equal` | `!== is not an operator; write !=, which converts nothing` | The token !== |
| `missing-else` | `default` | `an if needs an else branch, found {token}` |  |
| `if-form` | `form` | `the test of an if is written in parentheses and each branch in braces, found {token}` | A token other than ( after if, or other than { after the test or after else |
|  | `call` | `if is not a function; write c ? a : b` | A , after the test of an if |
| `branch-like-map` | `default` | `after if, else, try and catch a brace opens a branch, not a map; write the map inside a second pair of braces` |  |
| `empty-branch` | `default` | `a branch needs a value; write null, or an empty map inside a second pair of braces` |  |
| `needs-parentheses` | `power` | `a prefix {token} cannot apply directly to a power; put the operand or the power in parentheses` | 01-language.md 3.4 rule 1. token is the prefix operator |
|  | `in` | `a prefix ! cannot stand before in; write not in, or put the ! and its operand in parentheses` | 01-language.md 3.4 rule 2 |
|  | `coalesce` | `put one side of {token} in parentheses: ?? does not mix with \|\|, &&, ==, !=, <, <=, >, >=, in, not in, as or is without them` | 01-language.md 3.4 rule 3. token is the operator that cannot follow |
|  | `after-branch` | `an if or a try is a whole expression; put it in parentheses before {token}` | 01-language.md 3.4 rule 4. token is the token that cannot follow |
|  | `operand` | `put the {token} in parentheses: it cannot stand as an operand` | 01-language.md 3.4 rule 5. token is the if or the try |
|  | `match-subject` | `a match needs a subject, and a subject that starts with {token} is written in parentheses` | 01-language.md 3.4 rule 6. token is the first token of the subject |
|  | `not-test` | `a prefix ! cannot stand before {op}; write !(x {op} T), or put the ! and its operand in parentheses` | F4 2.5. op is the word as or is |
|  | `after-type` | `put the as or is test in parentheses before {token}` | F4 2.4: an operator of levels 7 to 10, ., ?., ?.[, a [ not followed by ], ( or { after a program type. token is that token |
| `try-call` | `default` | `try is not a function; write try and catch, each followed by a branch in braces` |  |
| `missing-catch` | `default` | `a try needs a catch, found {token}` |  |
| `try-form` | `default` | `the body of a try and its handler are written in braces, found {token}` |  |
| `catch-name` | `default` | `the parentheses after catch hold one name, found {token}; leave them out when the error is not used` |  |
| `finally-clause` | `default` | `there is no finally: nothing in a program has an effect to undo` |  |
| `throw-statement` | `default` | `there is no throw; write fail(message)` |  |
| `switch-statement` | `default` | `there is no switch or case; write a match` |  |
| `function-declaration` | `default` | `there is no function declaration; bind a lambda with const` |  |
| `return-statement` | `default` | `there is no return: the value of a block is its last expression` |  |
| `let-binding` | `default` | `there is no {token}; write const, since a binding never changes` |  |
| `loop-statement` | `default` | `there are no loops or comprehensions in 3.0; use map, filter or reduce` |  |
| `assignment` | `assign` | `a binding cannot be reassigned; bind a new name with const, or compare with ==` | A = after a complete operand |
|  | `compound` | `{token} does not exist: a binding cannot be changed; bind a new name with const` | One of +=, -=, *=, /=, %=, **=, ??=, \|\|=, &&=. token is that token |
|  | `increment` | `++ does not exist: a binding cannot be changed; write x + 1` | The token ++ |
| `import-statement` | `default` | `there are no imports in 3.0; a host passes shared data as a variable` |  |
| `export-statement` | `default` | `a program exports no name; its value is its last expression` |  |
| `block-without-const` | `default` | `a lambda body in braces is a map; write the expression without braces, or start a block with a const line` |  |
| `lambda-in-pattern` | `default` | `a lambda is bound to a name, not to a pattern` |  |
| `slice-syntax` | `default` | `there is no slice syntax; write x.slice(a, b)` |  |
| `optional-index` | `default` | `an optional index is written ?.[ with a dot` |  |
| `star-form` | `default` | `a projection is written xs[*].f` |  |
| `filter-step` | `default` | `there is no filter step; write xs.filter(x => test)` |  |
| `context-symbol` | `default` | `$ names nothing; write the name of the variable` |  |
| `bitwise-operator` | `and` | `& is not an operator; write && for a logical and` | The code point & |
|  | `or` | `\| is not an operator here; write \|\| for a logical or` | The token \| |
| `pipe-operator` | `default` | `\|> is not an operator; write f(x), or call a method` |  |
| `nil-literal` | `default` | `nil is not a value; write null` |  |
| `type-annotation` | `default` | `types are inferred; remove the annotation` |  |
| `satisfies-operator` | `default` | `satisfies is not an operator; x as T tests a value when the program runs` |  |
| `instanceof-operator` | `default` | `instanceof is not an operator; write x is T` |  |
| `typeof-operator` | `default` | `typeof is not an operator; write type(x) for the kind, or x is T for a test` |  |
| `angle-cast` | `default` | `<T>x is not a cast; write x as T` |  |
| `type-declaration` | `default` | `a program cannot declare a type with {token}; write the type in place, or use a name the host declared` |  |
| `as-const` | `default` | `as const is not needed, because no value can change; remove it` |  |
| `negated-is` | `default` | `is has no negated form; write !(x is T)` |  |
| `text-block-quote` | `default` | `a text block is written with three double quotes, not '''` |  |
| `string-prefix` | `prefix` | `{token} is not a string prefix; only a lower-case r directly before the quote makes a raw string` | A word, or $, directly before a quote or a backtick. token is the word or the $ |
|  | `space` | `the prefix r stands directly before the quote, with nothing between` | The word r or R, then white space or a comment, then a string |
|  | `guard` | `a raw string has no # guards; write the text in another form, or interpolate what a raw string cannot hold` | A # directly before a quote, or directly after the word r |
| `tagged-template` | `default` | `there are no tagged templates; raw text is written r"..." or as a raw text block` |  |
| `heredoc` | `default` | `there are no heredocs; write a text block` |  |
| `hash-comment` | `default` | `a comment is written // to the end of the line, or /* */` |  |
| `unquoted-key` | `default` | `a key that is not a word is quoted: write {key}` |  |

The parameters of each code, with their kinds:

| Code | Parameters |
| --- | --- |
| `unexpected-token` | `token` (token) |
| `unexpected-character` | `char` (char) |
| `bad-escape` | `token` (token) |
| `bad-number` | `token` (token) |
| `literal-range` | `token` (token) |
| `nesting-exceeded` | `token` (token) |
| `bad-type-text` | `token` (token), `key` (string) |
| `reserved-word` | `token` (token) |
| `missing-else` | `token` (token) |
| `if-form` | `token` (token) |
| `needs-parentheses` | `token` (token), `op` (word) |
| `missing-catch` | `token` (token) |
| `try-form` | `token` (token) |
| `catch-name` | `token` (token) |
| `let-binding` | `token` (token) |
| `assignment` | `token` (token) |
| `type-declaration` | `token` (token) |
| `string-prefix` | `token` (token) |
| `unquoted-key` | `key` (string) |

**`bad-type-text`.** The rows of F4 2.6 choose the template, in this order: a
template, a text block or a raw text block, `literal`; the token `>=` where the
`>` of `map<` is expected, `split`; a token other than `<` after `map`,
`map-open`; any other token where that `>` is expected, `map-close`; where the
`)` of a parenthesized type is expected, `paren`; a key written twice,
`duplicate`; any other error in a record type, `record`. Where a type is
expected, by the token found: a number token, `true`, `false` or `-`, `value`;
`number`, `boolean`, `undefined`, `void`, `nil` or `object`, the template of
that word; one upper-case letter, `letter`; any other lower-case word, a
reserved word or `_`, `lower`; any other token or the end of input, `expected`.

**`unterminated-string`.** The literal is the innermost that is still open, the
one whose span the diagnostic has (`01-language.md` 2.11 item 2).

**`text-block-indent`.** The first code point of the line that differs from the
indentation (F3 4.5) decides: a tab where the indentation has a space, `tab`; a
space where it has a tab, `space`; anything else, an escape, an interpolation
and a line shorter than the indentation among them, `default`.

**`reserved-word`.** `name` for 14.2 rule 1, `operand` for rule 2.

### 3.5 Check templates

Each code's templates, with the condition that chooses each and the source of
each parameter. A parameter of kind `type` is printed by `05-types.md` 3.9 and
is the type the checker gives at that place, as named.

| Code | Template | Text | Chosen when |
| --- | --- | --- | --- |
| `undefined-name` | `name` | `{name} is not defined` | An identifier that resolves to nothing (05-types.md 22.4) |
|  | `type` | `no type is named {name}` | A type name in a program type that the environment does not declare (F4 6.1) |
| `undefined-function` | `default` | `no function is named {name}` |  |
| `not-a-value` | `namespace` | `{name} is a namespace, not a value` | An identifier that names a namespace |
|  | `function` | `{name} is a function, not a value; call it` | An identifier that names a host function or a library function |
|  | `lambda` | `{name} is a const-bound lambda, not a value; call it, or pass it to a library entry that takes a function` | An identifier whose innermost binding is a const-bound lambda |
|  | `argument` | `a function cannot stand at parameter {param} of {name}, which takes a value` | A lambda argument at a parameter that is not an Fn. name is the callee; param is the parameter's name in the signature |
| `call-of-parameter` | `default` | `{name} is a value, not a function, and cannot be called` |  |
| `lambda-to-lambda` | `default` | `a function cannot be passed to {name}, which is a const-bound lambda; pass the data instead` |  |
| `duplicate-name` | `default` | `{name} is already bound here` |  |
| `shadows-namespace` | `default` | `{name} is a namespace and cannot be bound` |  |
| `duplicate-key` | `default` | `key {key} is written twice in this map` |  |
| `no-field` | `default` | `{type} has no field {key}` |  |
| `no-method` | `default` | `a value of type {type} has no method {name}` |  |
| `maybe-absent` | `field` | `field {key} may be absent from {type}` | A field or a String key. type is the record read |
|  | `index` | `the key may be absent from {type}` | An index read of a record with any other index. type is the record read |
| `maybe-null` | `default` | `{text} may be null; its type is {type}` |  |
| `kind` | `operand` | `{op} cannot take a value of type {type}` | An operand of -, *, /, %, unary -, or the left operand of **, that does not fit float; an operand of +, <, <=, > or >= that fits neither float nor string. type is the operand's type without null |
|  | `exponent` | `the exponent of ** must be of type int, found {type}` | The right operand of ** does not fit int. type is its type without null |
|  | `operands` | `{op} cannot take values of types {left} and {right}` | The two operands of +, <, <=, > or >= each fit float or string, and not the same one. left and right are their types without null |
|  | `in-left` | `the left operand of {op} must be a string here, found {type}` | The right operand of in or not in is a string or a record, and the left does not fit string. type is the left operand's type without null |
|  | `in-right` | `the right operand of {op} must be a list, a string or a map, found {type}` | The right operand of in or not in has two kinds or more, or another kind. type is its type without null |
|  | `receiver` | `a read or a method call needs one kind, and {type} has more than one` | 05-types.md 9.1 step 4. type is the receiver's type then |
|  | `field-read` | `a value of type {type} has no fields to read` | A field step on a receiver that is not a record. type is the receiver's type |
|  | `index-read` | `a value of type {type} cannot be indexed` | An index step on a receiver that is not a record, a list or a string. type is the receiver's type |
|  | `index` | `a list or string index must be of type int, found {type}` | The index of a list or a string does not fit int. type is the index's type without null |
|  | `key` | `a map key must be of type string, found {type}` | A computed key of a map literal, or the index of a record, does not fit string. type is its type without null |
|  | `spread-list` | `only a list can be spread into a list, found {type}` | type is the operand's type without null |
|  | `spread-map` | `only a map can be spread into a map, found {type}` | type is the operand's type without null |
|  | `pattern-list` | `a list pattern needs a list, found {type}` | type is the value bound's type without null |
|  | `pattern-map` | `a map pattern needs a map, found {type}` | type is the value bound's type without null |
|  | `rendered` | `a value of type {type} cannot be rendered as text, because it holds an opaque type` | A rendered expression whose type has an opaque type. type is its type |
|  | `star` | `a projection needs a list, found {type}` | The receiver of a star of F7 is not a list. type is the receiver's type without null |
| `never-equal` | `equality` | `a value of type {left} is never equal to a value of type {right}` | == or !=. left and right are the operands' types |
|  | `in` | `a value of type {left} is never an element of a list of type {right}` | in or not in on a list. left is the left operand's type, right the right operand's |
|  | `match` | `the subject, of type {type}, is never equal to {text}` | A literal of a match. type is the subject's type; text is the literal |
| `always-truthy` | `default` | `{text} is always truthy: its type is {type}` | type is the tested expression's type, as narrowed |
|  | `test` | `{text} is always true for a value of type {type}` | An x is T that the types decide (F4 6.3). type is the subject's type |
| `always-falsy` | `default` | `{text} is always falsy: its type is {type}` | type is the tested expression's type, as narrowed |
|  | `test` | `{text} is always false for a value of type {type}` | An x is T that the types decide (F4 6.3). type is the subject's type |
| `zero-divisor` | `default` | `the divisor is the number 0` |  |
| `arity` | `exact` | `the argument count of {name} must be {required}, found {count}` | Required and at most are equal |
|  | `range` | `the argument count of {name} must be from {required} to {most}, found {count}` | At most is more than required |
|  | `at-least` | `the argument count of {name} must be at least {required}, found {count}` | The last parameter is a rest parameter |
|  | `parameters` | `the function given to {name} declares more than {most} parameters` | A lambda argument with too many parameters. most is the count its Fn gives |
| `arg-mismatch` | `argument` | `expected {expected}, found {found}` | A place of failure of a plain argument (05-types.md 15.1). expected is the type there; found is the type of the expression there |
|  | `missing` | `required field {key} has no entry` | A map with no entry for a required field. key is the least such key |
|  | `extra` | `{expected} does not allow key {key}` | An entry whose key the record does not list, with rest never. expected is the record |
|  | `function` | `parameter {param} of {name} takes a function` | A plain argument at an Fn parameter |
|  | `body` | `the function given to {name} gives {found}, which does not fit {expected}` | A lambda body that does not fit R. found is the body's type; expected is R with its Vars replaced |
|  | `ordered` | `{name} needs all numbers or all strings, found {found}` | A bound ordered that does not hold. found is the solution |
|  | `number` | `{name} needs numbers, found {found}` | A bound number that does not hold. found is the solution |
| `cannot-infer` | `default` | `the types of the parameters of this function cannot be inferred from the call of {name}` |  |
| `check-limit` | `nesting` | `checking this use of {name} passes 100 levels of expanded nesting` | 05-types.md 20.2 |
|  | `bodies` | `the program checks more than 1000 lambda bodies` | 05-types.md 20.3 |
|  | `depth` | `the type here is more than 64 levels deep` | 05-types.md 20.4; a program type (F4 6.1) |
|  | `size` | `the type here has more than 100000 parts` | 05-types.md 20.7; a program type (F4 6.1) |
| `duplicate-case` | `default` | `{text} is listed twice in this match` |  |
| `unreachable-arm` | `default` | `this arm comes after _ and is never chosen` |  |
| `non-exhaustive` | `default` | `the match has no _ arm and does not cover every value of type {type}` |  |
| `result-mismatch` | `result` | `expected {expected}, found {found}` | A place of failure (05-types.md 15.1). expected is the type there; found is the type of the expression there |
|  | `missing` | `required field {key} has no entry` | A map with no entry for a required field. key is the least such key |
|  | `extra` | `{expected} does not allow key {key}` | An entry whose key the record does not list, with rest never. expected is the record |
| `unchecked` | `default` | `the check cannot show the kind of {text}, and this environment rejects dynamic sites` |  |
| `may-fail` | `index` | `this index may be outside its list or string, and this environment rejects index sites` | A site of class index |
|  | `divide` | `this divisor may be zero, and this environment rejects divide sites` | A site of class divide |
|  | `value` | `this call of {name} may fail on a value, and this environment rejects value sites` | A site of class value on a call. name is the entry's name |
|  | `test` | `this test may fail on a value, and this environment rejects value sites` | A site of class value on an as (F4 6.6) |
|  | `host` | `{name} may return an error, and this environment rejects host sites` | A site of class host. name is the function's or the method's name |
| `never-conforms` | `default` | `no value of type {found} conforms to {expected}` |  |
| `untagged-union` | `default` | `two record types in one union need a field that tells them apart` |  |

The parameters of each code, with their kinds:

| Code | Parameters |
| --- | --- |
| `undefined-name` | `name` (word) |
| `undefined-function` | `name` (word) |
| `not-a-value` | `name` (word), `param` (word) |
| `call-of-parameter` | `name` (word) |
| `lambda-to-lambda` | `name` (word) |
| `duplicate-name` | `name` (word) |
| `shadows-namespace` | `name` (word) |
| `duplicate-key` | `key` (string) |
| `no-field` | `type` (type), `key` (string) |
| `no-method` | `type` (type), `name` (word) |
| `maybe-absent` | `type` (type), `key` (string) |
| `maybe-null` | `text` (text), `type` (type) |
| `kind` | `op` (word), `type` (type), `left` (type), `right` (type) |
| `never-equal` | `left` (type), `right` (type), `type` (type), `text` (text) |
| `always-truthy` | `text` (text), `type` (type) |
| `always-falsy` | `text` (text), `type` (type) |
| `arity` | `name` (word), `count` (number), `required` (number), `most` (number) |
| `arg-mismatch` | `expected` (type), `found` (type), `key` (string), `name` (word), `param` (word) |
| `cannot-infer` | `name` (word) |
| `check-limit` | `name` (word) |
| `duplicate-case` | `text` (text) |
| `non-exhaustive` | `type` (type) |
| `result-mismatch` | `expected` (type), `found` (type), `key` (string) |
| `unchecked` | `text` (text) |
| `may-fail` | `name` (word) |
| `never-conforms` | `expected` (type), `found` (type) |

1. **The type there.** At a place of failure of `05-types.md` 15.1, `found` is
   the type synth gave the expression at that place, and `expected` is the type
   that place is fitted against: for an item, the element type E; for an entry's
   value, the field's type, or the rest; for an argument, its parameter type
   with every `Var` replaced by its solution, an unbound `Var` by `never`. With
   F5 a tagged union adds its row and its types.
2. **`missing` and `extra`.** `missing` is reported once, on the whole map, and
   names the least key by code point of the required fields with no entry.
   `extra` names the key of the entry it is reported on.
3. **`maybe-null`.** The type is the type at the span: the operand's, the
   receiver's before the null step of `05-types.md` 9.1, the value bound's, the
   computed key's, or the type at a place of failure.
4. **`kind`.** The template is chosen by the rule of `05-types.md` that reports
   it, and `star` by the rule of F7 for a star's receiver. For an operand the
   type is shown without its null alternative, since a null is `maybe-null`.
5. **`arity`.** required and most are the counts of `05-types.md` 22.3. For a
   method call argument zero is not counted, and count is the number of
   arguments written. For a `const`-bound lambda they are its own counts.
   `parameters` gives most as the parameters of the `Fn`.
6. **`always-truthy`, `always-falsy`.** `test` is chosen for an `As` node of F4,
   and `default` for every other tested expression.
7. **`may-fail`.** The template is the class of the site. For a `value` site on
   a call, name is the method's or the function's name; on an `As` node of F4,
   `test`. For a `host` site, name is the host function's name with its
   namespace, or the method's name.
8. **`never-equal`.** `match` for a literal of a `match`, whose text is the
   literal with its `-`; `in` for `in` and `not in` on a list; `equality` for
   `==` and `!=`.

### 3.6 Stability

A `parse` or `check` message may change in a minor version (`03-host.md` 3.8.4).
A program cannot see one: it never reaches a run.

## 4. The trace

### 4.1 What it holds

1. The diagnostic of a failed run has the field `trace`: the calls in progress
   when the run failed, from the outermost in. It may be empty. A diagnostic of
   another phase has no trace.
2. **A call in progress** is one of:
   - a call by name of a `const`-bound lambda, from its depth test to its end
     (`02-evaluation.md` 8.4 items 4 and 7): while its parameters are bound,
     while its defaults run and while its body runs;
   - a call of a library entry, while a call it made of a lambda passed to it
     has not returned: from the depth test of that call of the lambda to its
     end.
3. Nothing else has an entry: not a call by name whose arguments are still being
   evaluated, not a library call before it calls its lambda, between two calls
   of it, or after the last, not a library call that calls no lambda, not a host
   function or method, not a `try`, not a walk.
4. So a failure that a library entry raises itself, such as a callback's result
   of a wrong kind (`flatMap`) or keys that are not ordered (`sortBy`), has no
   entry for that call: the call of the lambda has returned. The failure's own
   span is the call.
5. A lambda passed by name, as in `xs.map(f)`, is called by the library entry,
   not by name: the entry is the library call, and `f` has none.
6. A projection of F7 has no entry. Its element is not a call.

### 4.2 An entry

| Field | Holds |
| --- | --- |
| `span` | The span of the call: the `Call` node, from its name or namespace to its `)`; the `MethodCall` node, from the start of its chain to its `)` (`05-types.md` 1.2) |
| `name` | For a `const`-bound lambda, its name. For a library entry, its name as written: `map`, `reduce` |
| `index` | For a library entry, the index of the element the entry gave the call of the lambda, from 0. For a `const`-bound lambda, null |

1. An entry's index is the element's position in the receiver as the entry
   visits it. For `reduce` it is the position of `item`. For `sortBy` it is the
   element whose key is being made.
2. A package that adds a library entry that calls a lambda states its index. An
   entry whose callback is given a key and no index gives the position of the
   key in key order, from 0.

### 4.3 Bound and cost

1. **Length.** Each call in progress counts at least 1 against the depth limit
   (`02-evaluation.md` 8.4), so a trace has at most as many entries as the depth
   limit, and at most 1,000. It is never cut.
2. **Cost.** None. A trace is not a value of the language, and a program cannot
   read it. Building it charges no step and no unit, and a run that does not
   fail builds none.
3. **A caught failure** has no trace that anything can see. A failure raised in
   a handler has the trace of the place where it is raised: the calls in
   progress then, among them calls outside the `try`.
4. **A limit failure** has a trace, and no case compares it, as its span is not
   compared (`03-host.md` 3.10.7 item 7).
5. **A failure of the conformance function** and of `ConformAll` has an empty
   trace, and no span.
6. Two implementations give the same trace for the same failure: the calls in
   progress are fixed by the order of evaluation of `02-evaluation.md`.

### 4.4 Examples

```ts
const total = o => o.lines.map(l => l.qty * l.price).sum();
orders.map(o => total(o))
```

With the third order's second line not finite, the diagnostic is `number-range`
at `1:37-1:52`, and its trace, from the outermost in, is `map` `[2, 1, 2, 26]`
with index 2, `total` `[2, 17, 2, 25]` with index null, and `map` `[1, 20, 1,
53]` with index 1.

| Program | Trace |
| --- | --- |
| `const f = x => 1 / x; f(0)` | `f` `[1, 23, 1, 27]`, null |
| `[1, 0].map(x => 1 / x)` | `map` `[1, 1, 1, 23]`, 1 |
| `const f = x => 1 / x; f(f(0))` | `f` `[1, 25, 1, 29]`, null: the outer `f` is not in progress while its argument runs |
| `[[1], 2].flatMap(x => x)` | Empty: `flatMap` tests the result after its call of the lambda returned |
| `try { [0].map(x => 1 / x) } catch { [1, 0].map(x => 1 / x) }` | `map` `[1, 37, 1, 59]`, 1: the handler's call |

## 5. Renderings

### 5.1 Common rules

1. A rendering is a function of a list of diagnostics and, for the detail form,
   the source text. Every binding gives the three forms. The diagnostics are
   rendered one after another, in the order of the list, and each one's text
   ends with U+000A.
2. A rendering is the same text, code point for code point, in every
   implementation.
3. A rendering holds only the fields this section names. `help`, `fix`,
   `expected`, `found`, `cause` and an offset are never written in it.
4. **A position** is written `L:C`, the line and the column (`03-host.md`
   3.7.7). **A span** is written `L1:C1-L2:C2`, always with all four numbers.
5. **A path** is written from its steps, in order: a key that is a `Word`
   (`01-language.md` 2.4 rule 1, a reserved word included) as `.` and the key;
   any other key as `[`, its JSON form, `]`; an index as `[`, its number, `]`. A
   key is cut first, as a parameter is (3.2 item 1): a `Word` key longer than 64
   code points to its first 61 and `...`, any other key's JSON form likewise. So
   `["lines", 1, "qty"]` is `.lines[1].qty`, and `["a-b", "x"]` is `["a-b"].x`.
6. **The message**, in the one-line and detail forms, is written as visible text
   (1.1). A message from a template has no code point to escape; the message of
   `failed` and of `function-error` may.

### 5.2 One line

| Part | Text |
| --- | --- |
| 1 | `error[`, the code, `]` |
| 2 | With a span: a space and the span. With none: nothing |
| 3 | `: ` and the message |
| 4 | With a path of one step or more: ` at ` and the path |
| 5 | U+000A |

So `error[divide-by-zero] 1:5-1:10: the divisor of / is zero`, and with no span
`error[not-conforming]: expected int, found "2" at .lines[1].qty`. The word is
always `error`: there is no severity.

### 5.3 Detail

The detail form of one diagnostic is these lines, each ending with U+000A.

| Line | Text | When |
| --- | --- | --- |
| 1 | The one-line form, without its U+000A | Always |
| 2 | Four spaces, then the excerpt (5.4) | When the diagnostic has a span and a source text is given |
| 3 | Four spaces, then the caret line (5.4) | The same |
| 4 and on | One line for each entry of the trace, the innermost first: two spaces, `in `, the name, then `, element ` and the index when the index is not null, then `, at ` and the span | For a run diagnostic whose trace is not empty |

1. With no source text, or no span, lines 2 and 3 are left out. The trace lines
   are still written: they need no source text.
2. Nothing is colored. The excerpt of an empty line is four spaces.

### 5.4 The excerpt and the caret line

1. **The source text** is the text that was parsed, after a U+FEFF at its start
   is removed (`01-language.md` 2.1 item 2). Its lines are separated by U+000A
   only (`03-host.md` 3.7.7). A U+000D is a code point of its line.
2. **The line** is the line of the start of the span: its code points, without
   the U+000A that ends it. Let n be its number of code points. The end of input
   after a final U+000A is on an empty line, with n = 0.
3. **The marked columns** are from a, the start column, to b, exclusive. b is
   the end column when the span ends on the same line, and n + 1 when it ends on
   a later line. When b is not more than a, b is a + 1: an empty span, at the
   end of input or at the end of a line, marks one column.
4. **Writing the line.** Each control character, U+0000 to U+001F and U+007F to
   U+009F, a tab and a U+000D among them, is written as one space. Every other
   code point is written as itself, a wide or a combining character among them.
   So every column is one code point of the text written.
5. **A long line.** The window is 120 columns wide. When n is at most 120, the
   whole line is written, and s = 1. Otherwise the window starts at column s = a
   − 40, raised to 1 when it is less, and lowered to n − 119 when it is more.
   The excerpt is then `...` when s is more than 1, the columns from s to s +
   119, and `...` when s + 120 is not more than n.
6. **The caret line** is the spaces before the carets, then the carets. The
   spaces: 3 when the excerpt starts with `...`, plus a − s. The carets: one `^`
   for each column from a to the lesser of b and s + 120, exclusive; at least
   one.
7. Columns count code points (`03-host.md` 3.7.7). A wide character or a
   combining character puts the carets out of line in a terminal. That is
   accepted: the rule is the same in every implementation.

### 5.5 JSON

1. The JSON form of one diagnostic is one object, written in the JSON form of
   `01-language.md` 12: no white space, keys in code point order, strings with
   its escapes, numbers in their rendering. Then U+000A.
2. Its keys are those of this table that the diagnostic has.

| Key | Value | Present |
| --- | --- | --- |
| `class` | The class | When the diagnostic has one (`03-host.md` 3.7.1) |
| `code` | The code | Always |
| `data` | The `data` (5.6) | For `failed` |
| `message` | The message, whole | Always |
| `path` | The path, a list of strings and numbers | For `not-conforming` |
| `phase` | The phase | Always |
| `span` | The span, a list of four numbers | When the diagnostic has one |
| `trace` | The trace, a list of objects with the keys `index`, `name` and `span`; `index` is a number or null | For every `run` diagnostic, empty or not |

So `{"class":"divide","code":"divide-by-zero","message":"the divisor of / is
zero","phase":"run","span":[1,5,1,10],"trace":[]}` is one line.

### 5.6 `data` in the JSON form

1. `data` is written in its JSON form, with each opaque value, at any depth,
   written as `null`.
2. When that form would have nesting deeper than 200, as the JSON text of
   `03-host.md` 3.2.4 counts it, or would hold more than 10,000 code points,
   `data` is written as `null`. An implementation finds this by writing at most
   that much: the work is bounded, also for host data that holds a cycle or
   shares a list many times.

### 5.7 Example

For the program of 4.4, the detail form is:

```
error[number-range] 1:37-1:52: the result of * is out of range
    const total = o => o.lines.map(l => l.qty * l.price).sum();
                                        ^^^^^^^^^^^^^^^
  in map, element 1, at 1:20-1:53
  in total, at 2:17-2:25
  in map, element 2, at 2:1-2:26
```

## 6. Every failure of a conformance test

### 6.1 The function

Every binding gives a second conformance function beside the one of F4 7.

| Binding | Form |
| --- | --- |
| Go | `risor.ConformAll(v, t, max)`, with `t` from `env.ParseType(text)`, which returns a `[]Diagnostic` |
| TypeScript | `env.conformAll(value, typeText, max)`, which returns a list of diagnostics |
| Rust | `env.ty(text)?.conform_all(&value, max)`, which returns a `Vec<Diagnostic>` |

| Matter | Rule |
| --- | --- |
| Takes | A value, a type resolved as F4 7.1 says, and a maximum, a whole number. A maximum less than 1 is read as 1 |
| Gives | A list of `not-conforming` diagnostics, at most the maximum, in the order of 6.2. Each is as the function of F4 7 gives one: class `value`, a path, a message by F4 4.4, no span and an empty trace. The list is empty exactly when the value conforms |
| Agrees with | Its first diagnostic, when there is one, is the diagnostic that the function of F4 7, `x as T` and `x is T` give: the same path and the same message, for every value and type |
| Changes | Nothing. The value is not copied, filled in or trimmed |
| Cost | None. It is not a run: it has no program, no usage, no limits and no cancellation |

### 6.2 The walk of every failure

**All**(v, T, path) adds the failures of v against a resolved type T to a list,
in this order. The walk stops at once when the list holds the maximum.

| Step | Rule |
| --- | --- |
| 1. `any` | If T is `any`: nothing. v is not looked at |
| 2. A value that cannot enter | If v is a host number that cannot enter the language: one failure at path, template `wrong` |
| 3. The alternative | Take the alternative of T of v's kind, as F4 4.2 step 5. None: one failure at path, template `wrong`. Nothing inside v is visited |
| 4. Scalars | As F4 4.2 step 6: an `int` that is not whole, or a string not in a literal set, is one failure at path, template `wrong` |
| 5. A list, E`[]` | All(v[i], E, path + [i]) for each index i in order |
| 6. A record, R | First the declared fields, in code point order of their keys: a required field v lacks is one failure at path + [k], template `missing`; a field v has is All(v[k], the field's type, path + [k]); an optional field v lacks adds nothing. Then the other keys of v, in code point order, always, whatever the declared fields gave: with rest `any`, nothing; with rest `never`, one failure at path + [k], template `extra`, for each other key k; with another rest U, All(v[k], U, path + [k]) for each other key k |

1. **The first failure.** The walk of F4 4.2 stops at the first failure of a
   list, stops at the first failing declared field, reaches the other keys only
   when every declared field passed, and for a rest U reports the least failing
   key. All visits in the same order and goes on, so its first failure is that
   failure.
2. **The other keys** are reached by All even after a declared field failed.
   With an exact record, each key it does not list is a failure of its own, not
   only the least.
3. **Its work** is bounded by the value, as F4 7.1 item 2 says, and what it
   builds by the maximum. **Its depth** is bounded by the type, at most 64.
4. A host that must say "and more" asks for one more than it shows.

### 6.3 Example

The type `{id: string, lines: {qty: int, sku: string}[]}`, exact at both levels,
and the value `{"id": 7, "lines": [{"sku": "x", "qty": 1}, {"sku": "y", "qty":
"2"}], "note": 1}`. With a maximum of 3, the failures are `["id"]` `expected
string, found 7`, `["lines", 1, "qty"]` `expected int, found "2"`, and
`["note"]` `the type does not allow key "note"`. With a maximum of 2, the first
two. The function of F4 7 gives the first.

## 7. Additions to the case format

Each addition is optional. Its absence leaves every existing case file valid,
with its meaning unchanged. A runner of an implementation that does not offer F8
does not run `suite/errors/`, `suite/messages-parse.json` or
`suite/messages-check.json`; for it these fields are unknown fields
(`03-host.md` 3.11 item 3).

### 7.1 `message` of a `parse` and of a `check` diagnostic

A case may state `message` in an expected diagnostic of phase `parse` or
`check`. It is compared whole, code point by code point, as for `run`
(`03-host.md` 3.10.6). A case of phase `declare` that states `message` makes the
file invalid.

### 7.2 The field `trace` of an expected diagnostic

| Field | Required | Compared with |
| --- | --- | --- |
| `trace` | No | The trace of the run failure (4): the same number of entries, and each expected entry with the actual one at the same position. An entry is an object with exactly the fields `span` (a span), `name` (a string) and `index` (a whole number of 0 or more, or null), each compared for equality |

1. `trace` in a diagnostic of a phase other than `run` makes the file invalid.
2. For a limit code, `trace` is not compared, even when the case gives it, as
   `span` is not (`03-host.md` 3.10.7 item 7).
3. A runner compares the trace in the checked run and in the run with the static
   rules alone: it does not depend on the check.

### 7.3 The field `rendered` of an expected failure

| Field of a failure | Holds | Compared with |
| --- | --- | --- |
| `rendered` | An object with one or more of `line`, `detail` and `json`, each a string | The three forms of 5 of the diagnostics of the failure, with the case's source text: for phase `parse` the first diagnostic only, for `check` the whole list, for `run` the one diagnostic. Each form given is compared whole, code point by code point |

1. `rendered` in a failure of phase `declare` makes the file invalid.
2. `rendered` of a run failure whose code is a limit code makes the file
   invalid: its span and trace are not fixed.
3. A case with `file` renders with the text of that file.

### 7.4 The case of the third kind: `render`

A case with the field `render` has no program and no environment. It gives
diagnostics and a source text, and expects their renderings.

| Field | Holds |
| --- | --- |
| `render` | An object with `diagnostics`, a list of one or more diagnostics, and optionally `source`, a string. A diagnostic is an object with `phase` and `code` (strings), `message` (a string), and optionally `class` (a string), `span` (a span), `path` (a path), `data` (a host value as `03-host.md` 3.10.3 reads one, opaque values included) and `trace` (a list of entries of 7.2) |

1. A case with `render` may have only `name`, `opaque`, `render`, `skip` and
   `expect`. `source`, `file`, `conform` and `render` count as one field for the
   header rule (`03-host.md` 3.10.2 item 4), and `render` may not stand in a
   header.
2. Its `expect` is an object with one or more of `line`, `detail` and `json`,
   each a string, compared whole with the forms of 5 of the diagnostics, with
   `source` as the source text, or with no source text when it is absent.
3. A span outside the source text, a `data` on a code other than `failed`, a
   `path` on a code other than `not-conforming`, or a `trace` on a phase other
   than `run`, makes the file invalid. `phase`, `code` and `class` are not
   tested further: the case tests the renderer, whatever the diagnostic.
4. The differences are named `line`, `detail` and `json`, in that order.

### 7.5 `max` and `failures` in a conform case

| Field | Holds |
| --- | --- |
| `max` in `conform` | A whole number of 1 or more. With it the case also runs `ConformAll` (6) with that maximum |
| `failures` in a conform result | A list in the order of 6.2, each an object with `path` and, optionally, `message`. Allowed only when `conform` has `max` |

1. With `max`, step 3 of F4 9.3 also runs `ConformAll`. Its list is compared
   with `failures`, when given: the same length, and each entry's `path` and
   `message`, when given, equal.
2. With `max`, whatever `failures` says, the runner also tests the agreement of
   6.1: the list is empty exactly when the value conforms, and its first entry
   has the path and the message of the function of F4 7. A disagreement is the
   difference `conforms`.
3. The differences of a conform case are named `outcome`, `conforms`, `path`,
   `message`, `steps` and `failures`, in that order.

### 7.6 When a file is invalid

These join the list of `03-host.md` 3.10.10.

1. `message` in a diagnostic of phase `declare`.
2. `trace` in a diagnostic of a phase other than `run`, or a trace entry that is
   not of the form of 7.2.
3. `rendered` that is not an object of 7.3, or one in a failure of phase
   `declare` or of a limit code.
4. A case with `render` that breaks 7.4.
5. `max` that is not a whole number of 1 or more; `failures` without `max`;
   `failures` that is not a list of objects of 7.5.

### 7.7 The JSON Schema of the additions

These are added to the schema of `03-host.md` 3.10.11: `trace` in
`$defs/diagnostic`, `rendered` in `$defs/failure`, `render` in both lists of
`propertyNames` of a case, `$defs/renderResult` as a branch of `$defs/expect`,
`max` in F4's `conform`, and `failures` in F4's `$defs/conformResult`.

```json
{
  "diagnostic": {"properties": {
    "trace": {"type": "array", "items": {"$ref": "#/$defs/traceEntry"}}}},
  "traceEntry": {
    "type": "object",
    "required": ["span", "name", "index"],
    "properties": {
      "span": {"$ref": "#/$defs/span"},
      "name": {"type": "string"},
      "index": {"oneOf": [{"type": "null"}, {"type": "integer", "minimum": 0}]}},
    "additionalProperties": false},
  "renderings": {
    "type": "object",
    "minProperties": 1,
    "properties": {
      "line": {"type": "string"},
      "detail": {"type": "string"},
      "json": {"type": "string"}},
    "additionalProperties": false},
  "failure": {"properties": {"rendered": {"$ref": "#/$defs/renderings"}}},
  "render": {
    "type": "object",
    "required": ["diagnostics"],
    "properties": {
      "source": {"type": "string"},
      "diagnostics": {
        "type": "array", "minItems": 1,
        "items": {
          "type": "object",
          "required": ["phase", "code", "message"],
          "properties": {
            "phase": {"enum": ["declare", "parse", "check", "run"]},
            "code": {"type": "string"},
            "message": {"type": "string"},
            "class": {"type": "string"},
            "span": {"$ref": "#/$defs/span"},
            "path": {"$ref": "#/$defs/diagnostic/properties/path"},
            "data": {"$ref": "#/$defs/hostValue"},
            "trace": {"type": "array", "items": {"$ref": "#/$defs/traceEntry"}}},
          "additionalProperties": false}}},
    "additionalProperties": false},
  "renderResult": {"$ref": "#/$defs/renderings"},
  "conform": {"properties": {"max": {"type": "integer", "minimum": 1}}},
  "conformResult": {"properties": {
    "failures": {
      "type": "array",
      "items": {
        "type": "object",
        "required": ["path"],
        "properties": {
          "path": {"$ref": "#/$defs/diagnostic/properties/path"},
          "message": {"type": "string"}},
        "additionalProperties": false}}}}
}
```

### 7.8 The runner

1. A runner of an implementation that offers F8 runs `suite/errors/`,
   `suite/messages-parse.json` and `suite/messages-check.json`, each case as
   `03-host.md` 3.10.8 item 5 says. A case of phase `parse` is run once: it
   never reaches the check. A case that states a `check` message, `type` or
   `sites`, or a `check` code that is not static, is run once, checked.
2. **Static diagnostics and their messages.** A `check` message of a static code
   is the same whether the program is checked or compiled with the static rules
   alone, so a case that states one is still run both ways when its codes are
   all static.
3. A render case is run once: it has no program.
4. **Earlier cases.** A runner of an implementation that offers F8 does not run
   the files named `until-f08-` (9); every other runner runs them.

## 8. The `outcome` command

The line of the `outcome` command (`suite/README.md`) changes so, for an
implementation that offers F8.

| Case | Line |
| --- | --- |
| A failure of phase `parse` | The first diagnostic has `code`, `span` and `message` |
| A failure of phase `check` | Each diagnostic has `code`, `span` and `message` |
| A failure of phase `run` | The diagnostic has `code`, `span`, `class`, `message` and, after them, `trace`, a list of `{"span", "name", "index"}`. For a failure of class `limit` there is no `trace`, as there is no span |
| A render case | `name`, then `line`, `detail` and `json`, the three forms |
| A conform case with `max` | After the fields of F4 9.5, `failures`: a list of `{"path", "message"}` from `ConformAll` |

The fuzzer compares `message` and `trace` among the implementations that write
them.

## 9. Earlier cases whose outcome this package changes

1. 317 earlier cases pin, for a text that this package gives a targeted code,
   the general code that Core reports: `unexpected-token`,
   `unexpected-character` or `reserved-word`, at the span this chapter keeps. By
   `03-host.md` 3.11 items 4 to 6 they are moved, unchanged, out of the 14 files
   that held them, each into a file `until-f08-` and the name of that file, in
   the same directory and with the same header. Nothing else in those files
   changes.

| Directory | Files `until-f08-` | Cases |
| --- | --- | ---: |
| `suite/core-run/` | `until-f08-parse-follow-on.json` (97), `until-f08-parse-errors.json` (85), `until-f08-parse-reserved.json` (46), `until-f08-lex-characters.json` (21), `until-f08-parse-first-diagnostic.json` (7) | 256 |
| `suite/conversion/` | `until-f08-conversion-syntax.json` (29), `until-f08-conversion-examples.json` (2) | 31 |
| `suite/strings/` | `until-f08-strings-core-texts.json` (11), `until-f08-strings-raw.json` (9), `until-f08-strings-errors.json` (4), `until-f08-strings-examples.json` (2) | 26 |
| `suite/templates/` | `until-f08-errors.json` (2), `until-f08-templates-strings.json` (1) | 3 |
| `suite/queries/` | `until-f08-queries-syntax.json` (1) | 1 |

2. A runner of an implementation that offers F8 does not run these files. Every
   other runner runs them as before, so an implementation that has not reached
   F8 keeps every case that pins what it does.
3. `suite/errors/errors-core-texts.json` holds each of the 317 texts, with the
   same source text and the same other fields, and the outcome of this chapter:
   the targeted code, the same span, and the message. Its cases are named after
   the file and the case they stand for: `core-run/parse-errors: rule 11: no
   else at the end of input`.
4. The files `until-f08-` are deleted when all three implementations offer F8.
5. No case of `check` or `run` changes. No earlier case states the message of a
   `parse` or a `check` diagnostic, so every earlier case outside these files
   keeps its outcome. Every span that 2.8 changes is unpinned: no earlier case
   of those texts states a span.
6. No case that this package changes stands in a file `until-f04-` or
   `until-f07-`.

## 10. Examples

| # | Text | Code | Span | Message |
| --- | --- | --- | --- | --- |
| 1 | `1 === 1` | `strict-equality` | `[1, 3, 1, 6]` | === is not an operator; write ==, which converts nothing |
| 2 | `if (c) { a }` | `missing-else` | `[1, 13, 1, 13]` | an if needs an else branch, found end of input |
| 3 | `if c { a } else { b }` | `if-form` | `[1, 4, 1, 5]` | the test of an if is written in parentheses and each branch in braces, found c |
| 4 | `if(c, a, b)` | `if-form` | `[1, 5, 1, 6]` | if is not a function; write c ? a : b |
| 5 | `a ?? b not in c` | `needs-parentheses` | `[1, 8, 1, 14]` | put one side of not in in parentheses: ?? does not mix with \|\|, &&, ==, !=, <, <=, >, >=, in, not in, as or is without them |
| 6 | `try e catch d` | `try-form` | `[1, 5, 1, 6]` | the body of a try and its handler are written in braces, found e |
| 7 | `try { 1 } catch (e: T) { 2 }` | `catch-name` | `[1, 19, 1, 20]` | the parentheses after catch hold one name, found :; leave them out when the error is not used |
| 8 | `switch (x) { }` | `switch-statement` | `[1, 1, 1, 7]` | there is no switch or case; write a match |
| 9 | `const f = x => { return x }; f(1)` | `return-statement` | `[1, 18, 1, 24]` | there is no return: the value of a block is its last expression |
| 10 | `{for}` | `loop-statement` | `[1, 2, 1, 5]` | there are no loops or comprehensions in 3.0; use map, filter or reduce |
| 11 | `f(a = 1)` | `assignment` | `[1, 5, 1, 6]` | a binding cannot be reassigned; bind a new name with const, or compare with == |
| 12 | `[1].map(x => { x + 1 })` | `block-without-const` | `[1, 14, 1, 15]` | a lambda body in braces is a map; write the expression without braces, or start a block with a const line |
| 13 | `(x: int) => x` | `type-annotation` | `[1, 3, 1, 4]` | types are inferred; remove the annotation |
| 14 | `typeof x` | `typeof-operator` | `[1, 1, 1, 7]` | typeof is not an operator; write type(x) for the kind, or x is T for a test |
| 15 | `a?[i]` | `optional-index` | `[1, 2, 1, 3]` | an optional index is written ?.[ with a dot |
| 16 | `xs[]` | `star-form` | `[1, 4, 1, 5]` | a projection is written xs[*].f |
| 17 | `$"x"` | `string-prefix` | `[1, 1, 1, 2]` | $ is not a string prefix; only a lower-case r directly before the quote makes a raw string |
| 18 | `r "a"` | `string-prefix` | `[1, 1, 1, 2]` | the prefix r stands directly before the quote, with nothing between |
| 19 | `` String.raw`x` `` | `tagged-template` | `[1, 8, 1, 11]` | there are no tagged templates; raw text is written r"..." or as a raw text block |
| 20 | `r'''a'''` | `text-block-quote` | `[1, 1, 1, 2]` | a text block is written with three double quotes, not ''' |
| 21 | `# comment` | `hash-comment` | `[1, 1, 1, 2]` | a comment is written // to the end of the line, or /* */ |
| 22 | `{max-retries: 3}` | `unquoted-key` | `[1, 2, 1, 13]` | a key that is not a word is quoted: write "max-retries" |
| 23 | `$x` | `context-symbol` | `[1, 1, 1, 2]` | $ names nothing; write the name of the variable |
| 24 | `r"a"b"` | `unexpected-token` | `[1, 5, 1, 6]` | unexpected b |
| 25 | `const {for} = {}; 1` | `reserved-word` | `[1, 8, 1, 11]` | for is a reserved word and cannot be a name |
