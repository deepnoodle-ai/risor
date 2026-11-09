// Package spike is an experiment: Risor's parser and AST, with no compiler
// to bytecode and no virtual machine. Each AST node is compiled once into a
// Go closure. Values are the host's Go values, read in place:
//
//	nil, bool, int64, float64, string, []any, map[string]any
//
// plus *closure and builtin for functions. Nothing is converted on the way
// in or on the way out.
//
// It is a spike, not a product. It has a step budget, a call depth limit and
// error positions, but no observer, no per-operation size limits, no
// destructuring, no spread, no template strings, no match and no try.
package spike

import (
	"context"
	"fmt"
	"math"
	"reflect"
	"strings"

	"github.com/deepnoodle-ai/risor/v2/pkg/ast"
	"github.com/deepnoodle-ai/risor/v2/pkg/parser"
)

// node is one compiled AST node.
type node func(fr *frame) any

// Program is a compiled source text. It is immutable and safe to run from
// several goroutines at once.
type Program struct {
	body    node
	nslots  int
	globals []string
}

// runtime is the state of one Run.
type runtime struct {
	globals  []any
	steps    int64
	maxSteps int64
	depth    int
	pool     []*frame // free frames for functions that no closure can capture
	args     []any    // argument stack for builtin calls
}

const (
	inlineSlots = 6
	maxDepth    = 2000
)

// frame holds the variables of one function activation.
type frame struct {
	slots     []any
	parent    *frame
	rt        *runtime
	ret       any
	returning bool
	inline    [inlineSlots]any
}

// proto is a compiled function literal.
type proto struct {
	name      string
	nparams   int
	nslots    int
	body      node
	usesFlag  bool // body may leave its value in frame.ret
	recycle   bool // frames can be reused: no inner function can capture one
	enclosing bool
}

type closure struct {
	proto *proto
	env   *frame
}

// builtin is a host function. args is valid only for the duration of the call.
type builtin func(rt *runtime, args []any) any

// Error is a compile or run error with a source position.
type Error struct {
	Line, Column int
	Msg          string
}

func (e *Error) Error() string {
	if e.Line == 0 {
		return e.Msg
	}
	return fmt.Sprintf("%d:%d: %s", e.Line, e.Column, e.Msg)
}

type positioned interface {
	LineNumber() int
	ColumnNumber() int
}

func fail(pos positioned, format string, args ...any) {
	e := &Error{Msg: fmt.Sprintf(format, args...)}
	if pos != nil {
		e.Line, e.Column = pos.LineNumber(), pos.ColumnNumber()
	}
	panic(e)
}

var errBudget = &Error{Msg: "step budget exceeded"}

// tick charges one unit of work. It is called for every operator, field
// read, index, call, and iteration of a list method, so a budget bounds
// builtin-driven iteration as well as user code.
func (rt *runtime) tick() {
	rt.steps++
	if rt.steps > rt.maxSteps {
		panic(errBudget)
	}
}

// ---------------------------------------------------------------- compile

type blockScope struct {
	names  map[string]int
	parent *blockScope
}

type funcScope struct {
	parent   *funcScope
	block    *blockScope
	nslots   int
	hasInner bool
}

type compiler struct {
	fn       *funcScope
	builtins map[string]builtin
	globals  []string
	gindex   map[string]int
}

// Compile parses src with Risor's parser and compiles it to closures.
func Compile(ctx context.Context, src string) (p *Program, err error) {
	tree, err := parser.Parse(ctx, src, nil)
	if err != nil {
		return nil, err
	}
	defer func() {
		if r := recover(); r != nil {
			e, ok := r.(*Error)
			if !ok {
				panic(r)
			}
			p, err = nil, e
		}
	}()
	c := &compiler{builtins: stdBuiltins, gindex: map[string]int{}}
	c.fn = &funcScope{block: &blockScope{names: map[string]int{}}}
	body := c.stmts(tree.Stmts)
	return &Program{body: body, nslots: c.fn.nslots, globals: c.globals}, nil
}

