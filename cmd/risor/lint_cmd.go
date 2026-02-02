package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/deepnoodle-ai/wonton/cli"
	"github.com/deepnoodle-ai/wonton/tui"
	"github.com/deepnoodle-ai/risor/v2/pkg/ast"
	"github.com/deepnoodle-ai/risor/v2/pkg/parser"
)

// LintIssue represents a code quality issue
type LintIssue struct {
	Line    int
	Column  int
	Rule    string
	Message string
	Level   string // "warning" or "error"
}

func lintHandler(ctx *cli.Context) error {
	// Get code from -c flag, --stdin, or file argument
	code, filename, err := getLintCode(ctx)
	if err != nil {
		return err
	}

	if filename == "" {
		filename = "<stdin>"
	}

	outputFormat := ctx.String("output")

	// Parse the code
	program, parseErr := parser.Parse(context.Background(), code, nil)
	if parseErr != nil {
		// Report parse error as a lint issue
		issues := []LintIssue{{
			Line:    1,
			Column:  1,
			Rule:    "parse-error",
			Message: parseErr.Error(),
			Level:   "error",
		}}
		printLintResults(filename, issues, outputFormat)
		return nil
	}

	// Run linting checks
	issues := lintProgram(program, code)

	// Print results
	printLintResults(filename, issues, outputFormat)

	if len(issues) > 0 {
		for _, issue := range issues {
			if issue.Level == "error" {
				os.Exit(1)
			}
		}
	}

	return nil
}

