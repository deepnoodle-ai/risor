//! The host API, and what no data case can raise: the deadline, cancellation, a host function that crashes, opaque
//! values in every place `03-host.md` 3.4.6 names, host numbers that cannot
//! enter (`03-host.md` 3.4.4), the depth test on empty containers
//! (`02-evaluation.md` 8.4), and the stack the deepest programs need.

use risor::{
    compile_unchecked as compile, parse, Data, Env, Limits, NoVars, OpaqueRef, Program, RunOptions,
    Value,
};
use std::collections::BTreeMap;
use std::sync::atomic::AtomicBool;
use std::sync::Arc;
use std::time::Duration;

fn program(env: &Env, src: &str) -> Program {
    compile(env, parse(src).unwrap()).unwrap_or_else(|e| panic!("{src}: {e}"))
}

fn vars(pairs: &[(&str, Data)]) -> BTreeMap<String, Data> {
    pairs
        .iter()
        .map(|(k, v)| (k.to_string(), v.clone()))
        .collect()
}

/// The code of a failed run, or the value's debug text.
fn outcome(p: &Program, v: &BTreeMap<String, Data>) -> Result<String, String> {
    let out = p.run(v);
    match out.result {
        Ok(v) => Ok(format!("{v:?}")),
        Err(f) => Err(f.code.to_string()),
    }
}

fn numbers(n: usize) -> Data {
    Data::List((0..n).map(|i| Data::Float(i as f64)).collect())
}

#[test]
fn deadline_exceeded() {
    let env = Env::builder()
        .vars("{xs: float[]}")
        .limits(Limits {
            deadline: Some(Duration::ZERO),
            ..Limits::default()
        })
        .build()
        .unwrap();
    let p = program(&env, "xs.map(x => x + 1)");
    let v = vars(&[("xs", numbers(5000))]);
    assert_eq!(outcome(&p, &v), Err("deadline-exceeded".into()));
    // `catch` does not catch it.
    let p = program(&env, "try { xs.map(x => x + 1) } catch { 0 }");
    assert_eq!(outcome(&p, &v), Err("deadline-exceeded".into()));
    // A short run finishes before the first check.
    let p = program(&env, "1 + 1");
    assert_eq!(outcome(&p, &v), Ok("2".into()));
}

#[test]
fn cancelled() {
    let env = Env::builder().vars("{xs: float[]}").build().unwrap();
    let p = program(&env, "try { xs.map(x => x * 2) } catch { 0 }");
    let v = vars(&[("xs", numbers(5000))]);
    let flag = Arc::new(AtomicBool::new(true));
    let out = p.run_with(&v, RunOptions { cancel: Some(flag) });
    assert_eq!(out.result.err().map(|f| f.code), Some("cancelled"));
}

#[test]
fn function_panic_is_not_caught() {
    let env = Env::builder()
        .function("boom", "() => int", |_, _| panic!("a host bug"))
        .function("err", "() => int", |_, _| {
            Err(risor::HostError("no".into()))
        })
        .build()
        .unwrap();
    let p = program(&env, "try { boom() } catch { 0 }");
    let out = p.run(&NoVars);
    let f = out.result.err().unwrap();
    assert_eq!(f.code, "function-panic");
    assert_eq!(f.message(), "host function boom crashed");
    assert_eq!(f.class(), Some("host"));
    let p = program(&env, "try { err() } catch (e) { e.message }");
    assert_eq!(outcome(&p, &BTreeMap::new()), Ok("\"no\"".into()));
}

#[test]
fn a_host_function_charges_through_its_handle() {
    let env = Env::builder()
        .function("h", "(x: float) => float", |call, args| {
            call.charge(3)
                .map_err(|_| risor::HostError("stop".into()))?;
            call.allocate(2)
                .map_err(|_| risor::HostError("stop".into()))?;
            match args[0] {
                Value::Num(n) => Ok(Data::Float(n * 2.0)),
                _ => Ok(Data::Null),
            }
        })
        .build()
        .unwrap();
    // The call 1, h's 3, `+` 1; units 2.
    let p = program(&env, "h(1) + 1");
    let out = p.run(&NoVars);
    assert_eq!(format!("{:?}", out.result.unwrap()), "3");
    assert_eq!((out.usage.steps, out.usage.allocated), (5, 2));
    let env = Env::builder()
        .function("h", "() => int", |call, _| {
            let _ = call.charge(10);
            Ok(Data::Float(1.0))
        })
        .limits(Limits {
            steps: Some(5),
            ..Limits::default()
        })
        .build()
        .unwrap();
    let p = program(&env, "try { h() } catch { 0 }");
    assert_eq!(outcome(&p, &BTreeMap::new()), Err("steps-exceeded".into()));
}

