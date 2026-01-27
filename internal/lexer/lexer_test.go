package lexer

import (
	"fmt"
	"testing"

	"github.com/deepnoodle-ai/wonton/assert"
	"github.com/risor-io/risor/internal/token"
)

func TestNil(t *testing.T) {
	input := "a = nil;"
	tests := []struct {
		expectedType    token.Type
		expectedLiteral string
	}{
		{token.IDENT, "a"},
		{token.ASSIGN, "="},
		{token.NIL, "nil"},
		{token.SEMICOLON, ";"},
		{token.EOF, ""},
	}
	l := New(input)
	for i, tt := range tests {
		tok, err := l.Next()
		assert.Nil(t, err)
		if tok.Type != tt.expectedType {
			t.Fatalf("tests[%d] - tokentype wrong, expected=%q, got=%q", i, tt.expectedType, tok.Type)
		}
		if tok.Literal != tt.expectedLiteral {
			t.Fatalf("tests[%d] - Literal wrong, expected=%q, got=%q", i, tt.expectedLiteral, tok.Literal)
		}
	}
}

func TestNextToken1(t *testing.T) {
	input := "%=+(){},;?|| &&`/foo`++--***=..&"

	tests := []struct {
		expectedType    token.Type
		expectedLiteral string
	}{
		{token.MOD, "%"},
		{token.ASSIGN, "="},
		{token.PLUS, "+"},
		{token.LPAREN, "("},
		{token.RPAREN, ")"},
		{token.LBRACE, "{"},
		{token.RBRACE, "}"},
		{token.COMMA, ","},
		{token.SEMICOLON, ";"},
		{token.QUESTION, "?"},
		{token.OR, "||"},
		{token.AND, "&&"},
		{token.TEMPLATE, "/foo"},
		{token.PLUS_PLUS, "++"},
		{token.MINUS_MINUS, "--"},
		{token.POW, "**"},
		{token.ASTERISK_EQUALS, "*="},
		{token.PERIOD, "."},
		{token.PERIOD, "."},
		{token.AMPERSAND, "&"},
		{token.EOF, ""},
	}
	l := New(input)
	for i, tt := range tests {
		tok, err := l.Next()
		assert.Nil(t, err)
		if tok.Type != tt.expectedType {
			t.Fatalf("tests[%d] - tokentype wrong, expected=%q, got=%q", i, tt.expectedType, tok.Type)
		}
		if tok.Literal != tt.expectedLiteral {
			t.Fatalf("tests[%d] - Literal wrong, expected=%q, got=%q", i, tt.expectedLiteral, tok.Literal)
		}
	}
}

func TestNextToken2(t *testing.T) {
	input := `let five=5;
let ten =10;
let add = function(x, y){
  x+y
};
let result = add(five, ten);
!- *5;
5<10>5;

if(5<10){
	return true;
}else{
	return false;
}
10 == 10;
10 != 9;
"foobar"
"foo bar"
[1,2];
{"foo":"bar"}
1.2
0.5
0.3
世界
2 >= 1
1 <= 3
`
	tests := []struct {
		expectedType    token.Type
		expectedLiteral string
	}{
		{token.LET, "let"},
		{token.IDENT, "five"},
		{token.ASSIGN, "="},
		{token.INT, "5"},
		{token.SEMICOLON, ";"},
		{token.NEWLINE, "\n"},
		{token.LET, "let"},
		{token.IDENT, "ten"},
		{token.ASSIGN, "="},
		{token.INT, "10"},
		{token.SEMICOLON, ";"},
		{token.NEWLINE, "\n"},
		{token.LET, "let"},
		{token.IDENT, "add"},
		{token.ASSIGN, "="},
		{token.FUNCTION, "function"},
		{token.LPAREN, "("},
		{token.IDENT, "x"},
		{token.COMMA, ","},
		{token.IDENT, "y"},
		{token.RPAREN, ")"},
		{token.LBRACE, "{"},
		{token.NEWLINE, "\n"},
		{token.IDENT, "x"},
		{token.PLUS, "+"},
		{token.IDENT, "y"},
		{token.NEWLINE, "\n"},
		{token.RBRACE, "}"},
		{token.SEMICOLON, ";"},
		{token.NEWLINE, "\n"},
		{token.LET, "let"},
		{token.IDENT, "result"},
		{token.ASSIGN, "="},
		{token.IDENT, "add"},
		{token.LPAREN, "("},
		{token.IDENT, "five"},
		{token.COMMA, ","},
		{token.IDENT, "ten"},
		{token.RPAREN, ")"},
		{token.SEMICOLON, ";"},
		{token.NEWLINE, "\n"},
		{token.BANG, "!"},
		{token.MINUS, "-"},
		{token.ASTERISK, "*"},
		{token.INT, "5"},
		{token.SEMICOLON, ";"},
		{token.NEWLINE, "\n"},
		{token.INT, "5"},
		{token.LT, "<"},
		{token.INT, "10"},
		{token.GT, ">"},
		{token.INT, "5"},
		{token.SEMICOLON, ";"},
		{token.NEWLINE, "\n"},
		{token.NEWLINE, "\n"},
		{token.IF, "if"},
		{token.LPAREN, "("},
		{token.INT, "5"},
		{token.LT, "<"},
		{token.INT, "10"},
		{token.RPAREN, ")"},
		{token.LBRACE, "{"},
		{token.NEWLINE, "\n"},
		{token.RETURN, "return"},
		{token.TRUE, "true"},
		{token.SEMICOLON, ";"},
		{token.NEWLINE, "\n"},
		{token.RBRACE, "}"},
		{token.ELSE, "else"},
		{token.LBRACE, "{"},
		{token.NEWLINE, "\n"},
		{token.RETURN, "return"},
		{token.FALSE, "false"},
		{token.SEMICOLON, ";"},
		{token.NEWLINE, "\n"},
		{token.RBRACE, "}"},
		{token.NEWLINE, "\n"},
		{token.INT, "10"},
		{token.EQ, "=="},
		{token.INT, "10"},
		{token.SEMICOLON, ";"},
		{token.NEWLINE, "\n"},
		{token.INT, "10"},
		{token.NOT_EQ, "!="},
		{token.INT, "9"},
		{token.SEMICOLON, ";"},
		{token.NEWLINE, "\n"},
		{token.STRING, "foobar"},
		{token.NEWLINE, "\n"},
		{token.STRING, "foo bar"},
		{token.NEWLINE, "\n"},
		{token.LBRACKET, "["},
		{token.INT, "1"},
		{token.COMMA, ","},
		{token.INT, "2"},
		{token.RBRACKET, "]"},
		{token.SEMICOLON, ";"},
		{token.NEWLINE, "\n"},
		{token.LBRACE, "{"},
		{token.STRING, "foo"},
		{token.COLON, ":"},
		{token.STRING, "bar"},
		{token.RBRACE, "}"},
		{token.NEWLINE, "\n"},
		{token.FLOAT, "1.2"},
		{token.NEWLINE, "\n"},
		{token.FLOAT, "0.5"},
		{token.NEWLINE, "\n"},
		{token.FLOAT, "0.3"},
		{token.NEWLINE, "\n"},
		{token.IDENT, "世界"},
		{token.NEWLINE, "\n"},
		{token.INT, "2"},
		{token.GT_EQUALS, ">="},
		{token.INT, "1"},
		{token.NEWLINE, "\n"},
		{token.INT, "1"},
		{token.LT_EQUALS, "<="},
		{token.INT, "3"},
		{token.NEWLINE, "\n"},
		{token.EOF, ""},
	}
	l := New(input)
	for i, tt := range tests {
		tok, err := l.Next()
		assert.Nil(t, err)
		if tok.Type != tt.expectedType {
			fmt.Println(tok.Literal)
			t.Fatalf("tests[%d] - tokentype wrong, expected=%q, got=%q", i, tt.expectedType, tok.Type)
		}
		if tok.Literal != tt.expectedLiteral {
			t.Fatalf("tests[%d] - Literal wrong, expected=%q, got=%q", i, tt.expectedLiteral, tok.Literal)
		}
	}
}

