// One part of test/flat.test.ts, in a process of its own: `node flat-run.ts PART`.
// PART is the name of a form of flat-forms.ts, which then goes through every
// entry point of the binding with 100,000 links; or `formatted text`,
// `diagnostics` or `runner`. It prints `ok`, or it throws.

import assert from "node:assert/strict";
import { RisorError, createEnv, execute, format, formatTemplate, parse, parseTemplate, renderDetail, renderJSON, renderLine } from "../../src/host/api.ts";
import { treeEqual } from "../../src/tools/format.ts";
import { tokens } from "../../src/tools/tokens.ts";
import { caseFile } from "../../src/testing.ts";
import { FORMS } from "./flat-forms.ts";
import type { FlatForm } from "./flat-forms.ts";

/** The links of each program. */
const LINKS = 100000;

/** The repeats that give a form at least this many links. */
const repeats = (f: FlatForm, links: number) => Math.ceil(links / (f.per ?? 1));

const envOf = (f: FlatForm) => createEnv({ varRecord: f.env, limits: { steps: null, allocation: null } });

/** The position after the last code point of a text. */
function end(text: string): { line: number; column: number } {
  const lines = text.split("\n");
  return { line: lines.length, column: [...lines[lines.length - 1]].length + 1 };
}

const WORD = /[A-Za-z0-9_]/;

/** The last word of a text: a position in it, and its span. Hover there reads the word's token (f12 3.13). */
function lastWord(text: string): { at: { line: number; column: number }; span: { start: { line: number; column: number }; end: { line: number; column: number } } } {
  const cps = [...text];
  let i = cps.length - 1;
  while (i > 0 && !WORD.test(cps[i])) i--;
  let j = i;
  while (j > 0 && WORD.test(cps[j - 1])) j--;
  const lineStart = cps.lastIndexOf("\n", i) + 1;
  let line = 1;
  for (let k = 0; k < lineStart; k++) if (cps[k] === "\n") line++;
  const at = (k: number) => ({ line, column: k - lineStart + 1 });
  return { at: at(i), span: { start: at(j), end: at(i + 1) } };
}

/** Run an entry point. A RangeError is the stack, or a text too long to make: it names the entry point. */
function through<T>(what: string, f: () => T): T {
  try {
    return f();
  } catch (e) {
    if (e instanceof RangeError) assert.fail(`${what}: ${e.name}: ${e.message}`);
    throw e;
  }
}

/** The code of the RisorError an entry point throws. */
function codeOf(what: string, f: () => unknown): string {
  try {
    through(what, f);
  } catch (e) {
    if (e instanceof RisorError) return e.diagnostics[0].code;
    throw e;
  }
  return assert.fail(`${what}: no diagnostic`);
}

/** Compile and run, and compare the value, the steps and the units with the form's. */
function run(f: FlatForm, n: number, source: string, check: boolean, what: string): void {
  const env = envOf(f);
  const vars = f.vars();
  const o = through(what, () => execute(env.compile(parse(source), { check }), vars));
  assert.ok(o.ok, `${what}: ${o.ok ? "" : o.diagnostic.code}`);
  assert.ok(f.gives(o.value, vars, n), `${what}: the value`);
  assert.equal(o.usage.steps, f.steps(n), `${what}: the steps`);
  assert.equal(o.usage.allocated, f.units(n), `${what}: the units`);
}

/** The laws of the formatter (f12 2.2): the same tree, and a text the formatter leaves as it is. */
function formatLaws(source: string, what: string): string {
  const text = through(what, () => format(source));
  assert.equal(through(`${what}, again`, () => format(text)), text, `${what}: formatted twice`);
  assert.ok(through(`${what}, the trees`, () => treeEqual(parse(source).program, parse(text).program)), `${what}: the same tree`);
  return text;
}

