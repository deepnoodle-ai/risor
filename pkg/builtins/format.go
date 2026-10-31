package builtins

import (
	"context"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"unicode/utf8"
	"unsafe"

	"github.com/deepnoodle-ai/risor/v2/pkg/object"
)

// checkFormatSize returns a value error if formatting args with format, as
// fmt.Sprintf does (or fmt.Errorf when wrapErrs is true), would produce more
// bytes than the byte limit in ctx. name describes the operation in the error
// message.
//
// It counts the length of the text without building it, so a format that
// amplifies a short input into a huge result is rejected before that result
// exists. The things in a format that amplify are a width or precision on
// each verb (including a * width taken from an argument), many verbs that
// reuse one argument through an explicit [n] index, a width that fmt applies
// to every element of a list or map, and lists that repeat one large value
// through shared references. The count covers all of them.
//
// It walks the format and the arguments the way fmt does. Strings, and the
// text that a String or Error method returns, are counted without building
// any quoted or hex text. A leaf value with no such path, such as a float,
// is counted by formatting that one value alone with the same directive.
// The count calls the String, Error, GoString, and Format methods of host Go
// values, and fmt then calls them again. The
// count stops as soon as it passes the limit, and a width or numeric
// precision larger than the rest of the limit stops it before the value is
// formatted, so counting allocates at most about the limit plus the text of
// one leaf value.
//
// The count is the length of the text that fmt produces. Tests compare the
// two for every verb and flag on each kind of value that Risor passes to
// fmt, so a change in a later Go release shows up there. Callers still check
// the length of the formatted text, so a count that falls short cannot
// return a result over the limit.
func checkFormatSize(ctx context.Context, name string, wrapErrs bool, format string, args []any) error {
	limit := uint64(object.GetLimits(ctx).MaxBytesSize)
	if formatCount(limit, wrapErrs, format, args) > limit {
		return object.ValueErrorf("%s result too large (exceeds limit of %d bytes)", name, limit)
	}
	return nil
}

// formatCount returns the length of the text that formatting args with
// format produces, or a count over limit if that length exceeds limit.
func formatCount(limit uint64, wrapErrs bool, format string, args []any) uint64 {
	c := &formatCounter{limit: limit, wrapErrs: wrapErrs}
	c.format(format, args)
	return c.total
}

// fmtSpec holds the flags, width, and precision of one directive, as fmt's
// internal fmtFlags does. For %v and %w, the # and + flags are moved to
// sharpV and plusV.
type fmtSpec struct {
	sharp, sharpV, plus, plusV, minus, space, zero bool
	widPresent, precPresent                        bool
	wid, prec                                      int
}

// String returns a directive that formats with the flags, width, and
// precision of s and the given verb.
func (s fmtSpec) String(verb rune) string {
	b := make([]byte, 0, 16)
	b = append(b, '%')
	if s.sharp || (s.sharpV && (verb == 'v' || verb == 'w')) {
		b = append(b, '#')
	}
	if s.plus || (s.plusV && (verb == 'v' || verb == 'w')) {
		b = append(b, '+')
	}
	if s.minus {
		b = append(b, '-')
	}
	if s.space {
		b = append(b, ' ')
	}
	if s.zero {
		b = append(b, '0')
	}
	if s.widPresent {
		b = strconv.AppendInt(b, int64(s.wid), 10)
	}
	if s.precPresent {
		b = append(b, '.')
		b = strconv.AppendInt(b, int64(s.prec), 10)
	}
	b = utf8.AppendRune(b, verb)
	return string(b)
}

// countWriter counts the bytes written to it and discards them.
type countWriter uint64

func (w *countWriter) Write(p []byte) (int, error) {
	*w += countWriter(len(p))
	return len(p), nil
}

// byteCacheKey identifies how a cached byte value was formatted.
type byteCacheKey struct {
	directive string
	erroring  bool
}

// formatCounter counts the length of the text that fmt produces. Its methods
// mirror the methods of fmt's printer: format is doPrintf, arg is printArg,
// value is printValue, methods is handleMethods, bytes is fmtBytes, and
// pointer is fmtPointer.
type formatCounter struct {
	limit    uint64
	total    uint64
	wrapErrs bool // %w is valid, as in fmt.Errorf

	// erroring is set while counting the value inside a %!verb(type=value)
	// error, where fmt calls no Format, String, or Error method.
	erroring bool

	// s is the directive being counted.
	s fmtSpec

	// byteCache holds the formatted length of each byte value for one
	// directive, so a large byte slice is not formatted byte by byte.
	byteCache    [256]uint32
	byteCacheSet [256]bool
	byteCacheKey byteCacheKey

	// The last directive built by directive.
	lastSpec      fmtSpec
	lastVerb      rune
	lastDirective string
}

