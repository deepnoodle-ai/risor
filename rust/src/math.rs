//! Package F13: the eleven math functions of the group `math`, correctly
//! rounded (`spec/packages/f13-math.md` 2.1).
//!
//! No platform function decides a result. Each function is evaluated in
//! fixed point with an explicit bound on its error, at a precision of P
//! bits; the result is the double both ends of the interval round to. When
//! the two ends round apart, P doubles and the evaluation is made again (the
//! method of Ziv). The exact cases, where the true value is a double or the
//! midpoint of two, are found first: there no precision decides.
//!
//! That exact path costs microseconds. Each entry first tries the fast phase
//! of `fast.rs`: double-double arithmetic with a proven error bound, which
//! decides nearly every call in well under a microsecond and declines the
//! rest, which come here. The exact path is also the oracle of its tests.

use std::cmp::Ordering;

mod fast;
#[cfg(test)]
mod gen;
mod tables;

// ---- big integers ---------------------------------------------------------

/// A magnitude: little-endian base 2^32 digits, no high zero digit.
type Mag = Vec<u32>;

fn trim(a: &mut Mag) {
    while a.last() == Some(&0) {
        a.pop();
    }
}

fn mag_from(v: u128) -> Mag {
    let mut m = Vec::new();
    let mut v = v;
    while v > 0 {
        m.push(v as u32);
        v >>= 32;
    }
    m
}

fn mag_cmp(a: &Mag, b: &Mag) -> Ordering {
    if a.len() != b.len() {
        return a.len().cmp(&b.len());
    }
    for i in (0..a.len()).rev() {
        if a[i] != b[i] {
            return a[i].cmp(&b[i]);
        }
    }
    Ordering::Equal
}

fn mag_add(a: &Mag, b: &Mag) -> Mag {
    let (a, b) = if a.len() >= b.len() { (a, b) } else { (b, a) };
    let mut out = Vec::with_capacity(a.len() + 1);
    let mut carry = 0u64;
    for (i, &d) in a.iter().enumerate() {
        let s = d as u64 + *b.get(i).unwrap_or(&0) as u64 + carry;
        out.push(s as u32);
        carry = s >> 32;
    }
    if carry > 0 {
        out.push(carry as u32);
    }
    out
}

/// a − b, for a ≥ b.
fn mag_sub(a: &Mag, b: &Mag) -> Mag {
    let mut out = Vec::with_capacity(a.len());
    let mut borrow = 0i64;
    for (i, &x) in a.iter().enumerate() {
        let mut d = x as i64 - *b.get(i).unwrap_or(&0) as i64 - borrow;
        borrow = 0;
        if d < 0 {
            d += 1 << 32;
            borrow = 1;
        }
        out.push(d as u32);
    }
    trim(&mut out);
    out
}

fn mag_mul(a: &Mag, b: &Mag) -> Mag {
    if a.is_empty() || b.is_empty() {
        return Vec::new();
    }
    let mut out = vec![0u32; a.len() + b.len()];
    for (i, &x) in a.iter().enumerate() {
        let mut carry = 0u64;
        for (j, &y) in b.iter().enumerate() {
            let t = x as u64 * y as u64 + out[i + j] as u64 + carry;
            out[i + j] = t as u32;
            carry = t >> 32;
        }
        let mut k = i + b.len();
        while carry > 0 {
            let t = out[k] as u64 + carry;
            out[k] = t as u32;
            carry = t >> 32;
            k += 1;
        }
    }
    trim(&mut out);
    out
}

fn mag_shl(a: &Mag, s: u64) -> Mag {
    if a.is_empty() {
        return Vec::new();
    }
    let (w, b) = ((s / 32) as usize, (s % 32) as u32);
    let mut out = vec![0u32; w];
    if b == 0 {
        out.extend_from_slice(a);
    } else {
        let mut carry = 0u32;
        for &x in a {
            out.push((x << b) | carry);
            carry = x >> (32 - b);
        }
        if carry > 0 {
            out.push(carry);
        }
    }
    out
}

fn mag_shr(a: &Mag, s: u64) -> Mag {
    let w = (s / 32) as usize;
    if w >= a.len() {
        return Vec::new();
    }
    let b = (s % 32) as u32;
    let mut out: Mag = a[w..].to_vec();
    if b > 0 {
        for i in 0..out.len() {
            let hi = if i + 1 < out.len() {
                out[i + 1] << (32 - b)
            } else {
                0
            };
            out[i] = (out[i] >> b) | hi;
        }
    }
    trim(&mut out);
    out
}

fn mag_bits(a: &Mag) -> u64 {
    match a.last() {
        None => 0,
        Some(&top) => (a.len() as u64 - 1) * 32 + (32 - top.leading_zeros() as u64),
    }
}

/// Whether bit `i` of a is set.
fn mag_bit(a: &Mag, i: u64) -> bool {
    a.get((i / 32) as usize)
        .is_some_and(|w| (w >> (i % 32)) & 1 == 1)
}

