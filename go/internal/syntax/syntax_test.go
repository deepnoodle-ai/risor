package syntax

import (
	"fmt"
	"strings"
	"testing"

	"github.com/deepnoodle-ai/wonton/assert"
)

// parse parses src and returns the tree, or fails the test.
func parse(t *testing.T, src string) *Program {
	t.Helper()
	prog, err := Parse(NewFile(src))
	if err != nil {
		t.Fatalf("%q: unexpected %s at %v", src, err.Code, NewFile(src).Span(err.Start, err.End))
	}
	return prog
}

// failure parses src and returns the code and span of the first diagnostic.
func failure(src string) (string, [4]int) {
	f := NewFile(src)
	_, err := Parse(f)
	if err == nil {
		return "", [4]int{}
	}
	return err.Code, f.Span(err.Start, err.End).Array()
}

type errCase struct {
	src  string
	code string
	span [4]int
}

func checkErrors(t *testing.T, cases []errCase) {
	t.Helper()
	for _, c := range cases {
		code, span := failure(c.src)
		assert.Equal(t, code, c.code, "code of %q", c.src)
		assert.Equal(t, span, c.span, "span of %q (%s)", c.src, c.code)
	}
}

func checkDumps(t *testing.T, cases [][2]string) {
	t.Helper()
	for _, c := range cases {
		prog := parse(t, c[0])
		assert.Equal(t, Dump(prog.Result), c[1], "tree of %q", c[0])
	}
}

// 01-language.md section 15: the examples. Those whose value is a parse code
// must fail with that code and span; every other one must parse.
func TestSection15Examples(t *testing.T) {
	ok := []string{
		`7 / 2`, `-7 % 3`, `2 ** 10`, `2 ** -1`, `9007199254740991 + 1`,
		`1e308 * 10`, `1 / 0`, `0.1 + 0.2`, "`${1e21} ${0.000001} ${1e-7}`",
		`u.name || "anonymous"`, `u.name ?? "anonymous"`, `!u.tags`, `[] && 1`,
		`1 == true`, `[1, [2]] == [1, [2]]`, `"a" < "b" && "ab" > "a"`, `"n=" + 5`,
		`"" in "abc"`, `"x" in {x: null}`, `1 in null`, `u.missing`, `u?.missing`,
		`null?.a.b.c`, `(null?.a).b`, `5?.a`, `"héllo"[1]`, `[1, 2][2]`,
		`[1, 2]?.[2]`, `{ const [a, ...r] = [1, 2, 3]; {a, r} }`,
		`{ const {a = 5, b} = {a: null, b: 2}; a + b }`, `{ const {a} = {}; a }`,
		`{ const f = (x, n = 2) => x * n; f(3) + f(3, null) }`,
		`if (0) { "a" } else if ("") { "b" } else { "c" }`,
		`(if (true) { 1 } else { 2 }) + 3`, `match 2 { 1 | 2 => "low", _ => "high" }`,
		`match "c" { "a" => 1, "b" => 2 }`, `{...{a: 1, b: 2}, b: 3, [("c")]: 4}`,
		`[...null]`, "`${[1, \"a\"]}-${null}-${{b: 1, a: true}}`",
		`try { 1 / 0 } catch (e) { e.code }`, `try { fail("no", {n: 1}) } catch (e) { e }`,
		`try { [][0] } catch { "none" }`, `{if: 1}.if`,
	}
	for _, src := range ok {
		parse(t, src)
	}
	checkErrors(t, []errCase{
		{`-2 ** 2`, "unexpected-token", [4]int{1, 1, 1, 2}},
		{`9007199254740992`, "literal-range", [4]int{1, 1, 1, 17}},
		{`010`, "bad-number", [4]int{1, 1, 1, 4}},
		{`if (true) { 1 }`, "unexpected-token", [4]int{1, 16, 1, 16}},
		{`if (true) { a: 1 } else { 2 }`, "unexpected-token", [4]int{1, 11, 1, 12}},
		{`try 1 catch { 2 }`, "unexpected-token", [4]int{1, 5, 1, 6}},
		{`try(1, 2)`, "unexpected-token", [4]int{1, 1, 1, 4}},
		{`const match = 1; match`, "reserved-word", [4]int{1, 7, 1, 12}},
		{`a ?? b > c`, "unexpected-token", [4]int{1, 8, 1, 9}},
		{`x as int`, "unexpected-token", [4]int{1, 3, 1, 5}},
		{`R"a"`, "unexpected-token", [4]int{1, 1, 1, 2}}, // with F3, r"a" is a raw string
		{`"a\q"`, "bad-escape", [4]int{1, 3, 1, 5}},
		{strings.Repeat("(", 101) + "1" + strings.Repeat(")", 101), "nesting-exceeded", [4]int{1, 101, 1, 102}},
	})
	// 100 levels are allowed.
	parse(t, strings.Repeat("(", 100)+"1"+strings.Repeat(")", 100))
}

