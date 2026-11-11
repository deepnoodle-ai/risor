//! Package F1: descriptions of types and environments, and the round trip
//! of `f01-host-types.md` 5.4.

use crate::check::{self, ty::print};
use crate::json;
use crate::testing::{case_env, case_files, load_file};
use crate::{parse, Env, Limits, SchemaOptions};
use std::path::PathBuf;

fn suite_dir() -> PathBuf {
    PathBuf::from(env!("CARGO_MANIFEST_DIR")).join("../suite")
}

/// What a check finds, as text: the type, the sites and the diagnostics.
fn check_text(env: &Env, source: &str, returns: Option<&str>) -> String {
    let Ok(parsed) = parse(source) else {
        return "parse".into();
    };
    let exp = match check::expected(env, returns) {
        Ok(x) => x,
        Err(d) => return format!("expected: {}", d.code),
    };
    let r = check::check(env, &parsed, &exp);
    let ty = r.ty.as_ref().map(print).unwrap_or_default();
    let sites: Vec<String> = r
        .sites
        .iter()
        .map(|s| format!("{} {:?}", s.class, parsed.source.line_span(s.span).arr()))
        .collect();
    let diags: Vec<String> = r
        .diagnostics
        .iter()
        .map(|d| format!("{} {:?} {}", d.code, d.span.map(|s| s.arr()), d.message))
        .collect();
    format!("{ty} | {} | {}", sites.join(", "), diags.join(", "))
}

/// 5.4 item 3, over every case of the suite whose environment builds: the
/// environment built from a description has the same description and
/// checks every program the same way.
#[test]
fn round_trip_over_the_suite() {
    let mut envs = 0;
    for dir in [
        "core-run",
        "core-check",
        "host-types",
        "strings",
        "conversion",
    ] {
        for path in case_files(&suite_dir().join(dir)) {
            let Ok(file) = load_file(&path) else { continue };
            for case in &file.cases {
                let Ok(e1) = case_env(case) else { continue };
                let e2 = Env::from_description(&e1.description_text())
                    .unwrap_or_else(|e| panic!("{}: {}: {e}", path.display(), case.name));
                assert_eq!(
                    e2.description_text(),
                    e1.description_text(),
                    "{}: {}",
                    path.display(),
                    case.name
                );
                // The text form reads back to the same description.
                let e3 = Env::from_description(&e1.description_text()).unwrap();
                assert_eq!(e3.description_text(), e1.description_text());
                if case.check {
                    assert_eq!(
                        check_text(&e2, &case.source, case.returns.as_deref()),
                        check_text(&e1, &case.source, case.returns.as_deref()),
                        "{}: {}",
                        path.display(),
                        case.name
                    );
                }
                envs += 1;
            }
        }
    }
    assert!(envs > 4000, "only {envs} environments were described");
}

#[test]
fn the_text_of_the_chapter_example() {
    let schema = json::parse(
        r#"{"type": "object", "properties": {"a": {"type": "string"}}, "required": ["a"]}"#,
    )
    .unwrap();
    let env = Env::builder()
        .var_schema_json("order", &schema, SchemaOptions::default())
        .function("rate", "(code:string)=>float!", |_, _| {
            Ok(crate::Data::Float(1.0))
        })
        .build()
        .unwrap();
    assert_eq!(
        env.description_text(),
        r#"{"functions":{"rate":"(code: string) => float !"},"limits":{"allocation":1000000,"depth":200,"steps":1000000},"opaque":{},"reject":["constant-condition"],"removed":[],"variables":{"order":"{a: string, ...}"},"version":"3.0"}"#
    );
}

#[test]
fn limits_that_are_off_are_null() {
    let env = Env::builder()
        .limits(Limits {
            steps: None,
            allocation: None,
            depth: 7,
            deadline: Some(std::time::Duration::from_secs(1)),
        })
        .build()
        .unwrap();
    let d = env.description();
    assert_eq!(
        json::to_text(d.get("limits").unwrap()),
        r#"{"allocation":null,"depth":7,"steps":null}"#
    );
    let e2 = Env::from_description(&env.description_text()).unwrap();
    assert_eq!(e2.data().limits.steps, None);
    assert_eq!(e2.data().limits.depth, 7);
    // The deadline is not described.
    assert_eq!(e2.data().limits.deadline, None);
}

#[test]
fn a_description_holds_a_type_of_depth_64() {
    let deep = format!("int{}", "[]".repeat(64));
    let text = format!(
        r#"{{"version":"3.0","variables":{{"v":"{deep}"}},"functions":{{}},"opaque":{{}},"removed":[],"reject":[],"limits":{{"steps":null,"allocation":null,"depth":200}}}}"#
    );
    let env = Env::from_description(&text).unwrap();
    assert_eq!(check_text(&env, "v", None), format!("{deep} |  | "));
}

fn declare_codes(text: &str) -> Vec<&'static str> {
    match Env::from_description(text) {
        Err(e) if e.first().code == "bad-description" => panic!("form error: {e}"),
        Err(e) => {
            let mut c: Vec<&'static str> = e.diagnostics.iter().map(|d| d.code).collect();
            c.sort();
            c
        }
        Ok(_) => Vec::new(),
    }
}

