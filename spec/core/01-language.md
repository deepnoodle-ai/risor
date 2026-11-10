# 01. The Core language

This chapter is normative. It defines the language of Core run: its text, its
grammar, its syntax tree, its values and the meaning of each form, and the
`parse` codes and `run` codes that the language itself raises. The order of
evaluation, the cost of each form and the limits are in `02-evaluation.md`. The
environment, values from the host, the shape of a diagnostic, positions and case
files are in `03-host.md`. Functions and methods are in `04-library.md`. Types
and the checker are in `05-types.md`.

## 1. Conventions

### 1.1 Terms

| Term | Meaning |
| --- | --- |
| Core | Core run: the forms of this chapter. A later package adds forms. It never changes a rule of this chapter |
| Follow-on form | A form that a later package adds. In Core its text is a `parse` error (14.3) |
| Code point | A Unicode scalar value: U+0000 to U+D7FF and U+E000 to U+10FFFF |
| Token | A unit of the text that the grammar reads (2.3) |
| Word | A token of the form of 2.4 |
| Name | A word that is not reserved and is not `_` (2.4) |
| Whole number | A number with no fractional part whose magnitude is at most 2^53 − 1, which is 9007199254740991 (6.2) |
| Falsy, truthy | 6.5 |
| Fails with `c` | The run ends with a failure whose code is `c`, unless a `try` catches it (11) |
| `const`-bound lambda | A lambda that is the value of a `const` line whose pattern is a name (8.1) |

### 1.2 Notation

The grammar is written in EBNF. `=` defines a production and `.` ends it. `|`
separates alternatives. `[ x ]` is zero or one `x`. `{ x }` is zero or more `x`.
`( )` groups. A string in quotes is a token, written as it appears in the text.
A word that starts with an upper-case letter is a production or a lexical class.
`(* *)` is a comment.

## 2. Lexical structure

### 2.1 Source text

1. The source text is a sequence of code points. How a host passes it is in
   `03-host.md`.
2. A U+FEFF at the start of the text is ignored. A U+FEFF anywhere else is an
   unexpected character (2.10).
3. A **line end** is U+000A, U+000D followed by U+000A, or U+000D alone. Line
   ends matter in four places: they end a line comment (2.2), they end a
   one-line string with an error (2.6), a `\` before one is an error in a
   template (2.8), and a raw U+000D in a template reads as U+000A (2.8). A
   position counts lines by `03-host.md`, at U+000A only.

### 2.2 White space and comments

1. **White space** is U+0020 (space), U+0009 (tab), U+000A and U+000D. It may
   stand between any two tokens. It has no meaning other than to separate
   tokens.
2. A **line comment** starts with `//` and runs to the next line end or the end
   of the text. The line end is not part of the comment.
3. A **block comment** starts with `/*` and ends with the first `*/` after it.
   Block comments do not nest: in `/* a /* b */`, the comment ends after `b`.
4. A `/*` with no `*/` after it is `unterminated-comment` (14.2).
5. A comment counts as white space. The characters `//` and `/*` inside a string
   or the text of a template do not start a comment.

### 2.3 Tokens

1. The text is read from the start into tokens, white space and comments.
2. At each place, the lexer reads the **longest** token that starts there. `>=`
   is always one token, and so is `...`.
3. These are the tokens.

| Class | Tokens |
| --- | --- |
| Words | 2.4 |
| Number literals | 2.5 |
| String literals | `"..."` and `'...'` (2.6) |
| Template literals | `` `...` ``, with its interpolations (2.8) |
| Brackets | `(` `)` `[` `]` `{` `}` `?.[` |
| Separators | `,` `;` `:` `.` `?.` `...` `=>` `=` `?` `\|` |
| Operators | `!` `-` `+` `*` `/` `%` `**` `==` `!=` `<` `<=` `>` `>=` `&&` `\|\|` `??` |
| Tokens that no rule accepts | `===` `!==` `+=` `-=` `*=` `/=` `%=` `**=` `??=` `\|\|=` `&&=` `++` `\|>`, the block token and the prefixed literal (rules 7 and 9) |

4. A token of the last row is read so that its text is one token. Wherever it
   stands it is `unexpected-token`. Its span is the whole token, but for the
   block token and the prefixed literal (rules 7 and 9).
5. `--` is not a token. `--x` is `-(-x)`.
6. `?.` is one token wherever `?` is directly followed by `.`. `?.[` is one
   token wherever `?.` is directly followed by `[`. `?. [`, with white space, is
   the token `?.` and then `[`.
7. **The block token.** Where a token starts with `"""`, or with `'''`, the
   token runs from there to the end of the next `"""`, or the next `'''`, after
   it, or to the end of input if there is none. Nothing in it is read as an
   escape, a line end or an interpolation. Its span, where a diagnostic names
   it, is its first three code points. So `""` is the empty string only when no
   `"` follows it directly, and `''` likewise.
8. The **end of input** is the place after the last code point of the text.
   Where a diagnostic names it, its span is the empty span at that place.
9. **The prefixed literal.** Where a `Word` that is not reserved is followed
   directly, with no white space, by `"`, `'` or `` ` ``, the word and what
   follows form one token. It runs to the end of the next code point equal to
   that quote, or to the end of the line or of the input if there is none first.
   For `` ` `` it runs to the next `` ` `` and may cross lines. Nothing in it is
   read as an escape or an interpolation. Its span, where a diagnostic names it,
   is the word.

### 2.4 Words, names and reserved words

1. A **word** is a sequence that matches `[A-Za-z_][A-Za-z0-9_]*`. Only ASCII
   letters and digits form words.
2. These 26 words are **reserved**.

| Group | Words |
| --- | --- |
| Words of the grammar | `true` `false` `null` `const` `if` `else` `in` `not` `try` `catch` `match` |
| Reserved for a follow-on form | `as` `is` |
| Reserved for a later version | `nil` `let` `var` `for` `while` `do` `break` `continue` `return` `function` `throw` `finally` `import` |

3. No other word is reserved. In particular `of`, `r`, `type`, `switch`, `case`,
   `export`, `typeof`, `instanceof`, `satisfies`, `interface`, `enum`,
   `undefined`, `it` and the names of the library are not reserved.
4. A **name** is a word that is not reserved and is not `_`. The grammar writes
   it `Name`.
5. `_` is a word. It is not a name. It may stand only where the grammar writes
   `"_"`: as a pattern that binds nothing (8.4), as the one parameter of a
   lambda written without parentheses (8.1) and as the default arm of a `match`
   (9.3). It cannot be read.
6. Any word, reserved or not and `_` included, may stand after `.` and `?.`, as
   a bare key of a map literal (10.2) and as a key of a field of a map pattern
   (8.4). There it is the text of a key. The grammar writes it `Word`.
7. Where the grammar needs a `Name`, or an operand, and a reserved word stands
   there, the code is `reserved-word` by the rule of 14.2.

### 2.5 Numbers

1. A **number token** starts at a digit, or at a `.` that is directly followed
   by a digit where a token starts. It continues over each following code point
   that is:
   - an ASCII letter, a digit or `_`;
   - a `+` or `-` directly after an `e` or `E`, when the token does not start
     with `0x` or `0X`;
   - a `.` that is followed by a digit, or by a code point that is not an ASCII
     letter and not `_`, or by the end of input.
2. So `5.a` is the number `5`, the token `.` and the word `a`. `5.` and `5.)`
   hold the number token `5.`. `0b1`, `1__0` and `3in` are each one number
   token.
3. A number token must match `Number` below. Otherwise it is `bad-number`, with
   the span of the whole token.

```ebnf
Number   = Decimal | Hex .
Decimal  = Int [ "." Digits ] [ ( "e" | "E" ) [ "+" | "-" ] Digits ] .
Int      = "0" | NonZero { [ "_" ] Digit } .
Digits   = Digit { [ "_" ] Digit } .
Hex      = "0x" HexDigit { [ "_" ] HexDigit } .
```

| Class | Code points |
| --- | --- |
| `Digit` | `0` to `9` |
| `NonZero` | `1` to `9` |
| `HexDigit` | `0` to `9`, `a` to `f`, `A` to `F` |

4. So a single `_` may stand between two digits and nowhere else. `.5`, `5.`,
   `010`, `00`, `1__0`, `1_`, `0x`, `0X1F`, `0b1` and `0o7` are `bad-number`.
5. **Value.** Remove every `_`.
   - A `Hex` token is the integer its digits name in base 16.
   - A `Decimal` token with no `.` and no exponent is the integer its digits
     name.
   - Any other `Decimal` token is the binary64 value nearest to the decimal
     number it names, ties to even. A value too small to represent is zero.
6. **Range.** These are `literal-range`, with the span of the token:
   - a `Hex` token, or a `Decimal` token with no `.` and no exponent, whose
     integer is not a whole number: `9007199254740992`, `0x20000000000000`;
   - any other `Decimal` token whose nearest binary64 value is not finite:
     `1e400`.
7. `1e20`, `9007199254740993.0` and `1e-400` are valid. Their values are `1e20`,
   `9007199254740992` and zero.
8. A number token is never negative. `-5` is the operator `-` applied to `5`.

### 2.6 One-line strings

1. A **one-line string** is `"` or `'`, then content, then the same quote. The
   two quotes mean the same.
2. The content is a sequence of code points and escapes (2.7). Any code point
   may stand in it except the opening quote, `\`, U+000A and U+000D. The other
   quote may stand in it as it is.
3. A line end, or the end of input, before the closing quote is
   `unterminated-string`. Its span runs from the opening quote to the place
   where the literal stops: the start of the line end, or the end of input. A
   `\` directly before that line end is part of the literal, and the code is
   still `unterminated-string`.
4. The value is the content, with each escape replaced by the code point it
   names.

### 2.7 Escapes

1. These are the escapes of a one-line string.

| Escape | Code point |
| --- | --- |
| `\"` | U+0022 |
| `\'` | U+0027 |
| `\\` | U+005C |
| `\/` | U+002F |
| `\b` | U+0008 |
| `\f` | U+000C |
| `\n` | U+000A |
| `\r` | U+000D |
| `\t` | U+0009 |
| `\uXXXX` | The code point with that number, four `HexDigit`s |
| `\u{X...}` | The code point with that number, one to six `HexDigit`s between the braces |

