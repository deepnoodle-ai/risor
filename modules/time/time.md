# time

Module `time` provides functionality for measuring and displaying time.

This is primarily a wrapper of the Go [time](https://pkg.go.dev/time) package,
with the one difference being duration values are represented as float values
(in seconds) instead of a dedicated duration type.

## Constants

Predefined layouts for use in `time.parse` and `time.format` include:

- ANSIC
- UnixDate
- RubyDate
- RFC822
- RFC822Z
- RFC850
- RFC1123
- RFC1123Z
- RFC3339
- RFC3339Nano
- Kitchen
- Stamp
- StampMilli
- StampMicro
- StampNano

### Example Constant Usage

```go copy filename="Example"
>>> time.parse(time.RFC3339, "2023-08-01T12:00:00-04:00")
time("2023-08-01T12:00:00-04:00")
```

## Functions

### now

```go filename="Function signature"
now() time
```

Returns the current time as a time object.

```go copy filename="Example"
>>> time.now()
time("2024-01-15T12:51:10-05:00")
```

### unix

```go filename="Function signature"
unix() time
```

Unix returns the local Time corresponding to the given Unix time, sec seconds and nsec nanoseconds since January 1, 1970 UTC

```go copy filename="Example"
>>> time.unix(1725885470, 0)
time("2024-09-09T14:37:50+02:00")
```

### parse

```go filename="Function signature"
parse(layout, value string) time
```

Parses a string into a time.

```go copy filename="Example"
>>> time.parse(time.RFC3339, "2023-08-07T21:19:27-04:00")
time("2023-08-07T21:19:27-04:00")
```

### since

```go filename="Function signature"
since(t time) float
```

Returns the elapsed time in seconds since the given time.

```go copy filename="Example"
>>> let t = time.now()
>>> time.since(t)
1.864104666
```

### sleep

```go filename="Function signature"
sleep(duration float)
```

Sleeps for the given duration in seconds.

```go copy filename="Example"
>>> time.sleep(1)
```

## Types

### time

The `time` type represents a moment in time.

#### Methods

##### time.add_date

```go filename="Method signature"
add_date(years int, months int, days int) bool
```

Returns the time corresponding to adding the given number of years, months, and days.

```go copy filename="Example"
>>> time.now().add_date(1,0,0)
time("2026-01-16T10:50:28+01:00")
```

##### time.before

```go filename="Method signature"
before(t time) bool
```

Returns whether this time is before the given time.

```go copy filename="Example"
>>> let t = time.parse(time.RFC3339, "2023-08-01T12:00:00-04:00")
>>> t.before(time.parse(time.RFC3339, "2023-08-02T00:00:00-04:00"))
true
```

##### time.after

```go filename="Method signature"
after(t time) bool
```

Returns whether this time is after the given time.

```go copy filename="Example"
>>> let t = time.parse(time.RFC3339, "2023-08-01T12:00:00-04:00")
>>> t.after(time.parse(time.RFC3339, "2023-08-02T00:00:00-04:00"))
false
```

##### time.format

```go filename="Method signature"
format(layout string) string
```

Formats the time according to the given layout.

```go copy filename="Example"
>>> let t = time.parse(time.RFC3339, "2023-08-01T12:00:00-04:00")
>>> t.format(time.RFC3339)
"2023-08-01T12:00:00-04:00"
>>> t.format(time.Kitchen)
"12:00PM"
>>> t.format(time.UnixDate)
"Tue Aug  1 12:00:00 EDT 2023"
```

##### time.utc

```go filename="Method signature"
utc() time
```

Returns the UTC time corresponding to this time.

```go copy filename="Example"
>>> let t = time.parse(time.RFC3339, "2023-08-01T12:00:00-04:00")
>>> t.utc()
time("2023-08-01T16:00:00Z")
```

##### time.unix

```go filename="Method signature"
unix() int
```

Returns the number of seconds elapsed since the Unix epoch.

```go copy filename="Example"
>>> let t = time.parse(time.RFC3339, "2023-08-01T12:00:00-04:00")
>>> t.unix()
1690905600
```
