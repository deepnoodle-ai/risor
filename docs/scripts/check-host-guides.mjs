#!/usr/bin/env node
/**
 * Extracts and verifies every Go, TypeScript, and Rust code example in
 * docs/hosts/{go,typescript,rust}.md by compiling AND executing them against
 * the local implementations (./go, ./ts, ./rust) with zero network access.
 *
 * Every ```go, ```ts, ```typescript, ```rust, or ```rs fenced block MUST carry
 * an immediately preceding `<!-- risor-verify: {...} -->` JSON directive.
 *
 * Usage:
 *   node docs/scripts/check-host-guides.mjs
 *   node docs/scripts/check-host-guides.mjs --lang=go
 *   node docs/scripts/check-host-guides.mjs --lang=ts [--typecheck]
 *   node docs/scripts/check-host-guides.mjs --lang=rust
 */

import { execFileSync } from "node:child_process";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { fileURLToPath } from "node:url";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
export const REPO_ROOT = fs.realpathSync(path.resolve(__dirname, "..", ".."));

function mkRealTempDir(prefix) {
  return fs.realpathSync(fs.mkdtempSync(path.join(os.tmpdir(), prefix)));
}

export const DEFAULT_GUIDES = {
  go: path.join(REPO_ROOT, "docs", "hosts", "go.md"),
  ts: path.join(REPO_ROOT, "docs", "hosts", "typescript.md"),
  rust: path.join(REPO_ROOT, "docs", "hosts", "rust.md"),
};

const CODE_LANGS = new Set(["go", "ts", "typescript", "rust", "rs"]);
const KNOWN_MODES = new Set(["go", "go-test", "ts", "rust", "skip"]);

/**
 * @typedef {{
 *   index: number,
 *   line: number,
 *   heading: string,
 *   lang: string,
 *   code: string,
 *   directive: Record<string, any> | null,
 *   directiveLine: number | null,
 * }} CodeBlock
 */

/**
 * Extract all fenced code blocks from a Markdown document along with any
 * immediately preceding `<!-- risor-verify: {...} -->` JSON directive.
 * @param {string} markdown
 * @param {string} fileLabel
 * @returns {CodeBlock[]}
 */
export function extractBlocks(markdown, fileLabel = "<input>") {
  const lines = markdown.replace(/\r\n/g, "\n").split("\n");
  /** @type {CodeBlock[]} */
  const blocks = [];

  let heading = "(top)";
  let pendingDirective = null;
  let pendingDirectiveLine = null;

  for (let i = 0; i < lines.length; i++) {
    const line = lines[i];

    const hMatch = /^(#{1,6})\s+(.*)$/.exec(line);
    if (hMatch) {
      if (pendingDirective !== null) {
        throw new Error(
          `${fileLabel}:${pendingDirectiveLine}: risor-verify comment is not followed by a fenced code block`,
        );
      }
      heading = hMatch[2].trim();
      continue;
    }

    const dirMatch = line.match(/^\s*<!--\s*risor-verify:\s*(\{[\s\S]*\})\s*-->$/);
    if (dirMatch) {
      if (pendingDirective !== null) {
        throw new Error(
          `${fileLabel}:${pendingDirectiveLine}: risor-verify comment is not followed by a fenced code block`,
        );
      }
      try {
        const parsed = JSON.parse(dirMatch[1]);
        if (parsed === null || typeof parsed !== "object" || Array.isArray(parsed)) {
          throw new Error("directive payload must be a JSON object");
        }
        pendingDirective = parsed;
        pendingDirectiveLine = i + 1;
      } catch (err) {
        throw new Error(
          `${fileLabel}:${i + 1}: invalid JSON in risor-verify comment: ${err.message}`,
        );
      }
      continue;
    }

    const fenceMatch = /^(\s*)(`{3,})([^`]*)$/.exec(line);
    if (fenceMatch) {
      const indent = fenceMatch[1];
      const fence = fenceMatch[2];
      const lang = fenceMatch[3].trim().toLowerCase();
      const startLine = i + 1;
      const bodyLines = [];
      let closed = false;
      i++;
      while (i < lines.length) {
        if (new RegExp(`^\\s*\`{${fence.length},}\\s*$`).test(lines[i])) {
          closed = true;
          break;
        }
        const raw = lines[i];
        bodyLines.push(
          indent && raw.startsWith(indent) ? raw.slice(indent.length) : raw,
        );
        i++;
      }
      if (!closed) {
        throw new Error(`${fileLabel}:${startLine}: unterminated code fence`);
      }
      blocks.push({
        index: blocks.length + 1,
        line: startLine,
        heading,
        lang,
        code: bodyLines.join("\n"),
        directive: pendingDirective,
        directiveLine: pendingDirectiveLine,
      });
      pendingDirective = null;
      pendingDirectiveLine = null;
      continue;
    }

    if (pendingDirective !== null && line.trim() !== "") {
      throw new Error(
        `${fileLabel}:${pendingDirectiveLine}: risor-verify comment is not followed by a fenced code block`,
      );
    }
  }

  if (pendingDirective !== null) {
    throw new Error(
      `${fileLabel}:${pendingDirectiveLine}: trailing risor-verify comment with no code block`,
    );
  }

  return blocks;
}

