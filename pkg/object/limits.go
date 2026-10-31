package object

import (
	"context"
	"math"
	"strconv"
	"strings"
	"unicode/utf8"
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

// CheckBytesSize returns a value error if a string or bytes value of size
// bytes would exceed the byte limit in ctx. name describes the operation in
// the error message.
func CheckBytesSize(ctx context.Context, name string, size int) error {
	return checkBytesSize(GetLimits(ctx), name, uint64(size))
}

// CheckFormatSize returns a value error if formatting objs as text, with
// Inspect or as JSON, would produce more bytes than the byte limit in ctx.
// It walks the values without building the text, so it rejects a value whose
// text is too large before that text exists. name describes the operation in
// the error message.
//
// The count is the length of the text that Inspect prints for each value,
// except that a string or an error passed directly counts the length of its
// value or message, as string templates and print write those unquoted. A
// container referenced more than once counts each time it is referenced, as
// the text repeats it. The walk stops as soon as the count passes the limit,
// so its cost is bounded by the limit too.
func CheckFormatSize(ctx context.Context, name string, objs ...Object) error {
	l := GetLimits(ctx)
	limit := uint64(l.MaxBytesSize)
	var total uint64
	for _, obj := range objs {
		switch obj := obj.(type) {
		case *String:
			total += uint64(len(obj.value))
		case *Error:
			total += uint64(len(obj.Message().Value()))
		default:
			total += formatSize(obj, nil, limit-min(total, limit))
		}
		if total > limit {
			break
		}
	}
	return checkBytesSize(l, name, total)
}

// formatSize returns the length of the text that obj.Inspect prints, without
// building that text for a list, map, cell, partial, string, bytes, or error.
// It stops counting once the count passes limit, and then returns a count
// over limit. path holds the containers being walked, so a cycle or a value
// nested too deeply counts as the placeholder that Inspect prints for it.
//
// The count is never less than the length of the text, so a bound measured
// with it bounds the text. Any type without a case here counts the length of
// its Inspect text, so a new type cannot undercount. Such a type must print
// no other objects, or print them through their bounded Inspect.
func formatSize(obj Object, path nestPath[Object], limit uint64) uint64 {
	var buf [32]byte
	switch obj := obj.(type) {
	case *String:
		return quotedStringSize(obj.value)
	case *Bytes:
		// bytes("...")
		return 7 + quotedBytesSize(obj.value)
	case *Error:
		// error("...")
		return 7 + quotedSize(obj.Message().Value())
	case *Int:
		return uint64(len(strconv.AppendInt(buf[:0], obj.value, 10)))
	case *Float:
		return uint64(len(strconv.AppendFloat(buf[:0], obj.value, 'f', -1, 64)))
	case *Bool:
		if obj.value {
			return 4
		}
		return 5
	case *NilType:
		return 4
	case *List:
		// [a, b]
		total := uint64(2)
		for i, item := range obj.items {
			if i > 0 {
				total += 2
			}
			total += nestedFormatSize(obj, item, &path, limit-min(total, limit))
			if total > limit {
				return total
			}
		}
		return total
	case *Map:
		// {"a": 1, "b": 2}
		total := uint64(2)
		i := 0
		for key, value := range obj.items {
			if i > 0 {
				total += 2
			}
			i++
			total += quotedSize(key) + 2
			total += nestedFormatSize(obj, value, &path, limit-min(total, limit))
			if total > limit {
				return total
			}
		}
		return total
	case *Partial:
		// partial(fn, a, b). The partial goes on the path, so a partial that
		// holds its own container counts that container as a placeholder.
		total := uint64(11) + nestedFormatSize(obj, obj.fn, &path, limit-min(11, limit))
		for i, arg := range obj.args {
			if total > limit {
				return total
			}
			if i > 0 {
				total += 2
			}
			total += nestedFormatSize(obj, arg, &path, limit-min(total, limit))
		}
		return total
	case *Cell:
		// cell(value). The cell goes on the path, so a cell that holds
		// itself, directly or through a container, counts as a placeholder.
		if obj.value == nil {
			return 6
		}
		return 6 + nestedFormatSize(obj, *obj.value, &path, limit-min(6, limit))
	}
	return uint64(len(obj.Inspect()))
}

func nestedFormatSize(parent, item Object, path *nestPath[Object], limit uint64) uint64 {
	if !isPrintedContainer(item) {
		return formatSize(item, nil, limit)
	}
	p, err := path.descend(parent, item)
	*path = p
	if err != nil {
		return uint64(len(inspectPlaceholder(item)))
	}
	return formatSize(item, p, limit)
}

// quotedStringSize returns the length of the text that String.Inspect prints
// for s.
func quotedStringSize(s string) uint64 {
	n := len(s)
	if n >= 2 && s[0] == '"' && s[n-1] == '"' && strings.Count(s, `"`) == 2 {
		// '"..."'
		return uint64(n) + 2
	}
	return quotedSize(s)
}

// quotedSize returns the length of strconv.Quote(s), the text that %q prints
// for a string, without building it.
func quotedSize(s string) uint64 {
	total := uint64(2)
	for i := 0; i < len(s); {
		if c := s[i]; c < utf8.RuneSelf {
			total += asciiQuotedSize(c)
			i++
			continue
		}
		r, width := utf8.DecodeRuneInString(s[i:])
		total += runeQuotedSize(r, width)
		i += width
	}
	return total
}

// quotedBytesSize returns the length of the text that %q prints for b.
func quotedBytesSize(b []byte) uint64 {
	total := uint64(2)
	for i := 0; i < len(b); {
		if c := b[i]; c < utf8.RuneSelf {
			total += asciiQuotedSize(c)
			i++
			continue
		}
		r, width := utf8.DecodeRune(b[i:])
		total += runeQuotedSize(r, width)
		i += width
	}
	return total
}

// asciiQuotedSize returns the length of an ASCII byte quoted by
// strconv.Quote.
func asciiQuotedSize(c byte) uint64 {
	switch {
	case c == '"' || c == '\\':
		return 2
	case c >= ' ' && c != 0x7f:
		return 1
	case c == '\a' || c == '\b' || c == '\f' || c == '\n' || c == '\r' ||
		c == '\t' || c == '\v':
		return 2
	}
	return 4 // \x00
}

// runeQuotedSize returns the length of a non-ASCII rune of width bytes
// quoted by strconv.Quote.
func runeQuotedSize(r rune, width int) uint64 {
	switch {
	case r == utf8.RuneError && width == 1:
		return 4 // \xff
	case strconv.IsPrint(r):
		return uint64(width)
	case r < 0x10000:
		return 6 // \u00a0
	}
	return 10 // \U000e0001
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

// maxCaseMapGrowth is the most that case mapping can multiply a string's
// length by. An invalid UTF-8 byte maps to U+FFFD, which is 3 bytes. A valid
// rune grows by at most half (2 bytes to 3), and ASCII stays ASCII.
const maxCaseMapGrowth = 3

// checkCaseMapSize returns an error if mapping each rune of s with mapping,
// as strings.ToUpper and strings.ToLower do, would produce a string larger
// than the byte limit. It counts the result's length without building it.
func checkCaseMapSize(l Limits, name, s string, mapping func(rune) rune) error {
	if uint64(len(s)) <= uint64(l.MaxBytesSize)/maxCaseMapGrowth {
		return nil
	}
	return checkBytesSize(l, name, uint64(caseMapLen(s, mapping)))
}

// caseMapLen returns the length of the string that strings.Map(mapping, s)
// returns, for a mapping that never drops runes. Like strings.Map, it reads
// each invalid UTF-8 byte as U+FFFD, which is 3 bytes in the result.
func caseMapLen(s string, mapping func(rune) rune) int {
	// ASCII maps to ASCII, so an ASCII prefix keeps its length.
	i := 0
	for i < len(s) && s[i] < utf8.RuneSelf {
		i++
	}
	n := i
	for _, r := range s[i:] {
		n += utf8.RuneLen(mapping(r))
	}
	return n
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

// convBudget bounds the total number of elements that one conversion of a
// value to Go may produce. It counts every list item, map entry, iterator
// item, and range element, including those of a container that is referenced
// more than once. Without a total bound, a small value that references one
// large list many times converts to a huge Go value. A conversion of a Go
// value to Risor uses the same budget, counting every slice, array, and map
// element.
type convBudget struct {
	ctx       context.Context
	remaining int64
	limit     int64
	// jsonOnly makes the conversion reject values that have no JSON form;
	// see ToJSONValue.
	jsonOnly bool
}

// newConvBudget returns a budget of the list length limit in ctx.
func newConvBudget(ctx context.Context) *convBudget {
	limit := GetLimits(ctx).MaxListLength
	return &convBudget{ctx: ctx, remaining: limit, limit: limit}
}

// take charges n elements to the budget. It returns a value error if the
// budget has fewer than n elements left. Call it before allocating, so an
// oversized container is never built. A nil budget has no limit.
func (b *convBudget) take(n uint64) error {
	if b == nil {
		return nil
	}
	if n > uint64(b.remaining) {
		return newValueErrorf("value too large to convert (exceeds limit of %d elements)", b.limit)
	}
	b.remaining -= int64(n)
	return nil
}
