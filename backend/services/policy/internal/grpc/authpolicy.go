package grpc

import (
	"ngac-platform/pkg/grpcauth"
	pb "ngac-platform/proto/policy"
)

// BackfillManagersService is the service identity of the one-off operator
// command messaging/cmd/backfill-managers.
const BackfillManagersService = "backfill-managers"

// AuthPolicy lists the policy RPCs that may run without an end user. Every
// other RPC needs the caller identity the REST edge verified (see package
// grpcauth); a service identity alone is refused.
//
// The auth service's signed identity may call four methods: the graph
// provisioning it performs while it is establishing who the user is, creating
// the user's node and attaching it to PublicUsers and to the tenant's
// member/owner UAs, and undoing the node when a later step fails. They run
// during signup and sign-in, before any token exists.
//
// InvalidateCache is also admitted for the operator tool `policy invalidate-cache`
// (service identity "policy-ctl"): after a migration rewrites the graph tables
// directly, it drops every workspace's cached decisions. It takes no user because
// an operator running a migration has none, and it can do nothing but invalidate.
//
// The operator command backfill-managers (service identity "backfill-managers")
// gives groups made before group roles existed their Managers UA. An operator
// running it has no user, so it may call exactly what that needs and nothing
// else: the writer's FindNodeByName (find the group's Managers UA), CreateNode
// (make it), CreateAssignment (put it under the workspace PC and the owner in
// it) and CreateAssociation (the managers' grant and the members' grant on the
// group's content). It can delete, remove or prohibit nothing.
func AuthPolicy() grpcauth.ServerPolicy {
	const signup = "auth provisions the user and tenant nodes during signup and sign-in, before any user is authenticated"
	const backfill = "the operator command backfill-managers gives groups made before group roles existed their Managers UA, with no user present"
	return grpcauth.ServerPolicy{
		Exempt: grpcauth.HealthExempt(),
		ServiceOK: map[string]grpcauth.ServiceRule{
			pb.PolicyWriteService_CreateNode_FullMethodName: grpcauth.ServiceOnly(
				signup+"; "+backfill+" (creates the Managers UA)", "auth", BackfillManagersService),
			pb.PolicyWriteService_CreateAssignment_FullMethodName: grpcauth.ServiceOnly(
				signup+"; "+backfill+" (puts it under the workspace PC and the owner in it)", "auth", BackfillManagersService),
			pb.PolicyWriteService_DeleteNode_FullMethodName: grpcauth.ServiceOnly(
				"auth removes the user or tenant node it just created when signup fails part-way, so no orphan is left in the graph", "auth"),
			pb.PolicyReadService_FindNodeByName_FullMethodName: grpcauth.ServiceOnly(
				signup+" (finds PublicUsers and the tenant UAs)", "auth"),
			pb.PolicyWriteService_FindNodeByName_FullMethodName: grpcauth.ServiceOnly(
				backfill+" (finds a group's Managers UA)", BackfillManagersService),
			pb.PolicyWriteService_CreateAssociation_FullMethodName: grpcauth.ServiceOnly(
				backfill+" (grants managers invite and members their ops on the group's content)", BackfillManagersService),
			pb.PolicyWriteService_InvalidateCache_FullMethodName: grpcauth.ServiceOnly(
				"the operator tool drops cached decisions after a migration rewrites the graph tables, with no user present",
				"policy-ctl"),
		},
	}
}
