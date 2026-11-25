//! Adversarial regression tests for limits, the host boundary, nested runs with
//! `remaining()`, host data conformance, and editor tools (issue #704).

use risor::{
    conform, conform_all, format, json_text, list_type, parse, parse_template, CompileOptions,
    Data, Env, Error, Expected, HostList, HostMap, Limits, Pos, RunOptions, Type, View,
};
use std::collections::BTreeMap;
use std::sync::{
    atomic::{AtomicBool, Ordering},
    Arc,
};
use std::time::{Duration, Instant};

fn err_code<T>(res: Result<T, Error>) -> &'static str {
    res.err().expect("expected error").first().code
}

#[test]
fn limits_exponential_and_oom_before_charge() {
    let env = Env::builder()
        .groups(&["time", "json", "base64", "math"])
        .var("s", "string")
        .var("xs", "any[]")
        .build()
        .unwrap();

    let empty: BTreeMap<String, Data> = BTreeMap::new();
    let p_repeat = env
        .compile(
            &parse(r#""a".repeat(1000000000000000)"#).unwrap(),
            CompileOptions::default(),
        )
        .unwrap();
    assert_eq!(err_code(p_repeat.run(&empty)), "steps-exceeded");

    let p_range = env
        .compile(
            &parse("range(1000000000000000)").unwrap(),
            CompileOptions::default(),
        )
        .unwrap();
    assert_eq!(err_code(p_range.run(&empty)), "steps-exceeded");

    let p_pow = env
        .compile(&parse("2 ** 10000").unwrap(), CompileOptions::default())
        .unwrap();
    assert_eq!(err_code(p_pow.run(&empty)), "number-range");

    // OOM-before-charge: flatMap must charge k steps and 1 + k units before flattening.
    let tiny = env
        .with()
        .limits(Limits {
            steps: Some(10_000_000),
            allocation: Some(200),
            ..Limits::default()
        })
        .build()
        .unwrap();
    let p_flat = tiny
        .compile(
            &parse("xs.flatMap(_ => xs)").unwrap(),
            CompileOptions::default(),
        )
        .unwrap();
    let mut vars = BTreeMap::new();
    vars.insert("s".into(), Data::Str(String::new()));
    vars.insert("xs".into(), Data::List(vec![Data::Int(1); 2_000]));
    let t0 = Instant::now();
    assert_eq!(err_code(p_flat.run(&vars)), "allocation-exceeded");
    assert!(
        t0.elapsed() < Duration::from_secs(2),
        "flatMap took {:?}",
        t0.elapsed()
    );

    // OOM-before-charge: json.parse, padStart, and list.map with tiny allocation limit.
    let tiny2 = env
        .with()
        .limits(Limits {
            steps: Some(10_000_000),
            allocation: Some(5),
            ..Limits::default()
        })
        .build()
        .unwrap();
    let p_json = tiny2
        .compile(&parse("json.parse(s)").unwrap(), CompileOptions::default())
        .unwrap();
    let mut jvars = BTreeMap::new();
    jvars.insert(
        "s".into(),
        Data::Str(format!("[{}]", vec!["1"; 50_000].join(","))),
    );
    jvars.insert("xs".into(), Data::List(vec![]));
    assert_eq!(err_code(p_json.run(&jvars)), "allocation-exceeded");

    let p_pad = tiny2
        .compile(
            &parse(r#""x".padStart(100000, s)"#).unwrap(),
            CompileOptions::default(),
        )
        .unwrap();
    let mut pvars = BTreeMap::new();
    pvars.insert("s".into(), Data::Str("a".repeat(100_000)));
    pvars.insert("xs".into(), Data::List(vec![]));
    assert_eq!(err_code(p_pad.run(&pvars)), "allocation-exceeded");

    // json.parse with many duplicate keys deduplicates in place during build
    // and keeps the last value with allocation charged only for distinct keys.
    let dup_json = format!("{{{}\"a\":42}}", "\"a\":0,".repeat(20_000));
    let mut dvars = BTreeMap::new();
    dvars.insert("s".into(), Data::Str(dup_json));
    dvars.insert("xs".into(), Data::List(vec![]));
    let p_dup = env
        .compile(&parse("json.parse(s)").unwrap(), CompileOptions::default())
        .unwrap();
    let (dup_res, dup_usage) = p_dup.measure(&dvars);
    assert_eq!(
        dup_res.unwrap().to_data(),
        Data::Map(BTreeMap::from([("a".into(), Data::Float(42.0))]))
    );
    assert_eq!(dup_usage.allocated, 3);

    // flatMap with a HostList reporting len == usize::MAX (or two lists whose
    // lengths overflow usize) fails cleanly with allocation-exceeded without
    // wrapping 1 + k or panicking in Vec::with_capacity.
    struct HugeList(usize);
    impl HostList for HugeList {
        fn len(&self) -> usize {
            self.0
        }
        fn get(&self, _index: usize) -> Option<View<'_>> {
            None
        }
    }
    struct HugeVars<'a> {
        huge: &'a HugeList,
    }
    impl risor::Vars for HugeVars<'_> {
        fn get(&self, name: &str) -> Option<View<'_>> {
            match name {
                "xs" => Some(View::List(self.huge)),
                _ => None,
            }
        }
    }
    let no_steps = Env::builder()
        .var("xs", "any[]")
        .limits(Limits {
            steps: None,
            allocation: Some(10),
            ..Limits::default()
        })
        .build()
        .unwrap();
    let p_one = no_steps
        .compile(
            &parse("[1].flatMap(_ => xs)").unwrap(),
            CompileOptions::default(),
        )
        .unwrap();
    let huge_max = HugeList(usize::MAX);
    assert_eq!(
        err_code(p_one.run(&HugeVars { huge: &huge_max })),
        "allocation-exceeded"
    );
    let p_two = no_steps
        .compile(
            &parse("[1, 2].flatMap(_ => xs)").unwrap(),
            CompileOptions::default(),
        )
        .unwrap();
    let huge_half = HugeList(usize::MAX / 2 + 1);
    assert_eq!(
        err_code(p_two.run(&HugeVars { huge: &huge_half })),
        "allocation-exceeded"
    );
}

