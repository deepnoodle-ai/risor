import assert from "node:assert/strict";
import { parse } from "../src/syntax/parser.ts";
import type { Program } from "../src/syntax/ast.ts";
import { RisorError } from "../src/host/api.ts";
import type { Diagnostic, Env, Expected, Expr, Site } from "../src/host/api.ts";

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

/** A check whose failure is data: the type as text and the sites, or the diagnostics. */
export type Checked = { ok: true; type: string; sites: Site[] } | { ok: false; diagnostics: readonly Diagnostic[] };

export function tryCheck(env: Env, x: Expr, expected?: Expected): Checked {
  try {
    const r = env.check(x, expected);
    return { ok: true, type: r.type.toString(), sites: r.sites };
  } catch (e) {
    if (e instanceof RisorError) return { ok: false, diagnostics: e.diagnostics };
    throw e;
  }
}