/**
 * Validate directives on all code blocks in a guide for a given target language.
 * Fails if any code block in CODE_LANGS is missing a directive or has an invalid mode.
 * @param {CodeBlock[]} blocks
 * @param {"go" | "ts" | "rust"} lang
 * @param {string} fileLabel
 */
export function selectVerifiedBlocks(blocks, lang, fileLabel) {
  const allowedModes =
    lang === "go"
      ? new Set(["go", "go-test", "skip"])
      : lang === "ts"
        ? new Set(["ts", "skip"])
        : new Set(["rust", "skip"]);

  /** @type {CodeBlock[]} */
  const runnable = [];
  let skipped = 0;

  for (const block of blocks) {
    const needsDirective = CODE_LANGS.has(block.lang) || block.directive !== null;
    if (!needsDirective) continue;

    if (!block.directive) {
      throw new Error(
        `${fileLabel}:${block.line} ("${block.heading}"): \`\`\`${block.lang} block is missing a <!-- risor-verify: {...} --> directive`,
      );
    }

    const mode = block.directive.mode;
    if (!KNOWN_MODES.has(mode) || !allowedModes.has(mode)) {
      throw new Error(
        `${fileLabel}:${block.line} ("${block.heading}"): invalid mode ${JSON.stringify(mode)} for ${lang} guide`,
      );
    }

    if (mode === "skip") {
      if (typeof block.directive.reason !== "string" || block.directive.reason.trim() === "") {
        throw new Error(
          `${fileLabel}:${block.line}: skip mode requires a non-empty "reason"`,
        );
      }
      skipped++;
      continue;
    }

    runnable.push(block);
  }

  if (runnable.length === 0) {
    throw new Error(`${fileLabel}: no runnable ${lang} examples found`);
  }

  return { runnable, skipped };
}

function writeAuxFiles(baseDir, files) {
  if (!files || typeof files !== "object") return;
  for (const [relPath, content] of Object.entries(files)) {
    const dest = path.join(baseDir, relPath);
    fs.mkdirSync(path.dirname(dest), { recursive: true });
    fs.writeFileSync(dest, content, "utf8");
  }
}

/**
 * Build the Go verification test file for a `package main` example.
 * Captures os.Stdout while calling main(), and asserts expectStdout if provided.
 */
