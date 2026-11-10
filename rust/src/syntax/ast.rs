//! The syntax tree (`01-language.md` section 4).
//!
//! The tree is held in an arena: every node is an element of `Ast::nodes`,
//! and a node refers to its children by [`NodeId`]. A sum of 100,000 terms is
//! a left-deep tree 100,000 nodes deep (13.2 rule 6), and an arena drops it
//! without recursion. Walkers follow the same rule as the parser: what opens
//! no level (a left-associative chain, the steps of a chain, the `else` of
//! `? :` and of `if`, `const` lines) is walked by a loop.

use super::token::StrForm;
use crate::source::Span;

#[derive(Clone, Copy, Debug, Default, PartialEq, Eq, Hash, PartialOrd, Ord)]
pub struct NodeId(pub u32);

impl NodeId {
    pub fn index(self) -> usize {
        self.0 as usize
    }
}

#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub enum UnaryOp {
    Not,
    Neg,
}

impl UnaryOp {
    pub fn as_str(self) -> &'static str {
        match self {
            UnaryOp::Not => "!",
            UnaryOp::Neg => "-",
        }
    }
}

#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub enum BinaryOp {
    Or,
    And,
    Eq,
    Ne,
    Lt,
    Le,
    Gt,
    Ge,
    In,
    NotIn,
    Coalesce,
    Add,
    Sub,
    Mul,
    Div,
    Rem,
    Pow,
}

impl BinaryOp {
    /// The name of the operator as the node kinds table writes it (4.1).
    pub fn as_str(self) -> &'static str {
        match self {
            BinaryOp::Or => "||",
            BinaryOp::And => "&&",
            BinaryOp::Eq => "==",
            BinaryOp::Ne => "!=",
            BinaryOp::Lt => "<",
            BinaryOp::Le => "<=",
            BinaryOp::Gt => ">",
            BinaryOp::Ge => ">=",
            BinaryOp::In => "in",
            BinaryOp::NotIn => "not-in",
            BinaryOp::Coalesce => "??",
            BinaryOp::Add => "+",
            BinaryOp::Sub => "-",
            BinaryOp::Mul => "*",
            BinaryOp::Div => "/",
            BinaryOp::Rem => "%",
            BinaryOp::Pow => "**",
        }
    }
}

#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub enum CondForm {
    /// `c ? a : b`
    Ternary,
    /// `if (c) { a } else { b }`
    If,
}

/// One part of a `Template`.
#[derive(Clone, Debug, PartialEq)]
pub enum TemplatePart {
    Text(String),
    Expr(NodeId),
}

/// The key of an `Entry`.
#[derive(Clone, Debug, PartialEq)]
pub enum EntryKey {
    /// A bare word, or the name of a shorthand entry.
    Word(String),
    /// A one-line string.
    Str(String),
    /// `[e]`
    Computed(NodeId),
}

/// A literal of a `match` arm. A number written with `-` holds the negated
/// number (4.2).
#[derive(Clone, Debug, PartialEq)]
pub enum Lit {
    Number(f64),
    Str(String),
    Bool(bool),
    Null,
}

#[derive(Clone, Debug, PartialEq)]
pub struct ArmLit {
    pub lit: Lit,
    pub span: Span,
}

/// A name with its span, as a pattern's rest or a `catch` name holds it.
#[derive(Clone, Debug, PartialEq)]
pub struct SpannedName {
    pub name: String,
    pub span: Span,
}

/// A field of a `MapPat`: `{a}`, `{b: x}`, `{"a-b": x}`, `{c = 0}`.
#[derive(Clone, Debug, PartialEq)]
pub struct PatField {
    pub key: String,
    pub key_span: Span,
    pub name: SpannedName,
    pub default: Option<NodeId>,
    /// From the key, or the name, to the end of the default, or the name.
    pub span: Span,
}

/// An element of a `ListPat`.
#[derive(Clone, Debug, PartialEq)]
pub struct PatElem {
    /// A `NamePat`.
    pub pat: NodeId,
    pub default: Option<NodeId>,
    pub span: Span,
}

