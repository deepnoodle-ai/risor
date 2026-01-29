package parser

import (
	"context"
	"testing"
	"time"
	"unicode/utf8"
)

// FuzzParseComprehensive tests that the parser doesn't panic on arbitrary input.
// The parser should either return a valid AST or an error, never crash.
// This extends the basic FuzzParse in parser_test.go with more seed corpus.
func FuzzParseComprehensive(f *testing.F) {
	// Seed corpus with valid Risor code
	seeds := []string{
		// Basic expressions
		"1 + 2",
		"x",
		"true",
		"false",
		"nil",
		"\"hello\"",
		"`template`",
		"[]",
		"{}",
		"[1, 2, 3]",
		"{a: 1, b: 2}",

		// Operators
		"a + b - c * d / e % f",
		"2 ** 3",
		"-x",
		"!flag",
		"a && b || c",
		"a ?? b",
		"a == b != c",
		"a < b <= c > d >= e",
		"a & b",
		"a << 2",
		"b >> 1",

		// Variables and assignments
		"let x = 1",
		"const y = 2",
		"x = 10",
		"x += 1",
		"x -= 1",
		"x *= 2",
		"x /= 2",
		"let a, b = [1, 2]",

		// Destructuring
		"let { a, b } = obj",
		"let { a: x, b: y } = obj",
		"let { a = 10 } = obj",
		"let [a, b] = arr",
		"let [a = 1, b = 2] = arr",

		// Functions
		"function f() { }",
		"function add(a, b) { return a + b }",
		"function f(a, b = 10) { }",
		"function f(...args) { }",
		"x => x + 1",
		"() => 42",
		"(a, b) => a + b",
		"(x = 1) => x",

		// Control flow
		"if (x) { y }",
		"if (x) { y } else { z }",
		"if (a) { x } else if (b) { y } else { z }",
		"a ? b : c",

		// Switch
		`switch (x) {
case 1:
	a
case 2, 3:
	b
default:
	c
}`,

		// Try/catch
		"try { x } catch { y }",
		"try { x } finally { y }",
		"try { x } catch e { y } finally { z }",

		// Membership
		"x in list",
		"y not in set",

		// Index and slice
		"arr[0]",
		"arr[1:2]",
		"arr[:2]",
		"arr[1:]",
		"arr[:]",

		// Attribute access
		"obj.field",
		"obj.method()",
		"obj?.field",
		"obj?.method()",
		"obj.field = 1",

		// Pipes
		"a |> b |> c",
		"data |> filter(f) |> map(g)",

		// Spread
		"[...arr]",
		"{...obj}",
		"f(...args)",

		// Complex nested
		"f(g(h(x)))",
		"a.b.c.d.e",
		"arr[0][1][2]",
		"((((x))))",

		// Multiline
		"a +\nb",
		"[\n1,\n2,\n3\n]",
		"{\na: 1,\nb: 2\n}",
		"f(\na,\nb\n)",

		// Template strings
		"`hello ${name}`",
		"`${a} and ${b}`",
		"`result: ${a + b}`",

		// Return and throw
		"return",
		"return 42",
		"throw error(\"oops\")",

		// Postfix
		"x++",
		"x--",
		"arr[0]++",

		// Edge cases - invalid but should not crash
		"",
		" ",
		"\n",
		"\t",
		"@",
		"#",
		"$",
		"(",
		")",
		"[",
		"]",
		"{",
		"}",
		"let",
		"if",
		"function",
		"1 +",
		"+ 1",
		"((",
		"))",
		"[[",
		"]]",
		"{{",
		"}}",
		"let x =",
		"let x = =",
		"if ()",
		"if (x)",
		"function(",
		"function f(",
		"a ? b",
		"a ? b :",
		"a ? : c",
		"? b : c",
		"switch",
		"switch ()",
		"switch (x)",
		"switch (x) {",
		"case",
		"case 1",
		"case 1:",
		"try",
		"try {",
		"try { }",
		"catch",
		"finally",
		"throw",
		"return return",
		"let let",
		"const const",
		"...",
		"...x...",
		"x..",
		"..x",
		".x",
		"x.",
		"x..",
		"x..y",
		"x?",
		"x?.",
		"x?.?",
		"=>",
		"() =>",
		"x =>",
		"(x) =>",
		"(x, y) =>",
		"(,) => x",
		"(x,,y) => x",
		"let {} = x",
		"let [] = x",
		"let { } = x",
		"let [ ] = x",
		"a ?? ?? b",
		"a || || b",
		"a && && b",
		"a ** ** b",
		"1 2 3",
		"x y z",
		"let x = let y = 1",
		"if if",
		"else",
		"else { }",
		"default",
		"default:",
		"in",
		"not",
		"not x",
		"x not",
		"x in",
		"x not in",
		"x not not in y",

		// Unicode
		"\"日本語\"",
		"`日本語`",
		"\"emoji: 🎉\"",
		"`emoji: 🎉`",
		"\"\\u0000\"",
		"\"\\xff\"",

		// Numbers
		"0",
		"00",
		"0x0",
		"0xDEADBEEF",
		"0777",
		"1.5",
		"0.0",
		"999999999999999999999999999999",

		// Long inputs
		"((((((((((((((((((((x))))))))))))))))))))",
		"a.b.c.d.e.f.g.h.i.j.k.l.m.n.o.p.q.r.s.t",
		"[[[[[[[[[[[[[[[[[[[x]]]]]]]]]]]]]]]]]]]",
	}

	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, input string) {
		// Skip very long inputs to avoid timeout
		if len(input) > 10000 {
			return
		}

		// Create a context with timeout to prevent infinite loops
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		// The parser should NEVER panic, regardless of input
		// It should either return a valid result or an error
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("Parser panicked on input %q: %v", truncate(input, 100), r)
				}
			}()

			program, err := Parse(ctx, input, nil)

			// If no error, the program should be non-nil
			if err == nil && program == nil {
				t.Errorf("Parse returned nil program without error for input %q", truncate(input, 100))
			}

			// If we got a program, verify it's valid
			if program != nil {
				// String() should not panic
				_ = program.String()

				// Verify statements are accessible
				for _, stmt := range program.Stmts {
					if stmt != nil {
						_ = stmt.String()
					}
				}
			}
		}()
	})
}

