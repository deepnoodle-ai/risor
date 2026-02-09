package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"reflect"
	"strings"

	"github.com/deepnoodle-ai/risor/v2/pkg/ast"
	"github.com/deepnoodle-ai/risor/v2/pkg/parser"
	"github.com/deepnoodle-ai/wonton/cli"
	"github.com/deepnoodle-ai/wonton/tui"
)

func astHandler(ctx *cli.Context) error {
	// Get code from -c flag, --stdin, or file argument
	code, err := getAstCode(ctx)
	if err != nil {
		return err
	}

	// Parse the code
	program, err := parser.Parse(context.Background(), code, nil)
	if err != nil {
		return err
	}

	outputFormat := ctx.String("output")
	if outputFormat == "json" {
		return printASTJSON(program)
	}

	// Print the AST
	printAST(program)
	return nil
}

// ASTNode represents a node in the JSON AST output
type ASTNode struct {
	Type     string     `json:"type"`
	Value    any        `json:"value,omitempty"`
	Children []*ASTNode `json:"children,omitempty"`
}

func printASTJSON(program *ast.Program) error {
	root := nodeToJSON(program)
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(root)
}

func nodeToJSON(node ast.Node) *ASTNode {
	if node == nil {
		return nil
	}

	typeName := reflect.TypeOf(node).Elem().Name()
	result := &ASTNode{Type: typeName}

	switch n := node.(type) {
	case *ast.Program:
		for _, stmt := range n.Stmts {
			if child := nodeToJSON(stmt); child != nil {
				result.Children = append(result.Children, child)
			}
		}

	case *ast.Ident:
		result.Value = n.Name

	case *ast.Int:
		result.Value = n.Value

	case *ast.Float:
		result.Value = n.Value

	case *ast.Bool:
		result.Value = n.Value

	case *ast.String:
		result.Value = n.Value

	case *ast.Nil:
		result.Value = nil

	case *ast.Var:
		result.Value = n.Name.Name
		if n.Value != nil {
			result.Children = append(result.Children, nodeToJSON(n.Value))
		}

	case *ast.Const:
		result.Value = n.Name.Name
		if n.Value != nil {
			result.Children = append(result.Children, nodeToJSON(n.Value))
		}

	case *ast.Assign:
		if n.Name != nil {
			result.Children = append(result.Children, nodeToJSON(n.Name))
		}
		if n.Value != nil {
			result.Children = append(result.Children, nodeToJSON(n.Value))
		}

	case *ast.Infix:
		result.Value = string(n.Op)
		if n.X != nil {
			result.Children = append(result.Children, nodeToJSON(n.X))
		}
		if n.Y != nil {
			result.Children = append(result.Children, nodeToJSON(n.Y))
		}

	case *ast.Prefix:
		result.Value = string(n.Op)
		if n.X != nil {
			result.Children = append(result.Children, nodeToJSON(n.X))
		}

	case *ast.Call:
		if n.Fun != nil {
			result.Children = append(result.Children, nodeToJSON(n.Fun))
		}
		for _, arg := range n.Args {
			result.Children = append(result.Children, nodeToJSON(arg))
		}

	case *ast.Func:
		if n.Name != nil {
			result.Value = n.Name.Name
		}
		if n.Body != nil {
			result.Children = append(result.Children, nodeToJSON(n.Body))
		}

	case *ast.Return:
		if n.Value != nil {
			result.Children = append(result.Children, nodeToJSON(n.Value))
		}

	case *ast.Block:
		for _, stmt := range n.Stmts {
			result.Children = append(result.Children, nodeToJSON(stmt))
		}

	case *ast.If:
		if n.Cond != nil {
			result.Children = append(result.Children, &ASTNode{
				Type:     "Condition",
				Children: []*ASTNode{nodeToJSON(n.Cond)},
			})
		}
		if n.Consequence != nil {
			result.Children = append(result.Children, &ASTNode{
				Type:     "Then",
				Children: []*ASTNode{nodeToJSON(n.Consequence)},
			})
		}
		if n.Alternative != nil {
			result.Children = append(result.Children, &ASTNode{
				Type:     "Else",
				Children: []*ASTNode{nodeToJSON(n.Alternative)},
			})
		}

	case *ast.List:
		for _, item := range n.Items {
			result.Children = append(result.Children, nodeToJSON(item))
		}

	case *ast.Map:
		for _, pair := range n.Items {
			pairNode := &ASTNode{Type: "MapPair"}
			if pair.Key != nil {
				pairNode.Children = append(pairNode.Children, nodeToJSON(pair.Key))
			}
			if pair.Value != nil {
				pairNode.Children = append(pairNode.Children, nodeToJSON(pair.Value))
			}
			result.Children = append(result.Children, pairNode)
		}

	case *ast.GetAttr:
		result.Value = n.Attr.Name
		if n.X != nil {
			result.Children = append(result.Children, nodeToJSON(n.X))
		}

	case *ast.Index:
		if n.X != nil {
			result.Children = append(result.Children, nodeToJSON(n.X))
		}
		if n.Index != nil {
			result.Children = append(result.Children, nodeToJSON(n.Index))
		}

	default:
		// Use reflection for other nodes
		v := reflect.ValueOf(node)
		if v.Kind() == reflect.Ptr {
			v = v.Elem()
		}
		if v.Kind() == reflect.Struct {
			t := v.Type()
			for i := 0; i < v.NumField(); i++ {
				field := v.Field(i)
				fieldName := t.Field(i).Name
				if !field.CanInterface() || fieldName == "From" || fieldName == "To" {
					continue
				}
				if child, ok := field.Interface().(ast.Node); ok && child != nil {
					result.Children = append(result.Children, nodeToJSON(child))
				}
				if field.Kind() == reflect.Slice {
					for j := 0; j < field.Len(); j++ {
						elem := field.Index(j)
						if child, ok := elem.Interface().(ast.Node); ok && child != nil {
							result.Children = append(result.Children, nodeToJSON(child))
						}
					}
				}
			}
		}
	}

	return result
}

