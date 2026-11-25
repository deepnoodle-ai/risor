package risor_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"testing"
	"time"

	risor "github.com/deepnoodle-ai/risor/go/v3"
	"github.com/deepnoodle-ai/wonton/assert"
)

// Adversarial regression tests for limits, the host boundary, nested runs with
// Remaining(), host data conformance, and editor tools (issue #704).

func risorErrCode(t *testing.T, err error) string {
	t.Helper()
	assert.Error(t, err)
	var re *risor.Error
	assert.True(t, errors.As(err, &re))
	assert.True(t, len(re.Diagnostics) > 0)
	return re.Diagnostics[0].Code
}

type uncomparableOpaqueBox struct {
	Data any
}

func (uncomparableOpaqueBox) OpaqueType() string { return "Box" }

func TestAdversarialLimitsAndOOMBeforeCharge(t *testing.T) {
	ctx := context.Background()
	env, err := risor.NewEnv(
		risor.Groups("time", "json", "base64", "math"),
		risor.Var("s", "string"),
		risor.Var("xs", "any[]"),
	)
	assert.NoError(t, err)

	// Exponential / huge arguments fail at step or range limits.
	for _, tc := range []struct {
		src  string
		code string
	}{
		{`"a".repeat(1000000000000000)`, "steps-exceeded"},
		{`range(1000000000000000)`, "steps-exceeded"},
		{`2 ** 10000`, "number-range"},
	} {
		ast, err := risor.Parse(tc.src)
		assert.NoError(t, err)
		p, err := env.Compile(ast)
		assert.NoError(t, err)
		_, err = p.Run(ctx, risor.Map{"s": "", "xs": []any{}})
		assert.Equal(t, risorErrCode(t, err), tc.code)
	}

	// OOM-before-charge: list methods, json.parse, and padStart with tiny allocation limit.
	tiny, err := env.With(risor.Limits{Steps: 10_000_000, Allocation: 5})
	assert.NoError(t, err)
	bigList := make([]any, 100_000)
	for i := range bigList {
		bigList[i] = float64(i)
	}
	for _, expr := range []string{
		`xs.map(x => x)`,
		`xs.sortBy(x => x)`,
	} {
		ast, err := risor.Parse(expr)
		assert.NoError(t, err)
		p, err := tiny.Compile(ast)
		assert.NoError(t, err)
		_, err = p.Run(ctx, risor.Map{"s": "", "xs": bigList})
		assert.Equal(t, risorErrCode(t, err), "allocation-exceeded")
	}

	// padStart with huge fill string bounds work to min(len(fill), m) and rejects allocation-exceeded.
	astPad, err := risor.Parse(`"x".padStart(100000, s)`)
	assert.NoError(t, err)
	pPad, err := tiny.Compile(astPad)
	assert.NoError(t, err)
	_, err = pPad.Run(ctx, risor.Map{"s": strings.Repeat("a", 100_000), "xs": []any{}})
	assert.Equal(t, risorErrCode(t, err), "allocation-exceeded")

	// padStart with small missing count and huge fill string succeeds fast without reading all of fill.
	astPadSmall, err := risor.Parse(`"x".padStart(4, s)`)
	assert.NoError(t, err)
	pPadSmall, err := env.Compile(astPadSmall)
	assert.NoError(t, err)
	v, err := pPadSmall.Run(ctx, risor.Map{"s": "abc" + strings.Repeat("z", 500_000), "xs": []any{}})
	assert.NoError(t, err)
	assert.Equal(t, v, "abcx")

	// time.duration stays linear on many terms with one long fraction.
	astDur, err := risor.Parse(`time.duration(s)`)
	assert.NoError(t, err)
	pDur, err := env.Compile(astDur)
	assert.NoError(t, err)
	dur := "0." + strings.Repeat("0", 10_000) + "1s" + strings.Repeat("1s", 10_000)
	t0 := time.Now()
	dv, err := pDur.Run(ctx, risor.Map{"s": dur, "xs": []any{}})
	assert.NoError(t, err)
	assert.Equal(t, dv, 10_000.0)
	assert.True(t, time.Since(t0) < 2*time.Second)
}

