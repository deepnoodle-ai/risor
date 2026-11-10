// Package F3, string forms: lexer rules the suite's cases cannot reach well
// (f03 4, 5.3, 7.2, 7.3), and the zero cost of a literal with no interpolation.

import { describe, test } from "node:test";
import assert from "node:assert/strict";
import { err, ok } from "./helpers.ts";
import { createEnv } from "../src/host/api.ts";

const value = (src: string) => createEnv({}).compile(src).run();

describe("f03: text blocks", () => {
  test("a block of 200,000 lines is read in linear time (7.3)", () => {
    const lines = Array.from({ length: 200_000 }, (_, i) => `    line ${i}`).join("\n");
    const t0 = performance.now();
    const v = value(`"""\n${lines}\n    """`) as string;
    assert.ok(performance.now() - t0 < 2000);
    assert.equal(v.split("\n").length, 200_000);
    assert.equal(v.slice(0, 7), "line 0\n");
  });
  test("a line end written U+000D alone ends a block line, but positions stay on line 1 (5.3)", () => {
    assert.equal(value('"""\r  a\r  b\r  """'), "a\nb");
    err('"""\r  a\r b\r  """', "text-block-indent", [1, 10, 1, 11]);
  });
  test("U+2028 and U+0085 are text, not line ends (4.2 row 5)", () => {
    assert.equal(value('"""\n  a b\u0085c\n  """'), "a b\u0085c");
  });
  test("an interpolation where the indentation is expected: the span is its `$`", () => {
    err('"""\n  a\n ${1}\n  """', "text-block-indent", [3, 2, 3, 3]);
  });
  test("an outer block's indent error comes before an inner block's (6.2 item 3)", () => {
    err('"""\n  ${"""\n  x\n y\n  """}\n x\n  """', "text-block-indent", [6, 2, 6, 3]);
  });
  test("a raw block: a backslash before a line end is text (4.7 item 2)", () => {
    assert.equal(value('r"""\n  a\\\n  b\n  """'), "a\\\nb");
  });
  test("each ${ of a block opens a level (7.2)", () => {
    ok("(".repeat(99) + '"""\n  ${1}\n  """' + ")".repeat(99));
    err("(".repeat(100) + '"""\n  ${1}\n  """' + ")".repeat(100), "nesting-exceeded", [2, 3, 2, 5]);
    ok("(".repeat(100) + 'r"""\n  ${1}\n  """' + ")".repeat(100));
  });
  test("a block nested in interpolations 100,000 deep needs no host stack", () => {
    const n = 100_000;
    err('"""\n${'.repeat(n) + "1" + '}\n"""'.repeat(n), "nesting-exceeded");
  });
});

describe("f03: raw strings", () => {
  test("the word r is still a name", () => {
    assert.equal(value("const r = 2; r"), 2);
    assert.equal(value('const r = "x"; r+"a"'), "xa");
  });
  test("a raw string and a block with no interpolation cost nothing (7.1)", () => {
    const env = createEnv({});
    assert.deepEqual(env.compile('[r"a\\b", """\n  abc\n  """]').measure().usage, { steps: 0, allocated: 3 });
    assert.deepEqual(env.compile('r"' + "x".repeat(10_000) + '"').measure().usage, { steps: 0, allocated: 0 });
  });
});
