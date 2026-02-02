# math

Module `math` provides mathematical constants and functions.

Risor provides equivalence between float and int types, so many of the
functions in this module accept both float and int as inputs. In the documentation below, "number" is used to refer to either float or int.

## Constants

### pi

```go filename="Constant"
pi float
```

The ratio of a circle's circumference to its diameter.

```go filename="Example"
>>> math.pi
3.141592653589793
```

### e

```go filename="Constant"
e float
```

Euler's number, the base of natural logarithms.

```go filename="Example"
>>> math.e
2.718281828459045
```

### tau

```go filename="Constant"
tau float
```

The ratio of a circle's circumference to its radius (2 * pi).

```go filename="Example"
>>> math.tau
6.283185307179586
```

### inf

```go filename="Constant"
inf float
```

Positive infinity.

```go filename="Example"
>>> math.inf
+Inf
>>> -math.inf
-Inf
```

### nan

```go filename="Constant"
nan float
```

Not a number.

```go filename="Example"
>>> math.nan
NaN
```

## Functions

### abs

```go filename="Function signature"
abs(x number) number
```

Returns the absolute value of x.

```go filename="Example"
>>> math.abs(-2)
2
>>> math.abs(3.3)
3.3
```

### sign

```go filename="Function signature"
sign(x number) int
```

Returns -1 if x < 0, 0 if x == 0, or 1 if x > 0.

```go filename="Example"
>>> math.sign(-5)
-1
>>> math.sign(0)
0
>>> math.sign(3.14)
1
```

### floor

```go filename="Function signature"
floor(x number) number
```

Returns the largest integer value less than or equal to x.

```go filename="Example"
>>> math.floor(2.7)
2
>>> math.floor(-2.3)
-3
```

### ceil

```go filename="Function signature"
ceil(x number) number
```

Returns the smallest integer value greater than or equal to x.

```go filename="Example"
>>> math.ceil(2.3)
3
>>> math.ceil(-2.7)
-2
```

### round

```go filename="Function signature"
round(x number) float
```

Returns x rounded to the nearest integer.

```go filename="Example"
>>> math.round(1.4)
1
>>> math.round(1.5)
2
```

### trunc

```go filename="Function signature"
trunc(x number) float
```

Returns the integer value of x, truncating toward zero.

```go filename="Example"
>>> math.trunc(2.7)
2
>>> math.trunc(-2.7)
-2
```

### min

```go filename="Function signature"
min(x, y, ... number) number
min(list) number
```

Returns the smallest of the given values. Accepts multiple arguments or a list.

```go filename="Example"
>>> math.min(3, 1, 4, 1, 5)
1
>>> math.min([3, 1, 4, 1, 5])
1
```

### max

```go filename="Function signature"
max(x, y, ... number) number
max(list) number
```

Returns the largest of the given values. Accepts multiple arguments or a list.

```go filename="Example"
>>> math.max(3, 1, 4, 1, 5)
5
>>> math.max([3, 1, 4, 1, 5])
5
```

### clamp

```go filename="Function signature"
clamp(x, min, max number) number
```

Returns x constrained to the range [min, max].

```go filename="Example"
>>> math.clamp(5, 0, 10)
5
>>> math.clamp(-5, 0, 10)
0
>>> math.clamp(15, 0, 10)
10
```

### sum

```go filename="Function signature"
sum(list) number
```

Returns the sum of all numbers in a list.

```go filename="Example"
>>> math.sum([1, 2, 3, 4, 5])
15
>>> math.sum([])
0
```

### sqrt

```go filename="Function signature"
sqrt(x number) float
```

Returns the square root of x.

```go filename="Example"
>>> math.sqrt(4)
2
>>> math.sqrt(2)
1.4142135623730951
```

### cbrt

```go filename="Function signature"
cbrt(x number) float
```

Returns the cube root of x.

```go filename="Example"
>>> math.cbrt(8)
2
>>> math.cbrt(27)
3
```

### pow

```go filename="Function signature"
pow(x, y number) float
```

Returns x raised to the power of y.

```go filename="Example"
>>> math.pow(2, 3)
8
>>> math.pow(2, 0.5)
1.4142135623730951
```

### exp

```go filename="Function signature"
exp(x number) float
```

Returns e raised to the power of x.

```go filename="Example"
>>> math.exp(0)
1
>>> math.exp(1)
2.718281828459045
```

