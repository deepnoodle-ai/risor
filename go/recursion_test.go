package risor_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	risor "github.com/deepnoodle-ai/risor/go/v3"
	"github.com/deepnoodle-ai/risor/go/v3/internal/assert"
)

// Package F17, step E0 (spec/packages/f17-recursion.md): what the suite does
// not reach. The suite cannot hold a Go struct or a value that holds itself.

type treeNode struct {
	Name string     `json:"name"`
	Kids []treeNode `json:"kids"`
}

func TestWalkGoStructs(t *testing.T) {
	// Nodes read in place from Go structs (5.2 item 5).
	env, err := risor.NewEnv(risor.VarRecord("{t: any}"))
	assert.NoError(t, err)
	tree := treeNode{"a", []treeNode{{"b", nil}, {"c", []treeNode{{"d", nil}}}}}
	assert.Equal(t, runIn(t, env, "walk(t, x => x.kids)[*].name", risor.Map{"t": tree}),
		[]any{"a", "b", "c", "d"})
}

func TestWalkValueThatHoldsItself(t *testing.T) {
	// A host list that is its own element stops at the depth limit (5.4
	// item 4), not at a native stack overflow.
	env, err := risor.NewEnv(risor.VarRecord("{t: any}"))
	assert.NoError(t, err)
	l := []any{nil}
	l[0] = l
	d := runErr(t, env, "walk(t, x => x)", risor.Map{"t": l})
	assert.Equal(t, d.Code, "depth-exceeded")
}

func TestWalkAtTheLargestDepth(t *testing.T) {
	// A walk keeps its own stack (5.3): 400 levels of host lists at the
	// largest depth limit, and one more fails.
	env, err := risor.NewEnv(risor.VarRecord("{t: any}"),
		risor.Limits{Depth: 400, Steps: risor.NoLimit, Allocation: risor.NoLimit})
	assert.NoError(t, err)
	var v any = []any{}
	for i := 1; i < 400; i++ {
		v = []any{v}
	}
	out := run(t, env, "walk(t, x => x)", risor.Map{"t": v})
	assert.Equal(t, len(out.([]any)), 400)
	d := runErr(t, env, "walk(t, x => x)", risor.Map{"t": []any{v}})
	assert.Equal(t, d.Code, "depth-exceeded")
}

func TestGuardRuns(t *testing.T) {
	// A guard is tested after its arm's literal, and a falsy one passes to
	// the next arm (3).
	env, err := risor.NewEnv(risor.VarRecord("{x: int, y: int}"))
	assert.NoError(t, err)
	src := `match x { 1 if (y > 0) => "a", 1 => "b", _ => "c" }`
	assert.Equal(t, runIn(t, env, src, risor.Map{"x": 1, "y": 1}), "a")
	assert.Equal(t, runIn(t, env, src, risor.Map{"x": 1, "y": 0}), "b")
	assert.Equal(t, runIn(t, env, src, risor.Map{"x": 2, "y": 1}), "c")
}

// Step E2 (12.4): a Go struct that contains itself, through TypeOf.
type Node struct {
	Name     string `json:"name"`
	Size     int    `json:"size"`
	Children []Node `json:"children"`
}

func TestTypeOfRecursiveStruct(t *testing.T) {
	ty, err := risor.TypeOf[Node]()
	assert.NoError(t, err)
	assert.Equal(t, ty.String(), "Node")
	env, err := risor.NewEnv(risor.VarType("tree", ty))
	assert.NoError(t, err)
	src := "const total = (n: Node): int => n.size + n.children.map(total).sum(); total(tree)"
	// The name Node in the program is no type of the environment.
	assert.Equal(t, checkErr(t, env, src)[0].Code, "undefined-name")
	env, err = risor.NewEnv(risor.NamedTypeOf("Node", ty), risor.VarType("tree", ty))
	assert.NoError(t, err)
	tree := Node{"a", 1, []Node{{"b", 2, nil}, {"c", 3, []Node{{"d", 4, nil}}}}}
	assert.Equal(t, runIn(t, env, src, risor.Map{"tree": tree}), float64(10))
	// A wrong field inside the tree names the type.
	ds := checkErr(t, env, "const f = (n: Node): int => len(n.childs); f(tree)")
	assert.Equal(t, ds[0].Message, `Node has no field "childs"`)
}

