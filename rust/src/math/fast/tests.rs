//! The fast phase against the exact path, which is the oracle: wherever the
//! fast phase gives a result, it must be the exact path's, bit for bit.
//!
//! `cargo test` runs a short version. The long version, at least 1,000,000
//! arguments for each function and the edge sets, is ignored by default:
//!
//! ```text
//! cargo test --release --lib math::fast::tests::long -- --ignored --nocapture
//! ```
//!
//! `RISOR_MATH_N` sets the number of random arguments for each function.

use super::super as m;

const NAMES: [&str; 11] = [
    "math.exp",
    "math.log",
    "math.log10",
    "math.sin",
    "math.cos",
    "math.tan",
    "math.asin",
    "math.acos",
    "math.atan",
    "math.atan2",
    "math.pow",
];

fn fast(name: &str, a: &[f64]) -> Option<f64> {
    let (x, y) = (a[0], a.get(1).copied().unwrap_or(0.0));
    let r = match name {
        "math.exp" => super::exp(x),
        "math.log" => super::log(x),
        "math.log10" => super::log10(x),
        "math.sin" => super::trig(x, 0),
        "math.cos" => super::trig(x, 1),
        "math.tan" => super::trig(x, 2),
        "math.asin" => super::asin(x),
        "math.acos" => super::acos(x),
        "math.atan" => super::atan2(x, 1.0),
        "math.atan2" => super::atan2(x, y),
        "math.pow" => super::pow(x, y),
        _ => unreachable!(),
    }?;
    Some(if r == 0.0 { 0.0 } else { r })
}

fn exact(name: &str, a: &[f64]) -> f64 {
    let (x, y) = (a[0], a.get(1).copied().unwrap_or(0.0));
    let r = match name {
        "math.exp" => m::exp(x),
        "math.log" => m::log(x),
        "math.log10" => m::log10(x),
        "math.sin" => m::trig(x, 0),
        "math.cos" => m::trig(x, 1),
        "math.tan" => m::trig(x, 2),
        "math.asin" => m::asin(x),
        "math.acos" => m::acos(x),
        "math.atan" => m::atan2(x, 1.0),
        "math.atan2" => m::atan2(x, y),
        "math.pow" => m::pow(x, y),
        _ => unreachable!(),
    };
    if r == 0.0 {
        0.0
    } else {
        r
    }
}

struct Rng(u64);

impl Rng {
    fn next(&mut self) -> u64 {
        self.0 ^= self.0 >> 12;
        self.0 ^= self.0 << 25;
        self.0 ^= self.0 >> 27;
        self.0.wrapping_mul(0x2545_f491_4f6c_dd1d)
    }

    fn below(&mut self, n: u64) -> u64 {
        self.next() % n
    }

    /// Uniform in [a, b).
    fn uniform(&mut self, a: f64, b: f64) -> f64 {
        let u = (self.next() >> 11) as f64 / (1u64 << 53) as f64;
        a + (b - a) * u
    }

    /// A positive double with a random mantissa and an exponent in [lo, hi].
    fn bits(&mut self, lo: i64, hi: i64) -> f64 {
        let e = lo + self.below((hi - lo + 1) as u64) as i64;
        let frac = self.next() >> 12;
        if e < -1022 {
            // Subnormal.
            f64::from_bits(frac >> (-1022 - e).min(52))
        } else {
            f64::from_bits(((e + 1023) as u64) << 52 | frac)
        }
    }

    fn sign(&mut self, v: f64) -> f64 {
        if self.next() & 1 == 0 {
            v
        } else {
            -v
        }
    }
}

