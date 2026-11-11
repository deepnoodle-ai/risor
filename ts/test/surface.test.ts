// The public surface of the package (05 Part E, ruling R200): what
// src/index.ts exports, and the calls the suite does not reach. Every import
// here is from the root, as a host's would be.

import { describe, test } from "node:test";
import assert from "node:assert/strict";
import * as risor from "../src/index.ts";
import {
  conform,
  conformAll,
  createEnv,
  decode,
  Env,
  exactRecordType,
  format,
  formatTemplate,
  join,
  listType,
  literalType,
  loadValue,
  LoadError,
  mapType,
  nullable,
  opaqueTypeNamed,
  parse,
  parseTemplate,
  recordType,
  renderDetail,
  renderJSON,
  renderLine,
  RisorError,
  taggedUnionType,
  typeAny,
  typeFromSchema,
  typeInt,
  typeNever,
  typeNull,
  typeString,
  unionType,
} from "../src/index.ts";

function thrown(f: () => unknown): RisorError {
  try {
    f();
  } catch (e) {
    assert.ok(e instanceof RisorError, String(e));
    return e;
  }
  assert.fail("no error");
}

describe("E.2: the root exports the surface and nothing else", () => {
  test("the value exports", () => {
    assert.deepEqual(Object.keys(risor).sort(), [
      "Analysis", "Env", "Expr", "HostError", "LoadError", "Program", "RisorError", "Template", "Type",
      "conform", "conformAll", "createEnv", "decode", "descriptionText", "exactRecordType", "format", "formatTemplate",
      "join", "jsonText", "listType", "literalType", "loadValue", "mapType", "nullable", "opaqueTypeNamed",
      "parse", "parseTemplate", "recordType", "renderDetail", "renderJSON", "renderLine", "taggedUnionType", "truthy",
      "typeAny", "typeBool", "typeFloat", "typeFromSchema", "typeInt", "typeNever", "typeNull", "typeString", "unionType",
    ]);
  });
});

describe("E.3: the environment", () => {
  test("vars, varRecord, varSchemas and varTypes declare variables", () => {
    const env = createEnv({
      vars: { a: "int" },
      varRecord: "{b: string}",
      varSchemas: { c: { schema: { type: "boolean" } } },
      varTypes: { d: listType(typeInt) },
    });
    assert.deepEqual(env.describe().variables, { a: "int", b: "string", c: "bool", d: "int[]" });
  });
  test("namedTypes by text, by schema and by a Type", () => {
    const env = createEnv({
      namedTypes: [
        { name: "Id", type: "int" },
        { name: "Tag", schema: { enum: ["x", "y"] } },
        { name: "Pair", typeOf: exactRecordType({ key: "a", type: typeInt, optional: false }) },
      ],
    });
    assert.deepEqual(env.describe().types, { Id: "int", Pair: "{a: int}", Tag: '"x" | "y"' });
  });
  test("without, someNotNull and deadlineMs", () => {
    const env = createEnv({ varRecord: "{r: int | null, s: int | null}", without: ["len"], someNotNull: [["r", "s"]], limits: { deadlineMs: 1000 } });
    assert.equal(env.libraryNames().functions.includes("len"), false);
    assert.equal(env.check(parse("r ?? s")).type.toString(), "int");
  });
  test("a varType naming an undeclared opaque type, or an invalid type, is bad-type", () => {
    assert.equal(thrown(() => createEnv({ varTypes: { t: opaqueTypeNamed("Time") } })).diagnostics[0].code, "bad-type");
    assert.equal(thrown(() => createEnv({ varTypes: { t: literalType() } })).diagnostics[0].code, "bad-type");
  });
  test("with, fromDescription, parseType and libraryNames", () => {
    const env = createEnv({ varRecord: "{n: int}" });
    const child = env.with({ vars: { m: "string" } });
    assert.deepEqual(Object.keys(child.describe().variables), ["m", "n"]);
    const back = Env.fromDescription(JSON.parse(risor.descriptionText(child.describe())));
    assert.deepEqual(back.describe(), child.describe());
    assert.equal(env.parseType("{a: int}[]").toString(), "{a: int}[]");
    assert.equal(thrown(() => env.parseType("Nope")).diagnostics[0].code, "bad-type");
    const names = env.libraryNames();
    assert.ok(names.functions.includes("len") && names.methods.list.includes("filter"));
    assert.deepEqual([names.namespaces, names.groupFunctions], [[], []]);
    assert.deepEqual(names.functions, [...names.functions].sort());
    const json = createEnv({ groups: ["json"], without: ["list.filter"] }).libraryNames();
    assert.deepEqual(json.namespaces, ["json"]);
    assert.ok(json.groupFunctions.includes("json.parse"));
    assert.ok(!json.methods.list.includes("filter"));
  });
});