func (c *compiler) declare(name string) int {
	slot := c.fn.nslots
	c.fn.nslots++
	c.fn.block.names[name] = slot
	return slot
}

func (c *compiler) pushBlock() {
	c.fn.block = &blockScope{names: map[string]int{}, parent: c.fn.block}
}
func (c *compiler) popBlock() { c.fn.block = c.fn.block.parent }

// resolve finds a local: how many function frames up, and which slot.
func (c *compiler) resolve(name string) (depth, slot int, ok bool) {
	for fn := c.fn; fn != nil; fn = fn.parent {
		for b := fn.block; b != nil; b = b.parent {
			if s, found := b.names[name]; found {
				return depth, s, true
			}
		}
		depth++
	}
	return 0, 0, false
}

// stmts compiles a statement list. Its value is the value of the last
// statement, unless a return ran, in which case frame.returning is set.
func (c *compiler) stmts(list []ast.Node) node {
	nodes := make([]node, len(list))
	for i, s := range list {
		nodes[i] = c.stmt(s)
	}
	switch len(nodes) {
	case 0:
		return func(*frame) any { return nil }
	case 1:
		return nodes[0]
	}
	return func(fr *frame) any {
		var v any
		for _, n := range nodes {
			v = n(fr)
			if fr.returning {
				return nil
			}
		}
		return v
	}
}

func (c *compiler) stmt(n ast.Node) node {
	switch n := n.(type) {
	case *ast.Var:
		value := c.expr(n.Value)
		slot := c.declare(n.Name.Name)
		return func(fr *frame) any { fr.slots[slot] = value(fr); return nil }
	case *ast.Const:
		value := c.expr(n.Value)
		slot := c.declare(n.Name.Name)
		return func(fr *frame) any { fr.slots[slot] = value(fr); return nil }
	case *ast.Return:
		if n.Value == nil {
			return func(fr *frame) any { fr.ret, fr.returning = nil, true; return nil }
		}
		value := c.expr(n.Value)
		return func(fr *frame) any { fr.ret, fr.returning = value(fr), true; return nil }
	case *ast.Func:
		if n.Name != nil {
			// Declare first so the body can call itself.
			slot := c.declare(n.Name.Name)
			fn := c.function(n)
			return func(fr *frame) any { fr.slots[slot] = fn(fr); return nil }
		}
		return c.function(n)
	case ast.Expr:
		return c.expr(n)
	}
	fail(n.Pos(), "unsupported statement %T", n)
	return nil
}

func hasReturn(list []ast.Node) bool {
	found := false
	for _, s := range list {
		ast.Inspect(s, func(n ast.Node) bool {
			switch n.(type) {
			case *ast.Func:
				return false
			case *ast.Return:
				found = true
			}
			return !found
		})
	}
	return found
}

func (c *compiler) function(n *ast.Func) node {
	if len(n.Defaults) > 0 || n.RestParam != nil {
		fail(n.Pos(), "default and rest parameters are not supported")
	}
	c.fn.hasInner = true
	fs := &funcScope{parent: c.fn, block: &blockScope{names: map[string]int{}}}
	c.fn = fs
	for _, p := range n.Params {
		id, ok := p.(*ast.Ident)
		if !ok {
			fail(p.Pos(), "destructuring parameters are not supported")
		}
		c.declare(id.Name)
	}
	p := &proto{nparams: len(n.Params)}
	if n.Name != nil {
		p.name = n.Name.Name
	}
	body := n.Body.Stmts
	if ret, ok := single(body).(*ast.Return); ok && ret.Value != nil {
		// An arrow function with an expression body, or `{ return x }`.
		p.body = c.expr(ret.Value)
	} else {
		p.usesFlag = hasReturn(body)
		p.body = c.stmts(body)
	}
	p.nslots = fs.nslots
	p.recycle = !fs.hasInner && fs.nslots <= inlineSlots
	c.fn = fs.parent
	return func(fr *frame) any { return &closure{proto: p, env: fr} }
}

