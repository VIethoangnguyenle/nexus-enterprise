package grpc

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"ngac-platform/ngac"
	policypb "ngac-platform/proto/policy"
	"ngac-platform/services/asset/internal/store"
)

// Authorization helpers shared by the asset servers. Every one of them is
// fail-closed: no caller, an unresolvable OA, or an error from the policy
// service denies.

func errDenied(op string) error {
	return status.Errorf(codes.PermissionDenied, "no %s access", op)
}

// authorize checks op for the caller on one object node.
func authorize(ctx context.Context, pr policypb.PolicyReadServiceClient, userNodeID, objectNodeID, op string) error {
	if userNodeID == "" || objectNodeID == "" {
		return errDenied(op)
	}
	resp, err := pr.CheckAccess(ctx, &policypb.CheckAccessRequest{
		UserNodeId: userNodeID, ObjectNodeId: objectNodeID, Operation: op,
	})
	if !ngac.Allowed(resp.GetDecision(), err) {
		return errDenied(op)
	}
	return nil
}

// resolveOA looks up a well-known OA by name (always built with a helper from
// package ngac). found is false, with no error, only when the policy service
// says the node does not exist; any other failure is returned as an error so
// callers cannot mistake an outage for an absent node.
func resolveOA(ctx context.Context, pr policypb.PolicyReadServiceClient, name string) (id string, found bool, err error) {
	node, err := pr.FindNodeByName(ctx, &policypb.FindNodeByNameRequest{Name: name, NodeType: ngac.TypeOA})
	if status.Code(err) == codes.NotFound {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	if node.GetId() == "" {
		return "", false, nil
	}
	return node.GetId(), true, nil
}

// authorizeOnNamedOA checks op for the caller on a well-known OA. An OA that
// does not exist, or cannot be resolved, denies.
func authorizeOnNamedOA(ctx context.Context, pr policypb.PolicyReadServiceClient, userNodeID, oaName, op string) error {
	if userNodeID == "" {
		return errDenied(op)
	}
	id, found, err := resolveOA(ctx, pr, oaName)
	if err != nil || !found {
		return errDenied(op)
	}
	return authorize(ctx, pr, userNodeID, id, op)
}

// permittedTypeIDs returns the IDs of the asset types on whose type OA the
// caller holds op, in one batch call. Permission on the Assets OA or a category
// OA reaches the type OAs beneath it, so checking the type OA covers grants at
// every level of the asset tree. A type with no OA is never permitted.
//
// An error from the policy service is returned rather than treated as an
// empty answer, so a caller can refuse to list anything at all.
func permittedTypeIDs(ctx context.Context, pr policypb.PolicyReadServiceClient, userNodeID string, types []*store.AssetType, op string) ([]string, error) {
	permitted := []string{}
	if userNodeID == "" {
		return permitted, nil
	}
	var oaIDs []string
	seen := map[string]bool{}
	for _, at := range types {
		if at.NgacOAID != "" && !seen[at.NgacOAID] {
			seen[at.NgacOAID] = true
			oaIDs = append(oaIDs, at.NgacOAID)
		}
	}
	if len(oaIDs) == 0 {
		return permitted, nil
	}
	batch, err := pr.BatchCheckAccess(ctx, &policypb.BatchCheckAccessRequest{
		UserNodeId: userNodeID, ObjectIds: oaIDs, Operations: []string{op},
	})
	if err != nil {
		return nil, err
	}
	for _, at := range types {
		if at.NgacOAID != "" && batch.GetResults()[at.NgacOAID].GetPermissions()[op] {
			permitted = append(permitted, at.ID)
		}
	}
	return permitted, nil
}