// 01-language.md 14.2: one row each, and the reserved-word examples.
func TestParseCodes(t *testing.T) {
	checkErrors(t, []errCase{
		{`1 2`, "unexpected-token", [4]int{1, 3, 1, 4}},
		{`1 ~ 2`, "unexpected-character", [4]int{1, 3, 1, 4}},
		{`"abc`, "unterminated-string", [4]int{1, 1, 1, 5}},
		{"`abc", "unterminated-string", [4]int{1, 1, 1, 5}},
		{`1 /* a`, "unterminated-comment", [4]int{1, 3, 1, 5}},
		{`"\x41"`, "bad-escape", [4]int{1, 2, 1, 4}},
		{"`a\\\nb`", "line-continuation", [4]int{1, 3, 1, 4}},
		{`1__0`, "bad-number", [4]int{1, 1, 1, 5}},
		{`1e400`, "literal-range", [4]int{1, 1, 1, 6}},
		{strings.Repeat("[", 101), "nesting-exceeded", [4]int{1, 101, 1, 102}},
		{`const match = 1;`, "reserved-word", [4]int{1, 7, 1, 12}},
		// The examples after the rule.
		{`is => 1`, "reserved-word", [4]int{1, 1, 1, 3}},
		{`f(is => 1)`, "reserved-word", [4]int{1, 3, 1, 5}},
		{`let x = 1;`, "reserved-word", [4]int{1, 1, 1, 4}},
		{`return x`, "reserved-word", [4]int{1, 1, 1, 7}},
		{`nil`, "reserved-word", [4]int{1, 1, 1, 4}},
		{`[for (x of xs) x]`, "reserved-word", [4]int{1, 2, 1, 5}},
		{`{for}`, "reserved-word", [4]int{1, 2, 1, 5}},
		{`1 + else`, "reserved-word", [4]int{1, 5, 1, 9}},
		// Where a Name is needed.
		{`f((as) => 1)`, "reserved-word", [4]int{1, 4, 1, 6}},
		{`const {a: in} = x; 1`, "reserved-word", [4]int{1, 11, 1, 13}},
		{`const [do] = x; 1`, "reserved-word", [4]int{1, 8, 1, 10}},
		{`const [...var] = x; 1`, "reserved-word", [4]int{1, 11, 1, 14}},
		{`try { 1 } catch (catch) { 2 }`, "reserved-word", [4]int{1, 18, 1, 23}},
		{`[const]`, "reserved-word", [4]int{1, 2, 1, 7}},
		{`1 + not`, "reserved-word", [4]int{1, 5, 1, 8}},
	})
	checkDumps(t, [][2]string{
		{`x.nil`, `(. x nil)`},
		{`err.catch`, `(. err catch)`},
		{`{for: 1, import: 2}`, `(map (entry "for" 1) (entry "import" 2))`},
		{`x in"abc"`, `(in x "abc")`},
	})
}

