package bytecode

import (
	"strconv"
	"strings"
	"testing"

	"github.com/deepnoodle-ai/risor/v2/pkg/op"
	"github.com/deepnoodle-ai/wonton/assert"
)

func TestUnmarshalMalformedReturnsError(t *testing.T) {
	// A column of 1<<40 fits in an int on 64-bit targets, so Validate rejects
	// it. On 32-bit targets the JSON decoder rejects it first.
	hugeColumn := "invalid column"
	if strconv.IntSize == 32 {
		hugeColumn = "cannot unmarshal number"
	}
	tests := []struct {
		name string
		data string
		want string
	}{
		{"no codes", `{"codes":[]}`, "no code blocks"},
		{"null codes", `{}`, "no code blocks"},
		{"null code block", `{"codes":[null]}`, "is null"},
		{"child out of range", `{"codes":[{"child_indices":[5]}]}`, "invalid child index"},
		{"negative child", `{"codes":[{"child_indices":[-1]}]}`, "invalid child index"},
		{"child before parent", `{"codes":[{"child_indices":[1]},{"child_indices":[1]}]}`, "invalid child index"},
		{"shared child", `{"codes":[{"child_indices":[1,1]},{}]}`, "more than one parent"},
		{"orphan block", `{"codes":[{},{}]}`, "no parent"},
		{
			"function without code",
			`{"codes":[{"constants":[{"type":"function","value":{"name":"f","code_index":-1}}]}]}`,
			"invalid code index",
		},
		{
			"function code is not built yet",
			`{"codes":[{"constants":[{"type":"function","value":{"name":"f","code_index":0}}]}]}`,
			"invalid code index",
		},
		{
			"function code is a grandchild",
			`{"codes":[{"child_indices":[1],"constants":[{"type":"function","value":{"name":"f","code_index":2}}]},` +
				`{"child_indices":[2]},{}]}`,
			`function "f" code is not a child of this block`,
		},
		{
			"handler catch start at end",
			`{"codes":[{"instructions":[1],"exception_handlers":[{"try_start":0,"try_end":1,"catch_start":1}]}]}`,
			"exception handler 0",
		},
		{"unknown opcode", `{"codes":[{"instructions":[999]}]}`, "unknown opcode 999"},
		{"invalid opcode", `{"codes":[{"instructions":[0]}]}`, "unknown opcode 0"},
		{"truncated instruction", `{"codes":[{"instructions":[24]}]}`, "truncated LOAD_CONST"},
		{"constant out of range", `{"codes":[{"instructions":[24,0]}]}`, "constant index 0 out of range"},
		{"name out of range", `{"codes":[{"instructions":[20,3]}]}`, "name index 3 out of range"},
		{"local out of range", `{"codes":[{"instructions":[21,0],"local_count":0}]}`, "local index 0 out of range"},
		{"global out of range", `{"codes":[{"instructions":[23,2],"global_count":2}]}`, "global index 2 out of range"},
		{"negative local count", `{"codes":[{"local_count":-1}]}`, "local count -1 out of range"},
		{"huge global count", `{"codes":[{"global_count":100000000}]}`, "global count 100000000 out of range"},
		{
			"closure of a non-function",
			`{"codes":[{"instructions":[120,0,0],"constants":[{"type":"int","value":1}]}]}`,
			"constant 0 is not a function",
		},
		{"jump past end", `{"codes":[{"instructions":[11,10]}]}`, "jump target 10"},
		{"jump to operand", `{"codes":[{"instructions":[11,3,24,0],"constants":[{"type":"nil"}]}]}`, "jump target 3"},
		{"jump before start", `{"codes":[{"instructions":[10,1]}]}`, "jump target -1"},
		{"catch target past end", `{"codes":[{"instructions":[140,9,0]}]}`, "catch target 9"},
		{"finally target past end", `{"codes":[{"instructions":[140,0,9]}]}`, "finally target 9"},
		{
			"handler past end",
			`{"codes":[{"instructions":[1],"exception_handlers":[{"try_start":0,"try_end":5}]}]}`,
			"exception handler 0",
		},
		{"huge column", `{"codes":[{"locations":[{"line":1,"column":1099511627776}]}]}`, hugeColumn},
		{"negative column", `{"codes":[{"locations":[{"line":1,"column":-1}]}]}`, "location 0 has an invalid column"},
		{"huge end column", `{"codes":[{"locations":[{"line":1,"column":1,"end_column":1099511627776}]}]}`, hugeColumn},
		{"negative end column", `{"codes":[{"locations":[{"line":1,"column":1,"end_column":-5}]}]}`, "invalid column"},
		{"bad json", `{"codes":`, "unexpected end"},
		{"unknown constant type", `{"codes":[{"constants":[{"type":"x"}]}]}`, "unknown constant type"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, err := Unmarshal([]byte(tt.data))
			assert.Nil(t, code)
			assert.NotNil(t, err)
			assert.True(t, strings.Contains(err.Error(), tt.want), err.Error())
		})
	}
}