/** One form through every entry point. */
function everyEntryPoint(f: FlatForm): void {
  const n = repeats(f, LINKS);
  const source = f.source(n);
  const env = envOf(f);
  const small = f.source(repeats(f, 3 * (f.per ?? 1)));

  // Parse, the static rules and the checker.
  const expr = through("parse", () => parse(source));
  const past = f.past?.(n) ?? null;
  if (past === null) assert.equal(through("check", () => String(env.check(expr).type)), f.type, "check: the type");
  else {
    // Past a limit of the checker: the check says so, and one repeat fewer is checked and run.
    assert.equal(codeOf("check", () => env.check(expr)), past.code);
    assert.equal(f.past!(n - 1), null);
    assert.equal(String(env.check(parse(f.source(n - 1))).type), f.type);
    run(f, n - 1, f.source(n - 1), true, "compile and run, one repeat fewer");
  }

  // Compile and run: with the check, and with the static rules alone.
  if (past === null) run(f, n, source, true, "compile and run");
  else assert.equal(codeOf("compile", () => env.compile(expr)), past.code);
  if (past === null || !past.alone) run(f, n, source, false, "compile and run with no check");
  else {
    assert.equal(codeOf("compile with no check", () => env.compile(expr, { check: false })), past.code);
    run(f, n - 1, f.source(n - 1), false, "compile and run with no check, one repeat fewer");
  }

  // The analysis: names, paths and calls.
  JSON.parse(through("analyze", () => expr.analyze().json()));

  // The tokens.
  assert.ok(through("tokens", () => tokens(source)).length > 0);

  // Completion and hover. A program of three repeats gives the same candidates and the same record.
  through("complete at the end", () => env.complete(source, end(source)));
  assert.deepEqual(
    through("complete after a dot", () => env.complete(source + ".", end(source + "."))).items,
    env.complete(small + ".", end(small + ".")).items,
    "complete after a dot",
  );
  const first = through("hover on the first token", () => env.hover(source, { line: 1, column: 1 }));
  assert.deepEqual(first, env.hover(small, { line: 1, column: 1 }), "hover on the first token");
  // On the last word: the record of three repeats, with the span of that word.
  const word = lastWord(source);
  const atEnd = through("hover on the last word", () => env.hover(source, word.at));
  const want = env.hover(small, lastWord(small).at);
  if (want === null) assert.equal(atEnd, null, "hover on the last word");
  else {
    assert.ok(atEnd !== null, "hover on the last word");
    assert.deepEqual(atEnd.span, word.span, "hover on the last word: the span");
    assert.deepEqual({ ...atEnd, span: null, declaration: null }, { ...want, span: null, declaration: null }, "hover on the last word");
  }
  const last = end(source);
  through("hover in the middle", () => env.hover(source, { line: last.line, column: Math.max(1, last.column >> 1) }));

  // The formatter.
  if (f.format === "text") formatLaws(source, "format");
  else if (f.format === "nested") {
    // f12 2.6 writes parentheses for the reader around each left operand here: the text of
    // more than 100 of them nests too deep to parse, so the laws are tested at 40 repeats.
    assert.equal(typeof through("format", () => format(source)), "string");
    formatLaws(f.source(40), "format, 40 repeats");
  } else {
    // f12 2.8 indents each `else` branch two more than the one before, so the text grows
    // with the square of the length. 2,000 branches are 8 million code points; 100,000
    // would be 20,000 million, which is more than a string can hold.
    formatLaws(f.source(2000), "format, 2,000 repeats");
  }

  // A template whose one part is the program. A part holds no `const` line and no type line.
  if (!/^(const|type) /.test(source)) {
    const text = "${{ " + source + " }}";
    const t = through("parse a template", () => parseTemplate(text));
    if (past === null) {
      const vars = f.vars();
      const o = through("run a template", () => execute(env.compile(t.program("value")), vars));
      assert.ok(o.ok && f.gives(o.value, vars, n), "run a template: the value");
    }
    JSON.parse(through("analyze a template", () => t.analyze("value").json()));
    if (f.format !== "quadratic") assert.equal(typeof through("format a template", () => formatTemplate(text)), "string");
  }
}

/** The text of the formatter for a long chain, written out. */
function formattedText(): void {
  const n = LINKS;
  // A run of `as` is one line: it holds no break (f04 8).
  assert.equal(format("y" + " as any".repeat(n)), "y" + " as any".repeat(n) + "\n");
  // A chain of calls that does not fit breaks before each call (f12 2.11).
  assert.equal(format("y" + ".trim()".repeat(n)), "y" + "\n  .trim()".repeat(n) + "\n");
  // A projection is a plain step: it stays with the call before it.
  assert.equal(format("[[y]]" + "[*].map(x => x)".repeat(n)), "[[y]][*]" + "\n  .map(x => x)[*]".repeat(n - 1) + "\n  .map(x => x)\n");
  // A run of one level breaks before each operand, and a comment stays with its token.
  assert.equal(format("1" + " + 1".repeat(n)), "1 +" + "\n  1 +".repeat(n - 1) + "\n  1\n");
  assert.equal(format("1" + " /* c */ + 1".repeat(n)), "1 /* c */ +" + "\n  1 /* c */ +".repeat(n - 1) + "\n  1\n");
  // A leading comment begins a line, and the last one stays before its token (f12 2.12 item 6).
  assert.equal(format("/* c */ ".repeat(n) + "1"), "/* c */\n".repeat(n - 1) + "/* c */ 1\n");
}

