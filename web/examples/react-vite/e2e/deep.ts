// The page of deep.spec.ts: it runs the deepest programs in Web Workers, where
// a page cannot change the stack (src/index.ts of @deepnoodle/risor, STACK).

import { createEnv } from "@deepnoodle/risor";
import { workerService } from "@deepnoodle/risor-service";
import type { DeepCase, DeepRequest, DeepResults, StackedCase } from "./deep-protocol.ts";

const service = workerService();

window.risorDeep = {
  async service(req: DeepRequest<DeepCase>): Promise<DeepResults> {
    const description = createEnv({ varRecord: req.vars, limits: { depth: req.depth, steps: null, allocation: null } }).describe();
    const env = service.environment(description);
    const out: DeepResults = {};
    for (const c of req.cases) {
      const r = await env.run(c.source, req.values, { check: false, timeoutMs: 60_000 });
      out[c.name] = r.ok ? r.json : r.problems.map((p) => p.code).join(", ");
    }
    env.release();
    return out;
  },

  stacked(req: DeepRequest<StackedCase>): Promise<DeepResults> {
    return new Promise((resolve, reject) => {
      const w = new Worker(new URL("./deep-worker.ts", import.meta.url), { type: "module" });
      w.onmessage = (e: MessageEvent<DeepResults>) => {
        w.terminate();
        resolve(e.data);
      };
      w.onerror = (e) => {
        w.terminate();
        reject(new Error(e.message));
      };
      w.postMessage(req);
    });
  },
};