#[test]
fn a_large_host_charge_does_not_wrap() {
    let env = Env::builder()
        .function("alloc", "() => int", |call, _| {
            assert!(call.allocate(1).is_ok());
            assert!(call.allocate(u64::MAX).is_err());
            Ok(Data::Float(0.0))
        })
        .function("charge", "() => int", |call, _| {
            assert!(call.charge(1).is_ok());
            assert!(call.charge(u64::MAX).is_err());
            Ok(Data::Float(0.0))
        })
        .build()
        .unwrap();
    let p = program(&env, "alloc()");
    let out = p.run(&NoVars);
    assert_eq!(
        out.result.err().map(|f| f.code),
        Some("allocation-exceeded")
    );
    assert_eq!(out.usage.allocated, 1);
    let p = program(&env, "charge()");
    let out = p.run(&NoVars);
    assert_eq!(out.result.err().map(|f| f.code), Some("steps-exceeded"));
    assert_eq!(out.usage.steps, 2);
    // With no limit, the count stops at u64::MAX.
    let env = Env::builder()
        .function("big", "() => int", |call, _| {
            for _ in 0..3 {
                assert!(call.charge(u64::MAX).is_ok());
                assert!(call.allocate(u64::MAX).is_ok());
            }
            Ok(Data::Float(0.0))
        })
        .limits(Limits {
            steps: None,
            allocation: None,
            ..Limits::default()
        })
        .build()
        .unwrap();
    let p = program(&env, "big()");
    let out = p.run(&NoVars);
    assert!(out.result.is_ok());
    assert_eq!((out.usage.steps, out.usage.allocated), (u64::MAX, u64::MAX));
}

#[test]
fn a_limit_diagnostic_has_a_span() {
    // `03-host.md` 3.7.1: present, and its value not specified.
    let env = Env::builder()
        .function("alloc", "() => int", |call, _| {
            let _ = call.allocate(100);
            Ok(Data::Float(0.0))
        })
        .limits(Limits {
            steps: Some(5),
            allocation: Some(3),
            ..Limits::default()
        })
        .build()
        .unwrap();
    for src in [
        "range(100).map(x => x * 2).sum()",
        "[1, 2, 3, 4]",
        "alloc()",
    ] {
        let p = program(&env, src);
        let f = p.run(&NoVars).result.err().unwrap();
        let d = p.diagnostic(&f);
        assert_eq!(d.class, Some("limit"), "{src}");
        assert!(d.span.is_some(), "{src}");
    }
}

fn opaque_env() -> Env {
    Env::builder()
        .vars("{t: Token, u: Token}")
        .opaque("Token")
        .function("same", "(a: Token, b: Token) => bool", |_, args| {
            Ok(Data::Bool(matches!((&args[0], &args[1]),
                (Value::Opaque(a), Value::Opaque(b)) if a.same(b))))
        })
        .function("make", "() => Token", |_, _| {
            Ok(Data::Opaque(OpaqueRef::new("Token", 7u32)))
        })
        .method_impl(
            "Token.id",
            "() => int",
            Some(Arc::new(|_, args| match &args[0] {
                Value::Opaque(o) => Ok(Data::Float(
                    *o.value.downcast_ref::<u32>().unwrap_or(&0) as f64
                )),
                _ => Ok(Data::Null),
            })),
        )
        .build()
        .unwrap()
}

