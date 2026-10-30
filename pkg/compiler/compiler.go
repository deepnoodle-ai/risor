// Package compiler is used to compile a Risor abstract syntax tree (AST) into
// the corresponding bytecode.
//
// # Two-Pass Compilation Strategy
//
// The compiler uses a two-pass approach to handle forward references. This
// allows functions to call other functions that are defined later in the source.
//
// Example where forward references are needed:
//
//	function isEven(n) { return n == 0 || isOdd(n - 1) }
//	function isOdd(n) { return n != 0 && isEven(n - 1) }
//
// Without the first pass, compiling isEven would fail because isOdd is not yet
// defined when isEven references it.
//
// Pass 1: collectFunctionDeclarations
//
// Walks the AST to find all named function declarations at the module (global)
// scope and registers them in the symbol table as constants. This ensures their
// names are available for resolution during the second pass.
//
// Only top-level functions are collected. Functions nested inside other
// functions or blocks are compiled in order and cannot be forward-referenced.
//
// Pass 2: compile
//
// Recursively compiles each AST node into bytecode. When the compiler encounters
// a reference to an identifier, it resolves the name via the symbol table. For
// forward-referenced functions, the symbol was already registered in pass 1.
//
// When compiling a named function definition, the compiler checks whether the
// name was already registered (from pass 1) and reuses that symbol slot rather
// than creating a duplicate.
//
// # Symbol Scopes
//
// The compiler tracks three variable scopes:
//
//   - Global: Module-level variables, accessed via LoadGlobal/StoreGlobal
//   - Local: Function-local variables, accessed via LoadFast/StoreFast
//   - Free: Captured closure variables, accessed via LoadFree/StoreFree
//
// The symbol table handles scope resolution and tracks which local variables
// are captured by nested functions (free variables). When a function references
// a variable from an enclosing scope, the compiler emits MakeCell instructions
// to capture the variable into the closure.
package compiler

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/deepnoodle-ai/risor/v2/internal/token"
	"github.com/deepnoodle-ai/risor/v2/pkg/ast"
	"github.com/deepnoodle-ai/risor/v2/pkg/bytecode"
	"github.com/deepnoodle-ai/risor/v2/pkg/errors"
	"github.com/deepnoodle-ai/risor/v2/pkg/op"
)

// SourceLocation is an alias to errors.SourceLocation for convenience.
type SourceLocation = errors.SourceLocation

const (
	// MaxArgs is the maximum number of arguments a function can have.
	MaxArgs = 255

	// Placeholder is a temporary value written during compilation, which is
	// always replaced before compilation is complete.
	Placeholder = uint16(math.MaxUint16)
)

// Compiler is used to compile Risor AST into its corresponding bytecode.
// This implements the ICompiler interface.
type Compiler struct {
	// The entrypoint code we are compiling. This remains fixed throughout
	// the compilation process.
	main *Code

	// The current code we are compiling into. This changes as we enter
	// and leave functions.
	current *Code

	// Set on a compilation error
	failure error

	// Names of globals to be available during compilation
	globalNames []string

	// Increments with each function compiled
	funcIndex int

	// Source filename
	filename string

	// Original source code (for better error messages)
	source string

	// sourceOffset is the byte offset of source within the main code's
	// source. They differ in the REPL, where the main code's source holds
	// every input so far. It is zero when source is empty.
	sourceOffset int

	// rootByteOffsets maps rune offsets in the root source to byte offsets,
	// since AST positions are rune offsets. It has one entry per rune plus a
	// final entry for the end of the source. It is nil when the source is all
	// ASCII, where the two offsets are equal. rootOffsetsOf is the source it
	// was built from, and rootOffsetsSet reports whether it was built.
	rootByteOffsets []int
	rootOffsetsOf   string
	rootOffsetsSet  bool

	// sourceLines is linesOf split into lines. It is cached so that the
	// source is split once per compilation rather than once per emitted
	// instruction. In the REPL the root source grows with each input, which
	// rebuilds it.
	sourceLines []string
	linesOf     string

	// Current AST node being compiled (used for source map tracking)
	currentNode ast.Node
}

// Config holds compiler configuration options.
type Config struct {
	// GlobalNames are the names of global variables available during compilation.
	// These are typically the keys from the environment map passed to the VM.
	GlobalNames []string

	// Filename is the source filename, used for error messages.
	Filename string

	// Source is the original source code, used for better error messages.
	Source string

	// Code is an existing code object to compile into. This is used for
	// REPL-style incremental compilation where state must be preserved.
	// If nil, a new code object is created.
	Code *Code
}

// Compile compiles the given AST node and returns immutable bytecode.
// This is the standard entry point for compiling code that will be executed.
// Pass nil for cfg to use default settings.
func Compile(node ast.Node, cfg *Config) (*bytecode.Code, error) {
	c, err := New(cfg)
	if err != nil {
		return nil, err
	}
	code, err := c.CompileAST(node)
	if err != nil {
		return nil, err
	}
	return code.ToBytecode(), nil
}

// New creates and returns a new Compiler. Pass nil for cfg to use defaults.
func New(cfg *Config) (*Compiler, error) {
	c := &Compiler{}
	if cfg != nil {
		c.globalNames = make([]string, len(cfg.GlobalNames))
		copy(c.globalNames, cfg.GlobalNames) // isolate from caller
		c.filename = cfg.Filename
		c.source = cfg.Source
		c.main = cfg.Code
	}
	// Create a default, empty code object to compile into if the caller didn't
	// supply one. If the caller did supply one, it may be a situation like the
	// REPL where compilation is done incrementally, as new code is entered.
	if c.main == nil {
		c.main = &Code{
			id:      "__main__",
			name:    "__main__",
			symbols: NewSymbolTable(),
		}
	}
	// Insert any supplied names for globals into the symbol table
	sort.Strings(c.globalNames)
	for _, name := range c.globalNames {
		if c.main.symbols.IsDefined(name) {
			continue
		}
		if _, err := c.main.symbols.InsertVariable(name); err != nil {
			return nil, err
		}
	}
	// Store the env keys on the main code for later validation
	c.main.envKeys = make([]string, len(c.globalNames))
	copy(c.main.envKeys, c.globalNames)
	// Start compiling into the main code object
	c.current = c.main
	return c, nil
}

// Code returns the compiled code for the entrypoint.
func (c *Compiler) Code() *Code {
	return c.main
}

// CompileAST compiles the given AST node and returns the mutable Code object.
// This is used for REPL-style incremental compilation where state must be
// preserved across multiple compilations. For normal compilation, use the
// package-level Compile function instead.
func (c *Compiler) CompileAST(node ast.Node) (*Code, error) {
	c.failure = nil

	// Snapshot compiler state so we can rollback if compilation fails.
	// This is critical for REPL-style incremental compilation where the
	// same Code object is reused across multiple CompileAST calls. Without
	// rollback, a failed compilation would leave partial bytecode and symbol
	// table entries that corrupt subsequent compilations.
	codeSnap := c.main.snapshot()
	symSnap := c.main.symbols.snapshot()
	funcIdx := c.funcIndex

	// rollback restores all compiler state to before this CompileAST call.
	rollback := func() {
		c.main.restore(codeSnap)
		c.main.symbols.restore(symSnap)
		c.funcIndex = funcIdx
		c.current = c.main
		c.failure = nil
	}

	// Use original source if available (better error messages with actual code),
	// otherwise fall back to AST string representation.
	nodeSource := c.source
	if nodeSource == "" {
		nodeSource = node.String()
	}

	if c.main.source == "" {
		c.main.source = nodeSource
	} else {
		c.main.source = fmt.Sprintf("%s\n%s", c.main.source, nodeSource)
	}
	c.sourceOffset = 0
	if c.source != "" {
		c.sourceOffset = len(c.main.source) - len(nodeSource)
	}
	if c.filename != "" {
		c.main.filename = c.filename
	}

	// First pass: collect function declarations to allow forward references
	if err := c.collectFunctionDeclarations(node); err != nil {
		rollback()
		return nil, err
	}

	// Second pass: actual compilation
	if err := c.compile(node); err != nil {
		rollback()
		return nil, err
	}
	// Check for failures that happened that aren't propagated up the call
	// stack. Some errors are difficult to propagate without bloating the code.
	if c.failure != nil {
		err := c.failure
		rollback()
		return nil, err
	}
	return c.main, nil
}

// collectFunctionDeclarations walks the AST and collects all function declarations,
// adding them to the symbol table to allow forward references.
//
// Only the program's top-level statements are visited. Functions declared in
// a block are scoped to that block (compileBlock opens a new block scope), so
// hoisting them into the root scope would create a global that is never set.
func (c *Compiler) collectFunctionDeclarations(node ast.Node) error {
	switch node := node.(type) {
	case *ast.Program:
		for _, stmt := range node.Stmts {
			if err := c.collectFunctionDeclarations(stmt); err != nil {
				return err
			}
		}
	case *ast.Func:
		// Only collect named functions at the top level
		if node.Name != nil && c.current.parent == nil {
			functionName := node.Name.Name
			if _, found := c.current.symbols.Get(functionName); found {
				return c.formatError(fmt.Sprintf("function %q redefined", functionName), node.Pos())
			}
			if _, err := c.current.symbols.InsertConstant(functionName); err != nil {
				return err
			}
		}
	}
	return nil
}

