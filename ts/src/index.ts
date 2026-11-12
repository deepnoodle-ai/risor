// Risor v3, the TypeScript implementation. Written from `spec/` and `suite/`
// only.
//
// Layout of the package (each stage lands in its own directory):
//
//   src/syntax/    S1  source text and positions, tokens, lexer, syntax tree, parser
//   src/types/     S1  type text (05 3.1, 3.2); later the checker
//   src/value/     S1  numbers (the representation is written in number.ts);
//                  S2  values, equality, order, rendering
//   src/host/      S1  environment declarations; S2 the host API: compile, run,
//                      variables, host functions, opaque values, usage
//   src/eval/      S2  static rules, the evaluator, steps, units, limits, try/catch
//   src/library/   S3  the 53 Core entries, built from suite/library.json and
//                      suite/casemap.json
//   src/suite/     S1  strict JSON, case files (03 3.10) and the runner
//
//   src/generated/ S2 the run catalog and the library records, generated from
//                      suite/*.json by scripts/generate.ts (never edited by hand)
//
// Positions are code-point offsets everywhere (src/syntax/source.ts).
//
// STACK (02 8.4, 01 13.2). The evaluator recurses only where the text opens a
// nesting level, through calls in progress, and in walks; long flat forms run
// in loops. The deepest programs within the limits (depth limit 1,000 reached
// by calls at nesting 0 plus a walk; 100 levels of source nesting) run with a
// V8 stack of 600 KB; test/deep.test.ts runs them with 700 KB. Node's default
// is 984 KB, so no option is needed. A stack overflow, should a host run with
// less, is reported as `internal-error`, never thrown to the host.
//
// MEMORY (02 8.5 item 2). On 64-bit V8 (pointer-compressed JS values), the heap
// memory a run creates as values of the language is bounded by 256 bytes per
// allocation unit charged plus 16 bytes per step charged. The bound is derived
// from V8's object layouts, not measured:
//   - string (1 + floor(w/64) units): a SeqString header (16 bytes) plus at
//     most 2 bytes per UTF-16 code unit (128 bytes per 64 weight units);
//   - list (1 + n units): a JSArray header (16 bytes) plus a FixedArray backing
//     store (8 bytes header + 4 to 8 bytes per element, up to 2x slack);
//   - map (1 + 2n units): an RMap, a JS Map, whose OrderedHashMap takes about
//     40 bytes per entry across its hash table and entries, so at most about
//     20 bytes per unit, with up to 2x slack after growth;
//   - step: at most one 16-byte HeapNumber when a numeric result does not fit a
//     Smi.

// The public surface is exactly that of 05 Part E (ruling R200), in the
// TypeScript column. Everything else in src/ is private. The case-file runner
// is the separate entry point "risor/testing".

// E.3 The environment.
export { createEnv, Env, descriptionText } from "./host/api.ts";
export type { EnvOptions, WithOptions, LibraryNames, ChildOpaqueType, NamedType, Limits, Description, HostFunction, OpaqueType } from "./host/api.ts";

// E.4 Programs.
export { parse, Expr, Analysis, Program } from "./host/api.ts";
export type { CallSite, Ref, Expected, CheckResult, Site, CompileOptions, Usage, Vars } from "./host/api.ts";

// E.5 Templates.
export { parseTemplate, Template, formatTemplate } from "./host/api.ts";
export type { TemplateMode, Segment } from "./host/api.ts";

// E.6 Host functions and values.
export { HostError, truthy, jsonText } from "./host/api.ts";
export type { Call } from "./host/api.ts";

// E.7 Types, conformance and config files.
export { Type, typeNever, typeAny, typeNull, typeBool, typeInt, typeFloat, typeString, listType, mapType, nullable, literalType, recordType, exactRecordType, unionType, taggedUnionType, opaqueTypeNamed, join } from "./host/type.ts";
export type { TypeKind, FieldType } from "./host/type.ts";
// E.1 row 5: the TypeScript type of a Risor type (R202).
export type { Infer } from "./host/infer.ts";
export { typeFromSchema, conform, conformAll, loadValue, decode, LoadError } from "./host/api.ts";
export type { LoadOptions, SchemaOption } from "./host/api.ts";

// E.8 Diagnostics and tools.
export { RisorError, renderLine, renderDetail, renderJSON, format } from "./host/api.ts";
export type { Diagnostic, Phase, Span, Pos, TraceEntry } from "./host/error.ts";
export type { Completion, Candidate, Hover } from "./host/api.ts";
