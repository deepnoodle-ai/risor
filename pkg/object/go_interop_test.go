package object

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/deepnoodle-ai/risor/v2/pkg/bytecode"
	"github.com/deepnoodle-ai/wonton/assert"
)

// These tests cover script values that cross into Go: null for interface
// targets, values without a Go form, defined types, and wrapped Go values
// passed back to Go. None of them may cause a reflect panic.

func callGo(t *testing.T, fn any, args ...Object) (Object, error) {
	t.Helper()
	return NewGoFunc(reflect.ValueOf(fn), "fn", DefaultRegistry()).Call(context.Background(), args...)
}

// noPanic fails the test if err comes from GoFunc's panic recovery.
func noPanic(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		assert.False(t, strings.Contains(err.Error(), "panic in"), err.Error())
	}
}

// =============================================================================
// NULL FOR INTERFACE TARGETS
// =============================================================================

func TestGoInterop_NullToAnyParameter(t *testing.T) {
	result, err := callGo(t, func(v any) bool { return v == nil }, Nil)
	assert.Nil(t, err)
	assert.Equal(t, result, True)
}

func TestGoInterop_NullToErrorParameter(t *testing.T) {
	result, err := callGo(t, func(e error) bool { return e == nil }, Nil)
	assert.Nil(t, err)
	assert.Equal(t, result, True)
}

func TestGoInterop_NullToStringerParameter(t *testing.T) {
	result, err := callGo(t, func(s fmt.Stringer) bool { return s == nil }, Nil)
	assert.Nil(t, err)
	assert.Equal(t, result, True)
}

func TestGoInterop_NullToVariadicAny(t *testing.T) {
	fn := func(args ...any) int {
		n := 0
		for _, a := range args {
			if a == nil {
				n++
			}
		}
		return n
	}
	result, err := callGo(t, fn, NewInt(1), Nil, Nil)
	assert.Nil(t, err)
	assert.Equal(t, result, NewInt(2))
}

func TestGoInterop_NullInAnySlice(t *testing.T) {
	ctx := context.Background()
	list := NewList([]Object{NewInt(1), Nil})

	v, err := DefaultRegistry().ToGo(ctx, list, reflect.TypeOf([]any{}))
	assert.Nil(t, err)
	assert.Equal(t, v, []any{int64(1), nil})

	v, err = DefaultRegistry().ToGo(ctx, list, reflect.TypeOf([2]any{}))
	assert.Nil(t, err)
	assert.Equal(t, v, [2]any{int64(1), nil})
}

func TestGoInterop_NullMapValueKeepsKey(t *testing.T) {
	m := NewMap(map[string]Object{"a": Nil, "b": NewInt(2)})
	v, err := DefaultRegistry().ToGo(context.Background(), m, reflect.TypeOf(map[string]any{}))
	assert.Nil(t, err)
	got := v.(map[string]any)
	assert.Len(t, got, 2)
	value, ok := got["a"]
	assert.True(t, ok)
	assert.Nil(t, value)
}

func TestGoInterop_NullToAnyField(t *testing.T) {
	type holder struct{ V any }
	h := &holder{V: 1}
	s := NewGoStruct(reflect.ValueOf(h), DefaultRegistry())
	assert.Nil(t, s.SetAttr("V", Nil))
	assert.Nil(t, h.V)
}

// =============================================================================
// VALUES WITHOUT A GO FORM
// =============================================================================

func TestGoInterop_NoGoValueToAny(t *testing.T) {
	builtin := NewBuiltin("b", func(ctx context.Context, args ...Object) (Object, error) {
		return Nil, nil
	})
	values := map[string]Object{
		"closure": NewClosure(bytecode.NewFunction(bytecode.FunctionParams{})),
		"builtin": builtin,
		"module":  NewBuiltinsModule("m", map[string]Object{}),
	}
	for name, value := range values {
		t.Run(name, func(t *testing.T) {
			_, err := callGo(t, func(v any) {}, value)
			assert.NotNil(t, err)
			noPanic(t, err)
			assert.Contains(t, err.Error(), "cannot convert "+string(value.Type()))

			_, err = callGo(t, func(v ...any) {}, value)
			assert.NotNil(t, err)
			noPanic(t, err)

			_, err = DefaultRegistry().ToGo(context.Background(),
				NewList([]Object{value}), reflect.TypeOf([]any{}))
			assert.NotNil(t, err)

			type holder struct{ V any }
			s := NewGoStruct(reflect.ValueOf(&holder{}), DefaultRegistry())
			assert.NotNil(t, s.SetAttr("V", value))
		})
	}
}

