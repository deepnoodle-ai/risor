// The source names its worker as ./worker.ts, so a bundler that reads the
// source (the risor-source condition) finds it. The built file names
// ./worker.js. tsc rewrites imports, not URLs, so this does.

import { readFileSync, writeFileSync } from "node:fs";

const file = process.argv[2];
const text = readFileSync(file, "utf8");
const out = text.replace('new URL("./worker.ts", import.meta.url)', 'new URL("./worker.js", import.meta.url)');
if (out === text) {
  console.error(`worker-url: no worker URL in ${file}`);
  process.exit(1);
}
writeFileSync(file, out);
