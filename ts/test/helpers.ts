import assert from "node:assert/strict";
import { parse } from "../src/syntax/parser.ts";
import type { Program } from "../src/syntax/ast.ts";

export type LS = [number, number, number, number];

/** The text parses. Returns the program. */
export function ok(src: string): Program {
  const r = parse(src);
  if (!r.ok) {
    const d = r.diagnostics[0];
    assert.fail(`expected ${JSON.stringify(src)} to parse, got ${d.code} at ${JSON.stringify(r.source.lineSpan(d.span!))}: ${d.message}`);
  }
  return r.program;
}

/** The first parse diagnostic has this code, and this span when one is given. */
export function err(src: string, code: string, span?: LS): void {
  const r = parse(src);
  if (r.ok) assert.fail(`expected ${JSON.stringify(src)} to fail with ${code}, it parsed`);
  const d = r.diagnostics[0];
  const got = r.source.lineSpan(d.span!);
  assert.equal(d.code, code, `${JSON.stringify(src)}: code (span ${JSON.stringify(got)}, ${d.message})`);
  if (span !== undefined) assert.deepEqual(got, span, `${JSON.stringify(src)}: span of ${code}`);
}
