package op

import (
	"testing"

	"github.com/deepnoodle-ai/wonton/assert"
)

func TestGetInfo(t *testing.T) {
	info := GetInfo(LoadClosure)
	assert.Equal(t, info.Name, "LOAD_CLOSURE")
	assert.Equal(t, info.OperandCount, 2)
	assert.Equal(t, info.Code, LoadClosure)
}

func TestGetInfoAllOpcodes(t *testing.T) {
	tests := []struct {
		code      Code
		name      string
		operands  int
	}{
		{Nop, "NOP", 0},
		{Halt, "HALT", 0},
		{Call, "CALL", 1},
		{ReturnValue, "RETURN_VALUE", 0},
		{CallSpread, "CALL_SPREAD", 0},
		{JumpBackward, "JUMP_BACKWARD", 1},
		{JumpForward, "JUMP_FORWARD", 1},
		{PopJumpForwardIfFalse, "POP_JUMP_FORWARD_IF_FALSE", 1},
		{PopJumpForwardIfTrue, "POP_JUMP_FORWARD_IF_TRUE", 1},
		{PopJumpForwardIfNotNil, "POP_JUMP_FORWARD_IF_NOT_NIL", 1},
		{PopJumpForwardIfNil, "POP_JUMP_FORWARD_IF_NIL", 1},
		{LoadAttr, "LOAD_ATTR", 1},
		{LoadFast, "LOAD_FAST", 1},
		{LoadFree, "LOAD_FREE", 1},
		{LoadGlobal, "LOAD_GLOBAL", 1},
		{LoadConst, "LOAD_CONST", 1},
		{LoadAttrOrNil, "LOAD_ATTR_OR_NIL", 1},
		{StoreAttr, "STORE_ATTR", 1},
		{StoreFast, "STORE_FAST", 1},
		{StoreFree, "STORE_FREE", 1},
		{StoreGlobal, "STORE_GLOBAL", 1},
		{BinaryOp, "BINARY_OP", 1},
		{CompareOp, "COMPARE_OP", 1},
		{UnaryNegative, "UNARY_NEGATIVE", 0},
		{UnaryNot, "UNARY_NOT", 0},
		{BuildList, "BUILD_LIST", 1},
		{BuildMap, "BUILD_MAP", 1},
		{BuildString, "BUILD_STRING", 1},
		{ListAppend, "LIST_APPEND", 0},
		{ListExtend, "LIST_EXTEND", 0},
		{MapMerge, "MAP_MERGE", 0},
		{MapSet, "MAP_SET", 0},
		{BinarySubscr, "BINARY_SUBSCR", 0},
		{StoreSubscr, "STORE_SUBSCR", 0},
		{ContainsOp, "CONTAINS_OP", 1},
		{Length, "LENGTH", 0},
		{Slice, "SLICE", 0},
		{Unpack, "UNPACK", 1},
		{Swap, "SWAP", 1},
		{Copy, "COPY", 1},
		{PopTop, "POP_TOP", 0},
		{Nil, "NIL", 0},
		{False, "FALSE", 0},
		{True, "TRUE", 0},
		{LoadClosure, "LOAD_CLOSURE", 2},
		{MakeCell, "MAKE_CELL", 2},
		{Partial, "PARTIAL", 1},
		{PushExcept, "PUSH_EXCEPT", 2},
		{PopExcept, "POP_EXCEPT", 0},
		{Throw, "THROW", 0},
		{EndFinally, "END_FINALLY", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := GetInfo(tt.code)
			assert.Equal(t, info.Code, tt.code)
			assert.Equal(t, info.Name, tt.name)
			assert.Equal(t, info.OperandCount, tt.operands)
		})
	}
}

func TestGetInfoInvalid(t *testing.T) {
	// Test with Invalid opcode
	info := GetInfo(Invalid)
	assert.Equal(t, info.Code, Code(0))
	assert.Equal(t, info.Name, "")
	assert.Equal(t, info.OperandCount, 0)
}

