package compiler

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"

	"github.com/deepnoodle-ai/risor/v2/pkg/bytecode"
	"github.com/deepnoodle-ai/risor/v2/pkg/errors"
	"github.com/deepnoodle-ai/risor/v2/pkg/op"
)

// MarshalCode converts a Code object into a JSON representation.
func MarshalCode(code *Code) ([]byte, error) {
	cdef, err := stateFromCode(code)
	if err != nil {
		return nil, err
	}
	return json.Marshal(cdef)
}

// UnmarshalCode converts a JSON representation of a Code object into a Code.
// It returns an error if the data is malformed or if the bytecode it holds
// fails the checks of [bytecode.Validate].
func UnmarshalCode(data []byte) (*Code, error) {
	var def state
	if err := json.Unmarshal(data, &def); err != nil {
		return nil, err
	}
	code, err := codeFromState(&def)
	if err != nil {
		return nil, err
	}
	if err := bytecode.Validate(code.ToBytecode()); err != nil {
		return nil, err
	}
	return code, nil
}

// Used to marshal a Function.
type functionDef struct {
	ID         string            `json:"id"`
	Name       string            `json:"name"`
	Parameters []string          `json:"parameters"`
	Defaults   []json.RawMessage `json:"defaults"`
	RestParam  string            `json:"rest_param,omitempty"`
}

type constantDef struct {
	Type string `json:"type"`
}

type boolConstantDef struct {
	Type  string `json:"type"`
	Value bool   `json:"value"`
}

type intConstantDef struct {
	Type  string `json:"type"`
	Value int64  `json:"value"`
}

type floatConstantDef struct {
	Type  string  `json:"type"`
	Value float64 `json:"value"`
}

type stringConstantDef struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}

type functionConstantDef struct {
	Type  string       `json:"type"`
	Value *functionDef `json:"value"`
}

// Used to marshal a Symbol.
type symbolDef struct {
	Name       string `json:"name"`
	Index      uint16 `json:"index"`
	IsConstant bool   `json:"is_constant,omitempty"`
	Value      any    `json:"value,omitempty"`
}

// Used to marshal a Resolution.
type resolutionDef struct {
	Symbol    *symbolDef `json:"symbol"`
	Scope     Scope      `json:"scope"`
	Depth     int        `json:"depth"`
	FreeIndex int        `json:"free_index"`
}

// Used to marshal a SymbolTable. ID is empty for a child table whose ID is
// derived from its parent's ID and its position, as it is for every child
// the compiler creates (see SymbolTable.ID). The ID of such a table holds the
// path from the root table, so writing it for each table would make the
// output grow with the square of the nesting depth. The root holds its ID.
// Older versions wrote the ID of every table. UnmarshalCode keeps them.
type symbolTableDef struct {
	ID            string                `json:"id,omitempty"`
	Symbols       []*symbolDef          `json:"symbols"`
	SymbolsByName map[string]*symbolDef `json:"symbols_by_name"`
	Free          []*resolutionDef      `json:"free,omitempty"`
	IsBlock       bool                  `json:"is_block,omitempty"`
	Children      []*symbolTableDef     `json:"children,omitempty"`
}

// locationDef is used to marshal a SourceLocation. It does not hold the
// source line: UnmarshalCode looks it up from the line number, so a script on
// one line is not written once per instruction. Older versions wrote the
// source line of each location. UnmarshalCode ignores it.
type locationDef struct {
	Filename  string `json:"filename,omitempty"`
	Line      int    `json:"line"`
	Column    int    `json:"column"`
	EndColumn int    `json:"end_column,omitempty"`
}

// exceptionHandlerDef is used to marshal an ExceptionHandler.
type exceptionHandlerDef struct {
	TryStart     int `json:"try_start"`
	TryEnd       int `json:"try_end"`
	CatchStart   int `json:"catch_start"`
	FinallyStart int `json:"finally_start"`
	CatchVarIdx  int `json:"catch_var_idx"`
}

