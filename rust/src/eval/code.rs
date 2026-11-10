//! A compiled form of the hot node kinds: a tree of closures, built once
//! when a program is compiled.
//!
//! The walk in `mod.rs` stays the definition of every form. A closure here
//! does what the walk does for its node, in the same order and with the same
//! charges, but reaches its children directly instead of through the arena
//! and the resolution. A node of another kind has no closure, and the walk
//! evaluates it; `Run::eval` tries the closure of a node first.
//!
//! **Stack.** The walk runs a left-associative chain of operators, the steps
//! of a chain and the `else` of a conditional in a loop. A closure recurses
//! into its children instead. So a node gets a closure only when the depth
//! of its subtree is at most [`MAX_DEPTH`]; above it, the walk runs the node
//! as before, and the closures it reaches below are bounded by that depth.

use super::{Arg, Run, R};
use crate::compile::Res;
use crate::source::Span;
use crate::syntax::ast::{Ast, BinaryOp, NodeId, NodeKind, TemplatePart};
use crate::value::{release, Str, Value};
use std::sync::Arc;

/// The closure of a node.
pub(crate) type Code =
    Arc<dyn for<'h> Fn(&mut Run<'h>) -> R<'h, Value<'h>> + Send + Sync + 'static>;

/// The greatest depth of a subtree that has closures.
const MAX_DEPTH: u32 = 48;

fn mk<F>(f: F) -> Code
where
    F: for<'h> Fn(&mut Run<'h>) -> R<'h, Value<'h>> + Send + Sync + 'static,
{
    Arc::new(f)
}

/// The children of a node, as the walk reaches them.
fn children(kind: &NodeKind, out: &mut Vec<NodeId>) {
    out.clear();
    match kind {
        NodeKind::Program { consts, result } | NodeKind::Block { consts, result } => {
            out.extend(consts.iter().copied());
            out.push(*result);
        }
        NodeKind::Const { pattern, value } => out.extend([*pattern, *value]),
        NodeKind::Template { parts } => out.extend(parts.iter().filter_map(|p| match p {
            TemplatePart::Expr(e) => Some(*e),
            TemplatePart::Text(_) => None,
        })),
        NodeKind::List { items } => out.extend(items.iter().copied()),
        NodeKind::Map { entries } => out.extend(entries.iter().copied()),
        NodeKind::Entry { key, value, .. } => {
            if let crate::syntax::ast::EntryKey::Computed(k) = key {
                out.push(*k);
            }
            out.push(*value);
        }
        NodeKind::Spread { value } => out.push(*value),
        NodeKind::Unary { x, .. } => out.push(*x),
        NodeKind::Binary { x, y, .. } => out.extend([*x, *y]),
        NodeKind::Cond {
            test, then, else_, ..
        } => out.extend([*test, *then, *else_]),
        NodeKind::Member { x, .. } => out.push(*x),
        NodeKind::Index { x, index, .. } => out.extend([*x, *index]),
        NodeKind::Call { args, .. } => out.extend(args.iter().copied()),
        NodeKind::MethodCall { x, args, .. } => {
            out.push(*x);
            out.extend(args.iter().copied());
        }
        NodeKind::OptChain { x } => out.push(*x),
        NodeKind::Lambda { params, body } => {
            out.extend(params.iter().copied());
            out.push(*body);
        }
        NodeKind::Param { pattern, default } => {
            out.push(*pattern);
            out.extend(default.iter().copied());
        }
        NodeKind::Match { subject, arms } => {
            out.push(*subject);
            out.extend(arms.iter().copied());
        }
        NodeKind::Arm { result, .. } => out.push(*result),
        NodeKind::Try { body, handler, .. } => out.extend([*body, *handler]),
        NodeKind::MapPat { fields, .. } => out.extend(fields.iter().filter_map(|f| f.default)),
        NodeKind::ListPat { elems, .. } => {
            for e in elems {
                out.push(e.pat);
                out.extend(e.default.iter().copied());
            }
        }
        NodeKind::As { x, .. } => out.push(*x),
        NodeKind::Each { x, reads, .. } => {
            out.extend(x.iter().copied());
            out.extend(reads.iter().copied());
        }
        NodeKind::Ident { .. }
        | NodeKind::Number { .. }
        | NodeKind::String { .. }
        | NodeKind::Bool { .. }
        | NodeKind::Null
        | NodeKind::NamePat { .. }
        | NodeKind::Elem
        | NodeKind::Bad => {}
    }
}

