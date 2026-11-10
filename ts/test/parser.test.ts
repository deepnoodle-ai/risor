import { describe, test } from "node:test";
import assert from "node:assert/strict";
import { err, ok } from "./helpers.ts";
import type * as A from "../src/syntax/ast.ts";
import { NODE_NUMBER } from "../src/syntax/ast.ts";

const result = (src: string) => ok(src).result;

describe("01 section 15: every example", () => {
  // Examples whose outcome is a parse diagnostic.
  test("parse failures", () => {
    err("-2 ** 2", "needs-parentheses", [1, 1, 1, 2]); // 8
    err("9007199254740992", "literal-range", [1, 1, 1, 17]); // 9
    err("010", "bad-number", [1, 1, 1, 4]); // 10
    err("if (true) { 1 }", "missing-else", [1, 16, 1, 16]); // 37
    err("if (true) { a: 1 } else { 2 }", "branch-like-map", [1, 11, 1, 12]); // 38
    err("try 1 catch { 2 }", "try-form", [1, 5, 1, 6]); // 48
    err("try(1, 2)", "try-call", [1, 1, 1, 4]); // 49
    err("const match = 1; match", "reserved-word", [1, 7, 1, 12]); // 50
    err("a ?? b > c", "needs-parentheses", [1, 8, 1, 9]); // 52
  });
  // Every other example parses.
  const parses = [
    "7 / 2", "-7 % 3", "2 ** 10", "2 ** -1", "9007199254740991 + 1", "1e308 * 10", "1 / 0",
    "0.1 + 0.2", "`${1e21} ${0.000001} ${1e-7}`", 'u.name || "anonymous"', 'u.name ?? "anonymous"',
    "!u.tags", "[] && 1", "1 == true", "[1, [2]] == [1, [2]]", '"a" < "b" && "ab" > "a"', '"n=" + 5',
    '"" in "abc"', '"x" in {x: null}', "1 in null", "u.missing", "u?.missing", "null?.a.b.c",
    "(null?.a).b", "5?.a", '"héllo"[1]', "[1, 2][2]", "[1, 2]?.[2]",
    "{ const [a, ...r] = [1, 2, 3]; {a, r} }", "{ const {a = 5, b} = {a: null, b: 2}; a + b }",
    "{ const {a} = {}; a }", "{ const f = (x, n = 2) => x * n; f(3) + f(3, null) }",
    'if (0) { "a" } else if ("") { "b" } else { "c" }', "(if (true) { 1 } else { 2 }) + 3",
    'match 2 { 1 | 2 => "low", _ => "high" }', 'match "c" { "a" => 1, "b" => 2 }',
    '{...{a: 1, b: 2}, b: 3, [("c")]: 4}', "[...null]", '`${[1, "a"]}-${null}-${{b: 1, a: true}}`',
    "try { 1 / 0 } catch (e) { e.code }", 'try { fail("no", {n: 1}) } catch (e) { e }',
    'try { [][0] } catch { "none" }', "{if: 1}.if",
  ];
  for (const src of parses) test(`parses: ${src}`, () => void ok(src));

  test("shapes of some examples", () => {
    const e4 = result("2 ** -1") as A.Binary;
    assert.equal(e4.op, "**");
    assert.equal((e4.y as A.Unary).op, "-");
    const e27 = result("(null?.a).b") as A.Member;
    assert.equal(e27.kind, "Member");
    assert.equal(e27.x.kind, "OptChain");
    const e26 = result("null?.a.b.c") as A.OptChain;
    assert.equal(e26.kind, "OptChain");
    assert.equal((e26.x as A.Member).name, "c");
    const e40 = result('match 2 { 1 | 2 => "low", _ => "high" }') as A.Match;
    assert.equal(e40.arms.length, 2);
    assert.deepEqual(e40.arms[0].literals.map((l) => (l as A.NumberLit).value), [1, 2]);
    assert.equal(e40.arms[1].isDefault, true);
    const e47 = result('try { [][0] } catch { "none" }') as A.Try;
    assert.equal(e47.name, null);
    const e36 = result('if (0) { "a" } else if ("") { "b" } else { "c" }') as A.Cond;
    assert.equal(e36.form, "if");
    assert.equal((e36.else as A.Cond).form, "if");
  });
});

