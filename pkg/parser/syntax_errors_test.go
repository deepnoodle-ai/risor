package parser

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/deepnoodle-ai/risor/v2/pkg/ast"
	"github.com/deepnoodle-ai/wonton/assert"
)

// parseErrors parses the input and returns the errors it reports.
func parseErrors(t *testing.T, input string) []ParserError {
	t.Helper()
	_, err := Parse(context.Background(), input, nil)
	assert.NotNil(t, err)
	errs, ok := err.(*Errors)
	assert.True(t, ok)
	return errs.Errors()
}

// A lexer error is reported once, at the bad input, and parsing continues
// after it.
func TestLexerErrorReportedOnceAtPosition(t *testing.T) {
	tests := []struct {
		input   string
		message string
		line    int
		column  int
	}{
		{"x = 1\ny = ~2", "unexpected character: '~'", 2, 5},
		{"let a = 1\nlet b = 12abc", "invalid decimal literal: 12abc", 2, 9},
		{"x\n~", "unexpected character: '~'", 2, 1},
		{"x\n~\ny", "unexpected character: '~'", 2, 1},
		// A lexer error after a newline is not reported by the lookahead for
		// a chained method call
		{"x = a\n  .b\n~", "unexpected character: '~'", 3, 1},
		{"let x = foo\n12abc", "invalid decimal literal: 12abc", 2, 1},
		// A lexer error read by a lookahead inside parentheses is kept, so
		// the parser adds no misleading error on the newline before it
		{"x = (a\n~)", "unexpected character: '~'", 2, 1},
		{"f(a\n~)", "unexpected character: '~'", 2, 1},
		{"x = 1\n/* oops", "unterminated multi-line comment", 2, 1},
		{"x = 1 /* oops\n y", "unterminated multi-line comment", 1, 7},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			errs := parseErrors(t, tt.input)
			assert.Len(t, errs, 1)
			assert.Equal(t, errs[0].Error(), "syntax error: "+tt.message)
			assert.Equal(t, errs[0].StartPosition().LineNumber(), tt.line)
			assert.Equal(t, errs[0].StartPosition().ColumnNumber(), tt.column)
		})
	}
}

// A lexer error kept from a lookahead does not hide a different lexer error
// later in the input.
func TestLexerErrorAfterLookaheadKeepsLaterErrors(t *testing.T) {
	errs := parseErrors(t, "x = (a\n~)\ny = ~")
	assert.Len(t, errs, 2)
	assert.Equal(t, errs[0].StartPosition().LineNumber(), 2)
	assert.Equal(t, errs[0].StartPosition().ColumnNumber(), 1)
	assert.Equal(t, errs[1].StartPosition().LineNumber(), 3)
	assert.Equal(t, errs[1].StartPosition().ColumnNumber(), 5)
}

// A bad token inside an expression is reported once, by the lexer, without
// follow-on parse errors.
func TestLexerErrorInExpressionReportedOnce(t *testing.T) {
	tests := []struct {
		input  string
		line   int
		column int
	}{
		{"x = 1 + ~", 1, 9},
		{"1 +\n@", 2, 1},
		{"x.y = ~", 1, 7},
		{"f(1, ~)", 1, 6},
		{"let x = [1, 2abc]", 1, 13},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			errs := parseErrors(t, tt.input)
			assert.Len(t, errs, 1)
			_, ok := errs[0].(*SyntaxError)
			assert.True(t, ok)
			assert.Equal(t, errs[0].StartPosition().LineNumber(), tt.line)
			assert.Equal(t, errs[0].StartPosition().ColumnNumber(), tt.column)
		})
	}
}

// Lexer errors stop at MaxErrors, and a long line of bad input parses fast.
func TestLexerErrorsRespectMaxErrors(t *testing.T) {
	for _, input := range []string{
		"x = " + strings.Repeat("~", 100_000),
		"x = 1 " + strings.Repeat("1a ", 35_000),
		strings.Repeat("~\n", 50_000),
	} {
		start := time.Now()
		errs := parseErrors(t, input)
		assert.True(t, len(errs) <= MaxErrors)
		assert.True(t, time.Since(start) < time.Second)
	}
}

func TestLexerErrorThenParseError(t *testing.T) {
	errs := parseErrors(t, "x = ~\nlet = 2")
	assert.Greater(t, len(errs), 1)
	assert.Equal(t, errs[0].StartPosition().LineNumber(), 1)
	assert.Equal(t, errs[1].StartPosition().LineNumber(), 2)
}

