// The case reader: the field `check` of ruling AP1 and its invalid-file rules.

import { test } from "node:test";
import assert from "node:assert/strict";
import { validateCaseFile } from "../src/suite/casefile.ts";
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
