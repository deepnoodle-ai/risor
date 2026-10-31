package object

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/deepnoodle-ai/risor/v2/pkg/op"
)

var stringMethods = NewMethodRegistry[*String]("string")

func init() {
	stringMethods.Define("compare").
		Doc("Compare to a string or bytes (-1, 0, or 1)").
		Arg("other").
		Returns("int").
		Impl(func(s *String, ctx context.Context, args ...Object) (Object, error) {
			result, err := s.Compare(args[0])
			if err != nil {
				return nil, err
			}
			return NewInt(int64(result)), nil
		})

	stringMethods.Define("contains").
		Doc("Check if substring exists").
		Arg("substr").
		Returns("bool").
		Impl(func(s *String, ctx context.Context, args ...Object) (Object, error) {
			return s.Contains(args[0]), nil
		})

	stringMethods.Define("count").
		Doc("Count occurrences of substring").
		Arg("substr").
		Returns("int").
		Impl(func(s *String, ctx context.Context, args ...Object) (Object, error) {
			return s.Count(args[0])
		})

	stringMethods.Define("fields").
		Doc("Split on whitespace").
		Returns("list").
		Impl(func(s *String, ctx context.Context, args ...Object) (Object, error) {
			return s.Fields(ctx)
		})

	stringMethods.Define("has_prefix").
		Doc("Check if string starts with prefix").
		Arg("prefix").
		Returns("bool").
		Impl(func(s *String, ctx context.Context, args ...Object) (Object, error) {
			return s.HasPrefix(args[0])
		})

	stringMethods.Define("has_suffix").
		Doc("Check if string ends with suffix").
		Arg("suffix").
		Returns("bool").
		Impl(func(s *String, ctx context.Context, args ...Object) (Object, error) {
			return s.HasSuffix(args[0])
		})

	stringMethods.Define("index").
		Doc("Find first rune index of substring (-1 if not found)").
		Arg("substr").
		Returns("int").
		Impl(func(s *String, ctx context.Context, args ...Object) (Object, error) {
			return s.Index(args[0])
		})

	stringMethods.Define("join").
		Doc("Join list elements with this string as separator").
		Arg("items").
		Returns("string").
		Impl(func(s *String, ctx context.Context, args ...Object) (Object, error) {
			return s.Join(ctx, args[0])
		})

	stringMethods.Define("last_index").
		Doc("Find last rune index of substring (-1 if not found)").
		Arg("substr").
		Returns("int").
		Impl(func(s *String, ctx context.Context, args ...Object) (Object, error) {
			return s.LastIndex(args[0])
		})

	stringMethods.Define("repeat").
		Doc("Repeat string n times").
		Arg("count").
		Returns("string").
		Impl(func(s *String, ctx context.Context, args ...Object) (Object, error) {
			return s.Repeat(ctx, args[0])
		})

	stringMethods.Define("replace_all").
		Doc("Replace all occurrences").
		Args("old", "new").
		Returns("string").
		Impl(func(s *String, ctx context.Context, args ...Object) (Object, error) {
			return s.ReplaceAll(ctx, args[0], args[1])
		})

	stringMethods.Define("split").
		Doc("Split by separator").
		Arg("sep").
		Returns("list").
		Impl(func(s *String, ctx context.Context, args ...Object) (Object, error) {
			return s.Split(ctx, args[0])
		})

	stringMethods.Define("to_lower").
		Doc("Convert to lowercase").
		Returns("string").
		Impl(func(s *String, ctx context.Context, args ...Object) (Object, error) {
			return s.ToLower(), nil
		})

	stringMethods.Define("to_upper").
		Doc("Convert to uppercase").
		Returns("string").
		Impl(func(s *String, ctx context.Context, args ...Object) (Object, error) {
			return s.ToUpper(), nil
		})

	stringMethods.Define("trim").
		Doc("Trim characters from both ends").
		Arg("chars").
		Returns("string").
		Impl(func(s *String, ctx context.Context, args ...Object) (Object, error) {
			return s.Trim(args[0])
		})

	stringMethods.Define("trim_prefix").
		Doc("Remove prefix if present").
		Arg("prefix").
		Returns("string").
		Impl(func(s *String, ctx context.Context, args ...Object) (Object, error) {
			return s.TrimPrefix(args[0])
		})

	stringMethods.Define("trim_space").
		Doc("Trim whitespace from both ends").
		Returns("string").
		Impl(func(s *String, ctx context.Context, args ...Object) (Object, error) {
			return s.TrimSpace(), nil
		})

	stringMethods.Define("trim_suffix").
		Doc("Remove suffix if present").
		Arg("suffix").
		Returns("string").
		Impl(func(s *String, ctx context.Context, args ...Object) (Object, error) {
			return s.TrimSuffix(args[0])
		})
}

