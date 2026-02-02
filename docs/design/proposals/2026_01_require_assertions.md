# Require Assertions for Input Validation

## Problem Statement

When writing policy rules or data transformations, scripts often need to validate
that input data has the expected structure before proceeding. Currently, this requires
verbose defensive coding:

```javascript
// Current approach: manual validation
if input.user == nil {
    return error("missing user")
}
if input.user.role == nil {
    return error("missing user.role")
}
if input.resource == nil {
    return error("missing resource")
}

// Now safe to use
let role = input.user.role
let resource_id = input.resource.id
```

This pattern is tedious, error-prone, and obscures the actual logic. Policy authors
want to declare their expectations upfront and fail fast if they're not met.

OPA/Rego handles this implicitly — if a path doesn't exist, the rule is undefined.
But Risor's explicit nature means we need a different approach: clear assertions
that validate structure and optionally bind values.

## Goals

- **Fail fast**: Invalid input should error immediately with a clear message
- **Declarative validation**: Express expectations as declarations, not imperative checks
- **Binding**: Extract values while validating structure
- **Clear errors**: Tell users exactly what's missing or wrong
- **Composable**: Work naturally with other Risor constructs

## Solution Overview

Add a `require` statement that validates conditions or patterns, failing with an
error if validation fails:

```javascript
// Condition-based
require user.authenticated
require request.method in ["GET", "POST", "PUT", "DELETE"]

// Pattern-based with binding
require {user: {role, id}} from input
require [first, ...rest] from items

// With custom error messages
require user.verified else "user must be verified"
require {api_key} from config else "missing api_key in config"
```

## Implementation Details

### Basic Syntax

**Grammar:**

```
require_stmt = "require" (pattern "from")? expression ("else" expression)?
```

**Forms:**

| Form | Meaning |
|------|---------|
| `require expr` | Assert `expr` is truthy |
| `require pattern from expr` | Assert `expr` matches `pattern`, bind variables |
| `require expr else msg` | Assert with custom error message |
| `require pattern from expr else msg` | Pattern match with custom error |

### Condition-Based Require

The simplest form asserts a condition is truthy:

```javascript
require user.authenticated
require age >= 18
require len(items) > 0
require method in ["GET", "POST"]
```

**Semantics:**
- If the expression is truthy, execution continues
- If falsy, raises an error

**Error messages:**

```
RequireError: assertion failed: user.authenticated
    at policy.risor:5
```

### Pattern-Based Require

Pattern-based require validates structure and binds values:

```javascript
require {name, email} from user
// name and email are now bound

require {config: {timeout, retries}} from settings
// timeout and retries are now bound

require [head, ...tail] from items
// head and tail are now bound
```

**Semantics:**
- The expression is evaluated
- The pattern is matched against the result
- If matching succeeds, bound variables enter scope
- If matching fails, raises an error

**Error messages:**

```
RequireError: pattern match failed: expected key "email" in {name: "Alice"}
    at policy.risor:3

RequireError: pattern match failed: expected list, got map
    at policy.risor:7
```

### Custom Error Messages

The `else` clause provides custom error messages:

```javascript
require user.admin else "admin access required"
require {token} from headers else "missing authentication token"
```

**Semantics:**
- If validation fails, the `else` expression is evaluated
- If it's a string, it becomes the error message
- If it's an error object, it's raised directly

```javascript
// String message
require valid else "validation failed"

// Error object
require valid else error("validation failed", {code: 401})
```

### Patterns

`require` uses the same pattern syntax as `match` expressions:

| Pattern | Matches | Binds |
|---------|---------|-------|
| `{a, b}` | Object with keys `a` and `b` | Values to `a` and `b` |
| `{a: x}` | Object with key `a` | Value of `a` to `x` |
| `{type: "user"}` | Object where `type == "user"` | Nothing |
| `[a, b]` | List with exactly 2 elements | Elements to `a` and `b` |
| `[head, ...tail]` | List with 1+ elements | First to `head`, rest to `tail` |

### Nested Patterns

