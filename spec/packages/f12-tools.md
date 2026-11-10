# F12. Tools

This chapter is normative. It specifies follow-on package F12. It adds to Core
and to the packages before it:

1. **The formatter**: the canonical text of a program, and of a template (2).
2. **The assist stage**: completion and hover at a position of a text that need
   not parse (3).
3. **The command-line program** `risor`: its commands, arguments, streams and
   exit statuses (4).
4. **The REPL** and **the editor grammar**, described as products (5, 6).
5. Three kinds of case and the `outcome` lines for them (7).

It adds no syntax, no node kind, no value, no reserved word, no library entry,
no run code, no check code, no `parse` code, no message template and no cost
row. It changes the outcome of no earlier case (8).

The chapters of Core are cited by file and section: `01-language.md` 3.1 is
section 3.1 of `spec/core/01-language.md`. F1 is
`spec/packages/f01-host-types.md`, F2 `spec/packages/f02-templates-analysis.md`,
F3 `spec/packages/f03-strings.md`, F4 `spec/packages/f04-conversion.md`, F5
`spec/packages/f05-unions.md`, F6 `spec/packages/f06-groups.md`, F7
`spec/packages/f07-queries.md`, F8 `spec/packages/f08-errors.md`, F9
`spec/packages/f09-testing.md` and F10 `spec/packages/f10-config.md`. Section
numbers without a file are of this chapter.

## 1. Conventions

### 1.1 Terms

| Term | Meaning |
| --- | --- |
| **Formatter** | The function of 2.1: a parsed program and its comments to text |
| **Formatted text** | What the formatter gives |
| **Assist** | The stage of 3: completion and hover |
| **Position** | A line and a column, as `03-host.md` 3.7.7 defines them |
| **Offset** | The number of code points of the text before a position |
| **Width** of a text | Its number of code points. A tab is one. Nothing else is measured: not the cells of a terminal |
| **Line end** | As `01-language.md` 2.1 item 3: U+000A, U+000D U+000A, or U+000D alone |
| **Blank space** | A run of white space (`01-language.md` 2.2 item 1) between two tokens, two comments, or a token and a comment. Comments are not part of it |
| **The CLI** | The command-line program `risor` of 4 |
| **Product** | A part of the package that one implementation builds and tests with its own tests. The suite does not test it |
| **Printed type** | A type printed by `05-types.md` 3.9 |
| **Printed signature** | A signature printed by F1 5.2 |

### 1.2 What is specified for every implementation, and what is a product

| Part | Specified as | Offered by | Tested by |
| --- | --- | --- | --- |
| The formatter, of a program and of a template (2) | A function, exactly: one output for one input | Every implementation, as calls of its binding | The suite: format cases (7.1) |
| Completion and hover (3) | A function, exactly: one result for one text, environment and position | Every implementation, as calls of its binding | The suite: completion and hover cases (7.2, 7.3) |
| The CLI (4) | A contract: commands, arguments, what is written to which stream, byte for byte, and the exit status | Go, in 3.0. The contract binds every implementation that ships a command-line program | The implementation that ships it, by golden files of its own |
| The operations behind the CLI | Each command is made of calls every binding has (4.2) | Every implementation | The suite, through the chapters that specify each call |
| The REPL (5) | What its values mean, and what it must agree with | Go, in 3.0 | Its own tests |
| The editor grammar (6) | What it must recognize | Built once, for every implementation | Its own tests |

1. The formatter's output and the assist results are data, like a value or a
   diagnostic. Two implementations that differ in them give an editor in the
   browser and a server different answers for one text. So each is specified
   to the code point, and each has cases.
2. The CLI is a program, not data. The suite's cases run inside a runner, not
   as a process, and cannot test one. Its output is fixed byte for byte from the
   renderings of F8, the report of F9 and the texts of F10, so its own tests
   compare whole outputs with golden files.
3. TypeScript and Rust ship no command-line program in 3.0. Every operation
   the CLI makes is a call of their bindings, so a host can build the same tool.

### 1.3 Requirements

1. An implementation that offers F12 offers sections 2, 3 and 7. Sections 4 to
   6 bind the implementation that ships the CLI, the REPL or the editor
   grammar.
2. Packages are offered in the order of `03-host.md` 3.11 item 2: F3, F1, F2,
   F4, F5, F6, F7, F8, F9, F10, F11, F13, F12. F12 is the last. So an
   implementation that offers F12 offers every form of F3, F4 and F7, the
   templates of F2, the groups of F6 with every function of F13, the renderings
   of F8, the report of F9 and the commands of F10. The formatter prints every
   one of those forms, assist lists every library entry, and the cases of
   `suite/tools/` use them.

### 1.4 Version

F12 is part of version `3.0`. It adds no version number.

### 1.5 What this package changes in earlier rules

| Earlier rule | With this package |
| --- | --- |
| `03-host.md` 3.2.1 | A fifth stage, assist (3.1), and the formatter (2.1). Neither is a stage of a run, and neither fails with a diagnostic of its own |
| `03-host.md` 3.10.2 item 4 | `format`, `formatTemplate`, `complete` and `hover` count with `source` as one field (7.4) |
| `03-host.md` 3.10.3, 3.10.6 | Three kinds of case, with their fields and results (7) |
| F9 6.3, 8.3 | The lines that say why a program case file is invalid are fixed for the CLI (4.7) |
| F10 9.1 | The rest of the commands, which F10 left to this package (4) |

No other rule changes. A program's type, sites, diagnostics, value and usage
are those of Core and of the packages before this one.

## 2. The formatter

### 2.1 The function

| Takes | Gives |
| --- | --- |
| A source text | The formatted text, or the `parse` diagnostics of the text |

1. The text is parsed by `01-language.md`, with the forms of every package the
   implementation offers. When it does not parse, the formatter gives no text
   and the `parse` diagnostics; only the first is specified (`03-host.md`
   3.7.2).
2. The formatter needs no environment. It does not check the program, and a
   program that breaks a static rule or a type rule is formatted.
3. The formatted text is a function of the syntax tree, of the comments and
   their places among the tokens (2.12), and of where the source has blank
   lines (2.9). Nothing else in the source's layout changes it: not its
   indentation, not where its lines end, not its parentheses.
4. The formatter has no options. Its line width is 80 and its indentation is two
   spaces.
5. The formatted text is UTF-8 text with U+000A for every line end. It has no
   U+FEFF, no tab outside a literal or a comment, no space at the end of a
   line outside a literal or a block comment, and it ends with exactly one
   U+000A.

### 2.2 The laws

Every implementation's formatter keeps these three laws. A format case tests
each of them (7.1).

| Law | Statement |
| --- | --- |
| Idempotent | The formatted text of a formatted text is that text |
| The same tree | The formatted text parses, and its tree equals the tree of the source |
| The same counts | The two texts, checked and run in one environment with the same variables, give the same type, sites apart from their spans, value or failure code and message, steps and units |

1. **Equal trees.** Two trees are equal when they have the same nodes, of the
   same kinds, with equal fields in the same order, spans aside. Two `String`
   nodes are equal when their `form` and their value are equal. Two `Template`
   nodes are equal when their `form`s are equal and their parts are equal one
   by one: a text part by its value, an interpolation by its tree. Two `Number`
   nodes are equal when their values are equal. Two program types (F4 2.2) are
   equal when they have the same structure by the productions of F4 2.1, so a
   final comma in a record type is not part of it.
2. The third law follows from the second, since a program's type, value and
   counts depend on its tree and not on its spans (`02-evaluation.md`). It is
   stated because it is what an author relies on.

### 2.3 What the formatter keeps and what it writes

