// The deepest programs within the depth limit (02 8.4) whose calls go
// through a library entry that calls a lambda: the forms that spend the most
// native stack for each unit of the depth count. test/fixtures/deepest.ts runs
// them in Node, and web/examples/react-vite/e2e/deep.spec.ts in a browser's
// Web Worker.
//
// Each form is a chain of `const`-bound lambdas, f1 calling f0 and so on, a
// stand-in for recursion. Every call is at nesting depth 0 within its body,
// so each link adds 1 to the depth count, and the top, a call by name, adds
// 1: with the depth limit D, the chain has D lambdas and the count reaches D
// exactly at f0. The programs are compiled with `check: false`, because a
// checked program cannot nest calls this deep (02 8.4 item 10).

/** The variables of every form, as one record type. */
export const VARS = "{y: any, c: any}";

/** The values of the variables. */
export const VALUES = { y: 1, c: false };

export interface DeepForm {
  name: string;
  /** f0. */
  base: string;
  /** f_i, given the name of f_(i-1). */
  link: (prev: string) => string;
  /** The value of the program. */
  expect: unknown;
}

export const FORMS: DeepForm[] = [
  { name: "map", base: "x => x", link: (f) => `x => [x].map(${f})[0]`, expect: 1 },
  { name: "flatMap", base: "x => [x]", link: (f) => `x => [x].flatMap(${f})`, expect: [1] },
  {
    // The most stack for each level, as measured (src/index.ts, STACK): a
    // parameter with a default, `? :` at the root of the body, and every
    // kind of operand that opens no level between the root and the call:
    // the receiver of steps, the left operand of `**`, of `*`, of `as` and of
    // `||`. sortBy keeps each key and element as it calls.
    name: "heaviest",
    base: "(x = 0) => x",
    link: (f) => `(x = 0) => c ? 0 : [x].sortBy(${f}).reverse().reverse()[0] ** 1 * 1 as any || 0 || 0`,
    expect: 1,
  },
];

/**
 * The program of `form` whose depth count reaches `depth`: `depth` lambdas.
 * With `bottom`, f0 is `x => bottom()` instead of the form's own.
 */
export function deepSource(form: DeepForm, depth: number, bottom?: string): string {
  let src = `const f0 = ${bottom === undefined ? form.base : `x => ${bottom}()`};\n`;
  for (let i = 1; i < depth; i++) src += `const f${i} = ${form.link(`f${i - 1}`)};\n`;
  return src + `f${depth - 1}(y)`;
}
