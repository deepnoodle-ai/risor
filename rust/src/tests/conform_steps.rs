//! F4 7.3 and F8 6.4: the conformance function and `conform_all` count
//! steps against a step limit. Host data that holds one list at every level
//! has 2^levels places; each is charged, so the limit ends the walk (#678).
//! A case file holds no shared data, so these are unit tests.

use crate::host::{HostList, View};
use crate::{ConformOptions, Env};
use std::rc::Rc;

/// A host list of two elements: the same inner list twice, or at the
/// bottom the same leaf twice.
enum Shared {
    Int(i64),
    Str(&'static str),
    Twice(Rc<Shared>),
}

impl HostList for Shared {
    fn len(&self) -> usize {
        2
    }
    fn get(&self, index: usize) -> Option<View<'_>> {
        if index >= 2 {
            return None;
        }
        Some(match self {
            Shared::Int(n) => View::Int(*n),
            Shared::Str(s) => View::Str(s),
            Shared::Twice(inner) => View::List(inner.as_ref()),
        })
    }
}

/// `levels` levels of lists; `leaf` is the bottom list.
fn shared(levels: usize, leaf: Shared) -> Shared {
    let mut v = leaf;
    for _ in 1..levels {
        v = Shared::Twice(Rc::new(v));
    }
    v
}

fn nested(n: usize) -> String {
    format!("int{}", "[]".repeat(n))
}

/// The walk over 2^31 - 1 shared places stops at the limit. An unmetered
/// walk would not end, so the test's own timeout guards against a hang; no
/// clock is needed. The limit is low so that it is fast.
#[test]
fn thirty_levels_of_shared_lists_end_at_the_limit() {
    let env = Env::default();
    let t = env.parse_type(&nested(30)).unwrap();
    let limit = ConformOptions {
        steps: Some(10_000),
    };
    let data = shared(30, Shared::Int(1));
    let v = crate::value::from_view(View::List(&data));
    let d = crate::conform_with(&v, &t, &limit).unwrap_err();
    assert_eq!(d.code, "steps-exceeded");
    assert_eq!(d.class, Some("limit"));
    assert!(d.path.is_none() && d.span.is_none());
    assert_eq!(
        d.message,
        "the run needed more than its limit of 10000 steps"
    );
    let all = crate::conform_all_with(&v, &t, 5, &limit);
    assert_eq!(all.len(), 1);
    assert_eq!(all[0].code, "steps-exceeded");
    // A failing leaf fails at the first leaf, long before the limit.
    let bad = shared(30, Shared::Str("x"));
    let bv = crate::value::from_view(View::List(&bad));
    let f = crate::conform_with(&bv, &t, &limit).unwrap_err();
    assert_eq!(f.code, "not-conforming");
    assert_eq!(f.path.as_ref().map(|p| p.len()), Some(30));
    let fs = crate::conform_all_with(&bv, &t, 5, &limit);
    assert_eq!(fs.len(), 5);
    assert!(fs.iter().all(|d| d.code == "not-conforming"));
}

#[test]
fn the_options_give_the_limit() {
    let env = Env::default();
    let t = env.parse_type(&nested(10)).unwrap();
    let data = shared(10, Shared::Int(1));
    let v = crate::value::from_view(View::List(&data));
    // 2^11 - 1 places, each charged 1.
    let at = |n| ConformOptions { steps: Some(n) };
    assert!(crate::conform_with(&v, &t, &at(2047)).is_ok());
    let d = crate::conform_with(&v, &t, &at(2046)).unwrap_err();
    assert_eq!(
        d.message,
        "the run needed more than its limit of 2046 steps"
    );
    // None turns it off.
    let t16 = env.parse_type(&nested(16)).unwrap();
    let d16 = shared(16, Shared::Int(1));
    let v16 = crate::value::from_view(View::List(&d16));
    assert!(crate::conform_with(&v16, &t16, &ConformOptions { steps: None }).is_ok());
    // A limit of 0 lets only any pass.
    let one = crate::Value::Num(1.0);
    assert!(crate::conform_with(&one, &env.parse_type("any").unwrap(), &at(0)).is_ok());
    assert_eq!(
        crate::conform_with(&one, &env.parse_type("int").unwrap(), &at(0))
            .unwrap_err()
            .message,
        "the run needed more than its limit of 0 steps"
    );
}

/// F8 6.4 step 1: when the first walk ends at the depth limit, the list is
/// that diagnostic alone, also after an earlier failure and with no step
/// limit (the decode, R324). Here the rest loop fails at `a`, then goes on
/// into `b`, which nests past 200.
#[test]
fn a_depth_limit_after_an_early_failure_is_the_whole_list() {
    let env = Env::builder()
        .named_type("Rest", "{...Rest | int}")
        .build()
        .unwrap();
    let t = env.parse_type("Rest").unwrap();
    let mut deep = crate::Data::Int(1);
    for _ in 0..201 {
        deep = crate::Data::Map([("b".to_string(), deep)].into_iter().collect());
    }
    let top = crate::Data::Map(
        [
            ("a".to_string(), crate::Data::Str("x".into())),
            ("b".to_string(), deep),
        ]
        .into_iter()
        .collect(),
    );
    let v = crate::Value::from_data(&top);
    let d = crate::conform(&v, &t).unwrap_err();
    assert_eq!(d.code, "depth-exceeded");
    for steps in [Some(crate::conform::FREE_STEPS), None] {
        let all = crate::conform_all_with(&v, &t, 1, &ConformOptions { steps });
        let codes: Vec<_> = all.iter().map(|d| d.code).collect();
        assert_eq!(codes, ["depth-exceeded"], "steps {steps:?}");
    }
}

/// The environment's step limit plays no part (R322).
#[test]
fn the_environment_limit_plays_no_part() {
    let env = Env::builder()
        .limits(crate::Limits {
            steps: Some(10),
            ..crate::Limits::default()
        })
        .build()
        .unwrap();
    let t = env.parse_type("int[]").unwrap();
    let data = crate::Data::List((0..12).map(crate::Data::Int).collect());
    assert!(crate::conform(&crate::Value::from_data(&data), &t).is_ok());
}

/// `conform_all` ends with the limit after the failures it found (F8 6.4
/// step 3).
#[test]
fn conform_all_ends_with_the_limit() {
    let env = Env::default();
    let t = env.parse_type("int[]").unwrap();
    let list = |xs: &[&str]| {
        crate::Data::List(
            xs.iter()
                .map(|x| match x.parse::<f64>() {
                    Ok(n) => crate::Data::Float(n),
                    Err(_) => crate::Data::Str((*x).into()),
                })
                .collect(),
        )
    };
    let at = ConformOptions { steps: Some(3) };
    let d = list(&["1", "a", "b"]);
    let ds = crate::conform_all_with(&crate::Value::from_data(&d), &t, 5, &at);
    let codes: Vec<_> = ds.iter().map(|d| d.code).collect();
    assert_eq!(codes, ["not-conforming", "steps-exceeded"]);
    assert!(ds[1].path.is_none());
    assert_eq!(
        crate::conform_all_with(&crate::Value::from_data(&d), &t, 1, &at).len(),
        1
    );
    let four = list(&["a", "b", "c", "d"]);
    let two = crate::conform_all_with(
        &crate::Value::from_data(&four),
        &t,
        2,
        &ConformOptions { steps: Some(2) },
    );
    let codes: Vec<_> = two.iter().map(|d| d.code).collect();
    assert_eq!(codes, ["not-conforming", "steps-exceeded"]);
}
