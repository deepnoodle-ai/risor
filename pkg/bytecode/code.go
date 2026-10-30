package bytecode

import (
	"strconv"
	"strings"
	"sync"
	"unicode"

	"github.com/deepnoodle-ai/risor/v2/pkg/op"
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

	// Global variable names, set only on root code. Every code shares the
	// root's globals, so the accessors on other code read the root's names.
	// A slot for a variable declared in a block has a name from
	// BlockGlobalName.
	globalNames []string

	// Local variable names (for debugging/disassembly)
	localNames []string

	// envKeys stores the names of globals that were provided via the
	// environment at compile time (as opposed to globals defined in the
	// script itself). Used for validation at run time.
	envKeys []string

	// sourceLines holds the source split into lines. It is computed once,
	// on first use, and only on root code.
	sourceLines     []string
	sourceLinesOnce sync.Once
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

// GlobalNameCount returns the number of global variable names of the root
// code.
func (c *Code) GlobalNameCount() int {
	return len(c.Root().globalNames)
}

// GlobalNameAt returns the global variable name at the given index, from the
// root code. Returns an empty string if the index is out of range.
func (c *Code) GlobalNameAt(index int) string {
	names := c.Root().globalNames
	if index < 0 || index >= len(names) {
		return ""
	}
	return names[index]
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

	// Use the source of the root for accurate line lookups in nested functions
	lines := c.Root().lines()
	if lineNum > len(lines) {
		return ""
	}
	return lines[lineNum-1]
}

// lines returns the source split into lines, splitting it on first use.
func (c *Code) lines() []string {
	c.sourceLinesOnce.Do(func() {
		if c.source != "" {
			c.sourceLines = strings.Split(c.source, "\n")
		}
	})
	return c.sourceLines
}

// Root returns the root (entrypoint) code that contains this code. The root
// of a root code is itself. Function code addresses globals by slot index in
// the layout of its root code, so the root tells which globals it expects.
func (c *Code) Root() *Code {
	root := c
	for root.parent != nil {
		root = root.parent
	}
	return root
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

// GlobalNames returns a copy of the names of the root (top-level) global
// variables. It excludes the slots of variables declared in blocks, so index i
// of the result does not always match slot i. To read every slot name, use
// GlobalCount and GlobalNameAt.
func (c *Code) GlobalNames() []string {
	var names []string
	for _, name := range c.Root().globalNames {
		if !IsBlockGlobalName(name) {
			names = append(names, name)
		}
	}
	return names
}

// blockGlobalSeparator cannot occur in an identifier, so a block global name
// never equals the name of a root global.
const blockGlobalSeparator = "#"

// BlockGlobalName returns the global slot name for a variable that is declared
// in a block at the top level of a script, such as in an if or try block. The
// name is "<name>#<slot index>". It is unique within the code, so a block
// variable never shadows a root variable with the same name when globals are
// looked up or carried over by name.
//
// It is exported for use by the compiler and the VM. The "#" form is an
// implementation detail, not a stable API: hosts must not build or parse
// these names.
func BlockGlobalName(name string, index int) string {
	return name + blockGlobalSeparator + strconv.Itoa(index)
}

// IsBlockGlobalName returns true if name has the exact form that
// BlockGlobalName makes: an identifier, "#", and one or more decimal digits.
// Other names that contain "#", such as the env key "a#b", are not block
// global names.
//
// It is exported for use by the compiler and the VM. The "#" form is an
// implementation detail, not a stable API: hosts must not build or parse
// these names.
func IsBlockGlobalName(name string) bool {
	prefix, suffix, found := strings.Cut(name, blockGlobalSeparator)
	if !found || prefix == "" || suffix == "" {
		return false
	}
	for i, ch := range prefix {
		if ch == '_' || unicode.IsLetter(ch) {
			continue
		}
		if i > 0 && unicode.IsDigit(ch) {
			continue
		}
		return false
	}
	for _, ch := range suffix {
		if ch < '0' || ch > '9' {
			return false
		}
	}
	return true
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
