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
	asked []string
}

func (p *pendingStore) ListPending(_ context.Context, userNodeID string) ([]*domain.RequestWithAssignment, error) {
	p.asked = append(p.asked, userNodeID)
	return nil, nil
}

func serve(t *testing.T) (pb.ApprovalServiceClient, *pendingStore) {
	t.Helper()
	st := &pendingStore{}
	srv := agrpc.NewServer(domain.NewService(st, nil))
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
	if _, err := c.GetAuditLog(context.Background(), &pb.GetAuditLogRequest{RequestId: "r"}); status.Code(err) != codes.Unauthenticated {
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
