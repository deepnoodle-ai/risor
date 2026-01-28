package object

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"github.com/deepnoodle-ai/wonton/assert"
)

func TestTypeRegistryFloat64(t *testing.T) {
	registry := DefaultRegistry()

	t.Run("FromGo", func(t *testing.T) {
		result, err := registry.FromGo(2.0)
		assert.Nil(t, err)
		assert.Equal(t, result, NewFloat(2.0))
	})

	t.Run("ToGo", func(t *testing.T) {
		result, err := registry.ToGo(NewFloat(3.0), reflect.TypeOf(float64(0)))
		assert.Nil(t, err)
		assert.Equal(t, result, 3.0)
	})
}

func TestTypeRegistryMapString(t *testing.T) {
	registry := DefaultRegistry()

	m := map[string]string{
		"a": "apple",
		"b": "banana",
	}

	t.Run("FromGo", func(t *testing.T) {
		result, err := registry.FromGo(m)
		assert.Nil(t, err)
		assert.Equal(t, result, NewMap(map[string]Object{
			"a": NewString("apple"),
			"b": NewString("banana"),
		}))
	})

	t.Run("ToGo", func(t *testing.T) {
		risorMap := NewMap(map[string]Object{
			"c": NewString("cod"),
			"d": NewString("deer"),
		})
		result, err := registry.ToGo(risorMap, reflect.TypeOf(map[string]string{}))
		assert.Nil(t, err)
		assert.Equal(t, result, map[string]string{
			"c": "cod",
			"d": "deer",
		})
	})
}

func TestTypeRegistryPointer(t *testing.T) {
	registry := DefaultRegistry()

	t.Run("FromGo pointer", func(t *testing.T) {
		v := 2.0
		result, err := registry.FromGo(&v)
		assert.Nil(t, err)
		assert.Equal(t, result, NewFloat(2.0))
	})

	t.Run("ToGo pointer", func(t *testing.T) {
		result, err := registry.ToGo(NewFloat(3.0), reflect.TypeOf((*float64)(nil)))
		assert.Nil(t, err)
		ptr, ok := result.(*float64)
		assert.True(t, ok)
		assert.Equal(t, *ptr, 3.0)
	})

	t.Run("ToGo multiple pointers are independent", func(t *testing.T) {
		result1, err := registry.ToGo(NewFloat(3.0), reflect.TypeOf((*float64)(nil)))
		assert.Nil(t, err)
		ptr1 := result1.(*float64)

		result2, err := registry.ToGo(NewFloat(4.0), reflect.TypeOf((*float64)(nil)))
		assert.Nil(t, err)
		ptr2 := result2.(*float64)

		// Confirm the two pointers are different and have correct values
		assert.Equal(t, *ptr1, 3.0)
		assert.Equal(t, *ptr2, 4.0)
	})
}

func TestCreatingPointerViaReflect(t *testing.T) {
	v := 3.0
	var vInterface any = v

	vPointer := reflect.New(reflect.TypeOf(vInterface))
	vPointer.Elem().Set(reflect.ValueOf(v))
	floatPointer := vPointer.Interface()

	result, ok := floatPointer.(*float64)
	assert.True(t, ok)
	assert.NotNil(t, result)
	assert.Equal(t, *result, 3.0)
	assert.Equal(t, result, &v)
}

func TestSetAttributeViaReflect(t *testing.T) {
	type test struct {
		A int
	}
	tStruct := test{A: 99}
	var tInterface any = tStruct

	if reflect.TypeOf(tInterface).Kind() != reflect.Pointer {
		// Create a pointer to the value
		tInterfacePointer := reflect.New(reflect.TypeOf(tInterface))
		tInterfacePointer.Elem().Set(reflect.ValueOf(tInterface))
		tInterface = tInterfacePointer.Interface()
	}

	// Set the field "A"
	value := reflect.ValueOf(tInterface)
	value.Elem().FieldByName("A").Set(reflect.ValueOf(100))

	// Confirm the field was set
	assert.Equal(t, value.Elem().FieldByName("A").Interface(), 100)
}