// 01-language.md 14.3: every row.
func TestFollowOnText(t *testing.T) {
	checkErrors(t, []errCase{
		{`"""abc"""`, "text-block-open", [4]int{1, 1, 1, 4}}, // F3 6.1
		{"'''abc\nd'''", "unexpected-token", [4]int{1, 1, 1, 4}},
		{`1 + """`, "unterminated-string", [4]int{1, 5, 1, 8}}, // F3 4.1 item 4
		{`r"a\"b"`, "unexpected-token", [4]int{1, 6, 1, 7}},    // F3 3 item 3: r"a\" then b"
		{`f"x"`, "unexpected-token", [4]int{1, 1, 1, 2}},
		{`R"x"`, "unexpected-token", [4]int{1, 1, 1, 2}},
		{"r`a\nb`", "unexpected-token", [4]int{1, 1, 1, 2}},
		{"String.raw`x`", "unexpected-token", [4]int{1, 8, 1, 11}},
		{`r "abc"`, "unexpected-token", [4]int{1, 1, 1, 2}},
		{`[R 'abc']`, "unexpected-token", [4]int{1, 2, 1, 3}},
		{`x as T`, "unexpected-token", [4]int{1, 3, 1, 5}},
		{`x is T`, "unexpected-token", [4]int{1, 3, 1, 5}},
		{`x as const`, "unexpected-token", [4]int{1, 3, 1, 5}},
		{`x is not T`, "unexpected-token", [4]int{1, 3, 1, 5}},
		{`a + b as T`, "unexpected-token", [4]int{1, 7, 1, 9}},
		{`!x as T`, "unexpected-token", [4]int{1, 1, 1, 2}},
		{`!x is T`, "unexpected-token", [4]int{1, 1, 1, 2}},
		{`a == !x is T`, "unexpected-token", [4]int{1, 6, 1, 7}},
		{`(!x) is T`, "unexpected-token", [4]int{1, 6, 1, 8}},
		{`xs[*]`, "unexpected-token", [4]int{1, 4, 1, 5}},
		{`xs?.[*]`, "unexpected-token", [4]int{1, 6, 1, 7}},
		{`match { 1 => 2 }`, "unexpected-token", [4]int{1, 7, 1, 8}},
		{`a === b`, "unexpected-token", [4]int{1, 3, 1, 6}},
		{`a !== b`, "unexpected-token", [4]int{1, 3, 1, 6}},
		{`x = 1`, "unexpected-token", [4]int{1, 3, 1, 4}},
		{`x += 1`, "unexpected-token", [4]int{1, 3, 1, 5}},
		{`x ++`, "unexpected-token", [4]int{1, 3, 1, 5}},
		{`x -= 1`, "unexpected-token", [4]int{1, 3, 1, 5}},
		{`x *= 1`, "unexpected-token", [4]int{1, 3, 1, 5}},
		{`x /= 1`, "unexpected-token", [4]int{1, 3, 1, 5}},
		{`x %= 1`, "unexpected-token", [4]int{1, 3, 1, 5}},
		{`x **= 1`, "unexpected-token", [4]int{1, 3, 1, 6}},
		{`x ??= 1`, "unexpected-token", [4]int{1, 3, 1, 6}},
		{`x ||= 1`, "unexpected-token", [4]int{1, 3, 1, 6}},
		{`x &&= 1`, "unexpected-token", [4]int{1, 3, 1, 6}},
		{`x |> f`, "unexpected-token", [4]int{1, 3, 1, 5}},
		{`a & b`, "unexpected-character", [4]int{1, 3, 1, 4}},
		{`$`, "unexpected-character", [4]int{1, 1, 1, 2}},
		{`$.a`, "unexpected-character", [4]int{1, 1, 1, 2}},
		{`$"..."`, "unexpected-character", [4]int{1, 1, 1, 2}},
		{`# comment`, "unexpected-character", [4]int{1, 1, 1, 2}},
		{`@`, "unexpected-character", [4]int{1, 1, 1, 2}},
		{`a | b`, "unexpected-token", [4]int{1, 3, 1, 4}},
		{`if (c) { a }`, "unexpected-token", [4]int{1, 13, 1, 13}},
		{`if (c) { a } 2`, "unexpected-token", [4]int{1, 14, 1, 15}},
		{`if (c) { a: 1 } else { 2 }`, "unexpected-token", [4]int{1, 8, 1, 9}},
		{`if (c) { a, b } else { 2 }`, "unexpected-token", [4]int{1, 8, 1, 9}},
		{`if (c) { ...a } else { 2 }`, "unexpected-token", [4]int{1, 8, 1, 9}},
		{`if (c) { 1 } else { "a": 2 }`, "unexpected-token", [4]int{1, 19, 1, 20}},
		{`if (c) {} else { 2 }`, "unexpected-token", [4]int{1, 8, 1, 9}},
		{`try {} catch { 2 }`, "unexpected-token", [4]int{1, 5, 1, 6}},
		{`if (c) { 1 } else { 2 } + 3`, "unexpected-token", [4]int{1, 25, 1, 26}},
		{`if (c) { 1 } else { 2 }.a`, "unexpected-token", [4]int{1, 24, 1, 25}},
		{`if (c) { 1 } else { 2 }[0]`, "unexpected-token", [4]int{1, 24, 1, 25}},
		{`if (c) { 1 } else { 2 } ? 1 : 2`, "unexpected-token", [4]int{1, 25, 1, 26}},
		{`if (c) { 1 } else { 2 }?.a`, "unexpected-token", [4]int{1, 24, 1, 26}},
		{`if (c) { 1 } else { 2 }?.[0]`, "unexpected-token", [4]int{1, 24, 1, 27}},
		{`try { 1 } catch { 2 } * 3`, "unexpected-token", [4]int{1, 23, 1, 24}},
		{`1 + if (c) { 1 } else { 2 }`, "unexpected-token", [4]int{1, 5, 1, 7}},
		{`-try { 1 } catch { 2 }`, "unexpected-token", [4]int{1, 2, 1, 5}},
		{`try(e, d)`, "unexpected-token", [4]int{1, 1, 1, 4}},
		{`try { e }`, "unexpected-token", [4]int{1, 10, 1, 10}},
		{`try { e } 1`, "unexpected-token", [4]int{1, 11, 1, 12}},
		{`a ?? b > c`, "unexpected-token", [4]int{1, 8, 1, 9}},
		{`a ?? b || c`, "unexpected-token", [4]int{1, 8, 1, 10}},
		{`a > b ?? c`, "unexpected-token", [4]int{1, 7, 1, 9}},
		{`a ?? b not in c`, "unexpected-token", [4]int{1, 8, 1, 14}},
		{`!a in b`, "unexpected-token", [4]int{1, 1, 1, 2}},
		{`!a not in b`, "unexpected-token", [4]int{1, 1, 1, 2}},
		{`-2 ** 2`, "unexpected-token", [4]int{1, 1, 1, 2}},
		{`!a ** b`, "unexpected-token", [4]int{1, 1, 1, 2}},
		{`2 ** -2 ** 2`, "unexpected-token", [4]int{1, 6, 1, 7}},
		{`- -2 ** 2`, "unexpected-token", [4]int{1, 1, 1, 2}},
		{`!-2 ** 2`, "unexpected-token", [4]int{1, 1, 1, 2}},
		{`!!a in b`, "unexpected-token", [4]int{1, 1, 1, 2}},
		{`!-a in b`, "unexpected-token", [4]int{1, 1, 1, 2}},
		{`match {a: 1} { _ => 1 }`, "unexpected-token", [4]int{1, 7, 1, 8}},
		{`match if (c) { 1 } else { 2 } { _ => 1 }`, "unexpected-token", [4]int{1, 7, 1, 9}},
		{`match try { 1 } catch { 2 } { _ => 1 }`, "unexpected-token", [4]int{1, 7, 1, 10}},
		{`const {a} = x => x; a`, "unexpected-token", [4]int{1, 15, 1, 17}},
		{`const [a] = (x) => x; a`, "unexpected-token", [4]int{1, 17, 1, 19}},
		{`const _ = x => x; 1`, "unexpected-token", [4]int{1, 13, 1, 15}},
		{`let`, "reserved-word", [4]int{1, 1, 1, 4}},
		{`var x = 1; x`, "reserved-word", [4]int{1, 1, 1, 4}},
		{`for`, "reserved-word", [4]int{1, 1, 1, 4}},
		{`while (x) {}`, "reserved-word", [4]int{1, 1, 1, 6}},
		{`do`, "reserved-word", [4]int{1, 1, 1, 3}},
		{`[break]`, "reserved-word", [4]int{1, 2, 1, 7}},
		{`f(continue)`, "reserved-word", [4]int{1, 3, 1, 11}},
		{`function f() {}`, "reserved-word", [4]int{1, 1, 1, 9}},
		{`throw x`, "reserved-word", [4]int{1, 1, 1, 6}},
		{`finally`, "reserved-word", [4]int{1, 1, 1, 8}},
		{`import x`, "reserved-word", [4]int{1, 1, 1, 7}},
		{`try { 1 } catch { 2 } finally { 3 }`, "unexpected-token", [4]int{1, 23, 1, 30}},
		{`x[a:b]`, "unexpected-token", [4]int{1, 4, 1, 5}},
		{`f((x: int) => x)`, "unexpected-token", [4]int{1, 5, 1, 6}},
		{`(x: int) => x`, "unexpected-token", [4]int{1, 3, 1, 4}},
		{`const n: int = 1; n`, "unexpected-token", [4]int{1, 8, 1, 9}},
		{`xs.*.f`, "unexpected-token", [4]int{1, 4, 1, 5}},
		{`xs[?c]`, "unexpected-token", [4]int{1, 4, 1, 5}},
		{`xs[].f`, "unexpected-token", [4]int{1, 4, 1, 5}},
		{`<T>x`, "unexpected-token", [4]int{1, 1, 1, 2}},
		{`<<EOF`, "unexpected-token", [4]int{1, 1, 1, 2}},
		{"\"abc\nd\"", "unterminated-string", [4]int{1, 1, 1, 5}},
		{"\"abc\\\nd\"", "unterminated-string", [4]int{1, 1, 1, 6}},
		{"`a\\\nb`", "line-continuation", [4]int{1, 3, 1, 4}},
		// The last row: the general rule of 3.5.
		{`if c { a } else { b }`, "unexpected-token", [4]int{1, 4, 1, 5}},
		{`if (c) a else b`, "unexpected-token", [4]int{1, 8, 1, 9}},
		{`if(c, a, b)`, "unexpected-token", [4]int{1, 5, 1, 6}},
		{`try e catch d`, "unexpected-token", [4]int{1, 5, 1, 6}},
		{`try { 1 } catch (e: T) { 2 }`, "unexpected-token", [4]int{1, 19, 1, 20}},
		{`try { 1 } catch ({message}) { 2 }`, "unexpected-token", [4]int{1, 18, 1, 19}},
		{`try { 1 } catch (_) { 2 }`, "unexpected-token", [4]int{1, 18, 1, 19}},
		{`try { 1 } catch () { 2 }`, "unexpected-token", [4]int{1, 18, 1, 19}},
		{`switch (x) { case 1: 2 }`, "unexpected-token", [4]int{1, 12, 1, 13}},
		{`typeof x`, "unexpected-token", [4]int{1, 8, 1, 9}},
		{`x instanceof T`, "unexpected-token", [4]int{1, 3, 1, 13}},
		{`x satisfies T`, "unexpected-token", [4]int{1, 3, 1, 12}},
		{`interface X {}`, "unexpected-token", [4]int{1, 11, 1, 12}},
		{`enum E { A }`, "unexpected-token", [4]int{1, 6, 1, 7}},
		{`type Name = T`, "unexpected-token", [4]int{1, 6, 1, 10}},
		{`export const x = 1;`, "unexpected-token", [4]int{1, 8, 1, 13}},
		{`f(x => { x + 1 })`, "unexpected-token", [4]int{1, 12, 1, 13}},
		{`a?[i]`, "unexpected-token", [4]int{1, 6, 1, 6}},
		{`{max-retries: 3}`, "unexpected-token", [4]int{1, 5, 1, 6}},
	})
}