func (c *formatCounter) over() bool {
	return c.total > c.limit
}

// remaining returns the number of bytes left before the limit.
func (c *formatCounter) remaining() uint64 {
	return c.limit - min(c.total, c.limit)
}

func (c *formatCounter) add(n int) {
	c.total += uint64(n)
}

// format counts the text of fmt's doPrintf.
func (c *formatCounter) format(format string, a []any) {
	end := len(format)
	argNum := 0
	afterIndex := false
	reordered := false
formatLoop:
	for i := 0; i < end; {
		if c.over() {
			return
		}
		goodArgNum := true
		lasti := i
		for i < end && format[i] != '%' {
			i++
		}
		c.add(i - lasti)
		if i >= end {
			break
		}
		i++

		var s fmtSpec
	simpleFormat:
		for ; i < end; i++ {
			ch := format[i]
			switch ch {
			case '#':
				s.sharp = true
			case '0':
				s.zero = true
			case '+':
				s.plus = true
			case '-':
				s.minus = true
			case ' ':
				s.space = true
			default:
				if 'a' <= ch && ch <= 'z' && argNum < len(a) {
					if ch == 'v' || ch == 'w' {
						s.sharpV, s.sharp = s.sharp, false
						s.plusV, s.plus = s.plus, false
					}
					c.directiveArg(s, a[argNum], rune(ch))
					argNum++
					i++
					continue formatLoop
				}
				break simpleFormat
			}
		}

		// argNumber reads an explicit argument index such as [3].
		argNumber := func() {
			if i >= end || format[i] != '[' {
				afterIndex = false
				return
			}
			reordered = true
			index, wid, ok := parseArgNumber(format[i:])
			i += wid
			if ok && 0 <= index && index < len(a) {
				argNum = index
				afterIndex = true
				return
			}
			goodArgNum = false
			afterIndex = ok
		}

		argNumber()

		// Width
		if i < end && format[i] == '*' {
			i++
			s.wid, s.widPresent, argNum = intFromArg(a, argNum)
			if !s.widPresent {
				c.add(len("%!(BADWIDTH)"))
			}
			if s.wid < 0 {
				s.wid = -s.wid
				s.minus = true
				s.zero = false
			}
			afterIndex = false
		} else {
			s.wid, s.widPresent, i = parsenum(format, i, end)
			if afterIndex && s.widPresent {
				goodArgNum = false
			}
		}

		// Precision
		if i+1 < end && format[i] == '.' {
			i++
			if afterIndex {
				goodArgNum = false
			}
			argNumber()
			if i < end && format[i] == '*' {
				i++
				s.prec, s.precPresent, argNum = intFromArg(a, argNum)
				if s.prec < 0 {
					s.prec = 0
					s.precPresent = false
				}
				if !s.precPresent {
					c.add(len("%!(BADPREC)"))
				}
				afterIndex = false
			} else {
				s.prec, s.precPresent, i = parsenum(format, i, end)
				if !s.precPresent {
					s.prec = 0
					s.precPresent = true
				}
			}
		}

		if !afterIndex {
			argNumber()
		}

		if i >= end {
			c.add(len("%!(NOVERB)"))
			break
		}

		verb, size := utf8.DecodeRuneInString(format[i:])
		i += size

		switch {
		case verb == '%':
			c.add(1)
		case !goodArgNum:
			c.add(len("%!") + utf8.RuneLen(verb) + len("(BADINDEX)"))
		case argNum >= len(a):
			c.add(len("%!") + utf8.RuneLen(verb) + len("(MISSING)"))
		default:
			if verb == 'v' || verb == 'w' {
				s.sharpV, s.sharp = s.sharp, false
				s.plusV, s.plus = s.plus, false
			}
			c.directiveArg(s, a[argNum], verb)
			argNum++
		}
	}

	if !reordered && argNum < len(a) {
		c.add(len("%!(EXTRA "))
		for i, arg := range a[argNum:] {
			if c.over() {
				return
			}
			if i > 0 {
				c.add(len(", "))
			}
			if arg == nil {
				c.add(len("<nil>"))
			} else {
				c.add(len(reflect.TypeOf(arg).String()) + 1)
				c.directiveArg(fmtSpec{}, arg, 'v')
			}
		}
		c.add(1)
	}
}

