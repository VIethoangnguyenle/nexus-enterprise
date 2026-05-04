// Package domain provides the condition matching engine for template resolution.
// Evaluates JSONB field conditions against entity data to find the best template.
package domain

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// EntityFields represents the entity data used for template matching.
// Keys are field names (e.g. "amount", "service_type"), values are string representations.
type EntityFields map[string]string

// MatchConditions evaluates whether all conditions on a template are satisfied
// by the given entity fields. Returns true only if ALL conditions pass.
func MatchConditions(conditions []*Condition, fields EntityFields) bool {
	for _, cond := range conditions {
		val, exists := fields[cond.Field]
		if !exists {
			return false
		}
		if !evaluateCondition(cond, val) {
			return false
		}
	}
	return true
}

// ResolveTemplate finds the highest-priority active template that matches
// the given entity type and field values.
func (s *Service) ResolveTemplate(ctx context.Context, entityType string, fields EntityFields) (*Template, error) {
	templates, err := s.store.ListTemplates(ctx, entityType, true)
	if err != nil {
		return nil, fmt.Errorf("list templates: %w", err)
	}

	var best *Template
	for _, t := range templates {
		if !MatchConditions(t.Conditions, fields) {
			continue
		}
		if best == nil || t.Priority > best.Priority {
			best = t
		}
	}

	if best == nil {
		return nil, ErrNoMatchingTemplate
	}

	// ListTemplates returns lightweight records without steps/conditions.
	// Reload the full template so that snapshot includes steps for assignment.
	full, err := s.store.GetTemplate(ctx, best.ID)
	if err != nil {
		return nil, fmt.Errorf("reload matched template: %w", err)
	}
	return full, nil
}

// evaluateCondition checks a single condition against a field value.
func evaluateCondition(cond *Condition, fieldValue string) bool {
	switch cond.Operator {
	case "eq":
		return evalEq(cond.Value, fieldValue)
	case "gt":
		return evalCompare(cond.Value, fieldValue, func(a, b float64) bool { return b > a })
	case "gte":
		return evalCompare(cond.Value, fieldValue, func(a, b float64) bool { return b >= a })
	case "lt":
		return evalCompare(cond.Value, fieldValue, func(a, b float64) bool { return b < a })
	case "lte":
		return evalCompare(cond.Value, fieldValue, func(a, b float64) bool { return b <= a })
	case "in":
		return evalIn(cond.Value, fieldValue)
	case "between":
		return evalBetween(cond.Value, fieldValue)
	default:
		return false
	}
}

// evalEq checks equality. Condition value is a JSON scalar (string or number).
func evalEq(condValue, fieldValue string) bool {
	var expected string
	if err := json.Unmarshal([]byte(condValue), &expected); err != nil {
		return condValue == fieldValue
	}
	return expected == fieldValue
}

// evalCompare performs numeric comparison using the provided comparator.
func evalCompare(condValue, fieldValue string, cmp func(a, b float64) bool) bool {
	var threshold float64
	if err := json.Unmarshal([]byte(condValue), &threshold); err != nil {
		return false
	}
	actual, err := strconv.ParseFloat(fieldValue, 64)
	if err != nil {
		return false
	}
	return cmp(threshold, actual)
}

// evalIn checks if the field value is in the JSON array condition value.
// Condition value format: ["value1", "value2", "value3"]
func evalIn(condValue, fieldValue string) bool {
	var allowed []string
	if err := json.Unmarshal([]byte(condValue), &allowed); err != nil {
		return false
	}
	for _, a := range allowed {
		if a == fieldValue {
			return true
		}
	}
	return false
}

// evalBetween checks if the numeric field is within [min, max].
// Condition value format: [min, max] as JSON array of numbers.
func evalBetween(condValue, fieldValue string) bool {
	var bounds [2]float64
	if err := json.Unmarshal([]byte(condValue), &bounds); err != nil {
		return false
	}
	actual, err := strconv.ParseFloat(fieldValue, 64)
	if err != nil {
		return false
	}
	return actual >= bounds[0] && actual <= bounds[1]
}

// ResolvePlaceholder replaces template placeholders with actual values.
// Supported: {creator_dept} → resolved from creator's department UA.
func ResolvePlaceholder(value string, creatorDeptID string) string {
	return strings.ReplaceAll(value, "{creator_dept}", creatorDeptID)
}