type String struct {
	value string
}

func (s *String) Attrs() []AttrSpec {
	return stringMethods.Specs()
}

func (s *String) GetAttr(name string) (Object, bool) {
	return stringMethods.GetAttr(s, name)
}

func (s *String) SetAttr(name string, value Object) error {
	return TypeErrorf("string has no attribute %q", name)
}

func (s *String) Type() Type {
	return STRING
}

func (s *String) Value() string {
	return s.value
}

func (s *String) Inspect() string {
	sLen := len(s.value)
	if sLen >= 2 {
		if s.value[0] == '"' && s.value[sLen-1] == '"' {
			if strings.Count(s.value, "\"") == 2 {
				return fmt.Sprintf("'%s'", s.value)
			}
		}
	}
	return fmt.Sprintf("%q", s.value)
}

func (s *String) String() string {
	return s.value
}

func (s *String) Interface() interface{} {
	return s.value
}

func (s *String) Compare(other Object) (int, error) {
	switch other := other.(type) {
	case *String:
		return strings.Compare(s.value, other.value), nil
	case *Bytes:
		return strings.Compare(s.value, string(other.value)), nil
	default:
		return 0, TypeErrorf("unable to compare string and %s", other.Type())
	}
}

func (s *String) Equals(other Object) bool {
	switch other := other.(type) {
	case *String:
		return s.value == other.value
	case *Bytes:
		return s.value == string(other.value)
	}
	return false
}

func (s *String) IsTruthy() bool {
	return s.value != ""
}

func (s *String) RunOperation(opType op.BinaryOpType, right Object) (Object, error) {
	switch right := right.(type) {
	case *String:
		return s.runOperationString(opType, right)
	default:
		return nil, newTypeErrorf("unsupported operation for string: %v on type %s", opType, right.Type())
	}
}

func (s *String) runOperationString(opType op.BinaryOpType, right *String) (Object, error) {
	switch opType {
	case op.Add:
		return NewString(s.value + right.value), nil
	default:
		return nil, newTypeErrorf("unsupported operation for string: %v on type %s", opType, right.Type())
	}
}

func (s *String) Reversed() *String {
	runes := []rune(s.value)
	for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
		runes[i], runes[j] = runes[j], runes[i]
	}
	return NewString(string(runes))
}

func (s *String) GetItem(key Object) (Object, *Error) {
	indexObj, ok := key.(*Int)
	if !ok {
		return nil, TypeErrorf("string index must be an int (got %s)", key.Type())
	}
	v := s.value
	if off, ok := runeOffset(v, indexObj.value); ok && off < len(v) {
		return NewString(runeAt(v, off)), nil
	}
	// The index is out of range. ResolveIndex reports it with the same
	// error as for a list.
	_, err := ResolveIndex(indexObj.value, int64(utf8.RuneCountInString(v)))
	return nil, NewError(err)
}

