# 02. Evaluation and cost

This chapter is normative. It fixes how a Core run program is evaluated, what
each form costs in steps and allocation units, and what the limits do. The
syntax, the values and the meaning of each form are in `01-language.md`. How a
host sets the limits and reads the usage, and the shape of a diagnostic, are in
`03-host.md`. The steps and units of each library entry are in `04-library.md`.
Types are in `05-types.md`.

## 1. Scope and terms

### 1.1 Scope

1. This chapter covers the forms of Core run. A form that Core does not have is
   not covered, and a later chapter adds rows for it. It never changes a row of
   this chapter.
2. "The reference evaluation" is the evaluation of the program as written, in
   the order of section 2, charging by sections 5 and 6. Section 10 says how far
   an implementation may depart from it.

### 1.2 Terms

| Term | Meaning |
| --- | --- |
| Step | The unit of work. Section 5 |
| Unit | The unit of what a run creates. Section 6 |
| Charge | To add steps or units to the run's usage, after testing them against the budget (8.2) |
| Usage | The steps and units charged so far in a run |
| Weight | Of a string: its number of code points, a code point above U+FFFF counted twice (4) |
| Size | Of a string: its weight divided by 64, rounded down (4) |
| Body | The program, or the body of a lambda |
| Call in progress | A call of a `const`-bound lambda by name, or a call that a library entry makes of a lambda passed to it, that has started and not ended (8.4) |
| Level | A level of nesting of the source text, as `01-language.md` counts it for the source nesting limit |
| Nesting depth | Of an expression: the number of levels open at its first token |
| Walk | A pass over the contents of a value: `==` and `!=` on lists and maps, and the rendering of a value as text |
| Host data | A value that the host supplied, read in place |
| `L(n)` | 0 when `n` is less than 2. Otherwise the smallest whole number `L` with 2 ** `L` >= `n` |

## 2. Evaluation order

### 2.1 General rules

1. Evaluation is strict. Every `const` line runs, in source order, whether or
   not its names are used.
2. Operands are evaluated left to right, before their operator.
3. A receiver is evaluated before its arguments. Arguments are evaluated left to
   right.
4. Every argument of a call is evaluated before any parameter of the callee is
   bound.
5. Only these forms evaluate fewer than all of their operands: `&&`, `||`, `??`,
   `? :`, `if`, `match`, `try`, a `?.` or `?.[` step that finds null, and a
   default in a pattern or a parameter. Each evaluates only what 2.2 says.
6. An operation charges its steps, then its units, then does its work. If a
   charge fails, the work is not done and the run ends (8.2).

### 2.2 Order by form

| Form | Order |
| --- | --- |
| Program, block, branch | Each `const` line in order, then the result expression |
| `const p = e;` | `e`, then the pattern `p` against its value (2.4) |
| Literal: number, string, `true`, `false`, `null` | Nothing to evaluate |
| Name | Reads the binding. A variable the host did not supply fails with `missing-variable` |
| List literal | Each item left to right. A plain item is its expression. A spread item is its expression, then the test that the value is a list, then the copy of its elements (5.2). Then the list's units, then the list is built |
| Map literal | Each entry left to right. A plain entry `k: e` is `e`. A shorthand entry `a` reads `a`. A computed entry `[k]: e` is `k`, then the test that its value is a string, then `e`. Each of these three then charges the size of its key (5.2). A spread entry is its expression, then the test that the value is a map, then the copy of its entries. A later entry with a key already present replaces it. Then the map's units, then the map is built |
| Template | Each part left to right. A text part is nothing. An expression part is its expression, then the rendering walk of its value (5.5). Then the template's step charge, then its units, then the string is built |
| Unary `!`, `-` | The operand, then the operator |
| Binary operator other than `&&`, `\|\|`, `??` | The left operand, the right operand, then the operator |
| `a && b` | `a`. If `a` is falsy, the value is `a` and `b` is not evaluated. Otherwise `b`, which is the value |
| `a \|\| b` | `a`. If `a` is truthy, the value is `a` and `b` is not evaluated. Otherwise `b`, which is the value |
| `a ?? b` | `a`. If `a` is not null, the value is `a` and `b` is not evaluated. Otherwise `b`, which is the value |
| `c ? a : b` | `c`. Then `a` if `c` is truthy, else `b`. The other branch is not evaluated |
| `if` | Each test in order, up to the first truthy one. Then the branch of that test, or the `else` branch if no test is truthy. No later test and no other branch is evaluated |
| `match` | The subject, once. Then each arm in order: the subject is compared with `==` to each literal of the arm, left to right, and the arm stops at its first equal literal. The first arm with an equal literal, or the arm `_`, is taken. Then the result of the arm taken. No other result is evaluated. If no arm is taken, the failure `no-match` |
| `try` | The body. If it gives a value, that is the value, and the handler is not evaluated. If it fails with a code that `catch` catches (12.2), the error value is charged (12.4), the name is bound, and the handler is evaluated. Its value is the value of the `try` |
| Chain | The primary, then each step left to right (2.3) |
| Call by name, `f(a, b)` | The arguments left to right, then the call (11) |
| Lambda | Evaluating a lambda runs nothing. Its body runs only when it is called (11) |

### 2.3 Chains

1. A chain is a primary and every `.name`, `?.name`, `[e]`, `?.[e]` and method
   call that follows it directly, as `01-language.md` defines it.
2. The steps of a chain are evaluated left to right. Each step receives the
   value of the steps before it as its receiver.
3. A step tests its receiver first, before its index expression or its arguments
   are evaluated:
   - For a `?.name`, `?.[e]` or `?.m(...)` step, a null receiver makes the value
     of the whole chain null. No later step runs, and no index expression or
     argument of a later step is evaluated or charged.
   - For a plain `.name`, `[e]` or `.m(...)` step, a null receiver fails with
     `null-operand`, and a receiver of a kind the step does not accept fails
     with `wrong-kind`.
   - For every method call step, the method is selected at run time from the
     receiver's kind and the name, whatever the checker inferred. Then the
     **shape tests** below are made, in this order, before any argument is
     evaluated and before the call's charge. The first that fails raises
     `wrong-kind` at the `MethodCall`, and nothing is charged for the call.

     | # | Test |
     | --- | --- |
     | 1 | The kind has a method of that name |
     | 2 | The method accepts the count of arguments |
     | 3 | Each argument at a callback parameter is a lambda: one written in place, or the name of a `const`-bound lambda |
     | 4 | Each argument at a parameter that is not a callback is not a lambda |
     | 5 | Each lambda has no more parameters than the entry calls it with |

     Tests 3 and 4 are one pass over the arguments, left to right, and the
     first argument that fails either gives the failure. Test 5 is a second
     pass, left to right. A method of an opaque type takes no callback, so for
     an opaque receiver test 4 fails for any lambda argument. The tests need
     only the program's text and the method's record, and an implementation
     may prepare them when compiling, but it raises them only at run time.
     Each test's message template is in `03-host.md`.