func TestConformDepth(t *testing.T) {
	// 12.7: the conformance function stops at the default depth limit.
	env, err := risor.NewEnv(risor.NamedType("Tree", "{kids: Tree[]}"))
	assert.NoError(t, err)
	ty, err := env.ParseType("Tree")
	assert.NoError(t, err)
	var v any = map[string]any{"kids": []any{}}
	for i := 1; i < 100; i++ {
		v = map[string]any{"kids": []any{v}}
	}
	// 100 maps and 100 lists: 200 levels.
	assert.NoError(t, risor.Conform(v, ty))
	assert.Equal(t, len(risor.ConformAll(v, ty, 10)), 0)
	v = map[string]any{"kids": []any{v}}
	err = risor.Conform(v, ty)
	var e *risor.Error
	assert.True(t, errors.As(err, &e))
	assert.Equal(t, e.Diagnostics[0].Code, "depth-exceeded")
	assert.Equal(t, e.Diagnostics[0].Message, "calls and nested values went deeper than the depth limit of 200")
	ds := risor.ConformAll(v, ty, 10)
	assert.Equal(t, len(ds), 1)
	assert.Equal(t, ds[0].Code, "depth-exceeded")
}

func TestRecursiveNameClash(t *testing.T) {
	// 12.4 item 2 (R315): a recursive type may bear the name of a named
	// type only when the named type is that recursive type.
	tree := map[string]any{"type": "array", "items": map[string]any{"$ref": "#"}}
	clash := func(decls ...risor.Decl) {
		t.Helper()
		_, err := risor.NewEnv(decls...)
		var e *risor.Error
		assert.True(t, errors.As(err, &e))
		assert.Equal(t, e.Diagnostics[0].Code, "name-clash")
	}
	// The name of an opaque type, in one environment and through a parent.
	clash(risor.OpaqueType{Name: "Tree"}, risor.VarSchema("tree", tree))
	parent, err := risor.NewEnv(risor.OpaqueType{Name: "Tree"})
	assert.NoError(t, err)
	_, err = parent.With(risor.VarSchema("tree", tree))
	var e *risor.Error
	assert.True(t, errors.As(err, &e))
	assert.Equal(t, e.Diagnostics[0].Code, "name-clash")
	// A named type that is an alias of another recursive type of the same
	// structure.
	clash(risor.NamedType("Other", "Other[]"), risor.NamedType("Tree", "Other"), risor.VarSchema("tree", tree))
	// The named type that is the recursive type, and its description, which
	// builds the same environment again.
	env, err := risor.NewEnv(risor.NamedType("Tree", "Tree[]"), risor.VarSchema("tree", tree))
	assert.NoError(t, err)
	d, err := env.Describe()
	assert.NoError(t, err)
	back, err := risor.FromDescription(d)
	assert.NoError(t, err)
	_, err = risor.NewEnv(back)
	assert.NoError(t, err)
	// A recursive type with no named type is listed in types too (F4 8
	// item 4), and the description builds an environment.
	env, err = risor.NewEnv(risor.VarSchema("tree", tree))
	assert.NoError(t, err)
	d, err = env.Describe()
	assert.NoError(t, err)
	assert.True(t, strings.Contains(string(d), `"types":{"Tree":"Tree[]"}`))
	back, err = risor.FromDescription(d)
	assert.NoError(t, err)
	_, err = risor.NewEnv(back)
	assert.NoError(t, err)
}

