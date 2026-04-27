package grpc

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/minio/minio-go/v7"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	pb "ngac-platform/proto/document"
)

// DocumentStorageServer implements the pure storage API — no NGAC awareness.
// Access control is handled by the Drive Service before calling these RPCs.
type DocumentStorageServer struct {
	pb.UnimplementedDocumentStorageServiceServer
	db            *pgxpool.Pool
	minioClient   *minio.Client // for server-side ops (StatObject, PutObject, CopyObject)
	presignClient *minio.Client // for presigned URL generation with public endpoint
}

// NewDocumentStorageServer creates a storage handler backed by MinIO.
func NewDocumentStorageServer(db *pgxpool.Pool, mc *minio.Client, presignMC *minio.Client) *DocumentStorageServer {
	return &DocumentStorageServer{db: db, minioClient: mc, presignClient: presignMC}
}

// bucketName derives the MinIO bucket name from a workspace ID.
func bucketName(workspaceID string) string {
	return fmt.Sprintf("ws-%s", workspaceID)
}

// ensureBucket creates the workspace bucket if it doesn't exist.
func (s *DocumentStorageServer) ensureBucket(ctx context.Context, bucket string) {
	exists, err := s.minioClient.BucketExists(ctx, bucket)
	if err != nil || exists {
		return
	}
	if err := s.minioClient.MakeBucket(ctx, bucket, minio.MakeBucketOptions{}); err != nil {
		slog.Warn("make bucket failed", "bucket", bucket, "error", err)
	}
}

// GetUploadURL generates a presigned PUT URL for direct-to-MinIO upload.
func (s *DocumentStorageServer) GetUploadURL(ctx context.Context, req *pb.GetUploadURLRequest) (*pb.GetUploadURLResponse, error) {
	bucket := bucketName(req.WorkspaceId)
	s.ensureBucket(ctx, bucket)

	key := fmt.Sprintf("drive/%s/%s", req.DocId, req.Filename)

	presignedURL, err := s.presignClient.PresignedPutObject(ctx, bucket, key, 5*time.Minute)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "generate upload URL: %v", err)
	}

	return &pb.GetUploadURLResponse{
		UploadUrl: presignedURL.String(),
		ObjectKey: key,
	}, nil
}

// ConfirmUpload verifies the file exists in MinIO and returns its metadata.
func (s *DocumentStorageServer) ConfirmUpload(ctx context.Context, req *pb.ConfirmUploadRequest) (*pb.ConfirmUploadResponse, error) {
	bucket := bucketName(req.WorkspaceId)

	info, err := s.minioClient.StatObject(ctx, bucket, req.ObjectKey, minio.StatObjectOptions{})
	if err != nil {
		return nil, status.Errorf(codes.FailedPrecondition, "file not uploaded: %v", err)
	}

	return &pb.ConfirmUploadResponse{
		SizeBytes:   info.Size,
		ContentType: info.ContentType,
	}, nil
}

// GetDownloadURL generates a presigned GET URL for downloading a file.
func (s *DocumentStorageServer) GetDownloadURL(ctx context.Context, req *pb.GetDownloadURLRequest) (*pb.GetDownloadURLResponse, error) {
	bucket := bucketName(req.WorkspaceId)

	presignedURL, err := s.presignClient.PresignedGetObject(ctx, bucket, req.ObjectKey, 15*time.Minute, nil)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "generate download URL: %v", err)
	}

	return &pb.GetDownloadURLResponse{DownloadUrl: presignedURL.String()}, nil
}

// DeleteObject removes an object from MinIO.
func (s *DocumentStorageServer) DeleteObject(ctx context.Context, req *pb.DeleteObjectRequest) (*pb.Empty, error) {
	bucket := bucketName(req.WorkspaceId)
	err := s.minioClient.RemoveObject(ctx, bucket, req.ObjectKey, minio.RemoveObjectOptions{})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "delete object: %v", err)
	}
	return &pb.Empty{}, nil
}

// CopyObject performs a server-side copy of an object in MinIO.
func (s *DocumentStorageServer) CopyObject(ctx context.Context, req *pb.CopyObjectRequest) (*pb.CopyObjectResponse, error) {
	srcBucket := bucketName(req.SrcWorkspaceId)
	dstBucket := bucketName(req.DstWorkspaceId)
	s.ensureBucket(ctx, dstBucket)

	src := minio.CopySrcOptions{Bucket: srcBucket, Object: req.SrcObjectKey}
	dst := minio.CopyDestOptions{Bucket: dstBucket, Object: req.DstObjectKey}

	info, err := s.minioClient.CopyObject(ctx, dst, src)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "copy object: %v", err)
	}

	return &pb.CopyObjectResponse{
		ObjectKey: req.DstObjectKey,
		SizeBytes: info.Size,
	}, nil
}

// GetObjectInfo returns metadata about an object in MinIO.
func (s *DocumentStorageServer) GetObjectInfo(ctx context.Context, req *pb.GetObjectInfoRequest) (*pb.ObjectInfo, error) {
	bucket := bucketName(req.WorkspaceId)

	info, err := s.minioClient.StatObject(ctx, bucket, req.ObjectKey, minio.StatObjectOptions{})
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "object not found: %v", err)
	}

	return &pb.ObjectInfo{
		ObjectKey:    req.ObjectKey,
		SizeBytes:    info.Size,
		ContentType:  info.ContentType,
		LastModified: timestamppb.New(info.LastModified),
	}, nil
}

// --- Legacy compatibility: PutObject for internal use ---

// PutObjectDirect stores content directly (used by legacy Upload flow during migration).
func (s *DocumentStorageServer) PutObjectDirect(ctx context.Context, bucket, key string, content []byte, contentType string) error {
	_, err := s.minioClient.PutObject(ctx, bucket, key,
		bytes.NewReader(content), int64(len(content)),
		minio.PutObjectOptions{ContentType: contentType})
	return err
}
