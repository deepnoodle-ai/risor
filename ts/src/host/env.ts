// The environment (03 3.3) and its declaration errors (03 3.3.4, 05 19.5).
//
// An implementation with no type checker parses every type text by the
// grammar (ruling A2) and applies the rules of names. The rules beyond the
// grammar (two records, an undeclared type name, depth, a key written twice)
// need the type checker and are not applied here.

import type { Diagnostic } from "../syntax/diagnostic.ts";
import { isName, isReserved } from "../syntax/token.ts";
import { parseEnvRecord, parseSignature, parseType } from "../types/typetext.ts";
import type { SigNode, TypeNode } from "../types/typetext.ts";
import { ANY, depth, fromNode, TypeError as TypeRuleError } from "../types/type.ts";
import type { Type } from "../types/type.ts";
import { SchemaError, typeOfSchema } from "../types/schema.ts";
import { FUNCTIONS, METHODS } from "../library/registry.ts";
import type { SchemaOption } from "../types/schema.ts";

/** The newest version of the specification this implementation offers. */
export const VERSION = "3.0";
export const SUPPORTED_VERSIONS: ReadonlySet<string> = new Set(["3.0"]);

/** The six check classes (05 17). */
export const CHECK_CLASSES: ReadonlySet<string> = new Set(["constant-condition", "dynamic", "index", "divide", "value", "host"]);

/** Namespaces reserved for the library (03 3.3.3 item 3). */
export const RESERVED_NAMESPACES: ReadonlySet<string> = new Set(["json", "time", "math", "base64"]);

export interface Limits {
  steps: number | null; // null: off
  allocation: number | null;
  depth: number;
}

export const DEFAULT_LIMITS: Limits = { steps: 1_000_000, allocation: 1_000_000, depth: 200 };

export interface FunctionDecl {
  name: string; // with its namespace, as `geo.distance`
  namespace: string | null;
  sig: SigNode;
  signature: string;
}

export interface MethodDecl {
  type: string;
  name: string;
  sig: SigNode;
  signature: string;
}

/** What a case or a host declares (03 3.3.1), in its portable form. */
export interface Declarations {
  version?: string;
  env?: string; // one record type: the variables
  /** Variables each declared by its own type text, as a description gives them (f01 5.4). */
  variableTexts?: Record<string, string>;
  /** Variables declared by JSON Schema (f01 2). */
  schemas?: Record<string, { schema: unknown; options?: SchemaOption[] }>;
  /** Library entries the environment does not have (03 3.3.1). */
  removed?: string[];
  /** Variables a child environment keeps from its parent, with their types (03 3.3.2). */
  inherited?: ReadonlyMap<string, { node: TypeNode; type: Type | undefined }>;
  opaque?: string[];
  methods?: Record<string, string>;
  functions?: Record<string, string>; // name -> signature
  reject?: string[];
  returns?: string;
  limits?: Partial<Limits>;
}

export interface Environment {
  version: string;
  variables: Map<string, TypeNode>;
  /** The variables' types, by the checker's rules (05 19.5). */
  variableTypes: Map<string, Type>;
  /** The library entries removed (03 3.3.1). */
  removed: Set<string>;
  functions: Map<string, FunctionDecl>;
  namespaces: Set<string>;
  opaque: Set<string>;
  methods: Map<string, MethodDecl>; // key "Type.method"
  reject: Set<string>;
  limits: Limits;
  /** The expected result: a type, the word condition, or nothing. */
  returns: TypeNode | "condition" | null;
  /** The library functions the environment has. */
  library: ReadonlySet<string>;
}

export type DeclareResult = { ok: true; env: Environment } | { ok: false; diagnostics: Diagnostic[] };

function diag(code: string, message: string): Diagnostic {
  return { phase: "declare", code, message };
}

function hasFn(t: SigNode): boolean {
  return t.params.some((p) => p.type.t === "fn");
}

/**
 * Build a child of the base environment from declarations. `library` is the
 * set of library function names the environment has.
 */
