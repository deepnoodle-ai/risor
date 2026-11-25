package benchexpr

// benchCase holds one representative expression written idiomatically in each
// engine. An empty string means the engine does not provide that operation
// natively without custom host code.
type benchCase struct {
	name       string
	category   string
	risor      string
	deepnoodle string
	exprLang   string
}

var exprCases = []benchCase{
	// Rules & conditions
	{
		name:       "field_read",
		category:   "rule",
		risor:      `input.event.pull_request.head.sha`,
		deepnoodle: `input.event.pull_request.head.sha`,
		exprLang:   `input.event.pull_request.head.sha`,
	},
	{
		name:       "simple_rule",
		category:   "rule",
		risor:      `gate.failed > 0`,
		deepnoodle: `gate.failed > 0`,
		exprLang:   `gate.failed > 0`,
	},
	{
		name:       "compound_rule",
		category:   "rule",
		risor:      `order.total > 100 && order.country == "US" && user.role in ["admin", "owner"] && !order.flagged`,
		deepnoodle: `order.total > 100 && order.country == "US" && contains(["admin", "owner"], user.role) && !order.flagged`,
		exprLang:   `order.total > 100 && order.country == "US" && user.role in ["admin", "owner"] && !order.flagged`,
	},
	{
		name:       "optional_coalesce",
		category:   "rule",
		risor:      `approve?.status ?? report?.status ?? "error"`,
		deepnoodle: `coalesce(approve?.status, report?.status, "error")`,
		exprLang:   `approve?.status ?? report?.status ?? "error"`,
	},
	{
		name:       "branch_arith",
		category:   "arith",
		risor:      `if (order.total >= 500) { order.total * 0.85 } else if (order.total >= 100) { order.total * 0.95 } else { order.total }`,
		deepnoodle: `if(order.total >= 500, order.total * 0.85, if(order.total >= 100, order.total * 0.95, order.total))`,
		exprLang:   `order.total >= 500 ? order.total * 0.85 : order.total >= 100 ? order.total * 0.95 : order.total`,
	},

	// String formatting / inline template expression
	{
		name:       "string_interp",
		category:   "template",
		risor:      "`${err.step} failed (${err.type}): ${err.message}`",
		deepnoodle: `err.step + " failed (" + err.type + "): " + err.message`,
		exprLang:   `err.step + " failed (" + err.type + "): " + err.message`,
	},

	// Queries over lists of records
	{
		name:       "any_check_10",
		category:   "query",
		risor:      `!verify.some(v => v.confirmed)`,
		deepnoodle: `!any(verify, it.confirmed)`,
		exprLang:   `!any(verify, .confirmed)`,
	},
	{
		name:       "filter_map_50",
		category:   "query",
		risor:      `fetch.tests50.filter(t => t.status == "fail").map(t => t.name)`,
		deepnoodle: `fetch.tests50 | filter(it.status == "fail") | map(it.name)`,
		exprLang:   `map(filter(fetch.tests50, .status == "fail"), .name)`,
	},
	{
		name:       "projection_100",
		category:   "query",
		risor:      `fetch.tests100[*].name`,
		deepnoodle: `fetch.tests100 | map(it.name)`,
		exprLang:   `map(fetch.tests100, .name)`,
	},
	{
		name:       "filter_sum_100",
		category:   "query",
		risor:      `fetch.tests100.filter(t => t.status == "fail").map(t => t.ms).sum()`,
		deepnoodle: `fetch.tests100 | filter(it.status == "fail") | map(it.ms) | sum()`,
		exprLang:   `sum(filter(fetch.tests100, .status == "fail"), .ms)`,
	},
	{
		name:       "sort_top3_100",
		category:   "query",
		risor:      `fetch.tests100.filter(t => t.status == "fail").sortBy(t => t.ms, "desc").slice(0, 3).map(t => t.name)`,
		deepnoodle: ``, // deepnoodle-ai/expr has no sortBy
		exprLang:   `map(take(sortBy(filter(fetch.tests100, .status == "fail"), .ms, "desc"), 3), .name)`,
	},
}

// templateCase compares host template engines (Risor v3 ParseTemplate and
// deepnoodle-ai/expr NewTemplate) alongside an equivalent expr-lang/expr
// string concatenation expression.
type templateCase struct {
	name       string
	risor      string
	deepnoodle string
	exprLang   string
}

var templateCases = []templateCase{
	{
		name:       "greeting",
		risor:      `Hello, ${{ user.name }} (${{ user.role }})!`,
		deepnoodle: `Hello, ${user.name} (${user.role})!`,
		exprLang:   `"Hello, " + user.name + " (" + user.role + ")!"`,
	},
	{
		name:       "pr_summary",
		risor:      `PR #${{ input.event.pull_request.number }} (${{ input.event.pull_request.title }}) on ${{ input.event.pull_request.head.ref }}: ${{ gate.failed }} failed, ${{ gate.passed }} passed`,
		deepnoodle: `PR #${input.event.pull_request.number} (${input.event.pull_request.title}) on ${input.event.pull_request.head.ref}: ${gate.failed} failed, ${gate.passed} passed`,
		exprLang:   `"PR #" + string(input.event.pull_request.number) + " (" + input.event.pull_request.title + ") on " + input.event.pull_request.head.ref + ": " + string(gate.failed) + " failed, " + string(gate.passed) + " passed"`,
	},
}
