package risor_test

import (
	"context"
	"errors"
	"math"
	"sync"
	"sync/atomic"
	"testing"

	risor "github.com/deepnoodle-ai/risor/go/v3"
	"github.com/deepnoodle-ai/wonton/assert"
)

func TestEntriesMetadataAndSchema(t *testing.T) {
	env, err := risor.NewEnv()
	assert.NoError(t, err)

	src := `
type Units = "metric" | "imperial";
const helper = (x: int) => x + 1;
/** Get the current weather for a city. */
export const weather = (
  /** The city name, such as "Paris". */
  city: string,
  /** Temperature units. */
  units: Units = "metric",
): {city: string, units: string} => ({city, units});
export const ping = () => "pong";
export const pair = ({a}: {a: int}, b: int) => a + b;
`
	expr, err := risor.Parse(src)
	assert.NoError(t, err)
	assert.Equal(t, expr.Text(), src[1:len(src)-1])

	cr, err := env.Check(expr)
	assert.NoError(t, err)
	assert.Equal(t, cr.Type.String(), "never")
	assert.Equal(t, len(cr.Entries), 3)

	assert.Equal(t, cr.Entries[0].Name, "weather")
	assert.Equal(t, cr.Entries[0].Doc, "Get the current weather for a city.")
	assert.Equal(t, len(cr.Entries[0].Params), 2)
	assert.Equal(t, cr.Entries[0].Params[0].Name, "city")
	assert.Equal(t, cr.Entries[0].Params[0].Doc, `The city name, such as "Paris".`)
	assert.False(t, cr.Entries[0].Params[0].Optional)
	assert.Equal(t, cr.Entries[0].Params[1].Name, "units")
	assert.Equal(t, cr.Entries[0].Params[1].Doc, "Temperature units.")
	assert.True(t, cr.Entries[0].Params[1].Optional)
	assert.NotNil(t, cr.Entries[0].InputType)
	assert.NotNil(t, cr.Entries[0].ResultType)
	assert.Equal(t, cr.Entries[0].ResultType.String(), "{city: string, units: string}")

	schema, err := risor.SchemaOf(*cr.Entries[0].InputType)
	assert.NoError(t, err)
	assert.Equal(t, string(schema), `{"additionalProperties":false,"properties":{"city":{"description":"The city name, such as \"Paris\".","type":"string"},"units":{"description":"Temperature units.","enum":["imperial","metric"],"type":"string"}},"required":["city"],"type":"object"}`)

	assert.Equal(t, cr.Entries[1].Name, "ping")
	assert.Equal(t, cr.Entries[1].InputType.String(), "{}")
	assert.Equal(t, cr.Entries[1].ResultType.String(), `"pong"`)

	assert.Equal(t, cr.Entries[2].Name, "pair")
	assert.Equal(t, cr.Entries[2].Params[0].Name, "")
	assert.Nil(t, cr.Entries[2].InputType)
	assert.Equal(t, cr.Entries[2].ResultType.String(), "int")

	// Compiled program carries the same Entries and HasResult() == false.
	prog, err := env.Compile(expr)
	assert.NoError(t, err)
	assert.False(t, prog.HasResult())
	es := prog.Entries()
	assert.Equal(t, len(es), 3)
	// Mutating returned slice must not mutate Program state.
	es[0].Name = "mutated"
	es[0].Params[0].Name = "mutated"
	assert.Equal(t, prog.Entries()[0].Name, "weather")
	assert.Equal(t, prog.Entries()[0].Params[0].Name, "city")

	// NoCheck program still exposes parameter types and InputType, with nil ResultType.
	uc, err := env.Compile(expr, risor.NoCheck)
	assert.NoError(t, err)
	ues := uc.Entries()
	assert.Equal(t, len(ues), 3)
	assert.NotNil(t, ues[0].InputType)
	assert.Nil(t, ues[0].ResultType)
}

func TestEntryCallIsolationAndConcurrency(t *testing.T) {
	var calls atomic.Int64
	var unusedCalls atomic.Int64
	env, err := risor.NewEnv(
		risor.VarRecord("{tenant: string}"),
		risor.Function{
			Name: "nextID",
			Type: "() => int",
			Impl: func(c *risor.Call, args []any) (any, error) {
				return int(calls.Add(1)), nil
			},
		},
		risor.Function{
			Name: "unusedFn",
			Type: "() => int",
			Impl: func(c *risor.Call, args []any) (any, error) {
				unusedCalls.Add(1)
				return 0, nil
			},
		},
	)
	assert.NoError(t, err)

	src := `
const id = nextID();
const skipped = unusedFn();
export const make = (label: string) => ({tenant, id, label});
`
	expr, err := risor.Parse(src)
	assert.NoError(t, err)
	prog, err := env.Compile(expr)
	assert.NoError(t, err)

	// Sequential calls re-evaluate needs and do not leak variables.
	v1, u1, err := prog.CallMeasure(context.Background(), "make", []any{"a"}, risor.Map{"tenant": "t1"})
	assert.NoError(t, err)
	assert.Equal(t, v1, map[string]any{"tenant": "t1", "id": float64(1), "label": "a"})
	assert.True(t, u1.Steps > 0)

	v2, err := prog.CallRecord(context.Background(), "make", map[string]any{"label": "b"}, risor.Map{"tenant": "t2"})
	assert.NoError(t, err)
	assert.Equal(t, v2, map[string]any{"tenant": "t2", "id": float64(2), "label": "b"})
	assert.Equal(t, unusedCalls.Load(), int64(0))

	// Concurrent calls on one *Program.
	const workers = 16
	var wg sync.WaitGroup
	errs := make(chan error, workers)
	for i := range workers {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			got, err := prog.CallRecord(context.Background(), "make", map[string]any{"label": "w"}, risor.Map{"tenant": "conc"})
			if err != nil {
				errs <- err
				return
			}
			m, ok := got.(map[string]any)
			if !ok || m["tenant"] != "conc" || m["label"] != "w" {
				errs <- errors.New("unexpected concurrent result")
			}
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		assert.NoError(t, err)
	}
	assert.Equal(t, calls.Load(), int64(2+workers))
	assert.Equal(t, unusedCalls.Load(), int64(0))
}

