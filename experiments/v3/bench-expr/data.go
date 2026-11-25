package benchexpr

import (
	"context"
	"fmt"
	"maps"

	dexpr "github.com/deepnoodle-ai/expr"
	risor "github.com/deepnoodle-ai/risor/go/v3"
)

// Shared Go struct types used by all three engines on struct benchmarks.
// Each field carries both json and expr tags so Risor v3 (json),
// deepnoodle-ai/expr (WithStructTags("json")), and expr-lang/expr (expr)
// expose the exact same field names.

type Head struct {
	SHA string `json:"sha" expr:"sha"`
	Ref string `json:"ref" expr:"ref"`
}

type PullRequest struct {
	Number int      `json:"number" expr:"number"`
	Title  string   `json:"title" expr:"title"`
	Draft  bool     `json:"draft" expr:"draft"`
	Head   Head     `json:"head" expr:"head"`
	Base   *Head    `json:"base" expr:"base"`
	Labels []string `json:"labels" expr:"labels"`
}

type Event struct {
	Action      string      `json:"action" expr:"action"`
	PullRequest PullRequest `json:"pull_request" expr:"pull_request"`
}

type Input struct {
	Role  string `json:"role" expr:"role"`
	Event Event  `json:"event" expr:"event"`
}

type Order struct {
	Total   float64 `json:"total" expr:"total"`
	Country string  `json:"country" expr:"country"`
	Flagged bool    `json:"flagged" expr:"flagged"`
}

type User struct {
	Name string `json:"name" expr:"name"`
	Role string `json:"role" expr:"role"`
}

type Gate struct {
	Failed int `json:"failed" expr:"failed"`
	Passed int `json:"passed" expr:"passed"`
}

type Report struct {
	Status string `json:"status" expr:"status"`
}

type RunError struct {
	Step    string `json:"step" expr:"step"`
	Type    string `json:"type" expr:"type"`
	Message string `json:"message" expr:"message"`
}

type CheckItem struct {
	Finding   string `json:"finding" expr:"finding"`
	Confirmed bool   `json:"confirmed" expr:"confirmed"`
}

type TestRecord struct {
	Name   string  `json:"name" expr:"name"`
	Status string  `json:"status" expr:"status"`
	MS     float64 `json:"ms" expr:"ms"`
}

type Fetch struct {
	Tests50  []TestRecord `json:"tests50" expr:"tests50"`
	Tests100 []TestRecord `json:"tests100" expr:"tests100"`
}

// RootEnv is the top-level Go struct passed to expr-lang/expr and
// deepnoodle-ai/expr on struct benchmarks. Risor v3 receives each top-level
// field as a typed variable via risor.Map + risor.TypeOf[T]().
type RootEnv struct {
	Input   *Input      `json:"input" expr:"input"`
	Order   *Order      `json:"order" expr:"order"`
	User    *User       `json:"user" expr:"user"`
	Gate    *Gate       `json:"gate" expr:"gate"`
	Report  *Report     `json:"report" expr:"report"`
	Approve *Report     `json:"approve" expr:"approve"`
	Err     *RunError   `json:"err" expr:"err"`
	Verify  []CheckItem `json:"verify" expr:"verify"`
	Fetch   *Fetch      `json:"fetch" expr:"fetch"`
}

func makeStructEnv() RootEnv {
	tests50 := make([]TestRecord, 50)
	for i := range tests50 {
		status := "pass"
		if i%10 == 0 {
			status = "fail"
		}
		tests50[i] = TestRecord{
			Name:   fmt.Sprintf("test_%d", i),
			Status: status,
			MS:     float64((i + 1) * 3),
		}
	}
	tests100 := make([]TestRecord, 100)
	for i := range tests100 {
		status := "pass"
		if i%10 == 0 {
			status = "fail"
		}
		tests100[i] = TestRecord{
			Name:   fmt.Sprintf("test_%d", i),
			Status: status,
			MS:     float64((i + 1) * 3),
		}
	}
	verify := make([]CheckItem, 10)
	for i := range verify {
		verify[i] = CheckItem{
			Finding:   fmt.Sprintf("f%d", i),
			Confirmed: false,
		}
	}
	return RootEnv{
		Input: &Input{
			Role: "owner",
			Event: Event{
				Action: "opened",
				PullRequest: PullRequest{
					Number: 42,
					Title:  "Fix cache invalidation",
					Draft:  false,
					Head:   Head{SHA: "0123456789abcdef", Ref: "fix/cache"},
					Base:   &Head{SHA: "fedcba9876543210", Ref: "main"},
					Labels: []string{"bug", "p1"},
				},
			},
		},
		Order:   &Order{Total: 250.0, Country: "US", Flagged: false},
		User:    &User{Name: "Ada", Role: "owner"},
		Gate:    &Gate{Failed: 2, Passed: 48},
		Report:  &Report{Status: "success"},
		Approve: nil,
		Err:     &RunError{Step: "fetch", Type: "timeout", Message: "deadline exceeded"},
		Verify:  verify,
		Fetch:   &Fetch{Tests50: tests50, Tests100: tests100},
	}
}

