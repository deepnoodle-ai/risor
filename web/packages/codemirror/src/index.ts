// @deepnoodle/risor-codemirror: Risor for CodeMirror 6. One extension, `risor(options)`,
// gives colours, errors as you type, completion, hover, format and a run
// key. It works in any framework; @deepnoodle/risor-react (#633) wraps it, and a Vue
// package is planned.
//
// The extension's options do not change after it is made. To change the
// environment or the expected type, put the extension in a Compartment and
// reconfigure it with a new one.

import { autocompletion } from "@codemirror/autocomplete";
import type { Completion as CMCompletion, CompletionContext, CompletionResult } from "@codemirror/autocomplete";
import { linter, setDiagnostics } from "@codemirror/lint";
import type { Diagnostic } from "@codemirror/lint";
import { EditorState, Facet, Prec } from "@codemirror/state";
import type { Extension } from "@codemirror/state";
import { EditorView, hoverTooltip, keymap, ViewPlugin } from "@codemirror/view";
import { workerService } from "@deepnoodle/risor-service";
import { tokens } from "@deepnoodle/risor/tokens";
import type { Description, Problem, RisorEnvironment, RisorService } from "@deepnoodle/risor-service";
import { colours, risorTheme } from "./colours.ts";

export { colourDecorations, risorTheme } from "./colours.ts";

export interface RisorOptions {
  /** The service that checks, completes and hovers. Default: one worker service shared by the page. */
  service?: RisorService;
  /** The environment: a description (Go's Env.Describe(), env.describe(), or describeSample). */
  environment?: Description;
  /** The type the text must have, in type text: "bool", "{id: int}[]". */
  expected?: string;
  /** Keep the text on one line: a newline typed or pasted becomes a space, and Enter runs. */
  singleLine?: boolean;
  /** Called with the problems after each check. */
  onDiagnostics?: (problems: Problem[]) => void;
  /** Called on Mod-Enter, and on Enter in single-line mode. */
  onRun?: () => void;
  /** The pause after typing before a check, in milliseconds. Default 300. */
  lintDelay?: number;
  /** Turn off completion, for a host that configures its own. Default true. */
  completion?: boolean;
}

let shared: RisorService | null = null;

/** The worker service the page shares, made on first use. */
export function sharedService(): RisorService {
  shared ??= workerService();
  return shared;
}

/** What the extension's parts share: the options and the service's environment. */
export class RisorSession {
  readonly options: RisorOptions;
  private env: RisorEnvironment | null = null;
  /** The problems of the last check that was not replaced. */
  last: Problem[] = [];

  constructor(options: RisorOptions) {
    this.options = options;
  }

  get environment(): RisorEnvironment {
    this.env ??= (this.options.service ?? sharedService()).environment(this.options.environment);
    return this.env;
  }

  get service(): RisorService {
    return this.options.service ?? sharedService();
  }

  release(): void {
    this.env?.release();
    this.env = null;
  }
}

/** The session of a state, when it has the risor extension. */
export const risorSession = Facet.define<RisorSession, RisorSession | null>({
  combine: (values) => values[values.length - 1] ?? null,
});

/**
 * Whether a text is empty: white space and comments only. An empty text has
 * no problems here; a field that must not be empty says so itself (for
 * example RisorInput's `required`). An empty program is a parse error to
 * Risor, but a red mark in a field the user has not touched is noise.
 */
export function isEmptyText(text: string): boolean {
  return tokens(text).every((t) => t.class === "comment");
}

/** Problems as CodeMirror diagnostics, each within the document. */
export function toDiagnostics(problems: Problem[], docLength: number): Diagnostic[] {
  return problems.map((p) => {
    const from = Math.max(0, Math.min(p.from ?? 0, docLength));
    const to = Math.max(from, Math.min(p.to ?? from, docLength));
    return { from, to, severity: "error", message: p.message, source: p.code };
  });
}

/** Map a candidate's kind to the icon CodeMirror draws for it. */
function iconOf(kind: string): string {
  switch (kind) {
    case "variable":
    case "name":
      return "variable";
    case "function":
    case "lambda":
      return "function";
    case "method":
      return "method";
    case "field":
    case "key":
      return "property";
    case "keyword":
    case "word":
      return "keyword";
    case "literal":
    case "string":
      return "text";
    case "type":
      return "type";
    case "namespace":
    case "group":
      return "namespace";
    case "constant":
      return "constant";
    default:
      return "text";
  }
}

