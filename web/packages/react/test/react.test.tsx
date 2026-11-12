import { describe, test } from "node:test";
import assert from "node:assert/strict";
import { renderToString } from "react-dom/server";
import { localService } from "@deepnoodle/risor-service";
import { RisorEditor, RisorProvider, RisorResult, RisorValue } from "../src/index.ts";

describe("server rendering", () => {
  test("the editor renders its text in a <pre> before it mounts", () => {
    const html = renderToString(
      <RisorProvider service={localService()}>
        <RisorEditor value={"order.total > 100"} singleLine />
      </RisorProvider>,
    );
    assert.match(html, /<pre class="risor-editor-fallback"[^>]*>order.total &gt; 100<\/pre>/);
  });

  test("a forced colour scheme names its class; auto names none", () => {
    const render = (colorScheme?: "light" | "dark" | "auto") =>
      renderToString(
        <RisorProvider service={localService()}>
          <RisorEditor value="1" colorScheme={colorScheme} />
        </RisorProvider>,
      ).match(/class="(risor-editor[^"]*)"/)![1];
    assert.equal(render("dark"), "risor-editor risor-editor-dark");
    assert.equal(render("light"), "risor-editor risor-editor-light");
    assert.equal(render("auto"), "risor-editor");
    assert.equal(render(), "risor-editor");
  });

  test("a result: the value as a tree, its type and its usage", () => {
    const html = renderToString(<RisorResult result={{ ok: true, json: `{"a":[1,2]}`, type: "{a: int[]}", usage: { steps: 3, allocated: 4 }, ms: 0.2 }} />);
    assert.match(html, /risor-v-key/);
    assert.match(html, /\{a: int\[\]\}/);
    assert.match(html, /3 steps/);
  });

  test("a large value renders one page of a list, and no closed branch", () => {
    const json = JSON.stringify({ big: Array.from({ length: 1000 }, (_, i) => i), deep: { a: { b: "needle" } } });
    const html = renderToString(<RisorValue json={json} />);
    assert.equal((html.match(/risor-v-number/g) ?? []).length, 200);
    assert.match(html, /Show 200 more of 800/);
    assert.doesNotMatch(html, /needle/); // below openDepth: closed, so not rendered
    assert.doesNotMatch(html, /role="tree/);
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
