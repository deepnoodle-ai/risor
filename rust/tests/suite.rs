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
use risor::testing::{
    case_files, load_doc, run_case, run_case_unchecked, run_file_with, Counts, Verdict,
};
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

fn undisputed(failed: &[String]) -> Vec<&String> {
    failed.iter().collect()
}

fn run(target: &Path, checked: bool) -> Counts {
    run_names(target, checked).0
}

fn run_names(target: &Path, checked: bool) -> (Counts, Vec<String>) {
    let files = if target.is_file() {
        vec![target.to_path_buf()]
    } else {
        case_files(target)
    };
    let mut total = Counts::default();
    let mut failed = Vec::new();
    let mut skips: BTreeMap<String, usize> = BTreeMap::new();
    for f in &files {
        let report = run_file_with(f, checked);
        print!("{report}");
        for (name, v) in &report.results {
            match v {
                Verdict::Skip(why) => *skips.entry(why.clone()).or_default() += 1,
                Verdict::Fail(_) => failed.push(name.clone()),
                _ => {}
            }
        }
        total.merge(report.counts());
    }
    print_skips(&skips);
    let mode = if checked {
        "checked"
    } else {
        "static rules alone"
    };
    println!(
        "{} ({} files, {mode}): {total}",
        target.display(),
        files.len()
    );
    (total, failed)
}

/// The headerless case lists of a data file's entries.
fn run_lists(label: &str, lists: Vec<(String, Json)>) -> Counts {
    // With the static rules alone, every case passes.
    let (plain, plain_failed) = run_lists_names(
        &format!("{label}, static rules alone"),
        lists.clone(),
        false,
    );
    assert!(plain_failed.is_empty(), "cases failed: {plain_failed:?}");
    assert_eq!(plain.invalid, 0);
    let (total, failed) = run_lists_names(&format!("{label}, checked"), lists, true);
    assert!(
        undisputed(&failed).is_empty(),
        "cases failed: {:?}",
        undisputed(&failed)
    );
    total
}

fn run_lists_names(
    label: &str,
    lists: Vec<(String, Json)>,
    checked: bool,
) -> (Counts, Vec<String>) {
    let mut failed_names = Vec::new();
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
                    let v = if checked {
                        run_case(c)
                    } else {
                        run_case_unchecked(c)
                    };
                    match &v {
                        Verdict::Fail(diffs) => {
                            failed_names.push(c.name.clone());
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
    (total, failed_names)
}

#[test]
fn conformance_suite() {
    let target = match std::env::var_os("RISOR_CASES") {
        Some(p) => PathBuf::from(p),
        None => suite_dir().join("core-run"),
    };
    let (total, failed) = run_names(&target, true);
    assert!(
        undisputed(&failed).is_empty(),
        "cases failed: {:?}",
        undisputed(&failed)
    );
    assert_eq!(total.invalid, 0, "files were invalid");
}

/// `suite/core-run/` with the static rules alone: a program compiled
/// without a check behaves as before (3.10.13).
#[test]
fn conformance_suite_unchecked() {
    let total = run(&suite_dir().join("core-run"), false);
    assert_eq!(total.fail, 0, "cases failed");
    assert_eq!(total.invalid, 0, "files were invalid");
}

#[test]
fn core_check() {
    let total = run(&suite_dir().join("core-check"), true);
    assert_eq!(total.invalid, 0, "files were invalid");
    assert_eq!(total.fail, 0, "cases failed");
}

#[test]
fn catalog_cases() {
    let lists = risor::messages::catalog().cases.clone();
    let total = run_lists("suite/messages.json", lists);
    assert_eq!(total.invalid, 0, "lists were invalid");
}

#[test]
fn library_cases() {
    let lists: Vec<(String, Json)> = risor::library::records()
        .iter()
        .filter_map(|r| r.cases.clone().map(|c| (r.name.clone(), c)))
        .collect();
    let total = run_lists("suite/library.json", lists);
    assert_eq!(total.invalid, 0, "lists were invalid");
}
