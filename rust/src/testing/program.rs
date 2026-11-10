//! Package F9: case files written as programs (`f09-testing.md` 5).

use crate::env::Env;
use crate::json::Json;

/// The value of a program case file as the JSON of the same case file
/// (5.2 steps 1 to 3, 5.3), or why the file is invalid. The program is
/// parsed, checked in the default environment with no expected result, and
/// run with the default limits; step 4, the reading of the value, is the
/// caller's.
pub fn program_value(text: &str) -> Result<Json, String> {
    let env = Env::default();
    let parsed =
        crate::syntax::parse(text).map_err(|e| crate::render::render_line(&e.diagnostics))?;
    let (program, _) =
        crate::check::compile_checked(&env, parsed, &crate::check::Expected::Nothing)
            .map_err(|e| crate::render::render_line(&e.diagnostics))?;
    let vars = std::collections::BTreeMap::<String, crate::host::Data>::new();
    let out = program.run(&vars);
    let v = out
        .result
        .map_err(|f| crate::render::render_line(&[program.diagnostic(&f)]))?;
    let j = super::run::value_to_json(&v);
    if depth(&j) > crate::json::MAX_DEPTH {
        return Err("the value nests deeper than a case file may".into());
    }
    Ok(j)
}

fn depth(j: &Json) -> usize {
    match j {
        Json::Arr(a) => 1 + a.iter().map(depth).max().unwrap_or(0),
        Json::Obj(m) => 1 + m.iter().map(|(_, x)| depth(x)).max().unwrap_or(0),
        _ => 1,
    }
}
