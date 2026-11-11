//! The lexer and parser against `01-language.md`: every example of section
//! 15, every row of the tables of 14.2 and 14.3, the spans of 4.3, and the
//! nesting of section 13. These are Core's diagnostics, before the mapping
//! of package F8 (`f08-errors.md` 2.1 item 1).

use crate::syntax::ast::{BinaryOp, CondForm, EntryKey, Lit, NodeKind, UnaryOp};
use crate::syntax::{parse, parse_core, Expr};

/// The code and span of the Core diagnostic.
fn err(src: &str) -> (&'static str, [u32; 4]) {
    match parse_core(src) {
        Ok(_) => panic!("expected a parse error for {src:?}"),
        Err(e) => {
            let d = e.first();
            (d.code, d.span.expect("a parse diagnostic has a span").arr())
        }
    }
}

fn ok(src: &str) -> Expr {
    match parse(src) {
        Ok(p) => p,
        Err(e) => panic!("{src:?} should parse: {e}"),
    }
}

/// The span `[1, a, 1, b]` on line 1.
fn l1(a: u32, b: u32) -> [u32; 4] {
    [1, a, 1, b]
}

fn check(src: &str, code: &str, span: [u32; 4]) {
    let (c, s) = err(src);
    assert_eq!((c, s), (code, span), "for {src:?}");
}

const UT: &str = "unexpected-token";
const RW: &str = "reserved-word";

// ---- section 15 ---------------------------------------------------------

#[test]
fn examples_of_section_15_that_parse() {
    for src in [
        "7 / 2",
        "-7 % 3",
        "2 ** 10",
        "2 ** -1",
        "9007199254740991 + 1",
        "1e308 * 10",
        "1 / 0",
        "0.1 + 0.2",
        "`${1e21} ${0.000001} ${1e-7}`",
        "u.name || \"anonymous\"",
        "u.name ?? \"anonymous\"",
        "!u.tags",
        "[] && 1",
        "1 == true",
        "[1, [2]] == [1, [2]]",
        "\"a\" < \"b\" && \"ab\" > \"a\"",
        "\"n=\" + 5",
        "\"\" in \"abc\"",
        "\"x\" in {x: null}",
        "1 in null",
        "u.missing",
        "u?.missing",
        "null?.a.b.c",
        "(null?.a).b",
        "5?.a",
        "\"h\u{e9}llo\"[1]",
        "[1, 2][2]",
        "[1, 2]?.[2]",
        "{ const [a, ...r] = [1, 2, 3]; {a, r} }",
        "{ const {a = 5, b} = {a: null, b: 2}; a + b }",
        "{ const {a} = {}; a }",
        "{ const f = (x, n = 2) => x * n; f(3) + f(3, null) }",
        "if (0) { \"a\" } else if (\"\") { \"b\" } else { \"c\" }",
        "(if (true) { 1 } else { 2 }) + 3",
        "match 2 { 1 | 2 => \"low\", _ => \"high\" }",
        "match \"c\" { \"a\" => 1, \"b\" => 2 }",
        "{...{a: 1, b: 2}, b: 3, [(\"c\")]: 4}",
        "[...null]",
        "`${[1, \"a\"]}-${null}-${{b: 1, a: true}}`",
        "try { 1 / 0 } catch (e) { e.code }",
        "try { fail(\"no\", {n: 1}) } catch (e) { e }",
        "try { [][0] } catch { \"none\" }",
        "{if: 1}.if",
        "[1, 2].map(_ => 0)",
        "2 ** -2000",
        "0.1 ** -400",
        "0 ** -1",
    ] {
        ok(src);
    }
}

#[test]
fn examples_of_section_15_that_do_not_parse() {
    check("-2 ** 2", UT, l1(1, 2));
    check("9007199254740992", "literal-range", l1(1, 17));
    check("010", "bad-number", l1(1, 4));
    check("if (true) { 1 }", UT, l1(16, 16));
    check("if (true) { a: 1 } else { 2 }", UT, l1(11, 12));
    check("try 1 catch { 2 }", UT, l1(5, 6));
    check("try(1, 2)", UT, l1(1, 4));
    check("const match = 1; match", RW, l1(7, 12));
    check("a ?? b > c", UT, l1(8, 9));
    // Core's `x as int` is an error; package F4 parses it (F4 10).
    ok("x as int");
    // Core's example 54; package F3 reads it as a raw string (F3 10).
    ok("r\"a\"");
    check("\"a\\q\"", "bad-escape", l1(3, 5));
    let deep = format!("{}1{}", "(".repeat(101), ")".repeat(101));
    check(&deep, "nesting-exceeded", l1(101, 102));
}

