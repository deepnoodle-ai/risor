# Condition Blocks: `all` and `any` for Declarative Logic

## Problem Statement

Policy rules and validation logic often involve multiple conditions that must be
combined with AND/OR. Current approaches in Risor are verbose:

```javascript
// Verbose AND chain
let allow = input.method == "GET" &&
            input.user.authenticated &&
            input.resource.public &&
            !input.user.banned

// Verbose OR chain
let allow = input.user.role == "admin" ||
            (input.method == "GET" && input.resource.public) ||
            (input.user.id == input.resource.owner)
```

This becomes hard to read as conditions grow. Compare to OPA/Rego's elegant approach:

```rego
allow if {
    input.method == "GET"
    input.user.authenticated
    input.resource.public
}
```

Each line is a condition. All must hold. The visual structure makes the logic clear.

Risor needs a way to express multi-condition logic that:
- Reads declaratively (stating what must be true)
- Scales to many conditions without becoming a wall of `&&` and `||`
- Composes naturally for complex policies

## Goals

- **Declarative style**: State conditions as facts, not computations
- **Visual clarity**: Each condition on its own line, easy to scan
- **Explicit semantics**: Clear distinction between AND and OR
- **Composable**: Nest and combine freely
- **Consistent**: Complement existing `all`/`any` functions on iterables

## Solution Overview

Add `all` and `any` block expressions:

```javascript
// all: every condition must be true (AND)
let can_access = all {
    input.user.authenticated
    input.user.active
    !input.user.banned
}

// any: at least one condition must be true (OR)
let is_admin = any {
    input.user.role == "admin"
    input.user.role == "superuser"
    "admin" in input.user.groups
}

// Compose freely
let allow = all {
    input.user.authenticated

    any {
        input.user.role == "admin"
        input.resource.public
        input.user.id == input.resource.owner
    }
}
```

## Implementation Details

### Grammar

```
all_expr = "all" "{" condition+ "}"
any_expr = "any" "{" condition+ "}"
condition = expression | all_expr | any_expr
```

### Semantics

**`all { ... }`**
- Evaluates conditions in order
- Returns `true` if every condition is truthy
- Returns `false` on first falsy condition (short-circuit)
- Empty block returns `true` (vacuous truth)

**`any { ... }`**
- Evaluates conditions in order
- Returns `true` on first truthy condition (short-circuit)
- Returns `false` if all conditions are falsy
- Empty block returns `false`

### Short-Circuit Evaluation

Like `&&` and `||`, condition blocks short-circuit:

```javascript
// Stops at first false
all {
    check_auth()      // called
    false             // returns false here
    expensive_check() // never called
}

// Stops at first true
any {
    false
    true              // returns true here
    expensive_check() // never called
}
```

### Nesting

Blocks nest naturally to express complex logic:

```javascript
let allow = all {
    // User must be valid
    input.user.authenticated
    input.user.active

    // And satisfy one of these access rules
    any {
        // Admin access
        input.user.role == "admin"

        // Owner access
        all {
            input.user.id == input.resource.owner
            input.action in ["read", "write", "delete"]
        }

        // Public read access
        all {
            input.resource.public
            input.action == "read"
        }

        // Department access
        all {
            input.user.department == input.resource.department
            input.action in ["read", "write"]
        }
    }
}
```

### Relationship to `all`/`any` Functions

Risor has `all(iterable, predicate)` and `any(iterable, predicate)` functions:

```javascript
// Existing function form
all(users, u => u.active)
any(items, i => i.price > 100)
```

The block form is complementary:

| Form | Use Case |
|------|----------|
| `all(iter, pred)` | Test predicate against collection items |
| `all { ... }` | Combine multiple independent conditions |

They can be used together:

```javascript
let valid_order = all {
    order.customer != null
    order.total > 0
    all(order.items, item => item.quantity > 0)
    any(order.items, item => item.in_stock)
}
```

### AST Nodes

```go
// AllBlock represents: all { cond1, cond2, ... }
type AllBlock struct {
    Token      token.Token
    Conditions []Expression
}

// AnyBlock represents: any { cond1, cond2, ... }
type AnyBlock struct {
    Token      token.Token
    Conditions []Expression
}
```

### Compilation

**`all { c1, c2, c3 }` compiles to:**

```
EVAL c1
JUMP_IF_FALSE END_FALSE
EVAL c2
JUMP_IF_FALSE END_FALSE
EVAL c3
JUMP_IF_FALSE END_FALSE
PUSH true
JUMP END
END_FALSE:
PUSH false
END:
```

