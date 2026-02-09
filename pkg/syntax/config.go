// Package syntax provides AST validation and transformation.
package syntax

// SyntaxConfig controls which language features are disallowed.
// Zero value allows all features (full language).
type SyntaxConfig struct {
	// Statements
	DisallowVariableDecl bool // let, const
	DisallowAssignment   bool // x = value, x += value

	// Functions
	DisallowReturn   bool // return statements
	DisallowFuncDef  bool // function declarations and arrow functions
	DisallowFuncCall bool // calling functions (rare)

	// Error handling
	DisallowTryCatch bool // try/catch/finally, throw

	// Control flow
	DisallowIf bool // if/else expressions

	// Advanced syntax
	DisallowDestructure bool // let {a, b} = obj, let [x, y] = arr, function({a, b}) {}
	DisallowSpread      bool // ...arr, ...obj
	DisallowPipe        bool // value | fn1 | fn2
	DisallowTemplates   bool // `hello ${name}`
}

// Presets for common use cases.
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
		DisallowDestructure:  true,
		DisallowSpread:       true,
		DisallowPipe:         true,
	}

	// BasicScripting allows most language features but disallows function
	// definitions and return statements. This is useful for scripting contexts
	// where users should be able to use control flow, error handling, and
	// advanced syntax, but shouldn't define reusable functions.
	BasicScripting = SyntaxConfig{
		DisallowReturn:  true,
		DisallowFuncDef: true,
	}

	// FullLanguage allows all features (zero value, default behavior).
	FullLanguage = SyntaxConfig{}
)
