/**
 * Risor bytecode opcode definitions.
 *
 * Opcodes are organized by category for clarity.
 * Each instruction consists of an opcode followed by 0-2 operands.
 */

/**
 * Bytecode opcodes for the Risor VM.
 * Using const enum for zero runtime cost.
 */
export const enum Op {
  // =========================================================================
  // Execution Control (1-9)
  // =========================================================================
  Nop = 1, // No operation
  Halt = 2, // Stop execution
  Call = 3, // Call function with N arguments
  ReturnValue = 4, // Return from function with value
  CallSpread = 7, // Call with spread arguments

  // =========================================================================
  // Jumps (10-19)
  // =========================================================================
  JumpBackward = 10, // Jump backward (for loops)
  JumpForward = 11, // Jump forward
  PopJumpForwardIfFalse = 12, // Pop and jump if false
  PopJumpForwardIfTrue = 13, // Pop and jump if true
  PopJumpForwardIfNotNil = 14, // Pop and jump if not nil
  PopJumpForwardIfNil = 15, // Pop and jump if nil

  // =========================================================================
  // Load Operations (20-29)
  // =========================================================================
  LoadAttr = 20, // Load object attribute
  LoadFast = 21, // Load local variable
  LoadFree = 22, // Load free variable (closure)
  LoadGlobal = 23, // Load global variable
  LoadConst = 24, // Load constant
  LoadAttrOrNil = 25, // Load attribute or nil (no error)

  // =========================================================================
  // Store Operations (30-39)
  // =========================================================================
  StoreAttr = 30, // Store to object attribute
  StoreFast = 31, // Store to local variable
  StoreFree = 32, // Store to free variable (closure)
  StoreGlobal = 33, // Store to global variable

  // =========================================================================
  // Binary/Unary Operations (40-49)
  // =========================================================================
  BinaryOp = 40, // Binary operation
  CompareOp = 41, // Comparison operation
  UnaryNegative = 42, // Negate number
  UnaryNot = 43, // Logical NOT

  // =========================================================================
  // Container Building (50-59)
  // =========================================================================
  BuildList = 50, // Build list from N items
  BuildMap = 51, // Build map from N pairs
  BuildString = 53, // Concatenate N strings
  ListAppend = 54, // Append to list
  ListExtend = 55, // Extend list with iterable
  MapMerge = 56, // Merge map into another
  MapSet = 57, // Set map key-value

  // =========================================================================
  // Container Access (60-69)
  // =========================================================================
  BinarySubscr = 60, // Index access
  StoreSubscr = 61, // Index assignment
  ContainsOp = 62, // Contains/in operator
  Length = 63, // Get length
  Slice = 64, // Slice operation
  Unpack = 65, // Unpack tuple/list

  // =========================================================================
  // Stack Manipulation (70-79)
  // =========================================================================
  Swap = 70, // Swap stack items
  Copy = 71, // Copy stack item
  PopTop = 72, // Discard top of stack

  // =========================================================================
  // Constants (80-89)
  // =========================================================================
  Nil = 80, // Push nil
  False = 81, // Push false
  True = 82, // Push true

  // =========================================================================
  // Closures (120-129)
  // =========================================================================
  LoadClosure = 120, // Load closure (function + free vars)
  MakeCell = 121, // Create cell for captured variable

  // =========================================================================
  // Partial Application (130-139)
  // =========================================================================
  Partial = 130, // Create partial function (for piping)

  // =========================================================================
  // Exception Handling (140-149)
  // =========================================================================
  PushExcept = 140, // Push exception handler
  PopExcept = 141, // Pop exception handler
  Throw = 142, // Throw exception
  EndFinally = 143, // End finally block
}

/**
 * Binary operation types for BinaryOp instruction.
 */
export const enum BinaryOpType {
  Add = 0,
  Subtract = 1,
  Multiply = 2,
  Divide = 3,
  Modulo = 4,
  And = 5,
  Or = 6,
  Xor = 7,
  Power = 8,
  LShift = 9,
  RShift = 10,
  BitwiseAnd = 11,
  BitwiseOr = 12,
  NullishCoalesce = 13,
}

/**
 * Comparison operation types for CompareOp instruction.
 */
export const enum CompareOpType {
  Lt = 0, // <
  LtEquals = 1, // <=
  Eq = 2, // ==
  NotEq = 3, // !=
  Gt = 4, // >
  GtEquals = 5, // >=
}

/**
 * Get number of operands for an opcode.
 */
export function operandCount(op: Op): number {
  switch (op) {
    // No operands
    case Op.Nop:
    case Op.Halt:
    case Op.ReturnValue:
    case Op.UnaryNegative:
    case Op.UnaryNot:
    case Op.Nil:
    case Op.False:
    case Op.True:
    case Op.PopTop:
    case Op.PopExcept:
    case Op.Throw:
    case Op.EndFinally:
      return 0;

    // One operand
    case Op.Call:
    case Op.CallSpread:
    case Op.JumpBackward:
    case Op.JumpForward:
    case Op.PopJumpForwardIfFalse:
    case Op.PopJumpForwardIfTrue:
    case Op.PopJumpForwardIfNotNil:
    case Op.PopJumpForwardIfNil:
    case Op.LoadAttr:
    case Op.LoadFast:
    case Op.LoadFree:
    case Op.LoadGlobal:
    case Op.LoadConst:
    case Op.LoadAttrOrNil:
    case Op.StoreAttr:
    case Op.StoreFast:
    case Op.StoreFree:
    case Op.StoreGlobal:
    case Op.BinaryOp:
    case Op.CompareOp:
    case Op.BuildList:
    case Op.BuildMap:
    case Op.BuildString:
    case Op.ListAppend:
    case Op.ListExtend:
    case Op.MapMerge:
    case Op.MapSet:
    case Op.BinarySubscr:
    case Op.StoreSubscr:
    case Op.ContainsOp:
    case Op.Length:
    case Op.Unpack:
    case Op.Swap:
    case Op.Copy:
    case Op.Partial:
      return 1;

    // Two operands
    case Op.Slice:
    case Op.LoadClosure:
    case Op.MakeCell:
    case Op.PushExcept:
      return 2;

    default:
      return 0;
  }
}

