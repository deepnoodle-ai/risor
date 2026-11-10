//! The library entries this build runs (`04-library.md` 5). Each charges by
//! its record's formula, in the order of 4.4. Every failure an entry raises
//! has the span of the whole call (2.6 item 6).

use super::{fail, kind_p, Arg, Run, R};
use crate::library::records;
use crate::source::Span;
use crate::value::{size, Kind, Str, Value};

impl<'h> Run<'h> {
    fn arg(&self, args: &[Arg<'h>], i: usize) -> Value<'h> {
        match args.get(i) {
            Some(Arg::Val(v)) => v.clone(),
            _ => Value::Null,
        }
    }

    fn argument(&self, name: &str, v: &Value<'h>, span: Span) -> Box<super::Fail<'h>> {
        fail(
            "wrong-kind",
            "argument",
            span,
            vec![("name", name.to_string()), ("kind", kind_p(v))],
        )
    }

    pub(super) fn lib_function(
        &mut self,
        idx: usize,
        args: Vec<Arg<'h>>,
        span: Span,
    ) -> R<'h, Value<'h>> {
        let name = records()[idx].name.as_str();
        match name {
            "len" => {
                let x = self.arg(&args, 0);
                let n = match &x {
                    Value::Str(s) => s.as_str().chars().count(),
                    _ if matches!(x.kind(), Kind::List | Kind::Map) => x.len().unwrap_or(0),
                    _ => return Err(self.argument(name, &x, span)),
                };
                self.charge(1)?;
                if let Value::Str(s) = &x {
                    self.charge(size(s.as_str()))?;
                }
                Ok(Value::Num(n as f64))
            }
            "string" => {
                let x = self.arg(&args, 0);
                self.charge(1)?;
                if let Value::Str(_) = x {
                    return Ok(x);
                }
                let mut out = String::new();
                self.render(&x, &mut out, true, span)?;
                let sz = size(&out);
                self.charge(sz)?;
                self.alloc(1 + sz)?;
                Ok(Value::str(out))
            }
            "type" => {
                let x = self.arg(&args, 0);
                self.charge(1)?;
                Ok(Value::Str(Str::Lent(x.kind().name())))
            }
            "bool" => {
                let x = self.arg(&args, 0);
                self.charge(1)?;
                Ok(Value::Bool(x.truthy()))
            }
            "fail" => {
                let m = self.arg(&args, 0);
                let Value::Str(msg) = &m else {
                    return Err(self.argument(name, &m, span));
                };
                self.charge(1)?;
                let mut f = fail("failed", "default", span, vec![]);
                f.text = Some(msg.as_str().to_string());
                f.data = Some(self.arg(&args, 1));
                Err(f)
            }
            _ => Err(self.internal(span)),
        }
    }

    pub(super) fn lib_method(
        &mut self,
        idx: usize,
        recv: Value<'h>,
        args: Vec<Arg<'h>>,
        span: Span,
        measure: u32,
    ) -> R<'h, Value<'h>> {
        let name = records()[idx].name.as_str();
        match name {
            "map" | "filter" => {
                let Some(Arg::Lam(f)) = args.first().cloned() else {
                    return Err(self.internal(span));
                };
                self.charge(1)?;
                let n = recv.len().unwrap_or(0);
                let mut out = Vec::new();
                for i in 0..n {
                    self.charge(2)?;
                    let e = self.read(recv.elem(i), span)?;
                    let r = self.invoke(f, &[e.clone(), Value::Num(i as f64)], measure, span)?;
                    if name == "map" {
                        out.push(r);
                    } else if r.truthy() {
                        out.push(e);
                    }
                }
                self.alloc(1 + out.len() as u64)?;
                Ok(Value::list(out))
            }
            _ => Err(self.internal(span)),
        }
    }
}