4. Then an index step evaluates its index expression and tests it: a null index
   or key fails with `null-operand`, and one of the wrong kind with `wrong-kind`
   (`01-language.md`). A method call step evaluates its arguments left to right.
5. A chain ends where its production ends. An index expression and each argument
   are expressions of their own, and a `?.` inside one does not end the outer
   chain.

### 2.4 Patterns

1. A pattern is matched against one value, in a `const` line or as a parameter
   when a lambda is called.
2. A name pattern binds the value. `_` binds nothing.
3. A map pattern first tests that the value is a map: null fails with
   `null-operand`, and another kind with `wrong-kind`. Then each field in
   written order: the key is looked up. If it is absent, or its value is null,
   the default is evaluated if there is one. If it is absent and there is no
   default, the failure is `missing-field`. A default sees the names bound to
   its left. Then the rest, if there is one: a new map of the entries whose keys
   no field names.
4. A list pattern first tests that the value is a list, with the same codes.
   Then each element in written order, `_` included: the element at its position
   is read. If it is absent, or null, the default is evaluated if there is one.
   If it is absent and there is no default, the failure is `index-range`. Then
   the rest, if there is one: a new list of the elements after the last position
   the pattern names.
5. A parameter whose argument is omitted or null takes its default, which is
   evaluated only then.

### 2.5 When a failure is raised

1. **Kind tests come before the charge.** An operation first tests that its
   operands are of kinds it accepts. A failure with the code `null-operand` or
   `wrong-kind` is raised there, and the operation charges nothing. Item 4 is
   the one exception. For an operator with two operands that rejects null, both
   operands are evaluated first. Then, if the left operand is null, or else the
   right one is, the failure is `null-operand`. Only then are the kinds tested,
   and a pair that does not go together is `wrong-kind`. Each of `"a" * null`,
   `null < [1]` and `[1] + null` fails with `null-operand`. This holds for every
   binary operator of `01-language.md` 14.5 that rejects null.
2. **Other failures come after the charge.** The operation charges, then does
   its work, and the work raises `divide-by-zero`, `number-range`,
   `index-range`, `missing-field` or `no-match`. What was charged stays charged.
3. A walk charges as it goes (5.5). A failure met inside a walk keeps what the
   walk charged before it.
4. **A failed element test is paid for.** A list method with no callback may
   scan the elements of its receiver with a test before its own charges, as
   `04-library.md` names for each entry: the list methods `min` and `max`,
   `sort`, `unique`, `sum`, `flat`, `join` and `toMap`. When such a test fails
   at an element, the call charges its 1 step, then 1 step for each element
   tested, the failing one included, and then raises the failure. So `[1, "a",
   2].sort()` charges 3 steps (W39). The functions `min` and `max` test their
   arguments as any function does, before the call's step and for nothing. If
   that charge passes the step budget, the failure is `steps-exceeded`. A scan
   that passes charges nothing of its own. A failed test of a callback's result,
   or of the keys a callback gave, charges nothing more: each of those values
   was paid for with the two steps of its element (5.6).
5. A library entry raises its other failures as `04-library.md` says. A null
   argument of an entry is `wrong-kind`, not `null-operand`. A host function
   raises its failure when it returns one (`03-host.md`).

### 2.6 Which failure wins

1. The outcome of a run is one value or one failure. A failure has one code.
2. The failure is the first one raised in the order of this section. Nothing is
   evaluated after it but what section 12 says.

## 3. Numbers at run time

### 3.1 Values

1. A number has the value of an IEEE 754 binary64 value that is finite. There is
   one zero: an operation whose IEEE result is negative zero gives zero.
2. A **whole number** is a number with no fractional part and a magnitude of at
   most 2^53 − 1 (9007199254740991).
3. No operation can tell how a number is stored. `2` and `2.0` are one value.

### 3.2 Operations

| Operation | Both operands whole | Otherwise |
| --- | --- | --- |
| `a + b`, `a - b`, `a * b` | The exact integer result. If it is not a whole number, `number-range` | The IEEE result, rounded to nearest, ties to even. If it is not finite, `number-range` |
| `-a` | The exact result. It is always whole | The IEEE negation |
| `a / b` | The IEEE quotient | The same |
| `a % b` | `a − b × trunc(a / b)`, computed exactly. The sign is the sign of `a`. A zero result is zero | The same |
| `a ** b`, `b` whole and >= 0 | Start with 1. For each bit of `b` from the highest: square, then multiply by `a` if the bit is set. Each square and product follows the first row, so the result is exact or `number-range` | The same steps with IEEE multiplication. If the result is not finite, `number-range` |
| `a ** b`, `b` whole and < 0 | `1 / p`, where `p` is `a ** -b` by the steps above with IEEE multiplication only. `p` infinite gives zero. `p` zero with `a` not zero fails with `number-range`. `a` zero fails with `divide-by-zero`. Otherwise a `1 / p` that is not finite fails with `number-range`: `0.5 ** -1024` does | The same |
| `a ** b`, `b` not whole | `wrong-kind` | The same |
| `==`, `!=`, `<`, `<=`, `>`, `>=` | Comparison of the two values | The same |

1. A zero divisor in `/` or `%` fails with `divide-by-zero`. So does `0 ** b`
   with `b` below zero.
2. A result of `/`, `%` or `**` that would be NaN or infinite fails with
   `number-range`. So `0.1 ** -400` fails with `number-range`, and `2 ** -2000`
   is `0`.
3. A result too small to represent is zero.
4. The tests of 2.5 apply: an operand that is null fails with `null-operand`,
   and one that is not a number with `wrong-kind`, before any charge. For `**`
   an exponent that is a number but not whole fails with `wrong-kind`, before
   the charge.
5. Converting a number to text, and text to a number, follow `01-language.md`.

### 3.3 Host numbers

1. An integer outside the whole range, a NaN or an infinity in host data cannot
   enter the language. Reading one fails with the code `number-range` and the
   class `environment`, at the span of the node that read it. `catch` catches
   the failure.
2. A host number is read when an operation yields it as a value: a variable
   read, a field or index read, a pattern binding, an element handed to a
   callback, an element that a library entry or a walk tests, compares, adds or
   renders, and an argument passed to a host function. Handing the result to
   the host is not a read: the run makes no walk of its result, and a number
   inside host data that the program passed through untouched goes back as the
   host supplied it.
