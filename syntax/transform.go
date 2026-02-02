package syntax

import "github.com/deepnoodle-ai/risor/v2/ast"

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

// Transform implements the Transformer interface.
func (f TransformerFunc) Transform(p *ast.Program) (*ast.Program, error) {
	return f(p)
}
