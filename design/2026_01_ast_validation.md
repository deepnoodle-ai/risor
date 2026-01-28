# AST Validation and Transformation

## Background and Motivation

Risor is used in diverse embedding contexts with varying security and complexity
requirements. Some use cases need the full language, while others only need
simple expression evaluation. This design introduces AST validation and
transformation as a post-parse, pre-compile phase.

### Use Cases

1. **Expression-only mode**: Evaluate configuration values, template expressions,
   or formula inputs without allowing function definitions or control flow
2. **Restricted scripting**: Allow basic scripting but disallow certain features
   (e.g., no try/catch, no destructuring) for simpler mental model
3. **Custom transforms**: Rewrite AST before compilation (e.g., inject logging,
   apply macros, normalize patterns)
4. **Linting/analysis**: Static analysis without modifying behavior

### Design Goals

1. **Parser stays simple**: No feature flags in the parser - it always parses
   the full language
2. **Clean separation**: Validation and transformation happen in a distinct phase
3. **Composable**: Multiple validators and transformers can be chained
4. **Good errors**: Validation errors include source location and clear messages
5. **Extensible**: Users can define custom validators and transformers

## Solution Overview

Add an optional AST processing phase between parsing and compilation:

```
Source → Parser → AST → [Validate] → [Transform] → Compiler → Bytecode → VM
                        ↑_______________________↑
                              New phase
```

The public API gains options to specify validators and transformers:

```go
result, err := risor.Eval(ctx, source,
    risor.WithEnv(risor.Builtins()),
    risor.WithSyntax(risor.ExpressionOnly),       // preset
    risor.WithTransform(myCustomTransformer),     // custom
)
```

## Proposed API

### Syntax Presets

Presets are predefined `SyntaxConfig` values for common use cases:

```go
// SyntaxConfig controls which language features are disallowed.
// Zero value allows all features (full language).
type SyntaxConfig struct {
    // Statements
    DisallowVariableDecl bool // let, const
    DisallowAssignment   bool // x = value, x += value
    DisallowReturn       bool // return statements

    // Functions
    DisallowFuncDef      bool // function declarations and arrow functions
    DisallowFuncCall     bool // calling functions (rare)

    // Error handling
    DisallowTryCatch     bool // try/catch/finally, throw

    // Control flow
    DisallowIf           bool // if/else expressions
    DisallowSwitch       bool // switch expressions

    // Advanced syntax
    DisallowDestructure  bool // let {a, b} = obj, let [x, y] = arr, function({a, b}) {}
    DisallowSpread       bool // ...arr, ...obj
    DisallowPipe         bool // value | fn1 | fn2
    DisallowTemplates    bool // `hello ${name}`
}

// Presets
var (
    // ExpressionOnly restricts syntax to expressions: literals, operators,
    // variable access, indexing, attribute access, and function calls.
    // No variable declarations, no function definitions, no control flow.
    // Note: side effects are still possible via function calls in the environment.
    ExpressionOnly = SyntaxConfig{
        DisallowVariableDecl: true,
        DisallowAssignment:   true,
        DisallowReturn:       true,
        DisallowFuncDef:      true,
        DisallowTryCatch:     true,
        DisallowIf:           true,
        DisallowSwitch:       true,
        DisallowDestructure:  true,
        DisallowSpread:       true,
        DisallowPipe:         true,
    }

    // BasicScripting adds variable declarations, assignment, and if/else.
    // Still no function definitions or error handling.
    BasicScripting = SyntaxConfig{
        DisallowReturn:      true,
        DisallowFuncDef:     true,
        DisallowTryCatch:    true,
        DisallowSwitch:      true,
        DisallowDestructure: true,
        DisallowSpread:      true,
        DisallowPipe:        true,
    }

    // FullLanguage allows all features (zero value, default behavior).
    FullLanguage = SyntaxConfig{}
)
```

### Validator Interface

Validators inspect the AST and return errors for disallowed constructs:

