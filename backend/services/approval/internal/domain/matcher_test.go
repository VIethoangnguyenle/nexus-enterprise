package domain

import (
	"testing"
)

func TestMatchConditions_AllMatch(t *testing.T) {
	conditions := []*Condition{
		{Field: "amount", Operator: "gt", Value: "1000"},
		{Field: "service_type", Operator: "eq", Value: `"transfer"`},
	}
	fields := EntityFields{"amount": "5000", "service_type": "transfer"}
	if !MatchConditions(conditions, fields) {
		t.Error("expected match, got no match")
	}
}

func TestMatchConditions_OneFails(t *testing.T) {
	conditions := []*Condition{
		{Field: "amount", Operator: "gt", Value: "1000"},
		{Field: "service_type", Operator: "eq", Value: `"transfer"`},
	}
	fields := EntityFields{"amount": "500", "service_type": "transfer"}
	if MatchConditions(conditions, fields) {
		t.Error("expected no match (amount 500 < 1000), got match")
	}
}

func TestMatchConditions_MissingField(t *testing.T) {
	conditions := []*Condition{
		{Field: "amount", Operator: "gt", Value: "1000"},
	}
	fields := EntityFields{"service_type": "transfer"}
	if MatchConditions(conditions, fields) {
		t.Error("expected no match (missing field), got match")
	}
}

func TestMatchConditions_EmptyConditions(t *testing.T) {
	fields := EntityFields{"amount": "5000"}
	if !MatchConditions(nil, fields) {
		t.Error("empty conditions should match any fields")
	}
}

func TestEvalEq_StringMatch(t *testing.T) {
	tests := []struct {
		condValue  string
		fieldValue string
		want       bool
	}{
		{`"transfer"`, "transfer", true},
		{`"transfer"`, "deposit", false},
		{`"123"`, "123", true},
	}
	for _, tt := range tests {
		got := evalEq(tt.condValue, tt.fieldValue)
		if got != tt.want {
			t.Errorf("evalEq(%q, %q) = %v, want %v", tt.condValue, tt.fieldValue, got, tt.want)
		}
	}
}

func TestEvalCompare_GT(t *testing.T) {
	cmp := func(a, b float64) bool { return b > a }
	tests := []struct {
		threshold  string
		fieldValue string
		want       bool
	}{
		{"1000", "5000", true},
		{"1000", "1000", false},
		{"1000", "500", false},
	}
	for _, tt := range tests {
		got := evalCompare(tt.threshold, tt.fieldValue, cmp)
		if got != tt.want {
			t.Errorf("evalCompare(gt, %s, %s) = %v, want %v", tt.threshold, tt.fieldValue, got, tt.want)
		}
	}
}

func TestEvalCompare_GTE(t *testing.T) {
	cmp := func(a, b float64) bool { return b >= a }
	tests := []struct {
		threshold  string
		fieldValue string
		want       bool
	}{
		{"1000", "1000", true},
		{"1000", "1001", true},
		{"1000", "999", false},
	}
	for _, tt := range tests {
		got := evalCompare(tt.threshold, tt.fieldValue, cmp)
		if got != tt.want {
			t.Errorf("evalCompare(gte, %s, %s) = %v, want %v", tt.threshold, tt.fieldValue, got, tt.want)
		}
	}
}

func TestEvalCompare_LT(t *testing.T) {
	cmp := func(a, b float64) bool { return b < a }
	tests := []struct {
		threshold  string
		fieldValue string
		want       bool
	}{
		{"1000", "500", true},
		{"1000", "1000", false},
		{"1000", "1500", false},
	}
	for _, tt := range tests {
		got := evalCompare(tt.threshold, tt.fieldValue, cmp)
		if got != tt.want {
			t.Errorf("evalCompare(lt, %s, %s) = %v, want %v", tt.threshold, tt.fieldValue, got, tt.want)
		}
	}
}

func TestEvalCompare_InvalidNumber(t *testing.T) {
	cmp := func(a, b float64) bool { return b > a }
	if evalCompare("1000", "not-a-number", cmp) {
		t.Error("expected false for non-numeric field")
	}
	if evalCompare("not-a-number", "5000", cmp) {
		t.Error("expected false for non-numeric threshold")
	}
}

func TestEvalIn(t *testing.T) {
	tests := []struct {
		condValue  string
		fieldValue string
		want       bool
	}{
		{`["transfer","deposit","withdrawal"]`, "transfer", true},
		{`["transfer","deposit","withdrawal"]`, "deposit", true},
		{`["transfer","deposit","withdrawal"]`, "payment", false},
		{`["VIP"]`, "VIP", true},
		{`[]`, "anything", false},
	}
	for _, tt := range tests {
		got := evalIn(tt.condValue, tt.fieldValue)
		if got != tt.want {
			t.Errorf("evalIn(%q, %q) = %v, want %v", tt.condValue, tt.fieldValue, got, tt.want)
		}
	}
}

func TestEvalIn_InvalidJSON(t *testing.T) {
	if evalIn("not-json", "value") {
		t.Error("expected false for invalid JSON")
	}
}

func TestEvalBetween(t *testing.T) {
	tests := []struct {
		condValue  string
		fieldValue string
		want       bool
	}{
		{`[100, 1000]`, "500", true},
		{`[100, 1000]`, "100", true},  // inclusive lower bound
		{`[100, 1000]`, "1000", true}, // inclusive upper bound
		{`[100, 1000]`, "99", false},
		{`[100, 1000]`, "1001", false},
		{`[0, 0]`, "0", true},
	}
	for _, tt := range tests {
		got := evalBetween(tt.condValue, tt.fieldValue)
		if got != tt.want {
			t.Errorf("evalBetween(%q, %q) = %v, want %v", tt.condValue, tt.fieldValue, got, tt.want)
		}
	}
}

func TestEvalBetween_InvalidInput(t *testing.T) {
	if evalBetween("not-json", "500") {
		t.Error("expected false for invalid JSON bounds")
	}
	if evalBetween(`[100, 1000]`, "not-a-number") {
		t.Error("expected false for non-numeric field")
	}
}

func TestEvaluateCondition_UnknownOperator(t *testing.T) {
	cond := &Condition{Field: "x", Operator: "unknown", Value: "1"}
	if evaluateCondition(cond, "1") {
		t.Error("expected false for unknown operator")
	}
}

func TestResolvePlaceholder(t *testing.T) {
	tests := []struct {
		input  string
		deptID string
		want   string
	}{
		{"{creator_dept}", "dept_123", "dept_123"},
		{"managers_of_{creator_dept}", "dept_456", "managers_of_dept_456"},
		{"no_placeholder", "dept_789", "no_placeholder"},
		{"{creator_dept}_{creator_dept}", "d1", "d1_d1"},
	}
	for _, tt := range tests {
		got := ResolvePlaceholder(tt.input, tt.deptID)
		if got != tt.want {
			t.Errorf("ResolvePlaceholder(%q, %q) = %q, want %q", tt.input, tt.deptID, got, tt.want)
		}
	}
}