func single(list []ast.Node) ast.Node {
	if len(list) == 1 {
		return list[0]
	}
	return nil
}

func (c *compiler) block(b *ast.Block) node {
	c.pushBlock()
	defer c.popBlock()
	return c.stmts(b.Stmts)
}

func constant(v any) node { return func(*frame) any { return v } }

// literal reports the value of a node that is a constant.
func literal(n ast.Node) (any, bool) {
	switch n := n.(type) {
	case *ast.Int:
		return n.Value, true
	case *ast.Float:
		return n.Value, true
	case *ast.Bool:
		return n.Value, true
	case *ast.Nil:
		return nil, true
	case *ast.String:
		if n.Template == nil {
			return n.Value, true
		}
	}
	return nil, false
}

func (c *compiler) expr(n ast.Node) node {
	if v, ok := literal(n); ok {
		return constant(v)
	}
	switch n := n.(type) {
	case *ast.Ident:
		return c.ident(n)
	case *ast.List:
		return c.list(n)
	case *ast.Map:
		return c.mapLit(n)
	case *ast.Prefix:
		return c.prefix(n)
	case *ast.Infix:
		return c.infix(n)
	case *ast.If:
		return c.ifExpr(n)
	case *ast.Func:
		if n.Name != nil {
			fail(n.Pos(), "a named function is a statement")
		}
		return c.function(n)
	case *ast.Call:
		return c.call(n)
	case *ast.ObjectCall:
		return c.method(n)
	case *ast.GetAttr:
		return c.attr(n)
	case *ast.Index:
		return c.index(n)
	case *ast.In:
		return c.in(n.X, n.Y, false)
	case *ast.NotIn:
		return c.in(n.X, n.Y, true)
	}
	fail(n.Pos(), "unsupported expression %T", n)
	return nil
}

func (c *compiler) ident(n *ast.Ident) node {
	name := n.Name
	if depth, slot, ok := c.resolve(name); ok {
		switch depth {
		case 0:
			return func(fr *frame) any { return fr.slots[slot] }
		case 1:
			return func(fr *frame) any { return fr.parent.slots[slot] }
		case 2:
			return func(fr *frame) any { return fr.parent.parent.slots[slot] }
		}
		return func(fr *frame) any {
			f := fr
			for i := 0; i < depth; i++ {
				f = f.parent
			}
			return f.slots[slot]
		}
	}
	if b, ok := c.builtins[name]; ok {
		return constant(b)
	}
	i, ok := c.gindex[name]
	if !ok {
		i = len(c.globals)
		c.globals = append(c.globals, name)
		c.gindex[name] = i
	}
	return func(fr *frame) any { return fr.rt.globals[i] }
}

func (c *compiler) list(n *ast.List) node {
	// A list of constants is built once. Values are immutable, so every
	// evaluation can share it.
	consts := make([]any, 0, len(n.Items))
	for _, item := range n.Items {
		v, ok := literal(item)
		if !ok {
			consts = nil
			break
		}
		consts = append(consts, v)
	}
	if consts != nil {
		return constant(consts)
	}
	items := make([]node, len(n.Items))
	for i, item := range n.Items {
		items[i] = c.expr(item)
	}
	return func(fr *frame) any {
		out := make([]any, len(items))
		for i, item := range items {
			out[i] = item(fr)
		}
		return out
	}
}

func (c *compiler) mapLit(n *ast.Map) node {
	keys := make([]node, len(n.Items))
	values := make([]node, len(n.Items))
	for i, item := range n.Items {
		if item.Key == nil {
			fail(n.Pos(), "spread is not supported")
		}
		keys[i], values[i] = c.expr(item.Key), c.expr(item.Value)
	}
	var pos positioned = n.Pos()
	return func(fr *frame) any {
		out := make(map[string]any, len(keys))
		for i, k := range keys {
			key, ok := k(fr).(string)
			if !ok {
				fail(pos, "a map key must be a string")
			}
			out[key] = values[i](fr)
		}
		return out
	}
}

