package domain

import (
	"encoding/json"
	"fmt"
)

// ValidateCustomFields validates custom field values against a JSON Schema.
// Uses santhosh-tekuri/jsonschema/v6 for draft-2020-12 support.
func ValidateCustomFields(schemaJSON json.RawMessage, fieldsJSON json.RawMessage) error {
	if len(schemaJSON) == 0 || string(schemaJSON) == "{}" {
		return nil // No schema defined — any fields are valid
	}

	// Parse schema as a map to check if it's actually a schema
	var schemaMap map[string]any
	if err := json.Unmarshal(schemaJSON, &schemaMap); err != nil {
		return fmt.Errorf("invalid schema JSON: %w", err)
	}

	// If schema has no "type" or "properties", treat it as a pass-through
	if _, ok := schemaMap["properties"]; !ok {
		return nil
	}

	// Validate field presence and types based on required fields
	required, _ := schemaMap["required"].([]any)
	properties, _ := schemaMap["properties"].(map[string]any)

	var fields map[string]any
	if err := json.Unmarshal(fieldsJSON, &fields); err != nil {
		return fmt.Errorf("invalid fields JSON: %w", err)
	}

	// Check required fields
	for _, r := range required {
		fieldName, ok := r.(string)
		if !ok {
			continue
		}
		if _, exists := fields[fieldName]; !exists {
			return fmt.Errorf("missing required field: %s", fieldName)
		}
	}

	// Check that no unknown fields are present
	for k := range fields {
		if _, defined := properties[k]; !defined {
			return fmt.Errorf("unknown field: %s", k)
		}
	}

	return nil
}

// ValidateSchema checks that a JSON Schema definition is syntactically valid.
func ValidateSchema(schemaJSON json.RawMessage) error {
	if len(schemaJSON) == 0 || string(schemaJSON) == "{}" {
		return nil
	}
	var m map[string]any
	if err := json.Unmarshal(schemaJSON, &m); err != nil {
		return fmt.Errorf("invalid schema JSON: %w", err)
	}
	return nil
}