**`any { c1, c2, c3 }` compiles to:**

```
EVAL c1
JUMP_IF_TRUE END_TRUE
EVAL c2
JUMP_IF_TRUE END_TRUE
EVAL c3
JUMP_IF_TRUE END_TRUE
PUSH false
JUMP END
END_TRUE:
PUSH true
END:
```

Semantically equivalent to chained `&&`/`||` but with cleaner syntax.

## Code Examples

### Access Control Policy

```javascript
function evaluate_access(input) {
    let {user, resource, action} = input

    // Deny conditions (check first)
    let denied = any {
        user.banned
        user.suspended
        resource.deleted
    }

    if denied {
        return {allow: false, reason: "access denied"}
    }

    // Allow conditions
    let allowed = any {
        // Admin override
        user.role == "admin"

        // Owner access
        all {
            user.id == resource.owner
            action in ["read", "write", "delete"]
        }

        // Public resources
        all {
            resource.visibility == "public"
            action == "read"
        }

        // Team access
        all {
            user.team == resource.team
            action in ["read", "write"]
            resource.visibility in ["public", "team"]
        }
    }

    return {
        allow: allowed,
        reason: allowed ? "access granted" : "default deny"
    }
}
```

### Request Validation

```javascript
function validate_request(req) {
    let valid = all {
        req.method in ["GET", "POST", "PUT", "DELETE"]
        req.path != null
        len(req.path) > 0

        // Headers validation
        all {
            req.headers != null
            req.headers["content-type"] != null
        }

        // Auth validation
        any {
            req.headers["authorization"] != null
            req.query["api_key"] != null
        }

        // Body validation for write methods
        any {
            req.method == "GET"
            req.method == "DELETE"
            all {
                req.body != null
                len(req.body) > 0
            }
        }
    }

    return valid
}
```

### Feature Flags

```javascript
function feature_enabled(feature, user, context) {
    match feature {
        "new_dashboard" => any {
            user.role == "admin"
            user.beta_tester
            context.env == "staging"
        }

        "dark_mode" => all {
            user.preferences.theme_enabled
            any {
                user.subscription == "premium"
                context.ab_test("dark_mode") == "enabled"
            }
        }

        "export_pdf" => all {
            user.subscription in ["pro", "enterprise"]
            user.verified
            context.region in ["us", "eu"]
        }

        _ => false
    }
}
```

### Data Validation

```javascript
function validate_user(user) {
    all {
        // Required fields
        user.name != null
        user.email != null

        // Field constraints
        len(user.name) >= 1
        len(user.name) <= 100
        user.email.contains("@")

        // Age validation (if provided)
        any {
            user.age == null
            all {
                user.age >= 0
                user.age <= 150
            }
        }

        // Role validation
        any {
            user.role == null
            user.role in ["user", "admin", "moderator"]
        }
    }
}
```

### Combining with `match` and `when`

```javascript
// With match guards
let result = match request {
    {method: "GET"} if all {
        request.user.authenticated
        request.resource.readable
    } => handle_get(request)

    {method: "POST"} if all {
        request.user.authenticated
        request.user.can_write
        request.body != null
    } => handle_post(request)

    _ => {status: 403, body: "forbidden"}
}

// With when
let access_level = when {
    all {
        user.role == "admin"
        user.mfa_verified
    } => "full"

    all {
        user.role in ["editor", "admin"]
        user.department == resource.department
    } => "write"

    all {
        user.authenticated
        resource.public
    } => "read"

    else => "none"
}
```

### Combining with `require`

```javascript
function process_order(input) {
    require {order, customer} from input

    // Validate order state
    require all {
        order.status == "pending"
        order.items != null
        len(order.items) > 0
        order.total > 0
    } else "invalid order state"

    // Validate customer
    require all {
        customer.active
        !customer.banned
        customer.payment_verified
    } else "customer not eligible"

    // Process...
}
```

## Rule Functions

Building on `all`/`any` blocks, Risor can support a `rule` keyword for declaring
policy functions with implicit `all` semantics. The core insight: **rules are just
functions whose body is implicitly wrapped in `all { }`**.

### The Core Idea

```javascript
rule allow(input) {
    input.user.authenticated
    input.method in ["GET", "POST"]
    input.resource.public
}
```

Is equivalent to:

```javascript
function allow(input) {
    return all {
        input.user.authenticated
        input.method in ["GET", "POST"]
        input.resource.public
    }
}
```

