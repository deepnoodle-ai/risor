# Match Expressions for Pattern-Based Control Flow

## Problem Statement

Risor currently supports conditional logic through `if` expressions. While these work
well for simple conditions, they become unwieldy when:

1. **Multiple conditions on the same value** — Requires repeated `if/else if` chains
2. **Structural matching** — Testing object shapes requires manual property access
3. **Combined destructuring and conditions** — No way to bind values while testing predicates

Consider a policy evaluation scenario:

```javascript
// Current approach: verbose and repetitive
let access = "none"
if user.role == "admin" {
    access = "full"
} else if user.role == "editor" && resource.draft {
    access = "edit"
} else if user.id == resource.owner {
    access = "full"
} else if resource.public {
    access = "read"
}
```

This pattern is common in policy engines, configuration logic, and data transformation —
all core Risor use cases. A more declarative approach would improve clarity.

## Goals

- **Declarative conditionals**: Express multi-branch logic as a single expression
- **Pattern matching**: Match on object structure, not just values
- **Guards**: Add predicates to pattern arms for fine-grained control
- **Exhaustiveness**: Require a default case to ensure all inputs are handled
- **Familiar syntax**: Draw from established patterns (Rust, Swift, Kotlin)

## Solution Overview

Add two complementary expression forms:

### 1. `when` Expression (Condition-Based)

For multi-branch conditionals without destructuring:

```javascript
let access = when {
    user.role == "admin" => "full"
    user.role == "editor" && resource.draft => "edit"
    user.id == resource.owner => "full"
    resource.public => "read"
    else => "none"
}
```

### 2. `match` Expression (Pattern-Based)

For structural matching with optional destructuring and guards:

```javascript
let result = match request {
    {method: "GET", path: "/health"} => {status: 200, body: "ok"}
    {method: "GET", auth: {role: "admin"}} => handle_admin_get(request)
    {method: "POST"} if request.body != null => handle_post(request)
    _ => {status: 404, body: "not found"}
}
```

## Implementation Details

### `when` Expression

The `when` expression evaluates conditions in order and returns the first matching result.

**Grammar:**

```
when_expr     = "when" "{" when_arm+ else_arm "}"
when_arm      = expression "=>" expression
else_arm      = "else" "=>" expression
```

**Semantics:**

1. Each condition is evaluated in order (short-circuit evaluation)
2. When a condition is truthy, its result expression is evaluated and returned
3. The `else` arm is required and handles unmatched cases
4. The entire `when` is an expression that produces a value

**Examples:**

```javascript
// Simple value selection
let tier = when {
    score >= 90 => "gold"
    score >= 70 => "silver"
    score >= 50 => "bronze"
    else => "none"
}

// With complex conditions
let action = when {
    user.banned => "deny"
    !user.verified && resource.sensitive => "deny"
    user.role in ["admin", "moderator"] => "allow"
    resource.public => "allow"
    else => "deny"
}

// Nested in other expressions
let message = "Access: " + when {
    allowed => "granted"
    else => "denied"
}
```

### `match` Expression

The `match` expression tests a value against patterns, optionally binding variables
and applying guard conditions.

**Grammar:**

```
match_expr    = "match" expression "{" match_arm+ default_arm "}"
match_arm     = pattern guard? "=>" expression
default_arm   = "_" "=>" expression
guard         = "if" expression
pattern       = literal | identifier | object_pattern | list_pattern | "_"
object_pattern = "{" (key_pattern ("," key_pattern)*)? "}"
key_pattern   = identifier (":" pattern)?
list_pattern  = "[" (pattern ("," pattern)*)? spread? "]"
spread        = "..." identifier?
```

**Pattern Types:**

| Pattern | Matches | Binds |
|---------|---------|-------|
| `42` | Exact value 42 | Nothing |
| `"hello"` | Exact string "hello" | Nothing |
| `true` / `false` | Boolean values | Nothing |
| `null` | Null value | Nothing |
| `x` | Any value | Value to `x` |
| `_` | Any value (wildcard) | Nothing |
| `{a, b}` | Object with keys `a` and `b` | Values to `a` and `b` |
| `{a: x}` | Object with key `a` | Value of `a` to `x` |
| `{type: "user"}` | Object where `type == "user"` | Nothing |
| `[a, b]` | List with exactly 2 elements | Elements to `a` and `b` |
| `[head, ...tail]` | List with 1+ elements | First to `head`, rest to `tail` |
| `[...items]` | Any list | All elements to `items` |

**Semantics:**

