# Resource Limits

Risor's limits catch scripts that go wrong by accident: a runaway recursion, a
loop that never ends, or a typo that asks for a billion-character string. They
are not a security boundary. A script written to exhaust memory can do it
despite every option on this page.

**To run scripts you don't trust, put them in a process the operating system
can limit.** See [Running untrusted scripts](#running-untrusted-scripts).

## What Risor bounds

| Option | Bounds | Error |
| --- | --- | --- |
| `WithMaxSteps(n)` | Instructions executed in one run | `ErrStepLimitExceeded` |
| `WithTimeout(d)` | Wall-clock time of the VM run, checked between instructions | `context.DeadlineExceeded` |
| `WithMaxStackDepth(n)` | Value stack depth and call depth | `ErrStackOverflow` |
| `WithLimits(Limits{...})` | Size of the value that one operation produces | `value error` |

A script cannot catch the step limit, a stack overflow, or the timeout. It can
catch a `value error` with `try`/`catch`.

`WithTimeout` covers only the VM run. `risor.Eval` parses and compiles the
source before the run starts, and converts the result to Go after it ends.
Neither is covered.

### `WithLimits`

`WithLimits` caps the operations that can turn a small input into a large
output:

- **On strings and bytes:** `repeat`, `replace_all`, `join`, `+`, `to_upper`,
  `to_lower`, string templates, `string()` of a list or map, and `encode`.
- **On lists:** list literals, `list()`, spread, `+`, `extend`, `append`,
  `insert`, `range(...).map`, `split`, `fields`, the `regexp` find and split
  functions, and `rand.bytes`.

The defaults are 64 MiB for a string or bytes value and 4,194,304 list
elements. A zero or negative field uses the default.

```go
risor.Eval(ctx, src,
    risor.WithEnv(risor.Builtins()),
    risor.WithLimits(risor.Limits{MaxBytesSize: 1 << 20, MaxListLength: 100_000}))
```

`MaxListLength` also caps conversions between Risor and Go. One conversion
produces at most `MaxListLength` elements, counting every list item, map entry,
and range element. A list referenced more than once counts each time. The
conversions it covers:

- **Risor to Go:** a script result, a host function argument, a value assigned
  to a Go struct field, and the values passed to `sprintf`, `error`, and
  `encode`.
- **Go to Risor:** a Go struct field that a script reads, and the results of a
  Go function or method that a script calls.

The error message contains "value too large to convert". The script result is
converted after the script ends, so the script cannot catch that error. It
reaches the Go caller as a `*errors.StructuredError` of kind `value error`.

#### Struct fields and embedders

The VM applies the limits to struct fields when the value's type is exactly
`*object.GoStruct`, or when a host type implements `object.AttrAssigner` (for
writes) or `object.AttrReader` (for reads). A host type that embeds
`*object.GoStruct` gets the limits only if it defines its own `AssignAttr` and
`ReadAttr`, which can delegate to `GoStruct.SetAttrContext` and
`GoStruct.GetAttrContext`. Otherwise the VM calls its `SetAttr` and `GetAttr`,
which use the default limits. Each read converts the field again.

In Go code, `Interface()`, `Inspect()`, and `String()` have no context and use
the default limits. Call `object.ToInterface(ctx, obj)` or
`object.InspectWithLimits(ctx, obj)` to apply the limits in `ctx`. A `print`
builtin that you add should call `object.CheckFormatSize(ctx, ...)` and format
with `object.InspectWithLimits`, as the `risor` CLI does.

## What Risor does not bound

These are known gaps. We do not plan to close them, because closing them one at
a time adds more complexity than the result is worth. Go gives a library no way
to count or refuse allocations, so every path that allocates would have to be
found and limited by hand. The list is not complete.

- **Total memory.** The caps apply to each operation, not to the run. A script
  can make many values, each under the cap, and keep all of them. There is no
  `WithMaxMemory`. A total budget was designed and rejected: see
  [`docs/design/2026_10_max_allocated_bytes.md`](../design/2026_10_max_allocated_bytes.md).
- **Operations that build first and check afterward.** `sprintf` formats its
  whole result and then checks its size. `error()` does not check the size of
  its message. `decode` builds the whole value, then checks only the length of
  each list, so it does not cap map entries. Each of these can allocate well
  past `MaxBytesSize` before it fails, and a script can catch the failure and
  repeat it. `encode` checks the text size of its input before encoding, but
  checks its output only after, so hex or base64 output can reach about twice
  `MaxBytesSize` before it fails.
- **Operations with no cap.** `sorted` and `chunk` have no size check.
- **Values from the host.** Values passed to `WithEnv` are converted once, when
  the run starts, without limits. `TypeRegistry.FromGo` has no limits.
- **Allocation inside Go code.** A builtin, or a function you expose, can
  allocate as much as it likes. The limits see only the value it returns.
- **Time inside Go code.** `WithTimeout` is checked between instructions, so a
  builtin that runs for a long time cannot be interrupted. For example,
  `string()` of a list that holds itself many times walks up to `MaxBytesSize`
  values before it fails.

## Running untrusted scripts

Use a boundary that the operating system enforces, with both a memory limit
and a deadline. A memory limit alone does not stop a script that is stuck in Go
code, so something outside the script must also end it at a deadline. In order
of effort:

- **AWS Lambda**, or a similar per-invocation runtime. Each invocation gets a
  memory ceiling and a timeout. If you already run in a cloud, this is usually
  the least work. Lambda can reuse an execution environment, and its process,
  for later invocations. State that your host code keeps, such as globals or
  caches, can then reach the next script. If scripts must not share state, run
  each one in a new subprocess inside the function, or keep no state between
  invocations.
- **A container with a memory limit:** `--memory` on Docker, or
  `resources.limits.memory` on Kubernetes. The kernel kills the container, not
  your host. Have the process that starts the container stop it at a deadline.
- **A subprocess with a memory limit,** started with `exec.CommandContext` so
  that it is killed at a deadline. Pass data in and out as JSON. This works on
  a single machine and keeps a failure to one child process. For the memory
  limit, prefer a cgroup limit, such as `systemd-run --scope -p MemoryMax=...`,
  which counts the memory the process uses. `RLIMIT_AS` and `RLIMIT_DATA` limit
  address space instead. The Go runtime reserves address space when it starts,
  so a low value can stop the child before the script runs. If you use an
  rlimit, test the value with your real binary.

All three fail safely: a script that goes too far ends its own process and
nothing else. Set `WithMaxSteps`, `WithTimeout`, and tight `WithLimits` inside
the boundary as well, so that most mistakes fail fast with a clear error
instead of a killed process.

You can also reduce what a script can reach:

- **Start from an empty environment.** It is the default. `risor.Builtins()` is
  opt-in, so add only the functions your scripts need.
- **Turn off syntax you don't need** with `WithSyntax`. Risor v2 has no loop
  statements, so a script iterates only through callbacks and recursion.
  `DisallowFuncDef` stops a script from defining functions, so it cannot
  recurse or write its own callbacks. It can still call functions from the
  environment and pass them as callbacks, as in `range(1000000).each(string)`.
  To remove iteration, also keep callback-taking functions out of the
  environment, or set `DisallowFuncCall`, which blocks every call, including
  method calls.

These reduce the risk. They do not remove the need for the boundary.

## See also

- [`docs/reviews/2026_08_memory_bounding.md`](../reviews/2026_08_memory_bounding.md),
  an earlier analysis with measurements.
- [`docs/design/2026_10_max_allocated_bytes.md`](../design/2026_10_max_allocated_bytes.md),
  the rejected total allocation budget and the review that rejected it.