// directiveArg counts one formatted argument.
func (c *formatCounter) directiveArg(s fmtSpec, arg any, verb rune) {
	c.s = s
	c.arg(arg, verb)
}

// directive returns s.String(verb), reusing the last result, so counting
// the elements of a large list does not build a directive for each one.
func (c *formatCounter) directive(s fmtSpec, verb rune) string {
	if c.lastDirective == "" || s != c.lastSpec || verb != c.lastVerb {
		c.lastSpec, c.lastVerb, c.lastDirective = s, verb, s.String(verb)
	}
	return c.lastDirective
}

// arg counts the text of fmt's printArg.
func (c *formatCounter) arg(arg any, verb rune) {
	if c.over() {
		return
	}
	if arg == nil {
		if verb == 'T' || verb == 'v' {
			c.padString("<nil>")
		} else {
			c.badVerbNil(verb)
		}
		return
	}
	switch verb {
	case 'T':
		c.fmtS(reflect.TypeOf(arg).String())
		return
	case 'p':
		c.pointer(reflect.ValueOf(arg), 'p', arg)
		return
	}
	switch f := arg.(type) {
	case bool, float32, float64, complex64, complex128,
		int, int8, int16, int32, int64,
		uint, uint8, uint16, uint32, uint64, uintptr, string:
		c.leaf(reflect.ValueOf(f), verb)
	case []byte:
		c.bytes(f, verb, "[]byte")
	case reflect.Value:
		if f.IsValid() && f.CanInterface() && c.methods(f.Interface(), verb) {
			return
		}
		c.value(f, verb, 0)
	default:
		if !c.methods(f, verb) {
			c.value(reflect.ValueOf(f), verb, 0)
		}
	}
}

// methods counts the text of fmt's handleMethods and reports whether a
// method of x formats it.
func (c *formatCounter) methods(x any, verb rune) bool {
	if c.erroring {
		return false
	}
	if verb == 'w' {
		if _, ok := x.(error); !ok || !c.wrapErrs {
			c.badVerb(verb, x)
			return true
		}
		verb = 'v'
	}
	if f, ok := x.(fmt.Formatter); ok {
		c.formatter(f, verb)
		return true
	}
	if c.s.sharpV {
		if g, ok := x.(fmt.GoStringer); ok {
			c.methodText(x, verb, func() { c.fmtS(g.GoString()) })
			return true
		}
		return false
	}
	switch verb {
	case 'v', 's', 'x', 'X', 'q':
		switch m := x.(type) {
		case error:
			c.methodText(x, verb, func() { c.fmtString(m.Error(), verb) })
			return true
		case fmt.Stringer:
			c.methodText(x, verb, func() { c.fmtString(m.String(), verb) })
			return true
		}
	}
	return false
}

// methodText runs count, which counts the text that a GoString, Error, or
// String method of x returns. If the method panics, it counts the text that
// fmt prints for the panic by formatting x alone.
func (c *formatCounter) methodText(x any, verb rune, count func()) {
	total := c.total
	defer func() {
		if r := recover(); r != nil {
			c.total = total
			c.formatted(x, verb)
		}
	}()
	count()
}

// fmtString counts the text of fmt's fmtString for s and reports whether
// verb is valid for a string. It counts quoted and hex text without building
// it.
func (c *formatCounter) fmtString(s string, verb rune) bool {
	switch {
	case verb == 's' || (verb == 'v' && !c.s.sharpV):
		c.fmtS(s)
	case verb == 'x' || verb == 'X':
		c.hexSize(len(s))
	case verb == 'q' || (verb == 'v' && c.s.sharpV):
		c.fmtQ(s)
	default:
		return false
	}
	return true
}

