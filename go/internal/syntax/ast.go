package syntax

// NodeKind is the number of a node kind (01-language.md 4.1). The numbers are
// fixed by the specification.
type NodeKind uint8

// The 30 node kinds of Core.
const (
	KindProgram    NodeKind = 1
	KindConst      NodeKind = 2
	KindBlock      NodeKind = 3
	KindIdent      NodeKind = 4
	KindNumber     NodeKind = 5
	KindString     NodeKind = 6
	KindTemplate   NodeKind = 7
	KindBool       NodeKind = 8
	KindNull       NodeKind = 9
	KindList       NodeKind = 10
	KindMap        NodeKind = 11
	KindEntry      NodeKind = 12
	KindSpread     NodeKind = 13
	KindUnary      NodeKind = 14
	KindBinary     NodeKind = 15
	KindCond       NodeKind = 16
	KindMember     NodeKind = 17
	KindIndex      NodeKind = 18
	KindCall       NodeKind = 19
	KindMethodCall NodeKind = 20
	KindOptChain   NodeKind = 21
	KindLambda     NodeKind = 22
	KindParam      NodeKind = 23
	KindMatch      NodeKind = 24
	KindArm        NodeKind = 25
	KindTry        NodeKind = 26
	KindNamePat    NodeKind = 27
	KindMapPat     NodeKind = 28
	KindListPat    NodeKind = 29
	KindBad        NodeKind = 30
	// KindAs is node 31 of package F4: x as T, x is T.
	KindAs NodeKind = 31
	// KindEach is node 32 of package F7: a projection.
	KindEach NodeKind = 32
)

var nodeNames = [...]string{"", "Program", "Const", "Block", "Ident", "Number", "String",
	"Template", "Bool", "Null", "List", "Map", "Entry", "Spread", "Unary", "Binary",
	"Cond", "Member", "Index", "Call", "MethodCall", "OptChain", "Lambda", "Param",
	"Match", "Arm", "Try", "NamePat", "MapPat", "ListPat", "Bad", "As", "Each"}

// String returns the node's name, such as "Binary".
func (k NodeKind) String() string {
	if int(k) < len(nodeNames) {
		return nodeNames[k]
	}
	return "NodeKind(?)"
}

// Span is a span of source text as code point offsets. End is exclusive.
type Span struct {
	Start int
	End   int
}

// Node is a node of the syntax tree.
type Node interface {
	Kind() NodeKind
	// Span is the node's span by 01-language.md 4.3.
	Span() Span
	// Depth is the nesting depth of the node: the number of levels open at
	// its first token (01-language.md 13.2 rule 5).
	Depth() int
}

// Pattern is a node that stands where a pattern stands: *NamePat, *MapPat or
// *ListPat.
type Pattern interface {
	Node
	pattern()
}

type base struct {
	span  Span
	depth int
}

// Span returns the node's span.
func (b *base) Span() Span { return b.span }

// Depth returns the levels of nesting open at the node's first token.
func (b *base) Depth() int { return b.depth }

// Program is a whole text: const lines and one expression.
type Program struct {
	base
	Consts []*Const
	Result Node
}

// Const is "const pattern = value;". Value is a *Lambda for a const-bound
// lambda.
type Const struct {
	base
	Pattern Pattern
	Value   Node
}

// Block is "{ const ...; e }", or a branch that has const lines. It has at
// least one Const.
type Block struct {
	base
	Consts []*Const
	Result Node
}

// Ident reads a name.
type Ident struct {
	base
	Name string
}

// Number is a number literal, or a match literal with its "-".
type Number struct {
	base
	Value float64
}

// String is a one-line string, or a template with no interpolation.
type String struct {
	base
	Value string
	// Form is '"', '\'' or '`'. Evaluation never reads it.
	Form rune
}

// TemplatePart is a text part (Expr nil) or an interpolation.
type TemplatePart struct {
	Text string
	Expr Node
}

// Template is a template with at least one interpolation. Its parts are in
// written order; the text parts may be empty.
type Template struct {
	base
	Parts []TemplatePart
}

// Bool is true or false.
type Bool struct {
	base
	Value bool
}

// Null is null.
type Null struct {
	base
}

// List is "[a, ...b]". Each item is an expression or a *Spread.
type List struct {
	base
	Items []Node
}

// Map is "{a: 1, ...m}". Each entry is an *Entry or a *Spread.
type Map struct {
	base
	Entries []Node
}

