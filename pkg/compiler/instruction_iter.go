package compiler

import (
	"fmt"

	"github.com/deepnoodle-ai/risor/v2/pkg/op"
)

type InstructionIter struct {
	code *Code
	pos  int
	err  error
}

// Next returns the next instruction and its operands. It returns false at the
// end of the instructions, or when the next instruction has an unknown opcode
// or is missing operands. Call Err to tell these cases apart.
func (i *InstructionIter) Next() ([]op.Code, bool) {
	if i.err != nil || i.pos >= len(i.code.instructions) {
		return nil, false
	}
	code := i.code.instructions[i.pos]
	info := op.GetInfo(code)
	if info.Name == "" {
		i.err = fmt.Errorf("unknown opcode %d at offset %d", code, i.pos)
		return nil, false
	}
	end := i.pos + 1 + info.OperandCount
	if end > len(i.code.instructions) {
		i.err = fmt.Errorf("truncated %s instruction at offset %d", info.Name, i.pos)
		return nil, false
	}
	instr := make([]op.Code, end-i.pos)
	copy(instr, i.code.instructions[i.pos:end])
	i.pos = end
	return instr, true
}

// Err returns the error that stopped the iteration, or nil.
func (i *InstructionIter) Err() error {
	return i.err
}

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

func NewInstructionIter(code *Code) *InstructionIter {
	return &InstructionIter{code: code}
}
