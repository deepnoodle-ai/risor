package hostread

import (
	"context"
	"fmt"
	"reflect"
	"testing"

	"github.com/deepnoodle-ai/expr"
	"github.com/deepnoodle-ai/wonton/assert"
)

var (
	input = &Input{
		Role: "owner", Count: 1200,
		Event: Event{Action: "opened", PullRequest: &PullRequest{Number: 42, Head: Head{SHA: "0123456789abcdef", Ref: "fix"}}},
	}
	inputMap = map[string]any{
		"role": "owner", "count": int64(1200), "score": 0.75,
		"event": map[string]any{"action": "opened", "pull_request": map[string]any{
			"number": int64(42), "head": map[string]any{"sha": "0123456789abcdef", "ref": "fix"},
		}},
	}
	inputType = reflect.TypeOf(input)
)

var (
	sink    any
	sinkStr string
	sinkInt int64
	sinkOK  bool
)

func TestAgree(t *testing.T) {
	for _, read := range []func(any) any{
		byName("Role"), byPlan("Role"), byIndex(inputType, "Role"),
		boxString(byIndexString(inputType, "Role")), boxString(byOffsetString(inputType, "Role")),
	} {
		assert.Equal(t, read(input), "owner")
	}
	assert.Equal(t, byIndexInt(inputType, "Count")(input), int64(1200))
	assert.Equal(t, byOffsetInt(inputType, "Count")(input), int64(1200))
	assert.Equal(t, chainReflect(inputType)(input), "0123456789abcdef")
	assert.Equal(t, chainOffset(inputType)(input), "0123456789abcdef")
	assert.Equal(t, normalize(int32(7)), int64(7))

	names, boxed, users, maps := lists()
	long := func(v any) bool { return len(v.(string)) > 6 }
	longPtr := func(v any) bool { return len(*v.(*string)) > 6 }
	want := countAny(boxed, long)
	assert.Equal(t, countStrings(names, long), want)
	assert.Equal(t, countReflect(names, long), want)
	assert.Equal(t, countPointers(names, longPtr), want)
	adult := func(v any) bool { return v.(*User).Age > 500 }
	wantUsers := countAny(maps, func(v any) bool { return v.(map[string]any)["age"].(int64) > 500 })
	assert.Equal(t, countReflectPointers(users, adult), wantUsers)
	assert.Equal(t, countUnsafePointers(reflect.TypeOf(User{}))(users, adult), wantUsers)

	m := orderedMap(8)
	var buf []string
	assert.Equal(t, sumSorted(m, &buf), sumUnordered(m))
}

func lists() (names []string, boxed []any, users []User, maps []any) {
	for i := range 1000 {
		name := fmt.Sprintf("user_%d", i)
		names = append(names, name)
		boxed = append(boxed, name)
		users = append(users, User{Name: name, Age: i})
		maps = append(maps, map[string]any{"name": name, "age": int64(i)})
	}
	return
}

func orderedMap(n int) map[string]any {
	m := map[string]any{}
	for i := range n {
		m[fmt.Sprintf("key_%03d", i)] = int64(i)
	}
	return m
}

// BenchmarkField reads one field: input.Role, a string, and input.Count, an
// int above 255.
func BenchmarkField(b *testing.B) {
	boxed := []struct {
		name string
		v    any
		read func(any) any
	}{
		{"string/map", inputMap, mapField("role")},
		{"string/map_normalized", inputMap, mapFieldNormalized("role")},
		{"string/struct_by_name", input, byName("Role")},
		{"string/struct_by_plan", input, byPlan("Role")},
		{"string/struct_by_index", input, byIndex(inputType, "Role")},
		{"string/struct_by_offset_boxed", input, boxString(byOffsetString(inputType, "Role"))},
		{"int/map", inputMap, mapField("count")},
		{"int/map_normalized", inputMap, mapFieldNormalized("count")},
		{"int/map_normalized_range_checked", inputMap, mapFieldChecked("count")},
		{"float/map", inputMap, mapField("score")},
		{"float/map_normalized_range_checked", inputMap, mapFieldChecked("score")},
		{"int/struct_by_plan", input, byPlan("Count")},
		{"int/struct_by_index", input, byIndex(inputType, "Count")},
		{"int/struct_by_offset_boxed", input, boxInt(byOffsetInt(inputType, "Count"))},
	}
	for _, c := range boxed {
		b.Run(c.name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				sink = c.read(c.v)
			}
		})
	}
	for name, read := range map[string]func(any) string{
		"string/struct_by_index_typed":  byIndexString(inputType, "Role"),
		"string/struct_by_offset_typed": byOffsetString(inputType, "Role"),
	} {
		b.Run(name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				sinkStr = read(input)
			}
		})
	}
	for name, read := range map[string]func(any) int64{
		"int/struct_by_index_typed":  byIndexInt(inputType, "Count"),
		"int/struct_by_offset_typed": byOffsetInt(inputType, "Count"),
	} {
		b.Run(name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				sinkInt = read(input)
			}
		})
	}
}

