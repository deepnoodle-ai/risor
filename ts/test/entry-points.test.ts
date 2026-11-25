// Host API tests for package F16 slice D2: `export const` entry points,
// `Entries`, per-entry expected results, and the Call stage (f16 section 12).

import { describe, test } from "node:test";
import assert from "node:assert/strict";
import {
  createEnv,
  parse,
  RisorError,
  schemaOf,
  typeInt,
  typeString,
} from "../src/index.ts";

function thrown(f: () => unknown): RisorError {
  try {
    f();
  } catch (e) {
    assert.ok(e instanceof RisorError, String(e));
    return e;
  }
  assert.fail("expected RisorError");
}

describe("F16 slice D2: Entries and hasResult", () => {
  test("CheckResult.entries and Program.entries report metadata, inputType, schemaOf, and resultType", () => {
    const src = `
type Node = {
  /** Node label. */
  label: string,
  next: Node | null,
};

/** Greet a user. */
export const greet = (
  /** User name. */
  name: string,
  /** Optional greeting. */
  prefix: string = "Hi",
): string => \`\${prefix}, \${name}!\`;

/** Sum a linked list of nodes. */
export const walkNode = (n: Node): int => if (n.next == null) { 1 } else { 1 + walkNode(n.next) };

export const untypedRec = ([a, b]: int[]) => a + b;
`;
    const env = createEnv({});
    const expr = parse(src);
    const checked = env.check(expr);
    assert.equal(checked.type.toString(), "never");
    assert.equal(checked.entries.length, 3);

    const [greet, walkNode, untypedRec] = checked.entries;
    assert.equal(greet.name, "greet");
    assert.equal(greet.doc, "Greet a user.");
    assert.deepEqual(
      greet.params.map((p) => ({ name: p.name, type: p.type.toString(), doc: p.doc, optional: p.optional })),
      [
        { name: "name", type: "string", doc: "User name.", optional: false },
        { name: "prefix", type: "string", doc: "Optional greeting.", optional: true },
      ],
    );
    assert.equal(greet.inputType?.toString(), "{name: string, prefix?: string}");
    assert.equal(greet.resultType?.toString(), "string");
    assert.deepEqual(schemaOf(greet.inputType!), {
      type: "object",
      additionalProperties: false,
      required: ["name"],
      properties: {
        name: { type: "string", description: "User name." },
        prefix: { type: "string", description: "Optional greeting." },
      },
    });

    // Row 1 recursive record keeps recursive reference for SchemaOf.
    assert.equal(walkNode.name, "walkNode");
    assert.equal(walkNode.doc, "Sum a linked list of nodes.");
    assert.equal(walkNode.inputType?.toString(), "Node");
    assert.ok((schemaOf(walkNode.inputType!) as Record<string, unknown>).$defs !== undefined);
    assert.equal(walkNode.resultType?.toString(), "int");

    // Row 3 destructured non-record parameter has no input record.
    assert.equal(untypedRec.name, "untypedRec");
    assert.equal(untypedRec.doc, null);
    assert.equal(untypedRec.params[0].name, null);
    assert.equal(untypedRec.inputType, null);
    assert.equal(untypedRec.resultType?.toString(), "int");

    const prog = env.compile(expr);
    assert.equal(prog.hasResult, false);
    assert.deepEqual(prog.entries().map((e) => e.name), ["greet", "walkNode", "untypedRec"]);

    // Compiled with check: false -> resultType is null, parameter types and inputType still known.
    const unchecked = env.compile(expr, { check: false });
    assert.equal(unchecked.hasResult, false);
    assert.equal(unchecked.checkResult, null);
    assert.equal(unchecked.entries()[0].resultType, null);
    assert.equal(unchecked.entries()[0].inputType?.toString(), "{name: string, prefix?: string}");

    // Program.entries() returns a defensive copy: mutating it does not affect future calls.
    const copied = prog.entries() as unknown as Array<{ name: string; params: Array<{ name: string | null }> }>;
    copied[0].name = "mutated";
    copied[0].params[0].name = "mutatedParam";
    copied[0].params.pop();
    copied.pop();
    const fresh = prog.entries();
    assert.deepEqual(fresh.map((e) => e.name), ["greet", "walkNode", "untypedRec"]);
    assert.deepEqual(fresh[0].params.map((p) => p.name), ["name", "prefix"]);
  });

  test("hasResult is true when result expression is present, and Run/Measure fail declare no-result when absent", () => {
    const env = createEnv({});
    const withRes = env.compile(parse("export const f = () => 1; f() + 1"));
    assert.equal(withRes.hasResult, true);
    assert.equal(withRes.run(), 2);

    const noRes = env.compile(parse("export const f = () => 1;"));
    assert.equal(noRes.hasResult, false);
    const e1 = thrown(() => noRes.run());
    assert.deepEqual([e1.phase, e1.diagnostics[0].code, e1.usage], ["declare", "no-result", { steps: 0, allocated: 0 }]);
    const e2 = thrown(() => noRes.measure());
    assert.deepEqual([e2.phase, e2.diagnostics[0].code, e2.usage], ["declare", "no-result", { steps: 0, allocated: 0 }]);
  });
});

