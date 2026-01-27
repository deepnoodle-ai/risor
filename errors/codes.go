package errors

// ErrorCode represents a unique identifier for error types.
// Codes are organized by category:
//   - E1xxx: Parse errors
//   - E2xxx: Compile errors
//   - E3xxx: Runtime errors
type ErrorCode string

const (
	// Parse errors (E1xxx)
	E1001 ErrorCode = "E1001" // Unexpected token
	E1002 ErrorCode = "E1002" // Unterminated string literal
	E1003 ErrorCode = "E1003" // Invalid syntax
	E1004 ErrorCode = "E1004" // Missing expression
	E1005 ErrorCode = "E1005" // Invalid assignment target
	E1006 ErrorCode = "E1006" // Expected identifier
	E1007 ErrorCode = "E1007" // Unclosed delimiter
	E1008 ErrorCode = "E1008" // Invalid number literal
	E1009 ErrorCode = "E1009" // Maximum nesting depth exceeded
	E1010 ErrorCode = "E1010" // Invalid escape sequence

	// Compile errors (E2xxx)
	E2001 ErrorCode = "E2001" // Undefined variable
	E2002 ErrorCode = "E2002" // Undefined function
	E2003 ErrorCode = "E2003" // Invalid break statement
	E2004 ErrorCode = "E2004" // Invalid continue statement
	E2005 ErrorCode = "E2005" // Invalid return statement
	E2006 ErrorCode = "E2006" // Duplicate parameter name
	E2007 ErrorCode = "E2007" // Too many local variables
	E2008 ErrorCode = "E2008" // Too many constants
	E2009 ErrorCode = "E2009" // Too many free variables
	E2010 ErrorCode = "E2010" // Invalid destructuring pattern

	// Runtime errors (E3xxx)
	E3001 ErrorCode = "E3001" // Type error
	E3002 ErrorCode = "E3002" // Division by zero
	E3003 ErrorCode = "E3003" // Index out of bounds
	E3004 ErrorCode = "E3004" // Key not found
	E3005 ErrorCode = "E3005" // Nil reference
	E3006 ErrorCode = "E3006" // Stack overflow
	E3007 ErrorCode = "E3007" // Invalid operation
	E3008 ErrorCode = "E3008" // Import error
	E3009 ErrorCode = "E3009" // Assertion failed
	E3010 ErrorCode = "E3010" // Invalid argument
)

// codeDescriptions maps error codes to their short descriptions.
var codeDescriptions = map[ErrorCode]string{
	E1001: "unexpected token",
	E1002: "unterminated string literal",
	E1003: "invalid syntax",
	E1004: "missing expression",
	E1005: "invalid assignment target",
	E1006: "expected identifier",
	E1007: "unclosed delimiter",
	E1008: "invalid number literal",
	E1009: "maximum nesting depth exceeded",
	E1010: "invalid escape sequence",

	E2001: "undefined variable",
	E2002: "undefined function",
	E2003: "invalid break statement",
	E2004: "invalid continue statement",
	E2005: "invalid return statement",
	E2006: "duplicate parameter name",
	E2007: "too many local variables",
	E2008: "too many constants",
	E2009: "too many free variables",
	E2010: "invalid destructuring pattern",

	E3001: "type error",
	E3002: "division by zero",
	E3003: "index out of bounds",
	E3004: "key not found",
	E3005: "nil reference",
	E3006: "stack overflow",
	E3007: "invalid operation",
	E3008: "import error",
	E3009: "assertion failed",
	E3010: "invalid argument",
}

// Description returns the short description for an error code.
func (c ErrorCode) Description() string {
	if desc, ok := codeDescriptions[c]; ok {
		return desc
	}
	return "unknown error"
}

// String returns the error code as a string.
func (c ErrorCode) String() string {
	return string(c)
}

// Category returns the error category based on the code prefix.
func (c ErrorCode) Category() string {
	if len(c) < 2 {
		return "unknown"
	}
	switch c[1] {
	case '1':
		return "parse"
	case '2':
		return "compile"
	case '3':
		return "runtime"
	default:
		return "unknown"
	}
}
