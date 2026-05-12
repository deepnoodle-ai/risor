# v3 Notes

A running list of changes to consider when planning a future v3 release.
Items here are deliberately deferred — they would break v2 scripts or commit
to implementation details we want flexibility to revisit.

This file is intentionally informal. Promote items to a proper proposal in
`docs/design/proposals/` when they're ready to be designed.

## Error handling

### Remove dependence on Go-specific error wrapping

**Today (v2):** The `error()` builtin uses `fmt.Errorf` internally, so scripts
can pass the `%w` verb to produce a wrapped error. `*Error.Equals` walks the
resulting chain via `errors.Is` (added in v2.2.0) so wrapped sentinels match
their descendants under `==`. Neither behavior is documented in the
language reference — both are implementation details.

**Concern:** Both `%w` and `errors.Is` are Go-specific. If Risor ever has a
Rust or TypeScript implementation, the wrap-chain model doesn't translate
cleanly. We do not want the script-level error semantics to require a
specific host language's error machinery.

**Direction for v3:**

- Switch the `error()` builtin from `fmt.Errorf` to `fmt.Sprintf`, with
  format-string validation that rejects unsupported verbs (`%w` in
  particular) with a clear scripting error.
- Reconsider what `==` should mean for errors. Options:
  - Identity-only (errors are equal iff they are the same value).
  - A portable "kind" model where errors carry a script-visible tag and
    `==` compares tags (`err.kind == "not_exist"` or similar).
  - Keep `errors.Is`-walking for errors produced by Go modules but disallow
    script-level construction of wrap chains.
- Drop the message-equality fallback in `*Error.Equals`. Two distinct errors
  with the same formatted message are not the same error.

**Compatibility plan:** This is a v3-only change. v2.2.0 leaves wrapping
working and undocumented; the v2.2.0 CHANGELOG explicitly calls wrap
mechanics "not part of the stable scripting API" so removing them in v3 is
not a documented-API break.

### Possible script-level introspection

If we keep the wrap model in any form, scripts may want to inspect chains
without going through `==`. Candidates:

- `err.cause()` / `err.unwrap()` — exposes the immediate underlying error
- `err.is(other)` — explicit chain check, one-directional, mirrors Go's
  `errors.Is`
- `err.kind` — a portable tag for categorizing errors

We deliberately did **not** add any of these in v2.2.0 to keep the API
surface small and to avoid committing to Go-shaped concepts that v3 might
remove.

## Other candidates

Add items here as they come up. Keep entries short — a few sentences each,
with a "concern" and a "direction" line. Anything that needs more than a
paragraph or two should graduate to a proposal.
