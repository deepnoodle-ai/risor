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

fn exact(r: i128) -> R {
    if r.abs() > MAX_WHOLE as i128 {
        Err(NumErr::Range)
    } else {
        Ok(r as f64)
    }
}

pub fn add(a: f64, b: f64) -> R {
    if is_whole(a) && is_whole(b) {
        exact(a as i128 + b as i128)
    } else {
        norm(a + b)
    }
}

pub fn sub(a: f64, b: f64) -> R {
    if is_whole(a) && is_whole(b) {
        exact(a as i128 - b as i128)
    } else {
        norm(a - b)
    }
}

pub fn mul(a: f64, b: f64) -> R {
    if is_whole(a) && is_whole(b) {
        exact(a as i128 * b as i128)
    } else {
        norm(a * b)
    }
}

pub fn div(a: f64, b: f64) -> R {
    if b == 0.0 {
        return Err(NumErr::DivZero);
    }
    norm(a / b)
}

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

/// Number to text: ECMAScript's `Number::toString` (`01-language.md` 6.3).
pub fn to_text(x: f64) -> String {
    if x == 0.0 {
        return "0".to_string();
    }
    if x < 0.0 {
        return format!("-{}", to_text(-x));
    }
    // Rust's `{:e}` gives the shortest digits that read back, the closest of
    // them.
    let e = format!("{x:e}");
    let (mant, exp) = e.split_once('e').unwrap_or((&e, "0"));
    let digits: String = mant.chars().filter(|c| *c != '.').collect();
    let digits = even_tie(x, digits, exp);
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
    // The exact decimal value of `x`: a double has at most 767 significant
    // digits, so this precision is exact.
    let exact = format!("{x:.800e}");
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
