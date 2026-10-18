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
	return servePolicyAs(t, "auth")
}

// servePolicyAs dials as service. "auth" is the only service the policy
// accepts without a user.
func servePolicyAs(t *testing.T, service string) (pb.PolicyWriteServiceClient, pb.PolicyReadServiceClient) {
	t.Helper()
	conn := testutil.ServeGRPCAs(t, service, pgrpc.AuthPolicy(), func(s *grpc.Server) {
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

	// The test client dials as the auth service; only the listed provisioning
	// RPCs accept a service identity in place of a user.
	_, err := w.RemoveAssignment(ctx, &pb.RemoveAssignmentRequest{})
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
	// Signup's rollback removes the node it just created.
	_, err = w.DeleteNode(ctx, &pb.DeleteNodeRequest{NodeId: "n"})
	wantCode(t, "DeleteNode", err, codes.Unimplemented)
}

func TestPolicyAdmitsAUserCallerOnAnyRPC(t *testing.T) {
	w, r := servePolicy(t)
	ctx := grpcauth.WithCaller(context.Background(), grpcauth.Caller{UserID: "u", NGACNodeID: "n"})

	_, err := w.DeleteNode(ctx, &pb.DeleteNodeRequest{})
	wantCode(t, "DeleteNode", err, codes.Unimplemented)
	_, err = r.CheckAccess(ctx, &pb.CheckAccessRequest{})
	wantCode(t, "CheckAccess", err, codes.Unimplemented)
}

// The writer's graph reads carry the same policy as the read service: a verified
// end user, never a bare service identity.
func TestPolicyWriterGraphReadsNeedAUserCaller(t *testing.T) {
	w, _ := servePolicy(t)
	bare := context.Background()
	user := grpcauth.WithCaller(context.Background(), grpcauth.Caller{UserID: "u", NGACNodeID: "n"})
	for name, call := range map[string]func(context.Context) error{
		"GetNode":        func(c context.Context) error { _, err := w.GetNode(c, &pb.GetNodeRequest{}); return err },
		"GetChildren":    func(c context.Context) error { _, err := w.GetChildren(c, &pb.GetChildrenRequest{}); return err },
		"GetParents":     func(c context.Context) error { _, err := w.GetParents(c, &pb.GetParentsRequest{}); return err },
		"GetAncestors":   func(c context.Context) error { _, err := w.GetAncestors(c, &pb.GetAncestorsRequest{}); return err },
		"GetDescendants": func(c context.Context) error { _, err := w.GetDescendants(c, &pb.GetDescendantsRequest{}); return err },
		"GetAssociations": func(c context.Context) error {
			_, err := w.GetAssociations(c, &pb.GetAssociationsRequest{})
			return err
		},
	} {
		wantCode(t, name+" without a caller", call(bare), codes.Unauthenticated)
		wantCode(t, name+" with a user", call(user), codes.Unimplemented)
	}
}

// A service other than auth gets no provisioning shortcut, and a forged
// identity (retired metadata, an unsigned token) gets nothing at all.
func TestPolicyRefusesServicesOtherThanAuthAndForgedIdentity(t *testing.T) {
	ctx := context.Background()

	w, r := servePolicyAs(t, "workspace")
	_, err := w.CreateNode(ctx, &pb.CreateNodeRequest{})
	wantCode(t, "CreateNode from workspace", err, codes.Unauthenticated)
	_, err = w.CreateAssignment(ctx, &pb.CreateAssignmentRequest{})
	wantCode(t, "CreateAssignment from workspace", err, codes.Unauthenticated)
	_, err = w.DeleteNode(ctx, &pb.DeleteNodeRequest{NodeId: "n"})
	wantCode(t, "DeleteNode from workspace", err, codes.Unauthenticated)
	_, err = r.FindNodeByName(ctx, &pb.FindNodeByNameRequest{})
	wantCode(t, "FindNodeByName from workspace", err, codes.Unauthenticated)

	conn := testutil.ServeGRPC(t, pgrpc.AuthPolicy(), func(s *grpc.Server) {
		pb.RegisterPolicyWriteServiceServer(s, pb.UnimplementedPolicyWriteServiceServer{})
		pb.RegisterPolicyReadServiceServer(s, pb.UnimplementedPolicyReadServiceServer{})
	})
	raw := testutil.Unsigned(t, conn)
	_, err = pb.NewPolicyWriteServiceClient(raw).CreateNode(testutil.ForgedIdentity(ctx), &pb.CreateNodeRequest{})
	wantCode(t, "CreateNode with forged identity", err, codes.Unauthenticated)
	_, err = pb.NewPolicyWriteServiceClient(raw).DeleteNode(testutil.ForgedIdentity(ctx), &pb.DeleteNodeRequest{NodeId: "n"})
	wantCode(t, "DeleteNode with forged identity", err, codes.Unauthenticated)
	_, err = pb.NewPolicyReadServiceClient(raw).CheckAccess(testutil.ForgedIdentity(ctx), &pb.CheckAccessRequest{})
	wantCode(t, "CheckAccess with forged identity", err, codes.Unauthenticated)
}

// The operator tool (`policy invalidate-cache`) dials as policy-ctl and may call
// InvalidateCache with no user; it gets nothing else, and no other service gets it.
func TestPolicyAdmitsOnlyThePolicyCtlToolToInvalidateTheCache(t *testing.T) {
	ctx := context.Background()

	w, _ := servePolicyAs(t, "policy-ctl")
	_, err := w.InvalidateCache(ctx, &pb.InvalidateCacheRequest{})
	wantCode(t, "InvalidateCache from policy-ctl", err, codes.Unimplemented)
	_, err = w.CreateNode(ctx, &pb.CreateNodeRequest{})
	wantCode(t, "CreateNode from policy-ctl", err, codes.Unauthenticated)
	_, err = w.RemoveAssignment(ctx, &pb.RemoveAssignmentRequest{})
	wantCode(t, "RemoveAssignment from policy-ctl", err, codes.Unauthenticated)

	for _, svc := range []string{"workspace", "auth", "messaging"} {
		w, _ := servePolicyAs(t, svc)
		_, err := w.InvalidateCache(ctx, &pb.InvalidateCacheRequest{})
		wantCode(t, "InvalidateCache from "+svc, err, codes.Unauthenticated)
	}
}

// The one-off managers backfill dials as backfill-managers with no user. It may
// look a group's Managers UA up on the writer and create it, its PC edge, its
// grant and the owner's edge; nothing else, and no other service gets those.
func TestPolicyAdmitsTheManagersBackfillOnlyOnWhatItWrites(t *testing.T) {
	ctx := context.Background()

	w, r := servePolicyAs(t, "backfill-managers")
	_, err := w.FindNodeByName(ctx, &pb.FindNodeByNameRequest{})
	wantCode(t, "writer FindNodeByName from backfill-managers", err, codes.Unimplemented)
	_, err = w.CreateNode(ctx, &pb.CreateNodeRequest{})
	wantCode(t, "CreateNode from backfill-managers", err, codes.Unimplemented)
	_, err = w.CreateAssignment(ctx, &pb.CreateAssignmentRequest{})
	wantCode(t, "CreateAssignment from backfill-managers", err, codes.Unimplemented)
	_, err = w.CreateAssociation(ctx, &pb.CreateAssociationRequest{})
	wantCode(t, "CreateAssociation from backfill-managers", err, codes.Unimplemented)

	// Deny: everything else it could ask stays refused.
	for name, call := range map[string]func() error{
		"DeleteNode":       func() error { _, err := w.DeleteNode(ctx, &pb.DeleteNodeRequest{NodeId: "n"}); return err },
		"RemoveAssignment": func() error { _, err := w.RemoveAssignment(ctx, &pb.RemoveAssignmentRequest{}); return err },
		"RemoveAssociation": func() error {
			_, err := w.RemoveAssociation(ctx, &pb.RemoveAssociationRequest{})
			return err
		},
		"CreateProhibition": func() error {
			_, err := w.CreateProhibition(ctx, &pb.CreateProhibitionRequest{})
			return err
		},
		"InvalidateCache": func() error { _, err := w.InvalidateCache(ctx, &pb.InvalidateCacheRequest{}); return err },
		"writer IsAssigned": func() error {
			_, err := w.IsAssigned(ctx, &pb.IsAssignedRequest{})
			return err
		},
		"writer GetChildren": func() error { _, err := w.GetChildren(ctx, &pb.GetChildrenRequest{}); return err },
		"reader FindNodeByName": func() error {
			_, err := r.FindNodeByName(ctx, &pb.FindNodeByNameRequest{})
			return err
		},
		"CheckAccess": func() error { _, err := r.CheckAccess(ctx, &pb.CheckAccessRequest{}); return err },
	} {
		wantCode(t, name+" from backfill-managers", call(), codes.Unauthenticated)
	}

	// Deny: another unknown service gets none of the backfill's methods, and auth
	// gains neither the writer's lookup nor the grant.
	for _, svc := range []string{"backfill-other", "messaging"} {
		w, _ := servePolicyAs(t, svc)
		_, err := w.FindNodeByName(ctx, &pb.FindNodeByNameRequest{})
		wantCode(t, "writer FindNodeByName from "+svc, err, codes.Unauthenticated)
		_, err = w.CreateNode(ctx, &pb.CreateNodeRequest{})
		wantCode(t, "CreateNode from "+svc, err, codes.Unauthenticated)
		_, err = w.CreateAssignment(ctx, &pb.CreateAssignmentRequest{})
		wantCode(t, "CreateAssignment from "+svc, err, codes.Unauthenticated)
		_, err = w.CreateAssociation(ctx, &pb.CreateAssociationRequest{})
		wantCode(t, "CreateAssociation from "+svc, err, codes.Unauthenticated)
	}
	wa, _ := servePolicyAs(t, "auth")
	_, err = wa.FindNodeByName(ctx, &pb.FindNodeByNameRequest{})
	wantCode(t, "writer FindNodeByName from auth", err, codes.Unauthenticated)
	_, err = wa.CreateAssociation(ctx, &pb.CreateAssociationRequest{})
	wantCode(t, "CreateAssociation from auth", err, codes.Unauthenticated)
}
