package grpc_test

import (
	"github.com/jackc/pgx/v5/pgxpool"

	docpb "ngac-platform/proto/document"
	policypb "ngac-platform/proto/policy"
	"ngac-platform/services/drive/internal/domain"
	grpcserver "ngac-platform/services/drive/internal/grpc"
	"ngac-platform/services/drive/internal/store"
)

// newDrive builds the gRPC server over a domain service on pool, the way the
// service's main wires them.
func newDrive(
	pool *pgxpool.Pool,
	pr policypb.PolicyReadServiceClient,
	pw policypb.PolicyWriteServiceClient,
	ds docpb.DocumentStorageServiceClient,
) *grpcserver.DriveServer {
	return grpcserver.NewServer(domain.NewService(store.NewStore(pool), pr, pw, ds))
}