fn with(key: &str, value: &str) -> String {
    let mut d: Vec<(String, String)> = vec![
        ("version".into(), r#""3.0""#.into()),
        ("variables".into(), "{}".into()),
        ("functions".into(), "{}".into()),
        ("opaque".into(), "{}".into()),
        ("removed".into(), "[]".into()),
        ("reject".into(), "[]".into()),
        (
            "limits".into(),
            r#"{"steps":1,"allocation":1,"depth":1}"#.into(),
        ),
    ];
    for e in d.iter_mut() {
        if e.0 == key {
            e.1 = value.into();
        }
    }
    let parts: Vec<String> = d.iter().map(|(k, v)| format!("\"{k}\":{v}")).collect();
    format!("{{{}}}", parts.join(","))
}

#[test]
fn errors_of_a_description() {
    assert_eq!(
        declare_codes(&with("version", r#""9.9""#)),
        ["unsupported-version"]
    );
    assert_eq!(
        declare_codes(&with("variables", r#"{"v":"Order"}"#)),
        ["bad-type"]
    );
    assert_eq!(
        declare_codes(&with("variables", r#"{"if":"int"}"#)),
        ["bad-name"]
    );
    assert_eq!(
        declare_codes(&with("functions", r#"{"f":"(x: Foo) => int"}"#)),
        ["bad-signature"]
    );
    assert_eq!(
        declare_codes(
            &with("variables", r#"{"f":"int"}"#)
                .replace(r#""functions":{}"#, r#""functions":{"f":"() => int"}"#)
        ),
        ["name-clash"]
    );
    assert_eq!(declare_codes(&with("reject", r#"["nope"]"#)), ["bad-class"]);
    // `f01-host-types.md` 5.4 item 2: a negative or fractional limit is `bad-limit`.
    for l in [
        r#"{"steps":-1,"allocation":1,"depth":1}"#,
        r#"{"steps":1,"allocation":1.5,"depth":1}"#,
        r#"{"steps":1,"allocation":1,"depth":0}"#,
        r#"{"steps":1,"allocation":1,"depth":1001}"#,
        r#"{"steps":1,"allocation":1,"depth":2.5}"#,
        r#"{"steps":9007199254740992,"allocation":1,"depth":1}"#,
        r#"{"steps":1,"allocation":9007199254740992,"depth":1}"#,
    ] {
        assert_eq!(declare_codes(&with("limits", l)), ["bad-limit"], "{l}");
    }
    for l in [
        Limits {
            steps: Some(1 << 53),
            ..Limits::default()
        },
        Limits {
            allocation: Some(1 << 53),
            ..Limits::default()
        },
    ] {
        let e = Env::builder().limits(l).build().err().unwrap();
        assert_eq!(e.diagnostics[0].code, "bad-limit");
    }
    // A wrong JSON kind or a missing key is the binding's own error.
    for (k, v) in [
        ("limits", r#"{"steps":"1","allocation":1,"depth":1}"#),
        ("limits", r#"{"steps":1,"depth":1}"#),
        ("limits", r#"{"steps":1,"allocation":1,"depth":null}"#),
        ("variables", r#"{"v":5}"#),
        ("reject", r#""dynamic""#),
        ("opaque", r#"{"Time":[]}"#),
    ] {
        assert!(
            matches!(
                Env::from_description(&with(k, v)),
                Err(e) if e.first().code == "bad-description"
            ),
            "{k}: {v}"
        );
    }
    assert!(matches!(
        Env::from_description(r#"{"version":"3.0"}"#),
        Err(e) if e.first().code == "bad-description"
    ));
    assert!(matches!(
        Env::from_description(&with("version", r#""3.0","extra":1"#)),
        Err(e) if e.first().code == "bad-description"
    ));
}

#[test]
fn a_function_from_a_description_has_no_implementation() {
    let env = Env::from_description(&with(
        "functions",
        r#"{"rate":"(code: string) => float !"}"#,
    ))
    .unwrap();
    // It checks.
    assert_eq!(
        check_text(&env, r#"rate("x")"#, None),
        "float | host [1, 1, 1, 10] | "
    );
    // A compile fails with `no-implementation`.
    let e = env
        .compile(&parse("1").unwrap(), Default::default())
        .err()
        .unwrap();
    assert_eq!(e.diagnostics[0].code, "no-implementation");
}

#[test]
fn a_schema_variable_reads_its_value_in_place() {
    let schema = json::parse(
        r#"{"type": "object", "properties": {"id": {"type": "integer"}, "tags": {"type": "array", "items": {"type": "string"}}}, "required": ["id"]}"#,
    )
    .unwrap();
    let env = Env::builder()
        .var_schema_json("order", &schema, SchemaOptions::default())
        .build()
        .unwrap();
    let x = parse("order.id + len(order?.tags ?? [])").unwrap();
    let p = env.compile(&x, Default::default()).unwrap();
    let mut m = std::collections::BTreeMap::new();
    let mut o = std::collections::BTreeMap::new();
    o.insert("id".to_string(), crate::Data::Int(1));
    m.insert("order".to_string(), crate::Data::Map(o));
    assert_eq!(
        format!("{:?}", p.run(&m).unwrap()),
        format!("{:?}", crate::Value::Num(1.0))
    );
}

#[test]
fn a_bad_schema_is_bad_type() {
    let e = Env::builder()
        .var_schema("v", "\"string\"", SchemaOptions::default())
        .build()
        .err()
        .unwrap();
    assert_eq!(e.diagnostics[0].code, "bad-type");
    assert!(e.diagnostics[0].span.is_none());
}
