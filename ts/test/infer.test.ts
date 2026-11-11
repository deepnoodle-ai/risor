// The TypeScript type of a Risor type (05 E.1 row 5, R202): `Infer` on type
// text and on the constructors, checked by `npm run typecheck`; and a `Type`
// as a Standard Schema, checked at run time.

import { describe, test } from "node:test";
import assert from "node:assert/strict";
import {
  createEnv, exactRecordType, listType, literalType, mapType, nullable, opaqueTypeNamed, parse, recordType, typeAny,
  typeInt, typeString, typeFloat, unionType,
} from "../src/index.ts";
import type { Env, Infer, Program, Type } from "../src/index.ts";

// ---- Compile-time assertions ----------------------------------------------------------

type Equal<A, B> = (<G>() => G extends A ? 1 : 2) extends <G>() => G extends B ? 1 : 2 ? true : false;
/** Mutual assignability: for record types that are an intersection. */
type Same<A, B> = [A] extends [B] ? ([B] extends [A] ? true : false) : false;
function expect<T extends true>(): T | void {}

// Primitives and their joins (05-types.md 3.3, 3.4).
expect<Equal<Infer<"int">, number>>();
expect<Equal<Infer<"float">, number>>();
expect<Equal<Infer<"string">, string>>();
expect<Equal<Infer<"bool">, boolean>>();
expect<Equal<Infer<"null">, null>>();
expect<Equal<Infer<"any">, unknown>>();
expect<Equal<Infer<"never">, never>>();
expect<Equal<Infer<"int | null">, number | null>>();
expect<Equal<Infer<"int | float">, number>>();
expect<Equal<Infer<"any | int">, unknown>>();
expect<Equal<Infer<"never | string">, string>>();

// Literals.
expect<Equal<Infer<'"pass" | "fail"'>, "pass" | "fail">>();
expect<Equal<Infer<"'a' | 'b' | null">, "a" | "b" | null>>();
expect<Equal<Infer<'"a\\"b"'>, unknown>>(); // an escaped quote: the token is not read
expect<Equal<Infer<'"a\\nb"'>, string>>(); // an escape: string, not the literal

// Lists.
expect<Equal<Infer<"int[]">, number[]>>();
expect<Equal<Infer<"int[][]">, number[][]>>();
expect<Equal<Infer<"never[]">, never[]>>();
expect<Equal<Infer<"(int | string)[]">, (number | string)[]>>();
expect<Equal<Infer<"int[] | string[]">, (number | string)[]>>(); // joined, as 05-types.md 3.4
expect<Equal<Infer<"int[] | null">, number[] | null>>();
expect<Equal<Infer<"int | string[]">, number | string[]>>();
expect<Equal<Infer<"( int ) [ ]">, number[]>>();

// Maps and records.
expect<Equal<Infer<"map<int>">, { [k: string]: number }>>();
expect<Equal<Infer<"map<map<string>>">, { [k: string]: { [k: string]: string } }>>();
expect<Equal<Infer<"{}">, { [k: string]: never }>>();
expect<Equal<Infer<"{name: string, age: int}">, { name: string; age: number }>>();
expect<Equal<Infer<"{note?: string, owner: string | null}">, { owner: string | null; note?: string }>>();
expect<Equal<Infer<'{"first name": string, \'x-y\'?: int,}'>, { "first name": string; "x-y"?: number }>>();
expect<Same<Infer<"{action: string, ...}">, { action: string } & { [k: string]: unknown }>>();
expect<Same<Infer<"{id: int, ...string}">, { id: number } & { [k: string]: string | number }>>();
expect<Equal<Infer<"{...float}">, { [k: string]: number }>>();
expect<Equal<Infer<"{tests: {name: string, ms: int}[]}">, { tests: { name: string; ms: number }[] }>>();

// Tagged unions (F5) are a union of the records.
expect<Equal<Infer<'{kind: "a", x: int} | {kind: "b", y: string}'>, { kind: "a"; x: number } | { kind: "b"; y: string }>>();

// White space and comments.
expect<
  Equal<
    Infer<`{
      // the user
      name: string, /* their age */ age: int,
    }`>,
    { name: string; age: number }
  >
>();

// Names: from the map given, else unknown.
class Point {
  readonly x: number;
  constructor(x: number) {
    this.x = x;
  }
}
expect<Equal<Infer<"Point">, unknown>>();
expect<Equal<Infer<"Point[]", { Point: Point }>, Point[]>>();
expect<Equal<Infer<"{at: Point | null}", { Point: Point }>, { at: Point | null }>>();

