// A tree of Risor's JSON text that keeps each number's text, for a result
// view. JSON.parse would turn `1.0` into 1 and lose an int beyond 2^53.

export type JsonNode =
  | { kind: "null" }
  | { kind: "bool"; value: boolean }
  | { kind: "number"; text: string }
  | { kind: "string"; value: string }
  | { kind: "array"; items: JsonNode[] }
  | { kind: "object"; entries: [string, JsonNode][] };

/** Parse JSON text into a tree. Throws SyntaxError for text that is not JSON. */
export function parseJsonTree(text: string): JsonNode {
  let i = 0;
  const fail = (): never => {
    throw new SyntaxError(`not JSON at offset ${i}`);
  };
  const ws = () => {
    while (i < text.length && (text[i] === " " || text[i] === "\n" || text[i] === "\r" || text[i] === "\t")) i++;
  };
  const str = (): string => {
    const start = i;
    i++;
    while (i < text.length && text[i] !== '"') i += text[i] === "\\" ? 2 : 1;
    if (i >= text.length) fail();
    i++;
    return JSON.parse(text.slice(start, i)) as string;
  };
  const value = (): JsonNode => {
    ws();
    const c = text[i];
    if (c === "{") {
      i++;
      const entries: [string, JsonNode][] = [];
      ws();
      if (text[i] === "}") {
        i++;
        return { kind: "object", entries };
      }
      for (;;) {
        ws();
        if (text[i] !== '"') fail();
        const k = str();
        ws();
        if (text[i++] !== ":") fail();
        entries.push([k, value()]);
        ws();
        if (text[i] === ",") i++;
        else if (text[i] === "}") {
          i++;
          return { kind: "object", entries };
        } else fail();
      }
    }
    if (c === "[") {
      i++;
      const items: JsonNode[] = [];
      ws();
      if (text[i] === "]") {
        i++;
        return { kind: "array", items };
      }
      for (;;) {
        items.push(value());
        ws();
        if (text[i] === ",") i++;
        else if (text[i] === "]") {
          i++;
          return { kind: "array", items };
        } else fail();
      }
    }
    if (c === '"') return { kind: "string", value: str() };
    if (text.startsWith("true", i)) {
      i += 4;
      return { kind: "bool", value: true };
    }
    if (text.startsWith("false", i)) {
      i += 5;
      return { kind: "bool", value: false };
    }
    if (text.startsWith("null", i)) {
      i += 4;
      return { kind: "null" };
    }
    const m = /^-?(0|[1-9][0-9]*)(\.[0-9]+)?([eE][+-]?[0-9]+)?/.exec(text.slice(i));
    if (!m) return fail();
    i += m[0].length;
    return { kind: "number", text: m[0] };
  };
  const v = value();
  ws();
  if (i !== text.length) fail();
  return v;
}

/** Indent compact JSON text without parsing its numbers, so each keeps its text. */
export function prettyJSON(text: string, indent = "  "): string {
  let out = "";
  let depth = 0;
  let inString = false;
  for (let i = 0; i < text.length; i++) {
    const ch = text[i];
    if (inString) {
      out += ch;
      if (ch === "\\") out += text[++i];
      else if (ch === '"') inString = false;
      continue;
    }
    switch (ch) {
      case '"':
        inString = true;
        out += ch;
        break;
      case "{":
      case "[": {
        const close = ch === "{" ? "}" : "]";
        if (text[i + 1] === close) {
          out += ch + close;
          i++;
        } else {
          depth++;
          out += ch + "\n" + indent.repeat(depth);
        }
        break;
      }
      case "}":
      case "]":
        depth--;
        out += "\n" + indent.repeat(depth) + ch;
        break;
      case ",":
        out += ",\n" + indent.repeat(depth);
        break;
      case ":":
        out += ": ";
        break;
      default:
        out += ch;
    }
  }
  return out;
}
