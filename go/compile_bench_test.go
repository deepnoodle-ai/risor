package risor_test

import (
	"errors"
	"strings"
	"testing"

	risor "github.com/deepnoodle-ai/risor/go/v3"
)

// midProgram is a program of middle size: const lines, records, lambdas,
// a match, list methods and a template, as a workflow step might hold.
const midProgram = `const items = input.items;
const failed = items.filter(i => i.status == "fail");
const passed = items.filter(i => i.status == "pass");
const owners = failed.map(i => i.owner ?? "nobody");
const slow = items.filter(i => i.ms > 100).sortBy(i => i.ms).slice(0, 5);
const rate = len(items) > 0 ? len(failed) / len(items) : 0;
const level = match input.role { "admin" => 3, "owner" => 2, _ => 1 };
const summary = {
  total: len(items),
  failed: len(failed),
  passed: len(passed),
  rate,
  owners: owners.unique(),
  slow: slow.map(({name, ms}) => {name, ms}),
  tags: input.tags.map(t => t.toLowerCase()).filter(t => t != ""),
};
const verdict = if (len(failed) == 0) { "pass" }
  else if (rate < 0.1) { "flaky" }
  else { "fail" };
const note = try { input?.note ?? "none" } catch { "error" };
const byOwner = failed.groupBy(i => i.owner ?? "nobody");
const retries = input.options?.retries ?? 0;
{
  verdict,
  level,
  note,
  retries,
  summary,
  groups: byOwner.keys().sort(),
  text: ` + "`${verdict}: ${len(failed)} of ${len(items)} failed`" + `,
  first: items.find(i => i.status == "fail")?.name,
}`

func BenchmarkCompileMid(b *testing.B) {
	env, err := risor.NewEnv(risor.Declare(`{input: {role: "admin" | "member" | "owner", note?: string,
  tags: string[], options: {retries?: int} | null,
  items: {name: string, status: "fail" | "pass", owner: string | null, ms: float}[]}}`))
	if err != nil {
		b.Fatal(err)
	}
	x, err := risor.Parse(midProgram)
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	for b.Loop() {
		if _, err := env.Compile(x); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkCompileCheckFail is the mid program with one mistake: a
// misspelled field, so the compile fails with one check diagnostic and
// builds its message (package F8 3.5).
func BenchmarkCompileCheckFail(b *testing.B) {
	env, err := risor.NewEnv(risor.Declare(`{input: {role: "admin" | "member" | "owner", note?: string,
  tags: string[], options: {retries?: int} | null,
  items: {name: string, status: "fail" | "pass", owner: string | null, ms: float}[]}}`))
	if err != nil {
		b.Fatal(err)
	}
	x, err := risor.Parse(strings.Replace(midProgram, "i.ms > 100", "i.mss > 100", 1))
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	for b.Loop() {
		_, err := env.Compile(x)
		var e *risor.Error
		if !errors.As(err, &e) || len(e.Diagnostics) != 1 || e.Diagnostics[0].Message == "" {
			b.Fatal(err)
		}
	}
}
