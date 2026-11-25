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
//   src/library/   S3  the 56 Core entries, built from suite/library.json and
//                      suite/casemap.json
//   src/suite/     S1  strict JSON, case files (03 3.10) and the runner
//
//   src/generated/ S2 the run catalog and the library records, generated from
//                      suite/*.json by scripts/generate.ts (never edited by hand)
//
// Positions are code-point offsets everywhere (src/syntax/source.ts).
//
// STACK (02 8.4, 01 13.2, 03 3.9.1). The evaluator recurses only where the
// text opens a nesting level, through calls in progress, and in walks; long
// flat forms run in loops. So the native stack a run needs grows with its
// depth count. A unit of the count costs the most where each call goes
// through a library entry that calls a lambda: the forms of
// test/fixtures/deep-forms.ts, of which `heaviest` adds every operand that
// opens no level. src/eval/compile.ts (Stack) says how the cost is kept down.
//
// The stack this binding needs is Node's default, 984 KB, so no option is
// needed. Measured (ran), each case in a fresh process, the V8 stack it needs
// in Node 22.22.3 (V8 12.4) on an Apple M4 Max, at the default depth limit,
// 200, and the largest, 400, with each call's names in a frame of its own
// (f17 11.5), to within 3 KB:
//
//   case                                      depth 200   depth 400
//   100 levels of nesting, parsed and run          144 KB
//   calls by name                               79 KB      113 KB
//   a walk                                      89 KB      142 KB
//   a call of `walk` (F17), nodes to the limit  57 KB       57 KB
//   deep-forms.ts `map`                        137 KB      232 KB
//   deep-forms.ts `flatMap`                    140 KB      238 KB
//   deep-forms.ts `heaviest`                   165 KB      288 KB
//   self-recursion (F17 11.6), by name         104 KB      163 KB
//   self-recursion through `map`               163 KB      282 KB
//   self-recursion through `flatMap`           167 KB      288 KB
//
// The frames cost at most 6% more stack than one fixed slot for each name did,
// measured the same way: calls by name needed 76 and 107 KB, `heaviest` 163
// and 282 KB.
//
// test/deep.test.ts runs each case at both limits with 656 KB, 984 KB divided
// by 1.5. A browser's Web Worker has a smaller stack, which a page cannot set.
// Measured before the largest limit was lowered from 1,000: in a Chrome 158
// worker every one of 28 such forms ran to depth 771 or more (`heaviest` to
// 783), in Firefox 159 to 1,000, and in WebKit 27.2 to 697 or more
// (Playwright's builds). web/examples/react-vite/e2e/deep.spec.ts runs the
// forms in a Chrome worker at 400, and under 200 levels of themselves. A
// stack overflow, should a host run with less, is reported as
// `internal-error`, never thrown to the host.
//
// What holds for the evaluator holds for every pass (01 13.2 rule 6). A form
// that opens no level (01 13.1, f04 2.8, f07 2.5) makes a tree as deep as the
// form is long: a receiver is inside its step, a left operand inside its
// operator, an `else` branch inside its conditional. The parser, the static
// rules, the checker, the compiler, the analysis, the formatter with its
// document and its layout, completion and hover each walk such a form in a
// loop with a stack of its own, and call themselves only where the text opens
// a level. test/flat.test.ts runs each form of test/fixtures/flat-forms.ts
// with 10,000 links through each of them.
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
// is the separate entry point "@deepnoodle/risor/testing".

// E.3 The environment.
export { createEnv, Env, descriptionText } from "./host/api.ts";
export type { EnvOptions, WithOptions, LibraryNames, ChildOpaqueType, NamedType, Limits, Description, HostFunction, OpaqueType } from "./host/api.ts";

// E.4 Programs.
export { parse, Expr, Analysis, Program } from "./host/api.ts";
export type { CallSite, Ref, Expected, CheckResult, Site, CompileOptions, SourceOptions, Usage, Vars } from "./host/api.ts";

// E.5 Templates.
export { parseTemplate, Template, formatTemplate } from "./host/api.ts";
export type { TemplateMode, Segment } from "./host/api.ts";

// E.6 Host functions and values.
export { HostError, truthy, jsonText } from "./host/api.ts";
export type { Call, Remaining } from "./host/api.ts";

// E.7 Types, conformance and config files.
export { Type, typeNever, typeAny, typeNull, typeBool, typeInt, typeFloat, typeString, listType, mapType, nullable, literalType, recordType, exactRecordType, unionType, taggedUnionType, opaqueTypeNamed, join } from "./host/type.ts";
export type { TypeKind, FieldType } from "./host/type.ts";
// E.1 row 5: the TypeScript type of a Risor type (R202).
export type { Infer } from "./host/infer.ts";
export { typeFromSchema, schemaOf, schemaText, conform, conformAll, loadValue, decode, LoadError } from "./host/api.ts";
export type { ConformOptions, LoadOptions, SchemaOption } from "./host/api.ts";

// E.8 Diagnostics and tools.
export { RisorError, renderLine, renderDetail, renderJSON, format } from "./host/api.ts";
export type { Diagnostic, Phase, Span, Pos, TraceEntry } from "./host/error.ts";
export type { Completion, Candidate, Hover } from "./host/api.ts";
