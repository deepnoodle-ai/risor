//! Numbers (`01-language.md` 6.2, 6.3; `02-evaluation.md` 3).

use crate::value::{is_whole, MAX_WHOLE};

/// Why an operation on numbers failed.
#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub enum NumErr {
    Range,
    DivZero,
}

type R = Result<f64, NumErr>;

fn norm(x: f64) -> R {
    if !x.is_finite() {
        Err(NumErr::Range)
    } else if x == 0.0 {
        Ok(0.0)
    } else {
        Ok(x)
    }
}

fn exact(r: i64) -> R {
    if r.unsigned_abs() > MAX_WHOLE as u64 {
        Err(NumErr::Range)
    } else {
        Ok(r as f64)
    }
}

// `+`, `-` and `*` test whether both operands are whole only when the
// result leaves ±(2^53 − 1). Inside that range the double result is the
// exact result for whole operands: rounding is monotonic, so an exact
// result past 2^53 − 1 never rounds back into the range, and one inside it
// is representable. So the fast branch gives what the whole-number rule
// gives.

#[inline]
fn small(x: f64) -> Option<f64> {
    if x.abs() <= MAX_WHOLE {
        Some(if x == 0.0 { 0.0 } else { x })
    } else {
        None
    }
}

#[inline]
pub fn add(a: f64, b: f64) -> R {
    match small(a + b) {
        Some(x) => Ok(x),
        None => add_whole(a, b),
    }
}

#[cold]
fn add_whole(a: f64, b: f64) -> R {
    if is_whole(a) && is_whole(b) {
        exact(a as i64 + b as i64)
    } else {
        norm(a + b)
    }
}

#[inline]
pub fn sub(a: f64, b: f64) -> R {
    match small(a - b) {
        Some(x) => Ok(x),
        None => sub_whole(a, b),
    }
}

#[cold]
fn sub_whole(a: f64, b: f64) -> R {
    if is_whole(a) && is_whole(b) {
        exact(a as i64 - b as i64)
    } else {
        norm(a - b)
    }
}

#[inline]
pub fn mul(a: f64, b: f64) -> R {
    match small(a * b) {
        Some(x) => Ok(x),
        None => mul_whole(a, b),
    }
}

#[cold]
fn mul_whole(a: f64, b: f64) -> R {
    if is_whole(a) && is_whole(b) {
        // A product that overflows i64 is far past 2^53 − 1.
        match (a as i64).checked_mul(b as i64) {
            Some(r) => exact(r),
            None => Err(NumErr::Range),
        }
    } else {
        norm(a * b)
    }
}

#[inline]
pub fn div(a: f64, b: f64) -> R {
    if b == 0.0 {
        return Err(NumErr::DivZero);
    }
    norm(a / b)
}

#[inline]
pub fn rem(a: f64, b: f64) -> R {
    if b == 0.0 {
        return Err(NumErr::DivZero);
    }
    if is_whole(a) && is_whole(b) {
        return norm(((a as i64) % (b as i64)) as f64);
    }
    // IEEE remainder by truncation (fmod) is exact.
    norm(a % b)
}

pub fn neg(a: f64) -> f64 {
    if a == 0.0 {
        0.0
    } else {
        -a
    }
}

/// `a ** b`. The caller has tested that `b` is whole.
pub fn pow(a: f64, b: f64) -> R {
    if b >= 0.0 {
        let e = b as u64;
        if is_whole(a) {
            let mut r: i128 = 1;
            let a = a as i128;
            for bit in (0..64 - e.leading_zeros()).rev() {
                r = r * r;
                if r.abs() > MAX_WHOLE as i128 {
                    return Err(NumErr::Range);
                }
                if e >> bit & 1 == 1 {
                    r *= a;
                    if r.abs() > MAX_WHOLE as i128 {
                        return Err(NumErr::Range);
                    }
                }
            }
            return Ok(if r == 0 { 0.0 } else { r as f64 });
        }
        return norm(ieee_pow(a, e));
    }
    if a == 0.0 {
        return Err(NumErr::DivZero);
    }
    let p = ieee_pow(a, (-b) as u64);
    if p.is_infinite() {
        return Ok(0.0);
    }
    if p == 0.0 {
        return Err(NumErr::Range);
    }
    norm(1.0 / p)
}