func (s *String) GetSlice(slice Slice) (Object, *Error) {
	v := s.value
	start, startOK := sliceBound(slice.Start)
	stop, stopOK := sliceBound(slice.Stop)
	if startOK && stopOK {
		lo, ok := runeOffset(v, start)
		hi := len(v)
		if ok && slice.Stop != nil {
			if start >= 0 && stop >= start {
				hi, ok = skipRunes(v, lo, stop-start)
			} else {
				hi, ok = runeOffset(v, stop)
			}
		}
		if ok && lo <= hi {
			return NewString(replaceInvalidBytes(v[lo:hi])), nil
		}
	}
	// A bound has the wrong type or is out of range, or start is after
	// stop. ResolveIntSlice reports it with the same error as for a list.
	_, _, err := ResolveIntSlice(slice, int64(utf8.RuneCountInString(v)))
	return nil, NewError(err)
}

// sliceBound returns the value of a slice bound. A nil bound is 0 and ok;
// a bound that is not an Int is not ok.
func sliceBound(obj Object) (int64, bool) {
	if obj == nil {
		return 0, true
	}
	i, ok := obj.(*Int)
	if !ok {
		return 0, false
	}
	return i.value, true
}

// runeOffset returns the byte offset in v of rune index i, where a negative
// i counts from the end. Offset len(v) is rune index len(runes). It reports
// false if i is out of range. It scans from the start of v for i >= 0 and
// from the end for i < 0, so it reads only the runes it skips.
func runeOffset(v string, i int64) (int, bool) {
	if i >= 0 {
		return skipRunes(v, 0, i)
	}
	if i == math.MinInt64 {
		return 0, false // -i overflows, and no string has that many runes
	}
	return skipRunesBack(v, len(v), -i)
}

// skipRunes returns the byte offset in v that is count runes after the byte
// offset off, which must start a rune. It reports false if v ends first.
// Runes are counted as a range loop counts them: a valid UTF-8 sequence is
// one rune, and each invalid byte is one rune.
func skipRunes(v string, off int, count int64) (int, bool) {
	for ; count > 0; count-- {
		if off >= len(v) {
			return off, false
		}
		if v[off] < utf8.RuneSelf {
			off++
			continue
		}
		_, size := utf8.DecodeRuneInString(v[off:])
		off += size
	}
	return off, true
}

// skipRunesBack returns the byte offset in v that is count runes before the
// byte offset end, which must start a rune or be len(v). It reports false
// if v starts first. It finds the same runes as skipRunes: a valid sequence
// that ends at end starts at the nearest preceding non-continuation byte,
// and utf8.DecodeLastRuneInString returns size 1 for anything else, which
// a forward scan also decodes as one invalid byte.
func skipRunesBack(v string, end int, count int64) (int, bool) {
	for ; count > 0; count-- {
		if end <= 0 {
			return end, false
		}
		if v[end-1] < utf8.RuneSelf {
			end--
			continue
		}
		_, size := utf8.DecodeLastRuneInString(v[:end])
		end -= size
	}
	return end, true
}

// runeAt returns the rune that starts at byte offset off in v, as a string.
func runeAt(v string, off int) string {
	_, size := utf8.DecodeRuneInString(v[off:])
	return replaceInvalidBytes(v[off : off+size])
}

// replaceInvalidBytes returns v with each invalid UTF-8 byte replaced by
// U+FFFD, the same result as string([]rune(v)). When v is valid UTF-8,
// which is the usual case, it returns v itself, sharing its bytes without
// a copy.
func replaceInvalidBytes(v string) string {
	if utf8.ValidString(v) {
		return v
	}
	var b strings.Builder
	b.Grow(len(v))
	for _, r := range v {
		b.WriteRune(r)
	}
	return b.String()
}

func (s *String) SetItem(key, value Object) *Error {
	return TypeErrorf("set item is unsupported for string")
}

func (s *String) DelItem(key Object) *Error {
	return TypeErrorf("del item is unsupported for string")
}

func (s *String) Contains(obj Object) *Bool {
	other, err := AsString(obj)
	if err != nil {
		return False
	}
	return NewBool(strings.Contains(s.value, other))
}

func (s *String) HasPrefix(obj Object) (Object, error) {
	prefix, err := AsString(obj)
	if err != nil {
		return nil, err
	}
	return NewBool(strings.HasPrefix(s.value, prefix)), nil
}