// A value without a Go form nested in a list or map is the same error when
// the target is a plain any.
func TestGoInterop_NestedNoGoValueToAny(t *testing.T) {
	closure := NewClosure(bytecode.NewFunction(bytecode.FunctionParams{}))
	values := []Object{
		NewList([]Object{NewInt(1), closure}),
		NewMap(map[string]Object{"f": closure}),
		NewList([]Object{NewMap(map[string]Object{"f": closure})}),
	}
	for _, value := range values {
		_, err := callGo(t, func(v any) {}, value)
		assert.NotNil(t, err)
		noPanic(t, err)
		assert.Contains(t, err.Error(), "cannot convert function to a Go value")
	}

	// Lists and maps of plain values still convert as before
	result, err := DefaultRegistry().ToGo(context.Background(),
		NewList([]Object{NewInt(1), NewMap(map[string]Object{"a": Nil})}),
		reflect.TypeOf((*any)(nil)).Elem())
	assert.Nil(t, err)
	assert.Equal(t, result, []any{int64(1), map[string]any{"a": nil}})
}

// A registered converter that returns the wrong type gives an error, not a
// panic.
func TestGoInterop_ConverterWrongType(t *testing.T) {
	type point struct{ X int }
	registry := NewRegistryBuilder().
		RegisterToGo(reflect.TypeOf(point{}), func(obj Object, _ reflect.Type) (any, error) {
			return "not a point", nil
		}).
		Build()
	fn := NewGoFunc(reflect.ValueOf(func(p point) {}), "fn", registry)
	_, err := fn.Call(context.Background(), NewInt(1))
	assert.NotNil(t, err)
	noPanic(t, err)
}

// =============================================================================
// DEFINED TYPES
// =============================================================================

type (
	testRole  string
	testFlag  bool
	testCount int
	testRatio float32
)

func TestGoInterop_DefinedPrimitiveTypes(t *testing.T) {
	result, err := callGo(t, func(d time.Duration) string { return d.String() }, NewInt(int64(time.Second)))
	assert.Nil(t, err)
	assert.Equal(t, result, NewString("1s"))

	result, err = callGo(t, func(r testRole) string { return string(r) }, NewString("admin"))
	assert.Nil(t, err)
	assert.Equal(t, result, NewString("admin"))

	result, err = callGo(t, func(f testFlag) bool { return bool(f) }, True)
	assert.Nil(t, err)
	assert.Equal(t, result, True)

	result, err = callGo(t, func(c testCount) int { return int(c) }, NewFloat(3))
	assert.Nil(t, err)
	assert.Equal(t, result, NewInt(3))

	result, err = callGo(t, func(r testRatio) float64 { return float64(r) }, NewInt(2))
	assert.Nil(t, err)
	assert.Equal(t, result, NewFloat(2))
}

func TestGoInterop_DefinedTypesInContainers(t *testing.T) {
	ctx := context.Background()
	r := DefaultRegistry()

	v, err := r.ToGo(ctx, NewList([]Object{NewString("a"), NewString("b")}), reflect.TypeOf([]testRole{}))
	assert.Nil(t, err)
	assert.Equal(t, v, []testRole{"a", "b"})

	m := NewMap(map[string]Object{"admin": NewInt(1)})
	v, err = r.ToGo(ctx, m, reflect.TypeOf(map[testRole]testCount{}))
	assert.Nil(t, err)
	assert.Equal(t, v, map[testRole]testCount{"admin": 1})

	v, err = r.ToGo(ctx, NewString(`{"a":1}`), reflect.TypeOf(json.RawMessage{}))
	assert.Nil(t, err)
	assert.Equal(t, v, json.RawMessage(`{"a":1}`))
}

func TestGoInterop_DefinedTypeField(t *testing.T) {
	type config struct {
		Timeout time.Duration
		Role    testRole
	}
	c := &config{}
	s := NewGoStruct(reflect.ValueOf(c), DefaultRegistry())
	assert.Nil(t, s.SetAttr("Timeout", NewInt(int64(time.Minute))))
	assert.Nil(t, s.SetAttr("Role", NewString("admin")))
	assert.Equal(t, c.Timeout, time.Minute)
	assert.Equal(t, c.Role, testRole("admin"))
}