The `rule` keyword signals: "this is a predicate function with declarative body".

### Integration with `require`

`require` statements execute before the implicit `all` block. This separates
validation/binding from decision logic:

```javascript
rule can_access(input) {
    // Require statements run first (can error)
    require {user, resource, action} from input
    require user.authenticated else "auth required"

    // Remaining lines are wrapped in all { }
    !user.banned
    resource.status == "active"

    any {
        user.role == "admin"
        user.id == resource.owner
        all { action == "read", resource.public }
    }
}
```

Compiles to:

```javascript
function can_access(input) {
    require {user, resource, action} from input
    require user.authenticated else "auth required"

    return all {
        !user.banned
        resource.status == "active"
        any {
            user.role == "admin"
            user.id == resource.owner
            all { action == "read", resource.public }
        }
    }
}
```

### Design Decisions

| Question | Decision | Rationale |
|----------|----------|-----------|
| Where do `require` statements go? | Outside implicit `all` | Validation runs first, can error |
| Can you use `let` statements? | Yes | They don't contribute to result |
| What's the return type? | Always boolean | Rules are predicates; use functions for other types |
| Can you use `return`? | No | Keeps rules declarative; use functions if needed |

### `let` Statements in Rules

`let` statements are allowed for intermediate values but don't contribute to
the boolean result:

```javascript
rule can_access(input) {
    require {user, resource} from input

    let min_role = resource.min_role ?? "user"
    let role_level = ROLE_LEVELS[user.role] ?? 0
    let required_level = ROLE_LEVELS[min_role] ?? 0

    user.authenticated
    !user.banned
    role_level >= required_level
}
```

### Nesting `any` and `all`

Since the body is implicitly `all`, use explicit `any` for OR logic:

```javascript
rule allow(input) {
    require {user, resource, action} from input

    user.authenticated
    !user.banned

    // Explicit any for OR conditions
    any {
        user.role == "admin"

        all {
            user.id == resource.owner
            action in ["read", "write", "delete"]
        }

        all {
            resource.public
            action == "read"
        }
    }
}
```

### Grammar

```
rule_decl = "rule" IDENT "(" params? ")" "{" rule_body "}"
rule_body = (require_stmt | let_stmt | condition)*
condition = expression | all_expr | any_expr
```

### AST Node

```go
// RuleDeclaration represents: rule name(params) { body }
type RuleDeclaration struct {
    Token      token.Token
    Name       *Identifier
    Parameters []*Identifier
    Body       *RuleBody
}

// RuleBody contains requires, lets, and conditions
type RuleBody struct {
    Requires   []*RequireStatement
    Statements []Statement      // let statements
    Conditions []Expression     // wrapped in implicit all
}
```

### Compilation

A rule declaration compiles to a function that:
1. Executes require statements (may error)
2. Executes let statements
3. Evaluates conditions as `all { ... }` block
4. Returns the boolean result

```
DEFINE_FUNCTION name, params
  ; require statements
  EVAL require1
  EVAL require2
  ; let statements
  EVAL let1
  STORE var1
  ; implicit all block
  EVAL cond1
  JUMP_IF_FALSE END_FALSE
  EVAL cond2
  JUMP_IF_FALSE END_FALSE
  ...
  PUSH true
  JUMP END
  END_FALSE:
  PUSH false
  END:
  RETURN
```

### Complete Example

```javascript
// Define rules
rule is_admin(user) {
    user.role == "admin"
    user.verified
    user.mfa_enabled
}

rule can_read(input) {
    require {user, resource} from input

    !user.banned

    any {
        is_admin(user)
        resource.public
        user.id == resource.owner
        user.team == resource.team
    }
}

rule can_write(input) {
    require {user, resource, action} from input

    can_read(input)
    action in ["create", "update", "delete"]

    any {
        is_admin(user)
        user.id == resource.owner
    }
}

// Use rules
let read_allowed = can_read({user: current_user, resource: doc})
let write_allowed = can_write({user: current_user, resource: doc, action: "update"})
```

### Why Not Just Use Functions?

You can always write:

```javascript
function allow(input) {
    require {user, resource} from input
    return all {
        user.authenticated
        !user.banned
    }
}
```

The `rule` keyword provides:

1. **Signal**: Clearly marks predicate functions
2. **Concision**: No `return all { }` boilerplate
3. **Consistency**: All rules have the same structure
4. **Tooling**: LSP/linters can treat rules specially

For non-boolean return values or complex control flow, use regular functions.

