package object

import (
	"fmt"
	"sort"
)

// Sort a list in place. If the list contains a non-comparable object, an error
// is returned.
func Sort(items []Object) *Error {
	var comparableErr string
	sort.SliceStable(items, func(a, b int) bool {
		itemA := items[a]
		itemB := items[b]
		compA, ok := itemA.(Comparable)
		if !ok {
			comparableErr = fmt.Sprintf(
				"type error: sorted() encountered a non-comparable item (%s)", itemA.Type())
		}
		if _, ok := itemB.(Comparable); !ok {
			comparableErr = fmt.Sprintf(
				"type error: sorted() encountered a non-comparable item (%s)", itemB.Type())
		}
		result, err := compA.Compare(itemB)
		if err != nil {
			comparableErr = err.Error()
		}
		return result == -1
	})
	if comparableErr != "" {
		return TypeErrorf("%s", comparableErr)
	}
	return nil
}
