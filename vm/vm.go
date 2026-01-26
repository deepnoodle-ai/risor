// Package vm provides a VirtualMachine that executes compiled Risor code.
package vm

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/risor-io/risor/compiler"
	"github.com/risor-io/risor/object"
	"github.com/risor-io/risor/op"
)

const (
	MaxArgs       = 256
	MaxFrameDepth = 1024
	MaxStackDepth = 1024
	StopSignal    = -1
	MB            = 1024 * 1024

	// InitialFrameCapacity is the initial size of the frame stack.
	// It grows dynamically up to MaxFrameDepth as needed.
	InitialFrameCapacity = 16

	// DefaultContextCheckInterval is the number of instructions between
	// deterministic checks of ctx.Done(). Set to 0 to disable.
	DefaultContextCheckInterval = 1000
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
	inputGlobals map[string]any
	globals      map[string]object.Object
	loadedCode   map[*compiler.Code]*code
	running      bool
	runMutex     sync.Mutex
	tmp          [MaxArgs]object.Object
	stack        [MaxStackDepth]object.Object
	frames       []frame // Dynamically sized, grows up to MaxFrameDepth

	// contextCheckInterval is the number of instructions between deterministic
	// checks of ctx.Done(). A value of 0 disables deterministic checking,
	// relying only on the background goroutine. Default is DefaultContextCheckInterval.
	contextCheckInterval int

	// observer receives callbacks for VM execution events (steps, calls, returns).
	// If nil, no callbacks are made.
	observer Observer

	// Exception handling state
	excStack     []exceptionFrame
	excStackSize int
}