// 01-language.md 3.4: the rules the productions do not show, and what stays
// valid next to them.
func TestGrammarRules(t *testing.T) {
	checkDumps(t, [][2]string{
		{`(-2) ** 2`, `(** (- 2) 2)`},
		{`-(2 ** 2)`, `(- (** 2 2))`},
		{`2 ** -1`, `(** 2 (- 1))`},
		{`a ** b ** c`, `(** a (** b c))`},
		{`a not in b`, `(not-in a b)`},
		{`(!a) in b`, `(in (! a) b)`},
		{`!(a in b)`, `(! (in a b))`},
		{`a ?? b ?? c`, `(?? (?? a b) c)`},
		{`(a ?? b) > c`, `(> (?? a b) c)`},
		{`a + b ?? c * d`, `(?? (+ a b) (* c d))`},
		{`a || b && c`, `(|| a (&& b c))`},
		{`a ? b : c ? d : e`, `(?: a b (?: c d e))`},
		{`a ? b ? c : d : e`, `(?: a (?: b c d) e)`},
		{`a == b != c`, `(!= (== a b) c)`},
		{`a < b + c * d`, `(< a (+ b (* c d)))`},
		{`!!a`, `(! (! a))`},
		{`- -a`, `(- (- a))`},
		{`--a`, `(- (- a))`},
		{`-a.b`, `(- (. a b))`},
		{`c ? if (x) { 1 } else { 2 } : try { 3 } catch { 4 }`, `(?: c (if x 1 2) (try 3 4))`},
		{`(if (c) { 1 } else { 2 }) + 3`, `(+ (if c 1 2) 3)`},
		{`if (a) { 1 } else if (b) { 2 } else { 3 }`, `(if a 1 (if b 2 3))`},
		{`if (a) { const x = 1; x } else { { b } }`, `(if a (block (const x 1) x) (map (entry b)))`},
		{`if (a) { {id: 1} } else { [] }`, `(if a (map (entry "id" 1)) (list))`},
		{`{const: 1}`, `(map (entry "const" 1))`},
		{`{ const a = 1; a }`, `(block (const a 1) a)`},
		{`f(x => {id: x.id})`, `(call f (lambda (x) (map (entry "id" (. x id)))))`},
		{`f((a, b) => a)`, `(call f (lambda (a b) a))`},
		{`f(_ => 1)`, `(call f (lambda (_) 1))`},
		{`f((a) + 1)`, `(call f (+ a 1))`},
	})
	checkErrors(t, []errCase{
		{`f((a, b = 2, {c, d: e = 3}, [g, _, ...h], ...) => a)`, "unexpected-token", [4]int{1, 43, 1, 46}},
		{`(a, b) + 1`, "unexpected-token", [4]int{1, 3, 1, 4}},
		{`f((a, b) + 1)`, "unexpected-token", [4]int{1, 5, 1, 6}},
		{`f(x)(y)`, "unexpected-token", [4]int{1, 5, 1, 6}},
		{`(x => x)(1)`, "unexpected-token", [4]int{1, 4, 1, 6}},
		{`x => 1`, "unexpected-token", [4]int{1, 3, 1, 5}},
		{`[x => 1]`, "unexpected-token", [4]int{1, 4, 1, 6}},
		{`f(x => y => 1)`, "unexpected-token", [4]int{1, 10, 1, 12}},
		{`a not b`, "unexpected-token", [4]int{1, 7, 1, 8}},
		{`match x {}`, "unexpected-token", [4]int{1, 10, 1, 11}},
		{`match x { _ | 1 => 2 }`, "unexpected-token", [4]int{1, 13, 1, 14}},
		{"match x { `a` => 2 }", "unexpected-token", [4]int{1, 11, 1, 14}},
		{`match x { -a => 2 }`, "unexpected-token", [4]int{1, 12, 1, 13}},
		{`{,}`, "unexpected-token", [4]int{1, 2, 1, 3}},
		{`const {,} = x; 1`, "unexpected-token", [4]int{1, 8, 1, 9}},
		{`const [,] = x; 1`, "unexpected-token", [4]int{1, 8, 1, 9}},
		{`const {...r, a} = x; 1`, "unexpected-token", [4]int{1, 14, 1, 15}},
		{`const {_} = x; 1`, "unexpected-token", [4]int{1, 8, 1, 9}},
		{`_`, "unexpected-token", [4]int{1, 1, 1, 2}},
		{`const a = 1;`, "unexpected-token", [4]int{1, 13, 1, 13}},
		{`1; 2`, "unexpected-token", [4]int{1, 4, 1, 5}},
		{"`${}`", "unexpected-token", [4]int{1, 4, 1, 5}},
		{"`${a b}`", "unexpected-token", [4]int{1, 6, 1, 7}},
		{"1 `a${b}c`", "unexpected-token", [4]int{1, 3, 1, 11}},
		{`{ const a = 1; }`, "unexpected-token", [4]int{1, 16, 1, 17}},
		{`{ const a = 1; a: 2 }`, "unexpected-token", [4]int{1, 17, 1, 18}},
		{`{1: 2}`, "unexpected-token", [4]int{1, 2, 1, 3}},
		{`a ? b`, "unexpected-token", [4]int{1, 6, 1, 6}},
	})
}

