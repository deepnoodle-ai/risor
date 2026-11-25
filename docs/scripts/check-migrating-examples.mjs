#!/usr/bin/env node
// Verifies every Risor v3 code block and Go host snippet in docs/migrating.md:
// - Risor blocks are parsed, type-checked, compiled, run against the TypeScript
//   implementation, and asserted against their exact expected output.
// - Go v3 snippets are wrapped into a temporary module replacing
//   github.com/deepnoodle-ai/risor/go/v3 with ./go and compiled with `go build`.
//
// Usage:
//   node docs/scripts/check-migrating-examples.mjs

import assert from "node:assert/strict";
import { execFileSync } from "node:child_process";
import fs from "node:fs";
import os from "node:os";
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

// Every `js`, `risor`, and `go` fenced block in docs/migrating.md must have a
// risor-verify directive (either verified or explicitly skipped for legacy
// "Before" snippets).
const goBlocks = [];
const risorBlocks = [];

for (const block of blocks) {
  if (block.lang === "go") {
    if (!block.directive) {
      throw new Error(
        `docs/migrating.md:${block.line}: \`\`\`go block is missing a <!-- risor-verify: {...} --> directive`,
      );
    }
    if (block.directive.mode === "go") {
      goBlocks.push(block);
    } else if (block.directive.mode !== "go-skip") {
      throw new Error(
        `docs/migrating.md:${block.line}: unexpected mode ${JSON.stringify(block.directive.mode)} for \`\`\`go block`,
      );
    }
    continue;
  }
  if (block.lang === "js" || block.lang === "risor" || block.directive !== null) {
    risorBlocks.push(block);
  }
}

if (risorBlocks.length === 0) {
  throw new Error("No Risor v3 examples found in docs/migrating.md");
}
if (goBlocks.length === 0) {
  throw new Error("No Go v3 examples found in docs/migrating.md");
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

/**
 * Splits a Go snippet into imports, top-level type/func declarations, and
 * function-body statements, then wraps it in a standalone compilable package.
 * @param {string} pkgName
 * @param {string} code
 * @param {{ imports?: string[], params?: string, use?: string[] }} cfg
 */
function wrapGoSnippet(pkgName, code, cfg) {
  const srcLines = code.split("\n");
  /** @type {string[]} */
  const importSpecs = [];
  /** @type {string[]} */
  const topDecls = [];
  /** @type {string[]} */
  const bodyLines = [];

  const countBraces = (line) => {
    let delta = 0;
    let inBacktick = false;
    let inString = false;
    let escaped = false;
    for (let j = 0; j < line.length; j++) {
      const ch = line[j];
      if (inBacktick) {
        if (ch === "`") inBacktick = false;
        continue;
      }
      if (inString) {
        if (escaped) {
          escaped = false;
        } else if (ch === "\\") {
          escaped = true;
        } else if (ch === '"') {
          inString = false;
        }
        continue;
      }
      if (ch === "/" && line[j + 1] === "/") break;
      if (ch === "`") inBacktick = true;
      else if (ch === '"') inString = true;
      else if (ch === "{") delta++;
      else if (ch === "}") delta--;
    }
    return delta;
  };

  for (let i = 0; i < srcLines.length; i++) {
    const line = srcLines[i];

    // Multi-line import (...) block
    if (/^import\s*\(\s*$/.test(line)) {
      i++;
      while (i < srcLines.length && !/^\)\s*$/.test(srcLines[i])) {
        const spec = srcLines[i].trim();
        if (spec) importSpecs.push(spec);
        i++;
      }
      continue;
    }

    // Single-line import
    const singleImport = line.match(/^import\s+(.+)$/);
    if (singleImport) {
      importSpecs.push(singleImport[1].trim());
      continue;
    }

    // Top-level type or func declaration (column 0)
    if (/^(type|func)\s+/.test(line)) {
      const declLines = [line];
      let depth = countBraces(line);
      while (depth > 0 && i + 1 < srcLines.length) {
        i++;
        declLines.push(srcLines[i]);
        depth += countBraces(srcLines[i]);
      }
      topDecls.push(declLines.join("\n"));
      continue;
    }

    bodyLines.push(line);
  }

  const importedPaths = new Set();
  for (const spec of importSpecs) {
    const m = spec.match(/"([^"]+)"/);
    if (m) importedPaths.add(m[1]);
  }

  const risorPkg = "github.com/deepnoodle-ai/risor/go/v3";
  if (!importedPaths.has(risorPkg)) {
    importSpecs.push(`risor "${risorPkg}"`);
    importedPaths.add(risorPkg);
  }
  for (const pkg of cfg.imports ?? []) {
    if (!importedPaths.has(pkg)) {
      importSpecs.push(`"${pkg}"`);
      importedPaths.add(pkg);
    }
  }

  const useLines = (cfg.use ?? []).map((name) => `\t_ = ${name}`);
  const params = cfg.params ?? "";

  const parts = [
    `package ${pkgName}`,
    "",
    "import (",
    ...importSpecs.map((spec) => `\t${spec}`),
    ")",
  ];
  if (topDecls.length > 0) {
    parts.push("", ...topDecls);
  }
  parts.push(
    "",
    `func check(${params}) {`,
    ...bodyLines.map((l) => (l ? `\t${l}` : "")),
    ...useLines,
    "}",
    "",
  );
  return parts.join("\n");
}

