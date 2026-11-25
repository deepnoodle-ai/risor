//! Package F9: the helper a host runs its case files with (`f09-testing.md`
//! 7.1, 7.4).
//!
//! ```no_run
//! # fn main() {
//! let env = risor::Env::default();
//! let file = risor::testing::load("tests/orders.test.json", &env).expect("a valid file");
//! let report = file.run(&risor::NoVars);
//! report.assert_ok();
//! # }
//! ```

pub(crate) mod case;
pub(crate) mod helper;
pub(crate) mod program;
pub(crate) mod run;
pub(crate) mod tools;
pub(crate) mod update;

pub use case::{from_text, load, Case, CaseFile, FileError};
pub use helper::Report;
pub use run::{Difference, Verdict};

#[cfg(test)]
pub(crate) use case::{load_doc, load_doc_lax, load_file};
#[cfg(test)]
pub(crate) use run::{
    case_env, case_files, check_only_file, outcome_line, run_case, run_case_in, run_case_unchecked,
    run_file_with, skipped_by_package, skipped_files, Counts,
};
