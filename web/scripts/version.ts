// Set one version on every npm package, and on the dependencies between
// them (design: web-packages.md, Release). Then refresh both lockfiles.
//
//   node scripts/version.ts 3.0.0-alpha.2
//
// The release workflow publishes on the tag npm-v<version>, and checks that
// every package carries that version.

import { execFileSync } from "node:child_process";
import { readdirSync, readFileSync, writeFileSync } from "node:fs";
import { join } from "node:path";
import { PACKAGES } from "./packages.ts";

const version = process.argv[2];
if (!version || !/^\d+\.\d+\.\d+(-[0-9A-Za-z.-]+)?$/.test(version)) {
  console.error("usage: node scripts/version.ts <version>, for example 3.0.0-alpha.2");
  process.exit(2);
}

const web = join(import.meta.dirname, "..");
const names = new Set(PACKAGES.map((p) => p.name));
// The example apps are private and keep their own version; their
// dependencies on the packages move with the packages.
const examples = readdirSync(join(web, "examples")).map((d) => ({ name: null, dir: join("examples", d) }));
for (const p of [...PACKAGES, ...examples]) {
  const file = join(web, p.dir, "package.json");
  const json = JSON.parse(readFileSync(file, "utf8"));
  if (p.name) json.version = version;
  for (const field of ["dependencies", "peerDependencies", "devDependencies"]) {
    for (const dep of Object.keys(json[field] ?? {})) {
      if (names.has(dep)) json[field][dep] = version;
    }
  }
  writeFileSync(file, JSON.stringify(json, null, 2) + "\n");
  console.log(`${p.name ?? p.dir} ${version}`);
}

execFileSync("npm", ["install", "--package-lock-only", "--ignore-scripts"], { cwd: join(web, "../ts"), stdio: "inherit" });
execFileSync("npm", ["install", "--package-lock-only", "--ignore-scripts"], { cwd: web, stdio: "inherit" });