| Matter | Rule |
| --- | --- |
| Literals | Every number, string and template is written as its source text, code point for code point, but for the indentation of a text block (2.7) and line ends |
| Keys | A key is written as the author wrote it: a word, a string in its form, or computed. A shorthand entry stays shorthand, and `{a: a}` in a map literal stays as written: the two are different trees (`01-language.md` 4.1, `shorthand`) |
| Order | Every element stays in its place. No key, entry or argument is sorted |
| Parentheses | Written where 2.6 says, wherever the source had them |
| Separators | A comma between elements, and after the last element when the sequence is broken over lines (2.9). The `;` after the last expression of a program is dropped |
| Words | One space around every binary operator, `=>`, `=` and `\|`, after `:` and `,`, and between words. None inside brackets |
| Comments | Every comment is written once, beside the token it is attached to (2.12) |
| Blank lines | At most one, where the source had one or more between two elements of a sequence (2.9) |

### 2.4 The document

The formatter builds a **document** from the tree, and then lays it out (2.5).
A document is a part or a list of parts.

| Part | Laid out flat | Laid out broken |
| --- | --- | --- |
| `text(s)` | The code points of s. s holds a line end only when it is the text of a literal or a block comment (2.7, 2.12) | The same |
| `space` | One space, unless the output is at the start of a line | The same |
| `line` | As `space` | A break |
| `soft` | Nothing | A break |
| `hard` | A break, always | A break, always |
| `blank` | An empty line, always | An empty line, always |
| `after-comment` | The next text starts a line (2.5 item 6) | The same |
| `indent(d)` | d | d, with the indentation of each break in it two more |
| `group(d)` | d, flat | d, broken. 2.5 decides which |
| `broken-group(d)` | d, broken | d, broken |
| `if-broken(a, b)` | b | a. Its mode is that of the innermost group around it |
| `choice(a1, ..., an)` | a1 | 2.5 item 9 decides |
| `text-block(t)` | The text block token t, re-indented (2.7) | The same |

1. **A break.** When the output is at the start of a line, the spaces at its end
   are replaced by the indentation. Otherwise the spaces that end the output are
   removed, and a U+000A and the indentation are written. The indentation is that
   many spaces. So a break never writes an empty line, and no line ends with a
   space.
2. **An empty line.** The spaces that end the output are removed. Then line
   feeds are written until the output ends with two U+000A, and then the
   indentation. At the start of the output nothing is written.
3. **The start of a line.** The output is at the start of a line when nothing
   but spaces has been written since its last U+000A, or since its start.
4. **A forced break.** A part holds a forced break when it is or holds a
   `hard`, a `blank`, an `after-comment` or a `broken-group`. A `choice` holds
   one when its first alternative does. A `text` that holds a line end is not a
   forced break.

### 2.5 The layout

The layout writes the output from the document, left to right. It keeps a
stack of commands, each a document with an indentation and a mode, `flat` or
`broken`. It starts with the whole document, indentation 0, mode `broken`.
The **column** is the width of the output after its last U+000A.

1. A list puts its parts on the stack, the first on top, with its own
   indentation and mode. `indent(d)` puts d with two more indentation.
2. `text(s)` is written. When s holds a line end, the column is then the width
   of s after its last line end.
3. `space`, `line`, `soft`, `hard`, `blank` and `if-broken` are as 2.4 says, by
   the mode of their command.
4. **A group** in a `flat` command is laid out flat, unless it holds a forced
   break. In a `broken` command it is laid out broken when it holds a forced
   break, flat when it fits (item 7), and broken otherwise. A `broken-group` is
   always laid out broken.
5. A group laid out flat gives its command the mode `flat`; every group inside
   it is then flat, but one that holds a forced break.
6. **`after-comment`.** It marks the output. When the next part written is a
   break or an empty line, the mark is cleared by it. When the next part written
   is a `text`, a `space`, a flat `line` or a `text-block`, a break at that part's
   indentation is written first and the mark is cleared; a `space` or a flat
   `line` is then not written.
7. **Fits.** A document d fits at the column c when this walk ends with success.
   Let r be 80 − c. Walk d in mode `flat`, and then the commands below it on the
   stack in their own modes, from the top. For each part met:
   - `text(s)` with no line end: r falls by the width of s, and the walk fails
     when r is below 0. `text(s)` with a line end: r falls by the width of s
     before its first line end, and the walk ends, with success when r is not
     below 0.
   - `space`, and `line` in mode `flat`: r falls by 1, and the walk fails when r
     is below 0. `soft` in mode `flat`: nothing.
   - `line` or `soft` in mode `broken`, `hard`, `blank`, `after-comment`: the
     walk ends with success.
   - `indent(d)`: d, in the same mode. `if-broken(a, b)`: a in mode `broken`, b
     in mode `flat`.
   - A group: walked in mode `broken` when it holds a forced break or is a
     `broken-group`, else in the mode of the walk.
   - A `choice`: its first alternative in mode `flat`, its last alternative in
     mode `broken`.
   - `text-block(t)`: r falls by the width of its opening delimiter, and the walk
     ends with success when r is not below 0.
   - The end of the stack: success.
8. So a group fits when its flat text, and what follows it up to the next place
   where a line may break, fit in what is left of the line.
9. **A choice** in a `flat` command is its first alternative, flat. In a
   `broken` command: when the choice's rule (2.10, 2.11) says it must be broken,
   its last alternative, broken. Otherwise the first alternative that fits by
   item 7, taken flat; when no alternative but the last fits, the last, broken.
10. Before the output is finished, its last line is ended: the document of a
    program ends with `hard` (2.8), and the text ends with one U+000A.

### 2.6 Parentheses

Parentheses are not part of the tree (`01-language.md` 4.2). The formatter
writes a pair around a subexpression exactly where this section says, and
nowhere else. Parentheses that the source had elsewhere are dropped. In each
row, the level of an expression is its level of `01-language.md` 3.3: 1 for
`? :`, `if` and `try`, 2 for `??`, and so on to 10 for `**`; an `As` node (F4)
has level 6; a prefix operator 9; a chain and a primary 11.

| The subexpression x | Is written in parentheses when |
| --- | --- |
| The left operand of a binary operator of level p, 3 to 8 | x has a level below p, or x is `??` |
| The right operand of a binary operator of level p, 3 to 8 | x has a level of p or below, or x is `??` |
| An operand of `??` | x has a level below 7. A `??` as the right operand |
| The left operand of `**` | x has a level below 11 |
| The right operand of `**` | x has a level below 9 |
| The left operand of `in`, `not in`, `as` and `is` | By the rows above; and when x is a prefix `!` (`01-language.md` 3.4 rule 2, F4 2.5) |
| The operand of a prefix `!` or `-` | x has a level below 9, or x is `**` (`01-language.md` 3.4 rule 1) |
| The receiver of a step of a chain | x has a level below 11; or x is a chain that holds a `?.`, `?.[` or `?.[*]` step, or a star (F7), since `(a?.b).c` and `a?.b.c` are different trees |
| The test of `? :` | x has level 1 |
| The subject of `match` | The text of x would begin with `{`, `if` or `try` (`01-language.md` 3.4 rule 6). The parentheses go around the whole subject |
| Any other place: an argument, an item, an entry's value, a key's expression, an index, a `const` value, a branch, a lambda's body, a default, an arm's result, the branches of `? :`, a spread's operand | Never |

These parentheses are added for the reader. Each changes no tree.

| x | Its parent | Example |
| --- | --- | --- |
| `&&` | An operand of `\|\|` | `(a && b) \|\| c` |
| `*`, `/` or `%` | The left operand of `*`, `/` or `%` with another operator, or of `%` with `%` | `(a % b) * c`, `(a * b) / c`, `(a % b) % c` |
| `==` or `!=` | The left operand of `==` or `!=` | `(a == b) != c` |

