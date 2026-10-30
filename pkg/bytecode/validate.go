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
//   - LOAD_CELL, LOAD_FREE, and STORE_FREE index the free variables of the
//     closure that runs the code. A block that some instruction loads as a
//     function must have fewer free variables than the smallest capture
//     count of those loads: LOAD_CLOSURE gives its capture count, and
//     LOAD_CONST gives zero. The root has no free variables. A block that no
//     instruction loads is not checked, and the VM still checks each index
//     at run time, since a host can call a function with fewer captures.
//   - Jump targets, PUSH_EXCEPT targets, and exception handler positions
//     are the start of an instruction or the end of the code.
//   - Constants have a type the VM supports, and each function constant
//     has code, at most 256 parameters, no more defaults than parameters,
//     and only nil, bool, int, float, or string defaults.
//   - The local count of each block is no larger than the block needs: its
//     highest local operand, or its function's parameters and rest
//     parameter, plus one slot for a named function's own name. The root
//     may also have one local per global, as the compiler emits. The VM
//     gives every call frame this many slots, so a larger count would only
//     waste memory.
//   - The local and global counts are in the range of a uint16 operand.
//   - Source location columns and end columns are from 0 to [math.MaxInt32].
//     They are not checked against the source text, since code compiled
//     without its original source stores a reformatted copy whose lines
//     may be shorter than the columns. Error formatting clamps carets to
//     the source line instead.
//
// Validate does not check how instructions use the value stack, the types of
// values at run time, or MAKE_CELL operands for an enclosing frame (frames
// back > 0), which depend on the call stack at run time. See the package documentation for what this means for
// bytecode from an untrusted source.
func Validate(code *Code) error {
	if code == nil {
		return fmt.Errorf("invalid bytecode: nil code")
	}
	v := &validator{
		root:        code,
		globalCount: code.GlobalCount(),
		paramSlots:  map[*Code]int{},
	}
	codes, err := v.collect(code)
	if err != nil {
		return err
	}
	v.freeCounts = freeCounts(code, codes)
	for _, c := range codes {
		if err := v.validateCode(c); err != nil {
			name := c.Name()
			if name == "" {
				name = c.ID()
			}
			return fmt.Errorf("invalid bytecode in %q: %w", name, err)
		}
	}
	return nil
}

type validator struct {
	root        *Code
	globalCount int
	// paramSlots maps a function's code to the local slots its parameters
	// and rest parameter fill when the function is called.
	paramSlots map[*Code]int
	// freeCounts maps a code block to the fewest free variables a closure
	// that runs it may have. A block that is not in the map is not checked.
	freeCounts map[*Code]int
}

// freeCounts returns, for each code block that an instruction loads as a
// function, the smallest number of free variables it is loaded with. It
// skips instructions it cannot decode or check, since validateCode reports
// them.
func freeCounts(root *Code, codes []*Code) map[*Code]int {
	counts := map[*Code]int{root: 0}
	record := func(code *Code, constIdx, count int) {
		if constIdx >= len(code.constants) {
			return
		}
		fn, ok := code.constants[constIdx].(*Function)
		if !ok || fn.code == nil {
			return
		}
		if prev, ok := counts[fn.code]; !ok || count < prev {
			counts[fn.code] = count
		}
	}
	for _, code := range codes {
		for pos := 0; pos < len(code.instructions); {
			instr, err := decodeInstruction(code.instructions, pos)
			if err != nil {
				break
			}
			switch instr[0] {
			case op.LoadClosure:
				record(code, int(instr[1]), int(instr[2]))
			case op.LoadConst:
				record(code, int(instr[1]), 0)
			}
			pos += len(instr)
		}
	}
	return counts
}

// collect returns every code block reachable from root, through children and
// function constants, and fills in v.paramSlots. It uses a work list rather
// than recursion, since the nesting depth comes from the input.
func (v *validator) collect(root *Code) ([]*Code, error) {
	seen := map[*Code]bool{root: true}
	codes := []*Code{root}
	for i := 0; i < len(codes); i++ {
		code := codes[i]
		next := make([]*Code, 0, len(code.children))
		for _, child := range code.children {
			if child == nil {
				return nil, fmt.Errorf("invalid bytecode in %q: nil child", code.Name())
			}
			next = append(next, child)
		}
		for _, constant := range code.constants {
			fn, ok := constant.(*Function)
			if !ok || fn.code == nil {
				continue // validateConstant reports a function without code
			}
			slots := len(fn.parameters)
			if fn.restParam != "" {
				slots++
			}
			v.paramSlots[fn.code] = max(v.paramSlots[fn.code], slots)
			next = append(next, fn.code)
		}
		for _, c := range next {
			if !seen[c] {
				seen[c] = true
				codes = append(codes, c)
			}
		}
	}
	return codes, nil
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
	if err := v.checkLocalCount(code, decoded); err != nil {
		return err
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
		case op.MakeCell:
			// A cell for the current frame (0 frames back) refers to a local
			if operand(2) == 0 {
				err = checkIndex("local", operand(1), code.localCount)
			}
		case op.LoadCell, op.LoadFree, op.StoreFree:
			if count, ok := v.freeCounts[code]; ok {
				err = checkIndex("free variable", operand(1), count)
			}
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

	for i, loc := range code.locations {
		if loc.Column < 0 || loc.Column > math.MaxInt32 ||
			loc.EndColumn < 0 || loc.EndColumn > math.MaxInt32 {
			return fmt.Errorf("location %d has an invalid column (%d to %d)",
				i, loc.Column, loc.EndColumn)
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

// checkLocalCount checks that the local count is no larger than the code
// needs. See Validate for the rule. It must match what the compiler emits:
// a named function may have its name slot after locals that no instruction
// uses, such as a destructured parameter's hidden slot, so the name slot is
// added after taking the maximum.
func (v *validator) checkLocalCount(code *Code, decoded [][]op.Code) error {
	needed := v.paramSlots[code]
	for _, instr := range decoded {
		switch instr[0] {
		case op.LoadFast, op.StoreFast:
			needed = max(needed, int(instr[1])+1)
		case op.MakeCell:
			// A cell for the current frame (0 frames back) refers to a local
			if instr[2] == 0 {
				needed = max(needed, int(instr[1])+1)
			}
		}
	}
	if code.isNamed {
		needed++
	}
	if code == v.root {
		needed = max(needed, code.globalCount)
	}
	if code.localCount > needed {
		return fmt.Errorf("local count %d is larger than the %d locals the code uses",
			code.localCount, needed)
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
