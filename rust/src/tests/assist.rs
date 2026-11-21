//! Package F12: completion and hover beyond the suite's cases.

use crate::assist::{complete, hover};
use crate::check::Want as Expected;
use crate::Env;

fn env() -> Env {
    Env::builder()
        .var_record("{user: {name: string}, n: int}")
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

/// The cut is in a template that is the argument of a call left open. The
/// repair closes the template and then fails at the closer of the outer
/// interpolation, where the call still needs its `)`. No token inserted
/// before the inner closer can mend that, so the repair fails at once, and
/// completion gives the fallback (3.5 item 1): the environment's names, as
/// if the program bound none, so `xy` is not listed.
#[test]
fn a_failure_after_a_template_closer_ends_the_repair() {
    assert_eq!(labels("`${f(`${us‸"), ["user"]);
    assert!(labels("const xy = 1; `${f(`${x‸").is_empty());
    // With no call left open, the same cut is repaired.
    assert_eq!(labels("const xy = 1; `${xy + `${x‸"), ["xy"]);
}

#[test]
fn the_repair_closes_what_is_open() {
    assert_eq!(labels("if (us‸"), ["user"]);
    assert_eq!(labels("match n { 1 => us‸"), ["user"]);
    assert_eq!(labels("try { 1 } catch (e) { e.co‸"), ["code"]);
    assert_eq!(labels("[1, 2].map((x, i) => x + i‸"), ["i", "int"]);
}

/// F12 3.6 item 1 (R255): the own name of a `const`-bound lambda is listed
/// with the names of its `const` line's scope, at its place in the order
/// bound, typed or not.
#[test]
fn a_lambda_own_name_is_in_the_scope_of_its_line() {
    assert_eq!(
        labels("const qg = 1; const qf = (qn: int): int => { const qh = 2; q‸"),
        ["qh", "qn", "qg", "qf"]
    );
    assert_eq!(
        labels("const qg = 1; const qf = qn => 1; const qh = 2; const qk = qx => q‸"),
        ["qx", "qg", "qf", "qh", "qk"]
    );
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
    use crate::compile::Res;
    use crate::syntax::ast::NodeKind;
    let p = crate::parse("const f = x => x;\n[f(1), f(\"a\")]").unwrap();
    // The `x` of the body: the second `Ident` named x.
    let id = p
        .ast
        .nodes
        .iter()
        .enumerate()
        .filter(|(_, n)| matches!(&n.kind, NodeKind::Ident { name } if name == "x"))
        .map(|(i, _)| crate::syntax::ast::NodeId(i as u32))
        .next_back()
        .unwrap();
    let pr = crate::check::check_probe(&env(), &p, &Expected::Nothing, Some(id));
    let Res::Slot(s) = pr.res[id.index()] else {
        panic!("x reads a slot");
    };
    let at = pr.at.expect("the check reached x");
    assert_eq!(crate::check::ty::print(&at[s as usize]), "int");
    assert_eq!(crate::check::ty::print(&pr.slots[s as usize]), "\"a\"");
}

/// F12 3.11 item 1 with F17 7.2 (R231): a string another arm with no guard
/// lists is left out, and so is one an earlier literal of the hole's own arm
/// lists, guard or not. A guarded other arm leaves out none, and a later
/// literal of the hole's own arm none.
#[test]
fn the_strings_of_a_match_arm() {
    let env = Env::builder()
        .var_record("{s: \"a\" | \"b\" | \"c\", c: bool}")
        .build()
        .unwrap();
    let labels = |text: &str| -> Vec<String> {
        let at = text.chars().position(|c| c == '‸').unwrap();
        let src = text.replace('‸', "");
        complete(&env, &src, &Expected::Nothing, at)
            .items
            .into_iter()
            .map(|c| c.label)
            .collect()
    };
    assert_eq!(labels("match s { \"a\" => 1, \"‸"), ["b", "c"]);
    assert_eq!(labels("match s { \"a\" | \"‸"), ["b", "c"]);
    assert_eq!(labels("match s { \"a\" if (c) => 1, \"‸"), ["a", "b", "c"]);
    assert_eq!(
        labels("match s { \"a\" if (c) => 1, \"b\" => 2, \"‸"),
        ["a", "c"]
    );
    // The own arm's earlier literal, the arm guarded.
    assert_eq!(labels("match s { \"a\" | \"‸\" if (c) => 1 }"), ["b", "c"]);
    // A later literal of the own arm is not an earlier one.
    assert_eq!(labels("match s { \"‸\" | \"a\" => 1 }"), ["a", "b", "c"]);
}