3. An operation that copies or measures a container without looking at its
   elements does not read them: `len`, a spread, `slice`, `reverse`.
4. The reading operation charges what it charges before it yields the value (2.5
   item 2). A variable read charges nothing.
5. A host negative zero reads as zero.

### 3.4 Rounding

Each operator rounds once. An implementation must not fuse two operators into
one rounding, and must not compute in a wider format.

## 4. Text size

1. The **weight** of a string is its number of code points, with each code point
   above U+FFFF counted twice. It is equal to the string's length in UTF-16 code
   units.
2. The **size** of a string is its weight divided by 64, rounded down.
3. The size of several strings taken together is the sum of their weights,
   divided by 64, rounded down.
4. The size of the longest common prefix of two strings is the size of the
   string made of the code points the two have in common at their start.
5. A string with a weight under 64 has size 0.

## 5. Steps

### 5.1 Rules

1. Each form charges the steps of 5.2 to 5.9, in addition to what its operands
   and parts charge.
2. A form's own steps are charged after its operands are evaluated and its kind
   tests pass (2.1, 2.5), and before its work, unless the table says otherwise.
3. Work that is not evaluated is not charged: a branch not taken, an operand a
   short circuit skipped, and every step a `?.` skipped.
4. A truthiness test charges nothing. This holds in `!`, `&&`, `||`, `? :`,
   `if`, and on the result of a callback.

### 5.2 Step table

| Form | Steps | When |
| --- | --- | --- |
| Number, string, `true`, `false`, `null` literal | 0 | |
| Name read | 0 | |
| `const` line, block, branch, program | 0 | |
| `&&`, `\|\|`, `??` | 0 | |
| `? :`, `if`, `else if`, `else` | 0 | |
| `try`, whether its body gives a value or fails | 0 | |
| Unary `-` on a number | 1 | After the operand |
| Unary `!`, on any value | 1 | After the operand |
| `+`, `-`, `*`, `/`, `%`, `**` on two numbers | 1 | After both operands |
| `+` on two strings | 1 + size of the result | After both operands |
| `<`, `<=`, `>`, `>=` on two numbers | 1 | After both operands |
| `<`, `<=`, `>`, `>=` on two strings | 1 + size of their longest common prefix | After both operands |
| `==`, `!=` | By 5.3 | After both operands |
| `in`, `not in` | By 5.3 | After both operands |
| `.name` or `?.name` on a map, with a receiver that is not null | 1 + size of the name, whether the key is present or not. The name is text of the program | After the receiver |
| `[k]` or `?.[k]` on a map, with a receiver that is not null | 1 + size of `k`, whether the key is present or not | After the key expression and its test |
| `[i]` or `?.[i]` on a list, with a receiver that is not null | 1, whether the position is present or not | After the index expression and its test |
| `[i]` or `?.[i]` on a string `s` | 1 + size of `s`, whether the position is present or not | After the index expression |
| A `?.`, `?.[` or `?.m(...)` step whose receiver is null | 0. The rest of the chain is not charged | |
| List literal, map literal, plain item of a list | 0 | |
| Entry of a map literal: written, shorthand or computed | Size of its key | At the entry's turn, after its key and value are evaluated |
| Element of a list copied by a spread in a list literal | 1 for each element | After the spread's expression and its test, before the next item |
| Entries of a map copied by a spread in a map literal | 1 for each entry, plus the size of the copied keys taken together | After the spread's expression and its test, before the next entry |
| Template | 1 + size of the result, once, plus the rendering walk of each part (5.5) | After every part and every walk |
| Lambda evaluated: a lambda written as an argument, or as the value of a `const` | 1 | Where the lambda stands, in the order of evaluation |
| Call of a `const`-bound lambda by name | 1, plus what the body charges | After the arguments (11.2) |
| Call of a library function or method | 1, plus the entry's formula and what the lambdas it calls charge (5.6) | After the receiver and the arguments, and their tests |
| Call of `fail(message, data?)` | 1, as any library call. Nothing for the message or the data | After the arguments |
| Call of a host function, or of a method that an opaque type declares | 1, plus what the function charges (5.8) | After the receiver and the arguments, before the function runs |
| `match` | 1 for each arm tested, in order, up to and including the arm taken. An arm with several literals counts 1. When no arm is taken, 1 for every arm. In addition, each literal compared that is a string, with a subject that is a string, charges the size of their longest common prefix, as `==` on two strings does. Literals of an arm are compared left to right, and the arm stops at the first equal one | Each arm's step before its comparisons. Each literal's prefix charge at its comparison |
| Field read by a map pattern | 1 + size of the field's key, for each field, whether the key is present or not | At the field's turn, before its default |
| Element read by a list pattern | 1 for each element of the pattern, `_` included, whether the position is present or not | At the element's turn, before its default |
| Element copied into the rest of a list pattern | 1 for each | After the elements |
| Entries copied into the rest of a map pattern | 1 for each, plus the size of the copied keys taken together | After the fields |
| A failure caught by `catch { }` | 0 | |
| A failure caught by `catch (err) { }` | By 12.4 | Before the handler |

### 5.3 Comparison and membership

| Form | Steps |
| --- | --- |
| `==` or `!=` on two operands of different kinds | 1 |
| `==` or `!=` on two operands of the same kind that is null, bool, number or opaque | 1 |
| `==` or `!=` on two strings | 1 + size of their longest common prefix |
| `==` or `!=` on two lists of different lengths | 1 |
| `==` or `!=` on two lists of the same length | 1 for each position compared, in order, up to and including the first position whose elements are not equal, or every position if none differs. Plus the steps of each comparison of two elements made, by this table |
| `==` or `!=` on two maps of different sizes | 1 |
| `==` or `!=` on two maps of the same size | 1 for each entry of the left map, for every entry, plus the size of the left map's keys taken together, charged with the entries. Plus the steps of the comparison of the two values under each key that both maps have, made for every such key, by this table. There is no early exit |
| `x in y`, `y` a list | 1 for each element `e` compared with `x`, in order, up to and including the first equal one, or every element if none is equal. In addition, for each `e` compared: when `x` and `e` are both strings, the size of their longest common prefix; when they are both lists of the same length, or both maps of the same size, the steps `x == e` charges by this table. Nothing more for any other pair |
| `x in y`, `y` a map and `x` a string | 1 + size of `x` |
| `x in y`, `y` and `x` strings | 1 + size of `y` |
| `x not in y` | What `x in y` charges |

1. `==` and `!=` never fail on kind. `in` and `not in` test kinds by 2.5: `y`
   null fails with `null-operand`, `y` of another kind with `wrong-kind`. When
   `y` is a map or a string, `x` null fails with `null-operand`, and `x` of
   another kind than string with `wrong-kind`.
