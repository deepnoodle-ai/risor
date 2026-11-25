import assert from "node:assert/strict";
import { mkdtempSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import test from "node:test";

import {
  COST_SYMBOLS,
  REPO_ROOT,
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

function extractMarkdownSection(markdown, headingTitle) {
  const lines = markdown.split("\n");
  let start = -1;
  let level = 0;
  for (let i = 0; i < lines.length; i++) {
    const m = lines[i].match(/^(#{1,6})\s+(.+?)\s*$/);
    if (m && m[2] === headingTitle) {
      start = i + 1;
      level = m[1].length;
      break;
    }
  }
  assert.notEqual(start, -1, `Section "${headingTitle}" not found`);
  let end = lines.length;
  for (let i = start; i < lines.length; i++) {
    const m = lines[i].match(/^(#{1,6})\s+/);
    if (m && m[1].length <= level) {
      end = i;
      break;
    }
  }
  return lines.slice(start, end).join("\n");
}

function parseSymbolTable(sectionText) {
  const rows = new Map();
  const lines = sectionText.split("\n");
  let inTable = false;
  for (let i = 0; i < lines.length; i++) {
    const line = lines[i].trim();
    if (!inTable) {
      if (line === "| Symbol | Meaning |" && lines[i + 1]?.trim() === "| --- | --- |") {
        inTable = true;
        i++;
      }
      continue;
    }
    if (!line.startsWith("|")) break;
    const cells = line
      .slice(1, -1)
      .split("|")
      .map((c) => c.trim());
    if (cells.length >= 2) {
      rows.set(cells[0], cells[1]);
    }
  }
  return rows;
}

test("cost symbol table matches normative spec chapters and covers all suite formulas", () => {
  const specCache = new Map();
  const readSpec = (relPath) => {
    if (!specCache.has(relPath)) {
      specCache.set(relPath, readFileSync(join(REPO_ROOT, relPath), "utf8"));
    }
    return specCache.get(relPath);
  };

  // Parse the two normative symbol tables in the spec.
  const coreSection = extractMarkdownSection(readSpec("spec/core/04-library.md"), "4.2 Symbols");
  const coreTable = parseSymbolTable(coreSection);
  assert.ok(coreTable.size > 0, "Expected symbols in spec/core/04-library.md 4.2");

  const groupsSection = extractMarkdownSection(
    readSpec("spec/packages/f06-groups.md"),
    "3.4 Symbols of formula text",
  );
  const groupsTable = parseSymbolTable(groupsSection);
  assert.ok(groupsTable.size > 0, "Expected symbols in spec/packages/f06-groups.md 3.4");

  const primaryTables = new Map([
    ["spec/core/04-library.md#4.2 Symbols", coreTable],
    ["spec/packages/f06-groups.md#3.4 Symbols of formula text", groupsTable],
  ]);

  // 1. Every symbol in COST_SYMBOLS must be defined in its primary spec table
  //    and mentioned in every additional cited spec section.
  for (const row of COST_SYMBOLS) {
    assert.ok(row.sources.length > 0, `COST_SYMBOLS row ${row.symbol} has no spec source`);
    const [primary, ...extras] = row.sources;
    const primaryKey = `${primary.file}#${primary.section}`;
    const table = primaryTables.get(primaryKey);
    assert.ok(table, `Unknown primary spec table ${primaryKey} for ${row.symbol}`);
    assert.ok(
      table.has(row.symbol),
      `Symbol ${row.symbol} is not defined in ${primary.file} section ${primary.section}`,
    );

    for (const extra of extras) {
      const sectionText = extractMarkdownSection(readSpec(extra.file), extra.section);
      assert.ok(
        sectionText.includes(row.symbol),
        `Symbol ${row.symbol} is not referenced in ${extra.file} section ${extra.section}`,
      );
    }
  }

  // 2. Conversely, every symbol defined in spec/core/04-library.md 4.2 and
  //    spec/packages/f06-groups.md 3.4 must appear in COST_SYMBOLS.
  const docSymbols = new Set(COST_SYMBOLS.map((r) => r.symbol));
  for (const sym of [...coreTable.keys(), ...groupsTable.keys()]) {
    assert.ok(docSymbols.has(sym), `Spec symbol ${sym} is missing from COST_SYMBOLS`);
  }
  assert.equal(docSymbols.size, coreTable.size + groupsTable.size);

  // 3. Verify F7's extensions to 4.2 symbols are cited in COST_SYMBOLS.
  const queriesSection = extractMarkdownSection(
    readSpec("spec/packages/f07-queries.md"),
    "6.2 Shared rules",
  );
  const queriesSymbolsLine = queriesSection
    .split("\n")
    .find((line) => line.startsWith("| 4.2, symbols |"));
  assert.ok(queriesSymbolsLine, "Expected `4.2, symbols` row in spec/packages/f07-queries.md 6.2");
  for (const sym of ["`g`", "`sz(keys)`"]) {
    assert.ok(queriesSymbolsLine.includes(sym));
    const row = COST_SYMBOLS.find((r) => r.symbol === sym);
    assert.ok(
      row.sources.some((s) => s.file === "spec/packages/f07-queries.md"),
      `COST_SYMBOLS row ${sym} must cite spec/packages/f07-queries.md`,
    );
  }

  // 4. Every symbol and call appearing in suite/library*.json steps/units formulas
  //    must be covered by COST_SYMBOLS.
  const keys = new Set(COST_SYMBOLS.map((r) => r.key));
  const { entries } = loadLibraryEntries(SUITE_DIR);

  const checkFormula = (formula, entry, paramNames) => {
    let expr = formula.trim();
    const condMatch = expr.match(/^([A-Za-z_]\w*)\s+is\s+string\s*\?\s*(.+?)\s*:\s*(.+)$/);
    const branches = condMatch ? [condMatch[2], condMatch[3]] : [expr];
    if (condMatch) {
      assert.ok(
        paramNames.has(condMatch[1]),
        `${entry.id}: conditional variable "${condMatch[1]}" in "${formula}" is not a parameter`,
      );
    }

    for (const branch of branches) {
      const stripped = branch.replace(/([A-Za-z_]\w*)\(([A-Za-z_]\w*)\)/g, (_, fn, arg) => {
        const exact = `${fn}(${arg})`;
        if (keys.has(exact)) return " 0 ";
        assert.ok(
          keys.has(`${fn}(a)`),
          `${entry.id}: call "${exact}" in "${formula}" is not covered by COST_SYMBOLS`,
        );
        const validArg =
          paramNames.has(arg) || (arg === "s" && entry.receiverKind === "string");
        assert.ok(
          validArg,
          `${entry.id}: call argument "${arg}" in "${formula}" is neither a parameter nor string receiver`,
        );
        return " 0 ";
      });

      for (const m of stripped.matchAll(/\b([A-Za-z_]\w*)\b/g)) {
        const sym = m[1];
        assert.ok(
          keys.has(sym),
          `${entry.id}: symbol "${sym}" in "${formula}" is not covered by COST_SYMBOLS`,
        );
      }
    }
  };

  for (const entry of entries) {
    const paramNames = new Set(
      entry.isConstant ? [] : parseSignature(entry.signature).params.map((p) => p.name),
    );
    checkFormula(entry.steps, entry, paramNames);
    checkFormula(entry.units, entry, paramNames);
  }

  // 5. Verify README.md renders every COST_SYMBOLS row and cites spec/core/04-library.md 2.7.
  const sigSection = extractMarkdownSection(readSpec("spec/core/04-library.md"), "2.7 Signatures").replace(
    /\s+/g,
    " ",
  );
  for (const token of ["`int`", "`float`", "`T[]`", "`map<T>`", "`T | null`", "`where T: ordered`", "`where T: number`", "`!`", "`never`"]) {
    assert.ok(sigSection.includes(token), `Expected ${token} in spec/core/04-library.md 2.7`);
  }

  const index = generateLibraryDocs(SUITE_DIR).outputs.get("README.md");
  for (const row of COST_SYMBOLS) {
    assert.ok(index.includes(`| ${row.symbol} |`), `README.md missing cost symbol ${row.symbol}`);
  }
});
