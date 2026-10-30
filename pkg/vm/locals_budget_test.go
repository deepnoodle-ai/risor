package vm

import (
	"context"
	"encoding/json"
	"errors"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/deepnoodle-ai/risor/v2/pkg/bytecode"
	"github.com/deepnoodle-ai/risor/v2/pkg/compiler"
	"github.com/deepnoodle-ai/risor/v2/pkg/object"
	"github.com/deepnoodle-ai/risor/v2/pkg/op"
	"github.com/deepnoodle-ai/risor/v2/pkg/parser"
	"github.com/deepnoodle-ai/wonton/assert"
)

// maxLocalCount is the most locals one function in loaded bytecode can have.
const maxLocalCount = 65535

// compileWithHighLocal compiles source and moves the local named local in the
// function named fnName to the highest slot, so that the function has
// maxLocalCount locals, the way hand-built bytecode could. The
// code goes through bytecode.Unmarshal, so it passes bytecode.Validate: the
// function really uses the high slot.
func compileWithHighLocal(t *testing.T, source, fnName, local string) *bytecode.Code {
	t.Helper()
	program, err := parser.Parse(context.Background(), source, nil)
	assert.Nil(t, err)
	var globalNames []string
	for name := range basicBuiltins() {
		globalNames = append(globalNames, name)
	}
	slices.Sort(globalNames)
	code, err := compiler.Compile(program, &compiler.Config{
		GlobalNames: globalNames,
		Source:      source,
	})
	assert.Nil(t, err)
	data, err := bytecode.Marshal(code)
	assert.Nil(t, err)

	var state struct {
		Codes []map[string]any `json:"codes"`
	}
	assert.Nil(t, json.Unmarshal(data, &state))
	patched := false
	for _, c := range state.Codes {
		if c["name"] != fnName {
			continue
		}
		var localNames []string
		for _, n := range c["local_names"].([]any) {
			localNames = append(localNames, n.(string))
		}
		slot := slices.Index(localNames, local)
		assert.True(t, slot >= 0, "no local %q in %s", local, fnName)

		// A named function keeps its own name in the slot after the others.
		highSlot := maxLocalCount - 1
		if named, _ := c["is_named"].(bool); named {
			highSlot--
		}
		instrs := c["instructions"].([]any)
		for ip := 0; ip < len(instrs); {
			opcode := op.Code(instrs[ip].(float64))
			if (opcode == op.LoadFast || opcode == op.StoreFast) && int(instrs[ip+1].(float64)) == slot {
				instrs[ip+1] = float64(highSlot)
			}
			// A cell for the current frame (0 frames back) refers to a local
			if opcode == op.MakeCell && int(instrs[ip+1].(float64)) == slot && instrs[ip+2].(float64) == 0 {
				instrs[ip+1] = float64(highSlot)
			}
			ip += 1 + op.GetInfo(opcode).OperandCount
		}
		c["local_count"] = maxLocalCount
		names := make([]any, maxLocalCount)
		for i := range names {
			names[i] = ""
		}
		c["local_names"] = names
		patched = true
	}
	assert.True(t, patched, "no function %q", fnName)

	data, err = json.Marshal(state)
	assert.Nil(t, err)
	loaded, err := bytecode.Unmarshal(data)
	assert.Nil(t, err)
	return loaded
}

// heapAllocBytes returns the bytes allocated while f runs.
func heapAllocBytes(f func()) uint64 {
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	f()
	runtime.ReadMemStats(&after)
	return after.TotalAlloc - before.TotalAlloc
}

// heapInUse returns the bytes of live heap after a collection.
func heapInUse() uint64 {
	runtime.GC()
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return m.HeapAlloc
}

const highLocalRecursion = `
function deep(n) {
	let big = n
	if (n > 0) { deep(n - 1) }
	return big
}
`

// Recursing into a function that uses a very high local slot stops at the
// locals budget, instead of reserving 64K slots for each of up to
// MaxFrameDepth frames (about 1 GB). The error is a stack overflow, which
// ends the run, so the script's catch block does not catch it.
func TestLocalsBudgetStopsHighSlotRecursion(t *testing.T) {
	code := compileWithHighLocal(t, highLocalRecursion+`
	let r = "none"
	try { deep(2000) } catch e { r = string(e) }
	r
	`, "deep", "big")

	var runErr error
	allocated := heapAllocBytes(func() {
		_, runErr = Run(context.Background(), code, WithGlobals(basicBuiltins()))
	})

	// The budget admits DefaultMaxLocals slots of 16 bytes each. Allow twice
	// that for everything else. Without the budget this allocates about
	// MaxFrameDepth * 64K * 16 bytes = 1 GB.
	limit := uint64(2 * DefaultMaxLocals * 16)
	t.Logf("allocated %d bytes (limit %d)", allocated, limit)
	assert.True(t, allocated < limit, "allocated %d bytes, limit %d", allocated, limit)

	assert.True(t, errors.Is(runErr, ErrStackOverflow), "got %v", runErr)
	assert.Contains(t, runErr.Error(), "local variable limit")
}