func (c *compiler) prefix(n *ast.Prefix) node {
	x := c.expr(n.X)
	var pos positioned = n.Pos()
	switch n.Op {
	case "!", "not":
		return func(fr *frame) any {
			v := x(fr)
			if b, ok := v.(bool); ok {
				return !b
			}
			return !truthy(v)
		}
	case "-":
		return func(fr *frame) any {
			switch v := x(fr).(type) {
			case int64:
				return -v
			case float64:
				return -v
			default:
				fail(pos, "cannot negate %s", typeName(v))
				return nil
			}
		}
	}
	fail(pos, "unsupported prefix operator %q", n.Op)
	return nil
}

func (c *compiler) ifExpr(n *ast.If) node {
	cond := c.expr(n.Cond)
	then := c.block(n.Consequence)
	if n.Alternative == nil {
		return func(fr *frame) any {
			if truthy(cond(fr)) {
				return then(fr)
			}
			return nil
		}
	}
	els := c.block(n.Alternative)
	return func(fr *frame) any {
		if truthy(cond(fr)) {
			return then(fr)
		}
		return els(fr)
	}
}

func (c *compiler) infix(n *ast.Infix) node {
	x, y := c.expr(n.X), c.expr(n.Y)
	var pos positioned = n.OpPos
	switch n.Op {
	case "&&":
		return func(fr *frame) any {
			if v := x(fr); !truthy(v) {
				return v
			}
			return y(fr)
		}
	case "||":
		return func(fr *frame) any {
			if v := x(fr); truthy(v) {
				return v
			}
			return y(fr)
		}
	case "??":
		return func(fr *frame) any {
			if v := x(fr); v != nil {
				return v
			}
			return y(fr)
		}
	case "==":
		return func(fr *frame) any {
			a, b := x(fr), y(fr)
			fr.rt.tick()
			return equal(a, b)
		}
	case "!=":
		return func(fr *frame) any {
			a, b := x(fr), y(fr)
			fr.rt.tick()
			return !equal(a, b)
		}
	case "<", "<=", ">", ">=":
		return c.compare(n.Op, x, y, pos)
	case "+":
		return func(fr *frame) any {
			a, b := x(fr), y(fr)
			fr.rt.tick()
			switch a := a.(type) {
			case int64:
				if b, ok := b.(int64); ok {
					return a + b
				}
			case float64:
				if b, ok := b.(float64); ok {
					return a + b
				}
			case string:
				if b, ok := b.(string); ok {
					return a + b
				}
			}
			return arith("+", a, b, pos)
		}
	case "-":
		return func(fr *frame) any {
			a, b := x(fr), y(fr)
			fr.rt.tick()
			switch a := a.(type) {
			case int64:
				if b, ok := b.(int64); ok {
					return a - b
				}
			case float64:
				if b, ok := b.(float64); ok {
					return a - b
				}
			}
			return arith("-", a, b, pos)
		}
	case "*":
		return func(fr *frame) any {
			a, b := x(fr), y(fr)
			fr.rt.tick()
			switch a := a.(type) {
			case int64:
				if b, ok := b.(int64); ok {
					return a * b
				}
			case float64:
				if b, ok := b.(float64); ok {
					return a * b
				}
			}
			return arith("*", a, b, pos)
		}
	case "%":
		return func(fr *frame) any {
			a, b := x(fr), y(fr)
			fr.rt.tick()
			if a, ok := a.(int64); ok {
				if b, ok := b.(int64); ok && b != 0 {
					return a % b
				}
			}
			return arith("%", a, b, pos)
		}
	case "/":
		return func(fr *frame) any {
			a, b := x(fr), y(fr)
			fr.rt.tick()
			return arith("/", a, b, pos)
		}
	}
	fail(pos, "unsupported operator %q", n.Op)
	return nil
}

