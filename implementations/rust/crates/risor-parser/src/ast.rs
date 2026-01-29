//! AST node types for the Risor parser.

use risor_lexer::Position;
use std::fmt;
use std::rc::Rc;

/// Base trait for all AST nodes.
pub trait Node: fmt::Display {
    fn pos(&self) -> Position;
    fn end(&self) -> Position;
}

// ============================================================================
// Expressions
// ============================================================================

/// Expression node enumeration.
#[derive(Debug, Clone, PartialEq)]
pub enum Expr {
    Int(IntLit),
    Float(FloatLit),
    Bool(BoolLit),
    Nil(NilLit),
    String(StringLit),
    Ident(Ident),
    Prefix(Box<PrefixExpr>),
    Infix(Box<InfixExpr>),
    Spread(Box<SpreadExpr>),
    List(ListLit),
    Map(MapLit),
    Func(Rc<FuncLit>),
    Call(Box<CallExpr>),
    GetAttr(Box<GetAttrExpr>),
    ObjectCall(Box<ObjectCallExpr>),
    Index(Box<IndexExpr>),
    Slice(Box<SliceExpr>),
    If(Box<IfExpr>),
    Switch(Box<SwitchExpr>),
    Match(Box<MatchExpr>),
    In(Box<InExpr>),
    NotIn(Box<NotInExpr>),
    Pipe(PipeExpr),
    Try(Box<TryExpr>),
}

impl Node for Expr {
    fn pos(&self) -> Position {
        match self {
            Expr::Int(e) => e.pos(),
            Expr::Float(e) => e.pos(),
            Expr::Bool(e) => e.pos(),
            Expr::Nil(e) => e.pos(),
            Expr::String(e) => e.pos(),
            Expr::Ident(e) => e.pos(),
            Expr::Prefix(e) => e.pos(),
            Expr::Infix(e) => e.pos(),
            Expr::Spread(e) => e.pos(),
            Expr::List(e) => e.pos(),
            Expr::Map(e) => e.pos(),
            Expr::Func(e) => e.pos(),
            Expr::Call(e) => e.pos(),
            Expr::GetAttr(e) => e.pos(),
            Expr::ObjectCall(e) => e.pos(),
            Expr::Index(e) => e.pos(),
            Expr::Slice(e) => e.pos(),
            Expr::If(e) => e.pos(),
            Expr::Switch(e) => e.pos(),
            Expr::Match(e) => e.pos(),
            Expr::In(e) => e.pos(),
            Expr::NotIn(e) => e.pos(),
            Expr::Pipe(e) => e.pos(),
            Expr::Try(e) => e.pos(),
        }
    }

    fn end(&self) -> Position {
        match self {
            Expr::Int(e) => e.end(),
            Expr::Float(e) => e.end(),
            Expr::Bool(e) => e.end(),
            Expr::Nil(e) => e.end(),
            Expr::String(e) => e.end(),
            Expr::Ident(e) => e.end(),
            Expr::Prefix(e) => e.end(),
            Expr::Infix(e) => e.end(),
            Expr::Spread(e) => e.end(),
            Expr::List(e) => e.end(),
            Expr::Map(e) => e.end(),
            Expr::Func(e) => e.end(),
            Expr::Call(e) => e.end(),
            Expr::GetAttr(e) => e.end(),
            Expr::ObjectCall(e) => e.end(),
            Expr::Index(e) => e.end(),
            Expr::Slice(e) => e.end(),
            Expr::If(e) => e.end(),
            Expr::Switch(e) => e.end(),
            Expr::Match(e) => e.end(),
            Expr::In(e) => e.end(),
            Expr::NotIn(e) => e.end(),
            Expr::Pipe(e) => e.end(),
            Expr::Try(e) => e.end(),
        }
    }
}