func makeRisorStructVars(r RootEnv) risor.Map {
	return risor.Map{
		"input":   r.Input,
		"order":   r.Order,
		"user":    r.User,
		"gate":    r.Gate,
		"report":  r.Report,
		"approve": r.Approve,
		"err":     r.Err,
		"verify":  r.Verify,
		"fetch":   r.Fetch,
	}
}

func makeMapEnv() map[string]any {
	tests50 := make([]any, 50)
	for i := range tests50 {
		status := "pass"
		if i%10 == 0 {
			status = "fail"
		}
		tests50[i] = map[string]any{
			"name":   fmt.Sprintf("test_%d", i),
			"status": status,
			"ms":     float64((i + 1) * 3),
		}
	}
	tests100 := make([]any, 100)
	for i := range tests100 {
		status := "pass"
		if i%10 == 0 {
			status = "fail"
		}
		tests100[i] = map[string]any{
			"name":   fmt.Sprintf("test_%d", i),
			"status": status,
			"ms":     float64((i + 1) * 3),
		}
	}
	verify := make([]any, 10)
	for i := range verify {
		verify[i] = map[string]any{
			"finding":   fmt.Sprintf("f%d", i),
			"confirmed": false,
		}
	}
	return map[string]any{
		"input": map[string]any{
			"role": "owner",
			"event": map[string]any{
				"action": "opened",
				"pull_request": map[string]any{
					"number": 42,
					"title":  "Fix cache invalidation",
					"draft":  false,
					"head":   map[string]any{"sha": "0123456789abcdef", "ref": "fix/cache"},
					"base":   map[string]any{"sha": "fedcba9876543210", "ref": "main"},
					"labels": []any{"bug", "p1"},
				},
			},
		},
		"order":   map[string]any{"total": 250.0, "country": "US", "flagged": false},
		"user":    map[string]any{"name": "Ada", "role": "owner"},
		"gate":    map[string]any{"failed": 2, "passed": 48},
		"report":  map[string]any{"status": "success"},
		"approve": nil,
		"err":     map[string]any{"step": "fetch", "type": "timeout", "message": "deadline exceeded"},
		"verify":  verify,
		"fetch":   map[string]any{"tests50": tests50, "tests100": tests100},
	}
}

// makeExprLangMapSchema returns a small prototype map describing the same
// schema as makeMapEnv for expr-lang/expr's compile-time Env option.
// Nullable fields like approve are given their map shape here so expr-lang
// can type-check approve?.status while runtime data supplies nil.
func makeExprLangMapSchema() map[string]any {
	testRec := map[string]any{"name": "", "status": "", "ms": 0.0}
	return map[string]any{
		"input": map[string]any{
			"role": "",
			"event": map[string]any{
				"action": "",
				"pull_request": map[string]any{
					"number": 0,
					"title":  "",
					"draft":  false,
					"head":   map[string]any{"sha": "", "ref": ""},
					"base":   map[string]any{"sha": "", "ref": ""},
					"labels": []any{""},
				},
			},
		},
		"order":   map[string]any{"total": 0.0, "country": "", "flagged": false},
		"user":    map[string]any{"name": "", "role": ""},
		"gate":    map[string]any{"failed": 0, "passed": 0},
		"report":  map[string]any{"status": ""},
		"approve": map[string]any{"status": ""},
		"err":     map[string]any{"step": "", "type": "", "message": ""},
		"verify":  []any{map[string]any{"finding": "", "confirmed": false}},
		"fetch":   map[string]any{"tests50": []any{testRec}, "tests100": []any{testRec}},
	}
}

const risorMapRecordType = `{
	input: {
		role: string,
		event: {
			action: string,
			pull_request: {
				number: int,
				title: string,
				draft: bool,
				head: {sha: string, ref: string},
				base: {sha: string, ref: string} | null,
				labels: string[],
			},
		},
	},
	order: {total: float, country: string, flagged: bool},
	user: {name: string, role: string},
	gate: {failed: int, passed: int},
	report: {status: string} | null,
	approve: {status: string} | null,
	err: {step: string, type: string, message: string},
	verify: {finding: string, confirmed: bool}[],
	fetch: {
		tests50: {name: string, status: string, ms: float}[],
		tests100: {name: string, status: string, ms: float}[],
	},
}`

