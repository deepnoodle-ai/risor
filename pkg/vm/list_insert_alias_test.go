package vm

import (
	"context"
	"testing"

	"github.com/deepnoodle-ai/wonton/assert"
)

// list.insert grows a list in place. These tests check that no other list
// visible to a script shares the backing array, so an insert into one list
// never changes another.

// A list literal with a spread is built with ListExtend and ListAppend. The
// result owns its backing array, apart from the spread source and from other
// lists built from the same source.
func TestListInsertAfterSpreadLiteral(t *testing.T) {
	result, err := run(context.Background(), `
	let base = [1, 2, 3]
	let a = [...base, 4, 5]
	let b = [...base, 6]
	a.insert(0, 0)
	a.insert(2, 9)
	b.insert(0, -1)
	base.insert(0, 7)
	[base, a, b]
	`)
	assert.Nil(t, err)
	assert.Equal(t, result.Inspect(), `[[7, 1, 2, 3], [0, 1, 9, 2, 3, 4, 5], [-1, 1, 2, 3, 6]]`)
}

// A rest parameter collects arguments from a spread call. Inserting into it
// does not change the list that was spread.
func TestListInsertIntoRestParam(t *testing.T) {
	result, err := run(context.Background(), `
	function f(first, ...rest) {
		rest.insert(0, first)
		rest.insert(1, 9)
		return rest
	}
	let xs = [1, 2, 3, 4]
	let ys = f(...xs)
	let zs = f(...xs, 5)
	[xs, ys, zs]
	`)
	assert.Nil(t, err)
	assert.Equal(t, result.Inspect(), `[[1, 2, 3, 4], [1, 9, 2, 3, 4], [1, 9, 2, 3, 4, 5]]`)
}

// Repeated inserts at the front, then at a negative index and past the end.
func TestListInsertFrontLoop(t *testing.T) {
	result, err := run(context.Background(), `
	let l = []
	range(5).each(i => l.insert(0, i))
	l.insert(-1, "x")
	l.insert(100, "end")
	l
	`)
	assert.Nil(t, err)
	assert.Equal(t, result.Inspect(), `[4, 3, 2, 1, "x", 0, "end"]`)
}