describe("F16 slice D2: expected and entryExpected validation", () => {
  const env = createEnv({});
  const noRes = parse("export const add = (a: int, b: int) => a + b;");

  test("expected on a result-less program fails declare no-result in check and compile", () => {
    assert.deepEqual([thrown(() => env.check(noRes, "int")).phase, thrown(() => env.check(noRes, "int")).diagnostics[0].code], ["declare", "no-result"]);
    assert.deepEqual([thrown(() => env.compile(noRes, { expected: "condition" })).phase, thrown(() => env.compile(noRes, { expected: "condition" })).diagnostics[0].code], ["declare", "no-result"]);
    assert.deepEqual([thrown(() => env.compile(noRes, { check: false, expected: "int" })).phase, thrown(() => env.compile(noRes, { check: false, expected: "int" })).diagnostics[0].code], ["declare", "no-result"]);
  });

  test("entryExpected accepts text, schema, and Type, rejects condition, unknown names, and mismatches", () => {
    env.check(noRes, undefined, { add: "int" });
    env.check(noRes, undefined, { add: { schema: { type: "integer" } } });
    env.check(noRes, undefined, { add: { type: typeInt } });
    env.compile(noRes, { entryExpected: { add: { type: typeInt } } });

    const mismatch = thrown(() => env.check(noRes, undefined, { add: { type: typeString } }));
    assert.deepEqual([mismatch.phase, mismatch.diagnostics[0].code], ["check", "result-mismatch"]);

    const badCond = thrown(() => env.check(noRes, undefined, { add: "condition" as unknown as string }));
    assert.deepEqual([badCond.phase, badCond.diagnostics[0].code], ["declare", "bad-type"]);

    const badName = thrown(() => env.check(noRes, undefined, { missing: "int" }));
    assert.deepEqual([badName.phase, badName.diagnostics[0].code], ["declare", "bad-name"]);

    // Unchecked compile still validates entryExpected names and types, skipping the fit.
    assert.equal(thrown(() => env.compile(noRes, { check: false, entryExpected: { missing: "int" } })).diagnostics[0].code, "bad-name");
    assert.equal(thrown(() => env.compile(noRes, { check: false, entryExpected: { add: "Nope" } })).diagnostics[0].code, "bad-type");
    assert.equal(env.compile(noRes, { check: false, entryExpected: { add: "string" } }).call("add", [2, 3]), 5);
  });
});