2. The comparison of two lists or two maps of the same length or size is a walk
   (8.4). It enters one level for the pair.
3. The comparison of two maps visits the left map's entries in key order.
   The order can show only in which limit a run passes first, and only when a
   value under some key is a list or a map. An implementation may visit the
   entries in any order when no value of the left map is a list or a map.
4. **Searching text.** A search for a string `x` in a string `s`, as `x in s`
   makes, does work of at most a constant times `1 + w(s)`, where `w` is weight
   (4). A search for text longer than the text searched is answered from the two
   lengths, before anything else is done with `x`. This binds every search, in
   the language and in the library. The charge stays as the table gives it. An
   implementation may use its platform's search function only where that bound
   holds for it. Where it does not, it may still use it when the weight of `x`
   is under 64, and otherwise uses a search of its own that meets the bound.
5. **The key rule.** Wherever the language probes or builds a map by a key, it
   charges the size of the key in steps, because hashing or comparing a key
   reads its text: a read by `.name` or `[k]`, `x in m`, an entry of a map
   literal, a spread into a map literal, a field and the rest of a map pattern,
   and `==` on maps (5.2 and this table). A key is not copied, so no unit is
   charged for it. Library entries apply the same rule (`04-library.md`).

### 5.4 Text

1. Text created by `+` charges 1 + the size of the new string.
2. Text compared by `==`, `!=`, `<`, `<=`, `>` or `>=` charges 1 + the size of
   the longest common prefix.
3. `s[i]` and `s?.[i]` on a string `s` charge 1 + the size of `s`: finding a
   position is a read of the text. A position that is present creates a string
   of one code point (6.1).
4. Text read, created or compared by a library entry is charged as
   `04-library.md` says.

### 5.5 Templates and the rendering walk

1. A template charges 1 + the size of its result, once, for the whole string.
   The rendering of each part is not a string of its own and is not charged as
   text.
2. After each expression part is evaluated, the walk of its rendering is made.
   The rendering is that of `01-language.md`.
3. A part that is a string, a number, a bool or null adds nothing.
4. A part that is a list or a map is walked depth first. A list's elements are
   visited by index, and a map's entries in key order by code point. On entering
   each list or map, at any depth, the walk checks the depth limit (8.4) and
   then charges:

   | Entered | Steps |
   | --- | --- |
   | A list of `n` elements | `n` |
   | A map of `n` entries | `n + L(n) × (n + s)`, where `s` is the size of the map's keys taken together |

5. An opaque value met by the walk fails with `wrong-kind`. What the walk
   charged before stays charged, and the template's own step is not charged. So
   `` `${[1, x]}` `` with `x` opaque charges 2 steps on entering the list, then
   fails.
6. The same walk and the same charges apply wherever the language renders a list
   or a map as text. Library entries that render, such as `string`, charge as
   `04-library.md` says.

### 5.6 Calls made by the library

1. When a library entry calls a lambda passed to it, the entry's formula in
   `04-library.md` includes 1 step for each element it visits and 1 step for
   each call of the lambda. In the reference evaluation the two steps for an
   element are charged just before the lambda is called for that element.
2. The lambda's parameter patterns, its defaults and its body charge in
   addition, by this chapter, as they run.
3. A lambda passed by the name of a `const`-bound lambda charges what the same
   lambda written there would charge, less the 1 step of a lambda evaluated:
   that step was charged where the `const` evaluated the lambda, and a name read
   charges nothing.
4. A library entry calls a lambda with every argument its signature shows. A
   lambda that declares fewer parameters drops the others, at no charge.
5. A test of a callback's result, or of the keys a callback gave, that fails
   charges nothing more than the two steps already charged for each element (2.5
   item 4). Which tests an entry makes, and when, is in `04-library.md`.

### 5.7 Patterns

1. A map pattern charges 1 + the size of the key for each field, and then 1 for
   each entry copied into its rest plus the size of the copied keys taken
   together. A list pattern charges 1 for each element, `_` included, and then 1
   for each element copied into its rest.
2. A default charges what its expression charges, and only when it is evaluated.
3. A pattern that fails its first test (2.4) charges nothing.

### 5.8 Host functions

1. A call of a host function charges 1 step before the function runs. A method
   that an opaque type declares is a host function whose receiver is the opaque
   value (`03-host.md`), and charges the same.
2. The function charges further steps and units through its call handle
   (`03-host.md`). Each such charge is made at the moment the function makes it,
   and is tested against the budget then (8.2).
3. A host function that charges nothing costs the 1 step of its call.
4. Reading host data charges what reading any value charges: 1 for a field read
   or an index. Host data is never charged units.

### 5.9 What is not charged

1. Leaving a call, a branch or a `try`, on a value or on a failure.
2. Binding a name, by a `const`, a parameter or `catch (err)`.
3. A truthiness test.
4. Testing the depth limit.
5. Building a failure and its diagnostic, but for the error value of 12.4.

## 6. Allocation units

### 6.1 Unit table

| Value created | Units |
| --- | --- |
| Number, bool, null | 0 |
| A string literal | 0. It is text of the program |
| A string created by `+` or by a template | 1 + size of the string |
| A string created by `s[i]` or `s?.[i]` at a position that is present | 1. It has one code point, so its size is 0 |
| A list of `n` elements, from a list literal or the rest of a list pattern | 1 + `n` |
| A map of `n` entries, from a map literal or the rest of a map pattern | 1 + 2`n` |
| A lambda evaluated | 0 |
| The result of `!`, a comparison, `in`, arithmetic, `&&`, `\|\|`, `??`, `? :`, `if`, `match`, a read | 0. None creates a value |
| A `try` whose body gives a value | 0 |
| A failure caught by `catch { }` | 0 |
| The error value bound by `catch (err)` | By 12.4 |
| A call of `fail` | 0. Nothing is copied |
| A failure and its diagnostic | 0. A diagnostic is not a value of the language |
| A value a library entry creates | As `04-library.md` says |
| A value a host function creates | What the function charges through its call handle |

### 6.2 Rules

1. **Containers and new text, not contents.** A list is charged for its slots,
   and a map for its entries, not for the values in them. A value that already
   exists is not charged again when a list or map holds it.
2. **Final size.** `n` is the number of elements or entries of the value as
   created: for a map literal, after later entries have replaced earlier ones
   with the same key.
3. **Before the memory is taken.** A value's units are charged after the steps
   of the operation that creates it and before it is built. Where the size
   depends on the work, it is found first.
4. **As written.** A literal costs its units each time it is evaluated, even if
   an implementation builds it once. An intermediate value costs its units even
   if an implementation never builds it.
