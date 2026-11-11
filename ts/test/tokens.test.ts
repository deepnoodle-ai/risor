// Unit tests of `tokens`, the colours of a text (src/tools/tokens.ts).

import { describe, test } from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { join } from "node:path";
import { caseFiles } from "../src/suite/runner.ts";
import { tokens } from "../src/tools/tokens.ts";
import type { ColourToken } from "../src/tools/tokens.ts";

const SUITE = join(import.meta.dirname, "..", "..", "suite");

/** Each range as `class:text`, for short assertions. */
function show(text: string): string[] {
  const cps = Array.from(text);
  return tokens(text).map((t) => `${t.class}:${cps.slice(t.start, t.end).join("")}`);
}

/** The ranges are in order, do not overlap, and cover every code point that is not white space. */
function assertCovers(text: string, ts: ColourToken[]): void {
  const cps = Array.from(text);
  let at = 0;
  for (const t of ts) {
    assert.ok(t.start >= at, `range ${t.start} overlaps the one before it in ${JSON.stringify(text)}`);
    assert.ok(t.end > t.start, `empty range at ${t.start} in ${JSON.stringify(text)}`);
    for (let i = at; i < t.start; i++) {
      assert.match(cps[i], /^[ \t\r\n]$/, `code point ${i} is in no range in ${JSON.stringify(text)}`);
    }
    at = t.end;
  }
  assert.ok(at <= cps.length);
  for (let i = at; i < cps.length; i++) assert.match(cps[i], /^[ \t\r\n]$/);
}

describe("tokens", () => {
  test("words, numbers, strings and operators", () => {
    assert.deepEqual(show(`const x = 1.5; x > 2 && name != "a"`), [
      "keyword:const", "name:x", "operator:=", "number:1.5", "punctuation:;",
      "name:x", "operator:>", "number:2", "operator:&&", "name:name", "operator:!=", 'string:"a"',
    ]);
  });

  test("constants, calls and properties", () => {
    assert.deepEqual(show(`len(order.items) == null || true`), [
      "function:len", "punctuation:(", "name:order", "punctuation:.", "property:items",
      "punctuation:)", "operator:==", "constant:null", "operator:||", "constant:true",
    ]);
    assert.deepEqual(show(`a?.b`), ["name:a", "operator:?.", "property:b"]);
  });

  test("a template and its interpolations", () => {
    assert.deepEqual(show("`n ${a + 1} of ${b}!`"), [
      "template:`n ", "interpolation:${", "name:a", "operator:+", "number:1", "interpolation:}",
      "template: of ", "interpolation:${", "name:b", "interpolation:}", "template:!`",
    ]);
  });

  test("nested templates", () => {
    assert.deepEqual(show("`a${`b${c}`}`"), [
      "template:`a", "interpolation:${", "template:`b", "interpolation:${", "name:c",
      "interpolation:}", "template:`", "interpolation:}", "template:`",
    ]);
  });

  test("comments", () => {
    assert.deepEqual(show("// one\nx /* two */ + 1 // three"), [
      "comment:// one", "name:x", "comment:/* two */", "operator:+", "number:1", "comment:// three",
    ]);
  });

  test("a text that does not lex is coloured to the error, and again from the next line", () => {
    assert.deepEqual(show(`x + "open\ny`), ["name:x", "operator:+", 'invalid:"open', "name:y"]);
  });

  test("a rejected token", () => {
    assert.deepEqual(show(`a === b`), ["name:a", "invalid:===", "name:b"]);
  });

  test("offsets count code points", () => {
    const ts = tokens(`"😀" + x`);
    assert.deepEqual(ts.map((t) => [t.start, t.end]), [[0, 3], [4, 5], [6, 7]]);
  });

  test("the empty text, and white space only", () => {
    assert.deepEqual(tokens(""), []);
    assert.deepEqual(tokens(" \n\t"), []);
  });

  test("an unclosed template in the middle of typing", () => {
    const text = "const a = `x ${b\nc";
    assertCovers(text, tokens(text));
  });

  test("every program of the suite is covered exactly, with no invalid range when it lexes", () => {
    const sources: string[] = [];
    const collect = (x: unknown): void => {
      if (Array.isArray(x)) x.forEach(collect);
      else if (x !== null && typeof x === "object") {
        const o = x as Record<string, unknown>;
        if (typeof o.source === "string") sources.push(o.source);
        for (const v of Object.values(o)) if (typeof v === "object") collect(v);
      }
    };
    for (const f of caseFiles(SUITE)) collect(JSON.parse(readFileSync(f, "utf8")));
    assert.ok(sources.length > 1000, `only ${sources.length} sources`);
    for (const s of sources) assertCovers(s, tokens(s));
  });
});
