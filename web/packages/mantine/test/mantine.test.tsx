import { describe, test } from "node:test";
import assert from "node:assert/strict";
import { renderToString } from "react-dom/server";
import { MantineProvider } from "@mantine/core";
import { localService } from "@risor/service";
import { RisorInput, RisorPreview, RisorProvider } from "../src/index.ts";

describe("server rendering", () => {
  test("the input: label, description, error, and the text", () => {
    const html = renderToString(
      <MantineProvider>
        <RisorProvider service={localService()}>
          <RisorInput id="cond" label="Run when" description="A Risor expression" error="Wrong" value="a > 1" />
        </RisorProvider>
      </MantineProvider>,
    );
    assert.match(html, /Run when/);
    assert.match(html, /A Risor expression/);
    assert.match(html, /Wrong/);
    assert.match(html, /a &gt; 1/);
    assert.match(html, /id="cond-label"/);
  });

  test("the preview renders before its first run", () => {
    const html = renderToString(
      <MantineProvider>
        <RisorProvider service={localService()}>
          <RisorPreview code="1 + 1" />
        </RisorProvider>
      </MantineProvider>,
    );
    assert.match(html, /Preview/);
  });
});
