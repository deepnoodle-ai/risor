// 03 3.9.1: a binding never answers a program within the limits with a stack
// overflow. @deepnoodle/risor-service runs programs in a Web Worker, whose
// stack a page cannot change and which is smaller than the main thread's. The
// forms of ts/test/fixtures/deep-forms.ts spend the most stack for each unit
// of the depth count: their calls go through a library entry that calls a
// lambda. Each runs at depth limit 400 in a worker, and again with 1.5 times
// headroom: under 200 levels of itself, so that the stack in use is that of
// a program 600 deep.

import { expect, test } from "@playwright/test";
import { FORMS, VALUES, VARS, deepSource } from "../../../../ts/test/fixtures/deep-forms.ts";
import type { DeepCase, DeepRequest, StackedCase } from "./deep-protocol.ts";

const DEPTH = 400;
const expected = Object.fromEntries(FORMS.map((f) => [f.name, JSON.stringify(f.expect)]));

test("the deepest programs run in a Web Worker at depth limit 400, with 1.5 times headroom", async ({ page }) => {
  await page.goto("/e2e/deep.html");
  await page.waitForFunction(() => window.risorDeep !== undefined);

  const direct: DeepRequest<DeepCase> = {
    vars: VARS,
    values: VALUES,
    depth: DEPTH,
    cases: FORMS.map((f) => ({ name: f.name, source: deepSource(f, DEPTH) })),
  };
  expect(await page.evaluate((req) => window.risorDeep.service(req), direct)).toEqual(expected);

  const stacked: DeepRequest<StackedCase> = {
    vars: VARS,
    values: VALUES,
    depth: DEPTH,
    cases: FORMS.map((f) => ({ name: f.name, outer: deepSource(f, DEPTH / 2, "inner"), inner: deepSource(f, DEPTH) })),
  };
  expect(await page.evaluate((req) => window.risorDeep.stacked(req), stacked)).toEqual(expected);
});
