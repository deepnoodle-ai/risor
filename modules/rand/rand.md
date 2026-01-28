# rand

Module `rand` provides pseudo-random number generation.

This module is not safe for security-sensitive applications.

## Functions

### random

```go filename="Function signature"
random() float
```

Returns a random float in [0.0, 1.0). Equivalent to Python's `random.random()` or JavaScript's `Math.random()`.

```go filename="Example"
>>> rand.random()
0.44997274093073925
```

### int

```go filename="Function signature"
int() int
int(max int) int
int(min, max int) int
```

Returns a random integer:
- With no arguments: returns a random non-negative int64
- With one argument: returns a random int in [0, max)
- With two arguments: returns a random int in [min, max)

```go filename="Example"
>>> rand.int()
1667297659146365586
>>> rand.int(10)
7
>>> rand.int(5, 10)
8
```

### randint

```go filename="Function signature"
randint(a, b int) int
```

Returns a random integer in [a, b] inclusive. Matches Python's `random.randint(a, b)` behavior.

```go filename="Example"
>>> rand.randint(1, 6)
4
```

### uniform

```go filename="Function signature"
uniform(a, b float) float
```

Returns a random float in [a, b]. Matches Python's `random.uniform(a, b)` behavior.

```go filename="Example"
>>> rand.uniform(0, 100)
42.5678
```

### normal

```go filename="Function signature"
normal() float
normal(mu, sigma float) float
```

Returns a random float from a normal (Gaussian) distribution.
- With no arguments: standard normal (mean=0, stddev=1)
- With two arguments: normal distribution with given mean and standard deviation

```go filename="Example"
>>> rand.normal()
0.44997274093073925
>>> rand.normal(100, 15)
112.34
```

### exponential

```go filename="Function signature"
exponential() float
exponential(lambda float) float
```

Returns a random float from an exponential distribution.
- With no arguments: lambda=1
- With one argument: uses the given rate parameter (lambda)

```go filename="Example"
>>> rand.exponential()
0.17764313580968902
>>> rand.exponential(0.5)
1.234
```

### choice

```go filename="Function signature"
choice(list) any
```

Returns a random element from a list. Raises an error if the list is empty.

```go filename="Example"
>>> rand.choice(["red", "green", "blue"])
"green"
>>> rand.choice([1, 2, 3, 4, 5])
3
```

### sample

```go filename="Function signature"
sample(list, k int) list
```

Returns k unique random elements from a list (without replacement). Raises an error if k is larger than the list length.

```go filename="Example"
>>> rand.sample([1, 2, 3, 4, 5], 3)
[4, 1, 5]
>>> rand.sample(["a", "b", "c", "d"], 2)
["c", "a"]
```

### shuffle

```go filename="Function signature"
shuffle(list) list
```

Shuffles a list in place and returns it.

```go filename="Example"
>>> let items = [1, 2, 3, 4, 5]
>>> rand.shuffle(items)
[3, 1, 5, 4, 2]
```

### bytes

```go filename="Function signature"
bytes(n int) list
```

Returns a list of n random bytes (integers 0-255).

```go filename="Example"
>>> rand.bytes(4)
[172, 45, 231, 89]
```