// value counts the text of fmt's printValue.
func (c *formatCounter) value(v reflect.Value, verb rune, depth int) {
	if c.over() {
		return
	}
	if depth > 0 && v.IsValid() && v.CanInterface() && c.methods(v.Interface(), verb) {
		return
	}
	switch v.Kind() {
	case reflect.Invalid:
		if depth == 0 {
			c.add(len("<invalid reflect.Value>"))
		} else if verb == 'v' {
			c.add(len("<nil>"))
		} else {
			c.badVerbNil(verb)
		}
	case reflect.Bool,
		reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr,
		reflect.Float32, reflect.Float64, reflect.Complex64, reflect.Complex128,
		reflect.String:
		c.leaf(v, verb)
	case reflect.Map:
		if c.s.sharpV {
			c.add(len(v.Type().String()))
			if v.IsNil() {
				c.add(len("(nil)"))
				return
			}
			c.add(1) // {
		} else {
			c.add(len("map["))
		}
		c.add(1) // } or ]
		if m, ok := interfaceOf(v).(map[string]any); ok {
			// Fast path for the maps that Risor passes to fmt
			i := 0
			for key, value := range m {
				if c.over() {
					return
				}
				if i > 0 {
					c.separator()
				}
				i++
				if !c.fastLeaf(key, verb) {
					c.value(reflect.ValueOf(key), verb, depth+1)
				}
				c.add(1) // :
				if !c.fastLeaf(value, verb) {
					c.value(v.MapIndex(reflect.ValueOf(key)), verb, depth+1)
				}
			}
			return
		}
		iter := v.MapRange()
		for i := 0; iter.Next(); i++ {
			if c.over() {
				return
			}
			if i > 0 {
				c.separator()
			}
			c.value(iter.Key(), verb, depth+1)
			c.add(1) // :
			c.value(iter.Value(), verb, depth+1)
		}
	case reflect.Struct:
		t := v.Type()
		if c.s.sharpV {
			c.add(len(t.String()))
		}
		c.add(2) // {}
		for i := 0; i < v.NumField(); i++ {
			if c.over() {
				return
			}
			if i > 0 {
				c.separator()
			}
			if c.s.plusV || c.s.sharpV {
				if name := t.Field(i).Name; name != "" {
					c.add(len(name) + 1)
				}
			}
			field := v.Field(i)
			if field.Kind() == reflect.Interface && !field.IsNil() {
				field = field.Elem()
			}
			c.value(field, verb, depth+1)
		}
	case reflect.Interface:
		elem := v.Elem()
		if !elem.IsValid() {
			if c.s.sharpV {
				c.add(len(v.Type().String()) + len("(nil)"))
			} else {
				c.add(len("<nil>"))
			}
			return
		}
		c.value(elem, verb, depth+1)
	case reflect.Array, reflect.Slice:
		t := v.Type()
		switch verb {
		case 's', 'q', 'x', 'X':
			if t.Elem().Kind() == reflect.Uint8 {
				var b []byte
				if v.Kind() == reflect.Slice || v.CanAddr() {
					b = v.Bytes()
				} else {
					b = make([]byte, v.Len())
					for i := range b {
						b[i] = byte(v.Index(i).Uint())
					}
				}
				c.bytes(b, verb, t.String())
				return
			}
		}
		if c.s.sharpV {
			c.add(len(t.String()))
			if v.Kind() == reflect.Slice && v.IsNil() {
				c.add(len("(nil)"))
				return
			}
		}
		c.add(2) // {} or []
		if items, ok := interfaceOf(v).([]any); ok {
			// Fast path for the lists that Risor passes to fmt
			for i, item := range items {
				if c.over() {
					return
				}
				if i > 0 {
					c.separator()
				}
				if !c.fastLeaf(item, verb) {
					c.value(v.Index(i), verb, depth+1)
				}
			}
			return
		}
		for i := 0; i < v.Len(); i++ {
			if c.over() {
				return
			}
			if i > 0 {
				c.separator()
			}
			c.value(v.Index(i), verb, depth+1)
		}
	case reflect.Pointer:
		if depth == 0 && v.UnsafePointer() != nil {
			switch elem := v.Elem(); elem.Kind() {
			case reflect.Array, reflect.Slice, reflect.Struct, reflect.Map:
				c.add(1) // &
				c.value(elem, verb, depth+1)
				return
			}
		}
		c.pointer(v, verb, nil)
	case reflect.Chan, reflect.Func, reflect.UnsafePointer:
		c.pointer(v, verb, nil)
	default:
		c.add(len("?" + v.Type().String() + "?"))
	}
}

