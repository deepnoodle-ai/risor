/**
 * Bytecode module exports.
 */

export {
  Op,
  BinaryOpType,
  CompareOpType,
  operandCount,
  opName,
  binaryOpName,
  compareOpName,
} from "./opcode.js";

export {
  Code,
  CodeBuilder,
  Constant,
  ConstantType,
  SourceLocation,
  ExceptionHandler,
  FunctionInfo,
} from "./code.js";
