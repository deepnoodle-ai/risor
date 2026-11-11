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

/// Long chains that open no level are formatted without deep recursion.
#[test]
fn long_chains() {
    let n = 5_000;
    let sum = vec!["a"; n].join(" + ");
    let f = format(&sum).unwrap();
    assert!(f.starts_with("a +\n  a +\n"));
    let cond = format!("{}0", "a ? 1 : ".repeat(n));
    format(&cond).unwrap();
    let ifs = format!("{}{{ 0 }}", "if (a) { 1 } else ".repeat(n));
    format(&ifs).unwrap();
    let chain = format!("xs{}", ".f(1)".repeat(n));
    format(&chain).unwrap();
}