// KeyForm is how the key of an Entry or of a map pattern field is written.
type KeyForm uint8

// The forms of a key.
const (
	KeyWord     KeyForm = iota // a: 1, and a shorthand entry
	KeyString                  // "a-b": 1
	KeyComputed                // [k]: 1
)

// Entry is one entry of a map literal.
type Entry struct {
	base
	KeyForm KeyForm
	// Key is the text of the key for KeyWord and KeyString.
	Key string
	// KeySpan is the span of the key as written, for KeyWord and KeyString.
	KeySpan Span
	// KeyExpr is the expression of a computed key.
	KeyExpr   Node
	Value     Node
	Shorthand bool
}

// Spread is "...e" in a list or a map.
type Spread struct {
	base
	Value Node
}

// Unary is "!x" or "-x".
type Unary struct {
	base
	Op string // "!" or "-"
	X  Node
}

// Binary is a binary operator. Op is one of
// || && == != < <= > >= in not-in ?? + - * / % **.
type Binary struct {
	base
	Op    string
	OpPos Span // the operator token, "not in" whole
	X, Y  Node
}

// Cond is "c ? a : b" (Form "?:") or "if (c) { a } else { b }" (Form "if").
type Cond struct {
	base
	Form string
	Test Node
	Then Node
	Else Node
}

// Member is "x.f" or "x?.f".
type Member struct {
	base
	X        Node
	Name     string
	NameSpan Span
	Optional bool
}

// Index is "x[i]" or "x?.[i]".
type Index struct {
	base
	X        Node
	Index    Node
	Optional bool
}

// Call is "f(a)" or, after name resolution, "ns.f(a)". The parser never sets
// Namespace.
type Call struct {
	base
	Namespace string
	Name      string
	NameSpan  Span
	Args      []Node
}

// MethodCall is "x.m(a)" or "x?.m(a)".
type MethodCall struct {
	base
	X        Node
	Name     string
	NameSpan Span
	Args     []Node
	Optional bool
}

// OptChain wraps the outermost node of a chain that has an optional step.
type OptChain struct {
	base
	X Node
}

// Lambda is "x => e", "_ => e" or "(p, q) => e".
type Lambda struct {
	base
	Params []*Param
	Body   Node
}

// Param is one parameter of a lambda.
type Param struct {
	base
	Pattern Pattern
	Default Node // nil when there is none
}

// Match is "match subject { arms }".
type Match struct {
	base
	Subject Node
	Arms    []*Arm
}

// Arm is one arm of a match: literals joined by "|", or the default "_".
type Arm struct {
	base
	// Literals are *Number, *String, *Bool and *Null nodes. Empty for the
	// default arm.
	Literals []Node
	Default  bool
	Result   Node
}

// Try is "try { e } catch (name) { d }" or "try { e } catch { d }".
type Try struct {
	base
	Body     Node
	Name     string // empty when there is no name
	NameSpan Span
	Handler  Node
}

// NamePat is a name pattern, or the discard "_".
type NamePat struct {
	base
	Name    string
	Discard bool
}

// MapField is one field of a map pattern.
type MapField struct {
	Span    Span
	KeyForm KeyForm // KeyWord or KeyString
	Key     string
	// KeySpan is the span of the key as written: the name for {a}.
	KeySpan Span
	Name    string
	// NameSpan is the span of the bound name.
	NameSpan Span
	Default  Node
}

// MapPat is "{a, b: x, c = 0, ...r}".
type MapPat struct {
	base
	Fields []*MapField
	Rest   *NamePat // nil when there is none
}

// ListElem is one element of a list pattern.
type ListElem struct {
	Span    Span
	Name    *NamePat
	Default Node
}

// ListPat is "[a, _, c = 0, ...r]".
type ListPat struct {
	base
	Elems []*ListElem
	Rest  *NamePat // nil when there is none
}

// Bad stands for text after a parse diagnostic.
type Bad struct {
	base
}

// Kind returns KindProgram.
func (*Program) Kind() NodeKind { return KindProgram }

// Kind returns KindConst.
func (*Const) Kind() NodeKind { return KindConst }

// Kind returns KindBlock.
func (*Block) Kind() NodeKind { return KindBlock }

// Kind returns KindIdent.
func (*Ident) Kind() NodeKind { return KindIdent }

// Kind returns KindNumber.
func (*Number) Kind() NodeKind { return KindNumber }