// separator counts the text between the elements of a list, map, or struct.
func (c *formatCounter) separator() {
	if c.s.sharpV {
		c.add(len(", "))
	} else {
		c.add(1)
	}
}

// leaf counts a bool, number, or string that no method formats.
func (c *formatCounter) leaf(v reflect.Value, verb rune) {
	// The text of a leaf is never shorter than its width.
	if c.s.widPresent && uint64(c.s.wid) > c.remaining() {
		c.add(c.s.wid)
		return
	}

	switch v.Kind() {
	case reflect.String:
		if c.fmtString(v.String(), verb) {
			return
		}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if !c.s.precPresent && (verb == 'd' || verb == 'v') {
			var buf [24]byte
			c.integer(len(strconv.AppendInt(buf[:0], v.Int(), 10)), v.Int() < 0)
			return
		}
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		if !c.s.precPresent && (verb == 'd' || (verb == 'v' && !c.s.sharpV)) {
			var buf [24]byte
			c.integer(len(strconv.AppendUint(buf[:0], v.Uint(), 10)), false)
			return
		}
	}
	if !validVerb(v.Kind(), verb) {
		c.badVerbValue(verb, v)
		return
	}
	// For these verbs, the text of a number is never shorter than its
	// precision, so a larger precision is over the limit.
	if c.s.precPresent && uint64(c.s.prec) > c.remaining() && precisionIsMinimum(v.Kind(), verb, c.s.sharp) {
		c.add(c.s.prec)
		return
	}
	x := c.leafValue(v)
	if verb == 'v' && (c.s.sharp || c.s.plus) {
		// Inside %!verb(type=value), fmt formats with %v but keeps the
		// # and + flags of the original verb, which a %v directive would
		// move to sharpV and plusV. The verb that formats the same way
		// keeps them.
		verb = plainVerb(v.Kind())
	}
	if b, ok := x.(uint8); ok {
		c.cachedByte(b, c.directive(c.s, verb))
		return
	}
	c.formatted(x, verb)
}

// validVerb reports whether fmt formats a bool, number, or string of kind
// with verb, rather than printing %!verb(type=value).
func validVerb(kind reflect.Kind, verb rune) bool {
	var verbs string
	switch kind {
	case reflect.Bool:
		verbs = "tv"
	case reflect.Float32, reflect.Float64, reflect.Complex64, reflect.Complex128:
		verbs = "vbgGxXfFeE"
	case reflect.String:
		verbs = "vsxXq"
	default:
		verbs = "vdboOxXcqU"
	}
	return strings.ContainsRune(verbs, verb)
}

// precisionIsMinimum reports whether fmt prints at least prec bytes for a
// number of kind formatted with verb and a precision of prec. %g and %v drop
// trailing zeros unless the # flag is set, and %c and %q ignore the
// precision.
func precisionIsMinimum(kind reflect.Kind, verb rune, sharp bool) bool {
	switch kind {
	case reflect.Bool, reflect.String:
		return false
	case reflect.Float32, reflect.Float64, reflect.Complex64, reflect.Complex128:
		switch verb {
		case 'e', 'E', 'f', 'F', 'x', 'X':
			return true
		case 'g', 'G', 'v':
			return sharp
		}
		return false
	}
	return strings.ContainsRune("vdboOxXU", verb)
}

// plainVerb returns the verb that formats a bool, number, or string of kind
// as %v does, without %v's handling of the # and + flags.
func plainVerb(kind reflect.Kind) rune {
	switch kind {
	case reflect.Bool:
		return 't'
	case reflect.Float32, reflect.Float64, reflect.Complex64, reflect.Complex128:
		return 'g'
	case reflect.String:
		return 's'
	}
	return 'd'
}

// interfaceOf returns v.Interface(), or nil if v cannot be used without
// panicking, as for a value of an unexported struct field.
func interfaceOf(v reflect.Value) any {
	if !v.CanInterface() {
		return nil
	}
	return v.Interface()
}

