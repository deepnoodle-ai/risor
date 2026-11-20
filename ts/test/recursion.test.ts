// Package F17 step E2, recursive named types: what the suite has no case for.
//
// The conformance function's depth (f17 12.7 item 3) is no conform result,
// so it has no case (f17 13 item 6). And a comparison that reaches the bound
// of 4,096 pairs (12.5 item 7) must reach it with no deep native stack: each
// part of fixtures/recursion-run.ts runs in a process of its own, with the
// stack of test/deep.test.ts, 656 KB.

import { describe, test } from "node:test";
import assert from "node:assert/strict";
import { execFile } from "node:child_process";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import { conform, conformAll, createEnv, RisorError, typeFromSchema } from "../src/host/api.ts";

const here = dirname(fileURLToPath(import.meta.url));

function part(name: string): Promise<void> {
  return new Promise((resolve, reject) => {
    execFile(process.execPath, ["--stack-size=656", join(here, "fixtures", "recursion-run.ts"), name], { encoding: "utf8" }, (err, stdout, stderr) => {
      try {
        assert.equal(err, null, stderr.slice(0, 4000));
        assert.equal(stdout, "ok\n");
        resolve();
      } catch (e) {
        reject(e);
      }
    });
  });
}

describe("long chains of recursive types need no deep stack (f17 12.5 item 7)", { concurrency: 4 }, () => {
  test("type lines whose comparison reaches the bound", () => part("type lines"));
  test("an environment's named types whose comparison reaches the bound", () => part("named types"));
  test("as follows the data to the run's depth limit", () => part("as"));
  test("the conformance function on data 100,000 levels deep", () => part("conform"));
});

/** A list nested `levels` deep around 1: [[...[1]...]]. */
function lists(levels: number): unknown {
  let v: unknown = 1;
  for (let i = 0; i < levels; i++) v = [v];
  return v;
}

describe("the conformance function's depth (f17 12.7 item 3)", () => {
  const env = createEnv({ namedTypes: [{ name: "Nest", type: "Nest[] | int" }] });
  const t = env.parseType("Nest");

  test("200 levels conform", () => {
    assert.equal(conform(lists(200), t), null);
    assert.deepEqual(conformAll(lists(200), t, 5), []);
  });

  test("past 200 levels: depth-exceeded, class limit, no path and no span", () => {
    const d = conform(lists(201), t);
    assert.ok(d !== null);
    assert.equal(d.phase, "run");
    assert.equal(d.code, "depth-exceeded");
    assert.equal(d.class, "limit");
    assert.equal(d.path, undefined);
    assert.equal(d.span, undefined);
    assert.equal(d.message, "calls and nested values went deeper than the depth limit of 200");
  });

  test("ConformAll ends with the depth failure", () => {
    // A failure before the deep part, then the depth.
    const v = [["x"], lists(300)];
    const ds = conformAll(v, t, 10);
    assert.deepEqual(ds.map((d) => d.code), ["not-conforming", "depth-exceeded"]);
    assert.equal(ds[1].path, undefined);
  });

  test("a type that bounds the depth is not walked past it", () => {
    const flat = createEnv({ namedTypes: [{ name: "Flat", type: "int[][]" }] }).parseType("Flat");
    assert.equal(conform(lists(300), flat)?.code, "not-conforming");
  });
});

describe("recursive types of an environment (f17 12.4)", () => {
  test("a named type may name a later one", () => {
    const env = createEnv({
      namedTypes: [
        { name: "Dir", type: "{files: File[], dirs: Dir[]}" },
        { name: "File", type: "{name: string, parent: Dir | null}" },
      ],
    });
    assert.equal(env.parseType("Dir").toString(), "Dir");
    assert.deepEqual(env.parseType("Dir").fields().map((f) => `${f.key}: ${f.type}`), ["dirs: Dir[]", "files: File[]"]);
    assert.equal(env.parseType("Dir").kind(), "record");
  });

  test("a cycle with no guard is bad-type", () => {
    assert.throws(() => createEnv({ namedTypes: [{ name: "Loop", type: "Loop | null" }] }), (e: unknown) => e instanceof RisorError && e.diagnostics[0].code === "bad-type");
  });

  test("the description declares its types again as one list", () => {
    const env = createEnv({ namedTypes: [{ name: "Tree", type: "{kids: Tree[]}" }], vars: { t: "Tree" } });
    const d = env.describe();
    assert.deepEqual(d.types, { Tree: "{kids: Tree[]}" });
  });

  test("two recursive types of one name and one structure are equal", () => {
    const schema = { $defs: { node: { type: "array", items: { $ref: "#/$defs/node" } } }, $ref: "#/$defs/node" };
    const a = typeFromSchema(schema);
    const b = typeFromSchema(schema);
    assert.equal(a.toString(), "Node");
    assert.equal(a.elem()?.toString(), "Node");
    assert.ok(a.equal(b));
    assert.ok(a.fits(b));
    const other = typeFromSchema({ $defs: { node: { type: "object", properties: { n: { $ref: "#/$defs/node" } } } }, $ref: "#/$defs/node" });
    assert.ok(!a.equal(other));
  });
});