// exceptionFrame represents an active exception handler on the exception stack.
type exceptionFrame struct {
	handler      *compiler.ExceptionHandler
	code         *code         // The code object containing this handler
	fp           int           // Frame pointer when handler was pushed
	pendingError *object.Error // Error to re-throw after finally (if any)
	inFinally    bool          // Are we currently executing a finally block?
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
		sp:                   -1,
		inputGlobals:         map[string]any{},
		globals:              map[string]object.Object{},
		loadedCode:           map[*compiler.Code]*code{},
		contextCheckInterval: DefaultContextCheckInterval,
		frames:               make([]frame, InitialFrameCapacity),
		excStack:             make([]exceptionFrame, 8), // Small initial exception stack
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
	vm.excStackSize = 0

	// Clear stack (only used portion would be cleaner but this ensures GC)
	for i := 0; i < MaxStackDepth; i++ {
		vm.stack[i] = nil
	}
	// Clear frames - only clear used frames, keep capacity
	for i := range vm.frames {
		vm.frames[i] = frame{}
	}
	// Clear tmp array
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
	// Instruction counter for deterministic context checking
	var instructionCount int
	checkInterval := vm.contextCheckInterval
	doneChan := ctx.Done()

	// Run to the end of the active code
	for vm.ip < len(vm.activeCode.Instructions) {

		if atomic.LoadInt32(&vm.halt) == 1 {
			return ctx.Err()
		}

		// Deterministic check of ctx.Done() every N instructions.
		// This guarantees responsiveness regardless of goroutine scheduling.
		if checkInterval > 0 && doneChan != nil {
			instructionCount++
			if instructionCount >= checkInterval {
				instructionCount = 0
				select {
				case <-doneChan:
					atomic.StoreInt32(&vm.halt, 1)
					return ctx.Err()
				default:
					// Context not cancelled, continue execution
				}
			}
		}

		// The current instruction opcode
		opcode := vm.activeCode.Instructions[vm.ip]

		// fmt.Println("ip", vm.ip, op.GetInfo(opcode).Name, "sp", vm.sp)

		// Call observer if present
		if vm.observer != nil {
			event := StepEvent{
				IP:         vm.ip,
				Opcode:     opcode,
				OpcodeName: op.GetInfo(opcode).Name,
				Location:   vm.activeCode.LocationAt(vm.ip),
				StackDepth: vm.sp + 1,
				FrameDepth: vm.fp + 1,
			}
			if !vm.observer.OnStep(event) {
				return fmt.Errorf("execution halted by observer")
			}
		}

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
				if herr := vm.tryHandleError(vm.typeError("attribute %q not found on %s object",
					name, obj.Type())); herr != nil {
					return herr
				}
				continue
			}
			switch value := value.(type) {
			case object.AttrResolver:
				attr, err := value.ResolveAttr(ctx, name)
				if err != nil {
					if herr := vm.tryHandleError(err); herr != nil {
						return herr
					}
					continue
				}
				vm.push(attr)
			default:
				vm.push(value)
			}
		case op.LoadAttrOrNil:
			// Like LoadAttr but returns nil instead of error for missing attributes
			obj := vm.pop()
			name := vm.activeCode.Names[vm.fetch()]
			value, found := obj.GetAttr(name)
			if !found {
				vm.push(object.Nil)
			} else {
				switch value := value.(type) {
				case object.AttrResolver:
					attr, err := value.ResolveAttr(ctx, name)
					if err != nil {
						vm.push(object.Nil)
					} else {
						vm.push(attr)
					}
				default:
					vm.push(value)
				}
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
				if herr := vm.tryHandleError(err); herr != nil {
					return herr
				}
				continue
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
					if herr := vm.tryHandleError(vm.evalError("expected cell")); herr != nil {
						return herr
					}
					continue
				}
			}
			fn := vm.activeCode.Constants[constIndex].(*object.Function)
			vm.push(object.NewClosure(fn, free))
		case op.MakeCell:
			symbolIndex := vm.fetch()
			framesBack := int(vm.fetch())
			frameIndex := vm.fp - framesBack
			if frameIndex < 0 {
				if herr := vm.tryHandleError(vm.evalError("no frame at depth %d", framesBack)); herr != nil {
					return herr
				}
				continue
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
				if herr := vm.tryHandleError(err); herr != nil {
					return herr
				}
				continue
			}
			vm.push(result)
		case op.BinaryOp:
			opType := op.BinaryOpType(vm.fetch())
			b := vm.pop()
			a := vm.pop()
			result, err := object.BinaryOp(opType, a, b)
			if err != nil {
				if herr := vm.tryHandleError(err); herr != nil {
					return herr
				}
				continue
			}
			vm.push(result)
		case op.Call:
			argc := int(vm.fetch())
			if argc > MaxArgs {
				if herr := vm.tryHandleError(vm.evalError("max args limit of %d exceeded (got %d)",
					MaxArgs, argc)); herr != nil {
					return herr
				}
				continue
			}
			args := make([]object.Object, argc)
			for argIndex := argc - 1; argIndex >= 0; argIndex-- {
				args[argIndex] = vm.pop()
			}
			obj := vm.pop()
			if err := vm.callObject(ctx, obj, args); err != nil {
				if herr := vm.tryHandleError(err); herr != nil {
					return herr
				}
				continue
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
		case op.CallSpread:
			// Call with arguments from a list on the stack
			argList := vm.pop()
			list, ok := argList.(*object.List)
			if !ok {
				if herr := vm.tryHandleError(vm.typeError("spread call requires list of arguments (got %s)", argList.Type())); herr != nil {
					return herr
				}
				continue
			}
			args := list.Value()
			obj := vm.pop()
			if err := vm.callObject(ctx, obj, args); err != nil {
				if herr := vm.tryHandleError(err); herr != nil {
					return herr
				}
				continue
			}
		case op.ReturnValue:
			activeFrame := vm.activeFrame

			// Call observer if present (before we lose the frame info)
			if vm.observer != nil {
				funcName := ""
				if activeFrame.fn != nil {
					funcName = activeFrame.fn.Name()
				}
				event := ReturnEvent{
					FunctionName: funcName,
					Location:     vm.getCurrentLocation(),
					FrameDepth:   vm.fp,
				}
				if !vm.observer.OnReturn(event) {
					return fmt.Errorf("execution halted by observer")
				}
			}

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
		case op.PopJumpForwardIfNotNil:
			tos := vm.pop()
			delta := int(vm.fetch()) - 2
			if tos != object.Nil {
				vm.ip += delta
			}
		case op.PopJumpForwardIfNil:
			tos := vm.pop()
			delta := int(vm.fetch()) - 2
			if tos == object.Nil {
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
		case op.ListAppend:
			// Append TOS to list at TOS-1
			item := vm.pop()
			listObj := vm.pop()
			list, ok := listObj.(*object.List)
			if !ok {
				if herr := vm.tryHandleError(vm.typeError("cannot append to non-list (got %s)", listObj.Type())); herr != nil {
					return herr
				}
				continue
			}
			newItems := append(list.Value(), item)
			vm.push(object.NewList(newItems))
		case op.ListExtend:
			// Extend list at TOS-1 with iterable at TOS
			iterableObj := vm.pop()
			listObj := vm.pop()
			list, ok := listObj.(*object.List)
			if !ok {
				if herr := vm.tryHandleError(vm.typeError("cannot extend non-list (got %s)", listObj.Type())); herr != nil {
					return herr
				}
				continue
			}
			// Get items from the enumerable
			// For maps, spread yields keys; for other containers, spread yields values
			if m, ok := iterableObj.(*object.Map); ok {
				newItems := list.Value()
				m.Enumerate(ctx, func(key, value object.Object) bool {
					newItems = append(newItems, key)
					return true
				})
				vm.push(object.NewList(newItems))
				continue
			}
			enumerable, ok := iterableObj.(object.Enumerable)
			if !ok {
				if herr := vm.tryHandleError(vm.typeError("spread requires an enumerable (got %s)", iterableObj.Type())); herr != nil {
					return herr
				}
				continue
			}
			newItems := list.Value()
			enumerable.Enumerate(ctx, func(key, value object.Object) bool {
				newItems = append(newItems, value)
				return true
			})
			vm.push(object.NewList(newItems))
		case op.MapMerge:
			// Merge map at TOS into map at TOS-1
			sourceObj := vm.pop()
			targetObj := vm.pop()
			target, ok := targetObj.(*object.Map)
			if !ok {
				if herr := vm.tryHandleError(vm.typeError("cannot merge into non-map (got %s)", targetObj.Type())); herr != nil {
					return herr
				}
				continue
			}
			source, ok := sourceObj.(*object.Map)
			if !ok {
				if herr := vm.tryHandleError(vm.typeError("spread requires a map (got %s)", sourceObj.Type())); herr != nil {
					return herr
				}
				continue
			}
			// Merge source into target (creating a new map)
			newItems := make(map[string]object.Object)
			for k, v := range target.Value() {
				newItems[k] = v
			}
			for k, v := range source.Value() {
				newItems[k] = v
			}
			vm.push(object.NewMap(newItems))
		case op.MapSet:
			// Set key (TOS-1) to value (TOS) in map at TOS-2
			value := vm.pop()
			keyObj := vm.pop()
			targetObj := vm.pop()
			target, ok := targetObj.(*object.Map)
			if !ok {
				if herr := vm.tryHandleError(vm.typeError("cannot set key in non-map (got %s)", targetObj.Type())); herr != nil {
					return herr
				}
				continue
			}
			key, ok := keyObj.(*object.String)
			if !ok {
				if herr := vm.tryHandleError(vm.typeError("map key must be string (got %s)", keyObj.Type())); herr != nil {
					return herr
				}
				continue
			}
			// Create a new map with the key-value pair
			newItems := make(map[string]object.Object)
			for k, v := range target.Value() {
				newItems[k] = v
			}
			newItems[key.Value()] = value
			vm.push(object.NewMap(newItems))
		case op.BinarySubscr:
			idx := vm.pop()
			lhs := vm.pop()
			container, ok := lhs.(object.Container)
			if !ok {
				if herr := vm.tryHandleError(vm.typeError("object is not a container (got %s)", lhs.Type())); herr != nil {
					return herr
				}
				continue
			}
			result, err := container.GetItem(idx)
			if err != nil {
				if herr := vm.handleException(err); herr != nil {
					return herr
				}
				continue
			}
			vm.push(result)
		case op.StoreSubscr:
			idx := vm.pop()
			lhs := vm.pop()
			rhs := vm.pop()
			container, ok := lhs.(object.Container)
			if !ok {
				if herr := vm.tryHandleError(vm.typeError("object is not a container (got %s)", lhs.Type())); herr != nil {
					return herr
				}
				continue
			}
			if err := container.SetItem(idx, rhs); err != nil {
				if herr := vm.handleException(err); herr != nil {
					return herr
				}
				continue
			}
		case op.UnaryNegative:
			obj := vm.pop()
			switch obj := obj.(type) {
			case *object.Int:
				vm.push(object.NewInt(-obj.Value()))
			case *object.Float:
				vm.push(object.NewFloat(-obj.Value()))
			default:
				if herr := vm.tryHandleError(vm.typeError("object is not a number (got %s)", obj.Type())); herr != nil {
					return herr
				}
				continue
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
				if herr := vm.tryHandleError(vm.typeError("object is not a container (got %s)",
					containerObj.Type())); herr != nil {
					return herr
				}
				continue
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
						if herr := vm.handleException(obj); herr != nil {
							return herr
						}
						continue
					}
					items[dst] = obj.Value().Error()
				case *object.String:
					items[dst] = obj.Value()
				default:
					items[dst] = obj.Inspect()
				}
			}
			vm.push(object.NewString(strings.Join(items, "")))
		case op.Slice:
			start := vm.pop()
			stop := vm.pop()
			containerObj := vm.pop()
			container, ok := containerObj.(object.Container)
			if !ok {
				if herr := vm.tryHandleError(vm.typeError("object is not a container (got %s)",
					containerObj.Type())); herr != nil {
					return herr
				}
				continue
			}
			slice := object.Slice{Start: start, Stop: stop}
			result, err := container.GetSlice(slice)
			if err != nil {
				if herr := vm.handleException(err); herr != nil {
					return herr
				}
				continue
			}
			vm.push(result)
		case op.Length:
			containerObj := vm.pop()
			container, ok := containerObj.(object.Container)
			if !ok {
				if herr := vm.tryHandleError(vm.typeError("object is not a container (got %s)",
					containerObj.Type())); herr != nil {
					return herr
				}
				continue
			}
			vm.push(container.Len())
		case op.Copy:
			offset := vm.fetch()
			vm.push(vm.stack[vm.sp-int(offset)])
		case op.PopTop:
			vm.pop()
		case op.Unpack:
			containerObj := vm.pop()
			nameCount := int64(vm.fetch())
			container, ok := containerObj.(object.Container)
			if !ok {
				if herr := vm.tryHandleError(vm.typeError("object is not a container (got %s)",
					containerObj.Type())); herr != nil {
					return herr
				}
				continue
			}
			containerSize := container.Len().Value()
			// Allow fewer elements than expected (for destructuring with defaults)
			if containerSize > nameCount {
				if herr := vm.tryHandleError(fmt.Errorf("unpack count mismatch: %d > %d", containerSize, nameCount)); herr != nil {
					return herr
				}
				continue
			}
			count := int64(0)
			// For maps, unpack yields keys; for other containers, unpack yields values
			if m, ok := containerObj.(*object.Map); ok {
				m.Enumerate(ctx, func(key, value object.Object) bool {
					vm.push(key)
					count++
					return true
				})
			} else {
				container.Enumerate(ctx, func(key, value object.Object) bool {
					vm.push(value)
					count++
					return true
				})
			}
			// Pad with nil for missing elements (allows defaults to work)
			for count < nameCount {
				vm.push(object.Nil)
				count++
			}
		case op.Halt:
			return nil
		case op.PushExcept:
			// Push an exception handler onto the exception stack
			catchOffset := vm.fetch()
			finallyOffset := vm.fetch()
			baseIP := vm.ip - 3 // Position of the PushExcept instruction

			// Find the handler for this position in the current code
			var handler *compiler.ExceptionHandler
			for _, h := range vm.activeCode.ExceptionHandlers {
				if h.TryStart <= baseIP && baseIP < h.TryEnd {
					handler = h
					break
				}
			}

			if handler == nil {
				// Create a temporary handler based on offsets
				handler = &compiler.ExceptionHandler{
					TryStart:     baseIP,
					CatchStart:   baseIP + int(catchOffset),
					FinallyStart: baseIP + int(finallyOffset),
					CatchVarIdx:  -1,
				}
			}

			vm.excStack[vm.excStackSize] = exceptionFrame{
				handler: handler,
				code:    vm.activeCode,
				fp:      vm.fp,
			}
			vm.excStackSize++
		case op.PopExcept:
			// Pop exception handler (normal completion of try block)
			if vm.excStackSize > 0 {
				vm.excStackSize--
			}
		case op.Throw:
			// Throw the value on TOS as an exception
			tosObj := vm.pop()

			// Convert to error if needed
			var errObj *object.Error
			switch v := tosObj.(type) {
			case *object.Error:
				errObj = v
			case *object.String:
				errObj = object.NewError(fmt.Errorf("%s", v.Value()))
			default:
				errObj = object.NewError(fmt.Errorf("%s", tosObj.Inspect()))
			}

			// Handle the exception
			if err := vm.handleException(errObj); err != nil {
				return err
			}
		case op.EndFinally:
			// End of finally block - re-raise pending exception if any
			if vm.excStackSize > 0 {
				excFrame := &vm.excStack[vm.excStackSize-1]
				if excFrame.inFinally && excFrame.pendingError != nil {
					// Re-raise the pending error
					pendingErr := excFrame.pendingError
					excFrame.pendingError = nil
					excFrame.inFinally = false
					vm.excStackSize-- // Pop this handler

					// Try to find another handler
					if err := vm.handleException(pendingErr); err != nil {
						return err
					}
				} else if excFrame.inFinally {
					// No pending error, just clear the finally flag
					excFrame.inFinally = false
					vm.excStackSize--
				}
			}
		default:
			if herr := vm.tryHandleError(vm.evalError("unknown opcode: %d", opcode)); herr != nil {
				return herr
			}
			continue
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
		return nil, vm.evalError("max args limit of %d exceeded (got %d)",
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
	// 2. Rest parameter (if any)
	// 3. Function name (if the function is named)

	localCount := paramsCount
	hasRestParam := fn.HasRestParam()

	if hasRestParam {
		// Copy regular parameters
		copyCount := argc
		if copyCount > paramsCount {
			copyCount = paramsCount
		}
		copy(vm.tmp[:copyCount], args[:copyCount])

		// Fill in defaults for missing regular params
		if copyCount < paramsCount {
			defaults := fn.Defaults()
			for i := copyCount; i < len(defaults); i++ {
				vm.tmp[i] = defaults[i]
			}
		}

		// Collect remaining args into rest param list
		var restArgs []object.Object
		if argc > paramsCount {
			restArgs = args[paramsCount:]
		} else {
			restArgs = []object.Object{}
		}
		vm.tmp[paramsCount] = object.NewList(restArgs)
		localCount = paramsCount + 1
	} else {
		// No rest param - original behavior
		copy(vm.tmp[:argc], args)
		if argc < paramsCount {
			defaults := fn.Defaults()
			for i := argc; i < len(defaults); i++ {
				vm.tmp[i] = defaults[i]
			}
		}
		localCount = paramsCount
	}

	code := fn.Code()
	if code.IsNamed() {
		vm.tmp[localCount] = fn
		localCount++
	}
	argc = localCount

	// Activate a frame for the function call
	vm.activateFunction(vm.fp+1, 0, fn, vm.tmp[:argc])

	// Call observer if present
	if vm.observer != nil {
		event := CallEvent{
			FunctionName: fn.Name(),
			ArgCount:     len(args),
			Location:     vm.getCurrentLocation(),
			FrameDepth:   vm.fp + 1,
		}
		if !vm.observer.OnCall(event) {
			return nil, fmt.Errorf("execution halted by observer")
		}
	}

	// Setting StopSignal as the return address will cause the eval function to
	// stop execution when it reaches the end of the active code.
	vm.activeFrame.returnAddr = StopSignal

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
			return vm.evalError("max arguments limit of %d exceeded (got %d)",
				MaxArgs, expandedCount)
		}
		newArgs := make([]object.Object, expandedCount)
		copy(newArgs[:argc], args)
		copy(newArgs[argc:], fn.Args())
		// Recursive call with the wrapped function and the combined args
		return vm.callObject(ctx, fn.Function(), newArgs)
	default:
		return vm.typeError("object is not callable (got %s)", fn.Type())
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

