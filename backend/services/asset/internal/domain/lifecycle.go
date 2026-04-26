package domain

import (
	"encoding/json"
	"fmt"
)

// LifecycleDefinition describes the state machine for an asset type.
type LifecycleDefinition struct {
	States       []string         `json:"states"`
	InitialState string           `json:"initial_state"`
	Transitions  []TransitionRule `json:"transitions"`
}

// TransitionRule defines a single permitted state transition.
type TransitionRule struct {
	FromState      string `json:"from_state"`
	ToState        string `json:"to_state"`
	Operation      string `json:"operation"`
	NgacPermission string `json:"ngac_permission"`
}

// DefaultLifecycle returns the standard lifecycle for generic asset types.
func DefaultLifecycle() LifecycleDefinition {
	return LifecycleDefinition{
		States:       []string{"requested", "available", "assigned", "maintenance", "retired", "disposed"},
		InitialState: "available",
		Transitions: []TransitionRule{
			{FromState: "requested", ToState: "available", Operation: "approve", NgacPermission: "approve"},
			{FromState: "available", ToState: "assigned", Operation: "assign", NgacPermission: "assign"},
			{FromState: "assigned", ToState: "available", Operation: "return", NgacPermission: "assign"},
			{FromState: "assigned", ToState: "maintenance", Operation: "flag_maintenance", NgacPermission: "manage"},
			{FromState: "available", ToState: "maintenance", Operation: "flag_maintenance", NgacPermission: "manage"},
			{FromState: "maintenance", ToState: "available", Operation: "complete_maintenance", NgacPermission: "manage"},
			{FromState: "available", ToState: "retired", Operation: "retire", NgacPermission: "manage"},
			{FromState: "maintenance", ToState: "retired", Operation: "retire", NgacPermission: "manage"},
			{FromState: "retired", ToState: "disposed", Operation: "dispose", NgacPermission: "dispose"},
		},
	}
}

// ValidateLifecycle checks that a lifecycle definition is structurally valid:
// all transition states exist, initial state exists, no orphan states.
func ValidateLifecycle(ld LifecycleDefinition) error {
	if len(ld.States) == 0 {
		return fmt.Errorf("lifecycle must have at least one state")
	}

	stateSet := make(map[string]bool, len(ld.States))
	for _, s := range ld.States {
		stateSet[s] = true
	}

	if !stateSet[ld.InitialState] {
		return fmt.Errorf("initial_state %q not in states list", ld.InitialState)
	}

	for i, t := range ld.Transitions {
		if !stateSet[t.FromState] {
			return fmt.Errorf("transition[%d]: from_state %q not in states", i, t.FromState)
		}
		if !stateSet[t.ToState] {
			return fmt.Errorf("transition[%d]: to_state %q not in states", i, t.ToState)
		}
		if t.Operation == "" {
			return fmt.Errorf("transition[%d]: operation is required", i)
		}
		if t.NgacPermission == "" {
			return fmt.Errorf("transition[%d]: ngac_permission is required", i)
		}
	}

	// Check reachability: every state should be reachable from initial or lead somewhere
	reachable := make(map[string]bool)
	reachable[ld.InitialState] = true
	changed := true
	for changed {
		changed = false
		for _, t := range ld.Transitions {
			if reachable[t.FromState] && !reachable[t.ToState] {
				reachable[t.ToState] = true
				changed = true
			}
		}
	}
	for _, s := range ld.States {
		if !reachable[s] {
			return fmt.Errorf("state %q is unreachable from initial state %q", s, ld.InitialState)
		}
	}

	return nil
}

// ParseLifecycle unmarshals a JSON lifecycle definition with validation.
func ParseLifecycle(data json.RawMessage) (LifecycleDefinition, error) {
	var ld LifecycleDefinition
	if err := json.Unmarshal(data, &ld); err != nil {
		return ld, fmt.Errorf("parsing lifecycle JSON: %w", err)
	}
	if err := ValidateLifecycle(ld); err != nil {
		return ld, fmt.Errorf("invalid lifecycle: %w", err)
	}
	return ld, nil
}

// AvailableTransitions returns all valid transitions from the current state.
func AvailableTransitions(ld LifecycleDefinition, currentState string) []TransitionRule {
	var result []TransitionRule
	for _, t := range ld.Transitions {
		if t.FromState == currentState {
			result = append(result, t)
		}
	}
	return result
}

// FindTransition looks up a specific transition by operation name from the current state.
func FindTransition(ld LifecycleDefinition, currentState, operation string) (TransitionRule, bool) {
	for _, t := range ld.Transitions {
		if t.FromState == currentState && t.Operation == operation {
			return t, true
		}
	}
	return TransitionRule{}, false
}
