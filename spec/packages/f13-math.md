# F13. Math functions

This chapter is normative. It specifies follow-on package F13: eleven entries
of the opt-in group `math`, `math.pow`, `math.exp`, `math.log`, `math.log10`,
`math.sin`, `math.cos`, `math.tan`, `math.asin`, `math.acos`, `math.atan` and
`math.atan2`. Each gives the correctly rounded result, the same double in every
implementation and on every platform. The package adds no node kind, value,
type, run code, message template, reserved word, declaration or case field.

The package only adds. Package F6 (`spec/packages/f06-groups.md`) defines the
group `math`, how a group is turned on, its entry `math.sqrt` and its two
constants. Without this package each of the eleven names is a name the group
does not have (F6 8.1 item 3). This package turns that error into a result.

Other chapters: `01-language.md` (numbers, `**`), `02-evaluation.md` (cost),
`03-host.md` (diagnostics, case files), `04-library.md` (the record format,
how an entry is specified), `05-types.md` (sites), and F6 (the group, the
records file of a group, the shared failures).

## 1. Scope

### 1.1 The entries

| Group | Entries | Constants |
| --- | --- | --- |
| `math` | 11: `math.pow`, `math.exp`, `math.log`, `math.log10`, `math.sin`, `math.cos`, `math.tan`, `math.asin`, `math.acos`, `math.atan`, `math.atan2` | None |

1. Every entry is a function of the group `math`, called as `math.sin(x)` when
   the group is on (F6 2.4). It is named for removal by its full name (F6 2.6).
2. Every entry is exact (2.1). The records have `exact` `true`.
3. `suite/library-math.json` has 11 records, one for each entry, in the format
   of F6 3.2.
4. With F6 and this package an implementation offers 72 library entries: the
   53 of Core, the 8 of F6 and these 11.

### 1.2 Packages this one requires

Packages are offered in a fixed order, each only with every package before it:
F3, F1, F2, F4, F5, F6, F7, F8, F9, F10, F11, F13, F14, F12. So an
implementation that offers F13 offers every package before it, F6 among them.

### 1.3 Version

The entries are part of version `3.0`, with `since` `3.0`. An implementation
that offers F6 and not F13 still offers version `3.0`; it lacks these entries.
This package adds no version number.

## 2. Correct rounding

### 2.1 The rule

1. Every entry gives the **correctly rounded** result: of the exact
   mathematical value of the function at its arguments, the nearest binary64
   value, ties to even, with subnormal values in the grid of binary64. This is
   one result, the same double in every implementation and on every platform.
2. An exact value whose magnitude rounds to 2^1024 or more, under that rule, is
   not finite: the call fails with `number-range`, template `entry`, whose
   parameter `name` is the entry's full name.
3. An exact value that rounds to zero gives zero. Negative zero is zero.
4. Where the exact value is not a real number the call fails with
   `invalid-argument`, form `not-real`, whose parameter `name` is the entry's
   full name. Each entry lists where. A value that is infinite at a pole, such
   as the logarithm of zero, is not a real number.
5. An implementation may compute a result in any way that gives the correctly
   rounded value for every argument. A platform's function that does not
   promise this may not be used for the result.
6. The arguments are doubles, and their exact values are used:
   `math.sin(math.pi)` is the sine of the double nearest to π, which is not
   zero.

| Entry | Exact value | `not-real` when |
| --- | --- | --- |
| `math.pow(x, y)` | x to the power y (4.1) | x = 0 and y < 0; x < 0 and y not an integer |
| `math.exp(x)` | e to the power x | Never |
| `math.log(x)` | The natural logarithm of x | x ≤ 0 |
| `math.log10(x)` | The base-10 logarithm of x | x ≤ 0 |
| `math.sin(x)`, `math.cos(x)`, `math.tan(x)` | The sine, cosine and tangent of x radians | Never |
| `math.asin(x)` | The arcsine of x, in [−π/2, π/2] | x < −1 or x > 1 |
| `math.acos(x)` | The arccosine of x, in [0, π] | x < −1 or x > 1 |
| `math.atan(x)` | The arctangent of x, in (−π/2, π/2) | Never |
| `math.atan2(y, x)` | The angle of the point (x, y), in (−π, π]; 0 when x and y are both zero (4.11) | Never |

