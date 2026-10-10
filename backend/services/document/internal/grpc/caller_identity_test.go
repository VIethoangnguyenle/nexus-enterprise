package grpc_test

import (
	"context"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"ngac-platform/pkg/grpcauth"
	pb "ngac-platform/proto/document"
	"ngac-platform/testutil"
)

// The storage server does no authorization of its own (drive does it before
// calling), but it must still refuse a request that carries no caller. The
// Unimplemented server answers Unimplemented once a request is admitted.
func TestStorageRefusesRequestsWithoutACaller(t *testing.T) {
	conn := testutil.ServeGRPC(t, grpcauth.ServerPolicy{Exempt: grpcauth.HealthExempt()}, func(s *grpc.Server) {
		pb.RegisterDocumentStorageServiceServer(s, pb.UnimplementedDocumentStorageServiceServer{})
	})
	c := pb.NewDocumentStorageServiceClient(conn)

	_, err := c.GetDownloadURL(context.Background(), &pb.GetDownloadURLRequest{})
	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("no caller: code = %v, want Unauthenticated", status.Code(err))
	}
	_, err = c.DeleteObject(context.Background(), &pb.DeleteObjectRequest{})
	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("no caller: code = %v, want Unauthenticated", status.Code(err))
	}

	ctx := grpcauth.WithCaller(context.Background(), grpcauth.Caller{UserID: "u", NGACNodeID: "n"})
	_, err = c.GetDownloadURL(ctx, &pb.GetDownloadURLRequest{})
	if status.Code(err) != codes.Unimplemented {
		t.Fatalf("with caller: code = %v, want Unimplemented (admitted)", status.Code(err))
	}
}

// Metadata an attacker can invent without the signing secret is not an
// identity, whatever user it names.
func TestStorageRefusesForgedIdentity(t *testing.T) {
	conn := testutil.ServeGRPC(t, grpcauth.ServerPolicy{Exempt: grpcauth.HealthExempt()}, func(s *grpc.Server) {
		pb.RegisterDocumentStorageServiceServer(s, pb.UnimplementedDocumentStorageServiceServer{})
	})
	c := pb.NewDocumentStorageServiceClient(testutil.Unsigned(t, conn))

	_, err := c.GetDownloadURL(testutil.ForgedIdentity(context.Background()), &pb.GetDownloadURLRequest{})
	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("forged identity: code = %v, want Unauthenticated", status.Code(err))
	}
}