func TestUnicodeLexer(t *testing.T) {
	input := `世界`
	l := New(input)
	tok, err := l.Next()
	assert.Nil(t, err)
	if tok.Type != token.IDENT {
		t.Fatalf("token type wrong, expected=%q, got=%q", token.IDENT, tok.Type)
	}
	if tok.Literal != "世界" {
		t.Fatalf("token literal wrong, expected=%q, got=%q", "世界", tok.Literal)
	}
}

func TestString(t *testing.T) {
	input := `"\n\r\t\\\""`

	tests := []struct {
		expectedType    token.Type
		expectedLiteral string
	}{
		{token.STRING, "\n\r\t\\\""},
		{token.EOF, ""},
	}
	l := New(input)
	for i, tt := range tests {
		tok, err := l.Next()
		assert.Nil(t, err)
		if tok.Type != tt.expectedType {
			t.Fatalf("tests[%d] - tokentype wrong, expected=%q, got=%q", i, tt.expectedType, tok.Type)
		}
		if tok.Literal != tt.expectedLiteral {
			t.Fatalf("tests[%d] - Literal wrong, expected=%q, got=%q", i, tt.expectedLiteral, tok.Literal)
		}
	}
}

func TestSimpleComment(t *testing.T) {
	input := `=+// This is a comment
// This is still a comment
let a = 1;
// This is a final
// comment on two-lines`

	tests := []struct {
		expectedType    token.Type
		expectedLiteral string
	}{
		{token.ASSIGN, "="},
		{token.PLUS, "+"},
		{token.NEWLINE, "\n"},
		{token.NEWLINE, "\n"},
		{token.LET, "let"},
		{token.IDENT, "a"},
		{token.ASSIGN, "="},
		{token.INT, "1"},
		{token.SEMICOLON, ";"},
		{token.NEWLINE, "\n"},
		{token.NEWLINE, "\n"},
		{token.EOF, ""},
	}
	l := New(input)
	for i, tt := range tests {
		tok, err := l.Next()
		assert.Nil(t, err)
		if tok.Type != tt.expectedType {
			t.Fatalf("tests[%d] - tokentype wrong, expected=%q, got=%q", i, tt.expectedType, tok.Type)
		}
		if tok.Literal != tt.expectedLiteral {
			t.Fatalf("tests[%d] - Literal wrong, expected=%q, got=%q", i, tt.expectedLiteral, tok.Literal)
		}
	}
}

func TestMultiLineComment(t *testing.T) {
	input := `=+/* This is a comment

We're still in a comment
let c = 2; */
let a = 1;
// This isa comment
// This is still a comment.
/* Now a multi-line again
   Which is two-lines
 */`

	tests := []struct {
		expectedType    token.Type
		expectedLiteral string
	}{
		{token.ASSIGN, "="},
		{token.PLUS, "+"},
		{token.NEWLINE, "\n"},
		{token.LET, "let"},
		{token.IDENT, "a"},
		{token.ASSIGN, "="},
		{token.INT, "1"},
		{token.SEMICOLON, ";"},
		{token.NEWLINE, "\n"},
		{token.NEWLINE, "\n"},
		{token.NEWLINE, "\n"},
		{token.EOF, ""},
	}
	l := New(input)
	for i, tt := range tests {
		tok, err := l.Next()
		assert.Nil(t, err)
		if tok.Type != tt.expectedType {
			t.Fatalf("tests[%d] - tokentype wrong, expected=%q, got=%q", i, tt.expectedType, tok.Type)
		}
		if tok.Literal != tt.expectedLiteral {
			t.Fatalf("tests[%d] - Literal wrong, expected=%q, got=%q", i, tt.expectedLiteral, tok.Literal)
		}
	}
}