/// One random argument list for an entry.
fn draw(name: &str, r: &mut Rng) -> Vec<f64> {
    let k = r.below(4);
    match name {
        "math.exp" => vec![match k {
            0 => r.uniform(-746.0, 710.0),
            1 => r.uniform(-20.0, 20.0),
            2 => r.uniform(-745.2, -700.0),
            _ => {
                let v = r.bits(-60, 9);
                r.sign(v)
            }
        }],
        "math.log" | "math.log10" => vec![match k {
            0 => r.bits(-1074, 1023),
            1 => r.uniform(0.5, 2.0),
            2 => {
                1.0 + {
                    let v = r.bits(-60, -10);
                    r.sign(v)
                }
            }
            _ => r.bits(-30, 30),
        }],
        "math.sin" | "math.cos" | "math.tan" => vec![{
            let v = match k {
                0 => r.bits(-30, 1023),
                1 => r.uniform(0.0, 10.0),
                2 => r.uniform(0.0, 1e6),
                _ => r.bits(-30, 60),
            };
            r.sign(v)
        }],
        "math.asin" | "math.acos" => vec![{
            let v = match k {
                0 => r.uniform(0.0, 1.0),
                1 => r.bits(-60, -1),
                2 => 1.0 - r.bits(-53, -5),
                _ => r.bits(-1074, -1),
            };
            r.sign(v.min(1.0))
        }],
        "math.atan" => vec![{
            let v = match k {
                0 => r.bits(-1074, 1023),
                1 => r.uniform(0.0, 10.0),
                _ => r.bits(-60, 60),
            };
            r.sign(v)
        }],
        "math.atan2" => {
            let (y, x) = match k {
                0 => (r.bits(-1074, 1023), r.bits(-1074, 1023)),
                1 => (r.uniform(0.0, 10.0), r.uniform(0.0, 10.0)),
                _ => {
                    let e = r.below(2000) as i64 - 1000;
                    let d = r.below(140) as i64 - 70;
                    {
                        let f = (e + d).clamp(-1074, 1023);
                        (r.bits(e, e), r.bits(f, f))
                    }
                }
            };
            vec![r.sign(y), r.sign(x)]
        }
        "math.pow" => {
            let (x, y) = match r.below(8) {
                0 => (r.bits(-20, 20), r.uniform(-50.0, 50.0)),
                1 => (r.bits(-1074, 1023), r.uniform(-2.0, 2.0)),
                2 => (r.bits(-30, 30), (r.below(141) as f64) - 70.0),
                3 => (r.bits(-30, 30), (r.below(129) as f64) / 2.0 - 32.0),
                4 => (
                    1.0 + {
                        let v = r.bits(-60, -5);
                        r.sign(v)
                    },
                    {
                        let v = r.bits(0, 60);
                        r.sign(v)
                    },
                ),
                5 => {
                    // Small integers and their roots: exact values.
                    let b = (r.below(1000) + 1) as f64;
                    let s = (r.below(41) as f64) - 20.0;
                    (b * b, s / 2.0)
                }
                6 => (r.uniform(0.0, 10.0), r.uniform(-10.0, 10.0)),
                _ => (r.bits(-1074, 1023), {
                    let v = r.bits(-60, 12);
                    r.sign(v)
                }),
            };
            let x = if r.below(4) == 0 && y.fract() == 0.0 {
                -x
            } else {
                x
            };
            vec![x, y]
        }
        _ => unreachable!(),
    }
}