func (c *compiler) compare(op string, x, y node, pos positioned) node {
	var test func(int) bool
	switch op {
	case "<":
		test = func(c int) bool { return c < 0 }
	case "<=":
		test = func(c int) bool { return c <= 0 }
	case ">":
		test = func(c int) bool { return c > 0 }
	default:
		test = func(c int) bool { return c >= 0 }
	}
	generic := func(a, b any) any {
		if af, ok := number(a); ok {
			if bf, ok := number(b); ok {
				switch {
				case af < bf:
					return test(-1)
				case af > bf:
					return test(1)
				}
				return test(0)
			}
		}
		if as, ok := a.(string); ok {
			if bs, ok := b.(string); ok {
				return test(strings.Compare(as, bs))
			}
		}
		fail(pos, "cannot compare %s and %s", typeName(a), typeName(b))
		return nil
	}
	// The same-type cases are written out per operator so that each is a
	// single comparison with no indirect call.
	switch op {
	case "<":
		return func(fr *frame) any {
			a, b := x(fr), y(fr)
			fr.rt.tick()
			switch a := a.(type) {
			case int64:
				if b, ok := b.(int64); ok {
					return a < b
				}
			case float64:
				if b, ok := b.(float64); ok {
					return a < b
				}
			}
			return generic(a, b)
		}
	case ">":
		return func(fr *frame) any {
			a, b := x(fr), y(fr)
			fr.rt.tick()
			switch a := a.(type) {
			case int64:
				if b, ok := b.(int64); ok {
					return a > b
				}
			case float64:
				if b, ok := b.(float64); ok {
					return a > b
				}
			}
			return generic(a, b)
		}
	}
	return func(fr *frame) any {
		a, b := x(fr), y(fr)
		fr.rt.tick()
		return generic(a, b)
	}
}

func (c *compiler) in(xn, yn ast.Expr, negate bool) node {
	x, y := c.expr(xn), c.expr(yn)
	var pos positioned = xn.Pos()
	return func(fr *frame) any {
		a, b := x(fr), y(fr)
		fr.rt.tick()
		found := false
		switch b := b.(type) {
		case []any:
			for _, item := range b {
				if equal(a, item) {
					found = true
					break
				}
			}
		case map[string]any:
			if key, ok := a.(string); ok {
				_, found = b[key]
			}
		case string:
			sub, ok := a.(string)
			if !ok {
				fail(pos, "cannot look for %s in a string", typeName(a))
			}
			found = strings.Contains(b, sub)
		default:
			fail(pos, "cannot use in on %s", typeName(b))
		}
		return found != negate
	}
}

// attr compiles x.name. A dot always reads a field. It never returns a
// method, so a map key named "keys" or "get" reads like any other key.
func (c *compiler) attr(n *ast.GetAttr) node {
	x := c.expr(n.X)
	name := n.Attr.Name
	optional := n.Optional
	var pos positioned = n.Attr.Pos()
	return func(fr *frame) any {
		v := x(fr)
		fr.rt.tick()
		if m, ok := v.(map[string]any); ok {
			if field, ok := m[name]; ok {
				return field
			}
			if optional {
				return nil
			}
			fail(pos, "no field %q", name)
		}
		if v == nil && optional {
			return nil
		}
		fail(pos, "cannot read field %q of %s", name, typeName(v))
		return nil
	}
}

func (c *compiler) index(n *ast.Index) node {
	x, idx := c.expr(n.X), c.expr(n.Index)
	var pos positioned = n.Lbrack
	return func(fr *frame) any {
		v, k := x(fr), idx(fr)
		fr.rt.tick()
		switch v := v.(type) {
		case []any:
			i, ok := k.(int64)
			if !ok {
				fail(pos, "a list index must be an int, got %s", typeName(k))
			}
			if i < 0 {
				i += int64(len(v))
			}
			if i < 0 || i >= int64(len(v)) {
				fail(pos, "index %d out of range for a list of %d", i, len(v))
			}
			return v[i]
		case map[string]any:
			key, ok := k.(string)
			if !ok {
				fail(pos, "a map key must be a string, got %s", typeName(k))
			}
			field, ok := v[key]
			if !ok {
				fail(pos, "no key %q", key)
			}
			return field
		}
		fail(pos, "cannot index %s", typeName(v))
		return nil
	}
}