5. **Host data** is never charged.
6. **Scratch space,** working memory that is not a value, is not charged. An
   implementation keeps it within a constant times the values that exist or the
   units charged.

## 7. Worked counts

1. In these programs the names other than `const` names are variables, supplied
   by the host with the values shown. Host data costs no units (6.2).
2. Each count is the usage of the whole run, as a host reads it.
3. A library entry's own formula is taken from `04-library.md`. Two entries are
   used here: `map`, with steps `2n` and units `1 + n`, and `fail`, which adds
   nothing to the step of its call.
4. Where a message is built from the catalog, its size is taken as 0: a message
   with a weight under 64. The real count comes from `messages.json`.

| # | Program and variables | Steps | Units |
| --- | --- | --- | --- |
| W1 | `1 + 2 * 3` | 2: `*` 1, `+` 1 | 0 |
| W2 | `const xs = [1, 2, 3]; [...xs, 4]` | 3: the spread copies 3 elements | 9: the first list 1 + 3 = 4, the second 1 + 4 = 5 |
| W3 | `const m = {a: 1, b: 2}; {...m, b: 3, c: 4}` | 2: the spread copies 2 entries | 12: the first map 1 + 2 × 2 = 5, the second has 3 entries, 1 + 2 × 3 = 7 |
| W4 | `` `n=${n}, xs=${xs}` ``, with `n` 5 and `xs` `[1, 2]` | 3: the walk of `xs` 2, the template 1 + 0. The result `n=5, xs=[1,2]` has weight 13 | 1: 1 + 0 |
| W5 | `` `${m}` ``, with `m` `{b: 1, a: 2}` | 5: the walk of `m`, `n` = 2, `L(2)` = 1, keys of weight 2 so `s` = 0, gives 2 + 1 × (2 + 0) = 4. The template 1 + 0 for `{"a":2,"b":1}` | 1 |
| W6 | `[s + t, s == u, s < t]`, with `s` 100 × `a`, `t` 40 × `b`, `u` 70 × `a` then `c` | 6: `s + t` has weight 140 and size 2, so 1 + 2 = 3. `s == u` has a common prefix of weight 70 and size 1, so 1 + 1 = 2. `s < t` has no common prefix, so 1 | 7: `s + t` 1 + 2 = 3, the list 1 + 3 = 4 |
| W7 | `[e + "!", e[0]]`, with `e` 40 × U+1F600 | 4: `e + "!"` has weight 2 × 40 + 1 = 81 and size 1, so 1 + 1 = 2. `e` has weight 80 and size 1, so `e[0]` is 1 + 1 = 2 | 6: `e + "!"` 1 + 1 = 2, `e[0]` 1, the list 1 + 2 = 3 |
| W8 | `if (x > 10) { "big" } else if (x > 0) { "small" } else { "neg" }`, with `x` 5 | 2: the first test 1, the second test 1. The branch is a literal | 0 |
| W9 | `[user.name \|\| "anon", user?.nick ?? "none"]`, with `user` `{name: ""}` | 2: `.name` 1, which is falsy, so `"anon"` 0. `?.nick` 1, absent, so null, so `"none"` 0 | 3: the list 1 + 2 |
| W10 | `match kind { "bug" => 1, "feature" \| "idea" => 2, _ => 3 }` | With `kind` `"idea"`: 2, the first arm and the second. With `kind` `"x"`: 3, every arm, `_` taken | 0 |
| W11 | `a?.b.c` | With `a` null: 0. With `a` `{b: {c: 1}}`: 2, `.b` and `.c` | 0 |
| W12 | `a?.b?.c.d`, with `a` `{b: null}` | 1: `?.b` reads null. `?.c` finds null, and the rest is not charged | 0 |
| W13 | `xs.map(x => x * 2)`, with `xs` `[1, 2, 3]` | 11: the lambda 1, the call 1, `map`'s `2n` = 6, three bodies of 1 | 4: `map`'s `1 + n` |
| W14 | `const sq = x => x * x; sq(3) + sq(4)` | 6: the lambda 1 at the `const`, `sq(3)` 1 + 1, `sq(4)` 1 + 1, `+` 1 | 0 |
| W15 | `const {a, b = 10, ...rest} = m; [a, b, rest]`, with `m` `{a: 1, c: 2, d: 3}` | 4: the field `a` 1, the field `b` 1 (absent, so the default, a literal), the rest copies `c` and `d`, 2 | 9: the rest 1 + 2 × 2 = 5, the list 1 + 3 = 4 |
| W16 | `ps.map(({x, y = 0}) => x + y)`, with `ps` `[{x: 1}, {x: 2, y: 3}]` | 12: the lambda 1, the call 1, `map`'s `2n` = 4, and each body 3: the field `x` 1, the field `y` 1, `+` 1 | 3: `map`'s `1 + n` |
| W17 | `[[1, 2, 3] == [1, 2, 4], "b" in ["a", "b", "c"]]` | 8: `==` compares 3 positions, each 1 for the position and 1 for the comparison of two numbers, so 6. `in` compares 2 elements, so 2 | 15: three lists of 3 at 1 + 3 = 4 each, 12, and the outer list 1 + 2 = 3 |
| W18 | `{a: 1, b: 2} == {b: 2, a: 1}` | 4: 2 entries at 1, and 2 comparisons of two numbers at 1 | 10: two maps at 1 + 2 × 2 = 5 |
| W19 | `try { 1 / d } catch (err) { err.code }`, with `d` 0 | 3: `/` 1, then `divide-by-zero`. The message 1 + 0. `.code` 1 | 10: the error value 9 + (1 + 0) |
| W20 | `try { fail("no stock", {sku}) } catch (err) { err.data.sku }`, with `sku` `"a"` | 3: the call of `fail` 1. The message exists, so 0. `.data` 1, `.sku` 1 | 12: the map `{sku}` 1 + 2 = 3, the error value 9 |
| W21 | `try { xs[5] } catch { -1 }`, with `xs` `[1, 2]` | 2: the index 1, then `index-range`. `catch { }` 0. The unary `-` of `-1` 1 | 0 |
| W22 | `try { "n=" + n } catch (err) { err.code }`, with `n` 5 | 2: `+` on a string and a number fails with `wrong-kind` and charges 0 (2.5). The message 1 + 0. `.code` 1 | 10: the error value 9 + (1 + 0) |
| W23 | `xs.map(x => [x])`, with `xs` `[1, 2]` | 6: the lambda 1, the call 1, `map`'s `2n` = 4. Each body is a list literal at 0 | 7: each `[x]` 1 + 1 = 2, twice, 4, and `map`'s `1 + n` = 3 |
| W24 | `h(x) + 1`, with `h` a host function that charges 3 steps and 2 units and returns a number | 5: the call 1, `h`'s 3, `+` 1 | 2: what `h` charges |
| W25 | `["ab" in s, !xs]`, with `s` 200 × `a` and `xs` `[]` | 5: `in` on strings 1 + size of `s`, 1 + 3 = 4. `!` 1 | 3: the list 1 + 2 |
| W26 | `s in [t, s]`, with `s` 100 × `a` and `t` 70 × `a` then `b` | 4: `t` 1, plus the common prefix of weight 70 and size 1, so 2. `s` 1, plus the common prefix of weight 100 and size 1, so 2. It is equal, and the scan stops | 3: the list 1 + 2 |
| W27 | `[1, 2] in [[1, 3], [1, 2]]` | 10: each element is a list of the same length as `x`, so each costs 1 plus `x == e`. `[1, 3]`: 1 + (2 positions × (1 + 1)) = 5, the second position differs. `[1, 2]`: 1 + 4 = 5, equal | 12: four lists of 2 at 1 + 2 = 3 |
| W28 | `[s[3], s?.[200]]`, with `s` 130 × `a` | 6: `s` has weight 130 and size 2. `s[3]` 1 + 2 = 3. `s?.[200]` 1 + 2 = 3, and is null | 4: `s[3]` 1, the list 1 + 2 = 3. The null creates nothing |
| W29 | `m[k]`, with `k` 64 × `k` and `m` a map with the key `k` | 2: the read 1 + size of `k`, 1 + 1 | 0 |
| W30 | `m.aaaa…a`, a name of 64 letters `a`, with `m` a map that has that key | 2: the read 1 + size of the name, 1 + 1 | 0 |
| W31 | `k in m`, with `k` and `m` as in W29 | 2: 1 + size of `k`, 1 + 1 | 0 |
| W32 | `{[k]: 1, b: 2}`, with `k` 64 × `k` | 1: the entry `[k]` charges the size of its key, 1. The entry `b` charges 0 | 5: 2 entries, 1 + 2 × 2 |
| W33 | `{...m}`, with `m` a map of two entries whose keys are 40 × `a` and 40 × `b` | 3: 2 entries copied, plus the copied keys taken together, weight 80 and size 1 | 5: 1 + 2 × 2 |
| W34 | `const {"kk…k": v} = m; v`, a key of 64 letters `k`, with `m` a map that has that key | 2: the field 1 + size of its key, 1 + 1 | 0 |
| W35 | `const {a, ...rest} = m; rest`, with `m` a map with the key `a` and two keys 40 × `a` and 40 × `b` | 4: the field `a` 1 + 0. The rest copies 2 entries, 2, plus their keys taken together, weight 80 and size 1 | 5: the rest 1 + 2 × 2 |
| W36 | `m == n`, with `m` and `n` maps of two entries each, both with the keys 40 × `a` and 40 × `b` and the values 1 and 2 | 5: 2 entries at 1, plus the left map's keys taken together, size 1, plus 2 comparisons of two numbers at 1 | 0 |
| W37 | `xs.map(1)`, and `try { xs.map(1) } catch (err) { err.code }`, with `xs` `[1, 2]` and declared `any` | `xs.map(1)`: 0. Shape test 3 fails with `wrong-kind` before the argument is evaluated, and the call is not charged. The run fails with usage 0 steps. In the `try`: 2, the message `argument 1 of method map must be a function` has weight 43 and size 0, so 1 + 0, and `.code` 1 | `xs.map(1)`: 0. In the `try`: 10, the error value 9 + (1 + 0) |
| W38 | `match s { "b" => 1, "a…a" \| "a…a" => 2, _ => 3 }`, where the second arm's literals are 70 × `a` and 100 × `a`, with `s` 100 × `a` | 4: the first arm 1, and `"b"` has no common prefix with `s`, 0. The second arm 1. Its first literal has a common prefix of weight 70 and size 1, and is not equal, 1. Its second literal has a common prefix of weight 100 and size 1, and is equal, 1. So 1 + 0 + 1 + 1 + 1 | 0 |
| W39 | `[1, "a", 2].sort()` | 3: the call 1, then the ordered test fails at `"a"`, the second element tested, 2. The failure is `wrong-kind` | 4: the list 1 + 3, built before the call |

