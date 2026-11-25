//! The call handle's `remaining` (`03-host.md` 3.5.2 item 3), and a nested
//! run that takes what the caller has left and charges back what it used
//! (`03-host.md` 3.2.5). The numbers are those of the Go and TypeScript
//! tests of the same name.

use risor::{
    parse, Call, CompileOptions, Data, Env, Expr, HostError, Limits, NoVars, Remaining, Usage,
    Value,
};
use std::collections::BTreeMap;
use std::sync::atomic::{AtomicUsize, Ordering};
use std::sync::{Arc, Mutex};

fn unchecked() -> CompileOptions {
    CompileOptions {
        check: false,
        ..CompileOptions::default()
    }
}

fn limits(steps: Option<u64>, allocation: Option<u64>) -> Limits {
    Limits {
        steps,
        allocation,
        ..Limits::default()
    }
}

/// The code of a failed run, or "", and the usage.
fn measure(env: &Env, src: &str) -> (String, Usage) {
    let p = env.compile(&parse(src).unwrap(), unchecked()).unwrap();
    let (r, u) = p.measure(&NoVars);
    (
        r.err()
            .map(|e| e.first().code.to_string())
            .unwrap_or_default(),
        u,
    )
}

fn stop(_: risor::Stop) -> HostError {
    HostError::Text("stop".into())
}

#[test]
fn remaining_gives_what_is_left() {
    let got: Arc<Mutex<Vec<Remaining>>> = Arc::default();
    let g = got.clone();
    let probe = move |c: &mut Call<'_>, _: &[Value<'_>]| {
        g.lock().unwrap().push(c.remaining());
        c.charge(5).map_err(stop)?;
        c.allocate(3).map_err(stop)?;
        g.lock().unwrap().push(c.remaining());
        Ok(Data::Int(0))
    };
    let r = |s, a| Remaining {
        steps: s,
        allocation: a,
    };
    let env = Env::builder()
        .limits(limits(Some(100), Some(50)))
        .function("probe", "() => int", probe)
        .build()
        .unwrap();
    // The call's one step is charged before the function runs.
    let (code, u) = measure(&env, "probe()");
    assert_eq!(code, "");
    assert_eq!(
        *got.lock().unwrap(),
        [r(Some(99), Some(50)), r(Some(94), Some(47))]
    );
    assert_eq!((u.steps, u.allocated), (6, 3));
    // Literals cost no step; the list's 4 units come after its elements.
    got.lock().unwrap().clear();
    let (code, u) = measure(&env, "[1, 2, probe()]");
    assert_eq!(code, "");
    assert_eq!(
        *got.lock().unwrap(),
        [r(Some(99), Some(50)), r(Some(94), Some(47))]
    );
    assert_eq!((u.steps, u.allocated), (6, 7));
    // A limit that is off gives None.
    got.lock().unwrap().clear();
    let off = env.with().limits(limits(None, None)).build().unwrap();
    assert_eq!(measure(&off, "probe()").0, "");
    assert_eq!(*got.lock().unwrap(), [r(None, None), r(None, None)]);
}

#[test]
fn a_failed_charge_changes_nothing() {
    let got: Arc<Mutex<Vec<Remaining>>> = Arc::default();
    let g = got.clone();
    let env = Env::builder()
        .limits(limits(Some(10), Some(10)))
        .function("f", "() => int", move |c, _| {
            let _ = c.charge(20);
            g.lock().unwrap().push(c.remaining());
            Ok(Data::Int(0))
        })
        .build()
        .unwrap();
    assert_eq!(measure(&env, "f()").0, "steps-exceeded");
    assert_eq!(
        *got.lock().unwrap(),
        [Remaining {
            steps: Some(9),
            allocation: Some(10)
        }]
    );
}

const INNER: &str = "xs.map(x => x * 2)";

fn inner_vars() -> BTreeMap<String, Data> {
    BTreeMap::from([(
        "xs".to_string(),
        Data::List((1..=10).map(Data::Int).collect()),
    )])
}

fn tool_env() -> Env {
    Env::builder().var("xs", "int[]").build().unwrap()
}

#[test]
fn a_remaining_of_zero_is_a_limit_of_zero() {
    let vars = inner_vars();
    let env = tool_env();
    let zero = env.with().limits(limits(Some(0), None)).build().unwrap();
    let p = zero.compile(&parse("xs[0]").unwrap(), unchecked()).unwrap();
    assert_eq!(p.run(&vars).unwrap_err().first().code, "steps-exceeded");
    let zero = env.with().limits(limits(None, Some(0))).build().unwrap();
    let p = zero
        .compile(&parse("xs.map(x => x)").unwrap(), unchecked())
        .unwrap();
    assert_eq!(
        p.run(&vars).unwrap_err().first().code,
        "allocation-exceeded"
    );
}