/// The quotient and remainder of a by b (b not zero), by the division of
/// Knuth's algorithm D.
fn mag_divrem(a: &Mag, b: &Mag) -> (Mag, Mag) {
    if mag_cmp(a, b) == Ordering::Less {
        return (Vec::new(), a.clone());
    }
    if b.len() == 1 {
        let d = b[0] as u64;
        let mut q = vec![0u32; a.len()];
        let mut r = 0u64;
        for i in (0..a.len()).rev() {
            let cur = (r << 32) | a[i] as u64;
            q[i] = (cur / d) as u32;
            r = cur % d;
        }
        trim(&mut q);
        return (q, mag_from(r as u128));
    }
    let s = b.last().map_or(0, |t| t.leading_zeros()) as u64;
    let b = mag_shl(b, s);
    let mut a = mag_shl(a, s);
    a.push(0);
    let n = b.len();
    let m = a.len() - n;
    let mut q = vec![0u32; m];
    let (b1, b2) = (b[n - 1] as u64, b[n - 2] as u64);
    for j in (0..m).rev() {
        let num = ((a[j + n] as u64) << 32) | a[j + n - 1] as u64;
        let mut qh = num / b1;
        let mut rh = num % b1;
        while qh >= 1 << 32 || qh * b2 > ((rh << 32) | a[j + n - 2] as u64) {
            qh -= 1;
            rh += b1;
            if rh >= 1 << 32 {
                break;
            }
        }
        // Multiply and subtract.
        let mut borrow = 0i64;
        let mut carry = 0u64;
        for i in 0..n {
            let p = qh * b[i] as u64 + carry;
            carry = p >> 32;
            let t = a[i + j] as i64 - (p & 0xffff_ffff) as i64 - borrow;
            a[i + j] = t as u32;
            borrow = if t < 0 { 1 } else { 0 };
        }
        let t = a[j + n] as i64 - carry as i64 - borrow;
        a[j + n] = t as u32;
        if t < 0 {
            // Add back.
            qh -= 1;
            let mut c = 0u64;
            for i in 0..n {
                let t = a[i + j] as u64 + b[i] as u64 + c;
                a[i + j] = t as u32;
                c = t >> 32;
            }
            a[j + n] = (a[j + n] as u64 + c) as u32;
        }
        q[j] = qh as u32;
    }
    trim(&mut q);
    a.truncate(n);
    trim(&mut a);
    (q, mag_shr(&a, s))
}

fn mag_isqrt(a: &Mag) -> Mag {
    if a.is_empty() {
        return Vec::new();
    }
    let mut x = mag_shl(&vec![1], mag_bits(a).div_ceil(2));
    loop {
        let (q, _) = mag_divrem(a, &x);
        let y = mag_shr(&mag_add(&x, &q), 1);
        if mag_cmp(&y, &x) != Ordering::Less {
            return x;
        }
        x = y;
    }
}

/// A signed big integer.
#[derive(Clone, Debug, PartialEq)]
struct Int {
    neg: bool,
    mag: Mag,
}

impl Int {
    fn zero() -> Int {
        Int {
            neg: false,
            mag: Vec::new(),
        }
    }

    fn from_i128(v: i128) -> Int {
        Int {
            neg: v < 0,
            mag: mag_from(v.unsigned_abs()),
        }
        .norm()
    }

    fn norm(mut self) -> Int {
        trim(&mut self.mag);
        if self.mag.is_empty() {
            self.neg = false;
        }
        self
    }

    fn is_zero(&self) -> bool {
        self.mag.is_empty()
    }

    fn neg(&self) -> Int {
        Int {
            neg: !self.neg,
            mag: self.mag.clone(),
        }
        .norm()
    }

    fn abs(&self) -> Int {
        Int {
            neg: false,
            mag: self.mag.clone(),
        }
    }

    fn add(&self, o: &Int) -> Int {
        if self.neg == o.neg {
            return Int {
                neg: self.neg,
                mag: mag_add(&self.mag, &o.mag),
            }
            .norm();
        }
        match mag_cmp(&self.mag, &o.mag) {
            Ordering::Less => Int {
                neg: o.neg,
                mag: mag_sub(&o.mag, &self.mag),
            }
            .norm(),
            _ => Int {
                neg: self.neg,
                mag: mag_sub(&self.mag, &o.mag),
            }
            .norm(),
        }
    }

    fn sub(&self, o: &Int) -> Int {
        self.add(&o.neg())
    }

    fn mul(&self, o: &Int) -> Int {
        Int {
            neg: self.neg != o.neg,
            mag: mag_mul(&self.mag, &o.mag),
        }
        .norm()
    }

    fn shl(&self, s: u64) -> Int {
        Int {
            neg: self.neg,
            mag: mag_shl(&self.mag, s),
        }
        .norm()
    }

    /// Shift right, toward zero.
    fn shr(&self, s: u64) -> Int {
        Int {
            neg: self.neg,
            mag: mag_shr(&self.mag, s),
        }
        .norm()
    }

    /// Divide, toward zero.
    fn div(&self, o: &Int) -> Int {
        Int {
            neg: self.neg != o.neg,
            mag: mag_divrem(&self.mag, &o.mag).0,
        }
        .norm()
    }

    fn div_small(&self, d: u64) -> Int {
        self.div(&Int::from_i128(d as i128))
    }

    fn bits(&self) -> u64 {
        mag_bits(&self.mag)
    }

    /// The magnitude as a `u64`, saturated.
    fn sat_u64(&self) -> u64 {
        if self.bits() > 64 {
            return u64::MAX;
        }
        let lo = *self.mag.first().unwrap_or(&0) as u64;
        let hi = *self.mag.get(1).unwrap_or(&0) as u64;
        lo | (hi << 32)
    }

    /// The value as an `i128`, when it fits.
    fn to_i128(&self) -> Option<i128> {
        if self.bits() > 120 {
            return None;
        }
        let mut v: i128 = 0;
        for &d in self.mag.iter().rev() {
            v = (v << 32) | d as i128;
        }
        Some(if self.neg { -v } else { v })
    }
}

