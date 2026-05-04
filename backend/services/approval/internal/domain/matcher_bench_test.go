package domain

import (
	"testing"
)

func BenchmarkMatchConditions_SimpleEq(b *testing.B) {
	conditions := []*Condition{
		{Field: "category", Operator: "eq", Value: "transfer"},
	}
	fields := EntityFields{"category": "transfer"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		MatchConditions(conditions, fields)
	}
}

func BenchmarkMatchConditions_ComplexMixed(b *testing.B) {
	conditions := []*Condition{
		{Field: "amount", Operator: "gt", Value: "1000000"},
		{Field: "category", Operator: "in", Value: `["transfer","payment","refund"]`},
		{Field: "priority", Operator: "between", Value: `[1, 10]`},
		{Field: "service_type", Operator: "eq", Value: "banking"},
	}
	fields := EntityFields{
		"amount":       "5000000",
		"category":     "transfer",
		"priority":     "5",
		"service_type": "banking",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		MatchConditions(conditions, fields)
	}
}

func BenchmarkMatchConditions_EarlyMismatch(b *testing.B) {
	conditions := []*Condition{
		{Field: "amount", Operator: "gt", Value: "1000000"},
		{Field: "category", Operator: "eq", Value: "refund"},
	}
	fields := EntityFields{
		"amount":   "500", // fails first condition
		"category": "transfer",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		MatchConditions(conditions, fields)
	}
}
