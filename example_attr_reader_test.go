package risor_test

import (
	"context"
	"fmt"
	"reflect"

	"github.com/deepnoodle-ai/risor/v2"
	"github.com/deepnoodle-ai/risor/v2/pkg/object"
)

type Account struct {
	Name   string
	Events []string
	Token  string
}

// AccountObject wraps an Account. Its ReadAttr receives the VM context, so
// reading a field from a script uses the limits from risor.WithLimits.
type AccountObject struct {
	*object.GoStruct
}

func (a *AccountObject) GetAttr(name string) (object.Object, bool) {
	if name == "Token" {
		return nil, false
	}
	return a.GoStruct.GetAttr(name)
}

// ReadAttr hides the same field as GetAttr, because the VM calls ReadAttr
// instead of GetAttr.
func (a *AccountObject) ReadAttr(ctx context.Context, name string) (object.Object, bool, error) {
	if name == "Token" {
		return nil, false, nil
	}
	return a.GoStruct.GetAttrContext(ctx, name)
}

// A host type that embeds *object.GoStruct implements object.AttrReader to
// apply the limits from risor.WithLimits to field reads.
func Example_attrReader() {
	account := &Account{Name: "a1", Events: []string{"open", "deposit", "withdraw"}, Token: "secret"}
	obj := &AccountObject{object.NewGoStruct(reflect.ValueOf(account), object.DefaultRegistry())}
	env := risor.WithEnv(map[string]any{"account": obj})
	limits := risor.WithLimits(risor.Limits{MaxListLength: 2})

	for _, src := range []string{`account.Name`, `account.Events`, `account.Token`} {
		result, err := risor.Eval(context.Background(), src, env, limits)
		fmt.Println(src, "->", result, err)
	}
	// Output:
	// account.Name -> a1 <nil>
	// account.Events -> <nil> value error: cannot read field "Events": value too large to convert (exceeds limit of 2 elements) (1:9)
	// account.Token -> <nil> type error: attribute "Token" not found on go_struct object (1:9)
}
