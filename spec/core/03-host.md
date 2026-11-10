# 3. The host

This chapter specifies what every host binding of Risor offers in Core, and
the case-file format that the conformance suite and every runner use. A
binding is the API of one implementation in its own language. This chapter
names no type or function of any binding. Each binding chooses its own
spellings for the operations here.

Other chapters: `01-language.md` (syntax, values and their rules),
`02-evaluation.md` (evaluation, costs and the meaning of each limit),
`04-library.md` (the library and `library.json`), `05-types.md` (types, type
text and the checker).

## 3.1 Terms

| Term | Meaning |
| --- | --- |
| Host | The program that embeds an implementation and runs Risor programs |
| Binding | The API an implementation gives a host |
| Environment | A set of declarations that a program is checked, compiled and run against (3.3) |
| Host value | A value that the host supplies: a variable's value, or a host function's result |
| Diagnostic | One problem found by a stage, in the one shape of 3.7 |
| Case file | A JSON file of test cases in the format of 3.10 |
| Runner | The part of a binding that runs a case file |

## 3.2 Stages

### 3.2.1 The four stages

| Stage | Takes | Gives | Fails with a list of diagnostics of phase |
| --- | --- | --- | --- |
| Parse | The source text of a program | A parsed program | `parse` |
| Check | A parsed program, an environment, and an expected result (3.2.3) | The result type, the sites, and the diagnostics found | `check` |
| Compile | A parsed program, an environment, and an expected result | A runnable program | `check`, or `declare` for `no-implementation` |
| Run | A runnable program, a supply of variables (3.4.1), and a cancellation signal | A value, or exactly one diagnostic; and the usage (3.6.3) in both cases | `run` |

1. Parse needs no environment. A text gives the same parsed program, or the
   same `parse` diagnostics, in every environment.
2. Check is specified by `05-types.md`. Check gives a result type, a list of
   sites, and a list of diagnostics. The program checks when the list of
   diagnostics is empty.
3. Compile with a check fails exactly when that check fails, with the same
   diagnostics.
4. A program may be compiled without a check. "Without a check" means without
   the type rules, never without the static rules. Every implementation, with
   or without a type checker, applies the static rules of `01-language.md`
   when it compiles, and reports each in phase `check`, as `05-types.md` says
   in its section on the static rules alone. So a name that is read and that
   nothing binds or declares is `undefined-name` in every implementation.
5. A run of a program compiled without a check reports classes by the rule for
   an unchecked program (3.7.6).
6. Compile fails with the `declare` code `no-implementation` when the
   environment declares a function or a method that has no implementation.
7. Compile does not call a host function, and evaluates nothing that can call
   one.
8. A run's limits come from the environment of the compile (3.6).

### 3.2.2 Sharing

1. An environment, a parsed program and a runnable program do not change
   after they are built.
2. Each may be used by several runs at the same time. The state of a run
   belongs to that run only.

### 3.2.3 The expected result

Check and compile take an expected result. It is one of three things.

| Expected result | Accepted results | Reported |
| --- | --- | --- |
| Nothing | A result of any type | Nothing |
| A type, in type text | A result whose type fits it, by the rules of `05-types.md` | `result-mismatch`, at each place of failure that `05-types.md` gives |
| `condition` | A result of any type | `always-truthy` or `always-falsy` at the result expression, as `05-types.md` gives, while the check class `constant-condition` is rejected |

1. The expected result is an argument of each check and each compile. It is
   not a declaration of the environment.
2. The expected result does not change the run. The compiled program does not
   keep it, and the run does not convert its value.
3. `condition` is not a type. In data it is the word `condition` in the place
   of a type text.
4. An expected type whose text the grammar of `05-types.md` does not accept is
   the error `bad-type`, of phase `declare` (`05-types.md`).

### 3.2.4 Two functions on a result

Every binding offers these two functions. Neither is a run. Neither charges
steps or units.

| Function | Takes | Gives |
| --- | --- | --- |
| Truthiness test | Any value a run can return, including one that shares memory with host data | `false` for null, `false`, zero, the empty string, an empty list and an empty map. `true` for every other value, an opaque value included. It cannot fail |
| JSON text | A value a run returned | The JSON form of `01-language.md`: keys in code point order, numbers in the rendering of `01-language.md`. It fails with `wrong-kind` and no span on an opaque value. It fails with `depth-exceeded` and no span on nesting deeper than the default depth limit, 200 |

A host does not use its own language's truthiness in place of the truthiness
test, and does not write its own list of falsy values.

## 3.3 The environment

### 3.3.1 Declarations

An environment is a set of declarations. Core has these kinds.

| Declaration | States | Portable form |
| --- | --- | --- |
| Version | The specification version the environment wants (3.9) | `major.minor` as text |
| Variable | A name and a type | The type in type text (`05-types.md`) |
| Function | A name, a signature, an implementation, and documentation | The signature in type text. The implementation is host code. The documentation is not normative |
| Opaque type | A name for a kind of host value that has behaviour, and its methods | The name. Each method is a name and a signature in type text, with the opaque type as its receiver |
| Removed entry | A library entry that the environment does not have | The entry's name, as `04-library.md` writes it |
| Rejected classes | The check classes whose findings are errors (`05-types.md`) | Class names: the classes added to the default set, and the classes taken out of it |
| Limits | The limits of 3.6 | Numbers |

1. An environment with no version declaration has the newest version the
   implementation offers.
2. Every implementation parses the type text of every declaration, by the
   grammar of `05-types.md`, whether or not it has a type checker. It needs the
   names of the variables and the count of parameters of each function. Text
   that the grammar does not accept is `bad-type`, or `bad-signature` for a
   signature, in every implementation. The rules of `05-types.md` beyond the
   grammar (two record alternatives, an undeclared type name, depth, a key
   written twice) need the type checker.
3. Check uses the types of variables. A run uses only their names.
4. An environment with no rejected-classes declaration rejects the default
   set, which is `constant-condition` alone. A name that is not one of the six
   check classes of `05-types.md` is the `declare` error `bad-class`.
5. An environment with no limits declaration has the default limits of 3.6.1.
6. A removed entry is absent from the environment: a program cannot call it,
   and a host function may then take its name.
7. A binding may derive a signature from its own language's types. The
   signature is still type text in the portable form.

### 3.3.2 Child environments

A child environment adds declarations to a parent environment and does not
change the parent.

1. A child that declares a name its parent declares replaces the parent's
   declaration, in the child only.
2. A child's rejected-classes declaration and limits declaration apply in the
   child only.

### 3.3.3 Names

1. A variable name, a function name and a namespace are each a `Name` of
   `01-language.md`.
