package grpc

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	pb "ngac-platform/proto/document"
	policypb "ngac-platform/proto/policy"
)

type DocumentServer struct {
	pb.UnimplementedDocumentServiceServer
	db           *pgxpool.Pool
	policyClient policypb.PolicyServiceClient
	dataDir      string
}

func NewDocumentServer(db *pgxpool.Pool, pc policypb.PolicyServiceClient, dataDir string) *DocumentServer {
	os.MkdirAll(dataDir, 0755)
	return &DocumentServer{db: db, policyClient: pc, dataDir: dataDir}
}

func (s *DocumentServer) Upload(ctx context.Context, req *pb.UploadRequest) (*pb.Document, error) {
	// Create NGAC Object node for the document
	docNode, err := s.policyClient.CreateNode(ctx, &policypb.CreateNodeRequest{
		Name: req.Title, NodeType: "O",
		Properties: map[string]string{"type": "document", "workspace_id": req.WorkspaceId},
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "create doc node: %v", err)
	}

	// Find workspace's DraftDocs OA and assign doc to it.
	// Look up the workspace to get the PC node, then find DraftDocs OA among descendants.
	var pcNodeID string
	s.db.QueryRow(ctx, "SELECT ngac_pc_id FROM workspaces WHERE id = $1", req.WorkspaceId).Scan(&pcNodeID)
	if pcNodeID != "" {
		desc, _ := s.policyClient.GetDescendants(ctx, &policypb.GetDescendantsRequest{NodeId: pcNodeID})
		if desc != nil {
			for _, n := range desc.Nodes {
				if n.NodeType == "OA" && strings.HasSuffix(n.Name, "_DraftDocs") {
					s.policyClient.CreateAssignment(ctx, &policypb.CreateAssignmentRequest{
						ChildId: docNode.Id, ParentId: n.Id,
					})
					break
				}
			}
		}
	}

	// Save file to disk
	docID := uuid.New().String()
	filePath := filepath.Join(s.dataDir, docID+"_"+req.Filename)
	if err := os.WriteFile(filePath, req.Content, 0644); err != nil {
		return nil, status.Errorf(codes.Internal, "save file: %v", err)
	}

	// Get owner name
	ownerName := ""

	// Insert into DB
	_, err = s.db.Exec(ctx,
		"INSERT INTO documents (id, title, filename, mime_type, owner_id, ngac_node, workspace_id) VALUES ($1, $2, $3, $4, $5, $6, $7)",
		docID, req.Title, req.Filename, req.MimeType, req.UserId, docNode.Id, req.WorkspaceId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "insert document: %v", err)
	}

	return &pb.Document{
		Id: docID, Title: req.Title, Filename: req.Filename,
		MimeType: req.MimeType, OwnerId: req.UserId, OwnerName: ownerName,
		NgacNodeId: docNode.Id, Status: "draft", WorkspaceId: req.WorkspaceId,
		CreatedAt: timestamppb.Now(),
	}, nil
}

func (s *DocumentServer) Get(ctx context.Context, req *pb.GetDocumentRequest) (*pb.Document, error) {
	var d pb.Document
	var ngacNode, wsID string
	err := s.db.QueryRow(ctx,
		"SELECT d.id, d.title, d.filename, d.mime_type, d.owner_id, COALESCE(u.username,''), d.ngac_node, COALESCE(d.workspace_id,''), d.created_at FROM documents d LEFT JOIN users u ON d.owner_id = u.id WHERE d.id = $1",
		req.DocumentId).Scan(&d.Id, &d.Title, &d.Filename, &d.MimeType, &d.OwnerId, &d.OwnerName, &ngacNode, &wsID, &d.CreatedAt)
	if err == pgx.ErrNoRows {
		return nil, status.Errorf(codes.NotFound, "document not found")
	}
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get document: %v", err)
	}
	d.NgacNodeId = ngacNode
	d.WorkspaceId = wsID

	// Check access
	resp, _ := s.policyClient.CheckAccess(ctx, &policypb.CheckAccessRequest{
		UserNodeId: req.UserNgacNodeId, ObjectNodeId: ngacNode, Operation: "read",
	})
	if resp != nil && resp.Decision == "DENY" {
		return nil, status.Errorf(codes.PermissionDenied, "access denied")
	}
	return &d, nil
}

