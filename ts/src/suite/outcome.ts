// The outcome of a case as one line of JSON (suite/README.md): what the
// differential fuzzer compares between the three implementations.

import { numberToText } from "../value/number.ts";
import { jsonString, mapGet, mapKeys, plainEqual, readHost, sortKeys, BAD_NUMBER } from "../value/value.ts";
import type { Case } from "./casefile.ts";
import { CLASSIFIER, OpaqueRef, analysisOf, assistOutcome, executeCase, renderCaseDiagnostics, runConform, runnerResult } from "./runner.ts";
import { formatProgram, formatTemplate } from "../tools/format.ts";
import { Source } from "../syntax/source.ts";
import { renderDetail, renderJSON, renderLine } from "../host/render.ts";
import { descriptionText } from "../host/api.ts";

/** A result as JSON (03 3.2.4), an opaque value as a case file writes it. */
export function valueJSON(v: unknown, depth = 0): string {
  if (depth > 1100) return "null";
  v = readHost(v);
  if (v === BAD_NUMBER) return "null";
  if (v instanceof OpaqueRef) return `{"$opaque":{"type":${jsonString(v.type)},"id":${v.id}}}`;
  switch (CLASSIFIER.kind(v)) {
    case "null":
      return "null";
    case "bool":
      return String(v);
    case "number":
      return numberToText(Number(v));
    case "string":
      return jsonString(v as string);
    case "list":
      return "[" + (v as unknown[]).map((x) => valueJSON(x, depth + 1)).join(",") + "]";
    case "map":
      return "{" + sortKeys(mapKeys(v as object)).map((k) => jsonString(k) + ":" + valueJSON(mapGet(v as object, k), depth + 1)).join(",") + "}";
    default:
      return '{"$opaque":null}';
  }
}

/** The line of suite/README.md for one case. Every `expect` is ignored. */
export function outcomeLine(c: Case): string {
  // f02 4.8 item 2: an analysis case has name, phase and analysis.
  if (c.expect.kind === "result" && c.expect.analysis !== undefined) {
    const a = analysisOf(c);
    if (!a.ok) {
      const d = a.diagnostics[0];
      return `{"name":${jsonString(c.name)},"phase":"parse","diagnostics":[{"code":${jsonString(d.code)},"span":${JSON.stringify(a.source.lineSpan(d.span!))}}]}`;
    }
    return `{"name":${jsonString(c.name)},"phase":null,"analysis":${a.report.json()}}`;
  }
  // f04 9.5: a conform case.
  if (c.conform !== undefined) return conformLine(c);
  // f12 7.7: a format, completion or hover case.
  if (c.tool !== undefined) {
    const name = `"name":${jsonString(c.name)}`;
    const t = c.tool;
    if (t.kind === "format" || t.kind === "formatTemplate") {
      const r = t.kind === "format" ? formatProgram(t.text) : formatTemplate(t.text);
      if (r.ok) return `{${name},"formatted":${jsonString(r.text)}}`;
      const src = t.kind === "format" ? new Source(t.text) : new Source(t.text, { keepBom: true });
      const d = r.diagnostics[0];
      return `{${name},"phase":"parse","diagnostics":[{"code":${jsonString(d.code)},"span":${JSON.stringify(src.lineSpan(d.span!))},"message":${jsonString(d.message)}}]}`;
    }
    const r = assistOutcome(c);
    if (!r.ok) return `{${name},"phase":"declare","diagnostics":[${r.diagnostics.map((d) => d.code).sort().map((code) => `{"code":${jsonString(code)}}`).join(",")}]}`;
    const str = (x: string | null) => (x === null ? "null" : jsonString(x));
    if (t.kind === "complete") {
      const cpl = r.completion!;
      const items = cpl.items.map((i) => `{"label":${jsonString(i.label)},"kind":${jsonString(i.kind)},"detail":${str(i.detail)},"insert":${jsonString(i.insert)}}`);
      return `{${name},"completion":{"span":${JSON.stringify(cpl.span)},"items":[${items.join(",")}]}}`;
    }
    const h = r.hover ?? null;
    if (h === null) return `{${name},"hover":null}`;
    return `{${name},"hover":{"kind":${jsonString(h.kind)},"span":${JSON.stringify(h.span)},"type":${str(h.type)},"signature":${str(h.signature)},"doc":${str(h.doc)},"declaration":${h.declaration === null ? "null" : JSON.stringify(h.declaration)}}}`;
  }
  // f09 9.4: a runner case gives name, invalid or verdicts, report, and text in update mode.
  if (c.runner !== undefined) {
    const r = runnerResult(c);
    if (r.invalid) return `{"name":${jsonString(c.name)},"invalid":true}`;
    const entries = r.verdicts.map((e) => {
      let line = `{"name":${jsonString(e.name)},"status":${jsonString(e.status)}`;
      if (e.differs !== undefined) line += `,"differs":${JSON.stringify(e.differs.map(String))}`;
      if (e.fixture !== undefined) line += `,"fixture":${JSON.stringify(e.fixture)},"message":${jsonString(e.message ?? "")}`;
      return line + "}";
    });
    let line = `{"name":${jsonString(c.name)},"verdicts":[${entries.join(",")}],"report":${jsonString(r.report)}`;
    if (c.update === true) line += `,"text":${jsonString(r.text ?? "")}`;
    return line + "}";
  }
  // f08 8: a render case gives the three forms.
  if (c.render !== undefined) {
    const ds = renderCaseDiagnostics(c);
    const src = c.render.source ?? null;
    return `{"name":${jsonString(c.name)},"line":${jsonString(renderLine(ds))},"detail":${jsonString(renderDetail(ds, src))},"json":${jsonString(renderJSON(ds, CLASSIFIER))}}`;
  }
  const e = executeCase(c, true, c.check !== false);
  const o = e.outcome;
  const parts: string[] = [`"name":${jsonString(c.name)}`];
  // f01 7.4: the description, when the case has schemas and its environment was built.
  if ((c.schemas !== undefined || c.types !== undefined || c.facts !== undefined) && e.env !== null) parts.push(`"description":${descriptionText(e.env.describe())}`);
  // When the program checks: its type and sites.
  const ti = o as { type?: string | null; sites?: { class: string; span: number[] }[] };
  if (ti.type !== undefined && ti.type !== null) {
    parts.push(`"type":${jsonString(ti.type)}`);
    parts.push(`"sites":[${(ti.sites ?? []).map((x) => `{"class":${jsonString(x.class)},"span":${JSON.stringify(x.span)}}`).join(",")}]`);
  }
  if (o.kind === "result") {
    parts.push(`"phase":null`);
    if (o.ran) parts.push(`"value":${valueJSON(o.value)}`);
  } else {
    parts.push(`"phase":${jsonString(o.phase)}`);
    const span = (d: { span?: { start: number; end: number } }) => (d.span !== undefined && o.source !== undefined ? JSON.stringify(o.source.lineSpan(d.span)) : "null");
    let diags: string[];
    switch (o.phase) {
      case "parse":
        // f08 8: the first diagnostic with its message.
        diags = [o.diagnostics[0]].map((d) => `{"code":${jsonString(d.code)},"span":${span(d)},"message":${jsonString(d.message)}}`);
        break;
      case "declare":
        diags = o.diagnostics.map((d) => d.code).sort().map((code) => `{"code":${jsonString(code)}}`);
        break;
      case "check":
        diags = o.diagnostics.map((d) => `{"code":${jsonString(d.code)},"span":${span(d)},"message":${jsonString(d.message)}}`);
        break;
      default:
        // f08 8: a run diagnostic has its trace, but for class limit.
        diags = o.diagnostics.map((d) => {
          // f10 10.4: a load that fails at step 5a has code, span null, class, message and path.
          if (c.load === true && d.span === undefined && d.path !== undefined) {
            return `{"code":${jsonString(d.code)},"span":null,"class":${jsonString(d.class ?? "value")},"message":${jsonString(d.message)},"path":${JSON.stringify(d.path)}}`;
          }
          const trace = d.class === "limit" ? "" : `,"trace":[${(d.trace ?? []).map((t) => `{"span":${span(t)},"name":${jsonString(t.name)},"index":${t.index === null ? "null" : t.index}}`).join(",")}]`;
          // R73: a limit failure's span is not specified, and its line has none.
          const sp = d.class === "limit" ? "null" : span(d);
          return `{"code":${jsonString(d.code)},"span":${sp},"class":${d.class === undefined ? "null" : jsonString(d.class)},"message":${jsonString(d.message)}${trace}}`;
        });
    }
    parts.push(`"diagnostics":[${diags.join(",")}]`);
  }
  // suite/README.md item 2: a failure of class `limit` has no usage in its line.
  const limit = o.kind === "failure" && o.diagnostics.some((d) => d.class === "limit");
  if (o.usage !== undefined && !limit) parts.push(`"steps":${o.usage.steps}`, `"units":${o.usage.allocated}`);
  // f02 4.8: the analysis, last, when the text parses.
  const a = analysisOf(c);
  if (a.ok) parts.push(`"analysis":${a.report.json()}`);
  return `{${parts.join(",")}}`;
}