func getAstCode(ctx *cli.Context) (string, error) {
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
		return "", errors.New("multiple input sources specified")
	}
	if count == 0 {
		return "", errors.New("no input provided")
	}

	if stdinSet {
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			return "", err
		}
		return string(data), nil
	}

	if fileProvided {
		data, err := os.ReadFile(ctx.Arg(0))
		if err != nil {
			return "", err
		}
		return string(data), nil
	}

	return ctx.String("code"), nil
}

// Color styles for AST display
var (
	nodeStyle    = tui.NewStyle().WithFgRGB(tui.RGB{R: 100, G: 200, B: 255}).WithBold()
	fieldStyle   = tui.NewStyle().WithFgRGB(tui.RGB{R: 180, G: 140, B: 220})
	valueStyle   = tui.NewStyle().WithFgRGB(tui.RGB{R: 150, G: 220, B: 150})
	literalStyle = tui.NewStyle().WithFgRGB(tui.RGB{R: 255, G: 200, B: 100})
	mutedStyle   = tui.NewStyle().WithFgRGB(tui.RGB{R: 120, G: 120, B: 130})
)

// printLine prints a tui.View followed by a newline
func printLine(view tui.View) {
	tui.Print(view)
	fmt.Println()
}

func printAST(program *ast.Program) {
	printLine(tui.Text("Program").Style(nodeStyle))
	for i, stmt := range program.Stmts {
		isLast := i == len(program.Stmts)-1
		printNode(stmt, "  ", isLast)
	}
}

