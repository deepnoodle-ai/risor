// Long flat programs (01 13.1, 13.2 item 6, f04 2.8, f07 2.5): the forms that
// open no nesting level, so their length must cost no native stack in any
// pass. Each form gives its source at a length, the environment it is checked
// in, the variables it runs on, and what the run gives: the value, the steps
// and the allocation units, each exact at every length.
//
// test/flat.test.ts runs each form at 10,000 links through every entry point.
// A form that writes more than one link for each repeat says how many (`per`),
// and is repeated until it has 10,000 of them.

/** A map whose field `a` is the map itself, and whose field `l` is a list of it. */
function cyclicMap(): Record<string, unknown> {
  const m: Record<string, unknown> = {};
  m.a = m;
  m.l = [m];
  return m;
}

/** A list whose one item is the list itself. */
function cyclicList(): unknown[] {
  const xs: unknown[] = [];
  xs.push(xs);
  return xs;
}

export interface FlatForm {
  name: string;
  /** The variables, as one record type. */
  env: string;
  /** The values of the variables, made new for each run. */
  vars: () => Record<string, unknown>;
  /** The program with `n` repeats. */
  source: (n: number) => string;
  /** The links of one repeat, when it is more than one. */
  per?: number;
  /** Whether `value` is the result of a run on `vars`. */
  gives: (value: unknown, vars: Record<string, unknown>, n: number) => boolean;
  /** The type the checker gives the program. */
  type: string;
  /**
   * The code of the check at `n` repeats, when the program is past a limit of
   * the checker there. `alone`: the static rules find it, so a compile with no
   * check has it too.
   */
  past?: (n: number) => { code: string; alone: boolean } | null;
  steps: (n: number) => number;
  units: (n: number) => number;
  /**
   * What the formatter does with the form at 10,000 links. `text`: the
   * formatted text parses to the same tree. `nested`: the formatter writes a
   * pair of parentheses for the reader around each left operand (f12 2.6), so
   * its text nests deeper than 100 levels and does not parse again. `quadratic`:
   * the document of f12 2.8 indents each `else` branch two more, so the text
   * grows with the square of the length and is not made at this length.
   */
  format: "text" | "nested" | "quadratic";
  /** The kind and the type of hover's record on the last word, where the form states them. */
  hover?: { kind: string; type: string };
}

const same = (want: unknown) => (value: unknown) => JSON.stringify(value) === JSON.stringify(want);
const rep = (s: string, n: number) => s.repeat(n);

/** One round of every kind of step, which gives `m` back from `m`: 14 links. */
const ROUND = `.a?.a["a"]?.["a"].l[0].l?.[0].l[*].a.reverse()?.reverse()[0]`;
const ROUND_LINKS = 14;

const names = (n: number, f: (i: number) => string, sep = ", ") => Array.from({ length: n }, (_, i) => f(i)).join(sep);

