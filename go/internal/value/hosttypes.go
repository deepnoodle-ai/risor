package value

import (
	"encoding"
	"reflect"
	"sort"
	"strings"
	"sync"
	"unsafe"
)

// This file presents Go host values by the binding rules of package F1 6.2
// (spec/packages/f01-host-types.md): a type that implements
// encoding.TextMarshaler as the string MarshalText gives, a pointer as its
// target, and a struct as a map of the fields encoding/json would write. A
// type that implements Opaque is opaque, and that test comes first. These
// rules are reached only on the reflection path, after the fast cases.

var textMarshaler = reflect.TypeFor[encoding.TextMarshaler]()

// StructField is one field a struct presents (6.2 items 1 to 4).
type StructField struct {
	Key string
	// Index is the path of the field through embedded structs.
	Index []int
	// Type is the field's Go type.
	Type reflect.Type
	// OmitEmpty and OmitZero mark a field that is absent when it holds what
	// encoding/json would leave out.
	OmitEmpty, OmitZero bool
	// ViaPointer is set when the path crosses an embedded pointer, which may
	// be nil: the field is then absent.
	ViaPointer bool
	// ViaUnexported is set when the path crosses an unexported embedded
	// struct, whose promoted fields encoding/json writes but reflection
	// reads only through the field's address.
	ViaUnexported bool
}

// Optional reports whether the field may be presented as absent.
func (f *StructField) Optional() bool { return f.OmitEmpty || f.OmitZero || f.ViaPointer }

type structInfo struct {
	fields []StructField
	byKey  map[string]int
}

var structCache sync.Map // reflect.Type -> *structInfo

// StructFields returns the fields a struct type presents, sorted by key.
func StructFields(t reflect.Type) []StructField { return structOf(t).fields }

func structOf(t reflect.Type) *structInfo {
	if si, ok := structCache.Load(t); ok {
		return si.(*structInfo)
	}
	si := &structInfo{byKey: map[string]int{}}
	si.fields = collectFields(t)
	sort.Slice(si.fields, func(i, j int) bool { return si.fields[i].Key < si.fields[j].Key })
	for i, f := range si.fields {
		si.byKey[f.Key] = i
	}
	actual, _ := structCache.LoadOrStore(t, si)
	return actual.(*structInfo)
}

type fieldCand struct {
	StructField
	depth  int
	tagged bool
}

// collectFields follows encoding/json: exported fields; the fields of an
// embedded struct promoted, the shallower winning, and of two at one depth
// the tagged one, else neither.
func collectFields(t reflect.Type) []StructField {
	var cands []fieldCand
	type level struct {
		t     reflect.Type
		index []int
		ptr   bool
		unexp bool
	}
	queue := []level{{t: t}}
	visited := map[reflect.Type]bool{}
	for depth := 0; len(queue) > 0; depth++ {
		var next []level
		for _, lv := range queue {
			if visited[lv.t] {
				continue
			}
			visited[lv.t] = true
			for i := 0; i < lv.t.NumField(); i++ {
				sf := lv.t.Field(i)
				name, opts, tagged, skip := fieldName(sf)
				if skip {
					continue
				}
				index := append(append([]int(nil), lv.index...), i)
				ft := sf.Type
				if sf.Anonymous && !tagged {
					et := ft
					ptr := lv.ptr
					unexp := lv.unexp || !sf.IsExported()
					if et.Kind() == reflect.Pointer {
						et = et.Elem()
						ptr = true
					}
					if et.Kind() == reflect.Struct && !implementsHost(et) {
						next = append(next, level{t: et, index: index, ptr: ptr, unexp: unexp})
						continue
					}
				}
				if !sf.IsExported() {
					continue
				}
				cands = append(cands, fieldCand{StructField: StructField{
					Key: name, Index: index, Type: ft,
					OmitEmpty: strings.Contains(opts, ",omitempty"), OmitZero: strings.Contains(opts, ",omitzero"),
					ViaPointer: lv.ptr, ViaUnexported: lv.unexp,
				}, depth: depth, tagged: tagged})
			}
		}
		queue = next
	}
	byName := map[string][]fieldCand{}
	var order []string
	for _, c := range cands {
		if _, ok := byName[c.Key]; !ok {
			order = append(order, c.Key)
		}
		byName[c.Key] = append(byName[c.Key], c)
	}
	var out []StructField
	for _, name := range order {
		cs := byName[name]
		best := cs[0].depth
		var top []fieldCand
		for _, c := range cs {
			if c.depth < best {
				best, top = c.depth, nil
			}
			if c.depth == best {
				top = append(top, c)
			}
		}
		if len(top) == 1 {
			out = append(out, top[0].StructField)
			continue
		}
		var tagged []fieldCand
		for _, c := range top {
			if c.tagged {
				tagged = append(tagged, c)
			}
		}
		if len(tagged) == 1 {
			out = append(out, tagged[0].StructField)
		}
	}
	return out
}

