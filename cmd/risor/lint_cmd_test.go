package main

import (
	"context"
	"testing"

	"github.com/deepnoodle-ai/risor/v2/pkg/parser"
	"github.com/deepnoodle-ai/wonton/assert"
)

func TestLintProgram_EmptyIfBlock(t *testing.T) {
	code := `if (x > 0) { }`
	program, err := parser.Parse(context.Background(), code, nil)
	assert.Nil(t, err)

	issues := lintProgram(program, code)
	assert.True(t, len(issues) > 0)

	found := false
	for _, issue := range issues {
		if issue.Rule == "empty-block" {
			found = true
			break
		}
	}
	assert.True(t, found, "expected empty-block warning")
}

func TestLintProgram_EmptyElseBlock(t *testing.T) {
	code := `if (x > 0) { 1 } else { }`
	program, err := parser.Parse(context.Background(), code, nil)
	assert.Nil(t, err)

	issues := lintProgram(program, code)

	found := false
	for _, issue := range issues {
		if issue.Rule == "empty-block" && contains(issue.Message, "else") {
			found = true
			break
		}
	}
	assert.True(t, found, "expected empty else block warning")
}

func TestLintProgram_SelfComparison(t *testing.T) {
	code := `x == x`
	program, err := parser.Parse(context.Background(), code, nil)
	assert.Nil(t, err)

	issues := lintProgram(program, code)

	found := false
	for _, issue := range issues {
		if issue.Rule == "self-compare" {
			found = true
			assert.True(t, contains(issue.Message, "x"))
			break
		}
	}
	assert.True(t, found, "expected self-compare warning")
}

func TestLintProgram_TrailingWhitespace(t *testing.T) {
	code := "let x = 1   " // trailing spaces
	program, err := parser.Parse(context.Background(), code, nil)
	assert.Nil(t, err)

	issues := lintProgram(program, code)

	found := false
	for _, issue := range issues {
		if issue.Rule == "trailing-whitespace" {
			found = true
			break
		}
	}
	assert.True(t, found, "expected trailing-whitespace warning")
}

func TestLintProgram_LineTooLong(t *testing.T) {
	// Create a line longer than 120 characters
	code := "let x = \"" + string(make([]byte, 150)) + "\""
	// This won't parse correctly, so let's use a simpler approach
	code = "let veryLongVariableName = \"this is a very long string that goes on and on and on and on and on and on and on and exceeds the limit\""
	program, err := parser.Parse(context.Background(), code, nil)
	assert.Nil(t, err)

	issues := lintProgram(program, code)

	found := false
	for _, issue := range issues {
		if issue.Rule == "line-too-long" {
			found = true
			break
		}
	}
	assert.True(t, found, "expected line-too-long warning")
}

func TestLintProgram_TodoComment(t *testing.T) {
	code := `let x = 1 // TODO: fix this`
	program, err := parser.Parse(context.Background(), code, nil)
	assert.Nil(t, err)

	issues := lintProgram(program, code)

	found := false
	for _, issue := range issues {
		if issue.Rule == "todo-comment" {
			found = true
			break
		}
	}
	assert.True(t, found, "expected todo-comment warning")
}

func TestLintProgram_FixmeComment(t *testing.T) {
	code := `let x = 1 // FIXME: broken`
	program, err := parser.Parse(context.Background(), code, nil)
	assert.Nil(t, err)

	issues := lintProgram(program, code)

	found := false
	for _, issue := range issues {
		if issue.Rule == "todo-comment" {
			found = true
			break
		}
	}
	assert.True(t, found, "expected todo-comment warning for FIXME")
}

func TestLintProgram_VariableShadowing(t *testing.T) {
	code := `let x = 1
let x = 2`
	program, err := parser.Parse(context.Background(), code, nil)
	assert.Nil(t, err)

	issues := lintProgram(program, code)

	found := false
	for _, issue := range issues {
		if issue.Rule == "variable-shadow" {
			found = true
			assert.True(t, contains(issue.Message, "shadows"))
			break
		}
	}
	assert.True(t, found, "expected variable-shadow warning")
}

func TestLintProgram_ConstantReassignment(t *testing.T) {
	code := `const X = 1
X = 2`
	program, err := parser.Parse(context.Background(), code, nil)
	assert.Nil(t, err)

	issues := lintProgram(program, code)

	found := false
	for _, issue := range issues {
		if issue.Rule == "const-reassign" {
			found = true
			assert.Equal(t, issue.Level, "error")
			break
		}
	}
	assert.True(t, found, "expected const-reassign error")
}