describe("01 14.2: each parse code of Core", () => {
  test("unexpected-token", () => err("1 2", "unexpected-token", [1, 3, 1, 4]));
  test("unexpected-character", () => err("a # b", "hash-comment", [1, 3, 1, 4]));
  test("unterminated-string", () => {
    err('"abc', "unterminated-string", [1, 1, 1, 5]);
    err("`abc", "unterminated-string", [1, 1, 1, 5]);
  });
  test("unterminated-comment", () => err("1 /* x", "unterminated-comment", [1, 3, 1, 5]));
  test("bad-escape", () => err(String.raw`"\x41"`, "bad-escape", [1, 2, 1, 4]));
  test("line-continuation", () => err("`a\\\nb`", "line-continuation", [1, 3, 1, 4]));
  test("bad-number", () => err("010", "bad-number", [1, 1, 1, 4]));
  test("literal-range", () => err("1e400", "literal-range", [1, 1, 1, 6]));
  test("nesting-exceeded", () => err("(".repeat(101) + "1" + ")".repeat(101), "nesting-exceeded", [1, 101, 1, 102]));
  test("bad-type-text (package F4); a text block parses with package F3", () => {
    ok('"""\n  a\n  """');
    ok("x as int");
    err("x as T", "bad-type-text", [1, 6, 1, 7]);
  });
  test("reserved-word: every example of the rule", () => {
    err("const match = 1;", "reserved-word", [1, 7, 1, 12]);
    err("is => 1", "reserved-word", [1, 1, 1, 3]);
    err("f(is => 1)", "reserved-word", [1, 3, 1, 5]);
    err("let x = 1;", "let-binding", [1, 1, 1, 4]);
    err("return x", "return-statement", [1, 1, 1, 7]);
    err("nil", "nil-literal", [1, 1, 1, 4]);
    err("[for (x of xs) x]", "loop-statement", [1, 2, 1, 5]);
    err("{for}", "loop-statement", [1, 2, 1, 5]);
    err("1 + else", "reserved-word", [1, 5, 1, 9]);
  });
  test("reserved-word where a Name is needed", () => {
    err("const f = (if) => 1; 1", "reserved-word", [1, 12, 1, 14]);
    err("const {a: in} = x; 1", "reserved-word", [1, 11, 1, 13]);
    err("const {in} = x; 1", "reserved-word", [1, 8, 1, 10]);
    err("const [for] = x; 1", "reserved-word", [1, 8, 1, 11]);
    err("const [...do] = x; 1", "reserved-word", [1, 11, 1, 13]);
    err("try { 1 } catch (catch) { 2 }", "reserved-word", [1, 18, 1, 23]);
    err("1 + const", "reserved-word", [1, 5, 1, 10]);
    err("not x", "reserved-word", [1, 1, 1, 4]);
  });
  test("a reserved word after . and ?. and as a bare key is a word", () => {
    ok("x.nil");
    ok("err.catch");
    ok("{for: 1, import: 2}");
    ok("x?.if");
    ok("x.match(1)");
    ok("const {if: a} = x; a");
  });
});