Patterns can be nested for deep validation:

```javascript
require {
    user: {id, role},
    resource: {owner, permissions}
} from input

// All of id, role, owner, permissions are now bound
```

### Guard Conditions

For additional validation beyond structure, use `if`:

```javascript
require {age} from user if age >= 18 else "must be 18 or older"
require {items} from cart if len(items) > 0 else "cart is empty"
```

**Semantics:**
1. Pattern is matched and variables bound
2. Guard condition is evaluated (with bindings in scope)
3. If guard fails, error is raised

### Multiple Requires

Multiple requires can be chained:

```javascript
require {user} from input
require user.authenticated
require {role} from user
require role in ["admin", "editor"]
```

Or combined with commas (syntax sugar):

```javascript
require {user} from input,
        user.authenticated,
        {role} from user,
        role in ["admin", "editor"]
```

### New AST Node

```go
// RequireStatement represents: require [pattern from] expr [else msg]
type RequireStatement struct {
    Token      token.Token
    Pattern    Pattern    // optional, nil for condition-only
    Expression Expression // the value to match or condition to check
    Guard      Expression // optional, nil if no guard
    Message    Expression // optional, nil for default message
}
```

### Compilation

`require` compiles to:

```
// require expr
EVAL expr
JUMP_IF_TRUE CONTINUE
PUSH error_message
THROW
CONTINUE:

// require pattern from expr
EVAL expr
PATTERN_MATCH pattern
JUMP_IF_TRUE CONTINUE
PUSH error_message
THROW
CONTINUE:
BIND_PATTERN_VARS
```

### Error Type

A new `RequireError` type for require failures:

```go
type RequireError struct {
    Message  string
    Pattern  string // if pattern-based
    Value    any    // the value that failed to match
    Location string // source location
}
```

## Code Examples

### Policy Validation

```javascript
// Validate policy input structure upfront
require {user, resource, action} from input
require user.authenticated else "authentication required"
require action in ["read", "write", "delete"] else "invalid action"

// Now write clean policy logic
let allowed = when {
    user.role == "admin" => true
    user.id == resource.owner => true
    action == "read" && resource.public => true
    else => false
}
```

### API Request Validation

```javascript
// Validate request structure
require {method, path, headers} from request
require {authorization} from headers else "missing authorization header"
require method in ["GET", "POST", "PUT", "DELETE"]

// Parse authorization
require {type, token} from parse_auth(authorization)
require type == "Bearer" else "only Bearer auth supported"
```

### Configuration Loading

```javascript
// Validate required configuration
require {database, server} from config else "invalid configuration"
require {host, port, name} from database
require {listen_port} from server
require listen_port > 0 && listen_port < 65536 else "invalid port"
```

### Data Pipeline

```javascript
// Validate pipeline input
require {records} from batch
require len(records) > 0 else "empty batch"

// Process with confidence
records.map(record => {
    require {id, timestamp, payload} from record
    transform(id, timestamp, payload)
})
```

### Combining with Match

`require` and `match` complement each other:

```javascript
// Use require for "must have" validation
require {type, data} from event

// Use match for "one of many" branching
let result = match type {
    "create" => handle_create(data)
    "update" => handle_update(data)
    "delete" => handle_delete(data)
    _ => error("unknown event type")
}
```

### Early Return Pattern

```javascript
function process_order(input) {
    // Validate everything upfront
    require {order, customer} from input
    require {items, total} from order
    require len(items) > 0 else "order has no items"
    require {id, email} from customer
    require total > 0

    // Core logic with validated data
    let receipt = {
        order_id: generate_id(),
        customer_email: email,
        items: items,
        total: total,
        timestamp: now()
    }

    send_confirmation(email, receipt)
    return receipt
}
```

## Testing

### Parser Tests

```go
func TestParseRequireStatement(t *testing.T) {
    tests := []struct {
        input      string
        hasPattern bool
        hasMessage bool
    }{
        {`require x`, false, false},
        {`require x > 0`, false, false},
        {`require x else "error"`, false, true},
        {`require {a} from x`, true, false},
        {`require {a, b} from x else "missing"`, true, true},
        {`require [h, ...t] from x`, true, false},
    }
    // ...
}
```

