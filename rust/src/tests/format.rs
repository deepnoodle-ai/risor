//! Package F12: the laws of the formatter (`f12-tools.md` 2.2) over every
//! program of the suite (7.6 item 3).

use crate::format::{canon, format};
use crate::json::Json;
use std::path::{Path, PathBuf};

fn sources(dir: &Path, out: &mut Vec<String>) {
    let Ok(rd) = std::fs::read_dir(dir) else {
        return;
    };
    let mut paths: Vec<PathBuf> = rd.flatten().map(|e| e.path()).collect();
    paths.sort();
    for p in paths {
        if p.is_dir() {
            sources(&p, out);
        } else if p.extension().is_some_and(|e| e == "json") {
            let Ok(text) = std::fs::read_to_string(&p) else {
                continue;
            };
            let Ok(doc) = crate::json::parse(&text) else {
                continue;
            };
            collect(&doc, out);
        }
    }
}

fn collect(j: &Json, out: &mut Vec<String>) {
    match j {
        Json::Obj(m) => {
            let is_template = m.iter().any(|(k, _)| k == "template");
            for (k, v) in m {
                match (k.as_str(), v) {
                    ("source", Json::Str(s)) | ("format", Json::Str(s)) if !is_template => {
                        out.push(s.clone())
                    }
                    _ => collect(v, out),
                }
            }
        }
        Json::Arr(a) => {
            for v in a {
                collect(v, out);
            }
        }
        _ => {}
    }
}

#[test]
fn the_laws_hold_over_the_suite() {
    let mut srcs = Vec::new();
    sources(
        &Path::new(env!("CARGO_MANIFEST_DIR")).join("../suite"),
        &mut srcs,
    );
    let mut checked = 0;
    let mut bad = Vec::new();
    for s in &srcs {
        let Ok(a) = crate::parse(s) else {
            continue;
        };
        // Deep nesting is left to the limits' own tests.
        if s.len() > 20_000 {
            continue;
        }
        let f = format(s).expect("a text that parses formats");
        checked += 1;
        match crate::parse(&f) {
            Err(e) => bad.push(format!("{s:?}: formatted {f:?} does not parse: {e:?}")),
            Ok(b) => {
                if canon(&a.ast, a.ast.root, a.source.chars())
                    != canon(&b.ast, b.ast.root, b.source.chars())
                {
                    bad.push(format!("{s:?}: the tree of {f:?} differs"));
                }
                match format(&f) {
                    Ok(g) if g == f => {}
                    Ok(g) => bad.push(format!("{s:?}: not idempotent: {f:?} then {g:?}")),
                    Err(e) => bad.push(format!("{s:?}: {e:?}")),
                }
            }
        }
    }
    for b in bad.iter().take(30) {
        println!("{b}");
    }
    assert!(checked > 3000, "{checked}");
    assert!(
        bad.is_empty(),
        "{} of {checked} programs break a law",
        bad.len()
    );
}

/// F16 7.1 with F12 2.12: no comment of an annotation is lost, and the
/// trailing comments of an annotated parameter's last token follow its
/// comma (item 5).
#[test]
fn comments_in_annotations() {
    for (src, want) in [
        (
            "const g = (f: (a: int /* a */, b: string) => bool, n) => n;\n0",
            "const g = (f: (a: int, /* a */ b: string) => bool, n) => n;\n0\n",
        ),
        (
            "const f = (x: int, // first\n y: string) => y;\nf(1, \"a\")",
            "const f = (\n  x: int, // first\n  y: string,\n) => y;\nf(1, \"a\")\n",
        ),
        (
            "const f = (x: int /* c */, y) => x;\nf(1, 2)",
            "const f = (x: int, /* c */ y) => x;\nf(1, 2)\n",
        ),
        (
            "const f = (x: /* t */ int) => x;\n1",
            "const f = (x: /* t */ int) => x;\n1\n",
        ),
    ] {
        assert_eq!(format(src).unwrap(), want, "{src:?}");
    }
}

/// F16: the trees the laws compare hold the annotations and the type lines.
#[test]
fn canon_holds_annotations() {
    let c = |s: &str| {
        let p = crate::parse(s).unwrap();
        canon(&p.ast, p.ast.root, p.source.chars())
    };
    assert_ne!(c("const x: int = 1; x"), c("const x = 1; x"));
    assert_ne!(c("const f = (x: int) => x; 1"), c("const f = (x) => x; 1"));
    assert_ne!(c("const f = (x): int => x; 1"), c("const f = (x) => x; 1"));
    assert_ne!(
        c("const g = (f: () => int) => 1; 1"),
        c("const g = (f: () => unknown) => 1; 1")
    );
    assert_ne!(
        c("type Aa = int; const b = 1; b"),
        c("const b = 1; type Aa = int; b")
    );
    assert_ne!(c("type Aa = int; 1"), c("1"));
}

/// Long chains that open no level are formatted without deep recursion.
#[test]
fn long_chains() {
    let n = 5_000;
    let sum = vec!["a"; n].join(" + ");
    let f = format(&sum).unwrap();
    assert!(f.starts_with("a +\n  a +\n"));
    let cond = format!("{}0", "a ? 1 : ".repeat(n));
    // A broken `? :` chain puts each link on a line (2.8 item 5).
    let want = format!("a ? 1\n{}: 0\n", ": a ? 1\n".repeat(n - 1));
    assert_eq!(format(&cond).unwrap(), want);
    let ifs = format!("{}{{ 0 }}", "if (a) { 1 } else ".repeat(n));
    format(&ifs).unwrap();
    let chain = format!("xs{}", ".f(1)".repeat(n));
    format(&chain).unwrap();
}

/// Comments after one token are attached in time linear in their number
/// (#675). Counting the line ends from the token for each comment took
/// time that grew with the square of the comments.
#[test]
fn many_comments_after_one_token() {
    let src = format!("1 {}", "/* c */ ".repeat(80_000));
    let start = std::time::Instant::now();
    let f = format(&src).unwrap();
    let took = start.elapsed();
    assert!(f.starts_with("1 /* c */ /* c */"));
    assert!(took < std::time::Duration::from_secs(3), "took {took:?}");
}