1. A chain written in parentheses as the receiver of a further step, which
   holds no optional step and no star, is the same tree as the chain without
   them: `(xs.map(f)).sum()` is written `xs.map(f).sum()`.
2. So the formatted text has the parentheses of its tree, wherever the source
   put its own.

### 2.7 Literals and line ends

1. **Verbatim.** A number, a one-line string, a raw string and a template are
   written as their token's text. Each line end in the text of a template,
   raw or not, is written U+000A. A template's interpolations are part of its
   text: nothing in them is formatted, and their comments are written as they
   stand.
2. **A text block** (F3 4), raw or not, is written with its indentation
   replaced, and nothing else changed:
   1. The opening delimiter, `"""` or `r"""`, then U+000A. The opening line's
      spaces and tabs are dropped.
   2. Let I be the block's indentation (F3 4.4) and J the new one: as many
      spaces as begin the output's current line when the literal is written,
      plus two.
   3. Each content line (F3 4.3) other than the closing line, in order: a blank
      line is written empty; any other is written with its first code points,
      I, replaced by J. Then U+000A.
   4. Lines that begin inside an interpolation are not content lines. They are
      written as they stand, with each line end written U+000A.
   5. Then J and `"""`.
3. So the value of a text block does not change: its content lines lose I and
   gain J, and its closing line is J. The value is made only from what follows
   the indentation (F3 4.6).
4. A template is never turned into a text block, and no literal into another
   form (F3 5.2 item 2).
5. Every line end the formatter writes is U+000A, inside a literal and a block
   comment too. The value of a template or a text block does not change by it
   (`01-language.md` 2.8 item 3, F3 4.6).

### 2.8 Documents of the nodes

Notation: `g(...)` is a group, `i(...)` an indent, `sp` a space, `L` a line, `S`
a soft line, `H` hard, `ib(a, b)` if-broken. A quoted token is a `text` of that
token with its comments (2.12). `‹x›` is the document of x, in parentheses when
2.6 says. `seq(o, xs, c)` is 2.9; `assigned(v)` is item 2.

| Node | Document |
| --- | --- |
| `Program` | For each `const` line and then the result: the document of it, then `H`. An empty line before an element as 2.9 says. Then the leading comments of the end of the text (2.12). The final `;` is not written |
| `Const` | `"const" sp ‹pattern› sp "=" assigned(value) ";"` |
| `Block` | `"{" i(H body) H "}"`, where body is the `const` lines and the result, separated by `H` and by empty lines as 2.9 says |
| A branch of `if` or `try` | `"{" i(L body) L "}"`. A branch whose whole content is a `Block` is written with that block's `const` lines and result as its body: the two are one tree (`01-language.md` 4.2) |
| `Cond`, form `if` | `g("if" sp "(" g(i(S ‹test›) S) ")" sp branch, then for each `else if`: sp "else" sp "if" sp "(" g(i(S ‹test›) S) ")" sp branch, then sp "else" sp branch)`. An `else` branch whose whole content is an `if` is written `else if`: the two are one tree |
| `Cond`, form `?:` | `g(‹test› i(L "?" sp ‹then› L ":" sp ‹else›))` |
| `Try` | `g("try" sp branch sp "catch" sp, then "(" name ")" sp when it has a name, then branch)` |
| `Binary` | The operands x1 to xn and operators o1 to on−1 of a run of one level (item 3): `g(‹x1› sp o1 i(L ‹x2› sp o2 L ‹x3› ... L ‹xn›))`. Unindented (item 4): the same without `i( )` |
| `As` (F4) | `‹x› sp "as" sp type`, or `"is"`; type by 2.13 |
| `Unary` | The operator, then `‹x›`. A space between when the operator is `-` and x is a prefix `-` written without parentheses: `- -a` |
| `Ident`, `Bool`, `Null`, `Number`, `String`, `Template` | The token (2.7) |
| `List` | `seq("[", items, "]")`. A spread item is `"..." ‹e›` |
| `Map` | `seq("{", entries, "}")`. An entry: `key ":" assigned(value)`; a computed key is `"[" ‹k› "]"`; a shorthand entry is its name; a spread is `"..." ‹e›` |
| `Call` | The name, then the arguments (2.10) |
| A chain | 2.11 |
| `Lambda` | params `sp "=>" assigned(body)`. params is the name, or `_`, when the lambda has one parameter whose pattern is a name or `_` and that has no default; else `seq("(", params, ")")`. A parameter is `‹pattern›`, then `sp "=" assigned(default)` when it has one |
| `NamePat` | The name or `_` |
| `MapPat` | `seq("{", fields and rest, "}")`. A field whose key is a word equal to its name is the name alone: `{a: a}` and `{a}` are one tree. Another field is `key ":" sp name`. A default is `sp "=" assigned(default)`. The rest is `"..." name` |
| `ListPat` | `seq("[", elements and rest, "]")`. An element is the name or `_`, then its default as for a field |
| `Match` | `g("match" sp ‹subject› sp "{" i(L arm1 "," L arm2 "," ... L armn ib(",", "")) L "}")`, with the leading comments of `}` inside the indent and empty lines as 2.9 says. An arm: its literals separated by `sp "\|" sp`, or `_`, then `sp "=>" assigned(result)`. A literal is `-` and the number when it has one |

1. A text such as `"const"` is the token of the source, not a new one, so its
   comments come with it (2.12). A comma and a parenthesis the formatter writes
   are new.
2. **Assigned.** `assigned(v)` is `g(i(L ‹v›))`, with v unindented (item 4),
   when v is a `Binary`, an `As` or a `Cond` of form `?:`. For any other v it
   is `sp ‹v›`. So such a value breaks after its `=`, `:` or `=>` when it does
   not fit on the line, and a map, a list, a call, a lambda, a block, a `match`,
   an `if` or a `try` stays on the line and breaks inside itself.
3. **A run of one level.** The left operand of a `Binary` is part of its run
   when it is a `Binary` of the same level, other than `**`, that 2.6 writes with
   no parentheses. So `a + b - c` is one run of three operands, `a && b || c`
   is a run of `||` whose first operand is the group of `(a && b)`, and `a ** b
   ** c` is a `**` whose right operand is a `**`.
4. **Unindented.** A run is written without `i( )` when it is the v of
   `assigned(v)`, or the test of an `if` or an `else if`. Its lines then stand at
   the indentation that the `assigned` or the parentheses already give.

### 2.9 Sequences, commas and empty lines

`seq(o, xs, c)` is a bracketed list of elements: the items of a list, the
entries of a map, the arguments of a call, the parameters of a lambda, and the
fields or elements of a pattern.

1. With no element: `o`, the leading comments of `c` (2.12), then `c`.
2. Otherwise: `g(o i(S x1 "," L x2 "," ... L xn ib(",", "")) S c)`, with the
   leading comments of `c` written inside the indent, after xn's comma.
3. So when the group is flat, the elements stand on one line with no comma after
   the last: `[1, 2, 3]`. When it is broken, each element stands on its own line,
   one level in, each with a comma after it, and `c` stands on its own line.
4. **Empty lines.** An element of a sequence, other than its first, is preceded
   by an empty line when the source has two or more line ends in the blank space
   directly before it, and it has no leading comment. In `seq` it is
   `ib(blank, "")` after the `L`, so a flat sequence drops it. The same holds for
   the `const` lines and result of a program, a block and a branch, and for the
   arms of a `match`. A leading comment carries its own empty lines (2.12).
5. No empty line is written after an opening bracket, before a closing one, or at
   the start or the end of the text.

