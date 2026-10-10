// Package domain provides business logic orchestration for the workspace service.
// It delegates to store for persistence and policy clients for NGAC graph operations.
package domain

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/minio/minio-go/v7"

	"ngac-platform/ngac"
	"ngac-platform/pkg/provision"
	drivepb "ngac-platform/proto/drive"
	policypb "ngac-platform/proto/policy"
)

// Folder represents an NGAC OA folder in a workspace.
type Folder struct {
	ID         string
	Name       string
	NGACNodeID string
}

// CreateFolder provisions a new OA folder under a parent (or workspace PC if no
// parent). The caller must hold manage on the workspace's Mgmt OA, and a parent,
// if given, must be an OA of this workspace.
func (s *Service) CreateFolder(ctx context.Context, callerNodeID, wsID, name, parentOaID string) (*Folder, error) {
	ws, err := s.authorizeAdmin(ctx, callerNodeID, wsID, ngac.OpManage)
	if err != nil {
		return nil, err
	}
	if parentOaID != "" {
		if err := s.requireInWorkspace(ctx, ws, parentOaID, ngac.TypeOA); err != nil {
			return nil, err
		}
	}
	prov := provision.NewCreator(s.policyWrite)
	node, err := prov.Node(ctx, &policypb.CreateNodeRequest{
		Name:       ngac.FolderNodeName(ngac.FolderID(uuid.New().String())),
		NodeType:   ngac.TypeOA,
		Properties: map[string]string{ngac.PropDisplayName: name, "workspace_id": wsID},
	})
	if err != nil {
		return nil, fmt.Errorf("create folder: %w", err)
	}
	parentID := parentOaID
	if parentID == "" {
		parentID = ws.PcNodeID
	}
	if err := prov.Assign(ctx, node.Id, parentID); err != nil {
		return nil, prov.Fail(ctx, fmt.Errorf("assign folder: %w", err))
	}
	prov.Done()
	return &Folder{ID: node.Id, Name: name, NGACNodeID: node.Id}, nil
}

// ListFolders returns all OA folders under the workspace PC. The caller must
// belong to the workspace.
func (s *Service) ListFolders(ctx context.Context, callerNodeID, wsID string) ([]*Folder, error) {
	ws, err := s.authorizeMember(ctx, callerNodeID, wsID)
	if err != nil {
		return nil, err
	}
	desc, err := s.policyWrite.GetDescendants(ctx, &policypb.GetDescendantsRequest{NodeId: ws.PcNodeID})
	if err != nil {
		return nil, fmt.Errorf("get descendants: %w", err)
	}
	var folders []*Folder
	for _, n := range desc.Nodes {
		if n.NodeType == ngac.TypeOA {
			folders = append(folders, &Folder{ID: n.Id, Name: ngac.DisplayName(n.Name, n.Properties), NGACNodeID: n.Id})
		}
	}
	return folders, nil
}

// DeleteFolder removes a folder (NGAC OA node). The caller must hold manage on
// the workspace's Mgmt OA, and the folder must be an OA of this workspace.
func (s *Service) DeleteFolder(ctx context.Context, callerNodeID, wsID, folderID string) error {
	ws, err := s.authorizeAdmin(ctx, callerNodeID, wsID, ngac.OpManage)
	if err != nil {
		return err
	}
	if err := s.requireInWorkspace(ctx, ws, folderID, ngac.TypeOA); err != nil {
		return err
	}
	if _, err := s.policyWrite.DeleteNode(ctx, &policypb.DeleteNodeRequest{NodeId: folderID}); err != nil {
		return fmt.Errorf("delete folder: %w", err)
	}
	return nil
}

// ensureMinioBucket creates a MinIO bucket for the workspace (non-fatal).
func (s *Service) ensureMinioBucket(ctx context.Context, wsID string) {
	if s.minioClient == nil {
		return
	}
	bucketName := fmt.Sprintf("ws-%s", wsID)
	err := s.minioClient.MakeBucket(ctx, bucketName, minio.MakeBucketOptions{})
	if err != nil {
		exists, errExists := s.minioClient.BucketExists(ctx, bucketName)
		if errExists != nil || !exists {
			slog.Warn("failed to create minio bucket", "bucket", bucketName, "error", err)
		}
	} else {
		slog.Info("created minio bucket", "bucket", bucketName)
	}
}

// ensureDriveRoot creates the root drive folder for the workspace (non-fatal).
func (s *Service) ensureDriveRoot(ctx context.Context, wsID, wsName, docsOaID, ownersUaID string) {
	if s.driveClient == nil {
		return
	}
	_, err := s.driveClient.CreateDriveForChannel(ctx, &drivepb.CreateDriveForChannelRequest{
		WorkspaceId:     wsID,
		ChannelId:       wsID,
		ChannelName:     wsName,
		ChannelNgacOaId: docsOaID,
		ChannelNgacUaId: ownersUaID,
	})
	if err != nil {
		slog.Warn("failed to create workspace drive", "workspace", wsID, "error", err)
	}
}
