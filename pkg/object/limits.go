package object

import (
	"context"
	"maps"
	"math"
	"slices"
)

// Allocation limits guard the host process against scripts that request huge
// allocations in a single operation (e.g. "a".repeat(1 << 40) or
// list(range(1 << 60))). Without them, one call can panic inside the Go
// runtime or exhaust memory before the VM's step and context checks run.
//
// The limits apply to operations whose output size can be much larger than
// their input: repeating, replacing, joining, and concatenating strings and
// bytes, concatenating and extending lists, splitting strings into lists, and
// materializing lazy sequences into lists. Exceeding a limit returns a value
// error.
const (
	// DefaultMaxBytesSize is the default maximum size, in bytes, of a string
	// or bytes value produced by a size-amplifying operation. 64 MiB is far
	// beyond typical embedded scripting needs while keeping a single result
	// small relative to host memory.
	DefaultMaxBytesSize = 64 << 20

	// DefaultMaxListLength is the default maximum number of elements in a
	// list produced by a size-amplifying operation. Each element occupies a
	// 16-byte interface slot on 64-bit platforms, so 1<<22 (4,194,304)
	// elements matches DefaultMaxBytesSize: 64 MiB of slots.
	DefaultMaxListLength = 1 << 22
)

// Limits bounds the size of the values a single operation may produce. A zero
// or negative field uses the corresponding default.
type Limits struct {
	// MaxBytesSize is the maximum size, in bytes, of a string or bytes value
	// produced by a size-amplifying operation.
	MaxBytesSize int64

	// MaxListLength is the maximum number of elements in a list produced by
	// a size-amplifying operation.
	MaxListLength int64
}

// DefaultLimits returns the limits used when none are configured.
func DefaultLimits() Limits {
	return Limits{
		MaxBytesSize:  DefaultMaxBytesSize,
		MaxListLength: DefaultMaxListLength,
	}
}

// withDefaults returns l with zero or negative fields replaced by defaults.
func (l Limits) withDefaults() Limits {
	if l.MaxBytesSize <= 0 {
		l.MaxBytesSize = DefaultMaxBytesSize
	}
	if l.MaxListLength <= 0 {
		l.MaxListLength = DefaultMaxListLength
	}
	return l
}

// ctxCheckInterval is how many iterations long-running Go-side loops run
// between ctx.Err() checks.
const ctxCheckInterval = 1024

// checkBytesSize returns a value error if size exceeds the byte limit.
func checkBytesSize(l Limits, name string, size uint64) error {
	if size > uint64(l.MaxBytesSize) {
		return newValueErrorf("%s result too large (%d bytes exceeds limit of %d)",
			name, size, l.MaxBytesSize)
	}
	return nil
}

// checkRepeatSize returns an error if repeating a value of unit bytes count
// times would overflow or exceed the byte limit. count must be non-negative.
func checkRepeatSize(l Limits, name string, unit int, count int64) error {
	if unit == 0 || count == 0 {
		return nil
	}
	if uint64(count) > uint64(l.MaxBytesSize)/uint64(unit) {
		return newValueErrorf("%s result too large (exceeds limit of %d bytes)",
			name, l.MaxBytesSize)
	}
	return nil
}

// checkReplaceSize returns an error if replacing n occurrences of a substring of
// length oldLen with one of length newLen, in an input of length inputLen,
// would produce a result larger than the byte limit.
func checkReplaceSize(l Limits, name string, inputLen, oldLen, newLen, n int) error {
	if newLen <= oldLen || n == 0 {
		return nil
	}
	growth := uint64(newLen - oldLen)
	if uint64(n) > uint64(l.MaxBytesSize)/growth {
		return newValueErrorf("%s result too large (exceeds limit of %d bytes)",
			name, l.MaxBytesSize)
	}
	return checkBytesSize(l, name, uint64(inputLen)+uint64(n)*growth)
}

// checkConcatSize returns an error if concatenating a and b would produce a
// string or bytes value larger than the byte limit, or a list longer than the
// list length limit. Other operand types are not checked.
func checkConcatSize(l Limits, a, b Object) error {
	if aList, ok := a.(*List); ok {
		if bList, ok := b.(*List); ok {
			return checkListLength(l, uint64(len(aList.items))+uint64(len(bList.items)))
		}
		return nil
	}
	aLen, aOk := byteLen(a)
	bLen, bOk := byteLen(b)
	if !aOk || !bOk {
		return nil
	}
	name := "string concatenation"
	if _, ok := a.(*Bytes); ok {
		name = "bytes concatenation"
	}
	return checkBytesSize(l, name, uint64(aLen)+uint64(bLen))
}

func byteLen(obj Object) (int, bool) {
	switch obj := obj.(type) {
	case *String:
		return len(obj.value), true
	case *Bytes:
		return len(obj.value), true
	}
	return 0, false
}

