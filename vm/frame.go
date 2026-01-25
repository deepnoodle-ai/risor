package vm

import (
	"github.com/risor-io/risor/object"
)

const DefaultFrameLocals = 8

type frame struct {
	returnAddr     int
	returnSp       int
	localsCount    uint16
	fn             *object.Function
	code           *code
	storage        [DefaultFrameLocals]object.Object
	locals         []object.Object
	extendedLocals []object.Object
	capturedLocals []object.Object
}

func (f *frame) ActivateCode(code *code) {
	f.code = code
	f.fn = nil
	f.returnAddr = 0
	f.localsCount = uint16(code.LocalsCount())
	f.capturedLocals = nil
	for i := 0; i < DefaultFrameLocals; i++ {
		f.storage[i] = nil
	}
	// Decide where to store local variables. If the frame storage has enough
	// space, use that. Otherwise, allocate a new slice as extendedLocals.
	// After this, f.locals will always point to the correct storage.
	if f.localsCount > DefaultFrameLocals {
		f.extendedLocals = make([]object.Object, f.localsCount)
		f.locals = f.extendedLocals
	} else {
		f.extendedLocals = nil
		f.locals = f.storage[:f.localsCount]
	}
}

func (f *frame) ActivateFunction(fn *object.Function, code *code, returnAddr, returnSp int, localValues []object.Object) {
	// Activate the function's code
	f.ActivateCode(code)
	f.fn = fn
	// Save the instruction and stack pointers of the caller
	f.returnAddr = returnAddr
	f.returnSp = returnSp
	// Initialize any local variables that were provided
	for i := 0; i < len(localValues); i++ {
		f.locals[i] = localValues[i]
	} //lint:ignore S1001 - this loop is faster than using copy
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
