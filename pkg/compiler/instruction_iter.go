package compiler

import "github.com/deepnoodle-ai/risor/v2/pkg/op"

type InstructionIter struct {
	code *Code
	pos  int
}

func (i *InstructionIter) Next() ([]op.Code, bool) {
	if i.pos >= len(i.code.instructions) {
		return nil, false
	}
	code := i.code.instructions[i.pos]
	i.pos++

	info := op.GetInfo(code)
	if info.OperandCount == 0 {
		return []op.Code{code}, true
	}
	instr := make([]op.Code, info.OperandCount+1)
	instr[0] = code

	for j := 0; j < info.OperandCount; j++ {
		instr[j+1] = i.code.instructions[i.pos]
		i.pos++
	}
	return instr, true
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