#[test]
fn opaque_values() {
    let env = opaque_env();
    let t = OpaqueRef::new("Token", 1u32);
    let u = OpaqueRef::new("Token", 2u32);
    let v = vars(&[("t", Data::Opaque(t.clone())), ("u", Data::Opaque(u))]);
    let ok = |src: &str, want: &str| {
        let p = program(&env, src);
        assert_eq!(outcome(&p, &v), Ok(want.to_string()), "{src}");
    };
    let err = |src: &str, want: &str| {
        let p = program(&env, src);
        assert_eq!(outcome(&p, &v), Err(want.to_string()), "{src}");
    };
    // A condition: always truthy.
    ok("t ? 1 : 2", "1");
    ok("!t", "false");
    ok("if (t) { 1 } else { 2 }", "1");
    ok("t && 5", "5");
    ok("(t || 5) == t", "true");
    // Bound, put in a list and a map, compared with ==.
    ok("{ const a = t; a == t }", "true");
    ok("[t, u][0] == t", "true");
    ok("{k: t}.k == t", "true");
    ok("t == u", "false");
    ok("t != u", "true");
    ok("t in [u, t]", "true");
    ok("type(t)", "\"opaque\"");
    ok("bool(t)", "true");
    // Passed to a host function, returned by one, used through its methods.
    ok("same(t, t)", "true");
    ok("same(t, u)", "false");
    ok("make().id()", "7");
    ok("t.id()", "1");
    ok("t?.id()", "1");
    // Nothing else.
    err("t.x", "wrong-kind");
    err("t[0]", "wrong-kind");
    err("t < u", "wrong-kind");
    err("t + 1", "wrong-kind");
    err("`${t}`", "wrong-kind");
    err("`${[1, t]}`", "wrong-kind");
    err("string(t)", "wrong-kind");
    err("t.nope()", "wrong-kind");
    err("t.id(1)", "wrong-kind");
    err("t.id(x => x)", "wrong-kind");
    err("t.map(x => x)", "wrong-kind");
    err("{...t}", "wrong-kind");
    err("[...t]", "wrong-kind");
    err("{ const [a] = t; a }", "wrong-kind");
    err("\"a\" in t", "wrong-kind");
}

#[test]
fn host_numbers_that_cannot_enter() {
    let env = Env::builder()
        .vars("{n: int, xs: int[], m: map<int>, f: float}")
        .function("big", "() => int", |_, _| Ok(Data::Int(1 << 60)))
        .function("bigs", "() => int[]", |_, _| {
            Ok(Data::List(vec![Data::Int(1), Data::Int(1 << 60)]))
        })
        .build()
        .unwrap();
    let big = Data::Int(9007199254740992);
    let m: BTreeMap<String, Data> = [("a".to_string(), big.clone())].into();
    let v = vars(&[
        ("n", big.clone()),
        ("xs", Data::List(vec![Data::Int(1), big.clone()])),
        ("m", Data::Map(m)),
        ("f", Data::Float(1e20)),
    ]);
    let run = |src: &str| {
        let p = program(&env, src);
        let out = p.run(&v);
        match out.result {
            Ok(v) => Ok(format!("{v:?}")),
            Err(f) => {
                if f.code == "number-range" {
                    assert_eq!(f.class(), Some("environment"), "{src}");
                    assert_eq!(f.message(), "a number from the host is out of range");
                }
                Err(f.code)
            }
        }
    };
    assert_eq!(run("n"), Err("number-range"));
    assert_eq!(
        run("try { n } catch (e) { e.code }"),
        Ok("\"number-range\"".into())
    );
    assert_eq!(run("xs[0]"), Ok("1".into()));
    assert_eq!(run("xs[1]"), Err("number-range"));
    assert_eq!(run("m.a"), Err("number-range"));
    // Measuring or copying a list reads no element (`02-evaluation.md` 8.5).
    assert_eq!(run("len(xs)"), Ok("2".into()));
    assert_eq!(run("len([...xs])"), Ok("2".into()));
    // Comparing and rendering read it.
    assert_eq!(run("xs == [1, 2]"), Err("number-range"));
    assert_eq!(run("`${xs}`"), Err("number-range"));
    assert_eq!(run("xs.map(x => 0)"), Err("number-range"));
    assert_eq!(run("{ const [a, b] = xs; a }"), Err("number-range"));
    // Handing the result over is not a read (`02-evaluation.md` 8.4).
    assert!(run("xs").is_ok());
    // A host float enters whenever it is finite.
    assert_eq!(run("f"), Ok("100000000000000000000".into()));
    assert_eq!(run("big()"), Err("number-range"));
    assert_eq!(run("bigs()[0]"), Ok("1".into()));
    assert_eq!(run("bigs()[1]"), Err("number-range"));
}

#[test]
fn empty_containers_are_entered_at_the_depth_limit() {
    let env = Env::builder()
        .limits(Limits {
            depth: 1,
            ..Limits::default()
        })
        .build()
        .unwrap();
    let none = BTreeMap::new();
    let run = |src: &str| outcome(&program(&env, src), &none);
    // Depth 1: one level may be entered, the second may not.
    assert_eq!(run("[] == []"), Ok("true".into()));
    assert_eq!(run("{} == {}"), Ok("true".into()));
    assert_eq!(run("[[]] == [[]]"), Err("depth-exceeded".into()));
    assert_eq!(run("[{}] == [{}]"), Err("depth-exceeded".into()));
    assert_eq!(run("`${[]}`"), Ok("\"[]\"".into()));
    assert_eq!(run("`${[[]]}`"), Err("depth-exceeded".into()));
    assert_eq!(run("`${{a: {}}}`"), Err("depth-exceeded".into()));
    // Lists of different lengths are not entered.
    assert_eq!(run("[[]] == [[], []]"), Ok("false".into()));
    // An empty pair costs nothing.
    let p = program(&env, "[] == []");
    let out = p.run(&none);
    assert_eq!(out.usage.steps, 0);
}

