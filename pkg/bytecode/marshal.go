package bytecode

import (
	"encoding/json"
	"fmt"

	"github.com/deepnoodle-ai/risor/v2/pkg/op"
)

// Marshal converts a Code object into a JSON representation.
func Marshal(code *Code) ([]byte, error) {
	state, err := stateFromCode(code)
	if err != nil {
		return nil, err
	}
	return json.Marshal(state)
}

// Unmarshal converts a JSON representation into a Code object. It returns an
// error if the data is malformed or if the code fails the checks of
// [Validate]. See Validate for what these checks do not cover.
func Unmarshal(data []byte) (*Code, error) {
	var state codeState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, err
	}
	code, err := codeFromState(&state)
	if err != nil {
		return nil, err
	}
	if err := Validate(code); err != nil {
		return nil, err
	}
	return code, nil
}

// Serialization types

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
	Type  string      `json:"type"`
	Value functionDef `json:"value"`
}

type functionDef struct {
	ID         string            `json:"id"`
	Name       string            `json:"name"`
	Parameters []string          `json:"parameters"`
	Defaults   []json.RawMessage `json:"defaults"`
	RestParam  string            `json:"rest_param,omitempty"`
	CodeIndex  int               `json:"code_index"` // Index into codes array
}

type locationDef struct {
	Filename  string `json:"filename,omitempty"`
	Line      int    `json:"line"`
	Column    int    `json:"column"`
	EndColumn int    `json:"end_column,omitempty"`
	Source    string `json:"source,omitempty"`
}

type exceptionHandlerDef struct {
	TryStart     int `json:"try_start"`
	TryEnd       int `json:"try_end"`
	CatchStart   int `json:"catch_start"`
	FinallyStart int `json:"finally_start"`
	CatchVarIdx  int `json:"catch_var_idx"`
}

type codeDef struct {
	ID                string                `json:"id"`
	Name              string                `json:"name"`
	IsNamed           bool                  `json:"is_named,omitempty"`
	ChildIndices      []int                 `json:"child_indices,omitempty"` // Indices of children in codes array
	FunctionID        string                `json:"function_id,omitempty"`
	Instructions      []op.Code             `json:"instructions"`
	Constants         []json.RawMessage     `json:"constants"`
	Names             []string              `json:"names"`
	Source            string                `json:"source,omitempty"`
	Filename          string                `json:"filename,omitempty"`
	Locations         []locationDef         `json:"locations,omitempty"`
	MaxCallArgs       int                   `json:"max_call_args"`
	LocalCount        int                   `json:"local_count"`
	GlobalCount       int                   `json:"global_count"`
	GlobalNames       []string              `json:"global_names,omitempty"`
	LocalNames        []string              `json:"local_names,omitempty"`
	EnvKeys           []string              `json:"env_keys,omitempty"`
	ExceptionHandlers []exceptionHandlerDef `json:"exception_handlers,omitempty"`
}

type codeState struct {
	Codes []*codeDef `json:"codes"`
}

func stateFromCode(code *Code) (*codeState, error) {
	allCodes := code.Flatten()
	codeIndexMap := make(map[*Code]int)
	for i, c := range allCodes {
		codeIndexMap[c] = i
	}

	state := &codeState{
		Codes: make([]*codeDef, len(allCodes)),
	}

	for i, c := range allCodes {
		constants, err := marshalConstants(c, codeIndexMap)
		if err != nil {
			return nil, err
		}

		handlers := make([]exceptionHandlerDef, c.ExceptionHandlerCount())
		for j := 0; j < c.ExceptionHandlerCount(); j++ {
			h := c.ExceptionHandlerAt(j)
			handlers[j] = exceptionHandlerDef{
				TryStart:     h.TryStart,
				TryEnd:       h.TryEnd,
				CatchStart:   h.CatchStart,
				FinallyStart: h.FinallyStart,
				CatchVarIdx:  h.CatchVarIdx,
			}
		}

		locations := make([]locationDef, c.LocationCount())
		filename := c.Filename()
		for j := 0; j < c.LocationCount(); j++ {
			loc := c.LocationAt(j)
			locations[j] = locationDef{
				Filename:  filename,
				Line:      loc.Line,
				Column:    loc.Column,
				EndColumn: loc.EndColumn,
				Source:    c.GetSourceLine(loc.Line),
			}
		}

		names := make([]string, c.NameCount())
		for j := 0; j < c.NameCount(); j++ {
			names[j] = c.NameAt(j)
		}

		// Global names are written once, on the first code, which becomes
		// the root when unmarshaled. Every other code reads the root's names.
		var globalNames []string
		if i == 0 {
			globalNames = copyStrings(c.Root().globalNames)
		}

		localNames := make([]string, c.LocalNameCount())
		for j := 0; j < c.LocalNameCount(); j++ {
			localNames[j] = c.LocalNameAt(j)
		}

		instructions := make([]op.Code, c.InstructionCount())
		for j := 0; j < c.InstructionCount(); j++ {
			instructions[j] = c.InstructionAt(j)
		}

		// Build child indices
		var childIndices []int
		for j := 0; j < c.ChildCount(); j++ {
			child := c.ChildAt(j)
			childIndices = append(childIndices, codeIndexMap[child])
		}

		state.Codes[i] = &codeDef{
			ID:                c.ID(),
			Name:              c.Name(),
			IsNamed:           c.IsNamed(),
			ChildIndices:      childIndices,
			FunctionID:        c.FunctionID(),
			Instructions:      instructions,
			Constants:         constants,
			Names:             names,
			Source:            c.Source(),
			Filename:          c.Filename(),
			Locations:         locations,
			MaxCallArgs:       c.MaxCallArgs(),
			LocalCount:        c.LocalCount(),
			GlobalCount:       c.GlobalCount(),
			GlobalNames:       globalNames,
			LocalNames:        localNames,
			EnvKeys:           c.EnvKeys(),
			ExceptionHandlers: handlers,
		}
	}

	return state, nil
}

