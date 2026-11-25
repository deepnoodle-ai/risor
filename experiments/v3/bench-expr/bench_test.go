package benchexpr

import (
	"context"
	"fmt"
	"reflect"
	"testing"

	dexpr "github.com/deepnoodle-ai/expr"
	risor "github.com/deepnoodle-ai/risor/go/v3"
	"github.com/deepnoodle-ai/wonton/assert"
	exprlang "github.com/expr-lang/expr"
)

// normalize converts numeric types to float64 and typed slices to []any so
// results from all three engines can be compared for semantic equality.
func normalize(v any) any {
	switch x := v.(type) {
	case int:
		return float64(x)
	case int64:
		return float64(x)
	case float64:
		return x
	case []any:
		out := make([]any, len(x))
		for i, item := range x {
			out[i] = normalize(item)
		}
		return out
	case []string:
		out := make([]any, len(x))
		for i, item := range x {
			out[i] = item
		}
		return out
	}
	return v
}

// TestAgree verifies that every engine returns the same value on every case
// for both map and struct inputs, and that static type checking passes.
func TestAgree(t *testing.T) {
	ctx := context.Background()

	mapData := makeMapEnv()
	eMapSchema := makeExprLangMapSchema()
	risorMapVars := risor.Map(mapData)
	structData := makeStructEnv()
	risorStructVars := makeRisorStructVars(structData)

	rMapEnv, err := newRisorMapEnv()
	assert.NoError(t, err)
	rStructEnv, err := newRisorStructEnv()
	assert.NoError(t, err)

	dMapOpts := deepnoodleMapOpts()
	dStructOpts := deepnoodleStructOpts()
	dTypes := deepnoodleTypeEnv()
	dCheckOpts := deepnoodleCheckOpts()

	for _, c := range exprCases {
		t.Run(c.name, func(t *testing.T) {
			// Risor v3 (maps)
			rx, err := risor.Parse(c.risor)
			assert.NoError(t, err, "risor parse")
			rpMap, err := rMapEnv.Compile(rx)
			assert.NoError(t, err, "risor compile maps")
			want, err := rpMap.Run(ctx, risorMapVars)
			assert.NoError(t, err, "risor run maps")
			wantNorm := normalize(want)

			// Risor v3 (structs, typed layer)
			rpStruct, err := rStructEnv.Compile(rx)
			assert.NoError(t, err, "risor compile structs")
			gotStruct, err := rpStruct.Run(ctx, risorStructVars)
			assert.NoError(t, err, "risor run structs")
			assert.True(t, reflect.DeepEqual(normalize(gotStruct), wantNorm),
				"risor structs %v != maps %v", gotStruct, want)

			// deepnoodle-ai/expr (maps + Check, and structs)
			if c.deepnoodle != "" {
				dpMap, err := dexpr.Compile(c.deepnoodle, dMapOpts...)
				assert.NoError(t, err, "deepnoodle/expr compile maps")
				_, err = dpMap.Check(dTypes, dCheckOpts...)
				assert.NoError(t, err, "deepnoodle/expr check maps")
				gotDM, err := dpMap.Run(ctx, mapData)
				assert.NoError(t, err, "deepnoodle/expr run maps")
				assert.True(t, reflect.DeepEqual(normalize(gotDM), wantNorm),
					"deepnoodle/expr maps %v != risor %v", gotDM, want)

				dpStruct, err := dexpr.Compile(c.deepnoodle, dStructOpts...)
				assert.NoError(t, err, "deepnoodle/expr compile structs")
				gotDS, err := dpStruct.Run(ctx, structData)
				assert.NoError(t, err, "deepnoodle/expr run structs")
				assert.True(t, reflect.DeepEqual(normalize(gotDS), wantNorm),
					"deepnoodle/expr structs %v != risor %v", gotDS, want)
			}

			// expr-lang/expr (maps and structs)
			if c.exprLang != "" {
				epMap, err := exprlang.Compile(c.exprLang, exprlang.Env(eMapSchema))
				assert.NoError(t, err, "expr-lang/expr compile maps")
				gotEM, err := exprlang.Run(epMap, mapData)
				assert.NoError(t, err, "expr-lang/expr run maps")
				assert.True(t, reflect.DeepEqual(normalize(gotEM), wantNorm),
					"expr-lang/expr maps %v != risor %v", gotEM, want)

				epStruct, err := exprlang.Compile(c.exprLang, exprlang.Env(structData))
				assert.NoError(t, err, "expr-lang/expr compile structs")
				gotES, err := exprlang.Run(epStruct, structData)
				assert.NoError(t, err, "expr-lang/expr run structs")
				assert.True(t, reflect.DeepEqual(normalize(gotES), wantNorm),
					"expr-lang/expr structs %v != risor %v", gotES, want)
			}

			t.Logf("%-18s %v", c.name, fmt.Sprint(want))
		})
	}

	for _, tc := range templateCases {
		t.Run("template_"+tc.name, func(t *testing.T) {
			rt, err := risor.ParseTemplate(tc.risor)
			assert.NoError(t, err, "risor parse template")
			rpMap, err := rMapEnv.Compile(rt.Program(risor.ModeString))
			assert.NoError(t, err, "risor compile template maps")
			want, err := rpMap.Run(ctx, risorMapVars)
			assert.NoError(t, err, "risor run template maps")

			rpStruct, err := rStructEnv.Compile(rt.Program(risor.ModeString))
			assert.NoError(t, err, "risor compile template structs")
			gotRS, err := rpStruct.Run(ctx, risorStructVars)
			assert.NoError(t, err, "risor run template structs")
			assert.Equal(t, gotRS, want)

			dtMap, err := dexpr.NewTemplate(tc.deepnoodle, dMapOpts...)
			assert.NoError(t, err, "deepnoodle/expr compile template maps")
			gotDM, err := dtMap.Render(ctx, mapData)
			assert.NoError(t, err, "deepnoodle/expr render template maps")
			assert.Equal(t, any(gotDM), want)

			dtStruct, err := dexpr.NewTemplate(tc.deepnoodle, dStructOpts...)
			assert.NoError(t, err, "deepnoodle/expr compile template structs")
			gotDS, err := dtStruct.Render(ctx, structData)
			assert.NoError(t, err, "deepnoodle/expr render template structs")
			assert.Equal(t, any(gotDS), want)

			epMap, err := exprlang.Compile(tc.exprLang, exprlang.Env(eMapSchema))
			assert.NoError(t, err, "expr-lang/expr compile template maps")
			gotEM, err := exprlang.Run(epMap, mapData)
			assert.NoError(t, err, "expr-lang/expr run template maps")
			assert.Equal(t, gotEM, want)

			epStruct, err := exprlang.Compile(tc.exprLang, exprlang.Env(structData))
			assert.NoError(t, err, "expr-lang/expr compile template structs")
			gotES, err := exprlang.Run(epStruct, structData)
			assert.NoError(t, err, "expr-lang/expr run template structs")
			assert.Equal(t, gotES, want)

			t.Logf("template %-10s %v", tc.name, want)
		})
	}
}

