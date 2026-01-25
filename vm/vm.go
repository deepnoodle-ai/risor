// Package vm provides a VirtualMachine that executes compiled Risor code.
package vm

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/risor-io/risor/compiler"
	"github.com/risor-io/risor/errz"
	"github.com/risor-io/risor/importer"
	"github.com/risor-io/risor/object"
	"github.com/risor-io/risor/op"
	"github.com/risor-io/risor/os"
)

const (
	MaxArgs       = 256
	MaxFrameDepth = 1024
	MaxStackDepth = 1024
	StopSignal    = -1
	MB            = 1024 * 1024
)

var ErrGlobalNotFound = errors.New("global not found")

type VirtualMachine struct {
	ip           int // instruction pointer
	sp           int // stack pointer
	fp           int // frame pointer
	halt         int32
	startCount   int64
	activeFrame  *frame
	activeCode   *code
	main         *compiler.Code
	importer     importer.Importer
	os           os.OS
	modules      map[string]*object.Module
	inputGlobals map[string]any
	globals      map[string]object.Object
	loadedCode   map[*compiler.Code]*code
	running      bool
	runMutex     sync.Mutex
	cloneMutex   sync.Mutex
	tmp          [MaxArgs]object.Object
	stack        [MaxStackDepth]object.Object
	frames       [MaxFrameDepth]frame
}

// New creates a new Virtual Machine.
func New(main *compiler.Code, options ...Option) *VirtualMachine {
	vm, err := createVM(options)
	if err != nil {
		// Being unable to convert globals to Risor objects is more likely a
		// programming error than a runtime error, so this panic is borderline
		// appropriate. The only reason we're keeping this function signature
		// and not just switching to returning an error is for compatibility.
		// Using NewEmpty instead of New addresses this.
		panic(err)
	}
	vm.main = main
	return vm
}

// NewEmpty creates a new Virtual Machine without initial main code.
// Code can be provided later using RunCode, or functions can be called
// directly using Call.
func NewEmpty() (*VirtualMachine, error) {
	return createVM(nil)
}

func createVM(options []Option) (*VirtualMachine, error) {
	vm := &VirtualMachine{
		sp:           -1,
		modules:      map[string]*object.Module{},
		inputGlobals: map[string]any{},
		globals:      map[string]object.Object{},
		loadedCode:   map[*compiler.Code]*code{},
	}
	if err := vm.applyOptions(options); err != nil {
		return nil, err
	}
	return vm, nil
}

func (vm *VirtualMachine) applyOptions(options []Option) error {
	vm.runMutex.Lock()
	defer vm.runMutex.Unlock()

	if vm.running {
		return fmt.Errorf("vm is already running")
	}

	// Apply options
	for _, opt := range options {
		opt(vm)
	}

	// Convert globals to Risor objects
	var err error
	vm.globals, err = object.AsObjects(vm.inputGlobals)
	if err != nil {
		return fmt.Errorf("invalid global provided: %v", err)
	}

	// Add any globals that are modules to a cache to make them available
	// to import statements
	for name, value := range vm.globals {
		if module, ok := value.(*object.Module); ok {
			vm.modules[name] = module
		}
	}
	return nil
}

func (vm *VirtualMachine) start(ctx context.Context) error {
	vm.runMutex.Lock()
	defer vm.runMutex.Unlock()
	if vm.running {
		return fmt.Errorf("vm is already running")
	}
	vm.running = true
	vm.startCount++
	// Halt execution when the context is cancelled
	vm.halt = 0
	if doneChan := ctx.Done(); doneChan != nil {
		go func() {
			<-doneChan
			atomic.StoreInt32(&vm.halt, 1)
		}()
	}
	return nil
}

func (vm *VirtualMachine) stop() {
	vm.runMutex.Lock()
	defer vm.runMutex.Unlock()
	vm.running = false
}

func (vm *VirtualMachine) Run(ctx context.Context) (err error) {
	if vm.main == nil {
		return fmt.Errorf("no main code available")
	}
	return vm.runCodeInternal(ctx, vm.main, false)
}

// RunCode runs the given compiled code object on the VM. This allows running
// multiple different code objects on the same VM instance sequentially.
// The VM must not be currently running when this method is called.
func (vm *VirtualMachine) RunCode(ctx context.Context, codeToRun *compiler.Code, opts ...Option) (err error) {
	if err := vm.applyOptions(opts); err != nil {
		return err
	}
	return vm.runCodeInternal(ctx, codeToRun, true)
}

