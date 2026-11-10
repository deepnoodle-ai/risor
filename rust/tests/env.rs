//! Removed library entries (`03-host.md` 3.3.1 item 6, rulings CD1 to CD6)
//! and child environments (3.3.2).

use risor::{check, parse, Env, Limits};

fn check_codes(env: &Env, src: &str) -> Vec<&'static str> {
    let p = parse(src).unwrap();
    let r = check::check(env, &p, &check::Expected::Nothing);
    r.diagnostics.iter().map(|d| d.code).collect()
}

#[test]
fn removed_entries_are_absent() {
    let env = Env::builder()
        .vars("{s: string, xs: int[]}")
        .remove("len")
        .remove("string.repeat")
        .remove("list.min")
        .build()
        .unwrap();
    assert_eq!(check_codes(&env, "len(s)"), ["undefined-function"]);
    assert_eq!(check_codes(&env, "s.repeat(2)"), ["no-method"]);
    assert_eq!(check_codes(&env, "xs.min()"), ["no-method"]);
    assert!(check_codes(&env, "min(1, 2)").is_empty());
    assert!(check_codes(&env, "s.slice(1)").is_empty());
    assert_eq!(env.removed_names(), ["len", "list.min", "string.repeat"]);
}

#[test]
fn a_bad_removed_name_is_bad_name() {
    for name in [
        "lenn",
        "repeat",
        "string.map",
        "number.abs",
        "list",
        "list.len",
    ] {
        let e = Env::builder().remove(name).build().err().unwrap();
        assert_eq!(e.diagnostics[0].code, "bad-name", "{name}");
    }
    assert!(Env::builder().remove("len").remove("len").build().is_ok());
}

#[test]
fn a_host_function_may_take_a_removed_name() {
    assert!(Env::builder()
        .declare_function("len", "(s: string) => int")
        .build()
        .is_err());
    let env = Env::builder()
        .remove("len")
        .declare_function("len", "(s: string) => int")
        .build()
        .unwrap();
    assert!(check_codes(&env, "len(\"a\")").is_empty());
}

#[test]
fn a_child_inherits_and_cannot_bring_back() {
    let parent = Env::builder()
        .vars("{a: int, b: string}")
        .remove("len")
        .reject(&["constant-condition", "dynamic"])
        .limits(Limits {
            steps: Some(10),
            ..Limits::default()
        })
        .build()
        .unwrap();
    let child = parent
        .child()
        .var("b", "bool")
        .remove("len")
        .remove("string.trim")
        .build()
        .unwrap();
    // The parent is unchanged.
    assert_eq!(parent.removed_names(), ["len"]);
    assert_eq!(child.removed_names(), ["len", "string.trim"]);
    assert_eq!(check_codes(&child, "len(\"a\")"), ["undefined-function"]);
    // The child's `b` replaces the parent's; `a` is kept.
    let d = child.describe();
    assert_eq!(
        risor::json::to_text(d.get("variables").unwrap()),
        r#"{"a":"int","b":"bool"}"#
    );
    // Rejected classes and limits come from the parent unless the child
    // gives its own.
    assert_eq!(child.data().reject, ["constant-condition", "dynamic"]);
    assert_eq!(child.data().limits.steps, Some(10));
    let own = parent.child().reject(&[]).build().unwrap();
    assert!(own.data().reject.is_empty());
}

#[test]
fn removed_entries_round_trip_through_a_description() {
    let env = Env::builder()
        .remove("map.keys")
        .remove("abs")
        .build()
        .unwrap();
    let d = env.describe();
    assert_eq!(
        risor::json::to_text(d.get("removed").unwrap()),
        r#"["abs","map.keys"]"#
    );
    let e2 = Env::from_description(&d).unwrap();
    assert_eq!(e2.description_text(), env.description_text());
    assert!(matches!(
        Env::from_description_text(&env.description_text().replace("abs", "abz")),
        Err(risor::DescriptionError::Declare(_))
    ));
}

#[test]
fn a_child_declaration_replaces_across_kinds() {
    let parent = Env::builder().vars("{a: int}").build().unwrap();
    let child = parent
        .child()
        .declare_function("a", "() => string")
        .build()
        .unwrap();
    assert!(child.data().vars.is_empty());
    assert_eq!(child.data().functions[0].name, "a");
    assert!(check_codes(&child, "a()").is_empty());
    // And back: a child variable replaces a parent function.
    let parent = Env::builder()
        .declare_function("f", "() => int")
        .build()
        .unwrap();
    let child = parent.child().var("f", "bool").build().unwrap();
    assert!(child.data().functions.is_empty());
    assert!(check_codes(&child, "f").is_empty());
}

