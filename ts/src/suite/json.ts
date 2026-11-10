// A strict JSON reader for case files (03 3.10.1, 3.10.10).
//
// JSON.parse is not enough: a case file that repeats a key in an object is
// invalid, and every number is read by the rule of text to number of
// 01 6.3, where a number that reads as an infinity makes the file invalid.

import { parseDecimal } from "../value/number.ts";

export type Json = null | boolean | number | string | Json[] | JsonObject;
export interface JsonObject {
  [key: string]: Json;
}

export class JsonError extends Error {}

export function isObject(v: Json | undefined): v is JsonObject {
  return typeof v === "object" && v !== null && !Array.isArray(v);
}

/** Read JSON text. Throws JsonError. Objects have a null prototype. */
export function readJson(text: string): Json {
  return new Reader(text).document();
}

class Reader {
  private i = 0;
  private readonly s: string;

  constructor(s: string) {
    this.s = s;
  }

  private fail(msg: string): never {
    throw new JsonError(`${msg} at offset ${this.i}`);
  }

  private ws(): void {
    const s = this.s;
    while (this.i < s.length) {
      const c = s.charCodeAt(this.i);
      if (c === 0x20 || c === 0x09 || c === 0x0a || c === 0x0d) this.i++;
      else break;
    }
  }

  document(): Json {
    if (this.s.charCodeAt(0) === 0xfeff) this.i = 1;
    this.ws();
    const v = this.value(0);
    this.ws();
    if (this.i !== this.s.length) this.fail("text after the JSON value");
    return v;
  }

  private value(depth: number): Json {
    const c = this.s[this.i];
    switch (c) {
      case "{":
        return this.object(depth);
      case "[":
        return this.array(depth);
      case '"':
        return this.string();
      case "t":
        return this.literal("true", true);
      case "f":
        return this.literal("false", false);
      case "n":
        return this.literal("null", null);
      default:
        if (c === "-" || (c >= "0" && c <= "9")) return this.number();
        this.fail("unexpected character");
    }
  }

  private literal<T extends Json>(word: string, v: T): T {
    if (this.s.startsWith(word, this.i)) {
      this.i += word.length;
      return v;
    }
    this.fail("unexpected word");
  }

  private number(): number {
    const m = /^-?(?:0|[1-9][0-9]*)(?:\.[0-9]+)?(?:[eE][+-]?[0-9]+)?/.exec(this.s.slice(this.i, this.i + 400));
    if (m === null) this.fail("bad number");
    let text = m[0];
    if (text.length === 400) {
      const m2 = /^-?(?:0|[1-9][0-9]*)(?:\.[0-9]+)?(?:[eE][+-]?[0-9]+)?/.exec(this.s.slice(this.i));
      text = m2![0];
    }
    this.i += text.length;
    const x = parseDecimal(text)!;
    if (!Number.isFinite(x)) this.fail("a number that reads as an infinity");
    return x + 0;
  }

  private string(): string {
    const s = this.s;
    this.i++; // "
    let out = "";
    let from = this.i;
    for (;;) {
      if (this.i >= s.length) this.fail("unterminated string");
      const c = s.charCodeAt(this.i);
      if (c === 0x22) {
        out += s.slice(from, this.i);
        this.i++;
        // Ruling V4: a lone surrogate reads as U+FFFD.
        return out.isWellFormed() ? out : out.toWellFormed();
      }
      if (c < 0x20) this.fail("control character in a string");
      if (c === 0x5c) {
        out += s.slice(from, this.i);
        const e = s[this.i + 1];
        const simple: Record<string, string> = { '"': '"', "\\": "\\", "/": "/", b: "\b", f: "\f", n: "\n", r: "\r", t: "\t" };
        if (e in simple) {
          out += simple[e];
          this.i += 2;
        } else if (e === "u") {
          const hex = s.slice(this.i + 2, this.i + 6);
          if (!/^[0-9a-fA-F]{4}$/.test(hex)) this.fail("bad \\u escape");
          out += String.fromCharCode(parseInt(hex, 16));
          this.i += 6;
        } else {
          this.fail("bad escape");
        }
        from = this.i;
        continue;
      }
      this.i++;
    }
  }

  private array(depth: number): Json[] {
    // 03 3.10.1 item 4 (ruling AA6): at most 1,024 levels.
    if (depth >= 1024) this.fail("JSON nested deeper than 1,024 levels");
    this.i++;
    const out: Json[] = [];
    this.ws();
    if (this.s[this.i] === "]") {
      this.i++;
      return out;
    }
    for (;;) {
      this.ws();
      out.push(this.value(depth + 1));
      this.ws();
      const c = this.s[this.i];
      if (c === ",") {
        this.i++;
        continue;
      }
      if (c === "]") {
        this.i++;
        return out;
      }
      this.fail("expected , or ]");
    }
  }

  private object(depth: number): JsonObject {
    if (depth >= 1024) this.fail("JSON nested deeper than 1,024 levels");
    this.i++;
    const out: JsonObject = Object.create(null);
    this.ws();
    if (this.s[this.i] === "}") {
      this.i++;
      return out;
    }
    for (;;) {
      this.ws();
      if (this.s[this.i] !== '"') this.fail("expected a key");
      const key = this.string();
      if (Object.prototype.hasOwnProperty.call(out, key)) this.fail(`the key ${JSON.stringify(key)} is repeated`);
      this.ws();
      if (this.s[this.i] !== ":") this.fail("expected :");
      this.i++;
      this.ws();
      out[key] = this.value(depth + 1);
      this.ws();
      const c = this.s[this.i];
      if (c === ",") {
        this.i++;
        continue;
      }
      if (c === "}") {
        this.i++;
        return out;
      }
      this.fail("expected , or }");
    }
  }
}