export function declare(d: Declarations, library: ReadonlySet<string>): DeclareResult {
  const errs: Diagnostic[] = [];
  const env: Environment = {
    version: VERSION,
    variables: new Map(),
    variableTypes: new Map(),
    removed: new Set(d.removed ?? []),
    functions: new Map(),
    namespaces: new Set(),
    opaque: new Set(),
    methods: new Map(),
    reject: new Set(["constant-condition"]),
    limits: { ...DEFAULT_LIMITS, ...(d.limits ?? {}) },
    returns: null,
    library,
  };

  // Ruling AM4: `bad-limit`.
  for (const [k, v] of Object.entries(d.limits ?? {})) {
    if (k === "depth") {
      if (typeof v !== "number" || !Number.isSafeInteger(v) || v < 1 || v > 1000) {
        errs.push(diag("bad-limit", `the depth limit must be a whole number from 1 to 1,000`));
      }
    } else if (v !== null && (typeof v !== "number" || !Number.isSafeInteger(v) || v < 0)) {
      errs.push(diag("bad-limit", `the ${k} limit must be a whole number of 0 or more, or off`));
    }
  }

  if (d.version !== undefined) {
    if (!SUPPORTED_VERSIONS.has(d.version)) errs.push(diag("unsupported-version", `version ${d.version} is not offered`));
    else env.version = d.version;
  }

  // A removed entry is named `len` or `string.repeat` (03 3.3.1, ruling CD1); any other name is `bad-name` (CD3).
  for (const name of d.removed ?? []) {
    const dot = name.indexOf(".");
    const ok = dot < 0 ? FUNCTIONS.has(name) : METHODS.get(name.slice(0, dot))?.has(name.slice(dot + 1)) === true;
    if (!ok) errs.push(diag("bad-name", `${name} is no entry of the library`));
  }

  for (const name of d.opaque ?? []) {
    if (!/^[A-Z][A-Za-z0-9_]+$/.test(name)) errs.push(diag("bad-name", `${name} is not an opaque type name`));
    else if (env.opaque.has(name)) errs.push(diag("name-clash", `the opaque type ${name} is declared twice`));
    else env.opaque.add(name);
  }

  // A child's inherited variables: their types were given when the parent was built.
  const inheritedVars = new Set<string>();
  for (const [name, v] of d.inherited ?? []) {
    env.variables.set(name, v.node);
    if (v.type !== undefined) {
      env.variableTypes.set(name, v.type);
      inheritedVars.add(name);
    }
  }

  if (d.env !== undefined) {
    const t = parseEnvRecord(d.env);
    if (!t.ok) errs.push(diag("bad-type", `the variables' type: ${t.message}`));
    else if (t.node.t !== "record") errs.push(diag("bad-type", "the variables are not declared as one record type"));
    else {
      for (const f of t.node.fields) {
        if (!/^[A-Za-z_][A-Za-z0-9_]*$/.test(f.key) || !isName(f.key)) {
          errs.push(diag("bad-name", `${JSON.stringify(f.key)} is not a name`));
        } else if (env.variables.has(f.key)) {
          errs.push(diag("name-clash", `the variable ${f.key} is declared twice`));
        } else {
          env.variables.set(f.key, f.type);
        }
      }
    }
  }

  const variableName = (name: string): boolean => {
    if (!/^[A-Za-z_][A-Za-z0-9_]*$/.test(name) || !isName(name)) {
      errs.push(diag("bad-name", `${JSON.stringify(name)} is not a name`));
      return false;
    }
    if (env.variables.has(name)) {
      errs.push(diag("name-clash", `the variable ${name} is declared twice`));
      return false;
    }
    return true;
  };
  for (const [name, text] of Object.entries(d.variableTexts ?? {})) {
    const t = parseType(text);
    if (!t.ok) {
      errs.push(diag("bad-type", `the type of ${name}: ${t.message}`));
      continue;
    }
    if (variableName(name)) env.variables.set(name, t.node);
  }
  // f01 2: a variable declared by schema; its type is mapped now, the schema not kept.
  const schemaVars = inheritedVars;
  for (const [name, decl] of Object.entries(d.schemas ?? {})) {
    if (!variableName(name)) continue;
    env.variables.set(name, { t: "word", word: "any" });
    schemaVars.add(name);
    try {
      env.variableTypes.set(name, typeOfSchema(decl.schema, decl.options ?? []));
    } catch (e) {
      if (!(e instanceof SchemaError)) throw e;
      errs.push(diag("bad-type", `the schema of ${name}: ${e.message}`));
    }
  }

  for (const [full, signature] of Object.entries(d.functions ?? {})) {
    const parts = full.split(".");
    if (parts.length > 2 || !parts.every((p) => /^[A-Za-z_][A-Za-z0-9_]*$/.test(p) && p !== "_")) {
      errs.push(diag("bad-name", `${full} is not a function name`));
      continue;
    }
    if (parts.some((p) => isReserved(p))) {
      errs.push(diag("bad-name", `${full} uses a reserved word`));
      continue;
    }
    const namespace = parts.length === 2 ? parts[0] : null;
    if (namespace !== null && RESERVED_NAMESPACES.has(namespace)) {
      errs.push(diag("reserved-namespace", `the namespace ${namespace} is reserved for the library`));
      continue;
    }
    const s = parseSignature(signature);
    if (!s.ok) {
      errs.push(diag("bad-signature", `the signature of ${full}: ${s.message}`));
      continue;
    }
    if (hasFn(s.node)) {
      errs.push(diag("bad-signature", `${full} has a parameter of a function type`));
      continue;
    }
    if (namespace === null && library.has(full)) {
      errs.push(diag("name-clash", `${full} is a library function`));
      continue;
    }
    if (env.variables.has(full) && namespace === null) {
      errs.push(diag("name-clash", `${full} is declared as a variable and a function`));
      continue;
    }
    env.functions.set(full, { name: full, namespace, sig: s.node, signature });
    if (namespace !== null) env.namespaces.add(namespace);
  }
  for (const ns of env.namespaces) {
    if (env.variables.has(ns)) errs.push(diag("name-clash", `the variable ${ns} has the name of a namespace`));
    if (env.functions.has(ns)) errs.push(diag("name-clash", `the function ${ns} has the name of a namespace`));
  }

  for (const [key, signature] of Object.entries(d.methods ?? {})) {
    const dot = key.indexOf(".");
    const type = key.slice(0, dot);
    const name = key.slice(dot + 1);
    const s = parseSignature(signature);
    if (!s.ok) {
      errs.push(diag("bad-signature", `the method ${key}: ${s.message}`));
      continue;
    }
    if (hasFn(s.node)) {
      errs.push(diag("bad-signature", `the method ${key} has a parameter of a function type`));
      continue;
    }
    if (!env.opaque.has(type)) {
      errs.push(diag("bad-signature", `the receiver of ${key} is not a declared opaque type`));
      continue;
    }
    env.methods.set(key, { type, name, sig: s.node, signature });
  }

  if (d.reject !== undefined) {
    env.reject = new Set();
    for (const c of d.reject) {
      if (!CHECK_CLASSES.has(c)) errs.push(diag("bad-class", `${c} is not a check class`));
      else env.reject.add(c);
    }
  }

  if (d.returns !== undefined) {
    if (d.returns === "condition") env.returns = "condition";
    else {
      const t = parseType(d.returns);
      if (!t.ok) errs.push(diag("bad-type", `the expected result: ${t.message}`));
      else env.returns = t.node;
    }
  }

  // The errors in declared type text that only a type checker finds (05 19.5).
  const typeOf = (n: TypeNode, vars: boolean): Type => {
    const t = fromNode(n, { opaque: env.opaque, variable: vars ? () => ANY : undefined });
    if (depth(t, 70) > 64) throw new TypeRuleError("a type deeper than 64");
    return t;
  };
  const nodesOf = (sig: SigNode): TypeNode[] => [
    ...sig.params.flatMap((p) => (p.type.t === "fn" ? [...p.type.params.map((q) => q.type), ...(p.type.result === "unknown" ? [] : [p.type.result])] : [p.type])),
    sig.result,
  ];
  for (const [name, node] of env.variables) {
    if (schemaVars.has(name)) continue;
    try {
      env.variableTypes.set(name, typeOf(node, false));
    } catch (e) {
      if (!(e instanceof TypeRuleError)) throw e;
      errs.push(diag("bad-type", `the type of ${name}: ${e.message}`));
    }
  }
  for (const fn of [...env.functions.values(), ...env.methods.values()]) {
    for (const n of nodesOf(fn.sig)) {
      try {
        typeOf(n, true);
      } catch (e) {
        if (!(e instanceof TypeRuleError)) throw e;
        // Ruling AO4: every error in a host signature or a method's text is bad-signature.
        errs.push(diag("bad-signature", `the signature of ${fn.name}: ${e.message}`));
        break;
      }
    }
  }
  if (env.returns !== null && env.returns !== "condition") {
    try {
      typeOf(env.returns, false);
    } catch (e) {
      if (!(e instanceof TypeRuleError)) throw e;
      errs.push(diag("bad-type", `the expected result: ${e.message}`));
    }
  }

  return errs.length > 0 ? { ok: false, diagnostics: errs } : { ok: true, env };
}
