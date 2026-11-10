package ast

// Clone makes a deep copy of a program.
func Clone(p *Program) *Program {
	return &Program{Consts: cloneConsts(p.Consts), Result: CloneExpr(p.Result), Semi: p.Semi}
}

func cloneConsts(cs []*Const) []*Const {
	if cs == nil {
		return nil
	}
	out := make([]*Const, len(cs))
	for i, c := range cs {
		out[i] = &Const{Pat: clonePat(c.Pat), Value: CloneExpr(c.Value)}
	}
	return out
}

func cloneBranch(b *Branch) *Branch {
	if b == nil {
		return nil
	}
	return &Branch{Consts: cloneConsts(b.Consts), Result: CloneExpr(b.Result)}
}

func cloneExprs(es []Expr) []Expr {
	if es == nil {
		return nil
	}
	out := make([]Expr, len(es))
	for i, e := range es {
		out[i] = CloneExpr(e)
	}
	return out
}

func clonePat(p Pattern) Pattern {
	switch p := p.(type) {
	case *NamePat:
		return &NamePat{Name: p.Name}
	case *MapPat:
		q := &MapPat{Rest: p.Rest, Fields: make([]Field, len(p.Fields))}
		for i, f := range p.Fields {
			f.Default = CloneExpr(f.Default)
			q.Fields[i] = f
		}
		return q
	case *ListPat:
		q := &ListPat{Rest: p.Rest, Elems: make([]Elem, len(p.Elems))}
		for i, el := range p.Elems {
			el.Default = CloneExpr(el.Default)
			q.Elems[i] = el
		}
		return q
	}
	return p
}

// CloneExpr makes a deep copy of an expression. It returns nil for nil.
func CloneExpr(e Expr) Expr {
	switch e := e.(type) {
	case nil:
		return nil
	case *Num:
		c := *e
		return &c
	case *Str:
		c := *e
		return &c
	case *Template:
		c := &Template{Parts: make([]Part, len(e.Parts))}
		for i, p := range e.Parts {
			c.Parts[i] = Part{Text: p.Text, X: CloneExpr(p.X)}
		}
		return c
	case *Bool:
		c := *e
		return &c
	case *Null:
		return &Null{}
	case *Ident:
		c := *e
		return &c
	case *List:
		c := &List{Items: make([]Item, len(e.Items))}
		for i, it := range e.Items {
			c.Items[i] = Item{Spread: it.Spread, X: CloneExpr(it.X)}
		}
		return c
	case *Map:
		c := &Map{Entries: make([]Entry, len(e.Entries))}
		for i, en := range e.Entries {
			en.KeyX = CloneExpr(en.KeyX)
			en.X = CloneExpr(en.X)
			c.Entries[i] = en
		}
		return c
	case *Unary:
		return &Unary{Op: e.Op, X: CloneExpr(e.X)}
	case *Binary:
		return &Binary{Op: e.Op, X: CloneExpr(e.X), Y: CloneExpr(e.Y)}
	case *Cond:
		return &Cond{Test: CloneExpr(e.Test), Then: CloneExpr(e.Then), Else: CloneExpr(e.Else)}
	case *If:
		c := &If{Tests: cloneExprs(e.Tests), Else: cloneBranch(e.Else)}
		for _, b := range e.Then {
			c.Then = append(c.Then, cloneBranch(b))
		}
		return c
	case *Block:
		return &Block{Consts: cloneConsts(e.Consts), Result: CloneExpr(e.Result)}
	case *Try:
		return &Try{Body: cloneBranch(e.Body), Name: e.Name, Handler: cloneBranch(e.Handler)}
	case *Match:
		c := &Match{Subject: CloneExpr(e.Subject), Arms: make([]Arm, len(e.Arms))}
		for i, a := range e.Arms {
			c.Arms[i] = Arm{Lits: cloneExprs(a.Lits), Default: a.Default, Result: CloneExpr(a.Result)}
		}
		return c
	case *Call:
		return &Call{NS: e.NS, Name: e.Name, Args: cloneExprs(e.Args)}
	case *Member:
		return &Member{X: CloneExpr(e.X), Name: e.Name, Opt: e.Opt}
	case *Index:
		return &Index{X: CloneExpr(e.X), I: CloneExpr(e.I), Opt: e.Opt}
	case *MethodCall:
		return &MethodCall{X: CloneExpr(e.X), Name: e.Name, Args: cloneExprs(e.Args), Opt: e.Opt}
	case *Lambda:
		c := &Lambda{Bare: e.Bare, Body: CloneExpr(e.Body)}
		for _, p := range e.Params {
			c.Params = append(c.Params, Param{Pat: clonePat(p.Pat), Default: CloneExpr(p.Default)})
		}
		return c
	case *Paren:
		return &Paren{X: CloneExpr(e.X)}
	case *Raw:
		c := *e
		return &c
	}
	panic("ast: cannot clone")
}

