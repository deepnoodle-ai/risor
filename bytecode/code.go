package bytecode

import (
	"strings"

	"github.com/deepnoodle-ai/risor/v2/op"
)

// Code represents a compiled code block (module, function body, etc.).
// It is immutable after creation and safe for concurrent use.
type Code struct {
	id       string
	name     string
	isNamed  bool
	children []*Code
	parent   *Code // Parent code (nil for root)

	instructions []op.Code
	constants    []any
	names        []string
	source       string
	filename     string
	functionID   string

	// Source map: one location per instruction for error reporting
	locations []SourceLocation

	// Metadata for VM optimizations
	maxCallArgs int
	localCount  int
	globalCount int

	// Exception handlers for try/catch/finally
	exceptionHandlers []ExceptionHandler

	// Global variable names (only set on root code)
	globalNames []string

	// Local variable names (for debugging/disassembly)
	localNames []string

	// envKeys stores the names of globals that were provided via the
	// environment at compile time (as opposed to globals defined in the
	// script itself). Used for validation at run time.
	envKeys []string
}

// CodeParams contains parameters for creating a new Code.
type CodeParams struct {
	ID           string
	Name         string
	IsNamed      bool
	Children     []*Code // Pre-built child code blocks
	Instructions []op.Code
	Constants    []any
	Names        []string
	Source       string
	Filename     string
	FunctionID   string
	Locations    []SourceLocation
	MaxCallArgs  int
	LocalCount   int
	GlobalCount  int
	GlobalNames  []string
	LocalNames   []string
	EnvKeys      []string // Names of globals from compile-time env (for validation)

	ExceptionHandlers []ExceptionHandler
}

// NewCode creates a new immutable Code from the given parameters.
// Input slices are copied to ensure immutability. The Code is fully
// immutable after construction - there are no mutation methods.
func NewCode(params CodeParams) *Code {
	// Copy children slice (children themselves are already immutable *Code)
	var children []*Code
	if len(params.Children) > 0 {
		children = make([]*Code, len(params.Children))
		copy(children, params.Children)
	}

	code := &Code{
		id:                params.ID,
		name:              params.Name,
		isNamed:           params.IsNamed,
		children:          children,
		instructions:      copyInstructions(params.Instructions),
		constants:         copyAny(params.Constants),
		names:             copyStrings(params.Names),
		source:            params.Source,
		filename:          params.Filename,
		functionID:        params.FunctionID,
		locations:         copyLocations(params.Locations),
		maxCallArgs:       params.MaxCallArgs,
		localCount:        params.LocalCount,
		globalCount:       params.GlobalCount,
		globalNames:       copyStrings(params.GlobalNames),
		localNames:        copyStrings(params.LocalNames),
		envKeys:           copyStrings(params.EnvKeys),
		exceptionHandlers: copyHandlers(params.ExceptionHandlers),
	}

	// Set parent reference on all children for source lookups
	for _, child := range code.children {
		child.parent = code
	}

	return code
}

// ID returns the unique identifier for this code block.
func (c *Code) ID() string {
	return c.id
}

// Name returns the name of this code block.
func (c *Code) Name() string {
	return c.name
}

// IsNamed returns true if this is a named function.
func (c *Code) IsNamed() bool {
	return c.isNamed
}

// FunctionID returns the function ID if this code belongs to a function.
func (c *Code) FunctionID() string {
	return c.functionID
}

// ChildCount returns the number of child code blocks.
func (c *Code) ChildCount() int {
	return len(c.children)
}

// ChildAt returns the child code block at the given index.
func (c *Code) ChildAt(index int) *Code {
	return c.children[index]
}

// InstructionCount returns the number of instructions.
func (c *Code) InstructionCount() int {
	return len(c.instructions)
}

// InstructionAt returns the instruction at the given index.
func (c *Code) InstructionAt(index int) op.Code {
	return c.instructions[index]
}

// ConstantCount returns the number of constants.
func (c *Code) ConstantCount() int {
	return len(c.constants)
}

// ConstantAt returns the constant at the given index.
func (c *Code) ConstantAt(index int) any {
	return c.constants[index]
}

// NameCount returns the number of names (attribute names used in this code).
func (c *Code) NameCount() int {
	return len(c.names)
}

// NameAt returns the attribute name at the given index.
func (c *Code) NameAt(index int) string {
	return c.names[index]
}

// Source returns the source code for this block.
func (c *Code) Source() string {
	return c.source
}

// Filename returns the source filename.
func (c *Code) Filename() string {
	return c.filename
}

