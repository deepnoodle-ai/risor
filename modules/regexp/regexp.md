# regexp

Module regexp provides regular expression matching.

The supported regular expression syntax is exactly as described
in the [Go regexp](https://pkg.go.dev/regexp) documentation.

## Functions

### compile

```go filename="Function signature"
compile(expr string) regexp
```

Compiles a regular expression string into a regexp object.

```go filename="Example"
>>> regexp.compile("a+")
regexp("a+")
>>> let r = regexp.compile("a+"); r.match("a")
true
>>> let r = regexp.compile("[0-9]+"); r.match("nope")
false
```

### match

```go filename="Function signature"
match(expr, s string) bool
```

Returns true if the string s contains any match of the regular expression pattern.

```go filename="Example"
>>> regexp.match("ab+a", "abba")
true
>>> regexp.match("[0-9]+", "nope")
false
```

## Types

### regexp

Represents a compiled regular expression.

#### Methods

##### regexp.match

```go filename="Method signature"
match(s string) bool
```

Returns true if the string s contains any match of the regular expression pattern.

```go filename="Example"
>>> let r = regexp.compile("a+"); r.match("a")
true
```

##### regexp.find

```go filename="Method signature"
find(s string) string
```

Returns the leftmost match of the regular expression pattern in the string s.

```go filename="Example"
>>> let r = regexp.compile("a+"); r.find("baaab")
"aaa"
```

##### regexp.find_all

```go filename="Method signature"
find_all(s string) []string
```

Returns a slice of all matches of the regular expression pattern in the string s.

```go filename="Example"
>>> let r = regexp.compile("(du)+"); r.find_all("dunk dug in the deep end")
["du", "du"]
```

##### regexp.find_submatch

```go filename="Method signature"
find_submatch(s string) []string
```

Returns a slice of all matches of the regular expression pattern in the string s,
and the matches, if any, of its subexpressions.

```go filename="Example"
>>> let r = regexp.compile("a(b+)a"); r.find_submatch("abba")
["abba", "bb"]
```

##### regexp.replace_all

```go filename="Method signature"
replace_all(s, repl string) string
```

Returns a copy of the string s with all matches of the regular expression pattern
replaced by repl.

```go filename="Example"
>>> let r = regexp.compile("a+"); r.replace_all("baaab", "x")
"bxb"
```

##### regexp.split

```go filename="Method signature"
split(s string) []string
```

Splits the string s into a slice of substrings separated by the regular expression
pattern.

```go filename="Example"
>>> let r = regexp.compile("a+"); r.split("baaab")
["b", "b"]
```
