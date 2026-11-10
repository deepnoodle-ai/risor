import { describe, test } from "node:test";
import assert from "node:assert/strict";
import { lex } from "../src/syntax/lexer.ts";
import { ParseError } from "../src/syntax/diagnostic.ts";
import { Source } from "../src/syntax/source.ts";
import type { Token } from "../src/syntax/token.ts";
import { err, ok } from "./helpers.ts";
import type { LS } from "./helpers.ts";

function tokens(src: string): Token[] {
  return lex(new Source(src)).filter((t) => t.kind !== "eof");
}

function texts(src: string): string[] {
  return tokens(src).map((t) => `${t.kind}:${t.kind === "number" ? t.num : t.kind === "string" ? t.str : t.text}`);
}

function lexErr(src: string, code: string, span: LS): void {
  const s = new Source(src);
  try {
    lex(s);
  } catch (e) {
    assert.ok(e instanceof ParseError);
    assert.equal(e.diagnostic.code, code, src);
    assert.deepEqual(s.lineSpan(e.diagnostic.span!), span, src);
    return;
  }
  assert.fail(`expected ${JSON.stringify(src)} to fail with ${code}`);
}

describe("2.1 source text and positions", () => {
  test("a U+FEFF at the start is ignored", () => {
    ok("\uFEFF1 + 2");
    err("\uFEFF$", "unexpected-character", [1, 1, 1, 2]);
  });
  test("a U+FEFF elsewhere is an unexpected character", () => {
    err("1 +\uFEFF2", "unexpected-character", [1, 4, 1, 5]);
  });
  test("lines count at U+000A only; U+000D is a column", () => {
    err("1\r\n2 $", "unexpected-character", [2, 3, 2, 4]);
    err("1\r2 $", "unexpected-character", [1, 5, 1, 6]);
  });
  test("a code point above U+FFFF is one column", () => {
    err('"😀" $', "unexpected-character", [1, 5, 1, 6]);
  });
  test("a tab is one column", () => {
    err("\t$", "unexpected-character", [1, 2, 1, 3]);
  });
});

describe("2.2 comments", () => {
  test("line and block comments are white space", () => {
    ok("1 // x\n+ 2");
    ok("1 /* x */ + 2");
    ok("1 // x\r+ 2");
  });
  test("block comments do not nest", () => {
    ok("/* a /* b */ 1");
    err("/* a /* b */ */ 1", "unexpected-token", [1, 14, 1, 15]);
  });
  test("unterminated comment", () => {
    err("1 /* x", "unterminated-comment", [1, 3, 1, 5]);
  });
  test("// and /* in a string or template text are not comments", () => {
    ok('"// x"');
    ok("`/* x`");
  });
});

describe("2.3 tokens", () => {
  test("longest token", () => {
    assert.deepEqual(texts(">= ... ?.[ ?. ?"), ["punct:>=", "punct:...", "punct:?.[", "punct:?.", "punct:?"]);
  });
  test("-- is not a token", () => {
    assert.deepEqual(texts("--x"), ["punct:-", "punct:-", "word:x"]);
  });
  test("?. [ with white space is ?. then [", () => {
    assert.deepEqual(texts("?. ["), ["punct:?.", "punct:["]);
  });
  test("each token that no rule accepts is one token and unexpected-token", () => {
    for (const t of ["===", "!==", "+=", "-=", "*=", "/=", "%=", "**=", "??=", "||=", "&&=", "++", "|>"]) {
      err(`a ${t} b`, "unexpected-token", [1, 3, 1, 3 + t.length]);
    }
  });
  test("the block token", () => {
    err('"""abc"""', "unexpected-token", [1, 1, 1, 4]);
    err("'''abc'''", "unexpected-token", [1, 1, 1, 4]);
    err('x + """\nabc', "unexpected-token", [1, 5, 1, 8]);
    // Nothing in a block token is an escape or a line end.
    err('"""\\q\n"""', "unexpected-token", [1, 1, 1, 4]);
    ok('""');
    ok("''");
  });
  test("the prefixed literal", () => {
    err('r"a\\"b"', "unexpected-token", [1, 1, 1, 2]);
    err("f'x'", "unexpected-token", [1, 1, 1, 2]);
    err("raw`a\nb`", "unexpected-token", [1, 1, 1, 4]);
    err('_"x"', "unexpected-token", [1, 1, 1, 2]);
  });
  test("the end of input is an empty span", () => {
    err("1 +", "unexpected-token", [1, 4, 1, 4]);
    err("", "unexpected-token", [1, 1, 1, 1]);
  });
});

