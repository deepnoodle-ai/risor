// The worker of deep.spec.ts for stacked cases: the outer program's host
// function `inner` runs the inner program at the outer one's deepest point.

import { createEnv, jsonText, parse, RisorError } from "@deepnoodle/risor";
import type { DeepRequest, DeepResults, StackedCase } from "./deep-protocol.ts";

function codeOf(x: unknown): string {
  return x instanceof RisorError ? x.diagnostics.map((d) => d.code).join(", ") : String(x);
}

self.onmessage = (e: MessageEvent<DeepRequest<StackedCase>>) => {
  const { vars, values, depth, cases } = e.data;
  const limits = { depth, steps: null, allocation: null };
  const out: DeepResults = {};
  for (const c of cases) {
    const inner = createEnv({ varRecord: vars, limits }).compile(parse(c.inner), { check: false });
    let innerFailure: string | null = null;
    const call = () => {
      try {
        return inner.run(values);
      } catch (x) {
        innerFailure = codeOf(x);
        throw x;
      }
    };
    const outer = createEnv({ varRecord: vars, functions: { inner: { type: "() => any", call } }, limits }).compile(parse(c.outer), { check: false });
    try {
      out[c.name] = jsonText(outer.run(values));
    } catch (x) {
      out[c.name] = innerFailure ?? codeOf(x);
    }
  }
  postMessage(out);
};
