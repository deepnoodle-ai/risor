// The outcome command (suite/README.md): its output form.

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

test("one line of JSON for each case, in order, fields in the order of suite/README.md and f08 8", () => {
  const r = outcome(join(here, "fixtures", "outcome.json"));
  assert.equal(r.status, 0, r.stderr);
  assert.deepEqual(r.stdout.trimEnd().split("\n"), [
    "{\"name\":\"value\",\"type\":\"{a: any, b: (float | \\\"\\\\n\\\")[]}\",\"sites\":[],\"phase\":null,\"value\":{\"a\":null,\"b\":[1,2.5,\"\\n\"]},\"steps\":0,\"units\":9,\"analysis\":{\"names\":[\"x\"],\"refs\":[{\"name\":\"x\",\"path\":[],\"span\":[1,24,1,25]}],\"calls\":[],\"literal\":false,\"segments\":null}}",
    "{\"name\":\"parse\",\"phase\":\"parse\",\"diagnostics\":[{\"code\":\"unexpected-token\",\"span\":[1,4,1,4],\"message\":\"unexpected end of input\"}]}",
    "{\"name\":\"check\",\"phase\":\"check\",\"diagnostics\":[{\"code\":\"undefined-name\",\"span\":[1,1,1,2],\"message\":\"y is not defined\"},{\"code\":\"undefined-name\",\"span\":[1,5,1,6],\"message\":\"z is not defined\"}],\"analysis\":{\"names\":[\"y\",\"z\"],\"refs\":[{\"name\":\"y\",\"path\":[],\"span\":[1,1,1,2]},{\"name\":\"z\",\"path\":[],\"span\":[1,5,1,6]}],\"calls\":[],\"literal\":false,\"segments\":null}}",
    "{\"name\":\"declare\",\"phase\":\"declare\",\"diagnostics\":[{\"code\":\"name-clash\"}],\"analysis\":{\"names\":[],\"refs\":[],\"calls\":[],\"literal\":{\"value\":1},\"segments\":null}}",
    "{\"name\":\"run\",\"type\":\"any\",\"sites\":[{\"class\":\"divide\",\"span\":[1,1,1,6]},{\"class\":\"dynamic\",\"span\":[1,5,1,6]}],\"phase\":\"run\",\"diagnostics\":[{\"code\":\"divide-by-zero\",\"span\":[1,1,1,6],\"class\":\"divide\",\"message\":\"the divisor of / is zero\",\"trace\":[]}],\"steps\":1,\"units\":0,\"analysis\":{\"names\":[\"x\"],\"refs\":[{\"name\":\"x\",\"path\":[],\"span\":[1,5,1,6]}],\"calls\":[],\"literal\":false,\"segments\":null}}",
    "{\"name\":\"limit\",\"type\":\"int[]\",\"sites\":[],\"phase\":\"run\",\"diagnostics\":[{\"code\":\"allocation-exceeded\",\"span\":null,\"class\":\"limit\",\"message\":\"the run needed more than its limit of 1 allocation units\"}],\"analysis\":{\"names\":[],\"refs\":[],\"calls\":[],\"literal\":{\"value\":[1,2]},\"segments\":null}}", // suite/README.md item 2: no usage
  ]);
  for (const line of r.stdout.trimEnd().split("\n")) JSON.parse(line);
});

test("an opaque value is written as a case file writes it", () => {
  const dir = mkdtempSync(join(tmpdir(), "risor-outcome-"));
  const f = join(dir, "c.json");
  writeFileSync(f, JSON.stringify([{ name: "o", source: "[t]", env: "{t: T1}", opaque: ["T1"], vars: { t: { $opaque: { type: "T1", id: 3 } } } }]));
  const r = outcome(f);
  assert.equal(r.status, 0, r.stderr);
  assert.equal(r.stdout.trim(), '{"name":"o","type":"T1[]","sites":[],"phase":null,"value":[{"$opaque":{"type":"T1","id":3}}],"steps":0,"units":2,"analysis":{"names":["t"],"refs":[{"name":"t","path":[],"span":[1,2,1,3]}],"calls":[],"literal":false,"segments":null}}');
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

test('suite/README.md: a case with "check": false has no type and no sites', () => {
  const dir = mkdtempSync(join(tmpdir(), "risor-outcome-"));
  const f = join(dir, "c.json");
  writeFileSync(f, JSON.stringify([{ name: "u", source: "1 < 2 < 3", check: false }]));
  const r = outcome(f);
  assert.equal(r.status, 0, r.stderr);
  assert.equal(r.stdout.trim(), '{"name":"u","phase":"run","diagnostics":[{"code":"wrong-kind","span":[1,1,1,10],"class":"dynamic","message":"cannot apply < to bool and number","trace":[]}],"steps":1,"units":0,"analysis":{"names":[],"refs":[],"calls":[],"literal":false,"segments":null}}');
});