1. The match subject is evaluated once
2. Patterns are tested in order
3. When a pattern matches, bound variables are available in the guard and result
4. If a guard is present, it must be truthy for the arm to match
5. The first matching arm's result is returned
6. A default arm (`_`) is required

**Examples:**

```javascript
// Structural matching
let response = match event {
    {type: "click", target: {id}} => handle_click(id)
    {type: "keydown", key: "Enter"} => submit_form()
    {type: "keydown", key} => handle_key(key)
    _ => null
}

// With guards
let access = match user {
    {role: "admin"} => "full"
    {role: "editor", department} if department == resource.department => "edit"
    {verified: true} if resource.public => "read"
    _ => "none"
}

// List patterns
let description = match items {
    [] => "empty"
    [only] => "single item: " + string(only)
    [first, second] => "pair: " + string(first) + ", " + string(second)
    [head, ...rest] => "list starting with " + string(head)
    _ => "unknown"
}

// Nested patterns
let result = match data {
    {user: {name, address: {city}}} => name + " from " + city
    {user: {name}} => name
    _ => "anonymous"
}

// Combining with other expressions
servers.filter(s => match s {
    {status: "healthy", load} if load < 0.8 => true
    _ => false
})
```

### Guard Expressions

Guards add conditional logic to pattern arms:

```javascript
match request {
    {method: "POST", body} if len(body) > MAX_SIZE => error("too large")
    {method: "POST", body} if body.valid => process(body)
    {method: "POST"} => error("invalid body")
    _ => error("unsupported")
}
```

**Guard semantics:**
- Guards are evaluated after pattern matching succeeds
- Bound variables from the pattern are available in the guard
- If the guard is falsy, matching continues to the next arm
- Guards must be pure expressions (no side effects)

### Exhaustiveness

Both `when` and `match` require a default case:

```javascript
// when requires else
let x = when {
    a => 1
    // Error: missing else arm
}

// match requires _ (wildcard)
let y = match value {
    1 => "one"
    // Error: missing default arm
}
```

This ensures all inputs produce a defined result — critical for policy evaluation
where "undefined" behavior is unacceptable.

### Compilation

**`when` compiles to:**

```
// when { a => x, b => y, else => z }
EVAL a
JUMP_IF_FALSE L1
EVAL x
JUMP END
L1: EVAL b
JUMP_IF_FALSE L2
EVAL y
JUMP END
L2: EVAL z
END:
```

**`match` compiles to:**

Pattern matching compiles to a series of type checks, property accesses, and
comparisons. The compiler generates efficient code that:

1. Evaluates the subject once, storing in a temporary
2. For each arm, generates pattern-matching code
3. On match, binds variables and evaluates guard (if present)
4. On success, evaluates result; on failure, jumps to next arm

### New AST Nodes

```go
// WhenExpression represents: when { cond => result, ... else => default }
type WhenExpression struct {
    Token    token.Token
    Arms     []*WhenArm
    Default  Expression
}

type WhenArm struct {
    Condition Expression
    Result    Expression
}

// MatchExpression represents: match subject { pattern => result, ... }
type MatchExpression struct {
    Token   token.Token
    Subject Expression
    Arms    []*MatchArm
    Default Expression
}

type MatchArm struct {
    Pattern Pattern
    Guard   Expression // optional
    Result  Expression
}

// Pattern types
type Pattern interface {
    patternNode()
}

type LiteralPattern struct {
    Value Expression // literal value to match
}

type IdentifierPattern struct {
    Name string // binds matched value to this name
}

type WildcardPattern struct{} // matches anything, binds nothing

type ObjectPattern struct {
    Keys []*KeyPattern
}

type KeyPattern struct {
    Key     string
    Pattern Pattern // nil means bind to Key name
}

type ListPattern struct {
    Elements []*Pattern
    Rest     *string // nil if no spread, otherwise variable name
}
```

## Code Examples

### Policy Evaluation

```javascript
// Access control policy
function evaluate_access(user, resource, action) {
    return match {user, resource, action} {
        {user: {role: "admin"}, action: _} => {allow: true, reason: "admin access"}

        {user: {banned: true}} => {allow: false, reason: "user banned"}

        {resource: {public: true}, action: "read"} =>
            {allow: true, reason: "public resource"}

        {user: {id}, resource: {owner}} if id == owner =>
            {allow: true, reason: "owner access"}

        {user: {role: "editor"}, resource: {draft: true}, action} if action in ["read", "edit"] =>
            {allow: true, reason: "editor draft access"}

        _ => {allow: false, reason: "default deny"}
    }
}
```

### Configuration Logic