2. A function name may have one namespace prefix, as in `geo.distance`. The
   namespace is not a value: a program that reads `geo` alone fails to check.
3. The namespaces `json`, `time`, `math` and `base64` are reserved for the
   library. A host function may not be declared in them, whatever the
   environment enables.
4. An opaque type name is an upper-case letter followed by one or more of
   `A` to `Z`, `a` to `z`, `0` to `9` and `_`.
5. A lambda parameter, a `const` name or a `catch` name equal to a name of the
   environment is not a clash: it shadows (`01-language.md`).
6. A field of a variable's type may have any name, a reserved word included.

### 3.3.4 Declaration errors

The environment is built when its declarations are given. Errors are reported
then, with phase `declare`, except `no-implementation`, which compile reports.

| Situation | Code |
| --- | --- |
| Two declarations of one name in one environment: two variables, two functions, or a variable and a function | `name-clash` |
| A variable named like a namespace that a function of the environment uses | `name-clash` |
| A host function named like a library entry that the environment has | `name-clash` |
| Two opaque types with one name in one environment | `name-clash` |
| A host function in the namespace `json`, `time`, `math` or `base64` | `reserved-namespace` |
| A variable, a function or a namespace named with a reserved word of `01-language.md` | `bad-name` |
| An opaque type name not of the form of 3.3.3, item 4. One upper-case letter alone is a type variable of a signature | `bad-name` |
| A signature that is malformed, that has a parameter of a function type, or a method whose receiver is not a declared opaque type | `bad-signature` |
| A type text that is malformed, that nests past the 64 levels of `05-types.md`, that names a type no declaration gives, or that `05-types.md` otherwise refuses in a declaration | `bad-type` |

Which of these an implementation with no type checker reports is 3.3.1, item
2. `05-types.md` gives the full list of errors in declared type text.
| A rejected class that is not a check class | `bad-class` |
| A version the implementation does not offer (3.9) | `unsupported-version` |
| A compile against an environment that has a function or a method with no implementation | `no-implementation` |

1. A type name and a variable, function or namespace name never clash. A type
   name is read only where a type is written.
2. A `declare` diagnostic has a code, a phase and a message. It has no span.

## 3.4 Variables and values

### 3.4.1 Supplying variables

1. The host supplies variables to each run, by name.
2. A run asks only for the names that the program reads on the path it takes.
   It asks for each name at most once. So a host may compute a value only when
   it is asked for.
3. A declared variable that is read and that the host does not supply fails
   the run with `missing-variable`, at the span of the read. `catch` does not
   catch it.

### 3.4.2 Host values are read in place

1. A run reads a host value where it is. Nothing is copied or converted when a
   run starts or when a value is supplied.
2. A host value is not charged to the allocation budget.
3. A run never changes a host value.
4. A result may share memory with host values. The host treats a result as
   read-only. The `data` of a `failed` diagnostic leaves a run in the same way.

### 3.4.3 Which host values enter the language

| Kind of value | A host may supply | Rule |
| --- | --- | --- |
| null | Its own value for nothing | A field that is absent is not null. Each binding says which host values count as absent |
| bool | Its boolean | None |
| number | Any of its number types | A whole number outside ±(2^53 − 1), a NaN and an infinity cannot enter (3.4.4) |
| string | Its text | Length and positions are in code points. Text that is not valid enters as 3.4.5 says |
| list | An ordered sequence | Read through, not copied |
| map | A mapping with string keys, or an object with named fields | Keys are visited in code point order (`01-language.md`). The binding must be able to report, for the value, its kind, its size, a field by name, and its keys |
| opaque | Any other host value | 3.4.6 |
| function | Never | No function value crosses the boundary, in either direction |

1. A binding must tell an empty host list or map from a non-empty one in
   bounded work.
2. A host value is presented as a map, read by its fields, or as an opaque
   value, used by its methods. No value is both.

### 3.4.4 Numbers that cannot enter

1. A host number that is a whole number outside ±(2^53 − 1), a NaN or an
   infinity cannot enter the language.
2. A run fails where it reads such a number, with the code `number-range` and
   the class `environment`. A value is read lazily, so a number that is only
   passed through is still read, and still fails.
3. `catch` catches this failure.
4. A host passes an identifier that does not fit as a string.

### 3.4.5 Text that is not valid

Host text that is not a valid sequence of Unicode scalar values reads as if
each malformed unit were U+FFFD. A malformed unit is one byte of UTF-8 or one
unit of UTF-16. An unpaired surrogate is one malformed unit.

### 3.4.6 Opaque values

1. An opaque value can be bound to a name, passed to a host function, put in a
   list or map, compared with `==`, tested, and used through the methods its
   opaque type declares. Nothing else.
2. An opaque value is always truthy.
3. The binding must be able to tell, for any host value, whether it is opaque
   and under which declared name. How it tells is the binding's matter.
4. **The identity test.** The binding must be able to tell whether two opaque
   values are the same host value. Two opaque values are `==` exactly when this
   test says they are the same (`01-language.md`). The test cannot fail and
   charges nothing beyond what `==` charges (`02-evaluation.md`).
5. Opaque types and their declared methods are in Core.

### 3.4.7 Nesting past the limit

Host data may be nested without bound, and may be cyclic. Every walk over a
value counts its nesting against the depth limit (`02-evaluation.md`): `==`,
rendering and encoding among them. A walk that passes the limit fails the run
with `depth-exceeded`. No walk over host data loops for ever.

### 3.4.8 A value that breaks its declared type

1. A declared type is a promise the run does not trust.
2. Where a run reads a value that does not fit its declared type, the run
   fails with the code of what it found: `null-operand`, `wrong-kind`,
   `missing-field`, `number-range` or `no-match`. It never crashes and never
   gives a wrong value.
3. The run finds such a value only where it reads it.
4. The class of the failure is given by 3.7.6.

## 3.5 Host functions

### 3.5.1 Declaration

1. A host function is declared with a name, a signature in type text, and an
   implementation. Example signature: `(s: string, n?: int) => string !`.
2. `!` after the result type marks a function that can fail.
3. A parameter may not have a function type. Such a signature is
   `bad-signature`. A lambda is passed only to a library entry.
4. A method of an opaque type is a host function whose receiver is a value of
   that type. Every rule of this section applies to it.
5. A binding may let a function carry host code that computes its result type
   at each call site. Its portable form is its declared signature only.

### 3.5.2 The call

1. A call of a host function is synchronous. The function returns before the
   run continues.
