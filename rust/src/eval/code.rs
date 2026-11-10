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

use super::{Run, R};
use crate::compile::Res;
use crate::source::Span;
use crate::syntax::ast::{Ast, BinaryOp, NodeId, NodeKind, TemplatePart};
use crate::value::{Str, Value};
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
pub(crate) fn build(ast: &Ast, res: &[Res]) -> Vec<Option<Code>> {
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
            node(ast, res, id, &code)
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

fn node(ast: &Ast, res: &[Res], id: NodeId, code: &[Option<Code>]) -> Option<Code> {
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
                mk(move |r| Ok(r.slots[s].clone()))
            }
            Res::Var(v) => {
                let v = v as usize;
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
            let y_id = *y;
            let (x, y) = (child(code, *x), child(code, *y));
            match *op {
                BinaryOp::And => mk(move |r| {
                    let v = x(r)?;
                    if v.truthy() {
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
                        y(r)
                    }
                }),
                BinaryOp::Coalesce => mk(move |r| {
                    let v = x(r)?;
                    if v.is_null() {
                        y(r)
                    } else {
                        Ok(v)
                    }
                }),
                // `x in [a, b, ...]`: the list's elements are tested where
                // they are, and the list itself is never built. Its charge
                // stands where the literal makes it.
                op @ (BinaryOp::In | BinaryOp::NotIn) if literal_list(ast, y_id).is_some() => {
                    let items: Box<[Code]> = literal_list(ast, y_id)
                        .unwrap_or_default()
                        .iter()
                        .map(|&i| child(code, i))
                        .collect();
                    mk(move |r| {
                        let v = x(r)?;
                        let found = r.in_literal(&v, &items, span)?;
                        Ok(Value::Bool(found == (op == BinaryOp::In)))
                    })
                }
                op => mk(move |r| {
                    let v = x(r)?;
                    let w = y(r)?;
                    r.operate(op, v, w, span)
                }),
            }
        }
        NodeKind::Cond {
            test, then, else_, ..
        } => {
            let (t, a, b) = (child(code, *test), child(code, *then), child(code, *else_));
            mk(move |r| if t(r)?.truthy() { a(r) } else { b(r) })
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
                            if let Some(s) = slot {
                                r.slots[*s] = v;
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
                let mut out = std::mem::take(&mut r.scratch);
                out.clear();
                for p in parts.iter() {
                    match p {
                        Part::Text(t) => out.push_str(t),
                        Part::Expr(e) => {
                            let v = e(r)?;
                            r.render(&v, &mut out, true, span)?;
                        }
                    }
                }
                r.template_done(out)
            })
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
