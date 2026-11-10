//! The generator of `tables.rs`, from the exact kernels of `math.rs`, and the
//! test that the tables compiled into the crate are the ones it gives.
//!
//! Print the file again with
//! `cargo test --lib math::gen::print_tables -- --ignored --nocapture --quiet`
//! and keep the text between the two marker lines.

use super::tables as t;
use super::{round, split, Cx, Fx, Int};
use std::fmt::Write;

/// The working precision of every entry but the bits of 2/π.
const P: u64 = 320;

/// n / 2^p as two doubles: the nearest double, then the nearest double to
/// what is left. The error is at most half the last place of the second, about
/// 2^-106 of the value, plus that of n itself (a few units of 2^-320).
fn dd(n: &Int, p: u64) -> [u64; 2] {
    let hi = round(n, -(p as i64));
    if hi == 0.0 {
        return [0, 0];
    }
    let (m, e) = split(hi);
    let rest = n.sub(&m.shl((e + p as i64) as u64));
    [hi.to_bits(), round(&rest, -(p as i64)).to_bits()]
}

/// n / 2^p as three doubles, each the nearest to what the ones before leave.
fn dd3(n: &Int, p: u64) -> [u64; 3] {
    let [a, _] = dd(n, p);
    let (m, e) = split(f64::from_bits(a));
    let rest = n.sub(&m.shl((e + p as i64) as u64));
    let [b, c] = dd(&rest, p);
    [a, b, c]
}

/// The reciprocal r_i of the table of the logarithm, for the mantissa
/// interval i of 512 in [1, 2): 1/mid, where mid is the middle of the
/// interval, halved from i = 256 on (`fast.rs`, log); 1 at the two ends; its
/// mantissa cut to 20 bits, so that m · r_i − 1 is small.
fn log_r(i: usize) -> f64 {
    if i == 0 || i == 511 {
        return 1.0;
    }
    let mut mid = 1.0 + (i as f64 + 0.5) / 512.0;
    if i >= 256 {
        mid *= 0.5;
    }
    f64::from_bits((1.0 / mid).to_bits() & !((1u64 << 33) - 1))
}

pub(super) struct Tables {
    pub exp_t1: Vec<[u64; 2]>,
    pub exp_t2: Vec<[u64; 2]>,
    pub ln2_4096: [u64; 3],
    pub ln2: [u64; 2],
    pub log_r: Vec<u64>,
    pub log_l: Vec<[u64; 2]>,
    pub inv_ln10: [u64; 2],
    pub sin128: Vec<[u64; 2]>,
    pub cos128: Vec<[u64; 2]>,
    pub atan128: Vec<[u64; 2]>,
    pub pio2: [u64; 2],
    pub pi: [u64; 2],
    pub third: [u64; 2],
    pub sixth: [u64; 2],
    pub two_over_pi: Vec<u64>,
}

pub(super) fn generate() -> Tables {
    let cx = Cx { p: P };
    let ln2 = cx.ln2();
    let pow2 = |num: i128, den: u64| {
        let z = cx.div_int(&cx.mul_int(&ln2, num), den);
        dd(&cx.exp_small(&z).n, P)
    };
    let pi = cx.pi();
    let one = cx.int(1);
    let at = |j: i64| Fx {
        n: Int::from_i128(j as i128).shl(P - 7),
        err: 0,
    };
    // 2/π to 1,280 bits: floor(2^1281 / π), from π at 1,400 bits.
    let q = 1400;
    let piq = Cx { p: q }.pi();
    let w = Int::from_i128(1).shl(1281 + q).div(&piq.n);
    let mut two_over_pi = vec![0u64];
    for k in 1..=20u64 {
        let word = w.shr(1280 - 64 * k);
        let lo = *word.mag.first().unwrap_or(&0) as u64;
        let hi = *word.mag.get(1).unwrap_or(&0) as u64;
        two_over_pi.push(lo | (hi << 32));
    }
    Tables {
        exp_t1: (0..64).map(|i| pow2(i, 64)).collect(),
        exp_t2: (0..64).map(|j| pow2(j, 4096)).collect(),
        ln2_4096: dd3(&ln2.n, P + 12),
        ln2: dd(&ln2.n, P),
        log_r: (0..512).map(|i| log_r(i).to_bits()).collect(),
        log_l: (0..512)
            .map(|i| {
                let r = log_r(i);
                if r == 1.0 {
                    [0, 0]
                } else {
                    dd(&cx.log(r).n.neg(), P)
                }
            })
            .collect(),
        inv_ln10: dd(&cx.div(&one, &cx.log(10.0)).n, P),
        sin128: (0..102).map(|j| dd(&cx.sin_cos(&at(j)).0.n, P)).collect(),
        cos128: (0..102).map(|j| dd(&cx.sin_cos(&at(j)).1.n, P)).collect(),
        atan128: (0..129).map(|j| dd(&cx.atan_unit(&at(j)).n, P)).collect(),
        pio2: dd(&pi.n, P + 1),
        pi: dd(&pi.n, P),
        third: dd(&cx.div_int(&one, 3).n, P),
        sixth: dd(&cx.div_int(&one, 6).n, P),
        two_over_pi,
    }
}

