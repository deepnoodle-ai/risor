// The size of a source text (01 2.1 items 4 to 6; 03 3.6.1 items 5 and 6):
// the default limit exactly, the count in code points, and the host's option
// on each call that takes source text. The suite states the rule at small
// limits; these tests state it at the default, which a case file would need a
// text of a mebibyte to reach.

import { describe, test } from "node:test";
import assert from "node:assert/strict";
import { createEnv, format, formatTemplate, LoadError, loadValue, parse, parseTemplate, RisorError } from "../src/index.ts";

const LIMIT = 1_048_576;

/** The diagnostics of the RisorError a call throws. */
function thrown(f: () => unknown): RisorError {
  try {
    f();
  } catch (e) {
    if (e instanceof RisorError) return e;
    throw e;
  }
  return assert.fail("no RisorError");
}

/** A call refused for its size: one diagnostic, of phase parse, with no span. */
function tooLarge(f: () => unknown, limit = LIMIT): void {
  const e = thrown(f);
  assert.equal(e.phase, "parse");
  assert.equal(e.diagnostics.length, 1);
  const d = e.diagnostics[0];
  assert.equal(d.code, "source-too-large");
  assert.equal(d.span, undefined);
  assert.ok(!("span" in d));
  assert.equal(d.message, `the source text is longer than the limit of ${limit} code points`);
}

function badLimit(f: () => unknown): void {
  const e = thrown(f);
  assert.equal(e.phase, "declare");
  assert.deepEqual(e.diagnostics.map((d) => d.code), ["bad-limit"]);
}

describe("the default source limit (01 2.1 item 4)", () => {
  const env = createEnv({ limits: { steps: null, allocation: null } });

  test("a text of exactly 1,048,576 code points parses; one more is source-too-large", () => {
    const at = "1" + " ".repeat(LIMIT - 1);
    assert.equal(at.length, LIMIT);
    assert.equal(env.compile(parse(at)).run(), 1);
    tooLarge(() => parse(at + " "));
  });

  test("a string literal that fills the limit", () => {
    const at = '"' + "x".repeat(LIMIT - 2) + '"';
    assert.equal((env.compile(parse(at)).run() as string).length, LIMIT - 2);
    tooLarge(() => parse('"' + "x".repeat(LIMIT - 1) + '"'));
  });

  test("code points above U+FFFF count once: 1,048,576 of them in UTF-16 is about 2 MiB", () => {
    const at = '"' + "\u{1F600}".repeat(LIMIT - 2) + '"';
    assert.equal(at.length, 2 * LIMIT - 2);
    assert.equal(env.compile(parse(at)).run(), "\u{1F600}".repeat(LIMIT - 2));
    // 2 * LIMIT units, so the count is read; then past 2 * LIMIT units, the length alone decides.
    tooLarge(() => parse('"' + "\u{1F600}".repeat(LIMIT - 1) + '"'));
    tooLarge(() => parse('"' + "\u{1F600}".repeat(LIMIT) + '"'));
  });

  test("the size is found before the text is lexed", () => {
    tooLarge(() => parse("@".repeat(LIMIT + 1)));
    tooLarge(() => parse('"' + "x".repeat(LIMIT)));
  });

  test("the formatter, completion and hover over the default limit", () => {
    const over = "1" + " ".repeat(LIMIT);
    tooLarge(() => format(over));
    tooLarge(() => formatTemplate("a" + " ".repeat(LIMIT)));
    tooLarge(() => parseTemplate("a" + " ".repeat(LIMIT)));
    const user = createEnv({ varRecord: "{user: string}" });
    const text = "use" + " ".repeat(LIMIT - 2);
    assert.deepEqual(user.complete(text, { line: 1, column: 4 }), { span: { start: { line: 1, column: 4 }, end: { line: 1, column: 4 } }, items: [] });
    assert.equal(user.hover(text, { line: 1, column: 2 }), null);
    // At the limit, both read the text.
    const at = text.slice(0, LIMIT);
    assert.deepEqual(user.complete(at, { line: 1, column: 4 }).items.map((c) => c.label), ["user"]);
    assert.equal(user.hover("user" + " ".repeat(LIMIT - 4), { line: 1, column: 2 })?.kind, "variable");
  });

  test("a load over the default limit fails at step 1", () => {
    const e = thrown(() => loadValue(env, "1" + " ".repeat(LIMIT), { expected: "int" }));
    assert.ok(e instanceof LoadError);
    assert.equal(e.step, 1);
    assert.equal(e.diagnostics[0].code, "source-too-large");
  });
});

