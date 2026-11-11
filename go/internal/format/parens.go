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

func isNot(n syntax.Node) bool {
	u, ok := n.(*syntax.Unary)
	return ok && u.Op == "!"
}

// needParens reports whether x is written in parentheses in its place.
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
		// The parentheses added for the reader.
		if op == "||" && isBin(x, "&&") {
			return true
		}
		if r == rLeft {
			xb, _ := x.(*syntax.Binary)
			if xb != nil {
				mul := func(o string) bool { return o == "*" || o == "/" || o == "%" }
				if mul(op) && mul(xb.Op) && (xb.Op != op || op == "%") {
					return true
				}
				if (op == "==" || op == "!=") && (xb.Op == "==" || xb.Op == "!=") {
					return true
				}
			}
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

// startsBrace reports whether the text of x begins with {, if or try
// (01-language.md 3.4 rule 6).
func startsBrace(x syntax.Node) bool {
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
			if needParens(n, rLeft, n.X) {
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