// ---- rounding to a double -------------------------------------------------

/// The double nearest to n · 2^e, ties to even, subnormals included; an
/// infinity when it rounds to 2^1024 or more. A zero is positive.
fn round(n: &Int, e: i64) -> f64 {
    if n.is_zero() {
        return 0.0;
    }
    let bl = n.bits() as i64;
    let top = bl - 1 + e;
    if top >= 1024 {
        return if n.neg {
            f64::NEG_INFINITY
        } else {
            f64::INFINITY
        };
    }
    // The exponent of the last place of the result.
    let q = (top - 52).max(-1074);
    let shift = q - e;
    let m: u64 = if shift <= 0 {
        mag_shl(&n.mag, (-shift) as u64)
            .iter()
            .rev()
            .fold(0u64, |acc, &d| (acc << 32) | d as u64)
    } else {
        let s = shift as u64;
        let kept = mag_shr(&n.mag, s);
        let mut m = kept
            .iter()
            .rev()
            .fold(0u64, |acc, &d| (acc << 32) | d as u64);
        // The bits shifted out against one half.
        let half_bit = mag_bit(&n.mag, s - 1);
        let below = (0..s - 1).any(|i| mag_bit(&n.mag, i));
        if half_bit && (below || m & 1 == 1) {
            m += 1;
        }
        m
    };
    if m == 0 {
        return 0.0;
    }
    let pow2 = |q: i64| -> f64 {
        if q >= -1022 {
            f64::from_bits(((q + 1023) as u64) << 52)
        } else {
            f64::from_bits(1u64 << (q + 1074))
        }
    };
    // m ≤ 2^53, so m and m · 2^q are exact; 2^1024 overflows to infinity.
    let v = if q > 1023 - 53 {
        (m as f64) * pow2(q - 60) * pow2(60)
    } else {
        (m as f64) * pow2(q)
    };
    if n.neg {
        -v
    } else {
        v
    }
}

/// A double as m · 2^e with m an integer.
fn split(x: f64) -> (Int, i64) {
    let b = x.to_bits();
    let neg = b >> 63 == 1;
    let ex = ((b >> 52) & 0x7ff) as i64;
    let frac = b & ((1 << 52) - 1);
    let (m, e) = if ex == 0 {
        (frac, -1074)
    } else {
        (frac | (1 << 52), ex - 1075)
    };
    let i = Int::from_i128(m as i128);
    (if neg { i.neg() } else { i }, e)
}

// ---- fixed point with an error bound ---------------------------------------

/// A real in fixed point: n / 2^p, with |true − n / 2^p| ≤ err / 2^p.
#[derive(Clone, Debug)]
struct Fx {
    n: Int,
    err: u64,
}

/// The precision of a computation.
#[derive(Clone, Copy)]
struct Cx {
    p: u64,
}

fn sat(a: u64, b: u64) -> u64 {
    a.saturating_add(b)
}

impl Cx {
    fn one(&self) -> Int {
        Int::from_i128(1).shl(self.p)
    }

    /// An exact integer.
    fn int(&self, v: i128) -> Fx {
        Fx {
            n: Int::from_i128(v).shl(self.p),
            err: 0,
        }
    }

    /// A double, exact when its last place is within the precision.
    fn double(&self, x: f64) -> Fx {
        let (m, e) = split(x);
        let s = e + self.p as i64;
        if s >= 0 {
            Fx {
                n: m.shl(s as u64),
                err: 0,
            }
        } else {
            Fx {
                n: m.shr((-s) as u64),
                err: 1,
            }
        }
    }

    /// A bound on |v| + 1, as an integer.
    fn mag(&self, a: &Fx) -> u64 {
        sat(a.n.shr(self.p).sat_u64(), 1)
    }

    fn add(&self, a: &Fx, b: &Fx) -> Fx {
        Fx {
            n: a.n.add(&b.n),
            err: sat(a.err, b.err),
        }
    }

    fn sub(&self, a: &Fx, b: &Fx) -> Fx {
        Fx {
            n: a.n.sub(&b.n),
            err: sat(a.err, b.err),
        }
    }

    fn mul(&self, a: &Fx, b: &Fx) -> Fx {
        let err = sat(
            sat(
                self.mag(a).saturating_mul(b.err),
                self.mag(b).saturating_mul(a.err),
            ),
            sat(
                (a.err as u128 * b.err as u128)
                    .checked_shr(self.p.min(127) as u32)
                    .unwrap_or(0)
                    .min(u64::MAX as u128) as u64,
                1,
            ),
        );
        Fx {
            n: a.n.mul(&b.n).shr(self.p),
            err,
        }
    }

    fn mul_int(&self, a: &Fx, k: i128) -> Fx {
        Fx {
            n: a.n.mul(&Int::from_i128(k)),
            err: a
                .err
                .saturating_mul(k.unsigned_abs().min(u64::MAX as u128) as u64),
        }
    }

    fn div_int(&self, a: &Fx, d: u64) -> Fx {
        Fx {
            n: a.n.div_small(d),
            err: sat(a.err / d, 1),
        }
    }

    /// a / b with its error as an integer: (q, err), in units of 2^-p.
    fn div_wide(&self, a: &Fx, b: &Fx) -> Option<(Int, Int)> {
        let q = a.n.shl(self.p).div(&b.n);
        let qm = q.abs().shr(self.p).add(&Int::from_i128(1));
        let num = Int::from_i128(a.err as i128)
            .add(&qm.mul(&Int::from_i128(b.err as i128)))
            .shl(self.p);
        let den = b.n.abs().sub(&Int::from_i128(b.err as i128));
        if den.neg || den.is_zero() {
            return None;
        }
        Some((q, num.div(&den).add(&Int::from_i128(2))))
    }

