package compiler

import (
	"errors"
	"fmt"
	"math"
)

// BlankIdentifier is the special identifier that discards values.
// It cannot be read, only assigned to.
const BlankIdentifier = "_"

// IsBlankIdentifier returns true if name is the blank identifier "_".
func IsBlankIdentifier(name string) bool {
	return name == BlankIdentifier
}

// SymbolTable tracks which symbols are defined and referenced in a given scope.
// These tables may have a parent table, which indicates that they represent a
// nested scope. If "isBlock" is set to true, this table represents a block
// within a function (like inside an if { ... }), rather than a function itself.
// Note there may be more symbols in the symbols array than there are in
// symbolsByName, because symbols defined in nested blocks don't use a name
// in the enclosing table.
type SymbolTable struct {
	id            string
	parent        *SymbolTable
	children      []*SymbolTable
	symbolsByName map[string]*Symbol
	freeByName    map[string]*Resolution
	symbols       []*Symbol
	free          []*Resolution
	isBlock       bool
}

// NewChild creates a new symbol table that is a child of the current table.
func (t *SymbolTable) NewChild() *SymbolTable {
	child := &SymbolTable{
		id:            fmt.Sprintf("%s.%d", t.ID(), len(t.children)),
		parent:        t,
		symbolsByName: map[string]*Symbol{},
		freeByName:    map[string]*Resolution{},
		symbols:       []*Symbol{},
		isBlock:       false,
	}
	t.children = append(t.children, child)
	return child
}

// NewBlock creates a new symbol table that is a child of the current table,
// and represents a block within a function. Blocks allocate symbol indexes
// from the enclosing function's symbol table.
func (t *SymbolTable) NewBlock() *SymbolTable {
	child := t.NewChild()
	child.isBlock = true
	return child
}

func (t *SymbolTable) ID() string {
	return t.id
}

func (t *SymbolTable) claimIndex(s *Symbol) (uint16, error) {
	if t.isBlock {
		return t.parent.claimIndex(s)
	}
	idx := len(t.symbols)
	if idx >= math.MaxUint16 {
		return 0, errors.New("compile error: too many symbols")
	}
	uidx := uint16(idx)
	t.symbols = append(t.symbols, s)
	s.index = uidx
	return uidx, nil
}

// ClaimSlot reserves an index slot without associating it with a name.
// This is used for blank identifier "_" parameters in functions, where
// we need to reserve space for the argument but don't need to look it up.
func (t *SymbolTable) ClaimSlot() (uint16, error) {
	if t.isBlock {
		return t.parent.ClaimSlot()
	}
	idx := len(t.symbols)
	if idx >= math.MaxUint16 {
		return 0, errors.New("compile error: too many symbols")
	}
	uidx := uint16(idx)
	// Append a nil placeholder - this slot exists but has no symbol
	t.symbols = append(t.symbols, nil)
	return uidx, nil
}

func (t *SymbolTable) GetFunction() (*SymbolTable, bool) {
	if t.parent == nil {
		return nil, false // global scope
	} else if t.isBlock {
		return t.parent.GetFunction()
	}
	return t, true
}

func (t *SymbolTable) GetFunctionID() (string, bool) {
	if t.parent == nil {
		return "", false
	} else if t.isBlock {
		return t.parent.GetFunctionID()
	}
	return t.ID(), true
}

func (t *SymbolTable) FunctionDepth() int {
	if t.parent == nil {
		return 0
	}
	if t.isBlock {
		return t.parent.FunctionDepth()
	}
	return 1 + t.parent.FunctionDepth()
}

