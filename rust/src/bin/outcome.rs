//! The outcome command (rulings round 10 AJ), for the differential fuzzer:
//! one line of JSON for each case of a case file, its `expect` ignored.
//!
//! ```text
//! cargo run --quiet --release --bin outcome -- FILE
//! ```

use risor::json;
use risor::testing::{load_doc_lax, outcome_line};
use std::path::Path;
use std::process::ExitCode;

fn main() -> ExitCode {
    let Some(path) = std::env::args().nth(1) else {
        eprintln!("usage: outcome FILE");
        return ExitCode::from(2);
    };
    let path = Path::new(&path);
    let cases = std::fs::read_to_string(path)
        .map_err(|e| e.to_string())
        .and_then(|t| json::parse(&t))
        .and_then(|doc| load_doc_lax(&doc, path.parent().unwrap_or(Path::new("."))));
    match cases {
        Err(why) => {
            eprintln!("invalid case file: {why}");
            ExitCode::from(2)
        }
        Ok(cases) => {
            let lines: Vec<String> = cases.iter().map(outcome_line).collect();
            println!("{}", lines.join("\n"));
            ExitCode::SUCCESS
        }
    }
}
