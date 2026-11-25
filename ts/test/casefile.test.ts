// The case reader: the field `check` (03 3.10.3) and its invalid-file rules.

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

test('"limits.source" is a whole number from 0 to 1,048,576, in a program case and a tool case (03 3.10.3; f12 7.1, 7.2)', () => {
  const limits = (l: unknown) => read([{ name: "a", source: "1", limits: l, expect: {} }]).ok;
  assert.ok(limits({ source: 0 }));
  assert.ok(limits({ source: 1048576, steps: 3 }));
  assert.equal(limits({ source: -1 }), false);
  assert.equal(limits({ source: 1048577 }), false);
  assert.equal(limits({ source: 1.5 }), false);
  assert.equal(limits({ size: 3 }), false);
  assert.ok(read([{ name: "a", format: "1", limits: { source: 1 }, expect: { formatted: "1\n" } }]).ok);
  assert.ok(read({ env: "{u: int}", cases: [{ name: "a", complete: "u‸", limits: { source: 1 }, expect: { completion: { span: [1, 1, 1, 2], items: [] } } }] }).ok);
  assert.ok(read([{ name: "a", hover: "u‸", limits: { source: 1 }, expect: { hover: null } }]).ok);
  assert.equal(read([{ name: "a", format: "1", limits: { source: -1 }, expect: { formatted: "1\n" } }]).ok, false);
  assert.equal(read([{ name: "a", format: "1", limits: { nope: 1 }, expect: { formatted: "1\n" } }]).ok, false);
});
