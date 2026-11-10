//! The conformance suite. Runs every case file under `suite/core-run/`, or
//! the file or directory named by the environment variable `RISOR_CASES`,
//! and prints the count of each verdict (`03-host.md` 3.10.8).
//!
//! ```text
//! cargo test --test suite -- --nocapture
//! RISOR_CASES=path/to/cases.json cargo test --test suite -- --nocapture
//! ```

use risor::testing::{case_files, run_file, Counts, Verdict};
use std::collections::BTreeMap;
use std::path::{Path, PathBuf};

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
    for (why, n) in &skips {
        println!("  skip x{n}: {why}");
    }
    println!("{} ({} files): {total}", target.display(), files.len());
    total
}

#[test]
fn conformance_suite() {
    let target = match std::env::var_os("RISOR_CASES") {
        Some(p) => PathBuf::from(p),
        None => Path::new(env!("CARGO_MANIFEST_DIR")).join("../suite/core-run"),
    };
    let total = run(&target);
    assert_eq!(total.fail, 0, "cases failed");
    assert_eq!(total.invalid, 0, "files were invalid");
}