// FuzzParseStringConsistency tests that parsing produces consistent String() output.
// Note: AST String() is for debugging, not code generation - it may not produce
// valid Risor syntax. This test verifies that String() is at least consistent
// (calling it twice produces the same result) and doesn't panic.
func FuzzParseStringConsistency(f *testing.F) {
	// Seed with valid Risor code
	seeds := []string{
		"1 + 2",
		"x",
		"let x = 1",
		"function f(a, b) { return a + b }",
		"if (x) { y } else { z }",
		"[1, 2, 3]",
		"{a: 1, b: 2}",
		"x => x + 1",
		"a ? b : c",
		"obj.field",
		"arr[0]",
		"a |> b |> c",
		"try { x } catch e { y }",
	}

	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, input string) {
		if len(input) > 5000 {
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		defer func() {
			if r := recover(); r != nil {
				t.Errorf("Panic on input %q: %v", truncate(input, 100), r)
			}
		}()

		program, err := Parse(ctx, input, nil)
		if err != nil || program == nil {
			return
		}

		// String() should be consistent (calling twice gives same result)
		str1 := program.String()
		str2 := program.String()
		if str1 != str2 {
			t.Errorf("String() not consistent: first=%q second=%q",
				truncate(str1, 200), truncate(str2, 200))
		}

		// String() output should be valid UTF-8
		if !utf8.ValidString(str1) {
			t.Errorf("String() produced invalid UTF-8 for input %q", truncate(input, 100))
		}
	})
}

// FuzzParseUTF8 tests the parser handles UTF-8 correctly
func FuzzParseUTF8(f *testing.F) {
	seeds := []string{
		"\"hello\"",
		"\"日本語\"",
		"`template`",
		"`日本語`",
		"x",
		"日本語",
		"let 変数 = 1",
	}

	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, input string) {
		if len(input) > 5000 {
			return
		}

		// Check if input is valid UTF-8
		validUTF8 := utf8.ValidString(input)

		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		defer func() {
			if r := recover(); r != nil {
				t.Errorf("Parser panicked on UTF-8 input (valid=%v) %q: %v",
					validUTF8, truncate(input, 100), r)
			}
		}()

		program, _ := Parse(ctx, input, nil)

		if program != nil {
			// Verify String() produces valid UTF-8
			str := program.String()
			if !utf8.ValidString(str) {
				t.Errorf("Program.String() produced invalid UTF-8 for input %q", truncate(input, 100))
			}
		}
	})
}

