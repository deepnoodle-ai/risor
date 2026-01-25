# Risor v2 Implementation Plan

## Overview

This plan covers three major changes for Risor v2:
1. **Module cleanup** - Remove all non-stdlib modules (keep x/crypto)
2. **TypeScript syntax alignment** - Clean break to TS-like syntax
3. **Simplification** - Remove concurrency and web server features

**Approach:** All changes in a single v2 release. Provide migration script for existing scripts.

---

## Critical Files

**Module removal:**
- `cmd/risor/options.go` - Remove module imports
- `cmd/risor/go.mod` - Remove replace directives
- `go.work` - Remove module paths
- `Makefile` - Remove build tags

**Syntax changes:**
- `token/token.go` - Token constants and keywords
- `lexer/lexer.go` - Tokenization (arrow, `?.`, `??`, template literals)
- `parser/parser.go` - Parse functions and precedence
- `ast/*.go` - AST node definitions

**Concurrency removal:**
- `object/chan.go`, `thread.go`, `spawn.go` - Delete
- `builtins/builtins.go` - Remove spawn/chan/close
- `vm/vm.go` - Remove concurrency opcodes
- `compiler/compiler.go` - Remove Go/Send/Receive compilation
- `modules/http/listen.go` - Delete

---

## Part 1: Module Removal

### Modules to KEEP (Standard Library Wrappers)
```
base64, bytes, dns, errors, exec, filepath, fmt, json, math,
net, os, rand, regexp, strconv, strings, time, http (client only)
```

### Modules to REMOVE (23 modules)
```
aws, github, slack, vault, kubernetes, redis, pgx, sql,
goquery, htmltomarkdown, jmespath, yaml, cli, color,
tablewriter, uuid, semver, shlex, sched, image, qrcode,
echarts, playwright
```

### Edge Cases
| Module | Dependency | Decision |
|--------|------------|----------|
| ssh | golang.org/x/crypto | **KEEP** - x/crypto is quasi-official |
| bcrypt | golang.org/x/crypto | **KEEP** - x/crypto is quasi-official |
| template | sprig, yaml, k8s libs | **REMOVE** - heavy external deps |
| isatty | go-isatty | **REMOVE** - external dep |
| gha | none | **REMOVE** - custom functionality |

### Implementation Order

1. **Update CLI** (`cmd/risor/options.go`)
   - Remove all non-stdlib module imports
   - Simplify `getGlobals()` function

2. **Update CLI go.mod** (`cmd/risor/go.mod`)
   - Remove replace directives for removed modules
   - Run `go mod tidy`

3. **Update Makefile**
   - Remove `GOFLAGS=-tags=aws,k8s,vault`
   - Remove `test-s3fs` and `lambda` targets

4. **Delete module directories** (28 directories in `modules/`)

5. **Delete supporting code**
   - `os/s3fs/` (AWS-dependent filesystem)
   - `examples/go/sqlite/` (uses sql module)

6. **Update go.work** - Remove ~30 module paths

7. **Run `make tidy` and `make test`**

---

## Part 2: TypeScript Syntax Alignment

### Approach: Clean Break

v2 uses TypeScript-like syntax only. A migration script (`risor migrate`) will convert v1 scripts.

### Phase 1: Core Syntax Changes

#### 1.1 Replace `var`/`:=` with `let`
- Add `LET` token, **remove** `VAR` token
- Remove `:=` declaration syntax (keep `=` for reassignment)
- `const` remains unchanged

#### 1.2 Replace `func` with `function`
- Add `FUNCTION` token, **remove** `FUNC` token
- Same semantics, different keyword

#### 1.3 Template literal syntax `` `text ${expr}` ``
- Modify `lexer/lexer.go` to recognize `${` in backticks
- **Remove** single-quote `{expr}` interpolation
- Single quotes become plain strings (like JS)

### Phase 2: Medium Priority

#### 2.1 Arrow functions `(x) => expr`
- Add `ARROW` token (`=>`)
- New AST node or flag on `Func`
- Support single expression (implicit return) and block body