// ListSite is a list in the tree from which an element may be removed.
type ListSite struct {
	Len    int
	Remove func(j int)
}

// Sites lists, in a fixed order, the expression slots and the lists of a
// program. Two programs of the same shape give sites in the same order, so a
// site found on a program can be applied to its clone.
func Sites(p *Program) (slots []*Expr, lists []ListSite) {
	var s sites
	s.consts(&p.Consts, 0)
	s.expr(&p.Result)
	return s.slots, s.lists
}

type sites struct {
	slots []*Expr
	lists []ListSite
	pats  []*Pattern
}

// PatternSites lists the patterns of const lines and parameters, in a fixed
// order, so that the reducer can replace a pattern with a name.
func PatternSites(p *Program) []*Pattern {
	var s sites
	s.consts(&p.Consts, 0)
	s.expr(&p.Result)
	return s.pats
}

func (s *sites) list(n int, remove func(int)) {
	if n > 0 {
		s.lists = append(s.lists, ListSite{Len: n, Remove: remove})
	}
}

func (s *sites) consts(cs *[]*Const, min int) {
	if len(*cs) > min {
		s.list(len(*cs), func(j int) { *cs = append((*cs)[:j:j], (*cs)[j+1:]...) })
	}
	for _, c := range *cs {
		if _, ok := c.Pat.(*NamePat); !ok {
			s.pats = append(s.pats, &c.Pat)
		}
		s.pat(c.Pat)
		s.expr(&c.Value)
	}
}

func (s *sites) branch(b *Branch) {
	s.consts(&b.Consts, 0)
	s.expr(&b.Result)
}

func (s *sites) exprs(es *[]Expr) {
	s.list(len(*es), func(j int) { *es = append((*es)[:j:j], (*es)[j+1:]...) })
	for i := range *es {
		s.expr(&(*es)[i])
	}
}

func (s *sites) pat(p Pattern) {
	switch p := p.(type) {
	case *MapPat:
		s.list(len(p.Fields), func(j int) { p.Fields = append(p.Fields[:j:j], p.Fields[j+1:]...) })
		for i := range p.Fields {
			if p.Fields[i].Default != nil {
				s.expr(&p.Fields[i].Default)
			}
		}
	case *ListPat:
		s.list(len(p.Elems), func(j int) { p.Elems = append(p.Elems[:j:j], p.Elems[j+1:]...) })
		for i := range p.Elems {
			if p.Elems[i].Default != nil {
				s.expr(&p.Elems[i].Default)
			}
		}
	}
}