func TestIntegers(t *testing.T) {
	input := `10 0x10 0xF0 0xFE 00101 0xFF 0101 0xFF;`

	tests := []struct {
		expectedType    token.Type
		expectedLiteral string
	}{
		{token.INT, "10"},
		{token.INT, "0x10"},
		{token.INT, "0xF0"},
		{token.INT, "0xFE"},
		{token.INT, "00101"},
		{token.INT, "0xFF"},
		{token.INT, "0101"},
		{token.INT, "0xFF"},
		{token.SEMICOLON, ";"},
		{token.EOF, ""},
	}
	l := New(input)
	for i, tt := range tests {
		tok, err := l.Next()
		assert.Nil(t, err)
		if tok.Type != tt.expectedType {
			t.Fatalf("tests[%d] - tokentype wrong, expected=%q, got=%q", i, tt.expectedType, tok.Type)
		}
		if tok.Literal != tt.expectedLiteral {
			t.Fatalf("tests[%d] - Literal wrong, expected=%q, got=%q", i, tt.expectedLiteral, tok.Literal)
		}
	}
}

func TestInvalidIntegers(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"42.foo()", "invalid decimal literal: 42.f"},
		{"12ab", "invalid decimal literal: 12a"},
		{"0x1aZ", "invalid decimal literal: 0x1aZ"},
		{"078", "invalid decimal literal: 078"},
	}
	for _, tt := range tests {
		l := New(tt.input)
		tok, err := l.Next()
		fmt.Println(tok, err)
		assert.NotNil(t, err)
		assert.Equal(t, err.Error(), tt.expected)
	}
}

// Test that the shebang-line is handled specially.
func TestShebang(t *testing.T) {
	input := `#!/bin/risor
10;`

	tests := []struct {
		expectedType    token.Type
		expectedLiteral string
	}{
		{token.NEWLINE, "\n"},
		{token.INT, "10"},
		{token.SEMICOLON, ";"},
		{token.EOF, ""},
	}
	l := New(input)
	for i, tt := range tests {
		tok, err := l.Next()
		assert.Nil(t, err)
		if tok.Type != tt.expectedType {
			t.Fatalf("tests[%d] - tokentype wrong, expected=%q, got=%q", i, tt.expectedType, tok.Type)
		}
		if tok.Literal != tt.expectedLiteral {
			t.Fatalf("tests[%d] - Literal wrong, expected=%q, got=%q", i, tt.expectedLiteral, tok.Literal)
		}
	}
}

// TestMoreHandling does nothing real, but it bumps our coverage!
func TestMoreHandling(t *testing.T) {
	input := `#!/bin/monkey
1 += 1;
2 -= 2;
3 /= 3;
x */ 3;

var t = true;
var f = false;

if ( t && f ) { puts( "What?" ); }
if ( t || f ) { puts( "What?" ); }

var a = 1;
a++;

var b = a % 1;
b--;
b -= 2;

if ( a<3 ) { puts( "Blah!"); }
if ( a>3 ) { puts( "Blah!"); }

var b = 3;
b**b;
b *= 3;
if ( b <= 3  ) { puts "blah\n" }
if ( b >= 3  ) { puts "blah\n" }

var a = "steve";
var a = "steve\n";
var a = "steve\t";
var a = "steve\r";
var a = "steve\\";
var a = "steve\"";
var c = 3.113;
.;`
	l := New(input)
	tok, _ := l.Next()
	for tok.Type != token.EOF {
		tok, _ = l.Next()
	}
}

// TestDotMethod ensures that identifiers are parsed correctly for the
// case where we need to split at periods.
func TestDotMethod(t *testing.T) {
	input := `
foo.bar();
baz.qux();
`

	tests := []struct {
		expectedType    token.Type
		expectedLiteral string
	}{
		{token.NEWLINE, "\n"},
		{token.IDENT, "foo"},
		{token.PERIOD, "."},
		{token.IDENT, "bar"},
		{token.LPAREN, "("},
		{token.RPAREN, ")"},
		{token.SEMICOLON, ";"},
		{token.NEWLINE, "\n"},
		{token.IDENT, "baz"},
		{token.PERIOD, "."},
		{token.IDENT, "qux"},
		{token.LPAREN, "("},
		{token.RPAREN, ")"},
		{token.SEMICOLON, ";"},
		{token.NEWLINE, "\n"},
		{token.EOF, ""},
	}
	l := New(input)
	for i, tt := range tests {
		tok, err := l.Next()
		assert.Nil(t, err)
		if tok.Type != tt.expectedType {
			t.Fatalf("tests[%d] - tokentype wrong, expected=%q, got=%q", i, tt.expectedType, tok.Type)
		}
		if tok.Literal != tt.expectedLiteral {
			t.Fatalf("tests[%d] - Literal wrong, expected=%q, got=%q", i, tt, tok)
		}
	}
}

// TestDiv is designed to test that a division is recognized; that it is
// not confused with a regular-expression.
func TestDiv(t *testing.T) {
	input := `a = b / c;
a = 3/4;`

	tests := []struct {
		expectedType    token.Type
		expectedLiteral string
	}{
		{token.IDENT, "a"},
		{token.ASSIGN, "="},
		{token.IDENT, "b"},
		{token.SLASH, "/"},
		{token.IDENT, "c"},
		{token.SEMICOLON, ";"},
		{token.NEWLINE, "\n"},
		{token.IDENT, "a"},
		{token.ASSIGN, "="},
		{token.INT, "3"},
		{token.SLASH, "/"},
		{token.INT, "4"},
		{token.SEMICOLON, ";"},
		{token.EOF, ""},
	}
	l := New(input)
	for i, tt := range tests {
		tok, _ := l.Next()
		if tok.Type != tt.expectedType {
			t.Fatalf("tests[%d] - tokentype wrong, expected=%q, got=%q", i, tt.expectedType, tok.Type)
		}
		if tok.Literal != tt.expectedLiteral {
			t.Fatalf("tests[%d] - Literal wrong, expected=%q, got=%q", i, tt.expectedLiteral, tok.Literal)
		}
	}
}