#### 2.2 Optional chaining `?.`
- Add `QUESTION_DOT` token
- New `OptionalGetAttr` AST node
- Returns `nil` if LHS is nil

#### 2.3 Nullish coalescing `??`
- Add `NULLISH` token
- Use existing `Infix` AST node
- Returns RHS only if LHS is `nil`

### Phase 3: Lower Priority

#### 3.1 ES-style imports
```javascript
import { x, y } from 'module'
```
- More complex parser changes
- Keep old import syntax working

#### 3.2 Type annotations (optional)
```typescript
let x: number = 5
function add(a: number, b: number): number { }
```
- Parse but initially ignore (documentation only)
- Defer to future version

### Syntax Summary

| Feature | v1 (removed) | v2 (new) |
|---------|--------------|----------|
| Variable | `x := val` / `var x = val` | `let x = val` |
| Constant | `const x = val` | `const x = val` |
| Reassign | `x = val` | `x = val` |
| Function | `func name() { }` | `function name(p) { }` |
| Arrow | N/A | `(x) => x + 1` |
| Interpolation | `'text {x}'` | `` `text ${x}` `` |
| Plain string | N/A | `'plain'` or `"plain"` |
| Null-safe | N/A | `obj?.prop` |
| Default | N/A | `x ?? default` |
| Range loop | `for i, v := range arr` | `for (i, v in arr)` or `for i, v in arr` |
| Import | `from m import x` | `import { x } from 'm'` |

---

## Part 3: Concurrency & Advanced Feature Removal

### Features to Remove

**Concurrency:**
- `go` statement
- Channels (`chan()`, `<-` send/receive, `close()`)
- `spawn()` builtin
- Thread object type

**Web Server:**
- `http.listen_and_serve()`
- `http.listen_and_serve_tls()`
- `http.handle()`

**Configuration Options:**
- `risor.WithConcurrency()`
- `risor.WithListenersAllowed()`

### Implementation Order (safest sequence)

1. **Remove API options** (`risor_options.go`, `risor_config.go`)
2. **Remove HTTP server** (`modules/http/listen.go`)
3. **Remove builtins** (`spawn`, `chan`, `close` from `builtins/builtins.go`)
4. **Remove object types** (`object/chan.go`, `thread.go`, `spawn.go`)
5. **Remove VM support** (concurrency opcodes in `vm/vm.go`)
6. **Remove compiler support** (`compiler/compiler.go`)
7. **Remove opcodes** (leave numeric gaps in `op/op.go`)
8. **Remove AST nodes** (`ast/statements.go`, `ast/expressions.go`)
9. **Remove parser support** (`parser/parser.go`)
10. **Keep `go` as reserved keyword** (prevents future conflicts)
11. **Remove tests** (`vm_test.go`, `builtins_test.go`, etc.)
12. **Remove examples** (`channels.risor`, `spawn.risor`)

### Files to Delete
```
object/chan.go
object/thread.go
object/spawn.go
object/thread_test.go
modules/http/listen.go
modules/http/listen_test.go
examples/scripts/channels.risor
examples/scripts/spawn.risor
examples/scripts/spawn_func.risor
```

---

## Verification

After each major phase:
```bash
make test      # Run full test suite
make format    # Verify formatting
go build ./... # Verify all packages build
```

---

## Migration Script

Provide `risor migrate <file.risor>` command that transforms v1 syntax to v2:

**Transformations:**
- `var x = ...` → `let x = ...`
- `x := ...` → `let x = ...`
- `func name(...)` → `function name(...)`
- `'text {expr}'` → `` `text ${expr}` ``
- `from m import x, y` → `import { x, y } from 'm'`
- `for i, v := range arr` → `for (i, v in arr)`

**Implementation:** Can be a simple AST-to-AST transform or regex-based for initial version.

---

## Key Decisions (Resolved)

| Question | Decision |
|----------|----------|
| x/crypto modules (ssh, bcrypt) | **Keep** - quasi-official |
| Backward compatibility | **Clean break** - v2 new syntax only |
| Release scope | **All at once** - single v2 release |
| `go` keyword | Keep reserved (prevents future conflicts) |
