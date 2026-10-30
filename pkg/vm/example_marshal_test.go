package vm_test

import (
	"context"
	"fmt"

	"github.com/deepnoodle-ai/risor/v2/pkg/bytecode"
	"github.com/deepnoodle-ai/risor/v2/pkg/compiler"
	"github.com/deepnoodle-ai/risor/v2/pkg/parser"
	"github.com/deepnoodle-ai/risor/v2/pkg/vm"
)

func Example_marshalBytecode() {
	ctx := context.Background()
	source := `
	function tail(first, ...rest) { return rest }
	try { throw "boom" } catch e { tail(1, 2, x) }`
	program, _ := parser.Parse(ctx, source, nil)
	code, _ := compiler.Compile(program, &compiler.Config{
		GlobalNames: []string{"x"},
		Source:      source, // errors show the line they occur on
	})

	data, _ := bytecode.Marshal(code)
	loaded, _ := bytecode.Unmarshal(data)
	result, err := vm.Run(ctx, loaded, vm.WithGlobals(map[string]any{"x": 3}))
	fmt.Println(loaded.EnvKeys(), result, err)

	_, err = bytecode.Unmarshal([]byte(`{"codes":[{"instructions":[24, 7]}]}`))
	fmt.Println(err)
	// Output:
	// [x] [2, 3] <nil>
	// invalid bytecode in "": LOAD_CONST at offset 0: constant index 7 out of range (count 0)
}