// CollectValues enumerates e and returns its values as a slice. It returns a
// value error if the result would exceed the list length limit, and returns
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
	maxLen := GetLimits(ctx).MaxListLength
	var items []Object
	if r, ok := e.(*Range); ok {
		// Ranges know their length up front, so reject oversized ranges
		// without enumerating them.
		n := r.length()
		if n > uint64(maxLen) {
			return nil, listTooLargeError(maxLen)
		}
		items = make([]Object, 0, n)
	}
	var limitErr error
	e.Enumerate(ctx, func(key, value Object) bool {
		if int64(len(items)) >= maxLen {
			limitErr = listTooLargeError(maxLen)
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
// the list length limit in ctx.
func CheckListLength(ctx context.Context, n int) error {
	return checkListLength(GetLimits(ctx), uint64(n))
}

func checkListLength(l Limits, n uint64) error {
	if n > uint64(l.MaxListLength) {
		return listTooLargeError(l.MaxListLength)
	}
	return nil
}

// LimitCount returns the count to pass to a Go function that takes a maximum
// number of results, such as strings.SplitN or regexp.FindAllString, so that
// it returns at most one result more than the list length limit in ctx. n is
// the requested count, and a negative n means no maximum. Check the length of
// the result with CheckListLength.
func LimitCount(ctx context.Context, n int) int {
	maxLen := GetLimits(ctx).MaxListLength
	if maxLen >= math.MaxInt {
		return n
	}
	if n < 0 || int64(n) > maxLen {
		return int(maxLen) + 1
	}
	return n
}

func listTooLargeError(maxLen int64) error {
	return newValueErrorf("list too large (exceeds limit of %d elements)", maxLen)
}

// ExpandRanges returns obj with each range, including ranges nested in lists,
// maps, and iterators, replaced by a list of its values. Iterators are
// replaced by a list of the values they yield. It returns a value error if a
// range or iterator yields more values than the list length limit in ctx, and
// ErrNestingDepth if obj is nested more deeply than MaxNestingDepth. Call it
// before Interface when a range must convert to its values: Range.Interface
// has no context, so it cannot apply the configured limit. Lists and maps
// that contain no range or iterator are returned unchanged.
func ExpandRanges(ctx context.Context, obj Object) (Object, error) {
	switch obj.(type) {
	case *Range, *List, *Map, *Iter:
		return expandRanges(ctx, obj, map[Object]Object{}, 1)
	}
	return obj, nil
}

// expandRanges does the work of ExpandRanges. seen maps each list, map, and
// iterator already visited to its result, so a shared container is expanded
// once and a container that contains itself does not recurse forever. depth
// counts the containers enclosing obj, including obj itself.
func expandRanges(ctx context.Context, obj Object, seen map[Object]Object, depth int) (Object, error) {
	if isNestedContainer(obj) {
		if result, ok := seen[obj]; ok {
			return result, nil
		}
		if depth > MaxNestingDepth {
			return nil, ErrNestingDepth
		}
	}
	switch obj := obj.(type) {
	case *Range:
		// The range becomes a list, which counts toward the nesting depth
		if depth > MaxNestingDepth {
			return nil, ErrNestingDepth
		}
		if maxLen := GetLimits(ctx).MaxListLength; obj.length() > uint64(maxLen) {
			return nil, newValueErrorf("range too large (exceeds limit of %d elements)", maxLen)
		}
		values, err := CollectValues(ctx, obj)
		if err != nil {
			return nil, err
		}
		return NewList(values), nil
	case *Iter:
		values, err := CollectValues(ctx, obj)
		if err != nil {
			return nil, err
		}
		list := NewList(values)
		seen[obj] = list
		expanded, err := expandRanges(ctx, list, seen, depth)
		if err != nil {
			return nil, err
		}
		seen[obj] = expanded
		return expanded, nil
	case *List:
		seen[obj] = obj
		var items []Object
		for i, item := range obj.items {
			expanded, err := expandRanges(ctx, item, seen, depth+1)
			if err != nil {
				return nil, err
			}
			if expanded != item && items == nil {
				items = slices.Clone(obj.items)
			}
			if items != nil {
				items[i] = expanded
			}
		}
		if items != nil {
			seen[obj] = NewList(items)
		}
		return seen[obj], nil
	case *Map:
		seen[obj] = obj
		var items map[string]Object
		for key, value := range obj.items {
			expanded, err := expandRanges(ctx, value, seen, depth+1)
			if err != nil {
				return nil, err
			}
			if expanded != value && items == nil {
				items = maps.Clone(obj.items)
			}
			if items != nil {
				items[key] = expanded
			}
		}
		if items != nil {
			seen[obj] = NewMap(items)
		}
		return seen[obj], nil
	}
	return obj, nil
}