func TestTypeRegistrySlice(t *testing.T) {
	registry := DefaultRegistry()

	t.Run("FromGo", func(t *testing.T) {
		v := []float64{1.0, 2.0, 3.0}
		result, err := registry.FromGo(v)
		assert.Nil(t, err)
		assert.Equal(t, result, NewList([]Object{
			NewFloat(1.0),
			NewFloat(2.0),
			NewFloat(3.0),
		}))
	})

	t.Run("ToGo", func(t *testing.T) {
		list := NewList([]Object{
			NewFloat(9.0),
			NewFloat(-8.0),
		})
		result, err := registry.ToGo(list, reflect.TypeOf([]float64{}))
		assert.Nil(t, err)
		assert.Equal(t, result, []float64{9.0, -8.0})
	})
}

func TestTypeRegistryTime(t *testing.T) {
	registry := DefaultRegistry()
	now := time.Now()

	t.Run("FromGo", func(t *testing.T) {
		result, err := registry.FromGo(now)
		assert.Nil(t, err)
		assert.Equal(t, result, NewTime(now))
	})

	t.Run("ToGo", func(t *testing.T) {
		result, err := registry.ToGo(NewTime(now), reflect.TypeOf(time.Time{}))
		assert.Nil(t, err)
		goTime, ok := result.(time.Time)
		assert.True(t, ok)
		assert.Equal(t, goTime, now)
	})
}

func TestTypeRegistryBytes(t *testing.T) {
	registry := DefaultRegistry()
	buf := []byte("abc")

	t.Run("FromGo", func(t *testing.T) {
		result, err := registry.FromGo(buf)
		assert.Nil(t, err)
		assert.Equal(t, result, NewBytes([]byte("abc")))
	})

	t.Run("ToGo", func(t *testing.T) {
		result, err := registry.ToGo(NewBytes([]byte("abc")), reflect.TypeOf([]byte{}))
		assert.Nil(t, err)
		goBuf, ok := result.([]byte)
		assert.True(t, ok)
		assert.Equal(t, goBuf, buf)
	})
}

func TestTypeRegistryArrayInt(t *testing.T) {
	registry := DefaultRegistry()
	arr := [4]int{2, 3, 4, 5}

	t.Run("FromGo", func(t *testing.T) {
		result, err := registry.FromGo(arr)
		assert.Nil(t, err)
		assert.Equal(t, result, NewList([]Object{
			NewInt(2),
			NewInt(3),
			NewInt(4),
			NewInt(5),
		}))
	})

	t.Run("ToGo", func(t *testing.T) {
		list := NewList([]Object{
			NewInt(-1),
			NewInt(-2),
		})
		result, err := registry.ToGo(list, reflect.TypeOf([4]int{}))
		assert.Nil(t, err)
		goArray, ok := result.([4]int)
		assert.True(t, ok)
		assert.Equal(t, goArray, [4]int{-1, -2})
	})
}

func TestTypeRegistryArrayFloat64(t *testing.T) {
	registry := DefaultRegistry()
	arr := [2]float64{100, 101}

	t.Run("FromGo", func(t *testing.T) {
		result, err := registry.FromGo(arr)
		assert.Nil(t, err)
		assert.Equal(t, result, NewList([]Object{
			NewFloat(100),
			NewFloat(101),
		}))
	})

	t.Run("ToGo", func(t *testing.T) {
		list := NewList([]Object{
			NewFloat(-1),
			NewFloat(-2),
		})
		result, err := registry.ToGo(list, reflect.TypeOf([2]float64{}))
		assert.Nil(t, err)
		goArray, ok := result.([2]float64)
		assert.True(t, ok)
		assert.Equal(t, goArray, [2]float64{-1, -2})
	})
}

func TestTypeRegistryGenericMap(t *testing.T) {
	registry := DefaultRegistry()

	m := map[string]any{
		"foo": 1,
		"bar": "two",
		"baz": []any{
			"three",
			4,
			false,
			map[string]any{
				"five": 5,
			},
		},
	}

	result, err := registry.FromGo(m)
	assert.Nil(t, err)
	assert.Equal(t, result, NewMap(map[string]Object{
		"foo": NewInt(1),
		"bar": NewString("two"),
		"baz": NewList([]Object{
			NewString("three"),
			NewInt(4),
			False,
			NewMap(map[string]Object{
				"five": NewInt(5),
			}),
		}),
	}))
}