// =============================================================================
// WRAPPED GO VALUES PASSED BACK TO GO
// =============================================================================

type testStringerStruct struct{ Name string }

func (s *testStringerStruct) String() string { return "stringer " + s.Name }

func TestGoInterop_GoStructRoundTrip(t *testing.T) {
	orig := &testStringerStruct{Name: "a"}
	s := NewGoStruct(reflect.ValueOf(orig), DefaultRegistry())

	result, err := callGo(t, func(p *testStringerStruct) bool { return p == orig }, s)
	assert.Nil(t, err)
	assert.Equal(t, result, True)

	result, err = callGo(t, func(p testStringerStruct) string { return p.Name }, s)
	assert.Nil(t, err)
	assert.Equal(t, result, NewString("a"))

	result, err = callGo(t, func(p fmt.Stringer) string { return p.String() }, s)
	assert.Nil(t, err)
	assert.Equal(t, result, NewString("stringer a"))

	result, err = callGo(t, func(p any) bool { return p == orig }, s)
	assert.Nil(t, err)
	assert.Equal(t, result, True)

	_, err = callGo(t, func(p *testPerson) {}, s)
	assert.NotNil(t, err)
	noPanic(t, err)

	_, err = callGo(t, func(p testPerson) {}, s)
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "cannot use *object.testStringerStruct as object.testPerson")
}

func TestGoInterop_GoStructToField(t *testing.T) {
	type holder struct {
		Ptr *testStringerStruct
		Val testStringerStruct
	}
	orig := &testStringerStruct{Name: "a"}
	h := &holder{}
	hs := NewGoStruct(reflect.ValueOf(h), DefaultRegistry())
	s := NewGoStruct(reflect.ValueOf(orig), DefaultRegistry())

	assert.Nil(t, hs.SetAttr("Ptr", s))
	assert.Nil(t, hs.SetAttr("Val", s))
	assert.True(t, h.Ptr == orig)
	assert.Equal(t, h.Val.Name, "a")
}

func TestGoInterop_GoFuncRoundTrip(t *testing.T) {
	double := NewGoFunc(reflect.ValueOf(func(x int) int { return x * 2 }), "double", DefaultRegistry())
	result, err := callGo(t, func(f func(int) int) int { return f(21) }, double)
	assert.Nil(t, err)
	assert.Equal(t, result, NewInt(42))
}

// =============================================================================
// GOFUNC AND GOSTRUCT EDGE CASES
// =============================================================================

type testErrno int

func (e testErrno) Error() string { return fmt.Sprintf("errno %d", int(e)) }

func TestGoInterop_ValueTypeError(t *testing.T) {
	result, err := callGo(t, func(n int) testErrno { return testErrno(n) }, NewInt(0))
	assert.Nil(t, err)
	assert.Equal(t, result, Nil)

	_, err = callGo(t, func(n int) testErrno { return testErrno(n) }, NewInt(5))
	assert.NotNil(t, err)
	assert.Equal(t, err.Error(), "errno 5")
}

func TestGoInterop_BoundMethodEquality(t *testing.T) {
	a := NewGoStruct(reflect.ValueOf(&testPerson{Name: "a"}), DefaultRegistry())
	b := NewGoStruct(reflect.ValueOf(&testPerson{Name: "b"}), DefaultRegistry())

	aGreet, _ := a.GetAttr("Greet")
	aGreet2, _ := a.GetAttr("Greet")
	bGreet, _ := b.GetAttr("Greet")
	aSetName, ok := a.GetAttr("SetName")
	assert.True(t, ok)

	assert.True(t, aGreet.Equals(aGreet2))
	assert.False(t, aGreet.Equals(bGreet))
	assert.False(t, aGreet.Equals(aSetName))

	plain := NewGoFunc(reflect.ValueOf(func() {}), "fn", DefaultRegistry())
	assert.False(t, aGreet.Equals(plain))
	assert.False(t, plain.Equals(aGreet))
}