// compile the given AST node and all its children.
func (c *Compiler) compile(node ast.Node) error {
	// Track the current node for source location mapping
	c.currentNode = node
	switch node := node.(type) {
	case *ast.Nil:
		if err := c.compileNil(); err != nil {
			return err
		}
	case *ast.Int:
		if err := c.compileInt(node); err != nil {
			return err
		}
	case *ast.Float:
		if err := c.compileFloat(node); err != nil {
			return err
		}
	case *ast.String:
		if err := c.compileString(node); err != nil {
			return err
		}
	case *ast.Bool:
		if err := c.compileBool(node); err != nil {
			return err
		}
	case *ast.If:
		if err := c.compileIf(node); err != nil {
			return err
		}
	case *ast.Infix:
		if err := c.compileInfix(node); err != nil {
			return err
		}
	case *ast.Program:
		if err := c.compileProgram(node); err != nil {
			return err
		}
	case *ast.Block:
		if err := c.compileBlock(node); err != nil {
			return err
		}
	case *ast.Var:
		if err := c.compileVar(node); err != nil {
			return err
		}
	case *ast.Assign:
		if err := c.compileAssign(node); err != nil {
			return err
		}
	case *ast.Ident:
		if err := c.compileIdent(node); err != nil {
			return err
		}
	case *ast.Return:
		if err := c.compileReturn(node); err != nil {
			return err
		}
	case *ast.Call:
		if err := c.compileCall(node); err != nil {
			return err
		}
	case *ast.Func:
		if err := c.compileFunc(node); err != nil {
			return err
		}
	case *ast.List:
		if err := c.compileList(node); err != nil {
			return err
		}
	case *ast.Map:
		if err := c.compileMap(node); err != nil {
			return err
		}
	case *ast.Index:
		if err := c.compileIndex(node); err != nil {
			return err
		}
	case *ast.GetAttr:
		if err := c.compileGetAttr(node); err != nil {
			return err
		}
	case *ast.ObjectCall:
		if err := c.compileObjectCall(node); err != nil {
			return err
		}
	case *ast.Prefix:
		if err := c.compilePrefix(node); err != nil {
			return err
		}
	case *ast.In:
		if err := c.compileIn(node); err != nil {
			return err
		}
	case *ast.NotIn:
		if err := c.compileNotIn(node); err != nil {
			return err
		}
	case *ast.Const:
		if err := c.compileConst(node); err != nil {
			return err
		}
	case *ast.Postfix:
		if err := c.compilePostfix(node); err != nil {
			return err
		}
	case *ast.Pipe:
		if err := c.compilePipe(node); err != nil {
			return err
		}
	case *ast.Slice:
		if err := c.compileSlice(node); err != nil {
			return err
		}
	case *ast.Match:
		if err := c.compileMatch(node); err != nil {
			return err
		}
	case *ast.MultiVar:
		if err := c.compileMultiVar(node); err != nil {
			return err
		}
	case *ast.ObjectDestructure:
		if err := c.compileObjectDestructure(node); err != nil {
			return err
		}
	case *ast.ArrayDestructure:
		if err := c.compileArrayDestructure(node); err != nil {
			return err
		}
	case *ast.SetAttr:
		if err := c.compileSetAttr(node); err != nil {
			return err
		}
	case *ast.Try:
		if err := c.compileTry(node); err != nil {
			return err
		}
	case *ast.Throw:
		if err := c.compileThrow(node); err != nil {
			return err
		}
	case *ast.BadExpr:
		return c.formatError("syntax error in expression", node.Pos())
	case *ast.BadStmt:
		return c.formatError("syntax error in statement", node.Pos())
	default:
		// The parser never produces these in an expression position, but
		// hand-built ASTs can. Report an error rather than panicking.
		if node == nil {
			return c.formatError("unexpected empty node", token.Position{})
		}
		return c.formatError(fmt.Sprintf("unexpected %s", describeNode(node)), node.Pos())
	}
	return nil
}

// describeNode names an AST node that is not valid where it appears.
func describeNode(node ast.Node) string {
	switch node.(type) {
	case *ast.Spread:
		return "spread expression (only valid in list literals, map literals, and call arguments)"
	case *ast.DefaultValue:
		return "default value (only valid in destructuring patterns)"
	default:
		return fmt.Sprintf("node type %T", node)
	}
}

func (c *Compiler) currentPosition() int {
	return len(c.current.instructions)
}

func (c *Compiler) compileNil() error {
	c.emit(op.Nil)
	return nil
}

func (c *Compiler) compileInt(node *ast.Int) error {
	c.emit(op.LoadConst, c.constant(node.Value))
	return nil
}

func (c *Compiler) compileFloat(node *ast.Float) error {
	c.emit(op.LoadConst, c.constant(node.Value))
	return nil
}

func (c *Compiler) compileBool(node *ast.Bool) error {
	if node.Value {
		c.emit(op.True)
	} else {
		c.emit(op.False)
	}
	return nil
}

// isExpr returns true if the given node is an expression node.
func isExpr(node ast.Node) bool {
	_, ok := node.(ast.Expr)
	return ok
}

func (c *Compiler) compileProgram(node *ast.Program) error {
	statements := node.Stmts
	count := len(statements)
	if count == 0 {
		// Guarantee that the program evaluates to a value
		c.emit(op.Nil)
	} else {
		for i, stmt := range statements {
			if err := c.compile(stmt); err != nil {
				return err
			}
			if i < count-1 {
				if isExpr(stmt) {
					c.emit(op.PopTop)
				}
			}
		}
		// Guarantee that the program evaluates to a value
		lastStatement := statements[count-1]
		if !isExpr(lastStatement) {
			c.emit(op.Nil)
		}
	}
	return nil
}

func (c *Compiler) compileBlock(node *ast.Block) error {
	code := c.current
	code.symbols = code.symbols.NewBlock()
	defer func() {
		code.symbols = code.symbols.parent
	}()
	statements := node.Stmts
	count := len(statements)
	if count == 0 {
		// Guarantee that the block evaluates to a value
		c.emit(op.Nil)
	} else {
		for i, stmt := range statements {
			if err := c.compile(stmt); err != nil {
				return err
			}
			if i < count-1 {
				if isExpr(stmt) {
					c.emit(op.PopTop)
				}
			}
		}
		// Guarantee that the block evaluates to a value
		lastStatement := statements[count-1]
		if !isExpr(lastStatement) {
			c.emit(op.Nil)
		}
	}
	return nil
}

func (c *Compiler) compileFunctionBlock(node *ast.Block) error {
	code := c.current
	code.symbols = code.symbols.NewBlock()
	defer func() {
		code.symbols = code.symbols.parent
	}()
	statements := normalizeFunctionBlock(node)
	count := len(statements)
	for i, stmt := range statements {
		if err := c.compile(stmt); err != nil {
			return err
		}
		if i < count-1 {
			if isExpr(stmt) {
				c.emit(op.PopTop)
			}
		}
	}
	return nil
}

func (c *Compiler) compileVar(node *ast.Var) error {
	name := node.Name.Name
	expr := node.Value
	if err := c.compile(expr); err != nil {
		return err
	}
	sym, err := c.current.symbols.InsertVariable(name)
	if err != nil {
		return err
	}
	// Blank identifier "_" returns nil - discard the value
	if sym == nil {
		c.emit(op.PopTop)
		return nil
	}
	if c.current.parent == nil {
		c.emit(op.StoreGlobal, sym.Index())
	} else {
		c.emit(op.StoreFast, sym.Index())
	}
	return nil
}

func (c *Compiler) compileIdent(node *ast.Ident) error {
	name := node.Name
	// The blank identifier cannot be read
	if IsBlankIdentifier(name) {
		return c.formatError("cannot use _ as value", node.Pos())
	}
	resolution, found := c.current.symbols.Resolve(name)
	if !found {
		return c.formatUndefinedVariableError(name, node.Pos())
	}
	c.emitLoad(resolution)
	return nil
}

func (c *Compiler) compileMultiVar(node *ast.MultiVar) error {
	names := node.Names
	expr := node.Value
	if len(names) > math.MaxUint16 {
		return c.formatError("too many variables in multi-variable assignment", node.Pos())
	}
	// Compile the RHS value
	if err := c.compile(expr); err != nil {
		return err
	}
	// Emit the Unpack opcode to unpack the tuple-like object onto the stack
	c.emit(op.Unpack, uint16(len(names)))
	// Iterate through the names in reverse order and declare the variables
	for i := len(names) - 1; i >= 0; i-- {
		name := names[i].Name
		sym, err := c.current.symbols.InsertVariable(name)
		if err != nil {
			return err
		}
		// Blank identifier "_" returns nil - discard the value
		if sym == nil {
			c.emit(op.PopTop)
			continue
		}
		if c.current.parent == nil {
			c.emit(op.StoreGlobal, sym.Index())
		} else {
			c.emit(op.StoreFast, sym.Index())
		}
	}
	return nil
}

func (c *Compiler) compileObjectDestructure(node *ast.ObjectDestructure) error {
	bindings := node.Bindings
	if len(bindings) > math.MaxUint16 {
		return c.formatError("too many bindings in object destructuring", node.Pos())
	}

	// Compile the source object
	if err := c.compile(node.Value); err != nil {
		return err
	}

	// For each binding, load the property and store it in a variable
	for _, binding := range bindings {
		// Duplicate the object on the stack (we need it for each property access)
		c.emit(op.Copy, 0)

		// Load the property - use LoadAttrOrNil if there's a default to avoid errors
		if binding.Default != nil {
			c.emit(op.LoadAttrOrNil, c.addName(binding.Key))
		} else {
			c.emit(op.LoadAttr, c.addName(binding.Key))
		}

		// Handle default value if present
		if binding.Default != nil {
			// Stack has the value at TOS. Check if it's nil.
			c.emit(op.Copy, 0) // Duplicate the value
			jumpPos := c.emit(op.PopJumpForwardIfNotNil, Placeholder)
			// If nil, pop the nil value and use the default
			c.emit(op.PopTop)
			if err := c.compile(binding.Default); err != nil {
				return err
			}
			c.emit(op.Nop)
			delta, err := c.calculateDelta(jumpPos)
			if err != nil {
				return err
			}
			c.changeOperand(jumpPos, delta)
		}

		// Determine the variable name (alias if provided, otherwise key)
		varName := binding.Alias
		if varName == "" {
			varName = binding.Key
		}

		// Insert the variable and store the value
		sym, err := c.current.symbols.InsertVariable(varName)
		if err != nil {
			return err
		}
		// Blank identifier "_" returns nil - discard the value
		if sym == nil {
			c.emit(op.PopTop)
			continue
		}
		if c.current.parent == nil {
			c.emit(op.StoreGlobal, sym.Index())
		} else {
			c.emit(op.StoreFast, sym.Index())
		}
	}

	// Pop the remaining object from the stack
	c.emit(op.PopTop)

	return nil
}

