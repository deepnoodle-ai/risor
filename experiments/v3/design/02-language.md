# Risor v3: language surface and semantics

Status: design proposal, revised to the seventh revision of `00-overview.md`.
Nothing here is implemented. Decision numbers such as N2, B5 and R5 refer to
that overview. The fourth revision applies D28 (`null` is the one spelling of
the absent value), D29 (type text in TypeScript's spelling), D30 (the falsy
set), D32 and D33 (constant conditions, and a host that tests a result by
truthiness) and D34 (where a lambda may go). The fifth applies D35 (`if`, `else
if` and `else` form an expression) and D36 (a binding is written `const`). The
sixth applies D39 to D45: `x as T` and `x is T` test a value against a type
(2.17). `07-checked-conversion.md` is the authority for that feature, and this
document states only the part it owns.

The seventh revision applies D46 to D51, six plans that the owner accepted:
string forms (`08`), tagged unions (`09`), `try` with `catch` and the error
value (`10`), testing (`11`), config files (`12`) and queries (`13`). Each plan
is the authority for its feature. This document holds what it owns: the grammar
and tokens of the new forms, the nodes, the reserved words, the nesting rules,
and the name of every `parse` code and of every run code the language raises
(2.12, 3.2). A decision of a plan is cited by its own number, such as 08-4 or
10-3, and `10` 2.8 means section 2.8 of plan `10`.

**How to read this.** Section 2 defines the language. In it, a rule is
normative: any implementation must do it. A paragraph headed *Why*, *Rejected*
or *Note* is not normative. Sections 3 to 8 are not normative. Nothing in
section 2 depends on Go, Rust or TypeScript. Notes for each of the three
implementations, and on v2's parser, are in section 8.

**Evidence.** A `file:line` reference means I read those lines. `v2:` is this
repository at `07efd88`, `spec:` is `docs/reference/spec.md` in the expr
repository at v1.4.1, `wf:` is `github.com/deepnoodle-ai/workflow@v0.1.0`.
**(ran)** means I ran the snippet in Risor v2, expr, Node 22, TypeScript 5.9, Go
1.26, Rust 1.99 or PyYAML, in throwaway programs outside the repository. **(I)**
marks an inference.

## 1. Summary

1. **One construct, not two forms.** A program is `const` bindings followed by
   an expression. The same thing in braces is a block, which is an expression. A
   `when:` field and a script are the same language (E2, E5).
2. **TypeScript's expression syntax, with TypeScript's meaning or an error**
   (C6, Y1). The nine places where the same text means something else are
   listed and defended in 3.1. The spellings that are not TypeScript's are
   listed with their reasons in 3.4.
3. **Three conditionals, each with its use.** `if`, `else if` and `else` form an
   expression whose branches are block bodies (D35). `? :` is the short form
   inside a larger expression. `match` chooses by literal cases, checked for
   exhaustiveness, and for now also by conditions (E1, D38).
4. **Destructuring** of maps and lists in `const` and in lambda parameters, with
   defaults and a rest name (E3).
5. **One kind of number** with IEEE double values. Whole numbers are exact in
   ±(2^53 − 1), and whole-number arithmetic that leaves that range is an error.
   `/` is true division. `**` is back, with a whole-number exponent (N1-N4, E4).
6. **Any value may be a condition** (B1). Null, `false`, zero and anything empty
   are falsy. `&&` and `||` return an operand; `||` replaces a falsy value and
   `??` replaces null only (B5, B6). **`?.` skips the rest of its chain when its
   receiver is null** (B4). A missing field is an error with `.` (B2). The
   absent value is written `null` and nothing else (D28).
7. **Values cannot be changed.** Spread, computed keys and rest patterns build
   new ones. A binding is written `const` and never changes (D36). There is no
   assignment and no loop in 3.0; their words are reserved. There is no
   recursion (E5, E6).
8. **One error-handling construct:** `try { e } catch (err) { f }`, an
   expression that stands where an `if` stands. `err` is an ordinary map with a
   `code`. A limit, a cancellation, a crash and a variable the host did not
   supply are not caught (D48; 2.12).
9. **Text is code points** (T1). Rendering a value as text has one exact rule,
   and numbers use ECMAScript's `Number::toString` (N7).
10. **Every program terminates, by construction.** A lambda is passed only to a
    library function or method, and a parameter cannot be called, so the text
    fixes which lambda every call enters (E6, D34). Every construct that creates
    a value is listed with its size, so a runtime can charge for it first (L2,
    L3).
11. **Source may open 100 levels of nesting and no more**, by a count on the
    text that every implementation makes the same way. A long chain is not
    nesting. The depth limit counts nesting across calls, so the stack an
    evaluator needs is bounded by a sum of the two limits, not their product
    (R5, 2.16).
12. **Untyped data is tested against a type.** `x as T` gives `x` back with type
    `T`, or fails. `x is T` gives a `bool`. The type is type text written in
    place or a name the host declared. Nothing is converted (D39, D40; 2.17). A
    type may hold several record types when one field tells them apart (D47).
13. **Five string forms, chosen by two questions:** one line or several, and
    values and escapes or neither. A text block, `"""`, holds lines and removes
    the indentation that its closing delimiter sets. A raw prefix, `r`, turns
    off escapes and `${ }`. The form is spelling: no node kind, value, type or
    cost depends on it (D46; 2.2).
14. **A projection reads through nested lists.** `orders[*].lines[*].sku`
    applies the reads after each star to every element and gives one flat list.
    A method call ends it and receives the list (D51; 2.8).
15. **Every failure has a code with a name.** 2.12 names the run codes that the
    language raises, and 3.2 names every `parse` code. The wording of each
    message is data of the specification, `messages.json`, and is not written
    here (D48).

Restored since the first draft: `match`, blocks, destructuring, default
parameters, `**`, string indexing, `//` comments everywhere, digit separators,
nested template strings, and, in the fifth revision, `if` and `else` (D35).
Added: computed map keys, and in the sixth revision `as` and `is` (D39). Added
in the seventh: the text block and the raw prefix (D46), `catch`, which
replaces the call form `try(e, fallback)` (D48), and the projection step (D51).
Not in 3.0, with their words reserved: loops, reassignment and early `return`
(D36, overview 2.10), and imports, which come in a minor version (`12` 3.9).
Still out, each with its reason in 4.1: named functions, slice syntax, negative
indexes, pipes, bitwise operators, spread arguments. Type annotations are out of
3.0 with the syntax reserved (8.1, question 8). Changed in the fourth and fifth
revisions: `nil` is no longer a keyword (D28), a lambda is not passed to a
`const`-bound lambda (D34), and `let` became `const` (D36).

## 2. The language

### 2.1 Programs, blocks and scope

- A **program** is `{ Const } Expr`. Its value is the value of the expression.
- A **block** is `"{" Const { Const } Expr "}"`. It is an expression and may
  appear wherever one may: a lambda body, a branch of `? :`, a `match` arm, an
  argument.
- A **branch** is `"{" { Const } Expr "}"`: a block body that needs no `const`
  line. An `if` has branches (2.10). So has a `try`: its body and its handler
  (2.12).
- **A `{` starts a block exactly when the next token is `const`** (and the token
  after that is not `:`). Every other `{` starts a map literal. There is one
  exception, and it is by position: directly after `if (c)`, `else`, `try`,
  `catch` and `catch (err)`, a `{` always opens a branch and never a map (2.10,
  2.12). And where a type is expected, after `as` or `is`, a `{` starts a record
  type (2.2).
- `const p = e;` evaluates `e` once and binds the names in pattern `p` for the
  rest of its block or branch. Bindings never change. A name is not in scope in
  its own value. A name may be bound once per block or branch. An inner block, a
  branch, a lambda parameter, a `const` or the name of a `catch` may shadow an
  outer name or an environment variable, but not a namespace (S1).
- Evaluation is strict and in source order: every `const` runs, whether or not
  its name is used.

A host that takes one expression in a `when:` field, a host that takes a script
file and a host that loads a config file (D50; `12`) call the same parser.
Nothing in the language differs. The host
passes the result it expects, and may refuse a program that contains a line
comment (see 3.3). A host that wants a real boolean passes `bool`. A host that
tests the result by truthiness passes `condition`, and any type fits. The
workflow engine does the second for `when:` (B8, D33; 2.7, `05` A.3).

*Why this shape.* A block that starts with `const` cannot be confused with a map
literal, so `o => {id: o.id}` is a map and `o => { const t = o.qty * o.price;
{id: o.id, total: t} }` is a block, with no rule about where the brace stands. A
brace with no `const` has no reason to be a block: write the expression.

*Why `const` (D36).* In TypeScript `let` says that the binding will be
reassigned, and nothing here can be. `const` is the word a TypeScript author,
and a model, writes first for a binding that never changes (C6). `let` and `var`
are reserved, and each is a targeted error that says `const` (3.2). If
reassignment arrives in a later version (overview X3, X4), `let` is free to mean
it.

*Rejected.* JavaScript's rule (a `{` after `=>` is a block) brings back its
best-known arrow trap. A `let ... in e` form without braces has no closing
token, which reads badly inside an argument list.

### 2.2 Grammar

```ebnf
Program = { Const } Expr [ ";" ] .
Const   = "const" Pattern "=" ( Lambda | Expr ) ";" .
Block   = "{" Const { Const } Expr "}" .

Expr    = If | Try | Binary [ "?" Expr ":" Expr ] .     (* right-associative *)
If      = "if" "(" Expr ")" Branch
          { "else" "if" "(" Expr ")" Branch } "else" Branch .
Try     = "try" Branch "catch" [ "(" Name ")" ] Branch .
Branch  = "{" { Const } Expr "}" .
Binary  = Or | Coal .
Coal    = Add "??" Add { "??" Add } .
Or      = And   { "||" And } .
And     = Equal { "&&" Equal } .
Equal   = Rel   { ( "==" | "!=" ) Rel } .
Rel     = Add   { ( "<" | "<=" | ">" | ">=" | "in" | "not" "in" ) Add
                | ( "as" | "is" ) Type } .              (* Type: 03, 2.2 *)
Add     = Mul   { ( "+" | "-" ) Mul } .
Mul     = Unary { ( "*" | "/" | "%" ) Unary } .
Unary   = ( "!" | "-" ) Unary | Power .
Power   = Chain [ "**" Unary ] .                        (* right-associative *)
Chain   = Primary { "." Word [ Args ] | "?." Word [ Args ]
                  | "[" Expr "]" | "?.[" Expr "]" | Star } .
Star    = "[" "*" "]" | "?.[" "*" "]" .                 (* a projection: 2.8 *)
Primary = Number | String | Template | "true" | "false" | "null"
        | Name [ Args ] | "(" Expr ")" | List | Map | Block | Match .

Args    = "(" [ Arg { "," Arg } [ "," ] ] ")" .
Arg     = Lambda | Expr .
Lambda  = ( Name | "(" [ Param { "," Param } [ "," ] ] ")" ) "=>" Expr .
Param   = Pattern [ "=" Expr ] .

Pattern = Name | "_" | MapPat | ListPat .
MapPat  = "{" [ Field { "," Field } ] [ "," "..." Name ] [ "," ] "}" .
Field   = ( Name | ( Word | String ) ":" Name ) [ "=" Expr ] .
ListPat = "[" [ Elem { "," Elem } ] [ "," "..." Name ] [ "," ] "]" .
Elem    = ( Name | "_" ) [ "=" Expr ] .

List    = "[" [ Item { "," Item } [ "," ] ] "]" .
Item    = "..." Expr | Expr .
Map     = "{" [ Entry { "," Entry } [ "," ] ] "}" .
Entry   = "..." Expr | Key ":" Expr | Name .
Key     = Word | String | "[" Expr "]" .

Match   = "match" Expr "{" Case { "," Case } [ "," ] "}"   (* by cases *)
        | "match" "{" When { "," When } [ "," ] "}" .      (* by conditions *)
Case    = ( Lit { "|" Lit } | "_" ) "=>" Expr .
When    = ( Expr | "_" ) "=>" Expr .
Lit     = [ "-" ] Number | String | "true" | "false" | "null" .
```

Rules the grammar does not show:

- A `match` subject that starts with `{`, with `if` or with `try` must be in
  parentheses.
- An `if` is a whole expression and not an operand. Nothing may follow its last
  branch that could not follow any `Expr`. As an operand or as the receiver of a
  chain it is written in parentheses: `(if (c) { 1 } else { 2 }) + 3`.
- **A `try` stands where an `if` stands** (10-1). It is a whole expression and
  not an operand, and nothing may follow its handler that could not follow any
  `Expr`: `(try { int(s) } catch { 0 }) > 10`. Before D48 `Try` was a call form
  in `Primary`. `try(` is now a targeted error (3.2).
- Directly after `if (c)`, `else`, `try`, `catch` and `catch (err)`, a `{` opens
  a `Branch`, whatever follows it. A branch is written like a map when its
  expression is followed by `:` or `,`, or when it starts with `...`. That is an
  error, and so is an empty branch. A map as a branch's value is written inside
  the branch: `{ {id: 1} }`.
- The `else` of an `if` is required (D37). The `catch` of a `try` is required,
  and a `try` has one. The parentheses after `catch` hold one `Name` and not a
  pattern. With no name, the parentheses are left out too.
- **A star.** `[*]` and `?.[*]` are steps of a chain (2.8). No expression starts
  with `*`, so a `*` directly after `[` or `?.[` is never a multiplication, and
  the `]` must follow it. White space may stand between the three tokens.
- **`String` and `Template` in this grammar are tokens.** `String` is a one-line
  string, raw or not. `Template` is a backtick template or a text block, raw or
  not. So a text block cannot be a `Key`, a `Field` key or a `Lit`, and a
  one-line raw string can (08-9). Which node a literal becomes is a separate
  rule (2.3).
- A lambda with a pattern other than a name as a `const` target is an error. A
  `MapPat` or `ListPat` with only a rest name needs no leading comma.
- The grammar puts a lambda in two places: an argument, and the value of a
  `const`. Which calls may take a lambda, and which names may be called, are
  rules about what a name is bound to (2.9). The checker reports them, not the
  parser.
- `-` or `!` may not be applied directly to a `**` expression: `-2 ** 2` is an
  error, as in JavaScript. Write `(-2) ** 2` or `-(2 ** 2)`.
- `??` cannot be mixed with `||`, `&&`, a comparison, `in` or `not in` without
  parentheses. The grammar gives this: a `Coal` is not an operand of those
  operators, and their expressions are not operands of `??`.
- The left operand of `in` or `not in` may not be a `!` expression written
  without parentheses: `!a in b` is an error. It would parse as `(!a) in b`,
  which with truthiness is a valid test of the wrong thing. Write `a not in b`
  or `(!a) in b`. The same holds for `as` and `is`: `!x is T` is an error, and
  the author writes `!(x is T)`.
- **The right operand of `as` and `is` is a type, not an expression.** Its
  grammar is `Type` in `03` 2.2, without the four forms that only a signature
  has: `Sig`, `Fn`, `Var` and `unknown`. So a type name has two characters or
  more. One upper-case letter alone is a `Var`, and is an error here.
- **Where the type ends.** The parser reads the longest type. After a complete
  alternative, two tokens continue it and no other: `|`, which must be followed
  by another alternative, and `[`, which must be followed by `]`. A `<` belongs
  to a type only directly after the word `map`. The type ends at the first token
  that does not continue it. So a `?` always ends it, because no type has a `?`
  suffix (D29), and `<` and `>` after a complete type are comparisons.
- **What may follow the type** is what may follow any operand of `Rel`: another
  operator of that level (`as` and `is` among them), `==`, `!=`, `&&`, `||`, the
  `?` of a conditional, or a token that closes the expression around it. An
  arithmetic operator, `**`, `.`, `?.`, `(`, or a `[` that is not `[]` cannot
  follow, and `??` cannot by the rule above. Each is an error whose message says
  to put the `as` in parentheses. 2.17 has the cases.
- **Wherever a type is expected, a `{` starts a record type,** never a map or a
  block: directly after `as` or `is`, and inside a type after `|`, `(`, `<` and
  a field's `:`. After a complete type a `{` ends the type, so `match x as Kind
  { ... }` needs no parentheses.
- **Words in a type.** `null`, `bool`, `int`, `float`, `string`, `any`, `never`
  and `map` are type words only where a type is expected. Elsewhere `int` and
  `string` are the library's functions, and none of the eight is reserved. An
  upper-case word where a type is expected is a type name, whatever value the
  same word names: types and values are two sets of names. Any other word there
  is an error.
- **`>` and the lexer.** The lexer does not know that it is in a type. There is
  no `>>` token, so `map<map<int>>` ends with two `>`. The characters `>=` are
  always the token `>=`, so `x is map<int>=> 1` is an error and needs a space
  before `=>`.
- A program may not open more than 100 levels of nesting at once (2.16).

**Lexical rules.**

| Element | Rule |
| --- | --- |
| Source | A sequence of Unicode scalar values. A U+FEFF at the start is ignored. |
| White space | Space, tab, line feed and carriage return, anywhere between tokens. A newline has no other meaning. Any other character outside a string, template or comment that is not part of a token is an error. |
| Comments | `//` to the end of the line, and `/* */` without nesting. |
| Word | `[A-Za-z_][A-Za-z0-9_]*`. ASCII only; other field names are written `x["café"]`. |
| Reserved | `true false null const if else in not as is try catch match`, and, for targeted errors (3.2), `nil let var for while do break continue return function throw finally import`. `of` and `r` are not reserved. A `Name` is a `Word` that is not reserved and is not `_`. |
| `_` | A pattern that binds nothing, and the default arm of `match`. It cannot be read. |
| Number | `Int [ "." Digits ] [ ("e" \| "E") [ "+" \| "-" ] Digits ]` or `"0x" HexDigits`, where `Int` is `0` or a digit run with no leading zero. A single `_` may stand between two digits. `.5`, `5.` and `010` are errors. |
| String | `"..."` or `'...'`, with the same meaning, each with an optional prefix `r`. One line: it may not contain a raw line feed or carriage return. |
| Escapes | `\"` `\'` `\\` `\/` `\b` `\f` `\n` `\r` `\t`, `\uXXXX`, `\u{X...}` (1 to 6 hex digits). A `\uXXXX` in the high-surrogate range must be followed by one in the low-surrogate range, and the pair is one code point; any other surrogate escape is an error. Any other escape is an error. A raw literal has no escapes. |
| Template | `` `...` ``, or a text block, `"""` ... `"""`, which may have the prefix `r`. With `${ Expr }` parts, but for a raw text block. Escapes as for strings, plus `` \` `` and `\$`. May span lines; a raw carriage return, alone or before a line feed, is read as a line feed. An interpolation may contain any expression, including another template or text block. A `\` directly before a line end is an error: it does not join lines (08-6). |
| Longest match | `"""` is always the delimiter of a text block. `""` is the empty string only when no `"` follows it directly. The same holds for `r"""` against `r""`. |
| Raw prefix | The lexer reads a `Word`. When the word is exactly `r` and the next character is `"` or `'`, the two begin a raw literal. `r` followed by anything else is a name, as before. `xr"a"` is the word `xr` and then a string, which is an error. |
| Span of a literal | From its first character, the `r` if there is one, to the end of its closing delimiter. A part's span is its place in the source text: removing indentation moves no position. |

**String forms (D46).** `08-strings.md` is the authority. It has the reasons, a
table of worked values and errors, and what the forms mean in a host string and
in YAML. These are the rules of the tokens.

| Form | Lines | Escapes | `${ }` | Indentation |
| --- | --- | --- | --- | --- |
| `"..."`, `'...'` | one | yes | no | nothing to remove |
| `` `...` `` | one or more | yes | yes | kept: every character between the backticks is the value |
| `"""` ... `"""` | any number, from the line after the opening `"""` | yes | yes | removed: what stands before the closing `"""` |
| `r"..."`, `r'...'` | one | no | no | nothing to remove |
| `r"""` ... `"""` | as a text block | no | no | removed, by the same rule |

An author asks two questions. One line or several? Values and escapes, or
neither? The three forms that existed before D46 do not change.

| The text block | Rule |
| --- | --- |
| Opening | `"""`, then only spaces and tabs to the end of its line. They are ignored. Anything else there is the error `text-block-open`. |
| Line ends | A line ends at U+000A, at U+000D U+000A, or at a U+000D alone, and each is read as one U+000A, as in a template. So the value does not depend on how the file's lines end. No other character ends a line. Positions keep their own rule (`05` A.6). |
| Content | The lines after the opening line, up to the line of the closing delimiter. A **content line** is one that begins in text. A line that begins inside an interpolation is program text and is not a content line. |
| Closing | The first `"""` that stands in the text of the content: not inside an interpolation, and with none of its quotes escaped. Only spaces and tabs may stand before it on its line. Anything else there is `text-block-close`. Anything may follow it: `;`, `,`, `.trim()`. |
| Indentation | The spaces and tabs before the closing delimiter, as a sequence of characters. Every content line that is not blank must start with that sequence, character for character, and it is removed. A tab is not a number of spaces. A line that does not start with it is `text-block-indent`, reported once for each such line (08-1, 08-3). |
| Blank line | A content line with only spaces and tabs, or with nothing. It is an empty line, whatever white space it has, and it needs no indentation. White space after the indentation, and at the end of a line that is not blank, is content (08-2). |
| Last line end | Each content line ends with a line end. That of the last content line is not part of the value. So a blank line before the closing delimiter gives a value that ends with a line feed, and with no content line the value is the empty string. |
| Escapes and `${ }` | Read after the removal, as in a template. So an escape is never indentation and never a line end: `\n` adds a line feed that no rule above sees. `"` and `""` are content. Three in a row close the block, so one is escaped: `\"""`. |
| An interpolated value | Inserted as 2.13 renders it. Its later lines get no indentation: what is removed is fixed by the text, never by a value (08-7). |
| Where it may stand | Where a template may: as an expression. It is not a map key, a pattern key, a `match` case or a literal in type text. Those stay on one line (08-9). |