// runCodeInternal is the shared implementation for Run and RunCode
func (vm *VirtualMachine) runCodeInternal(ctx context.Context, codeToRun *compiler.Code, resetState bool) (err error) {
	// Set up some guarantees:
	// 1. It is an error to call Run on a VM that is already running
	// 2. The running flag will always be set to false when Run returns
	// 3. Any panics are translated to errors and the VM is stopped
	if err := vm.start(ctx); err != nil {
		return err
	}
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic: %v", r)
		}
		vm.stop()
	}()

	// Reset VM state for new code execution if requested
	if resetState && vm.startCount > 1 {
		vm.resetForNewCode()
	}

	// Load the code to run - unified logic for both paths
	var codeObj *code

	// Check if we already have this code loaded
	if existingCode, exists := vm.loadedCode[codeToRun]; exists {
		if !resetState {
			// For Run(), we need to preserve globals from previous runs (REPL behavior)
			// Use reloadCode to get fresh code with preserved globals
			codeObj = vm.reloadCode(codeToRun)
		} else {
			// Reuse the existing code object as-is
			codeObj = existingCode
		}
	} else {
		// Load this code for the first time
		codeObj = vm.loadCode(codeToRun)
	}

	// Load function constants
	for i := 0; i < codeToRun.ConstantsCount(); i++ {
		if fn, ok := codeToRun.Constant(i).(*compiler.Function); ok {
			vm.loadCode(fn.Code())
		}
	}

	// Activate the entrypoint code in frame zero
	// Use vm.ip for Run (preserving existing behavior), 0 for RunCode
	startIP := 0
	if !resetState {
		startIP = vm.ip
	}
	vm.activateCode(0, startIP, codeObj)

	// Run the entrypoint until completion
	return vm.eval(vm.initContext(ctx))
}

// resetForNewCode resets the VM state for running a new code object
func (vm *VirtualMachine) resetForNewCode() {
	vm.sp = -1
	vm.ip = 0
	vm.fp = 0
	vm.halt = 0
	vm.activeFrame = nil
	vm.activeCode = nil
	vm.loadedCode = map[*compiler.Code]*code{}
	vm.modules = map[string]*object.Module{}

	// Clear arrays
	for i := 0; i < MaxStackDepth; i++ {
		vm.stack[i] = nil
	}
	for i := 0; i < MaxFrameDepth; i++ {
		vm.frames[i] = frame{}
	}
	for i := 0; i < MaxArgs; i++ {
		vm.tmp[i] = nil
	}
}

// Get a global variable by name as a Risor Object.
func (vm *VirtualMachine) Get(name string) (object.Object, error) {
	code := vm.activeCode
	if code == nil {
		return nil, errors.New("no active code")
	}
	for i := 0; i < code.GlobalsCount(); i++ {
		if g := code.Global(i); g.Name() == name {
			return code.Globals[g.Index()], nil
		}
	}
	return nil, fmt.Errorf("%w: %q", ErrGlobalNotFound, name)
}

// GlobalNames returns the names of all global variables in the active code.
func (vm *VirtualMachine) GlobalNames() []string {
	code := vm.activeCode
	if code == nil {
		return nil
	}
	count := code.GlobalsCount()
	names := make([]string, 0, count)
	for i := 0; i < count; i++ {
		names = append(names, code.Global(i).Name())
	}
	return names
}