// ---- 14.2 ---------------------------------------------------------------

#[test]
fn table_14_2_unexpected_token() {
    check("1 2", UT, l1(3, 4));
    check("", UT, l1(1, 1));
    check("1 +", UT, l1(4, 4));
}

#[test]
fn table_14_2_unexpected_character() {
    check("a # b", "unexpected-character", l1(3, 4));
    check("a\u{a0}b", "unexpected-character", l1(2, 3));
    check("\u{e9}", "unexpected-character", l1(1, 2));
    check("1 \u{feff}", "unexpected-character", l1(3, 4));
    check("\u{7}", "unexpected-character", l1(1, 2));
    for c in ["^", "~", "\\", "@", "$"] {
        check(&format!("a {c}"), "unexpected-character", l1(3, 4));
    }
    // A U+FEFF at the start is ignored.
    ok("\u{feff}1");
}

#[test]
fn table_14_2_unterminated_string() {
    check("\"abc", "unterminated-string", l1(1, 5));
    check("'ab\ncd'", "unterminated-string", l1(1, 4));
    check("\"ab\r\n\"", "unterminated-string", l1(1, 4));
    // A `\` directly before the line end is part of the literal.
    check("\"ab\\\nc\"", "unterminated-string", l1(1, 5));
    check("`abc", "unterminated-string", l1(1, 5));
    check("`a${1", "unterminated-string", l1(1, 6));
    check("`a${`b${1}`", "unterminated-string", l1(1, 12));
}

#[test]
fn table_14_2_unterminated_comment() {
    check("1 /* x", "unterminated-comment", l1(3, 5));
    ok("/* a /* b */ 1");
    ok("1 // comment");
}

#[test]
fn table_14_2_bad_escape_rows_of_2_7() {
    check("\"\\x41\"", "bad-escape", l1(2, 4));
    check("\"\\0\"", "bad-escape", l1(2, 4));
    check("\"\\v\"", "bad-escape", l1(2, 4));
    check("\"\\`\"", "bad-escape", l1(2, 4));
    check("\"\\$\"", "bad-escape", l1(2, 4));
    check("\"\\u12g4\"", "bad-escape", l1(2, 6));
    check("\"\\u\"", "bad-escape", l1(2, 4));
    check("\"\\u{}\"", "bad-escape", l1(2, 5));
    check("\"\\u{1234567}\"", "bad-escape", l1(2, 12));
    check("\"\\u{12\"", "bad-escape", l1(2, 7));
    check("\"\\u{110000}\"", "bad-escape", l1(2, 12));
    check("\"\\u{D800}\"", "bad-escape", l1(2, 10));
    check("\"\\uD800x\"", "bad-escape", l1(2, 8));
    check("\"\\uD800\\u0041\"", "bad-escape", l1(2, 8));
    check("\"\\uDC00\"", "bad-escape", l1(2, 8));
    ok("\"\\uD83D\\uDE00 \\u{1F600} \\u00e9 \\/ \\b\\f\\n\\r\\t \\\" \\' \\\\\"");
    ok("`\\` \\$ \\u{41}`");
    // 2.11: `bad-escape` is met before `unterminated-string`.
    check("\"a\\q\n", "bad-escape", l1(3, 5));
}

#[test]
fn table_14_2_line_continuation() {
    check("`a\\\nb`", "line-continuation", l1(3, 4));
    check("`a\\\r\nb`", "line-continuation", l1(3, 4));
}

#[test]
fn table_14_2_bad_number() {
    for (src, end) in [
        (".5", 3),
        ("5.", 3),
        ("010", 4),
        ("00", 3),
        ("1__0", 5),
        ("1_", 3),
        ("0x", 3),
        ("0X1F", 5),
        ("0b1", 4),
        ("0o7", 4),
        ("3in", 4),
        ("5..", 4),
        ("1e", 3),
        ("1e+", 4),
    ] {
        check(src, "bad-number", l1(1, end));
    }
    check("5.)", "bad-number", l1(1, 3));
    ok("5.a");
    ok("1_000 + 0x1F + 0xff_ff + 1.5e3 + 1E-3 + 0e5 + 0.5");
}

