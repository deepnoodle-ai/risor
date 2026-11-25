import assert from "node:assert/strict";
import { mkdtempSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import test from "node:test";

import {
  SUITE_DIR,
  checkLibraryDocs,
  generateLibraryDocs,
  getCanonicalId,
  listLibraryFiles,
  loadLibraryEntries,
  parseSignature,
  writeLibraryDocs,
} from "./generate-library-docs.mjs";

test("parseSignature handles callbacks, optional params, variadic params, and where clauses", () => {
  const reduce = parseSignature("(f: (acc: A, item: T, index: int) => A, init: A) => A");
  assert.equal(reduce.params.length, 2);
  assert.equal(reduce.params[0].name, "f");
  assert.equal(reduce.params[1].name, "init");
  assert.equal(reduce.returnType, "A");
  assert.equal(reduce.mayFail, false);
  assert.equal(reduce.whereClause, null);

  const sortBy = parseSignature(
    '(f: (item: T, index: int) => K, order?: "asc" | "desc") => T[] where K: ordered',
  );
  assert.equal(sortBy.params.length, 2);
  assert.equal(sortBy.params[0].name, "f");
  assert.equal(sortBy.params[1].name, "order");
  assert.equal(sortBy.params[1].optional, true);
  assert.equal(sortBy.returnType, "T[]");
  assert.equal(sortBy.whereClause, "K: ordered");

  const min = parseSignature("(a: T, b: T, ...rest: T[]) => T where T: ordered");
  assert.equal(min.params.length, 3);
  assert.equal(min.params[2].name, "rest");
  assert.equal(min.params[2].rest, true);

  const parse = parseSignature("(s: string) => any !");
  assert.equal(parse.returnType, "any");
  assert.equal(parse.mayFail, true);
});

test("every record across all suite/library*.json files is loaded and rendered", () => {
  const files = listLibraryFiles(SUITE_DIR);
  assert.ok(files.includes("library.json"));
  assert.ok(files.includes("library-groups.json"));
  assert.ok(files.includes("library-math.json"));
  assert.ok(files.includes("library-queries.json"));
  assert.ok(files.includes("library-recursion.json"));

  // Collect every unique canonical ID across all raw suite/library*.json files.
  const expectedIds = new Set();
  for (const file of files) {
    const raw = JSON.parse(readFileSync(join(SUITE_DIR, file), "utf8"));
    for (const r of raw) {
      expectedIds.add(r.extends || getCanonicalId(r));
    }
  }

  const { entries, outputs, pages } = generateLibraryDocs(SUITE_DIR);
  assert.equal(entries.length, expectedIds.size);

  // Verify extends merged sortBy's optional order parameter and cases.
  const sortBy = entries.find((e) => e.id === "list.sortBy");
  assert.ok(sortBy);
  assert.match(sortBy.signature, /order\?:/);
  assert.ok(sortBy.cases.length >= 15);

  // Verify every entry heading, doc text, and cost formulas appear in its page.
  for (const page of pages) {
    const content = outputs.get(page.file);
    assert.ok(content, `missing rendered page ${page.file}`);
    for (const entry of page.entries) {
      assert.ok(
        content.includes(`## \`${entry.name}\``),
        `${page.file} missing heading for ${entry.id}`,
      );
      assert.ok(
        content.includes(entry.doc),
        `${page.file} missing doc text for ${entry.id}`,
      );
      assert.ok(
        content.includes(`\`${entry.steps}\``),
        `${page.file} missing steps formula for ${entry.id}`,
      );
      assert.ok(
        content.includes(`\`${entry.units}\``),
        `${page.file} missing units formula for ${entry.id}`,
      );
      assert.ok(
        content.includes("**Examples**"),
        `${page.file} missing examples section`,
      );
    }

    // Verify every relative anchor link in the summary table points to a heading on the page.
    const headings = new Set(
      [...content.matchAll(/^## `([^`]+)`$/gm)].map((m) =>
        m[1].toLowerCase().replace(/[^a-z0-9_-]/g, ""),
      ),
    );
    for (const m of content.matchAll(/\]\(#([a-z0-9_-]+)\)/g)) {
      assert.ok(headings.has(m[1]), `${page.file}: broken anchor #${m[1]}`);
    }
  }

  // Verify index links every generated page.
  const index = outputs.get("README.md");
  for (const page of pages) {
    assert.ok(index.includes(`(${page.file})`), `README.md missing link to ${page.file}`);
  }
});

test("generation is deterministic and --check detects drift and stale files", () => {
  const first = generateLibraryDocs(SUITE_DIR).outputs;
  const second = generateLibraryDocs(SUITE_DIR).outputs;
  assert.deepEqual([...first.entries()], [...second.entries()]);

  // Committed docs/library/ must match generated output.
  const liveCheck = checkLibraryDocs(SUITE_DIR);
  assert.equal(liveCheck.ok, true, `Expected docs/library/ to be up to date: ${liveCheck.diffs.join("; ")}`);

  // Test drift and stale file detection in a temp dir.
  const dir = mkdtempSync(join(tmpdir(), "risor-lib-docs-"));
  try {
    writeLibraryDocs(SUITE_DIR, dir);
    assert.equal(checkLibraryDocs(SUITE_DIR, dir).ok, true);

    // Modify one file -> check fails.
    const target = join(dir, "math.md");
    writeFileSync(target, readFileSync(target, "utf8") + "\ndrift\n", "utf8");
    const drifted = checkLibraryDocs(SUITE_DIR, dir);
    assert.equal(drifted.ok, false);
    assert.ok(drifted.diffs.some((d) => d.includes("math.md")));

    // Re-write fixes drift; adding a stale file triggers failure; re-write prunes it.
    writeLibraryDocs(SUITE_DIR, dir);
    writeFileSync(join(dir, "stale.md"), "stale\n", "utf8");
    const stale = checkLibraryDocs(SUITE_DIR, dir);
    assert.equal(stale.ok, false);
    assert.ok(stale.diffs.some((d) => d.includes("stale.md")));

    writeLibraryDocs(SUITE_DIR, dir);
    assert.equal(checkLibraryDocs(SUITE_DIR, dir).ok, true);
  } finally {
    rmSync(dir, { recursive: true, force: true });
  }
});
