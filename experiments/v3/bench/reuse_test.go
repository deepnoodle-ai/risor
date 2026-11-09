package bench

import (
	"context"
	"reflect"
	"testing"

	"github.com/deepnoodle-ai/expr"
	"github.com/deepnoodle-ai/risor/v2"
	"github.com/deepnoodle-ai/risor/v2/pkg/object"
	"github.com/deepnoodle-ai/risor/v2/pkg/vm"
)

// BenchmarkRunReusedVM measures the floor a host could reach with today's
// packages: one VM kept between evaluations (vm.RunCode), values converted
// once, and the result converted back to Go as risor.Run does. A VM runs one
// evaluation at a time, so a host would keep a pool.
func BenchmarkRunReusedVM(b *testing.B) {
	ctx := context.Background()
	data := env(2000)
	pre := preconverted(data)
	for _, c := range cases {
		p, err := expr.Compile(c.expr, exprOptions...)
		if err != nil {
			b.Fatal(err)
		}
		want, err := p.Run(ctx, data)
		if err != nil {
			b.Fatal(err)
		}
		code, err := risor.Compile(ctx, c.risor, risorOpts(pre)...)
		if err != nil {
			b.Fatal(err)
		}
		machine, err := vm.New(code, vm.WithGlobals(pre), vm.WithMaxSteps(1_000_000))
		if err != nil {
			b.Fatal(err)
		}
		eval := func() (any, error) {
			if err := machine.RunCode(ctx, code, vm.WithGlobals(pre)); err != nil {
				return nil, err
			}
			result, _ := machine.TOS()
			return object.ToInterface(ctx, result)
		}
		got, err := eval()
		if err != nil {
			b.Fatal(err)
		}
		if !reflect.DeepEqual(got, want) {
			b.Fatalf("%s: reused VM gave %#v, expr %#v", c.name, got, want)
		}
		b.Run(c.name+"/risor_vm", func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				if _, err := eval(); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
