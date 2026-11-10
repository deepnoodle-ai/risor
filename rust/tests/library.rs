//! The library entries: what the data files cannot hold (brief, S3 item 6).
//! Host numbers that cannot enter where an entry reads an element (round 9
//! AC16), and the work bound of the searches, the sort and the join on large
//! inputs (`02-evaluation.md` 8.5).

use risor::{compile_unchecked as compile, parse, Data, Env, Limits, Program};
use std::collections::BTreeMap;
use std::time::{Duration, Instant};

fn program(env: &Env, src: &str) -> Program {
    compile(env, parse(src).unwrap()).unwrap_or_else(|e| panic!("{src}: {e}"))
}

fn run(env: &Env, src: &str, vars: &BTreeMap<String, Data>) -> Result<String, String> {
    let p = program(env, src);
    let out = p.run(vars);
    match out.result {
        Ok(v) => Ok(format!("{v:?}")),
        Err(f) => {
            if f.code == "number-range" {
                assert_eq!(f.class(), Some("environment"), "{src}");
            }
            Err(f.code.to_string())
        }
    }
}

#[test]
fn entries_read_host_numbers_only_where_they_read_elements() {
    let env = Env::builder()
        .vars("{xs: any, es: any, s: any}")
        .build()
        .unwrap();
    let big = Data::Int(1 << 60);
    let mut v = BTreeMap::new();
    v.insert(
        "xs".into(),
        Data::List(vec![Data::Int(1), big.clone(), Data::Int(2)]),
    );
    let entry: BTreeMap<String, Data> = [
        ("key".to_string(), Data::Str("a".into())),
        ("value".to_string(), big),
    ]
    .into();
    v.insert("es".into(), Data::List(vec![Data::Map(entry)]));
    v.insert("s".into(), Data::Str("a,b".into()));
    for src in [
        "xs.sum()",
        "xs.sort()",
        "xs.min()",
        "xs.max()",
        "xs.unique()",
        "xs.map(x => 0)",
        "xs.filter(x => true)",
        "xs.reduce((a, x) => a, 0)",
        "xs.some(x => false)",
        "xs.sortBy(x => 1)",
        "string(xs)",
        "xs.slice(1)[0]",
    ] {
        assert_eq!(run(&env, src, &v), Err("number-range".into()), "{src}");
    }
    // These measure or copy and read no element.
    for (src, want) in [
        ("len(xs)", "3"),
        ("xs.first()", "1"),
        ("len(xs.slice(0, 2))", "2"),
        ("len(xs.reverse())", "3"),
        ("xs.find(x => true)", "1"),
        ("len(es.toMap())", "1"),
    ] {
        assert_eq!(run(&env, src, &v), Ok(want.into()), "{src}");
    }
    assert_eq!(run(&env, "es.toMap().a", &v), Err("number-range".into()));
}

fn unlimited() -> Env {
    Env::builder()
        .vars("{s: string, t: string, xs: any}")
        .limits(Limits {
            steps: None,
            allocation: None,
            ..Limits::default()
        })
        .build()
        .unwrap()
}

/// A run that must finish in time in proportion to its charge: the limit
/// here is generous for an unoptimized build.
fn timed(env: &Env, src: &str, v: &BTreeMap<String, Data>) -> u64 {
    let p = program(env, src);
    let start = Instant::now();
    let out = p.run(v);
    let took = start.elapsed();
    assert!(
        out.result.is_ok(),
        "{src}: {:?}",
        out.result.err().map(|f| f.code)
    );
    assert!(took < Duration::from_secs(5), "{src} took {took:?}");
    out.usage.steps
}

#[test]
fn searches_are_linear_in_the_text() {
    // A text and a needle that a naive search takes `w(s) × w(x)` on.
    let mut v = BTreeMap::new();
    v.insert("s".into(), Data::Str("a".repeat(2_000_000)));
    v.insert("t".into(), Data::Str(format!("{}b", "a".repeat(100_000))));
    v.insert("xs".into(), Data::Null);
    let env = unlimited();
    for src in [
        "t in s",
        "len(s.split(t))",
        "len(s.replaceAll(t, \"x\"))",
        "s.startsWith(t)",
        "s.endsWith(t)",
        "len(s.removePrefix(t))",
        // A needle longer than the text is answered from the lengths.
        "s in t",
        "len(t.split(s))",
    ] {
        let steps = timed(&env, src, &v);
        // At most the text's size three times over: the entry's formula,
        // and `len` of a result as large as the text.
        assert!(steps <= 3 + 3 * (2_000_000 / 64) + 2_000, "{src}: {steps}");
    }
}

#[test]
fn sort_and_join_are_bounded_by_their_charge() {
    let n = 200_000;
    let mut v = BTreeMap::new();
    let words: Vec<Data> = (0..n)
        .map(|i| Data::Str(format!("w{:07}", (i * 7919) % n)))
        .collect();
    v.insert("xs".into(), Data::List(words));
    v.insert("s".into(), Data::Str(String::new()));
    v.insert("t".into(), Data::Str(String::new()));
    let env = unlimited();
    let steps = timed(&env, "len(xs.sort())", &v);
    // n + L*(n + sz(elements)): L = 18, w = 8 each.
    assert_eq!(steps, 1 + n + 18 * (n + n * 8 / 64) + 1);
    let steps = timed(&env, "len(xs.join(\", \"))", &v);
    assert!(steps > n);
    timed(&env, "len(xs.unique())", &v);
    timed(&env, "len(xs.groupBy(x => x.slice(0, 3)))", &v);
    timed(&env, "len(xs.sortBy(x => x))", &v);
    timed(
        &env,
        "len(xs.map((x, i) => {key: x, value: i}).toMap())",
        &v,
    );
}