func getLintCode(ctx *cli.Context) (string, string, error) {
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

func lintProgram(program *ast.Program, source string) []LintIssue {
	var issues []LintIssue
	lines := strings.Split(source, "\n")

	// Track declared variables for unused detection
	declared := make(map[string]int)        // name -> line
	used := make(map[string]bool)           // name -> was used
	constants := make(map[string]bool)      // track constants for reassignment check
	shadowWarnings := make(map[string]bool) // prevent duplicate shadow warnings

	// Visit all nodes
	ast.Inspect(program, func(node ast.Node) bool {
		if node == nil {
			return true
		}

		switch n := node.(type) {
		case *ast.Var:
			name := n.Name.Name
			line := n.Name.Pos().Line

			// Check for shadowing
			if prevLine, exists := declared[name]; exists && !shadowWarnings[name] {
				issues = append(issues, LintIssue{
					Line:    line,
					Column:  n.Name.Pos().Column,
					Rule:    "variable-shadow",
					Message: fmt.Sprintf("variable %q shadows declaration on line %d", name, prevLine),
					Level:   "warning",
				})
				shadowWarnings[name] = true
			}
			declared[name] = line
			used[name] = false

		case *ast.Const:
			name := n.Name.Name
			line := n.Name.Pos().Line
			declared[name] = line
			used[name] = false
			constants[name] = true

		case *ast.Assign:
			if n.Name != nil {
				name := n.Name.Name
				// Check for constant reassignment
				if constants[name] {
					issues = append(issues, LintIssue{
						Line:    n.Name.Pos().Line,
						Column:  n.Name.Pos().Column,
						Rule:    "const-reassign",
						Message: fmt.Sprintf("cannot reassign constant %q", name),
						Level:   "error",
					})
				}
				used[name] = true
			}

		case *ast.Ident:
			used[n.Name] = true

		case *ast.Func:
			// Check for functions without return
			if n.Name != nil && n.Body != nil {
				hasReturn := false
				ast.Inspect(n.Body, func(inner ast.Node) bool {
					if _, ok := inner.(*ast.Return); ok {
						hasReturn = true
						return false
					}
					return true
				})
				// This is just informational, not a warning
				_ = hasReturn
			}

		case *ast.If:
			// Check for empty blocks
			if n.Consequence != nil && len(n.Consequence.Stmts) == 0 {
				issues = append(issues, LintIssue{
					Line:    n.Consequence.Pos().Line,
					Column:  n.Consequence.Pos().Column,
					Rule:    "empty-block",
					Message: "empty if block",
					Level:   "warning",
				})
			}
			if n.Alternative != nil && len(n.Alternative.Stmts) == 0 {
				issues = append(issues, LintIssue{
					Line:    n.Alternative.Pos().Line,
					Column:  n.Alternative.Pos().Column,
					Rule:    "empty-block",
					Message: "empty else block",
					Level:   "warning",
				})
			}

		case *ast.Infix:
			// Check for self-comparison
			if leftIdent, ok := n.X.(*ast.Ident); ok {
				if rightIdent, ok := n.Y.(*ast.Ident); ok {
					if leftIdent.Name == rightIdent.Name {
						issues = append(issues, LintIssue{
							Line:    n.Pos().Line,
							Column:  n.Pos().Column,
							Rule:    "self-compare",
							Message: fmt.Sprintf("comparing %q to itself", leftIdent.Name),
							Level:   "warning",
						})
					}
				}
			}

		case *ast.String:
			// Check for very long strings
			if len(n.Value) > 1000 {
				issues = append(issues, LintIssue{
					Line:    n.Pos().Line,
					Column:  n.Pos().Column,
					Rule:    "long-string",
					Message: fmt.Sprintf("string literal is very long (%d characters)", len(n.Value)),
					Level:   "warning",
				})
			}
		}

		return true
	})

	// Check line-level issues
	for i, line := range lines {
		lineNum := i + 1

		// Check for trailing whitespace
		if strings.HasSuffix(line, " ") || strings.HasSuffix(line, "\t") {
			issues = append(issues, LintIssue{
				Line:    lineNum,
				Column:  len(line),
				Rule:    "trailing-whitespace",
				Message: "trailing whitespace",
				Level:   "warning",
			})
		}

		// Check for very long lines
		if len(line) > 120 {
			issues = append(issues, LintIssue{
				Line:    lineNum,
				Column:  121,
				Rule:    "line-too-long",
				Message: fmt.Sprintf("line exceeds 120 characters (%d)", len(line)),
				Level:   "warning",
			})
		}

		// Check for TODO/FIXME comments
		if strings.Contains(line, "TODO") || strings.Contains(line, "FIXME") {
			issues = append(issues, LintIssue{
				Line:    lineNum,
				Column:  1,
				Rule:    "todo-comment",
				Message: "TODO/FIXME comment found",
				Level:   "warning",
			})
		}
	}

	return issues
}

func printLintResults(filename string, issues []LintIssue, outputFormat string) {
	if outputFormat == "json" {
		printLintResultsJSON(filename, issues)
		return
	}

	warnStyle := tui.NewStyle().WithFgRGB(tui.RGB{R: 255, G: 200, B: 80})
	errorStyle := tui.NewStyle().WithFgRGB(tui.RGB{R: 255, G: 100, B: 100})
	fileStyle := tui.NewStyle().WithFgRGB(tui.RGB{R: 100, G: 200, B: 255})
	ruleStyle := tui.NewStyle().WithFgRGB(tui.RGB{R: 180, G: 140, B: 220})
	okStyle := tui.NewStyle().WithFgRGB(tui.RGB{R: 100, G: 220, B: 100})

	if len(issues) == 0 {
		fmt.Println(tui.Sprint(tui.Group(
			tui.Text("%s: ", filename).Style(fileStyle),
			tui.Text("OK").Style(okStyle),
		)))
		return
	}

	for _, issue := range issues {
		levelStyle := warnStyle
		levelText := "warning"
		if issue.Level == "error" {
			levelStyle = errorStyle
			levelText = "error"
		}

		fmt.Println(tui.Sprint(tui.Group(
			tui.Text("%s:%d:%d: ", filename, issue.Line, issue.Column).Style(fileStyle),
			tui.Text("%s", levelText).Style(levelStyle),
			tui.Text(" [%s]", issue.Rule).Style(ruleStyle),
			tui.Text(" %s", issue.Message),
		)))
	}

	// Summary
	warnings := 0
	errs := 0
	for _, issue := range issues {
		if issue.Level == "error" {
			errs++
		} else {
			warnings++
		}
	}

	fmt.Println()
	if errs > 0 {
		fmt.Println(tui.Sprint(tui.Text("%d error(s), %d warning(s)", errs, warnings).Style(errorStyle)))
	} else {
		fmt.Println(tui.Sprint(tui.Text("%d warning(s)", warnings).Style(warnStyle)))
	}
}

func printLintResultsJSON(filename string, issues []LintIssue) {
	result := struct {
		File     string      `json:"file"`
		Issues   []LintIssue `json:"issues"`
		Errors   int         `json:"errors"`
		Warnings int         `json:"warnings"`
	}{
		File:   filename,
		Issues: issues,
	}

	for _, issue := range issues {
		if issue.Level == "error" {
			result.Errors++
		} else {
			result.Warnings++
		}
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	enc.Encode(result)
}
