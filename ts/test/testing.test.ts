// Unit tests of the binding of package F9 (f09 7.3): what the suite's runner
// cases do not reach, the base environment, the host's variables and the
// writing of a file in place.

import { describe, test } from "node:test";
import assert from "node:assert/strict";
import { mkdtempSync, readFileSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { createEnv } from "../src/host/api.ts";
import { caseFile, caseFileText, defineCases, loadCaseFile } from "../src/testing.ts";

describe("defineCases (f09 7.3 items 1 and 2)", () => {
  test("one test for each case; a failure throws the lines of 6.1; skip goes to test.skip", () => {
    const f = caseFile({ cases: [
      { name: "ok", source: "1 + 1", expect: { value: 2 } },
      { name: "bad", source: "1", expect: { value: 2 } },
      { name: "later", source: "1", skip: "not yet", expect: { value: 1 } },
    ] }, { update: false });
    const ran: string[] = [];
    const errors: string[] = [];
    const skipped: string[] = [];
    const t = Object.assign((name: string, fn: () => void) => {
      ran.push(name);
      try {
        fn();
      } catch (e) {
        errors.push((e as Error).message);
      }
    }, { skip: (name: string) => void skipped.push(name) });
    defineCases(t, f);
    assert.deepEqual(ran, ["ok", "bad"]);
    assert.deepEqual(skipped, ["later"]);
    assert.deepEqual(errors, ["  FAIL bad\n    value: expected 2, found 1\n"]);
  });
  test("an invalid file throws", () => {
    assert.throws(() => caseFileText('[{"name": "a", "source": "1", "expect": {"vlaue": 1}}]', false, { update: false }), /invalid case file/);
    assert.throws(() => caseFileText("{cases: [}", true), /invalid case file/);
  });
});

describe("the base environment and the host's variables (03 3.10.9 item 3)", () => {
  const base = createEnv({ vars: "{n: int}", functions: { twice: { type: "(x: int) => int", call: (_h, x) => (x as number) * 2 } } });
  test("the host's function and variable are used where the file gives none", () => {
    const f = caseFile([{ name: "a", env: "{n: int}", source: "twice(n)", expect: { value: 8 } }], { update: false, base, vars: (k) => (k === "n" ? 4 : undefined) });
    assert.equal(f.cases[0].run().status, "pass");
  });
  test("a stub with no type keeps the base's signature", () => {
    const f = caseFile([{ name: "a", source: "twice(1)", functions: { twice: { calls: [{ args: [1], result: 5 }] } }, expect: { value: 5, type: "int" } }], { update: false, base });
    assert.equal(f.cases[0].run().status, "pass");
  });
});

describe("update mode (f09 4, 7.3 item 4)", () => {
  test("updatedText gives the new text, and null when nothing is written", () => {
    const f = caseFileText('[{"name": "a", "source": "1 + 2"}]', false, { update: true });
    assert.equal(f.cases[0].run().status, "updated");
    assert.equal(f.updatedText(), '[{"name": "a", "source": "1 + 2", "expect": {"allocated":0,"steps":1,"type":"int","value":3}}]');
    const g = caseFileText('[{"name": "a", "source": "1", "expect": {"value": 1}}]', false, { update: true });
    g.cases[0].run();
    assert.equal(g.updatedText(), null);
  });
  test("loadCaseFile writes the file in place once every case has run; a program file is never written", () => {
    const dir = mkdtempSync(join(tmpdir(), "risor-testing-"));
    const path = join(dir, "a.test.json");
    writeFileSync(path, '{"cases": [{"name": "a", "source": "2 * 3", "expect": {"value": 0}}, {"name": "b", "source": "1", "skip": "x"}]}');
    const f = loadCaseFile(path, { update: true });
    f.cases[0].run();
    assert.equal(readFileSync(path, "utf8"), '{"cases": [{"name": "a", "source": "2 * 3", "expect": {"value":6}}, {"name": "b", "source": "1", "skip": "x"}]}');
    const p = join(dir, "b.test.risor");
    writeFileSync(p, '{cases: [{name: "a", source: "1", expect: {value: 2}}]}');
    const g = loadCaseFile(p, { update: true });
    assert.equal(g.cases[0].run().status, "fail");
    assert.equal(g.updatedText(), null);
  });
  test("a filter selects cases by a part of the name", () => {
    const f = caseFileText('[{"name": "gold one", "source": "1", "expect": {"value": 1}}, {"name": "two", "source": "1", "expect": {"value": 1}}]', false, { filter: "gold" });
    assert.deepEqual(f.cases.map((c) => c.name), ["gold one"]);
  });
});