func (s *String) HasSuffix(obj Object) (Object, error) {
	suffix, err := AsString(obj)
	if err != nil {
		return nil, err
	}
	return NewBool(strings.HasSuffix(s.value, suffix)), nil
}

func (s *String) Count(obj Object) (Object, error) {
	substr, err := AsString(obj)
	if err != nil {
		return nil, err
	}
	return NewInt(int64(strings.Count(s.value, substr))), nil
}

func (s *String) Join(ctx context.Context, obj Object) (Object, error) {
	ls, err := AsList(obj)
	if err != nil {
		return nil, err
	}
	var strs []string
	var size uint64
	for i, item := range ls.Value() {
		itemStr, err := AsString(item)
		if err != nil {
			return nil, err
		}
		size += uint64(len(itemStr))
		if i > 0 {
			size += uint64(len(s.value))
		}
		if err := checkBytesSize(GetLimits(ctx), "string.join()", size); err != nil {
			return nil, err
		}
		strs = append(strs, itemStr)
	}
	return NewString(strings.Join(strs, s.value)), nil
}

// Split splits s around each instance of the separator. It returns a value
// error if the result would exceed the list length limit in ctx.
func (s *String) Split(ctx context.Context, obj Object) (Object, error) {
	sep, err := AsString(obj)
	if err != nil {
		return nil, err
	}
	// Count the parts first: strings.SplitN with a positive n allocates for n
	// parts, not for the number of separators it finds.
	n := strings.Count(s.value, sep) + 1
	if sep == "" {
		n = utf8.RuneCountInString(s.value)
	}
	if err := CheckListLength(ctx, n); err != nil {
		return nil, err
	}
	return NewStringList(strings.Split(s.value, sep)), nil
}

// Fields splits s around runs of white space. It returns a value error if
// the result would exceed the list length limit in ctx.
func (s *String) Fields(ctx context.Context) (Object, error) {
	if err := CheckListLength(ctx, countFields(s.value)); err != nil {
		return nil, err
	}
	return NewStringList(strings.Fields(s.value)), nil
}

// countFields returns the number of fields strings.Fields would return.
func countFields(s string) int {
	n := 0
	inField := false
	for _, r := range s {
		space := unicode.IsSpace(r)
		if !space && !inField {
			n++
		}
		inField = !space
	}
	return n
}

func (s *String) Index(obj Object) (Object, error) {
	substr, err := AsString(obj)
	if err != nil {
		return nil, err
	}
	if utf8.ValidString(substr) {
		return NewInt(s.runeIndex(strings.Index(s.value, substr))), nil
	}
	for from := 0; from <= len(s.value); {
		off := strings.Index(s.value[from:], substr)
		if off < 0 {
			break
		}
		off += from
		if s.isRuneBoundary(off) && s.isRuneBoundary(off+len(substr)) {
			return NewInt(s.runeIndex(off)), nil
		}
		from = off + 1
	}
	return NewInt(-1), nil
}

func (s *String) LastIndex(obj Object) (Object, error) {
	substr, err := AsString(obj)
	if err != nil {
		return nil, err
	}
	if utf8.ValidString(substr) {
		return NewInt(s.runeIndex(strings.LastIndex(s.value, substr))), nil
	}
	for end := len(s.value); end >= 0; {
		off := strings.LastIndex(s.value[:end], substr)
		if off < 0 {
			break
		}
		if s.isRuneBoundary(off) && s.isRuneBoundary(off+len(substr)) {
			return NewInt(s.runeIndex(off)), nil
		}
		end = off + len(substr) - 1
	}
	return NewInt(-1), nil
}

// runeIndex converts a byte offset into s to a rune index, the unit that
// s[i], slicing, and len(s) use. A negative offset (not found) is returned
// as -1. The offset must fall on a rune boundary. A match of a valid UTF-8
// substring always does: a valid substring cannot start or end inside a
// rune of s, because invalid bytes in s decode as one rune each.
func (s *String) runeIndex(byteOffset int) int64 {
	if byteOffset < 0 {
		return -1
	}
	return int64(utf8.RuneCountInString(s.value[:byteOffset]))
}

