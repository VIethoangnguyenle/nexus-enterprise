package grpc_test

import (
	"context"
	"reflect"
	"sort"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"ngac-platform/ngac"
	pb "ngac-platform/proto/asset"
	"ngac-platform/services/asset/internal/domain"
	agrpc "ngac-platform/services/asset/internal/grpc"
	"ngac-platform/services/asset/internal/store"
)

// A personal workspace has no assets and no asset requests. Every method of the
// three asset servers refuses there, before any policy check, even for a caller
// who holds every operation on every OA; an organization workspace is untouched.

type assetServers struct {
	assets   *agrpc.AssetServer
	types    *agrpc.AssetTypeServer
	requests *agrpc.AssetRequestServer
}

type assetCall struct {
	server string
	method string
	// mustSucceed marks reads that have to answer in an organization workspace;
	// the rest only have to get past the workspace-type guard.
	mustSucceed bool
	run         func(ctx context.Context, f *fixture, s assetServers) error
}

func assetCalls() []assetCall {
	return []assetCall{
		{"AssetServer", "CreateAsset", false, func(ctx context.Context, f *fixture, s assetServers) error {
			_, err := s.assets.CreateAsset(ctx, &pb.CreateAssetRequest{WorkspaceId: f.wsID, TypeId: f.typeA, Name: "laptop"})
			return err
		}},
		{"AssetServer", "GetAsset", true, func(ctx context.Context, f *fixture, s assetServers) error {
			_, err := s.assets.GetAsset(ctx, &pb.GetAssetRequest{AssetId: f.assetA})
			return err
		}},
		{"AssetServer", "ListAssets", true, func(ctx context.Context, f *fixture, s assetServers) error {
			_, err := s.assets.ListAssets(ctx, &pb.ListAssetsRequest{WorkspaceId: f.wsID})
			return err
		}},
		{"AssetServer", "UpdateAsset", false, func(ctx context.Context, f *fixture, s assetServers) error {
			_, err := s.assets.UpdateAsset(ctx, &pb.UpdateAssetRequest{AssetId: f.assetA, Name: "renamed"})
			return err
		}},
		{"AssetServer", "DeleteAsset", false, func(ctx context.Context, f *fixture, s assetServers) error {
			_, err := s.assets.DeleteAsset(ctx, &pb.DeleteAssetRequest{AssetId: f.assetB})
			return err
		}},
		{"AssetServer", "TransitionAsset", false, func(ctx context.Context, f *fixture, s assetServers) error {
			_, err := s.assets.TransitionAsset(ctx, &pb.TransitionRequest{AssetId: f.assetA, Action: "retire"})
			return err
		}},
		{"AssetServer", "GetAvailableTransitions", true, func(ctx context.Context, f *fixture, s assetServers) error {
			_, err := s.assets.GetAvailableTransitions(ctx, &pb.GetTransitionsRequest{AssetId: f.assetA})
			return err
		}},
		{"AssetServer", "GetAssetHistory", true, func(ctx context.Context, f *fixture, s assetServers) error {
			_, err := s.assets.GetAssetHistory(ctx, &pb.GetHistoryRequest{AssetId: f.assetA})
			return err
		}},
		{"AssetServer", "HandOverAsset", false, func(ctx context.Context, f *fixture, s assetServers) error {
			_, err := s.assets.HandOverAsset(ctx, &pb.HandOverRequest{AssetId: f.assetA, AssigneeId: f.userY})
			return err
		}},
		{"AssetServer", "GetSummary", true, func(ctx context.Context, f *fixture, s assetServers) error {
			_, err := s.assets.GetSummary(ctx, &pb.GetSummaryRequest{WorkspaceId: f.wsID})
			return err
		}},
		{"AssetServer", "ListActivity", true, func(ctx context.Context, f *fixture, s assetServers) error {
			_, err := s.assets.ListActivity(ctx, &pb.ListActivityRequest{WorkspaceId: f.wsID})
			return err
		}},

		{"AssetTypeServer", "CreateType", false, func(ctx context.Context, f *fixture, s assetServers) error {
			_, err := s.types.CreateType(ctx, &pb.CreateTypeRequest{WorkspaceId: f.wsID, Name: "Phones", Category: "hardware"})
			return err
		}},
		{"AssetTypeServer", "GetType", true, func(ctx context.Context, f *fixture, s assetServers) error {
			_, err := s.types.GetType(ctx, &pb.GetTypeRequest{TypeId: f.typeA})
			return err
		}},
		{"AssetTypeServer", "ListTypes", true, func(ctx context.Context, f *fixture, s assetServers) error {
			_, err := s.types.ListTypes(ctx, &pb.ListTypesRequest{WorkspaceId: f.wsID})
			return err
		}},
		{"AssetTypeServer", "UpdateTypeSchema", false, func(ctx context.Context, f *fixture, s assetServers) error {
			_, err := s.types.UpdateTypeSchema(ctx, &pb.UpdateTypeSchemaRequest{TypeId: f.typeA, FieldsSchema: "{}"})
			return err
		}},

		{"AssetRequestServer", "CreateRequest", false, func(ctx context.Context, f *fixture, s assetServers) error {
			_, err := s.requests.CreateRequest(ctx, &pb.CreateAssetRequestReq{WorkspaceId: f.wsID, TypeId: f.typeA, Justification: "need one"})
			return err
		}},
		{"AssetRequestServer", "ApproveRequest", false, func(ctx context.Context, f *fixture, s assetServers) error {
			_, err := s.requests.ApproveRequest(ctx, &pb.ApproveRequestReq{RequestId: f.reqByXonA})
			return err
		}},
		{"AssetRequestServer", "RejectRequest", false, func(ctx context.Context, f *fixture, s assetServers) error {
			_, err := s.requests.RejectRequest(ctx, &pb.RejectRequestReq{RequestId: f.reqByYB, Reason: "no"})
			return err
		}},
		{"AssetRequestServer", "AssignAsset", false, func(ctx context.Context, f *fixture, s assetServers) error {
			_, err := s.requests.AssignAsset(ctx, &pb.AssignAssetReq{RequestId: f.reqByXonA, AssetId: f.assetA})
			return err
		}},
		{"AssetRequestServer", "ReturnAsset", false, func(ctx context.Context, f *fixture, s assetServers) error {
			_, err := s.requests.ReturnAsset(ctx, &pb.ReturnAssetReq{AssetId: f.assetA})
			return err
		}},
		{"AssetRequestServer", "ListRequests", true, func(ctx context.Context, f *fixture, s assetServers) error {
			_, err := s.requests.ListRequests(ctx, &pb.ListRequestsReq{WorkspaceId: f.wsID})
			return err
		}},
		{"AssetRequestServer", "GetRequest", true, func(ctx context.Context, f *fixture, s assetServers) error {
			_, err := s.requests.GetRequest(ctx, &pb.GetRequestReq{RequestId: f.reqByXonA})
			return err
		}},
	}
}