// Uncaught, the error reaches the host as ErrStackOverflow. WithMaxLocals
// sets the budget.
func TestLocalsBudgetUncaught(t *testing.T) {
	code := compileWithHighLocal(t, highLocalRecursion+"deep(2000)", "deep", "big")
	_, err := Run(context.Background(), code,
		WithGlobals(basicBuiltins()), WithMaxLocals(4*maxLocalCount))
	assert.NotNil(t, err)
	assert.True(t, errors.Is(err, ErrStackOverflow), "got %v", err)
	assert.Contains(t, err.Error(), "local variable limit of 262140 slots exceeded")
}

// assertLocalsReleased runs f and checks that it leaves less than 16 MiB of
// heap alive. Keeping a 64K-slot frame at each of 500 depths would keep about
// 500 MB alive.
func assertLocalsReleased(t *testing.T, f func()) {
	t.Helper()
	before := heapInUse()
	f()
	after := heapInUse()
	var retained uint64
	if after > before {
		retained = after - before
	}
	limit := uint64(16 << 20)
	t.Logf("retained %d bytes (limit %d)", retained, limit)
	assert.True(t, retained < limit, "retained %d bytes, limit %d", retained, limit)
}

// chainSource defines chain(k), which calls deep at depth k, and gives deep
// the body deepBody. mid has more locals than fit in a frame's fixed storage,
// so its frames would reuse a large slice left behind by deep. Calling chains
// from the deepest to the shallowest leaves a deep frame at each depth, unless
// frames release their large locals when they become inactive.
func chainSource(deepBody string) string {
	return `
	function deep() {
		let big = 1
		` + deepBody + `
	}
	function mid(n, target) {
		let a = 1; let b = 2; let c = 3; let d = 4; let e = 5; let f = 6; let g = 7
		if (n == target) { return deep() }
		return mid(n + 1, target)
	}
	function chain(k) { return mid(0, k) }
	`
}

// Frames that have returned must not keep large locals alive. Otherwise a
// script could fill every frame slot with 64K locals, one call chain at a
// time, while each chain stays within the budget.
func TestLocalsBudgetReturnedFramesReleaseLocals(t *testing.T) {
	code := compileWithHighLocal(t, chainSource("return big")+`
	range(500, 0, -1).each(k => chain(k))
	`, "deep", "big")
	vm, err := New(code, WithGlobals(basicBuiltins()))
	assert.Nil(t, err)
	assertLocalsReleased(t, func() {
		assert.Nil(t, vm.Run(context.Background()))
	})
	runtime.KeepAlive(vm)
}

// Frames that an error unwinds must release their large locals too, whether
// the script catches the error or the error reaches the host.
func TestLocalsBudgetUnwoundFramesReleaseLocals(t *testing.T) {
	t.Run("caught", func(t *testing.T) {
		code := compileWithHighLocal(t, chainSource(`throw "boom"`)+`
		range(500, 0, -1).each(k => try { chain(k) } catch e { 0 })
		`, "deep", "big")
		vm, err := New(code, WithGlobals(basicBuiltins()))
		assert.Nil(t, err)
		assertLocalsReleased(t, func() {
			assert.Nil(t, vm.Run(context.Background()))
		})
		runtime.KeepAlive(vm)
	})

	t.Run("host", func(t *testing.T) {
		code := compileWithHighLocal(t, chainSource(`throw "boom"`), "deep", "big")
		vm, err := New(code, WithGlobals(basicBuiltins()))
		assert.Nil(t, err)
		assert.Nil(t, vm.Run(context.Background()))
		chainObj, err := vm.Get("chain")
		assert.Nil(t, err)
		chain := chainObj.(*object.Closure)
		assertLocalsReleased(t, func() {
			for k := 500; k > 0; k-- {
				args := []object.Object{object.NewInt(int64(k))}
				_, err := vm.Call(context.Background(), chain, args)
				assert.NotNil(t, err)
				assert.Contains(t, err.Error(), "boom")
			}
		})
		runtime.KeepAlive(vm)
	})
}

