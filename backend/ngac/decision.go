package ngac

// Allowed reports whether a policy response grants access.
//
// It is fail-closed by construction: an error from the policy call, a decision
// this package does not recognise, or the empty string a nil response yields
// through the generated GetDecision() accessor all deny. Only DecisionAllow
// allows.
//
// Every PEP MUST route its decision through this function rather than compare
// the decision string itself. Comparing against DecisionDeny instead of
// DecisionAllow inverts the system's default for every value that is neither,
// and discarding the transport error turns a policy-service outage into an
// open door.
//
// Usage, with the nil-safe protobuf getter:
//
//	resp, err := policyRead.CheckAccess(ctx, req)
//	if !ngac.Allowed(resp.GetDecision(), err) {
//		return ErrAccessDenied
//	}
func Allowed(decision string, err error) bool {
	return err == nil && decision == DecisionAllow
}
