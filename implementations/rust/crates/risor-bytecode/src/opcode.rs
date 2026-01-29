//! Risor bytecode opcode definitions.
//!
//! Opcodes are organized by category for clarity.
//! Each instruction consists of an opcode followed by 0-2 operands.

use std::fmt;

/// Bytecode opcodes for the Risor VM.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash)]
#[repr(u16)]
pub enum Op {
    // =========================================================================
    // Execution Control (1-9)
    // =========================================================================
    /// No operation.
    Nop = 1,
    /// Stop execution.
    Halt = 2,
    /// Call function with N arguments.
    Call = 3,
    /// Return from function with value.
    ReturnValue = 4,
    /// Call with spread arguments.
    CallSpread = 7,

    // =========================================================================
    // Jumps (10-19)
    // =========================================================================
    /// Jump backward (for loops).
    JumpBackward = 10,
    /// Jump forward.
    JumpForward = 11,
    /// Pop and jump if false.
    PopJumpForwardIfFalse = 12,
    /// Pop and jump if true.
    PopJumpForwardIfTrue = 13,
    /// Pop and jump if not nil.
    PopJumpForwardIfNotNil = 14,
    /// Pop and jump if nil.
    PopJumpForwardIfNil = 15,

    // =========================================================================
    // Load Operations (20-29)
    // =========================================================================
    /// Load object attribute.
    LoadAttr = 20,
    /// Load local variable.
    LoadFast = 21,
    /// Load free variable (closure).
    LoadFree = 22,
    /// Load global variable.
    LoadGlobal = 23,
    /// Load constant.
    LoadConst = 24,
    /// Load attribute or nil (no error).
    LoadAttrOrNil = 25,

    // =========================================================================
    // Store Operations (30-39)
    // =========================================================================
    /// Store to object attribute.
    StoreAttr = 30,
    /// Store to local variable.
    StoreFast = 31,
    /// Store to free variable (closure).
    StoreFree = 32,
    /// Store to global variable.
    StoreGlobal = 33,

    // =========================================================================
    // Binary/Unary Operations (40-49)
    // =========================================================================
    /// Binary operation.
    BinaryOp = 40,
    /// Comparison operation.
    CompareOp = 41,
    /// Negate number.
    UnaryNegative = 42,
    /// Logical NOT.
    UnaryNot = 43,

    // =========================================================================
    // Container Building (50-59)
    // =========================================================================
    /// Build list from N items.
    BuildList = 50,
    /// Build map from N pairs.
    BuildMap = 51,
    /// Concatenate N strings.
    BuildString = 53,
    /// Append to list.
    ListAppend = 54,
    /// Extend list with iterable.
    ListExtend = 55,
    /// Merge map into another.
    MapMerge = 56,
    /// Set map key-value.
    MapSet = 57,

    // =========================================================================
    // Container Access (60-69)
    // =========================================================================
    /// Index access.
    BinarySubscr = 60,
    /// Index assignment.
    StoreSubscr = 61,
    /// Contains/in operator.
    ContainsOp = 62,
    /// Get length.
    Length = 63,
    /// Slice operation.
    Slice = 64,
    /// Unpack tuple/list.
    Unpack = 65,

    // =========================================================================
    // Stack Manipulation (70-79)
    // =========================================================================
    /// Swap stack items.
    Swap = 70,
    /// Copy stack item.
    Copy = 71,
    /// Discard top of stack.
    PopTop = 72,

    // =========================================================================
    // Constants (80-89)
    // =========================================================================
    /// Push nil.
    Nil = 80,
    /// Push false.
    False = 81,
    /// Push true.
    True = 82,

    // =========================================================================
    // Closures (120-129)
    // =========================================================================
    /// Load closure (function + free vars).
    LoadClosure = 120,
    /// Create cell for captured variable.
    MakeCell = 121,

    // =========================================================================
    // Partial Application (130-139)
    // =========================================================================
    /// Create partial function (for piping).
    Partial = 130,

    // =========================================================================
    // Exception Handling (140-149)
    // =========================================================================
    /// Push exception handler.
    PushExcept = 140,
    /// Pop exception handler.
    PopExcept = 141,
    /// Throw exception.
    Throw = 142,
    /// End finally block.
    EndFinally = 143,
}