// A closure must keep only the locals it captures alive, not the locals slice
// of the frame that made it. Each call to big has 64K locals, so keeping that
// slice for each of the 200 closures would keep about 200 MB alive.
func TestClosuresReleaseFrameLocals(t *testing.T) {
	code := compileWithHighLocal(t, `
	function big(i) {
		let x = i
		let get = () => x
		x = x * 10
		return [get, v => { x = v }]
	}
	let keep = range(200).map(i => big(i))
	keep[7][1](-7)
	[keep[3][0](), keep[7][0](), len(keep)]
	`, "big", "x")
	vm, err := New(code, WithGlobals(basicBuiltins()))
	assert.Nil(t, err)
	assertLocalsReleased(t, func() {
		assert.Nil(t, vm.Run(context.Background()))
	})
	result, ok := vm.TOS()
	assert.True(t, ok)
	assert.Equal(t, result, object.NewList([]object.Object{
		object.NewInt(30), object.NewInt(-7), object.NewInt(200),
	}))

	// The closures still share x after the frames return
	keep, err := vm.Get("keep")
	assert.Nil(t, err)
	pair := keep.(*object.List).Value()[5].(*object.List).Value()
	get, set := pair[0].(*object.Closure), pair[1].(*object.Closure)
	_, err = vm.Call(context.Background(), set, []object.Object{object.NewInt(55)})
	assert.Nil(t, err)
	value, err := vm.Call(context.Background(), get, nil)
	assert.Nil(t, err)
	assert.Equal(t, value, object.NewInt(55))
	runtime.KeepAlive(vm)
}

// Ordinary recursion is unaffected by the default budget.
func TestLocalsBudgetOrdinaryRecursion(t *testing.T) {
	result, err := run(context.Background(), `
	function sum(n) {
		let a = 1; let b = 2; let c = 3; let d = 4; let e = 5; let f = 6
		if (n <= 0) { return 0 }
		return n + sum(n - 1)
	}
	sum(1000)
	`)
	assert.Nil(t, err)
	assert.Equal(t, result, object.NewInt(500500))
}

// The budget counts only active frames: slots come back when frames return,
// including when an error unwinds them.
func TestLocalsBudgetRecoversAfterReturn(t *testing.T) {
	// f and g use three slots per frame (n, x, and their own name), so a
	// budget of 200 admits a recursion of 60 but not 120.
	source := `
	function f(n) {
		let x = n
		if (n <= 0) { return 0 }
		return f(n - 1) + 1
	}
	function g(n) {
		let x = n
		if (n <= 0) { throw "boom" }
		return g(n - 1) + 1
	}
	let results = []
	results.append(f(60))
	results.append(f(60))
	results.append(try { g(60) } catch e { string(e) })
	results.append(f(60))
	results.append(range(3).map(i => f(60)))
	results
	`
	program, err := parser.Parse(context.Background(), source, nil)
	assert.Nil(t, err)
	var globalNames []string
	for name := range basicBuiltins() {
		globalNames = append(globalNames, name)
	}
	code, err := compiler.Compile(program, &compiler.Config{GlobalNames: globalNames})
	assert.Nil(t, err)
	result, err := Run(context.Background(), code,
		WithGlobals(basicBuiltins()), WithMaxLocals(200))
	assert.Nil(t, err)
	list, ok := result.(*object.List)
	assert.True(t, ok, "got %v", result)
	items := list.Value()
	assert.Equal(t, len(items), 5)
	assert.Equal(t, items[0], object.NewInt(60))
	assert.Equal(t, items[1], object.NewInt(60))
	assert.True(t, strings.Contains(items[2].Inspect(), "boom"), "got %s", items[2].Inspect())
	assert.Equal(t, items[3], object.NewInt(60))
	assert.Equal(t, items[4].Inspect(), "[60, 60, 60]")

	// Exceeding the budget ends the run, even inside a try block
	program, err = parser.Parse(context.Background(), `
	function f(n) {
		let x = n
		if (n <= 0) { return 0 }
		return f(n - 1) + 1
	}
	try { f(120) } catch e { 0 }
	`, nil)
	assert.Nil(t, err)
	code, err = compiler.Compile(program, nil)
	assert.Nil(t, err)
	_, err = Run(context.Background(), code, WithMaxLocals(200))
	assert.True(t, errors.Is(err, ErrStackOverflow), "got %v", err)
	assert.Contains(t, err.Error(), "local variable limit of 200 slots exceeded")
}

// Calls from the host with VM.Call share the budget with the script.
func TestLocalsBudgetHostCall(t *testing.T) {
	program, err := parser.Parse(context.Background(), `
	function f(n) {
		let x = n
		if (n <= 0) { return 0 }
		return f(n - 1) + 1
	}
	`, nil)
	assert.Nil(t, err)
	code, err := compiler.Compile(program, nil)
	assert.Nil(t, err)
	vm, err := New(code, WithMaxLocals(200))
	assert.Nil(t, err)
	assert.Nil(t, vm.Run(context.Background()))
	fnObj, err := vm.Get("f")
	assert.Nil(t, err)
	fn := fnObj.(*object.Closure)

	result, err := vm.Call(context.Background(), fn, []object.Object{object.NewInt(60)})
	assert.Nil(t, err)
	assert.Equal(t, result, object.NewInt(60))

	_, err = vm.Call(context.Background(), fn, []object.Object{object.NewInt(120)})
	assert.True(t, errors.Is(err, ErrStackOverflow), "got %v", err)

	result, err = vm.Call(context.Background(), fn, []object.Object{object.NewInt(60)})
	assert.Nil(t, err)
	assert.Equal(t, result, object.NewInt(60))
}