func TestAdversarialSourceSizeNestingCheckerDepthAndDeadline(t *testing.T) {
	ctx := context.Background()
	env, err := risor.NewEnv(risor.Var("a", "int"))
	assert.NoError(t, err)

	big := "1" + strings.Repeat(" ", 1_048_576)
	_, err = risor.Parse(big)
	assert.Equal(t, risorErrCode(t, err), "source-too-large")
	_, err = risor.ParseTemplate(big)
	assert.Equal(t, risorErrCode(t, err), "source-too-large")
	_, err = risor.Format(big)
	assert.Equal(t, risorErrCode(t, err), "source-too-large")
	_, err = risor.FormatTemplate(big)
	assert.Equal(t, risorErrCode(t, err), "source-too-large")
	assert.Equal(t, len(env.Complete(big, risor.Pos{Line: 1, Column: 2}).Items), 0)
	assert.True(t, env.Hover(big, risor.Pos{Line: 1, Column: 2}) == nil)

	_, err = risor.Parse(strings.Repeat("(", 500))
	assert.Equal(t, risorErrCode(t, err), "nesting-exceeded")

	// 25-level DAG record type hits check-limit.
	var dag strings.Builder
	dag.WriteString("const a0 = {x: 1, y: 1};\n")
	for i := 1; i <= 25; i++ {
		fmt.Fprintf(&dag, "const a%d = {x: a%d, y: a%d};\n", i, i-1, i-1)
	}
	dag.WriteString("a25")
	astDag, err := risor.Parse(dag.String())
	assert.NoError(t, err)
	_, err = env.Compile(astDag)
	assert.Equal(t, risorErrCode(t, err), "check-limit")

	// Depth limit across recursion and walk.
	recEnv, err := risor.NewEnv(risor.Limits{Depth: 20})
	assert.NoError(t, err)
	for _, src := range []string{
		`const f = (n: int): int => n <= 0 ? 0 : f(n - 1); f(100)`,
		`walk(0, (n, d) => d < 50 ? [n + 1] : null)`,
	} {
		ast, err := risor.Parse(src)
		assert.NoError(t, err)
		p, err := recEnv.Compile(ast)
		assert.NoError(t, err)
		_, err = p.Run(ctx, nil)
		assert.Equal(t, risorErrCode(t, err), "depth-exceeded")
	}

	// Deadline and context cancellation stop tight step loops promptly.
	dlEnv, err := risor.NewEnv(risor.Limits{Steps: risor.NoLimit, Allocation: risor.NoLimit, Deadline: 25 * time.Millisecond})
	assert.NoError(t, err)
	astLoop, err := risor.Parse(`const xs = range(10000); xs.map(x => xs.map(y => x + y))`)
	assert.NoError(t, err)
	pLoop, err := dlEnv.Compile(astLoop)
	assert.NoError(t, err)
	t0 := time.Now()
	_, err = pLoop.Run(ctx, nil)
	assert.Equal(t, risorErrCode(t, err), "deadline-exceeded")
	assert.True(t, time.Since(t0) < 2*time.Second)

	noDL, err := risor.NewEnv(risor.Limits{Steps: risor.NoLimit, Allocation: risor.NoLimit})
	assert.NoError(t, err)
	pCancel, err := noDL.Compile(astLoop)
	assert.NoError(t, err)
	cctx, cancel := context.WithCancel(ctx)
	cancel()
	_, err = pCancel.Run(cctx, nil)
	assert.Equal(t, risorErrCode(t, err), "cancelled")
}

