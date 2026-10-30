package object

import "context"

// Allocation limits guard the host process against scripts that request huge
// allocations in a single operation (e.g. "a".repeat(1 << 40) or
// list(range(1 << 60))). Without them, one call can panic inside the Go
// runtime or exhaust memory before the VM's step and context checks run.
//
// The limits apply to operations whose output size can be much larger than
// their input: repeating, replacing, joining, and concatenating strings and
// bytes, and materializing lazy sequences into lists. Exceeding a limit
// returns a value error.
const (
	// MaxBytesSize is the maximum size, in bytes, of a string or bytes value
	// produced by a size-amplifying operation. 64 MiB is far beyond typical
	// embedded scripting needs while keeping a single result small relative
	// to host memory.
	MaxBytesSize = 64 << 20

	// MaxListLength is the maximum number of elements produced when a lazy
	// sequence (such as a range) is materialized into a list. Each element
	// occupies a 16-byte interface slot on 64-bit platforms, so 1<<22
	// (4,194,304) elements matches MaxBytesSize: 64 MiB of slots.
	MaxListLength = 1 << 22
)

// ctxCheckInterval is how many iterations long-running Go-side loops run
// between ctx.Err() checks.
const ctxCheckInterval = 1024

// checkBytesSize returns a value error if size exceeds MaxBytesSize.
func checkBytesSize(name string, size uint64) error {
	if size > MaxBytesSize {
		return newValueErrorf("%s result too large (%d bytes exceeds limit of %d)",
			name, size, MaxBytesSize)
	}
	return nil
}

// checkRepeatSize returns an error if repeating a value of unit bytes count
// times would overflow or exceed MaxBytesSize. count must be non-negative.
func checkRepeatSize(name string, unit int, count int64) error {
	if unit == 0 || count == 0 {
		return nil
	}
	if uint64(count) > MaxBytesSize/uint64(unit) {
		return newValueErrorf("%s result too large (exceeds limit of %d bytes)",
			name, MaxBytesSize)
	}
	return nil
}

// checkReplaceSize returns an error if replacing n occurrences of a substring of
// length oldLen with one of length newLen, in an input of length inputLen,
// would produce a result larger than MaxBytesSize.
func checkReplaceSize(name string, inputLen, oldLen, newLen, n int) error {
	if newLen <= oldLen || n == 0 {
		return nil
	}
	growth := uint64(newLen - oldLen)
	if uint64(n) > MaxBytesSize/growth {
		return newValueErrorf("%s result too large (exceeds limit of %d bytes)",
			name, MaxBytesSize)
	}
	return checkBytesSize(name, uint64(inputLen)+uint64(n)*growth)
}

// CollectValues enumerates e and returns its values as a slice. It returns a
// value error if the result would exceed MaxListLength elements, and returns
// ctx.Err() if the context is canceled during enumeration.
func CollectValues(ctx context.Context, e Enumerable) ([]Object, error) {
	return collect(ctx, e, false)
}

// CollectKeys enumerates e and returns its keys as a slice, with the same
// limits as CollectValues.
func CollectKeys(ctx context.Context, e Enumerable) ([]Object, error) {
	return collect(ctx, e, true)
}

func collect(ctx context.Context, e Enumerable, keys bool) ([]Object, error) {
	var items []Object
	if r, ok := e.(*Range); ok {
		// Ranges know their length up front, so reject oversized ranges
		// without enumerating them.
		n := r.length()
		if n > MaxListLength {
			return nil, listTooLargeError()
		}
		items = make([]Object, 0, n)
	}
	var limitErr error
	e.Enumerate(ctx, func(key, value Object) bool {
		if len(items) >= MaxListLength {
			limitErr = listTooLargeError()
			return false
		}
		if keys {
			items = append(items, key)
		} else {
			items = append(items, value)
		}
		if len(items)%ctxCheckInterval == 0 && ctx.Err() != nil {
			return false
		}
		return true
	})
	if limitErr != nil {
		return nil, limitErr
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

// CheckListLength returns a value error if a list of n elements would exceed
// MaxListLength.
func CheckListLength(n int) error {
	if n > MaxListLength {
		return listTooLargeError()
	}
	return nil
}

func listTooLargeError() error {
	return newValueErrorf("list too large (exceeds limit of %d elements)", MaxListLength)
}
