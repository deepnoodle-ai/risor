package vm

import (
	"context"
	"reflect"
	"testing"

	"github.com/deepnoodle-ai/wonton/assert"
	"github.com/risor-io/risor/compiler"
	"github.com/risor-io/risor/object"
	"github.com/risor-io/risor/parser"
)

func TestTypeRegistryMethod(t *testing.T) {
	t.Run("returns default when not set", func(t *testing.T) {
		vm, err := NewEmpty()
		assert.Nil(t, err)
		registry := vm.TypeRegistry()
		assert.NotNil(t, registry)
		assert.Equal(t, registry, object.DefaultRegistry())
	})

	t.Run("returns custom when set", func(t *testing.T) {
		customRegistry := object.NewRegistryBuilder().Build()
		vm, err := NewEmpty()
		assert.Nil(t, err)

		// Apply option manually
		WithTypeRegistry(customRegistry)(vm)

		registry := vm.TypeRegistry()
		assert.Equal(t, registry, customRegistry)
	})
}

func TestWithTypeRegistryOption(t *testing.T) {
	// Create a custom registry that doubles integers
	customRegistry := object.NewRegistryBuilder().
		RegisterFromGo(reflect.TypeOf(int(0)), func(v any) (object.Object, error) {
			return object.NewInt(int64(v.(int) * 2)), nil
		}).
		Build()

	// Compile a simple program that uses a global
	ast, err := parser.Parse(context.Background(), "x", nil)
	assert.Nil(t, err)

	code, err := compiler.Compile(ast, &compiler.Config{GlobalNames: []string{"x"}})
	assert.Nil(t, err)

	// Run with custom registry - the value should be doubled
	vm := New(code,
		WithGlobals(map[string]any{"x": 21}),
		WithTypeRegistry(customRegistry),
	)
	err = vm.Run(context.Background())
	assert.Nil(t, err)

	tos, ok := vm.TOS()
	assert.True(t, ok)
	assert.Equal(t, tos, object.NewInt(42)) // 21 * 2 = 42
}

func TestTypeRegistryWithGlobalConversion(t *testing.T) {
	// Define a custom type
	type Point struct {
		X, Y int
	}

	// Create registry that knows how to convert Point
	registry := object.NewRegistryBuilder().
		RegisterFromGo(reflect.TypeOf(Point{}), func(v any) (object.Object, error) {
			p := v.(Point)
			return object.NewMap(map[string]object.Object{
				"x": object.NewInt(int64(p.X)),
				"y": object.NewInt(int64(p.Y)),
			}), nil
		}).
		Build()

	// Compile program that accesses point fields
	ast, err := parser.Parse(context.Background(), "point.x + point.y", nil)
	assert.Nil(t, err)

	code, err := compiler.Compile(ast, &compiler.Config{GlobalNames: []string{"point"}})
	assert.Nil(t, err)

	// Run with custom registry
	vm := New(code,
		WithGlobals(map[string]any{"point": Point{X: 10, Y: 20}}),
		WithTypeRegistry(registry),
	)
	err = vm.Run(context.Background())
	assert.Nil(t, err)

	tos, ok := vm.TOS()
	assert.True(t, ok)
	assert.Equal(t, tos, object.NewInt(30))
}

// Custom type implementing RisorValuer
type customPoint struct {
	X, Y int
}

func (p customPoint) RisorValue() object.Object {
	return object.NewMap(map[string]object.Object{
		"x": object.NewInt(int64(p.X)),
		"y": object.NewInt(int64(p.Y)),
	})
}

func TestRisorValuerWithVM(t *testing.T) {
	// Types implementing RisorValuer should work without custom registry
	ast, err := parser.Parse(context.Background(), "point.x * point.y", nil)
	assert.Nil(t, err)

	code, err := compiler.Compile(ast, &compiler.Config{GlobalNames: []string{"point"}})
	assert.Nil(t, err)

	vm := New(code,
		WithGlobals(map[string]any{"point": customPoint{X: 6, Y: 7}}),
	)
	err = vm.Run(context.Background())
	assert.Nil(t, err)

	tos, ok := vm.TOS()
	assert.True(t, ok)
	assert.Equal(t, tos, object.NewInt(42))
}