func TestTypeRegistryGenericMapFromJSON(t *testing.T) {
	registry := DefaultRegistry()

	jsonStr := `{
		"foo": 1,
		"bar": "two",
		"baz": [
			"three",
			4,
			false,
			{ "five": 5 }
		]
	}`
	var v any
	err := json.Unmarshal([]byte(jsonStr), &v)
	assert.Nil(t, err)

	result, err := registry.FromGo(v)
	assert.Nil(t, err)
	assert.Equal(t, result, NewMap(map[string]Object{
		"foo": NewFloat(1), // JSON numbers are float64
		"bar": NewString("two"),
		"baz": NewList([]Object{
			NewString("three"),
			NewFloat(4),
			False,
			NewMap(map[string]Object{
				"five": NewFloat(5),
			}),
		}),
	}))
}

func TestTypeRegistryFromGo(t *testing.T) {
	registry := DefaultRegistry()

	tests := []struct {
		name     string
		input    any
		expected Object
	}{
		{"nil", nil, Nil},
		{"bool true", true, True},
		{"bool false", false, False},
		{"int", 42, NewInt(42)},
		{"int64", int64(100), NewInt(100)},
		{"float64", 3.14, NewFloat(3.14)},
		{"string", "hello", NewString("hello")},
		{"[]byte", []byte("abc"), NewBytes([]byte("abc"))},
		{"[]int", []int{1, 2, 3}, NewList([]Object{NewInt(1), NewInt(2), NewInt(3)})},
		{"map[string]int", map[string]int{"a": 1}, NewMap(map[string]Object{"a": NewInt(1)})},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := registry.FromGo(tt.input)
			assert.Nil(t, err)
			assert.Equal(t, result, tt.expected)
		})
	}
}

func TestTypeRegistryToGo(t *testing.T) {
	registry := DefaultRegistry()

	t.Run("int conversion", func(t *testing.T) {
		result, err := registry.ToGo(NewInt(42), reflect.TypeOf(int(0)))
		assert.Nil(t, err)
		assert.Equal(t, result, 42)
	})

	t.Run("int to float64", func(t *testing.T) {
		result, err := registry.ToGo(NewInt(42), reflect.TypeOf(float64(0)))
		assert.Nil(t, err)
		assert.Equal(t, result, 42.0)
	})

	t.Run("float to int", func(t *testing.T) {
		result, err := registry.ToGo(NewFloat(3.7), reflect.TypeOf(int(0)))
		assert.Nil(t, err)
		assert.Equal(t, result, 3) // truncated
	})

	t.Run("string", func(t *testing.T) {
		result, err := registry.ToGo(NewString("hello"), reflect.TypeOf(""))
		assert.Nil(t, err)
		assert.Equal(t, result, "hello")
	})

	t.Run("list to slice", func(t *testing.T) {
		list := NewList([]Object{NewInt(1), NewInt(2), NewInt(3)})
		result, err := registry.ToGo(list, reflect.TypeOf([]int{}))
		assert.Nil(t, err)
		assert.Equal(t, result, []int{1, 2, 3})
	})
}

func TestRegistryBuilder(t *testing.T) {
	type CustomType struct {
		Value int
	}

	registry := NewRegistryBuilder().
		RegisterFromGo(reflect.TypeOf(CustomType{}), func(v any) (Object, error) {
			ct := v.(CustomType)
			return NewInt(int64(ct.Value * 2)), nil
		}).
		RegisterToGo(reflect.TypeOf(CustomType{}), func(obj Object, _ reflect.Type) (any, error) {
			i, err := AsInt(obj)
			if err != nil {
				return nil, err
			}
			return CustomType{Value: int(i / 2)}, nil
		}).
		Build()

	t.Run("custom FromGo", func(t *testing.T) {
		result, err := registry.FromGo(CustomType{Value: 21})
		assert.Nil(t, err)
		assert.Equal(t, result, NewInt(42))
	})

	t.Run("custom ToGo", func(t *testing.T) {
		result, err := registry.ToGo(NewInt(42), reflect.TypeOf(CustomType{}))
		assert.Nil(t, err)
		ct, ok := result.(CustomType)
		assert.True(t, ok)
		assert.Equal(t, ct.Value, 21)
	})
}

