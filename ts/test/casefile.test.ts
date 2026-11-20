// The case reader: the field `check` (03 3.10.3) and its invalid-file rules.

import { test } from "node:test";
import assert from "node:assert/strict";
import { mkdtempSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { validateCaseFile } from "../src/suite/casefile.ts";
import { runFile } from "../src/suite/runner.ts";
import type { Json } from "../src/suite/json.ts";

const read = (cases: unknown) => validateCaseFile(cases as Json, ".");

test('"check" is a boolean, in a case or in the header', () => {
  assert.ok(read([{ name: "a", source: "1", check: false, expect: {} }]).ok);
  assert.ok(read({ check: false, cases: [{ name: "a", source: "1", expect: {} }] }).ok);
  assert.equal(read([{ name: "a", source: "1", check: "no", expect: {} }]).ok, false);
});

test('"check": false with type or sites makes the file invalid', () => {
  assert.equal(read([{ name: "a", source: "1", check: false, expect: { type: "int" } }]).ok, false);
  assert.equal(read([{ name: "a", source: "1", check: false, expect: { sites: [] } }]).ok, false);
  assert.ok(read([{ name: "a", source: "1", check: true, expect: { type: "int" } }]).ok);
});

test('"check": false with a check code that is not static makes the file invalid', () => {
  const failure = (code: string) => ({ phase: "check", diagnostics: [{ code }] });
  assert.equal(read([{ name: "a", source: "1", check: false, expect: failure("kind") }]).ok, false);
  assert.ok(read([{ name: "a", source: "x", check: false, expect: failure("undefined-name") }]).ok);
});

test("the cases of a package's catalog file are read with the static codes they run with", () => {
  // f17 13.8 item 2: in messages-recursion.json, arg-mismatch is a static code.
  const path = join(mkdtempSync(join(tmpdir(), "risor-casefile-")), "messages-recursion.json");
  const source = "const apply = (f: (x: int) => int, x: int): int => f(x); apply(5, 1)";
  const expect = { phase: "check", diagnostics: [{ code: "arg-mismatch", span: [1, 64, 1, 65] }] };
  writeFileSync(path, JSON.stringify([{ code: "arg-mismatch", cases: [{ name: "static", source, check: false, expect }] }]));
  assert.deepEqual(runFile(path, {}).map((v) => v.verdict), ["pass"]);
});
