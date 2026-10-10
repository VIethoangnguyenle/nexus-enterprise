package grpc_test

import (
	"context"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"ngac-platform/pkg/grpcauth"
	pb "ngac-platform/proto/approval"
	"ngac-platform/services/approval/internal/domain"
	agrpc "ngac-platform/services/approval/internal/grpc"
	"ngac-platform/testutil"
)

// pendingStore records whose pending list was requested.
type pendingStore struct {
	domain.Store
	asked       []string
	scopeLookup [][]string // scope sets passed to ListByScopes
}

func (p *pendingStore) ListByScopes(_ context.Context, scopes []string, _ string, _ int) ([]*domain.Request, string, error) {
	p.scopeLookup = append(p.scopeLookup, scopes)
	return []*domain.Request{{ID: "visible", ScopeOAID: scopes[0]}}, "", nil
}

// scopePolicy resolves scopes only for the users it is told about.
type scopePolicy struct {
	byUser map[string][]string
	asked  []string
}

func (p *scopePolicy) ResolveAccessibleScopes(_ context.Context, userNodeID, _ string) ([]string, error) {
	p.asked = append(p.asked, userNodeID)
	return p.byUser[userNodeID], nil
}

func (p *scopePolicy) CheckAccess(context.Context, string, string, string) (bool, error) {
	return false, nil
}
func (p *scopePolicy) GetAncestors(context.Context, string) ([]string, error) { return nil, nil }
func (p *scopePolicy) GetMembers(context.Context, string) ([]string, error)   { return nil, nil }

func (p *pendingStore) ListPending(_ context.Context, userNodeID string, _ []string) ([]*domain.RequestWithAssignment, error) {
	p.asked = append(p.asked, userNodeID)
	return nil, nil
}

func serve(t *testing.T) (pb.ApprovalServiceClient, *pendingStore) {
	t.Helper()
	st := &pendingStore{}
	srv := agrpc.NewServer(domain.NewService(st, &scopePolicy{}))
	conn := testutil.ServeGRPC(t, grpcauth.ServerPolicy{}, func(s *grpc.Server) { pb.RegisterApprovalServiceServer(s, srv) })
	return pb.NewApprovalServiceClient(conn), st
}

func asCaller(userID, nodeID string) context.Context {
	return grpcauth.WithCaller(context.Background(), grpcauth.Caller{UserID: userID, NGACNodeID: nodeID})
}

func TestMissingCallerIsUnauthenticated(t *testing.T) {
	c, st := serve(t)

	if _, err := c.GetPending(context.Background(), &pb.GetPendingRequest{}); status.Code(err) != codes.Unauthenticated {
		t.Fatalf("GetPending: want Unauthenticated, got %v", err)
	}
	if _, err := c.GetAuditLog(context.Background(), &pb.GetAuditLogRequest{RequestId: "11111111-1111-4111-8111-111111111111"}); status.Code(err) != codes.Unauthenticated {
		t.Fatalf("GetAuditLog: want Unauthenticated, got %v", err)
	}
	if len(st.asked) != 0 {
		t.Fatalf("store reached without a caller: %v", st.asked)
	}
}

// The body asks for another user's pending approvals; the metadata decides
// whose list is read.
func TestBodyUserIsIgnored(t *testing.T) {
	c, st := serve(t)

	_, err := c.GetPending(asCaller("u-1", "node-metadata"), &pb.GetPendingRequest{
		UserNodeId: "node-body", //lint:ignore SA1019 proves the deprecated body field is not trusted
	})

	if err != nil {
		t.Fatal(err)
	}
	if len(st.asked) != 1 || st.asked[0] != "node-metadata" {
		t.Fatalf("pending list read for %v, want [node-metadata]", st.asked)
	}
}

func serveWithPolicy(t *testing.T, pol domain.PolicyClient) (pb.ApprovalServiceClient, *pendingStore) {
	t.Helper()
	st := &pendingStore{}
	srv := agrpc.NewServer(domain.NewService(st, pol))
	conn := testutil.ServeGRPC(t, grpcauth.ServerPolicy{}, func(s *grpc.Server) { pb.RegisterApprovalServiceServer(s, srv) })
	return pb.NewApprovalServiceClient(conn), st
}