func printNode(node ast.Node, indent string, isLast bool) {
	if node == nil {
		return
	}

	// Choose connector
	connector := "├─ "
	childIndent := indent + "│  "
	if isLast {
		connector = "└─ "
		childIndent = indent + "   "
	}

	typeName := reflect.TypeOf(node).Elem().Name()

	// Print node with type-specific details
	switch n := node.(type) {
	case *ast.Ident:
		printLine(tui.Group(
			tui.Text("%s%s", indent, connector).Style(mutedStyle),
			tui.Text("%s", typeName).Style(nodeStyle),
			tui.Text(" %q", n.Name).Style(literalStyle),
		))

	case *ast.Int:
		printLine(tui.Group(
			tui.Text("%s%s", indent, connector).Style(mutedStyle),
			tui.Text("%s", typeName).Style(nodeStyle),
			tui.Text(" %d", n.Value).Style(literalStyle),
		))

	case *ast.Float:
		printLine(tui.Group(
			tui.Text("%s%s", indent, connector).Style(mutedStyle),
			tui.Text("%s", typeName).Style(nodeStyle),
			tui.Text(" %g", n.Value).Style(literalStyle),
		))

	case *ast.Bool:
		printLine(tui.Group(
			tui.Text("%s%s", indent, connector).Style(mutedStyle),
			tui.Text("%s", typeName).Style(nodeStyle),
			tui.Text(" %v", n.Value).Style(literalStyle),
		))

	case *ast.String:
		// Show truncated string value
		val := n.Value
		if len(val) > 30 {
			val = val[:27] + "..."
		}
		printLine(tui.Group(
			tui.Text("%s%s", indent, connector).Style(mutedStyle),
			tui.Text("%s", typeName).Style(nodeStyle),
			tui.Text(" %q", val).Style(literalStyle),
		))
		// Print template expressions if any
		for i, expr := range n.Exprs {
			printNode(expr, childIndent, i == len(n.Exprs)-1)
		}

	case *ast.Nil:
		printLine(tui.Group(
			tui.Text("%s%s", indent, connector).Style(mutedStyle),
			tui.Text("%s", typeName).Style(nodeStyle),
		))

	case *ast.Var:
		printLine(tui.Group(
			tui.Text("%s%s", indent, connector).Style(mutedStyle),
			tui.Text("%s", typeName).Style(nodeStyle),
			tui.Text(" %s", n.Name).Style(valueStyle),
		))
		if n.Value != nil {
			printNode(n.Value, childIndent, true)
		}

	case *ast.Const:
		printLine(tui.Group(
			tui.Text("%s%s", indent, connector).Style(mutedStyle),
			tui.Text("%s", typeName).Style(nodeStyle),
			tui.Text(" %s", n.Name).Style(valueStyle),
		))
		if n.Value != nil {
			printNode(n.Value, childIndent, true)
		}

	case *ast.Assign:
		printLine(tui.Group(
			tui.Text("%s%s", indent, connector).Style(mutedStyle),
			tui.Text("%s", typeName).Style(nodeStyle),
		))
		if n.Name != nil {
			printNode(n.Name, childIndent, n.Value == nil)
		}
		if n.Value != nil {
			printNode(n.Value, childIndent, true)
		}

	case *ast.Infix:
		printLine(tui.Group(
			tui.Text("%s%s", indent, connector).Style(mutedStyle),
			tui.Text("%s", typeName).Style(nodeStyle),
			tui.Text(" %s", n.Op).Style(fieldStyle),
		))
		if n.X != nil {
			printNode(n.X, childIndent, n.Y == nil)
		}
		if n.Y != nil {
			printNode(n.Y, childIndent, true)
		}

	case *ast.Prefix:
		printLine(tui.Group(
			tui.Text("%s%s", indent, connector).Style(mutedStyle),
			tui.Text("%s", typeName).Style(nodeStyle),
			tui.Text(" %s", n.Op).Style(fieldStyle),
		))
		if n.X != nil {
			printNode(n.X, childIndent, true)
		}

	case *ast.Call:
		printLine(tui.Group(
			tui.Text("%s%s", indent, connector).Style(mutedStyle),
			tui.Text("%s", typeName).Style(nodeStyle),
		))
		if n.Fun != nil {
			printNode(n.Fun, childIndent, len(n.Args) == 0)
		}
		for i, arg := range n.Args {
			printNode(arg, childIndent, i == len(n.Args)-1)
		}

	case *ast.GetAttr:
		optMarker := ""
		if n.Optional {
			optMarker = "?"
		}
		printLine(tui.Group(
			tui.Text("%s%s", indent, connector).Style(mutedStyle),
			tui.Text("%s", typeName).Style(nodeStyle),
			tui.Text(" %s.%s", optMarker, n.Attr.Name).Style(fieldStyle),
		))
		if n.X != nil {
			printNode(n.X, childIndent, true)
		}

	case *ast.Index:
		printLine(tui.Group(
			tui.Text("%s%s", indent, connector).Style(mutedStyle),
			tui.Text("%s", typeName).Style(nodeStyle),
		))
		if n.X != nil {
			printNode(n.X, childIndent, n.Index == nil)
		}
		if n.Index != nil {
			printNode(n.Index, childIndent, true)
		}

	case *ast.If:
		printLine(tui.Group(
			tui.Text("%s%s", indent, connector).Style(mutedStyle),
			tui.Text("%s", typeName).Style(nodeStyle),
		))
		if n.Cond != nil {
			printLine(tui.Group(
				tui.Text("%s├─ ", childIndent).Style(mutedStyle),
				tui.Text("condition").Style(fieldStyle),
			))
			printNode(n.Cond, childIndent+"│  ", true)
		}
		if n.Consequence != nil {
			hasAlt := n.Alternative != nil
			if hasAlt {
				printLine(tui.Group(
					tui.Text("%s├─ ", childIndent).Style(mutedStyle),
					tui.Text("then").Style(fieldStyle),
				))
			} else {
				printLine(tui.Group(
					tui.Text("%s└─ ", childIndent).Style(mutedStyle),
					tui.Text("then").Style(fieldStyle),
				))
			}
			subIndent := childIndent
			if hasAlt {
				subIndent += "│  "
			} else {
				subIndent += "   "
			}
			printNode(n.Consequence, subIndent, true)
		}
		if n.Alternative != nil {
			printLine(tui.Group(
				tui.Text("%s└─ ", childIndent).Style(mutedStyle),
				tui.Text("else").Style(fieldStyle),
			))
			printNode(n.Alternative, childIndent+"   ", true)
		}

	case *ast.Func:
		name := "<anonymous>"
		if n.Name != nil {
			name = n.Name.Name
		}
		params := make([]string, len(n.Params))
		for i, p := range n.Params {
			params[i] = p.String()
		}
		printLine(tui.Group(
			tui.Text("%s%s", indent, connector).Style(mutedStyle),
			tui.Text("%s", typeName).Style(nodeStyle),
			tui.Text(" %s(%s)", name, strings.Join(params, ", ")).Style(valueStyle),
		))
		if n.Body != nil {
			printNode(n.Body, childIndent, true)
		}

	case *ast.Return:
		printLine(tui.Group(
			tui.Text("%s%s", indent, connector).Style(mutedStyle),
			tui.Text("%s", typeName).Style(nodeStyle),
		))
		if n.Value != nil {
			printNode(n.Value, childIndent, true)
		}

	case *ast.Block:
		printLine(tui.Group(
			tui.Text("%s%s", indent, connector).Style(mutedStyle),
			tui.Text("%s", typeName).Style(nodeStyle),
			tui.Text(" (%d stmts)", len(n.Stmts)).Style(mutedStyle),
		))
		for i, stmt := range n.Stmts {
			printNode(stmt, childIndent, i == len(n.Stmts)-1)
		}

	case *ast.List:
		printLine(tui.Group(
			tui.Text("%s%s", indent, connector).Style(mutedStyle),
			tui.Text("%s", typeName).Style(nodeStyle),
			tui.Text(" (%d items)", len(n.Items)).Style(mutedStyle),
		))
		for i, item := range n.Items {
			printNode(item, childIndent, i == len(n.Items)-1)
		}

	case *ast.Map:
		printLine(tui.Group(
			tui.Text("%s%s", indent, connector).Style(mutedStyle),
			tui.Text("%s", typeName).Style(nodeStyle),
			tui.Text(" (%d pairs)", len(n.Items)).Style(mutedStyle),
		))
		for i, pair := range n.Items {
			isLastPair := i == len(n.Items)-1
			pairConnector := "├─ "
			pairIndent := childIndent + "│  "
			if isLastPair {
				pairConnector = "└─ "
				pairIndent = childIndent + "   "
			}
			// Print key
			if pair.Key != nil {
				if ident, ok := pair.Key.(*ast.Ident); ok {
					printLine(tui.Group(
						tui.Text("%s%s", childIndent, pairConnector).Style(mutedStyle),
						tui.Text("%s", ident.Name).Style(fieldStyle),
						tui.Text(":").Style(mutedStyle),
					))
				} else {
					printLine(tui.Group(
						tui.Text("%s%s", childIndent, pairConnector).Style(mutedStyle),
						tui.Text("[computed]:").Style(fieldStyle),
					))
					printNode(pair.Key, pairIndent, false)
				}
			} else {
				printLine(tui.Group(
					tui.Text("%s%s", childIndent, pairConnector).Style(mutedStyle),
					tui.Text("[shorthand]").Style(fieldStyle),
				))
			}
			printNode(pair.Value, pairIndent, true)
		}

	default:
		// Generic handler for other node types
		printLine(tui.Group(
			tui.Text("%s%s", indent, connector).Style(mutedStyle),
			tui.Text("%s", typeName).Style(nodeStyle),
		))
		// Use reflection to find and print child nodes
		printChildrenReflect(node, childIndent)
	}
}