func TestMissingCommaInParameters(t *testing.T) {
	tests := []struct {
		input   string
		message string
	}{
		{"function(a b c) {}", "expected ',' or ')' in function parameters (got b)"},
		{"function(a = 1 b) {}", "expected ',' or ')' in function parameters (got b)"},
		{"function({a}, [b] c) {}", "expected ',' or ')' in function parameters (got c)"},
		{"function({a b}) {}", "expected ',' or '}' in destructuring pattern (got b)"},
		{"function({a: x b}) {}", "expected ',' or '}' in destructuring pattern (got b)"},
		{"function([a b]) {}", "expected ',' or ']' in array destructuring pattern (got b)"},
		{"function([a = 1 b]) {}", "expected ',' or ']' in array destructuring pattern (got b)"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			errs := parseErrors(t, tt.input)
			assert.Equal(t, errs[0].Message(), tt.message)
		})
	}
}

func TestParametersWithCommasAndNewlines(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"function(a, b,) {}", "function(a, b) {  }"},
		{"function(\n a,\n b\n) {}", "function(a, b) {  }"},
		{"function(\n a\n , b\n) {}", "function(a, b) {  }"},
		{"function({\n a,\n b: c = 2\n}, [\n d,\n e = 3,\n]) {}", "function({a, b: c = 2}, [d, e = 3]) {  }"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			program, err := Parse(context.Background(), tt.input, nil)
			assert.Nil(t, err)
			assert.Equal(t, program.String(), tt.expected)
		})
	}
}

func TestArrowParamRejectsCompoundAssign(t *testing.T) {
	for _, input := range []string{"f = (x += 1) => x", "f = (x, y -= 1) => x"} {
		t.Run(input, func(t *testing.T) {
			errs := parseErrors(t, input)
			assert.Equal(t, errs[0].Message(), "invalid arrow function parameter")
		})
	}
}

func TestArrowParamAliasWithDefault(t *testing.T) {
	program, err := Parse(context.Background(), "f = ({a: b = 1, c}) => b", nil)
	assert.Nil(t, err)
	fn := program.Stmts[0].(*ast.Assign).Value.(*ast.Func)
	param := fn.Params[0].(*ast.ObjectDestructureParam)
	assert.Len(t, param.Bindings, 2)
	assert.Equal(t, param.Bindings[0].Key, "a")
	assert.Equal(t, param.Bindings[0].Alias, "b")
	assert.Equal(t, param.Bindings[0].Default.String(), "1")
	assert.Equal(t, param.Bindings[1].Key, "c")

	// Outside of arrow function parameters, the default is an error
	for _, input := range []string{"x = {a: b = 1}", "x = ({a: b = 1})"} {
		errs := parseErrors(t, input)
		assert.Equal(t, errs[0].Message(), `default value for "b" is only allowed in arrow function destructuring parameters`)
	}
}

// The error for a non-ASCII name spans the name's runes, not its bytes.
func TestDefaultValueErrorSpansRunes(t *testing.T) {
	errs := parseErrors(t, "[ñame = 1]")
	start, end := errs[0].StartPosition(), errs[0].EndPosition()
	assert.Equal(t, start.Column, 1)
	assert.Equal(t, end.Column, 4) // inclusive: the last rune of "ñame"
}

// A match nested in a pattern or guard must not clobber the outer pattern
// context, and '=>' still starts an arrow function in its arm results.
func TestNestedMatchPatternContext(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{
			"match 1 { (match 2 { 2 => 3, _ => 3 }) => 4, _ => 5 }",
			"match 1 { match 2 { 2 => 3, _ => 3 } => 4, _ => 5 }",
		},
		{
			"match 1 { 1 if (match 2 { 2 => true, _ => false }) => 4, _ => 5 }",
			"match 1 { 1 if match 2 { 2 => true, _ => false } => 4, _ => 5 }",
		},
		{
			"match 1 { (match 2 { 2 => x => x, _ => 3 }) => 4, _ => 5 }",
			"match 1 { match 2 { 2 => function(x) { return x }, _ => 3 } => 4, _ => 5 }",
		},
		{
			"match 1 { (function() { return x => x })() => 4, _ => 5 }",
			"match 1 { function() { return function(x) { return x } }() => 4, _ => 5 }",
		},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			program, err := Parse(context.Background(), tt.input, nil)
			assert.Nil(t, err)
			assert.Equal(t, program.String(), tt.expected)
		})
	}
}

// A statement cut short by the end of the input reports an error, even when
// the inner parser that failed did not report one.
func TestStatementAtEOFReportsError(t *testing.T) {
	for _, input := range []string{"return if (", "return if (\n", "x = 1\nreturn if ("} {
		t.Run(input, func(t *testing.T) {
			errs := parseErrors(t, input)
			assert.Len(t, errs, 1)
			assert.Contains(t, errs[0].Message(), "unexpected end of file")
		})
	}
}
