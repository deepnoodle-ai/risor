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

	"github.com/risor-io/risor/ast"
	"github.com/risor-io/risor/bytecode"
	"github.com/risor-io/risor/errors"
	"github.com/risor-io/risor/internal/token"
	"github.com/risor-io/risor/op"
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
	if c.filename != "" {
		c.main.filename = c.filename
	}

	// First pass: collect function declarations to allow forward references
	if err := c.collectFunctionDeclarations(node); err != nil {
		return nil, err
	}

	// Second pass: actual compilation
	if err := c.compile(node); err != nil {
		return nil, err
	}
	// Check for failures that happened that aren't propagated up the call
	// stack. Some errors are difficult to propagate without bloating the code.
	if c.failure != nil {
		return nil, c.failure
	}
	return c.main, nil
}

// collectFunctionDeclarations walks the AST and collects all function declarations,
// adding them to the symbol table to allow forward references.
func (c *Compiler) collectFunctionDeclarations(node ast.Node) error {
	switch node := node.(type) {
	case *ast.Program:
		for _, stmt := range node.Stmts {
			if err := c.collectFunctionDeclarations(stmt); err != nil {
				return err
			}
		}
	case *ast.Block:
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
	case *ast.Switch:
		if err := c.compileSwitch(node); err != nil {
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
		panic(fmt.Sprintf("compile error: unknown ast node type: %T", node))
	}
	return nil
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
	if c.current.parent == nil {
		c.emit(op.StoreGlobal, sym.Index())
	} else {
		c.emit(op.StoreFast, sym.Index())
	}
	return nil
}

func (c *Compiler) compileIdent(node *ast.Ident) error {
	name := node.Name
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
			c.emit(op.LoadAttrOrNil, c.current.addName(binding.Key))
		} else {
			c.emit(op.LoadAttr, c.current.addName(binding.Key))
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

	// Store each value in reverse order (like MultiVar)
	for i := len(elements) - 1; i >= 0; i-- {
		element := elements[i]
		varName := element.Name.Name

		// Handle default value if present
		if hasDefaults && element.Default != nil {
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

		sym, err := c.current.symbols.InsertVariable(varName)
		if err != nil {
			return err
		}
		if c.current.parent == nil {
			c.emit(op.StoreGlobal, sym.Index())
		} else {
			c.emit(op.StoreFast, sym.Index())
		}
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
			c.emit(op.LoadAttrOrNil, c.current.addName(binding.Key))
		} else {
			c.emit(op.LoadAttr, c.current.addName(binding.Key))
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

		// Find the variable (already inserted in symbol table during setup)
		resolution, found := c.current.symbols.Resolve(varName)
		if !found {
			return c.formatError(fmt.Sprintf("undefined variable %q in destructuring", varName), param.Pos())
		}
		c.emit(op.StoreFast, resolution.symbol.Index())
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

	// Store each value in reverse order (like MultiVar)
	for i := len(elements) - 1; i >= 0; i-- {
		element := elements[i]
		varName := element.Name.Name

		// Handle default value if present
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

		// Find the variable (already inserted in symbol table during setup)
		resolution, found := c.current.symbols.Resolve(varName)
		if !found {
			return c.formatError(fmt.Sprintf("undefined variable %q in destructuring", varName), param.Pos())
		}
		c.emit(op.StoreFast, resolution.symbol.Index())
	}
	return nil
}

func (c *Compiler) compileSwitch(node *ast.Switch) error {
	// Compile the switch expression
	if err := c.compile(node.Value); err != nil {
		return err
	}

	choices := node.Cases

	// Emit jump positions for each case
	var caseJumpPositions []int
	defaultJumpPos := -1

	for i, choice := range choices {
		if choice.Default {
			defaultJumpPos = i
			continue
		}
		for _, expr := range choice.Exprs {
			// Duplicate the switch value for each case comparison
			c.emit(op.Copy, 0)
			// Compile the case expression
			if err := c.compile(expr); err != nil {
				return err
			}
			// Emit the CompareOp equal comparison
			c.emit(op.CompareOp, uint16(op.Equal))
			// Emit PopJumpForwardIfTrue and store its position
			jumpPos := c.emit(op.PopJumpForwardIfTrue, Placeholder)
			caseJumpPositions = append(caseJumpPositions, jumpPos)
		}
	}

	jumpDefaultPos := c.emit(op.JumpForward, Placeholder)

	// Update case jump positions and compile case blocks
	var offset int
	var endBlockPosits []int
	for i, choice := range choices {
		if i == defaultJumpPos {
			continue
		}
		for range choice.Exprs {
			delta, err := c.calculateDelta(caseJumpPositions[offset])
			if err != nil {
				return err
			}
			c.changeOperand(caseJumpPositions[offset], delta)
			offset++
		}
		if choice.Body == nil {
			// Empty case block
			c.emit(op.Nil)
		} else {
			if err := c.compile(choice.Body); err != nil {
				return err
			}
		}
		endBlockPosits = append(endBlockPosits, c.emit(op.JumpForward, Placeholder))
	}

	delta, err := c.calculateDelta(jumpDefaultPos)
	if err != nil {
		return err
	}
	c.changeOperand(jumpDefaultPos, delta)

	// Compile the default case block if it exists
	if defaultJumpPos != -1 {
		if err := c.compile(choices[defaultJumpPos].Body); err != nil {
			return err
		}
	} else {
		c.emit(op.Nil)
	}

	// Update end block jump positions
	for _, pos := range endBlockPosits {
		delta, err := c.calculateDelta(pos)
		if err != nil {
			return err
		}
		c.changeOperand(pos, delta)
	}

	c.emit(op.Swap, 1)

	// Remove the duplicated switch value from the stack
	c.emit(op.PopTop)
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
		return fmt.Errorf("compile error: string template exceeded max fragment size")
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
		return fmt.Errorf("compile error: invalid nested pipe")
	}
	exprs := node.Exprs
	if len(exprs) < 2 {
		return fmt.Errorf("compile error: the pipe operator requires at least two expressions")
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
		// 1. Load the current value
		if err := c.compile(x.X); err != nil {
			return err
		}
		if err := c.compile(x.Index); err != nil {
			return err
		}
		c.emit(op.BinarySubscr)
		// 2. Add the increment amount
		c.emit(op.LoadConst, c.constant(amount))
		c.emit(op.BinaryOp, uint16(op.Add))
		// 3. Store back
		if err := c.compile(x.X); err != nil {
			return err
		}
		if err := c.compile(x.Index); err != nil {
			return err
		}
		c.emit(op.StoreSubscr)

	case *ast.GetAttr:
		// Attribute expression: obj.x++
		idx := c.current.addName(x.Attr.Name)
		// 1. Load the current attribute value
		if err := c.compile(x.X); err != nil {
			return err
		}
		c.emit(op.LoadAttr, idx)
		// 2. Add the increment amount
		c.emit(op.LoadConst, c.constant(amount))
		c.emit(op.BinaryOp, uint16(op.Add))
		// 3. Store back
		if err := c.compile(x.X); err != nil {
			return err
		}
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
	args := node.Args
	argc := len(args)
	if argc > MaxArgs {
		return fmt.Errorf("compile error: max args limit of %d exceeded (got %d)", MaxArgs, argc)
	}

	// Check if any arguments are spread expressions
	hasSpread := false
	for _, arg := range args {
		if _, ok := arg.(*ast.Spread); ok {
			hasSpread = true
			break
		}
	}

	if err := c.compile(node.Fun); err != nil {
		return err
	}

	if !hasSpread {
		// Fast path: no spread, use regular Call
		for _, arg := range args {
			if err := c.compile(arg); err != nil {
				return err
			}
		}
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
		return fmt.Errorf("compile error: spread arguments not supported in pipe expressions")
	}
	c.emit(op.CallSpread)
	return nil
}

func (c *Compiler) compileObjectCall(node *ast.ObjectCall) error {
	if err := c.compile(node.X); err != nil {
		return err
	}
	// Handle optional chaining (?.)
	var jumpPos int
	if node.Optional {
		c.emit(op.Copy, 0)
		jumpPos = c.emit(op.PopJumpForwardIfNil, Placeholder)
	}
	method := node.Call
	name := method.Fun.String()
	c.emit(op.LoadAttr, c.current.addName(name))
	args := method.Args
	argc := len(args)
	if argc > MaxArgs {
		return fmt.Errorf("compile error: max args limit of %d exceeded (got %d)", MaxArgs, argc)
	}
	for _, arg := range args {
		if err := c.compile(arg); err != nil {
			return err
		}
	}
	if c.current.pipeActive {
		c.emit(op.Partial, uint16(len(args)))
	} else {
		c.emit(op.Call, uint16(len(args)))
	}
	if node.Optional {
		c.emit(op.Nop)
		delta, _ := c.calculateDelta(jumpPos)
		c.changeOperand(jumpPos, delta)
	}
	return nil
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
	idx := c.current.addName(node.Attr.Name)
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
		return fmt.Errorf("compile error: list literal exceeds max size")
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
				return fmt.Errorf("compile error: invalid map key type: %v", item.Key)
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
				return fmt.Errorf("compile error: expected spread expression in map")
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
				return fmt.Errorf("compile error: invalid map key type: %v", item.Key)
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

	if len(node.Params) > 255 {
		return c.formatError("function exceeded parameter limit of 255", node.Pos())
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
	bodySource := c.extractFunctionBodySource(node)
	code := c.current.newChild(functionName, bodySource, functionID)

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
			line := node.Pos().Line + 1
			return fmt.Errorf("compile error: unsupported default value (got %s, line %d)", expr, line)
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
	for _, paramName := range params {
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
		if _, err := code.symbols.InsertVariable(restParamName); err != nil {
			return err
		}
	}

	// Add all destructured variable names to the symbol table.
	// These come after rest param since the destructuring preamble will
	// store extracted values into these local variables.
	for _, di := range destructureParams {
		for _, name := range di.param.ParamNames() {
			if _, err := code.symbols.InsertVariable(name); err != nil {
				return err
			}
		}
	}

	// Add the function's own name to its symbol table. This supports recursive
	// calls to the function. Later when we create the function object, we'll
	// add the object value to the table.
	if code.isNamed {
		if _, err := code.symbols.InsertConstant(functionName); err != nil {
			return err
		}
	}

	// Emit destructuring preamble for any destructured parameters
	// This runs at the start of the function to extract values into local vars
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
	freeCount := code.symbols.FreeCount()
	if freeCount > 0 {
		for i := uint16(0); i < freeCount; i++ {
			resolution := code.symbols.Free(i)
			c.emit(op.MakeCell, resolution.symbol.Index(), uint16(resolution.depth-1))
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
		// 1. Load the current value: test[0]
		if err := c.compile(index.X); err != nil {
			return err
		}
		if err := c.compile(index.Index); err != nil {
			return err
		}
		c.emit(op.BinarySubscr)

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
			return fmt.Errorf("compile error: unsupported compound assignment operator: %s", node.Op)
		}
	} else {
		// Simple assignment
		if err := c.compile(node.Value); err != nil {
			return err
		}
	}

	// 4. Store the result back
	if err := c.compile(index.X); err != nil {
		return err
	}
	if err := c.compile(index.Index); err != nil {
		return err
	}
	c.emit(op.StoreSubscr)
	return nil
}

func (c *Compiler) compileAssign(node *ast.Assign) error {
	if node.Index != nil {
		return c.compileSetItem(node)
	}
	name := node.Name.Name
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
	idx := c.current.addName(node.Attr.Name)

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

	// Compound assignment: load current value, apply operation, store result
	// First, load current attribute value
	if err := c.compile(node.X); err != nil {
		return err
	}
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

	// Compile the object again and store the attribute
	if err := c.compile(node.X); err != nil {
		return err
	}
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
	if delta > math.MaxUint16 {
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
		return fmt.Errorf("compile error: max args limit of %d exceeded (got %d)", MaxArgs, argc)
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
	c.emit(op.LoadAttr, c.current.addName(name))
	args := method.Args
	argc := len(args)
	if argc > MaxArgs {
		return fmt.Errorf("compile error: max args limit of %d exceeded (got %d)", MaxArgs, argc)
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
		Source:    c.current.GetSourceLine(lineNum),
	}
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
	last := statements[count-1]
	switch last := last.(type) {
	case ast.Expr:
		statements[count-1] = &ast.Return{Value: last}
	default:
		statements = append(statements, returnNil)
	}
	return statements
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

// extractFunctionBodySource attempts to extract the function body content from
// the original source using AST positions. This extracts the inner content of
// the block (without braces) to match what node.Body.String() produces.
// Falls back to node.Body.String() if extraction fails.
func (c *Compiler) extractFunctionBodySource(node *ast.Func) string {
	rootSource := c.getRootSource()
	if rootSource == "" {
		return node.Body.String()
	}

	// Extract inner content of the block (skip the braces).
	// Lbrace is at the '{', Rbrace is at the '}'.
	// We want the content from after '{' to before '}'.
	bodyStart := node.Body.Lbrace.Char + 1 // Skip '{'
	bodyEnd := node.Body.Rbrace.Char       // Before '}'

	// Validate bounds
	if bodyStart < 0 || bodyStart >= len(rootSource) {
		return node.Body.String()
	}
	if bodyEnd > len(rootSource) {
		bodyEnd = len(rootSource)
	}
	if bodyEnd <= bodyStart {
		return node.Body.String()
	}

	// Extract and trim leading/trailing whitespace
	content := strings.TrimSpace(rootSource[bodyStart:bodyEnd])
	if content == "" {
		return node.Body.String()
	}

	return content
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

	// Emit PopExcept (normal completion - removes exception handler)
	// The try block's value remains on stack
	c.emit(op.PopExcept)

	// Jump to finally if we have one, otherwise past catch block
	jumpAfterTryPos := c.emit(op.JumpForward, Placeholder)

	// Record where catch block starts (or where an exception would land if no catch)
	catchStart := c.currentPosition()

	// Compile catch block if present
	catchBlock := node.CatchBlock
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
	if jumpDelta > int(Placeholder) {
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
