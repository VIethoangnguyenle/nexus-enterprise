package ngac_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	ops "ngac-platform/ngac"
	"ngac-platform/pkg/grpcauth"
	pb "ngac-platform/proto/policy"
	pgrpc "ngac-platform/services/policy/internal/grpc"
	"ngac-platform/services/policy/internal/ngac"
	"ngac-platform/testutil"
)

// Test vector: who may change the graph. A member's grant on a workspace OA
// (PC <- UA <- user, PC <- OA, UA -> OA [read, write]) is the decision under
// attack. Writes arriving through a service that never makes that call (a
// service holding no writer, one that only reads the graph, or one that makes
// other writes) are refused at the writer and leave the decision ALLOW; the
// same write through the service that really makes it lands and the decision
// follows the graph. The table itself is pgrpc.WriteCallers.
func TestWriteCallersVector_RefusedWritesLeaveDecisionsUnchanged(t *testing.T) {
	store, pool := setupStore(t)
	ctx := context.Background()
	sfx := uuid.NewString()[:8]
	ws := "ws-" + uuid.NewString()

	mk := func(name, typ string, props map[string]string) string {
		n, err := store.CreateNode(ctx, name+"_"+sfx, typ, props)
		require.NoError(t, err)
		return n.ID
	}
	pc := mk("PC_callers", ngac.NodeTypePolicyClass, map[string]string{"workspace_id": ws})
	ua := mk("UA_callers", ngac.NodeTypeUserAttribute, nil)
	oa := mk("OA_callers", ngac.NodeTypeObjectAttr, nil)
	user := mk("U_callers", ngac.NodeTypeUser, nil)
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), "DELETE FROM ngac_prohibitions WHERE name = $1", "callers_"+sfx)
		_, _ = pool.Exec(context.Background(), "DELETE FROM ngac_nodes WHERE id = ANY($1)", []string{pc, ua, oa, user})
	})
	for _, a := range [][2]string{{ua, pc}, {oa, pc}, {user, ua}} {
		_, err := store.CreateAssignment(ctx, a[0], a[1])
		require.NoError(t, err)
	}
	_, err := store.CreateAssociation(ctx, ua, oa, []string{ops.OpRead, ops.OpWrite})
	require.NoError(t, err)

	writer := pgrpc.NewWriteServer(store, nil, ngac.NewInvalidationCoordinator(nil), nil,
		ngac.NewProhibitionStore(pool, store.GetGraph()), false)
	via := func(service string) pb.PolicyWriteServiceClient {
		conn := testutil.ServeGRPCAs(t, service, pgrpc.AuthPolicy(), func(s *grpc.Server) {
			pb.RegisterPolicyWriteServiceServer(s, writer)
		})
		return pb.NewPolicyWriteServiceClient(conn)
	}
	// The caller is an administrator of the workspace in every call: the
	// question here is only which service the write came through.
	admin := grpcauth.WithCaller(ctx, grpcauth.Caller{UserID: "admin", NGACNodeID: "admin-node", TenantID: ws})

	allowed := func(op string) bool {
		return store.GetGraph().CheckAccess(user, oa, op).Decision == ngac.DecisionAllow
	}
	require.True(t, allowed(ops.OpRead), "fixture: member reads the OA")
	require.True(t, allowed(ops.OpWrite), "fixture: member writes the OA")

	refused := []struct {
		name    string
		service string
		call    func(pb.PolicyWriteServiceClient) error
	}{
		{"delete the OA via document", "document", func(c pb.PolicyWriteServiceClient) error {
			_, err := c.DeleteNode(admin, &pb.DeleteNodeRequest{NodeId: oa})
			return err
		}},
		{"delete the OA via approval", "approval", func(c pb.PolicyWriteServiceClient) error {
			_, err := c.DeleteNode(admin, &pb.DeleteNodeRequest{NodeId: oa})
			return err
		}},
		{"detach the member via asset", "asset", func(c pb.PolicyWriteServiceClient) error {
			_, err := c.RemoveAssignment(admin, &pb.RemoveAssignmentRequest{ChildId: user, ParentId: ua})
			return err
		}},
		{"drop the grant via drive", "drive", func(c pb.PolicyWriteServiceClient) error {
			_, err := c.RemoveAssociation(admin, &pb.RemoveAssociationRequest{UaId: ua, OaId: oa})
			return err
		}},
		{"prohibit write via messaging", "messaging", func(c pb.PolicyWriteServiceClient) error {
			_, err := c.CreateProhibition(admin, &pb.CreateProhibitionRequest{
				Name: "callers_" + sfx, SubjectId: ua, Operations: []string{ops.OpWrite}, TargetOaIds: []string{oa},
			})
			return err
		}},
		{"reload the graph via workspace", "workspace", func(c pb.PolicyWriteServiceClient) error {
			_, err := c.LoadGraph(admin, &pb.Empty{})
			return err
		}},
	}
	for _, r := range refused {
		err := r.call(via(r.service))
		require.Equal(t, codes.PermissionDenied, status.Code(err), "%s: %v", r.name, err)
		require.True(t, allowed(ops.OpRead), "%s: read must still be allowed", r.name)
		require.True(t, allowed(ops.OpWrite), "%s: write must still be allowed", r.name)
	}
	require.NotNil(t, store.GetNode(oa), "the OA survives every refused delete")

	// The same kind of write through the service that makes it lands, and the
	// decision follows: workspace narrows the grant (roles admin) to read.
	_, err = via("workspace").CreateAssociation(admin, &pb.CreateAssociationRequest{
		UaId: ua, OaId: oa, Operations: []string{ops.OpRead},
	})
	require.NoError(t, err)
	require.True(t, allowed(ops.OpRead), "read still granted")
	require.False(t, allowed(ops.OpWrite), "write withdrawn by the admitted write")
}
