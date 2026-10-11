package grpc_test

import (
	"context"
	"strings"
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

// workspaceTypeStore is a pendingStore whose workspace is of a chosen type.
type workspaceTypeStore struct {
	pendingStore
	kind string
}

func (s *workspaceTypeStore) WorkspaceType(context.Context, string) (string, error) {
	return s.kind, nil
}

func inTenant(tenant string) context.Context {
	return grpcauth.WithCaller(context.Background(), grpcauth.Caller{UserID: "u-1", NGACNodeID: "node-1", TenantID: tenant})
}

// rpcs calls every method of the ApprovalService API once. The methods this
// server has not implemented answer Unimplemented, which runs nothing.
func rpcs(c pb.ApprovalServiceClient, ctx context.Context) map[string]error {
	const id = "11111111-1111-4111-8111-111111111111"
	out := map[string]error{}
	_, out["CreateTemplate"] = c.CreateTemplate(ctx, &pb.CreateTemplateRequest{})
	_, out["GetTemplate"] = c.GetTemplate(ctx, &pb.GetTemplateRequest{})
	_, out["ListTemplates"] = c.ListTemplates(ctx, &pb.ListTemplatesRequest{})
	_, out["UpdateTemplate"] = c.UpdateTemplate(ctx, &pb.UpdateTemplateRequest{})
	_, out["CreateRequest"] = c.CreateRequest(ctx, &pb.CreateApprovalRequestReq{})
	_, out["Approve"] = c.Approve(ctx, &pb.ApproveRequest{})
	_, out["Reject"] = c.Reject(ctx, &pb.RejectRequest{})
	_, out["BatchApprove"] = c.BatchApprove(ctx, &pb.BatchApproveRequest{})
	_, out["GetPending"] = c.GetPending(ctx, &pb.GetPendingRequest{})
	_, out["GetHistory"] = c.GetHistory(ctx, &pb.GetHistoryRequest{})
	_, out["GetMyRequests"] = c.GetMyRequests(ctx, &pb.GetMyRequestsRequest{})
	_, out["GetDepartmentRequests"] = c.GetDepartmentRequests(ctx, &pb.GetDepartmentRequestsRequest{})
	_, out["GetAuditLog"] = c.GetAuditLog(ctx, &pb.GetAuditLogRequest{RequestId: id})
	return out
}

func serveWorkspace(t *testing.T, kind string) pb.ApprovalServiceClient {
	t.Helper()
	srv := agrpc.NewServer(domain.NewService(&workspaceTypeStore{kind: kind}, &scopePolicy{}))
	conn := testutil.ServeGRPC(t, grpcauth.ServerPolicy{}, func(s *grpc.Server) { pb.RegisterApprovalServiceServer(s, srv) })
	return pb.NewApprovalServiceClient(conn)
}

// In a personal workspace no method of the API does anything: the ones this
// server implements answer PermissionDenied, the rest were never implemented.
func TestPersonalWorkspace_EveryRPCIsRefused(t *testing.T) {
	implemented := map[string]bool{"GetPending": true, "GetHistory": true, "GetMyRequests": true, "GetDepartmentRequests": true, "GetAuditLog": true}
	for name, err := range rpcs(serveWorkspace(t, "personal"), inTenant("tenant-1")) {
		t.Run(name, func(t *testing.T) {
			got := status.Code(err)
			if implemented[name] && got != codes.PermissionDenied {
				t.Fatalf("code %v (%v); want PermissionDenied", got, err)
			}
			if !implemented[name] && got != codes.Unimplemented && got != codes.PermissionDenied {
				t.Fatalf("code %v (%v); an unimplemented method must stay unimplemented or refuse", got, err)
			}
		})
	}
}

func TestOrganizationWorkspace_ImplementedRPCsAnswer(t *testing.T) {
	errs := rpcs(serveWorkspace(t, "organization"), inTenant("tenant-1"))
	// GetPending is the call this fake store can answer end to end.
	if errs["GetPending"] != nil {
		t.Errorf("GetPending: %v", errs["GetPending"])
	}
	for name, err := range errs {
		if status.Code(err) == codes.PermissionDenied &&
			strings.Contains(status.Convert(err).Message(), "personal workspace") {
			t.Errorf("%s refused as personal in an organization: %v", name, err)
		}
	}
}
