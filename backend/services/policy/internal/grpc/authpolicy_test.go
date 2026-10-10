package grpc_test

import (
	"context"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"ngac-platform/pkg/grpcauth"
	pb "ngac-platform/proto/policy"
	pgrpc "ngac-platform/services/policy/internal/grpc"
	"ngac-platform/testutil"
)

// Unimplemented servers answer Unimplemented, which tells an admitted request
// (reached the handler) apart from a refused one (Unauthenticated).
func servePolicy(t *testing.T) (pb.PolicyWriteServiceClient, pb.PolicyReadServiceClient) {
	t.Helper()
	conn := testutil.ServeGRPC(t, pgrpc.AuthPolicy(), func(s *grpc.Server) {
		pb.RegisterPolicyWriteServiceServer(s, pb.UnimplementedPolicyWriteServiceServer{})
		pb.RegisterPolicyReadServiceServer(s, pb.UnimplementedPolicyReadServiceServer{})
	})
	return pb.NewPolicyWriteServiceClient(conn), pb.NewPolicyReadServiceClient(conn)
}

func wantCode(t *testing.T, what string, err error, want codes.Code) {
	t.Helper()
	if got := status.Code(err); got != want {
		t.Fatalf("%s: code = %v, want %v (err: %v)", what, got, want, err)
	}
}

func TestPolicyRefusesRequestsWithoutACaller(t *testing.T) {
	w, r := servePolicy(t)
	ctx := context.Background()

	// The test client dials with a service identity; only the listed
	// provisioning RPCs accept it in place of a user.
	_, err := w.DeleteNode(ctx, &pb.DeleteNodeRequest{NodeId: "n"})
	wantCode(t, "DeleteNode", err, codes.Unauthenticated)
	_, err = w.RemoveAssignment(ctx, &pb.RemoveAssignmentRequest{})
	wantCode(t, "RemoveAssignment", err, codes.Unauthenticated)
	_, err = w.CreateAssociation(ctx, &pb.CreateAssociationRequest{})
	wantCode(t, "CreateAssociation", err, codes.Unauthenticated)
	_, err = r.CheckAccess(ctx, &pb.CheckAccessRequest{})
	wantCode(t, "CheckAccess", err, codes.Unauthenticated)
	_, err = r.GetChildren(ctx, &pb.GetChildrenRequest{})
	wantCode(t, "GetChildren", err, codes.Unauthenticated)
}

func TestPolicyAdmitsSignupProvisioningFromAService(t *testing.T) {
	w, r := servePolicy(t)
	ctx := context.Background()

	_, err := w.CreateNode(ctx, &pb.CreateNodeRequest{})
	wantCode(t, "CreateNode", err, codes.Unimplemented)
	_, err = w.CreateAssignment(ctx, &pb.CreateAssignmentRequest{})
	wantCode(t, "CreateAssignment", err, codes.Unimplemented)
	_, err = r.FindNodeByName(ctx, &pb.FindNodeByNameRequest{})
	wantCode(t, "FindNodeByName", err, codes.Unimplemented)
}

func TestPolicyAdmitsAUserCallerOnAnyRPC(t *testing.T) {
	w, r := servePolicy(t)
	ctx := grpcauth.WithCaller(context.Background(), grpcauth.Caller{UserID: "u", NGACNodeID: "n"})

	_, err := w.DeleteNode(ctx, &pb.DeleteNodeRequest{})
	wantCode(t, "DeleteNode", err, codes.Unimplemented)
	_, err = r.CheckAccess(ctx, &pb.CheckAccessRequest{})
	wantCode(t, "CheckAccess", err, codes.Unimplemented)
}