// FuzzParseDeepNesting tests the parser handles deeply nested structures
func FuzzParseDeepNesting(f *testing.F) {
	f.Add(10)
	f.Add(50)
	f.Add(100)
	f.Add(200)
	f.Add(500)

	f.Fuzz(func(t *testing.T, depth int) {
		if depth < 1 || depth > 1000 {
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
		defer cancel()

		// Test deeply nested parentheses
		input := ""
		for i := 0; i < depth; i++ {
			input += "("
		}
		input += "x"
		for i := 0; i < depth; i++ {
			input += ")"
		}

		defer func() {
			if r := recover(); r != nil {
				t.Errorf("Parser panicked on depth %d parentheses: %v", depth, r)
			}
		}()

		_, _ = Parse(ctx, input, nil)

		// Test deeply nested lists
		input = ""
		for i := 0; i < depth; i++ {
			input += "["
		}
		input += "x"
		for i := 0; i < depth; i++ {
			input += "]"
		}

		_, _ = Parse(ctx, input, nil)

		// Test deeply nested maps
		input = ""
		for i := 0; i < depth; i++ {
			input += "{a:"
		}
		input += "x"
		for i := 0; i < depth; i++ {
			input += "}"
		}

		_, _ = Parse(ctx, input, nil)

		// Test deeply chained attribute access
		input = "x"
		for i := 0; i < depth; i++ {
			input += ".y"
		}

		_, _ = Parse(ctx, input, nil)
	})
}

// FuzzParseOperatorCombinations tests various operator combinations
func FuzzParseOperatorCombinations(f *testing.F) {
	operators := []string{"+", "-", "*", "/", "%", "**", "==", "!=", "<", ">", "<=", ">=", "&&", "||", "??", "&", "<<", ">>", "|"}

	// Add seed combinations
	for _, op1 := range operators[:5] {
		for _, op2 := range operators[:5] {
			f.Add(op1, op2)
		}
	}

	f.Fuzz(func(t *testing.T, op1, op2 string) {
		// Test: a op1 b op2 c
		input := "a " + op1 + " b " + op2 + " c"

		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		defer func() {
			if r := recover(); r != nil {
				t.Errorf("Parser panicked on operator combo %q %q: %v", op1, op2, r)
			}
		}()

		_, _ = Parse(ctx, input, nil)
	})
}

// FuzzParseStatementBoundaries tests edge cases around statement boundaries
// This targets potential issues with newline handling, semicolon insertion, etc.
func FuzzParseStatementBoundaries(f *testing.F) {
	// Seed with statement boundary edge cases
	seeds := []string{
		// Multiple statements
		"a\nb",
		"a;b",
		"a\n\nb",
		"a;;b",

		// Newlines in expressions
		"a +\nb",
		"a\n+ b",
		"a\n+\nb",
		"let x =\n1",
		"let x\n= 1",
		"let\nx = 1",

		// Mixed statements
		"let x = 1\nlet y = 2",
		"let x = 1; let y = 2",
		"function f() {}\nlet x = 1",

		// Empty lines
		"\n\n\n",
		"a\n\n\nb",
		"  \n  \n  ",

		// Trailing/leading newlines
		"\na",
		"a\n",
		"\na\n",
		"\n\na\n\n",

		// Comments (if supported)
		"a // comment\nb",
		"a /* comment */ b",
		"a /* multi\nline */ b",

		// Continuation after operators
		"a &&\nb",
		"a ||\nb",
		"a ?\nb : c",
		"a ?\nb :\nc",

		// Block boundaries
		"{ a\nb }",
		"{ a; b }",
		"{\na\nb\n}",
		"if (x) {\na\n} else {\nb\n}",

		// Arrow functions
		"x =>\nx + 1",
		"(x) =>\nx",
		"() => {\na\nb\n}",

		// Call arguments
		"f(a\n,b)",
		"f(a,\nb)",
		"f(\na\n,\nb\n)",

		// Array/map elements
		"[a\n,b]",
		"[a,\nb]",
		"[\na\n,\nb\n]",
		"{a: 1\n,b: 2}",
		"{a: 1,\nb: 2}",
	}

	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, input string) {
		if len(input) > 5000 {
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		defer func() {
			if r := recover(); r != nil {
				t.Errorf("Panic on statement boundary input %q: %v", truncate(input, 100), r)
			}
		}()

		program, _ := Parse(ctx, input, nil)

		if program != nil {
			_ = program.String()
		}
	})
}