func TestValidate(t *testing.T) {
	fnCode := NewCode(CodeParams{ID: "main.0", Name: "f"})
	fn := NewFunction(FunctionParams{ID: "f", Name: "f", Parameters: []string{"a"}, Code: fnCode})
	valid := NewCode(CodeParams{
		ID:           "main",
		Children:     []*Code{fnCode},
		Instructions: []op.Code{op.LoadClosure, 0, 0, op.PopJumpForwardIfTrue, 3, op.Nop},
		Constants:    []any{fn},
	})
	assert.Nil(t, Validate(valid))

	// Local counts the compiler emits: parameter and rest slots, a named
	// function's own slot, and one root local per global.
	named := NewCode(CodeParams{ID: "main.0", Name: "g", IsNamed: true, LocalCount: 3})
	assert.Nil(t, Validate(NewCode(CodeParams{
		GlobalCount: 4,
		LocalCount:  4,
		Children:    []*Code{named},
		Constants: []any{NewFunction(FunctionParams{
			Name: "g", Parameters: []string{"a"}, RestParam: "r", Code: named,
		})},
	})))

	tests := []struct {
		name string
		code *Code
		want string
	}{
		{"nil", nil, "nil code"},
		{
			"function without code",
			NewCode(CodeParams{Constants: []any{NewFunction(FunctionParams{Name: "f"})}}),
			`function "f" has no code`,
		},
		{
			"unsupported constant",
			NewCode(CodeParams{Constants: []any{float32(1)}}),
			"unsupported type float32",
		},
		{
			"too many parameters",
			NewCode(CodeParams{Constants: []any{NewFunction(FunctionParams{
				Name: "f", Parameters: make([]string, 257), Code: fnCode,
			})}}),
			"257 parameters",
		},
		{
			"more defaults than parameters",
			NewCode(CodeParams{Constants: []any{NewFunction(FunctionParams{
				Name: "f", Defaults: []any{int64(1)}, Code: fnCode,
			})}}),
			"1 defaults for 0 parameters",
		},
		{
			"fewer defaults than parameters",
			NewCode(CodeParams{Children: []*Code{fnCode}, Constants: []any{NewFunction(FunctionParams{
				Name: "f", Parameters: []string{"x", "y"}, Defaults: []any{int64(99)}, Code: fnCode,
			})}}),
			`function "f" has 1 defaults for 2 parameters`,
		},
		{
			"function code is not a child",
			NewCode(CodeParams{Constants: []any{fn}}),
			`constant 0: function "f" code is not a child of this block`,
		},
		{
			"function code is a child of another block",
			NewCode(CodeParams{Children: []*Code{
				fnCode,
				NewCode(CodeParams{Name: "other", Constants: []any{fn}}),
			}}),
			`invalid bytecode in "other": constant 0: function "f" code is not a child`,
		},
		{
			"catch target at end",
			NewCode(CodeParams{Instructions: []op.Code{op.PushExcept, 3, 0}}),
			"catch target 3 is not an instruction",
		},
		{
			"finally target at end",
			NewCode(CodeParams{Instructions: []op.Code{op.PushExcept, 3, 4, op.Nop}}),
			"finally target 4 is not an instruction",
		},
		{
			"handler catch start at end",
			NewCode(CodeParams{
				Instructions:      []op.Code{op.Nop, op.Nop},
				ExceptionHandlers: []ExceptionHandler{{TryStart: 0, TryEnd: 1, CatchStart: 2, CatchVarIdx: -1}},
			}),
			"exception handler 0 has an invalid position",
		},
		{
			"handler finally start at end",
			NewCode(CodeParams{
				Instructions: []op.Code{op.Nop, op.Nop},
				ExceptionHandlers: []ExceptionHandler{
					{TryStart: 0, TryEnd: 1, CatchStart: 1, FinallyStart: 2, CatchVarIdx: -1},
				},
			}),
			"exception handler 0 has an invalid position",
		},
		{
			"function default",
			NewCode(CodeParams{Constants: []any{NewFunction(FunctionParams{
				Name: "f", Parameters: []string{"a"}, Defaults: []any{fn}, Code: fnCode,
			})}}),
			"default of unsupported type",
		},
		{
			"invalid function code",
			func() *Code {
				body := NewCode(CodeParams{Name: "body", Instructions: []op.Code{op.LoadFast, 0}})
				return NewCode(CodeParams{Children: []*Code{body}, Constants: []any{NewFunction(FunctionParams{
					Name: "f", Code: body,
				})}})
			}(),
			`invalid bytecode in "body": LOAD_FAST at offset 0: local index 0 out of range`,
		},
		{
			"make cell local out of range",
			NewCode(CodeParams{Children: []*Code{
				NewCode(CodeParams{Name: "child", Instructions: []op.Code{op.MakeCell, 2, 0}}),
			}}),
			"MAKE_CELL at offset 0: local index 2 out of range",
		},
		{
			"free variable of a function loaded without captures",
			func() *Code {
				body := NewCode(CodeParams{Name: "body", Instructions: []op.Code{op.LoadFree, 0}})
				return NewCode(CodeParams{
					Instructions: []op.Code{op.LoadConst, 0},
					Children:     []*Code{body},
					Constants:    []any{NewFunction(FunctionParams{Name: "f", Code: body})},
				})
			}(),
			`invalid bytecode in "body": LOAD_FREE at offset 0: free variable index 0 out of range (count 0)`,
		},
		{
			"local count larger than used",
			NewCode(CodeParams{Children: []*Code{
				NewCode(CodeParams{Name: "child", LocalCount: 3, Instructions: []op.Code{op.LoadFast, 1}}),
			}}),
			"local count 3 is larger than the 2 locals the code uses",
		},
		{
			"root locals beyond globals",
			NewCode(CodeParams{Name: "main", GlobalCount: 2, LocalCount: 3}),
			"local count 3 is larger than the 2 locals",
		},
		{
			"negative column",
			NewCode(CodeParams{Locations: []SourceLocation{{Line: 1, Column: -1}}}),
			"location 0 has an invalid column (column -1, end column 0)",
		},
		{
			"child global checked against root",
			NewCode(CodeParams{GlobalCount: 1, Children: []*Code{
				NewCode(CodeParams{Name: "child", GlobalCount: 5, Instructions: []op.Code{op.LoadGlobal, 1}}),
			}}),
			"global index 1 out of range",
		},
	}
	// A column past math.MaxInt32 only fits in an int on 64-bit targets.
	if strconv.IntSize == 64 {
		huge := int64(1) << 40
		tests = append(tests, struct {
			name string
			code *Code
			want string
		}{
			"huge column in a child",
			NewCode(CodeParams{Source: "x", Children: []*Code{
				NewCode(CodeParams{Name: "child", Locations: []SourceLocation{
					{Line: 1, Column: 1}, {Line: 1, Column: int(huge)},
				}}),
			}}),
			`invalid bytecode in "child": location 1 has an invalid column (column 1099511627776, end column 0)`,
		})
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Validate(tt.code)
			assert.NotNil(t, err)
			assert.True(t, strings.Contains(err.Error(), tt.want), err.Error())
		})
	}
}

