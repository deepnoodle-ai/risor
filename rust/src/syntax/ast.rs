//! The syntax tree (`01-language.md` section 4).
//!
//! The tree is held in an arena: every node is an element of `Ast::nodes`,
//! and a node refers to its children by [`NodeId`]. A sum of 100,000 terms is
//! a left-deep tree 100,000 nodes deep (13.2 rule 6), and an arena drops it
//! without recursion. Walkers follow the same rule as the parser: what opens
//! no level (a left-associative chain, the steps of a chain, the `else` of
//! `? :` and of `if`, `const` lines) is walked by a loop.

use super::token::StrForm;
use crate::source::Range;
use std::sync::Arc;

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
    pub span: Range,
}

/// A name with its span, as a pattern's rest or a `catch` name holds it.
#[derive(Clone, Debug, PartialEq)]
pub struct SpannedName {
    pub name: String,
    pub span: Range,
}

/// A field of a `MapPat`: `{a}`, `{b: x}`, `{"a-b": x}`, `{c = 0}`.
#[derive(Clone, Debug, PartialEq)]
pub struct PatField {
    pub key: String,
    pub key_span: Range,
    pub name: SpannedName,
    pub default: Option<NodeId>,
    /// From the key, or the name, to the end of the default, or the name.
    pub span: Range,
}

/// An element of a `ListPat`.
#[derive(Clone, Debug, PartialEq)]
pub struct PatElem {
    /// A `NamePat`.
    pub pat: NodeId,
    pub default: Option<NodeId>,
    pub span: Range,
}

/// The 30 node kinds of Core (4.1). The comment on each gives its number.
#[derive(Clone, Debug, PartialEq)]
pub enum NodeKind {
    /// Kind 1. `types` are the type lines of package F16, in the order of
    /// the text (`f16-entry-points.md` 2.3); they are not nodes. `result` may
    /// be `None` when the program has at least one `export const` line
    /// (`f16-entry-points.md` 12.2).
    Program {
        consts: Vec<NodeId>,
        result: Option<NodeId>,
        types: Vec<TypeLine>,
    },
    /// Kind 2. `ty` is the annotation of package F16, or none.
    Const {
        pattern: NodeId,
        value: NodeId,
        ty: Option<Box<PType>>,
        /// F16 11.3: its description, or none.
        doc: Option<Arc<str>>,
        /// F16 12.2: `true` for `export const`, `false` for plain `const`.
        exported: bool,
    },
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
        key_span: Range,
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
        name_span: Range,
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
    /// Kind 22. `result` is the result annotation of package F16, or none.
    Lambda {
        params: Vec<NodeId>,
        body: NodeId,
        result: Option<Box<PType>>,
    },
    /// Kind 23. `ty` and `fn_` are the annotation of package F16, a program
    /// type or a function type; at most one of them is set.
    Param {
        pattern: NodeId,
        default: Option<NodeId>,
        ty: Option<Box<PType>>,
        fn_: Option<Box<PFn>>,
        /// F16 11.3: its description, or none.
        doc: Option<Arc<str>>,
    },
    /// 24
    Match { subject: NodeId, arms: Vec<NodeId> },
    /// Kind 25. `literals` is `None` for the default arm `_`. `guard` is
    /// the expression of `if (e)`, package F17 (`f17-recursion.md` 2.2).
    Arm {
        literals: Option<Vec<ArmLit>>,
        guard: Option<NodeId>,
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
    #[allow(dead_code)]
    Bad,
    /// Kind 31, package F4: `x as T` and `x is T`. `ty` is the program type,
    /// as written; the compile resolves it.
    As {
        form: AsForm,
        x: NodeId,
        ty: Box<PType>,
    },
    /// Kind 32, package F7: a projection (`f07-queries.md` 2.3). `x` is the
    /// chain before the first star, or none for a later star; `reads` are
    /// the steps of its level, a later star last; `star` is the star span.
    Each {
        x: Option<NodeId>,
        reads: Vec<NodeId>,
        optional: bool,
        star: Range,
    },
    /// Not a node of the language: the element in hand, the `x` of the first
    /// read of a level of a projection.
    Elem,
}

/// The form of an `As` node (F4 2.7).
#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub enum AsForm {
    As,
    Is,
}