// fastLeaf counts x and returns true if x is an int64 or a string that
// leaf would count without formatting it. An element of a list or map is
// counted with fastLeaf first, to skip reflection for the common cases.
func (c *formatCounter) fastLeaf(x any, verb rune) bool {
	if c.s.widPresent && uint64(c.s.wid) > c.remaining() {
		return false
	}
	switch x := x.(type) {
	case int64:
		if !c.s.precPresent && (verb == 'd' || verb == 'v') {
			var buf [24]byte
			c.integer(len(strconv.AppendInt(buf[:0], x, 10)), x < 0)
			return true
		}
	case string:
		if verb == 's' || (verb == 'v' && !c.s.sharpV) {
			c.fmtS(x)
			return true
		}
	}
	return false
}

// integer counts a decimal integer of n characters, including any minus
// sign.
func (c *formatCounter) integer(n int, negative bool) {
	if !negative && (c.s.plus || c.s.space) {
		n++
	}
	if c.s.widPresent && c.s.wid > n {
		n = c.s.wid
	}
	c.add(n)
}

// leafValue returns the Go value that fmt formats for v. Inside a
// %!verb(type=value) error, fmt calls no methods, so a value whose type has
// methods is converted to the plain value of its kind.
func (c *formatCounter) leafValue(v reflect.Value) any {
	if v.CanInterface() {
		x := v.Interface()
		if !c.erroring {
			return x
		}
		switch x.(type) {
		case fmt.Formatter, fmt.GoStringer, error, fmt.Stringer:
		default:
			return x
		}
	}
	switch v.Kind() {
	case reflect.Bool:
		return v.Bool()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return v.Int()
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return v.Uint()
	case reflect.Float32:
		return float32(v.Float())
	case reflect.Float64:
		return v.Float()
	case reflect.Complex64:
		return complex64(v.Complex())
	case reflect.Complex128:
		return v.Complex()
	default:
		return v.String()
	}
}

// formatted counts the text of x by formatting x alone with the current
// directive and verb.
func (c *formatCounter) formatted(x any, verb rune) {
	if c.s.widPresent && uint64(c.s.wid) > c.remaining() {
		c.add(c.s.wid)
		return
	}
	var w countWriter
	fmt.Fprintf(&w, c.directive(c.s, verb), x)
	c.total += uint64(w)
}

// formatter counts the text that the Format method of f writes, as fmt
// calls it.
func (c *formatCounter) formatter(f fmt.Formatter, verb rune) {
	state := &countState{s: c.s}
	defer func() {
		if r := recover(); r != nil {
			// fmt prints "<nil>" for a nil pointer and a short message
			// for any other panic.
			if v := reflect.ValueOf(f); v.Kind() == reflect.Pointer && v.IsNil() {
				c.add(state.n + len("<nil>"))
				return
			}
			c.add(state.n + len(fmt.Sprintf("%%!%c(PANIC=Format method: %v)", verb, r)))
		}
	}()
	f.Format(state, verb)
	c.add(state.n)
}

// countState is the fmt.State that formatter passes to a Format method. It
// reports the flags, width, and precision of the directive and counts the
// bytes written to it.
type countState struct {
	s fmtSpec
	n int
}

func (st *countState) Write(b []byte) (int, error) {
	st.n += len(b)
	return len(b), nil
}

func (st *countState) Width() (int, bool) {
	return st.s.wid, st.s.widPresent
}

func (st *countState) Precision() (int, bool) {
	return st.s.prec, st.s.precPresent
}

func (st *countState) Flag(b int) bool {
	switch b {
	case '-':
		return st.s.minus
	case '+':
		return st.s.plus || st.s.plusV
	case '#':
		return st.s.sharp || st.s.sharpV
	case ' ':
		return st.s.space
	case '0':
		return st.s.zero
	}
	return false
}

// cachedByte counts a byte formatted with directive.
func (c *formatCounter) cachedByte(b byte, directive string) {
	key := byteCacheKey{directive: directive, erroring: c.erroring}
	if key != c.byteCacheKey {
		c.byteCacheKey = key
		c.byteCacheSet = [256]bool{}
	}
	if !c.byteCacheSet[b] {
		var w countWriter
		fmt.Fprintf(&w, directive, b)
		c.byteCache[b] = uint32(w)
		c.byteCacheSet[b] = true
	}
	c.add(int(c.byteCache[b]))
}

// fmtS counts the text of fmt's fmtS: s truncated to the precision and
// padded to the width.
func (c *formatCounter) fmtS(s string) {
	if c.s.precPresent {
		n := c.s.prec
		for i := range s {
			n--
			if n < 0 {
				s = s[:i]
				break
			}
		}
	}
	c.padString(s)
}

