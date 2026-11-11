// The TypeScript type of a Risor type (05 E.1 row 5, ruling R202): a
// convenience of this binding only, in the types and not at run time.
//
// `Infer<X>` reads a `Type<T>` (T), or type text (05-types.md 3.1) parsed by
// the type checker of TypeScript, so a host writes a type once:
//
//   const env = createEnv({ vars: { user: "{name: string, age: int}" } });
//   env.compile(x).run({ user: { name: "Ada", age: 36 } }); // checked by tsc
//
// The TypeScript type is the shape of the value as a host gives it and as
// `decode` and `conform` read it: a record is a plain object, a list an array,
// `int` and `float` are `number`, `any` is `unknown`, a literal set is a union
// of string literals, `T | null` is `T | null`, and `f?: T` is an optional
// property. An opaque type is the type its test guards (`test(v): v is P`), or
// `unknown`.
//
// The parser here is not the language's. It reads the declared form of type
// text: no signatures and no type variables. Where it cannot read a text it
// gives `unknown`, never an error: the environment refuses a text that is not
// type text with `bad-type` when it is declared, so a host is told there.
// Two places are wider or narrower than the checker's type:
//   - a string with an escape is `string`, not the literal;
//   - lists among the alternatives are joined one level deep:
//     `int[] | string[]` is `(number | string)[]`, as 05-types.md 3.4 says,
//     but `int[][] | string[][]` is `(number[] | string[])[]`.

import type { Type } from "./type.ts";

/**
 * The TypeScript type of a Risor type: of a `Type<T>`, or of type text. `Names`
 * gives the TypeScript type of each opaque type name; a name it does not hold
 * is `unknown`.
 */
export type Infer<X, Names = {}> = X extends Type<infer T> ? T : X extends string ? InferText<X, Names> : unknown;

/** The TypeScript type of type text, or `unknown` when the text is not read. */
type InferText<S extends string, N> = string extends S
  ? unknown
  : ParseType<S, N> extends [infer T, infer R extends string]
    ? Skip<R> extends ""
      ? T
      : unknown
    : unknown;

// ---- Tokens ---------------------------------------------------------------------------

type Space = " " | "\n" | "\t" | "\r";

/** Drop white space and comments (01-language.md). */
type Skip<S extends string> = S extends `${Space}${infer R}`
  ? Skip<R>
  : S extends `//${string}\n${infer R}`
    ? Skip<R>
    : S extends `//${string}`
      ? ""
      : S extends `/*${string}*/${infer R}`
        ? Skip<R>
        : S;

type Upper = "A" | "B" | "C" | "D" | "E" | "F" | "G" | "H" | "I" | "J" | "K" | "L" | "M" | "N" | "O" | "P" | "Q" | "R" | "S" | "T" | "U" | "V" | "W" | "X" | "Y" | "Z";
type Digit = "0" | "1" | "2" | "3" | "4" | "5" | "6" | "7" | "8" | "9";
type WordChar = Upper | Lowercase<Upper> | Digit | "_";

/** The longest word at the start of S, and what follows it. */
type Word<S extends string, Acc extends string = ""> = S extends `${infer C}${infer R}`
  ? C extends WordChar
    ? Word<R, `${Acc}${C}`>
    : [Acc, S]
  : [Acc, S];

/** A string token's literal type; a string with an escape is `string`. */
type Lit<B extends string> = B extends `${string}\\${string}` ? string : B;

/** A string token at the start of S: its type and what follows. */
type StringToken<S extends string> = S extends `"${infer B}"${infer R}`
  ? [Lit<B>, R]
  : S extends `'${infer B}'${infer R}`
    ? [Lit<B>, R]
    : Fail;

declare const fail: unique symbol;
type Fail = typeof fail;

// ---- Types ----------------------------------------------------------------------------

interface Prims {
  null: null;
  bool: boolean;
  int: number;
  float: number;
  string: string;
  any: unknown;
  never: never;
}

