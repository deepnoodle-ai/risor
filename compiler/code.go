package compiler

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/risor-io/risor/errz"
	"github.com/risor-io/risor/op"
)

type loop struct {
	code        *Code
	continuePos []int
	breakPos    []int
	isRangeLoop bool
}

func (l *loop) end() {
	code := l.code
	code.loops = code.loops[:len(code.loops)-1]
}

// ExceptionHandler describes a try/catch/finally block for exception handling.
type ExceptionHandler struct {
	TryStart     int // IP where try block starts
	TryEnd       int // IP where try block ends (points to PopExcept)
	CatchStart   int // IP of catch block (0 if none)
	FinallyStart int // IP of finally block (0 if none)
	CatchVarIdx  int // Local index for catch var (-1 if none)
}

type Code struct {
	id           string
	name         string
	isNamed      bool
	parent       *Code
	children     []*Code
	symbols      *SymbolTable
	instructions []op.Code
	constants    []any
	names        []string
	source       string
	functionID   string
	filename     string // The source file this code came from

	// Source map: one location per instruction for error reporting
	locations []errz.SourceLocation

	// Metadata for VM optimizations (computed during compilation)
	maxCallArgs uint16 // Maximum argument count from any Call opcode in this code

	// Exception handlers for try/catch/finally
	exceptionHandlers []*ExceptionHandler

	// Used during compilation only
	loops      []*loop
	pipeActive bool
}

func (c *Code) ID() string {
	return c.id
}

func (c *Code) CodeName() string {
	return c.name
}

func (c *Code) addName(name string) uint16 {
	c.names = append(c.names, name)
	return uint16(len(c.names) - 1)
}

func (c *Code) IsNamed() bool {
	return c.isNamed
}

func (c *Code) FunctionID() string {
	return c.functionID
}

func (c *Code) Parent() *Code {
	return c.parent
}

func (c *Code) newChild(name, source, funcID string) *Code {
	child := &Code{
		id:         fmt.Sprintf("%s.%d", c.id, len(c.children)),
		name:       name,
		isNamed:    name != "",
		parent:     c,
		symbols:    c.symbols.NewChild(),
		source:     source,
		functionID: funcID,
		filename:   c.filename, // Inherit filename from parent
	}
	c.children = append(c.children, child)
	return child
}

func (c *Code) InstructionCount() int {
	return len(c.instructions)
}

func (c *Code) Instruction(index int) op.Code {
	return c.instructions[index]
}

func (c *Code) ConstantsCount() int {
	return len(c.constants)
}

func (c *Code) Constant(index int) any {
	return c.constants[index]
}

func (c *Code) NameCount() int {
	return len(c.names)
}

func (c *Code) Name(index int) string {
	return c.names[index]
}

func (c *Code) Source() string {
	return c.source
}

func (c *Code) LocalsCount() int {
	return int(c.symbols.Count())
}

// MaxCallArgs returns the maximum argument count from any Call opcode in this code.
// This is used by the VM for optimization purposes.
func (c *Code) MaxCallArgs() int {
	return int(c.maxCallArgs)
}

func (c *Code) Local(index int) *Symbol {
	return c.symbols.Symbol(uint16(index))
}

func (c *Code) GlobalsCount() int {
	return int(c.symbols.Root().Count())
}

func (c *Code) Global(index int) *Symbol {
	return c.symbols.Root().Symbol(uint16(index))
}

func (c *Code) GlobalNames() []string {
	root := c.symbols.Root()
	count := root.Count()
	names := make([]string, count)
	for i := uint16(0); i < count; i++ {
		names[i] = root.Symbol(i).Name()
	}
	return names
}

func (c *Code) Root() *Code {
	curr := c
	for curr.parent != nil {
		curr = curr.parent
	}
	return curr
}

func (c *Code) IsRoot() bool {
	return c.parent == nil
}

func (c *Code) MarshalJSON() ([]byte, error) {
	state, err := stateFromCode(c)
	if err != nil {
		return nil, err
	}
	return json.Marshal(state)
}

func (c *Code) Flatten() []*Code {
	var codes []*Code
	codes = append(codes, c)
	for _, child := range c.children {
		codes = append(codes, child.Flatten()...)
	}
	return codes
}

func (c *Code) Filename() string {
	return c.filename
}

// LocationAt returns the source location for the instruction at the given index.
// If no location is recorded, an empty SourceLocation is returned.
func (c *Code) LocationAt(ip int) errz.SourceLocation {
	if ip < 0 || ip >= len(c.locations) {
		return errz.SourceLocation{}
	}
	return c.locations[ip]
}

// LocationsCount returns the number of recorded source locations.
func (c *Code) LocationsCount() int {
	return len(c.locations)
}

// GetSourceLine returns the source code line at the given 1-based line number.
// If the line is out of range, an empty string is returned.
func (c *Code) GetSourceLine(lineNum int) string {
	if c.source == "" || lineNum < 1 {
		return ""
	}
	lines := strings.Split(c.source, "\n")
	if lineNum > len(lines) {
		return ""
	}
	return lines[lineNum-1]
}

// ExceptionHandlers returns all exception handlers in this code.
func (c *Code) ExceptionHandlers() []*ExceptionHandler {
	return c.exceptionHandlers
}

// AddExceptionHandler adds an exception handler to this code.
func (c *Code) AddExceptionHandler(handler *ExceptionHandler) {
	c.exceptionHandlers = append(c.exceptionHandlers, handler)
}
