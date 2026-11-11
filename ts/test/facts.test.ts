// Unit tests of package F14 for the binding (f14 2.6, 3.2, 9): child
// environments, which no case reaches, the errors of a declaration, and the
// description's round trip.

import { describe, test } from "node:test";
import assert from "node:assert/strict";
import { createEnv, Env, parse, RisorError } from "../src/host/api.ts";
import { tryCheck } from "./helpers.ts";

const VARS = "{report: {v: string} | null, approve: {v: string} | null, third: {v: string} | null}";
const READ = "(report ?? approve).v";

const checks = (env: ReturnType<typeof createEnv>, src: string) => tryCheck(env, parse(src)).ok;

describe("host facts in a child environment (f14 2.6)", () => {
  const parent = createEnv({ varRecord: VARS, someNotNull: [["report", "approve"]] });
  test("a child keeps its parent's host facts", () => {
    assert.ok(checks(parent, READ));
    assert.ok(checks(parent.with({}), READ));
  });
  test("a host fact that names a variable the child replaces is dropped whole", () => {
    const child = parent.with({ varRecord: "{approve: {v: string} | null}" });
    assert.ok(!checks(child, READ));
    assert.equal(child.describe().facts, undefined);
  });
  test("a child's own host fact may name the parent's variables and its own", () => {
    const child = parent.with({ varRecord: "{approve: {v: string} | null}", someNotNull: [["approve", "report"]] });
    assert.ok(checks(child, READ));
    const c2 = parent.with({ someNotNull: [["third"]] });
    assert.ok(checks(c2, "third.v"));
    assert.ok(!checks(parent, "third.v"), "the parent is not changed");
  });
  test("a child's host fact is tested against its variables", () => {
    assert.throws(() => parent.with({ someNotNull: [["nope"]] }), (e) => e instanceof RisorError && e.diagnostics[0].code === "bad-name");
  });
});

describe("the errors of a declaration (f14 2.5, 3.2 item 2)", () => {
  const codes = (facts: { someNotNull: string[] }[]) => {
    try {
      createEnv({ varRecord: VARS, functions: { f: { type: "() => int", call: () => 1 } }, someNotNull: facts.map((f) => f.someNotNull) });
    } catch (e) {
      assert.ok(e instanceof RisorError && e.phase === "declare");
      return e.diagnostics.map((d) => d.code);
    }
    return [];
  };
  test("no member, a name that is no variable, a member twice: each bad-name", () => {
    assert.deepEqual(codes([{ someNotNull: [] }]), ["bad-name"]);
    assert.deepEqual(codes([{ someNotNull: ["report", "f", "len", "Report", ""] }]), ["bad-name", "bad-name", "bad-name", "bad-name"]);
    assert.deepEqual(codes([{ someNotNull: ["report", "report"] }]), ["bad-name"]);
    assert.deepEqual(codes([{ someNotNull: ["report"] }, { someNotNull: ["report"] }]), []);
  });
});

describe("the description (f14 9)", () => {
  test("distinct host facts, sorted, and the round trip", () => {
    const env = createEnv({ varRecord: VARS, someNotNull: [["report", "approve"], ["approve", "report"], ["third"], ["approve"]] });
    const d = env.describe();
    assert.deepEqual(d.facts, [{ someNotNull: ["approve"] }, { someNotNull: ["approve", "report"] }, { someNotNull: ["third"] }]);
    const back = Env.fromDescription(JSON.parse(JSON.stringify(d)));
    assert.deepEqual(back.describe(), d);
    assert.ok(tryCheck(back, parse(READ)).ok);
  });
});

describe("the outcome line of a case with facts (f14 10.4)", () => {
  test('"facts": [] counts as a case with facts: the line writes description', async () => {
    const { validateCaseFile } = await import("../src/suite/casefile.ts");
    const { outcomeLine } = await import("../src/suite/outcome.ts");
    const f = validateCaseFile({ env: "{n: int}", cases: [{ name: "a", source: "1", facts: [], expect: { value: 1 } }, { name: "b", source: "1", expect: { value: 1 } }] }, ".");
    assert.ok(f.ok);
    assert.ok(outcomeLine(f.cases[0]).startsWith('{"name":"a","description":'));
    assert.ok(!outcomeLine(f.cases[1]).includes("description"));
  });
});