/** The line of a conform case (f04 9.5). */
function conformLine(c: Case): string {
  const r = runConform(c);
  const parts: string[] = [`"name":${jsonString(c.name)}`];
  if (!r.ok) {
    parts.push(`"phase":"declare"`);
    parts.push(`"diagnostics":[${r.diagnostics.map((d) => d.code).sort().map((code) => `{"code":${jsonString(code)}}`).join(",")}]`);
    return `{${parts.join(",")}}`;
  }
  parts.push(`"phase":null`);
  parts.push(`"conforms":${r.fn === null}`);
  if (r.fn !== null) {
    parts.push(`"path":${valueJSON(r.fn.path ?? [])}`);
    parts.push(`"message":${jsonString(r.fn.message)}`);
  }
  parts.push(`"steps":${r.asRun.steps}`);
  const asDiag = r.asRun.ok ? null : r.asRun.diagnostic;
  const fnOk = r.fn === null;
  const asOk = r.asRun.ok && plainEqual(r.asRun.value, r.value, CLASSIFIER);
  const isOk = r.isRun.ok && r.isRun.value === true;
  const agree =
    fnOk === asOk && asOk === isOk && r.isRun.ok && r.asRun.steps === r.isRun.steps &&
    (fnOk || (asDiag !== null && asDiag.code === "not-conforming" && JSON.stringify(asDiag.path) === JSON.stringify(r.fn!.path) && asDiag.message === r.fn!.message));
  parts.push(`"agree":${agree}`);
  // f08 8: the failures of ConformAll, with `max`.
  if (r.all !== null) parts.push(`"failures":[${r.all.map((x) => `{"path":${valueJSON(x.path ?? [])},"message":${jsonString(x.message)}}`).join(",")}]`);
  return `{${parts.join(",")}}`;
}