// BenchmarkCompileMaps measures source text to a runnable program against a
// map-backed environment.
func BenchmarkCompileMaps(b *testing.B) {
	rEnv, err := newRisorMapEnv()
	if err != nil {
		b.Fatal(err)
	}
	eMapSchema := makeExprLangMapSchema()
	dOpts := deepnoodleMapOpts()
	dTypes := deepnoodleTypeEnv()
	dCheckOpts := deepnoodleCheckOpts()

	for _, c := range exprCases {
		b.Run(c.name+"/risor_v3", func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				x, err := risor.Parse(c.risor)
				if err != nil {
					b.Fatal(err)
				}
				if _, err := rEnv.Compile(x); err != nil {
					b.Fatal(err)
				}
			}
		})
		b.Run(c.name+"/risor_v3_nocheck", func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				x, err := risor.Parse(c.risor)
				if err != nil {
					b.Fatal(err)
				}
				if _, err := rEnv.Compile(x, risor.NoCheck); err != nil {
					b.Fatal(err)
				}
			}
		})
		if c.deepnoodle != "" {
			b.Run(c.name+"/deepnoodle_expr", func(b *testing.B) {
				b.ReportAllocs()
				for b.Loop() {
					if _, err := dexpr.Compile(c.deepnoodle, dOpts...); err != nil {
						b.Fatal(err)
					}
				}
			})
			b.Run(c.name+"/deepnoodle_expr_check", func(b *testing.B) {
				b.ReportAllocs()
				for b.Loop() {
					p, err := dexpr.Compile(c.deepnoodle, dOpts...)
					if err != nil {
						b.Fatal(err)
					}
					if _, err := p.Check(dTypes, dCheckOpts...); err != nil {
						b.Fatal(err)
					}
				}
			})
		}
		if c.exprLang != "" {
			b.Run(c.name+"/expr_lang", func(b *testing.B) {
				b.ReportAllocs()
				for b.Loop() {
					if _, err := exprlang.Compile(c.exprLang, exprlang.Env(eMapSchema)); err != nil {
						b.Fatal(err)
					}
				}
			})
		}
	}
}