/**
 * Type = Alt { "|" Alt }. The lists among the alternatives are kept apart in
 * `L` (`[]` for none, `[E]` for elements E) so they join into one list type.
 */
type ParseType<S extends string, N, L extends unknown[] = [], O = never> =
  ParseAlt<Skip<S>, N> extends [infer A, infer R extends string]
    ? AddAlt<A, L, O> extends [infer L2 extends unknown[], infer O2]
      ? Skip<R> extends `|${infer R2}`
        ? ParseType<R2, N, L2, O2>
        : [O2 | (L2 extends [infer E] ? E[] : never), R]
      : Fail
    : Fail;

type AddAlt<A, L extends unknown[], O> = A extends ["list", infer E]
  ? [L extends [infer P] ? [P | E] : [E], O]
  : A extends ["one", infer T]
    ? [L, O | T]
    : Fail;

/** Alt = Prim { "[" "]" }: ["list", element] or ["one", type]. */
type ParseAlt<S extends string, N> = ParsePrim<S, N> extends [infer P, infer R extends string] ? Suffix<P, R, false> : Fail;

type Suffix<T, S extends string, IsList extends boolean> =
  Skip<S> extends `[${infer R}`
    ? Skip<R> extends `]${infer R2}`
      ? IsList extends true
        ? Suffix<T[], R2, true>
        : Suffix<T, R2, true>
      : Fail
    : [IsList extends true ? ["list", T] : ["one", T], S];

type ParsePrim<S extends string, N> = S extends `(${infer R}`
  ? ParseType<R, N> extends [infer T, infer R2 extends string]
    ? Skip<R2> extends `)${infer R3}`
      ? [T, R3]
      : Fail
    : Fail
  : S extends `{${infer R}`
    ? ParseRecord<Skip<R>, N, {}, {}>
    : S extends `"${string}` | `'${string}`
      ? StringToken<S>
      : Word<S> extends [infer W extends string, infer R extends string]
        ? W extends "map"
          ? ParseMap<Skip<R>, N>
          : W extends keyof Prims
            ? [Prims[W], R]
            : W extends `${Upper}${string}`
              ? [W extends keyof N ? N[W] : unknown, R]
              : Fail
        : Fail;

type ParseMap<S extends string, N> = S extends `<${infer R}`
  ? ParseType<R, N> extends [infer T, infer R2 extends string]
    ? Skip<R2> extends `>${infer R3}`
      ? [{ [k: string]: T }, R3]
      : Fail
    : Fail
  : Fail;

// ---- Records --------------------------------------------------------------------------

/** The fields so far: required in Req, optional in Opt. */
type ParseRecord<S extends string, N, Req, Opt> = S extends `}${infer R}`
  ? [Rec<Req, Opt, never>, R]
  : S extends `...${infer R}`
    ? ParseRest<Skip<R>, N, Req, Opt>
    : Key<S> extends [infer K extends string, infer R extends string]
      ? Skip<R> extends `?${infer R2}`
        ? ParseField<Skip<R2>, N, Req, Opt, K, true>
        : ParseField<Skip<R>, N, Req, Opt, K, false>
      : Fail;

type Key<S extends string> = S extends `"${string}` | `'${string}`
  ? StringToken<S> extends [infer K extends string, infer R extends string]
    ? string extends K
      ? Fail
      : [K, R]
    : Fail
  : Word<S> extends [infer W extends string, infer R extends string]
    ? W extends ""
      ? Fail
      : [W, R]
    : Fail;

type ParseField<S extends string, N, Req, Opt, K extends string, Optional extends boolean> = S extends `:${infer R}`
  ? ParseType<R, N> extends [infer T, infer R2 extends string]
    ? Optional extends true
      ? AfterField<Skip<R2>, N, Req, Opt & { [P in K]?: T }>
      : AfterField<Skip<R2>, N, Req & { [P in K]: T }, Opt>
    : Fail
  : Fail;