func (c *Compiler) compileArrayDestructure(node *ast.ArrayDestructure) error {
	elements := node.Elements
	if len(elements) > math.MaxUint16 {
		return c.formatError("too many elements in array destructuring", node.Pos())
	}

	// Check if any elements have defaults
	hasDefaults := false
	for _, elem := range elements {
		if elem.Default != nil {
			hasDefaults = true
			break
		}
	}

	// Compile the source array
	if err := c.compile(node.Value); err != nil {
		return err
	}

	// Emit the Unpack opcode to unpack the array onto the stack
	c.emit(op.Unpack, uint16(len(elements)))

	return c.emitArrayDestructureStores(elements, hasDefaults, func(element ast.ArrayDestructureElement) error {
		sym, err := c.current.symbols.InsertVariable(element.Name.Name)
		if err != nil {
			return err
		}
		// Blank identifier "_" returns nil - discard the value
		if sym == nil {
			c.emit(op.PopTop)
			return nil
		}
		if c.current.parent == nil {
			c.emit(op.StoreGlobal, sym.Index())
		} else {
			c.emit(op.StoreFast, sym.Index())
		}
		return nil
	})
}

// emitArrayDestructureStores stores the values that op.Unpack pushed for
// elements. The store callback pops TOS into the element's variable.
//
// Without defaults, values are popped in reverse order, since Unpack leaves
// the last element at TOS. With defaults, elements are handled left to
// right so that defaults run in source order and can refer to earlier
// names: each value is copied to TOS in turn, and the unpacked values are
// popped at the end.
func (c *Compiler) emitArrayDestructureStores(
	elements []ast.ArrayDestructureElement,
	hasDefaults bool,
	store func(ast.ArrayDestructureElement) error,
) error {
	if !hasDefaults {
		for i := len(elements) - 1; i >= 0; i-- {
			if err := store(elements[i]); err != nil {
				return err
			}
		}
		return nil
	}
	n := len(elements)
	for i, element := range elements {
		// Copy element i to TOS. The n unpacked values stay below it.
		c.emit(op.Copy, uint16(n-1-i))
		if element.Default != nil {
			// Stack has the value at TOS. Check if it's nil.
			c.emit(op.Copy, 0) // Duplicate the value
			jumpPos := c.emit(op.PopJumpForwardIfNotNil, Placeholder)
			// If nil, pop the nil value and use the default
			c.emit(op.PopTop)
			if err := c.compile(element.Default); err != nil {
				return err
			}
			c.emit(op.Nop)
			delta, err := c.calculateDelta(jumpPos)
			if err != nil {
				return err
			}
			c.changeOperand(jumpPos, delta)
		}
		if err := store(element); err != nil {
			return err
		}
	}
	for range elements {
		c.emit(op.PopTop)
	}
	return nil
}

// emitDestructurePreamble emits bytecode to destructure a function parameter.
// The parameter at paramIdx contains the value to destructure, and the
// destructured values are stored into local variables.
func (c *Compiler) emitDestructurePreamble(param ast.FuncParam, paramIdx int) error {
	switch p := param.(type) {
	case *ast.ObjectDestructureParam:
		return c.emitObjectDestructurePreamble(p, paramIdx)
	case *ast.ArrayDestructureParam:
		return c.emitArrayDestructurePreamble(p, paramIdx)
	}
	return nil
}

// emitObjectDestructurePreamble emits bytecode to destructure an object parameter.
func (c *Compiler) emitObjectDestructurePreamble(param *ast.ObjectDestructureParam, paramIdx int) error {
	// Load the parameter value onto the stack
	c.emit(op.LoadFast, uint16(paramIdx))

	// For each binding, load the property and store it in a variable
	for _, binding := range param.Bindings {
		// Duplicate the object on the stack (we need it for each property access)
		c.emit(op.Copy, 0)

		// Load the property - use LoadAttrOrNil if there's a default to avoid errors
		if binding.Default != nil {
			c.emit(op.LoadAttrOrNil, c.addName(binding.Key))
		} else {
			c.emit(op.LoadAttr, c.addName(binding.Key))
		}

		// Handle default value if present
		if binding.Default != nil {
			// Stack has the value at TOS. Check if it's nil.
			c.emit(op.Copy, 0) // Duplicate the value
			jumpPos := c.emit(op.PopJumpForwardIfNotNil, Placeholder)
			// If nil, pop the nil value and use the default
			c.emit(op.PopTop)
			if err := c.compile(binding.Default); err != nil {
				return err
			}
			c.emit(op.Nop)
			delta, err := c.calculateDelta(jumpPos)
			if err != nil {
				return err
			}
			c.changeOperand(jumpPos, delta)
		}

		// Determine the variable name (alias if provided, otherwise key)
		varName := binding.Alias
		if varName == "" {
			varName = binding.Key
		}

		// Blank identifier "_" - discard the value
		if IsBlankIdentifier(varName) {
			c.emit(op.PopTop)
			continue
		}

		// Insert the variable only now, after its default was compiled, so a
		// default cannot see this name or a later one (as with let).
		sym, err := c.current.symbols.InsertVariable(varName)
		if err != nil {
			return err
		}
		c.emit(op.StoreFast, sym.Index())
	}

	// Pop the remaining object from the stack
	c.emit(op.PopTop)
	return nil
}

// emitArrayDestructurePreamble emits bytecode to destructure an array parameter.
func (c *Compiler) emitArrayDestructurePreamble(param *ast.ArrayDestructureParam, paramIdx int) error {
	elements := param.Elements

	// Load the parameter value onto the stack
	c.emit(op.LoadFast, uint16(paramIdx))

	// Emit the Unpack opcode to unpack the array onto the stack
	c.emit(op.Unpack, uint16(len(elements)))

	hasDefaults := false
	for _, elem := range elements {
		if elem.Default != nil {
			hasDefaults = true
			break
		}
	}
	return c.emitArrayDestructureStores(elements, hasDefaults, func(element ast.ArrayDestructureElement) error {
		varName := element.Name.Name
		// Blank identifier "_" - discard the value
		if IsBlankIdentifier(varName) {
			c.emit(op.PopTop)
			return nil
		}
		// Insert the variable only now, after its default was compiled, so a
		// default cannot see this name or a later one (as with let).
		sym, err := c.current.symbols.InsertVariable(varName)
		if err != nil {
			return err
		}
		c.emit(op.StoreFast, sym.Index())
		return nil
	})
}

func (c *Compiler) compileMatch(node *ast.Match) error {
	// Compile the subject expression (remains on stack for comparisons)
	if err := c.compile(node.Subject); err != nil {
		return err
	}

	arms := node.Arms

	// Emit pattern match (and optional guard) + jump positions for each arm
	var armJumpPositions []int

	for _, arm := range arms {
		// Duplicate the subject value for pattern matching
		c.emit(op.Copy, 0)

		// Compile the pattern match (leaves true/false on stack)
		if err := c.compilePatternMatch(arm.Pattern); err != nil {
			return err
		}

		if arm.Guard != nil {
			// Pattern fails → skip guard, continue to next arm test
			guardSkipPos := c.emit(op.PopJumpForwardIfFalse, Placeholder)

			// Pattern matched → evaluate guard expression
			if err := c.compile(arm.Guard); err != nil {
				return err
			}

			// Guard passes → jump to arm body
			bodyJumpPos := c.emit(op.PopJumpForwardIfTrue, Placeholder)
			armJumpPositions = append(armJumpPositions, bodyJumpPos)

			// Guard failed → patch guardSkip to land here (next arm test)
			delta, err := c.calculateDelta(guardSkipPos)
			if err != nil {
				return err
			}
			c.changeOperand(guardSkipPos, delta)
		} else {
			// No guard: jump to arm body if pattern matches
			jumpPos := c.emit(op.PopJumpForwardIfTrue, Placeholder)
			armJumpPositions = append(armJumpPositions, jumpPos)
		}
	}

	// Jump to default arm if no patterns matched
	jumpDefaultPos := c.emit(op.JumpForward, Placeholder)

	// Compile each arm's body
	var endBlockPositions []int
	for i, arm := range arms {
		// Patch the jump to this arm's body
		delta, err := c.calculateDelta(armJumpPositions[i])
		if err != nil {
			return err
		}
		c.changeOperand(armJumpPositions[i], delta)

		// Compile the arm's result expression
		if err := c.compile(arm.Result); err != nil {
			return err
		}

		// Jump to end after arm body
		endBlockPositions = append(endBlockPositions, c.emit(op.JumpForward, Placeholder))
	}

	// Patch jump to default arm
	delta, err := c.calculateDelta(jumpDefaultPos)
	if err != nil {
		return err
	}
	c.changeOperand(jumpDefaultPos, delta)

	// Compile the default arm's body
	if err := c.compile(node.Default.Result); err != nil {
		return err
	}

	// Patch all end jumps to point here
	for _, pos := range endBlockPositions {
		delta, err := c.calculateDelta(pos)
		if err != nil {
			return err
		}
		c.changeOperand(pos, delta)
	}

	// Swap result with subject and pop subject
	c.emit(op.Swap, 1)
	c.emit(op.PopTop)

	return nil
}

// compilePatternMatch compiles code to match the TOS value against a pattern.
// Leaves true or false on the stack.
func (c *Compiler) compilePatternMatch(pattern ast.Pattern) error {
	switch p := pattern.(type) {
	case *ast.LiteralPattern:
		// Compile the literal value
		if err := c.compile(p.Value); err != nil {
			return err
		}
		// Compare TOS (literal) with TOS-1 (subject copy)
		c.emit(op.CompareOp, uint16(op.Equal))

	case *ast.WildcardPattern:
		// Wildcard matches everything - pop the copy and push true
		c.emit(op.PopTop)
		c.emit(op.True)

	default:
		return c.formatError("unsupported pattern type", pattern.Pos())
	}
	return nil
}

func (c *Compiler) compileSlice(node *ast.Slice) error {
	if err := c.compile(node.X); err != nil {
		return err
	}
	high := node.High
	if high == nil {
		c.emit(op.Copy, 0)
		c.emit(op.Length)
	} else {
		if err := c.compile(high); err != nil {
			return err
		}
	}
	low := node.Low
	if low == nil {
		c.emit(op.LoadConst, c.constant(int64(0)))
	} else {
		if err := c.compile(low); err != nil {
			return err
		}
	}
	c.emit(op.Slice)
	return nil
}

