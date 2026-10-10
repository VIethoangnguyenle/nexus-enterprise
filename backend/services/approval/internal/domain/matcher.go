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

	// ListTemplates returns lightweight records (conditions, no steps).
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

// parseForm reads the submitted form (label → value). Anything unreadable is
// no values: conditions that need a value then fail to match.
func parseForm(formDataJSON string) map[string]string {
	out := map[string]string{}
	if formDataJSON == "" {
		return out
	}
	var raw map[string]any
	if err := json.Unmarshal([]byte(formDataJSON), &raw); err != nil {
		return out
	}
	for k, v := range raw {
		switch x := v.(type) {
		case string:
			out[k] = x
		case float64:
			out[k] = strconv.FormatFloat(x, 'f', -1, 64)
		case bool:
			out[k] = strconv.FormatBool(x)
		}
	}
	return out
}

// fieldsFor derives the values a template's conditions are judged on from what
// was submitted: every answer by its label, and the first currency answer also
// as "amount". The client's own idea of the figures plays no part.
// trusted holds fields supplied by internal callers (gRPC), which win where
// they overlap; the REST layer passes none.
func fieldsFor(t *Template, form map[string]string, trusted EntityFields) EntityFields {
	out := EntityFields{}
	for k, v := range form {
		out[k] = v
	}
	for _, f := range t.FormFields {
		if f.FieldType == "currency" {
			if v, ok := form[f.Label]; ok && v != "" {
				if _, taken := out["amount"]; !taken {
					out["amount"] = v
				}
				break
			}
		}
	}
	for k, v := range trusted {
		out[k] = v
	}
	return out
}

// bestMatch returns the highest-priority template whose conditions hold on the form.
func bestMatch(templates []*Template, form map[string]string, trusted EntityFields) *Template {
	var pick *Template
	for _, t := range templates {
		if !MatchConditions(t.Conditions, fieldsFor(t, form, trusted)) {
			continue
		}
		if pick == nil || t.Priority > pick.Priority {
			pick = t
		}
	}
	return pick
}

// resolveForForm finds the template for an entity type from what was submitted.
func (s *Service) resolveForForm(ctx context.Context, entityType string, form map[string]string, trusted EntityFields) (*Template, error) {
	templates, err := s.store.ListTemplates(ctx, entityType, true)
	if err != nil {
		return nil, fmt.Errorf("list templates: %w", err)
	}
	pick := bestMatch(templates, form, trusted)
	if pick == nil {
		return nil, ErrNoMatchingTemplate
	}
	full, err := s.store.GetTemplate(ctx, pick.ID)
	if err != nil {
		return nil, fmt.Errorf("reload matched template: %w", err)
	}
	return full, nil
}

// checkChosen accepts a template the submitter picked only if it is the one the
// form would have been routed to: it is active, its conditions hold on the
// form, and no higher-priority active template of the same type also matches.
// Choosing a template is a way to name which of the matching ones, never a way
// around the conditions or the priorities.
func (s *Service) checkChosen(ctx context.Context, t *Template, form map[string]string, trusted EntityFields) error {
	if !t.IsActive {
		return fmt.Errorf("template is not active: %w", ErrInvalidInput)
	}
	if !MatchConditions(t.Conditions, fieldsFor(t, form, trusted)) {
		return fmt.Errorf("the form does not meet this template's conditions: %w", ErrInvalidInput)
	}
	others, err := s.store.ListTemplates(ctx, t.EntityType, true)
	if err != nil {
		return fmt.Errorf("list templates: %w", err)
	}
	for _, o := range others {
		if o.ID != t.ID && o.Priority > t.Priority && MatchConditions(o.Conditions, fieldsFor(o, form, trusted)) {
			return fmt.Errorf("a higher-priority template applies to this form: %w", ErrInvalidInput)
		}
	}
	return nil
}