```go
// ValidationError represents a syntax restriction violation.
type ValidationError struct {
    Message  string
    Node     ast.Node         // the offending node
    Position token.Position   // source location
}

func (e *ValidationError) Error() string {
    return fmt.Sprintf("%s at %s", e.Message, e.Position)
}

// Validator inspects an AST and returns validation errors.
// Validators should not modify the AST.
type Validator interface {
    // Validate checks the AST and returns any validation errors.
    // Multiple errors may be returned to show all violations at once.
    Validate(program *ast.Program) []ValidationError
}

// ValidatorFunc is an adapter to use a function as a Validator.
type ValidatorFunc func(*ast.Program) []ValidationError

func (f ValidatorFunc) Validate(p *ast.Program) []ValidationError {
    return f(p)
}
```

### Transformer Interface

Transformers can modify or replace AST nodes:

```go
// Transformer modifies an AST before compilation.
// Transformers receive ownership of the AST and return a (possibly new) AST.
type Transformer interface {
    // Transform processes the AST and returns the result.
    // The returned AST may be the same instance (modified in place)
    // or a completely new AST.
    Transform(program *ast.Program) (*ast.Program, error)
}

// TransformerFunc is an adapter to use a function as a Transformer.
type TransformerFunc func(*ast.Program) (*ast.Program, error)

func (f TransformerFunc) Transform(p *ast.Program) (*ast.Program, error) {
    return f(p)
}
```

### Public API Options

```go
// WithSyntax applies a syntax configuration that restricts allowed constructs.
// The validator runs after parsing and before any transformers.
func WithSyntax(config SyntaxConfig) Option {
    return func(o *options) {
        o.syntaxConfig = &config
    }
}

// WithValidator adds a custom validator to run after parsing.
// Multiple validators can be added; they run in order.
// Validation runs before transformation.
func WithValidator(v Validator) Option {
    return func(o *options) {
        o.validators = append(o.validators, v)
    }
}

// WithTransform adds a transformer to run after validation.
// Multiple transformers can be added; they run in order.
// Each transformer receives the output of the previous one.
func WithTransform(t Transformer) Option {
    return func(o *options) {
        o.transformers = append(o.transformers, t)
    }
}
```

## Implementation

### Package Structure

```
risor/
├── syntax/
│   ├── config.go       // SyntaxConfig and presets
│   ├── validator.go    // Validator interface, ValidationError
│   ├── validate.go     // SyntaxValidator implementation
│   └── validate_test.go
```

### SyntaxValidator Implementation

The `SyntaxValidator` walks the AST once and collects all violations:

```go
// SyntaxValidator validates an AST against a SyntaxConfig.
type SyntaxValidator struct {
    config SyntaxConfig
}

// NewSyntaxValidator creates a validator for the given configuration.
func NewSyntaxValidator(config SyntaxConfig) *SyntaxValidator {
    return &SyntaxValidator{config: config}
}

// Validate checks the AST against the syntax configuration.
func (v *SyntaxValidator) Validate(program *ast.Program) []ValidationError {
    var errors []ValidationError

    for node := range ast.Preorder(program) {
        if err := v.checkNode(node); err != nil {
            errors = append(errors, *err)
        }
    }

    return errors
}

func (v *SyntaxValidator) checkNode(node ast.Node) *ValidationError {
    switch n := node.(type) {
    case *ast.Var, *ast.Const, *ast.MultiVar:
        if v.config.DisallowVariableDecl {
            return &ValidationError{
                Message:  "variable declarations are not allowed",
                Node:     node,
                Position: node.Pos(),
            }
        }

    case *ast.ObjectDestructure, *ast.ArrayDestructure,
        *ast.ObjectDestructureParam, *ast.ArrayDestructureParam:
        if v.config.DisallowDestructure {
            return &ValidationError{
                Message:  "destructuring is not allowed",
                Node:     node,
                Position: node.Pos(),
            }
        }

    case *ast.Assign, *ast.SetAttr, *ast.Postfix:
        if v.config.DisallowAssignment {
            return &ValidationError{
                Message:  "assignment is not allowed",
                Node:     node,
                Position: node.Pos(),
            }
        }

    case *ast.Return:
        if v.config.DisallowReturn {
            return &ValidationError{
                Message:  "return statements are not allowed",
                Node:     node,
                Position: node.Pos(),
            }
        }

    case *ast.Func:
        if v.config.DisallowFuncDef {
            return &ValidationError{
                Message:  "function definitions are not allowed",
                Node:     node,
                Position: node.Pos(),
            }
        }

    case *ast.Call, *ast.ObjectCall:
        if v.config.DisallowFuncCall {
            return &ValidationError{
                Message:  "function calls are not allowed",
                Node:     node,
                Position: node.Pos(),
            }
        }

    case *ast.Try, *ast.Throw:
        if v.config.DisallowTryCatch {
            return &ValidationError{
                Message:  "try/catch/throw is not allowed",
                Node:     node,
                Position: node.Pos(),
            }
        }

    case *ast.If:
        if v.config.DisallowIf {
            return &ValidationError{
                Message:  "if expressions are not allowed",
                Node:     node,
                Position: node.Pos(),
            }
        }

    case *ast.Switch:
        if v.config.DisallowSwitch {
            return &ValidationError{
                Message:  "switch expressions are not allowed",
                Node:     node,
                Position: node.Pos(),
            }
        }

    case *ast.Spread:
        if v.config.DisallowSpread {
            return &ValidationError{
                Message:  "spread syntax is not allowed",
                Node:     node,
                Position: node.Pos(),
            }
        }

    case *ast.Pipe:
        if v.config.DisallowPipe {
            return &ValidationError{
                Message:  "pipe expressions are not allowed",
                Node:     node,
                Position: node.Pos(),
            }
        }

    case *ast.String:
        if n.Template != nil && v.config.DisallowTemplates {
            return &ValidationError{
                Message:  "template strings are not allowed",
                Node:     node,
                Position: node.Pos(),
            }
        }
    }

    return nil
}
```