func (s *sites) expr(ep *Expr) {
	s.slots = append(s.slots, ep)
	switch e := (*ep).(type) {
	case *Template:
		for i := range e.Parts {
			if e.Parts[i].X != nil {
				s.expr(&e.Parts[i].X)
			}
		}
	case *List:
		s.list(len(e.Items), func(j int) { e.Items = append(e.Items[:j:j], e.Items[j+1:]...) })
		for i := range e.Items {
			s.expr(&e.Items[i].X)
		}
	case *Map:
		s.list(len(e.Entries), func(j int) { e.Entries = append(e.Entries[:j:j], e.Entries[j+1:]...) })
		for i := range e.Entries {
			if e.Entries[i].KeyX != nil {
				s.expr(&e.Entries[i].KeyX)
			}
			if e.Entries[i].Kind != EntryShorthand {
				s.expr(&e.Entries[i].X)
			}
		}
	case *Unary:
		s.expr(&e.X)
	case *Binary:
		s.expr(&e.X)
		s.expr(&e.Y)
	case *Cond:
		s.expr(&e.Test)
		s.expr(&e.Then)
		s.expr(&e.Else)
	case *If:
		if len(e.Tests) > 1 {
			s.list(len(e.Tests), func(j int) {
				e.Tests = append(e.Tests[:j:j], e.Tests[j+1:]...)
				e.Then = append(e.Then[:j:j], e.Then[j+1:]...)
			})
		}
		for i := range e.Tests {
			s.expr(&e.Tests[i])
			s.branch(e.Then[i])
		}
		s.branch(e.Else)
	case *Block:
		s.consts(&e.Consts, 0)
		s.expr(&e.Result)
	case *Try:
		s.branch(e.Body)
		s.branch(e.Handler)
	case *Match:
		s.expr(&e.Subject)
		if len(e.Arms) > 1 {
			s.list(len(e.Arms), func(j int) { e.Arms = append(e.Arms[:j:j], e.Arms[j+1:]...) })
		}
		for i := range e.Arms {
			s.expr(&e.Arms[i].Result)
		}
	case *Call:
		s.exprs(&e.Args)
	case *Member:
		s.expr(&e.X)
	case *Index:
		s.expr(&e.X)
		s.expr(&e.I)
	case *MethodCall:
		s.expr(&e.X)
		s.exprs(&e.Args)
	case *Lambda:
		if e.Bare == "" {
			s.list(len(e.Params), func(j int) { e.Params = append(e.Params[:j:j], e.Params[j+1:]...) })
			for i := range e.Params {
				if _, ok := e.Params[i].Pat.(*NamePat); !ok {
					s.pats = append(s.pats, &e.Params[i].Pat)
				}
				s.pat(e.Params[i].Pat)
				if e.Params[i].Default != nil {
					s.expr(&e.Params[i].Default)
				}
			}
		}
		s.expr(&e.Body)
	case *Paren:
		s.expr(&e.X)
	}
}

// Children gives the direct expression children of a node, for the reducer.
func Children(e Expr) []Expr {
	var out []Expr
	add := func(xs ...Expr) {
		for _, x := range xs {
			if x != nil {
				out = append(out, x)
			}
		}
	}
	switch e := e.(type) {
	case *Template:
		for _, p := range e.Parts {
			add(p.X)
		}
	case *List:
		for _, it := range e.Items {
			add(it.X)
		}
	case *Map:
		for _, en := range e.Entries {
			add(en.KeyX)
			if en.Kind != EntryShorthand {
				add(en.X)
			}
		}
	case *Unary:
		add(e.X)
	case *Binary:
		add(e.X, e.Y)
	case *Cond:
		add(e.Test, e.Then, e.Else)
	case *If:
		for i, t := range e.Tests {
			add(t, e.Then[i].Result)
		}
		add(e.Else.Result)
	case *Block:
		add(e.Result)
	case *Try:
		add(e.Body.Result, e.Handler.Result)
	case *Match:
		add(e.Subject)
		for _, a := range e.Arms {
			add(a.Result)
		}
	case *Call:
		add(e.Args...)
	case *Member:
		add(e.X)
	case *Index:
		add(e.X, e.I)
	case *MethodCall:
		add(e.X)
		add(e.Args...)
	case *Paren:
		add(e.X)
	}
	return out
}