describe("01 14.3: every row", () => {
  test("a text block", () => {
    err('"""a"""', "text-block-open", [1, 1, 1, 4]);
    err("'''a'''", "text-block-quote", [1, 1, 1, 4]);
  });
  test("the prefixed literal", () => {
    ok('r"a"'); // a raw string, with package F3
    ok("r'a'");
    err('f"x"', "string-prefix", [1, 1, 1, 2]);
    err('R"x"', "string-prefix", [1, 1, 1, 2]);
    err("r`x`", "tagged-template", [1, 1, 1, 2]);
    err("String.raw`x`", "tagged-template", [1, 8, 1, 11]);
  });
  test("r or R, white space, then a quote", () => {
    err('r "x"', "string-prefix", [1, 1, 1, 2]);
    err("R 'x'", "string-prefix", [1, 1, 1, 2]);
    err('f(r /* c */ "x")', "string-prefix", [1, 3, 1, 4]);
    err('q "x"', "unexpected-token", [1, 3, 1, 6]);
  });
  test("as and is (package F4, f04 2.4, 2.6)", () => {
    err("x as T", "bad-type-text", [1, 6, 1, 7]);
    err("x is T", "bad-type-text", [1, 6, 1, 7]);
    err("x as const", "as-const", [1, 3, 1, 5]);
    err("x is not T", "negated-is", [1, 3, 1, 5]);
    err("a + b as T", "bad-type-text", [1, 10, 1, 11]);
    err("x as int + 1", "needs-parentheses", [1, 10, 1, 11]);
  });
  test("!x as T", () => {
    err("!x as T", "needs-parentheses", [1, 1, 1, 2]);
    err("!x is T", "needs-parentheses", [1, 1, 1, 2]);
    err("a == !x is T", "needs-parentheses", [1, 6, 1, 7]);
    err("(!x) is T", "bad-type-text", [1, 9, 1, 10]);
  });
  test("a projection (package F7, f07 2.2)", () => {
    ok("xs[*]");
    ok("xs?.[*].a");
    err("xs[*1]", "unexpected-token", [1, 5, 1, 6]);
    err("[*]", "unexpected-token", [1, 2, 1, 3]);
  });
  test("match with no subject", () => err("match { 1 => 2 }", "needs-parentheses", [1, 7, 1, 8]));
  test("=== and !==", () => {
    err("a === b", "strict-equality", [1, 3, 1, 6]);
    err("a !== b", "strict-equality", [1, 3, 1, 6]);
  });
  test("x = 1", () => err("x = 1", "assignment", [1, 3, 1, 4]));
  test("x += 1, x ++ and the rest of the last row of 2.3", () => {
    err("x += 1", "assignment", [1, 3, 1, 5]);
    err("x ++", "assignment", [1, 3, 1, 5]);
    err("x++", "assignment", [1, 2, 1, 4]);
    err("x ??= 1", "assignment", [1, 3, 1, 6]);
  });
  test("x |> f", () => err("x |> f", "pipe-operator", [1, 3, 1, 5]));
  test("& $ # @", () => {
    err("a & b", "bitwise-operator", [1, 3, 1, 4]);
    err("$", "context-symbol", [1, 1, 1, 2]);
    err("$.a", "context-symbol", [1, 1, 1, 2]);
    err('$"..."', "string-prefix", [1, 1, 1, 2]);
    err("# comment", "hash-comment", [1, 1, 1, 2]);
    err("@", "unexpected-character", [1, 1, 1, 2]);
  });
  test("a | b outside a match arm", () => err("a | b", "bitwise-operator", [1, 3, 1, 4]));
  test("if with no else", () => {
    err("if (c) { a }", "missing-else", [1, 13, 1, 13]);
    err("if (c) { a } + 1", "missing-else", [1, 14, 1, 15]);
  });
  test("a branch written like a map; an empty branch", () => {
    err("if (c) { a: 1 } else { 2 }", "branch-like-map", [1, 8, 1, 9]);
    err("if (c) { a, b } else { 2 }", "branch-like-map", [1, 8, 1, 9]);
    err("if (c) { ...a } else { 2 }", "branch-like-map", [1, 8, 1, 9]);
    err('if (c) { "a": 1 } else { 2 }', "branch-like-map", [1, 8, 1, 9]);
    err("if (c) { 1 } else { a: 1 }", "branch-like-map", [1, 19, 1, 20]);
    err("if (c) {} else { 2 }", "empty-branch", [1, 8, 1, 9]);
    err("try {} catch { 2 }", "empty-branch", [1, 5, 1, 6]);
    err("try { 1 } catch { a, b }", "branch-like-map", [1, 17, 1, 18]);
  });
  test("an if or a try followed by an operator, ., [, ?. or ?.[", () => {
    err("if (c) { 1 } else { 2 } + 3", "needs-parentheses", [1, 25, 1, 26]);
    err("if (c) { 1 } else { 2 }.x", "needs-parentheses", [1, 24, 1, 25]);
    err("if (c) { 1 } else { 2 }[0]", "needs-parentheses", [1, 24, 1, 25]);
    err("if (c) { 1 } else { 2 }?.x", "needs-parentheses", [1, 24, 1, 26]);
    err("if (c) { 1 } else { 2 }?.[0]", "needs-parentheses", [1, 24, 1, 27]);
    err("if (c) { 1 } else { 2 } ? 1 : 2", "needs-parentheses", [1, 25, 1, 26]);
    err("try { 1 } catch { 2 } ?? 3", "needs-parentheses", [1, 23, 1, 25]);
  });
  test("an if or a try where an operand is needed", () => {
    err("1 + if (c) { 1 } else { 2 }", "needs-parentheses", [1, 5, 1, 7]);
    err("-try { 1 } catch { 2 }", "needs-parentheses", [1, 2, 1, 5]);
  });
  test("try(e, d)", () => err("try(e, d)", "try-call", [1, 1, 1, 4]));
  test("try with no catch", () => {
    err("try { e }", "missing-catch", [1, 10, 1, 10]);
    err("try { e } 1", "missing-catch", [1, 11, 1, 12]);
  });
  test("B9 and E23 apply only after the => of a lambda, not of a match arm (f08 2.4, 2.7)", () => {
    err("match-3{true=>{", "unexpected-token", [1, 16, 1, 16]);
    err("[1].map(x => { a b })", "block-without-const", [1, 14, 1, 15]);
  });
  test("a = after a string or computed key is unexpected-token, not E23 (f08 2.7)", () => {
    err('f(x => { "a" = 1 })', "unexpected-token", [1, 14, 1, 15]);
    err("f(x => { [k] = 1 })", "unexpected-token", [1, 14, 1, 15]);
    err('{ "a" = 1 }', "unexpected-token", [1, 7, 1, 8]);
    err("f(x => { a = 1 })", "block-without-const", [1, 8, 1, 9]);
  });
  test("an earlier row wins over a row the parser meets at a fixed place (f08 2.1 item 2)", () => {
    // C5 before D13, D11 and D15: a token that no rule accepts.
    err("try{m}|>", "pipe-operator", [1, 7, 1, 9]);
    err("if (c) { 1 } === 2", "strict-equality", [1, 14, 1, 17]);
    err("try { 1 } catch (|>) { 2 }", "pipe-operator", [1, 18, 1, 20]);
    // E3 before E4 and E6.
    err("try finally", "finally-clause", [1, 5, 1, 12]);
    err("if finally", "finally-clause", [1, 4, 1, 11]);
    err("if (c) finally", "finally-clause", [1, 8, 1, 15]);
    // E3, E10 and E13 before E21.
    err("a?[1] finally", "finally-clause", [1, 7, 1, 14]);
    err("a?[1] | 2", "bitwise-operator", [1, 7, 1, 8]);
    err("a?[1] satisfies", "satisfies-operator", [1, 7, 1, 16]);
    err("a?[1] === 2", "strict-equality", [1, 7, 1, 10]);
  });
  test("rules 3, 2, 1 and 6 of 3.4", () => {
    err("a ?? b > c", "needs-parentheses", [1, 8, 1, 9]);
    err("a ?? b || c", "needs-parentheses", [1, 8, 1, 10]);
    err("a > b ?? c", "needs-parentheses", [1, 7, 1, 9]);
    err("a || b ?? c", "needs-parentheses", [1, 8, 1, 10]);
    err("a ?? b == c", "needs-parentheses", [1, 8, 1, 10]);
    err("a ?? b in c", "needs-parentheses", [1, 8, 1, 10]);
    err("!a in b", "needs-parentheses", [1, 1, 1, 2]);
    err("!a not in b", "needs-parentheses", [1, 1, 1, 2]);
    err("-2 ** 2", "needs-parentheses", [1, 1, 1, 2]);
    err("!a ** b", "needs-parentheses", [1, 1, 1, 2]);
    err("2 ** -2 ** 2", "needs-parentheses", [1, 6, 1, 7]);
    // 01 3.4 rule 1: the first prefix operator in the text; `not in` whole.
    err("- -2 ** 2", "needs-parentheses", [1, 1, 1, 2]);
    err("-!a ** 2", "needs-parentheses", [1, 1, 1, 2]);
    err("!!a in b", "needs-parentheses", [1, 1, 1, 2]);
    err("a ?? b not in c", "needs-parentheses", [1, 8, 1, 14]);
    err("match {a: 1} { _ => 1 }", "needs-parentheses", [1, 7, 1, 8]);
    err("match if (c) { 1 } else { 2 } { _ => 1 }", "needs-parentheses", [1, 7, 1, 9]);
    err("match try { 1 } catch { 2 } { _ => 1 }", "needs-parentheses", [1, 7, 1, 10]);
  });
  test("the valid forms beside rules 1 to 3", () => {
    ok("(-2) ** 2");
    ok("-(2 ** 2)");
    ok("2 ** -1");
    ok("a not in b");
    ok("(!a) in b");
    ok("(a ?? b) > c");
    ok("a ?? (b || c)");
    ok("a + b ?? c * d");
    ok("a ?? b ? c : d");
    ok("match (if (c) { 1 } else { 2 }) { _ => 1 }");
    ok("match ({a: 1}) { _ => 1 }");
  });
  test("const {a} = x => x;", () => {
    err("const {a} = x => x; 1", "lambda-in-pattern", [1, 15, 1, 17]);
    err("const _ = x => x; 1", "lambda-in-pattern", [1, 13, 1, 15]);
    err("const [a] = (x) => x; 1", "lambda-in-pattern", [1, 17, 1, 19]);
  });
  test("let, var, nil, ... at a place where a value may start", () => {
    // f08 2.4 B1 to B8.
    const codes: Record<string, string> = {
      let: "let-binding", var: "let-binding", nil: "nil-literal", for: "loop-statement", while: "loop-statement", do: "loop-statement",
      break: "loop-statement", continue: "loop-statement", return: "return-statement", function: "function-declaration",
      throw: "throw-statement", finally: "finally-clause", import: "import-statement",
    };
    for (const w of Object.keys(codes)) {
      err(`${w}`, codes[w], [1, 1, 1, 1 + w.length]);
      err(`f(1, ${w})`, codes[w], [1, 6, 1, 6 + w.length]);
      err(`match x { ${w} => 1 }`, codes[w], [1, 11, 1, 11 + w.length]);
    }
  });
  test("finally after the handler of a try", () => {
    err("try { 1 } catch { 2 } finally { 3 }", "finally-clause", [1, 23, 1, 30]);
  });
  test("x[a:b]; (x: int) => x; const n: int = 1;", () => {
    err("x[a:b]", "slice-syntax", [1, 4, 1, 5]);
    err("(x: int) => x", "type-annotation", [1, 3, 1, 4]);
    err("f((x: int) => x)", "type-annotation", [1, 5, 1, 6]);
    err("const n: int = 1; n", "type-annotation", [1, 8, 1, 9]);
  });
  test("xs.*.f; xs[?c]; xs[].f", () => {
    err("xs.*.f", "star-form", [1, 4, 1, 5]);
    err("xs[?c]", "filter-step", [1, 4, 1, 5]);
    err("xs[].f", "star-form", [1, 4, 1, 5]);
  });
  test("<T>x, <<EOF", () => {
    err("<T>x", "angle-cast", [1, 1, 1, 2]);
    err("<<EOF", "heredoc", [1, 1, 1, 2]);
  });
  test("a line end in a one-line string", () => err('"ab\nc"', "unterminated-string", [1, 1, 1, 4]));
  test("a \\ before a line end in a template", () => err("`ab\\\nc`", "line-continuation", [1, 4, 1, 5]));
  test("every other text that package F8 targets (f08 2)", () => {
    err("if c { a } else { b }", "if-form", [1, 4, 1, 5]);
    err("if (c) a else b", "if-form", [1, 8, 1, 9]);
    err("if(c, a, b)", "if-form", [1, 5, 1, 6]);
    err("try e catch d", "try-form", [1, 5, 1, 6]);
    err("try { 1 } catch (e: T) { 2 }", "catch-name", [1, 19, 1, 20]);
    err("try { 1 } catch ({message}) { 2 }", "catch-name", [1, 18, 1, 19]);
    err("try { 1 } catch (_) { 2 }", "catch-name", [1, 18, 1, 19]);
    err("try { 1 } catch () { 2 }", "catch-name", [1, 18, 1, 19]);
    err("switch (x) { }", "switch-statement", [1, 1, 1, 7]);
    err("typeof x", "typeof-operator", [1, 1, 1, 7]);
    err("x instanceof T", "instanceof-operator", [1, 3, 1, 13]);
    err("x satisfies T", "satisfies-operator", [1, 3, 1, 12]);
    err("interface Foo { }", "type-declaration", [1, 1, 1, 10]);
    err("enum Color { }", "type-declaration", [1, 1, 1, 5]);
    err("type Name = T", "type-declaration", [1, 1, 1, 5]);
    err("export const x = 1; x", "export-statement", [1, 1, 1, 7]);
    err("x => { x + 1 }", "unexpected-token", [1, 3, 1, 5]);
    err("f(x => { x + 1 })", "block-without-const", [1, 8, 1, 9]);
    err("a?[i]", "optional-index", [1, 2, 1, 3]);
    err("{max-retries: 3}", "unquoted-key", [1, 2, 1, 13]);
  });
});

