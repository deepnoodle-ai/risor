package regexp

import (
	"context"
	"fmt"
	"regexp"

	"github.com/deepnoodle-ai/risor/v2/pkg/object"
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

	return replace(ctx, r, str, repl, count)
}

// replace replaces up to count non-overlapping matches of r in str with repl,
// expanding $1, $2, etc. A count of 0 replaces all matches and a negative
// count replaces none. Matches are found in a single pass over the original
// string, so replacement text is never re-matched.
//
// The result size is checked against the byte limit in ctx before it is built,
// using a streaming pass that does not hold all matches in memory. Replacing
// fewer than all matches with a replacement that references groups holds the
// group indices of count matches, so count is bounded by the byte limit.
func replace(ctx context.Context, r *regexp.Regexp, str, repl string, count int) (object.Object, error) {
	if count < 0 {
		return object.NewString(str), nil
	}
	maxBytes := object.GetLimits(ctx).MaxBytesSize
	lit, refs := replacementShape(r, repl)
	var matches int
	var canceled, tooLarge bool
	size := int64(len(str))
	r.ReplaceAllStringFunc(str, func(m string) string {
		if canceled {
			return ""
		}
		if matches%1024 == 0 && ctx.Err() != nil {
			canceled = true
			return ""
		}
		if !tooLarge && (count == 0 || matches < count) {
			// Each group reference expands to at most the match length.
			size += int64(lit) + int64(refs)*int64(len(m)) - int64(len(m))
			tooLarge = size > maxBytes
		}
		matches++
		return ""
	})
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if tooLarge {
		return nil, object.ValueErrorf("regexp.replace: result too large (exceeds limit of %d bytes)",
			maxBytes)
	}
	if count == 0 || count >= matches {
		return object.NewString(r.ReplaceAllString(str, repl)), nil
	}
	// Replace only the first count matches.
	indices := 2 * (r.NumSubexp() + 1)
	if refs == 0 {
		// The expansion does not depend on the match, so replace in one
		// streaming pass that holds no match indices.
		expanded := string(r.ExpandString(nil, repl, "", make([]int, indices)))
		replaced := 0
		return object.NewString(r.ReplaceAllStringFunc(str, func(m string) string {
			if replaced == count {
				return m
			}
			replaced++
			return expanded
		})), nil
	}
	// Expanding group references needs each match's group indices. Go's
	// regexp package can only report them for all matches at once, so the
	// indices of count matches are held in memory. Bound them by the byte
	// limit: each match costs a slice header plus 8 bytes per index.
	perMatch := int64(24 + 8*indices)
	if maxCount := maxBytes / perMatch; int64(count) > maxCount {
		return nil, object.ValueErrorf("regexp.replace: count too large (%d exceeds limit of %d for this pattern)",
			count, maxCount)
	}
	var result []byte
	last := 0
	for _, m := range r.FindAllStringSubmatchIndex(str, count) {
		result = append(result, str[last:m[0]]...)
		result = r.ExpandString(result, repl, str, m)
		last = m[1]
	}
	result = append(result, str[last:]...)
	return object.NewString(string(result)), nil
}

// replacementShape returns the number of literal bytes repl expands to and
// the number of group references it contains, by expanding it once with
// empty groups and once with single-byte groups.
func replacementShape(r *regexp.Regexp, repl string) (lit, refs int) {
	n := 2 * (r.NumSubexp() + 1)
	empty := make([]int, n)
	one := make([]int, n)
	for i := 1; i < n; i += 2 {
		one[i] = 1
	}
	lit = len(r.ExpandString(nil, repl, "", empty))
	refs = len(r.ExpandString(nil, repl, "x", one)) - lit
	return lit, refs
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

	parts := r.Split(str, object.LimitCount(ctx, n))
	if err := object.CheckListLength(ctx, len(parts)); err != nil {
		return nil, err
	}
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

	matches := r.FindAllString(str, object.LimitCount(ctx, n))
	if err := object.CheckListLength(ctx, len(matches)); err != nil {
		return nil, err
	}
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
