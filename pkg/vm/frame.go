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

	// MaxRetainedLocals is the largest locals slice a frame keeps for reuse
	// after its function returns. Larger slices are dropped on return, so
	// inactive frames hold at most this many slots each.
	MaxRetainedLocals = 256
)

// cellScanLimit is the most cells a frame searches one by one. A frame with
// more cells indexes them by slot, so that each capture costs O(1).
const cellScanLimit = 8

type frame struct {
	returnAddr     int
	returnSp       int
	callSiteIP     int // IP of the call instruction in the caller's code (for stack traces)
	localsCount    uint16
	localsTotal    int // Local slots in this frame and all active frames below it
	fn             *object.Closure
	code           *loadedCode
	storage        [DefaultFrameLocals]object.Object
	locals         []object.Object
	extendedLocals []object.Object
	cells          []capturedCell // Cells that refer to this frame's locals
	cellsBySlot    []*object.Cell // Index of cells by slot, if len(cells) > cellScanLimit
}

// capturedCell is a cell that refers to one slot of a frame's locals.
type capturedCell struct {
	slot uint16
	cell *object.Cell
}

func (f *frame) ActivateCode(code *loadedCode) {
	f.code = code
	f.fn = nil
	f.returnAddr = 0
	f.callSiteIP = 0
	f.localsCount = uint16(code.LocalsCount())
	// Release the cells of the previous activation before reusing its
	// locals. A frame that a failed run left active may still have some.
	f.releaseCells()

	// Decide where to store local variables. If the frame storage has enough
	// space, use that. Otherwise, reuse extendedLocals if large enough, or
	// allocate a new slice. After this, f.locals will always point to the
	// correct storage. Unassigned locals hold Nil, never a raw Go nil.
	if f.localsCount > DefaultFrameLocals {
		// Need extended storage - reuse existing slice if large enough
		if cap(f.extendedLocals) >= int(f.localsCount) {
			// Reuse existing slice, just resize and clear
			f.extendedLocals = f.extendedLocals[:f.localsCount]
			for i := range f.extendedLocals {
				f.extendedLocals[i] = object.Nil
			}
		} else {
			// Need to allocate - size with some headroom for future calls
			// to reduce allocation churn for functions with varying local counts
			allocSize := int(f.localsCount)
			if allocSize < MinExtendedLocalsCapacity {
				allocSize = MinExtendedLocalsCapacity
			}
			f.extendedLocals = make([]object.Object, f.localsCount, allocSize)
			for i := range f.extendedLocals {
				f.extendedLocals[i] = object.Nil
			}
		}
		f.locals = f.extendedLocals
	} else {
		// Use stack-allocated storage - clear only the needed slots. Keep
		// extendedLocals, since CaptureLocal may reuse it.
		for i := uint16(0); i < f.localsCount; i++ {
			f.storage[i] = object.Nil
		}
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

// releaseLocals drops the frame's locals if they are too large to keep for
// reuse. Call it only on a frame that is no longer active, after
// releaseCells.
func (f *frame) releaseLocals() {
	if cap(f.extendedLocals) > MaxRetainedLocals || cap(f.locals) > MaxRetainedLocals {
		f.locals = nil
		f.extendedLocals = nil
	}
	if cap(f.cells) > MaxRetainedLocals {
		f.cells = nil
	}
	if cap(f.cellsBySlot) > MaxRetainedLocals {
		f.cellsBySlot = nil
	}
}

func (f *frame) Locals() []object.Object {
	return f.locals
}

// CaptureLocal returns a cell for the local in the given slot, or false if
// there is no such slot. While the frame is active, the cell refers to the
// frame's locals, so the frame and its closures see each other's writes.
// All captures of a slot during one activation share one cell.
func (f *frame) CaptureLocal(slot uint16) (*object.Cell, bool) {
	if int(slot) >= len(f.locals) {
		return nil, false
	}
	if len(f.cellsBySlot) > 0 {
		if cell := f.cellsBySlot[slot]; cell != nil {
			return cell, true
		}
	} else {
		for _, c := range f.cells {
			if c.slot == slot {
				return c.cell, true
			}
		}
	}
	if len(f.cells) == 0 && f.localsCount <= DefaultFrameLocals {
		// The locals are in the frame's fixed storage, inside vm.frames,
		// which the VM copies and clears outside of ActivateCode. Move them
		// to the heap before a cell refers to one.
		if cap(f.extendedLocals) < DefaultFrameLocals {
			f.extendedLocals = make([]object.Object, DefaultFrameLocals)
		}
		f.extendedLocals = f.extendedLocals[:f.localsCount]
		copy(f.extendedLocals, f.locals)
		f.locals = f.extendedLocals
	}
	cell := object.NewCell(&f.locals[slot])
	f.cells = append(f.cells, capturedCell{slot: slot, cell: cell})
	if len(f.cellsBySlot) > 0 {
		f.cellsBySlot[slot] = cell
	} else if len(f.cells) > cellScanLimit {
		f.indexCells()
	}
	return cell, true
}

// indexCells indexes the frame's cells by slot. The index has an entry for
// each local, and releaseCells clears the entries it set, so the index
// starts empty when the frame reuses it.
func (f *frame) indexCells() {
	if cap(f.cellsBySlot) < len(f.locals) {
		f.cellsBySlot = make([]*object.Cell, len(f.locals))
	}
	f.cellsBySlot = f.cellsBySlot[:len(f.locals)]
	for _, c := range f.cells {
		f.cellsBySlot[c.slot] = c.cell
	}
}

// releaseCells moves the values of the captured locals into a new slice that
// holds only those values, and points the cells at it. Call it when the frame
// stops being active. Closures that outlive the frame then keep only the
// variables the frame captured, not the frame's whole locals slice, and the
// frame can reuse its locals without trampling the captured values.
func (f *frame) releaseCells() {
	// This check runs on every return, so keep it small enough to inline
	if len(f.cells) > 0 {
		f.moveCells()
	}
}

//go:noinline
func (f *frame) moveCells() {
	values := make([]object.Object, len(f.cells))
	for i, c := range f.cells {
		c.cell.Move(&values[i])
	}
	if len(f.cellsBySlot) > 0 {
		for _, c := range f.cells {
			f.cellsBySlot[c.slot] = nil
		}
		f.cellsBySlot = f.cellsBySlot[:0]
	}
	clear(f.cells)
	f.cells = f.cells[:0]
}
