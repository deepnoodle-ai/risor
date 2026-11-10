//! With the feature `derive`, `#[derive(risor::Host)]` works with one
//! dependency (ruling BB26).
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
