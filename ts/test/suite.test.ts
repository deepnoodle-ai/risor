// Run the suite's case files: with the type checker at step 4, and again
// with the static rules alone (03 3.10.8 item 5), where every case that does
// not need a checker still passes. Set RISOR_CASES to a list of case files
// or directories, separated by the path delimiter, to run others instead.

import { test } from "node:test";
import assert from "node:assert/strict";
import { delimiter } from "node:path";
import { countVerdicts } from "../src/suite/runner.ts";
import { defaultTargets, runTargets } from "./suite-report.ts";
import { DISPUTED } from "./disputed.ts";

const targets = process.env.RISOR_CASES ? process.env.RISOR_CASES.split(delimiter) : defaultTargets();

for (const checker of [true, false]) {
  const which = checker ? "checked" : "static rules alone";
  for (const [target, verdicts] of runTargets(checker ? targets : targets.filter((t) => !t.endsWith("core-check") && !t.endsWith("host-types")), checker)) {
    test(`suite (${which}): ${target}`, (t) => {
      const n = countVerdicts(verdicts);
      t.diagnostic(`pass ${n.pass}, fail ${n.fail}, skip ${n.skip}, invalid ${n.invalid}`);
      const bad = verdicts.filter((v) => v.verdict === "fail" || v.verdict === "invalid");
      const disputed = (v: { file: string; name: string }) => `${v.file} :: ${v.name}` in DISPUTED;
      for (const v of bad) if (disputed(v)) t.diagnostic(`disputed: ${v.file} :: ${v.name}`);
      assert.deepEqual(
        bad.filter((v) => !disputed(v)).map((v) => `${v.verdict} ${v.file} :: ${v.name} ${v.reason ?? JSON.stringify(v.differences)}`),
        [],
      );
      if (checker) {
        // A disputed case that passes is no longer disputed: take it off the list.
        const passing = verdicts.filter((v) => v.verdict === "pass" && `${v.file} :: ${v.name}` in DISPUTED);
        assert.deepEqual(passing.map((v) => v.name), []);
      }
    });
  }
}