// isRuneBoundary reports whether the byte offset i in s starts a rune (or
// is len(s)), using the same decoding as a range loop: a valid sequence is
// one rune, and each invalid byte is one rune. Index and LastIndex use it
// for an invalid UTF-8 substring, which can match inside a rune of s. They
// skip those matches, so s[i] accepts every result. The check looks back
// at most utf8.UTFMax-1 bytes, so it allocates nothing.
func (s *String) isRuneBoundary(i int) bool {
	v := s.value
	if i <= 0 || i >= len(v) || utf8.RuneStart(v[i]) {
		return true
	}
	// v[i] is a continuation byte. It is inside a rune only when the
	// nearest preceding non-continuation byte begins a valid sequence that
	// extends past i. A non-continuation byte always starts a rune.
	for j := i - 1; j >= 0 && j > i-utf8.UTFMax; j-- {
		if utf8.RuneStart(v[j]) {
			_, size := utf8.DecodeRuneInString(v[j:])
			return j+size <= i
		}
	}
	return true
}

func (s *String) ReplaceAll(ctx context.Context, old, new Object) (Object, error) {
	oldStr, err := AsString(old)
	if err != nil {
		return nil, err
	}
	newStr, err := AsString(new)
	if err != nil {
		return nil, err
	}
	if err := checkReplaceSize(GetLimits(ctx), "string.replace_all()", len(s.value), len(oldStr), len(newStr),
		strings.Count(s.value, oldStr)); err != nil {
		return nil, err
	}
	return NewString(strings.ReplaceAll(s.value, oldStr, newStr)), nil
}

func (s *String) ToLower() Object {
	return NewString(strings.ToLower(s.value))
}

func (s *String) ToUpper() Object {
	return NewString(strings.ToUpper(s.value))
}

func (s *String) Trim(obj Object) (Object, error) {
	chars, err := AsString(obj)
	if err != nil {
		return nil, err
	}
	return NewString(strings.Trim(s.value, chars)), nil
}

func (s *String) TrimPrefix(obj Object) (Object, error) {
	prefix, err := AsString(obj)
	if err != nil {
		return nil, err
	}
	return NewString(strings.TrimPrefix(s.value, prefix)), nil
}

func (s *String) TrimSuffix(obj Object) (Object, error) {
	suffix, err := AsString(obj)
	if err != nil {
		return nil, err
	}
	return NewString(strings.TrimSuffix(s.value, suffix)), nil
}

func (s *String) TrimSpace() Object {
	return NewString(strings.TrimSpace(s.value))
}

func (s *String) Repeat(ctx context.Context, obj Object) (Object, error) {
	count, err := AsInt(obj)
	if err != nil {
		return nil, err
	}
	if count < 0 {
		return nil, newValueErrorf("negative repeat count")
	}
	if err := checkRepeatSize(GetLimits(ctx), "string.repeat()", len(s.value), count); err != nil {
		return nil, err
	}
	return NewString(strings.Repeat(s.value, int(count))), nil
}

func (s *String) Len() *Int {
	return NewInt(int64(len([]rune(s.value))))
}

// Enumerate calls fn for each rune, with its rune index as the key.
func (s *String) Enumerate(ctx context.Context, fn func(key, value Object) bool) {
	i := int64(0)
	for _, r := range s.value {
		if !fn(NewInt(i), NewString(string(r))) {
			return
		}
		i++
	}
}

func (s *String) Runes() []Object {
	runes := []rune(s.value)
	result := make([]Object, len(runes))
	for i, r := range runes {
		result[i] = NewString(string(r))
	}
	return result
}

func (s *String) MarshalJSON() ([]byte, error) {
	return json.Marshal(s.value)
}

func NewString(s string) *String {
	return &String{value: s}
}