impl fmt::Display for Expr {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            Expr::Int(e) => write!(f, "{}", e),
            Expr::Float(e) => write!(f, "{}", e),
            Expr::Bool(e) => write!(f, "{}", e),
            Expr::Nil(e) => write!(f, "{}", e),
            Expr::String(e) => write!(f, "{}", e),
            Expr::Ident(e) => write!(f, "{}", e),
            Expr::Prefix(e) => write!(f, "{}", e),
            Expr::Infix(e) => write!(f, "{}", e),
            Expr::Spread(e) => write!(f, "{}", e),
            Expr::List(e) => write!(f, "{}", e),
            Expr::Map(e) => write!(f, "{}", e),
            Expr::Func(e) => write!(f, "{}", e),
            Expr::Call(e) => write!(f, "{}", e),
            Expr::GetAttr(e) => write!(f, "{}", e),
            Expr::ObjectCall(e) => write!(f, "{}", e),
            Expr::Index(e) => write!(f, "{}", e),
            Expr::Slice(e) => write!(f, "{}", e),
            Expr::If(e) => write!(f, "{}", e),
            Expr::Switch(e) => write!(f, "{}", e),
            Expr::Match(e) => write!(f, "{}", e),
            Expr::In(e) => write!(f, "{}", e),
            Expr::NotIn(e) => write!(f, "{}", e),
            Expr::Pipe(e) => write!(f, "{}", e),
            Expr::Try(e) => write!(f, "{}", e),
        }
    }
}

// ============================================================================
// Statements
// ============================================================================

/// Statement node enumeration.
#[derive(Debug, Clone, PartialEq)]
pub enum Stmt {
    Var(VarStmt),
    MultiVar(MultiVarStmt),
    ObjectDestructure(ObjectDestructureStmt),
    ArrayDestructure(ArrayDestructureStmt),
    Const(ConstStmt),
    Return(ReturnStmt),
    Assign(AssignStmt),
    SetAttr(SetAttrStmt),
    Postfix(PostfixStmt),
    Throw(ThrowStmt),
    Expr(Expr),
    Func(Rc<FuncLit>),
    Try(Box<TryExpr>),
}

impl Node for Stmt {
    fn pos(&self) -> Position {
        match self {
            Stmt::Var(s) => s.pos(),
            Stmt::MultiVar(s) => s.pos(),
            Stmt::ObjectDestructure(s) => s.pos(),
            Stmt::ArrayDestructure(s) => s.pos(),
            Stmt::Const(s) => s.pos(),
            Stmt::Return(s) => s.pos(),
            Stmt::Assign(s) => s.pos(),
            Stmt::SetAttr(s) => s.pos(),
            Stmt::Postfix(s) => s.pos(),
            Stmt::Throw(s) => s.pos(),
            Stmt::Expr(e) => e.pos(),
            Stmt::Func(f) => f.pos(),
            Stmt::Try(t) => t.pos(),
        }
    }

    fn end(&self) -> Position {
        match self {
            Stmt::Var(s) => s.end(),
            Stmt::MultiVar(s) => s.end(),
            Stmt::ObjectDestructure(s) => s.end(),
            Stmt::ArrayDestructure(s) => s.end(),
            Stmt::Const(s) => s.end(),
            Stmt::Return(s) => s.end(),
            Stmt::Assign(s) => s.end(),
            Stmt::SetAttr(s) => s.end(),
            Stmt::Postfix(s) => s.end(),
            Stmt::Throw(s) => s.end(),
            Stmt::Expr(e) => e.end(),
            Stmt::Func(f) => f.end(),
            Stmt::Try(t) => t.end(),
        }
    }
}

impl fmt::Display for Stmt {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            Stmt::Var(s) => write!(f, "{}", s),
            Stmt::MultiVar(s) => write!(f, "{}", s),
            Stmt::ObjectDestructure(s) => write!(f, "{}", s),
            Stmt::ArrayDestructure(s) => write!(f, "{}", s),
            Stmt::Const(s) => write!(f, "{}", s),
            Stmt::Return(s) => write!(f, "{}", s),
            Stmt::Assign(s) => write!(f, "{}", s),
            Stmt::SetAttr(s) => write!(f, "{}", s),
            Stmt::Postfix(s) => write!(f, "{}", s),
            Stmt::Throw(s) => write!(f, "{}", s),
            Stmt::Expr(e) => write!(f, "{}", e),
            Stmt::Func(func) => write!(f, "{}", func),
            Stmt::Try(t) => write!(f, "{}", t),
        }
    }
}

// ============================================================================
// Literal Expressions
// ============================================================================

#[derive(Debug, Clone, PartialEq)]
pub struct IntLit {
    pub position: Position,
    pub literal: String,
    pub value: i64,
}