// padString counts s padded to the width.
func (c *formatCounter) padString(s string) {
	n := len(s)
	if c.s.widPresent {
		if pad := c.s.wid - utf8.RuneCountInString(s); pad > 0 {
			n += pad
		}
	}
	c.add(n)
}

// bytes counts the text of fmt's fmtBytes.
func (c *formatCounter) bytes(b []byte, verb rune, typeString string) {
	switch verb {
	case 'v', 'd':
		if c.s.sharpV {
			c.add(len(typeString))
			if b == nil {
				c.add(len("(nil)"))
				return
			}
			c.add(2) // {}
			hex := c.s
			hex.sharp = true
			directive := c.directive(hex, 'x')
			for i, ch := range b {
				if c.over() {
					return
				}
				if i > 0 {
					c.add(len(", "))
				}
				c.cachedByte(ch, directive)
			}
		} else {
			c.add(2) // []
			directive := c.directive(c.s, 'd')
			for i, ch := range b {
				if c.over() {
					return
				}
				if i > 0 {
					c.add(1)
				}
				c.cachedByte(ch, directive)
			}
		}
	case 's':
		if c.s.precPresent {
			n := c.s.prec
			for i := 0; i < len(b); {
				n--
				if n < 0 {
					b = b[:i]
					break
				}
				_, wid := utf8.DecodeRune(b[i:])
				i += wid
			}
		}
		n := len(b)
		if c.s.widPresent {
			if pad := c.s.wid - utf8.RuneCount(b); pad > 0 {
				n += pad
			}
		}
		c.add(n)
	case 'x', 'X':
		c.hexSize(len(b))
	case 'q':
		// fmt converts b to a string for %q. Counting reads b in place.
		c.fmtQ(unsafe.String(unsafe.SliceData(b), len(b)))
	default:
		c.value(reflect.ValueOf(b), verb, 0)
	}
}

// hexSize counts the text of fmt's fmtSbx for %x or %X of a string or byte
// slice of length bytes.
func (c *formatCounter) hexSize(length int) {
	if c.s.precPresent && c.s.prec < length {
		length = c.s.prec
	}
	width := 2 * length
	if width > 0 {
		if c.s.space {
			if c.s.sharp {
				width *= 2
			}
			width += length - 1
		} else if c.s.sharp {
			width += 2
		}
	}
	if c.s.widPresent && c.s.wid > width {
		width = c.s.wid
	}
	c.add(width)
}

// fmtQ counts the text of fmt's fmtQ: s truncated to the precision, quoted
// as strconv.Quote does (or strconv.QuoteToASCII with the + flag, or in
// backquotes with the # flag when possible), and padded to the width. It
// counts the quoted text without building it.
func (c *formatCounter) fmtQ(s string) {
	if c.s.precPresent {
		n := c.s.prec
		for i := range s {
			n--
			if n < 0 {
				s = s[:i]
				break
			}
		}
	}
	var size, runes int
	if c.s.sharp && strconv.CanBackquote(s) {
		size, runes = len(s)+2, utf8.RuneCountInString(s)+2
	} else {
		size, runes = quotedSize(s, c.s.plus)
	}
	if c.s.widPresent && c.s.wid > runes {
		size += c.s.wid - runes
	}
	c.add(size)
}

// quotedSize returns the length in bytes and in runes of strconv.Quote(s),
// or of strconv.QuoteToASCII(s) when asciiOnly is true.
func quotedSize(s string, asciiOnly bool) (size, runes int) {
	size, runes = 2, 2
	for i := 0; i < len(s); {
		r, width := utf8.DecodeRuneInString(s[i:])
		i += width
		n := 0
		switch {
		case width == 1 && r == utf8.RuneError:
			n = 4 // \xff
		case r == '"' || r == '\\':
			n = 2
		case r < utf8.RuneSelf && strconv.IsPrint(r):
			n = 1
		case !asciiOnly && strconv.IsPrint(r):
			// A printable rune is copied as it is: width bytes, one rune.
			size += width
			runes++
			continue
		case r == '\a' || r == '\b' || r == '\f' || r == '\n' || r == '\r' || r == '\t' || r == '\v':
			n = 2
		case r < ' ' || r == 0x7f:
			n = 4 // \x00
		case r < 0x10000:
			n = 6 // \u00a0
		default:
			n = 10 // \U000e0001
		}
		size += n
		runes += n
	}
	return size, runes
}