type testRisorValuer struct {
	value string
}

func (t testRisorValuer) RisorValue() Object {
	return NewString("custom:" + t.value)
}

func TestRisorValuerInterface(t *testing.T) {
	registry := DefaultRegistry()

	result, err := registry.FromGo(testRisorValuer{value: "test"})
	assert.Nil(t, err)
	assert.Equal(t, result, NewString("custom:test"))
}

func TestFromGoType(t *testing.T) {
	// Test the convenience function
	tests := []struct {
		name     string
		input    any
		expected Object
	}{
		{"nil", nil, Nil},
		{"int", 42, NewInt(42)},
		{"string", "hello", NewString("hello")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FromGoType(tt.input)
			assert.Equal(t, result, tt.expected)
		})
	}
}

func TestAsObjects(t *testing.T) {
	m := map[string]any{
		"a": 1,
		"b": "two",
		"c": true,
	}

	result, err := AsObjects(m)
	assert.Nil(t, err)
	assert.Equal(t, result["a"], NewInt(1))
	assert.Equal(t, result["b"], NewString("two"))
	assert.Equal(t, result["c"], True)
}

// Edge case tests

func TestTypeRegistryNilHandling(t *testing.T) {
	registry := DefaultRegistry()

	t.Run("FromGo nil", func(t *testing.T) {
		result, err := registry.FromGo(nil)
		assert.Nil(t, err)
		assert.Equal(t, result, Nil)
	})

	t.Run("FromGo nil pointer", func(t *testing.T) {
		var ptr *int
		result, err := registry.FromGo(ptr)
		assert.Nil(t, err)
		assert.Equal(t, result, Nil)
	})

	t.Run("FromGo nil slice", func(t *testing.T) {
		var slice []int
		result, err := registry.FromGo(slice)
		assert.Nil(t, err)
		// nil slice becomes empty list
		assert.Equal(t, result, NewList([]Object{}))
	})

	t.Run("FromGo nil map", func(t *testing.T) {
		var m map[string]int
		result, err := registry.FromGo(m)
		assert.Nil(t, err)
		assert.Equal(t, result, NewMap(map[string]Object{}))
	})

	t.Run("FromGo nil interface", func(t *testing.T) {
		var iface any
		result, err := registry.FromGo(iface)
		assert.Nil(t, err)
		assert.Equal(t, result, Nil)
	})

	t.Run("ToGo nil to pointer", func(t *testing.T) {
		result, err := registry.ToGo(Nil, reflect.TypeOf((*int)(nil)))
		assert.Nil(t, err)
		assert.Nil(t, result)
	})

	t.Run("ToGo nil to slice", func(t *testing.T) {
		result, err := registry.ToGo(Nil, reflect.TypeOf([]int{}))
		assert.Nil(t, err)
		assert.Nil(t, result)
	})

	t.Run("ToGo nil to map", func(t *testing.T) {
		result, err := registry.ToGo(Nil, reflect.TypeOf(map[string]int{}))
		assert.Nil(t, err)
		assert.Nil(t, result)
	})
}

