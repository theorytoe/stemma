package mcp

import (
	"reflect"
	"strings"
)

// Shape describes the JSON a payload marshals to, read off the struct's tags
// rather than written beside them, so a payload and its description cannot
// drift apart. The vocabulary is JSON Schema's — type, properties, items,
// required — but only the slice a payload needs: no patterns, no formats, no
// validation. A shape says what a consumer will see; the input schemas say
// what a caller may send.
//
// A field counts as required when its tag lacks omitempty, which is exactly
// when encoding/json always writes it. A nilable slice is described as an
// array all the same: a shape documents the intended contents, and the payload
// builders keep their slices non-nil where emptiness would be misleading.
func Shape(v any) map[string]any {
	return shapeOf(reflect.TypeOf(v))
}

// shapeOf walks one type. An unexported or anonymously-tagged field is
// skipped; an interface is described by an empty schema, the JSON Schema way
// of saying "anything".
func shapeOf(t reflect.Type) map[string]any {
	for t != nil && t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if t == nil {
		return map[string]any{}
	}
	switch t.Kind() {
	case reflect.Struct:
		props := map[string]any{}
		var required []string
		for i := 0; i < t.NumField(); i++ {
			f := t.Field(i)
			if !f.IsExported() {
				continue
			}
			name, omit := jsonName(f)
			if name == "-" {
				continue
			}
			props[name] = shapeOf(f.Type)
			if !omit {
				required = append(required, name)
			}
		}
		m := map[string]any{"type": "object", "properties": props}
		if required != nil {
			m["required"] = required
		}
		return m
	case reflect.Slice, reflect.Array:
		return map[string]any{"type": "array", "items": shapeOf(t.Elem())}
	case reflect.Map:
		return map[string]any{"type": "object", "additionalProperties": shapeOf(t.Elem())}
	case reflect.Bool:
		return map[string]any{"type": "boolean"}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return map[string]any{"type": "integer"}
	case reflect.Float32, reflect.Float64:
		return map[string]any{"type": "number"}
	case reflect.String:
		return map[string]any{"type": "string"}
	default:
		return map[string]any{}
	}
}

// jsonName reads a field's wire name and whether it may be absent, from the
// encoding/json tag. An untagged field travels under its Go name, as encoding
// //json sends it.
func jsonName(f reflect.StructField) (name string, omitempty bool) {
	tag := f.Tag.Get("json")
	if tag == "" {
		return f.Name, false
	}
	parts := strings.Split(tag, ",")
	name = parts[0]
	if name == "" {
		name = f.Name
	}
	for _, opt := range parts[1:] {
		if opt == "omitempty" {
			omitempty = true
		}
	}
	return name, omitempty
}