impl fmt::Display for Op {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(f, "{:?}", self)
    }
}

impl Op {
    /// Get number of operands for this opcode.
    pub fn operand_count(self) -> usize {
        match self {
            // No operands
            Op::Nop
            | Op::Halt
            | Op::ReturnValue
            | Op::UnaryNegative
            | Op::UnaryNot
            | Op::Nil
            | Op::False
            | Op::True
            | Op::PopTop
            | Op::PopExcept
            | Op::Throw
            | Op::EndFinally => 0,

            // One operand
            Op::Call
            | Op::CallSpread
            | Op::JumpBackward
            | Op::JumpForward
            | Op::PopJumpForwardIfFalse
            | Op::PopJumpForwardIfTrue
            | Op::PopJumpForwardIfNotNil
            | Op::PopJumpForwardIfNil
            | Op::LoadAttr
            | Op::LoadFast
            | Op::LoadFree
            | Op::LoadGlobal
            | Op::LoadConst
            | Op::LoadAttrOrNil
            | Op::StoreAttr
            | Op::StoreFast
            | Op::StoreFree
            | Op::StoreGlobal
            | Op::BinaryOp
            | Op::CompareOp
            | Op::BuildList
            | Op::BuildMap
            | Op::BuildString
            | Op::ListAppend
            | Op::ListExtend
            | Op::MapMerge
            | Op::MapSet
            | Op::BinarySubscr
            | Op::StoreSubscr
            | Op::ContainsOp
            | Op::Length
            | Op::Unpack
            | Op::Swap
            | Op::Copy
            | Op::Partial => 1,

            // Two operands
            Op::Slice | Op::LoadClosure | Op::MakeCell | Op::PushExcept => 2,
        }
    }
}

/// Binary operation types for BinaryOp instruction.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash)]
#[repr(u16)]
pub enum BinaryOpType {
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

impl fmt::Display for BinaryOpType {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        let s = match self {
            BinaryOpType::Add => "+",
            BinaryOpType::Subtract => "-",
            BinaryOpType::Multiply => "*",
            BinaryOpType::Divide => "/",
            BinaryOpType::Modulo => "%",
            BinaryOpType::And => "&&",
            BinaryOpType::Or => "||",
            BinaryOpType::Xor => "^",
            BinaryOpType::Power => "**",
            BinaryOpType::LShift => "<<",
            BinaryOpType::RShift => ">>",
            BinaryOpType::BitwiseAnd => "&",
            BinaryOpType::BitwiseOr => "|",
            BinaryOpType::NullishCoalesce => "??",
        };
        write!(f, "{}", s)
    }
}

/// Comparison operation types for CompareOp instruction.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash)]
#[repr(u16)]
pub enum CompareOpType {
    Lt = 0,
    LtEquals = 1,
    Eq = 2,
    NotEq = 3,
    Gt = 4,
    GtEquals = 5,
}

impl fmt::Display for CompareOpType {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        let s = match self {
            CompareOpType::Lt => "<",
            CompareOpType::LtEquals => "<=",
            CompareOpType::Eq => "==",
            CompareOpType::NotEq => "!=",
            CompareOpType::Gt => ">",
            CompareOpType::GtEquals => ">=",
        };
        write!(f, "{}", s)
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_operand_count() {
        assert_eq!(Op::Nop.operand_count(), 0);
        assert_eq!(Op::Halt.operand_count(), 0);
        assert_eq!(Op::Call.operand_count(), 1);
        assert_eq!(Op::LoadConst.operand_count(), 1);
        assert_eq!(Op::LoadClosure.operand_count(), 2);
        assert_eq!(Op::PushExcept.operand_count(), 2);
    }

    #[test]
    fn test_display() {
        assert_eq!(format!("{}", Op::Nop), "Nop");
        assert_eq!(format!("{}", Op::LoadConst), "LoadConst");
        assert_eq!(format!("{}", BinaryOpType::Add), "+");
        assert_eq!(format!("{}", CompareOpType::Eq), "==");
    }
}
