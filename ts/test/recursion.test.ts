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
import { conform, conformAll, createEnv, parse, RisorError, typeFromSchema } from "../src/host/api.ts";

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

describe("recursive types: the review of PR #677", () => {
  const declareCode = (f: () => unknown): string | undefined => {
    try {
      f();
    } catch (e) {
      if (e instanceof RisorError) return e.diagnostics[0]?.code;
      throw e;
    }
    return undefined;
  };

  test("a schema's recursive type with an opaque type's name is name-clash (f17 12.4 item 2, R315)", () => {
    const code = declareCode(() =>
      createEnv({ opaque: { Tree: { test: () => false } }, varSchemas: { tree: { schema: { type: "array", items: { $ref: "#" } } } } }),
    );
    assert.equal(code, "name-clash");
  });

  test("a named type of the environment at a definition's top is replaced by its definition (f17 12.2 item 4)", () => {
    const env = createEnv({ namedTypes: [{ name: "Tree", type: "{kids: Tree[]}" }] });
    const c = env.check(parse("type Rows = Rows[] | Tree; const x: Rows = {kids: []}; x is Rows[] ? null : x"));
    assert.equal(c.type.toString(), "{kids: Tree[]} | null");
  });

  test("a schema's type with a reference beside a record is bad-type (f17 12.4 items 4 and 5)", () => {
    const node = { type: "object", properties: { kids: { type: "array", items: { $ref: "#/$defs/node" } } }, required: ["kids"], additionalProperties: false };
    const end = { type: "object", properties: { end: { type: "integer" } }, required: ["end"] };
    assert.equal(declareCode(() => createEnv({ varSchemas: { t: { schema: { $defs: { node }, anyOf: [{ $ref: "#/$defs/node" }, end] } } } })), "bad-type");
    const env = createEnv({ varSchemas: { t: { schema: { $defs: { node }, anyOf: [{ $ref: "#/$defs/node" }, { type: "null" }] } } } });
    assert.equal(env.describe().variables.t, "Node | null");
  });

  test("completion reads a schema's recursive expected result whole, not by its name", () => {
    const env = createEnv({});
    const c = env.complete("{chi", { line: 1, column: 5 }, { schema: { type: "object", properties: { child: { $ref: "#" } } } });
    assert.deepEqual(c.items.map((i) => i.label), ["child"]);
  });

  test("completion of a string reads the literals of a recursive type's definition", () => {
    const env = createEnv({ namedTypes: [{ name: "Tt", type: '"a" | "b" | Tt[]' }], vars: { x: "Tt" } });
    const c = env.complete('x == "', { line: 1, column: 7 });
    assert.deepEqual(c.items.map((i) => i.label), ["a", "b"]);
  });

  test("a schema place is known by where it stands, not by the object that holds it (f17 12.4 item 1)", () => {
    const shared = { type: "object", properties: { next: { type: "array", items: { $ref: "#/properties/first" } } }, required: ["next"], additionalProperties: false };
    const schema = { type: "object", properties: { first: shared, second: shared }, required: ["first", "second"], additionalProperties: false };
    assert.equal(typeFromSchema(schema).toString(), "{first: First, second: {next: First[]}}");
    assert.equal(typeFromSchema(JSON.parse(JSON.stringify(schema))).toString(), "{first: First, second: {next: First[]}}");
  });

  test("the $ref that closes a cycle counts against the schema's depth (f01 3.10)", () => {
    const wrapped = (n: number): unknown => {
      let s: unknown = { type: "array", items: { $ref: "#" } };
      for (let i = 0; i < n; i++) s = { allOf: [s] };
      return s;
    };
    assert.equal(typeFromSchema(wrapped(62)).toString(), "Root");
    assert.throws(() => typeFromSchema(wrapped(63)), (e: unknown) => e instanceof RisorError && e.diagnostics[0].code === "bad-type");
  });

  test("a map's rest that is a reference to any reads by a computed key as map<any> does (f17 12.5)", () => {
    const sitesOf = (rest: string) => {
      const env = createEnv({ namedTypes: [{ name: "Top", type: "any | Top[]" }], varRecord: `{m: map<${rest}>, k: string}` });
      return env.check(parse("m[k]")).sites.map((x) => x.class);
    };
    assert.deepEqual(sitesOf("Top"), sitesOf("any"));
  });

  test("a schema place met again as a part of the schema that holds it needs no pointer to be named (f17 12.4 item 2)", () => {
    const schema = { $ref: "#/$defs/node/items", $defs: { node: { type: "array", items: { $ref: "#/$defs/node" } } } };
    assert.equal(typeFromSchema(schema).toString(), "Items");
  });

  /** Two families of 13 types, each holding the next twice: comparing one with the other passes 4,096 pairs. */
  const families = (): { name: string; type: string }[] => {
    const out: { name: string; type: string }[] = [];
    for (const f of ["A", "B"]) for (let i = 0; i < 13; i++) out.push({ name: `${f}${i}`, type: `{x: ${f}${(i + 1) % 13}[], y: ${f}${(i + 1) % 13}[]}` });
    return out;
  };

  test("a variable's type past the bound of pairs is bad-type (f17 12.5 item 7)", () => {
    const code = declareCode(() => createEnv({ namedTypes: families(), varRecord: "{v: {k: \"a\", t: A0} | {k: \"b\", t: B0}}" }));
    assert.equal(code, "bad-type");
  });

  test("check-limit of a Join of ? : is on the innermost conditional (f17 12.5 item 7)", () => {
    const env = createEnv({ namedTypes: families(), varRecord: "{p: bool, q: bool, a: A0, b: B0}" });
    let err: unknown;
    try {
      env.compile(parse("p ? 0 : q ? a : b"));
    } catch (e) {
      err = e;
    }
    assert.ok(err instanceof RisorError);
    assert.equal(err.diagnostics[0].code, "check-limit");
    assert.deepEqual(err.diagnostics[0].span, { start: { line: 1, column: 9 }, end: { line: 1, column: 18 } });
  });
});
