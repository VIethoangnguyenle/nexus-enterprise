package grpc

import (
	"ngac-platform/pkg/grpcauth"
	pb "ngac-platform/proto/policy"
)

// AuthPolicy lists the policy RPCs that may run without an end user. Every
// other RPC needs the caller identity the REST edge verified (see package
// grpcauth); a service identity alone is refused. The four auth-only methods
// below accept only the auth service's signed identity.
//
// InvalidateCache is also admitted for the operator tool `policy invalidate-cache`
// (service identity "policy-ctl"): after a migration rewrites the graph tables
// directly, it drops every workspace's cached decisions. It takes no user because
// an operator running a migration has none, and it can do nothing but invalidate.
//
// Those four are the graph provisioning the auth service performs while
// it is establishing who the user is: creating the user's node and attaching
// it to PublicUsers and to the tenant's member/owner UAs, and undoing the node when a later
// step fails. They run during signup and sign-in, before any token exists.
func AuthPolicy() grpcauth.ServerPolicy {
	rule := func(why string) grpcauth.ServiceRule {
		return grpcauth.ServiceOnly(why, "auth")
	}
	const signup = "auth provisions the user and tenant nodes during signup and sign-in, before any user is authenticated"
	return grpcauth.ServerPolicy{
		Exempt: grpcauth.HealthExempt(),
		ServiceOK: map[string]grpcauth.ServiceRule{
			pb.PolicyWriteService_CreateNode_FullMethodName:       rule(signup),
			pb.PolicyWriteService_CreateAssignment_FullMethodName: rule(signup),
			pb.PolicyWriteService_DeleteNode_FullMethodName:       rule("auth removes the user or tenant node it just created when signup fails part-way, so no orphan is left in the graph"),
			pb.PolicyReadService_FindNodeByName_FullMethodName:    rule(signup + " (finds PublicUsers and the tenant UAs)"),
			pb.PolicyWriteService_InvalidateCache_FullMethodName: grpcauth.ServiceOnly(
				"the operator tool drops cached decisions after a migration rewrites the graph tables, with no user present",
				"policy-ctl"),
		},
	}
}