## 8. Limits

### 8.1 The limits

| Limit | Counts | Default | Code | Deterministic |
| --- | --- | --- | --- | --- |
| Steps | Steps charged (section 5) | 1,000,000 | `steps-exceeded` | Yes |
| Allocation | Units charged (section 6) | 1,000,000 | `allocation-exceeded` | Yes |
| Deadline | Wall-clock time of one run, and the host's cancellation | None | `deadline-exceeded`; `cancelled` when the host cancels | No |
| Depth | Calls in progress and the levels of a walk (8.4) | 200 | `depth-exceeded` | Yes |
| Source nesting | Levels open at once in the program text | 100, fixed | `nesting-exceeded`, phase `parse` | Yes |

1. How a host sets, turns off or keeps each limit is in `03-host.md`. The source
   nesting limit cannot be set. `01-language.md` defines the levels and the
   parse error.
2. Each code of the first four rows has the class `limit`. `catch` does not
   catch any of them (12.2).

### 8.2 Steps and units

1. Every charge is tested against its budget before it is added. A charge of `c`
   steps fails when the steps used plus `c` is more than the step limit. A
   charge of `c` units fails the same way against the allocation limit.
2. A failed step charge raises `steps-exceeded`, and a failed unit charge raises
   `allocation-exceeded`, at that charge. The work the charge was for is not
   done.
3. An operation charges its steps before its units. When both would fail, the
   code is `steps-exceeded`.
4. A charge made by a host function through its call handle is tested the same
   way, at the moment it is made.

### 8.3 Deadline

1. An implementation checks the deadline and the cancellation signal at least
   once in every 1,024 steps charged. The check reads a cancellation signal or
   the clock, whichever it can observe during a run.
2. A run whose deadline has passed at a check fails there with
   `deadline-exceeded`. A run the host has cancelled fails at a check with
   `cancelled`.
3. A run overruns its deadline by at most the work of 1,024 steps and the host
   function in progress. A host function that blocks is not interrupted.

### 8.4 Depth

1. The depth count is the sum of:
   - for each call in progress, 1 plus the nesting depth of the call within the
     body that makes it;
   - the number of levels of the walk in progress, if any.
2. The nesting depth of a call within its body is the number of levels open at
   the call's first token, less the number open at the first token of the lambda
   whose body holds the call. For a call in the program, nothing is subtracted.
   A method call's first token is the first token of its receiver.