func (s *DocumentServer) List(ctx context.Context, req *pb.ListDocumentsRequest) (*pb.DocumentList, error) {
	rows, err := s.db.Query(ctx,
		"SELECT d.id, d.title, d.filename, d.mime_type, d.owner_id, COALESCE(u.username,''), d.ngac_node, COALESCE(d.workspace_id,''), d.created_at FROM documents d LEFT JOIN users u ON d.owner_id = u.id WHERE d.workspace_id = $1 ORDER BY d.created_at DESC",
		req.WorkspaceId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list documents: %v", err)
	}
	defer rows.Close()

	var docs []*pb.Document
	for rows.Next() {
		var d pb.Document
		if err := rows.Scan(&d.Id, &d.Title, &d.Filename, &d.MimeType, &d.OwnerId, &d.OwnerName, &d.NgacNodeId, &d.WorkspaceId, &d.CreatedAt); err != nil {
			return nil, err
		}
		// NGAC filter: only include docs user can read
		resp, _ := s.policyClient.CheckAccess(ctx, &policypb.CheckAccessRequest{
			UserNodeId: req.UserNgacNodeId, ObjectNodeId: d.NgacNodeId, Operation: "read",
		})
		if resp != nil && resp.Decision == "ALLOW" {
			docs = append(docs, &d)
		}
	}
	return &pb.DocumentList{Documents: docs}, nil
}

func (s *DocumentServer) Delete(ctx context.Context, req *pb.DeleteDocumentRequest) (*pb.Empty, error) {
	var ngacNode string
	s.db.QueryRow(ctx, "SELECT ngac_node FROM documents WHERE id = $1", req.DocumentId).Scan(&ngacNode)
	s.db.Exec(ctx, "DELETE FROM documents WHERE id = $1", req.DocumentId)
	if ngacNode != "" {
		s.policyClient.DeleteNode(ctx, &policypb.DeleteNodeRequest{NodeId: ngacNode})
	}
	return &pb.Empty{}, nil
}

func (s *DocumentServer) Approve(ctx context.Context, req *pb.ApproveDocumentRequest) (*pb.Document, error) {
	var ngacNode string
	s.db.QueryRow(ctx, "SELECT ngac_node FROM documents WHERE id = $1", req.DocumentId).Scan(&ngacNode)

	// Check approve permission
	resp, _ := s.policyClient.CheckAccess(ctx, &policypb.CheckAccessRequest{
		UserNodeId: req.UserNgacNodeId, ObjectNodeId: ngacNode, Operation: "approve",
	})
	if resp != nil && resp.Decision == "DENY" {
		return nil, status.Errorf(codes.PermissionDenied, "not authorized to approve")
	}

	// Move from DraftDocs to ApprovedDocs — find and reassign
	parents, _ := s.policyClient.GetParents(ctx, &policypb.GetParentsRequest{NodeId: ngacNode})
	if parents != nil {
		for _, p := range parents.Nodes {
			if strings.HasSuffix(p.Name, "_DraftDocs") {
				s.policyClient.RemoveAssignment(ctx, &policypb.RemoveAssignmentRequest{ChildId: ngacNode, ParentId: p.Id})
				// Find ApprovedDocs in same workspace
				approvedName := strings.Replace(p.Name, "_DraftDocs", "_ApprovedDocs", 1)
				approved, _ := s.policyClient.FindNodeByName(ctx, &policypb.FindNodeByNameRequest{Name: approvedName, NodeType: "OA"})
				if approved != nil {
					s.policyClient.CreateAssignment(ctx, &policypb.CreateAssignmentRequest{ChildId: ngacNode, ParentId: approved.Id})
				}
				break
			}
		}
	}
	return s.Get(ctx, &pb.GetDocumentRequest{DocumentId: req.DocumentId, UserNgacNodeId: req.UserNgacNodeId})
}