// Flat form of a Code object used in marshaling. It holds the source text of
// a code in Source, or in SourceRange the byte range of its source within the
// root code's source. The compiler records that range for each function body,
// so nested functions do not repeat their body text at each level. Older
// versions wrote the text of every code.
//
// A code refers to its parent by ParentIndex, the parent's index in the code
// list, and to its symbol table by SymbolTableIndex, the table's index in a
// depth-first, pre-order walk of the symbol table tree. ID is empty for a
// child whose ID is derived from its parent's ID and its position, as it is
// for compiled code (see Code.ID). Code and table IDs hold the path from the
// root, so writing them would make the output grow with the square of the
// nesting depth. Older versions wrote every ID, and referred to the parent
// and the symbol table by ID in ParentID and SymbolTableID. UnmarshalCode
// still reads them.
type codeDef struct {
	ID               string            `json:"id,omitempty"`
	Name             string            `json:"name"`
	ParentIndex      *int              `json:"parent_index,omitempty"`
	ParentID         string            `json:"parent_id,omitempty"`
	SymbolTableIndex *int              `json:"symbol_table_index,omitempty"`
	SymbolTableID    string            `json:"symbol_table_id,omitempty"`
	FunctionID       string            `json:"function_id,omitempty"`
	Instructions     []op.Code         `json:"instructions,omitempty"`
	Constants        []json.RawMessage `json:"constants,omitempty"`
	Names            []string          `json:"names,omitempty"`
	Source           string            `json:"source,omitempty"`
	SourceRange      []int             `json:"source_range,omitempty"`
	Filename         string            `json:"filename,omitempty"`
	Locations        []locationDef     `json:"locations,omitempty"`
	MaxCallArgs      uint16            `json:"max_call_args,omitempty"`
	EnvKeys          []string          `json:"env_keys,omitempty"`

	ExceptionHandlers []exceptionHandlerDef `json:"exception_handlers,omitempty"`
}

// A representation of a Code object that can be marshalled more easily.
type state struct {
	Code        []*codeDef      `json:"code"`
	SymbolTable *symbolTableDef `json:"symbol_table"`
}

