#!/usr/bin/env node
/**
 * Extracts every fenced code block from docs/guide.md and verifies it against
 * the TypeScript Risor v3 implementation (ts/src/index.ts).
 *
 * Every Risor code block (```js / ```risor), host template block
 * (```text with {"template": "string" | "value"}), and diagnostic output block
 * (```text with {"matchesPreviousDiagnostic": "detail" | "line"}) is checked.
 * Unverified fenced blocks fail the check so examples cannot silently drift.
 *
 * Usage:
 *   node docs/scripts/check-guide-examples.mjs
 *   node docs/scripts/check-guide-examples.mjs path/to/doc.md
 */

import { readFileSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";

import {
  HostError,
  RisorError,
  createEnv,
  jsonText,
  parse,
  parseTemplate,
  renderDetail,
  renderLine,
} from "../../ts/src/index.ts";

const __dirname = dirname(fileURLToPath(import.meta.url));
export const REPO_ROOT = resolve(__dirname, "..", "..");
export const DEFAULT_GUIDE_PATH = resolve(REPO_ROOT, "docs", "guide.md");

const RISOR_LANGS = new Set(["js", "risor"]);

const KNOWN_DIRECTIVE_KEYS = new Set([
  "args",
  "calls",
  "check",
  "checkError",
  "entries",
  "entry",
  "env",
  "expect",
  "expected",
  "functions",
  "groups",
  "input",
  "limits",
  "matchesPreviousDiagnostic",
  "messageContains",
  "namedTypes",
  "parseError",
  "reject",
  "runError",
  "template",
  "type",
  "vars",
]);

const KNOWN_CALL_KEYS = new Set([
  "args",
  "entry",
  "expect",
  "input",
  "messageContains",
  "runError",
  "vars",
]);

const KNOWN_ENTRY_KEYS = new Set([
  "doc",
  "inputType",
  "name",
  "params",
  "resultType",
]);

const KNOWN_PARAM_KEYS = new Set(["doc", "name", "optional", "type"]);

const KNOWN_FUNCTION_KEYS = new Set(["doc", "error", "returns", "type"]);

function assertKnownKeys(obj, known, label, loc) {
  const unknown = Object.keys(obj).filter((k) => !known.has(k));
  if (unknown.length > 0) {
    throw new Error(`${loc}: unknown ${label} key(s): ${unknown.join(", ")}`);
  }
}

/**
 * Parse all fenced code blocks from a Markdown document, attaching any
 * immediately preceding `<!-- risor { ... } -->` JSON directive.
 */
export function extractGuideBlocks(markdown) {
  const lines = markdown.replace(/\r\n/g, "\n").split("\n");
  const blocks = [];

  let heading = "(top)";
  let i = 0;

  while (i < lines.length) {
    const line = lines[i];

    // Track nearest heading for clear error reporting.
    const hMatch = /^(#{1,6})\s+(.*)$/.exec(line);
    if (hMatch) {
      heading = hMatch[2].trim();
      i++;
      continue;
    }

    // Look for a fenced code block opener.
    const fenceMatch = /^(`{3,})([^`]*)$/.exec(line);
    if (!fenceMatch) {
      i++;
      continue;
    }

    const fence = fenceMatch[1];
    const lang = fenceMatch[2].trim();
    const startLine = i + 1;

    // Look backwards across blank lines for a `<!-- risor ... -->` directive.
    const directive = findPrecedingDirective(lines, i - 1);

    // Collect block body until matching closing fence.
    const bodyLines = [];
    i++;
    let closed = false;
    while (i < lines.length) {
      if (new RegExp(`^\`{${fence.length},}\\s*$`).test(lines[i])) {
        closed = true;
        i++;
        break;
      }
      bodyLines.push(lines[i]);
      i++;
    }
    if (!closed) {
      throw new Error(`Unterminated code fence starting at line ${startLine}`);
    }

    blocks.push({
      index: blocks.length + 1,
      line: startLine,
      heading,
      lang,
      code: bodyLines.join("\n"),
      meta: directive.meta,
      metaLine: directive.line,
    });
  }

  return blocks;
}

/**
 * Scan backwards from `endIdx` skipping blank lines to see if a
 * `<!-- risor { ... } -->` comment directly precedes the code fence.
 */
function findPrecedingDirective(lines, endIdx) {
  let j = endIdx;
  while (j >= 0 && lines[j].trim() === "") {
    j--;
  }
  if (j < 0 || !lines[j].trim().endsWith("-->")) {
    return { meta: null, line: null };
  }

  // Walk up to find the matching `<!--`.
  const commentLines = [];
  let startIdx = j;
  while (startIdx >= 0) {
    commentLines.unshift(lines[startIdx]);
    if (lines[startIdx].includes("<!--")) {
      break;
    }
    startIdx--;
  }
  const commentText = commentLines.join("\n").trim();
  const match = /^<!--\s*risor\b([\s\S]*?)-->$/.exec(commentText);
  if (!match) {
    return { meta: null, line: null };
  }

  const payload = match[1].trim();
  if (payload === "") {
    return { meta: {}, line: startIdx + 1 };
  }

  try {
    const parsed = JSON.parse(payload);
    if (parsed === null || typeof parsed !== "object" || Array.isArray(parsed)) {
      throw new Error("directive payload must be a JSON object");
    }
    return { meta: parsed, line: startIdx + 1 };
  } catch (err) {
    throw new Error(
      `Invalid <!-- risor ... --> JSON directive at line ${startIdx + 1}: ${err.message}`,
    );
  }
}

function buildEnv(meta, loc) {
  const opts = {};
  if (typeof meta.env === "string") {
    opts.varRecord = meta.env;
  } else if (meta.env && typeof meta.env === "object") {
    opts.vars = meta.env;
  }
  if (meta.namedTypes) opts.namedTypes = meta.namedTypes;
  if (meta.groups) opts.groups = meta.groups;
  if (meta.limits) opts.limits = meta.limits;
  if (meta.reject) opts.reject = meta.reject;
  if (meta.functions) {
    opts.functions = {};
    for (const [name, spec] of Object.entries(meta.functions)) {
      assertKnownKeys(spec, KNOWN_FUNCTION_KEYS, `function "${name}"`, loc);
      opts.functions[name] = {
        type: spec.type,
        doc: spec.doc,
        call(_handle, ..._args) {
          if (spec.error !== undefined) {
            throw new HostError(spec.error);
          }
          return spec.returns;
        },
      };
    }
  }
  return createEnv(opts);
}

function assertDiagnosticError(err, expectedPhase, expectedCode, meta, loc) {
  if (!(err instanceof RisorError)) {
    throw new Error(`${loc}: expected RisorError (${expectedPhase}/${expectedCode}), got ${err}`);
  }
  const diag = err.diagnostics[0];
  if (!diag) {
    throw new Error(`${loc}: RisorError had no diagnostics`);
  }
  if (diag.phase !== expectedPhase) {
    throw new Error(
      `${loc}: expected phase "${expectedPhase}" (${expectedCode}), got phase "${diag.phase}" (${diag.code}): ${diag.message}`,
    );
  }
  if (diag.code !== expectedCode) {
    throw new Error(
      `${loc}: expected ${expectedPhase} error "${expectedCode}", got "${diag.code}": ${diag.message}`,
    );
  }
  if (meta.messageContains && !diag.message.includes(meta.messageContains)) {
    throw new Error(
      `${loc}: expected diagnostic message to contain ${JSON.stringify(meta.messageContains)}, got ${JSON.stringify(diag.message)}`,
    );
  }
}

function assertValueEquals(actual, expected, loc) {
  const actualJson = jsonText(actual);
  const expectedJson = jsonText(expected);
  if (actualJson !== expectedJson) {
    throw new Error(`${loc}: expected value ${expectedJson}, got ${actualJson}`);
  }
}

function assertEntries(program, expectedEntries, loc) {
  const actual = program.entries();
  if (actual.length !== expectedEntries.length) {
    throw new Error(
      `${loc}: expected ${expectedEntries.length} entries, got ${actual.length} (${actual.map((e) => e.name).join(", ")})`,
    );
  }
  for (let i = 0; i < expectedEntries.length; i++) {
    const exp = expectedEntries[i];
    assertKnownKeys(exp, KNOWN_ENTRY_KEYS, `entry #${i + 1}`, loc);
    const act = actual[i];
    if (act.name !== exp.name) {
      throw new Error(`${loc}: entry ${i} expected name "${exp.name}", got "${act.name}"`);
    }
    if ("doc" in exp && act.doc !== exp.doc) {
      throw new Error(
        `${loc}: entry "${act.name}" expected doc ${JSON.stringify(exp.doc)}, got ${JSON.stringify(act.doc)}`,
      );
    }
    if ("inputType" in exp) {
      const got = act.inputType ? act.inputType.toString() : null;
      if (got !== exp.inputType) {
        throw new Error(
          `${loc}: entry "${act.name}" expected inputType ${JSON.stringify(exp.inputType)}, got ${JSON.stringify(got)}`,
        );
      }
    }
    if ("resultType" in exp) {
      const got = act.resultType ? act.resultType.toString() : null;
      if (got !== exp.resultType) {
        throw new Error(
          `${loc}: entry "${act.name}" expected resultType ${JSON.stringify(exp.resultType)}, got ${JSON.stringify(got)}`,
        );
      }
    }
    if (exp.params) {
      if (act.params.length !== exp.params.length) {
        throw new Error(
          `${loc}: entry "${act.name}" expected ${exp.params.length} params, got ${act.params.length}`,
        );
      }
      for (let j = 0; j < exp.params.length; j++) {
        const ep = exp.params[j];
        assertKnownKeys(ep, KNOWN_PARAM_KEYS, `entry "${act.name}" param #${j + 1}`, loc);
        const ap = act.params[j];
        if ("name" in ep && ap.name !== ep.name) {
          throw new Error(
            `${loc}: entry "${act.name}" param ${j} expected name ${JSON.stringify(ep.name)}, got ${JSON.stringify(ap.name)}`,
          );
        }
        if ("type" in ep && ap.type.toString() !== ep.type) {
          throw new Error(
            `${loc}: entry "${act.name}" param ${j} expected type ${JSON.stringify(ep.type)}, got ${JSON.stringify(ap.type.toString())}`,
          );
        }
        if ("doc" in ep && ap.doc !== ep.doc) {
          throw new Error(
            `${loc}: entry "${act.name}" param ${j} expected doc ${JSON.stringify(ep.doc)}, got ${JSON.stringify(ap.doc)}`,
          );
        }
        if ("optional" in ep && ap.optional !== ep.optional) {
          throw new Error(
            `${loc}: entry "${act.name}" param ${j} expected optional=${ep.optional}, got ${ap.optional}`,
          );
        }
      }
    }
  }
}

/**
 * Verify every fenced code block in a Markdown guide.
 * Returns a summary object `{ ok, total, checked, errors }`.
 */
export function checkGuideMarkdown(markdown, fileLabel = "docs/guide.md") {
  const blocks = extractGuideBlocks(markdown);
  const errors = [];
  let checked = 0;
  let lastDiagnostic = null;

  for (const block of blocks) {
    const loc = `${fileLabel}:${block.line} (block #${block.index}, "${block.heading}")`;

    try {
      const meta = block.meta;
      if (meta) {
        assertKnownKeys(meta, KNOWN_DIRECTIVE_KEYS, "directive", loc);
      }

      // Verify diagnostic output blocks against the immediately preceding failing example.
      if (meta && meta.matchesPreviousDiagnostic) {
        if (!lastDiagnostic) {
          throw new Error(`${loc}: matchesPreviousDiagnostic has no preceding failing example`);
        }
        const mode = meta.matchesPreviousDiagnostic;
        let rendered;
        if (mode === "detail") {
          rendered = renderDetail(lastDiagnostic.diagnostics, lastDiagnostic.source).trimEnd();
        } else if (mode === "line") {
          rendered = renderLine(lastDiagnostic.diagnostics).trimEnd();
        } else {
          throw new Error(`${loc}: unknown matchesPreviousDiagnostic mode "${mode}"`);
        }
        if (block.code !== rendered) {
          throw new Error(
            `${loc}: diagnostic output does not match rendered ${mode}:\n--- expected ---\n${rendered}\n--- actual ---\n${block.code}`,
          );
        }
        checked++;
        continue;
      }

      // Every fenced block must be either a Risor program block or carry a directive.
      const isRisorBlock = RISOR_LANGS.has(block.lang);
      const isTemplateBlock = Boolean(meta && meta.template);
      if (!isRisorBlock && !isTemplateBlock) {
        throw new Error(
          `${loc}: unverified fenced block (lang="${block.lang}"). Every block in the guide must be verified.`,
        );
      }

      const cfg = meta ?? {};
      const env = buildEnv(cfg, loc);

      // 1. Parse
      let expr;
      try {
        if (cfg.template) {
          const tpl = parseTemplate(block.code);
          expr = tpl.program(cfg.template);
        } else {
          expr = parse(block.code);
        }
      } catch (err) {
        if (cfg.parseError) {
          assertDiagnosticError(err, "parse", cfg.parseError, cfg, loc);
          lastDiagnostic = { diagnostics: err.diagnostics, source: block.code };
          checked++;
          continue;
        }
        throw new Error(`${loc}: unexpected parse error: ${err.message}`);
      }
      if (cfg.parseError) {
        throw new Error(`${loc}: expected parse error "${cfg.parseError}", but code parsed`);
      }

      // 2. Compile & Check
      let program;
      try {
        program = env.compile(expr, {
          check: cfg.check !== false,
          expected: cfg.expected,
        });
      } catch (err) {
        if (cfg.checkError) {
          assertDiagnosticError(err, "check", cfg.checkError, cfg, loc);
          lastDiagnostic = { diagnostics: err.diagnostics, source: block.code };
          checked++;
          continue;
        }
        throw new Error(`${loc}: unexpected compile/check error: ${err.message}`);
      }
      if (cfg.checkError) {
        throw new Error(`${loc}: expected check error "${cfg.checkError}", but code checked`);
      }

      if (cfg.type !== undefined) {
        const actualType = program.checkResult?.type.toString() ?? null;
        if (actualType !== cfg.type) {
          throw new Error(`${loc}: expected type "${cfg.type}", got "${actualType}"`);
        }
      }

      if (cfg.entries) {
        assertEntries(program, cfg.entries, loc);
      }

      // 3. Run (program result, single entry call, or multiple entry calls)
      if (cfg.calls) {
        let callDiagnostic = null;
        for (let c = 0; c < cfg.calls.length; c++) {
          const callSpec = cfg.calls[c];
          const callLoc = `${loc} [call #${c + 1} ${callSpec.entry}]`;
          assertKnownKeys(callSpec, KNOWN_CALL_KEYS, "call", callLoc);
          const callVars = callSpec.vars ?? cfg.vars;
          let value;
          try {
            if ("input" in callSpec) {
              value = program.callRecord(callSpec.entry, callSpec.input, callVars);
            } else {
              value = program.call(callSpec.entry, callSpec.args ?? [], callVars);
            }
          } catch (err) {
            if (callSpec.runError) {
              assertDiagnosticError(err, "run", callSpec.runError, callSpec, callLoc);
              callDiagnostic = { diagnostics: err.diagnostics, source: block.code };
              continue;
            }
            throw new Error(`${callLoc}: unexpected run error: ${err.message}`);
          }
          if (callSpec.runError) {
            throw new Error(
              `${callLoc}: expected run error "${callSpec.runError}", but got ${jsonText(value)}`,
            );
          }
          if ("expect" in callSpec) {
            assertValueEquals(value, callSpec.expect, callLoc);
          }
          callDiagnostic = null;
        }
        lastDiagnostic = callDiagnostic;
        checked++;
        continue;
      }

      if (cfg.entry) {
        let value;
        try {
          if ("input" in cfg) {
            value = program.callRecord(cfg.entry, cfg.input, cfg.vars);
          } else {
            value = program.call(cfg.entry, cfg.args ?? [], cfg.vars);
          }
        } catch (err) {
          if (cfg.runError) {
            assertDiagnosticError(err, "run", cfg.runError, cfg, loc);
            lastDiagnostic = { diagnostics: err.diagnostics, source: block.code };
            checked++;
            continue;
          }
          throw new Error(`${loc}: unexpected entry run error: ${err.message}`);
        }
        if (cfg.runError) {
          throw new Error(
            `${loc}: expected run error "${cfg.runError}", but got ${jsonText(value)}`,
          );
        }
        if ("expect" in cfg) {
          assertValueEquals(value, cfg.expect, loc);
        }
        lastDiagnostic = null;
        checked++;
        continue;
      }

      if (!program.hasResult) {
        if (!cfg.entries) {
          throw new Error(
            `${loc}: result-less program must assert "entries", "entry", or "calls"`,
          );
        }
        lastDiagnostic = null;
        checked++;
        continue;
      }

      let value;
      try {
        value = program.run(cfg.vars);
      } catch (err) {
        if (cfg.runError) {
          assertDiagnosticError(err, "run", cfg.runError, cfg, loc);
          lastDiagnostic = { diagnostics: err.diagnostics, source: block.code };
          checked++;
          continue;
        }
        throw new Error(`${loc}: unexpected run error: ${err.message}`);
      }
      if (cfg.runError) {
        throw new Error(
          `${loc}: expected run error "${cfg.runError}", but got ${jsonText(value)}`,
        );
      }
      if ("expect" in cfg) {
        assertValueEquals(value, cfg.expect, loc);
      }

      lastDiagnostic = null;
      checked++;
    } catch (err) {
      errors.push(err.message);
    }
  }

  return {
    ok: errors.length === 0 && blocks.length > 0,
    total: blocks.length,
    checked,
    errors,
  };
}

export function checkGuideFile(guidePath = DEFAULT_GUIDE_PATH) {
  const markdown = readFileSync(guidePath, "utf8");
  return checkGuideMarkdown(markdown, guidePath);
}

function main(argv) {
  const args = argv.slice(2);
  if (args.length > 1) {
    console.error("Usage: node docs/scripts/check-guide-examples.mjs [path/to/guide.md]");
    process.exit(2);
  }
  const guidePath = args[0] ? resolve(args[0]) : DEFAULT_GUIDE_PATH;
  const res = checkGuideFile(guidePath);
  if (!res.ok) {
    console.error(
      `Guide example check failed (${res.errors.length} failure(s) out of ${res.total} block(s)):`,
    );
    for (const err of res.errors) {
      console.error(`  - ${err}`);
    }
    process.exit(1);
  }
  console.log(`All ${res.checked} code blocks in ${guidePath} passed.`);
}

if (fileURLToPath(import.meta.url) === resolve(process.argv[1] || "")) {
  main(process.argv);
}