/// Whether a node is a step of a chain, as `Run::chain` collects them.
fn is_step(ast: &Ast, res: &[Res], n: NodeId) -> Option<NodeId> {
    match ast.kind(n) {
        NodeKind::Member { .. } if matches!(res[n.index()], Res::Constant(_)) => None,
        NodeKind::Each { x: Some(x), .. } => Some(*x),
        NodeKind::Member { x, .. } | NodeKind::Index { x, .. } => Some(*x),
        NodeKind::MethodCall { x, .. }
            if !matches!(res[n.index()], Res::CallHost(_) | Res::CallLib(_)) =>
        {
            Some(*x)
        }
        _ => None,
    }
}

/// A step of a chain: a field read, compiled, or another step by its node.
enum Step {
    Member {
        name: Box<str>,
        optional: bool,
        span: Span,
    },
    Node(NodeId),
}

/// The closures of a program's nodes, indexed by node.
pub(crate) fn build(ast: &Ast, res: &[Res], var_index: &[u32]) -> Vec<Option<Code>> {
    let n = ast.nodes.len();
    // The depth of each node's subtree; `u32::MAX` while unknown, so that a
    // child built after its parent counts as too deep.
    let mut depth = vec![u32::MAX; n];
    let mut kids = Vec::new();
    for i in 0..n {
        children(&ast.nodes[i].kind, &mut kids);
        let mut d = 1u32;
        for k in &kids {
            d = d.max(
                depth
                    .get(k.index())
                    .copied()
                    .unwrap_or(u32::MAX)
                    .saturating_add(1),
            );
        }
        depth[i] = d;
    }
    // The nodes a chain reads as its steps: the walk never evaluates them on
    // their own.
    let mut inner = vec![false; n];
    for i in 0..n {
        let id = NodeId(i as u32);
        let x = match ast.kind(id) {
            NodeKind::OptChain { x } => Some(*x),
            _ => is_step(ast, res, id),
        };
        // The start of a chain is evaluated on its own; a step is not.
        if let Some(x) = x {
            if is_step(ast, res, x).is_some() {
                inner[x.index()] = true;
            }
        }
    }
    let mut code: Vec<Option<Code>> = Vec::with_capacity(n);
    for i in 0..n {
        let id = NodeId(i as u32);
        let c = if depth[i] <= MAX_DEPTH && !inner[i] {
            node(ast, res, var_index, id, &code)
        } else {
            None
        };
        code.push(c);
    }
    code
}

/// The closure of a child: its own, or the walk.
fn child(code: &[Option<Code>], c: NodeId) -> Code {
    match code.get(c.index()) {
        Some(Some(k)) => k.clone(),
        _ => mk(move |r| r.eval(c)),
    }
}