describe("01 3.2: how the parser chooses", () => {
  test("rule 1: block or map", () => {
    assert.equal(result("{const: 1}").kind, "Map");
    assert.equal(result("{ const a = 1; a }").kind, "Block");
    assert.equal(result("{}").kind, "Map");
    err("{ const }", "unexpected-token", [1, 9, 1, 10]);
  });
  test("rule 2: a branch is never a map", () => {
    const c = result("if (c) { {id: 1} } else { {} }") as A.Cond;
    assert.equal(c.then.kind, "Map");
    assert.equal(c.else.kind, "Map");
  });
  test("rule 5: a lambda where an argument or a const value stands", () => {
    const call = result("f((a, b) => a, x => x, _ => 1, () => 2)") as A.Call;
    assert.deepEqual(call.args.map((a) => a.kind), ["Lambda", "Lambda", "Lambda", "Lambda"]);
    assert.equal(((call.args[2] as A.Lambda).params[0].pattern as A.NamePat).name, null);
    const p = ok("const f = (x, {a, b: c = 1, ...r}, [d, _, e = 2, ...s], n = 3) => x; f(1)");
    assert.equal((p.consts[0].value as A.Lambda).params.length, 4);
    assert.equal(result("f((a))").kind, "Call");
    assert.equal((result("f((a))") as A.Call).args[0].kind, "Ident");
  });
  test("rule 6: the matching )", () => {
    assert.equal((result("f((a)[0])") as A.Call).args[0].kind, "Index");
    err("f((a, b))", "unexpected-token", [1, 5, 1, 6]);
    assert.equal((result("f(({a}) => a)") as A.Call).args[0].kind, "Lambda");
  });
  test("rules 7 and 8: entries and fields", () => {
    const m = result("{a: 1, b, 'c': 2, [k]: 3, ...d}") as A.MapLit;
    assert.deepEqual(m.entries.map((e) => (e.kind === "Entry" ? `${e.key.kind}:${e.shorthand}` : e.kind)), [
      "word:false", "word:true", "string:false", "expr:false", "Spread",
    ]);
  });
  test("rules 9 and 10: calls and reads", () => {
    assert.equal(result("x.m(1)").kind, "MethodCall");
    assert.equal(result("x.m").kind, "Member");
    assert.equal(result("ns.f(1)").kind, "MethodCall");
    assert.equal(result("f(1)").kind, "Call");
  });
  test("rule 11: the lambda body is the longest Expr", () => {
    const c = result("f(x => x + 1 ? 2 : 3, y)") as A.Call;
    assert.equal(c.args.length, 2);
    assert.equal((c.args[0] as A.Lambda).body.kind, "Cond");
  });
  test("calls only by name", () => {
    err("f(x)(y)", "unexpected-token", [1, 5, 1, 6]);
    err("(x => x)(1)", "unexpected-token", [1, 4, 1, 6]);
  });
});