func TestTrees(t *testing.T) {
	checkDumps(t, [][2]string{
		{`a.b?.c.d`, `(optchain (. (?. (. a b) c) d))`},
		{`(a?.b).c`, `(. (optchain (?. a b)) c)`},
		{`a?.[0]`, `(optchain (?.[] a 0))`},
		{`x.m(1, 2,)`, `(.m x 1 2)`},
		{`x?.m()`, `(optchain (?.m x))`},
		{`ns.f(a)`, `(.f ns a)`},
		{`f()`, `(call f)`},
		{`xs.map(double)`, `(.map xs double)`},
		{`[1, ...xs, 2,]`, `(list 1 (... xs) 2)`},
		{`{a, "b-c": 1, [k]: v, ...m,}`, `(map (entry a) (entry "b-c" 1) (entry k v) (... m))`},
		{`[...if (c) { a } else { b }]`, `(list (... (if c a b)))`},
		{"`a${b}c${d}`", `(template "a" b "c" d "")`},
		{"`plain`", `"plain"`},
		{"`a${`b${c}`}`", `(template "a" (template "b" c "") "")`},
		{"`${ {a: 1} }`", `(template "" (map (entry "a" 1)) "")`},
		{"`a\r\nb\rc`", `"a\nb\nc"`},
		{"`\\r`", `"\r"`},
		{`match x { 1 | -2 => "a", "s" | null => "b", true => 1, _ => 2, }`,
			`(match x (1|-2 "a") ("s"|null "b") (true 1) (_ 2))`},
		{`match -0 { -0 => 1 }`, `(match (- 0) (0 1))`},
		{`try { 1 } catch (e) { e.code }`, `(try 1 e (. e code))`},
		{`try { const a = 1; a } catch { const b = 2; b }`, `(try (block (const a 1) a) (block (const b 2) b))`},
		{`5.a`, `(. 5 a)`},
		{`1.5e3`, `1500`},
		{`0x1F`, `31`},
		{`1_000`, `1000`},
		{`1e20`, `1e+20`},
		{`9007199254740993.0`, `9.007199254740992e+15`},
		{`1e-400`, `0`},
		{`0`, `0`},
		{`0.5`, `0.5`},
		{`0x0000000000000001F`, `31`},
		{`"\uD83D\uDE00\u{1F600}\u0041\t\/"`, `"😀😀A\t/"`},
	})
	prog := parse(t, `const f = (x, n = 2) => x * n; const {a, b: c = 1, ...r} = m; const [p, _, q = 0, ...s] = l; f(3)`)
	assert.Equal(t, Dump(prog),
		`(program (const f (lambda (x (= n 2)) (* x n))) (const {a "b":c=1 ...r} m) (const [p _ q=0 ...s] l) (call f 3))`)
}