2. The function receives the argument values and a call handle.
3. The call handle gives three things: the run's cancellation signal;
   `charge(steps)`, which charges steps to the run; and `allocate(units)`,
   which charges allocation units to the run. A charge that the budget cannot
   cover ends the run with `steps-exceeded` or `allocation-exceeded`.
4. A call costs the one step of any call, plus what the function charges
   (`02-evaluation.md`).
5. A host function is never called at compile time, and its result is never
   cached, even when every argument is a literal.

### 3.5.3 What a host function returns

| The function | The run sees | Class | `catch` |
| --- | --- | --- | --- |
| Returns a value | The value, as a host value (3.4) | — | — |
| Returns an error | `function-error`, at the span of the call. The message is the text of the host's error, as the binding gives it. The host's error is the diagnostic's `cause` | `host` | Catches |
| Crashes: a panic, or a thrown value that is not an error | `function-panic`, at the span of the call | `host` | Passes |

1. A value that a host function returns is read as any host value is. A
   result that does not fit the declared result type fails where it is read,
   as 3.4.8 says.
2. A host function's error carries no `data`.
3. What counts as an error and what counts as a crash is the binding's matter.

### 3.5.4 Duties of a host function

| Duty | Effect when it is not met |
| --- | --- |
| Treat the arguments as read-only, and do not keep them after the call returns | Values shared between runs may change. Nothing checks this duty |
| Call `charge` in proportion to the work done: one step for each element visited, and text at the rate of `02-evaluation.md` | The step budget does not bound the run |
| Call `allocate` before building a result whose size depends on the input | The allocation budget does not bound the run |
| Stop when the cancellation signal says so, if the function can wait | The run cannot be interrupted while the function waits |

A host function that charges nothing costs the one step of its call. The
limits cover a host function exactly as far as it charges.

## 3.6 Limits and usage

### 3.6.1 The limits

`02-evaluation.md` gives the meaning of each limit and how it is counted. A
host sets the limits as a declaration of the environment.

| Limit | Default | The host may | Code when passed |
| --- | --- | --- | --- |
| Steps | 1,000,000 | Set a whole number, or turn it off | `steps-exceeded` |
| Allocation | 1,000,000 units | Set a whole number, or turn it off | `allocation-exceeded` |
| Deadline | Off | Give it as a duration of wall-clock time, measured from the start of the run, or turn it off | `deadline-exceeded` |
| Depth | 200 | Set a whole number from 1 to 1,000. It cannot be turned off | `depth-exceeded` |
| Source nesting | 100 levels | Nothing. It is fixed | `nesting-exceeded`, of phase `parse` |

1. A host cancels a run through the cancellation signal it passed to the run.
   A cancelled run fails with `cancelled`.
2. The five codes `steps-exceeded`, `allocation-exceeded`, `deadline-exceeded`,
   `depth-exceeded` and `cancelled` have the class `limit`. `catch` catches
   none of them.
3. The run tests the deadline and the cancellation signal at the interval
   of steps that `02-evaluation.md` gives.
4. The position of a limit error is not specified. Its span is not compared
   by a case.

### 3.6.2 Determinism

Steps, allocation and depth are deterministic. The deadline and cancellation
are not.

### 3.6.3 Usage

1. Every run reports its usage: the steps it charged and the allocation units
   it charged. A run reports its usage whether it succeeds or fails.
2. For a run that succeeds, and for a run that fails with a code that is not
   a limit code, the usage is exact: the same in every implementation and in
   every version of the specification.
3. For a run that fails with one of the five limit codes, the usage is not
   fixed. An implementation may merge charges (`02-evaluation.md`), so it may
   stop at a different point inside the operation in progress.

## 3.7 Diagnostics

### 3.7.1 The one shape

Every stage reports a failure as a list of diagnostics. Every diagnostic of
the list has the same phase. A run fails with exactly one diagnostic.

| Field | Holds | Present | Specified |
| --- | --- | --- | --- |
| `phase` | `declare`, `parse`, `check` or `run` | Always | Yes |
| `code` | The kind of problem, from 3.7.3 | Always | Yes |
| `class` | For a `run` diagnostic: its class (3.7.6). For a `check` diagnostic that exists because its check class is rejected: that class | When the rule gives one | Yes |
| `span` | Where the problem is in the source text (3.7.7) | For `parse`, `check` and `run`, except where 3.2.4 says otherwise | Yes. For a limit code it is present and not specified |
| `message` | One sentence (3.8) | Always | Yes |
| `path` | For `not-conforming`: the place of the failure in the tested value (3.7.8) | For `not-conforming` only | Yes |
| `data` | For `failed`: the second argument of `fail`, or null when it had none | For `failed` only | Yes |
| `help` | What to write instead | Optional | No |
| `expected`, `found` | Two types in type text | Optional | No |
| `fix` | A span and its replacement text | Optional | No |
| `cause` | The host's own error, for `function-error` | Optional | No |
| An offset | A position in a unit of the implementation's choice | Optional | No |

1. A diagnostic may carry further fields that a later package of the
   specification defines. A binding, a runner and a host ignore a field they
   do not know.
2. There is no severity. Every diagnostic is an error.
3. A program never sees a diagnostic. It sees the error value of
   `02-evaluation.md`, which has `code`, `message`, `path` and `data`, and no
   class and no span.

### 3.7.2 Phases

| Phase | Reported by | Diagnostics in one failure |
| --- | --- | --- |
| `declare` | Building an environment; compile, for `no-implementation` | One or more. Their order is not specified |
| `parse` | Parse | One or more. The first is specified: its code and its span. The rest are not specified, and a parser may stop after the first |
| `check` | Check, and compile with or without a check (3.2.1) | One or more, in the order of `05-types.md`. The whole list is specified: each code and span |
| `run` | Run | Exactly one. Its code, class, span, message, path and data are specified |

In Core the message of a `declare`, `parse` or `check` diagnostic is not
specified. Its code and span are. A later package adds the templates of those
phases to the catalog (3.11).

### 3.7.3 The codes of Core

A code is never renamed or reused within a major version. Core has these
codes.

| Phase | Number | Codes |
| --- | ---: | --- |
| `declare` | 8 | `name-clash`, `reserved-namespace`, `bad-name`, `bad-signature`, `bad-type`, `bad-class`, `unsupported-version`, `no-implementation` |
| `parse` | 14 | `unexpected-token`, `unexpected-character`, `unterminated-string`, `unterminated-comment`, `bad-escape`, `line-continuation`, `bad-number`, `literal-range`, `text-block-open`, `text-block-close`, `text-block-indent`, `nesting-exceeded`, `bad-type-text`, `reserved-word` |
| `check` | — | The codes of `05-types.md` |
| `run` | 19 | 3.7.4 |