// Kind returns KindString.
func (*String) Kind() NodeKind { return KindString }

// Kind returns KindTemplate.
func (*Template) Kind() NodeKind { return KindTemplate }

// Kind returns KindBool.
func (*Bool) Kind() NodeKind { return KindBool }

// Kind returns KindNull.
func (*Null) Kind() NodeKind { return KindNull }

// Kind returns KindList.
func (*List) Kind() NodeKind { return KindList }

// Kind returns KindMap.
func (*Map) Kind() NodeKind { return KindMap }

// Kind returns KindEntry.
func (*Entry) Kind() NodeKind { return KindEntry }

// Kind returns KindSpread.
func (*Spread) Kind() NodeKind { return KindSpread }

// Kind returns KindUnary.
func (*Unary) Kind() NodeKind { return KindUnary }

// Kind returns KindBinary.
func (*Binary) Kind() NodeKind { return KindBinary }

// Kind returns KindCond.
func (*Cond) Kind() NodeKind { return KindCond }

// Kind returns KindMember.
func (*Member) Kind() NodeKind { return KindMember }

// Kind returns KindIndex.
func (*Index) Kind() NodeKind { return KindIndex }

// Kind returns KindCall.
func (*Call) Kind() NodeKind { return KindCall }

// Kind returns KindMethodCall.
func (*MethodCall) Kind() NodeKind { return KindMethodCall }

// Kind returns KindOptChain.
func (*OptChain) Kind() NodeKind { return KindOptChain }

// Kind returns KindLambda.
func (*Lambda) Kind() NodeKind { return KindLambda }

// Kind returns KindParam.
func (*Param) Kind() NodeKind { return KindParam }

// Kind returns KindMatch.
func (*Match) Kind() NodeKind { return KindMatch }

// Kind returns KindArm.
func (*Arm) Kind() NodeKind { return KindArm }

// Kind returns KindTry.
func (*Try) Kind() NodeKind { return KindTry }

// Kind returns KindNamePat.
func (*NamePat) Kind() NodeKind { return KindNamePat }

// Kind returns KindMapPat.
func (*MapPat) Kind() NodeKind { return KindMapPat }

// Kind returns KindListPat.
func (*ListPat) Kind() NodeKind { return KindListPat }

// As is "x as T" or "x is T" (package F4 2.7). Type is the program type;
// it is not a node and nothing in it is evaluated.
type As struct {
	base
	Form string // "as" or "is"
	X    Node
	Type *PType
}

// Each is a projection of package F7 (f07-queries.md 2.3): xs[*].f. X is
// the chain before the first star, nil for a later star. Reads are the
// steps of its level: Member and Index nodes with no X, and a later star as
// the last. StarSpan is the step span of its own star.
type Each struct {
	base
	X        Node
	Reads    []Node
	Optional bool
	StarSpan Span
}

// Kind returns KindEach.
func (*Each) Kind() NodeKind { return KindEach }

// Kind returns KindAs.
func (*As) Kind() NodeKind { return KindAs }

// PType is a program type (F4 2.1): alternatives joined by "|".
type PType struct {
	Span Span
	Alts []*PAlt
}

// PAlt is one alternative: a primary and its count of [] suffixes.
type PAlt struct {
	Span  Span
	Prim  *PPrim
	Lists int
}

// PPrimKind is the form of a PPrim.
type PPrimKind uint8

// The forms of a PPrim.
const (
	PWord   PPrimKind = iota // null, bool, int, float, string, any, never
	PString                  // a literal set of one string
	PMap                     // map<T>
	PRecord                  // {f: T, ...}
	PName                    // a type name
	PParen                   // (T)
)

// PPrim is a primary of a program type.
type PPrim struct {
	Span Span
	Kind PPrimKind
	// Text is the word, the string's value or the name.
	Text string
	// Elem is the type of map<T> or (T).
	Elem *PType
	// Fields, HasRest and Rest are a record's.
	Fields  []*PField
	HasRest bool
	Rest    *PType // nil for "..." alone
}

// PField is one field of a PRecord.
type PField struct {
	Key      string
	KeySpan  Span
	Optional bool
	Type     *PType
}

// Kind returns KindBad.
func (*Bad) Kind() NodeKind { return KindBad }

func (*NamePat) pattern() {}
func (*MapPat) pattern()  {}
func (*ListPat) pattern() {}