func TestGoInterop_NestedValueStructWrites(t *testing.T) {
	emp := &testEmployee{Name: "Alice", Address: testAddress{City: "Springfield"}}
	s := NewGoStruct(reflect.ValueOf(emp), DefaultRegistry())

	addr, ok := s.GetAttr("Address")
	assert.True(t, ok)
	assert.Nil(t, addr.(*GoStruct).SetAttr("City", NewString("Shelbyville")))
	assert.Equal(t, emp.Address.City, "Shelbyville")

	// Two reads of the field are the same struct
	addr2, _ := s.GetAttr("Address")
	assert.True(t, addr.Equals(addr2))
}

// A struct field at offset 0 shares its address with the enclosing struct,
// but they are not equal.
func TestGoInterop_FirstFieldNotEqualToParent(t *testing.T) {
	type outer struct{ Inner testAddress }
	s := NewGoStruct(reflect.ValueOf(&outer{}), DefaultRegistry())
	inner, _ := s.GetAttr("Inner")
	assert.False(t, s.Equals(inner))
	assert.False(t, inner.Equals(s))
}

// time.Time values convert to Time objects, including struct fields. A
// *time.Time keeps its pointer identity as a GoStruct.
func TestGoInterop_TimeConversion(t *testing.T) {
	now := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)

	obj, err := DefaultRegistry().FromGo(now)
	assert.Nil(t, err)
	assert.Equal(t, obj.Type(), TIME)

	type event struct{ At time.Time }
	s := NewGoStruct(reflect.ValueOf(&event{At: now}), DefaultRegistry())
	at, ok := s.GetAttr("At")
	assert.True(t, ok)
	assert.Equal(t, at.Type(), TIME)

	obj, err = DefaultRegistry().FromGo(&now)
	assert.Nil(t, err)
	assert.Equal(t, obj.Type(), GOSTRUCT)

	// The GoStruct passes back to a time.Time parameter
	result, err := callGo(t, func(t time.Time) int { return t.Year() }, obj)
	assert.Nil(t, err)
	assert.Equal(t, result, NewInt(2026))
}

// =============================================================================
// SYNC TYPES
// =============================================================================

type testGuarded struct {
	Mu    sync.Mutex
	RW    *sync.RWMutex
	Count atomic.Int64
	N     int
}

type testEmbedsMutex struct {
	sync.Mutex
	N int
}

func (e *testEmbedsMutex) Get() int { return e.N }

type testEmbedsDeep struct {
	testEmbedsMutex
}

// Scripts cannot reach a host's sync or sync/atomic values, so they cannot
// lock, unlock, or copy them.
func TestGoInterop_SyncFieldsHidden(t *testing.T) {
	g := &testGuarded{RW: &sync.RWMutex{}}
	s := NewGoStruct(reflect.ValueOf(g), DefaultRegistry())
	for _, name := range []string{"Mu", "RW", "Count"} {
		_, ok := s.GetAttr(name)
		assert.False(t, ok, name)
		assert.NotNil(t, s.SetAttr(name, Nil), name)
		for _, spec := range s.Attrs() {
			assert.NotEqual(t, spec.Name, name)
		}
	}
	n, ok := s.GetAttr("N")
	assert.True(t, ok)
	assert.Equal(t, n, NewInt(0))
	assert.True(t, g.Mu.TryLock())
}

func TestGoInterop_EmbeddedSyncMethodsHidden(t *testing.T) {
	for _, v := range []any{&testEmbedsMutex{N: 1}, &testEmbedsDeep{testEmbedsMutex{N: 1}}} {
		s := NewGoStruct(reflect.ValueOf(v), DefaultRegistry())
		for _, name := range []string{"Lock", "Unlock", "TryLock", "Mutex"} {
			_, ok := s.GetAttr(name)
			assert.False(t, ok, name)
		}
		result, err := s.CallMethod(context.Background(), "Get")
		assert.Nil(t, err)
		assert.Equal(t, result, NewInt(1))
	}
}

type testOwnLock struct {
	sync.Mutex
	Calls int
}

// Lock shadows the promoted sync.Mutex.Lock
func (o *testOwnLock) Lock() { o.Calls++ }

// A type that embeds testOwnLock gets testOwnLock.Lock, which is shallower
// than sync.Mutex.Lock
type testEmbedsOwnLock struct {
	testOwnLock
}