func TestEntryCallCancellationAndExpectOptions(t *testing.T) {
	env, err := risor.NewEnv()
	assert.NoError(t, err)

	src := `export const loop = (n: int): int => n <= 0 ? 0 : 1 + loop(n - 1);`
	expr, err := risor.Parse(src)
	assert.NoError(t, err)

	// ExpectEntry, ExpectEntryType, ExpectEntrySchema.
	_, err = env.Check(expr, risor.ExpectEntry("loop", "int"))
	assert.NoError(t, err)

	_, err = env.Check(expr, risor.ExpectEntryType("loop", risor.TypeInt))
	assert.NoError(t, err)

	_, err = env.Check(expr, risor.ExpectEntrySchema("loop", map[string]any{"type": "integer"}))
	assert.NoError(t, err)

	_, err = env.Check(expr, risor.ExpectEntry("loop", "string"))
	var re *risor.Error
	assert.True(t, errors.As(err, &re))
	assert.Equal(t, re.Diagnostics[0].Code, "result-mismatch")

	// Condition on no-result program fails declare no-result.
	_, err = env.Check(expr, risor.Condition)
	assert.True(t, errors.As(err, &re))
	assert.Equal(t, re.Phase(), risor.PhaseDeclare)
	assert.Equal(t, re.Diagnostics[0].Code, "no-result")

	// Cancelled context fails run cancelled.
	prog, err := env.Compile(expr)
	assert.NoError(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, _, err = prog.CallMeasure(ctx, "loop", []any{5}, nil)
	assert.True(t, errors.As(err, &re))
	assert.Equal(t, re.Phase(), risor.PhaseRun)
	assert.Equal(t, re.Diagnostics[0].Code, "cancelled")

	// Run and Measure on no-result program fail declare no-result with 0 usage.
	_, u, err := prog.Measure(context.Background(), nil)
	assert.True(t, errors.As(err, &re))
	assert.Equal(t, re.Phase(), risor.PhaseDeclare)
	assert.Equal(t, re.Diagnostics[0].Code, "no-result")
	assert.Equal(t, u.Steps, int64(0))
	assert.Equal(t, u.Allocated, int64(0))
}

func TestEntryCallHostValueEdgeCases(t *testing.T) {
	env, err := risor.NewEnv()
	assert.NoError(t, err)

	// Host int, NaN on any parameter, and invalid UTF-8 string.
	expr, err := risor.Parse(`export const echo = (x: any) => x;`)
	assert.NoError(t, err)
	prog, err := env.Compile(expr)
	assert.NoError(t, err)

	// Go int works as Risor number (float64).
	v, err := prog.Call(context.Background(), "echo", []any{42}, nil)
	assert.NoError(t, err)
	assert.Equal(t, v, float64(42))

	// NaN on any passes conformance and fails number-range when read into Risor.
	_, _, err = prog.CallMeasure(context.Background(), "echo", []any{math.NaN()}, nil)
	var re *risor.Error
	assert.True(t, errors.As(err, &re))
	assert.Equal(t, re.Phase(), risor.PhaseRun)
	assert.Equal(t, re.Diagnostics[0].Code, "number-range")

	// NaN on float fails conformance with 0 steps and path [0].
	exprF, err := risor.Parse(`export const echoFloat = (x: float) => x;`)
	assert.NoError(t, err)
	progF, err := env.Compile(exprF)
	assert.NoError(t, err)
	_, u, err := progF.CallMeasure(context.Background(), "echoFloat", []any{math.NaN()}, nil)
	assert.True(t, errors.As(err, &re))
	assert.Equal(t, re.Phase(), risor.PhaseRun)
	assert.Equal(t, re.Diagnostics[0].Code, "not-conforming")
	assert.Equal(t, re.Diagnostics[0].Path, []any{0})
	assert.Equal(t, u.Steps, int64(0))
	assert.Equal(t, u.Allocated, int64(0))

	// Invalid UTF-8 bytes are replaced on read into Risor string.
	bad := "a\xffb"
	v, err = prog.Call(context.Background(), "echo", []any{bad}, nil)
	assert.NoError(t, err)
	assert.Equal(t, v, "a\ufffdb")
}