#[test]
fn time_duration_is_linear_on_many_terms_with_long_fraction() {
    let env = Env::builder()
        .groups(&["time"])
        .var("s", "string")
        .build()
        .unwrap();
    let p = env
        .compile(
            &parse("time.duration(s)").unwrap(),
            CompileOptions::default(),
        )
        .unwrap();
    let mut vars = BTreeMap::new();
    vars.insert(
        "s".into(),
        Data::Str(format!("0.{}1s{}", "0".repeat(10_000), "1s".repeat(10_000))),
    );
    let t0 = Instant::now();
    let got = p.run(&vars).unwrap();
    assert_eq!(got.as_f64(), Some(10_000.0));
    assert!(
        t0.elapsed() < Duration::from_secs(2),
        "time.duration took {:?}",
        t0.elapsed()
    );

    // Huge fraction uses fixed-column scratch memory and charges 0 allocation units.
    let zero_alloc = env
        .with()
        .limits(Limits {
            steps: Some(1_000_000),
            allocation: Some(0),
            ..Limits::default()
        })
        .build()
        .unwrap();
    let p_zero = zero_alloc
        .compile(
            &parse("time.duration(s)").unwrap(),
            CompileOptions::default(),
        )
        .unwrap();
    let mut huge_frac = BTreeMap::new();
    huge_frac.insert(
        "s".into(),
        Data::Str(format!("1.{}5s", "0".repeat(200_000))),
    );
    let (res, usage) = p_zero.measure(&huge_frac);
    assert_eq!(res.unwrap().as_f64(), Some(1.0));
    assert_eq!(usage.allocated, 0);

    // Exact midpoint tiebreaking around 1 + 2^-53 (midpoint between 1.0 and 1.0 + 2^-52).
    let next_up = f64::from_bits(1.0f64.to_bits() + 1);
    let mid_exact = "1.00000000000000011102230246251565404236316680908203125";
    let below_mid = format!(
        "1.00000000000000011102230246251565404236316680908203124{}s",
        "9".repeat(1120 - 53)
    );
    let tiny = format!("0.{}1s", "0".repeat(1120));
    let eval_dur = |text: String| -> f64 {
        let mut m = BTreeMap::new();
        m.insert("s".into(), Data::Str(text));
        let v = p.run(&m).unwrap();
        v.as_f64()
            .unwrap_or_else(|| panic!("expected number, got {v:?}"))
    };
    assert_eq!(
        eval_dur(format!("{mid_exact}s0.{}1s", "0".repeat(200_000))),
        next_up
    );
    assert_eq!(eval_dur(format!("{below_mid}{}", tiny.repeat(9))), 1.0);
    assert_eq!(eval_dur(format!("{below_mid}{}", tiny.repeat(10))), 1.0);
    assert_eq!(eval_dur(format!("{below_mid}{}", tiny.repeat(11))), next_up);
    // Multi-window tiebreaker past column 2144.
    let nines_win = format!("0.{}{}s", "0".repeat(1120), "9".repeat(1024));
    let tiny2 = format!("0.{}1s", "0".repeat(2144));
    assert_eq!(
        eval_dur(format!("{below_mid}{nines_win}{}", tiny2.repeat(10))),
        1.0
    );
    assert_eq!(
        eval_dur(format!("{below_mid}{nines_win}{}", tiny2.repeat(11))),
        next_up
    );
}

