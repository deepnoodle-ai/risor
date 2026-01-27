# Error Handling Improvements

This document describes the error handling improvements implemented in Risor's compiler and VM.

## Summary

All 8 error handling shortcomings have been addressed:

| Issue | Status | Description |
|-------|--------|-------------|
| 1. Runtime Errors Missing Source | Fixed | Runtime errors now show original source lines |
| 2. Function Bodies Lose Source | Fixed | Functions inherit original source from parent |
| 3. Line Number Mismatch with Comments | Fixed | Original source preserved through compilation |
| 4. No Source for Panics | Fixed | Go panics display source context correctly |
| 5. TypeError Missing Location | Fixed | VM wraps errors with proper location info |
| 6. StructuredError vs TypeError | Fixed | Consistent error handling in VM |
| 7. No EndColumn for Runtime Errors | Fixed | Multi-character underlines supported |
| 8. Incomplete Stack Traces for Panics | Fixed | Stack traces now show all call frames |

## Before and After

**Before:**
```
type error: unsupported operation for string: + on type int
  --> ./file.risor:9:21
   |
   = stack trace:
       at __main__ (./file.risor:9:21)
```

**After:**
```
type error: unsupported operation for string: + on type int
  --> ./file.risor:3:25
   |
 3 |     let result = name + 42  // Type error here
   |                         ^^^
   |
   = stack trace:
       at greet (./file.risor:3:25)
       at __main__ (./file.risor:4:12)
```

## Implementation Details

### Phase 1: EndColumn in Source Locations

Added `EndColumn` field to track the end of error spans. **Note:** EndColumn is exclusive (points to the position after the last character), matching standard AST position conventions. For a 3-character token starting at column 5, EndColumn would be 8.

**Important distinction:**
- AST node's `End()` is exclusive (uses `Advance(len)`)
- Lexer token's `EndPosition` is inclusive (points at last char)
- Parser errors add +1 when converting token EndPosition to EndColumn

- `bytecode/location.go`: Added `EndColumn int` to `SourceLocation`
- `errors/errors.go`: Added `EndColumn int` to `SourceLocation`
- `compiler/compiler.go`: Updated `getCurrentLocation()` to capture EndColumn from AST node's `End()` position
- `compiler/code.go`: Updated `ToBytecode` to include EndColumn in conversion
- `vm/code.go`: Updated `wrapCode` to include EndColumn when loading bytecode
- `bytecode/marshal.go`: Added `end_column` to JSON serialization
- `compiler/store.go`: Added `end_column` to JSON serialization

### Phase 2: Preserved Original Source Through Compilation

Child code blocks (functions) now reference the root source for accurate line lookups:

- `compiler/code.go`: Added `rootSource *string` pointer
- `compiler/compiler.go`:
  - Added `SetSource()` method for REPL usage
  - Added `extractFunctionBodySource()` to extract function body from original source
  - Updated `CompileAST` to use original source when available
- `bytecode/code.go`: Added `parent *Code` pointer with `GetSourceLine()` traversing to root for lookups
- `cmd/risor/vm_wrapper.go`: Updated REPL to call `SetSource()` before compilation

### Phase 3: Wired EndColumn to Error Formatting

Updated error formatters to use multi-character underlines:

- `errors/structured.go`: Updated `ToFormatted()` to include EndColumn; `FriendlyErrorMessage()` uses multi-character underlines (`^^^`)
- `errors/compile.go`: Updated `ToFormatted()` to include EndColumn

### Phase 4: Complete Stack Traces for Panics

Fixed stack traces for Go panics (e.g., division by zero) to show all call frames:

- `vm/frame.go`: Added `callSiteIP` field to track the IP of the call instruction in the caller's code (separate from `returnAddr` which gets overwritten with `StopSignal`)
- `vm/vm.go`:
  - Added `panicStack` field to capture stack during panic unwind (before `defer` restores frames)
  - Updated `callFunction`'s defer to capture stack trace before restoring caller's frame
  - Updated `captureStack` to use `callSiteIP` for caller frame locations
  - Updated `panicToError` to use captured panic stack

**Before:** Stack trace only showed `__main__` frame
```
   = stack trace:
       at __main__ (./file.risor:6:16)
```

**After:** Stack trace shows complete call chain
```
   = stack trace:
       at divide (./file.risor:6:16)
       at calculate (./file.risor:10:22)
       at __main__ (./file.risor:14:29)
```

## Key Improvements

1. **Multi-character underlines** (`^^^`) that span the actual error token
2. **Original source code** with comments preserved in error output
3. **Correct line numbers** in nested functions
4. **Source lines displayed** for runtime errors in functions
5. **Complete stack traces** showing all call frames for panics (division by zero, index out of bounds, etc.)

## Test Coverage

Tests have been added to verify all improvements:

### errors package (`errors/error_improvements_test.go`)
- `TestEndColumn_MultiCharacterUnderlines` - Verifies multi-char underlines work
- `TestSourceLocationWithEndColumn` - Verifies EndColumn field
- `TestFormattedErrorIncludesEndColumn` - Verifies ToFormatted includes EndColumn
- `TestErrorFormattingWithSourceAndComments` - Verifies source preservation
- `TestStackFrameWithLocation` - Verifies stack frames have locations
- `TestToFormattedPreservesAllFields` - Verifies all fields preserved
- `TestNestedFunctionErrorLocation` - Verifies nested function errors
- `TestZeroEndColumnDefaultsToSingleCaret` - Verifies default behavior
- `TestCaretPositionAccuracy` - Verifies caret positioning

### compiler package (`compiler/compiler_test.go`)
- `TestEndColumn_InLocation` - Verifies EndColumn is set during compilation
- `TestEndColumn_SpansToken` - Verifies EndColumn spans tokens correctly
- `TestRootSource_PreservedInFunctions` - Verifies source inheritance
- `TestSetSource_ForREPL` - Verifies REPL source handling
- `TestLocationTracking_WithComments` - Verifies comment-aware line numbers

### bytecode package (`bytecode/marshal_test.go`)
- `TestMarshalUnmarshalLocationsWithEndColumn` - Verifies EndColumn serialization

### vm package (`vm/error_improvements_test.go`)
- `TestRuntimeErrorHasEndColumn` - Integration test for EndColumn
- `TestRuntimeErrorInFunctionHasSource` - Integration test for source
- `TestNestedFunctionErrorStack` - Integration test for stack traces
- `TestDivisionByZeroError` - Tests panic error locations
- `TestIndexOutOfBoundsError` - Tests array index errors
- `TestFriendlyErrorMessageFormat` - Tests friendly message format
- `TestErrorInLambda` - Tests lambda error context
- `TestErrorSourceLinePreserved` - Tests source with comments
- `TestMultiCharacterUnderlineInError` - Tests multi-char spans
- `TestFormattedErrorFromStructured` - Tests ToFormatted
- `TestCaughtErrorPreservesLocation` - Tests try/catch location
- `TestAttributeErrorLocation` - Tests attribute error locations

## Related Files

- `errors/format.go` - Central error formatter
- `errors/structured.go` - StructuredError type with FriendlyErrorMessage()
- `errors/compile.go` - CompileError type
- `compiler/compiler.go` - Source storage during compilation
- `compiler/code.go` - Code block with rootSource field
- `bytecode/code.go` - Bytecode Code with parent pointer
- `vm/vm.go` - Runtime error creation, wrapping, and stack trace capture
- `vm/frame.go` - Call frame with callSiteIP for stack traces
