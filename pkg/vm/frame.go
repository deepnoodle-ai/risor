package vm

import (
	"github.com/deepnoodle-ai/risor/v2/pkg/object"
)

const (
	// DefaultFrameLocals is the number of local variables that can be stored
	// directly in the frame's fixed storage array, avoiding heap allocation.
	DefaultFrameLocals = 8

	// MinExtendedLocalsCapacity is the minimum capacity allocated for extended
	// locals when heap allocation is needed. This provides headroom for future
	// calls to reduce allocation churn for functions with varying local counts.
	MinExtendedLocalsCapacity = 32
)

type frame struct {
	returnAddr     int
	returnSp       int
	callSiteIP     int // IP of the call instruction in the caller's code (for stack traces)
	localsCount    uint16
	fn             *object.Closure
	code           *loadedCode
	storage        [DefaultFrameLocals]object.Object
	locals         []object.Object
	extendedLocals []object.Object
	capturedLocals []object.Object
}

func (f *frame) ActivateCode(code *loadedCode) {
	f.code = code
	f.fn = nil
	f.returnAddr = 0
	f.callSiteIP = 0
	f.localsCount = uint16(code.LocalsCount())
	f.capturedLocals = nil

	// Decide where to store local variables. If the frame storage has enough
	// space, use that. Otherwise, reuse extendedLocals if large enough, or
	// allocate a new slice. After this, f.locals will always point to the
	// correct storage.
	if f.localsCount > DefaultFrameLocals {
		// Need extended storage - reuse existing slice if large enough
		if cap(f.extendedLocals) >= int(f.localsCount) {
			// Reuse existing slice, just resize and clear
			f.extendedLocals = f.extendedLocals[:f.localsCount]
			for i := range f.extendedLocals {
				f.extendedLocals[i] = nil
			}
		} else {
			// Need to allocate - size with some headroom for future calls
			// to reduce allocation churn for functions with varying local counts
			allocSize := int(f.localsCount)
			if allocSize < MinExtendedLocalsCapacity {
				allocSize = MinExtendedLocalsCapacity
			}
			f.extendedLocals = make([]object.Object, f.localsCount, allocSize)
		}
		f.locals = f.extendedLocals
	} else {
		// Use stack-allocated storage - clear only the needed slots
		for i := uint16(0); i < f.localsCount; i++ {
			f.storage[i] = nil
		}
		f.extendedLocals = nil
		f.locals = f.storage[:f.localsCount]
	}
}

func (f *frame) ActivateFunction(fn *object.Closure, code *loadedCode, returnAddr, returnSp int, localValues []object.Object) {
	// Activate the function's code
	f.ActivateCode(code)
	f.fn = fn
	// Save the instruction and stack pointers of the caller
	f.returnAddr = returnAddr
	f.returnSp = returnSp
	// Save the call site IP for stack traces (returnAddr may be overwritten with StopSignal)
	f.callSiteIP = returnAddr
	// Initialize any local variables that were provided
	copy(f.locals, localValues)
}

func (f *frame) Locals() []object.Object {
	return f.locals
}

func (f *frame) CaptureLocals() []object.Object {
	if f.capturedLocals != nil {
		return f.capturedLocals
	}
	if f.extendedLocals != nil {
		f.capturedLocals = f.extendedLocals
		return f.capturedLocals
	}
	newStorage := make([]object.Object, len(f.locals))
	copy(newStorage, f.locals)
	f.capturedLocals = newStorage
	f.locals = newStorage
	return newStorage
}
