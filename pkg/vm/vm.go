// Package vm provides a VirtualMachine that executes compiled Risor code.
package vm

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/deepnoodle-ai/risor/v2/pkg/bytecode"
	"github.com/deepnoodle-ai/risor/v2/pkg/object"
	"github.com/deepnoodle-ai/risor/v2/pkg/op"
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

	// InitialExcStackCapacity is the initial size of the exception stack.
	// It grows dynamically up to excStackLimit as needed.
	InitialExcStackCapacity = 8

	// MaxExcHandlersPerFrame bounds the exception stack relative to the
	// call frame depth limit (on average, per frame).
	MaxExcHandlersPerFrame = 4

	// DefaultContextCheckInterval is the number of instructions between
	// deterministic checks of ctx.Done(). Set to 0 to disable.
	DefaultContextCheckInterval = 1000

	// DefaultMaxLocals is the default maximum number of local variable slots
	// across all active function call frames. Each slot is a 16-byte
	// interface value on 64-bit platforms, so 1<<22 (4,194,304) slots is
	// 64 MiB, matching object.DefaultMaxListLength. Compiled source rarely
	// uses more than a few dozen slots per function, so a full stack of
	// MaxFrameDepth frames fits with room to spare. Loaded bytecode can use
	// up to 65,535 slots per function, so without this bound a deep
	// recursion could reserve about 1 GB.
	DefaultMaxLocals = 1 << 22
)

var (
	ErrGlobalNotFound    = errors.New("global not found")
	ErrStepLimitExceeded = errors.New("step limit exceeded")
	ErrStackOverflow     = errors.New("stack overflow")

	// ErrReplacedCode is returned when a function from code that RunCode
	// replaced is called, and the globals of that code are laid out
	// differently from the globals of the loaded code.
	ErrReplacedCode = errors.New("function belongs to code that RunCode replaced")

	// ErrObserverHalt is returned when an observer callback returns false.
	ErrObserverHalt = errors.New("execution halted by observer")
)

type VirtualMachine struct {
	ip           int // instruction pointer
	sp           int // stack pointer
	fp           int // frame pointer
	halt         int32
	startCount   int64
	activeFrame  *frame
	activeCode   *loadedCode
	main         *bytecode.Code
	inputGlobals map[string]any
	globals      map[string]object.Object
	loadedCode   map[*bytecode.Code]*loadedCode
	running      bool
	runMutex     sync.Mutex
	tmp          [MaxArgs + 2]object.Object // args + rest param + function name
	stack        [MaxStackDepth]object.Object
	frames       []frame // Dynamically sized, grows up to MaxFrameDepth

	// stopWatcher ends the goroutine that watches the context of the
	// current run. It is nil when no watcher is running.
	stopWatcher func()

	// done is the Done channel of the context of the current run. It is nil
	// between runs and when the context cannot be canceled.
	done <-chan struct{}

	// requestedIP stores the starting instruction pointer requested via
	// WithInstructionOffset. This survives resetForNewCode() and is applied
	// when activating code. Used by REPL to skip past previously executed code.
	requestedIP int

	// contextCheckInterval is the number of instructions between deterministic
	// checks of ctx.Done(). A value of 0 disables deterministic checking,
	// relying only on the background goroutine. Default is DefaultContextCheckInterval.
	contextCheckInterval int

	// observer receives callbacks for VM execution events (steps, calls, returns).
	// If nil, no callbacks are made.
	observer Observer

	// observerConfig caches the normalized config from the observer.
	observerConfig ObserverConfig

	// Observer state for StepSampled and StepOnLine modes.
	sampleCount      int         // Counter for StepSampled mode
	lastObservedCode *loadedCode // Code object from last OnStep (changes on function call/return)
	lastObservedLine int         // Source line from last OnStep

	// Exception handling state
	excStack     []exceptionFrame
	excStackSize int

	// panicStack stores the stack trace captured during panic unwind.
	// This is populated by defer functions before they restore the frame pointer,
	// ensuring we preserve the full call stack when a panic occurs.
	panicStack []object.StackFrame

	// typeRegistry handles Go/Risor type conversions.
	// If nil, object.DefaultRegistry() is used.
	typeRegistry *object.TypeRegistry

	// limits bounds the size of values a single operation may produce. When
	// nil, limits already in the context (or the defaults) apply.
	limits *object.Limits

	// Resource limits
	maxSteps int64 // Maximum instructions. 0 = unlimited.
	// maxValueStackDepth limits the value stack depth (vm.sp).
	// A value of 0 uses the global MaxStackDepth constant.
	maxValueStackDepth int
	// valueStackLimit is the effective value stack limit for the current
	// run: maxValueStackDepth clamped to MaxStackDepth. push checks it.
	valueStackLimit int
	// maxFrameDepth limits the call frame depth (vm.fp).
	// A value of 0 uses the global MaxFrameDepth constant.
	maxFrameDepth int
	// maxLocals limits the total local variable slots across all active
	// function call frames. A value of 0 uses DefaultMaxLocals.
	maxLocals int
	timeout   time.Duration // Execution timeout. 0 = no timeout.

	// Step counting state for resource limits. These fields are stored on the
	// VM (rather than as local variables in eval) so that step counting persists
	// across recursive eval calls. This is important because methods like
	// list.each() and list.map() invoke callbacks via callFunction, which calls
	// eval recursively. Without VM-level counters, each callback would start
	// with fresh counters, allowing infinite iterations to bypass step limits.
	//
	// Steps are counted in batches for performance. A batch ends after
	// checkEvery instructions, or sooner if fewer steps remain under
	// maxSteps, so the step limit is exact.
	stepCount        int64 // Instructions counted at the end of earlier batches
	stepCheckCounter int   // Instructions since last periodic check
	checkEvery       int   // Instructions between periodic checks. 0 = never.
}

// exceptionFrame represents an active exception handler on the exception stack.
type exceptionFrame struct {
	handler       *bytecode.ExceptionHandler
	code          *loadedCode   // The code object containing this handler
	fp            int           // Frame pointer when handler was pushed
	sp            int           // Stack pointer when handler was pushed
	pendingError  *object.Error // Error to re-throw after finally (if any)
	pendingReturn object.Object // Value to return after finally (if any)
	inCatch       bool          // Are we currently executing a catch block?
	inFinally     bool          // Are we currently executing a finally block?
}

// New creates a new Virtual Machine with the given bytecode and options.
func New(main *bytecode.Code, options ...Option) (*VirtualMachine, error) {
	vm, err := createVM(options)
	if err != nil {
		return nil, err
	}
	vm.main = main
	return vm, nil
}

