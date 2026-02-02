# Concurrency in Risor

## Thread Safety

Risor objects are **not thread-safe**. Do not share Risor objects across
goroutines without external synchronization.

This is the same constraint as Python (GIL) and JavaScript (single-threaded).
Each Risor VM execution should run on a single goroutine.

### What's Safe

- **Compiled bytecode** is immutable and safe to share across goroutines
- **TypeRegistry** is immutable after construction and safe to share
- **Running multiple VMs** in parallel (on different goroutines) is safe,
  as long as they don't share mutable objects

### What's Not Safe

- Sharing `*List`, `*Map`, or other mutable objects across goroutines
- Calling methods on the same object from multiple goroutines
- Modifying environment values while a VM is running

### Example: Safe Parallel Execution

```go
// Compile once, run many times in parallel
code, _ := risor.Compile(ctx, source, opts...)

var wg sync.WaitGroup
for i := 0; i < 10; i++ {
    wg.Add(1)
    go func(n int) {
        defer wg.Done()
        // Each goroutine gets its own env - no sharing
        env := risor.Builtins()
        env["n"] = n
        result, _ := risor.Run(ctx, code, risor.WithEnv(env))
        fmt.Println(result)
    }(i)
}
wg.Wait()
```

### Example: Unsafe Sharing

```go
// DON'T DO THIS
shared := object.NewList(nil)
env := risor.Builtins()
env["list"] = shared

var wg sync.WaitGroup
for i := 0; i < 10; i++ {
    wg.Add(1)
    go func() {
        defer wg.Done()
        // Multiple goroutines modifying the same list = race condition
        risor.Eval(ctx, `list.append(1)`, risor.WithEnv(env))
    }()
}
```