// fullAccess is a policy in which the caller holds every operation on every OA
// the fixture has, so the workspace-type guard is the only thing that can refuse.
func (f *fixture) fullAccess(node string) *fakePolicyRead {
	p := f.policy()
	for _, oa := range []string{f.oaA, f.oaB, f.assetsOA(), f.mgmtOA()} {
		for _, op := range ngac.AllOwnerOps() {
			p.grant(node, oa, op)
		}
	}
	return p
}

func (f *fixture) servers(p *fakePolicyRead) assetServers {
	// CreateType wires its new OA to a real node (asset_types.ngac_oa_id is a FK).
	w := &fakePolicyWrite{nodeID: f.oaA}
	return assetServers{
		assets:   agrpc.NewAssetServer(f.st, p, nil),
		types:    agrpc.NewAssetTypeServer(f.st, p, w),
		requests: agrpc.NewAssetRequestServer(f.st, p, w, nil),
	}
}

func (f *fixture) setWorkspaceType(t *testing.T, kind string) {
	t.Helper()
	_, err := f.pool.Exec(context.Background(), `UPDATE workspaces SET type = $1 WHERE id = $2`, kind, f.wsID)
	require.NoError(t, err)
}

func refusalReason(err error) string {
	for _, d := range status.Convert(err).Details() {
		if info, ok := d.(*errdetails.ErrorInfo); ok {
			return info.GetReason()
		}
	}
	return ""
}

