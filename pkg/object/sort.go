package object

import (
	"context"
	"sort"
)

// Sort a list in place. If the list contains a non-comparable object, an error
// is returned.
func Sort(items []Object) *Error {
	return SortContext(context.Background(), items)
}

// SortContext sorts a list in place, like Sort, and stops comparing items once
// an error occurs or ctx is canceled. In either case the order of items is
// unspecified and the error is returned.
func SortContext(ctx context.Context, items []Object) *Error {
	var sortErr *Error
	var comparisons int
	sort.SliceStable(items, func(a, b int) bool {
		if sortErr != nil {
			return false
		}
		comparisons++
		if comparisons%ctxCheckInterval == 0 {
			if err := ctx.Err(); err != nil {
				sortErr = NewError(err)
				return false
			}
		}
		itemA := items[a]
		itemB := items[b]
		compA, ok := itemA.(Comparable)
		if !ok {
			sortErr = TypeErrorf("sorted() encountered a non-comparable item (%s)", itemA.Type())
			return false
		}
		if _, ok := itemB.(Comparable); !ok {
			sortErr = TypeErrorf("sorted() encountered a non-comparable item (%s)", itemB.Type())
			return false
		}
		result, err := compA.Compare(itemB)
		if err != nil {
			sortErr = NewError(err)
			return false
		}
		return result == -1
	})
	return sortErr
}
