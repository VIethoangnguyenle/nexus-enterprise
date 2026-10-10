package grpc

import (
	"ngac-platform/pkg/grpcauth"
	pb "ngac-platform/proto/auth"
)

// AuthPolicy lists the auth RPCs that may run without an end user: none but the
// health check, because sign-in happens over REST. Every other RPC needs the
// caller identity the REST edge verified (see package grpcauth).
func AuthPolicy() grpcauth.ServerPolicy {
	return grpcauth.ServerPolicy{
		Exempt: grpcauth.HealthExempt(),
		ServiceOK: map[string]grpcauth.ServiceRule{
			pb.AuthService_IsTokenRevoked_FullMethodName: grpcauth.ServiceOnly(
				"token validity is checked from a bare jti by services that hold no user session", "messaging"),
		},
	}
}