describe("01 3.3: precedence and associativity", () => {
  const show = (e: A.Expr): string => {
    switch (e.kind) {
      case "Binary": return `(${show(e.x)} ${e.op} ${show(e.y)})`;
      case "Unary": return `(${e.op}${show(e.x)})`;
      case "Cond": return `(${show(e.test)} ? ${show(e.then)} : ${show(e.else)})`;
      case "Ident": return e.name;
      case "Number": return String(e.value);
      default: return e.kind;
    }
  };
  const cases: [string, string][] = [
    ["a ? b : c ? d : e", "(a ? b : (c ? d : e))"],
    ["a || b && c", "(a || (b && c))"],
    ["a && b || c", "((a && b) || c)"],
    ["a == b < c", "(a == (b < c))"],
    ["a < b + c", "(a < (b + c))"],
    ["a + b * c", "(a + (b * c))"],
    ["a - b - c", "((a - b) - c)"],
    ["a ** b ** c", "(a ** (b ** c))"],
    ["-a * b", "((-a) * b)"],
    ["!!a", "(!(!a))"],
    ["- -a", "(-(-a))"],
    ["--a", "(-(-a))"],
    ["a ?? b ?? c", "((a ?? b) ?? c)"],
    ["a in b == c not in d", "((a in b) == (c not-in d))"],
    ["1 < 2 < 3", "((1 < 2) < 3)"],
    ["a * b ** c", "(a * (b ** c))"],
  ];
  for (const [src, want] of cases) test(src, () => assert.equal(show(result(src)), want));
});

