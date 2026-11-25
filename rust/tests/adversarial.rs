//! Adversarial regression tests for limits, the host boundary, nested runs with
//! `remaining()`, host data conformance, and editor tools (issue #704).

use risor::{
    conform, conform_all, format, json_text, list_type, parse, parse_template, CompileOptions,
    Data, Env, Error, Expected, Limits, Pos, RunOptions, Type,
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
    assert!(matches!(got, risor::Value::Num(x) if x == 10_000.0));
    assert!(
        t0.elapsed() < Duration::from_secs(2),
        "time.duration took {:?}",
        t0.elapsed()
    );
}

#[test]
fn fail_with_shared_dag_is_bounded_inside_program_run() {
    let env = Env::builder().build().unwrap();
    let mut src = String::from("const a0 = [1, 1];\n");
    for i in 1..=30 {
        src.push_str(&format!("const a{i} = [a{}, a{}];\n", i - 1, i - 1));
    }
    src.push_str(r#"fail("boom", a30)"#);
    let p = env
        .compile(&parse(&src).unwrap(), CompileOptions::default())
        .unwrap();
    let empty: BTreeMap<String, Data> = BTreeMap::new();
    let t0 = Instant::now();
    let err = p.run(&empty).expect_err("expected fail error");
    assert_eq!(err.first().code, "failed");
    assert!(
        t0.elapsed() < Duration::from_secs(2),
        "fail(DAG) took {:?}",
        t0.elapsed()
    );
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
    assert!(matches!(v, risor::Value::Num(x) if x.to_bits() == 0.0f64.to_bits()));

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
