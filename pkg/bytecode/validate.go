package bytecode

import (
	"fmt"
	"math"

	"github.com/deepnoodle-ai/risor/v2/pkg/op"
)

// maxParameters is the most parameters a function may declare. It matches
// the VM's limit on call arguments.
const maxParameters = 256

// Validate checks that code is well formed: that the VM can load it, and
// that every instruction refers to something that exists. [Unmarshal] calls
// it, so code read from bytes has already passed these checks.
//
// Validate checks code and every code block reachable from it, through
// children and function constants:
//
//   - Every opcode is known and has all of its operands.
//   - Constant, attribute name, local, and global operands are in range.
//     Global operands are checked against the global count of code, the
//     root, since every block shares the root's globals.
//   - LOAD_CLOSURE refers to a function constant.
//   - Jump targets, PUSH_EXCEPT targets, and exception handler positions
//     are the start of an instruction or the end of the code.
//   - Constants have a type the VM supports, and each function constant
//     has code, at most 256 parameters, no more defaults than parameters,
//     and only nil, bool, int, float, or string defaults.
//   - Local and global counts are in the range of a uint16 operand.
//
// Validate does not check how instructions use the value stack, the types of
// values at run time, or free variable and cell operands, which depend on
// run-time state. The VM reports those faults as errors. Validated bytecode
// from an untrusted source is therefore not a sandbox: it cannot crash the
// host, but it can do anything a compiled script could do, and its behavior
// on such faults is not specified. Compile untrusted source instead of
// loading untrusted bytecode where you can.
func Validate(code *Code) error {
	if code == nil {
		return fmt.Errorf("invalid bytecode: nil code")
	}
	v := &validator{
		globalCount: code.GlobalCount(),
		seen:        map[*Code]bool{},
	}
	return v.validate(code)
}

type validator struct {
	globalCount int
	seen        map[*Code]bool
}

func (v *validator) validate(code *Code) error {
	if v.seen[code] {
		return nil
	}
	v.seen[code] = true
	if err := v.validateCode(code); err != nil {
		name := code.Name()
		if name == "" {
			name = code.ID()
		}
		return fmt.Errorf("invalid bytecode in %q: %w", name, err)
	}
	for _, child := range code.children {
		if child == nil {
			return fmt.Errorf("invalid bytecode in %q: nil child", code.Name())
		}
		if err := v.validate(child); err != nil {
			return err
		}
	}
	for _, constant := range code.constants {
		if fn, ok := constant.(*Function); ok {
			if err := v.validate(fn.code); err != nil {
				return err
			}
		}
	}
	return nil
}

func (v *validator) validateCode(code *Code) error {
	if code.localCount < 0 || code.localCount > math.MaxUint16 {
		return fmt.Errorf("local count %d out of range", code.localCount)
	}
	if code.globalCount < 0 || code.globalCount > math.MaxUint16 {
		return fmt.Errorf("global count %d out of range", code.globalCount)
	}
	for i, constant := range code.constants {
		if err := validateConstant(constant); err != nil {
			return fmt.Errorf("constant %d: %w", i, err)
		}
	}

	// First pass: decode every instruction and mark where each one starts.
	instructions := code.instructions
	n := len(instructions)
	starts := make([]bool, n+1)
	starts[n] = true // Jumping to the end stops execution
	var decoded [][]op.Code
	var offsets []int
	for pos := 0; pos < n; {
		instr, err := decodeInstruction(instructions, pos)
		if err != nil {
			return err
		}
		starts[pos] = true
		decoded = append(decoded, instr)
		offsets = append(offsets, pos)
		pos += len(instr)
	}
	isTarget := func(target int) bool {
		return target >= 0 && target <= n && starts[target]
	}

	// Second pass: check each operand.
	for i, instr := range decoded {
		pos := offsets[i]
		opcode := instr[0]
		operand := func(j int) int { return int(instr[j]) }
		var err error
		switch opcode {
		case op.LoadConst:
			err = checkIndex("constant", operand(1), len(code.constants))
		case op.LoadAttr, op.LoadAttrOrNil, op.StoreAttr:
			err = checkIndex("name", operand(1), len(code.names))
		case op.LoadFast, op.StoreFast:
			err = checkIndex("local", operand(1), code.localCount)
		case op.LoadGlobal, op.StoreGlobal:
			err = checkIndex("global", operand(1), v.globalCount)
		case op.LoadClosure:
			idx := operand(1)
			if err = checkIndex("constant", idx, len(code.constants)); err == nil {
				if _, ok := code.constants[idx].(*Function); !ok {
					err = fmt.Errorf("constant %d is not a function", idx)
				}
			}
		case op.PopJumpForwardIfFalse, op.PopJumpForwardIfTrue,
			op.PopJumpForwardIfNil, op.PopJumpForwardIfNotNil, op.JumpForward:
			if target := pos + operand(1); !isTarget(target) {
				err = fmt.Errorf("jump target %d is not an instruction", target)
			}
		case op.JumpBackward:
			if target := pos - operand(1); !isTarget(target) {
				err = fmt.Errorf("jump target %d is not an instruction", target)
			}
		case op.PushExcept:
			if target := pos + operand(1); !isTarget(target) {
				err = fmt.Errorf("catch target %d is not an instruction", target)
			} else if operand(2) > 0 && !isTarget(pos+operand(2)) {
				err = fmt.Errorf("finally target %d is not an instruction", pos+operand(2))
			}
		}
		if err != nil {
			return fmt.Errorf("%s at offset %d: %w", op.GetInfo(opcode).Name, pos, err)
		}
	}

	for i, h := range code.exceptionHandlers {
		if h.TryStart > h.TryEnd || !isTarget(h.TryStart) || !isTarget(h.TryEnd) ||
			!isTarget(h.CatchStart) || !isTarget(h.FinallyStart) {
			return fmt.Errorf("exception handler %d has an invalid position", i)
		}
	}
	return nil
}

func checkIndex(kind string, idx, count int) error {
	if idx >= count {
		return fmt.Errorf("%s index %d out of range (count %d)", kind, idx, count)
	}
	return nil
}

func validateConstant(constant any) error {
	switch c := constant.(type) {
	case nil, bool, int, int64, float64, string:
		return nil
	case *Function:
		if c.code == nil {
			return fmt.Errorf("function %q has no code", c.name)
		}
		if len(c.parameters) > maxParameters {
			return fmt.Errorf("function %q has %d parameters (max %d)",
				c.name, len(c.parameters), maxParameters)
		}
		if len(c.defaults) > len(c.parameters) {
			return fmt.Errorf("function %q has more defaults than parameters", c.name)
		}
		for _, d := range c.defaults {
			switch d.(type) {
			case nil, bool, int, int64, float64, string:
			default:
				return fmt.Errorf("function %q has a default of unsupported type %T", c.name, d)
			}
		}
		return nil
	default:
		return fmt.Errorf("unsupported type %T", constant)
	}
}