func TestFuncWithRecursiveGoType(t *testing.T) {
	// 12.4, Go's TypeOf item 4: Func derives no signature that holds a
	// recursive type, and the host writes one in Type, naming a type that
	// NamedTypeOf declares.
	ty, err := risor.TypeOf[Node]()
	assert.NoError(t, err)
	size := func(n Node, k int) int { return n.Size*k + len(n.Children) }
	_, err = risor.NewEnv(risor.Func("size", size))
	var e *risor.Error
	assert.True(t, errors.As(err, &e))
	assert.Equal(t, len(e.Diagnostics), 1) // the int after Node is no recursion
	assert.Equal(t, e.Diagnostics[0].Code, "bad-type")
	assert.True(t, strings.Contains(e.Diagnostics[0].Message, "parameter 1"))
	f := risor.Func("size", size)
	f.Type = "(n: Node, k: int) => int"
	env, err := risor.NewEnv(risor.NamedTypeOf("Node", ty), f)
	assert.NoError(t, err)
	assert.Equal(t, run(t, env, `size({name: "a", size: 2, children: [{name: "b", size: 1, children: []}]}, 3)`, nil), float64(7))
	// The arity of the signature the host writes is still tested.
	f.Type = "(n: Node) => int"
	_, err = risor.NewEnv(risor.NamedTypeOf("Node", ty), f)
	assert.True(t, errors.As(err, &e))
	assert.Equal(t, e.Diagnostics[0].Code, "bad-signature")
}

func TestConformDepthAfterAMismatch(t *testing.T) {
	// 12.7 item 3: past the depth limit the conformance function fails
	// with depth-exceeded, even after a key of a rest that does not
	// conform.
	env, err := risor.NewEnv(risor.NamedType("Node", "{next?: Node}"))
	assert.NoError(t, err)
	ty, err := env.ParseType("map<Node>")
	assert.NoError(t, err)
	var deep any = map[string]any{}
	for i := 0; i < 250; i++ {
		deep = map[string]any{"next": deep}
	}
	err = risor.Conform(map[string]any{"a": 1, "b": deep}, ty)
	var e *risor.Error
	assert.True(t, errors.As(err, &e))
	assert.Equal(t, e.Diagnostics[0].Code, "depth-exceeded")
	// ConformAll agrees: its first failure is Conform's, and the depth
	// ends it (F8 6.1; 12.7 item 3).
	ds := risor.ConformAll(map[string]any{"a": 1, "b": deep}, ty, 10)
	assert.Equal(t, len(ds), 1)
	assert.Equal(t, ds[0].Code, "depth-exceeded")
	// With no deep value the first mismatch stands.
	err = risor.Conform(map[string]any{"a": 1, "b": map[string]any{}}, ty)
	assert.True(t, errors.As(err, &e))
	assert.Equal(t, e.Diagnostics[0].Code, "not-conforming")
	ds = risor.ConformAll(map[string]any{"a": 1, "b": map[string]any{}}, ty, 10)
	assert.Equal(t, len(ds), 1)
	assert.Equal(t, ds[0].Code, "not-conforming")
	// A deep value that Conform never reaches, past a failing declared
	// field, ends ConformAll after the first failure, which is Conform's.
	rec, err := env.ParseType("{a: int, b: Node}")
	assert.NoError(t, err)
	err = risor.Conform(map[string]any{"a": "x", "b": deep}, rec)
	assert.True(t, errors.As(err, &e))
	assert.Equal(t, e.Diagnostics[0].Code, "not-conforming")
	ds = risor.ConformAll(map[string]any{"a": "x", "b": deep}, rec, 10)
	assert.Equal(t, len(ds), 2)
	assert.Equal(t, ds[0].Code, "not-conforming")
	assert.Equal(t, ds[1].Code, "depth-exceeded")
}

