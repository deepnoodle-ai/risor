import { describe, test } from "node:test";
import assert from "node:assert/strict";
import { renderToString } from "react-dom/server";
import { localService } from "@risor/service";
import { RisorEditor, RisorProvider, RisorResult } from "../src/index.ts";

describe("server rendering", () => {
  test("the editor renders its text in a <pre> before it mounts", () => {
    const html = renderToString(
      <RisorProvider service={localService()}>
        <RisorEditor value={"order.total > 100"} singleLine />
      </RisorProvider>,
    );
    assert.match(html, /<pre class="risor-editor-fallback"[^>]*>order.total &gt; 100<\/pre>/);
  });

  test("a result: the value as a tree, its type and its usage", () => {
    const html = renderToString(<RisorResult result={{ ok: true, json: `{"a":[1,2]}`, type: "{a: int[]}", usage: { steps: 3, allocated: 4 }, ms: 0.2 }} />);
    assert.match(html, /risor-v-key/);
    assert.match(html, /\{a: int\[\]\}/);
    assert.match(html, /3 steps/);
  });

  test("a failed run: its problems with a caret under the line", () => {
    const html = renderToString(
      <RisorResult text={"a +"} result={{ ok: false, stage: "compile", problems: [{ phase: "parse", code: "unexpected-end", message: "the text ends", from: 3, to: 3, line: 1, column: 4 }] }} />,
    );
    assert.match(html, /Does not compile/);
    assert.match(html, /unexpected-end/);
    assert.match(html, /a \+\n {3}\^/);
  });
});