func TestAdversarialOpaqueAndHostBoundary(t *testing.T) {
	ctx := context.Background()

	// Regression: SameOpaque on a comparable struct type holding an uncomparable
	// slice inside an interface field must return false, never panic / internal-error.
	oEnv, err := risor.NewEnv(
		risor.OpaqueType{Name: "Box"},
		risor.Var("a", "Box"),
		risor.Var("b", "Box"),
	)
	assert.NoError(t, err)
	astEq, err := risor.Parse(`a == b`)
	assert.NoError(t, err)
	pEq, err := oEnv.Compile(astEq)
	assert.NoError(t, err)
	got, err := pEq.Run(ctx, risor.Map{
		"a": uncomparableOpaqueBox{Data: []int{1, 2}},
		"b": uncomparableOpaqueBox{Data: []int{1, 2}},
	})
	assert.NoError(t, err)
	assert.Equal(t, got, false)

	// Regression: cyclic Go pointer (var cyc any; cyc = &cyc) must not stack-overflow
	// in value.KindOf / value.Read.
	var cyc any
	cyc = &cyc
	cEnv, err := risor.NewEnv(risor.Var("x", "any"))
	assert.NoError(t, err)
	astX, err := risor.Parse(`type(x)`)
	assert.NoError(t, err)
	pX, err := cEnv.Compile(astX)
	assert.NoError(t, err)
	kindStr, err := pX.Run(ctx, risor.Map{"x": cyc})
	assert.NoError(t, err)
	assert.Equal(t, kindStr, "opaque")

	// Host function panic is uncatchable and nested RisorError cause depth caps at 8 in JSON.
	pEnv, err := risor.NewEnv(risor.Function{
		Name: "crash",
		Type: "() => int !",
		Impl: func(*risor.Call, []any) (any, error) { panic("boom") },
	})
	assert.NoError(t, err)
	astTry, err := risor.Parse(`try { crash() } catch { 0 }`)
	assert.NoError(t, err)
	pTry, err := pEnv.Compile(astTry)
	assert.NoError(t, err)
	_, err = pTry.Run(ctx, nil)
	assert.Equal(t, risorErrCode(t, err), "function-panic")

	// 12 nested host calls: JSON rendering caps cause chain at depth 8.
	inner, err := risor.NewEnv(risor.Function{
		Name: "leaf",
		Type: "() => int !",
		Impl: func(*risor.Call, []any) (any, error) { return nil, errors.New("leaf-err") },
	})
	assert.NoError(t, err)
	for i := 0; i < 12; i++ {
		nextEnv := inner
		callSrc := "leaf()"
		if i > 0 {
			callSrc = fmt.Sprintf("c%d()", i-1)
		}
		callAST, err := risor.Parse(callSrc)
		assert.NoError(t, err)
		inner, err = risor.NewEnv(risor.Function{
			Name: fmt.Sprintf("c%d", i),
			Type: "() => int !",
			Impl: func(c *risor.Call, _ []any) (any, error) {
				child, err := nextEnv.With(risor.Budget(c.Remaining()))
				if err != nil {
					return nil, err
				}
				p, err := child.Compile(callAST)
				if err != nil {
					return nil, err
				}
				return p.Run(c.Context(), nil)
			},
		})
		assert.NoError(t, err)
	}
	astTop, err := risor.Parse("c11()")
	assert.NoError(t, err)
	pTop, err := inner.Compile(astTop)
	assert.NoError(t, err)
	_, err = pTop.Run(ctx, nil)
	var re *risor.Error
	assert.True(t, errors.As(err, &re))
	var d map[string]any
	assert.NoError(t, json.Unmarshal([]byte(risor.RenderJSON(re.Diagnostics)), &d))
	depth := 0
	for {
		c, ok := d["cause"].(map[string]any)
		if !ok {
			break
		}
		depth++
		d = c
	}
	assert.Equal(t, depth, 8)
}

