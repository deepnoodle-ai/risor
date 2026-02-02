package object

import (
	"context"
	"testing"
	"time"

	"github.com/deepnoodle-ai/wonton/assert"
)

// TestAttrRegistryBasics tests the core AttrRegistry functionality.
func TestAttrRegistryBasics(t *testing.T) {
	type testObj struct {
		value int
	}

	registry := NewAttrRegistry[*testObj]("test")

	// Define a method
	registry.Define("double").
		Doc("Double the value").
		Returns("int").
		Impl(func(obj *testObj, ctx context.Context, args ...Object) (Object, error) {
			return NewInt(int64(obj.value * 2)), nil
		})

	// Define a property
	registry.Define("value").
		Doc("The value").
		Returns("int").
		Getter(func(obj *testObj) Object {
			return NewInt(int64(obj.value))
		})

	obj := &testObj{value: 21}

	// Test method access
	method, ok := registry.GetAttr(obj, "double")
	assert.True(t, ok)
	assert.NotNil(t, method)

	builtin, ok := method.(*Builtin)
	assert.True(t, ok)

	result, err := builtin.Call(context.Background())
	assert.Nil(t, err)
	assert.Equal(t, result.(*Int).Value(), int64(42))

	// Test property access
	prop, ok := registry.GetAttr(obj, "value")
	assert.True(t, ok)
	assert.Equal(t, prop.(*Int).Value(), int64(21))

	// Test unknown attribute
	_, ok = registry.GetAttr(obj, "unknown")
	assert.False(t, ok)
}

// TestAttrRegistrySpecs tests the Specs() method.
func TestAttrRegistrySpecs(t *testing.T) {
	type testObj struct{}
	registry := NewAttrRegistry[*testObj]("test")

	registry.Define("method1").
		Doc("First method").
		Arg("x").
		Returns("int").
		Impl(func(obj *testObj, ctx context.Context, args ...Object) (Object, error) {
			return Nil, nil
		})

	registry.Define("prop1").
		Doc("First property").
		Returns("string").
		Getter(func(obj *testObj) Object {
			return NewString("test")
		})

	specs := registry.Specs()
	assert.Equal(t, len(specs), 2)

	// Check method spec
	assert.Equal(t, specs[0].Name, "method1")
	assert.Equal(t, specs[0].Doc, "First method")
	assert.Equal(t, len(specs[0].Args), 1)
	assert.Equal(t, specs[0].Args[0], "x")
	assert.Equal(t, specs[0].Returns, "int")

	// Check property spec
	assert.Equal(t, specs[1].Name, "prop1")
	assert.Equal(t, specs[1].Doc, "First property")
	assert.Nil(t, specs[1].Args)
	assert.Equal(t, specs[1].Returns, "string")
}

// TestAttrRegistryArgValidation tests automatic argument count validation.
func TestAttrRegistryArgValidation(t *testing.T) {
	type testObj struct{}
	registry := NewAttrRegistry[*testObj]("test")

	registry.Define("two_args").
		Args("a", "b").
		Impl(func(obj *testObj, ctx context.Context, args ...Object) (Object, error) {
			return NewInt(42), nil
		})

	obj := &testObj{}
	method, ok := registry.GetAttr(obj, "two_args")
	assert.True(t, ok)

	builtin := method.(*Builtin)
	ctx := context.Background()

	// Correct number of args
	result, err := builtin.Call(ctx, NewInt(1), NewInt(2))
	assert.Nil(t, err)
	assert.Equal(t, result.(*Int).Value(), int64(42))

	// Too few args
	_, err = builtin.Call(ctx, NewInt(1))
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "expected 2 arguments")

	// Too many args
	_, err = builtin.Call(ctx, NewInt(1), NewInt(2), NewInt(3))
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "expected 2 arguments")
}

// TestAttrRegistrySingleArg tests singular argument error message.
func TestAttrRegistrySingleArg(t *testing.T) {
	type testObj struct{}
	registry := NewAttrRegistry[*testObj]("test")

	registry.Define("one_arg").
		Arg("x").
		Impl(func(obj *testObj, ctx context.Context, args ...Object) (Object, error) {
			return Nil, nil
		})

	obj := &testObj{}
	method, _ := registry.GetAttr(obj, "one_arg")
	builtin := method.(*Builtin)

	_, err := builtin.Call(context.Background())
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "expected 1 argument")
}

