// Unit tests of package F5 for the laws the data cases do not state one by
// one: Join never makes a union, Unite, printing that reads back, and size.

import { describe, test } from "node:test";
import assert from "node:assert/strict";
import { createEnv } from "../src/host/api.ts";
import { join, printType, typeEqual, typeSize, unite } from "../src/types/type.ts";

const env = createEnv({ types: [{ name: "Shape", type: '{kind: "circle", r: float} | {kind: "rect", h: float, w: float}' }] });
const T = (text: string) => env.declaredType(text);

describe("tagged unions (f05 2, 4)", () => {
  test("printing follows the least string of each tag set, and reads back", () => {
    const u = T('{a: "y", b: "p"} | {a: "x", b: "q"} | null');
    assert.equal(printType(u), '{a: "x", b: "q"} | {a: "y", b: "p"} | null');
    assert.ok(typeEqual(T(printType(u)), u));
    assert.equal(printType(T("Shape[]")), '({kind: "circle", r: float} | {h: float, kind: "rect", w: float})[]');
  });

  test("Join keeps a union only beside the same union; otherwise the record Join of the flats", () => {
    assert.ok(typeEqual(join(T("Shape"), T("Shape")), T("Shape")));
    assert.equal(printType(join(T("Shape"), T('{kind: "circle", r: int}'))), '{h?: float, kind: "circle" | "rect", r?: float, w?: float}');
    const j = join(T('{kind: "a", x: int}'), T('{kind: "b"}'));
    assert.equal(printType(j), '{kind: "a" | "b", x?: int}');
  });

  test("Unite makes a union where Join would not, and is Join without a tag", () => {
    assert.equal(printType(unite(T('{kind: "a", x: int}'), T('{kind: "b"}'))), '{kind: "a", x: int} | {kind: "b"}');
    assert.equal(printType(unite(T("{a: int}"), T("{b: int}"))), printType(join(T("{a: int}"), T("{b: int}"))));
    assert.ok(typeEqual(unite(T("Shape"), T('{kind: "circle", r: float}')), T("Shape")));
  });

  test("a union's size is the sum of its variants' sizes", () => {
    assert.equal(typeSize(T('{kind: "a", v: int} | {kind: "b", v: int}')), 2 * (1 + 2));
  });

  test("Shape | Shape forms no union, and Shape[] | Shape[] is a list", () => {
    assert.throws(() => T("Shape | Shape"));
    assert.equal(printType(T("Shape[] | Shape[]")), printType(T("Shape[]")));
  });
});