// Builds a Code object from its marshalled state.
func codeFromState(state *state) (*Code, error) {
	if len(state.Code) == 0 {
		return nil, fmt.Errorf("invalid code: no code blocks")
	}
	if state.SymbolTable == nil {
		return nil, fmt.Errorf("invalid code: missing symbol table")
	}
	if state.SymbolTable.IsBlock {
		return nil, fmt.Errorf("invalid symbol table: root table is a block")
	}
	table, err := symbolTableFromDefinition(state.SymbolTable)
	if err != nil {
		return nil, err
	}
	if err := linkSymbols(table); err != nil {
		return nil, err
	}
	tables := flattenSymbolTables(table, nil)
	// tablesByID maps each stored table ID to its table, for codes that name
	// their table by ID, as older versions did. It is built on first use.
	var tablesByID map[string]*SymbolTable
	codes := make([]*Code, 0, len(state.Code))
	// functionsByID maps a function ID to the function and the code that
	// holds it as a constant. The function's own code must be a child of
	// that code, which is the shape the compiler produces.
	type ownedFunction struct {
		fn    *Function
		owner *Code
	}
	functionsByID := map[string]ownedFunction{}
	// codesByID maps the root's ID and each stored child ID to its code, for
	// codes that name their parent by ID, as older versions did.
	codesByID := map[string]*Code{}
	// lines maps each code to the lines of its line source. Every child's
	// line source is the root source, so children share their parent's lines
	// and the root source is split once rather than once per code.
	lines := map[*Code][]string{}
	for i, c := range state.Code {
		if c == nil {
			return nil, fmt.Errorf("invalid code: code block %d is null", i)
		}
		var codeSymbols *SymbolTable
		if c.SymbolTableIndex == nil {
			if tablesByID == nil {
				tablesByID = symbolTablesByID(tables)
			}
			var found bool
			codeSymbols, found = tablesByID[c.SymbolTableID]
			if !found {
				return nil, fmt.Errorf("symbol table not found: %s", c.SymbolTableID)
			}
		} else {
			if c.SymbolTableID != "" {
				return nil, fmt.Errorf("invalid code: code block %d has both symbol table index and symbol table id", i)
			}
			idx := *c.SymbolTableIndex
			if idx < 0 || idx >= len(tables) {
				return nil, fmt.Errorf("invalid code: code block %d has symbol table index %d, want 0 to %d", i, idx, len(tables)-1)
			}
			codeSymbols = tables[idx]
		}
		// A code uses the table of its own scope: the root table for the
		// root code, and a function's table for a function. Block tables
		// only nest inside those.
		if i == 0 && codeSymbols != table {
			return nil, fmt.Errorf("invalid code: root code does not use the root symbol table")
		}
		if codeSymbols.isBlock {
			return nil, fmt.Errorf("invalid code: code block %d uses a block symbol table", i)
		}
		// The first code block is the root. Every other block names a
		// parent that appears before it, so the blocks form a tree.
		var parent *Code
		if i == 0 {
			if c.ParentIndex != nil {
				return nil, fmt.Errorf("invalid code: root code has a parent index: %d", *c.ParentIndex)
			}
			if c.ParentID != "" {
				return nil, fmt.Errorf("invalid code: root code has a parent: %s", c.ParentID)
			}
		} else if c.ParentIndex != nil {
			if c.ParentID != "" {
				return nil, fmt.Errorf("invalid code: code block %d has both parent index and parent id", i)
			}
			p := *c.ParentIndex
			if p < 0 || p >= i {
				return nil, fmt.Errorf("invalid code: code block %d has parent index %d, want a code block before it", i, p)
			}
			parent = codes[p]
		} else {
			var found bool
			parent, found = codesByID[c.ParentID]
			if !found {
				return nil, fmt.Errorf("parent code not found: %q", c.ParentID)
			}
		}
		// A child without a stored ID derives it from its parent's ID and
		// its position. Stored IDs must be unique, so that a parent named
		// by ID is not ambiguous.
		if i == 0 || c.ID != "" {
			if _, dup := codesByID[c.ID]; dup {
				return nil, fmt.Errorf("invalid code: duplicate code id %q", c.ID)
			}
		}
		constants, err := unmarshalConstants(c.Constants)
		if err != nil {
			return nil, err
		}
		source := c.Source
		var sourceStart, sourceEnd int
		if r := c.SourceRange; r != nil {
			if i == 0 {
				return nil, fmt.Errorf("invalid code: root code has a source range")
			}
			if c.Source != "" {
				return nil, fmt.Errorf("invalid code: code block %d has both source and source range", i)
			}
			if len(r) != 2 {
				return nil, fmt.Errorf("invalid code: code block %d has a source range with %d elements, want 2", i, len(r))
			}
			rootSource := codes[0].source
			if r[0] < 0 || r[0] > r[1] || r[1] > len(rootSource) {
				return nil, fmt.Errorf("invalid code: code block %d has source range %v outside the root source", i, r)
			}
			source = rootSource[r[0]:r[1]]
			sourceStart, sourceEnd = r[0], r[1]
		}

		// Reconstruct rootSource pointer from parent (same logic as newChild)
		var rootSource *string
		if parent != nil {
			if parent.rootSource != nil {
				rootSource = parent.rootSource
			} else {
				rootSource = &parent.source
			}
		}

		nameIndex, err := newNameIndex(c.Names)
		if err != nil {
			return nil, fmt.Errorf("invalid code: code block %d: %w", i, err)
		}

		code := &Code{
			id:           c.ID,
			parent:       parent,
			name:         c.Name,
			isNamed:      parent != nil && c.Name != "", // only the root is unnamed
			functionID:   c.FunctionID,
			symbols:      codeSymbols,
			instructions: CopyInstructions(c.Instructions),
			constants:    constants,
			names:        copyStrings(c.Names),
			nameIndex:    nameIndex,
			source:       source,
			sourceStart:  sourceStart,
			sourceEnd:    sourceEnd,
			filename:     c.Filename,
			rootSource:   rootSource,
			maxCallArgs:  c.MaxCallArgs,
			envKeys:      copyStrings(c.EnvKeys),

			exceptionHandlers: exceptionHandlersFromDefs(c.ExceptionHandlers),
		}
		// Like lineSource, a child uses the root code's source, which is
		// also its parent's line source, so the parent's lines apply.
		if parent != nil {
			lines[code] = lines[parent]
		} else {
			lines[code] = strings.Split(code.source, "\n")
		}
		code.locations = locationsFromDefs(c.Locations, lines[code])
		if i == 0 || c.ID != "" {
			codesByID[c.ID] = code
		}
		codes = append(codes, code)
		if parent != nil {
			code.position = len(parent.children)
			parent.children = append(parent.children, code)
		}
		for _, constant := range constants {
			if fn, ok := constant.(*Function); ok {
				if _, dup := functionsByID[fn.id]; dup {
					return nil, fmt.Errorf("invalid code: duplicate function id %q", fn.id)
				}
				functionsByID[fn.id] = ownedFunction{fn: fn, owner: code}
			}
		}
	}
	for _, c := range codes {
		if c.functionID != "" {
			owned, found := functionsByID[c.functionID]
			if !found {
				return nil, fmt.Errorf("function not found: %s", c.functionID)
			}
			if owned.owner != c.parent || owned.fn.code != nil {
				return nil, fmt.Errorf("invalid code: function %q has an invalid code block", c.functionID)
			}
			owned.fn.code = c
		}
	}
	for id, owned := range functionsByID {
		if owned.fn.code == nil {
			return nil, fmt.Errorf("invalid code: function %q has no code block", id)
		}
	}
	return codes[0], nil
}

