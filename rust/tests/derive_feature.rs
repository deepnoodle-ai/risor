//! With the feature `derive`, `#[derive(risor::Host)]` works with one
//! dependency.
#![cfg(feature = "derive")]

#[derive(risor::Host)]
struct Point {
    x: f64,
    #[risor(absent_if_none)]
    label: Option<String>,
}

#[test]
fn the_derive_through_the_library() {
    assert_eq!(
        risor::describe_host_type::<Point>().unwrap(),
        "{label?: string, x: float}"
    );
    let p = Point {
        x: 1.0,
        label: None,
    };
    assert_eq!(risor::HostMap::keys(&p), ["x"]);
}

#[derive(risor::Host, Debug, PartialEq)]
struct Backend {
    name: String,
    #[risor(rename = "port-number", default)]
    port: u16,
    #[risor(absent_if_none)]
    note: Option<String>,
    #[risor(skip)]
    cache: Vec<u8>,
}

#[derive(risor::Host, Debug, PartialEq)]
struct Server {
    listen: String,
    backends: Vec<Backend>,
    r#type: [u8; 2],
}

/// `f10-config.md` 6.5: the decode the derive implements.
#[test]
fn the_derive_decodes() {
    assert_eq!(
        risor::describe_host_type::<Backend>().unwrap(),
        "{name: string, note?: string, \"port-number\"?: int}"
    );
    let env = risor::Env::default();
    let src = r#"{listen: ":80", type: [1, 2],
        backends: [{name: "a", "port-number": 81}, {name: "b", note: "x"}]}"#;
    let got = risor::config::load_into::<Server>(&env, src, &risor::NoVars, 5).unwrap();
    assert_eq!(
        got.value,
        Server {
            listen: ":80".into(),
            r#type: [1, 2],
            backends: vec![
                Backend {
                    name: "a".into(),
                    port: 81,
                    note: None,
                    cache: vec![],
                },
                Backend {
                    name: "b".into(),
                    port: 0,
                    note: Some("x".into()),
                    cache: vec![],
                },
            ],
        }
    );
    // A key with no field fails the check; a port out of range, the decode.
    let bad = r#"{listen: ":80", type: [1, 2], backends: [{name: "a", "port-number": 70000}]}"#;
    let e = risor::config::load_into::<Server>(&env, bad, &risor::NoVars, 5).unwrap_err();
    assert_eq!(e.step, 5);
    assert_eq!(e.diagnostics[0].path.as_deref().map(|p| p.len()), Some(3));
    let short = r#"{listen: ":80", type: [1], backends: []}"#;
    assert_eq!(
        risor::config::load_into::<Server>(&env, short, &risor::NoVars, 5)
            .unwrap_err()
            .step,
        5
    );
}