/// The edge sets of each entry.
fn edges(name: &str) -> Vec<Vec<f64>> {
    let up = |v: f64, k: i64| f64::from_bits((v.to_bits() as i64 + k) as u64);
    let around = |v: f64, n: i64| -> Vec<f64> { (-n..=n).map(|k| up(v, k)).collect() };
    let one = |vs: Vec<f64>| vs.into_iter().map(|v| vec![v]).collect::<Vec<_>>();
    let mut out: Vec<Vec<f64>> = Vec::new();
    let tiny = [
        f64::from_bits(1),
        f64::from_bits(3),
        f64::MIN_POSITIVE,
        up(f64::MIN_POSITIVE, -1),
        f64::MAX,
    ];
    match name {
        "math.exp" => {
            let mut v = Vec::new();
            for c in [
                709.782712893384,
                709.7827128933841,
                -708.3964185322641,
                -708.39641853226,
                -745.1332191019411,
                -745.1332191019412,
                -744.4400719213812,
                -745.2,
                -746.0,
                710.0,
                1.0,
                -1.0,
                0.5,
                f64::from_bits(((1023 - 55) as u64) << 52),
                f64::from_bits(((1023 - 54) as u64) << 52),
                f64::from_bits(((1023 - 53) as u64) << 52),
                std::f64::consts::LN_2,
            ] {
                v.extend(around(c, 20));
                v.extend(around(-c, 20));
            }
            for k in -1100..=1030 {
                v.push(k as f64 * std::f64::consts::LN_2);
            }
            v.extend(tiny);
            out = one(v);
        }
        "math.log" | "math.log10" => {
            let mut v = Vec::new();
            v.extend(around(1.0, 2000));
            for j in 1..60 {
                v.push(1.0 + f64::from_bits(((1023 - j) as u64) << 52));
                v.push(1.0 - f64::from_bits(((1023 - j) as u64) << 52));
            }
            let mut p = 1.0f64;
            for _ in 0..=30 {
                v.extend(around(p, 3));
                v.extend(around(1.0 / p, 3));
                p *= 10.0;
            }
            for k in -1074..=1023 {
                v.push(super::scale_wide(1.0, k));
            }
            for i in 0..512u64 {
                // The ends of each interval of the table.
                v.extend(around(f64::from_bits((1023 << 52) | (i << 43)), 2));
            }
            v.extend(tiny);
            v.extend((1..2000).map(f64::from_bits));
            v.retain(|&x| x > 0.0);
            out = one(v);
        }
        "math.sin" | "math.cos" | "math.tan" => {
            let mut v = Vec::new();
            let pio2 = std::f64::consts::FRAC_PI_2;
            for k in 1..3000 {
                v.extend(around(k as f64 * pio2, 2));
            }
            for e in 0..1000 {
                // Near multiples of π/2 at every size: the nearest double to
                // a multiple, which the reduction meets with |r| small.
                let x = super::scale_wide(pio2, e);
                v.extend(around(x, 1));
            }
            // A double nearest a multiple of π/2 (|r| about 2^-61).
            v.extend(around(6381956970095103.0 * super::scale_wide(1.0, 797), 2));
            v.extend(around(0.785, 3));
            v.extend(around(std::f64::consts::FRAC_PI_4, 3));
            for e in -30..1024 {
                v.push(super::scale_wide(1.0, e));
            }
            v.extend([1e22, 1e300, f64::MAX, 0.0, 1.0, 2.0, 3.0]);
            v.extend(around(f64::from_bits(((1023 - 27) as u64) << 52), 3));
            let neg: Vec<f64> = v.iter().map(|x| -x).collect();
            v.extend(neg);
            out = one(v);
        }
        "math.asin" | "math.acos" => {
            let mut v = Vec::new();
            v.extend((0..3000).map(|k| up(1.0, -k)));
            v.extend(around(0.5, 50));
            v.extend(around(f64::from_bits(((1023 - 27) as u64) << 52), 3));
            v.extend(around(f64::from_bits(((1023 - 900) as u64) << 52), 3));
            v.extend([0.0, 1e-300, 5e-324, std::f64::consts::FRAC_1_SQRT_2]);
            for e in -1074..0 {
                v.push(super::scale_wide(1.0, e));
            }
            let neg: Vec<f64> = v.iter().map(|x| -x).collect();
            v.extend(neg);
            out = one(v);
        }
        "math.atan" => {
            let mut v = Vec::new();
            for j in 0..=128 {
                v.extend(around(j as f64 / 128.0 + 1.0 / 256.0, 2));
                v.extend(around(j as f64 / 128.0, 2));
            }
            for e in -1074..1024 {
                v.push(super::scale_wide(1.0, e));
                v.push(super::scale_wide(1.5, e));
            }
            v.extend(tiny);
            let neg: Vec<f64> = v.iter().map(|x| -x).collect();
            v.extend(neg);
            out = one(v);
        }
        "math.atan2" => {
            for e in -1100..1100 {
                for (y, x) in [(1.0, 1.0), (1.0, 3.0), (3.0, 1.0)] {
                    let ys = super::scale_wide(y, e.clamp(-1074, 1023));
                    for (sy, sx) in [(1.0, 1.0), (1.0, -1.0), (-1.0, 1.0), (-1.0, -1.0)] {
                        out.push(vec![sy * ys, sx * x]);
                        out.push(vec![sy * x, sx * ys]);
                    }
                }
            }
            for a in tiny {
                for b in tiny {
                    out.push(vec![a, b]);
                    out.push(vec![-a, -b]);
                    out.push(vec![a, -b]);
                }
            }
            for v in around(1.0, 50) {
                out.push(vec![v, 1.0]);
                out.push(vec![1.0, v]);
                out.push(vec![-v, -1.0]);
            }
        }
        "math.pow" => {
            let p2 = |e: i64| super::scale_wide(1.0, e);
            out.extend(
                [
                    [2.0, -1075.0],
                    [2.0, -1074.0],
                    [2.0, -1076.0],
                    [2.0, 1023.0],
                    [2.0, 1024.0],
                    [10.0, 22.0],
                    [10.0, 23.0],
                    [10.0, -1.0],
                    [10.0, 308.0],
                    [10.0, 309.0],
                    [1.5, 2.0],
                    [4.0, 0.5],
                    [-3.0, 3.0],
                    [-3.0, 2.0],
                    [-8.0, 3.0],
                    [2.0, 0.5],
                    [1.1, 10.0],
                    [9.0, 1.5],
                    [9.0, -1.5],
                    [0.25, 0.5],
                    [0.25, -0.5],
                    [p2(-1074), 0.5],
                    [p2(-1074), -1.0],
                    [p2(-1074), 1.0 / 1074.0],
                    [p2(-1072), 0.25],
                    [f64::MAX, 0.5],
                    [f64::MAX, -1.0],
                    [f64::MAX, 2.0],
                    [-1.0, 1e300],
                    [-1.0, 3.0],
                    [1.0, 1e300],
                    [up(1.0, 1), 1e17],
                    [up(1.0, -1), 1e17],
                    [up(1.0, 1), -1e300],
                    [up(1.0, 1), f64::MAX],
                    [0.5, 1074.0],
                    [0.5, 1075.0],
                    [3.0, 0.5],
                    [27.0, 1.0 / 3.0],
                    [(1u64 << 27) as f64 + 1.0, 2.0],
                    [(1u64 << 26) as f64 + 1.0, 2.0],
                    [((1u64 << 53) - 1) as f64, 2.0],
                    [3.0, 33.0],
                    [3.0, 34.0],
                    [3.0, 81.0],
                    [5.0, -3.0],
                    [p2(-537), 2.0],
                    [p2(-538), 2.0],
                    [1.5 * p2(-537), 2.0],
                ]
                .map(|a| a.to_vec()),
            );
            // Integer and half-integer powers of small integers and of
            // x · 2^k: many exact values, doubles and midpoints.
            for b in 1..=40 {
                for n in -70..=70 {
                    out.push(vec![b as f64, n as f64]);
                    out.push(vec![b as f64 * b as f64, n as f64 / 2.0]);
                    out.push(vec![-(b as f64), n as f64]);
                    out.push(vec![b as f64 * p2(-600), n as f64 / 4.0]);
                }
            }
            for e in -1074..=1023 {
                out.push(vec![p2(e), 0.5]);
                out.push(vec![p2(e), -0.5]);
                out.push(vec![p2(e), 1.5]);
                out.push(vec![p2(e), 3.0]);
                out.push(vec![p2(e), 1.0 / 3.0]);
            }
            // y log x at the limits of exp.
            for c in [709.782712893384, -708.3964185322641, -745.1332191019411] {
                for x in [2.0, 3.0, 10.0, 0.5, 1.1, 7.0] {
                    let y = c / super::log(x).unwrap_or(1.0);
                    for v in around(y, 5) {
                        out.push(vec![x, v]);
                    }
                }
            }
        }
        _ => unreachable!(),
    }
    out.retain(|a| a.iter().all(|v| v.is_finite()) && !m::not_real(name, a));
    out
}