// NewEmpty creates a new Virtual Machine without initial main code.
// Code is provided later using RunCode. Call needs code loaded with RunCode
// first, and returns an error until then.
func NewEmpty() (*VirtualMachine, error) {
	return createVM(nil)
}

func createVM(options []Option) (*VirtualMachine, error) {
	vm := &VirtualMachine{
		sp:                   -1,
		inputGlobals:         map[string]any{},
		globals:              map[string]object.Object{},
		loadedCode:           map[*bytecode.Code]*loadedCode{},
		contextCheckInterval: DefaultContextCheckInterval,
		frames:               make([]frame, InitialFrameCapacity),
		excStack:             make([]exceptionFrame, InitialExcStackCapacity),
		valueStackLimit:      MaxStackDepth,
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

	// Save the globals of the loaded code first, so that the globals given
	// now replace them.
	vm.saveGlobals()

	// Apply options. inputGlobals collects only the globals given now.
	vm.inputGlobals = map[string]any{}
	for _, opt := range options {
		opt(vm)
	}

	// Configure observer if present
	vm.configureObserver()

	// Convert globals to Risor objects using the type registry. A provided
	// global replaces the value the VM holds for that name, including a
	// value that a script changed.
	globals, err := object.AsObjectsWithRegistry(vm.inputGlobals, vm.TypeRegistry())
	if err != nil {
		return fmt.Errorf("invalid global provided: %v", err)
	}
	for name, value := range globals {
		vm.globals[name] = value
	}
	return nil
}

// configureObserver initializes the observer configuration from the observer.
func (vm *VirtualMachine) configureObserver() {
	if vm.observer == nil {
		return
	}
	vm.observerConfig = NormalizeConfig(vm.observer.Config())
}

// SetObserverConfig updates the observer configuration on a paused VM.
// Returns an error if the VM is currently running.
func (vm *VirtualMachine) SetObserverConfig(cfg ObserverConfig) error {
	vm.runMutex.Lock()
	defer vm.runMutex.Unlock()
	if vm.running {
		return errors.New("cannot update observer config while VM is running")
	}
	vm.observerConfig = NormalizeConfig(cfg)
	return nil
}

// dispatchObserver handles OnStep callbacks based on the observer config.
// Returns an error if the observer halts execution.
func (vm *VirtualMachine) dispatchObserver(opcode op.Code) error {
	if vm.observer == nil {
		return nil
	}

	cfg := vm.observerConfig
	var shouldStep bool
	var loc object.SourceLocation

	switch cfg.StepMode {
	case StepAll:
		shouldStep = true
	case StepNone:
		return nil
	case StepSampled:
		vm.sampleCount++
		if vm.sampleCount >= cfg.SampleInterval {
			vm.sampleCount = 0
			shouldStep = true
		}
	case StepOnLine:
		shouldStep, loc = vm.checkLineChanged()
	}

	if !shouldStep {
		return nil
	}

	// For modes other than StepOnLine, loc is not set yet
	if loc.Line == 0 {
		loc = vm.activeCode.LocationAt(vm.ip)
	}

	event := StepEvent{
		IP:         vm.ip,
		Opcode:     opcode,
		OpcodeName: op.GetInfo(opcode).Name,
		Location:   loc,
		StackDepth: vm.sp + 1,
		FrameDepth: vm.fp + 1,
	}
	if !vm.observer.OnStep(event) {
		return ErrObserverHalt
	}
	return nil
}

// checkLineChanged returns true if the source location has changed since
// the last OnStep call, along with the current location.
func (vm *VirtualMachine) checkLineChanged() (bool, object.SourceLocation) {
	loc := vm.activeCode.LocationAt(vm.ip)

	// Skip invalid locations (line 0 indicates no source info).
	// This suppresses OnStep events until a valid line is reached.
	if loc.Line == 0 {
		return false, loc
	}

	// Check if code object or line changed
	if vm.activeCode != vm.lastObservedCode || loc.Line != vm.lastObservedLine {
		vm.lastObservedCode = vm.activeCode
		vm.lastObservedLine = loc.Line
		return true, loc
	}
	return false, loc
}

func (vm *VirtualMachine) start(ctx context.Context) error {
	vm.runMutex.Lock()
	defer vm.runMutex.Unlock()
	if vm.running {
		return fmt.Errorf("vm is already running")
	}
	vm.running = true
	vm.startCount++
	// Each run has its own step budget
	vm.stepCount = 0
	vm.stepCheckCounter = 0
	vm.checkEvery = vm.nextCheckInterval()
	vm.valueStackLimit = vm.maxValueStackDepth
	if vm.valueStackLimit <= 0 || vm.valueStackLimit > MaxStackDepth {
		vm.valueStackLimit = MaxStackDepth
	}
	// Halt execution when the context is cancelled. The watcher goroutine
	// ends when the run stops, so a later cancellation of this context
	// cannot halt another run.
	atomic.StoreInt32(&vm.halt, 0)
	vm.done = ctx.Done()
	if doneChan := vm.done; doneChan != nil {
		stopChan := make(chan struct{})
		exited := make(chan struct{})
		go func() {
			defer close(exited)
			select {
			case <-doneChan:
				atomic.StoreInt32(&vm.halt, 1)
			case <-stopChan:
			}
		}()
		vm.stopWatcher = func() {
			close(stopChan)
			<-exited
		}
	}
	return nil
}

// nextCheckInterval returns the number of instructions to run before the
// next periodic check. The check runs every contextCheckInterval
// instructions, and also on the first instruction past the step limit.
// It returns 0 if no check is needed.
func (vm *VirtualMachine) nextCheckInterval() int {
	interval := int64(vm.contextCheckInterval)
	if interval < 0 {
		interval = 0
	}
	if vm.maxSteps > 0 {
		// Check on the instruction that would exceed the limit
		remaining := vm.maxSteps - vm.stepCount + 1
		if remaining < 1 {
			remaining = 1
		}
		if interval == 0 || remaining < interval {
			interval = remaining
		}
	}
	return int(interval)
}

// periodicCheck runs every checkEvery instructions. It checks the context
// and the step limit, and sets the length of the next batch.
func (vm *VirtualMachine) periodicCheck(ctx context.Context) error {
	steps := vm.stepCheckCounter
	vm.stepCheckCounter = 0

	// Context cancellation check
	if doneChan := ctx.Done(); doneChan != nil {
		select {
		case <-doneChan:
			atomic.StoreInt32(&vm.halt, 1)
			return ctx.Err()
		default:
		}
	}

	// Step limit check. The instruction about to run is counted in steps.
	if vm.maxSteps > 0 {
		vm.stepCount += int64(steps)
		if vm.stepCount > vm.maxSteps {
			// Fail every later instruction too, so the limit holds even if
			// a caller keeps running after this error.
			vm.checkEvery = 1
			return ErrStepLimitExceeded
		}
	}
	vm.checkEvery = vm.nextCheckInterval()
	return nil
}

func (vm *VirtualMachine) stop() {
	vm.runMutex.Lock()
	defer vm.runMutex.Unlock()
	if vm.stopWatcher != nil {
		vm.stopWatcher()
		vm.stopWatcher = nil
	}
	vm.done = nil
	vm.running = false
}

// TypeRegistry returns the VM's type registry for Go/Risor conversions.
// If no registry was configured, returns the default registry.
func (vm *VirtualMachine) TypeRegistry() *object.TypeRegistry {
	if vm.typeRegistry == nil {
		return object.DefaultRegistry()
	}
	return vm.typeRegistry
}

func (vm *VirtualMachine) Run(ctx context.Context) (err error) {
	if vm.main == nil {
		return fmt.Errorf("no main code available: Run requires code given to New, or use RunCode")
	}
	return vm.runCodeInternal(ctx, vm.main, false)
}

// RunCode runs the given compiled code object on the VM. This allows running
// multiple different code objects on the same VM instance sequentially.
// The VM must not be currently running when this method is called.
//
// Globals carry over between runs by name, with the values that earlier runs
// left in them. A global given with WithGlobals replaces the value for that
// name. A function from replaced code can still be called if its globals are
// laid out the same way as the globals of the new code. Otherwise the call
// fails with ErrReplacedCode.
func (vm *VirtualMachine) RunCode(ctx context.Context, codeToRun *bytecode.Code, opts ...Option) (err error) {
	if err := vm.applyOptions(opts); err != nil {
		return err
	}
	return vm.runCodeInternal(ctx, codeToRun, true)
}

// runCodeInternal is the shared implementation for Run and RunCode
func (vm *VirtualMachine) runCodeInternal(ctx context.Context, codeToRun *bytecode.Code, resetState bool) (err error) {
	// Apply timeout to context if configured
	if vm.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, vm.timeout)
		defer cancel()
	}

	// Set up some guarantees:
	// 1. It is an error to call Run on a VM that is already running
	// 2. The running flag will always be set to false when Run returns
	// 3. Any panics are translated to errors and the VM is stopped
	if err := vm.start(ctx); err != nil {
		return err
	}
	defer func() {
		if r := recover(); r != nil {
			err = vm.panicToError(r)
		}
		vm.stop()
	}()

	// Reset VM state for new code execution if requested
	if resetState && vm.startCount > 1 {
		vm.resetForNewCode()
	}
	// A previous run that ended inside a try block may have left its
	// handlers behind. No script frames are active between runs, so drop them.
	vm.excStackSize = 0

	// Update main if we're running different code (e.g., via RunCode)
	if resetState && codeToRun != vm.main {
		vm.main = codeToRun
	}

	// Load the code to run - unified logic for both paths
	var codeObj *loadedCode

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
	for i := 0; i < codeToRun.ConstantCount(); i++ {
		if fn, ok := codeToRun.ConstantAt(i).(*bytecode.Function); ok {
			vm.loadCode(fn.Code())
		}
	}

	// Activate the entrypoint code in frame zero.
	// For Run() (resetState=false), use vm.ip to continue from where we left off.
	// For RunCode() (resetState=true), use requestedIP if set (via WithInstructionOffset),
	// otherwise start from 0. The REPL uses WithInstructionOffset to skip past
	// previously executed (or errored) code in incremental compilation.
	startIP := 0
	if !resetState {
		startIP = vm.ip
	} else if vm.requestedIP > 0 {
		startIP = vm.requestedIP
		vm.requestedIP = 0 // Clear after use
	}
	if _, err := vm.activateCode(0, startIP, codeObj); err != nil {
		return err
	}

	// Run the entrypoint until completion
	return vm.eval(vm.initContext(ctx))
}

