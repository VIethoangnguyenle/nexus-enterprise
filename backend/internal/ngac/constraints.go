package ngac

import (
	"time"
)

// RequestContext holds information about the current access request
type RequestContext struct {
	Time      time.Time
	UserID    string
	ObjectID  string
	Operation string
}

// PolicyConstraint is a dynamic constraint evaluated after graph traversal
type PolicyConstraint struct {
	Name       string
	Operations []string // which operations this constraint applies to
	Evaluate   func(ctx RequestContext) (denied bool, message string)
}

// ConstraintEngine manages and evaluates policy constraints
type ConstraintEngine struct {
	constraints []*PolicyConstraint
}

func NewConstraintEngine() *ConstraintEngine {
	return &ConstraintEngine{}
}

// Register adds a constraint to the engine
func (ce *ConstraintEngine) Register(c *PolicyConstraint) {
	ce.constraints = append(ce.constraints, c)
}

// Evaluate checks all applicable constraints, returns denial info if any constraint fires
func (ce *ConstraintEngine) Evaluate(ctx RequestContext) (denied bool, constraintName string, message string, checked []string) {
	for _, c := range ce.constraints {
		if !operationMatches(c.Operations, ctx.Operation) {
			continue
		}
		checked = append(checked, c.Name)
		if d, msg := c.Evaluate(ctx); d {
			return true, c.Name, msg, checked
		}
	}
	return false, "", "", checked
}

func operationMatches(ops []string, target string) bool {
	for _, op := range ops {
		if op == target {
			return true
		}
	}
	return false
}

// WeekdayOnlyConstraint denies write/upload on weekends
var WeekdayOnlyConstraint = &PolicyConstraint{
	Name:       "weekday-only-editing",
	Operations: []string{"write", "upload"},
	Evaluate: func(ctx RequestContext) (bool, string) {
		day := ctx.Time.Weekday()
		if day == time.Saturday || day == time.Sunday {
			return true, "Editing is only allowed Monday-Friday"
		}
		return false, ""
	},
}