1. `01-language.md` says which text gives each `parse` code, and its span.
2. Text that a later package gives a targeted `parse` code is reported in
   Core with the general code that fits, `unexpected-token` among them, at the
   span the targeted code will have (`01-language.md`).

### 3.7.4 The run codes

| Code | Raised when | Class | `catch` |
| --- | --- | --- | --- |
| `steps-exceeded` | A charge passes the step limit | `limit` | Passes |
| `allocation-exceeded` | A charge passes the allocation limit | `limit` | Passes |
| `depth-exceeded` | A call or a walk passes the depth limit | `limit` | Passes |
| `deadline-exceeded` | The run passes its deadline | `limit` | Passes |
| `cancelled` | The host cancels the run | `limit` | Passes |
| `missing-variable` | A declared variable is read, and the host did not supply it (3.4.1) | `environment` | Passes |
| `function-panic` | A host function crashes (3.5.3) | `host` | Passes |
| `internal-error` | The implementation itself fails | None | Passes |
| `function-error` | A host function returns an error (3.5.3) | `host` | Catches |
| `number-range` | A result of arithmetic or of a library entry is out of range or not finite (`01-language.md`); or a host number that cannot enter the language is read (3.4.4) | `range` for arithmetic and library results. `environment` for a host number | Catches |
| `not-conforming` | A value does not conform to a type it is tested against | 3.7.6 | Catches |
| `invalid-argument` | A library entry is given a value it cannot use (`04-library.md`) | `value` | Catches |
| `failed` | The program calls `fail` | `value` | Catches |
| `null-operand` | A null stands where an operation needs a value (`01-language.md`) | 3.7.6 | Catches |
| `wrong-kind` | A value of the wrong kind stands where an operation needs another kind (`01-language.md`) | 3.7.6 | Catches |
| `missing-field` | A read finds no field or key (`01-language.md`) | 3.7.6 | Catches |
| `index-range` | A plain index is outside its list or string (`01-language.md`) | `index` | Catches |
| `divide-by-zero` | A divisor is zero (`01-language.md`) | `divide` | Catches |
| `no-match` | A `match` finds no arm (`01-language.md`) | 3.7.6 | Catches |

1. `catch` decides by the code alone. The last column is that rule. The
   catalog carries it as the field `caught` (3.8).
2. Eight codes pass through `catch`: the five limit codes, `missing-variable`,
   `function-panic` and `internal-error`. The other eleven are caught.
3. A run code is never also a `check` code.
4. No code is added to this list in a later minor version. A later package may
   raise a code of this list at a new place.

### 3.7.5 Codes that no template words

The message of `failed` is the first argument of `fail`, whole and not cut.
The message of `function-error` is the text of the host's error, as the
binding gives it, whole and not cut. Every other code has its message from the
catalog (3.8).

### 3.7.6 The class of a run diagnostic

The class depends on whether the program was checked.

| Failure | Class in a checked program | Class in a program compiled without a check |
| --- | --- | --- |
| `null-operand`, `wrong-kind`, `missing-field`, at a `dynamic` site (`05-types.md`) | `dynamic` | `dynamic` |
| `null-operand`, `wrong-kind`, `missing-field`, anywhere else | `environment` | `dynamic` |
| `no-match` | `environment` | `dynamic` |
| `not-conforming`, at a `value` site | `value` | `value` |
| `not-conforming`, anywhere else | `environment` | `value` |
| `number-range`, from a host number | `environment` | `environment` |
| `number-range`, from arithmetic or a library result | `range` | `range` |
| `missing-variable` | `environment` | `environment` |
| Every other code | As 3.7.4 gives | As 3.7.4 gives |

1. `internal-error` has no class. The field is absent.
2. In a checked program, the class `environment` means that a host value broke
   its declared type, or that the host did not supply a declared variable.
3. A program cannot see the class. A checked run and an unchecked run of one
   program on the same values give the same value, the same usage, and on
   failure the same code, span and message.

### 3.7.7 Positions and spans

1. A position is a line and a column. Both count from 1.
2. A line ends at U+000A and at nothing else. U+000D is an ordinary
   character.
3. A column counts code points. A tab is one column. A code point above
   U+FFFF is one column.
4. A span is a start position and an end position in the source text. The end
   is exclusive: it is the position just after the last code point of the
   span.
5. A span is written as four numbers: start line, start column, end line, end
   column. Example: `[1, 7, 1, 11]`.
6. Which node or token a span covers is given by `01-language.md` for `parse`
   and `run` codes, and by `05-types.md` for `check` codes.

### 3.7.8 Paths

1. A path is a list of steps from the tested value to the place of a failure.
2. A step is a key, as a string, or a list index, as a whole number from 0.
3. An empty path means that the tested value itself failed.
4. A path has at most 64 steps.

## 3.8 Messages

### 3.8.1 The catalog

1. A message is built from the catalog, `suite/messages.json`, which is part
   of this specification. No implementation writes a message by hand. Each
   generates its table from the file.
2. In Core the catalog has the 19 codes of phase `run`, with 64 templates in
   all. Five templates of `wrong-kind` are the shape test of a method call
   whose method is found only at run time, in this order: `method`, `count`,
   `callback`, `not-callback` and `parameters` (`04-library.md`). In
   `callback` and `not-callback`, `{count}` is the argument's position,
   counted from 1 and not counting the receiver. In `parameters`, it is the
   number of arguments the entry calls the function with. Entries for the other
   phases come with a later package. Each entry has a `phase`, so those entries
   are added without changing the run entries.
3. A run message is a value a program can read and is charged for. It is the
   same in every implementation.
4. A message is one sentence with no full stop at its end.

### 3.8.2 The file

The file is a JSON array. Each element is an entry for one code.

| Field | Holds | Present |
| --- | --- | --- |
| `code` | The code | Always |
| `phase` | `declare`, `parse`, `check` or `run` | Always |
| `since` | The specification version that added the code, as `"3.0"` | Always |
| `caught` | `true` when `catch` catches the code | For a `run` code only |
| `params` | An object that maps each parameter name to its kind (3.8.3) | Always. `{}` when there is none |
| `templates` | An object that maps a template name to its template text | Always |
| `when` | An object that maps each template name to the condition that selects it, in words | When there are two templates or more |
| `cases` | A list of cases in the format of 3.10, with no header | Optional |

1. One code has one entry. Two entries never have the same code.
2. An entry with one template names it `default`.
3. An entry for `failed` or `function-error` has `templates` equal to `{}`
   (3.7.5).