describe("a lower source limit, an option of each call (03 3.6.1 item 5)", () => {
  const env = createEnv({ varRecord: "{user: {name: string}}" });

  test("parse and template parse", () => {
    assert.ok(parse("1 + 2", { sourceLimit: 5 }));
    tooLarge(() => parse("1 + 2", { sourceLimit: 4 }), 4);
    tooLarge(() => parse("1", { sourceLimit: 0 }), 0);
    assert.ok(parseTemplate("a${{ 1 }}", { sourceLimit: 9 }));
    tooLarge(() => parseTemplate("ab${{ 1 }}", { sourceLimit: 9 }), 9);
  });

  test("counted in code points: a pair once, a lone surrogate once, a U+FEFF at the start", () => {
    assert.ok(parse('"\u{1F600}"', { sourceLimit: 3 }));
    tooLarge(() => parse('"\u{1F600}"', { sourceLimit: 2 }), 2);
    assert.ok(parse('"\ud800"', { sourceLimit: 3 }));
    assert.ok(parse('"\ud800\ud800"', { sourceLimit: 4 }));
    tooLarge(() => parse('"\ud800\ud800"', { sourceLimit: 3 }), 3);
    tooLarge(() => parse("﻿1", { sourceLimit: 1 }), 1);
    assert.ok(parse("1\r\n", { sourceLimit: 3 }));
    tooLarge(() => parse("1\r\n", { sourceLimit: 2 }), 2);
  });

  test("the formatter", () => {
    assert.equal(format("1+2", { sourceLimit: 3 }), "1 + 2\n");
    tooLarge(() => format("1 + 2", { sourceLimit: 4 }), 4);
    assert.equal(formatTemplate("a ${{1}}", { sourceLimit: 8 }), "a ${{ 1 }}");
    tooLarge(() => formatTemplate("a ${{ 1 }}", { sourceLimit: 9 }), 9);
  });

  test("completion and hover", () => {
    assert.deepEqual(env.complete("use", { line: 1, column: 4 }, undefined, { sourceLimit: 3 }).items.map((c) => c.label), ["user"]);
    assert.deepEqual(env.complete("use ", { line: 1, column: 4 }, undefined, { sourceLimit: 3 }), { span: { start: { line: 1, column: 4 }, end: { line: 1, column: 4 } }, items: [] });
    assert.equal(env.hover("user", { line: 1, column: 2 }, undefined, { sourceLimit: 4 })?.kind, "variable");
    assert.equal(env.hover("user.name", { line: 1, column: 2 }, undefined, { sourceLimit: 4 }), null);
  });

  test("a load", () => {
    assert.equal(loadValue(env, "12", { expected: "int", sourceLimit: 2 }), 12);
    const e = thrown(() => loadValue(env, "123", { expected: "int", sourceLimit: 2 }));
    assert.ok(e instanceof LoadError && e.step === 1);
    assert.equal(e.diagnostics[0].code, "source-too-large");
  });

  test("a source limit out of range is bad-limit, of phase declare", () => {
    for (const sourceLimit of [-1, 1.5, LIMIT + 1, Number.NaN, Infinity]) {
      badLimit(() => parse("1", { sourceLimit }));
      badLimit(() => parseTemplate("1", { sourceLimit }));
      badLimit(() => format("1", { sourceLimit }));
      badLimit(() => formatTemplate("1", { sourceLimit }));
      const e = thrown(() => loadValue(env, "1", { expected: "int", sourceLimit }));
      assert.ok(e instanceof LoadError && e.step === 0);
      assert.deepEqual(e.diagnostics.map((d) => d.code), ["bad-limit"]);
    }
    // The largest and the smallest are in range.
    assert.ok(parse("1", { sourceLimit: LIMIT }));
    tooLarge(() => parse("1", { sourceLimit: 0 }), 0);
  });

  test("completion and hover never fail: a limit out of range gives their empty result", () => {
    for (const sourceLimit of [-1, 1.5, LIMIT + 1, Number.NaN, null as unknown as number]) {
      assert.deepEqual(env.complete("use", { line: 1, column: 4 }, undefined, { sourceLimit }), { span: { start: { line: 1, column: 4 }, end: { line: 1, column: 4 } }, items: [] });
      assert.equal(env.hover("user", { line: 1, column: 2 }, undefined, { sourceLimit }), null);
    }
  });
});
