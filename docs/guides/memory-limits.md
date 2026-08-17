# Limiting Memory in Untrusted Scripts

Risor bounds instruction count (`WithMaxSteps`) and wall-clock time
(`WithTimeout`). It does not bound allocation, and it has no `WithMaxMemory`.
A single instruction can allocate gigabytes:

```go
// runs fine, returns a result, allocates 5.4 GB
risor.Eval(ctx, `len(list(range(50000000)))`,
    risor.WithMaxSteps(1000),
    risor.WithEnv(risor.Builtins()))
```

That is not a problem when you trust the script. It matters when you don't, and
this guide is for that case: you want to run scripts you didn't write, in your
own process, without letting one of them take the process down.

## Start here: bound it outside the process

**The only way to actually guarantee a memory bound today is to put the script
inside something the operating system can kill.** Nothing you build with
Risor's API gives you that guarantee, including everything in this guide.
Reach for an external boundary first, and treat the in-process techniques below
as a second layer rather than a substitute.

The options, roughly in order of how much work they are:

- **AWS Lambda**, or an equivalent per-invocation runtime on any cloud. If
  you're already deployed in a cloud this is usually the least effort for the
  most safety. You get a hard memory ceiling per invocation, a hard timeout,
  and a fresh process each time, so a script that misbehaves takes down its own
  invocation and nothing else. Configure the function's memory to what your
  scripts legitimately need and let the platform enforce it.
- **A container with a memory limit.** `--memory` on Docker, or `resources.limits.memory`
  on Kubernetes. The kernel OOM-kills the container rather than your host.
- **A subprocess with an rlimit.** `RLIMIT_AS` or `RLIMIT_DATA` via
  `syscall.Setrlimit` before `exec`. Cheapest to adopt if you're already on a
  single box, and it keeps the blast radius to one child process.

All three degrade safely when you get something wrong, which is the property
that matters. The in-process approach does not: when it fails, it fails by
taking down the process that was supposed to be doing the limiting.

## Read this first, if you're staying in-process