impl Node for IntLit {
    fn pos(&self) -> Position {
        self.position
    }
    fn end(&self) -> Position {
        self.position.advance(self.literal.len())
    }
}

impl fmt::Display for IntLit {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(f, "{}", self.literal)
    }
}

#[derive(Debug, Clone, PartialEq)]
pub struct FloatLit {
    pub position: Position,
    pub literal: String,
    pub value: f64,
}

impl Node for FloatLit {
    fn pos(&self) -> Position {
        self.position
    }
    fn end(&self) -> Position {
        self.position.advance(self.literal.len())
    }
}

impl fmt::Display for FloatLit {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(f, "{}", self.literal)
    }
}

#[derive(Debug, Clone, PartialEq)]
pub struct BoolLit {
    pub position: Position,
    pub value: bool,
}

impl Node for BoolLit {
    fn pos(&self) -> Position {
        self.position
    }
    fn end(&self) -> Position {
        let len = if self.value { 4 } else { 5 };
        self.position.advance(len)
    }
}

impl fmt::Display for BoolLit {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(f, "{}", if self.value { "true" } else { "false" })
    }
}

#[derive(Debug, Clone, PartialEq)]
pub struct NilLit {
    pub position: Position,
}

impl Node for NilLit {
    fn pos(&self) -> Position {
        self.position
    }
    fn end(&self) -> Position {
        self.position.advance(3)
    }
}

impl fmt::Display for NilLit {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(f, "nil")
    }
}

#[derive(Debug, Clone, PartialEq)]
pub struct StringLit {
    pub position: Position,
    pub value: String,
}

impl Node for StringLit {
    fn pos(&self) -> Position {
        self.position
    }
    fn end(&self) -> Position {
        self.position.advance(self.value.len() + 2)
    }
}

impl fmt::Display for StringLit {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(f, "\"{}\"", self.value)
    }
}

// ============================================================================
// Identifier
// ============================================================================

#[derive(Debug, Clone, PartialEq)]
pub struct Ident {
    pub position: Position,
    pub name: String,
}

impl Node for Ident {
    fn pos(&self) -> Position {
        self.position
    }
    fn end(&self) -> Position {
        self.position.advance(self.name.len())
    }
}

impl fmt::Display for Ident {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(f, "{}", self.name)
    }
}

// ============================================================================
// Operator Expressions
// ============================================================================

#[derive(Debug, Clone, PartialEq)]
pub struct PrefixExpr {
    pub op_pos: Position,
    pub op: String,
    pub right: Expr,
}

impl Node for PrefixExpr {
    fn pos(&self) -> Position {
        self.op_pos
    }
    fn end(&self) -> Position {
        self.right.end()
    }
}

impl fmt::Display for PrefixExpr {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(f, "({}{})", self.op, self.right)
    }
}

#[derive(Debug, Clone, PartialEq)]
pub struct InfixExpr {
    pub left: Expr,
    pub op_pos: Position,
    pub op: String,
    pub right: Expr,
}

impl Node for InfixExpr {
    fn pos(&self) -> Position {
        self.left.pos()
    }
    fn end(&self) -> Position {
        self.right.end()
    }
}

impl fmt::Display for InfixExpr {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(f, "({} {} {})", self.left, self.op, self.right)
    }
}

#[derive(Debug, Clone, PartialEq)]
pub struct SpreadExpr {
    pub ellipsis: Position,
    pub expr: Option<Expr>,
}

impl Node for SpreadExpr {
    fn pos(&self) -> Position {
        self.ellipsis
    }
    fn end(&self) -> Position {
        self.expr
            .as_ref()
            .map(|e| e.end())
            .unwrap_or_else(|| self.ellipsis.advance(3))
    }
}

impl fmt::Display for SpreadExpr {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match &self.expr {
            Some(e) => write!(f, "...{}", e),
            None => write!(f, "..."),
        }
    }
}

// ============================================================================
// Collection Literals
// ============================================================================

#[derive(Debug, Clone, PartialEq)]
pub struct ListLit {
    pub lbrack: Position,
    pub items: Vec<Expr>,
    pub rbrack: Position,
}

impl Node for ListLit {
    fn pos(&self) -> Position {
        self.lbrack
    }
    fn end(&self) -> Position {
        self.rbrack.advance(1)
    }
}