func TestLineNumbers(t *testing.T) {
	l := New("ab + cd\n foo+=111")
	tests := []struct {
		expectedType     token.Type
		expectedLiteral  string
		expectedLine     int
		expectedStartPos int
		expectedEndPos   int
	}{
		{token.IDENT, "ab", 0, 0, 1},
		{token.PLUS, "+", 0, 3, 3},
		{token.IDENT, "cd", 0, 5, 6},
		{token.NEWLINE, "\n", 0, 7, 7},
		{token.IDENT, "foo", 1, 1, 3},
		{token.PLUS_EQUALS, "+=", 1, 4, 5},
		{token.INT, "111", 1, 6, 8},
		{token.EOF, "", 1, 9, 9},
	}
	for i, tt := range tests {
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			tok, err := l.Next()
			assert.Nil(t, err)
			assert.Equal(t, tok.Type, tt.expectedType)
			assert.Equal(t, tok.Literal, tt.expectedLiteral)
			// require.Equal(t, tt.expectedLine, tok.Line) // FIXME
			assert.Equal(t, tok.StartPosition.Column, tt.expectedStartPos)
			assert.Equal(t, tok.EndPosition.Column, tt.expectedEndPos)
		})
	}
}

func TestTokenLengths(t *testing.T) {
	tests := []struct {
		input            string
		expectedType     token.Type
		expectedLiteral  string
		expectedLine     int
		expectedStartPos int
		expectedEndPos   int
	}{
		{"abc", token.IDENT, "abc", 0, 0, 2},
		{"111", token.INT, "111", 0, 0, 2},
		{"1.1", token.FLOAT, "1.1", 0, 0, 2},
		{`"b"`, token.STRING, "b", 0, 0, 2},
		{"let", token.LET, "let", 0, 0, 2},
		{"false", token.FALSE, "false", 0, 0, 4},
		{">=", token.GT_EQUALS, ">=", 0, 0, 1},
		{" \n", token.NEWLINE, "\n", 0, 1, 1},
		{" {", token.LBRACE, "{", 0, 1, 1},
		{" ++", token.PLUS_PLUS, "++", 0, 1, 2},
	}
	for i, tt := range tests {
		t.Run(fmt.Sprintf("%d-%s", i, tt.input), func(t *testing.T) {
			l := New(tt.input)
			tok, err := l.Next()
			assert.Nil(t, err)
			assert.Equal(t, tok.Type, tt.expectedType)
			assert.Equal(t, tok.Literal, tt.expectedLiteral)
			// require.Equal(t, tt.expectedLine, tok.Line) // FIXME
			assert.Equal(t, tok.StartPosition.Column, tt.expectedStartPos)
			assert.Equal(t, tok.EndPosition.Column, tt.expectedEndPos)
		})
	}
}

func TestStringTypes(t *testing.T) {
	tests := []struct {
		input           string
		expectedType    token.Type
		expectedLiteral string
	}{
		{`"\"foo'"`, token.STRING, "\"foo'"},
		{`'"foo\''`, token.STRING, "\"foo'"},
		{"`foo`", token.TEMPLATE, "foo"},
		{"\"\\nhey\"", token.STRING, "\nhey"},
	}
	for i, tt := range tests {
		t.Run(fmt.Sprintf("%d-%s", i, tt.input), func(t *testing.T) {
			l := New(tt.input)
			tok, err := l.Next()
			assert.Nil(t, err)
			assert.Equal(t, tok.Type, tt.expectedType)
			assert.Equal(t, tok.Literal, tt.expectedLiteral)
		})
	}
}

func TestIdentifiers(t *testing.T) {
	tests := []struct {
		input           string
		expectedType    token.Type
		expectedLiteral string
	}{
		{"abc", token.IDENT, "abc"},
		{"a1_", token.IDENT, "a1_"},
		{"__c__", token.IDENT, "__c__"},
		{" d-f ", token.IDENT, "d"},
		{" in ", token.IN, "in"},
		{"  ", token.EOF, ""},
	}
	for i, tt := range tests {
		t.Run(fmt.Sprintf("%d-%s", i, tt.input), func(t *testing.T) {
			l := New(tt.input)
			tok, err := l.Next()
			assert.Nil(t, err)
			assert.Equal(t, tok.Type, tt.expectedType)
			assert.Equal(t, tok.Literal, tt.expectedLiteral)
		})
	}
}

func TestInvalidIdentifiers(t *testing.T) {
	tests := []struct {
		input string
		err   string
	}{
		{"⺶", "invalid identifier: ⺶"},
		{"foo⺶bar", "invalid identifier: foo⺶"},
	}
	for i, tt := range tests {
		t.Run(fmt.Sprintf("%d-%s", i, tt.input), func(t *testing.T) {
			l := New(tt.input)
			_, err := l.Next()
			assert.NotNil(t, err)
			assert.Equal(t, err.Error(), tt.err)
		})
	}
}

func TestEscapeSequences(t *testing.T) {
	tests := []struct {
		name            string
		input           string
		expectedLiteral string
	}{
		{"alert", `"\a"`, "\a"},
		{"backspace", `"\b"`, "\b"},
		{"form feed", `"\f"`, "\f"},
		{"new line", `"\n"`, "\n"},
		{"carrige return", `"\r"`, "\r"},
		{"horizontal tab", `"\t"`, "\t"},
		{"vertical tab", `"\v"`, "\v"},
		{"backslash", `"\\"`, "\\"},
		{"escape", `"\e"`, "\x1B"},
		{"hex", `"\xFF"`, "ÿ"},
		{"unicode16", `"\u672C"`, "本"}, // No clue what it means. Found it here: https://go.dev/ref/spec#String_literals
		{"unicode32", `"\U0001F63C"`, "😼"},
		{"octal3", `"\300"`, "\300"},
		{"octal2", `"\241"`, "\241"},
		{"octal1", `"\141"`, "a"},
		{"octal0", `"\041"`, "!"},
		{"octalmax", `"\377"`, "\377"},
	}
	for i, tt := range tests {
		t.Run(fmt.Sprintf("%d-%s", i, tt.name), func(t *testing.T) {
			l := New(tt.input)
			tok, err := l.Next()
			assert.Nil(t, err)
			assert.Equal(t, tok.Type, token.STRING)
			assert.Equal(t, tok.Literal, tt.expectedLiteral)
		})
	}
}

