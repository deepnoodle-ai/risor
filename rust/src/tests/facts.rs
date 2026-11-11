//! Package F14, host facts: the binding's own rules (`f14-host-facts.md` 2.6,
//! 3.3, 9.2), which no suite case tests.

use crate::{check, parse, Env};

const E: &str = "{report: {verdict: string} | null, approve: {verdict: string} | null}";

fn check_codes(env: &Env, src: &str) -> Vec<&'static str> {
    let p = parse(src).unwrap();
    let r = check::check(env, &p, &check::Want::Nothing);
    r.diagnostics.iter().map(|d| d.code).collect()
}

fn base() -> Env {
    Env::builder()
        .var_record(E)
        .some_not_null(&["report", "approve"])
        .build()
        .unwrap()
}

#[test]
fn the_effect_needs_the_host_fact() {
    let env = base();
    assert!(check_codes(&env, "(report ?? approve).verdict").is_empty());
    let none = Env::builder().var_record(E).build().unwrap();
    assert_eq!(
        check_codes(&none, "(report ?? approve).verdict"),
        ["maybe-null"]
    );
}

#[test]
fn a_child_keeps_its_parents_host_facts() {
    let child = base().with().var_record("{n: int}").build().unwrap();
    assert!(check_codes(&child, "(report ?? approve).verdict").is_empty());
    assert_eq!(child.fact_names(), [["approve", "report"]]);
}

#[test]
fn a_replaced_member_drops_the_host_fact_whole() {
    let parent = base();
    let child = parent
        .with()
        .var("approve", "{verdict: string} | null")
        .build()
        .unwrap();
    assert!(child.fact_names().is_empty());
    assert_eq!(
        check_codes(&child, "(report ?? approve).verdict"),
        ["maybe-null"]
    );
    // The parent is not changed.
    assert!(check_codes(&parent, "(report ?? approve).verdict").is_empty());
    // A host function of the member's name replaces it too.
    let child = parent
        .with()
        .function("report", "() => int", |_, _| Ok(crate::Data::Int(0)))
        .build()
        .unwrap();
    assert!(child.fact_names().is_empty());
}

#[test]
fn a_child_declares_its_own_host_facts() {
    let parent = Env::builder().var_record(E).build().unwrap();
    // A parent's variable, and the child's own.
    let child = parent
        .with()
        .var("third", "{verdict: string} | null")
        .some_not_null(&["third", "report"])
        .build()
        .unwrap();
    assert!(check_codes(&child, "(report ?? third).verdict").is_empty());
    // A host fact that names a variable the child replaces names the
    // child's variable, and is kept.
    let child = base()
        .with()
        .var("approve", "{verdict: string} | null")
        .some_not_null(&["approve", "report"])
        .build()
        .unwrap();
    assert_eq!(child.fact_names(), [["approve", "report"]]);
    // Tested against the child's variables.
    let e = parent
        .with()
        .some_not_null(&["report", "nope"])
        .build()
        .err()
        .unwrap();
    assert_eq!(e.diagnostics[0].code, "bad-name");
}

#[test]
fn every_error_is_reported() {
    let e = Env::builder()
        .var_record(E)
        .some_not_null(&[])
        .some_not_null(&["report", "x", "report", "y"])
        .build()
        .err()
        .unwrap();
    let codes: Vec<&str> = e.diagnostics.iter().map(|d| d.code).collect();
    assert_eq!(codes, ["bad-name"; 4]);
}

#[test]
fn a_description_round_trips() {
    let env = base();
    let text = env.description_text();
    assert!(text.contains(r#""facts":[{"someNotNull":["approve","report"]}]"#));
    let back = Env::from_description(&text).unwrap();
    assert_eq!(back.description_text(), text);
    assert!(check_codes(&back, "(report ?? approve).verdict").is_empty());
    // An entry that names nothing is bad-name; one of the wrong form is the
    // binding's own error.
    let bad = text.replace(r#"["approve","report"]"#, r#"["approve","nope"]"#);
    assert!(matches!(
        Env::from_description(&bad),
        Err(crate::DescriptionError::Declare(_))
    ));
    let bad = text.replace(r#"{"someNotNull":["approve","report"]}"#, r#"["approve"]"#);
    assert!(matches!(
        Env::from_description(&bad),
        Err(crate::DescriptionError::Form(_))
    ));
}