fn hex(v: u64) -> String {
    format!("{v:#018x}")
}

fn pair(out: &mut String, name: &str, doc: &str, v: &[[u64; 2]]) {
    writeln!(out, "\n/// {doc}").unwrap();
    writeln!(out, "pub(super) const {name}: [[u64; 2]; {}] = [", v.len()).unwrap();
    for [a, b] in v {
        writeln!(out, "    [{}, {}],", hex(*a), hex(*b)).unwrap();
    }
    writeln!(out, "];").unwrap();
}

fn words(out: &mut String, name: &str, doc: &str, v: &[u64]) {
    writeln!(out, "\n/// {doc}").unwrap();
    writeln!(out, "pub(super) const {name}: [u64; {}] = [", v.len()).unwrap();
    for a in v {
        writeln!(out, "    {},", hex(*a)).unwrap();
    }
    writeln!(out, "];").unwrap();
}

fn render(t: &Tables) -> String {
    let mut o = String::new();
    o.push_str(
        "//! The tables of the fast phase (`fast.rs`), as bit patterns of doubles.\n\
         //! A pair [hi, lo] is a double-double: hi is the double nearest to the\n\
         //! value and lo the double nearest to the rest, so hi + lo is within\n\
         //! about 2^-106 of the value.\n\
         //!\n\
         //! Generated by `gen.rs` from the exact kernels of `math.rs`; do not edit.\n\
         //! The test `tables_are_generated` there computes them again.\n",
    );
    pair(&mut o, "EXP_T1", "2^(i/64), i = 0..64.", &t.exp_t1);
    pair(&mut o, "EXP_T2", "2^(j/4096), j = 0..64.", &t.exp_t2);
    words(
        &mut o,
        "LN2_4096",
        "ln 2 / 4096 as three doubles.",
        &t.ln2_4096,
    );
    words(&mut o, "LN2", "ln 2.", &t.ln2);
    words(
        &mut o,
        "LOG_R",
        "The reciprocals r_i of the logarithm (`gen.rs`, log_r).",
        &t.log_r,
    );
    pair(&mut o, "LOG_L", "−log r_i.", &t.log_l);
    words(&mut o, "INV_LN10", "1 / ln 10.", &t.inv_ln10);
    pair(&mut o, "SIN128", "sin(j/128), j = 0..=101.", &t.sin128);
    pair(&mut o, "COS128", "cos(j/128), j = 0..=101.", &t.cos128);
    pair(&mut o, "ATAN128", "atan(j/128), j = 0..=128.", &t.atan128);
    words(&mut o, "PIO2", "π/2.", &t.pio2);
    words(&mut o, "PI", "π.", &t.pi);
    words(&mut o, "THIRD", "1/3.", &t.third);
    words(&mut o, "SIXTH", "1/6.", &t.sixth);
    words(
        &mut o,
        "TWO_OVER_PI",
        "The bits of 2/π, from the first after the point, 64 to a word, the\n\
         /// first word most significant; word 0 is the zero word before them.",
        &t.two_over_pi,
    );
    o
}

#[test]
#[ignore]
fn print_tables() {
    println!("---- BEGIN tables.rs ----");
    print!("{}", render(&generate()));
    println!("---- END tables.rs ----");
}

#[test]
fn tables_are_generated() {
    let g = generate();
    let pairs = |a: &[[u64; 2]], b: &[[u64; 2]]| a == b;
    assert!(pairs(&g.exp_t1, &t::EXP_T1));
    assert!(pairs(&g.exp_t2, &t::EXP_T2));
    assert_eq!(g.ln2_4096, t::LN2_4096);
    assert_eq!(g.ln2, t::LN2);
    assert_eq!(g.log_r, t::LOG_R);
    assert!(pairs(&g.log_l, &t::LOG_L));
    assert_eq!(g.inv_ln10, t::INV_LN10);
    assert!(pairs(&g.sin128, &t::SIN128));
    assert!(pairs(&g.cos128, &t::COS128));
    assert!(pairs(&g.atan128, &t::ATAN128));
    assert_eq!(g.pio2, t::PIO2);
    assert_eq!(g.pi, t::PI);
    assert_eq!(g.third, t::THIRD);
    assert_eq!(g.sixth, t::SIXTH);
    assert_eq!(g.two_over_pi, t::TWO_OVER_PI);
}