/// A program type (F4 2.1): `PType = PAlt { "|" PAlt }`, with its span from
/// its first token to its last.
#[derive(Clone, Debug, PartialEq)]
pub struct PType {
    pub alts: Vec<PAlt>,
    pub span: Range,
}

/// `PAlt = PPrim { "[" "]" }`.
#[derive(Clone, Debug, PartialEq)]
pub struct PAlt {
    pub prim: PPrim,
    pub suffixes: u32,
    pub span: Range,
}

#[derive(Clone, Debug, PartialEq)]
pub enum PPrim {
    Null,
    Bool,
    Int,
    Float,
    String,
    Any,
    Never,
    Lit(String),
    Map(Box<PType>),
    Record {
        fields: Vec<PField>,
        /// `None`: exact. `Some(None)`: `...`. `Some(Some(T))`: `...T`.
        rest: Option<Option<Box<PType>>>,
    },
    /// A type name, with its span.
    Name(String, Range),
    Paren(Box<PType>),
}

/// A type line of package F16 (`f16-entry-points.md` 2.3): `type Name =
/// T;`, with the span of its name and its own span, from `type` to `;`.
#[derive(Clone, Debug, PartialEq)]
pub struct TypeLine {
    pub name: String,
    pub name_span: Range,
    pub ty: PType,
    pub span: Range,
    /// F16 11.3: its description, or none.
    pub doc: Option<Arc<str>>,
}

/// A function type on a parameter, package F16 (2.1, `PFn`): its parameters,
/// each a word and a program type, and its result, `None` for `unknown`.
/// The span runs from the `(` to the end of the result.
#[derive(Clone, Debug, PartialEq)]
pub struct PFn {
    pub params: Vec<(String, Range, PType)>,
    pub result: Option<PType>,
    pub result_span: Range,
    pub span: Range,
}

#[derive(Clone, Debug, PartialEq)]
pub struct PField {
    pub key: String,
    pub optional: bool,
    pub ty: PType,
    /// F16 11.3: the field's description, or none.
    pub doc: Option<Arc<str>>,
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
            NodeKind::As { .. } => 31,
            NodeKind::Each { .. } => 32,
            NodeKind::Elem => 33,
        }
    }

    /// The name of the kind (4.1).
    pub fn name(&self) -> &'static str {
        KIND_NAMES[self.number() as usize - 1]
    }
}

pub const KIND_NAMES: [&str; 33] = [
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
    "As",
    "Each",
    "Elem",
];

#[derive(Clone, Debug, PartialEq)]
pub struct Node {
    pub kind: NodeKind,
    /// The span of 4.3.
    pub span: Range,
    /// The span with the parentheses written around the node, if any. Equal
    /// to `span` when there are none. A node that holds this one starts or
    /// ends at `outer` (4.3 rule 1).
    pub outer: Range,
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
    /// For each `Lambda`, the greatest number of levels open at once inside
    /// its text, parameter list included (`05-types.md` 20.2).
    pub lambda_levels: std::collections::HashMap<u32, u16>,
    /// The highest level of nesting (13.1) the parse reached, for the
    /// formatter (F12 2.6 item 4).
    pub(crate) deepest: u16,
}

impl Ast {
    pub fn node(&self, id: NodeId) -> &Node {
        &self.nodes[id.index()]
    }

    pub fn kind(&self, id: NodeId) -> &NodeKind {
        &self.nodes[id.index()].kind
    }

    pub fn span(&self, id: NodeId) -> Range {
        self.nodes[id.index()].span
    }

    pub(crate) fn push(&mut self, kind: NodeKind, span: Range, level: u16) -> NodeId {
        let id = NodeId(self.nodes.len() as u32);
        self.nodes.push(Node {
            kind,
            span,
            outer: span,
            level,
        });
        id
    }

