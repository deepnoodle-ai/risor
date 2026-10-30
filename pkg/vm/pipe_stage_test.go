package vm

import (
	"testing"

	"github.com/deepnoodle-ai/risor/v2/pkg/object"
)

// Only a pipe stage's own call is completed by the piped value. Calls in the
// stage's arguments, receiver, or callee are ordinary calls.
func TestPipeStageNestedCalls(t *testing.T) {
	runTests(t, []testCase{
		// Calls in the stage's arguments
		{`let add = (a, b) => a + b; let two = () => 2; 10 |> add(two())`, object.NewInt(12)},
		{`let add = (a, b) => a + b; 10 |> add(len("abc"))`, object.NewInt(13)},
		{`let add = (a, b) => a + b; let obj = {n: () => 4}; 10 |> add(obj.n())`, object.NewInt(14)},
		// A call in the stage's receiver
		{`let getObj = () => ({add: (a, b) => a + b}); 5 |> getObj().add(10)`, object.NewInt(15)},
		// A call in the stage's callee
		{`let makeAdder = n => (x => x + n); 5 |> makeAdder(1)()`, object.NewInt(6)},
		// A pipe in the stage's arguments
		{`let add = (a, b) => a + b; let inc = x => x + 1; 5 |> add(1 |> inc)`, object.NewInt(7)},
		// A pipe in a function in a stage
		{`[1, 2] |> (xs => xs.map(x => x |> (y => y * 10)))`, object.NewList([]object.Object{object.NewInt(10), object.NewInt(20)})},
		// Stages after a stage with nested calls
		{`let add = (a, b) => a + b; let two = () => 2; 10 |> add(two()) |> add(two())`, object.NewInt(14)},
		{`let obj = {add: (a, b) => a + b}; let two = () => 2; 5 |> obj?.add(two())`, object.NewInt(7)},
	})
}
