package types

// This file resolves a list of named types together (F17 12.2): an
// environment's named types, or a program's type lines. A name may stand
// for itself and for any name of its list. The names on a cycle are
// recursive; each gets a reference.

// Use is one name in the text of a named type: the item of the list it
// names, or -1 for a type outside the list, and whether it stands at a
// guarded place: inside a list element, a record field, a record's rest or
// a map's value.
type Use struct {
	Target  int
	Guarded bool
}

// ListInfo is what the uses of a list say about it.
type ListInfo struct {
	// Recursive marks each item on a cycle of uses.
	Recursive []bool
	// Direct marks each item on a cycle of unguarded uses: an error.
	Direct []bool
	// Comp is the component of each item: the items that name one another.
	Comp []int
	// Order lists the components, each after every component it names.
	Order [][]int
}

// AnalyzeList finds the recursive items, the cycles with no guard and the
// order in which the components are resolved.
func AnalyzeList(uses [][]Use) *ListInfo {
	n := len(uses)
	info := &ListInfo{Recursive: make([]bool, n), Direct: make([]bool, n), Comp: make([]int, n)}
	all := func(i int) []int {
		var out []int
		for _, u := range uses[i] {
			if u.Target >= 0 {
				out = append(out, u.Target)
			}
		}
		return out
	}
	unguarded := func(i int) []int {
		var out []int
		for _, u := range uses[i] {
			if u.Target >= 0 && !u.Guarded {
				out = append(out, u.Target)
			}
		}
		return out
	}
	comps := tarjan(n, all)
	for ci, c := range comps {
		for _, i := range c {
			info.Comp[i] = ci
		}
		cyclic := len(c) > 1
		if !cyclic {
			for _, j := range all(c[0]) {
				if j == c[0] {
					cyclic = true
				}
			}
		}
		for _, i := range c {
			info.Recursive[i] = cyclic
		}
	}
	info.Order = comps
	for _, c := range tarjan(n, unguarded) {
		cyclic := len(c) > 1
		if !cyclic {
			for _, j := range unguarded(c[0]) {
				if j == c[0] {
					cyclic = true
				}
			}
		}
		if cyclic {
			for _, i := range c {
				info.Direct[i] = true
			}
		}
	}
	return info
}

// tarjan gives the strongly connected components of a graph, each after
// every component it reaches. The items of a component are in list order.
func tarjan(n int, next func(int) []int) [][]int {
	index := make([]int, n)
	low := make([]int, n)
	on := make([]bool, n)
	for i := range index {
		index[i] = -1
	}
	var stack []int
	var out [][]int
	k := 0
	var visit func(v int)
	visit = func(v int) {
		index[v], low[v] = k, k
		k++
		stack = append(stack, v)
		on[v] = true
		for _, w := range next(v) {
			if index[w] < 0 {
				visit(w)
				low[v] = min(low[v], low[w])
			} else if on[w] {
				low[v] = min(low[v], index[w])
			}
		}
		if low[v] == index[v] {
			var c []int
			for {
				w := stack[len(stack)-1]
				stack = stack[:len(stack)-1]
				on[w] = false
				c = append(c, w)
				if w == v {
					break
				}
			}
			// list order inside a component
			for i := 1; i < len(c); i++ {
				for j := i; j > 0 && c[j] < c[j-1]; j-- {
					c[j], c[j-1] = c[j-1], c[j]
				}
			}
			out = append(out, c)
		}
	}
	for i := 0; i < n; i++ {
		if index[i] < 0 {
			visit(i)
		}
	}
	return out
}

// ListBuild builds the types of a list, one component at a time.
type ListBuild struct {
	Info  *ListInfo
	Names []string
	// Named holds the reference of each recursive item.
	Named []*Named
	// Built holds each item's type once built: the definition of a
	// recursive item, the type of any other.
	Built  []*Type
	Failed []bool
	state  []uint8
	// BuildFn builds item i's type, naming items through Look. It reports
	// false when the text has a diagnostic.
	BuildFn func(i int) (*Type, bool)
}

// NewListBuild prepares the build of a list.
func NewListBuild(info *ListInfo, names []string) *ListBuild {
	n := len(names)
	b := &ListBuild{Info: info, Names: names, Named: make([]*Named, n), Built: make([]*Type, n),
		Failed: make([]bool, n), state: make([]uint8, n)}
	for i := range names {
		if info.Recursive[i] {
			b.Named[i] = NewNamed(names[i])
		}
	}
	return b
}

// Look gives what item j stands for where item i names it (12.2): a
// recursive item at a guarded place, or at the top of an item that is not
// recursive, is its reference; at the top of a recursive item, its
// definition, built first; any other item is its type.
func (b *ListBuild) Look(i, j int, guarded bool) *Type {
	if b.Info.Recursive[j] && (guarded || !b.Info.Recursive[i]) {
		return Ref(b.Named[j])
	}
	if b.Info.Comp[j] == b.Info.Comp[i] {
		b.build(j)
	}
	if b.Built[j] == nil {
		return Never
	}
	return b.Built[j]
}

func (b *ListBuild) build(i int) {
	if b.state[i] != 0 {
		return
	}
	b.state[i] = 1
	t, ok := b.BuildFn(i)
	b.state[i] = 2
	if !ok {
		b.Failed[i] = true
		return
	}
	if b.Info.Recursive[i] {
		// A definition holds no reference at its top (12.2 item 4).
		t = Unfold(t)
	}
	b.Built[i] = t
}