    /// For each node, whether it is the last step of a coalesced read under
    /// `??` (`01-language.md` 6.6): a `Member` or `Index` node that gives
    /// null, not `missing-field`, for an absent key. The rule reads the
    /// nodes only, so the checker and a run agree.
    pub fn coalesced_reads(&self) -> Vec<bool> {
        let n = self.nodes.len();
        let is_coalesce = |k: &NodeKind| {
            matches!(
                k,
                NodeKind::Binary {
                    op: BinaryOp::Coalesce,
                    ..
                }
            )
        };
        // The `??` nodes that are an operand of a `??` node: each belongs
        // to the chain of the node above it.
        let mut operand = vec![false; n];
        for node in &self.nodes {
            if let NodeKind::Binary {
                op: BinaryOp::Coalesce,
                x,
                y,
            } = &node.kind
            {
                operand[x.index()] = true;
                operand[y.index()] = true;
            }
        }
        let mut out = vec![false; n];
        let mut stack = Vec::new();
        let mut operands = Vec::new();
        for (i, node) in self.nodes.iter().enumerate() {
            if !is_coalesce(&node.kind) || operand[i] {
                continue;
            }
            // The operands of the chain in source order, by a loop: a long
            // chain opens no level.
            operands.clear();
            stack.push(NodeId(i as u32));
            while let Some(id) = stack.pop() {
                match self.kind(id) {
                    NodeKind::Binary {
                        op: BinaryOp::Coalesce,
                        x,
                        y,
                    } => {
                        stack.push(*y);
                        stack.push(*x);
                    }
                    _ => operands.push(id),
                }
            }
            // Every operand but the last.
            operands.pop();
            for &o in &operands {
                if let Some(s) = self.coalesced_step(o) {
                    out[s.index()] = true;
                }
            }
        }
        out
    }

    /// The last step of an operand of `??` when the operand is a coalesced
    /// read: a plain `Member`, or a plain `Index` by a `String` node, with
    /// or without an `OptChain` around it.
    fn coalesced_step(&self, id: NodeId) -> Option<NodeId> {
        let id = match self.kind(id) {
            NodeKind::OptChain { x } => *x,
            _ => id,
        };
        match self.kind(id) {
            NodeKind::Member {
                optional: false, ..
            } => Some(id),
            NodeKind::Index {
                index,
                optional: false,
                ..
            } if matches!(self.kind(*index), NodeKind::String { .. }) => Some(id),
            _ => None,
        }
    }
}

impl Ast {
    /// The arena of a program whose result is the node `top`: the nodes
    /// under `top`, in their order here, under a `Program` node of its own,
    /// with the spans of the whole text. An argument of a call in an
    /// analysis (F2 3.6 item 5) and a part of a template (F2 2.5) are each
    /// such a program.
    ///
    /// Only the nodes under `top` are copied, so the programs of many parts
    /// of one tree cost the size of the parts, not the whole tree once
    /// each. The arguments of a call are such parts. A node is copied once
    /// for each call whose arguments hold it, and an argument list opens a
    /// level (`01-language.md` 13.1), so no node is copied more than 100
    /// times.
    pub(crate) fn program_of(&self, top: NodeId) -> Ast {
        // The nodes under `top`, found by an explicit stack.
        let mut seen: std::collections::HashSet<u32> = std::collections::HashSet::from([top.0]);
        let mut stack = vec![top];
        let mut nodes: Vec<(NodeId, Node)> = Vec::new();
        while let Some(id) = stack.pop() {
            let mut node = self.node(id).clone();
            node_ids(&mut node.kind, &mut |c| {
                if seen.insert(c.0) {
                    stack.push(*c);
                }
            });
            nodes.push((id, node));
        }
        nodes.sort_by_key(|(id, _)| *id);
        let new_id: std::collections::HashMap<u32, NodeId> = nodes
            .iter()
            .enumerate()
            .map(|(i, (id, _))| (id.0, NodeId(i as u32)))
            .collect();
        let mut ast = Ast::default();
        for (old, mut node) in nodes {
            node_ids(&mut node.kind, &mut |c| *c = new_id[&c.0]);
            if let Some(&l) = self.lambda_levels.get(&old.0) {
                ast.lambda_levels.insert(new_id[&old.0].0, l);
            }
            ast.nodes.push(node);
        }
        let root = ast.push(
            NodeKind::Program {
                consts: Vec::new(),
                result: Some(new_id[&top.0]),
                types: Vec::new(),
            },
            self.span(top),
            0,
        );
        ast.root = root;
        ast
    }
}