// Evaluate the active code. The caller must initialize the following variables
// before calling this function:
//   - vm.ip - instruction pointer within the active code
//   - vm.fp - frame pointer with the active code already set
//   - vm.activeCode - the code object to execute
//   - vm.activeFrame - the active call frame to use
//
// Assuming this function returns without error, the result of the evaluation
// will be on the top of the stack.
func (vm *VirtualMachine) eval(ctx context.Context) error {
	// Run to the end of the active code
	for vm.ip < len(vm.activeCode.Instructions) {

		if atomic.LoadInt32(&vm.halt) == 1 {
			return ctx.Err()
		}

		// The current instruction opcode
		opcode := vm.activeCode.Instructions[vm.ip]

		// fmt.Println("ip", vm.ip, op.GetInfo(opcode).Name, "sp", vm.sp)

		// Advance the instruction pointer to the next instruction. Note that
		// this is done before we actually execute the current instruction, so
		// relative jump instructions will need to take this into account.
		vm.ip++

		// Dispatch the instruction
		switch opcode {
		case op.Nop:
		case op.LoadAttr:
			obj := vm.pop()
			name := vm.activeCode.Names[vm.fetch()]
			value, found := obj.GetAttr(name)
			if !found {
				return errz.TypeErrorf("type error: attribute %q not found on %s object",
					name, obj.Type())
			}
			switch value := value.(type) {
			case object.AttrResolver:
				attr, err := value.ResolveAttr(ctx, name)
				if err != nil {
					return err
				}
				vm.push(attr)
			default:
				vm.push(value)
			}
		case op.LoadConst:
			vm.push(vm.activeCode.Constants[vm.fetch()])
		case op.LoadFast:
			vm.push(vm.activeFrame.Locals()[vm.fetch()])
		case op.LoadGlobal:
			vm.push(vm.activeCode.Globals[vm.fetch()])
		case op.LoadFree:
			idx := vm.fetch()
			freeVars := vm.activeFrame.fn.FreeVars()
			obj := freeVars[idx].Value()
			vm.push(obj)
		case op.StoreFast:
			idx := vm.fetch()
			obj := vm.pop()
			vm.activeFrame.Locals()[idx] = obj
		case op.StoreGlobal:
			vm.activeCode.Globals[vm.fetch()] = vm.pop()
		case op.StoreFree:
			idx := vm.fetch()
			obj := vm.pop()
			freeVars := vm.activeFrame.fn.FreeVars()
			freeVars[idx].Set(obj)
		case op.StoreAttr:
			idx := vm.fetch()
			obj := vm.pop()
			value := vm.pop()
			name := vm.activeCode.Names[idx]
			if err := obj.SetAttr(name, value); err != nil {
				return err
			}
		case op.LoadClosure:
			constIndex := vm.fetch()
			freeCount := vm.fetch()
			free := make([]*object.Cell, freeCount)
			for i := uint16(0); i < freeCount; i++ {
				obj := vm.pop()
				switch obj := obj.(type) {
				case *object.Cell:
					free[freeCount-i-1] = obj
				default:
					return errz.EvalErrorf("eval error: expected cell")
				}
			}
			fn := vm.activeCode.Constants[constIndex].(*object.Function)
			vm.push(object.NewClosure(fn, free))
		case op.MakeCell:
			symbolIndex := vm.fetch()
			framesBack := int(vm.fetch())
			frameIndex := vm.fp - framesBack
			if frameIndex < 0 {
				return errz.EvalErrorf("eval error: no frame at depth %d", framesBack)
			}
			frame := &vm.frames[frameIndex]
			locals := frame.CaptureLocals()
			vm.push(object.NewCell(&locals[symbolIndex]))
		case op.Nil:
			vm.push(object.Nil)
		case op.True:
			vm.push(object.True)
		case op.False:
			vm.push(object.False)
		case op.CompareOp:
			opType := op.CompareOpType(vm.fetch())
			b := vm.pop()
			a := vm.pop()
			result, err := object.Compare(opType, a, b)
			if err != nil {
				return err
			}
			vm.push(result)
		case op.BinaryOp:
			opType := op.BinaryOpType(vm.fetch())
			b := vm.pop()
			a := vm.pop()
			result, err := object.BinaryOp(opType, a, b)
			if err != nil {
				return err
			}
			vm.push(result)
		case op.Call:
			argc := int(vm.fetch())
			if argc > MaxArgs {
				return errz.EvalErrorf("eval error: max args limit of %d exceeded (got %d)",
					MaxArgs, argc)
			}
			args := make([]object.Object, argc)
			for argIndex := argc - 1; argIndex >= 0; argIndex-- {
				args[argIndex] = vm.pop()
			}
			obj := vm.pop()
			if err := vm.callObject(ctx, obj, args); err != nil {
				return err
			}
		case op.Partial:
			argc := int(vm.fetch())
			args := make([]object.Object, argc)
			for i := argc - 1; i >= 0; i-- {
				args[i] = vm.pop()
			}
			obj := vm.pop()
			partial := object.NewPartial(obj, args)
			vm.push(partial)
		case op.ReturnValue:
			activeFrame := vm.activeFrame
			returnAddr := activeFrame.returnAddr
			returnSp := activeFrame.returnSp
			returnFp := vm.fp - 1
			vm.resumeFrame(returnFp, returnAddr, returnSp)
			if returnAddr == StopSignal {
				// If StopSignal is found as the return address, it means the
				// current eval call should stop.
				return nil
			}
		case op.PopJumpForwardIfTrue:
			tos := vm.pop()
			delta := int(vm.fetch()) - 2
			if tos.IsTruthy() {
				vm.ip += delta
			}
		case op.PopJumpForwardIfFalse:
			tos := vm.pop()
			delta := int(vm.fetch()) - 2
			if !tos.IsTruthy() {
				vm.ip += delta
			}
		case op.JumpForward:
			base := vm.ip - 1
			delta := int(vm.fetch())
			vm.ip = base + delta
		case op.JumpBackward:
			base := vm.ip - 1
			delta := int(vm.fetch())
			vm.ip = base - delta
		case op.BuildList:
			count := vm.fetch()
			items := make([]object.Object, count)
			for i := uint16(0); i < count; i++ {
				items[count-1-i] = vm.pop()
			}
			vm.push(object.NewList(items))
		case op.BuildMap:
			count := vm.fetch()
			items := make(map[string]object.Object, count)
			for i := uint16(0); i < count; i++ {
				v := vm.pop()
				k := vm.pop()
				items[k.(*object.String).Value()] = v
			}
			vm.push(object.NewMap(items))
		case op.BuildSet:
			count := vm.fetch()
			items := make([]object.Object, count)
			for i := uint16(0); i < count; i++ {
				items[i] = vm.pop()
			}
			vm.push(object.NewSet(items))
		case op.BinarySubscr:
			idx := vm.pop()
			lhs := vm.pop()
			container, ok := lhs.(object.Container)
			if !ok {
				return errz.TypeErrorf("type error: object is not a container (got %s)", lhs.Type())
			}
			result, err := container.GetItem(idx)
			if err != nil {
				return err.Value()
			}
			vm.push(result)
		case op.StoreSubscr:
			idx := vm.pop()
			lhs := vm.pop()
			rhs := vm.pop()
			container, ok := lhs.(object.Container)
			if !ok {
				return errz.TypeErrorf("type error: object is not a container (got %s)", lhs.Type())
			}
			if err := container.SetItem(idx, rhs); err != nil {
				return err.Value()
			}
		case op.UnaryNegative:
			obj := vm.pop()
			switch obj := obj.(type) {
			case *object.Int:
				vm.push(object.NewInt(-obj.Value()))
			case *object.Float:
				vm.push(object.NewFloat(-obj.Value()))
			default:
				return errz.TypeErrorf("type error: object is not a number (got %s)", obj.Type())
			}
		case op.UnaryNot:
			obj := vm.pop()
			if obj.IsTruthy() {
				vm.push(object.False)
			} else {
				vm.push(object.True)
			}
		case op.ContainsOp:
			obj := vm.pop()
			containerObj := vm.pop()
			invert := vm.fetch() == 1
			if container, ok := containerObj.(object.Container); ok {
				value := container.Contains(obj)
				if invert {
					value = object.Not(value)
				}
				vm.push(value)
			} else {
				return errz.TypeErrorf("type error: object is not a container (got %s)",
					containerObj.Type())
			}
		case op.Swap:
			vm.swap(int(vm.fetch()))
		case op.BuildString:
			count := vm.fetch()
			items := make([]string, count)
			for i := uint16(0); i < count; i++ {
				dst := count - 1 - i
				obj := vm.pop()
				switch obj := obj.(type) {
				case *object.Error:
					if obj.IsRaised() {
						return obj.Value()
					}
					items[dst] = obj.Value().Error()
				case *object.String:
					items[dst] = obj.Value()
				default:
					items[dst] = obj.Inspect()
				}
			}
			vm.push(object.NewString(strings.Join(items, "")))
		case op.Range:
			iterableObj := vm.pop()
			iterable, ok := iterableObj.(object.Iterable)
			if !ok {
				return errz.TypeErrorf("type error: object is not an iterable (got %s)",
					iterableObj.Type())
			}
			vm.push(iterable.Iter())
		case op.Slice:
			start := vm.pop()
			stop := vm.pop()
			containerObj := vm.pop()
			container, ok := containerObj.(object.Container)
			if !ok {
				return errz.TypeErrorf("type error: object is not a container (got %s)",
					containerObj.Type())
			}
			slice := object.Slice{Start: start, Stop: stop}
			result, err := container.GetSlice(slice)
			if err != nil {
				return err.Value()
			}
			vm.push(result)
		case op.Length:
			containerObj := vm.pop()
			container, ok := containerObj.(object.Container)
			if !ok {
				return errz.TypeErrorf("type error: object is not a container (got %s)",
					containerObj.Type())
			}
			vm.push(container.Len())
		case op.Copy:
			offset := vm.fetch()
			vm.push(vm.stack[vm.sp-int(offset)])
		case op.Import:
			name, ok := vm.pop().(*object.String)
			if !ok {
				return errz.TypeErrorf("type error: object is not a string (got %s)", name.Type())
			}
			module, err := vm.importModule(ctx, name.Value())
			if err != nil {
				return err
			}
			vm.push(module)
		case op.FromImport:
			parentLen := vm.fetch()
			importsCount := vm.fetch()
			if importsCount > 255 {
				return errz.EvalErrorf("eval error: invalid imports count: %d", importsCount)
			}
			var names []string
			for i := uint16(0); i < importsCount; i++ {
				name, ok := vm.pop().(*object.String)
				if !ok {
					return errz.TypeErrorf("type error: object is not a string (got %s)", name.Type())
				}
				names = append(names, name.Value())
			}
			from := make([]string, parentLen)
			for i := int(parentLen - 1); i >= 0; i-- {
				val, ok := vm.pop().(*object.String)
				if !ok {
					return errz.TypeErrorf("type error: object is not a string (got %s)", val.Type())
				}
				from[i] = val.Value()
			}
			for _, name := range names {
				// check if the name matches a module
				module, err := vm.importModule(ctx, filepath.Join(filepath.Join(from...), name))
				if err == nil {
					vm.push(module)
				} else {
					// otherwise, the name is a symbol inside a module
					module, err := vm.importModule(ctx, filepath.Join(from...))
					if err != nil {
						return err
					}
					attr, found := module.GetAttr(name)
					if !found {
						return fmt.Errorf("import error: cannot import name %q from %q",
							name, module.Name())
					}
					vm.push(attr)
				}
			}
		case op.PopTop:
			vm.pop()
		case op.Unpack:
			containerObj := vm.pop()
			nameCount := int64(vm.fetch())
			container, ok := containerObj.(object.Container)
			if !ok {
				return errz.TypeErrorf("type error: object is not a container (got %s)",
					containerObj.Type())
			}
			containerSize := container.Len().Value()
			if containerSize != nameCount {
				return fmt.Errorf("unpack count mismatch: %d != %d", containerSize, nameCount)
			}
			iter := container.Iter()
			for {
				val, ok := iter.Next(ctx)
				if !ok {
					break
				}
				vm.push(val)
			}
		case op.GetIter:
			obj := vm.pop()
			switch obj := obj.(type) {
			case object.Iterable:
				vm.push(obj.Iter())
			case object.Iterator:
				vm.push(obj)
			default:
				return errz.TypeErrorf("type error: object is not iterable (got %s)", obj.Type())
			}
		case op.ForIter:
			base := vm.ip - 1
			jumpAmount := vm.fetch()
			nameCount := vm.fetch()
			iter := vm.pop().(object.Iterator)
			if _, ok := iter.Next(ctx); !ok {
				vm.ip = base + int(jumpAmount)
			} else {
				obj, _ := iter.Entry()
				vm.push(iter)
				if nameCount == 1 {
					vm.push(obj.Key())
				} else if nameCount == 2 {
					vm.push(obj.Value())
					vm.push(obj.Key())
				} else if nameCount == 3 {
					// Python-style for-in: single variable gets the value
					vm.push(obj.Value())
				} else if nameCount != 0 {
					return errz.EvalErrorf("eval error: invalid iteration")
				}
			}
		case op.Defer:
			obj := vm.pop()
			partial, ok := obj.(*object.Partial)
			if !ok {
				return errz.TypeErrorf("type error: object is not a partial (got %s)", obj.Type())
			}
			vm.activeFrame.Defer(partial)
		case op.Halt:
			return nil
		default:
			return errz.EvalErrorf("eval error: unknown opcode: %d", opcode)
		}
	}
	return nil
}