### 2.2 Signatures and `!`

1. An entry that can fail with `invalid-argument` is marked `!`: `math.pow`,
   `math.log`, `math.log10`, `math.asin` and `math.acos`. The other six never
   fail with kind `value` and are not marked. `number-range` is never marked
   (`04-library.md` 3.1). F6's `math.sqrt` is marked.
2. A call of an entry marked `!` records a `value` site on the call
   (`05-types.md` 18.3). An entry not marked records none.
3. The tests of `04-library.md` 2.6 apply: an argument that is not a number,
   null included, fails with `wrong-kind`, template `argument`, before the
   call's step.

### 2.3 The constants and the entries

The constants of F6 8.2 agree with the entries: `math.pi` is `math.acos(-1)`
and `math.atan2(0, -1)`, and `math.e` is `math.exp(1)`.

| Program | Data | Result |
| --- | --- | --- |
| `math.sin(math.pi)` |  | `1.2246467991473532e-16` |
| `math.pi == math.acos(x)` | `{x: float}`, x = `-1` | `true`; 202 steps |
| `math.e == math.exp(1)` |  | `true`; 202 steps |

## 3. Cost

| Entry | Steps | Units |
| --- | --- | --- |
| Each of the 11 | `200` | `0` |

1. Each call of an entry charges 200 steps beyond the call's own step. The
   formula of each record is `200`.
2. **The order.** The arguments are evaluated and tested by `04-library.md`
   2.6; the call's 1 step is charged; the entry's **value test** is made, which
   reads only the arguments and finds `not-real` (2.1 item 4); then the 200
   steps are charged; then the result is computed. So:
   - a call that fails the tests of 2.6 charges nothing;
   - a call that fails `not-real` charges the call's step only, 1 step;
   - a call whose result is computed charges 201 steps, whether it gives a
     value or fails with `number-range`. A result too large for a number is
     found by computing it, so it comes after the 200 steps;
   - a run whose step limit the 200 steps pass fails with `steps-exceeded` at
     that charge, before the result is computed. Its usage is not fixed, as for
     every limit failure (`03-host.md` 3.6.3).
3. Every exact value counts as computed: `math.pow(5, 0)` charges 201 steps.
4. The work of a call is bounded by a constant that does not depend on its
   arguments (`02-evaluation.md` 8.5). An implementation's typical time for a
   call is within that of 200 ordinary steps; a rare argument that needs more
   precision may take longer.
5. `math.sqrt` and the constants of F6 keep their cost: 0 steps beyond the
   call, and 0 for a read of a constant.
6. This package adds no row to the tables of `02-evaluation.md`: the charge is
   each record's `steps` formula (`04-library.md` 4.1).

| Program | Data | Result |
| --- | --- | --- |
| `math.sin(x)` | `{x: float}`, x = `1` | `0.8414709848078965`; 201 steps, 0 units |
| `math.sin(x)` | the same, step limit 200 | Fails: `steps-exceeded` |
| `math.log(x)` | `{x: float}`, x = `0` | Fails: `invalid-argument`, form `not-real`; 1 step, 0 units |
| `math.exp(x)` | `{x: float}`, x = `710` | Fails: `number-range`; 201 steps, 0 units |
| `math.sin(x) + math.cos(x)` | `{x: float}`, x = `1` | `1.3817732906760363`; 403 steps: 201, 201 and the `+` 1 |
| `math.sin(x) + math.cos(x)` | the same, step limit 401 | Fails: `steps-exceeded`, at the 200 steps of `math.cos` |
| `[1, 2].map(n => math.atan(n))` | | 408 steps: the lambda 1, the call 1, `2n` 4, and 201 in each body. 6 units |

