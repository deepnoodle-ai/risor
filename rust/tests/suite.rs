//! The conformance suite. Runs every case file under `suite/core-run/`, or
//! the file or directory named by the environment variable `RISOR_CASES`,
//! and the cases of `suite/messages.json` and `suite/library.json`, and
//! prints the count of each verdict (`03-host.md` 3.10.8).
//!
//! ```text
//! cargo test --test suite -- --nocapture
//! RISOR_CASES=path/to/cases.json cargo test --test suite -- --nocapture
//! ```

use risor::json::Json;
use risor::testing::{case_files, load_doc, run_case, run_file, Counts, Verdict};
use std::collections::BTreeMap;
use std::path::{Path, PathBuf};

fn suite_dir() -> PathBuf {
    Path::new(env!("CARGO_MANIFEST_DIR")).join("../suite")
}

fn print_skips(skips: &BTreeMap<String, usize>) {
    for (why, n) in skips {
        println!("  skip x{n}: {why}");
    }
}

fn run(target: &Path) -> Counts {
    let files = if target.is_file() {
        vec![target.to_path_buf()]
    } else {
        case_files(target)
    };
    let mut total = Counts::default();
    let mut skips: BTreeMap<String, usize> = BTreeMap::new();
    for f in &files {
        let report = run_file(f);
        print!("{report}");
        for (_, v) in &report.results {
            if let Verdict::Skip(why) = v {
                *skips.entry(why.clone()).or_default() += 1;
            }
        }
        total.merge(report.counts());
    }
    print_skips(&skips);
    println!("{} ({} files): {total}", target.display(), files.len());
    total
}

/// The headerless case lists of a data file's entries.
fn run_lists(label: &str, lists: Vec<(String, Json)>) -> Counts {
    let mut total = Counts::default();
    let mut skips: BTreeMap<String, usize> = BTreeMap::new();
    for (entry, cases) in lists {
        match load_doc(&cases, &suite_dir()) {
            Err(why) => {
                println!("  invalid: {entry}: {why}");
                total.add(&Verdict::Invalid(why));
            }
            Ok(cases) => {
                for c in &cases {
                    let v = run_case(c);
                    match &v {
                        Verdict::Fail(diffs) => {
                            println!("  fail: {entry}: {}", c.name);
                            for d in diffs {
                                println!(
                                    "    {}: expected {}, found {}",
                                    d.name, d.expected, d.found
                                );
                            }
                        }
                        Verdict::Skip(why) => *skips.entry(why.clone()).or_default() += 1,
                        _ => {}
                    }
                    total.add(&v);
                }
            }
        }
    }
    print_skips(&skips);
    println!("{label}: {total}");
    total
}

#[test]
fn conformance_suite() {
    let target = match std::env::var_os("RISOR_CASES") {
        Some(p) => PathBuf::from(p),
        None => suite_dir().join("core-run"),
    };
    let total = run(&target);
    assert_eq!(total.fail, 0, "cases failed");
    assert_eq!(total.invalid, 0, "files were invalid");
}

#[test]
fn catalog_cases() {
    let lists = risor::messages::catalog().cases.clone();
    let total = run_lists("suite/messages.json", lists);
    assert_eq!(total.fail, 0, "cases failed");
    assert_eq!(total.invalid, 0, "lists were invalid");
}

#[test]
fn library_cases() {
    let lists: Vec<(String, Json)> = risor::library::records()
        .iter()
        .filter_map(|r| r.cases.clone().map(|c| (r.name.clone(), c)))
        .collect();
    let total = run_lists("suite/library.json", lists);
    assert_eq!(total.fail, 0, "cases failed");
    assert_eq!(total.invalid, 0, "lists were invalid");
}