/** A diagnostic at the end of a long flat program, in each phase. */
function diagnostics(): void {
  const n = LINKS;
  const m: Record<string, unknown> = {};
  m.a = m;
  const env = createEnv({ varRecord: "{y: string, m: any, c: bool}", limits: { steps: null, allocation: null } });
  // The phase, the code, the program, and the columns of the diagnostic's span in a program of l code points.
  const failing: [string, string, string, (l: number) => [number, number]][] = [
    // The end of the text.
    ["parse", "unexpected-token", "y" + ".trim()".repeat(n) + " +", (l) => [l + 1, l + 1]],
    ["parse", "bad-type-text", "y" + " as any".repeat(n) + " as", (l) => [l + 1, l + 1]],
    // `nope`.
    ["check", "no-method", "y" + ".trim()".repeat(n) + ".nope()", (l) => [l - 5, l - 1]],
    // The last `[true]`, the left operand of `+`.
    ["check", "kind", "c" + " as any in [true]".repeat(n) + " + 1", (l) => [l - 9, l - 3]],
    // The read that fails, from `m` to `.zz`.
    ["run", "missing-field", "m" + ".a".repeat(n) + ".zz.q", (l) => [1, l - 1]],
    // The whole chain.
    ["run", "not-conforming", "y" + " as any".repeat(n) + " as int", (l) => [1, l + 1]],
  ];
  for (const [phase, code, source, span] of failing) {
    let thrown: unknown;
    try {
      through(`${phase} ${code}`, () => env.compile(parse(source)).run({ y: "a", m, c: true }));
    } catch (e) {
      thrown = e;
    }
    assert.ok(thrown instanceof RisorError, `${phase} ${code}: ${String(thrown).slice(0, 200)}`);
    assert.equal(thrown.phase, phase);
    assert.equal(thrown.diagnostics[0].code, code);
    const [from, to] = span(source.length);
    assert.deepEqual(thrown.diagnostics[0].span, { start: { line: 1, column: from }, end: { line: 1, column: to } }, `${code}: the span`);
    // The three renderings (f08 5).
    for (const render of [() => renderLine(thrown.diagnostics), () => renderDetail(thrown.diagnostics, source), () => renderJSON(thrown.diagnostics)]) {
      assert.equal(typeof through(`render ${code}`, render), "string");
    }
    // The tools on a text that does not parse or does not check.
    through(`complete ${code}`, () => env.complete(source, end(source)));
    through(`hover ${code}`, () => env.hover(source, { line: 1, column: 1 }));
    through(`tokens ${code}`, () => tokens(source));
    if (phase !== "parse") formatLaws(source, `format ${code}`);
  }
}

// The three programs of the report that this file answers, with the outcomes it gives for 100,000 links.
const REPORTED = [
  { name: "as", source: "y" + " as any".repeat(LINKS), type: "any", value: "a", steps: 0, units: 0 },
  { name: "projection", source: "[[y]]" + "[*].map(x => x)".repeat(LINKS), type: "string[][]", value: [["a"]], steps: 5 * LINKS, units: 4 * LINKS + 4 },
  { name: "trim", source: "y" + ".trim()".repeat(LINKS), type: "string", value: "a", steps: LINKS, units: LINKS },
];

/** The runner of case files (package F9) on long flat cases. */
function runner(): void {
  const cases: Record<string, unknown>[] = [];
  for (const c of REPORTED) {
    cases.push({ name: c.name, source: c.source, env: "{y: string}", vars: { y: "a" }, expect: { value: c.value, steps: c.steps, allocated: c.units } });
  }
  cases.push({ name: "format as", format: REPORTED[0].source, expect: { formatted: REPORTED[0].source + "\n" } });
  cases.push({ name: "format trim", format: REPORTED[2].source, expect: { formatted: "y" + "\n  .trim()".repeat(LINKS) + "\n" } });
  const good = caseFile({ cases }, { update: false });
  for (const c of good.cases) {
    const v = through(`case ${c.name}`, () => c.run());
    assert.equal(v.status, "pass", `${c.name}: ${v.report.slice(0, 2000)}`);
  }
  // A case that fails: the report of the difference is made, and it is not a RangeError.
  const bad = caseFile({ cases: [{ name: "wrong", source: REPORTED[2].source, env: "{y: string}", vars: { y: "a" }, expect: { value: "b", steps: 1 } }] }, { update: false });
  const v = through("a failed case", () => bad.cases[0].run());
  assert.equal(v.status, "fail");
  assert.deepEqual(v.differences.map((d) => d.name), ["value", "steps"]);
}

const part = process.argv[2];
const form = FORMS.find((f) => f.name === part);
if (form !== undefined) everyEntryPoint(form);
else if (part === "formatted text") formattedText();
else if (part === "diagnostics") diagnostics();
else if (part === "runner") runner();
else throw new Error(`no part ${part}`);
process.stdout.write("ok\n");
