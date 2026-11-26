# Embedding Risor v3 in TypeScript

This guide shows how to embed Risor v3 in a TypeScript or Node.js application using `@deepnoodle/risor` and `@deepnoodle/risor/testing`.

Every TypeScript example in this guide is type-checked and executed in CI against `ts/src/index.ts`.

## Contents

1. [Install and quick start](#1-install-and-quick-start)
2. [Environments and types](#2-environments-and-types)
3. [Host functions and opaque values](#3-host-functions-and-opaque-values)
4. [Limits, usage, and cancellation](#4-limits-usage-and-cancellation)
5. [Static analysis and templates](#5-static-analysis-and-templates)
6. [Conformance of host data, Standard Schema, and config loading](#6-conformance-of-host-data-standard-schema-and-config-loading)
7. [`schemaOf`: exporting JSON Schema](#7-schemaof-exporting-json-schema)
8. [Entry points (`export const`)](#8-entry-points-export-const)
9. [The long-lived host pattern](#9-the-long-lived-host-pattern)
10. [Testing expressions and editor tools](#10-testing-expressions-and-editor-tools)

---

## 1. Install and quick start

`@deepnoodle/risor` has **no runtime dependencies** and runs in Node.js 22.18+ and modern browsers.

```bash
npm install @deepnoodle/risor
```

A TypeScript host works in four synchronous stages ([`spec/core/03-host.md`](../../spec/core/03-host.md) 3.2):

1. **`parse(source)`** returns an immutable `Expr` (no environment needed).
2. **`env.check(expr, expected?)`** checks types without compiling and returns `{ type, sites, entries }`.
3. **`env.compile(expr, options?)`** checks (by default) and returns an immutable `Program<V>`.
4. **`program.run(vars, { signal }?)`** (or **`program.measure(...)`**) runs the program synchronously and returns a JavaScript value.

Because TypeScript types are erased at runtime, you declare variable types in Risor's type syntax (or JSON Schema / structural builders). The package's type-level `Infer<X>` parses those declarations at compile time so `program.run(vars)` is type-checked by `tsc`.

Every stage throws `RisorError` on failure. Render diagnostics with `renderLine`, `renderDetail`, and `renderJSON`, and inspect values with `truthy` and `jsonText`.

<!-- risor-verify: {"mode": "ts", "expectStdout": "out=true truthy=true json=true\nerror[no-field] 1:6-1:10: {age: int, name: string} has no field \"nmae\"\n"} -->
```ts
import {
  RisorError,
  createEnv,
  jsonText,
  parse,
  renderLine,
  truthy,
} from "@deepnoodle/risor";

const env = createEnv({
  vars: {
    user: "{name: string, age: int}",
    discount: "int",
  },
});

const expr = parse("user.age >= 18 && discount > 0");
const prog = env.compile(expr, { expected: "bool" });

// `vars` is statically typed from the environment's `vars` declarations.
const out = prog.run({
  user: { name: "Ada", age: 36 },
  discount: 15,
});

console.log(`out=${String(out)} truthy=${truthy(out)} json=${jsonText(out)}`);

try {
  env.check(parse("user.nmae"));
} catch (err) {
  if (err instanceof RisorError) {
    process.stdout.write(renderLine(err.diagnostics));
  }
}
```

> **Tip:** Always use `truthy(v)` instead of JavaScript's `Boolean(v)` when evaluating a Risor condition—in Risor, `[]` and `{}` are **falsy**, whereas in JavaScript they are truthy. Pass `expected: "condition"` to `env.check` / `env.compile` to catch constant conditions at check time. Also note that Risor map values created inside a program are returned as JavaScript `Map` instances—read entries with `.get(key)` or convert nested results to plain objects with `decode(v)`.

---

## 2. Environments and types

`createEnv(options)` builds an immutable `Env<V, P>`. You can declare variables in four ways:

| Option | Shape | Notes |
| --- | --- | --- |
| `vars` | `Record<string, string>` | One type text per variable (`{ age: "int" }`). Feeds `Infer<X>` for typed `run(vars)`. |
| `varRecord` | `string` | One record type text (`"{user: {age: int}}"`). Feeds `Infer<X>`. |
| `varTypes` | `Record<string, Type>` | Built structurally (`exactRecordType`, `listType`, `nullable`, `taggedUnionType`, `typeFromSchema`, `env.parseType`). |
| `varSchemas` | `Record<string, { schema, options? }>` | JSON Schema with optional `["closed", "absent-is-null", "defaults-applied"]`. |

You can also configure `namedTypes`, opt-in library `groups` (`"json"`, `"time"`, `"math"`, `"base64"`), `without` (removed library entries), `reject` (rejected check classes), and `someNotNull` (host facts).

### Child environments and portable descriptions

`env.with(options)` creates a child environment without changing the parent. `env.describe()` returns a plain `Description` object and `descriptionText(desc)` formats it as canonical JSON. Another host (or a browser editor) can reconstruct a check-only environment with `Env.fromDescription(desc)`.

<!-- risor-verify: {"mode": "ts", "expectStdout": "eventType={kind: \"click\", x: int} | {code: string, kind: \"key\"}\nstatus=ok chosen=backup\nroundTripSame=true\n"} -->
```ts
import {
  Env,
  createEnv,
  decode,
  descriptionText,
  exactRecordType,
  literalType,
  parse,
  taggedUnionType,
  typeInt,
  typeString,
} from "@deepnoodle/risor";

// Build a tagged union with the structural type API.
const click = exactRecordType(
  { key: "kind", type: literalType("click"), optional: false },
  { key: "x", type: typeInt, optional: false },
);
const key = exactRecordType(
  { key: "kind", type: literalType("key"), optional: false },
  { key: "code", type: typeString, optional: false },
);
const eventType = taggedUnionType(click, key);
console.log(`eventType=${eventType.toString()}`);

const parent = createEnv({
  namedTypes: [{ name: "Event", typeOf: eventType }],
  varSchemas: {
    cfg: {
      schema: {
        type: "object",
        required: ["env"],
        properties: { env: { enum: ["dev", "prod"] } },
        additionalProperties: false,
      },
    },
  },
  vars: {
    primary: "string | null",
    fallback: "string | null",
  },
  someNotNull: [["primary", "fallback"]],
  groups: ["json"],
  without: ["string.repeat"],
});

// Child environment adds `ev` without mutating `parent`.
const child = parent.with({ vars: { ev: "Event" } });

const prog = child.compile(
  parse(`{
    status: match ev.kind {
      "click" => if (ev.x >= 0) { "ok" } else { "neg" },
      "key" => ev.code,
    },
    chosen: primary ?? fallback,
  }`),
);

const result = decode(
  prog.run({
    ev: { kind: "click", x: 12 },
    primary: null,
    fallback: "backup",
  }),
) as { status: string; chosen: string };
console.log(`status=${result.status} chosen=${result.chosen}`);

// Round-trip the portable description.
const text1 = descriptionText(child.describe());
const rebuilt = Env.fromDescription(JSON.parse(text1));
const text2 = descriptionText(rebuilt.describe());
console.log(`roundTripSame=${text1 === text2}`);
```

---

## 3. Host functions and opaque values

Host functions in Risor are **synchronous** (`call(handle, ...args)`). Key rules ([`spec/core/03-host.md`](../../spec/core/03-host.md) 3.5):

- Mark fallible functions with `!` in their signature (`"(key: string) => string !"`).
- Throw an `Error` (such as `new HostError("...")`) to report a catchable `function-error`. Throwing a non-`Error` value reports an uncatchable `function-panic`.
- Use `handle.charge(steps)` and `handle.allocate(units)` proportionally to your work, and check `handle.signal?.aborted` for cancellable work.
- Opaque types declare a `test(value)` predicate (use a TypeScript type guard `(v): v is Session => v instanceof Session` so `Infer` types variables of that opaque type as `Session`) and `methods`, where the receiver is the first argument after `handle`.

<!-- risor-verify: {"mode": "ts", "expectStdout": "slug=hello-world token=alice:admin caught=missing key \"nope\"\n"} -->
```ts
import { HostError, createEnv, decode, parse } from "@deepnoodle/risor";

class Session {
  readonly user: string;
  readonly role: string;
  constructor(user: string, role: string) {
    this.user = user;
    this.role = role;
  }
}

const store = new Map<string, string>([["plan", "pro"]]);

const env = createEnv({
  opaque: {
    Session: {
      doc: "An authenticated user session.",
      test: (v): v is Session => v instanceof Session,
      methods: {
        label: {
          type: "(sep: string) => string",
          doc: "Format user and role separated by sep.",
          call(handle, recv, sep) {
            const s = recv as Session;
            const out = `${s.user}${String(sep)}${s.role}`;
            handle.allocate(Math.floor(out.length / 64) + 1);
            return out;
          },
        },
      },
    },
  },
  functions: {
    "store.get": {
      type: "(key: string) => string !",
      doc: "Look up a value by key.",
      call(handle, rawKey) {
        if (handle.signal?.aborted) throw new HostError("aborted");
        handle.charge(1);
        const key = String(rawKey);
        const val = store.get(key);
        if (val === undefined) throw new HostError(`missing key "${key}"`);
        handle.allocate(Math.floor(val.length / 64) + 1);
        return val;
      },
    },
    slug: {
      type: "(text: string) => string",
      call(handle, rawText) {
        const text = String(rawText);
        handle.charge(Math.floor(text.length / 64) + 1);
        return text.trim().toLowerCase().replaceAll(" ", "-");
      },
    },
  },
  vars: {
    sess: "Session",
  },
});

const prog = env.compile(
  parse(`{
    slug: slug("Hello World"),
    token: sess.label(":"),
    caught: try { store.get("nope") } catch (e) { e.message },
  }`),
);

const out = decode(prog.run({ sess: new Session("alice", "admin") })) as {
  slug: string;
  token: string;
  caught: string;
};
console.log(`slug=${out.slug} token=${out.token} caught=${out.caught}`);
```

---

## 4. Limits, usage, and cancellation

Configure limits on `createEnv({ limits: { ... } })` or `env.with({ limits: { ... } })`:

| Limit | Default | Disable | Error code |
| --- | --- | --- | --- |
| `steps` | `1_000_000` | `null` | `steps-exceeded` |
| `allocation` | `1_000_000` | `null` | `allocation-exceeded` |
| `deadlineMs` | `null` (off) | `null` | `deadline-exceeded` |
| `depth` | `200` | `1` to `400` (cannot disable) | `depth-exceeded` |
| `sourceLimit` | `SOURCE_LIMIT` (1,048,576 code points) | Option on `parse`, `parseTemplate`, `format`, `loadValue`, `complete`, `hover` | `source-too-large` |

Pass an `AbortSignal` to `program.run(vars, { signal })` or `program.measure(vars, { signal })` to cancel a run (`cancelled`). On failure, `RisorError.usage` still reports the usage charged up to the failure. Limit errors cannot be caught by `try { ... } catch`.

<!-- risor-verify: {"mode": "ts", "expectStdout": "sum=45 steps>0=true\nlimitCode=steps-exceeded hasUsage=true\ncancelCode=cancelled\nsourceCode=source-too-large\n"} -->
```ts
import { RisorError, createEnv, parse } from "@deepnoodle/risor";

function errorCode(fn: () => unknown): { code: string; hasUsage: boolean } {
  try {
    fn();
    return { code: "", hasUsage: false };
  } catch (err) {
    if (err instanceof RisorError) {
      return {
        code: err.diagnostics[0]?.code ?? "",
        hasUsage: err.usage !== undefined,
      };
    }
    throw err;
  }
}

const env = createEnv({
  limits: {
    steps: 5_000,
    allocation: 2_000,
    deadlineMs: 500,
    depth: 50,
  },
});

const { value, usage } = env
  .compile(parse("range(0, 10).reduce((a, b) => a + b, 0)"))
  .measure();
console.log(`sum=${String(value)} steps>0=${usage.steps > 0}`);

// Limit failures cannot be caught inside the Risor program.
const heavy = env.compile(parse("try { len(range(0, 50000)) } catch { 0 }"));
const limitRes = errorCode(() => heavy.measure());
console.log(`limitCode=${limitRes.code} hasUsage=${limitRes.hasUsage}`);

// Cancel via AbortController / AbortSignal.
const ac = new AbortController();
ac.abort();
const loop = env.compile(parse("len(range(0, 4000).map(n => n + 1))"));
const cancelRes = errorCode(() => loop.run(undefined, { signal: ac.signal }));
console.log(`cancelCode=${cancelRes.code}`);

// Restrict source size at parse time.
const srcRes = errorCode(() => parse('"this source is too long"', { sourceLimit: 8 }));
console.log(`sourceCode=${srcRes.code}`);
```

---

## 5. Static analysis and templates

`expr.analyze()` inspects a parsed program without needing an environment. It never fails and returns:

- `names`: sorted unique free names read.
- `refs`: static paths (`name`, `path` of strings and `{ star: true }`, `span`).
- `calls` / `callsOf(...names)`: call sites with `args` (each non-lambda argument as its own `Expr` with spans into the full source), `argSpans`, and `inLambda`.
- `literal`: `{ value }` when the program is a literal, else `false`.

`parseTemplate(text)` parses host templates containing `${{ expr }}` parts. Use `template.program("string")` for string output, `template.program("value")` to preserve the expression's typed value on single-part templates (`template.isSingle`), and `template.part(i)` to inspect individual parts.

<!-- risor-verify: {"mode": "ts", "expectStdout": "names=order,user callArg=order.id\nmsg=Hi Ada, 2 items singleVal=36\n"} -->
```ts
import { createEnv, parse, parseTemplate } from "@deepnoodle/risor";

const analysis = parse("lookup(order.id) && user.active").analyze();
const [lookupCall] = analysis.callsOf("lookup");
console.log(`names=${analysis.names.join(",")} callArg=${lookupCall.args[0]?.text}`);

const env = createEnv({
  varRecord: "{user: {name: string, age: int}, items: string[]}",
});
const vars = {
  user: { name: "Ada", age: 36 },
  items: ["a", "b"],
};

const msgTmpl = parseTemplate("Hi ${{ user.name }}, ${{ len(items) }} items");
const msg = env.compile(msgTmpl.program("string")).run(vars);

const valTmpl = parseTemplate("${{ user.age }}");
if (!valTmpl.isSingle) throw new Error("expected single template");
const age = env.compile(valTmpl.program("value")).run(vars);

console.log(`msg=${String(msg)} singleVal=${String(age)}`);
```

---

## 6. Conformance of host data, Standard Schema, and config loading

Validate untrusted host values with `conform` and `conformAll`:

- `conform(value, type, { steps }?)` returns `null` when valid or the first `Diagnostic` (with `path`).
- `conformAll(value, type, max, { steps }?)` returns up to `max` diagnostics.

### Standard Schema v1 (`~standard`)

In TypeScript, every `Type<T>` implements the [Standard Schema v1](https://standardschema.dev) interface (`~standard`, vendor `"risor"`), so form, router, and RPC libraries that accept a Standard Schema can validate directly against a Risor `Type` without an adapter.

### Config files (`loadValue`, `decode`, `LoadError`)

`loadValue(env, source, { expected, vars, signal, max, decode })` runs all config-loading steps in order—declare (`STEP_DECLARE = 0`), parse (`1`), check (`2`), run (`3`), conform (`4`), and decode into plain JS objects/arrays (`5`)—throwing `LoadError` with `.step`, `.diagnostics`, and `.usage`. Call `decode(value)` standalone to convert any Risor result (including `Map` values) into plain JSON-compatible objects without sharing memory with host inputs.

<!-- risor-verify: {"mode": "ts", "expectStdout": "conformErrs=2 paths=port|tls\nstandardIssues=1\nloaded=0.0.0.0:8443\nloadFailStep=2\n"} -->
```ts
import {
  LoadError,
  STEP_CHECK,
  conformAll,
  createEnv,
  decode,
  loadValue,
} from "@deepnoodle/risor";

const env = createEnv({ vars: { prod: "bool" } });
const portType = env.parseType("{port: int, tls: bool}");

// Collect multiple conformance failures with structured paths.
const diags = conformAll({ port: "bad", tls: 123 }, portType, 10);
console.log(
  `conformErrs=${diags.length} paths=${diags.map((d) => d.path?.join(".")).join("|")}`,
);

// Standard Schema v1 validation on a Risor Type.
const stdRes = portType["~standard"].validate({ port: "bad", tls: true });
if ("issues" in stdRes && stdRes.issues) {
  console.log(`standardIssues=${stdRes.issues.length}`);
}

// Load a config program into plain data.
const cfg = loadValue<{ host: string; port: number }>(
  env,
  `{
    host: if (prod) { "0.0.0.0" } else { "127.0.0.1" },
    port: if (prod) { 8443 } else { 8080 },
  }`,
  {
    expected: "{host: string, port: int}",
    vars: { prod: true },
    max: 5,
  },
);
console.log(`loaded=${cfg.host}:${cfg.port}`);

// Standalone decode converts Map into a plain object.
const plain = decode(new Map([["ok", true]])) as { ok: boolean };
if (!plain.ok) throw new Error("decode failed");

// Extra key in a closed expected type fails at STEP_CHECK (step 2).
try {
  loadValue(env, `{host: "localhost", port: 80, extra: 1}`, {
    expected: "{host: string, port: int}",
  });
} catch (err) {
  if (err instanceof LoadError && err.step === STEP_CHECK) {
    console.log(`loadFailStep=${err.step}`);
  }
}
```

---

## 7. `schemaOf`: exporting JSON Schema

Export any non-opaque `Type` to JSON Schema with:

- **`schemaOf(type)`**: returns a plain JavaScript JSON Schema object.
- **`schemaText(type)`**: returns the canonical JSON string (keys in code point order, no whitespace, identical across Go, TypeScript, and Rust).

Field doc comments become `"description"`, recursive types are emitted once under `"$defs"`, `typeFromSchema` round-trips the schema back to an equal `Type`, and opaque types throw `RisorError` (`bad-type`).

<!-- risor-verify: {"mode": "ts", "expectStdout": "schema={\"additionalProperties\":false,\"properties\":{\"city\":{\"description\":\"Destination city.\",\"type\":\"string\"},\"units\":{\"enum\":[\"C\",\"F\"],\"type\":\"string\"}},\"required\":[\"city\"],\"type\":\"object\"}\nroundTripEqual=true opaqueCode=bad-type\n"} -->
```ts
import {
  RisorError,
  createEnv,
  opaqueTypeNamed,
  parse,
  schemaOf,
  schemaText,
  typeFromSchema,
} from "@deepnoodle/risor";

const env = createEnv({
  opaque: { Token: { test: () => false } },
  vars: {
    req: `{/** Destination city. */ city: string, units?: "C" | "F"}`,
  },
});

const { type } = env.check(parse("req"));
const text = schemaText(type);
console.log(`schema=${text}`);

const back = typeFromSchema(schemaOf(type));
let opaqueCode = "";
try {
  schemaOf(opaqueTypeNamed("Token"));
} catch (err) {
  if (err instanceof RisorError) opaqueCode = err.diagnostics[0].code;
}
console.log(`roundTripEqual=${back.equal(type)} opaqueCode=${opaqueCode}`);
```

---

## 8. Entry points (`export const`)

Export typed functions from a Risor program using `export const`, with or without a final result expression:

- **`program.entries()`** (and `checkResult.entries`) returns `Entry[]` in source order (`name`, `doc`, `params`, `inputType`, `resultType`).
- **`schemaOf(entry.inputType!)`** produces the JSON Schema for an entry point's inputs—including parameter `description` comments and optional defaults.
- **`entryExpected`** on `env.check` / `env.compile` checks each named entry point's return type.
- **`program.call` / `callMeasure`** invoke positionally; **`program.callRecord` / `callRecordMeasure`** invoke with a single input object conformed against `inputType`.

<!-- risor-verify: {"mode": "ts", "expectStdout": "hasResult=false entry=quote doc=Calculate an order total.\npositional=120 record=135\nschemaHasDesc=true\n"} -->
```ts
import { createEnv, parse, schemaText } from "@deepnoodle/risor";

const env = createEnv({ vars: { taxRate: "float" } });

const src = `
const multiplier = 1 + taxRate;

/** Calculate an order total. */
export const quote = (
  /** Subtotal in USD. */
  subtotal: float,
  /** Optional shipping fee. */
  shipping: float = 0.0,
): float => subtotal * multiplier + shipping;
`;

const prog = env.compile(parse(src), {
  entryExpected: { quote: "float" },
});

const [entry] = prog.entries();
console.log(`hasResult=${prog.hasResult} entry=${entry.name} doc=${entry.doc}`);

const vars = { taxRate: 0.2 };
const v1 = prog.call("quote", [100], vars);
const v2 = prog.callRecord("quote", { subtotal: 100, shipping: 15 }, vars);
console.log(`positional=${String(v1)} record=${String(v2)}`);

const inputSchema = schemaText(entry.inputType!);
console.log(
  `schemaHasDesc=${inputSchema.includes('"description":"Subtotal in USD."')}`,
);
```

---

## 9. The long-lived host pattern

Follow [`spec/core/03-host.md`](../../spec/core/03-host.md) 3.2.5 for long-lived servers, workflow engines, and UI event loops:

1. **Build one `Env` at startup** with your domain types, host functions, and limits.
2. **Parse, check, and compile once** when rules or handlers are loaded.
3. **Reuse the compiled `Program` across every event** with per-event variables, budgets, and `AbortSignal`.
4. **The host owns all state.** Each event takes `{ state, ev }` and returns the next state plus tagged-union actions for the host to apply.

### Poisoned runs and nested runs

- **Poisoned run:** Keep a per-run `poisoned` flag and an `AbortController`. If a host function hits a fatal violation, set `poisoned = true`, call `ac.abort()`, immediately throw from any later host call while `poisoned` is set, and discard the run's outcome if `poisoned` is true.
- **Nested run:** Inside a host function that runs another program, pass `handle.remaining()` directly to `toolEnv.with({ limits: handle.remaining() })`, forward `handle.signal`, and charge back the nested run's `usage.steps` and `usage.allocated`. If the nested run failed with `steps-exceeded` or `allocation-exceeded`, overcharge that budget (`handle.charge(Number.MAX_SAFE_INTEGER)` / `handle.allocate(Number.MAX_SAFE_INTEGER)`) so the caller stops with the same uncatchable limit code.

<!-- risor-verify: {"mode": "ts", "expectStdout": "finalCount=6\nnestedResult=12 nestedSteps>0=true\nnestedLimitCode=steps-exceeded\n"} -->
```ts
import { RisorError, createEnv, decode, parse } from "@deepnoodle/risor";

// 1. Build env once, compile handler once, reuse across many events.
const handlerEnv = createEnv({
  vars: {
    state: "{count: int}",
    ev: '{kind: "inc", by: int} | {kind: "reset"}',
  },
  limits: { steps: 10_000, allocation: 10_000 },
});

const handlerProg = handlerEnv.compile(
  parse(`
    match ev.kind {
      "inc" => {count: state.count + ev.by},
      "reset" => {count: 0},
    }
  `),
  { expected: "{count: int}" },
);

let state = { count: 0 };
for (const ev of [
  { kind: "inc" as const, by: 2 },
  { kind: "inc" as const, by: 4 },
]) {
  state = decode(handlerProg.run({ state, ev })) as { count: number };
}
console.log(`finalCount=${state.count}`);

// 2. Nested run sharing the caller's remaining step & allocation budget.
const toolEnv = createEnv({ vars: { xs: "int[]" } });
const toolExpr = parse("xs.map(n => n * 2).reduce((a, b) => a + b, 0)");

const parentEnv = createEnv({
  limits: { steps: 500, allocation: 500 },
  functions: {
    runTool: {
      type: "(xs: int[]) => int !",
      call(handle, xs) {
        const child = toolEnv.with({ limits: handle.remaining() });
        const prog = child.compile(toolExpr);
        try {
          const { value, usage } = prog.measure(
            { xs: xs as number[] },
            { signal: handle.signal },
          );
          handle.charge(usage.steps);
          handle.allocate(usage.allocated);
          return value;
        } catch (err) {
          if (err instanceof RisorError) {
            if (err.usage) {
              handle.charge(err.usage.steps);
              handle.allocate(err.usage.allocated);
            }
            const code = err.diagnostics[0]?.code;
            if (code === "steps-exceeded") handle.charge(Number.MAX_SAFE_INTEGER);
            if (code === "allocation-exceeded") handle.allocate(Number.MAX_SAFE_INTEGER);
          }
          throw err;
        }
      },
    },
  },
});

const callerExpr = parse("runTool([1, 2, 3])");
const { value, usage } = parentEnv.compile(callerExpr).measure();
console.log(`nestedResult=${String(value)} nestedSteps>0=${usage.steps > 0}`);

const tightProg = parentEnv.with({ limits: { steps: 5 } }).compile(callerExpr);
try {
  tightProg.run();
} catch (err) {
  if (err instanceof RisorError) {
    console.log(`nestedLimitCode=${err.diagnostics[0].code}`);
  }
}
```

---

## 10. Testing expressions and editor tools

### Case-file testing with `@deepnoodle/risor/testing`

`@deepnoodle/risor/testing` runs Risor case files inside any JS test runner (`node:test`, Vitest, Jest) without importing one:

- `loadCaseFile(path, { base: env })` loads a `.test.json` or `.test.risor` file from disk.
- `caseFile(data, { base: env })` and `caseFileText(text, isProgram, { base: env })` load in-memory case files.
- `defineCases(test, file)` registers one test per case; or iterate `file.cases` and call `c.run()` directly.

### Formatting, completion, and hover

Use `format(source)` / `formatTemplate(source)` for canonical formatting and `env.complete(source, pos)` / `env.hover(source, pos)` for editor assistance on incomplete input.

<!-- risor-verify: {"mode": "ts", "expectStdout": "casesPassed=2\nformatted=1 + 2\ncompletions=name\nhoverType=string\n"} -->
```ts
import { createEnv, format } from "@deepnoodle/risor";
import { caseFile } from "@deepnoodle/risor/testing";

// Run a case file against the host's environment.
const ruleEnv = createEnv({ vars: { vip: "bool", total: "float" } });
const file = caseFile(
  {
    env: "{vip: bool, total: float}",
    cases: [
      {
        name: "vip gets 20 percent",
        source: "if (vip) { total * 0.8 } else { total }",
        vars: { vip: true, total: 100 },
        expect: { value: 80 },
      },
      {
        name: "regular pays full price",
        source: "if (vip) { total * 0.8 } else { total }",
        vars: { vip: false, total: 100 },
        expect: { value: 100 },
      },
    ],
  },
  { base: ruleEnv },
);

let passed = 0;
for (const c of file.cases) {
  const verdict = c.run();
  if (verdict.status !== "pass") throw new Error(verdict.report);
  passed++;
}
console.log(`casesPassed=${passed}`);

// Formatting, completion, and hover.
console.log(`formatted=${format("1+2").trim()}`);
const editorEnv = createEnv({ varRecord: "{user: {name: string, age: int}}" });
const comp = editorEnv.complete("user.na", { line: 1, column: 8 });
console.log(`completions=${comp.items.map((i) => i.label).join(",")}`);
const hover = editorEnv.hover("user.name", { line: 1, column: 7 });
console.log(`hoverType=${hover?.type}`);
```

---

## Further reading

- [Risor v3 Language Guide](../guide.md)
- [Host specification (`spec/core/03-host.md`)](../../spec/core/03-host.md)
- [Go Host Guide](go.md)
- [Rust Host Guide](rust.md)