func TestTypeRegistryAllNumericTypes(t *testing.T) {
	registry := DefaultRegistry()

	t.Run("FromGo all int types", func(t *testing.T) {
		tests := []struct {
			name  string
			input any
		}{
			{"int", int(42)},
			{"int8", int8(42)},
			{"int16", int16(42)},
			{"int32", int32(42)},
			{"int64", int64(42)},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				result, err := registry.FromGo(tt.input)
				assert.Nil(t, err)
				assert.Equal(t, result, NewInt(42))
			})
		}
	})

	t.Run("FromGo all uint types", func(t *testing.T) {
		tests := []struct {
			name  string
			input any
		}{
			{"uint", uint(42)},
			{"uint8", uint8(42)},
			{"uint16", uint16(42)},
			{"uint32", uint32(42)},
			{"uint64", uint64(42)},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				result, err := registry.FromGo(tt.input)
				assert.Nil(t, err)
				assert.Equal(t, result, NewInt(42))
			})
		}
	})

	t.Run("FromGo all float types", func(t *testing.T) {
		tests := []struct {
			name     string
			input    any
			expected float64
		}{
			{"float32", float32(3.14), 3.14},
			{"float64", float64(3.14), 3.14},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				result, err := registry.FromGo(tt.input)
				assert.Nil(t, err)
				f, ok := result.(*Float)
				assert.True(t, ok)
				// float32 loses precision
				assert.True(t, f.value > 3.13 && f.value < 3.15)
			})
		}
	})

	t.Run("ToGo all int types", func(t *testing.T) {
		tests := []struct {
			name       string
			targetType reflect.Type
			expected   any
		}{
			{"int", reflect.TypeOf(int(0)), int(42)},
			{"int8", reflect.TypeOf(int8(0)), int8(42)},
			{"int16", reflect.TypeOf(int16(0)), int16(42)},
			{"int32", reflect.TypeOf(int32(0)), int32(42)},
			{"int64", reflect.TypeOf(int64(0)), int64(42)},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				result, err := registry.ToGo(NewInt(42), tt.targetType)
				assert.Nil(t, err)
				assert.Equal(t, result, tt.expected)
			})
		}
	})

	t.Run("ToGo all uint types", func(t *testing.T) {
		tests := []struct {
			name       string
			targetType reflect.Type
			expected   any
		}{
			{"uint", reflect.TypeOf(uint(0)), uint(42)},
			{"uint8", reflect.TypeOf(uint8(0)), uint8(42)},
			{"uint16", reflect.TypeOf(uint16(0)), uint16(42)},
			{"uint32", reflect.TypeOf(uint32(0)), uint32(42)},
			{"uint64", reflect.TypeOf(uint64(0)), uint64(42)},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				result, err := registry.ToGo(NewInt(42), tt.targetType)
				assert.Nil(t, err)
				assert.Equal(t, result, tt.expected)
			})
		}
	})

	t.Run("ToGo Byte to numeric", func(t *testing.T) {
		result, err := registry.ToGo(NewByte(42), reflect.TypeOf(int(0)))
		assert.Nil(t, err)
		assert.Equal(t, result, 42)
	})

	t.Run("ToGo Float to all int types", func(t *testing.T) {
		result, err := registry.ToGo(NewFloat(42.9), reflect.TypeOf(int(0)))
		assert.Nil(t, err)
		assert.Equal(t, result, 42) // truncated
	})
}

func TestTypeRegistryEmptyContainers(t *testing.T) {
	registry := DefaultRegistry()

	t.Run("FromGo empty slice", func(t *testing.T) {
		result, err := registry.FromGo([]int{})
		assert.Nil(t, err)
		list, ok := result.(*List)
		assert.True(t, ok)
		assert.Equal(t, list.Size(), 0)
	})

	t.Run("FromGo empty map", func(t *testing.T) {
		result, err := registry.FromGo(map[string]int{})
		assert.Nil(t, err)
		m, ok := result.(*Map)
		assert.True(t, ok)
		assert.Equal(t, m.Size(), 0)
	})

	t.Run("ToGo empty list to slice", func(t *testing.T) {
		result, err := registry.ToGo(NewList([]Object{}), reflect.TypeOf([]int{}))
		assert.Nil(t, err)
		slice, ok := result.([]int)
		assert.True(t, ok)
		assert.Equal(t, len(slice), 0)
	})

	t.Run("ToGo empty map", func(t *testing.T) {
		result, err := registry.ToGo(NewMap(map[string]Object{}), reflect.TypeOf(map[string]int{}))
		assert.Nil(t, err)
		m, ok := result.(map[string]int)
		assert.True(t, ok)
		assert.Equal(t, len(m), 0)
	})
}