// pointer counts the text of fmt's fmtPointer. arg is the argument that fmt
// names in a %!verb(type=value) error, or nil to name the value v.
func (c *formatCounter) pointer(v reflect.Value, verb rune, arg any) {
	switch v.Kind() {
	case reflect.Chan, reflect.Func, reflect.Map, reflect.Pointer, reflect.Slice, reflect.UnsafePointer:
	default:
		if arg != nil {
			c.badVerb(verb, arg)
		} else {
			c.badVerbValue(verb, v)
		}
		return
	}
	u := uint64(uintptr(v.UnsafePointer()))
	switch verb {
	case 'v':
		if c.s.sharpV {
			c.add(len("()()") + len(v.Type().String()))
			if u == 0 {
				c.add(len("nil"))
			} else {
				c.formattedAs(u, 'x', true)
			}
		} else if u == 0 {
			c.padString("<nil>")
		} else {
			c.formattedAs(u, 'x', !c.s.sharp)
		}
	case 'p':
		c.formattedAs(u, 'x', !c.s.sharp)
	case 'b', 'o', 'd', 'x', 'X':
		c.formatted(u, verb)
	default:
		c.badVerbValue(verb, v)
	}
}

// formattedAs counts u formatted with verb and the current directive, with
// the # flag set to sharp.
func (c *formatCounter) formattedAs(u uint64, verb rune, sharp bool) {
	saved := c.s
	c.s.sharp = sharp
	c.s.sharpV = false
	c.formatted(u, verb)
	c.s = saved
}

// badVerb counts %!verb(type=value) for an argument.
func (c *formatCounter) badVerb(verb rune, arg any) {
	c.add(len("%!") + utf8.RuneLen(verb) + len("(") + len(reflect.TypeOf(arg).String()) + len("=)"))
	erroring := c.erroring
	c.erroring = true
	c.arg(arg, 'v')
	c.erroring = erroring
}

// badVerbValue counts %!verb(type=value) for a value inside an argument.
func (c *formatCounter) badVerbValue(verb rune, v reflect.Value) {
	c.add(len("%!") + utf8.RuneLen(verb) + len("(") + len(v.Type().String()) + len("=)"))
	erroring := c.erroring
	c.erroring = true
	c.value(v, 'v', 0)
	c.erroring = erroring
}

// badVerbNil counts %!verb(<nil>).
func (c *formatCounter) badVerbNil(verb rune) {
	c.add(len("%!") + utf8.RuneLen(verb) + len("(<nil>)"))
}

// tooLarge, parsenum, intFromArg, and parseArgNumber are copies of the
// functions in fmt with the same names, so that the count reads a format as
// fmt does.

func tooLarge(x int) bool {
	const max int = 1e6
	return x > max || x < -max
}

func parsenum(s string, start, end int) (num int, isnum bool, newi int) {
	if start >= end {
		return 0, false, end
	}
	for newi = start; newi < end && '0' <= s[newi] && s[newi] <= '9'; newi++ {
		if tooLarge(num) {
			return 0, false, end
		}
		num = num*10 + int(s[newi]-'0')
		isnum = true
	}
	return
}

func intFromArg(a []any, argNum int) (num int, isInt bool, newArgNum int) {
	newArgNum = argNum
	if argNum < len(a) {
		num, isInt = a[argNum].(int)
		if !isInt {
			switch v := reflect.ValueOf(a[argNum]); v.Kind() {
			case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
				n := v.Int()
				if int64(int(n)) == n {
					num = int(n)
					isInt = true
				}
			case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
				n := v.Uint()
				if int64(n) >= 0 && uint64(int(n)) == n {
					num = int(n)
					isInt = true
				}
			}
		}
		newArgNum = argNum + 1
		if tooLarge(num) {
			num = 0
			isInt = false
		}
	}
	return
}

func parseArgNumber(format string) (index int, wid int, ok bool) {
	if len(format) < 3 {
		return 0, 1, false
	}
	for i := 1; i < len(format); i++ {
		if format[i] == ']' {
			width, ok, newi := parsenum(format, 1, i)
			if !ok || newi != i {
				return 0, i + 1, false
			}
			return width - 1, i + 1, true
		}
	}
	return 0, 1, false
}
