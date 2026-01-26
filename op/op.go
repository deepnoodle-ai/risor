// Package op defines opcodes used by the Risor compiler and virtual machine.
package op

// Code is an integer opcode that indicates an operation to execute.
type Code uint16

const (
	Invalid Code = 0

	// Execution
	Nop         Code = 1
	Halt        Code = 2
	Call        Code = 3
	ReturnValue Code = 4
	// Defer (removed in v2)    Code = 5
	// Go (removed in v2)       Code = 6
	CallSpread Code = 7 // Call with args from list on stack

	// Jump
	JumpBackward           Code = 10
	JumpForward            Code = 11
	PopJumpForwardIfFalse  Code = 12
	PopJumpForwardIfTrue   Code = 13
	PopJumpForwardIfNotNil Code = 14
	PopJumpForwardIfNil    Code = 15

	// Load
	LoadAttr      Code = 20
	LoadFast      Code = 21
	LoadFree      Code = 22
	LoadGlobal    Code = 23
	LoadConst     Code = 24
	LoadAttrOrNil Code = 25 // Like LoadAttr but returns nil instead of error for missing attrs

	// Store
	StoreAttr   Code = 30
	StoreFast   Code = 31
	StoreFree   Code = 32
	StoreGlobal Code = 33

	// Operations
	BinaryOp      Code = 40
	CompareOp     Code = 41
	UnaryNegative Code = 42
	UnaryNot      Code = 43

	// Build
	BuildList   Code = 50
	BuildMap    Code = 51
	BuildSet    Code = 52
	BuildString Code = 53
	ListAppend  Code = 54 // Append TOS to list at TOS-1
	ListExtend  Code = 55 // Extend list at TOS-1 with iterable at TOS
	MapMerge    Code = 56 // Merge map at TOS into map at TOS-1
	MapSet      Code = 57 // Set key (TOS-1) to value (TOS) in map at TOS-2

	// Containers
	BinarySubscr Code = 60
	StoreSubscr  Code = 61
	ContainsOp   Code = 62
	Length       Code = 63
	Slice        Code = 64
	Unpack       Code = 65

	// Stack
	Swap   Code = 70
	Copy   Code = 71
	PopTop Code = 72

	// Push constants
	Nil   Code = 80
	False Code = 81
	True  Code = 82

	// Iteration
	ForIter Code = 90
	GetIter Code = 91
	Range   Code = 92

	// Channels (removed in v2)
	// Receive Code = 110
	// Send    Code = 111

	// Closures
	LoadClosure Code = 120
	MakeCell    Code = 121

	// Partials
	Partial Code = 130

	// Exception handling
	PushExcept Code = 140 // Push exception handler: operand1=catch offset, operand2=finally offset
	PopExcept  Code = 141 // Pop exception handler (normal try completion)
	Throw      Code = 142 // Throw TOS as exception
	EndFinally Code = 143 // End finally block, re-raise pending exception if any
)

// BinaryOpType describes a type of binary operation, as in an operation that
// takes two operands. For example, addition, subtraction, multiplication, etc.
type BinaryOpType uint16

const (
	Add        BinaryOpType = 1
	Subtract   BinaryOpType = 2
	Multiply   BinaryOpType = 3
	Divide     BinaryOpType = 4
	Modulo     BinaryOpType = 5
	And        BinaryOpType = 6
	Or         BinaryOpType = 7
	Xor        BinaryOpType = 8
	Power      BinaryOpType = 9
	LShift     BinaryOpType = 10
	RShift     BinaryOpType = 11
	BitwiseAnd BinaryOpType = 12
	BitwiseOr  BinaryOpType = 13
)

// String returns a string representation of the binary operation.
// For example "+" for addition.
func (bop BinaryOpType) String() string {
	switch bop {
	case Add:
		return "+"
	case Subtract:
		return "-"
	case Multiply:
		return "*"
	case Divide:
		return "/"
	case Modulo:
		return "%"
	case And:
		return "&&"
	case Or:
		return "||"
	case Xor:
		return "^"
	case Power:
		return "**"
	case LShift:
		return "<<"
	case RShift:
		return ">>"
	case BitwiseAnd:
		return "&^"
	case BitwiseOr:
		return "|^"
	default:
		return ""
	}
}

// CompareOpType describes a type of comparison operation. For example, less
// than, greater than, equal, etc.
type CompareOpType uint16