### Evaluation Tests

```go
func TestRequireCondition(t *testing.T) {
    tests := []struct {
        input   string
        wantErr bool
    }{
        {`require true`, false},
        {`require false`, true},
        {`require 1 > 0`, false},
        {`require 1 < 0`, true},
    }
    // ...
}

func TestRequirePattern(t *testing.T) {
    tests := []struct {
        input   string
        wantErr bool
        binds   map[string]any
    }{
        {`require {a} from {a: 1}; a`, false, map[string]any{"a": 1}},
        {`require {a} from {b: 1}`, true, nil},
        {`require {a: 1} from {a: 1}`, false, nil},
        {`require {a: 1} from {a: 2}`, true, nil},
    }
    // ...
}

func TestRequireErrorMessage(t *testing.T) {
    tests := []struct {
        input   string
        wantMsg string
    }{
        {`require false else "custom"`, "custom"},
        {`require {a} from {} else "missing a"`, "missing a"},
    }
    // ...
}
```

### Error Message Tests

```go
func TestRequireErrorMessages(t *testing.T) {
    // Verify error messages are helpful
    tests := []struct {
        input      string
        errContains string
    }{
        {`require x.y.z`, "x.y.z"},
        {`require {a} from {b: 1}`, `key "a"`},
        {`require [a, b] from [1]`, "expected 2 elements"},
    }
    // ...
}
```

## Implementation Phases

### Phase 1: Condition-Based Require

- Add `require` keyword
- Parse `require expr`
- Parse `require expr else msg`
- Compile to conditional + throw
- Clear error messages with source location

### Phase 2: Pattern-Based Require

- Parse `require pattern from expr`
- Reuse pattern matching from `match` expressions
- Variable binding in scope
- Pattern-specific error messages

### Phase 3: Guards and Polish

- Parse `require pattern from expr if guard`
- Comma-separated multiple requires
- Improve error messages with value inspection

### Phase 4: Schema Integration

- Add `matches` operator support in require: `require x matches Schema`
- Integration with `schema` builtin (defined in match expressions proposal)
- Add `validate()` function for detailed error collection
- Schema validation error formatting

### Phase 5: Advanced Schema Features

- Combined pattern + schema: `require {a, b} from x if x matches Schema`
- `json_schema()` for loading external JSON Schema definitions
- Performance optimization for repeated schema validation

## Alternatives Considered

### 1. `assert` Instead of `require`

Use the common `assert` keyword. Rejected because:
- `assert` implies testing/debugging (often disabled in production)
- `require` better conveys "mandatory precondition"
- `require` is used similarly in Kotlin, Rust (via macros)

### 2. Implicit Validation (Rego-style)

Make undefined paths return undefined rather than error. Rejected because:
- Conflicts with Risor's explicit error handling
- "Undefined" semantics are subtle and error-prone
- Users expect clear errors when data is missing

### 3. Optional Chaining Only

Use `?.` operator for optional access. Rejected because:
- Doesn't fail when data is missing (just returns nil)
- Doesn't bind values
- Validation becomes implicit, not explicit

### 4. `guard` Keyword (Swift-style)

Use `guard condition else { ... }`. Rejected because:
- Requires block syntax for else clause
- More verbose for simple cases
- `require` with optional `else` is simpler

### 5. Pattern in `let` Statement

Use `let {a, b} = expr` and fail if pattern doesn't match. Considered for
future integration, but `require` is more explicit about intent to validate.

## Open Questions

1. **Scope of bindings**: Should bindings from `require` be available for the
   rest of the function, or limited to a block? Current lean: rest of function
   (like `let`).

2. **Multiple patterns**: Should `require {a} from x, {b} from y` be supported?
   Current lean: yes, as syntax sugar for multiple statements.

3. **Non-throwing variant**: Should there be a `require?` that returns a result
   instead of throwing? Current lean: no, use `match` for fallible matching.