func TestInstructionIterInvalidInstructions(t *testing.T) {
	truncated := NewInstructionIter(NewCode(CodeParams{
		Instructions: []op.Code{op.Nop, op.LoadConst},
	}))
	assert.Equal(t, truncated.All(), [][]op.Code{{op.Nop}})
	assert.NotNil(t, truncated.Err())
	assert.Equal(t, truncated.Err().Error(), "truncated LOAD_CONST instruction at offset 1")

	unknown := NewInstructionIter(NewCode(CodeParams{
		Instructions: []op.Code{op.Code(300), op.Nop},
	}))
	assert.Equal(t, len(unknown.All()), 0)
	assert.Equal(t, unknown.Err().Error(), "unknown opcode 300 at offset 0")

	ok := NewInstructionIter(NewCode(CodeParams{
		Instructions: []op.Code{op.LoadConst, 0, op.Nop},
	}))
	assert.Equal(t, ok.All(), [][]op.Code{{op.LoadConst, 0}, {op.Nop}})
	assert.Nil(t, ok.Err())
}

func FuzzUnmarshal(f *testing.F) {
	fnCode := NewCode(CodeParams{
		ID: "main.0", Name: "f", LocalCount: 2,
		Instructions: []op.Code{op.LoadFast, 1, op.ReturnValue},
	})
	main := NewCode(CodeParams{
		ID:           "main",
		Children:     []*Code{fnCode},
		Instructions: []op.Code{op.LoadClosure, 0, 0, op.LoadConst, 1, op.Call, 1, op.JumpForward, 2},
		Constants: []any{
			NewFunction(FunctionParams{ID: "f", Name: "f", Parameters: []string{"a"}, RestParam: "r", Code: fnCode}),
			"x",
		},
		GlobalCount: 1,
		EnvKeys:     []string{"x"},
		ExceptionHandlers: []ExceptionHandler{
			{TryStart: 0, TryEnd: 3, CatchStart: 3, CatchVarIdx: -1},
		},
	})
	seed, err := Marshal(main)
	assert.Nil(f, err)
	f.Add(seed)
	f.Add([]byte(`{"codes":[]}`))
	f.Add([]byte(`{"codes":[{"instructions":[24]}]}`))
	f.Fuzz(func(t *testing.T, data []byte) {
		code, err := Unmarshal(data)
		if err != nil {
			return
		}
		// Accepted code must marshal again and decode cleanly.
		_, err = Marshal(code)
		assert.Nil(t, err)
		for _, c := range code.Flatten() {
			iter := NewInstructionIter(c)
			iter.All()
			assert.Nil(t, iter.Err())
		}
	})
}