/**
 * Get opcode name for debugging.
 */
export function opName(op: Op): string {
  switch (op) {
    case Op.Nop:
      return "Nop";
    case Op.Halt:
      return "Halt";
    case Op.Call:
      return "Call";
    case Op.ReturnValue:
      return "ReturnValue";
    case Op.CallSpread:
      return "CallSpread";
    case Op.JumpBackward:
      return "JumpBackward";
    case Op.JumpForward:
      return "JumpForward";
    case Op.PopJumpForwardIfFalse:
      return "PopJumpForwardIfFalse";
    case Op.PopJumpForwardIfTrue:
      return "PopJumpForwardIfTrue";
    case Op.PopJumpForwardIfNotNil:
      return "PopJumpForwardIfNotNil";
    case Op.PopJumpForwardIfNil:
      return "PopJumpForwardIfNil";
    case Op.LoadAttr:
      return "LoadAttr";
    case Op.LoadFast:
      return "LoadFast";
    case Op.LoadFree:
      return "LoadFree";
    case Op.LoadGlobal:
      return "LoadGlobal";
    case Op.LoadConst:
      return "LoadConst";
    case Op.LoadAttrOrNil:
      return "LoadAttrOrNil";
    case Op.StoreAttr:
      return "StoreAttr";
    case Op.StoreFast:
      return "StoreFast";
    case Op.StoreFree:
      return "StoreFree";
    case Op.StoreGlobal:
      return "StoreGlobal";
    case Op.BinaryOp:
      return "BinaryOp";
    case Op.CompareOp:
      return "CompareOp";
    case Op.UnaryNegative:
      return "UnaryNegative";
    case Op.UnaryNot:
      return "UnaryNot";
    case Op.BuildList:
      return "BuildList";
    case Op.BuildMap:
      return "BuildMap";
    case Op.BuildString:
      return "BuildString";
    case Op.ListAppend:
      return "ListAppend";
    case Op.ListExtend:
      return "ListExtend";
    case Op.MapMerge:
      return "MapMerge";
    case Op.MapSet:
      return "MapSet";
    case Op.BinarySubscr:
      return "BinarySubscr";
    case Op.StoreSubscr:
      return "StoreSubscr";
    case Op.ContainsOp:
      return "ContainsOp";
    case Op.Length:
      return "Length";
    case Op.Slice:
      return "Slice";
    case Op.Unpack:
      return "Unpack";
    case Op.Swap:
      return "Swap";
    case Op.Copy:
      return "Copy";
    case Op.PopTop:
      return "PopTop";
    case Op.Nil:
      return "Nil";
    case Op.False:
      return "False";
    case Op.True:
      return "True";
    case Op.LoadClosure:
      return "LoadClosure";
    case Op.MakeCell:
      return "MakeCell";
    case Op.Partial:
      return "Partial";
    case Op.PushExcept:
      return "PushExcept";
    case Op.PopExcept:
      return "PopExcept";
    case Op.Throw:
      return "Throw";
    case Op.EndFinally:
      return "EndFinally";
    default:
      return `Unknown(${op})`;
  }
}

/**
 * Get binary operator name for debugging.
 */
export function binaryOpName(op: BinaryOpType): string {
  switch (op) {
    case BinaryOpType.Add:
      return "+";
    case BinaryOpType.Subtract:
      return "-";
    case BinaryOpType.Multiply:
      return "*";
    case BinaryOpType.Divide:
      return "/";
    case BinaryOpType.Modulo:
      return "%";
    case BinaryOpType.And:
      return "&&";
    case BinaryOpType.Or:
      return "||";
    case BinaryOpType.Xor:
      return "^";
    case BinaryOpType.Power:
      return "**";
    case BinaryOpType.LShift:
      return "<<";
    case BinaryOpType.RShift:
      return ">>";
    case BinaryOpType.BitwiseAnd:
      return "&";
    case BinaryOpType.BitwiseOr:
      return "|";
    case BinaryOpType.NullishCoalesce:
      return "??";
    default:
      return `Unknown(${op})`;
  }
}

/**
 * Get comparison operator name for debugging.
 */
export function compareOpName(op: CompareOpType): string {
  switch (op) {
    case CompareOpType.Lt:
      return "<";
    case CompareOpType.LtEquals:
      return "<=";
    case CompareOpType.Eq:
      return "==";
    case CompareOpType.NotEq:
      return "!=";
    case CompareOpType.Gt:
      return ">";
    case CompareOpType.GtEquals:
      return ">=";
    default:
      return `Unknown(${op})`;
  }
}