4. **Integration with `let`**: Should `let {a, b} = expr` eventually support
   pattern matching with failure? If so, how does it relate to `require`?
   Current lean: keep them separate — `let` for binding, `require` for validation.

5. **Schema error aggregation**: Should `require x matches Schema` fail on first
   error or collect all errors? Current lean: fail fast (use `validate()` for
   full error collection).

6. **Schema + pattern in one**: Should `require {name: string, age: int} from x`
   support inline type annotations in patterns? Attractive but conflates concepts.
   Current lean: keep separate — pattern for structure/binding, schema for types.

7. **Schema caching**: Should schema objects be compiled/cached on first use?
   Yes, for performance. Implementation detail.

8. **Custom validators**: Should users be able to define custom validation
   functions usable in schemas? Useful but adds complexity. Future consideration.

## Relationship to `match`

`require` and `match` are complementary:

| Construct | Use When |
|-----------|----------|
| `require` | Input MUST have this structure; fail otherwise |
| `match` | Input could be one of several structures; handle each |

```javascript
// require: "I need this exact shape"
require {user: {id, role}} from input

// match: "Handle whatever shape I get"
let name = match user {
    {display_name} => display_name
    {first, last} => first + " " + last
    {email} => email.split("@")[0]
    _ => "anonymous"
}
```

## Schema Validation Integration

Patterns validate *structure* (keys exist), but many use cases need *constraint*
validation (types, ranges, formats). `require` integrates with schema validation
through the `matches` operator.

### The `matches` Operator

Use `matches` to validate values against schemas:

```javascript
// Require input matches a schema
require input matches UserSchema
require request.body matches OrderSchema else "invalid order format"

// With binding - validate then destructure
require input matches PolicyInputSchema
require {user, resource, action} from input
```

### Schema Definition

Schemas are defined using the `schema` builtin with a concise DSL:

```javascript
// Object schema with type constraints
let UserSchema = schema({
    id: string,
    name: string & max(100),
    email: string & pattern(`^[^@]+@[^@]+$`),
    age: int & min(0) & max(150),
    role: string & enum("admin", "editor", "viewer"),
    active: bool,
    tags?: [string],  // optional field
})

// Array schema
let ItemsSchema = schema([{
    sku: string,
    quantity: int & min(1),
    price: number & min(0),
}], {min_items: 1})

// Nested schemas
let OrderSchema = schema({
    customer: UserSchema,
    items: ItemsSchema,
    total: number & min(0),
})
```

### Pattern vs Schema: When to Use Each

| Use Case | Pattern | Schema | Recommendation |
|----------|---------|--------|----------------|
| Key exists | `{name}` | — | Pattern |
| Key has specific value | `{status: "active"}` | — | Pattern |
| Key has type constraint | — | `{age: int}` | Schema |
| Key has range constraint | — | `{age: int & min(0)}` | Schema |
| Key has format constraint | — | `{email: string & pattern(...)}` | Schema |
| Bind value while checking | `{name, age}` | — | Pattern |
| Validate without binding | — | `matches Schema` | Schema |

**Rule of thumb**: Use patterns for shape + binding, schemas for constraints.

### Combining Patterns and Schemas

The most robust validation combines both:

```javascript
// First: validate constraints with schema
require input matches PolicyInputSchema else "invalid input format"

// Then: destructure with pattern
require {user, resource, action} from input

// Now user, resource, action are bound AND validated
```

Or in a single step using guards:

```javascript
require {user, resource, action} from input if input matches PolicyInputSchema
```

### Detailed Validation Errors

For rich error messages, use `validate()` instead of `matches`:

```javascript
let result = validate(input, UserSchema)

require result.valid else sprintf(
    "validation failed: %s",
    result.errors.map(e => e.path + ": " + e.message).join(", ")
)
```

Validation errors include path and message:

```javascript
{
    valid: false,
    errors: [
        {path: ".age", message: "must be >= 0"},
        {path: ".email", message: "does not match pattern"},
        {path: ".items[2].quantity", message: "must be >= 1"},
    ]
}
```