## Testing

### Parser Tests

```go
func TestParseAllBlock(t *testing.T) {
    tests := []struct {
        input      string
        conditions int
    }{
        {`all { a }`, 1},
        {`all { a, b }`, 2},
        {`all { a, b, c }`, 3},
        {`all { a, all { b, c } }`, 2}, // nested
    }
    // ...
}

func TestParseAnyBlock(t *testing.T) {
    tests := []struct {
        input      string
        conditions int
    }{
        {`any { a }`, 1},
        {`any { a, b }`, 2},
        {`any { a, any { b, c } }`, 2}, // nested
    }
    // ...
}
```

### Evaluation Tests

```go
func TestAllBlockEvaluation(t *testing.T) {
    tests := []struct {
        input string
        want  bool
    }{
        {`all { true }`, true},
        {`all { true, true }`, true},
        {`all { true, false }`, false},
        {`all { false, true }`, false},
        {`all { }`, true}, // vacuous truth
    }
    // ...
}

func TestAnyBlockEvaluation(t *testing.T) {
    tests := []struct {
        input string
        want  bool
    }{
        {`any { true }`, true},
        {`any { false }`, false},
        {`any { true, false }`, true},
        {`any { false, true }`, true},
        {`any { false, false }`, false},
        {`any { }`, false},
    }
    // ...
}

func TestNestedBlocks(t *testing.T) {
    tests := []struct {
        input string
        want  bool
    }{
        {`all { true, any { false, true } }`, true},
        {`all { true, any { false, false } }`, false},
        {`any { false, all { true, true } }`, true},
        {`any { all { false }, all { true } }`, true},
    }
    // ...
}
```

### Rule Tests

```go
func TestParseRule(t *testing.T) {
    tests := []struct {
        input      string
        name       string
        params     int
        conditions int
    }{
        {`rule allow() { true }`, "allow", 0, 1},
        {`rule allow(x) { x > 0 }`, "allow", 1, 1},
        {`rule allow(x, y) { x, y }`, "allow", 2, 2},
    }
    // ...
}

func TestRuleWithRequire(t *testing.T) {
    input := `
        rule allow(input) {
            require {user} from input
            user.active
        }
        allow({user: {active: true}})
    `
    result := eval(input)
    assert.Equal(t, true, result)
}

func TestRuleWithLet(t *testing.T) {
    input := `
        rule check(x) {
            let threshold = 10
            x > threshold
        }
        check(15)
    `
    result := eval(input)
    assert.Equal(t, true, result)
}

func TestRuleImplicitAll(t *testing.T) {
    tests := []struct {
        input string
        want  bool
    }{
        {`rule r() { true, true }; r()`, true},
        {`rule r() { true, false }; r()`, false},
        {`rule r() { false, true }; r()`, false},
        {`rule r(x) { x > 0, x < 10 }; r(5)`, true},
        {`rule r(x) { x > 0, x < 10 }; r(15)`, false},
    }
    // ...
}

func TestRuleWithNestedAny(t *testing.T) {
    input := `
        rule allow(user) {
            user.active
            any {
                user.role == "admin"
                user.role == "editor"
            }
        }
        allow({active: true, role: "editor"})
    `
    result := eval(input)
    assert.Equal(t, true, result)
}
```

### Short-Circuit Tests

```go
func TestAllBlockShortCircuit(t *testing.T) {
    // Verify second condition not evaluated when first is false
    input := `
        let called = false
        all {
            false
            called = true  // should not execute
        }
        called
    `
    result := eval(input)
    assert.Equal(t, false, result)
}

func TestAnyBlockShortCircuit(t *testing.T) {
    // Verify second condition not evaluated when first is true
    input := `
        let called = false
        any {
            true
            called = true  // should not execute
        }
        called
    `
    result := eval(input)
    assert.Equal(t, false, result)
}
```

## Implementation Phases

### Phase 1: Basic Blocks

- Add `all` and `any` as contextual keywords (when followed by `{`)
- Parse block expressions
- Compile to conditional jumps
- Basic evaluation tests

### Phase 2: Nesting and Composition

- Support nested blocks
- Integration with existing expressions
- Comprehensive test coverage

### Phase 3: Integration

- Integration with `match` guards
- Integration with `when` conditions
- Integration with `require`
- Documentation and examples

### Phase 4: Rule Functions

- Add `rule` keyword
- Parse rule declarations with implicit `all` semantics
- Handle `require` statements as preamble (outside implicit `all`)
- Allow `let` statements interleaved with conditions
- Compile rules to functions
- Rule-specific error messages

