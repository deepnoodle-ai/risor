/**
 * Risor - A fast, embedded scripting language for TypeScript/JavaScript.
 *
 * @packageDocumentation
 */

// Token exports
export {
  Token,
  TokenKind,
  Position,
  newToken,
  newPosition,
  NoPos,
  isValidPosition,
  lineNumber,
  columnNumber,
  advancePosition,
  lookupIdentifier,
} from "./token/token.js";

// Lexer exports
export { Lexer, LexerError, tokenize } from "./lexer/lexer.js";

// AST exports
export * from "./ast/nodes.js";

// Parser exports
export { Parser, ParserError, parse } from "./parser/parser.js";
export { Precedence, getPrecedence } from "./parser/precedence.js";

// Bytecode exports
export * from "./bytecode/index.js";

// Compiler exports
export * from "./compiler/index.js";

// Object exports
export * from "./object/index.js";

// VM exports
export * from "./vm/index.js";

// Builtins exports
export * from "./builtins/index.js";

// Runner exports
export { runFile, runCode } from "./runner.js";

// REPL export
export { startRepl } from "./repl.js";