// LocalCount returns the number of local variables.
func (c *Code) LocalCount() int {
	return c.localCount
}

// GlobalCount returns the number of global variables.
func (c *Code) GlobalCount() int {
	return c.globalCount
}

// MaxCallArgs returns the maximum argument count from any Call opcode.
func (c *Code) MaxCallArgs() int {
	return c.maxCallArgs
}

// LocationAt returns the source location for the instruction at the given index.
func (c *Code) LocationAt(ip int) SourceLocation {
	if ip < 0 || ip >= len(c.locations) {
		return SourceLocation{}
	}
	return c.locations[ip]
}

// LocationCount returns the number of recorded source locations.
func (c *Code) LocationCount() int {
	return len(c.locations)
}

// ExceptionHandlerCount returns the number of exception handlers.
func (c *Code) ExceptionHandlerCount() int {
	return len(c.exceptionHandlers)
}

// ExceptionHandlerAt returns the exception handler at the given index.
func (c *Code) ExceptionHandlerAt(index int) ExceptionHandler {
	return c.exceptionHandlers[index]
}

// GlobalNameCount returns the number of global variable names.
func (c *Code) GlobalNameCount() int {
	return len(c.globalNames)
}

// GlobalNameAt returns the global variable name at the given index.
// Returns an empty string if the index is out of range.
func (c *Code) GlobalNameAt(index int) string {
	if index < 0 || index >= len(c.globalNames) {
		return ""
	}
	return c.globalNames[index]
}

// LocalNameCount returns the number of local variable names.
func (c *Code) LocalNameCount() int {
	return len(c.localNames)
}

// LocalNameAt returns the local variable name at the given index.
// Returns an empty string if the index is out of range.
func (c *Code) LocalNameAt(index int) string {
	if index < 0 || index >= len(c.localNames) {
		return ""
	}
	return c.localNames[index]
}

// Flatten returns this code and all descendants in a flat slice.
// Note: This returns a newly allocated slice, not internal state.
// Modifying the returned slice does not affect the Code object.
func (c *Code) Flatten() []*Code {
	var codes []*Code
	codes = append(codes, c)
	for _, child := range c.children {
		codes = append(codes, child.Flatten()...)
	}
	return codes
}

// GetSourceLine returns the source code line at the given 1-based line number.
// For nested functions, it tries to look up the line from the root code's source
// to get the original source with correct line numbers.
func (c *Code) GetSourceLine(lineNum int) string {
	if lineNum < 1 {
		return ""
	}

	// Try to get source from root for accurate line lookups in nested functions
	source := c.getRootSource()
	if source == "" {
		return ""
	}

	lines := strings.Split(source, "\n")
	if lineNum > len(lines) {
		return ""
	}
	return lines[lineNum-1]
}

// getRootSource returns the source from the root code for accurate line lookups.
func (c *Code) getRootSource() string {
	// Walk up to root to get the full source
	root := c
	for root.parent != nil {
		root = root.parent
	}
	return root.source
}

// Stats returns statistics about this code block.
func (c *Code) Stats() Stats {
	functionCount := 0
	for i := 0; i < c.ConstantCount(); i++ {
		if _, ok := c.ConstantAt(i).(*Function); ok {
			functionCount++
		}
	}
	return Stats{
		InstructionCount: c.InstructionCount(),
		ConstantCount:    c.ConstantCount(),
		GlobalCount:      c.GlobalCount(),
		FunctionCount:    functionCount,
		SourceBytes:      len(c.source),
	}
}

// GlobalNames returns a copy of all global variable names.
func (c *Code) GlobalNames() []string {
	if len(c.globalNames) == 0 {
		return nil
	}
	names := make([]string, len(c.globalNames))
	copy(names, c.globalNames)
	return names
}

// EnvKeys returns a copy of the global names that were provided via the
// environment at compile time. This is a subset of GlobalNames() - it excludes
// globals that were defined within the script itself.
//
// Use this for validation: at run time, ensure the env contains all these keys.
func (c *Code) EnvKeys() []string {
	if len(c.envKeys) == 0 {
		return nil
	}
	keys := make([]string, len(c.envKeys))
	copy(keys, c.envKeys)
	return keys
}

// FunctionNames returns the names of all named functions in this code.
// Anonymous functions are not included.
func (c *Code) FunctionNames() []string {
	var names []string
	for i := 0; i < c.ConstantCount(); i++ {
		if fn, ok := c.ConstantAt(i).(*Function); ok {
			if name := fn.Name(); name != "" {
				names = append(names, name)
			}
		}
	}
	return names
}