fn ieee_pow(a: f64, e: u64) -> f64 {
    let mut r = 1.0f64;
    for bit in (0..64 - e.leading_zeros()).rev() {
        r *= r;
        if e >> bit & 1 == 1 {
            r *= a;
        }
    }
    r
}

/// The range where two shortest texts can tie: [2^30, 2^51).
const TIE_LOW: f64 = 1073741824.0;
const TIE_HIGH: f64 = 2251799813685248.0;

/// Append the text of a number (`to_text`) to `out`.
pub fn push_text(out: &mut String, x: f64) {
    if x.fract() == 0.0 && x.abs() <= MAX_WHOLE {
        // A whole number: its digits, written from the right.
        let mut buf = [0u8; 20];
        let mut i = buf.len();
        let mut n = (x as i64).unsigned_abs();
        loop {
            i -= 1;
            buf[i] = b'0' + (n % 10) as u8;
            n /= 10;
            if n == 0 {
                break;
            }
        }
        if x < 0.0 {
            out.push('-');
        }
        for &b in &buf[i..] {
            out.push(b as char);
        }
        return;
    }
    out.push_str(&to_text(x));
}

/// Number to text: ECMAScript's `Number::toString` (`01-language.md` 6.3).
pub fn to_text(x: f64) -> String {
    if x == 0.0 {
        return "0".to_string();
    }
    // A whole number below 2^53 in magnitude: its integer digits are the
    // shortest that read back (its neighbours are at most 1 away), and the
    // text of 6.3 is the integer written out (n ≤ 16 ≤ 21).
    if x.fract() == 0.0 && x.abs() <= MAX_WHOLE {
        return (x as i64).to_string();
    }
    if x < 0.0 {
        return format!("-{}", to_text(-x));
    }
    // Rust's `{:e}` gives the shortest digits that read back, the closest of
    // them.
    let e = format!("{x:e}");
    let (mant, exp) = e.split_once('e').unwrap_or((&e, "0"));
    let digits: String = mant.chars().filter(|c| *c != '.').collect();
    // Two shortest texts equally near exist only for doubles in
    // [2^30, 2^51) (`suite/core-run/number-text-ties.json`).
    let digits = if (TIE_LOW..TIE_HIGH).contains(&x) {
        even_tie(x, digits, exp)
    } else {
        digits
    };
    let k = digits.len() as i64;
    let n = exp.parse::<i64>().unwrap_or(0) + 1;
    if k <= n && n <= 21 {
        format!("{digits}{}", "0".repeat((n - k) as usize))
    } else if 0 < n && n <= 21 {
        format!("{}.{}", &digits[..n as usize], &digits[n as usize..])
    } else if -6 < n && n <= 0 {
        format!("0.{}{digits}", "0".repeat((-n) as usize))
    } else {
        let sign = if n > 0 { '+' } else { '-' };
        let rest = if k > 1 {
            format!(".{}", &digits[1..])
        } else {
            String::new()
        };
        format!("{}{rest}e{sign}{}", &digits[..1], (n - 1).abs())
    }
}

/// `01-language.md` 6.3 item 1 step 3: of two shortest digit strings
/// equally near `x`, the one whose last digit is even. Such a tie is when
/// the exact value of `x` has one digit more than the shortest, and it is
/// `5`; then the shortest is the truncation or one more than it.
fn even_tie(x: f64, digits: String, exp: &str) -> String {
    let k = digits.len();
    // The exact decimal value of `x`: below 2^51 a double has at most 16
    // integer digits and, at or above 2^30, at most 22 binary and so 22
    // decimal fraction digits, so 40 significant digits are exact.
    let exact = format!("{x:.40e}");
    let (em, ee) = exact.split_once('e').unwrap_or((&exact, "0"));
    if ee != exp {
        return digits;
    }
    let ed: String = em.chars().filter(|c| *c != '.').collect();
    let ed = ed.trim_end_matches('0');
    if ed.len() != k + 1 || !ed.ends_with('5') {
        return digits;
    }
    let low = ed[..k].to_string();
    // One more than `low`, at the same length.
    let mut up: Vec<u8> = low.bytes().collect();
    let mut i = up.len();
    loop {
        if i == 0 {
            return digits;
        }
        i -= 1;
        if up[i] == b'9' {
            up[i] = b'0';
        } else {
            up[i] += 1;
            break;
        }
    }
    let high = String::from_utf8(up).unwrap_or_default();
    let reads_back = |d: &str| {
        let t = format!("{}.{}e{}", &d[..1], &d[1..], exp);
        t.parse::<f64>().ok() == Some(x)
    };
    let even = |d: &str| {
        d.bytes()
            .last()
            .is_some_and(|b| (b - b'0').is_multiple_of(2))
    };
    for c in [&low, &high] {
        if even(c) && reads_back(c) {
            return c.trim_end_matches('0').to_string();
        }
    }
    digits
}