// GetIP returns the current instruction pointer.
func (vm *VirtualMachine) GetIP() int {
	return vm.ip
}

// SetIP sets the instruction pointer on a stopped VM. If the VM is running, an
// error is returned.
func (vm *VirtualMachine) SetIP(value int) error {
	vm.runMutex.Lock()
	defer vm.runMutex.Unlock()
	if vm.running {
		return errors.New("cannot set ip while the vm is running")
	}
	vm.ip = value
	return nil
}

// TOS returns the top-of-stack object if there is one, without modifying the
// stack. The returned bool value indicates whether there was a valid TOS. This
// only works on a stopped VM. If the VM is running, (nil, false) is returned.
func (vm *VirtualMachine) TOS() (object.Object, bool) {
	vm.runMutex.Lock()
	defer vm.runMutex.Unlock()
	if !vm.running && vm.sp >= 0 {
		return vm.stack[vm.sp], true
	}
	return nil, false
}

func (vm *VirtualMachine) pop() object.Object {
	obj := vm.stack[vm.sp]
	vm.stack[vm.sp] = nil
	vm.sp--
	return obj
}

func (vm *VirtualMachine) push(obj object.Object) {
	vm.sp++
	vm.stack[vm.sp] = obj
}

func (vm *VirtualMachine) swap(pos int) {
	otherIndex := vm.sp - pos
	tos := vm.stack[vm.sp]
	other := vm.stack[otherIndex]
	vm.stack[otherIndex] = tos
	vm.stack[vm.sp] = other
}

