# regexp

Module `regexp` provides regular expression matching using RE2 syntax.

## Functions

### compile

```go filename="Function signature"
compile(pattern string) regexp
```

Compiles a regular expression pattern into a regexp object for repeated use.

```go filename="Example"
>>> let r = regexp.compile("[0-9]+")
>>> r.test("abc123")
true
>>> r.find("abc123def")
"123"
```

### match

```go filename="Function signature"
match(pattern, str string) bool
```

Returns true if the pattern matches anywhere in the string. For repeated use, compile the pattern first with `regexp.compile()`.

```go filename="Example"
>>> regexp.match("[0-9]+", "abc123")
true
>>> regexp.match("[0-9]+", "abc")
false
```

### find

```go filename="Function signature"
find(pattern, str string) string | nil
```

Returns the first match of the pattern in the string, or nil if no match.

```go filename="Example"
>>> regexp.find("[0-9]+", "abc123def456")
"123"
>>> regexp.find("[0-9]+", "abc")
nil
```

### find_all

```go filename="Function signature"
find_all(pattern, str string) list
find_all(pattern, str string, n int) list
```

Returns all matches of the pattern in the string. With an optional third argument, limits to n matches.

```go filename="Example"
>>> regexp.find_all("[0-9]+", "a1b2c3")
["1", "2", "3"]
>>> regexp.find_all("[0-9]+", "a1b2c3", 2)
["1", "2"]
```

### search

```go filename="Function signature"
search(pattern, str string) int
```

Returns the index of the first match, or -1 if no match. The index is in characters (runes), not bytes.

```go filename="Example"
>>> regexp.search("[0-9]+", "abc123")
3
>>> regexp.search("[0-9]+", "abc")
-1
```

### replace

```go filename="Function signature"
replace(pattern, str, repl string) string
replace(pattern, str, repl string, count int) string
```

Replaces matches in the string. With 3 arguments, replaces all matches. With 4 arguments, replaces up to count matches (0 means all).

The replacement string can use `$1`, `$2`, etc. for captured groups.

```go filename="Example"
>>> regexp.replace("[0-9]+", "a1b2c3", "X")
"aXbXcX"
>>> regexp.replace("[0-9]+", "a1b2c3", "X", 2)
"aXbXc3"
>>> regexp.replace("(\\w+)@(\\w+)", "user@host", "$2:$1")
"host:user"
```

### split

```go filename="Function signature"
split(pattern, str string) list
split(pattern, str string, n int) list
```

Splits the string by the pattern. With an optional third argument, limits to n substrings.

```go filename="Example"
>>> regexp.split("[,;]", "a,b;c,d")
["a", "b", "c", "d"]
>>> regexp.split("[,;]", "a,b;c,d", 2)
["a", "b;c,d"]
```

### escape

```go filename="Function signature"
escape(str string) string
```

Returns a string with all regular expression metacharacters escaped, so the result matches the literal text.

```go filename="Example"
>>> regexp.escape("hello.world")
"hello\\.world"
>>> regexp.escape("[a-z]+")
"\\[a-z\\]\\+"
```

## Types

### regexp

Represents a compiled regular expression.

#### Properties

##### pattern

```go filename="Property"
pattern string
```

Returns the original pattern string.

```go filename="Example"
>>> let r = regexp.compile("[0-9]+")
>>> r.pattern
"[0-9]+"
```

##### num_groups

```go filename="Property"
num_groups int
```

Returns the number of capturing groups in the pattern.

```go filename="Example"
>>> let r = regexp.compile("(\\w+)@(\\w+)")
>>> r.num_groups
2
```

#### Methods

##### test

```go filename="Method signature"
test(str string) bool
```

Returns true if the pattern matches anywhere in the string. Alias for `match`.

```go filename="Example"
>>> let r = regexp.compile("[0-9]+")
>>> r.test("abc123")
true
>>> r.test("abc")
false
```

##### match

```go filename="Method signature"
match(str string) bool
```

Returns true if the pattern matches anywhere in the string.

```go filename="Example"
>>> let r = regexp.compile("[0-9]+")
>>> r.match("abc123")
true
```

##### find

```go filename="Method signature"
find(str string) string | nil
```

Returns the first match of the pattern in the string, or nil if no match.

```go filename="Example"
>>> let r = regexp.compile("[0-9]+")
>>> r.find("abc123def")
"123"
>>> r.find("abc")
nil
```

##### find_all

```go filename="Method signature"
find_all(str string) list
find_all(str string, n int) list
```

Returns all matches of the pattern in the string. With an optional second argument, limits to n matches.

```go filename="Example"
>>> let r = regexp.compile("[0-9]+")
>>> r.find_all("a1b2c3")
["1", "2", "3"]
>>> r.find_all("a1b2c3", 2)
["1", "2"]
```

##### search

```go filename="Method signature"
search(str string) int
```

Returns the index of the first match, or -1 if no match. The index is in characters (runes), not bytes.

```go filename="Example"
>>> let r = regexp.compile("[0-9]+")
>>> r.search("abc123")
3
>>> r.search("abc")
-1
```

##### groups

```go filename="Method signature"
groups(str string) list | nil
```

Returns the first match and all captured groups, or nil if no match. The first element is the full match, followed by each captured group.

```go filename="Example"
>>> let r = regexp.compile("(\\w+)@(\\w+)")
>>> r.groups("user@host")
["user@host", "user", "host"]
>>> r.groups("no match")
nil
```

##### find_all_groups

```go filename="Method signature"
find_all_groups(str string) list
find_all_groups(str string, n int) list
```

Returns all matches with their captured groups. Each element is a list where the first element is the full match, followed by captured groups.

```go filename="Example"
>>> let r = regexp.compile("(\\w+)@(\\w+)")
>>> r.find_all_groups("user@host admin@server")
[["user@host", "user", "host"], ["admin@server", "admin", "server"]]
```

##### replace

```go filename="Method signature"
replace(str, repl string) string
replace(str, repl string, count int) string
```

Replaces matches in the string. With 2 arguments, replaces all matches. With 3 arguments, replaces up to count matches (0 means all).

```go filename="Example"
>>> let r = regexp.compile("[0-9]+")
>>> r.replace("a1b2c3", "X")
"aXbXcX"
>>> r.replace("a1b2c3", "X", 1)
"aXb2c3"
```

##### replace_all

```go filename="Method signature"
replace_all(str, repl string) string
```

Replaces all matches in the string. Same as `replace(str, repl)`.

```go filename="Example"
>>> let r = regexp.compile("[0-9]+")
>>> r.replace_all("a1b2c3", "X")
"aXbXcX"
```

##### split

```go filename="Method signature"
split(str string) list
split(str string, n int) list
```

Splits the string by the pattern. With an optional second argument, limits to n substrings.

```go filename="Example"
>>> let r = regexp.compile("[,;]")
>>> r.split("a,b;c")
["a", "b", "c"]
>>> r.split("a,b;c", 2)
["a", "b;c"]
```
