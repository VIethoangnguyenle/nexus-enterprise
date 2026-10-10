package grpc

import (
	"ngac-platform/pkg/grpcauth"
	pb "ngac-platform/proto/policy"
)

// AuthPolicy lists the policy RPCs that may run without an end user. Every
// other RPC needs the caller identity the REST edge verified (see package
// grpcauth); a service identity alone is refused.
//
// The three below are the graph provisioning the auth service performs while
// it is establishing who the user is: creating the user's node and attaching
// it to PublicUsers and to the tenant's member/owner UAs. They run during
// signup and sign-in, before any token exists.
func AuthPolicy() grpcauth.ServerPolicy {
	const signup = "auth provisions the user and tenant nodes during signup and sign-in, before any user is authenticated"
	return grpcauth.ServerPolicy{
		Exempt: grpcauth.HealthExempt(),
		ServiceOK: map[string]string{
			pb.PolicyWriteService_CreateNode_FullMethodName:       signup,
			pb.PolicyWriteService_CreateAssignment_FullMethodName: signup,
			pb.PolicyReadService_FindNodeByName_FullMethodName:    signup + " (finds PublicUsers and the tenant UAs)",
		},
	}
}
