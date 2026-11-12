// The main-thread cost of a Risor field (PRD UC-1): RisorInput bundled
// without React, Mantine and CodeMirror, which the page has anyway. The rest
// of Risor loads in the worker. Fails above the limit.
//
//   node scripts/size.ts

import { build } from "esbuild";
import { gzipSync } from "node:zlib";

const LIMIT = 20 * 1024;

const r = await build({
  stdin: { contents: 'export { RisorInput } from "@risor/mantine";', resolveDir: import.meta.dirname, loader: "js" },
  bundle: true,
  minify: true,
  format: "esm",
  write: false,
  conditions: ["risor-source"],
  external: ["react", "react-dom", "react/jsx-runtime", "@mantine/*", "@codemirror/*"],
  logLevel: "error",
});
const bytes = gzipSync(r.outputFiles[0].contents).length;
console.log(`RisorInput on the main thread: ${(bytes / 1024).toFixed(1)} KB gzipped (limit ${LIMIT / 1024} KB)`);
if (bytes > LIMIT) process.exit(1);