4. A template's name is fixed with its text.
5. Each `fails` line of a library entry in `library.json` names its code, and
   names its template when that code's entry has two templates or more
   (`04-library.md`). A `number-range` failure of an entry names `entry`. An
   `invalid-argument` failure names one of the 14 forms. A `wrong-kind` or
   `missing-field` failure of an entry's own test names the template whose
   condition fits it: `argument`, `whole`, `unordered`, `element`, `result`,
   `key`, `text` or `field`.
6. A template is text with holes. A hole is `{name}`, where `name` is a key of
   `params`. No other brace stands in a template.
7. A parameter is used in a template only when the template's condition gives
   it a value.

### 3.8.3 Kinds of parameter

| Kind | Rendering |
| --- | --- |
| `word` | A name, a field word or an operator of the program or of the environment, as written |
| `string` | A string, in its JSON form (`01-language.md`), with the quotes |
| `value` | A null, a bool, a number or a string in its JSON form. For any other value, the word `list`, `map` or `opaque` |
| `kind` | What `type(x)` gives for the value (`04-library.md`): `null`, `bool`, `number`, `string`, `list`, `map` or `opaque` |
| `type` | Type text, printed as `05-types.md` prints it |
| `number` | The rendering of a number in `01-language.md` |
| `token` | In the `parse` phase: the token at the span as written, or `end of input` |

1. A rendering longer than 64 code points is cut to its first 61 code points,
   followed by `...`.
2. A parameter is taken from a value, or from the text of the program or of
   the environment. It is never taken from an inferred type.
3. A message does not hold a path, a span, or a suggestion found by search.

### 3.8.4 Stability

1. A `run` message does not change within a major version.
2. A message of another phase may change in a minor version.

## 3.9 Safety and versions

### 3.9.1 The safety contract

A program can read the variables its environment declares, call the functions
and methods its environment declares, and build new values. It can do nothing
else.

| Property | Guaranteed | Not guaranteed |
| --- | --- | --- |
| No I/O | The language and its library read no file, network, clock, environment or random source | What a host function does |
| No mutation | The run never writes to a value it did not create | That a host function does not change its arguments. A result may share memory with host data |
| Determinism | The same program, version, values, host function results and limits give the same value, or the same failure with its code, span and message, and the same usage | A run stopped by its deadline or by cancellation. The span of a limit error |
| Steps | The work of the language and its library is proportional to the steps charged. The run stops at the same point in every implementation and version | The time one step takes. Work in a host function beyond what it charges |
| Allocation | The units of every value the language and its library create in a run never pass the budget. The charge comes before the memory is taken | Live memory. Host data. What a host function builds without charging |
| Deadline | A run that passes its deadline, or that is cancelled, stops within one test interval of steps | That a blocked host function returns |
| Depth | The implementation's own stack does not overflow on any program or any data | — |
| No crash | A run does not crash or hang on any program or any values. A crash in host code is `function-panic`. A failure inside the implementation is `internal-error` | — |

The limits make accidents cheap. They are not a sandbox for hostile code.

### 3.9.2 Versions

1. A version is one number, `major.minor`, for the grammar, the semantics,
   the cost table, the library and the run messages. Core is version `3.0`.
2. A minor version only adds. It never changes the result or the usage of a
   program that was valid before it.
3. An implementation offers every minor version of its major version, up to
   its newest.
4. An environment states the version it wants (3.3.1). An implementation that
   does not offer it refuses with `unsupported-version`. It never substitutes
   another version.
5. In an environment of an earlier version, the library entries and the
   syntax that a later version added do not exist.

## 3.10 Case files

A case file holds test cases for programs. The conformance suite is a set of
case files, and every implementation runs it with its runner. This section is
the one contract for all runners.

### 3.10.1 The file

1. A case file is JSON text in UTF-8.
2. The file is one JSON object, which is a header and the field `cases`, a list
   of cases. Or the file is one JSON array, which is a list of cases with no
   header.
3. The `cases` of a catalog entry in `messages.json`, and of an entry in
   `library.json`, are a list of cases with no header.

### 3.10.2 The header

1. The header is every field of the file object except `cases`.
2. The header may hold any field of a case except `name`, `skip` and `expect`.
3. A case is the header with the case's own fields laid over it. A field of
   the case replaces the header's field of the same name, whole. Nothing is
   merged.
4. `source` and `file` count as one field for this rule. A case that gives
   either replaces the header's `source` or `file`.
### 3.10.3 The fields of a case

| Field | Holds | When absent |
| --- | --- | --- |
| `name` | The case's name, a string. Unique in the file | The file is invalid |
| `source` | The program text | Exactly one of `source` and `file` is present after the header rule |
| `file` | A path to a file that holds the program text, relative to the directory of the case file, with `/` between its parts. Spans are positions in that text | |
| `version` | The specification version, as `"3.0"` | The base environment's |
| `env` | One record type in type text. Each field is a declared variable, with its type | No variable is declared |
| `opaque` | A list of opaque type names | No opaque type is declared |
| `methods` | An object that maps `Type.method` to a method signature in type text | No method is declared |
| `functions` | Host functions, as stubs (3.10.4) | No function is declared |
| `reject` | The whole set of rejected check classes, as a list of names | The base environment's set |
| `returns` | The expected result (3.2.3): a type text, or the word `condition` | Nothing is expected |
| `vars` | An object: the value of each variable the case supplies | No variable is supplied |
| `limits` | An object with any of `steps`, `allocation` and `depth`, each a whole number. Each one given replaces the base environment's limit | The base environment's limits |
| `precondition` | `false`, or `true`. `false` says that the case supplies, on purpose, a value that breaks its declared type: in `vars`, or as a stub's `result` | `true` |
| `skip` | A reason, as a string. The case is not run | The case is run |
| `expect` | What is expected (3.10.6) | The file is invalid |

1. A field that this table does not have makes the file invalid.
2. A case cannot give a deadline.
3. The case's environment is a child of the base environment (3.10.9). Its
   declarations are those of `version`, `env`, `opaque`, `methods`,
   `functions`, `reject` and `limits`.
4. A runner supplies each value of `vars` as a host value: a JSON object as a
   map, an array as a list, and a string, a number, `true`, `false` and `null`
   as the language value of that kind. A key of `vars` whose value is `null`
   supplies null. A field of `env` that `vars` does not name is not supplied.
5. Every JSON number in a case file is read by the rule of text to number in
   `01-language.md`. A number that reads as an infinity makes the file invalid.
   So a case cannot hold a host number that cannot enter the language.
6. A Core runner accepts `precondition` and does nothing with it. It does not
   test the values of `vars`, or the results of stubs, against their declared
   types. A later package adds that test, which `"precondition": false` turns
   off (3.11). So a case that breaks a declared type on purpose carries
   `"precondition": false` from the start.

