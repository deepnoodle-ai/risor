//! The shared benchmark programs of `suite/bench`, run through the host API
//! only, as the Go and TypeScript harnesses run them:
//! `cargo run --release --example bench [-- NAME-FILTER]`.
//!
//! For each program it first checks the value, steps and units against the
//! file's expect, then measures:
//!
//! - `compile_ns`: `parse` and `compile` (with the check) of the source
//!   text, in an environment built once;
//! - `run_ns`: one run of the compiled program on the file's vars.
//!
//! Each is the median of 15 samples, after a warm-up of 200 ms; a sample is a
//! batch of calls that takes at least 10 ms, divided by its size. It prints
//! one JSON line for each program: impl, program, compile_ns, run_ns, steps,
//! allocated.

use risor::json::Json;
use risor::value::eq_data;
use risor::{compile, parse, Data, Env, Limits};
use std::collections::BTreeMap;
use std::hint::black_box;
use std::path::Path;
use std::time::{Duration, Instant};

fn data(j: &Json) -> Data {
    match j {
        Json::Null => Data::Null,
        Json::Bool(b) => Data::Bool(*b),
        Json::Num(n) => Data::Float(*n),
        Json::Str(s) => Data::Str(s.clone()),
        Json::Arr(a) => Data::List(a.iter().map(data).collect()),
        Json::Obj(o) => Data::Map(o.iter().map(|(k, v)| (k.clone(), data(v))).collect()),
    }
}

fn median(mut f: impl FnMut()) -> f64 {
    const WARM: Duration = Duration::from_millis(200);
    const MIN_SAMPLE: Duration = Duration::from_millis(10);
    const SAMPLES: usize = 15;
    let start = Instant::now();
    let mut calls = 0;
    while calls < 10 || start.elapsed() < WARM {
        f();
        calls += 1;
    }
    let mut batch = |n: u32| {
        let t = Instant::now();
        for _ in 0..n {
            f();
        }
        t.elapsed()
    };
    let mut n = 1;
    while batch(n) < MIN_SAMPLE {
        n *= 2;
    }
    let mut per: Vec<f64> = (0..SAMPLES)
        .map(|_| batch(n).as_nanos() as f64 / n as f64)
        .collect();
    per.sort_by(|a, b| a.partial_cmp(b).unwrap());
    per[SAMPLES / 2]
}

fn main() {
    let filter = std::env::args().nth(1).unwrap_or_default();
    let dir = Path::new(env!("CARGO_MANIFEST_DIR")).join("../suite/bench");
    let mut files: Vec<_> = std::fs::read_dir(&dir)
        .unwrap_or_else(|e| panic!("{}: {e}", dir.display()))
        .map(|e| e.unwrap().path())
        .filter(|p| p.extension().is_some_and(|x| x == "json"))
        .collect();
    files.sort();
    for path in files {
        let text = std::fs::read_to_string(&path).unwrap();
        let f = risor::json::parse(&text).unwrap();
        let limits = f.get("limits").unwrap();
        let env = Env::builder()
            .vars(f.get("env").unwrap().as_str().unwrap())
            .limits(Limits {
                steps: Some(limits.get("steps").unwrap().as_f64().unwrap() as u64),
                allocation: Some(limits.get("allocation").unwrap().as_f64().unwrap() as u64),
                ..Limits::default()
            })
            .build()
            .unwrap_or_else(|e| panic!("{}: {e}", path.display()));
        let vars: BTreeMap<String, Data> = match data(f.get("vars").unwrap()) {
            Data::Map(m) => m,
            _ => panic!("vars is not an object"),
        };
        let stem = path.file_stem().unwrap().to_str().unwrap();
        for c in f.get("cases").unwrap().as_arr().unwrap() {
            let name = format!("{stem}/{}", c.get("name").unwrap().as_str().unwrap());
            if !name.contains(&filter) {
                continue;
            }
            let src = c.get("source").unwrap().as_str().unwrap();
            let expect = c.get("expect").unwrap();
            let build =
                || compile(&env, parse(src).unwrap()).unwrap_or_else(|e| panic!("{name}: {e}"));
            let p = build();
            let out = p.run(&vars);
            let value = out
                .result
                .unwrap_or_else(|e| panic!("{name}: {}", e.message()));
            assert!(
                eq_data(&value, &data(expect.get("value").unwrap())),
                "{name}: value differs from expect: {value:?}"
            );
            let (steps, allocated) = (out.usage.steps, out.usage.allocated);
            assert_eq!(
                (steps as f64, allocated as f64),
                (
                    expect.get("steps").unwrap().as_f64().unwrap(),
                    expect.get("allocated").unwrap().as_f64().unwrap()
                ),
                "{name}: usage differs from expect"
            );
            let compile_ns = median(|| {
                black_box(build());
            });
            let run_ns = median(|| {
                let out = p.run(&vars);
                black_box(out.result.is_ok());
            });
            println!(
                "{{\"impl\":\"rust\",\"program\":\"{name}\",\"compile_ns\":{compile_ns},\"run_ns\":{run_ns},\"steps\":{steps},\"allocated\":{allocated}}}"
            );
        }
    }
}