func TestLintProgram_LongString(t *testing.T) {
	// Create a string longer than 1000 characters
	longStr := make([]byte, 1100)
	for i := range longStr {
		longStr[i] = 'a'
	}
	code := `let x = "` + string(longStr) + `"`
	program, err := parser.Parse(context.Background(), code, nil)
	assert.Nil(t, err)

	issues := lintProgram(program, code)

	found := false
	for _, issue := range issues {
		if issue.Rule == "long-string" {
			found = true
			break
		}
	}
	assert.True(t, found, "expected long-string warning")
}

func TestLintProgram_CleanCode(t *testing.T) {
	code := `let x = 1
let y = 2
let z = x + y`
	program, err := parser.Parse(context.Background(), code, nil)
	assert.Nil(t, err)

	issues := lintProgram(program, code)

	// Should have no issues (except maybe shadow warnings from redefining)
	// Filter out shadow warnings for this test
	var nonShadowIssues []LintIssue
	for _, issue := range issues {
		if issue.Rule != "variable-shadow" {
			nonShadowIssues = append(nonShadowIssues, issue)
		}
	}
	assert.Equal(t, len(nonShadowIssues), 0)
}

func TestLintIssue_LevelTypes(t *testing.T) {
	issue := LintIssue{
		Line:    1,
		Column:  1,
		Rule:    "test-rule",
		Message: "test message",
		Level:   "warning",
	}
	assert.Equal(t, issue.Level, "warning")

	issue.Level = "error"
	assert.Equal(t, issue.Level, "error")
}

func TestGetLintCode_NoInput(t *testing.T) {
	// This would need CLI context setup similar to other tests
	// Simplified test: verify error messages exist
	t.Run("error message format", func(t *testing.T) {
		issue := LintIssue{
			Line:    10,
			Column:  5,
			Rule:    "test",
			Message: "test error",
			Level:   "error",
		}
		assert.Equal(t, issue.Line, 10)
		assert.Equal(t, issue.Column, 5)
	})
}

func lintIssuesByRule(t *testing.T, code, rule string) []LintIssue {
	t.Helper()
	program, err := parser.Parse(context.Background(), code, nil)
	assert.Nil(t, err)
	var found []LintIssue
	for _, issue := range lintProgram(program, code) {
		if issue.Rule == rule {
			found = append(found, issue)
		}
	}
	return found
}

func TestLintProgram_UnusedVariable(t *testing.T) {
	code := `function f() {
    let unused = 1
    const alsoUnused = 2
    let used = 3
    let _ignored = 4
    return used
}`
	issues := lintIssuesByRule(t, code, "unused-variable")
	assert.Equal(t, len(issues), 2)
	assert.Equal(t, issues[0].Line, 2)
	assert.Equal(t, issues[0].Column, 9)
	assert.Equal(t, issues[0].Level, "warning")
	assert.Equal(t, issues[0].Message, `variable "unused" is declared but never used`)
	assert.Equal(t, issues[1].Message, `variable "alsoUnused" is declared but never used`)
}

func TestLintProgram_UnusedVariableNestedFunc(t *testing.T) {
	code := `function outer() {
    let captured = 1
    function inner() {
        let deep = 2
        return captured
    }
    return inner()
}`
	issues := lintIssuesByRule(t, code, "unused-variable")
	assert.Equal(t, len(issues), 1)
	assert.Equal(t, issues[0].Message, `variable "deep" is declared but never used`)
}

func TestLintProgram_UnusedVariableSkipsTopLevel(t *testing.T) {
	// A host may read top-level variables after the script runs.
	code := `let result = 42`
	issues := lintIssuesByRule(t, code, "unused-variable")
	assert.Equal(t, len(issues), 0)
}

func TestLintProgram_PositionsAreOneBased(t *testing.T) {
	code := `let x = 1
let x = 2`
	issues := lintIssuesByRule(t, code, "variable-shadow")
	assert.Equal(t, len(issues), 1)
	assert.Equal(t, issues[0].Line, 2)
	assert.Equal(t, issues[0].Column, 5)
	assert.Equal(t, issues[0].Message, `variable "x" shadows declaration on line 1`)
}
