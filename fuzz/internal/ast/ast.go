// Package ast holds the syntax tree that the fuzzer builds, and its printer.
//
// The tree follows the grammar of spec/core/01-language.md section 3. It is
// the fuzzer's own: no implementation's tree is used. The printer adds every
// parenthesis the grammar needs, so a printed tree parses back to the same
// tree (apart from the Paren nodes, which only add parentheses).
package ast

// Expr is an expression node.
type Expr interface{ isExpr() }

// Num is a number literal, kept as the text it is written with.
type Num struct{ Text string }

// Str is a one-line string, or a template with no interpolation when Quote
// is a backtick. Esc chooses how code points outside ASCII are written: 0 as
// they are, 1 as \u{X}, 2 as \uXXXX (with surrogate pairs).
//
// Package F3 adds two forms: Raw writes r"..." or r'...' (with Quote), and
// Block writes a text block (1) or a raw text block (2) whose closing line
// is Indent. A form that cannot hold the value is written as a one-line
// string instead.
type Str struct {
	Val    string
	Quote  byte
	Esc    int
	Raw    bool
	Block  int
	Indent string
}

// Part is one part of a template: text when X is nil, else an interpolation.
type Part struct {
	Text string
	X    Expr
}

// Template is a template with at least one interpolation.
// Block writes it as a text block of package F3, with the closing line
// Indent.
type Template struct {
	Parts  []Part
	Block  bool
	Indent string
}

// Bool is true or false.
type Bool struct{ V bool }

// Null is null.
type Null struct{}

// Ident is a name read.
type Ident struct{ Name string }

// Item is one item of a list literal.
type Item struct {
	Spread bool
	X      Expr
}

// List is a list literal.
type List struct{ Items []Item }

// EntryKind is the form of a map entry.
type EntryKind int

// The forms of a map entry.
const (
	EntryWord EntryKind = iota
	EntryString
	EntryComputed
	EntryShorthand
	EntrySpread
)

// Entry is one entry of a map literal.
type Entry struct {
	Kind EntryKind
	Key  string // the word, the string's value, or the shorthand name
	Raw  bool   // a string key written as a raw string, when it can be
	KeyX Expr   // the computed key
	X    Expr   // the value, or the spread operand
}

// Map is a map literal.
type Map struct{ Entries []Entry }

// Unary is ! or - applied to an operand.
type Unary struct {
	Op string
	X  Expr
}

// Binary is a binary operator. Op is written as in the source, "not in"
// included.
type Binary struct {
	Op   string
	X, Y Expr
}

// Cond is c ? a : b.
type Cond struct{ Test, Then, Else Expr }

// Branch is the braces of an if, a try body or a catch handler.
type Branch struct {
	Consts []*Const
	Result Expr
}

// If is if (c) {..} else if (d) {..} else {..}.
type If struct {
	Tests []Expr
	Then  []*Branch
	Else  *Branch
}

// Block is { const ...; e }, with at least one const line.
type Block struct {
	Consts []*Const
	Result Expr
}

// Try is try {..} catch (name) {..}. Name "" leaves out the parentheses.
type Try struct {
	Body    *Branch
	Name    string
	Handler *Branch
}

// Arm is one arm of a match. Lits are Num, Str, Bool, Null or Unary - over
// Num. Default is the arm _.
type Arm struct {
	Lits    []Expr
	Default bool
	Result  Expr
}

// Match is match subject { arms }.
type Match struct {
	Subject Expr
	Arms    []Arm
}

// Call is f(args) or ns.f(args).
type Call struct {
	NS, Name string
	Args     []Expr
}

// Member is x.name or x?.name.
type Member struct {
	X    Expr
	Name string
	Opt  bool
}

// Index is x[i] or x?.[i].
type Index struct {
	X, I Expr
	Opt  bool
}

// MethodCall is x.m(args) or x?.m(args).
type MethodCall struct {
	X    Expr
	Name string
	Args []Expr
	Opt  bool
}

// Param is one parameter of a lambda.
type Param struct {
	Pat     Pattern
	Default Expr
}

// Lambda is x => e, _ => e or (params) => e. Bare is the parameter of the
// form without parentheses; when it is "", Params is used.
type Lambda struct {
	Bare   string
	Params []Param
	Body   Expr
}

// As is x as T or x is T of package F4 (f04-conversion.md 2.7). Type is the
// program type, as text.
type As struct {
	Form string // "as" or "is"
	X    Expr
	Type string
}

// Paren is a pair of parentheses the printer would not need.
type Paren struct{ X Expr }

// Raw is text printed as it is. Loose programs use it for odd tokens.
type Raw struct{ Text string }

func (*Num) isExpr()        {}
func (*Str) isExpr()        {}
func (*Template) isExpr()   {}
func (*Bool) isExpr()       {}
func (*Null) isExpr()       {}
func (*Ident) isExpr()      {}
func (*List) isExpr()       {}
func (*Map) isExpr()        {}
func (*Unary) isExpr()      {}
func (*Binary) isExpr()     {}
func (*Cond) isExpr()       {}
func (*If) isExpr()         {}
func (*Block) isExpr()      {}
func (*Try) isExpr()        {}
func (*Match) isExpr()      {}
func (*Call) isExpr()       {}
func (*Member) isExpr()     {}
func (*Index) isExpr()      {}
func (*MethodCall) isExpr() {}
func (*Lambda) isExpr()     {}
func (*Paren) isExpr()      {}
func (*As) isExpr()         {}
func (*Raw) isExpr()        {}

// Pattern is a pattern of a const line or a parameter.
type Pattern interface{ isPat() }

// NamePat is a name, or _ when Name is "_".
type NamePat struct{ Name string }

// Field is one field of a map pattern. Key "" is the shorthand form, which
// binds Name under the key Name. KeyStr writes the key as a string.
type Field struct {
	Key     string
	KeyStr  bool
	Raw     bool // a string key written as a raw string, when it can be
	Name    string
	Default Expr
}

// MapPat is {a, b: x, c = 0, ...r}.
type MapPat struct {
	Fields []Field
	Rest   string
}

// Elem is one element of a list pattern. Name may be "_".
type Elem struct {
	Name    string
	Default Expr
}

// ListPat is [a, _, c = 0, ...r].
type ListPat struct {
	Elems []Elem
	Rest  string
}

func (*NamePat) isPat() {}
func (*MapPat) isPat()  {}
func (*ListPat) isPat() {}

// Const is const p = e;. Value may be a *Lambda when Pat is a NamePat.
type Const struct {
	Pat   Pattern
	Value Expr
}

// Program is the whole text.
type Program struct {
	Consts []*Const
	Result Expr
	Semi   bool
}