#[test]
fn table_14_2_literal_range() {
    check("9007199254740992", "literal-range", l1(1, 17));
    check("0x20000000000000", "literal-range", l1(1, 17));
    check("1e400", "literal-range", l1(1, 6));
    ok("9007199254740991 + 0x1FFFFFFFFFFFFF + 1e20 + 9007199254740993.0 + 1e-400");
}

#[test]
fn table_14_2_nesting_exceeded() {
    let s = format!("{}1{}", "[".repeat(101), "]".repeat(101));
    check(&s, "nesting-exceeded", l1(101, 102));
    // `not in` is one span.
    let s = format!("{}a not in b{}", "(".repeat(100), ")".repeat(100));
    check(&s, "nesting-exceeded", l1(103, 109));
}

#[test]
fn table_14_2_reserved_word() {
    check("const match = 1; match", RW, l1(7, 12));
    check("is => 1", RW, l1(1, 3));
    check("let x = 1;", RW, l1(1, 4));
    check("return x", RW, l1(1, 7));
    check("nil", RW, l1(1, 4));
    check("[for (x of xs) x]", RW, l1(2, 5));
    check("{for}", RW, l1(2, 5));
    check("1 + else", RW, l1(5, 9));
    check("f((for) => 1)", RW, l1(4, 7));
    check("const [a, in] = x; a", RW, l1(11, 13));
    check("const {a: not} = x; a", RW, l1(11, 14));
    check("const {...do} = x; 1", RW, l1(11, 13));
    check("try { 1 } catch (catch) { 2 }", RW, l1(18, 23));
    check("(const a = 1; a)", RW, l1(2, 7));
    check("1 + const", RW, l1(5, 10));
    ok("x.nil");
    ok("err.catch");
    ok("{for: 1, import: 2}");
    ok("{const: 1}");
}

// ---- 14.3 ---------------------------------------------------------------

#[test]
fn table_14_3_text_blocks_and_prefixed_literals() {
    // Package F3 gives meaning to `"""` and to `r` before a quote (F3 10).
    check("\"\"\"abc\"\"\"", "text-block-open", l1(1, 4));
    check("'''abc", UT, l1(1, 4));
    ok("1 + \"\"\"\n x \n\"\"\"");
    assert_eq!(str_value("r\"a\\q\""), "a\\q");
    assert_eq!(str_value("r'a'"), "a");
    check("r'''a'''", UT, l1(1, 2));
    check("f\"x\"", UT, l1(1, 2));
    check("R\"x\"", UT, l1(1, 2));
    check("r`x`", UT, l1(1, 2));
    check("String.raw`x`", UT, l1(8, 11));
    check("r \"a\"", UT, l1(1, 2));
    check("R 'a'", UT, l1(1, 2));
    check("r /* c */ 'a'", UT, l1(1, 2));
    ok("x in\"abc\"");
    ok("\"\"");
    ok("''");
}

#[test]
fn table_14_3_as_and_is() {
    // With package F4 (its chapter 10): a one-letter `T` is `bad-type-text`;
    // the rows it keeps stay Core's.
    check("x as T", "bad-type-text", l1(6, 7));
    check("x is T", "bad-type-text", l1(6, 7));
    check("x as const", UT, l1(3, 5));
    check("x is not T", UT, l1(3, 5));
    check("!x as T", UT, l1(1, 2));
    check("!x is T", UT, l1(1, 2));
    check("a == !x is T", UT, l1(6, 7));
    check("a + !x as T", "bad-type-text", l1(11, 12));
    check("(!x) as T", "bad-type-text", l1(9, 10));
    ok("(!x) as bool");
    ok("x as int[] | null");
}

