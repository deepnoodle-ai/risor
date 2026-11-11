//! The outcome command (`suite/README.md`), for the differential fuzzer:
//! one line of JSON for each case of a case file, its `expect` ignored.
//!
//! ```text
//! cargo run --quiet --release --bin outcome -- FILE
//! ```

use risor::testing::{load_file_lax, outcome_line};
use std::path::Path;
use std::process::ExitCode;

fn main() -> ExitCode {
    let Some(path) = std::env::args().nth(1) else {
        eprintln!("usage: outcome FILE");
        return ExitCode::from(2);
    };
    match load_file_lax(Path::new(&path)) {
        Err(why) => {
            eprintln!("invalid case file: {why}");
            ExitCode::from(2)
        }
        Ok(cases) => {
            use std::io::Write;
            let mut out = std::io::stdout().lock();
            for c in &cases {
                // A reader that stops early is not a failure of the command.
                if writeln!(out, "{}", outcome_line(c)).is_err() {
                    break;
                }
            }
            ExitCode::SUCCESS
        }
    }
}