function buildGoMainTest(expectStdout, fileLabel, line) {
  const hasExpect = typeof expectStdout === "string";
  return [
    "package main",
    "",
    "import (",
    '\t"bytes"',
    '\t"io"',
    '\t"os"',
    '\t"testing"',
    ")",
    "",
    "func TestHostGuideExample(t *testing.T) {",
    "\toldStdout := os.Stdout",
    "\tr, w, err := os.Pipe()",
    "\tif err != nil {",
    "\t\tt.Fatal(err)",
    "\t}",
    "\tos.Stdout = w",
    "\tdone := make(chan string, 1)",
    "\tgo func() {",
    "\t\tvar buf bytes.Buffer",
    "\t\t_, _ = io.Copy(&buf, r)",
    "\t\tdone <- buf.String()",
    "\t}()",
    "\tdefer func() {",
    "\t\tos.Stdout = oldStdout",
    "\t}()",
    "\tmain()",
    "\t_ = w.Close()",
    "\tos.Stdout = oldStdout",
    "\tgot := <-done",
    "\t_ = r.Close()",
    hasExpect
      ? `\twant := ${JSON.stringify(expectStdout)}\n\tif got != want {\n\t\tt.Fatalf(${JSON.stringify(`${fileLabel}:${line}: stdout mismatch`) + ' + "\\n--- want ---\\n%s--- got ---\\n%s"'}, want, got)\n\t}`
      : "\t_ = got",
    "}",
    "",
  ].join("\n");
}

/**
 * Compile and run every Go example in `markdown` offline against ./go.
 */
export function verifyGoGuide(markdown, fileLabel = DEFAULT_GUIDES.go) {
  const blocks = extractBlocks(markdown, fileLabel);
  const { runnable, skipped } = selectVerifiedBlocks(blocks, "go", fileLabel);

  const goModText = fs.readFileSync(path.join(REPO_ROOT, "go", "go.mod"), "utf8");
  const goVerMatch = goModText.match(/^go\s+(\S+)/m);
  if (!goVerMatch) {
    throw new Error("Could not read Go version from go/go.mod");
  }
  const goVersion = goVerMatch[1];
  const goDir = path.join(REPO_ROOT, "go").replaceAll("\\", "/");

  const tmpDir = mkRealTempDir("risor-host-guide-go-");
  const mapping = [];

  try {
    fs.writeFileSync(
      path.join(tmpDir, "go.mod"),
      [
        "module host_guide_check",
        "",
        `go ${goVersion}`,
        "",
        "require github.com/deepnoodle-ai/risor/go/v3 v3.0.0",
        "",
        `replace github.com/deepnoodle-ai/risor/go/v3 => ${goDir}`,
        "",
      ].join("\n"),
      "utf8",
    );

    for (let i = 0; i < runnable.length; i++) {
      const block = runnable[i];
      const pkgDirName = `ex${i + 1}`;
      const pkgDir = path.join(tmpDir, pkgDirName);
      fs.mkdirSync(pkgDir, { recursive: true });
      mapping.push(`  ${pkgDirName} -> ${fileLabel}:${block.line} ("${block.heading}")`);
      writeAuxFiles(pkgDir, block.directive.files);

      if (block.directive.mode === "go-test") {
        fs.writeFileSync(path.join(pkgDir, "guide_test.go"), block.code + "\n", "utf8");
      } else {
        if (!/^package\s+main\b/m.test(block.code)) {
          throw new Error(
            `${fileLabel}:${block.line}: mode "go" examples must declare \`package main\` and \`func main()\``,
          );
        }
        fs.writeFileSync(path.join(pkgDir, "main.go"), block.code + "\n", "utf8");
        fs.writeFileSync(
          path.join(pkgDir, "verify_test.go"),
          buildGoMainTest(block.directive.expectStdout, fileLabel, block.line),
          "utf8",
        );
      }
    }

    try {
      execFileSync("go", ["test", "-count=1", "./..."], {
        cwd: tmpDir,
        env: { ...process.env, GOWORK: "off", GOPROXY: "off" },
        encoding: "utf8",
        stdio: "pipe",
      });
    } catch (err) {
      const details = [err.stdout, err.stderr].filter(Boolean).join("\n").trim();
      throw new Error(
        `Go host guide verification failed (${fileLabel}):\n${details}\n\nExample locations:\n${mapping.join("\n")}`,
      );
    }

    return { lang: "go", verified: runnable.length, skipped };
  } finally {
    fs.rmSync(tmpDir, { recursive: true, force: true });
  }
}