#[derive(Default)]
struct Tally {
    calls: u64,
    fallbacks: u64,
}

/// Compare the fast phase with the exact path on each argument list.
fn compare(name: &str, args: impl Iterator<Item = Vec<f64>>, t: &mut Tally) {
    for a in args {
        if !a.iter().all(|v| v.is_finite()) || m::not_real(name, &a) {
            continue;
        }
        t.calls += 1;
        match fast(name, &a) {
            None => t.fallbacks += 1,
            Some(r) => {
                let e = exact(name, &a);
                assert_eq!(
                    r.to_bits(),
                    e.to_bits(),
                    "{name}{a:?}: fast {r:e}, exact {e:e}"
                );
            }
        }
    }
}

fn run(n: u64, with_edges: bool) {
    let results: Vec<(&str, Tally, Tally)> = std::thread::scope(|s| {
        let hs: Vec<_> = NAMES
            .iter()
            .enumerate()
            .map(|(i, &name)| {
                s.spawn(move || {
                    let mut rng = Rng(0x9e37_79b9_7f4a_7c15 ^ ((i as u64 + 1) * 0x1234_5678_9abc));
                    let mut rt = Tally::default();
                    compare(name, (0..n).map(|_| draw(name, &mut rng)), &mut rt);
                    let mut et = Tally::default();
                    if with_edges {
                        compare(name, edges(name).into_iter(), &mut et);
                    }
                    (name, rt, et)
                })
            })
            .collect();
        hs.into_iter().map(|h| h.join().unwrap()).collect()
    });
    for (name, rt, et) in results {
        println!(
            "{name:<11} random {:>8} calls, {:>6} to the exact path ({:.4}%); edges {:>6} calls, {:>5} to the exact path",
            rt.calls,
            rt.fallbacks,
            100.0 * rt.fallbacks as f64 / rt.calls.max(1) as f64,
            et.calls,
            et.fallbacks,
        );
    }
}