// InsertVariable adds a new variable into this symbol table, with an optional value.
// The symbol will be assigned the next available index.
//
// For the blank identifier "_", this returns nil without error. The blank identifier
// is never actually stored - it's used to discard values.
func (t *SymbolTable) InsertVariable(name string, value ...any) (*Symbol, error) {
	// The blank identifier is never stored - return nil to indicate "discard"
	if IsBlankIdentifier(name) {
		return nil, nil
	}
	if _, ok := t.symbolsByName[name]; ok {
		return nil, fmt.Errorf("compile error: variable %q already exists", name)
	}
	var obj any
	valueCount := len(value)
	if valueCount > 1 {
		return nil, errors.New("compile error: expected at most one value")
	} else if valueCount == 1 {
		obj = value[0]
	}
	s := &Symbol{name: name, value: obj}
	if _, err := t.claimIndex(s); err != nil {
		return nil, err
	}
	t.symbolsByName[name] = s
	return s, nil
}

// InsertConstant adds a new constant into this symbol table, with an optional value.
// The symbol will be assigned the next available index.
//
// For the blank identifier "_", this returns nil without error (same as InsertVariable).
func (t *SymbolTable) InsertConstant(name string, value ...any) (*Symbol, error) {
	sym, err := t.InsertVariable(name, value...)
	if err != nil {
		return nil, err
	}
	// Blank identifier returns nil from InsertVariable
	if sym == nil {
		return nil, nil
	}
	sym.isConstant = true
	return sym, nil
}

// SetValue associates a value with the specified symbol.
func (t *SymbolTable) SetValue(name string, value any) error {
	s, ok := t.symbolsByName[name]
	if !ok {
		return fmt.Errorf("compile error: variable %q not found", name)
	}
	s.value = value
	return nil
}

// IsDefined returns true if the specified symbol is defined in this table.
// Does not check any parent tables.
func (t *SymbolTable) IsDefined(name string) bool {
	_, ok := t.symbolsByName[name]
	return ok
}

// Get returns the symbol with the specified name and a boolean indicating
// whether the symbol was found. Does not check any parent tables.
func (t *SymbolTable) Get(name string) (*Symbol, bool) {
	s, ok := t.symbolsByName[name]
	return s, ok
}

// IsGlobal returns true if this table represents the top-level scope.
// In other words, this checks if the table has no parent.
func (t *SymbolTable) IsGlobal() bool {
	if t.parent == nil {
		return true
	}
	if t.isBlock {
		return t.parent.IsGlobal()
	}
	return false
}

// Resolve the specified symbol in this table or any parent tables, returning
// a Resolution if the symbol is found. The Resolution indicates the symbol's
// relative scope and depth. If the symbol is found to be a "free" variable,
// it will be added to the free map for this table.
//
// Note: The blank identifier "_" always returns (nil, false) because it cannot
// be read. Callers should use IsBlankIdentifier to provide a better error message.
func (t *SymbolTable) Resolve(name string) (*Resolution, bool) {
	// The blank identifier cannot be read
	if IsBlankIdentifier(name) {
		return nil, false
	}
	// Access the enclosing function, if any
	activeFunc, inFunc := t.GetFunction()
	var activeFuncID string
	if activeFunc != nil {
		activeFuncID = activeFunc.ID()
	}
	// Check if the symbol is defined directly in this table
	if s, ok := t.symbolsByName[name]; ok {
		var scope Scope
		if t.IsGlobal() {
			scope = Global
		} else {
			scope = Local
		}
		return &Resolution{symbol: s, scope: scope}, true
	}
	// Check if the symbol was previously found to be a "free" variable
	if rs, ok := t.freeByName[name]; ok {
		return rs, true
	}
	// At this point, if there is no parent then the symbol is undefined
	if t.parent == nil {
		return nil, false
	}
	// Search ancestors for the symbol
	ancestor := t
	for {
		ancestor = ancestor.parent
		if ancestor == nil {
			// Symbol is undefined in all ancestors
			return nil, false
		}
		ancestorFuncID, _ := ancestor.GetFunctionID()
		if sym, ok := ancestor.symbolsByName[name]; ok {
			if ancestor.IsGlobal() {
				// Global variable
				return &Resolution{symbol: sym, scope: Global}, true
			}
			if inFunc && ancestorFuncID == activeFuncID {
				// Local variable
				return &Resolution{symbol: sym, scope: Local}, true
			}
			// Free variable
			depth := t.FunctionDepth() - ancestor.FunctionDepth()
			freeIndex := len(activeFunc.free)
			rs := &Resolution{symbol: sym, scope: Free, depth: depth, freeIndex: freeIndex}
			activeFunc.freeByName[name] = rs
			activeFunc.free = append(activeFunc.free, rs)
			return rs, true
		}
	}
}

