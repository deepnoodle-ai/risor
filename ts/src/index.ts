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
// V8 stack of 400 KB; test/deep.test.ts runs them with 500 KB. Node's default
// is 984 KB, so no option is needed. A stack overflow, should a host run with
// less, is reported as `internal-error`, never thrown to the host.

export { parse, parseProgram, MAX_LEVELS } from "./syntax/parser.ts";
export type { ParseResult } from "./syntax/parser.ts";
export { Source } from "./syntax/source.ts";
export type { Span, LineSpan, Position } from "./syntax/source.ts";
export type { Diagnostic, Phase } from "./syntax/diagnostic.ts";
export type * as ast from "./syntax/ast.ts";
export { NODE_NUMBER } from "./syntax/ast.ts";
export { parseType, parseSignature, parseMethod, parseReceiver, arity } from "./types/typetext.ts";
export type { TypeNode, SigNode, MethodNode, FnNode } from "./types/typetext.ts";
export { declare, DEFAULT_LIMITS, VERSION } from "./host/env.ts";
export type { Declarations, Environment, Limits } from "./host/env.ts";
export { createEnv, Env, Program, RisorError, HostError, truthy, toJSON } from "./host/api.ts";
export { typeFromSchema, envFromDescription, descriptionText, printSignature } from "./host/api.ts";
export type { Description } from "./host/api.ts";
export { parseProgramText, parseTemplate, ParsedProgram, Template, reportJSON } from "./host/api.ts";
export type { AnalysisReport, AnalysisCall } from "./host/api.ts";
export type { TemplateMode, Segment } from "./syntax/template.ts";
export type { SchemaOption } from "./types/schema.ts";
export type { EnvOptions, HostFunction, OpaqueType, Limits as RunLimits, Usage, Vars, Outcome, CallHandle } from "./host/api.ts";