describe("E.4: programs", () => {
  const env = createEnv({ varRecord: "{x: int}" });
  test("parse gives an Expr with its text and analysis", () => {
    const x = parse("x + 1");
    assert.equal(x.text, "x + 1");
    assert.deepEqual(x.analyze().names, ["x"]);
    assert.equal(x.analyze().json(), '{"names":["x"],"refs":[{"name":"x","path":[],"span":[1,1,1,2]}],"calls":[],"literal":false,"segments":null}');
    assert.equal(thrown(() => parse("1 +")).phase, "parse");
    const [call] = parse("g(x => f(x), 2)").analyze().calls.filter((c) => c.name === "g");
    assert.deepEqual([call.args[0], call.args[1]?.text, call.argSpans, call.inLambda], [undefined, "g(x => f(x), 2)", [[1, 3, 1, 12], [1, 14, 1, 15]], false]);
    assert.equal(parse("g(x => f(x))").analyze().calls.find((c) => c.name === "f")?.inLambda, true);
  });
  test("check gives a Type and the sites; a failure is thrown with line spans", () => {
    const c = env.check(parse("x / x"));
    assert.equal(c.type.toString(), "float");
    assert.deepEqual(c.sites, [{ class: "divide", span: [1, 1, 1, 6] }]);
    const e = thrown(() => env.check(parse("y")));
    assert.equal(e.phase, "check");
    assert.deepEqual(e.diagnostics[0].span, [1, 1, 1, 2]);
  });
  test("the expected result: text, condition, a schema or a Type", () => {
    assert.equal(thrown(() => env.check(parse("x"), "string")).diagnostics[0].code, "result-mismatch");
    env.check(parse("x"), "condition");
    assert.equal(thrown(() => env.check(parse("x"), { schema: { type: "string" } })).diagnostics[0].code, "result-mismatch");
    assert.equal(thrown(() => env.check(parse("x"), { type: typeString })).diagnostics[0].code, "result-mismatch");
    env.check(parse("x"), { type: typeInt });
  });
  test("compile, checkResult, run and measure", () => {
    const p = env.compile(parse("x * 2"));
    assert.equal(p.checkResult?.type.toString(), "int");
    assert.equal(env.compile(parse("x"), { check: false }).checkResult, null);
    assert.equal(p.run({ x: 3 }), 6);
    assert.deepEqual(p.measure({ x: 3 }), { value: 6, usage: { steps: 1, allocated: 0 } });
    const e = thrown(() => env.compile(parse("x / 0"), { check: false }).run({ x: 1 }));
    assert.equal(e.phase, "run");
    assert.ok(e.usage !== undefined);
  });
});

describe("E.5: templates", () => {
  test("isSingle, segments, part and program", () => {
    const t = parseTemplate("a ${{ 1 }} b ${{ 2 }}");
    assert.equal(t.isSingle, false);
    assert.deepEqual(t.segments, [
      { kind: "text", value: "a ", span: [1, 1, 1, 3] },
      { kind: "part", span: [1, 3, 1, 11] },
      { kind: "text", value: " b ", span: [1, 11, 1, 14] },
      { kind: "part", span: [1, 14, 1, 22] },
    ]);
    const env = createEnv({});
    assert.equal(env.compile(t.part(1)!).run(), 2);
    assert.equal(t.part(2), undefined);
    assert.equal(env.compile(t.program("string")).run(), "a 1 b 2");
    assert.equal(formatTemplate("${{1+1}}"), "${{ 1 + 1 }}");
  });
});

