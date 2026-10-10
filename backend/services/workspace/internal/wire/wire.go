// Package wire converts the workspace domain's results to the workspace proto
// messages, which are also the JSON the REST edge answers with. Both transports
// use it so the two shapes cannot drift.
package wire

import (
	pb "ngac-platform/proto/workspace"
	"ngac-platform/services/workspace/internal/domain"
)

// Workspace converts a domain workspace.
func Workspace(r *domain.WorkspaceResult) *pb.Workspace {
	return &pb.Workspace{
		Id: r.ID, Name: r.Name, PcNodeId: r.PcNodeID,
		OwnersUaId: r.OwnersUaID, MembersUaId: r.MembersUaID,
		MgmtOaId: r.MgmtOaID, DocumentsOaId: r.DocumentsOaID,
		ChannelsOaId: r.ChannelsOaID, CreatedBy: r.CreatedBy,
	}
}

// Workspaces converts a list of domain workspaces.
func Workspaces(rs []*domain.WorkspaceResult) *pb.WorkspaceList {
	var ws []*pb.Workspace
	for _, r := range rs {
		ws = append(ws, Workspace(r))
	}
	return &pb.WorkspaceList{Workspaces: ws}
}

// Members converts a list of domain members.
func Members(ms []*domain.Member) *pb.MemberList {
	var out []*pb.Member
	for _, m := range ms {
		out = append(out, &pb.Member{NgacNodeId: m.NGACNodeID, Username: m.Username})
	}
	return &pb.MemberList{Members: out}
}

// Folder converts a domain folder.
func Folder(f *domain.Folder) *pb.Folder {
	return &pb.Folder{Id: f.ID, Name: f.Name, NgacNodeId: f.NGACNodeID}
}
