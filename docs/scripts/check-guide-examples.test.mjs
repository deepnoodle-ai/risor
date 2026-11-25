import assert from "node:assert/strict";
import test from "node:test";

import {
  DEFAULT_GUIDE_PATH,
  checkGuideFile,
  checkGuideMarkdown,
  extractGuideBlocks,
} from "./check-guide-examples.mjs";

test("docs/guide.md passes every fenced code block", () => {
  const res = checkGuideFile(DEFAULT_GUIDE_PATH);
  assert.deepEqual(res.errors, []);
  assert.equal(res.ok, true);
  assert.ok(res.checked >= 40, `expected at least 40 checked blocks, got ${res.checked}`);
  assert.equal(res.checked, res.total);
});

test("extractGuideBlocks parses single-line and multi-line directives", () => {
  const md = [
    "# Title",
    "",
    '<!-- risor {"expect": 3} -->',
    "```js",
    "1 + 2",
    "```",
    "",
    "<!-- risor",
    "{",
    '  "template": "string",',
    '  "env": "{name: string}",',
    '  "vars": {"name": "Ada"},',
    '  "expect": "Hi Ada"',
    "}",
    "-->",
    "```text",
    "Hi ${{ name }}",
    "```",
  ].join("\n");

  const blocks = extractGuideBlocks(md);
  assert.equal(blocks.length, 2);
  assert.equal(blocks[0].meta.expect, 3);
  assert.equal(blocks[1].meta.template, "string");

  const res = checkGuideMarkdown(md, "inline.md");
  assert.equal(res.ok, true);
  assert.equal(res.checked, 2);
});

test("checkGuideMarkdown catches wrong expected values, wrong error codes, and unverified blocks", () => {
  const badValue = [
    '<!-- risor {"expect": 4} -->',
    "```js",
    "1 + 2",
    "```",
  ].join("\n");
  const r1 = checkGuideMarkdown(badValue, "bad-value.md");
  assert.equal(r1.ok, false);
  assert.match(r1.errors[0], /expected value 4, got 3/);

  const badErr = [
    '<!-- risor {"checkError": "no-field"} -->',
    "```js",
    "1 + 2",
    "```",
  ].join("\n");
  const r2 = checkGuideMarkdown(badErr, "bad-err.md");
  assert.equal(r2.ok, false);
  assert.match(r2.errors[0], /expected check error "no-field"/);

  const unverified = [
    "```text",
    "unverified text block",
    "```",
  ].join("\n");
  const r3 = checkGuideMarkdown(unverified, "unverified.md");
  assert.equal(r3.ok, false);
  assert.match(r3.errors[0], /unverified fenced block/);
});

test("checkGuideMarkdown verifies diagnostic output blocks against the previous error", () => {
  const md = [
    '<!-- risor {"env": "{xs: int[]}", "checkError": "kind", "messageContains": "use len(...)"} -->',
    "```js",
    "xs.length",
    "```",
    "",
    '<!-- risor {"matchesPreviousDiagnostic": "detail"} -->',
    "```text",
    "error[kind] 1:1-1:3: a value of type int[] has no fields to read; use len(...)",
    "    xs.length",
    "    ^^",
    "```",
  ].join("\n");

  const res = checkGuideMarkdown(md, "diag.md");
  assert.deepEqual(res.errors, []);
  assert.equal(res.ok, true);
  assert.equal(res.checked, 2);
});

test("checkGuideMarkdown rejects unknown directive and call keys and continues checking", () => {
  const md = [
    '<!-- risor {"expcet": 3} -->',
    "```js",
    "1 + 2",
    "```",
    "",
    '<!-- risor {"expect": 3} -->',
    "```js",
    "1 + 2",
    "```",
    "",
    '<!-- risor {"calls": [{"entry": "double", "args": [2], "expcet": 4}]} -->',
    "```js",
    "export const double = (n: int): int => n * 2;",
    "```",
  ].join("\n");

  const res = checkGuideMarkdown(md, "typo.md");
  assert.equal(res.ok, false);
  assert.equal(res.total, 3);
  assert.equal(res.checked, 1);
  assert.equal(res.errors.length, 2);
  assert.match(res.errors[0], /unknown directive key\(s\): expcet/);
  assert.match(res.errors[1], /unknown call key\(s\): expcet/);
});

test("checkGuideMarkdown clears lastDiagnostic after a passing calls block", () => {
  const md = [
    '<!-- risor {"env": "{xs: int[]}", "checkError": "kind"} -->',
    "```js",
    "xs.length",
    "```",
    "",
    '<!-- risor {"calls": [{"entry": "double", "args": [3], "expect": 6}]} -->',
    "```js",
    "export const double = (n: int): int => n * 2;",
    "```",
    "",
    '<!-- risor {"matchesPreviousDiagnostic": "detail"} -->',
    "```text",
    "error[kind] 1:1-1:3: a value of type int[] has no fields to read; use len(...)",
    "    xs.length",
    "    ^^",
    "```",
  ].join("\n");

  const res = checkGuideMarkdown(md, "stale-diag.md");
  assert.equal(res.ok, false);
  assert.equal(res.checked, 2);
  assert.equal(res.errors.length, 1);
  assert.match(res.errors[0], /has no preceding failing example/);
});