func (vm *VirtualMachine) fetch() uint16 {
	ip := vm.ip
	vm.ip++
	return uint16(vm.activeCode.Instructions[ip])
}

// Call a function with the given arguments. If isolation between VMs is
// important to you, do not provide a function that obtained from another VM,
// since it could be a closure over variables there. If this VM is already
// running, an error is returned.
func (vm *VirtualMachine) Call(
	ctx context.Context,
	fn *object.Function,
	args []object.Object,
) (result object.Object, err error) {
	if err := vm.start(ctx); err != nil {
		return nil, err
	}
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic: %v", r)
		}
		vm.stop()
	}()
	return vm.callFunction(vm.initContext(ctx), fn, args)
}

// Calls a compiled function with the given arguments. This is used internally
// when a Risor object calls a function, e.g. [1, 2, 3].map(func(x) { x + 1 }).
func (vm *VirtualMachine) callFunction(
	ctx context.Context,
	fn *object.Function,
	args []object.Object,
) (result object.Object, resultErr error) {
	// Check that the argument count is appropriate
	paramsCount := len(fn.Parameters())
	argc := len(args)

	if argc > MaxArgs {
		return nil, errz.EvalErrorf("eval error: max args limit of %d exceeded (got %d)",
			MaxArgs, argc)
	}
	if err := checkCallArgs(fn, argc); err != nil {
		return nil, err
	}

	baseFP := vm.fp
	baseIP := vm.ip
	baseSP := vm.sp

	// Restore the previous frame when done
	defer vm.resumeFrame(baseFP, baseIP, baseSP)

	// Assemble frame local variables in vm.tmp. The local variable order is:
	// 1. Function parameters
	// 2. Function name (if the function is named)
	copy(vm.tmp[:argc], args)
	if argc < paramsCount {
		defaults := fn.Defaults()
		for i := argc; i < len(defaults); i++ {
			vm.tmp[i] = defaults[i]
		}
		argc = paramsCount
	}
	code := fn.Code()
	if code.IsNamed() {
		vm.tmp[paramsCount] = fn
		argc++
	}

	// Activate a frame for the function call
	vm.activateFunction(vm.fp+1, 0, fn, vm.tmp[:argc])

	// Setting StopSignal as the return address will cause the eval function to
	// stop execution when it reaches the end of the active code.
	vm.activeFrame.returnAddr = StopSignal

	// Set up deferred function calls
	callFrame := vm.activeFrame
	defer func() {
		for _, partial := range callFrame.defers {
			if err := vm.callObject(ctx, partial.Function(), partial.Args()); err != nil {
				result = nil
				resultErr = err
			} else {
				// Discard the result of the deferred function call, which is
				// guaranteed to have pushed a single value onto the stack.
				vm.pop()
			}
		}
	}()

	// Evaluate the function code then return the result from TOS
	if err := vm.eval(ctx); err != nil {
		return nil, err
	}
	return vm.pop(), nil
}