### 2.10 Arguments and the hugged last argument

1. The arguments of a call are `seq("(", args, ")")`, unless the last argument
   is **huggable**: a `Map`, a `List` or a `Block`; or a `Lambda` whose body is a
   `Map`, a `List`, a `Block`, a `Match`, a `Cond` of form `if`, or a `Try`.
2. With a huggable last argument they are `choice(a1, a2, a3)`:
   - a1, **on one line**: `"(" x1 "," sp x2 "," sp ... xn ")"`, with no `line`
     or `soft` of its own, and the leading comments of `)` before it;
   - a2, **hugged**: a1, but the outer group of xn is a `broken-group`. The
     outer group of a `Map`, a `List` or a `Match` is its own group; of a `Cond`
     or a `Try`, its group; of a lambda, the outer group of its body. A `Block`
     always breaks;
   - a3, **broken**: `seq("(", args, ")")`.
3. **The choice's rule.** When an argument other than the last holds a forced
   break, a3. Otherwise by 2.5 item 9: a1 when it fits; else a2 when its first
   line fits; else a3.
4. So `items.map(i => {id: i.id})` stays on one line when it fits; when it does
   not, the map breaks inside the call and the call's `(` and `)` stay beside it:

```
items.map(item => {
  id: item.id,
  name: item.name,
})
```

### 2.11 Chains

A chain (`01-language.md` 7.4) is its primary and its steps.

1. Its **receiver part** is the primary and the steps before its first method
   call. Each **link** is a method call step and the steps after it up to the
   next method call. A step is `"." name` or `"?." name`; `"[" ‹e› "]"` or
   `"?.[" ‹e› "]"`; a star `"[" "*" "]"` or `"?.[" "*" "]"` (F7); a method call
   is `"." name` or `"?." name`, then its arguments (2.10).
2. A chain with no link, or with one link whose first token has no leading
   comment, is its receiver part and its link, in a row.
3. A chain with two or more links is `choice(c1, c2)`: c1 is the receiver part
   and the links in a row; c2 is `broken-group(receiver i(S link1 S link2 ...
   S linkn))`. **The choice's rule:** c2 when a link other than the last holds a
   forced break. Otherwise by 2.5 item 9.
4. A chain whose link has a leading comment on its first token is c2.
5. So a broken chain puts each method call on its own line, one level in:

```
const names = fetch.tests
  .filter(t => t.status == "fail")
  .map(t => t.name);
```

### 2.12 Comments

A comment is attached to one token of the formatted text, before it or after
it, and is written there. No comment is lost or written twice.

1. **Kept tokens.** The formatted text has a token for each token of the source
   but these: a comma; the `;` that ends a program; a parenthesis of a
   parenthesized expression; the parentheses around the one parameter of a
   lambda that 2.8 writes without them; the braces of an `else` branch written
   `else if`, and of a `Block` that is the whole content of a branch (2.8); the
   key and the colon of a pattern field written as its name. The other tokens
   are **kept**.
2. **Trailing and leading.** A comment is **trailing** when a token stands
   before it and the comment starts on the line where that token ends. Every
   other comment is **leading**.
3. **Attached to.** A trailing comment is attached after the last kept token that
   ends before it. When there is none, it is a leading comment. A leading comment
   is attached before the first kept token that starts after it, or, when there
   is none, to the end of the text.
4. **A trailing comment** is written after its token: a space, then the comment.
   No space is written before a block comment directly after an opening bracket,
   and one space is written after such a comment when the bracket's sequence has
   an element: `f(/* no args */)`, `[/* first */ 1, 2]`. A line comment is
   followed by `after-comment`. Several trailing comments of one token are
   written in their order.
5. **After an element.** The trailing comments of the last token of an element
   are written after the whole element's document, and after its comma when the
   formatter writes one. The elements are those of a sequence (2.9), the `const`
   lines and the result of a program, a block and a branch, and the arms of a
   `match`. So `a, // c` and `a // c` then `,` both give `a, // c`; and the
   comment of `1 + 2; // done` stands outside the group of `1 + 2`, which a line
   comment would otherwise break. In a program type, the trailing comments of the
   last token of a field are written after the comma that follows it.
6. **A leading comment** is written before its token: `hard`, the comment, then
   `hard` when it is a line comment or a line end follows it in the source, and
   `space` when it is a block comment that its token follows on the same line.
   So a leading comment always starts a line.
7. **Empty lines around a leading comment.** The first leading comment of a token
   is preceded by `blank` when the blank space directly before it holds two or
   more line ends and the token is the first token of an element of a sequence
   other than its first element (2.9), a closing bracket of a sequence with
   elements, or the end of the text. A leading comment is followed by `blank`
   when the blank space directly after it holds two or more line ends, unless its
   token is a closing bracket or the end of the text and it is the last leading
   comment of that token.
8. **Leading comments of a closing bracket** are written inside the indentation
   of their sequence, block, branch or `match`, after its last element.
9. **The text of a comment.** A line comment is written without the spaces and
   tabs at its end. A block comment is written as it stands, each line end
   written U+000A. A block comment that holds a line end is a `text` with a line
   end (2.5 item 7).
10. A line comment or a leading comment holds a forced break, so every group
    around it is broken.
11. Comments inside a template's interpolation are part of the template's text
    (2.7).

The rule of item 3 never moves a comment across a kept token. A comment moves
across a comma (item 5), a parenthesis, or a brace that the formatter drops.
After a trailing line comment the next token starts a line: `a // why` then `&&
b` gives `a // why`, then `&&`, then `b` one level in, since the operator stands
at the end of a line (2.8).

### 2.13 Program types and stars

1. **A program type** (F4 2.2) is written on one line, from its tokens: `sp "|"
   sp` between alternatives; `", " ` between the fields of a record type, and no
   comma after its last field; `": "` after a field's key or its `?`; `[]`,
   `map<` and `>`, `(` and `)`, `{` and `}` with no space inside; `...` directly
   before its type. A comment in it is attached and written by 2.12.
2. **A star** step is `"[" "*" "]"`, with no space, or `"?.[" "*" "]"` (F7 2.1).

### 2.14 A template

The formatter of a template (F2 2) takes a template text and gives a text.

1. The template is read by F2 2.1 and 2.2. When it has a `parse` diagnostic,
   the formatter gives no text and the diagnostics.
2. Each text run is written as it stands, code point for code point, its line
   ends and U+000D included. A template is a host's document, and its text is
   the host's.
3. Each part: its expression's document (2.8) is laid out alone, at indentation
   0 and with no limit of width. When that text holds no line end and the part
   holds no line comment, the part is written `${{ `, the text, ` }}`. Otherwise
   the part is written as it stands.
4. So `${{a+b}}` is written `${{ a + b }}`, and a part that holds a line comment
   or a block keeps its own text.

### 2.15 Examples

`⏎` is U+000A.

| Source | Formatted |
| --- | --- |
| `const x=1;x+2;` | `const x = 1;⏎x + 2⏎` |
| `((a + b)) * c` | `(a + b) * c⏎` |
| `a && b \|\| c` | `(a && b) \|\| c⏎` |
| `f( a , b , )` | `f(a, b)⏎` |
| `1 + 2; // done` | `1 + 2 // done⏎` |
| `(x) => x` as an argument | `x => x` |
| `{a: a}` | `{a: a}⏎`: a different tree from `{a}` |
| `const {a: a} = m; a` | `const {a} = m;⏎a⏎` |
| `if (c) { 1 } else { if (d) { 2 } else { 3 } }` | `if (c) { 1 } else if (d) { 2 } else { 3 }⏎` |
| `- -x`, `-(-x)` | `- -x⏎` |
| `match (({a: 1}).a) { _ => 0 }` | `match ({a: 1}.a) { _ => 0 }⏎`: `match {a: 1}.a` does not parse |
| `0x1F + 1_000 + 1E3` | `0x1F + 1_000 + 1E3⏎` |