fn node(
    ast: &Ast,
    res: &[Res],
    var_index: &[u32],
    id: NodeId,
    code: &[Option<Code>],
) -> Option<Code> {
    let span = ast.span(id);
    Some(match ast.kind(id) {
        NodeKind::Number { value } => {
            let v = *value;
            mk(move |_| Ok(Value::Num(v)))
        }
        NodeKind::Bool { value } => {
            let v = *value;
            mk(move |_| Ok(Value::Bool(v)))
        }
        NodeKind::Null => mk(|_| Ok(Value::Null)),
        NodeKind::String { .. } => mk(move |r| match r.ast.kind(id) {
            NodeKind::String { value, .. } => Ok(Value::Str(Str::Lent(value))),
            _ => Err(r.internal(span)),
        }),
        NodeKind::Ident { .. } => match res[id.index()] {
            Res::Slot(s) => {
                let s = s as usize;
                mk(move |r| Ok(r.slots[s].dup()))
            }
            Res::Var(v) => {
                let v = var_index[v as usize] as usize;
                mk(move |r| r.var(v, id))
            }
            _ => return None,
        },
        NodeKind::Unary { op, x } => {
            let (op, x) = (*op, child(code, *x));
            mk(move |r| {
                let v = x(r)?;
                r.unary(op, v, span)
            })
        }
        NodeKind::Binary { op, x, y } => {
            let (x_id, y_id) = (*x, *y);
            let (x, y) = (child(code, *x), child(code, *y));
            match *op {
                BinaryOp::And => mk(move |r| {
                    let v = x(r)?;
                    if v.truthy() {
                        release(v);
                        y(r)
                    } else {
                        Ok(v)
                    }
                }),
                BinaryOp::Or => mk(move |r| {
                    let v = x(r)?;
                    if v.truthy() {
                        Ok(v)
                    } else {
                        release(v);
                        y(r)
                    }
                }),
                BinaryOp::Coalesce => mk(move |r| {
                    let v = x(r)?;
                    if v.is_null() {
                        std::mem::forget(v);
                        y(r)
                    } else {
                        Ok(v)
                    }
                }),
                // `x in [a, b, ...]`: the list's elements are tested where
                // they are, and the list itself is never built. Its charge
                // stands where the literal makes it.
                // Every item a scalar written in place: evaluating them
                // charges nothing and cannot fail, so each is tested where it
                // is written.
                op @ (BinaryOp::In | BinaryOp::NotIn)
                    if literal_list(ast, y_id)
                        .is_some_and(|items| items.iter().all(|i| scalar(ast, *i).is_some())) =>
                {
                    let items: Box<[Scalar]> = literal_list(ast, y_id)
                        .unwrap_or_default()
                        .iter()
                        .filter_map(|&i| scalar(ast, i))
                        .collect();
                    let units = 1 + items.len() as u64;
                    mk(move |r| {
                        let v = x(r)?;
                        let found = r.in_scalars(&v, &items, units);
                        release(v);
                        Ok(Value::Bool(found? == (op == BinaryOp::In)))
                    })
                }
                op @ (BinaryOp::In | BinaryOp::NotIn) if literal_list(ast, y_id).is_some() => {
                    let items: Box<[Code]> = literal_list(ast, y_id)
                        .unwrap_or_default()
                        .iter()
                        .map(|&i| child(code, i))
                        .collect();
                    mk(move |r| {
                        let v = x(r)?;
                        let found = r.in_literal(&v, &items, span);
                        release(v);
                        let found = found?;
                        Ok(Value::Bool(found == (op == BinaryOp::In)))
                    })
                }
                op => operator(ast, res, code, op, x_id, y_id, span),
            }
        }
        NodeKind::Cond {
            test, then, else_, ..
        } => {
            let (t, a, b) = (child(code, *test), child(code, *then), child(code, *else_));
            mk(move |r| {
                let v = t(r)?;
                let truthy = v.truthy();
                release(v);
                if truthy {
                    a(r)
                } else {
                    b(r)
                }
            })
        }
        NodeKind::Member { .. } if matches!(res[id.index()], Res::Constant(_)) => {
            let Res::Constant(v) = res[id.index()] else {
                return None;
            };
            mk(move |_| Ok(Value::Num(v)))
        }
        NodeKind::Member { .. }
        | NodeKind::Index { .. }
        | NodeKind::MethodCall { .. }
        | NodeKind::OptChain { .. }
        | NodeKind::Each { x: Some(_), .. } => {
            if matches!(ast.kind(id), NodeKind::MethodCall { .. })
                && matches!(res[id.index()], Res::CallHost(_) | Res::CallLib(_))
            {
                return None;
            }
            // As `Run::chain`: the steps from the start outward.
            let top = match ast.kind(id) {
                NodeKind::OptChain { x } => *x,
                _ => id,
            };
            let mut steps = Vec::new();
            let mut n = top;
            while let Some(x) = is_step(ast, res, n) {
                steps.push(n);
                n = x;
            }
            steps.reverse();
            let start = child(code, n);
            let steps: Box<[Step]> = steps
                .into_iter()
                .map(|s| match ast.kind(s) {
                    NodeKind::Member { name, optional, .. } => Step::Member {
                        name: name.as_str().into(),
                        optional: *optional,
                        span: ast.span(s),
                    },
                    _ => Step::Node(s),
                })
                .collect();
            mk(move |r| {
                let mut v = start(r)?;
                for s in steps.iter() {
                    let next = match s {
                        Step::Member {
                            name,
                            optional,
                            span,
                        } => r.member(v, name, *optional, *span)?,
                        Step::Node(s) => r.step(*s, v)?,
                    };
                    match next {
                        Some(nv) => v = nv,
                        None => return Ok(Value::Null),
                    }
                }
                Ok(v)
            })
        }
        NodeKind::Program { consts, result } | NodeKind::Block { consts, result } => {
            let lines: Box<[Line]> = consts
                .iter()
                .map(|&c| match ast.kind(c) {
                    NodeKind::Const { value, .. }
                        if matches!(ast.kind(*value), NodeKind::Lambda { .. }) =>
                    {
                        Line::Lambda
                    }
                    NodeKind::Const { pattern, value }
                        if matches!(ast.kind(*pattern), NodeKind::NamePat { .. }) =>
                    {
                        Line::Name {
                            value: child(code, *value),
                            slot: match res[pattern.index()] {
                                Res::Bind(s) => Some(s as usize),
                                _ => None,
                            },
                        }
                    }
                    _ => Line::Node(c),
                })
                .collect();
            let result = child(code, *result);
            mk(move |r| {
                for l in lines.iter() {
                    match l {
                        // A lambda evaluated: 1 step, nothing bound.
                        Line::Lambda => r.charge(1)?,
                        Line::Name { value, slot } => {
                            let v = value(r)?;
                            match slot {
                                Some(s) => crate::value::put(&mut r.slots[*s], v),
                                None => crate::value::release(v),
                            }
                        }
                        Line::Node(c) => r.const_line(*c)?,
                    }
                }
                result(r)
            })
        }
        NodeKind::Template { parts } => {
            let parts: Box<[Part]> = parts
                .iter()
                .map(|p| match p {
                    TemplatePart::Text(t) => Part::Text(t.as_str().into()),
                    TemplatePart::Expr(e) => Part::Expr(child(code, *e)),
                })
                .collect();
            mk(move |r| {
                let mut out = super::Rendering::new(r.take_scratch());
                let mut vals = std::mem::take(&mut r.parts_buf);
                for p in parts.iter() {
                    match p {
                        Part::Text(t) => out.push(t),
                        Part::Expr(e) => {
                            let v = e(r)?;
                            r.render(&v, &mut out, true, span)?;
                            vals.push(v);
                        }
                    }
                }
                let texts = parts.iter().map(|p| match p {
                    Part::Text(t) => Some(&**t),
                    Part::Expr(_) => None,
                });
                r.template_done(out, vals, texts, span)
            })
        }
        // A call of a library function whose arguments are all values.
        NodeKind::Call { args, .. } if matches!(res[id.index()], Res::CallLib(_)) => {
            let Res::CallLib(i) = res[id.index()] else {
                return None;
            };
            let i = i as usize;
            if args.iter().any(|a| {
                matches!(ast.kind(*a), NodeKind::Lambda { .. })
                    || matches!(res[a.index()], Res::LambdaRef(_))
            }) {
                return None;
            }
            match **args {
                [a] => {
                    let a = child(code, a);
                    mk(move |r| {
                        let v = [Arg::Val(a(r)?)];
                        let out = r.lib_function(i, &v, span);
                        let [Arg::Val(x)] = v else { return out };
                        release(x);
                        out
                    })
                }
                [a, b] => {
                    let (a, b) = (child(code, a), child(code, b));
                    mk(move |r| {
                        let x = a(r)?;
                        let v = [Arg::Val(x), Arg::Val(b(r)?)];
                        let out = r.lib_function(i, &v, span);
                        let [Arg::Val(x), Arg::Val(y)] = v else {
                            return out;
                        };
                        release(x);
                        release(y);
                        out
                    })
                }
                _ => return None,
            }
        }
        NodeKind::List { items } => {
            if items
                .iter()
                .any(|i| matches!(ast.kind(*i), NodeKind::Spread { .. }))
            {
                return None;
            }
            let items: Box<[Code]> = items.iter().map(|&i| child(code, i)).collect();
            mk(move |r| {
                let mut out = Vec::with_capacity(items.len());
                for i in items.iter() {
                    out.push(i(r)?);
                }
                r.alloc(1 + out.len() as u64)?;
                Ok(Value::list(out))
            })
        }
        _ => return None,
    })
}