func (c *compiler) args(list []ast.Node) []node {
	out := make([]node, len(list))
	for i, a := range list {
		e, ok := a.(ast.Expr)
		if !ok {
			fail(a.Pos(), "spread arguments are not supported")
		}
		out[i] = c.expr(e)
	}
	return out
}

func (c *compiler) call(n *ast.Call) node {
	args := c.args(n.Args)
	var pos positioned = n.Lparen
	// A call to a builtin by name is bound at compile time.
	if id, ok := n.Fun.(*ast.Ident); ok {
		if _, _, local := c.resolve(id.Name); !local {
			if b, ok := c.builtins[id.Name]; ok {
				return func(fr *frame) any { return callBuiltin(fr, b, args) }
			}
		}
	}
	fun := c.expr(n.Fun)
	return func(fr *frame) any {
		switch f := fun(fr).(type) {
		case *closure:
			return callClosure(fr, f, args, pos)
		case builtin:
			return callBuiltin(fr, f, args)
		default:
			fail(pos, "cannot call %s", typeName(f))
			return nil
		}
	}
}

func callBuiltin(fr *frame, b builtin, args []node) any {
	rt := fr.rt
	rt.tick()
	base := len(rt.args)
	for _, a := range args {
		v := a(fr)
		rt.args = append(rt.args, v)
	}
	out := b(rt, rt.args[base:])
	clear(rt.args[base:])
	rt.args = rt.args[:base]
	return out
}

func callClosure(fr *frame, cl *closure, args []node, pos positioned) any {
	p := cl.proto
	if len(args) != p.nparams {
		fail(pos, "function takes %d arguments, got %d", p.nparams, len(args))
	}
	rt := fr.rt
	rt.tick()
	f := rt.enter(p, cl.env)
	for i, a := range args {
		f.slots[i] = a(fr)
	}
	return rt.leave(p, f, p.body(f))
}

// enter returns a frame for one activation of p.
func (rt *runtime) enter(p *proto, env *frame) *frame {
	rt.depth++
	if rt.depth > maxDepth {
		fail(nil, "call depth exceeded")
	}
	var f *frame
	if n := len(rt.pool); p.recycle && n > 0 {
		f = rt.pool[n-1]
		rt.pool = rt.pool[:n-1]
	} else {
		f = &frame{rt: rt}
	}
	f.parent = env
	if p.nslots <= inlineSlots {
		f.slots = f.inline[:p.nslots]
	} else {
		f.slots = make([]any, p.nslots)
	}
	return f
}

// leave finishes an activation and returns its value.
func (rt *runtime) leave(p *proto, f *frame, v any) any {
	if p.usesFlag && f.returning {
		v = f.ret
		f.ret, f.returning = nil, false
	}
	rt.depth--
	if p.recycle {
		clear(f.slots)
		f.parent = nil
		rt.pool = append(rt.pool, f)
	}
	return v
}

func (rt *runtime) call1(fn any, a any, pos positioned) any {
	cl, ok := fn.(*closure)
	if !ok {
		if b, ok := fn.(builtin); ok {
			arg := [1]any{a}
			return b(rt, arg[:])
		}
		fail(pos, "expected a function, got %s", typeName(fn))
	}
	p := cl.proto
	if p.nparams != 1 {
		fail(pos, "expected a function of 1 argument, got one of %d", p.nparams)
	}
	f := rt.enter(p, cl.env)
	f.slots[0] = a
	return rt.leave(p, f, p.body(f))
}

func (rt *runtime) call2(fn any, a, b any, pos positioned) any {
	cl, ok := fn.(*closure)
	if !ok || cl.proto.nparams != 2 {
		fail(pos, "expected a function of 2 arguments")
	}
	p := cl.proto
	f := rt.enter(p, cl.env)
	f.slots[0], f.slots[1] = a, b
	return rt.leave(p, f, p.body(f))
}