func TestInvalidEscapeSequences(t *testing.T) {
	tests := []struct {
		input string
	}{
		{`"\P"`},     // unknown escape code
		{`"\u12_3"`}, // non-hex chars
		{`"\U1234"`}, // too few chars
		{`"\378"`},   // invalid char '8' in octal
	}
	for i, tt := range tests {
		t.Run(fmt.Sprintf("%d-%s", i, tt.input), func(t *testing.T) {
			l := New(tt.input)
			tok, err := l.Next()
			assert.Error(t, err, "Unexpected result: token=%s, literal=%q", tok.Type, tok.Literal)
		})
	}
}

func TestTokenLineText(t *testing.T) {
	l := New(` var x = 32; foo = bar
bar = baz
`)
	tok, err := l.Next()
	assert.Nil(t, err)
	fmt.Println(tok)

	line := l.GetLineText(tok)
	assert.Equal(t, line, " var x = 32; foo = bar")
}

func TestInvalids(t *testing.T) {
	type test struct {
		input string
		err   string
	}
	tests := []test{
		{"\x01", "invalid identifier: \x01"},
		{"4.f", "invalid decimal literal: 4.f"},
		{"4a.f", "invalid decimal literal: 4a"},
		{"0x.1", "invalid decimal literal: 0x."},
		{"0b.1", "invalid decimal literal: 0b."},
		{`"foo`, "unterminated string literal"},
		{"`foo", "unterminated string literal"},
		{"'foo", "unterminated string literal"},
		{"~", "unexpected character: '~'"},
	}
	for i, tt := range tests {
		t.Run(fmt.Sprintf("%d-%s", i, tt.input), func(t *testing.T) {
			l := New(tt.input)
			_, err := l.Next()
			assert.NotNil(t, err)
			assert.Equal(t, err.Error(), tt.err)
		})
	}
}

func TestStateSaveRestore(t *testing.T) {
	input := "let x = 1 + 2"
	l := New(input)

	// Read first two tokens
	tok1, err := l.Next()
	assert.Nil(t, err)
	assert.True(t, tok1.Type == token.LET)

	tok2, err := l.Next()
	assert.Nil(t, err)
	assert.True(t, tok2.Type == token.IDENT)
	assert.Equal(t, tok2.Literal, "x")

	// Save state
	state := l.SaveState()

	// Read more tokens
	tok3, err := l.Next()
	assert.Nil(t, err)
	assert.True(t, tok3.Type == token.ASSIGN)

	tok4, err := l.Next()
	assert.Nil(t, err)
	assert.True(t, tok4.Type == token.INT)
	assert.Equal(t, tok4.Literal, "1")

	// Restore state
	l.RestoreState(state)

	// Should read the same tokens again
	tok3Again, err := l.Next()
	assert.Nil(t, err)
	assert.True(t, tok3Again.Type == token.ASSIGN)
	assert.Equal(t, tok3Again.Literal, tok3.Literal)

	tok4Again, err := l.Next()
	assert.Nil(t, err)
	assert.True(t, tok4Again.Type == token.INT)
	assert.Equal(t, tok4Again.Literal, "1")

	// Continue reading
	tok5, err := l.Next()
	assert.Nil(t, err)
	assert.True(t, tok5.Type == token.PLUS)

	tok6, err := l.Next()
	assert.Nil(t, err)
	assert.True(t, tok6.Type == token.INT)
	assert.Equal(t, tok6.Literal, "2")
}

func TestStateSaveRestoreWithNewlines(t *testing.T) {
	input := "x\n\n\ny"
	l := New(input)

	// Read x
	tok1, err := l.Next()
	assert.Nil(t, err)
	assert.True(t, tok1.Type == token.IDENT)
	assert.Equal(t, tok1.Literal, "x")

	// Save state before newlines
	state := l.SaveState()

	// Read newlines and y
	tok2, err := l.Next()
	assert.Nil(t, err)
	assert.True(t, tok2.Type == token.NEWLINE)

	tok3, err := l.Next()
	assert.Nil(t, err)
	assert.True(t, tok3.Type == token.NEWLINE)

	tok4, err := l.Next()
	assert.Nil(t, err)
	assert.True(t, tok4.Type == token.NEWLINE)

	tok5, err := l.Next()
	assert.Nil(t, err)
	assert.True(t, tok5.Type == token.IDENT)
	assert.Equal(t, tok5.Literal, "y")

	// Restore and verify we can read the same sequence
	l.RestoreState(state)

	tok2Again, err := l.Next()
	assert.Nil(t, err)
	assert.True(t, tok2Again.Type == token.NEWLINE)
}

func TestSpreadOperator(t *testing.T) {
	tests := []struct {
		input    string
		expected []struct {
			typ     token.Type
			literal string
		}
	}{
		{
			input: "...",
			expected: []struct {
				typ     token.Type
				literal string
			}{
				{token.SPREAD, "..."},
				{token.EOF, ""},
			},
		},
		{
			input: "[...arr]",
			expected: []struct {
				typ     token.Type
				literal string
			}{
				{token.LBRACKET, "["},
				{token.SPREAD, "..."},
				{token.IDENT, "arr"},
				{token.RBRACKET, "]"},
				{token.EOF, ""},
			},
		},
		{
			// Two dots should be two periods, not spread
			input: "..",
			expected: []struct {
				typ     token.Type
				literal string
			}{
				{token.PERIOD, "."},
				{token.PERIOD, "."},
				{token.EOF, ""},
			},
		},
		{
			// Four dots = spread + period
			input: "....",
			expected: []struct {
				typ     token.Type
				literal string
			}{
				{token.SPREAD, "..."},
				{token.PERIOD, "."},
				{token.EOF, ""},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			l := New(tt.input)
			for i, exp := range tt.expected {
				tok, err := l.Next()
				assert.Nil(t, err)
				assert.Equal(t, tok.Type, exp.typ, "token %d type", i)
				assert.Equal(t, tok.Literal, exp.literal, "token %d literal", i)
			}
		})
	}
}