describe("2.4 words", () => {
  test("only ASCII letters form words", () => {
    err("é", "unexpected-character", [1, 1, 1, 2]);
    ok("_a1");
  });
  test("a reserved word directly followed by a quote is the word and a literal", () => {
    ok('x in"abc"');
  });
});

describe("2.5 numbers", () => {
  const value = (src: string) => {
    const t = tokens(src);
    assert.equal(t.length, 1, src);
    return t[0].num;
  };
  test("values", () => {
    assert.equal(value("42"), 42);
    assert.equal(value("1_000"), 1000);
    assert.equal(value("0x1F"), 31);
    assert.equal(value("0x1_f"), 31);
    assert.equal(value("1.5e3"), 1500);
    assert.equal(value("1E-2"), 0.01);
    assert.equal(value("1e20"), 1e20);
    assert.equal(value("9007199254740993.0"), 9007199254740992);
    assert.equal(value("1e-400"), 0);
    assert.ok(Object.is(value("0.0"), 0));
    assert.equal(value("9007199254740991"), 9007199254740991);
    assert.equal(value("0x1FFFFFFFFFFFFF"), 9007199254740991);
  });
  test("rule 2: where a number token stops", () => {
    assert.deepEqual(texts("5.a"), ["number:5", "punct:.", "word:a"]);
    err("5.", "bad-number", [1, 1, 1, 3]);
    err("5.)", "bad-number", [1, 1, 1, 3]);
    err("0b1", "bad-number", [1, 1, 1, 4]);
    err("1__0", "bad-number", [1, 1, 1, 5]);
    err("3in", "bad-number", [1, 1, 1, 4]);
    err("1..2", "bad-number", [1, 1, 1, 5]);
    err("1e+", "bad-number", [1, 1, 1, 4]);
  });
  test("rule 4: bad-number", () => {
    for (const t of [".5", "5.", "010", "00", "1__0", "1_", "0x", "0X1F", "0b1", "0o7", "1_e5", "0_1", "0x_1", "1e5_"]) {
      err(t, "bad-number", [1, 1, 1, 1 + t.length]);
    }
  });
  test("0x1e+5 is a hex number, then + and 5", () => {
    assert.deepEqual(texts("0x1e+5"), ["number:30", "punct:+", "number:5"]);
  });
  test("rule 6: literal-range", () => {
    for (const t of ["9007199254740992", "0x20000000000000", "1e400", "99999999999999999999"]) {
      err(t, "literal-range", [1, 1, 1, 1 + t.length]);
    }
  });
  test("rule 8: a number token is never negative", () => {
    assert.deepEqual(texts("-5"), ["punct:-", "number:5"]);
  });
});

