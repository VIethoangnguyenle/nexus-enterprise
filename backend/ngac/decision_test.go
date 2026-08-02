package ngac_test

import (
	"errors"
	"testing"

	"ngac-platform/ngac"
)

// The system's default is DENY. Every PEP interprets a policy response, and
// each one that re-implements that interpretation is a chance to get it
// backwards. These cases pin the only interpretation that is allowed to exist.
func TestAllowed(t *testing.T) {
	cases := []struct {
		name     string
		decision string
		err      error
		want     bool
	}{
		{"explicit allow", ngac.DecisionAllow, nil, true},
		{"explicit deny", ngac.DecisionDeny, nil, false},

		// Fail-closed: a policy service that is down, slow, or unreachable
		// must not grant access. This is the case messaging got wrong.
		{"transport error denies", ngac.DecisionAllow, errors.New("connection refused"), false},
		{"transport error with deny", ngac.DecisionDeny, errors.New("deadline exceeded"), false},

		// A nil response yields "" through the generated GetDecision() getter.
		{"empty decision denies", "", nil, false},

		// Anything the PDP did not explicitly authorize is a denial.
		{"unknown decision denies", "MAYBE", nil, false},
		{"lowercase allow denies", "allow", nil, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ngac.Allowed(tc.decision, tc.err); got != tc.want {
				t.Errorf("Allowed(%q, %v) = %v, want %v", tc.decision, tc.err, got, tc.want)
			}
		})
	}
}
