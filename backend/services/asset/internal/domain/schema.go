package domain

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

// A type's custom fields are a JSON-Schema-shaped object:
//
//	{"type": "object",
//	 "properties": {"<key>": {"title": "Cấu hình", "type": "string", "x-kind": "text"}, ...},
//	 "required": ["<key>"]}
//
// <key> is an opaque, stable name the client picks; title is what people read.
// x-kind is one of the kinds below; a "choice" lists its options in "enum".
// A property with no x-kind (a schema written before kinds existed) is only
// checked for being present when required and for being a known field.
const (
	KindText   = "text"
	KindNumber = "number"
	KindDate   = "date"
	KindPerson = "person" // the value is a user ID, picked from the workspace's people
	KindChoice = "choice"

	maxFields     = 20
	maxTitleRunes = 80
	maxValueRunes = 500
)

func knownKind(k string) bool {
	switch k {
	case KindText, KindNumber, KindDate, KindPerson, KindChoice:
		return true
	}
	return false
}

// ValidateCustomFields validates custom field values against a type's schema.
func ValidateCustomFields(schemaJSON json.RawMessage, fieldsJSON json.RawMessage) error {
	var fields map[string]any
	if len(fieldsJSON) > 0 {
		if err := json.Unmarshal(fieldsJSON, &fields); err != nil {
			return fmt.Errorf("invalid fields JSON: %w", err)
		}
	}

	var schemaMap map[string]any
	if len(schemaJSON) > 0 {
		if err := json.Unmarshal(schemaJSON, &schemaMap); err != nil {
			return fmt.Errorf("invalid schema JSON: %w", err)
		}
	}
	// A type with no fields defined holds no custom values: an unbounded bag of
	// unvalidated JSON is not "any field is valid".
	properties, ok := schemaMap["properties"].(map[string]any)
	if !ok || len(properties) == 0 {
		if len(fields) > 0 {
			return fmt.Errorf("unknown field: this type has no custom fields")
		}
		return nil
	}
	required, _ := schemaMap["required"].([]any)

	for _, r := range required {
		if name, ok := r.(string); ok {
			v, exists := fields[name]
			if !exists {
				return fmt.Errorf("missing required field: %s", name)
			}
			// Present but empty is not filled in.
			if str, isStr := v.(string); isStr && strings.TrimSpace(str) == "" {
				return fmt.Errorf("missing required field: %s", name)
			}
		}
	}

	for k, v := range fields {
		def, defined := properties[k].(map[string]any)
		if !defined {
			return fmt.Errorf("unknown field: %s", k)
		}
		if err := checkValue(k, def, v); err != nil {
			return err
		}
	}
	return nil
}

func checkValue(key string, def map[string]any, v any) error {
	kind, _ := def["x-kind"].(string)
	switch kind {
	case "":
		return nil
	case KindNumber:
		if _, ok := v.(float64); !ok {
			return fmt.Errorf("field %s must be a number", key)
		}
		return nil
	}
	s, ok := v.(string)
	if !ok {
		return fmt.Errorf("field %s must be text", key)
	}
	if utf8.RuneCountInString(s) > maxValueRunes {
		return fmt.Errorf("field %s is too long", key)
	}
	switch kind {
	case KindDate:
		if _, err := time.Parse("2006-01-02", s); err != nil {
			return fmt.Errorf("field %s must be a date (YYYY-MM-DD)", key)
		}
	case KindChoice:
		options, _ := def["enum"].([]any)
		for _, o := range options {
			if o == s {
				return nil
			}
		}
		return fmt.Errorf("field %s is not one of its options", key)
	}
	return nil
}

// ValidateSchema checks that a custom-field schema is well formed: JSON, at
// most maxFields fields, each with a readable title and a known kind, choices
// that list distinct options, and a "required" list naming fields that exist.
func ValidateSchema(schemaJSON json.RawMessage) error {
	if len(schemaJSON) == 0 || string(schemaJSON) == "{}" {
		return nil
	}
	var m map[string]any
	if err := json.Unmarshal(schemaJSON, &m); err != nil {
		return fmt.Errorf("invalid schema JSON: %w", err)
	}
	rawProps, has := m["properties"]
	if !has {
		return nil
	}
	props, ok := rawProps.(map[string]any)
	if !ok {
		return fmt.Errorf("properties must be an object")
	}
	if len(props) > maxFields {
		return fmt.Errorf("a type may have at most %d fields", maxFields)
	}
	for key, raw := range props {
		def, ok := raw.(map[string]any)
		if !ok {
			return fmt.Errorf("field %s must be an object", key)
		}
		if title, present := def["title"]; present {
			t, ok := title.(string)
			if !ok || utf8.RuneCountInString(t) > maxTitleRunes {
				return fmt.Errorf("field %s: title must be text of at most %d characters", key, maxTitleRunes)
			}
		}
		kindRaw, present := def["x-kind"]
		if !present {
			continue
		}
		kind, _ := kindRaw.(string)
		if !knownKind(kind) {
			return fmt.Errorf("field %s: unknown kind %v", key, kindRaw)
		}
		if kind == KindChoice {
			if err := checkOptions(key, def["enum"]); err != nil {
				return err
			}
		}
	}
	if rawReq, present := m["required"]; present {
		req, ok := rawReq.([]any)
		if !ok {
			return fmt.Errorf("required must be a list")
		}
		for _, r := range req {
			name, _ := r.(string)
			if _, defined := props[name]; !defined {
				return fmt.Errorf("required names a field that does not exist")
			}
		}
	}
	return nil
}

func checkOptions(key string, raw any) error {
	options, ok := raw.([]any)
	if !ok || len(options) == 0 {
		return fmt.Errorf("field %s: a choice needs at least one option", key)
	}
	seen := map[string]bool{}
	for _, o := range options {
		s, ok := o.(string)
		if !ok || s == "" || utf8.RuneCountInString(s) > maxTitleRunes {
			return fmt.Errorf("field %s: options must be non-empty text", key)
		}
		if seen[s] {
			return fmt.Errorf("field %s: options must be distinct", key)
		}
		seen[s] = true
	}
	return nil
}

// PersonValues returns the user IDs held in the person-kind fields of a value
// set, so the caller can check they are members of the right workspace.
func PersonValues(schemaJSON, fieldsJSON json.RawMessage) []string {
	var schema struct {
		Properties map[string]struct {
			Kind string `json:"x-kind"`
		} `json:"properties"`
	}
	var fields map[string]any
	if json.Unmarshal(schemaJSON, &schema) != nil || json.Unmarshal(fieldsJSON, &fields) != nil {
		return nil
	}
	var ids []string
	for key, def := range schema.Properties {
		if def.Kind != KindPerson {
			continue
		}
		if id, ok := fields[key].(string); ok && id != "" {
			ids = append(ids, id)
		}
	}
	return ids
}