### Integration with risor.go

Update the `Compile` function to run validation and transformation:

```go
func Compile(ctx context.Context, source string, opts ...Option) (*bytecode.Code, error) {
    o := collectOptions(opts...)

    // Parse
    var parserCfg *parser.Config
    if o.filename != "" {
        parserCfg = &parser.Config{Filename: o.filename}
    }
    program, err := parser.Parse(ctx, source, parserCfg)
    if err != nil {
        return nil, err
    }

    // Validate syntax config (if specified)
    if o.syntaxConfig != nil {
        validator := syntax.NewSyntaxValidator(*o.syntaxConfig)
        if errs := validator.Validate(program); len(errs) > 0 {
            return nil, syntax.NewValidationErrors(errs)
        }
    }

    // Run custom validators
    for _, v := range o.validators {
        if errs := v.Validate(program); len(errs) > 0 {
            return nil, syntax.NewValidationErrors(errs)
        }
    }

    // Run transformers
    for _, t := range o.transformers {
        program, err = t.Transform(program)
        if err != nil {
            return nil, err
        }
    }

    // Compile
    cfg := o.compilerConfig()
    cfg.Source = source
    return compiler.Compile(program, cfg)
}
```

## Examples

### Expression-Only Evaluation

```go
// Restrict syntax to pure expressions - no variable declarations, no control
// flow, no function definitions. Note: function calls are still allowed, so
// side effects depend on what functions are provided in the environment.
result, err := risor.Eval(ctx, "price * quantity * (1 - discount)",
    risor.WithEnv(map[string]any{
        "price":    100.0,
        "quantity": 5,
        "discount": 0.1,
    }),
    risor.WithSyntax(risor.ExpressionOnly),
)
// result: 450.0

// This would fail validation:
_, err := risor.Eval(ctx, "let x = 1; x + 1",
    risor.WithSyntax(risor.ExpressionOnly),
)
// err: "variable declarations are not allowed at line 1, column 1"
```

### Custom Validator

```go
// Disallow accessing the "secret" variable
noSecrets := risor.ValidatorFunc(func(p *ast.Program) []risor.ValidationError {
    var errs []risor.ValidationError
    for node := range ast.Preorder(p) {
        if ident, ok := node.(*ast.Ident); ok && ident.Name == "secret" {
            errs = append(errs, risor.ValidationError{
                Message:  "access to 'secret' is not allowed",
                Node:     node,
                Position: node.Pos(),
            })
        }
    }
    return errs
})

_, err := risor.Eval(ctx, "secret + 1",
    risor.WithValidator(noSecrets),
)
// err: "access to 'secret' is not allowed at line 1, column 1"
```