```javascript
// Environment-aware configuration
let config = match env {
    "production" => {
        log_level: "warn",
        cache_ttl: 3600,
        debug: false
    }
    "staging" => {
        log_level: "info",
        cache_ttl: 300,
        debug: true
    }
    _ => {
        log_level: "debug",
        cache_ttl: 0,
        debug: true
    }
}
```

### Data Transformation

```javascript
// Parse different event formats
function normalize_event(raw) {
    return match raw {
        {type: "v1", data: {user_id, action}} => {
            version: 1,
            user: user_id,
            action: action
        }

        {type: "v2", payload: {user, event}} => {
            version: 2,
            user: user.id,
            action: event.name
        }

        {legacy: true, uid, act} => {
            version: 0,
            user: uid,
            action: act
        }

        _ => error("unknown event format")
    }
}
```

### Error Handling

```javascript
// Handle different result types
let output = match result {
    {ok: true, value} => format_success(value)
    {ok: false, error: {code: 404}} => "not found"
    {ok: false, error: {code, message}} => sprintf("error %d: %s", code, message)
    _ => "unknown error"
}
```

## Testing

### Parser Tests

```go
func TestParseWhenExpression(t *testing.T) {
    tests := []struct {
        input    string
        arms     int
        hasElse  bool
    }{
        {`when { a => 1, else => 0 }`, 1, true},
        {`when { a => 1, b => 2, else => 0 }`, 2, true},
        {`when { x > 0 => "pos", x < 0 => "neg", else => "zero" }`, 2, true},
    }
    // ...
}

func TestParseMatchExpression(t *testing.T) {
    tests := []struct {
        input   string
        subject string
        arms    int
    }{
        {`match x { 1 => "one", _ => "other" }`, "x", 2},
        {`match obj { {a, b} => a + b, _ => 0 }`, "obj", 2},
        {`match list { [h, ...t] => h, _ => null }`, "list", 2},
    }
    // ...
}
```

### Evaluation Tests

```go
func TestWhenExpressionEvaluation(t *testing.T) {
    tests := []struct {
        input string
        want  any
    }{
        {`let x = 5; when { x > 3 => "big", else => "small" }`, "big"},
        {`let x = 1; when { x > 3 => "big", else => "small" }`, "small"},
        {`when { false => 1, true => 2, else => 3 }`, 2},
    }
    // ...
}

func TestMatchExpressionEvaluation(t *testing.T) {
    tests := []struct {
        input string
        want  any
    }{
        {`match 1 { 1 => "one", 2 => "two", _ => "other" }`, "one"},
        {`match {a: 1} { {a} => a, _ => 0 }`, 1},
        {`match {a: 1, b: 2} { {a, b} if a < b => "ok", _ => "no" }`, "ok"},
        {`match [1,2,3] { [h, ...t] => h, _ => 0 }`, 1},
    }
    // ...
}
```

### Pattern Matching Tests

```go
func TestObjectPatternMatching(t *testing.T) {
    tests := []struct {
        pattern string
        value   string
        matches bool
        binds   map[string]any
    }{
        {`{a}`, `{a: 1}`, true, map[string]any{"a": 1}},
        {`{a}`, `{b: 1}`, false, nil},
        {`{a: 1}`, `{a: 1}`, true, nil},
        {`{a: 1}`, `{a: 2}`, false, nil},
        {`{a: x}`, `{a: 1}`, true, map[string]any{"x": 1}},
    }
    // ...
}
```

## Implementation Phases

### Phase 1: `when` Expression

- Add `when` and `else` keywords
- Parse `when` expressions
- Compile to conditional jumps
- Basic evaluation tests

### Phase 2: Simple `match` (implemented)

- Add `match` keyword
- Parse literal and wildcard patterns
- Parse identifier patterns (variable binding)
- Basic match compilation and evaluation

### Phase 2.5: Guard Expressions (implemented)

- Guard expression parsing (`pattern if condition => result`)
- Guard evaluation after pattern match
- If guard fails, continue to next arm
- `inPatternContext` flag prevents `=>` from being parsed as arrow function in guards

### Phase 3: Object Patterns

- Parse object patterns with key matching
- Nested pattern support
- Pattern matching against maps

### Phase 4: List Patterns

- Parse list patterns with spread operator
- Complete test coverage

### Phase 5: Schema Integration

- Add `schema` builtin for schema definition
- Add `matches` operator
- Implement core type constraints (string, int, number, bool)
- Implement value constraints (min, max, pattern, enum)
- Add `validate()` function for detailed error reporting

