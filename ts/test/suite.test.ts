// Run the suite's case files. Set RISOR_CASES to a list of case files or
// directories, separated by the path delimiter, to run others instead.

import { test } from "node:test";
import assert from "node:assert/strict";
import { delimiter } from "node:path";
import { countVerdicts } from "../src/suite/runner.ts";
import { defaultTargets, runTargets } from "./suite-report.ts";
import { DISPUTED } from "./disputed.ts";

const targets = process.env.RISOR_CASES ? process.env.RISOR_CASES.split(delimiter) : defaultTargets();

for (const [target, verdicts] of runTargets(targets)) {
  test(`suite: ${target}`, (t) => {
    const n = countVerdicts(verdicts);
    t.diagnostic(`pass ${n.pass}, fail ${n.fail}, skip ${n.skip}, invalid ${n.invalid}`);
    const bad = verdicts.filter((v) => v.verdict === "fail" || v.verdict === "invalid");
    for (const v of bad) if (`${v.file} :: ${v.name}` in DISPUTED) t.diagnostic(`disputed: ${v.file} :: ${v.name}`);
    assert.deepEqual(
      bad.filter((v) => !(`${v.file} :: ${v.name}` in DISPUTED)).map((v) => `${v.verdict} ${v.file} :: ${v.name} ${v.reason ?? JSON.stringify(v.differences)}`),
      [],
    );
  });
}