/// Each node that a node holds. Every field is named, so a new field is
/// a compile error here and not a node left out of [`Ast::program_of`].
fn node_ids(kind: &mut NodeKind, f: &mut dyn FnMut(&mut NodeId)) {
    match kind {
        NodeKind::Program {
            consts,
            result,
            types: _,
        } => {
            consts.iter_mut().for_each(&mut *f);
            if let Some(r) = result {
                f(r);
            }
        }
        NodeKind::Block { consts, result } => {
            consts.iter_mut().for_each(&mut *f);
            f(result);
        }
        NodeKind::Const {
            pattern,
            value,
            ty: _,
            doc: _,
            exported: _,
        } => {
            f(pattern);
            f(value);
        }
        NodeKind::Ident { name: _ }
        | NodeKind::Number { value: _ }
        | NodeKind::String { value: _, form: _ }
        | NodeKind::Bool { value: _ }
        | NodeKind::Null
        | NodeKind::NamePat { name: _ }
        | NodeKind::Bad
        | NodeKind::Elem => {}
        NodeKind::Template { parts } => {
            for p in parts {
                match p {
                    TemplatePart::Expr(e) => f(e),
                    TemplatePart::Text(_) => {}
                }
            }
        }
        NodeKind::List { items } => items.iter_mut().for_each(f),
        NodeKind::Map { entries } => entries.iter_mut().for_each(f),
        NodeKind::Entry {
            key,
            key_span: _,
            value,
            shorthand: _,
        } => {
            match key {
                EntryKey::Computed(k) => f(k),
                EntryKey::Word(_) | EntryKey::Str(_) => {}
            }
            f(value);
        }
        NodeKind::Spread { value } => f(value),
        NodeKind::Unary { op: _, x } | NodeKind::OptChain { x } => f(x),
        NodeKind::Binary { op: _, x, y } => {
            f(x);
            f(y);
        }
        NodeKind::Cond {
            form: _,
            test,
            then,
            else_,
        } => {
            f(test);
            f(then);
            f(else_);
        }
        NodeKind::Member {
            x,
            name: _,
            name_span: _,
            optional: _,
        } => f(x),
        NodeKind::Index {
            x,
            index,
            optional: _,
        } => {
            f(x);
            f(index);
        }
        NodeKind::Call {
            namespace: _,
            name: _,
            args,
        } => args.iter_mut().for_each(f),
        NodeKind::MethodCall {
            x,
            name: _,
            args,
            optional: _,
        } => {
            f(x);
            args.iter_mut().for_each(f);
        }
        NodeKind::Lambda {
            params,
            body,
            result: _,
        } => {
            params.iter_mut().for_each(&mut *f);
            f(body);
        }
        NodeKind::Param {
            pattern,
            default,
            ty: _,
            fn_: _,
            doc: _,
        } => {
            f(pattern);
            default.iter_mut().for_each(f);
        }
        NodeKind::Match { subject, arms } => {
            f(subject);
            arms.iter_mut().for_each(f);
        }
        NodeKind::Arm {
            literals: _,
            guard,
            result,
        } => {
            guard.iter_mut().for_each(&mut *f);
            f(result);
        }
        NodeKind::Try {
            body,
            name: _,
            handler,
        } => {
            f(body);
            f(handler);
        }
        NodeKind::MapPat { fields, rest: _ } => {
            for PatField {
                key: _,
                key_span: _,
                name: _,
                default,
                span: _,
            } in fields
            {
                default.iter_mut().for_each(&mut *f);
            }
        }
        NodeKind::ListPat { elems, rest: _ } => {
            for PatElem {
                pat,
                default,
                span: _,
            } in elems
            {
                f(pat);
                default.iter_mut().for_each(&mut *f);
            }
        }
        NodeKind::As { form: _, x, ty: _ } => f(x),
        NodeKind::Each {
            x,
            reads,
            optional: _,
            star: _,
        } => {
            x.iter_mut().for_each(&mut *f);
            reads.iter_mut().for_each(f);
        }
    }
}
