// Publish every npm package, in dependency order. The release workflow runs
// it on the tag npm-v<version>, after the build and the tests.
//
//   node scripts/publish.ts --dry-run   # pack and check, publish nothing
//   node scripts/publish.ts --stage     # stage each version (CI: trusted publishing)
//   node scripts/publish.ts             # publish directly (by hand, with 2FA)
//
// The release workflow stages: npm holds each version until a maintainer
// approves it with 2FA on npmjs.com (Staged Packages) or with
// `npm stage approve`. The trusted publisher may stage, not publish.
//
// A version with a prerelease part (3.0.0-alpha.2) publishes under the
// dist-tag of its first identifier (alpha); others under latest. A package
// already on the registry at this version is skipped, so a failed release
// can run again.

import { execFileSync } from "node:child_process";
import { readFileSync } from "node:fs";
import { join } from "node:path";
import { PACKAGES } from "./packages.ts";

const dryRun = process.argv.includes("--dry-run");
const stage = process.argv.includes("--stage");
const web = join(import.meta.dirname, "..");

const versions = PACKAGES.map((p) => JSON.parse(readFileSync(join(web, p.dir, "package.json"), "utf8")).version as string);
const version = versions[0];
const odd = PACKAGES.filter((_, i) => versions[i] !== version);
if (odd.length > 0) fail(`every package must carry ${version}; run node scripts/version.ts: ${odd.map((p) => p.name).join(", ")}`);

const ref = process.env.GITHUB_REF_NAME;
if (ref !== undefined && ref !== `npm-v${version}`) fail(`tag ${ref} does not match the packages' version ${version}`);

const pre = version.split("-")[1];
const distTag = pre ? pre.split(".")[0] : "latest";
console.log(`${dryRun ? "dry run: " : ""}${stage ? "stage " : ""}${version} under the dist-tag ${distTag}`);

for (const p of PACKAGES) {
  if (!dryRun && published(p.name, version)) {
    console.log(`${p.name}@${version} is on the registry; skipped`);
    continue;
  }
  const args = [...(stage ? ["stage", "publish"] : ["publish"]), "-w", p.name, "--tag", distTag, "--access", "public"];
  if (dryRun) args.push("--dry-run");
  execFileSync("npm", args, { cwd: web, stdio: "inherit" });
}

if (stage && !dryRun) {
  console.log(`\nStaged ${version}. Approve each package at https://www.npmjs.com (your account, Staged Packages),\nin dependency order: ${PACKAGES.map((p) => p.name).join(", ")}.`);
} else if (!dryRun) {
  // The registry's read replicas lag behind a publish.
  for (const p of PACKAGES) {
    let ok = false;
    for (let attempt = 1; attempt <= 6 && !ok; attempt++) {
      ok = published(p.name, version);
      if (!ok) {
        console.log(`${p.name}@${version} not served yet (attempt ${attempt}); waiting 10 s`);
        await new Promise((r) => setTimeout(r, 10_000));
      }
    }
    if (!ok) fail(`the registry never served ${p.name}@${version}`);
    console.log(`the registry serves ${p.name}@${version}`);
  }
}

function published(name: string, v: string): boolean {
  try {
    return execFileSync("npm", ["view", `${name}@${v}`, "version"], { encoding: "utf8", stdio: ["ignore", "pipe", "ignore"] }).trim() === v;
  } catch {
    return false;
  }
}

function fail(message: string): never {
  console.error(message);
  process.exit(1);
}
