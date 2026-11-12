# risor

> **Risor v3 is an alpha and a request for comment.** The language and this
> API will change before 3.0.

Risor is a small expression language that a host program embeds. The host
gives it data and gets a value back. Typical uses are a condition in a
workflow step, a template, a query over JSON, and a config file.

This package is the TypeScript implementation. It has no dependencies, imports
no Node module, and runs in Node 22.18 and later, Deno, Bun and browsers. The
same language has implementations in Go and Rust; a conformance suite makes
the three agree.

```sh
npm install risor
```

## Run a program

```ts
import { createEnv } from "risor";

const env = createEnv({ vars: { order: "{total: float, items: {sku: string}[]}" } });
const program = env.compile(`order.total > 100 && len(order.items) > 0`);

program.run({ order: { total: 120, items: [{ sku: "A1" }] } }); // true
```

`compile` checks the program against the declared variables. A checked program
cannot fail with an unknown name, a missing field, a null or a wrong kind. A
program that does not check throws a `RisorError` with its diagnostics:

```ts
import { RisorError } from "risor";

try {
  env.compile(`order.totl > 100`);
} catch (e) {
  if (e instanceof RisorError) console.log(e.diagnostics[0].code); // "no-field"
}
```

## Limits

Steps, allocation, a deadline and a depth limit bound every run:

```ts
const env = createEnv({ vars: { n: "int" }, limits: { steps: 10_000, allocation: 100_000, deadlineMs: 50 } });
try {
  env.compile("range(n).map(i => i * i)").run({ n: 1_000_000 });
} catch (e) {
  if (e instanceof RisorError) console.log(e.diagnostics[0].code); // "steps-exceeded": a limit error, never a hung page
}
```

## Tools for editors

| Call | Does |
| --- | --- |
| `format(text)` | The one formatted text of a program |
| `env.complete(text, { line, column })` | Completion candidates at a position of a text that need not parse |
| `env.hover(text, { line, column })` | The type, signature and doc of the name at a position |
| `env.describe()`, `Env.fromDescription(json)` | An environment as JSON, so a browser can check text against a server's environment |
| `tokens(text)`, from `risor/tokens` | Coloured ranges for an editor, from the lexer only (about 12 KB gzipped). A call of this binding, not of the specification |

Positions count code points, not UTF-16 units.

## Links

- Specification and conformance suite: https://github.com/deepnoodle-ai/risor/tree/main/spec
- Go implementation: `github.com/deepnoodle-ai/risor/v3`
- License: Apache-2.0
