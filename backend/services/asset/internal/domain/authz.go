package domain

import (
	"context"
	"errors"
	"fmt"

	"ngac-platform/ngac"
	"ngac-platform/pkg/grpcauth"
	"ngac-platform/pkg/policyclient"
	policypb "ngac-platform/proto/policy"
	"ngac-platform/services/asset/internal/store"
)

// Authorization helpers shared by the asset servers. Every one of them is
// fail-closed: no caller, an unresolvable OA, or an error from the policy
// service denies.

func errDenied(op string) error {
	return denied("no %s access", op)
}

// authorize checks op for the caller on one object node.
func authorize(ctx context.Context, pr policypb.PolicyReadServiceClient, userNodeID, objectNodeID, op string) error {
	if userNodeID == "" || objectNodeID == "" {
		return errDenied(op)
	}
	if ok, _ := policyclient.New(pr).Check(ctx, userNodeID, objectNodeID, op); !ok {
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
	if policyclient.IsNotFound(err) {
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
	batch, err := policyclient.New(pr).BatchCheck(ctx, userNodeID, oaIDs, []string{op})
	if err != nil {
		return nil, err
	}
	for _, at := range types {
		if at.NgacOAID != "" && batch.Has(at.NgacOAID, op) {
			permitted = append(permitted, at.ID)
		}
	}
	return permitted, nil
}

// heldOnTypes returns, for each given type that has an OA, the subset of ops the
// caller holds on that OA, from one batch call. A type the caller holds none of
// them on is absent from the map. An error from the policy service is returned,
// never read as "nothing held".
func heldOnTypes(ctx context.Context, pr policypb.PolicyReadServiceClient, userNodeID string, types []*store.AssetType, ops []string) (map[string][]string, error) {
	held := map[string][]string{}
	if userNodeID == "" || len(ops) == 0 {
		return held, nil
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
		return held, nil
	}
	batch, err := policyclient.New(pr).BatchCheck(ctx, userNodeID, oaIDs, ops)
	if err != nil {
		return nil, err
	}
	for _, at := range types {
		if at.NgacOAID == "" {
			continue
		}
		for _, op := range ops {
			if batch.Has(at.NgacOAID, op) {
				held[at.ID] = append(held[at.ID], op)
			}
		}
	}
	return held, nil
}

// Reasons a call is refused, machine-readable beside the status code (carried as
// an ErrorInfo detail and handed to REST clients as `reason`), so a screen can
// say which of several 409s it was.
const (
	ReasonAssetUnavailable = "asset_unavailable"
	ReasonRequestNotOpen   = "request_not_open"
	ReasonWrongType        = "wrong_type"
	ReasonNotAMember       = "not_a_member"
	ReasonSameHolder       = "same_holder"
	ReasonStateChanged     = "state_changed"
)

// storeErr turns a refusal the store made inside a transaction into the status
// a caller should see. Anything else is a failure of ours.
func storeErr(err error, what string) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, store.ErrNotFound):
		return notFound("%s not found", what)
	case errors.Is(err, store.ErrAssetUnavailable):
		return Refuse(ErrConflict, ReasonAssetUnavailable, "asset is not available")
	case errors.Is(err, store.ErrRequestNotOpen):
		return Refuse(ErrConflict, ReasonRequestNotOpen, "request is not in a state that allows this")
	case errors.Is(err, store.ErrStateChanged):
		return Refuse(ErrConflict, ReasonStateChanged, "asset changed since it was read")
	case errors.Is(err, store.ErrWrongType):
		return Refuse(ErrInvalidInput, ReasonWrongType, "asset is not of the requested type")
	case errors.Is(err, store.ErrNotAMember):
		return Refuse(ErrInvalidInput, ReasonNotAMember, "the person is not a member of this workspace")
	case errors.Is(err, store.ErrSameHolder):
		return Refuse(ErrInvalidInput, ReasonSameHolder, "the asset is already held by this person")
	}
	return fmt.Errorf("%s failed: %w", what, err)
}

// loadForDecision reads a request and its type and requires op on the type's OA
// before saying anything about the request. A request that does not exist, and
// one the caller may not touch, both answer PermissionDenied: whether a request
// ID exists is not the caller's to learn.
func (s *AssetRequestService) loadForDecision(ctx context.Context, requestID, op string) (*store.AssetRequest, *store.AssetType, error) {
	who := grpcauth.CallerFrom(ctx)
	r, err := s.store.GetRequest(ctx, requestID)
	if err != nil {
		return nil, nil, errDenied(op)
	}
	at, err := s.store.GetType(ctx, r.TypeID)
	if err != nil {
		return nil, nil, errDenied(op)
	}
	if err := s.checkAccess(ctx, who.NGACNodeID, at.NgacOAID, op); err != nil {
		return nil, nil, err
	}
	return r, at, nil
}