func TestOptionalChainingAndNullish(t *testing.T) {
	tests := []struct {
		input    string
		expected []struct {
			typ     token.Type
			literal string
		}
	}{
		{
			input: "a?.b",
			expected: []struct {
				typ     token.Type
				literal string
			}{
				{token.IDENT, "a"},
				{token.QUESTION_DOT, "?."},
				{token.IDENT, "b"},
				{token.EOF, ""},
			},
		},
		{
			input: "a ?? b",
			expected: []struct {
				typ     token.Type
				literal string
			}{
				{token.IDENT, "a"},
				{token.NULLISH, "??"},
				{token.IDENT, "b"},
				{token.EOF, ""},
			},
		},
		{
			input: "a?.b ?? c",
			expected: []struct {
				typ     token.Type
				literal string
			}{
				{token.IDENT, "a"},
				{token.QUESTION_DOT, "?."},
				{token.IDENT, "b"},
				{token.NULLISH, "??"},
				{token.IDENT, "c"},
				{token.EOF, ""},
			},
		},
		{
			// Single question mark
			input: "a ? b : c",
			expected: []struct {
				typ     token.Type
				literal string
			}{
				{token.IDENT, "a"},
				{token.QUESTION, "?"},
				{token.IDENT, "b"},
				{token.COLON, ":"},
				{token.IDENT, "c"},
				{token.EOF, ""},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			l := New(tt.input)
			for i, exp := range tt.expected {
				tok, err := l.Next()
				assert.Nil(t, err)
				assert.Equal(t, tok.Type, exp.typ, "token %d type", i)
				assert.Equal(t, tok.Literal, exp.literal, "token %d literal", i)
			}
		})
	}
}

func TestArrowFunction(t *testing.T) {
	tests := []struct {
		input    string
		expected []struct {
			typ     token.Type
			literal string
		}
	}{
		{
			input: "x => x",
			expected: []struct {
				typ     token.Type
				literal string
			}{
				{token.IDENT, "x"},
				{token.ARROW, "=>"},
				{token.IDENT, "x"},
				{token.EOF, ""},
			},
		},
		{
			input: "(a, b) => a + b",
			expected: []struct {
				typ     token.Type
				literal string
			}{
				{token.LPAREN, "("},
				{token.IDENT, "a"},
				{token.COMMA, ","},
				{token.IDENT, "b"},
				{token.RPAREN, ")"},
				{token.ARROW, "=>"},
				{token.IDENT, "a"},
				{token.PLUS, "+"},
				{token.IDENT, "b"},
				{token.EOF, ""},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			l := New(tt.input)
			for i, exp := range tt.expected {
				tok, err := l.Next()
				assert.Nil(t, err)
				assert.Equal(t, tok.Type, exp.typ, "token %d type", i)
				assert.Equal(t, tok.Literal, exp.literal, "token %d literal", i)
			}
		})
	}
}

func TestBitShiftOperators(t *testing.T) {
	tests := []struct {
		input    string
		expected []struct {
			typ     token.Type
			literal string
		}
	}{
		{
			input: "a << 2",
			expected: []struct {
				typ     token.Type
				literal string
			}{
				{token.IDENT, "a"},
				{token.LT_LT, "<<"},
				{token.INT, "2"},
				{token.EOF, ""},
			},
		},
		{
			input: "b >> 3",
			expected: []struct {
				typ     token.Type
				literal string
			}{
				{token.IDENT, "b"},
				{token.GT_GT, ">>"},
				{token.INT, "3"},
				{token.EOF, ""},
			},
		},
		{
			input: "1<<2>>3",
			expected: []struct {
				typ     token.Type
				literal string
			}{
				{token.INT, "1"},
				{token.LT_LT, "<<"},
				{token.INT, "2"},
				{token.GT_GT, ">>"},
				{token.INT, "3"},
				{token.EOF, ""},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			l := New(tt.input)
			for i, exp := range tt.expected {
				tok, err := l.Next()
				assert.Nil(t, err)
				assert.Equal(t, tok.Type, exp.typ, "token %d type", i)
				assert.Equal(t, tok.Literal, exp.literal, "token %d literal", i)
			}
		})
	}
}

func TestCRLFNewlines(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []struct {
			typ     token.Type
			literal string
		}
	}{
		{
			name:  "CRLF",
			input: "a\r\nb",
			expected: []struct {
				typ     token.Type
				literal string
			}{
				{token.IDENT, "a"},
				{token.NEWLINE, "\r\n"},
				{token.IDENT, "b"},
				{token.EOF, ""},
			},
		},
		{
			name:  "CR only",
			input: "a\rb",
			expected: []struct {
				typ     token.Type
				literal string
			}{
				{token.IDENT, "a"},
				{token.NEWLINE, "\r"},
				{token.IDENT, "b"},
				{token.EOF, ""},
			},
		},
		{
			name:  "mixed newlines",
			input: "a\r\nb\nc\rd",
			expected: []struct {
				typ     token.Type
				literal string
			}{
				{token.IDENT, "a"},
				{token.NEWLINE, "\r\n"},
				{token.IDENT, "b"},
				{token.NEWLINE, "\n"},
				{token.IDENT, "c"},
				{token.NEWLINE, "\r"},
				{token.IDENT, "d"},
				{token.EOF, ""},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := New(tt.input)
			for i, exp := range tt.expected {
				tok, err := l.Next()
				assert.Nil(t, err)
				assert.Equal(t, tok.Type, exp.typ, "token %d type", i)
				assert.Equal(t, tok.Literal, exp.literal, "token %d literal", i)
			}
		})
	}
}