/// The 30 node kinds of Core (4.1). The comment on each gives its number.
#[derive(Clone, Debug, PartialEq)]
pub enum NodeKind {
    /// 1
    Program { consts: Vec<NodeId>, result: NodeId },
    /// 2
    Const { pattern: NodeId, value: NodeId },
    /// 3
    Block { consts: Vec<NodeId>, result: NodeId },
    /// 4
    Ident { name: String },
    /// 5
    Number { value: f64 },
    /// 6
    String { value: String, form: StrForm },
    /// 7
    Template { parts: Vec<TemplatePart> },
    /// 8
    Bool { value: bool },
    /// 9
    Null,
    /// 10
    List { items: Vec<NodeId> },
    /// 11
    Map { entries: Vec<NodeId> },
    /// 12
    Entry {
        key: EntryKey,
        /// The span of the key: the word, the string, or the `[` ... `]`.
        key_span: Span,
        value: NodeId,
        shorthand: bool,
    },
    /// 13
    Spread { value: NodeId },
    /// 14
    Unary { op: UnaryOp, x: NodeId },
    /// 15
    Binary { op: BinaryOp, x: NodeId, y: NodeId },
    /// 16
    Cond {
        form: CondForm,
        test: NodeId,
        then: NodeId,
        else_: NodeId,
    },
    /// 17
    Member {
        x: NodeId,
        name: String,
        name_span: Span,
        optional: bool,
    },
    /// 18
    Index {
        x: NodeId,
        index: NodeId,
        optional: bool,
    },
    /// Kind 19. The parser always gives `namespace: None`; name resolution turns a
    /// `MethodCall` on a namespace into a `Call` (4.2).
    Call {
        namespace: Option<SpannedName>,
        name: SpannedName,
        args: Vec<NodeId>,
    },
    /// 20
    MethodCall {
        x: NodeId,
        name: SpannedName,
        args: Vec<NodeId>,
        optional: bool,
    },
    /// 21
    OptChain { x: NodeId },
    /// 22
    Lambda { params: Vec<NodeId>, body: NodeId },
    /// 23
    Param {
        pattern: NodeId,
        default: Option<NodeId>,
    },
    /// 24
    Match { subject: NodeId, arms: Vec<NodeId> },
    /// Kind 25. `literals` is `None` for the default arm `_`.
    Arm {
        literals: Option<Vec<ArmLit>>,
        result: NodeId,
    },
    /// 26
    Try {
        body: NodeId,
        name: Option<SpannedName>,
        handler: NodeId,
    },
    /// Kind 27. `name` is `None` for the discard `_`.
    NamePat { name: Option<String> },
    /// 28
    MapPat {
        fields: Vec<PatField>,
        rest: Option<SpannedName>,
    },
    /// 29
    ListPat {
        elems: Vec<PatElem>,
        rest: Option<SpannedName>,
    },
    /// Kind 30. The parser stops at its first diagnostic and never builds a tree
    /// that holds `Bad`; the kind exists so that the numbering is whole.
    Bad,
}

impl NodeKind {
    /// The number of the kind (4.1).
    pub fn number(&self) -> u8 {
        match self {
            NodeKind::Program { .. } => 1,
            NodeKind::Const { .. } => 2,
            NodeKind::Block { .. } => 3,
            NodeKind::Ident { .. } => 4,
            NodeKind::Number { .. } => 5,
            NodeKind::String { .. } => 6,
            NodeKind::Template { .. } => 7,
            NodeKind::Bool { .. } => 8,
            NodeKind::Null => 9,
            NodeKind::List { .. } => 10,
            NodeKind::Map { .. } => 11,
            NodeKind::Entry { .. } => 12,
            NodeKind::Spread { .. } => 13,
            NodeKind::Unary { .. } => 14,
            NodeKind::Binary { .. } => 15,
            NodeKind::Cond { .. } => 16,
            NodeKind::Member { .. } => 17,
            NodeKind::Index { .. } => 18,
            NodeKind::Call { .. } => 19,
            NodeKind::MethodCall { .. } => 20,
            NodeKind::OptChain { .. } => 21,
            NodeKind::Lambda { .. } => 22,
            NodeKind::Param { .. } => 23,
            NodeKind::Match { .. } => 24,
            NodeKind::Arm { .. } => 25,
            NodeKind::Try { .. } => 26,
            NodeKind::NamePat { .. } => 27,
            NodeKind::MapPat { .. } => 28,
            NodeKind::ListPat { .. } => 29,
            NodeKind::Bad => 30,
        }
    }

    /// The name of the kind (4.1).
    pub fn name(&self) -> &'static str {
        KIND_NAMES[self.number() as usize - 1]
    }
}

pub const KIND_NAMES: [&str; 30] = [
    "Program",
    "Const",
    "Block",
    "Ident",
    "Number",
    "String",
    "Template",
    "Bool",
    "Null",
    "List",
    "Map",
    "Entry",
    "Spread",
    "Unary",
    "Binary",
    "Cond",
    "Member",
    "Index",
    "Call",
    "MethodCall",
    "OptChain",
    "Lambda",
    "Param",
    "Match",
    "Arm",
    "Try",
    "NamePat",
    "MapPat",
    "ListPat",
    "Bad",
];

#[derive(Clone, Debug, PartialEq)]
pub struct Node {
    pub kind: NodeKind,
    /// The span of 4.3.
    pub span: Span,
    /// The span with the parentheses written around the node, if any. Equal
    /// to `span` when there are none. A node that holds this one starts or
    /// ends at `outer` (4.3 rule 1).
    pub outer: Span,
    /// The levels open at the node's first token (`01-language.md` 13.2
    /// rule 5): its nesting depth.
    pub level: u16,
}

impl Node {
    /// Whether the node was written in parentheses.
    pub fn parenthesized(&self) -> bool {
        self.outer != self.span
    }
}

/// A parsed program: the arena and its root, which is a `Program` node.
#[derive(Clone, Debug, Default)]
pub struct Ast {
    pub nodes: Vec<Node>,
    pub root: NodeId,
}

impl Ast {
    pub fn node(&self, id: NodeId) -> &Node {
        &self.nodes[id.index()]
    }

    pub fn kind(&self, id: NodeId) -> &NodeKind {
        &self.nodes[id.index()].kind
    }

    pub fn span(&self, id: NodeId) -> Span {
        self.nodes[id.index()].span
    }

    pub(crate) fn push(&mut self, kind: NodeKind, span: Span, level: u16) -> NodeId {
        let id = NodeId(self.nodes.len() as u32);
        self.nodes.push(Node {
            kind,
            span,
            outer: span,
            level,
        });
        id
    }
}