#[test]
fn table_14_3_projection_and_operators() {
    // Package F7 makes a projection a program (`f07-queries.md` 2.2 item 3).
    ok("xs[*]");
    ok("xs?.[*]");
    check("xs[*1]", UT, l1(5, 6));
    check("match { 1 => 2 }", UT, l1(7, 8));
    check("a === b", UT, l1(3, 6));
    check("a !== b", UT, l1(3, 6));
    check("x = 1", UT, l1(3, 4));
    for (src, end) in [
        ("x += 1", 5),
        ("x -= 1", 5),
        ("x *= 1", 5),
        ("x /= 1", 5),
        ("x %= 1", 5),
        ("x **= 1", 6),
        ("x ??= 1", 6),
        ("x ||= 1", 6),
        ("x &&= 1", 6),
        ("x ++", 5),
        ("x |> f", 5),
    ] {
        check(src, UT, l1(3, end));
    }
    check("a & b", "unexpected-character", l1(3, 4));
    check("$", "unexpected-character", l1(1, 2));
    check("$.a", "unexpected-character", l1(1, 2));
    check("$\"a\"", "unexpected-character", l1(1, 2));
    check("# comment", "unexpected-character", l1(1, 2));
    check("@", "unexpected-character", l1(1, 2));
    check("a | b", UT, l1(3, 4));
}

#[test]
fn table_14_3_if_and_try() {
    check("if (c) { a }", UT, l1(13, 13));
    check("if (c) { a: 1 } else { 2 }", UT, l1(8, 9));
    check("if (c) { a, b } else { 2 }", UT, l1(8, 9));
    check("if (c) { ...m } else { 2 }", UT, l1(8, 9));
    check("if (c) { 1 } else { a: 1 }", UT, l1(19, 20));
    check("if (c) {} else { 2 }", UT, l1(8, 9));
    check("try { 1 } catch { }", UT, l1(17, 18));
    check("if (c) { 1 } else { 2 } + 3", UT, l1(25, 26));
    check("if (c) { 1 } else { 2 }.x", UT, l1(24, 25));
    check("if (c) { 1 } else { 2 }[0]", UT, l1(24, 25));
    check("if (c) { 1 } else { 2 }?.x", UT, l1(24, 26));
    check("if (c) { 1 } else { 2 }?.[0]", UT, l1(24, 27));
    check("if (c) { 1 } else { 2 } ? 1 : 2", UT, l1(25, 26));
    check("try { 1 } catch { 2 } * 2", UT, l1(23, 24));
    check("1 + if (c) { 1 } else { 2 }", UT, l1(5, 7));
    check("-try { 1 } catch { 2 }", UT, l1(2, 5));
    check("try(e, d)", UT, l1(1, 4));
    check("try { e }", UT, l1(10, 10));
    check("try { 1 } catch { 2 } finally { 3 }", UT, l1(23, 30));
    ok("c ? if (a) { 1 } else { 2 } : try { 3 } catch { 4 }");
    ok("f(if (a) { 1 } else { 2 }, try { 1 } catch (e) { e })");
}

#[test]
fn table_14_3_rules_of_3_4() {
    check("a ?? b > c", UT, l1(8, 9));
    check("a ?? b || c", UT, l1(8, 10));
    check("a > b ?? c", UT, l1(7, 9));
    check("a ?? b == c", UT, l1(8, 10));
    check("a ?? b in c", UT, l1(8, 10));
    check("a ?? b not in c", UT, l1(8, 14));
    check("!a in b", UT, l1(1, 2));
    check("!a not in b", UT, l1(1, 2));
    check("-2 ** 2", UT, l1(1, 2));
    check("!a ** b", UT, l1(1, 2));
    check("2 ** -2 ** 2", UT, l1(6, 7));
    check("match {a: 1} { _ => 1 }", UT, l1(7, 8));
    check("match if (c) { 1 } else { 2 } { _ => 1 }", UT, l1(7, 9));
    check("match try { 1 } catch { 2 } { _ => 1 }", UT, l1(7, 10));
    check("const {a} = x => x; 1", UT, l1(15, 17));
    check("const [a] = (x) => x; 1", UT, l1(17, 19));
    check("const _ = x => x; 1", UT, l1(13, 15));
    ok("a ?? b ?? c");
    ok("a ?? b + c");
    ok("(a ?? b) > c");
    ok("a not in b");
    ok("(!a) in b");
    ok("(-2) ** 2");
    ok("-(2 ** 2)");
    ok("2 ** -1");
    ok("- -a");
    ok("!!a");
    ok("match (if (c) { 1 } else { 2 }) { _ => 1 }");
    ok("match ({a: 1}) { _ => 1 }");
}

