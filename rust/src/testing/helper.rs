//! Package F9: the helper a host runs its case files with, inside its own
//! tests (`f09-testing.md` 7.1, 7.4).
//!
//! ```no_run
//! # fn main() {
//! let env = risor::Env::default();
//! let file = risor::testing::load("tests/orders.test.json", &env).expect("a valid file");
//! let report = file.run(&risor::NoVars);
//! report.assert_ok();
//! # }
//! ```

use super::case::{Case, CaseFile};
use super::run::{report_text, run_case_ctx, Ctx, Verdict};
use crate::host::Vars;

/// The verdicts of a file's cases, the report, and the text update mode
/// wrote.
#[derive(Clone, Debug)]
pub struct Report {
    names: Vec<String>,
    verdicts: Vec<Verdict>,
    updated: Option<String>,
}

impl Report {
    /// The verdict of each case, in the file's order.
    pub fn verdicts(&self) -> &[Verdict] {
        &self.verdicts
    }

    /// The names of the cases, in the file's order.
    pub fn names(&self) -> &[String] {
        &self.names
    }

    /// The lines of `f09-testing.md` 6.1 for the cases.
    pub fn text(&self) -> String {
        let rs: Vec<(String, Verdict)> = self
            .names
            .iter()
            .cloned()
            .zip(self.verdicts.iter().cloned())
            .collect();
        report_text(&rs)
    }

    /// The new text of the file, when update mode wrote a case.
    pub fn updated_text(&self) -> Option<String> {
        self.updated.clone()
    }

    /// Panics with `text()` unless every case passed, was skipped or was
    /// written by update mode.
    pub fn assert_ok(&self) {
        let ok = self
            .verdicts
            .iter()
            .all(|v| matches!(v, Verdict::Pass | Verdict::Skip(_) | Verdict::Updated));
        assert!(ok, "{}", self.text());
    }
}

impl CaseFile {
    /// The cases of the file.
    pub fn cases(&self) -> &[Case] {
        &self.cases
    }

    /// Only the cases whose names contain `text`, code point for code point
    /// (`f09-testing.md` 7.1).
    pub fn filter(mut self, text: &str) -> CaseFile {
        self.cases.retain(|c| c.name.contains(text));
        self
    }

    /// Run every case in a child of the base the file was loaded with, with `vars` for the names a case
    /// does not supply. With `RISOR_UPDATE=1`, the file is run in update
    /// mode, and a file loaded from a path is written in place.
    pub fn run(&self, vars: &dyn Vars) -> Report {
        let names: Vec<String> = self.cases.iter().map(|c| c.name.clone()).collect();
        let update = std::env::var("RISOR_UPDATE").is_ok_and(|v| v == "1");
        if update && !self.program {
            if let Some(text) = &self.text {
                let (rs, new) = super::update::update_text(text, &self.cases);
                if let Some(t) = &new {
                    if !self.path.as_os_str().is_empty() {
                        // The file is the host's own; a write that fails is
                        // reported by the test that reads it next.
                        let _ = std::fs::write(&self.path, t);
                    }
                }
                return Report {
                    names,
                    verdicts: rs.into_iter().map(|(_, v)| v).collect(),
                    updated: new,
                };
            }
        }
        Report {
            names,
            verdicts: self.cases.iter().map(|c| c.run(vars)).collect(),
            updated: None,
        }
    }
}

impl Case {
    /// Run the case in a child of the base its file was loaded with, with
    /// `vars` for the names it does not supply.
    pub fn run(&self, vars: &dyn Vars) -> Verdict {
        run_case_ctx(
            self,
            Ctx {
                base: self.base.as_ref().map(|b| &b.0),
                vars: Some(vars),
            },
        )
    }
}
