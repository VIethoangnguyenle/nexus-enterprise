package grpc

import (
	"ngac-platform/pkg/grpcauth"
	pb "ngac-platform/proto/policy"
)

// BackfillManagersService is the service identity of the one-off operator
// command messaging/cmd/backfill-managers.
const BackfillManagersService = "backfill-managers"

// PolicyCtlService is the service identity of the operator tool
// `policy invalidate-cache`.
const PolicyCtlService = "policy-ctl"

// WriteCallers is who may call each PolicyWriteService RPC: Users lists the
// services a user's request must come through (grpcauth UserVia), Bare the
// services that may call with no user at all (grpcauth ServiceOK). Every writer
// RPC has a row; an RPC added without one admits nobody (see AuthPolicy).
//
// The user lists are the services that make that call today, found at every
// call site in backend/services/*/. Six services provision graph nodes through
// pkg/provision, whose Creator reaches CreateNode, CreateAssignment,
// CreateAssociation and, when it rolls back, DeleteNode. approval holds the
// writer only to ask membership questions that must see a change at once, so it
// gets the writer's graph reads and no write. document, and any service not
// named, holds no writer and gets nothing. Adding a caller means adding it here.
//
// Bare service identities:
//   - auth provisions the user and tenant nodes during signup and sign-in,
//     before any user is authenticated: it finds PublicUsers and the tenant UAs
//     (on the reader), creates the user's node, attaches it, and deletes the
//     node it just created when a later step fails.
//   - backfill-managers gives groups made before group roles existed their
//     Managers UA: it finds the UA on the writer, creates it, puts it under the
//     workspace PC and the owner in it, and grants it. It can delete, remove or
//     prohibit nothing.
//   - policy-ctl drops cached decisions after a migration rewrote the graph
//     tables; it can do nothing else.
func WriteCallers() map[string]WriteCaller {
	provisioners := []string{"asset", "auth", "calendar", "drive", "messaging", "workspace"}
	graphReaders := append([]string{"approval"}, provisioners...)

	const (
		provisioning = "graph provisioning through pkg/provision (its rollback deletes the nodes it created)"
		reads        = "graph reads on the authoritative writer, for decisions that must see a change made a moment ago"
		signup       = "auth provisions the user and tenant nodes during signup and sign-in, before any user is authenticated"
		backfill     = "the operator command backfill-managers gives groups made before group roles existed their Managers UA, with no user present"
		nobody       = "no service calls this on behalf of a user"
	)
	read := WriteCaller{Users: grpcauth.ServiceOnly(reads, graphReaders...)}

	return map[string]WriteCaller{
		pb.PolicyWriteService_CreateNode_FullMethodName: {
			Users: grpcauth.ServiceOnly(provisioning, provisioners...),
			Bare:  grpcauth.ServiceOnly(signup+"; "+backfill+" (creates the Managers UA)", "auth", BackfillManagersService),
		},
		pb.PolicyWriteService_DeleteNode_FullMethodName: {
			Users: grpcauth.ServiceOnly(provisioning+"; calendar, drive, messaging and workspace delete what they own",
				provisioners...),
			Bare: grpcauth.ServiceOnly("auth removes the user or tenant node it just created when signup fails part-way, so no orphan is left in the graph", "auth"),
		},
		pb.PolicyWriteService_CreateAssignment_FullMethodName: {
			Users: grpcauth.ServiceOnly(provisioning, provisioners...),
			Bare:  grpcauth.ServiceOnly(signup+"; "+backfill+" (puts it under the workspace PC and the owner in it)", "auth", BackfillManagersService),
		},
		pb.PolicyWriteService_CreateAssociation_FullMethodName: {
			Users: grpcauth.ServiceOnly(provisioning, provisioners...),
			Bare:  grpcauth.ServiceOnly(backfill+" (grants managers invite and members their ops on the group's content)", BackfillManagersService),
		},
		pb.PolicyWriteService_RemoveAssignment_FullMethodName: {
			Users: grpcauth.ServiceOnly("membership, attendee, share and department changes and tenant leave",
				"auth", "calendar", "drive", "messaging", "workspace"),
		},
		pb.PolicyWriteService_RemoveAssociation_FullMethodName: {
			Users: grpcauth.ServiceOnly("calendar unsharing; workspace role, directory and group-creation grants", "calendar", "workspace"),
		},
		pb.PolicyWriteService_CreateProhibition_FullMethodName: {
			Users: grpcauth.ServiceOnly("the workspace directory's own-department limit", "workspace"),
		},
		pb.PolicyWriteService_RemoveProhibition_FullMethodName: {
			Users: grpcauth.ServiceOnly("the workspace directory's own-department limit", "workspace"),
		},
		pb.PolicyWriteService_RegisterOperations_FullMethodName: {Users: grpcauth.ServiceOnly(nobody)},
		pb.PolicyWriteService_InvalidateCache_FullMethodName: {
			Users: grpcauth.ServiceOnly(nobody),
			Bare:  grpcauth.ServiceOnly("the operator tool drops cached decisions after a migration rewrites the graph tables, with no user present", PolicyCtlService),
		},
		pb.PolicyWriteService_InitSchema_FullMethodName: {Users: grpcauth.ServiceOnly(nobody)},
		pb.PolicyWriteService_LoadGraph_FullMethodName:  {Users: grpcauth.ServiceOnly(nobody)},

		pb.PolicyWriteService_GetAssociations_FullMethodName: read,
		pb.PolicyWriteService_GetNode_FullMethodName:         read,
		pb.PolicyWriteService_FindNodeByName_FullMethodName: {
			Users: read.Users,
			Bare:  grpcauth.ServiceOnly(backfill+" (finds a group's Managers UA)", BackfillManagersService),
		},
		pb.PolicyWriteService_IsAssigned_FullMethodName:     read,
		pb.PolicyWriteService_GetChildren_FullMethodName:    read,
		pb.PolicyWriteService_GetParents_FullMethodName:     read,
		pb.PolicyWriteService_GetAncestors_FullMethodName:   read,
		pb.PolicyWriteService_GetDescendants_FullMethodName: read,
	}
}

