package grpc

import (
	"context"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"ngac-platform/services/policy/internal/ngac"
	pb "ngac-platform/proto/policy"
)

type PolicyServer struct {
	pb.UnimplementedPolicyServiceServer
	store      *ngac.Store
	constraint *ngac.ConstraintEngine
}

func NewPolicyServer(store *ngac.Store, constraint *ngac.ConstraintEngine) *PolicyServer {
	return &PolicyServer{store: store, constraint: constraint}
}

func (s *PolicyServer) CheckAccess(ctx context.Context, req *pb.CheckAccessRequest) (*pb.AccessDecision, error) {
	graph := s.store.GetGraph()
	decision := graph.CheckAccess(req.UserNodeId, req.ObjectNodeId, req.Operation)

	resp := &pb.AccessDecision{
		Decision:  decision.Decision,
		User:      decision.User,
		Object:    decision.Object,
		Operation: decision.Operation,
		Explanation: &pb.AccessExplanation{
			Path:              decision.Explanation.Path,
			PolicyClass:       decision.Explanation.PolicyClass,
			UserAttributes:    decision.Explanation.UserAttributes,
			ObjectAttributes:  decision.Explanation.ObjectAttributes,
			Reason:            decision.Explanation.Reason,
			ConstraintsChecked: decision.Explanation.ConstraintsChecked,
		},
	}

	// If graph allows, check constraints
	if decision.Decision == "ALLOW" && s.constraint != nil {
		reqCtx := ngac.RequestContext{
			Time: time.Now(), UserID: req.UserNodeId,
			ObjectID: req.ObjectNodeId, Operation: req.Operation,
		}
		denied, name, msg, checked := s.constraint.Evaluate(reqCtx)
		resp.Explanation.ConstraintsChecked = checked
		if denied {
			resp.Decision = "DENY"
			resp.Explanation.Reason = msg
			resp.Explanation.ConstraintDenied = &pb.ConstraintDenial{Name: name, Message: msg}
		}
	}

	return resp, nil
}

func (s *PolicyServer) CreateNode(ctx context.Context, req *pb.CreateNodeRequest) (*pb.NGACNode, error) {
	props := make(map[string]string)
	for k, v := range req.Properties {
		props[k] = v
	}
	node, err := s.store.CreateNode(ctx, req.Name, req.NodeType, props)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "create node: %v", err)
	}
	return nodeToProto(node), nil
}

func (s *PolicyServer) DeleteNode(ctx context.Context, req *pb.DeleteNodeRequest) (*pb.Empty, error) {
	if err := s.store.DeleteNode(ctx, req.NodeId); err != nil {
		return nil, status.Errorf(codes.Internal, "delete node: %v", err)
	}
	return &pb.Empty{}, nil
}

func (s *PolicyServer) GetNode(ctx context.Context, req *pb.GetNodeRequest) (*pb.NGACNode, error) {
	node := s.store.GetNode(req.NodeId)
	if node == nil {
		return nil, status.Errorf(codes.NotFound, "node %s not found", req.NodeId)
	}
	return nodeToProto(node), nil
}

func (s *PolicyServer) FindNodeByName(ctx context.Context, req *pb.FindNodeByNameRequest) (*pb.NGACNode, error) {
	node := s.store.FindNodeByName(req.Name, req.NodeType)
	if node == nil {
		return nil, status.Errorf(codes.NotFound, "node %s (%s) not found", req.Name, req.NodeType)
	}
	return nodeToProto(node), nil
}

func (s *PolicyServer) GetNodesByType(ctx context.Context, req *pb.GetNodesByTypeRequest) (*pb.NodeList, error) {
	nodes := s.store.GetNodesByType(req.NodeType)
	return &pb.NodeList{Nodes: nodesToProto(nodes)}, nil
}

func (s *PolicyServer) CreateAssignment(ctx context.Context, req *pb.CreateAssignmentRequest) (*pb.Assignment, error) {
	a, err := s.store.CreateAssignment(ctx, req.ChildId, req.ParentId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "create assignment: %v", err)
	}
	return &pb.Assignment{Id: a.ID, ChildId: a.ChildID, ParentId: a.ParentID}, nil
}