// flattenSymbolTables appends table and its descendants to tables in
// depth-first, pre-order: each table comes before its children, and its
// children are in order. A code's symbol_table_index is an index into it.
func flattenSymbolTables(table *SymbolTable, tables []*SymbolTable) []*SymbolTable {
	tables = append(tables, table)
	for _, child := range table.children {
		tables = flattenSymbolTables(child, tables)
	}
	return tables
}

// symbolTablesByID maps the ID of the root, the first of tables, and each
// stored ID of the other tables to the first table that has it, the table
// that FindTable returns. Older versions stored the ID of every table. It
// does not build derived IDs, which would take memory that grows with the
// square of the nesting depth.
func symbolTablesByID(tables []*SymbolTable) map[string]*SymbolTable {
	byID := make(map[string]*SymbolTable, len(tables))
	for i, table := range tables {
		if i > 0 && table.id == "" {
			continue
		}
		if _, ok := byID[table.id]; !ok {
			byID[table.id] = table
		}
	}
	return byID
}

// newNameIndex returns the index map for a names table, or nil if it is empty.
// It returns an error if the table has more names than a uint16 operand can
// index, the same limit that Code.addName enforces.
func newNameIndex(names []string) (map[string]uint16, error) {
	if len(names) == 0 {
		return nil, nil
	}
	if len(names) > math.MaxUint16 {
		return nil, fmt.Errorf("%d attribute names exceeds the limit of %d", len(names), math.MaxUint16)
	}
	index := make(map[string]uint16, len(names))
	for i, name := range names {
		if _, ok := index[name]; !ok {
			index[name] = uint16(i)
		}
	}
	return index, nil
}

func exceptionHandlersFromDefs(defs []exceptionHandlerDef) []*ExceptionHandler {
	if defs == nil {
		return nil
	}
	handlers := make([]*ExceptionHandler, len(defs))
	for i, def := range defs {
		handlers[i] = &ExceptionHandler{
			TryStart:     def.TryStart,
			TryEnd:       def.TryEnd,
			CatchStart:   def.CatchStart,
			FinallyStart: def.FinallyStart,
			CatchVarIdx:  def.CatchVarIdx,
		}
	}
	return handlers
}

func exceptionHandlersToDefs(handlers []*ExceptionHandler) []exceptionHandlerDef {
	if handlers == nil {
		return nil
	}
	defs := make([]exceptionHandlerDef, len(handlers))
	for i, h := range handlers {
		defs[i] = exceptionHandlerDef{
			TryStart:     h.TryStart,
			TryEnd:       h.TryEnd,
			CatchStart:   h.CatchStart,
			FinallyStart: h.FinallyStart,
			CatchVarIdx:  h.CatchVarIdx,
		}
	}
	return defs
}

