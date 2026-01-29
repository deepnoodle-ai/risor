/**
 * Compiler module exports.
 */

export {
  Compiler,
  CompilerError,
  CompilerConfig,
  compile,
} from "./compiler.js";

export {
  SymbolTable,
  Symbol,
  Resolution,
  Scope,
  createRootSymbolTable,
} from "./symbol-table.js";
