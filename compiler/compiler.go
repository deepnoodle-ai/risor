// Package compiler is used to compile a Risor abstract syntax tree (AST) into
// the corresponding bytecode.
package compiler

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/risor-io/risor/ast"
	"github.com/risor-io/risor/op"
	"github.com/risor-io/risor/token"
)

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
}

// Option is a configuration function for a Compiler.
type Option func(*Compiler)

// WithGlobalNames configures the compiler with the given global variable names.
func WithGlobalNames(names []string) Option {
	return func(c *Compiler) {
		c.globalNames = make([]string, len(names))
		copy(c.globalNames, names) // isolate from caller
	}
}

// WithCode configures the compiler to compile into the given code object.
func WithCode(code *Code) Option {
	return func(c *Compiler) {
		c.main = code
	}
}

// WithFilename configures the compiler with the source filename.
func WithFilename(filename string) Option {
	return func(c *Compiler) {
		c.filename = filename
	}
}

// Compile the given AST node and return the compiled code object. This is a
// shorthand for compiler.New(options).Compile(node).
func Compile(node ast.Node, options ...Option) (*Code, error) {
	c, err := New(options...)
	if err != nil {
		return nil, err
	}
	return c.Compile(node)
}

// New creates and returns a new Compiler. Any supplied options are used to
// configure the compilation process.
func New(options ...Option) (*Compiler, error) {
	c := &Compiler{}
	for _, opt := range options {
		opt(c)
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
	// Start compiling into the main code object
	c.current = c.main
	return c, nil
}

// Code returns the compiled code for the entrypoint.
func (c *Compiler) Code() *Code {
	return c.main
}

// Compile the given AST node and return the compiled code object.
func (c *Compiler) Compile(node ast.Node) (*Code, error) {
	c.failure = nil
	if c.main.source == "" {
		c.main.source = node.String()
	} else {
		c.main.source = fmt.Sprintf("%s\n%s", c.main.source, node.String())
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
		for _, stmt := range node.Statements() {
			if err := c.collectFunctionDeclarations(stmt); err != nil {
				return err
			}
		}
	case *ast.Block:
		for _, stmt := range node.Statements() {
			if err := c.collectFunctionDeclarations(stmt); err != nil {
				return err
			}
		}
	case *ast.Func:
		// Only collect named functions at the top level
		if node.Name() != nil && c.current.parent == nil {
			functionName := node.Name().Literal()
			if _, found := c.current.symbols.Get(functionName); found {
				return c.formatError(fmt.Sprintf("function %q redefined", functionName), node.Token().StartPosition)
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
	case *ast.For:
		if err := c.compileFor(node); err != nil {
			return err
		}
	case *ast.ForIn:
		if err := c.compileForIn(node); err != nil {
			return err
		}
	case *ast.Control:
		if err := c.compileControl(node); err != nil {
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
	case *ast.Ternary:
		if err := c.compileTernary(node); err != nil {
			return err
		}
	case *ast.Range:
		if err := c.compileRange(node); err != nil {
			return err
		}
	case *ast.Slice:
		if err := c.compileSlice(node); err != nil {
			return err
		}
	case *ast.Import:
		if err := c.compileImport(node); err != nil {
			return err
		}
	case *ast.FromImport:
		if err := c.compileFromImport(node); err != nil {
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
	case *ast.SetAttr:
		if err := c.compileSetAttr(node); err != nil {
			return err
		}
	default:
		panic(fmt.Sprintf("compile error: unknown ast node type: %T", node))
	}
	return nil
}

// startLoop should be called when starting to compile a new loop. This is used
// to understand which loop that "break" and "continue" statements should target.
func (c *Compiler) startLoop() *loop {
	currentCode := c.current
	loop := &loop{code: currentCode}
	currentCode.loops = append(currentCode.loops, loop)
	return loop
}

// currentLoop returns the loop that is currently being compiled, which is the
// loop that "break" and "continue" statements should target.
func (c *Compiler) currentLoop() *loop {
	loops := c.current.loops
	if len(loops) == 0 {
		return nil
	}
	return loops[len(loops)-1]
}

func (c *Compiler) currentPosition() int {
	return len(c.current.instructions)
}

func (c *Compiler) compileNil() error {
	c.emit(op.Nil)
	return nil
}

func (c *Compiler) compileInt(node *ast.Int) error {
	c.emit(op.LoadConst, c.constant(node.Value()))
	return nil
}

func (c *Compiler) compileFloat(node *ast.Float) error {
	c.emit(op.LoadConst, c.constant(node.Value()))
	return nil
}

func (c *Compiler) compileBool(node *ast.Bool) error {
	if node.Value() {
		c.emit(op.True)
	} else {
		c.emit(op.False)
	}
	return nil
}

func (c *Compiler) compileProgram(node *ast.Program) error {
	statements := node.Statements()
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
				if stmt.IsExpression() {
					c.emit(op.PopTop)
				}
			}
		}
		// Guarantee that the program evaluates to a value
		lastStatement := statements[count-1]
		if !lastStatement.IsExpression() {
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
	statements := node.Statements()
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
				if stmt.IsExpression() {
					c.emit(op.PopTop)
				}
			}
		}
		// Guarantee that the block evaluates to a value
		lastStatement := statements[count-1]
		if !lastStatement.IsExpression() {
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
			if stmt.IsExpression() {
				c.emit(op.PopTop)
			}
		}
	}
	return nil
}

func (c *Compiler) compileVar(node *ast.Var) error {
	name, expr := node.Value()
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
	name := node.Literal()
	resolution, found := c.current.symbols.Resolve(name)
	if !found {
		return c.formatError(fmt.Sprintf("undefined variable %q", name), node.Token().StartPosition)
	}
	switch resolution.scope {
	case Global:
		c.emit(op.LoadGlobal, resolution.symbol.Index())
	case Local:
		c.emit(op.LoadFast, resolution.symbol.Index())
	case Free:
		c.emit(op.LoadFree, uint16(resolution.freeIndex))
	}
	return nil
}

func (c *Compiler) compileMultiVar(node *ast.MultiVar) error {
	names, expr := node.Value()
	if len(names) > math.MaxUint16 {
		return c.formatError("too many variables in multi-variable assignment", node.Token().StartPosition)
	}
	// Compile the RHS value
	if err := c.compile(expr); err != nil {
		return err
	}
	// Emit the Unpack opcode to unpack the tuple-like object onto the stack
	c.emit(op.Unpack, uint16(len(names)))
	// Iterate through the names in reverse order and declare the variables
	for i := len(names) - 1; i >= 0; i-- {
		name := names[i]
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
	bindings := node.Bindings()
	if len(bindings) > math.MaxUint16 {
		return c.formatError("too many bindings in object destructuring", node.Token().StartPosition)
	}

	// Compile the source object
	if err := c.compile(node.Value()); err != nil {
		return err
	}

	// For each binding, load the property and store it in a variable
	for _, binding := range bindings {
		// Duplicate the object on the stack (we need it for each property access)
		c.emit(op.Copy, 0)

		// Load the property using LoadAttr
		c.emit(op.LoadAttr, c.current.addName(binding.Key))

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

func (c *Compiler) compileSwitch(node *ast.Switch) error {
	// Compile the switch expression
	if err := c.compile(node.Value()); err != nil {
		return err
	}

	choices := node.Choices()

	// Emit jump positions for each case
	var caseJumpPositions []int
	defaultJumpPos := -1

	for i, choice := range choices {
		if choice.IsDefault() {
			defaultJumpPos = i
			continue
		}
		for _, expr := range choice.Expressions() {
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
		for range choice.Expressions() {
			delta, err := c.calculateDelta(caseJumpPositions[offset])
			if err != nil {
				return err
			}
			c.changeOperand(caseJumpPositions[offset], delta)
			offset++
		}
		if choice.Block() == nil {
			// Empty case block
			c.emit(op.Nil)
		} else {
			if err := c.compile(choice.Block()); err != nil {
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
		if err := c.compile(choices[defaultJumpPos].Block()); err != nil {
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

func (c *Compiler) compileImport(node *ast.Import) error {
	moduleName := node.Path().Value()
	c.emit(op.LoadConst, c.constant(moduleName))
	c.emit(op.Import)

	// Determine the variable name to store the imported module
	var name string
	if node.Alias() != nil {
		name = node.Alias().String()
	} else {
		name = node.ModuleName()
	}

	var sym *Symbol
	var found bool
	sym, found = c.current.symbols.Get(name)
	if !found {
		var err error
		sym, err = c.current.symbols.InsertConstant(name)
		if err != nil {
			return err
		}
	}
	if c.current.parent == nil {
		c.emit(op.StoreGlobal, sym.Index())
	} else {
		c.emit(op.StoreFast, sym.Index())
	}
	return nil
}

func (c *Compiler) compileFromImport(node *ast.FromImport) error {
	if len(node.Parents()) > 255 {
		return fmt.Errorf("compile error: too many parents in from-import")
	}
	for _, parent := range node.Parents() {
		c.emit(op.LoadConst, c.constant(parent.String()))
	}
	aliases := map[string]string{}
	for _, im := range node.Imports() {
		name := im.Path().Value()
		alias := name
		if im.Alias() != nil {
			alias = im.Alias().String()
		}
		c.emit(op.LoadConst, c.constant(name))
		aliases[name] = alias
	}
	c.emit(op.FromImport, uint16(len(node.Parents())), uint16(len(node.Imports())))
	for _, im := range node.Imports() {
		alias := aliases[im.Path().Value()]
		var sym *Symbol
		var found bool
		sym, found = c.current.symbols.Get(alias)
		if !found {
			var err error
			sym, err = c.current.symbols.InsertConstant(alias)
			if err != nil {
				return err
			}
		}
		if c.current.parent == nil {
			c.emit(op.StoreGlobal, sym.Index())
		} else {
			c.emit(op.StoreFast, sym.Index())
		}
	}
	return nil
}

func (c *Compiler) compileSlice(node *ast.Slice) error {
	if err := c.compile(node.Left()); err != nil {
		return err
	}
	to := node.ToIndex()
	if to == nil {
		c.emit(op.Copy, 0)
		c.emit(op.Length)
	} else {
		if err := c.compile(to); err != nil {
			return err
		}
	}
	from := node.FromIndex()
	if from == nil {
		c.emit(op.LoadConst, c.constant(int64(0)))
	} else {
		if err := c.compile(from); err != nil {
			return err
		}
	}
	c.emit(op.Slice)
	return nil
}

func (c *Compiler) compileRange(node *ast.Range) error {
	if err := c.compile(node.Container()); err != nil {
		return err
	}
	c.emit(op.Range)
	return nil
}

func (c *Compiler) compileTernary(node *ast.Ternary) error {
	// evaluate the condition and then conditionally jump to the false case
	if err := c.compile(node.Condition()); err != nil {
		return err
	}
	jumpIfFalsePos := c.emit(op.PopJumpForwardIfFalse, Placeholder)

	// true case execution, then jump over false case
	if err := c.compile(node.IfTrue()); err != nil {
		return err
	}
	trueCaseEndPos := c.emit(op.JumpForward, Placeholder)

	// set the jump amount to reach the beginning of the false case
	falseCaseDelta, err := c.calculateDelta(jumpIfFalsePos)
	if err != nil {
		return err
	}
	c.changeOperand(jumpIfFalsePos, falseCaseDelta)

	// false case execution
	if err := c.compile(node.IfFalse()); err != nil {
		return err
	}

	// set the jump amount for the end of the true case
	endDelta, err := c.calculateDelta(trueCaseEndPos)
	if err != nil {
		return err
	}
	c.changeOperand(trueCaseEndPos, endDelta)
	return nil
}

func (c *Compiler) compileString(node *ast.String) error {
	// Is the string a template or a simple string?
	tmpl := node.Template()

	// Simple strings are just emitted as a constant
	if tmpl == nil {
		c.emit(op.LoadConst, c.constant(node.Value()))
		return nil
	}

	fragments := tmpl.Fragments()
	if len(fragments) > math.MaxUint16 {
		return fmt.Errorf("compile error: string template exceeded max fragment size")
	}

	var expressionIndex int
	expressions := node.TemplateExpressions()

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
	exprs := node.Expressions()
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
	name := node.Literal()
	resolution, found := c.current.symbols.Resolve(name)
	if !found {
		return c.formatError(fmt.Sprintf("undefined variable %q", name), node.Token().StartPosition)
	}
	symbolIndex := resolution.symbol.Index()
	// Push the named variable onto the stack
	switch resolution.scope {
	case Global:
		c.emit(op.LoadGlobal, symbolIndex)
	case Local:
		c.emit(op.LoadFast, symbolIndex)
	case Free:
		c.emit(op.LoadFree, uint16(resolution.freeIndex))
	}
	// Push the integer amount to the stack (1 or -1)
	operator := node.Operator()
	if operator == "++" {
		c.emit(op.LoadConst, c.constant(int64(1)))
	} else if operator == "--" {
		c.emit(op.LoadConst, c.constant(int64(-1)))
	} else {
		return c.formatError(fmt.Sprintf("unknown postfix operator %q", operator), node.Token().StartPosition)
	}
	// Run increment or decrement as an Add BinaryOp
	c.emit(op.BinaryOp, uint16(op.Add))
	// Store TOS in LHS
	switch resolution.scope {
	case Global:
		c.emit(op.StoreGlobal, symbolIndex)
	case Local:
		c.emit(op.StoreFast, symbolIndex)
	case Free:
		c.emit(op.StoreFree, uint16(resolution.freeIndex))
	}
	return nil
}

func (c *Compiler) compileConst(node *ast.Const) error {
	name, expr := node.Value()
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
	if err := c.compile(node.Right()); err != nil {
		return err
	}
	if err := c.compile(node.Left()); err != nil {
		return err
	}
	c.emit(op.ContainsOp, 0)
	return nil
}

func (c *Compiler) compileNotIn(node *ast.NotIn) error {
	if err := c.compile(node.Right()); err != nil {
		return err
	}
	if err := c.compile(node.Left()); err != nil {
		return err
	}
	c.emit(op.ContainsOp, 0)
	c.emit(op.UnaryNot)
	return nil
}

func (c *Compiler) compilePrefix(node *ast.Prefix) error {
	if err := c.compile(node.Right()); err != nil {
		return err
	}
	switch node.Operator() {
	case "!":
		c.emit(op.UnaryNot)
	case "-":
		c.emit(op.UnaryNegative)
	}
	return nil
}

func (c *Compiler) compileCall(node *ast.Call) error {
	args := node.Arguments()
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

	if err := c.compile(node.Function()); err != nil {
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
			if err := c.compile(spread.Value()); err != nil {
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
	if err := c.compile(node.Object()); err != nil {
		return err
	}
	// Handle optional chaining (?.)
	var jumpPos int
	if node.IsOptional() {
		c.emit(op.Copy, 0)
		jumpPos = c.emit(op.PopJumpForwardIfNil, Placeholder)
	}
	expr := node.Call()
	method, ok := expr.(*ast.Call)
	if !ok {
		return fmt.Errorf("compile error: invalid call expression")
	}
	name := method.Function().String()
	c.emit(op.LoadAttr, c.current.addName(name))
	args := method.Arguments()
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
	if node.IsOptional() {
		c.emit(op.Nop)
		delta, _ := c.calculateDelta(jumpPos)
		c.changeOperand(jumpPos, delta)
	}
	return nil
}

func (c *Compiler) compileGetAttr(node *ast.GetAttr) error {
	if err := c.compile(node.Object()); err != nil {
		return err
	}
	// Handle optional chaining (?.)
	var jumpPos int
	if node.IsOptional() {
		c.emit(op.Copy, 0)
		jumpPos = c.emit(op.PopJumpForwardIfNil, Placeholder)
	}
	idx := c.current.addName(node.Name())
	c.emit(op.LoadAttr, idx)
	if node.IsOptional() {
		c.emit(op.Nop)
		delta, _ := c.calculateDelta(jumpPos)
		c.changeOperand(jumpPos, delta)
	}
	return nil
}

func (c *Compiler) compileIndex(node *ast.Index) error {
	if err := c.compile(node.Left()); err != nil {
		return err
	}
	if err := c.compile(node.Index()); err != nil {
		return err
	}
	c.emit(op.BinarySubscr)
	return nil
}

func (c *Compiler) compileList(node *ast.List) error {
	items := node.Items()
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
			if err := c.compile(spread.Value()); err != nil {
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
	items := node.Items()
	count := len(items)
	for k, v := range items {
		switch k := k.(type) {
		case *ast.String:
			if err := c.compile(k); err != nil {
				return err
			}
		case *ast.Ident:
			c.emit(op.LoadConst, c.constant(k.String()))
		default:
			return fmt.Errorf("compile error: invalid map key type: %v", k)
		}
		if err := c.compile(v); err != nil {
			return err
		}
	}
	c.emit(op.BuildMap, uint16(count))
	return nil
}

func (c *Compiler) compileFunc(node *ast.Func) error {
	// Python cell variables:
	// https://stackoverflow.com/questions/23757143/what-is-a-cell-in-the-context-of-an-interpreter-or-compiler

	if len(node.Parameters()) > 255 {
		return c.formatError("function exceeded parameter limit of 255", node.Token().StartPosition)
	}

	// The function has an optional name. If it is named, the name will be
	// stored in the function's own symbol table to support recursive calls.
	var functionName string
	if ident := node.Name(); ident != nil {
		functionName = ident.Literal()
	}

	// This new code object will store the compiled code for this function.
	c.funcIndex++
	functionID := fmt.Sprintf("%d", c.funcIndex)
	code := c.current.newChild(functionName, node.Body().String(), functionID)

	// Setting current here means subsequent calls to compile will add to this
	// code object instead of the parent.
	c.current = code

	// Make it quick to look up the index of a parameter
	paramsIdx := map[string]int{}
	params := node.ParameterNames()
	for i, name := range params {
		paramsIdx[name] = i
	}

	// Build an array of default values for parameters, supporting only
	// the basic types of int, string, bool, float, and nil.
	defaults := make([]any, len(params))
	defaultsSet := map[int]bool{}
	for name, expr := range node.Defaults() {
		var value any
		switch expr := expr.(type) {
		case *ast.Int:
			value = expr.Value()
		case *ast.String:
			value = expr.Value()
		case *ast.Bool:
			value = expr.Value()
		case *ast.Float:
			value = expr.Value()
		case *ast.Nil:
			value = nil
		default:
			line := node.Token().StartPosition.Line + 1
			return fmt.Errorf("compile error: unsupported default value (got %s, line %d)", expr, line)
		}
		index := paramsIdx[name]
		defaults[index] = value
		defaultsSet[index] = true
	}

	// Confirm only trailing parameters have defaults
	if len(node.Defaults()) > 0 {
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
				return c.formatError(msg, node.Token().StartPosition)
			}
		}
	}

	// Add the parameter names to the symbol table
	for _, arg := range node.Parameters() {
		if _, err := code.symbols.InsertVariable(arg.Literal()); err != nil {
			return err
		}
	}

	// Add rest parameter to symbol table if present
	var restParamName string
	if restParam := node.RestParam(); restParam != nil {
		restParamName = restParam.Literal()
		if _, err := code.symbols.InsertVariable(restParamName); err != nil {
			return err
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

	// Compile the function body
	if err := c.compileFunctionBlock(node.Body()); err != nil {
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

func (c *Compiler) compileControl(node *ast.Control) error {
	literal := node.Literal()
	loop := c.currentLoop()
	if loop == nil {
		if literal == "break" {
			return c.formatError("invalid break statement outside of a loop", node.Token().StartPosition)
		}
		return c.formatError("invalid continue statement outside of a loop", node.Token().StartPosition)
	}
	if literal == "break" {
		// When breaking from a for-range loop, we need to pop the iterator from the stack
		if loop.isRangeLoop {
			c.emit(op.PopTop)
		}
		position := c.emit(op.JumpForward, Placeholder)
		loop.breakPos = append(loop.breakPos, position)
	} else {
		position := c.emit(op.JumpForward, Placeholder)
		loop.continuePos = append(loop.continuePos, position)
	}
	return nil
}

func (c *Compiler) compileReturn(node *ast.Return) error {
	if c.current.IsRoot() {
		return c.formatError("invalid return statement outside of a function", node.Token().StartPosition)
	}
	value := node.Value()
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
	index := node.Index()

	// Handle compound operators (*=, +=, etc.)
	if node.Operator() != "=" {
		// 1. Load the current value: test[0]
		if err := c.compile(index.Left()); err != nil {
			return err
		}
		if err := c.compile(index.Index()); err != nil {
			return err
		}
		c.emit(op.BinarySubscr)

		// 2. Load the RHS value
		if err := c.compile(node.Value()); err != nil {
			return err
		}

		// 3. Apply the compound operation
		switch node.Operator() {
		case "+=":
			c.emit(op.BinaryOp, uint16(op.Add))
		case "-=":
			c.emit(op.BinaryOp, uint16(op.Subtract))
		case "*=":
			c.emit(op.BinaryOp, uint16(op.Multiply))
		case "/=":
			c.emit(op.BinaryOp, uint16(op.Divide))
		default:
			return fmt.Errorf("compile error: unsupported compound assignment operator: %s", node.Operator())
		}
	} else {
		// Simple assignment
		if err := c.compile(node.Value()); err != nil {
			return err
		}
	}

	// 4. Store the result back
	if err := c.compile(index.Left()); err != nil {
		return err
	}
	if err := c.compile(index.Index()); err != nil {
		return err
	}
	c.emit(op.StoreSubscr)
	return nil
}

func (c *Compiler) compileAssign(node *ast.Assign) error {
	if node.Index() != nil {
		return c.compileSetItem(node)
	}
	name := node.Name()
	resolution, found := c.current.symbols.Resolve(name)
	if !found {
		return c.formatError(fmt.Sprintf("undefined variable %q", name), node.Token().StartPosition)
	}
	sym := resolution.symbol
	if sym.IsConstant() {
		return c.formatError(fmt.Sprintf("cannot assign to constant %q", name), node.Token().StartPosition)
	}
	symbolIndex := sym.Index()
	if node.Operator() == "=" {
		if err := c.compile(node.Value()); err != nil {
			return err
		}
		switch resolution.scope {
		case Global:
			c.emit(op.StoreGlobal, symbolIndex)
		case Local:
			c.emit(op.StoreFast, symbolIndex)
		case Free:
			c.emit(op.StoreFree, uint16(resolution.freeIndex))
		}
		return nil
	}
	// Push LHS as TOS
	switch resolution.scope {
	case Global:
		c.emit(op.LoadGlobal, symbolIndex)
	case Local:
		c.emit(op.LoadFast, symbolIndex)
	case Free:
		c.emit(op.LoadFree, uint16(resolution.freeIndex))
	}
	// Push RHS as TOS
	if err := c.compile(node.Value()); err != nil {
		return err
	}
	// Result becomes TOS
	switch node.Operator() {
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
	switch resolution.scope {
	case Global:
		c.emit(op.StoreGlobal, symbolIndex)
	case Local:
		c.emit(op.StoreFast, symbolIndex)
	case Free:
		c.emit(op.StoreFree, uint16(resolution.freeIndex))
	}
	return nil
}

func (c *Compiler) compileSetAttr(node *ast.SetAttr) error {
	// Handle compound operators (*=, +=, etc.)
	if node.Token().Type != token.ASSIGN {
		// 1. Load the current value
		if err := c.compile(node.Object()); err != nil {
			return err
		}
		idx := c.current.addName(node.Name())
		c.emit(op.LoadAttr, idx)

		// 2. Load the RHS value
		if err := c.compile(node.Value()); err != nil {
			return err
		}

		// 3. Apply the compound operation
		switch node.Token().Type {
		case token.PLUS_EQUALS:
			c.emit(op.BinaryOp, uint16(op.Add))
		case token.MINUS_EQUALS:
			c.emit(op.BinaryOp, uint16(op.Subtract))
		case token.ASTERISK_EQUALS:
			c.emit(op.BinaryOp, uint16(op.Multiply))
		case token.SLASH_EQUALS:
			c.emit(op.BinaryOp, uint16(op.Divide))
		default:
			return fmt.Errorf("compile error: unsupported compound assignment operator: %s", node.Token().Literal)
		}
	} else {
		// Simple assignment
		if err := c.compile(node.Value()); err != nil {
			return err
		}
	}

	// 4. Store the result back
	if err := c.compile(node.Object()); err != nil {
		return err
	}
	idx := c.current.addName(node.Name())
	c.emit(op.StoreAttr, idx)
	return nil
}

func (c *Compiler) compileForRange(forNode *ast.For, names []string, container ast.Node) error {
	if err := c.compile(container); err != nil {
		return err
	}
	// Get an iterator for the container at TOS
	c.emit(op.GetIter)

	code := c.current
	code.symbols = code.symbols.NewBlock()
	loop := c.startLoop()
	loop.isRangeLoop = true
	defer func() {
		loop.end()
		code.symbols = code.symbols.parent
	}()

	iterPos := c.emit(op.ForIter, 0, uint16(len(names)))

	// assign the current value of the iterator to the loop variable
	for _, name := range names {
		sym, err := code.symbols.InsertVariable(name)
		if err != nil {
			return err
		}
		if code.symbols.IsGlobal() {
			c.emit(op.StoreGlobal, sym.Index())
		} else {
			c.emit(op.StoreFast, sym.Index())
		}
	}

	// compile the body of the loop
	if err := c.compile(forNode.Consequence()); err != nil {
		return err
	}
	c.emit(op.PopTop)

	// jump back to the start of the loop
	delta, err := c.calculateDelta(iterPos)
	if err != nil {
		return err
	}
	jumpBackPos := c.emit(op.JumpBackward, delta)

	// update the ForIter instruction to jump "here" when done
	delta, err = c.calculateDelta(iterPos)
	if err != nil {
		return err
	}
	c.changeOperand(iterPos, delta)

	// Update breaks to jump to this point
	for _, pos := range loop.breakPos {
		delta, err = c.calculateDelta(pos)
		if err != nil {
			return err
		}
		c.changeOperand(pos, uint16(delta))
	}

	// Update continues
	for _, pos := range loop.continuePos {
		delta := jumpBackPos - pos
		if delta > math.MaxUint16 {
			return c.formatError("loop code size exceeded limits", forNode.Token().StartPosition)
		}
		c.changeOperand(pos, uint16(delta))
	}
	return nil
}

func (c *Compiler) compileForCondition(forNode *ast.For, condition ast.Expression) error {
	code := c.current
	code.symbols = code.symbols.NewBlock()
	loop := c.startLoop()
	defer func() {
		loop.end()
		code.symbols = code.symbols.parent
	}()
	startPos := c.currentPosition()
	if err := c.compile(condition); err != nil {
		return err
	}
	jumpDefaultPos := c.emit(op.PopJumpForwardIfFalse, Placeholder)
	if err := c.compile(forNode.Consequence()); err != nil {
		return err
	}
	c.emit(op.PopTop)
	delta, err := c.calculateDelta(startPos)
	if err != nil {
		return err
	}
	jumpBackPos := c.emit(op.JumpBackward, delta)
	nopPos := c.emit(op.Nop)
	for _, pos := range loop.breakPos {
		delta := nopPos - pos
		if delta > math.MaxUint16 {
			return fmt.Errorf("compile error: loop code size exceeded limits")
		}
		c.changeOperand(pos, uint16(delta))
	}
	for _, pos := range loop.continuePos {
		delta := jumpBackPos - pos
		if delta > math.MaxUint16 {
			return fmt.Errorf("compile error: loop code size exceeded limits")
		}
		c.changeOperand(pos, uint16(delta))
	}
	delta, err = c.calculateDelta(jumpDefaultPos)
	if err != nil {
		return err
	}
	c.changeOperand(jumpDefaultPos, delta)
	return nil
}

func (c *Compiler) compileFor(node *ast.For) error {
	// Simple loop e.g. `for { ... }`
	if node.IsSimpleLoop() {
		return c.compileSimpleFor(node)
	}

	// For-Range loop e.g. `for i, value := range container { ... }`
	if node.Init() == nil && node.Post() == nil {
		cond := node.Condition()
		switch cond := cond.(type) {
		case *ast.Var:
			name, rhs := cond.Value()
			if rangeNode, ok := rhs.(*ast.Range); ok {
				return c.compileForRange(node, []string{name}, rangeNode.Container())
			} else {
				return c.compileForRange(node, []string{name}, rhs)
			}
		case *ast.MultiVar:
			names, rhs := cond.Value()
			if len(names) != 2 {
				return c.formatError("invalid for loop", node.Token().StartPosition)
			}
			if rangeNode, ok := rhs.(*ast.Range); ok {
				return c.compileForRange(node, names, rangeNode.Container())
			} else {
				return c.compileForRange(node, names, rhs)
			}
		case *ast.Range:
			return c.compileForRange(node, nil, cond.Container())
		case ast.Expression:
			return c.compileForCondition(node, cond)
		default:
			return c.formatError("invalid for loop", node.Token().StartPosition)
		}
	}

	// For-Condition loop e.g. `for i := 0; i < 10; i++ { ... }`
	code := c.current
	code.symbols = code.symbols.NewBlock()
	loop := c.startLoop()
	defer func() {
		loop.end()
		code.symbols = code.symbols.parent
	}()

	// Compile the init statement if present
	if node.Init() != nil {
		if err := c.compile(node.Init()); err != nil {
			return err
		}
	}

	// Mark the position of the loop start
	loopStart := c.currentPosition()

	// Compile the condition expression if present
	var conditionJumpPos int
	if node.Condition() != nil {
		if err := c.compile(node.Condition()); err != nil {
			return err
		}
		// Emit a jump to execute if the condition fails
		conditionJumpPos = c.emit(op.PopJumpForwardIfFalse, Placeholder)
	}

	// Compile the loop body
	if err := c.compile(node.Consequence()); err != nil {
		return err
	}
	c.emit(op.PopTop)

	// This is where "continue" statements should jump to so that they pick
	// up the "post" statement if there is one before going back to the beginning.
	continueDst := len(c.current.instructions)

	// Compile the post statement if present
	if node.Post() != nil {
		post := node.Post()
		if err := c.compile(post); err != nil {
			return err
		}
		// If the post statement is an expression, pop the value so its ignored
		if post.IsExpression() {
			c.emit(op.PopTop)
		}
	}

	// Jump back to the loop start
	delta, err := c.calculateDelta(loopStart)
	if err != nil {
		return err
	}
	c.emit(op.JumpBackward, delta)

	// Update the condition jump position
	if conditionJumpPos != 0 {
		delta, err = c.calculateDelta(conditionJumpPos)
		if err != nil {
			return err
		}
		c.changeOperand(conditionJumpPos, delta)
	}

	// Update breaks to jump to this point
	for _, pos := range loop.breakPos {
		delta, err = c.calculateDelta(pos)
		if err != nil {
			return err
		}
		c.changeOperand(pos, uint16(delta))
	}

	// Update continues to jump to the post statement
	for _, pos := range loop.continuePos {
		delta := continueDst - pos
		if delta > math.MaxUint16 {
			return fmt.Errorf("compile error: loop code size exceeded limits")
		}
		c.changeOperand(pos, uint16(delta))
	}
	return nil
}

func (c *Compiler) compileSimpleFor(node *ast.For) error {
	code := c.current
	code.symbols = code.symbols.NewBlock()
	loop := c.startLoop()
	defer func() {
		loop.end()
		code.symbols = code.symbols.parent
	}()
	startPos := c.currentPosition()
	if err := c.compile(node.Consequence()); err != nil {
		return err
	}
	c.emit(op.PopTop)
	delta, err := c.calculateDelta(startPos)
	if err != nil {
		return err
	}
	jumpBackPos := c.emit(op.JumpBackward, delta)
	nopPos := c.emit(op.Nop)
	for _, pos := range loop.breakPos {
		delta := nopPos - pos
		if delta > math.MaxUint16 {
			return fmt.Errorf("compile error: loop code size exceeded limits")
		}
		c.changeOperand(pos, uint16(delta))
	}
	for _, pos := range loop.continuePos {
		delta := jumpBackPos - pos
		if delta > math.MaxUint16 {
			return fmt.Errorf("compile error: loop code size exceeded limits")
		}
		c.changeOperand(pos, uint16(delta))
	}
	return nil
}

func (c *Compiler) compileForIn(node *ast.ForIn) error {
	// Compile the iterable expression
	if err := c.compile(node.Iterable()); err != nil {
		return err
	}
	// Get an iterator for the container at TOS
	c.emit(op.GetIter)

	code := c.current
	code.symbols = code.symbols.NewBlock()
	loop := c.startLoop()
	loop.isRangeLoop = true
	defer func() {
		loop.end()
		code.symbols = code.symbols.parent
	}()

	variables := node.Variables()
	numVars := len(variables)

	// ForIter instruction: pop iterator, advance it, push current values
	// numVars == 1 with value 3: Python-style single variable (gets value only)
	// numVars == 2: gets both key/index and value
	iterMode := uint16(numVars)
	if numVars == 1 {
		iterMode = 3 // Python-style: single variable gets value only
	}
	iterPos := c.emit(op.ForIter, 0, iterMode)

	// Assign iteration values to loop variables
	for _, variable := range variables {
		varName := variable.Literal()
		sym, err := code.symbols.InsertVariable(varName)
		if err != nil {
			return err
		}
		if code.symbols.IsGlobal() {
			c.emit(op.StoreGlobal, sym.Index())
		} else {
			c.emit(op.StoreFast, sym.Index())
		}
	}

	// Compile the body of the loop
	if err := c.compile(node.Consequence()); err != nil {
		return err
	}
	c.emit(op.PopTop)

	// Jump back to the start of the loop
	delta, err := c.calculateDelta(iterPos)
	if err != nil {
		return err
	}
	jumpBackPos := c.emit(op.JumpBackward, delta)

	// Update the ForIter instruction to jump "here" when done
	delta, err = c.calculateDelta(iterPos)
	if err != nil {
		return err
	}
	c.changeOperand(iterPos, delta)

	// Update breaks to jump to this point
	for _, pos := range loop.breakPos {
		delta, err = c.calculateDelta(pos)
		if err != nil {
			return err
		}
		c.changeOperand(pos, uint16(delta))
	}

	// Update continues
	for _, pos := range loop.continuePos {
		delta := jumpBackPos - pos
		if delta > math.MaxUint16 {
			return c.formatError("loop code size exceeded limits", node.Token().StartPosition)
		}
		c.changeOperand(pos, uint16(delta))
	}
	return nil
}

func (c *Compiler) compileIf(node *ast.If) error {
	if err := c.compile(node.Condition()); err != nil {
		return err
	}
	jumpIfFalsePos := c.emit(op.PopJumpForwardIfFalse, Placeholder)
	if err := c.compile(node.Consequence()); err != nil {
		return err
	}
	alternative := node.Alternative()
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
	operator := node.Operator()
	// Short-circuit operators
	if operator == "&&" {
		return c.compileAnd(node)
	} else if operator == "||" {
		return c.compileOr(node)
	} else if operator == "??" {
		return c.compileNullish(node)
	}
	// Non-short-circuit operators
	if err := c.compile(node.Left()); err != nil {
		return err
	}
	if err := c.compile(node.Right()); err != nil {
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
		return c.formatError(fmt.Sprintf("unknown operator %q", node.Operator()), node.Token().StartPosition)
	}
	return nil
}

func (c *Compiler) compileAnd(node *ast.Infix) error {
	// The "&&" AND operator needs to have "short circuit" behavior
	if err := c.compile(node.Left()); err != nil {
		return err
	}
	c.emit(op.Copy, 0) // Duplicate LHS
	jumpPos := c.emit(op.PopJumpForwardIfFalse, Placeholder)
	if err := c.compile(node.Right()); err != nil {
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
	if err := c.compile(node.Left()); err != nil {
		return err
	}
	c.emit(op.Copy, 0) // Duplicate LHS
	jumpPos := c.emit(op.PopJumpForwardIfTrue, Placeholder)
	if err := c.compile(node.Right()); err != nil {
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
	if err := c.compile(node.Left()); err != nil {
		return err
	}
	c.emit(op.Copy, 0) // Duplicate LHS
	jumpPos := c.emit(op.PopJumpForwardIfNotNil, Placeholder)
	c.emit(op.PopTop) // Pop the nil value
	if err := c.compile(node.Right()); err != nil {
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
	args := call.Arguments()
	argc := len(args)
	if argc > MaxArgs {
		return fmt.Errorf("compile error: max args limit of %d exceeded (got %d)", MaxArgs, argc)
	}
	if err := c.compile(call.Function()); err != nil {
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
	if err := c.compile(node.Object()); err != nil {
		return err
	}
	expr := node.Call()
	method, ok := expr.(*ast.Call)
	if !ok {
		return fmt.Errorf("compile error: invalid call expression")
	}
	name := method.Function().String()
	c.emit(op.LoadAttr, c.current.addName(name))
	args := method.Arguments()
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
	return pos
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
	returnNil := ast.NewReturn(token.Token{}, ast.NewNil(token.Token{}))
	statements := node.Statements()
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
	case ast.Expression:
		statements[count-1] = ast.NewReturn(token.Token{}, last)
	default:
		statements = append(statements, returnNil)
	}
	return statements
}

// formatError creates a detailed error message including file, line and column information
func (c *Compiler) formatError(msg string, pos token.Position) error {
	lineCol := fmt.Sprintf("line %d, column %d", pos.LineNumber(), pos.ColumnNumber())
	filename := c.filename
	if filename == "" {
		filename = "unknown"
	}
	var b strings.Builder
	b.WriteString("compile error: ")
	b.WriteString(msg)
	b.WriteString("\n\n")
	b.WriteString("location: ")
	b.WriteString(fmt.Sprintf("%s:%d:%d", filename, pos.LineNumber(), pos.ColumnNumber()))
	b.WriteString(fmt.Sprintf(" (%s)", lineCol))
	return fmt.Errorf("%s", b.String())
}