## 3. The assist stage

### 3.1 The stage

| Stage | Takes | Gives | Fails |
| --- | --- | --- | --- |
| Assist | A source text that need not parse, an environment, an expected result (`03-host.md` 3.2.3) or none, and a position | Completion: a span and a list of candidates. Hover: a hover record, or nothing | Never |

1. Assist reports no diagnostic. A text that cannot be helped gives an empty
   list, or no hover.
2. The environment and the expected result are those a check would take. Assist
   uses the check of `05-types.md` on a repaired program (3.3) for the types of
   nodes and the scopes of names, and ignores its diagnostics.
3. Assist charges no step and no unit. It is bounded as a check is: its work is
   one lex, one parse and one check of a text no longer than the source, and the
   candidates it lists.
4. A position that is not in the text, or that is in no line of it, gives an
   empty list and no hover.

### 3.2 The cut

Completion and hover read the text only up to a place, the **cut**. What follows
the cut is not read.

1. A U+FEFF at the start of the text is removed first, as for parse.
2. **Completion's cut** is the position. **The prefix** is the code points
   between the start of the word token the position is in or at the end of, and
   the position, when the code point before the position is part of a word
   (`01-language.md` 2.4). Otherwise the prefix is empty. **The prefix span**
   runs from the start of the prefix to the position.
3. **Hover's cut** is the end of the token that holds the code point at the
   position, or the end of the `(` that directly follows that token, when one
   does: so `xs.first(` is still a method call.
4. **The text before the cut** is lexed by `01-language.md` 2 and F3. When the
   cut stands inside an interpolation of a template or a text block, the
   interpolation and every literal around it end at the cut: their tokens are
   complete, and the parser reads the interpolation's tokens up to the cut.