impl fmt::Display for ListLit {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        let items: Vec<String> = self.items.iter().map(|i| i.to_string()).collect();
        write!(f, "[{}]", items.join(", "))
    }
}

#[derive(Debug, Clone, PartialEq)]
pub struct MapItem {
    pub key: Option<Expr>, // None for spread
    pub value: Expr,
}

#[derive(Debug, Clone, PartialEq)]
pub struct MapLit {
    pub lbrace: Position,
    pub items: Vec<MapItem>,
    pub rbrace: Position,
}

impl Node for MapLit {
    fn pos(&self) -> Position {
        self.lbrace
    }
    fn end(&self) -> Position {
        self.rbrace.advance(1)
    }
}

impl fmt::Display for MapLit {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        let pairs: Vec<String> = self
            .items
            .iter()
            .map(|item| match &item.key {
                Some(k) => format!("{}: {}", k, item.value),
                None => format!("...{}", item.value),
            })
            .collect();
        write!(f, "{{{}}}", pairs.join(", "))
    }
}

// ============================================================================
// Function Expressions
// ============================================================================

#[derive(Debug, Clone, PartialEq)]
pub struct DestructureBinding {
    pub key: String,
    pub alias: Option<String>,
    pub default_value: Option<Expr>,
}

#[derive(Debug, Clone, PartialEq)]
pub struct ArrayDestructureElement {
    pub name: Ident,
    pub default_value: Option<Expr>,
}

#[derive(Debug, Clone, PartialEq)]
pub enum FuncParam {
    Ident(Ident),
    ObjectDestructure {
        lbrace: Position,
        bindings: Vec<DestructureBinding>,
        rbrace: Position,
    },
    ArrayDestructure {
        lbrack: Position,
        elements: Vec<ArrayDestructureElement>,
        rbrack: Position,
    },
}

impl fmt::Display for FuncParam {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            FuncParam::Ident(id) => write!(f, "{}", id.name),
            FuncParam::ObjectDestructure { bindings, .. } => {
                let parts: Vec<String> = bindings
                    .iter()
                    .map(|b| {
                        let mut s = b.key.clone();
                        if let Some(alias) = &b.alias {
                            if alias != &b.key {
                                s.push_str(": ");
                                s.push_str(alias);
                            }
                        }
                        if let Some(def) = &b.default_value {
                            s.push_str(" = ");
                            s.push_str(&def.to_string());
                        }
                        s
                    })
                    .collect();
                write!(f, "{{{}}}", parts.join(", "))
            }
            FuncParam::ArrayDestructure { elements, .. } => {
                let parts: Vec<String> = elements
                    .iter()
                    .map(|e| {
                        let mut s = e.name.name.clone();
                        if let Some(def) = &e.default_value {
                            s.push_str(" = ");
                            s.push_str(&def.to_string());
                        }
                        s
                    })
                    .collect();
                write!(f, "[{}]", parts.join(", "))
            }
        }
    }
}

#[derive(Debug, Clone, PartialEq)]
pub struct FuncLit {
    pub func_pos: Position,
    pub name: Option<Ident>,
    pub lparen: Position,
    pub params: Vec<FuncParam>,
    pub defaults: Vec<(String, Expr)>,
    pub rest_param: Option<Ident>,
    pub rparen: Position,
    pub body: Block,
}

impl Node for FuncLit {
    fn pos(&self) -> Position {
        self.func_pos
    }
    fn end(&self) -> Position {
        self.body.end()
    }
}

impl fmt::Display for FuncLit {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        let params: Vec<String> = self.params.iter().map(|p| p.to_string()).collect();
        let name = self
            .name
            .as_ref()
            .map(|n| format!(" {}", n.name))
            .unwrap_or_default();
        write!(f, "function{}({}) {{ {} }}", name, params.join(", "), self.body)
    }
}

// ============================================================================
// Access Expressions
// ============================================================================

#[derive(Debug, Clone, PartialEq)]
pub struct CallExpr {
    pub func: Expr,
    pub lparen: Position,
    pub args: Vec<Expr>,
    pub rparen: Position,
}

impl Node for CallExpr {
    fn pos(&self) -> Position {
        self.func.pos()
    }
    fn end(&self) -> Position {
        self.rparen.advance(1)
    }
}