func TestAdversarialRemainingAndHostConformance(t *testing.T) {
	ctx := context.Background()

	// Nested runs with Remaining() cannot amplify step budget even across try/catch.
	childEnv, err := risor.NewEnv()
	assert.NoError(t, err)
	childAST, err := risor.Parse(`range(20).map(x => x + 1)`)
	assert.NoError(t, err)
	parent, err := risor.NewEnv(
		risor.Limits{Steps: 100, Allocation: 100},
		risor.Function{
			Name: "sub",
			Type: "() => int !",
			Impl: func(c *risor.Call, _ []any) (any, error) {
				child, err := childEnv.With(risor.Budget(c.Remaining()))
				if err != nil {
					return nil, err
				}
				p, err := child.Compile(childAST)
				if err != nil {
					return nil, err
				}
				_, u, runErr := p.Measure(c.Context(), nil)
				if err := c.Charge(u.Steps); err != nil {
					return nil, err
				}
				if err := c.Allocate(u.Allocated); err != nil {
					return nil, err
				}
				var re *risor.Error
				if errors.As(runErr, &re) && len(re.Diagnostics) > 0 {
					switch re.Diagnostics[0].Code {
					case "steps-exceeded":
						return nil, c.Charge(math.MaxInt64)
					case "allocation-exceeded":
						return nil, c.Allocate(math.MaxInt64)
					}
				}
				return int64(1), runErr
			},
		},
	)
	assert.NoError(t, err)
	astRun, err := risor.Parse(`try { range(10).map(_ => sub()) } catch { [] }`)
	assert.NoError(t, err)
	pRun, err := parent.Compile(astRun)
	assert.NoError(t, err)
	_, err = pRun.Run(ctx, nil)
	assert.Equal(t, risorErrCode(t, err), "steps-exceeded")

	// Regression: ConformAll on a tagged union with a bad host number (NaN / MaxInt64)
	// in the tag field must report "found number", matching Conform.
	ut := risor.TaggedUnionType(
		risor.ExactRecordType(risor.FieldType{Key: "kind", Type: risor.LiteralType("a")}),
		risor.ExactRecordType(risor.FieldType{Key: "kind", Type: risor.LiteralType("b")}),
	)
	for _, bad := range []any{math.NaN(), math.Inf(1), int64(math.MaxInt64)} {
		v := map[string]any{"kind": bad}
		cErr := risor.Conform(v, ut)
		assert.Error(t, cErr)
		var ce *risor.Error
		assert.True(t, errors.As(cErr, &ce))
		all := risor.ConformAll(v, ut, 10)
		assert.Equal(t, len(all), 1)
		assert.Equal(t, all[0].Code, "not-conforming")
		assert.Equal(t, all[0].Message, ce.Diagnostics[0].Message)
		assert.Contains(t, all[0].Message, "found number")
	}

	// Cyclic map hits depth-exceeded in JSONText and in recursive type conformance.
	cycMap := map[string]any{}
	cycMap["self"] = cycMap
	_, err = risor.JSONText(cycMap)
	assert.Equal(t, risorErrCode(t, err), "depth-exceeded")

	recEnv, err := risor.NewEnv(risor.NamedType("Node", "{ self: Node }"), risor.Var("x", "any"))
	assert.NoError(t, err)
	astAs, err := risor.Parse(`x as Node`)
	assert.NoError(t, err)
	pAs, err := recEnv.Compile(astAs)
	assert.NoError(t, err)
	_, err = pAs.Run(ctx, risor.Map{"x": cycMap})
	assert.Equal(t, risorErrCode(t, err), "depth-exceeded")

	// Invalid UTF-8 host string replaces each maximal subpart with U+FFFD (03 3.4.5).
	strEnv, err := risor.NewEnv(risor.Var("s", "string"))
	assert.NoError(t, err)
	astS, err := risor.Parse(`s`)
	assert.NoError(t, err)
	pS, err := strEnv.Compile(astS)
	assert.NoError(t, err)
	sv, err := pS.Run(ctx, risor.Map{"s": string([]byte{'a', 0xe2, 0x82, 'A'})})
	assert.NoError(t, err)
	assert.Equal(t, sv, "a\uFFFDA")
}

func TestAdversarialEditorToolsOnHostileText(t *testing.T) {
	env, err := risor.NewEnv(risor.Var("a", "int"), risor.Var("s", "string"))
	assert.NoError(t, err)

	hostile := []string{
		strings.Repeat("(", 5_000),
		strings.Repeat("{", 5_000),
		strings.Repeat("[", 5_000),
		strings.Repeat("const a = ; ", 5_000),
		strings.Repeat("a + ", 10_000),
		strings.Repeat("if (1) { ", 500),
		"\"\"\"\n" + strings.Repeat("a", 20_000),
		strings.Repeat("/*\n", 5_000),
		"const a = 1;\x00\x00\x00a.",
		string([]byte{'c', 'o', 'n', 's', 't', ' ', 'a', ' ', '=', ' ', 0xff, 0xfe, 0xfd, ' ', 'a', '.'}),
	}
	for _, text := range hostile {
		t0 := time.Now()
		_ = env.Complete(text, risor.Pos{Line: 1, Column: len(text) + 1})
		_ = env.Hover(text, risor.Pos{Line: 1, Column: 2})
		_, _ = risor.Format(text)
		assert.True(t, time.Since(t0) < 2*time.Second)
	}
}