func (c *Compiler) compileString(node *ast.String) error {
	// Is the string a template or a simple string?
	tmpl := node.Template

	// Simple strings are just emitted as a constant
	if tmpl == nil {
		c.emit(op.LoadConst, c.constant(node.Value))
		return nil
	}

	fragments := tmpl.Fragments()
	if len(fragments) > math.MaxUint16 {
		return c.formatError("string template exceeded max fragment size", node.Pos())
	}

	var expressionIndex int
	expressions := node.Exprs

	// Emit code that pushes each fragment of the string onto the stack
	for _, f := range fragments {
		switch f.IsVariable() {
		case true:
			expr := expressions[expressionIndex]
			expressionIndex++
			// Nil expression should be treated as empty string
			if expr == nil {
				c.emit(op.LoadConst, c.constant(""))
				continue
			}
			if err := c.compile(expr); err != nil {
				return err
			}
		case false:
			// Push the fragment as a constant as TOS
			c.emit(op.LoadConst, c.constant(f.Value()))
		}
	}
	// Emit a BuildString to concatenate all the fragments
	c.emit(op.BuildString, uint16(len(fragments)))
	return nil
}

func (c *Compiler) compilePipe(node *ast.Pipe) error {
	if c.current.pipeActive {
		return c.formatError("invalid nested pipe", node.Pos())
	}
	exprs := node.Exprs
	if len(exprs) < 2 {
		return c.formatError("the pipe operator requires at least two expressions", node.Pos())
	}
	// Compile the first expression (filling TOS with the initial pipe value)
	if err := c.compile(exprs[0]); err != nil {
		return err
	}
	// Set the pipe active flag for the remainder of the pipe
	c.current.pipeActive = true
	defer func() {
		c.current.pipeActive = false
	}()
	// Iterate over the remaining expressions. Each should eval to a function.
	// TODO: may need to compile to a partial as well.
	for i := 1; i < len(exprs); i++ {
		// An optional method call (obj?.method()) whose object is nil
		// short-circuits the stage: its result is nil, and nothing is called.
		if call, ok := exprs[i].(*ast.ObjectCall); ok && call.Optional {
			if err := c.compileOptionalPipeStage(call); err != nil {
				return err
			}
			continue
		}
		// Compile the current expression, pushing a function as TOS
		if err := c.compile(exprs[i]); err != nil {
			return err
		}
		// Swap the function (TOS) with the argument below it on the stack
		// and then call the function with one argument
		c.emit(op.Swap, 1)
		c.emit(op.Call, 1)
	}
	return nil
}

// compileOptionalPipeStage compiles a pipe stage of the form obj?.method().
// When obj is nil, the stack holds [arg, nil] instead of [arg, partial], so
// the argument is dropped and nil becomes the stage result.
func (c *Compiler) compileOptionalPipeStage(call *ast.ObjectCall) error {
	nilJumpPos, err := c.emitObjectCall(call)
	if err != nil {
		return err
	}
	c.emit(op.Swap, 1)
	c.emit(op.Call, 1)
	endJumpPos := c.emit(op.JumpForward, Placeholder)
	delta, err := c.calculateDelta(nilJumpPos)
	if err != nil {
		return err
	}
	c.changeOperand(nilJumpPos, delta)
	c.emit(op.Swap, 1)
	c.emit(op.PopTop)
	delta, err = c.calculateDelta(endJumpPos)
	if err != nil {
		return err
	}
	c.changeOperand(endJumpPos, delta)
	return nil
}

func (c *Compiler) compilePostfix(node *ast.Postfix) error {
	// Determine the increment/decrement amount
	var amount int64
	switch node.Op {
	case "++":
		amount = 1
	case "--":
		amount = -1
	default:
		return c.formatError(fmt.Sprintf("unknown postfix operator %q", node.Op), node.Pos())
	}

	switch x := node.X.(type) {
	case *ast.Ident:
		// Simple variable: x++
		name := x.Name
		resolution, found := c.current.symbols.Resolve(name)
		if !found {
			return c.formatUndefinedVariableError(name, node.Pos())
		}
		// Push the named variable onto the stack
		c.emitLoad(resolution)
		// Push the increment amount
		c.emit(op.LoadConst, c.constant(amount))
		// Add
		c.emit(op.BinaryOp, uint16(op.Add))
		// Store back
		c.emitStore(resolution)

	case *ast.Index:
		// Index expression: arr[i]++
		// 1. Load the current value, keeping the container and index
		if err := c.emitLoadIndexKeep(x); err != nil {
			return err
		}
		// 2. Add the increment amount
		c.emit(op.LoadConst, c.constant(amount))
		c.emit(op.BinaryOp, uint16(op.Add))
		// 3. Store back
		c.emitStoreIndexKept()

	case *ast.GetAttr:
		// Attribute expression: obj.x++
		idx := c.addName(x.Attr.Name)
		// 1. Load the current attribute value, keeping the object
		if err := c.compile(x.X); err != nil {
			return err
		}
		c.emit(op.Copy, 0)
		c.emit(op.LoadAttr, idx)
		// 2. Add the increment amount
		c.emit(op.LoadConst, c.constant(amount))
		c.emit(op.BinaryOp, uint16(op.Add))
		// 3. Store back: StoreAttr expects [value, obj]
		c.emit(op.Swap, 1)
		c.emit(op.StoreAttr, idx)

	default:
		return c.formatError("cannot apply postfix operator to this expression", node.Pos())
	}
	return nil
}

func (c *Compiler) compileConst(node *ast.Const) error {
	name := node.Name.Name
	expr := node.Value
	if err := c.compile(expr); err != nil {
		return err
	}
	sym, err := c.current.symbols.InsertConstant(name)
	if err != nil {
		return err
	}
	// Blank identifier "_" returns nil - discard the value
	if sym == nil {
		c.emit(op.PopTop)
		return nil
	}
	if c.current.parent == nil {
		c.emit(op.StoreGlobal, sym.Index())
	} else {
		c.emit(op.StoreFast, sym.Index())
	}
	return nil
}

func (c *Compiler) compileIn(node *ast.In) error {
	if err := c.compile(node.Y); err != nil {
		return err
	}
	if err := c.compile(node.X); err != nil {
		return err
	}
	c.emit(op.ContainsOp, 0)
	return nil
}

func (c *Compiler) compileNotIn(node *ast.NotIn) error {
	if err := c.compile(node.Y); err != nil {
		return err
	}
	if err := c.compile(node.X); err != nil {
		return err
	}
	c.emit(op.ContainsOp, 0)
	c.emit(op.UnaryNot)
	return nil
}

func (c *Compiler) compilePrefix(node *ast.Prefix) error {
	if err := c.compile(node.X); err != nil {
		return err
	}
	switch node.Op {
	case "!":
		c.emit(op.UnaryNot)
	case "-":
		c.emit(op.UnaryNegative)
	}
	return nil
}

func (c *Compiler) compileCall(node *ast.Call) error {
	if err := c.compile(node.Fun); err != nil {
		return err
	}
	return c.compileCallArgs(node)
}

// compileCallArgs compiles the arguments of a call and emits the call
// instruction. The callee must already be on the stack.
func (c *Compiler) compileCallArgs(node *ast.Call) error {
	args := node.Args
	argc := len(args)
	if argc > MaxArgs {
		return c.formatError(fmt.Sprintf("max args limit of %d exceeded (got %d)", MaxArgs, argc), node.Pos())
	}

	// Check if any arguments are spread expressions
	hasSpread := false
	for _, arg := range args {
		if _, ok := arg.(*ast.Spread); ok {
			hasSpread = true
			break
		}
	}

	if !hasSpread {
		// Fast path: no spread, use regular Call
		for _, arg := range args {
			if err := c.compile(arg); err != nil {
				return err
			}
		}
		// An error from the call points at the call, not the last argument
		c.currentNode = node
		if c.current.pipeActive {
			c.emit(op.Partial, uint16(argc))
		} else {
			c.emit(op.Call, uint16(argc))
		}
		return nil
	}

	// Slow path: has spread, build args list then use CallSpread
	c.emit(op.BuildList, 0) // Start with empty list
	for _, arg := range args {
		if spread, ok := arg.(*ast.Spread); ok {
			// Spread: extend the args list with the iterable
			if err := c.compile(spread.X); err != nil {
				return err
			}
			c.emit(op.ListExtend)
		} else {
			// Normal arg: append to the args list
			if err := c.compile(arg); err != nil {
				return err
			}
			c.emit(op.ListAppend)
		}
	}
	if c.current.pipeActive {
		// For pipe, we can't easily support spread (would need PartialSpread)
		return c.formatError("spread arguments not supported in pipe expressions", node.Pos())
	}
	c.currentNode = node
	c.emit(op.CallSpread)
	return nil
}

func (c *Compiler) compileObjectCall(node *ast.ObjectCall) error {
	nilJumpPos, err := c.emitObjectCall(node)
	if err != nil {
		return err
	}
	if node.Optional {
		c.emit(op.Nop)
		delta, err := c.calculateDelta(nilJumpPos)
		if err != nil {
			return err
		}
		c.changeOperand(nilJumpPos, delta)
	}
	return nil
}

// emitObjectCall emits the method call in node. For an optional call (?.)
// it returns the position of the jump taken when the object is nil, which
// the caller must patch. On that path the nil object is left on the stack.
func (c *Compiler) emitObjectCall(node *ast.ObjectCall) (int, error) {
	if err := c.compile(node.X); err != nil {
		return 0, err
	}
	// Handle optional chaining (?.)
	var nilJumpPos int
	if node.Optional {
		c.emit(op.Copy, 0)
		nilJumpPos = c.emit(op.PopJumpForwardIfNil, Placeholder)
	}
	method := node.Call
	name := method.Fun.String()
	// Restore currentNode so LoadAttr gets the method name position
	c.currentNode = method.Fun
	c.emit(op.LoadAttr, c.addName(name))
	if err := c.compileCallArgs(method); err != nil {
		return 0, err
	}
	return nilJumpPos, nil
}