// Parent returns the parent table of this table, if any.
func (t *SymbolTable) Parent() *SymbolTable {
	return t.parent
}

// Root returns the outermost table that encloses this table.
func (t *SymbolTable) Root() *SymbolTable {
	current := t
	for current.parent != nil {
		current = current.parent
	}
	return current
}

// LocalTable returns the table that defines the local variables for this table.
// This is useful to find the enclosing function when in a block.
func (t *SymbolTable) LocalTable() *SymbolTable {
	current := t
	for current.isBlock {
		current = current.parent
	}
	return current
}

// Count returns the number of symbols defined in this table.
func (t *SymbolTable) Count() uint16 {
	return uint16(len(t.symbols))
}

// Symbol returns the Symbol located at the specified index.
func (t *SymbolTable) Symbol(index uint16) *Symbol {
	return t.symbols[index]
}

// FreeCount returns the number of free variables defined in this table.
func (t *SymbolTable) FreeCount() uint16 {
	return uint16(len(t.free))
}

// Free returns the free variable Resolution located at the specified index.
func (t *SymbolTable) Free(index uint16) *Resolution {
	return t.free[index]
}

// FindTable returns the table with the specified ID. This may be the
// current table or any child table.
func (t *SymbolTable) FindTable(id string) (*SymbolTable, bool) {
	if t.id == id {
		return t, true
	}
	for _, child := range t.children {
		if child, ok := child.FindTable(id); ok {
			return child, true
		}
	}
	return nil, false
}

// AllNames returns all symbol names defined in this table and its ancestors.
// This is useful for generating "Did you mean?" suggestions.
func (t *SymbolTable) AllNames() []string {
	seen := make(map[string]bool)
	var names []string

	// Walk up the symbol table hierarchy
	current := t
	for current != nil {
		for name := range current.symbolsByName {
			if !seen[name] {
				seen[name] = true
				names = append(names, name)
			}
		}
		current = current.parent
	}
	return names
}

// symbolTableSnapshot captures the state of a SymbolTable for rollback.
type symbolTableSnapshot struct {
	symbolLen  int
	childLen   int
	knownNames map[string]struct{}
}

// snapshot captures the current state of the symbol table so it can be
// restored later if a compilation attempt fails.
func (t *SymbolTable) snapshot() symbolTableSnapshot {
	names := make(map[string]struct{}, len(t.symbolsByName))
	for k := range t.symbolsByName {
		names[k] = struct{}{}
	}
	return symbolTableSnapshot{
		symbolLen:  len(t.symbols),
		childLen:   len(t.children),
		knownNames: names,
	}
}

// restore reverts the symbol table to a previous snapshot, removing any
// symbols or children added after the snapshot was taken.
func (t *SymbolTable) restore(s symbolTableSnapshot) {
	t.symbols = t.symbols[:s.symbolLen]
	t.children = t.children[:s.childLen]
	for name := range t.symbolsByName {
		if _, ok := s.knownNames[name]; !ok {
			delete(t.symbolsByName, name)
		}
	}
}

// NewSymbolTable returns a new root symbol table.
func NewSymbolTable() *SymbolTable {
	return &SymbolTable{
		id:            "root",
		symbolsByName: map[string]*Symbol{},
		freeByName:    map[string]*Resolution{},
		symbols:       []*Symbol{},
	}
}