func codeFromState(state *codeState) (*Code, error) {
	if err := checkCodeTree(state.Codes); err != nil {
		return nil, err
	}

	// Build bottom-up: process codes in reverse order so children are built before parents.
	// In the flattened representation, children always come after their parent,
	// so reversing ensures we build children first.
	codes := make([]*Code, len(state.Codes))

	// Process in reverse order (children before parents)
	for i := len(state.Codes) - 1; i >= 0; i-- {
		def := state.Codes[i]

		handlers := make([]ExceptionHandler, len(def.ExceptionHandlers))
		for j, h := range def.ExceptionHandlers {
			handlers[j] = ExceptionHandler{
				TryStart:     h.TryStart,
				TryEnd:       h.TryEnd,
				CatchStart:   h.CatchStart,
				FinallyStart: h.FinallyStart,
				CatchVarIdx:  h.CatchVarIdx,
			}
		}

		locations := make([]SourceLocation, len(def.Locations))
		for j, loc := range def.Locations {
			locations[j] = SourceLocation{
				Line:      loc.Line,
				Column:    loc.Column,
				EndColumn: loc.EndColumn,
			}
		}

		// Collect already-built children using stored child indices
		var children []*Code
		if len(def.ChildIndices) > 0 {
			children = make([]*Code, len(def.ChildIndices))
			for j, childIdx := range def.ChildIndices {
				children[j] = codes[childIdx]
			}
		}

		// Unmarshal constants - at this point, any codes referenced by function
		// constants have already been built (they're children of this code)
		constants, err := unmarshalConstantsImmutable(def.Constants, codes)
		if err != nil {
			return nil, err
		}

		// Only the root uses global names. Names on other code, as written
		// by older versions, are ignored.
		var globalNames []string
		if i == 0 {
			globalNames = def.GlobalNames
		}

		codes[i] = NewCode(CodeParams{
			ID:                def.ID,
			Name:              def.Name,
			IsNamed:           def.IsNamed,
			Children:          children,
			FunctionID:        def.FunctionID,
			Instructions:      def.Instructions,
			Constants:         constants,
			Names:             def.Names,
			Source:            def.Source,
			Filename:          def.Filename,
			Locations:         locations,
			MaxCallArgs:       def.MaxCallArgs,
			LocalCount:        def.LocalCount,
			GlobalCount:       def.GlobalCount,
			GlobalNames:       globalNames,
			LocalNames:        def.LocalNames,
			EnvKeys:           def.EnvKeys,
			ExceptionHandlers: handlers,
		})
	}

	return codes[0], nil
}

// checkCodeTree checks that the flattened code blocks form a tree rooted at
// the first block: each other block is the child of exactly one block that
// comes before it. This is the order Flatten produces, and it lets
// codeFromState build every child before its parent.
func checkCodeTree(defs []*codeDef) error {
	if len(defs) == 0 {
		return fmt.Errorf("invalid bytecode: no code blocks")
	}
	hasParent := make([]bool, len(defs))
	for i, def := range defs {
		if def == nil {
			return fmt.Errorf("invalid bytecode: code block %d is null", i)
		}
		for _, childIdx := range def.ChildIndices {
			if childIdx <= i || childIdx >= len(defs) {
				return fmt.Errorf("invalid bytecode: code block %d has invalid child index %d", i, childIdx)
			}
			if hasParent[childIdx] {
				return fmt.Errorf("invalid bytecode: code block %d has more than one parent", childIdx)
			}
			hasParent[childIdx] = true
		}
	}
	for i := 1; i < len(defs); i++ {
		if !hasParent[i] {
			return fmt.Errorf("invalid bytecode: code block %d has no parent", i)
		}
	}
	return nil
}