/**
 * Rewrite `@deepnoodle/risor` and `@deepnoodle/risor/testing` imports to local
 * TypeScript sources relative to `fromDir`.
 */
export function rewriteTsImports(code, fromDir) {
  const indexRel = path
    .relative(fromDir, path.join(REPO_ROOT, "ts", "src", "index.ts"))
    .replaceAll("\\", "/");
  const testingRel = path
    .relative(fromDir, path.join(REPO_ROOT, "ts", "src", "testing.ts"))
    .replaceAll("\\", "/");
  const norm = (p) => (p.startsWith(".") ? p : `./${p}`);

  return code
    .replaceAll('"@deepnoodle/risor/testing"', JSON.stringify(norm(testingRel)))
    .replaceAll("'@deepnoodle/risor/testing'", JSON.stringify(norm(testingRel)))
    .replaceAll('"@deepnoodle/risor"', JSON.stringify(norm(indexRel)))
    .replaceAll("'@deepnoodle/risor'", JSON.stringify(norm(indexRel)));
}

/**
 * Execute (and optionally `tsc --noEmit` type-check) every TypeScript example
 * in `markdown` against ./ts/src/index.ts.
 */
export function verifyTsGuide(
  markdown,
  fileLabel = DEFAULT_GUIDES.ts,
  { typecheck = false } = {},
) {
  const blocks = extractBlocks(markdown, fileLabel);
  const { runnable, skipped } = selectVerifiedBlocks(blocks, "ts", fileLabel);

  const tmpDir = mkRealTempDir("risor-host-guide-ts-");
  const writtenFiles = [];
  const mapping = [];

  try {
    fs.writeFileSync(
      path.join(tmpDir, "package.json"),
      JSON.stringify({ type: "module" }, null, 2) + "\n",
      "utf8",
    );
    for (let i = 0; i < runnable.length; i++) {
      const block = runnable[i];
      const exDir = path.join(tmpDir, `ex${i + 1}`);
      fs.mkdirSync(exDir, { recursive: true });
      writeAuxFiles(exDir, block.directive.files);

      const filePath = path.join(exDir, "main.ts");
      const rewritten = rewriteTsImports(block.code, exDir);
      fs.writeFileSync(filePath, rewritten + "\n", "utf8");
      writtenFiles.push({ filePath, exDir, block });
      mapping.push(`  ex${i + 1}/main.ts -> ${fileLabel}:${block.line} ("${block.heading}")`);
    }

    const tscBin = path.join(
      REPO_ROOT,
      "ts",
      "node_modules",
      ".bin",
      process.platform === "win32" ? "tsc.cmd" : "tsc",
    );
    const shouldTypecheck = typecheck || fs.existsSync(tscBin);
    if (shouldTypecheck) {
      if (!fs.existsSync(tscBin)) {
        throw new Error(
          `--typecheck requested but tsc was not found at ${tscBin}. Run \`npm ci --ignore-scripts\` in ts/ first.`,
        );
      }
      const tsconfigPath = path.join(tmpDir, "tsconfig.json");
      const typesRoot = path
        .relative(tmpDir, path.join(REPO_ROOT, "ts", "node_modules", "@types"))
        .replaceAll("\\", "/");
      fs.writeFileSync(
        tsconfigPath,
        JSON.stringify(
          {
            compilerOptions: {
              target: "es2022",
              lib: ["es2024"],
              module: "nodenext",
              moduleResolution: "nodenext",
              types: ["node"],
              typeRoots: [typesRoot.startsWith(".") ? typesRoot : `./${typesRoot}`],
              strict: true,
              noImplicitOverride: true,
              noFallthroughCasesInSwitch: true,
              verbatimModuleSyntax: true,
              erasableSyntaxOnly: true,
              allowImportingTsExtensions: true,
              noEmit: true,
              skipLibCheck: true,
            },
            files: writtenFiles.map((f) => path.relative(tmpDir, f.filePath)),
          },
          null,
          2,
        ),
        "utf8",
      );
      try {
        execFileSync(tscBin, ["-p", tsconfigPath, "--noEmit"], {
          cwd: tmpDir,
          encoding: "utf8",
          stdio: "pipe",
        });
      } catch (err) {
        const details = [err.stdout, err.stderr].filter(Boolean).join("\n").trim();
        throw new Error(
          `TypeScript host guide typecheck failed (${fileLabel}):\n${details}\n\nExample locations:\n${mapping.join("\n")}`,
        );
      }
    }

    // Execute every TypeScript example under Node 22 native TS stripping.
    for (const { filePath, exDir, block } of writtenFiles) {
      let stdout;
      try {
        stdout = execFileSync(process.execPath, [filePath], {
          cwd: exDir,
          encoding: "utf8",
          stdio: "pipe",
        });
      } catch (err) {
        const details = [err.stdout, err.stderr].filter(Boolean).join("\n").trim();
        throw new Error(
          `TypeScript host guide execution failed at ${fileLabel}:${block.line} ("${block.heading}"):\n${details}`,
        );
      }

      if (typeof block.directive.expectStdout === "string") {
        if (stdout !== block.directive.expectStdout) {
          throw new Error(
            `${fileLabel}:${block.line} ("${block.heading}"): stdout mismatch\n--- want ---\n${block.directive.expectStdout}--- got ---\n${stdout}`,
          );
        }
      }
    }

    return {
      lang: "ts",
      verified: runnable.length,
      skipped,
      typechecked: shouldTypecheck,
    };
  } finally {
    fs.rmSync(tmpDir, { recursive: true, force: true });
  }
}

