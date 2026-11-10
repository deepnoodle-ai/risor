//! Package F12: completion and hover beyond the suite's cases.

use risor::assist::{complete, hover};
use risor::check::Expected;
use risor::Env;

fn env() -> Env {
    Env::builder()
        .vars("{user: {name: string}, n: int}")
        .build()
        .unwrap()
}

fn labels(text: &str) -> Vec<String> {
    let at = text.chars().position(|c| c == '‸').unwrap();
    let src = text.replace('‸', "");
    complete(&env(), &src, &Expected::Nothing, at)
        .items
        .into_iter()
        .map(|c| c.label)
        .collect()
}

#[test]
fn a_text_block_interpolation_left_open() {
    assert_eq!(labels("const s = \"\"\"\n  a ${us‸"), ["user"]);
    assert_eq!(labels("`${user.na‸"), ["name"]);
}

#[test]
fn the_repair_closes_what_is_open() {
    assert_eq!(labels("if (us‸"), ["user"]);
    assert_eq!(labels("match n { 1 => us‸"), ["user"]);
    assert_eq!(labels("try { 1 } catch (e) { e.co‸"), ["code"]);
    assert_eq!(labels("[1, 2].map((x, i) => x + i‸"), ["i", "int"]);
}

#[test]
fn positions_outside_the_text() {
    let c = complete(&env(), "us", &Expected::Nothing, 9);
    assert!(c.items.is_empty());
    assert!(hover(&env(), "us", &Expected::Nothing, 9).is_none());
}

#[test]
fn a_long_text_is_helped() {
    let text = format!("{}us", "n + ".repeat(2000));
    let c = complete(&env(), &text, &Expected::Nothing, text.chars().count());
    assert_eq!(c.items.len(), 1);
}

/// F12 3.12: a lambda checked more than once, as a `const` lambda is at
/// each use, gives a parameter the type of the first check that reaches the
/// position. Completion never reads past the cut, so this is tested on the
/// check that assist reads.
#[test]
fn a_parameter_takes_the_first_check() {
    use risor::compile::Res;
    use risor::syntax::ast::NodeKind;
    let p = risor::parse("const f = x => x;\n[f(1), f(\"a\")]").unwrap();
    // The `x` of the body: the second `Ident` named x.
    let id = p
        .ast
        .nodes
        .iter()
        .enumerate()
        .filter(|(_, n)| matches!(&n.kind, NodeKind::Ident { name } if name == "x"))
        .map(|(i, _)| risor::syntax::ast::NodeId(i as u32))
        .last()
        .unwrap();
    let pr = risor::check::check_probe(&env(), &p, &Expected::Nothing, Some(id));
    let Res::Slot(s) = pr.res[id.index()] else {
        panic!("x reads a slot");
    };
    let at = pr.at.expect("the check reached x");
    assert_eq!(risor::check::ty::print(&at[s as usize]), "int");
    assert_eq!(risor::check::ty::print(&pr.slots[s as usize]), "\"a\"");
}