    /// a / b, for b whose interval does not hold zero.
    fn div(&self, a: &Fx, b: &Fx) -> Fx {
        let q = a.n.shl(self.p).div(&b.n);
        // |a/b − a'/b'| ≤ (ea + |q| eb) / (|b| − eb), in units.
        let qm = Int::from_i128(self.mag(&Fx {
            n: q.clone(),
            err: 0,
        }) as i128);
        let num = Int::from_i128(a.err as i128)
            .add(&qm.mul(&Int::from_i128(b.err as i128)))
            .shl(self.p);
        let den = b.n.abs().sub(&Int::from_i128(b.err as i128));
        let err = if den.neg || den.is_zero() {
            u64::MAX
        } else {
            sat(num.div(&den).sat_u64(), 2)
        };
        Fx { n: q, err }
    }

    /// √a, for a ≥ 0.
    fn sqrt(&self, a: &Fx) -> Fx {
        let n = if a.n.neg { Int::zero() } else { a.n.clone() };
        let r = Int {
            neg: false,
            mag: mag_isqrt(&n.shl(self.p).mag),
        };
        // |√a − √a'| ≤ e / (√a' + √(a' − e)) ≤ e / √a', in units, plus one.
        let err = if a.err == 0 {
            1
        } else if r.is_zero() {
            u64::MAX
        } else {
            sat(
                Int::from_i128(a.err as i128).shl(self.p).div(&r).sat_u64(),
                2,
            )
        };
        Fx { n: r, err }
    }

    // ---- constants ----

    /// atan(1/m), for m ≥ 2, by its series.
    fn atan_inv(&self, m: u64) -> Fx {
        let mut term = self.one().div_small(m);
        let m2 = m * m;
        let mut sum = term.clone();
        let mut k = 1u64;
        let mut ops = 1u64;
        while !term.is_zero() {
            term = term.div_small(m2);
            let t = term.div_small(2 * k + 1);
            sum = if k % 2 == 1 { sum.sub(&t) } else { sum.add(&t) };
            k += 1;
            ops += 2;
        }
        Fx { n: sum, err: ops }
    }

    /// π, by Machin's formula.
    fn pi(&self) -> Fx {
        let a = self.atan_inv(5);
        let b = self.atan_inv(239);
        self.sub(&self.mul_int(&a, 16), &self.mul_int(&b, 4))
    }

    /// atanh(1/m), for m ≥ 2.
    fn atanh_inv(&self, m: u64) -> Fx {
        let mut term = self.one().div_small(m);
        let m2 = m * m;
        let mut sum = term.clone();
        let mut k = 1u64;
        let mut ops = 1u64;
        while !term.is_zero() {
            term = term.div_small(m2);
            sum = sum.add(&term.div_small(2 * k + 1));
            k += 1;
            ops += 2;
        }
        Fx { n: sum, err: ops }
    }

    /// ln 2 = 2 atanh(1/3).
    fn ln2(&self) -> Fx {
        self.mul_int(&self.atanh_inv(3), 2)
    }

    // ---- series ----

    /// e^r, for |r| ≤ 1.
    fn exp_small(&self, r: &Fx) -> Fx {
        let mut term = Fx {
            n: self.one(),
            err: 0,
        };
        let mut sum = term.clone();
        let mut k = 1u64;
        loop {
            term = self.div_int(&self.mul(&term, r), k);
            if term.n.is_zero() {
                break;
            }
            sum = self.add(&sum, &term);
            k += 1;
        }
        Fx {
            n: sum.n,
            err: sat(sum.err, k + 2),
        }
    }

    /// sin r and cos r, for |r| ≤ 1.
    fn sin_cos(&self, r: &Fx) -> (Fx, Fx) {
        let r2 = self.mul(r, r);
        let mut s = r.clone();
        let mut term = r.clone();
        let mut k = 1u64;
        loop {
            term = self.div_int(&self.mul(&term, &r2), (2 * k) * (2 * k + 1));
            if term.n.is_zero() {
                break;
            }
            s = if k % 2 == 1 {
                self.sub(&s, &term)
            } else {
                self.add(&s, &term)
            };
            k += 1;
        }
        let mut c = Fx {
            n: self.one(),
            err: 0,
        };
        let mut term = c.clone();
        let mut j = 1u64;
        loop {
            term = self.div_int(&self.mul(&term, &r2), (2 * j - 1) * (2 * j));
            if term.n.is_zero() {
                break;
            }
            c = if j % 2 == 1 {
                self.sub(&c, &term)
            } else {
                self.add(&c, &term)
            };
            j += 1;
        }
        s.err = sat(s.err, k + 2);
        c.err = sat(c.err, j + 2);
        (s, c)
    }

    /// atan t, for 0 ≤ t ≤ 1: three halvings of the angle, then the series.
    fn atan_unit(&self, t: &Fx) -> Fx {
        let one = Fx {
            n: self.one(),
            err: 0,
        };
        let mut t = t.clone();
        for _ in 0..3 {
            // tan(θ/2) = t / (1 + √(1 + t²)).
            let d = self.add(&one, &self.sqrt(&self.add(&one, &self.mul(&t, &t))));
            t = self.div(&t, &d);
        }
        let t2 = self.mul(&t, &t);
        let mut pw = t.clone();
        let mut sum = t.clone();
        let mut k = 1u64;
        loop {
            pw = self.mul(&pw, &t2);
            let term = self.div_int(&pw, 2 * k + 1);
            if term.n.is_zero() {
                break;
            }
            sum = if k % 2 == 1 {
                self.sub(&sum, &term)
            } else {
                self.add(&sum, &term)
            };
            k += 1;
        }
        sum.err = sat(sum.err, k + 2);
        self.mul_int(&sum, 8)
    }

