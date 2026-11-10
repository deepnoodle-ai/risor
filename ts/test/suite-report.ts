// Run case files and print each verdict that is not `pass`, and the counts.
//
//   node test/suite-report.ts                 # suite/core-run, library.json, messages.json
//   node test/suite-report.ts path ...        # any case files or directories
//   node test/suite-report.ts -v path ...     # also print every skip reason
//   RISOR_NO_CHECK=1 node test/suite-report.ts # step 4 with the static rules alone

import { existsSync, statSync } from "node:fs";
import { dirname, join, relative } from "node:path";
import { fileURLToPath } from "node:url";
import { caseFiles, countVerdicts, libraryFunctionNames, runFile } from "../src/suite/runner.ts";
import type { Verdict } from "../src/suite/runner.ts";
import { DISPUTED } from "./disputed.ts";

const here = dirname(fileURLToPath(import.meta.url));
export const SUITE = join(here, "..", "..", "suite");

export function defaultTargets(): string[] {
  return [join(SUITE, "core-run"), join(SUITE, "core-check"), join(SUITE, "strings"), join(SUITE, "host-types"), join(SUITE, "templates"), join(SUITE, "analysis"), join(SUITE, "library.json"), join(SUITE, "messages.json")].filter((p) => existsSync(p));
}

export function runTargets(targets: string[], checker = true): Map<string, Verdict[]> {
  const library = libraryFunctionNames(join(SUITE, "library.json"));
  const out = new Map<string, Verdict[]>();
  for (const t of targets) {
    const files = statSync(t).isDirectory() ? caseFiles(t) : [t];
    const vs: Verdict[] = [];
    for (const f of files) vs.push(...runFile(f, { library, checker }));
    out.set(t, vs);
  }
  return out;
}

function main(): void {
  const args = process.argv.slice(2);
  const verbose = args[0] === "-v";
  const targets = verbose ? args.slice(1) : args;
  const results = runTargets(targets.length > 0 ? targets : defaultTargets(), process.env.RISOR_NO_CHECK === undefined);
  let failed = false;
  for (const [t, vs] of results) {
    for (const v of vs) {
      if (v.verdict === "fail" || v.verdict === "invalid" || (verbose && v.verdict === "skip")) {
        console.log(`${v.verdict.toUpperCase()} ${v.file} :: ${v.name}${v.reason ? ` (${v.reason})` : ""}`);
        for (const d of v.differences ?? []) {
          console.log(`  ${d.field}: expected ${JSON.stringify(d.expected)}`);
          console.log(`  ${" ".repeat(d.field.length)}  found    ${JSON.stringify(d.found)}`);
        }
      }
      // A case listed in test/disputed.ts is reported, and does not fail the command.
      if ((v.verdict === "fail" || v.verdict === "invalid") && !(`${v.file} :: ${v.name}` in DISPUTED)) failed = true;
    }
    const n = countVerdicts(vs);
    console.log(`${relative(process.cwd(), t) || t}: pass ${n.pass}, fail ${n.fail}, skip ${n.skip}, invalid ${n.invalid}`);
  }
  process.exitCode = failed ? 1 : 0;
}

if (process.argv[1] !== undefined && fileURLToPath(import.meta.url) === process.argv[1]) main();
