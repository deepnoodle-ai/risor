// The outcome command (suite/README.md): `node scripts/outcome.ts FILE` writes one
// line of JSON for each case of a case file, in order. Status 0 when every
// case has its line; status 2, with a message on standard error, when the
// file is invalid (03 3.10.10). It sets no deadline and reads no clock.

import { loadCaseFile } from "../src/suite/casefile.ts";
import { outcomeLine } from "../src/suite/outcome.ts";

const path = process.argv[2];
if (path === undefined) {
  process.stderr.write("usage: node scripts/outcome.ts FILE\n");
  process.exit(2);
}
const file = loadCaseFile(path, { noExpect: true });
if (!file.ok) {
  process.stderr.write(`invalid case file: ${file.error}\n`);
  process.exit(2);
}
const out: string[] = [];
for (const c of file.cases) out.push(outcomeLine(c));
process.stdout.write(out.join("\n") + (out.length > 0 ? "\n" : ""));