func TestLexical(t *testing.T) {
	checkErrors(t, []errCase{
		// Numbers (2.5).
		{`.5`, "bad-number", [4]int{1, 1, 1, 3}},
		{`5.`, "bad-number", [4]int{1, 1, 1, 3}},
		{`(5.)`, "bad-number", [4]int{1, 2, 1, 4}},
		{`00`, "bad-number", [4]int{1, 1, 1, 3}},
		{`1_`, "bad-number", [4]int{1, 1, 1, 3}},
		{`0x`, "bad-number", [4]int{1, 1, 1, 3}},
		{`0X1F`, "bad-number", [4]int{1, 1, 1, 5}},
		{`0b1`, "bad-number", [4]int{1, 1, 1, 4}},
		{`0o7`, "bad-number", [4]int{1, 1, 1, 4}},
		{`3in x`, "bad-number", [4]int{1, 1, 1, 4}},
		{`1e`, "bad-number", [4]int{1, 1, 1, 3}},
		{`1e+`, "bad-number", [4]int{1, 1, 1, 4}},
		{`1.2.3`, "bad-number", [4]int{1, 1, 1, 6}},
		{`x.0`, "bad-number", [4]int{1, 2, 1, 4}},
		{`0x20000000000000`, "literal-range", [4]int{1, 1, 1, 17}},
		{`12345678901234567890`, "literal-range", [4]int{1, 1, 1, 21}},
		// Escapes (2.7), one row each.
		{`"\0"`, "bad-escape", [4]int{1, 2, 1, 4}},
		{`"\v"`, "bad-escape", [4]int{1, 2, 1, 4}},
		{"\"\\`\"", "bad-escape", [4]int{1, 2, 1, 4}},
		{`"\$"`, "bad-escape", [4]int{1, 2, 1, 4}},
		{`"\u12"`, "bad-escape", [4]int{1, 2, 1, 6}},
		{`"\uZ"`, "bad-escape", [4]int{1, 2, 1, 4}},
		{`"\u{}"`, "bad-escape", [4]int{1, 2, 1, 5}},
		{`"\u{1234567}"`, "bad-escape", [4]int{1, 2, 1, 12}},
		{`"\u{12"`, "bad-escape", [4]int{1, 2, 1, 7}},
		{`"\u{110000}"`, "bad-escape", [4]int{1, 2, 1, 12}},
		{`"\u{D800}"`, "bad-escape", [4]int{1, 2, 1, 10}},
		{`"\uD800"`, "bad-escape", [4]int{1, 2, 1, 8}},
		{`"\uD800\u0041"`, "bad-escape", [4]int{1, 2, 1, 8}},
		{`"\uDC00"`, "bad-escape", [4]int{1, 2, 1, 8}},
		{"`\\x`", "bad-escape", [4]int{1, 2, 1, 4}},
		// The first lexical diagnostic comes first (2.11).
		{"\"a\\q\n", "bad-escape", [4]int{1, 3, 1, 5}},
		{`) "abc`, "unterminated-string", [4]int{1, 3, 1, 7}},
		{`const match = 1; @`, "unexpected-character", [4]int{1, 18, 1, 19}},
		{`R"a\"b"`, "unexpected-token", [4]int{1, 1, 1, 2}},
		// Unterminated literals.
		{"\"abc\r\n", "unterminated-string", [4]int{1, 1, 1, 5}},
		{"'abc\rx'", "unterminated-string", [4]int{1, 1, 1, 5}},
		{"`a${b", "unterminated-string", [4]int{1, 1, 1, 6}},
		{"`a${`b${c}", "unterminated-string", [4]int{1, 5, 1, 11}},
		{"`a${`b`", "unterminated-string", [4]int{1, 1, 1, 8}},
		{"`a\\", "unterminated-string", [4]int{1, 1, 1, 4}},
		{`"a\`, "unterminated-string", [4]int{1, 1, 1, 4}},
		// Characters (2.10).
		{"1\u00a0+ 2", "unexpected-character", [4]int{1, 2, 1, 3}},
		{"1 + \ufeff2", "unexpected-character", [4]int{1, 5, 1, 6}},
		{"\ufeff)", "unexpected-token", [4]int{1, 1, 1, 2}},
		{"\ufeff\ufeff1", "unexpected-character", [4]int{1, 1, 1, 2}},
		{"1 \x7f", "unexpected-character", [4]int{1, 3, 1, 4}},
		{"1 \u0085", "unexpected-character", [4]int{1, 3, 1, 4}},
		{"`${\"abc", "unterminated-string", [4]int{1, 4, 1, 8}},
		{"é", "unexpected-character", [4]int{1, 1, 1, 2}},
		{"1 \x01", "unexpected-character", [4]int{1, 3, 1, 4}},
		{`\`, "unexpected-character", [4]int{1, 1, 1, 2}},
		{`^`, "unexpected-character", [4]int{1, 1, 1, 2}},
		// Positions: lines at U+000A only, columns in code points.
		{"1 +\n\"é\" @", "unexpected-character", [4]int{2, 5, 2, 6}},
		{"1\r+ @", "unexpected-character", [4]int{1, 5, 1, 6}},
		{"\"😀\" @", "unexpected-character", [4]int{1, 5, 1, 6}},
		{"1 +\n\t@", "unexpected-character", [4]int{2, 2, 2, 3}},
	})
	checkDumps(t, [][2]string{
		{"\ufeff1", `1`},
		{"/* a /* b */ 1", `1`},
		{"1 // c\r+ 2", `(+ 1 2)`},
		{"1 /* * / */ + 2 // end", `(+ 1 2)`},
		{`"a//b"`, `"a//b"`},
		{`""`, `""`},
		{`"" + ''`, `(+ "" "")`},
		{`"'"`, `"'"`},
		{`'"'`, `"\""`},
		{`[1,2]?.[0]`, `(optchain (?.[] (list 1 2) 0))`},
	})
	// "?. [" is "?." and then "[", which is not a word.
	code, span := failure(`a?. [0]`)
	assert.Equal(t, code, "unexpected-token")
	assert.Equal(t, span, [4]int{1, 5, 1, 6})
}

// findIdent returns the first Ident named name in the tree.
func findIdent(n Node, name string) *Ident {
	var found *Ident
	Walk(n, func(n Node) bool {
		if id, ok := n.(*Ident); ok && id.Name == name && found == nil {
			found = id
		}
		return found == nil
	})
	return found
}

// 01-language.md 13.2: the table of levels open at x.
func TestNestingDepth(t *testing.T) {
	cases := []struct {
		src   string
		depth int
	}{
		{`x`, 0},
		{`f(x)`, 1},
		{`a + b * x`, 2},
		{`a + b + x`, 1},
		{`-(x)`, 2},
		{`c ? x : y`, 1},
		{`c ? y : x`, 0},
		{"`${[x]}`", 2},
		{`if (a) { 1 } else if (b) { 2 } else { x }`, 1},
		{`match x { 1 => 2 }`, 1},
	}
	for _, c := range cases {
		id := findIdent(parse(t, c.src), "x")
		assert.NotNil(t, id, c.src)
		assert.Equal(t, id.Depth(), c.depth, "depth of x in %q", c.src)
	}
	// What opens no level costs no level: these each parse.
	n := 100000
	var b strings.Builder
	for i := 0; i < n; i++ {
		fmt.Fprintf(&b, "const a%d = %d;\n", i, i)
	}
	b.WriteString("a0")
	big := []string{
		strings.Repeat("1 + ", n) + "1",
		"x" + strings.Repeat(".f()", n),
		strings.Repeat("c ? 1 : ", n) + "2",
		"if (c) { 1 }" + strings.Repeat(" else if (c) { 1 }", n) + " else { 2 }",
		b.String(),
		strings.Repeat("a ?? ", n) + "b",
		"[" + strings.Repeat("1, ", n) + "]",
	}
	for _, src := range big {
		parse(t, src)
	}
	// Each kind of level counts toward the 100.
	checkErrors(t, []errCase{
		{strings.Repeat("-", 101) + "1", "nesting-exceeded", [4]int{1, 101, 1, 102}},
		{strings.Repeat("!", 100) + "1", "", [4]int{}},
		{"1" + strings.Repeat(" + (1", 51) + strings.Repeat(")", 51), "nesting-exceeded", [4]int{1, 253, 1, 254}},
		{strings.Repeat("{a: ", 101) + "1" + strings.Repeat("}", 101), "nesting-exceeded", [4]int{1, 401, 1, 402}},
		{strings.Repeat("`${", 101) + "1" + strings.Repeat("}`", 101), "nesting-exceeded", [4]int{1, 302, 1, 304}},
		{strings.Repeat("c ? ", 101) + "1", "nesting-exceeded", [4]int{1, 403, 1, 404}},
		{strings.Repeat("match ", 101) + "x", "nesting-exceeded", [4]int{1, 601, 1, 606}},
		{strings.Repeat("a ** ", 101) + "x", "nesting-exceeded", [4]int{1, 503, 1, 505}},
		{strings.Repeat("a not in (", 51) + "x", "nesting-exceeded", [4]int{1, 503, 1, 509}},
	})
}

// Spans of nodes (01-language.md 4.3).
func TestSpans(t *testing.T) {
	src := `(a + b) * c`
	f := NewFile(src)
	prog := parse(t, src)
	mul := prog.Result.(*Binary)
	assert.Equal(t, f.Span(mul.Span().Start, mul.Span().End).Array(), [4]int{1, 1, 1, 12})
	add := mul.X.(*Binary)
	assert.Equal(t, f.Span(add.Span().Start, add.Span().End).Array(), [4]int{1, 2, 1, 7})

	src = "const x = {a: [1]};\nif (x) { x?.a[0] } else { -x.b.m(1) };"
	f = NewFile(src)
	prog = parse(t, src)
	sp := func(n Node) [4]int { return f.Span(n.Span().Start, n.Span().End).Array() }
	assert.Equal(t, sp(prog), [4]int{1, 1, 2, 39})
	assert.Equal(t, sp(prog.Consts[0]), [4]int{1, 1, 1, 20})
	m := prog.Consts[0].Value.(*Map)
	assert.Equal(t, sp(m), [4]int{1, 11, 1, 19})
	assert.Equal(t, sp(m.Entries[0]), [4]int{1, 12, 1, 18})
	cond := prog.Result.(*Cond)
	assert.Equal(t, sp(cond), [4]int{2, 1, 2, 38})
	oc := cond.Then.(*OptChain)
	assert.Equal(t, sp(oc), [4]int{2, 10, 2, 17})
	assert.Equal(t, sp(oc.X), [4]int{2, 10, 2, 17})
	u := cond.Else.(*Unary)
	assert.Equal(t, sp(u), [4]int{2, 27, 2, 36})
	mc := u.X.(*MethodCall)
	assert.Equal(t, sp(mc), [4]int{2, 28, 2, 36})

	src = "f((x, y = 1) => x, z => z)"
	f = NewFile(src)
	call := parse(t, src).Result.(*Call)
	l := call.Args[0].(*Lambda)
	assert.Equal(t, sp(l), [4]int{1, 3, 1, 18})
	assert.Equal(t, sp(l.Params[1]), [4]int{1, 7, 1, 12})
	assert.Equal(t, sp(call.Args[1]), [4]int{1, 20, 1, 26})
	assert.Equal(t, sp(call), [4]int{1, 1, 1, 27})

	src = `match x { "a" | "b" => 1, _ => 2 }`
	f = NewFile(src)
	mt := parse(t, src).Result.(*Match)
	assert.Equal(t, sp(mt), [4]int{1, 1, 1, 35})
	assert.Equal(t, sp(mt.Arms[0]), [4]int{1, 11, 1, 25})
	assert.Equal(t, sp(mt.Arms[1]), [4]int{1, 27, 1, 33})

	src = "try { 1 } catch (e) { e }"
	f = NewFile(src)
	assert.Equal(t, sp(parse(t, src).Result), [4]int{1, 1, 1, 26})

	src = "`a${b}c`"
	f = NewFile(src)
	assert.Equal(t, sp(parse(t, src).Result), [4]int{1, 1, 1, 9})

	src = "const {a, \"b\": c = 1} = m; [a]"
	f = NewFile(src)
	mp := parse(t, src).Consts[0].Pattern.(*MapPat)
	assert.Equal(t, sp(mp), [4]int{1, 7, 1, 22})
	assert.Equal(t, f.Span(mp.Fields[1].Span.Start, mp.Fields[1].Span.End).Array(), [4]int{1, 11, 1, 21})
}
