package mcp

import (
	"encoding/json"
	"fmt"
	"slices"
	"sort"
	"strings"
)

// validateArgs holds a call's arguments to the input schema the registry
// advertises. The schemas use one slice of JSON Schema — an object with named
// properties that are strings, booleans, integers, string enums, or arrays of
// strings, with a required list and additionalProperties false — and the
// check walks exactly that slice, so an argument the schema does not allow is
// refused before a tool runs and the caller is told which argument offended.
//
// This is the protocol's own gate, distinct from what a tool says about the
// KB: an argument that does not match the schema never reaches a handler and
// is a JSON-RPC error, while everything a handler finds is a result.
func validateArgs(schema map[string]any, raw json.RawMessage) error {
	props, _ := schema["properties"].(map[string]any)

	args := map[string]json.RawMessage{}
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &args); err != nil {
			return fmt.Errorf("arguments are not an object")
		}
	}

	// Unknown arguments are refused by name. additionalProperties is false on
	// every schema this package writes; a caller that sends a mistyped
	// argument is better served by the refusal than by silence.
	for name := range args {
		if _, known := props[name]; !known {
			return fmt.Errorf("unknown argument %q", name)
		}
	}
	required, _ := schema["required"].([]string)
	for _, name := range required {
		if _, present := args[name]; !present {
			return fmt.Errorf("missing required argument %q", name)
		}
	}

	names := make([]string, 0, len(args))
	for name := range args {
		names = append(names, name)
	}
	// Sorted so a refusal that names several arguments is deterministic.
	sort.Strings(names)
	for _, name := range names {
		if err := validateOne(name, props[name], args[name]); err != nil {
			return err
		}
	}
	return nil
}

// validateOne checks one argument against one property schema. A property the
// schema describes but the walk does not model cannot occur — the registry's
// helper functions write the five shapes — so falling through reports the
// argument as unusable rather than accepting it unchecked.
func validateOne(name string, schema any, raw json.RawMessage) error {
	s, ok := schema.(map[string]any)
	if !ok {
		return fmt.Errorf("argument %q cannot be checked against its schema", name)
	}
	typ, _ := s["type"].(string)
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return fmt.Errorf("argument %q is malformed", name)
	}
	switch typ {
	case "string":
		str, ok := v.(string)
		if !ok {
			return fmt.Errorf("argument %q is not a string", name)
		}
		if values, ok := s["enum"].([]string); ok && len(values) > 0 {
			if !slices.Contains(values, str) {
				return fmt.Errorf("argument %q is %q; it is one of %s", name, str, strings.Join(quoteAll(values), ", "))
			}
		}
	case "boolean":
		if _, ok := v.(bool); !ok {
			return fmt.Errorf("argument %q is not a boolean", name)
		}
	case "integer":
		num, ok := v.(float64)
		if !ok || num != float64(int64(num)) {
			return fmt.Errorf("argument %q is not an integer", name)
		}
		if min, ok := s["minimum"].(int); ok && num < float64(min) {
			return fmt.Errorf("argument %q is %d; it is %d or more", name, int64(num), min)
		}
	case "array":
		items, ok := v.([]any)
		if !ok {
			return fmt.Errorf("argument %q is not an array", name)
		}
		for _, item := range items {
			if _, ok := item.(string); !ok {
				return fmt.Errorf("argument %q holds something that is not a string", name)
			}
		}
	default:
		return fmt.Errorf("argument %q has an unusable schema", name)
	}
	return nil
}

func quoteAll(values []string) []string {
	out := make([]string, len(values))
	for i, v := range values {
		out[i] = fmt.Sprintf("%q", v)
	}
	return out
}
