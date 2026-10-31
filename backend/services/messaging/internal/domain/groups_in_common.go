package domain

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"ngac-platform/ngac"
	"ngac-platform/pkg/policyclient"
	pb "ngac-platform/proto/messaging"
	policypb "ngac-platform/proto/policy"
)

// GroupsInCommon lists the channels of one workspace (groups and the
// workspace-wide channel, never a direct message) that both the caller and
// another person are in: what a profile in Danh bạ calls "nhóm chung".
//
// It says nothing the caller could not already see. The caller must be a member
// of the workspace before any channel is looked at; a channel is listed only
// when the caller sits in its members UA and the policy lets them read it, and
// the other person sits in the same UA. Membership is the graph's (the direct
// assignment under the channel's members UA, as ListMembers reads it), not the
// member cache. A policy answer that cannot be had fails the whole request
// rather than shortening the list.
//
// The policy is asked a fixed number of questions whatever the number of
// groups: one batch read check for the caller over every candidate, and one
// direct-parents lookup per person, intersected with the groups' members UAs
// in memory.
func (s *Service) GroupsInCommon(ctx context.Context, workspaceID, callerNodeID, otherNodeID string) ([]*pb.Channel, error) {
	if callerNodeID == "" {
		return nil, fmt.Errorf("%w: no caller", ErrAccessDenied)
	}
	if workspaceID == "" || otherNodeID == "" {
		return nil, fmt.Errorf("%w: workspace and person are required", ErrInvalidInput)
	}
	if otherNodeID == callerNodeID {
		return nil, fmt.Errorf("%w: groups in common need another person", ErrInvalidInput)
	}

	member, err := policyclient.IsWorkspaceMember(ctx, s.policyRead, callerNodeID, ngac.WorkspaceID(workspaceID))
	if err != nil {
		return nil, fmt.Errorf("check workspace membership: %w", err)
	}
	if !member {
		return nil, fmt.Errorf("%w: not a member of the workspace", ErrAccessDenied)
	}

	channels, err := s.store.ListChannelsByWorkspace(ctx, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("list channels: %w", err)
	}
	candidates := channels[:0:0]
	objectIDs := make([]string, 0, len(channels))
	for _, ch := range channels {
		// The store already leaves direct messages out; a channel with no members UA has no members to share.
		if ch.ChannelType == "dm" || ch.WorkspaceID != workspaceID || ch.NGACOaID == "" || ch.NGACUaID == "" {
			continue
		}
		candidates = append(candidates, ch)
		objectIDs = append(objectIDs, ch.NGACOaID)
	}
	if len(candidates) == 0 {
		return []*pb.Channel{}, nil
	}

	batch, err := policyclient.New(s.policyRead).BatchCheck(ctx, callerNodeID, objectIDs, []string{ngac.OpRead})
	if err != nil {
		return nil, fmt.Errorf("batch access check: %w", err)
	}

	mine, err := s.directParents(ctx, callerNodeID)
	if err != nil {
		return nil, err
	}
	theirs, err := s.directParents(ctx, otherNodeID)
	if err != nil {
		return nil, err
	}

	out := make([]*pb.Channel, 0, len(candidates))
	for _, ch := range candidates {
		if batch.Has(ch.NGACOaID, ngac.OpRead) && mine[ch.NGACUaID] && theirs[ch.NGACUaID] {
			out = append(out, channelToProto(ch))
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name) })
	return out, nil
}

// directParents is the set of nodes the user is directly assigned to: a
// channel's members are the direct children of its members UA.
func (s *Service) directParents(ctx context.Context, nodeID string) (map[string]bool, error) {
	res, err := s.policyRead.GetParents(ctx, &policypb.GetParentsRequest{NodeId: nodeID})
	if err != nil {
		return nil, fmt.Errorf("check channel membership: %w", err)
	}
	out := make(map[string]bool, len(res.GetNodes()))
	for _, n := range res.GetNodes() {
		out[n.GetId()] = true
	}
	return out, nil
}
