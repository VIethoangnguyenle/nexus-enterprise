package grpc

import (
	"ngac-platform/pkg/grpcauth"
	pb "ngac-platform/proto/auth"
)

// AuthPolicy lists the auth RPCs that may run without an end user. Every
// other RPC needs the caller identity the REST edge verified (see package
// grpcauth).
func AuthPolicy() grpcauth.ServerPolicy {
	const credentials = "the request carries the credentials that authenticate it; there is no caller yet"
	exempt := grpcauth.HealthExempt()
	exempt[pb.AuthService_Register_FullMethodName] = credentials
	exempt[pb.AuthService_Login_FullMethodName] = credentials
	exempt[pb.AuthService_Signup_FullMethodName] = credentials
	exempt[pb.AuthService_Signin_FullMethodName] = credentials
	return grpcauth.ServerPolicy{
		Exempt: exempt,
		ServiceOK: map[string]string{
			pb.AuthService_IsTokenRevoked_FullMethodName: "token validity is checked from a bare jti by services that hold no user session",
		},
	}
}
