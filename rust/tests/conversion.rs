//! Package F4: the binding surface the suite does not reach.

use risor::{Data, Env};
use std::collections::BTreeMap;

fn order(id: Data, lines: Vec<Data>) -> Data {
    let mut m = BTreeMap::new();
    m.insert("id".to_string(), id);
    m.insert("lines".to_string(), Data::List(lines));
    Data::Map(m)
}

fn line(sku: &str, qty: Data) -> Data {
    let mut m = BTreeMap::new();
    m.insert("sku".to_string(), Data::Str(sku.into()));
    m.insert("qty".to_string(), qty);
    Data::Map(m)
}

fn env() -> Env {
    Env::builder()
        .named_type("Line", "{sku: string, qty: int}")
        .named_type("Order", "{id: string, lines: Line[], ...}")
        .build()
        .unwrap()
}

#[test]
fn the_conformance_function() {
    let t = env().ty("Order").unwrap();
    assert!(t
        .conform(&order(Data::Str("a".into()), vec![line("x", Data::Int(1))]))
        .is_ok());
    let bad = order(
        Data::Str("a".into()),
        vec![line("x", Data::Int(1)), line("y", Data::Str("2".into()))],
    );
    let d = t.conform(&bad).err().unwrap();
    assert_eq!(d.code, "not-conforming");
    assert_eq!(d.class, Some("value"));
    assert!(d.span.is_none());
    assert_eq!(d.message, "expected int, found \"2\"");
    use risor::diag::PathStep::{Index, Key};
    assert_eq!(
        d.path.as_deref().unwrap().as_slice(),
        [Key("lines".into()), Index(1), Key("qty".into())]
    );
    // A type that names nothing declared is `bad-type`.
    assert_eq!(
        env().ty("Item").err().unwrap().diagnostics[0].code,
        "bad-type"
    );
}

#[test]
fn the_walk_counts_steps() {
    use risor::conform::{test, Count};
    let t = env().ty("Order").unwrap();
    let v = risor::Value::from_data(&order(
        Data::Str("a".into()),
        vec![line("x", Data::Int(1)), line("y", Data::Str("2".into()))],
    ));
    let mut c = Count::default();
    assert!(test(&v, &t.ty, &mut c).unwrap().is_some());
    // 9.7 item 2: the map, id, lines, 3 for the first line, then the
    // second line and its qty.
    assert_eq!(c.0, 8);
}

#[test]
fn named_types_in_order_and_in_the_description() {
    // A name used before it is declared is `bad-type`.
    let e = Env::builder()
        .named_type("Aa", "Bb[]")
        .named_type("Bb", "int")
        .build()
        .err()
        .unwrap();
    assert_eq!(e.diagnostics[0].code, "bad-type");
    let env = env();
    let d = env.describe();
    assert_eq!(
        risor::json::to_text(d.get("types").unwrap()),
        r#"{"Line":"{qty: int, sku: string}","Order":"{id: string, lines: {qty: int, sku: string}[], ...}"}"#
    );
    let e2 = Env::from_description(&d).unwrap();
    assert_eq!(e2.description_text(), env.description_text());
    // No named type: no key `types`.
    assert!(Env::default().describe().get("types").is_none());
}

#[test]
fn a_doubling_chain_of_named_types_is_shared() {
    let mut b = Env::builder().named_type("T0", "{l: int, r: int}");
    for i in 1..20 {
        b = b.named_type(
            &format!("T{i}"),
            &format!("{{l: T{p}, r: T{p}}}", p = i - 1),
        );
    }
    let start = std::time::Instant::now();
    let e = b.build().err().unwrap();
    // T15 passes the size limit (F4 3.2); the later ones name it.
    assert!(e.diagnostics.iter().all(|d| d.code == "bad-type"));
    assert!(start.elapsed().as_secs() < 2);
}

#[test]
fn as_and_is_in_a_program() {
    let env = Env::builder()
        .vars("{body: any}")
        .named_type("Order", "{id: string, total: float, ...}")
        .build()
        .unwrap();
    let p = risor::compile(
        &env,
        risor::parse("try { body as Order } catch (err) { err.path }").unwrap(),
    )
    .unwrap();
    let mut vars = BTreeMap::new();
    let mut m = BTreeMap::new();
    m.insert("id".to_string(), Data::Int(1));
    vars.insert("body".to_string(), Data::Map(m));
    let out = p.run(&vars).result.unwrap();
    assert_eq!(out.to_data(), Data::List(vec![Data::Str("id".into())]));
}