describe("E.7: the structural type API", () => {
  const rec = recordType([{ key: "b", type: typeString, optional: true }, { key: "a", type: typeInt, optional: false }], typeAny);
  test("constructors and their text", () => {
    assert.equal(listType(typeInt).toString(), "int[]");
    assert.equal(mapType(typeInt).toString(), "map<int>");
    assert.equal(nullable(typeInt).toString(), "int | null");
    assert.equal(literalType("b", "a").toString(), '"a" | "b"');
    assert.equal(rec.toString(), "{a: int, b?: string, ...}");
    assert.equal(exactRecordType({ key: "a", type: typeInt, optional: false }).toString(), "{a: int}");
    assert.equal(unionType(typeInt, typeNull, typeString).toString(), "int | string | null");
    assert.equal(join(typeInt, typeString).toString(), "int | string");
  });
  test("kind", () => {
    assert.equal(typeNever.kind(), "never");
    assert.equal(typeAny.kind(), "any");
    assert.equal(literalType("a").kind(), "string");
    assert.equal(mapType(typeInt).kind(), "record");
    assert.equal(nullable(typeInt).kind(), "mixed");
    assert.equal(opaqueTypeNamed("Time").kind(), "opaque");
    assert.equal(literalType().kind(), "invalid");
  });
  test("accessors", () => {
    assert.ok(listType(typeInt).elem()!.equal(typeInt));
    assert.equal(typeInt.elem(), undefined);
    assert.deepEqual(rec.fields().map((f) => [f.key, f.type.toString(), f.optional]), [["a", "int", false], ["b", "string", true]]);
    assert.equal(rec.field("b")?.optional, true);
    assert.equal(rec.field("c"), undefined);
    assert.ok(rec.rest()!.equal(typeAny));
    assert.equal(rec.isExact(), false);
    assert.equal(nullable(rec).isNullable(), true);
    assert.ok(nullable(rec).nonNull().equal(rec));
    assert.deepEqual(unionType(typeString, typeNull, typeInt).alternatives().map(String), ["null", "int", "string"]);
    assert.deepEqual(literalType("b", "a").literals(), ["a", "b"]);
    assert.deepEqual(typeString.literals(), []);
    assert.equal(opaqueTypeNamed("Time").opaqueName(), "Time");
    assert.equal(typeInt.opaqueName(), "");
  });
  test("tagged unions", () => {
    const a = exactRecordType({ key: "kind", type: literalType("a"), optional: false }, { key: "n", type: typeInt, optional: false });
    const b = exactRecordType({ key: "kind", type: literalType("b"), optional: false });
    const u = taggedUnionType(b, a);
    assert.equal(u.kind(), "union");
    assert.equal(u.tag(), "kind");
    assert.deepEqual(u.variants().map(String), ['{kind: "a", n: int}', '{kind: "b"}']);
    assert.ok(unionType(a, b).equal(u));
    assert.equal(taggedUnionType(a, a).kind(), "invalid");
    assert.equal(taggedUnionType(typeInt).err()?.diagnostics[0].code, "bad-type");
  });
  test("fits, equal and an invalid type", () => {
    assert.ok(typeInt.fits(nullable(typeInt)));
    assert.ok(!nullable(typeInt).fits(typeInt));
    const bad = recordType([{ key: "a", type: typeInt, optional: false }, { key: "a", type: typeInt, optional: false }], typeNever);
    assert.equal(bad.kind(), "invalid");
    assert.equal(bad.err()?.diagnostics[0].code, "bad-type");
    assert.equal(listType(bad).kind(), "invalid");
    assert.equal(typeInt.err(), undefined);
  });
  test("typeFromSchema gives a Type", () => {
    assert.equal(typeFromSchema({ type: "array", items: { type: "integer" } }).toString(), "int[]");
  });
});

describe("E.7: conformance and loads", () => {
  const env = createEnv({ opaque: { Time: { test: (v) => v instanceof Date } } });
  test("conform and conformAll take a Type; opaque values by the tests of its environment", () => {
    assert.equal(conform(new Date(), env.parseType("Time")), null);
    assert.equal(conform(1, env.parseType("Time"))?.code, "not-conforming");
    assert.equal(conformAll({ a: "x", b: "y" }, env.parseType("{a: int, b: int}"), 5).length, 2);
  });
  test("loadValue with text, a schema or a Type, and LoadError", () => {
    assert.deepEqual(loadValue(env, "{port: 80}", { expected: "{port: int}" }), { port: 80 });
    assert.deepEqual(loadValue(env, "[1]", { expected: { type: listType(typeInt) } }), [1]);
    assert.throws(() => loadValue(env, "1 +", { expected: "int" }), (e) => e instanceof LoadError && e.step === 1 && e.diagnostics[0].span !== undefined);
  });
  test("decode throws for an opaque value", () => {
    assert.deepEqual(decode(new Map([["a", 1]])), { a: 1 });
    assert.equal(thrown(() => decode([Symbol("x")])).phase, "run");
  });
});

describe("E.8: diagnostics and tools", () => {
  test("renderLine, renderDetail and renderJSON take a failure's diagnostics", () => {
    const e = thrown(() => createEnv({}).compile(parse("1 / 0"), { check: false }).run());
    assert.equal(renderLine(e.diagnostics), "error[divide-by-zero] 1:1-1:6: the divisor of / is zero\n");
    assert.equal(renderDetail(e.diagnostics, "1 / 0"), "error[divide-by-zero] 1:1-1:6: the divisor of / is zero\n    1 / 0\n    ^^^^^\n");
    assert.equal(renderJSON(e.diagnostics), '{"class":"divide","code":"divide-by-zero","message":"the divisor of / is zero","phase":"run","span":[1,1,1,6],"trace":[]}\n');
  });
  test("format, complete and hover", () => {
    assert.equal(format("1+2"), "1 + 2\n");
    assert.equal(thrown(() => format("1 +")).phase, "parse");
    const env = createEnv({ varRecord: "{user: {name: string}}" });
    assert.deepEqual(env.complete("user.n", { line: 1, column: 7 }).items.map((i) => i.label), ["name"]);
    assert.equal(env.hover("user.name", { line: 1, column: 7 })?.type, "string");
  });
});