// method compiles x.name(args). Methods exist only in call position.
func (c *compiler) method(n *ast.ObjectCall) node {
	x := c.expr(n.X)
	id, ok := n.Call.Fun.(*ast.Ident)
	if !ok {
		fail(n.Pos(), "unsupported method call")
	}
	name := id.Name
	args := c.args(n.Call.Args)
	optional := n.Optional
	var pos positioned = id.Pos()
	listMethod := listMethods[name]
	return func(fr *frame) any {
		recv := x(fr)
		switch recv := recv.(type) {
		case []any:
			if listMethod == nil {
				fail(pos, "a list has no method %q", name)
			}
			return listMethod(fr, recv, args, pos)
		case map[string]any:
			// A map has no methods. A field that holds a function is called.
			if field, ok := recv[name]; ok {
				if cl, ok := field.(*closure); ok {
					return callClosure(fr, cl, args, pos)
				}
			}
			fail(pos, "no method %q", name)
		case nil:
			if optional {
				return nil
			}
		}
		fail(pos, "%s has no method %q", typeName(recv), name)
		return nil
	}
}

type listMethod func(fr *frame, recv []any, args []node, pos positioned) any

var listMethods = map[string]listMethod{
	"filter": func(fr *frame, recv []any, args []node, pos positioned) any {
		fn := oneArg(fr, args, pos)
		rt := fr.rt
		var out []any
		for _, item := range recv {
			rt.tick()
			if truthy(rt.call1(fn, item, pos)) {
				out = append(out, item)
			}
		}
		if out == nil {
			out = []any{}
		}
		return out
	},
	"map": func(fr *frame, recv []any, args []node, pos positioned) any {
		fn := oneArg(fr, args, pos)
		rt := fr.rt
		out := make([]any, len(recv))
		for i, item := range recv {
			rt.tick()
			out[i] = rt.call1(fn, item, pos)
		}
		return out
	},
	"reduce": func(fr *frame, recv []any, args []node, pos positioned) any {
		if len(args) != 2 {
			fail(pos, "reduce takes 2 arguments, got %d", len(args))
		}
		acc, fn := args[0](fr), args[1](fr)
		rt := fr.rt
		for _, item := range recv {
			rt.tick()
			acc = rt.call2(fn, acc, item, pos)
		}
		return acc
	},
}

func oneArg(fr *frame, args []node, pos positioned) any {
	if len(args) != 1 {
		fail(pos, "method takes 1 argument, got %d", len(args))
	}
	return args[0](fr)
}

// ------------------------------------------------------------------ values

func typeName(v any) string {
	switch v.(type) {
	case nil:
		return "nil"
	case bool:
		return "bool"
	case int64:
		return "int"
	case float64:
		return "float"
	case string:
		return "string"
	case []any:
		return "list"
	case map[string]any:
		return "map"
	case *closure, builtin:
		return "function"
	}
	return fmt.Sprintf("%T", v)
}

func truthy(v any) bool {
	switch v := v.(type) {
	case bool:
		return v
	case nil:
		return false
	case int64:
		return v != 0
	case float64:
		return v != 0
	case string:
		return v != ""
	case []any:
		return len(v) > 0
	case map[string]any:
		return len(v) > 0
	}
	return true
}

func number(v any) (float64, bool) {
	switch v := v.(type) {
	case int64:
		return float64(v), true
	case float64:
		return v, true
	}
	return 0, false
}

func equal(a, b any) bool {
	switch a := a.(type) {
	case string:
		b, ok := b.(string)
		return ok && a == b
	case bool:
		b, ok := b.(bool)
		return ok && a == b
	case nil:
		return b == nil
	case int64:
		switch b := b.(type) {
		case int64:
			return a == b
		case float64:
			return float64(a) == b
		}
		return false
	case float64:
		switch b := b.(type) {
		case float64:
			return a == b
		case int64:
			return a == float64(b)
		}
		return false
	}
	return reflect.DeepEqual(a, b)
}