#[test]
fn short() {
    run(if cfg!(debug_assertions) { 300 } else { 3000 }, false);
}

/// A sample of each edge set, small enough for a debug build.
#[test]
fn short_edges() {
    for name in NAMES {
        let e = edges(name);
        let step = (e.len() / 200).max(1);
        compare(name, e.into_iter().step_by(step), &mut Tally::default());
    }
}

#[test]
#[ignore]
fn long() {
    let n = std::env::var("RISOR_MATH_N")
        .ok()
        .and_then(|v| v.parse().ok())
        .unwrap_or(1_000_000);
    run(n, true);
}

#[test]
fn exact_powers() {
    for (x, y, r) in [
        (2.0, -1075.0, 0.0),
        (2.0, -1074.0, f64::from_bits(1)),
        (10.0, 22.0, 1e22),
        (10.0, 23.0, 1e23),
        (1.5, 2.0, 2.25),
        (4.0, 0.5, 2.0),
        (-3.0, 3.0, -27.0),
        (9.0, 1.5, 27.0),
    ] {
        assert_eq!(m::eval("math.pow", &[x, y]), r, "pow({x}, {y})");
    }
}

/// Print arguments the fast phase declines, for the entry named by
/// `RISOR_MATH_NAME` (default `math.log`).
#[test]
#[ignore]
fn show_fallbacks() {
    let name = std::env::var("RISOR_MATH_NAME").unwrap_or("math.log".into());
    let mut rng = Rng(7);
    let mut shown = 0;
    for _ in 0..200000 {
        let a = draw(&name, &mut rng);
        if !a.iter().all(|v| v.is_finite()) || m::not_real(&name, &a) {
            continue;
        }
        if fast(&name, &a).is_none() && shown < 20 {
            println!("{a:?} -> {:e}", exact(&name, &a));
            shown += 1;
        }
    }
}
