package strings

import (
	"context"
	"math"
	"strings"

	"github.com/risor-io/risor/object"
)

// Contains checks if substr is within s.
func Contains(ctx context.Context, args ...object.Object) object.Object {
	if len(args) != 2 {
		return object.NewArgsError("strings.contains", 2, len(args))
	}
	s, err := object.AsString(args[0])
	if err != nil {
		return err
	}
	substr, err := object.AsString(args[1])
	if err != nil {
		return err
	}
	return object.NewBool(strings.Contains(s, substr))
}

// HasPrefix tests whether the string s begins with prefix.
func HasPrefix(ctx context.Context, args ...object.Object) object.Object {
	if len(args) != 2 {
		return object.NewArgsError("strings.has_prefix", 2, len(args))
	}
	s, err := object.AsString(args[0])
	if err != nil {
		return err
	}
	prefix, err := object.AsString(args[1])
	if err != nil {
		return err
	}
	return object.NewBool(strings.HasPrefix(s, prefix))
}

// HasSuffix tests whether the string s ends with suffix.
func HasSuffix(ctx context.Context, args ...object.Object) object.Object {
	if len(args) != 2 {
		return object.NewArgsError("strings.has_suffix", 2, len(args))
	}
	s, err := object.AsString(args[0])
	if err != nil {
		return err
	}
	suffix, err := object.AsString(args[1])
	if err != nil {
		return err
	}
	return object.NewBool(strings.HasSuffix(s, suffix))
}

// Count counts the number of non-overlapping instances of substr in s.
func Count(ctx context.Context, args ...object.Object) object.Object {
	if len(args) != 2 {
		return object.NewArgsError("strings.count", 2, len(args))
	}
	s, err := object.AsString(args[0])
	if err != nil {
		return err
	}
	substr, err := object.AsString(args[1])
	if err != nil {
		return err
	}
	return object.NewInt(int64(strings.Count(s, substr)))
}

// Compare returns an integer comparing two strings lexicographically.
func Compare(ctx context.Context, args ...object.Object) object.Object {
	if len(args) != 2 {
		return object.NewArgsError("strings.compare", 2, len(args))
	}
	a, err := object.AsString(args[0])
	if err != nil {
		return err
	}
	b, err := object.AsString(args[1])
	if err != nil {
		return err
	}
	return object.NewInt(int64(strings.Compare(a, b)))
}

// Repeat returns a new string consisting of count copies of the string s.
func Repeat(ctx context.Context, args ...object.Object) object.Object {
	if len(args) != 2 {
		return object.NewArgsError("strings.repeat", 2, len(args))
	}
	s, err := object.AsString(args[0])
	if err != nil {
		return err
	}
	countRaw, err := object.AsInt(args[1])
	if err != nil {
		return err
	}
	if countRaw > math.MaxInt {
		return object.TypeErrorf("type error: strings.repeat argument 'count' (index 1) cannot be > %v", math.MaxInt)
	}
	if countRaw < math.MinInt {
		return object.TypeErrorf("type error: strings.repeat argument 'count' (index 1) cannot be < %v", math.MinInt)
	}
	return object.NewString(strings.Repeat(s, int(countRaw)))
}

// Join concatenates the elements of its first argument to create a single string.
func Join(ctx context.Context, args ...object.Object) object.Object {
	if len(args) != 2 {
		return object.NewArgsError("strings.join", 2, len(args))
	}
	list, err := object.AsStringSlice(args[0])
	if err != nil {
		return err
	}
	sep, err := object.AsString(args[1])
	if err != nil {
		return err
	}
	return object.NewString(strings.Join(list, sep))
}

// Split slices s into all substrings separated by sep.
func Split(ctx context.Context, args ...object.Object) object.Object {
	if len(args) != 2 {
		return object.NewArgsError("strings.split", 2, len(args))
	}
	s, err := object.AsString(args[0])
	if err != nil {
		return err
	}
	sep, err := object.AsString(args[1])
	if err != nil {
		return err
	}
	return object.NewStringList(strings.Split(s, sep))
}

// Fields splits the string s around each instance of one or more consecutive white space characters.
func Fields(ctx context.Context, args ...object.Object) object.Object {
	if len(args) != 1 {
		return object.NewArgsError("strings.fields", 1, len(args))
	}
	s, err := object.AsString(args[0])
	if err != nil {
		return err
	}
	return object.NewStringList(strings.Fields(s))
}

// Index returns the index of the first instance of substr in s, or -1 if not present.
func Index(ctx context.Context, args ...object.Object) object.Object {
	if len(args) != 2 {
		return object.NewArgsError("strings.index", 2, len(args))
	}
	s, err := object.AsString(args[0])
	if err != nil {
		return err
	}
	substr, err := object.AsString(args[1])
	if err != nil {
		return err
	}
	return object.NewInt(int64(strings.Index(s, substr)))
}