// Call a callable object with the given arguments. Returns an error if the
// object is not callable. If this call succeeds, the result of the call will
// have been pushed onto the stack.
func (vm *VirtualMachine) callObject(
	ctx context.Context,
	fn object.Object,
	args []object.Object,
) error {
	switch fn := fn.(type) {
	case *object.Function:
		result, err := vm.callFunction(ctx, fn, args)
		if err != nil {
			return err
		}
		vm.push(result)
		return nil
	case object.Callable:
		result := fn.Call(ctx, args...)
		if err, ok := result.(*object.Error); ok && err.IsRaised() {
			return err.Value()
		}
		vm.push(result)
		return nil
	case *object.Partial:
		// Combine the current arguments with the partial's arguments
		argc := len(args)
		expandedCount := argc + len(fn.Args())
		if expandedCount > MaxArgs {
			return errz.EvalErrorf("eval error: max arguments limit of %d exceeded (got %d)",
				MaxArgs, expandedCount)
		}
		newArgs := make([]object.Object, expandedCount)
		copy(newArgs[:argc], args)
		copy(newArgs[argc:], fn.Args())
		// Recursive call with the wrapped function and the combined args
		return vm.callObject(ctx, fn.Function(), newArgs)
	default:
		return errz.TypeErrorf("type error: object is not callable (got %s)", fn.Type())
	}
}