impl fmt::Display for CallExpr {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        let args: Vec<String> = self.args.iter().map(|a| a.to_string()).collect();
        write!(f, "{}({})", self.func, args.join(", "))
    }
}

#[derive(Debug, Clone, PartialEq)]
pub struct GetAttrExpr {
    pub object: Expr,
    pub period: Position,
    pub attr: Ident,
    pub optional: bool,
}

impl Node for GetAttrExpr {
    fn pos(&self) -> Position {
        self.object.pos()
    }
    fn end(&self) -> Position {
        self.attr.end()
    }
}

impl fmt::Display for GetAttrExpr {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        let op = if self.optional { "?." } else { "." };
        write!(f, "{}{}{}", self.object, op, self.attr.name)
    }
}

#[derive(Debug, Clone, PartialEq)]
pub struct ObjectCallExpr {
    pub object: Expr,
    pub period: Position,
    pub call: CallExpr,
    pub optional: bool,
}

impl Node for ObjectCallExpr {
    fn pos(&self) -> Position {
        self.object.pos()
    }
    fn end(&self) -> Position {
        self.call.end()
    }
}

impl fmt::Display for ObjectCallExpr {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        let op = if self.optional { "?." } else { "." };
        write!(f, "{}{}{}", self.object, op, self.call)
    }
}

#[derive(Debug, Clone, PartialEq)]
pub struct IndexExpr {
    pub object: Expr,
    pub lbrack: Position,
    pub index: Expr,
    pub rbrack: Position,
}

impl Node for IndexExpr {
    fn pos(&self) -> Position {
        self.object.pos()
    }
    fn end(&self) -> Position {
        self.rbrack.advance(1)
    }
}

impl fmt::Display for IndexExpr {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(f, "{}[{}]", self.object, self.index)
    }
}

#[derive(Debug, Clone, PartialEq)]
pub struct SliceExpr {
    pub object: Expr,
    pub lbrack: Position,
    pub low: Option<Expr>,
    pub high: Option<Expr>,
    pub rbrack: Position,
}

impl Node for SliceExpr {
    fn pos(&self) -> Position {
        self.object.pos()
    }
    fn end(&self) -> Position {
        self.rbrack.advance(1)
    }
}

impl fmt::Display for SliceExpr {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        let low = self.low.as_ref().map(|e| e.to_string()).unwrap_or_default();
        let high = self
            .high
            .as_ref()
            .map(|e| e.to_string())
            .unwrap_or_default();
        write!(f, "{}[{}:{}]", self.object, low, high)
    }
}

// ============================================================================
// Control Flow
// ============================================================================

#[derive(Debug, Clone, PartialEq)]
pub struct Block {
    pub lbrace: Position,
    pub stmts: Vec<Stmt>,
    pub rbrace: Position,
}

impl Node for Block {
    fn pos(&self) -> Position {
        self.lbrace
    }
    fn end(&self) -> Position {
        self.rbrace.advance(1)
    }
}

impl fmt::Display for Block {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        let stmts: Vec<String> = self.stmts.iter().map(|s| s.to_string()).collect();
        write!(f, "{}", stmts.join("\n"))
    }
}

#[derive(Debug, Clone, PartialEq)]
pub struct IfExpr {
    pub if_pos: Position,
    pub condition: Expr,
    pub consequence: Block,
    pub alternative: Option<Block>,
}

impl Node for IfExpr {
    fn pos(&self) -> Position {
        self.if_pos
    }
    fn end(&self) -> Position {
        self.alternative
            .as_ref()
            .map(|a| a.end())
            .unwrap_or_else(|| self.consequence.end())
    }
}

impl fmt::Display for IfExpr {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(f, "if ({}) {}", self.condition, self.consequence)?;
        if let Some(alt) = &self.alternative {
            write!(f, " else {}", alt)?;
        }
        Ok(())
    }
}

#[derive(Debug, Clone, PartialEq)]
pub struct CaseClause {
    pub case_pos: Position,
    pub exprs: Option<Vec<Expr>>, // None for default
    pub colon: Position,
    pub body: Block,
    pub is_default: bool,
}

impl Node for CaseClause {
    fn pos(&self) -> Position {
        self.case_pos
    }
    fn end(&self) -> Position {
        self.body.end()
    }
}