func TestTypeRegistryPreservedAcrossRuns(t *testing.T) {
	customRegistry := object.NewRegistryBuilder().Build()

	ast, err := parser.Parse(context.Background(), "1 + 2", nil)
	assert.Nil(t, err)

	code, err := compiler.Compile(ast, nil)
	assert.Nil(t, err)

	vm := New(code, WithTypeRegistry(customRegistry))

	// First run
	err = vm.Run(context.Background())
	assert.Nil(t, err)

	// Registry should still be the custom one
	assert.Equal(t, vm.TypeRegistry(), customRegistry)

	// Second run with new code
	ast2, err := parser.Parse(context.Background(), "3 + 4", nil)
	assert.Nil(t, err)
	code2, err := compiler.Compile(ast2, nil)
	assert.Nil(t, err)

	err = vm.RunCode(context.Background(), code2)
	assert.Nil(t, err)

	// Registry should still be preserved
	assert.Equal(t, vm.TypeRegistry(), customRegistry)
}

func TestTypeRegistryWithDifferentGlobalTypes(t *testing.T) {
	tests := []struct {
		name     string
		globals  map[string]any
		code     string
		expected object.Object
	}{
		{
			name:     "int global",
			globals:  map[string]any{"x": 42},
			code:     "x",
			expected: object.NewInt(42),
		},
		{
			name:     "string global",
			globals:  map[string]any{"s": "hello"},
			code:     "s",
			expected: object.NewString("hello"),
		},
		{
			name:     "bool global",
			globals:  map[string]any{"b": true},
			code:     "b",
			expected: object.True,
		},
		{
			name:     "slice global",
			globals:  map[string]any{"arr": []int{1, 2, 3}},
			code:     "arr[1]",
			expected: object.NewInt(2),
		},
		{
			name:     "map global",
			globals:  map[string]any{"m": map[string]int{"a": 10}},
			code:     "m.a",
			expected: object.NewInt(10),
		},
		{
			name:     "nested map global",
			globals:  map[string]any{"m": map[string]any{"inner": map[string]int{"val": 99}}},
			code:     "m.inner.val",
			expected: object.NewInt(99),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ast, err := parser.Parse(context.Background(), tt.code, nil)
			assert.Nil(t, err)

			globalNames := make([]string, 0, len(tt.globals))
			for name := range tt.globals {
				globalNames = append(globalNames, name)
			}

			code, err := compiler.Compile(ast, &compiler.Config{GlobalNames: globalNames})
			assert.Nil(t, err)

			vm := New(code, WithGlobals(tt.globals))
			err = vm.Run(context.Background())
			assert.Nil(t, err)

			tos, ok := vm.TOS()
			assert.True(t, ok)
			assert.Equal(t, tos, tt.expected)
		})
	}
}

func TestTypeRegistryErrorOnInvalidGlobal(t *testing.T) {
	// Functions cannot be converted to Risor objects
	globals := map[string]any{
		"fn": func() {},
	}

	ast, err := parser.Parse(context.Background(), "fn", nil)
	assert.Nil(t, err)

	code, err := compiler.Compile(ast, &compiler.Config{GlobalNames: []string{"fn"}})
	assert.Nil(t, err)

	// New() should panic because of invalid global
	defer func() {
		r := recover()
		assert.NotNil(t, r)
	}()

	_ = New(code, WithGlobals(globals))
	t.Fatal("expected panic")
}

func TestNewEmptyWithTypeRegistry(t *testing.T) {
	customRegistry := object.NewRegistryBuilder().Build()

	vm, err := NewEmpty()
	assert.Nil(t, err)

	// Apply type registry option
	err = vm.applyOptions([]Option{WithTypeRegistry(customRegistry)})
	assert.Nil(t, err)

	assert.Equal(t, vm.TypeRegistry(), customRegistry)
}
