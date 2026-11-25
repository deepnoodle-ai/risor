// The call handle's remaining() (03 3.5.2 item 3), and a nested run that
// takes what the caller has left and charges back what it used (03 3.2.5).
// The numbers are those of the Go and Rust tests of the same name.

import { describe, test } from "node:test";
import assert from "node:assert/strict";
import { createEnv, execute, parse, type Call, type HostFunction, type Limits, type Remaining, type Usage } from "../src/host/api.ts";

describe("03 3.5.2 item 3: remaining", () => {
  test("each limit less what the run has charged, with the call's own step", () => {
    let got: Remaining[] = [];
    const probe: HostFunction = {
      type: "() => int",
      call: (h) => {
        got.push(h.remaining());
        h.charge(5);
        h.allocate(3);
        got.push(h.remaining());
        return 0;
      },
    };
    const env = createEnv({ limits: { steps: 100, allocation: 50 }, functions: { probe } });
    let o = execute(env.compile(parse("probe()"), { check: false }));
    assert.ok(o.ok);
    assert.deepEqual(got, [{ steps: 99, allocation: 50 }, { steps: 94, allocation: 47 }]);
    assert.deepEqual(o.usage, { steps: 6, allocated: 3 });
    // Literals cost no step; the list's 4 units come after its elements.
    got = [];
    o = execute(env.compile(parse("[1, 2, probe()]"), { check: false }));
    assert.ok(o.ok);
    assert.deepEqual(got, [{ steps: 99, allocation: 50 }, { steps: 94, allocation: 47 }]);
    assert.deepEqual(o.usage, { steps: 6, allocated: 7 });
    // A limit that is off gives null.
    got = [];
    const off = env.with({ limits: { steps: null, allocation: null } });
    assert.ok(execute(off.compile(parse("probe()"), { check: false })).ok);
    assert.deepEqual(got, [{ steps: null, allocation: null }, { steps: null, allocation: null }]);
  });

  test("a failed charge changes nothing; after the call, 0 and 0", () => {
    let kept: Call | undefined;
    const got: Remaining[] = [];
    const env = createEnv({
      limits: { steps: 10, allocation: 10 },
      functions: {
        f: {
          type: "() => int",
          call: (h) => {
            kept = h;
            try {
              h.charge(20);
            } catch {
              // the run still ends with steps-exceeded
            }
            got.push(h.remaining());
            return 0;
          },
        },
      },
    });
    const o = execute(env.compile(parse("f()"), { check: false }));
    assert.ok(!o.ok);
    assert.equal(o.diagnostic.code, "steps-exceeded");
    assert.deepEqual(got, [{ steps: 9, allocation: 10 }]);
    assert.deepEqual(kept!.remaining(), { steps: 0, allocation: 0 });
  });

  test("a remaining of 0 is a limit of zero for a nested run", () => {
    const env = createEnv({ vars: { xs: "int[]" } });
    const vars = { xs: [1, 2, 3] };
    let o = execute(env.with({ limits: { steps: 0, allocation: null } }).compile(parse("xs[0]"), { check: false }), vars);
    assert.ok(!o.ok);
    assert.equal(o.diagnostic.code, "steps-exceeded");
    o = execute(env.with({ limits: { steps: null, allocation: 0 } }).compile(parse("xs.map(x => x)"), { check: false }), vars);
    assert.ok(!o.ok);
    assert.equal(o.diagnostic.code, "allocation-exceeded");
  });
});

const INNER = "xs.map(x => x * 2)";
const innerVars = { xs: [1, 2, 3, 4, 5, 6, 7, 8, 9, 10] };
const toolEnv = createEnv({ vars: { xs: "int[]" } });
const inner = parse(INNER);

/** The code of a run failure that a host function threw, or "". */
function limitCode(o: ReturnType<typeof execute>): string {
  return o.ok ? "" : o.diagnostic.code;
}

