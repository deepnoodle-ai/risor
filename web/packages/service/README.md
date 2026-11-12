# @risor/service

Check, complete, hover, format and run [Risor](https://github.com/deepnoodle-ai/risor)
off the page's main thread. Framework-free: `@risor/codemirror` is built on it,
and `@risor/react` (#633) and `@risor/mantine` (#634) follow.

> Risor v3 is an alpha. This API will change before 3.0.

```sh
npm install @risor/service
```

```ts
import { describeSample, workerService } from "@risor/service";

const service = workerService();             // one web worker, shared
const env = service.environment(describeSample({ order: { total: 120.5 } }));

await env.check("order.totl > 100");         // [{ code: "no-field", from: 6, to: 10, ... }]
await env.complete("order.", 6);              // { from: 6, to: 6, items: [{ label: "total", ... }] }
await env.run("order.total > 100", { order: { total: 120.5 } });
// { ok: true, json: "true", type: "bool", usage: { steps: 2, allocated: 0 }, ms: 0.2 }
```

## The environment is a description

Every call is made against an environment, given as a **description**: the
JSON that Go's `env.Describe()` or TypeScript's `createEnv(...).describe()`
makes. A Go server can send its description to the page, so the browser
checks text against exactly the variables, functions and types the server
will use.

| You have | Pass |
| --- | --- |
| A Go server with the real environment | `env.Describe()`, sent to the page |
| Options in TypeScript | `createEnv(options).describe()` |
| Sample data only | `describeSample(sample, { groups })` |
| Nothing | no description: the empty environment |

A description that declares a host function checks, completes and hovers,
but cannot run in the browser: the function exists only on the host. `run`
then returns one problem, `host-functions`. Run such a program on your server
instead, and return the same `RunResult` shape.

## Behaviour

- **Offsets are UTF-16**, as the DOM and CodeMirror count. `line` and
  `column` in a problem count code points, as Risor's messages do.
- **A run that does not end** within its deadline (`timeoutMs`, default
  2,000 ms) plus 1,500 ms is stopped: the worker is ended and a new one
  starts. The result has the stage `stopped`.
- **`check`, `complete` and `hover` drop stale calls**: a call made while an
  earlier one of the same kind waits replaces it, and the earlier one
  resolves `null`. `validate` is never replaced; await it on a form's submit.
- **The worker** is loaded with
  `new Worker(new URL("./worker.js", import.meta.url), { type: "module" })`,
  which Vite, webpack 5 and Rollup understand. If your bundler does not,
  build `@risor/service/worker` yourself and pass
  `workerService({ worker: () => new Worker(yourUrl, { type: "module" }) })`.
- `localService()` runs on the main thread, for Node and tests.

## RunResult

```ts
type RunResult =
  | { ok: true; json: string; type: string | null; usage: { steps: number; allocated: number }; ms: number }
  | { ok: false; stage: "declare" | "compile" | "run" | "stopped"; problems: Problem[]; usage?: Usage; ms?: number };
```

`json` is Risor's JSON text of the value. `parseJsonTree(json)` gives a tree
for a view.

License: Apache-2.0
