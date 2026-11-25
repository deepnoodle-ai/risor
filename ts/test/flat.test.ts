// 01 13.1, 13.2 rule 6, f04 2.8, f07 2.5: what opens no nesting level costs no
// stack, in any pass. Each form of fixtures/flat-forms.ts is written with
// 10,000 links and goes through every entry point of the binding: parse, the
// static rules and the checker, compile and run with and without the check,
// the analysis, the formatter and its laws, completion, hover, the tokens, and
// a template. The value, the steps and the allocation units of each run are
// exact, and no entry point throws a RangeError.
//
// The tree of such a program is as deep as the program is long: a receiver is
// inside its step, a left operand inside its operator, an `else` branch inside
// its conditional. So is the formatter's document. Every walk of either is a
// loop with its own stack (src/index.ts, STACK).

import { describe, test } from "node:test";
import assert from "node:assert/strict";
import { execFile } from "node:child_process";
import { mkdtempSync, writeFileSync } from "node:fs";
import { availableParallelism, tmpdir } from "node:os";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import { FORMS } from "./fixtures/flat-forms.ts";

const here = dirname(fileURLToPath(import.meta.url));

/** The links of each program. */
const LINKS = 10000;

/**
 * Run a script in a process of its own, with the stack of test/deep.test.ts:
 * 656 KB, Node's default divided by 1.5. The parts run side by side, so the
 * file takes the time of its longest parts and not the sum of them all.
 */
function node(args: string[]): Promise<{ status: number; stdout: string; stderr: string }> {
  return new Promise((resolve) => {
    execFile(process.execPath, ["--stack-size=656", ...args], { encoding: "utf8", maxBuffer: 1 << 26 }, (err, stdout, stderr) => {
      resolve({ status: err === null ? 0 : typeof err.code === "number" ? err.code : 1, stdout, stderr });
    });
  });
}

async function part(name: string): Promise<void> {
  const r = await node([join(here, "fixtures", "flat-run.ts"), name]);
  assert.equal(r.status, 0, r.stderr.slice(0, 4000));
  assert.equal(r.stdout, "ok\n");
}

describe("what opens no level costs no stack (01 13.2 rule 6)", { concurrency: Math.max(2, Math.min(8, availableParallelism() - 1)) }, () => {
  for (const f of FORMS) test(`${f.name}: ${LINKS} links through every entry point`, () => part(f.name));
  test("the text of the formatter for a long chain", () => part("formatted text"));
  test("a diagnostic at the end of a long flat program", () => part("diagnostics"));
  test("the runner of case files on long flat cases", () => part("runner"));

  // The three programs of the report that this file answers, with the outcomes it gives for 10,000 links.
  const REPORTED = [
    { name: "as", source: "y" + " as any".repeat(LINKS), type: "any", value: "a", steps: 0, units: 0 },
    { name: "projection", source: "[[y]]" + "[*].map(x => x)".repeat(LINKS), type: "string[][]", value: [["a"]], steps: 5 * LINKS, units: 4 * LINKS + 4 },
    { name: "trim", source: "y" + ".trim()".repeat(LINKS), type: "string", value: "a", steps: LINKS, units: LINKS },
  ];

  test("the outcome command on long flat cases", async () => {
    const dir = mkdtempSync(join(tmpdir(), "risor-flat-"));
    const file = join(dir, "flat.json");
    writeFileSync(file, JSON.stringify(REPORTED.map((c) => ({ name: c.name, source: c.source, env: "{y: string}", vars: { y: "a" } }))));
    const r = await node([join(here, "..", "scripts", "outcome.ts"), file]);
    assert.equal(r.status, 0, r.stderr.slice(0, 2000));
    const lines = r.stdout.trimEnd().split("\n").map((l) => JSON.parse(l));
    assert.deepEqual(
      lines.map((l) => ({ name: l.name, type: l.type, sites: l.sites, phase: l.phase, value: l.value, steps: l.steps, units: l.units })),
      REPORTED.map((c) => ({ name: c.name, type: c.type, sites: [], phase: null, value: c.value, steps: c.steps, units: c.units })),
    );
  });
});