// BenchmarkChain reads input.Event.PullRequest.Head.SHA.
func BenchmarkChain(b *testing.B) {
	mapChain := chain(mapField("event"), mapField("pull_request"), mapField("head"), mapField("sha"))
	b.Run("map", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			sink = mapChain(inputMap)
		}
	})
	for name, read := range map[string]func(any) string{
		"struct_reflect_fused": chainReflect(inputType),
		"struct_offset_fused":  chainOffset(inputType),
	} {
		b.Run(name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				sinkStr = read(input)
			}
		})
	}
	// expr, whole Run, for scale: it includes expr's per-run overhead.
	ctx := context.Background()
	for name, env := range map[string]any{
		"expr_run_struct": map[string]any{"input": input},
		"expr_run_map":    map[string]any{"input": inputMap},
	} {
		src := "input.Event.PullRequest.Head.SHA"
		if name == "expr_run_map" {
			src = "input.event.pull_request.head.sha"
		}
		p, err := expr.Compile(src)
		if err != nil {
			b.Fatal(err)
		}
		b.Run(name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				sink, _ = p.Run(ctx, env)
			}
		})
	}
}

// BenchmarkSlice counts the elements of a 1,000-element list that pass a
// predicate.
func BenchmarkSlice(b *testing.B) {
	names, boxed, users, maps := lists()
	long := func(v any) bool { return len(v.(string)) > 6 }
	longPtr := func(v any) bool { return len(*v.(*string)) > 6 }
	adult := func(v any) bool { return v.(*User).Age > 500 }
	adultMap := func(v any) bool { return v.(map[string]any)["age"].(int64) > 500 }
	for _, c := range []struct {
		name  string
		list  any
		count func(any, func(any) bool) int
		pred  func(any) bool
	}{
		{"strings/any_slice", boxed, countAny, long},
		{"strings/typed_boxing", names, countStrings, long},
		{"strings/reflect_boxing", names, countReflect, long},
		{"strings/typed_pointers", names, countPointers, longPtr},
		{"strings/reflect_pointers", names, countReflectPointers, longPtr},
		{"structs/any_slice_of_maps", maps, countAny, adultMap},
		{"structs/reflect_pointers", users, countReflectPointers, adult},
		{"structs/unsafe_pointers", users, countUnsafePointers(reflect.TypeOf(User{})), adult},
	} {
		b.Run(c.name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				sinkInt = int64(c.count(c.list, c.pred))
			}
		})
	}
}

// BenchmarkMapOrder visits every entry of a map, unordered and in key order.
func BenchmarkMapOrder(b *testing.B) {
	for _, n := range []int{8, 64, 512} {
		m := orderedMap(n)
		b.Run(fmt.Sprintf("unordered/%d", n), func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				sinkInt = sumUnordered(m)
			}
		})
		b.Run(fmt.Sprintf("sorted/%d", n), func(b *testing.B) {
			var buf []string
			b.ReportAllocs()
			for b.Loop() {
				sinkInt = sumSorted(m, &buf)
			}
		})
		b.Run(fmt.Sprintf("sorted_no_scratch/%d", n), func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				var buf []string
				sinkInt = sumSorted(m, &buf)
			}
		})
	}
}