// ensureFrameCapacity grows the frames slice if needed to accommodate the given frame index.
// Returns an error if the frame index exceeds MaxFrameDepth.
func (vm *VirtualMachine) ensureFrameCapacity(fp int) error {
	if fp >= MaxFrameDepth {
		return fmt.Errorf("stack overflow: frame depth %d exceeds maximum %d", fp, MaxFrameDepth)
	}
	if fp >= len(vm.frames) {
		// Grow the slice - double capacity or grow to fit, whichever is larger
		newCap := len(vm.frames) * 2
		if newCap < fp+1 {
			newCap = fp + 1
		}
		if newCap > MaxFrameDepth {
			newCap = MaxFrameDepth
		}
		newFrames := make([]frame, newCap)
		copy(newFrames, vm.frames)
		vm.frames = newFrames
	}
	return nil
}

// Activate a frame with the given code. This is typically used to begin
// running the entrypoint for a module or script.
func (vm *VirtualMachine) activateCode(fp, ip int, code *code) *frame {
	// Ensure we have capacity for this frame (panics on overflow for now,
	// matching the previous fixed-array behavior)
	if err := vm.ensureFrameCapacity(fp); err != nil {
		panic(err)
	}
	vm.fp = fp
	vm.ip = ip
	vm.activeFrame = &vm.frames[fp]
	vm.activeFrame.ActivateCode(code)
	vm.activeCode = code
	return vm.activeFrame
}

