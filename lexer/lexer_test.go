package lexer

import (
	"fmt"
	"testing"

	"github.com/risor-io/risor/token"
	"github.com/stretchr/testify/require"
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
		require.Nil(t, err)
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
		require.Nil(t, err)
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
for
2 >= 1
1 <= 3
break
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
		{token.FOR, "for"},
		{token.NEWLINE, "\n"},
		{token.INT, "2"},
		{token.GT_EQUALS, ">="},
		{token.INT, "1"},
		{token.NEWLINE, "\n"},
		{token.INT, "1"},
		{token.LT_EQUALS, "<="},
		{token.INT, "3"},
		{token.NEWLINE, "\n"},
		{token.BREAK, "break"},
		{token.NEWLINE, "\n"},
		{token.EOF, ""},
	}
	l := New(input)
	for i, tt := range tests {
		tok, err := l.Next()
		require.Nil(t, err)
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
	require.Nil(t, err)
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
		require.Nil(t, err)
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
		require.Nil(t, err)
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
		require.Nil(t, err)
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
		require.Nil(t, err)
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
		require.NotNil(t, err)
		require.Equal(t, tt.expected, err.Error())
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
		require.Nil(t, err)
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
		require.Nil(t, err)
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
			require.Nil(t, err)
			require.Equal(t, tt.expectedType, tok.Type)
			require.Equal(t, tt.expectedLiteral, tok.Literal)
			// require.Equal(t, tt.expectedLine, tok.Line) // FIXME
			require.Equal(t, tt.expectedStartPos, tok.StartPosition.Column)
			require.Equal(t, tt.expectedEndPos, tok.EndPosition.Column)
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
		{"for", token.FOR, "for", 0, 0, 2},
		{"let", token.LET, "let", 0, 0, 2},
		{"false", token.FALSE, "false", 0, 0, 4},
		{"import", token.IMPORT, "import", 0, 0, 5},
		{">=", token.GT_EQUALS, ">=", 0, 0, 1},
		{" \n", token.NEWLINE, "\n", 0, 1, 1},
		{" {", token.LBRACE, "{", 0, 1, 1},
		{" ++", token.PLUS_PLUS, "++", 0, 1, 2},
	}
	for i, tt := range tests {
		t.Run(fmt.Sprintf("%d-%s", i, tt.input), func(t *testing.T) {
			l := New(tt.input)
			tok, err := l.Next()
			require.Nil(t, err)
			require.Equal(t, tt.expectedType, tok.Type)
			require.Equal(t, tt.expectedLiteral, tok.Literal)
			// require.Equal(t, tt.expectedLine, tok.Line) // FIXME
			require.Equal(t, tt.expectedStartPos, tok.StartPosition.Column)
			require.Equal(t, tt.expectedEndPos, tok.EndPosition.Column)
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
			require.Nil(t, err)
			require.Equal(t, tt.expectedType, tok.Type)
			require.Equal(t, tt.expectedLiteral, tok.Literal)
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
			require.Nil(t, err)
			require.Equal(t, tt.expectedType, tok.Type)
			require.Equal(t, tt.expectedLiteral, tok.Literal)
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
			require.NotNil(t, err)
			require.Equal(t, tt.err, err.Error())
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
			require.Nil(t, err)
			require.Equal(t, token.Type(token.STRING), tok.Type)
			require.Equal(t, tt.expectedLiteral, tok.Literal)
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
			require.Errorf(t, err, "Unexpected result: token=%s, literal=%q", tok.Type, tok.Literal)
		})
	}
}

func TestTokenLineText(t *testing.T) {
	l := New(` var x = 32; foo = bar
bar = baz
`)
	tok, err := l.Next()
	require.Nil(t, err)
	fmt.Println(tok)

	line := l.GetLineText(tok)
	require.Equal(t, " var x = 32; foo = bar", line)
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
		{"0b.1", "invalid decimal literal: 0b"},
		{`"foo`, "unterminated string literal"},
		{"`foo", "unterminated string literal"},
		{"'foo", "unterminated string literal"},
		{"~", "unexpected character: '~'"},
	}
	for i, tt := range tests {
		t.Run(fmt.Sprintf("%d-%s", i, tt.input), func(t *testing.T) {
			l := New(tt.input)
			_, err := l.Next()
			require.NotNil(t, err)
			require.Equal(t, tt.err, err.Error())
		})
	}
}
