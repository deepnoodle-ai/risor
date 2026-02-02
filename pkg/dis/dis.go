// Package dis supports analysis of Risor bytecode by disassembling it.
// This works with the opcodes defined in the `op` package and uses the
// InstructionIter type from the `bytecode` package.
package dis

import (
	"fmt"
	"io"
	"strings"

	"github.com/deepnoodle-ai/risor/v2/internal/table"
	"github.com/deepnoodle-ai/risor/v2/pkg/bytecode"
	"github.com/deepnoodle-ai/risor/v2/pkg/op"
	"github.com/deepnoodle-ai/wonton/color"
)

// Instruction represents a single bytecode instruction and its operands.
type Instruction struct {
	Offset     int
	Name       string
	Opcode     op.Code
	Operands   []op.Code
	Annotation string
	Constant   interface{}
}

// Disassemble returns a parsed representation of the given bytecode.
func Disassemble(code *bytecode.Code) ([]Instruction, error) {
	var instructions []Instruction
	var offset int
	iter := bytecode.NewInstructionIter(code)
	for {
		val, ok := iter.Next()
		if !ok {
			break
		}
		var err error
		info := op.GetInfo(val[0])
		var constant interface{}
		var annotation string
		switch info.Name {
		case "LOAD_FAST", "STORE_FAST":
			annotation, err = getLocalVariableName(code, int(val[1]))
			if err != nil {
				return nil, err
			}
		case "LOAD_GLOBAL", "STORE_GLOBAL":
			annotation, err = getGlobalVariableName(code, int(val[1]))
			if err != nil {
				return nil, err
			}
		case "LOAD_ATTR", "STORE_ATTR":
			nameIndex := int(val[1])
			name, err := getName(code, nameIndex)
			if err != nil {
				return nil, err
			}
			annotation = fmt.Sprintf("%v", name)
		case "BINARY_OP":
			annotation = op.BinaryOpType(val[1]).String()
		case "COMPARE_OP":
			annotation = op.CompareOpType(val[1]).String()
		case "LOAD_CONST":
			constant, err = getConstantValue(code, int(val[1]))
			if err != nil {
				return nil, err
			}
			annotation = fmt.Sprintf("%v", constant)
		}
		instructions = append(instructions, Instruction{
			Offset:     offset,
			Name:       info.Name,
			Opcode:     val[0],
			Operands:   val[1:],
			Annotation: annotation,
			Constant:   constant,
		})
		offset += len(val)
	}
	return instructions, nil
}

// italic applies italic formatting (ANSI code 3) if colors are enabled.
func italic(s string) string {
	if !color.Enabled {
		return s
	}
	return "\033[3m" + s + "\033[0m"
}

// bold applies bold formatting if colors are enabled.
func bold(s string) string {
	if !color.Enabled {
		return s
	}
	return color.ApplyBold(s)
}

// Print a string representation of the given instructions to the given writer.
func Print(instructions []Instruction, writer io.Writer) {
	var lines [][]string
	for _, instr := range instructions {
		var values []string
		values = append(values, fmt.Sprintf("%d", instr.Offset))
		values = append(values, bold(instr.Name))
		values = append(values, formatOperands(instr.Operands))
		if instr.Constant != nil {
			switch c := instr.Constant.(type) {
			case int64:
				values = append(values, color.Colorize(color.Yellow, fmt.Sprintf("%d", c)))
			case float64:
				values = append(values, color.Colorize(color.Yellow, fmt.Sprintf("%f", c)))
			case string:
				if len(c) > 80 {
					c = c[:77] + "..."
				}
				values = append(values, color.Colorize(color.Green, fmt.Sprintf("%q", c)))
			case *bytecode.Function:
				name := c.Name()
				if name == "" {
					name = italic("<anonymous>")
				}
				values = append(values, color.Colorize(color.Magenta, fmt.Sprintf("func:%s", name)))
			default:
				values = append(values, bold(fmt.Sprintf("%v", c)))
			}
		} else if instr.Annotation != "" {
			values = append(values, color.Colorize(color.BrightCyan, fmt.Sprintf("%v", instr.Annotation)))
		} else {
			values = append(values, "")
		}
		lines = append(lines, values)
	}

	table.NewTable(writer).
		WithHeader([]string{"OFFSET", "OPCODE", "OPERANDS", "INFO"}).
		WithColumnAlignment([]table.Alignment{
			table.AlignRight,
			table.AlignLeft,
			table.AlignRight,
			table.AlignLeft,
		}).
		WithHeaderAlignment([]table.Alignment{
			table.AlignCenter,
			table.AlignCenter,
			table.AlignCenter,
			table.AlignCenter,
		}).
		WithRows(lines).
		Render()
}

func formatOperands(ops []op.Code) string {
	var sb strings.Builder
	for i, op := range ops {
		if i > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString(fmt.Sprintf("%d", op))
	}
	return sb.String()
}

func getLocalVariableName(code *bytecode.Code, index int) (string, error) {
	if code.LocalCount() <= index {
		return "", fmt.Errorf("local variable index out of range: %d", index)
	}
	// Try to get the actual name if available
	if name := code.LocalNameAt(index); name != "" {
		return name, nil
	}
	// Fall back to showing the index if no name is stored
	return fmt.Sprintf("local_%d", index), nil
}

func getGlobalVariableName(code *bytecode.Code, index int) (string, error) {
	if code.GlobalCount() <= index {
		return "", fmt.Errorf("global variable index out of range: %d", index)
	}
	return code.GlobalNameAt(index), nil
}

func getConstantValue(code *bytecode.Code, index int) (any, error) {
	if code.ConstantCount() <= index {
		return "", fmt.Errorf("constant index out of range: %d", index)
	}
	return code.ConstantAt(index), nil
}

func getName(code *bytecode.Code, index int) (string, error) {
	if code.NameCount() <= index {
		return "", fmt.Errorf("name index out of range: %d", index)
	}
	return code.NameAt(index), nil
}