/** The completion source. Usable in a host's own `autocompletion({ override })`. */
export async function risorCompletions(context: CompletionContext): Promise<CompletionResult | null> {
  const session = context.state.facet(risorSession);
  if (!session) return null;
  const before = context.matchBefore(/[A-Za-z_][A-Za-z0-9_]*$/);
  const afterDot = /\??\.$/.test(context.state.sliceDoc(Math.max(0, context.pos - 2), context.pos));
  if (!context.explicit && !before && !afterDot) return null;
  const text = context.state.doc.toString();
  const r = await session.environment.complete(text, context.pos, { expected: session.options.expected });
  if (!r || context.aborted) return null;
  if (r.items.length === 0 && !context.explicit) return null;
  const options: CMCompletion[] = r.items.map((i) => ({
    label: i.label,
    type: iconOf(i.kind),
    detail: i.detail ?? undefined,
    apply: i.insert,
  }));
  return { from: r.from, to: r.to, options, validFor: /^[A-Za-z0-9_]*$/ };
}

const hover = hoverTooltip(async (view, pos) => {
  const session = view.state.facet(risorSession);
  if (!session) return null;
  const h = await session.environment.hover(view.state.doc.toString(), pos, { expected: session.options.expected });
  // `never` is the type of a name the checker could not resolve: its lint message says more.
  if (!h || ((!h.type || h.type === "never") && !h.signature && !h.doc)) return null;
  return {
    pos: h.from,
    end: h.to,
    above: true,
    create() {
      const dom = document.createElement("div");
      dom.className = "risor-hover";
      const code = document.createElement("code");
      code.textContent = h.signature ?? (h.type === "never" ? "" : h.type) ?? "";
      if (code.textContent) dom.appendChild(code);
      if (h.doc) {
        const doc = document.createElement("div");
        doc.className = "risor-hover-doc";
        doc.textContent = h.doc;
        dom.appendChild(doc);
      }
      return { dom };
    },
  };
});

/** A newline typed or pasted becomes a space. */
export const singleLineFilter: Extension = EditorState.transactionFilter.of((tr) => {
  if (!tr.docChanged || !/\n/.test(tr.newDoc.toString())) return tr;
  const changes: { from: number; to: number; insert: string }[] = [];
  tr.changes.iterChanges((fromA, toA, _fromB, _toB, inserted) => {
    changes.push({ from: fromA, to: toA, insert: inserted.toString().replace(/\r?\n/g, " ") });
  });
  return [{ changes, selection: tr.selection, effects: tr.effects, scrollIntoView: tr.scrollIntoView }];
});

/** Check the text now, skipping the pause, and show its problems. For a form's submit. */
export async function validateRisor(view: EditorView): Promise<Problem[]> {
  const session = view.state.facet(risorSession);
  if (!session) return [];
  // Check again if the text changes while a check is in flight, so the
  // result is always for the text as it is now.
  for (;;) {
    const text = view.state.doc.toString();
    const problems = isEmptyText(text) ? [] : await session.environment.validate(text, { expected: session.options.expected });
    if (view.state.doc.toString() !== text) continue;
    session.last = problems;
    view.dispatch(setDiagnostics(view.state, toDiagnostics(problems, view.state.doc.length)));
    session.options.onDiagnostics?.(problems);
    return problems;
  }
}

/** Format the text. Returns false when it does not parse; its problems then show. */
export async function formatRisor(view: EditorView): Promise<boolean> {
  const session = view.state.facet(risorSession);
  if (!session) return false;
  const text = view.state.doc.toString();
  const r = await session.service.format(text);
  if (view.state.doc.toString() !== text) return false;
  if (!r.ok) {
    view.dispatch(setDiagnostics(view.state, toDiagnostics(r.problems, view.state.doc.length)));
    return false;
  }
  const out = session.options.singleLine ? r.text.replace(/\n$/, "") : r.text;
  if (out !== text) view.dispatch({ changes: { from: 0, to: text.length, insert: out } });
  return true;
}

/** Risor in a CodeMirror editor. */
export function risor(options: RisorOptions = {}): Extension {
  const session = new RisorSession(options);
  const lifecycle = ViewPlugin.define(() => ({ destroy: () => session.release() }));
  const run = () => {
    options.onRun?.();
    return options.onRun !== undefined;
  };
  const keys = keymap.of([
    { key: "Mod-Enter", run },
    { key: "Shift-Alt-f", run: (view) => (void formatRisor(view), true) },
    ...(options.singleLine ? [{ key: "Enter", run: () => (options.onRun?.(), true) }] : []),
  ]);
  const lint = linter(
    async (view) => {
      const text = view.state.doc.toString();
      if (isEmptyText(text)) {
        session.last = [];
        options.onDiagnostics?.([]);
        return [];
      }
      const problems = await session.environment.check(text, { expected: options.expected });
      if (problems === null) return toDiagnostics(session.last, view.state.doc.length);
      session.last = problems;
      options.onDiagnostics?.(problems);
      return toDiagnostics(problems, view.state.doc.length);
    },
    { delay: options.lintDelay ?? 300 },
  );
  return [
    risorSession.of(session),
    lifecycle,
    colours,
    risorTheme,
    lint,
    hover,
    Prec.high(keys),
    options.completion === false ? [] : autocompletion({ override: [risorCompletions] }),
    options.singleLine ? singleLineFilter : [],
  ];
}
