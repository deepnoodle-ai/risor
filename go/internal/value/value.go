// Package value holds the value model of the evaluator. Values are plain Go
// values, read in place (spec/core/03-host.md 3.4):
//
//	null    nil
//	bool    bool
//	number  float64 once read; a host may supply any Go integer or float type
//	string  string, valid UTF-8 once read
//	list    []any, or a host slice or array of another element type
//	map     map[string]any, or a host map with string keys
//	opaque  any other value; a value that implements Opaque names its type
//
// A value the language builds is always one of nil, bool, float64, string,
// []any and map[string]any.
package value

import (
	"math"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"
)

// Kind is one of the seven kinds of value (01-language.md 6.1).
type Kind uint8

// The seven kinds.
const (
	Null Kind = iota
	Bool
	Number
	String
	List
	Map
	OpaqueKind
)

var kindNames = [...]string{"null", "bool", "number", "string", "list", "map", "opaque"}

// String returns the name of the kind as type() gives it.
func (k Kind) String() string { return kindNames[k] }

// Opaque is implemented by a host value that is an opaque value of a declared
// opaque type. OpaqueType returns the type's name.
type Opaque interface {
	OpaqueType() string
}

// MaxWhole is 2^53 - 1, the largest whole number.
const MaxWhole = 9007199254740991

// KindOf returns the kind of a value.
func KindOf(v any) Kind {
	switch v := v.(type) {
	case nil:
		return Null
	case bool:
		return Bool
	case float64, int, int64, int32, int16, int8, uint, uint64, uint32, uint16, uint8, float32:
		return Number
	case string:
		return String
	case []any:
		return List
	case map[string]any:
		return Map
	case Opaque:
		return OpaqueKind
	default:
		return reflectKind(v)
	}
}

func reflectKind(v any) Kind {
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Slice:
		if rv.IsNil() {
			return Null
		}
		return List
	case reflect.Array:
		return List
	case reflect.Map:
		if rv.Type().Key().Kind() != reflect.String {
			return OpaqueKind
		}
		if rv.IsNil() {
			return Null
		}
		return Map
	case reflect.Pointer, reflect.Interface:
		if rv.IsNil() {
			return Null
		}
	}
	return OpaqueKind
}

// ReadError is the reason a host value cannot enter the language.
type ReadError uint8

// The reasons.
const (
	ReadOK ReadError = iota
	ReadRange
)

// Read turns a host value into the form the evaluator works on: a number to
// float64, a string to valid UTF-8 (AC18), a nil slice, map or pointer to
// nil. Containers are left as they are; their elements are read when an
// operation yields them. A host integer whose magnitude is over 2^53 - 1, a
// NaN and an infinity give ReadRange (03-host.md 3.4.4).
func Read(v any) (any, ReadError) {
	switch x := v.(type) {
	case nil, bool, []any, map[string]any:
		return v, ReadOK
	case float64:
		if math.IsNaN(x) || math.IsInf(x, 0) {
			return nil, ReadRange
		}
		if x == 0 {
			return 0.0, ReadOK
		}
		return x, ReadOK
	case string:
		if utf8.ValidString(x) {
			return x, ReadOK
		}
		return ReplaceInvalid(x), ReadOK
	case int:
		return readInt(int64(x))
	case int64:
		return readInt(x)
	case int32:
		return float64(x), ReadOK
	case int16:
		return float64(x), ReadOK
	case int8:
		return float64(x), ReadOK
	case uint:
		return readUint(uint64(x))
	case uint64:
		return readUint(x)
	case uint32:
		return float64(x), ReadOK
	case uint16:
		return float64(x), ReadOK
	case uint8:
		return float64(x), ReadOK
	case float32:
		return Read(float64(x))
	case Opaque:
		return v, ReadOK
	}
	if KindOf(v) == Null {
		return nil, ReadOK
	}
	return v, ReadOK
}

func readInt(x int64) (any, ReadError) {
	if x > MaxWhole || x < -MaxWhole {
		return nil, ReadRange
	}
	return float64(x), ReadOK
}

func readUint(x uint64) (any, ReadError) {
	if x > MaxWhole {
		return nil, ReadRange
	}
	return float64(x), ReadOK
}