describe("2.6 and 2.7 strings and escapes", () => {
  const str = (src: string) => tokens(src)[0].str;
  test("values", () => {
    assert.equal(str(`"a'b"`), "a'b");
    assert.equal(str(`'a"b'`), 'a"b');
    assert.equal(str(String.raw`"\"\'\\\/\b\f\n\r\t"`), "\"'\\/\b\f\n\r\t");
    assert.equal(str(String.raw`"\u00e9\u{1F600}\u{0}"`), "é😀\u0000");
    assert.equal(str(String.raw`"\uD83D\uDE00"`), "😀");
  });
  test("unterminated", () => {
    err('"abc', "unterminated-string", [1, 1, 1, 5]);
    err('"ab\ncd"', "unterminated-string", [1, 1, 1, 4]);
    err('"ab\r\ncd"', "unterminated-string", [1, 1, 1, 4]);
    err('"ab\\\ncd"', "unterminated-string", [1, 1, 1, 5]);
  });
  test("each row of the bad-escape table", () => {
    err(String.raw`"\x41"`, "bad-escape", [1, 2, 1, 4]);
    err(String.raw`"\0"`, "bad-escape", [1, 2, 1, 4]);
    err(String.raw`"\v"`, "bad-escape", [1, 2, 1, 4]);
    err('"\\`"', "bad-escape", [1, 2, 1, 4]);
    err(String.raw`"\$"`, "bad-escape", [1, 2, 1, 4]);
    err(String.raw`"\u12"`, "bad-escape", [1, 2, 1, 6]);
    err(String.raw`"\uZ"`, "bad-escape", [1, 2, 1, 4]);
    err(String.raw`"\u{}"`, "bad-escape", [1, 2, 1, 5]);
    err(String.raw`"\u{1234567}"`, "bad-escape", [1, 2, 1, 12]);
    err(String.raw`"\u{12"`, "bad-escape", [1, 2, 1, 7]);
    err(String.raw`"\u{110000}"`, "bad-escape", [1, 2, 1, 12]);
    err(String.raw`"\u{D800}"`, "bad-escape", [1, 2, 1, 10]);
    err(String.raw`"\uD800x"`, "bad-escape", [1, 2, 1, 8]);
    err(String.raw`"\uD800\u0041"`, "bad-escape", [1, 2, 1, 8]);
    err(String.raw`"\uDC00"`, "bad-escape", [1, 2, 1, 8]);
  });
  test("2.11: a bad escape is met before the unterminated string", () => {
    err('"a\\q\n', "bad-escape", [1, 3, 1, 5]);
  });
});

describe("2.8 templates", () => {
  test("text parts and escapes", () => {
    const t = tokens("`a\\`\\${b}`")[0];
    assert.equal(t.kind, "template");
    assert.deepEqual(t.parts, [{ kind: "text", value: "a`${b}" }]);
  });
  test("raw CR and CRLF read as LF; the escape \\r stays", () => {
    const t = tokens("`a\r\nb\rc\\r`")[0];
    assert.deepEqual(t.parts, [{ kind: "text", value: "a\nb\nc\r" }]);
  });
  test("line-continuation", () => {
    err("`a\\\nb`", "line-continuation", [1, 3, 1, 4]);
    err("`a\\\r\nb`", "line-continuation", [1, 3, 1, 4]);
  });
  test("interpolations match braces and nest", () => {
    ok("`${ {a: 1}.a }`");
    ok("`a ${ `b ${ `c ${1}` }` } d`");
    ok("`${ /* } */ 1 }`");
  });
  test("unterminated: from the innermost open template to the end", () => {
    err("`abc", "unterminated-string", [1, 1, 1, 5]);
    err("x + `a ${ `b ${ 1", "unterminated-string", [1, 11, 1, 18]);
    err("`a ${ 1 ", "unterminated-string", [1, 1, 1, 9]);
  });
  test("a template nested 100,000 deep is lexed without the host's stack", () => {
    const n = 100000;
    err("`${".repeat(n) + "1" + "}`".repeat(n), "nesting-exceeded");
  });
});

describe("2.9 and 2.10 characters", () => {
  test("each character that begins no token", () => {
    for (const c of ["#", "$", "@", "\\", "^", "~", "&", "\u00a0", "\u0001", "\u007f"]) {
      err(`a ${c} b`, "unexpected-character", [1, 3, 1, 4]);
    }
  });
  test("$ is a token only inside a template", () => {
    ok("`$a $ {`");
    err("$.a", "unexpected-character", [1, 1, 1, 2]);
  });
});

describe("2.11 order of lexical diagnostics", () => {
  test("a lexical diagnostic comes before a parse diagnostic", () => {
    err("1 2 $", "unexpected-character", [1, 5, 1, 6]);
    err('"""x""" $', "unexpected-character", [1, 9, 1, 10]);
  });
  test("the first lexical diagnostic in reading order", () => {
    lexErr("$ 010", "unexpected-character", [1, 1, 1, 2]);
    lexErr("010 $", "bad-number", [1, 1, 1, 4]);
  });
});
