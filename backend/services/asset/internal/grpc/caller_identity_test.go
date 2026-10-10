package grpc_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"ngac-platform/ngac"
	"ngac-platform/pkg/grpcauth"
	"ngac-platform/testutil"

	pb "ngac-platform/proto/asset"
	agrpc "ngac-platform/services/asset/internal/grpc"
)

// serveAsset exposes the asset servers over a real gRPC connection, so the
// caller travels as metadata exactly as it does between services.
func serveAsset(t *testing.T, f *fixture, p *fakePolicyRead) (pb.AssetServiceClient, pb.AssetTypeServiceClient, pb.AssetRequestServiceClient) {
	t.Helper()
	conn := testutil.ServeGRPC(t, grpcauth.ServerPolicy{}, func(s *grpc.Server) {
		pb.RegisterAssetServiceServer(s, agrpc.NewAssetServer(f.st, p, &fakePolicyWrite{}, nil))
		pb.RegisterAssetTypeServiceServer(s, agrpc.NewAssetTypeServer(f.st, p, &fakePolicyWrite{}))
		pb.RegisterAssetRequestServiceServer(s, agrpc.NewAssetRequestServer(f.st, p, &fakePolicyWrite{}, nil))
	})
	return pb.NewAssetServiceClient(conn), pb.NewAssetTypeServiceClient(conn), pb.NewAssetRequestServiceClient(conn)
}

func TestOverTheWire_MissingCallerIsUnauthenticated(t *testing.T) {
	f := newFixture(t)
	assets, types, reqs := serveAsset(t, f, f.policy())
	ctx := context.Background()

	_, err := assets.ListAssets(ctx, &pb.ListAssetsRequest{WorkspaceId: f.wsID})
	assert.Equal(t, codes.Unauthenticated, status.Code(err))
	_, err = types.GetType(ctx, &pb.GetTypeRequest{TypeId: f.typeA})
	assert.Equal(t, codes.Unauthenticated, status.Code(err))
	_, err = reqs.GetRequest(ctx, &pb.GetRequestReq{RequestId: f.reqByXonA})
	assert.Equal(t, codes.Unauthenticated, status.Code(err))
}

// The body names a reader, the metadata names someone with no grants: the
// decision follows the metadata.
func TestOverTheWire_BodyCallerIsIgnored_Deny(t *testing.T) {
	f := newFixture(t)
	p := f.policy()
	p.grant("n-reader", f.oaA, ngac.OpRead)
	assets, _, _ := serveAsset(t, f, p)

	list, err := assets.ListAssets(asCaller("u-other", "n-other"), &pb.ListAssetsRequest{
		WorkspaceId:    f.wsID,
		UserNgacNodeId: "n-reader", //lint:ignore SA1019 proves the deprecated body field is not trusted
	})

	require.NoError(t, err)
	assert.Empty(t, list.Assets)
}

// And the other way round: a body naming nobody does not hide the reader the
// metadata names.
func TestOverTheWire_MetadataCallerDecides_Allow(t *testing.T) {
	f := newFixture(t)
	p := f.policy()
	p.grant("n-reader", f.oaA, ngac.OpRead)
	assets, _, _ := serveAsset(t, f, p)

	list, err := assets.ListAssets(asCaller("u-reader", "n-reader"), &pb.ListAssetsRequest{
		WorkspaceId:    f.wsID,
		UserNgacNodeId: "n-other", //lint:ignore SA1019 proves the deprecated body field is not trusted
	})

	require.NoError(t, err)
	assert.Equal(t, []string{f.assetA}, assetIDs(list))
}

// GetType used to read an in-process context value and denied every call that
// arrived over the network. With the caller in metadata it works.
func TestOverTheWire_GetTypeWorksForAuthorizedCaller(t *testing.T) {
	f := newFixture(t)
	p := f.policy()
	p.grant("n-reader", f.assetsOA(), ngac.OpRead)
	_, types, _ := serveAsset(t, f, p)

	at, err := types.GetType(asCaller("u-reader", "n-reader"), &pb.GetTypeRequest{TypeId: f.typeA})
	require.NoError(t, err)
	assert.Equal(t, f.typeA, at.Id)

	_, err = types.GetType(asCaller("u-other", "n-other"), &pb.GetTypeRequest{TypeId: f.typeA})
	assert.Equal(t, codes.PermissionDenied, status.Code(err))
}
