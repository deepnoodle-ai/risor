package vm_test

import (
	"context"
	"errors"
	"fmt"

	"github.com/deepnoodle-ai/risor/v2/pkg/bytecode"
	"github.com/deepnoodle-ai/risor/v2/pkg/compiler"
	"github.com/deepnoodle-ai/risor/v2/pkg/object"
	"github.com/deepnoodle-ai/risor/v2/pkg/parser"
	"github.com/deepnoodle-ai/risor/v2/pkg/vm"
)

// Reuse one VM for several runs. Globals that a script changed carry over,
// a value given with WithGlobals replaces the current value, and a function
// from replaced code returns ErrReplacedCode.
func ExampleVirtualMachine_RunCode() {
	ctx := context.Background()
	compile := func(source string, globals ...string) *bytecode.Code {
		ast, _ := parser.Parse(ctx, source, nil)
		code, _ := compiler.Compile(ast, &compiler.Config{GlobalNames: globals})
		return code
	}
	machine, _ := vm.New(compile("let count = 1\nfunction get() { return count }"))
	_ = machine.Run(ctx)
	get, _ := machine.Get("get")

	result, _ := vm.RunCodeOnVM(ctx, machine, compile("count += 1\ncount", "count"))
	fmt.Println(result)

	for _, user := range []string{"alice", "bob"} {
		input := vm.WithGlobals(map[string]any{"user": user})
		result, _ = vm.RunCodeOnVM(ctx, machine, compile("user", "user"), input)
		fmt.Println(result)
	}

	_, err := machine.Call(ctx, get.(*object.Closure), nil)
	fmt.Println(errors.Is(err, vm.ErrReplacedCode))
	// Output:
	// 2
	// alice
	// bob
	// true
}
