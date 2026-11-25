// Unit tests of package F12 (f12 2.2, 7.6 item 3): the formatter's laws over
// every program of the suite, and the binding's calls.

import { describe, test } from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { join } from "node:path";
import { caseFiles } from "../src/suite/runner.ts";
import { formatProgram, treeEqual } from "../src/tools/format.ts";
import { deepestLevel, parse } from "../src/syntax/parser.ts";
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
      // f12 2.2 item 4: not for a source deeper than 96 levels.
      assert.ok(again.ok && (again.text === r.text || deepestLevel(p.program) > 96), `idempotent: ${s}`);
    }
    assert.ok(n > 1000);
  });
});

describe("the binding's calls (f12 1.2)", () => {
  const env = createEnv({ varRecord: "{user: {name: string}}" });
  test("completion and hover", () => {
    const c = env.complete("user.n", { line: 1, column: 7 });
    assert.deepEqual(c.items.map((i) => i.label), ["name"]);
    const h = env.hover("user.name", { line: 1, column: 7 });
    assert.equal(h?.kind, "field");
    assert.equal(h?.type, "string");
  });
  test("a position in no line gives nothing", () => {
    assert.deepEqual(env.complete("user", { line: 3, column: 1 }).items, []);
    assert.equal(env.hover("user", { line: 1, column: 9 }), null);
  });
});

describe("the clarifications of f12 3.2 and 3.12", () => {
  const env = createEnv({});
  test("a parameter's type takes the first check of a const lambda", () => {
    // Hover on the parameter's use in the body, and on its binding: both from the first use, f(1).
    assert.equal(env.hover('const f = x => x;\n[f(1), f("a")]', { line: 1, column: 16 })?.type, "int");
    assert.equal(env.hover('const f = x => x;\n[f(1), f("a")]', { line: 1, column: 11 })?.type, "int");
  });
  test("hover's cut takes a ( only when it is the very next character", () => {
    const env2 = createEnv({ varRecord: "{user: {tags: string[]}}" });
    assert.equal(env2.hover("user.tags.first(", { line: 1, column: 12 })?.type, "string | null");
    assert.equal(env2.hover("user.tags.first (", { line: 1, column: 12 })?.kind, "field");
  });
});

describe("formatter comment and record-type edge cases (f12 2.8, 2.12, 2.13)", () => {
  const cases: [string, string][] = [
    ["a || (/* why */ b && c)", "a || (/* why */ b && c)\n"],
    ["(/* why */ a ?? b) as int", "(/* why */ a ?? b) as int\n"],
    ["(/* why */ a ? b : c) ? d : e", "(/* why */ a ? b : c) ? d : e\n"],
    ["(/* why */ a?.b).c", "(/* why */ a?.b).c\n"],
    ["(/* why */ a[*].b).c", "(/* why */ a[*].b).c\n"],
    ["if (( // why\n  c)) { 1 } else { 2 }", "if (// why\n  c\n) {\n  1\n} else {\n  2\n}\n"],
    ["type Empty = {/* none */};\n1", "type Empty = {/* none */};\n1\n"],
    ["type Rec = {\n  a: int, // a\n\n  b: string,\n};\n1", "type Rec = {\n  a: int, // a\n\n  b: string,\n};\n1\n"],
    ["type Rec = {\n  a: int,\n\n  // more\n};\n1", "type Rec = {\n  a: int,\n\n  // more\n};\n1\n"],
    [
      "type MapRec = map<{\n  /** key */\n  a: int,\n}>;\n1",
      "type MapRec = map<{\n  /** key */\n  a: int,\n}>;\n1\n",
    ],
    [
      "const g = (f: (a: {\n  /** id */\n  id: int,\n}) => int) => 1;\n0",
      "const g = (\n  f: (a: {\n    /** id */\n    id: int,\n  }) => int,\n) => 1;\n0\n",
    ],
  ];
  for (const [src, want] of cases) {
    test(src.replace(/\n/g, "\\n"), () => {
      const r = formatProgram(src);
      assert.ok(r.ok);
      assert.equal(r.text, want);
      const again = formatProgram(r.text);
      assert.ok(again.ok);
      assert.equal(again.text, r.text);
      const p1 = parse(src);
      const p2 = parse(r.text);
      assert.ok(p1.ok && p2.ok && treeEqual(p1.program, p2.program));
    });
  }
});
