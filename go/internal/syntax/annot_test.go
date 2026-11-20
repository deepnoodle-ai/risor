package syntax

import (
	"testing"

	"github.com/deepnoodle-ai/wonton/assert"
)

// Package F16, slice D1: type lines and annotations (f16-entry-points.md 2).
func TestAnnotations(t *testing.T) {
	for _, c := range [][2]string{
		{`type Order = {id: string}; const o: Order = {id: "a"}; o`, `(program (type Order {"id": string}) (const o : Order (map (entry "id" "a"))) o)`},
		{`const f = (x: int, y: string = "a"): int => x; f(1)`, `(program (const f (lambda ((: x int) (: y string = "a")) : int x)) (call f 1))`},
		{`const g = (f: (x: int) => int, n) => n; 1`, `(program (const g (lambda ((: f (x: int) => int) n) n)) 1)`},
		{`const g = (f: () => unknown) => 1; 1`, `(program (const g (lambda ((: f () => unknown)) 1)) 1)`},
		{`[1].map((x): int => x)`, `(program (.map (list 1) (lambda (x) : int x)))`},
		{`xs.map((x: int, i: int) => x + i)`, `(program (.map xs (lambda ((: x int) (: i int)) (+ x i))))`},
		{`const n: int = 1; type Big = {n: int}; n`, `(program (type Big {"n": int}) (const n : int 1) n)`},
		{`type(x)`, `(program (call type x))`},
		{`const type = 1; type`, `(program (const type 1) type)`},
		{`const f = (x): int[] => [x]; 1`, `(program (const f (lambda (x) : int[] (list x))) 1)`},
	} {
		assert.Equal(t, Dump(parse(t, c[0])), c[1], "tree of %q", c[0])
	}
	checkErrors(t, []errCase{
		{`(x: int) => x`, "unexpected-token", [4]int{1, 3, 1, 4}},
		{`f((x: int))`, "unexpected-token", [4]int{1, 5, 1, 6}},
		{`const f: int = x => x; 1`, "unexpected-token", [4]int{1, 18, 1, 20}},
		{`const f: int = (x: int) => x; 1`, "unexpected-token", [4]int{1, 18, 1, 19}},
		{`type T = int; 1`, "bad-type-text", [4]int{1, 6, 1, 7}},
		{`type foo = int; 1`, "unexpected-token", [4]int{1, 6, 1, 9}},
		{`{ type Foo = int; 1 }`, "unexpected-token", [4]int{1, 8, 1, 11}},
		{`const g = (f: (x: int) => int = h) => 1; 1`, "unexpected-token", [4]int{1, 31, 1, 32}},
		{`const g = ({a}: (x: int) => int) => 1; 1`, "bad-type-text", [4]int{1, 18, 1, 19}},
		{`const g = (): (x: int) => int => 1; 1`, "bad-type-text", [4]int{1, 16, 1, 17}},
		{`const g: (x: int) => int = 1; 1`, "bad-type-text", [4]int{1, 11, 1, 12}},
		{`f((a): b)`, "bad-type-text", [4]int{1, 8, 1, 9}},
		{`type Foo = int`, "unexpected-token", [4]int{1, 15, 1, 15}},
		{`type Foo = int = 2; 1`, "unexpected-token", [4]int{1, 16, 1, 17}},
	})
}
