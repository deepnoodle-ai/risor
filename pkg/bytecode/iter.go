package bytecode

import "github.com/deepnoodle-ai/risor/v2/pkg/op"

// InstructionIter iterates over instructions in a Code object.
type InstructionIter struct {
	code *Code
	pos  int
}

// Next returns the next instruction and its operands.
// Returns false when there are no more instructions.
func (i *InstructionIter) Next() ([]op.Code, bool) {
	if i.pos >= i.code.InstructionCount() {
		return nil, false
	}
	opcode := i.code.InstructionAt(i.pos)
	i.pos++

	info := op.GetInfo(opcode)
	if info.OperandCount == 0 {
		return []op.Code{opcode}, true
	}
	instr := make([]op.Code, info.OperandCount+1)
	instr[0] = opcode

	for j := 0; j < info.OperandCount; j++ {
		instr[j+1] = i.code.InstructionAt(i.pos)
		i.pos++
	}
	return instr, true
}

// All returns all instructions as a newly allocated slice.
// This is a convenience method that collects all results from Next().
func (i *InstructionIter) All() [][]op.Code {
	var results [][]op.Code
	for {
		instr, ok := i.Next()
		if !ok {
			break
		}
		results = append(results, instr)
	}
	return results
}

// NewInstructionIter creates a new instruction iterator for the given code.
func NewInstructionIter(code *Code) *InstructionIter {
	return &InstructionIter{code: code}
}
