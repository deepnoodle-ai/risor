package risor_test

import (
	"context"
	"fmt"
	"strings"

	"github.com/deepnoodle-ai/risor/go/v3"
)

// Example is the program in README.md: declare the host's data, compile once,
// run many times.
func Example() {
	env, err := risor.NewEnv(
		risor.Var("tests", "{name: string, status: string, ms: int}[]"),
	)
	if err != nil {
		panic(err)
	}
	expr, err := risor.Parse(`
const failed = tests.filter(t => t.status == "fail");
{
  verdict: if (!failed) { "pass" } else { "fail" },
  names: failed[*].name,
}`)
	if err != nil {
		panic(err)
	}
	prog, err := env.Compile(expr)
	if err != nil {
		panic(err)
	}
	out, err := prog.Run(context.Background(), risor.Map(map[string]any{
		"tests": []any{
			map[string]any{"name": "login", "status": "pass", "ms": 12},
			map[string]any{"name": "upload", "status": "fail", "ms": 340},
		},
	}))
	if err != nil {
		panic(err)
	}
	fmt.Println(out)
	// Output: map[names:[upload] verdict:fail]
}

// Account is a host value that a program uses through its methods.
type Account struct {
	Owner string
	Cents int64
}

func (*Account) OpaqueType() string        { return "Account" }
func (a *Account) Name() string            { return a.Owner }
func (a *Account) Balance() int64          { return a.Cents }
func (a *Account) CanPay(cents int64) bool { return cents <= a.Cents }

// Example_func is the example in README.md of host functions and an opaque
// type derived from Go: Func takes a Go function, Methods a Go type's
// methods, and each signature comes from the Go types.
func Example_func() {
	accounts := map[string]*Account{"ada": {Owner: "Ada", Cents: 1250}}
	account := risor.Func("account", func(id string) (*Account, error) {
		if a, ok := accounts[id]; ok {
			return a, nil
		}
		return nil, fmt.Errorf("no account %q", id)
	}, risor.Params("id"))
	upper := risor.Func("upper", strings.ToUpper)
	env, err := risor.NewEnv(
		risor.Var("id", "string"),
		risor.Methods[*Account]("Account", risor.ForMethod("CanPay", risor.Params("cents"))),
		account,
		upper,
	)
	if err != nil {
		panic(err)
	}
	fmt.Println(account.Type)
	fmt.Println(upper.Type)
	expr, err := risor.Parse(`
const a = account(id);
{owner: upper(a.name()), balance: a.balance(), canPay: a.canPay(500)}`)
	if err != nil {
		panic(err)
	}
	prog, err := env.Compile(expr)
	if err != nil {
		panic(err)
	}
	out, err := prog.Run(context.Background(), risor.Map{"id": "ada"})
	fmt.Println(out, err)
	_, err = prog.Run(context.Background(), risor.Map{"id": "bob"})
	fmt.Println(err)
	// Output:
	// (id: string) => Account !
	// (a1: string) => string
	// map[balance:1250 canPay:true owner:ADA] <nil>
	// run function-error at [2, 11, 2, 22]: no account "bob"
}