// arith is the slow path of the arithmetic operators: mixed int and float,
// division, remainder, and the errors.
func arith(op string, a, b any, pos positioned) any {
	ai, aInt := a.(int64)
	bi, bInt := b.(int64)
	if aInt && bInt {
		switch op {
		case "+":
			return ai + bi
		case "-":
			return ai - bi
		case "*":
			return ai * bi
		case "/":
			if bi == 0 {
				fail(pos, "division by zero")
			}
			return ai / bi
		case "%":
			if bi == 0 {
				fail(pos, "division by zero")
			}
			return ai % bi
		}
	}
	af, aNum := number(a)
	bf, bNum := number(b)
	if !aNum || !bNum {
		fail(pos, "unsupported operation: %s %s %s", typeName(a), op, typeName(b))
	}
	switch op {
	case "+":
		return af + bf
	case "-":
		return af - bf
	case "*":
		return af * bf
	case "/":
		if bf == 0 {
			fail(pos, "division by zero")
		}
		return af / bf
	case "%":
		if bf == 0 {
			fail(pos, "division by zero")
		}
		return math.Mod(af, bf)
	}
	fail(pos, "unsupported operator %q", op)
	return nil
}

var stdBuiltins = map[string]builtin{
	"len": func(rt *runtime, args []any) any {
		if len(args) != 1 {
			fail(nil, "len takes 1 argument, got %d", len(args))
		}
		switch v := args[0].(type) {
		case string:
			return int64(len(v))
		case []any:
			return int64(len(v))
		case map[string]any:
			return int64(len(v))
		}
		fail(nil, "len of %s", typeName(args[0]))
		return nil
	},
	"coalesce": func(rt *runtime, args []any) any {
		for _, a := range args {
			if a != nil {
				return a
			}
		}
		return nil
	},
	"any": func(rt *runtime, args []any) any {
		list := listArg("any", args)
		for _, item := range list {
			rt.tick()
			if truthy(item) {
				return true
			}
		}
		return false
	},
	"all": func(rt *runtime, args []any) any {
		list := listArg("all", args)
		for _, item := range list {
			rt.tick()
			if !truthy(item) {
				return false
			}
		}
		return true
	},
}

func listArg(name string, args []any) []any {
	if len(args) != 1 {
		fail(nil, "%s takes 1 argument, got %d", name, len(args))
	}
	list, ok := args[0].([]any)
	if !ok {
		fail(nil, "%s takes a list, got %s", name, typeName(args[0]))
	}
	return list
}

// -------------------------------------------------------------------- run

// run is everything one evaluation allocates up front, in one object.
type run struct {
	rt   runtime
	top  frame
	gbuf [4]any
	abuf [4]any
}

// Run evaluates the program against env. The values in env are read in
// place and are never written.
func (p *Program) Run(env map[string]any, maxSteps int64) (result any, err error) {
	r := &run{}
	rt := &r.rt
	rt.maxSteps = maxSteps
	if maxSteps <= 0 {
		rt.maxSteps = math.MaxInt64
	}
	if len(p.globals) <= len(r.gbuf) {
		rt.globals = r.gbuf[:len(p.globals)]
	} else {
		rt.globals = make([]any, len(p.globals))
	}
	for i, name := range p.globals {
		v, ok := env[name]
		if !ok {
			return nil, &Error{Msg: fmt.Sprintf("undefined variable %q", name)}
		}
		rt.globals[i] = v
	}
	rt.args = r.abuf[:0]
	top := &r.top
	top.rt = rt
	if p.nslots <= inlineSlots {
		top.slots = top.inline[:p.nslots]
	} else {
		top.slots = make([]any, p.nslots)
	}
	defer func() {
		if rec := recover(); rec != nil {
			e, ok := rec.(*Error)
			if !ok {
				panic(rec)
			}
			result, err = nil, e
		}
	}()
	v := p.body(top)
	if top.returning {
		v = top.ret
	}
	return v, nil
}