impl fmt::Display for CaseClause {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        if self.is_default {
            write!(f, "default: {}", self.body)
        } else {
            let exprs: Vec<String> = self
                .exprs
                .as_ref()
                .unwrap()
                .iter()
                .map(|e| e.to_string())
                .collect();
            write!(f, "case {}: {}", exprs.join(", "), self.body)
        }
    }
}

#[derive(Debug, Clone, PartialEq)]
pub struct SwitchExpr {
    pub switch_pos: Position,
    pub value: Expr,
    pub lbrace: Position,
    pub cases: Vec<CaseClause>,
    pub rbrace: Position,
}

impl Node for SwitchExpr {
    fn pos(&self) -> Position {
        self.switch_pos
    }
    fn end(&self) -> Position {
        self.rbrace.advance(1)
    }
}

impl fmt::Display for SwitchExpr {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        let cases: Vec<String> = self.cases.iter().map(|c| c.to_string()).collect();
        write!(f, "switch ({}) {{\n{}\n}}", self.value, cases.join("\n"))
    }
}

// ============================================================================
// Pattern Matching
// ============================================================================

#[derive(Debug, Clone, PartialEq)]
pub enum Pattern {
    Wildcard(Position),
    Literal(Expr),
}

impl Node for Pattern {
    fn pos(&self) -> Position {
        match self {
            Pattern::Wildcard(pos) => *pos,
            Pattern::Literal(expr) => expr.pos(),
        }
    }
    fn end(&self) -> Position {
        match self {
            Pattern::Wildcard(pos) => pos.advance(1),
            Pattern::Literal(expr) => expr.end(),
        }
    }
}

impl fmt::Display for Pattern {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            Pattern::Wildcard(_) => write!(f, "_"),
            Pattern::Literal(expr) => write!(f, "{}", expr),
        }
    }
}

#[derive(Debug, Clone, PartialEq)]
pub struct MatchArm {
    pub pattern: Pattern,
    pub guard: Option<Expr>,
    pub arrow: Position,
    pub result: Expr,
}

impl Node for MatchArm {
    fn pos(&self) -> Position {
        self.pattern.pos()
    }
    fn end(&self) -> Position {
        self.result.end()
    }
}

impl fmt::Display for MatchArm {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(f, "{}", self.pattern)?;
        if let Some(guard) = &self.guard {
            write!(f, " if {}", guard)?;
        }
        write!(f, " => {}", self.result)
    }
}

#[derive(Debug, Clone, PartialEq)]
pub struct MatchExpr {
    pub match_pos: Position,
    pub subject: Expr,
    pub lbrace: Position,
    pub arms: Vec<MatchArm>,
    pub default_arm: Option<MatchArm>,
    pub rbrace: Position,
}

impl Node for MatchExpr {
    fn pos(&self) -> Position {
        self.match_pos
    }
    fn end(&self) -> Position {
        self.rbrace.advance(1)
    }
}

impl fmt::Display for MatchExpr {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        let mut arms: Vec<String> = self.arms.iter().map(|a| a.to_string()).collect();
        if let Some(def) = &self.default_arm {
            arms.push(def.to_string());
        }
        write!(f, "match {} {{ {} }}", self.subject, arms.join(", "))
    }
}

// ============================================================================
// Membership and Pipe
// ============================================================================

#[derive(Debug, Clone, PartialEq)]
pub struct InExpr {
    pub left: Expr,
    pub in_pos: Position,
    pub right: Expr,
}

impl Node for InExpr {
    fn pos(&self) -> Position {
        self.left.pos()
    }
    fn end(&self) -> Position {
        self.right.end()
    }
}

impl fmt::Display for InExpr {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(f, "{} in {}", self.left, self.right)
    }
}

#[derive(Debug, Clone, PartialEq)]
pub struct NotInExpr {
    pub left: Expr,
    pub not_pos: Position,
    pub right: Expr,
}

impl Node for NotInExpr {
    fn pos(&self) -> Position {
        self.left.pos()
    }
    fn end(&self) -> Position {
        self.right.end()
    }
}

impl fmt::Display for NotInExpr {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(f, "{} not in {}", self.left, self.right)
    }
}

#[derive(Debug, Clone, PartialEq)]
pub struct PipeExpr {
    pub exprs: Vec<Expr>,
}

