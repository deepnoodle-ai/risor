package object

import (
	"sort"
)

// Sort a list in place. If the list contains a non-comparable object, an error
// is returned.
func Sort(items []Object) *Error {
	var sortErr *Error
	sort.SliceStable(items, func(a, b int) bool {
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
