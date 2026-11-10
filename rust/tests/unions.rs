//! Package F5: tagged unions, through the binding surface.

use risor::check::ty::{find_tag, print, Ty};
use risor::{Data, Env};
use std::collections::BTreeMap;

const SHAPE: &str = "{kind: \"circle\", r: float} | {kind: \"rect\", h: float, w: float}";

fn env() -> Env {
    Env::builder()
        .named_type("Shape", SHAPE)
        .vars("{s: Shape, blob: any}")
        .build()
        .unwrap()
}

fn shape_ty() -> Ty {
    env().ty("Shape").unwrap().ty
}

#[test]
fn a_union_prints_by_its_variants_and_finds_its_tag() {
    let t = shape_ty();
    assert_eq!(
        print(&t),
        "{kind: \"circle\", r: float} | {h: float, kind: \"rect\", w: float}"
    );
    let vs = t.variants();
    assert_eq!(find_tag(&vs).as_deref(), Some("kind"));
    // The record every read sees is flat(U).
    assert_eq!(
        print(&Ty::rec(t.record().unwrap().clone())),
        "{h?: float, kind: \"circle\" | \"rect\", r?: float, w?: float}"
    );
    // Records with no tag stay an error.
    assert!(env().ty("{a: int} | {b: string}").is_err());
    assert!(env().ty("Shape | Shape").is_err());
}

#[test]
fn the_walk_reads_the_tag_first() {
    use risor::conform::{test, Count};
    let t = shape_ty();
    let rect = |w: f64, h: Option<f64>| {
        let mut m = BTreeMap::new();
        m.insert("kind".to_string(), Data::Str("rect".into()));
        m.insert("w".to_string(), Data::Float(w));
        if let Some(h) = h {
            m.insert("h".to_string(), Data::Float(h));
        }
        risor::Value::from_data(&Data::Map(m))
    };
    let mut c = Count::default();
    assert!(test(&rect(2.0, Some(3.0)), &t, &mut c).unwrap().is_none());
    // F5 7.4: the map, `kind`, `h`, `w`.
    assert_eq!(c.0, 4);
    let mut c = Count::default();
    let f = test(&rect(2.0, None), &t, &mut c).unwrap().unwrap();
    assert_eq!(f.message(), "required field \"h\" is missing");
    assert_eq!(c.0, 3);
}

#[test]
fn a_test_of_the_tag_narrows() {
    let env = env();
    let p = risor::parse("s.kind == \"circle\" ? s.r : s.w * s.h").unwrap();
    let r = risor::check(&env, &p, &risor::Expected::Nothing);
    assert!(r.diagnostics.is_empty(), "{:?}", r.diagnostics);
    assert_eq!(print(r.ty.as_ref().unwrap()), "float");
    let p = risor::parse("s.r").unwrap();
    let r = risor::check(&env, &p, &risor::Expected::Nothing);
    assert_eq!(r.diagnostics[0].code, "maybe-absent");
}

#[test]
fn a_schema_union_round_trips_through_a_description() {
    let schema = risor::json::parse(
        r#"{"oneOf": [{"type": "object", "properties": {"kind": {"const": "a"}}, "required": ["kind"]},
                     {"type": "object", "properties": {"kind": {"const": "b"}}, "required": ["kind"]}]}"#,
    )
    .unwrap();
    let env = Env::builder()
        .var_schema("v", &schema, risor::SchemaOptions::default())
        .build()
        .unwrap();
    let d = env.describe();
    assert_eq!(
        d.get("variables").unwrap().get("v").unwrap().as_str(),
        Some("{kind: \"a\", ...} | {kind: \"b\", ...}")
    );
    let e2 = Env::from_description(&d).unwrap();
    assert_eq!(e2.description_text(), env.description_text());
}