**What you can build with today's API is real but incomplete.** It gives you a
hard cap on the operations a script is most likely to abuse. It does not give
you a guarantee that the process cannot be pushed out of memory. The gaps are
specific and listed under [What this does not cover](#what-this-does-not-cover).
Read that section before you rely on any of this.

The rest of this guide is worth your time when a script exceeding its budget is
a bug to be reported rather than a security incident, or when you want a fast
in-process check in front of an external boundary so that ordinary abuse fails
cleanly instead of costing you a killed container.

## The idea

Risor v2 has no loop keywords. There is no `for`, no `while`, no `for-in`.
Iteration exists only through callbacks (`.map`, `.each`, `.filter`,
`.reduce`) and through recursion, and both of those need a function definition.

So if you disallow function definitions, every script runs top to bottom
exactly once. Its instruction count is bounded by the length of its source, and
its total allocation is the sum over a fixed number of operations. At that
point, capping each individual operation caps the whole program.

That is what makes the rest of this guide work. Without it you are trying to
bound an unbounded number of operations, which the current API cannot do.

Three layers, each useful on its own:

1. Start from an empty environment and add back only what you need.
2. Turn off the syntax you don't need, starting with function definitions.
3. Route `+` and method calls through a budget that checks sizes before
   allocating.

## Layer 1: start from an empty environment

The environment is empty by default. `risor.Builtins()` is opt-in, so the first
and cheapest thing you can do is not call it.

```go
all := risor.Builtins()
env := map[string]any{
    "len":     all["len"],
    "string":  all["string"],
    "int":     all["int"],
    "float":   all["float"],
    "bool":    all["bool"],
    "type":    all["type"],
    "sprintf": all["sprintf"],
}
risor.Eval(ctx, src, risor.WithEnv(env))
```

Leaving out `list`, `range`, `sorted`, `chunk` and `encode` removes the worst
cases outright. `list(range(N))` is the single most expensive thing a script
can do, because every element of a list is an interface header plus a boxed
value, roughly 100× the per-element cost of a string.

One thing this does **not** do: it does not remove type methods.
`"a".repeat(1000000000)` still works no matter what is in the environment,
because methods resolve through `GetAttr` on the object rather than through the
environment. Layer 3 is what handles those.

## Layer 2: turn off syntax you don't need

```go
syntax := risor.SyntaxConfig{
    DisallowFuncDef:   true,  // the important one: no callbacks, no recursion
    DisallowReturn:    true,
    DisallowSpread:    true,  // [...a, ...a] grows a list with no call to intercept
    DisallowTemplates: true,  // `${a}${a}` concatenates with no call to intercept
    DisallowPipe:      true,
    DisallowTryCatch:  true,
}
risor.Eval(ctx, src, risor.WithEnv(env), risor.WithSyntax(syntax))
```

`DisallowFuncDef` is the one that matters most, for the reason above.

`DisallowSpread` and `DisallowTemplates` matter because those two constructs
allocate inside VM instructions, with no function call anywhere for Layer 3 to
wrap. Turning them off in the grammar is the only way to bound them from
outside the library. Without `DisallowSpread`, six lines of nested spreads
allocate about 1 GB.

Don't reach for the presets here. `ExpressionOnly` looks like it should help
and doesn't: it still allows operators and calls, so `"aaaa".repeat(n) + "b"`
passes straight through. It restricts grammar, not cost.

## Layer 3: budget the operations that remain

`WithTransform` runs between parsing and compiling, and `pkg/ast` is fully
exported with settable fields. That is enough to rewrite `a + b` into
`__add(a, b)` and `x.m(args...)` into `__method(x, "m", args...)`, where both
guards are functions you control.

The pieces below concatenate into a single file.

### The budget

Charge **before** allocating. Charging afterwards tells you a script went over,
but the memory has already been handed to the runtime, and one operation is
enough to get the process killed.

```go
// Budget is a memory allowance for a single script execution. It is not safe
// for concurrent use; give each execution its own.
type Budget struct {
    limit int64
    used  int64
}

func NewBudget(limit int64) *Budget { return &Budget{limit: limit} }

func (b *Budget) Reserve(n int64) error {
    if n < 0 || b.used+n > b.limit {
        return fmt.Errorf("memory limit exceeded: %d bytes requested, %d of %d used",
            n, b.used, b.limit)
    }
    b.used += n
    return nil
}
```

### Measuring a value

`Sizeof` needs caps on how much work it will do. List literals can share
structure, so `a = [a, a]` repeated thirty times builds a tree of 2^30 elements
in almost no memory. A naive recursive size function walks all of them and
hangs, which turns your safeguard into the vulnerability.

```go
const (
    maxVisits = 10_000 // nodes Sizeof will look at before giving up
    maxDepth  = 32     // how deep it will nest
)

// ErrTooLarge means Sizeof gave up. Treat it as "does not fit".
var ErrTooLarge = errors.New("value too large or too deeply nested to measure")

func Sizeof(o object.Object) (int64, error) {
    visits := maxVisits
    return sizeof(o, 0, &visits)
}

func sizeof(o object.Object, depth int, visits *int) (int64, error) {
    if depth > maxDepth || *visits <= 0 {
        return 0, ErrTooLarge
    }
    *visits--

    switch o := o.(type) {
    case *object.String:
        return int64(len(o.Value())) + 16, nil
    case *object.Bytes:
        return int64(len(o.Value())) + 24, nil
    case *object.List:
        total := int64(24)
        for _, item := range o.Value() {
            n, err := sizeof(item, depth+1, visits)
            if err != nil {
                return 0, err
            }
            total += 16 + n
        }
        return total, nil
    case *object.Map:
        total := int64(48)
        for k, v := range o.Value() {
            n, err := sizeof(v, depth+1, visits)
            if err != nil {
                return 0, err
            }
            total += int64(len(k)) + 32 + n
        }
        return total, nil
    default:
        return 16, nil // scalars: close enough
    }
}
```

These are estimates, not exact figures. That is fine for a budget, as long as
you set the limit with headroom.

### The transform

Note that method calls parse as `ast.ObjectCall`, not as
`ast.Call{Fun: *ast.GetAttr}`. Assuming the latter is an easy way to write a
transform that silently misses every method call.

```go
func Transform(program *ast.Program) (*ast.Program, error) {
    rewrite(program)
    return program, nil
}

func rewrite(n ast.Node) ast.Node {
    if n == nil {
        return nil
    }
    v := reflect.ValueOf(n)
    if v.Kind() != reflect.Ptr || v.IsNil() || v.Elem().Kind() != reflect.Struct {
        return n
    }

    // Recurse into every exported field that can hold a node.
    s := v.Elem()
    for i := 0; i < s.NumField(); i++ {
        f := s.Field(i)
        if !f.CanSet() {
            continue
        }
        switch f.Kind() {
        case reflect.Interface:
            if !f.IsNil() {
                if child, ok := f.Interface().(ast.Node); ok {
                    replaceIn(f, rewrite(child))
                }
            }
        case reflect.Ptr:
            if !f.IsNil() {
                if child, ok := f.Interface().(ast.Node); ok {
                    rewrite(child)
                }
            }
        case reflect.Slice:
            for j := 0; j < f.Len(); j++ {
                e := f.Index(j)
                if e.Kind() == reflect.Interface && !e.IsNil() {
                    if child, ok := e.Interface().(ast.Node); ok {
                        replaceIn(e, rewrite(child))
                    }
                } else if e.Kind() == reflect.Ptr && !e.IsNil() {
                    if child, ok := e.Interface().(ast.Node); ok {
                        rewrite(child)
                    }
                }
            }
        }
    }

    // Then rewrite this node itself.
    switch x := n.(type) {
    case *ast.Infix:
        if x.Op == "+" {
            return &ast.Call{
                Fun:    &ast.Ident{NamePos: x.OpPos, Name: "__add"},
                Lparen: x.OpPos,
                Args:   []ast.Node{x.X, x.Y},
                Rparen: x.OpPos,
            }
        }
    case *ast.ObjectCall:
        name, ok := x.Call.Fun.(*ast.Ident)
        if !ok {
            return n
        }
        args := []ast.Node{x.X, &ast.String{Value: name.Name}}
        args = append(args, x.Call.Args...)
        return &ast.Call{
            Fun:    &ast.Ident{NamePos: x.Period, Name: "__method"},
            Lparen: x.Period,
            Args:   args,
            Rparen: x.Call.Rparen,
        }
    }
    return n
}

func replaceIn(field reflect.Value, replacement ast.Node) {
    if replacement == nil {
        return
    }
    if rv := reflect.TypeOf(replacement); rv.AssignableTo(field.Type()) {
        field.Set(reflect.ValueOf(replacement))
    }
}
```

### The guards

`__add` checks operand sizes before the concatenation happens. `__method`
handles all type methods through one wrapper, because `GetAttr` returns a
`*Builtin` that already satisfies `object.Callable`.

Methods whose output size is a known function of their inputs get checked up
front. `repeat` is the one that matters. Everything else charges afterwards,
which is acceptable only because its inputs are already inside the budget.

```go
func Guards(b *Budget) map[string]any {
    return map[string]any{
        "__add":    object.NewBuiltin("__add", guardAdd(b)),
        "__method": object.NewBuiltin("__method", guardMethod(b)),
    }
}

func guardAdd(b *Budget) func(context.Context, ...object.Object) (object.Object, error) {
    return func(ctx context.Context, args ...object.Object) (object.Object, error) {
        if len(args) != 2 {
            return nil, errors.New("__add: expected 2 arguments")
        }
        left, right := args[0], args[1]

        // Only the growable types need checking; int + int cannot run away.
        switch left.(type) {
        case *object.String, *object.Bytes, *object.List:
            ln, err := Sizeof(left)
            if err != nil {
                return nil, err
            }
            rn, err := Sizeof(right)
            if err != nil {
                return nil, err
            }
            if err := b.Reserve(ln + rn); err != nil {
                return nil, err
            }
        }
        return object.BinaryOp(op.Add, left, right)
    }
}

func guardMethod(b *Budget) func(context.Context, ...object.Object) (object.Object, error) {
    return func(ctx context.Context, args ...object.Object) (object.Object, error) {
        if len(args) < 2 {
            return nil, errors.New("__method: expected a receiver and a method name")
        }
        self := args[0]
        name, ok := args[1].(*object.String)
        if !ok {
            return nil, errors.New("__method: method name must be a string")
        }
        rest := args[2:]

        // repeat multiplies its receiver, so its output size is known up front.
        checked := false
        if name.Value() == "repeat" && len(rest) == 1 {
            if count, ok := rest[0].(*object.Int); ok {
                n, err := Sizeof(self)
                if err != nil {
                    return nil, err
                }
                if err := b.Reserve(n * count.Value()); err != nil {
                    return nil, err
                }
                checked = true
            }
        }

        attr, found := self.GetAttr(name.Value())
        if !found {
            return nil, fmt.Errorf("attribute %q not found on %s", name.Value(), self.Type())
        }
        fn, ok := attr.(object.Callable)
        if !ok {
            return nil, fmt.Errorf("attribute %q is not callable", name.Value())
        }

        result, err := fn.Call(ctx, rest...)
        if err != nil {
            return nil, err
        }
        if checked {
            return result, nil
        }
        n, err := Sizeof(result)
        if err != nil {
            return nil, err
        }
        return result, b.Reserve(n)
    }
}
```

### Wrapping your own builtins

The transform rewrites operators and method calls. It does **not** rewrite
plain function calls, so anything you put in the environment needs wrapping
yourself.

```go
func Wrap(b *Budget, name string, fn object.Callable) object.Object {
    return object.NewBuiltin(name, func(ctx context.Context, args ...object.Object) (object.Object, error) {
        result, err := fn.Call(ctx, args...)
        if err != nil {
            return nil, err
        }
        n, err := Sizeof(result)
        if err != nil {
            return nil, err
        }
        return result, b.Reserve(n)
    })
}
```

## Putting it together

```go
func Eval(ctx context.Context, src string, env map[string]any, limit int64) (any, error) {
    b := NewBudget(limit)

    full := make(map[string]any, len(env)+2)
    for k, v := range env {
        if fn, ok := v.(object.Callable); ok {
            full[k] = Wrap(b, k, fn)
            continue
        }
        full[k] = v
    }
    for k, v := range Guards(b) {
        full[k] = v
    }

    return risor.Eval(ctx, src,
        risor.WithEnv(full),
        risor.WithSyntax(risor.SyntaxConfig{
            DisallowFuncDef:   true,
            DisallowReturn:    true,
            DisallowSpread:    true,
            DisallowTemplates: true,
            DisallowPipe:      true,
            DisallowTryCatch:  true,
        }),
        risor.WithTransform(risor.TransformerFunc(Transform)),
        risor.WithMaxSteps(10_000),
    )
}
```

Imports: `context`, `errors`, `fmt`, `reflect`, plus `risor`, `pkg/ast`,
`pkg/object` and `pkg/op`.

### What this measures

Against a 10 MB budget, with the environment from Layer 1:

| Script | Peak allocation | Result |
| --- | ---: | --- |
| `code + ": " + street + " " + num` | 0 MB | `DE: Hauptstrasse 42` |
| `"  Germany  ".trim_space().to_upper()` | 0 MB | `GERMANY` |
| `"a,b,c".split(",")[1]` | 0 MB | `b` |
| Chained `+` doubling ten times per line | 8 MB | refused at the budget |
| `len("a".repeat(1000000000))` | 0 MB | refused before allocating |
| `[...a, ...a, ...a]` | 0 MB | rejected at validation |
| `` len(`${a}${a}`) `` | 0 MB | rejected at validation |
| `[1,2,3].map(x => x * 2)` | 0 MB | rejected at validation |

Without any of this, the chained `+` case allocates 572 MB and the `repeat`
case about 950 MB.

Good numbers, and still not a guarantee. Everything in that table is a case
somebody thought to test. The section below is the list of cases where this
approach is known to fall short, which is why it belongs behind an external
boundary rather than in place of one.

## What this does not cover

Set expectations accordingly. These are the gaps, and none of them can be
closed from outside the library. Each is a reason to keep the container,
Lambda, or rlimit from the top of this guide in place.

**Shared list structure.** `a = [a, a]` repeated thirty times is a few lines of
source, allocates almost nothing, and reports `len(a) == 2`. The list literal
is built inside a VM instruction, so there is no call for the transform to
wrap. It stays harmless until something expands it, which is why you should not
put `string`, `sorted`, `encode` or `chunk` over lists into the environment. If
you need `string()` on arbitrary values, this approach is not enough for you.

**Methods that charge after the fact.** Only `repeat` has its output size
checked before the work happens. Any method whose output size isn't a cheap
function of its inputs allocates first and charges second. In practice its
inputs are already inside the budget, so this costs you a constant factor
rather than leaving an unbounded hole, but it does mean the peak is higher than
the number you set.

**`Sizeof` is an estimate.** It ignores Go's allocator overhead, slice capacity
beyond length, and map bucket overhead. Set your limit well below the memory
you can actually afford to lose.

**`WithTimeout` is not a hard bound.** Independently of memory, a builtin whose
runtime grows faster than its input can run past the deadline. The VM sets a
halt flag from a goroutine but only reads it between instructions, so Go code
running inside a builtin cannot be interrupted. `string()` over the shared
structure above will run for minutes under a two-second timeout.

**The transform is coupled to AST node shapes.** If a node type is added to
`pkg/ast` in a future release, the rewrite may silently stop covering some
construct. Write a test that asserts your guards actually fire on every
construct you care about, and re-run it when you upgrade.

**Nothing here bounds the Go side.** Any function you expose can allocate
whatever it likes before your wrapper ever sees the result. Wrapping charges
for what comes back, not for what happened inside.

## Summary

The two layers do different jobs, and you want both.

An **external boundary** is the only thing that actually guarantees a bound. It
covers the gaps above and the ones nobody has found yet, because it doesn't
depend on anyone having enumerated them. If you're in a cloud already, AWS
Lambda or an equivalent per-invocation runtime is usually the cheapest way to
get there. Otherwise, a container memory limit or a subprocess with `RLIMIT_AS`.

The **in-process budget** in this guide gives you a fast, cheap failure for
ordinary abuse, with a clear error you can log and hand back to whoever wrote
the script. That's worth having in front of the boundary, since a refused
operation is much nicer to operate than a killed container. It is not worth
having instead of one.

## See also

- [`docs/reviews/2026_08_memory_bounding.md`](../reviews/2026_08_memory_bounding.md)
  for the measurements behind this guide and what it would take to support
  memory limits in the library itself.
