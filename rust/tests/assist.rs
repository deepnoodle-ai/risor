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