func unmarshalConstant(constant json.RawMessage) (any, error) {
	var def constantDef
	if err := json.Unmarshal(constant, &def); err != nil {
		return nil, err
	}
	switch def.Type {
	case "nil":
		return nil, nil
	case "nil_default":
		return bytecode.NilDefault{}, nil
	case "bool":
		var def boolConstantDef
		if err := json.Unmarshal(constant, &def); err != nil {
			return nil, err
		}
		return def.Value, nil
	case "int":
		var def intConstantDef
		if err := json.Unmarshal(constant, &def); err != nil {
			return nil, err
		}
		return def.Value, nil
	case "float":
		var def floatConstantDef
		if err := json.Unmarshal(constant, &def); err != nil {
			return nil, err
		}
		return def.Value, nil
	case "string":
		var def stringConstantDef
		if err := json.Unmarshal(constant, &def); err != nil {
			return nil, err
		}
		return def.Value, nil
	case "function":
		var def functionConstantDef
		if err := json.Unmarshal(constant, &def); err != nil {
			return nil, err
		}
		if def.Value == nil {
			return nil, fmt.Errorf("invalid function constant: missing value")
		}
		defaults, err := unmarshalConstants(def.Value.Defaults)
		if err != nil {
			return nil, err
		}
		f := NewFunction(FunctionOpts{
			ID:         def.Value.ID,
			Name:       def.Value.Name,
			Parameters: def.Value.Parameters,
			Defaults:   defaults,
			RestParam:  def.Value.RestParam,
		})
		return f, nil
	default:
		return nil, fmt.Errorf("unknown constant type: %s", def.Type)
	}
}

func unmarshalConstants(constants []json.RawMessage) ([]any, error) {
	if constants == nil {
		return nil, nil
	}
	dst := make([]any, 0, len(constants))
	for _, constant := range constants {
		c, err := unmarshalConstant(constant)
		if err != nil {
			return nil, err
		}
		dst = append(dst, c)
	}
	return dst, nil
}

func marshalConstant(c any) (json.RawMessage, error) {
	switch c := c.(type) {
	case nil:
		return json.Marshal(constantDef{Type: "nil"})
	case bytecode.NilDefault:
		return json.Marshal(constantDef{Type: "nil_default"})
	case bool:
		return json.Marshal(boolConstantDef{Type: "bool", Value: c})
	case int:
		return json.Marshal(intConstantDef{Type: "int", Value: int64(c)})
	case int64:
		return json.Marshal(intConstantDef{Type: "int", Value: c})
	case float32:
		return json.Marshal(floatConstantDef{Type: "float", Value: float64(c)})
	case float64:
		return json.Marshal(floatConstantDef{Type: "float", Value: c})
	case string:
		return json.Marshal(stringConstantDef{Type: "string", Value: c})
	case *Function:
		fn, err := definitionFromFunction(c)
		if err != nil {
			return nil, err
		}
		return json.Marshal(functionConstantDef{Type: "function", Value: fn})
	default:
		return nil, fmt.Errorf("unknown constant type: %T", c)
	}
}

func marshalConstants(constants []any) ([]json.RawMessage, error) {
	if constants == nil {
		return nil, nil
	}
	dst := make([]json.RawMessage, 0, len(constants))
	for _, constant := range constants {
		c, err := marshalConstant(constant)
		if err != nil {
			return nil, err
		}
		dst = append(dst, c)
	}
	return dst, nil
}

// symbolTableFromDefinition builds the symbol table tree that def defines. A
// child table without a stored ID derives it from its parent's ID and its
// position.
func symbolTableFromDefinition(def *symbolTableDef) (*SymbolTable, error) {
	if def == nil {
		return nil, fmt.Errorf("invalid symbol table: null")
	}
	if len(def.Symbols) > math.MaxUint16 {
		return nil, fmt.Errorf("invalid symbol table: too many symbols")
	}
	table := &SymbolTable{
		id:            def.ID,
		symbolsByName: map[string]*Symbol{},
		freeByName:    map[string]*Resolution{},
		isBlock:       def.IsBlock,
		symbols:       []*Symbol{},
	}
	for _, symbol := range def.Symbols {
		obj := symbolFromDefinition(symbol)
		table.symbols = append(table.symbols, obj)
	}
	for name, symbol := range def.SymbolsByName {
		if symbol == nil {
			return nil, fmt.Errorf("invalid symbol table: null symbol %q", name)
		}
		obj := symbolFromDefinition(symbol)
		table.symbolsByName[name] = obj
	}
	for _, resolution := range def.Free {
		if resolution == nil || resolution.Symbol == nil {
			return nil, fmt.Errorf("invalid symbol table: null free variable")
		}
		obj := resolutionFromDefinition(resolution)
		table.free = append(table.free, obj)
		table.freeByName[obj.symbol.name] = obj
	}
	for j, child := range def.Children {
		obj, err := symbolTableFromDefinition(child)
		if err != nil {
			return nil, err
		}
		table.children = append(table.children, obj)
		obj.parent = table
		obj.position = j
	}
	return table, nil
}