### API Request Validation Example

```javascript
// Define request/response schemas
let CreateUserRequest = schema({
    name: string & min(1) & max(100),
    email: string & pattern(`^[^@]+@[^@]+\.[^@]+$`),
    password: string & min(8),
    role?: string & enum("user", "admin"),
})

let CreateUserResponse = schema({
    id: string,
    name: string,
    email: string,
    created_at: string,
})

function handle_create_user(request) {
    // Validate input
    require request.body matches CreateUserRequest else {
        let result = validate(request.body, CreateUserRequest)
        return {
            status: 400,
            body: {error: "validation failed", details: result.errors}
        }
    }

    // Destructure validated input
    require {name, email, password, role} from request.body
    let user_role = role ?? "user"

    // Create user...
    let user = create_user(name, email, password, user_role)

    // Validate output
    let response = {
        id: user.id,
        name: user.name,
        email: user.email,
        created_at: user.created_at.format(),
    }
    require response matches CreateUserResponse else "internal error: invalid response"

    return {status: 201, body: response}
}
```

### Policy Validation Example

```javascript
// Policy schemas following OPA patterns
let PolicyInput = schema({
    subject: {
        id: string,
        groups: [string],
        attributes: object,
    },
    resource: {
        type: string,
        id: string,
        attributes: object,
    },
    action: {
        name: string,
        attributes?: object,
    },
    context?: {
        timestamp: string,
        ip_address?: string,
    },
})

let PolicyOutput = schema({
    allow: bool,
    reasons: [string],
    obligations?: [{
        type: string,
        parameters: object,
    }],
})

function evaluate(input) {
    // Validate input matches expected schema
    require input matches PolicyInput else "malformed policy input"

    // Extract fields
    require {subject, resource, action} from input

    // Evaluate policy rules...
    let allow = when {
        "admin" in subject.groups => true
        action.name == "read" && resource.attributes.public => true
        else => false
    }

    let result = {
        allow: allow,
        reasons: [allow ? "policy matched" : "default deny"],
    }

    // Validate output
    require result matches PolicyOutput
    return result
}
```

### JSON Schema Interoperability

For existing JSON Schema definitions (from OpenAPI specs, etc.):

```javascript
// Load JSON Schema from string
let ExternalSchema = json_schema(`{
    "type": "object",
    "properties": {
        "name": {"type": "string", "minLength": 1},
        "age": {"type": "integer", "minimum": 0}
    },
    "required": ["name", "age"]
}`)

// Use with require
require input matches ExternalSchema
```

This enables using schemas from external sources while keeping Risor syntax for
the validation logic.

### Output Validation

Schema validation is valuable for outputs as well as inputs:

```javascript
function transform_data(raw) {
    require raw matches RawDataSchema

    // ... transformation logic ...

    let result = {/* ... */}

    // Catch bugs: ensure output matches contract
    require result matches TransformedDataSchema else "BUG: invalid output"

    return result
}
```

This catches bugs where transformation logic produces unexpected shapes, which is
especially valuable in policy engines where incorrect outputs can have security
implications.

## Conclusion

`require` brings explicit, declarative validation to Risor. It addresses a common
pain point in policy evaluation and data transformation: validating input structure
before processing. By combining condition checking, pattern matching, and variable
binding in one construct, `require` makes scripts both safer and more readable.

The integration with schema validation via `matches` extends `require` to handle
rich constraint validation (types, ranges, formats) alongside structural validation.
This makes Risor suitable for API validation, policy engines, and data pipelines
where both structure and constraints matter.

The design aligns with Risor's principles:
- **Correctness**: Fail fast on invalid input with clear errors
- **Clarity**: Validation is explicit, not hidden in defensive code
- **Foundation**: Reuses pattern matching from `match`, integrates with schemas
- **Focused**: Solves real embedding use cases (policy, API, config)
- **Elegant**: Concise syntax for common patterns; schemas for complex constraints