/// Text to number (`01-language.md` 6.3 rule 2): the JSON number grammar.
/// `None` when the text is not accepted; `Some(inf)` is never returned: text
/// that reads as an infinity gives `Err(())`.
pub fn from_text(t: &str) -> Option<Result<f64, ()>> {
    let b = t.as_bytes();
    let mut i = 0;
    if b.first() == Some(&b'-') {
        i += 1;
    }
    let digits = |i: &mut usize| {
        let s = *i;
        while *i < b.len() && b[*i].is_ascii_digit() {
            *i += 1;
        }
        *i - s
    };
    if b.get(i) == Some(&b'0') {
        i += 1;
    } else if digits(&mut i) == 0 {
        return None;
    }
    if b.get(i) == Some(&b'.') {
        i += 1;
        if digits(&mut i) == 0 {
            return None;
        }
    }
    if matches!(b.get(i), Some(b'e' | b'E')) {
        i += 1;
        if matches!(b.get(i), Some(b'+' | b'-')) {
            i += 1;
        }
        if digits(&mut i) == 0 {
            return None;
        }
    }
    if i != b.len() {
        return None;
    }
    let v: f64 = t.parse().ok()?;
    if !v.is_finite() {
        return Some(Err(()));
    }
    Some(Ok(if v == 0.0 { 0.0 } else { v }))
}

#[cfg(test)]
mod tests {
    use super::*;

    /// `+`, `-`, `*` by the rule as written: exact integer arithmetic when
    /// both operands are whole, else the double result.
    fn by_rule(op: char, a: f64, b: f64) -> R {
        if is_whole(a) && is_whole(b) {
            let (x, y) = (a as i64, b as i64);
            return match op {
                '+' => exact(x + y),
                '-' => exact(x - y),
                _ => x.checked_mul(y).map_or(Err(NumErr::Range), exact),
            };
        }
        norm(match op {
            '+' => a + b,
            '-' => a - b,
            _ => a * b,
        })
    }

    #[test]
    fn fast_arithmetic_is_the_rule() {
        let mut xs = vec![
            0.0,
            -0.0,
            1.0,
            -1.0,
            0.5,
            -2.5,
            3.0,
            MAX_WHOLE,
            -MAX_WHOLE,
            MAX_WHOLE - 1.0,
            9007199254740992.0,
            -9007199254740992.0,
            9007199254740994.0,
            4503599627370496.5,
            94906265.0,
            94906266.0,
            1e300,
            -1e300,
            f64::MAX,
            5e-324,
            0.1,
            0.2,
        ];
        let mut seed = 0x9E3779B97F4A7C15u64;
        for _ in 0..2000 {
            seed ^= seed << 13;
            seed ^= seed >> 7;
            seed ^= seed << 17;
            let f = match seed % 4 {
                0 => (seed >> 11) as f64,
                1 => ((seed >> 30) as f64) * if seed & 1 == 1 { -1.0 } else { 1.0 },
                2 => f64::from_bits(seed >> 2),
                _ => (seed >> 40) as f64 / 7.0,
            };
            if f.is_finite() {
                xs.push(f);
            }
        }
        for &a in &xs {
            for &b in xs.iter().take(60) {
                for (op, got) in [('+', add(a, b)), ('-', sub(a, b)), ('*', mul(a, b))] {
                    let want = by_rule(op, a, b);
                    assert_eq!(
                        got.map(f64::to_bits),
                        want.map(f64::to_bits),
                        "{a} {op} {b}"
                    );
                }
            }
        }
    }

    #[test]
    fn text() {
        for (x, t) in [
            (100.0, "100"),
            (123456789012345680000.0, "123456789012345680000"),
            (123456.789, "123456.789"),
            (4.35, "4.35"),
            (0.5, "0.5"),
            (0.000001, "0.000001"),
            (0.1 + 0.2, "0.30000000000000004"),
            (1e21, "1e+21"),
            (1.5e-7, "1.5e-7"),
            (5e-324, "5e-324"),
            (-2.5, "-2.5"),
            (1e-7, "1e-7"),
            // A tie between two shortest texts takes the even last digit.
            (9007199254740991.0 / 6.0, "1501199875790165.2"),
        ] {
            assert_eq!(to_text(x), t);
        }
    }