func TestSinglePipe(t *testing.T) {
	tests := []struct {
		input    string
		expected []struct {
			typ     token.Type
			literal string
		}
	}{
		{
			input: "a | b",
			expected: []struct {
				typ     token.Type
				literal string
			}{
				{token.IDENT, "a"},
				{token.PIPE, "|"},
				{token.IDENT, "b"},
				{token.EOF, ""},
			},
		},
		{
			input: "a || b | c",
			expected: []struct {
				typ     token.Type
				literal string
			}{
				{token.IDENT, "a"},
				{token.OR, "||"},
				{token.IDENT, "b"},
				{token.PIPE, "|"},
				{token.IDENT, "c"},
				{token.EOF, ""},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			l := New(tt.input)
			for i, exp := range tt.expected {
				tok, err := l.Next()
				assert.Nil(t, err)
				assert.Equal(t, tok.Type, exp.typ, "token %d type", i)
				assert.Equal(t, tok.Literal, exp.literal, "token %d literal", i)
			}
		})
	}
}

func TestAsAfterPeriod(t *testing.T) {
	// The lexer has special handling for "as" after a period to ensure
	// it's treated as an identifier (for method names like obj.as())
	// rather than potentially being a keyword in the future.
	tests := []struct {
		name     string
		input    string
		expected []struct {
			typ     token.Type
			literal string
		}
	}{
		{
			name:  "as standalone is ident",
			input: "x as int",
			expected: []struct {
				typ     token.Type
				literal string
			}{
				{token.IDENT, "x"},
				{token.IDENT, "as"},
				{token.IDENT, "int"},
				{token.EOF, ""},
			},
		},
		{
			name:  "as after period is ident",
			input: "obj.as",
			expected: []struct {
				typ     token.Type
				literal string
			}{
				{token.IDENT, "obj"},
				{token.PERIOD, "."},
				{token.IDENT, "as"},
				{token.EOF, ""},
			},
		},
		{
			name:  "as method call",
			input: "obj.as()",
			expected: []struct {
				typ     token.Type
				literal string
			}{
				{token.IDENT, "obj"},
				{token.PERIOD, "."},
				{token.IDENT, "as"},
				{token.LPAREN, "("},
				{token.RPAREN, ")"},
				{token.EOF, ""},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := New(tt.input)
			for i, exp := range tt.expected {
				tok, err := l.Next()
				assert.Nil(t, err)
				assert.Equal(t, tok.Type, exp.typ, "token %d type", i)
				assert.Equal(t, tok.Literal, exp.literal, "token %d literal", i)
			}
		})
	}
}

func TestEmptyInput(t *testing.T) {
	l := New("")
	tok, err := l.Next()
	assert.Nil(t, err)
	assert.Equal(t, tok.Type, token.EOF)
	assert.Equal(t, tok.Literal, "")
}

func TestMultipleEOFReads(t *testing.T) {
	l := New("x")

	// Read the identifier
	tok, err := l.Next()
	assert.Nil(t, err)
	assert.Equal(t, tok.Type, token.IDENT)
	assert.Equal(t, tok.Literal, "x")

	// Read EOF multiple times
	for i := 0; i < 5; i++ {
		tok, err = l.Next()
		assert.Nil(t, err)
		assert.Equal(t, tok.Type, token.EOF, "EOF read %d", i)
	}
}

func TestUnterminatedMultiLineComment(t *testing.T) {
	// Unterminated multi-line comment should eventually hit EOF
	l := New("a /* unterminated comment")

	tok, err := l.Next()
	assert.Nil(t, err)
	assert.Equal(t, tok.Type, token.IDENT)
	assert.Equal(t, tok.Literal, "a")

	// The comment consumes everything until EOF
	tok, err = l.Next()
	assert.Nil(t, err)
	assert.Equal(t, tok.Type, token.EOF)
}

func TestShebangNotAtStart(t *testing.T) {
	// #! not at start should be treated differently
	tests := []struct {
		name     string
		input    string
		expected []struct {
			typ     token.Type
			literal string
		}
	}{
		{
			name:  "shebang at start is skipped",
			input: "#!/bin/risor\nx",
			expected: []struct {
				typ     token.Type
				literal string
			}{
				{token.NEWLINE, "\n"},
				{token.IDENT, "x"},
				{token.EOF, ""},
			},
		},
		{
			name:  "hash not followed by bang",
			input: "# comment",
			expected: []struct {
				typ     token.Type
				literal string
			}{
				// # is not a valid identifier start, should error
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := New(tt.input)
			for i, exp := range tt.expected {
				tok, err := l.Next()
				assert.Nil(t, err)
				assert.Equal(t, tok.Type, exp.typ, "token %d type", i)
				assert.Equal(t, tok.Literal, exp.literal, "token %d literal", i)
			}
		})
	}
}

func TestShebangMidFile(t *testing.T) {
	// #! after newline should error (not treated as shebang)
	input := "x\n#!/bin/risor"
	l := New(input)

	tok, err := l.Next()
	assert.Nil(t, err)
	assert.Equal(t, tok.Type, token.IDENT)

	tok, err = l.Next()
	assert.Nil(t, err)
	assert.Equal(t, tok.Type, token.NEWLINE)

	// # is not a valid identifier, should error
	_, err = l.Next()
	assert.NotNil(t, err)
}