#[test]
fn table_14_3_reserved_words_where_a_value_may_start() {
    for w in [
        "let", "var", "nil", "for", "while", "do", "break", "continue", "return", "function",
        "throw", "finally", "import",
    ] {
        let len = w.chars().count() as u32;
        check(w, RW, l1(1, 1 + len));
        check(&format!("f({w})"), RW, l1(3, 3 + len));
    }
}

#[test]
fn table_14_3_annotations_and_queries() {
    check("x[a:b]", UT, l1(4, 5));
    check("(x: int) => x", UT, l1(3, 4));
    check("f((x: int) => x)", UT, l1(5, 6));
    check("const n: int = 1; n", UT, l1(8, 9));
    check("xs.*.f", UT, l1(4, 5));
    check("xs[?c]", UT, l1(4, 5));
    check("xs[].f", UT, l1(4, 5));
    check("<T>x", UT, l1(1, 2));
    check("<<EOF", UT, l1(1, 2));
    check("\"a\nb\"", "unterminated-string", l1(1, 3));
    check("`a\\\n`", "line-continuation", l1(3, 4));
}

#[test]
fn table_14_3_every_other_text() {
    check("if c { a } else { b }", UT, l1(4, 5));
    check("if (c) a else b", UT, l1(8, 9));
    check("if(c, a, b)", UT, l1(5, 6));
    check("try e catch d", UT, l1(5, 6));
    check("try { 1 } catch (e: T) { 2 }", UT, l1(19, 20));
    check("try { 1 } catch ({message}) { 2 }", UT, l1(18, 19));
    check("try { 1 } catch (_) { 2 }", UT, l1(18, 19));
    check("try { 1 } catch () { 2 }", UT, l1(18, 19));
    check("switch (x) { }", UT, l1(12, 13));
    check("typeof x", UT, l1(8, 9));
    check("x instanceof T", UT, l1(3, 13));
    check("x satisfies T", UT, l1(3, 12));
    check("interface I { }", UT, l1(11, 12));
    check("enum E { }", UT, l1(6, 7));
    check("type Name = T", UT, l1(6, 10));
    check("export const x = 1;", UT, l1(8, 13));
    check("x => { x + 1 }", UT, l1(3, 5));
    check("f(x => { x + 1 })", UT, l1(12, 13));
    check("a?[i]", UT, l1(6, 6));
    check("{max-retries: 3}", UT, l1(5, 6));
}

// ---- grammar -------------------------------------------------------------

#[test]
fn other_grammar() {
    check("f(x)(y)", UT, l1(5, 6));
    check("(x => x)(1)", UT, l1(4, 6));
    check("{_}", UT, l1(2, 3));
    check("const {_} = x; 1", UT, l1(8, 9));
    check("{ const a = 1; a; }", UT, l1(17, 18));
    check("{ const a = 1; }", UT, l1(16, 17));
    check("match x { }", UT, l1(11, 12));
    check("match x { a => 1 }", UT, l1(11, 12));
    check("match x { `a` => 1 }", UT, l1(11, 14));
    check("match x { `a${1}` => 1 }", UT, l1(11, 18));
    check("match x { _ | 1 => 1 }", UT, l1(13, 14));
    check("{`a`: 1}", UT, l1(2, 5));
    check("const {...r, a} = x; 1", UT, l1(14, 15));
    check("[1 2]", UT, l1(4, 5));
    check("a not b", UT, l1(7, 8));
    check("1;2", UT, l1(3, 4));
    check("_", UT, l1(1, 2));
    ok("1;");
    ok("const a = 1; const f = (x, {y, z: w = 2, ...r}, [p, _, q = 3, ...s]) => x; f(a)");
    ok("[1, 2,]");
    ok("{a: 1, \"b-c\": 2, [k]: 3, d, ...m,}");
    ok("f(a, b,)");
    ok("match x { -1 | 0 => \"a\", \"s\" | true | false | null => 1, _ => 2, }");
    ok("{ const x = 1; x }");
    ok("if (c) { {id: 1} } else { {} }");
    check("{ {id: 1} }", UT, l1(3, 4));
    ok("x?.a?.[0]?.b(1).c");
    ok("`a ${ `b ${c} d` } e`");
    ok("const f = _ => 1; f(1)");
    ok("f((a, b) => a, (x) => x, () => 1, _ => 2, y => y)");
    ok("c ? a : b ? d : e");
    ok("try { 1 } catch { 2 }");
    ok("x.if.else.true");
}