### log

```go filename="Function signature"
log(x number) float
```

Returns the natural logarithm (base e) of x.

```go filename="Example"
>>> math.log(1)
0
>>> math.log(math.e)
1
```

### log10

```go filename="Function signature"
log10(x number) float
```

Returns the base 10 logarithm of x.

```go filename="Example"
>>> math.log10(1)
0
>>> math.log10(100)
2
```

### log2

```go filename="Function signature"
log2(x number) float
```

Returns the base 2 logarithm of x.

```go filename="Example"
>>> math.log2(1)
0
>>> math.log2(8)
3
```

### sin

```go filename="Function signature"
sin(x number) float
```

Returns the sine of x (in radians).

```go filename="Example"
>>> math.sin(0)
0
>>> math.sin(math.pi / 2)
1
```

### cos

```go filename="Function signature"
cos(x number) float
```

Returns the cosine of x (in radians).

```go filename="Example"
>>> math.cos(0)
1
>>> math.cos(math.pi)
-1
```

### tan

```go filename="Function signature"
tan(x number) float
```

Returns the tangent of x (in radians).

```go filename="Example"
>>> math.tan(0)
0
>>> math.tan(math.pi / 4)
1
```

### asin

```go filename="Function signature"
asin(x number) float
```

Returns the arc sine of x in radians.

```go filename="Example"
>>> math.asin(0)
0
>>> math.asin(1)
1.5707963267948966
```

### acos

```go filename="Function signature"
acos(x number) float
```

Returns the arc cosine of x in radians.

```go filename="Example"
>>> math.acos(1)
0
>>> math.acos(0)
1.5707963267948966
```

### atan

```go filename="Function signature"
atan(x number) float
```

Returns the arc tangent of x in radians.

```go filename="Example"
>>> math.atan(0)
0
>>> math.atan(1)
0.7853981633974483
```

### atan2

```go filename="Function signature"
atan2(y, x number) float
```

Returns the arc tangent of y/x, using the signs of both to determine the quadrant.

```go filename="Example"
>>> math.atan2(1, 1)
0.7853981633974483
>>> math.atan2(-1, -1)
-2.356194490192345
```

### hypot

```go filename="Function signature"
hypot(x, y number) float
```

Returns the Euclidean distance sqrt(x*x + y*y).

```go filename="Example"
>>> math.hypot(3, 4)
5
>>> math.hypot(5, 12)
13
```

### sinh

```go filename="Function signature"
sinh(x number) float
```

Returns the hyperbolic sine of x.

```go filename="Example"
>>> math.sinh(0)
0
>>> math.sinh(1)
1.1752011936438014
```

### cosh

```go filename="Function signature"
cosh(x number) float
```

Returns the hyperbolic cosine of x.

```go filename="Example"
>>> math.cosh(0)
1
>>> math.cosh(1)
1.5430806348152437
```

### tanh

```go filename="Function signature"
tanh(x number) float
```

Returns the hyperbolic tangent of x.

```go filename="Example"
>>> math.tanh(0)
0
>>> math.tanh(1)
0.7615941559557649
```

### degrees

```go filename="Function signature"
degrees(x number) float
```

Converts radians to degrees.

```go filename="Example"
>>> math.degrees(math.pi)
180
>>> math.degrees(math.pi / 2)
90
```

### radians

```go filename="Function signature"
radians(x number) float
```

Converts degrees to radians.

```go filename="Example"
>>> math.radians(180)
3.141592653589793
>>> math.radians(90)
1.5707963267948966
```

### is_inf

```go filename="Function signature"
is_inf(x number) bool
```

Returns true if x is positive or negative infinity.

```go filename="Example"
>>> math.is_inf(math.inf)
true
>>> math.is_inf(-math.inf)
true
>>> math.is_inf(0)
false
```

### is_finite

```go filename="Function signature"
is_finite(x number) bool
```

Returns true if x is neither infinity nor NaN.

```go filename="Example"
>>> math.is_finite(1.0)
true
>>> math.is_finite(math.inf)
false
>>> math.is_finite(math.nan)
false
```

### is_nan

```go filename="Function signature"
is_nan(x number) bool
```

Returns true if x is NaN (not a number).

```go filename="Example"
>>> math.is_nan(math.nan)
true
>>> math.is_nan(0)
false
```