func TestBinaryOpTypeString(t *testing.T) {
	tests := []struct {
		op   BinaryOpType
		want string
	}{
		{Add, "+"},
		{Subtract, "-"},
		{Multiply, "*"},
		{Divide, "/"},
		{Modulo, "%"},
		{And, "&&"},
		{Or, "||"},
		{Xor, "^"},
		{Power, "**"},
		{LShift, "<<"},
		{RShift, ">>"},
		{BitwiseAnd, "&^"},
		{BitwiseOr, "|^"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			assert.Equal(t, tt.op.String(), tt.want)
		})
	}
}

func TestBinaryOpTypeStringInvalid(t *testing.T) {
	// Test with an unknown BinaryOpType
	invalid := BinaryOpType(255)
	assert.Equal(t, invalid.String(), "")
}

func TestCompareOpTypeString(t *testing.T) {
	tests := []struct {
		op   CompareOpType
		want string
	}{
		{LessThan, "<"},
		{LessThanOrEqual, "<="},
		{Equal, "=="},
		{NotEqual, "!="},
		{GreaterThan, ">"},
		{GreaterThanOrEqual, ">="},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			assert.Equal(t, tt.op.String(), tt.want)
		})
	}
}

func TestCompareOpTypeStringInvalid(t *testing.T) {
	// Test with an unknown CompareOpType
	invalid := CompareOpType(255)
	assert.Equal(t, invalid.String(), "")
}

func TestOpcodeConstants(t *testing.T) {
	// Verify opcode constants have expected values
	assert.Equal(t, Invalid, Code(0))
	assert.Equal(t, Nop, Code(1))
	assert.Equal(t, Halt, Code(2))
	assert.Equal(t, Call, Code(3))
	assert.Equal(t, ReturnValue, Code(4))
	assert.Equal(t, CallSpread, Code(7))
	assert.Equal(t, JumpBackward, Code(10))
	assert.Equal(t, JumpForward, Code(11))
	assert.Equal(t, LoadAttr, Code(20))
	assert.Equal(t, StoreAttr, Code(30))
	assert.Equal(t, BinaryOp, Code(40))
	assert.Equal(t, BuildList, Code(50))
	assert.Equal(t, BinarySubscr, Code(60))
	assert.Equal(t, Swap, Code(70))
	assert.Equal(t, Nil, Code(80))
	assert.Equal(t, LoadClosure, Code(120))
	assert.Equal(t, Partial, Code(130))
	assert.Equal(t, PushExcept, Code(140))
}

func TestBinaryOpTypeConstants(t *testing.T) {
	// Verify BinaryOpType constants have expected values
	assert.Equal(t, Add, BinaryOpType(1))
	assert.Equal(t, Subtract, BinaryOpType(2))
	assert.Equal(t, Multiply, BinaryOpType(3))
	assert.Equal(t, Divide, BinaryOpType(4))
	assert.Equal(t, Modulo, BinaryOpType(5))
	assert.Equal(t, And, BinaryOpType(6))
	assert.Equal(t, Or, BinaryOpType(7))
	assert.Equal(t, Xor, BinaryOpType(8))
	assert.Equal(t, Power, BinaryOpType(9))
	assert.Equal(t, LShift, BinaryOpType(10))
	assert.Equal(t, RShift, BinaryOpType(11))
	assert.Equal(t, BitwiseAnd, BinaryOpType(12))
	assert.Equal(t, BitwiseOr, BinaryOpType(13))
}

func TestCompareOpTypeConstants(t *testing.T) {
	// Verify CompareOpType constants have expected values
	assert.Equal(t, LessThan, CompareOpType(1))
	assert.Equal(t, LessThanOrEqual, CompareOpType(2))
	assert.Equal(t, Equal, CompareOpType(3))
	assert.Equal(t, NotEqual, CompareOpType(4))
	assert.Equal(t, GreaterThan, CompareOpType(5))
	assert.Equal(t, GreaterThanOrEqual, CompareOpType(6))
}