// Activate a frame with the given function, to implement a function call.
func (vm *VirtualMachine) activateFunction(fp, ip int, fn *object.Function, locals []object.Object) *frame {
	// Ensure we have capacity for this frame (panics on overflow for now,
	// matching the previous fixed-array behavior)
	if err := vm.ensureFrameCapacity(fp); err != nil {
		panic(err)
	}
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

func (vm *VirtualMachine) initContext(ctx context.Context) context.Context {
	return object.WithCallFunc(ctx, vm.callFunction)
}

// captureStack builds a stack trace from the current call frames.
func (vm *VirtualMachine) captureStack() []object.StackFrame {
	var frames []object.StackFrame

	// Walk through all active frames
	for i := vm.fp; i >= 0; i-- {
		frame := &vm.frames[i]
		if frame.code == nil {
			continue
		}

		// Get the function name
		funcName := ""
		if frame.fn != nil {
			funcName = frame.fn.Name()
			if funcName == "" {
				funcName = "<anonymous>"
			}
		} else if frame.code.CodeName() != "" {
			funcName = frame.code.CodeName()
		} else {
			funcName = "<main>"
		}

		// Get the location - use returnAddr for previous frames, current ip for active frame
		ip := 0
		if i == vm.fp {
			ip = vm.ip - 1 // Current instruction (ip was already incremented)
			if ip < 0 {
				ip = 0
			}
		} else {
			ip = frame.returnAddr - 1
			if ip < 0 {
				ip = 0
			}
		}

		loc := frame.code.LocationAt(ip)

		frames = append(frames, object.StackFrame{
			Function: funcName,
			Location: loc,
		})
	}
	return frames
}

// getCurrentLocation returns the source location of the current instruction.
func (vm *VirtualMachine) getCurrentLocation() object.SourceLocation {
	if vm.activeCode == nil {
		return object.SourceLocation{}
	}
	ip := vm.ip - 1 // Current instruction (ip was already incremented)
	if ip < 0 {
		ip = 0
	}
	return vm.activeCode.LocationAt(ip)
}

// runtimeError creates a StructuredError with source location and stack trace.
func (vm *VirtualMachine) runtimeError(kind object.ErrorKind, format string, args ...any) *object.StructuredError {
	return object.NewStructuredErrorf(kind, vm.getCurrentLocation(), vm.captureStack(), format, args...)
}

// typeError creates a type error with location and stack trace.
func (vm *VirtualMachine) typeError(format string, args ...any) *object.StructuredError {
	return vm.runtimeError(object.ErrType, format, args...)
}

// evalError creates an evaluation error with location and stack trace.
func (vm *VirtualMachine) evalError(format string, args ...any) *object.StructuredError {
	return vm.runtimeError(object.ErrRuntime, format, args...)
}

// handleException handles a thrown exception by finding an appropriate handler.
// If no handler is found, the error is returned to propagate up the call stack.
func (vm *VirtualMachine) handleException(errObj *object.Error) error {
	// Look for an exception handler on the exception stack
	for vm.excStackSize > 0 {
		excFrame := &vm.excStack[vm.excStackSize-1]

		// Check if this handler is for the current frame
		if excFrame.fp != vm.fp || excFrame.code != vm.activeCode {
			// Handler is for a different frame
			if excFrame.fp > vm.fp {
				// Handler is for a deeper frame we've returned from - pop it as stale
				vm.excStackSize--
				continue
			}
			// Handler is for a caller frame - let error propagate up
			// The caller's tryHandleError will find this handler after frame is restored
			return errObj.Value()
		}

		handler := excFrame.handler

		// If we have a catch block, it handles the exception
		// When catch completes normally, exception is considered handled
		if handler.CatchStart > 0 && handler.CatchStart != handler.FinallyStart {
			// Pop the exception handler since catch will handle it
			vm.excStackSize--

			// Push error onto stack for catch block
			vm.push(errObj)
			vm.ip = handler.CatchStart
			return nil
		}

		// No catch block, but we have finally - run finally with pending error
		if handler.FinallyStart > 0 && !excFrame.inFinally {
			// Store the pending error for re-raising after finally
			excFrame.pendingError = errObj
			excFrame.inFinally = true

			// Go to finally
			vm.ip = handler.FinallyStart
			return nil
		}

		// No handler available, pop and try next
		vm.excStackSize--
	}

	// No handler found, return the error to propagate up
	return errObj.Value()
}

// tryHandleError attempts to handle an error via exception handling.
// If a handler is found and jumped to, returns nil (exception was handled).
// If no handler is found, returns the error to propagate up.
func (vm *VirtualMachine) tryHandleError(err error) error {
	// Convert error to object.Error
	errObj := object.NewError(err)
	return vm.handleException(errObj)
}