// A method the host type declares stays visible even when an embedded sync
// type has a method of the same name. Only the promoted sync methods are
// hidden.
func TestGoInterop_DeclaredMethodShadowsSync(t *testing.T) {
	own := &testOwnLock{}
	outer := &testEmbedsOwnLock{}
	for _, tc := range []struct {
		value any
		calls *int
	}{{own, &own.Calls}, {outer, &outer.Calls}} {
		s := NewGoStruct(reflect.ValueOf(tc.value), DefaultRegistry())
		_, err := s.CallMethod(context.Background(), "Lock")
		assert.Nil(t, err)
		assert.Equal(t, *tc.calls, 1)
		for _, name := range []string{"Unlock", "TryLock"} {
			_, ok := s.GetAttr(name)
			assert.False(t, ok, name)
		}
	}
	assert.True(t, own.Mutex.TryLock())
	assert.True(t, outer.Mutex.TryLock())
}

func TestGoInterop_SyncValuesNotConverted(t *testing.T) {
	var locker sync.Locker = &sync.Mutex{}
	for _, v := range []any{&sync.Mutex{}, &sync.WaitGroup{}, &atomic.Int64{}, locker} {
		_, err := DefaultRegistry().FromGo(v)
		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), "sync types are not available to scripts")
	}
}

type testEmbedsLocker struct {
	sync.Locker
	Name string
}

// An embedded sync.Locker is an interface, and its promoted Lock and Unlock
// are hidden like those of an embedded sync.Mutex.
func TestGoInterop_EmbeddedSyncInterfaceMethodsHidden(t *testing.T) {
	mu := &sync.Mutex{}
	obj, err := DefaultRegistry().FromGo(&testEmbedsLocker{Locker: mu, Name: "test"})
	assert.Nil(t, err)
	s := obj.(*GoStruct)
	for _, name := range []string{"Lock", "Unlock", "Locker"} {
		_, ok := s.GetAttr(name)
		assert.False(t, ok, name)
		for _, spec := range s.Attrs() {
			assert.NotEqual(t, spec.Name, name)
		}
	}
	name, ok := s.GetAttr("Name")
	assert.True(t, ok)
	assert.Equal(t, name, NewString("test"))
	assert.True(t, mu.TryLock())
}

type testNilValuer struct{ n int }

func (v *testNilValuer) RisorValue() Object { return NewInt(int64(v.n)) }

type testNilResultValuer struct{}

func (testNilResultValuer) RisorValue() Object { return (*List)(nil) }

// A typed nil pointer converts to null, even when its type is an Object or a
// RisorValuer.
func TestGoInterop_TypedNilPointerIsNull(t *testing.T) {
	r := DefaultRegistry()
	for _, v := range []any{
		(*List)(nil),
		(*String)(nil),
		(*testNilValuer)(nil),
		(*bytecode.Function)(nil),
		testNilResultValuer{},
	} {
		obj, err := r.FromGo(v)
		assert.Nil(t, err)
		assert.Equal(t, obj, Object(Nil), "%T", v)
	}
	obj, err := r.FromGo(&testNilValuer{n: 3})
	assert.Nil(t, err)
	assert.Equal(t, obj, Object(NewInt(3)))
}

type testByteRole uint8

// A slice of a defined uint8 type converts to bytes and back.
func TestGoInterop_DefinedByteSliceRoundTrip(t *testing.T) {
	ctx := context.Background()
	r := DefaultRegistry()
	obj, err := r.FromGo([]testByteRole{1, 2, 3})
	assert.Nil(t, err)
	assert.Equal(t, obj, Object(NewBytes([]byte{1, 2, 3})))

	v, err := r.ToGo(ctx, obj, reflect.TypeOf([]testByteRole{}))
	assert.Nil(t, err)
	assert.Equal(t, v, []testByteRole{1, 2, 3})

	v, err = r.ToGo(ctx, NewString("ab"), reflect.TypeOf([]testByteRole{}))
	assert.Nil(t, err)
	assert.Equal(t, v, []testByteRole{'a', 'b'})

	v, err = r.ToGo(ctx, NewBytes(nil), reflect.TypeOf([]testByteRole{}))
	assert.Nil(t, err)
	assert.Equal(t, v, []testByteRole{})

	type config struct{ Roles []testByteRole }
	cfg := &config{Roles: []testByteRole{1, 2, 3}}
	s := NewGoStruct(reflect.ValueOf(cfg), r)
	roles, ok := s.GetAttr("Roles")
	assert.True(t, ok)
	assert.Nil(t, s.SetAttr("Roles", roles))
	assert.Equal(t, cfg.Roles, []testByteRole{1, 2, 3})
}
