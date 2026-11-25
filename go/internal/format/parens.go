package format

import "github.com/deepnoodle-ai/risor/go/v3/internal/syntax"

// The parentheses of section 2.6.

// role is the place a subexpression stands in its parent.
type role uint8

const (
	rOther    role = iota
	rLeft          // the left operand of a binary operator, or of as and is
	rRight         // the right operand of a binary operator
	rOperand       // the operand of a prefix operator
	rReceiver      // the receiver of a step of a chain
	rTest          // the test of ? :
	rSubject       // the subject of match
)

var binLevel = map[string]int{
	"??": 2, "||": 3, "&&": 4, "==": 5, "!=": 5,
	"<": 6, "<=": 6, ">": 6, ">=": 6, "in": 6, "not-in": 6,
	"+": 7, "-": 7, "*": 8, "/": 8, "%": 8, "**": 10,
}

// level is the level of an expression (2.6).
func level(n syntax.Node) int {
	switch n := n.(type) {
	case *syntax.Cond, *syntax.Try:
		return 1
	case *syntax.Binary:
		return binLevel[n.Op]
	case *syntax.As:
		return 6
	case *syntax.Unary:
		return 9
	case *syntax.Lambda:
		return 0
	}
	return 11
}

func isBin(n syntax.Node, ops ...string) bool {
	b, ok := n.(*syntax.Binary)
	if !ok {
		return false
	}
	for _, o := range ops {
		if b.Op == o {
			return true
		}
	}
	return false
}

// isCmp reports whether n is a comparison (01-language.md 3.3).
func isCmp(n syntax.Node) bool {
	b, ok := n.(*syntax.Binary)
	return ok && (binLevel[b.Op] == 5 || binLevel[b.Op] == 6)
}

// cmpUnder reports whether n is a comparison, or an as or is whose left
// operand, written without parentheses, is one: the text of n then holds a
// comparison operator outside parentheses (01-language.md 3.4 rule 13).
func cmpUnder(n syntax.Node) bool {
	for {
		a, ok := n.(*syntax.As)
		if !ok || needParens(a, rLeft, a.X) {
			return isCmp(n)
		}
		n = a.X
	}
}

func isNot(n syntax.Node) bool {
	u, ok := n.(*syntax.Unary)
	return ok && u.Op == "!"
}

// needParens reports whether x is written in parentheses in its place by
// the first table of 2.6. The pairs for the reader are readerParens.
func needParens(parent syntax.Node, r role, x syntax.Node) bool {
	lx := level(x)
	switch r {
	case rLeft, rRight:
		var op string
		var p int
		switch pn := parent.(type) {
		case *syntax.Binary:
			op, p = pn.Op, binLevel[pn.Op]
		case *syntax.As:
			op, p = "as", 6
		}
		switch op {
		case "??":
			if r == rRight {
				return lx < 7
			}
			return lx < 7 && !isBin(x, "??")
		case "**":
			if r == rLeft {
				return lx < 11
			}
			return lx < 9
		}
		if (op == "in" || op == "not-in" || op == "as") && r == rLeft && isNot(x) {
			return true
		}
		if isBin(x, "??") {
			return true
		}
		if r == rLeft && lx < p || r == rRight && lx <= p {
			return true
		}
		// A comparison inside a comparison, and on the right one under as
		// or is (01-language.md 3.4 rule 13). as and is stand at level 6 but
		// are not comparisons: a < b as bool needs no pair.
		if op != "as" && (p == 5 || p == 6) && (isCmp(x) || r == rRight && cmpUnder(x)) {
			return true
		}
		return false
	case rOperand:
		return lx < 9 || isBin(x, "**")
	case rReceiver:
		if lx < 11 {
			return true
		}
		if _, ok := x.(*syntax.OptChain); ok {
			return true
		}
		if _, ok := x.(*syntax.Each); ok {
			_, call := parent.(*syntax.MethodCall)
			return !call
		}
		return false
	case rTest:
		return lx == 1
	case rSubject:
		return startsBrace(x)
	}
	return false
}

