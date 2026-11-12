import { describe, test } from "node:test";
import assert from "node:assert/strict";
import { CompletionContext } from "@codemirror/autocomplete";
import { EditorState } from "@codemirror/state";
import { describeSample, localService } from "@risor/service";
import { colourDecorations, isEmptyText, risor, risorCompletions, risorSession, singleLineFilter, toDiagnostics, validateRisor } from "../src/index.ts";
import type { EditorView } from "@codemirror/view";
import type { RisorService } from "@risor/service";

/** Each decorated range as `class:text`. */
function marks(text: string): string[] {
  const out: string[] = [];
  colourDecorations(text).between(0, text.length, (from, to, d) => {
    out.push(`${String(d.spec.class).replace("risor-t-", "")}:${text.slice(from, to)}`);
  });
  return out;
}

describe("colours", () => {
  test("marks from the lexer, at UTF-16 offsets", () => {
    assert.deepEqual(marks(`const s = "😀"; // hi\nlen(s) > 1`), [
      "keyword:const", "operator:=", 'string:"😀"', "comment:// hi", "function:len", "operator:>", "number:1",
    ]);
  });
});

describe("toDiagnostics", () => {
  test("keeps each range inside the document", () => {
    assert.deepEqual(
      toDiagnostics([{ phase: "check", code: "x", message: "m", from: 2, to: 99 }, { phase: "service", code: "y", message: "n" }], 5),
      [
        { from: 2, to: 5, severity: "error", message: "m", source: "x" },
        { from: 0, to: 0, severity: "error", message: "n", source: "y" },
      ],
    );
  });
});

describe("single-line mode", () => {
  test("a newline typed or pasted becomes a space", () => {
    const s = EditorState.create({ doc: "a", extensions: [singleLineFilter] });
    const t = s.update({ changes: { from: 1, insert: "\n+\r\nb" } });
    assert.equal(t.state.doc.toString(), "a + b");
  });
});

describe("completion", () => {
  const service = localService();
  const state = (doc: string) =>
    EditorState.create({
      doc,
      extensions: [risor({ service, environment: describeSample({ order: { total: 1.5, items: [] } }), completion: true })],
    });

  test("the fields of a record after a dot", async () => {
    const doc = `"😀" + order.`;
    const s = state(doc);
    assert.ok(s.facet(risorSession));
    const r = await risorCompletions(new CompletionContext(s, doc.length, false));
    assert.ok(r);
    const labels = r.options.map((o) => o.label);
    assert.ok(labels.includes("items") && labels.includes("total"), labels.join());
    assert.equal(r.from, doc.length);
  });

  test("names while typing a word, and nothing in white space unless asked", async () => {
    const s = state("ord");
    const r = await risorCompletions(new CompletionContext(s, 3, false));
    assert.ok(r?.options.some((o) => o.label === "order" && o.type === "variable"));
    const s2 = state("1 + ");
    assert.equal(await risorCompletions(new CompletionContext(s2, 4, false)), null);
  });
});

describe("isEmptyText", () => {
  test("white space and comments only", () => {
    assert.equal(isEmptyText(""), true);
    assert.equal(isEmptyText("  \n // a note\n /* b */"), true);
    assert.equal(isEmptyText("x // a note"), false);
  });
});

describe("validateRisor", () => {
  test("a text changed while the check is in flight is checked again", async () => {
    const real = localService();
    let view: { state: EditorState; dispatch(spec: unknown): void };
    let first = true;
    // A service whose first validate edits the text before it answers, as a user typing would.
    const service: RisorService = {
      ...real,
      environment(d) {
        const env = real.environment(d);
        return {
          ...env,
          async validate(text, options) {
            if (first) {
              first = false;
              view.dispatch({ changes: { from: 0, to: view.state.doc.length, insert: "a +" } });
            }
            return env.validate(text, options);
          },
        };
      },
    };
    view = {
      state: EditorState.create({ doc: "a + 1", extensions: risor({ service, environment: describeSample({ a: 1 }) }) }),
      dispatch(spec) {
        this.state = this.state.update(spec as never).state;
      },
    };
    const problems = await validateRisor(view as unknown as EditorView);
    assert.equal(view.state.doc.toString(), "a +");
    assert.equal(problems.length, 1); // the problems of "a +", not of "a + 1"
  });
});