func (c *Compiler) compileGetAttr(node *ast.GetAttr) error {
	if err := c.compile(node.X); err != nil {
		return err
	}
	// Handle optional chaining (?.)
	var jumpPos int
	if node.Optional {
		c.emit(op.Copy, 0)
		jumpPos = c.emit(op.PopJumpForwardIfNil, Placeholder)
	}
	// Restore currentNode so LoadAttr gets the attribute name position
	c.currentNode = node.Attr
	idx := c.addName(node.Attr.Name)
	if node.Optional {
		c.emit(op.LoadAttrOrNil, idx)
	} else {
		c.emit(op.LoadAttr, idx)
	}
	if node.Optional {
		c.emit(op.Nop)
		delta, _ := c.calculateDelta(jumpPos)
		c.changeOperand(jumpPos, delta)
	}
	return nil
}

func (c *Compiler) compileIndex(node *ast.Index) error {
	if err := c.compile(node.X); err != nil {
		return err
	}
	if err := c.compile(node.Index); err != nil {
		return err
	}
	c.emit(op.BinarySubscr)
	return nil
}

func (c *Compiler) compileList(node *ast.List) error {
	items := node.Items
	count := len(items)
	if count > math.MaxUint16 {
		return c.formatError("list literal exceeds max size", node.Pos())
	}

	// Check if any items are spread expressions
	hasSpread := false
	for _, expr := range items {
		if _, ok := expr.(*ast.Spread); ok {
			hasSpread = true
			break
		}
	}

	if !hasSpread {
		// Fast path: no spread, use simple BuildList
		for _, expr := range items {
			if err := c.compile(expr); err != nil {
				return err
			}
		}
		c.emit(op.BuildList, uint16(count))
		return nil
	}

	// Slow path: has spread, build incrementally
	// Start with an empty list
	c.emit(op.BuildList, 0)

	for _, expr := range items {
		if spread, ok := expr.(*ast.Spread); ok {
			// Spread: extend the list with the iterable
			if err := c.compile(spread.X); err != nil {
				return err
			}
			c.emit(op.ListExtend)
		} else {
			// Normal item: append to the list
			if err := c.compile(expr); err != nil {
				return err
			}
			c.emit(op.ListAppend)
		}
	}
	return nil
}

func (c *Compiler) compileMap(node *ast.Map) error {
	items := node.Items

	// Check if any items are spread expressions (key is nil)
	hasSpread := node.HasSpread()

	if !hasSpread {
		// Fast path: no spread, use simple BuildMap
		for _, item := range items {
			switch k := item.Key.(type) {
			case *ast.String:
				if err := c.compile(k); err != nil {
					return err
				}
			case *ast.Ident:
				c.emit(op.LoadConst, c.constant(k.String()))
			default:
				return c.formatError(fmt.Sprintf("invalid map key type: %T", item.Key), item.Key.Pos())
			}
			if err := c.compile(item.Value); err != nil {
				return err
			}
		}
		c.emit(op.BuildMap, uint16(len(items)))
		return nil
	}

	// Slow path: has spread, build incrementally
	c.emit(op.BuildMap, 0) // Start with empty map

	for _, item := range items {
		if item.Key == nil {
			// Spread: merge the map with the existing one
			// The Value is a Spread node, we need to compile its inner value
			spread, ok := item.Value.(*ast.Spread)
			if !ok {
				return c.formatError("expected spread expression in map", item.Value.Pos())
			}
			if err := c.compile(spread.X); err != nil {
				return err
			}
			c.emit(op.MapMerge)
		} else {
			// Normal key-value: set in the map
			switch k := item.Key.(type) {
			case *ast.String:
				if err := c.compile(k); err != nil {
					return err
				}
			case *ast.Ident:
				c.emit(op.LoadConst, c.constant(k.String()))
			default:
				return c.formatError(fmt.Sprintf("invalid map key type: %T", item.Key), item.Key.Pos())
			}
			if err := c.compile(item.Value); err != nil {
				return err
			}
			c.emit(op.MapSet)
		}
	}
	return nil
}

func (c *Compiler) compileFunc(node *ast.Func) error {
	// Python cell variables:
	// https://stackoverflow.com/questions/23757143/what-is-a-cell-in-the-context-of-an-interpreter-or-compiler

	if len(node.Params) > MaxArgs {
		return c.formatError(fmt.Sprintf("function exceeded parameter limit of %d", MaxArgs), node.Pos())
	}

	// The function has an optional name. If it is named, the name will be
	// stored in the function's own symbol table to support recursive calls.
	var functionName string
	if ident := node.Name; ident != nil {
		functionName = ident.Name
	}

	// This new code object will store the compiled code for this function.
	// Extract source from original if available for better error messages.
	c.funcIndex++
	functionID := fmt.Sprintf("%d", c.funcIndex)
	bodySource, sourceStart, sourceEnd := c.extractFunctionBodySource(node)
	code := c.current.newChild(functionName, bodySource, functionID)
	code.sourceStart, code.sourceEnd = sourceStart, sourceEnd

	// Setting current here means subsequent calls to compile will add to this
	// code object instead of the parent.
	c.current = code

	// Process parameters - generate synthetic names for destructured params
	// and track which params need destructuring preamble
	type destructureInfo struct {
		param ast.FuncParam
		index int // original parameter index
	}
	paramsIdx := map[string]int{}
	params := make([]string, len(node.Params))
	destructureParams := make([]destructureInfo, 0) // params that need destructuring preamble
	for i, p := range node.Params {
		switch param := p.(type) {
		case *ast.Ident:
			params[i] = param.Name
			paramsIdx[param.Name] = i
		case *ast.ObjectDestructureParam:
			// Generate synthetic name for the positional parameter
			syntheticName := fmt.Sprintf("__destructure_%d", i)
			params[i] = syntheticName
			paramsIdx[syntheticName] = i
			destructureParams = append(destructureParams, destructureInfo{param: p, index: i})
		case *ast.ArrayDestructureParam:
			// Generate synthetic name for the positional parameter
			syntheticName := fmt.Sprintf("__destructure_%d", i)
			params[i] = syntheticName
			paramsIdx[syntheticName] = i
			destructureParams = append(destructureParams, destructureInfo{param: p, index: i})
		default:
			return c.formatError(fmt.Sprintf("unexpected parameter type: %T", p), node.Pos())
		}
	}

	// Build an array of default values for parameters, supporting only
	// the basic types of int, string, bool, float, and nil.
	defaults := make([]any, len(params))
	defaultsSet := map[int]bool{}
	for name, expr := range node.Defaults {
		var value any
		switch expr := expr.(type) {
		case *ast.Int:
			value = expr.Value
		case *ast.String:
			value = expr.Value
		case *ast.Bool:
			value = expr.Value
		case *ast.Float:
			value = expr.Value
		case *ast.Nil:
			value = nil
		default:
			return c.formatError(fmt.Sprintf("unsupported default value type: %T", expr), expr.Pos())
		}
		index := paramsIdx[name]
		defaults[index] = value
		defaultsSet[index] = true
	}

	// Confirm only trailing parameters have defaults
	if len(node.Defaults) > 0 {
		hasDefaults := false
		for i := 0; i < len(params); i++ {
			if defaultsSet[i] {
				hasDefaults = true
			} else if hasDefaults {
				msg := "invalid argument defaults for"
				if functionName != "" {
					msg = fmt.Sprintf("%s function %q", msg, functionName)
				} else {
					msg = fmt.Sprintf("%s anonymous function", msg)
				}
				return c.formatError(msg, node.Pos())
			}
		}
	}

	// Add all parameter names to the symbol table (including synthetic ones)
	// For blank identifier "_" parameters, we claim a slot but don't add to lookup
	for _, paramName := range params {
		if IsBlankIdentifier(paramName) {
			// Claim a slot for the argument but don't add to name lookup
			if _, err := code.symbols.ClaimSlot(); err != nil {
				return err
			}
			continue
		}
		if _, err := code.symbols.InsertVariable(paramName); err != nil {
			return err
		}
	}

	// Add rest parameter to symbol table if present.
	// IMPORTANT: This must come before destructured variable names because
	// the VM places the rest param value at index `paramsCount`, right after
	// the regular parameters.
	var restParamName string
	if restParam := node.RestParam; restParam != nil {
		restParamName = restParam.Name
		if IsBlankIdentifier(restParamName) {
			// Claim a slot for the rest args but don't add to name lookup
			if _, err := code.symbols.ClaimSlot(); err != nil {
				return err
			}
		} else {
			if _, err := code.symbols.InsertVariable(restParamName); err != nil {
				return err
			}
		}
	}

	// Add the function's own name to its symbol table. This supports recursive
	// calls to the function. IMPORTANT: This must come right after the rest
	// param because the VM places the function object at the next index.
	if code.isNamed {
		if _, err := code.symbols.InsertConstant(functionName); err != nil {
			return err
		}
	}

	// Emit destructuring preamble for any destructured parameters
	// This runs at the start of the function to extract values into local vars.
	// The preamble adds each destructured name to the symbol table after
	// compiling its default, so the names come after the function's own name.
	for _, di := range destructureParams {
		if err := c.emitDestructurePreamble(di.param, di.index); err != nil {
			return err
		}
	}

	// Compile the function body
	if err := c.compileFunctionBlock(node.Body); err != nil {
		return err
	}

	// We're done compiling the function, so switch back to compiling the parent
	c.current = c.current.parent

	// Create the function that contains the compiled code
	fn := NewFunction(FunctionOpts{
		ID:         functionID,
		Name:       functionName,
		Parameters: params,
		Defaults:   defaults,
		RestParam:  restParamName,
		Code:       code,
	})

	// Emit the code to load the function object onto the stack. If there are
	// free variables, we use LoadClosure, otherwise we use LoadConst.
	//
	// Each free variable is captured from the enclosing function, which is
	// the active frame when the closure is created. The variable is either a
	// local of the enclosing function or, when it is defined further out, one
	// of its free variables. Resolving it here makes it a free variable of the
	// enclosing function if it wasn't one already, so the cell passes through
	// every function level between the definition and this closure.
	freeCount := code.symbols.FreeCount()
	if freeCount > 0 {
		for i := uint16(0); i < freeCount; i++ {
			name := code.symbols.Free(i).symbol.Name()
			resolution, found := c.current.symbols.Resolve(name)
			if !found {
				return c.formatError(fmt.Sprintf("undefined variable %q", name), node.Pos())
			}
			switch resolution.scope {
			case Local:
				c.emit(op.MakeCell, resolution.symbol.Index(), 0)
			case Free:
				c.emit(op.LoadCell, uint16(resolution.freeIndex))
			default:
				return c.formatError(fmt.Sprintf("invalid capture of global variable %q", name), node.Pos())
			}
		}
		c.emit(op.LoadClosure, c.constant(fn), freeCount)
	} else {
		c.emit(op.LoadConst, c.constant(fn))
	}

	// If the function was named, we store it as a named variable in the current
	// code. Otherwise, we just leave it on the stack.
	if code.isNamed {
		// Check if the function name already exists in the symbol table
		// (it would have been added in the first pass for forward references)
		funcSymbol, found := c.current.symbols.Get(functionName)
		if !found {
			var err error
			funcSymbol, err = c.current.symbols.InsertConstant(functionName)
			if err != nil {
				return err
			}
		}
		// Duplicate function on the stack, so that we ensure the function
		// evaluates to a value even when it's named.
		c.emit(op.Copy, 0)
		if c.current.parent == nil {
			c.emit(op.StoreGlobal, funcSymbol.Index())
		} else {
			c.emit(op.StoreFast, funcSymbol.Index())
		}
	}
	return nil
}

