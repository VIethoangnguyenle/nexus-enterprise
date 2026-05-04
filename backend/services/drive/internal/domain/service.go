// Package domain provides business logic for the drive service.
// Contains access control and root folder management extracted from the gRPC handler.
package domain

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/google/uuid"

	"ngac-platform/ngac"
	policypb "ngac-platform/proto/policy"
	"ngac-platform/services/drive/internal/store"
)

// DriveStore defines persistence operations the domain needs.
type DriveStore interface {
	GetItem(ctx context.Context, id string) (*store.DriveItem, error)
	InsertItem(ctx context.Context, item *store.DriveItem) error
	FindRootByContext(ctx context.Context, workspaceID, driveCtx, driveCtxID string) (*store.DriveItem, error)
	GetWorkspacePCID(ctx context.Context, workspaceID string) (string, error)
	CheckQuota(ctx context.Context, workspaceID string, sizeBytes int64) (bool, error)
}

// Service provides domain operations for the drive service.
type Service struct {
	store       DriveStore
	policyRead  policypb.PolicyReadServiceClient
	policyWrite policypb.PolicyWriteServiceClient
}

// NewService creates a drive domain service.
func NewService(st DriveStore, pr policypb.PolicyReadServiceClient, pw policypb.PolicyWriteServiceClient) *Service {
	return &Service{store: st, policyRead: pr, policyWrite: pw}
}

// CheckAccess verifies that a user has the given operation on an NGAC object node.
func (s *Service) CheckAccess(ctx context.Context, userNodeID, objectNodeID, operation string) error {
	resp, err := s.policyRead.CheckAccess(ctx, &policypb.CheckAccessRequest{
		UserNodeId: userNodeID, ObjectNodeId: objectNodeID, Operation: operation,
	})
	if err != nil {
		return fmt.Errorf("check access: %w", err)
	}
	if resp.Decision == ngac.DecisionDeny {
		return ErrAccessDenied
	}
	return nil
}

// CheckQuota verifies workspace storage quota.
func (s *Service) CheckQuota(ctx context.Context, workspaceID string, sizeBytes int64) error {
	ok, err := s.store.CheckQuota(ctx, workspaceID, sizeBytes)
	if err != nil {
		return fmt.Errorf("check quota: %w", err)
	}
	if !ok {
		return fmt.Errorf("%w: storage quota exceeded", ErrInvalidInput)
	}
	return nil
}

// EnsureRoot finds or auto-creates the root drive folder for a workspace/channel context.
// Self-heals workspaces created before drive tables existed.
func (s *Service) EnsureRoot(ctx context.Context, workspaceID, driveCtx, driveCtxID, userNodeID string) (*store.DriveItem, error) {
	root, err := s.store.FindRootByContext(ctx, workspaceID, driveCtx, driveCtxID)
	if err != nil {
		return nil, fmt.Errorf("find root: %w", err)
	}
	if root != nil {
		return root, nil
	}

	ngacNodeID, err := s.findDocumentsOA(ctx, workspaceID)
	if err != nil {
		return nil, err
	}

	contextID := driveCtxID
	if contextID == "" {
		contextID = workspaceID
	}
	item := &store.DriveItem{
		ID:             uuid.New().String(),
		WorkspaceID:    workspaceID,
		DriveContext:   driveCtx,
		DriveContextID: contextID,
		ItemType:       "folder",
		Name:           "Root",
		NGACNodeID:     ngacNodeID,
		ScopeOAID:      ngacNodeID,
		OwnerID:        userNodeID,
		Status:         "active",
	}
	if err := s.store.InsertItem(ctx, item); err != nil {
		return nil, fmt.Errorf("insert root: %w", err)
	}

	slog.Info("auto-created drive root", "workspace", workspaceID, "context", driveCtx, "root_id", item.ID)
	return item, nil
}

// findDocumentsOA locates the Documents OA node under a workspace PC, or creates a fallback.
func (s *Service) findDocumentsOA(ctx context.Context, workspaceID string) (string, error) {
	pcID, err := s.store.GetWorkspacePCID(ctx, workspaceID)
	if err != nil || pcID == "" {
		return s.createFallbackOA(ctx, workspaceID, "")
	}

	desc, err := s.policyRead.GetChildren(ctx, &policypb.GetChildrenRequest{NodeId: pcID})
	if err != nil {
		return s.createFallbackOA(ctx, workspaceID, pcID)
	}

	var firstOA string
	for _, n := range desc.Nodes {
		if n.NodeType != ngac.TypeOA || n.Name == "" {
			continue
		}
		if firstOA == "" {
			firstOA = n.Id
		}
		if strings.Contains(n.Name, "Documents") || strings.Contains(n.Name, "Docs") {
			return n.Id, nil
		}
	}
	if firstOA != "" {
		return firstOA, nil
	}
	return s.createFallbackOA(ctx, workspaceID, pcID)
}

// createFallbackOA creates a new OA node for a workspace root when no Documents OA exists.
func (s *Service) createFallbackOA(ctx context.Context, workspaceID, pcID string) (string, error) {
	node, err := s.policyWrite.CreateNode(ctx, &policypb.CreateNodeRequest{
		Name: fmt.Sprintf("DriveRoot_%s", workspaceID[:8]), NodeType: ngac.TypeOA,
	})
	if err != nil {
		return "", fmt.Errorf("create root node: %w", err)
	}
	if pcID != "" {
		if _, err := s.policyWrite.CreateAssignment(ctx, &policypb.CreateAssignmentRequest{
			ChildId: node.Id, ParentId: pcID,
		}); err != nil {
			slog.Warn("failed to assign drive root to PC", "error", err)
		}
	}
	return node.Id, nil
}

// CreateNGACFolder creates an NGAC OA node and assigns it under a parent.
func (s *Service) CreateNGACFolder(ctx context.Context, name, workspaceID, parentNGACID string) (string, error) {
	folderNode, err := s.policyWrite.CreateNode(ctx, &policypb.CreateNodeRequest{
		Name: ngac.FolderNodeName(name), NodeType: ngac.TypeOA,
		Properties: map[string]string{"type": "drive_folder", "workspace_id": workspaceID},
	})
	if err != nil {
		return "", fmt.Errorf("create folder node: %w", err)
	}
	if _, err := s.policyWrite.CreateAssignment(ctx, &policypb.CreateAssignmentRequest{
		ChildId: folderNode.Id, ParentId: parentNGACID,
	}); err != nil {
		return "", fmt.Errorf("assign folder node: %w", err)
	}
	return folderNode.Id, nil
}

// Note: Files do NOT need NGAC nodes. They inherit permissions from their
// parent folder's OA node. Only folders/containers need CreateNode (OA type).


// ResolveParent resolves the parent NGAC node for an item, using ensureRoot for root-level items.
func (s *Service) ResolveParent(ctx context.Context, parentItemID, workspaceID, driveCtx, driveCtxID, userNodeID string) (parentNGACID string, scopeOAID string, err error) {
	if parentItemID != "" {
		parent, err := s.store.GetItem(ctx, parentItemID)
		if err != nil || parent == nil {
			return "", "", ErrNotFound
		}
		if err := s.CheckAccess(ctx, userNodeID, parent.NGACNodeID, ngac.OpWrite); err != nil {
			return "", "", err
		}
		return parent.NGACNodeID, parent.ScopeOAID, nil
	}

	root, err := s.EnsureRoot(ctx, workspaceID, driveCtx, driveCtxID, userNodeID)
	if err != nil {
		return "", "", err
	}
	return root.NGACNodeID, root.ScopeOAID, nil
}
