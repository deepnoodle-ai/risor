// The worker script. workerService() starts it; a host whose bundler cannot
// load a worker from a dependency builds this file itself and passes
// `workerService({ worker: () => new Worker(...) })`.

import { Engine } from "./engine.ts";
import type { Request, Response } from "./protocol.ts";

interface WorkerScope {
  onmessage: ((e: MessageEvent<Request>) => void) | null;
  postMessage(message: Response): void;
}

const scope = globalThis as unknown as WorkerScope;
const engine = new Engine();

/** Answer one request. Exported for tests, which have no worker. */
export function answer(m: Request): Response {
  try {
    switch (m.op) {
      case "open":
        engine.open(m.env, m.description);
        return { id: m.id, result: null };
      case "close":
        engine.close(m.env);
        return { id: m.id, result: null };
      case "check":
      case "validate":
        return { id: m.id, result: engine.check(m.env, m.text, m.options) };
      case "complete":
        return { id: m.id, result: engine.complete(m.env, m.text, m.offset, m.options) };
      case "hover":
        return { id: m.id, result: engine.hover(m.env, m.text, m.offset, m.options) };
      case "run":
        return { id: m.id, result: engine.run(m.env, m.text, m.vars, m.options) };
      case "format":
        return { id: m.id, result: engine.format(m.text) };
    }
  } catch (e) {
    return { id: m.id, error: String((e as Error)?.message ?? e) };
  }
}

if (typeof scope.postMessage === "function" && typeof (globalThis as { document?: unknown }).document === "undefined") {
  scope.onmessage = (e) => scope.postMessage(answer(e.data));
}