// BenchmarkCompileStructs measures source text to a runnable program against a
// Go struct-backed environment (Risor v3 with TypeOf + F11 typed layer,
// deepnoodle-ai/expr with json tags, expr-lang/expr with struct Env).
func BenchmarkCompileStructs(b *testing.B) {
	rEnv, err := newRisorStructEnv()
	if err != nil {
		b.Fatal(err)
	}
	structSchema := RootEnv{}
	dOpts := deepnoodleStructOpts()

	for _, c := range exprCases {
		b.Run(c.name+"/risor_v3", func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				x, err := risor.Parse(c.risor)
				if err != nil {
					b.Fatal(err)
				}
				if _, err := rEnv.Compile(x); err != nil {
					b.Fatal(err)
				}
			}
		})
		b.Run(c.name+"/risor_v3_nocheck", func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				x, err := risor.Parse(c.risor)
				if err != nil {
					b.Fatal(err)
				}
				if _, err := rEnv.Compile(x, risor.NoCheck); err != nil {
					b.Fatal(err)
				}
			}
		})
		if c.deepnoodle != "" {
			b.Run(c.name+"/deepnoodle_expr", func(b *testing.B) {
				b.ReportAllocs()
				for b.Loop() {
					if _, err := dexpr.Compile(c.deepnoodle, dOpts...); err != nil {
						b.Fatal(err)
					}
				}
			})
		}
		if c.exprLang != "" {
			b.Run(c.name+"/expr_lang", func(b *testing.B) {
				b.ReportAllocs()
				for b.Loop() {
					if _, err := exprlang.Compile(c.exprLang, exprlang.Env(structSchema)); err != nil {
						b.Fatal(err)
					}
				}
			})
		}
	}
}

// BenchmarkRunMaps measures evaluating a pre-compiled program over JSON-shaped
// map[string]any data.
func BenchmarkRunMaps(b *testing.B) {
	ctx := context.Background()
	mapData := makeMapEnv()
	eMapSchema := makeExprLangMapSchema()
	risorVars := risor.Map(mapData)
	rEnv, err := newRisorMapEnv()
	if err != nil {
		b.Fatal(err)
	}
	dOpts := deepnoodleMapOpts()

	for _, c := range exprCases {
		rx, err := risor.Parse(c.risor)
		if err != nil {
			b.Fatal(c.name, err)
		}
		rp, err := rEnv.Compile(rx)
		if err != nil {
			b.Fatal(c.name, err)
		}
		b.Run(c.name+"/risor_v3", func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				if _, err := rp.Run(ctx, risorVars); err != nil {
					b.Fatal(err)
				}
			}
		})

		if c.deepnoodle != "" {
			dp, err := dexpr.Compile(c.deepnoodle, dOpts...)
			if err != nil {
				b.Fatal(c.name, err)
			}
			b.Run(c.name+"/deepnoodle_expr", func(b *testing.B) {
				b.ReportAllocs()
				for b.Loop() {
					if _, err := dp.Run(ctx, mapData); err != nil {
						b.Fatal(err)
					}
				}
			})
		}

		if c.exprLang != "" {
			ep, err := exprlang.Compile(c.exprLang, exprlang.Env(eMapSchema))
			if err != nil {
				b.Fatal(c.name, err)
			}
			b.Run(c.name+"/expr_lang", func(b *testing.B) {
				b.ReportAllocs()
				for b.Loop() {
					if _, err := exprlang.Run(ep, mapData); err != nil {
						b.Fatal(err)
					}
				}
			})
		}
	}
}

// BenchmarkRunStructs measures evaluating a pre-compiled program over Go
// structs. Risor v3 uses TypeOf + the F11 typed layer; expr-lang/expr uses
// struct bytecode field ops; deepnoodle-ai/expr uses json struct tags.
func BenchmarkRunStructs(b *testing.B) {
	ctx := context.Background()
	structData := makeStructEnv()
	risorVars := makeRisorStructVars(structData)
	rEnv, err := newRisorStructEnv()
	if err != nil {
		b.Fatal(err)
	}
	dOpts := deepnoodleStructOpts()

	for _, c := range exprCases {
		rx, err := risor.Parse(c.risor)
		if err != nil {
			b.Fatal(c.name, err)
		}
		rp, err := rEnv.Compile(rx)
		if err != nil {
			b.Fatal(c.name, err)
		}
		b.Run(c.name+"/risor_v3", func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				if _, err := rp.Run(ctx, risorVars); err != nil {
					b.Fatal(err)
				}
			}
		})

		if c.deepnoodle != "" {
			dp, err := dexpr.Compile(c.deepnoodle, dOpts...)
			if err != nil {
				b.Fatal(c.name, err)
			}
			b.Run(c.name+"/deepnoodle_expr", func(b *testing.B) {
				b.ReportAllocs()
				for b.Loop() {
					if _, err := dp.Run(ctx, structData); err != nil {
						b.Fatal(err)
					}
				}
			})
		}

		if c.exprLang != "" {
			ep, err := exprlang.Compile(c.exprLang, exprlang.Env(structData))
			if err != nil {
				b.Fatal(c.name, err)
			}
			b.Run(c.name+"/expr_lang", func(b *testing.B) {
				b.ReportAllocs()
				for b.Loop() {
					if _, err := exprlang.Run(ep, structData); err != nil {
						b.Fatal(err)
					}
				}
			})
		}
	}
}