// Text that is not read is unknown, never an error.
expect<Equal<Infer<"number">, unknown>>();
expect<Equal<Infer<"{name string}">, unknown>>();
expect<Equal<Infer<"int extra">, unknown>>();
expect<Equal<Infer<string>, unknown>>();

// The constructors.
expect<Equal<Infer<typeof typeInt>, number>>();
expect<Equal<Infer<Type>, unknown>>();
const user = exactRecordType(
  { key: "name", type: typeString, optional: false },
  { key: "age", type: typeInt, optional: false },
  { key: "tags", type: listType(literalType("admin", "dev")), optional: true },
);
expect<Equal<Infer<typeof user>, { name: string; age: number; tags?: ("admin" | "dev")[] }>>();
const open = recordType([{ key: "id", type: typeInt, optional: false }], typeAny);
expect<Same<Infer<typeof open>, { id: number } & { [k: string]: unknown }>>();
expect<Equal<Infer<ReturnType<typeof mapType<number>>>, { [key: string]: number }>>();
expect<Equal<Infer<ReturnType<typeof nullable<string>>>, string | null>>();
const u = unionType(typeInt, nullable(typeString));
expect<Equal<Infer<typeof u>, number | string | null>>();
expect<Equal<Infer<ReturnType<typeof opaqueTypeNamed<Point>>>, Point>>();
expect<Equal<Infer<typeof user>, typeof user extends Type<infer V> ? V : never>>();
void typeFloat;

// ---- A typed environment --------------------------------------------------------------

describe("R202: an environment's variables type a run", () => {
  const env = createEnv({
    vars: { user: "{name: string, age: int, email: string | null}", n: "int" },
    opaque: { Point: { test: (v) => v instanceof Point } },
    functions: { twice: { type: "(x: int) => int", call: (_h, x) => (x as number) * 2 } },
  });
  type V = Parameters<ReturnType<typeof env.compile>["run"]>[0];

  test("a run takes the declared variables", () => {
    const p = env.compile(parse("user.age >= 18 && twice(n) > 0"));
    assert.equal(p.run({ user: { name: "Ada", age: 36, email: null }, n: 1 }), true);
    // A Map for a record and a bigint for an int are what a host may give.
    assert.equal(p.run({ user: new Map<string, unknown>([["name", "Ada"], ["age", 36], ["email", null]]), n: 1n }), true);
    // A variable may be left out: a run fails only when it reads it.
    assert.equal(env.compile(parse("n")).run({ n: 2 }), 2);
    // @ts-expect-error a string where the variable is an int
    void (() => p.run({ n: "1" }));
    // @ts-expect-error a field of the wrong type
    void (() => p.run({ user: { name: "Ada", age: "36", email: null } }));
    // @ts-expect-error a variable the environment does not declare
    void (() => p.run({ m: 1 }));
    // A Map or a function is not checked.
    assert.equal(env.compile(parse("n")).run(new Map([["n", 3]])), 3);
    assert.equal(env.compile(parse("n")).run(() => 4), 4);
  });

  test("an opaque type is the type its test guards", () => {
    const t = env.parseType("{at: Point, tags: string[]}");
    expect<Equal<Infer<typeof t>, { at: Point; tags: string[] }>>();
    assert.equal(t.toString(), "{at: Point, tags: string[]}");
  });

  test("an opaque variable takes the opaque value only", () => {
    class Box {
      readonly #v: number;
      constructor(v: number) {
        this.#v = v;
      }
      get v(): number {
        return this.#v;
      }
    }
    const e = createEnv({
      vars: { at: "Point", box: "Box | null" },
      opaque: { Point: { test: (v) => v instanceof Point }, Box: { test: (v) => v instanceof Box } },
    });
    const p = e.compile(parse("box"));
    assert.equal(p.run({ at: new Point(1), box: null }), null);
    assert.equal(p.run({ box: new Box(1) }) instanceof Box, true);
    // @ts-expect-error a Map is not a Point
    void (() => p.run({ at: new Map([["x", 1]]) }));
    // @ts-expect-error a bigint field is not a Point
    void (() => p.run({ at: { x: 1n } }));
    // @ts-expect-error a plain object is not a class with a private member
    void (() => p.run({ box: { v: 1 } }));
  });

  test("a child's opaque test replaces the parent's", () => {
    class Point3 {
      readonly z: string = "";
    }
    const child = env.with({ opaque: { Point: { test: (v) => v instanceof Point3 } } });
    const t = child.parseType("Point");
    expect<Equal<Infer<typeof t>, Point3>>();
    const kept = env.with({ opaque: { Point: { methods: {} } } }).parseType("Point");
    expect<Equal<Infer<typeof kept>, Point>>();
  });

  test("a child adds and replaces variables", () => {
    const child = env.with({ vars: { n: "string", extra: "bool" } });
    child.compile(parse("n")).run({ n: "x", extra: true, user: { name: "A", age: 1, email: null } });
    // @ts-expect-error the child's n is a string
    void (() => child.compile(parse("n")).run({ n: 1 }));
  });

  test("varRecord, varTypes and varSchemas", () => {
    const e = createEnv({
      varRecord: "{a: int, b: string | null}",
      varTypes: { c: listType(typeString) },
      varSchemas: { d: { schema: { type: "integer" } } },
    });
    const p = e.compile(parse("a"));
    expect<Same<NonNullable<Parameters<typeof p.run>[0]>, ReadonlyMap<string, unknown> | ((name: string) => unknown) | { a?: number | bigint; b?: string | null; c?: readonly string[]; d?: unknown }>>();
    assert.equal(p.run({ a: 1, c: ["x"], d: 2 }), 1);
  });

  test("an environment with no declarations takes any variables", () => {
    const e = createEnv();
    assert.equal(e.compile(parse("1"), { check: false }).run({ anything: 1 }), 1);
  });

  test("a typed environment is an Env", () => {
    const take = (e: Env): Program => e.compile(parse("1"));
    assert.equal(take(env).run(), 1);
  });

  void (null as unknown as V);
});