func TestTypeRegistryErrorCases(t *testing.T) {
	registry := DefaultRegistry()

	t.Run("FromGo func converts to GoFunc", func(t *testing.T) {
		fn := func() {}
		result, err := registry.FromGo(fn)
		assert.Nil(t, err)
		_, ok := result.(*GoFunc)
		assert.True(t, ok)
	})

	t.Run("FromGo unsupported map key type", func(t *testing.T) {
		m := map[int]string{1: "one"}
		_, err := registry.FromGo(m)
		assert.NotNil(t, err)
	})

	t.Run("ToGo type mismatch - string to int", func(t *testing.T) {
		_, err := registry.ToGo(NewString("hello"), reflect.TypeOf(int(0)))
		assert.NotNil(t, err)
	})

	t.Run("ToGo type mismatch - int to string", func(t *testing.T) {
		_, err := registry.ToGo(NewInt(42), reflect.TypeOf(""))
		assert.NotNil(t, err)
	})

	t.Run("ToGo type mismatch - string to bool", func(t *testing.T) {
		_, err := registry.ToGo(NewString("true"), reflect.TypeOf(true))
		assert.NotNil(t, err)
	})

	t.Run("ToGo list with wrong element type", func(t *testing.T) {
		list := NewList([]Object{NewString("not an int")})
		_, err := registry.ToGo(list, reflect.TypeOf([]int{}))
		assert.NotNil(t, err)
	})

	t.Run("ToGo map with wrong value type", func(t *testing.T) {
		m := NewMap(map[string]Object{"a": NewString("not an int")})
		_, err := registry.ToGo(m, reflect.TypeOf(map[string]int{}))
		assert.NotNil(t, err)
	})

	t.Run("ToGo non-list to slice", func(t *testing.T) {
		_, err := registry.ToGo(NewString("hello"), reflect.TypeOf([]int{}))
		assert.NotNil(t, err)
	})

	t.Run("ToGo non-map to map", func(t *testing.T) {
		_, err := registry.ToGo(NewString("hello"), reflect.TypeOf(map[string]int{}))
		assert.NotNil(t, err)
	})
}

func TestTypeRegistryBytesFromString(t *testing.T) {
	registry := DefaultRegistry()

	t.Run("ToGo String to []byte", func(t *testing.T) {
		result, err := registry.ToGo(NewString("hello"), reflect.TypeOf([]byte{}))
		assert.Nil(t, err)
		assert.Equal(t, result, []byte("hello"))
	})
}

func TestTypeRegistryTimeFromString(t *testing.T) {
	registry := DefaultRegistry()

	t.Run("ToGo String to time.Time", func(t *testing.T) {
		result, err := registry.ToGo(NewString("2024-01-15T10:30:00Z"), reflect.TypeOf(time.Time{}))
		assert.Nil(t, err)
		tm, ok := result.(time.Time)
		assert.True(t, ok)
		assert.Equal(t, tm.Year(), 2024)
		assert.Equal(t, tm.Month(), time.January)
		assert.Equal(t, tm.Day(), 15)
	})

	t.Run("ToGo invalid time string", func(t *testing.T) {
		_, err := registry.ToGo(NewString("not a time"), reflect.TypeOf(time.Time{}))
		assert.NotNil(t, err)
	})
}

func TestTypeRegistryInterface(t *testing.T) {
	registry := DefaultRegistry()

	t.Run("ToGo to any interface", func(t *testing.T) {
		result, err := registry.ToGo(NewInt(42), reflect.TypeOf((*any)(nil)).Elem())
		assert.Nil(t, err)
		// Should return the Object's Interface() value
		assert.Equal(t, result, int64(42))
	})

	t.Run("ToGo to error interface", func(t *testing.T) {
		errObj := Errorf("test error")
		result, err := registry.ToGo(errObj, reflect.TypeOf((*error)(nil)).Elem())
		assert.Nil(t, err)
		goErr, ok := result.(error)
		assert.True(t, ok)
		assert.NotNil(t, goErr)
	})

	t.Run("ToGo String to error interface", func(t *testing.T) {
		result, err := registry.ToGo(NewString("error message"), reflect.TypeOf((*error)(nil)).Elem())
		assert.Nil(t, err)
		goErr, ok := result.(error)
		assert.True(t, ok)
		assert.Equal(t, goErr.Error(), "error message")
	})
}

func TestTypeRegistryObjectPassthrough(t *testing.T) {
	registry := DefaultRegistry()

	t.Run("FromGo passes through Object", func(t *testing.T) {
		obj := NewInt(42)
		result, err := registry.FromGo(obj)
		assert.Nil(t, err)
		assert.Equal(t, result, obj)
	})
}