### AST Transformation

```go
// Wrap all function calls with logging
loggingTransformer := risor.TransformerFunc(func(p *ast.Program) (*ast.Program, error) {
    // Walk and transform...
    // This is a simplified example - real implementation would use
    // a proper tree rewriter
    return p, nil
})

result, err := risor.Eval(ctx, source,
    risor.WithEnv(risor.Builtins()),
    risor.WithTransform(loggingTransformer),
)
```

### Chaining Validators and Transformers

```go
result, err := risor.Eval(ctx, source,
    risor.WithEnv(risor.Builtins()),
    risor.WithSyntax(risor.BasicScripting),    // built-in validation
    risor.WithValidator(noSecrets),             // custom validation
    risor.WithValidator(maxComplexity(10)),     // another validator
    risor.WithTransform(normalizeOperators),    // transform 1
    risor.WithTransform(injectLogging),         // transform 2
)
```

## AST Rewriter Utilities

For complex transformations, provide a `Rewriter` helper:

```go
// Rewriter helps transform AST nodes.
// It walks the tree depth-first and calls the transform function for each node.
// If the function returns a different node, it replaces the original.
type Rewriter struct {
    // Transform is called for each node. Return the same node to keep it,
    // or return a new node to replace it. Return nil to remove the node
    // (only valid for statements in a block).
    Transform func(node ast.Node) ast.Node
}

// Rewrite transforms the program using the configured transform function.
func (r *Rewriter) Rewrite(program *ast.Program) *ast.Program {
    // Implementation walks and rebuilds the tree...
}

// Example usage:
rewriter := &syntax.Rewriter{
    Transform: func(node ast.Node) ast.Node {
        // Double all integer literals
        if intNode, ok := node.(*ast.Int); ok {
            return &ast.Int{
                ValuePos: intNode.ValuePos,
                Literal:  fmt.Sprintf("%d", intNode.Value*2),
                Value:    intNode.Value * 2,
            }
        }
        return node
    },
}

transformer := risor.TransformerFunc(func(p *ast.Program) (*ast.Program, error) {
    return rewriter.Rewrite(p), nil
})
```

## Error Handling

### Multiple Validation Errors

Validators can return multiple errors to show all violations at once:

```go
// ValidationErrors wraps multiple validation errors.
type ValidationErrors struct {
    Errors []ValidationError
}

func (e *ValidationErrors) Error() string {
    if len(e.Errors) == 1 {
        return e.Errors[0].Error()
    }
    var b strings.Builder
    fmt.Fprintf(&b, "%d validation errors:\n", len(e.Errors))
    for _, err := range e.Errors {
        fmt.Fprintf(&b, "  - %s\n", err.Error())
    }
    return b.String()
}

// Unwrap returns the first error for errors.Is/As compatibility.
func (e *ValidationErrors) Unwrap() error {
    if len(e.Errors) > 0 {
        return &e.Errors[0]
    }
    return nil
}
```

### Transformer Errors

Transformers return a single error (they stop on first error):

```go
transformer := risor.TransformerFunc(func(p *ast.Program) (*ast.Program, error) {
    // If transformation fails
    return nil, fmt.Errorf("transform failed: %w", someErr)
})
```

## Testing Strategy

### Unit Tests

1. **Preset validation**: Each preset blocks expected constructs
2. **Individual flags**: Each config flag works independently
3. **Error messages**: Errors include correct positions and messages
4. **Multiple errors**: Validator returns all violations, not just first
5. **Transformer chaining**: Multiple transformers apply in order

### Integration Tests

1. **End-to-end presets**: `risor.Eval` with each preset
2. **Custom validators**: User-defined validators integrate correctly
3. **Transform + validate**: Transformers run after validation
4. **Error propagation**: Validation errors propagate through Eval/Compile

### Example Test Cases

