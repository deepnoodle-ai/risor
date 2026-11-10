// What it costs Rust to know the weight (UTF-16 length) of a &str it only
// reads. The byte length is free. Build and run:
//   rustc -O weight.rs -o /tmp/weight && /tmp/weight
use std::hint::black_box;
use std::time::Instant;

// Weight: one per code point, two for a code point above U+FFFF. In UTF-8 that
// is the bytes that are not continuation bytes, plus the four-byte leads.
fn weight_bytes(s: &str) -> usize {
    let mut n = 0usize;
    for &b in s.as_bytes() {
        n += ((b as i8) >= -0x40) as usize; // not a continuation byte
        n += (b >= 0xf0) as usize; // lead of a four-byte sequence
    }
    n
}

// The standard library counts code points a word at a time. Add the
// four-byte leads, which the compiler turns into a vector count.
fn weight_std(s: &str) -> usize {
    s.chars().count() + s.as_bytes().iter().filter(|&&b| b >= 0xf0).count()
}

fn weight_ascii_fast(s: &str) -> usize {
    if s.is_ascii() {
        return s.len();
    }
    weight_std(s)
}

fn size_rule(s: &str) -> usize {
    // 04 A.5: a string under 64 bytes has size 0 without a scan.
    if s.len() < 64 {
        return 0;
    }
    weight_ascii_fast(s) / 64
}

fn make(bytes: usize, mixed: bool) -> String {
    let mut s = String::new();
    let mut i = 0;
    while s.len() < bytes {
        if mixed && i % 3 == 0 { s.push('é') } else { s.push('a') }
        i += 1;
    }
    s
}

fn bench(f: fn(&str) -> usize, s: &str) -> f64 {
    let mut iters: u64 = 1000;
    loop {
        let t0 = Instant::now();
        let mut sink = 0usize;
        for _ in 0..iters {
            sink = sink.wrapping_add(f(black_box(s)));
        }
        black_box(sink);
        let ns = t0.elapsed().as_nanos() as f64;
        if ns > 2e8 {
            return ns / iters as f64;
        }
        iters *= 4;
    }
}

fn main() {
    let inputs = [
        ("16 B ASCII", make(16, false)),
        ("256 B ASCII", make(256, false)),
        ("4,096 B ASCII", make(4096, false)),
        ("4,096 B, one third non-ASCII", make(4096, true)),
    ];
    let cases: [(&str, fn(&str) -> usize); 7] = [
        ("len (UTF-8 bytes)", |s| s.len()),
        ("chars().count()", |s| s.chars().count()),
        ("encode_utf16().count()", |s| s.encode_utf16().count()),
        ("weight, byte loop", weight_bytes),
        ("weight, chars + leads", weight_std),
        ("weight, ASCII test first", weight_ascii_fast),
        ("size by the rule of A.5", size_rule),
    ];
    for (name, f) in cases {
        let mut row = Vec::new();
        for (_, s) in &inputs {
            let mut runs: Vec<f64> = (0..5).map(|_| bench(f, s)).collect();
            runs.sort_by(|a, b| a.partial_cmp(b).unwrap());
            row.push(format!("{:.1} ns", runs[2]));
        }
        println!("{:<28} {}", name, row.join("  |  "));
    }
    let names: Vec<&str> = inputs.iter().map(|(n, _)| *n).collect();
    println!("columns: {}", names.join("  |  "));
}