// resetForNewCode resets the VM state for running a new code object
// while preserving the globals of previous runs. The caller must save the
// globals with saveGlobals first.
func (vm *VirtualMachine) resetForNewCode() {
	vm.sp = -1
	vm.ip = 0
	vm.fp = 0
	vm.activeFrame = nil
	vm.activeCode = nil
	vm.loadedCode = map[*bytecode.Code]*loadedCode{}
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
	for i := range vm.tmp {
		vm.tmp[i] = nil
	}
}

// saveGlobals copies the current values of the main code's globals into
// vm.globals, which holds the globals by name across runs.
func (vm *VirtualMachine) saveGlobals() {
	main, ok := vm.loadedCode[vm.main]
	if !ok {
		return
	}
	for i, value := range main.Globals {
		if value != nil {
			vm.globals[main.GlobalNameAt(i)] = value
		}
	}
}

// Get a root (top-level) global variable by name as a Risor Object. A
// variable declared in a block, such as in an if or try block, is not a global.
func (vm *VirtualMachine) Get(name string) (object.Object, error) {
	code := vm.activeCode
	if code == nil {
		return nil, errors.New("no active code")
	}
	if bytecode.IsBlockGlobalName(name) {
		return nil, fmt.Errorf("%w: %q", ErrGlobalNotFound, name)
	}
	globalCount := code.GlobalCount()
	for i := 0; i < globalCount; i++ {
		if code.GlobalNameAt(i) == name {
			return code.Globals[i], nil
		}
	}
	return nil, fmt.Errorf("%w: %q", ErrGlobalNotFound, name)
}

