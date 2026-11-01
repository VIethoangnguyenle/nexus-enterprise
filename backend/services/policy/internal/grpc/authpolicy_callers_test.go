package grpc_test

import (
	"context"
	"slices"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"

	"ngac-platform/pkg/grpcauth"
	pb "ngac-platform/proto/policy"
	pgrpc "ngac-platform/services/policy/internal/grpc"
	"ngac-platform/testutil"
)

// The services that hold a policy writer client and what they call on it,
// found at every call site in backend/services/*/ (direct calls, and the
// pkg/provision Creator, whose Node, Assign, Associate and rollback reach
// CreateNode, CreateAssignment, CreateAssociation and DeleteNode). These are
// written out here, not read from the policy under test, so a change to the
// table that drops a real caller or adds a new one fails this test.
var (
	provisioners = []string{"asset", "auth", "calendar", "drive", "messaging", "workspace"}
	graphReaders = append([]string{"approval"}, provisioners...)

	wantUserVia = map[string][]string{
		pb.PolicyWriteService_CreateNode_FullMethodName:         provisioners,
		pb.PolicyWriteService_DeleteNode_FullMethodName:         provisioners,
		pb.PolicyWriteService_CreateAssignment_FullMethodName:   provisioners,
		pb.PolicyWriteService_CreateAssociation_FullMethodName:  provisioners,
		pb.PolicyWriteService_RemoveAssignment_FullMethodName:   {"auth", "calendar", "drive", "messaging", "workspace"},
		pb.PolicyWriteService_RemoveAssociation_FullMethodName:  {"calendar", "workspace"},
		pb.PolicyWriteService_CreateProhibition_FullMethodName:  {"workspace"},
		pb.PolicyWriteService_RemoveProhibition_FullMethodName:  {"workspace"},
		pb.PolicyWriteService_RegisterOperations_FullMethodName: nil,
		pb.PolicyWriteService_InvalidateCache_FullMethodName:    nil,
		pb.PolicyWriteService_InitSchema_FullMethodName:         nil,
		pb.PolicyWriteService_LoadGraph_FullMethodName:          nil,
		pb.PolicyWriteService_GetAssociations_FullMethodName:    graphReaders,
		pb.PolicyWriteService_GetNode_FullMethodName:            graphReaders,
		pb.PolicyWriteService_FindNodeByName_FullMethodName:     graphReaders,
		pb.PolicyWriteService_IsAssigned_FullMethodName:         graphReaders,
		pb.PolicyWriteService_GetChildren_FullMethodName:        graphReaders,
		pb.PolicyWriteService_GetParents_FullMethodName:         graphReaders,
		pb.PolicyWriteService_GetAncestors_FullMethodName:       graphReaders,
		pb.PolicyWriteService_GetDescendants_FullMethodName:     graphReaders,
	}

	// Bare service identities, unchanged by the user allowlist.
	wantServiceOK = map[string][]string{
		pb.PolicyWriteService_CreateNode_FullMethodName:        {"auth", pgrpc.BackfillManagersService},
		pb.PolicyWriteService_CreateAssignment_FullMethodName:  {"auth", pgrpc.BackfillManagersService},
		pb.PolicyWriteService_DeleteNode_FullMethodName:        {"auth"},
		pb.PolicyWriteService_FindNodeByName_FullMethodName:    {pgrpc.BackfillManagersService},
		pb.PolicyWriteService_CreateAssociation_FullMethodName: {pgrpc.BackfillManagersService},
		pb.PolicyWriteService_InvalidateCache_FullMethodName:   {pgrpc.PolicyCtlService},
	}

	// Every name a request could arrive under: each real service, the operator
	// tools, a service that holds no writer (document), an unknown one, and none.
	callingServices = []string{
		"approval", "asset", "auth", "calendar", "document", "drive", "messaging", "workspace",
		pgrpc.BackfillManagersService, pgrpc.PolicyCtlService, "rogue", "",
	}
)

func serveWriter(t *testing.T, service string) *grpc.ClientConn {
	t.Helper()
	return testutil.ServeGRPCAs(t, service, pgrpc.AuthPolicy(), func(s *grpc.Server) {
		pb.RegisterPolicyWriteServiceServer(s, pb.UnimplementedPolicyWriteServiceServer{})
	})
}

// invoke calls method with an empty request: the unimplemented server answers
// Unimplemented once the request is admitted, so the code says only whether
// the gate let it through.
func invoke(ctx context.Context, conn *grpc.ClientConn, method string) error {
	return conn.Invoke(ctx, method, &pb.Empty{}, &pb.Empty{})
}

// A new writer RPC has to be placed in the table before it can be called by
// anyone; the test's own expectations must name it too.
func TestEveryWriterRPCHasACallerRule(t *testing.T) {
	rules := pgrpc.WriteCallers()
	for _, m := range pb.PolicyWriteService_ServiceDesc.Methods {
		full := "/" + pb.PolicyWriteService_ServiceDesc.ServiceName + "/" + m.MethodName
		if _, ok := rules[full]; !ok {
			t.Errorf("%s has no row in the writer's caller table", full)
		}
		if _, ok := wantUserVia[full]; !ok {
			t.Errorf("%s has no expectation in this test", full)
		}
	}
	if n := len(pb.PolicyWriteService_ServiceDesc.Streams); n != 0 {
		t.Errorf("the writer has %d streaming RPCs; give them caller rules and tests", n)
	}
}

// Deny first: for every writer RPC, a user request is refused unless the
// service that sent it is one that really makes that call, and a bare service
// identity is refused unless it is on the method's service allowlist.
func TestWriterAdmitsOnlyTheServicesThatMakeEachCall(t *testing.T) {
	user := grpcauth.WithCaller(context.Background(),
		grpcauth.Caller{UserID: "u", NGACNodeID: "n", TenantID: "t"})
	for _, svc := range callingServices {
		conn := serveWriter(t, svc)
		for method, allowed := range wantUserVia {
			want := codes.PermissionDenied
			if svc != "" && slices.Contains(allowed, svc) {
				want = codes.Unimplemented
			}
			wantCode(t, "user via "+quoted(svc)+" on "+method, invoke(user, conn, method), want)

			want = codes.Unauthenticated
			if svc != "" && slices.Contains(wantServiceOK[method], svc) {
				want = codes.Unimplemented
			}
			wantCode(t, "bare "+quoted(svc)+" on "+method, invoke(context.Background(), conn, method), want)
		}
	}
}

// The read service is not narrowed: any user, from any service, may ask it.
func TestReaderStaysOpenToUsersFromAnyService(t *testing.T) {
	user := grpcauth.WithCaller(context.Background(), grpcauth.Caller{UserID: "u", NGACNodeID: "n"})
	for _, svc := range []string{"document", "approval", ""} {
		_, r := servePolicyAs(t, svc)
		_, err := r.CheckAccess(user, &pb.CheckAccessRequest{})
		wantCode(t, "CheckAccess via "+quoted(svc), err, codes.Unimplemented)
		_, err = r.GetChildren(user, &pb.GetChildrenRequest{})
		wantCode(t, "reader GetChildren via "+quoted(svc), err, codes.Unimplemented)
	}
}

func quoted(s string) string { return "\"" + s + "\"" }