3. For a call that a library entry makes of a lambda, the call that is measured
   is the library call. Each call of the lambda adds that measure while it is in
   progress. A library call that calls no lambda adds nothing.
4. A call is in progress from its depth test (item 7) to its end. The arguments
   of a call by name are evaluated before the call's step and its test, so the
   call is not in progress while they run. In `inc(inc(x))` the outer `inc` is
   not in progress while the inner one runs.
5. A walk adds 1 for each list or map it has entered and not left. A rendering
   walk enters every list and map it renders. A comparison enters a pair of
   lists of the same length, or of maps of the same size, as one level. An
   empty list or map, and an empty pair, is entered like any other, and the
   test of item 7 is made for it.
6. A projection does not exist in Core. A `try` adds nothing of its own: a call
   in its body or handler is deeper by the level of the braces.
7. The test is made at these points, and `depth-exceeded` is raised when the
   count would be more than the depth limit:
   - for a call by name, after the call's step and before the parameters are
     bound;
   - for a call a library entry makes of a lambda, after the two steps of the
     element (5.6) and before the parameters are bound;
   - for a walk, on entering each list or map, before the walk charges for it.
8. A failure that `catch` catches returns the depth count to its value when the
   `try` began.
9. Within these rules the count depends only on the text and on the values
   walked, so a run reaches the limit at the same call or the same level in
   every implementation.
10. In a program that the type rules accepted, the calls in progress never add
    more than 100 to the count: `05-types.md` 20.2 bounds the expanded nesting
    of every use of a `const`-bound lambda, and the parser bounds the rest
    (`01-language.md` 13.2). Such a program passes the default limit of 200
    only in a walk of data. A program compiled without a check has no such
    bound.

In this table `f` is a `const`-bound lambda.

| Program | Calls in progress at `f` | Count at `f` |
| --- | --- | --- |
| `xs.map(x => ys.filter(y => f(y)))` | 3 | 3: each call is at nesting depth 0 within its body |
| `xs.map(x => [(f(x))])` | 2 | 4: the call of the lambda by `map` is 1. `f` stands inside `map(`, `[` and `(`, 3 levels, less the 1 open at the lambda's first token `x`, so 1 + 2 = 3 |
| `try { xs.map(x => f(x)) } catch { 0 }` | 2 | 3: `xs` stands inside the braces of the body, so `map` is 1 + 1 = 2. `f` is 1 + 0 |
| `inc(inc(x))`, with `inc` a `const`-bound lambda, at the inner `inc` | 1: the outer `inc` is not in progress while its argument runs | 2: the inner `inc` stands inside the outer's `(`, 1 level, so 1 + 1. When it returns, the outer `inc` is tested at 1 + 0 = 1 |
| ``xs.map(x => `${v}`)``, with `v` `[[1]]` | 1, and a walk, at the inner list | 3: `map` is 1, and the walk has entered the outer list and the inner list, 2 |

### 8.5 What the limits guarantee

1. The work the language does in a run is at most a constant times the steps
   charged, with one stated exception: putting the keys of a map of `n`
   entries in order may take a constant times `n` × log2(`n`) comparisons of
   keys. This covers building a map and every place where the order of a host
   map's keys shows. No other operation has a logarithmic factor that its
   charge does not carry.
2. The memory a run creates as values of the language is at most a constant
   times the units charged plus a smaller constant times the steps. Each
   implementation publishes its constants.
3. An evaluator's recursion over the source, through every call in progress, is
   at most `D` + 100 levels, where `D` is the depth limit. A walk adds at most
   `D` more. A run in all is at most 2`D` + 100 levels.
4. The limits do not cover a host function beyond what it charges, host data,
   live memory or the fixed state of a run. They are not a security boundary.

### 8.6 Usage

1. Every run reports its usage: the steps charged and the units charged.
   `03-host.md` says how.
2. For a run that ends with a value, or with a failure that is not of class
   `limit`, the usage is fixed by this chapter.
3. For a run that ends with a failure of class `limit`, the usage is not fixed.
4. A charge made ahead for work that was not done (10.2) is given back before
   the usage is reported.

## 9. Determinism

1. The same program, the same variable values, the same results and charges of
   host functions, the same step limit, the same allocation limit and the same
   depth limit give the same outcome and the same usage in every implementation.
2. The same outcome is the same value, or a failure with the same code, span,
   message, path and data.
3. These may differ:
   - whether and where a run stops on the deadline or on cancellation;
   - the position at which a failure of class `limit` is reported, and the
     usage of a run that ends with one;
   - the class of a null, kind or field failure, which follows from the
     checker (`05-types.md`). A program cannot see it.
4. A run diagnostic may carry fields this chapter does not define.
5. To give this, an implementation must:
   - evaluate in the order of section 2 and report the first failure;
   - build each message from the catalog, `messages.json`, and from nothing
     else;
   - visit and render a map in key order by code point wherever the order can
     be seen;
   - give the number results of section 3 on every platform;
   - charge by sections 5 and 6;
   - use no clock, random source or shared state but the deadline check.

## 10. Optimizing

### 10.1 What must hold

An implementation may evaluate a program in any way that gives, against the
reference evaluation:

1. the same value, or the same failure as section 9 counts it;
2. the same usage at the end of the run, at every call of a host function, and
   whenever `catch` catches a failure, before the error value is charged;
3. a failure of class `limit` exactly when the reference has one, and never a
   call of a host function that the reference would not make.

### 10.2 Allowed

| An implementation may | Under this condition |
| --- | --- |
| Merge consecutive charges into one | Nothing between them can fail or call a host function |
| Fold a constant expression | The fold's steps and units are charged each time the expression is evaluated, where it stands |
| Fuse a chain of reads, or run a lambda in place | The charges are the same numbers, made where 10.1 can see them |
| Charge several operations at once and correct on failure | A failure part way gives back what was not done before anything can see the usage. A budget too small for the whole falls back to charging one operation at a time, so a limit failure comes exactly where the reference has one |
| Skip building an intermediate value | Its steps and units are charged as if it were built. The error value of a `catch (err)` whose handler does not read the name is one |
| Replace a scan by a lookup | The lookup charges what the scan would have charged. `x in [constants]` charges what the scan of 5.3 charges |
| Build a constant list or map once | Its units are charged at each evaluation (6.2) |
| Bind each call to its callee when compiling, keep each binding in one fixed place, and create nothing for a lambda | 11.1 |
| Resolve a method when compiling | The run still tests the receiver's kind and makes the shape tests of 2.3, so the method selected and every failure are those of the reference |
| Make no depth test at a call | The largest count the program's calls can reach is within the depth limit (11.1) |

### 10.3 Not allowed

