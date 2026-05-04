package grpc

import (
	"google.golang.org/protobuf/types/known/timestamppb"

	pb "ngac-platform/proto/policy"
	"ngac-platform/services/policy/internal/ngac"
)

// nodeToProto converts an internal NGACNode to its protobuf representation.
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

// nodesToProto converts a slice of internal NGACNodes to protobuf.
func nodesToProto(nodes []*ngac.NGACNode) []*pb.NGACNode {
	var result []*pb.NGACNode
	for _, n := range nodes {
		result = append(result, nodeToProto(n))
	}
	return result
}
