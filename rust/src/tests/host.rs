//! The host API, and what no data case can raise: the deadline, cancellation, a host function that crashes, opaque
//! values in every place `03-host.md` 3.4.6 names, host numbers that cannot
//! enter (`03-host.md` 3.4.4), the depth test on empty containers
//! (`02-evaluation.md` 8.4), and the stack the deepest programs need.

use crate::compile::compile_unchecked as compile;
use crate::{parse, Data, Env, Limits, NoVars, OpaqueRef, Program, RunOptions, Value};
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
    let out = p.outcome(v);
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
        .var_record("{xs: float[]}")
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
    let env = Env::builder().var_record("{xs: float[]}").build().unwrap();
    let p = program(&env, "try { xs.map(x => x * 2) } catch { 0 }");
    let v = vars(&[("xs", numbers(5000))]);
    let flag = Arc::new(AtomicBool::new(true));
    let out = p.outcome_with(&v, RunOptions { cancel: Some(flag) });
    assert_eq!(out.result.err().map(|f| f.code), Some("cancelled"));
}

#[test]
fn function_panic_is_not_caught() {
    let env = Env::builder()
        .function("boom", "() => int", |_, _| panic!("a host bug"))
        .function("err", "() => int", |_, _| {
            Err(crate::HostError::Text("no".into()))
        })
        .build()
        .unwrap();
    let p = program(&env, "try { boom() } catch { 0 }");
    let out = p.outcome(&NoVars);
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
                .map_err(|_| crate::HostError::Text("stop".into()))?;
            call.allocate(2)
                .map_err(|_| crate::HostError::Text("stop".into()))?;
            match args[0] {
                Value::Num(n) => Ok(Data::Float(n * 2.0)),
                _ => Ok(Data::Null),
            }
        })
        .build()
        .unwrap();
    // The call 1, h's 3, `+` 1; units 2.
    let p = program(&env, "h(1) + 1");
    let out = p.outcome(&NoVars);
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
    let out = p.outcome(&NoVars);
    assert_eq!(
        out.result.err().map(|f| f.code),
        Some("allocation-exceeded")
    );
    assert_eq!(out.usage.allocated, 1);
    let p = program(&env, "charge()");
    let out = p.outcome(&NoVars);
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
    let out = p.outcome(&NoVars);
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
        let f = p.outcome(&NoVars).result.err().unwrap();
        let d = p.diagnostic(&f);
        assert_eq!(d.class, Some("limit"), "{src}");
        assert!(d.span.is_some(), "{src}");
    }
}