func TestTypeRegistryNestedStructures(t *testing.T) {
	registry := DefaultRegistry()

	t.Run("deeply nested FromGo", func(t *testing.T) {
		nested := map[string]any{
			"level1": map[string]any{
				"level2": map[string]any{
					"level3": []any{1, 2, 3},
				},
			},
		}
		result, err := registry.FromGo(nested)
		assert.Nil(t, err)

		m, ok := result.(*Map)
		assert.True(t, ok)

		level1, _ := AsMap(m.Get("level1"))
		level2, _ := AsMap(level1.Get("level2"))
		level3, _ := AsList(level2.Get("level3"))
		assert.Equal(t, level3.Size(), 3)
	})

	t.Run("deeply nested ToGo", func(t *testing.T) {
		nested := NewMap(map[string]Object{
			"a": NewMap(map[string]Object{
				"b": NewList([]Object{NewInt(1), NewInt(2)}),
			}),
		})

		targetType := reflect.TypeOf(map[string]map[string][]int{})
		result, err := registry.ToGo(nested, targetType)
		assert.Nil(t, err)

		m := result.(map[string]map[string][]int)
		assert.Equal(t, m["a"]["b"], []int{1, 2})
	})
}

func TestAsObjectsWithRegistry(t *testing.T) {
	type Custom struct {
		Value int
	}

	registry := NewRegistryBuilder().
		RegisterFromGo(reflect.TypeOf(Custom{}), func(v any) (Object, error) {
			return NewInt(int64(v.(Custom).Value * 10)), nil
		}).
		Build()

	m := map[string]any{
		"custom": Custom{Value: 5},
		"normal": 42,
	}

	result, err := AsObjectsWithRegistry(m, registry)
	assert.Nil(t, err)
	assert.Equal(t, result["custom"], NewInt(50)) // Custom converter
	assert.Equal(t, result["normal"], NewInt(42)) // Default
}

func TestAsObjectsWithFunc(t *testing.T) {
	m := map[string]any{
		"fn": func() {}, // Functions are now converted to GoFunc
	}

	result, err := AsObjects(m)
	assert.Nil(t, err)
	_, ok := result["fn"].(*GoFunc)
	assert.True(t, ok)
}

func TestFromGoTypeFunc(t *testing.T) {
	// FromGoType now converts functions to GoFunc
	result := FromGoType(func() {})
	_, ok := result.(*GoFunc)
	assert.True(t, ok)
}

func TestRuneConversion(t *testing.T) {
	t.Run("RuneToObject", func(t *testing.T) {
		result := RuneToObject('A')
		assert.Equal(t, result, NewString("A"))
	})

	t.Run("RuneToObject unicode", func(t *testing.T) {
		result := RuneToObject('日')
		assert.Equal(t, result, NewString("日"))
	})

	t.Run("ObjectToRune from String", func(t *testing.T) {
		result, err := ObjectToRune(NewString("A"))
		assert.Nil(t, err)
		assert.Equal(t, result, 'A')
	})

	t.Run("ObjectToRune from Int", func(t *testing.T) {
		result, err := ObjectToRune(NewInt(65))
		assert.Nil(t, err)
		assert.Equal(t, result, 'A')
	})

	t.Run("ObjectToRune error on multi-char string", func(t *testing.T) {
		_, err := ObjectToRune(NewString("AB"))
		assert.NotNil(t, err)
	})

	t.Run("ObjectToRune error on wrong type", func(t *testing.T) {
		_, err := ObjectToRune(NewFloat(65.0))
		assert.NotNil(t, err)
	})
}

func TestRegistryBuilderOverridesBase(t *testing.T) {
	// Custom registry that converts int differently
	registry := NewRegistryBuilder().
		RegisterFromGo(reflect.TypeOf(int(0)), func(v any) (Object, error) {
			return NewInt(int64(v.(int) * 100)), nil
		}).
		Build()

	result, err := registry.FromGo(5)
	assert.Nil(t, err)
	assert.Equal(t, result, NewInt(500)) // Custom behavior
}

func TestDefaultRegistrySingleton(t *testing.T) {
	r1 := DefaultRegistry()
	r2 := DefaultRegistry()
	assert.True(t, r1 == r2) // Same instance
}
