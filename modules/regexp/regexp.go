package regexp

import (
	"context"
	"fmt"
	"regexp"

	"github.com/risor-io/risor/object"
)

// Compile compiles a regular expression pattern and returns a Regexp object.
func Compile(ctx context.Context, args ...object.Object) (object.Object, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("regexp.compile: expected 1 argument, got %d", len(args))
	}
	pattern, err := object.AsString(args[0])
	if err != nil {
		return nil, err
	}
	r, rErr := regexp.Compile(pattern)
	if rErr != nil {
		return nil, rErr
	}
	return NewRegexp(r), nil
}

// Match tests whether a pattern matches a string.
// This is a convenience function that compiles the pattern each time.
// For repeated use, compile the pattern first with regexp.compile().
func Match(ctx context.Context, args ...object.Object) (object.Object, error) {
	if len(args) != 2 {
		return nil, fmt.Errorf("regexp.match: expected 2 arguments, got %d", len(args))
	}
	pattern, err := object.AsString(args[0])
	if err != nil {
		return nil, err
	}
	str, err := object.AsString(args[1])
	if err != nil {
		return nil, err
	}
	matched, rErr := regexp.MatchString(pattern, str)
	if rErr != nil {
		return nil, rErr
	}
	return object.NewBool(matched), nil
}

// Escape returns a string with all regular expression metacharacters escaped.
// The returned string is a regular expression that matches the literal text.
func Escape(ctx context.Context, args ...object.Object) (object.Object, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("regexp.escape: expected 1 argument, got %d", len(args))
	}
	str, err := object.AsString(args[0])
	if err != nil {
		return nil, err
	}
	return object.NewString(regexp.QuoteMeta(str)), nil
}

// Replace replaces matches in a string.
// With 3 arguments (pattern, str, repl): replaces all matches.
// With 4 arguments (pattern, str, repl, count): replaces up to count matches (0 = all).
func Replace(ctx context.Context, args ...object.Object) (object.Object, error) {
	if len(args) < 3 || len(args) > 4 {
		return nil, fmt.Errorf("regexp.replace: expected 3-4 arguments, got %d", len(args))
	}
	pattern, err := object.AsString(args[0])
	if err != nil {
		return nil, err
	}
	str, err := object.AsString(args[1])
	if err != nil {
		return nil, err
	}
	repl, err := object.AsString(args[2])
	if err != nil {
		return nil, err
	}

	r, rErr := regexp.Compile(pattern)
	if rErr != nil {
		return nil, rErr
	}

	count := 0 // 0 means replace all
	if len(args) == 4 {
		c, err := object.AsInt(args[3])
		if err != nil {
			return nil, err
		}
		count = int(c)
	}

	if count == 0 {
		return object.NewString(r.ReplaceAllString(str, repl)), nil
	}

	// Replace up to count matches
	result := str
	for i := 0; i < count; i++ {
		loc := r.FindStringIndex(result)
		if loc == nil {
			break
		}
		// Expand replacement (handles $1, $2, etc.)
		expanded := r.ReplaceAllString(result[loc[0]:loc[1]], repl)
		result = result[:loc[0]] + expanded + result[loc[1]:]
	}
	return object.NewString(result), nil
}

// Split splits a string by a pattern.
// With 2 arguments (pattern, str): splits into all substrings.
// With 3 arguments (pattern, str, n): splits into at most n substrings.
func Split(ctx context.Context, args ...object.Object) (object.Object, error) {
	if len(args) < 2 || len(args) > 3 {
		return nil, fmt.Errorf("regexp.split: expected 2-3 arguments, got %d", len(args))
	}
	pattern, err := object.AsString(args[0])
	if err != nil {
		return nil, err
	}
	str, err := object.AsString(args[1])
	if err != nil {
		return nil, err
	}

	r, rErr := regexp.Compile(pattern)
	if rErr != nil {
		return nil, rErr
	}

	n := -1 // -1 means split all
	if len(args) == 3 {
		nVal, err := object.AsInt(args[2])
		if err != nil {
			return nil, err
		}
		n = int(nVal)
	}

	parts := r.Split(str, n)
	result := make([]object.Object, len(parts))
	for i, part := range parts {
		result[i] = object.NewString(part)
	}
	return object.NewList(result), nil
}

// Find returns the first match of pattern in string, or null if no match.
func Find(ctx context.Context, args ...object.Object) (object.Object, error) {
	if len(args) != 2 {
		return nil, fmt.Errorf("regexp.find: expected 2 arguments, got %d", len(args))
	}
	pattern, err := object.AsString(args[0])
	if err != nil {
		return nil, err
	}
	str, err := object.AsString(args[1])
	if err != nil {
		return nil, err
	}

	r, rErr := regexp.Compile(pattern)
	if rErr != nil {
		return nil, rErr
	}

	match := r.FindString(str)
	if match == "" && !r.MatchString(str) {
		return object.Nil, nil
	}
	return object.NewString(match), nil
}

// FindAll returns all matches of pattern in string.
// With 3 arguments, limits to n matches.
func FindAll(ctx context.Context, args ...object.Object) (object.Object, error) {
	if len(args) < 2 || len(args) > 3 {
		return nil, fmt.Errorf("regexp.find_all: expected 2-3 arguments, got %d", len(args))
	}
	pattern, err := object.AsString(args[0])
	if err != nil {
		return nil, err
	}
	str, err := object.AsString(args[1])
	if err != nil {
		return nil, err
	}

	r, rErr := regexp.Compile(pattern)
	if rErr != nil {
		return nil, rErr
	}

	n := -1
	if len(args) == 3 {
		nVal, err := object.AsInt(args[2])
		if err != nil {
			return nil, err
		}
		n = int(nVal)
	}

	matches := r.FindAllString(str, n)
	result := make([]object.Object, len(matches))
	for i, m := range matches {
		result[i] = object.NewString(m)
	}
	return object.NewList(result), nil
}

// Search returns the index of the first match, or -1 if no match.
func Search(ctx context.Context, args ...object.Object) (object.Object, error) {
	if len(args) != 2 {
		return nil, fmt.Errorf("regexp.search: expected 2 arguments, got %d", len(args))
	}
	pattern, err := object.AsString(args[0])
	if err != nil {
		return nil, err
	}
	str, err := object.AsString(args[1])
	if err != nil {
		return nil, err
	}

	r, rErr := regexp.Compile(pattern)
	if rErr != nil {
		return nil, rErr
	}

	loc := r.FindStringIndex(str)
	if loc == nil {
		return object.NewInt(-1), nil
	}
	// Convert byte index to rune index for Unicode correctness
	runeIndex := len([]rune(str[:loc[0]]))
	return object.NewInt(int64(runeIndex)), nil
}

func Module() *object.Module {
	return object.NewBuiltinsModule("regexp", map[string]object.Object{
		"compile":  object.NewBuiltin("compile", Compile),
		"match":    object.NewBuiltin("match", Match),
		"escape":   object.NewBuiltin("escape", Escape),
		"replace":  object.NewBuiltin("replace", Replace),
		"split":    object.NewBuiltin("split", Split),
		"find":     object.NewBuiltin("find", Find),
		"find_all": object.NewBuiltin("find_all", FindAll),
		"search":   object.NewBuiltin("search", Search),
	}, Compile)
}
