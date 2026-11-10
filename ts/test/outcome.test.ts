// The outcome command (ruling AJ): its output form.

import { test } from "node:test";
import assert from "node:assert/strict";
import { spawnSync } from "node:child_process";
import { mkdtempSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const here = dirname(fileURLToPath(import.meta.url));
const command = join(here, "..", "scripts", "outcome.ts");

function outcome(file: string) {
  return spawnSync(process.execPath, [command, file], { encoding: "utf8" });
}

test("one line of JSON for each case, in order, fields in the order of AJ item 3", () => {
  const r = outcome(join(here, "fixtures", "outcome.json"));
  assert.equal(r.status, 0, r.stderr);
  assert.deepEqual(r.stdout.trimEnd().split("\n"), [
    '{"name":"value","phase":null,"value":{"a":null,"b":[1,2.5,"\\n"]},"steps":0,"units":9}',
    '{"name":"parse","phase":"parse","diagnostics":[{"code":"unexpected-token","span":[1,4,1,4]}]}',
    '{"name":"check","phase":"check","diagnostics":[{"code":"undefined-name","span":[1,1,1,2]},{"code":"undefined-name","span":[1,5,1,6]}]}',
    '{"name":"declare","phase":"declare","diagnostics":[{"code":"name-clash"}]}',
    '{"name":"run","phase":"run","diagnostics":[{"code":"divide-by-zero","span":[1,1,1,6],"class":"divide","message":"the divisor of / is zero"}],"steps":1,"units":0}',
    '{"name":"limit","phase":"run","diagnostics":[{"code":"allocation-exceeded","span":null,"class":"limit","message":"the run needed more than its limit of 1 allocation units"}],"steps":0,"units":0}',
  ]);
  for (const line of r.stdout.trimEnd().split("\n")) JSON.parse(line);
});

test("an opaque value is written as a case file writes it", () => {
  const dir = mkdtempSync(join(tmpdir(), "risor-outcome-"));
  const f = join(dir, "c.json");
  writeFileSync(f, JSON.stringify([{ name: "o", source: "[t]", env: "{t: T1}", opaque: ["T1"], vars: { t: { $opaque: { type: "T1", id: 3 } } } }]));
  const r = outcome(f);
  assert.equal(r.status, 0, r.stderr);
  assert.equal(r.stdout.trim(), '{"name":"o","phase":null,"value":[{"$opaque":{"type":"T1","id":3}}],"steps":0,"units":2}');
});

test("an invalid file: status 2 and a message on standard error", () => {
  const dir = mkdtempSync(join(tmpdir(), "risor-outcome-"));
  const f = join(dir, "c.json");
  writeFileSync(f, '[{"name": "a", "source": "1", "bogus": 1}]');
  const r = outcome(f);
  assert.equal(r.status, 2);
  assert.equal(r.stdout, "");
  assert.match(r.stderr, /invalid case file/);
});