1. Stopping early where the reference goes on. `xs.filter(f)[0]` must not become
   a search that stops at the first match, because the calls of `f` that the
   reference makes charge steps.
2. Skipping a `const` whose name is not used.
3. Charging a different number of steps or units for any form, in any case.

## 11. Calls

### 11.1 What the text fixes

A lambda is passed only to a library entry, and a name bound by a parameter or a
pattern cannot be called (`01-language.md`). So these hold for every program
that runs:

| Fact | Consequence an implementation may rely on |
| --- | --- |
| The callee of every call is known when the program is compiled: a library entry, a host function, or a `const`-bound lambda | A call can be bound to its callee before the run |
| A lambda runs only while every body around it is in progress. It cannot be returned, stored, or passed to a host function | A body may read an outer name from the place where the outer body wrote it |
| No lambda is entered while an earlier call of it is in progress | Each parameter, each `const` name and the name of each `catch (err)` may have one fixed place for the whole run |
| A program with `k` lambdas has at most `k` calls in progress | |
| The calls of a program form a graph with no cycle | The largest depth count its calls can reach is the longest path in that graph, each call weighted as in 8.4, and can be found before the run |

### 11.2 A call by name

1. The arguments are evaluated left to right.
2. The call charges its 1 step.
3. The depth limit is tested (8.4).
4. The parameters are bound left to right. A pattern charges by 5.7. An argument
   omitted or null takes the parameter's default.
5. The body is evaluated. Its value is the value of the call.

### 11.3 A call of a library entry

1. The receiver, if any, is evaluated and tested (2.3), then the arguments, left
   to right. A lambda argument charges 1 step where it stands.
2. The call charges its 1 step. Then the entry charges and works as
   `04-library.md` says, and calls its lambdas by 5.6 and 8.4.

### 11.4 A call of a host function

1. The arguments are evaluated left to right. For a method that an opaque type
   declares, the receiver is evaluated and tested first (2.3).
2. The call charges its 1 step, then the function runs (5.8).
3. A function that returns an error fails with `function-error`. A function that
   panics or throws fails with `function-panic`. `03-host.md` defines both.

## 12. Failures

### 12.1 Unwinding

1. A failure leaves every operation and every call in progress between the place
   that raised it and the innermost `try` whose body is in progress. When there
   is no such `try`, or when `catch` does not catch the code, it leaves the run.
2. Nothing is evaluated on the way.
3. Every step and unit charged before the failure stays charged.
4. Leaving is free. No step and no unit is charged for leaving a call or a
   `try`.
5. The depth count returns to its value when the `try` began. Nothing else is
   restored. A name bound in the body is not in scope in the handler.
6. A failure of the handler is the failure of the `try`. Its own `try` does not
   catch it. An outer `try` may.

### 12.2 What `catch` catches

1. `catch` decides by the code of the failure, and by nothing else.
2. It does not catch these codes. Each passes through every `try` in progress
   and ends the run:
   - a limit or a cancellation: `steps-exceeded`, `allocation-exceeded`,
     `deadline-exceeded`, `depth-exceeded`, `cancelled`;
   - a crash: `function-panic`, and a failure inside the implementation;
   - `missing-variable`.
3. It catches every other run code. The field `caught` of each run code in
   `messages.json` gives the same answer.

### 12.3 `catch { }`

A failure caught by `catch { }` charges no step and no unit. No error value and
no message is built.

### 12.4 The error value

`catch (err)` binds a map of four entries: `code`, `message`, `path`, `data`.
`01-language.md` defines the fields. It costs:

| Part | Steps | Units |
| --- | --- | --- |
| The map of four entries | 0 | 9 |
| `code` | 0 | 0. A code is text of the specification |
| `message` built from a template of the catalog | 1 + size of the message | 1 + size of the message |
| `message` of `failed` or `function-error` | 0 | 0. It is the string `fail` was given, or the host's text, which already exists |
| `path` | 0 | 0. In Core it is null for every failure |
| `data` | 0 | 0. It is the value `fail` was given, the same value and not a copy, or null |

1. **Charged as written.** The whole value is charged whether or not the handler
   reads the name.
2. **Charged at once, before it is built:** all its steps, then all its units,
   after the failure has reached the `try` and before the name is bound. If
   either charge fails, the run ends with that limit failure and the handler
   does not run.
3. **The size of a message** is the size, by section 4, of the text that the
   catalog's template gives with its parameters filled in. The catalog cuts each
   parameter's rendering, so the work of building a message is bounded.
4. **A message is built in the run only here.** The message of a failure that no
   `catch (err)` receives is built for the host's diagnostic, outside the usage.

| Program | Steps | Units |
| --- | --- | --- |
| `try { 1 / d } catch { 0 }`, with `d` 0 | 1: the `/` | 0 |
| `try { 1 / d } catch (err) { 0 }`, with `d` 0 | 2: the `/`, and 1 + 0 for the message | 10: 9 for the map, 1 + 0 for the message |
| `try { fail("no stock", {sku}) } catch (err) { err.data }`, with `sku` `"a"` | 2: the call, and `.data` | 12: 3 for `{sku}`, 9 for the map |

The message counts assume a message of size 0.

### 12.5 `fail`

`fail(message, data?)` charges the 1 step of a library call and no unit. It
raises the code `failed`. Its message is the first argument, whole, and its data
the second, or null.

## 13. What packages add

A package of the specification adds rows to the tables of 5.2 and 6.1 for its
own forms and changes no row above. Its chapter is the home of those rows.

| Package | Adds | Chapter |
| --- | --- | --- |
| F1, host types | No row. A program's counts do not depend on how its variables were declared | `spec/packages/f01-host-types.md` 8 |
| F2, templates and analysis | No row. A host template is evaluated as the template of this chapter, in one run with one budget | `spec/packages/f02-templates-analysis.md` 2 |
| F3, string forms | A raw string and a text block with no interpolation: 0 steps, 0 units. A text block with an interpolation: as a template | `spec/packages/f03-strings.md` 7 |
| F6, opt-in groups | The cost of each entry of the four groups, in the form of `04-library.md`. Two formula symbols, `c` and `u`, for `json.parse` | `spec/packages/f06-groups.md` 5 |
| F13, math functions | Each entry: 0 steps and 0 units beyond the call | `spec/packages/f13-math.md` |
| F4, checked conversion | The conformance walk of `as` and `is`: its steps for each value visited and each key, its place in the depth count, the units of an error value's path | `spec/packages/f04-conversion.md` 4 and 5 |
| F7, queries | The cost of a projection: a step for each element visited, and the units of its list. The cost of its seven entries | `spec/packages/f07-queries.md` 3 and 6 |