impl Node for PipeExpr {
    fn pos(&self) -> Position {
        self.exprs.first().map(|e| e.pos()).unwrap_or_default()
    }
    fn end(&self) -> Position {
        self.exprs.last().map(|e| e.end()).unwrap_or_default()
    }
}

impl fmt::Display for PipeExpr {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        let exprs: Vec<String> = self.exprs.iter().map(|e| e.to_string()).collect();
        write!(f, "({})", exprs.join(" | "))
    }
}

// ============================================================================
// Try/Catch/Finally
// ============================================================================

#[derive(Debug, Clone, PartialEq)]
pub struct TryExpr {
    pub try_pos: Position,
    pub body: Block,
    pub catch_ident: Option<Ident>,
    pub catch_block: Option<Block>,
    pub finally_block: Option<Block>,
}

impl Node for TryExpr {
    fn pos(&self) -> Position {
        self.try_pos
    }
    fn end(&self) -> Position {
        self.finally_block
            .as_ref()
            .map(|b| b.end())
            .or_else(|| self.catch_block.as_ref().map(|b| b.end()))
            .unwrap_or_else(|| self.body.end())
    }
}

impl fmt::Display for TryExpr {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(f, "try {}", self.body)?;
        if let Some(catch) = &self.catch_block {
            write!(f, " catch")?;
            if let Some(id) = &self.catch_ident {
                write!(f, " {}", id.name)?;
            }
            write!(f, " {}", catch)?;
        }
        if let Some(fin) = &self.finally_block {
            write!(f, " finally {}", fin)?;
        }
        Ok(())
    }
}

// ============================================================================
// Statements
// ============================================================================

#[derive(Debug, Clone, PartialEq)]
pub struct VarStmt {
    pub let_pos: Position,
    pub name: Ident,
    pub value: Expr,
}

impl Node for VarStmt {
    fn pos(&self) -> Position {
        self.let_pos
    }
    fn end(&self) -> Position {
        self.value.end()
    }
}

impl fmt::Display for VarStmt {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(f, "let {} = {}", self.name.name, self.value)
    }
}

#[derive(Debug, Clone, PartialEq)]
pub struct MultiVarStmt {
    pub let_pos: Position,
    pub names: Vec<Ident>,
    pub value: Expr,
}

impl Node for MultiVarStmt {
    fn pos(&self) -> Position {
        self.let_pos
    }
    fn end(&self) -> Position {
        self.value.end()
    }
}

impl fmt::Display for MultiVarStmt {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        let names: Vec<String> = self.names.iter().map(|n| n.name.clone()).collect();
        write!(f, "let {} = {}", names.join(", "), self.value)
    }
}

#[derive(Debug, Clone, PartialEq)]
pub struct ObjectDestructureStmt {
    pub let_pos: Position,
    pub lbrace: Position,
    pub bindings: Vec<DestructureBinding>,
    pub rbrace: Position,
    pub value: Expr,
}

impl Node for ObjectDestructureStmt {
    fn pos(&self) -> Position {
        self.let_pos
    }
    fn end(&self) -> Position {
        self.value.end()
    }
}

impl fmt::Display for ObjectDestructureStmt {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        let parts: Vec<String> = self
            .bindings
            .iter()
            .map(|b| {
                let mut s = b.key.clone();
                if let Some(alias) = &b.alias {
                    if alias != &b.key {
                        s.push_str(": ");
                        s.push_str(alias);
                    }
                }
                if let Some(def) = &b.default_value {
                    s.push_str(" = ");
                    s.push_str(&def.to_string());
                }
                s
            })
            .collect();
        write!(f, "let {{ {} }} = {}", parts.join(", "), self.value)
    }
}

#[derive(Debug, Clone, PartialEq)]
pub struct ArrayDestructureStmt {
    pub let_pos: Position,
    pub lbrack: Position,
    pub elements: Vec<ArrayDestructureElement>,
    pub rbrack: Position,
    pub value: Expr,
}

impl Node for ArrayDestructureStmt {
    fn pos(&self) -> Position {
        self.let_pos
    }
    fn end(&self) -> Position {
        self.value.end()
    }
}

impl fmt::Display for ArrayDestructureStmt {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        let parts: Vec<String> = self
            .elements
            .iter()
            .map(|e| {
                let mut s = e.name.name.clone();
                if let Some(def) = &e.default_value {
                    s.push_str(" = ");
                    s.push_str(&def.to_string());
                }
                s
            })
            .collect();
        write!(f, "let [{}] = {}", parts.join(", "), self.value)
    }
}