// GlobalNames returns the names of the root (top-level) global variables in
// the active code. Variables declared in blocks are not included.
func (vm *VirtualMachine) GlobalNames() []string {
	code := vm.activeCode
	if code == nil {
		return nil
	}
	return code.GlobalNames()
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
evalLoop:
	for vm.ip < len(vm.activeCode.Instructions) {

		if atomic.LoadInt32(&vm.halt) == 1 {
			return ctx.Err()
		}

		// Periodic checks (context, steps) every checkEvery instructions.
		// This amortizes the cost of resource limit checking.
		// Using VM fields ensures counts persist across recursive eval calls
		// (e.g., when callbacks are invoked via list.each(), list.map(), etc.)
		if vm.checkEvery > 0 {
			vm.stepCheckCounter++
			if vm.stepCheckCounter >= vm.checkEvery {
				if err := vm.periodicCheck(ctx); err != nil {
					return err
				}
			}
		}

		// The current instruction opcode
		opcode := vm.activeCode.Instructions[vm.ip]

		// fmt.Println("ip", vm.ip, op.GetInfo(opcode).Name, "sp", vm.sp)

		// Dispatch observer callbacks based on observer config
		if err := vm.dispatchObserver(opcode); err != nil {
			return err
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
			idx := int(vm.fetch())
			fn := vm.activeFrame.fn
			if fn == nil || idx >= fn.FreeVarCount() {
				if herr := vm.tryHandleError(vm.evalError("no free variable at index %d", idx)); herr != nil {
					return herr
				}
				continue
			}
			vm.push(fn.FreeVar(idx).Value())
		case op.StoreFast:
			idx := vm.fetch()
			obj := vm.pop()
			vm.activeFrame.Locals()[idx] = obj
		case op.StoreGlobal:
			vm.activeCode.Globals[vm.fetch()] = vm.pop()
		case op.StoreFree:
			idx := int(vm.fetch())
			obj := vm.pop()
			fn := vm.activeFrame.fn
			if fn == nil || idx >= fn.FreeVarCount() {
				if herr := vm.tryHandleError(vm.evalError("no free variable at index %d", idx)); herr != nil {
					return herr
				}
				continue
			}
			fn.FreeVar(idx).Set(obj)
		case op.StoreAttr:
			idx := vm.fetch()
			obj := vm.pop()
			value := vm.pop()
			name := vm.activeCode.Names[idx]
			var err error
			// *GoStruct gets the ctx directly. Any other type gets it only
			// if it implements AttrAssigner. *GoStruct does not define
			// AssignAttr, so a type that embeds it cannot get AssignAttr
			// by promotion, and a SetAttr override on that type still runs.
			switch o := obj.(type) {
			case *object.GoStruct:
				err = o.SetAttrContext(ctx, name, value)
			case object.AttrAssigner:
				err = o.AssignAttr(ctx, name, value)
			default:
				err = obj.SetAttr(name, value)
			}
			if err != nil {
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
					continue evalLoop
				}
			}
			fn := vm.activeCode.Constants[constIndex].(*object.Closure)
			vm.push(object.CloneWithCaptures(fn, free))
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
			locals := vm.frames[frameIndex].CaptureLocals()
			if int(symbolIndex) >= len(locals) {
				if herr := vm.tryHandleError(vm.evalError("no local at index %d", symbolIndex)); herr != nil {
					return herr
				}
				continue
			}
			vm.push(object.NewCell(&locals[symbolIndex]))
		case op.LoadCell:
			idx := int(vm.fetch())
			fn := vm.activeFrame.fn
			if fn == nil || idx >= fn.FreeVarCount() {
				if herr := vm.tryHandleError(vm.evalError("no free variable at index %d", idx)); herr != nil {
					return herr
				}
				continue
			}
			vm.push(fn.FreeVar(idx))
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
			result, err := object.BinaryOp(ctx, opType, a, b)
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
			if len(args) > MaxArgs {
				if herr := vm.tryHandleError(vm.evalError("max args limit of %d exceeded (got %d)",
					MaxArgs, len(args))); herr != nil {
					return herr
				}
				continue
			}
			if err := vm.callObject(ctx, obj, args); err != nil {
				if herr := vm.tryHandleError(err); herr != nil {
					return herr
				}
				continue
			}
		case op.ReturnValue:
			// Run any pending finally blocks in this function first. The
			// return completes when the outermost finally reaches EndFinally.
			if vm.unwindForReturn() {
				continue evalLoop
			}
			if stop, err := vm.returnFromFrame(); err != nil || stop {
				return err
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
			if err := object.CheckListLength(ctx, int(count)); err != nil {
				vm.restoreStack(vm.sp - int(count))
				if herr := vm.tryHandleError(err); herr != nil {
					return herr
				}
				continue
			}
			items := make([]object.Object, count)
			for i := uint16(0); i < count; i++ {
				items[count-1-i] = vm.pop()
			}
			vm.push(object.NewList(items))
		case op.BuildMap:
			// Insert the pairs in source order, so that a later duplicate
			// key wins, as it does for MapSet.
			count := int(vm.fetch())
			base := vm.sp - count*2 + 1
			items := make(map[string]object.Object, count)
			for i := 0; i < count; i++ {
				k := vm.stack[base+i*2]
				v := vm.stack[base+i*2+1]
				items[k.(*object.String).Value()] = v
			}
			vm.restoreStack(base - 1)
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
			if err := object.CheckListLength(ctx, len(list.Value())+1); err != nil {
				if herr := vm.tryHandleError(err); herr != nil {
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
			enumerable, ok := iterableObj.(object.Enumerable)
			if !ok {
				if herr := vm.tryHandleError(vm.typeError("spread requires an enumerable (got %s)", iterableObj.Type())); herr != nil {
					return herr
				}
				continue
			}
			// For maps, spread yields keys; for other containers, spread yields
			// values. Collecting enforces the list length limit and ctx.
			var items []object.Object
			var err error
			if _, isMap := iterableObj.(*object.Map); isMap {
				items, err = object.CollectKeys(ctx, enumerable)
			} else {
				items, err = object.CollectValues(ctx, enumerable)
			}
			existing := list.Value()
			if err == nil {
				err = object.CheckListLength(ctx, len(existing)+len(items))
			}
			if err != nil {
				if herr := vm.tryHandleError(err); herr != nil {
					return herr
				}
				continue
			}
			newItems := make([]object.Object, 0, len(existing)+len(items))
			newItems = append(newItems, existing...)
			newItems = append(newItems, items...)
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
				if herr := vm.tryHandleError(err); herr != nil {
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
				if herr := vm.tryHandleError(err); herr != nil {
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
			// Check before formatting, so that an operand whose text is too
			// large, such as a list that holds one string many times, is
			// never formatted
			operands := vm.stack[vm.sp-int(count)+1 : vm.sp+1]
			if err := object.CheckFormatSize(ctx, "string template", operands...); err != nil {
				vm.restoreStack(vm.sp - int(count))
				if herr := vm.tryHandleError(err); herr != nil {
					return herr
				}
				continue
			}
			items := make([]string, count)
			size := 0
			for i := uint16(0); i < count; i++ {
				dst := count - 1 - i
				obj := vm.pop()
				switch obj := obj.(type) {
				case *object.Error:
					// Errors are values - stringify them in templates
					items[dst] = obj.String()
				case *object.String:
					items[dst] = obj.Value()
				default:
					items[dst] = obj.Inspect()
				}
				size += len(items[dst])
			}
			if err := object.CheckBytesSize(ctx, "string template", size); err != nil {
				if herr := vm.tryHandleError(err); herr != nil {
					return herr
				}
				continue
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
				if herr := vm.tryHandleError(err); herr != nil {
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
			// Exactly nameCount values are pushed below
			if int64(vm.sp)+nameCount >= int64(vm.valueStackLimit) {
				return ErrStackOverflow
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
			var handler *bytecode.ExceptionHandler
			for i := range vm.activeCode.ExceptionHandlers {
				h := &vm.activeCode.ExceptionHandlers[i]
				if h.TryStart <= baseIP && baseIP < h.TryEnd {
					handler = h
					break
				}
			}

			if handler == nil {
				// Create a temporary handler based on offsets
				handler = &bytecode.ExceptionHandler{
					TryStart:    baseIP,
					CatchStart:  baseIP + int(catchOffset),
					CatchVarIdx: -1,
				}
				if finallyOffset > 0 {
					handler.FinallyStart = baseIP + int(finallyOffset)
				}
			}

			if err := vm.pushExceptionFrame(handler); err != nil {
				if herr := vm.tryHandleError(err); herr != nil {
					return herr
				}
				continue
			}
		case op.PopExcept:
			// Normal completion of a try or catch block. If the handler has a
			// finally block, keep the frame and mark it as running finally so
			// that EndFinally can pop it and a throw or return inside finally
			// does not re-enter this handler. Otherwise, pop the handler.
			if vm.excStackSize > 0 {
				excFrame := &vm.excStack[vm.excStackSize-1]
				if excFrame.handler.FinallyStart > 0 {
					excFrame.inCatch = false
					excFrame.inFinally = true
				} else {
					vm.popExceptionFrame()
				}
			}
		case op.Throw:
			// Throw the value on TOS as an exception
			tosObj := vm.pop()

			// Convert to an error with the location of the throw. A
			// re-thrown error keeps the location where it was first raised.
			var thrown error
			switch v := tosObj.(type) {
			case *object.Error:
				thrown = v
			case *object.String:
				thrown = errors.New(v.Value())
			default:
				thrown = errors.New(tosObj.Inspect())
			}
			errObj := object.NewError(vm.wrapError(thrown))

			// Handle the exception
			if err := vm.handleException(errObj); err != nil {
				return err
			}
		case op.EndFinally:
			// End of a finally block. The handler's frame is on top of the
			// exception stack in the inFinally state; pop it, then complete
			// any pending return or re-raise any pending exception.
			if vm.excStackSize == 0 {
				continue
			}
			excFrame := &vm.excStack[vm.excStackSize-1]
			if !excFrame.inFinally || excFrame.fp != vm.fp || excFrame.code != vm.activeCode {
				continue
			}
			pendingReturn := excFrame.pendingReturn
			pendingErr := excFrame.pendingError
			vm.popExceptionFrame()

			if pendingErr != nil {
				if err := vm.handleException(pendingErr); err != nil {
					return err
				}
				continue
			}
			if pendingReturn != nil {
				// Continue the return, running any outer finally blocks
				vm.push(pendingReturn)
				if vm.unwindForReturn() {
					continue evalLoop
				}
				if stop, err := vm.returnFromFrame(); err != nil || stop {
					return err
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

// push places obj on top of the value stack. Exceeding the value stack limit
// panics with ErrStackOverflow, which panicToError reports as-is.
func (vm *VirtualMachine) push(obj object.Object) {
	if vm.sp+1 >= vm.valueStackLimit {
		panic(ErrStackOverflow)
	}
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
// running, an error is returned. Call needs main code, given to New or loaded
// with RunCode, and returns an error if the VM has none.
//
// The function must come from the loaded main code, or from code whose
// globals are laid out the same way, as in a REPL where each input extends
// the previous one. A function from code that RunCode replaced, with a
// different layout, returns ErrReplacedCode.
func (vm *VirtualMachine) Call(
	ctx context.Context,
	fn *object.Closure,
	args []object.Object,
) (result object.Object, err error) {
	if err := vm.start(ctx); err != nil {
		return nil, err
	}
	defer func() {
		if r := recover(); r != nil {
			err = vm.panicToError(r)
		}
		vm.stop()
	}()
	// The main code owns the globals array that function code shares, so it
	// must be loaded even if Run has not been called yet.
	if vm.main == nil {
		return nil, fmt.Errorf("no main code available: Call requires code given to New, or loaded with RunCode")
	}
	vm.loadCode(vm.main)
	// A previous run that was halted inside a try block may have left its
	// handlers behind. No script frames are active between runs, so drop them.
	vm.excStackSize = 0
	return vm.callFunction(vm.initContext(ctx), fn, args)
}

// callFunction executes a compiled function with the given arguments. This is
// used internally when a Risor object invokes a callback, e.g.:
//
//	[1, 2, 3].map(func(x) { x + 1 })
//
// The function calls vm.eval() recursively, which means step counting and
// resource limits apply to callback execution. The VM's stepCount field
// persists across these recursive calls, ensuring that callbacks cannot
// bypass step limits by executing in a "fresh" eval context.
func (vm *VirtualMachine) callFunction(
	ctx context.Context,
	fn *object.Closure,
	args []object.Object,
) (result object.Object, resultErr error) {
	// Check that the argument count is appropriate
	paramsCount := fn.ParameterCount()
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

	// Restore the previous frame when done. The result is returned to the
	// Go caller rather than left on the stack, so anything above baseSP is
	// discarded. If panicking, capture the stack trace before restoring.
	defer func() {
		if r := recover(); r != nil {
			// Capture stack while frames are still intact
			if vm.panicStack == nil {
				vm.panicStack = vm.captureStack()
			}
			vm.restoreFrame(baseFP, baseIP, baseSP)
			panic(r) // Re-panic to continue unwinding
		}
		vm.restoreFrame(baseFP, baseIP, baseSP)
	}()

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

		vm.fillDefaults(fn, copyCount, paramsCount)

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
		vm.fillDefaults(fn, argc, paramsCount)
		localCount = paramsCount
	}

	code := fn.Code()
	if code.IsNamed() {
		vm.tmp[localCount] = fn
		localCount++
	}
	argc = localCount

	// Activate a frame for the function call
	if _, err := vm.activateFunction(vm.fp+1, 0, fn, vm.tmp[:argc]); err != nil {
		return nil, err
	}

	// Call observer if present and configured to observe calls
	if vm.observer != nil && vm.observerConfig.ObserveCalls {
		event := CallEvent{
			FunctionName: fn.Name(),
			ArgCount:     len(args),
			Location:     vm.getCurrentLocation(),
			FrameDepth:   vm.fp + 1,
		}
		if !vm.observer.OnCall(event) {
			return nil, ErrObserverHalt
		}
	}

	// Setting StopSignal as the return address will cause the eval function to
	// stop execution when it reaches the end of the active code.
	vm.activeFrame.returnAddr = StopSignal

	// Evaluate the function code then return the result from TOS
	if err := vm.eval(ctx); err != nil {
		return nil, err
	}
	if vm.sp <= baseSP {
		return object.Nil, nil
	}
	return vm.pop(), nil
}

// fillDefaults sets vm.tmp[from:to], the parameters the caller did not pass,
// to their defaults. A parameter without a default gets Nil, so a frame never
// sees a raw Go nil or a value left in vm.tmp by an earlier call.
func (vm *VirtualMachine) fillDefaults(fn *object.Closure, from, to int) {
	for i := from; i < to; i++ {
		value := fn.Default(i)
		if value == nil {
			value = object.Nil
		}
		vm.tmp[i] = value
	}
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
	case *object.Closure:
		result, err := vm.callFunction(ctx, fn, args)
		if err != nil {
			return err
		}
		vm.push(result)
		return nil
	case object.Callable:
		result, err := fn.Call(ctx, args...)
		if err != nil {
			return err
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
// The value on top of the stack (the return value of the previous frame) is
// carried over onto the resumed frame's stack.
func (vm *VirtualMachine) resumeFrame(fp, ip, sp int) *frame {
	// The return value of the previous frame is on the top of the stack
	var frameResult object.Object = nil
	if vm.sp > sp {
		frameResult = vm.pop()
	}
	vm.restoreFrame(fp, ip, sp)
	// Push the frame result back onto the stack
	if frameResult != nil {
		vm.push(frameResult)
	}
	return vm.activeFrame
}

// restoreFrame activates the frame at the given frame pointer, restoring the
// given IP and SP. Any values above sp are discarded.
func (vm *VirtualMachine) restoreFrame(fp, ip, sp int) *frame {
	for i := vm.sp; i > sp; i-- {
		vm.stack[i] = nil
	}
	// Frames above fp are no longer active. Drop their large locals so that
	// only active frames, which the locals budget bounds, hold large slices.
	for i := fp + 1; i <= vm.fp && i < len(vm.frames); i++ {
		vm.frames[i].releaseLocals()
	}
	vm.sp = sp
	vm.fp = fp
	vm.ip = ip
	vm.activeFrame = &vm.frames[fp]
	vm.activeCode = vm.activeFrame.code
	return vm.activeFrame
}

// ensureFrameCapacity grows the frames slice if needed to accommodate the given frame index.
// Returns an error if the frame index exceeds the configured limit or MaxFrameDepth.
func (vm *VirtualMachine) ensureFrameCapacity(fp int) error {
	// Check against configured frame depth limit first (if set and smaller than MaxFrameDepth)
	if vm.maxFrameDepth > 0 && vm.maxFrameDepth < MaxFrameDepth && fp >= vm.maxFrameDepth {
		return ErrStackOverflow
	}
	if fp >= MaxFrameDepth {
		return ErrStackOverflow
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

// localsLimit returns the maximum total local variable slots across all
// active function call frames.
func (vm *VirtualMachine) localsLimit() int {
	if vm.maxLocals > 0 {
		return vm.maxLocals
	}
	return DefaultMaxLocals
}

// Activate a frame with the given code. This is typically used to begin
// running the entrypoint for a module or script. The entry frame does not
// count toward the locals budget.
func (vm *VirtualMachine) activateCode(fp, ip int, code *loadedCode) (*frame, error) {
	if err := vm.ensureFrameCapacity(fp); err != nil {
		return nil, err
	}
	vm.fp = fp
	vm.ip = ip
	vm.activeFrame = &vm.frames[fp]
	vm.activeFrame.ActivateCode(code)
	vm.activeFrame.localsTotal = 0
	vm.activeCode = code
	return vm.activeFrame, nil
}

// Activate a frame with the given function, to implement a function call.
func (vm *VirtualMachine) activateFunction(fp, ip int, fn *object.Closure, locals []object.Object) (*frame, error) {
	if err := vm.ensureFrameCapacity(fp); err != nil {
		return nil, err
	}
	code, ok := vm.loadedCode[fn.Code()]
	if !ok {
		// Function code reads globals by slot index, so it can only run
		// with the globals of the main code if the slots match.
		root := fn.Code().Root()
		if vm.main == nil || (root != vm.main && !sameGlobalLayout(root, vm.main)) {
			return nil, ErrReplacedCode
		}
		code = vm.loadCode(fn.Code())
	}
	// Check the locals budget before the frame allocates its locals. The
	// total is derived from the caller's frame, so returning or unwinding
	// to the caller restores the caller's total with no bookkeeping.
	localsTotal := code.LocalsCount()
	if fp > 0 {
		localsTotal += vm.frames[fp-1].localsTotal
	}
	if limit := vm.localsLimit(); localsTotal > limit {
		return nil, fmt.Errorf("%w: local variable limit of %d slots exceeded",
			ErrStackOverflow, limit)
	}
	returnAddr := vm.ip
	returnSp := vm.sp
	vm.fp = fp
	vm.ip = ip
	vm.activeFrame = &vm.frames[fp]
	vm.activeFrame.ActivateFunction(fn, code, returnAddr, returnSp, locals)
	vm.activeFrame.localsTotal = localsTotal
	vm.activeCode = code
	return vm.activeFrame, nil
}

// Wrap the *bytecode.Code in a *loadedCode object to make it usable by the VM.
func (vm *VirtualMachine) loadCode(bc *bytecode.Code) *loadedCode {
	if lc, ok := vm.loadedCode[bc]; ok {
		return lc
	}
	// Loading is slightly different if this is the "root" (entrypoint) code
	// vs. a child of that. The root code owns the globals array, while the
	// children will reuse the globals from the root.
	var c *loadedCode
	if vm.main == bc {
		c = loadRootCode(bc, vm.globals)
	} else {
		c = loadChildCode(vm.loadedCode[vm.main], bc)
	}
	vm.loadedCode[bc] = c
	return c
}

// sameGlobalLayout reports whether main names each global slot of root the
// same way root does.
func sameGlobalLayout(root, main *bytecode.Code) bool {
	if root.GlobalCount() > main.GlobalCount() {
		return false
	}
	for i := 0; i < root.GlobalCount(); i++ {
		if root.GlobalNameAt(i) != main.GlobalNameAt(i) {
			return false
		}
	}
	return true
}

// Reloads the main code while preserving global variables. This happens as
// part of a typical REPL workflow, where the main code is appended to with
// each new input.
func (vm *VirtualMachine) reloadCode(main *bytecode.Code) *loadedCode {
	oldWrappedMain, ok := vm.loadedCode[main]
	if !ok {
		panic("main code not loaded")
	}
	delete(vm.loadedCode, main)
	newWrappedMain := vm.loadCode(main)
	copy(newWrappedMain.Globals, oldWrappedMain.Globals)
	// Function code loaded earlier (e.g. by Call before Run) shares the old
	// globals slice. Point it at the new one so all code sees the same globals.
	for _, lc := range vm.loadedCode {
		lc.Globals = newWrappedMain.Globals
	}
	return newWrappedMain
}

func (vm *VirtualMachine) initContext(ctx context.Context) context.Context {
	if vm.limits != nil {
		ctx = object.WithLimits(ctx, *vm.limits)
	}
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
			funcName = "__main__"
		}

		// Get the location:
		// - Active frame: use current ip (where the error occurred)
		// - Caller frames: use callSiteIP from the callee frame (where the call was made)
		ip := 0
		if i == vm.fp {
			ip = vm.ip - 1 // Current instruction (ip was already incremented)
			if ip < 0 {
				ip = 0
			}
		} else if i < vm.fp {
			// For caller frames, the call site is stored in the callee's callSiteIP.
			// callSiteIP is captured as vm.ip after the Call instruction was read,
			// so subtract 1 to get the actual Call instruction's source location.
			calleeFrame := &vm.frames[i+1]
			ip = calleeFrame.callSiteIP - 1
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

// typeError creates a type error with location and stack trace. Its cause is
// a TypeError, so errors.As finds the kind.
func (vm *VirtualMachine) typeError(format string, args ...any) *object.StructuredError {
	se := vm.runtimeError(object.ErrType, format, args...)
	se.Cause = object.NewTypeError(errors.New(se.Message))
	return se
}

// evalError creates an evaluation error with location and stack trace.
func (vm *VirtualMachine) evalError(format string, args ...any) *object.StructuredError {
	return vm.runtimeError(object.ErrRuntime, format, args...)
}

// wrapError gives an error a source location and stack trace. Every runtime
// error goes through it, so Go callers and scripts see one error shape.
//
// An error that already has a location, such as one raised in a called
// function, is returned as is. Otherwise the result is a StructuredError at
// the current location. Its kind and message come from object.NewError, and
// its cause is the original error, so errors.As still finds it.
func (vm *VirtualMachine) wrapError(err error) *object.StructuredError {
	if e, ok := err.(*object.Error); ok {
		err = e.Value()
	}
	if se, ok := err.(*object.StructuredError); ok && !se.Location.IsZero() {
		return se
	}
	kind, msg := object.ErrRuntime, err.Error()
	loc, stack := vm.getCurrentLocation(), vm.captureStack()
	if s := object.NewError(err).Structured(); s != nil {
		kind, msg = s.Kind, s.Message
		if !s.Location.IsZero() {
			loc, stack = s.Location, s.Stack
		}
	}
	se := object.NewStructuredError(kind, msg, loc, stack)
	se.Cause = err
	return se
}

// panicToError converts a recovered panic value to a structured error.
// It attempts to categorize common Go runtime panics into user-friendly errors.
func (vm *VirtualMachine) panicToError(r any) error {
	// Check if it's one of our sentinel errors - return directly to preserve error chain
	if err, ok := r.(error); ok {
		if errors.Is(err, ErrStackOverflow) || errors.Is(err, ErrStepLimitExceeded) {
			return err
		}
	}

	msg := fmt.Sprintf("%v", r)

	// Categorize common Go runtime panics
	kind := object.ErrRuntime
	var friendlyMsg string

	switch {
	case strings.Contains(msg, "integer divide by zero"):
		kind = object.ErrValue
		friendlyMsg = "division by zero"
	case strings.Contains(msg, "index out of range"):
		kind = object.ErrValue
		friendlyMsg = "index out of range"
	case strings.Contains(msg, "slice bounds out of range"):
		kind = object.ErrValue
		friendlyMsg = "slice bounds out of range"
	case strings.Contains(msg, "nil pointer dereference"):
		kind = object.ErrValue
		friendlyMsg = "nil value access"
	case strings.Contains(msg, "invalid memory address"):
		kind = object.ErrValue
		friendlyMsg = "invalid memory access"
	default:
		friendlyMsg = msg
	}

	// Use the panic stack if it was captured during unwind, otherwise use current stack
	stack := vm.panicStack
	if stack == nil {
		stack = vm.captureStack()
	}

	// Get location from the first frame (where the panic occurred)
	var loc object.SourceLocation
	if len(stack) > 0 {
		loc = stack[0].Location
	} else {
		loc = vm.getCurrentLocation()
	}

	// Clear the panic stack for next use
	vm.panicStack = nil

	return object.NewStructuredError(kind, friendlyMsg, loc, stack)
}

// excStackLimit returns the maximum number of active exception handlers.
func (vm *VirtualMachine) excStackLimit() int {
	frameDepth := MaxFrameDepth
	if vm.maxFrameDepth > 0 && vm.maxFrameDepth < MaxFrameDepth {
		frameDepth = vm.maxFrameDepth
	}
	return frameDepth * MaxExcHandlersPerFrame
}

// pushExceptionFrame pushes a handler for the active frame onto the exception
// stack, growing the stack if needed. Returns an error if the limit is reached.
func (vm *VirtualMachine) pushExceptionFrame(handler *bytecode.ExceptionHandler) error {
	if vm.excStackSize >= len(vm.excStack) {
		limit := vm.excStackLimit()
		if vm.excStackSize >= limit {
			return fmt.Errorf("%w: exception handler depth limit of %d exceeded",
				ErrStackOverflow, limit)
		}
		newCap := min(max(len(vm.excStack)*2, InitialExcStackCapacity), limit)
		newStack := make([]exceptionFrame, newCap)
		copy(newStack, vm.excStack[:vm.excStackSize])
		vm.excStack = newStack
	}
	vm.excStack[vm.excStackSize] = exceptionFrame{
		handler: handler,
		code:    vm.activeCode,
		fp:      vm.fp,
		sp:      vm.sp,
	}
	vm.excStackSize++
	return nil
}

// popExceptionFrame removes the top exception frame, clearing its references.
func (vm *VirtualMachine) popExceptionFrame() {
	vm.excStackSize--
	vm.excStack[vm.excStackSize] = exceptionFrame{}
}

// restoreStack truncates the operand stack to the given stack pointer.
func (vm *VirtualMachine) restoreStack(sp int) {
	for i := vm.sp; i > sp; i-- {
		vm.stack[i] = nil
	}
	vm.sp = sp
}

// unwindForReturn prepares the active frame to return the value on TOS. It
// pops this frame's exception handlers, innermost first. Handlers already
// running finally are discarded, so a return inside finally overrides any
// pending return or exception. If a handler with a finally block that has not
// run is found, the return value is saved on it, execution jumps to its
// finally block, and true is returned. EndFinally resumes the return.
func (vm *VirtualMachine) unwindForReturn() bool {
	for vm.excStackSize > 0 {
		excFrame := &vm.excStack[vm.excStackSize-1]
		if excFrame.fp != vm.fp || excFrame.code != vm.activeCode {
			break
		}
		if excFrame.handler.FinallyStart > 0 && !excFrame.inFinally {
			excFrame.pendingReturn = vm.pop()
			excFrame.inCatch = false
			excFrame.inFinally = true
			vm.restoreStack(excFrame.sp)
			vm.ip = excFrame.handler.FinallyStart
			return true
		}
		vm.popExceptionFrame()
	}
	return false
}

// returnFromFrame returns from the active frame with the value on TOS.
// It reports stop=true if the current eval call should stop.
func (vm *VirtualMachine) returnFromFrame() (stop bool, err error) {
	activeFrame := vm.activeFrame

	// Call observer if present and configured to observe returns (before we lose the frame info)
	if vm.observer != nil && vm.observerConfig.ObserveReturns {
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
			return true, ErrObserverHalt
		}
	}

	// The root frame has no caller. The compiler never returns from it, but
	// bytecode may: end the run with the value on TOS as its result. Move
	// the IP to the end, so a later Run on this VM does not resume after the
	// return.
	if vm.fp == 0 {
		vm.ip = len(vm.activeCode.Instructions)
		return true, nil
	}

	returnAddr := activeFrame.returnAddr
	returnSp := activeFrame.returnSp
	returnFp := vm.fp - 1
	vm.resumeFrame(returnFp, returnAddr, returnSp)
	// If StopSignal is found as the return address, it means the current
	// eval call should stop.
	return returnAddr == StopSignal, nil
}

// handleException handles a thrown exception by finding an appropriate handler.
// If no handler is found, the error is returned to propagate up the call stack.
//
// A handler's frame moves through these states: in the try block, in the
// catch block (kept on the stack only when there is a finally block), and in
// the finally block. An exception never re-enters the block it was thrown from
// or an earlier one.
func (vm *VirtualMachine) handleException(errObj *object.Error) error {
	// Look for an exception handler on the exception stack
	for vm.excStackSize > 0 {
		excFrame := &vm.excStack[vm.excStackSize-1]

		// Check if this handler is for the current frame
		if excFrame.fp != vm.fp || excFrame.code != vm.activeCode {
			if excFrame.fp >= vm.fp {
				// Handler is for a frame we've returned from - pop it as stale
				vm.popExceptionFrame()
				continue
			}
			// Handler is for a caller frame - let error propagate up
			// The caller's tryHandleError will find this handler after frame is restored
			return errObj.Value()
		}

		handler := excFrame.handler
		hasCatch := handler.CatchStart > 0 && handler.CatchStart != handler.FinallyStart
		hasFinally := handler.FinallyStart > 0

		switch {
		case excFrame.inFinally:
			// Exception thrown in finally replaces any pending return or
			// exception. This handler is done; try the next one.
			vm.popExceptionFrame()
		case excFrame.inCatch:
			// Exception thrown in catch (only kept in this state when there
			// is a finally block). Run finally, then re-raise.
			excFrame.pendingError = errObj
			excFrame.inCatch = false
			excFrame.inFinally = true
			vm.restoreStack(excFrame.sp)
			vm.ip = handler.FinallyStart
			return nil
		case hasCatch:
			// Exception thrown in try: enter catch. If there's a finally
			// block, keep the frame so that a throw or return in catch still
			// runs finally. Otherwise catch fully handles the exception.
			vm.restoreStack(excFrame.sp)
			if hasFinally {
				excFrame.inCatch = true
			} else {
				vm.popExceptionFrame()
			}
			vm.push(errObj)
			vm.ip = handler.CatchStart
			return nil
		case hasFinally:
			// No catch block: run finally, then re-raise
			excFrame.pendingError = errObj
			excFrame.inFinally = true
			vm.restoreStack(excFrame.sp)
			vm.ip = handler.FinallyStart
			return nil
		default:
			// No handler available, pop and try next
			vm.popExceptionFrame()
		}
	}

	// No handler found, return the error to propagate up
	return errObj.Value()
}

// tryHandleError attempts to handle an error via exception handling.
// If a handler is found and jumped to, returns nil (exception was handled).
// If no handler is found, returns the error to propagate up. An error that
// ends the run is returned without running any catch or finally block. It
// gets the source location and stack trace like any other error, except
// that a context error is returned as is, so that callers can compare it
// with context.Canceled or context.DeadlineExceeded directly.
func (vm *VirtualMachine) tryHandleError(err error) error {
	if vm.isFatal(err) {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return err
		}
		return vm.wrapError(err)
	}
	return vm.handleException(object.NewError(vm.wrapError(err)))
}

// isFatal reports whether err ends the run. A script cannot catch such an
// error, and its finally blocks do not run, so a script cannot use them to
// keep running past a resource limit, a halt by the observer, or the
// cancellation of the run's context. Errors raised in called functions reach
// the caller's frame through tryHandleError, so this check covers them too.
//
// Once the run's context is done, every error ends the run, whatever its
// type. A host function can return a context error of its own, such as the
// timeout of a request it made, while the run's context is live, and the
// script can catch that.
func (vm *VirtualMachine) isFatal(err error) bool {
	if vm.done != nil {
		select {
		case <-vm.done:
			return true
		default:
		}
	}
	return errors.Is(err, ErrStepLimitExceeded) ||
		errors.Is(err, ErrStackOverflow) ||
		errors.Is(err, ErrReplacedCode) ||
		errors.Is(err, ErrObserverHalt)
}