export const FORMS: FlatForm[] = [
  // ---- chains of one kind of link -------------------------------------------------
  {
    name: "as",
    env: "{y: string}",
    vars: () => ({ y: "a" }),
    source: (n) => "y" + rep(" as any", n),
    gives: same("a"),
    type: "any",
    steps: () => 0,
    units: () => 0,
    format: "text",
  },
  {
    // `is` gives a bool, and `bool is T` is a constant condition, so `is` chains through `as any`.
    name: "is",
    env: "{y: string}",
    vars: () => ({ y: "a" }),
    source: (n) => "y" + rep(" as any is bool", n),
    per: 2,
    gives: (v, _vars, n) => v === n > 1,
    type: "bool",
    steps: (n) => n,
    units: () => 0,
    format: "text",
  },
  {
    name: "projection and method call",
    env: "{y: string}",
    vars: () => ({ y: "a" }),
    source: (n) => "[[y]]" + rep("[*].map(x => x)", n),
    per: 2,
    gives: same([["a"]]),
    type: "string[][]",
    steps: (n) => 5 * n,
    units: (n) => 4 * n + 4,
    format: "text",
  },
  {
    name: "method call",
    env: "{y: string}",
    vars: () => ({ y: "a" }),
    source: (n) => "y" + rep(".trim()", n),
    gives: same("a"),
    type: "string",
    steps: (n) => n,
    units: (n) => n,
    format: "text",
    hover: { kind: "method", type: "string" },
  },
  {
    name: "optional method call",
    env: "{y: string | null}",
    vars: () => ({ y: "a" }),
    source: (n) => "y" + rep("?.trim()", n),
    gives: same("a"),
    type: "string | null",
    steps: (n) => n,
    units: (n) => n,
    format: "text",
  },
  {
    name: "member",
    env: "{m: any}",
    vars: () => ({ m: cyclicMap() }),
    source: (n) => "m" + rep(".a", n),
    gives: (v, vars) => v === vars.m,
    type: "any",
    steps: (n) => n,
    units: () => 0,
    format: "text",
  },
  {
    name: "optional member",
    env: "{m: any}",
    vars: () => ({ m: cyclicMap() }),
    source: (n) => "m" + rep("?.a", n),
    gives: (v, vars) => v === vars.m,
    type: "any",
    steps: (n) => n,
    units: () => 0,
    format: "text",
  },
  {
    name: "index",
    env: "{xs: any}",
    vars: () => ({ xs: cyclicList() }),
    source: (n) => "xs" + rep("[0]", n),
    gives: (v, vars) => v === vars.xs,
    type: "any",
    steps: (n) => n,
    units: () => 0,
    format: "text",
  },
  {
    name: "optional index",
    env: "{xs: any}",
    vars: () => ({ xs: cyclicList() }),
    source: (n) => "xs" + rep("?.[0]", n),
    gives: (v, vars) => v === vars.xs,
    type: "any",
    steps: (n) => n,
    units: () => 0,
    format: "text",
  },
  {
    name: "index by a string",
    env: "{m: any}",
    vars: () => ({ m: cyclicMap() }),
    source: (n) => "m" + rep(`["a"]`, n),
    gives: (v, vars) => v === vars.m,
    type: "any",
    steps: (n) => n,
    units: () => 0,
    format: "text",
  },
  {
    name: "reads of one projection",
    env: "{m: any}",
    vars: () => ({ m: cyclicMap() }),
    source: (n) => "m.l[*]" + rep(".a", n),
    gives: (v, vars) => Array.isArray(v) && v.length === 1 && v[0] === vars.m,
    type: "any[]",
    steps: (n) => n + 2,
    units: () => 2,
    format: "text",
  },
  // ---- every kind of link in one chain --------------------------------------------
  {
    name: "every step",
    env: "{m: any}",
    vars: () => ({ m: cyclicMap() }),
    source: (n) => "m" + rep(ROUND, n),
    per: ROUND_LINKS,
    gives: (v, vars) => v === vars.m,
    type: "any",
    steps: (n) => 16 * n,
    units: (n) => 6 * n,
    format: "text",
  },
  {
    // `in` and `as` have one level (01 3.3, f04 2.3), so each is the left operand of the next.
    name: "as and a binary operator",
    env: "{c: bool}",
    vars: () => ({ c: true }),
    source: (n) => "c" + rep(" as any in [true]", n),
    per: 2,
    gives: same(true),
    type: "bool",
    steps: (n) => n,
    units: (n) => 2 * n,
    format: "text",
  },
  {
    name: "every step, as, is and operators",
    env: "{m: any, c: bool}",
    vars: () => ({ m: cyclicMap(), c: true }),
    source: (n) => "m" + rep(ROUND, n) + rep(" as any in [true] as any is bool", n) + rep(" && c", n),
    per: ROUND_LINKS + 5,
    gives: same(true),
    type: "bool",
    steps: (n) => 18 * n,
    units: (n) => 8 * n,
    format: "text",
  },
  // ---- the forms 01 13.2 item 6 names -----------------------------------------------
  {
    name: "sum",
    env: "{}",
    vars: () => ({}),
    source: (n) => "1" + rep(" + 1", n),
    gives: (v, _vars, n) => v === n + 1,
    type: "int",
    steps: (n) => n,
    units: () => 0,
    format: "text",
  },
  {
    name: "and",
    env: "{c: bool}",
    vars: () => ({ c: true }),
    source: (n) => "c" + rep(" && c", n),
    gives: same(true),
    type: "bool",
    steps: () => 0,
    units: () => 0,
    format: "text",
  },
  {
    name: "coalesce",
    env: "{z: int | null}",
    vars: () => ({ z: null }),
    source: (n) => "z" + rep(" ?? z", n) + " ?? 1",
    gives: same(1),
    type: "int",
    steps: () => 0,
    units: () => 0,
    format: "text",
  },
  {
    name: "equality",
    env: "{c: any}",
    vars: () => ({ c: true }),
    source: (n) => "c" + rep(" == c", n),
    gives: same(true),
    type: "bool",
    steps: (n) => n,
    units: () => 0,
    format: "nested",
  },
  {
    name: "product and quotient",
    env: "{}",
    vars: () => ({}),
    source: (n) => "1" + rep(" * 2 / 2", n),
    per: 2,
    gives: same(1),
    type: "float",
    steps: (n) => 2 * n,
    units: () => 0,
    format: "nested",
  },
  {
    name: "else branches of ? :",
    env: "{c: bool}",
    vars: () => ({ c: false }),
    source: (n) => rep("c ? 1 : ", n) + "2",
    gives: same(2),
    type: "int",
    steps: () => 0,
    units: () => 0,
    format: "quadratic",
  },
  {
    name: "else if",
    env: "{c: bool}",
    vars: () => ({ c: false }),
    source: (n) => "if (c) { 1 } " + rep("else if (c) { 2 } ", n) + "else { 3 }",
    gives: same(3),
    type: "int",
    steps: () => 0,
    units: () => 0,
    format: "text",
  },
  {
    name: "const lines",
    env: "{}",
    vars: () => ({}),
    source: (n) => Array.from({ length: n }, (_, i) => `const a${i} = ${i};`).join(" ") + ` a${n - 1}`,
    gives: (v, _vars, n) => v === n - 1,
    type: "int",
    steps: () => 0,
    units: () => 0,
    format: "text",
  },
  // ---- the next item, entry, arm and part (01 13.1) ----------------------------------
  {
    name: "items",
    env: "{}",
    vars: () => ({}),
    source: (n) => "len([" + rep("1, ", n) + "1])",
    gives: (v, _vars, n) => v === n + 1,
    type: "int",
    steps: () => 1,
    units: (n) => n + 2,
    format: "text",
  },
  {
    name: "entries",
    env: "{}",
    vars: () => ({}),
    source: (n) => "len({" + Array.from({ length: n }, (_, i) => `k${i}: ${i}`).join(", ") + "})",
    gives: (v, _vars, n) => v === n,
    type: "int",
    // 05 20.7: the type of the map has one field for each entry, and a type of a size over 100,000 is `check-limit`.
    past: (n) => (n >= 100000 ? { code: "check-limit", alone: false } : null),
    steps: () => 1,
    units: (n) => 2 * n + 1,
    format: "text",
  },
  {
    name: "arms",
    env: "{y: string}",
    vars: () => ({ y: "a" }),
    source: (n) => "match y { " + Array.from({ length: n }, (_, i) => `"k${i}" => ${i}, `).join("") + "_ => -1 }",
    gives: same(-1),
    type: "int",
    steps: (n) => n + 2,
    units: () => 0,
    format: "text",
  },
  {
    name: "parts of a template",
    env: "{y: string}",
    vars: () => ({ y: "a" }),
    source: (n) => "len(`" + rep("${y}", n) + "`)",
    gives: (v, _vars, n) => v === n,
    type: "int",
    steps: (n) => 2 + 2 * Math.floor(n / 64),
    units: (n) => 1 + Math.floor(n / 64),
    format: "text",
  },
  {
    name: "arguments",
    env: "{}",
    vars: () => ({}),
    source: (n) => "max(" + rep("1, ", n) + "2)",
    gives: same(2),
    type: "int",
    steps: (n) => n + 1,
    units: () => 0,
    format: "text",
  },
  {
    name: "parameters",
    env: "{}",
    vars: () => ({}),
    source: (n) => "const f = (" + names(n, (i) => `a${i} = ${i}`) + ") => a0; f()",
    gives: same(0),
    type: "int",
    steps: () => 2,
    units: () => 0,
    format: "text",
  },
  {
    name: "fields of a pattern",
    env: "{m: map<int>}",
    vars: () => ({ m: {} }),
    source: (n) => "const {" + names(n, (i) => `k${i} = ${i}`) + "} = m; k0",
    gives: same(0),
    type: "int",
    steps: (n) => n,
    units: () => 0,
    format: "text",
  },
  {
    name: "elements of a pattern",
    env: "{xs: int[]}",
    vars: () => ({ xs: [] }),
    source: (n) => "const [" + names(n, (i) => `a${i} = ${i}`) + "] = xs; a0",
    gives: same(0),
    type: "int",
    steps: (n) => n,
    units: () => 0,
    format: "text",
  },
  {
    name: "literals of an arm",
    env: "{y: string}",
    vars: () => ({ y: "a" }),
    source: (n) => "match y { " + names(n, (i) => `"k${i}"`, " | ") + " => 1, _ => 2 }",
    gives: same(2),
    type: "int",
    steps: () => 2,
    units: () => 0,
    format: "text",
  },
  // ---- a program type (f04 2.8 item 1) and the type lines ------------------------------
  {
    name: "alternatives of a program type",
    env: "{y: any}",
    vars: () => ({ y: "a" }),
    source: (n) => "y is " + names(n, (i) => `"k${i}"`, " | "),
    gives: same(false),
    type: "bool",
    steps: () => 1,
    units: () => 0,
    format: "text",
  },
  {
    name: "fields of a program type",
    env: "{m: any}",
    vars: () => ({ m: {} }),
    source: (n) => "m is {" + names(n, (i) => `k${i}?: int`) + "}",
    gives: same(true),
    type: "bool",
    // 05 20.7: the size of a program type, by the static rules.
    past: (n) => (n >= 100000 ? { code: "check-limit", alone: true } : null),
    steps: (n) => n + 1,
    units: () => 0,
    format: "text",
  },
  {
    name: "type lines",
    env: "{}",
    vars: () => ({}),
    source: (n) => names(n, (i) => `type T${i} = int;`, " ") + " 1",
    gives: same(1),
    type: "int",
    steps: () => 0,
    units: () => 0,
    format: "text",
  },
  // ---- what is no part of the tree (01 13.1) -----------------------------------------
  {
    name: "comments",
    env: "{}",
    vars: () => ({}),
    source: (n) => rep("/* c */ ", n) + "1",
    gives: same(1),
    type: "int",
    steps: () => 0,
    units: () => 0,
    format: "text",
  },
];