// printChildrenReflect uses reflection to find and print child nodes
func printChildrenReflect(node ast.Node, indent string) {
	v := reflect.ValueOf(node)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return
	}

	var children []struct {
		name string
		node ast.Node
	}

	t := v.Type()
	for i := 0; i < v.NumField(); i++ {
		field := v.Field(i)
		fieldName := t.Field(i).Name

		// Skip non-exported fields and position fields
		if !field.CanInterface() || fieldName == "From" || fieldName == "To" {
			continue
		}

		// Check if field is a Node
		if n, ok := field.Interface().(ast.Node); ok && n != nil {
			children = append(children, struct {
				name string
				node ast.Node
			}{fieldName, n})
		}

		// Check if field is a slice of Nodes
		if field.Kind() == reflect.Slice {
			for j := 0; j < field.Len(); j++ {
				elem := field.Index(j)
				if n, ok := elem.Interface().(ast.Node); ok && n != nil {
					children = append(children, struct {
						name string
						node ast.Node
					}{fmt.Sprintf("%s[%d]", fieldName, j), n})
				}
			}
		}
	}

	for i, child := range children {
		printLine(tui.Group(
			tui.Text("%s", indent).Style(mutedStyle),
			tui.Text("%s: ", child.name).Style(fieldStyle),
		))
		printNode(child.node, indent, i == len(children)-1)
	}
}