/// Ruling CH2: the clash rules apply to the child's whole environment.
#[test]
fn a_child_clashes_with_what_it_inherits() {
    let parent = Env::builder()
        .declare_function("geo.distance", "(a: string) => float")
        .build()
        .unwrap();
    let e = parent.child().var("geo", "int").build().err().unwrap();
    assert_eq!(e.diagnostics[0].code, "name-clash");
    let e = parent
        .child()
        .declare_function("geo", "() => int")
        .build()
        .err()
        .unwrap();
    assert_eq!(e.diagnostics[0].code, "name-clash");
    let parent = Env::builder().vars("{geo: int}").build().unwrap();
    let e = parent
        .child()
        .declare_function("geo.x", "() => int")
        .build()
        .err()
        .unwrap();
    assert_eq!(e.diagnostics[0].code, "name-clash");
}

#[test]
fn a_child_limits_are_its_own_when_given() {
    let parent = Env::builder()
        .limits(Limits {
            steps: Some(10),
            ..Limits::default()
        })
        .build()
        .unwrap();
    let child = parent
        .child()
        .limits(Limits {
            depth: 5,
            ..Limits::default()
        })
        .build()
        .unwrap();
    assert_eq!(child.data().limits.depth, 5);
    assert_eq!(child.data().limits.steps, Some(1_000_000));
    assert_eq!(parent.data().limits.steps, Some(10));
    assert_eq!(
        parent.child().build().unwrap().data().reject,
        ["constant-condition"]
    );
}

/// Ruling CH1: a `host` site counts only on the failing call itself.
#[test]
fn a_host_site_on_an_argument_is_not_the_outer_calls() {
    let env = Env::builder()
        .function("rate", "(code: string) => string !", |_, _| {
            Ok(risor::Data::Str("x".into()))
        })
        .function("geo.distance", "(a: string, b: string) => float", |_, _| {
            Err(risor::HostError("far".into()))
        })
        .build()
        .unwrap();
    let p = risor::compile(&env, parse(r#"geo.distance(rate("x"), "b")"#).unwrap()).unwrap();
    let f = p.run(&risor::NoVars).result.err().unwrap();
    let d = p.diagnostic(&f);
    assert_eq!((d.code, d.class), ("function-error", Some("environment")));
    // The marked call itself is at its own `host` site.
    let env = Env::builder()
        .function("rate", "(code: string) => string !", |_, _| {
            Err(risor::HostError("no".into()))
        })
        .build()
        .unwrap();
    let p = risor::compile(&env, parse(r#"rate("x")"#).unwrap()).unwrap();
    let f = p.run(&risor::NoVars).result.err().unwrap();
    assert_eq!(p.diagnostic(&f).class, Some("host"));
}

/// Ruling CH3: a child replaces only in the same space of names. A child
/// variable `Time` leaves an inherited opaque type `Time` and its methods.
#[test]
fn a_child_variable_does_not_replace_an_opaque_type() {
    let parent = Env::builder()
        .opaque("Time")
        .method_impl("Time.format", "(layout: string) => string", None)
        .var("t", "Time")
        .build()
        .unwrap();
    let child = parent.child().var("Time", "int").build().unwrap();
    assert_eq!(child.data().opaque, ["Time"]);
    assert_eq!(child.data().methods.len(), 1);
    assert!(check_codes(&child, "Time + 1").is_empty());
    assert!(check_codes(&child, "t.format(\"x\")").is_empty());
}

/// Ruling CH4: a child's opaque type replaces the parent's whole; a child's
/// method alone adds or replaces one method.
#[test]
fn a_child_opaque_type_replaces_the_parents_whole() {
    let parent = Env::builder()
        .opaque("Time")
        .method_impl("Time.format", "(layout: string) => string", None)
        .method_impl("Time.unix", "() => int", None)
        .build()
        .unwrap();
    let child = parent
        .child()
        .opaque("Time")
        .method_impl("Time.year", "() => int", None)
        .build()
        .unwrap();
    let names: Vec<&str> = child
        .data()
        .methods
        .iter()
        .map(|m| m.name.as_str())
        .collect();
    assert_eq!(names, ["year"]);
    let child = parent
        .child()
        .method_impl("Time.unix", "() => float", None)
        .method_impl("Time.year", "() => int", None)
        .build()
        .unwrap();
    let mut got: Vec<String> = child
        .data()
        .methods
        .iter()
        .map(|m| format!("{} {}", m.name, risor::print_sig(&m.sig, &[])))
        .collect();
    got.sort();
    assert_eq!(
        got,
        [
            "format (layout: string) => string",
            "unix () => float",
            "year () => int"
        ]
    );
    // The parent is unchanged.
    assert_eq!(parent.data().methods.len(), 2);
}