#[test]
fn first_diagnostics_and_spans() {
    // A leading U+FEFF is removed and not counted.
    check("\u{feff})", UT, l1(1, 2));
    check("\u{feff}\u{feff}", "unexpected-character", l1(1, 2));
    // A `\` before the end of input.
    check("\"ab\\", "unterminated-string", l1(1, 5));
    check("`ab\\", "unterminated-string", l1(1, 5));
    // The innermost literal is met.
    check("`${\"abc", "unterminated-string", l1(4, 8));
    // The level of `match` closes before its `{` opens.
    ok(&format!(
        "{}match x {{ _ => 1 }}{}",
        "(".repeat(99),
        ")".repeat(99)
    ));
    // The outermost prefix operator.
    check("- -2 ** 2", UT, l1(1, 2));
    check("--2 ** 2", UT, l1(1, 2));
    check("!-2 ** 2", UT, l1(1, 2));
    check("!!a in b", UT, l1(1, 2));
    // `not in` that cannot follow.
    check("a ?? b not in c", UT, l1(8, 14));
    check("a ?? b not\n  in c", UT, [1, 8, 2, 5]);
    // A lambda where none may stand; a number after `.`.
    check("[x => x]", UT, l1(4, 6));
    check("x.0", "bad-number", l1(2, 4));
    // With F3, the text block's row: `text-block-open` at the same span.
    check("r \"\"\"a\"\"\"", "text-block-open", l1(3, 6));
    check("x.r \"a\"", UT, l1(3, 4));
    // A reserved word where a case needs a literal.
    check("match x { nil => 1 }", RW, l1(11, 14));
    check("match x { 1 | for => 1 }", RW, l1(15, 18));
    // A template where no token may stand; a `}` that closes too early.
    check("1 `a${b}c`", UT, l1(3, 11));
    check("`${}`", UT, l1(4, 5));
    check("`${a}${}`", UT, l1(8, 9));
    check("1 `a${`b${c}`}`", UT, l1(3, 16));
}

// ---- spans and nodes (4.2, 4.3) ------------------------------------------

#[test]
fn spans_of_nodes() {
    let p = ok("(a + b) * c");
    let root = p.ast.kind(p.ast.root);
    let NodeKind::Program { result, .. } = root else {
        panic!()
    };
    let NodeKind::Binary { op, x, .. } = p.ast.kind(*result) else {
        panic!()
    };
    assert_eq!(*op, BinaryOp::Mul);
    assert_eq!(p.source.line_span(p.ast.span(*result)).arr(), l1(1, 12));
    assert_eq!(p.source.line_span(p.ast.span(*x)).arr(), l1(2, 7));

    let p = ok("const a = 1;\n  a ;");
    assert_eq!(
        p.source.line_span(p.ast.span(p.ast.root)).arr(),
        [1, 1, 2, 6]
    );

    let p = ok("if (c) { a } else if (d) { b } else { e }");
    let NodeKind::Program { result, .. } = p.ast.kind(p.ast.root) else {
        panic!()
    };
    let NodeKind::Cond { form, else_, .. } = p.ast.kind(*result) else {
        panic!()
    };
    assert_eq!(*form, CondForm::If);
    assert_eq!(p.source.line_span(p.ast.span(*result)).arr(), l1(1, 42));
    assert_eq!(p.source.line_span(p.ast.span(*else_)).arr(), l1(19, 42));

    let p = ok("a?.b.c");
    let NodeKind::Program { result, .. } = p.ast.kind(p.ast.root) else {
        panic!()
    };
    assert!(matches!(p.ast.kind(*result), NodeKind::OptChain { .. }));
    assert_eq!(p.source.line_span(p.ast.span(*result)).arr(), l1(1, 7));

    let p = ok("match x { -5 => 1 }");
    let n = p
        .ast
        .nodes
        .iter()
        .find(|n| matches!(n.kind, NodeKind::Arm { .. }))
        .unwrap();
    let NodeKind::Arm {
        literals: Some(lits),
        ..
    } = &n.kind
    else {
        panic!()
    };
    assert_eq!(lits[0].lit, Lit::Number(-5.0));
    assert_eq!(p.source.line_span(n.span).arr(), l1(11, 18));

    let p = ok("{a, \"b\": 1}");
    let entries: Vec<_> = p
        .ast
        .nodes
        .iter()
        .filter(|n| matches!(n.kind, NodeKind::Entry { .. }))
        .collect();
    assert!(matches!(
        &entries[0].kind,
        NodeKind::Entry { key: EntryKey::Word(w), shorthand: true, .. } if w == "a"
    ));
    assert_eq!(p.source.line_span(entries[1].span).arr(), l1(5, 11));

    let p = ok("-x.y");
    let NodeKind::Program { result, .. } = p.ast.kind(p.ast.root) else {
        panic!()
    };
    assert!(matches!(
        p.ast.kind(*result),
        NodeKind::Unary {
            op: UnaryOp::Neg,
            ..
        }
    ));

    let p = ok("`a${1}`");
    let NodeKind::Program { result, .. } = p.ast.kind(p.ast.root) else {
        panic!()
    };
    assert_eq!(p.ast.kind(*result).number(), 7);
    let p = ok("`a`");
    let NodeKind::Program { result, .. } = p.ast.kind(p.ast.root) else {
        panic!()
    };
    assert_eq!(p.ast.kind(*result).number(), 6);
}