func TestUnionTypeKeepsReference(t *testing.T) {
	// UnionType is | of type text: a reference that shares no kind but
	// null with another alternative stays by name (12.2 item 5).
	env, err := risor.NewEnv(risor.NamedType("Link", "{next?: Link}"))
	assert.NoError(t, err)
	link, err := env.ParseType("Link")
	assert.NoError(t, err)
	assert.Equal(t, risor.UnionType(link, risor.TypeBool).String(), "bool | Link")
	text, err := env.ParseType("Link | bool")
	assert.NoError(t, err)
	assert.Equal(t, risor.UnionType(link, risor.TypeBool).String(), text.String())
	// One that shares a kind is unfolded, as in type text.
	env, err = risor.NewEnv(risor.NamedType("Tree", "Tree[]"))
	assert.NoError(t, err)
	tree, err := env.ParseType("Tree")
	assert.NoError(t, err)
	text, err = env.ParseType("Tree | bool[]")
	assert.NoError(t, err)
	assert.Equal(t, risor.UnionType(tree, risor.ListType(risor.TypeBool)).String(), text.String())
}

// bigLines are type lines Rr0 to Rr15, each a record of two fields of the
// one before: Rr15 has size 65,535 (05-types.md 20.7).
func bigLines(top int) string {
	var b strings.Builder
	b.WriteString("type Rr0 = int; ")
	for i := 1; i <= top; i++ {
		fmt.Fprintf(&b, "type Rr%d = {a: Rr%d, b: Rr%d}; ", i, i-1, i-1)
	}
	return b.String()
}

func TestUnfoldedDefinitionSize(t *testing.T) {
	// 12.2 item 4, 12.6: a reference that reaches the top of a recursive
	// item through a line that is not recursive is replaced by its
	// definition, and the definition so made is tested for size.
	lines := func(top int) string {
		return bigLines(top) + fmt.Sprintf("type Tree = {kids: Tree[], payload: Rr%d}; type Alias = Tree; type Xs = {again: Xs, payload: Rr%d}[] | Alias; ", top, top)
	}
	env, err := risor.NewEnv()
	assert.NoError(t, err)
	ds := checkErr(t, env, lines(15)+"1")
	assert.Equal(t, len(ds), 1)
	assert.Equal(t, ds[0].Code, "check-limit")
	x, err := risor.Parse(lines(14) + "1")
	assert.NoError(t, err)
	_, err = env.Check(x)
	assert.NoError(t, err)
	// The same in an environment's named types: bad-type.
	var decls []risor.Decl
	decls = append(decls, risor.NamedType("Rr0", "int"))
	for i := 1; i <= 15; i++ {
		decls = append(decls, risor.NamedType(fmt.Sprintf("Rr%d", i), fmt.Sprintf("{a: Rr%d, b: Rr%d}", i-1, i-1)))
	}
	decls = append(decls, risor.NamedType("Tree", "{kids: Tree[], payload: Rr15}"), risor.NamedType("Alias", "Tree"),
		risor.NamedType("Xs", "{again: Xs, payload: Rr15}[] | Alias"))
	_, err = risor.NewEnv(decls...)
	var e *risor.Error
	assert.True(t, errors.As(err, &e))
	assert.Equal(t, e.Diagnostics[0].Code, "bad-type")
}

// Sz0 to Sz15: each a struct of two fields of the one before, so Sz15's
// type has size 65,535 (05-types.md 20.7).
type (
	Sz0  int
	Sz1  struct{ A, B Sz0 }
	Sz2  struct{ A, B Sz1 }
	Sz3  struct{ A, B Sz2 }
	Sz4  struct{ A, B Sz3 }
	Sz5  struct{ A, B Sz4 }
	Sz6  struct{ A, B Sz5 }
	Sz7  struct{ A, B Sz6 }
	Sz8  struct{ A, B Sz7 }
	Sz9  struct{ A, B Sz8 }
	Sz10 struct{ A, B Sz9 }
	Sz11 struct{ A, B Sz10 }
	Sz12 struct{ A, B Sz11 }
	Sz13 struct{ A, B Sz12 }
	Sz14 struct{ A, B Sz13 }
	Sz15 struct{ A, B Sz14 }
)

// Link is a pointer to BigNode. Derived from Link, BigNode's record has
// size 100,000, and Link's definition, that record with null, 100,001.
// Link2 and BigNode2 have one int less.
type Link *BigNode

