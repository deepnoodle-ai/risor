package object

import (
	"context"
	"fmt"
	"reflect"
	"sync"

	"github.com/deepnoodle-ai/risor/v2/pkg/op"
)

// structMeta caches reflection metadata for a struct type.
type structMeta struct {
	fields  map[string]int // field name -> field index
	methods map[string]int // method name -> method index (on pointer type)
}

// structMetaCache stores metadata for struct types.
var structMetaCache sync.Map // map[reflect.Type]*structMeta

// getStructMeta returns cached metadata for a struct type, creating it if needed.
func getStructMeta(structType reflect.Type) *structMeta {
	if meta, ok := structMetaCache.Load(structType); ok {
		return meta.(*structMeta)
	}

	meta := &structMeta{
		fields:  make(map[string]int),
		methods: make(map[string]int),
	}

	// Index fields
	for i := range structType.NumField() {
		field := structType.Field(i)
		if field.IsExported() {
			meta.fields[field.Name] = i
		}
	}

	// Index methods on pointer type (includes both pointer and value receiver methods)
	ptrType := reflect.PointerTo(structType)
	for i := range ptrType.NumMethod() {
		method := ptrType.Method(i)
		if method.IsExported() {
			meta.methods[method.Name] = i
		}
	}

	structMetaCache.Store(structType, meta)
	return meta
}

// GoStruct wraps a Go struct for use in Risor.
// It exposes struct fields and methods via GetAttr/SetAttr.
type GoStruct struct {
	value      reflect.Value // The struct (always a pointer for addressability)
	structType reflect.Type  // The struct type (non-pointer)
	registry   *TypeRegistry // For type conversion
}

// NewGoStruct creates a new GoStruct wrapping the given Go struct pointer.
// The value must be a pointer to a struct.
func NewGoStruct(ptrVal reflect.Value, registry *TypeRegistry) *GoStruct {
	if ptrVal.Kind() != reflect.Ptr {
		panic(fmt.Sprintf("GoStruct: expected pointer, got %s", ptrVal.Kind()))
	}
	if ptrVal.Elem().Kind() != reflect.Struct {
		panic(fmt.Sprintf("GoStruct: expected pointer to struct, got pointer to %s", ptrVal.Elem().Kind()))
	}
	return &GoStruct{
		value:      ptrVal,
		structType: ptrVal.Elem().Type(),
		registry:   registry,
	}
}

func (g *GoStruct) Type() Type {
	return GOSTRUCT
}

func (g *GoStruct) Inspect() string {
	return fmt.Sprintf("go_struct(%s)", g.structType.String())
}

func (g *GoStruct) String() string {
	return g.Inspect()
}

func (g *GoStruct) Interface() any {
	return g.value.Interface()
}

func (g *GoStruct) Equals(other Object) bool {
	otherStruct, ok := other.(*GoStruct)
	if !ok {
		return false
	}
	// Compare by pointer identity
	return g.value.Pointer() == otherStruct.value.Pointer()
}

func (g *GoStruct) Attrs() []AttrSpec {
	// Return specs for fields and methods
	meta := getStructMeta(g.structType)
	specs := make([]AttrSpec, 0, len(meta.fields)+len(meta.methods))

	for name := range meta.fields {
		specs = append(specs, AttrSpec{Name: name})
	}
	for name := range meta.methods {
		specs = append(specs, AttrSpec{Name: name})
	}

	return specs
}

func (g *GoStruct) GetAttr(name string) (Object, bool) {
	meta := getStructMeta(g.structType)

	// Check for field first
	if idx, ok := meta.fields[name]; ok {
		fieldVal := g.value.Elem().Field(idx)
		obj, err := g.registry.FromGo(fieldVal.Interface())
		if err != nil {
			return nil, false
		}
		return obj, true
	}

	// Check for method
	if idx, ok := meta.methods[name]; ok {
		method := g.value.Method(idx)
		methodName := fmt.Sprintf("%s.%s", g.structType.Name(), name)
		return NewGoFunc(method, methodName, g.registry), true
	}

	return nil, false
}

func (g *GoStruct) SetAttr(name string, value Object) error {
	meta := getStructMeta(g.structType)

	idx, ok := meta.fields[name]
	if !ok {
		return TypeErrorf("go_struct %s has no field %q", g.structType.Name(), name)
	}

	fieldVal := g.value.Elem().Field(idx)
	if !fieldVal.CanSet() {
		return TypeErrorf("field %q of %s is not settable", name, g.structType.Name())
	}

	goVal, err := g.registry.ToGo(value, fieldVal.Type())
	if err != nil {
		return fmt.Errorf("cannot set field %q: %w", name, err)
	}

	fieldVal.Set(reflect.ValueOf(goVal))
	return nil
}

func (g *GoStruct) IsTruthy() bool {
	return !g.value.IsNil()
}

func (g *GoStruct) RunOperation(opType op.BinaryOpType, right Object) (Object, error) {
	return nil, newTypeErrorf("unsupported operation for go_struct: %v", opType)
}

// Value returns the underlying Go struct pointer as a reflect.Value.
func (g *GoStruct) Value() reflect.Value {
	return g.value
}

// StructType returns the type of the underlying struct.
func (g *GoStruct) StructType() reflect.Type {
	return g.structType
}

// CallMethod calls a method on the struct by name with the given arguments.
// This is a convenience method that combines GetAttr and Call.
func (g *GoStruct) CallMethod(ctx context.Context, name string, args ...Object) (Object, error) {
	attr, ok := g.GetAttr(name)
	if !ok {
		return nil, fmt.Errorf("go_struct %s has no method %q", g.structType.Name(), name)
	}

	callable, ok := attr.(Callable)
	if !ok {
		return nil, fmt.Errorf("%q on %s is not callable", name, g.structType.Name())
	}

	return callable.Call(ctx, args...)
}
