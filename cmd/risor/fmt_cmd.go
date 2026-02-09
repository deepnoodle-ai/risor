package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/deepnoodle-ai/risor/v2/pkg/ast"
	"github.com/deepnoodle-ai/risor/v2/pkg/parser"
	"github.com/deepnoodle-ai/wonton/cli"
)

func fmtHandler(ctx *cli.Context) error {
	write := ctx.Bool("write")

	// Get code from -c flag, --stdin, or file argument
	code, filePath, err := getFmtCode(ctx)
	if err != nil {
		return err
	}

	// Parse the code
	program, err := parser.Parse(context.Background(), code, nil)
	if err != nil {
		return err
	}

	// Format the code
	formatted := formatProgram(program)

	if write && filePath != "" {
		// Write back to file
		return os.WriteFile(filePath, []byte(formatted), 0o644)
	}

	// Print to stdout
	fmt.Print(formatted)
	return nil
}

func getFmtCode(ctx *cli.Context) (string, string, error) {
	codeSet := ctx.IsSet("code")
	stdinSet := ctx.Bool("stdin")
	fileProvided := ctx.Arg(0) != ""

	// Check for conflicting input sources
	count := 0
	if codeSet {
		count++
	}
	if stdinSet {
		count++
	}
	if fileProvided {
		count++
	}
	if count > 1 {
		return "", "", errors.New("multiple input sources specified")
	}
	if count == 0 {
		return "", "", errors.New("no input provided")
	}

	if stdinSet {
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			return "", "", err
		}
		return string(data), "", nil
	}

	if fileProvided {
		data, err := os.ReadFile(ctx.Arg(0))
		if err != nil {
			return "", "", err
		}
		return string(data), ctx.Arg(0), nil
	}

	return ctx.String("code"), "", nil
}

// Formatter holds state for pretty-printing AST
type Formatter struct {
	buf    bytes.Buffer
	indent int
}

func formatProgram(program *ast.Program) string {
	f := &Formatter{}
	for i, stmt := range program.Stmts {
		if i > 0 {
			f.buf.WriteString("\n")
		}
		f.formatNode(stmt)
		f.buf.WriteString("\n")
	}
	return f.buf.String()
}

func (f *Formatter) writeIndent() {
	f.buf.WriteString(strings.Repeat("    ", f.indent))
}