// Resume the frame at the given frame pointer, restoring the given IP and SP.
func (vm *VirtualMachine) resumeFrame(fp, ip, sp int) *frame {
	// The return value of the previous frame is on the top of the stack
	var frameResult object.Object = nil
	if vm.sp > sp {
		frameResult = vm.pop()
	}
	// Remove any items left on the stack by the previous frame
	for i := vm.sp; i > sp; i-- {
		vm.stack[i] = nil
	}
	vm.sp = sp
	// Push the frame result back onto the stack
	if frameResult != nil {
		vm.push(frameResult)
	}
	// Activate the resumed frame
	vm.fp = fp
	vm.ip = ip
	vm.activeFrame = &vm.frames[fp]
	vm.activeCode = vm.activeFrame.code
	return vm.activeFrame
}

// Activate a frame with the given code. This is typically used to begin
// running the entrypoint for a module or script.
func (vm *VirtualMachine) activateCode(fp, ip int, code *code) *frame {
	vm.fp = fp
	vm.ip = ip
	vm.activeFrame = &vm.frames[fp]
	vm.activeFrame.ActivateCode(code)
	vm.activeCode = code
	return vm.activeFrame
}

// Activate a frame with the given function, to implement a function call.
func (vm *VirtualMachine) activateFunction(fp, ip int, fn *object.Function, locals []object.Object) *frame {
	code := vm.loadCode(fn.Code())
	returnAddr := vm.ip
	returnSp := vm.sp
	vm.fp = fp
	vm.ip = ip
	vm.activeFrame = &vm.frames[fp]
	vm.activeFrame.ActivateFunction(fn, code, returnAddr, returnSp, locals)
	vm.activeCode = code
	return vm.activeFrame
}

// Wrap the *compiler.Code in a *vm.code object to make it usable by the VM.
func (vm *VirtualMachine) loadCode(cc *compiler.Code) *code {
	if code, ok := vm.loadedCode[cc]; ok {
		return code
	}
	// Loading is slightly different if this is the "root" (entrypoint) code
	// vs. a child of that. The root code owns the globals array, while the
	// children will reuse the globals from the root.
	var c *code
	rootCompiled := cc.Root()
	if rootCompiled == cc {
		c = loadRootCode(cc, vm.globals)
	} else {
		c = loadChildCode(vm.loadedCode[rootCompiled], cc)
	}
	// Store the loaded code but ensure we don't modify the map during a clone
	vm.cloneMutex.Lock()
	defer vm.cloneMutex.Unlock()
	vm.loadedCode[cc] = c
	return c
}

// Reloads the main code while preserving global variables. This happens as
// part of a typical REPL workflow, where the main code is appended to with
// each new input.
func (vm *VirtualMachine) reloadCode(main *compiler.Code) *code {
	oldWrappedMain, ok := vm.loadedCode[main]
	if !ok {
		panic("main code not loaded")
	}
	delete(vm.loadedCode, main)
	newWrappedMain := vm.loadCode(main)
	copy(newWrappedMain.Globals, oldWrappedMain.Globals)
	return newWrappedMain
}