// ---- A Type is a Standard Schema ------------------------------------------------------

describe("R202: a Type is a Standard Schema", () => {
  test("validate gives a conforming value back", () => {
    const r = user["~standard"].validate({ name: "Ada", age: 36 });
    assert.deepEqual(r, { value: { name: "Ada", age: 36 } });
    assert.equal(user["~standard"].vendor, "risor");
    assert.equal(user["~standard"].version, 1);
  });

  test("validate gives the plain form", () => {
    const t = listType(mapType(typeInt));
    assert.deepEqual(t["~standard"].validate([new Map([["a", 1n]])]), { value: [{ a: 1 }] });
  });

  test("validate gives every failure with its path", () => {
    const r = user["~standard"].validate({ name: 1, tags: ["admin", "x"] });
    assert.ok(!(r instanceof Promise) && r.issues !== undefined);
    const got = r.issues.map((i) => [i.path, i.message]);
    assert.equal(got.length, 3, JSON.stringify(got));
    assert.deepEqual(
      got.map(([p]) => p),
      [["age"], ["name"], ["tags", 1]],
    );
    for (const [, m] of got) assert.equal(typeof m, "string");
  });

  test("a number that cannot enter is an issue below any", () => {
    for (const bad of [NaN, Infinity, 2n ** 60n]) {
      const r = typeAny["~standard"].validate(bad);
      assert.ok(!(r instanceof Promise) && r.issues?.length === 1 && r.issues[0].path?.length === 0);
    }
    const rec = exactRecordType({ key: "v", type: typeAny, optional: false });
    const r = rec["~standard"].validate({ v: [1, NaN] });
    assert.ok(!(r instanceof Promise) && r.issues !== undefined);
    assert.deepEqual(r.issues.map((i) => i.path), [["v", 1]]);
  });

  test("a list that holds itself is an issue", () => {
    const xs: unknown[] = [1];
    xs.push(xs);
    const r = listType(typeAny)["~standard"].validate(xs);
    assert.ok(!(r instanceof Promise) && r.issues !== undefined);
    assert.deepEqual(r.issues.map((i) => i.path), [[1]]);
    // The same value twice, not inside itself, is no cycle.
    const y = [1];
    assert.deepEqual(typeAny["~standard"].validate([y, y]), { value: [[1], [1]] });
  });

  test("a __proto__ key is kept", () => {
    const r = mapType(typeInt)["~standard"].validate(new Map([["__proto__", 1]]));
    assert.ok(!(r instanceof Promise) && r.issues === undefined);
    assert.ok(Object.hasOwn(r.value as object, "__proto__"));
    assert.equal((r.value as Record<string, number>)["__proto__"], 1);
  });

  test("an invalid type gives one issue", () => {
    const bad = literalType();
    const r = bad["~standard"].validate("x");
    assert.ok(!(r instanceof Promise) && r.issues?.length === 1);
  });

  test("a type the environment read tells its opaque values", () => {
    const env = createEnv({ opaque: { Point: { test: (v) => v instanceof Point } } });
    const t = env.parseType("{at: Point}");
    assert.deepEqual(t["~standard"].validate({ at: new Point(1) }), { value: { at: new Point(1) } });
    const r = t["~standard"].validate({ at: { x: 1 } });
    assert.ok(!(r instanceof Promise) && r.issues?.length === 1);
  });
});