#[test]
fn spans_count_code_points() {
    check("\"\u{1F600}\" + \u{e9}", "unexpected-character", l1(7, 8));
    check("\"\u{1F600}\"\n  # x", "unexpected-character", [2, 3, 2, 4]);
}

// ---- nesting (13) ----------------------------------------------------------

fn wrap(n: usize, inner: &str) -> String {
    format!("{}{}{}", "(".repeat(n), inner, ")".repeat(n))
}

#[test]
fn levels_of_13_2() {
    // `a + b * x`: 2 levels at `x`.
    ok(&wrap(98, "a + b * x"));
    check(&wrap(99, "a + b * x"), "nesting-exceeded", l1(106, 107));
    // `a + b + x`: 1.
    ok(&wrap(99, "a + b + x"));
    // `-(x)`: 2.
    ok(&wrap(98, "-(x)"));
    check(&wrap(99, "-(x)"), "nesting-exceeded", l1(101, 102));
    // `c ? y : x`: 0 at `x`; `c ? x : y`: 1.
    ok(&wrap(99, "c ? x : y"));
    check(&wrap(100, "c ? x : y"), "nesting-exceeded", l1(103, 104));
    // A template's `${`.
    ok(&wrap(98, "`${[x]}`"));
    check(&wrap(99, "`${[x]}`"), "nesting-exceeded", l1(103, 104));
    check(&wrap(100, "`${x}`"), "nesting-exceeded", l1(102, 104));
    // The last branch of an `if`.
    ok(&wrap(99, "if (a) { 1 } else if (b) { 2 } else { x }"));
    check(
        &wrap(100, "if (a) { 1 } else { 2 }"),
        "nesting-exceeded",
        l1(104, 105),
    );
    // The word `match`.
    ok(&wrap(99, "match x { 1 => 2 }"));
    check(
        &wrap(100, "match x { 1 => 2 }"),
        "nesting-exceeded",
        l1(101, 106),
    );
    // Prefix operators and `**`.
    ok(&format!("{}x", "-".repeat(100)));
    check(
        &format!("{}x", "!".repeat(101)),
        "nesting-exceeded",
        l1(101, 102),
    );
    let pow = vec!["2"; 101].join(" ** ");
    ok(&pow);
    let pow = vec!["2"; 102].join(" ** ");
    check(&pow, "nesting-exceeded", l1(503, 505));
}