### Phase 6: Advanced Schema Features

- Schema composition (union, intersection)
- Array constraints (min_items, max_items, unique)
- Optional fields
- `json_schema()` for external JSON Schema support
- Schema caching and optimization

## Alternatives Considered

### 1. Only `match`, No `when`

Could use `match true { ... }` for condition-based switching. Rejected because:
- Less readable for the common case of simple conditions
- Requires awkward `true` subject
- `when` is more explicit about intent

### 2. `switch` Instead of `match`

More familiar from C-family languages. Rejected because:
- `switch` has fall-through semantics expectations
- `match` clearly signals pattern matching (Rust, F#, Scala)
- Avoids confusion with statement-based switch

### 3. No Exhaustiveness Requirement

Allow omitting default case. Rejected because:
- "Undefined" results are problematic for policy evaluation
- Forces authors to consider all cases
- Matches Risor's "explicit over implicit" principle

### 4. `case` Keyword

Use `case` instead of `match`. Rejected because:
- `case` often implies statement-based switching
- `match` is becoming standard for expression-based pattern matching

## Schema Validation Integration

Patterns check *structure* (keys exist, shape matches), but many policy and validation
use cases also need *constraint* checking (types, ranges, formats). JSON Schema is the
standard for this. Rather than reinvent it, Risor can integrate schema validation
alongside pattern matching.

### Design Philosophy

- **Patterns for structure, schemas for constraints**: Use patterns when you care about
  shape and want to bind values. Use schemas when you need rich validation (types,
  ranges, formats, enums).
- **Schemas are values**: Define schemas as Risor values, pass them around, compose them.
- **Familiar semantics**: Follow JSON Schema concepts but with Risor-native syntax.

### Schema Definition

A `schema` builtin creates schema objects using a concise DSL:

```javascript
// Basic type schemas
let StringSchema = schema(string)
let IntSchema = schema(int)
let NumberSchema = schema(number)
let BoolSchema = schema(bool)

// Constrained types
let AgeSchema = schema(int, {min: 0, max: 150})
let EmailSchema = schema(string, {pattern: `^[^@]+@[^@]+$`})
let StatusSchema = schema(string, {enum: ["pending", "active", "closed"]})

// Object schemas
let UserSchema = schema({
    name: string,
    age: int & min(0),
    email: string & pattern(`^[^@]+@[^@]+$`),
    role?: string,  // optional field
})

// Array schemas
let TagsSchema = schema([string], {min_items: 1, max_items: 10})
let PointSchema = schema([number, number])  // tuple: exactly 2 numbers

// Nested schemas
let OrderSchema = schema({
    id: string,
    customer: UserSchema,
    items: [{
        sku: string,
        quantity: int & min(1),
        price: number & min(0),
    }],
    total: number & min(0),
})
```

### The `matches` Operator

A new `matches` operator tests values against schemas:

```javascript
// Returns bool
input matches UserSchema

// In conditions
if request.body matches OrderSchema {
    process_order(request.body)
}

// In filters
valid_users := users.filter(u => u matches UserSchema)
```

### Schema Validation in Match Guards

Use `matches` in match guards for type-safe branching:

```javascript
let result = match event {
    {type: "user.created", data} if data matches UserSchema =>
        create_user(data)

    {type: "order.placed", data} if data matches OrderSchema =>
        process_order(data)

    {type: "order.placed", data} =>
        error("invalid order data")

    _ => error("unknown event")
}
```

### Schema Validation in When Conditions

```javascript
let response = when {
    input matches AdminRequestSchema => handle_admin(input)
    input matches UserRequestSchema => handle_user(input)
    input matches PublicRequestSchema => handle_public(input)
    else => error("invalid request format")
}
```

### Validation Results

For detailed validation errors, use `validate()` instead of `matches`:

```javascript
let result = validate(input, UserSchema)

match result {
    {valid: true} => process(input)
    {valid: false, errors} => {
        log("Validation failed:", errors)
        error("invalid input", {validation_errors: errors})
    }
    _ => error("unexpected validation result")
}
```

Validation errors include path information:

```javascript
// Example error structure
{
    valid: false,
    errors: [
        {path: ".age", message: "expected int, got string"},
        {path: ".email", message: "does not match pattern"},
        {path: ".role", message: "not one of: admin, user, guest"},
    ]
}
```

### Schema Composition

Schemas compose using operators:

```javascript
// Union (anyOf)
let IdSchema = schema(string | int)

// Intersection (allOf)
let AdminUserSchema = schema(UserSchema & {role: "admin"})

// Optional wrapper
let MaybeUserSchema = schema(UserSchema | null)

// Refinement
let AdultSchema = schema(UserSchema & {age: int & min(18)})
```

### Schema Constraints Reference

| Constraint | Applies To | Example |
|------------|------------|---------|
| `min(n)` | int, number, string (length), array (items) | `int & min(0)` |
| `max(n)` | int, number, string (length), array (items) | `string & max(100)` |
| `pattern(re)` | string | `string & pattern(`^\d+$`)` |
| `enum(...)` | any | `string & enum("a", "b", "c")` |
| `min_items(n)` | array | `[string] & min_items(1)` |
| `max_items(n)` | array | `[string] & max_items(10)` |
| `unique` | array | `[string] & unique` |

### JSON Schema Interoperability

For external JSON Schema definitions, use `json_schema()`:

```javascript
// Load from string
let ExternalSchema = json_schema(`{
    "type": "object",
    "properties": {
        "name": {"type": "string"},
        "age": {"type": "integer", "minimum": 0}
    },
    "required": ["name", "age"]
}`)

// Use like any other schema
input matches ExternalSchema
```

This allows using schemas from OpenAPI specs, JSON Schema registries, or other tools.

### Schema for Output Validation

Schemas can validate function outputs as well as inputs:

```javascript
function process_order(input) {
    require input matches OrderInputSchema

    // ... processing logic ...

    let result = {
        order_id: generate_id(),
        status: "confirmed",
        total: calculate_total(input.items),
    }

    // Validate output before returning
    require result matches OrderOutputSchema else "internal error: invalid output"
    return result
}
```

### Example: Complete Policy with Schema Validation

```javascript
// Define schemas for policy inputs/outputs
let PolicyInputSchema = schema({
    user: {
        id: string,
        role: string & enum("admin", "editor", "viewer"),
        department: string,
        active: bool,
    },
    resource: {
        id: string,
        owner: string,
        department: string,
        classification: string & enum("public", "internal", "confidential"),
    },
    action: string & enum("read", "write", "delete", "admin"),
})

let PolicyOutputSchema = schema({
    allowed: bool,
    reason: string,
    audit: {
        timestamp: string,
        decision_path: string,
    }?,
})

// Policy function with validated inputs/outputs
function evaluate_policy(input) {
    require input matches PolicyInputSchema else "invalid policy input"

    let {user, resource, action} = input

    let decision = when {
        !user.active => {allowed: false, reason: "user inactive"}
        user.role == "admin" => {allowed: true, reason: "admin access"}
        action == "read" && resource.classification == "public" =>
            {allowed: true, reason: "public resource"}
        user.department == resource.department =>
            {allowed: true, reason: "same department"}
        else => {allowed: false, reason: "default deny"}
    }

    let result = {
        ...decision,
        audit: {
            timestamp: now().format(),
            decision_path: decision.reason,
        }
    }

    require result matches PolicyOutputSchema
    return result
}
```

## Open Questions

1. **Irrefutable patterns**: Should `let {a, b} = obj` fail if `obj` lacks keys,
   or should that only happen in `match`? Current lean: fail (use `match` for fallible destructuring).

2. **Or-patterns**: Should we support `match x { 1 | 2 | 3 => "small", _ => "big" }`?
   Useful but adds complexity. Could be a future addition.

3. **Nested `when`/`match`**: Should these be allowed inside pattern arms?
   Yes, they're expressions, so they compose naturally.

4. **Pattern coverage analysis**: Should the compiler warn about overlapping patterns?
   Nice to have but complex to implement correctly.

5. **Schema as pattern**: Should schemas be usable directly as patterns in match arms?
   E.g., `match input { UserSchema => ..., OrderSchema => ... }`. Interesting but
   conflates two concepts. Current lean: keep separate, use guards.

6. **Schema inference**: Should the compiler infer schemas from patterns? Could enable
   better error messages and tooling. Future consideration.

## Conclusion

`when` and `match` expressions bring declarative, pattern-based control flow to Risor.
They're particularly valuable for policy evaluation, configuration logic, and data
transformation — all core embedding use cases. The design prioritizes clarity and
exhaustiveness over maximum flexibility, aligning with Risor's principles.

The schema validation integration extends these constructs to handle rich constraint
validation through the `matches` operator and `schema` builtin. This combination
provides:
- **Pattern matching** for structure and variable binding
- **Schema validation** for type and constraint checking
- **Guards** for arbitrary conditions

Together, they give Risor the expressive power needed for policy engines and data
validation while maintaining the language's commitment to clarity and correctness.