fn nested(depth: usize) -> Data {
    let mut d = Data::List(vec![]);
    for _ in 1..depth {
        d = Data::List(vec![d]);
    }
    d
}

/// The deepest programs the limits allow: a walk at depth
/// 1,000, calls in progress, and the nesting limit, run on a thread with the
/// stack `DESIGN.md` states for this build.
#[test]
fn the_deepest_programs_run_within_the_stated_stack() {
    // Measured: 2 MiB optimized and 4 MiB unoptimized at depth 1,000.
    let stack = if cfg!(debug_assertions) { 6 } else { 3 } << 20;
    std::thread::Builder::new()
        .stack_size(stack)
        .spawn(|| {
            let env = Env::builder()
                .vars("{v: any, w: any}")
                .limits(Limits {
                    depth: 1000,
                    ..Limits::default()
                })
                .build()
                .unwrap();
            let v = vars(&[("v", nested(1000)), ("w", nested(1000))]);
            assert_eq!(outcome(&program(&env, "v == w"), &v), Ok("true".into()));
            assert!(outcome(&program(&env, "len(string(v))"), &v).is_ok());
            let v2 = vars(&[("v", nested(1001)), ("w", nested(1001))]);
            assert_eq!(
                outcome(&program(&env, "v == w"), &v2),
                Err("depth-exceeded".into())
            );
            // 1,000 calls in progress, each through a lambda of its own, at the
            // nesting limit of the source inside the last one.
            let mut src = String::from("const f0 = x => x;\n");
            for i in 1..1000 {
                src.push_str(&format!("const f{i} = x => f{}(x);\n", i - 1));
            }
            let deep = format!("{}1{}", "(".repeat(99), ")".repeat(99));
            src.push_str(&format!("f999({deep})"));
            assert_eq!(outcome(&program(&env, &src), &v), Ok("1".into()));
            let mut src = format!("const g0 = x => {}x{};\n", "[".repeat(98), "]".repeat(98));
            for i in 1..1000 {
                src.push_str(&format!("const g{i} = x => g{}(x);\n", i - 1));
            }
            src.push_str("const r = g999(1); len(r)");
            assert_eq!(outcome(&program(&env, &src), &v), Ok("1".into()));
        })
        .unwrap()
        .join()
        .unwrap();
}

/// What opens no level costs no stack at run time either (13.2 rule 6).
#[test]
fn flat_programs_run() {
    let env = Env::default();
    let none = BTreeMap::new();
    let n = 100_000;
    let run = |src: String| outcome(&program(&env, &src), &none);
    assert_eq!(run(vec!["1"; n].join(" + ")), Ok(n.to_string()));
    let mut t = String::new();
    for _ in 0..n {
        t.push_str("false ? 1 : ");
    }
    t.push('2');
    assert_eq!(run(t), Ok("2".into()));
    let mut t = String::from("if (false) { 0 }");
    for _ in 0..n {
        t.push_str(" else if (false) { 1 }");
    }
    t.push_str(" else { 2 }");
    assert_eq!(run(t), Ok("2".into()));
    let mut t = String::from("{ ");
    for i in 0..n {
        t.push_str(&format!("const a{i} = {i}; "));
    }
    t.push_str("a99999 }");
    assert_eq!(run(t), Ok("99999".into()));
    assert_eq!(run(format!("\"a\"{}", "[0]".repeat(n))), Ok("\"a\"".into()));
    assert_eq!(
        run(format!("[{}].map(x => x)[0]", vec!["1"; n].join(","))),
        Ok("1".into())
    );
    assert_eq!(
        run(format!("[1]{}", ".map(x => x)".repeat(n))),
        Ok("[1]".into())
    );
}

/// A depth limit outside 1 to 1,000 is `bad-limit` (`03-host.md` 3.3.4).
#[test]
fn a_depth_limit_out_of_range_is_bad_limit() {
    for depth in [0, 1001] {
        let e = Env::builder()
            .limits(Limits {
                depth,
                ..Limits::default()
            })
            .build()
            .err()
            .expect("the environment is refused");
        assert_eq!(e.first().code, "bad-limit");
    }
    for depth in [1, 1000] {
        assert!(Env::builder()
            .limits(Limits {
                depth,
                ..Limits::default()
            })
            .build()
            .is_ok());
    }
}