/// An operand of an operator, as its closure reaches it: a number written
/// in place, a slot, or another closure.
enum Operand {
    Num(f64),
    Slot(usize),
    Code(Code),
}

fn operand(ast: &Ast, res: &[Res], code: &[Option<Code>], n: NodeId) -> Operand {
    match (ast.kind(n), &res[n.index()]) {
        (NodeKind::Number { value }, _) => Operand::Num(*value),
        (NodeKind::Ident { .. }, Res::Slot(s)) => Operand::Slot(*s as usize),
        _ => Operand::Code(child(code, n)),
    }
}

/// An operator that is not `&&`, `||` or `??`: a closure for each operator
/// and each form of its operands, with the case of two numbers in place.
/// Every other case goes to `Run::operate`, as before.
fn operator(
    ast: &Ast,
    res: &[Res],
    code: &[Option<Code>],
    op: BinaryOp,
    x: NodeId,
    y: NodeId,
    span: Span,
) -> Code {
    use crate::number;
    let (a, b) = (operand(ast, res, code, x), operand(ast, res, code, y));
    // `==` and `!=` against a string written in place.
    if let (BinaryOp::Eq | BinaryOp::Ne, NodeKind::String { value, .. }) = (op, ast.kind(y)) {
        let lit: Box<str> = value.as_str().into();
        let x = child(code, x);
        let eq = op == BinaryOp::Eq;
        // A common prefix of fewer than 64 code points has size 0.
        let short = lit.len() < 64;
        return mk(move |r| {
            let v = x(r)?;
            if let Value::Str(s) = &v {
                let s = s.as_str();
                let sz = if short {
                    0
                } else {
                    crate::value::lcp_size(s, &lit)
                };
                let charged = r.charge(1 + sz);
                let equal = crate::value::str_eq(s, &lit);
                release(v);
                charged?;
                return Ok(Value::Bool(equal == eq));
            }
            let NodeKind::String { value, .. } = r.ast.kind(y) else {
                return Err(r.internal(span));
            };
            r.operate(op, v, Value::Str(Str::Lent(value)), span)
        });
    }
    macro_rules! num {
        ($f:expr) => {
            numeric(op, a, b, span, $f)
        };
    }
    match op {
        BinaryOp::Add => num!(|p, q| number::add(p, q).ok().map(Value::Num)),
        BinaryOp::Sub => num!(|p, q| number::sub(p, q).ok().map(Value::Num)),
        BinaryOp::Mul => num!(|p, q| number::mul(p, q).ok().map(Value::Num)),
        BinaryOp::Div => num!(|p, q| number::div(p, q).ok().map(Value::Num)),
        BinaryOp::Rem => num!(|p, q| number::rem(p, q).ok().map(Value::Num)),
        BinaryOp::Lt => num!(|p, q| Some(Value::Bool(p < q))),
        BinaryOp::Le => num!(|p, q| Some(Value::Bool(p <= q))),
        BinaryOp::Gt => num!(|p, q| Some(Value::Bool(p > q))),
        BinaryOp::Ge => num!(|p, q| Some(Value::Bool(p >= q))),
        BinaryOp::Eq => num!(|p, q| Some(Value::Bool(p == q))),
        BinaryOp::Ne => num!(|p, q| Some(Value::Bool(p != q))),
        _ => {
            let (x, y) = (child(code, x), child(code, y));
            mk(move |r| {
                let v = x(r)?;
                let w = y(r)?;
                r.operate(op, v, w, span)
            })
        }
    }
}