describe("01 4: nodes and spans", () => {
  test("32 node kinds", () => {
    // Core's 30, `As` of package F4 (f04 2.7) and `Each` of package F7 (f07 2.3).
    assert.equal(Object.keys(NODE_NUMBER).length, 32);
    assert.equal(NODE_NUMBER.Bad, 30);
    assert.equal(NODE_NUMBER.As, 31);
    assert.equal(NODE_NUMBER.Each, 32);
  });
  test("parentheses belong to the node that holds them", () => {
    const b = result("(a + b) * c") as A.Binary;
    assert.deepEqual(b.span, { start: 0, end: 11 });
    assert.deepEqual(b.x.span, { start: 1, end: 6 });
    const u = result("-(x)") as A.Unary;
    assert.deepEqual(u.span, { start: 0, end: 4 });
    const m = result("(a).b") as A.Member;
    assert.deepEqual(m.span, { start: 0, end: 5 });
  });
  test("spans by node", () => {
    const p = ok("const a = 1; a;");
    assert.deepEqual(p.span, { start: 0, end: 15 });
    assert.deepEqual(p.consts[0].span, { start: 0, end: 12 });
    const c = result("x?.f(1)[0].g") as A.OptChain;
    assert.deepEqual(c.span, { start: 0, end: 12 });
    const call = result("f(a, b)") as A.Call;
    assert.deepEqual(call.span, { start: 0, end: 7 });
    const lam = (result("f(x => x + 1)") as A.Call).args[0] as A.Lambda;
    assert.deepEqual(lam.span, { start: 2, end: 12 });
    const cond = result("if (a) { 1 } else if (b) { 2 } else { 3 }") as A.Cond;
    assert.deepEqual(cond.span, { start: 0, end: 41 });
    assert.deepEqual(cond.else.span, { start: 18, end: 41 });
    const t = result("try { 1 } catch (e) { 2 }") as A.Try;
    assert.deepEqual(t.span, { start: 0, end: 25 });
    const s = result("`a`") as A.StringLit;
    assert.equal(s.kind, "String");
    assert.equal(s.form, "`");
    assert.deepEqual(s.span, { start: 0, end: 3 });
    const arm = (result("match x { -1 | 2 => 3 }") as A.Match).arms[0];
    assert.deepEqual(arm.span, { start: 10, end: 21 });
    assert.equal((arm.literals[0] as A.NumberLit).value, -1);
    assert.deepEqual(arm.literals[0].span, { start: 10, end: 12 });
    const e = (result("{a: (1)}") as A.MapLit).entries[0];
    assert.deepEqual(e.span, { start: 1, end: 7 });
  });
  test("a branch with const lines is a Block; without, its expression", () => {
    const c = result("if (a) { const b = 1; b } else { 2 }") as A.Cond;
    assert.equal(c.then.kind, "Block");
    assert.equal(c.else.kind, "Number");
  });
  test("a template with interpolations", () => {
    const t = result("`a ${b} c ${d}`") as A.Template;
    assert.equal(t.parts.length, 4);
    assert.equal(t.parts[0], "a ");
  });
  test("an interpolation holds one Expr", () => {
    err("`${}`", "unexpected-token", [1, 4, 1, 5]);
    err("`${1 2}`", "unexpected-token", [1, 6, 1, 7]);
    ok("`${ if (a) { 1 } else { 2 } }`");
  });
});