// ReplaceInvalid replaces each maximal subpart of an ill-formed UTF-8
// sequence with one U+FFFD, as Unicode recommends (ruling AC18).
func ReplaceInvalid(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); {
		c := s[i]
		if c < 0x80 {
			b.WriteByte(c)
			i++
			continue
		}
		n, lo, hi := 0, byte(0x80), byte(0xBF)
		switch {
		case c >= 0xC2 && c <= 0xDF:
			n = 1
		case c == 0xE0:
			n, lo = 2, 0xA0
		case c >= 0xE1 && c <= 0xEC, c == 0xEE, c == 0xEF:
			n = 2
		case c == 0xED:
			n, hi = 2, 0x9F
		case c == 0xF0:
			n, lo = 3, 0x90
		case c >= 0xF1 && c <= 0xF3:
			n = 3
		case c == 0xF4:
			n, hi = 3, 0x8F
		default:
			b.WriteRune(utf8.RuneError)
			i++
			continue
		}
		j := i + 1
		ok := true
		for k := 0; k < n; k++ {
			if j >= len(s) {
				ok = false
				break
			}
			d := s[j]
			l, h := byte(0x80), byte(0xBF)
			if k == 0 {
				l, h = lo, hi
			}
			if d < l || d > h {
				ok = false
				break
			}
			j++
		}
		if ok {
			b.WriteString(s[i:j])
		} else {
			b.WriteRune(utf8.RuneError)
		}
		i = j
	}
	return b.String()
}

// IsWhole reports whether a number is whole: no fractional part and a
// magnitude of at most 2^53 - 1.
func IsWhole(f float64) bool {
	return f == math.Trunc(f) && f <= MaxWhole && f >= -MaxWhole
}

// Truthy is the truthiness test (01-language.md 6.5).
func Truthy(v any) bool {
	switch x := v.(type) {
	case nil:
		return false
	case bool:
		return x
	case float64:
		return x != 0
	case string:
		return x != ""
	case []any:
		return len(x) > 0
	case map[string]any:
		return len(x) > 0
	}
	switch KindOf(v) {
	case Null:
		return false
	case Number:
		f, _ := Read(v)
		return f != nil && f.(float64) != 0
	case List, Map:
		return reflect.ValueOf(v).Len() > 0
	}
	return true
}

// Len returns the number of elements of a list or entries of a map.
func Len(v any) int {
	switch x := v.(type) {
	case []any:
		return len(x)
	case map[string]any:
		return len(x)
	}
	return reflect.ValueOf(v).Len()
}

// At returns element i of a list, unread.
func At(v any, i int) any {
	if x, ok := v.([]any); ok {
		return x[i]
	}
	return reflect.ValueOf(v).Index(i).Interface()
}

// Get returns the value under key k of a map, unread.
func Get(v any, k string) (any, bool) {
	if m, ok := v.(map[string]any); ok {
		x, ok := m[k]
		return x, ok
	}
	rv := reflect.ValueOf(v)
	kv := reflect.ValueOf(k).Convert(rv.Type().Key())
	x := rv.MapIndex(kv)
	if !x.IsValid() {
		return nil, false
	}
	return x.Interface(), true
}

// Keys returns the keys of a map in code point order.
func Keys(v any) []string {
	var keys []string
	if m, ok := v.(map[string]any); ok {
		keys = make([]string, 0, len(m))
		for k := range m {
			keys = append(keys, k)
		}
	} else {
		rv := reflect.ValueOf(v)
		keys = make([]string, 0, rv.Len())
		for _, k := range rv.MapKeys() {
			keys = append(keys, k.String())
		}
	}
	// Byte order of valid UTF-8 is code point order. A host key that is not
	// valid UTF-8 is compared as written.
	sort.Strings(keys)
	return keys
}

// ForEachKey calls fn for each key of a map, in no particular order.
func ForEachKey(v any, fn func(k string)) {
	if m, ok := v.(map[string]any); ok {
		for k := range m {
			fn(k)
		}
		return
	}
	for _, k := range reflect.ValueOf(v).MapKeys() {
		fn(k.String())
	}
}

// SameOpaque is the identity test of 03-host.md 3.4.6.
func SameOpaque(a, b any) bool {
	ta, tb := reflect.TypeOf(a), reflect.TypeOf(b)
	if ta != tb || !ta.Comparable() {
		return false
	}
	return a == b
}

// OpaqueTypeName returns the declared type name of an opaque value, or "".
func OpaqueTypeName(v any) string {
	if o, ok := v.(Opaque); ok {
		return o.OpaqueType()
	}
	return ""
}

// Weight returns the weight of a string: its code points, those above
// U+FFFF counted twice (02-evaluation.md 4).
func Weight(s string) int64 {
	var w int64
	for i := 0; i < len(s); {
		c := s[i]
		switch {
		case c < 0x80:
			i++
		case c < 0xE0:
			i += 2
		case c < 0xF0:
			i += 3
		default:
			i += 4
			w++
		}
		w++
	}
	return w
}

// Size returns the size of a string: its weight divided by 64.
func Size(s string) int64 { return Weight(s) / 64 }

// CommonPrefix returns the longest common prefix of two strings, by code
// point.
func CommonPrefix(a, b string) string {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	i := 0
	for i < n && a[i] == b[i] {
		i++
	}
	// Back up to the start of a code point.
	for i > 0 && i < len(a) && !utf8.RuneStart(a[i]) {
		i--
	}
	return a[:i]
}

// RuneCount returns the number of code points of a string.
func RuneCount(s string) int { return utf8.RuneCountInString(s) }