// Comp builds the items of a component, in list order, and sets the
// definitions of its recursive items. It reports whether every item
// built.
func (b *ListBuild) Comp(c []int) bool {
	for _, i := range c {
		b.build(i)
	}
	ok := true
	for _, i := range c {
		if b.Failed[i] {
			ok = false
		}
	}
	if !ok {
		for _, i := range c {
			b.Failed[i] = true
			b.Built[i] = nil
		}
		return false
	}
	for _, i := range c {
		if b.Named[i] != nil {
			b.Named[i].SetDef(b.Built[i])
		}
	}
	return true
}

// TypeOf is what the list declares item i to be: its reference when
// recursive, its type otherwise; nil when it failed.
func (b *ListBuild) TypeOf(i int) *Type {
	if b.Failed[i] || b.Built[i] == nil {
		return nil
	}
	if b.Named[i] != nil {
		return Ref(b.Named[i])
	}
	return b.Built[i]
}

// Shared is a reference that shares a kind other than null with another
// alternative of the type where it stands: the error of 12.2 item 6. It
// gives the first such reference of t, by the order printed.
func Shared(t *Type) (*Named, bool) {
	for _, n := range t.refs {
		rest := t.plain()
		var others []*Named
		for _, m := range t.refs {
			if m != n {
				others = append(others, m)
			}
		}
		rest = withRefs(rest, others)
		if SharesKind(Ref(n), rest) {
			return n, true
		}
	}
	return nil, false
}

// SharedIn is the first reference in item i's definition that shares a
// kind other than null with another alternative of the type where it
// stands (12.2 item 6). The walk takes a type before its parts, and the
// parts in the order printed: the list's element, then each record by
// the order of its variants, its fields in key order and then its rest.
// A reference is not followed.
func (b *ListBuild) SharedIn(i int) (*Named, bool) { return SharedDeep(b.Built[i]) }

// SharedDeep is SharedIn of a type.
func SharedDeep(t *Type) (*Named, bool) {
	var walk func(t *Type) (*Named, bool)
	walk = func(t *Type) (*Named, bool) {
		if t == nil || t.any {
			return nil, false
		}
		if n, bad := Shared(t); bad {
			return n, true
		}
		if t.list != nil {
			if n, bad := walk(t.list); bad {
				return n, true
			}
		}
		for _, r := range ownVariants(t) {
			for _, f := range r.Fields {
				if n, bad := walk(f.Type); bad {
					return n, true
				}
			}
			if n, bad := walk(r.Rest); bad {
				return n, true
			}
		}
		return nil, false
	}
	return walk(t)
}

// Shown is the type a message prints for a receiver whose map alternative
// is read: t without null when t is a recursive name and null alone, so
// that the message names it (12.6); else nil.
func Shown(t *Type) *Type {
	if len(t.refs) != 1 || !t.plain().withoutNullPlain().IsNever() {
		return nil
	}
	return Ref(t.refs[0])
}

// SetDefs sets the definitions of the references ns that one mapping made,
// a schema's or a Go type's (12.4): defs[i] is ns[i]'s type, whose top may
// hold references of ns, each replaced by its own definition. It reports
// the first reference that shares a kind other than null with another
// alternative where it stands, at the top or below.
func SetDefs(ns []*Named, defs []*Type) (*Named, bool) {
	idx := make(map[*Named]int, len(ns))
	for i, n := range ns {
		idx[n] = i
	}
	out := make([]*Type, len(ns))
	state := make([]uint8, len(ns))
	var bad *Named
	var get func(i int) *Type
	get = func(i int) *Type {
		if state[i] == 2 || state[i] == 1 {
			return out[i] // a cycle of tops cannot be: the mapping refused it
		}
		state[i] = 1
		t := defs[i]
		p := t.plain()
		var kept []*Named
		for _, r := range t.refs {
			j, ok := idx[r]
			if !ok {
				kept = append(kept, r)
				continue
			}
			d := get(j)
			if d == nil {
				continue
			}
			if SharesKind(d, p) && bad == nil {
				bad = r
			}
			p = mergeKinds(p, d)
		}
		out[i] = withRefs(p, kept)
		state[i] = 2
		return out[i]
	}
	for i := range ns {
		get(i)
	}
	if bad != nil {
		return bad, true
	}
	for i, n := range ns {
		n.SetDef(out[i])
	}
	for _, n := range ns {
		if m, ok := SharedDeep(n.def); ok {
			return m, true
		}
	}
	return nil, false
}

// RecNames are the recursive names that ts reach, at any depth and through
// the definitions, each once, in the order met.
func RecNames(ts ...*Type) []*Named {
	var out []*Named
	seen := visited{}
	done := map[*Type]bool{}
	var walk func(t *Type)
	walk = func(t *Type) {
		if t == nil || t.any || done[t] {
			return
		}
		done[t] = true
		for _, n := range t.refs {
			if !seen[n] {
				seen[n] = true
				out = append(out, n)
				if n.def != nil {
					walk(n.def)
				}
			}
		}
		if t.list != nil {
			walk(t.list)
		}
		for _, r := range ownVariants(t) {
			for _, f := range r.Fields {
				walk(f.Type)
			}
			walk(r.Rest)
		}
	}
	for _, t := range ts {
		walk(t)
	}
	return out
}

// SameDef reports whether two recursive names stand for one type: the
// same structure, coinductively (12.4).
func SameDef(a, b *Named) bool {
	if a == b {
		return true
	}
	ok := false
	if Guard(func() { ok = Equal(Ref(a), Ref(b)) }) {
		return false
	}
	return ok
}
