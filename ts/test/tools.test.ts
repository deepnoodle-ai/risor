// Unit tests of package F12 (f12 2.2, 7.6 item 3): the formatter's laws over
// every program of the suite, and the binding's calls.

import { describe, test } from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { join } from "node:path";
import { caseFiles } from "../src/suite/runner.ts";
import { formatProgram, treeEqual } from "../src/tools/format.ts";
import { complete, hover } from "../src/tools/assist.ts";
import { parse } from "../src/syntax/parser.ts";
import { createEnv } from "../src/host/api.ts";

const SUITE = join(import.meta.dirname, "..", "..", "suite");

describe("the laws of the formatter over the suite's programs (f12 7.6 item 3)", () => {
  test("idempotent, and the same tree", () => {
    const sources: string[] = [];
    const collect = (x: unknown): void => {
      if (Array.isArray(x)) x.forEach(collect);
      else if (x !== null && typeof x === "object") {
        const o = x as Record<string, unknown>;
        if (typeof o.source === "string" && o.template === undefined) sources.push(o.source);
        for (const v of Object.values(o)) if (typeof v === "object") collect(v);
      }
    };
    for (const d of ["core-run", "core-check", "strings", "conversion", "unions", "groups", "queries", "errors", "config", "facts"]) {
      for (const f of caseFiles(join(SUITE, d))) collect(JSON.parse(readFileSync(f, "utf8")));
    }
    let n = 0;
    for (const s of sources) {
      const p = parse(s);
      if (!p.ok) continue;
      n++;
      const r = formatProgram(s);
      assert.ok(r.ok, s);
      const q = parse(r.text);
      assert.ok(q.ok && treeEqual(p.program, q.program), `the same tree: ${s}`);
      const again = formatProgram(r.text);
      assert.ok(again.ok && again.text === r.text, `idempotent: ${s}`);
    }
    assert.ok(n > 1000);
  });
});

describe("the binding's calls (f12 1.2)", () => {
  const env = createEnv({ vars: "{user: {name: string}}" });
  test("completion and hover", () => {
    const c = complete(env, "user.n", { line: 1, column: 7 });
    assert.deepEqual(c.items.map((i) => i.label), ["name"]);
    const h = hover(env, "user.name", { line: 1, column: 7 });
    assert.equal(h?.kind, "field");
    assert.equal(h?.type, "string");
  });
  test("a position in no line gives nothing", () => {
    assert.deepEqual(complete(env, "user", { line: 3, column: 1 }).items, []);
    assert.equal(hover(env, "user", { line: 1, column: 9 }), null);
  });
});

describe("the clarifications of f12 3.2 and 3.12", () => {
  const env = createEnv({});
  test("a parameter's type takes the first check of a const lambda", () => {
    // Hover on the parameter's use in the body, and on its binding: both from the first use, f(1).
    assert.equal(hover(env, 'const f = x => x;\n[f(1), f("a")]', { line: 1, column: 16 })?.type, "int");
    assert.equal(hover(env, 'const f = x => x;\n[f(1), f("a")]', { line: 1, column: 11 })?.type, "int");
  });
  test("hover's cut takes a ( only when it is the very next character", () => {
    const env2 = createEnv({ vars: "{user: {tags: string[]}}" });
    assert.equal(hover(env2, "user.tags.first(", { line: 1, column: 12 })?.type, "string | null");
    assert.equal(hover(env2, "user.tags.first (", { line: 1, column: 12 })?.kind, "field");
  });
});
