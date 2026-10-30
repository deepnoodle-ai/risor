package bytecode

import (
	"fmt"

	"github.com/deepnoodle-ai/risor/v2/pkg/op"
)

// InstructionIter iterates over instructions in a Code object.
type InstructionIter struct {
	code *Code
	pos  int
	err  error
}

// Next returns the next instruction and its operands.
// Returns false when there are no more instructions, or when the next
// instruction has an unknown opcode or is missing operands. Call Err to tell
// these cases apart.
func (i *InstructionIter) Next() ([]op.Code, bool) {
	if i.err != nil || i.pos >= i.code.InstructionCount() {
		return nil, false
	}
	instr, err := decodeInstruction(i.code.instructions, i.pos)
	if err != nil {
		i.err = err
		return nil, false
	}
	i.pos += len(instr)
	return instr, true
}

// Err returns the error that stopped the iteration, or nil if the iteration
// reached the end of the instructions or has not stopped yet.
func (i *InstructionIter) Err() error {
	return i.err
}

// All returns all instructions as a newly allocated slice.
// This is a convenience method that collects all results from Next().
// It stops at the first invalid instruction; check Err afterward.
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

// decodeInstruction returns the instruction at offset pos: the opcode followed
// by its operands. It returns an error if the opcode is unknown or if the
// instructions end before all operands are read.
func decodeInstruction(instructions []op.Code, pos int) ([]op.Code, error) {
	opcode := instructions[pos]
	info := op.GetInfo(opcode)
	if info.Name == "" {
		return nil, fmt.Errorf("unknown opcode %d at offset %d", opcode, pos)
	}
	end := pos + 1 + info.OperandCount
	if end > len(instructions) {
		return nil, fmt.Errorf("truncated %s instruction at offset %d", info.Name, pos)
	}
	instr := make([]op.Code, end-pos)
	copy(instr, instructions[pos:end])
	return instr, nil
}
