// The service on the main thread. It loads all of Risor into the page and a
// run blocks the page until its limits end it, so it is for Node, tests, and
// pages whose programs are small. A browser page should use workerService.

import type { Description } from "@deepnoodle/risor";
import { Engine } from "./engine.ts";
import type { CheckOptions, RisorEnvironment, RisorService, RunOptions } from "./types.ts";

export function localService(): RisorService {
  const engine = new Engine();
  let next = 1;
  return {
    environment(description?: Description): RisorEnvironment {
      const id = next++;
      let open = false;
      // Open again after a release: React's StrictMode releases in an
      // effect's cleanup and then uses the same object.
      const ready = () => {
        if (!open) engine.open(id, description);
        open = true;
        return id;
      };
      ready();
      return {
        check: async (text: string, options?: CheckOptions) => engine.check(ready(), text, options),
        validate: async (text: string, options?: CheckOptions) => engine.check(ready(), text, options),
        complete: async (text: string, offset: number, options?: CheckOptions) => engine.complete(ready(), text, offset, options),
        hover: async (text: string, offset: number, options?: CheckOptions) => engine.hover(ready(), text, offset, options),
        run: async (text: string, vars?: Record<string, unknown>, options?: RunOptions) => engine.run(ready(), text, vars, options),
        release: () => {
          if (open) engine.close(id);
          open = false;
        },
      };
    },
    format: async (text: string) => engine.format(text),
    dispose() {},
  };
}
