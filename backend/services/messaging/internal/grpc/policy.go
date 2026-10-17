package grpc

import (
	"context"
	"slices"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"ngac-platform/pkg/grpcauth"
	pb "ngac-platform/proto/messaging"
)

// ServerPolicy is who may call the messaging gRPC server without a user.
//
// Two methods are for services alone: the auth service makes the workspace's
// workspace-wide channel when it provisions a workspace, and the auth and
// workspace services put a new member into it when they admit one. Neither has an
// end user in hand (sign-in and signup run before any token exists, and the
// person being admitted is not the one calling).
func ServerPolicy() grpcauth.ServerPolicy {
	return grpcauth.ServerPolicy{
		Exempt: grpcauth.HealthExempt(),
		ServiceOK: map[string]grpcauth.ServiceRule{
			pb.MessagingService_ProvisionWorkspaceChannel_FullMethodName: grpcauth.ServiceOnly(
				"the auth service makes a new workspace's workspace-wide channel while provisioning it, before its owner holds a token",
				"auth"),
			pb.MessagingService_JoinWorkspaceChannel_FullMethodName: grpcauth.ServiceOnly(
				"the auth and workspace services add a person they have just admitted to the workspace-wide channel",
				"auth", "workspace"),
		},
	}
}

// requireService refuses a request that was not admitted on the identity of one of
// the named services. The interceptor admits any verified user on any method that
// is not exempt, and the policy above is the first list of services; a method
// meant for services says so again here, so a person's identity, or a service added
// to the policy for another method, never reaches it.
func requireService(ctx context.Context, services ...string) error {
	if !slices.Contains(services, grpcauth.ServiceFrom(ctx)) {
		return status.Error(codes.PermissionDenied, "this method is for the "+strings.Join(services, " and ")+" services only")
	}
	return nil
}