// FuzzParseErrorRecovery tests that the parser handles errors gracefully
// and doesn't get stuck in infinite loops or produce corrupt ASTs
func FuzzParseErrorRecovery(f *testing.F) {
	// Seed with inputs that should produce errors but not crashes
	seeds := []string{
		// Missing parts
		"let",
		"let =",
		"let x =",
		"let x = =",
		"const",
		"const =",
		"const x =",
		"function",
		"function f",
		"function f(",
		"function f()",
		"function f() {",

		// Mismatched brackets
		"[",
		"]",
		"{",
		"}",
		"(",
		")",
		"([)",
		"({)",
		"[(]",
		"{(}",
		"[{]",
		"({)",
		"(((",
		")))",
		"[[[",
		"]]]",
		"{{{",
		"}}}",
		"(()",
		"())",
		"[[]",
		"[]]",
		"{{]",
		"{}}",

		// Incomplete operators
		"a +",
		"+ a",
		"a -",
		"- a",
		"a *",
		"* a",
		"a /",
		"a %",
		"a **",
		"** a",
		"a &&",
		"&& a",
		"a ||",
		"|| a",
		"a ??",
		"?? a",
		"a ?",
		"? a",
		"a ? b",
		"a ? b :",
		"a ? : c",

		// Double operators
		"a ++ ++",
		"a -- --",
		"a + +",
		"a - -",
		"a * *",
		"a && &&",
		"a || ||",

		// Invalid keywords in context
		"let let",
		"const const",
		"return return",
		"throw throw",
		"if if",
		"else",
		"else {}",
		"case",
		"case:",
		"case 1:",
		"default",
		"default:",
		"catch",
		"catch {}",
		"finally",
		"finally {}",

		// Invalid destructuring
		"let {} =",
		"let [] =",
		"let {a:} = x",
		"let {:a} = x",
		"let {=1} = x",
		"let [=1] = x",

		// Invalid arrow functions
		"=>",
		"=> x",
		"() =>",
		"x =>",
		"(x) =>",
		"(,) => x",
		"(x,,y) => x",
		"(...) => x",
		"(x, ...y, z) => x",

		// Invalid spread
		"...",
		"...x...",
		"[...]",
		"{...}",
		"f(...)",
	}

	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, input string) {
		if len(input) > 5000 {
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		defer func() {
			if r := recover(); r != nil {
				t.Errorf("Panic during error recovery on input %q: %v", truncate(input, 100), r)
			}
		}()

		program, _ := Parse(ctx, input, nil)

		// Even on error, if a program is returned it should be usable
		if program != nil {
			_ = program.String()
			for _, stmt := range program.Stmts {
				if stmt != nil {
					_ = stmt.String()
				}
			}
		}
	})
}

// FuzzParseRandomBytes tests the parser with arbitrary byte sequences
// This can find issues with invalid UTF-8, control characters, etc.
func FuzzParseRandomBytes(f *testing.F) {
	// Seed with some byte patterns that might cause issues
	seeds := [][]byte{
		[]byte("normal"),
		{0x00},                             // NULL byte
		{0x7f},                             // DEL
		{0xff},                             // Invalid UTF-8
		{0x80},                             // Invalid UTF-8 continuation
		{0xc0, 0x80},                       // Overlong encoding
		{0xfe, 0xff},                       // UTF-16 BOM
		{0xef, 0xbb, 0xbf},                 // UTF-8 BOM
		{0xed, 0xa0, 0x80},                 // UTF-16 surrogate
		[]byte("let x = \x00"),             // NULL in code
		[]byte("let x = \xff"),             // Invalid byte in code
		[]byte("\x1b[31m"),                 // ANSI escape sequence
		[]byte("a\rb"),                     // Carriage return
		[]byte("a\r\nb"),                   // Windows newline
		[]byte("a\x0bb"),                   // Vertical tab
		[]byte("a\x0cb"),                   // Form feed
		{0xf0, 0x9f, 0x98, 0x80},           // Valid emoji (😀)
		[]byte("let \xf0\x9f\x98\x80 = 1"), // Emoji in identifier
	}

	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, input []byte) {
		if len(input) > 5000 {
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		defer func() {
			if r := recover(); r != nil {
				t.Errorf("Panic on random bytes %v: %v", input[:min(len(input), 20)], r)
			}
		}()

		program, _ := Parse(ctx, string(input), nil)

		if program != nil {
			_ = program.String()
		}
	})
}

// truncate truncates a string for display
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
