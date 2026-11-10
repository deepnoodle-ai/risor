import { test } from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import { generated } from "../scripts/generate.ts";

const dir = join(dirname(fileURLToPath(import.meta.url)), "..", "src", "generated");

test("src/generated is current with suite/messages.json and suite/library.json (npm run generate)", () => {
  for (const [name, text] of Object.entries(generated())) {
    assert.equal(readFileSync(join(dir, name), "utf8"), text, name);
  }
});
