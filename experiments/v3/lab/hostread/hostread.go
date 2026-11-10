// Package hostread asks one question: what does it cost to read a host value
// in place when it is not a map[string]any or a []any?
//
// It compares ways to read a Go struct field (by name, through a cached
// plan, through an index resolved at compile time, and through an offset),
// ways to walk a typed slice, the price of normalizing a scalar on the way
// in, and the price of visiting a map in key order.
//
//	GOWORK=off go test ./lab/hostread -run xxx -bench . -benchmem -count 6 | go run ./lab/median
package hostread

import (
	"errors"
	"reflect"
	"slices"
	"sync"
	"unsafe"
)

var errType = errors.New("type error")

// The host's types. The path input.Event.PullRequest.Head.SHA crosses an
// embedded struct, a pointer and another embedded struct.
type Head struct{ SHA, Ref string }

type PullRequest struct {
	Number int
	Title  string
	Head   Head
}

type Event struct {
	Action      string
	PullRequest *PullRequest
}

type Input struct {
	Role  string
	Count int
	Event Event
}

type User struct {
	Name string
	Age  int
}

// ------------------------------------------------------------- one field

// byName looks the field up by name on every read.
func byName(name string) func(any) any {
	return func(v any) any {
		return reflect.ValueOf(v).Elem().FieldByName(name).Interface()
	}
}

// byPlan is what expr and Risor v2 do: a plan per struct type in a sync.Map,
// found on every read, then a map from name to index.
var plans sync.Map // reflect.Type -> map[string]int

func byPlan(name string) func(any) any {
	return func(v any) any {
		rv := reflect.ValueOf(v).Elem()
		t := rv.Type()
		p, ok := plans.Load(t)
		if !ok {
			fields := map[string]int{}
			for i := range t.NumField() {
				fields[t.Field(i).Name] = i
			}
			p, _ = plans.LoadOrStore(t, fields)
		}
		return rv.Field(p.(map[string]int)[name]).Interface()
	}
}

// byIndex resolves the field at compile time. The run checks that the value
// has the type the checker promised, then reads by index.
func byIndex(t reflect.Type, name string) func(any) any {
	f, _ := t.Elem().FieldByName(name)
	i := f.Index[0]
	return func(v any) any {
		rv := reflect.ValueOf(v)
		if rv.Type() != t {
			panic(errType)
		}
		return rv.Elem().Field(i).Interface()
	}
}

// byIndexString is byIndex with a typed result: nothing is boxed.
func byIndexString(t reflect.Type, name string) func(any) string {
	f, _ := t.Elem().FieldByName(name)
	i := f.Index[0]
	return func(v any) string {
		rv := reflect.ValueOf(v)
		if rv.Type() != t {
			panic(errType)
		}
		return rv.Elem().Field(i).String()
	}
}

func byIndexInt(t reflect.Type, name string) func(any) int64 {
	f, _ := t.Elem().FieldByName(name)
	i := f.Index[0]
	return func(v any) int64 {
		rv := reflect.ValueOf(v)
		if rv.Type() != t {
			panic(errType)
		}
		return rv.Elem().Field(i).Int()
	}
}

// pointerOf returns the pointer held by v after checking its type. It uses
// only documented API: reflect.Value.UnsafePointer.
func pointerOf(v any, t reflect.Type) unsafe.Pointer {
	rv := reflect.ValueOf(v)
	if rv.Type() != t {
		panic(errType)
	}
	return rv.UnsafePointer()
}

// byOffset resolves the field to a byte offset at compile time.
func byOffsetString(t reflect.Type, name string) func(any) string {
	f, _ := t.Elem().FieldByName(name)
	off := f.Offset
	return func(v any) string {
		return *(*string)(unsafe.Add(pointerOf(v, t), off))
	}
}

func byOffsetInt(t reflect.Type, name string) func(any) int64 {
	f, _ := t.Elem().FieldByName(name)
	off := f.Offset
	return func(v any) int64 {
		return int64(*(*int)(unsafe.Add(pointerOf(v, t), off)))
	}
}

// boxString and boxInt adapt a typed read for a consumer that wants an any.
func boxString(read func(any) string) func(any) any {
	return func(v any) any { return read(v) }
}

func boxInt(read func(any) int64) func(any) any {
	return func(v any) any { return read(v) }
}

func mapField(name string) func(any) any {
	return func(v any) any {
		if m, ok := v.(map[string]any); ok {
			if f, ok := m[name]; ok {
				return f
			}
		}
		panic(errType)
	}
}

// --------------------------------------------------------------- a chain

func chain(steps ...func(any) any) func(any) any {
	return func(v any) any {
		for _, s := range steps {
			v = s(v)
		}
		return v
	}
}

// chainReflect walks input.Event.PullRequest.Head.SHA in reflect.Values, so
// no struct on the way is copied or boxed. This is the shape of expr's
// evalSelectorChainRV, with the indices resolved at compile time.
func chainReflect(t reflect.Type) func(any) string {
	return func(v any) string {
		rv := reflect.ValueOf(v)
		if rv.Type() != t {
			panic(errType)
		}
		pr := rv.Elem().Field(2).Field(1) // Event.PullRequest
		if pr.IsNil() {
			panic(errType)
		}
		return pr.Elem().Field(2).Field(0).String() // Head.SHA
	}
}