describe("F16 slice D2: Call stage (positional, record, conformance, isolation, opaque, cancel)", () => {
  test("call, callMeasure, callRecord, callRecordMeasure, and isolation between calls", () => {
    const env = createEnv({ varRecord: "{mult: int}" });
    const prog = env.compile(parse(`
const unused = fail("must not run");
const factor = mult * 2;
export const scale = (x: int, offset: int = 1) => x * factor + offset;
export const ping = () => "pong";
`));

    assert.equal(prog.call("scale", [5], { mult: 3 }), 31);
    assert.deepEqual(prog.callMeasure("scale", [5, 10], { mult: 4 }), { value: 50, usage: { steps: 4, allocated: 0 } });
    assert.equal(prog.callRecord("scale", { x: 5 }, { mult: 2 }), 21);
    assert.deepEqual(prog.callRecordMeasure("ping", {}, { mult: 1 }), { value: "pong", usage: { steps: 1, allocated: 0 } });

    // Isolation: earlier call's vars do not leak into later call.
    assert.equal(prog.call("scale", [2, 0], { mult: 10 }), 40);
    assert.equal(prog.call("scale", [2, 0], { mult: 1 }), 4);
  });

  test("declare failures on Call carry zero usage", () => {
    const env = createEnv({});
    const prog = env.compile(parse(`
export const pair = (a: int, b: int = 1) => a + b;
export const noRec = ([a, b]: int[]) => a + b;
`));

    const badName = thrown(() => prog.call("nope"));
    assert.deepEqual([badName.phase, badName.diagnostics[0].code, badName.usage], ["declare", "bad-name", { steps: 0, allocated: 0 }]);

    const badArity = thrown(() => prog.call("pair", []));
    assert.deepEqual([badArity.phase, badArity.diagnostics[0].code, badArity.usage], ["declare", "arity", { steps: 0, allocated: 0 }]);

    const noInput = thrown(() => prog.callRecord("noRec", { a: 1 }));
    assert.deepEqual([noInput.phase, noInput.diagnostics[0].code, noInput.usage], ["declare", "no-input-record", { steps: 0, allocated: 0 }]);
  });

  test("argument conformance caps at 100 diagnostics across args and stops on depth-exceeded", () => {
    const env = createEnv({});
    const prog = env.compile(parse(`
type Node = { next: Node | null };
export const twoLists = (a: int[], b: int[]) => len(a) + len(b);
export const deep = (n: Node) => n == null;
`));

    const bad60 = Array.from({ length: 60 }, () => "x");
    const posErr = thrown(() => prog.call("twoLists", [bad60, bad60]));
    assert.equal(posErr.phase, "run");
    assert.deepEqual(posErr.usage, { steps: 0, allocated: 0 });
    assert.equal(posErr.diagnostics.length, 100);
    assert.deepEqual(posErr.diagnostics[0].path, [0, 0]);
    assert.deepEqual(posErr.diagnostics[59].path, [0, 59]);
    assert.deepEqual(posErr.diagnostics[60].path, [1, 0]);
    assert.deepEqual(posErr.diagnostics[99].path, [1, 39]);
    assert.equal(posErr.diagnostics[0].span, undefined);
    assert.equal(posErr.diagnostics[0].class, "value");
    assert.deepEqual(posErr.diagnostics[0].trace, []);

    const recErr = thrown(() => prog.callRecord("twoLists", { a: bad60, b: bad60 }));
    assert.equal(recErr.diagnostics.length, 100);
    assert.deepEqual(recErr.diagnostics[0].path, ["a", 0]);
    assert.deepEqual(recErr.diagnostics[60].path, ["b", 0]);

    // Depth-exceeded (>200 nested objects) stops conformance with class "limit" and no path.
    let nested: Record<string, unknown> = { next: null };
    for (let i = 0; i < 205; i++) nested = { next: nested };
    const depErr = thrown(() => prog.call("deep", [nested]));
    assert.equal(depErr.phase, "run");
    assert.equal(depErr.diagnostics.length, 1);
    assert.equal(depErr.diagnostics[0].code, "depth-exceeded");
    assert.equal(depErr.diagnostics[0].class, "limit");
    assert.equal(depErr.diagnostics[0].path, undefined);
    assert.deepEqual(depErr.usage, { steps: 0, allocated: 0 });
  });

  test("unchecked compile still validates arguments on Call", () => {
    const env = createEnv({});
    const prog = env.compile(parse("export const f = (x: int) => x + 1;"), { check: false });
    const e = thrown(() => prog.call("f", ["bad"]));
    assert.deepEqual([e.phase, e.diagnostics[0].code, e.diagnostics[0].path, e.usage], ["run", "not-conforming", [0], { steps: 0, allocated: 0 }]);
    assert.equal(prog.call("f", [41]), 42);
  });

  test("opaque values and cancellation signal in Call stage", () => {
    class Token {
      readonly user: string;
      constructor(user: string) {
        this.user = user;
      }
    }
    const env = createEnv({
      opaque: {
        Token: {
          test: (v): v is Token => v instanceof Token,
          methods: { user: { type: "() => string", call: (_h, r) => (r as Token).user } },
        },
      },
    });
    const prog = env.compile(parse(`
export const who = (t: Token): string => t.user();
export const whoRec = (req: { tok: Token }): string => req.tok.user();
`));
    assert.equal(prog.call("who", [new Token("alice")]), "alice");
    assert.equal(prog.callRecord("whoRec", { tok: new Token("bob") }), "bob");

    const badOpaque = thrown(() => prog.call("who", ["not-a-token"]));
    assert.deepEqual([badOpaque.phase, badOpaque.diagnostics[0].code, badOpaque.diagnostics[0].path], ["run", "not-conforming", [0]]);

    const ac = new AbortController();
    ac.abort();
    const cancelled = thrown(() => prog.call("who", [new Token("alice")], undefined, { signal: ac.signal }));
    assert.deepEqual([cancelled.phase, cancelled.diagnostics[0].code], ["run", "cancelled"]);
  });

  test("throwing getters on host call inputs are translated into RisorError instead of escaping raw", () => {
    const env = createEnv({});
    const prog = env.compile(parse(`
export const pair = (a: int, b: int) => a + b;
export const rec = (r: { a: int }) => r.a;
`));

    // Getter throws during record conformance (conformAllWalk).
    const throwsInConform = {
      get a(): number {
        throw new Error("conform getter boom");
      },
      b: 1,
    };
    const e1 = thrown(() => prog.callRecord("pair", throwsInConform));
    assert.deepEqual([e1.phase, e1.diagnostics[0].code, e1.usage], ["run", "internal-error", { steps: 0, allocated: 0 }]);

    // Getter passes conformance on first read, then throws during fields-mode argument extraction.
    let reads = 0;
    const throwsInExtract = {
      get a(): number {
        reads++;
        if (reads > 1) throw new Error("extract getter boom");
        return 1;
      },
      b: 2,
    };
    const e2 = thrown(() => prog.callRecordMeasure("pair", throwsInExtract));
    assert.deepEqual([e2.phase, e2.diagnostics[0].code, e2.usage], ["run", "internal-error", { steps: 0, allocated: 0 }]);

    // Positional call with a throwing getter on a record argument is also translated.
    const throwsPositional = {
      get a(): number {
        throw new Error("positional getter boom");
      },
    };
    const e3 = thrown(() => prog.call("rec", [throwsPositional]));
    assert.deepEqual([e3.phase, e3.diagnostics[0].code, e3.usage], ["run", "internal-error", { steps: 0, allocated: 0 }]);
  });

  test("annotated entry point result is checked by its annotation, not body form", () => {
    const env = createEnv({});
    const e1 = thrown(() => env.check(parse("export const f = (): {a: int, b?: int} => ({a: 1});"), undefined, { f: "{a: int}" }));
    assert.deepEqual([e1.phase, e1.diagnostics[0].code], ["check", "result-mismatch"]);
    const e2 = thrown(() => env.check(parse("export const f = (x: int): int | null => { x };"), undefined, { f: "int" }));
    assert.deepEqual([e2.phase, e2.diagnostics[0].code], ["check", "result-mismatch"]);
  });
});