fn count_data_nodes(d: &Data) -> usize {
    match d {
        Data::List(items) => 1 + items.iter().map(count_data_nodes).sum::<usize>(),
        Data::Map(entries) => 1 + entries.values().map(count_data_nodes).sum::<usize>(),
        _ => 1,
    }
}

#[test]
fn fail_with_shared_dag_is_bounded_inside_program_run() {
    let env = Env::builder()
        .var("xs", "any[]")
        .var("m", "map<any>")
        .build()
        .unwrap();
    let mut src = String::from("const a0 = [1, 1];\n");
    for i in 1..=30 {
        src.push_str(&format!("const a{i} = [a{}, a{}];\n", i - 1, i - 1));
    }
    src.push_str(r#"fail("boom", a30)"#);
    let p = env
        .compile(&parse(&src).unwrap(), CompileOptions::default())
        .unwrap();
    let mut empty: BTreeMap<String, Data> = BTreeMap::new();
    empty.insert("xs".into(), Data::List(vec![]));
    empty.insert("m".into(), Data::Map(BTreeMap::new()));
    let t0 = Instant::now();
    let err = p.run(&empty).expect_err("expected fail error");
    assert_eq!(err.first().code, "failed");
    assert!(
        t0.elapsed() < Duration::from_secs(2),
        "fail(DAG) took {:?}",
        t0.elapsed()
    );
    let dag_data = err.first().data.as_deref().expect("expected data");
    assert!(count_data_nodes(dag_data) <= 100_000);
    match dag_data {
        Data::List(top) => assert_eq!(top.len(), 2),
        other => panic!("expected 2-element top list, got {other:?}"),
    }

    // A container that fits within the 100,000-node/slot budget converts
    // completely without truncation.
    let p_big = env
        .compile(
            &parse(r#"fail("big", xs)"#).unwrap(),
            CompileOptions::default(),
        )
        .unwrap();
    let mut fits_vars = BTreeMap::new();
    fits_vars.insert("xs".into(), Data::List(vec![Data::Int(1); 99_999]));
    fits_vars.insert("m".into(), Data::Map(BTreeMap::new()));
    let err_fits = p_big.run(&fits_vars).expect_err("expected fail error");
    match err_fits.first().data.as_deref() {
        Some(d @ Data::List(list)) => {
            assert_eq!(list.len(), 99_999);
            assert_eq!(list[0], Data::Float(1.0));
            assert_eq!(list[99_998], Data::Float(1.0));
            assert_eq!(count_data_nodes(d), 100_000);
        }
        other => panic!("expected complete 99,999-element Data::List, got {other:?}"),
    }

    // A container wider than the remaining slot budget collapses as a whole
    // subtree to Data::Null instead of emitting >100,000 slots or a truncated list.
    let mut over_vars = BTreeMap::new();
    over_vars.insert("xs".into(), Data::List(vec![Data::Int(1); 100_005]));
    over_vars.insert("m".into(), Data::Map(BTreeMap::new()));
    let err_over = p_big.run(&over_vars).expect_err("expected fail error");
    assert_eq!(err_over.first().code, "failed");
    assert_eq!(err_over.first().data.as_deref(), Some(&Data::Null));

    // Shared wide-container DAG: 100 references to a 5,000-element list emit
    // at most 100,000 total slots; the first 19 expand completely and the rest
    // collapse cleanly as Data::Null.
    let p_wide = env
        .compile(
            &parse(r#"const wide = range(5000); fail("wide", range(100).map(_ => wide))"#).unwrap(),
            CompileOptions::default(),
        )
        .unwrap();
    let t_wide = Instant::now();
    let err_wide = p_wide.run(&empty).expect_err("expected fail error");
    assert!(t_wide.elapsed() < Duration::from_secs(2));
    let wide_data = err_wide.first().data.as_deref().expect("expected data");
    assert!(count_data_nodes(wide_data) <= 100_000);
    match wide_data {
        Data::List(outer) => {
            assert_eq!(outer.len(), 100);
            assert!(matches!(&outer[0], Data::List(inner) if inner.len() == 5_000));
            assert!(matches!(&outer[18], Data::List(inner) if inner.len() == 5_000));
            assert_eq!(outer[19], Data::Null);
            assert_eq!(outer[99], Data::Null);
        }
        other => panic!("expected 100-element outer Data::List, got {other:?}"),
    }

    // Map with an expanding subtree keeps all keys and collapses any container
    // child whose slots exceed the remaining budget.
    let mut map_src = String::from("const a0 = [1, 1];\n");
    for i in 1..=17 {
        map_src.push_str(&format!("const a{i} = [a{}, a{}];\n", i - 1, i - 1));
    }
    map_src.push_str(r#"fail("bigm", {a: a17, b: [1, 2]})"#);
    let p_map = env
        .compile(&parse(&map_src).unwrap(), CompileOptions::default())
        .unwrap();
    let err_map = p_map.run(&empty).expect_err("expected fail error");
    assert_eq!(err_map.first().code, "failed");
    let map_data = err_map.first().data.as_deref().expect("expected data");
    assert!(count_data_nodes(map_data) <= 100_000);
    match map_data {
        Data::Map(m) => {
            assert_eq!(m.len(), 2);
            assert!(m.contains_key("a"));
            assert_eq!(m.get("b"), Some(&Data::Null));
        }
        other => panic!("expected Data::Map, got {other:?}"),
    }

    // Oversized host map collapses to Data::Null without enumerating its keys.
    struct PanicKeysMap(usize);
    impl HostMap for PanicKeysMap {
        fn len(&self) -> usize {
            self.0
        }
        fn get(&self, _key: &str) -> Option<View<'_>> {
            None
        }
        fn keys(&self) -> Vec<&str> {
            panic!("oversized host map keys() should not be enumerated")
        }
    }
    struct BigMapVars<'a> {
        map: &'a PanicKeysMap,
    }
    impl risor::Vars for BigMapVars<'_> {
        fn get(&self, name: &str) -> Option<View<'_>> {
            match name {
                "m" => Some(View::Map(self.map)),
                _ => None,
            }
        }
    }
    let p_big_map = env
        .compile(
            &parse(r#"fail("bigm", m)"#).unwrap(),
            CompileOptions::default(),
        )
        .unwrap();
    let big_map = PanicKeysMap(100_005);
    let err_big_map = p_big_map
        .run(&BigMapVars { map: &big_map })
        .expect_err("expected fail error");
    assert_eq!(err_big_map.first().code, "failed");
    assert_eq!(err_big_map.first().data.as_deref(), Some(&Data::Null));
}

#[test]
fn source_size_nesting_checker_depth_and_deadline() {
    let env = Env::builder().var("a", "int").build().unwrap();
    let big = format!("1{}", " ".repeat(1_048_576));
    assert_eq!(err_code(parse(&big)), "source-too-large");
    assert_eq!(err_code(parse_template(&big)), "source-too-large");
    assert_eq!(err_code(format(&big)), "source-too-large");
    assert!(env
        .complete(&big, Pos { line: 1, column: 2 }, &Expected::default())
        .items
        .is_empty());
    assert!(env
        .hover(&big, Pos { line: 1, column: 2 }, &Expected::default())
        .is_none());

    assert_eq!(err_code(parse(&"(".repeat(500))), "nesting-exceeded");

    // 25-level DAG record type hits check-limit.
    let mut dag = String::from("const a0 = {x: 1, y: 1};\n");
    for i in 1..=25 {
        dag.push_str(&format!("const a{i} = {{x: a{}, y: a{}}};\n", i - 1, i - 1));
    }
    dag.push_str("a25");
    assert_eq!(
        err_code(env.compile(&parse(&dag).unwrap(), CompileOptions::default())),
        "check-limit"
    );

    // Depth limit across recursion and walk.
    let rec = Env::builder()
        .limits(Limits {
            depth: 20,
            ..Limits::default()
        })
        .build()
        .unwrap();
    let empty: BTreeMap<String, Data> = BTreeMap::new();
    let p_rec = rec
        .compile(
            &parse("const f = (n: int): int => n <= 0 ? 0 : f(n - 1); f(100)").unwrap(),
            CompileOptions::default(),
        )
        .unwrap();
    assert_eq!(err_code(p_rec.run(&empty)), "depth-exceeded");
    let p_walk = rec
        .compile(
            &parse("walk(0, (n, d) => d < 50 ? [n + 1] : null)").unwrap(),
            CompileOptions::default(),
        )
        .unwrap();
    assert_eq!(err_code(p_walk.run(&empty)), "depth-exceeded");

    // Deadline and cancellation in tight step loop.
    let dl = Env::builder()
        .limits(Limits {
            steps: None,
            allocation: None,
            deadline: Some(Duration::from_millis(25)),
            ..Limits::default()
        })
        .build()
        .unwrap();
    let p_loop = dl
        .compile(
            &parse("const xs = range(10000); xs.map(x => xs.map(y => x + y))").unwrap(),
            CompileOptions::default(),
        )
        .unwrap();
    let t0 = Instant::now();
    assert_eq!(err_code(p_loop.run(&empty)), "deadline-exceeded");
    assert!(t0.elapsed() < Duration::from_secs(2));

    let cancel = Arc::new(AtomicBool::new(true));
    let opts = RunOptions {
        cancel: Some(cancel),
    };
    assert_eq!(err_code(p_loop.run_with(&empty, opts)), "cancelled");
}

#[test]
fn host_boundary_conformance_and_nested_remaining() {
    let env = Env::builder().var("x", "any").build().unwrap();
    let p = env
        .compile(&parse("x").unwrap(), CompileOptions::default())
        .unwrap();
    for bad in [
        Data::Int(i64::MAX),
        Data::Float(f64::NAN),
        Data::Float(f64::INFINITY),
    ] {
        let mut vars = BTreeMap::new();
        vars.insert("x".into(), bad);
        assert_eq!(err_code(p.run(&vars)), "number-range");
    }
    let mut neg_zero = BTreeMap::new();
    neg_zero.insert("x".into(), Data::Float(-0.0));
    let v = p.run(&neg_zero).unwrap();
    assert!(matches!(v.as_f64(), Some(x) if x.to_bits() == 0.0f64.to_bits()));

    // ConformAll caps errors and enforces step limit.
    let big_bad = Data::List(vec![Data::Str("bad".into()); 10_000]);
    let val = risor::Value::from_data(&big_bad);
    let ds = conform_all(&val, &list_type(&Type::INT), 5);
    assert_eq!(ds.len(), 5);
    assert!(conform(&val, &list_type(&Type::INT)).is_err());

    // json_text enforces depth 200 on deeply nested lists.
    let mut deep = Data::Int(1);
    for _ in 0..210 {
        deep = Data::List(vec![deep]);
    }
    let deep_val = risor::Value::from_data(&deep);
    assert_eq!(err_code(json_text(&deep_val)), "depth-exceeded");

    // Nested run via remaining() cannot amplify steps even inside try/catch.
    let child_env = Env::builder().build().unwrap();
    let child_expr = parse("range(20).map(x => x + 1)").unwrap();
    let parent = Env::builder()
        .limits(Limits {
            steps: Some(100),
            allocation: Some(100),
            ..Limits::default()
        })
        .function("sub", "() => int !", move |call, _args| {
            let rem = call.remaining();
            let child = child_env
                .with()
                .limits(Limits {
                    steps: rem.steps,
                    allocation: rem.allocation,
                    ..Limits::default()
                })
                .build()
                .map_err(risor::HostError::from)?;
            let prog = child
                .compile(&child_expr, CompileOptions::default())
                .map_err(risor::HostError::from)?;
            let empty: BTreeMap<String, Data> = BTreeMap::new();
            let (res, usage) = prog.measure(&empty);
            let _ = call.charge(usage.steps);
            let _ = call.allocate(usage.allocated);
            if let Err(e) = res {
                match e.first().code {
                    "steps-exceeded" => {
                        let _ = call.charge(u64::MAX);
                    }
                    "allocation-exceeded" => {
                        let _ = call.allocate(u64::MAX);
                    }
                    _ => {}
                }
                return Err(risor::HostError::from(e));
            }
            Ok(Data::Int(1))
        })
        .build()
        .unwrap();
    let p_parent = parent
        .compile(
            &parse("try { range(10).map(_ => sub()) } catch { [] }").unwrap(),
            CompileOptions::default(),
        )
        .unwrap();
    let empty: BTreeMap<String, Data> = BTreeMap::new();
    assert_eq!(err_code(p_parent.run(&empty)), "steps-exceeded");
    let _ = Ordering::Relaxed;
}

#[test]
fn editor_tools_on_hostile_text() {
    let env = Env::builder()
        .var("a", "int")
        .var("s", "string")
        .build()
        .unwrap();
    let exp = Expected::default();
    let cases = [
        "(".repeat(5_000),
        "{".repeat(5_000),
        "[".repeat(5_000),
        "const a = ; ".repeat(5_000),
        "a + ".repeat(10_000),
        "if (1) { ".repeat(500),
        format!("\"\"\"\n{}", "a".repeat(20_000)),
        "/*\n".repeat(5_000),
        "const a = 1;\0\0\0a.".to_string(),
    ];
    for text in &cases {
        let t0 = Instant::now();
        let _ = env.complete(
            text,
            Pos {
                line: 1,
                column: (text.len() + 1) as u32,
            },
            &exp,
        );
        let _ = env.hover(text, Pos { line: 1, column: 2 }, &exp);
        let _ = format(text);
        assert!(
            t0.elapsed() < Duration::from_secs(2),
            "editor tools took {:?} on {:?}",
            t0.elapsed(),
            &text[..text.len().min(20)]
        );
    }
}