func marshalConstants(code *Code, codeIndexMap map[*Code]int) ([]json.RawMessage, error) {
	constants := make([]json.RawMessage, code.ConstantCount())
	for i := 0; i < code.ConstantCount(); i++ {
		c := code.ConstantAt(i)
		data, err := marshalConstant(c, codeIndexMap)
		if err != nil {
			return nil, err
		}
		constants[i] = data
	}
	return constants, nil
}

func marshalConstant(c any, codeIndexMap map[*Code]int) (json.RawMessage, error) {
	switch v := c.(type) {
	case nil:
		return json.Marshal(constantDef{Type: "nil"})
	case bool:
		return json.Marshal(boolConstantDef{Type: "bool", Value: v})
	case int:
		return json.Marshal(intConstantDef{Type: "int", Value: int64(v)})
	case int64:
		return json.Marshal(intConstantDef{Type: "int", Value: v})
	case float32:
		return json.Marshal(floatConstantDef{Type: "float", Value: float64(v)})
	case float64:
		return json.Marshal(floatConstantDef{Type: "float", Value: v})
	case string:
		return json.Marshal(stringConstantDef{Type: "string", Value: v})
	case *Function:
		defaults, err := marshalDefaults(v)
		if err != nil {
			return nil, err
		}
		codeIndex := -1
		if v.Code() != nil {
			if idx, ok := codeIndexMap[v.Code()]; ok {
				codeIndex = idx
			}
		}
		params := make([]string, v.ParameterCount())
		for i := 0; i < v.ParameterCount(); i++ {
			params[i] = v.Parameter(i)
		}
		return json.Marshal(functionConstantDef{
			Type: "function",
			Value: functionDef{
				ID:         v.ID(),
				Name:       v.Name(),
				Parameters: params,
				Defaults:   defaults,
				RestParam:  v.RestParam(),
				CodeIndex:  codeIndex,
			},
		})
	default:
		return nil, fmt.Errorf("unknown constant type: %T", c)
	}
}

func marshalDefaults(fn *Function) ([]json.RawMessage, error) {
	defaults := make([]json.RawMessage, fn.DefaultCount())
	for i := 0; i < fn.DefaultCount(); i++ {
		data, err := marshalConstant(fn.Default(i), nil)
		if err != nil {
			return nil, err
		}
		defaults[i] = data
	}
	return defaults, nil
}

// unmarshalConstantsImmutable unmarshals constants without requiring mutation.
// The codes slice must have all referenced codes already built (achieved by
// processing codes in reverse order during unmarshaling).
func unmarshalConstantsImmutable(data []json.RawMessage, codes []*Code) ([]any, error) {
	constants := make([]any, len(data))
	for i, d := range data {
		c, err := unmarshalConstantImmutable(d, codes)
		if err != nil {
			return nil, err
		}
		constants[i] = c
	}
	return constants, nil
}

func unmarshalConstantImmutable(data json.RawMessage, codes []*Code) (any, error) {
	var def constantDef
	if err := json.Unmarshal(data, &def); err != nil {
		return nil, err
	}

	switch def.Type {
	case "nil":
		return nil, nil
	case "bool":
		var d boolConstantDef
		if err := json.Unmarshal(data, &d); err != nil {
			return nil, err
		}
		return d.Value, nil
	case "int":
		var d intConstantDef
		if err := json.Unmarshal(data, &d); err != nil {
			return nil, err
		}
		// Return as int64 to match the compiler's internal representation.
		// The VM handles both int and int64 appropriately.
		return d.Value, nil
	case "float":
		var d floatConstantDef
		if err := json.Unmarshal(data, &d); err != nil {
			return nil, err
		}
		return d.Value, nil
	case "string":
		var d stringConstantDef
		if err := json.Unmarshal(data, &d); err != nil {
			return nil, err
		}
		return d.Value, nil
	case "function":
		var d functionConstantDef
		if err := json.Unmarshal(data, &d); err != nil {
			return nil, err
		}
		defaults, err := unmarshalConstantsImmutable(d.Value.Defaults, codes)
		if err != nil {
			return nil, err
		}
		// The function's code must already be built: a block after the one
		// that holds this constant.
		var fnCode *Code
		if d.Value.CodeIndex >= 0 && d.Value.CodeIndex < len(codes) {
			fnCode = codes[d.Value.CodeIndex]
		}
		if fnCode == nil {
			return nil, fmt.Errorf("invalid bytecode: function %q has invalid code index %d",
				d.Value.Name, d.Value.CodeIndex)
		}
		fn := NewFunction(FunctionParams{
			ID:         d.Value.ID,
			Name:       d.Value.Name,
			Parameters: d.Value.Parameters,
			Defaults:   defaults,
			RestParam:  d.Value.RestParam,
			Code:       fnCode,
		})
		return fn, nil
	default:
		return nil, fmt.Errorf("unknown constant type: %s", def.Type)
	}
}
