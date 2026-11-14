# Risor v3 design: regular expressions

Status: plan. The owner settled on 2026-11-14 that regular expressions come
now, as a follow-on package, which reverses D9 (D52 in `00-overview.md`). The
decisions this conversation settled are marked so in section 9. The others are
recommendations. No code was written and nothing was measured. Its decisions
are numbered `16-1` to `16-12` (section 9).

**How to read this.** Section 3 is the proposal, written as rules so that it can
move into a specification chapter. A paragraph headed *Why* is not a rule.
Sections 1, 2 and 4 to 10 are not rules.

**Evidence.** **(R)** read in the specification or a sibling document at the
place cited. **(from memory)** not checked for this draft. **(I)** inferred.
`06:` to `15:` are the sibling documents in this directory.

## 1. Summary

1. **The problem.** Agent tools take values out of text: an id from a log
   line, a version from a command's output, a field from an e-mail. Today that
   is a chain of `split` (#640, friction 7). Validation in config files and
   workflow conditions needs the same.
2. **The proposal.** Package F15 adds the opt-in group `regexp` with 7 entries:
   `regexp.test`, `find`, `findAll`, `replaceAll`, `replaceWith`, `split` and
   `escape`. The pattern comes first.
3. **One dialect and one matcher.** The dialect is the subset of `06:2.8`, with
   the changes of 3.2. Every implementation builds the same matcher from the
   specification: a thread-set simulation with priority (a Pike VM). No
   implementation uses its platform's engine for a result.
4. **Patterns are checked when they are constant.** A pattern written as a
   literal is compiled when the program is checked. A bad one is a static
   error. A call with a constant pattern cannot fail, so it records no site.
5. **Groups are typed.** With a constant pattern, `find` gives a record whose
   `groups` field has one field for each named group. A group that may not take
   part in a match is `string | null`.
6. **Cost is exact.** A call charges for the pattern and for the matcher's work,
   counted in the matcher's own units, as it goes. Every implementation stops
   at the same point.
7. **`regexp` is reserved in Core now,** whatever else happens. Today it is not
   (3.8).

## 2. The problem

### 2.1 Evidence

| Source | What it shows |
| --- | --- |
| #640, friction 7 | `int(args.text.split("#")[1].split(" ")[0])` takes `1234` from `"Order #1234 shipped"`. It is wrong for `"#12 of #34"` and fails on text with no `#` |
| #640, question 1 | Agent tooling is a consumer `06:2.8` did not count. Its first reason, "No consumer has them", no longer holds |
| `06:2.8` (R) | The subset, the entries, the cost and the matcher are already designed. The other reasons were cost of portability and that a released entry cannot change |
| `00-overview.md` T4, R11 (R) | Go's and Rust's engines are linear but differ in syntax. JavaScript's backtracks and matches UTF-16 units |

### 2.2 Why the old reasons no longer decide it

| Reason in `06:2.8` | Now |
| --- | --- |
| No consumer has them | Agent tools, config validation and workflow conditions are consumers (#640) |
| The most expensive entries to make portable | Still true. About 700 lines in each implementation (`06:2.8`, I). This plan pays it once, by specifying one algorithm |
| A released entry cannot change | Answered by keeping the dialect strict: every doubtful form is an error, so it can be given a meaning later without a break (3.2) |
| The gap is small | It is not for extraction: `split` cannot say "digits after `#`" |

A host can declare its own pattern functions today. That gives each host a
dialect of its own and no checking, and it is what C1 is meant to avoid.

## 3. The proposal

### 3.1 The package

| Matter | Rule |
| --- | --- |
| Name | F15, regular expressions. F11 is the Go typed layer and has no chapter of its own (`14-implementation-plan.md`, F11) (R) |
| Order | After F14 and before F16 to F19 and F12, which stays last: F3, F1, F2, F4, F5, F6, F7, F8, F9, F10, F11, F13, F14, F15, F16, F17, F18, F19, F12 (`00` section 4) |
| Requires | F6, for the group mechanism. F3 is before it, so raw strings are there for patterns |
| Group | `regexp`, turned on as F6 2.1 says. F6 2.1 item 3 lists `regexp` as a name that is not a group; with F15 it is one |
| Version | `since` `3.0` |
| Files | `suite/library-regexp.json` (7 records, the format of F6 3.2, plus the field `pattern` of 3.6); `suite/messages-regexp.json` (3.7); cases in `suite/regexp/` |
| Adds | 7 library entries, 1 check code, 2 templates of `invalid-argument`, 1 record field, 1 typing rule. No node kind, value, type, run code or reserved word |

### 3.2 The dialect

The matching unit is the code point. Every position and length is in code
points, as everywhere in the library (`04-library.md` 3.5 item 1) (R).

| Area | In | An error |
| --- | --- | --- |
| Literals | Any code point but the metacharacters `\ . + * ? ( ) \| [ ] { } ^ $` | — |
| Escapes | `\` before any ASCII punctuation; `\n \r \t \f \v`; `\u{H...}`, one to six hex digits naming a scalar value | `\` before a letter or digit not listed here; `\x`; octal; `\c`; `\Q...\E` |
| Classes | `[abc]`, `[^abc]`, ranges by code point, escapes inside; `-` literal first, last, or escaped | An empty class `[]`; a range whose ends are reversed; an unescaped `[` or `]` inside a class |
| Shorthand | `\d` is `[0-9]`, `\w` is `[0-9A-Za-z_]`, `\s` is `[\t\n\v\f\r ]`, and `\D \W \S` their complements. Also inside a class | `\p{...}`, `[[:alpha:]]` |
| Any | `.` is any code point but U+000A. Under `(?s)`, any code point | — |
| Anchors | `^`, `$` at the start and end of the text. Under `(?m)`, also after and before U+000A. `\b`, `\B` on the `\w` set | `\A`, `\z`, `\Z`, `\G` |
| Repeats | `*`, `+`, `?`, `{n}`, `{n,}`, `{n,m}`, each greedy or lazy (`?` after) | A count over 1,000; `{n,m}` with m < n; `{,m}`; a `{` that does not begin a valid repeat; a repeat with nothing before it; possessive forms; **a repeated part that can match the empty string** |
| Groups | `(...)`, `(?:...)`, `(?<name>...)`, name a `Name` of `01-language.md` 2.4 | Two groups of one name; more than 32 capturing groups; back-references `\1`, `\k<..>`; look-around; atomic groups; conditionals |
| Flags | `(?i)`, `(?m)`, `(?s)` and combinations such as `(?im)`, only at the start of the pattern | A flag anywhere else; `(?i:...)`; an unknown flag |
| Case | Under `(?i)`, two code points match when they are in one class of the case table's simple mappings (3.3) | — |
| Size | At most 10,000 instructions after compiling (3.4) | A larger pattern |

Changes from `06:2.8`:

| Change | *Why* |
| --- | --- |
| `\u{H}` in place of `\x{H}` | It is the spelling of a code point in Risor strings and in JavaScript patterns with the `u` flag (from memory). One spelling |
| A `{` that is not a repeat is an error | JavaScript and RE2 read it as a literal (from memory). Strict now, so a later meaning is no break |
| An unescaped `[` in a class is an error | It keeps room for set operations, as JavaScript's `v` flag added them (from memory) |
| `(?i)` follows the case table, not ASCII only | `toUpperCase` already uses `suite/casemap.json` (R). A pattern that folds `é` and `É` is less surprising than one that does not. The table is data every implementation already has |
| At most 32 capturing groups and 10,000 instructions | Bounds the work for each instruction a thread visits, so cost stays proportional to what is charged (`02-evaluation.md` 8.5) |
| `\v` added to `\s` | It is in `\s` in JavaScript and RE2 (from memory). `\d \w \s` stay ASCII |

*Why the empty repeat is an error.* A repeat of something that can match empty
(`(a*)*`, `(a|)+`, `(\b)*`) is where engines disagree on captures, and where a
thread simulation and a backtracking matcher can disagree on priority (from
memory, RE2 and Rust `regex` documentation). Without it, the two agree (I). The
rewrite is always possible: `(a*)*` is `a*`.

### 3.3 Case-insensitive matching

1. The **case class** of a code point c is the set reached from c by any number
   of steps of the simple upper and lower mappings of `suite/casemap.json`, in
   either direction.
2. Under `(?i)`, a literal c matches d when d is in c's case class. A class C
   matches d when some member of d's case class is in C. A complemented class
   matches d when no member is.
3. `\d`, `\w`, `\s`, `\b` and `.` do not change under `(?i)`.

### 3.4 The matcher

The specification gives one algorithm. Its results, and its count of visits
(3.5), are normative. An implementation may use another algorithm only if it
gives the same results and the same count on every input. The platform engines
do not (2.1).

**The program.** A pattern compiles to a list of instructions.

| Instruction | Meaning |
| --- | --- |
| `char c`, `class C`, `any` | Consume one code point that matches |
| `split x, y` | Go on at x and at y; x has priority |
| `jmp x` | Go on at x |
| `save k` | Record the position in slot k |
| `assert a` | Go on only if a holds here: `^`, `$`, `\b`, `\B` under the flags |
| `match` | A match ends here |

| Pattern | Code |
| --- | --- |
| The whole pattern e | `save 0`; e; `save 1`; `match` |
| e1 e2 | e1; e2 |
| e1 \| e2 | `split L1, L2`; L1: e1; `jmp L3`; L2: e2; L3: |
| e* | L1: `split L2, L3`; L2: e; `jmp L1`; L3:. Lazy: `split L3, L2` |
| e+ | L1: e; `split L1, L3`; L3:. Lazy: `split L3, L1` |
| e? | `split L1, L2`; L1: e; L2:. Lazy: `split L2, L1` |
| e{n} | e, n times |
| e{n,} | e, n times; then e* |
| e{n,m} | e, n times; then m − n nested e?: `(e(e)?)?` for m − n = 2 |
| ( e ) for group k | `save 2k`; e; `save 2k+1` |
| (?: e ) | e |

The size n of a pattern is its number of instructions.

**A search from position p.**

1. The current list starts empty. At each position i from p to the end of the
   text, inclusive:
   - If no match has been found, add a thread at instruction 0 with every slot
     empty, after the threads already in the list.
   - Take the threads of the current list in order. A `match` thread records
     its slots as the match and ends the loop for this position: the threads
     after it have lower priority and are dropped. A consuming thread whose
     instruction matches the code point at i adds a thread at its next
     instruction to the next list, at position i + 1.
   - If the next list is empty and a match was found, stop. Otherwise go on.
2. **Adding a thread** at instruction x and position i: if x is already in the
   list for position i, do nothing. Else count one visit and mark x. Then
   follow `jmp` to its target; `split` to x first, then y; `save` by recording
   i in a copy of the slots; `assert` only when it holds at i. A consuming
   instruction or `match` is put on the list.
3. The match is the slots of the recorded thread: slot 0 to slot 1 is the
   match, slots 2k and 2k+1 group k. An empty slot is a group that took no
   part.

*Why this algorithm.* Its work is at most n visits at each position, so a
search is linear in the text for a fixed pattern. It gives the leftmost match,
and among those the one a backtracking matcher would find first (from memory;
the restriction of 3.2 makes this hold, I).

**Several matches.** `findAll`, `replaceAll`, `replaceWith` and `split` search
from position 0. After a match from s to e, the next search starts at e when
e > s, and at e + 1 when the match is empty. An empty match where the previous
match ended is a match. So `a*` in `baaac` matches `""` at 0, `aaa` at 1, `""`
at 4 and `""` at 5. This is JavaScript's rule (from memory); Go drops the empty
match at 4 (from memory).

### 3.5 The entries

`M` is the type of one match. With a pattern that is not constant (3.6) it is
`{text: string, index: int, groups: map<string | null>, captures: (string |
null)[]}`.

| Entry | Signature | Result |
| --- | --- | --- |
| `regexp.test` | `(pattern: string, s: string) => bool !` | Whether a match exists |
| `regexp.find` | `(pattern: string, s: string) => M \| null !` | The first match, or null |
| `regexp.findAll` | `(pattern: string, s: string) => M[] !` | Every match, by 3.4 |
| `regexp.replaceAll` | `(pattern: string, s: string, replacement: string) => string !` | s with every match replaced by the expansion of `replacement` |
| `regexp.replaceWith` | `(pattern: string, s: string, f: (m: M) => string) => string !` | s with every match replaced by what f gives for it |
| `regexp.split` | `(pattern: string, s: string) => string[] !` | The parts of s between matches |
| `regexp.escape` | `(s: string) => string` | s with each metacharacter of 3.2 preceded by `\` |

| Field of M | Value |
| --- | --- |
| `text` | The text of the match |
| `index` | Its position, in code points, as `slice` counts |
| `groups` | One key for each named group, in pattern order. The value is the group's text, or null when it took no part |
| `captures` | Index 0 is `text`. Index k is group k's text, or null. So `captures[1]` is what `$1` gives |

1. **Replacement text.** `$$` is `$`. `$0` is the match. `$` and one or more
   digits is the group of that number: every digit is taken, so `$10` is group
   10. `$<name>` is a named group, and `$<1>` group 1, so `$<1>0` is group 1
   then `0`. A group that took no part gives the empty string. Any other `$`,
   and a group the pattern does not have, is an error.
2. **`split`.** The parts are the text between the matches of 3.4, skipping
   each empty match at position 0, at the end of the text, or where the
   previous kept match ended. Groups are not put in the result. If s is empty,
   the result is `[]` when the pattern matches the empty string and `[""]`
   otherwise. This follows Core's `split` (`04-library.md` 5.3.8) and
   JavaScript (from memory).
3. **`replaceWith`** calls f once for each match, in order. It is a library
   entry, so it may take a lambda (D34). Its result is tested to be a string.

*Why these seven.* `06:2.8` listed `test`, `find`, `findAll`, `groups`,
`replaceAll` and `escape`. `groups` is now a field of `find`'s result. `split`
and `replaceWith` cover the next most common uses (I). There is no `replace`
for the first match only: `find` and `slice` do it, and two spellings of
replacement would invite the JavaScript mistake of forgetting `g`.

### 3.6 Constant patterns and the checker

1. A **constant pattern** is an argument to a `pattern` parameter that is a
   string literal of any form with no interpolation (F3 included), or an
   `Ident` bound by a `const` whose value is a constant pattern. A
   `replacement` argument is constant by the same rule.
2. The record of each entry but `escape` has the field `pattern`, naming the
   parameter. `replaceAll`'s record also names `replacement`.
3. **A static rule** (`05-types.md` 22). A constant pattern that breaks 3.2 is
   the check code `bad-pattern`, template `pattern`, on the argument. A
   constant replacement with a reference the constant pattern lacks, or a bad
   `$`, is `bad-pattern`, template `replacement`. The rule runs with or without
   the type checker (`05-types.md` 22.12).
4. **The type.** With a constant pattern, `M` is
   `{text: string, index: int, groups: G, captures: C}`. G has one field for
   each named group, in order. A group is **always present** when it is not
   inside a `?`, a `*`, a repeat with minimum 0, or one side of a `|`. Its field
   is `string` when always present, else `string | null`. C is `string[]` when
   every group is always present, else `(string | null)[]`.
5. **The site.** A call whose every `pattern` and `replacement` argument is
   constant and passes item 3 records no `value` site (`05-types.md` 18.3). The
   entry cannot fail then: its other parameters are typed.
6. A pattern that is not constant is compiled at each call. A bad one fails the
   run with `invalid-argument`, template `bad-pattern`.

| Program | Type or diagnostic |
| --- | --- |
| `regexp.find(r"#(?<id>\d+)", s)` | `{text: string, index: int, groups: {id: string}, captures: string[]} \| null` |
| `regexp.find(r"(?<v>\d+)(?<pre>-\w+)?", s)` | `groups: {v: string, pre: string \| null}` |
| `regexp.test("(a*)*", s)` | `bad-pattern`: a repeated part can match the empty string |
| `regexp.replaceAll(r"(\d+)", s, "$2")` | `bad-pattern`, template `replacement`: no group 2 |
| `regexp.test(p, s)`, `p: string` | `bool`, and a `value` site on the call |

*Why constant patterns only through `const`.* It follows the narrowing rule for
a const-bound lambda (`05-types.md` 13.8) (R). A pattern from a variable or a
parameter is data, and is checked when it arrives.

### 3.7 Failures and messages

| Failure | Code | Template | Parameters |
| --- | --- | --- | --- |
| A constant pattern breaks 3.2 | `bad-pattern` (check) | `pattern` | `reason`, `at` |
| A constant replacement is bad | `bad-pattern` (check) | `replacement` | `reason`, `at` |
| A pattern at run time breaks 3.2 | `invalid-argument` | `bad-pattern` | `reason`, `at` |
| A replacement at run time is bad | `invalid-argument` | `bad-replacement` | `reason`, `at` |
| `replaceWith`'s callback gives a value that is not a string | `wrong-kind` | Core's template for a callback result | As in Core |

1. `at` is the code point offset in the pattern or replacement where the fault
   is found. `reason` is one phrase from a fixed table of about 15 rows in the
   chapter: `unclosed group`, `unmatched )`, `unclosed class`, `empty class`,
   `reversed range`, `unknown escape`, `nothing to repeat`, `count over 1000`,
   `bad repeat`, `repeat of an empty match`, `not in the subset: <feature>`,
   `duplicate group name`, `more than 32 groups`, `pattern too large`, `bad
   flag`, `no group <n>`, `bad $`.
2. Templates, in the style of F8 3.5: `pattern`: `the pattern is not valid at
   {at}: {reason}`. The run templates read the same.
3. The span of the check code is the argument. A position inside the literal is
   not given, because escapes make source and value offsets differ.
4. `catch` catches `invalid-argument` and `wrong-kind` as in Core. The check
   code reaches no run.

### 3.8 Reserving the namespace in Core

1. `03-host.md` 3.3.3 item 3 and the `reserved-namespace` row of 3.3.4 add
   `regexp` to `json`, `time`, `math` and `base64`. A host function may not be
   declared in it, in any environment.
2. This is a Core change, made now, before F15 is built. Without it a host can
   declare `regexp.match` today (R: `go/env.go:335`), and F15 would break it.
3. `06-stdlib.md` 2.8 and its namespace table already say `regexp` is reserved
   (R). The specification did not carry that through.
4. Done in pull request #642: `03-host.md` 3.3.3 and 3.3.4, the three
   implementations, and a `reserved-namespace` suite case.

### 3.9 Cost

| Entry | Steps | Units |
| --- | --- | --- |
| `test`, `find`, `findAll`, `split` | `sz(pattern) + (n + v) / 8` | `wm` plus the units of the result |
| `replaceAll` | the same, plus `sz(replacement) + sz(result)` | `wm + 1 + sz(result)` |
| `replaceWith` | the same as `replaceAll`, plus each call of f | the same, plus each call of f |
| `escape` | `sz(s) + sz(result)` | `1 + sz(result)` |

1. n is the pattern's size (3.4). v is the number of visits of 3.4 in all the
   searches of the call. `/ 8` rounds down. The divisor is to be calibrated
   before release (16-9).
2. `wm`, the working memory, is `1 + (n × (2 + 2c)) / 64`, rounded down, where
   c is the number of capturing groups. It is charged once, before the first
   search: two thread lists of n entries, each with its slots.
3. **Order.** The arguments are tested and the call's step charged
   (`04-library.md` 4.4). Then `sz(pattern) + n / 8` is charged and the pattern
   compiled; a bad pattern fails here. Then `wm`. Then the searches charge one
   step for every 8 visits as they make them, so a limit is reached at the
   same visit in every implementation. Each match's result units are charged
   before it is built.
4. A constant pattern is charged at every call, as one that is not. An
   implementation may compile it once; the count does not change.

*Why count visits.* `06:2.8` charged `8 × (1 + sz(pattern)) × sz(s)`, a bound
paid in full even when the first code point matches. Visits are exact and
usually far fewer (I). `findAll` can scan the text again after each match, for
patterns like `a.*b|a` (I), and visits charge for that where a bound on
`sz(s)` would not.

### 3.10 Raw strings and escaping

1. A raw string (F3 3) is the natural form for a pattern: `r"\d+"`. In a plain
   string the same pattern is `"\\d+"`. Both are constant patterns.
2. A template with an interpolation is not constant. To put text into a pattern
   literally, use `regexp.escape`: `` `^${regexp.escape(name)}:` ``. The call is
   then not constant, so it is checked at run time and records a site.
3. `regexp.match` cannot be written, because `match` is a keyword (`06:2.8`)
   (R). No entry is named `match`.

## 4. Examples

```js
// #640, friction 7
const m = regexp.find(r"#(?<id>\d+)", args.text);
if (m) { int(m.groups.id) } else { fail("no order number") }

// A version from command output
regexp.find(r"v(?<major>\d+)\.(?<minor>\d+)", out)?.groups

// Every key=value pair
regexp.findAll(r"(?<k>\w+)=(?<v>[^;]*)", header)
  .map(m => [m.groups.k, m.groups.v]).toMap()

// Mask e-mail addresses
regexp.replaceWith(r"(?<user>[\w.]+)@(?<host>[\w.]+)", text,
  m => `${m.groups.user[0]}***@${m.groups.host}`)
```

## 5. What changes in other documents

| Document | Section | Change |
| --- | --- | --- |
| `00-overview.md` | 4.3 | D9 marked reversed by D52. New row D52. T4 and R11 point to this plan |
| `00-overview.md` | 5.2 | "the library 78 entries" becomes 85 with F15 |
| `06-stdlib.md` | 2.8, namespace table, 2.12 drop table | Point to this plan. `regexp` is no longer "none in 3.0". The 20 deferred regexp entries of v2 map to the 7 here |
| `14-implementation-plan.md` | Package table, order | Add F15 with its order. About 900 lines a language (section 6) |
| `15-build-rulings.md` | R133 | The package order gains F15 before F12 |
| `spec/core/03-host.md` | 3.3.3 item 3; 3.3.4 `reserved-namespace` row; the package order (about line 1358) | Add `regexp`. Add F15 to the order |
| `spec/core/05-types.md` | 18.3; 22 | Point to F15 for the site rule and the static rule. The chapter holds the rules |
| `spec/packages/f06-groups.md` | 2.1 item 3; 1.5 and the order lines | `regexp` is a group name when F15 is offered. Add F15 to the order |
| `spec/packages/f08-errors.md` | 3.1 item 2 | The check codes become 30 with F15. `messages-regexp.json` is a package catalog file (3.1 item 6) |
| `spec/packages/f12-tools.md`, `f13-math.md` | The order lines | Add F15 |
| `spec/packages/f15-regexp.md` | New | The chapter |
| `go/env.go:335`, and the TypeScript and Rust lists of reserved namespaces | — | Add `regexp`, with a suite case for `reserved-namespace` |
| `suite/README.md` | — | List the new files |

Counts that move: library entries (Core 53; all packages 78 to 85); reserved
namespaces (4 to 5); check codes (29 to 30, with F15); templates of
`invalid-argument` (14 to 16, with F15); record fields (one new: `pattern`).

## 6. Cost to build

| Part | Each implementation | Notes |
| --- | --- | --- |
| Pattern parser and checks of 3.2 | About 300 lines | I |
| Compiler to the program of 3.4 | About 150 lines | I |
| The matcher, with the visit count | About 250 lines | `06:2.8` estimated 700 for the three parts together (I) |
| The seven entries, replacement text, split | About 200 lines | I |
| Typing rule of 3.6 | About 100 lines | In the checker; shares the parser |
| Suite | About 250 cases | Shared. The differential fuzzer in `fuzz/` gains a pattern generator |

TypeScript cannot use `RegExp`: it backtracks, and it matches UTF-16 units
(`06:2.8`) (R). Go must not use `regexp` either: its empty-match rule for
`FindAll` differs (from memory), and its count of work is not the count of
3.4. Rust has no library dependencies, so it writes its own in any case
(`AGENTS.md`).

## 7. Risks

| Risk | Answer |
| --- | --- |
| A rule for captures is wrong and permanent | The strict dialect, the empty-repeat error, and one algorithm. The suite holds cases for each row of 3.2 and each compile row of 3.4 |
| Charges are too high for a 10 KB log | Calibrate `/ 8` on real tool patterns before release (16-9). A host can raise the step limit |
| Users expect `\d` to match Arabic digits | `\d` is ASCII in RE2 and in JavaScript without `u` semantics on `\d` (from memory). `[0-9]` is what nearly every extraction means (I) |
| A `(?i)` class is large | The test is on the case class of the input code point, at most a few members (I), not an expansion of the class |

## 8. Alternatives not taken

| Alternative | Why not |
| --- | --- |
| Each implementation's own engine | Results differ on empty matches and captures, and JavaScript's engine is not linear (`06:2.8`) |
| A pattern literal `/.../` | A new token beside `/`, and a `RegExp` value kind. A constant string gives the same checking (3.6) |
| Methods on strings: `s.test(p)` | Groups have no methods (F6 1.2 item 1). The pattern-first function reads as `06` planned |
| Patterns as a value kind, compiled once | A new kind of value. 3.9 item 4 lets an implementation cache without one |

## 9. Decisions for the owner

| # | Decision | Recommendation | Strongest alternative |
| --- | --- | --- | --- |
| 16-1 | Regular expressions now, as package F15, `since` `3.0` | **Settled by the owner (2026-11-14).** Reverses D9 | Keep D9 and leave matching to hosts |
| 16-2 | One normative matcher that every implementation builds | **Settled by the owner (2026-11-14):** "one algorithm everywhere" | Platform engines, with a suite to catch differences |
| 16-3 | Reserve `regexp` in Core now | **Settled by the owner (2026-11-14)** | Reserve it when F15 ships |
| 16-4 | `find` gives `{text, index, groups}`, groups typed from a constant pattern | **Settled by the owner (2026-11-14).** This plan adds `captures`, a recommendation | `groups` as a separate entry, untyped |
| 16-5 | The seven entries of 3.5 | Recommended. No `replace` for the first match | Add `replace`; drop `replaceWith` |
| 16-6 | The dialect of 3.2: `06:2.8` with `\u{}`, strict `{` and `[`, limits of 1,000, 32 and 10,000 | Recommended | `06:2.8` unchanged |
| 16-7 | `(?i)` by the case table | Recommended | ASCII only, as `06:2.8` had |
| 16-8 | JavaScript's rule for empty matches and `split` | Recommended | Go's rule, which drops an empty match that abuts the last one |
| 16-9 | Cost by visits, divided by 8, charged as the search goes | Recommended. The divisor is calibrated before release | A bound paid up front: `8 × (1 + n) × sz(s)` |
| 16-10 | Replacement syntax `$n`, `$<name>`, `$0`, `$$` | Recommended | JavaScript's `$&` and two-digit guessing |
| 16-11 | Constant patterns are a static rule, and a call with them records no site | Recommended | Check patterns only at run time |
| 16-12 | An opt-in group, off by default | Recommended, as F6's groups are | Core entries, always on |

## 10. What I did not check

1. That the thread simulation of 3.4 gives the backtracking result for every
   pattern the dialect allows. I believe it from RE2 and Rust `regex`
   documentation (from memory). A differential test against a backtracking
   reference, on generated patterns, should prove it before release.
2. The behaviour of JavaScript, Go and Python for empty matches and `split`
   (from memory).
3. The line counts in section 6 (I), and whether the TypeScript matcher is fast
   enough over code points (I).
4. The size of case classes in `suite/casemap.json` (I).
5. The divisor 8 of 3.9: nothing was measured.