func mustRisorType[T any]() risor.Type {
	t, err := risor.TypeOf[T]()
	if err != nil {
		panic(err)
	}
	return t
}

func newRisorMapEnv() (*risor.Env, error) {
	return risor.NewEnv(risor.VarRecord(risorMapRecordType))
}

func newRisorStructEnv() (*risor.Env, error) {
	return risor.NewEnv(
		risor.VarType("input", mustRisorType[Input]()),
		risor.VarType("order", mustRisorType[Order]()),
		risor.VarType("user", mustRisorType[User]()),
		risor.VarType("gate", mustRisorType[Gate]()),
		risor.VarType("report", mustRisorType[*Report]()),
		risor.VarType("approve", mustRisorType[*Report]()),
		risor.VarType("err", mustRisorType[RunError]()),
		risor.VarType("verify", mustRisorType[[]CheckItem]()),
		risor.VarType("fetch", mustRisorType[Fetch]()),
	)
}

func deepnoodleExprFuncs() map[string]any {
	all := map[string]any{}
	for _, g := range []map[string]any{
		dexpr.Builtins(),
		dexpr.StringFuncs(),
		dexpr.MathFuncs(),
		dexpr.CollectionFuncs(),
	} {
		maps.Copy(all, g)
	}
	// deepnoodle-ai/expr has ?. but no ?? operator; hosts register coalesce.
	all["coalesce"] = dexpr.Func(func(_ context.Context, args []any) (any, error) {
		for _, a := range args {
			if a != nil {
				return a, nil
			}
		}
		return nil, nil
	})
	return all
}

func deepnoodleCheckOpts() []dexpr.CheckOption {
	return []dexpr.CheckOption{
		dexpr.WithFuncType("coalesce", func(args []*dexpr.Type) (*dexpr.Type, error) {
			if len(args) == 0 {
				return dexpr.Null(), nil
			}
			return args[len(args)-1], nil
		}),
	}
}

func deepnoodleMapOpts() []dexpr.Option {
	return []dexpr.Option{
		dexpr.WithFunctions(deepnoodleExprFuncs()),
		dexpr.WithEvalBudget(1_000_000),
	}
}

func deepnoodleStructOpts() []dexpr.Option {
	return []dexpr.Option{
		dexpr.WithFunctions(deepnoodleExprFuncs()),
		dexpr.WithEvalBudget(1_000_000),
		dexpr.WithStructTags("json"),
	}
}

func deepnoodleTypeEnv() map[string]*dexpr.Type {
	head := dexpr.ObjectOf(map[string]*dexpr.Type{
		"sha": dexpr.String(),
		"ref": dexpr.String(),
	})
	testRec := dexpr.ObjectOf(map[string]*dexpr.Type{
		"name":   dexpr.String(),
		"status": dexpr.String(),
		"ms":     dexpr.Number(),
	})
	report := dexpr.Join(
		dexpr.ObjectOf(map[string]*dexpr.Type{"status": dexpr.String()}),
		dexpr.Null(),
	)
	return map[string]*dexpr.Type{
		"input": dexpr.ObjectOf(map[string]*dexpr.Type{
			"role": dexpr.String(),
			"event": dexpr.ObjectOf(map[string]*dexpr.Type{
				"action": dexpr.String(),
				"pull_request": dexpr.ObjectOf(map[string]*dexpr.Type{
					"number": dexpr.Number(),
					"title":  dexpr.String(),
					"draft":  dexpr.Bool(),
					"head":   head,
					"base":   dexpr.Join(head, dexpr.Null()),
					"labels": dexpr.ListOf(dexpr.String()),
				}),
			}),
		}),
		"order": dexpr.ObjectOf(map[string]*dexpr.Type{
			"total":   dexpr.Number(),
			"country": dexpr.String(),
			"flagged": dexpr.Bool(),
		}),
		"user": dexpr.ObjectOf(map[string]*dexpr.Type{
			"name": dexpr.String(),
			"role": dexpr.String(),
		}),
		"gate": dexpr.ObjectOf(map[string]*dexpr.Type{
			"failed": dexpr.Number(),
			"passed": dexpr.Number(),
		}),
		"report":  report,
		"approve": report,
		"err": dexpr.ObjectOf(map[string]*dexpr.Type{
			"step":    dexpr.String(),
			"type":    dexpr.String(),
			"message": dexpr.String(),
		}),
		"verify": dexpr.ListOf(dexpr.ObjectOf(map[string]*dexpr.Type{
			"finding":   dexpr.String(),
			"confirmed": dexpr.Bool(),
		})),
		"fetch": dexpr.ObjectOf(map[string]*dexpr.Type{
			"tests50":  dexpr.ListOf(testRec),
			"tests100": dexpr.ListOf(testRec),
		}),
	}
}