// WriteCaller is one row of WriteCallers. A zero Bare admits no bare service.
type WriteCaller struct {
	Users grpcauth.ServiceRule
	Bare  grpcauth.ServiceRule
}

// AuthPolicy is the caller policy of both policy servers. The read service
// needs a verified end user on every RPC, from any service, except the
// reader's FindNodeByName for auth's signup lookups. The writer admits what
// WriteCallers lists and nothing more: an RPC on the writer's service
// descriptor with no row there gets an empty user allowlist, so it fails
// closed for every caller.
func AuthPolicy() grpcauth.ServerPolicy {
	p := grpcauth.ServerPolicy{
		Exempt: grpcauth.HealthExempt(),
		ServiceOK: map[string]grpcauth.ServiceRule{
			pb.PolicyReadService_FindNodeByName_FullMethodName: grpcauth.ServiceOnly(
				"auth provisions the user and tenant nodes during signup and sign-in, before any user is authenticated (finds PublicUsers and the tenant UAs)",
				"auth"),
		},
		UserVia: map[string]grpcauth.ServiceRule{},
	}
	for _, m := range pb.PolicyWriteService_ServiceDesc.Methods {
		full := "/" + pb.PolicyWriteService_ServiceDesc.ServiceName + "/" + m.MethodName
		p.UserVia[full] = grpcauth.ServiceOnly("no caller rule: refused")
	}
	for method, row := range WriteCallers() {
		p.UserVia[method] = row.Users
		if len(row.Bare.Services) > 0 {
			p.ServiceOK[method] = row.Bare
		}
	}
	return p
}