/// Runs the inner program with what the caller has left, then charges back
/// what it used.
fn nested_tool(
    calls: Arc<AtomicUsize>,
) -> impl Fn(&mut Call<'_>, &[Value<'_>]) -> Result<Data, HostError> + Send + Sync {
    let env = tool_env();
    let inner: Expr = parse(INNER).unwrap();
    move |c, _| {
        calls.fetch_add(1, Ordering::Relaxed);
        let left = c.remaining();
        let child = env
            .with()
            .limits(Limits {
                steps: left.steps,
                allocation: left.allocation,
                ..Limits::default()
            })
            .build()?;
        let p = child.compile(&inner, CompileOptions::default())?;
        let vars = inner_vars();
        let (result, used) = p.measure(&vars);
        c.charge(used.steps).map_err(stop)?;
        c.allocate(used.allocated).map_err(stop)?;
        match result {
            Ok(_) => Ok(Data::Int(1)),
            // A nested run that stopped at a limit stops the caller with the
            // same code: more than is left cannot be charged.
            Err(e) => match e.first().code {
                "steps-exceeded" => Err(c.charge(u64::MAX).map_err(stop).unwrap_err()),
                "allocation-exceeded" => Err(c.allocate(u64::MAX).map_err(stop).unwrap_err()),
                _ => Err(e.into()),
            },
        }
    }
}

fn cost() -> Usage {
    let p = tool_env()
        .compile(&parse(INNER).unwrap(), unchecked())
        .unwrap();
    let vars = inner_vars();
    let (_, used) = p.measure(&vars);
    used
}

/// Charges what the inner program costs, as if its work were done in the
/// caller.
fn inline_tool(
    calls: Arc<AtomicUsize>,
) -> impl Fn(&mut Call<'_>, &[Value<'_>]) -> Result<Data, HostError> + Send + Sync {
    let cost = cost();
    move |c, _| {
        calls.fetch_add(1, Ordering::Relaxed);
        c.charge(cost.steps).map_err(stop)?;
        c.allocate(cost.allocated).map_err(stop)?;
        Ok(Data::Int(1))
    }
}

#[derive(Clone, Copy)]
enum Tool {
    Nested,
    Inline,
}

fn outcome(tool: Tool, l: Limits, src: &str) -> (String, Usage, usize) {
    let calls = Arc::new(AtomicUsize::new(0));
    let b = Env::builder().limits(l);
    let b = match tool {
        Tool::Nested => b.function("tool", "() => int !", nested_tool(calls.clone())),
        Tool::Inline => b.function("tool", "() => int !", inline_tool(calls.clone())),
    };
    let (code, u) = measure(&b.build().unwrap(), src);
    (code, u, calls.load(Ordering::Relaxed))
}

#[test]
fn a_nested_run_with_the_remaining_budget() {
    let cost = cost();
    assert_eq!((cost.steps, cost.allocated), (32, 11));
    let src = "[tool(), tool(), tool()]";

    // Unlimited, the caller's usage is its own plus the inner runs'.
    let off = limits(None, None);
    let (c1, nested, n) = outcome(Tool::Nested, off, src);
    let (c2, inline, _) = outcome(Tool::Inline, off, src);
    assert_eq!((c1.as_str(), c2.as_str(), n), ("", "", 3));
    assert_eq!(nested, inline);
    assert_eq!(
        (nested.steps, nested.allocated),
        (3 * cost.steps + 3, 3 * cost.allocated + 4)
    );

    // The nested run gets exactly what is left.
    let d = Limits::default();
    let (code, u, _) = outcome(
        Tool::Nested,
        limits(Some(1 + cost.steps), d.allocation),
        "tool()",
    );
    assert_eq!(code, "");
    assert_eq!((u.steps, u.allocated), (1 + cost.steps, cost.allocated));
    let (code, _, _) = outcome(
        Tool::Nested,
        limits(Some(cost.steps), d.allocation),
        "tool()",
    );
    assert_eq!(code, "steps-exceeded");
    let (code, u, _) = outcome(
        Tool::Nested,
        limits(d.steps, Some(cost.allocated)),
        "tool()",
    );
    assert_eq!(code, "");
    assert_eq!(u.allocated, cost.allocated);
    let (code, _, _) = outcome(
        Tool::Nested,
        limits(d.steps, Some(cost.allocated - 1)),
        "tool()",
    );
    assert_eq!(code, "allocation-exceeded");

    // At every limit, the nested and the inline tool stop at the same call
    // with the same code, and a run that succeeds has the same usage.
    // `catch` catches neither.
    for s in [src, "try { [tool(), tool(), tool()] } catch { [] }"] {
        for limit in 0..=nested.steps {
            let l = limits(Some(limit), d.allocation);
            let (c1, u1, n1) = outcome(Tool::Nested, l, s);
            let (c2, u2, n2) = outcome(Tool::Inline, l, s);
            assert_eq!((&c1, n1), (&c2, n2), "limit {limit}");
            if c1.is_empty() {
                assert_eq!(u1, u2, "limit {limit}");
            }
        }
        for limit in 0..=nested.allocated {
            let l = limits(d.steps, Some(limit));
            let (c1, u1, n1) = outcome(Tool::Nested, l, s);
            let (c2, u2, n2) = outcome(Tool::Inline, l, s);
            assert_eq!((&c1, n1), (&c2, n2), "units {limit}");
            if c1.is_empty() {
                assert_eq!(u1, u2, "units {limit}");
            }
        }
    }
}
