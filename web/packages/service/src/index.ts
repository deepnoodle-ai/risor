// @deepnoodle/risor-service: check, complete, hover, format and run Risor, in a web
// worker (workerService) or on the main thread (localService). See
// docs/design/web-packages.md in the risor repository.
//
// Nothing in this entry loads Risor itself except localService: a page that
// uses workerService loads Risor only in the worker.

export type {
  CheckOptions, Completion, CompletionItem, Description, FormatResult, Hover, Problem,
  RisorEnvironment, RisorService, RunOptions, RunResult, Usage,
} from "./types.ts";
export { workerService } from "./client.ts";
export type { WorkerLike, WorkerServiceOptions } from "./client.ts";
export { localService } from "./local.ts";
export { describeSample, typeOfJSON, isName } from "./sample.ts";
export { parseJsonTree, prettyJSON } from "./json.ts";
export type { JsonNode } from "./json.ts";
export { TextIndex } from "./positions.ts";
export { DEFAULT_TIMEOUT_MS } from "./defaults.ts";