/// An environment and a compiled program are shared between threads
/// (`03-host.md` 3.2.2).
#[test]
fn programs_and_environments_are_send_and_sync() {
    fn check<T: Send + Sync>() {}
    check::<Env>();
    check::<Program>();
}

fn deep(n: usize) -> Data {
    let mut d = Data::List(vec![Data::Float(1.0)]);
    for _ in 1..n {
        d = Data::List(vec![d]);
    }
    d
}

/// Handing the result over is not a walk, so host data nested
/// past the depth limit is the value, and the binding's own conversion of
/// it to JSON (the outcome command's) neither fails nor overflows.
#[test]
fn a_deep_result_is_handed_over() {
    let env = Env::builder().vars("{a: any}").build().unwrap();
    let p = program(&env, "a");
    let v = vars(&[("a", deep(1000))]);
    let out = p.run(&v);
    assert!(out.result.is_ok());
    assert_eq!(out.usage.steps, 0);
    // The outcome command's path, on the same data.
    let doc = risor::json::parse(
        r#"[{"name": "deep", "env": "{a: any}", "source": "a", "vars": {"a": 0}}]"#,
    )
    .unwrap();
    let mut cases = risor::testing::load_doc_lax(&doc, std::path::Path::new(".")).unwrap();
    let mut j = String::from("1");
    for _ in 0..1000 {
        j = format!("[{j}]");
    }
    cases[0].vars = Some(vec![("a".into(), risor::json::parse(&j).unwrap())]);
    let line = risor::testing::outcome_line(&cases[0]);
    assert!(
        line.contains("\"phase\":null"),
        "{}",
        &line[..80.min(line.len())]
    );
    assert!(line.contains(&"[".repeat(1000)));
}

/// `03-host.md` 3.8: no message a run builds leaves a template's hole unfilled.
/// `messages::build` asserts it in a test build; these are the limit
/// messages, which carry the limit.
#[test]
fn limit_messages_carry_their_limit() {
    let none = BTreeMap::new();
    for (limits, src, want) in [
        (
            Limits {
                steps: Some(2),
                ..Limits::default()
            },
            "1 + 1 + 1 + 1",
            "the run needed more than its limit of 2 steps",
        ),
        (
            Limits {
                allocation: Some(2),
                ..Limits::default()
            },
            "[1, 2, 3]",
            "the run needed more than its limit of 2 allocation units",
        ),
        (
            Limits {
                depth: 1,
                ..Limits::default()
            },
            "`${[[1]]}`",
            "calls and nested values went deeper than the depth limit of 1",
        ),
    ] {
        let env = Env::builder().limits(limits).build().unwrap();
        let p = program(&env, src);
        let f = p.run(&none).result.err().unwrap();
        assert_eq!(f.message(), want);
        assert!(!f.message().contains('{'));
    }
}

/// `03-host.md` 3.2.1: compile checks by default; compiling without a check is an
/// explicit option, and gives the run's classes of the column "not checked".
#[test]
fn compile_checks_by_default() {
    let env = Env::builder()
        .vars("{x: any, n: int | null}")
        .build()
        .unwrap();
    let e = risor::compile(&env, parse("n + 1").unwrap()).err().unwrap();
    assert_eq!(e.first().code, "maybe-null");
    assert!(risor::compile_unchecked(&env, parse("n + 1").unwrap()).is_ok());
    // The same failure has the checked class at a `dynamic` site, and the
    // unchecked class without a check.
    let v = vars(&[("x", Data::Float(1.0)), ("n", Data::Null)]);
    let p = risor::compile(&env, parse("x.a").unwrap()).unwrap();
    let f = p.run(&v).result.err().unwrap();
    assert_eq!(p.class_of(&f), Some("dynamic"));
    let p = risor::compile(&env, parse("match x { 2 => 1 }").unwrap()).err();
    assert!(p.is_some(), "an `any` subject needs `_`");
    let p = risor::compile_unchecked(&env, parse("match x { 2 => 1 }").unwrap()).unwrap();
    let f = p.run(&v).result.err().unwrap();
    assert_eq!(p.class_of(&f), Some("dynamic"));
    let r = risor::check(&env, &parse("x").unwrap(), &risor::Expected::Nothing);
    assert_eq!(risor::check::ty::print(r.ty.as_ref().unwrap()), "any");
}