func (c *Compiler) compileReturn(node *ast.Return) error {
	if c.current.IsRoot() {
		return c.formatError("invalid return statement outside of a function", node.Pos())
	}
	value := node.Value
	if value == nil {
		c.emit(op.Nil)
	} else {
		if err := c.compile(value); err != nil {
			return err
		}
	}
	c.emit(op.ReturnValue)
	return nil
}

func (c *Compiler) compileSetItem(node *ast.Assign) error {
	index := node.Index

	// Handle compound operators (*=, +=, etc.)
	if node.Op != "=" {
		// 1. Load the current value: test[0], keeping the container and index
		if err := c.emitLoadIndexKeep(index); err != nil {
			return err
		}

		// 2. Load the RHS value
		if err := c.compile(node.Value); err != nil {
			return err
		}

		// 3. Apply the compound operation
		switch node.Op {
		case "+=":
			c.emit(op.BinaryOp, uint16(op.Add))
		case "-=":
			c.emit(op.BinaryOp, uint16(op.Subtract))
		case "*=":
			c.emit(op.BinaryOp, uint16(op.Multiply))
		case "/=":
			c.emit(op.BinaryOp, uint16(op.Divide))
		default:
			return c.formatError(fmt.Sprintf("unsupported compound assignment operator: %s", node.Op), node.Pos())
		}

		// 4. Store the result back
		c.emitStoreIndexKept()
		return nil
	}

	// Simple assignment
	if err := c.compile(node.Value); err != nil {
		return err
	}
	if err := c.compile(index.X); err != nil {
		return err
	}
	if err := c.compile(index.Index); err != nil {
		return err
	}
	c.emit(op.StoreSubscr)
	return nil
}

// emitLoadIndexKeep evaluates the container and index of node once and
// loads the item, leaving [container, index, item] on the stack. This lets
// compound assignment and postfix operators read and write the same item
// without evaluating the container or index expressions twice.
func (c *Compiler) emitLoadIndexKeep(node *ast.Index) error {
	if err := c.compile(node.X); err != nil {
		return err
	}
	if err := c.compile(node.Index); err != nil {
		return err
	}
	c.emit(op.Copy, 1)
	c.emit(op.Copy, 1)
	c.emit(op.BinarySubscr)
	return nil
}

// emitStoreIndexKept stores TOS into the container and index left below it
// by emitLoadIndexKeep. StoreSubscr expects [value, container, index], so
// [container, index, value] is rotated into that order first.
func (c *Compiler) emitStoreIndexKept() {
	c.emit(op.Swap, 2) // [value, index, container]
	c.emit(op.Swap, 1) // [value, container, index]
	c.emit(op.StoreSubscr)
}

func (c *Compiler) compileAssign(node *ast.Assign) error {
	if node.Index != nil {
		return c.compileSetItem(node)
	}
	name := node.Name.Name
	// The blank identifier discards the value
	if IsBlankIdentifier(name) {
		// Compound assignment to _ doesn't make sense (can't read _)
		if node.Op != "=" {
			return c.formatError("cannot use _ in compound assignment", node.Pos())
		}
		if err := c.compile(node.Value); err != nil {
			return err
		}
		c.emit(op.PopTop)
		return nil
	}
	resolution, found := c.current.symbols.Resolve(name)
	if !found {
		return c.formatUndefinedVariableError(name, node.Pos())
	}
	if resolution.symbol.IsConstant() {
		return c.formatError(fmt.Sprintf("cannot assign to constant %q", name), node.Pos())
	}
	if node.Op == "=" {
		if err := c.compile(node.Value); err != nil {
			return err
		}
		c.emitStore(resolution)
		return nil
	}
	// Push LHS as TOS
	c.emitLoad(resolution)
	// Push RHS as TOS
	if err := c.compile(node.Value); err != nil {
		return err
	}
	// Result becomes TOS
	switch node.Op {
	case "+=":
		c.emit(op.BinaryOp, uint16(op.Add))
	case "-=":
		c.emit(op.BinaryOp, uint16(op.Subtract))
	case "*=":
		c.emit(op.BinaryOp, uint16(op.Multiply))
	case "/=":
		c.emit(op.BinaryOp, uint16(op.Divide))
	}
	// Store TOS in LHS
	c.emitStore(resolution)
	return nil
}

func (c *Compiler) compileSetAttr(node *ast.SetAttr) error {
	idx := c.addName(node.Attr.Name)

	if node.Op == "=" {
		// Simple assignment: compile value, compile object, store attr
		if err := c.compile(node.Value); err != nil {
			return err
		}
		if err := c.compile(node.X); err != nil {
			return err
		}
		c.emit(op.StoreAttr, idx)
		return nil
	}

	// Compound assignment: load current value, apply operation, store result.
	// The object is evaluated once and kept on the stack for the store.
	if err := c.compile(node.X); err != nil {
		return err
	}
	c.emit(op.Copy, 0)
	c.emit(op.LoadAttr, idx)

	// Compile the RHS value
	if err := c.compile(node.Value); err != nil {
		return err
	}

	// Apply the binary operation
	switch node.Op {
	case "+=":
		c.emit(op.BinaryOp, uint16(op.Add))
	case "-=":
		c.emit(op.BinaryOp, uint16(op.Subtract))
	case "*=":
		c.emit(op.BinaryOp, uint16(op.Multiply))
	case "/=":
		c.emit(op.BinaryOp, uint16(op.Divide))
	}

	// StoreAttr expects [value, obj]
	c.emit(op.Swap, 1)
	c.emit(op.StoreAttr, idx)
	return nil
}

func (c *Compiler) compileIf(node *ast.If) error {
	if err := c.compile(node.Cond); err != nil {
		return err
	}
	jumpIfFalsePos := c.emit(op.PopJumpForwardIfFalse, Placeholder)
	if err := c.compile(node.Consequence); err != nil {
		return err
	}
	alternative := node.Alternative
	if alternative != nil {
		// Jump forward to skip the alternative by default
		jumpForwardPos := c.emit(op.JumpForward, Placeholder)
		// Update PopJumpForwardIfFalse to point to this alternative,
		// so that the alternative is executed if the condition is false
		delta, err := c.calculateDelta(jumpIfFalsePos)
		if err != nil {
			return err
		}
		c.changeOperand(jumpIfFalsePos, delta)
		if err := c.compile(alternative); err != nil {
			return err
		}
		delta, err = c.calculateDelta(jumpForwardPos)
		if err != nil {
			return err
		}
		c.changeOperand(jumpForwardPos, delta)
	} else {
		// Jump forward to skip the alternative by default
		jumpForwardPos := c.emit(op.JumpForward, Placeholder)
		// Update PopJumpForwardIfFalse to point to this alternative,
		// so that the alternative is executed if the condition is false
		delta, err := c.calculateDelta(jumpIfFalsePos)
		if err != nil {
			return err
		}
		c.changeOperand(jumpIfFalsePos, delta)
		// This allows ifs to be used as expressions. If the if check fails and
		// there is no alternative, the result of the if expression is nil.
		c.emit(op.Nil)
		delta, err = c.calculateDelta(jumpForwardPos)
		if err != nil {
			return err
		}
		c.changeOperand(jumpForwardPos, delta)
	}
	return nil
}

func (c *Compiler) calculateDelta(pos int) (uint16, error) {
	instrCount := len(c.current.instructions)
	delta := instrCount - pos
	// Use >= because Placeholder (MaxUint16) is reserved for unpatched jumps
	if delta >= int(Placeholder) {
		return 0, fmt.Errorf("compile error: jump destination is too far away")
	}
	return uint16(delta), nil
}

func (c *Compiler) changeOperand(instructionIndex int, operand uint16) {
	c.current.instructions[instructionIndex+1] = op.Code(operand)
}

