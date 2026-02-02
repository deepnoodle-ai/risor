package compiler

import (
	"encoding/json"
	"fmt"

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
func UnmarshalCode(data []byte) (*Code, error) {
	var def state
	if err := json.Unmarshal(data, &def); err != nil {
		return nil, err
	}
	return codeFromState(&def)
}

// Used to marshal a Function.
type functionDef struct {
	ID         string            `json:"id"`
	Name       string            `json:"name"`
	Parameters []string          `json:"parameters"`
	Defaults   []json.RawMessage `json:"defaults"`
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

// Used to marshal a SymbolTable.
type symbolTableDef struct {
	ID            string                `json:"id,omitempty"`
	Symbols       []*symbolDef          `json:"symbols"`
	SymbolsByName map[string]*symbolDef `json:"symbols_by_name"`
	Free          []*resolutionDef      `json:"free,omitempty"`
	IsBlock       bool                  `json:"is_block,omitempty"`
	Children      []*symbolTableDef     `json:"children,omitempty"`
}

// locationDef is used to marshal a SourceLocation.
type locationDef struct {
	Filename  string `json:"filename,omitempty"`
	Line      int    `json:"line"`
	Column    int    `json:"column"`
	EndColumn int    `json:"end_column,omitempty"`
	Source    string `json:"source,omitempty"`
}

// Flat form of a Code object used in marshaling.
type codeDef struct {
	ID            string            `json:"id,omitempty"`
	Name          string            `json:"name"`
	ParentID      string            `json:"parent_id,omitempty"`
	SymbolTableID string            `json:"symbol_table_id"`
	FunctionID    string            `json:"function_id,omitempty"`
	Instructions  []op.Code         `json:"instructions,omitempty"`
	Constants     []json.RawMessage `json:"constants,omitempty"`
	Names         []string          `json:"names,omitempty"`
	Source        string            `json:"source,omitempty"`
	Locations     []locationDef     `json:"locations,omitempty"`
	MaxCallArgs   uint16            `json:"max_call_args,omitempty"`
	EnvKeys       []string          `json:"env_keys,omitempty"`
}

// A representation of a Code object that can be marshalled more easily.
type state struct {
	Code        []*codeDef      `json:"code"`
	SymbolTable *symbolTableDef `json:"symbol_table"`
}

// Builds a Code object from its marshalled state.
func codeFromState(state *state) (*Code, error) {
	table, err := symbolTableFromDefinition(state.SymbolTable)
	if err != nil {
		return nil, err
	}
	codes := make([]*Code, 0, len(state.Code))
	functionsByID := map[string]*Function{}
	codesByID := map[string]*Code{}
	for _, c := range state.Code {
		codeSymbols, found := table.FindTable(c.SymbolTableID)
		if !found {
			return nil, fmt.Errorf("symbol table not found: %s", c.SymbolTableID)
		}
		parent, found := codesByID[c.ParentID]
		if !found && c.ParentID != "" {
			return nil, fmt.Errorf("parent code not found: %s", c.ParentID)
		}
		constants, err := unmarshalConstants(c.Constants)
		if err != nil {
			return nil, err
		}

		// Reconstruct rootSource pointer from parent (same logic as newChild)
		var rootSource *string
		if parent != nil {
			if parent.rootSource != nil {
				rootSource = parent.rootSource
			} else if parent.source != "" {
				rootSource = &parent.source
			}
		}

		code := &Code{
			id:           c.ID,
			parent:       parent,
			name:         c.Name,
			isNamed:      c.Name != "" && c.Name != "__main__",
			functionID:   c.FunctionID,
			symbols:      codeSymbols,
			instructions: CopyInstructions(c.Instructions),
			constants:    constants,
			names:        copyStrings(c.Names),
			source:       c.Source,
			rootSource:   rootSource,
			locations:    locationsFromDefs(c.Locations),
			maxCallArgs:  c.MaxCallArgs,
			envKeys:      copyStrings(c.EnvKeys),
		}
		codesByID[code.id] = code
		codes = append(codes, code)
		if parent != nil {
			parent.children = append(parent.children, code)
		}
		for _, constant := range constants {
			if fn, ok := constant.(*Function); ok {
				functionsByID[fn.id] = fn
			}
		}
	}
	for _, c := range codes {
		if c.functionID != "" {
			fn, found := functionsByID[c.functionID]
			if !found {
				return nil, fmt.Errorf("function not found: %s", c.functionID)
			}
			fn.code = c
		}
	}
	return codes[0], nil
}

func unmarshalConstant(constant json.RawMessage) (any, error) {
	var def constantDef
	if err := json.Unmarshal(constant, &def); err != nil {
		return nil, err
	}
	switch def.Type {
	case "nil":
		return nil, nil
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
		defaults, err := unmarshalConstants(def.Value.Defaults)
		if err != nil {
			return nil, err
		}
		f := NewFunction(FunctionOpts{
			ID:         def.Value.ID,
			Name:       def.Value.Name,
			Parameters: def.Value.Parameters,
			Defaults:   defaults,
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

func symbolTableFromDefinition(def *symbolTableDef) (*SymbolTable, error) {
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
		obj := symbolFromDefinition(symbol)
		table.symbolsByName[name] = obj
	}
	for _, resolution := range def.Free {
		obj := resolutionFromDefinition(resolution)
		table.free = append(table.free, obj)
		table.freeByName[obj.symbol.name] = obj
	}
	for _, child := range def.Children {
		obj, err := symbolTableFromDefinition(child)
		if err != nil {
			return nil, err
		}
		table.children = append(table.children, obj)
		obj.parent = table
	}
	return table, nil
}

func symbolFromDefinition(def *symbolDef) *Symbol {
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
	allCode := code.Flatten()
	for _, code := range allCode {
		constants, err := marshalConstants(code.constants)
		if err != nil {
			return nil, err
		}
		cdef := &codeDef{
			ID:            code.id,
			Constants:     constants,
			FunctionID:    code.functionID,
			SymbolTableID: code.symbols.ID(),
			Instructions:  CopyInstructions(code.instructions),
			Name:          code.name,
			Names:         copyStrings(code.names),
			Source:        code.source,
			Locations:     locationsToDefs(code.locations),
			MaxCallArgs:   code.maxCallArgs,
			EnvKeys:       copyStrings(code.envKeys),
		}
		if code.parent != nil {
			cdef.ParentID = code.parent.id
		}
		state.Code = append(state.Code, cdef)
	}
	return state, nil
}

func definitionFromSymbolTable(table *SymbolTable) *symbolTableDef {
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
		children = append(children, definitionFromSymbolTable(child))
	}
	return &symbolTableDef{
		ID:            table.ID(),
		Symbols:       symbols,
		SymbolsByName: symbolsByName,
		Free:          free,
		IsBlock:       table.isBlock,
		Children:      children,
	}
}

func definitionFromSymbol(symbol *Symbol) *symbolDef {
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

func locationsFromDefs(defs []locationDef) []errors.SourceLocation {
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
			Source:    def.Source,
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
			Source:    loc.Source,
		}
	}
	return defs
}

func CopyInstructions(src []op.Code) []op.Code {
	dst := make([]op.Code, len(src))
	copy(dst, src)
	return dst
}