2. A template has the same escapes and two more: `` \` `` for U+0060 and `\$`
   for U+0024 (2.8).
3. **Surrogates.** A `\uXXXX` whose number is in U+D800 to U+DBFF must be
   followed directly by a `\uXXXX` whose number is in U+DC00 to U+DFFF. The two
   are one escape, and name the one code point the pair encodes.
4. Each of these is `bad-escape`:

| Text | Span |
| --- | --- |
| `\` followed by a code point that no row of 1 or 2 begins with: `\x41`, `\0`, `\v`, `` \` `` in a one-line string | The `\` and that code point |
| `\u` not followed by four `HexDigit`s or by `{` | The `\u` and the `HexDigit`s that follow it |
| `\u{` with no `HexDigit`, more than six, or no `}` directly after them | From the `\` to the last `HexDigit` read, or to the `{` if there is none |
| `\u{X...}` whose number is above U+10FFFF or in U+D800 to U+DFFF | The whole escape |
| A `\uXXXX` in U+D800 to U+DBFF not followed directly by a `\uXXXX` in U+DC00 to U+DFFF | The first `\uXXXX` |
| A `\uXXXX` in U+DC00 to U+DFFF that does not complete a pair | That `\uXXXX` |

### 2.8 Templates

1. A **template** is `` ` ``, then text parts and interpolations, then `` ` ``.
2. A text part is a sequence of code points and escapes (2.7). Any code point
   may stand in it except `` ` ``, `\` and a `$` directly followed by `{`. It
   may hold line ends.
3. In the value of a text part, a raw U+000D followed by U+000A, and a raw
   U+000D alone, each read as one U+000A. An escape `\r` still gives U+000D.
4. A `\` directly before a line end is `line-continuation`, with the span of the
   `\`.
5. An **interpolation** is `${`, then tokens, white space and comments, then the
   `}` that matches the `${`. The tokens must form one `Expr` (3.1). The
   matching `}` is found by counting the `{` and `}` tokens between: the
   interpolation ends at the first `}` that is not matched by an earlier `{`
   within it. A `${` of a nested template, and that template's own `}`, are
   inside that template and are not counted.
6. An interpolation may hold any expression, a template included.
7. **Unterminated.** If the end of input comes inside a template, in a text part
   or in an interpolation, the code is `unterminated-string`. Its span runs from
   the opening `` ` `` of the innermost template that is still open to the end
   of input.
8. A template with no interpolation is a `String` node. A template with at least
   one is a `Template` node (4.2).

### 2.9 What the lexer reads where

1. A `Word` that is not reserved and is directly followed by `"`, `'` or `` ` ``
   is the prefixed literal of 2.3 rule 9. A reserved word directly followed by a
   quote is the word and then the literal: `x in"abc"` is valid.
2. A `$` is part of a token only inside a template. Elsewhere it is an
   unexpected character.

### 2.10 Unexpected characters

1. Outside a string, a template's text and a comment, a code point that begins
   no token and is not white space is `unexpected-character`, with the span of
   that code point.
2. These begin no token: every code point that is not ASCII (a non-breaking
   space and a letter with an accent among them); a U+FEFF that is not at the
   start of the text; every control character other than the four of white
   space; and `#`, `$`, `@`, `\`, `^`, `~`, and a `&` that is not followed by
   `&`.

### 2.11 Order of lexical diagnostics

1. The whole text is divided into tokens before it is parsed.
2. If that finds a lexical diagnostic, the first diagnostic of the parse is the
   first lexical diagnostic the lexer meets, reading from the start. An
   unterminated literal or comment is met where it stops. So in `"a\q` and then
   a line end, `bad-escape` is met before `unterminated-string`.
3. Lexical diagnostics are `unexpected-character`, `unterminated-string`,
   `unterminated-comment`, `bad-escape`, `line-continuation`, `bad-number` and
   `literal-range`.
4. If there is no lexical diagnostic, the first diagnostic is the first one the
   parser meets (14.1).

## 3. Grammar

### 3.1 Productions

`Number`, `String`, `Template`, `Word` and `Name` are tokens (section 2).
`String` is a one-line string. `Template` is a template, with or without
interpolations.

```ebnf
Program = { Const } Expr [ ";" ] .
Const   = "const" Pattern "=" ( Lambda | Expr ) ";" .
Block   = "{" Const { Const } Expr "}" .
Branch  = "{" { Const } Expr "}" .

Expr    = If | Try | Binary [ "?" Expr ":" Expr ] .          (* right-assoc *)
If      = "if" "(" Expr ")" Branch
          { "else" "if" "(" Expr ")" Branch } "else" Branch .
Try     = "try" Branch "catch" [ "(" Name ")" ] Branch .

Binary  = Or | Coal .
Coal    = Add "??" Add { "??" Add } .
Or      = And   { "||" And } .
And     = Equal { "&&" Equal } .
Equal   = Rel   { ( "==" | "!=" ) Rel } .
Rel     = Add   { ( "<" | "<=" | ">" | ">=" | "in" | "not" "in" ) Add } .
Add     = Mul   { ( "+" | "-" ) Mul } .
Mul     = Unary { ( "*" | "/" | "%" ) Unary } .
Unary   = ( "!" | "-" ) Unary | Power .
Power   = Chain [ "**" Unary ] .                             (* right-assoc *)
Chain   = Primary { "." Word [ Args ] | "?." Word [ Args ]
                  | "[" Expr "]" | "?.[" Expr "]" } .
Primary = Number | String | Template | "true" | "false" | "null"
        | Name [ Args ] | "(" Expr ")" | List | Map | Block | Match .

Args    = "(" [ Arg { "," Arg } [ "," ] ] ")" .
Arg     = Lambda | Expr .
Lambda  = ( Name | "_" | "(" [ Param { "," Param } [ "," ] ] ")" ) "=>" Expr .
Param   = Pattern [ "=" Expr ] .

Pattern = Name | "_" | MapPat | ListPat .
MapPat  = "{" [ ( Field { "," Field } [ "," Rest ] | Rest ) [ "," ] ] "}" .
Field   = ( Name | ( Word | String ) ":" Name ) [ "=" Expr ] .
ListPat = "[" [ ( Elem { "," Elem } [ "," Rest ] | Rest ) [ "," ] ] "]" .
Elem    = ( Name | "_" ) [ "=" Expr ] .
Rest    = "..." Name .

List    = "[" [ Item { "," Item } [ "," ] ] "]" .
Item    = "..." Expr | Expr .
Map     = "{" [ Entry { "," Entry } [ "," ] ] "}" .
Entry   = "..." Expr | Key ":" Expr | Name .
Key     = Word | String | "[" Expr "]" .

Match   = "match" Expr "{" Case { "," Case } [ "," ] "}" .
Case    = ( Lit { "|" Lit } | "_" ) "=>" Expr .
Lit     = [ "-" ] Number | String | "true" | "false" | "null" .
```

### 3.2 How the parser chooses

Where the grammar allows more than one reading of the next tokens, these rules
choose. Each is part of the grammar.

| # | Place | Rule |
| --- | --- | --- |
| 1 | A `{` where a `Primary` may stand | It starts a `Block` exactly when the next token is `const` and the token after that is not `:`. Otherwise it starts a `Map` |
| 2 | A `{` directly after `if (` `Expr` `)`, after `else`, after `try`, after `catch` and after `catch (` `Name` `)` | It starts a `Branch`, whatever follows it. It is never a `Map`. A map as a branch's value is written inside the branch: `{ {id: 1} }` |
| 3 | `else` followed by `if` | The `if` continues the same `If`. `else` followed by `{` is the last branch |
| 4 | `Binary` | Read an `Add`. If the next token is `??`, it is a `Coal`. Otherwise the `Add` is the first operand of an `Or` |
| 5 | Where an `Arg`, or the value of a `Const`, may stand | A `Name` or `_` directly followed by `=>` begins a `Lambda`. `_ => e` is the same as `(_) => e`. A `(` begins a `Lambda` when the token after its matching `)` is `=>`. Otherwise the place holds an `Expr` |
| 6 | The matching `)` of rule 5 | Count `(`, `[`, `?.[` and `{` as openers and `)`, `]` and `}` as closers, over the tokens after the `(`. The matching token is the first closer that brings the count below zero. If it is not a `)`, or there is none, the `(` does not begin a `Lambda` |
| 7 | An `Entry` that starts with a `Word` | If the token after the word is `:`, the word is a `Key`. Otherwise the entry is the shorthand `Name` |
| 8 | A `Field` that starts with a `Word` | The same rule |
| 9 | `.` or `?.` followed by a `Word` and then `(` | A method call. Without the `(`, a field read |
| 10 | A `Name` followed by `(` where a `Primary` may stand | A call. Without the `(`, a name |
| 11 | `Lambda` body | The longest `Expr`. It ends at the first token that cannot continue that `Expr` |

So `{const: 1}` is a map, `{ const a = 1; a }` is a block, `x => {id: x.id}` is
a lambda whose body is a map, and `f((a, b) => a)` passes a lambda.

### 3.3 Precedence and associativity

From the loosest to the tightest.

| Level | Operators | Associativity | Operands |
| --- | --- | --- | --- |
| 1 | `c ? a : b`, `if`, `try` | `? :` is right: `a ? b : c ? d : e` is `a ? b : (c ? d : e)` | `a` is any `Expr`. `if` and `try` are whole expressions, never operands (3.4) |
| 2 | `??` | left | Each operand is an `Add`. `??` does not mix with levels 3 to 6 (3.4) |
| 3 | `\|\|` | left | |
| 4 | `&&` | left | |
| 5 | `==` `!=` | left | |
| 6 | `<` `<=` `>` `>=` `in` `not in` | left | |
| 7 | `+` `-` | left | |
| 8 | `*` `/` `%` | left | |
| 9 | prefix `!` `-` | right: `!!a`, `- -a` | |
| 10 | `**` | right: `a ** b ** c` is `a ** (b ** c)` | The right operand is a `Unary`: `2 ** -1` is valid |
| 11 | `.name` `?.name` `[e]` `?.[e]` `.m(...)` `?.m(...)` | left | A chain (7.4) |

`&&` binds tighter than `||`: `a || b && c` is `a || (b && c)`.

### 3.4 Rules the productions do not show

Each rule is part of the grammar. Text that breaks one is `unexpected-token` in
Core, at the span in the third column.

| # | Rule | Span of the error |
| --- | --- | --- |
| 1 | `-` or `!` may not apply directly to a `Power` that has `**`. `-2 ** 2` and `!a ** b` are errors. `(-2) ** 2` and `-(2 ** 2)` are valid | The `-` or the `!` |
| 2 | The left operand of `in` or `not in` may not be a `!` expression written without parentheses. `!a in b` is an error. `a not in b` and `(!a) in b` are valid | The `!` |
| 3 | `??` does not mix with `\|\|`, `&&`, `==`, `!=`, `<`, `<=`, `>`, `>=`, `in` or `not in` without parentheses: `a ?? b > c`, `a > b ?? c`, `a ?? b \|\| c` are errors. The productions give this | The operator token that cannot follow |
| 4 | An `if` or a `try` is a whole expression, not an operand. After its last branch or its handler, only a token that may follow an `Expr` may stand. `if (c) { 1 } else { 2 } + 3` is an error, and so is a `.`, `[`, `?`, `?.` or `?.[` there. Write `(if (c) { 1 } else { 2 }) + 3`. The productions give this | The token that cannot follow |
| 5 | An `if` or a `try` may not stand where an operand is needed: `1 + if (c) { 1 } else { 2 }` | The `if` or the `try` |
| 6 | A `match` subject may not start with `{`, `if` or `try`. Write it in parentheses. `match {` with no subject is this error | That first token of the subject |
| 7 | A branch is **written like a map** when its expression is followed by `:` or `,`, or when the branch starts with `...`. That is an error | The `{` of the branch |
| 8 | A branch with nothing between its braces, `{}`, is an error | The `{` of the branch |
| 9 | A `Lambda` may be the value of a `Const` only when its pattern is a `Name`: `const {a} = x => x;` is an error | The `=>` |
| 10 | `try` followed by the token `(` is an error: `try(e, d)` | The `try` |
| 11 | An `else` is required after the last branch of an `if`. The productions give this | The token that stands where `else` should, or the end of input |
| 12 | A `catch` is required after the body of a `try`. Its parentheses hold one `Name`. With no name, the parentheses are left out. The productions give this | The token that does not fit |

### 3.5 Other text that does not parse

1. Text that does not fit section 3.1 and that no rule of 3.4 or 14.3 covers is
   `unexpected-token`. Its span is the first token at which the tokens read so
   far, together with the choices of 3.2, cannot be continued to a `Program`. At
   the end of input, the span is the empty span there (2.3).
2. A reserved word at a place where the grammar needs a `Name` or an operand is
   `reserved-word` by 14.2, not `unexpected-token`.

## 4. The syntax tree

### 4.1 Node kinds

The parser builds a tree of these nodes. The numbers and names are fixed. A
later package adds nodes with new numbers and never renumbers one.

| # | Node | Fields | Written |
| --- | --- | --- | --- |
| 1 | `Program` | `consts` (list of `Const`), `result` | the whole text |
| 2 | `Const` | `pattern`, `value` | `const p = e;` |
| 3 | `Block` | `consts` (list of one or more `Const`), `result` | `{ const ...; e }`, and a branch that has `const` lines |
| 4 | `Ident` | `name` | `x` |
| 5 | `Number` | `value` | `42`, `1.5e3`, `0x1F` |
| 6 | `String` | `value`, `form` | `"a"`, `'a'`, and a template with no interpolation |
| 7 | `Template` | `parts` (each a text or an expression), `form` | `` `a ${e}` `` |
| 8 | `Bool` | `value` | `true`, `false` |
| 9 | `Null` | | `null` |
| 10 | `List` | `items` (each an expression or a `Spread`) | `[a, ...b]` |
| 11 | `Map` | `entries` (each an `Entry` or a `Spread`) | `{a: 1, ...m}` |
| 12 | `Entry` | `key` (a word, a string or an expression), `value`, `shorthand` | `a: 1`, `"a-b": 1`, `[k]: 1`, `a` |
| 13 | `Spread` | `value` | `...e` |
| 14 | `Unary` | `op` (`!` or `-`), `x` | `!a`, `-a` |
| 15 | `Binary` | `op`, `x`, `y` | `\|\| && == != < <= > >= in not-in ?? + - * / % **` |
| 16 | `Cond` | `form` (`?:` or `if`), `test`, `then`, `else` | `c ? a : b`, `if (c) { a } else { b }` |
| 17 | `Member` | `x`, `name`, `optional` | `x.f`, `x?.f` |
| 18 | `Index` | `x`, `index`, `optional` | `x[i]`, `x?.[i]` |
| 19 | `Call` | `namespace` (or none), `name`, `args` | `f(a)`, `ns.f(a)` |
| 20 | `MethodCall` | `x`, `name`, `args`, `optional` | `x.m(a)`, `x?.m(a)` |
| 21 | `OptChain` | `x` | wraps a chain that has a `?.` or `?.[` step (4.2) |
| 22 | `Lambda` | `params` (list of `Param`), `body` | `(x, i) => e` |
| 23 | `Param` | `pattern`, `default` (or none) | `x`, `{a, b}`, `n = 2` |
| 24 | `Match` | `subject`, `arms` (list of `Arm`) | `match x { ... }` |
| 25 | `Arm` | `literals` (list), or default; `result` | `"a" \| "b" => e`, `_ => e` |
| 26 | `Try` | `body`, `name` (or none), `handler` | `try { e } catch (err) { d }`, `try { e } catch { d }` |
| 27 | `NamePat` | `name`, or discard | `x`, `_` |
| 28 | `MapPat` | `fields` (each with `key`, `name`, `default` or none), `rest` (or none) | `{a, b: x, c = 0, ...r}` |
| 29 | `ListPat` | `elems` (each a `NamePat` with `default` or none), `rest` (or none) | `[a, _, c = 0, ...r]` |
| 30 | `Bad` | | text that did not parse |

1. Core has 30 node kinds, numbered 1 to 30.
2. Numbers 31 and 32 are not in Core. Number 31 is `As` (`x as T`, `x is T`) and
   number 32 is `Each` (the projection `xs[*]`). Both belong to follow-on
   packages.
3. In Core, `Match` always has a subject, and every `Arm` has literals or is the
   default. An `Arm` with a test is not in Core.
4. In Core, the `form` of a `String` is `"`, `'` or `` ` ``, and the `form` of a
   `Template` is `` ` ``.
5. `Bad` stands for text after a `parse` diagnostic. A tree that holds `Bad` is
   never checked or run.

### 4.2 From text to nodes

| Text | Node |
| --- | --- |
| A `Program` with no `const` line | `Program` with empty `consts` |
| A `Block` written with braces | `Block` |
| A branch of an `if`, or the body or handler of a `try`, with `const` lines | `Block` |
| The same with no `const` line | Its expression. The braces have no node |
| `(e)` | The node of `e`. Parentheses have no node |
| `if (c) { a } else { b }` | `Cond` with `form` `if` |
| `if (c) { a } else if (d) { b } else { e }` | `Cond` with `form` `if`, whose `else` is a `Cond` with `form` `if` |
| `c ? a : b` | `Cond` with `form` `?:` |
| `a not in b` | `Binary` with `op` `not-in` |
| A `String` token | `String` |
| A `Template` token with no interpolation | `String`, with `form` `` ` `` |
| A `Template` token with an interpolation | `Template`. Its parts are its text parts and its interpolations, in order |
| `-5` | `Unary` `-` over `Number` `5`. In a `Lit` of a `match`, the arm holds the number `-5` |
| `f(a)` | `Call` with no namespace |
| `ns.f(a)` | `MethodCall` from the parser. When `ns` names a namespace of the environment (`03-host.md`), name resolution makes it a `Call` with that namespace |
| `x.f`, `x.m(a)`, `x[i]` | `Member`, `MethodCall`, `Index` with `optional` false |
| `x?.f`, `x?.m(a)`, `x?.[i]` | The same nodes with `optional` true |
| A chain with at least one optional step | The outermost node of the chain is wrapped in one `OptChain`. The chain is the `Chain` production: it ends at a closing parenthesis around it and at any operator (7.4) |
| `{a}` in a map literal | `Entry` with `key` the word `a`, `value` an `Ident` `a`, `shorthand` true |
| `{a}` in a pattern | A field with `key` `a` and `name` `a` |
| `{"a-b": x}` in a pattern | A field with `key` `"a-b"` and `name` `x` |

The `form` fields are for a formatter and for messages. Evaluation never reads
them: two nodes that differ only in `form` have the same meaning.

### 4.3 Spans

A span is a start and an end position in the source text. `03-host.md` defines
positions. The span of a token runs from its first code point to the end of its
last. The span of a node runs from the start of its first token to the end of
its last token, where its tokens are these:

| Node | First token | Last token |
| --- | --- | --- |
| `Program` | the first token of the text | the last token, the final `;` if there is one |
| `Const` | `const` | `;` |
| `Block` | `{` | `}` |
| `Ident`, `Number`, `String`, `Template`, `Bool`, `Null` | the token | the token. A literal's span holds its delimiters |
| `List`, `Map`, `MapPat`, `ListPat` | `[` or `{` | `]` or `}` |
| `Entry` | the key, or the `[` of a computed key | the end of the value. A shorthand entry: the name |
| `Spread` | `...` | the end of its value |
| `Unary` | the operator | the end of its operand |
| `Binary` | the start of `x` | the end of `y` |
| `Cond`, form `?:` | the start of `test` | the end of `else` |
| `Cond`, form `if` | its `if` | the `}` of the last branch |
| `Member`, `Index`, `MethodCall` | the start of `x` | the name, the `]`, the `)` |
| `Call` | the namespace, or the name | the `)` |
| `OptChain` | the start of the chain | the end of the chain |
| `Lambda` | the parameter name or `(` | the end of the body |
| `Param` | the start of the pattern | the end of the default, or of the pattern |
| `Match` | `match` | `}` |
| `Arm` | the first literal, or `_` | the end of the result |
| `Try` | `try` | the `}` of the handler |
| `NamePat` | the name or `_` | the same |
| A field of a `MapPat` | its key, or its name | the end of its default, or its name |
| An element of a `ListPat` | its name or `_` | the end of its default, or its name or `_` |

1. Parentheses around an expression are not part of that expression's node. They
   are part of the node that holds it: in `(a + b) * c`, the `Binary` `+` spans
   `a + b`, and the `Binary` `*` spans from `(` to `c`.
2. The span of `Bad` is not specified.
3. The span of each `parse` diagnostic is given in 2.5 to 2.10, 3.4, 3.5, 13.2,
   14.2 and 14.3. The span of each `run` diagnostic that the language raises is
   given in 14.5.

## 5. Programs, blocks and scope

1. A **program** is zero or more `const` lines and then one expression. Its
   value is the value of that expression.
2. A **block** is one or more `const` lines and one expression, in braces. A
   **branch** is zero or more `const` lines and one expression, in braces. Each
   is an expression whose value is the value of its expression.
3. `const p = e;` evaluates `e` once and matches the pattern `p` against its
   value (8.4). The names that `p` binds are in scope from the next `const`
   line, or the result, to the end of the program, block or branch.
4. A `const` line whose pattern is a name and whose value is a lambda binds the
   name to the lambda (8.1). It evaluates nothing.
5. **Scopes nest.** From the outside in: the environment's variables and
   functions; the program; each block and branch; the parameters of a lambda;
   the name of a `catch` (11.1). A name refers to the innermost binding of it.
6. A name is not in scope in its own value. In `const x = x + 1;`, the `x` on
   the right refers to an outer `x`.
7. A name may be bound once in one program, block, branch or parameter list, and
   once in one pattern.
8. An inner scope may bind a name that an outer scope binds, or that the
   environment declares, and then hides it. A binding may not have the name of a
   namespace of the environment.
9. The body of a `try` and its handler are two scopes. A name bound in the body
   is not in scope in the handler.
10. Every `const` line is evaluated, in order, whether or not its names are
    used (`02-evaluation.md`).
11. Rules 6 to 8 are static rules (14.4).

## 6. Values

### 6.1 Kinds

There are seven kinds of value.

| Kind | What it is |
| --- | --- |
| `null` | The one absent value. It is written `null` |
| `bool` | `true` or `false` |
| `number` | A finite binary64 value that is not negative zero (6.2) |
| `string` | A finite sequence of code points (6.4) |
| `list` | A finite sequence of values |
| `map` | A finite set of entries. Each entry is a string key and a value. Keys are unique. A map has no order of its own. Wherever an order shows, it is the order of the keys by code point (6.7) |
| opaque | A value the host supplies that is none of the above (`03-host.md`) |

1. A function is not a value (8.1).
2. An error is not a kind of value. The error value of a `catch` is a map
   (11.2).
3. No value can be changed. A value has no identity: two lists with equal
   elements cannot be told apart.
4. An opaque value may be bound, passed to a function, put in a list or a map,
   compared with `==`, and tested for truthiness. It has no fields. It cannot be
   indexed, ordered or rendered as text. Methods on it are declared by the host
   (`03-host.md`).

### 6.2 Numbers

1. A number is a finite IEEE 754 binary64 value. There is no NaN, no infinity
   and no negative zero.
2. **One zero.** An operation whose binary64 result is negative zero gives zero.
   `-0`, `0 * -1` and `-1 % 1` are zero.
3. A **whole number** is a number with no fractional part and a magnitude of at
   most 2^53 − 1. An index, a count and an exponent must be whole. Nothing else
   tells `2` from `2.0`: they are one value.
4. The operators on numbers:

| Operation | Both operands whole | Otherwise |
| --- | --- | --- |
| `a + b`, `a - b`, `a * b` | The exact integer result. If it is not a whole number, the operation fails with `number-range` | The binary64 operation, rounded to nearest, ties to even |
| `-a` | The exact negation. It is always whole | The binary64 negation |
| `a / b` | The binary64 quotient, rounded to nearest, ties to even: `7 / 2` is `3.5`, `6 / 3` is `2` | The same |
| `a % b` | The exact value of `a − b × q`, where `q` is the mathematical quotient `a / b` truncated toward zero. The result has the sign of `a`: `-7 % 3` is `-1`, `5.5 % 2` is `1.5` | The same |
| `a ** b`, `b` whole and `b ≥ 0` | Let `r` be 1. For each bit of `b`, from the highest set bit to the lowest: set `r` to `r × r`, then, if the bit is 1, set `r` to `r × a`. Each `×` follows the row of `a * b`. The result is `r`. `a ** 0` is 1 | The same steps, where each `×` is the binary64 multiplication |
| `a ** b`, `b` whole and `b < 0` | `1 / p`, where `p` is `a ** -b` computed by the steps above with the binary64 multiplication for every `×`, whole or not | The same |
| `a ** b`, `b` not whole | Fails with `wrong-kind` | |

5. A zero divisor in `/` or `%` fails with `divide-by-zero`. So does `0 ** b`
   with `b` below zero.
6. Any result that is NaN or not finite fails with `number-range`. In `a ** b`
   with `b` below zero and `a` not zero: a `p` that is not finite gives the
   result zero, so `2 ** -2000` is zero; a `p` that is zero fails with
   `number-range`, so `0.1 ** -400` fails. With `a` zero the failure is
   `divide-by-zero` (rule 5).
7. A result too small to represent is zero.
8. Each operator rounds once (`02-evaluation.md`).
9. The operands of `+`, `-`, `*`, `/`, `%`, `**` and of prefix `-` are tested by
   14.5: a null operand fails with `null-operand`, and an operand of another
   kind with `wrong-kind`. `+` also joins two strings (6.4).

### 6.3 Numbers and text

1. **Number to text** is the algorithm below. It is ECMAScript's
   `Number::toString` with radix 10.
   1. If the number is zero, the text is `0`.
   2. If the number is negative, the text is `-` followed by the text of its
      magnitude.
   3. Otherwise find the integers `k ≥ 1` and `n` and the digits `d1 … dk`,
      with `d1` not zero, such that `0.d1…dk × 10^n` is a decimal that reads
      back as the number (rounded to nearest, ties to even), and `k` is as small
      as possible. If several digit strings of that length read back, take the
      one closest in value to the number. If two are equally close, take the
      one whose last digit is even.
   4. Then write:

| Case | Text | Examples |
| --- | --- | --- |
| `k ≤ n ≤ 21` | the `k` digits, then `n − k` zeros | `100`, `123456789012345680000` |
| `0 < n < k` | the first `n` digits, `.`, the other digits | `123456.789`, `4.35` |
| `−6 < n ≤ 0` | `0.`, then `−n` zeros, then the digits | `0.5`, `0.000001`, `0.30000000000000004` |
| otherwise | `d1`; if `k > 1`, `.` and the other digits; then `e`, then `+` if `n − 1 ≥ 0` or `-` if not, then the magnitude of `n − 1` in decimal | `1e+21`, `1.5e-7`, `5e-324` |

2. **Text to number** accepts exactly the JSON number grammar: an optional `-`,
   then `0` or a digit run with no leading zero, then optionally `.` and one or
   more digits, then optionally `e` or `E`, an optional `+` or `-`, and one or
   more digits. No `+` at the start, no hex, no `_`, no white space. The value
   is the nearest binary64 value, ties to even. Text whose value is not finite
   is not accepted. `-0` reads as zero.
3. Reading back the text of any number gives that number.
4. The library functions that convert text to a number are built on rule 2 and
   say what they add (`04-library.md`).

### 6.4 Text

1. A string is a sequence of code points. Its length, its positions and its
   order are counted in code points. There is no byte view and no UTF-16 view.
2. Two strings are equal when their sequences of code points are equal. Nothing
   is normalized.
3. `s + t`, with both operands strings, is the string of the code points of `s`
   followed by those of `t`. `+` with one string operand and one operand of any
   other kind fails: with `null-operand` if that operand is null, else with
   `wrong-kind`.
4. `s[i]` is the string of the one code point at position `i`, counting from 0
   (7.1).
5. How the host's text that is not valid enters the language is in `03-host.md`.

### 6.5 Truthiness

1. Any value may be tested as a condition. A value is **falsy** when it is null,
   `false`, zero, the empty string, the empty list or the empty map. Every other
   value is **truthy**.

| Kind | Falsy | Truthy |
| --- | --- | --- |
| null | always | never |
| bool | `false` | `true` |
| number | zero | every other number |
| string | the string with no code point | every other string, `" "`, `"0"` and `"false"` among them |
| list | the list with no element | every list with an element, `[null]`, `[0]` and `[[]]` among them |
| map | the map with no entry | every map with an entry, `{a: null}` among them |
| opaque | never | always |

2. A test reads the kind, and for a string, list or map only whether it has at
   least one code point, element or entry. It never reads an element or a code
   point, and it never asks the host anything about an opaque value.
3. These forms test by truthiness: `!`, `&&`, `||`, the test of `? :`, the tests
   of `if`, and the result of a predicate passed to a library method
   (`04-library.md`). No other form does.

### 6.6 Logical operators and `??`

| Form | Value |
| --- | --- |
| `!a` | `true` if `a` is falsy, else `false`. Always a bool |
| `a && b` | If `a` is falsy, `a`, and `b` is not evaluated. Otherwise `b` |
| `a \|\| b` | If `a` is truthy, `a`, and `b` is not evaluated. Otherwise `b` |
| `a ?? b` | If `a` is not null, `a`, and `b` is not evaluated. Otherwise `b` |

1. `&&` and `||` give back one of their operands, not a bool.
2. None of the four fails on any kind of operand. `??` does not catch a failure
   of `a`.

### 6.7 Equality and order

1. **`a == b`** is a bool and never fails.
   - Values of two different kinds are unequal. Nothing is converted: `1 ==
     true`, `"1" == 1` and `"" == null` are all false.
   - null equals null. Two bools are equal when they are the same.
   - Two numbers are equal when they are the same value. `0.1 + 0.2 == 0.3` is
     false.
   - Two strings are equal when their code points are equal (6.4).
   - Two lists are equal when they have the same length and each element
     equals the element at the same position.
   - Two maps are equal when they have the same set of keys and the values
     under each key are equal.
   - Two opaque values are equal only when they are the same host value, by the
     identity test of `03-host.md`.
2. **`a != b`** has the value of `!(a == b)`. It charges what `a == b` charges,
   and not the step of a `!` (`02-evaluation.md`).
3. A comparison of lists or maps walks into them. A walk nested deeper than the
   depth limit fails with `depth-exceeded` (`02-evaluation.md`).
4. **`a < b`**, `a <= b`, `a > b` and `a >= b` take two numbers or two strings.
   - Numbers compare by value.
   - Strings compare code point by code point, from the first. The first
     position where they differ decides. If one is a proper prefix of the
     other, it is the smaller.
   - A null operand fails with `null-operand`. Any other pair fails with
     `wrong-kind`, so `1 < 2 < 3` fails: its left operand is a bool.
5. **Key order.** Wherever the keys of a map are taken in order, the order is by
   code point, as rule 4 orders strings.

### 6.8 `in` and `not in`

| `y` | `x in y` |
| --- | --- |
| a list | `true` when some element of `y` is `==` to `x`, else `false`. `x` may be any value |
| a string | `x` must be a string. `true` when the code points of `x` occur in `y` as a contiguous run. The empty string occurs in every string |
| a map | `x` must be a string. `true` when `x` is a key of `y` |
| null | fails with `null-operand` |
| any other kind | fails with `wrong-kind` |

1. When `y` is a string or a map and `x` is not a string, `x in y` fails: with
   `null-operand` if `x` is null, else with `wrong-kind`.
2. `x not in y` is `!(x in y)`. It fails exactly when `x in y` fails.

## 7. Reads, indexes, calls and chains

### 7.1 Reads and indexes

| Form | Receiver | Value | When it is not there |
| --- | --- | --- | --- |
| `x.name` | a map | the value under the key `name` | fails with `missing-field` |
| `x[k]` | a map, `k` a string | the value under the key `k` | fails with `missing-field` |
| `x[i]` | a list, `i` a whole number | the element at position `i`, from 0 | fails with `index-range` when `i < 0` or `i ≥` the length |
| `x[i]` | a string, `i` a whole number | the one-code-point string at position `i` | fails with `index-range`, as for a list |
| `x?.name`, `x?.[k]`, `x?.[i]` | as above | as above | null, for that step (7.4) |

1. **The receiver.** A null receiver of a plain step fails with `null-operand`.
   For an optional step it makes the whole chain null (7.4). A receiver of a
   kind that the table does not list fails with `wrong-kind`, for a plain step
   and for an optional step alike: `5.a` and `5?.a` both fail.
2. **The index.** On a list or a string, an index that is a number but not whole
   fails with `wrong-kind`: `xs[1.5]` and `xs[1e20]` both do. An index of
   another kind fails with `wrong-kind`. On a map, a key that is not a string
   fails with `wrong-kind`. A null index or key fails with `null-operand`.
3. The index or key is tested after the receiver, and evaluated after the
   receiver is tested (`02-evaluation.md`).
4. An optional step finds nothing when the key is absent or the whole index is
   out of range. It still fails on a receiver or an index of the wrong kind.
5. `.` reads a field and never a method. `x.name` with no `(` after it reads the
   key `name`, whatever methods the kind of `x` has.
6. There are no negative indexes and no slices.

### 7.2 Method calls

1. `x.m(a, b)` calls the method `m` of the kind of `x`, with `x` as its receiver
   and `a` and `b` as its arguments. A method is a method of the library for
   that kind (`04-library.md`) or one the host declared for it (`03-host.md`).
   The name never reads the map's data: `m.keys()` calls the method even when
   `m` has a key `keys`.
2. A null receiver of `x.m(...)` fails with `null-operand`.
3. **The shape test.** Where the method was not resolved when compiling, it is
   found at run time from the receiver's kind and the name. Then, before any
   argument is evaluated and before the call's charge, these are tested in
   order. The first that fails raises `wrong-kind` at the `MethodCall`:

| # | Test |
| --- | --- |
| 1 | The kind has a method of that name |
| 2 | The method accepts the count of arguments |
| 3 | Each argument at a callback parameter is a lambda: one written in place, or the name of a `const`-bound lambda |
| 4 | Each argument at a parameter that is not a callback is not a lambda |
| 5 | Each lambda has no more parameters than the method calls it with |

Tests 3 and 4 are one pass over the arguments, left to right: the first argument
that fails either decides. Test 5 is a second pass, left to right. A method of
an opaque type takes no callback, so there test 4 fails for any lambda argument.
The five need only the text and the method's record. An implementation may
prepare them when compiling but does not report them then. `04-library.md` and
`02-evaluation.md` have the same tests.

4. `x?.m(...)` with a null receiver makes the chain null (7.4). Otherwise it is
   `x.m(...)`.
5. The receiver is evaluated first, then the arguments, left to right
   (`02-evaluation.md`).

### 7.3 Calls

1. The target of a call is a `Name`, or a namespace and a name (4.2). `f(x)(y)`
   and `(x => x)(1)` do not parse.
2. In `f(a)`, `f` refers to the innermost binding of that name (5).
   - If it is a `const`-bound lambda, the call runs that lambda (8.3).
   - If nothing in the program binds `f`, the call is of the environment's
     function `f`: the library's (`04-library.md`) or the host's
     (`03-host.md`).
   - Any other binding of `f` is an error of the static rules (8.2, 14.4).
3. Arguments are positional and are evaluated left to right. There are no spread
   arguments and no named arguments.

### 7.4 Chains

A **chain** is one `Chain` production: a primary and every `.name`, `?.name`,
`[e]`, `?.[e]` and method call that follows it directly.

1. The steps of a chain are evaluated left to right. Each step receives the
   value of the steps before it.
2. If the receiver of a `?.name`, `?.[e]` or `?.m(...)` step is null, the value
   of the **whole chain** is null. No later step runs. No index expression and
   no argument of that step or of a later step is evaluated.
3. If the receiver of an optional step is not null, the step is the read or call
   of 7.1 and 7.2. A read that finds nothing gives null for that step, and the
   chain goes on. So `a?.b` with no key `b` is null, and `a?.b.c` with no key
   `b` then fails on `.c` with `null-operand`.
4. A plain step on null fails with `null-operand`.
5. A chain ends where its production ends: at a closing parenthesis around it
   and at any operator. `(a?.b).c` is two chains, and the outer one fails when
   `a` is null. A prefix `-` or `!`, a `**` and every binary operator apply to
   the value of the whole chain.
6. An index expression and each argument are expressions of their own. A `?.`
   inside one does not end or skip the outer chain.

So `input.event?.pull_request.head.sha` is null when `input.event` is null, and
fails when `input.event` is a map with no key `pull_request`.

## 8. Lambdas and patterns

### 8.1 Lambdas

1. A lambda is `x => e`, `_ => e`, `(x, i) => e` or `() => e`. Its body is one
   expression, which may be a block. The body reads the names in scope where the
   lambda is written.
2. A lambda may be written in two places and nowhere else:
   - as the value of a `const` line whose pattern is a name, which makes a
     **`const`-bound lambda**;
   - as an argument of a call.
3. The name of a `const`-bound lambda may be used in two ways and no other: it
   may be called by name, as in `double(4)`; and it may be passed as an argument
   where a library function or method takes a function, as in `xs.map(double)`,
   which means what the lambda written there would mean.
4. A function is not a value. A lambda, and the name of a `const`-bound lambda,
   cannot be stored in a list or a map, compared, used as a default, or be the
   value of a `const` line other than as rule 2 says, of a block, of a branch,
   of a lambda or of the program.
5. There is no implicit parameter: no `it` and no `index`.

### 8.2 What may be called, and what may be passed

1. **A lambda is passed only to the library.** An argument that is a lambda,
   written in place or named, may stand only where a library function or method
   takes a function (`04-library.md`).
2. **A lambda is not passed to a `const`-bound lambda.** An argument of a call
   of a `const`-bound lambda that is a lambda, or the name of a `const`-bound
   lambda, is an error.
3. **A lambda is not passed to a host function or method** (`03-host.md`).
4. **Only a function can be called.** In `f(a)`, if the innermost binding of `f`
   is a parameter, a name bound by a pattern, a `const` whose value is not a
   lambda, the name of a `catch`, or a variable of the environment, the call is
   an error. A value cannot be called, whatever it holds.
5. These rules are static rules (14.4). They are decided from the text and the
   bindings alone, with no type, wherever the text stands: `const twice = (f, x)
   => f(f(x));` is an error even when `twice` is never called.

| Written | Allowed |
| --- | --- |
| `const double = x => x * 2;` and then `double(4)` | Yes |
| `xs.map(x => x * 2)`, `xs.map(double)` | Yes |
| `const total = (xs, r) => xs.map(x => x * r).reduce((a, b) => a + b, 0);` | Yes |
| `const twice = (f, x) => f(f(x));` | No: `f` is a parameter |
| `const each = (xs, g) => xs.map(g);` | No: `g` is a parameter, so it is a value |
| `const apply = (x, n) => x * n;` and then `apply(y => y, 2)` or `apply(double, 2)` | No: a lambda passed to a `const`-bound lambda |
| `const {run} = input; run(1)` | No: `run` is a name from a pattern |

### 8.3 Calling a lambda

1. **A call by name** of a `const`-bound lambda supplies an argument for every
   parameter that has no default, and no more arguments than there are
   parameters. Another count is a static error (14.4).
2. **A call by the library** passes the arguments that `04-library.md` gives.
   When it passes more arguments than the lambda has parameters, the extra ones
   are dropped. A lambda that has more parameters than the library's signature
   gives is an error: a static error (`arity`, 14.4) in a call by name; a type
   rule of `05-types.md` at a method call whose receiver's type is known; and
   otherwise `wrong-kind` at run time, by the shape test of 7.2.
3. **Binding.** The parameters are bound left to right. Each parameter's pattern
   is matched against its argument (8.4). A parameter whose argument is omitted
   or is null takes its default, if it has one. The default is evaluated only
   then, and it sees the parameters to its left.
4. The body is then evaluated in a scope that holds the parameters. Its value is
   the value of the call.

### 8.4 Patterns

Patterns stand in `const` lines, in parameters and nowhere else.

| Pattern | Matches | Binds | Fails when |
| --- | --- | --- | --- |
| `x` | any value | `x` to the value | never |
| `_` | any value | nothing | never |
| `{a, b: x}` | a map | `a` to the value under key `a`, `x` to the value under key `b` | the value is null (`null-operand`) or not a map (`wrong-kind`); a key is absent and its field has no default (`missing-field`) |
| `{a, ...rest}` | a map | also `rest`, to a new map of the entries whose keys no field names | as above |
| `[a, _, c]` | a list | `a` to element 0 and `c` to element 2 | the value is null (`null-operand`) or not a list (`wrong-kind`); an element is absent and has no default (`index-range`) |
| `[a, ...rest]` | a list | also `rest`, to a new list of the elements after the last position the pattern names, empty if there are none | as above |

1. **A default**, `name = e`, is used when the key or element is absent or its
   value is null. `e` is evaluated only then. It sees the names bound to its
   left in the same pattern, and in a parameter list the parameters to its left.
2. A field with no default whose key is present with the value null binds null.
3. A list longer than the pattern is not an error.
4. A string is not a list: `const [a] = "x";` fails with `wrong-kind`.
5. Patterns do not nest. A field binds a `Name`, and an element binds a `Name`
   or `_`.
6. A name may be bound once in one pattern (14.4).
7. The fields are matched in written order, then the rest (`02-evaluation.md`).

## 9. Conditionals and `match`

### 9.1 `? :`

1. `c ? a : b` evaluates `c`. If `c` is truthy, the value is `a`, and `b` is not
   evaluated. Otherwise the value is `b`, and `a` is not evaluated.
2. Both branches are required.

### 9.2 `if`

1. The shape is `if (c) { ... } else if (d) { ... } else { ... }`, with any
   number of `else if` parts. The parentheses and the braces are required. The
   `else` is required.
2. Each part in braces is a branch (5). A name bound in a branch is in scope to
   the end of that branch.
3. The tests are evaluated from the first, by truthiness. The branch of the
   first truthy test is taken. If no test is truthy, the `else` branch is taken.
   No later test and no other branch is evaluated.
4. The value of the `if` is the value of the branch taken.
5. `if (c) { a } else { b }` and `c ? a : b` have the same meaning. Both are the
   node `Cond` (4.2).
6. An `if` is a whole expression. It may stand wherever an `Expr` may: a `const`
   value, an argument, a lambda body, a branch, a list item, a map value, an
   interpolation, the branch of `? :`. As an operand, a chain receiver or a
   `match` subject, it is written in parentheses (3.4).

### 9.3 `match`

1. The shape is `match subject { cases }`. Each arm is one or more literals
   joined by `|`, or `_`, then `=>` and a result. Arms are separated by `,`, and
   a final `,` is allowed.
2. A literal is a number, with an optional `-` before it, a one-line string,
   `true`, `false` or `null`.
3. The subject is evaluated once. The arms are tried from the first. In each,
   the subject is compared with `==` (6.7) to each literal, from the first. The
   first arm with an equal literal is chosen. `_` is chosen whatever the subject
   is.
4. The value is the result of the chosen arm. No other result is evaluated.
5. If no arm is chosen, the `match` fails with `no-match`.
6. Cases compare with `==`, never by truthiness. `match n { 0 => "none", _ =>
   "some" }` tests for the number zero.
7. An arm binds no name, and there are no guards.
8. A `match` without a subject is not in the language (3.4).
9. These are static rules (14.4): a literal listed twice; an arm after `_`.
   `05-types.md` has the rules that need types: a literal the subject can never
   equal, and cases that are not covered.

## 10. Building lists and maps

### 10.1 Lists

1. A list literal holds its items in order. A plain item is one element. A
   spread item `...e` puts each element of the list `e`, in order.
2. The operand of a list spread must be a list. Null fails with `null-operand`.
   Any other kind fails with `wrong-kind`.

### 10.2 Maps

1. A map literal holds its entries in written order, and a later entry with a
   key already present replaces it.
   - `word: e` has the key that is the text of the word. Any word may be a bare
     key, a reserved word among them: `{if: 1}`.
   - `"text": e` has the key that is the string's value.
   - `[k]: e` has the key that is the value of `k`, which must be a string. A
     null `k` fails with `null-operand`. Any other kind fails with
     `wrong-kind`.
   - `a`, a shorthand entry, means `a: a`.
   - `...e` puts each entry of the map `e`. Null fails with `null-operand`. Any
     other kind fails with `wrong-kind`.
2. A key written as a word, a string or a shorthand name, twice in one literal,
   is a static error (14.4). A computed key equal to an earlier key replaces
   that entry, and so does a spread.
3. A map keeps no record of the order of its entries. The order shows only by
   key (6.7).

| Want | Write |
| --- | --- |
| Set or add a field | `{...user, role: "admin"}` |
| A key from data | `{[item.id]: item.status}` |
| Remove a field | `{ const {password, ...safe} = user; safe }` |
| Append, prepend, concatenate | `[...xs, x]`, `[x, ...xs]`, `[...a, ...b]` |

## 11. `try`, `catch` and `fail`

### 11.1 `try`

1. The shape is `try { ... } catch (name) { ... }` or `try { ... } catch { ...
   }`. The part after `try` is the **body** and the part after `catch` is the
   **handler**. Each is a branch (5). There is one `catch`, and it is required.
   There is no `finally`.
2. The parentheses after `catch` hold one `Name`. With no name, they are left
   out.
3. The name is in scope in the handler only. It may hide an outer name as a
   `const` name may, and it may not have the name of a namespace (5).
4. The body is evaluated. If it gives a value, that is the value of the `try`,
   and the handler is not evaluated.
5. If the body fails with a code that `catch` catches (11.3), the handler is
   evaluated, with the name, if there is one, bound to the error value (11.2).
   The value of the handler is the value of the `try`.
6. If the body fails with any other code, the `try` fails with that failure. A
   failure of the handler is the failure of the `try`: a `try` does not catch
   what its own handler raises.
7. A `try` is a whole expression, placed as an `if` is (9.2).

### 11.2 The error value

The name of `catch (name)` is bound to a map with exactly these four keys.

| Key | Value |
| --- | --- |
| `code` | The code of the failure, as a string. It is one of the eleven codes of 11.3 |
| `message` | The message of the diagnostic. For `failed`, the first argument of `fail`, the same value. For `function-error`, the text of the host's error. For every other code, the message built from the catalog `messages.json` (`03-host.md`) |
| `path` | For `not-conforming`, the path of the failure (`03-host.md`). For every other code, null. No form of Core raises `not-conforming`, so in Core it is null |
| `data` | For `failed`, the second argument of `fail`, or null when there was none. For every other code, null |

1. The error value is an ordinary map. It may be read, compared, spread,
   rendered, returned and passed on.
2. It has no class, no span and no trace.
3. Its rendering, as for any map, has the keys in code point order: `code`,
   `data`, `message`, `path`.

### 11.3 What `catch` catches

`catch` decides by the code of the failure and by nothing else.

| `catch` catches | `catch` lets pass |
| --- | --- |
| `divide-by-zero`, `failed`, `function-error`, `index-range`, `invalid-argument`, `missing-field`, `no-match`, `not-conforming`, `null-operand`, `number-range`, `wrong-kind` | `steps-exceeded`, `allocation-exceeded`, `deadline-exceeded`, `depth-exceeded`, `cancelled`, `function-panic`, `internal-error`, `missing-variable` |

1. A code that `catch` lets pass goes through every `try` in progress and ends
   the run.
2. `03-host.md` lists every run code. `02-evaluation.md` says what a caught
   failure costs and how the depth count is restored.

### 11.4 `fail`

1. `fail(message, data)` and `fail(message)` are calls of the library function
   `fail` (`04-library.md`). The call always fails.
2. The failure has the code `failed` and the span of the `Call` node. Its
   message is the first argument, whole. Its `data` is the second argument, or
   null when there is none.
3. A caught error cannot be raised again with its own code. `fail(err.message,
   err)` raises a new `failed` failure that carries the old error value as its
   data.
4. `throw` is a reserved word, and there is no `throw` form.

## 12. Rendering

The **rendering** of a value is its text. A template's value is the
concatenation of its text parts and the rendering of the value of each
interpolation, in order. The library function `string(v)` gives the rendering of
`v` (`04-library.md`).

| Value | Rendering |
| --- | --- |
| string | its code points, unchanged |
| number | the text of 6.3 |
| bool | `true` or `false` |
| null | the empty string |
| list | `[`, then the JSON form of each element, separated by `,`, then `]` |
| map | `{`, then for each entry in key order (6.7): the JSON form of the key, `:`, the JSON form of the value; the entries separated by `,`; then `}` |
| opaque | fails with `wrong-kind` |

The **JSON form** of a value:

| Value | JSON form |
| --- | --- |
| string | `"`, then each code point as below, then `"` |
| number | its rendering |
| bool | its rendering |
| null | `null` |
| list, map | its rendering |
| opaque | fails with `wrong-kind`, at any depth |

| Code point in a string | Written as |
| --- | --- |
| `"` | `\"` |
| `\` | `\\` |
| U+0008, U+000C, U+000A, U+000D, U+0009 | `\b`, `\f`, `\n`, `\r`, `\t` |
| any other code point below U+0020 | `\u00` and two lower-case hex digits |
| every other code point, `/`, `<`, `&`, U+007F and U+2028 among them | itself |

1. No white space is added anywhere.
2. Rendering a list or a map is a walk. A walk nested deeper than the depth
   limit fails with `depth-exceeded` (`02-evaluation.md`).
3. A template whose rendering fails fails with that code.

| Value | Rendering |
| --- | --- |
| `null` | (empty) |
| `1.5e-7` | `1.5e-7` |
| `[1, "a", null, true]` | `[1,"a",null,true]` |
| `{b: [], a: {c: "x\ny"}}` | `{"a":{"c":"x\ny"},"b":[]}`, where `\n` is the two characters `\` and `n` |
| `"say \"hi\""` | `say "hi"` |
| `["say \"hi\""]` | `["say \"hi\""]` |

## 13. Nesting

### 13.1 Levels

The parser counts the **levels** of nesting that are open. Each of these opens
one level.

| Opens a level | The level lasts |
| --- | --- |
| A bracket: `(`, `[`, `?.[`, `{`, or the `${` of a template | to its closing bracket. This covers a parenthesized expression, an argument list, a parameter list, an index, a list, a map, a computed key, a pattern, a block, a branch, the test of an `if`, the parentheses of `catch (name)`, the arms of a `match` and an interpolation |
| A prefix operator: `!` or `-` | to the end of its operand |
| A binary operator: `\|\|`, `&&`, `??`, `==`, `!=`, `<`, `<=`, `>`, `>=`, `in`, `not in`, `+`, `-`, `*`, `/`, `%` or `**` | to the end of its right operand |
| The `?` of a conditional | to its `:` |
| The word `match` | to the `{` after its subject |

Nothing else opens a level. In particular these do not:

| Not a level | So this is flat, however long |
| --- | --- |
| The left operand of a binary operator | `a + b + c + ...`: each term after the first is one level inside, and no term is inside another |
| The receiver of a step or a method call | `x.f().g().h()...`: each argument list is one level, and no call is inside another |
| The test and the `else` branch of `? :` | `a ? x : b ? y : c ? z : w` |
| The words `if` and `else`, and an `if` that follows an `else` | an `if` with any number of `else if` parts: each test and each branch is one level inside, and no branch is inside another |
| The words `try` and `catch` | `try { a } catch (e) { b }`: the body, the parentheses and the handler are each one level |
| The next `const` line, and the result of a program, block or branch | a script of any number of `const` lines |
| The next item, entry, argument, parameter, field, element or arm | a long list; a `match` with many arms |
| A lambda's `=>` and body | the body is at the level of the lambda |
| The `-` of a `match` literal | |
| White space, comments, and the text of a string or a template | only a `${` in a template opens a level |

### 13.2 The limit

1. At most **100** levels may be open at once.
2. The token that would open level 101 is `nesting-exceeded`, of phase `parse`.
   Its span is that token. For `not in`, the span runs from `not` to the end of
   `in`.
3. Parsing ends at that token. Nothing is reported for the text after it.
4. The number belongs to this specification. A host cannot change it.
5. The **nesting depth** of an expression is the number of levels open at its
   first token. `02-evaluation.md` uses it for the depth limit.
6. What does not open a level must not cost an implementation stack. A sum of
   100,000 terms, a chain of 100,000 calls, a `? :` with 100,000 `else`
   branches, an `if` with 100,000 `else if` parts and a block of 100,000 `const`
   lines each parse, and each runs within the limits of `02-evaluation.md`.

| Text | Levels open at `x` |
| --- | --- |
| `x` | 0 |
| `f(x)` | 1 |
| `a + b * x` | 2: the `+` and the `*` |
| `a + b + x` | 1: the second `+` |
| `-(x)` | 2: the `-` and the `(` |
| `c ? x : y` | 1: the `?` |
| `c ? y : x` | 0 |
| `` `${[x]}` `` | 2: the `${` and the `[` |
| `if (a) { 1 } else if (b) { 2 } else { x }` | 1: the braces of the last branch |
| `match x { 1 => 2 }` | 1: the word `match` |

## 14. Diagnostics of the language

### 14.1 The first `parse` diagnostic

1. Only the first `parse` diagnostic is specified: its code and its span. Its
   message is not specified in Core. What a parser reports after it is in
   `03-host.md`.
2. If the text has a lexical diagnostic, the first diagnostic is lexical (2.11).
3. Otherwise the parser reads the tokens from the start. The first diagnostic is
   the first rule the text breaks as the parser reads it: a token that does not
   fit (3.5), a rule of 3.4 or 14.3, a reserved word by 14.2, or a level past
   the limit (13.2). A rule of 3.4 whose span is an earlier token is broken at
   the token where the parser finds it: rule 1 at the `**`, rule 2 at the `in`,
   rule 7 at the `:`, `,` or `...`, rule 8 at the `}`, rule 9 at the `=>`, rule
   10 at the `(`. The `!x as T` row of 14.3 is broken at the `as` or the `is`.

### 14.2 The `parse` codes in Core

Core has 14 `parse` codes: the 13 general codes and `reserved-word`.

| Code | Reported when | Span |
| --- | --- | --- |
| `unexpected-token` | Text does not fit the grammar (3.5); a rule of 3.4 is broken; a token of the last row of 2.3 stands anywhere; text of 14.3 | 3.4, 3.5, 14.3; for a token of 2.3, 2.3 rule 4 |
| `unexpected-character` | A code point that begins no token (2.10) | The code point |
| `unterminated-string` | A one-line string or a template has no closing delimiter (2.6, 2.8) | From the opening delimiter to where the literal stops |
| `unterminated-comment` | A `/*` has no `*/` (2.2) | The `/*` |
| `bad-escape` | 2.7 | 2.7 |
| `line-continuation` | A `\` directly before a line end in a template (2.8) | The `\` |
| `bad-number` | A number token that does not match `Number` (2.5) | The number token |
| `literal-range` | A number literal that is not in range (2.5) | The number token |
| `nesting-exceeded` | A token would open level 101 (13.2) | That token, or `not in` |
| `text-block-open` | Not reported in Core | |
| `text-block-close` | Not reported in Core | |
| `text-block-indent` | Not reported in Core | |
| `bad-type-text` | Not reported in Core | |
| `reserved-word` | The rule below | The word |

**`reserved-word`.** A reserved word is `reserved-word` when it stands:

1. where the grammar needs a `Name`: the pattern of a `const`, a parameter, a
   field or element of a pattern, a rest, the name of a `catch`, a shorthand
   entry of a map (3.2 rule 7); or
2. where the grammar needs the start of an `Expr` or of an operand, and the word
   cannot begin one there. `true`, `false`, `null` and `match` begin an operand.
   `if` and `try` begin an `Expr` but not an operand: where an operand is needed
   they are `unexpected-token` (3.4 rule 5). `const` begins a `Const` only at
   the start of a program, block or branch, or after a `Const`.

So `const match = 1;`, `is => 1`, `let x = 1;`, `return x`, `nil`, `[for (x of
xs) x]`, `{for}` and `1 + else` are each `reserved-word` at the word. After `.`
and `?.`, and as a bare key, a reserved word is a word like any other: `x.nil`,
`err.catch` and `{for: 1, import: 2}` are valid.

### 14.3 Follow-on text and targeted errors

A later package gives some text a meaning, or a `parse` code of its own. In Core
such text is an error with a general code, at the span in the third column. That
span is the span the later code has. Each row holds when the text has no lexical
diagnostic. A lexical diagnostic comes first (2.11).

| Text | Core code | Span |
| --- | --- | --- |
| A text block, `"""`; `'''` | `unexpected-token` | The first three code points of the block token (2.3 rule 7) |
| A raw string, `r"..."` or `r'...'`; a prefixed string, `f"..."` or `R"..."`; a tagged template, `` r`...` `` or `` String.raw`...` ``: the prefixed literal (2.3 rule 9) | `unexpected-token` | The word |
| The word `r` or `R` followed by white space and then `"` or `'` | `unexpected-token` | The word |
| `x as T`, `x is T`, `x as const`, `x is not T` | `unexpected-token` | The `as` or the `is` |
| `!x as T`, `!x is T`: `as` or `is` whose left operand is a `!` expression written without parentheses | `unexpected-token` | The `!` |
| A projection, `xs[*]` or `xs?.[*]` | `unexpected-token` | The `*` |
| `match {`, with no subject | `unexpected-token` | The `{` |
| `===`, `!==` | `unexpected-token` | The token |
| `x = 1` | `unexpected-token` | The `=` |
| `x += 1`, `x ++`, and the other tokens of the last row of 2.3 | `unexpected-token` | The token |
| `x \|> f` | `unexpected-token` | The `\|>` |
| `a & b`, `$`, `$.a`, `$"..."`, `# comment`, `@` | `unexpected-character` | The `&`, `$`, `#` or `@` |
| `a \| b` outside a `match` arm | `unexpected-token` | The `\|` |
| `if (c) { a }` with no `else` | `unexpected-token` | 3.4 rule 11 |
| A branch written like a map; an empty branch | `unexpected-token` | 3.4 rules 7 and 8 |
| An `if` or a `try` followed by an operator, `.`, `[`, `?.` or `?.[` | `unexpected-token` | 3.4 rule 4 |
| `try(e, d)` | `unexpected-token` | The `try` |
| `try { e }` with no `catch` | `unexpected-token` | 3.4 rule 12 |
| `a ?? b > c`, `a ?? b \|\| c`; `!a in b`; `-2 ** 2`; a `match` subject that starts with `{`, `if` or `try` | `unexpected-token` | 3.4 rules 3, 2, 1 and 6 |
| `const {a} = x => x;` | `unexpected-token` | The `=>` |
| `let`, `var`, `nil`, `for`, `while`, `do`, `break`, `continue`, `return`, `function`, `throw`, `finally`, `import`, at a place where a value may start | `reserved-word` | The word |
| `finally` after the handler of a `try` | `unexpected-token` | The `finally` |
| `x[a:b]`; `(x: int) => x`; `const n: int = 1;` | `unexpected-token` | The `:` |
| `xs.*.f`; `xs[?c]`; `xs[].f` | `unexpected-token` | The `*`; the `?`; the `]` |
| `<T>x`, `<<EOF` | `unexpected-token` | The first `<` |
| A line end in a one-line string | `unterminated-string` | 2.6 |
| A `\` before a line end in a template | `line-continuation` | The `\` |
| Every other text that a later package covers: `if c { a } else { b }`, `if (c) a else b`, `if(c, a, b)`, `try e catch d`, `catch (e: T)`, `catch ({message})`, `catch (_)`, `catch ()`, `switch`, `typeof x`, `x instanceof T`, `x satisfies T`, `interface`, `enum`, `type Name = T`, `export const`, `x => { x + 1 }`, `a?[i]`, `{max-retries: 3}` | `unexpected-token` | 3.5 |

`02` 3.2 fixes the span of some rows only in general terms and leaves the exact
span to the suite. For those rows the Core suite states the phase and no span,
and follow-on F8 adds the cases that pin it. They are the last row of the table;
`x as const` and `x is not T`; `xs.*.f` and `xs[].f`; `const {a} = x => x;`; and
a `match` subject that starts with `{`, `if` or `try`. The Core suite pins the
span of every other row.

### 14.4 Static rules

These rules need the program's text and the environment's names, and no types. A
program that breaks one does not compile. Every implementation applies them when
it compiles, whether or not it has a type checker, and reports each in phase
`check` with the code below. A program compiled without a check is compiled
without the type rules, never without these. `05-types.md`, section "The static
rules alone", gives each code's span, the order of the diagnostics, and what is
skipped after one. The message of a `check` diagnostic is not specified in Core.

| Rule | Section | Code |
| --- | --- | --- |
| A name that is read is bound or declared | 5 | `undefined-name` |
| A name that is called is a `const`-bound lambda or a function of the environment | 7.3, 8.2 | `undefined-function` or `call-of-parameter` |
| A lambda is not passed to a `const`-bound lambda | 8.2 | `lambda-to-lambda` |
| A function, a `const`-bound lambda's name or a namespace is not used as a value; in a call by name, a lambda is passed only where the signature takes a function | 8.1, 8.2 | `not-a-value` |
| A call by name, of a `const`-bound lambda, a library function or a host function, has a right count of arguments; a lambda passed to a function known by name has no more parameters than its signature gives | 8.3 | `arity` |
| A name is bound once in a scope and once in a pattern | 5, 8.4 | `duplicate-name` |
| A binding does not have the name of a namespace | 5 | `shadows-namespace` |
| A map literal does not write one key twice as a word, a string or a shorthand | 10.2 | `duplicate-key` |
| A `match` does not list a literal twice | 9.3 | `duplicate-case` |
| No arm of a `match` follows `_` | 9.3 | `unreachable-arm` |

At a method call none of these is a static rule: the count of arguments, a
lambda where the method takes a value, a value where it takes a function, and a
lambda with too many parameters. Each needs the receiver's type, so it is a type
rule of `05-types.md` (`arity`, `not-a-value`, `arg-mismatch`). Where no type
rule applied, the run tests them by the shape test of 7.2.

### 14.5 The run codes the language raises

Each row is a failure of a form of this chapter. The span is the span of the
node in the third column (4.3). The `path` of each is null.

| Code | Raised when | Span |
| --- | --- | --- |
| `null-operand` | The receiver of a plain `.name` or `[e]` is null (7.1) | The `Member` or `Index` |
| `null-operand` | The index or key of `[e]` or `?.[e]` is null (7.1) | The `Index` |
| `null-operand` | The receiver of a plain method call is null (7.2) | The `MethodCall` |
| `null-operand` | An operand of `+`, `-`, `*`, `/`, `%`, `**` or prefix `-` is null (6.2, 6.4) | The `Binary` or `Unary` |
| `null-operand` | An operand of `<`, `<=`, `>` or `>=` is null (6.7) | The `Binary` |
| `null-operand` | The right operand of `in` or `not in` is null; or it is a string or a map and the left operand is null (6.8) | The `Binary` |
| `null-operand` | The operand of a list or map spread is null (10) | The `Spread` |
| `null-operand` | A computed key is null (10.2) | The `Entry` |
| `null-operand` | A map or list pattern is matched against null (8.4) | The `MapPat` or `ListPat` |
| `wrong-kind` | The receiver of `.name`, `?.name`, `[e]` or `?.[e]` is not null and not of a kind 7.1 lists for the step | The `Member` or `Index` |
| `wrong-kind` | An index on a list or a string is not a whole number; a key on a map is not a string (7.1) | The `Index` |
| `wrong-kind` | A test of the shape test fails at a method call: the receiver's kind has no method of that name; the method does not accept the count of arguments; an argument at a callback parameter is not a lambda; a lambda stands at a parameter that is not a callback; a lambda has more parameters than the method calls it with (7.2) | The `MethodCall` |
| `wrong-kind` | An operand of `-`, `*`, `/`, `%`, `**` or prefix `-` is not a number; the operands of `+` are not two numbers or two strings; the exponent of `**` is not whole (6.2, 6.4) | The `Binary` or `Unary` |
| `wrong-kind` | The operands of `<`, `<=`, `>` or `>=` are not two numbers or two strings (6.7) | The `Binary` |
| `wrong-kind` | The right operand of `in` is not a list, a string or a map; or it is a string or a map and the left operand is not a string (6.8) | The `Binary` |
| `wrong-kind` | A list spread of a value that is not a list; a map spread of a value that is not a map (10) | The `Spread` |
| `wrong-kind` | A computed key that is not a string (10.2) | The `Entry` |
| `wrong-kind` | A map pattern matched against a value that is not a map; a list pattern against a value that is not a list (8.4) | The `MapPat` or `ListPat` |
| `wrong-kind` | An opaque value is rendered, at any depth (12) | The `Template` |
| `missing-field` | A plain `.name` or `[k]` on a map that has no key `name` or `k` (7.1) | The `Member` or `Index` |
| `missing-field` | A field of a map pattern whose key is absent and that has no default (8.4) | The field |
| `index-range` | A plain `[i]` on a list or a string with `i` whole and below 0 or not below the length (7.1) | The `Index` |
| `index-range` | An element of a list pattern that is absent and has no default (8.4) | The element |
| `divide-by-zero` | The divisor of `/` or `%` is zero; `0 ** b` with `b` below zero (6.2) | The `Binary` |
| `number-range` | `+`, `-`, `*` or `**` on whole numbers whose exact result is not whole; any arithmetic result that is not finite (6.2) | The `Binary` or `Unary` |
| `no-match` | No arm of a `match` is chosen (9.3) | The `Match` |
| `failed` | A call of `fail` (11.4) | The `Call` |

1. A `null-operand` and a `wrong-kind` at the same place are different codes: a
   null raises the first, and a value of another kind the second.
2. `not-conforming` is a run code of Core, and `catch` catches it, but no form
   of Core raises it.
3. These codes are raised by other chapters, at forms of this chapter:
   `missing-variable`, when a name of the environment is read and the host did
   not supply it (`03-host.md`); `number-range` for a host number that cannot
   enter the language (`03-host.md`); `function-error` and `function-panic` from
   a host function or method (`03-host.md`); `invalid-argument` from a library
   entry, and `wrong-kind` for a null or wrongly kinded argument or element of a
   library entry (`04-library.md`); `depth-exceeded`, `steps-exceeded`,
   `allocation-exceeded`, `deadline-exceeded` and `cancelled`
   (`02-evaluation.md`).
4. The message of each code is built from `messages.json` (`03-host.md`). The
   class of each is in `03-host.md`.

## 15. Examples

The variables in these examples are values the host supplied. `u` is `{name: "",
tags: []}`. A value is shown as a literal.

| # | Text | Value or code |
| --- | --- | --- |
| 1 | `7 / 2` | `3.5` |
| 2 | `-7 % 3` | `-1` |
| 3 | `2 ** 10` | `1024` |
| 4 | `2 ** -1` | `0.5` |
| 5 | `9007199254740991 + 1` | `number-range` |
| 6 | `1e308 * 10` | `number-range` |
| 7 | `1 / 0` | `divide-by-zero` |
| 8 | `-2 ** 2` | `unexpected-token` at `-` |
| 9 | `9007199254740992` | `literal-range` |
| 10 | `010` | `bad-number` |
| 11 | `0.1 + 0.2` | `0.30000000000000004` |
| 12 | `` `${1e21} ${0.000001} ${1e-7}` `` | `"1e+21 0.000001 1e-7"` |
| 13 | `u.name \|\| "anonymous"` | `"anonymous"` |
| 14 | `u.name ?? "anonymous"` | `""` |
| 15 | `!u.tags` | `true` |
| 16 | `[] && 1` | `[]` |
| 17 | `1 == true` | `false` |
| 18 | `[1, [2]] == [1, [2]]` | `true` |
| 19 | `"a" < "b" && "ab" > "a"` | `true` |
| 20 | `"n=" + 5` | `wrong-kind` |
| 21 | `"" in "abc"` | `true` |
| 22 | `"x" in {x: null}` | `true` |
| 23 | `1 in null` | `null-operand` |
| 24 | `u.missing` | `missing-field` |
| 25 | `u?.missing` | `null` |
| 26 | `null?.a.b.c` | `null` |
| 27 | `(null?.a).b` | `null-operand` |
| 28 | `5?.a` | `wrong-kind` |
| 29 | `"héllo"[1]` | `"é"` |
| 30 | `[1, 2][2]` | `index-range` |
| 31 | `[1, 2]?.[2]` | `null` |
| 32 | `{ const [a, ...r] = [1, 2, 3]; {a, r} }` | `{a: 1, r: [2, 3]}` |
| 33 | `{ const {a = 5, b} = {a: null, b: 2}; a + b }` | `7` |
| 34 | `{ const {a} = {}; a }` | `missing-field` |
| 35 | `{ const f = (x, n = 2) => x * n; f(3) + f(3, null) }` | `12` |
| 36 | `if (0) { "a" } else if ("") { "b" } else { "c" }` | `"c"` |
| 37 | `if (true) { 1 }` | `unexpected-token` at the end of input |
| 38 | `if (true) { a: 1 } else { 2 }` | `unexpected-token` at the first `{` |
| 39 | `(if (true) { 1 } else { 2 }) + 3` | `4` |
| 40 | `match 2 { 1 \| 2 => "low", _ => "high" }` | `"low"` |
| 41 | `match "c" { "a" => 1, "b" => 2 }` | `no-match` |
| 42 | `{...{a: 1, b: 2}, b: 3, [("c")]: 4}` | `{a: 1, b: 3, c: 4}` |
| 43 | `[...null]` | `null-operand` |
| 44 | `` `${[1, "a"]}-${null}-${{b: 1, a: true}}` `` | the string whose text is `[1,"a"]--{"a":true,"b":1}` |
| 45 | `try { 1 / 0 } catch (e) { e.code }` | `"divide-by-zero"` |
| 46 | `try { fail("no", {n: 1}) } catch (e) { e }` | `{code: "failed", message: "no", path: null, data: {n: 1}}` |
| 47 | `try { [][0] } catch { "none" }` | `"none"` |
| 48 | `try 1 catch { 2 }` | `unexpected-token` at `1` |
| 49 | `try(1, 2)` | `unexpected-token` at `try` |
| 50 | `const match = 1; match` | `reserved-word` at the first `match` |
| 51 | `{if: 1}.if` | `1` |
| 52 | `a ?? b > c` | `unexpected-token` at `>` |
| 53 | `x as int` | `unexpected-token` at `as` |
| 54 | `r"a"` | `unexpected-token` at `r` |
| 55 | `"a\q"` | `bad-escape` at `\q` |
| 56 | 101 `(` followed by `1` and 101 `)` | `nesting-exceeded` at the 101st `(` |
| 57 | `[1, 2].map(_ => 0)` | `[0, 0]` |
| 58 | `2 ** -2000` | `0` |
| 59 | `0.1 ** -400` | `number-range` |
| 60 | `0 ** -1` | `divide-by-zero` |