func (s *PolicyServer) RemoveAssignment(ctx context.Context, req *pb.RemoveAssignmentRequest) (*pb.Empty, error) {
	if err := s.store.RemoveAssignment(ctx, req.ChildId, req.ParentId); err != nil {
		return nil, status.Errorf(codes.Internal, "remove assignment: %v", err)
	}
	return &pb.Empty{}, nil
}

func (s *PolicyServer) IsAssigned(ctx context.Context, req *pb.IsAssignedRequest) (*pb.BoolResponse, error) {
	return &pb.BoolResponse{Value: s.store.IsAssigned(req.ChildId, req.ParentId)}, nil
}

func (s *PolicyServer) CreateAssociation(ctx context.Context, req *pb.CreateAssociationRequest) (*pb.Association, error) {
	a, err := s.store.CreateAssociation(ctx, req.UaId, req.OaId, req.Operations)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "create association: %v", err)
	}
	return &pb.Association{Id: a.ID, UaId: a.UAID, OaId: a.OAID, Operations: a.Operations}, nil
}

func (s *PolicyServer) RemoveAssociation(ctx context.Context, req *pb.RemoveAssociationRequest) (*pb.Empty, error) {
	if err := s.store.RemoveAssociationByUAOA(ctx, req.UaId, req.OaId); err != nil {
		return nil, status.Errorf(codes.Internal, "remove association: %v", err)
	}
	return &pb.Empty{}, nil
}

func (s *PolicyServer) GetAncestors(ctx context.Context, req *pb.GetAncestorsRequest) (*pb.NodeList, error) {
	graph := s.store.GetGraph()
	ancestors := graph.GetAncestors(req.NodeId)
	var nodes []*ngac.NGACNode
	for _, n := range ancestors {
		nodes = append(nodes, n)
	}
	return &pb.NodeList{Nodes: nodesToProto(nodes)}, nil
}

func (s *PolicyServer) GetDescendants(ctx context.Context, req *pb.GetDescendantsRequest) (*pb.NodeList, error) {
	graph := s.store.GetGraph()
	desc := graph.GetDescendants(req.NodeId)
	var nodes []*ngac.NGACNode
	for _, n := range desc {
		nodes = append(nodes, n)
	}
	return &pb.NodeList{Nodes: nodesToProto(nodes)}, nil
}

func (s *PolicyServer) GetChildren(ctx context.Context, req *pb.GetChildrenRequest) (*pb.NodeList, error) {
	graph := s.store.GetGraph()
	children := graph.GetChildren(req.NodeId)
	return &pb.NodeList{Nodes: nodesToProto(children)}, nil
}

func (s *PolicyServer) GetParents(ctx context.Context, req *pb.GetParentsRequest) (*pb.NodeList, error) {
	graph := s.store.GetGraph()
	parents := graph.GetParents(req.NodeId)
	return &pb.NodeList{Nodes: nodesToProto(parents)}, nil
}

func (s *PolicyServer) InitSchema(ctx context.Context, _ *pb.Empty) (*pb.Empty, error) {
	if err := s.store.InitSchema(ctx); err != nil {
		return nil, status.Errorf(codes.Internal, "init schema: %v", err)
	}
	return &pb.Empty{}, nil
}

func (s *PolicyServer) LoadGraph(ctx context.Context, _ *pb.Empty) (*pb.Empty, error) {
	if err := s.store.LoadGraph(ctx); err != nil {
		return nil, status.Errorf(codes.Internal, "load graph: %v", err)
	}
	return &pb.Empty{}, nil
}

func nodeToProto(n *ngac.NGACNode) *pb.NGACNode {
	props := make(map[string]string)
	for k, v := range n.Properties {
		props[k] = v
	}
	return &pb.NGACNode{
		Id: n.ID, Name: n.Name, NodeType: n.NodeType,
		Properties: props, CreatedAt: timestamppb.New(n.CreatedAt),
	}
}

func nodesToProto(nodes []*ngac.NGACNode) []*pb.NGACNode {
	var result []*pb.NGACNode
	for _, n := range nodes {
		result = append(result, nodeToProto(n))
	}
	return result
}