/// The closure of a numeric operator: when both operands are numbers and
/// `f` gives a result, 1 step and the result, as `Run::operate` does it;
/// else `Run::operate`.
fn numeric<F>(op: BinaryOp, a: Operand, b: Operand, span: Span, f: F) -> Code
where
    F: Fn(f64, f64) -> Option<Value<'static>> + Copy + Send + Sync + 'static,
{
    macro_rules! body {
        ($r:ident, $v:expr, $w:expr) => {
            mk(move |$r| {
                let v = $v;
                let w = $w;
                if let (Value::Num(p), Value::Num(q)) = (&v, &w) {
                    if let Some(z) = f(*p, *q) {
                        std::mem::forget(v);
                        std::mem::forget(w);
                        $r.charge(1)?;
                        return Ok(z);
                    }
                }
                $r.operate(op, v, w, span)
            })
        };
    }
    match (a, b) {
        (Operand::Slot(s), Operand::Num(c)) => body!(r, r.slots[s].dup(), Value::Num(c)),
        (Operand::Slot(s), Operand::Slot(t)) => body!(r, r.slots[s].dup(), r.slots[t].dup()),
        (Operand::Slot(s), Operand::Code(y)) => body!(r, r.slots[s].dup(), y(r)?),
        (Operand::Code(x), Operand::Num(c)) => body!(r, x(r)?, Value::Num(c)),
        (Operand::Code(x), Operand::Slot(t)) => body!(r, x(r)?, r.slots[t].dup()),
        (Operand::Num(c), Operand::Slot(t)) => body!(r, Value::Num(c), r.slots[t].dup()),
        (Operand::Num(c), Operand::Code(y)) => body!(r, Value::Num(c), y(r)?),
        (a, b) => {
            let x = into_code(a);
            let y = into_code(b);
            body!(r, x(r)?, y(r)?)
        }
    }
}

