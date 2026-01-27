# rand

Module `rand` provides pseudo-random number generation.

As with the Go [math/rand](https://pkg.go.dev/math/rand) package
on which it is based, this module is not safe for use for
security-sensitive applications.

## Functions

### float

```go filename="Function signature"
float() float
```

Returns a random float between 0 and 1.

```go filename="Example"
>>> rand.float()
0.44997274093073925
```

### int

```go filename="Function signature"
int() int
```

Returns a non-negative pseudo-random 64 bit integer.

```go filename="Example"
>>> rand.int()
1667297659146365586
```

### intn

```go filename="Function signature"
intn(n int) int
```

Returns a non-negative pseudo-random 64 bit integer in the range [0, n).

```go filename="Example"
>>> rand.intn(10)
7
```

### norm_float

```go filename="Function signature"
norm_float() float
```

Returns a normally distributed float in the range [-math.MaxFloat64, +math.MaxFloat64]
with standard normal distribution (mean = 0, stddev = 1).

```go filename="Example"
>>> rand.norm_float()
0.44997274093073925
```

### exp_float

```go filename="Function signature"
exp_float() float
```

Returns an exponentially distributed float in the range (0, +math.MaxFloat64]
with an exponential distribution whose rate parameter (lambda) is 1 and whose
mean is 1/lambda (1).

```go filename="Example"
>>> rand.exp_float()
0.17764313580968902
```

### shuffle

```go filename="Function signature"
shuffle(list)
```

Shuffles a list in place and returns it.

```go filename="Example"
>>> let l = [1, 2, 3, 4, 5]
>>> rand.shuffle(l)
[3, 1, 5, 4, 2]
```