## 4. The entries

### 4.1 `math.pow`

| Field | Value |
| --- | --- |
| Signature | `math.pow: (x: float, y: float) => float !` |
| Steps, units | `200`, `0` |
| Fails | `invalid-argument`, form `not-real`: x is zero and y below zero, or x is below zero and y is not an integer; `number-range`, template `entry`: the result is too large for a number |
| Order | Tests; `not-real` tested; steps; result, or `number-range`. |

1. If y is zero, the result is 1, whatever x is, zero included.
2. Otherwise, if x is zero: 0 when y is above zero; `not-real` when y is below
   zero.
3. Otherwise, if x is below zero and y is not an integer (a double with a
   fraction), `not-real`. So `math.pow(-8, 1 / 3)` fails.
4. Otherwise the exact value of x to the power y, correctly rounded. For x
   below zero and y an integer, it is negative when y is odd. Every double of
   magnitude 2^53 or more is an even integer.
5. `math.pow` is correctly rounded and `**` is not (`01-language.md` 6.2): for
   a whole y the two may differ in the last digit, as `math.pow(1.1, 10)` and
   `1.1 ** 10` do.

| Program | Data | Result |
| --- | --- | --- |
| `math.pow(2, 0.5)` |  | `1.4142135623730951` |
| `math.pow(1.1, 10)` |  | `2.5937424601000023`. `1.1 ** 10` is `2.593742460100002` |
| `math.pow(0, 0)` |  | `1` |
| `math.pow(2, y)` | `{y: float}`, y = `-1075` | `0`: 2^−1075 is half of the least double, a tie, and goes to the even zero |
| `math.pow(x, y)` | `{x: float, y: float}`, x = `-8`, y = `1 / 3` | Fails: `invalid-argument`, form `not-real` |
| `math.pow(0, y)` | `{y: float}`, y = `-1` | Fails: `invalid-argument`, form `not-real` |
| `math.pow(10, 400)` |  | Fails: `number-range` |

### 4.2 `math.exp`

| Field | Value |
| --- | --- |
| Signature | `math.exp: (x: float) => float` |
| Steps, units | `200`, `0` |
| Fails | `number-range`, template `entry`: the result is too large for a number |
| Order | Tests; steps; result, or `number-range`. |

| Program | Data | Result |
| --- | --- | --- |
| `math.exp(1)` |  | `2.718281828459045` |
| `math.exp(710)` |  | Fails: `number-range` |
| `math.exp(x)` | `{x: float}`, x = `-746` | `0` |

### 4.3 `math.log`

| Field | Value |
| --- | --- |
| Signature | `math.log: (x: float) => float !` |
| Steps, units | `200`, `0` |
| Fails | `invalid-argument`, form `not-real`: x is zero or below |
| Order | Tests; `not-real` tested; steps; result. |

| Program | Data | Result |
| --- | --- | --- |
| `math.log(10)` |  | `2.302585092994046` |
| `math.log(0)` |  | Fails: `invalid-argument`, form `not-real` |

### 4.4 `math.log10`

| Field | Value |
| --- | --- |
| Signature | `math.log10: (x: float) => float !` |
| Steps, units | `200`, `0` |
| Fails | `invalid-argument`, form `not-real`: x is zero or below |
| Order | Tests; `not-real` tested; steps; result. |

1. A power of ten that is a double gives its exponent exactly:
   `math.log10(1000)` is `3`.

| Program | Data | Result |
| --- | --- | --- |
| `math.log10(1000)` |  | `3` |
| `math.log10(2)` |  | `0.3010299956639812` |

### 4.5 `math.sin`, 4.6 `math.cos`, 4.7 `math.tan`