func TestGetLineTextEdgeCases(t *testing.T) {
	t.Run("empty input", func(t *testing.T) {
		l := New("")
		tok, err := l.Next()
		assert.Nil(t, err)
		assert.Equal(t, tok.Type, token.EOF)
		line := l.GetLineText(tok)
		assert.Equal(t, line, "")
	})

	t.Run("single token no newline", func(t *testing.T) {
		l := New("hello")
		tok, err := l.Next()
		assert.Nil(t, err)
		line := l.GetLineText(tok)
		assert.Equal(t, line, "hello")
	})

	t.Run("token at start of line", func(t *testing.T) {
		l := New("first\nsecond")
		// Skip to second line
		l.Next() // first
		l.Next() // newline
		tok, err := l.Next()
		assert.Nil(t, err)
		assert.Equal(t, tok.Literal, "second")
		line := l.GetLineText(tok)
		assert.Equal(t, line, "second")
	})

	t.Run("token on last line without trailing newline", func(t *testing.T) {
		l := New("line1\nline2")
		l.Next() // line1
		l.Next() // newline
		tok, err := l.Next()
		assert.Nil(t, err)
		line := l.GetLineText(tok)
		assert.Equal(t, line, "line2")
	})

	t.Run("multiple tokens on same line", func(t *testing.T) {
		l := New("a + b")
		tok1, _ := l.Next() // a
		tok2, _ := l.Next() // +
		tok3, _ := l.Next() // b

		assert.Equal(t, l.GetLineText(tok1), "a + b")
		assert.Equal(t, l.GetLineText(tok2), "a + b")
		assert.Equal(t, l.GetLineText(tok3), "a + b")
	})

	t.Run("EOF token", func(t *testing.T) {
		l := New("x")
		l.Next() // x
		tok, err := l.Next()
		assert.Nil(t, err)
		assert.Equal(t, tok.Type, token.EOF)
		line := l.GetLineText(tok)
		assert.Equal(t, line, "x")
	})

	t.Run("EOF on empty line", func(t *testing.T) {
		// Note: GetLineText for EOF returns the previous line's content
		// as context, not an empty string for the empty line after newline
		l := New("x\n")
		l.Next() // x
		l.Next() // newline
		tok, err := l.Next()
		assert.Nil(t, err)
		assert.Equal(t, tok.Type, token.EOF)
		line := l.GetLineText(tok)
		assert.Equal(t, line, "x")
	})
}

func TestFilenameOption(t *testing.T) {
	t.Run("WithFile option", func(t *testing.T) {
		l := New("x", WithFile("test.risor"))
		assert.Equal(t, l.Filename(), "test.risor")

		tok, err := l.Next()
		assert.Nil(t, err)
		assert.Equal(t, tok.StartPosition.File, "test.risor")
		assert.Equal(t, tok.EndPosition.File, "test.risor")
	})

	t.Run("SetFilename method", func(t *testing.T) {
		l := New("x")
		assert.Equal(t, l.Filename(), "")

		l.SetFilename("updated.risor")
		assert.Equal(t, l.Filename(), "updated.risor")

		tok, err := l.Next()
		assert.Nil(t, err)
		assert.Equal(t, tok.StartPosition.File, "updated.risor")
	})

	t.Run("Position method includes file", func(t *testing.T) {
		l := New("x", WithFile("pos.risor"))
		pos := l.Position()
		assert.Equal(t, pos.File, "pos.risor")
	})
}

func TestTemplateStringWithNewlines(t *testing.T) {
	tests := []struct {
		name            string
		input           string
		expectedLiteral string
	}{
		{
			name:            "single line",
			input:           "`hello`",
			expectedLiteral: "hello",
		},
		{
			name:            "with newline",
			input:           "`hello\nworld`",
			expectedLiteral: "hello\nworld",
		},
		{
			name:            "multiple newlines",
			input:           "`line1\nline2\nline3`",
			expectedLiteral: "line1\nline2\nline3",
		},
		{
			name:            "with CRLF",
			input:           "`hello\r\nworld`",
			expectedLiteral: "hello\r\nworld",
		},
		{
			name:            "with tabs and spaces",
			input:           "`  \t  hello  \t  `",
			expectedLiteral: "  \t  hello  \t  ",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := New(tt.input)
			tok, err := l.Next()
			assert.Nil(t, err)
			assert.Equal(t, tok.Type, token.TEMPLATE)
			assert.Equal(t, tok.Literal, tt.expectedLiteral)
		})
	}
}

func TestFloatEdgeCases(t *testing.T) {
	tests := []struct {
		input           string
		expectedType    token.Type
		expectedLiteral string
	}{
		{"0.0", token.FLOAT, "0.0"},
		{"0.1", token.FLOAT, "0.1"},
		{"0.123456789", token.FLOAT, "0.123456789"},
		{"123.0", token.FLOAT, "123.0"},
		{"0", token.INT, "0"},
		{"00", token.INT, "00"}, // octal zero
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			l := New(tt.input)
			tok, err := l.Next()
			assert.Nil(t, err)
			assert.Equal(t, tok.Type, tt.expectedType)
			assert.Equal(t, tok.Literal, tt.expectedLiteral)
		})
	}
}

func TestStringWithEmbeddedNewline(t *testing.T) {
	// String literals (not templates) cannot span lines
	tests := []struct {
		name  string
		input string
	}{
		{"double quote with newline", "\"hello\nworld\""},
		{"single quote with newline", "'hello\nworld'"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := New(tt.input)
			_, err := l.Next()
			assert.NotNil(t, err)
			assert.Equal(t, err.Error(), "unterminated string literal")
		})
	}
}

func TestWhitespaceOnly(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"spaces only", "   "},
		{"tabs only", "\t\t\t"},
		{"mixed tabs and spaces", "  \t  \t  "},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := New(tt.input)
			tok, err := l.Next()
			assert.Nil(t, err)
			assert.Equal(t, tok.Type, token.EOF)
		})
	}
}

func TestSlashEquals(t *testing.T) {
	input := "a /= 2"
	expected := []struct {
		typ     token.Type
		literal string
	}{
		{token.IDENT, "a"},
		{token.SLASH_EQUALS, "/="},
		{token.INT, "2"},
		{token.EOF, ""},
	}
	l := New(input)
	for i, exp := range expected {
		tok, err := l.Next()
		assert.Nil(t, err)
		assert.Equal(t, tok.Type, exp.typ, "token %d type", i)
		assert.Equal(t, tok.Literal, exp.literal, "token %d literal", i)
	}
}