// fieldName gives a field's key (6.2 item 2) and its json options.
func fieldName(sf reflect.StructField) (name, opts string, tagged, skip bool) {
	if tag, ok := sf.Tag.Lookup("risor"); ok {
		n, _, _ := strings.Cut(tag, ",")
		if n == "-" {
			return "", "", false, true
		}
		if n != "" {
			name, tagged = n, true
		}
	}
	if tag, ok := sf.Tag.Lookup("json"); ok {
		n, rest, _ := strings.Cut(tag, ",")
		if n == "-" && rest == "" {
			return "", "", false, true
		}
		opts = "," + rest
		if name == "" && n != "" {
			name, tagged = n, true
		}
	}
	if name == "" {
		name = sf.Name
	}
	return name, opts, tagged, false
}

// implementsHost reports whether a type is presented by a rule before the
// struct rule: opaque, or a text type.
func implementsHost(t reflect.Type) bool {
	return t.Implements(opaqueType) || t.Implements(textMarshaler)
}

var opaqueType = reflect.TypeFor[Opaque]()

// field returns a struct value's field, and whether it is present.
func field(rv reflect.Value, f *StructField) (reflect.Value, bool) {
	v := rv
	if f.ViaUnexported && !v.CanAddr() {
		c := reflect.New(v.Type()).Elem()
		c.Set(v)
		v = c
	}
	for i, x := range f.Index {
		if i > 0 && v.Kind() == reflect.Pointer {
			if v.IsNil() {
				return reflect.Value{}, false
			}
			v = v.Elem()
		}
		v = v.Field(x)
	}
	if f.ViaUnexported && !v.CanInterface() {
		// The field is exported; only the path to it is not.
		v = reflect.NewAt(v.Type(), unsafe.Pointer(v.UnsafeAddr())).Elem()
	}
	if (f.OmitEmpty && isEmptyValue(v)) || (f.OmitZero && isZeroValue(v)) {
		return reflect.Value{}, false
	}
	return v, true
}

// isEmptyValue is what encoding/json leaves out under omitempty.
func isEmptyValue(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.Array, reflect.Map, reflect.Slice, reflect.String:
		return v.Len() == 0
	case reflect.Bool:
		return !v.Bool()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return v.Int() == 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return v.Uint() == 0
	case reflect.Float32, reflect.Float64:
		return v.Float() == 0
	case reflect.Interface, reflect.Pointer:
		return v.IsNil()
	}
	return false
}

// isZeroValue is what encoding/json leaves out under omitzero: an IsZero
// method's answer, else the zero value.
func isZeroValue(v reflect.Value) bool {
	if z, ok := v.Interface().(interface{ IsZero() bool }); ok {
		if v.Kind() == reflect.Pointer && v.IsNil() {
			return true
		}
		return z.IsZero()
	}
	return v.IsZero()
}

func isStruct(v any) (reflect.Value, *structInfo, bool) {
	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Struct {
		return rv, nil, false
	}
	return rv, structOf(rv.Type()), true
}

func structLen(rv reflect.Value, si *structInfo) int {
	n := 0
	for i := range si.fields {
		if _, ok := field(rv, &si.fields[i]); ok {
			n++
		}
	}
	return n
}

func structGet(rv reflect.Value, si *structInfo, k string) (any, bool) {
	i, ok := si.byKey[k]
	if !ok {
		return nil, false
	}
	fv, ok := field(rv, &si.fields[i])
	if !ok {
		return nil, false
	}
	return fv.Interface(), true
}

func structKeys(rv reflect.Value, si *structInfo) []string {
	var keys []string
	for i := range si.fields {
		if _, ok := field(rv, &si.fields[i]); ok {
			keys = append(keys, si.fields[i].Key)
		}
	}
	return keys
}

// marshalText presents a text type (6.2): the text MarshalText gives.
func marshalText(m encoding.TextMarshaler) string {
	b, err := m.MarshalText()
	if err != nil {
		return ""
	}
	return string(b)
}
