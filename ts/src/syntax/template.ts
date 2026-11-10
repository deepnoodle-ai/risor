// Package F2: templates, host texts with `${{ }}` parts (f02-templates-analysis.md 2).

import type * as A from "./ast.ts";
import { ParseError } from "./diagnostic.ts";
import type { Diagnostic } from "./diagnostic.ts";
import { lexPart } from "./lexer.ts";
import { parsePartTokens } from "./parser.ts";
import { Source, fromCodePoints } from "./source.ts";
import type { Span } from "./source.ts";
import type { Token } from "./token.ts";

export type TemplateMode = "string" | "value";

/** A segment of a template (f02 2.4). */
export type Segment =
  | { kind: "text"; value: string; span: Span }
  | { kind: "part"; expr: A.Expr; span: Span };

export interface ParsedTemplateData {
  source: Source;
  segments: Segment[];
  /** Exactly one part, with no text run before or after it. */
  single: boolean;
}

export type TemplateResult = { ok: true; template: ParsedTemplateData } | { ok: false; diagnostics: Diagnostic[]; source: Source };

const DOLLAR = 0x24;
const LBRACE = 0x7b;
const RBRACE = 0x7d;

/** Template parse (f02 2.1 to 2.3). Never throws for any text. */
export function parseTemplateText(text: string): TemplateResult {
  const source = new Source(text, { keepBom: true });
  try {
    return { ok: true, template: scan(source) };
  } catch (e) {
    if (e instanceof ParseError) return { ok: false, diagnostics: [e.diagnostic], source };
    throw e;
  }
}

function scan(source: Source): ParsedTemplateData {
  const cps = source.cps;
  // Read the whole text first, every part's tokens included (2.3 item 3).
  type Raw = { kind: "text"; cps: number[]; start: number; end: number } | { kind: "part"; tokens: Token[]; start: number; end: number };
  const raws: Raw[] = [];
  let text: number[] = [];
  let textStart = 0;
  let i = 0;
  const flush = (end: number) => {
    if (end > textStart) raws.push({ kind: "text", cps: text, start: textStart, end });
    text = [];
  };
  const at = (k: number) => (k < cps.length ? cps[k] : -1);
  while (i < cps.length) {
    if (at(i) === DOLLAR && at(i + 1) === DOLLAR && at(i + 2) === LBRACE && at(i + 3) === LBRACE) {
      text.push(DOLLAR, LBRACE, LBRACE);
      i += 4;
      continue;
    }
    if (at(i) === DOLLAR && at(i + 1) === LBRACE && at(i + 2) === LBRACE) {
      flush(i);
      const tokens = lexPart(source, i + 3, i);
      const close = tokens[tokens.length - 1];
      let end: number;
      if (at(close.start + 1) === RBRACE) {
        close.end = close.start + 2; // the closing delimiter `}}`
        end = close.end;
      } else {
        close.text = "lone"; // 2.2 item 5: unexpected-token where the parser meets it
        end = close.start + 1;
      }
      raws.push({ kind: "part", tokens, start: i, end });
      i = end;
      textStart = i;
      continue;
    }
    text.push(cps[i]);
    i++;
  }
  flush(cps.length);
  // Then parse the parts in order.
  const segments: Segment[] = raws.map((r) =>
    r.kind === "text"
      ? { kind: "text", value: fromCodePoints(r.cps), span: { start: r.start, end: r.end } }
      : { kind: "part", expr: parsePartTokens(r.tokens, source), span: { start: r.start, end: r.end } },
  );
  return { source, segments, single: segments.length === 1 && segments[0].kind === "part" };
}

/** The program of a template in a mode (f02 2.5). */
export function templateProgram(t: ParsedTemplateData, mode: TemplateMode): A.Program {
  if (mode === "value" && t.single) {
    const e = (t.segments[0] as { expr: A.Expr }).expr;
    return { kind: "Program", consts: [], result: e, span: { start: e.span.start, end: e.span.end } };
  }
  const span = { start: 0, end: t.source.length };
  let result: A.Expr;
  if (t.segments.some((s) => s.kind === "part")) {
    result = { kind: "Template", parts: t.segments.map((s) => (s.kind === "text" ? s.value : s.expr)), form: "${{", span };
  } else {
    result = { kind: "String", value: t.segments.map((s) => (s as { value: string }).value).join(""), form: "${{", span };
  }
  return { kind: "Program", consts: [], result, span };
}
