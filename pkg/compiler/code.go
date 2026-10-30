package compiler

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/deepnoodle-ai/risor/v2/pkg/bytecode"
	"github.com/deepnoodle-ai/risor/v2/pkg/errors"
	"github.com/deepnoodle-ai/risor/v2/pkg/op"
)

// ExceptionHandler describes a try/catch/finally block for exception handling.
type ExceptionHandler struct {
	TryStart     int // IP where try block starts
	TryEnd       int // IP where try block ends (points to PopExcept)
	CatchStart   int // IP of catch block (0 if none)
	FinallyStart int // IP of finally block (0 if none)
	CatchVarIdx  int // Local index for catch var (-1 if none)
}

type Code struct {
	// id is empty for a child whose ID is derived from its parent's ID and
	// its position, as it is for every compiled child. See ID.
	id           string
	name         string
	isNamed      bool
	parent       *Code
	position     int // Index among the parent's children
	children     []*Code
	symbols      *SymbolTable
	instructions []op.Code
	constants    []any
	names        []string
	source       string
	functionID   string
	filename     string // The source file this code came from

	// nameIndex maps each entry of names to its index, so that addName
	// stores each attribute name once.
	nameIndex map[string]uint16

	// rootSource points to the source of the root Code, which line numbers
	// in every code refer to. It is nil for the root itself.
	rootSource *string

	// sourceStart and sourceEnd give the byte range of source within the
	// root code's source, when source is a slice of it. Both are zero
	// otherwise. Marshaling writes the range in place of the text.
	sourceStart int
	sourceEnd   int

	// Source map: one location per instruction for error reporting
	locations []errors.SourceLocation

	// Metadata for VM optimizations (computed during compilation)
	maxCallArgs uint16 // Maximum argument count from any Call opcode in this code

	// Exception handlers for try/catch/finally
	exceptionHandlers []*ExceptionHandler

	// envKeys stores the names of globals from the compile-time env.
	// Only set on root code. Used for validation at run time.
	envKeys []string

	// Used during compilation only
	pipeActive bool
}

// codeSnapshot captures the state of a Code object for rollback.
type codeSnapshot struct {
	instructionLen    int
	constantLen       int
	nameLen           int
	locationLen       int
	childLen          int
	exceptionHandlers int
	source            string
	maxCallArgs       uint16
}

// snapshot captures the current state of the Code so it can be restored
// later if a compilation attempt fails.
func (c *Code) snapshot() codeSnapshot {
	return codeSnapshot{
		instructionLen:    len(c.instructions),
		constantLen:       len(c.constants),
		nameLen:           len(c.names),
		locationLen:       len(c.locations),
		childLen:          len(c.children),
		exceptionHandlers: len(c.exceptionHandlers),
		source:            c.source,
		maxCallArgs:       c.maxCallArgs,
	}
}

// restore reverts the Code to a previous snapshot, truncating any bytecode,
// constants, names, locations, children, or exception handlers that were
// appended after the snapshot was taken.
func (c *Code) restore(s codeSnapshot) {
	c.instructions = c.instructions[:s.instructionLen]
	c.constants = c.constants[:s.constantLen]
	for _, name := range c.names[s.nameLen:] {
		delete(c.nameIndex, name)
	}
	c.names = c.names[:s.nameLen]
	c.locations = c.locations[:s.locationLen]
	c.children = c.children[:s.childLen]
	c.exceptionHandlers = c.exceptionHandlers[:s.exceptionHandlers]
	c.source = s.source
	c.maxCallArgs = s.maxCallArgs
}

// ID returns the ID of the code. The root has the ID it was created with,
// such as "__main__". A child has its parent's ID, a dot, and its index among
// the parent's children, such as "__main__.0.1". A child's ID is built on each
// call, not stored, so the memory used by IDs does not grow with the square of
// the nesting depth. A child loaded by UnmarshalCode from older data has the
// ID stored there.
func (c *Code) ID() string {
	if c.id != "" || c.parent == nil {
		return c.id
	}
	var positions []int
	code := c
	for code.id == "" && code.parent != nil {
		positions = append(positions, code.position)
		code = code.parent
	}
	return pathID(code.id, positions)
}