// LastIndex returns the index of the last instance of substr in s, or -1 if not present.
func LastIndex(ctx context.Context, args ...object.Object) object.Object {
	if len(args) != 2 {
		return object.NewArgsError("strings.last_index", 2, len(args))
	}
	s, err := object.AsString(args[0])
	if err != nil {
		return err
	}
	substr, err := object.AsString(args[1])
	if err != nil {
		return err
	}
	return object.NewInt(int64(strings.LastIndex(s, substr)))
}

// ReplaceAll returns a copy of the string s with all non-overlapping instances of old replaced by new.
func ReplaceAll(ctx context.Context, args ...object.Object) object.Object {
	if len(args) != 3 {
		return object.NewArgsError("strings.replace_all", 3, len(args))
	}
	s, err := object.AsString(args[0])
	if err != nil {
		return err
	}
	old, err := object.AsString(args[1])
	if err != nil {
		return err
	}
	new, err := object.AsString(args[2])
	if err != nil {
		return err
	}
	return object.NewString(strings.ReplaceAll(s, old, new))
}

// ToLower returns s with all Unicode letters mapped to their lower case.
func ToLower(ctx context.Context, args ...object.Object) object.Object {
	if len(args) != 1 {
		return object.NewArgsError("strings.to_lower", 1, len(args))
	}
	s, err := object.AsString(args[0])
	if err != nil {
		return err
	}
	return object.NewString(strings.ToLower(s))
}

// ToUpper returns s with all Unicode letters mapped to their upper case.
func ToUpper(ctx context.Context, args ...object.Object) object.Object {
	if len(args) != 1 {
		return object.NewArgsError("strings.to_upper", 1, len(args))
	}
	s, err := object.AsString(args[0])
	if err != nil {
		return err
	}
	return object.NewString(strings.ToUpper(s))
}

// Trim returns a slice of the string s with all leading and trailing Unicode code points contained in cutset removed.
func Trim(ctx context.Context, args ...object.Object) object.Object {
	if len(args) != 2 {
		return object.NewArgsError("strings.trim", 2, len(args))
	}
	s, err := object.AsString(args[0])
	if err != nil {
		return err
	}
	cutset, err := object.AsString(args[1])
	if err != nil {
		return err
	}
	return object.NewString(strings.Trim(s, cutset))
}

// TrimPrefix returns s without the provided leading prefix string.
func TrimPrefix(ctx context.Context, args ...object.Object) object.Object {
	if len(args) != 2 {
		return object.NewArgsError("strings.trim_prefix", 2, len(args))
	}
	s, err := object.AsString(args[0])
	if err != nil {
		return err
	}
	prefix, err := object.AsString(args[1])
	if err != nil {
		return err
	}
	return object.NewString(strings.TrimPrefix(s, prefix))
}

// TrimSuffix returns s without the provided trailing suffix string.
func TrimSuffix(ctx context.Context, args ...object.Object) object.Object {
	if len(args) != 2 {
		return object.NewArgsError("strings.trim_suffix", 2, len(args))
	}
	s, err := object.AsString(args[0])
	if err != nil {
		return err
	}
	suffix, err := object.AsString(args[1])
	if err != nil {
		return err
	}
	return object.NewString(strings.TrimSuffix(s, suffix))
}

// TrimSpace returns a slice of the string s, with all leading and trailing white space removed.
func TrimSpace(ctx context.Context, args ...object.Object) object.Object {
	if len(args) != 1 {
		return object.NewArgsError("strings.trim_space", 1, len(args))
	}
	s, err := object.AsString(args[0])
	if err != nil {
		return err
	}
	return object.NewString(strings.TrimSpace(s))
}

// Module returns the Risor module object with all the associated builtin functions.
func Module() *object.Module {
	return object.NewBuiltinsModule("strings", map[string]object.Object{
		"contains":    object.NewBuiltin("contains", Contains),
		"has_prefix":  object.NewBuiltin("has_prefix", HasPrefix),
		"has_suffix":  object.NewBuiltin("has_suffix", HasSuffix),
		"count":       object.NewBuiltin("count", Count),
		"compare":     object.NewBuiltin("compare", Compare),
		"repeat":      object.NewBuiltin("repeat", Repeat),
		"join":        object.NewBuiltin("join", Join),
		"split":       object.NewBuiltin("split", Split),
		"fields":      object.NewBuiltin("fields", Fields),
		"index":       object.NewBuiltin("index", Index),
		"last_index":  object.NewBuiltin("last_index", LastIndex),
		"replace_all": object.NewBuiltin("replace_all", ReplaceAll),
		"to_lower":    object.NewBuiltin("to_lower", ToLower),
		"to_upper":    object.NewBuiltin("to_upper", ToUpper),
		"trim":        object.NewBuiltin("trim", Trim),
		"trim_prefix": object.NewBuiltin("trim_prefix", TrimPrefix),
		"trim_suffix": object.NewBuiltin("trim_suffix", TrimSuffix),
		"trim_space":  object.NewBuiltin("trim_space", TrimSpace),
	})
}