#[derive(Debug, Clone, PartialEq)]
pub struct ConstStmt {
    pub const_pos: Position,
    pub name: Ident,
    pub value: Expr,
}

impl Node for ConstStmt {
    fn pos(&self) -> Position {
        self.const_pos
    }
    fn end(&self) -> Position {
        self.value.end()
    }
}

impl fmt::Display for ConstStmt {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(f, "const {} = {}", self.name.name, self.value)
    }
}

#[derive(Debug, Clone, PartialEq)]
pub struct ReturnStmt {
    pub return_pos: Position,
    pub value: Option<Expr>,
}

impl Node for ReturnStmt {
    fn pos(&self) -> Position {
        self.return_pos
    }
    fn end(&self) -> Position {
        self.value
            .as_ref()
            .map(|v| v.end())
            .unwrap_or_else(|| self.return_pos.advance(6))
    }
}

impl fmt::Display for ReturnStmt {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match &self.value {
            Some(v) => write!(f, "return {}", v),
            None => write!(f, "return"),
        }
    }
}

/// Assignment target (identifier or index expression).
#[derive(Debug, Clone, PartialEq)]
pub enum AssignTarget {
    Ident(Ident),
    Index(Box<IndexExpr>),
}

#[derive(Debug, Clone, PartialEq)]
pub struct AssignStmt {
    pub target: AssignTarget,
    pub op_pos: Position,
    pub op: String,
    pub value: Expr,
}

impl Node for AssignStmt {
    fn pos(&self) -> Position {
        match &self.target {
            AssignTarget::Ident(id) => id.pos(),
            AssignTarget::Index(idx) => idx.pos(),
        }
    }
    fn end(&self) -> Position {
        self.value.end()
    }
}

impl fmt::Display for AssignStmt {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        let target = match &self.target {
            AssignTarget::Ident(id) => id.to_string(),
            AssignTarget::Index(idx) => idx.to_string(),
        };
        write!(f, "{} {} {}", target, self.op, self.value)
    }
}

#[derive(Debug, Clone, PartialEq)]
pub struct SetAttrStmt {
    pub object: Expr,
    pub period: Position,
    pub attr: Ident,
    pub op_pos: Position,
    pub op: String,
    pub value: Expr,
}

impl Node for SetAttrStmt {
    fn pos(&self) -> Position {
        self.object.pos()
    }
    fn end(&self) -> Position {
        self.value.end()
    }
}

impl fmt::Display for SetAttrStmt {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(
            f,
            "{}.{} {} {}",
            self.object, self.attr.name, self.op, self.value
        )
    }
}

#[derive(Debug, Clone, PartialEq)]
pub struct PostfixStmt {
    pub operand: Expr,
    pub op_pos: Position,
    pub op: String,
}

impl Node for PostfixStmt {
    fn pos(&self) -> Position {
        self.operand.pos()
    }
    fn end(&self) -> Position {
        self.op_pos.advance(2)
    }
}

impl fmt::Display for PostfixStmt {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(f, "({}{})", self.operand, self.op)
    }
}

#[derive(Debug, Clone, PartialEq)]
pub struct ThrowStmt {
    pub throw_pos: Position,
    pub value: Expr,
}

impl Node for ThrowStmt {
    fn pos(&self) -> Position {
        self.throw_pos
    }
    fn end(&self) -> Position {
        self.value.end()
    }
}

impl fmt::Display for ThrowStmt {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(f, "throw {}", self.value)
    }
}

// ============================================================================
// Program
// ============================================================================

#[derive(Debug, Clone, PartialEq)]
pub struct Program {
    pub stmts: Vec<Stmt>,
}

impl Node for Program {
    fn pos(&self) -> Position {
        self.stmts.first().map(|s| s.pos()).unwrap_or_default()
    }
    fn end(&self) -> Position {
        self.stmts.last().map(|s| s.end()).unwrap_or_default()
    }
}

impl fmt::Display for Program {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        let stmts: Vec<String> = self.stmts.iter().map(|s| s.to_string()).collect();
        write!(f, "{}", stmts.join("\n"))
    }
}