// pathID returns base followed by a dot and each position, from the last
// position to the first. Callers collect positions from a node up to the
// nearest ancestor with a stored ID, which is base.
func pathID(base string, positions []int) string {
	var b strings.Builder
	b.WriteString(base)
	for i := len(positions) - 1; i >= 0; i-- {
		b.WriteByte('.')
		b.WriteString(strconv.Itoa(positions[i]))
	}
	return b.String()
}

func (c *Code) CodeName() string {
	return c.name
}

// addName returns the index of name in the names table, adding it if it is
// not there yet. It returns an error if the table is full: name operands are
// uint16, and the compiler reserves math.MaxUint16 as a placeholder.
func (c *Code) addName(name string) (uint16, error) {
	if idx, ok := c.nameIndex[name]; ok {
		return idx, nil
	}
	if len(c.names) >= math.MaxUint16 {
		return 0, fmt.Errorf("compile error: number of attribute names exceeded limits")
	}
	if c.nameIndex == nil {
		c.nameIndex = map[string]uint16{}
	}
	idx := uint16(len(c.names))
	c.names = append(c.names, name)
	c.nameIndex[name] = idx
	return idx, nil
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
	// Propagate rootSource from parent: if parent has a rootSource, use it;
	// otherwise the parent is the root, so use its source.
	rootSrc := c.rootSource
	if rootSrc == nil {
		rootSrc = &c.source
	}
	child := &Code{
		name:       name,
		isNamed:    name != "",
		parent:     c,
		position:   len(c.children),
		symbols:    c.symbols.NewChild(),
		source:     source,
		functionID: funcID,
		filename:   c.filename, // Inherit filename from parent
		rootSource: rootSrc,
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

// GlobalNames returns the name of each global slot. A variable declared in a
// block at the top level gets a slot name from bytecode.BlockGlobalName, so it
// is never mistaken for the root variable with the same name.
func (c *Code) GlobalNames() []string {
	root := c.symbols.Root()
	count := root.Count()
	names := make([]string, count)
	for i := uint16(0); i < count; i++ {
		sym := root.Symbol(i)
		if sym == nil {
			names[i] = BlankIdentifier // Slot for discarded value
		} else if rootSym, ok := root.Get(sym.Name()); ok && rootSym == sym {
			names[i] = sym.Name()
		} else {
			names[i] = bytecode.BlockGlobalName(sym.Name(), int(i))
		}
	}
	return names
}

func (c *Code) LocalNames() []string {
	count := c.symbols.Count()
	names := make([]string, count)
	for i := uint16(0); i < count; i++ {
		sym := c.symbols.Symbol(i)
		if sym != nil {
			names[i] = sym.Name()
		} else {
			names[i] = BlankIdentifier // Slot for discarded value
		}
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
func (c *Code) LocationAt(ip int) errors.SourceLocation {
	if ip < 0 || ip >= len(c.locations) {
		return errors.SourceLocation{}
	}
	return c.locations[ip]
}

// LocationsCount returns the number of recorded source locations.
func (c *Code) LocationsCount() int {
	return len(c.locations)
}

// GetSourceLine returns the source code line at the given 1-based line number.
// If the line is out of range or there is no source, an empty string is
// returned. Line numbers refer to the root code's source, also in nested
// functions.
func (c *Code) GetSourceLine(lineNum int) string {
	return sourceLine(strings.Split(c.lineSource(), "\n"), lineNum)
}

// lineSource returns the source that line numbers in c refer to: the root
// code's source. A function's own source is its body, which line numbers do
// not index.
func (c *Code) lineSource() string {
	if c.rootSource != nil {
		return *c.rootSource
	}
	return c.source
}

// sourceLine returns the given 1-based line of lines, or an empty string if
// the line is out of range or there is no source.
func sourceLine(lines []string, lineNum int) string {
	if lineNum < 1 || lineNum > len(lines) {
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

// ToBytecode converts this mutable Code to an immutable bytecode.Code.
// This recursively converts all child code blocks and Function constants.
// The conversion is done bottom-up to ensure true immutability - children
// are fully constructed before their parent.
//
// The result is a root bytecode.Code, even when c is a child (for example
// from Flatten or Function.Code), so it always gets the global names.
func (c *Code) ToBytecode() *bytecode.Code {
	// Build a map from compiler.Code to bytecode.Code for function linking
	codeMap := make(map[*Code]*bytecode.Code)
	// Global names are a property of the root, which every code shares.
	// Building them for each function would make compile time grow with the
	// number of functions times the number of globals, so only the top-level
	// target of the conversion gets them. The target is a root, so it gets
	// its full ID. Each child keeps its stored ID, which is empty when the ID
	// is derived from its position: bytecode.Code derives the same ID.
	return c.toBytecodeWithMap(codeMap, c.ID(), c.GlobalNames())
}

// toBytecodeWithMap converts c and its children, giving the result the ID id.
// globalNames is set only for the top-level target of the conversion and is
// nil for its children.
func (c *Code) toBytecodeWithMap(codeMap map[*Code]*bytecode.Code, id string, globalNames []string) *bytecode.Code {
	// Step 1: Recursively convert all children first (bottom-up construction)
	// This ensures child bytecode.Code objects exist before we need them for constants
	children := make([]*bytecode.Code, len(c.children))
	for i, child := range c.children {
		children[i] = child.toBytecodeWithMap(codeMap, child.id, nil)
	}

	// Step 2: Convert exception handlers
	handlers := make([]bytecode.ExceptionHandler, len(c.exceptionHandlers))
	for i, h := range c.exceptionHandlers {
		handlers[i] = bytecode.ExceptionHandler{
			TryStart:     h.TryStart,
			TryEnd:       h.TryEnd,
			CatchStart:   h.CatchStart,
			FinallyStart: h.FinallyStart,
			CatchVarIdx:  h.CatchVarIdx,
		}
	}

	// Step 3: Convert source locations
	locations := make([]bytecode.SourceLocation, len(c.locations))
	for i, loc := range c.locations {
		locations[i] = bytecode.SourceLocation{
			Line:      loc.Line,
			Column:    loc.Column,
			EndColumn: loc.EndColumn,
		}
	}

	// Step 4: Convert constants, replacing compiler.Function with bytecode.Function
	// At this point, all child codes are in codeMap
	constants := make([]any, len(c.constants))
	for i, constant := range c.constants {
		if fn, ok := constant.(*Function); ok {
			// Get the bytecode.Code for this function from the map
			fnCode, exists := codeMap[fn.code]
			if !exists {
				panic("function code not found in codeMap - function's code should be a child")
			}
			bcFn := bytecode.NewFunction(bytecode.FunctionParams{
				ID:         fn.id,
				Name:       fn.name,
				Parameters: fn.parameters,
				Defaults:   fn.defaults,
				RestParam:  fn.restParam,
				Code:       fnCode,
			})
			constants[i] = bcFn
		} else {
			constants[i] = constant
		}
	}

	// Step 5: Create the immutable bytecode.Code with all data
	bc := bytecode.NewCode(bytecode.CodeParams{
		ID:                id,
		Name:              c.name,
		IsNamed:           c.isNamed,
		Children:          children,
		Instructions:      c.instructions,
		Constants:         constants,
		Names:             c.names,
		Source:            c.source,
		SourceStart:       c.sourceStart,
		SourceEnd:         c.sourceEnd,
		Filename:          c.filename,
		FunctionID:        c.functionID,
		Locations:         locations,
		MaxCallArgs:       int(c.maxCallArgs),
		LocalCount:        c.LocalsCount(),
		GlobalCount:       c.GlobalsCount(),
		GlobalNames:       globalNames,
		LocalNames:        c.LocalNames(),
		EnvKeys:           c.envKeys,
		ExceptionHandlers: handlers,
	})

	// Register in map for use by parent's function constants
	codeMap[c] = bc

	return bc
}