### 3.10.4 Stubs

A stub is a host function given as a table of rows. The field `functions` maps
a function name to a stub.

| Matter | Rule |
| --- | --- |
| Form | A signature text alone, which is a stub with no rows. Or an object `{"type": signature, "calls": [row, ...]}`. `type` may be left out when the base environment declares the name. `calls` may be left out, which gives no rows |
| A row | An object. `args`: a list of JSON values, optional. Exactly one of `result`, a JSON value, and `error`, a string. Optional: `charge` and `allocate`, two whole numbers of 0 or more |
| Matching | At each call the rows are tried in order. A row with no `args` matches every call. A row with `args` matches when the call has that many arguments and each argument is `==` to the row's value (`01-language.md`). The first row that matches answers |
| Answer | The stub passes the row's `charge` to `charge(steps)`, then the row's `allocate` to `allocate(units)`, then answers. With `result`, it returns the value, supplied as a host value as `vars` are. With `error`, it returns an error whose text is the string |
| No row matches | The stub returns an error. The case's verdict is `fail` with the difference `functions`, whatever the run then gives. A `try` in the program does not change this |
| Where it is declared | In the case's environment. A name the base environment declares is replaced in the child only. With no `type`, the base's signature is kept and only the implementation is replaced |
| Cost | Matching is the runner's work and charges nothing. A stubbed call costs the one step of any call, plus its row's `charge` steps, plus its row's `allocate` units. So the usage of a case does not depend on the implementation |

1. A stub with `error` gives the run `function-error`, class `host`. Its
   message is the string of `error`.
2. A case cannot describe a function that crashes, or one that waits until it
   is cancelled. Each binding tests those with tests of its own.

### 3.10.5 How a case is run

| Step | What happens | A failure here gives |
| --- | --- | --- |
| 1. Read | The file is read and checked against 3.10.3 and 3.10.11. Each case is laid over the header | The verdict `invalid` for every case of the file |
| 2. Declare | The case's environment is built, as a child of the base environment | The outcome: a failure of phase `declare` |
| 3. Parse | The program is parsed | The outcome: a failure of phase `parse` |
| 4. Check | The program is checked against the case's environment, with `returns` as the expected result | The outcome: a failure of phase `check`. Otherwise the check gives a type and sites |
| 5. Run | Only when the case has `vars`, or `expect` has `value`, `truthy`, `steps` or `allocated`, or `expect` is a failure of phase `run`. The program is compiled and run, with the values of `vars` | The outcome: a value and its usage, or a failure of phase `run` and its usage |
| 6. Compare | The outcome is compared with `expect` (3.10.7) | The verdict `fail` |

1. A case that does not reach step 5 is only checked. Its outcome is a
   result, with a type and sites and no value.
2. Each case is run on its own. Nothing of one case reaches another.

### 3.10.6 What is expected

`expect` is a result or a failure. A failure has the field `phase`. A result
does not.

| Field of a result | Compared with | How |
| --- | --- | --- |
| `type` | The type the check gives | Both are parsed as type text and compared by structure (`05-types.md`) |
| `sites` | The sites the check gives, each an object with `class` and `span` | The whole list, in order |
| `value` | The value of the run | With the language's `==` (`01-language.md`) |
| `truthy` | The truthiness test (3.2.4) applied to the value | Equal, `true` or `false` |
| `steps` | The steps the run charged | Equal |
| `allocated` | The allocation units the run charged | Equal |

| Field of a failure | Compared with | How |
| --- | --- | --- |
| `phase` | The phase of the failure | Equal. A difference is the difference `outcome` |
| `diagnostics` | The diagnostics of the failure | 3.10.7 |
| `steps` | For phase `run`, when the code is not a limit code: the steps the failed run charged | Equal |
| `allocated` | For phase `run`, when the code is not a limit code: the allocation units the failed run charged | Equal |

Each element of `diagnostics` is an object with these fields.

| Field | Required | Compared with |
| --- | --- | --- |
| `code` | Yes | The code. Equal |
| `class` | No | The class. Equal |
| `span` | No | The span, as `[line, column, end line, end column]` in the source text. Equal |
| `path` | No | The path. Equal, step by step |
| `message` | No | The message, whole. Equal, code point by code point |
| `data` | No | The `data`, with the language's `==` |

1. `expect` with `phase` and any of `type`, `sites`, `value` and `truthy` makes
   the file invalid.
2. `steps` and `allocated` in an `expect` whose `phase` is not `run` make the
   file invalid.
3. `steps` and `allocated` in an `expect` whose diagnostic has a limit code
   make the file invalid. The limit codes are `steps-exceeded`,
   `allocation-exceeded`, `depth-exceeded`, `deadline-exceeded` and
   `cancelled`. The usage of such a run is not fixed (3.6.3), and no runner
   compares it.
4. A case that expects an error writes `expect` as a failure:
   `{"phase": "run", "diagnostics": [{"code": "divide-by-zero"}]}`.

### 3.10.7 How the outcome is compared

1. **A field present in `expect` is compared. A field absent from `expect` is
   not.** So `"expect": {}` means: the program checks, and the outcome is a
   result.
2. The outcome kind is compared first. A result where a failure is expected,
   a failure where a result is expected, and a failure of another phase are
   each the difference `outcome`. When `outcome` differs, no other field is
   compared.
3. Numbers compare by value: `2` and `2.0` are equal. The order of keys in a
   map does not matter. Both follow from `==`.
4. `diagnostics` for phase `check` or `run` is compared whole: the same
   number of diagnostics, and each expected one with the actual one at the
   same position. A run failure has one.
5. `diagnostics` for phase `parse` is compared with the start of what the
   parser reported: the first expected diagnostic with the first reported, and
   so on, for as many as the case gives. Further reported diagnostics are not
   compared.
6. `diagnostics` for phase `declare` is compared without regard to order: the
   same number of diagnostics, and each expected one equal to a different
   actual one.
7. For a limit code, `span` is not compared, even when the case gives it.
8. `class` is compared only by an implementation that checked the program at
   step 4.
9. For `function-error`, the message is the string of the stub's `error`. For
   `failed`, it is the text given to `fail`.
10. Step and unit counts are exact, for a result and for a run failure whose
    code is not a limit code. A case states them as whole numbers in `steps`
    and `allocated`. A case may leave either out, and then it is not compared.
    A case that expects a limit code does not state them (3.10.6, item 3).
    An author who wants only a bound gives `limits` instead: a run that
    passes it fails with `steps-exceeded` or `allocation-exceeded`.
11. No field states a tolerance.

