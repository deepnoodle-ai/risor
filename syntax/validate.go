package syntax

import "github.com/risor-io/risor/ast"

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
		// Also check variable declaration for destructuring statements
		if v.config.DisallowVariableDecl {
			if _, isObjDestructure := node.(*ast.ObjectDestructure); isObjDestructure {
				return &ValidationError{
					Message:  "variable declarations are not allowed",
					Node:     node,
					Position: node.Pos(),
				}
			}
			if _, isArrDestructure := node.(*ast.ArrayDestructure); isArrDestructure {
				return &ValidationError{
					Message:  "variable declarations are not allowed",
					Node:     node,
					Position: node.Pos(),
				}
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

	case *ast.Call:
		if v.config.DisallowFuncCall {
			return &ValidationError{
				Message:  "function calls are not allowed",
				Node:     node,
				Position: node.Pos(),
			}
		}

	case *ast.ObjectCall:
		if v.config.DisallowFuncCall {
			return &ValidationError{
				Message:  "function calls are not allowed",
				Node:     node,
				Position: node.Pos(),
			}
		}

	case *ast.Try:
		if v.config.DisallowTryCatch {
			return &ValidationError{
				Message:  "try/catch/throw is not allowed",
				Node:     node,
				Position: node.Pos(),
			}
		}

	case *ast.Throw:
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
