//! The derive of F1 6.3: the type of a struct, and the presentation of its
//! values, which must agree.

use risor::{type_of, Data, Env, HostMap, HostType, Opaque, OpaqueType, View};
use risor_derive::Host;
use std::collections::{BTreeMap, HashMap};

fn text<T: HostType + ?Sized>() -> String {
    type_of::<T>()
        .map(|t| t.to_string())
        .unwrap_or_else(|e| format!("error: {e}"))
}

#[derive(Host)]
struct Order {
    id: u64,
    #[risor(rename = "customer-name")]
    customer: String,
    #[risor(absent_if_none)]
    note: Option<String>,
    discount: Option<f64>,
    tags: Vec<String>,
    #[risor(skip)]
    #[allow(dead_code)]
    secret: std::cell::Cell<u8>,
    lines: Vec<Line>,
    attrs: BTreeMap<String, Data>,
    r#type: &'static str,
}

#[derive(Host)]
pub struct Line {
    pub sku: String,
    pub(crate) qty: i32,
}

#[test]
fn the_type_of_a_struct() {
    assert_eq!(text::<Line>(), "{qty: int, sku: string}");
    assert_eq!(
        text::<Order>(),
        "{attrs: map<any>, \"customer-name\": string, discount: float | null, id: int, \
         lines: {qty: int, sku: string}[], note?: string, tags: string[], type: string}"
    );
}

fn order(note: Option<&str>) -> Order {
    Order {
        id: 7,
        customer: "Ada".into(),
        note: note.map(String::from),
        discount: None,
        tags: vec!["a".into()],
        secret: std::cell::Cell::new(1),
        lines: vec![Line {
            sku: "x".into(),
            qty: 2,
        }],
        attrs: BTreeMap::new(),
        r#type: "web",
    }
}

#[test]
fn a_value_is_presented_as_its_type_says() {
    let o = order(None);
    let mut keys = o.keys();
    keys.sort();
    assert_eq!(
        keys,
        [
            "attrs",
            "customer-name",
            "discount",
            "id",
            "lines",
            "tags",
            "type"
        ]
    );
    assert!(o.get("note").is_none(), "absent when None");
    assert!(
        o.get("secret").is_none(),
        "a skipped field is not presented"
    );
    assert!(o.get("customer").is_none(), "presented under its key only");
    assert!(matches!(o.get("discount"), Some(View::Null)));
    assert!(matches!(o.get("customer-name"), Some(View::Str("Ada"))));
    let o = order(Some("hi"));
    assert_eq!(o.len(), 8);
    assert!(matches!(o.get("note"), Some(View::Str("hi"))));
}

#[test]
fn a_derived_variable_checks_and_runs() {
    let env = Env::builder().var_host::<Order>("order").build().unwrap();
    let Data::Map(d) = env.describe() else {
        panic!("a description is a map")
    };
    let Some(Data::Map(vars)) = d.get("variables") else {
        panic!("variables")
    };
    assert_eq!(vars.get("order"), Some(&Data::Str(text::<Order>())));
    let src = r#"`${order["customer-name"]}: ${len(order.lines)} ${order?.note ?? "none"} ${order.type}`"#;
    let p = env
        .compile(&risor::parse(src).unwrap(), Default::default())
        .unwrap();
    let mut vars: HashMap<String, Order> = HashMap::new();
    vars.insert("order".into(), order(None));
    assert_eq!(
        format!("{:?}", p.run(&vars).unwrap()),
        format!(
            "{:?}",
            risor::Value::from_data(&Data::Str("Ada: 1 none web".into()))
        )
    );
    // The record is exact: a key it does not list is `no-field`.
    let e = env
        .compile(&risor::parse("order.secret").unwrap(), Default::default())
        .err()
        .unwrap();
    assert_eq!(e.diagnostics[0].code, "no-field");
    // `note` is optional.
    let e = env
        .compile(&risor::parse("order.note").unwrap(), Default::default())
        .err()
        .unwrap();
    assert_eq!(e.diagnostics[0].code, "maybe-absent");
}

struct Time(u64);
impl OpaqueType for Time {
    const NAME: &'static str = "Time";
}

#[derive(Host)]
struct Event {
    at: Opaque<Time>,
    later: Option<Opaque<Time>>,
}

#[test]
fn an_opaque_field_names_its_registered_type() {
    assert_eq!(text::<Event>(), "{at: Time, later: Time | null}");
    // The environment must declare it.
    let e = Env::builder().var_host::<Event>("e").build().err().unwrap();
    assert_eq!(e.diagnostics[0].code, "bad-type");
    let env = Env::builder()
        .opaque("Time")
        .var_host::<Event>("e")
        .build()
        .unwrap();
    let p = env
        .compile(
            &risor::parse("e.at == e.later").unwrap(),
            Default::default(),
        )
        .unwrap();
    let t = Opaque::new(Time(5));
    assert_eq!(t.get().0, 5);
    let ev = Event {
        at: t.clone(),
        later: Some(t),
    };
    let mut vars = HashMap::new();
    vars.insert("e".to_string(), ev);
    let out = p.run(&vars).unwrap();
    assert!(risor::truthy(&out), "one Opaque is one host value");
}

#[derive(Host)]
struct Node {
    value: i64,
    next: Option<Box<Node>>,
}

#[derive(Host)]
struct Ping {
    pong: Vec<Pong>,
}

#[derive(Host)]
struct Pong {
    ping: Option<Ping>,
}

#[test]
fn a_type_that_contains_itself_is_bad_type() {
    assert!(text::<Node>().contains("contains itself"));
    assert!(text::<Ping>().contains("contains itself"));
    let e = Env::builder().var_host::<Node>("n").build().err().unwrap();
    assert_eq!(e.diagnostics[0].code, "bad-type");
    // Presenting a value is no problem.
    let n = Node {
        value: 1,
        next: Some(Box::new(Node {
            value: 2,
            next: None,
        })),
    };
    assert_eq!(n.keys().len(), 2);
}

#[derive(Host)]
struct Pair<'a, T, const N: usize>
where
    T: Clone,
{
    name: &'a str,
    items: [T; N],
}

#[test]
fn generic_structs() {
    assert_eq!(
        text::<Pair<'static, bool, 2>>(),
        "{items: bool[], name: string}"
    );
    let p = Pair {
        name: "x",
        items: [1u8, 2],
    };
    assert!(matches!(p.get("name"), Some(View::Str("x"))));
}

#[derive(Host)]
struct Shared {
    a: Line,
    b: Line,
    c: Vec<Line>,
}

#[test]
fn a_struct_used_twice_is_derived_once() {
    assert_eq!(
        text::<Shared>(),
        "{a: {qty: int, sku: string}, b: {qty: int, sku: string}, c: {qty: int, sku: string}[]}"
    );
}

#[derive(Host)]
struct Combined {
    #[risor(rename = "a\u{2d}b", absent_if_none)]
    x: Option<i32>,
    #[risor(rename = r"raw")]
    y: bool,
}

#[test]
fn attributes_combine_and_literals_unescape() {
    assert_eq!(text::<Combined>(), "{\"a-b\"?: int, raw: bool}");
}

#[test]
fn the_round_trip_of_a_derived_environment() {
    let env = Env::builder()
        .opaque("Time")
        .var_host::<Order>("order")
        .var_host::<Event>("event")
        .build()
        .unwrap();
    let e2 = Env::from_description(&env.description_text()).unwrap();
    assert_eq!(e2.description_text(), env.description_text());
}