/**
 * Generate the single Rust integration test harness source for all extracted
 * Rust guide examples.
 */
export function buildRustHarness(runnable, fileLabel) {
  const lines = [
    "trait IntoCheckResult {",
    "    fn check(self, loc: &str);",
    "}",
    "",
    "impl IntoCheckResult for () {",
    "    fn check(self, _loc: &str) {}",
    "}",
    "",
    "impl<E: std::fmt::Debug> IntoCheckResult for Result<(), E> {",
    "    fn check(self, loc: &str) {",
    '        if let Err(e) = self { panic!("{loc}: example returned Err: {e:?}"); }',
    "    }",
    "}",
    "",
  ];

  for (let i = 0; i < runnable.length; i++) {
    const block = runnable[i];
    const modName = `ex_${i + 1}`;
    const loc = `${fileLabel}:${block.line} (${block.heading})`;
    lines.push(`mod ${modName} {`);
    lines.push("    #![allow(dead_code)]");
    lines.push(block.code);
    lines.push("");
    lines.push("    #[test]");
    lines.push("    fn run() {");
    lines.push(`        super::IntoCheckResult::check(main(), ${JSON.stringify(loc)});`);
    lines.push("    }");
    lines.push("}");
    lines.push("");
  }

  return lines.join("\n");
}

/**
 * Compile and execute every Rust example in `markdown` offline against ./rust
 * (with feature `derive` enabled).
 */
