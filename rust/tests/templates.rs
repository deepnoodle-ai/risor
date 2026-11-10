//! Package F2: the binding surface of templates and analysis that the suite
//! does not reach.

use risor::{
    analyze, analyze_template, compile, parse, parse_template, Data, Env, Mode, Segment, Step,
    Value,
};
use std::collections::BTreeMap;

fn env() -> Env {
    Env::builder()
        .vars("{pr: {number: int}, tags: string[]}")
        .build()
        .unwrap()
}

fn vars() -> BTreeMap<String, Data> {
    let mut pr = BTreeMap::new();
    pr.insert("number".to_string(), Data::Int(42));
    let mut v = BTreeMap::new();
    v.insert("pr".to_string(), Data::Map(pr));
    v.insert(
        "tags".to_string(),
        Data::List(vec![Data::Str("a".into()), Data::Str("b".into())]),
    );
    v
}

fn run(p: risor::Parsed) -> String {
    let prog = compile(&env(), p).unwrap();
    format!("{:?}", prog.run(&vars()).result.unwrap())
}

#[test]
fn segments_and_single() {
    let t = parse_template("Review of #${{ pr.number }} $${{ x }}").unwrap();
    assert!(!t.is_single());
    assert_eq!(t.parts(), 1);
    match &t.segments()[0] {
        Segment::Text { value, .. } => assert_eq!(value, "Review of #"),
        s => panic!("{s:?}"),
    }
    match &t.segments()[2] {
        Segment::Text { value, .. } => assert_eq!(value, " ${{ x }}"),
        s => panic!("{s:?}"),
    }
    assert!(parse_template("${{ pr.number }}").unwrap().is_single());
    assert!(!parse_template(" ${{ pr.number }}").unwrap().is_single());
    assert!(parse_template("").unwrap().segments().is_empty());
}

#[test]
fn the_two_modes() {
    let t = parse_template("${{ tags }}").unwrap();
    let list = format!(
        "{:?}",
        Value::from_data(&Data::List(vec![
            Data::Str("a".into()),
            Data::Str("b".into())
        ]))
    );
    assert_eq!(run(t.program(Mode::Value)), list);
    let s = format!("{:?}", Value::from_data(&Data::Str("[\"a\",\"b\"]".into())));
    assert_eq!(run(t.program(Mode::String)), s);
    // Not single: a string in both modes.
    let t = parse_template("#${{ pr.number }}").unwrap();
    let want = format!("{:?}", Value::from_data(&Data::Str("#42".into())));
    assert_eq!(run(t.program(Mode::Value)), want);
    assert_eq!(run(t.program(Mode::String)), want);
}

#[test]
fn a_part_as_its_own_program() {
    let t = parse_template("a ${{ pr.number + 1 }} b ${{ len(tags) }}").unwrap();
    let p = t.part_program(1).unwrap();
    assert_eq!(run(p), format!("{:?}", Value::Num(2.0)));
    let p = t.part_program(0).unwrap();
    // Spans are in the whole text.
    let r = risor::check(&env(), &p, &risor::Expected::Nothing);
    assert!(r.diagnostics.is_empty());
    assert!(t.part_program(2).is_none());
}

#[test]
fn a_template_parse_error_is_in_the_whole_text() {
    let e = parse_template("ab\n${{ }}").err().unwrap();
    assert_eq!(e.diagnostics[0].code, "unexpected-token");
    assert_eq!(e.diagnostics[0].span.unwrap().0, [2, 5, 2, 7]);
}

#[test]
fn analysis_of_a_program() {
    let p = parse("const e = input.event; xs.map(x => lookup(x.id) + e.id + y)").unwrap();
    let r = analyze(&p);
    assert_eq!(r.names, ["input", "xs", "y"]);
    assert_eq!(r.refs[0].path, [Step::Key("event".into())]);
    let calls: Vec<(&str, Option<&str>, bool)> = r
        .calls
        .iter()
        .map(|c| (c.name.as_str(), c.namespace.as_deref(), c.lambda))
        .collect();
    assert_eq!(
        calls,
        [("xs.map", Some("xs"), false), ("lookup", None, true)]
    );
    assert!(r.literal.is_none());
    assert!(r.segments.is_none());
    let lit = analyze(&parse("{a: 1, b: [true, null], a: -2}").unwrap());
    assert_eq!(
        lit.literal.map(|v| risor::json::to_text(&v)),
        Some(r#"{"a":-2,"b":[true,null]}"#.to_string())
    );
}

#[test]
fn analysis_of_a_template() {
    let t = parse_template("Hi ${{ user.name }}, ${{ geo.distance(a, 1 + 2) }}").unwrap();
    let r = analyze_template(&t, Mode::String);
    assert_eq!(r.names, ["a", "geo", "user"]);
    assert_eq!(r.segments.as_ref().unwrap().len(), 4);
    assert_eq!(r.calls[0].name, "geo.distance");
    // An argument as a program of its own text, with spans in the whole text.
    let program = t.program(Mode::String);
    let arg = r.arg_program(&program, 0, 1).unwrap();
    let prog = compile(&Env::default(), arg).unwrap();
    let out = prog.run(&risor::NoVars).result.unwrap();
    assert_eq!(format!("{out:?}"), format!("{:?}", Value::Num(3.0)));
    // A lambda argument is not a program.
    let p = parse("f(x => x)").unwrap();
    assert!(analyze(&p).arg_program(&p, 0, 0).is_none());
    // An argument that reads a name the program binds does not compile alone.
    let p = parse("const k = 1; f(k)").unwrap();
    let r = analyze(&p);
    let arg = r.arg_program(&p, 0, 0).unwrap();
    assert!(compile(&Env::default(), arg).is_err());
}

/// 3.1 item 5: what does not open a level costs no stack. A long chain and a
/// long operator chain are analyzed on a small stack.
#[test]
fn deep_chains_without_host_recursion() {
    let chain = format!("a{}", ".b".repeat(50_000));
    let sum = vec!["x"; 50_000].join(" + ");
    let ors = format!("{} || q", vec!["p.c"; 20_000].join(" || "));
    let idx = format!("m{}", "[\"k\"]".repeat(20_000));
    let h = std::thread::Builder::new()
        .stack_size(256 * 1024)
        .spawn(move || {
            let p = parse(&chain).unwrap();
            let r = analyze(&p);
            assert_eq!(r.refs[0].path.len(), 50_000);
            let r = analyze(&parse(&sum).unwrap());
            assert_eq!(r.refs.len(), 50_000);
            let r = analyze(&parse(&ors).unwrap());
            assert_eq!(r.names, ["p", "q"]);
            let r = analyze(&parse(&idx).unwrap());
            assert_eq!(r.refs[0].path.len(), 20_000);
        })
        .unwrap();
    h.join().unwrap();
}