// TestArgHelper tests the Arg helper function.
func TestArgHelper(t *testing.T) {
	args := []Object{NewInt(42), NewString("hello")}

	// Correct type
	intVal, err := Arg[*Int](args, 0, "test")
	assert.Nil(t, err)
	assert.Equal(t, intVal.Value(), int64(42))

	strVal, err := Arg[*String](args, 1, "test")
	assert.Nil(t, err)
	assert.Equal(t, strVal.Value(), "hello")

	// Wrong type
	_, err = Arg[*String](args, 0, "test")
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "expected")

	// Out of bounds
	_, err = Arg[*Int](args, 5, "test")
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "missing argument")
}

// TestMethodRegistryAlias tests backward compatibility alias.
func TestMethodRegistryAlias(t *testing.T) {
	type testObj struct{}

	// NewMethodRegistry should work the same as NewAttrRegistry
	registry := NewMethodRegistry[*testObj]("test")

	registry.Define("method").
		Impl(func(obj *testObj, ctx context.Context, args ...Object) (Object, error) {
			return NewString("works"), nil
		})

	obj := &testObj{}
	method, ok := registry.GetAttr(obj, "method")
	assert.True(t, ok)

	result, err := method.(*Builtin).Call(context.Background())
	assert.Nil(t, err)
	assert.Equal(t, result.(*String).Value(), "works")
}

// TestStringMethods tests String methods via GetAttr.
func TestStringMethods(t *testing.T) {
	ctx := context.Background()
	s := NewString("hello world")

	// Test contains
	contains, ok := s.GetAttr("contains")
	assert.True(t, ok)
	result, err := contains.(*Builtin).Call(ctx, NewString("world"))
	assert.Nil(t, err)
	assert.Equal(t, result.(*Bool).Value(), true)

	// Test to_upper
	toUpper, ok := s.GetAttr("to_upper")
	assert.True(t, ok)
	result, err = toUpper.(*Builtin).Call(ctx)
	assert.Nil(t, err)
	assert.Equal(t, result.(*String).Value(), "HELLO WORLD")

	// Test split
	split, ok := s.GetAttr("split")
	assert.True(t, ok)
	result, err = split.(*Builtin).Call(ctx, NewString(" "))
	assert.Nil(t, err)
	list := result.(*List)
	assert.Equal(t, list.Len().Value(), int64(2))
}

// TestStringAttrs tests String.Attrs() for introspection.
func TestStringAttrs(t *testing.T) {
	s := NewString("test")
	attrs := s.Attrs()
	assert.True(t, len(attrs) >= 18) // At least 18 methods defined

	// Verify some expected methods exist
	names := make(map[string]bool)
	for _, attr := range attrs {
		names[attr.Name] = true
	}
	assert.True(t, names["contains"])
	assert.True(t, names["split"])
	assert.True(t, names["to_upper"])
	assert.True(t, names["to_lower"])
}

// TestBytesMethods tests Bytes methods via GetAttr.
func TestBytesMethods(t *testing.T) {
	ctx := context.Background()
	b := NewBytes([]byte("hello"))

	// Test clone
	clone, ok := b.GetAttr("clone")
	assert.True(t, ok)
	result, err := clone.(*Builtin).Call(ctx)
	assert.Nil(t, err)
	assert.True(t, result.(*Bytes).Equals(b))

	// Test contains
	contains, ok := b.GetAttr("contains")
	assert.True(t, ok)
	result, err = contains.(*Builtin).Call(ctx, NewBytes([]byte("ell")))
	assert.Nil(t, err)
	assert.Equal(t, result.(*Bool).Value(), true)

	// Test has_prefix
	hasPrefix, ok := b.GetAttr("has_prefix")
	assert.True(t, ok)
	result, err = hasPrefix.(*Builtin).Call(ctx, NewBytes([]byte("he")))
	assert.Nil(t, err)
	assert.Equal(t, result.(*Bool).Value(), true)
}

// TestBytesAttrs tests Bytes.Attrs() for introspection.
func TestBytesAttrs(t *testing.T) {
	b := NewBytes([]byte("test"))
	attrs := b.Attrs()
	assert.True(t, len(attrs) >= 15) // At least 15 methods defined

	names := make(map[string]bool)
	for _, attr := range attrs {
		names[attr.Name] = true
	}
	assert.True(t, names["clone"])
	assert.True(t, names["contains"])
	assert.True(t, names["replace_all"])
}

// TestTimeMethods tests Time methods via GetAttr.
func TestTimeMethods(t *testing.T) {
	ctx := context.Background()
	testTime, _ := time.Parse(time.RFC3339, "2024-01-15T10:30:00Z")
	tm := NewTime(testTime)

	// Test unix
	unix, ok := tm.GetAttr("unix")
	assert.True(t, ok)
	result, err := unix.(*Builtin).Call(ctx)
	assert.Nil(t, err)
	assert.True(t, result.(*Int).Value() > 0)

	// Test utc
	utc, ok := tm.GetAttr("utc")
	assert.True(t, ok)
	result, err = utc.(*Builtin).Call(ctx)
	assert.Nil(t, err)
	assert.NotNil(t, result.(*Time))

	// Test format
	format, ok := tm.GetAttr("format")
	assert.True(t, ok)
	result, err = format.(*Builtin).Call(ctx, NewString("2006-01-02"))
	assert.Nil(t, err)
	assert.Equal(t, result.(*String).Value(), "2024-01-15")
}