### 3.10.8 Verdicts and what a runner reports

1. The verdict of a case is `pass`, `fail`, `skip` or `invalid`.
2. A `fail` carries the list of what differed. Its names are, in this order:
   `outcome`, `type`, `sites`, `value`, `truthy`, `diagnostics`, `steps`,
   `allocated`, `functions`. `diagnostics` is one name for the whole list.
3. A runner reports, for each case, its name and its verdict, and for a
   `fail` the list of what differed. For each difference it should also show
   what was expected and what was found.
4. A case with `skip` has the verdict `skip`, whatever else it holds, if the
   file is valid.
5. A runner with no type checker does not run the directory
   `suite/core-check/` (3.10.13). In any other case file it gives the verdict
   `skip` to a case whose `expect` has `type` or `sites`, or expects a failure
   of phase `check` with a code outside the static codes (3.10.13). It runs
   every other case with the static rules alone at step 4 (3.2.1, item 4), and
   does not compare `class`.
6. Each implementation publishes, for each directory of the suite, the number
   of cases of each verdict.

### 3.10.9 The base environment

1. Every case runs in a child of a base environment.
2. The suite's base environment is the default environment: the newest
   version, the default limits, the whole Core library, no variable, no host
   function, no opaque type, and the default set of rejected classes.
3. A binding may let a host run a case file against its own environment as
   the base. Its own host functions are then used except those the file
   stubs, and its own variables are supplied for names that `vars` does not
   give.

### 3.10.10 When a file is invalid

Each of these makes the file invalid, and every case of the file has the
verdict `invalid`.

1. Text that is not JSON, or an object that repeats a key.
2. A file that is neither an object with `cases` nor an array.
3. A field that 3.10.3 does not list, in the header, in a case, in `expect`,
   in a diagnostic, in a stub or in a row. The fields of each are exactly those
   this section names.
4. A field of the wrong JSON type, as the schema of 3.10.11 gives.
5. A case with no `name`, or a name used twice in the file.
6. A case with no `expect`.
7. A case with neither `source` nor `file`, or with both, after the header
   rule.
8. A `file` that cannot be read.
9. A key of `vars` that is not a field of `env`.
10. A stub row with both `result` and `error`, or with neither.
11. A stub with no `type`, for a name the base environment does not declare.
12. An `expect` that breaks 3.10.6, item 1, 2 or 3: a failure with `type`,
    `sites`, `value` or `truthy`; `steps` or `allocated` on a failure whose
    phase is not `run`; or `steps` or `allocated` on a failure whose
    diagnostic has a limit code.
13. A JSON number that reads as an infinity.

### 3.10.11 JSON Schema of a case file

The schema gives the shape of the file. The rules of 3.10.2 and 3.10.10 that
it cannot express still apply: the header rule, the one of `source` and `file`
after it, unique names, keys of `vars`, and stubs without `type`.

```json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "title": "Risor case file, Core",
  "oneOf": [
    {"type": "array", "items": {"$ref": "#/$defs/case"}},
    {"allOf": [
      {"$ref": "#/$defs/fields"},
      {"type": "object",
       "required": ["cases"],
       "properties": {
         "cases": {"type": "array", "items": {"$ref": "#/$defs/case"}}},
       "propertyNames": {"enum": [
         "cases", "source", "file", "version", "env", "opaque", "methods",
         "functions", "reject", "returns", "vars", "limits",
         "precondition"]}}
    ]}
  ],
  "$defs": {
    "fields": {
      "type": "object",
      "properties": {
        "source": {"type": "string"},
        "file": {"type": "string", "minLength": 1},
        "version": {"type": "string", "pattern": "^(0|[1-9][0-9]*)\\.(0|[1-9][0-9]*)$"},
        "env": {"type": "string"},
        "opaque": {"type": "array", "items": {"type": "string"}},
        "methods": {
          "type": "object",
          "propertyNames": {"pattern": "^[A-Z][A-Za-z0-9_]+\\.[A-Za-z_][A-Za-z0-9_]*$"},
          "additionalProperties": {"type": "string"}},
        "functions": {
          "type": "object",
          "additionalProperties": {"$ref": "#/$defs/stub"}},
        "reject": {"type": "array", "items": {"type": "string"}},
        "returns": {"type": "string"},
        "vars": {"type": "object"},
        "limits": {"$ref": "#/$defs/limits"},
        "precondition": {"type": "boolean"}
      }
    },
    "case": {
      "allOf": [
        {"$ref": "#/$defs/fields"},
        {"type": "object",
         "required": ["name", "expect"],
         "properties": {
           "name": {"type": "string"},
           "skip": {"type": "string"},
           "expect": {"$ref": "#/$defs/expect"}},
         "propertyNames": {"enum": [
           "name", "source", "file", "version", "env", "opaque", "methods",
           "functions", "reject", "returns", "vars", "limits",
           "precondition", "skip", "expect"]}}
      ]
    },
    "limits": {
      "type": "object",
      "properties": {
        "steps": {"type": "integer", "minimum": 0},
        "allocation": {"type": "integer", "minimum": 0},
        "depth": {"type": "integer", "minimum": 1, "maximum": 1000}
      },
      "additionalProperties": false
    },
    "stub": {
      "oneOf": [
        {"type": "string"},
        {"type": "object",
         "properties": {
           "type": {"type": "string"},
           "calls": {"type": "array", "items": {"$ref": "#/$defs/row"}}},
         "additionalProperties": false}
      ]
    },
    "row": {
      "type": "object",
      "properties": {
        "args": {"type": "array"},
        "result": true,
        "error": {"type": "string"},
        "charge": {"type": "integer", "minimum": 0},
        "allocate": {"type": "integer", "minimum": 0}
      },
      "oneOf": [
        {"required": ["result"], "not": {"required": ["error"]}},
        {"required": ["error"], "not": {"required": ["result"]}}
      ],
      "additionalProperties": false
    },
    "span": {
      "type": "array",
      "items": {"type": "integer", "minimum": 1},
      "minItems": 4,
      "maxItems": 4
    },
    "count": {"type": "integer", "minimum": 0},
    "limitCode": {"enum": [
      "steps-exceeded", "allocation-exceeded", "depth-exceeded",
      "deadline-exceeded", "cancelled"]},
    "noUsage": {"not": {"anyOf": [
      {"required": ["steps"]}, {"required": ["allocated"]}]}},
    "expect": {
      "oneOf": [
        {"$ref": "#/$defs/result"},
        {"$ref": "#/$defs/failure"}
      ]
    },
    "result": {
      "type": "object",
      "properties": {
        "type": {"type": "string"},
        "sites": {
          "type": "array",
          "items": {
            "type": "object",
            "required": ["class", "span"],
            "properties": {
              "class": {"enum": ["dynamic", "index", "divide", "value", "host"]},
              "span": {"$ref": "#/$defs/span"}},
            "additionalProperties": false}},
        "value": true,
        "truthy": {"type": "boolean"},
        "steps": {"$ref": "#/$defs/count"},
        "allocated": {"$ref": "#/$defs/count"}
      },
      "additionalProperties": false
    },
    "failure": {
      "type": "object",
      "required": ["phase", "diagnostics"],
      "properties": {
        "phase": {"enum": ["declare", "parse", "check", "run"]},
        "diagnostics": {
          "type": "array",
          "minItems": 1,
          "items": {"$ref": "#/$defs/diagnostic"}},
        "steps": {"$ref": "#/$defs/count"},
        "allocated": {"$ref": "#/$defs/count"}
      },
      "allOf": [
        {"if": {"properties": {"phase": {"not": {"const": "run"}}}},
         "then": {"$ref": "#/$defs/noUsage"}},
        {"if": {"properties": {"diagnostics": {"contains": {
           "type": "object", "required": ["code"],
           "properties": {"code": {"$ref": "#/$defs/limitCode"}}}}}},
         "then": {"$ref": "#/$defs/noUsage"}}
      ],
      "additionalProperties": false
    },
    "diagnostic": {
      "type": "object",
      "required": ["code"],
      "properties": {
        "code": {"type": "string", "pattern": "^[a-z]+(-[a-z0-9]+)*$"},
        "class": {"enum": [
          "constant-condition", "dynamic", "index", "divide", "value", "host",
          "environment", "range", "limit"]},
        "span": {"$ref": "#/$defs/span"},
        "path": {
          "type": "array",
          "maxItems": 64,
          "items": {"oneOf": [
            {"type": "string"},
            {"type": "integer", "minimum": 0}]}},
        "message": {"type": "string"},
        "data": true
      },
      "additionalProperties": false
    }
  }
}
```