/** Runs the inner program with what the caller has left, then charges back what it used. */
function nestedTool(counter: { calls: number }): HostFunction {
  return {
    type: "() => int !",
    call: (h) => {
      counter.calls++;
      const left = h.remaining();
      const child = toolEnv.with({ limits: left });
      const o = execute(child.compile(inner), innerVars, { signal: h.signal });
      h.charge(o.usage.steps);
      h.allocate(o.usage.allocated);
      // A nested run that stopped at a limit stops the caller with the same
      // code: more than is left cannot be charged.
      if (limitCode(o) === "steps-exceeded") h.charge(Number.MAX_SAFE_INTEGER);
      if (limitCode(o) === "allocation-exceeded") h.allocate(Number.MAX_SAFE_INTEGER);
      if (!o.ok) throw new Error(o.diagnostic.message);
      return 1;
    },
  };
}

const cost: Usage = execute(toolEnv.compile(inner, { check: false }), innerVars).usage;

/** Charges what the inner program costs, as if its work were done in the caller. */
function inlineTool(counter: { calls: number }): HostFunction {
  return {
    type: "() => int !",
    call: (h) => {
      counter.calls++;
      h.charge(cost.steps);
      h.allocate(cost.allocated);
      return 1;
    },
  };
}

function outcome(fn: (c: { calls: number }) => HostFunction, limits: Limits, src: string): [string, Usage, number] {
  const counter = { calls: 0 };
  const env = createEnv({ limits, functions: { tool: fn(counter) } });
  const o = execute(env.compile(parse(src), { check: false }));
  return [limitCode(o), o.usage, counter.calls];
}

describe("03 3.2.5: a nested run with the remaining budget", () => {
  const src = "[tool(), tool(), tool()]";
  test("the inner program's cost", () => {
    assert.deepEqual(cost, { steps: 32, allocated: 11 });
  });

  test("unlimited, the caller's usage is its own plus the inner runs'", () => {
    const off = { steps: null, allocation: null };
    const [c1, nested, n] = outcome(nestedTool, off, src);
    const [c2, inline] = outcome(inlineTool, off, src);
    assert.equal(c1, "");
    assert.equal(c2, "");
    assert.equal(n, 3);
    assert.deepEqual(nested, inline);
    assert.deepEqual(nested, { steps: 3 * cost.steps + 3, allocated: 3 * cost.allocated + 4 });
  });

  test("the nested run gets exactly what is left", () => {
    let [code, u] = outcome(nestedTool, { steps: 1 + cost.steps }, "tool()");
    assert.equal(code, "");
    assert.deepEqual(u, { steps: 1 + cost.steps, allocated: cost.allocated });
    [code] = outcome(nestedTool, { steps: cost.steps }, "tool()");
    assert.equal(code, "steps-exceeded");
    [code, u] = outcome(nestedTool, { allocation: cost.allocated }, "tool()");
    assert.equal(code, "");
    assert.equal(u.allocated, cost.allocated);
    [code] = outcome(nestedTool, { allocation: cost.allocated - 1 }, "tool()");
    assert.equal(code, "allocation-exceeded");
  });

  test("the caller stops at the same call as when the work is charged inline", () => {
    const total = { steps: 3 * cost.steps + 3, allocated: 3 * cost.allocated + 4 };
    for (const s of [src, "try { [tool(), tool(), tool()] } catch { [] }"]) {
      for (let limit = 0; limit <= total.steps; limit++) {
        const [c1, u1, n1] = outcome(nestedTool, { steps: limit }, s);
        const [c2, u2, n2] = outcome(inlineTool, { steps: limit }, s);
        assert.equal(c1, c2, `limit ${limit}`);
        assert.equal(n1, n2, `limit ${limit}`);
        if (c1 === "") assert.deepEqual(u1, u2, `limit ${limit}`);
      }
      for (let limit = 0; limit <= total.allocated; limit++) {
        const [c1, u1, n1] = outcome(nestedTool, { allocation: limit }, s);
        const [c2, u2, n2] = outcome(inlineTool, { allocation: limit }, s);
        assert.equal(c1, c2, `units ${limit}`);
        assert.equal(n1, n2, `units ${limit}`);
        if (c1 === "") assert.deepEqual(u1, u2, `units ${limit}`);
      }
    }
  });
});