func TestPersonalWorkspace_EveryAssetMethodIsRefused(t *testing.T) {
	for _, c := range assetCalls() {
		t.Run(c.server+"."+c.method, func(t *testing.T) {
			f := newFixture(t)
			f.setWorkspaceType(t, "personal")
			p := f.fullAccess("n-all")
			// A different user from the one who filed request A, so the only
			// possible refusal is the workspace type.
			ctx := asCaller(f.userY, "n-all")

			err := c.run(ctx, f, f.servers(p))

			require.Error(t, err)
			assert.Equal(t, codes.PermissionDenied, status.Code(err), err.Error())
			assert.Equal(t, domain.ReasonPersonalWorkspace, refusalReason(err))
			assert.NotContains(t, status.Convert(err).Message(), f.wsID, "the refusal names no ID")
			assert.Empty(t, p.checks, "the workspace type is decided before any policy check")
		})
	}
}

func TestOrganizationWorkspace_AssetMethodsGetPastTheGuard(t *testing.T) {
	for _, c := range assetCalls() {
		t.Run(c.server+"."+c.method, func(t *testing.T) {
			f := newFixture(t)
			p := f.fullAccess("n-all")
			ctx := asCaller(f.userY, "n-all")

			err := c.run(ctx, f, f.servers(p))

			if c.mustSucceed {
				require.NoError(t, err)
			}
			assert.NotEqual(t, domain.ReasonPersonalWorkspace, refusalReason(err))
		})
	}
}

// Both lists must stay in step with the servers: a new RPC without a row here
// is an entry point nobody proved refuses in a personal workspace.
func TestPersonalWorkspace_TableCoversEveryRPC(t *testing.T) {
	want := map[string][]string{}
	for _, srv := range []any{&agrpc.AssetServer{}, &agrpc.AssetTypeServer{}, &agrpc.AssetRequestServer{}} {
		typ := reflect.TypeOf(srv)
		name := typ.Elem().Name()
		for i := 0; i < typ.NumMethod(); i++ {
			want[name] = append(want[name], typ.Method(i).Name)
		}
	}
	got := map[string][]string{}
	for _, c := range assetCalls() {
		got[c.server] = append(got[c.server], c.method)
	}
	for k := range want {
		sort.Strings(want[k])
		sort.Strings(got[k])
	}
	assert.Equal(t, want, got)
}

func TestWorkspaceGuard_FailsClosed(t *testing.T) {
	t.Run("unknown workspace is refused", func(t *testing.T) {
		f := newFixture(t)
		p := f.fullAccess("n-all")
		_, err := f.servers(p).assets.ListAssets(asCaller(f.userY, "n-all"), &pb.ListAssetsRequest{WorkspaceId: "no-such-workspace"})
		assert.Equal(t, codes.PermissionDenied, status.Code(err))
	})
	t.Run("an unreadable workspace type is an internal error, never an allow", func(t *testing.T) {
		f := newFixture(t)
		p := f.fullAccess("n-all")
		dead, err := pgxpool.New(context.Background(), testDBURL())
		require.NoError(t, err)
		dead.Close()
		srv := agrpc.NewAssetServer(store.New(dead), p, nil)

		_, err = srv.ListAssets(asCaller(f.userY, "n-all"), &pb.ListAssetsRequest{WorkspaceId: f.wsID})

		assert.Equal(t, codes.Internal, status.Code(err))
		assert.Empty(t, p.checks)
	})
}