    /// The angle of (b, a), for a, b ≥ 0 not both zero: atan(a / b).
    fn angle(&self, a: &Fx, b: &Fx, pi: &Fx) -> Fx {
        if a.n.abs().sub(&b.n.abs()).neg || a.n == b.n {
            self.atan_unit(&self.div(a, b))
        } else {
            let half = self.div_int(pi, 2);
            self.sub(&half, &self.atan_unit(&self.div(b, a)))
        }
    }

    /// log m, for m in [1/2, 2], by 2 atanh((m − 1) / (m + 1)).
    fn log_near1(&self, m: &Fx) -> Fx {
        let one = Fx {
            n: self.one(),
            err: 0,
        };
        let t = self.div(&self.sub(m, &one), &self.add(m, &one));
        let t2 = self.mul(&t, &t);
        let mut pw = t.clone();
        let mut sum = t.clone();
        let mut k = 1u64;
        loop {
            pw = self.mul(&pw, &t2);
            let term = self.div_int(&pw, 2 * k + 1);
            if term.n.is_zero() {
                break;
            }
            sum = self.add(&sum, &term);
            k += 1;
        }
        sum.err = sat(sum.err, k + 2);
        self.mul_int(&sum, 2)
    }

    /// log x, for a double x > 0.
    fn log(&self, x: f64) -> Fx {
        let (m, e) = split(x);
        // x = m · 2^e; choose k with x / 2^k in [√½, √2).
        let mut k = m.bits() as i64 - 1 + e;
        // Compare x / 2^k with √2 by its square: m² · 2^(2(e − k)) ≥ 2.
        let sq = m.mul(&m);
        let ge = |k: i64| {
            let s = 2 * (e - k) - 1;
            if s >= 0 {
                !sq.shl(s as u64).sub(&Int::from_i128(1)).neg
            } else {
                !sq.sub(&Int::from_i128(1).shl((-s) as u64)).neg
            }
        };
        if ge(k) {
            k += 1;
        }
        let shift = e - k + self.p as i64;
        let mfx = if shift >= 0 {
            Fx {
                n: m.shl(shift as u64),
                err: 0,
            }
        } else {
            Fx {
                n: m.shr((-shift) as u64),
                err: 1,
            }
        };
        let lm = self.log_near1(&mfx);
        if k == 0 {
            lm
        } else {
            self.add(&self.mul_int(&self.ln2(), k as i128), &lm)
        }
    }

    /// e^z with the scale: (E, k) for e^z = E · 2^k, E near 1.
    fn exp_scaled(&self, z: &Fx) -> (Fx, i64) {
        let ln2 = self.ln2();
        // k, an integer near z / ln 2.
        let zi = z.n.shr(self.p.saturating_sub(16)).to_i128().unwrap_or(0) as f64 / 65536.0;
        let k = (zi / std::f64::consts::LN_2).round() as i64;
        let r = self.sub(z, &self.mul_int(&ln2, k as i128));
        (self.exp_small(&r), k)
    }
}

/// The double both ends of an interval round to, or `None`.
fn decide_at(a: &Fx, e: i64) -> Option<f64> {
    if a.err == u64::MAX {
        return None;
    }
    let lo = round(&a.n.sub(&Int::from_i128(a.err as i128)), e);
    let hi = round(&a.n.add(&Int::from_i128(a.err as i128)), e);
    (lo.to_bits() == hi.to_bits() || (lo == 0.0 && hi == 0.0)).then_some(lo)
}

/// The method of Ziv: evaluate at rising precision until the result is
/// decided. The last precision gives its nearest double.
fn ziv(f: impl Fn(Cx) -> (Fx, i64)) -> f64 {
    let mut p = 128;
    loop {
        let (v, e) = f(Cx { p });
        if let Some(r) = decide_at(&v, e) {
            return if r == 0.0 { 0.0 } else { r };
        }
        if p >= 1 << 14 {
            return round(&v.n, e);
        }
        p *= 2;
    }
}

/// |x| below 2^-27: the functions whose result is decided by x alone.
fn tiny(x: f64) -> bool {
    x.abs() < f64::from_bits(((1023 - 27) as u64) << 52)
}

// ---- the entries ------------------------------------------------------------

/// Whether the exact value of an entry is not a real number (2.1 item 4):
/// the value test, made before the cost is charged.
pub fn not_real(name: &str, args: &[f64]) -> bool {
    let x = args.first().copied().unwrap_or(0.0);
    match name {
        "math.pow" => {
            let y = args.get(1).copied().unwrap_or(0.0);
            y != 0.0 && ((x == 0.0 && y < 0.0) || (x < 0.0 && y.fract() != 0.0))
        }
        "math.log" | "math.log10" => x <= 0.0,
        "math.asin" | "math.acos" => !(-1.0..=1.0).contains(&x),
        _ => false,
    }
}