// chainOffset is the same path as two additions and one pointer load. The
// offsets of embedded structs are summed at compile time.
func chainOffset(t reflect.Type) func(any) string {
	var in Input
	var pr PullRequest
	off1 := unsafe.Offsetof(in.Event) + unsafe.Offsetof(in.Event.PullRequest)
	off2 := unsafe.Offsetof(pr.Head) + unsafe.Offsetof(pr.Head.SHA)
	return func(v any) string {
		p := *(**PullRequest)(unsafe.Add(pointerOf(v, t), off1))
		if p == nil {
			panic(errType)
		}
		return *(*string)(unsafe.Add(unsafe.Pointer(p), off2))
	}
}

// ---------------------------------------------------------- typed slices

// countAny is the baseline: a []any of strings, as a JSON decoder makes.
func countAny(list any, pred func(any) bool) int {
	n := 0
	for _, item := range list.([]any) {
		if pred(item) {
			n++
		}
	}
	return n
}

// countStrings takes the []string fast path and boxes each element.
func countStrings(list any, pred func(any) bool) int {
	n := 0
	for _, item := range list.([]string) {
		if pred(item) {
			n++
		}
	}
	return n
}

// countReflect walks any slice through reflection.
func countReflect(list any, pred func(any) bool) int {
	rv := reflect.ValueOf(list)
	n := 0
	for i := range rv.Len() {
		if pred(rv.Index(i).Interface()) {
			n++
		}
	}
	return n
}

// countPointers passes a pointer to each element. A pointer in an any does
// not allocate, and the predicate reads through it.
func countPointers(list any, pred func(any) bool) int {
	items := list.([]string)
	n := 0
	for i := range items {
		if pred(&items[i]) {
			n++
		}
	}
	return n
}

// countReflectPointers is countPointers for a slice type known only at run
// time, such as []User.
func countReflectPointers(list any, pred func(any) bool) int {
	rv := reflect.ValueOf(list)
	n := 0
	for i := range rv.Len() {
		if pred(rv.Index(i).Addr().Interface()) {
			n++
		}
	}
	return n
}

// countUnsafePointers steps through the slice's memory by element size.
func countUnsafePointers(elem reflect.Type) func(list any, pred func(any) bool) int {
	size := elem.Size()
	return func(list any, pred func(any) bool) int {
		rv := reflect.ValueOf(list)
		base, length := rv.UnsafePointer(), rv.Len()
		n := 0
		for i := range length {
			p := reflect.NewAt(elem, unsafe.Add(base, uintptr(i)*size))
			if pred(p.Interface()) {
				n++
			}
		}
		return n
	}
}

// ------------------------------------------------------ the read barrier

// normalize is the check a value passes when it leaves a host container. The
// seven shapes the evaluator works in pass through. Anything else takes the
// slow path, which widens Go's other numeric types.
func normalize(v any) any {
	switch v.(type) {
	case nil, string, bool, float64, int64, map[string]any, []any:
		return v
	}
	return normalizeSlow(v)
}

func normalizeSlow(v any) any {
	switch v := v.(type) {
	case int:
		return int64(v)
	case int32:
		return int64(v)
	case float32:
		return float64(v)
	}
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return rv.Int()
	case reflect.String:
		return rv.String()
	}
	return v
}

// normalizeChecked is normalize with the number rules: an int64 outside
// ±(2^53 - 1) and a float64 that is not finite cannot enter the language.
func normalizeChecked(v any) any {
	switch n := v.(type) {
	case nil, string, bool, map[string]any, []any:
		return v
	case int64:
		if uint64(n+maxSafe) > 2*maxSafe {
			panic(errType)
		}
		return v
	case float64:
		if n-n != 0 { // NaN or an infinity
			panic(errType)
		}
		return v
	}
	return normalizeSlow(v)
}

const maxSafe = 1<<53 - 1

func mapFieldChecked(name string) func(any) any {
	return func(v any) any {
		if m, ok := v.(map[string]any); ok {
			if f, ok := m[name]; ok {
				return normalizeChecked(f)
			}
		}
		panic(errType)
	}
}

func mapFieldNormalized(name string) func(any) any {
	return func(v any) any {
		if m, ok := v.(map[string]any); ok {
			if f, ok := m[name]; ok {
				return normalize(f)
			}
		}
		panic(errType)
	}
}

// ------------------------------------------------------------- map order

func sumUnordered(m map[string]any) int64 {
	var n int64
	for _, v := range m {
		n += v.(int64)
	}
	return n
}

// sumSorted visits the map in key order. buf is the run's scratch space.
func sumSorted(m map[string]any, buf *[]string) int64 {
	keys := (*buf)[:0]
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	var n int64
	for _, k := range keys {
		n += m[k].(int64)
	}
	*buf = keys
	return n
}