const (
	LessThan           CompareOpType = 1
	LessThanOrEqual    CompareOpType = 2
	Equal              CompareOpType = 3
	NotEqual           CompareOpType = 4
	GreaterThan        CompareOpType = 5
	GreaterThanOrEqual CompareOpType = 6
)

// String returns a string representation of the comparison operation.
// For example "<" for less than.
func (cop CompareOpType) String() string {
	switch cop {
	case LessThan:
		return "<"
	case LessThanOrEqual:
		return "<="
	case Equal:
		return "=="
	case NotEqual:
		return "!="
	case GreaterThan:
		return ">"
	case GreaterThanOrEqual:
		return ">="
	default:
		return ""
	}
}

// Info contains information about an opcode.
type Info struct {
	Code         Code
	Name         string
	OperandCount int
}

var infos = make([]Info, 256)

func init() {
	type opInfo struct {
		op    Code
		name  string
		count int
	}
	ops := []opInfo{
		{BinaryOp, "BINARY_OP", 1},
		{BinarySubscr, "BINARY_SUBSCR", 0},
		{BuildList, "BUILD_LIST", 1},
		{BuildMap, "BUILD_MAP", 1},
		{BuildSet, "BUILD_SET", 1},
		{BuildString, "BUILD_STRING", 1},
		{Call, "CALL", 1},
		{CallSpread, "CALL_SPREAD", 0},
		{CompareOp, "COMPARE_OP", 1},
		{ContainsOp, "CONTAINS_OP", 1},
		{Copy, "COPY", 1},
		{False, "FALSE", 0},
		{ForIter, "FOR_ITER", 2},
		{GetIter, "GET_ITER", 0},
		{Halt, "HALT", 0},
		{JumpBackward, "JUMP_BACKWARD", 1},
		{JumpForward, "JUMP_FORWARD", 1},
		{Length, "LENGTH", 0},
		{ListAppend, "LIST_APPEND", 0},
		{ListExtend, "LIST_EXTEND", 0},
		{MapMerge, "MAP_MERGE", 0},
		{MapSet, "MAP_SET", 0},
		{LoadAttr, "LOAD_ATTR", 1},
		{LoadAttrOrNil, "LOAD_ATTR_OR_NIL", 1},
		{LoadClosure, "LOAD_CLOSURE", 2},
		{LoadConst, "LOAD_CONST", 1},
		{LoadFast, "LOAD_FAST", 1},
		{LoadFree, "LOAD_FREE", 1},
		{LoadGlobal, "LOAD_GLOBAL", 1},
		{MakeCell, "MAKE_CELL", 2},
		{Nil, "NIL", 0},
		{Nop, "NOP", 0},
		{Partial, "PARTIAL", 1},
		{PopJumpForwardIfFalse, "POP_JUMP_FORWARD_IF_FALSE", 1},
		{PopJumpForwardIfNil, "POP_JUMP_FORWARD_IF_NIL", 1},
		{PopJumpForwardIfNotNil, "POP_JUMP_FORWARD_IF_NOT_NIL", 1},
		{PopJumpForwardIfTrue, "POP_JUMP_FORWARD_IF_TRUE", 1},
		{PopTop, "POP_TOP", 0},
		{Range, "RANGE", 0},
		{ReturnValue, "RETURN_VALUE", 0},
		{Slice, "SLICE", 0},
		{StoreAttr, "STORE_ATTR", 1},
		{StoreFast, "STORE_FAST", 1},
		{StoreFree, "STORE_FREE", 1},
		{StoreGlobal, "STORE_GLOBAL", 1},
		{StoreSubscr, "STORE_SUBSCR", 0},
		{Swap, "SWAP", 1},
		{True, "TRUE", 0},
		{UnaryNegative, "UNARY_NEGATIVE", 0},
		{UnaryNot, "UNARY_NOT", 0},
		{Unpack, "UNPACK", 1},
		{PushExcept, "PUSH_EXCEPT", 2},
		{PopExcept, "POP_EXCEPT", 0},
		{Throw, "THROW", 0},
		{EndFinally, "END_FINALLY", 0},
	}
	for _, o := range ops {
		infos[o.op] = Info{
			Name:         o.name,
			Code:         o.op,
			OperandCount: o.count,
		}
	}
}

// GetInfo returns information about the given opcode.
func GetInfo(op Code) Info {
	return infos[op]
}