/// The correctly rounded value of an entry; an infinity when it is too
/// large for a number. The arguments pass the value test.
pub fn eval(name: &str, args: &[f64]) -> f64 {
    let x = args.first().copied().unwrap_or(0.0);
    let y = args.get(1).copied().unwrap_or(0.0);
    let r = match name {
        "math.exp" => fast::exp(x).unwrap_or_else(|| exp(x)),
        "math.log" => fast::log(x).unwrap_or_else(|| log(x)),
        "math.log10" => fast::log10(x).unwrap_or_else(|| log10(x)),
        "math.sin" => fast::trig(x, 0).unwrap_or_else(|| trig(x, 0)),
        "math.cos" => fast::trig(x, 1).unwrap_or_else(|| trig(x, 1)),
        "math.tan" => fast::trig(x, 2).unwrap_or_else(|| trig(x, 2)),
        "math.asin" => fast::asin(x).unwrap_or_else(|| asin(x)),
        "math.acos" => fast::acos(x).unwrap_or_else(|| acos(x)),
        "math.atan" => fast::atan2(x, 1.0).unwrap_or_else(|| atan2(x, 1.0)),
        "math.atan2" => fast::atan2(x, y).unwrap_or_else(|| atan2(x, y)),
        "math.pow" => fast::pow(x, y).unwrap_or_else(|| pow(x, y)),
        _ => f64::NAN,
    };
    if r == 0.0 {
        0.0
    } else {
        r
    }
}

fn exp(x: f64) -> f64 {
    if x.abs() < f64::from_bits(((1023 - 55) as u64) << 52) {
        return 1.0;
    }
    if x > 710.0 {
        return f64::INFINITY;
    }
    if x < -746.0 {
        return 0.0;
    }
    ziv(|cx| {
        let (e, k) = cx.exp_scaled(&cx.double(x));
        (e, k - cx.p as i64)
    })
}

fn log(x: f64) -> f64 {
    if x == 1.0 {
        return 0.0;
    }
    ziv(|cx| (cx.log(x), -(cx.p as i64)))
}

fn log10(x: f64) -> f64 {
    // 4.4 item 1: a power of ten that is a double gives its exponent.
    let mut p = 1.0f64;
    for k in 0..=22 {
        if x == p {
            return k as f64;
        }
        p *= 10.0;
    }
    ziv(|cx| {
        let ln10 = cx.log(10.0);
        (cx.div(&cx.log(x), &ln10), -(cx.p as i64))
    })
}

/// sin (0), cos (1) or tan (2) of x.
fn trig(x: f64, which: u8) -> f64 {
    if x == 0.0 {
        return if which == 1 { 1.0 } else { 0.0 };
    }
    if tiny(x) {
        return if which == 1 { 1.0 } else { x };
    }
    let ex = (x.abs().log2().ceil() as i64).max(0) as u64;
    ziv(|cx| {
        // Reduce at a precision that holds the multiple of π/2 exactly
        // enough: Q = P + the exponent of x.
        let q = Cx { p: cx.p + ex + 16 };
        let half = q.div_int(&q.pi(), 2);
        let xf = q.double(x);
        let kq = xf.n.shl(q.p).div(&half.n);
        let k = kq
            .add(
                &Int::from_i128(1)
                    .shl(q.p - 1)
                    .mul(&Int::from_i128(if kq.neg { -1 } else { 1 })),
            )
            .shr(q.p);
        // The error, in units of Q: |k| times that of π/2, and x's.
        let errq = Int::from_i128(half.err as i128)
            .mul(&k.abs())
            .add(&Int::from_i128(xf.err as i128 + 1));
        let s = q.p - cx.p;
        let r = Fx {
            n: xf.n.sub(&half.n.mul(&k)).shr(s),
            err: sat(errq.shr(s).sat_u64(), 2),
        };
        let (s, c) = cx.sin_cos(&r);
        let quad = (k.mag.first().copied().unwrap_or(0) as i64 & 3) as u8;
        let quad = if k.neg { (4 - quad) % 4 } else { quad };
        let neg = |a: &Fx| Fx {
            n: a.n.neg(),
            err: a.err,
        };
        let (sx, cx2) = match quad {
            0 => (s, c),
            1 => (c.clone(), neg(&s)),
            2 => (neg(&s), neg(&c)),
            _ => (neg(&c), s),
        };
        match which {
            0 => (sx, -(cx.p as i64)),
            1 => (cx2, -(cx.p as i64)),
            _ => match cx.div_wide(&sx, &cx2) {
                // A large quotient, near an odd multiple of π/2: drop the
                // low bits so that its error fits.
                Some((n, err)) => {
                    let g = err.bits().saturating_sub(40);
                    (
                        Fx {
                            n: n.shr(g),
                            err: sat(err.shr(g).sat_u64(), 1),
                        },
                        g as i64 - cx.p as i64,
                    )
                }
                None => (
                    Fx {
                        n: Int::zero(),
                        err: u64::MAX,
                    },
                    0,
                ),
            },
        }
    })
}

fn asin(x: f64) -> f64 {
    if x == 0.0 {
        return 0.0;
    }
    if tiny(x) {
        return x;
    }
    if x.abs() == 1.0 {
        return x.signum() * std::f64::consts::FRAC_PI_2;
    }
    ziv(|cx| {
        let pi = cx.pi();
        let xf = cx.double(x.abs());
        let one = cx.int(1);
        let c = cx.sqrt(&cx.sub(&one, &cx.mul(&xf, &xf)));
        let a = cx.angle(&xf, &c, &pi);
        let a = if x < 0.0 {
            Fx {
                n: a.n.neg(),
                err: a.err,
            }
        } else {
            a
        };
        (a, -(cx.p as i64))
    })
}