## Alternatives Considered

### 1. Rego-Style Implicit OR via Multiple Definitions

```javascript
rule allow {
    input.user.role == "admin"
}

rule allow {
    input.method == "GET"
    input.resource.public
}
```

Rejected because:
- Multiple definitions merging is magical/implicit
- Conflicts with "one way to do things"
- Harder to reason about when rules are scattered

### 2. Extended `if` Blocks

```javascript
let allow = if {
    input.method == "GET"
    input.user.authenticated
}
```

Rejected because:
- Overloads `if` with new semantics
- Unclear what happens on false (return false? null?)
- `all` is more explicit about intent

### 3. `and`/`or` Keywords Instead of `all`/`any`

```javascript
let allow = and {
    condition1
    condition2
}
```

Rejected because:
- `all`/`any` already exist as functions, this extends them consistently
- `and`/`or` could be confused with operators
- `all`/`any` read better: "all of these must be true"

### 4. List Syntax with Implicit AND

```javascript
let allow = check [
    condition1,
    condition2,
    condition3
]
```

Rejected because:
- Unclear semantics (AND or OR?)
- Doesn't compose as naturally
- New syntax for unclear benefit

### 5. Arrow Function Blocks

```javascript
let allow = all(() => {
    condition1
    condition2
})
```

Rejected because:
- Verbose
- Unclear semantics (what does the function return?)
- Doesn't read as declaratively

## Open Questions

1. **Empty blocks**: Should `all { }` return `true` (vacuous truth) and `any { }`
   return `false`? Current lean: yes, matches logical conventions.

2. **Single condition**: Should `all { x }` be allowed or require 2+ conditions?
   Current lean: allow single condition for consistency, even if `x` alone suffices.

3. **Newlines vs commas**: Should conditions be separated by newlines, commas, or
   either? Current lean: either (like existing block statements).

4. **Side effects**: Should conditions be allowed to have side effects? Current
   lean: yes, but discouraged. Same rules as `&&`/`||`.

5. **Truthiness**: Should conditions use Risor's standard truthiness rules?
   Current lean: yes, for consistency.

6. **Keyword collision**: `all` and `any` are existing function names. Using them
   as block keywords when followed by `{` should be unambiguous. Verify no parsing
   conflicts.

7. **Rule keyword**: Should `rule` be a hard keyword or contextual? Current lean:
   contextual (only keyword when followed by identifier and `(`).

8. **Rules as values**: Should rules be first-class values (assignable, passable)?
   Current lean: yes, they're just functions with special declaration syntax.

9. **Rule composition**: Should there be syntax for combining rules (e.g.,
   `rule allow = can_read || can_write`)? Current lean: no, use explicit
   `any { can_read(x), can_write(x) }` in a wrapper rule or function.

## Relationship to Other Proposals

| Construct | Purpose |
|-----------|---------|
| `all { }` / `any { }` | Combine multiple boolean conditions |
| `rule` | Predicate functions with implicit `all` body |
| `match` | Branch on value patterns |
| `when` | Branch on conditions |
| `require` | Assert conditions, fail if false |

They compose naturally:

```javascript
// require with all
require all {
    input.user.authenticated
    input.resource.accessible
} else "access denied"

// when with all/any in conditions
let level = when {
    all { user.admin, user.verified } => "full"
    any { resource.public, user.owner } => "read"
    else => "none"
}

// match with all/any in guards
match request {
    {method: "POST"} if all { user.auth, body.valid } => process(body)
    _ => error("invalid")
}
```

## Conclusion

`all` and `any` blocks bring declarative, readable condition logic to Risor. They
solve a real problem — complex AND/OR chains become unreadable — with minimal syntax
addition. The design:

- **Extends consistently**: Complements existing `all`/`any` functions
- **Composes naturally**: Nests and combines with other constructs
- **Reads declaratively**: Each line states a condition, intent is clear
- **Stays explicit**: No magic merging or implicit behavior

The `rule` keyword builds on this foundation, providing a concise way to declare
predicate functions with implicit `all` semantics. Rules integrate naturally with
`require` for input validation — requires run first as a preamble, then remaining
conditions are evaluated as an `all` block.

Together, `all`/`any` blocks and `rule` functions give Risor the expressive power
of Rego's condition blocks while maintaining explicit semantics aligned with Risor's
principles. The key insight — that rules are just functions with special body
semantics — keeps the design minimal and composable.