func (vm *VirtualMachine) importModule(ctx context.Context, name string) (*object.Module, error) {
	if module, ok := vm.modules[name]; ok {
		return module, nil
	}
	if vm.importer == nil {
		return nil, fmt.Errorf("imports are disabled")
	}
	module, err := vm.importer.Import(ctx, name)
	if err != nil {
		return nil, err
	}
	// Activate a new frame to evaluate the module code
	baseFP := vm.fp
	baseIP := vm.ip
	baseSP := vm.sp
	code := vm.loadCode(module.Code())
	vm.activateCode(vm.fp+1, 0, code)
	// Restore the previous frame when done
	defer vm.resumeFrame(baseFP, baseIP, baseSP)
	// Evaluate the module code
	if err := vm.eval(ctx); err != nil {
		return nil, err
	}
	module.UseGlobals(code.Globals)
	// Store the loaded module but ensure we don't modify the map during a clone
	vm.cloneMutex.Lock()
	defer vm.cloneMutex.Unlock()
	vm.modules[name] = module
	return module, nil
}

// Clone the Virtual Machine. The returned clone has its own independent
// frame stack and data stack, but shares the loaded modules and global
// variables with the original VM.
//
// Clone is designed to be safe to call from any goroutine.
//
// The caller and the user code that runs are responsible for thread safety when
// using modules and global variables, since concurrently executing cloned VMs
// can modify the same objects.
//
// The returned clone has an empty frame stack and data stack, which makes this
// most useful for cloning a VM then using vm.Call() to call a function, rather
// than calling vm.Run() on the clone, which would start execution at the
// beginning of the main entrypoint.
//
// Do not use Clone if you want a strict guarantee of isolation between VMs.
//
// If an OS was provided to the original VM via the WithOS option, it will be
// copied into the clone. Otherwise, the clone will use the standard OS fallback
// behavior of using any OS present in the context or NewSimpleOS as a default.
func (vm *VirtualMachine) Clone() (*VirtualMachine, error) {
	// Locking cloneMutex is done to prevent clones while code is being loaded
	// or modules are being imported
	vm.cloneMutex.Lock()
	defer vm.cloneMutex.Unlock()

	// Snapshot the loaded modules
	modules := make(map[string]*object.Module, len(vm.modules))
	for name, module := range vm.modules {
		modules[name] = module
	}

	// Snapshot the loaded code
	loadedCode := make(map[*compiler.Code]*code, len(vm.loadedCode))
	for cc, c := range vm.loadedCode {
		loadedCode[cc] = c
	}

	clone := &VirtualMachine{
		sp:           -1,
		ip:           0,
		fp:           0,
		running:      false,
		importer:     vm.importer,
		os:           vm.os,
		main:         vm.main,
		inputGlobals: vm.inputGlobals,
		globals:      vm.globals,
		modules:      modules,
		loadedCode:   loadedCode,
	}

	// Only activate main code if it exists
	if clone.main != nil {
		clone.activateCode(clone.fp, clone.ip, clone.loadCode(clone.main))
	}

	return clone, nil
}

// Clones the VM and then calls the function synchronously in the clone.
func (vm *VirtualMachine) cloneCallSync(
	ctx context.Context,
	fn *object.Function,
	args []object.Object,
) (object.Object, error) {
	clone, err := vm.Clone()
	if err != nil {
		return nil, err
	}
	return clone.callFunction(clone.initContext(ctx), fn, args)
}

func (vm *VirtualMachine) initContext(ctx context.Context) context.Context {
	oss := vm.getOS(ctx)
	ctx = os.WithOS(ctx, oss)
	ctx = object.WithCallFunc(ctx, vm.callFunction)
	ctx = object.WithCloneCallFunc(ctx, vm.cloneCallSync)
	return ctx
}

// getOS retrieves the OS reference for a VM. Before v1.8.0, the OS was passed
// solely via a context value. Starting with v1.8.0, the WithOS option is the
// preferred way to set the VM's OS. This getOS method bridges the old and new
// approaches. If the OS is not provided via context or WithOS, NewSimpleOS is
// used as a default.
func (vm *VirtualMachine) getOS(ctx context.Context) os.OS {
	v, ok := os.GetOS(ctx)
	if ok {
		return v
	}
	if vm.os != nil {
		return vm.os
	}
	return os.NewSimpleOS(ctx)
}