func (f *Formatter) formatNode(node ast.Node) {
	if node == nil {
		return
	}

	switch n := node.(type) {
	case *ast.Program:
		for i, stmt := range n.Stmts {
			if i > 0 {
				f.buf.WriteString("\n")
			}
			f.formatNode(stmt)
		}

	case *ast.Var:
		f.buf.WriteString("let ")
		f.buf.WriteString(n.Name.Name)
		if n.Value != nil {
			f.buf.WriteString(" = ")
			f.formatNode(n.Value)
		}

	case *ast.Const:
		f.buf.WriteString("const ")
		f.buf.WriteString(n.Name.Name)
		if n.Value != nil {
			f.buf.WriteString(" = ")
			f.formatNode(n.Value)
		}

	case *ast.Assign:
		if n.Name != nil {
			f.formatNode(n.Name)
		}
		if n.Index != nil {
			f.buf.WriteString("[")
			f.formatNode(n.Index)
			f.buf.WriteString("]")
		}
		f.buf.WriteString(" = ")
		if n.Value != nil {
			f.formatNode(n.Value)
		}

	case *ast.Return:
		f.buf.WriteString("return")
		if n.Value != nil {
			f.buf.WriteString(" ")
			f.formatNode(n.Value)
		}

	case *ast.Block:
		f.buf.WriteString("{\n")
		f.indent++
		for i, stmt := range n.Stmts {
			if i > 0 {
				f.buf.WriteString("\n")
			}
			f.writeIndent()
			f.formatNode(stmt)
		}
		f.indent--
		f.buf.WriteString("\n")
		f.writeIndent()
		f.buf.WriteString("}")

	case *ast.If:
		f.buf.WriteString("if (")
		f.formatNode(n.Cond)
		f.buf.WriteString(") ")
		f.formatNode(n.Consequence)
		if n.Alternative != nil {
			f.buf.WriteString(" else ")
			f.formatNode(n.Alternative)
		}

	case *ast.Func:
		f.buf.WriteString("function")
		if n.Name != nil {
			f.buf.WriteString(" ")
			f.buf.WriteString(n.Name.Name)
		}
		f.buf.WriteString("(")
		f.formatParams(n.Params, n.Defaults, n.RestParam)
		f.buf.WriteString(") ")
		f.formatNode(n.Body)

	case *ast.Call:
		f.formatNode(n.Fun)
		f.buf.WriteString("(")
		for i, arg := range n.Args {
			if i > 0 {
				f.buf.WriteString(", ")
			}
			f.formatNode(arg)
		}
		f.buf.WriteString(")")

	case *ast.ObjectCall:
		f.formatNode(n.X)
		f.buf.WriteString(".")
		if n.Call != nil {
			f.formatNode(n.Call)
		}

	case *ast.GetAttr:
		f.formatNode(n.X)
		if n.Optional {
			f.buf.WriteString("?")
		}
		f.buf.WriteString(".")
		f.buf.WriteString(n.Attr.Name)

	case *ast.SetAttr:
		f.formatNode(n.X)
		f.buf.WriteString(".")
		f.buf.WriteString(n.Attr.Name)
		f.buf.WriteString(" = ")
		f.formatNode(n.Value)

	case *ast.Index:
		f.formatNode(n.X)
		f.buf.WriteString("[")
		f.formatNode(n.Index)
		f.buf.WriteString("]")

	case *ast.Slice:
		f.formatNode(n.X)
		f.buf.WriteString("[")
		if n.Low != nil {
			f.formatNode(n.Low)
		}
		f.buf.WriteString(":")
		if n.High != nil {
			f.formatNode(n.High)
		}
		f.buf.WriteString("]")

	case *ast.Infix:
		f.formatNode(n.X)
		f.buf.WriteString(" ")
		f.buf.WriteString(string(n.Op))
		f.buf.WriteString(" ")
		f.formatNode(n.Y)

	case *ast.Prefix:
		f.buf.WriteString(string(n.Op))
		f.formatNode(n.X)

	case *ast.Postfix:
		f.formatNode(n.X)
		f.buf.WriteString(string(n.Op))

	case *ast.In:
		f.formatNode(n.X)
		f.buf.WriteString(" in ")
		f.formatNode(n.Y)

	case *ast.NotIn:
		f.formatNode(n.X)
		f.buf.WriteString(" not in ")
		f.formatNode(n.Y)

	case *ast.Try:
		f.buf.WriteString("try ")
		f.formatNode(n.Body)
		if n.CatchBlock != nil {
			f.buf.WriteString(" catch")
			if n.CatchIdent != nil {
				f.buf.WriteString(" ")
				f.buf.WriteString(n.CatchIdent.Name)
			}
			f.buf.WriteString(" ")
			f.formatNode(n.CatchBlock)
		}
		if n.FinallyBlock != nil {
			f.buf.WriteString(" finally ")
			f.formatNode(n.FinallyBlock)
		}

	case *ast.Throw:
		f.buf.WriteString("throw ")
		f.formatNode(n.Value)

	case *ast.Pipe:
		for i, expr := range n.Exprs {
			if i > 0 {
				f.buf.WriteString(" | ")
			}
			f.formatNode(expr)
		}

	case *ast.Spread:
		f.buf.WriteString("...")
		f.formatNode(n.X)

	// Literals
	case *ast.Ident:
		f.buf.WriteString(n.Name)

	case *ast.Int:
		fmt.Fprintf(&f.buf, "%d", n.Value)

	case *ast.Float:
		fmt.Fprintf(&f.buf, "%g", n.Value)

	case *ast.Bool:
		if n.Value {
			f.buf.WriteString("true")
		} else {
			f.buf.WriteString("false")
		}

	case *ast.Nil:
		f.buf.WriteString("nil")

	case *ast.String:
		if n.Template != nil {
			f.buf.WriteString("`")
			f.buf.WriteString(n.Value)
			f.buf.WriteString("`")
		} else {
			fmt.Fprintf(&f.buf, "%q", n.Value)
		}

	case *ast.List:
		f.buf.WriteString("[")
		for i, item := range n.Items {
			if i > 0 {
				f.buf.WriteString(", ")
			}
			f.formatNode(item)
		}
		f.buf.WriteString("]")

	case *ast.Map:
		if len(n.Items) == 0 {
			f.buf.WriteString("{}")
			return
		}
		f.buf.WriteString("{")
		for i, item := range n.Items {
			if i > 0 {
				f.buf.WriteString(", ")
			}
			if item.Key == nil {
				// Spread
				f.formatNode(item.Value)
			} else if ident, ok := item.Key.(*ast.Ident); ok {
				f.buf.WriteString(ident.Name)
				f.buf.WriteString(": ")
				f.formatNode(item.Value)
			} else {
				f.buf.WriteString("[")
				f.formatNode(item.Key)
				f.buf.WriteString("]: ")
				f.formatNode(item.Value)
			}
		}
		f.buf.WriteString("}")

	// Destructuring
	case *ast.ObjectDestructure:
		f.buf.WriteString("let {")
		for i, b := range n.Bindings {
			if i > 0 {
				f.buf.WriteString(", ")
			}
			f.buf.WriteString(b.Key)
			if b.Alias != "" && b.Alias != b.Key {
				f.buf.WriteString(": ")
				f.buf.WriteString(b.Alias)
			}
			if b.Default != nil {
				f.buf.WriteString(" = ")
				f.formatNode(b.Default)
			}
		}
		f.buf.WriteString("}")
		if n.Value != nil {
			f.buf.WriteString(" = ")
			f.formatNode(n.Value)
		}

	case *ast.ArrayDestructure:
		f.buf.WriteString("let [")
		for i, e := range n.Elements {
			if i > 0 {
				f.buf.WriteString(", ")
			}
			if e.Name != nil {
				f.formatNode(e.Name)
			}
			if e.Default != nil {
				f.buf.WriteString(" = ")
				f.formatNode(e.Default)
			}
		}
		f.buf.WriteString("]")
		if n.Value != nil {
			f.buf.WriteString(" = ")
			f.formatNode(n.Value)
		}

	case *ast.MultiVar:
		f.buf.WriteString("let ")
		for i, name := range n.Names {
			if i > 0 {
				f.buf.WriteString(", ")
			}
			f.buf.WriteString(name.Name)
		}
		if n.Value != nil {
			f.buf.WriteString(" = ")
			f.formatNode(n.Value)
		}

	default:
		// Fallback: print type name
		fmt.Fprintf(&f.buf, "/* %T */", n)
	}
}

func (f *Formatter) formatParams(params []ast.FuncParam, defaults map[string]ast.Expr, rest *ast.Ident) {
	for i, p := range params {
		if i > 0 {
			f.buf.WriteString(", ")
		}
		// For simple identifier params, check for defaults
		if ident, ok := p.(*ast.Ident); ok {
			f.buf.WriteString(ident.Name)
			if def, ok := defaults[ident.Name]; ok && def != nil {
				f.buf.WriteString(" = ")
				f.formatNode(def)
			}
		} else {
			// For destructuring params, use String() representation
			f.buf.WriteString(p.String())
		}
	}
	if rest != nil {
		if len(params) > 0 {
			f.buf.WriteString(", ")
		}
		f.buf.WriteString("...")
		f.buf.WriteString(rest.Name)
	}
}