5. These give an empty list, or no hover:
   - a lexical diagnostic in the text before the cut, other than a literal or an
     interpolation left open by the cut;
   - a cut inside a comment, inside the text of a template or of a text block,
     inside a raw string, or inside or at the end of a number;
   - for completion, a cut inside a one-line string that holds a `\` before the
     cut, unless 3.5 gives a string context.

### 3.3 The repair

The text before the cut, with the hole of 3.4 for completion, is made a program
by the **repair**.

1. The tokens are parsed by `01-language.md` 3 and the packages, with this one
   change: where the parser, at the end of the tokens, would report a `parse`
   diagnostic, it inserts a token in place of the end and goes on. It inserts the
   first token of this list that it accepts there:

   `null`, `)`, `]`, `}`, `;`, `:`, `=>`, `=`, `{`, `else`, `catch`, `>`

2. It goes on inserting until the program is complete. When no token of the list
   is accepted, the repair **fails**.
3. A `parse` diagnostic at a token before the end, the hole included, makes the
   repair fail.
4. The repaired program is checked by `05-types.md` against the environment and
   the expected result. Its diagnostics are not reported.
5. So `items.map(i => i.` with the hole is `items.map(i => i.◊)`, and `if (` with
   the hole is `if (◊) { null } else { null }`. The tokens the repair inserts
   never stand before the hole, so the scope of the hole and the type of what
   stands before it are those of the text the author wrote.

### 3.4 The hole

1. **The hole** is a token that no text can spell. The parser reads it wherever
   it needs a `Name`, a `Word` or a type name (F4 2.1). Its span is the prefix
   span.
2. Read as a value, the hole is bound, and has the type `never`. It is not
   `undefined-name`, and nothing is reported for it.
3. Completion puts the hole at the cut, in place of the prefix. When the cut is
   inside a one-line string, the string's token is replaced by a string literal
   that is the hole: its value is the code points from after its opening quote
   to the cut.
4. Hover puts no hole.

### 3.5 Completion: the context

The **context** is decided by where the hole stands in the repaired program.

| The hole stands as | Context |
| --- | --- |
| An `Ident` | names (3.6) |
| The name of a shorthand entry of a `Map` that has an expected record (3.9) | keys (3.9); otherwise names |
| The word of a `Member` or a `MethodCall`, after `.` or `?.` | members of the receiver (3.7), or entries of a namespace (3.8) |
| A type name in a program type (F4) | types (3.10) |
| A string literal, as the literal of a `match` arm, as an operand of `==` or `!=`, or where a value has an expected type (3.9) | strings (3.11) |
| A name that a pattern, a parameter or `catch` binds; a key of a pattern or of a record type; anywhere else | none: the empty list |

1. **The fallback.** When the repair fails: if the last token before the prefix
   is `.` or `?.`, the list is empty; otherwise the list is the candidates of the
   names context for the environment alone (3.6 items 4 to 6), as if the program
   bound no name.
2. Every list is filtered by the prefix: a candidate is listed only when its
   label begins with the prefix, code point for code point. Case counts.
3. **The result** is the prefix span and the list. The span is the same for every
   candidate: an editor replaces the prefix with a candidate's insert text.

### 3.6 Names

The candidates of the names context are, in this order:

1. The names bound in the program that are in scope at the hole, by
   `01-language.md` 5 and `05-types.md` 5.1: from the innermost scope out, and
   in one scope in the order they are bound. A name is in scope at the hole when
   it would be in scope for an `Ident` there: a `const` name from the end of its
   line, a parameter in its lambda's body and in the defaults to its right, a
   pattern's name in the defaults to its right, the name of `catch` in its
   handler. A `const` name is not in scope in its own value.
2. Each name once, at its innermost binding. `_` binds nothing.
3. A const-bound lambda has the kind `lambda`; every other name bound in the
   program has the kind `name`.
4. Then the environment's variables, in code point order, kind `variable`.
5. Then the functions of the environment: the library's functions that are not
   removed, and the host's functions with no namespace, in code point order,
   kind `function`. A function whose name an earlier candidate has is not listed:
   a variable hides a library function of its name (`01-language.md` 8.2).
6. Then the namespaces: the groups that are on (F6) and the prefixes of the
   host's functions, in code point order, kind `namespace`.

### 3.7 Members of a receiver

The receiver is the expression before the `.` or `?.`. Its type R is the type
the check gives the receiver of the step (`05-types.md` 9.1), with `null` taken
out. When R is `any` or `never`, the list is empty.

1. **Fields.** The keys each record alternative of R lists that are words
   (`01-language.md` 2.4 rule 1), in code point order, kind `field`. A key is
   listed once. A key that is not a word is not listed: `.` cannot reach it.
2. **Methods.** Then the methods of each kind among R's alternatives: for a
   string, the library's string methods; for a list, its list methods; for a
   record or a `map<T>`, its map methods; for an opaque type, the methods the
   environment declares for it. Removed entries are not listed. In code point
   order of their names, kind `method`. A name is listed once; its detail is that
   of the first kind in this order: string, list, map, then opaque types in code
   point order.
3. The fields come before the methods.
4. When the receiver is an `Ident` that names a namespace of the environment and
   no binding hides it, the context is the namespace's entries (3.8), not
   members.

### 3.8 Entries of a namespace

1. For a group that is on: its functions and its constants (F6), by their names
   after the dot, in code point order. A function has the kind `function`, a
   constant the kind `constant`.
2. For a prefix of host functions: each host function of that namespace, by its
   name after the dot, in code point order, kind `function`.

### 3.9 Expected types and keys

The **expected type** of a node of the repaired program is given by these rows,
from the outside in. A node no row reaches has none.

| Node | Its expected type |
| --- | --- |
| The program's result | The expected result, when it is a type (`03-host.md` 3.2.3) |
| An argument of a call of a host function | The parameter's type, when it holds no type variable |
| The result of a `Cond`, each arm's result of a `Match`, the result of a `Block` or a branch, whose node has an expected type E | E |
| An item of a `List` whose expected type E has list alternatives | The Join of their element types |
| The value of an entry with key k of a `Map` whose expected type E has record alternatives | The Join, over those records, of the field type for k: its field, or else its rest when the rest is not `never` |

1. **Keys.** In the keys context, the candidates are the keys that the record
   alternatives of the map's expected type list, less the keys the map already
   writes as a word, a string or a shorthand, in code point order, kind `key`.
   Each key is listed once.
2. A key's detail is the printed Join of its field types over the records that
   list it. A key's insert text is the key when it is a word, else its JSON form
   (`01-language.md` 12).
3. So with the expected result `{name: string, port: int, debug?: bool}`, the
   text `{name: "x", ‸` (the position at `‸`) lists `debug` and `port`.

### 3.10 Types

In a program type, the candidates are the eight type words `any`, `bool`,
`float`, `int`, `map`, `never`, `null` and `string`, the environment's named
types (F4 3), and its opaque types, together in code point order, kind `type`.

### 3.11 Strings

When the hole is a string literal, the candidates are string values, kind
`literal`. The **string type** is:

| The literal | Its string type |
| --- | --- |
| A literal of a `match` arm | The type of the subject |
| An operand of `==` or `!=` | The type of the other operand |
| A node with an expected type (3.9) | That type |

1. The candidates are the strings of the literal sets among the alternatives of
   the string type, in code point order. For a `match`, a string another arm of
   that `match` already lists is left out.
2. The prefix is the hole's value, and the prefix span runs from after the
   opening quote to the position.
3. A candidate's insert text is its value written as the content of a one-line
   string in the literal's quote: each `\` as `\\`, that quote as `\` and the
   quote, U+0008, U+000C, U+000A, U+000D and U+0009 as `\b`, `\f`, `\n`, `\r`,
   `\t`, every other code point below U+0020 as `\u` and four lower-case hex
   digits, and every other code point as itself.
4. So with a subject of type `"a" | "b" | "c"`, `match s { "a" => 1, "‸` lists
   `b` and `c`; and with the expected result `{level: "debug" | "info"}`,
   `{level: "‸` lists `debug` and `info`. A tag of a tagged union (F5) is a
   literal set, so `{kind: "‸` lists each variant's tag.

### 3.12 A candidate

| Field | Holds |
| --- | --- |
| `label` | The name, key, string value or type word |
| `kind` | `name`, `lambda`, `variable`, `function`, `namespace`, `field`, `method`, `constant`, `key`, `literal` or `type` |
| `detail` | A text or null, by the table below |
| `insert` | The text an editor puts in place of the prefix: the label, but for a key (3.9 item 2) and a string (3.11 item 3) |

| Kind | Detail |
| --- | --- |
| `name` | The printed type the name is bound to: for a `const` name, the type of its value or of its pattern's field; for a parameter, its type in the check of its lambda at that place |
| `variable` | Its declared type, printed |
| `field` | The printed Join of its field types over the record alternatives that list it |
| `key` | 3.9 item 2 |
| `function`, `method` | Its printed signature. A method's is printed without its receiver |
| `constant` | Its type, printed |
| `lambda`, `namespace`, `literal`, `type` | null |

### 3.13 Hover

1. **The token.** Hover is about the token that holds the code point at the
   position. When that is not a word, there is no hover.
2. **The tree.** When the whole text parses, hover reads its tree, checked as 3.3
   item 4 says. Otherwise it reads the repaired program of the text before
   hover's cut (3.2 item 3). When the repair fails, there is no hover.
3. **The record**, by what the word is in the tree:

| The word is | `kind` | `type` | `signature` | `doc` | `declaration` |
| --- | --- | --- | --- | --- | --- |
| An `Ident` of a value bound in the program | `name` | The printed type of the `Ident` (narrowed by the facts in force, `05-types.md` 5.3) | null | null | The span of the name where it is bound |
| An `Ident` of an environment variable | `variable` | The same | null | null | null |
| A name a `const`, a pattern, a parameter or `catch` binds | `name` | The printed type it is bound to | null | null | Its own span |
| The name of a `Call` of a library function, or of a function of a group | `function` | The printed type of the `Call` | Its printed signature | The `doc` of its record | null |
| The name of a `Call` of a host function | `function` | The same | Its printed signature | The doc its declaration carries in the binding, or null | null |
| The name of a `Call` of a const-bound lambda | `lambda` | The printed type of the `Call` | null | null | The span of its name in its `const` line |
| The word of a `MethodCall` | `method` | The printed type of the `MethodCall` | The printed signature of the method the check resolved, without its receiver, or null | The `doc` of its record, or null | null |
| The word of a `Member` | `field` | The printed type of the `Member` | null | null | null |
| The `Ident` of a namespace | `namespace` | null | null | null | null |
| A constant of a group (F6) | `constant` | Its printed type | null | The `doc` of its record | null |

4. `span` is the span of the word's token.
5. Any other word, a keyword or a type word among them, has no hover.

## 4. The command-line program

### 4.1 Scope

These rules bind every implementation that ships a command-line program. In
3.0 the Go implementation ships one, `risor`. F9 8 fixes `risor test`, and F10
9 fixes what a config needs of `eval` and `check`. This section fixes the rest,
and changes nothing those sections fix.

```
risor eval FILE [--type TEXT | --schema FILE] [--format text|json] [--pretty]
               [ENV ...] [--max-failures N] [--color WHEN]
risor check FILE [--type TEXT | --schema FILE] [--format text|json]
               [--print-type] [ENV ...] [--color WHEN]
risor fmt [PATH ...] [--write | --check] [--template] [--color WHEN]
risor test [PATH ...] [--run TEXT ...] [--update] [--format text|json]
               [--color WHEN]
risor test --suite DIR [--format text|json] [--color WHEN]
risor [ENV ...]
risor help | risor version
```

ENV stands for the flags of 4.3.

### 4.2 Common rules

1. **Input.** A FILE is read as UTF-8 text. `-` as a FILE or a PATH is standard
   input. Text that is not UTF-8 is an error of the command line (4.9).
2. **Streams.** Standard output holds a command's product: the value of `eval`,
   the diagnostics of `check`, the formatted text of `fmt`, the report of
   `test`. Diagnostics of `eval` and `fmt` go to standard error. Everything is
   UTF-8, and every line ends with U+000A.
3. **Diagnostics.** With `--format text`, the default, each diagnostic is
   written in the detail form of F8 5.3, with the source text of the file. With
   `--format json`, each is written in the JSON form of F8 5.5. A `parse`
   failure writes its first diagnostic only, the one the specification fixes; a
   `check` failure every diagnostic, in order; a `run` failure its one
   diagnostic; a failure of step 4 of a load (F10 5.2) each of its diagnostics,
   in order.
4. **Colour.** `--color auto`, the default, colours the text written to a
   stream only when that stream is a terminal. `--color always` colours and
   `--color never` does not. Colour is ANSI SGR sequences, `ESC [` … `m`, put
   into the text: removing every such sequence gives the text this chapter fixes,
   byte for byte. Which parts are coloured is not specified. Nothing is coloured
   in a pipe or a file under `auto`. No process variable is read for colour
   (F10 9.4 item 1).
5. **Byte for byte.** For the same arguments, files and standard input, with
   colour off, the bytes written to standard output and standard error and the
   exit status are those this chapter, F8, F9 and F10 give, except what item 6
   names. So an implementation's tests of its CLI compare whole outputs with
   golden files.
6. **Not specified:** the message of a `declare` diagnostic (F8 1.2 item 2), so
   its renderings differ in the message only; the text of an error of the
   command line (4.9); the text of `--pretty` (F10 4.3); the text of `help` and
   `version`; the lines of F9 6.3 that say why a file is invalid, other than
   those of 4.7 item 3; and the REPL (5).
7. **The calls behind each command.** Each command is made of calls that every
   binding offers: parse, check with an expected result, compile and run
   (`03-host.md` 3.2), the JSON text (`03-host.md` 3.2.4), the conformance
   function of every failure (F8 6), the renderings (F8 5), the description of
   an environment (F1 5), the case runner and update mode (F9), and the
   formatter (2). A command adds only reading files and writing streams.

### 4.3 The environment of a command

| Flag | Declares |
| --- | --- |
| `--var NAME=TEXT`, `--vars FILE.json`, `--env NAME` | Variables, as F10 9.4 says |
| `--description FILE` | The environment that the description in FILE gives (F1 5.3) |
| `--group NAME` | Turns the group NAME on (F6 2.1). It may be repeated |
| `--steps N`, `--allocation N`, `--depth N` | That limit (`03-host.md` 3.6.1), a whole number of 0 or more |

1. With no `--description`, the environment is the default environment, with
   the variables, groups and limits the flags give.
2. With `--description`, the environment is the description's. `--var`,
   `--vars` and `--env` then only supply values: each names a variable the
   description declares, and a name it does not declare is an error of the
   command line. `--group` and the limits add to the description's.
3. A description that declares a host function has no implementation for it: a
   run fails at compile with `no-implementation` (`03-host.md` 3.2.1 item 6).
   `check` needs none.
4. `--type TEXT` may name a named type the environment declares (F4 3).
5. No run has a deadline. The limits bound it.

### 4.4 `eval`

1. F10 9.2 fixes the steps, the value written and its text.
2. On success, the value is written to standard output and nothing to standard
   error. On failure, nothing is written to standard output, and the
   diagnostics to standard error (4.2 item 3).
3. `--max-failures N` is the maximum of step 4 of a load (F10 5.2), a whole
   number of 1 or more. Its default is 10.
4. `--format json` sets the form of the value (F10 9.2) and of the
   diagnostics.

### 4.5 `check`

1. F10 9.3 fixes the steps.
2. With no diagnostic, nothing is written, or with `--print-type` the printed
   result type and U+000A, to standard output.
3. With diagnostics, they are written to standard output (4.2 item 3).

### 4.6 `fmt`

1. **Input.** Each PATH that is a file is formatted. A PATH that is a directory
   is searched, through its subdirectories, for files whose names end in
   `.risor`, by the rules of F9 8.2 items 2 and 3 for the search and the order.
   With no PATH, standard input is formatted.
2. **With neither `--write` nor `--check`,** the formatted text of each file is
   written to standard output, in the order of item 1.
3. **`--write`** writes each file's formatted text back to the file when it
   differs from the file's text, and writes nothing to standard output. It
   cannot be given with standard input.
4. **`--check`** writes nothing to the files. It writes to standard output the
   path of each file whose text differs from its formatted text, one per line,
   as given or found. For standard input it writes `-`.
5. **`--template`** reads each input as a template and formats it by 2.14. A
   directory is not searched under `--template`.
6. **A file that does not parse** is not written and gives no formatted text.
   Its path, as given or found, is written to standard error on a line of its
   own, then its first `parse` diagnostic in the detail form of F8 5.3.
7. Every file is tried, in order, whatever the earlier ones gave.

### 4.7 `test`

F9 8 fixes the command. This package adds:

1. `--color WHEN` (4.2 item 4). F9 8.3 item 4 says which words may be coloured.
2. There is no progress output and no watch mode in 3.0. Standard error is
   empty unless a PATH does not exist (F9 8.2 item 4).
3. **A program case file that does not load** (F9 5.2) is reported by the lines
   of F9 6.3: `  INVALID` and the path of the file, then, for each diagnostic of
   the step that failed (4.2 item 3), four spaces and its one-line form (F8
   5.2), without the colour of 4.2.

### 4.8 Exit status

| Status | When |
| --- | --- |
| 0 | The command did what was asked: `eval` wrote a value; `check` found no diagnostic; `fmt` formatted every file, and under `--check` every file was formatted already; `test` as F9 8.5 says |
| 1 | `eval` or `check` failed with diagnostics, of any phase (F10 9.5); `fmt` met a file that does not parse, or under `--check` a file that is not formatted; `test` as F9 8.5 says |
| 2 | An error of the command line (4.9); `test` as F9 8.5 says |

### 4.9 Errors of the command line

These are errors of the command line: an unknown command or flag; a flag with
no value or a value of the wrong form; `--type` with `--schema`; `--write` with
`--check`, or with standard input; a FILE or PATH that cannot be read, or a file
that cannot be written; input that is not UTF-8; a variable named by two flags
(F10 9.4 item 2), or one a description does not declare (4.3 item 2); a
`--vars` file that is not a JSON object; a description that is not one (F1
5.3). Each writes one or more lines to standard error, whose text is not
specified, writes nothing to standard output, and ends with status 2.

## 5. The REPL

The REPL is a product of the Go implementation. This section says what its
values mean. Its prompts, its editing of lines, its commands and its output
around values are its own.

1. `risor`, with no command, starts the REPL in an environment the flags of
   4.3 give.
2. **An entry** is the text the author gives before it is evaluated: zero or
   more `const` lines, then one expression or nothing. An entry that does not
   yet parse, because the end of its text is the end of input where the
   grammar needs more, is read on: more lines are joined to it.
3. **Cumulative `const` lines.** The session holds the `const` lines of the
   entries that succeeded, in order. An entry's **program** is the held lines,
   then the entry's own `const` lines, then its expression, or `null` when it
   has none. Its value is the value of that program, checked and run by
   `03-host.md` in the session's environment.
4. When the program fails, in any phase, the REPL shows its diagnostics, and the
   session holds nothing of the entry. When it succeeds, the entry's `const`
   lines are held, and the value is shown. An entry with no expression shows no
   value.
5. A name the held lines bind cannot be bound again: the program is
   `duplicate-name` (`05-types.md` 5.1). The REPL may offer a command that
   forgets the held lines.
6. Since a program is pure, the REPL may keep the values of the held lines and
   evaluate only an entry's own text. The value shown must be the value of the
   entry's program. The usage it shows, if any, is its own.
7. Its value is shown in the JSON form of `01-language.md` 12, so that a string
   shows its quotes; an opaque value cannot be shown. Its diagnostics are shown
   in the detail form of F8 5.3, with the entry's text.
8. Its tests are its own.

## 6. The editor grammar

The editor grammar colours a program in an editor: a TextMate grammar for
`.risor` files, and an injection that colours the parts of a template (F2) in a
YAML file. It is built once, for every implementation. It is the one place a
generated grammar is allowed: no implementation depends on it
(`14-implementation-plan.md` 4.3).

1. It recognizes every token class of `01-language.md` 2.3: words, the 26
   reserved words of 2.4 with the grammar's own words told from those reserved
   for later, `true`, `false` and `null`, numbers in every form of 2.5, the
   one-line strings and their escapes of 2.6 and 2.7, templates and their
   interpolations of 2.8 nested to any depth, brackets, separators, operators,
   and both kinds of comment.
2. It recognizes every string form of F3: the raw string `r"..."` and `r'...'`,
   the text block, the raw text block, and the escapes and interpolations of a
   text block.
3. It recognizes the words of a program type after `as` and `is` (F4), and the
   star step (F7).
4. It need not report errors: a text that does not lex may be coloured in any
   way.
5. It is tested by its own tests: texts and the scopes it must give each token.

## 7. Additions to the case format

Each addition is optional. Its absence leaves every existing case file valid,
with its meaning unchanged.

### 7.1 The format case

| Field | Holds |
| --- | --- |
| `format` | A source text to format as a program (2.1) |
| `formatTemplate` | A template text to format (2.14) |

1. A case with `format` or `formatTemplate` is a **format case**. It may have,
   after the header rule, only `name`, `format` or `formatTemplate`, `env`,
   `vars`, `skip` and `expect`.
2. Its `expect` is a **format result**, an object with the one field
   `formatted`, a string; or a failure of phase `parse` (`03-host.md` 3.10.6).
3. **How it is run.** The text is formatted. A text that does not parse gives
   the outcome of a `parse` failure, compared as `03-host.md` 3.10.7 says. A
   text that parses gives a format result: its `formatted` is compared with the
   expected one, code point for code point.
4. **The laws.** For a text that parses, the runner also tests:
   - **idempotent**: formatting the formatted text gives that text;
   - **tree**: for `format`, the formatted text parses to a tree equal to the
     source's (2.2 item 1); for `formatTemplate`, the template parses, and its
     segments' values and its parts' trees are equal to the source's;
   - **run**: when the case has `vars`, the source and the formatted text are
     each checked against `env` and run with the `vars`, and the two outcomes
     agree in phase, type, value, code, class, message, steps and units.
   A law that does not hold is a difference, whatever `expect` says.
5. The differences are named `outcome`, `diagnostics`, `formatted`,
   `idempotent`, `tree` and `run`, in that order.
6. A format case is run once. It has no fixture test (F9 2).

### 7.2 The completion case

| Field | Holds |
| --- | --- |
| `complete` | A text in which the code point U+2038 `‸` stands exactly once. The text without it is the source, and its place is the position |

1. A case with `complete` may have, after the header rule, only `name`,
   `complete`, `version`, `env`, `opaque`, `methods`, `functions`, `removed`,
   `groups`, `types`, `returns`, `skip` and `expect`. Its environment is built
   from them as for a program case (`03-host.md` 3.10.3 item 3), and `returns` is
   the expected result. A stub's rows are not used.
2. Its `expect` is `{"completion": C}` or a failure of phase `declare`. C is an
   object with `span`, the prefix span, and `items`, a list of candidates, each
   an object with `label` and `kind`, and optionally `detail` (a string or null)
   and `insert` (a string).
3. **How it is compared.** `span` is compared. `items` is compared whole: the
   same number of candidates, and each expected one with the actual one at the
   same place, its `label` and `kind`, and its `detail` and `insert` when the
   expected candidate has them. A difference is the difference `completion`.

### 7.3 The hover case

| Field | Holds |
| --- | --- |
| `hover` | A text in which `‸` stands exactly once, as in 7.2 |

1. A case with `hover` may have the fields of 7.2 item 1, with `hover` in
   place of `complete`.
2. Its `expect` is `{"hover": R}` or a failure of phase `declare`. R is null, for
   no hover, or an object with `kind` and `span`, and optionally `type`,
   `signature`, `doc` (each a string or null) and `declaration` (a span or
   null).
3. **How it is compared.** R null is compared with no hover. Otherwise each field
   R has is compared, the texts code point for code point. A difference is the
   difference `hover`.

### 7.4 The header rule and invalid files

1. `source`, `file`, `template`, `conform`, `render`, `runner`, `format`,
   `formatTemplate`, `complete` and `hover` count as one field for the header
   rule (`03-host.md` 3.10.2 item 4). None of the last four may stand in a
   header.
2. These join the list of `03-host.md` 3.10.10: a format, completion or hover
   case with a field 7.1, 7.2 or 7.3 does not allow; `format`,
   `formatTemplate`, `complete` or `hover` that is not a string; a `complete`
   or `hover` text in which `‸` does not stand exactly once; an `expect` that
   is not of the form 7.1, 7.2 or 7.3 gives, or that has `formatted`,
   `completion` or `hover` in a case of another kind; a candidate or a hover
   record with a field those sections do not name, or of the wrong JSON type.

### 7.5 The JSON Schema of the additions

These are added to the schema of `03-host.md` 3.10.11: the four fields in the
list of `propertyNames` of a case (not of the file), their properties in
`$defs/case`, and three branches of `$defs/expect`.

```json
{
  "case": {"properties": {
    "format": {"type": "string"},
    "formatTemplate": {"type": "string"},
    "complete": {"type": "string"},
    "hover": {"type": "string"}}},
  "formatResult": {
    "type": "object", "required": ["formatted"],
    "properties": {"formatted": {"type": "string"}},
    "additionalProperties": false},
  "completionResult": {
    "type": "object", "required": ["completion"],
    "properties": {"completion": {
      "type": "object", "required": ["span", "items"],
      "properties": {
        "span": {"$ref": "#/$defs/span"},
        "items": {"type": "array", "items": {
          "type": "object", "required": ["label", "kind"],
          "properties": {
            "label": {"type": "string"},
            "kind": {"enum": ["name", "lambda", "variable", "function",
                              "namespace", "field", "method", "constant",
                              "key", "literal", "type"]},
            "detail": {"type": ["string", "null"]},
            "insert": {"type": "string"}},
          "additionalProperties": false}}},
      "additionalProperties": false}},
    "additionalProperties": false},
  "hoverResult": {
    "type": "object", "required": ["hover"],
    "properties": {"hover": {"oneOf": [
      {"type": "null"},
      {"type": "object", "required": ["kind", "span"],
       "properties": {
         "kind": {"enum": ["name", "lambda", "variable", "function", "namespace",
                           "field", "method", "constant"]},
         "span": {"$ref": "#/$defs/span"},
         "type": {"type": ["string", "null"]},
         "signature": {"type": ["string", "null"]},
         "doc": {"type": ["string", "null"]},
         "declaration": {"oneOf": [{"type": "null"}, {"$ref": "#/$defs/span"}]}},
       "additionalProperties": false}]}},
    "additionalProperties": false}
}
```

### 7.6 The directory and the runner

1. The cases of this package are in `suite/tools/`. A runner of an
   implementation that offers F12 runs them; a runner of one that does not
   offers none of these fields, and does not run the directory (`03-host.md`
   3.11 items 2 and 3).
2. Each case of `suite/tools/` is run once. Completion and hover always use the
   full checker; a format case checks only for the law `run`.
3. An implementation should also run its formatter over every program of the
   suite and test the laws of 2.2 on each: the suite's programs reach every form.

### 7.7 The `outcome` command

The `outcome` command (`suite/README.md`) writes, for a case of this package:

| Case | Line |
| --- | --- |
| A format case | `name`, then `formatted`; or, when the text does not parse, `phase` and `diagnostics` as for a `parse` failure |
| A completion case | `name`, then `completion`: `span` and `items`, each candidate with `label`, `kind`, `detail` and `insert` |
| A hover case | `name`, then `hover`: null, or `kind`, `span`, `type`, `signature`, `doc` and `declaration` |

A declare failure of a completion or hover case is written as for a program
case. The fuzzer compares these fields among the implementations that write
them.

## 8. Costs, and earlier cases

1. The formatter and assist charge no step and no unit. The CLI's runs charge
   what their programs charge.
2. No earlier case changes its outcome. This package gives no text a new
   meaning, adds no code and changes no diagnostic. No `until-f12-` file exists,
   and this package moves none.
3. The fields of 7 are new. No earlier file uses them.