```go
func TestExpressionOnlyPreset(t *testing.T) {
    tests := []struct {
        source  string
        wantErr bool
    }{
        {"1 + 2", false},                           // allowed
        {"x * y", false},                           // variable access
        {"items.filter(x => x > 0)", false},        // method call
        {"let x = 1", true},                        // variable decl
        {"function foo() {}", true},                // function def
        {"if (true) { 1 }", true},                  // if expression
    }

    for _, tt := range tests {
        _, err := risor.Eval(ctx, tt.source,
            risor.WithEnv(map[string]any{"x": 1, "y": 2, "items": []int{1, 2, 3}}),
            risor.WithSyntax(risor.ExpressionOnly),
        )
        if tt.wantErr && err == nil {
            t.Errorf("%q: expected error", tt.source)
        }
        if !tt.wantErr && err != nil {
            t.Errorf("%q: unexpected error: %v", tt.source, err)
        }
    }
}
```

## Design Decisions

### Why Post-Parse Validation?

**Alternative considered**: Feature flags in the parser.

**Reasons for post-parse:**
1. Parser stays simple with single responsibility
2. Same AST types work for all configurations
3. Validation logic is isolated and testable
4. Enables multiple validators and custom validators
5. Opens door to AST transformation
6. Negligible performance cost for typical script sizes

### Why SyntaxConfig Uses Bool Fields?

**Alternative considered**: `map[string]bool` for feature flags.

**Reasons for struct:**
1. Compile-time checking of feature names
2. IDE autocomplete support
3. Clear documentation via field comments
4. Presets are just struct values
5. No string typos possible

### Why Separate Validator and Transformer Interfaces?

**Alternative considered**: Single interface that can both validate and transform.

**Reasons for separation:**
1. Clear intent: validators don't modify, transformers do
2. Validation runs first, transformation runs second
3. Multiple validators can run and aggregate errors
4. Transformers are sequential (each sees previous output)
5. Easier to reason about behavior

### Transformation Phase Ordering

Validators run before transformers because:
1. Validate the original source, not transformed version
2. User intent is to restrict what they wrote
3. Transformers might introduce constructs that would fail validation

If a use case needs post-transform validation, run a validator manually:

```go
postValidator := risor.ValidatorFunc(...)

risor.WithTransform(risor.TransformerFunc(func(p *ast.Program) (*ast.Program, error) {
    transformed := doTransform(p)
    if errs := postValidator.Validate(transformed); len(errs) > 0 {
        return nil, syntax.NewValidationErrors(errs)
    }
    return transformed, nil
}))
```

## Files to Create/Modify

### New Files

- `syntax/config.go` - SyntaxConfig struct and presets
- `syntax/validator.go` - Validator interface, ValidationError, ValidationErrors
- `syntax/validate.go` - SyntaxValidator implementation
- `syntax/transform.go` - Transformer interface
- `syntax/rewriter.go` - Rewriter helper (optional, phase 2)
- `syntax/validate_test.go` - Tests

### Modified Files

- `risor.go` - Add options, integrate into Compile(), re-export types for ergonomic API
- `options.go` (or inline in risor.go) - Add syntaxConfig, validators, transformers fields

### Type Re-exports in risor.go

To provide a clean API where users can write `risor.ValidatorFunc` instead of
`syntax.ValidatorFunc`, re-export the key types:

```go
// Re-export syntax types for convenient access
type (
    SyntaxConfig     = syntax.SyntaxConfig
    Validator        = syntax.Validator
    ValidatorFunc    = syntax.ValidatorFunc
    ValidationError  = syntax.ValidationError
    ValidationErrors = syntax.ValidationErrors
    Transformer      = syntax.Transformer
    TransformerFunc  = syntax.TransformerFunc
)

// Re-export presets
var (
    ExpressionOnly = syntax.ExpressionOnly
    BasicScripting = syntax.BasicScripting
    FullLanguage   = syntax.FullLanguage
)
```

## Future Extensions

1. **Built-in transformers**: Common transforms like constant folding, dead code removal
2. **AST pretty printer**: Regenerate source from AST (useful after transforms)
3. **Macro system**: User-defined syntax extensions via transformers
4. **Analysis tools**: Complexity metrics, dependency graphs, etc.
5. **Source maps**: Track original positions through transforms