// readerParens reports whether x has a place for a pair for the reader
// (2.6, the second table). Each such x is a Binary.
func readerParens(parent syntax.Node, r role, x syntax.Node) bool {
	pb, ok := parent.(*syntax.Binary)
	if !ok || (r != rLeft && r != rRight) || needParens(parent, r, x) {
		return false
	}
	xb, ok := x.(*syntax.Binary)
	if !ok {
		return false
	}
	if pb.Op == "||" && xb.Op == "&&" {
		return true
	}
	mul := func(o string) bool { return o == "*" || o == "/" || o == "%" }
	return r == rLeft && mul(pb.Op) && mul(xb.Op) && (xb.Op != pb.Op || pb.Op == "%")
}

// isMul reports whether n is a Binary of *, / or %.
func isMul(n syntax.Node) bool { return isBin(n, "*", "/", "%") }

// reader reports whether a pair for the reader is written around x (2.6
// items 3 and 4): only in a run of at most 3 operators, and only when no
// pair of another run encloses the run. parent is a Binary of the run: of
// *, / and % for the second row, of || for the first.
func (b *builder) reader(parent syntax.Node, r role, x syntax.Node) bool {
	return !b.plain && readerParens(parent, r, x) && b.rdepth == 0 && b.runLen(parent.(*syntax.Binary)) <= 3
}

// runLen is the number of operators of the run that n is in: the Binary
// nodes of one kind (*, / and %, or ||) along the left operands, from the top
// of the run. The builder meets the top of a run first, so the first call
// for a run is at its top.
func (b *builder) runLen(n *syntax.Binary) int {
	if l, ok := b.runs[n]; ok {
		return l
	}
	same := func(x syntax.Node) bool {
		if n.Op == "||" {
			return isBin(x, "||")
		}
		return isMul(x)
	}
	var nodes []*syntax.Binary
	for x := syntax.Node(n); same(x); x = x.(*syntax.Binary).X {
		nodes = append(nodes, x.(*syntax.Binary))
	}
	for _, m := range nodes {
		b.runs[m] = len(nodes)
	}
	return len(nodes)
}

// paren reports whether x is written in parentheses in its place: by the
// first table of 2.6, or for the reader.
func (b *builder) paren(parent syntax.Node, r role, x syntax.Node) bool {
	if r == rSubject {
		return b.startsBrace(x)
	}
	return needParens(parent, r, x) || b.reader(parent, r, x)
}

// startsBrace reports whether the text of x begins with {, if or try
// (01-language.md 3.4 rule 6).
func startsBrace(x syntax.Node) bool {
	return (&builder{runs: map[*syntax.Binary]int{}}).startsBrace(x)
}

// startsBrace reports whether the text of x, written at this place, begins
// with {, if or try. A pair for the reader on its left edge is a (.
func (b *builder) startsBrace(x syntax.Node) bool {
	for {
		switch n := x.(type) {
		case *syntax.Map, *syntax.Block:
			return true
		case *syntax.Cond:
			if n.Form == "if" {
				return true
			}
			if needParens(n, rTest, n.Test) {
				return false
			}
			x = n.Test
		case *syntax.Try:
			return true
		case *syntax.Binary:
			if needParens(n, rLeft, n.X) || b.reader(n, rLeft, n.X) {
				return false
			}
			x = n.X
		case *syntax.As:
			if needParens(n, rLeft, n.X) {
				return false
			}
			x = n.X
		case *syntax.OptChain:
			x = n.X
		case *syntax.Member:
			if needParens(n, rReceiver, n.X) {
				return false
			}
			x = n.X
		case *syntax.Index:
			if needParens(n, rReceiver, n.X) {
				return false
			}
			x = n.X
		case *syntax.MethodCall:
			if needParens(n, rReceiver, n.X) {
				return false
			}
			x = n.X
		case *syntax.Each:
			if needParens(n, rReceiver, n.X) {
				return false
			}
			x = n.X
		default:
			return false
		}
	}
}