export function verifyRustGuide(markdown, fileLabel = DEFAULT_GUIDES.rust) {
  const blocks = extractBlocks(markdown, fileLabel);
  const { runnable, skipped } = selectVerifiedBlocks(blocks, "rust", fileLabel);

  const rustDir = path.join(REPO_ROOT, "rust").replaceAll("\\", "/");
  const targetDir = path.join(REPO_ROOT, "rust", "target");
  const tmpDir = mkRealTempDir("risor-host-guide-rust-");
  const mapping = [];

  try {
    fs.writeFileSync(
      path.join(tmpDir, "Cargo.toml"),
      [
        "[package]",
        'name = "risor-host-guide-check"',
        'version = "0.1.0"',
        'edition = "2021"',
        "publish = false",
        "",
        "[dependencies]",
        `risor = { path = ${JSON.stringify(rustDir)}, features = ["derive"] }`,
        "",
      ].join("\n"),
      "utf8",
    );

    const srcDir = path.join(tmpDir, "src");
    const testsDir = path.join(tmpDir, "tests");
    fs.mkdirSync(srcDir, { recursive: true });
    fs.mkdirSync(testsDir, { recursive: true });
    fs.writeFileSync(path.join(srcDir, "lib.rs"), "", "utf8");

    for (let i = 0; i < runnable.length; i++) {
      const block = runnable[i];
      mapping.push(`  test_ex_${i + 1} -> ${fileLabel}:${block.line} ("${block.heading}")`);
      writeAuxFiles(tmpDir, block.directive.files);
    }

    fs.writeFileSync(
      path.join(testsDir, "host_guide.rs"),
      buildRustHarness(runnable, fileLabel),
      "utf8",
    );

    try {
      execFileSync("cargo", ["test", "--offline", "--test", "host_guide"], {
        cwd: tmpDir,
        env: { ...process.env, CARGO_TARGET_DIR: targetDir },
        encoding: "utf8",
        stdio: "pipe",
      });
    } catch (err) {
      const details = [err.stdout, err.stderr].filter(Boolean).join("\n").trim();
      throw new Error(
        `Rust host guide verification failed (${fileLabel}):\n${details}\n\nExample locations:\n${mapping.join("\n")}`,
      );
    }

    return { lang: "rust", verified: runnable.length, skipped };
  } finally {
    fs.rmSync(tmpDir, { recursive: true, force: true });
  }
}

export function verifyHostGuides({ langs = ["go", "ts", "rust"], typecheck = false } = {}) {
  const results = [];
  for (const lang of langs) {
    const filePath = DEFAULT_GUIDES[lang];
    if (!filePath) {
      throw new Error(`Unknown language ${JSON.stringify(lang)} (expected go, ts, or rust)`);
    }
    const markdown = fs.readFileSync(filePath, "utf8");
    const rel = path.relative(REPO_ROOT, filePath);
    if (lang === "go") {
      results.push(verifyGoGuide(markdown, rel));
    } else if (lang === "ts") {
      results.push(verifyTsGuide(markdown, rel, { typecheck }));
    } else if (lang === "rust") {
      results.push(verifyRustGuide(markdown, rel));
    }
  }
  return results;
}

function parseArgs(argv) {
  const langs = [];
  let typecheck = false;
  for (const arg of argv) {
    if (arg.startsWith("--lang=")) {
      const val = arg.slice("--lang=".length);
      for (const part of val.split(",")) {
        if (part) langs.push(part);
      }
    } else if (arg === "--typecheck") {
      typecheck = true;
    } else if (arg === "--help" || arg === "-h") {
      console.log(
        "Usage: node docs/scripts/check-host-guides.mjs [--lang=go|ts|rust] [--typecheck]",
      );
      process.exit(0);
    } else {
      throw new Error(`Unknown argument: ${arg}`);
    }
  }
  return { langs: langs.length > 0 ? langs : ["go", "ts", "rust"], typecheck };
}

function main(argv) {
  const opts = parseArgs(argv.slice(2));
  const results = verifyHostGuides(opts);
  for (const r of results) {
    const extra = r.typechecked ? " (typechecked + executed)" : " (compiled + executed)";
    console.log(
      `docs/hosts/${r.lang === "ts" ? "typescript" : r.lang}.md: verified ${r.verified} example(s)${extra}.`,
    );
  }
}

if (fileURLToPath(import.meta.url) === path.resolve(process.argv[1] || "")) {
  main(process.argv);
}