// BenchmarkTemplateCompile measures host template compilation (Risor v3
// ParseTemplate + Compile, deepnoodle-ai/expr NewTemplate, and expr-lang/expr
// sprintf compile).
func BenchmarkTemplateCompile(b *testing.B) {
	rEnv, err := newRisorStructEnv()
	if err != nil {
		b.Fatal(err)
	}
	structSchema := RootEnv{}
	dOpts := deepnoodleStructOpts()

	for _, tc := range templateCases {
		b.Run(tc.name+"/risor_v3", func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				tpl, err := risor.ParseTemplate(tc.risor)
				if err != nil {
					b.Fatal(err)
				}
				if _, err := rEnv.Compile(tpl.Program(risor.ModeString)); err != nil {
					b.Fatal(err)
				}
			}
		})
		b.Run(tc.name+"/risor_v3_nocheck", func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				tpl, err := risor.ParseTemplate(tc.risor)
				if err != nil {
					b.Fatal(err)
				}
				if _, err := rEnv.Compile(tpl.Program(risor.ModeString), risor.NoCheck); err != nil {
					b.Fatal(err)
				}
			}
		})
		b.Run(tc.name+"/deepnoodle_expr", func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				if _, err := dexpr.NewTemplate(tc.deepnoodle, dOpts...); err != nil {
					b.Fatal(err)
				}
			}
		})
		b.Run(tc.name+"/expr_lang", func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				if _, err := exprlang.Compile(tc.exprLang, exprlang.Env(structSchema)); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// BenchmarkTemplateRun measures rendering a pre-compiled host template over
// map and struct data.
func BenchmarkTemplateRun(b *testing.B) {
	ctx := context.Background()
	mapData := makeMapEnv()
	eMapSchema := makeExprLangMapSchema()
	risorMapVars := risor.Map(mapData)
	structData := makeStructEnv()
	risorStructVars := makeRisorStructVars(structData)

	rMapEnv, err := newRisorMapEnv()
	if err != nil {
		b.Fatal(err)
	}
	rStructEnv, err := newRisorStructEnv()
	if err != nil {
		b.Fatal(err)
	}
	dMapOpts := deepnoodleMapOpts()
	dStructOpts := deepnoodleStructOpts()

	for _, tc := range templateCases {
		tpl, err := risor.ParseTemplate(tc.risor)
		if err != nil {
			b.Fatal(err)
		}
		rpMap, err := rMapEnv.Compile(tpl.Program(risor.ModeString))
		if err != nil {
			b.Fatal(err)
		}
		rpStruct, err := rStructEnv.Compile(tpl.Program(risor.ModeString))
		if err != nil {
			b.Fatal(err)
		}
		dtMap, err := dexpr.NewTemplate(tc.deepnoodle, dMapOpts...)
		if err != nil {
			b.Fatal(err)
		}
		dtStruct, err := dexpr.NewTemplate(tc.deepnoodle, dStructOpts...)
		if err != nil {
			b.Fatal(err)
		}
		epMap, err := exprlang.Compile(tc.exprLang, exprlang.Env(eMapSchema))
		if err != nil {
			b.Fatal(err)
		}
		epStruct, err := exprlang.Compile(tc.exprLang, exprlang.Env(structData))
		if err != nil {
			b.Fatal(err)
		}

		b.Run(tc.name+"_maps/risor_v3", func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				if _, err := rpMap.Run(ctx, risorMapVars); err != nil {
					b.Fatal(err)
				}
			}
		})
		b.Run(tc.name+"_maps/deepnoodle_expr", func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				if _, err := dtMap.Render(ctx, mapData); err != nil {
					b.Fatal(err)
				}
			}
		})
		b.Run(tc.name+"_maps/expr_lang", func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				if _, err := exprlang.Run(epMap, mapData); err != nil {
					b.Fatal(err)
				}
			}
		})

		b.Run(tc.name+"_structs/risor_v3", func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				if _, err := rpStruct.Run(ctx, risorStructVars); err != nil {
					b.Fatal(err)
				}
			}
		})
		b.Run(tc.name+"_structs/deepnoodle_expr", func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				if _, err := dtStruct.Render(ctx, structData); err != nil {
					b.Fatal(err)
				}
			}
		})
		b.Run(tc.name+"_structs/expr_lang", func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				if _, err := exprlang.Run(epStruct, structData); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
