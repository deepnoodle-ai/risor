// The messages between the worker client and the worker script.

import type { Description } from "@deepnoodle/risor";
import type { CheckOptions, RunOptions } from "./types.ts";

export type Request =
  | { id: number; op: "open"; env: number; description: Description | undefined }
  | { id: number; op: "close"; env: number }
  | { id: number; op: "check" | "validate"; env: number; text: string; options?: CheckOptions }
  | { id: number; op: "complete" | "hover"; env: number; text: string; offset: number; options?: CheckOptions }
  | { id: number; op: "run"; env: number; text: string; vars?: Record<string, unknown>; options?: RunOptions }
  | { id: number; op: "format"; text: string };

export type Response = { id: number; result: unknown } | { id: number; error: string };
