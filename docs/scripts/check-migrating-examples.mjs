#!/usr/bin/env node
// Verifies every Risor v3 code block in docs/migrating.md against the
// TypeScript Risor v3 implementation: parses, type-checks, compiles, runs
// (or calls an entry point), and asserts the exact expected output.
//
// Usage:
//   node docs/scripts/check-migrating-examples.mjs

import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

import {
  createEnv,
  jsonText,
  parse,
  parseTemplate,
  RisorError,
} from "../../ts/src/index.ts";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const repoRoot = path.resolve(__dirname, "../..");
const docPath = path.join(repoRoot, "docs", "migrating.md");

const markdown = fs.readFileSync(docPath, "utf8");
const lines = markdown.split("\n");

/** @typedef {{ line: number, lang: string, code: string, directive: any, directiveLine: number | null }} Block */

/** @type {Block[]} */
const blocks = [];

let pendingDirective = null;
let pendingDirectiveLine = null;

for (let i = 0; i < lines.length; i++) {
  const line = lines[i];

  const dirMatch = line.match(/^\s*<!--\s*risor-verify:\s*(\{.*\})\s*-->$/);
  if (dirMatch) {
    try {
      pendingDirective = JSON.parse(dirMatch[1]);
      pendingDirectiveLine = i + 1;
    } catch (err) {
      throw new Error(
        `docs/migrating.md:${i + 1}: invalid JSON in risor-verify comment: ${err.message}`,
      );
    }
    continue;
  }

  const fenceMatch = line.match(/^(\s*)```([a-zA-Z0-9_-]*)\s*$/);
  if (fenceMatch) {
    const indent = fenceMatch[1];
    const lang = fenceMatch[2];
    const startLine = i + 1;
    const bodyLines = [];
    i++;
    while (i < lines.length && !lines[i].match(/^\s*```\s*$/)) {
      const raw = lines[i];
      bodyLines.push(
        indent && raw.startsWith(indent) ? raw.slice(indent.length) : raw,
      );
      i++;
    }
    blocks.push({
      line: startLine,
      lang,
      code: bodyLines.join("\n"),
      directive: pendingDirective,
      directiveLine: pendingDirectiveLine,
    });
    pendingDirective = null;
    pendingDirectiveLine = null;
    continue;
  }

  // Only allow blank lines between a directive and its code block.
  if (pendingDirective !== null && line.trim() !== "") {
    throw new Error(
      `docs/migrating.md:${pendingDirectiveLine}: risor-verify comment is not followed by a fenced code block`,
    );
  }
}

if (pendingDirective !== null) {
  throw new Error(
    `docs/migrating.md:${pendingDirectiveLine}: trailing risor-verify comment with no code block`,
  );
}

// Every `js` or `risor` fenced block in docs/migrating.md must be verified.
const risorBlocks = blocks.filter(
  (b) => b.lang === "js" || b.lang === "risor" || b.directive !== null,
);

if (risorBlocks.length === 0) {
  throw new Error("No Risor v3 examples found in docs/migrating.md");
}

let verified = 0;

for (const block of risorBlocks) {
  if (!block.directive) {
    throw new Error(
      `docs/migrating.md:${block.line}: \`\`\`${block.lang} block is missing a <!-- risor-verify: {...} --> directive`,
    );
  }

  const cfg = block.directive;
  const loc = `docs/migrating.md:${block.line}`;
  const mode = cfg.mode ?? "expr";

  try {
    const env = createEnv(cfg.env ?? {});
    let actual;
    if (mode === "expr") {
      const expr = parse(block.code);
      const prog = env.compile(expr, cfg.compile ?? {});
      actual = prog.run(cfg.vars ?? {});
    } else if (mode === "entry") {
      const expr = parse(block.code);
      const prog = env.compile(expr, cfg.compile ?? {});
      if ("input" in cfg) {
        actual = prog.callRecord(cfg.entry, cfg.input, cfg.vars ?? {});
      } else {
        actual = prog.call(cfg.entry, cfg.args ?? [], cfg.vars ?? {});
      }
    } else if (mode === "template") {
      const tmpl = parseTemplate(block.code);
      const prog = env.compile(
        tmpl.program(cfg.templateMode ?? "string"),
        cfg.compile ?? {},
      );
      actual = prog.run(cfg.vars ?? {});
    } else {
      throw new Error(`${loc}: unknown mode ${JSON.stringify(mode)}`);
    }

    if ("error" in cfg) {
      throw new Error(
        `${loc}: expected error ${JSON.stringify(cfg.error)}, but succeeded with ${jsonText(actual)}`,
      );
    }

    const gotJson = jsonText(actual);
    const wantJson = jsonText(cfg.expect);
    assert.equal(
      gotJson,
      wantJson,
      `${loc}: result mismatch\n  code: ${block.code}\n  got:  ${gotJson}\n  want: ${wantJson}`,
    );
    verified++;
  } catch (err) {
    if ("error" in cfg && err instanceof RisorError) {
      const diag = err.diagnostics[0];
      assert.equal(
        diag?.code,
        cfg.error,
        `${loc}: expected error code ${cfg.error}, got ${diag?.code} (${err.message})`,
      );
      verified++;
      continue;
    }
    if (err instanceof RisorError) {
      throw new Error(`${loc}: Risor error: ${err.message}\n---\n${block.code}\n---`);
    }
    throw err;
  }
}

console.log(
  `docs/migrating.md: verified ${verified} of ${risorBlocks.length} Risor v3 examples.`,
);