#[test]
fn what_opens_no_level_costs_no_stack() {
    let n = 100_000;
    ok(&vec!["1"; n].join(" + "));
    ok(&format!("x{}", ".f()".repeat(n)));
    ok(&format!("x{}", ".a[0]?.b".repeat(n)));
    let mut t = String::new();
    for _ in 0..n {
        t.push_str("c ? 1 : ");
    }
    t.push('2');
    ok(&t);
    let mut t = String::from("if (c) { 0 }");
    for _ in 0..n {
        t.push_str(" else if (c) { 1 }");
    }
    t.push_str(" else { 2 }");
    ok(&t);
    let mut t = String::from("{ ");
    for i in 0..n {
        t.push_str(&format!("const a{i} = {i}; "));
    }
    t.push_str("a0 }");
    ok(&t);
    let list = format!("[{}]", vec!["1"; n].join(", "));
    ok(&list);
}

#[test]
fn deep_text_ends_at_the_limit() {
    let n = 1_000_000;
    check(&wrap(n, "1"), "nesting-exceeded", l1(101, 102));
    check(
        &format!("{}1", "-".repeat(n)),
        "nesting-exceeded",
        l1(101, 102),
    );
    let mut t = String::new();
    for _ in 0..n {
        t.push_str("`${");
    }
    check(
        &t,
        "unterminated-string",
        [1, n as u32 * 3 - 2, 1, n as u32 * 3 + 1],
    );
    t.push('1');
    for _ in 0..n {
        t.push_str("}`");
    }
    check(&t, "nesting-exceeded", l1(302, 304));
}

// ---- package F3: what the suite cannot reach ------------------------------

fn str_value(src: &str) -> String {
    let p = ok(src);
    let NodeKind::Program { result, .. } = p.ast.kind(p.ast.root) else {
        panic!()
    };
    match p.ast.kind(*result) {
        NodeKind::String { value, .. } => value.clone(),
        k => panic!("{src:?}: {}", k.name()),
    }
}

/// F3 7.3: a block is read in time linear in its length.
#[test]
fn a_long_text_block_reads_in_linear_time() {
    // Two sizes are compared, so the test does not depend on how fast the
    // machine is: reading each earlier line again would take 64 times as
    // long for 8 times the text.
    fn block(n: usize) -> String {
        let mut src = String::from("\"\"\"\n");
        for _ in 0..n {
            src.push_str("    line of text\n");
        }
        src.push_str("    \"\"\"");
        src
    }
    fn time_of(n: usize) -> std::time::Duration {
        let src = block(n);
        let mut best = std::time::Duration::MAX;
        for _ in 0..3 {
            let start = std::time::Instant::now();
            let _ = str_value(&src);
            best = best.min(start.elapsed());
        }
        best
    }
    let n = 200_000;
    let small = time_of(n / 8);
    let large = time_of(n);
    assert!(
        large < small * 24 + std::time::Duration::from_millis(50),
        "{} lines took {:?}, {} lines took {:?}",
        n / 8,
        small,
        n,
        large
    );
    let v = str_value(&block(n));
    assert_eq!(v.len(), n * 13 - 1);
}

/// F3 7.2: only `${` opens a level; blocks nest through interpolations by
/// the lexer's own stack.
#[test]
fn blocks_nest_without_recursion() {
    let depth = 200_000;
    let mut src = String::new();
    for _ in 0..depth {
        src.push_str("\"\"\"\n${");
    }
    src.push('1');
    for _ in 0..depth {
        src.push_str("}\n\"\"\"");
    }
    // The 101st `${` is on line 102: each block's `${` follows its line end.
    check(&src, "nesting-exceeded", [102, 1, 102, 3]);
    let ok_depth = format!("{}1{}", "\"\"\"\n${".repeat(100), "}\n\"\"\"".repeat(100));
    ok(&ok_depth);
}

/// F3 4.6 and 5.3: lines ended by U+000D alone end block lines, but every
/// position stays on one line.
#[test]
fn carriage_returns_end_block_lines_but_not_positions() {
    assert_eq!(str_value("\"\"\"\r  a\r    b\r  \"\"\""), "a\n  b");
    check("\"\"\"\r  a\r b\r  \"\"\"", "text-block-indent", l1(10, 11));
    // Only U+0020 and U+0009 are white space for indentation.
    check(
        "\"\"\"\n\u{a0}a\n  \"\"\"",
        "text-block-indent",
        [2, 1, 2, 2],
    );
    assert_eq!(str_value("\"\"\"\n\u{3000}a\n\"\"\""), "\u{3000}a");
}