fn into_code(a: Operand) -> Code {
    match a {
        Operand::Num(c) => mk(move |_| Ok(Value::Num(c))),
        Operand::Slot(s) => mk(move |r| Ok(r.slots[s].dup())),
        Operand::Code(c) => c,
    }
}

/// A scalar written in place: an item of a list literal that `in` tests.
pub(crate) enum Scalar {
    Null,
    Bool(bool),
    Num(f64),
    Str(Box<str>),
}

fn scalar(ast: &Ast, n: NodeId) -> Option<Scalar> {
    Some(match ast.kind(n) {
        NodeKind::Null => Scalar::Null,
        NodeKind::Bool { value } => Scalar::Bool(*value),
        NodeKind::Number { value } => Scalar::Num(*value),
        NodeKind::String { value, .. } => Scalar::Str(value.as_str().into()),
        _ => return None,
    })
}

/// The items of a list literal with no spread.
fn literal_list(ast: &Ast, n: NodeId) -> Option<&[NodeId]> {
    match ast.kind(n) {
        NodeKind::List { items }
            if !items
                .iter()
                .any(|i| matches!(ast.kind(*i), NodeKind::Spread { .. })) =>
        {
            Some(items)
        }
        _ => None,
    }
}

/// A part of a template.
enum Part {
    Text(Box<str>),
    Expr(Code),
}

/// A `const` line of a program or a block.
enum Line {
    /// `const f = (...) => ...`.
    Lambda,
    /// `const name = value`, and the slot it binds (`None` for `_`).
    Name { value: Code, slot: Option<usize> },
    /// Another pattern, by the walk.
    Node(NodeId),
}

/// For each lambda whose parameters are all names with no default: the
/// slot each binds (`None` for `_`), indexed by node. A call binds them
/// without the walk of patterns.
pub(crate) fn params(ast: &Ast, res: &[Res]) -> Vec<Option<Box<[Option<u32>]>>> {
    ast.nodes
        .iter()
        .map(|n| {
            let NodeKind::Lambda { params, .. } = &n.kind else {
                return None;
            };
            let mut out = Vec::with_capacity(params.len());
            for p in params {
                let NodeKind::Param {
                    pattern,
                    default: None,
                } = ast.kind(*p)
                else {
                    return None;
                };
                let NodeKind::NamePat { .. } = ast.kind(*pattern) else {
                    return None;
                };
                out.push(match res[pattern.index()] {
                    Res::Bind(s) => Some(s),
                    _ => None,
                });
            }
            Some(out.into())
        })
        .collect()
}