fn opaque_env() -> Env {
    Env::builder()
        .var_record("{t: Token, u: Token}")
        .opaque("Token")
        .function("same", "(a: Token, b: Token) => bool", |_, args| {
            Ok(Data::Bool(matches!((&args[0], &args[1]),
                (Value::Opaque(a), Value::Opaque(b)) if a.same(b))))
        })
        .function("make", "() => Token", |_, _| {
            Ok(Data::Opaque(OpaqueRef::new("Token", 7u32)))
        })
        .method_decl(
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
        .var_record("{n: int, xs: int[], m: map<int>, f: float}")
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
        let out = p.outcome(&v);
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

/// `03-host.md` 3.4.6 item 6 and 3.5.2 item 5: the host owns an opaque
/// value's state, a method may change it, and no call is cached.
#[test]
fn an_opaque_value_state_belongs_to_the_host() {
    use std::sync::Mutex;
    let env = Env::builder()
        .var_record("{w: Label}")
        .opaque("Label")
        .method("Label", "text", "() => string", |_, args| match &args[0] {
            Value::Opaque(o) => {
                let m = o.value.downcast_ref::<Mutex<String>>().unwrap();
                Ok(Data::Str(m.lock().unwrap().clone()))
            }
            _ => Ok(Data::Null),
        })
        .method(
            "Label",
            "setText",
            "(s: string) => null",
            |_, args| match (&args[0], &args[1]) {
                (Value::Opaque(o), Value::Str(s)) => {
                    let m = o.value.downcast_ref::<Mutex<String>>().unwrap();
                    *m.lock().unwrap() = s.as_str().to_string();
                    Ok(Data::Null)
                }
                _ => Ok(Data::Null),
            },
        )
        .build()
        .unwrap();
    let w = OpaqueRef::new("Label", Mutex::new("a".to_string()));
    let v = vars(&[("w", Data::Opaque(w.clone()))]);
    let p = program(
        &env,
        r#"{ const a = w.text(); const _ = w.setText("x"); [a, w.text()] }"#,
    );
    assert_eq!(outcome(&p, &v), Ok(r#"["a", "x"]"#.into()));
    // The host keeps the value across runs and sees the change.
    let m = w.value.downcast_ref::<Mutex<String>>().unwrap();
    assert_eq!(*m.lock().unwrap(), "x");
    let p = program(&env, "w.text()");
    assert_eq!(outcome(&p, &v), Ok("\"x\"".into()));
}

/// `03-host.md` 3.4.4: NaN and the infinities never enter, as a variable,
/// inside a list or a map, or as a host function's result.
#[test]
fn non_finite_host_numbers_do_not_enter() {
    for x in [f64::NAN, f64::INFINITY, f64::NEG_INFINITY] {
        let env = Env::builder()
            .var_record("{n: float, xs: float[], m: map<float>}")
            .function("bad", "() => float", move |_, _| Ok(Data::Float(x)))
            .function("bads", "() => float[]", move |_, _| {
                Ok(Data::List(vec![Data::Float(1.0), Data::Float(x)]))
            })
            .function("badm", "() => map<float>", move |_, _| {
                Ok(Data::Map([("a".to_string(), Data::Float(x))].into()))
            })
            .build()
            .unwrap();
        let m: BTreeMap<String, Data> = [("a".to_string(), Data::Float(x))].into();
        let v = vars(&[
            ("n", Data::Float(x)),
            ("xs", Data::List(vec![Data::Float(1.0), Data::Float(x)])),
            ("m", Data::Map(m)),
        ]);
        for src in [
            "n",
            "n + 0",
            "xs[1]",
            "m.a",
            "m[\"a\"]",
            "bad()",
            "bads()[1]",
            "badm().a",
        ] {
            let p = program(&env, src);
            let f = p.outcome(&v).result.err();
            let f = f.unwrap_or_else(|| panic!("{x} {src}: no failure"));
            assert_eq!(f.code, "number-range", "{x} {src}");
            assert_eq!(f.class(), Some("environment"), "{x} {src}");
        }
        // The finite neighbours enter.
        let p = program(&env, "xs[0] + bads()[0]");
        assert_eq!(format!("{:?}", p.outcome(&v).result.unwrap()), "2");
    }
}

/// `03-host.md` 3.5.3 items 5 to 7: a host function that fails with a
/// failure of this binding, such as that of a nested run.
#[test]
fn a_nested_failure_is_a_function_error_with_its_cause() {
    fn nested(
        src: &'static str,
    ) -> impl Fn(&mut crate::Call<'_>, &[Value<'_>]) -> Result<Data, crate::HostError> {
        move |_, _| {
            let env = Env::builder().build()?;
            let p = env.compile(&parse(src)?, crate::CompileOptions::default())?;
            let v = p.run(&NoVars)?;
            Ok(if crate::truthy(&v) {
                Data::Bool(true)
            } else {
                Data::Null
            })
        }
    }
    let env = Env::builder()
        .function(
            "city",
            "() => bool !",
            nested(r#"fail("unknown city", {n: 1})"#),
        )
        .function("syntax", "() => bool !", nested("1 +"))
        .build()
        .unwrap();
    let p = program(&env, "city()");
    let e = p.run(&NoVars).unwrap_err();
    let d = e.first();
    assert_eq!(d.code, "function-error");
    assert_eq!(d.class, Some("host"));
    // The message is the inner diagnostic's, whole; the data stays null.
    assert_eq!(d.message, "unknown city");
    assert!(d.data.is_none());
    let cause = d.cause.as_deref().expect("a cause");
    assert_eq!(
        (cause.code, cause.message.as_str()),
        ("failed", "unknown city")
    );
    assert!(cause.data.is_some());
    // `source` gives the same diagnostic.
    let src = std::error::Error::source(&e).expect("a source");
    let src = src
        .downcast_ref::<crate::Diagnostic>()
        .expect("a Diagnostic");
    assert_eq!(src.code, "failed");
    // `catch` catches it, by its own code.
    let p = program(
        &env,
        "try { city() } catch (e) { `${e.code}: ${e.message}` }",
    );
    assert_eq!(
        format!("{:?}", p.run(&NoVars).unwrap()),
        "\"function-error: unknown city\""
    );
    // A failure of another stage gives its message the same way.
    let p = program(&env, "syntax()");
    let e = p.run(&NoVars).unwrap_err();
    let cause = e.first().cause.as_deref().expect("a cause");
    assert_eq!(cause.phase, crate::Phase::Parse);
    assert_eq!(e.first().message, cause.message);
    // A rendering writes neither the cause nor its code (`f08-errors.md` 6).
    let p = program(&env, "city()");
    let e = p.run(&NoVars).unwrap_err();
    for r in [
        crate::render_line(&e.diagnostics),
        crate::render_json(&e.diagnostics),
        crate::render_detail(&e.diagnostics, Some("city()")),
    ] {
        assert!(r.contains("function-error"), "{r}");
        assert!(r.contains("unknown city"), "{r}");
        assert!(!r.contains("failed") && !r.contains("cause"), "{r}");
    }
    // A plain error has no cause.
    let env = Env::builder()
        .function("err", "() => int !", |_, _| Err("no".into()))
        .build()
        .unwrap();
    let e = program(&env, "err()").run(&NoVars).unwrap_err();
    assert_eq!(e.first().message, "no");
    assert!(e.first().cause.is_none());
}

/// 3.5.3 item 7: a chain of nested runs gives the innermost message.
#[test]
fn a_chain_of_nested_runs_gives_the_innermost_message() {
    let env = Env::builder()
        .function("outer", "() => int !", |_, _| {
            let inner = Env::builder()
                .function("deep", "() => int !", |_, _| {
                    let env = Env::builder().build()?;
                    let p = env.compile(&parse(r#"fail("at the bottom")"#)?, Default::default())?;
                    p.run(&NoVars)?;
                    Ok(Data::Int(0))
                })
                .build()?;
            let p = inner.compile(&parse("deep()")?, Default::default())?;
            p.run(&NoVars)?;
            Ok(Data::Int(0))
        })
        .build()
        .unwrap();
    let e = program(&env, "outer()").run(&NoVars).unwrap_err();
    let d = e.first();
    assert_eq!(
        (d.code, d.message.as_str()),
        ("function-error", "at the bottom")
    );
    let cause = d.cause.as_deref().expect("a cause");
    assert_eq!(cause.code, "function-error");
    let root = cause.cause.as_deref().expect("a second cause");
    assert_eq!(
        (root.code, root.message.as_str()),
        ("failed", "at the bottom")
    );
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
    let out = p.outcome(&none);
    assert_eq!(out.usage.steps, 0);
}

fn nested(depth: usize) -> Data {
    let mut d = Data::List(vec![]);
    for _ in 1..depth {
        d = Data::List(vec![d]);
    }
    d
}

/// The stack `DESIGN.md` (Stack) states that a run needs at the depth limit
/// `depth`, in this build.
pub(super) fn stated_stack(depth: u32) -> usize {
    let mib = match (cfg!(debug_assertions), depth) {
        (false, 200) => 1,
        (false, 400) | (true, 200) => 2,
        (true, 400) => 4,
        _ => panic!("no stated stack at depth {depth}"),
    };
    mib << 20
}

/// `e` inside 98 levels of `{a: ...}.a`, inside a lambda: the source nesting
/// limit, of the kind that costs the most stack.
fn in_maps(e: &str) -> String {
    let mut s = e.to_string();
    for _ in 0..98 {
        s = format!("{{a: {s}}}.a");
    }
    s
}

/// A chain of `n` lambdas: `f0`, then each `link` with `{P}` the one before;
/// then `top` with `{N}` the last.
fn chain(f0: &str, link: &str, top: &str, n: usize) -> String {
    let mut src = format!("const f0 = {f0};\n");
    for i in 1..n {
        let l = link.replace("{P}", &format!("f{}", i - 1));
        src.push_str(&format!("const f{i} = {l};\n"));
    }
    src.push_str(&top.replace("{N}", &format!("f{}", n - 1)));
    src
}

/// The deepest programs the limits allow, at the depth limits 200 and 400:
/// a walk, calls by name, calls of lambdas by library entries with the
/// source nesting limit inside the last call, and the library entry `walk`
/// (F17 5.3). Each runs on a thread with two
/// thirds of the stack `DESIGN.md` states for this build, so the stated
/// figure has a margin of 1.5 (`03-host.md` 3.9.1).
#[test]
fn the_deepest_programs_run_within_the_stated_stack() {
    for depth in [200, 400] {
        std::thread::Builder::new()
            .name(format!("depth {depth}"))
            .stack_size(stated_stack(depth) / 3 * 2)
            .spawn(move || deepest_programs(depth))
            .unwrap()
            .join()
            .unwrap();
    }
}

fn deepest_programs(depth: u32) {
    let d = depth as usize;
    let env = Env::builder()
        .var_record("{v: any, w: any}")
        .limits(Limits {
            depth,
            ..Limits::default()
        })
        .build()
        .unwrap();
    let v = vars(&[("v", nested(d)), ("w", nested(d))]);
    let run = |src: &str| outcome(&program(&env, src), &v);
    // A walk to the limit, and one level past it.
    assert_eq!(run("v == w"), Ok("true".into()));
    assert!(run("len(string(v))").is_ok());
    let v2 = vars(&[("v", nested(d + 1)), ("w", nested(d + 1))]);
    assert_eq!(
        outcome(&program(&env, "v == w"), &v2),
        Err("depth-exceeded".into())
    );
    // Calls by name to the limit, each through a lambda of its own: with the
    // nesting limit in the argument of the first, and inside the last.
    let parens = format!("{}1{}", "(".repeat(99), ")".repeat(99));
    let src = chain("x => x", "x => {P}(x)", &format!("{{N}}({parens})"), d);
    assert_eq!(run(&src), Ok("1".into()));
    let brackets = format!("x => {}x{}", "[".repeat(98), "]".repeat(98));
    let src = chain(&brackets, "x => {P}(x)", "const r = {N}(1); len(r)", d);
    assert_eq!(run(&src), Ok("1".into()));
    // Calls of lambdas by library entries: a lambda by name, an inline
    // lambda, and lambdas with patterns, which bind their parameters by the
    // general path. Each link adds 1, the inline lambda's 2, so the chain
    // reaches the limit. One more link passes it: the failure unwinds from
    // the deepest call, with a trace entry for each call in progress and for
    // the call that failed its test (`f08-errors.md` 4.1).
    let cases = [
        (
            "x => [x].map({P})[0]",
            format!("x => {}", in_maps("x")),
            "{N}(1)",
            1,
            "1",
        ),
        (
            "x => [x].map(v => {P}(v))[0]",
            format!("x => {}", in_maps("x")),
            "[1].map(v => {N}(v))[0]",
            2,
            "1",
        ),
        (
            "x => [x].flatMap({P})",
            format!("x => [{}]", in_maps("x")),
            "{N}(1)[0]",
            1,
            "1",
        ),
        (
            "({v}) => [{v}].filter({P})[0].v",
            format!("({{v}}) => {}", in_maps("v")),
            "{N}({v: 1})",
            1,
            "1",
        ),
        (
            "({v}) => [{v}].some({P})",
            format!("({{v}}) => {}", in_maps("v")),
            "{N}({v: 1})",
            1,
            "true",
        ),
        (
            "({v}) => [{v}].sortBy({P})[0].v",
            format!("({{v}}) => {}", in_maps("v")),
            "{N}({v: 1})",
            1,
            "1",
        ),
        (
            "({v}) => {a: {v}}.mapValues({P}).a",
            format!("({{v}}) => {}", in_maps("v")),
            "{N}({v: 1})",
            1,
            "1",
        ),
        (
            "(a, {v}) => [{v}].reduce({P}, a)",
            format!("(a, {{v}}) => {}", in_maps("v")),
            "{N}(0, {v: 1})",
            1,
            "1",
        ),
    ];
    for (link, f0, top, each, want) in cases {
        let n = d / each;
        let src = chain(&f0, link, top, n);
        assert_eq!(run(&src), Ok(want.into()), "{link}");
        let p = program(&env, &chain(&f0, link, top, n + 1));
        let Err(f) = p.outcome(&v).result else {
            panic!("{link}: past the limit, no failure");
        };
        assert_eq!(f.code, "depth-exceeded", "{link}");
        assert_eq!(f.trace.len(), d + 1, "{link}");
    }
    // A failure in the deepest call, with the trace of every call.
    let src = chain(
        &format!("x => {}", in_maps("fail(\"deep\")")),
        "x => [x].map({P})[0]",
        "{N}(1)",
        d,
    );
    let p = program(&env, &src);
    let Err(f) = p.outcome(&v).result else {
        panic!("no failure");
    };
    assert_eq!(f.code, "failed");
    assert_eq!(f.trace.len(), d);
    // F17 5.3: `walk` keeps its place in a stack of its own, so a chain of
    // nodes to the limit costs no host stack for its levels. The deepest
    // node's callback runs at depth d - 1, with the source nesting limit in
    // its body; one node more fails its depth test, with the one trace
    // entry of the walk. The lambda stands in the call's parentheses, so 96
    // levels of maps, the `?`, the brackets and the `+` reach the nesting
    // limit.
    let body = |last: usize| {
        let mut s = format!("k < {last} ? [x + 1] : null");
        for _ in 0..96 {
            s = format!("{{a: {s}}}.a");
        }
        s
    };
    let src = format!("walk(0, (x, k) => {})[{}]", body(d - 1), d - 1);
    assert_eq!(run(&src), Ok((d - 1).to_string()));
    let p = program(&env, &format!("walk(0, (x, k) => {})", body(d)));
    let Err(f) = p.outcome(&v).result else {
        panic!("walk past the limit: no failure");
    };
    assert_eq!(f.code, "depth-exceeded");
    assert_eq!(f.trace.len(), 1);
    // Data nested to the limit, walked with the identity, and one level more.
    // The walk stands at nesting 0, so its call measures 1.
    let src = "const r = walk(v, x => x); len(r)";
    assert_eq!(run(src), Ok(d.to_string()));
    let p = program(&env, src);
    assert_eq!(
        outcome(&p, &v2),
        Err("depth-exceeded".into()),
        "walk of data past the limit"
    );
    // Walks inside walks: each link calls the one before from the callback
    // of its walk, so a frame of each walk is live while the links under it
    // run. A link adds 4 to the depth count: 1 for its walk's call of the
    // callback, 3 for the call by name inside `? :` and `[...]`.
    let link = "x => walk(x, (y, k) => k < 1 ? [{P}(y)] : null)[1]";
    let links = (d - 1) / 4;
    assert_eq!(
        run(&chain("x => x", link, "{N}(1)", links + 1)),
        Ok("1".into())
    );
    let p = program(&env, &chain("x => x", link, "{N}(1)", links + 2));
    assert_eq!(outcome(&p, &v), Err("depth-exceeded".into()));
    // F17 11.6: a typed lambda that calls itself, directly and through
    // `map` and `flatMap`, each call adding 1. With N one less than the
    // limit the run reaches it; with N the limit it passes it.
    for (src, want) in [
        (
            "const d = (n: int): int => n <= 0 ? 0 : d(n - 1); d(N)",
            "0",
        ),
        (
            "const d = (n: int): int => n <= 0 ? 0 : [n - 1].map(d)[0]; d(N)",
            "0",
        ),
        (
            "const d = (n: int): int[] => n <= 0 ? [] : [n - 1].flatMap(d); d(N)",
            "[]",
        ),
    ] {
        let at = |n: usize| src.replace('N', &n.to_string());
        assert_eq!(run(&at(d - 1)), Ok(want.into()), "{src}");
        assert_eq!(run(&at(d)), Err("depth-exceeded".into()), "{src}");
    }
    // F17 13.4 item 6, the two forms of `03-host.md` 3.9.1: calls through
    // function parameters. In the first, `d` and `down` call each other,
    // `down` through its function parameter: a count of 2N + 2. In the
    // second, each call of `r` gives on a new lambda that calls the one
    // before, and the last call of `r` runs that chain: a count of 2N + 3.
    // With N the largest whose count stays within the limit the run gives
    // its value; with N one more it passes the limit.
    for (src, at_limit) in [
        (
            "const down = (f: (x: int) => int, n: int): int => n <= 0 ? 0 : f(n - 1); \
             const d = (n: int): int => down(d, n); d(N)",
            (d - 2) / 2,
        ),
        (
            "const r = (f: (x: int) => int, n: int): int => \
             n == 0 ? f(0) : r(y => f(y), n - 1); r(x => x, N)",
            (d - 3) / 2,
        ),
    ] {
        let at = |n: usize| src.replace('N', &n.to_string());
        assert_eq!(run(&at(at_limit)), Ok("0".into()), "{src}");
        assert_eq!(
            run(&at(at_limit + 1)),
            Err("depth-exceeded".into()),
            "{src}"
        );
    }
}

/// F17 11.5: each call reads the names of the calls on its own chain. A
/// lambda two frames down calls itself from a lambda three down and reads
/// the names of the frame above it after each call; one lambda calls
/// another of the same depth, whose call must give back the caller's frame;
/// and a failure that unwinds calls leaves the names of the `try`'s call as
/// they were. Each runs checked and with the static rules alone.
#[test]
fn each_call_reads_the_frames_of_its_own_chain() {
    let env = Env::default();
    let none = BTreeMap::new();
    let cases = [
        (
            "const outer = (a: int): int => { const inner = (b: int): int => \
             b <= 0 ? a : [b].map(k => inner(k - 1) + a + b)[0]; inner(a) }; \
             [outer(3), outer(1)]",
            "[18, 3]",
        ),
        (
            "const g = (n: int): int => n * 10; \
             const f = (n: int): int => n <= 0 ? 0 : g(n) + f(n - 1) + n; f(3)",
            "66",
        ),
        (
            "const f = (n: int): int => n <= 0 ? fail(\"x\") : f(n - 1); \
             const h = (n: int, acc: string): string => n <= 0 ? acc : \
             h(n - 1, acc + `${n}:${try { f(n) } catch { n }},`); h(3, \"\")",
            "\"3:3,2:2,1:1,\"",
        ),
    ];
    for (src, want) in cases {
        let unchecked = program(&env, src);
        assert_eq!(outcome(&unchecked, &none), Ok(want.into()), "{src}");
        let checked = env
            .compile(&parse(src).unwrap(), crate::CompileOptions::default())
            .unwrap_or_else(|e| panic!("{src}: {e}"));
        assert_eq!(outcome(&checked, &none), Ok(want.into()), "{src}");
    }
}

/// F17 13.2 item 4: a hint parameter is one of a `const`-bound lambda, also
/// one bound by a `const` line in a block; a parameter of a lambda written
/// as an argument, also inside such a lambda's body, is not one.
#[test]
fn a_hint_parameter_is_one_of_a_const_bound_lambda() {
    let env = Env::default();
    let messages = |src: &str| -> Vec<String> {
        let p = parse(src).unwrap();
        crate::check::check(&env, &p, &crate::check::Want::Nothing)
            .diagnostics
            .iter()
            .map(|d| format!("{} {}", d.code, d.message))
            .collect()
    };
    let hint = "call-of-parameter x is a parameter; to call it, give it a function type, \
                as x: (x: int) => int";
    let plain = "call-of-parameter x is a value, not a function, and cannot be called";
    for (src, want) in [
        ("{ const g = (x) => x(1); 1 }", hint),
        ("const h = (n) => { const g = (x) => x(n); 1 }; 1", hint),
        ("const h = (n) => { const g = [1].map(x => x(n)); 1 }; 1", plain),
        ("{ const g = (xs) => xs.map(x => x(1)); 1 }", plain),
    ] {
        assert_eq!(messages(src), vec![want.to_string()], "{src}");
    }
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

/// A depth limit outside 1 to 400 is `bad-limit` (`03-host.md` 3.3.4).
#[test]
fn a_depth_limit_out_of_range_is_bad_limit() {
    for depth in [0, 401, 1000] {
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
    for depth in [1, 400] {
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
    let env = Env::builder().var_record("{a: any}").build().unwrap();
    let p = program(&env, "a");
    let v = vars(&[("a", deep(1000))]);
    let out = p.outcome(&v);
    assert!(out.result.is_ok());
    assert_eq!(out.usage.steps, 0);
    // The outcome command's path, on the same data.
    let doc = crate::json::parse(
        r#"[{"name": "deep", "env": "{a: any}", "source": "a", "vars": {"a": 0}}]"#,
    )
    .unwrap();
    let mut cases = crate::testing::load_doc_lax(&doc, std::path::Path::new(".")).unwrap();
    let mut j = String::from("1");
    for _ in 0..1000 {
        j = format!("[{j}]");
    }
    cases[0].vars = Some(vec![("a".into(), crate::json::parse(&j).unwrap())]);
    let line = crate::testing::outcome_line(&cases[0]);
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
        let f = p.outcome(&none).result.err().unwrap();
        assert_eq!(f.message(), want);
        assert!(!f.message().contains('{'));
    }
}

/// `03-host.md` 3.2.1: compile checks by default; compiling without a check is an
/// explicit option, and gives the run's classes of the column "not checked".
#[test]
fn compile_checks_by_default() {
    let env = Env::builder()
        .var_record("{x: any, n: int | null}")
        .build()
        .unwrap();
    let e = env
        .compile(&parse("n + 1").unwrap(), Default::default())
        .err()
        .unwrap();
    assert_eq!(e.first().code, "maybe-null");
    assert!(compile(&env, parse("n + 1").unwrap()).is_ok());
    // The same failure has the checked class at a `dynamic` site, and the
    // unchecked class without a check.
    let v = vars(&[("x", Data::Float(1.0)), ("n", Data::Null)]);
    let p = env
        .compile(&parse("x.a").unwrap(), Default::default())
        .unwrap();
    let f = p.outcome(&v).result.err().unwrap();
    assert_eq!(p.class_of(&f), Some("dynamic"));
    let p = env
        .compile(&parse("match x { 2 => 1 }").unwrap(), Default::default())
        .err();
    assert!(p.is_some(), "an `any` subject needs `_`");
    let p = compile(&env, parse("match x { 2 => 1 }").unwrap()).unwrap();
    let f = p.outcome(&v).result.err().unwrap();
    assert_eq!(p.class_of(&f), Some("dynamic"));
    let r = env
        .check(&parse("x").unwrap(), &crate::Expected::None)
        .unwrap();
    assert_eq!(r.ty.to_string(), "any");
}

/// R73: the outcome line of a limit failure writes no span, as Go's and
/// TypeScript's do, though the diagnostic carries one (`03-host.md` 3.7.1).
#[test]
fn an_outcome_line_leaves_out_a_limits_span() {
    let doc =
        crate::json::parse(r#"[{"name": "steps", "source": "range(10)", "limits": {"steps": 1}}]"#)
            .unwrap();
    let cases = crate::testing::load_doc_lax(&doc, std::path::Path::new(".")).unwrap();
    let line = crate::testing::outcome_line(&cases[0]);
    assert!(
        line.contains(r#""code":"steps-exceeded","span":null"#),
        "{line}"
    );
}

/// R246: the outcome line writes `description` for a case with `types`
/// (F4 9.5).
#[test]
fn an_outcome_line_describes_a_case_with_types() {
    let doc = crate::json::parse(
        r#"[{"name": "t", "source": "1", "types": [{"name": "Order", "type": "{id: string}"}]}]"#,
    )
    .unwrap();
    let cases = crate::testing::load_doc_lax(&doc, std::path::Path::new(".")).unwrap();
    let line = crate::testing::outcome_line(&cases[0]);
    assert!(line.starts_with(r#"{"name":"t","description":{"#), "{line}");
    assert!(line.contains(r#""Order""#), "{line}");
}

/// `suite/README.md` item 1: the outcome command reads a format, completion
/// or hover case with no `expect`.
#[test]
fn an_outcome_line_for_tool_cases_with_no_expect() {
    let doc = crate::json::parse(
        r#"[{"name": "f", "format": "1+2"},
            {"name": "c", "complete": "type ‸"},
            {"name": "h", "hover": "const f = x => x; [‸f]"}]"#,
    )
    .unwrap();
    let cases = crate::testing::load_doc_lax(&doc, std::path::Path::new(".")).unwrap();
    let lines: Vec<String> = cases.iter().map(crate::testing::outcome_line).collect();
    assert!(
        lines[0].contains(r#""formatted":"1 + 2\n""#),
        "{}",
        lines[0]
    );
    assert!(lines[1].contains(r#""items":[]"#), "{}", lines[1]);
    assert!(lines[2].contains(r#""kind":"lambda""#), "{}", lines[2]);
    assert!(crate::testing::load_doc(&doc, std::path::Path::new(".")).is_err());
}

#[test]
fn precharge_allocation_before_building_containers_and_strings() {
    let xs = numbers(100_000);
    let m = Data::Map(
        (0..20_000)
            .map(|i| (format!("k{i}"), Data::Int(i)))
            .collect(),
    );
    let s = Data::Str("a\"\\\n\x01é😀".repeat(20_000));
    let v = vars(&[("xs", xs), ("m", m), ("s", s)]);

    let env = Env::builder()
        .var_record("{xs: float[], m: map<int>, s: string}")
        .groups(&["json"])
        .limits(Limits {
            steps: None,
            allocation: Some(5),
            ..Limits::default()
        })
        .build()
        .unwrap();

    for src in [
        "[...xs]",
        "{...m}",
        "`${s}${s}`",
        "string([s])",
        "json.stringify(s)",
    ] {
        let p = program(&env, src);
        assert_eq!(outcome(&p, &v), Err("allocation-exceeded".into()), "{src}");
    }

    // Later evaluation failures and step limits keep precedence.
    for (src, want) in [
        ("[...xs, 1 / 0]", "divide-by-zero"),
        ("{...m, [null]: 1}", "null-operand"),
        ("`${s}${1 / 0}`", "divide-by-zero"),
    ] {
        let p = program(&env, src);
        assert_eq!(outcome(&p, &v), Err(want.into()), "{src}");
    }

    let both = Env::builder()
        .var_record("{s: string}")
        .limits(Limits {
            steps: Some(2),
            allocation: Some(2),
            ..Limits::default()
        })
        .build()
        .unwrap();
    assert_eq!(
        outcome(&program(&both, "`${s}`"), &v),
        Err("steps-exceeded".into())
    );

    // Text past the length the walk keeps (64 KiB) is rendered a second
    // time, after the charges, to the same string.
    let ok_env = Env::builder()
        .var_record("{s: string}")
        .groups(&["json"])
        .limits(Limits {
            steps: None,
            allocation: None,
            ..Limits::default()
        })
        .build()
        .unwrap();
    let sample = "x\"\\\t\x01é😀".repeat(6000);
    let sv = vars(&[("s", Data::Str(sample.clone()))]);
    let p_tmpl = program(&ok_env, "`<${s}>`");
    let got_tmpl = p_tmpl.outcome(&sv).result.unwrap();
    assert_eq!(got_tmpl.as_str(), Some(format!("<{sample}>").as_str()));
    let want_json = crate::eval::to_json(&Value::from_data(&Data::Str(sample))).unwrap();
    let p_json = program(&ok_env, "json.stringify(s)");
    let got_json = p_json.outcome(&sv).result.unwrap();
    assert_eq!(got_json.as_str(), Some(want_json.as_str()));
    let p_str = program(&ok_env, "string([s])");
    let got_str = p_str.outcome(&sv).result.unwrap();
    assert_eq!(got_str.as_str(), Some(format!("[{want_json}]").as_str()));
    let p_mixed = program(&ok_env, "string([s, 1.5, true, null, {a: s}])");
    let got_mixed = p_mixed.outcome(&sv).result.unwrap();
    let want_mixed = format!("[{want_json},1.5,true,null,{{\"a\":{want_json}}}]");
    assert_eq!(got_mixed.as_str(), Some(want_mixed.as_str()));
    let p_parts = program(&ok_env, "`<${s}${1}${[s]}>`");
    let got_parts = p_parts.outcome(&sv).result.unwrap();
    let want_parts = format!("<{}1[{want_json}]>", "x\"\\\t\x01é😀".repeat(6000));
    assert_eq!(got_parts.as_str(), Some(want_parts.as_str()));
}