fn acos(x: f64) -> f64 {
    if x == 1.0 {
        return 0.0;
    }
    if x == -1.0 {
        return std::f64::consts::PI;
    }
    if x == 0.0 {
        return std::f64::consts::FRAC_PI_2;
    }
    ziv(|cx| {
        let pi = cx.pi();
        let xf = cx.double(x.abs());
        let one = cx.int(1);
        let s = cx.sqrt(&cx.sub(&one, &cx.mul(&xf, &xf)));
        // acos |x| = atan(√(1 − x²) / |x|); acos(−x) = π − acos x.
        let a = cx.angle(&s, &xf, &pi);
        let a = if x < 0.0 { cx.sub(&pi, &a) } else { a };
        (a, -(cx.p as i64))
    })
}

fn atan2(y: f64, x: f64) -> f64 {
    use std::f64::consts::{FRAC_PI_2, PI};
    if y == 0.0 {
        return if x < 0.0 { PI } else { 0.0 };
    }
    if x == 0.0 {
        return y.signum() * FRAC_PI_2;
    }
    let (ym, ye) = split(y.abs());
    let (xm, xe) = split(x.abs());
    let ey = ym.bits() as i64 + ye;
    let ex = xm.bits() as i64 + xe;
    // |y| / x below 2^-30 with x > 0: atan t = t (1 − t²/3 + …), from the
    // exact quotient, scaled.
    if x > 0.0 && ey - ex < -30 {
        let r = ziv(|cx| {
            // t = ym / xm · 2^(ye − xe); q = ym · 2^p / xm.
            let q = ym.shl(cx.p + 64).div(&xm);
            let scale = ye - xe - (cx.p as i64 + 64);
            // The factor 1 − t²/3 + t⁴/5, t² below 2^-58: two terms do.
            let t_exp = ey - ex;
            let t2 = Fx {
                n: Int::from_i128(1).shl((cx.p as i64 + 2 * t_exp).max(0) as u64),
                err: 0,
            };
            let f = cx.sub(&cx.int(1), &cx.div_int(&t2, 3));
            // The factor's error: the dropped terms are below t² itself.
            let fx = Fx {
                n: f.n,
                err: sat(t2.n.sat_u64(), 2),
            };
            let v = q.mul(&fx.n).shr(cx.p);
            // q is low by less than one unit; with the factor's bound.
            let err = sat(q.shr(cx.p).sat_u64().saturating_mul(fx.err), 2 + 2);
            (Fx { n: v, err }, scale)
        });
        return if y < 0.0 { -r } else { r };
    }
    // Scale both so that the larger is near 1: exact, since the smaller is
    // within 2^30 of it.
    let shift = ey.max(ex);
    let (ys, xs) = {
        let f = |v: f64, s: i64| {
            let mut v = v;
            let mut s = s;
            while s > 0 {
                let d = s.min(1000);
                v *= f64::from_bits(((1023 - d) as u64) << 52);
                s -= d;
            }
            while s < 0 {
                let d = (-s).min(1000);
                v *= f64::from_bits(((1023 + d) as u64) << 52);
                s += d;
            }
            v
        };
        (f(y.abs(), shift), f(x.abs(), shift))
    };
    let r = ziv(|cx| {
        let pi = cx.pi();
        let a = cx.angle(&cx.double(ys), &cx.double(xs), &pi);
        let a = if x < 0.0 { cx.sub(&pi, &a) } else { a };
        (a, -(cx.p as i64))
    });
    if y < 0.0 {
        -r
    } else {
        r
    }
}

fn pow(x: f64, y: f64) -> f64 {
    if y == 0.0 {
        return 1.0;
    }
    if x == 0.0 {
        return 0.0;
    }
    let negative = x < 0.0 && y.fract() == 0.0 && is_odd(y);
    let ax = x.abs();
    if let Some(v) = pow_exact(ax, y) {
        return if negative { -v } else { v };
    }
    // Range: |y ln x| estimated with a margin decides the far cases.
    let est = y * ax.ln();
    if est > 760.0 {
        return if negative {
            f64::NEG_INFINITY
        } else {
            f64::INFINITY
        };
    }
    if est < -760.0 {
        return 0.0;
    }
    if est.abs() < f64::from_bits(((1023 - 60) as u64) << 52) && ax != 1.0 {
        return if negative { -1.0 } else { 1.0 };
    }
    let r = ziv(|cx| {
        // z = y · log x, at P + 80 bits, its error carried as an integer:
        // |y| is below 2^63 here.
        let wide = Cx { p: cx.p + 80 };
        let l = wide.log(ax);
        let (ym, ye) = split(y);
        let prod = l.n.mul(&ym);
        let perr = Int::from_i128(l.err as i128 + 1).mul(&ym.abs());
        // prod · 2^ye in units of 2^-(p+80); to units of 2^-p.
        let sh = 80 - ye;
        let (n, err) = if sh >= 0 {
            (prod.shr(sh as u64), sat(perr.shr(sh as u64).sat_u64(), 2))
        } else {
            (prod.shl((-sh) as u64), perr.shl((-sh) as u64).sat_u64())
        };
        let z = Fx { n, err };
        let (e, k) = cx.exp_scaled(&z);
        (e, k - cx.p as i64)
    });
    if negative {
        -r
    } else {
        r
    }
}

fn is_odd(y: f64) -> bool {
    y.abs() < 9007199254740992.0 && (y.abs() as u64) % 2 == 1
}