describe("01 13: nesting", () => {
  const nest = (n: number, inner: string) => "(".repeat(n) + inner + ")".repeat(n);
  // For each row of the table of 13.2, `x` replaced by text that opens the
  // levels left up to 100 parses, and one more fails.
  const rows: [string, number][] = [
    ["x", 0], ["f(x)", 1], ["a + b * x", 2], ["a + b + x", 1], ["-(x)", 2], ["c ? x : y", 1],
    ["c ? y : x", 0], ["`${[x]}`", 2], ["if (a) { 1 } else if (b) { 2 } else { x }", 1], ["match x { 1 => 2 }", 1],
  ];
  for (const [text, levels] of rows) {
    test(`levels open at x in ${text}: ${levels}`, () => {
      const at = text.lastIndexOf("x");
      const sub = (n: number) => text.slice(0, at) + nest(n, "z") + text.slice(at + 1);
      if (text.startsWith("match x")) {
        // x is the subject: the word match is open.
        ok(`match ${nest(99, "z")} { 1 => 2 }`);
        err(`match ${nest(100, "z")} { 1 => 2 }`, "nesting-exceeded");
        return;
      }
      ok(sub(100 - levels));
      err(sub(101 - levels), "nesting-exceeded");
    });
  }
  test("each kind of opener", () => {
    err("[".repeat(101) + "]".repeat(101), "nesting-exceeded", [1, 101, 1, 102]);
    err("-".repeat(101) + "1", "nesting-exceeded", [1, 101, 1, 102]);
    err("!".repeat(101) + "1", "nesting-exceeded", [1, 101, 1, 102]);
    err(nest(100, "a ?? b"), "nesting-exceeded", [1, 103, 1, 105]);
    err(nest(100, "a not in b"), "nesting-exceeded", [1, 103, 1, 109]);
    err(nest(100, "c ? 1 : 2"), "nesting-exceeded", [1, 103, 1, 104]);
    err(nest(100, "match x { _ => 1 }"), "nesting-exceeded", [1, 101, 1, 106]);
    err(nest(100, "x?.[0]"), "nesting-exceeded", [1, 102, 1, 105]);
    err(nest(100, "`${1}`"), "nesting-exceeded", [1, 102, 1, 104]);
    err(nest(99, "{a: [1]}"), "nesting-exceeded", [1, 104, 1, 105]);
    err("2" + " ** 2".repeat(101), "nesting-exceeded", [1, 503, 1, 505]);
  });
  test("parsing ends at that token", () => {
    err(nest(101, "1 2"), "nesting-exceeded");
  });
  test("what opens no level costs no stack", () => {
    const n = 100000;
    ok(Array.from({ length: n }, (_, i) => String(i)).join(" + "));
    ok("x" + ".f()".repeat(n));
    ok("a ? 1 : ".repeat(n) + "2");
    ok("if (a) { 1 } " + "else if (b) { 2 } ".repeat(n) + "else { 3 }");
    ok("const a = 1; ".repeat(1).concat(Array.from({ length: n }, (_, i) => `const a${i} = ${i};`).join(" "), " a"));
    ok("[" + "1, ".repeat(n) + "]");
  });
});
