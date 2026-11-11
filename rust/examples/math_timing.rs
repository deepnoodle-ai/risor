//! The time of one call of each F13 math entry, for typical arguments:
//! `cargo run --release --example math_timing`.
//!
//! For each entry it draws 1,024 arguments in a typical range, then measures
//! batches of calls over them. It prints the median time of one call, in
//! nanoseconds, of 15 batches that each take at least 10 ms.

use risor::__private::math_eval;
use std::hint::black_box;
use std::time::{Duration, Instant};

struct Rng(u64);

impl Rng {
    fn next(&mut self) -> u64 {
        // xorshift64*
        self.0 ^= self.0 >> 12;
        self.0 ^= self.0 << 25;
        self.0 ^= self.0 >> 27;
        self.0.wrapping_mul(0x2545_f491_4f6c_dd1d)
    }

    /// Uniform in [a, b).
    fn uniform(&mut self, a: f64, b: f64) -> f64 {
        let u = (self.next() >> 11) as f64 / (1u64 << 53) as f64;
        a + (b - a) * u
    }

    /// Log-uniform in [a, b), for 0 < a < b.
    fn log_uniform(&mut self, a: f64, b: f64) -> f64 {
        // 2^t by exact scaling of a uniform mantissa: no platform function.
        let (la, lb) = (a.log2(), b.log2());
        let t = self.uniform(la, lb);
        let k = t.floor();
        let m = 1.0 + (t - k) * (1.0 - (t - k) * 0.3);
        m * 2f64.powi(k as i32)
    }
}

fn median(args: &[Vec<f64>], name: &str) -> f64 {
    const MIN_SAMPLE: Duration = Duration::from_millis(10);
    const SAMPLES: usize = 15;
    let run = |n: usize| {
        let t = Instant::now();
        for i in 0..n {
            black_box(math_eval(name, black_box(&args[i % args.len()])));
        }
        t.elapsed()
    };
    let start = Instant::now();
    while start.elapsed() < Duration::from_millis(100) {
        run(args.len());
    }
    let mut n = args.len();
    while run(n) < MIN_SAMPLE {
        n *= 2;
    }
    let mut per: Vec<f64> = (0..SAMPLES)
        .map(|_| run(n).as_nanos() as f64 / n as f64)
        .collect();
    per.sort_by(|a, b| a.partial_cmp(b).unwrap());
    per[SAMPLES / 2]
}

fn main() {
    let mut rng = Rng(0x9e37_79b9_7f4a_7c15);
    let n = 1024;
    let mut one = |f: &mut dyn FnMut(&mut Rng) -> Vec<f64>| -> Vec<Vec<f64>> {
        (0..n).map(|_| f(&mut rng)).collect()
    };
    let cases: Vec<(&str, Vec<Vec<f64>>)> = vec![
        ("math.exp", one(&mut |r| vec![r.uniform(-20.0, 20.0)])),
        ("math.log", one(&mut |r| vec![r.log_uniform(1e-5, 1e5)])),
        ("math.log10", one(&mut |r| vec![r.log_uniform(1e-5, 1e5)])),
        ("math.sin", one(&mut |r| vec![r.uniform(-10.0, 10.0)])),
        ("math.cos", one(&mut |r| vec![r.uniform(-10.0, 10.0)])),
        ("math.tan", one(&mut |r| vec![r.uniform(-10.0, 10.0)])),
        ("math.asin", one(&mut |r| vec![r.uniform(-1.0, 1.0)])),
        ("math.acos", one(&mut |r| vec![r.uniform(-1.0, 1.0)])),
        ("math.atan", one(&mut |r| vec![r.uniform(-10.0, 10.0)])),
        (
            "math.atan2",
            one(&mut |r| vec![r.uniform(-10.0, 10.0), r.uniform(-10.0, 10.0)]),
        ),
        (
            "math.pow",
            one(&mut |r| vec![r.log_uniform(0.1, 10.0), r.uniform(-10.0, 10.0)]),
        ),
    ];
    for (name, args) in &cases {
        println!("{name:<12} {:>10.1} ns", median(args, name));
    }
}