| Field | `math.sin` | `math.cos` | `math.tan` |
| --- | --- | --- | --- |
| Signature | `(x: float) => float` | `(x: float) => float` | `(x: float) => float` |
| Steps, units | `200`, `0` | `200`, `0` | `200`, `0` |
| Fails | Only as 2.2 item 3 says | Only as 2.2 item 3 says | Only as 2.2 item 3 says |
| Order | Tests; steps; result. | Tests; steps; result. | Tests; steps; result. |

1. Every finite x is accepted, however large. The argument is reduced exactly:
   `math.sin(1e22)` is the sine of the exact value of the double `1e22`.
2. No double is an odd multiple of π/2, so `math.tan` never fails.

| Program | Data | Result |
| --- | --- | --- |
| `math.sin(1)` |  | `0.8414709848078965` |
| `math.sin(3.141592653589793)` |  | `1.2246467991473532e-16` |
| `math.sin(1e22)` |  | `-0.8522008497671888` |
| `math.cos(1)` |  | `0.5403023058681398` |
| `math.tan(1)` |  | `1.5574077246549023` |
| `math.tan(1.5707963267948966)` |  | `16331239353195370` |

### 4.8 `math.asin`, 4.9 `math.acos`

| Field | `math.asin` | `math.acos` |
| --- | --- | --- |
| Signature | `(x: float) => float !` | `(x: float) => float !` |
| Steps, units | `200`, `0` | `200`, `0` |
| Fails | `invalid-argument`, form `not-real`: x is below −1 or above 1 | The same |
| Order | Tests; `not-real` tested; steps; result. | The same |

| Program | Data | Result |
| --- | --- | --- |
| `math.asin(0.5)` |  | `0.5235987755982989` |
| `math.asin(2)` |  | Fails: `invalid-argument`, form `not-real` |
| `math.acos(x)` | `{x: float}`, x = `-1` | `3.141592653589793` |

### 4.10 `math.atan`, 4.11 `math.atan2`

| Field | `math.atan` | `math.atan2` |
| --- | --- | --- |
| Signature | `(x: float) => float` | `(y: float, x: float) => float` |
| Steps, units | `200`, `0` | `200`, `0` |
| Fails | Only as 2.2 item 3 says | Only as 2.2 item 3 says |
| Order | Tests; steps; result. | Tests; steps; result. |

1. `math.atan2(y, x)` is the angle θ in (−π, π] with `cos θ = x / r` and `sin θ
   = y / r`, where `r` is √(x² + y²), correctly rounded. The language has one
   zero, so with y zero and x below zero the angle is π, and its result is
   `math.pi`.
2. `math.atan2(0, 0)` is `0`.

| Program | Data | Result |
| --- | --- | --- |
| `math.atan(x)` | `{x: float}`, x = `-0.5` | `-0.4636476090008061` |
| `math.atan2(0, x)` | `{x: float}`, x = `-1` | `3.141592653589793` |
| `math.atan2(0, 0)` |  | `0` |
| `math.atan2(y, x)` | `{y: float, x: float}`, y = `-1`, x = `-1` | `-2.356194490192345` |

## 5. Records and cases

1. `suite/library-math.json` holds the 11 records, in the order of 1.1. Each is
   a function's record of F6 3.2, with `group` `math`. Every case of a record
   states `groups` with `math`.
2. A runner of an implementation that offers F13 loads `library-math.json`
   beside `library.json` and `library-groups.json`, and runs its cases and the
   files of `suite/math/`, as it runs those of F6 (F6 11.4).
3. `suite/math/math-f06-texts.json` holds the texts of
   `suite/groups/until-f13-math-functions.json` with this package's outcomes.
   That `until-f13-` file is removed when this package's cases are added.
4. The suite states bare results, compared with `==`: one double, by 2.1. No
   case states a tolerance.

## 6. Examples

| Program | Data | Result |
| --- | --- | --- |
| `fixed(math.log10(x), 3)` | `{x: float}`, x = `2` | `"0.301"`; 202 steps, 1 unit |
| `[math.pow(1.1, 10), 1.1 ** 10]` |  | `[2.5937424601000023, 2.593742460100002]`; 202 steps, 3 units |
