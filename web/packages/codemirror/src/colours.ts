// Colours from Risor's own lexer (risor/tokens), as mark decorations. Not a
// Lezer grammar: the colours always agree with the lexer the conformance
// suite tests. The whole text is lexed on each change; Risor texts are
// expressions and configs, and 20 KB lexes in about 3 ms.

import { RangeSetBuilder } from "@codemirror/state";
import type { Extension } from "@codemirror/state";
import { Decoration, EditorView, ViewPlugin } from "@codemirror/view";
import type { DecorationSet, ViewUpdate } from "@codemirror/view";
import { tokens } from "risor/tokens";
import type { TokenClass } from "risor/tokens";
import { TextIndex } from "@risor/service";

const CLASSES: TokenClass[] = [
  "keyword", "constant", "name", "function", "property", "number", "string",
  "template", "interpolation", "operator", "punctuation", "comment", "invalid",
];

const MARKS = new Map(CLASSES.map((c) => [c, Decoration.mark({ class: `risor-t-${c}` })]));

/** The colour decorations of a text. */
export function colourDecorations(text: string): DecorationSet {
  const index = new TextIndex(text);
  const b = new RangeSetBuilder<Decoration>();
  for (const t of tokens(text)) {
    if (t.class === "name" || t.class === "punctuation") continue;
    b.add(index.unitOf(t.start), index.unitOf(t.end), MARKS.get(t.class)!);
  }
  return b.finish();
}

export const colours: Extension = ViewPlugin.fromClass(
  class {
    decorations: DecorationSet;
    constructor(view: EditorView) {
      this.decorations = colourDecorations(view.state.doc.toString());
    }
    update(u: ViewUpdate) {
      if (u.docChanged) this.decorations = colourDecorations(u.state.doc.toString());
    }
  },
  { decorations: (p) => p.decorations },
);

/**
 * The colours, as CSS variables with light and dark defaults. A page sets
 * `--risor-keyword` and the others on any parent to change them.
 */
export const risorTheme: Extension = EditorView.baseTheme({
  ".risor-t-keyword": { color: "var(--risor-keyword, #8250df)" },
  ".risor-t-constant": { color: "var(--risor-constant, #0550ae)" },
  ".risor-t-function": { color: "var(--risor-function, #6639ba)" },
  ".risor-t-property": { color: "var(--risor-property, #0a3069)" },
  ".risor-t-number": { color: "var(--risor-number, #0550ae)" },
  ".risor-t-string": { color: "var(--risor-string, #0a7a32)" },
  ".risor-t-template": { color: "var(--risor-string, #0a7a32)" },
  ".risor-t-interpolation": { color: "var(--risor-interpolation, #cf222e)" },
  ".risor-t-operator": { color: "var(--risor-operator, #cf222e)" },
  ".risor-t-comment": { color: "var(--risor-comment, #6e7781)", fontStyle: "italic" },
  ".risor-t-invalid": { color: "var(--risor-invalid, #cf222e)", textDecoration: "underline wavy" },
  "&dark .risor-t-keyword": { color: "var(--risor-keyword, #d2a8ff)" },
  "&dark .risor-t-constant": { color: "var(--risor-constant, #79c0ff)" },
  "&dark .risor-t-function": { color: "var(--risor-function, #d2a8ff)" },
  "&dark .risor-t-property": { color: "var(--risor-property, #a5d6ff)" },
  "&dark .risor-t-number": { color: "var(--risor-number, #79c0ff)" },
  "&dark .risor-t-string": { color: "var(--risor-string, #7ee787)" },
  "&dark .risor-t-template": { color: "var(--risor-string, #7ee787)" },
  "&dark .risor-t-interpolation": { color: "var(--risor-interpolation, #ff7b72)" },
  "&dark .risor-t-operator": { color: "var(--risor-operator, #ff7b72)" },
  "&dark .risor-t-comment": { color: "var(--risor-comment, #8b949e)" },
  "&dark .risor-t-invalid": { color: "var(--risor-invalid, #ff7b72)" },
  ".cm-tooltip.risor-hover": { padding: "4px 8px", maxWidth: "480px", fontSize: "0.9em" },
  ".risor-hover code": { fontFamily: "var(--risor-font-mono, ui-monospace, SFMono-Regular, Menlo, monospace)" },
  ".risor-hover-doc": { marginTop: "4px", whiteSpace: "pre-wrap" },
});