func (c *Compiler) compileInfix(node *ast.Infix) error {
	operator := node.Op
	// Short-circuit operators
	if operator == "&&" {
		return c.compileAnd(node)
	} else if operator == "||" {
		return c.compileOr(node)
	} else if operator == "??" {
		return c.compileNullish(node)
	}
	// Non-short-circuit operators
	if err := c.compile(node.X); err != nil {
		return err
	}
	if err := c.compile(node.Y); err != nil {
		return err
	}
	switch operator {
	case "+":
		c.emit(op.BinaryOp, uint16(op.Add))
	case "-":
		c.emit(op.BinaryOp, uint16(op.Subtract))
	case "*":
		c.emit(op.BinaryOp, uint16(op.Multiply))
	case "/":
		c.emit(op.BinaryOp, uint16(op.Divide))
	case "%":
		c.emit(op.BinaryOp, uint16(op.Modulo))
	case "**":
		c.emit(op.BinaryOp, uint16(op.Power))
	case "<<":
		c.emit(op.BinaryOp, uint16(op.LShift))
	case ">>":
		c.emit(op.BinaryOp, uint16(op.RShift))
	case "&":
		c.emit(op.BinaryOp, uint16(op.BitwiseAnd))
	case "|":
		c.emit(op.BinaryOp, uint16(op.BitwiseOr))
	case "^":
		c.emit(op.BinaryOp, uint16(op.Xor))
	case ">":
		c.emit(op.CompareOp, uint16(op.GreaterThan))
	case ">=":
		c.emit(op.CompareOp, uint16(op.GreaterThanOrEqual))
	case "<":
		c.emit(op.CompareOp, uint16(op.LessThan))
	case "<=":
		c.emit(op.CompareOp, uint16(op.LessThanOrEqual))
	case "==":
		c.emit(op.CompareOp, uint16(op.Equal))
	case "!=":
		c.emit(op.CompareOp, uint16(op.NotEqual))
	default:
		return c.formatError(fmt.Sprintf("unknown operator %q", node.Op), node.Pos())
	}
	return nil
}

func (c *Compiler) compileAnd(node *ast.Infix) error {
	// The "&&" AND operator needs to have "short circuit" behavior
	if err := c.compile(node.X); err != nil {
		return err
	}
	c.emit(op.Copy, 0) // Duplicate LHS
	jumpPos := c.emit(op.PopJumpForwardIfFalse, Placeholder)
	if err := c.compile(node.Y); err != nil {
		return err
	}
	c.emit(op.BinaryOp, uint16(op.And))
	c.emit(op.Nop)
	delta, err := c.calculateDelta(jumpPos)
	if err != nil {
		return err
	}
	c.changeOperand(jumpPos, delta)
	return nil
}

func (c *Compiler) compileOr(node *ast.Infix) error {
	// The "||" OR operator needs to have "short circuit" behavior
	if err := c.compile(node.X); err != nil {
		return err
	}
	c.emit(op.Copy, 0) // Duplicate LHS
	jumpPos := c.emit(op.PopJumpForwardIfTrue, Placeholder)
	if err := c.compile(node.Y); err != nil {
		return err
	}
	c.emit(op.BinaryOp, uint16(op.Or))
	c.emit(op.Nop)
	delta, err := c.calculateDelta(jumpPos)
	if err != nil {
		return err
	}
	c.changeOperand(jumpPos, delta)
	return nil
}

func (c *Compiler) compileNullish(node *ast.Infix) error {
	// The "??" nullish coalescing operator returns the RHS only if LHS is nil
	// Unlike ||, it doesn't treat falsy values (0, "", false) as triggering the default
	if err := c.compile(node.X); err != nil {
		return err
	}
	c.emit(op.Copy, 0) // Duplicate LHS
	jumpPos := c.emit(op.PopJumpForwardIfNotNil, Placeholder)
	c.emit(op.PopTop) // Pop the nil value
	if err := c.compile(node.Y); err != nil {
		return err
	}
	c.emit(op.Nop) // Jump target
	delta, err := c.calculateDelta(jumpPos)
	if err != nil {
		return err
	}
	c.changeOperand(jumpPos, delta)
	return nil
}

func (c *Compiler) compilePartial(call *ast.Call) error {
	args := call.Args
	argc := len(args)
	if argc > MaxArgs {
		return c.formatError(fmt.Sprintf("max args limit of %d exceeded (got %d)", MaxArgs, argc), call.Pos())
	}
	if err := c.compile(call.Fun); err != nil {
		return err
	}
	for _, arg := range args {
		if err := c.compile(arg); err != nil {
			return err
		}
	}
	c.emit(op.Partial, uint16(argc))
	return nil
}

func (c *Compiler) compilePartialObjectCall(node *ast.ObjectCall) error {
	if err := c.compile(node.X); err != nil {
		return err
	}
	method := node.Call
	name := method.Fun.String()
	c.emit(op.LoadAttr, c.addName(name))
	args := method.Args
	argc := len(args)
	if argc > MaxArgs {
		return c.formatError(fmt.Sprintf("max args limit of %d exceeded (got %d)", MaxArgs, argc), node.Pos())
	}
	for _, arg := range args {
		if err := c.compile(arg); err != nil {
			return err
		}
	}
	c.emit(op.Partial, uint16(len(args)))
	return nil
}

func (c *Compiler) constant(obj any) uint16 {
	code := c.current
	if len(code.constants) >= math.MaxUint16 {
		c.failure = fmt.Errorf("compile error: number of constants exceeded limits")
		return 0
	}
	code.constants = append(code.constants, obj)
	return uint16(len(code.constants) - 1)
}

// addName returns the index of an attribute name in the current code's names
// table. Like constant, it records a failure if the table is full.
func (c *Compiler) addName(name string) uint16 {
	idx, err := c.current.addName(name)
	if err != nil {
		c.failure = err
		return 0
	}
	return idx
}

func (c *Compiler) emit(opcode op.Code, operands ...uint16) int {
	inst := makeInstruction(opcode, operands...)
	code := c.current
	pos := len(code.instructions)
	code.instructions = append(code.instructions, inst...)

	// Track maximum call arguments for VM optimization
	if opcode == op.Call && len(operands) > 0 {
		argc := operands[0]
		if argc > code.maxCallArgs {
			code.maxCallArgs = argc
		}
	}

	// Record source location for each instruction byte
	loc := c.getCurrentLocation()
	for range inst {
		code.locations = append(code.locations, loc)
	}
	return pos
}

// emitLoad emits the appropriate load instruction based on the variable's scope.
func (c *Compiler) emitLoad(resolution *Resolution) {
	switch resolution.scope {
	case Global:
		c.emit(op.LoadGlobal, resolution.symbol.Index())
	case Local:
		c.emit(op.LoadFast, resolution.symbol.Index())
	case Free:
		c.emit(op.LoadFree, uint16(resolution.freeIndex))
	}
}

// emitStore emits the appropriate store instruction based on the variable's scope.
func (c *Compiler) emitStore(resolution *Resolution) {
	switch resolution.scope {
	case Global:
		c.emit(op.StoreGlobal, resolution.symbol.Index())
	case Local:
		c.emit(op.StoreFast, resolution.symbol.Index())
	case Free:
		c.emit(op.StoreFree, uint16(resolution.freeIndex))
	}
}

// getCurrentLocation returns the source location of the current AST node being compiled.
func (c *Compiler) getCurrentLocation() SourceLocation {
	if c.currentNode == nil {
		return SourceLocation{}
	}
	pos := c.currentNode.Pos()
	end := c.currentNode.End()
	lineNum := pos.LineNumber()

	// EndColumn is only meaningful if the end is on the same line
	endColumn := 0
	if end.Line == pos.Line {
		endColumn = end.ColumnNumber()
	}

	return SourceLocation{
		Filename:  c.filename,
		Line:      lineNum,
		Column:    pos.ColumnNumber(),
		EndColumn: endColumn,
		Source:    c.sourceLine(lineNum),
	}
}

// sourceLine returns the given 1-based line of the source that the current
// code's line numbers refer to. It gives the same result as
// c.current.GetSourceLine without splitting the source on every call.
func (c *Compiler) sourceLine(lineNum int) string {
	source := c.current.lineSource()
	if c.sourceLines == nil || source != c.linesOf {
		c.sourceLines = strings.Split(source, "\n")
		c.linesOf = source
	}
	return sourceLine(c.sourceLines, lineNum)
}

func makeInstruction(opcode op.Code, operands ...uint16) []op.Code {
	opInfo := op.GetInfo(opcode)
	if len(operands) != opInfo.OperandCount {
		panic("compile error: wrong operand count")
	}
	instruction := make([]op.Code, 1+opInfo.OperandCount)
	instruction[0] = opcode
	offset := 1
	for _, o := range operands {
		instruction[offset] = op.Code(o)
		offset++
	}
	return instruction
}

func normalizeFunctionBlock(node *ast.Block) []ast.Node {
	// Return a new slice of ast.Node that has some guarantees:
	// 1. The slice ends with the first return statement
	// 2. If there are no return statements, append one implicitly
	// 3. An implicit return will return either the value of the
	//    last expression, or nil if the last statement is not an expression.
	returnNil := &ast.Return{Value: &ast.Nil{}}
	statements := node.Stmts
	count := len(statements)
	if count == 0 {
		return []ast.Node{returnNil}
	}
	// Look for an explicit return statement. If one is found, return the
	// statements up to and including the return statement.
	for i, stmt := range statements {
		if _, ok := stmt.(*ast.Return); ok {
			return statements[:i+1]
		}
	}
	// At this point, we know there was no explicit return statement.
	// There are two cases to handle:
	//  1. The last statement is an expression (return that value)
	//  2. The last statement is not an expression (return nil)
	// Build a new slice so the caller's AST is never modified.
	last := statements[count-1]
	switch last := last.(type) {
	case ast.Expr:
		result := make([]ast.Node, count)
		copy(result, statements[:count-1])
		result[count-1] = &ast.Return{Value: last}
		return result
	default:
		result := make([]ast.Node, count, count+1)
		copy(result, statements)
		return append(result, returnNil)
	}
}

// formatError creates a detailed error message including file, line and column information
func (c *Compiler) formatError(msg string, pos token.Position) error {
	return c.formatErrorWithCode(errors.ErrorCode(""), msg, pos, nil)
}

// formatErrorWithCode creates a CompileError with an error code and optional suggestions.
func (c *Compiler) formatErrorWithCode(code errors.ErrorCode, msg string, pos token.Position, suggestions []errors.Suggestion) error {
	filename := c.filename
	if filename == "" {
		filename = "unknown"
	}

	err := &errors.CompileError{
		Code:        code,
		Message:     msg,
		Filename:    filename,
		Line:        pos.LineNumber(),
		Column:      pos.ColumnNumber(),
		SourceLine:  c.getSourceLine(pos.Line),
		Suggestions: suggestions,
	}

	return err
}