// TestTimeAttrs tests Time.Attrs() for introspection.
func TestTimeAttrs(t *testing.T) {
	testTime, _ := time.Parse(time.RFC3339, "2024-01-15T10:30:00Z")
	tm := NewTime(testTime)
	attrs := tm.Attrs()
	assert.Equal(t, len(attrs), 6)

	names := make(map[string]bool)
	for _, attr := range attrs {
		names[attr.Name] = true
	}
	assert.True(t, names["add_date"])
	assert.True(t, names["unix"])
	assert.True(t, names["format"])
}

// TestColorMethods tests Color methods via GetAttr.
func TestColorMethods(t *testing.T) {
	ctx := context.Background()
	c := NewColor(&testColor{r: 255, g: 128, b: 64, a: 255})

	rgba, ok := c.GetAttr("rgba")
	assert.True(t, ok)
	result, err := rgba.(*Builtin).Call(ctx)
	assert.Nil(t, err)
	list := result.(*List)
	assert.Equal(t, list.Len().Value(), int64(4))
}

// TestColorAttrs tests Color.Attrs() for introspection.
func TestColorAttrs(t *testing.T) {
	c := NewColor(&testColor{})
	attrs := c.Attrs()
	assert.Equal(t, len(attrs), 1)
	assert.Equal(t, attrs[0].Name, "rgba")
}

// TestModuleAttrs tests Module attributes.
func TestModuleAttrs(t *testing.T) {
	m := NewBuiltinsModule("mymodule", map[string]Object{
		"foo": NewString("bar"),
	})

	// Test __name__ property
	name, ok := m.GetAttr("__name__")
	assert.True(t, ok)
	assert.Equal(t, name.(*String).Value(), "mymodule")

	// Test dynamic attribute from builtins
	foo, ok := m.GetAttr("foo")
	assert.True(t, ok)
	assert.Equal(t, foo.(*String).Value(), "bar")

	// Test Attrs() returns registry specs
	attrs := m.Attrs()
	assert.Equal(t, len(attrs), 1)
	assert.Equal(t, attrs[0].Name, "__name__")
}

// TestBuiltinAttrs tests Builtin attributes.
func TestBuiltinAttrs(t *testing.T) {
	b := NewBuiltin("mybuiltin", func(ctx context.Context, args ...Object) (Object, error) {
		return Nil, nil
	}).InModule("mymodule")

	// Test __name__ property
	name, ok := b.GetAttr("__name__")
	assert.True(t, ok)
	assert.Equal(t, name.(*String).Value(), "mymodule.mybuiltin")

	// Test __module__ property (should be nil since no actual module)
	module, ok := b.GetAttr("__module__")
	assert.True(t, ok)
	assert.Equal(t, module, Nil)

	// Test Attrs() returns registry specs
	attrs := b.Attrs()
	assert.Equal(t, len(attrs), 2)

	names := make(map[string]bool)
	for _, attr := range attrs {
		names[attr.Name] = true
	}
	assert.True(t, names["__name__"])
	assert.True(t, names["__module__"])
}

// TestRangeAttrs tests Range.Attrs() returns property specs.
func TestRangeAttrs(t *testing.T) {
	r := NewRange(0, 10, 2)
	attrs := r.Attrs()
	assert.Equal(t, len(attrs), 3)

	// Verify it's a new slice (not the internal one)
	attrs[0].Name = "modified"
	origAttrs := r.Attrs()
	assert.Equal(t, origAttrs[0].Name, "start")
}

// TestErrorAttrs tests Error.Attrs() for introspection.
func TestErrorAttrs(t *testing.T) {
	e := Errorf("test error")
	attrs := e.Attrs()
	assert.Equal(t, len(attrs), 8)

	names := make(map[string]bool)
	for _, attr := range attrs {
		names[attr.Name] = true
	}
	assert.True(t, names["error"])
	assert.True(t, names["message"])
	assert.True(t, names["line"])
	assert.True(t, names["kind"])
}

// testColor implements color.Color for testing.
type testColor struct {
	r, g, b, a uint8
}

func (c *testColor) RGBA() (r, g, b, a uint32) {
	return uint32(c.r), uint32(c.g), uint32(c.b), uint32(c.a)
}
