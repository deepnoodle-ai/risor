// Library entries (04-library.md section 5). Stage S2 brings the few that
// exercise the call paths: fail, len, type, string, map, filter. S3 adds the rest.

import { RunFailure } from "../eval/failure.ts";
import { render } from "../eval/ops.ts";
import { cpLength, mapSize, size, truthy } from "../value/value.ts";
import { implement } from "./registry.ts";
import type { Callback } from "./registry.ts";

implement(null, "fail", (c) => {
  throw new RunFailure("failed", c.span, null, {}, { text: c.args[0] as string, data: c.args.length > 1 ? c.args[1] : null });
});

implement(null, "len", (c) => {
  const x = c.args[0];
  switch (c.run.kind(x)) {
    case "string":
      c.run.charge(size(x as string));
      return cpLength(x as string);
    case "list":
      return (x as unknown[]).length;
    default:
      return mapSize(x as object);
  }
});

implement(null, "type", (c) => c.run.kind(c.args[0]));

implement(null, "string", (c) => {
  const x = c.args[0];
  if (c.run.kind(x) === "string") return x;
  const s = render(c.run, x, c.span);
  c.run.charge(size(s));
  c.run.alloc(1 + size(s));
  return s;
});

implement("list", "map", (c) => {
  const xs = c.receiver as unknown[];
  const f = c.args[0] as Callback;
  const out: unknown[] = [];
  for (let i = 0; i < xs.length; i++) {
    c.run.charge(2);
    const e = c.run.read(xs[i], c.span);
    out.push(f.call([e, i]));
  }
  c.run.alloc(1 + xs.length);
  return out;
});

implement("list", "filter", (c) => {
  const xs = c.receiver as unknown[];
  const f = c.args[0] as Callback;
  const out: unknown[] = [];
  for (let i = 0; i < xs.length; i++) {
    c.run.charge(2);
    const e = c.run.read(xs[i], c.span);
    if (truthy(f.call([e, i]), c.run.cls)) out.push(e);
  }
  c.run.alloc(1 + out.length);
  return out;
});