// formatUndefinedVariableError creates an error for undefined variables with "Did you mean?" suggestions.
func (c *Compiler) formatUndefinedVariableError(name string, pos token.Position) error {
	// Get all available names for suggestions
	allNames := c.current.symbols.AllNames()

	// Find similar names
	suggestions := errors.SuggestSimilar(name, allNames)

	return c.formatErrorWithCode(
		errors.E2001,
		fmt.Sprintf("undefined variable %q", name),
		pos,
		suggestions,
	)
}

// getSourceLine retrieves a specific line from the source code.
// lineNum is 0-indexed.
func (c *Compiler) getSourceLine(lineNum int) string {
	// Prefer the original source if available
	source := c.source
	if source == "" {
		source = c.current.source
	}
	if source == "" {
		source = c.main.source
	}
	if source == "" {
		return ""
	}

	lines := strings.Split(source, "\n")
	if lineNum < 0 || lineNum >= len(lines) {
		return ""
	}
	return lines[lineNum]
}

// SetSource sets the original source code for better error messages.
// This should be called before CompileAST when the original source is available.
func (c *Compiler) SetSource(source string) {
	c.source = source
}

// getRootSource returns the best available source for line lookups.
func (c *Compiler) getRootSource() string {
	if c.source != "" {
		return c.source
	}
	return c.main.source
}

// rootByteOffset converts a rune offset in the root source to a byte offset.
// The offset must be in the range [0, number of runes].
func (c *Compiler) rootByteOffset(source string, runeOffset int) int {
	if !c.rootOffsetsSet || source != c.rootOffsetsOf {
		c.rootByteOffsets = nil
		if utf8.RuneCountInString(source) != len(source) {
			offsets := make([]int, 0, utf8.RuneCountInString(source)+1)
			for i := range source {
				offsets = append(offsets, i)
			}
			c.rootByteOffsets = append(offsets, len(source))
		}
		c.rootOffsetsOf = source
		c.rootOffsetsSet = true
	}
	if c.rootByteOffsets == nil {
		return runeOffset
	}
	return c.rootByteOffsets[runeOffset]
}

// rootRuneCount returns the number of runes in the root source.
func (c *Compiler) rootRuneCount(source string) int {
	c.rootByteOffset(source, 0)
	if c.rootByteOffsets == nil {
		return len(source)
	}
	return len(c.rootByteOffsets) - 1
}

// extractFunctionBodySource attempts to extract the function body content from
// the original source using AST positions. This extracts the inner content of
// the block (without braces), or the expression of an arrow function with an
// expression body. Falls back to node.Body.String() if extraction fails.
//
// The result is a slice of the root source, not a copy, so nested functions
// share its memory. When the original source was given, start and end give
// the result's byte range within the main code's source. Otherwise, and for
// the fallback, both are zero.
func (c *Compiler) extractFunctionBodySource(node *ast.Func) (source string, start, end int) {
	rootSource := c.getRootSource()
	if rootSource == "" {
		return node.Body.String(), 0, 0
	}
	// Without the original source, the root source is built from the AST
	// text, which AST positions do not index. An expression body uses its
	// AST text, as it did before it had positions (#540).
	if c.source == "" && isExpressionBody(node.Body) {
		return node.Body.String(), 0, 0
	}
	runeCount := c.rootRuneCount(rootSource)

	// Extract inner content of the block (skip the braces).
	// Lbrace is at the '{', Rbrace is at the '}'.
	// We want the content from after '{' to before '}'.
	// An expression body has no braces: Lbrace is at its first character
	// and Rbrace at its last, so the content includes both.
	// These are rune offsets.
	bodyStart := node.Body.Lbrace.Char + 1 // Skip '{'
	bodyEnd := node.Body.Rbrace.Char       // Before '}'
	if isExpressionBody(node.Body) {
		bodyStart = node.Body.Lbrace.Char
		bodyEnd = node.Body.Rbrace.Char + 1
	}

	// Validate bounds
	if bodyStart < 0 || bodyStart >= runeCount {
		return node.Body.String(), 0, 0
	}
	if bodyEnd > runeCount {
		bodyEnd = runeCount
	}
	if bodyEnd <= bodyStart {
		return node.Body.String(), 0, 0
	}

	// Trim leading/trailing whitespace, keeping track of the byte range.
	start = c.rootByteOffset(rootSource, bodyStart)
	end = c.rootByteOffset(rootSource, bodyEnd)
	content := rootSource[start:end]
	trimmed := strings.TrimLeftFunc(content, unicode.IsSpace)
	start += len(content) - len(trimmed)
	end = start + len(strings.TrimRightFunc(trimmed, unicode.IsSpace))
	if end == start {
		return node.Body.String(), 0, 0
	}

	if c.source == "" {
		return rootSource[start:end], 0, 0
	}
	return rootSource[start:end], start + c.sourceOffset, end + c.sourceOffset
}

// isExpressionBody reports whether block is the body the parser builds for an
// arrow function with an expression body: a single return, at the same
// position as the block, which has no braces. A block without positions, as
// one built by hand may be, is not: an expression body never starts at the
// start of the source.
func isExpressionBody(block *ast.Block) bool {
	if len(block.Stmts) != 1 || !block.Lbrace.IsValid() {
		return false
	}
	ret, ok := block.Stmts[0].(*ast.Return)
	return ok && ret.Return == block.Lbrace
}

func (c *Compiler) compileTry(node *ast.Try) error {
	// Record the start of the try block
	tryStart := c.currentPosition()

	// Emit PushExcept with placeholders for catch/finally offsets
	pushExceptPos := c.emit(op.PushExcept, Placeholder, Placeholder)

	// Compile the try body - its value stays on stack as the expression result
	if err := c.compileBlock(node.Body); err != nil {
		return err
	}

	// Emit PopExcept (normal completion). This removes the exception handler,
	// or if there is a finally block, marks the handler as running finally.
	// The try block's value remains on stack
	c.emit(op.PopExcept)

	// Jump to finally if we have one, otherwise past catch block
	jumpAfterTryPos := c.emit(op.JumpForward, Placeholder)

	// Record where catch block starts (or where an exception would land if no catch)
	catchStart := c.currentPosition()

	// Compile catch block if present
	catchBlock := node.CatchBlock
	// catchVarIdx is -1 if there's no catch identifier; otherwise it's the
	// local variable index where the caught exception value will be stored.
	catchVarIdx := -1
	if catchBlock != nil {
		// Create a new scope for the catch block
		code := c.current
		code.symbols = code.symbols.NewBlock()

		// If there's a catch identifier, create a variable for it
		// The error value will be on the stack when we enter the catch block
		catchIdent := node.CatchIdent
		if catchIdent != nil {
			sym, err := code.symbols.InsertVariable(catchIdent.Name)
			if err != nil {
				code.symbols = code.symbols.parent
				return err
			}
			catchVarIdx = int(sym.Index())
			// Store the error from the stack into the catch variable
			if code.parent == nil {
				c.emit(op.StoreGlobal, sym.Index())
			} else {
				c.emit(op.StoreFast, sym.Index())
			}
		} else {
			// No catch identifier, just pop the error
			c.emit(op.PopTop)
		}

		// Compile the catch block body - its value stays on stack as the expression result
		if err := c.compileBlock(catchBlock); err != nil {
			code.symbols = code.symbols.parent
			return err
		}

		// Exit scope (catch block's value remains on stack)
		code.symbols = code.symbols.parent

		// With a finally block, the handler stays active during catch so a
		// throw or return there still runs finally. Mark catch as complete
		// before falling through, so finally runs exactly once and
		// EndFinally pops the handler.
		if node.FinallyBlock != nil {
			c.emit(op.PopExcept)
		}
	}

	// After catch, we fall through to finally (if present) or to end
	// No jump needed here - execution flows naturally to finally

	// Record where finally block starts
	finallyStart := c.currentPosition()

	// Compile finally block if present
	finallyBlock := node.FinallyBlock
	if finallyBlock != nil {
		// Compile the finally block body
		if err := c.compileBlock(finallyBlock); err != nil {
			return err
		}

		// Pop the finally block's value - in Kotlin-style semantics, finally
		// doesn't contribute to the expression result. The try/catch value
		// is already on the stack underneath.
		c.emit(op.PopTop)

		// EndFinally will re-raise any pending exception or complete pending return
		c.emit(op.EndFinally)
	}

	// Record the end position
	endPos := c.currentPosition()

	// Patch the PushExcept instruction with actual offsets
	catchOffset := uint16(catchStart - pushExceptPos)
	finallyOffset := uint16(0)
	if finallyBlock != nil {
		finallyOffset = uint16(finallyStart - pushExceptPos)
	}
	c.changeOperand(pushExceptPos, catchOffset)
	c.changeOperand(pushExceptPos+1, finallyOffset)

	// Patch jump after try:
	// - If we have finally, jump to finally
	// - If we only have catch, jump past catch to the end
	var jumpTarget int
	if finallyBlock != nil {
		jumpTarget = finallyStart
	} else {
		jumpTarget = endPos
	}
	jumpDelta := jumpTarget - jumpAfterTryPos
	// Use >= because Placeholder (MaxUint16) is reserved for unpatched jumps
	if jumpDelta >= int(Placeholder) {
		return c.formatError("try block too large", node.Pos())
	}
	c.changeOperand(jumpAfterTryPos, uint16(jumpDelta))

	// Record the exception handler
	// Only set FinallyStart if there's actually a finally block
	handlerFinallyStart := 0
	if finallyBlock != nil {
		handlerFinallyStart = finallyStart
	}
	handler := &ExceptionHandler{
		TryStart:     tryStart,
		TryEnd:       endPos,
		CatchStart:   catchStart,
		FinallyStart: handlerFinallyStart,
		CatchVarIdx:  catchVarIdx,
	}
	c.current.AddExceptionHandler(handler)

	// Try is an expression (Kotlin-style):
	// - If try succeeds: returns try block's value
	// - If exception caught: returns catch block's value
	// - Finally block runs for side effects but doesn't affect return value
	// The value is already on the stack from try or catch block.

	return nil
}

func (c *Compiler) compileThrow(node *ast.Throw) error {
	// Compile the expression to throw
	if err := c.compile(node.Value); err != nil {
		return err
	}

	// Emit Throw opcode
	c.emit(op.Throw)
	return nil
}