| The raw prefix | Rule |
| --- | --- |
| Form | A lower-case `r` directly before the opening quote of `"..."`, `'...'` or `"""`, with no white space between (08-4). Never before a backtick: `` r`...` `` is an error, because in TypeScript it calls `r` as a tag (**ran**, Node 22), so here it cannot mean raw text (Y1). |
| Content | Every character up to the closing delimiter is the value. `\` is a backslash, and `${` is two characters. |
| End | `r"..."` ends at the next `"`, `r'...'` at the next `'`, and a raw text block at the next `"""`. Nothing escapes a delimiter, so `r"\"` is a complete string of one backslash. |
| What it cannot hold | Its own delimiter, and a carriage return. A one-line raw string holds no line end. Such text is written in a plain form with an escape, or the piece is interpolated: `${'"""'}` (08-5). |
| A raw text block | Follows the table above for its delimiters, line ends, indentation, blank lines and last line end. |
| Where it may stand | A one-line raw string is a `String` and may stand wherever a string may. A raw text block may stand where a text block may. |

- After `text-block-open` or `text-block-close` the lexer reads on to the end of
  the literal and reports nothing more for it. A text block with no closing
  delimiter is `unterminated-string`. The node of a literal with an error is
  `Bad` (2.3). 3.2 has every code.
- **No text changes meaning.** `"""` and `r"` are syntax errors in TypeScript,
  and were errors here: in this grammar no `String` directly follows a `String`
  or a `Name`. The text of a template, of a comment and of a host string outside
  its segments is not lexed, so `"""` and `r"` there are characters as before
  (`08` 2.6).
- **No diagnostic for `${` in a plain string** (08-10). `"cost: ${n}"` is valid
  TypeScript with the same value, and such text is often meant.
- A formatter writes a literal back from its source text and never changes one
  form into another. In a text block it may replace the indentation, which is
  why a text block is the form for text of several lines (`08` 2.5).

### 2.3 Node kinds

This is the complete abstract syntax. `03-types.md` and `04-runtime.md` cover
exactly these.

| # | Node | Fields | Written |
| --- | --- | --- | --- |
| 1 | `Program` | `consts`, `result` | the whole source |
| 2 | `Const` | `pattern`, `value` | `const p = e;` |
| 3 | `Block` | `consts`, `result` | `{ const ...; e }`, and a branch of an `if` that has `const` lines |
| 4 | `Ident` | `name` | `x` |
| 5 | `Number` | `value` | `42`, `1.5e3`, `0x1F` |
| 6 | `String` | `value`, `form` | `"a"`, `'a'`, `r"a\b"`, and a backtick template or a text block with no `${ }` |
| 7 | `Template` | `parts` (text or expression), `form` | `` `a ${e}` ``, and a text block with a `${ }` |
| 8 | `Bool` | `value` | `true` |
| 9 | `Null` | | `null` |
| 10 | `List` | `items` (expression or `Spread`) | `[a, ...b]` |
| 11 | `Map` | `entries` (`Entry` or `Spread`) | `{a: 1, ...m}` |
| 12 | `Entry` | `key` (word, string or expression), `value`, `shorthand` | `a: 1`, `"a-b": 1`, `[k]: 1`, `a` |
| 13 | `Spread` | `value` | `...e` |
| 14 | `Unary` | `op` (`!` `-`), `x` | `!a` |
| 15 | `Binary` | `op`, `x`, `y` | `\|\| && == != < <= > >= in not-in ?? + - * / % **` |
| 16 | `Cond` | `form` (`?:` or `if`), `test`, `then`, `else` | `c ? a : b`, `if (c) { a } else { b }` |
| 17 | `Member` | `x`, `name`, `optional` | `x.f`, `x?.f` |
| 18 | `Index` | `x`, `index`, `optional` | `x[i]`, `x?.[i]` |
| 19 | `Call` | `namespace` (or none), `name`, `args` | `f(a)`, `math.sqrt(a)` |
| 20 | `MethodCall` | `x`, `name`, `args`, `optional` | `x.m(a)`, `x?.m(a)` |
| 21 | `OptChain` | `x` | wraps a `Chain` that has a `?.`, a `?.[` or a `?.[*]` outside a projection's reads (2.8) |
| 22 | `Lambda` | `params` (`Param`), `body` | `(x, i) => e` |
| 23 | `Param` | `pattern`, `default` | `x`, `{a, b}`, `n = 2` |
| 24 | `Match` | `subject` (or none), `arms` | `match x { ... }` |
| 25 | `Arm` | `literals`, or `test`, or default; `result` | `"a" \| "b" => e`, `n > 3 => e`, `_ => e` |
| 26 | `Try` | `body`, `name` (or none), `handler` | `try { e } catch (err) { d }`, `try { e } catch { d }` |
| 27 | `NamePat` | `name` (or discard) | `x`, `_` |
| 28 | `MapPat` | `fields` (`key`, `name`, `default`), `rest` | `{a, b: x, c = 0, ...r}` |
| 29 | `ListPat` | `elems` (`NamePat`, `default`), `rest` | `[a, _, c = 0, ...r]` |
| 30 | `Bad` | | a span that did not parse |
| 31 | `As` | `form` (`as` or `is`), `x`, `type` | `x as T`, `x is T` |
| 32 | `Each` | `x`, `reads`, `optional` | `x[*].f`, `x?.[*].f` |

A parser produces `MethodCall` for `math.sqrt(a)`; name resolution turns it into
a `Call` when `math` is a namespace.

**There are 32 kinds: `if` shares node 16 with `? :`, `As` is node 31 (D39), and
`Each` is node 32 (D51).** `if` and `? :` differ in spelling and in nothing
else: the test, the value, the type, the narrowing and the cost are the same
(2.10). So `if (c) { a } else { b }` is a `Cond` whose `form` is `if`, and an
`else if` is a `Cond` of that form in the `else` field. A branch with `const`
lines is a `Block`. A branch with none is its expression. The parentheses and
the braces of an `if` have no node, like parentheses anywhere, and the parser
counts their levels on the text (2.16). `form` is there for a formatter and for
messages. Neither the checker nor an evaluator reads it, so neither has a rule
that is particular to `if`.

`x as T` and `x is T` are one node, `As`, told apart by `form`, because they
share the operand, the type and the test (2.17). It has the last number so that
no other number moves. Its `type` is type text, held in the form `03` 2.2
defines. A type is not an expression and its parts are not among these kinds: no
walk of 2.3 enters it, and nothing in it is evaluated.

**The form of a literal is spelling (D46, 08-8).** A literal with at least one
`${ }` is a `Template`. Every other literal is a `String`, in every form: the
backtick and the text block included, which earlier revisions left unsaid. A raw
literal is always a `String`. Nodes 6 and 7 each have a field `form`: the
delimiter (`"`, `'`, `` ` `` or `"""`) and whether `r` was written. Like the
`form` of `Cond` and `As`, it is for a formatter and for messages, and neither
the checker nor an evaluator reads it. So no string form has a rule, a type or a
cost of its own: a `String` is a literal at 0 steps and 0 units, and a
`Template` costs what its parts cost (2.14; `04` A.3, A.4).

**`Try` keeps number 26, with new fields (D48).** Before, they were `x` and
`fallback`. `body` and `handler` are each what a branch of an `if` is: a `Block`
when the branch has `const` lines, and its expression when it has none. `name`
is the name in `catch (err)`, or none for `catch { ... }`. The braces and the
parentheses have no node.

**`Each` is one projection (D51; 2.8).** `x` is the list. `optional` says the
star was written `?.[*]`. `reads` is the list of the steps that follow the star
directly, in order. A step is a `Member` or an `Index` with no `x` of its own,
because it is applied to the element. The last step may be a further `Each` with
no `x`, for a second star, and it holds the steps after it. So
`orders[*].lines[*].sku` is an `Each` whose reads are `.lines` and an `Each`
whose one read is `.sku`. An optional read among the reads needs no `OptChain`:
for one element the reads are a chain of their own, and the `Each` is where a
skip ends (2.8). `Each` has the last number so that no other number moves.

Two nodes have new names. Node 2 was `Let` before D36, and its field in nodes 1
and 3 was `lets`. Node 9 was `Nil` before D28.

### 2.4 Values

There are seven kinds of value.

| Kind | What it is |
| --- | --- |
| `null` | The one absent value. It has one spelling, `null` (D28). `nil` is a compile error whose message names `null` (3.2). |
| `bool` | `true`, `false`. |
| `number` | 2.5. |
| `string` | A finite sequence of Unicode scalar values (2.6). |
| `list` | A finite sequence of values. |
| `map` | A finite set of entries, each a string key and a value. Keys are unique. A map has no order of its own; wherever order shows (iteration, rendering), it is by key, comparing code points. |
| opaque | A value the host supplies that is none of the above. A program can bind it, pass it to a function, put it in a list or map, compare it with null, and test it with `is` against the name of its type (2.17). It has no fields, cannot be indexed, ordered or rendered as text, and two opaque values are equal only when they are the same host value, by an identity test the binding supplies (overview S11). A host gives such values behaviour by declaring methods on them. |

- A function is not a value (2.9).
- An error is not a kind of value. What `catch (err)` binds is a map (2.12).
- No value can be changed, and no value has an identity: two lists with equal
  elements cannot be told apart.
- How a host presents its own data as these kinds (which of its types are maps,
  which names their fields have) belongs to the host API.

### 2.5 Numbers

A number is a finite IEEE 754 binary64 value. There is one zero: an operation
whose IEEE result is negative zero gives zero. There is no NaN and no infinity
(N1, N3).

A **whole number** is a number with no fractional part and magnitude at most
2^53 − 1 (9007199254740991). An index, a count and an integer parameter require
a whole number. Nothing else distinguishes `2` from `2.0`: they are one value.

| Operation | Whole operands | Otherwise |
| --- | --- | --- |
| `a + b`, `a - b`, `a * b`, `-a` | The exact integer result. If it is not a whole number, an error. | The IEEE operation, round to nearest, ties to even. |
| `a / b` | The IEEE quotient: `7 / 2` is `3.5`, `6 / 3` is `2`. | The same. |
| `a % b` | `a − b × trunc(a / b)`, computed exactly. The sign is the dividend's: `-7 % 3` is `-1`, `5.5 % 2` is `1.5`. | The same. |
| `a ** b`, `b` a whole number ≥ 0 | Start with 1; for each bit of `b` from the highest, square, and multiply by `a` if the bit is set. With whole `a`, each step follows the first row, so the result is exact or an error. | The same steps with IEEE multiplication. |
| `a ** b`, `b` a whole number < 0 | `1 / p`, where `p` is `a ** -b` by the steps above with IEEE multiplication only. `0 ** -1` is a zero divisor. | The same. |
| `a ** b`, `b` not whole | An error: an exponent is a whole number, as an index is (`wrong-kind`, 2.12). Roots and fractional powers are library functions. | |
| `==`, `!=`, `<`, `<=`, `>`, `>=` | Comparison of the two values. `0.1 + 0.2 == 0.3` is false. | The same. |

- A zero divisor in `/` or `%` is an error, `divide-by-zero`. Any result that
  would be NaN or infinite is an error, `number-range`, as is whole-number
  arithmetic that leaves the whole range (2.12). A result too small to represent
  is zero.
- **Literals.** A literal with no fraction and no exponent, decimal or hex, must
  be a whole number: `9007199254740993` is a compile error, not a rounded value.
  The code is `literal-range`, of phase `parse` (3.2).
  A literal with a fraction or an exponent is the nearest binary64 value (ties
  to even) and must be finite: `1e20` and `9007199254740993.0` are valid, and
  `1e400` is an error. A literal's static type comes from its value, not its
  spelling: `1e3` and `2.0` are whole (N5).
- **Truncation** is a library function (`int(x)`, toward zero), as are floor,
  ceiling and rounding. Their result must be a whole number or the call fails.
  For whole `a` and `b`, `int(a / b)` is the exact integer quotient ((I); no
  counterexample in four million adversarial pairs, **ran**).
- A host number that is NaN, infinite, or an integer outside the whole range
  cannot enter the language as a number (N2).

**Number to text** is ECMAScript's `Number::toString` with radix 10 (N7). Take
the shortest digit string `d1…dk` and the exponent `n` such that `0.d1…dk ×
10^n` reads back as the number. If several strings of that length do, take the
closest, then the one with an even last digit. A negative number has `-` in
front. Then:

| Case | Text | Examples (**ran**, Node 22) |
| --- | --- | --- |
| `k ≤ n ≤ 21` | the digits, then `n − k` zeros | `100`, `123456789012345680000` |
| `0 < n < k` | the digits with a point after the first `n` | `123456.789`, `4.35` |
| `−6 < n ≤ 0` | `0.`, then `−n` zeros, then the digits | `0.5`, `0.000001`, `0.30000000000000004` |
| otherwise | `d1`; if `k > 1`, a point and the other digits; `e`; `+` or `-`; the magnitude of `n − 1` | `1e+21`, `1.5e-7`, `5e-324` |

**Text to number** accepts exactly the JSON number grammar (an optional `-`, no
`+`, no leading zeros, no hex, no separators, no white space) and gives the
nearest binary64 value, ties to even. Text that reads as an infinity is an
error, and `-0` reads as zero. Reading back the text of a number gives that
number. Library conversions are built on this rule and say what they add
(trimming, requiring a whole number).

*Why the whole-number rule is stated on operands.* N2 says an integer result
outside the range is an error. Stated on results alone, `1e20 * 1.5` would be an
error, which makes large measurements unusable. Stated as above, it depends only
on values, not on how an implementation stores them, and it costs a JavaScript
implementation one `Number.isSafeInteger` test per operand and result. The price
is one visible seam: `10 ** 20` is an error and `1e20` is not (open question 1).

*Why `**` takes whole exponents.* A general power function is not correctly
rounded in every runtime, so `2 ** 0.5` could differ in its last digit between
implementations. Squaring and multiplying uses only operations IEEE defines
exactly. The uses are `2 ** attempt`, `10 ** digits` and `x ** 2`.

### 2.6 Text

- A string's length, positions and order are in code points (T1). There is no
  byte view and no UTF-16 view. Strings are equal when their code point
  sequences are equal; nothing is normalized.
- `s < t` compares code point by code point; a proper prefix is smaller.
- `s + t` joins two strings. `+` on a string and anything else is an error.
- `s[i]` is the one-code-point string at position `i`, a whole number from 0.
  Out of range is an error; `s?.[i]` gives null.
- Host text that is not a valid sequence of scalar values (an unpaired surrogate
  or a malformed byte sequence) reads as if each malformed unit, a byte of UTF-8
  or a unit of UTF-16, were U+FFFD.

### 2.7 Truthiness, null, equality, order and `in`

**Truthiness (B1, B5).** Any value may be a condition. A value is **falsy** if
it is null, `false`, zero, the empty string, the empty list or the empty map.
Every other value is **truthy**. The owner has confirmed this set (D30).

| Kind | Falsy | Truthy |
| --- | --- | --- |
| null | always | never |
| bool | `false` | `true` |
| number | zero. There is one zero (2.5), so `0`, `0.0`, `-0` and a negative zero the host supplies are all falsy | every other number, however small |
| string | the empty string | any string with a code point: `" "`, `"0"`, `"false"`. The content is never read |
| list | no elements, whether the program built it or the host supplied it | one element or more, whatever they are: `[null]`, `[0]`, `[[]]` |
| map | no entries: `{}`, a host map read in place that has no entries, a record with no field present | one entry or more, whatever the values: `{a: null}`. A record with a field is a map with an entry |
| opaque | never | always. The test does not ask the host |

- A record's entries are the fields that are present. An optional field that is
  absent is not an entry (B2); a field that holds null is.
- Whether a host's null collection arrives as null or as an empty list or map is
  the binding's rule (`05`). The test gives the same answer either way.
- **A truthiness test is bounded work on any value.** It reads the kind, and for
  a string, list or map it asks only whether there is at least one code point,
  element or entry. It never measures a string, visits an element, orders keys
  or runs host code. A binding must answer the question for a host list or map
  read in place without visiting its contents; for a record it is at most one
  presence test per optional field of the declared type.
- **The test adds no step.** A construct that tests a value charges what it
  charged before: nothing for `&&`, `||`, `? :` and `if`, one step for `!`, one
  step for each `match` arm tried (`04` A.3).

| Form | Rule |
| --- | --- |
| `!a` | `true` if `a` is falsy, else `false`. Always a `bool`. `!!a` is the truthiness of `a` as a `bool`. |
| `a && b` | Evaluates `a`. If it is falsy, the value is `a` and `b` is not evaluated. Otherwise the value is `b`. The value is an operand, not a `bool` (B6). |
| `a \|\| b` | Evaluates `a`. If it is truthy, the value is `a` and `b` is not evaluated. Otherwise the value is `b`. |
| `c ? a : b`, `if (c) { a } else { b }`, a condition arm of `match` | Chosen by the truthiness of the condition. Only the chosen branch is evaluated. |
| a predicate passed to a library method (`filter`, `find`, `some`, `every`, and the others `06` lists) | Its result is tested by truthiness. |
| `a ?? b` | `a` if it is not null, else `b`, which is evaluated only then. It does not catch errors. |
| `a == b`, `a != b` | Different kinds are unequal; nothing is converted, so `1 == true` and `"" == null` are false. Within a kind: numbers and booleans by value, strings by code points, lists by length and then element by element, maps by key set and then value by value. Two opaque values are equal only if they are the same host value, by the binding's identity test (overview S11). `==` never fails. `!=` is the negation. |
| `a < b` and the other three | Two numbers, or two strings. Anything else is an error, so `1 < 2 < 3` does not check. |
| `x in y` | `y` a list: some element `==` `x`. `y` a string: `x` is a string and occurs in `y` (the empty string always does). `y` a map: `x` is a string and a key of `y`. Any other `y`, including null, is an error. |
| `x not in y` | `!(x in y)`. |
| `x is T` | `true` if `x` conforms to the type `T`, else `false`. Always a `bool`. It never fails (2.17). |

**`||` or `??`.** Both stay, because they answer different questions.

| Written | `a` is null | `a` is `false`, `0`, `""`, `[]` or `{}` | `a` is anything else |
| --- | --- | --- | --- |
| `a \|\| d` | `d` | `d` | `a` |
| `a ?? d` | `d` | `a` | `a` |

So `user.name || "anonymous"` covers a missing name and an empty one, and
`input.retries ?? 3` keeps a `0` that the caller asked for.

*Note.* `a && b` gives back `a` when `a` is falsy, so `` `${failed && "see the
log"}` `` renders `[]` for an empty list and `0` for zero (2.13). It is the trap
JavaScript authors know from `count && ...` in a view. In a template, write
`failed ? "see the log" : ""`.

*Why this falsy set.* JavaScript agrees on null, `false`, zero and the empty
string. It differs on one point: there an empty list and an empty object are
truthy. Three reasons to make them falsy (B5, D30). An author who writes `when:
results.items` means "there are items". One sentence states the whole rule:
null, `false`, zero and anything empty. And the test gives the same answer
whether a host supplied a null collection or an empty one. A Go host passes a
nil slice and an empty slice interchangeably, so a rule that separates null from
empty would make a program's result depend on the host's language (C1). Python
and Go's templates use this set. So do Risor v2 and expr
(`v2:pkg/object/list.go:538`, `spec:216-230`), which is a convenience and not
the reason (C7). A string is tested for emptiness and never read, so `"0"` and
`"false"` are truthy, as in JavaScript. An opaque value is always truthy because
the alternative is an operator that runs host code, which is also why `==` on
opaque values is identity (overview S11).

*What a host does with a result (B8, D33).* Truthiness is a rule of the
operators, not of what a host accepts. A host has two choices. `03` and `05` A.3
define both.

| The host passes | What fits | What the host does with the value |
| --- | --- | --- |
| `bool` | A program whose type is `bool`. `verify`, a list, does not fit. Neither does `a.items && len(a.items) > 0`, which is a list or a `bool`. The author writes `len(verify) > 0`, `!!verify` or `bool(verify)` | reads the boolean. On data typed `any` it tests the kind of the result |
| `condition` | Any type. A result that can never be falsy, or never truthy, is reported as in any tested position (2.15) | tests it by truthiness, with the binding's test |

The workflow engine takes the second for `when:` (D33). Its documentation today
says that `when: verify` on a list "is an error"
(`wf:documentation/definitions.md:211-212`), and explains at length that
`a.items && len(a.items) > 0` is not a boolean (`:779-783`). Under D33 both are
accepted, and each means what it means before a `?`.

### 2.8 Fields, indexes, calls and chains

| Form | Meaning | When it is not there |
| --- | --- | --- |
| `x.name` | The value under key `name` of a map | error |
| `x["a-b"]`, `x[k]` | The same, for any string key | error |
| `xs[i]`, `s[i]` | Element or code point `i`, counting from 0 | error, including a negative or fractional `i` |
| `x?.name`, `x?.[k]`, `x?.[i]` | The same reads | null when the key is absent or the index is out of range |
| `xs[*]`, `xs?.[*]` | A projection: the reads that follow apply to each element of the list `xs` (below) | error when `xs` is not a list. With `?.[*]`, a null `xs` makes the chain null |
| `f(a, b)` | Call the function named `f` | compile error if `f` is unknown, or is a local name that is not a `const`-bound lambda (2.9) |
| `x.m(a)`, `x?.m(a)` | Call method `m` on `x` | compile error if there is no such method |

- Where the third column says "error" for a read, the run code is in 2.12:
  `missing-field` for a key, `index-range` for an index, `null-operand` and
  `wrong-kind` for a receiver that is null or of another kind. With types known,
  the checker reports most of these first (2.15).
- **`.` reads a field and never a method.** Any `Word` may follow `.` and `?.`
  and may be a bare map key: `error.type`, `tool.function.name`, `{if: 1}`.
- **Wrong kind is always an error**, with or without `?`: `5?.a` fails. `?.`
  forgives "not there", not a mistake.
- **A method** is found by the kind of `x`: a library method of that kind, or
  one the host declared for it. A method name never reads the map's data.
- **A call target is a name**, or a name in a namespace (S1). `f(x)(y)` and `(x
  => x)(1)` do not parse, so a host can find every call of a function by walking
  the tree. Which names may be called is in 2.9. Arguments are positional and
  evaluated left to right. There are no spread or named arguments.

**Chains (B4).** A chain is one `Chain` production: a primary and every `.name`,
`?.name`, `[e]`, `?.[e]`, star and call that follows it directly.

1. The steps of a chain are evaluated left to right.
2. If the receiver of a `?.`, `?.[` or `?.[*]` step is null, the value of the
   **whole chain** is null. No later step runs, and no index expression or
   argument in a later step is evaluated.
3. If the receiver is not null, the step is the read in the table above. A read
   that finds nothing gives null **for that step**, and the chain goes on:
   `a?.b` with no `b` is null, and `a?.b.c` with no `b` then fails on `.c`.
4. A plain `.`, `[ ]` or method-call step on null is an error.
5. A chain ends where the production ends: at a closing parenthesis around it
   and at any operator. `(a?.b).c` is two chains, and fails when `a` is null.
   `-` and `!` in front, `**` and every binary operator behind, apply to the
   chain's value. An index expression and each argument are expressions of their
   own.

So `input.event?.pull_request.head.sha` is null when there is no event, and is
an error when there is an event with no `pull_request`.

**Projections (D51).** A **projection** is a star, `[*]` or `?.[*]`, and the
reads and stars that follow it directly. A **read** is `.name`, `?.name`, `[e]`
or `?.[e]`. `orders[*].lines[*].sku` is one projection. `13-queries.md` has the
workload it was chosen for, its types and its cost (`13` 5.2).

| Matter | Rule |
| --- | --- |
| The receiver | Must be a list. Null is an error with `[*]`, as for any plain step, and with `?.[*]` rule 2 applies. Any other kind is an error, a string and a map among them. Nothing is treated as a list of one. For the values of a map, write `m.values()[*]` |
| What a star covers | The reads and stars after it, up to the first method call or the end of the chain. They are applied to each element in turn, in order. **A method call ends the projection and receives the list** (13-2). So `orders[*].lines[*].qty.sum()` sums one list and needs no parentheses. `users[*].email.toLowerCase()` calls a method that a list does not have, and the author writes `users.map(u => u.email.toLowerCase())` |
| The value | Always one new list: what the reads gave for each element, in order. With one star its length is the length of the receiver. An empty list gives an empty list, and a list of one gives a list of one. A star with no read after it gives a list of the same elements |
| A second star | Iterates the list that the reads before it gave for one element, and its results go into the same list. So a projection is nested passes with one flat result. A list that no star iterated is never flattened: `orders[*].lines` is a list of lists |
| A read that finds nothing | An error with `.name` and `[e]`, as everywhere. With `?.name` and `?.[e]` that element gives null, and the null is kept |
| `?.` inside a projection | For one element, the reads after the star are a chain of their own, and rules 2 to 4 apply to it. When the receiver of a `?.`, a `?.[` or an inner `?.[*]` is null, the rest of the projection's reads are skipped for that element, which gives one null in the list. The other elements go on. Nothing is dropped: `orders[*].lines?.[*].sku` has one null for an order whose `lines` is null |
| An index read | `orders[*].lines[0]` is the first line of each order. An index expression is evaluated again for each element |
| A failure | The first failure in evaluation order ends the projection, as it ends any expression. No partial list is given |

- A star gives no index and binds no name. A step that needs either is `map`
  with a lambda: `xs.map((x, i) => ...)`.
- There is no filter step. `xs[c]` is an index, as in TypeScript, and `xs[?c]`
  is a targeted error that names `filter` (3.2).
- `xs[*].f` says what `xs.map(x => x.f)` says. `map` stays: it is the general
  form and the one a TypeScript author writes. The star is the short form for a
  body that is only a path.

*Why a mark.* Where other languages map without one, the shape of a result
depends on its length, or a mistake becomes null or nothing (`13` section 3).
Here the star says where the mapping is. The result is always a list. A mistake
is an error, or a null where the author wrote `?.`. And `x[*]` is a syntax error
in TypeScript (**ran**, Node 22), so no text changes meaning (Y1).

### 2.9 Lambdas and patterns

- A lambda is `x => e`, `(x, i) => e` or `() => e`. The body is one expression,
  which may be a block. It reads the names in scope where it is written.
- **A lambda is written in two places, and nowhere else** (E6): as the value of
  a `const` whose pattern is a name, or as an argument of a library function or
  method whose signature takes a function there (`06`).
- **The name of a `const`-bound lambda is used in two ways, and no other.** It
  is called, as in `double(4)`. Or it is passed to a library function or method,
  as in `xs.map(double)`, which means what the lambda written there would mean.
- **A lambda is passed only to the library (D34).** Passing one to a
  `const`-bound lambda is an error, whether the lambda is written in place or
  named. Passing one to a host function or a host method is an error too (S3).
- **Only a function can be called (D34).** In `f(a)`, `f` refers to the
  innermost binding of that name. If that is a `const`-bound lambda, the call
  runs it. If nothing binds the name, `f` is a function of the environment: the
  library's or the host's. Every other binding is an error. A name bound by a
  parameter or by a pattern, a `const` that holds a value and a variable of the
  environment are values, and a value cannot be called, whatever it holds.
- A function is not a value. A lambda, and the name of one, cannot be stored in
  a list or a map, compared, given as a default, or be the result of a block, of
  a lambda or of the program.
- When a library method calls a lambda with more arguments than it declares, the
  extra ones are dropped. A call by name must supply every parameter that has no
  default, and no more than are declared.
- There is no `it` and no implicit `index`.

| Written | Allowed |
| --- | --- |
| `const double = x => x * 2;` and then `double(4)` | Yes: a call by name |
| `xs.map(x => x * 2)`, `xs.map(double)` | Yes: a lambda, written or named, passed to a library method |
| `const total = (xs, rate) => xs.map(x => x * rate).reduce((a, b) => a + b, 0);` | Yes: the helper takes data, and passes its lambdas to the library |
| `const twice = (f, x) => f(f(x));` | No: `f` is a parameter, and a parameter cannot be called |
| `const each = (xs, g) => xs.map(g);` | No: `g` is a parameter, so it is a value and not a function |
| `const apply = (x, n) => x * n; apply(y => y, 2)`, `apply(double, 2)` | No: a lambda passed to a `const`-bound lambda |
| `const {run} = input; run(1)` | No: `run` is a name from a pattern |

**The two errors of D34.** Each is a compile error with a code. `03` 2.13 has
both in its catalog, under these names. Both are of phase `check`, and both are
reported whether or not types are known.

| Code | Reported when | Span |
| --- | --- | --- |
| `call-of-parameter` | The name of a `Call` refers to a binding that is not a `const`-bound lambda: a parameter, a name in a pattern, a `const` whose value is not a lambda, or a variable of the environment | the name |
| `lambda-to-lambda` | An argument of a call of a `const`-bound lambda is a lambda, or is the name of a `const`-bound lambda | the argument |

- Both are found from bindings alone. Neither needs a type, so each is reported
  where it is written: `const twice = (f, x) => f(f(x));` is an error even if
  `twice` is never called, and it is an error on data typed `any`.
- Neither belongs to a check class. A host cannot turn either off.
- A name passed where a library signature takes a function must be the name of a
  `const`-bound lambda. Any other argument there is `arg-mismatch`. A lambda
  passed to a host function, or to a library parameter that takes no function,
  is `not-a-value` (`03` 2.11).

*Why the checker and not the parser.* Three reasons.

1. Both are rules about what a name is bound to. Names are resolved in one
   place, the checker (`03` 2.4), which already reports a function used as a
   value, a name bound twice and a call of an unknown function. The parser keeps
   no scope, and 2.16 counts it against a design that it would make the parser
   resolve names.
2. Half of the rule needs the environment. In `f(x => x)` with no local `f`,
   whether `f` is a library function, a host function or nothing at all is not
   in the text, and parsing takes no environment (`05` A.1). Put in the parser,
   the rule would be split across two phases, and the phase of the error for
   `f(x => x)` would depend on whether a `const` above it binds `f`.
3. Nothing is lost. No program runs without passing the check (`05` A.1), so the
   guarantee of 2.14 holds for every program that runs.

*Why the rule (E9).* The earlier revisions allowed a lambda as an argument of
any call, and a parameter could be called. Then `const om = x => x(y => x(y));
om(z => om(w => z(w)))` never returns, although no name is in scope in its own
value and every lambda is written in place (**ran**, as JavaScript: stack
overflow). Such a program ended at a limit and not by construction, which is not
what E6 says. The price is the user-written helper that takes a function:
`twice` above cannot be written. A helper that takes data and gives data is
unaffected, and the library's methods are the higher-order functions.

**Patterns** appear in `const` and in parameters.

| Pattern | Binds | Fails when |
| --- | --- | --- |
| `x` | the value | |
| `_` | nothing | |
| `{a, b: x}` | `a` to field `a`, `x` to field `b` | the value is not a map, or a field is absent |
| `{a, ...rest}` | also `rest`, a new map of the other entries | |
| `[a, _, c]` | the first and third elements | the value is not a list, or is too short |
| `[a, ...rest]` | also `rest`, a new list of the other elements (empty if none) | |

- **A default**, `name = e`, is used when the field or element is absent or its
  value is null, and when a parameter's argument is omitted or null. `e` is
  evaluated only then, and sees the names bound to its left.
- Patterns do not nest in 3.0, and a name is bound once per pattern. A string is
  not a list. A longer list than the pattern is not an error.
- The parentheses of `catch (err)` hold a name and not a pattern (2.12). The
  author writes `const {code} = err;` in the handler.

*Why not `it`.* It saves five characters in the flat case. It costs a second
call shape for nesting (`spec:498-538`) and two magic names, one of which,
`index`, is also a workflow variable (`wf:llms.txt:358`).

### 2.10 Conditionals

There are three. Each tests by truthiness where it tests a condition (2.7),
evaluates only the branch it takes, and has the join of its branches as its
type.

| Form | Use |
| --- | --- |
| `c ? a : b` | The short form, inside a larger expression. Both branches are required |
| `if (c) { a } else if (d) { b } else { e }` | A choice with room: a branch may bind names, and a chain reads from top to bottom |
| `match` | A choice by literal cases, checked for coverage. Its second shape, by conditions, says what an `else if` chain says (D38; question 11 of 8.1) |

**`if`, `else if` and `else` (D35).**

```js
if (gate.failed == 0) {
  "pass"
} else if (gate.failed <= 2) {
  "warn"
} else {
  const names = gate.results.filter(r => !r.ok).map(r => r.name);
  `fail: ${names.join(", ")}`
}
```

| Rule | Text |
| --- | --- |
| Shape | `if (c) { ... } else if (d) { ... } else { ... }`. The parentheses and the braces are required. |
| Branch | A block body: zero or more `const` lines, then one expression. A name bound in a branch is in scope to the end of that branch. |
| The brace | Directly after `if (c)` and after `else`, a `{` always opens a branch, never a map. A map as a branch's value is written inside the branch: `{ {id: 1} }`. A branch written like a map, `{ a: 1 }`, or left empty, `{}`, is a targeted error (3.2). Everywhere else the rule of 2.1 stands. |
| `else` | Required in 3.0 (D37). Nothing has an effect, so an `if` with no `else` could only mean "this value or null", and `c ? a : null` says that in the open. |
| Test | By truthiness, as for `? :`. A test that can only go one way is the checker's diagnostic (2.15). |
| Evaluation | The tests are evaluated from top to bottom. The branch of the first truthy test is taken. If no test is truthy, the `else` branch is taken. No other branch is evaluated, and no later test. |
| Value | The value of the branch taken, which is the value of its expression. |
| Place | An `if` is a whole expression, at the level of `? :`. It may stand wherever an expression may: a `const` value, an argument, a lambda body, a branch, a list item, a map value, an interpolation. It is not an operand. As an operand, a chain receiver or a `match` subject it needs parentheses: `(if (c) { 1 } else { 2 }) + 3`. So no reader has to ask whether `+ 3` or `.map(f)` belongs to the last branch or to the whole. |
| Cost and nesting | Steps as for `? :` (`04` A.3). The parentheses and each branch's braces open a level. An `else if` opens none (2.16). |

`if (c) { a } else { b }` and `c ? a : b` mean the same, and they are one node
(2.3).

**`match`.**

```js
match classify.kind {                 match {
  "bug"              => "fix",          gate.failed == 0 => "pass",
  "feature" | "idea" => "plan",         gate.failed <= 2 => "warn",
  "question"         => "answer",       _                => "fail",
}                                     }
```

| | By cases: `match subject { ... }` | By conditions: `match { ... }` |
| --- | --- | --- |
| Arm | one or more literals joined by `\|`, or `_` | any expression, tested by truthiness (2.7), or `_` |
| Evaluation | The subject is evaluated once. Arms are tried top to bottom; the first with a literal `==` to the subject is chosen. | Conditions are evaluated top to bottom; the first that is truthy is chosen. |
| `_` | Required unless the arms cover every value the subject's type allows. Allowed even then. | Required, and last. |
| Value | The chosen arm's result. No other result is evaluated. | The same. |
| Compile errors | A literal the subject can never equal; a literal listed twice; an arm after `_`; missing cases, named in the message. | An arm after `_`; no `_`. |
| Run time | If no arm matches and there is no `_` (untyped data that broke its type), the error `no-match` (2.12). | |

- **No binding and no guards.** A pattern is a literal, never a name, so a name
  in pattern position can never silently bind where a comparison was meant, the
  known trap of Python's and Rust's `match`. A test that is not "equals a
  literal" belongs in the second shape. To name the subject, use a block.
- I expect the checker to treat `null =>` as a null test, so the subject is
  known not to be null in later arms.
- **A `match` on the tag of a tagged union narrows.** With `s` a circle or a
  rectangle told apart by `kind`, `match s.kind { "circle" => s.r, "rect" =>
  s.w }` reads `s.r` only where `s` is a circle, and a third variant added to
  the type makes the `match` a compile error until it has an arm (D47; `09`
  2.5). No syntax is added for it: the arms are still literals.
- **Cases compare with `==`, never by truthiness.** `match n { 0 => "none", _ =>
  "some" }` tests for the number zero. `match xs { true => ... }` on a list is a
  compile error, because a list never equals `true`. To branch on truthiness,
  use the second shape: `match { xs => ..., _ => ... }`.

The same choice written four ways, with `kind` typed `"bug" | "feature" | "idea"
| "question"`:

| Form | Text | What goes wrong |
| --- | --- | --- |
| `match` | above | A fifth kind added to the schema is a compile error here. |
| Ternary chain | `k == "bug" ? "fix" : k == "feature" \|\| k == "idea" ? "plan" : "answer"` | The subject is repeated. A fifth kind silently gets `"answer"`. |
| `if` chain | `if (k == "bug") { "fix" } else if (k == "feature" \|\| k == "idea") { "plan" } else { "answer" }` | The same two faults as the ternary chain. It reads better, and it checks no more. |
| Map lookup | `{bug: "fix", feature: "plan", idea: "plan", question: "answer"}[k]` | Every result is evaluated. A fifth kind is a run-time error. The results cannot be expressions that only make sense for their case. |

*Why `if` is in (D35).* The first four revisions rejected `if (c) { a } else { b
}` as a third conditional, whose braces would need a second meaning beside "map"
and "block starting with `let`". The owner reversed that: a language with no
statements at all is more than C3 asked for (overview 2.10). `if` is what a
TypeScript author writes first (C6), a chain of `? :` reads badly past two arms
(C4), and blocks already exist. The objection about braces is met by position.
The second meaning holds only directly after `if (c)` and after `else`, and a
branch that looks like a map is an error, not a guess.

*Rejected.*

| Alternative | Why not |
| --- | --- |
| An `if` with no `else`, whose value is null when the test fails (D37) | It hides a null in a missing word, and `c ? a : null` says it in the open. It can be allowed later without a break. The reverse cannot. Question 10 of 8.1 |
| A branch brace that follows the rule of 2.1, so that `{ a: 1 }` after `else` is a map | Then `if (c) { x } else { y }` would give the maps `{x: x}` and `{y: y}`, which no author of an `if` means |
| An `if` as an operand without parentheses | `if (c) { 1 } else { 2 } + 3` has two readings, and a JavaScript author has no habit to settle it, because there an `if` has no value |
| `if c { a } else { b }`, or `if (c) a else b` | The first is not TypeScript's spelling. In the second a branch could hold no `const` line, and in `if (c) a else b + 1` the reader must know that `+ 1` belongs to `b` |
| A `switch` with `case` and `default` | JavaScript's is a statement with fall-through, so the familiar text would not have the familiar meaning |

### 2.11 Building values

| Want | Write |
| --- | --- |
| set or add a field | `{...user, role: "admin"}` (later entries win) |
| a key from data | `{[item.id]: item.status}`; the key must be a string |
| remove a field | `{ const {password, ...safe} = user; safe }` |
| append, prepend, concatenate | `[...xs, x]`, `[x, ...xs]`, `[...a, ...b]` |
| change or remove elements | `xs.map(x => {...x, seen: true})`, `xs.filter(x => x.id != id)` |

- In a map literal, a bare key is the string of that word, `{a, b}` means `{a:
  a, b: b}`, and a literal key written twice is a compile error. A computed key
  that repeats an earlier key replaces it.
- A list spread takes a list and a map spread takes a map. Spreading null or any
  other kind is an error; write `...(m ?? {})`.

### 2.12 Errors, `try` and `catch`

`10-errors.md` is the authority (D48). It has the type of the error value, what
a caught error costs, the trace, the message catalog, the three renderings and
the host function that reports every failure of a conformance test. This section
has what the language's surface owns: when a program fails, the form of `try`,
what `catch` catches, and the name of every run code that the language itself
raises.

| When | What |
| --- | --- |
| Compile, always | syntax, each mistake with a `parse` code (3.2); nesting deeper than 100 levels (2.16); unknown name, with a hint; a reserved construct, with the fix (3.2); a lambda in the wrong place; a call of a name that is not a function, and a lambda passed to a `const`-bound lambda (2.9); a name bound twice; wrong argument count; two record types in a union with no field that tells them apart (`untagged-union`; 2.17) |
| Compile, when types are known | unknown field on a closed type; a value that may be null used without `?.`, `??` or a test; wrong kind for an operator; a condition that can never be falsy, or never truthy, unless the host has taken that class out (2.15); a result or an argument that does not fit the type the host or the signature expects, reported at the part of a literal that is wrong (D47; 2.15); comparison or `match` literal that can never be equal; an `as` that no value can pass (`never-conforms`); `match` that is not exhaustive; unknown method |
| Run time, on untyped data | the same null, kind and field mistakes: `null-operand`, `wrong-kind` and `missing-field` |
| Run time, always possible | index out of range; missing key on an open map; a pattern that does not fit; zero divisor; number out of range; a failed conversion; a value that does not conform to the type after `as` (`not-conforming`; 2.17); a call of `fail`; a host function's error; a limit (steps, allocation, deadline, depth) |

**`try` and `catch` (10-1).**

```js
// A fallback for a bad value. The handler needs no name.
const n = try { int(input.count) } catch { 0 };

// The error as data.
const order = try {
  json.parse(msg.text) as Order
} catch (err) {
  match err.code {
    "not-conforming" => fail(`order: ${err.message}`, {path: err.path}),
    _                => null,
  }
};
```

| Rule | Text |
| --- | --- |
| Shape | `try { ... } catch (err) { ... }`, or `try { ... } catch { ... }`. The part after `try` is the **body** and the part after `catch` is the **handler**. One `catch`, required. There is no `finally`: nothing in a program has an effect to undo. |
| Branches | The body and the handler are branches, as in an `if` (2.10): zero or more `const` lines, then one expression. A name bound in the body is not in scope in the handler. |
| The brace | Directly after `try`, `catch` and `catch (err)`, a `{` opens a branch and never a map. A branch written like a map, or left empty, is the targeted error it is for an `if` (3.2). |
| The name | A `Name`, in scope in the handler only. It may shadow as a `const` name may, and may not shadow a namespace. It is not a pattern, and `_` is not a name: when the error is not used, the parentheses are left out. |
| Evaluation | The body is evaluated. If it gives a value, that is the value of the `try`, and the handler is not evaluated. If it fails with an error that `catch` catches, the handler is evaluated, with the name bound to the error value, and its value is the value of the `try`. Any other failure of the body, and any failure of the handler, is the failure of the `try`. |
| Place | A whole expression, at the level of `if` and `? :`. As an operand, a chain receiver or a `match` subject it needs parentheses: `(try { int(s) } catch { 0 }) > 10`. |
| Checking | `try` is for bad values and does not relax the checker: a static error in either part is reported as anywhere else. `?.` and `??` are for null. The type is the join of the two parts (2.15). |
| Cost and nesting | No step for a `try` whose body gives a value. A failure caught with no name costs nothing more. With a name, the error value is charged as written, whether or not the handler reads it (`10` 2.7). What the body charged before it failed stays charged. Each branch's braces and the parentheses of `catch (err)` open a level, and the two words open none (2.16). |

**The error value (10-2).** In the handler the name is bound to a map with
exactly four keys.

| Field | Holds |
| --- | --- |
| `code` | The code of the failure: a string, one of the run codes that `catch` catches |
| `message` | The diagnostic's message, as the catalog words it. For `failed` it is the first argument of `fail`: the same value, not a copy |
| `path` | For `not-conforming`, the path to the first failure in the tested value: a list of keys and indexes (2.17). Otherwise null |
| `data` | For `failed`, the second argument of `fail`. Otherwise null |

- **It is an ordinary value:** a map, and `type(err)` is `"map"`. It may be
  compared, spread, rendered, returned and passed to a host function. There are
  still seven kinds of value (2.4).
- **Its type is an exact record,** and `code` has the literal set of the codes
  that `catch` catches. So `err.code == "not-conformin"` is a compile error, and
  a `match` on `err.code` is checked for its cases. The set belongs to the
  specification version (`10` 2.3).
- **It has no class, no span and no trace.** The class of a failure depends on
  what the checker inferred, and a program must not see that. A position is for
  a person and for the host: a program that branches on one breaks when a line
  is added above it.

**What `catch` catches (10-3).** It decides by the code of the failure, and by
nothing else.

| Passes through | Codes | Why |
| --- | --- | --- |
| A limit, a cancellation | `steps-exceeded`, `allocation-exceeded`, `deadline-exceeded`, `depth-exceeded`, `cancelled` | A handler would run on a budget that is spent, or after the host said stop |
| A crash | `function-panic`, and a failure inside the implementation | The state of the host function is not known (`04` A.1) |
| A variable the host did not supply | `missing-variable` | No value caused it, and nothing in the program can cure it. A handler would hide a fault in how the host is wired, on every run |

Every other run error is caught: a failed `as`, a failed conversion, `fail`, an
index out of range, a zero divisor, a number out of range, an error that a host
function returned, and a null, a wrong kind or a missing field on data. Earlier
revisions of this section did not list a crash among what passes. `04` A.1 did,
and it governs.

**`fail` (10-4).** To raise an error, a program calls the library function
`fail(message, data?)`, typed `never` (`06`). The diagnostic has the code
`failed`, at the span of the call. Its message is the first argument, whole, and
its `data` is the second, or null. A caught error cannot be raised again
unchanged in 3.0: `fail(err.message, err)` raises a new `failed` error that
carries the old one as data. There is no `throw`. The word is reserved, and it
is a targeted error that names `fail` (3.2).

**The run codes of the language.** Every run failure has a code. These are the
codes of the failures that sections 2.5 to 2.17 define. Six of the names are
new, and they are this document's. A run code is never also a `check` code:
`maybe-null` says that a value may be null, and `null-operand` says that one
was.

| Code | Raised when | Kind (`03` 2.14) |
| --- | --- | --- |
| `null-operand` | Null stands where an operation needs a value: the receiver of a plain `.`, `[ ]`, `[*]` or method call (2.8); an operand of arithmetic, of `<` and the other three, or of `+`; the right operand of `in`; a spread; a computed key; the value that a map or list pattern is matched against | `null` |
| `wrong-kind` | A value that is not null is of the wrong kind for the same uses: `5.a`, `"n=" + 5`, `"2" < 10`, `x in 5`, a spread of a string, a computed key that is not a string, an index or an exponent that is not a whole number, a star on a map or a string, an opaque value rendered as text (2.13) | `kind` |
| `missing-field` | A plain `.name` or `["k"]` reads a key that the map does not have; a field of a map pattern is absent and has no default | `field` |
| `index-range` | A plain `[i]` on a list or a string, with `i` a whole number below 0 or not below the length; an element of a list pattern has no default and the list is too short | `index` |
| `divide-by-zero` | A zero divisor in `/` or `%`, and `0 ** b` with `b` below zero (2.5) | `divide` |
| `no-match` | A `match` by cases in which no arm matches and there is no `_`: data that broke its declared type (2.10) | none. Its class is `environment` |
| `number-range` | Whole-number arithmetic whose exact result is not a whole number, and a result that would be NaN or infinite (2.5). `05` named this code | `range` |
| `not-conforming` | `x as T` where `x` does not conform to `T` (2.17). `07` named this code | `value` |
| `failed` | A call of `fail`. `06` named this code | `value` |

- **`catch` catches all nine.**
- **The class** that a host sees for each is `03`'s rule (`03` 2.14, `05` A.6).
  A null, kind or field failure is `dynamic` at a `dynamic` site and
  `environment` anywhere else. A program compiled with no check has no sites, so
  there such a failure is `dynamic`. A program never sees the class.
- **The other run codes belong to other documents:** the library's
  `invalid-argument` (`06`), the host's `function-error`, `function-panic` and
  `missing-variable`, and the five codes of a limit or a cancellation (`05`
  A.5). `05` A.6 lists every code of every phase.
- A walk over a value that goes past the depth limit (`==`, rendering) is
  `depth-exceeded`, and `catch` does not catch it.

**The trace (10-5).** The diagnostic of a failed run lists the calls in progress
when it failed, from the outermost in. A call in progress is what 2.16 counts
for the depth limit. A star in progress has an entry of the same shape as a
library callback's: the span of the star and the index of the element. A program
cannot read the trace (`10` 2.8).

**Messages (10-6).** Every code of every phase has its message in one catalog,
`messages.json`, which is data of the specification. Its rules are in `10` 2.9
and `05` A.6. This document names codes and says what a message tells the
author. It words none.

### 2.13 Templates and rendering

A template string's value is the concatenation of its text parts and the
**rendering** of each interpolated value. The same rendering defines the
library's `string(v)` and a host's `${{ }}` strings.

A template string here is a `Template` node: a backtick template or a text block
that has at least one `${ }` (2.3). The two forms render alike. In a text block
the text parts are what is left after the indentation is removed (2.2), and a
rendered value is inserted as it is: when it has line feeds, its later lines get
no indentation (08-7). A literal with no `${ }` is a `String`. Nothing is
rendered, and its value is its text.

| Value | Rendering |
| --- | --- |
| string | its code points, unchanged |
| number | `Number::toString` (2.5) |
| bool | `true` or `false` |
| null | the empty string |
| list | `[`, the JSON form of each element separated by `,`, `]` |
| map | `{`, then for each entry in key order by code point: the JSON form of the key, `:`, the JSON form of the value, separated by `,`, then `}` |
| opaque | an error |

The **JSON form** of a string is `"`, its code points, `"`, with `"` as `\"`,
`\` as `\\`, U+0008, U+000C, U+000A, U+000D and U+0009 as `\b`, `\f`, `\n`, `\r`
and `\t`, and every other code point below U+0020 as `\u00` and two lower-case
hex digits. Every other code point is written as it is, including `/`, `<`, `&`
and U+2028. The JSON form of a number or a boolean is its rendering, of null is
`null`, and of a list or map is its rendering. No white space is added. Nesting
deeper than the depth limit is the error `depth-exceeded`. An opaque value, at
any depth, is `wrong-kind` (2.12).

**Host strings.** A host may define strings such as `Review of #${{ pr.number
}}`. The delimiters are `${{` and `}}`, fixed in 3.0 (overview S14). After an
opening delimiter, one `Expr` is parsed by the grammar above, and the closing
delimiter must follow it, so braces and strings inside the expression cannot end
it early. Rendering is the table above. How the opening delimiter is escaped,
and whether a string that is exactly one interpolation yields the value and not
its text, are the host API's rules.

The string forms of 2.2 are not part of a host string's own text: outside `${{
}}` the text is the host's, and `"""` and `r"` there are characters. Inside a
segment stands an expression, so every form may be written. A segment ends where
the language's lexer says, and that lexer reads raw strings: `${{ r"\" }}` ends
after the second quote. A splitter that only steps over `\"` is wrong (`08` 2.9,
`05` A.7). In a template or a text block of a program, `${{` is `${` and then a
map, as before.

### 2.14 Limits: what the language guarantees

- **Termination.** Every program terminates if the host's functions and the
  library's do. It does so by construction, and not by reaching a limit (E6,
  D34). The argument has four steps.
  1. There is no loop. A projection repeats its reads (2.8), but it makes one
     pass for each element of a list that exists before the first pass, and it
     carries nothing from one pass to the next. So the number of passes is fixed
     before any is made. A read is not a lambda: an index expression in it is
     text of the body it is written in.
  2. A lambda is entered in two ways only (2.9): by a call by name of a
     `const`-bound lambda, and by a call that a library function or method makes
     of a lambda passed to it. Nothing else can run a lambda, because a function
     is not a value: it cannot be stored or returned, and a parameter cannot be
     called.
  3. So the text fixes which lambdas a lambda `L` can enter: one written inside
     `L`, or one bound by a `const` that is in scope where `L` is written. A
     name is not in scope in its own value, so such a `const` ends before `L`
     starts. Either way the lambda entered ends earlier in the text than `L`
     ends.
  4. Along any chain of calls in progress the end positions therefore fall, and
     **no lambda is entered while an earlier call of it is in progress.** A body
     is a finite text with no loop, a projection in it makes a finite number of
     passes, and a library function calls a lambda a finite number of times. So,
     by induction over that order, every call returns.
- **Calls in progress.** A program with `k` lambdas has at most `k` calls of
  lambdas in progress at once, each of a different lambda. This follows from
  step 4. It bounds the calls and not the stack: 2.16 has the bound an
  implementation sizes a stack by.
- **Work.** Evaluation order is fully specified (2.1, 2.7, 2.8, 2.10, 2.12), so
  the sequence of operations is the same in every implementation. A lambda body
  runs only when a library method or a call by name runs it.
- **What a program can build.** Numbers have a fixed size, so arithmetic and
  `**` build nothing. Without a library entry that makes a sequence from a
  number, the size of every value is bounded by the input size times a factor
  that depends only on the text: `const b = [...a, ...a];` doubles, so the
  factor can be exponential in the number of such lines. If the library gains
  `range` or `repeat` (E8), size also depends on number values. Either way the
  allocation budget is what bounds memory.
- **Constructs that create a value**, and the size a runtime can compute before
  creating it:

  | Construct | Creates |
  | --- | --- |
  | list literal | one list: its plain items plus the length of each spread list |
  | map literal | one map: at most its plain entries plus the size of each spread map |
  | `...rest` in a pattern | one list or map: at most the size of the matched value |
  | template string, in either form: a `Template` (2.3) | one string: the text parts plus each rendering; the rendering of a list or map is found by a walk that is itself charged as work |
  | `s + t` | one string: the two lengths |
  | `s[i]` | one string of one code point |
  | a projection (2.8) | one list: one item for each value its reads gave. The size is not known before the reads have run. It is charged when they have, and before the list is built (`13` 5.2, `04` A.4) |
  | `catch (err)`, when it catches | one map of four entries; the message, when it is built for this error; and the path, when there is one (`10` 2.7). `catch` with no name creates nothing |
  | a `String` literal, in every form | nothing. The string is text of the program: 0 steps and 0 units, the backtick and the text block included (08-8) |
  | lambda, block, `const`, `if`, `match`, `? :`, `!`, `&&`, `\|\|`, `??`, a `try` whose body gives a value, `as`, `is`, a chain with no star, arithmetic, comparison | no value of the language. `as` gives back its operand. `&&` and `\|\|` give back an operand that already exists |

- **Operations whose work grows with their operands:** `==` and `!=` on lists
  and maps (each element compared), `in` (each element, or the string's length),
  `<` on strings, spread, rendering, `s[i]` (the position must be found), a
  projection (each element a star visits), and `as` and `is` (each value the
  test looks at; `07` 2.7).
- Host data may be nested without end. Every walk the language does over a value
  (`==`, rendering) is bounded by the depth limit. The walk of `as` and `is`
  goes only as deep as its type, and a type is at most 64 deep (`03` 2.17).
- **Source text may not be nested without end.** 2.16 fixes the maximum and says
  how it combines with the depth limit, so that every recursion an
  implementation makes over a program has a bound the specification states (R5).

### 2.15 What the checker is expected to do

`03-types.md` owns this. The language was designed on these expectations:

- `? :`, `if`, `??`, `try`, `match` and list literals type as the join of their
  parts. A map spread merges fields, later entries winning; a computed key makes
  the result an open map; `...rest` types as the map without the named fields.
- A `try` types as the join of its body and its handler. The name of `catch
  (err)` has the type of the error value: an exact record whose `code` is the
  literal set of the codes that `catch` catches. Sites recorded in the body are
  dropped, and nothing learned in the body holds in the handler (`10` 2.6).
- A `String` types as the literal set of its value and a `Template` as `string`,
  whatever form wrote it (08-8).
- **Tagged unions (D47; `09`).** A type may hold several record types when one
  required field with string literal values tells them apart. That field is the
  tag, and the checker finds it. Before a test, a read sees the joined record,
  so a field that only some variants have may be absent. `s.kind == "circle"`,
  `!=`, `match s.kind` and `s is T` narrow `s` to the variants that remain. Join
  never makes a tagged union: only a written type does (09-3).
- **A literal fits a written type through its form (`09` 2.6).** Where an
  expression must fit a type that someone wrote, as the program's expected
  result or as an argument, the fit goes through a list literal, a map literal,
  a conditional, a `match` and a block. So a mismatch is reported at the element
  or the entry that is wrong, and not on the whole expression. A required field
  with no entry is reported on the whole map. No type changes.
- A projection over a `T[]` whose reads end at `U` types as `U[]`, with null
  added to `U` where a `?.` can skip. On `any` it is `any[]` (`13` 5.2).
  `filter` and `find` narrow the element by the true facts of the predicate, so
  `xs.filter(x => x != null)` loses null (`13` 5.1).
- A lambda argument takes its parameter types from the callee's signature. A
  `const`-bound lambda has no type of its own: its body is checked again at each
  use, which terminates because the uses form no cycle (2.14).
- The two rules of 2.9 are checked on bindings alone, before any type is known.
  A parameter never holds a function, so every parameter has a type.
- `!a` types as `bool`. `a || b` has the part of `a`'s type that can be truthy,
  joined with `b`. `a && b` has the part that can be falsy, joined with `b`
  (B7). So with `user.name` a `string | null`, `user.name || "anonymous"` is a
  `string`. With `a.items` a `string[]`, the type of `a.items && len(a.items) >
  0` is a list or a `bool`, written `bool | string[]`.
- **A truthy test narrows null away.** `x` is not null in `e` of `x && e`, in
  the first branch of `x ? e : d` and of `if (x) { e } else { d }`, in `e` of
  `!x || e`, and in the result of a condition arm `x => e`. `x != null`, `x ==
  null` and the arms after `null =>` narrow as well. In an `if` chain, as in a
  chain of `? :`, each later branch also has the negation of every test before
  it. A falsy test proves nothing about null, because zero and the empty string
  are falsy too. A default in a pattern removes null from the name. So a name
  typed `string | null` is a `string` after any of these.
- `x as T` types as `T`, and `x is T` as `bool`. Where an `is` test passed, `x`
  has type `T`, if `x` is a name or a path the checker can narrow. An `as` that
  no value can pass is `never-conforms`. An `is` that can only go one way is a
  constant condition. Otherwise an `as` is a site of class `value`, as a call of
  a library entry marked `!` is (`07` 2.6, `03`).
- **A condition that can never be falsy, or never truthy, is a diagnostic**
  (`always-truthy` and `always-falsy` in `03`): a record with a required field,
  an opaque value, a string literal that is not empty, `null`. It applies to the
  test of `? :` and of `if`, a condition arm, the operand of `!`, the left
  operand of `&&` and `||`, and the body of a predicate. TypeScript reports the
  same mistake as "this condition will always return true".
- The two codes form one check class, `constant-condition`. It is in the default
  set of classes that an environment's check rejects. A host may take it out,
  and then neither code is reported (D32; `03` 2.13, `05` A.2).
- When the host passes a type as the expected result, `bool` for one, the
  program's type must fit it. When it passes `condition`, any type fits, and the
  result is a tested position that the same class covers (B8, D33; `05` A.3).
- An `OptChain` types as its last step, with `null` added if any `?.` receiver
  may be null: `review?.verdict` is `string | null` when `review` is `{verdict:
  string} | null`. A step after an optional read that may find nothing needs its
  own `?.`.
- `**` types as `int` when both operands are `int` and the exponent is a
  non-negative literal, else `float`.
- A value typed `any` turns checking off and the run-time rules apply.

### 2.16 Nesting

A stack overflow ends a Rust process. In JavaScript it comes at a depth the
engine chooses. In Go it comes only when a stack reaches its maximum, a gigabyte
by default, long after the other two. So how deep a program may nest is a rule
of the language, with one number, counted one way (R5).

**Levels.** A parser counts the levels of nesting that are open. Each of these
opens one level:

| Opens a level | The level lasts |
| --- | --- |
| A bracket: `(`, `[`, `?.[`, `{`, or the `${` of a template string or of a text block | to its closing bracket. This covers a parenthesized expression, an argument list, a parameter list, an index, a list, a map, a computed key, a pattern, a block, the test of an `if` and each of its branches, the body and the handler of a `try`, the parentheses of `catch (err)`, the arms of a `match` and an interpolation |
| The star of a projection: `[*]` or `?.[*]` | to the end of its projection (2.8). So the reads after one star are one level inside it, and those after a second star are two. The brackets of the star itself hold nothing and open no other level |
| A prefix operator: `!` or `-` | to the end of its operand |
| A binary operator: `\|\|`, `&&`, `??`, `==`, `!=`, `<`, `<=`, `>`, `>=`, `in`, `not in`, `as`, `is`, `+`, `-`, `*`, `/`, `%` or `**` | to the end of its right operand. For `as` and `is` that is the end of the type |
| In type text: `(`, `{`, and the `<` of `map<` | to its closing bracket |
| In type text: each `[]` after a type | to the last `[]` of that type. The levels of the suffixes add up: in `int[][][]` the third `[` is three levels above the `int` |
| The `?` of a conditional | to its `:` |
| The word `match`, when a subject follows | to the `{` after the subject |

Nothing else opens a level. In particular:

| Not a level | So this is flat, however long it is |
| --- | --- |
| The left operand of a binary operator | `a + b + c + ...`. Each term after the first is one level inside the chain. No term is inside another |
| The receiver of `.name`, `?.name`, `[ ]`, `?.[ ]`, a star or a method call | `x.f().g().h()...`. Each argument list is one level. The calls are not inside one another |
| The test and the else branch of a conditional | `a ? x : b ? y : c ? z : w` |
| The words `if` and `else`, and an `if` that follows an `else` | `if (a) { x } else if (b) { y } else if (c) { z } else { w }`. Each test and each branch is one level inside the chain. No branch is inside another |
| The words `try` and `catch` | `try { a } catch (err) { b }`. The body, the parentheses and the handler are each one level. None is inside another |
| The next `const` of a block, and the block's result | a script with any number of `const` lines |
| The next item, entry, argument, parameter or arm | a long list; a `match` with many arms |
| A lambda's body | It is at the level of the lambda, which stands in an argument list or is a `const` value |
| The `\|` between the alternatives of a type, and the next field of a record type | `x as "a" \| "b" \| "c" \| ...`, and a record type with many fields |
| White space, comments, the text of a string, a template or a text block, and the `-` of a `match` case literal | A text block of any number of lines. Only a `${` in it opens a level |

**The rule.** At most **100** levels may be open at once. The token that would
open level 101 is the error `nesting-exceeded`, of phase `parse`. Its span is
that token (for `not in`, both words). Parsing ends there: diagnostics found
before it stand, and none is reported for the text after it. The number belongs
to the specification version. A host cannot raise or lower it, so whether a text
is a program does not depend on the host or on the implementation. In a host
string (2.13), each `${{ }}` segment starts with one level open, as after `${`.

The **nesting depth** of an expression is the number of levels open at its first
token. The count is made on the text, not on a tree, so it does not depend on
what tree an implementation builds: parentheses count, and they have no node.

**What is not a level must not cost stack.** A parser, checker, compiler and
evaluator, and any other walk over a program (analysis, formatting, releasing
the tree), may recurse a bounded number of times for each level and not
otherwise. What the second table lists is handled with a loop: the operands of a
left-associative chain, the steps of a chain, an else chain of `? :` or of `if`,
the `const` lines of a block or branch. So a sum of 100,000 terms, a chain of
100,000 method calls, a conditional with 100,000 else branches, an `if` with
100,000 `else if` arms and a block of 100,000 `const` lines all parse and,
within the step budget, run, in every implementation on its default stack. 2.3
shows a chain as nested `Binary`, `Member` and `MethodCall` nodes. An
implementation may hold it as one node with a list, which keeps every walk
shallow without further care.

Between one level and the next, a walk passes a bounded number of the nodes of
2.3: six at most by my count, for example an `Entry`, a `Lambda`, a `Cond`, a
`Binary`, an `OptChain` and a `MethodCall` (I). A `Try` stands where a `Cond`
stands, and an `Each` opens a level of its own, so neither raises the count (I).
So the recursion of any walk over a program is at most that constant times 100.
A projection of many stars is as deep as its stars are many, and the 100 levels
bound it: an evaluator may recurse once for each star.

**How it combines with the depth limit.** Three things use an implementation's
stack during a run: the nesting of the source, the calls in progress, and a walk
over a value (`==`, rendering, encoding). The depth limit of `04` A.6 and `05`
A.5, 200 by default, bounds the second and the third. But an evaluator inside a
call is also inside the nesting of the place the call was made from, so what the
stack holds is the nesting at every call in progress, added up. The depth limit
counts that:

- **A call in progress counts against the depth limit as 1 plus the nesting
  depth of the call within the body that makes it.** A body is the program or a
  lambda. The depth of a call within its body is the number of levels open at
  the call's first token, less the number open at the first token of the lambda
  (none, for the program).
- A call here is a call of a `const`-bound lambda by name, or a call that a
  library function or method makes of a lambda passed to it. In the second case
  the call that is measured is the library call. These are the only two ways a
  lambda is entered (2.9, D34).
- A projection is not a call, and a star in progress counts nothing against the
  depth limit. A call made from inside a projection stands one level deeper for
  each star around it, by the rule above. A `try` counts nothing either: a call
  in its body or its handler is one level deeper, for the braces.
- A call that would take the total past the limit fails with `depth-exceeded`.
  The count is fixed by the text, so it fails at the same call in every
  implementation.

| Program | Calls in progress at `f` | Counted |
| --- | --- | --- |
| `xs.map(x => ys.filter(y => f(y)))` | 3 | 3: each call is at depth 0 within its body |
| `xs.map(x => [(f(x))])` | 2 | 4: the callback is 1, and `f` is 1 plus the 2 levels it stands in |

A program whose calls are not wrapped in brackets or operators is counted one
per call in progress, which is the plain count of calls.

**The bound an implementation can rely on.** Let `D` be the depth limit.

| Recursion | Bound, in levels |
| --- | --- |
| Parser, checker, compiler, analysis: over the source | 100 |
| Evaluator: over the source, through every call in progress | `D` + 100 |
| A walk over a value. It runs no lambda, so it sits at the end of the evaluator's recursion | `D` more |
| The walk of `as` or `is`. It is a walk over a value too, and counts the same way: 1 against `D` for each list or record it enters (`04` A.6). It enters a level only where the type has one, so at most 64 whatever the data (`03` 2.17, `07` 2.7) | within the `D` above |
| A run, in all | 2`D` + 100, which is 500 at the default |

**Type text counts toward the 100 levels.** A type after `as` or `is` is source
text that a parser recurses over, so its brackets are levels like any others,
and the token that would open level 101 is `nesting-exceeded`. A `[]` suffix
counts although the parser reads it in a loop, because the type it builds is
nested: a list of lists of lists. Counting it keeps every walk over a written
type within the 100, with no second number at parse time. The limit of 64 on the
depth of a type is separate and stays with the checker (`03` 2.17): a name the
host declared adds depth that the text does not show. So a type written in place
may parse and then be refused as too deep.

The checker's check of a `const`-bound lambda at each use (2.15) is also a
recursion through calls. `03` 2.17 bounds it by a sum of the same kind, made
when the program is checked, and reports `check-limit`.

Calls in progress are bounded by the text as well as by the limit. A lambda can
enter only lambdas bound before it or written inside it (2.14), so a program
with `k` lambdas has at most `k` calls in progress, each of a different lambda.
Since D34 this holds for every program. That bound is true and too large to size
a stack by, since `k` can be in the thousands. The depth limit is the one to
rely on.

**What D34 simplifies.** The rule above is unchanged. What it rests on is
simpler.

| Before D34 | Now |
| --- | --- |
| A third kind of call existed: a call of a parameter that held a lambda. The rule did not say what it counted | The two kinds in the rule are all there are, and each names its lambda in the text: by a `const` name, or as an argument of the library call |
| Which lambda a call entered could depend on the run | It is fixed by the text, so what each call adds to the count is a constant of the call |
| A lambda could be in progress more than once, so only the limit bounded the calls in progress | Each lambda is in progress at most once (2.14) |
| The largest count a program could reach was known only by running it | It is the longest path in a finite graph with no cycle. A checker or a compiler can find it before the run (`03` 2.17, `04` A.9) (I) |
| A count made ahead of the run was unsound for a lambda passed to a `const`-bound lambda | It is an upper bound for every run |

The run-time count stays, for three reasons. The depth limit is the host's
number, and a host may set it below what the text can reach. A count over the
text includes branches that a run never takes, so it can refuse a program but
cannot say where a run stands. And a walk over a value uses the same limit.

*Why a sum and not a product.* If the depth limit counted calls alone, each of
200 calls could sit 100 levels deep: 20,100 levels. A closure evaluator and a
tree walker each overflowed near 9,000 frames in Node 22 (**ran**). A small tree
walker in Rust used 287 bytes of stack a level (**ran**), which is 5.8 MB for
20,100 levels against the 2 MiB of a spawned thread. For the product to be safe
the limits would have to be about 20 calls or about 10 levels, and either
rejects real programs. With the sum, 300 levels at six frames each is 1,800
frames, or about 520 KB in that walker.

*Why 100.* The deepest example in section 5 opens 5 levels (example 34; counted
by hand). A JSON value pasted as a literal opens one level for each bracket, and
serde_json refuses JSON nested deeper than 128 by default
(`serde_json-1.0.154:src/de.rs:63`). 100 is half the default depth limit, so a
call made from the deepest legal place still runs: it counts 101 of 200. A
recursive-descent parser for 2.2 uses about twelve frames a level, one for each
production from `Expr` to `Primary`, so 1,200 at the limit (I).

*Rejected.*

| Alternative | Why not |
| --- | --- |
| The depth of the tree, with every link of a chain counting. v2 does this, with 500 (`v2:pkg/parser/parser.go:46-52`) | A sum of 600 terms is an error. A number high enough for long chains is too high for a stack once calls are added. And it depends on the tree: parentheses have no node |
| Brackets only | Four recursions have no bracket: `!!!!x`, `a ** b ** c`, `a ? b ? c : d : e` and `match match x { ... } { ... }`. And six binary operators can nest by precedence between two brackets, so a level would cost a walker seven times the frames |
| Count the links of a chain, with a separate maximum length | A chain inside a bracket inside a chain multiplies. 100 levels of 1,000 links is 100,000 frames for a walker that recurses along chains |
| Leave the depth limit a plain count of calls | The product above |
| Count through calls in the parser: add a helper's own depth wherever it is called, and need no run-time rule | It rejects a program for a call path it never takes, and it makes the parser resolve names. D34 removed a third objection, that the count was unsound for a lambda passed to a `const`-bound lambda. The checker does make this count (`03` 2.17), because it expands each call anyway. A run still needs the rule above, because a host may set the depth limit lower than the checker's number |

### 2.17 Testing a value against a type: `as` and `is`

`07-checked-conversion.md` is the authority (D39 to D45). It has the conformance
rule, the order of the walk, the cost and the host's side. This section has what
the language's surface owns: what the two operators are, and how their text is
read.

| | `x as T` | `x is T` |
| --- | --- | --- |
| Value | `x` itself, when it conforms to `T`. The same value: nothing is copied | `true` when `x` conforms to `T`, else `false` |
| When it does not conform | A run-time error with code `not-conforming` and class `value`. `catch` catches it, and `err.path` is the path to the first failure (2.12) | `false`. It never fails |
| Evaluation | `x` is evaluated once, then tested. The type is text: nothing in it is evaluated | The same |

- **It is a test, not a conversion.** The value does not change. Nothing is
  coerced, filled in or removed: `"42" as int` fails, and `int("42")` is the
  conversion.
- **`T` is a type:** type text written in place, by the grammar of `03` 2.2, or
  a name the host declared (`05` A.2). A program cannot declare a type name in
  3.0. `type Name = ...` is a targeted error (3.2, D45), and `type(x)` stays the
  library's function.
- **A type may be a tagged union (D47).** Several record types joined by `|`
  are one type when one required field tells them apart: its type in each is a
  set of string literals, and no string is in two of the sets. The field is
  found from the records and never written. No grammar is added: `Type` of `03`
  2.2 already reads `{...} | {...}`. Two record types with no such field are the
  check error `untagged-union`, on the union (`09` 2.3). There is no untagged
  union of records. The test reads the tag first and then one variant, so a
  value that conforms costs what it costs against its variant alone (`09` 2.7).
- **The rule is conformance,** as `07` 2.2 and `03` 2.14 define it. Three points
  an author meets: a record type with no `...` is exact, so any other key fails
  (D41); a field that is absent is not a field that is null (D42); and the test
  does not look under `any`.
- **Cost.** One step for each value the test looks at, and no allocation when
  the value conforms (`07` 2.7, `04` A.3). The order of the walk is specified,
  so every implementation reports the same first failure and charges the same
  steps (`07` 2.3).
- **`is` in a condition narrows.** In `if (body is Order[]) { len(body) } else {
  0 }`, `body` is an `Order[]` in the first branch (2.15).

**How the text is read.** The two operators stand at the level of a comparison
and associate to the left, and the type ends by the rules of 2.2. Below, `T` and
`U` stand for any type. The third column is TypeScript 5.9 on the same text with
its own type names (**ran**).

| Text | Here | TypeScript |
| --- | --- | --- |
| `a + b as T` | `(a + b) as T`: the sum is tested | the same |
| `x as int > 3` | `(x as int) > 3` | the same |
| `3 < n as int` | `(3 < n) as int`. The checker rejects it, because a `bool` is never an `int` (`never-conforms`). Write `3 < (n as int)` | the same grouping |
| `y == x as T` | `y == (x as T)` | the same |
| `x is T ? a : b` | `(x is T) ? a : b`: a `?` ends the type | the same, for `as` |
| `x as T \| null` | the type is `T \| null` | the same |
| `x as {kind: "a"} \| {kind: "b", n: int}` | one type, a tagged union of two records: the `\|` continues the type, and the `{` after it starts a record type (2.2) | the same grouping. There any two object types form a union, with or without a tag |
| `x as T \|\| d` | `(x as T) \|\| d`: `\|\|` ends the type | the same |
| `x as T[]` | the type is a list of `T` | the same |
| `x as map<int> > y` | `(x as map<int>) > y`. The first `>` closes `map<`. The checker then rejects the comparison, because a map has no order | the same grouping |
| `x as T as U` | `(x as T) as U` | the same |
| `c ? x as T : y`, `{k: x as T, j: 1}`, `xs[i as int]` | the `:`, the `,` and the `]` end the type | the same |
| `match x as "a" \| "b" { ... }` | the `{` after a complete type ends it and opens the arms | no `match` |
| `x as int + 1` | an error: write `(x as int) + 1` | accepts it, with that meaning |
| `x as T ?? d` | an error, by the rule for `??` (2.2): write `(x as T) ?? d` | accepts it, with that meaning |
| `x as T[0]`, `x as T[][0]` | an error: a `[` after a type must be `[]`. Write `(x as T[])[0]` | reads `T[0]` as a type |
| `x as T.f` | an error: write `(x as T).f` | reads `T.f` as a type name |
| `!x is T`, `!x as T` | an error: write `!(x is T)` | `(!x) as T` |

Where the two languages differ on a text, this one gives an error and never a
second meaning. The one difference in meaning is `as` itself (3.1).

*Why the level of a comparison.* It is TypeScript's (C6), and it makes `x.n as
int > 3` and `x is Order && x.total > 0` read without parentheses. Its cost is
`(x as Order).id`, which needs them.

*Why `!x is T` is an error.* It would parse as `(!x) is T`, a valid test of the
wrong thing, as `!a in b` would. The checker reports it as a constant condition,
but a host may take that class out (D32), and then nothing would.

## 3. Least surprise

### 3.1 Where the same text means something else than in JavaScript

Where a construct is valid in both languages it means what it means in
TypeScript (Y1). These nine are the exceptions, and each removes a wart. What
v3 does not accept is an error: at compile time for syntax, and for kinds when
types are known.

| Text | JavaScript | v3 | Why v3 keeps its meaning |
| --- | --- | --- | --- |
| `x in list` | tests for an index | tests for an element | The reason this work started (expr issue 35). Python, SQL, CEL and the engine's authors all read `in` as membership, and a JavaScript author who writes `role in roles` means that too; the index test is almost never what was wanted. On a map the two agree. |
| `a == b` | converts across kinds; compares lists and objects by identity | never converts; compares lists and maps by content | Coercion is the best-known defect of this family of languages (`01-prior-art.md`, lesson 2). Values here have no identity to compare. `===` gets an error that says `==` already means it. |
| `` `${v}` `` | `null`, `1,2`, `[object Object]` | empty, `[1,2]`, `{"a":1}` | JavaScript's renderings are the surprising ones. Null as empty text is the convention of template languages. |
| `x => { const y = f(x); y * y }` | returns `undefined` | returns `y * y` | Everything is an expression (E5). The JavaScript reading is a bug in JavaScript. `return` gets a targeted error. An `if` at the end of a block is the same case: JavaScript discards its value, and here it is the block's value (D35). So is `try { f(x) } catch (err) { g(err) }`, which is valid in both: there its value is discarded, and here it is the value of the body or of the handler (D48). |
| `({a = 1}) => a`, `(x, n = 2) => n` given `null` | keeps `null` | uses the default | JavaScript has two absent values and a default replaces one of them. v3 has one, and `??` already treats it as "use the default". |
| `xs \|\| ys`, `xs ? a : b`, `!m`, with `xs` an empty list or `m` an empty map | an empty array and an empty object are truthy: `[] \|\| ys` is `[]` | they are falsy: `[] \|\| ys` is `ys` | Three reasons (B5, D30, 2.7). `when: results.items` means "there are items". One sentence states the whole rule: null, `false`, zero and anything empty. And the answer is the same whether a host supplied a null collection or an empty one. On null, `false`, zero and the empty string the two languages agree. |
| `x as T` | trusted: nothing is tested, and a wrong claim shows later as a wrong value somewhere else | tested at run time: `x` conforms to `T` or the run fails with `not-conforming` | D39. A trusted `as` would end the checker's promise at the first one written. The surprise is on the safe side: a cost and a clear error, where TypeScript gives a wrong value later. A second surprise comes with it: a record type with no `...` is exact here, and every TypeScript object type is open (D41). |
| `try { f(x) } catch (err) { g(err) }`, when `f` fails | `catch` catches whatever was thrown, a stack overflow among them | `catch` catches every run error but a limit, a cancellation, a crash and a variable the host did not supply (2.12) | 10-3. A handler would run on a budget that is spent, or after the host said stop. The surprise is on the safe side: the run ends with the error the host must see. |
| `err` in that handler | whatever was thrown: by habit an `Error`, with `name`, `message` and `stack` | a map with exactly `code`, `message`, `path` and `data`. `err.message` reads the same. `err.name` and `err.stack` are compile errors (`no-field`) | 10-2. An error is data, like every value here. A code from a fixed set lets the checker find a misspelled one, which a class name in a string never did. A position is for a person and for the host, not for a program to branch on. |

Also `x => {id: x.id}` is a map here and a labelled statement there, and string
order differs for the few code points above U+FFFF (**ran**: `"\u{1F600}" <
"\uFFFF"` is true in Node and false by code point).

The forms added in the seventh revision change the meaning of no text. Each is a
syntax error in JavaScript (**ran**, Node 22): `"""` before a line end,
`r"a"`, `x[*]`, `x?.[*]`, and a `try` as a value, `const v = try { 1 } catch { 2
};`. TypeScript adds no expression form to JavaScript among these (I). One text
is valid there and an error here: `x as {a: int} | {b: string}`, a union of two
record types with no tag, is a type in TypeScript and `untagged-union` here
(D47).

Three things JavaScript accepts are errors here that types cannot always catch
before the run. Each is the same rule, that nothing is converted or invented
(Y2):

| Text | JavaScript | v3 |
| --- | --- | --- |
| `x.missing`, `xs[99]` | `undefined` | an error; `x?.missing` and `xs?.[99]` give null (B2) |
| `"n=" + 5` | `"n=5"` | an error; write `` `n=${5}` `` |
| `"2" < 10` | converts, then compares | an error |

Two departures in the first draft are withdrawn:

- **`?.`** now has JavaScript's meaning (B4).
- **`??`** had Swift's precedence, so `a ?? 0 > 3` meant `(a ?? 0) > 3`. In
  JavaScript it means `a ?? (0 > 3)` (**ran**). Neither reading is safe to give
  silently, so mixing `??` with a comparison or a logical operator now needs
  parentheses, the rule JavaScript itself applies to `??` beside `||`.

### 3.2 Targeted errors, and every `parse` code

A targeted error is a compile error whose message names the fix. This section
also names every code of phase `parse`, which no document did before D48.

**How the codes are chosen.** One code names one mistake. Each targeted error
has a code of its own, named for what the author wrote. Rows that are the same
mistake in another spelling share a code, and the catalog then has a template
for each row (`10` 2.9). So a conformance case, and a host that shows its own
wording, can tell one mistake from another by the code alone. The wording is the
catalog's, `messages.json`. The third column below says what each message tells
the author, and it is not that wording.

**The codes that are not targeted errors.**

| Code | Reported when | Span |
| --- | --- | --- |
| `unexpected-token` | The text does not fit the grammar of 2.2, and no other code names the mistake. The message names the token found, or `end of input`, and not what was expected (`10` 2.9) | the token |
| `unexpected-character` | A character outside a string, a template and a comment that begins no token: a non-breaking space, a letter that is not ASCII, `@` | the character |
| `unterminated-string` | A string, a template or a text block has no closing delimiter. For a one-line string, raw or not, that is a line end before the closing quote | from the opening delimiter to where the literal stops |
| `unterminated-comment` | A `/*` has no `*/` | the `/*` |
| `bad-escape` | An escape that 2.2 does not list, such as `\x41`, `\0` and `\v`, or a surrogate escape with no partner | the escape |
| `line-continuation` | A `\` stands directly before a line end, in a template or a text block (08-6) | the `\` |
| `bad-number` | A number form that 2.2 does not have: `.5`, `5.`, `010`, `1__0`, `0b1`, `0o7` | the number |
| `literal-range` | A number literal with no fraction and no exponent is not a whole number, or a literal is not finite: `9007199254740993`, `1e400` (2.5) | the number |
| `text-block-open` | A character other than a space or a tab stands between the opening `"""` and the end of its line | that character |
| `text-block-close` | A character other than a space or a tab stands before the closing `"""` on its line | the closing delimiter |
| `text-block-indent` | A content line that is not blank does not start with the indentation. Reported once for each such line, in order | the first character of the line that differs from the indentation |
| `nesting-exceeded` | A token would open level 101 (2.16) | that token |
| `bad-type-text` | Type text after `as` or `is` that the grammar of `03` 2.2 does not accept there: a form that only a signature has, a TypeScript word such as `number` or `boolean`, a value. `03` 2.2 owns what each message says | the token at fault |

**The targeted errors.** A code in the second column is of phase `parse`. A row
marked *check* is the checker's diagnostic, with the code that `03` 2.13 gives
it, and this document owns only the hint.

| Written | Code | Message says |
| --- | --- | --- |
| `===`, `!==` | `strict-equality` | use `==`, `!=`; nothing is converted |
| `if (c) { a }` with no `else` | `missing-else` | add an `else`, or write `c ? a : null` (D37) |
| `if c { a } else { b }`, `if (c) a else b` | `if-form` | the parentheses and the braces are required |
| `if(c, a, b)`, expr's function | `if-form` | write `c ? a : b`, or `if (c) { a } else { b }` |
| a branch written like a map: `if (c) { a: 1 } else { ... }`, `try { a: 1 } catch { ... }` | `branch-like-map` | after `if (c)`, `else`, `try` and `catch` a `{` opens a branch; write the map inside it, `{ {a: 1} }` |
| an empty branch: `else {}`, `catch {}` | `empty-branch` | a branch needs a value; write `null`, or `{ {} }` for an empty map |
| `if (c) { 1 } else { 2 } + 3`, `try { e } catch { d } + 1`, or a `.` or `[` after the last branch or the handler | `needs-parentheses` | put the `if` or the `try` in parentheses |
| `try(e, d)` | `try-call` | write `try { e } catch { d }` (D48) |
| `try { e }` with no `catch` | `missing-catch` | a `try` needs a `catch` |
| `try e catch d` | `try-form` | the braces are required |
| `catch (e: T)`, `catch ({message})`, `catch (_)`, `catch ()` | `catch-name` | the parentheses hold one name; write `catch { ... }` when the error is not used |
| `finally` | `finally-clause` | there is no `finally`: nothing in a program has an effect to undo; remove it |
| `throw e`, `throw new Error("m")` | `throw-statement` | write `fail("m")` |
| `switch`, `case` | `switch-statement` | `match x { ... }` |
| `function f(x) { }` | `function-declaration` | `const f = x => ...;` |
| `return` | `return-statement` | a block's value is its last expression; early `return` is not in 3.0 |
| `let`, `var` | `let-binding` | write `const`; a binding never changes |
| `for`, `while`, `do`, `break`, `continue`, and `[for (x of xs) ...]`, `{for ...}` | `loop-statement` | there are no loops and no comprehensions in 3.0; use `map`, `filter` or `reduce` |
| `x = 1`, `x += 1`, `x++` | `assignment` | a binding cannot be reassigned in 3.0; bind a new name |
| `import "p"`, `import x from "p"`, `import {x} from "p"`, `import("p")` | `import-statement` | imports are not in 3.0; a host passes shared data as a variable (`12` 3.9) |
| `export const x = ...` | `export-statement` | a program has one value and exports no name |
| `x => { e }` (a brace with no `const` that is not a map) | `block-without-const` | `x => e` |
| `const {a} = x => x;` | `lambda-in-pattern` | a lambda is bound to a name, not to a pattern |
| a reserved word as a name: `const match = 1`, `is => ...` | `reserved-word` | the word is reserved; choose another name. After `.` and as a bare map key it is allowed |
| `a ?? b > c`, `a ?? b \|\| c` | `needs-parentheses` | add parentheses |
| `!a in b` | `needs-parentheses` | `a not in b`, or `(!a) in b` |
| `-2 ** 2` | `needs-parentheses` | `(-2) ** 2` or `-(2 ** 2)` |
| a `match` subject that starts with `{`, `if` or `try` | `needs-parentheses` | put the subject in parentheses |
| `x[a:b]` | `slice-syntax` | `x.slice(a, b)` |
| `a?[i]` | `optional-index` | `a?.[i]` |
| `xs.*.f`, `xs[].f` | `star-form` | a projection is written `xs[*].f` |
| `xs[?c]` | `filter-step` | `xs.filter(x => c)` |
| `$.a`, `$` | `context-symbol` | there is no name for the root or for the current element; write the variable's name |
| `a & b`, `a \| b` outside `match` and type text | `bitwise-operator` | `&&`, `\|\|` |
| `x \|> f` | `pipe-operator` | `f(x)`, or the method |
| `nil` | `nil-literal` | `null`, the one spelling of the absent value (D28) |
| `undefined` | *check:* `undefined-name` | `null` |
| `(x: int) => ...`, `const n: int = ...` | `type-annotation` | types are inferred; remove the annotation (8.1, question 8) |
| `it`, where it is not a variable | *check:* `undefined-name` | name the parameter: `x => ...` |
| `f(x)` where `f` is a parameter, a name from a pattern or any other value (2.9) | *check:* `call-of-parameter` | a value cannot be called; only a `const`-bound lambda and a function of the environment can |
| a lambda passed to a `const`-bound lambda (2.9) | *check:* `lambda-to-lambda` | a lambda is passed only to a library function or method; pass the data, and write the lambda where it is used |
| `assert(c)`, `assert(c, "m")` | *check:* `undefined-function` | for a guard in a program, `c ? x : fail("m")`; for a test, a case file run by `risor test` (D49; `11`) |
| `require("p")` | *check:* `undefined-function` | as for `import` |
| `s.trimIndent()`, `s.stripIndent()`, `dedent(s)` | *check:* `no-method`, `undefined-function` | a text block removes its indentation (2.2). `06` owns the hint |
| `users[*].email.toLowerCase()`: a method of the element, after a projection | *check:* `no-method` | a method call ends a projection and receives the list; write `users.map(u => u.email.toLowerCase())` |
| `m[*]`, with `m` a map | *check:* `kind` | a star reads a list; write `m.values()[*]` |
| `x as {a: int} \| {b: string}` | *check:* `untagged-union` | two record types in a union need a field that tells them apart (`09` 2.3) |
| `x satisfies T` | `satisfies-operator` | there is no static-only claim in 3.0; `x as T` tests the value at run time |
| `x instanceof T` | `instanceof-operator` | `x is T` |
| `typeof x` | `typeof-operator` | `type(x)` for the name of the kind, or `x is T` for a test |
| `<T>x` | `angle-cast` | `x as T` |
| `interface Name { ... }`, `enum Name { ... }` | `type-declaration` | a program declares no type in 3.0; write the type in place, `{...}` or `"a" \| "b"`, or use a name the host declared |
| `type Name = ...` | `type-declaration` | `type` lines are not in 3.0 (D45); the same fix. `type(x)` is the library's function |
| `x as const` | `as-const` | remove it; no value can change |
| `x is not T`, `x !is T` | `negated-is` | `!(x is T)` |
| `!x is T`, `!x as T` | `needs-parentheses` | `!(x is T)` |
| `(x as T).f`, `(x as T[])[0]`, `(x as int) + 1` or `(x as T) ?? d` written without the parentheses | `needs-parentheses` | add them |
| `x is 3`, `x is true` | `bad-type-text` | a type is expected; compare with `==` |
| a lower-case word that is not a type, or one upper-case letter, where a type is expected | `bad-type-text` | a type name starts with an upper-case letter and has two characters or more |
| `x is map<int>=> ...` | `bad-type-text` | put a space before `=>` |
| `'''` | `text-block-quote` | a text block is written `"""` |
| `"""text` on the opening line | `text-block-open` | the content starts on the next line |
| `text"""` | `text-block-close` | the closing `"""` stands on its own line, and what stands before it is the indentation removed |
| a content line to the left of the closing delimiter | `text-block-indent` | indent the line, or move the closing `"""` left. When the difference is a tab against a space, it says so |
| a line end inside `"..."`, `'...'` or `r"..."` | `unterminated-string` | a string ends on its line; write a text block, or `\n` |
| `\` before a line end, in a template or a text block | `line-continuation` | `\` does not join lines in 3.0; write `\\` for a backslash, or `r"""` |
| `f"..."`, `$"..."` | `string-prefix` | a template or a text block interpolates with `${ }`, and has no prefix |
| `R"..."`, `r "..."` | `string-prefix` | the prefix is a lower-case `r` directly before the quote |
| `r#"..."#`, `#"..."#` | `string-prefix` | guards are not in 3.0; what a raw string cannot hold is written in a plain form, or interpolated (2.2) |
| `` r`...` ``, `` String.raw`...` ``, any `` name`...` `` | `tagged-template` | there are no tagged templates; raw text is `r"..."` or `r"""` |
| `<<EOF`, `<<-EOF`, `<<~EOF` | `heredoc` | write a text block |
| `# a comment` | `hash-comment` | a comment is `//` to the end of the line, or `/* */` |
| `{max-retries: 3}`: a bare key that is not a word | `unquoted-key` | quote the key: `"max-retries": 3` |
| a non-breaking space or other stray character | `unexpected-character` | the character and its position |

That is 58 codes of phase `parse`: the 13 of the first table and 45 more in the
second. Three of the 13, the codes of a text block, also stand in the second
table with what their messages say, and so do `unterminated-string`,
`line-continuation`, `bad-type-text` and `unexpected-character`.

- **`needs-parentheses` is one code for seven rows.** They are one mistake: a
  form that must stand in parentheses where it was written. Its span is the
  token that cannot follow, or for `!a in b`, `!x is T` and `-2 ** 2` the `!` or
  the `-`.
- **Spans.** The span is the one token that shows the mistake: the `===`, the
  `try` of `try(`, the reserved word, the `:` of an annotation or of a slice,
  the prefix of a string. `missing-else` and `missing-catch` have the span of
  the token that stands where the word should. `branch-like-map` and
  `empty-branch` have the span of the branch's `{`. The suite's cases fix the
  span of each row (7.1).
- **After an error.** After `nesting-exceeded` parsing ends (2.16). After
  `text-block-open` or `text-block-close` the lexer reads on to the end of the
  literal and reports nothing more for it (2.2). What a parser reports after any
  other first error is `05` A.6's matter.
- The parser reports a reserved word wherever a name or a value may stand. After
  `.` and as a bare map key it is a word like any other (2.8), so `x.nil`,
  `err.catch` and `{for: 1, import: 2}` are valid.
- `satisfies`, `instanceof`, `typeof`, `interface`, `enum`, `type`, `switch`,
  `case` and `export` are not reserved. Each is recognized where two words, or a
  word and a literal, stand side by side, or where a `{` follows a call, which
  is never valid. `as` and `is` are reserved, and since D48 so are `catch`,
  `throw` and `finally`. `throw` was not reserved before, and had no row.
- `import` is reserved in 3.0, so that imports can arrive in a minor version
  without breaking a program that binds the name (12-8). `export`, `from` and
  `require` need no reservation: each text that uses one is an error today.
- A row marked *check* needs the environment. `assert(c)` is an error only when
  nothing declares `assert`, and a host may declare it. Such a row is reported
  when the program is checked, with the checker's code. The hint is the
  diagnostic's `help`, which the catalog does not specify and the suite does
  not compare (`10` 2.9).
- A word that is wrong inside type text, such as `number`, `boolean` or
  `undefined`, is `bad-type-text`, and `03` 2.2 owns what its message says.
- The nesting error of 2.16 says which bracket or operator opened the level too
  many. A condition that can never be falsy is the checker's diagnostic (2.15),
  not one of these.
- Loops, reassignment and early `return` are deferred, not refused (overview
  2.10). So are imports (`12` 3.9), comprehensions (`13` 6.1), a pattern in
  `catch ( )` (`10` section 7) and a `\` that joins lines (08-6). Their messages
  say "not in 3.0". When such a form arrives, its row goes and its code is no
  longer reported: a code is never renamed or reused (`05` A.6).

### 3.3 Authoring in YAML

- **Both quote styles**, so a YAML value in single quotes can hold an expression
  that uses double quotes, and the reverse.
- **Newlines are white space.** A `|` block and a `>-` block give the same
  program, with one exception: `>-` joins lines, so a `//` comment swallows the
  lines after it. `a // first` and `&& b` fold to `a // first && b` (**ran**),
  which evaluates `a` alone. The language keeps `//` because a script without it
  would surprise more people. A host that reads single expressions from YAML
  should refuse line comments there; the parser reports them.
- **YAML mistakes are loud.** An unquoted ternary or map literal is a YAML error
  because of `: ` or a leading `{` (**ran**). A ` #` truncates the value
  silently (**ran**), but the language has no `#` token, so it can only sit in a
  string, and the remainder then fails to compile.
- **An `if` needs no quotes** unless a branch holds a map literal. `when: if (a)
  { 1 } else { 2 }` is a plain YAML value. With `{id: 1}` inside a branch it is
  a YAML error, because of `: ` (both **ran**, PyYAML). A `|` block holds one
  written over several lines (**ran**).
- **A `try` needs no quotes either,** with the same exception. `when: try {
  int(input.count) } catch { 0 }` is a plain YAML value, and so is one with
  `catch (err)`. With `{id: 1}` in the handler it is a YAML error (all **ran**,
  PyYAML).
- **A projection needs no quotes:** `when: orders[*].lines[*].sku` is a plain
  YAML value (**ran**). A `*` is a YAML alias only at the start of a value
  (**ran**), and no expression starts with one.
- **A text block needs lines.** In a `|` block it works: YAML removes its own
  indentation first, and the text block's indentation is counted in the program
  text that is left (**ran**). In a `>-` block the lines are joined, so the
  opening delimiter no longer ends its line, and the program fails with
  `text-block-open` (**ran**: the folded text is `""" a b """`). The failure is
  loud. A plain value that starts with `"""` is a YAML error (**ran**). A host
  that reads programs from folded scalars cannot offer text blocks (`08` 2.9).
- **A raw string fits a field of one line.** Both `r"..."` and `r'...'` exist,
  so a value in YAML's single quotes can hold `r"\d+"`, and a plain value that
  starts with `r"` or `r'` is read as written (both **ran**). YAML's double
  quotes are the exception: there a backslash is YAML's own escape, so `"r'\d+'
  in x"` is a YAML error (**ran**). `08` 2.9 says "and the reverse", which holds
  only for a raw string with no backslash.
- **JSON is valid:** a model can paste any JSON value. There are four
  exceptions, each an error and never a second meaning (`12` 3.2): a key written
  twice (`duplicate-key`), an integer outside ±(2^53 − 1) written with no
  fraction (`literal-range`), an escape of a lone surrogate (`bad-escape`), and
  nesting past 100 levels (`nesting-exceeded`).
- **A YAML author's habits** that are errors here have targeted errors (3.2): a
  `#` comment, a bare key such as `max-retries`, and a bare word for a string,
  which is an unknown name. `12` 3.8 has the full list for a config file.

### 3.4 The TypeScript lean (C6): what is not TypeScript's spelling, and why

The syntax is TypeScript's wherever TypeScript has the construct. Each row below
is a place where it is not. A break with v2 or expr is not a cost by itself
(C7), so none of these reasons is compatibility.

| v3 | TypeScript | Why v3 keeps its spelling |
| --- | --- | --- |
| `if` and `else` form an expression with a value, and `else` is required | a statement with no value; `else` is optional | Everything has a value (E5, D35), and an `if` with no `else` would hide a null (D37; question 10 of 8.1) |
| After `if (c)` and after `else`, `{ a: 1 }` is an error | a block that holds a labelled statement | 2.10: a branch that looks like a map is an error, not a guess |
| `x not in y` | `!(x in y)` | It removes the trap `!x in y`, which is an error here (2.2), and it reads as the sentence. It costs one reserved word |
| `match` | none: `switch` is a statement with fall-through | 2.10. TypeScript has no conditional expression that checks its cases for coverage |
| `try { e } catch (err) { f }` is an expression with a value. One `catch`, required. No `finally`. The parentheses hold a name | a statement with no value; `finally`; `catch` may be left out when there is a `finally`; `catch ({message})` | Everything is an expression (E5), and nothing has an effect for a `finally` to undo. A pattern in the parentheses can be added later (D48; `10` section 7) |
| `fail(message, data?)` | `throw e` | `throw` is a statement. `fail` is a call typed `never`, so it stands wherever a value may: `c ? x : fail("m")` |
| `"""` ... `"""`, a text block | none: a template literal of several lines keeps the indentation of the program around it | Text of several lines needs its indentation removed, and a library `dedent` would run after interpolation and on every evaluation (D46; `08` section 7) |
| `r"..."`, `r'...'`, `r"""` | `` String.raw`...` ``, a tagged template | A tag is a function used as a value, and there is none here (D34). The prefix is one character and a syntax error in TypeScript today |
| `xs[*].f`, a projection | `xs.map(x => x.f)`, and `flatMap` for each nested list | A path through nested lists with no lambda and no made-up name at each level, which no library entry gives. `map` stays, and means the same (D51; 2.8) |
| `_` binds nothing and may repeat | `_` is an ordinary name; `[a, , c]` skips an element | `(_, i) => i` is already what TypeScript authors write. The empty slot is easy to misread and is an error here |
| `+`, `<` and `==` never convert | they convert | Y2 |
| No type annotations | `(x: number) => ...`, `const n: number = ...` | Out of 3.0, with the syntax reserved. Question 8 of 8.1 |
| Function names such as `len(x)` and `int(x)`; namespaces such as `math` | `x.length`, `Number(x)`, `Math` | `06` owns names. D7 settled JavaScript's names for methods and kept short names for functions |
| `x is T` as an operator that tests any type | `x is T` only as the result type of a guard the author writes; the tests are `typeof`, `instanceof` and `in` | One test, for every type, that narrows (2.17). The word is TypeScript's own for "this value has that type" |
| Type text after `as`: `int`, `float`, `bool`, `map<T>`, an exact record | `number`, `boolean`, `Record<string, T>`, an open object type | `03` 2.2 owns these and has the reasons |

Two rows have left this table. `null` is TypeScript's spelling of the absent
value, and `nil` is gone (D28). `const` is TypeScript's word for a binding that
never changes, and `let` is reserved (D36). Type text, where a diagnostic shows
it, is in TypeScript's spelling as well (D29, `03` 2.2).

TypeScript forms that v3 leaves out are compile errors, and each can be added
later without a break: binary and octal literals, `.5` and `5.`, the escapes
`\x41`, `\0` and `\v`, Unicode identifiers (P3), nested patterns, spread
arguments, rest parameters, `x?.()`, an `if` with no `else`, `type` lines and
annotations (a minor version, D40), a `\` before a line end that joins the lines
of a template (**ran**, Node 22; 08-6), a pattern in `catch ( )`, imports, whose
word is reserved for a minor version (`12` 3.9), and loops, reassignment and
early `return`, whose words are reserved for that purpose (overview X6). Two
TypeScript forms are not planned: `finally`, and tagged templates.

## 4. Dispositions

### 4.1 Risor v2

Paths are under `v2:pkg/ast/` unless they start with `parser/` or `lexer/`.
"Restored" means the first draft removed it and this one keeps it.

| v2 node or feature | Where | v3 | One-line reason |
| --- | --- | --- | --- |
| `Program` | `program.go:11` | change | `const` bindings and one result |
| `Ident`, `Bool` | `expressions.go:12`, `literals.go:60` | keep | |
| `Nil` (`nil`, `null`) | `literals.go:42` | change | the node is `Null` and the one spelling is `null`. v2 accepts both words; `nil` is now a targeted error (D28). TypeScript and JSON say `null` (C6), and v2's users are few (C7) |
| `Int`, `Float` | `literals.go:14`, `:28` | change | one `Number` node; exponents and `_` added; binary and leading-zero octal removed (no consumer; `010` is 8 in v2, **ran**) |
| `String` | `literals.go:98` | change | escapes of 2.2; template is its own node and may nest. New: the raw prefix `r` and the text block (D46). v2 has no raw string, and its backtick is the template (`lexer/lexer.go:364-372`) |
| `List` | `literals.go:264` | keep | |
| `Map`, `MapItem` | `literals.go:288-295` | change | keyword keys; computed keys added (no other way to build a map keyed by data); duplicate literal key is an error |
| `Spread` | `expressions.go:53` | change | in list and map literals; not in arguments (the checker cannot count them, and a list parameter says the same) |
| `Prefix` | `expressions.go:26` | change | `!` on any value, giving a `bool`; `-` on numbers |
| `Infix` | `expressions.go:79` | change | `&&` above `\|\|` (one level in v2, `parser/precedence.go:12`; **ran**); semantics of 2.5 to 2.7. `&&` and `\|\|` return an operand, as in v2 (`v2:pkg/object/operations.go:60-74`) |
| Truthiness | `v2:pkg/object/object.go:99-100` | keep, narrowed | the same falsy set for the absent value (v2's `nil`, v3's `null`), booleans, numbers, strings, lists and maps (`v2:pkg/object/list.go:538`, `map.go:449`). Every opaque value is truthy, where v2 asks each type: a zero time is falsy there (`v2:pkg/object/time.go:164`) |
| `**` | `parser/precedence.go:49` | restored | one kind of number removed the objection; whole exponents only (2.5); `-2 ** 2` is an error, as in JavaScript, not `-4` as in v2 (**ran**) |
| `& \| ^ << >>` | `parser/precedence.go:44-48` | remove | no consumer, and `a & b` is nearly always a typo for `&&` |
| `If` | `expressions.go:113` | restored, changed | an expression, as in v2, with the parentheses, the braces and now the `else` required (D35, D37). v2's `else` is optional (`v2:pkg/parser/expressions.go:375-398`). A branch is a block body. It shares the node `Cond` with `? :` (2.3) |
| `Match`, `MatchArm`, patterns | `expressions.go:399-454` | restored, changed | literal patterns with `\|`, exhaustiveness, a condition form; v2's patterns are arbitrary expressions and its guards cannot see the subject (**ran**) |
| `Call` | `expressions.go:149` | change | the target is a name, and the name is a `const`-bound lambda or a function of the environment, never a parameter (D34) |
| `GetAttr` | `expressions.go:181` | change | any word; never a method (`v2:pkg/object/map.go:217-226`) |
| `ObjectCall` | `expressions.go:246` | keep | |
| Optional chaining | `expressions.go:185`, `:250` | change | skips the rest of the chain (B4); v2 is per access, and `5?.a` is nil in v2 (**ran**) |
| `Index` | `expressions.go:276` | change | `?.[` added; whole-number index |
| Index on strings | | restored | text is code points (T1), so `s[0]` has one meaning |
| Negative indexes | | remove | a trap: a computed index that goes negative reads a real element. JavaScript gives `undefined`. Use `xs.last()` or `slice(-1)` |
| `Slice` (`x[a:b]`) | `expressions.go:303` | remove | a second way to write `.slice(a, b)`, not JavaScript, and `:` inside brackets competes with the ternary |
| `Pipe` (`\|>`) | `expressions.go:212` | remove | a second call syntax; method chains are the pipeline |
| `In`, `NotIn` | `expressions.go:337`, `:368` | keep | as `Binary` operators |
| `Func` (arrow) | `literals.go:221` | change | the value of a `const`, or an argument of a library function or method (E6, D34). In v2 a function is a value and goes anywhere |
| Default parameters | `literals.go:78` | restored | a `const`-bound helper needs them; same rule as pattern defaults |
| Rest parameters | `parser/literals.go:564` | remove | need spread arguments; a list parameter says the same |
| Destructuring parameters | `literals.go:136`, `:183` | restored | E3; one level, with defaults and a rest name |
| `ObjectDestructure`, `ArrayDestructure` | `statements.go:85`, `:133` | restored | as `const` patterns; a longer list is not an error (it is in v2, **ran**) |
| `MultiVar` (`let a, b = xs`) | `statements.go:42` | remove | a second way to write `const [a, b] = xs` |
| `Var` (`let`) | `statements.go:12` | remove | v2's `let` can be reassigned. Nothing here can, so the binding is written `const` (D36). `let` and `var` are reserved, each with a targeted error |
| `Const` | `statements.go:170` | keep, changed | the one binding form (D36): in any block or branch; takes a pattern; ends with `;` |
| `Block` | `statements.go:229` | restored, changed | an expression that starts with `const`, and the body of an `if` branch; nothing in it exists only for its effect |
| named `function` | `parser/literals.go:491` | remove | `const f = x => ...` is the one way; no recursion (E6). The word is reserved |
| `Return` | `statements.go:198` | not in 3.0 | a block's value is its last expression. Early `return` is deferred and the word is reserved (overview X5) |
| `Assign`, `Postfix` | `statements.go:261`, `:298` | not in 3.0 | E5. Reassignment is deferred together with loops, and `let` is reserved for it (overview X3, X4) |
| `SetAttr` | `statements.go:319` | remove | E5: no value can be changed in place |
| `Try`, `Throw` | `statements.go:346`, `:394` | `Try` kept, changed; `Throw` removed | `try { } catch (err) { }` is an expression with a value, as it is in v2, where `try` is parsed as a prefix expression (`parser/parser.go:175`). v2 also accepts `catch e { }` without parentheses, a `finally`, and a `try` with only a `finally` (`parser/statements.go:381`, `:423-425`). Here one `catch` is required, there is no `finally`, the handler is a branch, and a library `fail` replaces `throw` (D48). Revisions one to six had the call form `try(e, fallback)` |
| Template strings | `parser/literals.go:92-171` | change | escapes defined; nesting allowed; rendering of 2.13. A text block is a second form of the same node (D46) |
| Significant newlines | `parser/parser.go:654-749` | remove | a YAML `>-` block changes them |
| `//`, `/* */` | `lexer/lexer.go:178-189` | restored | everywhere; `#!` is the CLI's business, not the language's |
| `SyntaxConfig` | `v2:pkg/syntax/config.go` | remove | one language |
| `MaxDepth` | `parser/parser.go:46-52` | change | 500 and settable in v2, with every link of a chain counting. 100, fixed by the specification, with chains not counting (2.16) |

### 4.2 expr

| expr feature (`spec:` lines) | v3 |
| --- | --- |
| Integer forms: hex, octal, binary, `_` (25-32) | decimal and hex, with `_`; a literal outside the whole range is a compile error |
| Rune literal `'a'` (41-43) | dropped; single quotes are strings |
| Raw string `` `...` `` (47-48) | backticks are template strings. The raw string returns with another spelling: `r"..."`, `r'...'` and `r"""` (D46) |
| `true`, `false`, `nil` (52-53) | `true` and `false` are the same. expr's `nil` is written `null`, and `nil` is a targeted error (D28) |
| Go precedence (62-70) | 2.2; the same order for the operators both have |
| 64-bit integers with checked overflow (78-87) | whole numbers to 2^53 − 1 with checked overflow; `/` is true division |
| `&&`, `\|\|` return an operand (98-113); truthiness, `bool(v)` (216-230) | same operators and the same falsy set, with every opaque value truthy; `??` added for null only; `bool(v)` stays (`06`), and `!!v` means the same |
| Pipeline `x \| f(y)` (115-191) | dropped; `x.f(y)` for a method, else `f(x, y)` |
| Unary `!`, `-`, `+` (193-197) | `!` by truthiness; `-`; unary `+` dropped |
| Loose numeric equality (199-214) | one kind of number; lists and maps also compare |
| Names resolved at run time (232-246) | resolved at compile time |
| Selectors; keyword field names (283-302) | same |
| Index; string index by code point; no negatives (307-320) | same |
| Call target is a name or selector (327-337) | same |
| `map`, `filter` and the other forms with `it`, `index` or a named element (488-582) | methods that take an arrow function. A `map` whose body is only a path may be written as a projection, `xs[*].f` (D51) |
| `try(value, default)` (631-650) | `try { value } catch { default }` (D48). `try(` is a targeted error that says so. The checker does not relax inside it |
| `if(cond, then, else)` (652-665) | `cond ? then : else`, `if (cond) { ... } else { ... }`, or `match` |
| `?.` (680-705) | skips the rest of the chain; expr is per access (**ran**) |
| `?[` (680-705) | spelled `?.[` |
| No `??` (722-726) | `??` exists, beside `\|\|`: one replaces null, the other any falsy value |
| `[]any{...}`, `map[string]any{...}` (926-939) | dropped |
| JSON-style literals (941-960) | same, but a bare key in `{a: 1}` is the string `"a"`; expr evaluates `a` (**ran**). A key from a value is `{[a]: 1}` |
| `${...}` templates and their rendering (962-997) | 2.13; the same rendering except that numbers and string escapes are now exact |
| `Identifiers`, `Calls`, `Literal` (1081-1145) | host API over the nodes of 2.3 |

## 5. Examples

Method and function names are those of `06-stdlib.md`.

| # | Expression | Meaning |
| --- | --- | --- |
| 1 | `input.event.pull_request.head.sha` | a field path |
| 2 | `gate.failed > 0` | a condition |
| 3 | `!poll.done` | not done; null, zero and an empty value count as not done too |
| 4 | `input.role in ["admin", "owner"]` | membership |
| 5 | `input.user.id not in blocked` | |
| 6 | `"urgent" in issue.labels[*].name` | membership in a derived list: a projection (2.8). `issue.labels.map(l => l.name)` says the same |
| 7 | `"error" in charge` | the map has the key |
| 8 | `review?.verdict` | null if the step did not run |
| 9 | `run.error?.message ?? "ok"` | default for null |
| 10 | `ack_manager?.by ?? ack_secondary?.by ?? ack_primary?.by` | the first that is not null |
| 11 | `(report?.count ?? 0) > 3` | `??` beside a comparison needs the parentheses |
| 12 | `a.n != null && a.n > 3` | the null test narrows `a.n`, and the result is a `bool`. `a.n && a.n > 3` narrows too, but gives null or `0` when `a.n` is falsy, so its type is `bool \| int \| null` |
| 13 | `error.type == "timeout"` | a keyword as a field name |
| 14 | `input.headers["x-request-id"]` | a key that is not a word |
| 15 | `input.labels?.[0]?.name ?? "unlabeled"` | safe on null, an empty list, a missing field |
| 16 | `fetch.tests.filter(t => t.status == "fail").map(t => t.name)` | filter, then map |
| 17 | `len(verify.filter(v => v.confirmed)) >= 2` | count |
| 18 | `[...correctness.items, ...security.items]` | concatenate |
| 19 | `{...input.user, role: "admin"}` | an updated copy |
| 20 | `run.status == "completed" ? "Review posted" : "Review failed"` | two-way choice |
| 21 | `gate.failed / (gate.failed + gate.passed) > 0.1` | a ratio; true division |
| 22 | `` `Review of #${pr.number}: ${len(findings)} findings` `` | text from numbers |
| 23 | `(try { int(input.count) } catch { 0 }) > 10` | fall back when the value is bad. As an operand a `try` needs the parentheses |
| 24 | `decide("Is this refund legitimate?", intake.body) > 0.9` | a call the host can find statically |

Restored expressiveness:

| # | Expression | Shows |
| --- | --- | --- |
| 25 | `match classify.kind { "bug" => "fix", "feature" \| "idea" => "plan", "question" => "answer" }` | cases, alternatives, exhaustive without `_` |
| 26 | `match run.status { "completed" => "Posted", "failed" => "Failed: " + (run.error?.message ?? "unknown"), _ => "Cancelled" }` | a result that only makes sense for its case |
| 27 | `match { gate.failed == 0 => "pass", gate.failed <= 2 => "warn", _ => "fail" }` | conditions in order |
| 28 | `match input.priority { null => "normal", 1 \| 2 => "high", _ => "low" }` | null and numbers as cases |
| 29 | `orders.map(o => { const total = o.qty * o.price; {id: o.id, total, big: total > 100} })` | a block in a lambda; the total is computed once |
| 30 | `labels.entries().map(({key, value}) => key + "=" + value).join(",")` | a map pattern in a parameter |
| 31 | `{ const [owner, repo] = input.repository.full_name.split("/"); {owner, repo} }` | a list pattern |
| 32 | `{ const {password, token, ...safe} = input.user; safe }` | remove fields |
| 33 | `{ const {retries = 3, backoff = "1s"} = input.options ?? {}; {retries, backoff} }` | defaults for absent or null fields |
| 34 | `min(60, base_delay * 2 ** (attempt - 1))` | exponential backoff |
| 35 | `round(ratio * 10 ** 2) / 10 ** 2` | round to two places |
| 36 | `items.reduce((m, it) => {...m, [it.id]: it.status}, {})` | a map keyed by data |
| 37 | `input.event?.pull_request.head.sha` | one `?.`: null when there is no event |
| 38 | `input.name[0].toUpperCase() + input.name.slice(1)` | string index |
| 39 | `size > 5_000_000 ? "large" : "ok"` | a readable constant |
| 40 | `` `${len(failed)} failed${failed ? `: ${failed.join(", ")}` : ""}` `` | a template inside a template; a list as a condition |

Truthiness (2.7):

| # | Expression | Shows |
| --- | --- | --- |
| 41 | `user.name \|\| "anonymous"` | a default for a null name and for an empty one |
| 42 | `input.retries ?? 3` | a default for null only: a `0` the caller sent stays `0` |
| 43 | `!results.items` | no items: the list is null or empty |
| 44 | `issue.labels.filter(l => l.color)` | a predicate by truthiness: the labels with a color that is not null or empty |
| 45 | `match { errors => "fail", warnings => "warn", _ => "pass" }` | lists as conditions: the first that has elements |
| 46 | `review?.approved && !review.dismissed` | a guard. The result is null when the step did not run, so its type is `bool \| null`. A host that asks for `bool` rejects it. One that asks for `condition` accepts it (D33) |
| 47 | `!!input.dry_run` | a `bool` from any value, for a host that asks for one (B8) |

`if` (2.10) and `const`-bound lambdas (2.9):

| # | Expression | Shows |
| --- | --- | --- |
| 48 | `if (gate.failed == 0) { "pass" } else if (gate.failed <= 2) { "warn" } else { "fail" }` | example 27 as an `if` chain: the same meaning |
| 49 | `if (!review) { "skipped" } else if (review.verdict == "APPROVE") { "approved" } else { const n = len(review.findings); `${n} findings to fix` }` | the first test narrows: `review` is not null in the later branches. A `const` in a branch |
| 50 | `orders.map(o => if (o.qty > 100) { {...o, tier: "bulk"} } else { o })` | an `if` as a lambda body; a map as a branch's value, inside the branch's braces |
| 51 | `(if (input.dry_run) { [] } else { plan.steps }).map(s => s.name)` | an `if` as a chain receiver needs parentheses |
| 52 | `{ const name = t => t.name.trim(); [...failed.map(name), ...flaky.map(name)] }` | a `const`-bound lambda passed by name to a library method, twice |

`as` and `is` (2.17). `Order` and `Config` are names the host declared:

| # | Expression | Shows |
| --- | --- | --- |
| 53 | `json.parse(msg.text) as {id: string, total: float, ...}` | untyped data into a type. The `...` allows other keys; without it they fail |
| 54 | `(json.parse(resp.body) as Order).lines.map(l => l.qty * 2)` | a named type. The parentheses are needed before a chain |
| 55 | `if (body is Order[]) { len(body) } else { 0 }` | `is` in an `if`: `body` is an `Order[]` in the first branch and still `any` in the second |
| 56 | `match input.kind as "bug" \| "feature" { "bug" => "fix", "feature" => "plan" }` | a string into a literal set, so that `match` can check the cases |
| 57 | `try { input.config as Config } catch { defaults }` | a fallback when the data does not conform |
| 58 | `xs[(len(xs) / 2) as int]` | a `float` into an `int` without rounding: it fails unless the number is whole |
| 59 | `input.limit is int && input.limit > 0` | a test and a use; the `is` narrows `input.limit` for the comparison |

Projections (2.8), `catch` with the error as data (2.12), raw strings (2.2) and
tagged unions (2.15). `Shape` is a name the host declared, a circle or a
rectangle told apart by `kind`, as in `09`, and `s` is a `Shape`:

| # | Expression | Shows |
| --- | --- | --- |
| 60 | `orders[*].lines[*].sku` | every sku ordered: two nested lists read as one flat list, with no lambda |
| 61 | `orders[*].lines[*].qty.sum()` | a method call ends the projection and receives the list |
| 62 | `orders.filter(o => o.status == "open")[*].id` | a star after a method call |
| 63 | `input.labels?.[*].name ?? []` | `?.[*]`: null when there are no labels, and then the default |
| 64 | `` try { json.parse(msg.text) as Order } catch (err) { fail(`order: ${err.message}`, {path: err.path}) } `` | the error as data: a new failure with the author's message, which carries the path of the first as data |
| 65 | `try { int(s) } catch (err) { match err.code { "invalid-argument" => 0, _ => fail(err.message) } }` | a choice by code. The checker knows the set of codes, so a misspelled one is a compile error |
| 66 | `r"C:\builds\new"`, `r'^\d{4}-\d{2}$'` | raw: a backslash is a backslash, in either quote |
| 67 | `` `${r"\d+"} matched ${n} times` `` | a raw piece inside a template |
| 68 | `s.kind == "circle" ? 3.14159 * s.r ** 2 : s.w * s.h` | a test of the tag narrows: `s.r` reads only where `s` is a circle |
| 69 | `match s.kind { "circle" => s.r * 2, "rect" => s.w + s.h }` | a `match` on the tag, checked for coverage: a third variant makes it a compile error |
| 70 | `[{kind: "circle", r: 1}, {kind: "rect", w: 2}]` | a config value. With `Shape[]` as the expected result, the check error is on the second element, which has no `h` (2.15) |

Text blocks (2.2) need lines, so they are shown as a script:

```js
// The two spaces before the closing delimiter are removed from every line.
// The value starts at "Review" and has no line feed at its end. Its third
// line is empty, and its fourth starts with two spaces.
const prompt = """
  Review pull request #${pr.number}.
  Reply with one of: ${verdicts.join(", ")}.

    - Quote the line you object to.
  """;

// Raw: nothing is an escape, and `${` is two characters.
const step = r"""
  echo "${HOME}" \
    | tr -d '\n'
  """;

{prompt, step}
```

A script, which is the same language. No example passes a lambda to a
`const`-bound lambda, and none calls a parameter (D34): `pct` below is called by
name with numbers, and every lambda written as an argument goes to a library
method.

```js
// Summarize a test run.
const failed = fetch.tests.filter(t => t.status == "fail");
const pct = (n, digits = 0) => `${round(100 * n * 10 ** digits) / 10 ** digits}%`;
const verdict = match {
  !failed                              => "pass",
  len(failed) / len(fetch.tests) < 0.1 => "flaky",
  _                                    => "fail",
};
{
  verdict,
  summary: `${len(failed)} of ${len(fetch.tests)} failed`,
  rate: pct(len(failed) / len(fetch.tests), 1),
  slowest: failed.sortBy(t => t.ms, "desc").slice(0, 3)
    .map(({name, ms}) => `${name} ${ms}ms`),
}
```

## 6. Alternatives rejected

| Alternative | Why not |
| --- | --- |
| v2's statement language (`function`, `return`, assignment, loops, `throw`) | E5 and E6. Blocks, `if`, `match`, patterns, `const`-bound lambdas and `try` with `catch` give what those statements were used for, as expressions. Loops, reassignment and early `return` are not in 3.0; the words are reserved (overview X3 to X6). |
| No `if` and `else` (the first four revisions) | Reversed by the owner (D35). 2.10 has the reasons and the rules. |
| `let` as the keyword for a binding (the first four revisions) | Reversed by the owner (D36). In TypeScript `let` says the binding will be reassigned, and nothing here can be. The earlier case for `let` was that it is one keyword, valid TypeScript, and the block rule's word. `const` is all three. |
| Two forms, expression and script (the first draft) | With blocks, the script is a block without braces. A second form would be a difference to learn for no gain. |
| `const` only at the top (the first draft) | A lambda body or an arm could not name an intermediate value, so authors repeated sub-expressions. |
| Ternary as the only conditional (the first draft) | 2.10: a ternary chain repeats the subject and cannot be checked for coverage. |
| `match` arms that bind names, or take guards | 2.10: the bind-or-compare trap. The condition shape covers guards. Structural patterns can be added later without changing what is here. |
| Nested patterns | Each level is a field read the author can write; the type and error rules double. Addable later. |
| `**` as a general power function | 2.5: not the same last digit in every runtime. |
| Swift's precedence for `??`, or JavaScript's | 3.1: each silently disagrees with half the authors. |
| Per-access `?.` (the first draft, v2, expr) | B4. The syntax is JavaScript's, so its meaning is too. |
| A nesting limit by tree depth, by brackets alone, or left to each parser | 2.16. |
| Accepting `===`, `let`, `var` or `nil` as synonyms | One way to write each thing; an error that names the fix costs one round trip. |
| `nil` and `null` both accepted (the first three revisions), or `nil` alone | Decided the other way by the owner (D28). Both were kept "because each has a large body of existing text". That body is small (C7). TypeScript and JSON say `null` (C6). The JSON form of the value already prints `null` (2.13), so `nil` alone would write one word and print another. And two spellings were the last exception to "one way to write each thing". The cost is that Risor has said `nil` since v1. The targeted error pays it in one round trip (3.2). |
| A lambda as an argument of any call, and a parameter that can be called (the first three revisions) | E9: `const om = x => x(y => x(y)); om(z => om(w => z(w)))` never returns, so E6 held only up to a limit. D34 closes it (2.9, 2.14). The cost is a helper that takes a function, such as `const twice = (f, x) => f(f(x))`. |
| Reporting the two errors of D34 in the parser | 2.9: they are rules about bindings, half of the rule needs the environment, and nothing runs unchecked. |
| Strict booleans: a condition must be a `bool` (the second revision) | The owner's decision (B1). `user.name \|\| "anonymous"` and `x && x.y` are what an author of TypeScript-shaped text writes (C6), and each was a compile error with a longer rewrite (C4). What strictness bought is kept another way: a host that needs a boolean checks the result type (B8), and the checker reports a condition that can never be falsy (B7). |
| JavaScript's falsy set exactly: an empty list and an empty map are truthy | B5 and 2.7: `when: results.items` would be true with no items; the rule would need a second sentence; and a program's result would depend on whether a host supplied a null collection or an empty one. |
| Truthiness that an opaque value's host type decides, as in v2 | An operator would then run host code. A host that wants "empty" for its type declares a method. |
| Implicit `it`; a pipe operator; uniform call syntax | 2.9; method chains are the pipe; `x.f(a)` meaning `f(x, a)` would let a new library method change which function a stored program calls. |
| Unicode identifiers | The identifier tables differ between runtimes and versions (P3). Any key is reachable with `x["..."]`. |
| `as` trusted and free, as in TypeScript; a type's name as a function, `Order(x)`; a function with the type in a string | `07` section 8. |
| `as` and `is` binding tighter than arithmetic, so that `x as int + 1` needs no parentheses | Then `a + b as T` would test `b`, against TypeScript's reading. An error that names the parentheses is the safer difference. |
| Reading `x as T[0]` as an index into `x as T` | `[` would continue a type or end it depending on the token after it, and `x as T[]` beside `x as T[0]` would differ by one character and mean unrelated things. |
| Type text outside the 100 levels, with its own maximum at parse time | 2.16: a second number, and a parser's recursion would be bounded by their sum. |
| `==` on opaque values by a comparison the host defines | An operator would then run host code. Identity is enough to keep `==` total (overview S11); a host that wants another equality exposes a field or a function. |
| `try(e, fallback)`, a call form (revisions one to six) | Reversed by the owner (D48). The fallback could not see what had failed, so it answered every failure alike. `try { e } catch (err) { f }` is the form a TypeScript author knows (C6), and it is v2's. The price is length: `(try { int(s) } catch { 0 })` against `try(int(s), 0)`. A short second form, such as `e catch d`, is not planned: one way to write a thing is worth the 14 characters (`10` section 7). |
| A `class` field on the error value; an error as an eighth kind of value; a trace or a span on it; several `catch` clauses; `finally` | `10` 2.3 and section 5. The class depends on what the checker inferred, and a program must not see that. `match err.code` in the handler does what several clauses would, with its cases checked. |
| `catch` that catches by class, so that a broken promise of the host passes through | 2.12 and `10` 2.4: an evaluator with no checker cannot tell `dynamic` from `environment`, so two runs on the same values would differ. `catch` decides by code. `missing-variable` is the one such failure with a code of its own, and it passes. |
| A library `dedent(s)`; removing indentation from a backtick template; `'''`; a node kind for each string form; a prefix on a backtick | `08` section 7. `dedent` runs after interpolation, so a value of several lines changes what is removed, and it is charged on every evaluation. A backtick template of several lines is valid TypeScript with its indentation kept (Y1). |
| Untagged unions of records; a tag named in the type text; a Join that makes a tagged union | `09` section 7. The first failure and the step count of a test would depend on the order of the variants. |
| A projection that covers the rest of the chain, method calls too, as `?.` does | 13-2. `orders[*].lines[*].qty.sum()` would need parentheses. A method call ends a projection and receives the list. |
| Mapping implied by `.`; a filter step, `xs[c]` or `xs[?c]`; a star on a map; dropping nulls from a projection | 2.8 and `13` 6.1. `xs[c]` is valid TypeScript with another meaning. The rest are the surprises of other path languages: a result whose shape depends on its length, and a mistake that becomes nothing. |
| Comprehensions, `[for (x of xs) ...]`, and a query expression, `from x in xs ...`, in 3.0 | `13` 6.1 (D51). Comprehensions may come in a minor version, on evidence. `[for` and `{for` stay errors until then, so it is an addition. A query expression is not planned. |
| Imports in 3.0 | 12-8: a minor version. `import` is reserved now, and a host layers files by passing one file's value to the next as a variable (`12` 3.9). |
| A test construct, `assert`, or a `test` word | D49: tests are data, in the case format of the suite (`11`). `c ? x : fail("m")` is the guard inside a program. |
| One `parse` code for every syntax error, or one for each family of targeted errors | 3.2. A conformance case and a host identify a mistake by its code. With one shared code they would need the message, or a template name that no diagnostic carries. |
| One name for a `check` code and the run code of the same mistake, such as `kind` in both phases | 2.12. A code then has two phases and two entries in the catalog. And `maybe-null` cannot name a null that is there. |

## 7. Rulings relied on, and what is left to the other documents

| Ruling | Where this document uses it |
| --- | --- |
| C1, P1-P4, R1 | Section 2 is written as a definition for three implementations; 7.1 lists what a conformance suite must cover; 8.2 to 8.4 are the notes for each. |
| N1-N4, N6, N7 | 2.5. N2 is stated on operands. N7's candidate is adopted. |
| N5 | A literal is `int` when its value is whole (2.5), whatever its spelling. |
| B1, B5, B6, D30 | 2.7: the falsy set, confirmed, kind by kind; `&&` and `\|\|` return an operand; conditions and predicates are tested by truthiness. |
| B7, D32 | 2.15 gives what the language expects of the checker; `03` defines it. The two codes are the class `constant-condition`, which a host may take out. |
| B8, D33 | 2.1 and 2.7: a host that needs a boolean passes `bool` as the expected result, and one that tests by truthiness passes `condition`. |
| B2-B4 | 2.8. |
| E1-E6 | 2.10; 2.1; 2.9; 2.5; nothing exists only for its effect; no recursion. |
| E9, D34 | 2.9: a lambda is passed only to a library function or method, and a parameter cannot be called, with the codes `call-of-parameter` and `lambda-to-lambda`. 2.14: termination by construction, and the bound on calls in progress. 2.16: what it simplifies. |
| X1, D35, D37 | 2.2, 2.3, 2.10 and 2.16: `if`, `else if` and `else` as an expression, on node 16. D37 and D38 are questions 10 and 11 of 8.1. |
| X2, X6, D36 | 2.1 and 2.2: `const`, and the reserved words. 3.2 has their targeted errors. |
| X3-X5 | Loops, reassignment and early `return` are not in 3.0. This document says "not in 3.0" and not "never". |
| L2, L3 | 2.14 gives the constructs and sizes; `04` assigns the units. A truthiness test adds no step (2.7). |
| R5, D31 | 2.16: what a level is, the number, the diagnostic, and the rule that combines it with the depth limit. |
| T1, T2 | 2.6; map order in 2.4 and 2.13. |
| S1, S3 | Namespaces in 2.1 and 2.3; lambdas and host functions in 2.9. |
| C6, Y1, Y2 | 3.1 lists the nine exceptions to Y1; 3.4 lists every spelling that is not TypeScript's, with its reason. |
| Y3, D28 | Applied. `null` is the keyword and `nil` is a targeted error (2.2, 3.2). Node 9 is `Null` and the kind is `null` (2.3, 2.4). Rendering is unchanged (2.13). |
| Y4, D29 | Type text in this document is in TypeScript's spelling (2.15, section 5, 8.1). `03` 2.2 owns the grammar. |
| C7, Y5 | No reason in this document is "v2 or expr does this". Section 4 stays as the record of what changes. |
| V1-V5, D39-D45 | 2.2: `as` and `is` at the level of `Rel`, where the type ends, the reserved words. 2.3: node 31. 2.16: type text counts toward the levels. 2.17: the two operators and how their text is read. 3.1 and 3.2. `07` is the authority for conformance, cost, named types and the host's function, and none of that is restated here. |
| W1, D46 | 2.2: the five string forms, the tokens and rules of the text block and the raw prefix, longest match. 2.3: `form` on nodes 6 and 7, and the rule that a literal with no `${ }` is a `String`. 2.13, 2.16, 3.2 to 3.4. `08` is the authority for the worked values, the host strings and the formatter. |
| W2, W3, D47 | No grammar, node or word. 2.15: what the language expects of the checker for a tagged union and for a literal's fit through its form. 2.17: a tagged union in type text, and the reading row. 3.1, 3.2: `untagged-union`. `09` is the authority, and `03` holds the rules. |
| W4, W5, D48 | 2.2: `Try` at the level of `If`, and the reserved words `catch`, `throw`, `finally`. 2.3: the fields of node 26. 2.12: the form, the error value, what `catch` catches by code, `fail`, and the names of the run codes. 2.14, 2.16. 3.1: two rows. 3.2: seven rows, and a name for every `parse` code. `10` is the authority for the cost, the trace, the catalog and the renderings. |
| W6, D49 | 3.2: the hint for `assert(`. Nothing else here: no node, no reserved word, no library entry. |
| W7, D50 | 2.1: a config file is a program. 2.2: `import` is reserved. 3.2: the errors for `import`, `export`, `require`, a `#` comment and a bare key that is not a word. 3.3: JSON as a program, with four exceptions. Imports themselves are not in 3.0 (`12` 3.9). |
| W8, D51 | 2.2: `Star` in `Chain`. 2.3: node 32, `Each`. 2.8: the rules of a projection, 13-2 among them, and B4 inside one. 2.14: a projection among the constructs that create a value, and step 1 of the termination argument. 2.16: a star opens a level. 3.2: four rows. The library entries and the narrowing of `filter` and `find` are `06`'s and `03`'s. |
| Round 8, rulings 2, 5 to 8, 10, 12 | 2.12: a star has a trace entry; a crash passes `catch`, as `04` A.1 has it; `missing-variable` passes; a failure with no checker is `dynamic`; every code has a name. 2.8: a star on a map is an error of kind `kind`, and a null from `?.` inside a projection is kept. |

What this document needs from the others:

| From | Need |
| --- | --- |
| `03` | Cover exactly the nodes of 2.3. Type truthiness as B7 and 2.15 say: `!`, the operand types of `&&` and `\|\|`, narrowing by a truthy test, and the diagnostic for a condition that can never be falsy or never truthy. Conditions and predicate results are any type, not `bool`. `match` exhaustiveness over literal sets and null; blocks; patterns with defaults and rest; `OptChain`; computed keys; `**`. Type a literal by its value. In the expanded nesting of `03` 2.17, count a use of a `const`-bound lambda as 1 plus its nesting depth, as 2.16 counts a call, so the check and the run agree on what a level is. Report `call-of-parameter` and `lambda-to-lambda` from bindings alone (2.9). An `if` is node 16, `Cond`, with `form` `if`: it needs no rule and no number of its own, and a branch with `const` lines is a `Block` (2.3). For node 31: the subset of `Type` that may follow `as` and `is` (2.2); the code for a type written in place that is deeper than 64 once its names are expanded (2.16); `never-conforms`; `is` as a narrowing test. Say whether a record type may end with a comma, as a map literal may. For node 26: the join of body and handler, the type of the `catch` name from the catalog, and sites in the body dropped (`10` 2.6). For node 32: the row of `13` 5.2. For nodes 6 and 7: the type by node, whatever the form. `untagged-union`, a 29th code, and the fit of a literal through its form for `result-mismatch` and `arg-mismatch` (`09`). `filter` and `find` narrow. The class of each run code of 2.12, with the rule for a program compiled with no check. The hints of the rows marked *check* in 3.2. |
| `06` | Callbacks receive `(item, index)`. Only a library signature has a function parameter, and it takes a lambda or the name of a `const`-bound lambda (D34). `type(null)` is `"null"` (D28). A predicate's result is tested by truthiness (2.7). `bool(x)` and `!!x` both give the truthiness of a value as a `bool`. `string(v)` is 2.13 and number parsing is 2.5; `int`, `floor`, `ceil` and `round` fail outside the whole range, not the 64-bit range. `fail(message, data?)`, typed `never` (10-4). The entries of D51, with `sortBy(f, "desc")` as the script of section 5 uses it, and `values()` for the hint of a star on a map. Hints for `assert`, `dedent`, `trimIndent` and `stripIndent`. Keep `slice`, `first` and `last`, since slice syntax and negative indexes stay out. `math.pow` is marked implementation-defined (T3); `math.sqrt` is exact, since IEEE 754 requires a correctly rounded square root, and `**` is exact. If `range` or `repeat` return (E8), 2.14's size bound changes as noted there. The hint for `m.get(k)` is `m?.[k]`. |
| `04` | Evaluate a chain as a unit (2.8). Normalize negative zero. Charge the constructs of 2.14. A default expression, a `match` arm and the handler of a `try` are evaluated lazily. `catch` decides by code (2.12), and `catch (err)` creates the error value at the cost of `10` 2.7. A `String` literal costs 0 steps and 0 units in every form. A projection charges 1 step for each element a star visits and `1 + k` units for its list, before the list is built (`13` 5.2), and a star in progress gives a trace entry. A truthiness test is bounded work and adds no step (2.7). The depth limit counts a call as 1 plus its nesting depth (2.16). A left-associative chain, an else chain of `? :` or of `if`, and the `const` lines of a block or branch run in a loop, not by recursion (2.16). An `if` costs what `? :` costs. No lambda has two calls in progress, and no parameter holds a function (2.14). The walk of `as` and `is`: its order, its steps, and that each level it enters counts against the depth limit like any walk over a value, so a run's recursion stays 2`D` + 100 (2.16). |
| `05` | The parser reports comments. Host strings parse one `Expr` per interpolation (2.13). A host cannot present NaN, an infinity or an out-of-range integer as a number, and malformed text reads as U+FFFD. `nesting-exceeded` joins the `parse` codes, and the maximum is not a host option (2.16). A host that needs a boolean passes `bool` as the expected result, and one that tests by truthiness passes `condition` (B8, D33). The targeted errors of 3.2 are `parse` diagnostics, but for the rows marked *check*, the two of D34 among them. A binding answers "is it empty" for a host list or map without visiting its contents (2.7). The stack an implementation needs grows with the depth limit, so a binding states the largest value it accepts. Named types as a declaration (`07` 2.5). `as` and `is` are reserved, so neither can be the name of a variable or a function, and since D46 to D51 neither can `catch`, `throw`, `finally` or `import`. `not-conforming` joins the `run` codes. A.6 lists the 58 `parse` codes of 3.2 and the six run codes that 2.12 names (`null-operand`, `wrong-kind`, `missing-field`, `index-range`, `divide-by-zero`, `no-match`), and holds the rules of the catalog `messages.json`. It says what a parser reports after its first error. A.7: a segment's end is found by a lexer that reads raw strings. |

### 7.1 What the specification must pin down

Each line is a rule a conformance test needs. All are decided in section 2
unless marked *open*.

1. The token grammar: white space, comments, words, reserved words (those
   reserved for an error among them: `nil`, `let`, `var`, the loop words,
   `return`, `function`, `throw`, `finally`, `import`), number forms, `_`
   separators, every string and template escape, surrogate pairs. Each reserved
   word after `.` and as a bare map key. The name `r` before a quote and
   anywhere else.
2. Precedence and associativity of every operator; the seven rows of
   `needs-parentheses` (3.2): `??` mixing, unary before `**`, a `{`, `if` or
   `try` subject, `!a in b`, `!x is T`, an `if` or a `try` as an operand, and a
   type followed by what cannot follow it.
3. The `{` rule: block if and only if `const` follows; directly after `if (c)`,
   `else`, `try`, `catch` and `catch (err)`, a branch, whatever follows.
4. Evaluation order and which operands are lazy: `&&`, `||`, `??`, `? :`, `if`,
   `match`, the handler of a `try`, defaults, chain steps after a null receiver.
   Which operand `&&` and `||` return, and that it is evaluated once.
5. **Truthiness**, for each kind of value. Falsy: null; `false`; zero, written
   `0`, `0.0`, `0x0` or `-0`, computed as a negative zero, or supplied by the
   host as a negative zero; the empty string, built or supplied; the empty list,
   built or supplied, a host's null list included where the binding presents it
   as a list; the empty map: `{}`, a map read in place from the host with no
   entries, and a record with no field present. Truthy: `true`; every other
   number, the smallest included; every other string, among them `" "`, `"0"`,
   `"false"` and host text that is only malformed units; a list with an element,
   among them `[null]`, `[0]` and `[[]]`; a map with an entry, among them `{a:
   null}` and any record with a field; every opaque value, with no call to the
   host. Each is tested through `!`, `&&`, `||`, `? :`, `if`, a condition arm
   and a predicate. A test adds no step and is bounded work on a host list or
   map of any size.
6. `||` against `??` on null and on each falsy value.
7. Scope: a name is not visible in its own value; one binding per block;
   shadowing; namespaces cannot be shadowed.
8. Numbers: the whole range; exact and IEEE cases per operator; the `**`
   algorithm; zero divisor; non-finite results; a single zero; literal range
   errors.
9. Number to text, and text to number, including round trip.
10. String length, index and order by code point; malformed host text, with one
    U+FFFD for each malformed byte of UTF-8 or unit of UTF-16, not one for each
    run.
11. Equality across and within kinds, including nested values and opaque values;
    no conversion, so `1 == true` is false.
12. `in` for each kind of right operand, including the empty string.
13. Reads: present, absent and wrong-kind, for `.`, `[ ]`, `?.` and `?.[`, on
    maps, lists, strings, null and other kinds.
14. Chains: where short-circuit starts, what it skips, where it ends.
15. Patterns: absent field, short list, non-map, non-list, defaults on absent
    and on null, rest, discard.
16. Lambdas: allowed positions; a `const`-bound lambda passed by name to a
    library method; `call-of-parameter` for a call of a parameter, of a pattern
    name and of a `const` that holds a value, reported even when the lambda
    around it is never called; `lambda-to-lambda` for a lambda written in place
    and for a named one; dropped extra arguments; argument counts for a call by
    name; defaults.
17. `if`: the test by truthiness; only the branch taken is evaluated, and no
    later test; a `const` in a branch and its scope; a map as a branch's value;
    the errors for a missing `else`, a missing parenthesis or brace, a branch
    written like a map, an empty branch, and an operator after the last branch.
    `match`: order, `==` against literals and never truthiness, truthiness of a
    condition arm, exhaustiveness errors, the run-time error when nothing
    matches.
18. Map literals: duplicate literal keys, computed keys, spread order, spreads
    of the wrong kind.
19. `try` and `catch` (2.12). For each run code, that `catch` catches it or
    lets it pass: the five of a limit or a cancellation, `function-panic` and
    `missing-variable` pass, and every other is caught. The four fields of the
    error value for each code. The handler is not evaluated when the body gives
    a value. A failure in a handler. A name bound in the body is not in scope in
    the handler. The name's scope and shadowing. A `try` inside a callback and
    around one. Each of its seven targeted errors, with its span. The steps and
    units with a name and with none are `10`'s cases, owned by `04`.
20. Rendering of every kind, the JSON form, map key order, depth.
21. **Nesting** (2.16). For each thing that opens a level (each bracket, `!`,
    `-`, a binary operator, `?`, a `match` subject, and the parentheses and
    braces of an `if`): 100 levels parse, and 101 are `nesting-exceeded` with
    the span of the token that opened the last. Mixed kinds of level in one
    program. For each thing that does not: a sum of 100,000 terms, a chain of
    100,000 method calls, 100,000 else branches, 100,000 `else if` arms and
    100,000 `const` lines parse and run. The depth limit counts 1 plus the
    nesting depth of each call: a program that reaches the limit exactly, and
    one that passes it by one level, with brackets around the calls and without.
22. **Codes.** Each `parse` code of 3.2, with its span: each row of the two
    tables has a case. Each run code of 2.12, raised by each construct its row
    lists. That a null and a wrong kind in the same place give different codes.
    The names are now given here (P6). The classes are `03`'s and `05`'s, the
    messages are the catalog's (`10` 2.9), and the step and allocation units
    are `04`'s (L6, L7).
23. **`as` and `is`** (2.2, 2.17). Every row of the reading table in 2.17, as a
    parse or as an error. Where the type ends, for each token that can follow
    it. A `{` as a record type wherever a type is expected, and as the arms of a
    `match` after a complete type. `map<map<int>>`, and `>=` directly after
    `map<int>`. One upper-case letter, an all upper-case name of two letters,
    and a lower-case word, each as a type name. A value and a type with the same
    name. `as` and `is` as a field name after `.` and as a bare map key. Each
    targeted error of 3.2. The levels that type text opens, a `[]` suffix among
    them, at 100 and at 101. That `x` is evaluated once, that `as` gives back
    `x` itself, that `catch` catches `not-conforming` and gives its path, and
    that `is` never fails. A tagged union after `as` reads as one type, and two
    records with no tag are `untagged-union` (`09`).
    The conformance rule, the order of the walk, its steps and the path of the
    first failure are the cases of `07` section 4, owned by `03` and `04`.
24. **String forms** (2.2). Every row of the example table of `08` 2.3, with its
    value or its code and span. Each form against the one-line string of the
    same value: `==` is true and the node is a `String`. One block with its line
    ends as U+000A, as U+000D U+000A and as U+000D: one value. Raw: `r"\n"` has
    length 2, `r"\"` is one backslash, and `r"${x}"` checks with no `x`
    declared. Longest match: `""`, `"" + "a"` and `r""`, each against a text
    block. One `text-block-indent` for each bad line, and nothing more reported
    after `text-block-open`. A text block as a map key, a `match` case and a
    literal in type text, each an error. 100 levels through nested text blocks,
    and the 101st.
25. **Projections** (2.8). An empty list. A null receiver with `[*]` and with
    `?.[*]`. A map and a string as receivers: `wrong-kind` at run time and
    `kind` when types are known. A missing field with `.` and with `?.`, where
    the null is kept. Two stars with an empty inner list. An inner `?.[*]` on
    null: one null in the result, and the other elements go on. A list that no
    star iterated is not flattened. A method call that ends the projection, and
    a method of the element after one, which is `no-method`. An index
    expression evaluated once for each element. A star with white space inside
    its brackets. 100 stars in one projection, and the 101st. A call inside a
    projection, at the depth limit and one past it. A failure inside one, with
    the star and the index in its trace.

## 8. Open questions, and notes for implementers

### 8.1 Open questions for the owner

| # | Question | Recommendation |
| --- | --- | --- |
| 1 | **N2 as stated on operands.** N2 is settled (D3). What is open is this document's reading of it: whole-number arithmetic that leaves ±(2^53 − 1) is an error, and arithmetic on other numbers is plain IEEE. So `10 ** 20` and `9007199254740991 + 1` fail while `1e20` and `1e20 + 1` do not. | Accept. The alternative that removes the seam is JavaScript's silent rounding, which N2 exists to prevent. |
| 2 | **A block's value is its last expression, and `return` is an error in 3.0.** A JavaScript author will write `return` once. | Keep for 3.0. Early `return` is deferred by the owner, not refused (overview X5): the word is reserved, and the message says "not in 3.0". What stays ruled out is a `return` that works only on the last line. It would promise an early exit and not deliver it. |
| 3 | **`match` has literal cases and conditions, and binds nothing.** | Ship this. Structural patterns (`{type: "error", message}`) needed tagged unions in the type system, and those now exist (D47): a `match` on the tag already narrows and is checked for coverage (2.10). What a structural pattern would still add is binding in a pattern. That is its own plan, and it can be added without breaking anything here (`09` section 3). |
| 4 | **Mixing `??` with comparison or logic needs parentheses**, so `role in input.roles ?? []` is an error until parenthesized. | Keep. It is the only choice with no silent misreading. |
| 5 | **A default replaces null as well as "absent"**, unlike TypeScript (D12). C6 leans the other way. | Keep. JSON payloads carry explicit nulls, and authors mean "use the default" for both. TypeScript's rule exists because it has two absent values. |
| 6 | **`in` on a list is membership**, unlike TypeScript (D12, 3.1). C6 leans the other way. | Keep. No author who writes `role in roles` wants the index test. |
| 8 | **Optional type annotations** on lambda parameters and `const` names: `(x: int) => ...`, `const n: int = ...`. TypeScript has them (C6), and they would help (C4): a `const`-bound lambda with annotated parameters could be checked once, where it is written, and not at each call (2.15). Its errors would point at the helper, and it would need no check limit. D29 is now decided, so the spelling is known: an annotation is the type text of `03` 2.2, as in `(xs: string[], sep: string \| null) => ...`. That removes one of the four reasons against. Three stand for 3.0. Where the checker already knows the type, an annotation can only agree or be an error. Types never change results (`03` 2.12), so an annotation on data typed `any` asserts what nothing tests at run time, unless it becomes a test, which TypeScript's are not. And a return type, `(x): int => e`, collides with `c ? (x) : y`. | Not in 3.0. Reserve the syntax now: `:` after a parameter or a `const` name is a targeted error (3.2), so no text can come to mean something else. Add it in a minor version, for `const` names and the parameters of `const`-bound lambdas, as a static claim with no effect at run time. It is an addition, not a break. D29 no longer stands in the way, and D34 makes it smaller: a parameter never holds a function, so no annotation needs a function type (I). **The timing is now settled (D40):** a minor version, together with `type` lines (`07` 2.5). The two belong together, because an annotation is most useful with a name for the type. What stays open is the form. An annotation is a static claim and `as` is the run-time test, so an annotation on data typed `any` should be an error that says to write `as`, and not a hidden test (`07` section 8). |
| 9 | **D31: the nesting maximum and how it combines** (2.16). 100 levels, counted on the text with chains free; and the depth limit counts a call as 1 plus its nesting depth, which changes one sentence in `04` A.6 and `05` A.5. The overview has D31 as a recommendation that the owner was not asked about, with the two numbers to be settled after the Rust reference evaluator has run the limit cases (overview 4.4). Round six adds one ruling to it: type text counts toward the 100 levels. | Accept both. If the depth limit must stay a plain count of calls, the bound is the product, and one of the two numbers has to fall to about a tenth. |
| 10 | **D37: an `if` must have an `else`** (2.10). The overview ruled this and did not ask the owner. An author who writes `if (c) { a }` gets a targeted error. | Keep for 3.0. Nothing has an effect, so the short form could only mean "this value or null", and that null would come from a missing word. Allowing it later is an addition. Taking it away later would be a break. If reversed: the last part of `If` in 2.2 becomes optional, the type of such an `if` gains `null`, one row leaves 3.2, and nothing else here changes. |
| 11 | **D38: does `match` by conditions stay?** (2.10) It now says what an `else if` chain says, and a chain of `? :` is a third way to say it. The overview would drop it and has left it in the documents for now, so it is still in 2.2 and 2.10. | Drop it. "One way to write each thing" is this document's own rule, and `if` is the spelling a TypeScript author knows (C6). `match` by cases stays either way: it is the one form that checks coverage. If dropped: the second alternative of `Match` and the production `When` leave 2.2, a `match` with no subject becomes a targeted error that names `if`, node 25 loses `test`, and examples 27 and 45 and the script's `verdict` are rewritten as `if` chains. The checker and the evaluators lose a rule and gain none. |
| 12 | **The names of the codes** (2.12, 3.2). The round asked this document to name every `parse` code and the run codes it gave only in prose. It names 58 `parse` codes, one for each mistake and shared only between spellings of one mistake, and six run codes: `null-operand`, `wrong-kind`, `missing-field`, `index-range`, `divide-by-zero` and `no-match`. A code cannot be renamed within a major version (`05` A.6), and a run code is a value a program compares, as `err.code`. | Accept the scheme, and read the names once more before `messages.json` is written, because that is the last point at which a name is free to change. The alternative for `parse` is a few shared codes with many templates each: fewer names to keep for ever, and a case or a host would then need the message to tell two mistakes apart. The span of each targeted error is fixed with the suite's cases (3.2). |

Six earlier questions are settled and are no longer in the table.

| Was | Matter | Settled as |
| --- | --- | --- |
| 8 | One spelling for the absent value (D28) | `null`. The change is in 2.2, 2.3, 2.4, 2.13 and 3.2 |
| 10 | `let` or `const` (D36) | `const`, against this document's earlier advice. 2.1 has the reason |
| 11 | The falsy set (D30) | Confirmed as 2.7 has it |
| 13 | A lambda passed to a `const`-bound lambda (D34, E9) | Closed as recommended: 2.9 |
| none | `if` and `else` (D35) | In, against this document's earlier advice. 2.10 has the reasons |
| 7 | `try(e, fallback)` can hide a bug behind a default (D48) | The call form is gone. `try { e } catch (err) { f }` gives the handler the error, and `catch` decides by code: 2.12 |

Questions 9 and 12 of the earlier list are now 8 and 9. Questions 1 to 6 keep
their numbers. Number 7 is not used again, so that 8 to 11 keep theirs, and
question 12 is new in the seventh revision.

The seventh revision also closed four points that the plans left open, by the
rulings of round 8. A literal with no `${ }` is a `String` in the backtick form
too (2.3). `text-block-indent` is reported once for each bad line (2.2). A star
on a map is an error, and a null that a `?.` gives inside a projection is kept
(2.8). `missing-variable` passes through `catch` (2.12). D37 and D38, questions
10 and 11, are still open.

No ruling in the overview seems wrong to me. Four have consequences worth the
owner's eye. N2 (question 1). N5, under which `xs[10 / 2]` is a static error
although its value is 5, because `/` types as `float`. B6, under which `a && b`
in a template renders a falsy `a` as `[]` or `0` (2.7). And R5, which describes
an evaluator's stack as call depth times nesting: 2.16 changes what the depth
limit counts so that the bound is a sum. A fifth, E6's promise of termination by
construction, had a hole, and D34 closed it (2.9, 2.14).

### 8.2 Notes for the TypeScript implementation

- Number to text is `String(x)`; text to number is `Number(s)` after checking
  the JSON grammar. The whole-number rule is `Number.isSafeInteger`. Add `0` to
  results to remove negative zero. `**` must not use `Math.pow`.
- Strings: compare and index by code point, not with `<` and `s[i]`, which use
  UTF-16 units. Reject or replace unpaired surrogates on input.
- Maps: sort keys by code point explicitly. Object key order puts `"9"` before
  `"10"`, and string comparison is by UTF-16 unit (both **ran**).
- The JSON form of a string is what `JSON.stringify` gives for a well-formed
  string (**ran**).
- Truthiness is not JavaScript's: `[]` and `{}` are truthy there (**ran**). So
  `!`, `&&`, `||` and `? :` cannot be the host operators applied to the values;
  each goes through one `truthy(v)` function. For an object read in place, "is
  it empty" is a `for...in` loop that returns at the first own property. If a
  property that holds `undefined` is absent (`05` Part C), the loop passes over
  it, and an object of such properties alone is the one value on which the test
  is not constant work. `len(m)` has the same case.
- The stack: a closure evaluator and a tree walker each overflowed near 9,000
  frames in Node 22 (**ran**), and the number is the engine's to change. Count
  levels in the parser (2.16), loop along chains, else chains and `const` lines
  in every walk, and never rely on catching `RangeError`.
- `as` is erased by TypeScript and tested here, so there is no host operator to
  lean on: an `As` node runs the walk of `07` 2.3. An evaluator carries the type
  text parser and a run-time form of a type, which before only a checker needed.
- `if` is an expression here and a statement in JavaScript. An evaluator gives
  it the code of `? :`, since the two are one node (2.3), and a branch with
  `const` lines is a block. A program is not turned into JavaScript source, so
  the difference costs nothing.
- `const` here allows a pattern with defaults that replace null, where
  JavaScript's replace only `undefined` (3.1). Destructuring is therefore the
  evaluator's own code, not the host's.
- `try` is an expression here and a statement in JavaScript. A `Try` node is one
  host `try` that tests the code of the thrown object and throws again what
  `catch` lets pass (2.12; `04` Part C). JavaScript's own `catch` takes
  everything, a `RangeError` from the stack among it, so the test by code is not
  optional. What a `catch` at each call of a lambda costs, for the trace, is not
  measured (`10` section 8).
- A text block is not a template literal with its indentation trimmed: the
  lexer removes the indentation before escapes are read (2.2). A raw string is
  not `String.raw`: there is no tag, and `r"\"` is one backslash.
- A projection is a loop for each star, not `flatMap` of the host: a null from
  a `?.` is kept, and a list that no star iterated is not flattened (2.8).

### 8.3 Notes for the Go implementation and v2's parser

All sizes are (I). v2 today, without tests: lexer 823 lines, parser 3,358, AST
1,930, `pkg/syntax` 328, `internal/tmpl` 154.

- **Numbers.** A whole number may be held as `int64` and other numbers as
  `float64`, provided a whole `float64` behaves as the integer. Overflow checks
  compare against 2^53 − 1, not the 64-bit range. `encoding/json` gives the same
  number text as Node for the 20 values I tried, except `-0` (**ran**), so
  `strconv` with the ECMAScript layout rule is enough.
- **Text.** Length and index count runes. Byte order of UTF-8 equals code point
  order, so string comparison is unchanged. The JSON form must not come from
  `encoding/json`, which escapes `<`, `>`, `&` and U+2028 (**ran**).
- **Truthiness.** A nil slice and an empty slice are both falsy, as are a nil
  map and an empty one, however the binding presents them. `reflect.Value.Len`
  answers for a host list or map without visiting it.
- **Nesting.** A goroutine's stack grows, so Go would run a program nested far
  deeper than 100 levels, and would evaluate a long chain by recursion without
  harm. It must still report `nesting-exceeded` at level 101, and it should loop
  along chains so that the three evaluators have one shape. v2's counter adds
  one for every link of a chain (`v2:pkg/parser/parser.go:46-52`, `:472-475`);
  2.16 counts differently.
- **Reused from v2.** The Pratt loop, and its depth counter with the counting
  rule changed (`v2:pkg/parser/parser.go:468-553`); the error list with
  positions; `walk.go`; the literal, list, map, call and `in` parsers; `parseIf`
  (`parser/expressions.go:375-398`), which already requires the parentheses and
  reads `else if`, changed to require the `else`, to read a branch as a block
  body and to loop along a chain; `parseMatch` (`parser/expressions.go:700-890`)
  without its pattern-context switch, which is not needed once a lambda is only
  an argument; the grouped-expression code that tells `({a, b}) => ...` from a
  parenthesized map (`:103-325`) and the pattern parsers in
  `parser/statements.go:55-230`, cut down to one level; and, since D48,
  `parseTry` (`parser/statements.go:356-438`), which is already a prefix
  expression with a value. It is changed to read the body and the handler as
  branches, to require the `catch`, and to drop `finally` and the form `catch e
  { }` with no parentheses.
- **Deleted.** `parsePipe`, the slice half of `parseIndex`, the Python-style
  `**` handling (`:45-72`), `parseFunc`, assignment and postfix
  (`parser/statements.go:302-354`), `parseThrow` (`:440-459`), every newline
  helper (`parser/parser.go:654-749`) and the `NEWLINE` token, `pkg/syntax`, and
  `internal/tmpl` once templates are lexed in place.
- **New.** Chain grouping (`OptChain`), blocks, computed keys, the two `match`
  shapes, the `??` mixing check, the `!a in b` check, nested templates, targeted
  errors, the branch rule of `if` with its errors, `as` and `is` with the type
  text parser they call, and the reserved words. The keyword `let` becomes
  `const`, and v2's `nil` and `null` become `null` alone. The two errors of D34
  need scopes, so they are the checker's work and not the front end's. Since the
  seventh revision: the text block and the raw prefix in the lexer, the star
  step and the `Each` node, and a code for every error the front end reports.
- **String forms.** The indentation of a text block is known only at its closing
  delimiter, so the lexer holds the block until then, in time and memory linear
  in its length. Every delimiter and every character of indentation is ASCII. A
  text block's interpolations nest as a template's do. v2's lexer reads a
  template to the next backtick (`v2:internal/lexer/lexer.go:752-766`), so
  nesting is new for both forms.
- **Codes.** v2's parser reports an error as text (`setTokenError`, as at
  `v2:pkg/parser/parser.go:286-288`). Here every
  error has one of the 58 codes of 3.2, and its message comes from a table
  generated from `messages.json` (`10` 2.9). No message is written by hand.
- **Estimate.** Lexer about 900 lines, parser about 2,170, AST about 1,130:
  still about two thirds of today's front end, for 32 node kinds against v2's
  43. `if` adds about 50 lines to the parser and no node. `as` and `is` add
  about 80 lines and one node (`07` section 5). The type text parser, about 300
  lines, moves from the checker to the core, because every evaluator now needs
  it. The seventh revision adds about 150 lines to the lexer for the two string
  forms and 20 to the parser for `form` (`08` section 5); about 70 to the parser
  for `try`, its three words and its seven errors (`10` section 3); about 30 for
  the star (`13` 5.6); about 15 for `import` and the five errors of `12`; and
  about 30 to the AST. The table of codes and messages is generated, and is
  counted in `10` section 3.
- **Order of work.** Lexer, with the string forms; operators and chains, with
  the star; literals, spread and computed keys; lambdas and patterns; blocks and
  `const`; `if`; `match`; `try` and `catch`; templates; the codes and targeted
  errors; then the conformance files of 7.1, which the checker's and the
  evaluator's agreement tests both read.

### 8.4 Notes for the Rust implementation

Nothing was built. Each **ran** below is a throwaway program in Rust 1.99.

- **Numbers.** A number is an `f64`, and the range rule is applied explicitly: a
  value is whole when `x.fract() == 0.0 && x.abs() <= 9007199254740991.0`,
  tested on the operands and on the result of `+`, `-`, `*` and unary `-`. An
  `i64` may be carried beside it, with `checked_add` and a comparison against
  2^53 − 1, not against the 64-bit range. Add `0.0` to a result to remove
  negative zero. `%` on `f64` is the rule of 2.5 (**ran**: `-7 % 3` is `-1`,
  `5.5 % 2` is `1.5`). `**` must be the loop of 2.5 and not `powi`, which gave a
  different last digit for `1.1 ** 10` (**ran**).
- **Number to text.** `{}` never writes an exponent: `1e21` prints as 22 digits
  (**ran**). Take the shortest digits and the exponent from `{:e}`, which gave
  `1e21`, `1e-6` and `1.5e-7` (**ran**), and lay them out by the table in 2.5.
- **Text to number.** `str::parse::<f64>` rounds correctly but accepts `inf`,
  `NaN`, `+1`, `.5` and `1.`, gives an infinity for `1e400`, and gives negative
  zero for `-0` (**ran**). Check the JSON grammar first and the result after.
- **Text.** A `str` is valid UTF-8, and the byte order of UTF-8 is code point
  order. So `==` and `<` on `str` are the rules of 2.6 as they stand (**ran**:
  `"\u{1F600}" < "\u{FFFF}"` is false), and a `BTreeMap<String, T>` iterates in
  the key order of 2.4 (**ran**). Length and `s[i]` by code point are scans. The
  emptiness that truthiness needs is `is_empty()`, with no scan.
- **Malformed text** cannot occur in a `str`. A binding that accepts bytes must
  not use `String::from_utf8_lossy`: it writes one U+FFFD for a truncated
  sequence, where 2.6 asks for one for each byte (**ran**: the bytes `61 E2 82
  7A` are 3 code points by it and by Node's `TextDecoder`, and 4 by Go's
  `[]rune`).
- **The stack.** The parser, the checker, the compiler and the evaluator
  recurse, and a stack overflow ends the process. It is not a panic, and
  `catch_unwind` does not see it (**ran**: exit by abort). 2.16 is what makes
  recursion safe: each of the four carries the level count. A small tree walker
  used 287 bytes of stack a level in a release build (**ran**). A real one will
  use more, and the implementation publishes the figure for 2`D` + 100 levels.
- **Chains and release.** A left-associative chain held as nested boxes is as
  deep as it is long, and dropping a `Box` recurses: a chain of a million boxes
  overflowed the stack when dropped (**ran**). Hold a chain, an else chain of `?
  :` or of `if`, and a block's or a branch's `const` lines as vectors. The tree
  is then no deeper than the nesting, and the derived `Drop`, `Clone` and
  `Debug` are safe. Releasing a deep value is `04`'s matter (overview R6).
- **Type text.** The evaluator needs the type text parser and a run-time form of
  a type, although the checker comes last in the roadmap (`07` section 4). A
  list type written with many `[]` is as deep as its suffixes are many. 2.16
  counts them, so a written type is at most 100 deep and the derived `Drop` is
  safe.
- **Truthiness** is a `match` on the kind. For a host value it needs one
  question of the trait that reads host data (`05` Part D): is this list or map
  empty.
- **`try`.** An error that `catch` catches and one that it lets pass are two
  variants of the evaluator's error already (`04` Part D), so a `Try` node is a
  `match` on the result of its body. The code of an error is an enum generated
  from `messages.json`, and the type of `err.code` is generated from the same
  file.
- **Projections.** Hold the reads of an `Each` as a vector, with a further
  `Each` as the last element for a second star. The recursion is then once for
  each star, which 2.16 bounds at 100, and the derived `Drop` is safe.
- **String forms.** Every delimiter and every character of indentation is ASCII,
  so the lexer works on bytes here. Only the column of a span counts code
  points.
- **Positions.** A column counts code points (T5), so it is the `chars()` count
  of the line before the token, not a byte offset.