### 3.10.12 Five example cases

This is one complete case file. Its header declares two variables and one
stubbed host function. Each case lays its own fields over the header.

```json
{
  "env": "{order: {total: float, coupon: string | null}, d: int}",
  "functions": {
    "rate": {"type": "(code: string) => float !",
             "calls": [{"args": ["SAVE10"], "result": 0.1},
                       {"error": "unknown coupon"}]}
  },
  "returns": "float",
  "cases": [
    {"name": "a known coupon gives its rate",
     "source": "order.total * (1 - rate(order.coupon ?? \"none\"))",
     "vars": {"order": {"total": 200, "coupon": "SAVE10"}, "d": 1},
     "expect": {"type": "float", "value": 180}},

    {"name": "an unknown coupon fails the run with the stub's text",
     "source": "order.total * (1 - rate(order.coupon ?? \"none\"))",
     "vars": {"order": {"total": 200, "coupon": "NOPE"}, "d": 1},
     "expect": {"phase": "run",
                "diagnostics": [{"code": "function-error", "class": "host",
                                 "message": "unknown coupon"}]}},

    {"name": "a caught error is charged for its value and its message",
     "source": "try { 1 / d } catch (err) { err.message }",
     "returns": "float | string",
     "vars": {"d": 0},
     "expect": {"type": "float | string",
                "value": "the divisor of / is zero",
                "steps": 3, "allocated": 10}},

    {"name": "a misspelled field is a check error",
     "source": "order.totl * 2",
     "expect": {"phase": "check",
                "diagnostics": [{"code": "no-field", "span": [1, 7, 1, 11]}]}},

    {"name": "a reserved word cannot be bound",
     "source": "const match = 1; match",
     "returns": "int",
     "expect": {"phase": "parse",
                "diagnostics": [{"code": "reserved-word",
                                 "span": [1, 7, 1, 12]}]}}
  ]
}
```

1. The first case states no counts, so none is compared.
2. The second case states the class `host`. A runner compares it only after
   a check.
3. The third case states exact counts. `02-evaluation.md` gives them: one step
   for `/`, one step and one unit for a message under 64 units of weight, nine
   units for the error value's map, and one step for the read of
   `err.message`.
4. The fourth case is only checked: it has no `vars`, and its `expect` names
   nothing that only a run gives.
5. The fifth case gives a `parse` failure. It states the code, which is a Core
   code, and the span, and leaves the message out: the catalog's `parse`
   templates come with a later package.

### 3.10.13 The suite's directories

The Core cases of the suite are in two directories, by what a case needs.

| Directory | Holds |
| --- | --- |
| `suite/core-run/` | Cases that every Core implementation passes with no type checker |
| `suite/core-check/` | Every Core case that needs the type checker |

A case belongs in `suite/core-run/` only when all of these hold:

1. Its `expect` has neither `type` nor `sites`.
2. A failure of phase `check` that it expects has static codes only.
3. A failure of phase `declare` that it expects is one that the grammar of
   type text alone finds (3.3.1, item 2).
4. Its program, checked by the full checker against its environment, has no
   diagnostic other than those it expects.

The **static codes** are the codes of the static rules of `01-language.md`:
`undefined-name`, `undefined-function`, `call-of-parameter`,
`lambda-to-lambda`, `not-a-value`, `arity`, `duplicate-name`,
`shadows-namespace`, `duplicate-key`, `duplicate-case` and `unreachable-arm`.
`arity` is static for a call by name only. On a method call it is a type rule,
so a case that expects `arity` on a method call belongs in
`suite/core-check/`.

## 3.11 What later packages add

These parts of the host contract and of the case format are not in Core. Each
comes with a later package of the specification, and adds without changing
anything above.

| Part | Adds |
| --- | --- |
| Types from JSON Schema and from host types | Declarations of a variable's type in JSON Schema, and the environment's description |
| Named types | The declaration of a named type, and the case field `types` |
| Templates and analysis | The stages of template parsing and analysis, the case field `template`, and the result field `names` |
| Checked conversion | The conformance function, and the case of the second kind, with the field `conform` |
| Opt-in library groups | The declaration that enables a group, and the case field `groups`. The runner's comparison of a result's JSON text (3.2.4) with what `json.stringify` gives |
| Errors, second part | The trace of a run diagnostic, and the field `trace` in an expected diagnostic. The three renderings. The `parse` and `check` templates of the catalog |
| Testing for authors | Testing the fixtures of a case against their declared types, which `"precondition": false` turns off. Update mode. Case files written as programs. Kinds of case that test the runner |
| Tools | The assist stage: completion and hover |

A case file that uses a field of this table is invalid for a Core runner.
