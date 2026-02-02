# Forward Reference Improvement for Risor

## Problem

Previously, Risor required functions to be defined before they could be used,
which caused compilation errors for forward references. For example:

```risor
func say() {
    print(hello())  // Error: undefined variable "hello"
}

func hello() {
    return "hello"
}
```

This would fail with: `compile error: undefined variable "hello"`

## Solution

Implemented a two-pass compilation system that allows functions to be defined in
any order, similar to how Python and Go work.

### Changes Made

1. **Modified `compiler.go`**: Added a new `collectFunctionDeclarations` method that performs a first pass to collect all function declarations and add them to the symbol table.

2. **Updated `Compile` method**: Modified the main `Compile` method to perform two passes:
   - First pass: Collect function declarations
   - Second pass: Perform actual compilation

3. **Fixed `compileFunc` method**: Updated to handle cases where function names already exist in the symbol table from the first pass.

### Key Implementation Details

- **First Pass (`collectFunctionDeclarations`)**: Walks the AST and identifies all named functions at the top level, adding them to the symbol table as constants.

- **Second Pass**: Performs normal compilation, but now function names are already resolved in the symbol table, preventing "undefined variable" errors.

- **Compatibility**: The change maintains backward compatibility and doesn't break existing functionality.

### Code Changes

```go
// In compiler.go, modified the Compile method:
func (c *Compiler) Compile(node ast.Node) (*Code, error) {
    // ... existing setup code ...
    
    // First pass: collect function declarations to allow forward references
    if err := c.collectFunctionDeclarations(node); err != nil {
        return nil, err
    }
    
    // Second pass: actual compilation
    if err := c.compile(node); err != nil {
        return nil, err
    }
    
    // ... rest of method ...
}

// Added new method to collect function declarations:
func (c *Compiler) collectFunctionDeclarations(node ast.Node) error {
    switch node := node.(type) {
    case *ast.Program:
        for _, stmt := range node.Statements() {
            if err := c.collectFunctionDeclarations(stmt); err != nil {
                return err
            }
        }
    case *ast.Block:
        for _, stmt := range node.Statements() {
            if err := c.collectFunctionDeclarations(stmt); err != nil {
                return err
            }
        }
    case *ast.Func:
        // Only collect named functions at the top level
        if node.Name() != nil && c.current.parent == nil {
            functionName := node.Name().Literal()
            if _, found := c.current.symbols.Get(functionName); !found {
                if _, err := c.current.symbols.InsertConstant(functionName); err != nil {
                    return err
                }
            }
        }
    }
    return nil
}
```

### Result

After this improvement, the following code now works correctly:

```risor
func say() {
    print(hello())  // No error - hello is found in symbol table
}

func hello() {
    return "hello"
}

say()  // Outputs: hello
```

### Testing

- Created comprehensive tests to verify the fix works correctly
- Verified that existing functionality remains intact
- Tested with both simple forward references and more complex scenarios

This improvement makes Risor more user-friendly by allowing natural function
organization without worrying about declaration order.