function verifyGoSnippets() {
  const goModText = fs.readFileSync(path.join(repoRoot, "go", "go.mod"), "utf8");
  const goVerMatch = goModText.match(/^go\s+(\S+)/m);
  if (!goVerMatch) {
    throw new Error("Could not read Go version from go/go.mod");
  }
  const goVersion = goVerMatch[1];
  const goDir = path.join(repoRoot, "go").replaceAll("\\", "/");

  const tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), "risor-migrating-go-"));
  /** @type {Map<string, number>} */
  const pkgToLine = new Map();

  try {
    fs.writeFileSync(
      path.join(tmpDir, "go.mod"),
      [
        "module migrating_examples_check",
        "",
        `go ${goVersion}`,
        "",
        "require github.com/deepnoodle-ai/risor/go/v3 v3.0.0",
        "",
        `replace github.com/deepnoodle-ai/risor/go/v3 => ${goDir}`,
        "",
      ].join("\n"),
    );

    for (let i = 0; i < goBlocks.length; i++) {
      const block = goBlocks[i];
      const pkgName = `s${i + 1}`;
      pkgToLine.set(pkgName, block.line);
      const pkgDir = path.join(tmpDir, pkgName);
      fs.mkdirSync(pkgDir);
      fs.writeFileSync(
        path.join(pkgDir, `${pkgName}.go`),
        wrapGoSnippet(pkgName, block.code, block.directive),
      );
    }

    try {
      execFileSync("go", ["build", "./..."], {
        cwd: tmpDir,
        env: { ...process.env, GOWORK: "off", GOPROXY: "off" },
        encoding: "utf8",
        stdio: "pipe",
      });
    } catch (err) {
      const details = [err.stdout, err.stderr].filter(Boolean).join("\n").trim();
      const mapping = [...pkgToLine.entries()]
        .map(([pkg, line]) => `  ${pkg}/${pkg}.go -> docs/migrating.md:${line}`)
        .join("\n");
      throw new Error(
        `Go snippet compilation failed against ./go:\n${details}\n\nSnippet locations:\n${mapping}`,
      );
    }
  } finally {
    fs.rmSync(tmpDir, { recursive: true, force: true });
  }
}

verifyGoSnippets();

console.log(
  `docs/migrating.md: verified ${verified} of ${risorBlocks.length} Risor v3 examples and compiled ${goBlocks.length} Go snippets.`,
);