// FormatNumber is number to text (01-language.md 6.3), ECMAScript's
// Number::toString with radix 10.
func FormatNumber(f float64) string {
	if f == 0 {
		return "0"
	}
	if f < 0 {
		return "-" + FormatNumber(-f)
	}
	// Shortest digits that read back: d.ddddde±x.
	e := strconv.FormatFloat(f, 'e', -1, 64)
	mant, exp, _ := strings.Cut(e, "e")
	digits := strings.Replace(mant, ".", "", 1)
	x, _ := strconv.Atoi(exp)
	k := len(digits)
	n := x + 1
	switch {
	case k <= n && n <= 21:
		return digits + strings.Repeat("0", n-k)
	case 0 < n && n < k:
		return digits[:n] + "." + digits[n:]
	case -6 < n && n <= 0:
		return "0." + strings.Repeat("0", -n) + digits
	}
	s := digits[:1]
	if k > 1 {
		s += "." + digits[1:]
	}
	if n-1 >= 0 {
		return s + "e+" + strconv.Itoa(n-1)
	}
	return s + "e-" + strconv.Itoa(1-n)
}

// QuoteJSON returns the JSON form of a string (01-language.md 12).
func QuoteJSON(s string) string {
	var b strings.Builder
	b.Grow(len(s) + 2)
	b.WriteByte('"')
	writeEscaped(&b, s)
	b.WriteByte('"')
	return b.String()
}

func writeEscaped(b *strings.Builder, s string) {
	const hex = "0123456789abcdef"
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch c {
		case '"':
			b.WriteString(`\"`)
		case '\\':
			b.WriteString(`\\`)
		case '\b':
			b.WriteString(`\b`)
		case '\f':
			b.WriteString(`\f`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		default:
			if c < 0x20 {
				b.WriteString(`\u00`)
				b.WriteByte(hex[c>>4])
				b.WriteByte(hex[c&15])
			} else {
				b.WriteByte(c)
			}
		}
	}
}

// WriteEscaped writes the inside of the JSON form of a string to b.
func WriteEscaped(b *strings.Builder, s string) { writeEscaped(b, s) }

// IsWord reports whether s is a word of 01-language.md 2.4.
func IsWord(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		letter := (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || c == '_'
		if !letter && !(i > 0 && c >= '0' && c <= '9') {
			return false
		}
	}
	return true
}

// Cut cuts a rendering longer than 64 code points to its first 61 code
// points and "..." (03-host.md 3.8.3 item 1).
func Cut(s string) string {
	if utf8.RuneCountInString(s) <= 64 {
		return s
	}
	n := 0
	for i := range s {
		if n == 61 {
			return s[:i] + "..."
		}
		n++
	}
	return s
}

// Equal is the language's == without charges, for a runner and tests. A host
// value is read as a run reads it.
func Equal(a, b any) bool {
	a, _ = Read(a)
	b, _ = Read(b)
	ka, kb := KindOf(a), KindOf(b)
	if ka != kb {
		return false
	}
	switch ka {
	case Null:
		return true
	case Bool:
		return a.(bool) == b.(bool)
	case Number:
		return a.(float64) == b.(float64)
	case String:
		return a.(string) == b.(string)
	case OpaqueKind:
		return SameOpaque(a, b)
	case List:
		n := Len(a)
		if n != Len(b) {
			return false
		}
		for i := 0; i < n; i++ {
			if !Equal(At(a, i), At(b, i)) {
				return false
			}
		}
		return true
	case Map:
		if Len(a) != Len(b) {
			return false
		}
		for _, k := range Keys(a) {
			bv, ok := Get(b, k)
			if !ok {
				return false
			}
			av, _ := Get(a, k)
			if !Equal(av, bv) {
				return false
			}
		}
		return true
	}
	return false
}

// Debug formats a value for people: the JSON form, with an opaque value as
// opaque(Type).
func Debug(v any) string {
	v, _ = Read(v)
	switch KindOf(v) {
	case Null:
		return "null"
	case Bool:
		if v.(bool) {
			return "true"
		}
		return "false"
	case Number:
		return FormatNumber(v.(float64))
	case String:
		return QuoteJSON(v.(string))
	case List:
		var b strings.Builder
		b.WriteByte('[')
		for i := 0; i < Len(v); i++ {
			if i > 0 {
				b.WriteByte(',')
			}
			b.WriteString(Debug(At(v, i)))
		}
		b.WriteByte(']')
		return b.String()
	case Map:
		var b strings.Builder
		b.WriteByte('{')
		for i, k := range Keys(v) {
			if i > 0 {
				b.WriteByte(',')
			}
			x, _ := Get(v, k)
			b.WriteString(QuoteJSON(k) + ":" + Debug(x))
		}
		b.WriteByte('}')
		return b.String()
	}
	return "opaque(" + OpaqueTypeName(v) + ")"
}