/// x^y for x > 0 when the exact value is a dyadic number: a double, a
/// midpoint, or another value with few bits that rounds exactly.
fn pow_exact(x: f64, y: f64) -> Option<f64> {
    let (m, e) = split(x);
    // x = mo · 2^e2 with mo odd.
    let tz = (0..64).find(|&i| mag_bit(&m.mag, i)).unwrap_or(0);
    let mo = m.shr(tz);
    let e2 = e + tz as i64;
    let mo1 = mo.mag == vec![1];
    // y = n / 2^k.
    let (n, k) = {
        let mut k = 0;
        let mut v = y;
        while v.fract() != 0.0 && k < 60 {
            v *= 2.0;
            k += 1;
        }
        if v.fract() != 0.0 || v.abs() >= 9.2e18 {
            return None;
        }
        (v as i64, k as u32)
    };
    if mo1 {
        // x^y = 2^(e2 · y): exact when e2 · n is a multiple of 2^k.
        let t = e2 as i128 * n as i128;
        let d = 1i128 << k;
        if t % d != 0 {
            return None;
        }
        let t = t / d;
        if t >= 1024 {
            return Some(f64::INFINITY);
        }
        if t < -1100 {
            return Some(0.0);
        }
        return Some(round(&Int::from_i128(1), t as i64));
    }
    if n <= 0 {
        return None;
    }
    // mo must be a perfect 2^k-th power, and e2 a multiple of 2^k.
    let root = if k == 0 {
        mo.clone()
    } else {
        if k > 5 || e2 % (1i64 << k) != 0 {
            return None;
        }
        let mut r = mo.clone();
        for _ in 0..k {
            let s = Int {
                neg: false,
                mag: mag_isqrt(&r.mag),
            };
            if s.mul(&s) != r {
                return None;
            }
            r = s;
        }
        r
    };
    let base_e = e2 >> k;
    // root^n odd: a double or a midpoint only below 2^54.
    let mut acc = Int::from_i128(1);
    for _ in 0..n {
        acc = acc.mul(&root);
        if acc.bits() > 54 {
            return None;
        }
    }
    let t = base_e as i128 * n as i128;
    if t > 2000 {
        return Some(f64::INFINITY);
    }
    if t < -2200 {
        return Some(0.0);
    }
    Some(round(&acc, t as i64))
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn rounding() {
        assert_eq!(round(&Int::from_i128(1), -1075), 0.0);
        assert_eq!(round(&Int::from_i128(3), -1075), f64::from_bits(2));
        assert_eq!(round(&Int::from_i128(1), 1024), f64::INFINITY);
        assert_eq!(round(&Int::from_i128((1 << 53) + 1), 0), 9007199254740992.0);
        assert_eq!(round(&Int::from_i128((1 << 53) + 3), 0), 9007199254740996.0);
    }

    #[test]
    fn division() {
        let a = Int::from_i128(1).shl(300).sub(&Int::from_i128(12345));
        let b = Int::from_i128(98765432123456789)
            .shl(40)
            .add(&Int::from_i128(7));
        let q = a.div(&b);
        let r = a.sub(&q.mul(&b));
        assert!(!r.neg && r.sub(&b).neg);
    }

    #[test]
    #[allow(clippy::approx_constant)]
    fn the_examples_of_the_chapter() {
        assert_eq!(eval("math.pow", &[2.0, 0.5]), 1.4142135623730951);
        assert_eq!(eval("math.pow", &[1.1, 10.0]), 2.5937424601000023);
        assert_eq!(eval("math.pow", &[2.0, -1075.0]), 0.0);
        assert_eq!(eval("math.pow", &[-8.0, 3.0]), -512.0);
        assert_eq!(eval("math.pow", &[10.0, 400.0]), f64::INFINITY);
        assert_eq!(eval("math.exp", &[1.0]), 2.718281828459045);
        assert_eq!(eval("math.exp", &[710.0]), f64::INFINITY);
        assert_eq!(eval("math.exp", &[-746.0]), 0.0);
        assert_eq!(eval("math.log", &[10.0]), 2.302585092994046);
        assert_eq!(eval("math.log10", &[1000.0]), 3.0);
        assert_eq!(eval("math.log10", &[2.0]), 0.3010299956639812);
        assert_eq!(eval("math.sin", &[1.0]), 0.8414709848078965);
        assert_eq!(
            eval("math.sin", &[std::f64::consts::PI]),
            1.2246467991473532e-16
        );
        assert_eq!(eval("math.sin", &[1e22]), -0.8522008497671888);
        assert_eq!(eval("math.cos", &[1.0]), 0.5403023058681398);
        assert_eq!(eval("math.tan", &[1.0]), 1.5574077246549023);
        assert_eq!(eval("math.tan", &[1.5707963267948966]), 16331239353195370.0);
        assert_eq!(eval("math.asin", &[0.5]), 0.5235987755982989);
        assert_eq!(eval("math.acos", &[-1.0]), std::f64::consts::PI);
        assert_eq!(eval("math.atan", &[-0.5]), -0.4636476090008061);
        assert_eq!(eval("math.atan2", &[0.0, -1.0]), std::f64::consts::PI);
        assert_eq!(eval("math.atan2", &[-1.0, -1.0]), -2.356194490192345);
        assert_eq!(eval("math.exp", &[1.0]), std::f64::consts::E);
        assert!(not_real("math.pow", &[-8.0, 1.0 / 3.0]));
        assert!(not_real("math.log", &[0.0]));
    }
}