    /// The text by the full search, with the tie test at the exact value
    /// for every double: the oracle of the fast paths of `to_text`.
    fn to_text_full(x: f64) -> String {
        if x == 0.0 {
            return "0".to_string();
        }
        if x < 0.0 {
            return format!("-{}", to_text_full(-x));
        }
        let e = format!("{x:e}");
        let (mant, exp) = e.split_once('e').unwrap_or((&e, "0"));
        let digits: String = mant.chars().filter(|c| *c != '.').collect();
        let k0 = digits.len();
        let exact = format!("{x:.800e}");
        let (em, ee) = exact.split_once('e').unwrap_or((&exact, "0"));
        let ed: String = em.chars().filter(|c| *c != '.').collect();
        let ed = ed.trim_end_matches('0');
        let tie = ee == exp && ed.len() == k0 + 1 && ed.ends_with('5');
        if tie {
            assert!(
                (TIE_LOW..TIE_HIGH).contains(&x),
                "a tie outside the range: {x:e}"
            );
        }
        let digits = if tie {
            even_tie(x, digits, exp)
        } else {
            digits
        };
        let k = digits.len() as i64;
        let n = exp.parse::<i64>().unwrap_or(0) + 1;
        if k <= n && n <= 21 {
            format!("{digits}{}", "0".repeat((n - k) as usize))
        } else if 0 < n && n <= 21 {
            format!("{}.{}", &digits[..n as usize], &digits[n as usize..])
        } else if -6 < n && n <= 0 {
            format!("0.{}{digits}", "0".repeat((-n) as usize))
        } else {
            let sign = if n > 0 { '+' } else { '-' };
            let rest = if k > 1 {
                format!(".{}", &digits[1..])
            } else {
                String::new()
            };
            format!("{}{rest}e{sign}{}", &digits[..1], (n - 1).abs())
        }
    }

    #[test]
    fn fast_paths_agree_with_the_full_search() {
        // A fixed generator: any double by its bits, whole numbers, and
        // doubles of few fraction bits near the tie range.
        let mut s: u64 = 0x9E37_79B9_7F4A_7C15;
        let mut next = || {
            s ^= s << 13;
            s ^= s >> 7;
            s ^= s << 17;
            s
        };
        for i in 0..200_000u32 {
            let r = next();
            let x = match i % 4 {
                0 => f64::from_bits(r),
                1 => (r >> (11 + r % 40)) as f64,
                2 => (r >> 12) as f64 / (1u64 << (r % 30)) as f64,
                _ => (r >> 11) as f64 / (1u64 << (r % 64)) as f64,
            };
            if !x.is_finite() {
                continue;
            }
            for x in [x, -x] {
                let full = to_text_full(x);
                assert_eq!(to_text(x), full, "{x:e}");
                let mut pushed = String::from("<");
                push_text(&mut pushed, x);
                assert_eq!(pushed, format!("<{full}"), "{x:e}");
            }
        }
    }

    #[test]
    fn arithmetic() {
        assert_eq!(add(MAX_WHOLE, 1.0), Err(NumErr::Range));
        assert_eq!(mul(1e308, 10.0), Err(NumErr::Range));
        assert_eq!(rem(-7.0, 3.0), Ok(-1.0));
        assert_eq!(rem(5.5, 2.0), Ok(1.5));
        assert_eq!(rem(-1.0, 1.0), Ok(0.0));
        assert_eq!(pow(2.0, 10.0), Ok(1024.0));
        assert_eq!(pow(2.0, -1.0), Ok(0.5));
        assert_eq!(pow(2.0, -2000.0), Ok(0.0));
        assert_eq!(pow(0.1, -400.0), Err(NumErr::Range));
        assert_eq!(pow(0.0, -1.0), Err(NumErr::DivZero));
        assert_eq!(pow(0.5, -1024.0), Err(NumErr::Range));
        assert_eq!(pow(3.0, 0.0), Ok(1.0));
        assert_eq!(div(7.0, 2.0), Ok(3.5));
    }
}