// linkSymbols makes each by-name entry of the table and its children the same
// *Symbol as the indexed entry it names. Marshaling stores the two separately,
// and code such as Code.GlobalNames compares them by pointer.
//
// It returns an error if the two disagree: an indexed symbol whose index is
// not its position, or a by-name entry that does not name an indexed symbol
// of the same name. A block's by-name entries name symbols of the enclosing
// function's table, which holds the block's slots.
func linkSymbols(table *SymbolTable) error {
	for i, sym := range table.symbols {
		if sym != nil && int(sym.index) != i {
			return fmt.Errorf("invalid symbol table: symbol %q at position %d has index %d",
				sym.name, i, sym.index)
		}
	}
	local := table.LocalTable()
	for name, sym := range table.symbolsByName {
		var indexed *Symbol
		if int(sym.index) < len(local.symbols) {
			indexed = local.symbols[sym.index]
		}
		if indexed == nil || indexed.name != name || sym.name != name {
			return fmt.Errorf("invalid symbol table: symbol %q does not match the symbol at index %d",
				name, sym.index)
		}
		table.symbolsByName[name] = indexed
	}
	for _, child := range table.children {
		if err := linkSymbols(child); err != nil {
			return err
		}
	}
	return nil
}

// symbolFromDefinition returns nil for a nil def, the form of an unused slot.
func symbolFromDefinition(def *symbolDef) *Symbol {
	if def == nil {
		return nil
	}
	return &Symbol{
		name:       def.Name,
		index:      def.Index,
		isConstant: def.IsConstant,
		value:      def.Value,
	}
}

func resolutionFromDefinition(def *resolutionDef) *Resolution {
	return &Resolution{
		symbol:    symbolFromDefinition(def.Symbol),
		scope:     def.Scope,
		depth:     def.Depth,
		freeIndex: def.FreeIndex,
	}
}

func stateFromCode(code *Code) (*state, error) {
	state := &state{
		Code:        []*codeDef{},
		SymbolTable: definitionFromSymbolTable(code.symbols),
	}
	tableIndices := map[*SymbolTable]int{}
	for i, table := range flattenSymbolTables(code.symbols, nil) {
		tableIndices[table] = i
	}
	allCode := code.Flatten()
	codeIndices := make(map[*Code]int, len(allCode))
	for i, code := range allCode {
		codeIndices[code] = i
	}
	// parentIndices holds the index of each code's parent. Flatten puts a
	// parent before its children, so each is set before it is read.
	parentIndices := make([]int, len(allCode))
	for i, code := range allCode {
		for _, child := range code.children {
			parentIndices[codeIndices[child]] = i
		}
		source := code.source
		var sourceRange []int
		if i > 0 && rootSourceRange(code, allCode[0]) {
			source, sourceRange = "", []int{code.sourceStart, code.sourceEnd}
		}
		constants, err := marshalConstants(code.constants)
		if err != nil {
			return nil, err
		}
		cdef := &codeDef{
			Constants:    constants,
			FunctionID:   code.functionID,
			Instructions: CopyInstructions(code.instructions),
			Name:         code.name,
			Names:        copyStrings(code.names),
			Source:       source,
			SourceRange:  sourceRange,
			Filename:     code.filename,
			Locations:    locationsToDefs(code.locations),
			MaxCallArgs:  code.maxCallArgs,
			EnvKeys:      copyStrings(code.envKeys),

			ExceptionHandlers: exceptionHandlersToDefs(code.exceptionHandlers),
		}
		// The first code becomes the root when unmarshaled, so it holds its
		// full ID. Every other code holds only a stored ID.
		if i == 0 {
			cdef.ID = code.ID()
		} else {
			cdef.ID = code.id
		}
		if i > 0 {
			parentIdx := parentIndices[i]
			cdef.ParentIndex = &parentIdx
		}
		// A code's table is in its root code's table tree, as the compiler
		// builds it. If it is not, write the table's ID as older versions
		// did, and UnmarshalCode reports that the table is not found.
		if idx, ok := tableIndices[code.symbols]; ok {
			cdef.SymbolTableIndex = &idx
		} else {
			cdef.SymbolTableID = code.symbols.ID()
		}
		state.Code = append(state.Code, cdef)
	}
	return state, nil
}