type AfterField<S extends string, N, Req, Opt> = S extends `,${infer R}`
  ? ParseRecord<Skip<R>, N, Req, Opt>
  : S extends `}${infer R}`
    ? [Rec<Req, Opt, never>, R]
    : Fail;

/** Rest = "..." [ Type ], then an optional "," and "}". */
type ParseRest<S extends string, N, Req, Opt> = S extends `,${string}` | `}${string}`
  ? EndRecord<S, Rec<Req, Opt, unknown>>
  : ParseType<S, N> extends [infer T, infer R extends string]
    ? EndRecord<Skip<R>, Rec<Req, Opt, T>>
    : Fail;

type EndRecord<S extends string, T> = S extends `,${infer R}`
  ? Skip<R> extends `}${infer R2}`
    ? [T, R2]
    : Fail
  : S extends `}${infer R}`
    ? [T, R]
    : Fail;

type Flat<T> = { [K in keyof T]: T[K] } & {};

/** Internal: the TypeScript type of `recordType(fields, rest)`. */
export type RecordOf<F extends readonly { key: string; type: Type<any>; optional: boolean }[], R> = Rec<
  { [E in F[number] as E["optional"] extends false ? E["key"] : never]: E["type"] extends Type<infer V> ? V : never },
  { [E in F[number] as E["optional"] extends false ? never : E["key"]]?: E["type"] extends Type<infer V> ? V : never },
  R
>;

/**
 * A record's TypeScript type. An exact record (rest `never`) is its fields;
 * a record with a rest also takes other keys, of the rest's type.
 */
type Rec<Req, Opt, Rest> = [Rest] extends [never]
  ? keyof (Req & Opt) extends never
    ? { [k: string]: never }
    : Flat<Req & Opt>
  : keyof (Req & Opt) extends never
    ? { [k: string]: Rest }
    : Flat<Req & Opt> & { [k: string]: Rest | (Req & Opt)[keyof (Req & Opt)] };

// ---- Environments ---------------------------------------------------------------------

/** The variables of an environment with no declarations the types can read. */
export type AnyVars = { readonly [name: string]: unknown };

/**
 * Internal: the variables an environment declares, by name, with the
 * TypeScript type of each: from `vars` (M) and `varRecord` (R), both type text;
 * `varTypes` (T), each a `Type`; and `varSchemas` (S), each `unknown`. Every
 * one may be left out: a run fails with `missing-variable` only when the
 * program reads one it was not given. With no declarations, any variables.
 */
export type VarsOf<M, R, T, S, N> = [keyof M | keyof T | keyof S | R] extends [never]
  ? AnyVars
  : Partial<
      Flat<
        { [K in keyof M]: M[K] extends string ? InferText<M[K], N> : unknown } &
          ([R] extends [never] ? {} : R extends string ? RecordVars<InferText<R, N>> : {}) &
          { [K in keyof T]: Infer<T[K]> } &
          { [K in keyof S]: unknown }
      >
    >;

/** The variables of a record type; any variables when it is not a record. */
type RecordVars<T> = unknown extends T ? AnyVars : T extends object ? T : AnyVars;

/** Internal: the variables of a child environment: the parent's, with the child's over them. */
export type ChildVars<V, W> = string extends keyof W ? V : string extends keyof V ? W : Flat<Omit<V, keyof W> & W>;

/**
 * Internal: what a host may give where a value of T is read: as T, with a
 * `Map` for a record and a `bigint` for a number (03 3.4.4).
 */
export type HostInput<T> = T extends number
  ? number | bigint
  : T extends readonly (infer E)[]
    ? readonly HostInput<E>[]
    : T extends (...args: any[]) => unknown
      ? T
      : T extends object
        ? { [K in keyof T]: HostInput<T[K]> } | ReadonlyMap<string, unknown>
        : T;

/** Internal: the variables a run takes, as a host gives them. */
export type HostVars<V> = string extends keyof V ? V : { [K in keyof V]: HostInput<V[K]> };