// Deny: the body names a node that does hold a department scope; the metadata
// caller holds none. The scope lookup must be made for the metadata caller, so
// the result is empty and the store is never asked for the body node's scope.
func TestBodyUserIsIgnored_DepartmentScopeDeny(t *testing.T) {
	pol := &scopePolicy{byUser: map[string][]string{"node-body": {"oa-secret"}}}
	c, st := serveWithPolicy(t, pol)

	got, err := c.GetDepartmentRequests(asCaller("u-1", "node-metadata"), &pb.GetDepartmentRequestsRequest{
		UserNodeId: "node-body", //lint:ignore SA1019 proves the deprecated body field is not trusted
	})

	if err != nil {
		t.Fatal(err)
	}
	if len(got.Requests) != 0 {
		t.Fatalf("got %d requests, want none: the metadata caller has no scope", len(got.Requests))
	}
	if len(pol.asked) != 1 || pol.asked[0] != "node-metadata" {
		t.Fatalf("scopes resolved for %v, want [node-metadata]", pol.asked)
	}
	if len(st.scopeLookup) != 0 {
		t.Fatalf("store queried with scopes %v; it must not be reached for a caller without scopes", st.scopeLookup)
	}
}

// Allow counterpart: the same call succeeds when the metadata caller is the one
// holding the scope, so the deny above is not an artefact of the fixture.
func TestMetadataCallerScopeDecides_DepartmentAllow(t *testing.T) {
	pol := &scopePolicy{byUser: map[string][]string{"node-metadata": {"oa-dept"}}}
	c, st := serveWithPolicy(t, pol)

	got, err := c.GetDepartmentRequests(asCaller("u-1", "node-metadata"), &pb.GetDepartmentRequestsRequest{
		UserNodeId: "node-body", //lint:ignore SA1019 proves the deprecated body field is not trusted
	})

	if err != nil {
		t.Fatal(err)
	}
	if len(got.Requests) != 1 || len(st.scopeLookup) != 1 || st.scopeLookup[0][0] != "oa-dept" {
		t.Fatalf("requests=%v lookups=%v, want one request in oa-dept", got.Requests, st.scopeLookup)
	}
}

func (p *pendingStore) GetRequest(_ context.Context, id string) (*domain.Request, error) {
	return &domain.Request{ID: id, CreatedBy: "node-requester", ScopeOAID: "oa-dept"}, nil
}

func (p *pendingStore) HasAssignment(_ context.Context, _ string, nodeIDs []string) (bool, error) {
	for _, id := range nodeIDs {
		if id == "node-approver" {
			return true, nil
		}
	}
	return false, nil
}

func (p *pendingStore) ListAuditEntries(_ context.Context, requestID string) ([]*domain.AuditEntry, error) {
	return []*domain.AuditEntry{{ID: "e1", RequestID: requestID, Action: "created"}}, nil
}

// The audit trail is returned to the requester and to an assignee, and refused
// to anyone else - whatever node the request body names.
func TestGetAuditLog_OverTheWire(t *testing.T) {
	c, _ := serveWithPolicy(t, &scopePolicy{byUser: map[string][]string{}})

	for _, node := range []string{"node-requester", "node-approver"} {
		got, err := c.GetAuditLog(asCaller("u", node), &pb.GetAuditLogRequest{RequestId: "11111111-1111-4111-8111-111111111111"})
		if err != nil || len(got.Entries) != 1 {
			t.Errorf("%s: entries=%v err=%v, want 1 entry", node, got, err)
		}
	}

	got, err := c.GetAuditLog(asCaller("u", "node-stranger"), &pb.GetAuditLogRequest{RequestId: "11111111-1111-4111-8111-111111111111"})
	if status.Code(err) != codes.PermissionDenied {
		t.Fatalf("stranger: want PermissionDenied, got %v (%v)", err, got)
	}
}

func TestGetAuditLog_MalformedIDIsInvalidArgument(t *testing.T) {
	c, _ := serveWithPolicy(t, &scopePolicy{byUser: map[string][]string{}})
	_, err := c.GetAuditLog(asCaller("u", "node-requester"), &pb.GetAuditLogRequest{RequestId: "not-a-uuid"})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("want InvalidArgument, got %v", err)
	}
}