// rootSourceRange reports whether c has a source range that MarshalCode can
// write in place of its source text: root is the root code, the range is
// within its source, and the range gives c's source. The comparison is quick
// when c's source shares the root's memory, as it does for compiled code.
func rootSourceRange(c, root *Code) bool {
	if root.parent != nil || c.sourceEnd <= c.sourceStart {
		return false
	}
	if c.sourceStart < 0 || c.sourceEnd > len(root.source) {
		return false
	}
	return root.source[c.sourceStart:c.sourceEnd] == c.source
}

// definitionFromSymbolTable returns the definition of table and its children.
// table becomes the root when unmarshaled, so it holds its full ID. Each
// child holds only a stored ID.
func definitionFromSymbolTable(table *SymbolTable) *symbolTableDef {
	def := symbolTableDefinition(table)
	def.ID = table.ID()
	return def
}

// symbolTableDefinition returns the definition of table and its children,
// with the stored ID of each.
func symbolTableDefinition(table *SymbolTable) *symbolTableDef {
	free := make([]*resolutionDef, 0, len(table.free))
	for _, resolution := range table.free {
		free = append(free, definitionFromResolution(resolution))
	}
	symbols := make([]*symbolDef, 0, len(table.symbols))
	for _, symbol := range table.symbols {
		symbols = append(symbols, definitionFromSymbol(symbol))
	}
	symbolsByName := map[string]*symbolDef{}
	for _, symbol := range table.symbolsByName {
		symbolsByName[symbol.name] = definitionFromSymbol(symbol)
	}
	children := make([]*symbolTableDef, 0, len(table.children))
	for _, child := range table.children {
		children = append(children, symbolTableDefinition(child))
	}
	return &symbolTableDef{
		ID:            table.id,
		Symbols:       symbols,
		SymbolsByName: symbolsByName,
		Free:          free,
		IsBlock:       table.isBlock,
		Children:      children,
	}
}

// definitionFromSymbol returns nil for a nil symbol, the form of an unused slot.
func definitionFromSymbol(symbol *Symbol) *symbolDef {
	if symbol == nil {
		return nil
	}
	return &symbolDef{
		Name:       symbol.name,
		Index:      symbol.index,
		IsConstant: symbol.isConstant,
		Value:      symbol.value,
	}
}

func definitionFromFunction(function *Function) (*functionDef, error) {
	defaults, err := marshalConstants(function.defaults)
	if err != nil {
		return nil, err
	}
	return &functionDef{
		ID:         function.id,
		Name:       function.name,
		Parameters: copyStrings(function.parameters),
		Defaults:   defaults,
		RestParam:  function.restParam,
	}, nil
}

func definitionFromResolution(resolution *Resolution) *resolutionDef {
	return &resolutionDef{
		Symbol:    definitionFromSymbol(resolution.symbol),
		Scope:     resolution.scope,
		Depth:     resolution.depth,
		FreeIndex: resolution.freeIndex,
	}
}

func copyStrings(src []string) []string {
	if src == nil {
		return nil
	}
	dst := make([]string, len(src))
	copy(dst, src)
	return dst
}

// locationsFromDefs converts location definitions, taking each source line
// from lines, the lines of the code's line source.
func locationsFromDefs(defs []locationDef, lines []string) []errors.SourceLocation {
	if defs == nil {
		return nil
	}
	locs := make([]errors.SourceLocation, len(defs))
	for i, def := range defs {
		locs[i] = errors.SourceLocation{
			Filename:  def.Filename,
			Line:      def.Line,
			Column:    def.Column,
			EndColumn: def.EndColumn,
			Source:    sourceLine(lines, def.Line),
		}
	}
	return locs
}

func locationsToDefs(locs []errors.SourceLocation) []locationDef {
	if locs == nil {
		return nil
	}
	defs := make([]locationDef, len(locs))
	for i, loc := range locs {
		defs[i] = locationDef{
			Filename:  loc.Filename,
			Line:      loc.Line,
			Column:    loc.Column,
			EndColumn: loc.EndColumn,
		}
	}
	return defs
}

func CopyInstructions(src []op.Code) []op.Code {
	dst := make([]op.Code, len(src))
	copy(dst, src)
	return dst
}