func (s *DocumentServer) Share(ctx context.Context, req *pb.ShareDocumentRequest) (*pb.ShareInfo, error) {
	var ngacNode string
	s.db.QueryRow(ctx, "SELECT ngac_node FROM documents WHERE id = $1", req.DocumentId).Scan(&ngacNode)

	// Create share OA
	shareOA, err := s.policyClient.CreateNode(ctx, &policypb.CreateNodeRequest{
		Name: fmt.Sprintf("Share_%s", req.DocumentId[:8]), NodeType: "OA",
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "create share OA: %v", err)
	}
	// Assign doc to share OA
	s.policyClient.CreateAssignment(ctx, &policypb.CreateAssignmentRequest{ChildId: ngacNode, ParentId: shareOA.Id})
	// Find PC_Global and assign share OA
	pcGlobal, _ := s.policyClient.FindNodeByName(ctx, &policypb.FindNodeByNameRequest{Name: "PC_Global", NodeType: "PC"})
	if pcGlobal != nil {
		s.policyClient.CreateAssignment(ctx, &policypb.CreateAssignmentRequest{ChildId: shareOA.Id, ParentId: pcGlobal.Id})
	}
	// Create association
	s.policyClient.CreateAssociation(ctx, &policypb.CreateAssociationRequest{
		UaId: req.TargetUaId, OaId: shareOA.Id, Operations: req.Operations,
	})

	return &pb.ShareInfo{
		Id: shareOA.Id, DocumentId: req.DocumentId,
		TargetUaId: req.TargetUaId, Operations: req.Operations,
		ShareOaId: shareOA.Id,
	}, nil
}

func (s *DocumentServer) RevokeShare(ctx context.Context, req *pb.RevokeShareRequest) (*pb.Empty, error) {
	s.policyClient.DeleteNode(ctx, &policypb.DeleteNodeRequest{NodeId: req.ShareOaId})
	return &pb.Empty{}, nil
}

func (s *DocumentServer) ListShares(ctx context.Context, req *pb.ListSharesRequest) (*pb.ShareList, error) {
	return &pb.ShareList{}, nil
}

func (s *DocumentServer) Publish(ctx context.Context, req *pb.PublishDocumentRequest) (*pb.Empty, error) {
	var ngacNode string
	s.db.QueryRow(ctx, "SELECT ngac_node FROM documents WHERE id = $1", req.DocumentId).Scan(&ngacNode)
	publicDocs, _ := s.policyClient.FindNodeByName(ctx, &policypb.FindNodeByNameRequest{Name: "PublicDocs", NodeType: "OA"})
	if publicDocs != nil {
		s.policyClient.CreateAssignment(ctx, &policypb.CreateAssignmentRequest{ChildId: ngacNode, ParentId: publicDocs.Id})
	}
	return &pb.Empty{}, nil
}

func (s *DocumentServer) Unpublish(ctx context.Context, req *pb.UnpublishDocumentRequest) (*pb.Empty, error) {
	var ngacNode string
	s.db.QueryRow(ctx, "SELECT ngac_node FROM documents WHERE id = $1", req.DocumentId).Scan(&ngacNode)
	publicDocs, _ := s.policyClient.FindNodeByName(ctx, &policypb.FindNodeByNameRequest{Name: "PublicDocs", NodeType: "OA"})
	if publicDocs != nil {
		s.policyClient.RemoveAssignment(ctx, &policypb.RemoveAssignmentRequest{ChildId: ngacNode, ParentId: publicDocs.Id})
	}
	return &pb.Empty{}, nil
}

func (s *DocumentServer) CheckAccess(ctx context.Context, req *pb.CheckDocAccessRequest) (*pb.DocAccessDecision, error) {
	resp, err := s.policyClient.CheckAccess(ctx, &policypb.CheckAccessRequest{
		UserNodeId: req.UserNgacNodeId, ObjectNodeId: req.ObjectNgacNodeId, Operation: req.Operation,
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "check access: %v", err)
	}
	return &pb.DocAccessDecision{
		Decision: resp.Decision, User: resp.User, Object: resp.Object, Operation: resp.Operation,
	}, nil
}