type BigNode struct {
	Link Link
	Self *BigNode
	P15  Sz15
	P14  Sz14
	P9   Sz9
	P8   Sz8
	P6   Sz6
	P4   Sz4
	X, Y int
}

type Link2 *BigNode2

type BigNode2 struct {
	Link Link2
	Self *BigNode2
	P15  Sz15
	P14  Sz14
	P9   Sz9
	P8   Sz8
	P6   Sz6
	P4   Sz4
	X    int
}

func TestTypeOfDefinitionSize(t *testing.T) {
	// 12.4 item 5: each definition TypeOf sets is tested for size, after
	// the references at its top are replaced.
	n, err := risor.TypeOf[Link2]()
	assert.NoError(t, err)
	assert.Equal(t, risor.DefSize(n), 100000)
	_, err = risor.TypeOf[Link]()
	var e *risor.Error
	assert.True(t, errors.As(err, &e), "a definition of size 100,001")
	assert.Equal(t, e.Diagnostics[0].Code, "bad-type")
}

// manyTypeLines is a program of 2n type lines: a flat chain of forward
// aliases, and one guarded ring, which is one component.
func manyTypeLines(n int) string {
	var b strings.Builder
	for i := 0; i < n-1; i++ {
		fmt.Fprintf(&b, "type Ta%d = Ta%d; ", i, i+1)
	}
	fmt.Fprintf(&b, "type Ta%d = int; ", n-1)
	for i := 0; i < n; i++ {
		fmt.Fprintf(&b, "type Tr%d = {next: Tr%d[]}; ", i, (i+n-1)%n)
	}
	b.WriteString("const x: Ta0 = 1; x")
	return b.String()
}

func TestManyTypeLines(t *testing.T) {
	// A long list of type lines resolves. BenchmarkManyTypeLines measures
	// how its time grows: near linear (it was quadratic).
	env, err := risor.NewEnv()
	assert.NoError(t, err)
	x, err := risor.Parse(manyTypeLines(10000)) // within the size of a source text
	assert.NoError(t, err)
	r, err := env.Check(x)
	assert.NoError(t, err)
	assert.Equal(t, r.Type.String(), "int")
}

func BenchmarkManyTypeLines(b *testing.B) {
	env, err := risor.NewEnv()
	if err != nil {
		b.Fatal(err)
	}
	for _, n := range []int{2500, 5000, 10000} {
		x, err := risor.Parse(manyTypeLines(n))
		if err != nil {
			b.Fatal(err)
		}
		b.Run(fmt.Sprint(n), func(b *testing.B) {
			for b.Loop() {
				if _, err := env.Check(x); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// pairFamilies are the named types A0 to A12 and B0 to B12: two families
// of one structure, each holding the next twice, which take more than
// 4,096 pairs to compare (12.5 item 7).
func pairFamilies() []risor.Decl {
	var ds []risor.Decl
	for _, p := range []string{"A", "B"} {
		for i := 0; i <= 12; i++ {
			next := fmt.Sprintf("%s%d", p, (i+1)%13)
			ds = append(ds, risor.NamedType(fmt.Sprintf("%s%d", p, i), fmt.Sprintf("{x: %s[], y: %s[]}", next, next)))
		}
	}
	return ds
}

func TestPairBoundInSignature(t *testing.T) {
	// R317: the pair bound passed in a host function's signature is
	// bad-signature, as every error of a signature is (03-host.md 3.3.4).
	f := risor.Function{Name: "pick", Type: "(x: A0[] | B0[]) => int",
		Impl: func(c *risor.Call, args []any) (any, error) { return 1, nil }}
	_, err := risor.NewEnv(append(pairFamilies(), f)...)
	var e *risor.Error
	assert.True(t, errors.As(err, &e))
	assert.Equal(t, e.Diagnostics[0].Code, "bad-signature")
	// The same Join in a variable's type is bad-type.
	_, err = risor.NewEnv(append(pairFamilies(), risor.Var("v", "A0[] | B0[]"))...)
	assert.True(t, errors.As(err, &e))
	assert.Equal(t, e.Diagnostics[0].Code, "bad-type")
}
